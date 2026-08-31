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

// This file implements csp.FailureParser for AWS EC2 RunInstances rejections.
//
// AWS is the only provider observed to name both the zone it rejected and the
// zones that would work, so those are worth extracting verbatim rather than
// falling back to the region's full zone list:
//
//	InsufficientInstanceCapacity: We currently do not have sufficient
//	g6e.2xlarge capacity in the Availability Zone you requested (us-west-2a).
//	... You can currently get g6e.2xlarge capacity by not specifying an
//	Availability Zone in your request or choosing us-west-2b, us-west-2c,
//	us-west-2d. status code: 500, request id: 2c2fa8a8-...

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	cspconst "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
)

func init() {
	csp.RegisterFailureParser(&failureParser{})
}

type failureParser struct{}

func (p *failureParser) Provider() string { return cspconst.AWS }

var (
	// Leading "ErrorCode: message" form used by the AWS SDK.
	awsErrCodeRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9.]*):\s`)
	// "Availability Zone you requested (us-west-2a)"
	awsRequestedZoneRe = regexp.MustCompile(`Availability Zone you requested \(([a-z0-9-]+)\)`)
	// "choosing us-west-2b, us-west-2c, us-west-2d." — the trailing period is
	// part of the sentence, not the last zone.
	awsChoosingRe   = regexp.MustCompile(`choosing ([a-z0-9-]+(?:\s*,\s*[a-z0-9-]+)*)`)
	awsStatusCodeRe = regexp.MustCompile(`status code:\s*(\d{3})`)
	awsRequestIdRe  = regexp.MustCompile(`request id:\s*([0-9a-fA-F-]{8,})`)
)

// Parse recognizes AWS SDK error strings. It reports ok=false for messages
// without an AWS-style error code so the generic classifier can try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	m := awsErrCodeRe.FindStringSubmatch(raw)
	if m == nil {
		return model.ProvisioningFailure{}, false
	}
	code := m[1]

	f := model.ProvisioningFailure{CspErrorCode: code}

	switch lower := strings.ToLower(code); {
	case lower == "insufficientinstancecapacity",
		lower == "insufficientcapacity",
		lower == "unfulfillablecapacity",
		lower == "insufficienthostcapacity":
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone

	// Throttling first: RequestLimitExceeded also contains "limitexceeded", so the
	// account-quota case below would otherwise claim it and report a rate limit —
	// which clears on its own — as something only a quota increase can fix.
	case lower == "requestlimitexceeded", strings.Contains(lower, "throttl"):
		f.Class = model.FailureThrottling
		f.Retryable = true
		f.RetryHint = model.RetryHintWaitAndRetry

	// Account limits are region-wide: another AZ hits the same ceiling.
	case strings.Contains(lower, "limitexceeded"), strings.Contains(lower, "quota"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case lower == "authfailure", lower == "unauthorizedoperation",
		lower == "accessdenied", lower == "invalidclienttokenid":
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case strings.HasPrefix(lower, "invalidparameter"), strings.HasPrefix(lower, "invalidami"),
		lower == "unsupportedoperation":
		f.Class = model.FailureImageSpecMismatch
		f.Retryable = false
		f.RetryHint = model.RetryHintDifferentImage

	default:
		// A recognized AWS envelope with an unmapped code: let the generic
		// keyword pass decide the class, but keep the code we extracted.
		return model.ProvisioningFailure{}, false
	}

	if z := awsRequestedZoneRe.FindStringSubmatch(raw); z != nil {
		f.ReportedZone = z[1]
	}
	if c := awsChoosingRe.FindStringSubmatch(raw); c != nil {
		for _, z := range strings.Split(c[1], ",") {
			if z = strings.TrimSpace(z); z != "" {
				f.SuggestedZones = append(f.SuggestedZones, z)
			}
		}
	}
	if s := awsStatusCodeRe.FindStringSubmatch(raw); s != nil {
		f.HttpStatus, _ = strconv.Atoi(s[1])
	}
	if r := awsRequestIdRe.FindStringSubmatch(raw); r != nil {
		f.RequestId = r[1]
	}
	return f, true
}
