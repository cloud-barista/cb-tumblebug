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

package nhn

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
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/extensions/limits"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/extensions/quotasets"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/flavors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

func init() {
	csp.RegisterAvailabilityChecker(&nhnAvailabilityChecker{})
}

type nhnAvailabilityChecker struct{}

func (c *nhnAvailabilityChecker) Provider() string { return csptypes.NHN }

type nhnFlavorInfo struct {
	id       string
	name     string
	vcpus    int
	ramMB    int
	disabled bool
}

type nhnFlavorCacheEntry struct {
	byKey     map[string]nhnFlavorInfo
	fetchedAt time.Time
}

type nhnLimitsCacheEntry struct {
	abs       *limits.Absolute
	fetchedAt time.Time
}

var (
	nhnFlavorCache   = make(map[string]*nhnFlavorCacheEntry)
	nhnFlavorCacheMu sync.RWMutex
	nhnFlavorSF      singleflight.Group
	nhnFlavorTTL     = 30 * time.Minute

	nhnLimitsCache   = make(map[string]*nhnLimitsCacheEntry)
	nhnLimitsCacheMu sync.RWMutex
	nhnLimitsSF      singleflight.Group
	nhnLimitsTTL     = 2 * time.Minute
)

func getNHNFlavors(ctx context.Context, region string) (map[string]nhnFlavorInfo, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := csp.CredKey(c.TenantID) + "+" + strings.ToUpper(region)
	nhnFlavorCacheMu.RLock()
	entry, ok := nhnFlavorCache[cacheKey]
	nhnFlavorCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < nhnFlavorTTL {
		return entry.byKey, nil
	}

	v, err, _ := nhnFlavorSF.Do(cacheKey, func() (any, error) {
		nhnFlavorCacheMu.RLock()
		entry, ok := nhnFlavorCache[cacheKey]
		nhnFlavorCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < nhnFlavorTTL {
			return entry.byKey, nil
		}

		client, err := newComputeClient(ctx, region)
		if err != nil {
			return nil, err
		}
		pages, err := flavors.ListDetail(client, flavors.ListOpts{}).AllPages()
		if err != nil {
			return nil, fmt.Errorf("NHN flavors.ListDetail failed (region=%s): %w", region, err)
		}
		list, err := flavors.ExtractFlavors(pages)
		if err != nil {
			return nil, fmt.Errorf("NHN flavors.ExtractFlavors failed (region=%s): %w", region, err)
		}
		byKey := make(map[string]nhnFlavorInfo, len(list)*2)
		for _, f := range list {
			info := nhnFlavorInfo{
				id:       f.ID,
				name:     f.Name,
				vcpus:    f.VCPUs,
				ramMB:    f.RAM,
				disabled: f.Disabled,
			}
			if f.Name != "" {
				byKey[strings.ToLower(f.Name)] = info
			}
			if f.ID != "" {
				byKey[strings.ToLower(f.ID)] = info
			}
		}
		e := &nhnFlavorCacheEntry{byKey: byKey, fetchedAt: time.Now()}
		nhnFlavorCacheMu.Lock()
		nhnFlavorCache[cacheKey] = e
		nhnFlavorCacheMu.Unlock()
		return byKey, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]nhnFlavorInfo), nil
}

