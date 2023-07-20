package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/controllers"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure/mock_azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

var _ = Describe("AzureClusterReconciler", func() {
	var subscriptionID string
	//var location string
	//var mcResourceGroup string
	var workloadClusterName string
	var managementAzureCluster *capz.AzureCluster
	var workloadAzureCluster *capz.AzureCluster
	var k8sClient client.Client
	var privateEndpointsClientCreator azure.PrivateEndpointsClientCreator
	var managementClusterName types.NamespacedName
	var reconciler *controllers.AzureClusterReconciler

	BeforeEach(func() {
		subscriptionID = "1234"
		//location = "westeurope"
		//mcResourceGroup = "test-mc-rg"
		workloadClusterName = "test-wc"

		privateEndpointsClientCreator = func(context.Context, client.Client, *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
			gomockController := gomock.NewController(GinkgoT())
			return mock_azure.NewMockPrivateEndpointsClient(gomockController), nil
		}
		managementClusterName = types.NamespacedName{
			Namespace: "org-giantswarm",
			Name:      "giant",
		}
	})

	JustBeforeEach(func() {
		capzSchema, err := capz.SchemeBuilder.Build()
		Expect(err).NotTo(HaveOccurred())

		var objects []client.Object
		if managementAzureCluster != nil {
			objects = append(objects, managementAzureCluster)
		}
		if workloadAzureCluster != nil {
			objects = append(objects, workloadAzureCluster)
		}
		k8sClientBuilder := fake.NewClientBuilder().WithScheme(capzSchema)
		if len(objects) > 0 {
			k8sClientBuilder.WithObjects(objects...)
		}
		k8sClient = k8sClientBuilder.Build()
	})

	Describe("creating reconciler", func() {
		It("creates reconciler", func(ctx context.Context) {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).NotTo(HaveOccurred())
			Expect(reconciler).NotTo(BeNil())
		})

		It("fails to create reconciler when client is nil", func(ctx context.Context) {
			var err error
			_, err = controllers.NewAzureClusterReconciler(nil, privateEndpointsClientCreator, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when private endpoints creator is nil", func(ctx context.Context) {
			var err error
			_, err = controllers.NewAzureClusterReconciler(k8sClient, nil, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when MC name is empty", func(ctx context.Context) {
			var err error
			managementClusterName.Name = ""
			_, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when MC namespace is empty", func(ctx context.Context) {
			var err error
			managementClusterName.Namespace = ""
			_, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})
	})

	When("workload AzureCluster resources does not exist", func() {
		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns empty result without errors", func(ctx context.Context) {
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "org-giantswarm",
					Name:      "ghost",
				},
			}
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	When("workload cluster has a public load balancer", func() {
		BeforeEach(func() {
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType(capz.Public).
				Build()
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns empty result without errors", func(ctx context.Context) {
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "org-giantswarm",
					Name:      workloadClusterName,
				},
			}
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	When("workload cluster has an unknown type load balancer", func() {
		BeforeEach(func() {
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType("SomethingNew").
				Build()
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns UnknownLoadBalancerTypeError", func(ctx context.Context) {
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "org-giantswarm",
					Name:      workloadClusterName,
				},
			}
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsUnknownLoadBalancerType(err)).To(BeTrue())
		})
	})

	When("MC AzureCluster resource is not found (e.g. misconfigured operator)", func() {
		BeforeEach(func() {
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType(capz.Internal).
				Build()
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns not found error", func(ctx context.Context) {
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "org-giantswarm",
					Name:      workloadClusterName,
				},
			}
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})
})
