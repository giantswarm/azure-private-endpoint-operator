package testhelpers

import (
	"fmt"

	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type PrivateEndpointBuilder struct {
	name                          string
	location                      string
	privateLinkServiceConnections []capz.PrivateLinkServiceConnection
}

func NewPrivateEndpointBuilder(name string) *PrivateEndpointBuilder {
	return &PrivateEndpointBuilder{
		name: name,
	}
}

func (b *PrivateEndpointBuilder) WithLocation(location string) *PrivateEndpointBuilder {
	b.location = location
	return b
}

func (b *PrivateEndpointBuilder) WithPrivateLinkServiceConnection(subscriptionID, resourceGroup, privateLinkName string) *PrivateEndpointBuilder {
	b.privateLinkServiceConnections = append(b.privateLinkServiceConnections, capz.PrivateLinkServiceConnection{
		Name: FakePrivateLinkConnectionName(privateLinkName),
		PrivateLinkServiceID: fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
			subscriptionID,
			resourceGroup,
			privateLinkName),
	})
	return b
}

func (b *PrivateEndpointBuilder) WithPrivateLinkServiceConnectionWithName(subscriptionID, resourceGroup, privateLinkName, connectionName string) *PrivateEndpointBuilder {
	b.privateLinkServiceConnections = append(b.privateLinkServiceConnections, capz.PrivateLinkServiceConnection{
		Name: connectionName,
		PrivateLinkServiceID: fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
			subscriptionID,
			resourceGroup,
			privateLinkName),
	})
	return b
}
func (b *PrivateEndpointBuilder) Build() capz.PrivateEndpointSpec {
	privateEndpoint := capz.PrivateEndpointSpec{
		Name:                          b.name,
		Location:                      b.location,
		PrivateLinkServiceConnections: b.privateLinkServiceConnections,
		ManualApproval:                false,
	}

	return privateEndpoint
}

func FakePrivateLinkConnectionName(privateLinkName string) string {
	return fmt.Sprintf("%s-connection", privateLinkName)
}
