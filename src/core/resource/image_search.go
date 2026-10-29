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
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func SearchImage(nsId string, req model.SearchImageRequest, isCustomImage bool) ([]model.ImageInfo, int, error) {
	err := common.CheckString(nsId)
	cnt := 0
	if err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return nil, cnt, err
	}

	var specInfo *model.SpecInfo
	// If MatchedSpecId is provided, fetch spec information and apply to search criteria
	if req.MatchedSpecId != "" {
		spec, err := GetSpec(model.SystemCommonNs, req.MatchedSpecId)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get spec information for MatchedSpecId: %s", req.MatchedSpecId)
			return nil, cnt, err
		}
		specInfo = &spec

		// Apply spec information to search criteria if not already specified
		if specInfo.ProviderName != "" {
			req.ProviderName = specInfo.ProviderName
			log.Debug().Msgf("Applied ProviderName from spec: %s", req.ProviderName)
		}
		if specInfo.RegionName != "" {
			req.RegionName = specInfo.RegionName
			log.Debug().Msgf("Applied RegionName from spec: %s", req.RegionName)
		}
		if specInfo.Architecture != "" && specInfo.Architecture != string(model.ArchitectureNA) {
			req.OSArchitecture = model.OSArchitecture(specInfo.Architecture)
			log.Debug().Msgf("Applied OSArchitecture from spec: %s", req.OSArchitecture)
		}

		log.Info().Msgf("SearchImage with MatchedSpecId %s: providerName=%s, regionName=%s, osArchitecture=%s",
			req.MatchedSpecId, req.ProviderName, req.RegionName, req.OSArchitecture)
	}

	var images []model.ImageInfo
	sqlQuery := model.ORM.Where("namespace = ?", nsId)

	// Apply isCustomImage filter first (highest priority)
	if isCustomImage {
		sqlQuery = sqlQuery.Where("resource_type = ?", model.StrCustomImage)
		log.Debug().Msg("Applied isCustomImage filter: resource_type = customImage")
	}

	if req.ProviderName != "" {
		sqlQuery = sqlQuery.Where("provider_name = ?", req.ProviderName)
	}

	// regionName needs to be searched from region_list
	if req.RegionName != "" {
		sqlQuery = sqlQuery.Where(
			model.ORM.Where("LOWER(region_list) LIKE ?", "%"+strings.ToLower(req.RegionName)+"%").
				Or("LOWER(region_list) LIKE ?", "%"+strings.ToLower(model.StrCommon)+"%"))
	}

	if req.OSType != "" {
		osTypeLower := strings.ToLower(req.OSType)
		osKeywords := strings.Fields(osTypeLower)

		if len(osKeywords) == 1 {
			keyword := osKeywords[0]
			sqlQuery = sqlQuery.Where(
				model.ORM.Where("LOWER(os_type) LIKE ?", "%"+keyword+"%").
					Or("REPLACE(LOWER(os_type), ' ', '') LIKE ?", "%"+keyword+"%"))
		} else {
			for _, keyword := range osKeywords {
				sqlQuery = sqlQuery.Where("LOWER(os_type) LIKE ?", "%"+keyword+"%")
			}

		}
	}

	if req.OSArchitecture != "" {
		// Include images with the requested architecture OR "na" / empty (not available/specified by the CSP)
		// Images with "na" or empty architecture are treated as compatible with any architecture
		sqlQuery = sqlQuery.Where(
			model.ORM.Where("LOWER(os_architecture) = ?", strings.ToLower(string(req.OSArchitecture))).
				Or("LOWER(os_architecture) = ?", strings.ToLower(string(model.ArchitectureNA))).
				Or("os_architecture = ''"))
	}

	if req.IsGPUImage != nil {
		sqlQuery = sqlQuery.Where("is_gpu_image = ?", *req.IsGPUImage)
	}

	if req.IsKubernetesImage != nil {
		sqlQuery = sqlQuery.Where("is_kubernetes_image = ?", *req.IsKubernetesImage)
	}

	if req.IsBasicGpuImage != nil {
		// isBasicGpuImage=true implies isGPUImage=true; reject contradictory filters
		if *req.IsBasicGpuImage && req.IsGPUImage != nil && !*req.IsGPUImage {
			return nil, cnt, fmt.Errorf("isBasicGpuImage=true is incompatible with isGPUImage=false")
		}
		sqlQuery = sqlQuery.Where("is_basic_gpu_image = ?", *req.IsBasicGpuImage)
	}

	// Check if isRegisteredByAsset is true
	// If it is true, filter by system_label = StrFromAssets
	if req.IsRegisteredByAsset != nil {
		if *req.IsRegisteredByAsset {
			sqlQuery = sqlQuery.Where("system_label = ?", model.StrFromAssets)
		}
	}

	// Check if includeDeprecated is nil or false
	if req.IncludeDeprecatedImage != nil {
		if !*req.IncludeDeprecatedImage {
			sqlQuery = sqlQuery.Where("image_status != ?", model.ImageDeprecated)
		}
	} else {
		sqlQuery = sqlQuery.Where("image_status != ?", model.ImageDeprecated)
	}

	// Deletion tombstones are not selectable
	sqlQuery = sqlQuery.Where("deletion_requested_at IS NULL OR deletion_requested_at = ''")

	if len(req.DetailSearchKeys) > 0 {
		// Build a single query to check if all keywords are included in either os_type or details
		for _, keyword := range req.DetailSearchKeys {
			keyword = strings.ToLower(keyword)
			sqlQuery = sqlQuery.Where("(LOWER(details) LIKE ?)", "%"+keyword+"%")
		}
	}

	log.Info().Msgf("SearchImage: matchedSpecId=%s, providerName=%s, regionName=%s, osType=%s, osArchitecture=%s, isGPUImage=%v, isKubernetesImage=%v, isRegisteredByAsset=%v, includeDeprecatedImage=%v",
		req.MatchedSpecId, req.ProviderName, req.RegionName, req.OSType, req.OSArchitecture, req.IsGPUImage, req.IsKubernetesImage, req.IsRegisteredByAsset, req.IncludeDeprecatedImage)

	result := sqlQuery.Find(&images)
	log.Info().Msgf("SearchImage: Found %d images for namespace %s", len(images), nsId)

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to retrieve images")
		return nil, cnt, result.Error
	}
	cnt = len(images)

	// Apply CSP-specific image filtering based on spec compatibility first,
	// before deduplicating by date/version, so spec-compatible images are never dropped prematurely.
	if specInfo != nil && len(images) > 0 {
		filteredImages := applyCspSpecificImageFiltering(images, *specInfo)
		if len(filteredImages) != len(images) {
			log.Info().Msgf("SearchImage: CSP-specific filtering removed %d images, %d images remaining for provider %s",
				len(images)-len(filteredImages), len(filteredImages), specInfo.ProviderName)
			images = filteredImages
			cnt = len(images)
		}
	}

	// Filter duplicate images with same OS details but different dates, keeping only the latest 2
	allowedDuplicationCount := 2

	if len(images) > 0 {
		filteredImages := filterDuplicateImagesByDate(images, allowedDuplicationCount)
		log.Info().Msgf("SearchImage: Filtered %d duplicate images, %d images remaining",
			len(images)-len(filteredImages), len(filteredImages))
		images = filteredImages
		cnt = len(images)
	}

	// Sort images by OSDistribution in natural descending order (so "Rocky Linux 8.10" > "Rocky Linux 8.8")
	sort.Slice(images, func(i, j int) bool {
		return compareNaturalDesc(images[i].OSDistribution, images[j].OSDistribution)
	})

	// Additional filtering: Keep only the top image for each group with same base distribution text
	if len(images) > 0 {
		finalImages := filterDuplicateImagesByVersion(images)
		log.Info().Msgf("SearchImage: Additional filtering removed %d images, %d images remaining",
			len(images)-len(finalImages), len(finalImages))
		images = finalImages
		cnt = len(images)
	}

	// Move basic images and basic GPU images to the front using partition approach (O(n))
	// Priority: isBasicImage=true first, then isBasicGpuImage=true, then the rest
	if len(images) > 0 {
		basicIndex := 0
		for i := 0; i < len(images); i++ {
			if images[i].IsBasicImage {
				if i != basicIndex {
					images[basicIndex], images[i] = images[i], images[basicIndex]
				}
				basicIndex++
			}
		}
		// Then move basic GPU images immediately after basic OS images
		gpuBasicIndex := basicIndex
		for i := basicIndex; i < len(images); i++ {
			if images[i].IsBasicGpuImage {
				if i != gpuBasicIndex {
					images[gpuBasicIndex], images[i] = images[i], images[gpuBasicIndex]
				}
				gpuBasicIndex++
			}
		}
	}

	// Apply IncludeBasicImageOnly filter (documented request field)
	if req.IncludeBasicImageOnly != nil && *req.IncludeBasicImageOnly {
		basicOnly := make([]model.ImageInfo, 0, len(images))
		for _, img := range images {
			if img.IsBasicImage {
				basicOnly = append(basicOnly, img)
			}
		}
		images = basicOnly
		cnt = len(images)
	}

	// Apply MaxResults limit (documented request field)
	if req.MaxResults != nil && *req.MaxResults > 0 && len(images) > *req.MaxResults {
		images = images[:*req.MaxResults]
		cnt = len(images)
	}

	return images, cnt, nil
}

