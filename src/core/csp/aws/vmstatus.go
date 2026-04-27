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

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterBatchVMStatusHandler(csptypes.AWS, BatchDescribeInstanceStatuses)
}

// describeInstancesBatchSize is the maximum number of instance IDs per DescribeInstances call.
// AWS EC2 API allows up to 1000 per call; 200 keeps requests well within limits.
const describeInstancesBatchSize = 200

// BatchDescribeInstanceStatuses queries EC2 DescribeInstances for the given AWS instance IDs
// (e.g. "i-0abc123def456") and returns a map of instanceId → TB status string.
//
// Requests are batched in groups of 200 and fired sequentially within one call.
// The EC2 client reuses a pooled HTTP transport (no SetCloseConnection), so
// repeated calls across goroutines share keep-alive connections to AWS.
//
// Instance IDs not found in AWS are omitted from the returned map; callers
// should treat a missing key as model.StatusUndefined.
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}

	accessKey, secretKey, err := getAWSCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS vmstatus: cannot get credentials: %w", err)
	}

	cfg := awssdk.Config{
		Region:      region,
		Credentials: awscreds.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}
	client := ec2.NewFromConfig(cfg)

	result := make(map[string]string, len(instanceIds))

	for i := 0; i < len(instanceIds); i += describeInstancesBatchSize {
		end := i + describeInstancesBatchSize
		if end > len(instanceIds) {
			end = len(instanceIds)
		}
		batch := instanceIds[i:end]

		out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("DescribeInstances failed (region=%s, ids=%d): %w", region, len(batch), err)
		}

		for _, reservation := range out.Reservations {
			for _, instance := range reservation.Instances {
				if instance.InstanceId == nil || instance.State == nil {
					continue
				}
				result[*instance.InstanceId] = awsStateToTBStatus(string(instance.State.Name))
			}
		}
	}

	log.Trace().
		Str("region", region).
		Int("queried", len(instanceIds)).
		Int("found", len(result)).
		Msg("[AWS] BatchDescribeInstanceStatuses completed")

	return result, nil
}

// awsStateToTBStatus maps an AWS EC2 instance state name to the equivalent TB status string.
func awsStateToTBStatus(awsState string) string {
	switch awsState {
	case "pending":
		return model.StatusCreating
	case "running":
		return model.StatusRunning
	case "shutting-down":
		return model.StatusTerminating
	case "terminated":
		return model.StatusTerminated
	case "stopping":
		return model.StatusSuspending
	case "stopped":
		return model.StatusSuspended
	default:
		return model.StatusUndefined
	}
}
