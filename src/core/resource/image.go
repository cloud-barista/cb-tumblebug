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

// TbImageReqStructLevelValidation func is for Validation
func TbImageReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbImageReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// ConvertSpiderImageToTumblebugImage accepts an Spider image object, converts to and returns an TB image object
func ConvertSpiderImageToTumblebugImage(nsId, connConfig string, spiderImage model.SpiderImageInfo) (model.TbImageInfo, error) {

	regionAgnosticProviders := []string{"azure", "gcp", "tencent"}

	if spiderImage.IId.NameId == "" {
		err := fmt.Errorf("ConvertSpiderImageToTumblebugImage failed; spiderImage.IId.NameId == EmptyString")
		emptyTumblebugImage := model.TbImageInfo{}
		return emptyTumblebugImage, err
	}

	connectionConfig, err := common.GetConnConfig(connConfig)
	if err != nil {
		err = fmt.Errorf("cannot retrieve ConnectionConfig %s: %v", connectionConfig.ConfigName, err)
		log.Error().Err(err).Msg("")
		return model.TbImageInfo{}, err
	}

	cspImageName := spiderImage.IId.NameId
	providerName := connectionConfig.ProviderName
	currentRegion := connectionConfig.RegionDetail.RegionName
	if slices.Contains(regionAgnosticProviders, providerName) {
		// For region-agnostic providers, use common region
		currentRegion = model.StrCommon
	}

	// Create new image instance
	tumblebugImage := model.TbImageInfo{}

	// Generate ID for backward compatibility
	tumblebugImageId := GetProviderRegionZoneResourceKey(providerName, "", "", cspImageName)

	// Set basic fields
	tumblebugImage.Id = tumblebugImageId
	tumblebugImage.Name = tumblebugImageId
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
		if providerName == csp.KTCloud {
			tumblebugImage.OSArchitecture = model.X86_64
		}
		// For IBM Cloud, we set S390X if the architecture is not specified
		if providerName == csp.IBM {
			tumblebugImage.OSArchitecture = model.S390X
		}
	}
	tumblebugImage.OSPlatform = spiderImage.OSPlatform
	tumblebugImage.OSDistribution = spiderImage.OSDistribution
	tumblebugImage.OSDiskType = spiderImage.OSDiskType
	tumblebugImage.OSDiskSizeGB, _ = strconv.ParseFloat(spiderImage.OSDiskSizeGB, 64)

	tumblebugImage.Details = spiderImage.KeyValueList

	return tumblebugImage, nil
}