// compareNaturalDesc returns true if string a is greater than string b using natural
// numeric comparison (e.g., "Rocky Linux 8.10" > "Rocky Linux 8.8").
func compareNaturalDesc(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		ca, cb := a[ia], b[ib]
		isDigitA := ca >= '0' && ca <= '9'
		isDigitB := cb >= '0' && cb <= '9'
		if isDigitA && isDigitB {
			ja := ia
			for ja < len(a) && a[ja] >= '0' && a[ja] <= '9' {
				ja++
			}
			jb := ib
			for jb < len(b) && b[jb] >= '0' && b[jb] <= '9' {
				jb++
			}
			numA := strings.TrimLeft(a[ia:ja], "0")
			numB := strings.TrimLeft(b[ib:jb], "0")
			if len(numA) != len(numB) {
				return len(numA) > len(numB)
			}
			if numA != numB {
				return numA > numB
			}
			ia, ib = ja, jb
			continue
		}
		if ca != cb {
			return ca > cb
		}
		ia++
		ib++
	}
	return len(a) > len(b)
}

// filterDuplicateImagesByDate filters duplicate images keeping only the latest 2 versions
// of images with same OSType, OSArchitecture, OSPlatform, and similar OSDistribution (excluding dates)
func filterDuplicateImagesByDate(images []model.ImageInfo, allowedDuplicationCount int) []model.ImageInfo {

	if allowedDuplicationCount < 1 {
		return images
	}

	type ImageGroup struct {
		Images []model.ImageInfo
		Key    string
	}

	// Group images by normalized key (excluding date patterns)
	imageGroups := make(map[string]*ImageGroup)

	for _, img := range images {
		// Create a normalized key excluding date patterns
		normalizedDistribution := normalizeDateInDistribution(img.OSDistribution)
		key := fmt.Sprintf("%s|%s|%s|%s",
			strings.ToLower(img.OSType),
			strings.ToLower(string(img.OSArchitecture)),
			strings.ToLower(string(img.OSPlatform)),
			normalizedDistribution)

		if group, exists := imageGroups[key]; exists {
			group.Images = append(group.Images, img)
		} else {
			imageGroups[key] = &ImageGroup{
				Images: []model.ImageInfo{img},
				Key:    key,
			}
		}
	}

	var result []model.ImageInfo

	for _, group := range imageGroups {
		if len(group.Images) <= allowedDuplicationCount {
			// If allowedDuplicationCount or fewer images, keep all
			result = append(result, group.Images...)
		} else {
			// Sort by date extracted from distribution string and keep latest allowedDuplicationCount
			sortedImages := sortImagesByDateInDistribution(group.Images)
			// Keep the latest allowedDuplicationCount images
			result = append(result, sortedImages[:allowedDuplicationCount]...)
		}
	}

	return result
}

// filterDuplicateImagesByVersion keeps only the latest image for each group with same base distribution text
// When OSDistribution is identical (e.g., Alibaba images), uses cspImageName date to determine the latest
func filterDuplicateImagesByVersion(images []model.ImageInfo) []model.ImageInfo {
	if len(images) == 0 {
		return images
	}

	// Group images by normalized key
	imageGroups := make(map[string][]model.ImageInfo)

	for _, img := range images {
		// Create grouping key based on OSType, OSArchitecture, OSPlatform, and base distribution text
		baseDistribution := removeNumbersFromDistribution(img.OSDistribution)
		key := fmt.Sprintf("%s|%s|%s|%s",
			strings.ToLower(img.OSType),
			strings.ToLower(string(img.OSArchitecture)),
			strings.ToLower(string(img.OSPlatform)),
			strings.ToLower(baseDistribution))

		imageGroups[key] = append(imageGroups[key], img)
	}

	var result []model.ImageInfo

	for _, group := range imageGroups {
		if len(group) == 1 {
			result = append(result, group[0])
		} else {
			// Multiple images with same key - select the latest one
			// Try to extract date from cspImageName first, then OSDistribution
			latestImg := group[0]
			latestDate := extractLatestDateFromString(latestImg.CspImageName)
			if latestDate.IsZero() {
				latestDate = extractLatestDateFromDistribution(latestImg.OSDistribution)
			}

			for i := 1; i < len(group); i++ {
				imgDate := extractLatestDateFromString(group[i].CspImageName)
				if imgDate.IsZero() {
					imgDate = extractLatestDateFromDistribution(group[i].OSDistribution)
				}

				if imgDate.After(latestDate) {
					latestImg = group[i]
					latestDate = imgDate
				} else if imgDate.Equal(latestDate) {
					if compareNaturalDesc(group[i].OSDistribution, latestImg.OSDistribution) ||
						(group[i].OSDistribution == latestImg.OSDistribution && compareNaturalDesc(group[i].CspImageName, latestImg.CspImageName)) {
						latestImg = group[i]
					}
				}
			}

			result = append(result, latestImg)
		}
	}

	return result
}

