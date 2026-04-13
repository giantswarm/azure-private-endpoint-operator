package testhelpers

import (
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
)

func NewKubeadmControlPlaneBuilder(namespace, name string) *KubeadmControlPlaneBuilder {
	b := &KubeadmControlPlaneBuilder{
		o: new(v1beta1.KubeadmControlPlane),
	}

	b.o.SetNamespace(namespace)
	b.o.SetName(name)

	return b
}

type KubeadmControlPlaneBuilder struct {
	o *v1beta1.KubeadmControlPlane
}

func (b *KubeadmControlPlaneBuilder) WithDeletionTimestamp() *KubeadmControlPlaneBuilder {
	// Generate a timestamp 10 seconds in the past.
	time := v1.NewTime(time.Now().Add(time.Duration(-10) * time.Second))
	b.o.ObjectMeta.SetDeletionTimestamp(&time)
	return b
}

func (b *KubeadmControlPlaneBuilder) WithStatusProvisioned() *KubeadmControlPlaneBuilder {
	b.o.Status.Ready = true
	return b
}

func (b *KubeadmControlPlaneBuilder) Build() *v1beta1.KubeadmControlPlane {
	return b.o
}
