package privatelinks

import (
	"fmt"
	"net"

	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azurecluster"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

const (
	AzurePrivateEndpointOperatorApiServerAnnotation string = "azure-private-endpoint-operator.giantswarm.io/private-link-apiserver-ip"
	AzurePrivateEndpointOperatorMcIngressAnnotation string = "azure-private-endpoint-operator.giantswarm.io/private-link-mc-ingress-ip"
)

func NewScope(workloadCluster *capz.AzureCluster, client client.Client) (*Scope, error) {
	if workloadCluster == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "workloadCluster must be set")
	}
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must be set")
	}

	baseScope, err := azurecluster.NewBaseScope(workloadCluster, client)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	scope := Scope{
		BaseScope:    *baseScope,
		privateLinks: workloadCluster.Spec.NetworkSpec.APIServerLB.PrivateLinks,
	}

	return &scope, nil
}

type Scope struct {
	azurecluster.BaseScope
	privateLinks []capz.PrivateLink
}

func (s *Scope) LookupPrivateLink(privateLinkResourceID string) (capz.PrivateLink, bool) {
	for _, privateLink := range s.privateLinks {
		currentPrivateLinkResourceID := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
			s.GetSubscriptionID(),
			s.GetResourceGroup(),
			privateLink.Name)

		if currentPrivateLinkResourceID == privateLinkResourceID {
			return privateLink, true
		}
	}

	return capz.PrivateLink{}, false
}

func (s *Scope) GetPrivateLinksWithAllowedSubscription(managementClusterSubscriptionID string) []capz.PrivateLink {
	var privateLinksWhereMCSubscriptionIsAllowed []capz.PrivateLink
	for _, privateLink := range s.privateLinks {
		if containsPtr(privateLink.AllowedSubscriptions, managementClusterSubscriptionID) {
			privateLinksWhereMCSubscriptionIsAllowed = append(privateLinksWhereMCSubscriptionIsAllowed, privateLink)
		}
	}
	return privateLinksWhereMCSubscriptionIsAllowed
}

func (s *Scope) PrivateLinksReady() bool {
	return s.IsConditionTrue(capz.PrivateLinksReadyCondition)
}

func (s *Scope) SetPrivateEndpointIPAddressForWcApi(ip net.IP) {
	s.BaseScope.SetAnnotation(AzurePrivateEndpointOperatorApiServerAnnotation, ip.String())
}

func (s *Scope) SetPrivateEndpointIPAddressForMcIngress(ip net.IP) {
	s.BaseScope.SetAnnotation(AzurePrivateEndpointOperatorMcIngressAnnotation, ip.String())
}

func containsPtr(slice []*string, str string) bool {
	for _, v := range slice {
		if v != nil && *v == str {
			return true
		}
	}
	return false
}
