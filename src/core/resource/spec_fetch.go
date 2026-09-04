/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	alibabaPricing "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

// LookupSpecList accepts Spider conn config,
// lookups and returns the list of all specs in the region of conn config
// in the form of the list of Spider spec objects
func LookupSpecList(connConfig string) (model.SpiderSpecList, error) {

	if connConfig == "" {
		content := model.SpiderSpecList{}
		err := fmt.Errorf("LookupSpec called with empty connConfig.")
		log.Error().Err(err).Msg("")
		return content, err
	}

	var callResult model.SpiderSpecList
	client := clientManager.NewHttpClient()
	client.SetTimeout(10 * time.Minute)
	url := model.SpiderRestUrl + "/vmspec"
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig

	_, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.VeryLongDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("")
		content := model.SpiderSpecList{}
		return content, err
	}

	temp := callResult
	return temp, nil

}

// LookupSpec accepts Spider conn config and CSP spec name, lookups and returns the Spider spec object
func LookupSpec(connConfig string, specName string) (model.SpiderSpecInfo, error) {

	if connConfig == "" {
		content := model.SpiderSpecInfo{}
		err := fmt.Errorf("LookupSpec() called with empty connConfig.")
		log.Error().Err(err).Msg("")
		return content, err
	} else if specName == "" {
		content := model.SpiderSpecInfo{}
		err := fmt.Errorf("LookupSpec() called with empty specName.")
		log.Error().Err(err).Msg("")
		return content, err
	}

	client := clientManager.NewHttpClient()
	client.SetTimeout(4 * time.Minute)
	url := model.SpiderRestUrl + "/vmspec/" + specName
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderSpecInfo{}

	err := executeWithTimeoutRetry(func() error {
		callResult = model.SpiderSpecInfo{}
		_, err := clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&callResult,
			clientManager.VeryLongDuration,
		)
		return err
	}, "LookupSpec", specName)

	if err != nil {
		log.Error().Err(err).Msg("")
		return callResult, err
	}

	return callResult, nil
}

