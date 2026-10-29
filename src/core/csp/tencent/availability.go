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

package tencent

// This file implements csp.AvailabilityChecker for Tencent CVM using the
// DescribeZoneInstanceConfigInfos API. The API answers, for a given
// (region, instance-type[, zone]) tuple, which zones currently have stock
// for the requested instance type and the per-zone sell status.
//
// Rationale:
//   - The most common Tencent VM-creation failure pattern observed in
//     practice is "ResourceInsufficient.SpecifiedInstanceType ... The
//     specified type of instance is understocked." This is a transient
//     stock issue that pure spec-catalog checks cannot detect ahead of
//     time.
//   - DescribeZoneInstanceConfigInfos returns per-zone instance
//     configurations whose Status field ("SELL" / "SOLD_OUT") directly
//     surfaces the same stock state.
//
// Note: Tencent's API does not return the system-disk × zone matrix in the
// same call (unlike Alibaba's DescribeAvailableResource). SupportedDisks
// is therefore left unset; the primary failure mode this checker prevents
// is instance-type stock exhaustion.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

func init() {
	csp.RegisterAvailabilityChecker(&availabilityChecker{})
}

type availabilityChecker struct{}

func (c *availabilityChecker) Provider() string { return cspconst.Tencent }

type tencentQuotaEntry struct {
	postPaidRemainingByZone map[string]uint64
	postPaidLimitByZone     map[string]uint64
	fetchedAt               time.Time
}

var (
	tencentQuotaCache   = make(map[string]*tencentQuotaEntry)
	tencentQuotaCacheMu sync.RWMutex
	tencentQuotaSF      singleflight.Group
	tencentQuotaTTL     = 2 * time.Minute
)

