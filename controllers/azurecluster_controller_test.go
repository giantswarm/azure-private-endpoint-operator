package controllers_test

import (
	"context"
	"fmt"
	"time"

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
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privatelinks"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

const (
	testPrivateLinkNameForWcAPI       = "super-private-link"
	testPrivateEndpointIpForWcAPI     = "10.10.10.10"
	testPrivateLinkNameForMcIngress   = "giant-ingress-privatelink"
	testPrivateEndpointIpForMcIngress = "10.10.10.11"
)

var _ = Describe("AzureClusterReconciler", func() {
	var subscriptionID string
	var location string
	var managementClusterName string
	var managementClusterNamespacedName types.NamespacedName
	var managementAzureCluster *capz.AzureCluster
	var workloadClusterName string
	var workloadClusterNamespacedName types.NamespacedName
	var workloadClusterRequest ctrl.Request
	var workloadAzureCluster *capz.AzureCluster
	var k8sClient client.Client
	var privateEndpointsClientCreator azure.PrivateEndpointsClientCreator
	var reconciler *controllers.AzureClusterReconciler

	BeforeEach(func() {
		subscriptionID = "1234"
		location = "westeurope"

		managementClusterName = "giant"
		managementClusterNamespacedName = types.NamespacedName{
			Namespace: "org-giantswarm",
			Name:      managementClusterName,
		}
		managementAzureCluster = nil
		workloadClusterName = "awesome-wc"
		workloadClusterNamespacedName = types.NamespacedName{
			Namespace: "org-giantswarm",
			Name:      workloadClusterName,
		}
		workloadClusterRequest = ctrl.Request{
			NamespacedName: workloadClusterNamespacedName,
		}
		workloadAzureCluster = nil

		k8sClient = nil
		privateEndpointsClientCreator = func(context.Context, client.Client, *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
			gomockController := gomock.NewController(GinkgoT())
			return mock_azure.NewMockPrivateEndpointsClient(gomockController), nil
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
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(reconciler).NotTo(BeNil())
		})

		It("fails to create reconciler when client is nil", func(ctx context.Context) {
			var err error
			_, err = controllers.NewAzureClusterReconciler(nil, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when private endpoints creator is nil", func(ctx context.Context) {
			var err error
			_, err = controllers.NewAzureClusterReconciler(k8sClient, nil, managementClusterNamespacedName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when MC name is empty", func(ctx context.Context) {
			var err error
			managementClusterNamespacedName.Name = ""
			_, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when MC namespace is empty", func(ctx context.Context) {
			var err error
			managementClusterNamespacedName.Namespace = ""
			_, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})
	})

	Describe("checking errors before reconciling AzureCluster", func() {
		When("workload AzureCluster resources does not exist", func() {
			JustBeforeEach(func() {
				var err error
				reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
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

		When("the cluster is the MC", func() {
			BeforeEach(func() {
				workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterName).
					WithAPILoadBalancerType(capz.Internal).
					Build()
			})

			JustBeforeEach(func() {
				var err error
				reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns empty result without errors", func(ctx context.Context) {
				request := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "org-giantswarm",
						Name:      managementClusterName,
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

				managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterName).
					WithAPILoadBalancerType(capz.Public).
					Build()
			})

			JustBeforeEach(func() {
				var err error
				reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
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
				reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
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
		var expectedPrivateEndpointIp string
		var privateEndpointGetCallCounter int

		BeforeEach(func() {
			// MC AzureCluster resource (without private endpoints, as the WC has just been created)
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
				WithLocation(location).
				WithAPILoadBalancerType(capz.Public).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType(capz.Internal).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
					WithAllowedSubscription(subscriptionID).
					WithAutoApprovedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			privateEndpointGetCallCounter = 0

			privateEndpointsClientCreator = func(context.Context, client.Client, *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
				gomockController := gomock.NewController(GinkgoT())
				privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
				expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI)
				expectedPrivateEndpointIp = testPrivateEndpointIpForWcAPI
				testhelpers.SetupPrivateEndpointClientToReturnNotFoundAndThenPrivateEndpointWithPrivateIp(
					privateEndpointsClient,
					managementClusterNamespacedName.Name,
					expectedPrivateEndpointName,
					expectedPrivateEndpointIp,
					&privateEndpointGetCallCounter)

				return privateEndpointsClient, nil
			}
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets a finalizer on workload AzureCluster resource", func(ctx context.Context) {
			err := k8sClient.Get(ctx, workloadClusterNamespacedName, workloadAzureCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(workloadAzureCluster.Finalizers).To(HaveLen(0))

			_, err = reconciler.Reconcile(ctx, workloadClusterRequest)
			Expect(err).NotTo(HaveOccurred())

			// get updated workload AzureCluster
			err = k8sClient.Get(ctx, workloadClusterNamespacedName, workloadAzureCluster)
			Expect(err).NotTo(HaveOccurred())

			// finalizer has been set
			Expect(workloadAzureCluster.Finalizers).To(HaveLen(1))
			Expect(workloadAzureCluster.Finalizers[0]).To(Equal(controllers.AzureClusterControllerFinalizer))
		})

		It("creates a new private endpoint for the private link", func(ctx context.Context) {
			// private endpoint does not exist yet
			err := k8sClient.Get(ctx, managementClusterNamespacedName, managementAzureCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(0))

			//
			// first ReconcileMcToWcApi call that does one part of the job
			//
			var result ctrl.Result
			result, err = reconciler.Reconcile(ctx, workloadClusterRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: time.Minute}))

			// get updated management AzureCluster
			err = k8sClient.Get(ctx, managementClusterNamespacedName, managementAzureCluster)
			Expect(err).NotTo(HaveOccurred())

			// done: expected private endpoint has been added to the management AzureCluster
			expectedPrivateEndpoint := testhelpers.NewPrivateEndpointBuilder(fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI)).
				WithLocation(location).
				WithPrivateLinkServiceConnection(subscriptionID, workloadClusterName, testPrivateLinkNameForWcAPI).
				Build()
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// normalize resource before comparison (we don't care about this field here)
			managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0].PrivateLinkServiceConnections[0].RequestMessage = ""
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0]).To(Equal(expectedPrivateEndpoint))

			// still not done: workload AzureCluster still does not have private endpoint IP set,
			// because the private endpoint is still not created on Azure
			err = k8sClient.Get(ctx, workloadClusterNamespacedName, workloadAzureCluster)
			Expect(err).NotTo(HaveOccurred())
			_, ok := workloadAzureCluster.Annotations[privatelinks.AzurePrivateEndpointOperatorApiServerAnnotation]
			Expect(ok).To(BeFalse())

			//
			// second ReconcileMcToWcApi call that finishes the job (since the private endpoint has been
			// fully created now)
			//
			result, err = reconciler.Reconcile(ctx, workloadClusterRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// done: workload AzureCluster now has private endpoint IP set, because the private
			// endpoint has been created
			err = k8sClient.Get(ctx, workloadClusterNamespacedName, workloadAzureCluster)
			Expect(err).NotTo(HaveOccurred())
			privateEndpointIp, ok := workloadAzureCluster.Annotations[privatelinks.AzurePrivateEndpointOperatorApiServerAnnotation]
			Expect(ok).To(BeTrue())
			Expect(privateEndpointIp).To(Equal(expectedPrivateEndpointIp))
		})
	})

	When("management cluster and workload cluster are private clusters", func() {

		BeforeEach(func() {
			// MC AzureCluster resource (without private endpoints, as the WC has just been created)
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
				WithLocation(location).
				WithAPILoadBalancerType(capz.Internal).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType(capz.Internal).
				WithLocation(location).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
					WithAllowedSubscription(subscriptionID).
					WithAutoApprovedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			privateEndpointsClientCreator = func(_ context.Context, _ client.Client, cluster *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
				gomockController := gomock.NewController(GinkgoT())
				privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
				if cluster.Name == managementClusterName {
					testhelpers.SetupPrivateEndpointClientToReturnPrivateIp(
						privateEndpointsClient,
						managementClusterName,
						fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI),
						testPrivateEndpointIpForWcAPI)
				} else {
					testhelpers.SetupPrivateEndpointClientToReturnPrivateIp(
						privateEndpointsClient,
						workloadClusterName,
						fmt.Sprintf("%s-to-%s-privatelink-privateendpoint", workloadClusterName, managementClusterName),
						testPrivateEndpointIpForMcIngress)
				}
				return privateEndpointsClient, nil
			}
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("injects private endpoints to both WC and MC", func(ctx context.Context) {
			var result ctrl.Result
			var err error

			result, err = reconciler.Reconcile(ctx, workloadClusterRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// MC checks

			// get updated management AzureCluster
			err = k8sClient.Get(ctx, managementClusterNamespacedName, managementAzureCluster)
			Expect(err).NotTo(HaveOccurred())

			// done: expected private endpoint has been added to the management AzureCluster
			expectedPrivateEndpoint := testhelpers.NewPrivateEndpointBuilder(fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI)).
				WithLocation(location).
				WithPrivateLinkServiceConnection(subscriptionID, workloadClusterName, testPrivateLinkNameForWcAPI).
				Build()
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// normalize resource before comparison (we don't care about this field here)
			managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0].PrivateLinkServiceConnections[0].RequestMessage = ""
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0]).To(Equal(expectedPrivateEndpoint))

			// wc checks

			// get updated management AzureCluster
			err = k8sClient.Get(ctx, workloadClusterNamespacedName, workloadAzureCluster)
			Expect(err).NotTo(HaveOccurred())

			// done: workload AzureCluster has right annotations
			privateEndpointIpForWcApi, ok := workloadAzureCluster.Annotations[privatelinks.AzurePrivateEndpointOperatorApiServerAnnotation]
			Expect(ok).To(BeTrue())
			Expect(privateEndpointIpForWcApi).To(Equal(testPrivateEndpointIpForWcAPI))

			privateEndpointIpForMcIngress, ok := workloadAzureCluster.Annotations[privatelinks.AzurePrivateEndpointOperatorMcIngressAnnotation]
			Expect(ok).To(BeTrue())
			Expect(privateEndpointIpForMcIngress).To(Equal(testPrivateEndpointIpForMcIngress))

			// done: private endpoint has been added to the WC
			expectedPrivateEndpointInWc := testhelpers.NewPrivateEndpointBuilder(fmt.Sprintf("%s-to-%s-privatelink-privateendpoint", workloadClusterName, managementClusterName)).
				WithLocation(location).
				WithPrivateLinkServiceConnectionWithName(subscriptionID, managementClusterName, testPrivateLinkNameForMcIngress,
					fmt.Sprintf("%s-to-%s-connection", workloadClusterName, managementClusterName)).
				Build()
			Expect(workloadAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// normalize resource before comparison (we don't care about this field here)
			workloadAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0].PrivateLinkServiceConnections[0].RequestMessage = ""
			Expect(workloadAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints[0]).To(Equal(expectedPrivateEndpointInWc))
		})
	})

	When("workload cluster has been deleted", func() {
		BeforeEach(func() {
			// MC AzureCluster resource
			privateEndpoints := capz.PrivateEndpoints{
				testhelpers.NewPrivateEndpointBuilder(fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI)).
					WithLocation(location).
					WithPrivateLinkServiceConnection(subscriptionID, workloadClusterName, testPrivateLinkNameForWcAPI).
					Build(),
			}
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
				WithLocation(location).
				WithAPILoadBalancerType(capz.Public).
				WithSubnet("test-subnet", capz.SubnetNode, privateEndpoints).
				Build()

			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
				WithAPILoadBalancerType(capz.Internal).
				WithSubnet("test-subnet", capz.SubnetNode, privateEndpoints).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
					WithAllowedSubscription(subscriptionID).
					WithAutoApprovedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				WithFinalizer(controllers.AzureClusterControllerFinalizer).
				WithDeletionTimestamp(time.Now()).
				Build()
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes private endpoint from management AzureCluster", func(ctx context.Context) {
			// reconcile deleted workload cluster
			result, err := reconciler.Reconcile(ctx, workloadClusterRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// workload AzureCluster is deleted
			err = k8sClient.Get(ctx, workloadClusterNamespacedName, workloadAzureCluster)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			// workload AzureCluster is deleted
			err = k8sClient.Get(ctx, managementClusterNamespacedName, managementAzureCluster)
			Expect(err).ToNot(HaveOccurred())

			// private endpoint in management cluster is deleted
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(0))
		})
	})

	Describe("scenarios where reconciliation is requeued after a minute", func() {
		var expectedResultRequeueAfterMinute ctrl.Result
		BeforeEach(func() {
			expectedResultRequeueAfterMinute = ctrl.Result{
				RequeueAfter: time.Minute,
			}
		})

		JustBeforeEach(func() {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(k8sClient, privateEndpointsClientCreator, managementClusterNamespacedName)
			Expect(err).NotTo(HaveOccurred())
		})

		// workload AzureCluster has private link, but the private link is still not ready, which
		// happens when the workload cluster has just been created
		When("workload cluster private links are not ready", func() {
			BeforeEach(func() {
				// MC AzureCluster resource (without private endpoints, as the WC has just been created)
				managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
					WithLocation(location).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					WithAPILoadBalancerType(capz.Public).
					Build()

				workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
					WithAPILoadBalancerType(capz.Internal).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
						WithAllowedSubscription(subscriptionID).
						WithAutoApprovedSubscription(subscriptionID).
						Build()).
					// private links conditions is not set, meaning it's treated as Unknown
					Build()
			})

			It("will requeue reconciliation after 1 minute", func(ctx context.Context) {
				result, err := reconciler.Reconcile(ctx, workloadClusterRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedResultRequeueAfterMinute))
			})
		})

		// workload cluster has been created, private links are ready, private endpoint has been
		// added to the management cluster, but CAPZ still hasn't created the private endpoint
		When("private endpoint in MC has not been created yet", func() {
			BeforeEach(func() {
				managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
					WithLocation(location).
					WithAPILoadBalancerType(capz.Public).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					Build()

				workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
					WithAPILoadBalancerType(capz.Internal).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
						WithAllowedSubscription(subscriptionID).
						WithAutoApprovedSubscription(subscriptionID).
						Build()).
					WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
					Build()

				privateEndpointsClientCreator = func(_ context.Context, _ client.Client, cluster *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
					gomockController := gomock.NewController(GinkgoT())
					privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
					if cluster.Name == managementClusterName {
						expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI)
						testhelpers.SetupPrivateEndpointClientToReturnNotFound(
							privateEndpointsClient,
							managementClusterNamespacedName.Name,
							expectedPrivateEndpointName)
					}
					return privateEndpointsClient, nil
				}
			})

			It("will requeue reconciliation after 1 minute", func(ctx context.Context) {
				result, err := reconciler.Reconcile(ctx, workloadClusterRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedResultRequeueAfterMinute))
			})
		})

		// private endpoint has been added to the workload cluster, but CAPZ still hasn't created the private endpoint
		When("private endpoint in WC has not been created yet", func() {
			BeforeEach(func() {
				managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
					WithLocation(location).
					WithAPILoadBalancerType(capz.Internal).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					Build()

				workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
					WithAPILoadBalancerType(capz.Public).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
						WithAllowedSubscription(subscriptionID).
						WithAutoApprovedSubscription(subscriptionID).
						Build()).
					WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
					Build()

				privateEndpointsClientCreator = func(_ context.Context, _ client.Client, cluster *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
					gomockController := gomock.NewController(GinkgoT())
					privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
					if cluster.Name == workloadClusterName {
						expectedPrivateEndpointName := fmt.Sprintf("%s-to-%s-privatelink-privateendpoint", workloadClusterName, managementClusterName)
						testhelpers.SetupPrivateEndpointClientToReturnNotFound(
							privateEndpointsClient,
							workloadClusterName,
							expectedPrivateEndpointName)
					}
					return privateEndpointsClient, nil
				}
			})

			It("will requeue reconciliation after 1 minute", func(ctx context.Context) {
				result, err := reconciler.Reconcile(ctx, workloadClusterRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedResultRequeueAfterMinute))
			})
		})

		// workload cluster has been created, private links are ready, private endpoint has been
		// added to the management cluster, but private endpoint creation is still in progress on
		// Azure
		When("private endpoint in MC doesn't yet have a network interface with private IP", func() {
			BeforeEach(func() {
				managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
					WithLocation(location).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					WithAPILoadBalancerType(capz.Public).
					Build()

				workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
					WithAPILoadBalancerType(capz.Internal).
					WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
						WithAllowedSubscription(subscriptionID).
						WithAutoApprovedSubscription(subscriptionID).
						Build()).
					WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					Build()

				privateEndpointsClientCreator = func(_ context.Context, _ client.Client, cluster *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
					gomockController := gomock.NewController(GinkgoT())
					privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
					if cluster.Name == managementClusterName {
						expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkNameForWcAPI)
						testhelpers.SetupPrivateEndpointClientWithoutPrivateIp(
							privateEndpointsClient,
							managementClusterNamespacedName.Name,
							expectedPrivateEndpointName)
					}

					return privateEndpointsClient, nil
				}
			})

			It("will requeue reconciliation after 1 minute", func(ctx context.Context) {
				result, err := reconciler.Reconcile(ctx, workloadClusterRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedResultRequeueAfterMinute))
			})
		})

		// Private endpoint has been added to the workload cluster,
		// but private endpoint creation is still in progress on Azure
		When("private endpoint in WC doesn't yet have a network interface with private IP", func() {
			BeforeEach(func() {
				managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, managementClusterNamespacedName.Name).
					WithLocation(location).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					WithAPILoadBalancerType(capz.Internal).
					Build()

				workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, workloadClusterName).
					WithAPILoadBalancerType(capz.Public).
					WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkNameForWcAPI).
						WithAllowedSubscription(subscriptionID).
						WithAutoApprovedSubscription(subscriptionID).
						Build()).
					WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
					WithSubnet("test-subnet", capz.SubnetNode, nil).
					Build()

				privateEndpointsClientCreator = func(_ context.Context, _ client.Client, cluster *capz.AzureCluster) (azure.PrivateEndpointsClient, error) {
					gomockController := gomock.NewController(GinkgoT())
					privateEndpointsClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
					if cluster.Name == workloadClusterName {
						expectedPrivateEndpointName := fmt.Sprintf("%s-to-%s-privatelink-privateendpoint", workloadClusterName, managementClusterName)
						testhelpers.SetupPrivateEndpointClientWithoutPrivateIp(
							privateEndpointsClient,
							workloadClusterName,
							expectedPrivateEndpointName)
					}
					return privateEndpointsClient, nil
				}
			})

			It("will requeue reconciliation after 1 minute", func(ctx context.Context) {
				result, err := reconciler.Reconcile(ctx, workloadClusterRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedResultRequeueAfterMinute))
			})
		})
	})
})
