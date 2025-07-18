// Code generated by MockGen. DO NOT EDIT.
// Source: ../privateendpoints.go
//
// Generated by this command:
//
//	mockgen -destination privateendpoints_mock.go -package mock_azure -source ../privateendpoints.go PrivateEndpointsClient -imports armnetwork=github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2
//

// Package mock_azure is a generated GoMock package.
package mock_azure

import (
	context "context"
	reflect "reflect"

	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	gomock "go.uber.org/mock/gomock"
)

// MockPrivateEndpointsClient is a mock of PrivateEndpointsClient interface.
type MockPrivateEndpointsClient struct {
	ctrl     *gomock.Controller
	recorder *MockPrivateEndpointsClientMockRecorder
}

// MockPrivateEndpointsClientMockRecorder is the mock recorder for MockPrivateEndpointsClient.
type MockPrivateEndpointsClientMockRecorder struct {
	mock *MockPrivateEndpointsClient
}

// NewMockPrivateEndpointsClient creates a new mock instance.
func NewMockPrivateEndpointsClient(ctrl *gomock.Controller) *MockPrivateEndpointsClient {
	mock := &MockPrivateEndpointsClient{ctrl: ctrl}
	mock.recorder = &MockPrivateEndpointsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPrivateEndpointsClient) EXPECT() *MockPrivateEndpointsClientMockRecorder {
	return m.recorder
}

// Get mocks base method.
func (m *MockPrivateEndpointsClient) Get(ctx context.Context, resourceGroupName, privateEndpointName string, options *armnetwork.PrivateEndpointsClientGetOptions) (armnetwork.PrivateEndpointsClientGetResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, resourceGroupName, privateEndpointName, options)
	ret0, _ := ret[0].(armnetwork.PrivateEndpointsClientGetResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockPrivateEndpointsClientMockRecorder) Get(ctx, resourceGroupName, privateEndpointName, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockPrivateEndpointsClient)(nil).Get), ctx, resourceGroupName, privateEndpointName, options)
}
