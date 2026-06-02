package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/giantswarm/azure-private-endpoint-operator/controllers"
	. "github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
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
			cluster := NewClusterBuilder(namespace, "foo").WithAzureCluster(azureCluster).Build()

			want := controllers.NewProviderConfig(req.Name)
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

			got := controllers.NewProviderConfig(req.Name)
			err = k8sClient.Get(ctx, req.NamespacedName, got)
			Expect(err).To(BeNil())
			Expect(got).To(EqualObject(want, IgnorePaths{
				"metadata.creationTimestamp",
				"metadata.generation",
				"metadata.managedFields",
				"metadata.resourceVersion",
				"metadata.uid",
			}))
		})

		It("returns error when identityRef is unset", func() {
			azureCluster := NewAzureClusterBuilder(namespace, "foo").Build()
			cluster := NewClusterBuilder(namespace, "foo").WithAzureCluster(azureCluster).Build()

			CreateObjects(ctx, k8sClient, azureCluster, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			req := Request(namespace, "foo")
			_, err = r.Reconcile(context.Background(), req)
			Expect(err).To(MatchError(controllers.ErrIdentityRefUnset))
		})
	})

	Describe("Deleting Cluster", func() {
		ctx := context.Background()
		It("deletes ProviderConfig", func() {
			name := "deleting-cluster"
			req := Request(namespace, name)

			azureClusterIdentity := NewAzureClusterIdentityBuilder(namespace, "foo").
				WithTenantID("123").
				WithClientID("456").
				Build()
			azureCluster := NewAzureClusterBuilder(namespace, name).
				WithIdentity(azureClusterIdentity).
				Build()
			cluster := NewClusterBuilder(namespace, req.Name).
				WithAzureCluster(azureCluster).
				Build()
			providerConfig := controllers.NewProviderConfig(req.Name)
			providerConfig.Object["spec"] = map[string]any{
				"credentials": map[string]any{
					"source": "UserAssignedManagedIdentity",
				},
				"clientID":       azureClusterIdentity.Spec.ClientID,
				"subscriptionID": azureCluster.Spec.SubscriptionID,
				"tenantID":       azureClusterIdentity.Spec.TenantID,
			}

			CreateObjects(ctx, k8sClient, providerConfig, azureClusterIdentity, azureCluster, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			// Reconcile a first time to apply the finalizer.
			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			// Cluster should be in deleting state, and the reconciler should pick that up.
			DeleteObjects(ctx, k8sClient, cluster)

			// Reconciler should detect the Cluster is deleting, and remove the ProviderConfig.
			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			Eventually(Get(providerConfig)).ShouldNot(Succeed())
		})
	})
})
