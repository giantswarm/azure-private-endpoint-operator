package testhelpers

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type AzureClusterBuilder struct {
	subscriptionID string
	resourceGroup  string
	privateLinks   []capz.PrivateLink
}

func NewAzureClusterBuilder(subscriptionID, resourceGroup string) *AzureClusterBuilder {
	return &AzureClusterBuilder{
		subscriptionID: subscriptionID,
		resourceGroup:  resourceGroup,
	}
}

func (b *AzureClusterBuilder) WithPrivateLink(privateLink capz.PrivateLink) *AzureClusterBuilder {
	b.privateLinks = append(b.privateLinks, privateLink)
	return b
}

func (b *AzureClusterBuilder) Build() *capz.AzureCluster {
	azureCluster := capz.AzureCluster{
		ObjectMeta: meta.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "org-giantswarm",
		},
		Spec: capz.AzureClusterSpec{
			ResourceGroup: b.resourceGroup,
			AzureClusterClassSpec: capz.AzureClusterClassSpec{
				SubscriptionID: b.subscriptionID,
			},
			NetworkSpec: capz.NetworkSpec{
				APIServerLB: capz.LoadBalancerSpec{
					PrivateLinks: b.privateLinks,
				},
			},
		},
	}

	return &azureCluster
}
