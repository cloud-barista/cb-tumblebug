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

package infra

import (
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// applyQuotaReview warns every NodeGroup whose request, alone or together with the other NodeGroups
// drawing on the same account quota, asks for more than the quota has left, and reports whether it
// warned. It only warns: the figures can be minutes old and a user may be raising the quota at the
// CSP. NodeGroups without quota information are left alone.
func applyQuotaReview(reviews []model.ReviewNodeGroupDynamicReqInfo) bool {
	type demand struct {
		members   []int
		need      int64
		remaining int64
	}
	demands := map[string]*demand{}
	var order []string
	for i := range reviews {
		quota := reviews[i].SpecValidation.Quota
		if quota == nil || quota.PerInstance <= 0 || !reviews[i].CanCreate {
			continue
		}
		key := reviews[i].ConnectionName + "|" + quota.Scope + "|" + quota.Name
		d, ok := demands[key]
		if !ok {
			d = &demand{remaining: quota.Remaining()}
			demands[key] = d
			order = append(order, key)
		}
		d.members = append(d.members, i)
		d.need += int64(max(reviews[i].NodeGroupSize, 1)) * quota.PerInstance
		d.remaining = min(d.remaining, quota.Remaining())
	}

	warned := false
	for _, key := range order {
		d := demands[key]
		if d.need <= d.remaining {
			continue
		}
		for _, i := range d.members {
			review := &reviews[i]
			quota := review.SpecValidation.Quota
			nodes := max(review.NodeGroupSize, 1)
			usage := "usage not reported"
			if quota.Used != nil {
				usage = fmt.Sprintf("used %d", *quota.Used)
			}
			var msg string
			if len(d.members) == 1 {
				msg = fmt.Sprintf("Quota: %d node(s) need %d %s, but %d remain in '%s' (%s %s, limit %d, %s); about %d node(s) fit. "+
					"The rest will fail unless the quota is raised at the CSP",
					nodes, d.need, quota.Unit, d.remaining, quota.Name, review.ProviderName, review.RegionName, quota.Limit, usage, d.remaining/quota.PerInstance)
			} else {
				msg = fmt.Sprintf("Quota: %d NodeGroups in %s %s need %d %s together, but %d remain in '%s' (limit %d, %s); this one asks for %d node(s). "+
					"Nodes beyond the quota will fail unless it is raised at the CSP",
					len(d.members), review.ProviderName, review.RegionName, d.need, quota.Unit, d.remaining, quota.Name, quota.Limit, usage, nodes)
			}
			review.Warnings = append(review.Warnings, msg)
			if review.Status != "Error" {
				review.Status = "Warning"
				review.Message = fmt.Sprintf("VM can be created but has %d warning(s)", len(review.Warnings))
			}
			warned = true
		}
	}
	return warned
}
