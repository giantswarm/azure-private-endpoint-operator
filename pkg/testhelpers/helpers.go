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
		// Cluster-scoped objects will not have a namespace set, but client.Create _requires_
		// that a namespace is set.
		// if obj.GetNamespace() == "" {
		// 	obj.SetNamespace("default")
		// }

		Expect(client.Create(ctx, obj)).To(Succeed(), fmt.Sprintf("%s/%s, gvk: %s",
			obj.GetNamespace(), obj.GetName(),
			obj.GetObjectKind().GroupVersionKind().String()),
		)
	}
}
