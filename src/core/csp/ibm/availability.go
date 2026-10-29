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

package ibm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterAvailabilityChecker(&ibmAvailabilityChecker{})
}

type ibmAvailabilityChecker struct{}

func (c *ibmAvailabilityChecker) Provider() string { return csptypes.IBM }

// CheckInstance queries IBM Cloud VPC GetInstanceProfile for the given
// (region, instance profile name) and returns whether the profile is supported
// and which zones in the region offer it.
func (c *ibmAvailabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	svc, err := newVPCService(ctx, region)
	if err != nil {
		return model.AvailabilityResult{}, err
	}

	start := time.Now()
	profile, detailResp, err := svc.GetInstanceProfileWithContext(ctx, &vpcv1.GetInstanceProfileOptions{
		Name: core.StringPtr(instanceType),
	})
	elapsed := time.Since(start)

	result := model.AvailabilityResult{
		Provider:     csptypes.IBM,
		Region:       region,
		InstanceType: instanceType,
		Source:       "ibm:GetInstanceProfile",
		QueriedAt:    time.Now(),
	}

	if err != nil {
		if detailResp != nil && detailResp.StatusCode == 404 {
			result.Available = false
			result.Status = model.AvailabilityUnsupported
			result.Reason = fmt.Sprintf("IBM VPC instance profile %q is not offered in region %q", instanceType, region)
			return result, nil
		}
		return model.AvailabilityResult{}, fmt.Errorf("IBM GetInstanceProfile(region=%s, profile=%s) failed after %s: %w",
			region, instanceType, elapsed, err)
	}

	if profile == nil {
		result.Available = false
		result.Status = model.AvailabilityUnsupported
		result.Reason = fmt.Sprintf("IBM VPC instance profile %q returned empty response in region %q", instanceType, region)
		return result, nil
	}

	status := str(profile.Status)
	if status == "" {
		status = "current"
	}

	for _, z := range profile.Zones {
		zoneName := str(z.Name)
		if zoneName == "" {
			continue
		}
		if q.PreferredZone != "" && !strings.EqualFold(zoneName, q.PreferredZone) {
			continue
		}
		result.Zones = append(result.Zones, model.ZoneAvailability{
			ZoneId:    zoneName,
			Status:    status,
			Available: true,
		})
		result.Available = true
	}

	if !result.Available && len(profile.Zones) == 0 && q.PreferredZone == "" {
		// Profile exists in region but no per-zone list was returned.
		result.Available = true
	}

	if result.Available {
		result.Status = model.AvailabilityAvailable
	} else {
		result.Status = model.AvailabilityUnsupported
		result.Reason = fmt.Sprintf("IBM VPC instance profile %q is not supported in the requested zone of region %q", instanceType, region)
	}

	log.Debug().
		Str("provider", csptypes.IBM).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Str("status", string(result.Status)).
		Int("zoneCount", len(result.Zones)).
		Dur("elapsed", elapsed).
		Msg("ibm availability check completed")

	return result, nil
}

