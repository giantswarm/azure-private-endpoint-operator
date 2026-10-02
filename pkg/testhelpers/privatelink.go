package testhelpers

import (
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type PrivateLinkBuilder struct {
	name                      string
	allowedSubscriptions      []string
	autoApprovedSubscriptions []string
}

func NewPrivateLinkBuilder(name string) *PrivateLinkBuilder {
	return &PrivateLinkBuilder{
		name: name,
	}
}

func (b *PrivateLinkBuilder) WithAllowedSubscription(subscriptionID string) *PrivateLinkBuilder {
	b.allowedSubscriptions = append(b.allowedSubscriptions, subscriptionID)
	return b
}

func (b *PrivateLinkBuilder) WithAutoApprovedSubscription(subscriptionID string) *PrivateLinkBuilder {
	b.autoApprovedSubscriptions = append(b.autoApprovedSubscriptions, subscriptionID)
	return b
}

func (b *PrivateLinkBuilder) Build() capz.PrivateLink {
	privateLink := capz.PrivateLink{
		Name:                      b.name,
		AllowedSubscriptions:      b.allowedSubscriptions,
		AutoApprovedSubscriptions: b.autoApprovedSubscriptions,
	}

	return privateLink
}
