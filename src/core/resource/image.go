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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	azurecsp "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ImageReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.ImageReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// usesTypeIdentifierK8sImage returns true for CSPs whose K8s node images are
// abstract type identifiers (e.g., AMI type, image family) registered via
// cloudimage.csv, not real images discovered through the CSP image API.
//
// For these CSPs, IsKubernetesImage=true is set only by cloudimage.csv loading
// (keyword-based detection on VM images is skipped), and K8s cluster creation
// must validate that the requested imageId resolves to a registered entry.
func usesTypeIdentifierK8sImage(providerName string) bool {
	switch csp.ResolveCloudPlatform(providerName) {
	case csp.AWS, csp.GCP, csp.Tencent, csp.Alibaba:
		return true
	}
	return false
}

// ConvertSpiderImageToTumblebugImage accepts an Spider image object, converts to and returns an TB image object
func ConvertSpiderImageToTumblebugImage(nsId, connConfig string, spiderImage model.SpiderImageInfo) (model.ImageInfo, error) {

	regionAgnosticProviders := []string{csp.Azure, csp.GCP}

	// Note: regionAgnosticProviders is checked against resolved platform below via slices.Contains.

	if spiderImage.IId.NameId == "" {
		err := fmt.Errorf("ConvertSpiderImageToTumblebugImage failed; spiderImage.IId.NameId == EmptyString")
		emptyTumblebugImage := model.ImageInfo{}
		return emptyTumblebugImage, err
	}

	connectionConfig, err := common.GetConnConfig(connConfig)
	if err != nil {
		err = fmt.Errorf("cannot retrieve ConnectionConfig %s: %v", connectionConfig.ConfigName, err)
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	cspImageName := spiderImage.IId.NameId
	providerName := connectionConfig.ProviderName
	platformName := csp.ResolveCloudPlatform(providerName)
	currentRegion := connectionConfig.RegionDetail.RegionName
	if slices.Contains(regionAgnosticProviders, platformName) {
		// For region-agnostic providers, use common region
		currentRegion = model.StrCommon
	}

	// Create new image instance
	tumblebugImage := model.ImageInfo{}

	// // Generate ID for backward compatibility
	// tumblebugImageId := GetProviderRegionZoneResourceKey(providerName, "", "", cspImageName)
	tumblebugImageId := cspImageName

	// Set basic fields
	tumblebugImage.ResourceType = model.StrImage
	tumblebugImage.Id = tumblebugImageId
	tumblebugImage.Name = tumblebugImageId
	tumblebugImage.Uid = common.GenUid()
	tumblebugImage.Namespace = nsId
	tumblebugImage.ConnectionName = connConfig
	tumblebugImage.ProviderName = providerName
	tumblebugImage.FetchedTime = time.Now().Format("2006.01.02 15:04:05 Mon")

	// Set region information (array and default region)
	tumblebugImage.RegionList = make([]string, 0)
	tumblebugImage.RegionList = append(tumblebugImage.RegionList, currentRegion)

	tumblebugImage.CspImageName = spiderImage.IId.NameId
	tumblebugImage.Description = common.LookupKeyValueList(spiderImage.KeyValueList, "Description")
	tumblebugImage.CreationDate = common.LookupKeyValueList(spiderImage.KeyValueList, "CreationDate")

	tumblebugImage.OSPlatform = spiderImage.OSPlatform
	tumblebugImage.OSDistribution = spiderImage.OSDistribution
	if platformName == csp.NHN {
		// For NHN Cloud, we need to extract the OS distribution from KeyValueList
		tumblebugImage.OSDistribution = common.LookupKeyValueList(spiderImage.KeyValueList, "Name")
	}
	if platformName == csp.NCP {
		// For NCP, we need to extract the hypervisor type from KeyValueList and append it to the OSDistribution
		hypervisorInfo := common.LookupKeyValueList(spiderImage.KeyValueList, "HypervisorType")
		if hypervisorInfo != "" {
			if strings.Contains(strings.ToUpper(hypervisorInfo), "KVM") {
				hypervisorInfo = "KVM"
			} else if strings.Contains(strings.ToUpper(hypervisorInfo), "XEN") {
				hypervisorInfo = "Xen"
			} else if strings.Contains(strings.ToUpper(hypervisorInfo), "BM") {
				hypervisorInfo = "BM"
			}
		} else {
			// If hypervisor type is not found, we can set it to "Unknown"
			hypervisorInfo = "Unknown"
		}
		tumblebugImage.OSDistribution += " (Hypervisor:" + hypervisorInfo + ")"
	}

	// Stringify Values in the KeyValueList for information extraction
	strDetails := ""
	strSeparator := " "
	values := make([]string, len(spiderImage.KeyValueList))
	for i, kv := range spiderImage.KeyValueList {
		values[i] = kv.Value
	}
	strDetails = strings.Join(values, strSeparator)

	// Extract OS Type primarily from OSDistribution and NameId to avoid false version matches
	// from random numbers/UUIDs/dates in KeyValueList; fall back to full details only if empty.
	primaryOSStr := fmt.Sprintf("%s%s%s", tumblebugImage.OSDistribution, strSeparator, spiderImage.IId.NameId)
	tumblebugImage.OSType = common.ExtractOSInfo(primaryOSStr)
	if tumblebugImage.OSType == "" {
		fullOSStr := fmt.Sprintf("%s%s%s", primaryOSStr, strSeparator, strDetails)
		tumblebugImage.OSType = common.ExtractOSInfo(fullOSStr)
	}

	searchStr := fmt.Sprintf("%s%s%s%s%s", spiderImage.IId.NameId, strSeparator, tumblebugImage.OSDistribution, strSeparator, strDetails)
	// Check if this is a GPU image
	if common.IsGPUImage(searchStr) {
		tumblebugImage.IsGPUImage = true
	}
	// Check if this is a Kubernetes image.
	// Skip keyword-based detection for type-identifier CSPs (AWS/GCP/Tencent):
	// their IsKubernetesImage=true is established via cloudimage.csv loading only.
	if !usesTypeIdentifierK8sImage(platformName) {
		if common.IsK8sImage(searchStr) {
			tumblebugImage.IsKubernetesImage = true
		}
	}
	tumblebugImage.ImageStatus = spiderImage.ImageStatus
	// Check if this is a deprecated image
	if common.IsDeprecatedImage(searchStr) {
		tumblebugImage.ImageStatus = model.ImageDeprecated
	}

	// Set additional fields
	tumblebugImage.OSArchitecture = model.OSArchitecture(strings.ToLower(string(spiderImage.OSArchitecture)))

	// Handle specific cases for OSArchitecture when Spider returns NA or empty
	if spiderImage.OSArchitecture == model.ArchitectureNA || tumblebugImage.OSArchitecture == "" {
		primaryLower := strings.ToLower(primaryOSStr)
		switch {
		case strings.Contains(primaryLower, "arm64") || strings.Contains(primaryLower, "aarch64"):
			tumblebugImage.OSArchitecture = model.ARM64
		case strings.Contains(primaryLower, "s390x"):
			tumblebugImage.OSArchitecture = model.S390X
		case strings.Contains(primaryLower, "32bit") || strings.Contains(primaryLower, "32-bit") || strings.Contains(primaryLower, "i386"):
			tumblebugImage.OSArchitecture = model.X86_32
		case strings.Contains(primaryLower, "x86_64") || strings.Contains(primaryLower, "amd64") || strings.Contains(primaryLower, "x64") || strings.Contains(primaryLower, "64bit"):
			tumblebugImage.OSArchitecture = model.X86_64
		case platformName == csp.KT || platformName == csp.GCP || platformName == csp.OpenStack:
			tumblebugImage.OSArchitecture = model.X86_64
		case platformName == csp.IBM:
			tumblebugImage.OSArchitecture = model.S390X
		}
	}

	// Only mark as basic image if the image is "Available"
	// Non-available images (deprecated, unavailable, etc.) should not be considered basic
	// isBasicImage and isBasicGpuImage are mutually exclusive:
	// - isBasicImage: clean non-GPU OS install (no GPU drivers)
	// - isBasicGpuImage: GPU image with drivers pre-installed
	if tumblebugImage.ImageStatus == model.ImageAvailable {
		if tumblebugImage.IsGPUImage {
			tumblebugImage.IsBasicGpuImage = common.CheckBasicGpuImage(searchStr, providerName)
			tumblebugImage.IsBasicImage = false
		} else {
			tumblebugImage.IsBasicImage = common.CheckBasicOSImage(tumblebugImage.OSDistribution, providerName)
			tumblebugImage.IsBasicGpuImage = false
		}
	} else {
		tumblebugImage.IsBasicImage = false
		tumblebugImage.IsBasicGpuImage = false
	}

	tumblebugImage.OSDiskType = spiderImage.OSDiskType
	tumblebugImage.OSDiskSizeGB, _ = strconv.ParseFloat(spiderImage.OSDiskSizeGB, 64)
	if tumblebugImage.OSDiskSizeGB <= 0 {
		for _, diskKey := range []string{"MinDiskGigabytes", "MinimumProvisionedSize", "DiskSizeGb", "Size", "ImageSize"} {
			if valStr := common.LookupKeyValueList(spiderImage.KeyValueList, diskKey); valStr != "" {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(valStr), 64); err == nil && parsed > 0 {
					tumblebugImage.OSDiskSizeGB = parsed
					break
				}
			}
		}
	}

	tumblebugImage.Details = spiderImage.KeyValueList

	return tumblebugImage, nil
}

