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

package ibm

// This file implements csp.FailureParser for IBM Cloud VPC.
//
// IBM Cloud VPC errors arrive either wrapped in CB-Spider text or as IBM SDK errors:
//
//	Failed to Create VM. err = Creating a new floating IP address will put the user over quota.
//	Allocated: 40, Requested: 1, Quota: 40 (from cb-spider:1024/spider/vm (500 Internal Server Error))

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

func (p *failureParser) Provider() string { return cspconst.IBM }

var (
	// Code: quota_exceeded or Code: bad_request or ErrorCode: ...
	ibmCodeRe = regexp.MustCompile(`(?i)(?:Code|ErrorCode):\s*([a-z0-9_.]+)`)
	// StatusCode: 400 or HTTP 400
	ibmStatusRe = regexp.MustCompile(`(?i)(?:StatusCode|Status):\s*(\d{3})`)
	// Transaction-Id: abc... or Incident ID: abc...
	ibmTxIdRe = regexp.MustCompile(`(?i)(?:Transaction-Id|Incident ID|request id):\s*([0-9a-fA-F-]{8,})`)
	// Extract the informative error message following "err ="
	ibmSpiderErrRe = regexp.MustCompile(`(?i)err\s*=\s*(.+?)(?:\s*\(from cb-spider|$)`)
)

// Parse recognizes IBM Cloud VPC error strings. Returns ok=false when no recognizable
// IBM error pattern is found, allowing the generic classifier to try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	lower := strings.ToLower(raw)

	// Check if this error contains known IBM VPC keywords or patterns
	hasIbmMarker := strings.Contains(lower, "floating ip") ||
		strings.Contains(lower, "over quota") ||
		strings.Contains(lower, "quota exceeded") ||
		strings.Contains(lower, "vpcv1") ||
		strings.Contains(lower, "ibmcloud") ||
		strings.Contains(lower, "instance_profile") ||
		strings.Contains(lower, "cannot create floating ip") ||
		strings.Contains(lower, "creating a new floating ip")

	if !hasIbmMarker {
		return model.ProvisioningFailure{}, false
	}

	var code string
	if m := ibmCodeRe.FindStringSubmatch(raw); m != nil {
		code = m[1]
	}

	var status int
	if m := ibmStatusRe.FindStringSubmatch(raw); m != nil {
		status, _ = strconv.Atoi(m[1])
	}

	var txId string
	if m := ibmTxIdRe.FindStringSubmatch(raw); m != nil {
		txId = m[1]
	}

	var message string
	if m := ibmSpiderErrRe.FindStringSubmatch(raw); m != nil {
		message = strings.TrimSpace(m[1])
	} else {
		message = raw
	}

	f := model.ProvisioningFailure{
		CspErrorCode: code,
		HttpStatus:   status,
		Message:      message,
		RequestId:    txId,
	}

	lowerCode := strings.ToLower(code)

	switch {
	// Account or VPC-level quota exhaustion (Floating IP, instances, cores, volumes)
	case strings.Contains(lower, "over quota"),
		strings.Contains(lower, "quota exceeded"),
		strings.Contains(lower, "maximum number of floating ips"),
		strings.Contains(lower, "floating_ips_per_vpc"),
		strings.Contains(lowerCode, "quota_exceeded"),
		strings.Contains(lowerCode, "over_quota"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable
		if f.CspErrorCode == "" {
			f.CspErrorCode = "QuotaExceeded"
		}

	// Zone capacity shortage
	case strings.Contains(lower, "out of capacity"),
		strings.Contains(lower, "capacity not available"),
		strings.Contains(lower, "insufficient capacity"),
		strings.Contains(lowerCode, "insufficient_capacity"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone
		if f.CspErrorCode == "" {
			f.CspErrorCode = "InsufficientCapacity"
		}

	// Throttling / Rate limiting
	case strings.Contains(lower, "rate limit"),
		strings.Contains(lowerCode, "rate_limit_exceeded"),
		status == 429:
		f.Class = model.FailureThrottling
		f.Retryable = true
		f.RetryHint = model.RetryHintSameConfig

	// Authentication / Access Forbidden
	case strings.Contains(lowerCode, "unauthorized"),
		strings.Contains(lowerCode, "forbidden"),
		status == 401 || status == 403:
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	// Invalid parameters or profile mismatch
	case strings.Contains(lowerCode, "not_found"),
		strings.Contains(lowerCode, "bad_request"),
		strings.Contains(lower, "instance profile not found"):
		f.Class = model.FailureInvalidRequest
		f.Retryable = false
		f.RetryHint = model.RetryHintAdjustRequest

	default:
		f.Class = model.FailureUnknown
		f.Retryable = true
		f.RetryHint = model.RetryHintSameConfig
	}

	return f, true
}
