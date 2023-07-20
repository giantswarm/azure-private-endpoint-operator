package testhelpers

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type AzureClusterBuilder struct {
	subscriptionID string
	location       string
	resourceGroup  string
	subnets        capz.Subnets
	apiServerLB    capz.LoadBalancerSpec
	privateLinks   []capz.PrivateLink
	conditions     capi.Conditions
}

func NewAzureClusterBuilder(subscriptionID, resourceGroup string) *AzureClusterBuilder {
	return &AzureClusterBuilder{
		subscriptionID: subscriptionID,
		resourceGroup:  resourceGroup,
	}
}

func (b *AzureClusterBuilder) WithLocation(location string) *AzureClusterBuilder {
	b.location = location
	return b
}

func (b *AzureClusterBuilder) WithSubnet(name string, role capz.SubnetRole, privateEndpoints capz.PrivateEndpoints) *AzureClusterBuilder {
	b.subnets = append(b.subnets, capz.SubnetSpec{
		SubnetClassSpec: capz.SubnetClassSpec{
			Name:             name,
			Role:             role,
			PrivateEndpoints: privateEndpoints,
		},
	})
	return b
}

func (b *AzureClusterBuilder) WithAPILoadBalancerType(lbType capz.LBType) *AzureClusterBuilder {
	b.apiServerLB.Type = lbType
	return b
}

func (b *AzureClusterBuilder) WithPrivateLink(privateLink capz.PrivateLink) *AzureClusterBuilder {
	b.apiServerLB.PrivateLinks = append(b.apiServerLB.PrivateLinks, privateLink)
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
			Name:      b.resourceGroup,
			Namespace: "org-giantswarm",
		},
		Spec: capz.AzureClusterSpec{
			ResourceGroup: b.resourceGroup,
			AzureClusterClassSpec: capz.AzureClusterClassSpec{
				SubscriptionID: b.subscriptionID,
				Location:       b.location,
			},
			NetworkSpec: capz.NetworkSpec{
				APIServerLB: b.apiServerLB,
				Subnets:     b.subnets,
			},
		},
		Status: capz.AzureClusterStatus{
			Conditions: b.conditions,
		},
	}

	return &azureCluster
}
