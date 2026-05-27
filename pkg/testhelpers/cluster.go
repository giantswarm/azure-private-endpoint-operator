package testhelpers

import (
	"k8s.io/apimachinery/pkg/runtime"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	kcp "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewClusterBuilder(scheme *runtime.Scheme) *ClusterBuilder {
	return &ClusterBuilder{
		o:      new(capi.Cluster),
		scheme: scheme,
	}
}

type ClusterBuilder struct {
	o      *capi.Cluster
	scheme *runtime.Scheme
}

func (b *ClusterBuilder) WithPause() *ClusterBuilder {
	b.o.Spec.Paused = new(true)
	return b
}

func (b *ClusterBuilder) WithControlPlane(kcp *kcp.KubeadmControlPlane) *ClusterBuilder {
	b.o.ObjectMeta.Namespace = kcp.Namespace
	b.o.ObjectMeta.Name = kcp.Name
	b.o.Spec.ControlPlaneRef = capi.ContractVersionedObjectReference{
		Kind: kcp.Kind,
		Name: kcp.Name,
	}
	err := ctrl.SetControllerReference(b.o, kcp, b.scheme)
	if err != nil {
		panic(err)
	}
	return b
}

func (b *ClusterBuilder) WithAzureCluster(ac *capz.AzureCluster) *ClusterBuilder {
	b.o.Namespace = ac.Namespace
	b.o.Name = ac.Name
	b.o.Spec.InfrastructureRef = capi.ContractVersionedObjectReference{
		Kind: ac.Kind,
		Name: ac.Name,
	}
	return b
}

func (b *ClusterBuilder) Build() *capi.Cluster {
	return b.o
}
