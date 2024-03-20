package privateendpoints

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azurecluster"
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

func NewScope(ctx context.Context, cluster *capz.AzureCluster, client client.Client, privateEndpointClient azure.PrivateEndpointsClient) (Scope, error) {
	if cluster == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "cluster must be set")
	}
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must be set")
	}
	if privateEndpointClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "privateEndpointClient must be set")
	}

	var nodeSubnet *capz.SubnetSpec
	for i := range cluster.Spec.NetworkSpec.Subnets {
		subnet := &cluster.Spec.NetworkSpec.Subnets[i]

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
		if len(cluster.Spec.NetworkSpec.Subnets) == 0 {
			return nil, microerror.Maskf(errors.SubnetsNotSetError, "the cluster does not have any subnets set")
		}

		privateEndpointsSubnet = &cluster.Spec.NetworkSpec.Subnets[0]
	}

	baseScope, err := azurecluster.NewBaseScope(cluster, client)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	privateEndpointsScope := scope{
		BaseScope:              *baseScope,
		privateEndpoints:       &privateEndpointsSubnet.PrivateEndpoints,
		privateEndpointsClient: privateEndpointClient,
	}

	return &privateEndpointsScope, nil
}

type scope struct {
	azurecluster.BaseScope
	privateEndpoints       *capz.PrivateEndpoints
	privateEndpointsClient azure.PrivateEndpointsClient
}

func (s *scope) GetPrivateEndpointIPAddress(ctx context.Context, privateEndpointName string) (net.IP, error) {
	privateEndpointResponse, err := s.privateEndpointsClient.Get(
		ctx,
		s.GetResourceGroup(),
		privateEndpointName,
		&armnetwork.PrivateEndpointsClientGetOptions{
			Expand: to.Ptr[string]("NetworkInterfaces"),
		})
	if errors.IsAzureResourceNotFound(err) {
		return net.IP{}, microerror.Maskf(errors.PrivateEndpointNotFoundError, "private endpoint not found")
	} else if err != nil {
		return net.IP{}, microerror.Mask(err)
	}
	privateEndpoint := privateEndpointResponse.PrivateEndpoint

	var result net.IP
	if privateEndpoint.Properties == nil ||
		privateEndpoint.Properties.NetworkInterfaces == nil ||
		len(privateEndpoint.Properties.NetworkInterfaces) == 0 {
		return result, microerror.Maskf(errors.PrivateEndpointNetworkInterfaceNotFoundError, "could not find private endpoint network interface")
	}

	found := false
	for _, networkInterface := range privateEndpoint.Properties.NetworkInterfaces {
		if networkInterface == nil ||
			networkInterface.Properties == nil ||
			networkInterface.Properties.IPConfigurations == nil ||
			len(networkInterface.Properties.IPConfigurations) == 0 {
			continue
		}
		for _, ipConfig := range networkInterface.Properties.IPConfigurations {
			if ipConfig == nil ||
				ipConfig.Properties == nil ||
				ipConfig.Properties.PrivateIPAddress == nil {
				continue
			}
			result = net.ParseIP(*ipConfig.Properties.PrivateIPAddress)
			found = true
			break
		}
		if found {
			break
		}
	}

	if found {
		return result, nil
	} else {
		return result, microerror.Maskf(
			errors.PrivateEndpointNetworkInterfacePrivateAddressNotFoundError,
			fmt.Sprintf("could not find private endpoint network interface private IP address, got private endpoint: %v", privateEndpoint))
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
