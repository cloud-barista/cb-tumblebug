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

package kt

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/compute/v2/extensions/limits"
	"github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/compute/v2/extensions/quotasets"
	"github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/compute/v2/flavors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

func init() {
	csp.RegisterAvailabilityChecker(&ktAvailabilityChecker{})
}

type ktAvailabilityChecker struct{}

func (c *ktAvailabilityChecker) Provider() string { return csptypes.KT }

type ktFlavorInfo struct {
	id    string
	name  string
	vcpus int
	ramMB int
}

type ktFlavorCacheEntry struct {
	byKey     map[string]ktFlavorInfo
	fetchedAt time.Time
}

type ktLimitsCacheEntry struct {
	abs       *limits.Absolute
	fetchedAt time.Time
}

var (
	ktFlavorCache   = make(map[string]*ktFlavorCacheEntry)
	ktFlavorCacheMu sync.RWMutex
	ktFlavorSF      singleflight.Group
	ktFlavorTTL     = 30 * time.Minute

	ktLimitsCache   = make(map[string]*ktLimitsCacheEntry)
	ktLimitsCacheMu sync.RWMutex
	ktLimitsSF      singleflight.Group
	ktLimitsTTL     = 2 * time.Minute
)

func getKTFlavors(ctx context.Context, zone string) (map[string]ktFlavorInfo, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := csp.CredKey(c.ProjectID) + "+" + strings.ToUpper(zone)
	ktFlavorCacheMu.RLock()
	entry, ok := ktFlavorCache[cacheKey]
	ktFlavorCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < ktFlavorTTL {
		return entry.byKey, nil
	}

	v, err, _ := ktFlavorSF.Do(cacheKey, func() (any, error) {
		ktFlavorCacheMu.RLock()
		entry, ok := ktFlavorCache[cacheKey]
		ktFlavorCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < ktFlavorTTL {
			return entry.byKey, nil
		}

		client, err := newComputeClient(ctx, zone)
		if err != nil {
			return nil, err
		}
		pages, err := flavors.ListDetail(client, flavors.ListOpts{}).AllPages()
		if err != nil {
			return nil, fmt.Errorf("KT flavors.ListDetail failed (zone=%s): %w", zone, err)
		}
		list, err := flavors.ExtractFlavors(pages)
		if err != nil {
			return nil, fmt.Errorf("KT flavors.ExtractFlavors failed (zone=%s): %w", zone, err)
		}
		byKey := make(map[string]ktFlavorInfo, len(list)*2)
		for _, f := range list {
			info := ktFlavorInfo{
				id:    f.ID,
				name:  f.Name,
				vcpus: f.VCPUs,
				ramMB: f.RAM,
			}
			if f.Name != "" {
				byKey[strings.ToLower(f.Name)] = info
			}
			if f.ID != "" {
				byKey[strings.ToLower(f.ID)] = info
			}
		}
		e := &ktFlavorCacheEntry{byKey: byKey, fetchedAt: time.Now()}
		ktFlavorCacheMu.Lock()
		ktFlavorCache[cacheKey] = e
		ktFlavorCacheMu.Unlock()
		return byKey, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]ktFlavorInfo), nil
}

