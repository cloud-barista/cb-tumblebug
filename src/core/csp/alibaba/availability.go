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

package alibaba

// This file implements csp.AvailabilityChecker for Alibaba ECS using the
// DescribeAvailableResource API. The API answers, for a given (region,
// instance-type[, system-disk-category]) tuple, which zones currently
// have stock and which system-disk categories are available in each zone.
//
// Rationale:
//   - The most common Alibaba VM-creation failure pattern observed in
//     practice is "No AvailableSystemDisk for that instance type in the
//     request region", a 4-tuple stock issue (region, zone, instance type,
//     disk category) that pure spec-catalog checks ("LookupSpec") cannot
//     detect ahead of time.
//   - DescribeAvailableResource with DestinationResource="SystemDisk"
//     returns the full zone × disk-category matrix in a single call.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterAvailabilityChecker(&availabilityChecker{})
}

type availabilityChecker struct{}

func (c *availabilityChecker) Provider() string { return cspconst.Alibaba }

// CheckInstance queries Alibaba ECS DescribeAvailableResource for the given
// (region, instance type, optional disk category) and returns per-zone
// availability with supported system-disk categories.
func (c *availabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	accessKeyID, accessKeySecret, err := getAlibabaCreds(ctx)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to get Alibaba credentials: %w", err)
	}

	client, err := ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to create Alibaba ECS client for region %s: %w", region, err)
	}

	req := ecs.CreateDescribeAvailableResourceRequest()
	req.Scheme = "https"
	req.RegionId = region
	// SystemDisk gives the richest matrix: zone × disk-category × stock status.
	// It also implicitly answers instance-type availability per zone.
	req.DestinationResource = "SystemDisk"
	req.ResourceType = "instance"
	req.IoOptimized = "optimized"
	req.InstanceType = instanceType
	if disk := strings.TrimSpace(q.SystemDiskCategory); disk != "" {
		req.SystemDiskCategory = disk
	}
	if zone := strings.TrimSpace(q.PreferredZone); zone != "" {
		req.ZoneId = zone
	}

	start := time.Now()
	resp, err := client.DescribeAvailableResource(req)
	elapsed := time.Since(start)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("DescribeAvailableResource(region=%s, instanceType=%s) failed after %s: %w",
			region, instanceType, elapsed, err)
	}

	result := model.AvailabilityResult{
		Provider:     cspconst.Alibaba,
		Region:       region,
		InstanceType: instanceType,
		Source:       "alibaba:DescribeAvailableResource",
		QueriedAt:    time.Now(),
	}

	for _, az := range resp.AvailableZones.AvailableZone {
		zoneEntry := model.ZoneAvailability{
			ZoneId: az.ZoneId,
			Status: az.Status, // Alibaba enum: "Available" / "SoldOut"
		}
		// Aggregate currently-available SystemDisk categories from this zone.
		// SupportedResource.Status uses the same enum: "Available" / "SoldOut".
		// Treat empty status as available too (some responses omit it on success).
		seen := map[string]bool{}
		for _, ar := range az.AvailableResources.AvailableResource {
			for _, sr := range ar.SupportedResources.SupportedResource {
				if sr.Value == "" || seen[sr.Value] {
					continue
				}
				if sr.Status == "" || strings.EqualFold(sr.Status, "Available") {
					zoneEntry.SupportedDisks = append(zoneEntry.SupportedDisks, sr.Value)
					seen[sr.Value] = true
				}
			}
		}
		// A zone is considered available when its status is "Available" and it
		// has at least one supported disk category. (Empty zone status is also
		// treated as available to be tolerant of partial responses.)
		zoneOk := az.Status == "" || strings.EqualFold(az.Status, "Available")
		zoneEntry.Available = zoneOk && len(zoneEntry.SupportedDisks) > 0
		if !zoneEntry.Available && zoneEntry.Reason == "" {
			zoneEntry.Reason = fmt.Sprintf("zone status=%q, supportedDisks=%d", az.Status, len(zoneEntry.SupportedDisks))
		}
		result.Zones = append(result.Zones, zoneEntry)
		if zoneEntry.Available {
			result.Available = true
		}
	}

	if !result.Available {
		// Render disk hint as "any" when no specific category was requested,
		// so the user-facing message doesn't show an empty quoted string.
		diskHint := strings.TrimSpace(q.SystemDiskCategory)
		if diskHint == "" {
			diskHint = "any"
		}
		if len(result.Zones) == 0 {
			result.Reason = fmt.Sprintf("Alibaba reports no zones for instance type %q in region %q right now (disk: %s). "+
				"This usually means temporary out-of-stock for this instance family in this region; "+
				"try a different region, a different instance type, or retry later.",
				instanceType, region, diskHint)
		} else {
			// Build a short per-zone status hint for transparency.
			zoneStatuses := make([]string, 0, len(result.Zones))
			for _, z := range result.Zones {
				if z.Status != "" {
					zoneStatuses = append(zoneStatuses, z.ZoneId+"="+z.Status)
				}
			}
			zoneSummary := ""
			if len(zoneStatuses) > 0 {
				zoneSummary = " [" + strings.Join(zoneStatuses, ", ") + "]"
			}
			result.Reason = fmt.Sprintf("Alibaba reports no zone with stock for instance type %q in region %q right now (disk: %s)%s. "+
				"This is typically temporary out-of-stock; try a different region, a different instance type, or retry later.",
				instanceType, region, diskHint, zoneSummary)
		}
	}

	log.Debug().
		Str("provider", cspconst.Alibaba).
		Str("region", region).
		Str("instanceType", instanceType).
		Str("diskCategory", q.SystemDiskCategory).
		Bool("available", result.Available).
		Int("zoneCount", len(result.Zones)).
		Dur("elapsed", elapsed).
		Msg("alibaba availability check completed")

	return result, nil
}
