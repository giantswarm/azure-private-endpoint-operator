package testhelpers

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func NewAzureClusterIdentityBuilder(namespace, name string) *AzureClusterIdentityBuilder {
	return &AzureClusterIdentityBuilder{
		namespace: namespace,
		name:      name,
	}
}

type AzureClusterIdentityBuilder struct {
	namespace, name string
	tenantID        string
	clientID        string
}

func (b *AzureClusterIdentityBuilder) WithTenantID(id string) *AzureClusterIdentityBuilder {
	b.tenantID = id
	return b
}

func (b *AzureClusterIdentityBuilder) WithClientID(id string) *AzureClusterIdentityBuilder {
	b.clientID = id
	return b
}

func (b *AzureClusterIdentityBuilder) Build() *capz.AzureClusterIdentity {
	return &capz.AzureClusterIdentity{
		TypeMeta: v1.TypeMeta{
			Kind: capz.AzureClusterIdentityKind,
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: b.namespace,
			Name:      b.name,
		},
		Spec: capz.AzureClusterIdentitySpec{
			Type:     capz.WorkloadIdentity,
			TenantID: b.tenantID,
			ClientID: b.clientID,
		},
	}
}
