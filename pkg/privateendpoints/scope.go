package privateendpoints

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azurecluster"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

const (
	clientSecretKeyName = "clientSecret"
)

func NewScope(ctx context.Context, managementCluster *capz.AzureCluster, client client.Client) (*scope, error) {
	if managementCluster == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "managementCluster must be set")
	}
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must be set")
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

	privateEndpointClient, err := newPrivateEndpointClient(ctx, client, managementCluster)
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

func newPrivateEndpointClient(ctx context.Context, client client.Client, azureCluster *capz.AzureCluster) (*armnetwork.PrivateEndpointsClient, error) {
	var cred azcore.TokenCredential
	var err error

	azureClusterIdentity := &capz.AzureClusterIdentity{}
	name := types.NamespacedName{
		Namespace: azureCluster.Spec.IdentityRef.Namespace,
		Name:      azureCluster.Spec.IdentityRef.Name,
	}
	err = client.Get(ctx, name, azureClusterIdentity)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	switch azureClusterIdentity.Spec.Type {
	case capz.UserAssignedMSI:
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(azureClusterIdentity.Spec.ClientID),
		})
		if err != nil {
			return nil, microerror.Mask(err)
		}
	case capz.ManualServicePrincipal:
		clientSecretName := types.NamespacedName{
			Namespace: azureClusterIdentity.Spec.ClientSecret.Namespace,
			Name:      azureClusterIdentity.Spec.ClientSecret.Name,
		}
		secret := &corev1.Secret{}
		err = client.Get(ctx, clientSecretName, secret)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		cred, err = azidentity.NewClientSecretCredential(
			azureClusterIdentity.Spec.TenantID,
			azureClusterIdentity.Spec.ClientID,
			string(secret.Data[clientSecretKeyName]),
			nil)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	privateEndpointsClient, err := armnetwork.NewPrivateEndpointsClient(azureCluster.Spec.SubscriptionID, cred, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return privateEndpointsClient, nil
}

type scope struct {
	azurecluster.BaseScope
	privateEndpoints       *capz.PrivateEndpoints
	privateEndpointsClient *armnetwork.PrivateEndpointsClient
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