// GetImageInfoFromLookupImage
func GetImageInfoFromLookupImage(nsId string, u model.TbImageReq) (model.TbImageInfo, error) {
	content := model.TbImageInfo{}
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

// RegisterImageWithInfoInBulk register a list of images in bulk
func RegisterImageWithInfoInBulk(imageList []model.TbImageInfo) error {
	// Advanced deduplication logic with region merging
	uniqueImages := make(map[string]model.TbImageInfo)
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
	dedupedImageList := make([]model.TbImageInfo, 0, len(uniqueImages))

	for _, img := range uniqueImages {
		// Check if image exists in database
		var dbImage model.TbImageInfo
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
					altTx.Raw("SELECT EXISTS(SELECT 1 FROM tb_image_infos WHERE namespace = ? AND provider_name = ? AND csp_image_name = ?)",
						img.Namespace, img.ProviderName, img.CspImageName).Scan(&exists)

					if exists {
						// Update - using composite key
						if err := altTx.Model(&model.TbImageInfo{}).
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
    DELETE FROM tb_image_infos
    WHERE ctid NOT IN (
        SELECT MIN(ctid)
        FROM tb_image_infos
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
func RegisterImageWithId(nsId string, u *model.TbImageReq, update bool, RDBonly bool) (model.TbImageInfo, error) {

	content := model.TbImageInfo{}

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
			updateResult := model.ORM.Model(&model.TbImageInfo{}).Where("namespace = ? AND id = ?", content.Namespace, content.Id).Updates(content)
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
func RegisterImageWithInfo(nsId string, content *model.TbImageInfo, update bool) (model.TbImageInfo, error) {

	resourceType := model.StrImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbImageInfo{}, err
	}
	// err = common.CheckString(content.Name)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return model.TbImageInfo{}, err
	// }
	check, err := CheckResource(nsId, resourceType, content.Name)

	if !update {
		if check {
			err := fmt.Errorf("The image " + content.Name + " already exists.")
			return model.TbImageInfo{}, err
		}
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the image " + content.Name + ".")
		return model.TbImageInfo{}, err
	}

	content.Namespace = nsId
	//content.Id = common.GenUid()
	content.Id = content.Name

	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbImageInfo{}, err
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
func LookupImage(connConfig string, imageId string) (model.SpiderImageInfo, error) {

	if connConfig == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("LookupImage() called with empty connConfig.")
		log.Error().Err(err).Msg("")
		return content, err
	} else if imageId == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("LookupImage() called with empty imageId.")
		log.Error().Err(err).Msg("")
		return content, err
	}

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	url := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

	err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("")
		return callResult, err
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

	tmpImageList := []model.TbImageInfo{}

	for _, spiderImage := range spiderImageList.Image {
		if spiderImage.ImageStatus == model.ImageUnavailable {
			log.Debug().Msgf("Skipping image in the unavailable status: %s (%s)", spiderImage.IId.NameId, connConfig)
			continue
		}

		tumblebugImage, err := ConvertSpiderImageToTumblebugImage(nsId, connConfig, spiderImage)
		if err != nil {
			log.Error().Err(err).Msg("")
			return 0, err
		}

		imageCount++

		tmpImageList = append(tmpImageList, tumblebugImage)
	}

	if len(tmpImageList) > 0 {
		err = RegisterImageWithInfoInBulk(tmpImageList)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to register images in bulk for %s", connConfig)
			return 0, err
		}
		log.Info().Msgf("Successfully registered %d images for connection %s", len(tmpImageList), connConfig)
	}

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
			log.Info().Msgf("[%s] Skipping excluded provider: %s", nsId, provider)
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
				providerParallelConn = 15 // to handle more parallel connections
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
						log.Warn().Msgf("[%s] Skipping region for provider %s (%d/%d)",
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
	file, fileErr := os.Open("../assets/cloudimage.csv")
	if fileErr != nil {
		log.Error().Err(fileErr).Msg("Failed to open cloudimage.csv")
		result.InProgress = false
		result.ElapsedTime = time.Since(startTime).String()
		updateFetchImagesProgress(nsId, result)
		return result, fileErr
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

	tmpImageList := []model.TbImageInfo{}
	var wait sync.WaitGroup
	var mutex sync.Mutex

	// // waitSpecImg.Add(1)
	// go func(rowsImg [][]string) {
	// 	// defer waitSpecImg.Done()
	lenImages := len(rowsImg[1:])
	for i, row := range rowsImg[1:] {

		imageReqTmp := model.TbImageReq{}
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
			// fmt.Printf("[%d] i, row := range rowsImg[1:] %s\n", i, row)
			// goroutine
			go func(i int, row []string, lenImages int) {
				defer wait.Done()

				// RandomSleep for safe parallel executions
				common.RandomSleep(0, lenImages/8)
				log.Info().Msgf("Failed to get existing image, Provider: %s, Region: %s, CspImageName: %s Error: %s", providerName, regionName, imageReqTmp.CspImageName, err.Error())

				if regionName == model.StrCommon {
					// If region is common, check all regions for the provider
					for _, connConfig := range connectionList.Connectionconfig {
						if connConfig.ProviderName == providerName {
							regionNameForConnection = connConfig.RegionDetail.RegionName
							imageReqTmp.ConnectionName = providerName + "-" + regionNameForConnection

							tmpImageInfo, err1 := GetImageInfoFromLookupImage(model.SystemCommonNs, imageReqTmp)
							if err1 != nil {
								log.Info().Msgf("lookup failure, Provider: %s, Region: %s, CspImageName: %s Error: %s", providerName, regionName, imageReqTmp.CspImageName, err1.Error())

							} else {
								// Update registered image object with OsType info
								expandedInfraType := expandInfraType(infraType)

								tmpImageInfo.OSType = osType
								tmpImageInfo.Description = description
								tmpImageInfo.InfraType = expandedInfraType
								tmpImageInfo.SystemLabel = model.StrFromAssets

								mutex.Lock()
								tmpImageList = append(tmpImageList, tmpImageInfo)
								mutex.Unlock()
								break // Exit loop after first successful lookup
							}

						}
					}
				} else {
					tmpImageInfo, err1 := GetImageInfoFromLookupImage(model.SystemCommonNs, imageReqTmp)
					if err1 != nil {
						log.Info().Msgf("lookup failure, Provider: %s, Region: %s, CspImageName: %s Error: %s", providerName, regionName, imageReqTmp.CspImageName, err1.Error())

					} else {
						// Update registered image object with OsType info
						expandedInfraType := expandInfraType(infraType)

						tmpImageInfo.OSType = osType
						tmpImageInfo.Description = description
						tmpImageInfo.InfraType = expandedInfraType
						tmpImageInfo.SystemLabel = model.StrFromAssets

						mutex.Lock()
						tmpImageList = append(tmpImageList, tmpImageInfo)
						mutex.Unlock()

					}
				}

			}(i, row, lenImages)
		} else {
			// Update existing image with new information from the asset file
			log.Info().Msgf("Found existing image, Provider: %s, Region: %s, CspImageName: %s", providerName, regionName, imageReqTmp.CspImageName)
			tmpImageInfo := existingImage
			// Update registered image object with OsType info
			expandedInfraType := expandInfraType(infraType)

			tmpImageInfo.OSType = osType
			tmpImageInfo.Description = description
			tmpImageInfo.InfraType = expandedInfraType
			tmpImageInfo.SystemLabel = model.StrFromAssets

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
func SearchImage(nsId, providerName, regionName, osType, osArchitecture string, isGPUImage, isKubernetesImage, isRegisteredByAsset, includeDeprecatedImage *bool, keywords ...string) ([]model.TbImageInfo, int, error) {
	err := common.CheckString(nsId)
	cnt := 0
	if err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return nil, cnt, err
	}

	var images []model.TbImageInfo
	sqlQuery := model.ORM.Where("namespace = ?", nsId)

	if providerName != "" {
		sqlQuery = sqlQuery.Where("provider_name = ?", providerName)
	}

	// regionName needs to be searched from region_list
	if regionName != "" {
		sqlQuery = sqlQuery.Where(
			model.ORM.Where("LOWER(region_list) LIKE ?", "%"+strings.ToLower(regionName)+"%").
				Or("LOWER(region_list) LIKE ?", "%"+strings.ToLower(model.StrCommon)+"%"))
	}

	if osType != "" {
		osTypeLower := strings.ToLower(osType)
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

	if osArchitecture != "" {
		sqlQuery = sqlQuery.Where("LOWER(os_architecture) = ?", strings.ToLower(osArchitecture))
	}

	if isGPUImage != nil {
		sqlQuery = sqlQuery.Where("is_gpu_image = ?", *isGPUImage)
	}

	if isKubernetesImage != nil {
		sqlQuery = sqlQuery.Where("is_kubernetes_image = ?", *isKubernetesImage)
	}

	// Check if isRegisteredByAsset is true
	// If it is true, filter by system_label = StrFromAssets
	if isRegisteredByAsset != nil {
		if *isRegisteredByAsset {
			sqlQuery = sqlQuery.Where("system_label = ?", model.StrFromAssets)
		}
	}

	// Check if includeDeprecated is nil or false
	if includeDeprecatedImage != nil {
		if !*includeDeprecatedImage {
			sqlQuery = sqlQuery.Where("image_status != ?", model.ImageDeprecated)
		}
	} else {
		sqlQuery = sqlQuery.Where("image_status != ?", model.ImageDeprecated)
	}

	if len(keywords) > 0 {
		// Build a single query to check if all keywords are included in either os_type or details
		for _, keyword := range keywords {
			keyword = strings.ToLower(keyword)
			sqlQuery = sqlQuery.Where("(LOWER(details) LIKE ?)", "%"+keyword+"%")
		}
	}

	log.Info().Msgf("SearchImage: providerName=%s, regionName=%s, osType=%s, isGPUImage=%v, isKubernetesImage=%v, isRegisteredByAsset=%v, includeDeprecatedImage=%v",
		providerName, regionName, osType, isGPUImage, isKubernetesImage, isRegisteredByAsset, includeDeprecatedImage)

	result := sqlQuery.Find(&images)
	log.Info().Msgf("SearchImage: Found %d images for namespace %s", len(images), nsId)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to retrieve images")
		return nil, cnt, result.Error
	}
	cnt = len(images)

	return images, cnt, nil
}

// SearchImageOptions returns the available options for searching images
func SearchImageOptions() (model.SearchImageRequestOptions, error) {
	var options model.SearchImageRequestOptions

	// Get distinct provider names
	if err := model.ORM.Model(&model.TbImageInfo{}).
		Distinct("provider_name").
		Order("provider_name").
		Pluck("provider_name", &options.ProviderName).Error; err != nil {
		log.Error().Err(err).Msg("Failed to get distinct provider names")
		return options, err
	}

	// Get regions (application-level processing)
	var images []model.TbImageInfo
	if err := model.ORM.Model(&model.TbImageInfo{}).
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
	if err := model.ORM.Model(&model.TbImageInfo{}).
		Where("os_type != ''").
		Distinct("os_type").
		Order("os_type").
		Pluck("os_type", &options.OSType).Error; err != nil {
		log.Error().Err(err).Msg("Failed to get distinct OS types")
		return options, err
	}

	// Get distinct OS architectures (non-empty only)
	if err := model.ORM.Model(&model.TbImageInfo{}).
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
func UpdateImage(nsId string, imageId string, fieldsToUpdate model.TbImageInfo, RDBonly bool) (model.TbImageInfo, error) {
	if !RDBonly {

		resourceType := model.StrImage
		temp := model.TbImageInfo{}
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
		asIsImage := model.TbImageInfo{}
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
	result := model.ORM.Model(&model.TbImageInfo{}).Where("namespace = ? AND id = ?", nsId, imageId).Updates(fieldsToUpdate)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("")
		return fieldsToUpdate, result.Error
	} else {
		log.Trace().Msg("SQL: Update success")
	}

	return fieldsToUpdate, nil
}

// GetImage accepts namespace Id and imageKey(Id,CspResourceName,GuestOS,...), and returns the TB image object
func GetImage(nsId string, imageKey string) (model.TbImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.TbImageInfo{}, err
	}

	log.Debug().Msg("[Get image] " + imageKey)

	// make comparison case-insensitive
	nsId = strings.ToLower(nsId)
	imageKey = strings.ToLower(imageKey)
	imageKey = strings.ReplaceAll(imageKey, " ", "")

	providerName, regionName, _, imageIdentifier, err := ResolveProviderRegionZoneResourceKey(imageKey)
	if err != nil {
		// imageKey does not include information for providerName, regionName
		image := model.TbImageInfo{Namespace: nsId, Id: imageKey}

		// 1) Check if the image is a custom image
		// ex: custom-img-487zeit5
		tempInterface, err := GetResource(nsId, model.StrCustomImage, imageKey)
		customImage := model.TbCustomImageInfo{}
		if err == nil {
			err = common.CopySrcToDest(&tempInterface, &customImage)
			if err != nil {
				log.Error().Err(err).Msg("TbCustomImageInfo CopySrcToDest error")
				return model.TbImageInfo{}, err
			}
			image.CspImageName = customImage.CspResourceName
			image.SystemLabel = model.StrCustomImage
			return image, nil
		}

		// 2) Check if the image is a registered image in the given namespace
		// ex: img-487zeit5
		image = model.TbImageInfo{Namespace: nsId, Id: imageKey}
		result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ?", nsId, imageKey).First(&image)
		if result.Error != nil {
			log.Info().Err(result.Error).Msgf("Cannot get image %s by ID from %s", imageKey, nsId)
		} else {
			return image, nil
		}

	} else {
		// imageKey includes information for providerName, regionName

		// 1) Check if the image is a registered image in the common namespace model.SystemCommonNs by ImageId
		// ex: tencent+ap-jakarta+ubuntu22.04 or tencent+ap-jakarta+img-487zeit5
		image := model.TbImageInfo{Namespace: model.SystemCommonNs, Id: imageKey}
		result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ?", model.SystemCommonNs, imageKey).First(&image)
		if result.Error != nil {
			log.Info().Err(result.Error).Msgf("Cannot get image %s by ID from %s", imageKey, model.SystemCommonNs)
		} else {
			return image, nil
		}

		// 2) Check if the image is a registered image in the common namespace model.SystemCommonNs by CspImageName
		// ex: tencent+img-487zeit5
		image, err := GetImageByPrimaryKey(model.SystemCommonNs, providerName, imageIdentifier)
		if err != nil {
			log.Info().Err(result.Error).Msgf("Cannot get image %s by CspImageName", imageIdentifier)
		} else {
			return image, nil
		}

		// 3) Check if the image is a registered image in the common namespace model.SystemCommonNs by GuestOS
		// ex: tencent+ap-jakarta+Ubuntu22.04

		//isKubernetesImage := false
		isRegisteredByAsset := true
		includeDeprecatedImage := false

		images, imageCnt, err := SearchImage(
			model.SystemCommonNs,
			providerName,
			regionName,
			imageIdentifier,
			"",
			nil,
			nil,
			&isRegisteredByAsset,
			&includeDeprecatedImage,
			"",
		)
		if err != nil || imageCnt == 0 {
			log.Info().Err(result.Error).Msgf("Failed to get image %s by OS type", imageIdentifier)
		} else {
			// Return the first image found
			return images[0], nil
		}
	}

	return model.TbImageInfo{}, fmt.Errorf("The imageKey %s not found by any of ID, CspImageName, GuestOS", imageKey)
}

// GetImageByPrimaryKey retrieves image information based on namespace, provider, and CSP image name
func GetImageByPrimaryKey(nsId string, provider string, cspImageName string) (model.TbImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.TbImageInfo{}, err
	}

	log.Debug().Msgf("[Get image] Namespace: %s, Provider: %s, CSP Image Name: %s", nsId, provider, cspImageName)

	// Convert inputs to lowercase for case-insensitive comparison
	nsId = strings.ToLower(nsId)
	provider = strings.ToLower(provider)
	cspImageName = strings.ToLower(cspImageName)

	// Query the database for the image
	var image model.TbImageInfo
	result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(provider_name) = ? AND LOWER(csp_image_name) = ?", nsId, provider, cspImageName).First(&image)
	if result.Error != nil {
		log.Debug().Err(result.Error).Msgf("Failed to retrieve image for Namespace: %s, Provider: %s, CSP Image Name: %s", nsId, provider, cspImageName)
		return model.TbImageInfo{}, result.Error
	}

	return image, nil
}

// GetImagesByRegion retrieves images based on namespace, provider, and region
func GetImagesByRegion(nsId string, provider string, region string) ([]model.TbImageInfo, error) {
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
	var images []model.TbImageInfo
	result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(provider_name) = ? AND LOWER(region_list) LIKE ?", nsId, provider, "%"+region+"%").Find(&images)
	if result.Error != nil {
		log.Error().Err(result.Error).Msgf("Failed to retrieve images for Namespace: %s, Provider: %s, Region: %s", nsId, provider, region)
		return nil, result.Error
	}

	return images, nil
}
