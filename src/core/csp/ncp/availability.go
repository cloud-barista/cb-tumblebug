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

package ncp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterAvailabilityChecker(&ncpAvailabilityChecker{})
}

type ncpAvailabilityChecker struct{}

func (c *ncpAvailabilityChecker) Provider() string { return csptypes.NCP }

// CheckInstance queries NCP vserver GetServerSpecList for the given
// (region, server spec code, optional zone) to verify whether the spec
// is supported in that region/zone.
func (c *ncpAvailabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.ToUpper(strings.TrimSpace(q.Region))
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	client, err := newClient(ctx)
	if err != nil {
		return model.AvailabilityResult{}, err
	}

	req := &vserver.GetServerSpecListRequest{
		RegionCode:         ncloud.String(region),
		ServerSpecCodeList: []*string{ncloud.String(instanceType)},
	}
	if zone := strings.TrimSpace(q.PreferredZone); zone != "" {
		req.ZoneCode = ncloud.String(zone)
	}

	start := time.Now()
	resp, err := client.V2Api.GetServerSpecList(req)
	elapsed := time.Since(start)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("NCP GetServerSpecList(region=%s, spec=%s) failed after %s: %w",
			region, instanceType, elapsed, err)
	}

	result := model.AvailabilityResult{
		Provider:     csptypes.NCP,
		Region:       q.Region,
		InstanceType: instanceType,
		Source:       "ncp:GetServerSpecList",
		QueriedAt:    time.Now(),
	}

	found := false
	if resp != nil {
		for _, s := range resp.ServerSpecList {
			if s != nil && s.ServerSpecCode != nil && strings.EqualFold(*s.ServerSpecCode, instanceType) {
				found = true
				break
			}
		}
	}

	if found {
		result.Available = true
		result.Status = model.AvailabilityAvailable
		if zone := strings.TrimSpace(q.PreferredZone); zone != "" {
			result.Zones = []model.ZoneAvailability{
				{
					ZoneId:    zone,
					Status:    "AVAILABLE",
					Available: true,
				},
			}
		}
	} else {
		result.Available = false
		result.Status = model.AvailabilityUnsupported
		result.Reason = fmt.Sprintf("NCP server spec %q is not offered in region %q", instanceType, q.Region)
	}

	log.Debug().
		Str("provider", csptypes.NCP).
		Str("region", q.Region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Str("status", string(result.Status)).
		Dur("elapsed", elapsed).
		Msg("ncp availability check completed")

	return result, nil
}

