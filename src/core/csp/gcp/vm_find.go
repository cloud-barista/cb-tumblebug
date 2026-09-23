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
	csp.RegisterFindVMsByUIDHandler(csptypes.GCP, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on GCP Compute Engine
// by matching instance names in the project.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCP vmfind: cannot get credentials: %w", err)
	}

	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("GCP vmfind: cannot create compute service: %w", err)
	}

	want := make(map[string]struct{}, len(uids))
	for _, id := range uids {
		want[id] = struct{}{}
	}

	result := make(map[string]string)
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
			zoneName := strings.TrimPrefix(zoneKey, "zones/")
			if region != "" && !strings.HasPrefix(zoneName, region+"-") {
				continue
			}
			for _, inst := range items.Instances {
				if _, ok := want[inst.Name]; ok {
					// In GCP, instance Name serves as the CspResourceId/CspResourceName
					result[inst.Name] = inst.Name
				}
			}
		}

		pageToken = aggResp.NextPageToken
		if pageToken == "" || len(result) == len(want) {
			break
		}
	}

	log.Debug().
		Str("region", region).
		Str("project", creds.ProjectID).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[GCP] FindInstancesByUIDs completed")

	return result, nil
}
