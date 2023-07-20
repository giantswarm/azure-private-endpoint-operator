package testhelpers

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"go.uber.org/mock/gomock"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/azure/mock_azure"
)

func SetupPrivateEndpointClientWithoutPrivateIp(
	privateEndpointClient *mock_azure.MockPrivateEndpointsClient,
	mcResourceGroup string,
	expectedPrivateEndpointName string) {
	SetupPrivateEndpointClientToReturnPrivateIp(privateEndpointClient, mcResourceGroup, expectedPrivateEndpointName, "")
}

func SetupPrivateEndpointClientToReturnPrivateIp(
	privateEndpointClient *mock_azure.MockPrivateEndpointsClient,
	mcResourceGroup string,
	expectedPrivateEndpointName string,
	expectedPrivateIpString string) {

	var ipConfigurations []*armnetwork.InterfaceIPConfiguration

	if expectedPrivateIpString != "" {
		ipConfigurations = []*armnetwork.InterfaceIPConfiguration{
			{
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
					PrivateIPAddress: to.Ptr(expectedPrivateIpString),
				},
			},
		}
	}

	privateEndpointClient.
		EXPECT().
		Get(
			gomock.Any(),
			gomock.Eq(mcResourceGroup),
			gomock.Eq(expectedPrivateEndpointName),
			gomock.Eq(&armnetwork.PrivateEndpointsClientGetOptions{
				Expand: to.Ptr[string]("NetworkInterfaces"),
			})).
		Return(armnetwork.PrivateEndpointsClientGetResponse{
			PrivateEndpoint: armnetwork.PrivateEndpoint{
				Properties: &armnetwork.PrivateEndpointProperties{
					NetworkInterfaces: []*armnetwork.Interface{
						{
							Properties: &armnetwork.InterfacePropertiesFormat{
								IPConfigurations: ipConfigurations,
							},
						},
					},
				},
			},
		}, nil)
}

func SetupPrivateEndpointClientToReturnNotFound(
	privateEndpointClient *mock_azure.MockPrivateEndpointsClient,
	mcResourceGroup string,
	expectedPrivateEndpointName string) {

	privateEndpointClient.
		EXPECT().
		Get(
			gomock.Any(),
			gomock.Eq(mcResourceGroup),
			gomock.Eq(expectedPrivateEndpointName),
			gomock.Eq(&armnetwork.PrivateEndpointsClientGetOptions{
				Expand: to.Ptr[string]("NetworkInterfaces"),
			})).
		Return(armnetwork.PrivateEndpointsClientGetResponse{}, &azcore.ResponseError{
			StatusCode: http.StatusNotFound,
		})
}