// removeNumbersFromDistribution removes all numbers from distribution string to get base text
func removeNumbersFromDistribution(distribution string) string {
	// Remove all numbers (including version numbers, dates, etc.)
	re := regexp.MustCompile(`\d+`)
	normalized := re.ReplaceAllString(distribution, "")

	// Clean up extra spaces, dashes, and dots
	normalized = regexp.MustCompile(`[.\-_\s]+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// normalizeDateInDistribution removes date patterns from distribution string for grouping
func normalizeDateInDistribution(distribution string) string {
	// Pattern 1: YYYYMMDD (e.g., 20250712, 20250508)
	re1 := regexp.MustCompile(`-?\d{8}`)
	normalized := re1.ReplaceAllString(distribution, "")

	// Pattern 2: YYYY-MM-DD or YYYY.MM.DD
	re2 := regexp.MustCompile(`-?\d{4}[-.]?\d{2}[-.]?\d{2}`)
	normalized = re2.ReplaceAllString(normalized, "")

	// Pattern 3: YYYYMMDDHHMM (e.g., 202506030226)
	re3 := regexp.MustCompile(`-?\d{12}`)
	normalized = re3.ReplaceAllString(normalized, "")

	// Pattern 4: ISO date format (e.g., 2025-06-03T02-30-35.058Z)
	re4 := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3}Z`)
	normalized = re4.ReplaceAllString(normalized, "")

	// Clean up extra dashes and spaces
	normalized = regexp.MustCompile(`--+`).ReplaceAllString(normalized, "-")
	normalized = regexp.MustCompile(`^-|-$`).ReplaceAllString(normalized, "")
	normalized = strings.TrimSpace(normalized)

	return strings.ToLower(normalized)
}

// sortImagesByDateInDistribution sorts images by dates found in distribution strings (newest first)
func sortImagesByDateInDistribution(images []model.ImageInfo) []model.ImageInfo {
	sort.Slice(images, func(i, j int) bool {
		dateI := extractLatestDateFromDistribution(images[i].OSDistribution)
		dateJ := extractLatestDateFromDistribution(images[j].OSDistribution)

		// If dates are equal, compare by creation date or natural distribution/name order
		if dateI.Equal(dateJ) {
			// Parse creation date if available
			if images[i].CreationDate != "" && images[j].CreationDate != "" {
				timeI, errI := time.Parse("2006-01-02T15:04:05.000Z", images[i].CreationDate)
				timeJ, errJ := time.Parse("2006-01-02T15:04:05.000Z", images[j].CreationDate)
				if errI == nil && errJ == nil && !timeI.Equal(timeJ) {
					return timeI.After(timeJ)
				}
			}
			if images[i].OSDistribution != images[j].OSDistribution {
				return compareNaturalDesc(images[i].OSDistribution, images[j].OSDistribution)
			}
			return compareNaturalDesc(images[i].Name, images[j].Name)
		}

		return dateI.After(dateJ) // Newest first
	})

	return images
}