// FetchSpecsForConnConfig lookups all specs for region of conn config, and saves into TB spec objects
func FetchSpecsForConnConfig(connConfigName string, nsId string) (uint, error) {
	log.Debug().Msg("FetchSpecsForConnConfig(" + connConfigName + ")")

	connConfig, err := common.GetConnConfig(connConfigName)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot GetConnConfig in %s", connConfigName)
		return 0, err
	}

	var specsInConnection model.SpiderSpecList
	if csp.ResolveCloudPlatform(connConfig.ProviderName) == csp.Alibaba {
		baseCtx := context.Background()
		if connConfig.CredentialHolder != "" {
			baseCtx = common.WithCredentialHolder(baseCtx, connConfig.CredentialHolder)
		}
		directCtx, cancel := context.WithTimeout(baseCtx, 5*time.Minute)
		defer cancel()
		specsInConnection, err = alibabaPricing.FetchAvailableSpecListByRegion(
			directCtx,
			connConfig.RegionDetail.RegionName,
			connConfig.RegionZoneInfo.AssignedZone,
		)
		if err != nil {
			log.Error().Err(err).Msgf("Cannot fetch Alibaba available specs directly in %s", connConfigName)
			return 0, err
		}
	} else {
		specsInConnection, err = LookupSpecList(connConfigName)
		if err != nil {
			log.Error().Err(err).Msgf("Cannot LookupSpecList in %s", connConfigName)
			return 0, err
		}
	}

	if len(specsInConnection.Vmspec) == 0 {
		log.Debug().Msgf("No specs found for connection %s", connConfigName)
		return 0, nil
	}

	// Step 1: Pre-filter specs to ignore based on cloudspec_ignore.yaml
	totalSpecs := len(specsInConnection.Vmspec)
	filteredSpecs := make([]model.SpiderSpecInfo, 0, totalSpecs)
	ignoredCount := 0
	skipIgnoreFilter := csp.ResolveCloudPlatform(connConfig.ProviderName) == csp.Alibaba

	// log.Debug().
	// 	Str("connection", connConfigName).
	// 	Str("provider", connConfig.ProviderName).
	// 	Str("region", connConfig.RegionDetail.RegionName).
	// 	Int("totalSpecs", totalSpecs).
	// 	Msg("Starting spec filtering process")

	// Filter out specs that should be ignored
	for i := range specsInConnection.Vmspec {
		spiderSpec := specsInConnection.Vmspec[i]

		if !skipIgnoreFilter && shouldIgnoreSpec(spiderSpec.Name, connConfig.ProviderName, connConfig.RegionDetail.RegionName) {
			// log.Debug().
			// 	Str("spec", spiderSpec.Name).
			// 	Str("provider", connConfig.ProviderName).
			// 	Str("region", connConfig.RegionDetail.RegionName).
			// 	Msg("Ignoring Spec")
			ignoredCount++
			continue
		}

		filteredSpecs = append(filteredSpecs, spiderSpec)
	}

	// Clear original specs list to free memory
	specsInConnection.Vmspec = nil
	specsInConnection = model.SpiderSpecList{}

	// Log filtering results
	filteredCount := len(filteredSpecs)
	// log.Info().
	// 	Str("connection", connConfigName).
	// 	Str("provider", connConfig.ProviderName).
	// 	Str("region", connConfig.RegionDetail.RegionName).
	// 	Int("totalSpecs", totalSpecs).
	// 	Int("ignoredSpecs", ignoredCount).
	// 	Int("filteredSpecs", filteredCount).
	// 	Msgf("Spec filtering completed: %d/%d specs will be processed (%d ignored)",
	// 		filteredCount, totalSpecs, ignoredCount)

	// Step 2: Process filtered specs and convert to Tumblebug format
	tmpSpecList := make([]model.SpecInfo, 0, filteredCount)

	// Process only the filtered specs
	for i := range filteredSpecs {
		spiderSpec := filteredSpecs[i]

		tumblebugSpec, errConvert := ConvertSpiderSpecToTumblebugSpec(connConfig, spiderSpec)
		if errConvert != nil {
			// log.Debug().Err(errConvert).Msgf("Skip ConvertSpiderSpecToTumblebugSpec for %s", spiderSpec.Name)
			// Clear the processed item immediately
			filteredSpecs[i] = model.SpiderSpecInfo{}
			continue
		}

		// Set basic information
		key := GetProviderRegionZoneResourceKey(connConfig.ProviderName, connConfig.RegionDetail.RegionName, "", spiderSpec.Name)
		tumblebugSpec.Namespace = nsId
		tumblebugSpec.Id = key
		tumblebugSpec.Name = key
		tumblebugSpec.ConnectionName = connConfig.ConfigName
		tumblebugSpec.ProviderName = strings.ToLower(connConfig.ProviderName)
		tumblebugSpec.RegionName = connConfig.RegionDetail.RegionName
		tumblebugSpec.InfraType = model.StrNode // default value should be enhanced later
		tumblebugSpec.SystemLabel = "auto-gen"  // default value
		tumblebugSpec.AssociatedObjectList = []string{}

		tumblebugSpec.CostPerHour = -1
		tumblebugSpec.EvaluationScore01 = -1
		tumblebugSpec.EvaluationScore02 = -1
		tumblebugSpec.EvaluationScore03 = -1
		tumblebugSpec.EvaluationScore04 = -1
		tumblebugSpec.EvaluationScore05 = -1
		tumblebugSpec.EvaluationScore06 = -1
		tumblebugSpec.EvaluationScore07 = -1
		tumblebugSpec.EvaluationScore08 = -1
		tumblebugSpec.EvaluationScore09 = -1
		tumblebugSpec.EvaluationScore10 = -1

		tmpSpecList = append(tmpSpecList, tumblebugSpec)

		// Clear the processed spider spec immediately to free memory
		filteredSpecs[i] = model.SpiderSpecInfo{}
	}

	// Release the filtered specs list immediately after processing
	filteredSpecs = nil

	specCount := uint(len(tmpSpecList))

	// Log spec processing summary
	log.Info().
		Str("connection", connConfigName).
		Str("provider", connConfig.ProviderName).
		Str("region", connConfig.RegionDetail.RegionName).
		Int("totalSpecs", totalSpecs).
		Int("ignoredSpecs", ignoredCount).
		Int("processedSpecs", int(specCount)).
		Msgf("Spec processing completed: %d/%d (%d ignored)",
			specCount, totalSpecs, ignoredCount)

	// Perform bulk registration
	if len(tmpSpecList) > 0 {
		err = RegisterSpecWithInfoInBulk(tmpSpecList)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to register specs in bulk for %s", connConfigName)
			// Clean up before returning error
			tmpSpecList = nil
			return 0, err
		}
		// log.Info().Msgf("Successfully registered %d specs for connection %s", len(tmpSpecList), connConfigName)
	}

	// Clear the temporary spec list after successful registration
	tmpSpecList = nil

	// Force garbage collection hint for large datasets
	if specCount > 100 {
		runtime.GC()
	}

	//log.Debug().Msgf("Memory cleanup completed for connection %s", connConfigName)
	return specCount, nil
}

