package testhelpers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
)

func NewKubeadmControlPlaneBuilder(namespace, name string) *KubeadmControlPlaneBuilder {
	b := &KubeadmControlPlaneBuilder{
		o: new(kcpv1.KubeadmControlPlane),
	}

	b.o.SetNamespace(namespace)
	b.o.SetName(name)

	return b
}

type KubeadmControlPlaneBuilder struct {
	o *kcpv1.KubeadmControlPlane
}

func (b *KubeadmControlPlaneBuilder) WithPause() *KubeadmControlPlaneBuilder {
	annotations := b.o.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[clusterv1.PausedAnnotation] = "true"
	b.o.SetAnnotations(annotations)
	return b
}

func (b *KubeadmControlPlaneBuilder) WithDeletionTimestamp() *KubeadmControlPlaneBuilder {
	// Generate a timestamp 10 seconds in the past.
	time := metav1.NewTime(time.Now().Add(time.Duration(-10) * time.Second))
	b.o.ObjectMeta.SetDeletionTimestamp(&time)
	return b
}

func (b *KubeadmControlPlaneBuilder) WithStatusProvisioned() *KubeadmControlPlaneBuilder {
	b.o.Status.Ready = true
	return b
}

func (b *KubeadmControlPlaneBuilder) Build() *kcpv1.KubeadmControlPlane {
	return b.o
}
