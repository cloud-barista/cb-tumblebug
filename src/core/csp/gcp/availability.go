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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func init() {
	csp.RegisterAvailabilityChecker(&gcpAvailabilityChecker{})
}

type gcpAvailabilityChecker struct{}

func (c *gcpAvailabilityChecker) Provider() string { return cspconst.GCP }

type gcpRegionQuotaCacheEntry struct {
	quotas    map[string]*compute.Quota
	zones     []string
	fetchedAt time.Time
}

type gcpRegionCatalogCacheEntry struct {
	// zoneName -> machineType -> guestCpus
	zoneMachineTypes map[string]map[string]int64
	fetchedAt        time.Time
}

var (
	gcpQuotaCache   = make(map[string]*gcpRegionQuotaCacheEntry)
	gcpQuotaCacheMu sync.RWMutex
	gcpQuotaSF      singleflight.Group
	gcpQuotaTTL     = 2 * time.Minute

	gcpCatalogCache   = make(map[string]*gcpRegionCatalogCacheEntry)
	gcpCatalogCacheMu sync.RWMutex
	gcpCatalogSF      singleflight.Group
	gcpCatalogTTL     = 30 * time.Minute
)

func getGCPRegionInfoAndQuotas(ctx context.Context, svc *compute.Service, projectID, region string) (map[string]*compute.Quota, []string, error) {
	cacheKey := projectID + "+" + region
	gcpQuotaCacheMu.RLock()
	entry, ok := gcpQuotaCache[cacheKey]
	gcpQuotaCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < gcpQuotaTTL {
		return entry.quotas, entry.zones, nil
	}

	v, err, _ := gcpQuotaSF.Do(cacheKey, func() (any, error) {
		gcpQuotaCacheMu.RLock()
		entry, ok := gcpQuotaCache[cacheKey]
		gcpQuotaCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < gcpQuotaTTL {
			return entry, nil
		}

		regionInfo, err := svc.Regions.Get(projectID, region).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("regions.get(%s/%s) failed: %w", projectID, region, err)
		}
		quotas := make(map[string]*compute.Quota, len(regionInfo.Quotas))
		for _, q := range regionInfo.Quotas {
			if q != nil && q.Metric != "" {
				quotas[q.Metric] = q
			}
		}
		var zones []string
		for _, zURL := range regionInfo.Zones {
			if idx := strings.LastIndex(zURL, "/"); idx >= 0 && idx+1 < len(zURL) {
				zones = append(zones, zURL[idx+1:])
			}
		}
		sort.Strings(zones)
		e := &gcpRegionQuotaCacheEntry{
			quotas:    quotas,
			zones:     zones,
			fetchedAt: time.Now(),
		}
		gcpQuotaCacheMu.Lock()
		gcpQuotaCache[cacheKey] = e
		gcpQuotaCacheMu.Unlock()
		return e, nil
	})
	if err != nil {
		return nil, nil, err
	}
	res := v.(*gcpRegionQuotaCacheEntry)
	return res.quotas, res.zones, nil
}

func getGCPRegionQuotas(ctx context.Context, svc *compute.Service, projectID, region string) (map[string]*compute.Quota, error) {
	quotas, _, err := getGCPRegionInfoAndQuotas(ctx, svc, projectID, region)
	return quotas, err
}

