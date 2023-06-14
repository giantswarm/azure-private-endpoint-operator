package errors

func IsRetriable(err error) bool {
	return IsPrivateLinksNotReady(err) ||
		IsPrivateEndpointNetworkInterfacePrivateAddressNotFound(err)
}
