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
	"fmt"
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

// Asymmetric cache TTLs by availability status and hardware class (Standard CPU vs GPU/Accelerator).
//
// Rationale:
//   - Standard CPU specs rarely go out of stock, so a 1-hour Available TTL avoids redundant API calls,
//     while write-through invalidation (RecordProvisioningFailureToCache) immediately downgrades the
//     cache entry if a real VM creation fails due to SoldOut or QuotaExceeded.
//   - GPU/Accelerator specs experience frequent stock/quota contention, so Available is cached for 5m
//     and QuotaExceeded for 2m.
//   - SoldOut is always cached for 1m so recovery is detected quickly without hammering CSP APIs on bulk fetches.
const (
	availabilityTTLStandardAvailable     = 1 * time.Hour
	availabilityTTLStandardQuotaExceeded = 5 * time.Minute
	availabilityTTLStandardSoldOut       = 1 * time.Minute
	availabilityTTLStandardUnsupported   = 1 * time.Hour

	availabilityTTLAcceleratorAvailable     = 5 * time.Minute
	availabilityTTLAcceleratorQuotaExceeded = 2 * time.Minute
	availabilityTTLAcceleratorSoldOut       = 1 * time.Minute
	availabilityTTLAcceleratorUnsupported   = 30 * time.Minute
)

type availabilityCacheEntry struct {
	result    model.AvailabilityResult
	expiresAt time.Time
}

type zoneSoldOutEntry struct {
	reason    string
	expiresAt time.Time
}

var (
	checkers          = map[string]AvailabilityChecker{}
	checkersMu        sync.RWMutex
	availabilityCache sync.Map // key: availabilityCacheKey(...) -> *availabilityCacheEntry
	availabilityGroup singleflight.Group

	zoneSoldOutMu    sync.RWMutex
	zoneSoldOutCache = map[string]map[string]zoneSoldOutEntry{} // key: holder|provider|region|instanceType -> zoneId -> entry
)

func zoneSoldOutBaseKey(ctx context.Context, provider, region, instanceType string) string {
	holder := credentialHolderFromContext(ctx)
	return holder + "|" + strings.ToLower(strings.TrimSpace(provider)) + "|" +
		strings.ToLower(strings.TrimSpace(region)) + "|" + strings.TrimSpace(instanceType)
}

func recordZoneSoldOut(ctx context.Context, provider, region, instanceType, zoneId, reason string, expiresAt time.Time) {
	if zoneId == "" {
		return
	}
	bKey := zoneSoldOutBaseKey(ctx, provider, region, instanceType)
	zoneSoldOutMu.Lock()
	defer zoneSoldOutMu.Unlock()
	if zoneSoldOutCache[bKey] == nil {
		zoneSoldOutCache[bKey] = make(map[string]zoneSoldOutEntry)
	}
	zoneSoldOutCache[bKey][zoneId] = zoneSoldOutEntry{
		reason:    reason,
		expiresAt: expiresAt,
	}
}