// GetImageInfoFromLookupImage looks up image from Spider and converts to TumblebugImage
// allowCustomImage: if false, only looks up public images (used for enrichment); if true, also checks custom images
func GetImageInfoFromLookupImage(nsId string, u model.ImageReq, allowCustomImage ...bool) (model.ImageInfo, error) {
	content := model.ImageInfo{}

	// Default to true for backward compatibility (check both public and custom images)
	checkCustom := true
	if len(allowCustomImage) > 0 {
		checkCustom = allowCustomImage[0]
	}

	var res model.SpiderImageInfo
	var err error

	if checkCustom {
		res, err = LookupImage(u.ConnectionName, u.CspImageName)
	} else {
		res, err = LookupPublicImageOnly(u.ConnectionName, u.CspImageName)
	}

	if err != nil {
		log.Trace().Err(err).Msg("")
		return content, err
	}
	if res.IId.NameId == "" {
		err := fmt.Errorf("spider returned empty IId.NameId without Error: %s", u.ConnectionName)
		log.Error().Err(err).Msgf("Cannot LookupImage %s %v", u.CspImageName, res)
		return content, err
	}
	if res.ImageStatus == model.ImageUnavailable {
		err := fmt.Errorf("image status of %s is unavailable", u.CspImageName)
		return content, err
	}

	content, err = ConvertSpiderImageToTumblebugImage(nsId, u.ConnectionName, res)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	return content, nil
}

