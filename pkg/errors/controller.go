package errors

func IsRetriable(err error) bool {
	return IsPrivateLinksNotReady(err) ||
		IsPrivateEndpointNotFound(err) ||
		IsPrivateEndpointNetworkInterfaceNotFound(err) ||
		IsPrivateEndpointNetworkInterfacePrivateAddressNotFound(err)
}
