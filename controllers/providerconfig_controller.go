package controllers

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaerr "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ProviderConfigControllerFinalizer = "azure.giantswarm.io/providerconfig"
)

var (
	ErrIdentityRefUnset = errors.New("identity ref is not set")
)

func NewProviderConfigReconciler(client client.Client) (*ProviderConfigReconciler, error) {
	if client == nil {
		return nil, errors.New("client may not be nil")
	}

	return &ProviderConfigReconciler{
		client: client,
	}, nil
}

// ProviderConfigReconciler manages Crossplane ProviderConfig resources for Azure and AKS clusters.
type ProviderConfigReconciler struct {
	client client.Client
}

func (r *ProviderConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	logger := log.FromContext(ctx)

	logger.Info("starting reconciliation")
	defer logger.Info("finished reconciliation")

	cluster := new(capi.Cluster)
	err = r.client.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		logger.Error(err, "failed to get cluster")
		return
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster)
	}

	return r.reconcileNormal(ctx, cluster)
}

func (r *ProviderConfigReconciler) reconcileNormal(ctx context.Context, cluster *capi.Cluster) (result reconcile.Result, err error) {
	logger := log.FromContext(ctx)

	var info identityInfo
	identityRef := new(corev1.ObjectReference)
	switch {
	case cluster.Spec.ControlPlaneRef.Kind == capz.AzureManagedControlPlaneKind:
		azureManagedControlPlane := new(capz.AzureManagedControlPlane)
		name := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Spec.ControlPlaneRef.Name,
		}
		err = r.client.Get(ctx, name, azureManagedControlPlane)
		if err != nil {
			logger.Error(err, "failed to get controlplane", "kind", azureManagedControlPlane.GroupVersionKind(), "name", name)
			return
		}
		identityRef = azureManagedControlPlane.Spec.IdentityRef
		info.SubscriptionID = azureManagedControlPlane.Spec.SubscriptionID

	case cluster.Spec.InfrastructureRef.Kind == capz.AzureClusterKind:
		azureCluster := new(capz.AzureCluster)
		name := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Spec.InfrastructureRef.Name,
		}
		err = r.client.Get(ctx, name, azureCluster)
		if err != nil {
			logger.Error(err, "failed to get infracluster", "kind", azureCluster.GroupVersionKind(), "name", name)
			return
		}
		identityRef = azureCluster.Spec.IdentityRef
		info.SubscriptionID = azureCluster.Spec.SubscriptionID

	default:
		logger.Info("skipping provider config generation for unsupported cluster")
		return
	}

	if identityRef == nil {
		logger.Error(ErrIdentityRefUnset, "unable to proceed")
		err = reconcile.TerminalError(ErrIdentityRefUnset)
		return result, err
	}

	switch identityRef.Kind {
	case capz.AzureClusterIdentityKind:
		identity := new(capz.AzureClusterIdentity)
		name := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      identityRef.Name,
		}
		err = r.client.Get(ctx, name, identity)
		if err != nil {
			logger.Error(err, "failed to get identity", "name", name)
			return
		}

		switch identity.Spec.Type {
		case capz.WorkloadIdentity:
			info.Type = identity.Spec.Type
			info.TenantID = identity.Spec.TenantID
			info.ClientID = identity.Spec.ClientID
		default:
			logger.Info("skipping provider config generation for unsupported cluster identity type", "type", identity.Spec.Type)
			return
		}

	default:
		logger.Info("skipping provider config generation for unsupported identity", "kind", identityRef.GroupVersionKind())
		return
	}

	// Cluster has a supported configuration, so we will create a ProviderConfig.
	// Let's add a finalizer to the Cluster so that we can clean up the ProviderConfig
	// when this Cluster gets deleted.
	origCluster := cluster.DeepCopy()
	if controllerutil.AddFinalizer(cluster, ProviderConfigControllerFinalizer) {
		if err = r.client.Patch(ctx, cluster, client.MergeFrom(origCluster)); err != nil {
			return
		}
	}

	providerConfig := NewProviderConfig(cluster.Name)
	_, err = controllerutil.CreateOrPatch(ctx, r.client, providerConfig, func() error {
		providerConfig.Object["spec"] = map[string]any{
			"credentials": map[string]any{
				"source": "UserAssignedManagedIdentity",
			},
			"clientID":       info.ClientID,
			"subscriptionID": info.SubscriptionID,
			"tenantID":       info.TenantID,
		}
		return nil
	})

	return
}

func (r *ProviderConfigReconciler) reconcileDelete(ctx context.Context, cluster *capi.Cluster) (result reconcile.Result, err error) {
	providerConfig := NewProviderConfig(cluster.Name)
	err = r.client.Delete(ctx, providerConfig)
	if err != nil {
		if !apierrors.IsNotFound(err) &&
			!metaerr.IsNoMatchError(err) {
			return
		}
		err = nil
	}

	origCluster := cluster.DeepCopy()
	if controllerutil.RemoveFinalizer(cluster, ProviderConfigControllerFinalizer) {
		if err = r.client.Patch(ctx, cluster, client.MergeFrom(origCluster)); err != nil {
			return
		}
	}

	return
}

func (r *ProviderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capi.Cluster{}).
		Complete(r)
}

type identityInfo struct {
	Type           capz.IdentityType
	TenantID       string
	SubscriptionID string
	ClientID       string
}

// NewProviderConfig returns an [unstructured.Unstructured] prepared for use as a ProviderConfig.
func NewProviderConfig(name string) *unstructured.Unstructured {
	providerConfig := new(unstructured.Unstructured)
	providerConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "azure.upbound.io",
		Version: "v1beta1",
		Kind:    "ProviderConfig",
	})
	providerConfig.SetName(name)
	return providerConfig
}