// EnsureImageAvailable checks if an image is available in DB or CSP, and auto-registers if needed.
// It first checks the DB (including CustomImage), then looks up in CSP and registers if found.
// ctx carries the x-credential-holder so any provider-specific "latest image"
// resolution (e.g. Alibaba ImageFamily) uses the requesting tenant's credentials.
// Returns: ImageInfo, isAutoRegistered, error
func EnsureImageAvailable(ctx context.Context, nsId, connectionName, imageId string) (model.ImageInfo, bool, error) {
	if connectionName == "" {
		return model.ImageInfo{}, false, fmt.Errorf("connectionName is required for EnsureImageAvailable")
	}
	if imageId == "" {
		return model.ImageInfo{}, false, fmt.Errorf("imageId is required for EnsureImageAvailable")
	}

	// 1. Check if the image exists in DB (user namespace)
	imageInfo, err := GetImage(nsId, imageId)
	if err == nil {
		log.Debug().Msgf("Image '%s' found in DB (namespace: %s)", imageId, nsId)
		warn, mismatchErr := CheckImageRegionCompatibility(connectionName, imageInfo)
		if mismatchErr != nil {
			return model.ImageInfo{}, false, mismatchErr
		}
		if warn != "" {
			log.Warn().Msg(warn)
		}
		return resolveLatestCspImage(ctx, connectionName, imageInfo), false, nil
	}

	// 2. Check if the image exists in DB (SystemCommonNs)
	imageInfo, err = GetImage(model.SystemCommonNs, imageId)
	if err == nil {
		log.Debug().Msgf("Image '%s' found in DB (namespace: %s)", imageId, model.SystemCommonNs)
		warn, mismatchErr := CheckImageRegionCompatibility(connectionName, imageInfo)
		if mismatchErr != nil {
			return model.ImageInfo{}, false, mismatchErr
		}
		if warn != "" {
			log.Warn().Msg(warn)
		}
		return resolveLatestCspImage(ctx, connectionName, imageInfo), false, nil
	}

	log.Debug().Msgf("Image '%s' not found in DB, checking CSP...", imageId)

	// 3. Try to lookup as a regular image from CSP (Spider /vmimage API)
	spiderImage, lookupErr := lookupRegularImageOnly(connectionName, imageId)
	if lookupErr == nil && spiderImage.IId.NameId != "" {
		log.Info().Msgf("Image '%s' found in CSP as regular image, auto-registering...", imageId)

		// Convert and register as regular image
		imageReq := &model.ImageReq{
			ConnectionName: connectionName,
			CspImageName:   imageId,
			Name:           imageId,
		}
		registeredImage, regErr := RegisterImageWithId(model.SystemCommonNs, imageReq, true, false)
		if regErr != nil {
			log.Warn().Err(regErr).Msgf("Failed to auto-register image '%s', but CSP lookup succeeded", imageId)
			// Even if registration fails, return the converted image info
			tempImage, convErr := ConvertSpiderImageToTumblebugImage(model.SystemCommonNs, connectionName, spiderImage)
			if convErr != nil {
				return model.ImageInfo{}, false, fmt.Errorf("image '%s' found in CSP but failed to convert: %w", imageId, convErr)
			}
			return tempImage, false, nil
		}
		log.Info().Msgf("Successfully auto-registered image '%s' from CSP", imageId)
		return registeredImage, true, nil
	}

	// 4. Try to lookup as a custom image (MyImage) from CSP (Spider /myimage API)
	myImage, myImageErr := LookupMyImage(connectionName, imageId)
	if myImageErr == nil && myImage.IId.NameId != "" {
		log.Info().Msgf("Image '%s' found in CSP as custom image (MyImage), auto-registering...", imageId)

		// Get connection config for provider and region information
		connConfig, configErr := common.GetConnConfig(connectionName)
		if configErr != nil {
			return model.ImageInfo{}, false, fmt.Errorf("failed to get connection config for custom image registration: %w", configErr)
		}

		// Convert Spider MyImage to Tumblebug CustomImage
		customImageInfo, convErr := ConvertSpiderMyImageToTumblebugCustomImage(connConfig, myImage)
		if convErr != nil {
			return model.ImageInfo{}, false, fmt.Errorf("image '%s' found as custom image but failed to convert: %w", imageId, convErr)
		}

		// Set required fields for registration
		customImageInfo.Namespace = nsId
		customImageInfo.Id = imageId
		customImageInfo.Name = imageId
		customImageInfo.Uid = common.GenUid()
		customImageInfo.SystemLabel = "Auto-registered from CSP custom image"

		// Register as custom image
		registeredImage, regErr := RegisterCustomImageWithInfo(nsId, customImageInfo)
		if regErr != nil {
			log.Warn().Err(regErr).Msgf("Failed to auto-register custom image '%s'", imageId)
			// Return the converted info even if registration fails
			return customImageInfo, false, nil
		}
		log.Info().Msgf("Successfully auto-registered custom image '%s' from CSP", imageId)
		return registeredImage, true, nil
	}

	// 5. Image not found anywhere
	return model.ImageInfo{}, false, fmt.Errorf("image '%s' not found in DB or CSP (checked both regular and custom images)", imageId)
}