func getKTLimits(ctx context.Context, zone string) (*limits.Absolute, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := csp.CredKey(c.ProjectID) + "+" + strings.ToUpper(zone)
	ktLimitsCacheMu.RLock()
	entry, ok := ktLimitsCache[cacheKey]
	ktLimitsCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < ktLimitsTTL {
		return entry.abs, nil
	}

	v, err, _ := ktLimitsSF.Do(cacheKey, func() (any, error) {
		ktLimitsCacheMu.RLock()
		entry, ok := ktLimitsCache[cacheKey]
		ktLimitsCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < ktLimitsTTL {
			return entry.abs, nil
		}

		client, err := newComputeClient(ctx, zone)
		if err != nil {
			return nil, err
		}
		var resolved *limits.Absolute
		if lim, lErr := limits.Get(client, limits.GetOpts{}).Extract(); lErr == nil && lim != nil {
			abs := lim.Absolute
			resolved = &abs
		} else if qd, qdErr := quotasets.GetDetail(client, c.ProjectID).Extract(); qdErr == nil {
			resolved = &limits.Absolute{
				MaxTotalInstances:  qd.Instances.Limit,
				TotalInstancesUsed: qd.Instances.InUse + qd.Instances.Reserved,
				MaxTotalCores:      qd.Cores.Limit,
				TotalCoresUsed:     qd.Cores.InUse + qd.Cores.Reserved,
				MaxTotalRAMSize:    qd.RAM.Limit,
				TotalRAMUsed:       qd.RAM.InUse + qd.RAM.Reserved,
			}
		} else if qs, qsErr := quotasets.Get(client, c.ProjectID).Extract(); qsErr == nil && qs != nil {
			resolved = &limits.Absolute{
				MaxTotalInstances: qs.Instances,
				MaxTotalCores:     qs.Cores,
				MaxTotalRAMSize:   qs.RAM,
			}
		}
		e := &ktLimitsCacheEntry{abs: resolved, fetchedAt: time.Now()}
		ktLimitsCacheMu.Lock()
		ktLimitsCache[cacheKey] = e
		ktLimitsCacheMu.Unlock()
		return resolved, nil
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*limits.Absolute), nil
}

// CheckInstance checks KT Cloud VPC (OpenStack Nova) flavor existence and project compute
// quotas (instances, vCPUs, RAM) via flavors.ListDetail and limits.Get.
func (c *ktAvailabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	targetZone := zoneArg(region, strings.TrimSpace(q.PreferredZone))
	start := time.Now()
	result := model.AvailabilityResult{
		Provider:     csptypes.KT,
		Region:       region,
		InstanceType: instanceType,
		Available:    true,
		Status:       model.AvailabilityAvailable,
		Source:       "kt:flavors+limits",
		QueriedAt:    start,
	}

	reqVCPUs := q.VCPU
	reqRAMMB := int(math.Round(float64(q.MemoryGiB) * 1024))

	flavorMap, fErr := getKTFlavors(ctx, targetZone)
	if fErr != nil {
		return model.AvailabilityResult{}, fErr
	}
	if len(flavorMap) > 0 {
		f, found := flavorMap[strings.ToLower(instanceType)]
		if !found {
			result.Available = false
			result.Status = model.AvailabilityUnsupported
			result.Reason = fmt.Sprintf("KT flavor %q is not offered in zone/region %q", instanceType, targetZone)
			return result, nil
		}
		if reqVCPUs == 0 && f.vcpus > 0 {
			reqVCPUs = f.vcpus
		}
		if reqRAMMB == 0 && f.ramMB > 0 {
			reqRAMMB = f.ramMB
		}
	}

	abs, lErr := getKTLimits(ctx, targetZone)
	if lErr != nil {
		log.Warn().Err(lErr).Str("zone", targetZone).Msg("[KT] limits.Get failed; skipping quota check")
	} else if abs != nil {
		if abs.MaxTotalInstances > 0 {
			remInst := max(abs.MaxTotalInstances-abs.TotalInstancesUsed, 0)
			if remInst < 1 {
				result.Available = false
				result.Status = model.AvailabilityQuotaExceeded
				result.Reason = fmt.Sprintf("KT instance quota exhausted in %q: 0 instances remaining (used %d / limit %d)",
					targetZone, abs.TotalInstancesUsed, abs.MaxTotalInstances)
			}
		}
		if result.Available && reqVCPUs > 0 && abs.MaxTotalCores > 0 {
			remCores := max(abs.MaxTotalCores-abs.TotalCoresUsed, 0)
			if remCores < reqVCPUs {
				result.Available = false
				result.Status = model.AvailabilityQuotaExceeded
				result.Reason = fmt.Sprintf("KT vCPU quota insufficient for %q in %q: need %d cores, %d remaining (used %d / limit %d)",
					instanceType, targetZone, reqVCPUs, remCores, abs.TotalCoresUsed, abs.MaxTotalCores)
			}
		}
		if result.Available && reqRAMMB > 0 && abs.MaxTotalRAMSize > 0 {
			remRAM := max(abs.MaxTotalRAMSize-abs.TotalRAMUsed, 0)
			if remRAM < reqRAMMB {
				result.Available = false
				result.Status = model.AvailabilityQuotaExceeded
				result.Reason = fmt.Sprintf("KT RAM quota insufficient for %q in %q: need %d MB, %d MB remaining (used %d / limit %d MB)",
					instanceType, targetZone, reqRAMMB, remRAM, abs.TotalRAMUsed, abs.MaxTotalRAMSize)
			}
		}
	}

	log.Debug().
		Str("provider", csptypes.KT).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Str("status", string(result.Status)).
		Dur("elapsed", time.Since(start)).
		Msg("kt availability check completed")

	return result, nil
}
