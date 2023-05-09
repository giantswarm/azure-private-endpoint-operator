package privateendpoints

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

type Service struct {
	ManagementClusterScope ManagementClusterScope
	WorkloadClusterScope   WorkloadClusterScope
}

func NewService(managementClusterScope ManagementClusterScope, workloadClusterScope WorkloadClusterScope) (*Service, error) {
	if managementClusterScope == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "managementClusterScope must be set")
	}
	if workloadClusterScope == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "workloadClusterScope must be set")
	}

	return &Service{
		ManagementClusterScope: managementClusterScope,
		WorkloadClusterScope:   workloadClusterScope,
	}, nil
}

func (s *Service) Reconcile(_ context.Context) error {
	//
	// First get all workload cluster private links. We will create private endpoints for all of
	// them (by default there will be only one).
	//
	privateLinks, err := s.WorkloadClusterScope.GetPrivateLinks(
		s.ManagementClusterScope.GetName().Name,
		s.ManagementClusterScope.GetSubscriptionID())
	if err != nil {
		return microerror.Mask(err)
	}

	//
	// Add new private endpoints
	//
	for _, privateLink := range privateLinks {
		manualApproval := !slice.Contains(privateLink.AutoApprovedSubscriptions, s.ManagementClusterScope.GetSubscriptionID())
		var requestMessage string
		if manualApproval {
			requestMessage = fmt.Sprintf("Giant Swarm azure-private-endpoint-operator that is running in "+
				"management cluster %s created private endpoint in order to access private workload cluster %s",
				s.ManagementClusterScope.GetName().Name,
				s.WorkloadClusterScope.GetName().Name)
		}

		wantedPrivateEndpoint := capz.PrivateEndpointSpec{
			Name:     fmt.Sprintf("%s-privateendpoint", privateLink.Name),
			Location: s.ManagementClusterScope.GetLocation(),
			PrivateLinkServiceConnections: []capz.PrivateLinkServiceConnection{
				{
					Name: fmt.Sprintf("%s-connection", privateLink.Name),
					PrivateLinkServiceID: fmt.Sprintf(
						"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
						s.WorkloadClusterScope.GetSubscriptionID(),
						s.WorkloadClusterScope.GetResourceGroup(),
						privateLink.Name),
					RequestMessage: requestMessage,
				},
			},
			ManualApproval: manualApproval,
		}
		s.ManagementClusterScope.AddPrivateEndpointSpec(wantedPrivateEndpoint)
	}

	//
	// Remove unused private endpoints that are connecting to the deleted private links.
	// We are not deleting private links from running workload clusters, nor we planned to do so, but implementing this
	// for the sake of the implementation being more future-proof.
	//
	privateEndpointsToWorkloadCluster := s.ManagementClusterScope.GetPrivateEndpointsToWorkloadCluster(
		s.WorkloadClusterScope.GetSubscriptionID(),
		s.WorkloadClusterScope.GetResourceGroup())
	for _, privateEndpoint := range privateEndpointsToWorkloadCluster {
		privateEndpointIsUsed := false
		// check all private link connections in the private endpoint (we create just one, there shouldn't be more, but
		// here we check the whole slice just in case)
		for _, privateLinkConnection := range privateEndpoint.PrivateLinkServiceConnections {
			_, foundPrivateLinkForTheConnection := s.WorkloadClusterScope.LookupPrivateLink(privateLinkConnection.PrivateLinkServiceID)
			if foundPrivateLinkForTheConnection {
				privateEndpointIsUsed = true
			}
		}
		if !privateEndpointIsUsed {
			s.ManagementClusterScope.RemovePrivateEndpoint(privateEndpoint)
		}
	}

	return nil
}