// CheckImageRegionCompatibility evaluates whether the stored image can be
// used in the target connection's region.
//
// Returns:
//   - ("", nil)        : compatible, or check skipped (empty inputs / lookup failure)
//   - (warning, nil)   : region mismatch but the provider has a family-based
//     resolver (e.g. Alibaba ImageFamily) that will pick the correct image at
//     VM-creation time. Caller may surface the warning.
//   - ("", err)        : region mismatch and no recovery path; treat as hard error.
func CheckImageRegionCompatibility(connectionName string, imageInfo model.ImageInfo) (string, error) {
	if connectionName == "" || len(imageInfo.RegionList) == 0 {
		return "", nil
	}
	connConfig, err := common.GetConnConfig(connectionName)
	if err != nil {
		return "", nil
	}
	targetRegion := connConfig.RegionDetail.RegionName
	if targetRegion == "" {
		return "", nil
	}
	for _, r := range imageInfo.RegionList {
		if strings.EqualFold(r, model.StrCommon) || strings.EqualFold(r, targetRegion) {
			return "", nil
		}
	}

	// Region mismatch detected. Decide recoverability by provider.
	if strings.EqualFold(imageInfo.ProviderName, csp.Alibaba) &&
		extractAlibabaImageFamily(imageInfo.Details) != "" {
		return fmt.Sprintf(
			"image '%s' (CSP id: %s) is registered for region(s) %v but connection '%s' targets region '%s'; "+
				"will auto-resolve to the latest image in the same Alibaba ImageFamily for region '%s' at VM creation",
			imageInfo.Id, imageInfo.CspImageName, imageInfo.RegionList,
			connectionName, targetRegion, targetRegion,
		), nil
	}
	if strings.EqualFold(imageInfo.ProviderName, csp.Azure) {
		if _, _, _, _, ok := parseAzureUrn(imageInfo.CspImageName); ok {
			return fmt.Sprintf(
				"image '%s' (CSP id: %s) is registered for region(s) %v but connection '%s' targets region '%s'; "+
					"will auto-resolve to the latest image version in the same Azure SKU for region '%s' at VM creation",
				imageInfo.Id, imageInfo.CspImageName, imageInfo.RegionList,
				connectionName, targetRegion, targetRegion,
			), nil
		}
	}

	return "", fmt.Errorf(
		"image '%s' (CSP id: %s) is registered for region(s) %v but connection '%s' targets region '%s'; "+
			"pick an image registered for region '%s'",
		imageInfo.Id, imageInfo.CspImageName, imageInfo.RegionList,
		connectionName, targetRegion, targetRegion,
	)
}

// ResolveLatestImageForVMCreation performs CSP-specific "latest image"
// resolution for the given stored ImageInfo, right before the image is handed
// off to cb-spider for VM creation.
//
// Currently implemented for Alibaba Cloud only: Alibaba deprecates individual
// image IDs (date-stamped builds) rapidly while the enclosing ImageFamily
// (e.g. "acs:ubuntu_22_04_x64") remains stable. This function extracts the
// family stored in imageInfo.Details, asks the Alibaba ECS API for the
// latest available image in that family, and returns an ImageInfo copy whose
// CspImageName is replaced with the resolved latest ID. All other fields are
// preserved. The DB record is NOT modified.
//
// For other providers the input is returned unchanged.
//
// Any resolution failure (missing family, missing region, API error, empty
// result) is non-fatal: the original imageInfo is returned so the caller can
// proceed with the stored ID. Observability is provided via logs.
//
// Safe to call more than once per VM creation; resolution is short-circuited
// when the resolved ID equals the stored ID.

func lookupRegularImageOnly(connConfig string, imageId string) (model.SpiderImageInfo, error) {
	if connConfig == "" {
		return model.SpiderImageInfo{}, fmt.Errorf("lookupRegularImageOnly() called with empty connConfig")
	}
	if imageId == "" {
		return model.SpiderImageInfo{}, fmt.Errorf("lookupRegularImageOnly() called with empty imageId")
	}

	client := clientManager.NewHttpClient()
	client.SetTimeout(2 * time.Minute)
	apiUrl := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

	_, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		apiUrl,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.VeryLongDuration,
	)

	if err != nil {
		return model.SpiderImageInfo{}, err
	}

	return callResult, nil
}