// Common internal function for fetching specs that can be used by both sync and async versions
func fetchSpecsForAllConnConfigsInternal(nsId string, option *model.SpecFetchOption, result *FetchSpecsAsyncResult) (*FetchSpecsAsyncResult, error) {
	// Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		return nil, err
	}

	// Initialize fetch options
	if option == nil {
		option = &model.SpecFetchOption{}
	}

	// Set default parallel connections per provider if not specified
	parallelConnPerProvider := 50

	log.Info().Msgf("[%s] Starting spec fetch operation", nsId)

	// Get all connection configs
	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msgf("[%s] Failed to get connection configs", nsId)
		return nil, err
	}

	// Initialize result object
	result.TotalRegions = len(connConfigs.Connectionconfig)
	result.FetchOption = *option
	result.ResultInDetail = make([]ConnectionSpecResult, 0, len(connConfigs.Connectionconfig))

	updateFetchSpecsProgress(nsId, result)

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
		} else if len(option.ExcludedProviders) > 0 {
			// Skip excluded providers (only when targetProviders is not specified)
			excluded := false
			for _, excludedProvider := range option.ExcludedProviders {
				if strings.EqualFold(provider, excludedProvider) {
					excluded = true
					break
				}
			}
			if excluded {
				log.Info().Msgf("[%s] Skipping excluded provider: %s", nsId, provider)
				continue
			}
		}

		providerConnMap[provider] = append(providerConnMap[provider], connConfig)
	}

	log.Info().Msgf("[%s] Grouped connections by provider: %d providers",
		nsId, len(providerConnMap))

	// Channel to collect results from all goroutines
	resultChan := make(chan ConnectionSpecResult, len(connConfigs.Connectionconfig))
	var wg sync.WaitGroup

	// Create a goroutine for each provider
	for provider, connConfigList := range providerConnMap {
		wg.Add(1)
		go func(provider string, connConfigList []model.ConnConfig) {
			defer wg.Done()
			log.Info().Msgf("[%s] Processing provider %s with %d connections",
				nsId, provider, len(connConfigList))

			// Adjust parallel connections for specific providers
			// Parallel spec fetch capacity test results (2026-04-06, cb-spider vmspec API):
			//   AWS     (28 conns): 28/28 OK at full parallel (~8.8s)  -> no limit needed
			//   GCP     (42 conns): 42/42 OK at full parallel (~3.2s)  -> no limit needed
			//   Azure   (46 conns): 46/46 OK at full parallel (~100s)  -> no limit needed
			//   Tencent (16 conns): 16/16 OK at full parallel (~11.0s) -> no limit needed
			//   IBM     (11 conns): 11/11 OK at full parallel (~8.5s)  -> no limit needed
			//   Alibaba (29 conns): 5/5 OK (100%), 10+ causes ~90% timeout -> limit to 5
			providerParallelConn := parallelConnPerProvider
			if provider == csp.Alibaba {
				providerParallelConn = 5 // Direct ECS API (DescribeAvailableResource): 0 failures at concurrency=5;
			}

			// Set up semaphore for controlled parallelism
			semaphore := make(chan struct{}, providerParallelConn)

			var providerWg sync.WaitGroup

			// Process connections of this provider with controlled parallelism
			for i, connConfig := range connConfigList {
				// Acquire semaphore to limit concurrent connections
				semaphore <- struct{}{}

				providerWg.Add(1)
				go func(connConfig model.ConnConfig, index int) {
					defer providerWg.Done()
					defer func() { <-semaphore }()

					connName := connConfig.ConfigName
					region := connConfig.RegionZoneInfo.AssignedRegion

					// Initialize connection result
					connResult := ConnectionSpecResult{
						ConnName:  connName,
						Provider:  provider,
						Region:    region,
						StartTime: time.Now(),
						Success:   false,
					}

					log.Info().Msgf("[%s][Provider-%s][Conn-%d] Processing connection %s (%s/%s)",
						nsId, provider, index, connName, provider, region)

					// Set timeout for this connection
					timeout := 20 * time.Minute
					ctx, cancel := context.WithTimeout(context.Background(), timeout)

					// Process specs for this connection
					doneChan := make(chan struct{})
					var specCount int
					var fetchErr error

					// Fetch specs in a separate goroutine to handle timeout
					go func() {
						defer close(doneChan)
						count, err := FetchSpecsForConnConfig(connName, nsId)
						specCount = int(count)
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
							log.Error().Err(fetchErr).Msgf("[%s][Provider-%s][Conn-%d] Failed to fetch specs for %s",
								nsId, provider, index, connName)
						} else {
							connResult.Success = true
							connResult.SpecCount = specCount
							log.Info().Msgf("[%s][%s][%d] Successfully fetched %d specs from %s",
								nsId, provider, index, specCount, connName)
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
			result.RegisteredSpecs += connResult.SpecCount
		} else {
			result.FailedRegions++
		}
	}

	// Finalize result
	endTime := time.Now()
	result.ElapsedTime = endTime.Sub(result.StartTime).String()
	result.InProgress = false
	updateFetchSpecsProgress(nsId, result)

	// Log provider statistics
	providerStats := make(map[string]struct {
		Count     int
		Success   int
		Failed    int
		SpecCount int
	})

	for _, connResult := range result.ResultInDetail {
		stats := providerStats[connResult.Provider]
		stats.Count++
		if connResult.Success {
			stats.Success++
			stats.SpecCount += connResult.SpecCount
		} else {
			stats.Failed++
		}
		providerStats[connResult.Provider] = stats
	}

	for provider, stats := range providerStats {
		log.Info().Msgf("[%s] Provider %s: %d connections (%d success, %d failed), %d specs",
			nsId, provider, stats.Count, stats.Success, stats.Failed, stats.SpecCount)
	}

	log.Info().Msgf("[%s] Spec fetch completed: %d specs from %d/%d connections (took %s)",
		nsId, result.RegisteredSpecs, result.SucceedRegions,
		result.SucceedRegions+result.FailedRegions, result.ElapsedTime)

	return result, nil
}

// FetchSpecsForAllConnConfigsAsync starts fetching specs in background with provider-based grouping
func FetchSpecsForAllConnConfigsAsync(nsId string, option *model.SpecFetchOption) error {
	// Check if there's already an operation in progress
	if isSpecFetchInProgress(nsId) {
		return fmt.Errorf("a spec fetch operation is already in progress")
	}

	result := &FetchSpecsAsyncResult{
		NamespaceID: nsId,
		StartTime:   time.Now(),
		InProgress:  true,
	}
	updateFetchSpecsProgress(nsId, result)

	// Process asynchronously
	go func() {
		result, err := fetchSpecsForAllConnConfigsInternal(nsId, option, result)
		if err != nil {
			log.Error().Err(err).Msgf("[%s] Failed to fetch specs asynchronously", nsId)
			result.InProgress = false
			result.ElapsedTime = time.Since(result.StartTime).String()
			updateFetchSpecsProgress(nsId, result)
			return
		}
		log.Info().Msgf("[%s] Async spec fetch operation completed and result saved", nsId)
	}()

	return nil
}

// GetFetchSpecsAsyncResult returns the result of the most recent fetch specs operation
func GetFetchSpecsAsyncResult(nsId string) (*FetchSpecsAsyncResult, error) {
	lastFetchSpecsResult.RLock()
	defer lastFetchSpecsResult.RUnlock()

	result, exists := lastFetchSpecsResult.Result[nsId]
	if !exists {
		return nil, fmt.Errorf("no fetch specs result found for namespace %s", nsId)
	}

	// Update elapsed time if still in progress
	if result.InProgress {
		result.ElapsedTime = time.Since(result.StartTime).String()
	}

	return result, nil
}

// FetchSpecsForAllConnConfigs synchronously fetches specs for all connection configs in the namespace
func FetchSpecsForAllConnConfigs(nsId string, option *model.SpecFetchOption) (*FetchSpecsAsyncResult, error) {
	// Check if there's already an operation in progress
	if isSpecFetchInProgress(nsId) {
		return nil, fmt.Errorf("a spec fetch operation is already in progress")
	}

	result := &FetchSpecsAsyncResult{
		NamespaceID: nsId,
		StartTime:   time.Now(),
		InProgress:  true,
	}
	updateFetchSpecsProgress(nsId, result)

	// Direct call to internal function and wait for completion
	result, err := fetchSpecsForAllConnConfigsInternal(nsId, option, result)
	if err != nil {
		log.Error().Err(err).Msgf("[%s] Failed to fetch specs synchronously", nsId)
		result.InProgress = false
		result.ElapsedTime = time.Since(result.StartTime).String()
		updateFetchSpecsProgress(nsId, result)
		return nil, err
	}

	return result, nil
}

var lastFetchSpecsResult struct {
	sync.RWMutex
	Result map[string]*FetchSpecsAsyncResult
}

func init() {
	lastFetchSpecsResult.Result = make(map[string]*FetchSpecsAsyncResult)
}

// updateFetchSpecsProgress updates the progress of fetch specs operation
func updateFetchSpecsProgress(nsId string, result *FetchSpecsAsyncResult) {
	lastFetchSpecsResult.Lock()
	lastFetchSpecsResult.Result[nsId] = result
	lastFetchSpecsResult.Unlock()
}

// isSpecFetchInProgress checks if there's an ongoing spec fetch operation for the given namespace
func isSpecFetchInProgress(nsId string) bool {
	lastFetchSpecsResult.RLock()
	defer lastFetchSpecsResult.RUnlock()

	result, exists := lastFetchSpecsResult.Result[nsId]
	if exists && result != nil && result.InProgress {
		return true
	}
	return false
}

// ConnectionSpecResult is the result of fetching specs for a single connection
type ConnectionSpecResult struct {
	ConnName    string    `json:"connName"`
	Provider    string    `json:"provider"`
	Region      string    `json:"region"`
	SpecCount   int       `json:"specCount"`
	StartTime   time.Time `json:"startTime"`
	ElapsedTime string    `json:"elapsedTime"`
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"errorMsg,omitempty"`
}

// FetchSpecsAsyncResult is the result of the most recent fetch specs operation
type FetchSpecsAsyncResult struct {
	NamespaceID     string                 `json:"namespaceId"`
	TotalRegions    int                    `json:"totalRegions"`
	FetchOption     model.SpecFetchOption  `json:"fetchOption"`
	InProgress      bool                   `json:"inProgress"`
	RegisteredSpecs int                    `json:"registeredSpecs"`
	SucceedRegions  int                    `json:"succeedRegions"`
	FailedRegions   int                    `json:"failedRegions"`
	StartTime       time.Time              `json:"startTime"`
	ElapsedTime     string                 `json:"elapsedTime"`
	ResultInDetail  []ConnectionSpecResult `json:"resultInDetail"`
}

// UpdateSpecsFromAsset updates spec information based on cloudspec.csv asset file
func UpdateSpecsFromAsset(nsId string) error {
	if nsId == "" {
		nsId = model.SystemCommonNs
	}

	// Open and read CSV file
	csvPath := common.GetAssetsFilePath("cloudspec.csv")
	file, err := os.Open(csvPath)
	if err != nil {
		log.Error().
			Err(err).
			Str("attempted_path", csvPath).
			Msg("Failed to open cloudspec.csv")
		return fmt.Errorf("failed to open cloudspec.csv at %s: %w", csvPath, err)
	}
	defer file.Close()

	rdr := csv.NewReader(bufio.NewReader(file))
	rows, err := rdr.ReadAll()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read cloudspec.csv")
		return err
	}
	// row[0]	providerName
	// row[1]	regionName
	// row[2]	cspSpecName
	// row[3]	costPerHour
	// row[4]	currency
	// row[5]	evaluationScore01
	// row[6]	evaluationScore02
	// row[7]	evaluationScore03
	// row[8]	evaluationScore04
	// row[9]	evaluationScore05
	// row[10]	evaluationScore06
	// row[11]	evaluationScore07
	// row[12]	evaluationScore08
	// row[13]	evaluationScore09
	// row[14]	evaluationScore10
	// row[15]	rootDiskType
	// row[16]	rootDiskSize
	// row[17]	acceleratorType
	// row[18]	acceleratorModel
	// row[19]	acceleratorCount
	// row[20]	acceleratorMemoryGB
	// row[21]	description
	// row[22]	infraType

	// expending rows with "all" connectionName into each region
	// "all" means the values in the row are applicable to all connectionNames in a CSP

	connectionList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot GetConnConfigList")
		return err
	}
	if len(connectionList.Connectionconfig) == 0 {
		log.Error().Err(err).Msg("No registered connection config")
		return err
	}

	newRowsSpec := make([][]string, 0, len(rows))
	for _, row := range rows {
		if row[1] == "all" {
			for _, connConfig := range connectionList.Connectionconfig {
				if strings.EqualFold(connConfig.ProviderName, row[0]) {
					newRow := make([]string, len(row))
					copy(newRow, row)
					newRow[1] = connConfig.RegionDetail.RegionName
					newRowsSpec = append(newRowsSpec, newRow)
					//log.Info().Msgf("Expended row: %s", newRow)
				}
			}
		} else {
			newRowsSpec = append(newRowsSpec, row)
		}
	}
	rows = newRowsSpec

	startTime := time.Now()
	// Load all existing specs for the namespace into memory
	existingSpecsMap, err := loadAllSpecsIntoMemory(nsId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load existing specs into memory")
		return err
	}
	log.Info().Msgf("Loaded %d existing specs into memory in %v", len(existingSpecsMap), time.Since(startTime))

	var specList []model.SpecInfo

	// Process each row (skip header)
	for _, row := range rows[1:] {

		// Parse CSV data directly into SpecInfo struct
		specInfo := model.SpecInfo{}

		providerName := strings.ToLower(row[0])
		regionName := strings.ToLower(row[1])
		cspSpecName := row[2]
		specInfoId := GetProviderRegionZoneResourceKey(providerName, regionName, "", cspSpecName)
		rootDiskType := row[15]
		rootDiskSize := 0
		if s, err := strconv.Atoi(strings.ReplaceAll(row[16], " ", "")); err == nil {
			rootDiskSize = s
		}
		acceleratorType := row[17]
		acceleratorModel := row[18]
		acceleratorCount := 0
		if s, err := strconv.Atoi(strings.ReplaceAll(row[19], " ", "")); err == nil {
			// Enforce bounds for uint8 before conversion
			if s < 0 || s > 255 {
				acceleratorCount = 0 // or choose another safe default, e.g., 255
			} else {
				acceleratorCount = s
			}
		}
		acceleratorMemoryGB := 0.0
		if s, err := strconv.ParseFloat(strings.ReplaceAll(row[20], " ", ""), 32); err == nil {
			acceleratorMemoryGB = s
		}
		description := row[21]
		infraType := strings.ToLower(row[22])
		costPerHour, err := strconv.ParseFloat(strings.ReplaceAll(row[3], " ", ""), 32)
		currency := strings.ToUpper(row[4])

		if err != nil {
			log.Error().Msgf("Not valid CostPerHour value in the asset: %s", specInfoId)
			costPerHour = -1
		} else {
			costPerHour = float64(common.ConvertToBaseCurrency(float32(costPerHour), currency))
		}
		evaluationScore01, err := strconv.ParseFloat(strings.ReplaceAll(row[5], " ", ""), 32)
		if err != nil {
			evaluationScore01 = -1
		}
		evaluationScore02, err := strconv.ParseFloat(strings.ReplaceAll(row[6], " ", ""), 32)
		if err != nil {
			evaluationScore02 = -1
		}
		evaluationScore03, err := strconv.ParseFloat(strings.ReplaceAll(row[7], " ", ""), 32)
		if err != nil {
			evaluationScore03 = -1
		}
		evaluationScore04, err := strconv.ParseFloat(strings.ReplaceAll(row[8], " ", ""), 32)
		if err != nil {
			evaluationScore04 = -1
		}
		evaluationScore05, err := strconv.ParseFloat(strings.ReplaceAll(row[9], " ", ""), 32)
		if err != nil {
			evaluationScore05 = -1
		}
		evaluationScore06, err := strconv.ParseFloat(strings.ReplaceAll(row[10], " ", ""), 32)
		if err != nil {
			evaluationScore06 = -1
		}
		evaluationScore07, err := strconv.ParseFloat(strings.ReplaceAll(row[11], " ", ""), 32)
		if err != nil {
			evaluationScore07 = -1
		}
		evaluationScore08, err := strconv.ParseFloat(strings.ReplaceAll(row[12], " ", ""), 32)
		if err != nil {
			evaluationScore08 = -1
		}
		evaluationScore09, err := strconv.ParseFloat(strings.ReplaceAll(row[13], " ", ""), 32)
		if err != nil {
			evaluationScore09 = -1
		}
		evaluationScore10, err := strconv.ParseFloat(strings.ReplaceAll(row[14], " ", ""), 32)
		if err != nil {
			evaluationScore10 = -1
		}

		expandedInfraType := expandInfraType(infraType)

		specInfo.Namespace = nsId
		specInfo.Id = specInfoId
		specInfo.Name = specInfoId
		specInfo.ProviderName = providerName
		specInfo.RegionName = regionName
		specInfo.CspSpecName = cspSpecName
		specInfo.CostPerHour = float32(costPerHour)
		specInfo.RootDiskType = rootDiskType
		specInfo.RootDiskSize = rootDiskSize
		specInfo.AcceleratorType = acceleratorType
		specInfo.AcceleratorModel = acceleratorModel
		specInfo.AcceleratorCount = uint8(acceleratorCount)
		specInfo.AcceleratorMemoryGB = float32(acceleratorMemoryGB)
		specInfo.EvaluationScore01 = float32(evaluationScore01)
		specInfo.EvaluationScore02 = float32(evaluationScore02)
		specInfo.EvaluationScore03 = float32(evaluationScore03)
		specInfo.EvaluationScore04 = float32(evaluationScore04)
		specInfo.EvaluationScore05 = float32(evaluationScore05)
		specInfo.EvaluationScore06 = float32(evaluationScore06)
		specInfo.EvaluationScore07 = float32(evaluationScore07)
		specInfo.EvaluationScore08 = float32(evaluationScore08)
		specInfo.EvaluationScore09 = float32(evaluationScore09)
		specInfo.EvaluationScore10 = float32(evaluationScore10)
		specInfo.Description = description
		specInfo.SystemLabel = model.StrFromAssets
		specInfo.InfraType = expandedInfraType

		//log.Debug().Msgf("Processing row %d: %s-%s-%s", i+1, specInfo.ProviderName, specInfo.RegionName, specInfo.CspSpecName)

		// Check if spec exists in memory map (O(1) lookup)
		if existingSpec, exists := existingSpecsMap[specInfo.Id]; exists {
			// Existing spec found - merge with CSV data
			// log.Debug().Msgf("Found existing spec: %s, merging with CSV data", specInfo.Id)
			mergedSpec := mergeSpecWithCSVData(existingSpec, specInfo)
			specList = append(specList, mergedSpec)
		} else {
			// Spec not found in DB - try LookupSpec from CSP
			log.Debug().Msgf("Spec %s not found in DB, recommended to remove from assets", specInfo.Id)
		}
		// clear memory for specInfo
		specInfo = model.SpecInfo{}
	}
	existingSpecsMap = nil
	runtime.GC()

	// Update database with bulk operation
	if len(specList) > 0 {
		err = RegisterSpecWithInfoInBulk(specList)
		if err != nil {
			log.Error().Err(err).Msg("RegisterSpecWithInfoInBulk failed")
			return err
		}
		log.Info().Msgf("Updated %d specs from asset file", len(specList))
	} else {
		log.Warn().Msg("No specs were processed from the asset file")
	}
	specList = nil
	runtime.GC()

	return nil
}

// loadAllSpecsIntoMemory loads all existing specs for a namespace into a map for O(1) lookup
func loadAllSpecsIntoMemory(nsId string) (map[string]model.SpecInfo, error) {
	var allSpecs []model.SpecInfo

	// Single query to get all specs for the namespace
	result := model.ORM.Where("namespace = ?", nsId).Find(&allSpecs)
	if result.Error != nil {
		return nil, result.Error
	}

	// Build map for O(1) lookup using spec ID as key
	specsMap := make(map[string]model.SpecInfo, len(allSpecs))
	for _, spec := range allSpecs {
		specsMap[spec.Id] = spec
	}

	log.Debug().Msgf("Loaded %d existing specs into memory for namespace %s", len(allSpecs), nsId)
	return specsMap, nil
}

// mergeSpecWithCSVData merges CSV spec data into existing spec (CSV data has priority for non-empty values)
func mergeSpecWithCSVData(existingSpec model.SpecInfo, csvSpec model.SpecInfo) model.SpecInfo {
	mergedSpec := existingSpec

	// Merge cost information (existingSpec priority)
	// If existingSpec.CostPerHour is -1 or 0, use CSV value
	if existingSpec.CostPerHour <= 0 {
		mergedSpec.CostPerHour = csvSpec.CostPerHour
	}

	// Merge evaluation scores (existingSpec priority)
	if existingSpec.EvaluationScore01 <= 0 {
		mergedSpec.EvaluationScore01 = csvSpec.EvaluationScore01
	}
	if existingSpec.EvaluationScore02 <= 0 {
		mergedSpec.EvaluationScore02 = csvSpec.EvaluationScore02
	}
	if existingSpec.EvaluationScore03 <= 0 {
		mergedSpec.EvaluationScore03 = csvSpec.EvaluationScore03
	}
	if existingSpec.EvaluationScore04 <= 0 {
		mergedSpec.EvaluationScore04 = csvSpec.EvaluationScore04
	}
	if existingSpec.EvaluationScore05 <= 0 {
		mergedSpec.EvaluationScore05 = csvSpec.EvaluationScore05
	}
	if existingSpec.EvaluationScore06 <= 0 {
		mergedSpec.EvaluationScore06 = csvSpec.EvaluationScore06
	}
	if existingSpec.EvaluationScore07 <= 0 {
		mergedSpec.EvaluationScore07 = csvSpec.EvaluationScore07
	}
	if existingSpec.EvaluationScore08 <= 0 {
		mergedSpec.EvaluationScore08 = csvSpec.EvaluationScore08
	}
	if existingSpec.EvaluationScore09 <= 0 {
		mergedSpec.EvaluationScore09 = csvSpec.EvaluationScore09
	}
	if existingSpec.EvaluationScore10 <= 0 {
		mergedSpec.EvaluationScore10 = csvSpec.EvaluationScore10
	}

	// Merge disk specifications (existingSpec priority for non-empty values)
	if existingSpec.RootDiskType == "" {
		mergedSpec.RootDiskType = csvSpec.RootDiskType
	}
	if existingSpec.RootDiskSize <= 0 {
		mergedSpec.RootDiskSize = csvSpec.RootDiskSize
	}

	// Merge accelerator specifications (existingSpec priority)
	if existingSpec.AcceleratorModel == "" {
		mergedSpec.AcceleratorModel = csvSpec.AcceleratorModel
	}
	if existingSpec.AcceleratorCount <= 0 {
		mergedSpec.AcceleratorCount = csvSpec.AcceleratorCount
	}
	if existingSpec.AcceleratorMemoryGB <= 0 {
		mergedSpec.AcceleratorMemoryGB = csvSpec.AcceleratorMemoryGB
	}

	if existingSpec.Description == "" {
		mergedSpec.Description = csvSpec.Description
	}
	if existingSpec.InfraType == "" {
		mergedSpec.InfraType = csvSpec.InfraType
	}

	// Always update SystemLabel to indicate data source
	mergedSpec.SystemLabel = model.StrFromAssets

	return mergedSpec
}
