package privateendpoints_test

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure/mock_azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privateendpoints"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

const (
	testPrivateEndpointNameSuffix = "test-private-endpoint"
	testPrivateLinkNameSuffix     = "test-private-link"
)

var _ = Describe("Scope", func() {
	var err error
	var subscriptionID string
	var resourceGroup string
	var gomockController *gomock.Controller
	var privateEndpointClient *mock_azure.MockPrivateEndpointsClient
	var scope privateendpoints.Scope
	var privateEndpointNames []string
	var privateEndpointsCount int

	BeforeEach(func() {
		subscriptionID = "1234"
		resourceGroup = "test-rg"
		gomockController = gomock.NewController(GinkgoT())
		err = nil
		privateEndpointNames = []string{
			"test-private-endpoint-0",
			"test-private-endpoint-1",
			"test-private-endpoint-2",
		}
		privateEndpointsCount = len(privateEndpointNames)
	})

	Describe("creating scope", func() {
		var azureCluster *capz.AzureCluster
		var client client.Client
		var privateEndpointClient azure.PrivateEndpointsClient

		BeforeEach(func() {
			azureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).
				WithSubnet("test-subnet", capz.SubnetNode, nil).
				Build()
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client = fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)
		})

		It("creates scope", func(ctx context.Context) {
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope).NotTo(BeNil())
		})

		It("fails to create scope when AzureCluster is nil", func(ctx context.Context) {
			azureCluster = nil
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create scope when Kubernetes client is nil", func(ctx context.Context) {
			client = nil
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create scope when Azure private endpoints client is nil", func(ctx context.Context) {
			privateEndpointClient = nil
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create scope when AzureCluster does not have subnets", func(ctx context.Context) {
			azureCluster.Spec.NetworkSpec.Subnets = capz.Subnets{}
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsSubnetsNotSetError(err)).To(BeTrue())
		})
	})

	Describe("getting private endpoints", func() {
		var azureCluster *capz.AzureCluster

		BeforeEach(func(ctx context.Context) {
			azureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).
				WithSubnet("test-subnet", capz.SubnetNode, fakePrivateEndpoints(subscriptionID, resourceGroup, privateEndpointNames)).
				Build()
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("gets all private endpoints", func() {
			privateEndpoints := scope.GetPrivateEndpoints()
			Expect(len(privateEndpoints)).To(Equal(privateEndpointsCount))
			for i := range privateEndpoints {
				privateEndpoint := privateEndpoints[i]
				Expect(privateEndpoint.Name).To(Equal(privateEndpointNames[i]))
				Expect(privateEndpoint.PrivateLinkServiceConnections).To(HaveLen(1))
				Expect(privateEndpoint.PrivateLinkServiceConnections[0].Name).To(Equal(
					testhelpers.FakePrivateLinkConnectionName(subscriptionID, resourceGroup, privateLinkName(i))))
			}
		})

		It("gets private endpoints to a workload cluster", func() {
			// Add another set of private endpoints to private links in other resource group (other cluster)
			otherResourceGroup := "other" // also cluster name
			azureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints = append(
				azureCluster.Spec.NetworkSpec.Subnets[0].PrivateEndpoints,
				fakePrivateEndpoints(subscriptionID, otherResourceGroup, privateEndpointNames)...)

			// Check getting private endpoints to a workload cluster in "test-rg" resource group (initially added in BeforeEach)
			privateEndpoints := scope.GetPrivateEndpointsToWorkloadCluster(subscriptionID, resourceGroup)
			Expect(len(privateEndpoints)).To(Equal(privateEndpointsCount))
			for i := range privateEndpoints {
				privateEndpoint := privateEndpoints[i]
				Expect(privateEndpoint.Name).To(Equal(privateEndpointNames[i]))
				Expect(privateEndpoint.PrivateLinkServiceConnections).To(HaveLen(1))
				Expect(privateEndpoint.PrivateLinkServiceConnections[0].Name).To(Equal(
					testhelpers.FakePrivateLinkConnectionName(subscriptionID, resourceGroup, privateLinkName(i))))
			}

			// Check getting private endpoints to a workload cluster in "other" resource group (newly added in this spec)
			privateEndpoints = scope.GetPrivateEndpointsToWorkloadCluster(subscriptionID, otherResourceGroup)
			Expect(len(privateEndpoints)).To(Equal(privateEndpointsCount))
			for i := range privateEndpoints {
				privateEndpoint := privateEndpoints[i]
				Expect(privateEndpoint.Name).To(Equal(privateEndpointNames[i]))
				Expect(privateEndpoint.PrivateLinkServiceConnections).To(HaveLen(1))
				Expect(privateEndpoint.PrivateLinkServiceConnections[0].Name).To(Equal(
					testhelpers.FakePrivateLinkConnectionName(subscriptionID, otherResourceGroup, privateLinkName(i))))
			}
		})

		Describe("checking if the scope contains the specified private endpoint", func() {
			It("returns true when the scope contains the specified  private endpoint", func() {
				for i := 0; i < privateEndpointsCount; i++ {
					contains := scope.ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec{
						Name: privateEndpointNames[i],
					})
					Expect(contains).To(BeTrue())
				}
			})
			It("returns false when the scope doesn't contain the specified  private endpoint", func() {
				contains := scope.ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec{
					Name: "some-other-private-endpoint",
				})
				Expect(contains).To(BeFalse())
			})
		})
	})

	Describe("getting private endpoint properties", func() {
		var azureCluster *capz.AzureCluster

		BeforeEach(func(ctx context.Context) {
			azureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).
				WithSubnet("test-subnet", capz.SubnetNode, fakePrivateEndpoints(subscriptionID, resourceGroup, privateEndpointNames)).
				Build()
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
			privateEndpointClient = mock_azure.NewMockPrivateEndpointsClient(gomockController)
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("gets the private endpoint's IP address", func(ctx context.Context) {
			// setup Azure client mock
			privateEndpointName := "test-private-endpoint"
			expectedPrivateIpString := "10.10.10.10"
			privateEndpointClient.
				EXPECT().
				Get(
					gomock.Eq(ctx),
					gomock.Eq(resourceGroup),
					gomock.Eq(privateEndpointName),
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

			// test scope
			privateEndpointIpAddress, err := scope.GetPrivateEndpointIPAddress(ctx, privateEndpointName)
			Expect(err).NotTo(HaveOccurred())
			Expect(privateEndpointIpAddress).To(Equal(net.ParseIP(expectedPrivateIpString)))
		})

		It("gets PrivateEndpointNotFound error when private endpoint does not exist", func(ctx context.Context) {
			// setup Azure client mock
			privateEndpointName := "test-private-endpoint"
			privateEndpointClient.
				EXPECT().
				Get(
					gomock.Eq(ctx),
					gomock.Eq(resourceGroup),
					gomock.Eq(privateEndpointName),
					gomock.Eq(&armnetwork.PrivateEndpointsClientGetOptions{
						Expand: to.Ptr[string]("NetworkInterfaces"),
					})).
				Return(armnetwork.PrivateEndpointsClientGetResponse{}, &azcore.ResponseError{
					StatusCode: http.StatusNotFound,
				})

			// test scope
			_, err := scope.GetPrivateEndpointIPAddress(ctx, privateEndpointName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsPrivateEndpointNotFound(err)).To(BeTrue())
		})

		It("gets PrivateEndpointNetworkInterfaceNotFoundError error when private endpoint does not have network interfaces", func(ctx context.Context) {
			// setup Azure client mock
			privateEndpointName := "test-private-endpoint"
			privateEndpointClient.
				EXPECT().
				Get(
					gomock.Eq(ctx),
					gomock.Eq(resourceGroup),
					gomock.Eq(privateEndpointName),
					gomock.Eq(&armnetwork.PrivateEndpointsClientGetOptions{
						Expand: to.Ptr[string]("NetworkInterfaces"),
					})).
				Return(armnetwork.PrivateEndpointsClientGetResponse{
					PrivateEndpoint: armnetwork.PrivateEndpoint{
						Properties: &armnetwork.PrivateEndpointProperties{
							NetworkInterfaces: []*armnetwork.Interface{},
						},
					},
				}, nil)

			// test scope
			_, err := scope.GetPrivateEndpointIPAddress(ctx, privateEndpointName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsPrivateEndpointNetworkInterfaceNotFound(err)).To(BeTrue())
		})

		It("gets PrivateEndpointNetworkInterfacePrivateAddressNotFoundError error when private endpoint network interfaces does not have a private IP address", func(ctx context.Context) {
			// setup Azure client mock
			privateEndpointName := "test-private-endpoint"
			privateEndpointClient.
				EXPECT().
				Get(
					gomock.Eq(ctx),
					gomock.Eq(resourceGroup),
					gomock.Eq(privateEndpointName),
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
													PublicIPAddress: &armnetwork.PublicIPAddress{},
												},
											},
										},
									},
								},
							},
						},
					},
				}, nil)

			// test scope
			_, err := scope.GetPrivateEndpointIPAddress(ctx, privateEndpointName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsPrivateEndpointNetworkInterfacePrivateAddressNotFound(err)).To(BeTrue())
		})
	})

	Describe("adding and removing private endpoints", func() {
		var azureCluster *capz.AzureCluster

		BeforeEach(func(ctx context.Context) {
			azureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).
				WithSubnet("test-subnet", capz.SubnetNode, fakePrivateEndpoints(subscriptionID, resourceGroup, privateEndpointNames)).
				Build()
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
			privateEndpointClient := mock_azure.NewMockPrivateEndpointsClient(gomockController)
			scope, err = privateendpoints.NewScope(ctx, azureCluster, client, privateEndpointClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds a private endpoint", func() {
			// first check that the private endpoint does not exist in scope
			testPrivateEndpointName := "some-other-private-endpoint"
			contains := scope.ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec{
				Name: testPrivateEndpointName,
			})
			Expect(contains).To(BeFalse())

			// now add new private endpoint
			privateEndpoint := testhelpers.NewPrivateEndpointBuilder(testPrivateEndpointName).
				WithPrivateLinkServiceConnection(subscriptionID, resourceGroup, privateLinkName(0)).
				Build()
			scope.AddPrivateEndpointSpec(privateEndpoint)

			// and test again
			contains = scope.ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec{
				Name: testPrivateEndpointName,
			})
			Expect(contains).To(BeTrue())
		})

		It("removes a private endpoint by name", func() {
			// first check that the private endpoint exists in scope
			testPrivateEndpointName := privateEndpointNames[1]
			contains := scope.ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec{
				Name: testPrivateEndpointName,
			})
			Expect(contains).To(BeTrue())

			// now remove the private endpoint
			scope.RemovePrivateEndpointByName(testPrivateEndpointName)

			// and test again
			contains = scope.ContainsPrivateEndpointSpec(capz.PrivateEndpointSpec{
				Name: testPrivateEndpointName,
			})
			Expect(contains).To(BeFalse())
		})
	})
})

func fakePrivateEndpoints(privateLinkSubscriptionID, privateLinkResourceGroup string, privateEndpointNames []string) capz.PrivateEndpoints {
	var privateEndpoints capz.PrivateEndpoints
	for i, privateEndpointName := range privateEndpointNames {
		privateEndpoint := testhelpers.NewPrivateEndpointBuilder(privateEndpointName).
			WithPrivateLinkServiceConnection(privateLinkSubscriptionID, privateLinkResourceGroup, privateLinkName(i)).
			Build()
		privateEndpoints = append(privateEndpoints, privateEndpoint)
	}

	return privateEndpoints
}

func privateLinkName(index int) string {
	return resourceName(testPrivateLinkNameSuffix, index)
}

func resourceName(suffix string, index int) string {
	return fmt.Sprintf("%s-%d", suffix, index)
}
