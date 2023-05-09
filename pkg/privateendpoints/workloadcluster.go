package privateendpoints

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

type WorkloadClusterScope interface {
	GetName() types.NamespacedName
	GetSubscriptionID() string
	GetLocation() string
	GetResourceGroup() string
	GetPrivateLinks(managementClusterName, managementClusterSubscriptionID string) ([]capz.PrivateLink, error)
	LookupPrivateLink(privateLinkResourceID string) (capz.PrivateLink, bool)
}

func NewWorkloadClusterScope(_ context.Context, workloadCluster *capz.AzureCluster) (WorkloadClusterScope, error) {
	if workloadCluster == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "workloadCluster must be set")
	}

	scope := workloadClusterScope{
		baseWorkloadClusterScope: newBaseWorkloadClusterScope(workloadCluster),
		privateLinks:             workloadCluster.Spec.NetworkSpec.APIServerLB.PrivateLinks,
	}

	return &scope, nil
}

type baseWorkloadClusterScope struct {
	name           types.NamespacedName
	subscriptionID string
	location       string
	resourceGroup  string
}

func newBaseWorkloadClusterScope(azureCluster *capz.AzureCluster) baseWorkloadClusterScope {
	return baseWorkloadClusterScope{
		name: types.NamespacedName{
			Namespace: azureCluster.Namespace,
			Name:      azureCluster.Name,
		},
		subscriptionID: azureCluster.Spec.SubscriptionID,
		location:       azureCluster.Spec.Location,
		resourceGroup:  azureCluster.Spec.ResourceGroup,
	}
}

func (s *baseWorkloadClusterScope) GetName() types.NamespacedName {
	return s.name
}

func (s *baseWorkloadClusterScope) GetSubscriptionID() string {
	return s.subscriptionID
}

func (s *baseWorkloadClusterScope) GetLocation() string {
	return s.location
}

func (s *baseWorkloadClusterScope) GetResourceGroup() string {
	return s.resourceGroup
}

type workloadClusterScope struct {
	baseWorkloadClusterScope
	privateLinks []capz.PrivateLink
}

func (s *workloadClusterScope) LookupPrivateLink(privateLinkResourceID string) (capz.PrivateLink, bool) {
	for _, privateLink := range s.privateLinks {
		currentPrivateLinkResourceID := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
			s.subscriptionID,
			s.resourceGroup,
			privateLink.Name)

		if currentPrivateLinkResourceID == privateLinkResourceID {
			return privateLink, true
		}
	}

	return capz.PrivateLink{}, false
}

func (s *workloadClusterScope) GetPrivateLinks(managementClusterName, managementClusterSubscriptionID string) ([]capz.PrivateLink, error) {
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
				s.name)
	}

	return privateLinksWhereMCSubscriptionIsAllowed, nil
}
