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
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm/clause"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"

	"slices"

	validator "github.com/go-playground/validator/v10"
)

// ImageReqStructLevelValidation func is for Validation
func ImageReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.ImageReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// ConvertSpiderImageToTumblebugImage accepts an Spider image object, converts to and returns an TB image object
func ConvertSpiderImageToTumblebugImage(nsId, connConfig string, spiderImage model.SpiderImageInfo) (model.ImageInfo, error) {

	regionAgnosticProviders := []string{csp.Azure, csp.GCP, csp.Tencent}

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
	currentRegion := connectionConfig.RegionDetail.RegionName
	if slices.Contains(regionAgnosticProviders, providerName) {
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

	// Stringify Values in the KeyValueList for information extraction
	strDetails := ""
	strSeparator := " "
	values := make([]string, len(spiderImage.KeyValueList))
	for i, kv := range spiderImage.KeyValueList {
		values[i] = kv.Value
	}
	strDetails = strings.Join(values, strSeparator)

	// Extract OS, GPU, K8s information
	searchStr := fmt.Sprintf("%s%s%s%s%s", spiderImage.OSDistribution, strSeparator, spiderImage.IId.NameId, strSeparator, strDetails)
	tumblebugImage.OSType = common.ExtractOSInfo(searchStr)

	searchStr = fmt.Sprintf("%s%s%s%s%s", spiderImage.IId.NameId, strSeparator, spiderImage.OSDistribution, strSeparator, strDetails)
	// Check if this is a GPU image
	if common.IsGPUImage(searchStr) {
		tumblebugImage.IsGPUImage = true
	}
	// Check if this is a Kubernetes image
	if common.IsK8sImage(searchStr) {
		tumblebugImage.InfraType = "k8s|kubernetes|container"
		tumblebugImage.IsKubernetesImage = true
	}
	tumblebugImage.ImageStatus = spiderImage.ImageStatus
	// Check if this is a deprecated image
	if common.IsDeprecatedImage(searchStr) {
		tumblebugImage.ImageStatus = model.ImageDeprecated
	}

	// Set additional fields
	tumblebugImage.OSArchitecture = model.OSArchitecture(strings.ToLower(string(spiderImage.OSArchitecture)))

	// Handle specific cases for OSArchitecture
	// KT Cloud and IBM Cloud have specific architecture mappings
	if spiderImage.OSArchitecture == model.ArchitectureNA {
		// For KT Cloud, we set X86_64 if the architecture is not specified
		if providerName == csp.KT {
			tumblebugImage.OSArchitecture = model.X86_64
		}
		// For IBM Cloud, we set S390X if the architecture is not specified
		if providerName == csp.IBM {
			tumblebugImage.OSArchitecture = model.S390X
		}
	}
	tumblebugImage.OSPlatform = spiderImage.OSPlatform
	tumblebugImage.OSDistribution = spiderImage.OSDistribution
	if providerName == csp.NHN {
		// For NHN Cloud, we need to extract the OS distribution from KeyValueList
		tumblebugImage.OSDistribution = common.LookupKeyValueList(spiderImage.KeyValueList, "Name")
	}
	if providerName == csp.NCP {
		// For NCP, we need to extract the hypervisor type from KeyValueList and append it to the OSDistribution
		hypervisorInfo := common.LookupKeyValueList(spiderImage.KeyValueList, "HypervisorType")
		if hypervisorInfo != "" {
			if strings.Contains(strings.ToUpper(hypervisorInfo), "KVM") {
				hypervisorInfo = "KVM"
			} else if strings.Contains(strings.ToUpper(hypervisorInfo), "XEN") {
				hypervisorInfo = "Xen"
			}
		} else {
			// If hypervisor type is not found, we can set it to "Unknown"
			hypervisorInfo = "Unknown"
		}
		tumblebugImage.OSDistribution += " (Hypervisor:" + hypervisorInfo + ")"
	}

	tumblebugImage.IsBasicImage = common.CheckBasicOSImage(tumblebugImage.OSDistribution, providerName)

	tumblebugImage.OSDiskType = spiderImage.OSDiskType
	tumblebugImage.OSDiskSizeGB, _ = strconv.ParseFloat(spiderImage.OSDiskSizeGB, 64)

	tumblebugImage.Details = spiderImage.KeyValueList

	return tumblebugImage, nil
}

// GetImageInfoFromLookupImage
func GetImageInfoFromLookupImage(nsId string, u model.ImageReq) (model.ImageInfo, error) {
	content := model.ImageInfo{}
	res, err := LookupImage(u.ConnectionName, u.CspImageName)
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
// Returns: ImageInfo, isAutoRegistered, error
func EnsureImageAvailable(nsId, connectionName, imageId string) (model.ImageInfo, bool, error) {
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
		return imageInfo, false, nil
	}

	// 2. Check if the image exists in DB (SystemCommonNs)
	imageInfo, err = GetImage(model.SystemCommonNs, imageId)
	if err == nil {
		log.Debug().Msgf("Image '%s' found in DB (namespace: %s)", imageId, model.SystemCommonNs)
		return imageInfo, false, nil
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

// lookupRegularImageOnly looks up only regular images from CSP (Spider /vmimage API).
// Unlike LookupImage, this does NOT fall back to checking custom images (MyImage).
// This is a simpler version of LookupImage for cases where we need to distinguish
// between regular images and custom images explicitly.
// See also: LookupImage (with CustomImage fallback), LookupMyImage (CustomImage only)
func lookupRegularImageOnly(connConfig string, imageId string) (model.SpiderImageInfo, error) {
	if connConfig == "" {
		return model.SpiderImageInfo{}, fmt.Errorf("lookupRegularImageOnly() called with empty connConfig")
	}
	if imageId == "" {
		return model.SpiderImageInfo{}, fmt.Errorf("lookupRegularImageOnly() called with empty imageId")
	}

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	apiUrl := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

	err := clientManager.ExecuteHttpRequest(
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

			// Merge new region information
			regionsAdded := false
			for _, newRegion := range img.RegionList {
				regionExists := slices.Contains(dbImage.RegionList, newRegion)

				if !regionExists {
					// log.Debug().Msgf("Adding region %s to DB image %s",
					// 	newRegion, key)
					dbImage.RegionList = append(dbImage.RegionList, newRegion)
					regionsAdded = true
				}
			}

			if regionsAdded {
				// Sort regions
				sort.Strings(dbImage.RegionList)

				// log.Info().Msgf("Merged regions for image %s: %v",
				// 	key, dbImage.RegionList)
			}

			dedupedImageList = append(dedupedImageList, dbImage)
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
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := dedupedImageList[i:end]

		tx := model.ORM.Begin()
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to begin transaction")
			return tx.Error
		}

		// Use UPSERT approach - update on duplicate
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}, {Name: "provider_name"}, {Name: "csp_image_name"}},
			UpdateAll: true,
		}).CreateInBatches(&batch, len(batch))

		if result.Error != nil {
			tx.Rollback()

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
				continue
			}

			log.Error().Err(result.Error).Msg("Error inserting images in bulk")
			return result.Error
		}

		if err := tx.Commit().Error; err != nil {
			log.Error().Err(err).Msg("Failed to commit transaction")
			return err
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
				err := fmt.Errorf("The image " + u.Name + " already exists.")
				return content, err
			}
		}
		if err != nil {
			err := fmt.Errorf("Failed to check the existence of the image " + u.Name + ".")
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
			err := fmt.Errorf("The image " + content.Name + " already exists.")
			return model.ImageInfo{}, err
		}
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the image " + content.Name + ".")
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

	var callResult model.SpiderImageList
	client := resty.New()
	client.SetTimeout(100 * time.Minute)

	url := model.SpiderRestUrl + "/vmimage"
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfigName

	err := clientManager.ExecuteHttpRequest(
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

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	apiUrl := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

	err := clientManager.ExecuteHttpRequest(
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

		statusErr := clientManager.ExecuteHttpRequest(
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
func FetchImagesForConnConfig(connConfig string, nsId string) (imageCount uint, err error) {
	log.Debug().Msg("FetchImages: " + connConfig)

	spiderImageList, err := LookupImageList(connConfig)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	// Pre-allocate slice with known capacity to reduce memory allocations
	tmpImageList := make([]model.ImageInfo, 0, len(spiderImageList.Image))

	// Process images and clean up memory immediately
	for i := range spiderImageList.Image {
		spiderImage := spiderImageList.Image[i]

		if spiderImage.ImageStatus == model.ImageUnavailable {
			// log.Debug().Msgf("Skipping image in the unavailable status: %s (%s)", spiderImage.IId.NameId, connConfig)

			// Clear the processed item immediately
			spiderImageList.Image[i] = model.SpiderImageInfo{}
			continue
		}

		tumblebugImage, err := ConvertSpiderImageToTumblebugImage(nsId, connConfig, spiderImage)
		if err != nil {
			log.Error().Err(err).Msg("")
			// Clean up before returning error
			spiderImageList.Image = nil
			tmpImageList = nil
			return 0, err
		}

		imageCount++
		tmpImageList = append(tmpImageList, tumblebugImage)

		// Clear the processed spider image immediately to free memory
		spiderImageList.Image[i] = model.SpiderImageInfo{}
	}

	// Release the original spider image list immediately after processing
	spiderImageList.Image = nil
	spiderImageList = model.SpiderImageList{}

	// Perform bulk registration
	if len(tmpImageList) > 0 {
		err = RegisterImageWithInfoInBulk(tmpImageList)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to register images in bulk for %s", connConfig)
			// Clean up before returning error
			tmpImageList = nil
			return 0, err
		}
		log.Info().Msgf("Successfully registered %d images for connection %s", len(tmpImageList), connConfig)
	}

	// Clear the temporary image list after successful registration
	tmpImageList = nil

	// Force garbage collection hint for large datasets
	if imageCount > 100 {
		runtime.GC()
	}

	// log.Debug().Msgf("Memory cleanup completed for connection %s", connConfig)
	return imageCount, nil
}

// ConnectionImageResult is the result of fetching images for a single connection
type ConnectionImageResult struct {
	ConnName    string    `json:"connName"`
	Provider    string    `json:"provider"`
	Region      string    `json:"region"`
	ImageCount  int       `json:"imageCount"`
	StartTime   time.Time `json:"startTime"`
	ElapsedTime string    `json:"elapsedTime"`
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"errorMsg,omitempty"`
}

// FetchImagesAsyncResult is the result of the most recent fetch images operation
type FetchImagesAsyncResult struct {
	NamespaceID      string                  `json:"namespaceId"`
	TotalRegions     int                     `json:"totalRegions"`
	FetchOption      model.ImageFetchOption  `json:"fetchOption"`
	InProgress       bool                    `json:"inProgress"`
	RegisteredImages int                     `json:"registeredImages"`
	SucceedRegions   int                     `json:"succeedRegions"`
	FailedRegions    int                     `json:"failedRegions"`
	StartTime        time.Time               `json:"startTime"`
	ElapsedTime      string                  `json:"elapsedTime"`
	ResultInDetail   []ConnectionImageResult `json:"resultInDetail"`
}

// lastFetchResult stores the result of the most recent fetch images operation
var lastFetchResult struct {
	sync.RWMutex
	Result map[string]*FetchImagesAsyncResult
}

func init() {
	lastFetchResult.Result = make(map[string]*FetchImagesAsyncResult)
}

func updateFetchImagesProgress(nsId string, result *FetchImagesAsyncResult) {
	lastFetchResult.Lock()
	lastFetchResult.Result[nsId] = result
	lastFetchResult.Unlock()
}

// isImageFetchInProgress checks if there's an ongoing image fetch operation for the given namespace
func isImageFetchInProgress(nsId string) bool {
	lastFetchResult.RLock()
	defer lastFetchResult.RUnlock()

	result, exists := lastFetchResult.Result[nsId]
	if exists && result != nil && result.InProgress {
		return true
	}
	return false
}

// Common internal function for fetching images that can be used by both sync and async versions
func fetchImagesForAllConnConfigsInternal(nsId string, option *model.ImageFetchOption, result *FetchImagesAsyncResult) (*FetchImagesAsyncResult, error) {
	// Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		return nil, err
	}

	// Initialize fetch options
	if option == nil {
		option = &model.ImageFetchOption{}
	}

	// Set default parallel connections per provider if not specified
	parallelConnPerProvider := 1 // Default: sequential execution

	log.Info().Msgf("[%s] Starting image fetch operation", nsId)

	// Get all connection configs
	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msgf("[%s] Failed to get connection configs", nsId)
		return nil, err
	}

	// Initialize result object
	result.TotalRegions = len(connConfigs.Connectionconfig)
	result.FetchOption = *option
	result.ResultInDetail = make([]ConnectionImageResult, 0, len(connConfigs.Connectionconfig))

	updateFetchImagesProgress(nsId, result)

	// Group connection configs by provider
	providerConnMap := make(map[string][]model.ConnConfig)
	for _, connConfig := range connConfigs.Connectionconfig {
		provider := connConfig.ProviderName

		// Skip excluded providers
		if slices.Contains(option.ExcludedProviders, provider) {
			log.Debug().Msgf("[%s] Skipping excluded provider: %s", nsId, provider)
			continue
		}

		providerConnMap[provider] = append(providerConnMap[provider], connConfig)
	}

	log.Info().Msgf("[%s] Grouped connections by provider: %d providers",
		nsId, len(providerConnMap))

	// Channel to collect results from all goroutines
	resultChan := make(chan ConnectionImageResult, len(connConfigs.Connectionconfig))
	var wg sync.WaitGroup

	// Create a goroutine for each provider
	for provider, connConfigList := range providerConnMap {
		wg.Add(1)
		go func(provider string, connConfigList []model.ConnConfig) {
			defer wg.Done()
			log.Info().Msgf("[%s] Processing provider %s with %d connections",
				nsId, provider, len(connConfigList))

			// Adjust parallel connections for specific providers
			providerParallelConn := parallelConnPerProvider
			if provider == csp.AWS {
				providerParallelConn = 3 // to handle more parallel connections
			}

			// Set up semaphore for controlled parallelism
			semaphore := make(chan struct{}, providerParallelConn)

			var providerWg sync.WaitGroup
			regionAgnosticProcessed := false

			// Process connections of this provider with controlled parallelism
			for i, connConfig := range connConfigList {
				// Check if the provider is region-agnostic
				if slices.Contains(option.RegionAgnosticProviders, provider) {
					if regionAgnosticProcessed {
						log.Debug().Msgf("[%s] Skipping region for provider %s (%d/%d)",
							nsId, provider, i+1, len(connConfigList))
						continue
					}
					regionAgnosticProcessed = true
				}

				// Acquire semaphore to limit concurrent connections
				semaphore <- struct{}{}

				providerWg.Add(1)
				go func(connConfig model.ConnConfig, index int) {
					defer providerWg.Done()
					defer func() { <-semaphore }()

					connName := connConfig.ConfigName
					region := connConfig.RegionZoneInfo.AssignedRegion

					if slices.Contains(option.RegionAgnosticProviders, provider) {
						region = model.StrCommon
					}

					// Initialize connection result
					connResult := ConnectionImageResult{
						ConnName:  connName,
						Provider:  provider,
						Region:    region,
						StartTime: time.Now(),
						Success:   false,
					}

					log.Info().Msgf("[%s][Provider-%s][Conn-%d] Processing connection %s (%s/%s)",
						nsId, provider, index, connName, provider, region)

					// Set timeout for this connection
					timeout := 110 * time.Minute
					ctx, cancel := context.WithTimeout(context.Background(), timeout)

					// Process images for this connection
					doneChan := make(chan struct{})
					var imageCount int
					var fetchErr error

					// Fetch images in a separate goroutine to handle timeout
					go func() {
						defer close(doneChan)
						count, err := FetchImagesForConnConfig(connName, nsId)
						imageCount = int(count)
						fetchErr = err
					}()

					// Wait for completion or timeout
					select {
					case <-ctx.Done():
						// Timeout occurred
						connResult.Success = false
						connResult.ErrorMsg = "Operation timed out after " + timeout.String()
						log.Warn().Msgf("[%s][Provider-%s][Conn-%d] Connection %s timed out",
							nsId, provider, index, connName)
					case <-doneChan:
						// Process completed
						if fetchErr != nil {
							connResult.Success = false
							connResult.ErrorMsg = fetchErr.Error()
							log.Error().Err(fetchErr).Msgf("[%s][Provider-%s][Conn-%d] Failed to fetch images for %s",
								nsId, provider, index, connName)
						} else {
							connResult.Success = true
							connResult.ImageCount = imageCount
							log.Info().Msgf("[%s][Provider-%s][Conn-%d] Successfully fetched %d images from %s",
								nsId, provider, index, imageCount, connName)
						}
					}

					// Clean up and finalize result
					cancel()
					endTime := time.Now()
					connResult.ElapsedTime = endTime.Sub(connResult.StartTime).String()
					resultChan <- connResult
				}(connConfig, i)
			}

			providerWg.Wait()
			log.Info().Msgf("[%s] Completed processing all connections for provider %s",
				nsId, provider)

		}(provider, connConfigList)
	}

	// Close result channel when all providers are processed
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from all connections
	for connResult := range resultChan {
		result.ResultInDetail = append(result.ResultInDetail, connResult)

		if connResult.Success {
			result.SucceedRegions++
			result.RegisteredImages += connResult.ImageCount
		} else {
			result.FailedRegions++
		}
	}

	// Finalize result
	endTime := time.Now()
	result.ElapsedTime = endTime.Sub(result.StartTime).String()
	result.InProgress = false
	updateFetchImagesProgress(nsId, result)

	// Log provider statistics
	providerStats := make(map[string]struct {
		Count      int
		Success    int
		Failed     int
		ImageCount int
	})

	for _, connResult := range result.ResultInDetail {
		stats := providerStats[connResult.Provider]
		stats.Count++
		if connResult.Success {
			stats.Success++
			stats.ImageCount += connResult.ImageCount
		} else {
			stats.Failed++
		}
		providerStats[connResult.Provider] = stats
	}

	for provider, stats := range providerStats {
		log.Info().Msgf("[%s] Provider %s: %d connections (%d success, %d failed), %d images",
			nsId, provider, stats.Count, stats.Success, stats.Failed, stats.ImageCount)
	}

	log.Info().Msgf("[%s] Image fetch completed: %d images from %d/%d connections (took %s)",
		nsId, result.RegisteredImages, result.SucceedRegions,
		result.SucceedRegions+result.FailedRegions, result.ElapsedTime)

	return result, nil
}

// FetchImagesForAllConnConfigsAsync starts fetching images in background with provider-based grouping
func FetchImagesForAllConnConfigsAsync(nsId string, option *model.ImageFetchOption) error {
	// Check if there's already an operation in progress
	if isImageFetchInProgress(nsId) {
		return fmt.Errorf("an image fetch operation is already in progress")
	}

	result := &FetchImagesAsyncResult{
		NamespaceID: nsId,
		StartTime:   time.Now(),
		InProgress:  true,
	}
	updateFetchImagesProgress(nsId, result)

	// Process asynchronously
	go func() {
		result, err := fetchImagesForAllConnConfigsInternal(nsId, option, result)
		if err != nil {
			log.Error().Err(err).Msgf("[%s] Failed to fetch images asynchronously", nsId)
			result.InProgress = false
			result.ElapsedTime = time.Since(result.StartTime).String()
			updateFetchImagesProgress(nsId, result)
			return
		}
		log.Info().Msgf("[%s] Async image fetch operation completed and result saved", nsId)
	}()

	return nil
}

// FetchImagesForAllConnConfigs fetches images synchronously for all connection configs
func FetchImagesForAllConnConfigs(nsId string, option *model.ImageFetchOption) (*FetchImagesAsyncResult, error) {
	// Check if there's already an operation in progress
	if isImageFetchInProgress(nsId) {
		return nil, fmt.Errorf("an image fetch operation is already in progress")
	}
	result := &FetchImagesAsyncResult{
		NamespaceID: nsId,
		StartTime:   time.Now(),
		InProgress:  true,
	}
	updateFetchImagesProgress(nsId, result)

	// Direct call to internal function and wait for completion
	result, err := fetchImagesForAllConnConfigsInternal(nsId, option, result)
	if err != nil {
		log.Error().Err(err).Msgf("[%s] Failed to fetch images synchronously", nsId)
		result.InProgress = false
		result.ElapsedTime = time.Since(result.StartTime).String()
		updateFetchImagesProgress(nsId, result)
		return nil, err
	}

	return result, nil
}

// GetFetchImagesAsyncResult returns the result of the most recent fetch images operation
func GetFetchImagesAsyncResult(nsId string) (*FetchImagesAsyncResult, error) {
	lastFetchResult.RLock()
	defer lastFetchResult.RUnlock()

	result, exists := lastFetchResult.Result[nsId]
	result.ElapsedTime = time.Since(result.StartTime).String()
	if !exists {
		return nil, fmt.Errorf("No fetch images result found for namespace %s", nsId)
	}

	return result, nil
}

// createBasicImageInfoFromCSV creates a basic ImageInfo structure from CSV data
func createBasicImageInfoFromCSV(nsId, providerName, regionName, cspImageName, connectionName, osType, description, infraType string) model.ImageInfo {
	imageInfo := model.ImageInfo{
		ResourceType:   model.StrImage,
		Id:             cspImageName,
		Name:           cspImageName,
		Uid:            common.GenUid(),
		Namespace:      nsId,
		ConnectionName: connectionName,
		ProviderName:   providerName,
		CspImageName:   cspImageName,
		OSType:         osType,
		Description:    description,
		SystemLabel:    model.StrFromAssets,
		FetchedTime:    time.Now().Format("2006.01.02 15:04:05 Mon"),
		RegionList:     []string{},
	}

	// Set region information
	if strings.EqualFold(regionName, model.StrCommon) {
		imageInfo.RegionList = append(imageInfo.RegionList, model.StrCommon)
	} else {
		imageInfo.RegionList = append(imageInfo.RegionList, regionName)
	}

	// Set infra type
	imageInfo.InfraType = expandInfraType(infraType)

	return imageInfo
}

// enrichImageInfoFromCSP enriches ImageInfo with additional details from CSP lookup
func enrichImageInfoFromCSP(imageInfo *model.ImageInfo, imageReq model.ImageReq, regionName string, connectionList model.ConnConfigList) bool {
	// Try to get additional details from CSP lookup (optional)
	if strings.EqualFold(regionName, model.StrCommon) {
		// If region is common, try to lookup from any region for this provider
		for _, connConfig := range connectionList.Connectionconfig {
			if strings.EqualFold(connConfig.ProviderName, imageInfo.ProviderName) {
				lookupReq := imageReq
				lookupReq.ConnectionName = imageInfo.ProviderName + "-" + connConfig.RegionDetail.RegionName

				if detailedInfo, err := GetImageInfoFromLookupImage(model.SystemCommonNs, lookupReq); err == nil {
					mergeCSPDetails(imageInfo, &detailedInfo)
					log.Info().Msgf("Successfully looked up image details from CSP: %s", imageReq.CspImageName)
					return true
				}
			}
		}
	} else {
		if detailedInfo, err := GetImageInfoFromLookupImage(model.SystemCommonNs, imageReq); err == nil {
			mergeCSPDetails(imageInfo, &detailedInfo)
			log.Info().Msgf("Successfully looked up image details from CSP: %s", imageReq.CspImageName)
			return true
		}
	}

	log.Info().Msgf("CSP lookup failed, but will register with CSV data only: Provider: %s, Region: %s, CspImageName: %s",
		imageInfo.ProviderName, regionName, imageReq.CspImageName)
	return false
}

// mergeCSPDetails merges CSP lookup details into the base ImageInfo
func mergeCSPDetails(target *model.ImageInfo, source *model.ImageInfo) {
	target.OSArchitecture = source.OSArchitecture
	target.OSPlatform = source.OSPlatform
	target.OSDistribution = source.OSDistribution
	target.IsBasicImage = source.IsBasicImage
	target.OSDiskType = source.OSDiskType
	target.OSDiskSizeGB = source.OSDiskSizeGB
	target.CreationDate = source.CreationDate
	target.ImageStatus = source.ImageStatus
	target.IsGPUImage = source.IsGPUImage
	target.IsKubernetesImage = source.IsKubernetesImage
	target.Details = source.Details
}

// updateExistingImageFromCSV updates existing image with CSV data
func updateExistingImageFromCSV(existingImage model.ImageInfo, osType, description, infraType string) model.ImageInfo {
	existingImage.OSType = osType
	existingImage.Description = description
	existingImage.InfraType = expandInfraType(infraType)
	existingImage.SystemLabel = model.StrFromAssets
	return existingImage
}

// UpdateImagesFromAsset updates image information based on cloudimage.csv asset file
func UpdateImagesFromAsset(nsId string) (*FetchImagesAsyncResult, error) {
	if nsId == "" {
		nsId = model.SystemCommonNs
	}

	startTime := time.Now()
	result := &FetchImagesAsyncResult{
		NamespaceID: nsId,
		StartTime:   startTime,
		InProgress:  true,
	}
	updateFetchImagesProgress(nsId, result)

	// Get all connection configs for provider and region information
	connectionList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot GetConnConfigList")
		result.InProgress = false
		result.ElapsedTime = time.Since(startTime).String()
		updateFetchImagesProgress(nsId, result)
		return result, err
	}

	// Map to store valid connections by provider and region
	validConnectionMap := make(map[string]model.ConnConfig)
	for _, connConfig := range connectionList.Connectionconfig {
		key := strings.ToLower(connConfig.ProviderName) + "-" + strings.ToLower(connConfig.RegionDetail.RegionName)
		validConnectionMap[key] = connConfig
	}

	// Open cloudimage.csv file
	csvPath := common.GetAssetsFilePath("cloudimage.csv")
	file, fileErr := os.Open(csvPath)
	if fileErr != nil {
		log.Error().
			Err(fileErr).
			Str("attempted_path", csvPath).
			Msg("Failed to open cloudimage.csv")
		result.InProgress = false
		result.ElapsedTime = time.Since(startTime).String()
		updateFetchImagesProgress(nsId, result)
		return result, fmt.Errorf("failed to open cloudimage.csv at %s: %w", csvPath, fileErr)
	}
	defer file.Close()

	// Read CSV file
	rdr := csv.NewReader(bufio.NewReader(file))
	rowsImg, err := rdr.ReadAll()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read cloudimage.csv")
		result.InProgress = false
		result.ElapsedTime = time.Since(startTime).String()
		updateFetchImagesProgress(nsId, result)
		return result, err
	}

	tmpImageList := []model.ImageInfo{}
	var wait sync.WaitGroup
	var mutex sync.Mutex

	// // waitSpecImg.Add(1)
	// go func(rowsImg [][]string) {
	// 	// defer waitSpecImg.Done()
	lenImages := len(rowsImg[1:])
	for i, row := range rowsImg[1:] {

		imageReqTmp := model.ImageReq{}
		// row0: ProviderName
		// row1: regionName
		// row2: cspResourceId
		// row3: OsType
		// row4: description
		// row5: supportedInstance
		// row6: infraType
		providerName := strings.ToLower(row[0])
		regionName := strings.ToLower(row[1])
		imageReqTmp.CspImageName = row[2]
		osType := row[3]
		description := row[4]
		infraType := strings.ToLower(row[6])

		regionNameForConnection := regionName
		if regionName == "all" {
			regionName = model.StrCommon
		}
		imageReqTmp.ConnectionName = providerName + "-" + regionNameForConnection

		log.Trace().Msgf("[%d] register Common Image: %s", i, imageReqTmp.Name)

		existingImage, err := GetImageByPrimaryKey(nsId, providerName, imageReqTmp.CspImageName)
		if err != nil {
			wait.Add(1)
			go func(i int, row []string, lenImages int) {
				defer wait.Done()

				// RandomSleep for safe parallel executions
				common.RandomSleep(0, lenImages/8*1000)
				log.Info().Msgf("New image from CSV, Provider: %s, Region: %s, CspImageName: %s", providerName, regionName, imageReqTmp.CspImageName)

				// Create a basic image info from CSV data
				tmpImageInfo := createBasicImageInfoFromCSV(nsId, providerName, regionName, imageReqTmp.CspImageName,
					imageReqTmp.ConnectionName, osType, description, infraType)

				// Try to enrich with CSP lookup (optional)
				enrichImageInfoFromCSP(&tmpImageInfo, imageReqTmp, regionName, connectionList)

				// Add to list regardless of CSP lookup success
				mutex.Lock()
				tmpImageList = append(tmpImageList, tmpImageInfo)
				mutex.Unlock()

			}(i, row, lenImages)
		} else {
			// Update existing image with new information from the asset file
			tmpImageInfo := updateExistingImageFromCSV(existingImage, osType, description, infraType)

			mutex.Lock()
			tmpImageList = append(tmpImageList, tmpImageInfo)
			mutex.Unlock()
		}

	}
	wait.Wait()
	// }(rowsImg)

	log.Info().Msgf("tmpImageList %d", len(tmpImageList))

	err = RegisterImageWithInfoInBulk(tmpImageList)
	if err != nil {
		log.Info().Err(err).Msg("RegisterImage WithInfo failed")
	}

	elapsedUpdateImg := time.Since(startTime)

	log.Info().Msgf("Updated the registered Images according to the asset file. Elapsed [%s]", elapsedUpdateImg)

	result.InProgress = false
	result.ElapsedTime = time.Since(startTime).String()
	updateFetchImagesProgress(nsId, result)
	return result, nil
}

// SearchImage returns a list of images based on the search criteria
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
		sqlQuery = sqlQuery.Where("LOWER(os_architecture) = ?", strings.ToLower(string(req.OSArchitecture)))
	}

	if req.IsGPUImage != nil {
		sqlQuery = sqlQuery.Where("is_gpu_image = ?", *req.IsGPUImage)
	}

	if req.IsKubernetesImage != nil {
		sqlQuery = sqlQuery.Where("is_kubernetes_image = ?", *req.IsKubernetesImage)
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

	// Filter duplicate images with same OS details but different dates, keeping only the latest 2
	allowedDuplicationCount := 2

	if len(images) > 0 {
		filteredImages := filterDuplicateImagesByDate(images, allowedDuplicationCount)
		log.Info().Msgf("SearchImage: Filtered %d duplicate images, %d images remaining",
			len(images)-len(filteredImages), len(filteredImages))
		images = filteredImages
		cnt = len(images)
	}

	// Sort images by OS disctibution in descending order
	sort.Slice(images, func(i, j int) bool {
		return images[i].OSDistribution > images[j].OSDistribution
	})

	// Additional filtering: Keep only the top image for each group with same base distribution text
	if len(images) > 0 {
		finalImages := filterDuplicateImagesByVersion(images)
		log.Info().Msgf("SearchImage: Additional filtering removed %d images, %d images remaining",
			len(images)-len(finalImages), len(finalImages))
		images = finalImages
		cnt = len(images)
	}

	// Apply CSP-specific image filtering based on spec compatibility
	if specInfo != nil && len(images) > 0 {
		filteredImages := applyCspSpecificImageFiltering(images, *specInfo)
		if len(filteredImages) != len(images) {
			log.Info().Msgf("SearchImage: CSP-specific filtering removed %d images, %d images remaining for provider %s",
				len(images)-len(filteredImages), len(filteredImages), specInfo.ProviderName)
			images = filteredImages
			cnt = len(images)
		}
	}

	// Move basic images to the front using partition approach (O(n) instead of O(n log n))
	if len(images) > 0 {
		basicIndex := 0
		for i := 0; i < len(images); i++ {
			if images[i].IsBasicImage {
				if i != basicIndex {
					// Swap basic image to the front
					images[basicIndex], images[i] = images[i], images[basicIndex]
				}
				basicIndex++
			}
		}
	}

	return images, cnt, nil
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

// filterDuplicateImagesByVersion keeps only the first (top) image for each group with same base distribution text
func filterDuplicateImagesByVersion(images []model.ImageInfo) []model.ImageInfo {
	if len(images) == 0 {
		return images
	}

	seen := make(map[string]bool)
	var result []model.ImageInfo

	for _, img := range images {
		// Create grouping key based on OSType, OSArchitecture, OSPlatform, and base distribution text
		baseDistribution := removeNumbersFromDistribution(img.OSDistribution)
		key := fmt.Sprintf("%s|%s|%s|%s",
			strings.ToLower(img.OSType),
			strings.ToLower(string(img.OSArchitecture)),
			strings.ToLower(string(img.OSPlatform)),
			strings.ToLower(baseDistribution))

		// Keep only the first image for each unique key (since images are already sorted)
		if !seen[key] {
			seen[key] = true
			result = append(result, img)
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

		// If dates are equal, compare by creation date or name
		if dateI.Equal(dateJ) {
			// Parse creation date if available
			if images[i].CreationDate != "" && images[j].CreationDate != "" {
				timeI, errI := time.Parse("2006-01-02T15:04:05.000Z", images[i].CreationDate)
				timeJ, errJ := time.Parse("2006-01-02T15:04:05.000Z", images[j].CreationDate)
				if errI == nil && errJ == nil {
					return timeI.After(timeJ)
				}
			}
			// Fallback to name comparison for stable sorting
			return images[i].Name > images[j].Name
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
			err := fmt.Errorf("The image " + imageId + " does not exist.")
			return temp, err
		}

		tempInterface, err := GetResource(nsId, resourceType, imageId)
		if err != nil {
			err := fmt.Errorf("Failed to get the image " + imageId + ".")
			return temp, err
		}
		asIsImage := model.ImageInfo{}
		err = common.CopySrcToDest(&tempInterface, &asIsImage)
		if err != nil {
			err := fmt.Errorf("Failed to CopySrcToDest() " + imageId + ".")
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
func GetImage(nsId string, cspImageName string) (model.ImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.ImageInfo{}, err
	}

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
			log.Info().Err(result.Error).Msgf("Cannot get image %s by ID from %s", cspImageName, model.SystemCommonNs)
		} else {
			return image, nil
		}

		// 2) Check if the image is a registered image in the given namespace
		// ex: img-487zeit5

		result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(csp_image_name) = ?", nsId, cspImageName).First(&image)
		if result.Error != nil {
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
func applyCspSpecificImageFiltering(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	switch strings.ToLower(specInfo.ProviderName) {
	case csp.NCP:
		return filterImagesByCorrespondingIds(images, specInfo)
	// Add more CSP-specific filtering logic here as needed
	// case "aws":
	//     return filterImagesByHypervisor(images, specInfo)
	// case "azure":
	//     return filterImagesByGeneration(images, specInfo)
	default:
		// No specific filtering for other CSPs
		return images
	}
}

// filterImagesByCorrespondingIds filters images based on CorrespondingImageIds from spec details
func filterImagesByCorrespondingIds(images []model.ImageInfo, specInfo model.SpecInfo) []model.ImageInfo {
	// Find CorrespondingImageIds from spec details
	correspondingIds := extractCorrespondingImageIds(specInfo.Details)
	if len(correspondingIds) == 0 {
		log.Warn().Msgf("No CorrespondingImageIds found in spec %s for provider %s", specInfo.Id, specInfo.ProviderName)
		return images
	}

	// Convert to map for efficient lookup
	validImageIds := make(map[string]bool)
	for _, id := range correspondingIds {
		validImageIds[strings.TrimSpace(id)] = true
	}

	// Filter images based on cspImageName matching
	var filteredImages []model.ImageInfo
	for _, image := range images {
		if validImageIds[image.CspImageName] {
			filteredImages = append(filteredImages, image)
		}
	}

	log.Info().Msgf("CorrespondingIds filtering: %d corresponding image IDs found, filtered from %d to %d images for provider %s",
		len(correspondingIds), len(images), len(filteredImages), specInfo.ProviderName)

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
