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

	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchVMStatusHandler(csptypes.Tencent, BatchDescribeInstanceStatuses)
}

// tencentBatchSize is the maximum number of instance IDs per DescribeInstances call.
const tencentBatchSize = 100

// BatchDescribeInstanceStatuses queries Tencent CVM DescribeInstances for the given
// instance IDs and returns a map of instanceId → TB status string.
// Requests are batched in groups of 100.
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	secretID, secretKey, err := getTencentCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Tencent vmstatus: cannot get credentials: %w", err)
	}

	credential := tccommon.NewCredential(secretID, secretKey)
	client, err := cvm.NewClient(credential, region, profile.NewClientProfile())
	if err != nil {
		return nil, fmt.Errorf("Tencent vmstatus: failed to create CVM client (region=%s): %w", region, err)
	}

	result := make(map[string]string, len(instanceIds))

	for i := 0; i < len(instanceIds); i += tencentBatchSize {
		end := i + tencentBatchSize
		if end > len(instanceIds) {
			end = len(instanceIds)
		}
		batch := instanceIds[i:end]

		req := cvm.NewDescribeInstancesRequest()
		ptrs := make([]*string, len(batch))
		for j := range batch {
			ptrs[j] = tccommon.StringPtr(batch[j])
		}
		req.InstanceIds = ptrs

		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("Tencent DescribeInstances failed (region=%s): %w", region, err)
		}

		for _, inst := range resp.Response.InstanceSet {
			if inst.InstanceId == nil || inst.InstanceState == nil {
				continue
			}
			result[*inst.InstanceId] = tencentStateToTBStatus(*inst.InstanceState)
		}
	}

	log.Trace().
		Str("region", region).
		Int("queried", len(instanceIds)).
		Int("found", len(result)).
		Msg("[Tencent] BatchDescribeInstanceStatuses completed")

	return result, nil
}

// tencentStateToTBStatus maps Tencent CVM instance state strings to TB status strings.
func tencentStateToTBStatus(state string) string {
	switch state {
	case "PENDING", "STARTING":
		return model.StatusCreating
	case "RUNNING":
		return model.StatusRunning
	case "STOPPING":
		return model.StatusSuspending
	case "STOPPED":
		return model.StatusSuspended
	case "REBOOTING":
		return model.StatusRebooting
	case "SHUTDOWN", "TERMINATING":
		return model.StatusTerminating
	case "LAUNCH_FAILED":
		return model.StatusFailed
	default:
		return model.StatusUndefined
	}
}
