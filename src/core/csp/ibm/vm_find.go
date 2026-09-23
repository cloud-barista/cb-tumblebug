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

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.IBM, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on IBM Cloud VPC
// by matching instance Name against uids.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	svc, err := newVPCService(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("IBM vmfind: cannot create service: %w", err)
	}

	result := make(map[string]string)

	for _, uid := range uids {
		opts := &vpcv1.ListInstancesOptions{
			Name: core.StringPtr(uid),
		}
		coll, _, err := svc.ListInstancesWithContext(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("IBM ListInstances by name failed (region=%s, uid=%s): %w", region, uid, err)
		}
		if coll != nil {
			for _, inst := range coll.Instances {
				if inst.Name != nil && *inst.Name == uid && inst.ID != nil {
					result[uid] = *inst.ID
					break
				}
			}
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[IBM] FindInstancesByUIDs completed")

	return result, nil
}
