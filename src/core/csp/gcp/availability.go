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

// This file implements csp.AvailabilityChecker for GCP Compute Engine using
// the machineTypes.aggregatedList API. The API returns, across all zones in a
// project, which zones support a given machine type.
//
// Rationale:
//   - The most common GCP VM-creation failure is:
//     "ZONE_RESOURCE_POOL_EXHAUSTED: The zone does not have enough resources
//     available to fulfill the request. (resource type:compute)"
//   - machineTypes.aggregatedList with a filter on machine type name returns
//     all zones where that machine type exists. Zones where the machine type
//     is not listed are definitively unsupported; supported zones may still
//     fail due to transient capacity — which is exactly when zone retry helps.
//   - Unlike Tencent/Alibaba, GCP's machineTypes API does not expose real-time
//     stock status (SELL/SOLD_OUT). We therefore treat "zone lists this machine
//     type" as Available=true; zone retry handles the transient capacity case.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterAvailabilityChecker(&gcpAvailabilityChecker{})
}

type gcpAvailabilityChecker struct{}

func (c *gcpAvailabilityChecker) Provider() string { return cspconst.GCP }

// CheckInstance queries GCP machineTypes.aggregatedList for the given
// (region, machine type) and returns per-zone availability based on which
// zones list that machine type. The region filter restricts results to zones
// whose name starts with the region prefix (e.g., "asia-south2-" for "asia-south2").
func (c *gcpAvailabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	creds, err := getGCPCreds(ctx)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to get GCP credentials: %w", err)
	}

	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to create GCP compute service: %w", err)
	}

	start := time.Now()
	// Filter by exact machine type name. The filter syntax is "name=MACHINE_TYPE".
	resp, err := svc.MachineTypes.AggregatedList(creds.ProjectID).
		Filter(fmt.Sprintf("name=%s", instanceType)).
		Context(ctx).
		Do()
	elapsed := time.Since(start)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("machineTypes.aggregatedList(project=%s, instanceType=%s) failed after %s: %w",
			creds.ProjectID, instanceType, elapsed, err)
	}

	result := model.AvailabilityResult{
		Provider:     cspconst.GCP,
		Region:       region,
		InstanceType: instanceType,
		Source:       "gcp:machineTypes.aggregatedList",
		QueriedAt:    time.Now(),
	}

	// regionPrefix is used to restrict results to zones within this region.
	// GCP zone names follow the pattern: {region}-{letter}, e.g., "asia-south2-a".
	regionPrefix := region + "-"

	for zoneKey, items := range resp.Items {
		// zoneKey: "zones/asia-south2-a" or "regions/..." (skip non-zone keys)
		if !strings.HasPrefix(zoneKey, "zones/") {
			continue
		}
		zoneName := strings.TrimPrefix(zoneKey, "zones/")
		if !strings.HasPrefix(zoneName, regionPrefix) {
			continue
		}

		// If the preferred zone is specified, only report that zone.
		if q.PreferredZone != "" && !strings.EqualFold(zoneName, q.PreferredZone) {
			continue
		}

		// items.MachineTypes is non-nil and non-empty only when the machine type
		// exists in that zone. A Warning with code NO_RESULTS_ON_PAGE means
		// the zone does not support this machine type.
		available := len(items.MachineTypes) > 0
		z := model.ZoneAvailability{
			ZoneId:    zoneName,
			Available: available,
		}
		if available {
			z.Status = "AVAILABLE"
			result.Available = true
		} else {
			z.Status = "UNAVAILABLE"
			z.Reason = fmt.Sprintf("machine type %q not listed in zone %q", instanceType, zoneName)
		}
		result.Zones = append(result.Zones, z)
	}

	if !result.Available {
		result.Reason = fmt.Sprintf("machine type %q is not listed in any zone of region %q "+
			"(machineTypes.aggregatedList returned no matching zones). "+
			"The machine type may not be supported in this region, or may require a reservation.",
			instanceType, region)
	}

	// GPU regional quota check.
	// machineTypes.aggregatedList only tells us whether the machine type exists in a zone —
	// it does not expose real-time GPU quota. For GPU specs, also check the regional quota
	// via regions.get so that quota exhaustion is detected before provisioning time.
	if result.Available && q.AcceleratorModel != "" && q.AcceleratorCount > 0 {
		quotaResult, quotaErr := checkGPURegionQuota(ctx, region, q.AcceleratorModel)
		if quotaErr != nil {
			log.Warn().Err(quotaErr).
				Str("region", region).
				Str("acceleratorModel", q.AcceleratorModel).
				Msg("GCP GPU quota check failed; treating as available")
		} else if quotaResult != nil && quotaResult.available < int64(q.AcceleratorCount) {
			result.Available = false
			result.Reason = fmt.Sprintf(
				"GCP GPU quota insufficient: %d %s available in region %q, need %d",
				quotaResult.available, quotaResult.metric, region, q.AcceleratorCount)
		}
	}

	log.Debug().
		Str("provider", cspconst.GCP).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Int("zoneCount", len(result.Zones)).
		Dur("elapsed", elapsed).
		Msg("gcp availability check completed")

	return result, nil
}

