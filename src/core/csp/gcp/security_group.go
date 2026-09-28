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
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterCheckSGInUseHandler(csptypes.GCP, CheckInstancesUsingSG)
}

// CheckInstancesUsingSG checks whether any active GCP Compute Engine instances currently
// have the given security group tag attached.
func CheckInstancesUsingSG(ctx context.Context, region string, cspSgId string) ([]string, error) {
	if cspSgId == "" {
		return nil, nil
	}

	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCP sg check: cannot get credentials: %w", err)
	}

	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("GCP sg check: cannot create compute service: %w", err)
	}

	var activeInstances []string
	pageToken := ""

	for {
		call := svc.Instances.AggregatedList(creds.ProjectID).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		aggResp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("GCP AggregatedList failed (project=%s): %w", creds.ProjectID, err)
		}

		for zoneKey, items := range aggResp.Items {
			zoneName := strings.TrimPrefix(zoneKey, "zones/")
			if region != "" && !strings.HasPrefix(zoneName, region+"-") {
				continue
			}
			for _, inst := range items.Instances {
				// TERMINATED instances do not occupy active network traffic
				if inst.Status == "TERMINATED" {
					continue
				}
				if inst.Tags != nil {
					for _, tag := range inst.Tags.Items {
						if tag == cspSgId {
							activeInstances = append(activeInstances, inst.Name)
							break
						}
					}
				}
			}
		}

		pageToken = aggResp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	log.Debug().
		Str("region", region).
		Str("cspSgId", cspSgId).
		Int("inUseCount", len(activeInstances)).
		Msg("[GCP] CheckInstancesUsingSG completed")

	return activeInstances, nil
}
