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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	alibabacsp "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	azurecsp "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func ResolveLatestImageForVMCreation(ctx context.Context, connectionName string, imageInfo model.ImageInfo) model.ImageInfo {
	return resolveLatestCspImage(ctx, connectionName, imageInfo)
}

// ResolveLatestCspImageNameForVMCreation is a convenience wrapper that takes a
// stored CSP image name, looks up the full ImageInfo, applies the provider-
// specific "latest image" resolution via ResolveLatestImageForVMCreation, and
// returns the (possibly updated) CSP image name to use in the VM-creation
// request to cb-spider.
//
// If the lookup fails, the original cspImageName is returned unchanged and no
// error is raised (the caller should already have validated the image via
// EnsureImageAvailable). Resolution failure is also non-fatal.
//
// Used by K8s cluster/node-group creation paths where only the CSP image
// name string (not the full ImageInfo) is threaded into the Spider request.
func ResolveLatestCspImageNameForVMCreation(ctx context.Context, nsId, connectionName, tumblebugImageId, cspImageName string) string {
	if cspImageName == "" || tumblebugImageId == "" {
		return cspImageName
	}
	imageInfo, err := GetImage(nsId, tumblebugImageId)
	if err != nil {
		// Fall back to SystemCommonNs; callers typically try both namespaces.
		imageInfo, err = GetImage(model.SystemCommonNs, tumblebugImageId)
		if err != nil {
			log.Debug().Err(err).Msgf("image '%s' not found in ns '%s' or SystemCommonNs; skipping latest resolution",
				tumblebugImageId, nsId)
			return cspImageName
		}
	}
	resolved := ResolveLatestImageForVMCreation(ctx, connectionName, imageInfo)
	if resolved.CspImageName == "" {
		return cspImageName
	}
	return resolved.CspImageName
}

// resolveLatestCspImage returns a copy of imageInfo with CspImageName
// replaced by the CSP's current "latest" equivalent, when the provider
// supports such resolution. Currently implemented for:
//   - Alibaba: uses ImageFamily (stored in imageInfo.Details) to call
//     DescribeImageFromFamily, avoiding VM-creation failures caused by
//     Alibaba rapidly deprecating individual date-stamped image IDs.
//   - Azure: uses (publisher, offer, sku) parsed from the URN to call
//     VirtualMachineImages.List with "name desc", picking the latest
//     version. Avoids failures caused by Azure deprecating older URN
//     versions while the SKU remains supported.
//
// For providers without a resolver, imageInfo is returned unchanged.
// The DB record is NEVER modified; only the returned copy carries the
// resolved id. Any failure (missing metadata, region lookup, API error,
// empty result) is non-fatal: the original imageInfo is returned and a
// warning is logged.
func resolveLatestCspImage(ctx context.Context, connectionName string, imageInfo model.ImageInfo) model.ImageInfo {
	switch {
	case strings.EqualFold(imageInfo.ProviderName, csp.Alibaba):
		return resolveLatestAlibabaImage(ctx, connectionName, imageInfo)
	case strings.EqualFold(imageInfo.ProviderName, csp.Azure):
		return resolveLatestAzureImage(ctx, connectionName, imageInfo)
	default:
		return imageInfo
	}
}

// resolveLatestAlibabaImage performs Alibaba ImageFamily-based latest resolution.
func resolveLatestAlibabaImage(ctx context.Context, connectionName string, imageInfo model.ImageInfo) model.ImageInfo {
	family := extractAlibabaImageFamily(imageInfo.Details)
	if family == "" {
		log.Debug().Msgf("alibaba image '%s' has no ImageFamily in details; skipping latest resolution",
			imageInfo.CspImageName)
		return imageInfo
	}

	region := ""
	if connectionName != "" {
		if conn, err := common.GetConnConfig(connectionName); err == nil {
			region = strings.TrimSpace(conn.RegionDetail.RegionName)
		} else {
			log.Warn().Err(err).Msgf("alibaba image resolution: failed to get conn config for '%s'", connectionName)
		}
	}
	if region == "" {
		log.Warn().Msgf("alibaba image resolution: region unavailable for connection '%s'; skipping (image=%s family=%s)",
			connectionName, imageInfo.CspImageName, family)
		return imageInfo
	}

	cacheKey := "alibaba|" + region + "|" + family
	resolvedId, err := getCachedLatestImageId(cacheKey, func() (string, error) {
		id, _, e := alibabacsp.ResolveLatestIdByFamily(ctx, region, family)
		return strings.TrimSpace(id), e
	})
	if err != nil {
		log.Warn().Err(err).Msgf("alibaba image resolution failed (region=%s family=%s original=%s); falling back to original",
			region, family, imageInfo.CspImageName)
		return imageInfo
	}
	if resolvedId == "" {
		log.Warn().Msgf("alibaba image family '%s' returned no image in region '%s'; falling back to original id '%s'",
			family, region, imageInfo.CspImageName)
		return imageInfo
	}
	if resolvedId == imageInfo.CspImageName {
		log.Debug().Msgf("alibaba image '%s' is already latest in family '%s' (region=%s)",
			imageInfo.CspImageName, family, region)
		return imageInfo
	}

	log.Info().Msgf("alibaba image resolved to latest via family: %s -> %s (family=%s region=%s)",
		imageInfo.CspImageName, resolvedId, family, region)

	// Return a copy with the resolved id; leave the DB record untouched.
	resolved := imageInfo
	resolved.CspImageName = resolvedId
	return resolved
}