// getGCPRegionMachineTypes fetches and caches all machine types for the zones in a single GCP region
// using parallel zonal MachineTypes.List calls (with partial field projection) instead of a slow global
// MachineTypes.AggregatedList across 120+ worldwide zones.
func getGCPRegionMachineTypes(ctx context.Context, svc *compute.Service, projectID, region string) (map[string]map[string]int64, error) {
	cacheKey := projectID + "+" + region
	gcpCatalogCacheMu.RLock()
	entry, ok := gcpCatalogCache[cacheKey]
	gcpCatalogCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < gcpCatalogTTL {
		return entry.zoneMachineTypes, nil
	}

	v, err, _ := gcpCatalogSF.Do(cacheKey, func() (any, error) {
		gcpCatalogCacheMu.RLock()
		entry, ok := gcpCatalogCache[cacheKey]
		gcpCatalogCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < gcpCatalogTTL {
			return entry.zoneMachineTypes, nil
		}

		// Use a background-derived timeout so one caller's tight deadline doesn't abort
		// the shared singleflight fetch for sibling callers.
		fetchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		_, zones, err := getGCPRegionInfoAndQuotas(fetchCtx, svc, projectID, region)
		if err != nil || len(zones) == 0 {
			return nil, fmt.Errorf("failed to resolve zones for GCP region %s: %w", region, err)
		}

		zoneMap := make(map[string]map[string]int64, len(zones))
		var mu sync.Mutex
		var wg sync.WaitGroup
		var firstErr error

		for _, zName := range zones {
			wg.Add(1)
			go func(zone string) {
				defer wg.Done()
				mtMap := make(map[string]int64, 256)
				pageToken := ""
				for {
					call := svc.MachineTypes.List(projectID, zone).
						Fields(googleapi.Field("items(name,guestCpus),nextPageToken")).
						MaxResults(500).
						Context(fetchCtx)
					if pageToken != "" {
						call = call.PageToken(pageToken)
					}
					resp, lErr := call.Do()
					if lErr != nil {
						mu.Lock()
						if firstErr == nil {
							firstErr = fmt.Errorf("machineTypes.list(%s/%s) failed: %w", projectID, zone, lErr)
						}
						mu.Unlock()
						return
					}
					for _, mt := range resp.Items {
						if mt != nil && mt.Name != "" {
							mtMap[mt.Name] = mt.GuestCpus
						}
					}
					if resp.NextPageToken == "" {
						break
					}
					pageToken = resp.NextPageToken
				}
				mu.Lock()
				zoneMap[zone] = mtMap
				mu.Unlock()
			}(zName)
		}
		wg.Wait()

		if firstErr != nil && len(zoneMap) == 0 {
			return nil, firstErr
		}

		e := &gcpRegionCatalogCacheEntry{
			zoneMachineTypes: zoneMap,
			fetchedAt:        time.Now(),
		}
		gcpCatalogCacheMu.Lock()
		gcpCatalogCache[cacheKey] = e
		gcpCatalogCacheMu.Unlock()
		return zoneMap, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]map[string]int64), nil
}

// resolveFamilyCPUQuotaMetric maps a GCP machine type prefix (e.g. "c2-standard-4" -> "C2_CPUS")
// to its family-specific regional CPU quota metric name when applicable.
func resolveFamilyCPUQuotaMetric(instanceType string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(instanceType)), "-")
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "c2":
		return "C2_CPUS"
	case "c2d":
		return "C2D_CPUS"
	case "c3":
		return "C3_CPUS"
	case "c3d":
		return "C3D_CPUS"
	case "n2":
		return "N2_CPUS"
	case "n2d":
		return "N2D_CPUS"
	case "t2d":
		return "T2D_CPUS"
	case "t2a":
		return "T2A_CPUS"
	case "a2":
		return "A2_CPUS"
	case "a3":
		return "A3_CPUS"
	case "g2":
		return "G2_CPUS"
	case "m1":
		return "M1_CPUS"
	case "m2":
		return "M2_CPUS"
	case "m3":
		return "M3_CPUS"
	default:
		return ""
	}
}

