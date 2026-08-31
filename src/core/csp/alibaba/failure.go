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

// This file implements csp.FailureParser for Alibaba Cloud ECS.
//
// Alibaba reports the least of the three providers with observed samples: it
// says a zone ran out but never which one, and offers no alternatives:
//
//	SDK.ServerError ErrorCode: OperationDenied.NoStock
//	Recommend: https://api.alibabacloud.com/troubleshoot?...
//	RequestId: 01A046FB-... Message: The resource is out of stock in the
//	specified zone. Please try other types, or choose other regions and zones.
//
// So ReportedZone and SuggestedZones stay empty here and the retry path relies
// on ProvisioningFailure.AttemptedZone, which CB-Tumblebug records itself.

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

func (p *failureParser) Provider() string { return cspconst.Alibaba }

var (
	aliErrCodeRe   = regexp.MustCompile(`ErrorCode:\s*([A-Za-z][A-Za-z0-9._]*)`)
	aliRequestIdRe = regexp.MustCompile(`RequestId:\s*([0-9A-Fa-f-]{8,})`)
)

// Parse recognizes Alibaba SDK error strings. Returns ok=false without an
// ErrorCode so the generic classifier can try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	m := aliErrCodeRe.FindStringSubmatch(raw)
	if m == nil {
		return model.ProvisioningFailure{}, false
	}
	code := m[1]
	lower := strings.ToLower(code)

	f := model.ProvisioningFailure{CspErrorCode: code}

	switch {
	case strings.Contains(lower, "nostock"), strings.Contains(lower, "resourcenotavailable"),
		strings.Contains(lower, "zonenotopen"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone

	case strings.Contains(lower, "quotaexceed"), strings.Contains(lower, "instancequotaexceed"),
		strings.Contains(lower, "limitexceed"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case strings.Contains(lower, "throttling"), strings.Contains(lower, "requestlimitexceeded"):
		f.Class = model.FailureThrottling
		f.Retryable = true
		f.RetryHint = model.RetryHintWaitAndRetry

	case strings.Contains(lower, "forbidden"), strings.Contains(lower, "invalidaccesskey"),
		strings.Contains(lower, "signaturedoesnotmatch"):
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case strings.Contains(lower, "invalidsystemdiskcategory"),
		strings.Contains(lower, "diskcategory"):
		f.Class = model.FailureDiskTypeUnavailable
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentSpec

	// InvalidParameter.NotMatch is how Alibaba reports a spec that accepts only
	// certain images ("only supports some specific images"), not a bad literal.
	case strings.HasPrefix(lower, "invalidimageid"), strings.Contains(lower, "imagenotsupport"),
		lower == "invalidparameter.notmatch":
		f.Class = model.FailureImageSpecMismatch
		f.Retryable = false
		f.RetryHint = model.RetryHintDifferentImage

	case strings.HasPrefix(lower, "invalidparameter"), strings.HasPrefix(lower, "missingparameter"):
		f.Class = model.FailureInvalidRequest
		f.Retryable = false
		f.RetryHint = model.RetryHintAdjustRequest

	default:
		return model.ProvisioningFailure{}, false
	}

	if r := aliRequestIdRe.FindStringSubmatch(raw); r != nil {
		f.RequestId = r[1]
	}
	return f, true
}
