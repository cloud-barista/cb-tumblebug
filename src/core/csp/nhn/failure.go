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

package nhn

// This file implements csp.FailureParser for NHN Cloud (OpenStack-derived).
//
// NHN wraps the OpenStack response in CB-Spider's own text and reports the real
// cause in a nested badRequest object:
//
//	Failed to Create a VM with the Block Storage Volume!! [Bad request with:
//	[POST https://kr1-api-instance-infrastructure.nhncloudservice.com/v2/.../servers],
//	error message: {badRequest:{message:Volume size is too small.,code:400}}]
//
// Or Public IP allocation failures:
//
//	Failed to Start VM. Failed to Associate PublicIP : Failed to Create Public IP!! : [Internal Server Error]

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

func (p *failureParser) Provider() string { return cspconst.NHN }

// The nested OpenStack error object; message and code arrive unquoted.
var (
	nhnBadRequestRe = regexp.MustCompile(`\{(badRequest|forbidden|itemNotFound|overLimit)\s*:\s*\{message\s*:\s*([^,}]+),\s*code\s*:\s*(\d{3})`)
	nhnPublicIpRe   = regexp.MustCompile(`(?i)Failed to Create Public IP(?:!!)?\s*:\s*\[([^\]]+)\]`)
)

// Parse recognizes the nested OpenStack error object or Public IP failure strings.
// Returns ok=false when the message has no recognizable NHN patterns so the generic
// classifier can try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	lower := strings.ToLower(raw)

	// Check for Public IP allocation failures
	if m := nhnPublicIpRe.FindStringSubmatch(raw); m != nil {
		detail := strings.TrimSpace(m[1])
		f := model.ProvisioningFailure{
			CspErrorCode: "PublicIPAllocationFailed",
			Message:      "Failed to Create Public IP: " + detail,
		}
		if strings.Contains(strings.ToLower(detail), "internal server error") {
			f.HttpStatus = 500
		}

		if strings.Contains(lower, "no available") || strings.Contains(lower, "quota") || strings.Contains(lower, "overlimit") {
			f.Class = model.FailureRegionCapacity
			f.Retryable = false
			f.RetryHint = model.RetryHintDifferentRegion
		} else {
			// Transient 500 server error during IP creation: retryable with same config
			f.Class = model.FailureUnknown
			f.Retryable = true
			f.RetryHint = model.RetryHintSameConfig
		}
		return f, true
	}

	m := nhnBadRequestRe.FindStringSubmatch(raw)
	if m == nil {
		return model.ProvisioningFailure{}, false
	}
	kind, detail := m[1], strings.TrimSpace(m[2])
	status, _ := strconv.Atoi(m[3])
	lowerDetail := strings.ToLower(detail)

	f := model.ProvisioningFailure{
		CspErrorCode: kind,
		HttpStatus:   status,
		Message:      detail,
	}

	switch {
	case kind == "overLimit", strings.Contains(lowerDetail, "quota"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case kind == "forbidden":
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	case strings.Contains(lowerDetail, "no valid host"), strings.Contains(lowerDetail, "no available"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone

	case kind == "itemNotFound", strings.Contains(lowerDetail, "image"), strings.Contains(lowerDetail, "flavor"):
		f.Class = model.FailureImageSpecMismatch
		f.Retryable = false
		f.RetryHint = model.RetryHintDifferentImage

	default:
		// A 4xx badRequest is the CSP rejecting the request as written (e.g.
		// "Volume size is too small"); repeating it unchanged fails identically.
		f.Class = model.FailureInvalidRequest
		f.Retryable = false
		f.RetryHint = model.RetryHintAdjustRequest
	}
	return f, true
}
