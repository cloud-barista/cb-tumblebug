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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.AWS, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on AWS EC2
// by filtering on tag:Name (which matches Node.Uid) and excluding terminated instances.
// Batches up to 200 UIDs per DescribeInstances call.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	accessKey, secretKey, err := getAWSCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS vmfind: cannot get credentials: %w", err)
	}

	client := ec2.NewFromConfig(newConfig(region, accessKey, secretKey))
	result := make(map[string]string)

	for i := 0; i < len(uids); i += describeInstancesBatchSize {
		end := min(i+describeInstancesBatchSize, len(uids))
		batch := uids[i:end]

		out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("tag:Name"), Values: batch},
				{Name: aws.String("instance-state-name"), Values: []string{"pending", "running", "shutting-down", "stopping", "stopped"}},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("AWS DescribeInstances by tag:Name failed (region=%s, uids=%d): %w", region, len(batch), err)
		}

		for _, reservation := range out.Reservations {
			for _, instance := range reservation.Instances {
				if instance.InstanceId == nil {
					continue
				}
				for _, tag := range instance.Tags {
					if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
						result[*tag.Value] = *instance.InstanceId
					}
				}
			}
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[AWS] FindInstancesByUIDs completed")

	return result, nil
}
