package controllers_test

import (
	"context"

	"github.com/giantswarm/azure-private-endpoint-operator/controllers"
	. "github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("CrossplaneProviderConfigReconciler", func() {
	Describe("Constructor", func() {
		It("creates reconciler", func() {
			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())
			Expect(r).ToNot(BeNil())
		})

		It("fails to create a reconciler when the client is nil", func() {
			r, err := controllers.NewProviderConfigReconciler(nil)
			Expect(err).ToNot(BeNil())
			Expect(r).To(BeNil())
		})
	})

	Describe("Reconciling AzureCluster", func() {
		ctx := context.Background()
		It("creates a ProviderConfig", func() {
			req := Request(namespace, "foo")

			azureClusterIdentity := NewAzureClusterIdentityBuilder(namespace, "foo").
				WithTenantID("123").
				WithClientID("456").
				Build()
			azureCluster := NewAzureClusterBuilder(namespace, "foo").
				WithIdentity(azureClusterIdentity).
				Build()
			cluster := NewClusterBuilder(scheme).WithAzureCluster(azureCluster).Build()

			want := new(unstructured.Unstructured)
			want.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "azure.upbound.io",
				Version: "v1beta1",
				Kind:    "ProviderConfig",
			})
			want.SetName(req.Name)
			want.Object["spec"] = map[string]any{
				"credentials": map[string]any{
					"source": "UserAssignedManagedIdentity",
				},
				"clientID":       azureClusterIdentity.Spec.ClientID,
				"subscriptionID": azureCluster.Spec.SubscriptionID,
				"tenantID":       azureClusterIdentity.Spec.TenantID,
			}

			CreateObjects(ctx, k8sClient, azureClusterIdentity, azureCluster, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			_, err = r.Reconcile(context.Background(), req)
			Expect(err).To(BeNil())

			providerConfig := new(unstructured.Unstructured)
			providerConfig.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "azure.upbound.io",
				Version: "v1beta1",
				Kind:    "ProviderConfig",
			})
			providerConfig.SetName(req.Name)

			// TODO: Also verify that the ProviderConfig is correct.
			err = k8sClient.Get(ctx, req.NamespacedName, providerConfig)
			Expect(err).To(BeNil())
			Expect(providerConfig).To(EqualObject(want, IgnorePaths{
				"metadata.creationTimestamp",
				"metadata.generation",
				"metadata.managedFields",
				"metadata.resourceVersion",
				"metadata.uid",
			}))
		})

		It("returns error when identityRef is unset", func() {
			azureCluster := NewAzureClusterBuilder(namespace, "foo").Build()
			cluster := NewClusterBuilder(scheme).WithAzureCluster(azureCluster).Build()

			CreateObjects(ctx, k8sClient, azureCluster, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			req := Request(namespace, "foo")
			_, err = r.Reconcile(context.Background(), req)
			Expect(err).To(MatchError(controllers.ErrIdentityRefUnset))
		})
	})
})
