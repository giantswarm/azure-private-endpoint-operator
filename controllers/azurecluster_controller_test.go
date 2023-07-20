package controllers_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/controllers"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure/mock_azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

const (
	testPrivateLinkName   = "super-private-link"
	testPrivateEndpointIp = "10.10.10.10"
)

var _ = Describe("AzureClusterReconciler", func() {
	var subscriptionID string
	var location string
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
		workloadClusterName = "awesome-wc"

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

	Describe("checking errors before reconciling AzureCluster", func() {
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

	When("workload cluster with private link has just been created and private links are ready", func() {
		BeforeEach(func() {
			// MC AzureCluster resource (without private endpoints, as the WC has just been created)
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterName.Name).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType(capz.Internal).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(subscriptionID).
					WithAutoApprovedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				Build()

			privateEndpointsClientCreator = func(context.Context, client.Client, *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
				gomockController := gomock.NewController(GinkgoT())
				privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
				expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)
				expectedPrivateEndpointIp := testPrivateEndpointIp
				testhelpers.SetupPrivateEndpointClientToReturnPrivateIp(
					privateEndpointsClient,
					managementClusterName.Name,
					expectedPrivateEndpointName,
					expectedPrivateEndpointIp)

				return privateEndpointsClient, nil
			}
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates a new private endpoint for the private link", func(ctx context.Context) {
			// private endpoint does not exist yet
			err := k8sClient.Get(ctx, managementClusterName, managementAzureCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(0))

			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "org-giantswarm",
					Name:      workloadClusterName,
				},
			}
			var result ctrl.Result
			result, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// get updated management AzureCluster
			err = k8sClient.Get(ctx, managementClusterName, managementAzureCluster)
			Expect(err).NotTo(HaveOccurred())

			// expected private endpoint has been created
			expectedPrivateEndpoint := testhelpers.NewPrivateEndpointBuilder(fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)).
				WithLocation(location).
				WithPrivateLinkServiceConnection(subscriptionID, workloadClusterName, testPrivateLinkName).
				Build()
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// normalize resource before comparison (we don't care about this field here)
			managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0].PrivateLinkServiceConnections[0].RequestMessage = ""
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0]).To(Equal(expectedPrivateEndpoint))
		})
	})
})