// RegisterImageWithInfoInBulk register a list of images in bulk
func RegisterImageWithInfoInBulk(imageList []model.ImageInfo) error {
	// Advanced deduplication logic with region merging
	uniqueImages := make(map[string]model.ImageInfo)
	for _, img := range imageList {
		key := img.Namespace + ":" + img.ProviderName + ":" + img.CspImageName

		// Check if the image already exists in the map
		if existingImg, exists := uniqueImages[key]; exists {
			// log.Debug().Msgf("Found duplicate image: %s/%s/%s",
			// 	img.Namespace, img.ProviderName, img.CspImageName)

			// Merge region information if the image already exists
			// 1. Check and initialize RegionList if nil
			if existingImg.RegionList == nil {
				existingImg.RegionList = make([]string, 0)
			}

			// 2. Merge new image's RegionList information
			if len(img.RegionList) > 0 {
				for _, newRegion := range img.RegionList {
					regionExists := slices.Contains(existingImg.RegionList, newRegion)

					if !regionExists {
						log.Debug().Msgf("Adding region %s to image %s from RegionList",
							newRegion, key)
						existingImg.RegionList = append(existingImg.RegionList, newRegion)
					}
				}
			}

			// Save the updated image
			sort.Strings(existingImg.RegionList)
			uniqueImages[key] = existingImg
		} else {
			// Add new image - initialize and check RegionList
			if img.RegionList == nil {
				img.RegionList = make([]string, 0)
			}
			uniqueImages[key] = img
		}
	}

	// Step 2: Selectively check and merge with existing images in DB
	dedupedImageList := make([]model.ImageInfo, 0, len(uniqueImages))

	for _, img := range uniqueImages {
		// Check if image exists in database
		var dbImage model.ImageInfo
		result := model.ORM.Where("namespace = ? AND provider_name = ? AND csp_image_name = ?",
			img.Namespace, img.ProviderName, img.CspImageName).First(&dbImage)

		if result.Error == nil {
			// Merge region information if image exists in DB
			// log.Debug().Msgf("Found existing image in DB: %s/%s/%s with regions %v",
			// 	img.Namespace, img.ProviderName, img.CspImageName, dbImage.RegionList)

			// Initialize RegionList if nil in DB image
			if dbImage.RegionList == nil {
				dbImage.RegionList = make([]string, 0)
			}

			// Merge DB regions into incoming img (preserve all field updates from img).
			// DB is used only to carry over regions not present in the incoming entry —
			// all other fields (e.g. IsKubernetesImage, OSType) come from img so that
			// policy updates from updateExistingImageFromCSV are not silently discarded.
			for _, dbRegion := range dbImage.RegionList {
				if !slices.Contains(img.RegionList, dbRegion) {
					img.RegionList = append(img.RegionList, dbRegion)
				}
			}
			sort.Strings(img.RegionList)

			dedupedImageList = append(dedupedImageList, img)
		} else {
			// Add new image if not found in DB
			//log.Debug().Msgf("Image not found in DB, will insert new: %s", key)
			dedupedImageList = append(dedupedImageList, img)
		}
	}

	log.Info().Msgf("Identified %d unique images after region merging (from %d total)",
		len(dedupedImageList), len(imageList))

	batchSize := 100

	total := len(dedupedImageList)
	for i := 0; i < total; i += batchSize {
		end := min(i+batchSize, total)
		batch := dedupedImageList[i:end]

		// Batch processing with deadlock retry (max 2 attempts)
		maxRetries := 2
		var result *gorm.DB
		var lastErr error

		for attempt := range maxRetries {
			tx := model.ORM.Begin()
			if tx.Error != nil {
				log.Error().Err(tx.Error).Msg("Failed to begin transaction")
				return tx.Error
			}

			// Use UPSERT approach - update on duplicate
			result = tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "namespace"}, {Name: "provider_name"}, {Name: "csp_image_name"}},
				UpdateAll: true,
			}).CreateInBatches(&batch, len(batch))

			if result.Error != nil {
				tx.Rollback()

				// Check for deadlock (SQLSTATE 40P01)
				if strings.Contains(result.Error.Error(), "40P01") || strings.Contains(result.Error.Error(), "deadlock") {
					if attempt < maxRetries-1 {
						log.Warn().Msgf("Deadlock detected in batch %d-%d, retrying (attempt %d/%d)...", i, end-1, attempt+1, maxRetries)
						continue
					}
					lastErr = result.Error
				}

				// Switch to individual processing if duplicate key error occurs
				if strings.Contains(result.Error.Error(), "duplicate key value") {
					log.Warn().Msg("Falling back to individual record processing due to duplicate key issue")

					// Process individual records
					altTx := model.ORM.Begin()
					for _, img := range batch {
						var exists bool
						altTx.Raw("SELECT EXISTS(SELECT 1 FROM image_infos WHERE namespace = ? AND provider_name = ? AND csp_image_name = ?)",
							img.Namespace, img.ProviderName, img.CspImageName).Scan(&exists)

						if exists {
							// Update - using composite key
							if err := altTx.Model(&model.ImageInfo{}).
								Where("namespace = ? AND provider_name = ? AND csp_image_name = ?",
									img.Namespace, img.ProviderName, img.CspImageName).
								Updates(img).Error; err != nil {
								altTx.Rollback()
								return err
							}
						} else {
							// Insert
							if err := altTx.Create(&img).Error; err != nil {
								altTx.Rollback()
								return err
							}
						}
					}

					if err := altTx.Commit().Error; err != nil {
						return err
					}

					log.Info().Msgf("Individual processing completed for batch %d-%d", i, end-1)
					break
				}

				if lastErr != nil {
					log.Error().Err(lastErr).Msgf("Failed after %d deadlock retry attempts for batch %d-%d", maxRetries, i, end-1)
					return lastErr
				}

				log.Error().Err(result.Error).Msg("Error inserting images in bulk")
				return result.Error
			} else {
				// Success - commit and exit retry loop
				if err := tx.Commit().Error; err != nil {
					log.Error().Err(err).Msg("Failed to commit transaction")
					return err
				}
				break
			}
		}

		//log.Info().Msgf("Bulk insert/update success: %d records affected", result.RowsAffected)
	}

	return nil
}

// RemoveDuplicateImagesInSQL is to remove duplicate images in db to refine batch insert duplicates
func RemoveDuplicateImagesInSQL() error {
	// PostgreSQL deduplication query (using ctid)
	sqlStr := `
    DELETE FROM image_infos
    WHERE ctid NOT IN (
        SELECT MIN(ctid)
        FROM image_infos
        GROUP BY namespace, provider_name, csp_image_name
    );
    `

	result := model.ORM.Exec(sqlStr)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Error deleting duplicate images")
		return result.Error
	}

	log.Info().Msg("Duplicate images removed successfully")
	return nil
}

