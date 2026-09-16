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

package azure

// This file implements csp.FailureParser for Microsoft Azure Compute.
//
// Azure ARM REST responses wrap failure details in a structured block:
//
//	RESPONSE 409: 409 Conflict ERROR CODE: OperationNotAllowed
//	{ error: { code: OperationNotAllowed, message: Operation could not be completed
//	as it results in exceeding approved standardBSFamily Cores quota. ... } }

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

func (p *failureParser) Provider() string { return cspconst.Azure }

var (
	// ERROR CODE: OperationNotAllowed or code: OperationNotAllowed
	azureCodeRe = regexp.MustCompile(`(?:ERROR CODE:\s*|code:\s*)([A-Za-z][A-Za-z0-9_.]*)`)
	// RESPONSE 409: 409 Conflict
	azureStatusRe = regexp.MustCompile(`RESPONSE\s*(\d{3})`)
	// message: Operation could not be completed as it results in exceeding approved ...
	azureMessageRe = regexp.MustCompile(`message:\s*([^}\r\n]+?)(?:\.\s*Additional details|\.\s*Setup Alerts|$)`)
	// correlationId or trackingId
	azureTrackingIdRe = regexp.MustCompile(`(?:trackingId|correlationId|x-ms-request-id):\s*([0-9a-fA-F-]{8,})`)
)

// Parse recognizes Azure ARM error responses. Returns ok=false when no Azure error
// marker is present, allowing generic keyword classification as fallback.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	codeMatch := azureCodeRe.FindStringSubmatch(raw)
	statusMatch := azureStatusRe.FindStringSubmatch(raw)

	// If neither ARM error code nor HTTP response header is matched, not an Azure error block.
	if codeMatch == nil && statusMatch == nil && !strings.Contains(raw, "management.azure.com") {
		return model.ProvisioningFailure{}, false
	}

	var code string
	if codeMatch != nil {
		code = codeMatch[1]
	}

	var status int
	if statusMatch != nil {
		status, _ = strconv.Atoi(statusMatch[1])
	}

	var message string
	if msgMatch := azureMessageRe.FindStringSubmatch(raw); msgMatch != nil {
		message = strings.TrimSpace(msgMatch[1])
	}

	var trackingId string
	if trMatch := azureTrackingIdRe.FindStringSubmatch(raw); trMatch != nil {
		trackingId = trMatch[1]
	}

	lowerCode := strings.ToLower(code)
	lowerRaw := strings.ToLower(raw)

	f := model.ProvisioningFailure{
		CspErrorCode: code,
		HttpStatus:   status,
		Message:      message,
		RequestId:    trackingId,
	}

	switch {
	// Account / Subscription-level quota limits
	case strings.Contains(lowerCode, "quotaexceeded"),
		strings.Contains(lowerCode, "resourcequotaexceeded"),
		(lowerCode == "operationnotallowed" && (strings.Contains(lowerRaw, "quota") || strings.Contains(lowerRaw, "limit") || strings.Contains(lowerRaw, "exceeding approved"))),
		strings.Contains(lowerRaw, "exceeding approved") && strings.Contains(lowerRaw, "quota"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	// Regional / Zonal VM capacity shortage
	case strings.Contains(lowerCode, "skunotavailable"),
		strings.Contains(lowerCode, "allocationfailed"),
		strings.Contains(lowerCode, "overconstrainedallocationrequest"),
		strings.Contains(lowerCode, "zonalallocationfailed"),
		strings.Contains(lowerRaw, "skunotavailable"),
		strings.Contains(lowerRaw, "allocation failed"),
		strings.Contains(lowerRaw, "currently not available in region"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone

	// API Rate Limiting / Throttling
	case strings.Contains(lowerCode, "toomanyrequests"), status == 429:
		f.Class = model.FailureThrottling
		f.Retryable = true
		f.RetryHint = model.RetryHintSameConfig

	// Authentication / Authorization / Subscription state
	case strings.Contains(lowerCode, "authorizationfailed"),
		strings.Contains(lowerCode, "authenticationfailed"),
		strings.Contains(lowerCode, "subscriptionnotfound"),
		strings.Contains(lowerCode, "subscriptionnotregistered"),
		status == 401 || status == 403:
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	// Image / Spec / Parameter invalid
	case strings.Contains(lowerCode, "imagenotfound"),
		strings.Contains(lowerCode, "invalidparameter"),
		strings.Contains(lowerCode, "badrequest"),
		strings.Contains(lowerCode, "resourcenotfound"):
		f.Class = model.FailureInvalidRequest
		f.Retryable = false
		f.RetryHint = model.RetryHintAdjustRequest

	default:
		// If status 409 or 400 but code unrecognized, treat as invalid request or policy
		if status == 409 || status == 400 {
			f.Class = model.FailureInvalidRequest
			f.Retryable = false
			f.RetryHint = model.RetryHintAdjustRequest
		} else {
			f.Class = model.FailureUnknown
			f.Retryable = true
			f.RetryHint = model.RetryHintSameConfig
		}
	}

	return f, true
}
