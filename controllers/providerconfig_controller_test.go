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
		It("creates a ProviderConfig", func(ctx context.Context) {
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

			CreateObjects(ctx, azureClusterIdentity, azureCluster, cluster)

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

		It("returns error when identityRef is unset", func(ctx context.Context) {
			azureCluster := NewAzureClusterBuilder(namespace, "foo").Build()
			cluster := NewClusterBuilder(namespace, "foo").WithAzureCluster(azureCluster).Build()

			CreateObjects(ctx, azureCluster, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			req := Request(namespace, "foo")
			_, err = r.Reconcile(context.Background(), req)
			Expect(err).To(MatchError(controllers.ErrIdentityRefUnset))
		})
	})

	Describe("Reconciling AzureASOManagedCluster", func() {
		It("creates a ProviderConfig", func(ctx context.Context) {
			req := Request(namespace, "foo")

			secret := NewAzureASOCredentialsSecretBuilder(req.Namespace, req.Name).
				WithTenantID("123").
				WithClientID("456").
				WithSubscriptionID("789").
				Build()
			azureAsoControlPlane := NewAzureASOManagedControlPlaneBuilder(req.Namespace, req.Name).
				WithCredentialSecret(secret).
				Build()
			cluster := NewClusterBuilder(req.Namespace, req.Name).
				WithAzureASOManagedControlPlane(azureAsoControlPlane).
				Build()

			want := controllers.NewProviderConfig(req.Name)
			want.Object["spec"] = map[string]any{
				"credentials": map[string]any{
					"source": "UserAssignedManagedIdentity",
				},
				"clientID":       secret.StringData["AZURE_CLIENT_ID"],
				"subscriptionID": secret.StringData["AZURE_SUBSCRIPTION_ID"],
				"tenantID":       secret.StringData["AZURE_TENANT_ID"],
			}

			CreateObjects(ctx, secret, azureAsoControlPlane, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			got := controllers.NewProviderConfig(req.Name)
			GetObjects(ctx, got)
			Expect(got).To(EqualObject(want, IgnorePaths{
				"metadata.creationTimestamp",
				"metadata.generation",
				"metadata.managedFields",
				"metadata.resourceVersion",
				"metadata.uid",
			}))
		})
	})

	Describe("Reconciling unsupported configuration", func() {
		It("does not add a finalizer", func(ctx context.Context) {
			name := "unsupported-cluster"
			req := Request(namespace, name)

			cluster := NewClusterBuilder(namespace, name).WithDummyReferences().Build()
			CreateObjects(ctx, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			GetObjects(ctx, cluster)
			Expect(cluster.Finalizers).ToNot(ContainElement(controllers.ProviderConfigControllerFinalizer))
		})
	})

	Describe("Deleting Cluster", func() {
		It("does not error reconciling deleted cluster", func(ctx context.Context) {
			name := "deleted-cluster"
			req := Request(namespace, name)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())
		})

		It("deletes ProviderConfig", func(ctx context.Context) {
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

			CreateObjects(ctx, providerConfig, azureClusterIdentity, azureCluster, cluster)

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			// Reconcile a first time to apply the finalizer.
			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			// Cluster should be in deleting state, and the reconciler should pick that up.
			DeleteObjects(ctx, cluster)

			// Reconciler should detect the Cluster is deleting, and remove the ProviderConfig.
			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			Eventually(Get(providerConfig)).ShouldNot(Succeed())
		})

		It("removes finalizer if ProviderConfig does not exist", func(ctx context.Context) {
			name := "deleting-cluster"
			req := Request(namespace, name)

			cluster := NewClusterBuilder(namespace, req.Name).
				WithFinalizers(controllers.ProviderConfigControllerFinalizer).
				WithDummyReferences().
				Build()

			CreateObjects(ctx, cluster)
			// Put Cluster into Deleting state. The Finalizer will prevent it from being deleted.
			DeleteObjects(ctx, cluster)
			// Ensure that the Cluster is actually present, because we will later assert that it is not.
			Eventually(Get(cluster)).Should(Succeed())

			r, err := controllers.NewProviderConfigReconciler(k8sClient)
			Expect(err).To(BeNil())

			_, err = r.Reconcile(ctx, req)
			Expect(err).To(BeNil())

			// The Cluster should be gone now, indicating that the finalizer was removed.
			Eventually(Get(cluster)).ShouldNot(Succeed())
		})
	})
})
