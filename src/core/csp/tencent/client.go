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
	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
)

// newCVMClient creates a CVM client with network-failure retry enabled.
// All CVM calls here are read-only Describe* queries, so retry is safe.
func newCVMClient(region, secretID, secretKey string) (*cvm.Client, error) {
	credential := tccommon.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.NetworkFailureMaxRetries = 2
	cpf.NetworkFailureRetryDuration = profile.ExponentialBackoff
	cpf.UnsafeRetryOnConnectionFailure = true
	return cvm.NewClient(credential, region, cpf)
}
