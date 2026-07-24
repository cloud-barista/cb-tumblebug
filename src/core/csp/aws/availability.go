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

package aws

// This file implements csp.AvailabilityChecker for AWS EC2 using the
// DescribeInstanceTypeOfferings API. The API answers, for a given
// (region, instance-type), which availability zones support that instance type.
//
// Rationale:
//   - The most common AWS VM-creation failure for zone-specific requests is
//     "InsufficientInstanceCapacity: We currently do not have sufficient
//     capacity in the Availability Zone you requested."
//   - DescribeInstanceTypeOfferings returns the set of AZs where a given
//     instance type is offered. Combined with zone retry, this allows
//     autopilot to proactively suggest an AZ that supports the instance type.
//
// Note: DescribeInstanceTypeOfferings reports whether an instance type is
// *offered* in an AZ, not real-time capacity. Capacity fluctuates; this
// check reduces (but does not eliminate) zone-level failures.

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterAvailabilityChecker(&availabilityChecker{})
}

type availabilityChecker struct{}

func (c *availabilityChecker) Provider() string { return cspconst.AWS }

// CheckInstance queries EC2 DescribeInstanceTypeOfferings for the given
// (region, instance type) and returns per-AZ availability based on whether
// the instance type is offered there.
func (c *availabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if region == "" {
		return model.AvailabilityResult{}, fmt.Errorf("region is empty")
	}
	if instanceType == "" {
		return model.AvailabilityResult{}, fmt.Errorf("instance type is empty")
	}

	accessKey, secretKey, err := getAWSCreds(ctx)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("failed to get AWS credentials: %w", err)
	}

	client := ec2.NewFromConfig(newConfig(region, accessKey, secretKey))

	input := &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: ec2types.LocationTypeAvailabilityZone,
		Filters: []ec2types.Filter{
			{
				Name:   awssdk.String("instance-type"),
				Values: []string{instanceType},
			},
		},
	}

	// If a preferred zone is specified, filter to that zone only to check quickly.
	if q.PreferredZone != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   awssdk.String("location"),
			Values: []string{q.PreferredZone},
		})
	}

	start := time.Now()
	resp, err := client.DescribeInstanceTypeOfferings(ctx, input)
	elapsed := time.Since(start)
	if err != nil {
		return model.AvailabilityResult{}, fmt.Errorf("DescribeInstanceTypeOfferings(region=%s, instanceType=%s) failed after %s: %w",
			region, instanceType, elapsed, err)
	}

	result := model.AvailabilityResult{
		Provider:     cspconst.AWS,
		Region:       region,
		InstanceType: instanceType,
		Source:       "aws:DescribeInstanceTypeOfferings",
		QueriedAt:    time.Now(),
	}

	for _, offering := range resp.InstanceTypeOfferings {
		if offering.Location == nil {
			continue
		}
		zoneID := *offering.Location
		// DescribeInstanceTypeOfferings only returns AZs where the type IS offered;
		// presence in the list means available.
		z := model.ZoneAvailability{
			ZoneId:    zoneID,
			Status:    "AVAILABLE",
			Available: true,
		}
		result.Zones = append(result.Zones, z)
		result.Available = true
	}

	if !result.Available {
		result.Reason = fmt.Sprintf("instance type %q is not offered in any AZ of region %q "+
			"(DescribeInstanceTypeOfferings returned no results). "+
			"The instance type may not be supported in this region.",
			instanceType, region)
	}

	log.Debug().
		Str("provider", cspconst.AWS).
		Str("region", region).
		Str("instanceType", instanceType).
		Bool("available", result.Available).
		Int("zoneCount", len(result.Zones)).
		Dur("elapsed", elapsed).
		Msg("aws availability check completed")

	return result, nil
}
