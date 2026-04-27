package privateendpoints_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure/mock_azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privateendpoints"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privatelinks"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

const (
	testPrivateLinkName   = "test-private-link"
	testPrivateEndpointIp = "10.10.10.10"
)

var _ = Describe("Service", func() {
	var err error
	var subscriptionID string
	var location string
	var mcResourceGroup string
	var wcResourceGroup string
	var managementAzureCluster *capz.AzureCluster
	var workloadAzureCluster *capz.AzureCluster
	var privateEndpointClient *mock_azure.MockPrivateEndpointsClient
	var privateLinksScope privateendpoints.PrivateLinksScope
	var privateEndpointsScope privateendpoints.Scope
	var service *privateendpoints.Service

	BeforeEach(func() {
		subscriptionID = "1234"
		location = "westeurope"
		mcResourceGroup = "test-mc-rg"
		wcResourceGroup = "test-wc-rg"
	})

	When("there is no private link where MC subscription is allowed", func() {
		BeforeEach(func(ctx context.Context) {
			otherSubscription := "abcd"

			// MC AzureCluster resource (without private endpoints, as the WC has just been created)
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, mcResourceGroup).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			// WC AzureClusterResource
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(otherSubscription, wcResourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(otherSubscription).
					Build()).
				Build()

			// Kubernetes client
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(managementAzureCluster, workloadAzureCluster).
				Build()

			// Azure private endpoints mock client
			gomockController := gomock.NewController(GinkgoT())
			privateEndpointClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)

			// Private endpoints scope
			privateEndpointsScope, err = privateendpoints.NewScope(ctx, managementAzureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())

			// Private links scope
			privateLinksScope, err = privatelinks.NewScope(workloadAzureCluster, client)
			Expect(err).NotTo(HaveOccurred())

			// Private endpoints service
			service, err = privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns SubscriptionCannotConnectToPrivateLink error", func(ctx context.Context) {
			err = service.ReconcileMcToWcApi(ctx)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsSubscriptionCannotConnectToPrivateLinkError(err)).To(BeTrue())
		})
	})

	When("workload cluster with private link has just been created and private links are still not ready", func() {
		BeforeEach(func(ctx context.Context) {
			// MC AzureCluster resource (without private endpoints, as the WC has just been created)
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, mcResourceGroup).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			// WC AzureClusterResource
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, wcResourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(subscriptionID).
					Build()).
				Build()

			// Kubernetes client
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(managementAzureCluster, workloadAzureCluster).
				Build()

			// Azure private endpoints mock client
			gomockController := gomock.NewController(GinkgoT())
			privateEndpointClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)

			// Private endpoints scope
			privateEndpointsScope, err = privateendpoints.NewScope(ctx, managementAzureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())

			// Private links scope
			privateLinksScope, err = privatelinks.NewScope(workloadAzureCluster, client)
			Expect(err).NotTo(HaveOccurred())

			// Private endpoints service
			service, err = privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns PrivateLinksNotReady error", func(ctx context.Context) {
			err = service.ReconcileMcToWcApi(ctx)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsPrivateLinksNotReady(err)).To(BeTrue())
		})
	})

	When("workload cluster with private link has just been created and private links are ready", func() {
		// This the scenario where we have a newly created workload AzureCluster. Here management
		// AzureCluster still does not have corresponding private endpoint, and we test that it
		// will be created successfully.

		BeforeEach(func(ctx context.Context) {
			// MC AzureCluster resource (without private endpoints, as the WC has just been created)
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, mcResourceGroup).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()

			// WC AzureClusterResource
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, wcResourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				Build()

			// Kubernetes client
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(managementAzureCluster, workloadAzureCluster).
				Build()

			// Azure private endpoints mock client
			gomockController := gomock.NewController(GinkgoT())
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)

			// Private endpoints scope
			privateEndpointsScope, err = privateendpoints.NewScope(ctx, managementAzureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())

			// Private links scope
			privateLinksScope, err = privatelinks.NewScope(workloadAzureCluster, client)
			Expect(err).NotTo(HaveOccurred())

			// Private endpoints service
			service, err = privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates a new private endpoint for the private link, but still doesn't know the private endpoint IP", func(ctx context.Context) {
			expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)
			testhelpers.SetupPrivateEndpointClientWithoutPrivateIp(
				privateEndpointClient,
				mcResourceGroup,
				expectedPrivateEndpointName)

			expectedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, testPrivateLinkName)

			// private endpoint does not exist yet
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeFalse())

			// ReconcileMcToWcApi newly created workload cluster
			err = service.ReconcileMcToWcApi(ctx)

			// private endpoint now exists
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// A retriable error has occurred because the private endpoint IP is still not set
			Expect(err).To(HaveOccurred())
			Expect(errors.IsPrivateEndpointNetworkInterfacePrivateAddressNotFound(err))
			Expect(errors.IsRetriable(err))
			// since the private endpoint IP is still not set, then the private endpoint IP annotation is also not set
			_, ok := workloadAzureCluster.Annotations[privatelinks.AzurePrivateEndpointOperatorApiServerAnnotation]
			Expect(ok).To(BeFalse())
		})

		It("creates a new private endpoint for the private link, and sets the private endpoint IP annotation", func(ctx context.Context) {
			expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)
			expectedPrivateEndpointIp := testPrivateEndpointIp
			testhelpers.SetupPrivateEndpointClientToReturnPrivateIp(
				privateEndpointClient,
				mcResourceGroup,
				expectedPrivateEndpointName,
				expectedPrivateEndpointIp)

			expectedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, testPrivateLinkName)

			// private endpoint does not exist yet
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeFalse())

			// reconcile newly created workload cluster
			err = service.ReconcileMcToWcApi(ctx)
			Expect(err).NotTo(HaveOccurred())

			// private endpoint exists
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// since the private endpoint IP is set, then the private endpoint IP annotation is set
			privateEndpointIpAnnotation, ok := workloadAzureCluster.Annotations[privatelinks.AzurePrivateEndpointOperatorApiServerAnnotation]
			Expect(ok).To(BeTrue())
			Expect(privateEndpointIpAnnotation).To(Equal(expectedPrivateEndpointIp))
		})
	})

	When("workload cluster has already been reconciled and private endpoint has already been created", func() {
		BeforeEach(func(ctx context.Context) {
			// MC AzureCluster resource with private endpoints, as the WC has already been reconciled
			expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)
			privateEndpoints := capz.PrivateEndpoints{expectedPrivateEndpointSpec(location, subscriptionID, mcResourceGroup, testPrivateLinkName)}
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, mcResourceGroup).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, privateEndpoints).
				Build()

			// WC AzureClusterResource
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, wcResourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				Build()

			// Kubernetes client
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(managementAzureCluster, workloadAzureCluster).
				Build()

			// Azure private endpoints mock client
			gomockController := gomock.NewController(GinkgoT())
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)
			testhelpers.SetupPrivateEndpointClientToReturnPrivateIp(
				privateEndpointClient,
				mcResourceGroup,
				expectedPrivateEndpointName,
				testPrivateEndpointIp)

			// Private endpoints scope
			privateEndpointsScope, err = privateendpoints.NewScope(ctx, managementAzureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())

			// Private links scope
			privateLinksScope, err = privatelinks.NewScope(workloadAzureCluster, client)
			Expect(err).NotTo(HaveOccurred())

			// Private endpoints service
			service, err = privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("reconciles in an idempotent way, so new private endpoint is not added", func(ctx context.Context) {
			expectedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, testPrivateLinkName)

			// private endpoint already exists
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// there is one private endpoint in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// reconcile newly created workload cluster
			err = service.ReconcileMcToWcApi(ctx)
			Expect(err).NotTo(HaveOccurred())

			// same private endpoint still exists
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// and there is still just one private endpoint in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))
		})
	})

	When("workload cluster private link has been removed", func() {
		var removedPrivateLinkName string

		BeforeEach(func(ctx context.Context) {
			// MC AzureCluster resource with private endpoints, as the WC has already been reconciled
			expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)
			removedPrivateLinkName = "another-private-link"
			privateEndpoints := capz.PrivateEndpoints{
				expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, testPrivateLinkName),
				expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, removedPrivateLinkName),
			}
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, mcResourceGroup).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, privateEndpoints).
				Build()

			// WC AzureClusterResource
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, wcResourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				Build()

			// Kubernetes client
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(managementAzureCluster, workloadAzureCluster).
				Build()

			// Azure private endpoints mock client
			gomockController := gomock.NewController(GinkgoT())
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)
			testhelpers.SetupPrivateEndpointClientToReturnPrivateIp(
				privateEndpointClient,
				mcResourceGroup,
				expectedPrivateEndpointName,
				testPrivateEndpointIp)

			// Private endpoints scope
			privateEndpointsScope, err = privateendpoints.NewScope(ctx, managementAzureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())

			// Private links scope
			privateLinksScope, err = privatelinks.NewScope(workloadAzureCluster, client)
			Expect(err).NotTo(HaveOccurred())

			// Private endpoints service
			service, err = privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes corresponding private endpoint from the management cluster", func(ctx context.Context) {
			removedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, removedPrivateLinkName)
			presentPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, testPrivateLinkName)

			// there are two private endpoints in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(2))

			// both endpoints initially exist
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(removedPrivateEndpoint)
			Expect(exists).To(BeTrue())
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(presentPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// reconcile newly created workload cluster
			err = service.ReconcileMcToWcApi(ctx)
			Expect(err).NotTo(HaveOccurred())

			//// and now there is still just one private endpoint in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// removed private endpoint does not exist anymore
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(removedPrivateEndpoint)
			Expect(exists).To(BeFalse())

			// other private endpoint still exists
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(presentPrivateEndpoint)
			Expect(exists).To(BeTrue())
		})
	})

	When("workload cluster has been deleted", func() {
		BeforeEach(func(ctx context.Context) {
			// MC AzureCluster resource with private endpoints, as the WC has already been reconciled
			privateEndpoints := capz.PrivateEndpoints{expectedPrivateEndpointSpec(location, subscriptionID, mcResourceGroup, testPrivateLinkName)}
			managementAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, mcResourceGroup).
				WithLocation(location).
				WithSubnet("test-subnet", capz.SubnetNode, privateEndpoints).
				Build()

			// WC AzureClusterResource
			workloadAzureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, wcResourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder(testPrivateLinkName).
					WithAllowedSubscription(subscriptionID).
					Build()).
				WithCondition(conditions.TrueCondition(capz.PrivateLinksReadyCondition)).
				Build()

			// Kubernetes client
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(managementAzureCluster, workloadAzureCluster).
				Build()

			// Azure private endpoints mock client
			gomockController := gomock.NewController(GinkgoT())
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)

			// Private endpoints scope
			privateEndpointsScope, err = privateendpoints.NewScope(ctx, managementAzureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())

			// Private links scope
			privateLinksScope, err = privatelinks.NewScope(workloadAzureCluster, client)
			Expect(err).NotTo(HaveOccurred())

			// Private endpoints service
			service, err = privateendpoints.NewService(privateEndpointsScope, privateLinksScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes private endpoint from the management cluster", func(ctx context.Context) {
			removedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, testPrivateLinkName)

			// initially there is one private endpoint in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// and the removed private endpoint exists initially
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(removedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// reconcile newly created workload cluster
			err = service.DeleteMcToWcApi(ctx)
			Expect(err).NotTo(HaveOccurred())

			// and now there are no private endpoints in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(0))

			// so the removed private endpoint does not exist anymore
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(removedPrivateEndpoint)
			Expect(exists).To(BeFalse())
		})
	})
})

func expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, privateLinkName string) capz.PrivateEndpointSpec {
	privateEndpointSpec := testhelpers.NewPrivateEndpointBuilder(fmt.Sprintf("%s-privateendpoint", privateLinkName)).
		WithLocation(location).
		WithPrivateLinkServiceConnection(subscriptionID, wcResourceGroup, privateLinkName).
		Build()

	return privateEndpointSpec
}
