package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/azure-private-endpoint-operator/controllers"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
)

var _ = Describe("AzureClusterReconciler", func() {
	Describe("creating reconciler", func() {
		var client client.Client
		var privateEndpointsClientCreator azure.PrivateEndpointsClientCreator
		var managementClusterName types.NamespacedName
		var reconciler *controllers.AzureClusterReconciler

		BeforeEach(func() {
			client = fake.NewClientBuilder().Build()
			privateEndpointsClientCreator = azure.NewPrivateEndpointClient
			managementClusterName = types.NamespacedName{
				Namespace: "org-giantswarm",
				Name:      "giant",
			}
		})

		It("creates reconciler", func(ctx context.Context) {
			var err error
			reconciler, err = controllers.NewAzureClusterReconciler(client, privateEndpointsClientCreator, managementClusterName)
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
			_, err = controllers.NewAzureClusterReconciler(client, nil, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when MC name is empty", func(ctx context.Context) {
			var err error
			managementClusterName.Name = ""
			_, err = controllers.NewAzureClusterReconciler(client, privateEndpointsClientCreator, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})

		It("fails to create reconciler when MC namespace is empty", func(ctx context.Context) {
			var err error
			managementClusterName.Namespace = ""
			_, err = controllers.NewAzureClusterReconciler(client, privateEndpointsClientCreator, managementClusterName)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalidConfig(err)).To(BeTrue())
		})
	})
})
