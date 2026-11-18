/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privateendpoints"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privatelinks"
)

const (
	AzureClusterControllerFinalizer string = "azure-private-endpoint-operator.giantswarm.io/azurecluster"
)

type NetworkMode int

const (
	NetworkModeAnnotation string = "giantswarm.io/network-mode"

	NetworkModePublic NetworkMode = iota
	NetworkModePrivate
	NetworkModeBYO
)

// Options holds optional configuration for AzureClusterReconciler.
type Options struct{}

// AzureClusterReconciler reconciles a AzureCluster object
type AzureClusterReconciler struct {
	client.Client
	// apiReader reads from the API server instead of the cache, see removeFinalizer.
	apiReader                     client.Reader
	privateEndpointsClientCreator azure.PrivateEndpointsClientCreator
	managementClusterName         types.NamespacedName
	options                       Options
}

func NewAzureClusterReconciler(client client.Client, apiReader client.Reader, privateEndpointsClientCreator azure.PrivateEndpointsClientCreator, managementClusterName types.NamespacedName, options Options) (*AzureClusterReconciler, error) {
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must be set")
	}
	if apiReader == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "apiReader must be set")
	}
	if privateEndpointsClientCreator == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "privateEndpointsClientCreator must be set")
	}
	if managementClusterName.Name == "" {
		return nil, microerror.Maskf(errors.InvalidConfigError, "%T.Name must be set", managementClusterName)
	}
	if managementClusterName.Namespace == "" {
		return nil, microerror.Maskf(errors.InvalidConfigError, "%T.Namespace must be set", managementClusterName)
	}
	return &AzureClusterReconciler{
		Client:                        client,
		apiReader:                     apiReader,
		privateEndpointsClientCreator: privateEndpointsClientCreator,
		managementClusterName:         managementClusterName,
		options:                       options,
	}, nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=azureclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=azureclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=azureclusters/finalizers,verbs=update

