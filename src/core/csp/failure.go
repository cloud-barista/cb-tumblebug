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

package csp

// This file turns a CSP's free-form VM-creation error into a
// model.ProvisioningFailure so retry logic can branch on a class instead of
// matching substrings at every call site.
//
// Each provider may register a parser that extracts its own error code, the
// zone it rejected, and any alternative zones it offered. Coverage is partial
// by design: only providers with observed failure samples have parsers, and
// everything else falls back to keyword classification. Classification never
// fails — an unrecognized message becomes FailureUnknown with the raw text
// preserved.

import (
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// FailureParser is implemented by CSP-specific packages and registered via
// RegisterFailureParser at init() time.
type FailureParser interface {
	// Provider returns the CSP identifier (must match model/csp constants).
	Provider() string
	// Parse extracts structured fields from a raw CSP error message. It
	// returns ok=false when the message is not one this provider recognizes,
	// letting the generic classifier handle it. Implementations must not
	// panic on arbitrary input.
	Parse(raw string) (model.ProvisioningFailure, bool)
}

var (
	failureParsers   = map[string]FailureParser{}
	failureParsersMu sync.RWMutex
)

// RegisterFailureParser registers a CSP-specific parser. Safe to call from init().
func RegisterFailureParser(p FailureParser) {
	if p == nil {
		return
	}
	failureParsersMu.Lock()
	defer failureParsersMu.Unlock()
	failureParsers[strings.ToLower(p.Provider())] = p
}

// rawMessageMaxLen caps stored error text. Alibaba errors carry a full
// RespHeaders map dump that dwarfs the useful part.
const rawMessageMaxLen = 2000

// noiseMarkers cut provider debris that follows the informative part of a
// message. Everything from the first marker onward is dropped.
var noiseMarkers = []string{
	"RespHeaders:",  // Alibaba: whole HTTP response header map
	"RespBodyBytes", // Alibaba
}

// ClassifyProvisioningFailure converts a CSP error into a ProvisioningFailure.
//
// provider selects the parser; attemptedZone is what CB-Tumblebug requested and
// is always recorded, because several CSPs never name the zone they rejected.
// The returned value is always usable: on an unrecognized message the class is
// FailureUnknown and RawMessage holds the (redacted, trimmed) original.
func ClassifyProvisioningFailure(provider, region, attemptedZone, raw string) model.ProvisioningFailure {
	provider = strings.ToLower(strings.TrimSpace(provider))
	clean := NormalizeFailureMessage(raw)

	var f model.ProvisioningFailure
	failureParsersMu.RLock()
	p, ok := failureParsers[provider]
	failureParsersMu.RUnlock()

	if ok {
		if parsed, matched := p.Parse(clean); matched {
			f = parsed
		}
	}
	if f.Class == "" {
		f = classifyGeneric(clean)
	}

	f.Provider = provider
	f.Region = region
	f.AttemptedZone = attemptedZone
	f.RawMessage = clean
	if f.OccurredAt.IsZero() {
		f.OccurredAt = time.Now()
	}
	if f.Message == "" {
		f.Message = summarize(clean)
	}
	// A parser may have found the zone the CSP named while the caller had no
	// zone to report; keep the record self-consistent for aggregation.
	if f.AttemptedZone == "" && f.ReportedZone != "" {
		f.AttemptedZone = f.ReportedZone
	}
	// A zone shift is pointless when the CSP already named the failing zone as
	// the only alternative.
	f.SuggestedZones = removeZone(f.SuggestedZones, f.AttemptedZone)
	if f.Class == model.FailureZoneCapacity && len(f.SuggestedZones) == 0 && f.RetryHint == "" {
		f.RetryHint = model.RetryHintDifferentZone
	}
	return f
}

// NormalizeFailureMessage redacts secrets, cuts provider debris and caps length.
// Exported so callers that store a message without classifying it apply the
// same treatment.
func NormalizeFailureMessage(raw string) string {
	s := strings.TrimSpace(raw)
	for _, marker := range noiseMarkers {
		if i := strings.Index(s, marker); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	s = RedactSecrets(s)
	if len(s) > rawMessageMaxLen {
		s = s[:rawMessageMaxLen] + " ...(truncated)"
	}
	return s
}

// classifyGeneric is the provider-agnostic fallback. It merges the keyword sets
// that used to live in infra.isAvailabilityFailure and
// infra.isQuotaOrCapacityError, keeping their ordering rule: throttling and
// transport errors are checked before capacity, and account-level quota before
// zone capacity, because quota messages often also contain the word "limit".
func classifyGeneric(msg string) model.ProvisioningFailure {
	lower := strings.ToLower(msg)
	has := func(patterns ...string) bool {
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				return true
			}
		}
		return false
	}

	switch {
	case has("requestlimitexceeded", "throttling", "toomanyrequests", "too many requests",
		"frequency limit", "reduce the frequency", "ratelimitexceeded"):
		return model.ProvisioningFailure{
			Class: model.FailureThrottling, Retryable: true,
			RetryHint: model.RetryHintWaitAndRetry,
		}

	case has("connection refused", "connection reset", "no such host", "i/o timeout",
		"context deadline exceeded", "eof"):
		return model.ProvisioningFailure{
			Class: model.FailureNetwork, Retryable: true,
			RetryHint: model.RetryHintWaitAndRetry,
		}

	case has("unauthorized", "forbidden", "accessdenied", "invalidclienttokenid",
		"authfailure", "signaturedoesnotmatch", "invalid credential"):
		return model.ProvisioningFailure{
			Class: model.FailureAuth, Retryable: false,
			RetryHint: model.RetryHintNotRetryable,
		}

	// Account limits apply region-wide, so they must not be mistaken for a
	// zone shortage: a different zone in the same account hits the same wall.
	case has("quota", "overlimit", "limitexceeded", "limit exceeded", "instancelimitexceeded",
		"vcpulimitexceeded", "quotaexceed", "creation limit", "operationnotallowed"):
		return model.ProvisioningFailure{
			Class: model.FailureAccountQuota, Retryable: false,
			RetryHint: model.RetryHintNotRetryable,
		}

	// A request the CSP rejected on its own terms fails identically on a retry,
	// so it must not fall through to the Unknown "try once more" default.
	case has("invalidimageid", "image not found", "is not found", "imagenotfound",
		"only supports some specific images", "invalidinstancetype", "unsupported instance type"):
		return model.ProvisioningFailure{
			Class: model.FailureImageSpecMismatch, Retryable: false,
			RetryHint: model.RetryHintDifferentImage,
		}

	case has("invalidparameter", "missingparameter", "too small", "too large",
		"out of range", "is invalid", "badrequest"):
		return model.ProvisioningFailure{
			Class: model.FailureInvalidRequest, Retryable: false,
			RetryHint: model.RetryHintAdjustRequest,
		}

	// Public-IP exhaustion is region-wide and must be matched before the generic
	// "no available" capacity keyword below, which would otherwise swallow it.
	// Only genuine exhaustion of the shared pool. A quota on addresses is an
	// account limit and is matched above; a bare creation failure says nothing at
	// all and must not be reported as a shortage only another region can fix.
	case has("no available public ip", "사용 가능한 공인 ip가 없습니다"):
		return model.ProvisioningFailure{
			Class: model.FailureRegionCapacity, Retryable: true,
			RetryHint: model.RetryHintDifferentRegion,
		}

	// "enough resources" and "sufficient capacity" are deliberately loose: the
	// providers wrap them ("does not have enough resources", "do not have
	// sufficient <type> capacity"), so anchoring on the negation misses both.
	case has("insufficientinstancecapacity", "insufficient capacity", "insufficientcapacity",
		"sufficient capacity", "enough resources",
		"unfulfillablecapacity", "zone_resource_pool_exhausted",
		"resource_pool_exhausted", "stockout", "nostock", "out of stock", "stock",
		"resourceinsufficient", "insufficient_resources", "resourcesexhausted",
		"skunotavailable", "no available"):
		return model.ProvisioningFailure{
			Class: model.FailureZoneCapacity, Retryable: true,
			RetryHint: model.RetryHintDifferentZone,
		}

	}

	// Unrecognized: allow exactly one same-config retry rather than declaring
	// the node permanently dead on a message nobody has taught us yet.
	return model.ProvisioningFailure{
		Class: model.FailureUnknown, Retryable: true,
		RetryHint: model.RetryHintSameConfig,
	}
}

// summarize builds a one-line message for UIs from a possibly multi-line error.
func summarize(msg string) string {
	const maxLen = 200
	s := strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))
	if i := strings.Index(s, ". "); i > 0 && i < maxLen {
		return s[:i+1]
	}
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func removeZone(zones []string, drop string) []string {
	if drop == "" || len(zones) == 0 {
		return zones
	}
	out := zones[:0]
	for _, z := range zones {
		if !strings.EqualFold(z, drop) {
			out = append(out, z)
		}
	}
	return out
}
