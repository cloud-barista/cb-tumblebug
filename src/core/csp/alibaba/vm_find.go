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

package alibaba

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.Alibaba, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on Alibaba ECS
// by matching InstanceName against uids.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	ak, sk, err := getAlibabaCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Alibaba vmfind: cannot get credentials: %w", err)
	}

	client, err := newECSClient(region, ak, sk)
	if err != nil {
		return nil, fmt.Errorf("Alibaba vmfind: failed to create client (region=%s): %w", region, err)
	}

	result := make(map[string]string)

	for _, uid := range uids {
		req := ecs.CreateDescribeInstancesRequest()
		req.RegionId = region
		req.InstanceName = uid

		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("Alibaba DescribeInstances by InstanceName failed (region=%s, uid=%s): %s", region, uid, csp.RedactErr(err))
		}

		for _, inst := range resp.Instances.Instance {
			if inst.InstanceName == uid {
				result[uid] = inst.InstanceId
				break
			}
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[Alibaba] FindInstancesByUIDs completed")

	return result, nil
}
