package azurecluster

import (
	"context"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BaseScope struct {
	azureCluster *capz.AzureCluster
	patchHelper  *patch.Helper
}

func NewBaseScope(azureCluster *capz.AzureCluster, client client.Client) (*BaseScope, error) {
	patchHelper, err := patch.NewHelper(azureCluster, client)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &BaseScope{
		azureCluster: azureCluster,
		patchHelper:  patchHelper,
	}, nil
}

func (s *BaseScope) GetClusterName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: s.azureCluster.Namespace,
		Name:      s.azureCluster.Name,
	}
}

func (s *BaseScope) GetSubscriptionID() string {
	return s.azureCluster.Spec.SubscriptionID
}

func (s *BaseScope) GetLocation() string {
	return s.azureCluster.Spec.Location
}

func (s *BaseScope) GetResourceGroup() string {
	return s.azureCluster.Spec.ResourceGroup
}

func (s *BaseScope) PatchObject(ctx context.Context) error {
	err := s.patchHelper.Patch(ctx, s.azureCluster)
	if err != nil {
		return microerror.Mask(err)
	}
	return nil
}

func (s *BaseScope) IsConditionTrue(conditionType capi.ConditionType) bool {
	return conditions.IsTrue(s.azureCluster, conditionType)
}

func (s *BaseScope) Close(ctx context.Context) error {
	err := s.PatchObject(ctx)
	if err != nil {
		return microerror.Mask(err)
	}
	return nil
}
