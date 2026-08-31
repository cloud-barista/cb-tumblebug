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

// This file implements csp.FailureParser for Tencent Cloud CVM.
//
// Tencent errors carry a dotted code and a request id, and name neither the
// zone that failed nor an alternative:
//
//	[TencentCloudSDKError] Code=InvalidImageId.NotFound,
//	Message=ImageId img-7rotv4ux is not found, RequestId=ac88e205-...

import (
	"regexp"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

func init() {
	csp.RegisterFailureParser(&failureParser{})
}

type failureParser struct{}

func (p *failureParser) Provider() string { return cspconst.Tencent }

var (
	tencentCodeRe      = regexp.MustCompile(`Code=([A-Za-z][A-Za-z0-9._]*)`)
	tencentRequestIdRe = regexp.MustCompile(`RequestId=([0-9a-fA-F-]{8,})`)
)

// Parse recognizes Tencent SDK error strings. Returns ok=false without a
// Code= field so the generic classifier can try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	m := tencentCodeRe.FindStringSubmatch(raw)
	if m == nil {
		return model.ProvisioningFailure{}, false
	}
	code := m[1]
	lower := strings.ToLower(code)

	f := model.ProvisioningFailure{CspErrorCode: code}

	switch {
	case strings.HasPrefix(lower, "resourceinsufficient"),
		strings.HasPrefix(lower, "resourcesoldout"),
		strings.Contains(lower, "zonesoldout"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone

	case strings.HasPrefix(lower, "limitexceeded"), strings.Contains(lower, "quota"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case strings.HasPrefix(lower, "requestlimitexceeded"):
		f.Class = model.FailureThrottling
		f.Retryable = true
		f.RetryHint = model.RetryHintWaitAndRetry

	case strings.HasPrefix(lower, "authfailure"), strings.Contains(lower, "unauthorized"):
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case strings.HasPrefix(lower, "invalidimageid"), strings.HasPrefix(lower, "invalidinstancetype"):
		f.Class = model.FailureImageSpecMismatch
		f.Retryable = false
		f.RetryHint = model.RetryHintDifferentImage

	case strings.HasPrefix(lower, "invalidparameter"), strings.HasPrefix(lower, "missingparameter"),
		strings.HasPrefix(lower, "invalidzone"):
		f.Class = model.FailureInvalidRequest
		f.Retryable = false
		f.RetryHint = model.RetryHintAdjustRequest

	default:
		return model.ProvisioningFailure{}, false
	}

	if r := tencentRequestIdRe.FindStringSubmatch(raw); r != nil {
		f.RequestId = r[1]
	}
	return f, true
}
