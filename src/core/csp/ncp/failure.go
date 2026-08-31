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

package ncp

// This file implements csp.FailureParser for NAVER Cloud Platform.
//
// NCP reports a numeric returnCode plus an English returnMessage:
//
//	[Status: 400 Bad Request, Body: { responseError: { returnCode: 1153027,
//	returnMessage: Server (VPC) product generation limit exceeded.
//	Product Type: GPU - T4 - G1 Creation Limit:0 / Usage:0 / Creation Request:1 } }]
//
// "Server (VPC)" names NCP's platform (VPC vs Classic, matching the VSVR segment
// of a ServerProductCode) — it is not about VPC networking. The limit is on the
// server product generation, and a Creation Limit of 0 means the account was
// never entitled to the product, which needs a different answer from a limit
// that has merely been used up.

import (
	"fmt"
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

func (p *failureParser) Provider() string { return cspconst.NCP }

var (
	ncpReturnCodeRe = regexp.MustCompile(`returnCode:\s*(\d+)`)
	ncpReturnMsgRe  = regexp.MustCompile(`returnMessage:\s*(.+?)\s*\}`)
	ncpProductRe    = regexp.MustCompile(`Product Type:\s*([A-Za-z0-9 \-]+?)\s*Creation Limit`)
	ncpLimitsRe     = regexp.MustCompile(`Creation Limit:\s*(\d+)\s*/\s*Usage:\s*(\d+)\s*/\s*Creation Request:\s*(\d+)`)
)

// Parse recognizes the NCP responseError envelope. Returns ok=false without a
// returnCode so the generic classifier can try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	m := ncpReturnCodeRe.FindStringSubmatch(raw)
	if m == nil {
		return model.ProvisioningFailure{}, false
	}

	f := model.ProvisioningFailure{CspErrorCode: m[1]}

	detail := ""
	if d := ncpReturnMsgRe.FindStringSubmatch(raw); d != nil {
		detail = strings.TrimSpace(d[1])
	}
	lower := strings.ToLower(detail)

	switch {
	case strings.Contains(lower, "limit exceeded"), strings.Contains(lower, "quota"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable
		f.Message = ncpQuotaMessage(raw, detail)

	case strings.Contains(lower, "insufficient"), strings.Contains(lower, "no available"),
		strings.Contains(lower, "sold out"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone

	case strings.Contains(lower, "not found"), strings.Contains(lower, "invalid image"):
		f.Class = model.FailureImageSpecMismatch
		f.Retryable = false
		f.RetryHint = model.RetryHintDifferentImage

	case strings.Contains(lower, "authentication"), strings.Contains(lower, "unauthorized"):
		f.Class = model.FailureAuth
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable

	default:
		return model.ProvisioningFailure{}, false
	}
	return f, true
}

// ncpQuotaMessage separates "the account may not create this product at all"
// (limit 0) from "the existing allowance is used up", which need different
// actions from the user.
func ncpQuotaMessage(raw, detail string) string {
	product := ""
	if p := ncpProductRe.FindStringSubmatch(raw); p != nil {
		product = strings.TrimSpace(p[1])
	}
	l := ncpLimitsRe.FindStringSubmatch(raw)
	if l == nil {
		return detail
	}
	limit, _ := strconv.Atoi(l[1])
	usage, _ := strconv.Atoi(l[2])
	if product == "" {
		product = "this server product"
	}
	if limit == 0 {
		return fmt.Sprintf("the account is not entitled to %s on NCP (creation limit is 0); request access to the product before retrying", product)
	}
	return fmt.Sprintf("NCP creation limit for %s is used up (%d of %d in use)", product, usage, limit)
}
