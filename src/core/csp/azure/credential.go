/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

import (
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// credentialCache stores ClientSecretCredential objects keyed by "tenantID|clientID".
// Reusing the same object is critical: the Azure SDK caches the OAuth token internally,
// so reuse means Azure AD is called only once per token lifetime (~1 hour) instead of
// on every API call.
var credentialCache sync.Map // key: "tenantID|clientID" → *azidentity.ClientSecretCredential

// vmClientCache stores VirtualMachinesClient objects keyed by subscriptionID.
// Reusing the client also reuses the underlying HTTP transport connection pool.
var vmClientCache sync.Map // key: subscriptionID → *armcompute.VirtualMachinesClient

// tagsClientCache stores TagsClient objects keyed by subscriptionID.
var tagsClientCache sync.Map // key: subscriptionID → *armresources.TagsClient

// getOrCreateCredential returns a cached ClientSecretCredential for the given Azure
// credentials, creating one if it does not already exist.
//
// Caching is essential for performance: azidentity.NewClientSecretCredential fetches
// an OAuth token from Azure AD (login.microsoftonline.com) on its first actual API
// call and caches it internally. Without caching the object itself, every Azure API
// call would discard that cached token and trigger a new Azure AD round-trip.
func getOrCreateCredential(creds *azureCreds) (*azidentity.ClientSecretCredential, error) {
	key := creds.TenantID + "|" + creds.ClientID
	if v, ok := credentialCache.Load(key); ok {
		return v.(*azidentity.ClientSecretCredential), nil
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("Azure: failed to create ClientSecretCredential: %w", err)
	}

	actual, _ := credentialCache.LoadOrStore(key, credential)
	return actual.(*azidentity.ClientSecretCredential), nil
}

// getOrCreateVMClient returns a cached VirtualMachinesClient for the given subscription.
func getOrCreateVMClient(creds *azureCreds) (*armcompute.VirtualMachinesClient, error) {
	if v, ok := vmClientCache.Load(creds.SubscriptionID); ok {
		return v.(*armcompute.VirtualMachinesClient), nil
	}

	credential, err := getOrCreateCredential(creds)
	if err != nil {
		return nil, err
	}

	client, err := armcompute.NewVirtualMachinesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure: failed to create VirtualMachinesClient: %w", err)
	}

	actual, _ := vmClientCache.LoadOrStore(creds.SubscriptionID, client)
	return actual.(*armcompute.VirtualMachinesClient), nil
}

// getOrCreateTagsClient returns a cached TagsClient for the given subscription.
func getOrCreateTagsClient(creds *azureCreds) (*armresources.TagsClient, error) {
	if v, ok := tagsClientCache.Load(creds.SubscriptionID); ok {
		return v.(*armresources.TagsClient), nil
	}

	credential, err := getOrCreateCredential(creds)
	if err != nil {
		return nil, err
	}

	client, err := armresources.NewTagsClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure: failed to create TagsClient: %w", err)
	}

	actual, _ := tagsClientCache.LoadOrStore(creds.SubscriptionID, client)
	return actual.(*armresources.TagsClient), nil
}
