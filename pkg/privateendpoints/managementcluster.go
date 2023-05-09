package privateendpoints

import (
	"context"
	"fmt"
	"strings"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

type ManagementClusterScope interface {
	GetName() types.NamespacedName
	GetSubscriptionID() string
	GetLocation() string
	GetResourceGroup() string
	GetPrivateEndpoints() []capz.PrivateEndpointSpec
	GetPrivateEndpointsToWorkloadCluster(workloadClusterSubscriptionID, workloadClusterResourceGroup string) []capz.PrivateEndpointSpec
	ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec) bool
	AddPrivateEndpointSpec(capz.PrivateEndpointSpec)
	RemovePrivateEndpointByName(string)
}

func NewManagementClusterScope(_ context.Context, managementCluster *capz.AzureCluster) (ManagementClusterScope, error) {
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

	scope := managementClusterScope{
		baseWorkloadClusterScope: newBaseWorkloadClusterScope(managementCluster),
		privateEndpoints:         privateEndpointsSubnet.PrivateEndpoints,
	}

	return &scope, nil
}

type managementClusterScope struct {
	baseWorkloadClusterScope
	privateEndpoints []capz.PrivateEndpointSpec
}

func (s managementClusterScope) GetPrivateEndpointsToWorkloadCluster(workloadClusterSubscriptionID, workloadClusterResourceGroup string) []capz.PrivateEndpointSpec {
	workloadClusterSubscriptionIDPrefix := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s",
		workloadClusterSubscriptionID,
		workloadClusterResourceGroup)
	var privateEndpointsToWorkloadCluster []capz.PrivateEndpointSpec
	for _, privateEndpoint := range s.privateEndpoints {
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

func (s managementClusterScope) ContainsPrivateEndpointSpec(privateEndpoint capz.PrivateEndpointSpec) bool {
	return sliceContains(s.privateEndpoints, privateEndpoint, arePrivateEndpointsEqual)
}

func (s managementClusterScope) GetPrivateEndpoints() []capz.PrivateEndpointSpec {
	return s.privateEndpoints
}

func (s managementClusterScope) AddPrivateEndpointSpec(spec capz.PrivateEndpointSpec) {
	if !s.ContainsPrivateEndpointSpec(spec) {
		s.privateEndpoints = append(s.privateEndpoints, spec)
	}
}

func (s managementClusterScope) RemovePrivateEndpointByName(privateEndpointName string) {
	for i := len(s.privateEndpoints) - 1; i >= 0; i-- {
		if s.privateEndpoints[i].Name == privateEndpointName {
			s.privateEndpoints = append(
				s.privateEndpoints[:i],
				s.privateEndpoints[i+1:]...)
			break
		}
	}
}

func arePrivateEndpointsEqual(a, b capz.PrivateEndpointSpec) bool {
	return a.Name == b.Name
}