func getNHNLimits(ctx context.Context, region string) (*limits.Absolute, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := csp.CredKey(c.TenantID) + "+" + strings.ToUpper(region)
	nhnLimitsCacheMu.RLock()
	entry, ok := nhnLimitsCache[cacheKey]
	nhnLimitsCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < nhnLimitsTTL {
		return entry.abs, nil
	}

	v, err, _ := nhnLimitsSF.Do(cacheKey, func() (any, error) {
		nhnLimitsCacheMu.RLock()
		entry, ok := nhnLimitsCache[cacheKey]
		nhnLimitsCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < nhnLimitsTTL {
			return entry.abs, nil
		}

		client, err := newComputeClient(ctx, region)
		if err != nil {
			return nil, err
		}
		var resolved *limits.Absolute
		if lim, lErr := limits.Get(client, limits.GetOpts{}).Extract(); lErr == nil && lim != nil {
			abs := lim.Absolute
			resolved = &abs
		} else if qd, qdErr := quotasets.GetDetail(client, c.TenantID).Extract(); qdErr == nil {
			resolved = &limits.Absolute{
				MaxTotalInstances:  qd.Instances.Limit,
				TotalInstancesUsed: qd.Instances.InUse + qd.Instances.Reserved,
				MaxTotalCores:      qd.Cores.Limit,
				TotalCoresUsed:     qd.Cores.InUse + qd.Cores.Reserved,
				MaxTotalRAMSize:    qd.RAM.Limit,
				TotalRAMUsed:       qd.RAM.InUse + qd.RAM.Reserved,
			}
		} else if qs, qsErr := quotasets.Get(client, c.TenantID).Extract(); qsErr == nil && qs != nil {
			resolved = &limits.Absolute{
				MaxTotalInstances: qs.Instances,
				MaxTotalCores:     qs.Cores,
				MaxTotalRAMSize:   qs.RAM,
			}
		}
		e := &nhnLimitsCacheEntry{abs: resolved, fetchedAt: time.Now()}
		nhnLimitsCacheMu.Lock()
		nhnLimitsCache[cacheKey] = e
		nhnLimitsCacheMu.Unlock()
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

// CheckInstance checks NHN Cloud (OpenStack Nova) flavor existence and tenant compute
// quotas (instances, vCPUs, RAM) via flavors.ListDetail and limits.Get.
func (c *nhnAvailabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	start := time.Now()
	result := model.AvailabilityResult{
		Provider:     csptypes.NHN,
		Region:       region,
		InstanceType: instanceType,
		Available:    true,
		Status:       model.AvailabilityAvailable,
		Source:       "nhn:flavors+limits",
		QueriedAt:    start,
	}

	reqVCPUs := q.VCPU
	reqRAMMB := int(math.Round(float64(q.MemoryGiB) * 1024))

	flavorMap, fErr := getNHNFlavors(ctx, region)
	if fErr != nil {
		return model.AvailabilityResult{}, fErr
	}
	if len(flavorMap) > 0 {
		f, found := flavorMap[strings.ToLower(instanceType)]
		if !found {
			result.Available = false
			result.Status = model.AvailabilityUnsupported
			result.Reason = fmt.Sprintf("NHN flavor %q is not offered in region %q", instanceType, region)
			return result, nil
		}
		if f.disabled {
			result.Available = false
			result.Status = model.AvailabilitySoldOut
			result.Reason = fmt.Sprintf("NHN flavor %q is currently disabled in region %q", instanceType, region)
			return result, nil
		}
		if reqVCPUs == 0 && f.vcpus > 0 {
			reqVCPUs = f.vcpus
		}
		if reqRAMMB == 0 && f.ramMB > 0 {
			reqRAMMB = f.ramMB
		}
	}

	abs, lErr := getNHNLimits(ctx, region)
	if lErr != nil {
		log.Warn().Err(lErr).Str("region", region).Msg("[NHN] limits.Get failed; skipping quota check")
	} else if abs != nil {
		if abs.MaxTotalInstances > 0 {
			remInst := max(abs.MaxTotalInstances-abs.TotalInstancesUsed, 0)
			if remInst < 1 {
				result.Available = false
				result.Status = model.AvailabilityQuotaExceeded
				result.Reason = fmt.Sprintf("NHN instance quota exhausted in region %q: 0 instances remaining (used %d / limit %d)",
					region, abs.TotalInstancesUsed, abs.MaxTotalInstances)
			}
		}
		if result.Available && reqVCPUs > 0 && abs.MaxTotalCores > 0 {
			remCores := max(abs.MaxTotalCores-abs.TotalCoresUsed, 0)
			if remCores < reqVCPUs {
				result.Available = false
				result.Status = model.AvailabilityQuotaExceeded
				result.Reason = fmt.Sprintf("NHN vCPU quota insufficient for %q in region %q: need %d cores, %d remaining (used %d / limit %d)",
					instanceType, region, reqVCPUs, remCores, abs.TotalCoresUsed, abs.MaxTotalCores)
			}
		}
		if result.Available && reqRAMMB > 0 && abs.MaxTotalRAMSize > 0 {
			remRAM := max(abs.MaxTotalRAMSize-abs.TotalRAMUsed, 0)
			if remRAM < reqRAMMB {
				result.Available = false
				result.Status = model.AvailabilityQuotaExceeded
				result.Reason = fmt.Sprintf("NHN RAM quota insufficient for %q in region %q: need %d MB, %d MB remaining (used %d / limit %d MB)",
					instanceType, region, reqRAMMB, remRAM, abs.TotalRAMUsed, abs.MaxTotalRAMSize)
			}
		}
	}

	log.Debug().
		Str("provider", csptypes.NHN).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Str("status", string(result.Status)).
		Dur("elapsed", time.Since(start)).
		Msg("nhn availability check completed")

	return result, nil
}

