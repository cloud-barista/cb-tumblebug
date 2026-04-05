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

// Package azure provides direct Azure API call utilities for cases where
// CB-Spider is too slow or does not provide adequate functionality.
package azure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/rs/zerolog/log"
)

// azureCreds holds Azure service principal credentials fetched from OpenBao.
type azureCreds struct {
	ClientID       string
	ClientSecret   string
	TenantID       string
	SubscriptionID string
}

// SpecCheckResult holds the detailed result of an Azure spec availability check.
type SpecCheckResult struct {
	Available    bool   // true if the spec can be provisioned
	Reason       string // human-readable reason when not available
	VCPUs        int32  // vCPU count required by this spec
	QuotaFamily  string // vCPU family name (e.g., "standardDSv3Family")
	QuotaCurrent int64  // current vCPU usage in this family
	QuotaLimit   int64  // vCPU limit for this family
}

// skuDetail stores parsed SKU information.
type skuDetail struct {
	family     string
	vcpus      int32
	restricted bool
	reasonCode string // "NotAvailableForSubscription", "QuotaId"
}

type specCacheEntry struct {
	skus      map[string]*skuDetail
	fetchedAt time.Time
}

type quotaUsage struct {
	currentValue int64
	limit        int64
}

type quotaCacheEntry struct {
	usages    map[string]*quotaUsage
	fetchedAt time.Time
}

var (
	specCache    = make(map[string]*specCacheEntry)
	specCacheMu  sync.RWMutex
	quotaCache   = make(map[string]*quotaCacheEntry)
	quotaCacheMu sync.RWMutex
	cacheTTL     = 10 * time.Minute
)

func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			specCacheMu.Lock()
			for k, e := range specCache {
				if now.Sub(e.fetchedAt) > cacheTTL {
					delete(specCache, k)
				}
			}
			specCacheMu.Unlock()
			quotaCacheMu.Lock()
			for k, e := range quotaCache {
				if now.Sub(e.fetchedAt) > cacheTTL {
					delete(quotaCache, k)
				}
			}
			quotaCacheMu.Unlock()
		}
	}()
}

// CheckSpecAvailability checks whether a given Azure VM spec is available
// in the specified region by checking both SKU restrictions (including
// opt-in / subscription-level) and vCPU quota.
func CheckSpecAvailability(ctx context.Context, region, vmSize string) (*SpecCheckResult, error) {
	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	cacheKey := creds.SubscriptionID + "+" + region
	vmSizeLower := strings.ToLower(vmSize)

	// Step 1: SKU existence & restrictions
	sku, err := getSKUDetail(ctx, creds, credential, cacheKey, region, vmSizeLower)
	if err != nil {
		return nil, err
	}

	if sku == nil {
		return &SpecCheckResult{
			Available: false,
			Reason:    fmt.Sprintf("VM size '%s' does not exist in region '%s'", vmSize, region),
		}, nil
	}

	if sku.restricted {
		reason := fmt.Sprintf("VM size '%s' is restricted in region '%s'", vmSize, region)
		switch sku.reasonCode {
		case "NotAvailableForSubscription":
			reason = fmt.Sprintf("VM size '%s' requires opt-in for your subscription in region '%s' (request via Azure Portal > Subscription > Usage + quotas)", vmSize, region)
		case "QuotaId":
			reason = fmt.Sprintf("VM size '%s' is restricted by quota policy in region '%s'", vmSize, region)
		}
		return &SpecCheckResult{
			Available:   false,
			Reason:      reason,
			VCPUs:       sku.vcpus,
			QuotaFamily: sku.family,
		}, nil
	}

	result := &SpecCheckResult{
		Available:   true,
		VCPUs:       sku.vcpus,
		QuotaFamily: sku.family,
	}

	// Step 2: vCPU quota check
	if sku.family != "" && sku.vcpus > 0 {
		quota, qErr := getQuotaUsage(ctx, creds, credential, cacheKey, region, sku.family)
		if qErr != nil {
			log.Warn().Err(qErr).Str("region", region).Msg("[AzureSpec] Failed to check vCPU quota, skipping quota validation")
		} else if quota != nil {
			result.QuotaCurrent = quota.currentValue
			result.QuotaLimit = quota.limit
			remaining := quota.limit - quota.currentValue
			if int64(sku.vcpus) > remaining {
				result.Available = false
				result.Reason = fmt.Sprintf(
					"Insufficient vCPU quota for '%s' in region '%s': need %d vCPUs but only %d remaining (used %d / limit %d, family: %s). Request increase via Azure Portal > Subscription > Usage + quotas",
					vmSize, region, sku.vcpus, remaining, quota.currentValue, quota.limit, sku.family,
				)
			}
		}
	}

	log.Debug().
		Str("region", region).
		Str("vmSize", vmSize).
		Bool("available", result.Available).
		Str("reason", result.Reason).
		Int32("vcpus", result.VCPUs).
		Str("family", result.QuotaFamily).
		Msg("[AzureSpec] Spec check completed")

	return result, nil
}

