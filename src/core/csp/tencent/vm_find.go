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

import (
	"context"
	"fmt"

	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.Tencent, FindInstancesByUIDs)
}

func ptrVal[T any](v T) *T {
	return &v
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on Tencent CVM
// by filtering on instance-name (which matches Node.Uid).
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	secretID, secretKey, err := getTencentCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Tencent vmfind: cannot get credentials: %w", err)
	}

	client, err := newCVMClient(region, secretID, secretKey)
	if err != nil {
		return nil, fmt.Errorf("Tencent vmfind: failed to create CVM client (region=%s): %w", region, err)
	}

	result := make(map[string]string)

	for i := 0; i < len(uids); i += tencentBatchSize {
		end := min(i+tencentBatchSize, len(uids))
		batch := uids[i:end]

		req := cvm.NewDescribeInstancesRequest()
		ptrs := make([]*string, len(batch))
		for j := range batch {
			ptrs[j] = ptrVal(batch[j])
		}
		req.Filters = []*cvm.Filter{
			{
				Name:   ptrVal("instance-name"),
				Values: ptrs,
			},
		}
		req.Limit = ptrVal(int64(len(batch)))

		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("Tencent DescribeInstances by instance-name failed (region=%s): %w", region, err)
		}

		if resp.Response != nil {
			for _, inst := range resp.Response.InstanceSet {
				if inst.InstanceId == nil || inst.InstanceName == nil {
					continue
				}
				result[*inst.InstanceName] = *inst.InstanceId
			}
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[Tencent] FindInstancesByUIDs completed")

	return result, nil
}
