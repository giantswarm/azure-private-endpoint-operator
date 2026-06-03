package testhelpers

import (
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	kcp "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
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

func (b *ClusterBuilder) WithDeletionTimestamp(time time.Time) *ClusterBuilder {
	ts := meta.NewTime(time)
	b.o.DeletionTimestamp = &ts
	return b
}

func (b *ClusterBuilder) WithPause() *ClusterBuilder {
	b.o.Spec.Paused = new(true)
	return b
}

func (b *ClusterBuilder) WithControlPlane(kcp *kcp.KubeadmControlPlane) *ClusterBuilder {
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

func (b *ClusterBuilder) WithAzureCluster(ac *capz.AzureCluster) *ClusterBuilder {
	b.o.Spec.InfrastructureRef = capi.ContractVersionedObjectReference{
		APIGroup: capi.GroupVersionInfrastructure.Group,
		Kind:     ac.Kind,
		Name:     ac.Name,
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
