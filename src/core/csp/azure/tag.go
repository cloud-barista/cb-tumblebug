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
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchTagHandler(csptypes.Azure, BatchUpsertTags)
}

// BatchUpsertTags merges multiple tags onto an Azure ARM resource in a single PATCH call.
// The region parameter is unused for Azure (ARM Tags API uses the full resource ID scope).
// The cspResourceId must be the full ARM resource ID
// (e.g., "/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}").
func BatchUpsertTags(ctx context.Context, region, zone, cspResourceId, resourceType string, tags map[string]string) error {
	creds, err := getCreds(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}

	tagsClient, err := armresources.NewTagsClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create Azure Tags client: %w", err)
	}

	// Convert map to *string map for ARM SDK
	armTags := make(map[string]*string, len(tags))
	for k, v := range tags {
		v := v // capture loop var
		armTags[k] = &v
	}

	// PATCH merge — adds/updates tags without removing existing ones
	mergeOp := armresources.TagsPatchOperationMerge
	_, err = tagsClient.UpdateAtScope(ctx, cspResourceId, armresources.TagsPatchResource{
		Operation: &mergeOp,
		Properties: &armresources.Tags{
			Tags: armTags,
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("Azure Tags UpdateAtScope failed for %s: %w", cspResourceId, err)
	}

	log.Debug().
		Str("resourceId", cspResourceId).
		Int("tagCount", len(tags)).
		Msg("[Azure] Batch tags upserted via ARM Tags API")

	return nil
}