// CheckInstance queries GCP regional machineTypes (per-zone MachineTypes.List cached per region)
// and returns per-zone availability based on which zones list that machine type, followed by
// regional quota verification (CPU, machine-family CPU, instances, and GPU quotas) via regions.get.
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
	zoneMachineTypes, err := getGCPRegionMachineTypes(ctx, svc, creds.ProjectID, region)
	elapsed := time.Since(start)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("GCP regional machineTypes lookup(project=%s, region=%s, instanceType=%s) failed after %s: %w",
			creds.ProjectID, region, instanceType, elapsed, err)
	}

	result := model.AvailabilityResult{
		Provider:     cspconst.GCP,
		Region:       region,
		InstanceType: instanceType,
		Source:       "gcp:machineTypes.list",
		QueriedAt:    time.Now(),
	}

	guestCpus := int64(q.VCPU)
	zones := make([]string, 0, len(zoneMachineTypes))
	for zName := range zoneMachineTypes {
		zones = append(zones, zName)
	}
	sort.Strings(zones)

	for _, zoneName := range zones {
		if q.PreferredZone != "" && !strings.EqualFold(zoneName, q.PreferredZone) {
			continue
		}
		mtMap := zoneMachineTypes[zoneName]
		cpus, available := mtMap[instanceType]
		if available && guestCpus == 0 && cpus > 0 {
			guestCpus = cpus
		}
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

	if result.Available {
		result.Status = model.AvailabilityAvailable
	} else {
		result.Status = model.AvailabilityUnsupported
		result.Reason = fmt.Sprintf("machine type %q is not listed in any zone of region %q. "+
			"The machine type may not be supported in this region, or may require a reservation.",
			instanceType, region)
	}

	// Regional quota check (CPU, machine-family CPU, instances, and GPU quotas) via regions.get.
	if result.Available {
		quotas, quotaErr := getGCPRegionQuotas(ctx, svc, creds.ProjectID, region)
		if quotaErr != nil {
			log.Warn().Err(quotaErr).
				Str("region", region).
				Msg("GCP regional quota check failed; treating as available")
		} else if quotas != nil {
			// 1. Check INSTANCES quota
			if instQ, ok := quotas["INSTANCES"]; ok {
				rem := max(int64(instQ.Limit)-int64(instQ.Usage), 0)
				if rem < 1 {
					result.Available = false
					result.Status = model.AvailabilityQuotaExceeded
					result.Reason = fmt.Sprintf("GCP regional quota insufficient: 0 INSTANCES remaining in region %q (used %d / limit %d)",
						region, int64(instQ.Usage), int64(instQ.Limit))
				}
			}
			// 2. Check total regional CPUS quota
			if result.Available && guestCpus > 0 {
				if cpuQ, ok := quotas["CPUS"]; ok {
					rem := max(int64(cpuQ.Limit)-int64(cpuQ.Usage), 0)
					if rem < guestCpus {
						result.Available = false
						result.Status = model.AvailabilityQuotaExceeded
						result.Reason = fmt.Sprintf("GCP regional CPU quota insufficient for %q in region %q: need %d CPUs, %d remaining (used %d / limit %d, metric: CPUS)",
							instanceType, region, guestCpus, rem, int64(cpuQ.Usage), int64(cpuQ.Limit))
					}
				}
			}
			// 3. Check machine-family specific CPU quota (e.g., C2_CPUS, N2_CPUS, A2_CPUS, G2_CPUS)
			if result.Available && guestCpus > 0 {
				if famMetric := resolveFamilyCPUQuotaMetric(instanceType); famMetric != "" {
					if famQ, ok := quotas[famMetric]; ok {
						rem := max(int64(famQ.Limit)-int64(famQ.Usage), 0)
						if rem < guestCpus {
							result.Available = false
							result.Status = model.AvailabilityQuotaExceeded
							result.Reason = fmt.Sprintf("GCP family CPU quota insufficient for %q in region %q: need %d CPUs, %d remaining (used %d / limit %d, metric: %s)",
								instanceType, region, guestCpus, rem, int64(famQ.Usage), int64(famQ.Limit), famMetric)
						}
					}
				}
			}
			// 4. Check GPU accelerator quota
			if result.Available && q.AcceleratorModel != "" && q.AcceleratorCount > 0 {
				if gpuMetric := resolveGPUQuotaMetric(q.AcceleratorModel); gpuMetric != "" {
					if gpuQ, ok := quotas[gpuMetric]; ok {
						rem := max(int64(gpuQ.Limit)-int64(gpuQ.Usage), 0)
						if rem < int64(q.AcceleratorCount) {
							result.Available = false
							result.Status = model.AvailabilityQuotaExceeded
							result.Reason = fmt.Sprintf(
								"GCP GPU quota insufficient: %d %s available in region %q, need %d (used %d / limit %d)",
								rem, gpuMetric, region, q.AcceleratorCount, int64(gpuQ.Usage), int64(gpuQ.Limit))
						}
					}
				}
			}
		}
	}

	log.Debug().
		Str("provider", cspconst.GCP).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Str("status", string(result.Status)).
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
