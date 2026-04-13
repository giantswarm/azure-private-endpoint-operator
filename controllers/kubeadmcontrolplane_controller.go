package controllers

import (
	"context"
	"errors"
	"fmt"

	gsutil "github.com/giantswarm/azure-private-endpoint-operator/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	ErrReconcileCancelled            = errors.New("reconciliation cancelled")
	ErrReasonControlPlaneDeleted     = errors.New("control plane has been deleted")
	ErrReasonControlPlaneDeleting    = errors.New("control plane is being deleted")
	ErrReasonControlPlaneProvisioned = errors.New("control plane is provisioned")
	ErrReasonControlPlaneHasNoOwner  = errors.New("control plane does not yet have an owner")
	ErrReasonClusterPaused           = errors.New("owning cluster is paused")
	ErrReasonInfraClusterMissing     = errors.New("owning cluster has no infrastructure ref")
)

type ReconcileError struct {
	Reason string
}

type KubeadmControlPlaneReconcilerOptions struct {
	AzureClusterGates []capi.ConditionType
}

func NewKubeadmControlPlaneReconciler(client client.Client, opts *KubeadmControlPlaneReconcilerOptions) (*KubeadmControlPlaneReconciler, error) {
	if client == nil {
		return nil, errors.New("failed to build reconciler: client is nil")
	}

	r := &KubeadmControlPlaneReconciler{
		client: client,
	}

	if opts != nil {
		r.azureClusterGates = opts.AzureClusterGates
	}

	return r, nil
}

// KubeadmControlPlaneReconciler pauses or unpauses reconciliation of a cluster's KubeadmControlPlaneReconciler
// based on a number of gates. A new KubeadmControlPlane will be automatically paused, and will remain so
// until all gates pass. In practice, we use gates to pause CAPI or CAPZ reconcilers until our own reconcilers
// complete their tasks.
type KubeadmControlPlaneReconciler struct {
	client            client.Client
	azureClusterGates []capi.ConditionType
}

// Reconcile KubeadmControlPlane to ensure that its associated InfraCluster has passed specific conditions.
// As long as these conditions are not met, the KubeadmControlPlane is paused.
func (r *KubeadmControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx).WithValues("controlplane", req.NamespacedName)

	logger.Info("starting reconciliation")
	defer logger.Info("finished reconciliation")

	kcp := new(v1beta1.KubeadmControlPlane)
	err = r.client.Get(ctx, req.NamespacedName, kcp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("control plane has been deleted")
			return result, nil
		}
		return
	}

	if err = r.PreflightCheckControlPlane(ctx, kcp); err != nil {
		if errors.Is(err, ErrReconcileCancelled) {
			logger.Info(err.Error())
			return result, nil
		}
		return
	}

	cluster, err := util.GetOwnerCluster(ctx, r.client, kcp.ObjectMeta)
	if err != nil {
		return
	}

	if err = r.PreflightCheckCluster(ctx, cluster); err != nil {
		if errors.Is(err, ErrReconcileCancelled) {
			logger.Info(err.Error())
			return result, nil
		}
		return
	}

	infraCluster := new(capz.AzureCluster)
	err = r.client.Get(ctx, types.NamespacedName{
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}, infraCluster)
	if err != nil {
		return
	}

	helper, err := patch.NewHelper(kcp, r.client)
	if err != nil {
		return result, err
	}
	defer helper.Patch(ctx, kcp)

	unmet := gsutil.AreStatusConditionsMet(infraCluster.Status.Conditions, r.azureClusterGates)
	if len(unmet) != 0 {
		logger.Info("pausing control plane because infrastructure cluster conditions were not met", "conditions", unmet)
		annotations := kcp.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[capi.PausedAnnotation] = "true"
		kcp.SetAnnotations(annotations)
		return
	}

	logger.Info("unpausing control plane because all infrastructure cluster conditions were met")
	anns := kcp.GetAnnotations()
	delete(anns, capi.PausedAnnotation)
	kcp.SetAnnotations(anns)
	return
}

// PreflightCheckControlPlane asserts that it is safe to proceed reconciling the KubeamControlPlane.
func (r *KubeadmControlPlaneReconciler) PreflightCheckControlPlane(ctx context.Context, kcp *v1beta1.KubeadmControlPlane) error {
	if kcp.Status.Ready {
		return fmt.Errorf("%w: %w", ErrReconcileCancelled, ErrReasonControlPlaneProvisioned)
	}

	// Normally when an object is being deleted, a reconciler goes into a deletion reconcile loop.
	// But as of writing, this reconciler only pauses or unpauses the control plane.
	// It is not involved at all in deletion.
	if !kcp.DeletionTimestamp.IsZero() {
		return fmt.Errorf("%w: %w", ErrReconcileCancelled, ErrReasonControlPlaneDeleting)
	}

	if !controllerutil.HasControllerReference(kcp) {
		return fmt.Errorf("%w: %w", ErrReconcileCancelled, ErrReasonControlPlaneHasNoOwner)
	}

	return nil
}

func (r *KubeadmControlPlaneReconciler) PreflightCheckCluster(ctx context.Context, cluster *capi.Cluster) error {
	// If the Cluster is paused, then we should not, in any circumstance, unpause the control plane.
	if cluster.Spec.Paused {
		return fmt.Errorf("%w: %w", ErrReconcileCancelled, ErrReasonClusterPaused)
	}

	if cluster.Spec.InfrastructureRef == nil {
		return fmt.Errorf("%w: %w", ErrReconcileCancelled, ErrReasonInfraClusterMissing)
	}

	return nil
}

func (r *KubeadmControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.KubeadmControlPlane{}).
		Watches(&capz.AzureCluster{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, ac client.Object) []reconcile.Request {
			logger := mgr.GetLogger()

			infraClusters := new(capz.AzureClusterList)
			if err := mgr.GetClient().List(ctx, infraClusters); err != nil {
				logger.Error(err, "while listing AzureClusters")
				return nil
			}

			reqs := make([]ctrl.Request, 0, len(infraClusters.Items))
			for _, item := range infraClusters.Items {
				cluster, err := util.GetOwnerCluster(ctx, mgr.GetClient(), item.ObjectMeta)
				if err != nil {
					logger.Error(err, "while getting owning cluster", "infracluster", item.Name)
				}

				reqs = append(reqs, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: cluster.Spec.ControlPlaneRef.Namespace,
						Name:      cluster.Spec.ControlPlaneRef.Name,
					},
				})
			}

			return nil
		})).
		Complete(r)
}
