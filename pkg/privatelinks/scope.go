package privatelinks

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azurecluster"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privateendpoints"
)

func NewScope(_ context.Context, workloadCluster *capz.AzureCluster, client client.Client) (privateendpoints.PrivateLinksScope, error) {
	if workloadCluster == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "workloadCluster must be set")
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

func (s *Scope) PrivateLinksReady() bool {
	return s.IsConditionTrue(capz.PrivateLinksReadyCondition)
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

func (s *Scope) GetPrivateLinks(managementClusterName, managementClusterSubscriptionID string) ([]capz.PrivateLink, error) {
	var privateLinksWhereMCSubscriptionIsAllowed []capz.PrivateLink
	for _, privateLink := range s.privateLinks {
		if !slice.Contains(privateLink.AllowedSubscriptions, managementClusterSubscriptionID) {
			continue
		}
		privateLinksWhereMCSubscriptionIsAllowed = append(privateLinksWhereMCSubscriptionIsAllowed, privateLink)
	}

	// return an error if the MC subscription is not allowed to connect to any API server private link
	if len(privateLinksWhereMCSubscriptionIsAllowed) == 0 && len(s.privateLinks) > 0 {
		return nil,
			microerror.Maskf(
				errors.SubscriptionCannotConnectToPrivateLinkError,
				"MC %s Azure subscription %s is not allowed to connect to any private link from the private workload cluster %s",
				managementClusterName,
				managementClusterSubscriptionID,
				s.GetClusterName())
	}

	return privateLinksWhereMCSubscriptionIsAllowed, nil
}
