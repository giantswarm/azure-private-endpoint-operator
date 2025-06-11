package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PrivateEndpointsClientCreator func(context.Context, client.Client, *capz.AzureCluster) (PrivateEndpointsClient, error)

type PrivateEndpointsClient interface {
	Get(ctx context.Context, resourceGroupName string, privateEndpointName string, options *armnetwork.PrivateEndpointsClientGetOptions) (armnetwork.PrivateEndpointsClientGetResponse, error)
}

const (
	clientSecretKeyName = "clientSecret"
)

func NewPrivateEndpointClient(ctx context.Context, client client.Client, azureCluster *capz.AzureCluster) (PrivateEndpointsClient, error) {
	var cred azcore.TokenCredential
	var err error

	azureClusterIdentity := &capz.AzureClusterIdentity{}
	name := types.NamespacedName{
		Namespace: azureCluster.Spec.IdentityRef.Namespace,
		Name:      azureCluster.Spec.IdentityRef.Name,
	}
	err = client.Get(ctx, name, azureClusterIdentity)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	switch azureClusterIdentity.Spec.Type {
	case capz.UserAssignedMSI:
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(azureClusterIdentity.Spec.ClientID),
		})
		if err != nil {
			return nil, microerror.Mask(err)
		}
	case capz.ManualServicePrincipal:
		clientSecretName := types.NamespacedName{
			Namespace: azureClusterIdentity.Spec.ClientSecret.Namespace,
			Name:      azureClusterIdentity.Spec.ClientSecret.Name,
		}
		secret := &corev1.Secret{}
		err = client.Get(ctx, clientSecretName, secret)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		cred, err = azidentity.NewClientSecretCredential(
			azureClusterIdentity.Spec.TenantID,
			azureClusterIdentity.Spec.ClientID,
			string(secret.Data[clientSecretKeyName]),
			nil)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	case capz.WorkloadIdentity:
		cred, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientID: azureClusterIdentity.Spec.ClientID,
			TenantID: azureClusterIdentity.Spec.TenantID,
		})
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	privateEndpointsClient, err := armnetwork.NewPrivateEndpointsClient(azureCluster.Spec.SubscriptionID, cred, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return privateEndpointsClient, nil
}