// acceleratorQuotaMetrics maps lowercase GCP accelerator type names to the
// corresponding Compute Engine regional quota metric name (from regions.get).
var acceleratorQuotaMetrics = map[string]string{
	"nvidia-tesla-a100":     "NVIDIA_A100_GPUS",
	"nvidia-a100-80gb":      "NVIDIA_A100_80GB_GPUS",
	"nvidia-tesla-v100":     "NVIDIA_V100_GPUS",
	"nvidia-tesla-t4":       "NVIDIA_T4_GPUS",
	"nvidia-h100-80gb":      "NVIDIA_H100_GPUS",
	"nvidia-h100-mega-80gb": "NVIDIA_H100_MEGA_GPUS",
	"nvidia-l4":             "NVIDIA_L4_GPUS",
	"nvidia-tesla-k80":      "NVIDIA_K80_GPUS",
	"nvidia-tesla-p4":       "NVIDIA_P4_GPUS",
	"nvidia-tesla-p100":     "NVIDIA_P100_GPUS",
	"nvidia-a10g":           "NVIDIA_A10_GPUS",
}

type gpuQuotaCheckResult struct {
	metric    string
	limit     int64
	used      int64
	available int64
}

// checkGPURegionQuota checks the GCP Compute Engine regional quota for a given
// accelerator type. Returns (nil, nil) when the accelerator type is unrecognised
// or the quota metric is absent — caller should treat this as "quota unknown,
// do not block provisioning".
func checkGPURegionQuota(ctx context.Context, region, acceleratorModel string) (*gpuQuotaCheckResult, error) {
	metric := resolveGPUQuotaMetric(acceleratorModel)
	if metric == "" {
		return nil, nil
	}

	creds, err := getGCPCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCP GPU quota check: failed to get credentials: %w", err)
	}

	svc, err := newComputeService(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("GCP GPU quota check: failed to create compute service: %w", err)
	}

	regionInfo, err := svc.Regions.Get(creds.ProjectID, region).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("GCP GPU quota check: regions.get(%s/%s) failed: %w", creds.ProjectID, region, err)
	}

	for _, q := range regionInfo.Quotas {
		if q.Metric == metric {
			available := int64(q.Limit) - int64(q.Usage)
			if available < 0 {
				available = 0
			}
			return &gpuQuotaCheckResult{
				metric:    metric,
				limit:     int64(q.Limit),
				used:      int64(q.Usage),
				available: available,
			}, nil
		}
	}

	return nil, nil
}

// resolveGPUQuotaMetric maps an accelerator model name to its GCP regional quota
// metric name. Returns empty string for unknown models.
func resolveGPUQuotaMetric(acceleratorModel string) string {
	if acceleratorModel == "" {
		return ""
	}
	lower := strings.ToLower(acceleratorModel)
	if m, ok := acceleratorQuotaMetrics[lower]; ok {
		return m
	}
	// Substring fallback for variant names (e.g. "nvidia-tesla-a100-sxm4-40gb").
	for k, v := range acceleratorQuotaMetrics {
		if strings.Contains(lower, strings.TrimPrefix(k, "nvidia-tesla-")) ||
			strings.Contains(lower, strings.TrimPrefix(k, "nvidia-")) {
			return v
		}
	}
	return ""
}