func getTencentPostPaidQuotas(ctx context.Context, client *cvm.Client, secretID, region string) (*tencentQuotaEntry, error) {
	cacheKey := csp.CredKey(secretID) + "+" + region
	tencentQuotaCacheMu.RLock()
	entry, ok := tencentQuotaCache[cacheKey]
	tencentQuotaCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < tencentQuotaTTL {
		return entry, nil
	}

	v, err, _ := tencentQuotaSF.Do(cacheKey, func() (any, error) {
		tencentQuotaCacheMu.RLock()
		entry, ok := tencentQuotaCache[cacheKey]
		tencentQuotaCacheMu.RUnlock()
		if ok && time.Since(entry.fetchedAt) < tencentQuotaTTL {
			return entry, nil
		}

		qReq := cvm.NewDescribeAccountQuotaRequest()
		qResp, err := client.DescribeAccountQuotaWithContext(ctx, qReq)
		if err != nil {
			return nil, err
		}
		rem := make(map[string]uint64)
		lim := make(map[string]uint64)
		if qResp != nil && qResp.Response != nil && qResp.Response.AccountQuotaOverview != nil && qResp.Response.AccountQuotaOverview.AccountQuota != nil {
			for _, pq := range qResp.Response.AccountQuotaOverview.AccountQuota.PostPaidQuotaSet {
				if pq == nil || pq.Zone == nil {
					continue
				}
				z := *pq.Zone
				if pq.RemainingQuota != nil {
					rem[z] = *pq.RemainingQuota
				}
				if pq.TotalQuota != nil {
					lim[z] = *pq.TotalQuota
				}
			}
		}
		e := &tencentQuotaEntry{
			postPaidRemainingByZone: rem,
			postPaidLimitByZone:     lim,
			fetchedAt:               time.Now(),
		}
		tencentQuotaCacheMu.Lock()
		tencentQuotaCache[cacheKey] = e
		tencentQuotaCacheMu.Unlock()
		return e, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*tencentQuotaEntry), nil
}

// CheckInstance queries Tencent CVM DescribeZoneInstanceConfigInfos for the
// given (region, instance type, optional zone) and returns per-zone
// availability based on the Status field ("SELL" / "SOLD_OUT") and
// DescribeAccountQuota (PostPaidQuotaSet).
func (c *availabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	secretID, secretKey, err := getTencentCreds(ctx)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to get Tencent credentials: %w", err)
	}

	client, err := newCVMClient(region, secretID, secretKey)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to create Tencent CVM client for region %s: %w", region, err)
	}

	req := cvm.NewDescribeZoneInstanceConfigInfosRequest()
	filters := []*cvm.Filter{
		{
			Name:   new("instance-type"),
			Values: tccommon.StringPtrs([]string{instanceType}),
		},
	}
	if zone := strings.TrimSpace(q.PreferredZone); zone != "" {
		filters = append(filters, &cvm.Filter{
			Name:   new("zone"),
			Values: tccommon.StringPtrs([]string{zone}),
		})
	}
	req.Filters = filters

	start := time.Now()
	resp, err := client.DescribeZoneInstanceConfigInfosWithContext(ctx, req)
	elapsed := time.Since(start)
	if err != nil {
		if sdkErr, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			return model.AvailabilityResult{}, fmt.Errorf("DescribeZoneInstanceConfigInfos(region=%s, instanceType=%s) failed after %s: code=%s, message=%s",
				region, instanceType, elapsed, sdkErr.Code, sdkErr.Message)
		}
		return model.AvailabilityResult{}, fmt.Errorf("DescribeZoneInstanceConfigInfos(region=%s, instanceType=%s) failed after %s: %w",
			region, instanceType, elapsed, err)
	}

	result := model.AvailabilityResult{
		Provider:     cspconst.Tencent,
		Region:       region,
		InstanceType: instanceType,
		Source:       "tencent:DescribeZoneInstanceConfigInfos",
		QueriedAt:    time.Now(),
	}

	var items []*cvm.InstanceTypeQuotaItem
	if resp != nil && resp.Response != nil {
		items = resp.Response.InstanceTypeQuotaSet
	}

	// Prefer POSTPAID_BY_HOUR items for on-demand VM provisioning so SPOTPAID stock
	// does not mask an on-demand SOLD_OUT state.
	var postPaidItems []*cvm.InstanceTypeQuotaItem
	for _, item := range items {
		if item != nil && item.InstanceChargeType != nil && strings.EqualFold(*item.InstanceChargeType, "POSTPAID_BY_HOUR") {
			postPaidItems = append(postPaidItems, item)
		}
	}
	if len(postPaidItems) > 0 {
		items = postPaidItems
	}

	quotaEntry, _ := getTencentPostPaidQuotas(ctx, client, secretID, region)
	quotaExhaustedZones := 0

	zoneIndex := map[string]int{}
	for _, item := range items {
		if item == nil || item.Zone == nil {
			continue
		}
		zoneID := *item.Zone
		status := ""
		if item.Status != nil {
			status = *item.Status
		}
		zoneOk := strings.EqualFold(status, "SELL")
		reason := ""
		if !zoneOk {
			reason = fmt.Sprintf("zone status=%q", status)
		} else if quotaEntry != nil {
			if rem, hasQuota := quotaEntry.postPaidRemainingByZone[zoneID]; hasQuota && rem == 0 {
				zoneOk = false
				status = "QUOTA_EXCEEDED"
				reason = fmt.Sprintf("postpaid quota exhausted in zone %s (remaining=0 / limit=%d)", zoneID, quotaEntry.postPaidLimitByZone[zoneID])
				quotaExhaustedZones++
			}
		}

		if idx, ok := zoneIndex[zoneID]; ok {
			z := result.Zones[idx]
			if zoneOk {
				z.Available = true
				z.Status = status
				z.Reason = ""
			}
			result.Zones[idx] = z
		} else {
			z := model.ZoneAvailability{
				ZoneId:    zoneID,
				Status:    status,
				Available: zoneOk,
				Reason:    reason,
			}
			zoneIndex[zoneID] = len(result.Zones)
			result.Zones = append(result.Zones, z)
		}
		if zoneOk {
			result.Available = true
		}
	}

	if result.Available {
		result.Status = model.AvailabilityAvailable
	} else if quotaExhaustedZones > 0 && quotaExhaustedZones == len(result.Zones) {
		result.Status = model.AvailabilityQuotaExceeded
		result.Reason = fmt.Sprintf("Tencent postpaid instance quota is exhausted across all zones for %q in region %q.", instanceType, region)
	} else {
		result.Status = model.AvailabilitySoldOut
		if len(result.Zones) == 0 {
			result.Reason = fmt.Sprintf("Tencent reports no zones for instance type %q in region %q right now. "+
				"This usually means temporary out-of-stock for this instance family in this region; "+
				"try a different region, a different instance type, or retry later.",
				instanceType, region)
		} else {
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
			result.Reason = fmt.Sprintf("Tencent reports no zone with stock for instance type %q in region %q right now%s. "+
				"This is typically temporary out-of-stock; try a different region, a different instance type, or retry later.",
				instanceType, region, zoneSummary)
		}
	}

	log.Debug().
		Str("provider", cspconst.Tencent).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Str("status", string(result.Status)).
		Int("zoneCount", len(result.Zones)).
		Dur("elapsed", elapsed).
		Msg("tencent availability check completed")

	return result, nil
}

func getTencentCreds(ctx context.Context) (secretID, secretKey string, err error) {
	path := csp.BuildSecretPath(ctx, cspconst.Tencent)
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return "", "", err
	}

	secretID = csp.GetString(data, "TENCENTCLOUD_SECRET_ID")
	secretKey = csp.GetString(data, "TENCENTCLOUD_SECRET_KEY")
	if secretID == "" || secretKey == "" {
		return "", "", fmt.Errorf("Tencent credentials incomplete at %s", path)
	}

	return secretID, secretKey, nil
}
