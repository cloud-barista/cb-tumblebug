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

package gcp

// This file implements csp.FailureParser for GCP Compute Engine.
//
// GCP names the failing zone inside a resource path but never offers an
// alternative, so ReportedZone is extractable and SuggestedZones stays empty:
//
//	Operation errors: The zone 'projects/<project>/zones/europe-west4-b' does
//	not have enough resources available to fulfill the request.
//	'NULL:0/NULL:0/NULL:0 (state:STOCKOUT, sub-state:STOCKOUT, resource type:compute)'.
//
// The resource path carries the GCP project id; the caller redacts and the
// parser does not copy it into any field of its own.

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

func (p *failureParser) Provider() string { return cspconst.GCP }

var (
	// "zones/europe-west4-b" from either a full resource path or a bare one.
	gcpZoneRe = regexp.MustCompile(`zones/([a-z0-9-]+)`)
	// "state:STOCKOUT" / "sub-state:ZONE_RESOURCE_POOL_EXHAUSTED"
	gcpStateRe = regexp.MustCompile(`\bstate:([A-Z_]+)`)
)

// Parse recognizes GCP Compute error text. Returns ok=false when no GCP marker
// is present so the generic classifier can try.
func (p *failureParser) Parse(raw string) (model.ProvisioningFailure, bool) {
	lower := strings.ToLower(raw)

	f := model.ProvisioningFailure{}
	if s := gcpStateRe.FindStringSubmatch(raw); s != nil {
		f.CspErrorCode = s[1]
	}

	switch {
	case strings.Contains(lower, "stockout"),
		strings.Contains(lower, "zone_resource_pool_exhausted"),
		strings.Contains(lower, "does not have enough resources"):
		f.Class = model.FailureZoneCapacity
		f.Retryable = true
		f.RetryHint = model.RetryHintDifferentZone
		if f.CspErrorCode == "" {
			f.CspErrorCode = "ZONE_RESOURCE_POOL_EXHAUSTED"
		}

	case strings.Contains(lower, "quota_exceeded"), strings.Contains(lower, "quotaexceeded"),
		strings.Contains(lower, "quota exceeded"):
		f.Class = model.FailureAccountQuota
		f.Retryable = false
		f.RetryHint = model.RetryHintNotRetryable
		if f.CspErrorCode == "" {
			f.CspErrorCode = "QUOTA_EXCEEDED"
		}

	case strings.Contains(lower, "ratelimitexceeded"), strings.Contains(lower, "rate_limit_exceeded"):
		f.Class = model.FailureThrottling
		f.Retryable = true
		f.RetryHint = model.RetryHintWaitAndRetry

	default:
		return model.ProvisioningFailure{}, false
	}

	// Only trust the zone when it came from a zones/<id> path; a bare region
	// name elsewhere in the message would be misleading as a zone.
	if z := gcpZoneRe.FindStringSubmatch(raw); z != nil {
		f.ReportedZone = z[1]
	}
	return f, true
}
