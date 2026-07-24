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
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

// newECSClient creates an ECS client with timeouts raised above the SDK
// defaults (5s connect / 10s read), which are too tight for high-latency paths.
func newECSClient(region, accessKeyID, accessKeySecret string) (*ecs.Client, error) {
	client, err := ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}
	client.SetConnectTimeout(15 * time.Second)
	client.SetReadTimeout(60 * time.Second)
	return client, nil
}
