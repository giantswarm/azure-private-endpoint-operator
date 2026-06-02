package testhelpers

import (
	"context"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateObjects ensures that the given Kubernetes objects are created using the client.
func CreateObjects(ctx context.Context, client client.Client, objs ...client.Object) {
	for _, obj := range objs {
		Expect(client.Create(ctx, obj)).To(Succeed())
	}
}
