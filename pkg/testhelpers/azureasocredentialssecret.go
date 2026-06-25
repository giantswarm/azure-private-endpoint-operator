package testhelpers

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewAzureASOCredentialsSecretBuilder(namespace, name string) *AzureASOCredentialsSecretBuilder {
	b := new(AzureASOCredentialsSecretBuilder)
	b.o = &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: make(map[string]string),
	}
	return b
}

type AzureASOCredentialsSecretBuilder struct {
	o *corev1.Secret
}

func (b *AzureASOCredentialsSecretBuilder) WithSubscriptionID(id string) *AzureASOCredentialsSecretBuilder {
	b.o.StringData["AZURE_SUBSCRIPTION_ID"] = id
	return b
}

func (b *AzureASOCredentialsSecretBuilder) WithTenantID(id string) *AzureASOCredentialsSecretBuilder {
	b.o.StringData["AZURE_TENANT_ID"] = id
	return b
}

func (b *AzureASOCredentialsSecretBuilder) WithClientID(id string) *AzureASOCredentialsSecretBuilder {
	b.o.StringData["AZURE_CLIENT_ID"] = id
	return b
}

func (b *AzureASOCredentialsSecretBuilder) Build() *corev1.Secret {
	b.o.StringData["AUTH_MODE"] = "workloadidentity"
	return b.o.DeepCopy()
}
