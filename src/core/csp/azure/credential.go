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
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
)

// credentialCache stores ClientSecretCredential objects, keyed by the account and the
// credential in use. Reusing the object is what makes the Azure SDK reuse its cached
// OAuth token (~1 hour) instead of calling Azure AD on every API call. Client objects
// (VirtualMachines, Tags) are not cached: their constructors do no I/O and azcore
// shares one HTTP transport process-wide, so building them per call costs nothing.
var credentialCache sync.Map

// getOrCreateCredential returns a cached ClientSecretCredential for the given Azure
// credentials, creating one if it does not already exist.
func getOrCreateCredential(creds *azureCreds) (*azidentity.ClientSecretCredential, error) {
	account := creds.TenantID + "|" + creds.ClientID
	credKey := csp.CredKey(creds.ClientSecret)
	if v, ok := csp.LoadClient(&credentialCache, account, credKey); ok {
		return v.(*azidentity.ClientSecretCredential), nil
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("Azure: failed to create ClientSecretCredential: %w", err)
	}

	return csp.StoreClient(&credentialCache, account, credKey, credential).(*azidentity.ClientSecretCredential), nil
}

// newVMClient returns a VirtualMachinesClient built on the cached credential.
func newVMClient(creds *azureCreds) (*armcompute.VirtualMachinesClient, error) {
	credential, err := getOrCreateCredential(creds)
	if err != nil {
		return nil, err
	}
	client, err := armcompute.NewVirtualMachinesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure: failed to create VirtualMachinesClient: %w", err)
	}
	return client, nil
}

// newTagsClient returns a TagsClient built on the cached credential.
func newTagsClient(creds *azureCreds) (*armresources.TagsClient, error) {
	credential, err := getOrCreateCredential(creds)
	if err != nil {
		return nil, err
	}
	client, err := armresources.NewTagsClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure: failed to create TagsClient: %w", err)
	}
	return client, nil
}