// RegisterImageWithId accepts image creation request, creates and returns an TB image object
func RegisterImageWithId(nsId string, u *model.ImageReq, update bool, RDBonly bool) (model.ImageInfo, error) {

	content := model.ImageInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	resourceType := model.StrImage
	if !RDBonly {
		check, err := CheckResource(nsId, resourceType, u.Name)
		if !update {
			if check {
				err := fmt.Errorf("The image %s already exists.", u.Name)
				return content, err
			}
		}
		if err != nil {
			err := fmt.Errorf("Failed to check the existence of the image %s.", u.Name)
			return content, err
		}
	}

	res, err := LookupImage(u.ConnectionName, u.CspImageName)
	if err != nil {
		log.Trace().Err(err).Msg("")
		return content, err
	}
	if res.IId.NameId == "" {
		err := fmt.Errorf("CB-Spider returned empty IId.NameId without Error: %s", u.ConnectionName)
		log.Error().Err(err).Msgf("Cannot LookupImage %s %v", u.CspImageName, res)
		return content, err
	}

	content, err = ConvertSpiderImageToTumblebugImage(nsId, u.ConnectionName, res)
	if err != nil {
		log.Error().Err(err).Msg("")
		//err := fmt.Errorf("an error occurred while converting Spider image info to Tumblebug image info.")
		return content, err
	}

	if !RDBonly {
		Key := common.GenResourceKey(nsId, resourceType, content.Id)
		Val, _ := json.Marshal(content)
		err = kvstore.Put(Key, string(Val))
		if err != nil {
			log.Error().Err(err).Msg("")
			return content, err
		}
	}

	// "INSERT INTO `image`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	// Attempt to insert the new record
	result := model.ORM.Create(&content)
	if result.Error != nil {
		if update {
			// If insert fails and update is true, attempt to update the existing record
			updateResult := model.ORM.Model(&model.ImageInfo{}).Where("namespace = ? AND id = ?", content.Namespace, content.Id).Updates(content)
			if updateResult.Error != nil {
				log.Error().Err(updateResult.Error).Msg("Error updating image after insert failure")
				return content, updateResult.Error
			} else {
				log.Trace().Msg("SQL: Update success after insert failure")
			}
		} else {
			log.Error().Err(result.Error).Msg("Error inserting image and update flag is false")
			return content, result.Error
		}
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return content, nil
}

// RegisterImageWithInfo accepts image creation request, creates and returns an TB image object
func RegisterImageWithInfo(nsId string, content *model.ImageInfo, update bool) (model.ImageInfo, error) {

	resourceType := model.StrImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}
	// err = common.CheckString(content.Name)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return model.ImageInfo{}, err
	// }
	check, err := CheckResource(nsId, resourceType, content.Name)

	if !update {
		if check {
			err := fmt.Errorf("The image %s already exists.", content.Name)
			return model.ImageInfo{}, err
		}
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the image %s.", content.Name)
		return model.ImageInfo{}, err
	}

	content.Namespace = nsId
	//content.Id = common.GenUid()
	content.Id = content.Name

	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	// "INSERT INTO `image`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	result := model.ORM.Create(content)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("")
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return *content, nil
}

// LookupImageList accepts Spider conn config,
// lookups and returns the list of all images in the region of conn config
// in the form of the list of Spider image objects
func LookupImageList(connConfigName string) (model.SpiderImageList, error) {
	startTime := time.Now()
	var callResult model.SpiderImageList

	// For Azure, fetch images directly via Azure SDK in cb-tb to avoid Spider dependency.
	if connCfg, cfgErr := common.GetConnConfig(connConfigName); cfgErr == nil {
		if strings.EqualFold(csp.ResolveCloudPlatform(connCfg.ProviderName), csp.Azure) {
			region := connCfg.RegionDetail.RegionName
			if region == "" {
				region = connCfg.RegionZoneInfo.AssignedRegion
			}

			log.Debug().Str("connConfig", connConfigName).Str("region", region).Msg("[AzureImage] LookupImageList using direct Azure SDK")

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
			defer cancel()

			if connCfg.CredentialHolder != "" {
				ctx = common.WithCredentialHolder(ctx, connCfg.CredentialHolder)
			}

			images, err := azurecsp.ListImages(ctx, region)
			if err != nil {
				log.Error().Err(err).Str("connConfig", connConfigName).Str("region", region).Msg("Failed direct Azure image listing")
				return callResult, err
			}

			callResult.Image = images
			elapsed := time.Since(startTime)
			log.Info().Str("connConfig", connConfigName).Str("region", region).Int("imageCount", len(images)).Dur("totalDuration", elapsed).Msg("[AzureImage] LookupImageList completed via direct SDK")
			return callResult, nil
		}
	}

	client := clientManager.NewHttpClient()
	client.SetTimeout(100 * time.Minute)

	url := model.SpiderRestUrl + "/vmimage"
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfigName

	_, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Lookup Image List from Spider")
		return callResult, err
	}
	return callResult, nil
}

// LookupPublicImageOnly accepts Spider conn config and CSP image ID, lookups public image only (no custom image fallback)
// Used for enrichment during asset loading to avoid unnecessary custom image checks
// For Azure, uses direct Azure SDK instead of Spider API
func LookupPublicImageOnly(connConfig string, imageId string) (model.SpiderImageInfo, error) {
	startTime := time.Now()
	if connConfig == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("lookupPublicImageOnly() called with empty connConfig")
		log.Error().Err(err).Msg("")
		return content, err
	} else if imageId == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("lookupPublicImageOnly() called with empty imageId")
		log.Error().Err(err).Msg("")
		return content, err
	}

	// For Azure, use direct SDK instead of Spider
	connCfg, err := common.GetConnConfig(connConfig)
	if err == nil && strings.EqualFold(csp.ResolveCloudPlatform(connCfg.ProviderName), csp.Azure) {
		// Extract region from connection config
		region := connCfg.RegionDetail.RegionName
		if region == "" {
			region = connCfg.RegionZoneInfo.AssignedRegion
		}

		if strings.TrimSpace(region) == "" {
			content := model.SpiderImageInfo{}
			err := fmt.Errorf("lookupPublicImageOnly() failed to determine Azure region from connConfig '%s'", connConfig)
			log.Error().Err(err).Msg("")
			return content, err
		}

		log.Debug().Str("connConfig", connConfig).Str("imageId", imageId).Str("region", region).Msg("[AzureImage] LookupPublicImageOnly using direct Azure SDK")

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
		defer cancel()

		if connCfg.CredentialHolder != "" {
			ctx = common.WithCredentialHolder(ctx, connCfg.CredentialHolder)
		}

		// Call direct Azure SDK
		image, azureErr := azurecsp.GetImage(ctx, region, imageId)
		if azureErr != nil {
			log.Trace().Err(azureErr).Str("connConfig", connConfig).Str("imageId", imageId).Msg("Azure image lookup failed (using direct SDK)")
			return model.SpiderImageInfo{}, azureErr
		}

		elapsed := time.Since(startTime)
		log.Debug().Str("connConfig", connConfig).Str("imageId", imageId).Dur("duration", elapsed).Msg("[AzureImage] LookupPublicImageOnly completed via direct SDK")
		return image, nil
	}

	// For non-Azure CSPs, use Spider API (existing behavior)
	client := clientManager.NewHttpClient()
	client.SetTimeout(2 * time.Minute)
	apiUrl := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		apiUrl,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("Public image lookup failed (no custom image fallback for enrichment)")
		return callResult, err
	}

	return callResult, nil
}

