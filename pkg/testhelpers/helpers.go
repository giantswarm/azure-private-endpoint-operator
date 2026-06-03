package testhelpers

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega" //nolint: staticcheck
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme    *runtime.Scheme
	k8sClient client.Client
)

// SetHelperScheme configures the Kubernetes API scheme used by the testhelpers package.
func SetHelperScheme(s *runtime.Scheme) {
	scheme = s
}

func ensureSchemeSet() {
	if scheme == nil {
		panic("attempted to use nil Kubernetes client scheme; " +
			"ensure testhelpers.SetHelperScheme() is called with a non-nil scheme")
	}
}

// SetClient configuures the Kubernetes client used by the testhelpers package.
func SetHelperClient(c client.Client) {
	k8sClient = c
}

func ensureClientSet() {
	if k8sClient == nil {
		panic("attempted to use nil Kubernetes client; " +
			"ensure testhelpers.SetHelperClient() is called with a non-nil client")
	}
}

// GetObjects retrieves the given objects.
func GetObjects(ctx context.Context, objs ...client.Object) {
	ensureClientSet()

	for _, obj := range objs {
		key := types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}
		Expect(k8sClient.Get(ctx, key, obj)).To(Succeed(),
			fmt.Sprintf("%s/%s, gvk: %s",
				obj.GetNamespace(), obj.GetName(),
				obj.GetObjectKind().GroupVersionKind().String()),
		)
	}
}

// CreateObjectsWithClient ensures that the given Kubernetes objects are created.
func CreateObjects(ctx context.Context, objs ...client.Object) {
	ensureClientSet()

	for _, obj := range objs {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed(),
			fmt.Sprintf("%s/%s, gvk: %s",
				obj.GetNamespace(), obj.GetName(),
				obj.GetObjectKind().GroupVersionKind().String()),
		)
	}
}

// DeleteObjectsWithClient ensures that the given Kubernetes objects are deleted.
func DeleteObjects(ctx context.Context, objs ...client.Object) {
	ensureClientSet()

	for _, obj := range objs {
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
	}
}