// applyZoneSoldOutOverlay overlays any active (non-expired) per-zone SoldOut entries
// recorded from provisioning failures onto an AvailabilityResult.
//
// Why this matters (especially for AWS / GCP):
//   - AWS DescribeInstanceTypeOfferings and GCP machineTypes.aggregatedList report whether
//     a spec is *offered* in each AZ, not real-time stock.
//   - When a VM creation fails in one AZ (e.g. us-west-2a InsufficientInstanceCapacity),
//     only us-west-2a is SoldOut; us-west-2b and us-west-2c may still have stock.
//   - Overlaying the per-zone SoldOut map marks us-west-2a as Available=false while keeping
//     the region Available=true as long as at least one other zone remains available, and flips
//     the region to SoldOut when all zones in the region are exhausted.
func applyZoneSoldOutOverlay(ctx context.Context, provider string, q model.AvailabilityQuery, r *model.AvailabilityResult) {
	if r == nil {
		return
	}
	bKey := zoneSoldOutBaseKey(ctx, provider, q.Region, q.InstanceType)
	now := time.Now()

	zoneSoldOutMu.RLock()
	zoneMap := zoneSoldOutCache[bKey]
	if len(zoneMap) == 0 {
		zoneSoldOutMu.RUnlock()
		return
	}
	activeSoldOut := make(map[string]zoneSoldOutEntry, len(zoneMap))
	for zId, entry := range zoneMap {
		if now.Before(entry.expiresAt) {
			activeSoldOut[zId] = entry
		}
	}
	zoneSoldOutMu.RUnlock()

	if len(activeSoldOut) == 0 {
		return
	}

	// If the caller queried a specific PreferredZone that is currently SoldOut, mark it SoldOut.
	if pref := strings.TrimSpace(q.PreferredZone); pref != "" {
		if entry, ok := activeSoldOut[pref]; ok {
			r.Available = false
			r.Status = model.AvailabilitySoldOut
			r.Reason = entry.reason
			exp := entry.expiresAt
			r.RefreshAfter = &exp
			r.TTLSeconds = int(resolveAvailabilityTTL(q, model.AvailabilitySoldOut).Seconds())
		}
	}

	if len(r.Zones) == 0 {
		return
	}

	seenZones := make(map[string]bool, len(r.Zones))
	anyAvailable := false
	var soldOutZones []string
	var availZones []string
	var earliestExpiry time.Time

	for i := range r.Zones {
		z := &r.Zones[i]
		seenZones[z.ZoneId] = true
		if entry, ok := activeSoldOut[z.ZoneId]; ok {
			z.Available = false
			z.Status = string(model.AvailabilitySoldOut)
			z.Reason = entry.reason
			if earliestExpiry.IsZero() || entry.expiresAt.Before(earliestExpiry) {
				earliestExpiry = entry.expiresAt
			}
		}
		if z.Available {
			anyAvailable = true
			availZones = append(availZones, z.ZoneId)
		} else if z.ZoneId != "" {
			soldOutZones = append(soldOutZones, z.ZoneId)
		}
	}

	// Also append any active SoldOut zones not yet present in r.Zones (if PreferredZone is empty).
	if strings.TrimSpace(q.PreferredZone) == "" {
		for zId, entry := range activeSoldOut {
			if !seenZones[zId] {
				r.Zones = append(r.Zones, model.ZoneAvailability{
					ZoneId:    zId,
					Available: false,
					Status:    string(model.AvailabilitySoldOut),
					Reason:    entry.reason,
				})
				soldOutZones = append(soldOutZones, zId)
				if earliestExpiry.IsZero() || entry.expiresAt.Before(earliestExpiry) {
					earliestExpiry = entry.expiresAt
				}
			}
		}
	}

	// Re-evaluate region-level status only if the spec wasn't already rejected for QuotaExceeded/Unsupported.
	if r.Status == model.AvailabilityQuotaExceeded || r.Status == model.AvailabilityUnsupported {
		return
	}

	if anyAvailable {
		r.Available = true
		r.Status = model.AvailabilityAvailable
		if len(soldOutZones) > 0 {
			r.Reason = fmt.Sprintf("zone(s) [%s] temporarily out of stock (SoldOut); available in [%s]",
				strings.Join(soldOutZones, ", "), strings.Join(availZones, ", "))
		}
	} else {
		r.Available = false
		r.Status = model.AvailabilitySoldOut
		if r.Reason == "" && len(soldOutZones) > 0 {
			r.Reason = fmt.Sprintf("all checked zone(s) [%s] in region %s are temporarily out of stock (SoldOut)",
				strings.Join(soldOutZones, ", "), q.Region)
		}
		if !earliestExpiry.IsZero() {
			r.RefreshAfter = &earliestExpiry
			r.TTLSeconds = int(resolveAvailabilityTTL(q, model.AvailabilitySoldOut).Seconds())
		}
	}
}

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

// HasAvailabilityChecker reports whether a live availability checker is registered for provider.
func HasAvailabilityChecker(provider string) bool {
	checkersMu.RLock()
	defer checkersMu.RUnlock()
	_, ok := checkers[strings.ToLower(strings.TrimSpace(provider))]
	return ok
}

func isAcceleratorQuery(q model.AvailabilityQuery) bool {
	return q.AcceleratorCount > 0 || strings.TrimSpace(q.AcceleratorModel) != ""
}

