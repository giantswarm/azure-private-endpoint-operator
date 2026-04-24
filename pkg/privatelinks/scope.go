package privatelinks

import (
	"fmt"
	"net"
	"regexp"

	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azurecluster"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/util"
)

const (
	AzurePrivateEndpointOperatorApiServerAnnotation string = "azure-private-endpoint-operator.giantswarm.io/private-link-apiserver-ip"
	AzurePrivateEndpointOperatorMcIngressAnnotation string = "azure-private-endpoint-operator.giantswarm.io/private-link-mc-ingress-ip"
)

var (
	ingressEndpointRegex = regexp.MustCompile(`-privatelink-privateendpoint$`)
	gatewayEndpointRegex = regexp.MustCompile(`-gateway-privateendpoint$`)
)

func NewScope(workloadCluster *capz.AzureCluster, client client.Client, mcIngressIPSource string) (*Scope, error) {
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
		BaseScope:         *baseScope,
		privateLinks:      workloadCluster.Spec.NetworkSpec.APIServerLB.PrivateLinks,
		mcIngressIPSource: mcIngressIPSource,
	}

	return &scope, nil
}

type Scope struct {
	azurecluster.BaseScope
	privateLinks      []capz.PrivateLink
	mcIngressIPSource string
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
		if util.ContainsPtr(privateLink.AllowedSubscriptions, managementClusterSubscriptionID) {
			privateLinksWhereMCSubscriptionIsAllowed = append(privateLinksWhereMCSubscriptionIsAllowed, privateLink)
		}
	}
	return privateLinksWhereMCSubscriptionIsAllowed
}

func (s *Scope) PrivateLinksReady() bool {
	return s.IsConditionTrue(capz.PrivateLinksReadyCondition)
}

// IsMcIngressEndpoint returns true if the given endpoint name matches the pattern
// for the configured MC ingress IP source (ingress or gateway).
func (s *Scope) IsMcIngressEndpoint(name string) bool {
	switch s.mcIngressIPSource {
	case "ingress":
		return ingressEndpointRegex.MatchString(name)
	case "gateway":
		return gatewayEndpointRegex.MatchString(name)
	}
	return false
}

func (s *Scope) SetPrivateEndpointIPAddressForWcApi(ip net.IP) {
	s.SetAnnotation(AzurePrivateEndpointOperatorApiServerAnnotation, ip.String())
}

func (s *Scope) SetPrivateEndpointIPAddressForMcIngress(ip net.IP) {
	s.SetAnnotation(AzurePrivateEndpointOperatorMcIngressAnnotation, ip.String())
}
