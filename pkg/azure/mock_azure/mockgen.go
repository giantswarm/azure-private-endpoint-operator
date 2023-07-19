package mock_azure

// Run go generate to regenerate this mock.
//
//go:generate ../../../bin/mockgen -destination privateendpoints_mock.go -package mock_azure -source ../privateendpoints.go PrivateEndpointsClient -imports armnetwork=github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2
