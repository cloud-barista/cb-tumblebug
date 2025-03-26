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
	"fmt"
	"net/url"
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

	// Create new image instance
	tumblebugImage := model.TbImageInfo{}

	// Generate ID for backward compatibility
	tumblebugImageId := GetProviderRegionZoneResourceKey(providerName, currentRegion, "", cspImageName)

	// Set basic fields
	tumblebugImage.Id = tumblebugImageId
	tumblebugImage.Name = tumblebugImageId
	tumblebugImage.Namespace = nsId
	tumblebugImage.ConnectionName = connConfig
	tumblebugImage.ProviderName = providerName

	// Set region information (array and default region)
	tumblebugImage.RegionList = make([]string, 0)
	tumblebugImage.RegionList = append(tumblebugImage.RegionList, currentRegion)

	tumblebugImage.RegionName = currentRegion

	tumblebugImage.CspImageName = spiderImage.IId.NameId
	tumblebugImage.Description = common.LookupKeyValueList(spiderImage.KeyValueList, "Description")
	tumblebugImage.CreationDate = common.LookupKeyValueList(spiderImage.KeyValueList, "CreationDate")

	// Extract OS, GPU, K8s information
	searchStr := spiderImage.IId.NameId + " " + spiderImage.OSDistribution
	tumblebugImage.GuestOS = common.ExtractOSInfo(searchStr)

	// Check if this is a GPU image
	if common.IsGPUImage(searchStr) {
		tumblebugImage.GPUReady = model.GPUReadyYes
	}

	// Check if this is a Kubernetes image
	if common.IsK8sImage(searchStr) {
		tumblebugImage.InfraType = "k8s|kubernetes|container"
	}

	// Set additional fields
	tumblebugImage.Architecture = string(spiderImage.OSArchitecture)
	tumblebugImage.Platform = string(spiderImage.OSPlatform)
	tumblebugImage.Distribution = spiderImage.OSDistribution
	tumblebugImage.RootDiskType = spiderImage.OSDiskType
	rootDiskMinSizeGB, _ := strconv.ParseFloat(spiderImage.OSDiskSizeGB, 32)
	tumblebugImage.RootDiskMinSizeGB = float32(rootDiskMinSizeGB)

	tumblebugImage.Status = string(spiderImage.ImageStatus)
	tumblebugImage.KeyValueList = spiderImage.KeyValueList
	tumblebugImage.AssociatedObjectList = make([]string, 0)

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
			log.Debug().Msgf("Found duplicate image: %s/%s/%s",
				img.Namespace, img.ProviderName, img.CspImageName)

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

			// Add RegionName to RegionList if it exists and isn't already in the list
			if img.RegionName != "" {
				regionExists := false
				for _, region := range img.RegionList {
					if region == img.RegionName {
						regionExists = true
						break
					}
				}

				if !regionExists {
					img.RegionList = append(img.RegionList, img.RegionName)
				}
			}

			uniqueImages[key] = img
		}
	}

	// Step 2: Selectively check and merge with existing images in DB
	dedupedImageList := make([]model.TbImageInfo, 0, len(uniqueImages))

	for key, img := range uniqueImages {
		// Check if image exists in database
		var dbImage model.TbImageInfo
		result := model.ORM.Where("namespace = ? AND provider_name = ? AND csp_image_name = ?",
			img.Namespace, img.ProviderName, img.CspImageName).First(&dbImage)

		if result.Error == nil {
			// Merge region information if image exists in DB
			log.Debug().Msgf("Found existing image in DB: %s/%s/%s with regions %v",
				img.Namespace, img.ProviderName, img.CspImageName, dbImage.RegionList)

			// Initialize RegionList if nil in DB image
			if dbImage.RegionList == nil {
				dbImage.RegionList = make([]string, 0)
			}

			// Merge new region information
			regionsAdded := false
			for _, newRegion := range img.RegionList {
				regionExists := slices.Contains(dbImage.RegionList, newRegion)

				if !regionExists {
					log.Debug().Msgf("Adding region %s to DB image %s",
						newRegion, key)
					dbImage.RegionList = append(dbImage.RegionList, newRegion)
					regionsAdded = true
				}
			}

			if regionsAdded {
				// Sort regions
				sort.Strings(dbImage.RegionList)

				log.Info().Msgf("Merged regions for image %s: %v",
					key, dbImage.RegionList)
			}

			dedupedImageList = append(dedupedImageList, dbImage)
		} else {
			// Add new image if not found in DB
			log.Debug().Msgf("Image not found in DB, will insert new: %s", key)
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

		log.Info().Msgf("Bulk insert/update success: %d records affected", result.RowsAffected)
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
	content.AssociatedObjectList = []string{}

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
	EndTime     time.Time `json:"endTime"`
	ElapsedTime string    `json:"elapsedTime"`
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"errorMsg,omitempty"`
}

// FetchImagesAsyncResult is the result of the most recent fetch images operation
type FetchImagesAsyncResult struct {
	NamespaceID  string                  `json:"namespaceId"`
	TotalImages  int                     `json:"totalImages"`
	SuccessCount int                     `json:"successCount"`
	FailCount    int                     `json:"failCount"`
	StartTime    time.Time               `json:"startTime"`
	EndTime      time.Time               `json:"endTime"`
	TotalTime    string                  `json:"totalTime"`
	ConnResults  []ConnectionImageResult `json:"connResults"`
}

// lastFetchResult stores the result of the most recent fetch images operation
var lastFetchResult struct {
	sync.RWMutex
	Result map[string]*FetchImagesAsyncResult
}

func init() {
	lastFetchResult.Result = make(map[string]*FetchImagesAsyncResult)
}

// FetchImagesForAllConnConfigsAsync starts fetching images in background with provider-based grouping
func FetchImagesForAllConnConfigsAsync(nsId string) error {
	// Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		return err
	}

	// Process asynchronously
	go func() {
		startTime := time.Now()
		log.Info().Msgf("[%s] Starting async image fetch operation", nsId)

		// Get all connection configs
		connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
		if err != nil {
			log.Error().Err(err).Msgf("[%s] Failed to get connection configs", nsId)
			return
		}

		// Initialize result object
		result := &FetchImagesAsyncResult{
			NamespaceID: nsId,
			StartTime:   startTime,
			ConnResults: make([]ConnectionImageResult, 0, len(connConfigs.Connectionconfig)),
		}

		// Store initial result
		lastFetchResult.Lock()
		lastFetchResult.Result[nsId] = result
		lastFetchResult.Unlock()

		// Group connection configs by provider
		providerConnMap := make(map[string][]model.ConnConfig)
		for _, connConfig := range connConfigs.Connectionconfig {
			provider := connConfig.ProviderName
			providerConnMap[provider] = append(providerConnMap[provider], connConfig)
		}

		log.Info().Msgf("[%s] Grouped connections by provider: %d providers",
			nsId, len(providerConnMap))

		// Channel to collect results from all goroutines
		resultChan := make(chan ConnectionImageResult, len(connConfigs.Connectionconfig))
		var wg sync.WaitGroup

		// Create a goroutine for each provider to process its connections sequentially
		for provider, connConfigList := range providerConnMap {
			wg.Add(1)
			go func(provider string, connConfigList []model.ConnConfig) {
				defer wg.Done()
				log.Info().Msgf("[%s] Processing provider %s with %d connections",
					nsId, provider, len(connConfigList))

				// Process each connection of this provider sequentially
				for _, connConfig := range connConfigList {
					connName := connConfig.ConfigName
					region := connConfig.RegionZoneInfo.AssignedRegion

					// Initialize connection result
					connResult := ConnectionImageResult{
						ConnName:  connName,
						Provider:  provider,
						Region:    region,
						StartTime: time.Now(),
						Success:   false,
					}

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
						log.Info().Msgf("[%s] Fetching images for connection %s (%s/%s)",
							nsId, connName, provider, region)

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
						log.Warn().Msgf("[%s] Connection %s timed out", nsId, connName)
					case <-doneChan:
						// Process completed
						if fetchErr != nil {
							connResult.Success = false
							connResult.ErrorMsg = fetchErr.Error()
							log.Error().Err(fetchErr).Msgf("[%s] Failed to fetch images for %s",
								nsId, connName)
						} else {
							connResult.Success = true
							connResult.ImageCount = imageCount
							log.Info().Msgf("[%s] Successfully fetched %d images from %s",
								nsId, imageCount, connName)
						}
					}

					// Clean up and finalize result
					cancel()
					connResult.EndTime = time.Now()
					connResult.ElapsedTime = connResult.EndTime.Sub(connResult.StartTime).String()
					resultChan <- connResult
				}
			}(provider, connConfigList)
		}

		// Close result channel when all providers are processed
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results from all connections
		for connResult := range resultChan {
			result.ConnResults = append(result.ConnResults, connResult)

			if connResult.Success {
				result.SuccessCount++
				result.TotalImages += connResult.ImageCount
			} else {
				result.FailCount++
			}

			// Update intermediate result
			lastFetchResult.Lock()
			lastFetchResult.Result[nsId] = result
			lastFetchResult.Unlock()
		}

		// Finalize result
		result.EndTime = time.Now()
		result.TotalTime = result.EndTime.Sub(result.StartTime).String()

		// Log provider statistics
		providerStats := make(map[string]struct {
			Count      int
			Success    int
			Failed     int
			ImageCount int
		})

		for _, connResult := range result.ConnResults {
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

		log.Info().Msgf("[%s] Async image fetch completed: %d images from %d/%d connections (took %s)",
			nsId, result.TotalImages, result.SuccessCount,
			result.SuccessCount+result.FailCount, result.TotalTime)

		// Save final result
		lastFetchResult.Lock()
		lastFetchResult.Result[nsId] = result
		lastFetchResult.Unlock()
	}()

	return nil
}

// GetFetchImagesAsyncResult returns the result of the most recent fetch images operation
func GetFetchImagesAsyncResult(nsId string) (*FetchImagesAsyncResult, error) {
	lastFetchResult.RLock()
	defer lastFetchResult.RUnlock()

	result, exists := lastFetchResult.Result[nsId]
	if !exists {
		return nil, fmt.Errorf("No fetch images result found for namespace %s", nsId)
	}

	return result, nil
}

// FetchImagesForConnConfig gets lookups all images for the region of conn config, and saves into TB image objects
func FetchImagesForAllConnConfigs(nsId string) (connConfigCount uint, imageCount uint, err error) {
	return fetchImagesForAllConnConfigsInternal(nsId)
}

// FetchImagesForAllConnConfigsInternal gets lookups all images for the region of conn config, and saves into TB image objects
func fetchImagesForAllConnConfigsInternal(nsId string) (connConfigCount uint, imageCount uint, err error) {

	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		return 0, 0, err
	}

	for _, connConfig := range connConfigs.Connectionconfig {
		connConfigCount++

		imageCountPerConn, err := FetchImagesForConnConfig(connConfig.ConfigName, nsId)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to fetch images for connection %s", connConfig.ConfigName)
			continue
		}

		imageCount += imageCountPerConn
	}

	return connConfigCount, imageCount, nil
}

// SearchImage accepts arbitrary number of keywords, and returns the list of matched TB image objects
func SearchImage(nsId string, keywords ...string) ([]model.TbImageInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	tempList := []model.TbImageInfo{}

	//sqlQuery := "SELECT * FROM `image` WHERE `namespace`='" + nsId + "'"
	sqlQuery := model.ORM.Where("namespace = ?", nsId)

	for _, keyword := range keywords {
		keyword = ToNamingRuleCompatible(keyword)
		sqlQuery = sqlQuery.Where("name LIKE ?", "%"+keyword+"%")
	}

	result := sqlQuery.Find(&tempList)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("")
		return tempList, result.Error
	}
	return tempList, nil
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

	providerName, regionName, _, resourceName, err := ResolveProviderRegionZoneResourceKey(imageKey)
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
		// ex: tencent+ap-jakarta+img-487zeit5
		image = model.TbImageInfo{Namespace: model.SystemCommonNs, CspImageName: resourceName}
		result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(csp_image_name) = ? AND LOWER(id) LIKE ? AND LOWER(id) LIKE ?",
			model.SystemCommonNs,
			resourceName,
			"%"+strings.ToLower(providerName)+"%",
			"%"+strings.ToLower(regionName)+"%").First(&image)
		if result.Error != nil {
			log.Info().Err(result.Error).Msgf("Cannot get image %s by CspImageName", resourceName)
		} else {
			return image, nil
		}

		// 3) Check if the image is a registered image in the common namespace model.SystemCommonNs by GuestOS
		// ex: tencent+ap-jakarta+Ubuntu22.04
		image = model.TbImageInfo{Namespace: model.SystemCommonNs, GuestOS: resourceName}
		result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(guest_os) LIKE ? AND LOWER(id) LIKE ? AND LOWER(id) LIKE ?",
			model.SystemCommonNs,
			"%"+strings.ToLower(resourceName)+"%",
			"%"+strings.ToLower(providerName)+"%",
			"%"+strings.ToLower(regionName)+"%").First(&image)
		if result.Error != nil {
			log.Info().Err(result.Error).Msgf("Failed to get image %s by GuestOS type", resourceName)
		} else {
			return image, nil
		}

	}

	return model.TbImageInfo{}, fmt.Errorf("The imageKey %s not found by any of ID, CspImageName, GuestOS", imageKey)
}
