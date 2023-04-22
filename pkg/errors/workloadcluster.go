package errors

import (
	"github.com/giantswarm/microerror"
)

var UnknownLoadBalancerTypeError = &microerror.Error{
	Kind: "unknownLoadBalancerTypeError",
}

// IsUnknownLoadBalancerType asserts unknownLoadBalancerTypeError.
func IsUnknownLoadBalancerType(err error) bool {
	return microerror.Cause(err) == UnknownLoadBalancerTypeError
}
