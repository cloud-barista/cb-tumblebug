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

package csp

import (
	"context"
	"strings"

	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

// BatchTagHandler defines the function signature for CSP-specific batch tag upsert.
// Returns error if the operation fails. The handler should set all given tags on
// the CSP resource identified by cspResourceId in a single API call.
// resourceType is the CB-Tumblebug label type (e.g., "node", "vNet", "sshKey").
// region is the CSP region (e.g., "us-east-1"), zone is the availability zone (e.g., "us-east-1a").
type BatchTagHandler func(ctx context.Context, region, zone, cspResourceId, resourceType string, tags map[string]string) error

// batchTagHandlers maps CSP platform names to their batch tag implementations.
// Populated by init() in each CSP package (e.g., csp/aws/tag.go, csp/azure/tag.go).
var batchTagHandlers = make(map[string]BatchTagHandler)

// RegisterBatchTagHandler registers a batch tag upsert handler for a CSP.
// Called by CSP-specific packages during init().
func RegisterBatchTagHandler(platform string, handler BatchTagHandler) {
	batchTagHandlers[strings.ToLower(platform)] = handler
}

// TryBatchUpsertTags attempts to upsert multiple tags on a CSP resource in a single API call.
// resourceType is the CB-Tumblebug label type (e.g., "node", "vNet", "sshKey").
// region is the CSP region, zone is the availability zone (used by GCP; can be empty for others).
// Returns (true, nil) if successfully handled by a direct CSP batch API.
// Returns (false, nil) if no batch handler exists for this CSP (caller should fall back to Spider).
// Returns (false, err) if a batch handler exists but failed (caller should fall back to Spider).
func TryBatchUpsertTags(ctx context.Context, providerName, region, zone, cspResourceId, resourceType string, tags map[string]string) (bool, error) {
	if cspResourceId == "" {
		return false, nil
	}

	platform := csptypes.ResolveCloudPlatform(providerName)
	handler, exists := batchTagHandlers[platform]
	if !exists {
		return false, nil
	}

	// Send all labels (including sys.*) to CSP, consistent with the CB-Spider fallback path.
	if len(tags) == 0 {
		return true, nil // nothing to sync
	}

	log.Debug().
		Str("provider", platform).
		Str("region", region).
		Str("zone", zone).
		Str("cspResourceId", cspResourceId).
		Int("tagCount", len(tags)).
		Msg("[CSP] Batch upsert tags via direct CSP API")

	if err := handler(ctx, region, zone, cspResourceId, resourceType, tags); err != nil {
		return false, err
	}

	return true, nil
}