func resolveAvailabilityTTL(q model.AvailabilityQuery, status model.AvailabilityStatus) time.Duration {
	isAccel := isAcceleratorQuery(q)
	switch status {
	case model.AvailabilitySoldOut:
		if isAccel {
			return availabilityTTLAcceleratorSoldOut
		}
		return availabilityTTLStandardSoldOut
	case model.AvailabilityQuotaExceeded:
		if isAccel {
			return availabilityTTLAcceleratorQuotaExceeded
		}
		return availabilityTTLStandardQuotaExceeded
	case model.AvailabilityUnsupported:
		if isAccel {
			return availabilityTTLAcceleratorUnsupported
		}
		return availabilityTTLStandardUnsupported
	default:
		if isAccel {
			return availabilityTTLAcceleratorAvailable
		}
		return availabilityTTLStandardAvailable
	}
}

func inferAvailabilityStatus(r model.AvailabilityResult) model.AvailabilityStatus {
	if r.Status != "" {
		return r.Status
	}
	if r.Available {
		return model.AvailabilityAvailable
	}
	lower := strings.ToLower(r.Reason)
	switch {
	case strings.Contains(lower, "quota") || strings.Contains(lower, "opt-in") || strings.Contains(lower, "limit"):
		return model.AvailabilityQuotaExceeded
	case strings.Contains(lower, "not offered") || strings.Contains(lower, "not listed") || strings.Contains(lower, "does not exist"):
		return model.AvailabilityUnsupported
	default:
		return model.AvailabilitySoldOut
	}
}

func cloneAvailabilityResult(in model.AvailabilityResult) model.AvailabilityResult {
	out := in
	if len(in.Zones) > 0 {
		out.Zones = make([]model.ZoneAvailability, len(in.Zones))
		copy(out.Zones, in.Zones)
	}
	return out
}

// LookupCachedAvailability performs a 0ms non-blocking lookup in the availability cache
// for the current credential holder in ctx. It never invokes a CSP API.
func LookupCachedAvailability(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, bool) {
	provider := strings.ToLower(strings.TrimSpace(q.Provider))
	now := time.Now()

	key := availabilityCacheKey(ctx, provider, q)
	if v, ok := availabilityCache.Load(key); ok {
		if e, ok := v.(*availabilityCacheEntry); ok && now.Before(e.expiresAt) {
			r := cloneAvailabilityResult(e.result)
			r.Cached = true
			applyZoneSoldOutOverlay(ctx, provider, q, &r)
			return r, true
		}
	}

	// Fallback: if the caller queried with a specific disk category or zone hint,
	// also check the region-wide entry (empty disk / empty zone) so write-through
	// failures or region-wide checks are visible on spec lists.
	if q.SystemDiskCategory != "" || q.PreferredZone != "" {
		fallbackQ := q
		fallbackQ.SystemDiskCategory = ""
		fallbackQ.PreferredZone = ""
		fallbackKey := availabilityCacheKey(ctx, provider, fallbackQ)
		if v, ok := availabilityCache.Load(fallbackKey); ok {
			if e, ok := v.(*availabilityCacheEntry); ok && now.Before(e.expiresAt) {
				r := cloneAvailabilityResult(e.result)
				r.Cached = true
				applyZoneSoldOutOverlay(ctx, provider, q, &r)
				return r, true
			}
		}
	}

	return model.AvailabilityResult{}, false
}

