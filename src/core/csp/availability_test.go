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

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

type mockAvailabilityChecker struct {
	provider string
	calls    atomic.Int32
	fn       func(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error)
}

func (m *mockAvailabilityChecker) Provider() string { return m.provider }

func (m *mockAvailabilityChecker) CheckInstance(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
	m.calls.Add(1)
	return m.fn(ctx, q)
}

func TestResolveAvailabilityTTL(t *testing.T) {
	cpuQuery := model.AvailabilityQuery{
		Provider:     "mock-csp",
		Region:       "ap-northeast-2",
		InstanceType: "t3.medium",
	}
	gpuQuery := model.AvailabilityQuery{
		Provider:         "mock-csp",
		Region:           "ap-northeast-2",
		InstanceType:     "g5.xlarge",
		AcceleratorModel: "NVIDIA A10G",
		AcceleratorCount: 1,
	}

	if got := resolveAvailabilityTTL(cpuQuery, model.AvailabilityAvailable); got != time.Hour {
		t.Fatalf("CPU Available TTL = %v, want 1h", got)
	}
	if got := resolveAvailabilityTTL(cpuQuery, model.AvailabilityQuotaExceeded); got != 5*time.Minute {
		t.Fatalf("CPU QuotaExceeded TTL = %v, want 5m", got)
	}
	if got := resolveAvailabilityTTL(cpuQuery, model.AvailabilitySoldOut); got != time.Minute {
		t.Fatalf("CPU SoldOut TTL = %v, want 1m", got)
	}

	if got := resolveAvailabilityTTL(gpuQuery, model.AvailabilityAvailable); got != 5*time.Minute {
		t.Fatalf("GPU Available TTL = %v, want 5m", got)
	}
	if got := resolveAvailabilityTTL(gpuQuery, model.AvailabilityQuotaExceeded); got != 2*time.Minute {
		t.Fatalf("GPU QuotaExceeded TTL = %v, want 2m", got)
	}
	if got := resolveAvailabilityTTL(gpuQuery, model.AvailabilitySoldOut); got != time.Minute {
		t.Fatalf("GPU SoldOut TTL = %v, want 1m", got)
	}
}

func TestCheckAvailability_CredentialHolderIsolationAndZoneWriteThrough(t *testing.T) {
	provider := "mock-holder-test"
	checker := &mockAvailabilityChecker{
		provider: provider,
		fn: func(ctx context.Context, q model.AvailabilityQuery) (model.AvailabilityResult, error) {
			holder := credentialHolderFromContext(ctx)
			if holder == "tenant-quota-full" {
				return model.AvailabilityResult{
					Provider:     provider,
					Region:       q.Region,
					InstanceType: q.InstanceType,
					Available:    false,
					Status:       model.AvailabilityQuotaExceeded,
					Reason:       "vCPU quota exceeded for tenant-quota-full",
					Source:       provider + ":mock",
				}, nil
			}
			return model.AvailabilityResult{
				Provider:     provider,
				Region:       q.Region,
				InstanceType: q.InstanceType,
				Available:    true,
				Status:       model.AvailabilityAvailable,
				Zones: []model.ZoneAvailability{
					{ZoneId: "us-west-2a", Available: true, Status: "Available"},
					{ZoneId: "us-west-2b", Available: true, Status: "Available"},
				},
				Source: provider + ":mock",
			}, nil
		},
	}
	RegisterAvailabilityChecker(checker)

	q := model.AvailabilityQuery{
		Provider:     provider,
		Region:       "us-west-2",
		InstanceType: "g5.xlarge",
	}

	ctxAdmin := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, "admin")
	ctxTenant := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, "tenant-quota-full")

	resAdmin := CheckAvailability(ctxAdmin, q)
	if !resAdmin.Available || resAdmin.Status != model.AvailabilityAvailable {
		t.Fatalf("admin result = %+v, want Available", resAdmin)
	}
	if resAdmin.TTLSeconds != 3600 || resAdmin.RefreshAfter == nil {
		t.Fatalf("admin TTLSeconds = %d, want 3600", resAdmin.TTLSeconds)
	}

	// Second call for a different credential holder must not reuse admin's cache entry.
	resTenant := CheckAvailability(ctxTenant, q)
	if resTenant.Available || resTenant.Status != model.AvailabilityQuotaExceeded {
		t.Fatalf("tenant result = %+v, want QuotaExceeded", resTenant)
	}
	if resTenant.TTLSeconds != 300 {
		t.Fatalf("tenant QuotaExceeded TTLSeconds = %d, want 300", resTenant.TTLSeconds)
	}

	// 1) Simulate a single-zone provisioning failure (SoldOut in us-west-2a only).
	// Since us-west-2b is still available, the region remains Available while us-west-2a is marked SoldOut.
	RecordProvisioningFailureToCache(ctxAdmin, q, model.ProvisioningFailure{
		Class:         model.FailureZoneCapacity,
		Provider:      provider,
		Region:        q.Region,
		AttemptedZone: "us-west-2a",
		Message:       "InsufficientInstanceCapacity in us-west-2a",
	})

	cachedAdmin, ok := LookupCachedAvailability(ctxAdmin, q)
	if !ok {
		t.Fatalf("expected cached entry for admin after write-through")
	}
	if !cachedAdmin.Available || cachedAdmin.Status != model.AvailabilityAvailable {
		t.Fatalf("expected region to stay Available when us-west-2b still has stock, got %+v", cachedAdmin)
	}
	if cachedAdmin.Zones[0].Available || cachedAdmin.Zones[0].Status != string(model.AvailabilitySoldOut) {
		t.Fatalf("expected us-west-2a to be marked SoldOut, got %+v", cachedAdmin.Zones[0])
	}
	if !cachedAdmin.Zones[1].Available {
		t.Fatalf("expected us-west-2b to remain Available, got %+v", cachedAdmin.Zones[1])
	}

	// Zone-pinned query for us-west-2a must report SoldOut.
	zoneQuery := q
	zoneQuery.PreferredZone = "us-west-2a"
	resZoneA := CheckAvailability(ctxAdmin, zoneQuery)
	if resZoneA.Available || resZoneA.Status != model.AvailabilitySoldOut {
		t.Fatalf("zone-pinned query for us-west-2a = %+v, want SoldOut", resZoneA)
	}

	// 2) Now simulate us-west-2b also failing with ZoneCapacity -> entire region flips to SoldOut (60s TTL).
	RecordProvisioningFailureToCache(ctxAdmin, q, model.ProvisioningFailure{
		Class:         model.FailureZoneCapacity,
		Provider:      provider,
		Region:        q.Region,
		AttemptedZone: "us-west-2b",
		Message:       "InsufficientInstanceCapacity in us-west-2b",
	})

	cachedAdminAllSoldOut, ok := LookupCachedAvailability(ctxAdmin, q)
	if !ok {
		t.Fatalf("expected cached entry after second zone failure")
	}
	if cachedAdminAllSoldOut.Available || cachedAdminAllSoldOut.Status != model.AvailabilitySoldOut {
		t.Fatalf("expected region to flip to SoldOut when all zones are exhausted, got %+v", cachedAdminAllSoldOut)
	}
	if cachedAdminAllSoldOut.TTLSeconds != 60 {
		t.Fatalf("cachedAdminAllSoldOut SoldOut TTLSeconds = %d, want 60", cachedAdminAllSoldOut.TTLSeconds)
	}
}
