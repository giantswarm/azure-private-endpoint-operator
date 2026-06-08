package testhelpers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kcp "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func NewKubeadmControlPlaneBuilder(namespace, name string) *KubeadmControlPlaneBuilder {
	b := &KubeadmControlPlaneBuilder{
		o: new(kcp.KubeadmControlPlane),
	}

	b.o.SetNamespace(namespace)
	b.o.SetName(name)

	return b
}

type KubeadmControlPlaneBuilder struct {
	o *kcp.KubeadmControlPlane
}

func (b *KubeadmControlPlaneBuilder) WithPause() *KubeadmControlPlaneBuilder {
	annotations := b.o.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[capi.PausedAnnotation] = "true"
	b.o.SetAnnotations(annotations)
	return b
}

func (b *KubeadmControlPlaneBuilder) WithDeletionTimestamp() *KubeadmControlPlaneBuilder {
	// Generate a timestamp 10 seconds in the past.
	time := metav1.NewTime(time.Now().Add(time.Duration(-10) * time.Second))
	b.o.SetDeletionTimestamp(&time)
	return b
}

func (b *KubeadmControlPlaneBuilder) WithStatusProvisioned() *KubeadmControlPlaneBuilder {
	b.o.Status.Initialization.ControlPlaneInitialized = new(true)
	return b
}

func (b *KubeadmControlPlaneBuilder) Build() *kcp.KubeadmControlPlane {
	return b.o
}