// resolveLatestAzureImage performs Azure URN-based latest resolution.
// The CspImageName is expected to be a URN "publisher:offer:sku:version";
// this function picks the latest "version" for the (publisher, offer, sku)
// in the target region. Non-URN identifiers (e.g. ARM resource IDs for
// Shared Image Gallery / Custom Image) are returned unchanged.
func resolveLatestAzureImage(ctx context.Context, connectionName string, imageInfo model.ImageInfo) model.ImageInfo {
	publisher, offer, sku, _, ok := parseAzureUrn(imageInfo.CspImageName)
	if !ok {
		log.Debug().Msgf("azure image '%s' is not a URN; skipping latest resolution", imageInfo.CspImageName)
		return imageInfo
	}

	region := ""
	if connectionName != "" {
		if conn, err := common.GetConnConfig(connectionName); err == nil {
			region = strings.TrimSpace(conn.RegionDetail.RegionName)
		} else {
			log.Warn().Err(err).Msgf("azure image resolution: failed to get conn config for '%s'", connectionName)
		}
	}
	if region == "" {
		log.Warn().Msgf("azure image resolution: region unavailable for connection '%s'; skipping (image=%s)",
			connectionName, imageInfo.CspImageName)
		return imageInfo
	}

	cacheKey := "azure|" + region + "|" + publisher + "|" + offer + "|" + sku
	resolvedUrn, err := getCachedLatestImageId(cacheKey, func() (string, error) {
		urn, _, e := azurecsp.ResolveLatestUrn(ctx, region, publisher, offer, sku)
		return strings.TrimSpace(urn), e
	})
	if err != nil {
		log.Warn().Err(err).Msgf("azure image resolution failed (region=%s sku=%s:%s:%s original=%s); falling back to original",
			region, publisher, offer, sku, imageInfo.CspImageName)
		return imageInfo
	}
	if resolvedUrn == "" || resolvedUrn == imageInfo.CspImageName {
		log.Debug().Msgf("azure image '%s' is already latest in sku %s:%s:%s (region=%s)",
			imageInfo.CspImageName, publisher, offer, sku, region)
		return imageInfo
	}

	log.Info().Msgf("azure image resolved to latest via sku: %s -> %s (region=%s)",
		imageInfo.CspImageName, resolvedUrn, region)

	resolved := imageInfo
	resolved.CspImageName = resolvedUrn
	return resolved
}

// latestImageIdCache caches the resolved "latest" CSP image identifier per
// provider+region+key for a short TTL. Since the same image is typically
// resolved twice per VM creation (once at review, once at provisioning) and
// Infra objects may launch multiple VMs sharing the same SKU/family, this avoids
// duplicate SDK calls within a creation cycle without holding stale results
// long enough to miss new image releases.
var (
	latestImageIdCache    sync.Map // key string -> latestImageIdCacheEntry
	latestImageIdCacheTTL = 5 * time.Minute
)

type latestImageIdCacheEntry struct {
	value     string
	expiresAt time.Time
}

// getCachedLatestImageId returns the cached value for key, or invokes fetch
// and caches the result on success. Errors are NOT cached. An empty value
// (legitimate "no result") is cached so repeated lookups do not hammer the
// SDK.
func getCachedLatestImageId(key string, fetch func() (string, error)) (string, error) {
	if v, ok := latestImageIdCache.Load(key); ok {
		entry := v.(latestImageIdCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.value, nil
		}
		latestImageIdCache.Delete(key)
	}
	value, err := fetch()
	if err != nil {
		return "", err
	}
	latestImageIdCache.Store(key, latestImageIdCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(latestImageIdCacheTTL),
	})
	return value, nil
}

// parseAzureUrn splits an Azure image URN "publisher:offer:sku:version" into
// its four parts. Returns ok=false if the input is not a 4-part URN with all
// parts non-empty (e.g. ARM resource IDs starting with "/subscriptions/...").
func parseAzureUrn(s string) (publisher, offer, sku, version string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "/") {
		return "", "", "", "", false
	}
	parts := strings.Split(s, ":")
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	trimmed := make([]string, len(parts))
	for i, p := range parts {
		trimmed[i] = strings.TrimSpace(p)
		if trimmed[i] == "" {
			return "", "", "", "", false
		}
	}
	return trimmed[0], trimmed[1], trimmed[2], trimmed[3], true
}

// extractAlibabaImageFamily returns the Alibaba ECS ImageFamily value stored
// in the image detail key-value list. Returns "" if not present or empty.
// The lookup is case-insensitive on the key and trims surrounding whitespace
// on the value (some older records carry a leading space).
func extractAlibabaImageFamily(details []model.KeyValue) string {
	for _, kv := range details {
		if strings.EqualFold(strings.TrimSpace(kv.Key), "ImageFamily") {
			return strings.TrimSpace(kv.Value)
		}
	}
	return ""
}

// lookupRegularImageOnly looks up only regular images from CSP (Spider /vmimage API).
// Unlike LookupImage, this does NOT fall back to checking custom images (MyImage).
// This is a simpler version of LookupImage for cases where we need to distinguish
// between regular images and custom images explicitly.
// See also: LookupImage (with CustomImage fallback), LookupMyImage (CustomImage only)