// getSKUDetail retrieves SKU detail for a VM size, using cache if available.
func getSKUDetail(ctx context.Context, creds *azureCreds, cred *azidentity.ClientSecretCredential, cacheKey, region, vmSizeLower string) (*skuDetail, error) {
	specCacheMu.RLock()
	entry, ok := specCache[cacheKey]
	specCacheMu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < cacheTTL {
		log.Debug().Str("region", region).Msg("[AzureSpec] SKU cache hit")
		return entry.skus[vmSizeLower], nil
	}

	log.Debug().Str("region", region).Msg("[AzureSpec] Fetching resource SKUs from Azure")
	skus, err := fetchVMSKUs(ctx, creds, cred, region)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Azure SKUs for region %s: %w", region, err)
	}

	specCacheMu.Lock()
	specCache[cacheKey] = &specCacheEntry{skus: skus, fetchedAt: time.Now()}
	specCacheMu.Unlock()

	return skus[vmSizeLower], nil
}

// getQuotaUsage retrieves quota usage for a VM family, using cache if available.
func getQuotaUsage(ctx context.Context, creds *azureCreds, cred *azidentity.ClientSecretCredential, cacheKey, region, family string) (*quotaUsage, error) {
	familyLower := strings.ToLower(family)

	quotaCacheMu.RLock()
	entry, ok := quotaCache[cacheKey]
	quotaCacheMu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < cacheTTL {
		log.Debug().Str("region", region).Msg("[AzureSpec] Quota cache hit")
		return entry.usages[familyLower], nil
	}

	log.Debug().Str("region", region).Msg("[AzureSpec] Fetching vCPU quotas from Azure")
	usages, err := fetchQuotas(ctx, creds, cred, region)
	if err != nil {
		return nil, err
	}

	quotaCacheMu.Lock()
	quotaCache[cacheKey] = &quotaCacheEntry{usages: usages, fetchedAt: time.Now()}
	quotaCacheMu.Unlock()

	return usages[familyLower], nil
}

// fetchVMSKUs queries Azure Resource SKUs API and returns detailed info per VM size.
func fetchVMSKUs(ctx context.Context, creds *azureCreds, cred *azidentity.ClientSecretCredential, region string) (map[string]*skuDetail, error) {
	client, err := armcompute.NewResourceSKUsClient(creds.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ResourceSKUs client: %w", err)
	}

	skus := make(map[string]*skuDetail)
	pager := client.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: toPtr(fmt.Sprintf("location eq '%s'", region)),
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list resource SKUs: %w", err)
		}

		for _, sku := range page.Value {
			if sku.ResourceType == nil || sku.Name == nil {
				continue
			}
			if !strings.EqualFold(*sku.ResourceType, "virtualMachines") {
				continue
			}

			d := &skuDetail{}
			if sku.Family != nil {
				d.family = *sku.Family
			}
			for _, cap := range sku.Capabilities {
				if cap.Name != nil && cap.Value != nil && strings.EqualFold(*cap.Name, "vCPUs") {
					if v, parseErr := strconv.ParseInt(*cap.Value, 10, 32); parseErr == nil {
						d.vcpus = int32(v)
					}
					break
				}
			}

			// Check restrictions (Location type with reason code)
			for _, r := range sku.Restrictions {
				if r.Type == nil {
					continue
				}
				if *r.Type == armcompute.ResourceSKURestrictionsTypeLocation && r.RestrictionInfo != nil {
					for _, loc := range r.RestrictionInfo.Locations {
						if loc != nil && strings.EqualFold(*loc, region) {
							d.restricted = true
							if r.ReasonCode != nil {
								d.reasonCode = string(*r.ReasonCode)
							}
							break
						}
					}
				}
				if d.restricted {
					break
				}
			}

			skus[strings.ToLower(*sku.Name)] = d
		}
	}

	return skus, nil
}

// fetchQuotas queries Azure Compute Usage API and returns quota usage per resource name.
func fetchQuotas(ctx context.Context, creds *azureCreds, cred *azidentity.ClientSecretCredential, region string) (map[string]*quotaUsage, error) {
	client, err := armcompute.NewUsageClient(creds.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Usage client: %w", err)
	}

	usages := make(map[string]*quotaUsage)
	pager := client.NewListPager(region, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list compute usages: %w", err)
		}

		for _, u := range page.Value {
			if u.Name == nil || u.Name.Value == nil {
				continue
			}
			var cur int64
			if u.CurrentValue != nil {
				cur = int64(*u.CurrentValue)
			}
			var lim int64
			if u.Limit != nil {
				lim = *u.Limit
			}
			usages[strings.ToLower(*u.Name.Value)] = &quotaUsage{
				currentValue: cur,
				limit:        lim,
			}
		}
	}

	return usages, nil
}

// getCreds fetches Azure credentials from OpenBao based on the credential holder in context.
func getCreds(ctx context.Context) (*azureCreds, error) {
	path := csp.BuildSecretPath(ctx, "azure")
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return nil, err
	}

	clientID := csp.GetString(data, "ARM_CLIENT_ID")
	clientSecret := csp.GetString(data, "ARM_CLIENT_SECRET")
	tenantID := csp.GetString(data, "ARM_TENANT_ID")
	subscriptionID := csp.GetString(data, "ARM_SUBSCRIPTION_ID")

	if clientID == "" || clientSecret == "" || tenantID == "" || subscriptionID == "" {
		return nil, fmt.Errorf("Azure credentials incomplete in OpenBao (need ARM_CLIENT_ID, ARM_CLIENT_SECRET, ARM_TENANT_ID, ARM_SUBSCRIPTION_ID)")
	}

	return &azureCreds{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TenantID:       tenantID,
		SubscriptionID: subscriptionID,
	}, nil
}

func toPtr[T any](v T) *T {
	return &v
}
