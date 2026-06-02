package testhelpers

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme *runtime.Scheme

// SetScheme configures the Kubernetes API scheme used by the testhelpers package.
func SetScheme(s *runtime.Scheme) {
	scheme = s
}

// CreateObjects ensures that the given Kubernetes objects are created using the client.
func CreateObjects(ctx context.Context, client client.Client, objs ...client.Object) {
	for _, obj := range objs {
		Expect(client.Create(ctx, obj)).To(Succeed(), fmt.Sprintf("%s/%s, gvk: %s",
			obj.GetNamespace(), obj.GetName(),
			obj.GetObjectKind().GroupVersionKind().String()),
		)
	}
}

// DeleteObjects ensures that the given Kubernetes objects are deleted using the client.
func DeleteObjects(ctx context.Context, client client.Client, objs ...client.Object) {
	for _, obj := range objs {
		Expect(client.Delete(ctx, obj)).To(Succeed())
	}
}
