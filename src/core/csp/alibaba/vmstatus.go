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
	"encoding/json"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchVMStatusHandler(csptypes.Alibaba, BatchDescribeInstanceStatuses)
}

// alibabaBatchSize is the maximum number of instance IDs per DescribeInstances call.
const alibabaBatchSize = 100

// BatchDescribeInstanceStatuses queries Alibaba ECS DescribeInstances for the given
// instance IDs and returns a map of instanceId → TB status string.
// Requests are batched in groups of 100.
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	accessKeyID, accessKeySecret, err := getAlibabaCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Alibaba vmstatus: cannot get credentials: %w", err)
	}

	client, err := ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("Alibaba vmstatus: failed to create ECS client (region=%s): %w", region, err)
	}

	result := make(map[string]string, len(instanceIds))

	for i := 0; i < len(instanceIds); i += alibabaBatchSize {
		end := i + alibabaBatchSize
		if end > len(instanceIds) {
			end = len(instanceIds)
		}
		batch := instanceIds[i:end]

		// Alibaba DescribeInstances takes InstanceIds as a JSON-encoded array string.
		idsJSON, err := json.Marshal(batch)
		if err != nil {
			return nil, fmt.Errorf("Alibaba vmstatus: failed to encode instance IDs: %w", err)
		}

		req := ecs.CreateDescribeInstancesRequest()
		req.RegionId = region
		req.InstanceIds = string(idsJSON)
		req.PageSize = "100"

		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("Alibaba DescribeInstances failed (region=%s): %w", region, err)
		}

		for _, inst := range resp.Instances.Instance {
			result[inst.InstanceId] = alibabaStateToTBStatus(inst.Status)
		}
	}

	log.Trace().
		Str("region", region).
		Int("queried", len(instanceIds)).
		Int("found", len(result)).
		Msg("[Alibaba] BatchDescribeInstanceStatuses completed")

	return result, nil
}

// alibabaStateToTBStatus maps Alibaba ECS instance status strings to TB status strings.
func alibabaStateToTBStatus(state string) string {
	switch state {
	case "Pending", "Starting":
		return model.StatusCreating
	case "Running":
		return model.StatusRunning
	case "Stopping":
		return model.StatusSuspending
	case "Stopped":
		return model.StatusSuspended
	default:
		return model.StatusUndefined
	}
}
