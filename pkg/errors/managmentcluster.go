package errors

import (
	"github.com/giantswarm/microerror"
)

var SubscriptionCannotConnectToPrivateLinkError = &microerror.Error{
	Kind: "SubscriptionCannotConnectToPrivateLinkError",
}

// IsSubscriptionCannotConnectToPrivateLinkError asserts SubscriptionCannotConnectToPrivateLinkError.
func IsSubscriptionCannotConnectToPrivateLinkError(err error) bool {
	return microerror.Cause(err) == SubscriptionCannotConnectToPrivateLinkError
}
