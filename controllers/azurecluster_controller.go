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

// AzureClusterReconciler reconciles a AzureCluster object
type AzureClusterReconciler struct {
	client.Client
	privateEndpointsClientCreator azure.PrivateEndpointsClientCreator
	managementClusterName         types.NamespacedName
}

func NewAzureClusterReconciler(client client.Client, privateEndpointsClientCreator azure.PrivateEndpointsClientCreator, managementClusterName types.NamespacedName) (*AzureClusterReconciler, error) {
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must be set")
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
		privateEndpointsClientCreator: privateEndpointsClientCreator,
		managementClusterName:         managementClusterName,
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
		r.setFinalizer(&workloadAzureCluster)

		if workloadAzureCluster.Spec.NetworkSpec.APIServerLB.Type == capz.Internal {
			err = mcPrivateEndpointsService.ReconcileMcToWcApi(ctx)
		}

		// When LB of k8s api of MC is internal load balancer, we assume
		// - The cluster is private and the ingress LB is also internal LB.
		// - There is already a private link for the ingress LB with the name of MC.
		// We add a private endpoint to WC so that monitoring tools in WC can access ingress of MC.
		if err == nil && managementAzureCluster.Spec.NetworkSpec.APIServerLB.Type == capz.Internal {
			err = wcPrivateEndpointsService.ReconcileWcToMcIngress(ctx, generateWcToMcPrivateEndpointSpec(workloadAzureCluster, managementAzureCluster))
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
		if workloadAzureCluster.Spec.NetworkSpec.APIServerLB.Type == capz.Internal {
			err = mcPrivateEndpointsService.DeleteMcToWcApi(ctx)
		}
		// We don't need to do anything for WC to MC connections.
		// CAPI controllers will clean private endpoints in WC side automatically.

		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		r.removeFinalizer(&workloadAzureCluster)
	}

	return ctrl.Result{}, nil
}

// generateWcToMcPrivateEndpointSpec generates a PrivateEndpointSpec for the private endpoint that
// connects the WC to the ingress of management cluster.
// It assumes we already have a private link with name "<mc-name>-ingress-privatelink".
func generateWcToMcPrivateEndpointSpec(wc capz.AzureCluster, mc capz.AzureCluster) capz.PrivateEndpointSpec {
	return capz.PrivateEndpointSpec{
		Name:     fmt.Sprintf("%s-to-%s-privatelink-privateendpoint", wc.Name, mc.Name),
		Location: wc.Spec.Location,
		PrivateLinkServiceConnections: []capz.PrivateLinkServiceConnection{
			{
				Name: fmt.Sprintf("%s-to-%s-connection", wc.Name, mc.Name),
				PrivateLinkServiceID: fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
					mc.Spec.SubscriptionID,
					mc.Name,
					fmt.Sprintf("%s-ingress-privatelink", mc.Name)),
			},
		},
		ManualApproval: false,
	}
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

func (r *AzureClusterReconciler) setFinalizer(workloadCluster *capz.AzureCluster) {
	if !controllerutil.ContainsFinalizer(workloadCluster, AzureClusterControllerFinalizer) {
		controllerutil.AddFinalizer(workloadCluster, AzureClusterControllerFinalizer)
	}
}

func (r *AzureClusterReconciler) removeFinalizer(workloadCluster *capz.AzureCluster) {
	if controllerutil.ContainsFinalizer(workloadCluster, AzureClusterControllerFinalizer) {
		controllerutil.RemoveFinalizer(workloadCluster, AzureClusterControllerFinalizer)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capz.AzureCluster{}).
		Complete(r)
}
