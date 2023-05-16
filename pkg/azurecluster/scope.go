package azurecluster

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type BaseScope struct {
	name           types.NamespacedName
	subscriptionID string
	location       string
	resourceGroup  string
}

func NewBaseScope(azureCluster *v1beta1.AzureCluster) BaseScope {
	return BaseScope{
		name: types.NamespacedName{
			Namespace: azureCluster.Namespace,
			Name:      azureCluster.Name,
		},
		subscriptionID: azureCluster.Spec.SubscriptionID,
		location:       azureCluster.Spec.Location,
		resourceGroup:  azureCluster.Spec.ResourceGroup,
	}
}

func (s *BaseScope) GetClusterName() types.NamespacedName {
	return s.name
}

func (s *BaseScope) GetSubscriptionID() string {
	return s.subscriptionID
}

func (s *BaseScope) GetLocation() string {
	return s.location
}

func (s *BaseScope) GetResourceGroup() string {
	return s.resourceGroup
}
