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
	"strings"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

// AzureClusterReconciler reconciles a AzureCluster object
type AzureClusterReconciler struct {
	client.Client
	managementClusterName types.NamespacedName
}

func NewAzureClusterReconciler(client client.Client, managementClusterName types.NamespacedName) (*AzureClusterReconciler, error) {
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must be set")
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AzureCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *AzureClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	// First we get workload cluster AzureCluster CR, and we check if the cluster is private or public.
	var workloadCluster capz.AzureCluster
	err = r.Client.Get(ctx, req.NamespacedName, &workloadCluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// If the workload cluster is public then we return, as there is no need to create a private endpoint to access
	// a public load balancer
	if workloadCluster.Spec.NetworkSpec.APIServerLB.Type == capz.Public {
		logger.Info(fmt.Sprintf("Skipping reconciliation of public workload cluster %s", workloadCluster.Name))
		return ctrl.Result{}, nil
	}

	// We now expect that we are working with an internal load balancer (which is used for private clusters), so any
	// other load balancer type (e.g. potentially added in the future) is considered an error here.
	if workloadCluster.Spec.NetworkSpec.APIServerLB.Type != capz.Internal {
		return ctrl.Result{},
			microerror.Maskf(
				errors.UnknownLoadBalancerTypeError,
				"expected that load balancer type is %s, got %s",
				capz.Internal,
				workloadCluster.Spec.NetworkSpec.APIServerLB.Type)
	}

	managementClusterChanged := false

	var managementCluster capz.AzureCluster
	err = r.Client.Get(ctx, r.managementClusterName, &managementCluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Update the management cluster if it had any changes
	defer func() {
		// Skip updating if there was an error
		if err != nil {
			return
		}
		if !managementClusterChanged {
			return
		}
		err = r.Client.Update(ctx, &managementCluster)
		if err != nil {
			err = microerror.Mask(err)
		}
	}()

	// <start> moved to management cluster scope
	var mcNodeSubnet *capz.SubnetSpec
	for i := range managementCluster.Spec.NetworkSpec.Subnets {
		subnet := &managementCluster.Spec.NetworkSpec.Subnets[i]

		// We add private endpoints to "node" subnet, or to be precise, we allocate private endpoint IPs from the "node"
		// subnet, the endpoints are not really "added" to the subnet.
		if subnet.Role == capz.SubnetNode {
			mcNodeSubnet = subnet
			break
		}
	}

	if mcNodeSubnet == nil {
		logger.Info(fmt.Sprintf("Node subnet not found in management cluster %s, private endpoints will not be added", managementCluster.Name))
		return ctrl.Result{}, nil
	}
	// <end>

	//
	// Add new private endpoint
	//
	mcSubscriptionID := managementCluster.Spec.SubscriptionID
	for _, privateLink := range workloadCluster.Spec.NetworkSpec.APIServerLB.PrivateLinks {
		if !slice.Contains(privateLink.AllowedSubscriptions, mcSubscriptionID) {
			return ctrl.Result{},
				microerror.Maskf(
					errors.SubscriptionCannotConnectToPrivateLinkError,
					"MC subscription %s cannot connect to private link %s from private workload cluster %s, update private link allowed subscriptions",
					mcSubscriptionID,
					privateLink.Name,
					workloadCluster.Name)
		}

		manualApproval := !slice.Contains(privateLink.AutoApprovedSubscriptions, mcSubscriptionID)
		var requestMessage string
		if manualApproval {
			requestMessage = fmt.Sprintf("Giant Swarm azure-private-endpoint-operator that is running in "+
				"management cluster %s created private endpoint in order to access private workload cluster %s",
				managementCluster.Name, workloadCluster.Name)
		}

		privateEndpoint := capz.PrivateEndpointSpec{
			Name:     fmt.Sprintf("%s-privateendpoint", privateLink.Name),
			Location: managementCluster.Spec.Location,
			PrivateLinkServiceConnections: []capz.PrivateLinkServiceConnection{
				{
					Name: fmt.Sprintf("%s-connection", privateLink.Name),
					PrivateLinkServiceID: fmt.Sprintf(
						"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
						workloadCluster.Spec.SubscriptionID,
						workloadCluster.Spec.ResourceGroup,
						privateLink.Name),
					RequestMessage: requestMessage,
				},
			},
			ManualApproval: manualApproval,
		}

		// Check if the management cluster already contains wanted private endpoint, and add it if it doesn't.
		mcSubnetContainsWantedPrivateEndpoint := Contains(mcNodeSubnet.PrivateEndpoints, privateEndpoint, func(a, b capz.PrivateEndpointSpec) bool {
			return a.Name == b.Name
		})
		if !mcSubnetContainsWantedPrivateEndpoint {
			mcNodeSubnet.PrivateEndpoints = append(mcNodeSubnet.PrivateEndpoints, privateEndpoint)
			managementClusterChanged = true
		}
	}

	wantedPrivateLinks := workloadCluster.Spec.NetworkSpec.APIServerLB.PrivateLinks

	// Remove unwanted private endpoints
	for i := len(mcNodeSubnet.PrivateEndpoints) - 1; i >= 0; i-- {
		privateEndpoint := &mcNodeSubnet.PrivateEndpoints[i]

		// Remove connections to private links that do not exist in current workload cluster
		for j := len(privateEndpoint.PrivateLinkServiceConnections) - 1; j >= 0; j-- {
			currentPrivateLinkConnection := privateEndpoint.PrivateLinkServiceConnections[j]

			// We are checking connections for the currently reconciled workload cluster
			workloadClusterPrivateLinkIDPrefix := fmt.Sprintf(
				"/subscriptions/%s/resourceGroups/%s",
				workloadCluster.Spec.SubscriptionID,
				workloadCluster.Spec.ResourceGroup)
			connectionForReconciledWorkloadCluster := strings.Index(currentPrivateLinkConnection.PrivateLinkServiceID, workloadClusterPrivateLinkIDPrefix) == 0
			if !connectionForReconciledWorkloadCluster {
				continue
			}

			// Remove unwanted connection
			currentPrivateLinkID := currentPrivateLinkConnection.PrivateLinkServiceID
			isCurrentPrivateLinkConnectionWanted := Contains(wantedPrivateLinks, currentPrivateLinkID, func(pl capz.PrivateLink, id string) bool {
				wantedPrivateLinkServiceID := fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
					workloadCluster.Spec.SubscriptionID,
					workloadCluster.Spec.ResourceGroup,
					pl.Name)

				return wantedPrivateLinkServiceID == id
			})
			if !isCurrentPrivateLinkConnectionWanted {
				mcNodeSubnet.PrivateEndpoints[i].PrivateLinkServiceConnections = append(
					mcNodeSubnet.PrivateEndpoints[i].PrivateLinkServiceConnections[:j],
					mcNodeSubnet.PrivateEndpoints[i].PrivateLinkServiceConnections[j+1:]...)
				managementClusterChanged = true
			}
		}

		endpointWanted := len(privateEndpoint.PrivateLinkServiceConnections) > 0
		if !endpointWanted {
			mcNodeSubnet.PrivateEndpoints = append(
				mcNodeSubnet.PrivateEndpoints[:i],
				mcNodeSubnet.PrivateEndpoints[i+1:]...)
			managementClusterChanged = true
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capz.AzureCluster{}).
		Complete(r)
}

func Contains[T1, T2 any](items []T1, t T2, equal func(a T1, b T2) bool) bool {
	for _, item := range items {
		if equal(item, t) {
			return true
		}
	}

	return false
}
