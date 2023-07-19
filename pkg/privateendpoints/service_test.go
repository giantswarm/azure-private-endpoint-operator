package privateendpoints_test

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
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
			err = service.Reconcile(ctx)
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
			err = service.Reconcile(ctx)
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
			setupPrivateEndpointClientWithoutPrivateIp(
				privateEndpointClient,
				mcResourceGroup,
				expectedPrivateEndpointName)

			expectedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, expectedPrivateEndpointName)

			// private endpoint does not exist yet
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeFalse())

			// Reconcile newly created workload cluster
			err = service.Reconcile(ctx)

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
			setupPrivateEndpointClientToReturnPrivateIp(
				privateEndpointClient,
				mcResourceGroup,
				expectedPrivateEndpointName,
				expectedPrivateEndpointIp)

			expectedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, expectedPrivateEndpointName)

			// private endpoint does not exist yet
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeFalse())

			// reconcile newly created workload cluster
			err = service.Reconcile(ctx)
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
			privateEndpoints := capz.PrivateEndpoints{expectedPrivateEndpointSpec(location, subscriptionID, mcResourceGroup, expectedPrivateEndpointName)}
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
			setupPrivateEndpointClientToReturnPrivateIp(
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
			expectedPrivateEndpointName := fmt.Sprintf("%s-privateendpoint", testPrivateLinkName)
			expectedPrivateEndpoint := expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, expectedPrivateEndpointName)

			// private endpoint already exists
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// there is one private endpoint in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))

			// reconcile newly created workload cluster
			err = service.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())

			// same private endpoint still exists
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())

			// and there is still just one private endpoint in the MC AzureCluster
			Expect(managementAzureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints).To(HaveLen(1))
		})
	})
})

func expectedPrivateEndpointSpec(location, subscriptionID, wcResourceGroup, expectedPrivateEndpointName string) capz.PrivateEndpointSpec {
	return capz.PrivateEndpointSpec{
		Name:     expectedPrivateEndpointName,
		Location: location,
		PrivateLinkServiceConnections: []capz.PrivateLinkServiceConnection{
			{
				Name: fmt.Sprintf("%s-connection", testPrivateLinkName),
				PrivateLinkServiceID: fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
					subscriptionID,
					wcResourceGroup,
					testPrivateLinkName),
				RequestMessage: "",
			},
		},
		ManualApproval: false,
	}
}

func setupPrivateEndpointClientWithoutPrivateIp(
	privateEndpointClient *mock_azure.MockPrivateEndpointsClient,
	mcResourceGroup string,
	expectedPrivateEndpointName string) {
	setupPrivateEndpointClientToReturnPrivateIp(privateEndpointClient, mcResourceGroup, expectedPrivateEndpointName, "")
}

func setupPrivateEndpointClientToReturnPrivateIp(
	privateEndpointClient *mock_azure.MockPrivateEndpointsClient,
	mcResourceGroup string,
	expectedPrivateEndpointName string,
	expectedPrivateIpString string) {

	var ipConfigurations []*armnetwork.InterfaceIPConfiguration

	if expectedPrivateIpString != "" {
		ipConfigurations = []*armnetwork.InterfaceIPConfiguration{
			{
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
					PrivateIPAddress: to.Ptr(expectedPrivateIpString),
				},
			},
		}
	}

	privateEndpointClient.
		EXPECT().
		Get(
			gomock.Any(),
			gomock.Eq(mcResourceGroup),
			gomock.Eq(expectedPrivateEndpointName),
			gomock.Eq(&armnetwork.PrivateEndpointsClientGetOptions{
				Expand: to.Ptr[string]("NetworkInterfaces"),
			})).
		Return(armnetwork.PrivateEndpointsClientGetResponse{
			PrivateEndpoint: armnetwork.PrivateEndpoint{
				Properties: &armnetwork.PrivateEndpointProperties{
					NetworkInterfaces: []*armnetwork.Interface{
						{
							Properties: &armnetwork.InterfacePropertiesFormat{
								IPConfigurations: ipConfigurations,
							},
						},
					},
				},
			},
		}, nil)
}
