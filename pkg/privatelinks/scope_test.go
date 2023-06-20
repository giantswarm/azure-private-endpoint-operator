package privatelinks_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privatelinks"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

var _ = Describe("Scope", func() {
	var subscriptionID string
	var resourceGroup string

	BeforeEach(func() {
		subscriptionID = "1234"
		resourceGroup = "test-rg"
	})

	Describe("creating scope", func() {
		var azureCluster *capz.AzureCluster
		var client client.Client

		BeforeEach(func() {
			azureCluster = testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).Build()
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client = fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
		})

		When("AzureCluster is not set", func() {
			It("returns an invalid config error", func() {
				_, err := privatelinks.NewScope(nil, client)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalidConfig(err)).To(BeTrue())
			})
		})

		When("kubernetes client is not set", func() {
			It("returns an invalid config error", func() {
				_, err := privatelinks.NewScope(azureCluster, nil)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalidConfig(err)).To(BeTrue())
			})
		})

		When("both AzureCluster and client are set", func() {
			It("creates the scope", func() {
				_, err := privatelinks.NewScope(azureCluster, client)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
