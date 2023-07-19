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

	"github.com/giantswarm/microerror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
		Client:                client,
		managementClusterName: managementClusterName,
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
	err = r.Client.Get(ctx, req.NamespacedName, &workloadAzureCluster)
	if apierrors.IsNotFound(err) {
		logger.Info("AzureCluster no longer exists")
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// If the workload cluster is public then we return, as there is no need to create a private endpoint to access
	// a public load balancer
	if workloadAzureCluster.Spec.NetworkSpec.APIServerLB.Type == capz.Public {
		logger.Info(fmt.Sprintf("Skipping reconciliation of public workload cluster %s", workloadAzureCluster.Name))
		return ctrl.Result{}, nil
	}

	// We now expect that we are working with an internal load balancer (which is used for private clusters), so any
	// other load balancer type (e.g. potentially added in the future) is considered an error here.
	if workloadAzureCluster.Spec.NetworkSpec.APIServerLB.Type != capz.Internal {
		return ctrl.Result{},
			microerror.Maskf(
				errors.UnknownLoadBalancerTypeError,
				"expected that load balancer type is %s, got %s",
				capz.Internal,
				workloadAzureCluster.Spec.NetworkSpec.APIServerLB.Type)
	}

	var managementAzureCluster capz.AzureCluster
	err = r.Client.Get(ctx, r.managementClusterName, &managementAzureCluster)
	if err != nil {
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
	privateEndpointsClient, err := r.privateEndpointsClientCreator(ctx, r.Client, &managementAzureCluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	privateEndpointsScope, err := privateendpoints.NewScope(ctx, &managementAzureCluster, r.Client, privateEndpointsClient)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	// Always close the scope when exiting this function, so we can persist any MC AzureCluster changes.
	defer func() {
		if closeErr := privateEndpointsScope.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Finally, reconcile private links to private endpoints
	privateEndpointsService, err := privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	if workloadAzureCluster.DeletionTimestamp.IsZero() {
		r.setFinalizer(&workloadAzureCluster)
		err = privateEndpointsService.Reconcile(ctx)
		if errors.IsRetriable(err) {
			logger.Info("A retriable error occurred, trying again in a minute", "error", err)
			return ctrl.Result{
				RequeueAfter: time.Minute,
			}, nil
		} else if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	} else {
		err = privateEndpointsService.Delete(ctx)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		r.removeFinalizer(&workloadAzureCluster)
	}

	return ctrl.Result{}, nil
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
