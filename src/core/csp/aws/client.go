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
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
)

// awsMaxAttempts is the total attempts per AWS API call (SDK default is 3).
const awsMaxAttempts = 4

// newConfig returns an aws.Config with static credentials and a standard
// retryer, shared by every direct AWS call path.
func newConfig(region, accessKey, secretKey string) awssdk.Config {
	return awssdk.Config{
		Region:      region,
		Credentials: awscreds.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		Retryer: func() awssdk.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = awsMaxAttempts
			})
		},
	}
}
