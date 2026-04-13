package testhelpers

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewClusterBuilder(scheme *runtime.Scheme) *ClusterBuilder {
	return &ClusterBuilder{
		o:      new(clusterv1.Cluster),
		scheme: scheme,
	}
}

type ClusterBuilder struct {
	o      *clusterv1.Cluster
	scheme *runtime.Scheme
}

func (b *ClusterBuilder) WithPause() *ClusterBuilder {
	b.o.Spec.Paused = true
	return b
}

func (b *ClusterBuilder) WithControlPlane(kcp *kcpv1.KubeadmControlPlane) *ClusterBuilder {
	b.o.ObjectMeta.Namespace = kcp.Namespace
	b.o.ObjectMeta.Name = kcp.Name
	b.o.Spec.ControlPlaneRef = &v1.ObjectReference{
		Kind:      kcp.Kind,
		Namespace: kcp.Namespace,
		Name:      kcp.Name,
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
	b.o.Spec.InfrastructureRef = &v1.ObjectReference{
		Kind:      ac.Kind,
		Namespace: ac.Namespace,
		Name:      ac.Name,
	}
	return b
}

func (b *ClusterBuilder) Build() *clusterv1.Cluster {
	return b.o
}
