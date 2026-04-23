package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/controllers"
	. "github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

var _ = Describe("KubeadmControlPlaneReconciler", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		utilruntime.Must(kcpv1.AddToScheme(scheme))
		utilruntime.Must(capi.AddToScheme(scheme))
		utilruntime.Must(capz.AddToScheme(scheme))
	})

	Describe("Constructor", func() {
		It("creates reconciler", func() {
			client := fake.NewClientBuilder().Build()
			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(reconciler).NotTo(BeNil())
		})

		It("it fails to create a reconciler when the client is nil", func() {
			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(nil, nil)
			Expect(err).Should(HaveOccurred())
			Expect(reconciler).To(BeNil())
		})
	})

	Describe("PreflightChecks", func() {
		// These tests don't rely on internal state.
		reconciler := new(controllers.KubeadmControlPlaneReconciler)
		namespace, name := "default", "test"

		Describe("ControlPlane", func() {
			It("cancels when the control plane is being deleted", func(ctx context.Context) {
				kcp := NewKubeadmControlPlaneBuilder(namespace, name).
					WithDeletionTimestamp().
					Build()

				err := reconciler.PreflightCheckControlPlane(ctx, kcp)
				Expect(err).To(MatchError(controllers.ErrReasonControlPlaneDeleting))
			})

			It("cancels when the control plane is already provisioned", func(ctx context.Context) {
				kcp := NewKubeadmControlPlaneBuilder(namespace, name).
					WithStatusProvisioned().
					Build()

				err := reconciler.PreflightCheckControlPlane(ctx, kcp)
				Expect(err).To(MatchError(controllers.ErrReasonControlPlaneProvisioned))
			})

			It("cancels when the control plane does not yet have an owning cluster", func(ctx context.Context) {
				kcp := NewKubeadmControlPlaneBuilder(namespace, name).Build()

				err := reconciler.PreflightCheckControlPlane(ctx, kcp)
				Expect(err).To(MatchError(controllers.ErrReasonControlPlaneHasNoOwner))
			})

			It("proceeds when all preflight conditions are met", func(ctx context.Context) {
				kcp := NewKubeadmControlPlaneBuilder(namespace, name).Build()
				_ = NewClusterBuilder(scheme).WithControlPlane(kcp).Build()

				err := reconciler.PreflightCheckControlPlane(ctx, kcp)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Describe("Cluster", func() {
			It("cancels when the cluster is paused", func(ctx context.Context) {
				cluster := NewClusterBuilder(scheme).WithPause().Build()

				err := reconciler.PreflightCheckCluster(ctx, cluster)
				Expect(err).To(MatchError(controllers.ErrReasonClusterPaused))
			})

			It("cancels when the cluster has no infrastructure ref", func(ctx context.Context) {
				cluster := NewClusterBuilder(scheme).Build()

				err := reconciler.PreflightCheckCluster(ctx, cluster)
				Expect(err).To(MatchError(controllers.ErrReasonInfraClusterMissing))
			})

			It("proceeds when all conditions are met", func(ctx context.Context) {
				azureCluster := NewAzureClusterBuilder("", "").Build()
				cluster := NewClusterBuilder(scheme).WithAzureCluster(azureCluster).Build()

				err := reconciler.PreflightCheckCluster(ctx, cluster)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Describe("AzureCluster", func() {
			It("cancels when the AzureCluster is not private", func(ctx context.Context) {
				azureCluster := NewAzureClusterBuilder("", "").WithAPILoadBalancerType(capz.Public).Build()

				err := reconciler.PreflightCheckAzureCluster(ctx, azureCluster)
				Expect(err).To(MatchError(controllers.ErrReasonInfraClusterNotPrivate))
			})

			It("proceeds when all conditions are met", func(ctx context.Context) {
				azureCluster := NewAzureClusterBuilder("", "").WithAPILoadBalancerType(capz.Internal).Build()

				err := reconciler.PreflightCheckAzureCluster(ctx, azureCluster)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("Reconciliation", func() {
		It("pauses the control plane when conditions are unmet", func(ctx context.Context) {
			name, namespace := "test", "org-giantswarm"
			kcp := NewKubeadmControlPlaneBuilder(namespace, name).Build()
			infraCluster := NewAzureClusterBuilder("", name).
				WithAPILoadBalancerType(capz.Internal).
				Build()
			cluster := NewClusterBuilder(scheme).
				WithControlPlane(kcp).
				WithAzureCluster(infraCluster).
				Build()

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(kcp, infraCluster, cluster).
				Build()

			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, &controllers.KubeadmControlPlaneReconcilerOptions{
				AzureClusterGates: []capi.ConditionType{"NotMet"},
			})
			Expect(err).ShouldNot(HaveOccurred())

			request := Request(namespace, name)
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result.Requeue).Should(BeFalse())

			err = client.Get(ctx, request.NamespacedName, kcp)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(kcp.Annotations).To(HaveKey(capi.PausedAnnotation))
		})

		It("unpauses the control plane when all conditions are met", func(ctx context.Context) {
			name, namespace := "test", "org-giantswarm"
			condition := capi.Condition{
				Type:   "YesMet",
				Status: corev1.ConditionTrue,
			}
			kcp := NewKubeadmControlPlaneBuilder(namespace, name).WithPause().Build()
			infraCluster := NewAzureClusterBuilder("", name).
				WithAPILoadBalancerType(capz.Internal).
				WithCondition(&condition).
				Build()
			cluster := NewClusterBuilder(scheme).
				WithControlPlane(kcp).
				WithAzureCluster(infraCluster).
				Build()

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(kcp, infraCluster, cluster).
				Build()

			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, &controllers.KubeadmControlPlaneReconcilerOptions{
				AzureClusterGates: []capi.ConditionType{condition.Type},
			})
			Expect(err).ShouldNot(HaveOccurred())

			request := Request(namespace, name)
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result.Requeue).Should(BeFalse())

			err = client.Get(ctx, request.NamespacedName, kcp)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(kcp.Annotations).To(Not(HaveKey(capi.PausedAnnotation)))
		})
	})
})
