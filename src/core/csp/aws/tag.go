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
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

// ec2TaggableTypes lists CB-Tumblebug resource types whose cspResourceId (from CB-Spider IId.SystemId)
// is a valid EC2 resource ID (e.g., i-xxx, vpc-xxx, subnet-xxx, sg-xxx, vol-xxx).
// Resource types NOT listed here either:
//   - store a CB-Spider NameId as cspResourceId (e.g., sshKey stores the key name, not key-xxx)
//   - are not EC2 resources (e.g., nlb, k8s)
var ec2TaggableTypes = map[string]bool{
	model.StrNode:          true, // EC2 instance ID (i-xxx)
	model.StrVNet:          true, // VPC ID (vpc-xxx)
	model.StrSubnet:        true, // Subnet ID (subnet-xxx)
	model.StrSecurityGroup: true, // Security Group ID (sg-xxx)
	model.StrDataDisk:      true, // EBS Volume ID (vol-xxx)
}

func init() {
	csp.RegisterBatchTagHandler(csptypes.AWS, BatchUpsertTags)
}

// BatchUpsertTags sets multiple tags on an AWS EC2 resource in a single CreateTags call.
// Only resource types in ec2TaggableTypes are handled; others fall back to CB-Spider.
func BatchUpsertTags(ctx context.Context, region, zone, cspResourceId, resourceType string, tags map[string]string) error {
	if !ec2TaggableTypes[resourceType] {
		return fmt.Errorf("resource type %q is not EC2-taggable via batch (cspResourceId=%s)", resourceType, cspResourceId)
	}

	accessKey, secretKey, err := getAWSCreds(ctx)
	if err != nil {
		return fmt.Errorf("failed to get AWS credentials: %w", err)
	}

	cfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}
	client := ec2.NewFromConfig(cfg)

	// Build EC2 tag list
	ec2Tags := make([]ec2types.Tag, 0, len(tags))
	for k, v := range tags {
		ec2Tags = append(ec2Tags, ec2types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	_, err = client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{cspResourceId},
		Tags:      ec2Tags,
	})
	if err != nil {
		return fmt.Errorf("AWS EC2 CreateTags failed for %s: %w", cspResourceId, err)
	}

	log.Debug().
		Str("region", region).
		Str("resourceId", cspResourceId).
		Int("tagCount", len(tags)).
		Msg("[AWS] Batch tags upserted via EC2 CreateTags")

	return nil
}

// getAWSCreds retrieves AWS credentials from OpenBao.
func getAWSCreds(ctx context.Context) (accessKey, secretKey string, err error) {
	path := csp.BuildSecretPath(ctx, "aws")
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return "", "", err
	}

	accessKey = csp.GetString(data, "AWS_ACCESS_KEY_ID")
	secretKey = csp.GetString(data, "AWS_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return "", "", fmt.Errorf("AWS credentials incomplete at %s", path)
	}
	return accessKey, secretKey, nil
}






