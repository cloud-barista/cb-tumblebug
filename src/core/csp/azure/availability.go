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

package azure

// This file plugs the existing Azure spec/quota check (CheckSpecAvailability)
// into the provider-agnostic csp.AvailabilityChecker interface so that callers
// (e.g. ReviewSpecImagePair) can route all CSPs through a single dispatcher.

import (
	"context"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

func init() {
	csp.RegisterAvailabilityChecker(&availabilityChecker{})
}

type availabilityChecker struct{}

func (c *availabilityChecker) Provider() string { return cspconst.Azure }

// CheckInstance wraps CheckSpecAvailability. Azure's API is not zone-aware
// in the same way Alibaba's is for stock, so the result has no per-zone
// breakdown; the boolean Available is sufficient for the pre-flight purpose.
func (c *availabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	specResult, err := CheckSpecAvailability(ctx, q.Region, q.InstanceType)
	if err != nil {
		return model.AvailabilityResult{}, err
	}
	out := model.AvailabilityResult{
		Provider:     cspconst.Azure,
		Region:       q.Region,
		InstanceType: q.InstanceType,
		Available:    specResult.Available,
		Source:       "azure:CheckSpecAvailability",
	}
	if !specResult.Available {
		out.Reason = specResult.Reason
	}
	return out, nil
}
