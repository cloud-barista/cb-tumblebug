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

package model

import "time"

// FailureClass groups CSP provisioning rejections by what a caller should do
// about them. The CSP-native wording differs per provider (AWS
// "InsufficientInstanceCapacity", GCP "STOCKOUT", Alibaba
// "OperationDenied.NoStock"), so the class — not the message — is what
// retry logic branches on.
const (
	// FailureZoneCapacity is a transient shortage in one zone. Another zone in
	// the same region may succeed right now.
	FailureZoneCapacity = "ZoneCapacity"
	// FailureRegionCapacity is a shortage across the whole region.
	FailureRegionCapacity = "RegionCapacity"
	// FailureAccountQuota is an account limit. Every zone in the region hits it.
	FailureAccountQuota = "AccountQuota"
	// FailureThrottling is CSP API rate limiting; the request itself was fine.
	FailureThrottling = "Throttling"
	// FailureDiskTypeUnavailable is a root-disk category the spec or zone rejects.
	FailureDiskTypeUnavailable = "DiskTypeUnavailable"
	// FailureImageSpecMismatch is an image incompatible with the spec, or an
	// image the CSP cannot find.
	FailureImageSpecMismatch = "ImageSpecMismatch"
	// FailureInvalidRequest is a request the CSP rejected on its own terms —
	// a disk too small, a parameter out of range. Retrying it unchanged fails
	// the same way; the request has to be corrected.
	FailureInvalidRequest = "InvalidRequest"
	// FailureAuth is a credential or permission rejection.
	FailureAuth = "Auth"
	// FailureNetwork is a transport failure reaching the CSP or CB-Spider.
	FailureNetwork = "Network"
	// FailureUnknown is anything not recognized. Callers must still handle it:
	// most providers have no dedicated parser yet.
	FailureUnknown = "Unknown"
)

// RetryHint is the recommended next move for a ProvisioningFailure.
const (
	RetryHintDifferentZone   = "differentZone"
	RetryHintDifferentSpec   = "differentSpec"
	RetryHintDifferentRegion = "differentRegion"
	RetryHintWaitAndRetry    = "waitAndRetry"
	RetryHintSameConfig      = "sameConfig"
	// RetryHintDifferentImage means the spec is fine but this image is not:
	// the CSP cannot find it, or the spec accepts only certain images.
	RetryHintDifferentImage = "differentImage"
	// RetryHintAdjustRequest means a request field has to change — a root disk
	// too small for the flavor, a parameter out of range.
	RetryHintAdjustRequest = "adjustRequest"
	// RetryHintNotRetryable means nothing in this request can be changed to make
	// it succeed: the account lacks quota or permission.
	RetryHintNotRetryable = "notRetryable"
)

// ProvisioningFailure is the structured form of one node-creation failure.
// It replaces the free-form error strings that used to be copied into
// NodeInfo.SystemMessage, NodeCreationError.Error, ProvisioningLog.FailureMessages
// and ProvisioningAttempt.FailureReason, so failures can be aggregated by zone
// and acted on programmatically.
//
// Produced in exactly one place: csp.ClassifyProvisioningFailure.
type ProvisioningFailure struct {
	// Class is one of the Failure* constants.
	Class string `json:"class" example:"ZoneCapacity"`
	// Retryable reports whether retrying can plausibly succeed without the
	// user changing something (quota increase, different spec).
	Retryable bool `json:"retryable" example:"true"`
	// RetryHint is one of the RetryHint* constants.
	RetryHint string `json:"retryHint,omitempty" example:"differentZone"`

	Provider string `json:"provider,omitempty" example:"aws"`
	Region   string `json:"region,omitempty" example:"us-west-2"`
	// CspErrorCode is the provider's own error identifier when one could be
	// extracted (e.g. "InsufficientInstanceCapacity", "STOCKOUT").
	CspErrorCode string `json:"cspErrorCode,omitempty" example:"InsufficientInstanceCapacity"`

	// AttemptedZone is the zone CB-Tumblebug actually requested, recorded from
	// the request itself. Authoritative: several CSPs (e.g. Alibaba) never name
	// the zone in their error text.
	AttemptedZone string `json:"attemptedZone,omitempty" example:"us-west-2a"`
	// ReportedZone is the zone parsed out of the CSP message, when it names one.
	// A mismatch with AttemptedZone means the VM was placed somewhere other than
	// where CB-Tumblebug intended — worth surfacing rather than hiding.
	ReportedZone string `json:"reportedZone,omitempty" example:"us-west-2a"`
	// SuggestedZones are alternatives the CSP itself offered. Only AWS provides
	// these today; empty for every other provider.
	SuggestedZones []string `json:"suggestedZones,omitempty" example:"us-west-2b,us-west-2c"`

	// Message is a single-line human-readable summary.
	Message string `json:"message,omitempty"`
	// RawMessage is the original CSP text after secret redaction, noise
	// trimming and length capping. Kept so an unrecognized failure can still
	// be diagnosed by a human.
	RawMessage string `json:"rawMessage,omitempty"`

	Source     string    `json:"source,omitempty" example:"cb-spider:1024/spider/vm"`
	HttpStatus int       `json:"httpStatus,omitempty" example:"500"`
	RequestId  string    `json:"requestId,omitempty"`
	OccurredAt time.Time `json:"occurredAt,omitempty"`
}

// ZoneCapability answers whether moving a node to another zone is even
// possible for a given connection. Two independent gates must both pass:
// the CSP driver must support zone-based control (CB-Spider's
// ZoneBasedControl capability), and the region must actually have more than
// one zone (Azure has 10 regions with none at all).
type ZoneCapability struct {
	// ZoneControl mirrors CB-Spider's per-driver ZoneBasedControl capability.
	ZoneControl bool `json:"zoneControl"`
	// Zones is the region's zone list from the connection config.
	Zones []string `json:"zones,omitempty"`
	// Shiftable is ZoneControl && len(Zones) >= 2.
	Shiftable bool `json:"shiftable"`
	// Reason explains a false Shiftable in user-facing terms.
	Reason string `json:"reason,omitempty"`
}
