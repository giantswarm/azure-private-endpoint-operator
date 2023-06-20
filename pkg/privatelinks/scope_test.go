package privatelinks_test

import (
	"fmt"

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

	Describe("looking up a private link with its resource ID", func() {
		var privateLinkNames []string
		var scope *privatelinks.Scope

		JustBeforeEach(func() {
			azureClusterBuilder := testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup)
			for _, privateLinkName := range privateLinkNames {
				azureClusterBuilder.WithPrivateLink(testhelpers.NewPrivateLinkBuilder(privateLinkName).Build())
			}
			azureCluster := azureClusterBuilder.Build()
			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
			scope, err = privatelinks.NewScope(azureCluster, client)
			Expect(err).NotTo(HaveOccurred())
		})

		When("AzureCluster has one private link", func() {
			BeforeEach(func() {
				privateLinkNames = []string{
					"test-private-link",
				}
			})

			It("finds the private link when looking up with the correct resource ID", func() {
				privateLinkID := fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
					subscriptionID,
					resourceGroup,
					privateLinkNames[0])
				privateLink, ok := scope.LookupPrivateLink(privateLinkID)
				Expect(ok).To(BeTrue())
				Expect(privateLink.Name).To(Equal(privateLinkNames[0]))
			})

			It("doesn't find the private link when looking up with an incorrect resource ID", func() {
				privateLinkID := fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
					subscriptionID,
					resourceGroup,
					"some-other-private-link")
				_, ok := scope.LookupPrivateLink(privateLinkID)
				Expect(ok).To(BeFalse())
			})
		})

		When("AzureCluster has multiple private links", func() {
			BeforeEach(func() {
				privateLinkNames = []string{
					"test-private-link-1",
					"test-private-link-2",
				}
			})

			It("finds all private links when looking up with the correct resource ID", func() {
				for _, privateLinkName := range privateLinkNames {
					privateLinkID := fmt.Sprintf(
						"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privateLinkServices/%s",
						subscriptionID,
						resourceGroup,
						privateLinkName)
					privateLink, ok := scope.LookupPrivateLink(privateLinkID)
					Expect(ok).To(BeTrue())
					Expect(privateLink.Name).To(Equal(privateLinkName))
				}
			})
		})
	})
})
