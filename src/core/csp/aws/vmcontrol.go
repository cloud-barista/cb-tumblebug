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
	csp.RegisterBatchVMControlHandlers(csptypes.AWS, csp.BatchVMControlHandlers{
		Suspend:   BatchStopInstances,
		Resume:    BatchStartInstances,
		Terminate: BatchTerminateInstances,
	})
}

// batchControlSize is the max number of instance IDs per EC2 control API call.
// AWS does not document a hard limit for Stop/Start/Terminate, but 200 keeps
// HTTP request sizes manageable and avoids hitting undocumented service limits.
const batchControlSize = 200

// newEC2Client creates an EC2 client for the given region using credentials from ctx.
func newEC2Client(ctx context.Context, region string) (*ec2.Client, error) {
	accessKey, secretKey, err := getAWSCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS vmcontrol: cannot get credentials: %w", err)
	}
	cfg := awssdk.Config{
		Region:      region,
		Credentials: awscreds.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}
	return ec2.NewFromConfig(cfg), nil
}

// BatchStopInstances issues StopInstances for the given instance IDs and returns
// a map of instanceId → model.StatusSuspending for each accepted instance.
func BatchStopInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(instanceIds))
	for i := 0; i < len(instanceIds); i += batchControlSize {
		end := i + batchControlSize
		if end > len(instanceIds) {
			end = len(instanceIds)
		}
		out, err := client.StopInstances(ctx, &ec2.StopInstancesInput{
			InstanceIds: instanceIds[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("StopInstances failed (region=%s, count=%d): %w", region, end-i, err)
		}
		for _, s := range out.StoppingInstances {
			if s.InstanceId != nil {
				result[*s.InstanceId] = model.StatusSuspending
			}
		}
	}

	log.Debug().Str("region", region).Int("sent", len(instanceIds)).Int("accepted", len(result)).
		Msg("[AWS] BatchStopInstances completed")
	return result, nil
}

// BatchStartInstances issues StartInstances for the given instance IDs and returns
// a map of instanceId → model.StatusResuming for each accepted instance.
func BatchStartInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(instanceIds))
	for i := 0; i < len(instanceIds); i += batchControlSize {
		end := i + batchControlSize
		if end > len(instanceIds) {
			end = len(instanceIds)
		}
		out, err := client.StartInstances(ctx, &ec2.StartInstancesInput{
			InstanceIds: instanceIds[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("StartInstances failed (region=%s, count=%d): %w", region, end-i, err)
		}
		for _, s := range out.StartingInstances {
			if s.InstanceId != nil {
				result[*s.InstanceId] = model.StatusResuming
			}
		}
	}

	log.Debug().Str("region", region).Int("sent", len(instanceIds)).Int("accepted", len(result)).
		Msg("[AWS] BatchStartInstances completed")
	return result, nil
}

// BatchTerminateInstances issues TerminateInstances for the given instance IDs and returns
// a map of instanceId → model.StatusTerminating for each accepted instance.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	if len(instanceIds) == 0 {
		return map[string]string{}, nil
	}
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(instanceIds))
	for i := 0; i < len(instanceIds); i += batchControlSize {
		end := i + batchControlSize
		if end > len(instanceIds) {
			end = len(instanceIds)
		}
		out, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: instanceIds[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("TerminateInstances failed (region=%s, count=%d): %w", region, end-i, err)
		}
		for _, s := range out.TerminatingInstances {
			if s.InstanceId != nil {
				result[*s.InstanceId] = model.StatusTerminating
			}
		}
	}

	log.Debug().Str("region", region).Int("sent", len(instanceIds)).Int("accepted", len(result)).
		Msg("[AWS] BatchTerminateInstances completed")
	return result, nil
}
