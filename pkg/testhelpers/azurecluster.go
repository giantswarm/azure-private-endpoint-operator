package testhelpers

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type AzureClusterBuilder struct {
	subscriptionID string
	resourceGroup  string
	subnets        capz.Subnets
	privateLinks   []capz.PrivateLink
	conditions     capi.Conditions
}

func NewAzureClusterBuilder(subscriptionID, resourceGroup string) *AzureClusterBuilder {
	return &AzureClusterBuilder{
		subscriptionID: subscriptionID,
		resourceGroup:  resourceGroup,
	}
}

func (b *AzureClusterBuilder) WithSubnet(name string, role capz.SubnetRole) *AzureClusterBuilder {
	b.subnets = append(b.subnets, capz.SubnetSpec{
		SubnetClassSpec: capz.SubnetClassSpec{
			Name: name,
			Role: role,
		},
	})
	return b
}

func (b *AzureClusterBuilder) WithPrivateLink(privateLink capz.PrivateLink) *AzureClusterBuilder {
	b.privateLinks = append(b.privateLinks, privateLink)
	return b
}

func (b *AzureClusterBuilder) WithCondition(condition *capi.Condition) *AzureClusterBuilder {
	if condition != nil {
		b.conditions = append(b.conditions, *condition)
	}
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
				Subnets: b.subnets,
			},
		},
		Status: capz.AzureClusterStatus{
			Conditions: b.conditions,
		},
	}

	return &azureCluster
}
