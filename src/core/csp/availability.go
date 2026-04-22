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

// This file provides a provider-agnostic availability pre-check facility.
//
// Goals:
//   - Allow callers (e.g. ReviewSpecImagePair) to ask "can this VM spec be
//     provisioned now in this region?" without knowing CSP-specific APIs.
//   - Let each CSP plug in its own checker (Alibaba: DescribeAvailableResource,
//     Azure: Resource SKU + quota, AWS: DescribeInstanceTypeOfferings, ...).
//   - Avoid burst load on CSP control planes via a TTL cache + singleflight.
//   - Be non-fatal: if no checker is registered for a provider, treat the
//     request as available (do not block provisioning on missing coverage).

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

// AvailabilityChecker is implemented by CSP-specific packages and registered
// via Register at init() time.
type AvailabilityChecker interface {
	// Provider returns the CSP identifier (must match model/csp constants).
	Provider() string
	// CheckInstance performs a pre-flight availability query for the given
	// instance type in the given region. Implementations should populate
	// Zones with per-zone disk-category availability when their CSP API
	// supports it. Implementations should return non-fatal results: a true
	// inability to determine availability should be returned as
	// (result with Available=true, Reason="...") with err=nil so that
	// provisioning is never blocked solely by a checker failure.
	CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error)
}

// availabilityTTL is the lifetime of a cached availability result. Stock
// state changes faster than image families, so a short TTL is appropriate.
const availabilityTTL = 5 * time.Minute

type availabilityCacheEntry struct {
	result    model.AvailabilityResult
	expiresAt time.Time
}

var (
	checkers          = map[string]AvailabilityChecker{}
	checkersMu        sync.RWMutex
	availabilityCache sync.Map // key: cacheKey(...) -> *availabilityCacheEntry
	availabilityGroup singleflight.Group
)

// RegisterAvailabilityChecker registers a CSP-specific checker. It is safe
// to call from package init() functions.
func RegisterAvailabilityChecker(c AvailabilityChecker) {
	if c == nil {
		return
	}
	checkersMu.Lock()
	defer checkersMu.Unlock()
	checkers[strings.ToLower(c.Provider())] = c
}

// CheckAvailability dispatches to the registered checker for q.Provider and
// caches the result for availabilityTTL. Concurrent misses for the same
// (provider, region, instanceType, disk) are deduplicated via singleflight.
//
// If no checker is registered for the provider, the function returns a
// "no-checker" result with Available=true so that callers can proceed.
// All checker errors are also turned into non-fatal "Available=true" results
// with the error captured in Reason; this is intentional to avoid blocking
// provisioning on pre-check failures.
func CheckAvailability(ctx context.Context, q model.AvailabilityQuery) model.AvailabilityResult {
	provider := strings.ToLower(strings.TrimSpace(q.Provider))

	checkersMu.RLock()
	c, ok := checkers[provider]
	checkersMu.RUnlock()

	if !ok {
		return model.AvailabilityResult{
			Provider:     provider,
			Region:       q.Region,
			InstanceType: q.InstanceType,
			Available:    true,
			Reason:       "no availability checker registered for provider; assumed available",
			Source:       "none",
			QueriedAt:    time.Now(),
		}
	}

	key := availabilityCacheKey(provider, q)

	// Fast path: cache hit.
	if v, ok := availabilityCache.Load(key); ok {
		if e, ok := v.(*availabilityCacheEntry); ok && time.Now().Before(e.expiresAt) {
			r := e.result
			r.Cached = true
			return r
		}
	}

	// Slow path: dedupe concurrent misses.
	v, _, _ := availabilityGroup.Do(key, func() (interface{}, error) {
		// Re-check cache inside singleflight to avoid redundant API calls
		// when a prior in-flight request just populated it.
		if v, ok := availabilityCache.Load(key); ok {
			if e, ok := v.(*availabilityCacheEntry); ok && time.Now().Before(e.expiresAt) {
				return e.result, nil
			}
		}
		r, err := c.CheckInstance(ctx, q)
		if err != nil {
			// Non-fatal: turn error into an "available" result so the caller
			// is never blocked by checker problems. Do NOT cache errors.
			log.Warn().Err(err).
				Str("provider", provider).
				Str("region", q.Region).
				Str("instanceType", q.InstanceType).
				Msg("availability checker failed; treating as available")
			return model.AvailabilityResult{
				Provider:     provider,
				Region:       q.Region,
				InstanceType: q.InstanceType,
				Available:    true,
				Reason:       "availability checker failed: " + err.Error(),
				Source:       c.Provider() + ":error",
				QueriedAt:    time.Now(),
			}, nil
		}
		if r.QueriedAt.IsZero() {
			r.QueriedAt = time.Now()
		}
		availabilityCache.Store(key, &availabilityCacheEntry{
			result:    r,
			expiresAt: time.Now().Add(availabilityTTL),
		})
		return r, nil
	})
	return v.(model.AvailabilityResult)
}

func availabilityCacheKey(provider string, q model.AvailabilityQuery) string {
	// PreferredZone must be part of the key: callers passing different zone
	// hints would otherwise reuse a cached result computed for another zone
	// and observe stale availability for the requested zone.
	return provider + "|" + q.Region + "|" + q.InstanceType + "|" + q.SystemDiskCategory + "|" + q.PreferredZone
}
