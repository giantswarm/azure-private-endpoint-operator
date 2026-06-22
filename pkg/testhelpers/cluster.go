package testhelpers

import (
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	kcp "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func NewClusterBuilder(namespace, name string) *ClusterBuilder {
	ensureSchemeSet()

	o := new(capi.Cluster)
	o.SetNamespace(namespace)
	o.SetName(name)
	return &ClusterBuilder{o}
}

type ClusterBuilder struct {
	o *capi.Cluster
}

func (b *ClusterBuilder) WithFinalizers(finalizers ...string) *ClusterBuilder {
	if b.o.Finalizers == nil {
		b.o.Finalizers = make([]string, 0)
	}
	b.o.Finalizers = append(b.o.Finalizers, finalizers...)
	return b
}

func (b *ClusterBuilder) WithDeletionTimestamp(time time.Time) *ClusterBuilder {
	ts := meta.NewTime(time)
	b.o.DeletionTimestamp = &ts
	return b
}

func (b *ClusterBuilder) WithPause() *ClusterBuilder {
	b.o.Spec.Paused = new(true)
	return b
}

func (b *ClusterBuilder) WithKubeadmControlPlane(kcp *kcp.KubeadmControlPlane) *ClusterBuilder {
	b.o.Spec.ControlPlaneRef = capi.ContractVersionedObjectReference{
		APIGroup: capi.GroupVersionControlPlane.Group,
		Kind:     kcp.Kind,
		Name:     kcp.Name,
	}
	err := ctrl.SetControllerReference(b.o, kcp, scheme)
	if err != nil {
		panic(err)
	}
	return b
}

func (b *ClusterBuilder) WithAzureASOManagedControlPlane(o *capz.AzureASOManagedControlPlane) *ClusterBuilder {
	b.o.Spec.ControlPlaneRef = capi.ContractVersionedObjectReference{
		APIGroup: capi.GroupVersionControlPlane.Group,
		Kind:     o.Kind,
		Name:     o.Name,
	}
	b.o.Spec.InfrastructureRef = capi.ContractVersionedObjectReference{
		APIGroup: capi.GroupVersionInfrastructure.Group,
		Kind:     capz.AzureASOManagedClusterKind,
		Name:     o.Name,
	}
	return b
}

func (b *ClusterBuilder) WithAzureCluster(ac *capz.AzureCluster) *ClusterBuilder {
	b.o.Spec.InfrastructureRef = capi.ContractVersionedObjectReference{
		APIGroup: capi.GroupVersionInfrastructure.Group,
		Kind:     ac.Kind,
		Name:     ac.Name,
	}
	return b
}

// WithDummyReferences populates the Cluster with dummy controlplane and infrastructure references.
// This is useful for when you need only need a Cluster object that does not need to point
// to real controlplane or infrastructure objects.
// If you need a dummy controlplane but not a dummy infrastructure reference, or vice versa,
// you can call this method and then afterwards call a method to set a reference to the real object
// that you need.
func (b *ClusterBuilder) WithDummyReferences() *ClusterBuilder {
	b.o.Spec.InfrastructureRef = v1beta2.ContractVersionedObjectReference{
		APIGroup: "infrastructure.cluster.x-k8s.io",
		Kind:     "DummyInfraCluster",
		Name:     "dummy-infracluster",
	}
	b.o.Spec.ControlPlaneRef = v1beta2.ContractVersionedObjectReference{
		APIGroup: "controlplane.cluster.x-k8s.io",
		Kind:     "DummyControlPlane",
		Name:     "dummy-controlplane",
	}
	return b
}

func (b *ClusterBuilder) Build() *capi.Cluster {
	gvk, err := apiutil.GVKForObject(b.o, scheme)
	if err != nil {
		panic(err)
	}
	b.o.SetGroupVersionKind(gvk)
	return b.o
}
