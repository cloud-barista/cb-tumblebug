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
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tcprofile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

func init() {
	csp.RegisterAvailabilityChecker(&availabilityChecker{})
}

type availabilityChecker struct{}

func (c *availabilityChecker) Provider() string { return cspconst.Tencent }

// CheckInstance queries Tencent CVM DescribeZoneInstanceConfigInfos for the
// given (region, instance type, optional zone) and returns per-zone
// availability based on the Status field ("SELL" / "SOLD_OUT").
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

	credential := tccommon.NewCredential(secretID, secretKey)
	cpf := tcprofile.NewClientProfile()
	client, err := cvm.NewClient(credential, region, cpf)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to create Tencent CVM client for region %s: %w", region, err)
	}

	req := cvm.NewDescribeZoneInstanceConfigInfosRequest()
	filters := []*cvm.Filter{
		{
			Name:   tccommon.StringPtr("instance-type"),
			Values: tccommon.StringPtrs([]string{instanceType}),
		},
	}
	if zone := strings.TrimSpace(q.PreferredZone); zone != "" {
		filters = append(filters, &cvm.Filter{
			Name:   tccommon.StringPtr("zone"),
			Values: tccommon.StringPtrs([]string{zone}),
		})
	}
	req.Filters = filters

	start := time.Now()
	resp, err := client.DescribeZoneInstanceConfigInfosWithContext(ctx, req)
	elapsed := time.Since(start)
	if err != nil {
		// Surface known SDK errors with their code so callers can see them
		// in the dispatcher's "treated as available" warn log.
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

	// One quota item per (zone, instance type, charge type, ...). Multiple
	// items can therefore reference the same zone (e.g., POSTPAID_BY_HOUR
	// vs SPOTPAID); merge them so the zone is "Available" if at least one
	// charge type is sellable.
	zoneIndex := map[string]int{}
	if resp != nil && resp.Response != nil {
		for _, item := range resp.Response.InstanceTypeQuotaSet {
			if item == nil || item.Zone == nil {
				continue
			}
			zoneID := *item.Zone
			status := ""
			if item.Status != nil {
				status = *item.Status
			}
			// Tencent enum: "SELL" (available) / "SOLD_OUT" (out of stock).
			zoneOk := strings.EqualFold(status, "SELL")

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
				}
				if !zoneOk {
					z.Reason = fmt.Sprintf("zone status=%q", status)
				}
				zoneIndex[zoneID] = len(result.Zones)
				result.Zones = append(result.Zones, z)
			}
			if zoneOk {
				result.Available = true
			}
		}
	}

	if !result.Available {
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
