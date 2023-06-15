package privateendpoints

import (
	"context"
	"fmt"
	"net"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

// Scope is the interface for working with private endpoints.
type Scope interface {
	GetClusterName() types.NamespacedName
	GetSubscriptionID() string
	GetLocation() string
	GetResourceGroup() string
	GetPrivateEndpoints() []capz.PrivateEndpointSpec
	GetPrivateEndpointsToWorkloadCluster(workloadClusterSubscriptionID, workloadClusterResourceGroup string) []capz.PrivateEndpointSpec
	GetPrivateEndpointIPAddress(ctx context.Context, privateEndpointName string) (net.IP, error)
	ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec) bool
	AddPrivateEndpointSpec(capz.PrivateEndpointSpec)
	RemovePrivateEndpointByName(string)
	PatchObject(ctx context.Context) error
	Close(ctx context.Context) error
}

// PrivateLinksScope is the interface for getting private links for which the private endpoints are needed.
type PrivateLinksScope interface {
	GetClusterName() types.NamespacedName
	GetSubscriptionID() string
	GetLocation() string
	GetResourceGroup() string
	GetPrivateLinks(managementClusterName, managementClusterSubscriptionID string) ([]capz.PrivateLink, error)
	LookupPrivateLink(privateLinkResourceID string) (capz.PrivateLink, bool)
	PatchObject(ctx context.Context) error
	PrivateLinksReady() bool
	SetPrivateEndpointIPAddress(ip net.IP)
	Close(ctx context.Context) error
}

type Service struct {
	privateEndpointsScope Scope
	privateLinksScope     PrivateLinksScope
}

func NewService(privateEndpointsScope Scope, privateLinksScope PrivateLinksScope) (*Service, error) {
	if privateEndpointsScope == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "privateEndpointsScope must be set")
	}
	if privateLinksScope == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "privateLinksScope must be set")
	}

	return &Service{
		privateEndpointsScope: privateEndpointsScope,
		privateLinksScope:     privateLinksScope,
	}, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	logger := log.FromContext(ctx)
	//
	// First get all workload cluster private links. We will create private endpoints for all of
	// them (by default there will be only one).
	//
	privateLinks, err := s.privateLinksScope.GetPrivateLinks(
		s.privateEndpointsScope.GetClusterName().Name,
		s.privateEndpointsScope.GetSubscriptionID())
	if err != nil {
		return microerror.Mask(err)
	}

	if len(privateLinks) > 0 && !s.privateLinksScope.PrivateLinksReady() {
		errorMessage := "Workload cluster private links are not yet ready"
		logger.Info(errorMessage)
		return microerror.Maskf(errors.PrivateLinksNotReadyError, errorMessage)
	}

	//
	// Add new private endpoints
	//
	for _, privateLink := range privateLinks {
		logger.Info(fmt.Sprintf("Found private link %s", privateLink.Name))
		manualApproval := !slice.Contains(privateLink.AutoApprovedSubscriptions, s.privateEndpointsScope.GetSubscriptionID())
		var requestMessage string
		if manualApproval {
			requestMessage = fmt.Sprintf("Giant Swarm azure-private-endpoint-operator that is running in "+
				"management cluster %s created private endpoint in order to access private workload cluster %s",
				s.privateEndpointsScope.GetClusterName().Name,
				s.privateLinksScope.GetClusterName().Name)
		}

		wantedPrivateEndpoint := capz.PrivateEndpointSpec{
			Name:     fmt.Sprintf("%s-privateendpoint", privateLink.Name),
			Location: s.privateEndpointsScope.GetLocation(),
			PrivateLinkServiceConnections: []capz.PrivateLinkServiceConnection{
				{
					Name: fmt.Sprintf("%s-connection", privateLink.Name),
					PrivateLinkServiceID: fmt.Sprintf(
						"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
						s.privateLinksScope.GetSubscriptionID(),
						s.privateLinksScope.GetResourceGroup(),
						privateLink.Name),
					RequestMessage: requestMessage,
				},
			},
			ManualApproval: manualApproval,
		}
		s.privateEndpointsScope.AddPrivateEndpointSpec(wantedPrivateEndpoint)
		logger.Info(fmt.Sprintf("Ensured private endpoint %s is added to %s", wantedPrivateEndpoint.Name, s.privateEndpointsScope.GetClusterName()))

		// Apps running in the MC access WC API server via private endpoint that connects to private
		// link. For that access MC apps are connecting to the private endpoint IP from the MC
		// subnet. Here we set that private endpoint IP in the workload AzureCluster as an
		// annotation. dns-operator-azure will then use that annotation to create A records in the
		// DNS zone in the MC resource group, so that MC apps can access WC API server via FQDN.
		privateEndpointIPAddress, err := s.privateEndpointsScope.GetPrivateEndpointIPAddress(ctx, wantedPrivateEndpoint.Name)
		if err != nil {
			return microerror.Mask(err)
		}
		logger.Info("found private endpoint IP address in MC", "ipAddress", privateEndpointIPAddress.String())
		s.privateLinksScope.SetPrivateEndpointIPAddress(privateEndpointIPAddress)
		logger.Info("set private endpoint IP address in WC AzureCluster", "ipAddress", privateEndpointIPAddress.String())
	}

	//
	// Remove unused private endpoints that are connecting to the deleted private links.
	// We are not deleting private links from running workload clusters, nor we planned to do so, but implementing this
	// for the sake of the implementation being more future-proof.
	//
	privateEndpointsToWorkloadCluster := s.privateEndpointsScope.GetPrivateEndpointsToWorkloadCluster(
		s.privateLinksScope.GetSubscriptionID(),
		s.privateLinksScope.GetResourceGroup())
	for _, privateEndpoint := range privateEndpointsToWorkloadCluster {
		privateEndpointIsUsed := false
		// check all private link connections in the private endpoint (we create just one, there shouldn't be more, but
		// here we check the whole slice just in case)
		for _, privateLinkConnection := range privateEndpoint.PrivateLinkServiceConnections {
			_, foundPrivateLinkForTheConnection := s.privateLinksScope.LookupPrivateLink(privateLinkConnection.PrivateLinkServiceID)
			if foundPrivateLinkForTheConnection {
				privateEndpointIsUsed = true
			}
		}
		if !privateEndpointIsUsed {
			s.privateEndpointsScope.RemovePrivateEndpointByName(privateEndpoint.Name)
			logger.Info(fmt.Sprintf("Removed private endpoint %s that is not used", privateEndpoint.Name))
		}
	}

	return nil
}

func (s *Service) Delete(_ context.Context) error {
	// First get all workload cluster private links. We will delete private endpoints for all of
	// them.
	privateLinks, err := s.privateLinksScope.GetPrivateLinks(
		s.privateEndpointsScope.GetClusterName().Name,
		s.privateEndpointsScope.GetSubscriptionID())
	if err != nil {
		return microerror.Mask(err)
	}

	// For every private link, delete its corresponding private endpoint.
	for _, privateLink := range privateLinks {
		privateEndpointName := fmt.Sprintf("%s-privateendpoint", privateLink.Name)
		s.privateEndpointsScope.RemovePrivateEndpointByName(privateEndpointName)
	}

	return nil
}
