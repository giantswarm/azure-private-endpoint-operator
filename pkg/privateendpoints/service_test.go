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
	testPrivateLinkName = "test-private-link"
)

var _ = Describe("Service", func() {
	var err error
	var subscriptionID string
	var location string
	var mcResourceGroup string
	var wcResourceGroup string
	var managementAzureCluster *capz.AzureCluster
	var workloadAzureCluster *capz.AzureCluster
	var privateLinksScope privateendpoints.PrivateLinksScope
	var privateEndpointsScope privateendpoints.Scope
	var service *privateendpoints.Service

	BeforeEach(func() {
		subscriptionID = "1234"
		location = "westeurope"
		mcResourceGroup = "test-mc-rg"
		wcResourceGroup = "test-wc-rg"
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
			privateEndpointClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
			expectedPrivateEndpointName := fmt.Sprintf("%s-%s", testPrivateLinkName, "privateendpoint")
			expectedPrivateIpString := "10.10.10.10"
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
										IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
											{
												Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
													PrivateIPAddress: to.Ptr(expectedPrivateIpString),
												},
											},
										},
									},
								},
							},
						},
					},
				}, nil)

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

		It("creates a new private endpoint for the private link", func(ctx context.Context) {
			expectedPrivateEndpoint := capz.PrivateEndpointSpec{
				Name:     fmt.Sprintf("%s-privateendpoint", testPrivateLinkName),
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

			// private endpoint does not exist yet
			exists := privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeFalse())

			// Reconcile newly created workload cluster
			err = service.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())

			// private endpoint now exists
			exists = privateEndpointsScope.ContainsPrivateEndpointSpec(expectedPrivateEndpoint)
			Expect(exists).To(BeTrue())
		})
	})

	//When("there is no private link where MC subscription is allowed", func() {
	//	It("returns SubscriptionCannotConnectToPrivateLink error", func(ctx context.Context) {
	//		err = service.Reconcile(ctx)
	//		Expect(err).To(HaveOccurred())
	//		Expect(errors.IsSubscriptionCannotConnectToPrivateLinkError(err)).To(BeTrue())
	//	})
	//})
	//
})