// Reconcile AzureCluster for private workload clusters by ensuring that there is a private
// endpoint for every private link.
func (r *AzureClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	logger.Info(fmt.Sprintf("Reconciling workload cluster %s", req.NamespacedName))
	defer logger.Info(fmt.Sprintf("Finished reconciling workload cluster %s", req.NamespacedName))

	// First we get workload cluster AzureCluster CR, and we check if the cluster is private or public.
	var workloadAzureCluster capz.AzureCluster
	err = r.Get(ctx, req.NamespacedName, &workloadAzureCluster)
	if apierrors.IsNotFound(err) {
		logger.Info("AzureCluster no longer exists")
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// We don't need to do anything special for connections in MC itself
	if workloadAzureCluster.Name == r.managementClusterName.Name &&
		workloadAzureCluster.Namespace == r.managementClusterName.Namespace {
		logger.Info(fmt.Sprintf("Skipping reconciliation of management cluster %s", workloadAzureCluster.Name))
		return ctrl.Result{}, nil
	}

	var managementAzureCluster capz.AzureCluster
	if err = r.Get(ctx, r.managementClusterName, &managementAzureCluster); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	if err = validateLBType(workloadAzureCluster); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	if err = validateLBType(managementAzureCluster); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Add the finalizer before anything is created in Azure, and before the scopes below create
	// their patch helpers, so they start from the patched object. Finalizers are written with their
	// own patch, see addFinalizer.
	if workloadAzureCluster.DeletionTimestamp.IsZero() {
		if err = r.addFinalizer(ctx, &workloadAzureCluster); err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	}

	// The finalizer is removed only after the scopes have written the other changes to the
	// AzureCluster, so this is deferred before the scopes' Close.
	privateEndpointsDeleted := false
	defer func() {
		if privateEndpointsDeleted && err == nil {
			err = r.removeFinalizer(ctx, &workloadAzureCluster)
		}
	}()

	// Create WC private links scope - we use this to get the info about the private workload
	// cluster private links, and then we make sure to have a private endpoints that connect to the
	// private links.
	privateLinksScope, err := privatelinks.NewScope(&workloadAzureCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	// Always close the scope when exiting this function, so we can persist any WC AzureCluster changes.
	defer func() {
		if closeErr := privateLinksScope.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Create MC private endpoints scope - we use this to get the info about the management cluster
	// private endpoints and to update them.
	mcPrivateEndpointsClient, err := r.privateEndpointsClientCreator(ctx, r.Client, &managementAzureCluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	wcPrivateEndpointsClient, err := r.privateEndpointsClientCreator(ctx, r.Client, &workloadAzureCluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// will be used for MC to WC connections
	mcPrivateEndpointsScope, err := privateendpoints.NewScope(ctx, &managementAzureCluster, r.Client, mcPrivateEndpointsClient)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	// Always close the scope when exiting this function, so we can persist any MC AzureCluster changes.
	defer func() {
		if closeErr := mcPrivateEndpointsScope.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// will be used for WC to MC connections
	// we don't need to close this scope here. WC will be patched by privateLinksScope.Close above.
	wcPrivateEndpointsScope, err := privateendpoints.NewScope(ctx, &workloadAzureCluster, r.Client, wcPrivateEndpointsClient)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	mcPrivateEndpointsService, err := privateendpoints.NewService(mcPrivateEndpointsScope, privateLinksScope)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	wcPrivateEndpointsService, err := privateendpoints.NewService(wcPrivateEndpointsScope, privateLinksScope)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	if workloadAzureCluster.DeletionTimestamp.IsZero() {
		if getNetworkMode(workloadAzureCluster) == NetworkModePrivate {
			err = mcPrivateEndpointsService.ReconcileMcToWcApi(ctx)
		}

		// When the cluster network mode is private, we assume the gateway LB is internal
		// with a private link (<mc-name>-gateway-privatelink). We add a private endpoint to WC
		// so that monitoring tools in WC can access the MC gateway.
		if err == nil && getNetworkMode(managementAzureCluster) == NetworkModePrivate {
			err = wcPrivateEndpointsService.ReconcileWcToMcIngress(ctx, generateWcToMcPrivateEndpointSpecs(workloadAzureCluster, managementAzureCluster))
		}

		if errors.IsRetriable(err) {
			logger.Info("A retriable error occurred, trying again in a minute", "error", err)
			return ctrl.Result{
				RequeueAfter: time.Minute,
			}, nil
		} else if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	} else {
		if getNetworkMode(workloadAzureCluster) == NetworkModePrivate {
			err = mcPrivateEndpointsService.DeleteMcToWcApi(ctx)
		}
		// We don't need to do anything for WC to MC connections.
		// CAPI controllers will clean private endpoints in WC side automatically.

		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		privateEndpointsDeleted = true
	}

	return ctrl.Result{}, nil
}

// generateWcToMcPrivateEndpointSpecs generates PrivateEndpointSpecs for the private endpoint that
// connects the WC to the gateway of the management cluster.
// It assumes a private link named "<mc-name>-gateway-privatelink".
func generateWcToMcPrivateEndpointSpecs(wc capz.AzureCluster, mc capz.AzureCluster) []capz.PrivateEndpointSpec {
	gatewaySpec := capz.PrivateEndpointSpec{
		Name:     fmt.Sprintf("%s-to-%s-gateway-privateendpoint", wc.Name, mc.Name),
		Location: wc.Spec.Location,
		PrivateLinkServiceConnections: []capz.PrivateLinkServiceConnection{
			{
				Name: fmt.Sprintf("%s-to-%s-gateway-connection", wc.Name, mc.Name),
				PrivateLinkServiceID: fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
					mc.Spec.SubscriptionID,
					mc.Name,
					fmt.Sprintf("%s-gateway-privatelink", mc.Name)),
			},
		},
		ManualApproval: false,
	}
	return []capz.PrivateEndpointSpec{gatewaySpec}
}

// validateLBType checks if the load balancer type is either Internal or Public. Any
// other load balancer type (e.g. potentially added in the future) is considered an error here.
func validateLBType(azureCluster capz.AzureCluster) error {
	if azureCluster.Spec.NetworkSpec.APIServerLB.Type != capz.Internal &&
		azureCluster.Spec.NetworkSpec.APIServerLB.Type != capz.Public {
		return microerror.Maskf(
			errors.UnknownLoadBalancerTypeError,
			"expected that load balancer type is %s or %s, got %s in cluster %s",
			capz.Internal,
			capz.Public,
			azureCluster.Spec.NetworkSpec.APIServerLB.Type,
			azureCluster.Name)
	}
	return nil
}

// getNetworkMode returns the cluster's network mode based on an annotation value.
// If the annotation is not present, it falls back to the old behavior and assumes that
// a cluster with an internal loadbalancer is private.
func getNetworkMode(ac capz.AzureCluster) NetworkMode {
	if mode, ok := ac.Annotations[NetworkModeAnnotation]; ok {
		switch mode {
		case "public":
			return NetworkModePublic
		case "private":
			return NetworkModePrivate
		case "byo":
			return NetworkModeBYO
		}
	}

	if ac.Spec.NetworkSpec.APIServerLB.Type == capz.Internal {
		return NetworkModePrivate
	}

	return NetworkModePublic
}

// addFinalizer adds AzureClusterControllerFinalizer to workloadCluster with a merge patch that
// only contains the finalizers and carries its resourceVersion. A merge patch replaces the whole
// finalizers list, and other controllers put finalizers on the AzureCluster too, so without the
// resourceVersion a patch built from an outdated object could add back a finalizer another
// controller has removed. With it, the API server rejects an outdated patch with a 409 Conflict,
// and the reconciliation is retried with the current object.
//
// The patch helpers of the scopes do not send a resourceVersion, which is why the finalizers are
// not left to them.
func (r *AzureClusterReconciler) addFinalizer(ctx context.Context, workloadCluster *capz.AzureCluster) error {
	original := workloadCluster.DeepCopy()
	if !controllerutil.AddFinalizer(workloadCluster, AzureClusterControllerFinalizer) {
		return nil
	}

	return r.Patch(ctx, workloadCluster, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
}

// removeFinalizer removes AzureClusterControllerFinalizer with a merge patch that carries the
// resourceVersion, see addFinalizer. It reads the AzureCluster from the API server, not from the
// cache: the scopes' patch helpers patch copies of workloadCluster, so neither it nor the cache
// has their write yet, and a patch based on either would conflict with it.
func (r *AzureClusterReconciler) removeFinalizer(ctx context.Context, workloadCluster *capz.AzureCluster) error {
	latest := &capz.AzureCluster{}
	if err := r.apiReader.Get(ctx, client.ObjectKeyFromObject(workloadCluster), latest); err != nil {
		return microerror.Mask(client.IgnoreNotFound(err))
	}

	original := latest.DeepCopy()
	if !controllerutil.RemoveFinalizer(latest, AzureClusterControllerFinalizer) {
		return nil
	}

	return microerror.Mask(r.Patch(ctx, latest, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})))
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capz.AzureCluster{}).
		Complete(r)
}
