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

package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchVMStatusHandler(csptypes.GCP, BatchDescribeInstanceStatuses)
}

// BatchDescribeInstanceStatuses queries GCP Compute Engine AggregatedList for all
// instances in the project and returns a map of instanceName → TB status string,
// filtered to instances whose zone starts with the given region prefix and whose
// name appears in instanceIds.
//
// GCP instances are zone-scoped, so AggregatedList (one call for the whole project)
// is used instead of per-zone calls to avoid needing zone information per node.
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCP vmstatus: cannot get credentials: %w", err)
	}

	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("GCP vmstatus: cannot create compute service: %w", err)
	}

	// Build a lookup set for fast membership test.
	want := make(map[string]struct{}, len(instanceIds))
	for _, id := range instanceIds {
		want[id] = struct{}{}
	}

	result := make(map[string]string, len(instanceIds))

	// AggregatedList returns instances across all zones in the project.
	// Filter by zone prefix matching the region (e.g., "us-central1-a" starts with "us-central1").
	// Use manual pagination via Do() to access typed fields directly.
	pageToken := ""
	for {
		call := svc.Instances.AggregatedList(creds.ProjectID)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		aggResp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("GCP AggregatedList failed (project=%s): %w", creds.ProjectID, err)
		}

		for zoneKey, items := range aggResp.Items {
			// zoneKey is "zones/us-central1-a" — extract just the zone name.
			zoneName := strings.TrimPrefix(zoneKey, "zones/")
			// Only consider zones in the requested region.
			if !strings.HasPrefix(zoneName, region+"-") {
				continue
			}
			for _, inst := range items.Instances {
				if _, ok := want[inst.Name]; !ok {
					continue
				}
				result[inst.Name] = gcpStateToTBStatus(inst.Status)
			}
		}

		pageToken = aggResp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	log.Trace().
		Str("region", region).
		Str("project", creds.ProjectID).
		Int("queried", len(instanceIds)).
		Int("found", len(result)).
		Msg("[GCP] BatchDescribeInstanceStatuses completed")

	return result, nil
}

// gcpStateToTBStatus maps GCP Compute Engine instance status strings to TB status strings.
func gcpStateToTBStatus(state string) string {
	switch state {
	case "PROVISIONING", "STAGING":
		return model.StatusCreating
	case "RUNNING":
		return model.StatusRunning
	case "SUSPENDING", "STOPPING":
		return model.StatusSuspending
	case "SUSPENDED", "TERMINATED":
		// GCP "TERMINATED" means stopped (not billed for compute), not deleted.
		return model.StatusSuspended
	default:
		return model.StatusUndefined
	}
}
