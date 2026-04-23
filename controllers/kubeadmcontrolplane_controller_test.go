package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		var client client.WithWatch
		var mcName types.NamespacedName

		BeforeEach(func() {
			client = fake.NewClientBuilder().Build()
			mcName = types.NamespacedName{Namespace: "foo", Name: "bar"}
		})

		It("creates reconciler", func() {
			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, mcName, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(reconciler).NotTo(BeNil())
		})

		It("it fails to create a reconciler when the client is nil", func() {
			client = nil
			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, mcName, nil)
			Expect(err).Should(HaveOccurred())
			Expect(reconciler).To(BeNil())
		})

		It("fails to create reconciler when MC name is empty", func(ctx context.Context) {
			mcName.Name = ""
			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, mcName, nil)
			Expect(err).Should(HaveOccurred())
			Expect(reconciler).To(BeNil())
		})

		It("fails to create reconciler when MC namespace is empty", func(ctx context.Context) {
			mcName.Namespace = ""
			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, mcName, nil)
			Expect(err).Should(HaveOccurred())
			Expect(reconciler).To(BeNil())
		})
	})

	Describe("PreflightChecks", func() {
		// These tests don't rely on internal state.
		reconciler := new(controllers.KubeadmControlPlaneReconciler)
		namespace, name := "default", "test"

		Describe("Management AzureCluster", func() {
			It("cancels when the management cluster is not private", func(ctx context.Context) {
				azureCluster := NewAzureClusterBuilder("", "").WithAPILoadBalancerType(capz.Public).Build()

				err := reconciler.PreflightCheckManagementCluster(ctx, azureCluster)
				Expect(err).To(MatchError(controllers.ErrReasonManagementClusterNotPrivate))
			})

			It("proceeds when all preflight checks pass", func(ctx context.Context) {
				azureCluster := NewAzureClusterBuilder("", "").WithAPILoadBalancerType(capz.Internal).Build()

				err := reconciler.PreflightCheckManagementCluster(ctx, azureCluster)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

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

			It("proceeds when all preflight checks pass", func(ctx context.Context) {
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

			It("proceeds when all preflight checks pass", func(ctx context.Context) {
				azureCluster := NewAzureClusterBuilder("", "").Build()
				cluster := NewClusterBuilder(scheme).WithAzureCluster(azureCluster).Build()

				err := reconciler.PreflightCheckCluster(ctx, cluster)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("Reconciliation", func() {
		It("pauses the control plane when infracluster status conditions are unmet", func(ctx context.Context) {
			name, namespace := "test", "org-giantswarm"
			mcInfraCluster := NewAzureClusterBuilder("", "management-cluster").
				WithAPILoadBalancerType(capz.Internal).
				Build()
			mcName := types.NamespacedName{Namespace: mcInfraCluster.Namespace, Name: mcInfraCluster.Name}
			kcp := NewKubeadmControlPlaneBuilder(namespace, name).Build()
			infraCluster := NewAzureClusterBuilder("", name).
				Build()
			cluster := NewClusterBuilder(scheme).
				WithControlPlane(kcp).
				WithAzureCluster(infraCluster).
				Build()

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(mcInfraCluster, kcp, infraCluster, cluster).
				Build()

			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, mcName, &controllers.KubeadmControlPlaneReconcilerOptions{
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
			mcInfraCluster := NewAzureClusterBuilder("", "management-cluster").
				WithAPILoadBalancerType(capz.Internal).
				Build()
			mcName := types.NamespacedName{Namespace: mcInfraCluster.Namespace, Name: mcInfraCluster.Name}
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
				WithObjects(mcInfraCluster, kcp, infraCluster, cluster).
				Build()

			reconciler, err := controllers.NewKubeadmControlPlaneReconciler(client, mcName, &controllers.KubeadmControlPlaneReconcilerOptions{
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
