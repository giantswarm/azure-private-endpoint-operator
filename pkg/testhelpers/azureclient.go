package testhelpers

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
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
		Times(1).
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
		Times(1).
		Return(armnetwork.PrivateEndpointsClientGetResponse{}, &azcore.ResponseError{
			StatusCode: http.StatusNotFound,
		})
}

func SetupPrivateEndpointClientToReturnNotFoundAndThenPrivateEndpointWithPrivateIp(
	privateEndpointClient *mock_azure.MockPrivateEndpointsClient,
	mcResourceGroup string,
	expectedPrivateEndpointName string,
	expectedPrivateIpString string,
	callCounter *int) {

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
		MinTimes(1).
		MaxTimes(2).
		DoAndReturn(func(ctx context.Context, resourceGroupName string, privateEndpointName string, options *armnetwork.PrivateEndpointsClientGetOptions) (armnetwork.PrivateEndpointsClientGetResponse, error) {
			if *callCounter == 0 {
				*callCounter++
				return armnetwork.PrivateEndpointsClientGetResponse{}, &azcore.ResponseError{
					StatusCode: http.StatusNotFound,
				}
			} else {
				return armnetwork.PrivateEndpointsClientGetResponse{
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
				}, nil
			}
		})
}
