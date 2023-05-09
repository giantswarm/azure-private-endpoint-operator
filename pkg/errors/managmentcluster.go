package errors

import (
	"github.com/giantswarm/microerror"
)

var SubnetsNotSetError = &microerror.Error{
	Kind: "SubnetsNotSetError",
}

// IsSubnetsNotSetError asserts SubnetsNotSetError.
func IsSubnetsNotSetError(err error) bool {
	return microerror.Cause(err) == SubnetsNotSetError
}

var SubscriptionCannotConnectToPrivateLinkError = &microerror.Error{
	Kind: "SubscriptionCannotConnectToPrivateLinkError",
}

// IsSubscriptionCannotConnectToPrivateLinkError asserts SubscriptionCannotConnectToPrivateLinkError.
func IsSubscriptionCannotConnectToPrivateLinkError(err error) bool {
	return microerror.Cause(err) == SubscriptionCannotConnectToPrivateLinkError
}