// RecordProvisioningFailureToCache immediately writes through a real VM provisioning failure
// (SoldOut / QuotaExceeded) into the availability cache for the credential holder in ctx.
//
// Zone-aware behavior:
//   - AccountQuota and RegionCapacity are region-wide: the entire region is marked Unavailable.
//   - ZoneCapacity (e.g. AWS InsufficientInstanceCapacity in us-west-2a) is zone-scoped:
//     the attempted zone is recorded in zoneSoldOutCache (1m TTL) and overlaid onto the region's
//     Zones list. If other zones in the region remain available, the region stays Available
//     (with UnavailableZones showing the SoldOut zone); if no other zones remain available,
//     the region flips to SoldOut.
func RecordProvisioningFailureToCache(ctx context.Context, q model.AvailabilityQuery, failure model.ProvisioningFailure) {
	provider := strings.ToLower(strings.TrimSpace(q.Provider))
	region := strings.TrimSpace(q.Region)
	instanceType := strings.TrimSpace(q.InstanceType)
	if provider == "" || region == "" || instanceType == "" {
		return
	}

	var status model.AvailabilityStatus
	switch failure.Class {
	case model.FailureAccountQuota:
		status = model.AvailabilityQuotaExceeded
	case model.FailureZoneCapacity, model.FailureRegionCapacity:
		status = model.AvailabilitySoldOut
	default:
		// Non-availability failure (e.g. image, network, auth, transient): do not poison spec availability cache.
		return
	}

	now := time.Now()
	ttl := resolveAvailabilityTTL(q, status)
	expiresAt := now.Add(ttl)
	reason := strings.TrimSpace(failure.Message)
	if reason == "" {
		reason = strings.TrimSpace(failure.RawMessage)
	}

	// 1. If this is a zone-scoped capacity failure with a known AttemptedZone, record the zone SoldOut overlay.
	// Note: Many CSPs (e.g. AWS DescribeInstanceTypeOfferings, GCP machineTypes, Azure ResourceSkus) only
	// report catalog support rather than real-time physical host stock. Keep the per-zone SoldOut overlay
	// from an actual VM creation rejection for 10 minutes so subsequent placement and UI queries avoid the
	// exhausted zone, while region-wide SoldOut (when all zones are exhausted) still refreshes in 1m.
	if failure.Class == model.FailureZoneCapacity && failure.AttemptedZone != "" {
		const zoneOverlayTTL = 10 * time.Minute
		zoneOverlayExpiresAt := now.Add(zoneOverlayTTL)
		recordZoneSoldOut(ctx, provider, region, instanceType, failure.AttemptedZone, reason, zoneOverlayExpiresAt)

		// Store zone-pinned entry (PreferredZone = failure.AttemptedZone).
		zoneQ := q
		zoneQ.PreferredZone = failure.AttemptedZone
		zoneEntry := &availabilityCacheEntry{
			result: model.AvailabilityResult{
				Provider:     provider,
				Region:       region,
				InstanceType: instanceType,
				Available:    false,
				Status:       model.AvailabilitySoldOut,
				Zones: []model.ZoneAvailability{
					{
						ZoneId:    failure.AttemptedZone,
						Available: false,
						Status:    string(model.AvailabilitySoldOut),
						Reason:    reason,
					},
				},
				Reason:       reason,
				Source:       provider + ":ProvisioningFailureWriteThrough",
				Cached:       true,
				QueriedAt:    now,
				RefreshAfter: &zoneOverlayExpiresAt,
				TTLSeconds:   int(zoneOverlayTTL.Seconds()),
			},
			expiresAt: zoneOverlayExpiresAt,
		}
		availabilityCache.Store(availabilityCacheKey(ctx, provider, zoneQ), zoneEntry)

		// Update or seed the region-wide entry (PreferredZone = "", SystemDiskCategory = "").
		regionQ := q
		regionQ.SystemDiskCategory = ""
		regionQ.PreferredZone = ""
		regionKey := availabilityCacheKey(ctx, provider, regionQ)

		if v, ok := availabilityCache.Load(regionKey); ok {
			if existing, ok := v.(*availabilityCacheEntry); ok && now.Before(existing.expiresAt) {
				updated := cloneAvailabilityResult(existing.result)
				// Add SuggestedZones if CSP provided them and they are not yet in updated.Zones.
				seen := map[string]bool{}
				for _, z := range updated.Zones {
					seen[z.ZoneId] = true
				}
				for _, sz := range failure.SuggestedZones {
					if sz != "" && !seen[sz] {
						updated.Zones = append(updated.Zones, model.ZoneAvailability{
							ZoneId:    sz,
							Available: true,
							Status:    string(model.AvailabilityAvailable),
						})
						seen[sz] = true
					}
				}
				applyZoneSoldOutOverlay(ctx, provider, regionQ, &updated)
				newExp := existing.expiresAt
				if !updated.Available {
					newExp = expiresAt
					updated.RefreshAfter = &expiresAt
					updated.TTLSeconds = int(ttl.Seconds())
				} else if newExp.Before(zoneOverlayExpiresAt) {
					newExp = zoneOverlayExpiresAt
				}
				availabilityCache.Store(regionKey, &availabilityCacheEntry{
					result:    updated,
					expiresAt: newExp,
				})
				log.Info().
					Str("provider", provider).
					Str("region", region).
					Str("zone", failure.AttemptedZone).
					Str("instanceType", instanceType).
					Str("regionStatus", string(updated.Status)).
					Dur("ttl", zoneOverlayTTL).
					Msg("availability cache updated via zone-level provisioning failure write-through")
				return
			}
		}

		// No existing region-wide cache entry: build one using AttemptedZone + SuggestedZones (if any).
		zones := []model.ZoneAvailability{
			{
				ZoneId:    failure.AttemptedZone,
				Available: false,
				Status:    string(model.AvailabilitySoldOut),
				Reason:    reason,
			},
		}
		for _, sz := range failure.SuggestedZones {
			if sz != "" && sz != failure.AttemptedZone {
				zones = append(zones, model.ZoneAvailability{
					ZoneId:    sz,
					Available: true,
					Status:    string(model.AvailabilityAvailable),
				})
			}
		}
		seedExp := expiresAt
		if len(zones) > 1 {
			seedExp = zoneOverlayExpiresAt
		}
		regionResult := model.AvailabilityResult{
			Provider:     provider,
			Region:       region,
			InstanceType: instanceType,
			Zones:        zones,
			Reason:       reason,
			Source:       provider + ":ProvisioningFailureWriteThrough",
			Cached:       true,
			QueriedAt:    now,
			RefreshAfter: &seedExp,
			TTLSeconds:   int(seedExp.Sub(now).Seconds()),
		}
		applyZoneSoldOutOverlay(ctx, provider, regionQ, &regionResult)
		if !regionResult.Available {
			regionResult.RefreshAfter = &expiresAt
			regionResult.TTLSeconds = int(ttl.Seconds())
			seedExp = expiresAt
		}
		availabilityCache.Store(regionKey, &availabilityCacheEntry{
			result:    regionResult,
			expiresAt: seedExp,
		})
		log.Info().
			Str("provider", provider).
			Str("region", region).
			Str("zone", failure.AttemptedZone).
			Str("instanceType", instanceType).
			Str("regionStatus", string(regionResult.Status)).
			Dur("ttl", seedExp.Sub(now)).
			Msg("availability cache seeded via zone-level provisioning failure write-through")
		return
	}

	// 2. Region-wide failure (AccountQuota or RegionCapacity or unspecified zone).
	entryResult := model.AvailabilityResult{
		Provider:     provider,
		Region:       region,
		InstanceType: instanceType,
		Available:    false,
		Status:       status,
		Reason:       reason,
		Source:       provider + ":ProvisioningFailureWriteThrough",
		Cached:       true,
		QueriedAt:    now,
		RefreshAfter: &expiresAt,
		TTLSeconds:   int(ttl.Seconds()),
	}

	entry := &availabilityCacheEntry{
		result:    entryResult,
		expiresAt: expiresAt,
	}

	exactKey := availabilityCacheKey(ctx, provider, q)
	availabilityCache.Store(exactKey, entry)

	regionQ := q
	regionQ.SystemDiskCategory = ""
	regionQ.PreferredZone = ""
	regionKey := availabilityCacheKey(ctx, provider, regionQ)
	if regionKey != exactKey {
		availabilityCache.Store(regionKey, entry)
	}

	log.Info().
		Str("provider", provider).
		Str("region", region).
		Str("instanceType", instanceType).
		Str("status", string(status)).
		Dur("ttl", ttl).
		Msg("availability cache updated via region-wide provisioning failure write-through")
}

