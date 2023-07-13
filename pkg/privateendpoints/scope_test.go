package privateendpoints_test

import (
	"context"

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

var _ = Describe("Scope", func() {
	var err error
	var subscriptionID string
	var resourceGroup string
	var gomockController *gomock.Controller
	var scope privateendpoints.Scope

	BeforeEach(func() {
		subscriptionID = "1234"
		resourceGroup = "test-rg"
		gomockController = gomock.NewController(GinkgoT())
		err = nil
	})

	Describe("creating scope", func() {
		var azureCluster *capz.AzureCluster
		var client client.Client
		var privateEndpointClient azure.PrivateEndpointsClient

		BeforeEach(func() {
			azureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).
				WithSubnet("test-subnet", capz.SubnetNode).
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
		Describe("getting all private endpoints", func() {
			// TBA
		})

		Describe("getting private endpoints to a workload cluster", func() {
			// TBA
		})

		Describe("getting a private endpoint IP address", func() {
			// TBA
		})

		Describe("checking of scope contains a specified private endpoint", func() {
			// TBA
		})
	})

	Describe("adding a private endpoint", func() {
		// TBA
	})

	Describe("removing a private endpoint by name", func() {
		// TBA
	})
})
