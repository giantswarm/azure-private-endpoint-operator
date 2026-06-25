package testhelpers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func NewAzureASOManagedControlPlaneBuilder(namespace, name string) *AzureASOManagedControlPlaneBuilder {
	b := &AzureASOManagedControlPlaneBuilder{
		o:              new(capz.AzureASOManagedControlPlane),
		managedCluster: new(unstructured.Unstructured),
	}

	b.o.SetNamespace(namespace)
	b.o.SetName(name)

	b.managedCluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "containerservice.azure.com",
		Version: "v1api20240901",
		Kind:    capz.AzureASOManagedClusterKind,
	})
	b.managedCluster.SetNamespace(namespace)
	b.managedCluster.SetName(name)

	return b
}

type AzureASOManagedControlPlaneBuilder struct {
	o              *capz.AzureASOManagedControlPlane
	managedCluster *unstructured.Unstructured
}

func (b *AzureASOManagedControlPlaneBuilder) WithCredentialSecret(o *corev1.Secret) *AzureASOManagedControlPlaneBuilder {
	b.managedCluster.SetAnnotations(map[string]string{
		"serviceoperator.azure.com/credential-from": o.Name,
	})
	return b
}

func (b *AzureASOManagedControlPlaneBuilder) Build() *capz.AzureASOManagedControlPlane {
	b.o.Kind = capz.AzureASOManagedControlPlaneKind
	b.o.Spec.Resources = []runtime.RawExtension{
		{Object: b.managedCluster.DeepCopy()},
	}

	return b.o
}
