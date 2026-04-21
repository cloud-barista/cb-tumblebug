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

// Package alibaba provides direct-SDK helpers for Alibaba Cloud that
// complement the cb-spider based flow of cb-tumblebug.
//
// This file implements image-family based resolution for Alibaba ECS images.
// Rationale:
//   - Alibaba deprecates/removes individual public image IDs relatively fast
//     (date-stamped builds), which makes long-lived image assets stored in the
//     cb-tumblebug DB break when used to launch a VM.
//   - Each Alibaba public image is tagged with an "ImageFamily" (e.g.
//     "acs:ubuntu_22_04_x64") that is stable across builds. Calling
//     DescribeImageFromFamily returns the latest available image in that family.
//   - Resolving "latest id by family" at VM-creation time sidesteps the
//     deprecation issue entirely, without requiring changes to cb-spider.
package alibaba

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

// familyResolveTTL is the lifetime of a cached (region, family) -> latest image
// resolution. Alibaba publishes new public image builds for a given family
// roughly daily or slower, so a short TTL is enough to eliminate burst traffic
// from bulk VM creation while keeping staleness negligible in practice.
const familyResolveTTL = 10 * time.Minute

type familyCacheEntry struct {
	imageId      string
	creationTime string
	expiresAt    time.Time
}

// familyCache stores per-(region|family) resolution results with a TTL.
// sync.Map gives lock-free hot-path reads; cache entries are immutable once
// stored, so no per-entry locking is needed.
var familyCache sync.Map // key: "region|family" -> *familyCacheEntry

// familyGroup deduplicates concurrent misses for the same key so that a burst
// of VM creations targeting the same image family triggers at most one API call.
var familyGroup singleflight.Group

func familyCacheKey(region, family string) string {
	return region + "|" + family
}

// ResolveLatestIdByFamily returns the image ID of the most recent available
// image within the given Alibaba ImageFamily in the given region.
//
// The family argument must be a non-empty Alibaba ImageFamily identifier
// (e.g. "acs:ubuntu_22_04_x64"). The caller is responsible for extracting
// the family from stored metadata (e.g. image_infos.details) before calling
// this function.
//
// Returns:
//   - imageId:      resolved latest image ID, empty if not found
//   - creationTime: ISO8601 creation time of the resolved image (as returned by ECS)
//   - err:          non-nil only on transport/credential/API errors; a legitimate
//                   "family has no available image" result is returned as
//                   (imageId="", creationTime="", err=nil) so the caller can
//                   fall back to the original image ID.
func ResolveLatestIdByFamily(ctx context.Context, region, family string) (imageId string, creationTime string, err error) {
	region = strings.TrimSpace(region)
	family = strings.TrimSpace(family)
	if region == "" {
		return "", "", fmt.Errorf("region is empty")
	}
	if family == "" {
		return "", "", fmt.Errorf("image family is empty")
	}

	key := familyCacheKey(region, family)

	// Fast path: TTL cache hit.
	if v, ok := familyCache.Load(key); ok {
		if e, ok := v.(*familyCacheEntry); ok && time.Now().Before(e.expiresAt) {
			return e.imageId, e.creationTime, nil
		}
	}

	// Slow path: dedupe concurrent misses via singleflight so that a burst of
	// VM creations triggers at most one CSP API call per (region, family).
	type result struct {
		id       string
		creation string
	}
	v, err, _ := familyGroup.Do(key, func() (interface{}, error) {
		// Re-check cache inside the singleflight to avoid a redundant API call
		// when a prior in-flight request just populated it.
		if v, ok := familyCache.Load(key); ok {
			if e, ok := v.(*familyCacheEntry); ok && time.Now().Before(e.expiresAt) {
				return result{id: e.imageId, creation: e.creationTime}, nil
			}
		}
		id, creation, apiErr := describeImageFromFamily(ctx, region, family)
		if apiErr != nil {
			return nil, apiErr
		}
		familyCache.Store(key, &familyCacheEntry{
			imageId:      id,
			creationTime: creation,
			expiresAt:    time.Now().Add(familyResolveTTL),
		})
		return result{id: id, creation: creation}, nil
	})
	if err != nil {
		return "", "", err
	}
	r := v.(result)
	return r.id, r.creation, nil
}

// describeImageFromFamily performs the actual Alibaba ECS API call without
// caching. Kept small so the caching logic above stays readable.
func describeImageFromFamily(ctx context.Context, region, family string) (imageId string, creationTime string, err error) {
	accessKeyID, accessKeySecret, err := getAlibabaCreds(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get Alibaba credentials: %w", err)
	}

	client, err := ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Alibaba ECS client for region %s: %w", region, err)
	}

	req := ecs.CreateDescribeImageFromFamilyRequest()
	req.Scheme = "https"
	req.RegionId = region
	req.ImageFamily = family

	start := time.Now()
	resp, err := client.DescribeImageFromFamily(req)
	elapsed := time.Since(start)
	if err != nil {
		return "", "", fmt.Errorf("DescribeImageFromFamily(region=%s, family=%s) failed after %s: %w",
			region, family, elapsed, err)
	}

	// Alibaba returns Image with empty ImageId when no image exists in the family.
	// Treat this as a soft miss rather than an error.
	img := resp.Image
	if strings.TrimSpace(img.ImageId) == "" {
		log.Debug().Msgf("alibaba: no image found in family=%s region=%s (elapsed=%s)", family, region, elapsed)
		return "", "", nil
	}

	log.Debug().Msgf("alibaba: resolved family=%s region=%s -> imageId=%s creation=%s (elapsed=%s)",
		family, region, img.ImageId, img.CreationTime, elapsed)
	return img.ImageId, img.CreationTime, nil
}
