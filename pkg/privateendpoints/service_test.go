package privateendpoints_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/errors"
	"github.com/giantswarm/azure-private-endpoint-operator/pkg/privateendpoints"
)

var _ = Describe("Service", func() {
	var err error
	var privateLinksScope privateendpoints.PrivateLinksScope
	var privateEndpointsScope privateendpoints.Scope
	var service privateendpoints.Service

	When("workload cluster has a private link", func() {
		It("successfully reconciles by creating a private endpoint for the private link", func(ctx context.Context) {
			err = service.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("there is no private link where MC subscription is allowed", func() {
		It("returns SubscriptionCannotConnectToPrivateLink error", func(ctx context.Context) {
			err = service.Reconcile(ctx)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsSubscriptionCannotConnectToPrivateLinkError(err)).To(BeTrue())
		})
	})

	When("there workload cluster private links are not ready", func() {
		It("returns SubscriptionCannotConnectToPrivateLink error", func(ctx context.Context) {
			err = service.Reconcile(ctx)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsPrivateLinksNotReady(err)).To(BeTrue())
		})
	})
})