// LookupImage accepts Spider conn config and CSP image ID, lookups and returns the Spider image object
// If the regular image lookup fails, it checks for custom images in the database
func LookupImage(connConfig string, imageId string) (model.SpiderImageInfo, error) {

	if connConfig == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("lookupImage() called with empty connConfig")
		log.Error().Err(err).Msg("")
		return content, err
	} else if imageId == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("lookupImage() called with empty imageId")
		log.Error().Err(err).Msg("")
		return content, err
	}

	client := clientManager.NewHttpClient()
	client.SetTimeout(2 * time.Minute)
	apiUrl := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

	_, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		apiUrl,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("Failed to lookup regular image")

		// Try to check if it's a custom image directly from Spider
		log.Debug().Msgf("Checking if '%s' exists as a custom image", imageId)

		// Query Spider for custom image using ExecuteHttpRequest
		customImageUrl := model.SpiderRestUrl + "/myimage/" + url.QueryEscape(imageId)
		requestBody.ConnectionName = connConfig

		var spiderMyImageResult model.SpiderMyImageInfo

		_, statusErr := clientManager.ExecuteHttpRequest(
			client,
			method,
			customImageUrl,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&spiderMyImageResult,
			clientManager.MediumDuration,
		)

		if statusErr != nil {
			// Custom image also not found in Spider
			enhancedErr := fmt.Errorf("image '%s' not found in both regular and custom images: %w", imageId, err)
			log.Trace().Err(enhancedErr).Msg("Image not found in both sources")
			return callResult, enhancedErr
		}

		// Successfully found custom image in Spider
		currentStatus := model.ImageStatus(spiderMyImageResult.Status)

		// Check if status is Available
		if currentStatus == model.ImageAvailable {
			// Custom image is available - return success with nil error
			log.Debug().Msgf("Custom image found and available with status: %s", currentStatus)
			return callResult, nil
		} else {
			// Custom image exists but status is not Available
			enhancedErr := fmt.Errorf("custom image exists but has status '%s' (not Available yet): %w",
				currentStatus, err)
			log.Trace().Err(enhancedErr).Msgf("Custom image found with status: %s", currentStatus)
			return callResult, enhancedErr
		}
	}

	return callResult, nil
}

// FetchImagesForConnConfig gets lookups all images for the region of conn config, and saves into TB image objects

func UpdateImage(nsId string, imageId string, fieldsToUpdate model.ImageInfo, RDBonly bool) (model.ImageInfo, error) {
	if !RDBonly {

		resourceType := model.StrImage
		temp := model.ImageInfo{}
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}

		if len(fieldsToUpdate.Namespace) > 0 {
			err := fmt.Errorf("You should not specify 'namespace' in the JSON request body.")
			log.Error().Err(err).Msg("")
			return temp, err
		}

		if len(fieldsToUpdate.Id) > 0 {
			err := fmt.Errorf("You should not specify 'id' in the JSON request body.")
			log.Error().Err(err).Msg("")
			return temp, err
		}

		check, err := CheckResource(nsId, resourceType, imageId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}

		if !check {
			err := fmt.Errorf("The image %s does not exist.", imageId)
			return temp, err
		}

		tempInterface, err := GetResource(nsId, resourceType, imageId)
		if err != nil {
			err := fmt.Errorf("Failed to get the image %s.", imageId)
			return temp, err
		}
		asIsImage := model.ImageInfo{}
		err = common.CopySrcToDest(&tempInterface, &asIsImage)
		if err != nil {
			err := fmt.Errorf("Failed to CopySrcToDest() %s.", imageId)
			return temp, err
		}

		// Update specified fields only
		toBeImage := asIsImage
		toBeImageJSON, _ := json.Marshal(fieldsToUpdate)
		err = json.Unmarshal(toBeImageJSON, &toBeImage)

		Key := common.GenResourceKey(nsId, resourceType, toBeImage.Id)
		Val, _ := json.Marshal(toBeImage)
		err = kvstore.Put(Key, string(Val))
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}

	}
	// "UPDATE `image` SET `id`='" + imageId + "', ... WHERE `namespace`='" + nsId + "' AND `id`='" + imageId + "';"
	result := model.ORM.Model(&model.ImageInfo{}).Where("namespace = ? AND id = ?", nsId, imageId).Updates(fieldsToUpdate)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("")
		return fieldsToUpdate, result.Error
	} else {
		log.Trace().Msg("SQL: Update success")
	}

	return fieldsToUpdate, nil
}

// GetImage accepts namespace Id and imageKey(CspImageName), and returns the TB image object
// imageInfoCache caches successful GetImage lookups.
// ImageInfo is immutable once registered, so process-lifetime caching is safe.
var imageInfoCache sync.Map // key: "nsId/cspImageName", value: model.ImageInfo

// GetImage retrieves an image by namespace and CSP image name, with process-level caching.
func GetImage(nsId string, cspImageName string) (model.ImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.ImageInfo{}, err
	}
	cacheKey := strings.ToLower(nsId) + "/" + strings.ToLower(cspImageName)
	if cached, ok := imageInfoCache.Load(cacheKey); ok {
		return cached.(model.ImageInfo), nil
	}
	img, err := getImageFromDB(nsId, cspImageName)
	if err == nil {
		// Custom images in non-stable states (Creating, etc.) must not be cached:
		// their status transitions over time and GetImage does not refresh from Spider.
		// Only stable states (Available, Failed, Deprecated) are safe to cache permanently.
		if img.ResourceType != model.StrCustomImage || isStableImageStatus(img.ImageStatus) {
			imageInfoCache.Store(cacheKey, img)
		}
	}
	return img, err
}

