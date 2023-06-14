package privateendpoints

import (
	"context"
	"fmt"
	"net"
	"strings"

	// ignore SA1019 deprecated package because we are using CAPZ that still uses the old package
	//nolint:staticcheck
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/giantswarm/microerror"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azurecluster"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

func NewScope(_ context.Context, managementCluster *capz.AzureCluster, client client.Client, authorizer azure.Authorizer) (Scope, error) {
	if managementCluster == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "managementCluster must be set")
	}

	var nodeSubnet *capz.SubnetSpec
	for i := range managementCluster.Spec.NetworkSpec.Subnets {
		subnet := &managementCluster.Spec.NetworkSpec.Subnets[i]

		// We add private endpoints to "node" subnet, or to be precise, we allocate private endpoint IPs from the "node"
		// subnet, the endpoints are not really "added" to the subnet.
		if subnet.Role == capz.SubnetNode {
			nodeSubnet = subnet
			break
		}
	}

	var privateEndpointsSubnet *capz.SubnetSpec
	if nodeSubnet != nil {
		privateEndpointsSubnet = nodeSubnet
	} else {
		if len(managementCluster.Spec.NetworkSpec.Subnets) == 0 {
			return nil, microerror.Maskf(errors.SubnetsNotSetError, "Management cluster does not have any subnets set")
		}

		privateEndpointsSubnet = &managementCluster.Spec.NetworkSpec.Subnets[0]
	}

	baseScope, err := azurecluster.NewBaseScope(managementCluster, client)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	privateEndpointClient := network.NewPrivateEndpointsClientWithBaseURI(authorizer.BaseURI(), authorizer.SubscriptionID())
	azure.SetAutoRestClientDefaults(&privateEndpointClient.Client, authorizer.Authorizer())

	privateEndpointsScope := scope{
		BaseScope:              *baseScope,
		privateEndpoints:       &privateEndpointsSubnet.PrivateEndpoints,
		privateEndpointsClient: &privateEndpointClient,
	}

	return &privateEndpointsScope, nil
}

type scope struct {
	azurecluster.BaseScope
	privateEndpoints       *capz.PrivateEndpoints
	privateEndpointsClient *network.PrivateEndpointsClient
}

func (s *scope) GetPrivateEndpointIPAddress(ctx context.Context, privateEndpointName string) (net.IP, error) {
	privateEndpoint, err := s.privateEndpointsClient.Get(ctx, s.GetResourceGroup(), privateEndpointName, "")
	if err != nil {
		return net.IP{}, microerror.Mask(err)
	}
	var result net.IP
	if privateEndpoint.NetworkInterfaces == nil {
		return result, microerror.Maskf(errors.PrivateEndpointNetworkInterfaceNotFoundError, "could not find private endpoint network interface")
	}

	found := false
	for _, networkInterface := range *privateEndpoint.NetworkInterfaces {
		if networkInterface.IPConfigurations == nil {
			continue
		}
		for _, ipConfig := range *networkInterface.IPConfigurations {
			if ipConfig.PrivateIPAddress == nil {
				continue
			}
			result = net.ParseIP(*ipConfig.PrivateIPAddress)
			found = true
		}
		if found {
			break
		}
	}

	if found {
		return result, nil
	} else {
		return result, microerror.Maskf(errors.PrivateEndpointNetworkInterfacePrivateAddressNotFoundError, "could not find private endpoint network interface private IP address")
	}
}

func (s *scope) GetPrivateEndpointsToWorkloadCluster(workloadClusterSubscriptionID, workloadClusterResourceGroup string) []capz.PrivateEndpointSpec {
	workloadClusterSubscriptionIDPrefix := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s",
		workloadClusterSubscriptionID,
		workloadClusterResourceGroup)
	var privateEndpointsToWorkloadCluster []capz.PrivateEndpointSpec
	for _, privateEndpoint := range *s.privateEndpoints {
		foundPrivateEndpoint := false
		for _, connection := range privateEndpoint.PrivateLinkServiceConnections {
			if strings.Index(connection.PrivateLinkServiceID, workloadClusterSubscriptionIDPrefix) == 0 {
				foundPrivateEndpoint = true
				break
			}
		}
		if foundPrivateEndpoint {
			privateEndpointsToWorkloadCluster = append(privateEndpointsToWorkloadCluster, privateEndpoint)
		}
	}

	return privateEndpointsToWorkloadCluster
}

func (s *scope) ContainsPrivateEndpointSpec(privateEndpoint capz.PrivateEndpointSpec) bool {
	return sliceContains(*s.privateEndpoints, privateEndpoint, arePrivateEndpointsEqual)
}

func (s *scope) GetPrivateEndpoints() []capz.PrivateEndpointSpec {
	return *s.privateEndpoints
}

func (s *scope) AddPrivateEndpointSpec(spec capz.PrivateEndpointSpec) {
	if !s.ContainsPrivateEndpointSpec(spec) {
		*s.privateEndpoints = append(*s.privateEndpoints, spec)
	}
}

func (s *scope) RemovePrivateEndpointByName(privateEndpointName string) {
	for i := len(*s.privateEndpoints) - 1; i >= 0; i-- {
		if (*s.privateEndpoints)[i].Name == privateEndpointName {
			*s.privateEndpoints = append(
				(*s.privateEndpoints)[:i],
				(*s.privateEndpoints)[i+1:]...)
			break
		}
	}
}

func arePrivateEndpointsEqual(a, b capz.PrivateEndpointSpec) bool {
	return a.Name == b.Name
}

func sliceContains[T1, T2 any](items []T1, t T2, equal func(a T1, b T2) bool) bool {
	for _, item := range items {
		if equal(item, t) {
			return true
		}
	}

	return false
}
