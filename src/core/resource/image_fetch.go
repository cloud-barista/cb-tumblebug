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
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func FetchImagesForConnConfig(connConfig string, nsId string) (imageCount uint, err error) {
	log.Debug().Msg("FetchImages: " + connConfig)

	spiderImageList, err := LookupImageList(connConfig)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	// Pre-allocate slice with known capacity to reduce memory allocations
	tmpImageList := make([]model.ImageInfo, 0, len(spiderImageList.Image))

	// Get connConfig once for skip checks (providerName, regionName)
	fetchConnConfig, connConfigErr := common.GetConnConfig(connConfig)
	fetchProviderName := ""
	fetchRegionName := ""
	if connConfigErr == nil {
		fetchProviderName = fetchConnConfig.ProviderName
		fetchRegionName = fetchConnConfig.RegionDetail.RegionName
	}

	// Process images and clean up memory immediately
	for i := range spiderImageList.Image {
		spiderImage := spiderImageList.Image[i]

		// Pre-filter: skip images already known to be unavailable before conversion
		if spiderImage.ImageStatus == model.ImageUnavailable {
			spiderImageList.Image[i] = model.SpiderImageInfo{}
			continue
		}

		// Pre-filter: skip images matching cloudimage_ignore.yaml (e.g., ParallelCluster AMIs)
		if fetchProviderName != "" {
			if shouldSkipImage(spiderImage.IId.NameId, spiderImage.OSDistribution, fetchProviderName, fetchRegionName, spiderImage.KeyValueList) {
				spiderImageList.Image[i] = model.SpiderImageInfo{}
				continue
			}
		}

		tumblebugImage, err := ConvertSpiderImageToTumblebugImage(nsId, connConfig, spiderImage)
		if err != nil {
			log.Error().Err(err).Msg("")
			// Clean up before returning error
			spiderImageList.Image = nil
			tmpImageList = nil
			return 0, err
		}

		// Post-filter: skip deprecated images (some CSPs report DEPRECATED images as Available;
		// ConvertSpiderImageToTumblebugImage applies keyword-based detection to correct this)
		if tumblebugImage.ImageStatus == model.ImageDeprecated {
			spiderImageList.Image[i] = model.SpiderImageInfo{}
			continue
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

	// Group connection configs by provider
	providerConnMap := make(map[string][]model.ConnConfig)
	for _, connConfig := range connConfigs.Connectionconfig {
		provider := connConfig.ProviderName

		// If targetProviders is specified, only process those providers
		if len(option.TargetProviders) > 0 {
			isTarget := false
			for _, targetProvider := range option.TargetProviders {
				if strings.EqualFold(provider, targetProvider) {
					isTarget = true
					break
				}
			}
			if !isTarget {
				log.Debug().Msgf("[%s] Skipping non-target provider: %s", nsId, provider)
				continue
			}
		} else {
			// Skip excluded providers (only when targetProviders is not specified)
			if slices.Contains(option.ExcludedProviders, provider) {
				log.Debug().Msgf("[%s] Skipping excluded provider: %s", nsId, provider)
				continue
			}
		}

		providerConnMap[provider] = append(providerConnMap[provider], connConfig)
	}

	// Calculate exact target connection count (region-agnostic providers execute 1 connection)
	totalTargetConns := 0
	for provider, connList := range providerConnMap {
		if slices.Contains(option.RegionAgnosticProviders, provider) && len(connList) > 0 {
			totalTargetConns += 1
		} else {
			totalTargetConns += len(connList)
		}
	}

	// Initialize result object
	lastFetchResult.Lock()
	result.TotalRegions = totalTargetConns
	result.FetchOption = *option
	result.ResultInDetail = make([]ConnectionImageResult, 0, totalTargetConns)
	lastFetchResult.Result[nsId] = result
	lastFetchResult.Unlock()

	log.Info().Msgf("[%s] Grouped connections by provider: %d providers (%d effective connections)",
		nsId, len(providerConnMap), totalTargetConns)

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
			if csp.ResolveCloudPlatform(provider) == csp.AWS {
				providerParallelConn = 2 // reduced to mitigate large DescribeImages response stream pressure
			}
			if csp.ResolveCloudPlatform(provider) == csp.Alibaba {
				providerParallelConn = 2 // reduced to mitigate deadlock pressure
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

	// Collect results from all connections and update progress in real time
	for connResult := range resultChan {
		lastFetchResult.Lock()
		result.ResultInDetail = append(result.ResultInDetail, connResult)

		if connResult.Success {
			result.SucceedRegions++
			result.RegisteredImages += connResult.ImageCount
		} else {
			result.FailedRegions++
		}
		result.ElapsedTime = time.Since(result.StartTime).Round(time.Second).String()
		lastFetchResult.Unlock()
	}

	// Finalize result
	endTime := time.Now()
	lastFetchResult.Lock()
	result.ElapsedTime = endTime.Sub(result.StartTime).String()
	result.InProgress = false
	lastFetchResult.Result[nsId] = result
	lastFetchResult.Unlock()

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

// GetFetchImagesAsyncResult returns a snapshot of the most recent fetch images operation
func GetFetchImagesAsyncResult(nsId string) (*FetchImagesAsyncResult, error) {
	lastFetchResult.RLock()
	defer lastFetchResult.RUnlock()

	result, exists := lastFetchResult.Result[nsId]
	if !exists || result == nil {
		return nil, fmt.Errorf("no fetch images result found for namespace %s", nsId)
	}

	snapshot := *result
	if snapshot.InProgress {
		snapshot.ElapsedTime = time.Since(snapshot.StartTime).Round(time.Second).String()
	}

	return &snapshot, nil
}

// createBasicImageInfoFromCSV creates a basic ImageInfo structure from CSV data
func createBasicImageInfoFromCSV(nsId, providerName, regionName, cspImageName, connectionName, osType, description, infraType, osArchitecture, osDistribution string) model.ImageInfo {
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
	imageInfo.ImageStatus = model.ImageAvailable
	if strings.Contains(strings.ToLower(osType+" "+osDistribution), "windows") {
		imageInfo.OSPlatform = model.Windows
	} else {
		imageInfo.OSPlatform = model.Linux_UNIX
	}
	imageInfo.OSDiskSizeGB = -1

	// Set architecture and distribution from CSV if provided.
	// These are populated for CSP images that cannot be enriched via CSP lookup (e.g., EKS/GKE node images).
	if osArchitecture != "" {
		imageInfo.OSArchitecture = model.OSArchitecture(osArchitecture)
	}
	if osDistribution != "" {
		imageInfo.OSDistribution = osDistribution
	}

	// Type-identifier CSP rows registered in cloudimage.csv are always K8s node image types.
	// They are not real imageIds but abstract type identifiers (ami-type / image-type / NodePoolOs).
	if usesTypeIdentifierK8sImage(providerName) {
		imageInfo.IsKubernetesImage = true
	} else if imageInfo.OSDistribution != "" {
		imageInfo.IsBasicImage = common.CheckBasicOSImage(imageInfo.OSDistribution, providerName)
	}

	return imageInfo
}

// enrichImageInfoFromCSP enriches ImageInfo with additional details from CSP lookup
// Uses public image lookup only (no custom image fallback) to avoid unnecessary Spider calls during asset loading
func enrichImageInfoFromCSP(imageInfo *model.ImageInfo, imageReq model.ImageReq, regionName string, connectionList model.ConnConfigList) bool {
	// Try to get additional details from CSP lookup (optional)
	// Use public image only (false parameter) during enrichment to avoid custom image checks
	if strings.EqualFold(regionName, model.StrCommon) {
		// If region is common, try to lookup from any region for this provider
		for _, connConfig := range connectionList.Connectionconfig {
			if strings.EqualFold(connConfig.ProviderName, imageInfo.ProviderName) {
				lookupReq := imageReq
				lookupReq.ConnectionName = imageInfo.ProviderName + "-" + connConfig.RegionDetail.RegionName

				// Pass false to use public image lookup only (no custom image fallback)
				if detailedInfo, err := GetImageInfoFromLookupImage(model.SystemCommonNs, lookupReq, false); err == nil {
					mergeCSPDetails(imageInfo, &detailedInfo)
					log.Info().Msgf("Successfully looked up image details from CSP: %s", imageReq.CspImageName)
					return true
				}
			}
		}
	} else {
		// Pass false to use public image lookup only (no custom image fallback)
		if detailedInfo, err := GetImageInfoFromLookupImage(model.SystemCommonNs, imageReq, false); err == nil {
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
	target.IsBasicGpuImage = source.IsBasicGpuImage
	target.OSDiskType = source.OSDiskType
	target.OSDiskSizeGB = source.OSDiskSizeGB
	target.CreationDate = source.CreationDate
	target.ImageStatus = source.ImageStatus
	target.IsGPUImage = source.IsGPUImage
	// Do not overwrite IsKubernetesImage for type-identifier CSPs (AWS/GCP/Tencent).
	// Their IsKubernetesImage=true is set by policy (cloudimage.csv), not by CSP lookup.
	if !usesTypeIdentifierK8sImage(target.ProviderName) {
		target.IsKubernetesImage = source.IsKubernetesImage
	}
	target.Details = source.Details
}

// updateExistingImageFromCSV updates existing image with CSV data
func updateExistingImageFromCSV(existingImage model.ImageInfo, osType, description, infraType, osArchitecture, osDistribution string) model.ImageInfo {
	existingImage.OSType = osType
	existingImage.Description = description
	existingImage.InfraType = expandInfraType(infraType)
	existingImage.SystemLabel = model.StrFromAssets

	// Set architecture and distribution only when CSV provides a value.
	// If empty, preserve the value already set by CSP lookup (important for Azure).
	if osArchitecture != "" {
		existingImage.OSArchitecture = model.OSArchitecture(osArchitecture)
	}
	if osDistribution != "" {
		existingImage.OSDistribution = osDistribution
	}

	// Re-apply policy: type-identifier CSP images registered via cloudimage.csv
	// are always K8s node image types.
	if usesTypeIdentifierK8sImage(existingImage.ProviderName) {
		existingImage.IsKubernetesImage = true
	}

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
		// row7: osArchitecture (optional)
		// row8: osDistribution (optional)
		providerName := strings.ToLower(row[0])
		regionName := strings.ToLower(row[1])
		imageReqTmp.CspImageName = row[2]
		osType := row[3]
		description := row[4]
		infraType := strings.ToLower(row[6])
		osArchitecture := ""
		osDistribution := ""
		if len(row) > 7 {
			osArchitecture = strings.ToLower(row[7])
		}
		if len(row) > 8 {
			osDistribution = row[8]
		}

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
					imageReqTmp.ConnectionName, osType, description, infraType, osArchitecture, osDistribution)

				// K8s image type identifiers (EKS AMI types, GKE image families, TKE OS names,
				// ACK image_type names) registered in cloudimage.csv are not real CSP image IDs.
				// CSP lookup via CB-Spider will always fail for these, so skip enrichment.
				skipCSPLookup := tmpImageInfo.IsKubernetesImage && usesTypeIdentifierK8sImage(providerName)

				if !skipCSPLookup {
					// Try to enrich with CSP lookup (optional)
					enrichImageInfoFromCSP(&tmpImageInfo, imageReqTmp, regionName, connectionList)
				}

				// Add to list regardless of CSP lookup success
				mutex.Lock()
				tmpImageList = append(tmpImageList, tmpImageInfo)
				mutex.Unlock()

			}(i, row, lenImages)
		} else {
			// Update existing image with new information from the asset file
			tmpImageInfo := updateExistingImageFromCSV(existingImage, osType, description, infraType, osArchitecture, osDistribution)

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

	if updatedCount, refreshErr := refreshImageOSTypesFromPatterns(nsId); refreshErr != nil {
		log.Warn().Err(refreshErr).Msg("Failed to refresh existing image OS types from extractionpatterns.yaml")
	} else if updatedCount > 0 {
		log.Info().Int("updatedImages", updatedCount).Msg("Refreshed existing image OS types from extractionpatterns.yaml")
	}

	elapsedUpdateImg := time.Since(startTime)

	log.Info().Msgf("Updated the registered Images according to the asset file. Elapsed [%s]", elapsedUpdateImg)

	result.InProgress = false
	result.ElapsedTime = time.Since(startTime).String()
	updateFetchImagesProgress(nsId, result)
	return result, nil
}

// refreshImageOSTypesFromPatterns re-evaluates os_type, basic/k8s/gpu flags, architecture, and disk size
// for existing images in DB using the current extractionpatterns.yaml and utility_image.go logic.
func refreshImageOSTypesFromPatterns(nsId string) (int, error) {
	totalUpdated := 0

	// 1. Normalize NCP Bare Metal hypervisor suffix in os_distribution if present
	ncpFixRes := model.ORM.Exec(`
		UPDATE image_infos
		SET os_distribution = REPLACE(os_distribution, '(Hypervisor:{code:BM,codeName:BM})', '(Hypervisor:BM)')
		WHERE namespace = ? AND provider_name = 'ncp' AND os_distribution LIKE '%(Hypervisor:{code:BM,codeName:BM})%'
	`, nsId)
	if ncpFixRes.Error == nil {
		totalUpdated += int(ncpFixRes.RowsAffected)
	}

	// 2. Refresh os_type and is_basic_image (for non-GPU images) based on current extractionpatterns.yaml
	type distRow struct {
		ProviderName   string `gorm:"column:provider_name"`
		OSDistribution string `gorm:"column:os_distribution"`
		OSType         string `gorm:"column:os_type"`
		IsGPUImage     bool   `gorm:"column:is_gpu_image"`
		IsBasicImage   bool   `gorm:"column:is_basic_image"`
		FromAssets     bool   `gorm:"column:from_assets"`
	}
	var rows []distRow
	if err := model.ORM.Model(&model.ImageInfo{}).
		Select("DISTINCT provider_name, os_distribution, os_type, is_gpu_image, is_basic_image, (COALESCE(system_label, '') = ?) AS from_assets", model.StrFromAssets).
		Where("namespace = ? AND os_distribution != ''", nsId).
		Find(&rows).Error; err != nil {
		return totalUpdated, err
	}

	type groupKey struct {
		provider     string
		newOSType    string
		isGPU        bool
		newBasic     bool
		fromAssets   bool
		updateOSType bool
	}
	updates := make(map[groupKey][]string)
	for _, r := range rows {
		newOS := common.ExtractOSInfo(r.OSDistribution)
		if r.FromAssets || newOS == "" {
			newOS = r.OSType
		}
		newBasic := !r.IsGPUImage && common.CheckBasicOSImage(r.OSDistribution, r.ProviderName)

		if newOS != r.OSType || newBasic != r.IsBasicImage {
			k := groupKey{
				provider:     r.ProviderName,
				newOSType:    newOS,
				isGPU:        r.IsGPUImage,
				newBasic:     newBasic,
				fromAssets:   r.FromAssets,
				updateOSType: !r.FromAssets && newOS != "",
			}
			updates[k] = append(updates[k], r.OSDistribution)
		}
	}

	for k, dists := range updates {
		for i := 0; i < len(dists); i += 200 {
			end := min(i+200, len(dists))
			updateMap := map[string]interface{}{
				"is_basic_image": k.newBasic,
			}
			if k.updateOSType {
				updateMap["os_type"] = k.newOSType
			}
			q := model.ORM.Model(&model.ImageInfo{}).
				Where("namespace = ? AND provider_name = ? AND is_gpu_image = ? AND os_distribution IN ?",
					nsId, k.provider, k.isGPU, dists[i:end])
			if k.fromAssets {
				q = q.Where("COALESCE(system_label, '') = ?", model.StrFromAssets)
			} else {
				q = q.Where("COALESCE(system_label, '') != ?", model.StrFromAssets)
			}
			res := q.Updates(updateMap)
			if res.Error != nil {
				return totalUpdated, res.Error
			}
			totalUpdated += int(res.RowsAffected)
		}
	}

	// 2b. Refresh is_basic_gpu_image for GPU images (excluding AWS where 90k DLAMIs are evaluated by OS/distribution)
	var gpuImages []model.ImageInfo
	if err := model.ORM.Select("provider_name, csp_image_name, name, os_distribution, is_basic_gpu_image").
		Where("namespace = ? AND is_gpu_image = true AND provider_name != 'aws'", nsId).
		Find(&gpuImages).Error; err == nil {
		var setTrueIDs, setFalseIDs []string
		for _, img := range gpuImages {
			searchStr := strings.Join([]string{img.OSDistribution, img.CspImageName, img.Name}, " ")
			want := common.CheckBasicGpuImage(searchStr, img.ProviderName)
			if want != img.IsBasicGpuImage {
				if want {
					setTrueIDs = append(setTrueIDs, img.CspImageName)
				} else {
					setFalseIDs = append(setFalseIDs, img.CspImageName)
				}
			}
		}
		if len(setTrueIDs) > 0 {
			res := model.ORM.Model(&model.ImageInfo{}).
				Where("namespace = ? AND provider_name != 'aws' AND csp_image_name IN ?", nsId, setTrueIDs).
				Update("is_basic_gpu_image", true)
			if res.Error == nil {
				totalUpdated += int(res.RowsAffected)
			}
		}
		if len(setFalseIDs) > 0 {
			res := model.ORM.Model(&model.ImageInfo{}).
				Where("namespace = ? AND provider_name != 'aws' AND csp_image_name IN ?", nsId, setFalseIDs).
				Update("is_basic_gpu_image", false)
			if res.Error == nil {
				totalUpdated += int(res.RowsAffected)
			}
		}
	}

	// 3. Re-evaluate is_kubernetes_image for non-asset images currently marked true
	var k8sCandidates []model.ImageInfo
	if err := model.ORM.Where("namespace = ? AND is_kubernetes_image = true AND COALESCE(system_label, '') != ?",
		nsId, model.StrFromAssets).Find(&k8sCandidates).Error; err == nil {
		falseIDs := make([]string, 0)
		for _, img := range k8sCandidates {
			var kvStrBuilder strings.Builder
			for _, kv := range img.Details {
				kvStrBuilder.WriteString(kv.Key + ":" + kv.Value + " ")
			}
			fullSearchStr := strings.Join([]string{img.OSDistribution, img.CspImageName, img.Name, kvStrBuilder.String()}, " ")
			if !common.IsK8sImage(fullSearchStr) {
				falseIDs = append(falseIDs, img.CspImageName)
			}
		}
		for i := 0; i < len(falseIDs); i += 200 {
			end := min(i+200, len(falseIDs))
			res := model.ORM.Model(&model.ImageInfo{}).
				Where("namespace = ? AND COALESCE(system_label, '') != ? AND csp_image_name IN ?",
					nsId, model.StrFromAssets, falseIDs[i:end]).
				Update("is_kubernetes_image", false)
			if res.Error == nil {
				totalUpdated += int(res.RowsAffected)
			}
		}
	}

	// 4. Repair missing os_architecture ('na' or '') for GCP/KT/OpenStack/Tencent/etc.
	var naArchImages []model.ImageInfo
	if err := model.ORM.Where("namespace = ? AND os_architecture IN ('na', '')", nsId).Find(&naArchImages).Error; err == nil {
		for _, img := range naArchImages {
			lowerStr := strings.ToLower(img.OSDistribution + " " + img.CspImageName + " " + img.Name)
			newArch := model.OSArchitecture("")
			switch {
			case strings.Contains(lowerStr, "arm64") || strings.Contains(lowerStr, "aarch64"):
				newArch = model.ARM64
			case strings.Contains(lowerStr, "s390x"):
				newArch = model.S390X
			case strings.Contains(lowerStr, "32bit") || strings.Contains(lowerStr, "32-bit") || strings.Contains(lowerStr, "i386"):
				newArch = model.X86_32
			case strings.Contains(lowerStr, "x86_64") || strings.Contains(lowerStr, "amd64") ||
				strings.Contains(lowerStr, "x64") || strings.Contains(lowerStr, "64bit") || strings.Contains(lowerStr, "64-bit"):
				newArch = model.X86_64
			case strings.EqualFold(img.ProviderName, csp.KT) ||
				strings.EqualFold(img.ProviderName, csp.GCP) ||
				strings.EqualFold(img.ProviderName, csp.OpenStack):
				newArch = model.X86_64
			}
			if newArch != "" {
				res := model.ORM.Model(&model.ImageInfo{}).
					Where("namespace = ? AND provider_name = ? AND csp_image_name = ?", nsId, img.ProviderName, img.CspImageName).
					Update("os_architecture", newArch)
				if res.Error == nil {
					totalUpdated += int(res.RowsAffected)
				}
			}
		}
	}

	// 5. Repair missing os_disk_size_gb (<= 0) from Details KeyValueList (KT, IBM, etc.)
	var missingDiskImages []model.ImageInfo
	if err := model.ORM.Where("namespace = ? AND os_disk_size_gb <= 0 AND provider_name IN ('kt', 'ibm')", nsId).Find(&missingDiskImages).Error; err == nil {
		for _, img := range missingDiskImages {
			var parsedDisk float64
			for _, kv := range img.Details {
				kLower := strings.ToLower(strings.TrimSpace(kv.Key))
				if kLower == "mindiskgigabytes" || kLower == "minimumprovisionedsize" ||
					kLower == "disksizegb" || kLower == "mindisk" || kLower == "size" || kLower == "imagesize" {
					if v, err := strconv.ParseFloat(strings.TrimSpace(kv.Value), 64); err == nil && v > 0 {
						if v > 1024*1024*1024 {
							v = float64(int64((v + (1024*1024*1024 - 1)) / (1024 * 1024 * 1024)))
						}
						parsedDisk = v
						break
					}
				}
			}
			if parsedDisk > 0 {
				res := model.ORM.Model(&model.ImageInfo{}).
					Where("namespace = ? AND provider_name = ? AND csp_image_name = ?", nsId, img.ProviderName, img.CspImageName).
					Update("os_disk_size_gb", parsedDisk)
				if res.Error == nil {
					totalUpdated += int(res.RowsAffected)
				}
			}
		}
	}

	return totalUpdated, nil
}