// extractLatestDateFromDistribution extracts the latest date from distribution string
func extractLatestDateFromDistribution(distribution string) time.Time {
	var latestDate time.Time

	// Pattern 1: YYYYMMDD (e.g., 20250712)
	re1 := regexp.MustCompile(`\d{8}`)
	matches1 := re1.FindAllString(distribution, -1)
	for _, match := range matches1 {
		if date, err := time.Parse("20060102", match); err == nil {
			if date.After(latestDate) {
				latestDate = date
			}
		}
	}

	// Pattern 2: YYYY-MM-DD or YYYY.MM.DD
	re2 := regexp.MustCompile(`\d{4}[-.]?\d{2}[-.]?\d{2}`)
	matches2 := re2.FindAllString(distribution, -1)
	for _, match := range matches2 {
		// Try different date formats
		formats := []string{"2006-01-02", "2006.01.02", "20060102"}
		for _, format := range formats {
			if date, err := time.Parse(format, match); err == nil {
				if date.After(latestDate) {
					latestDate = date
				}
				break
			}
		}
	}

	// Pattern 3: YYYYMMDDHHMM (e.g., 202506030226)
	re3 := regexp.MustCompile(`\d{12}`)
	matches3 := re3.FindAllString(distribution, -1)
	for _, match := range matches3 {
		if date, err := time.Parse("200601021504", match); err == nil {
			if date.After(latestDate) {
				latestDate = date
			}
		}
	}

	// Pattern 4: ISO date format (e.g., 2025-06-03T02-30-35.058Z)
	re4 := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3}Z`)
	matches4 := re4.FindAllString(distribution, -1)
	for _, match := range matches4 {
		if date, err := time.Parse("2006-01-02T15-04-05.000Z", match); err == nil {
			if date.After(latestDate) {
				latestDate = date
			}
		}
	}

	return latestDate
}

// extractLatestDateFromString extracts the latest date from any string (e.g., cspImageName)
// This is useful for cases like Alibaba where dates are in the image name (e.g., ubuntu_22_04_x64_20G_alibase_20251126.vhd)
func extractLatestDateFromString(str string) time.Time {
	return extractLatestDateFromDistribution(str)
}

// SearchImageOptions returns the available options for searching images
func SearchImageOptions() (model.SearchImageRequestOptions, error) {
	var options model.SearchImageRequestOptions

	// Get sample MatchedSpecId options (diverse CSP examples for better representation)
	var sampleSpecs []string

	// Get specs grouped by provider to ensure diversity
	var specsByProvider []struct {
		ProviderName string `json:"provider_name"`
		Id           string `json:"id"`
	}

	if err := model.ORM.Model(&model.SpecInfo{}).
		Select("provider_name, id").
		Where("namespace = ?", model.SystemCommonNs).
		Order("provider_name, id").
		Find(&specsByProvider).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to get spec IDs by provider, using default examples")
		// Fallback to default examples if query fails
		options.MatchedSpecId = []string{
			"aws+ap-northeast-2+t2.small",
			"azure+koreacentral+Standard_B1s",
			"gcp+asia-northeast3+e2-micro",
			"ncp+kr+m8-g3a",
		}
	} else {
		// Group specs by provider and take 1-2 examples from each
		providerSpecs := make(map[string][]string)
		for _, spec := range specsByProvider {
			providerSpecs[spec.ProviderName] = append(providerSpecs[spec.ProviderName], spec.Id)
		}

		// Collect diverse examples (max 2 per provider, total max 20)
		maxPerProvider := 2
		totalLimit := 20
		for _, specs := range providerSpecs {
			taken := 0
			for _, specId := range specs {
				if taken < maxPerProvider && len(sampleSpecs) < totalLimit {
					sampleSpecs = append(sampleSpecs, specId)
					taken++
				}
			}
			if len(sampleSpecs) >= totalLimit {
				break
			}
		}

		// If no specs found in DB, use fallback examples
		if len(sampleSpecs) == 0 {
			sampleSpecs = []string{
				"aws+ap-northeast-2+t2.small",
				"azure+koreacentral+Standard_B1s",
				"gcp+asia-northeast3+e2-micro",
				"ncp+kr+m8-g3a",
			}
		}

		options.MatchedSpecId = sampleSpecs
	}

	// Get distinct provider names
	if err := model.ORM.Model(&model.ImageInfo{}).
		Distinct("provider_name").
		Order("provider_name").
		Pluck("provider_name", &options.ProviderName).Error; err != nil {
		log.Error().Err(err).Msg("Failed to get distinct provider names")
		return options, err
	}

	// Get regions (application-level processing)
	var images []model.ImageInfo
	if err := model.ORM.Model(&model.ImageInfo{}).
		Select("region_list").
		Find(&images).Error; err != nil {
		log.Error().Err(err).Msg("Failed to get region lists")
		return options, err
	}

	// Use a map for deduplication
	regionMap := make(map[string]struct{})
	for _, img := range images {
		for _, region := range img.RegionList {
			regionMap[region] = struct{}{}
		}
	}

	// Convert map to sorted slice
	options.RegionName = make([]string, 0, len(regionMap))
	for region := range regionMap {
		options.RegionName = append(options.RegionName, region)
	}
	sort.Strings(options.RegionName)

	// Get distinct OS types (non-empty only)
	if err := model.ORM.Model(&model.ImageInfo{}).
		Where("os_type != ''").
		Distinct("os_type").
		Order("os_type").
		Pluck("os_type", &options.OSType).Error; err != nil {
		log.Error().Err(err).Msg("Failed to get distinct OS types")
		return options, err
	}

	// Get distinct OS architectures (non-empty only)
	if err := model.ORM.Model(&model.ImageInfo{}).
		Where("os_architecture != ''").
		Distinct("os_architecture").
		Order("os_architecture").
		Pluck("os_architecture", &options.OSArchitecture).Error; err != nil {
		log.Error().Err(err).Msg("Failed to get distinct OS architectures")
		return options, err
	}

	// Set boolean options
	options.IsGPUImage = []bool{true, false}
	options.IsKubernetesImage = []bool{true, false}
	options.IsRegisteredByAsset = []bool{true, false}
	options.IncludeDeprecatedImage = []bool{true, false}

	// Set DetailSearchKeys example
	options.DetailSearchKeys = [][]string{
		{"This is just an example", "omit this option if not needed", "requires more time to search"},
		{"sql", "2022"},
		{"tensorflow", "2.17"},
	}

	return options, nil
}

// UpdateImage accepts to-be TB image objects,
// updates and returns the updated TB image objects

func applyCspSpecificImageFiltering(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	var filtered []model.ImageInfo
	switch csp.ResolveCloudPlatform(specInfo.ProviderName) {
	case csp.NCP:
		filtered = filterImagesByCorrespondingIds(images, specInfo)
	case csp.Azure:
		filtered = filterImagesForAzure(images, specInfo)
	case csp.NHN:
		filtered = filterImagesForNHN(images, specInfo)
	case csp.AWS:
		filtered = filterImagesForAWS(images, specInfo)
	case csp.Alibaba:
		filtered = filterImagesForAlibaba(images, specInfo)
	case csp.GCP:
		filtered = filterImagesForGCP(images, specInfo)
	case csp.IBM:
		filtered = filterImagesForIBM(images, specInfo)
	default:
		filtered = images
	}
	return filterImagesByFixedRootDisk(filtered, specInfo)
}

// CheckSpecImageCompatibility checks whether a specific SpecInfo and ImageInfo pair are compatible
// (architecture, hypervisor, boot mode, disk controller, virtualization type, root disk size, etc.).
// Returns nil if compatible, or a descriptive error explaining why VM creation would fail.
func CheckSpecImageCompatibility(specInfo model.SpecInfo, imageInfo model.ImageInfo) error {
	// Skip strict metadata checks for custom user snapshots/MyImages that may lack full CSP catalog Details
	if imageInfo.ResourceType == model.StrCustomImage {
		return nil
	}

	// 1. OS Architecture check
	specArch := strings.TrimSpace(specInfo.Architecture)
	imgArch := strings.TrimSpace(string(imageInfo.OSArchitecture))
	if specArch != "" && specArch != string(model.ArchitectureUnknown) && specArch != string(model.ArchitectureNA) &&
		imgArch != "" && imgArch != string(model.ArchitectureUnknown) && imgArch != string(model.ArchitectureNA) {
		if !strings.EqualFold(specArch, imgArch) {
			return fmt.Errorf("architecture mismatch: spec '%s' is '%s' but image '%s' is '%s'",
				specInfo.CspSpecName, specArch, imageInfo.CspImageName, imgArch)
		}
	}

	// 2. Fixed local root disk size check (for CSPs where SpecInfo.RootDiskSize is a fixed OS disk limit)
	if hasFixedOSRootDisk(specInfo) && imageInfo.OSDiskSizeGB > 0 && imageInfo.OSDiskSizeGB > float64(specInfo.RootDiskSize) {
		return fmt.Errorf("root disk size mismatch: spec '%s' has fixed %d GB root disk, which is smaller than image '%s' minimum disk size (%.0f GB)",
			specInfo.CspSpecName, specInfo.RootDiskSize, imageInfo.CspImageName, imageInfo.OSDiskSizeGB)
	}

	// 3. CSP-specific hypervisor / boot mode / controller / flavor compatibility checks
	switch csp.ResolveCloudPlatform(specInfo.ProviderName) {
	case csp.NCP:
		specHV := extractNCPHypervisorCode(specInfo.Details, specInfo.CspSpecName)
		imgHV := extractNCPHypervisorCode(imageInfo.Details, imageInfo.OSDistribution)
		if specHV != "" && imgHV != "" && !strings.EqualFold(specHV, imgHV) {
			return fmt.Errorf("hypervisor mismatch: NCP spec '%s' requires '%s' hypervisor, but image '%s' (%s) is for '%s' hypervisor",
				specInfo.CspSpecName, specHV, imageInfo.CspImageName, imageInfo.OSDistribution, imgHV)
		}
		correspondingIds := extractCorrespondingImageIds(specInfo.Details)
		if len(correspondingIds) > 0 && imageInfo.CspImageName != "" {
			matched := false
			for _, id := range correspondingIds {
				if strings.EqualFold(strings.TrimSpace(id), strings.TrimSpace(imageInfo.CspImageName)) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("NCP spec '%s' does not list image '%s' in its CorrespondingImageIds",
					specInfo.CspSpecName, imageInfo.CspImageName)
			}
		}

	case csp.Azure:
		if !isAzureImageCompatible(imageInfo, specInfo) {
			return fmt.Errorf("Azure spec '%s' (HyperVGenerations=%q, DiskControllerTypes=%q, ConfidentialComputingType=%q) is incompatible with image '%s'",
				specInfo.CspSpecName,
				common.LookupKeyValueList(specInfo.Details, "HyperVGenerations"),
				common.LookupKeyValueList(specInfo.Details, "DiskControllerTypes"),
				common.LookupKeyValueList(specInfo.Details, "ConfidentialComputingType"),
				imageInfo.CspImageName)
		}

	case csp.AWS:
		if !isAWSImageCompatible(imageInfo, specInfo) {
			return fmt.Errorf("AWS spec '%s' (BootModes=%q, VirtTypes=%q, Hypervisor=%q) is incompatible with image '%s' (BootMode=%q, VirtType=%q, EnaSupport=%q)",
				specInfo.CspSpecName,
				common.LookupKeyValueList(specInfo.Details, "SupportedBootModes"),
				common.LookupKeyValueList(specInfo.Details, "SupportedVirtualizationTypes"),
				common.LookupKeyValueList(specInfo.Details, "Hypervisor"),
				imageInfo.CspImageName,
				common.LookupKeyValueList(imageInfo.Details, "BootMode"),
				common.LookupKeyValueList(imageInfo.Details, "VirtualizationType"),
				common.LookupKeyValueList(imageInfo.Details, "EnaSupport"))
		}

	case csp.Alibaba:
		if !isAlibabaImageCompatible(imageInfo, specInfo) {
			return fmt.Errorf("Alibaba spec '%s' is incompatible with image '%s' (BootMode=%q, Features=%q)",
				specInfo.CspSpecName, imageInfo.CspImageName,
				common.LookupKeyValueList(imageInfo.Details, "BootMode"),
				common.LookupKeyValueList(imageInfo.Details, "Features"))
		}

	case csp.GCP:
		if !isGCPImageCompatible(imageInfo, specInfo) {
			return fmt.Errorf("GCP spec '%s' is incompatible with image '%s' (GuestOsFeatures=%q)",
				specInfo.CspSpecName, imageInfo.CspImageName,
				common.LookupKeyValueList(imageInfo.Details, "GuestOsFeatures"))
		}

	case csp.IBM:
		if !isIBMImageCompatible(imageInfo) {
			return fmt.Errorf("IBM image '%s' is restricted (bare-metal-only or dedicated-host-only) and cannot be used with VSI profile '%s'",
				imageInfo.CspImageName, specInfo.CspSpecName)
		}

	case csp.NHN:
		if !isNHNImageCompatible(imageInfo, specInfo) {
			return fmt.Errorf("NHN spec '%s' is incompatible with image '%s' based on NHN image properties",
				specInfo.CspSpecName, imageInfo.CspImageName)
		}
	}

	return nil
}

var reNHNMinCPU = regexp.MustCompile(`min_cpu:(\d+)`)

func isNHNImageCompatible(img model.ImageInfo, specInfo model.SpecInfo) bool {
	props := strings.ToLower(common.LookupKeyValueList(img.Details, "Properties"))
	if props == "" {
		return true
	}
	isGPUSpec := specInfo.AcceleratorCount > 0
	isLocalBootSpec := specInfo.RootDiskSize > 0

	if strings.Contains(props, "nhncloud_allow_compute_flavor:false") &&
		strings.Contains(props, "nhncloud_allow_gpu_flavor:false") {
		return false
	}
	if isGPUSpec {
		if strings.Contains(props, "nhncloud_allow_gpu_flavor:false") {
			return false
		}
	} else {
		if strings.Contains(props, "nhncloud_allow_cpu_flavor:false") {
			return false
		}
	}
	if isLocalBootSpec && strings.Contains(props, "nhncloud_allow_local_bootdisk_flavor:false") {
		return false
	}
	if specInfo.VCPU > 0 {
		if m := reNHNMinCPU.FindStringSubmatch(props); len(m) == 2 {
			if minCPU, err := strconv.Atoi(m[1]); err == nil && minCPU > int(specInfo.VCPU) {
				return false
			}
		}
	}
	return true
}

// filterImagesForNHN filters NHN Cloud images using the explicit compatibility flags in Properties:
//   - nhncloud_allow_compute_flavor / nhncloud_allow_cpu_flavor / nhncloud_allow_gpu_flavor
//   - nhncloud_allow_local_bootdisk_flavor (when specInfo.RootDiskSize > 0, e.g., u2.* flavors)
//   - min_cpu:<N> vs specInfo.VCPU
func filterImagesForNHN(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	var filtered []model.ImageInfo
	for _, img := range images {
		if isNHNImageCompatible(img, specInfo) {
			filtered = append(filtered, img)
		}
	}
	if len(filtered) == 0 {
		return images
	}
	return filtered
}

func isAWSImageCompatible(img model.ImageInfo, specInfo model.SpecInfo) bool {
	if len(img.Details) == 0 {
		return true
	}
	specBootModes := strings.ToLower(common.LookupKeyValueList(specInfo.Details, "SupportedBootModes"))
	specVirtTypes := strings.ToLower(common.LookupKeyValueList(specInfo.Details, "SupportedVirtualizationTypes"))
	specRootDevs := strings.ToLower(common.LookupKeyValueList(specInfo.Details, "SupportedRootDeviceTypes"))
	specNetInfo := strings.ToLower(common.LookupKeyValueList(specInfo.Details, "NetworkInfo"))
	specHypervisor := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(specInfo.Details, "Hypervisor")))

	supportsUEFI := strings.Contains(specBootModes, "uefi")
	supportsBIOS := strings.Contains(specBootModes, "legacy-bios")
	exclusiveBootMode := specBootModes != "" && (supportsUEFI != supportsBIOS)
	enaRequired := strings.Contains(specNetInfo, "enasupport:required") || specHypervisor == "nitro"

	if specVirtTypes != "" {
		imgVirt := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(img.Details, "VirtualizationType")))
		if imgVirt != "" && !strings.Contains(specVirtTypes, imgVirt) {
			return false
		}
	}
	if specRootDevs != "" {
		imgRoot := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(img.Details, "RootDeviceType")))
		if imgRoot != "" && !strings.Contains(specRootDevs, imgRoot) {
			return false
		}
	}
	if enaRequired {
		imgEna := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(img.Details, "EnaSupport")))
		if imgEna == "false" || (imgEna != "true" && common.LookupKeyValueList(img.Details, "VirtualizationType") != "") {
			return false
		}
	}
	if exclusiveBootMode {
		imgBoot := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(img.Details, "BootMode")))
		if supportsUEFI && !supportsBIOS {
			if imgBoot == "legacy-bios" || (imgBoot == "" && img.OSArchitecture == model.X86_64) {
				return false
			}
		} else if supportsBIOS && !supportsUEFI {
			if imgBoot == "uefi" {
				return false
			}
		}
	}
	return true
}

// filterImagesForAWS filters AWS AMIs by SupportedBootModes, SupportedVirtualizationTypes, SupportedRootDeviceTypes, and EnaSupport.
func filterImagesForAWS(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	var filtered []model.ImageInfo
	for _, img := range images {
		if isAWSImageCompatible(img, specInfo) {
			filtered = append(filtered, img)
		}
	}
	if len(filtered) == 0 {
		return images
	}
	return filtered
}

func isAlibabaLegacyBIOSOnlyFamily(family string) bool {
	f := strings.ToLower(strings.TrimSpace(family))
	switch {
	case strings.HasPrefix(f, "g5"), strings.HasPrefix(f, "c5"), strings.HasPrefix(f, "r5"),
		strings.HasPrefix(f, "hfc5"), strings.HasPrefix(f, "hfg5"), strings.HasPrefix(f, "ic5"),
		strings.HasPrefix(f, "t5"), strings.HasPrefix(f, "s6"), strings.HasPrefix(f, "n4"),
		strings.HasPrefix(f, "mn4"), strings.HasPrefix(f, "e4"), strings.HasPrefix(f, "c4"),
		strings.HasPrefix(f, "ce4"), strings.HasPrefix(f, "cm4"), strings.HasPrefix(f, "sn1"),
		strings.HasPrefix(f, "sn2"), strings.HasPrefix(f, "se1"), strings.HasPrefix(f, "d1"),
		strings.HasPrefix(f, "i1"), strings.HasPrefix(f, "i2"), strings.HasPrefix(f, "gn5"):
		return true
	default:
		return false
	}
}

func isAlibabaNVMeRequiredFamily(family string) bool {
	f := strings.ToLower(strings.TrimSpace(family))
	switch {
	case strings.HasPrefix(f, "g7se"), strings.HasPrefix(f, "c7se"), strings.HasPrefix(f, "r7se"),
		strings.HasPrefix(f, "g8"), strings.HasPrefix(f, "c8"), strings.HasPrefix(f, "r8"),
		strings.HasPrefix(f, "hfc8"), strings.HasPrefix(f, "hfg8"), strings.HasPrefix(f, "hfr8"),
		strings.HasPrefix(f, "hpc8"), strings.HasPrefix(f, "re8"),
		strings.HasPrefix(f, "g9"), strings.HasPrefix(f, "c9"), strings.HasPrefix(f, "r9"),
		strings.HasPrefix(f, "hfc9"), strings.HasPrefix(f, "hfg9"), strings.HasPrefix(f, "hfr9"),
		strings.HasPrefix(f, "ebm"), strings.HasPrefix(f, "i4"), strings.HasPrefix(f, "i5"),
		strings.HasPrefix(f, "d3s"), strings.HasPrefix(f, "gn8"), strings.HasPrefix(f, "gn9"),
		strings.HasPrefix(f, "sgn8"), strings.HasPrefix(f, "u2"):
		return true
	default:
		return false
	}
}

func isAlibabaImageCompatible(img model.ImageInfo, specInfo model.SpecInfo) bool {
	if len(img.Details) == 0 {
		return true
	}
	parts := strings.Split(specInfo.CspSpecName, ".")
	family := ""
	if len(parts) >= 2 {
		family = parts[1]
	}

	isARM := strings.EqualFold(specInfo.Architecture, string(model.ARM64))
	bootMode := strings.ToUpper(strings.TrimSpace(common.LookupKeyValueList(img.Details, "BootMode")))
	features := strings.ToLower(common.LookupKeyValueList(img.Details, "Features"))
	nvmeSupport := strings.ToLower(strings.TrimSpace(common.LookupKeyValueList(img.Details, "NvmeSupport")))

	if isARM && bootMode == "BIOS" {
		return false
	}
	if !isARM && isAlibabaLegacyBIOSOnlyFamily(family) && bootMode == "UEFI" {
		return false
	}
	if isAlibabaNVMeRequiredFamily(family) && (strings.Contains(features, "nvmesupport:unsupported") || nvmeSupport == "unsupported") {
		return false
	}
	return true
}

// filterImagesForAlibaba filters Alibaba Cloud images by BootMode (BIOS vs UEFI vs UEFI-Preferred)
// and NvmeSupport (for Gen 7se/8/9, bare-metal, and local-NVMe instance families).
func filterImagesForAlibaba(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	var filtered []model.ImageInfo
	for _, img := range images {
		if isAlibabaImageCompatible(img, specInfo) {
			filtered = append(filtered, img)
		}
	}
	if len(filtered) == 0 {
		return images
	}
	return filtered
}

func isGCPImageCompatible(img model.ImageInfo, specInfo model.SpecInfo) bool {
	guestFeatures := strings.ToUpper(common.LookupKeyValueList(img.Details, "GuestOsFeatures"))
	if guestFeatures == "" {
		return true
	}
	series := strings.ToLower(strings.SplitN(specInfo.CspSpecName, "-", 2)[0])
	switch series {
	case "n1", "n2", "n2d", "e2", "c2", "c2d", "t2d", "m1", "m2", "a2":
		if !strings.Contains(guestFeatures, "VIRTIO_SCSI_MULTIQUEUE") {
			return false
		}
	case "c4", "c4a", "c4d", "n4", "n4a", "n4d", "a4", "a4x", "x4", "m4":
		if !strings.Contains(guestFeatures, "IDPF") {
			return false
		}
	}
	return true
}

// filterImagesForGCP filters GCP images by GuestOsFeatures (VIRTIO_SCSI_MULTIQUEUE for Gen 1/2 SCSI series,
// and IDPF for Gen 4 / Blackwell / Titanium series).
func filterImagesForGCP(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	var filtered []model.ImageInfo
	for _, img := range images {
		if isGCPImageCompatible(img, specInfo) {
			filtered = append(filtered, img)
		}
	}
	if len(filtered) == 0 {
		return images
	}
	return filtered
}

func isIBMImageCompatible(img model.ImageInfo) bool {
	allowedUse := strings.ToLower(common.LookupKeyValueList(img.Details, "AllowedUse"))
	if strings.Contains(allowedUse, "instance:false") {
		return false
	}
	osInfo := strings.ToLower(common.LookupKeyValueList(img.Details, "OperatingSystem"))
	if strings.Contains(osInfo, "dedicated_host_only:true") {
		return false
	}
	return true
}

// filterImagesForIBM excludes IBM Cloud images restricted to bare metal servers (instance:false) or dedicated hosts.
func filterImagesForIBM(images []model.ImageInfo, _ model.SpecInfo) []model.ImageInfo {
	var filtered []model.ImageInfo
	for _, img := range images {
		if isIBMImageCompatible(img) {
			filtered = append(filtered, img)
		}
	}
	if len(filtered) == 0 {
		return images
	}
	return filtered
}

func isAzureImageCompatible(img model.ImageInfo, specInfo model.SpecInfo) bool {
	specGen := strings.TrimSpace(common.LookupKeyValueList(specInfo.Details, "HyperVGenerations"))
	supportsV1 := strings.Contains(specGen, "V1")
	supportsV2 := strings.Contains(specGen, "V2")
	if specGen != "" && (supportsV1 != supportsV2) {
		requiredGen := "V2"
		if supportsV1 {
			requiredGen = "V1"
		}
		imgGen := strings.TrimSpace(common.LookupKeyValueList(img.Details, "HyperVGeneration"))
		if imgGen != "" && imgGen != requiredGen {
			return false
		}
	}

	specDiskCtrl := strings.ToUpper(strings.TrimSpace(common.LookupKeyValueList(specInfo.Details, "DiskControllerTypes")))
	specConfType := strings.TrimSpace(common.LookupKeyValueList(specInfo.Details, "ConfidentialComputingType"))
	supportsNVMe := strings.Contains(specDiskCtrl, "NVME")
	supportsSCSI := strings.Contains(specDiskCtrl, "SCSI")
	nvmeOnly := specDiskCtrl != "" && supportsNVMe && !supportsSCSI
	scsiOnly := specDiskCtrl != "" && supportsSCSI && !supportsNVMe

	features := strings.ToUpper(common.LookupKeyValueList(img.Details, "Features"))
	if features != "" {
		if specConfType != "" && !strings.Contains(features, "CONFIDENTIALVMSUPPORTED") {
			return false
		}
		if strings.Contains(features, "DISKCONTROLLERTYPES=") {
			idx := strings.Index(features, "DISKCONTROLLERTYPES=")
			sub := features[idx+len("DISKCONTROLLERTYPES="):]
			if eqIdx := strings.Index(sub, "="); eqIdx != -1 {
				sub = sub[:eqIdx]
			}
			if nvmeOnly && !strings.Contains(sub, "NVME") {
				return false
			}
			if scsiOnly && !strings.Contains(sub, "SCSI") {
				return false
			}
		} else if nvmeOnly {
			return false
		}
	}
	return true
}

// filterImagesForAzure filters Azure images by HyperVGeneration, DiskControllerTypes (NVMe vs SCSI),
// and ConfidentialComputingType (SNP/TDX).
func filterImagesForAzure(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	filtered := filterImagesByHyperVGeneration(images, specInfo)

	specDiskCtrl := strings.ToUpper(strings.TrimSpace(common.LookupKeyValueList(specInfo.Details, "DiskControllerTypes")))
	specConfType := strings.TrimSpace(common.LookupKeyValueList(specInfo.Details, "ConfidentialComputingType"))

	supportsNVMe := strings.Contains(specDiskCtrl, "NVME")
	supportsSCSI := strings.Contains(specDiskCtrl, "SCSI")
	nvmeOnly := specDiskCtrl != "" && supportsNVMe && !supportsSCSI
	scsiOnly := specDiskCtrl != "" && supportsSCSI && !supportsNVMe

	if !nvmeOnly && !scsiOnly && specConfType == "" {
		return filtered
	}

	var ctrlFiltered []model.ImageInfo
	for _, img := range filtered {
		if isAzureImageCompatible(img, specInfo) {
			ctrlFiltered = append(ctrlFiltered, img)
		}
	}

	if len(ctrlFiltered) == 0 {
		return filtered
	}
	return ctrlFiltered
}

func hasFixedOSRootDisk(specInfo model.SpecInfo) bool {
	if specInfo.RootDiskSize <= 0 {
		return false
	}
	switch csp.ResolveCloudPlatform(specInfo.ProviderName) {
	case csp.NHN, csp.KT, csp.OpenStack, csp.NCP:
		return true
	default:
		return false
	}
}

// filterImagesByFixedRootDisk excludes images whose minimum OS disk size exceeds a spec's fixed local root disk size.
// Note: Only applied to CSPs where SpecInfo.RootDiskSize represents a fixed OS root disk (NHN, KT, OpenStack, NCP),
// not Azure/Alibaba where Spider's DiskSizeGB reflects temporary scratch or local NVMe data disks.
func filterImagesByFixedRootDisk(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	if !hasFixedOSRootDisk(specInfo) {
		return images
	}
	maxDiskGB := float64(specInfo.RootDiskSize)
	var filtered []model.ImageInfo
	for _, img := range images {
		if img.OSDiskSizeGB > 0 && img.OSDiskSizeGB > maxDiskGB {
			continue
		}
		filtered = append(filtered, img)
	}
	if len(filtered) == 0 {
		return images
	}
	return filtered
}

// filterImagesByHyperVGeneration filters Azure images to match the Hyper-V generation
// required by the spec.
//
// Azure VM sizes declare which Hyper-V generations they support via the HyperVGenerations
// field in their spec details:
//
//   - "V1"     — VM can only boot Generation 1 images (e.g., Standard_A1_v2)
//   - "V2"     — VM can only boot Generation 2 images (e.g., Standard_NV4ads_V710_v5)
//   - "V1,V2"  — VM supports both (majority of current Azure VM sizes)
//   - absent   — unknown; no filtering applied (safe fallback)
//
// When the spec is generation-exclusive (V1-only or V2-only), images of the wrong
// generation are excluded. If no images of the required generation are found, all
// images are returned as a fallback so provisioning is not hard-blocked.
func filterImagesByHyperVGeneration(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	// Read HyperVGenerations from spec details (e.g., "V2" or "V1,V2")
	var specGen string
	for _, kv := range specInfo.Details {
		if kv.Key == "HyperVGenerations" {
			specGen = strings.TrimSpace(kv.Value)
			break
		}
	}

	// If spec supports both generations or the field is absent, no restriction needed.
	supportsV1 := strings.Contains(specGen, "V1")
	supportsV2 := strings.Contains(specGen, "V2")
	if specGen == "" || (supportsV1 && supportsV2) {
		return images
	}

	// Determine required generation for exclusive specs.
	var requiredGen string
	switch {
	case supportsV1 && !supportsV2:
		requiredGen = "V1"
	case supportsV2 && !supportsV1:
		requiredGen = "V2"
	default:
		return images
	}

	// Keep images whose HyperVGeneration matches the required generation, or whose
	// generation is unspecified (treated as compatible — older catalog entries may
	// lack this metadata).
	var filtered []model.ImageInfo
	for _, img := range images {
		imgGen := ""
		for _, kv := range img.Details {
			if kv.Key == "HyperVGeneration" {
				imgGen = strings.TrimSpace(kv.Value)
				break
			}
		}
		if imgGen == "" || imgGen == requiredGen {
			filtered = append(filtered, img)
		}
	}

	if len(filtered) == 0 {
		log.Warn().Msgf(
			"Azure spec %s requires HyperVGeneration %s but no matching images found; using all images as fallback",
			specInfo.Id, requiredGen,
		)
		return images
	}

	log.Info().Msgf(
		"Azure HyperVGeneration filter: spec=%s requires %s-only, filtered from %d to %d images",
		specInfo.Id, requiredGen, len(images), len(filtered),
	)
	return filtered
}

func extractNCPHypervisorCode(details []model.KeyValue, fallbackStr string) string {
	hvRaw := strings.ToUpper(common.LookupKeyValueList(details, "HypervisorType"))
	switch {
	case strings.Contains(hvRaw, "KVM"):
		return "KVM"
	case strings.Contains(hvRaw, "XEN"):
		return "XEN"
	case strings.Contains(hvRaw, "BM"):
		return "BM"
	}
	upper := strings.ToUpper(fallbackStr)
	switch {
	case strings.Contains(upper, "HYPERVISOR:KVM") || strings.Contains(upper, "-G3"):
		return "KVM"
	case strings.Contains(upper, "HYPERVISOR:XEN") || strings.Contains(upper, "-G2") || strings.Contains(upper, "-G1"):
		return "XEN"
	case strings.Contains(upper, "HYPERVISOR:BM"):
		return "BM"
	}
	return ""
}

// filterImagesByCorrespondingIds filters NCP images based on HypervisorType (KVM vs XEN vs BM)
// and CorrespondingImageIds from spec details.
func filterImagesByCorrespondingIds(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	specHV := extractNCPHypervisorCode(specInfo.Details, specInfo.CspSpecName)
	correspondingIds := extractCorrespondingImageIds(specInfo.Details)

	validImageIds := make(map[string]bool, len(correspondingIds))
	for _, id := range correspondingIds {
		validImageIds[strings.TrimSpace(id)] = true
	}

	var filteredImages []model.ImageInfo
	for _, image := range images {
		if specHV != "" {
			imgHV := extractNCPHypervisorCode(image.Details, image.OSDistribution)
			if imgHV != "" && !strings.EqualFold(specHV, imgHV) {
				continue
			}
		}
		if len(validImageIds) > 0 && !validImageIds[image.CspImageName] {
			continue
		}
		filteredImages = append(filteredImages, image)
	}

	if len(filteredImages) == 0 && len(correspondingIds) == 0 {
		return images
	}

	log.Info().Msgf("NCP CorrespondingIds/Hypervisor(%s) filtering: %d corresponding IDs, filtered from %d to %d images for provider %s",
		specHV, len(correspondingIds), len(images), len(filteredImages), specInfo.ProviderName)

	return filteredImages
}

// extractCorrespondingImageIds extracts and parses CorrespondingImageIds from spec details
func extractCorrespondingImageIds(details []model.KeyValue) []string {
	for _, detail := range details {
		if detail.Key == "CorrespondingImageIds" {
			// Split comma-separated values and trim whitespace
			ids := strings.Split(detail.Value, ",")
			var cleanIds []string
			for _, id := range ids {
				if trimmed := strings.TrimSpace(id); trimmed != "" {
					cleanIds = append(cleanIds, trimmed)
				}
			}
			return cleanIds
		}
	}
	return nil
}

var (
	imageIgnoreConfig     *model.CloudImageIgnoreConfig
	imageIgnoreConfigOnce sync.Once
	imageIgnoreConfigErr  error
)

// loadCloudImageIgnoreConfig loads cloudimage_ignore.yaml (once, then cached).
func loadCloudImageIgnoreConfig() (*model.CloudImageIgnoreConfig, error) {
	imageIgnoreConfigOnce.Do(func() {
		ignoreViper := viper.New()
		common.SetupViperPaths(ignoreViper)
		ignoreViper.SetConfigName("cloudimage_ignore")
		ignoreViper.SetConfigType("yaml")

		if err := ignoreViper.ReadInConfig(); err != nil {
			log.Warn().Err(err).Msg("Could not load cloudimage_ignore.yaml, no image filtering will be applied")
			imageIgnoreConfigErr = err
			return
		}

		log.Debug().Str("path", ignoreViper.ConfigFileUsed()).Msg("Found cloudimage_ignore.yaml")

		var config model.CloudImageIgnoreConfig

		// Extract global patterns
		if raw := ignoreViper.Get("global.patterns"); raw != nil {
			if patterns, ok := raw.([]any); ok {
				for _, p := range patterns {
					if s, ok := p.(string); ok {
						config.Global.Patterns = append(config.Global.Patterns, s)
					}
				}
			}
		}

		// Extract CSP-specific patterns
		config.CSPs = make(map[string]model.CSPImageIgnorePatterns)
		if cspsRaw := ignoreViper.Get("csps"); cspsRaw != nil {
			if cspsMap, ok := cspsRaw.(map[string]any); ok {
				for cspName, cspDataRaw := range cspsMap {
					if cspData, ok := cspDataRaw.(map[string]any); ok {
						var cspConfig model.CSPImageIgnorePatterns

						if desc, exists := cspData["description"]; exists {
							if s, ok := desc.(string); ok {
								cspConfig.Description = s
							}
						}

						if raw, exists := cspData["global_patterns"]; exists && raw != nil {
							if patterns, ok := raw.([]any); ok {
								for _, p := range patterns {
									if s, ok := p.(string); ok {
										cspConfig.GlobalPatterns = append(cspConfig.GlobalPatterns, s)
									}
								}
							}
						}

						if metaRaw, exists := cspData["metadata_filters"]; exists && metaRaw != nil {
							if metaList, ok := metaRaw.([]any); ok {
								for _, mRaw := range metaList {
									if mMap, ok := mRaw.(map[string]any); ok {
										var mf model.MetadataFilter
										if k, ok := mMap["key"].(string); ok {
											mf.Key = k
										}
										if v, ok := mMap["value"].(string); ok {
											mf.Value = v
										}
										if d, ok := mMap["description"].(string); ok {
											mf.Description = d
										}
										if mf.Key != "" && mf.Value != "" {
											cspConfig.MetadataFilters = append(cspConfig.MetadataFilters, mf)
										}
									}
								}
							}
						}

						if regionsRaw, exists := cspData["regions"]; exists && regionsRaw != nil {
							if regionsMap, ok := regionsRaw.(map[string]any); ok {
								cspConfig.Regions = make(map[string]model.RegionIgnorePatterns)
								for regionName, regionDataRaw := range regionsMap {
									var regionConfig model.RegionIgnorePatterns
									if regionPatterns, ok := regionDataRaw.([]any); ok {
										for _, p := range regionPatterns {
											if s, ok := p.(string); ok {
												regionConfig.Patterns = append(regionConfig.Patterns, s)
											}
										}
									}
									cspConfig.Regions[regionName] = regionConfig
								}
							}
						}

						config.CSPs[cspName] = cspConfig
					}
				}
			}
		}

		imageIgnoreConfig = &config
		log.Info().Msg("Successfully loaded cloudimage_ignore.yaml")
	})

	return imageIgnoreConfig, imageIgnoreConfigErr
}

var (
	imageIgnoreRegexpCache   = make(map[string]*regexp.Regexp)
	imageIgnoreRegexpCacheMu sync.RWMutex
)

// compileIgnorePattern converts a glob pattern (supporting *, ?, and [abc]
// character classes) into a compiled regexp that performs case-insensitive
// substring matching. Unlike filepath.Match, the pattern is NOT anchored to
// the whole string and '*' also matches '/'. This is required because the
// matched text combines imageName + osDistribution, where the descriptive
// name may appear anywhere (e.g. AWS prepends an opaque "ami-xxxx" ID).
// Compiled patterns are cached for reuse.
func compileIgnorePattern(pattern string) (*regexp.Regexp, error) {
	imageIgnoreRegexpCacheMu.RLock()
	if re, ok := imageIgnoreRegexpCache[pattern]; ok {
		imageIgnoreRegexpCacheMu.RUnlock()
		return re, nil
	}
	imageIgnoreRegexpCacheMu.RUnlock()

	var sb strings.Builder
	sb.WriteString("(?i)") // case-insensitive, substring (no ^...$ anchors)
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		case '[':
			// Pass through a character class up to the matching ']'.
			j := i + 1
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			if j >= len(pattern) {
				// No closing ']': treat '[' literally.
				sb.WriteString(regexp.QuoteMeta("["))
			} else {
				sb.WriteString(pattern[i : j+1])
				i = j
			}
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}

	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, err
	}

	imageIgnoreRegexpCacheMu.Lock()
	imageIgnoreRegexpCache[pattern] = re
	imageIgnoreRegexpCacheMu.Unlock()
	return re, nil
}

// matchIgnorePattern reports whether text matches the glob pattern using
// case-insensitive substring semantics.
func matchIgnorePattern(text, pattern, source string) bool {
	re, err := compileIgnorePattern(pattern)
	if err != nil {
		log.Warn().Err(err).Str("pattern", pattern).Msgf("Invalid glob in cloudimage_ignore.yaml %s", source)
		return false
	}
	return re.MatchString(text)
}

// shouldSkipImage returns true if the image matches any filter in cloudimage_ignore.yaml.
// Name/OS patterns are checked against imageName + " " + osDistribution (case-insensitive).
// Metadata filters are checked against the image's KeyValueList (exact case-insensitive match).
func shouldSkipImage(imageName, osDistribution, providerName, regionName string, kvList []model.KeyValue) bool {
	config, err := loadCloudImageIgnoreConfig()
	if err != nil || config == nil {
		return false
	}

	combined := imageName + " " + osDistribution

	// Check global patterns
	for _, pattern := range config.Global.Patterns {
		if matchIgnorePattern(combined, pattern, "global.patterns") {
			return true
		}
	}

	// Check CSP-specific patterns
	cspKey := strings.ToLower(csp.ResolveCloudPlatform(providerName))
	cspConfig, exists := config.CSPs[cspKey]
	if !exists {
		return false
	}

	for _, pattern := range cspConfig.GlobalPatterns {
		if matchIgnorePattern(combined, pattern, "csps.*.global_patterns") {
			return true
		}
	}

	if regionPatterns, ok := cspConfig.Regions[regionName]; ok {
		for _, pattern := range regionPatterns.Patterns {
			if matchIgnorePattern(combined, pattern, "csps.*.regions") {
				return true
			}
		}
	}

	// Check metadata filters (key-value pairs in the image's CSP metadata)
	for _, mf := range cspConfig.MetadataFilters {
		for _, kv := range kvList {
			if kv.Key == mf.Key && strings.EqualFold(kv.Value, mf.Value) {
				return true
			}
		}
	}

	return false
}
