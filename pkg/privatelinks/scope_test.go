package privatelinks_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
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

	Describe("getting private links for a management cluster subscription", func() {
		var privateLinksWithAllowedSubscriptions map[string][]string
		var scope *privatelinks.Scope

		JustBeforeEach(func() {
			azureClusterBuilder := testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup)
			for privateLinkName, allowedSubscriptions := range privateLinksWithAllowedSubscriptions {
				privateLinkBuilder := testhelpers.NewPrivateLinkBuilder(privateLinkName)
				for _, allowedSubscription := range allowedSubscriptions {
					privateLinkBuilder = privateLinkBuilder.WithAllowedSubscription(allowedSubscription)
				}
				privateLink := privateLinkBuilder.Build()
				azureClusterBuilder.WithPrivateLink(privateLink)
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

		When("AzureCluster has one private link with allowed MC subscription", func() {
			var privateLinkName string
			BeforeEach(func() {
				privateLinkName = "test-private-link"
				privateLinksWithAllowedSubscriptions = map[string][]string{
					privateLinkName: {
						subscriptionID,
					},
				}
			})

			It("gets one private link for the allowed subscription ID", func() {
				privateLinks := scope.GetPrivateLinksWithAllowedSubscription(subscriptionID)
				Expect(privateLinks).To(HaveLen(1))
				Expect(privateLinks[0].Name).To(Equal(privateLinkName))
				Expect(privateLinks[0].AllowedSubscriptions).To(Equal(privateLinksWithAllowedSubscriptions[privateLinkName]))
			})

			It("doesn't get a private link for the disallowed subscription ID", func() {
				privateLinks := scope.GetPrivateLinksWithAllowedSubscription("some-other-subs")
				Expect(privateLinks).To(BeEmpty())
			})
		})

		When("AzureCluster has multiple private links with MC subscription being allowed in one", func() {
			var privateLinkName string
			BeforeEach(func() {
				privateLinkName = "test-private-link"
				privateLinksWithAllowedSubscriptions = map[string][]string{
					privateLinkName: {
						subscriptionID,
					},
					"some-other-private-link": {
						"some-other-subscription",
					},
				}
			})

			It("gets one private link for the allowed subscription ID", func() {
				privateLinks := scope.GetPrivateLinksWithAllowedSubscription(subscriptionID)
				Expect(privateLinks).To(HaveLen(1))
				Expect(privateLinks[0].Name).To(Equal(privateLinkName))
				Expect(privateLinks[0].AllowedSubscriptions).To(Equal(privateLinksWithAllowedSubscriptions[privateLinkName]))
			})
		})
	})

	Describe("Check if private links are ready", func() {
		var privateLinksCondition *capi.Condition
		var scope *privatelinks.Scope

		JustBeforeEach(func() {
			azureCluster := testhelpers.NewAzureClusterBuilder(subscriptionID, resourceGroup).
				WithPrivateLink(testhelpers.NewPrivateLinkBuilder("test-private-link").Build()).
				WithCondition(privateLinksCondition).
				Build()

			capzSchema, err := capz.SchemeBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().
				WithScheme(capzSchema).
				WithObjects(azureCluster).Build()
			scope, err = privatelinks.NewScope(azureCluster, client)
			Expect(err).NotTo(HaveOccurred())
		})

		When("PrivateLinksReady condition has status True", func() {
			BeforeEach(func() {
				privateLinksCondition = conditions.TrueCondition(capz.PrivateLinksReadyCondition)
			})
			It("scope.PrivateLinksReady returns true", func() {
				privateLinksReady := scope.PrivateLinksReady()
				Expect(privateLinksReady).To(BeTrue())
			})
		})

		When("PrivateLinksReady condition has status False", func() {
			BeforeEach(func() {
				privateLinksCondition = conditions.FalseCondition(capz.PrivateLinksReadyCondition, "Something", capi.ConditionSeverityError, "some error")
			})
			It("scope.PrivateLinksReady returns false", func() {
				privateLinksReady := scope.PrivateLinksReady()
				Expect(privateLinksReady).To(BeFalse())
			})
		})

		When("PrivateLinksReady condition has status Unknown", func() {
			BeforeEach(func() {
				privateLinksCondition = conditions.UnknownCondition(capz.PrivateLinksReadyCondition, "Something", "some error")
			})
			It("scope.PrivateLinksReady returns false", func() {
				privateLinksReady := scope.PrivateLinksReady()
				Expect(privateLinksReady).To(BeFalse())
			})
		})

		When("PrivateLinksReady condition is not set", func() {
			BeforeEach(func() {
				privateLinksCondition = nil
			})
			It("scope.PrivateLinksReady returns false", func() {
				privateLinksReady := scope.PrivateLinksReady()
				Expect(privateLinksReady).To(BeFalse())
			})
		})
	})
})