func getImageFromDB(nsId string, cspImageName string) (model.ImageInfo, error) {
	log.Debug().Msg("[Get image] " + cspImageName)

	// Normalize the image name to lower case for searching
	cspImageName = strings.ToLower(cspImageName)

	// imageKey does not include information for providerName, regionName
	// 1) Check if the image is a custom image
	// ex: custom-img-487zeit5
	var customImage model.ImageInfo
	result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ? AND resource_type = ?",
		nsId, cspImageName, model.StrCustomImage).First(&customImage)
	if result.Error == nil {
		return customImage, nil
	}

	providerName, regionName, _, imageIdentifier, err := ResolveProviderRegionZoneResourceKey(cspImageName)
	if err != nil {

		// 1) Check if the image is a registered image in the common namespace model.SystemCommonNs by ImageId
		// ex: tencent+ap-jakarta+ubuntu22.04 or tencent+ap-jakarta+img-487zeit5
		image := model.ImageInfo{Namespace: model.SystemCommonNs, Id: cspImageName}
		result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ?", model.SystemCommonNs, cspImageName).First(&image)
		if result.Error != nil {
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return model.ImageInfo{}, fmt.Errorf("DB error looking up image '%s' by ID in %s: %w", cspImageName, model.SystemCommonNs, result.Error)
			}
			log.Info().Err(result.Error).Msgf("Cannot get image %s by ID from %s", cspImageName, model.SystemCommonNs)
		} else {
			return image, nil
		}

		// 2) Check if the image is a registered image in the given namespace
		// ex: img-487zeit5

		result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(csp_image_name) = ?", nsId, cspImageName).First(&image)
		if result.Error != nil {
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return model.ImageInfo{}, fmt.Errorf("DB error looking up image '%s' by CspImageName in %s: %w", cspImageName, nsId, result.Error)
			}
			log.Info().Err(result.Error).Msgf("Cannot get image %s by ID from %s", cspImageName, nsId)
		} else {
			return image, nil
		}
	} else {
		// imageKey includes information for providerName, regionName

		// 1) Check if the image is a registered image in the common namespace model.SystemCommonNs by CspImageName
		// ex: tencent+img-487zeit5
		image, err := GetImageByPrimaryKey(model.SystemCommonNs, providerName, imageIdentifier)
		if err != nil {
			log.Info().Err(result.Error).Msgf("Cannot get image %s by CspImageName", imageIdentifier)
		} else {
			return image, nil
		}

		// 2) Check if the image is a registered image in the common namespace model.SystemCommonNs by GuestOS
		// ex: tencent+ap-jakarta+Ubuntu22.04

		//isKubernetesImage := false
		isRegisteredByAsset := true
		includeDeprecatedImage := false

		req := model.SearchImageRequest{
			ProviderName:           providerName,
			RegionName:             regionName,
			OSType:                 imageIdentifier,
			IsRegisteredByAsset:    &isRegisteredByAsset,
			IncludeDeprecatedImage: &includeDeprecatedImage,
		}

		images, imageCnt, err := SearchImage(model.SystemCommonNs, req, false)
		if err != nil || imageCnt == 0 {
			log.Info().Err(result.Error).Msgf("Failed to get image %s by OS type", imageIdentifier)
		} else {
			// Return the first image found
			return images[0], nil
		}
	}

	return model.ImageInfo{}, fmt.Errorf("The imageKey %s not found by any of ID, CspImageName, GuestOS", cspImageName)
}

// GetImageByPrimaryKey retrieves image information based on namespace, provider, and CSP image name
func GetImageByPrimaryKey(nsId string, provider string, cspImageName string) (model.ImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.ImageInfo{}, err
	}

	log.Debug().Msgf("[Get image] Namespace: %s, Provider: %s, CSP Image Name: %s", nsId, provider, cspImageName)

	// Convert inputs to lowercase for case-insensitive comparison
	nsId = strings.ToLower(nsId)
	provider = strings.ToLower(provider)
	cspImageName = strings.ToLower(cspImageName)

	// Query the database for the image
	var image model.ImageInfo
	result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(provider_name) = ? AND LOWER(csp_image_name) = ?", nsId, provider, cspImageName).First(&image)
	if result.Error != nil {
		log.Debug().Err(result.Error).Msgf("Failed to retrieve image for Namespace: %s, Provider: %s, CSP Image Name: %s", nsId, provider, cspImageName)
		return model.ImageInfo{}, result.Error
	}

	return image, nil
}

// GetImagesByRegion retrieves images based on namespace, provider, and region
func GetImagesByRegion(nsId string, provider string, region string) ([]model.ImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return nil, err
	}

	log.Debug().Msgf("[Get images] Namespace: %s, Provider: %s, Region: %s", nsId, provider, region)

	// Convert inputs to lowercase for case-insensitive comparison
	nsId = strings.ToLower(nsId)
	provider = strings.ToLower(provider)
	region = strings.ToLower(region)

	// Query the database for the images
	var images []model.ImageInfo
	result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(provider_name) = ? AND LOWER(region_list) LIKE ?", nsId, provider, "%"+region+"%").Find(&images)
	if result.Error != nil {
		log.Error().Err(result.Error).Msgf("Failed to retrieve images for Namespace: %s, Provider: %s, Region: %s", nsId, provider, region)
		return nil, result.Error
	}

	return images, nil
}

// applyCspSpecificImageFiltering applies CSP-specific filtering rules based on spec information
