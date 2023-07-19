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
		Name: FakePrivateLinkConnectionName(subscriptionID, resourceGroup, privateLinkName),
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
	}

	return privateEndpoint
}

func FakePrivateLinkConnectionName(subscriptionID, resourceGroup, privateLinkName string) string {
	return fmt.Sprintf("connection-to-%s-%s-%s", subscriptionID, resourceGroup, privateLinkName)
}