// CheckAvailability dispatches to the registered checker for q.Provider and
// caches the result with asymmetric TTLs. Concurrent misses for the same
// (credentialHolder, provider, region, instanceType, disk, zone) are deduplicated via singleflight.
//
// If no checker is registered for the provider, the function returns a
// "no-checker" result with Available=true and Status=Unverified so that callers can proceed.
// All checker errors are also turned into non-fatal "Available=true" results
// with Status=Unverified and the error captured in Reason; this is intentional to avoid blocking
// provisioning on pre-check failures.
func CheckAvailability(ctx context.Context, q model.AvailabilityQuery) model.AvailabilityResult {
	provider := strings.ToLower(strings.TrimSpace(q.Provider))

	checkersMu.RLock()
	c, ok := checkers[provider]
	checkersMu.RUnlock()

	if !ok {
		r := model.AvailabilityResult{
			Provider:     provider,
			Region:       q.Region,
			InstanceType: q.InstanceType,
			Available:    true,
			Status:       model.AvailabilityUnverified,
			Reason:       "no availability checker registered for provider; assumed available",
			Source:       "none",
			QueriedAt:    time.Now(),
		}
		applyZoneSoldOutOverlay(ctx, provider, q, &r)
		return r
	}

	key := availabilityCacheKey(ctx, provider, q)

	// Fast path: cache hit (unless ForceRefresh is requested).
	if !q.ForceRefresh {
		if v, ok := availabilityCache.Load(key); ok {
			if e, ok := v.(*availabilityCacheEntry); ok && time.Now().Before(e.expiresAt) {
				r := cloneAvailabilityResult(e.result)
				r.Cached = true
				applyZoneSoldOutOverlay(ctx, provider, q, &r)
				return r
			}
		}
	}

	// Slow path: dedupe concurrent misses.
	v, _, _ := availabilityGroup.Do(key, func() (any, error) {
		// Re-check cache inside singleflight to avoid redundant API calls
		// when a prior in-flight request just populated it.
		if !q.ForceRefresh {
			if v, ok := availabilityCache.Load(key); ok {
				if e, ok := v.(*availabilityCacheEntry); ok && time.Now().Before(e.expiresAt) {
					r := cloneAvailabilityResult(e.result)
					r.Cached = true
					applyZoneSoldOutOverlay(ctx, provider, q, &r)
					return r, nil
				}
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
				Status:       model.AvailabilityUnverified,
				Reason:       "availability checker failed: " + err.Error(),
				Source:       c.Provider() + ":error",
				QueriedAt:    time.Now(),
			}, nil
		}
		if r.QueriedAt.IsZero() {
			r.QueriedAt = time.Now()
		}
		r.Status = inferAvailabilityStatus(r)
		applyZoneSoldOutOverlay(ctx, provider, q, &r)

		ttl := resolveAvailabilityTTL(q, r.Status)
		expiresAt := r.QueriedAt.Add(ttl)
		if r.RefreshAfter == nil {
			r.RefreshAfter = &expiresAt
		}
		if r.TTLSeconds == 0 {
			r.TTLSeconds = int(ttl.Seconds())
		}

		entry := &availabilityCacheEntry{
			result:    r,
			expiresAt: expiresAt,
		}
		availabilityCache.Store(key, entry)

		// If the query included a SystemDiskCategory but no PreferredZone, and the region-wide
		// key is not yet populated, also seed the region-wide key so spec list lookups benefit.
		if q.SystemDiskCategory != "" && q.PreferredZone == "" {
			regionQ := q
			regionQ.SystemDiskCategory = ""
			regionKey := availabilityCacheKey(ctx, provider, regionQ)
			if _, exists := availabilityCache.Load(regionKey); !exists {
				availabilityCache.Store(regionKey, entry)
			}
		}

		return r, nil
	})
	return v.(model.AvailabilityResult)
}

func credentialHolderFromContext(ctx context.Context) string {
	if ctx == nil {
		return model.DefaultCredentialHolder
	}
	if v, ok := ctx.Value(model.CtxKeyCredentialHolder).(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return model.DefaultCredentialHolder
}

func availabilityCacheKey(ctx context.Context, provider string, q model.AvailabilityQuery) string {
	// Include credentialHolder so quota/subscription checks are isolated per account,
	// and PreferredZone so zone-pinned queries do not collide with region-wide queries.
	holder := credentialHolderFromContext(ctx)
	return holder + "|" + provider + "|" + strings.ToLower(strings.TrimSpace(q.Region)) + "|" +
		strings.TrimSpace(q.InstanceType) + "|" + strings.TrimSpace(q.SystemDiskCategory) + "|" +
		strings.TrimSpace(q.PreferredZone)
}
