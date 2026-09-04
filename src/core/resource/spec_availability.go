/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	cspcheck "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

// pickAnyConnectionForProvider returns the name of any verified connection
// configured for the given provider, scoped to the credential holder carried
// by ctx. Returns an error when no such connection exists. Used to avoid
// hardcoding a specific connection name when calling region-agnostic CSP APIs
// that only need a credential carrier.
//
// The provider string may be a derived CSP name (e.g. "openstack-new01"); it
// is normalized via csp.ResolveCloudPlatform before comparison so that
// derived connections still match the canonical provider.
func pickAnyConnectionForProvider(ctx context.Context, provider string) (string, error) {
	credentialHolder := common.CredentialHolderFromContext(ctx)
	list, err := common.GetConnConfigList(credentialHolder, true, false)
	if err != nil {
		return "", err
	}
	normalized := csp.ResolveCloudPlatform(provider)
	for _, c := range list.Connectionconfig {
		if csp.ResolveCloudPlatform(c.ProviderName) == normalized {
			return c.ConfigName, nil
		}
	}
	return "", fmt.Errorf("no verified connection configured for provider %q (credentialHolder=%q)", provider, credentialHolder)
}

// GetAvailableRegionZonesForSpec queries the availability of a specific spec across all regions/zones
// Returns detailed availability information including regions, zones, and query performance metrics.
// ctx carries the x-credential-holder so the underlying connection lookup is
// scoped to the requesting tenant rather than the system default holder.
func GetAvailableRegionZonesForSpec(ctx context.Context, provider string, cspSpecName string) (model.SpecAvailabilityInfo, error) {
	startTime := time.Now()

	// Normalize derived CSP names (e.g. "openstack-new01" -> "openstack") so
	// downstream provider comparisons and connection lookups behave consistently.
	provider = csp.ResolveCloudPlatform(provider)

	result := model.SpecAvailabilityInfo{
		Provider:    provider,
		CspSpecName: cspSpecName,
		Success:     false,
	}

	// Currently only Alibaba Cloud is supported
	if !strings.EqualFold(provider, csp.Alibaba) {
		result.ErrorMessage = fmt.Sprintf("Provider %s is not supported yet. Currently only Alibaba Cloud is supported.", provider)
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}

	// Get any verified connection config for this provider. The legacy
	// hardcoded "alibaba-ap-northeast-1" was fragile because that specific
	// connection might not be configured in every deployment. The Alibaba
	// AnyCall (GetInstanceTypeAvailableAllZones) returns all-region results
	// regardless of which regional connection is used to call it, so any
	// connection of the same provider works as a credential carrier.
	connectionName, err := pickAnyConnectionForProvider(ctx, provider)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("No usable connection found for provider %s: %v", provider, err)
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, err
	}
	connConfig, err := common.GetConnConfig(connectionName)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get connection config for %s: %v", connectionName, err)
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// Prepare API call
	client := clientManager.NewHttpClient()
	client.SetTimeout(120 * time.Second)
	url := model.SpiderRestUrl + "/anycall"
	method := "POST"

	requestBody := map[string]any{
		"ConnectionName": connConfig.ConfigName,
		"ReqInfo": map[string]any{
			"FID": "GetInstanceTypeAvailableAllZones",
			"IKeyValueList": []map[string]string{
				{"Key": "InstanceType", "Value": cspSpecName},
			},
		},
	}

	// Make API call
	var apiResponse map[string]any
	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&apiResponse,
		clientManager.VeryLongDuration,
	)

	if err != nil {
		result.ErrorMessage = fmt.Sprintf("API call failed: %v", err)
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// Parse response
	oKeyValueList, ok := apiResponse["OKeyValueList"].([]any)
	if !ok {
		result.ErrorMessage = "Invalid API response format: OKeyValueList not found"
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}

	var availableZones string
	var queryResult string

	for _, item := range oKeyValueList {
		if keyValue, ok := item.(map[string]any); ok {
			key, keyOk := keyValue["Key"].(string)
			value, valueOk := keyValue["Value"].(string)

			if keyOk && valueOk {
				switch key {
				case "AvailableAllZones":
					availableZones = value
				case "Result":
					queryResult = value
				}
			}
		}
	}

	// Check if the query was successful
	if queryResult != "true" {
		result.ErrorMessage = fmt.Sprintf("Spec %s is not available in any zone", cspSpecName)
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, nil // Not an error, just not available
	}

	// Parse available zones
	if availableZones == "" {
		result.ErrorMessage = "No available zones returned"
		result.QueryDurationMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	// Parse region:zone pairs
	// Format: "region1:zone1,region1:zone2,region2:zone1,..."
	regionZoneMap := make(map[string][]string)

	zonePairs := strings.SplitSeq(availableZones, ",")
	for pair := range zonePairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) == 2 {
			region := strings.TrimSpace(parts[0])
			zone := strings.TrimSpace(parts[1])

			if region != "" && zone != "" {
				regionZoneMap[region] = append(regionZoneMap[region], zone)
			}
		}
	}

	// Convert map to structured result
	for region, zones := range regionZoneMap {
		result.AvailableRegions = append(result.AvailableRegions, model.SpecRegionZoneInfo{
			RegionName: region,
			Zones:      zones,
		})
	}

	// Sort regions for consistent output
	sort.Slice(result.AvailableRegions, func(i, j int) bool {
		return result.AvailableRegions[i].RegionName < result.AvailableRegions[j].RegionName
	})

	result.Success = true
	result.QueryDurationMs = time.Since(startTime).Milliseconds()

	log.Debug().
		Str("provider", provider).
		Str("cspSpecName", cspSpecName).
		Int("regions", len(result.AvailableRegions)).
		Int64("durationMs", result.QueryDurationMs).
		Msg("Successfully queried spec availability")

	return result, nil
}

// GetAvailableRegionZonesForSpecList queries availability for multiple specs in parallel
// Returns batch results with performance metrics for all specs.
// ctx carries the x-credential-holder so per-spec connection lookups are
// scoped to the requesting tenant.
func GetAvailableRegionZonesForSpecList(ctx context.Context, provider string, cspSpecNames []string) (model.SpecAvailabilityBatchResult, error) {
	startTime := time.Now()

	result := model.SpecAvailabilityBatchResult{
		Provider:       provider,
		TotalSpecs:     len(cspSpecNames),
		FastestQueryMs: math.MaxInt64,
		SlowestQueryMs: 0,
	}

	if len(cspSpecNames) == 0 {
		result.TotalDurationMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	// Control concurrency - limit parallel queries to avoid overwhelming the API
	maxConcurrent := 100
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Channel to collect results
	resultChan := make(chan model.SpecAvailabilityInfo, len(cspSpecNames))

	// Launch parallel queries
	for _, specName := range cspSpecNames {
		wg.Add(1)
		go func(spec string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			common.RandomSleep(0, 20*1000)

			// Query single spec
			specResult, err := GetAvailableRegionZonesForSpec(ctx, provider, spec)
			if err != nil {
				// Even if there's an error, we want to include the result
				log.Debug().Err(err).Str("spec", spec).Msg("Failed to query spec availability")
			}

			// Send result to channel
			resultChan <- specResult

			// Update performance metrics (thread-safe)
			mutex.Lock()
			if specResult.Success {
				result.SuccessfulQueries++
			} else {
				result.FailedQueries++
			}

			// Update timing statistics
			if specResult.QueryDurationMs < result.FastestQueryMs {
				result.FastestQueryMs = specResult.QueryDurationMs
			}
			if specResult.QueryDurationMs > result.SlowestQueryMs {
				result.SlowestQueryMs = specResult.QueryDurationMs
			}
			mutex.Unlock()
		}(specName)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results
	for specResult := range resultChan {
		result.SpecResults = append(result.SpecResults, specResult)
	}

	// Calculate final metrics
	result.TotalDurationMs = time.Since(startTime).Milliseconds()

	if result.SuccessfulQueries > 0 {
		var totalDuration int64
		for _, spec := range result.SpecResults {
			totalDuration += spec.QueryDurationMs
		}
		result.AverageQueryMs = totalDuration / int64(len(result.SpecResults))
	}

	// Handle case where no queries were successful
	if result.SuccessfulQueries == 0 {
		result.FastestQueryMs = 0
	}

	// Sort results by spec name for consistent output
	sort.Slice(result.SpecResults, func(i, j int) bool {
		return result.SpecResults[i].CspSpecName < result.SpecResults[j].CspSpecName
	})

	log.Info().
		Str("provider", provider).
		Int("totalSpecs", result.TotalSpecs).
		Int("successfulQueries", result.SuccessfulQueries).
		Int("failedQueries", result.FailedQueries).
		Int64("totalDurationMs", result.TotalDurationMs).
		Int64("averageQueryMs", result.AverageQueryMs).
		Msg("Completed batch spec availability query")

	return result, nil
}

// UpdateExistingSpecListByAvailableRegionZones cleans up unavailable specs from the database
// Queries all specs for a specific provider across all regions, checks their availability, and removes specs that are not available in their respective regions
func UpdateExistingSpecListByAvailableRegionZones(ctx context.Context, nsId string, provider string) (model.SpecCleanupResult, error) {
	startTime := time.Now()

	result := model.SpecCleanupResult{
		Provider: provider,
		Region:   "all", // Indicates we're processing all regions
	}

	// Currently only Alibaba Cloud is supported
	if !strings.EqualFold(provider, csp.Alibaba) {
		return result, fmt.Errorf("Provider %s is not supported yet. Currently only Alibaba Cloud is supported.", provider)
	}

	log.Info().
		Str("namespace", nsId).
		Str("provider", provider).
		Msg("Starting spec cleanup for all regions of provider")

	// Step 1: Get unique CspSpecNames for the provider using efficient DB query
	var uniqueSpecNames []string

	// Build the ID pattern for the specific provider (all regions)
	// Spec ID format: provider+region+cspSpecName (e.g., "alibaba+ap-northeast-1+ecs.t5.large")
	idPattern := fmt.Sprintf("%s+%%", strings.ToLower(provider))

	// Use DISTINCT to get unique CspSpecNames directly from DB
	queryResult := model.ORM.Model(&model.SpecInfo{}).
		Distinct("csp_spec_name").
		Where("namespace = ? AND LOWER(id) LIKE ?", nsId, idPattern).
		Pluck("csp_spec_name", &uniqueSpecNames)

	if queryResult.Error != nil {
		log.Error().Err(queryResult.Error).
			Str("namespace", nsId).
			Str("idPattern", idPattern).
			Msg("Failed to execute DISTINCT query")
		return result, fmt.Errorf("failed to query unique spec names from database: %v", queryResult.Error)
	}

	if len(uniqueSpecNames) == 0 {
		log.Info().
			Str("provider", provider).
			Str("namespace", nsId).
			Msg("No specs found for the given provider")
		result.CleanupDurationMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	log.Info().
		Int("uniqueSpecNames", len(uniqueSpecNames)).
		Str("provider", provider).
		Msg("Found unique spec names for availability check")

	// Step 2: Query availability for all unique spec names
	availabilityStartTime := time.Now()
	availabilityResult, err := GetAvailableRegionZonesForSpecList(ctx, provider, uniqueSpecNames)
	if err != nil {
		log.Error().Err(err).
			Str("provider", provider).
			Int("specCount", len(uniqueSpecNames)).
			Msg("Failed to query spec availability")
		return result, fmt.Errorf("failed to query spec availability: %v", err)
	}
	result.AvailabilityCheckMs = time.Since(availabilityStartTime).Milliseconds()
	result.AvailabilityResults = availabilityResult

	log.Info().
		Int("totalSpecs", availabilityResult.TotalSpecs).
		Int("successfulQueries", availabilityResult.SuccessfulQueries).
		Int("failedQueries", availabilityResult.FailedQueries).
		Int64("availabilityCheckMs", result.AvailabilityCheckMs).
		Msg("Availability check completed")

	// Step 3: Create availability map for quick lookup
	// Map: specName -> set of available regions
	specAvailabilityMap := make(map[string]map[string]bool)
	var failedSpecs []string

	for _, specResult := range availabilityResult.SpecResults {
		if !specResult.Success {
			// If the query failed or spec is not available anywhere, mark as unavailable everywhere
			specAvailabilityMap[specResult.CspSpecName] = make(map[string]bool)
			failedSpecs = append(failedSpecs, specResult.CspSpecName)
			continue
		}

		availableRegions := make(map[string]bool)
		for _, regionInfo := range specResult.AvailableRegions {
			availableRegions[regionInfo.RegionName] = true
		}
		specAvailabilityMap[specResult.CspSpecName] = availableRegions
	}

	if len(failedSpecs) > 0 {
		log.Warn().
			Int("failedSpecsCount", len(failedSpecs)).
			Strs("failedSpecs", failedSpecs).
			Msg("Some specs marked as unavailable everywhere (query failed)")
	}

	// Step 4: Query specs to be deleted - only get specs that are NOT available in their regions
	var specIdsToDelete []string
	var processedSpecsCount int
	var skippedSpecsCount int

	// Collect deletion information for YAML file
	specsToIgnore := make(map[string]map[string][]string) // provider -> region -> []specNames
	globalIgnoreSpecs := make(map[string][]string)        // provider -> []specNames (specs to ignore in all regions)

	// Initialize provider entry
	if specsToIgnore[provider] == nil {
		specsToIgnore[provider] = make(map[string][]string)
	}

	// For each unique spec name, find specs in regions where it's not available
	for _, specName := range uniqueSpecNames {
		availableRegions, exists := specAvailabilityMap[specName]
		if !exists {
			// If availability check failed, delete all instances of this spec
			var specsToDelete []model.SpecInfo
			deleteQuery := model.ORM.Select("id").
				Where("namespace = ? AND LOWER(id) LIKE ? AND csp_spec_name = ?",
					nsId, idPattern, specName).
				Find(&specsToDelete)

			if deleteQuery.Error != nil {
				log.Error().Err(deleteQuery.Error).
					Str("specName", specName).
					Msg("Failed to query specs for deletion")
				skippedSpecsCount++
				continue
			}

			for _, spec := range specsToDelete {
				specIdsToDelete = append(specIdsToDelete, spec.Id)
			}

			// Add to global ignore list (spec not available in any region)
			globalIgnoreSpecs[provider] = append(globalIgnoreSpecs[provider], specName)

			processedSpecsCount++
			continue
		}

		// For available spec, find instances in regions where it's NOT available
		var specsInUnavailableRegions []model.SpecInfo

		// Query all instances of this spec name for the provider
		specQuery := model.ORM.Select("id").
			Where("namespace = ? AND LOWER(id) LIKE ? AND csp_spec_name = ?",
				nsId, idPattern, specName).
			Find(&specsInUnavailableRegions)

		if specQuery.Error != nil {
			log.Error().Err(specQuery.Error).
				Str("specName", specName).
				Msg("Failed to query spec instances")
			skippedSpecsCount++
			continue
		}

		var instancesMarkedForDeletion int
		// Check each spec instance and mark for deletion if not available in its region
		for _, spec := range specsInUnavailableRegions {
			// Use the spec's own region field rather than parsing its ID.
			region := spec.RegionName
			if region == "" {
				log.Warn().
					Str("specId", spec.Id).
					Msg("Spec has no region - skipping")
				continue
			}

			// Check if this spec is available in its region
			if !availableRegions[region] {
				// Spec is not available in its region, mark for deletion
				specIdsToDelete = append(specIdsToDelete, spec.Id)
				instancesMarkedForDeletion++

				// Add to region-specific ignore list
				if specsToIgnore[provider][region] == nil {
					specsToIgnore[provider][region] = make([]string, 0)
				}
				// Check if spec is already in the list for this region
				found := slices.Contains(specsToIgnore[provider][region], specName)
				if !found {
					specsToIgnore[provider][region] = append(specsToIgnore[provider][region], specName)
				}
			}
		}

		processedSpecsCount++
	}

	if skippedSpecsCount > 0 {
		log.Warn().
			Int("skippedSpecsCount", skippedSpecsCount).
			Msg("Some specs were skipped due to query errors")
	}

	// Count total specs checked (for reporting)
	var totalSpecCount int64
	countQuery := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND LOWER(id) LIKE ?", nsId, idPattern).
		Count(&totalSpecCount)

	if countQuery.Error != nil {
		log.Warn().Err(countQuery.Error).
			Msg("Failed to count total specs")
		totalSpecCount = int64(len(specIdsToDelete)) // Fallback estimate
	}

	result.TotalSpecsChecked = int(totalSpecCount)
	result.SpecsToDelete = len(specIdsToDelete)

	log.Info().
		Int("specIdsToDelete", len(specIdsToDelete)).
		Int("totalSpecs", int(totalSpecCount)).
		Float64("deletionPercentage", func() float64 {
			if totalSpecCount > 0 {
				return float64(len(specIdsToDelete)) / float64(totalSpecCount) * 100
			}
			return 0
		}()).
		Msg("Identified specs for deletion")

	// Step 5: Delete unavailable specs from database
	if len(specIdsToDelete) > 0 {
		log.Info().
			Int("specIdsToDeleteCount", len(specIdsToDelete)).
			Msg("Starting database deletion")

		deleteStartTime := time.Now()
		deleteResult := model.ORM.Where("namespace = ? AND id IN ?", nsId, specIdsToDelete).Delete(&model.SpecInfo{})
		deleteDuration := time.Since(deleteStartTime)

		if deleteResult.Error != nil {
			log.Error().Err(deleteResult.Error).
				Int("requestedDeletions", len(specIdsToDelete)).
				Int64("deleteDurationMs", deleteDuration.Milliseconds()).
				Msg("Failed to delete specs from database")
			// Record failed deletions
			result.FailedDeletions = specIdsToDelete
			return result, fmt.Errorf("failed to delete specs: %v", deleteResult.Error)
		}

		result.SpecsDeleted = int(deleteResult.RowsAffected)

		if result.SpecsDeleted != len(specIdsToDelete) {
			log.Warn().
				Int("requestedDeletions", len(specIdsToDelete)).
				Int("actualDeletions", result.SpecsDeleted).
				Msg("Mismatch between requested and actual deletions")
		}

		log.Info().
			Int("deletedCount", result.SpecsDeleted).
			Int("requestedCount", len(specIdsToDelete)).
			Msg("Successfully deleted unavailable specs")

		// Log a few examples of deleted specs for reference
		if len(specIdsToDelete) > 0 {
			exampleCount := min(len(specIdsToDelete), 5)
			log.Debug().
				Strs("exampleDeletedSpecs", specIdsToDelete[:exampleCount]).
				Msg("Examples of deleted specs")
		}
	} else {
		log.Info().
			Str("provider", provider).
			Str("namespace", nsId).
			Msg("No specs identified for deletion - all specs are available in their regions")
	}

	// Step 6: Log deletion information in structured format and prepare API response
	logSpecsToIgnoreInfo(provider, specsToIgnore, globalIgnoreSpecs)

	// Prepare specs to ignore info for API response
	specsToIgnoreData := &model.SpecsToIgnoreData{
		LastUpdated:          time.Now(),
		Description:          "Specs that should be ignored during availability checks. Global specs are unavailable in all regions, region-specific specs are unavailable only in specific regions.",
		GlobalIgnoreSpecs:    make(map[string][]string),
		RegionSpecificIgnore: make(map[string]map[string][]string),
	}

	// Populate global ignore specs for API response
	for cspProvider, specs := range globalIgnoreSpecs {
		if len(specs) > 0 {
			sort.Strings(specs)
			specsToIgnoreData.GlobalIgnoreSpecs[cspProvider] = specs
		}
	}

	// Populate region-specific ignore specs for API response
	for cspProvider, regions := range specsToIgnore {
		if specsToIgnoreData.RegionSpecificIgnore[cspProvider] == nil {
			specsToIgnoreData.RegionSpecificIgnore[cspProvider] = make(map[string][]string)
		}

		for region, specs := range regions {
			if len(specs) > 0 {
				sort.Strings(specs)
				specsToIgnoreData.RegionSpecificIgnore[cspProvider][region] = specs
			}
		}
	}

	result.SpecsToIgnoreInfo = specsToIgnoreData
	result.CleanupDurationMs = time.Since(startTime).Milliseconds()

	log.Info().
		Str("provider", provider).
		Int("totalChecked", result.TotalSpecsChecked).
		Int("toDelete", result.SpecsToDelete).
		Int("deleted", result.SpecsDeleted).
		Int64("totalDurationMs", result.CleanupDurationMs).
		Int64("availabilityCheckMs", result.AvailabilityCheckMs).
		Msg("Completed spec cleanup for all regions")

	return result, nil
}

// EnsureSpecAvailable resolves a CSP spec (by its CSP spec name on the given connection) to
// a TB SpecInfo, registering it into SystemCommonNs on demand when it is not already in the DB.
// It mirrors EnsureImageAvailable so a discovered VM can be registered with a real spec instead
// of an unresolved placeholder. The bool return reports whether the spec was auto-registered.
// Price/cost is not fetched here (that is a connection-wide bulk operation); the on-demand spec
// carries the canonical key, so the regular price-fetch flow fills its cost when it next runs.
func EnsureSpecAvailable(connectionName, cspSpecName string) (model.SpecInfo, bool, error) {
	if connectionName == "" {
		return model.SpecInfo{}, false, fmt.Errorf("connectionName is required for EnsureSpecAvailable")
	}
	if cspSpecName == "" {
		return model.SpecInfo{}, false, fmt.Errorf("cspSpecName is required for EnsureSpecAvailable")
	}

	connConfig, err := common.GetConnConfig(connectionName)
	if err != nil {
		return model.SpecInfo{}, false, fmt.Errorf("cannot GetConnConfig for %s: %w", connectionName, err)
	}

	// Same key the bulk fetch path uses, so an on-demand spec is indistinguishable from a
	// fetched one and later price updates match it by key.
	specKey := GetProviderRegionZoneResourceKey(connConfig.ProviderName, connConfig.RegionDetail.RegionName, "", cspSpecName)

	if spec, err := GetSpec(model.SystemCommonNs, specKey); err == nil {
		return spec, false, nil
	}

	log.Info().Msgf("Spec '%s' not in DB; fetching from CSP and registering on demand", specKey)
	spec, err := RegisterSpecWithCspResourceId(model.SystemCommonNs, &model.SpecReq{
		Name:           specKey,
		ConnectionName: connectionName,
		CspSpecName:    cspSpecName,
	}, false)
	if err != nil {
		return model.SpecInfo{}, false, fmt.Errorf("failed to fetch/register spec '%s' from CSP: %w", cspSpecName, err)
	}
	return spec, true, nil
}

const (
	ZoneErrorCodeNone                  = ""
	ZoneErrorCodeSpecNotFound          = "SPEC_NOT_FOUND"
	ZoneErrorCodeProviderNotAvailable  = "PROVIDER_NOT_AVAILABLE"
	ZoneErrorCodeRegionNotAvailable    = "REGION_NOT_AVAILABLE"
	ZoneErrorCodeNoVerifiedZones       = "NO_VERIFIED_ZONES"
	ZoneErrorCodeNoZonesAfterFiltering = "NO_ZONES_AFTER_FILTERING"
	ZoneErrorCodeInternalError         = "INTERNAL_ERROR"
)

// GetAvailableZonesForSpec queries available (verified) zones for a specific spec ID
// It uses connection configs to determine which zones are verified and available.
// For Alibaba Cloud, it additionally filters zones using CSP API to check spec availability.
//
// Parameters:
//   - ctx: Request context (contains credential holder info)
//   - specId: TB spec ID (format: provider+region+cspSpecName)
//
// Returns:
//   - *model.AvailableZonesInfo: Success result with available zones (nil if error)
//   - *model.AvailableZonesError: Error result with details (nil if success)
func GetAvailableZonesForSpec(ctx context.Context, specId string) (*model.AvailableZonesInfo, *model.AvailableZonesError) {
	startTime := time.Now()

	// Get credential holder from context
	credentialHolder := common.CredentialHolderFromContext(ctx)

	// Helper function to create error response
	makeError := func(errorCode, errorMessage, suggestion string, alternativeRegions []string) *model.AvailableZonesError {
		return &model.AvailableZonesError{
			SpecId:             specId,
			ErrorCode:          errorCode,
			ErrorMessage:       errorMessage,
			Suggestion:         suggestion,
			AlternativeRegions: alternativeRegions,
			QueryDurationMs:    time.Since(startTime).Milliseconds(),
		}
	}

	// Step 1: Get spec information using GetSpec
	// Use "system" namespace as specs are stored in system namespace
	specInfo, err := GetSpec("system", specId)
	if err != nil {
		return nil, makeError(
			ZoneErrorCodeSpecNotFound,
			fmt.Sprintf("Spec '%s' not found: %v", specId, err),
			"Verify the spec ID format (provider+region+cspSpecName) and ensure the spec exists in the system namespace. Use /tumblebug/ns/system/resources/spec to list available specs.",
			nil,
		)
	}

	// Step 2: Get all connection configs filtered by credential holder and verified status
	connConfigs, err := common.GetConnConfigList(credentialHolder, true, false)
	if err != nil {
		return nil, makeError(
			ZoneErrorCodeInternalError,
			fmt.Sprintf("Failed to get connection configs: %v", err),
			"",
			nil,
		)
	}

	if len(connConfigs.Connectionconfig) == 0 {
		return nil, makeError(
			ZoneErrorCodeProviderNotAvailable,
			fmt.Sprintf("No verified connection configs found for credential holder '%s'", credentialHolder),
			"Register credentials for the desired cloud provider using the credential registration API.",
			nil,
		)
	}

	// Step 3: Filter connection configs by provider
	var providerConfigs []model.ConnConfig
	var allAvailableProviders []string
	providerSet := make(map[string]bool)

	for _, cc := range connConfigs.Connectionconfig {
		if !providerSet[cc.ProviderName] {
			providerSet[cc.ProviderName] = true
			allAvailableProviders = append(allAvailableProviders, cc.ProviderName)
		}
		if strings.EqualFold(cc.ProviderName, specInfo.ProviderName) {
			providerConfigs = append(providerConfigs, cc)
		}
	}

	if len(providerConfigs) == 0 {
		return nil, makeError(
			ZoneErrorCodeProviderNotAvailable,
			fmt.Sprintf("Provider '%s' is not available. No verified connection configs found for this provider.", specInfo.ProviderName),
			fmt.Sprintf("Available providers: %v. Register credentials for '%s' or choose a spec from an available provider.", allAvailableProviders, specInfo.ProviderName),
			nil,
		)
	}

	// Step 4: Filter connection configs by region
	var regionConfigs []model.ConnConfig
	var allAvailableRegions []string
	regionSet := make(map[string]bool)

	for _, cc := range providerConfigs {
		regionName := cc.RegionZoneInfo.AssignedRegion
		if !regionSet[regionName] {
			regionSet[regionName] = true
			allAvailableRegions = append(allAvailableRegions, regionName)
		}
		if strings.EqualFold(regionName, specInfo.RegionName) {
			regionConfigs = append(regionConfigs, cc)
		}
	}

	if len(regionConfigs) == 0 {
		return nil, makeError(
			ZoneErrorCodeRegionNotAvailable,
			fmt.Sprintf("Region '%s' is not available for provider '%s'. No verified connection configs found for this region.", specInfo.RegionName, specInfo.ProviderName),
			fmt.Sprintf("Available regions for %s: %v. Choose a spec from an available region.", specInfo.ProviderName, allAvailableRegions),
			allAvailableRegions,
		)
	}

	// Step 5: Get zone list from cloudinfo.yaml (authoritative source)
	// Connection configs are registered per-region (one conn config per region, not per zone),
	// so extracting zones from conn configs yields only the representative zone.
	// cloudinfo.yaml contains the complete zone list for each region.
	//
	// NOTE: "verified" in the variable name below refers only to the region being accessible
	// via at least one verified connection config (confirmed in Steps 3-4 above).
	// It does NOT mean every individual zone was independently verified against the CSP.
	// When cloudinfo.yaml is available, this slice contains the full known zone list
	// for the region; for Alibaba, Step 6 further filters it via CSP API.

	// Check whether this CSP uses empty representative zone (e.g., Azure).
	// When useEmptyRepresentativeZone is true, callers must NOT specify a zone —
	// doing so can cause OverconstrainedZonalAllocationRequest errors for GPU VMs.
	// Return auto-selection (HasZoneConcept: false) for such providers regardless
	// of whether cloudinfo.yaml lists zones for the region.
	cloudInfo, cloudInfoErr := common.GetCloudInfo()
	if cloudInfoErr == nil {
		if cspDetail, ok := cloudInfo.CSPs[strings.ToLower(specInfo.ProviderName)]; ok {
			if cspDetail.UseEmptyRepresentativeZone {
				log.Info().
					Str("specId", specId).
					Str("provider", specInfo.ProviderName).
					Str("region", specInfo.RegionName).
					Msg("CSP uses empty representative zone (useEmptyRepresentativeZone=true), returning auto-selection")
				return &model.AvailableZonesInfo{
					SpecId:           specId,
					ProviderName:     specInfo.ProviderName,
					RegionName:       specInfo.RegionName,
					CspSpecName:      specInfo.CspSpecName,
					CredentialHolder: credentialHolder,
					AvailableZones:   []string{},
					HasZoneConcept:   false,
					QueryDurationMs:  time.Since(startTime).Milliseconds(),
				}, nil
			}
		}
	}

	regionDetail, err := common.GetRegion(specInfo.ProviderName, specInfo.RegionName)
	if err != nil {
		log.Warn().Err(err).
			Str("provider", specInfo.ProviderName).
			Str("region", specInfo.RegionName).
			Msg("Failed to get region detail from cloudinfo, falling back to conn config zones")
	}

	var verifiedZones []string
	if err == nil && len(regionDetail.Zones) > 0 {
		// Use the complete known zone list for the verified region from cloudinfo.yaml.
		verifiedZones = make([]string, len(regionDetail.Zones))
		copy(verifiedZones, regionDetail.Zones)
		sort.Strings(verifiedZones)
	} else {
		// Fallback: derive zones from verified region-level conn configs (legacy behavior).
		zoneSet := make(map[string]bool)
		hasEmptyZone := false
		for _, cc := range regionConfigs {
			zoneName := cc.RegionZoneInfo.AssignedZone
			if zoneName == "" {
				hasEmptyZone = true
				continue
			}
			if !zoneSet[zoneName] {
				zoneSet[zoneName] = true
				verifiedZones = append(verifiedZones, zoneName)
			}
		}
		sort.Strings(verifiedZones)

		if len(verifiedZones) == 0 && hasEmptyZone {
			// Zone concept might not exist for this provider/region
			log.Info().
				Str("specId", specId).
				Str("provider", specInfo.ProviderName).
				Str("region", specInfo.RegionName).
				Msg("No zone concept for this provider/region, auto-selection will be used")

			return &model.AvailableZonesInfo{
				SpecId:           specId,
				ProviderName:     specInfo.ProviderName,
				RegionName:       specInfo.RegionName,
				CspSpecName:      specInfo.CspSpecName,
				CredentialHolder: credentialHolder,
				AvailableZones:   []string{}, // Empty array means auto-selection
				HasZoneConcept:   false,
				QueryDurationMs:  time.Since(startTime).Milliseconds(),
			}, nil
		}
	}

	if len(verifiedZones) == 0 {
		return nil, makeError(
			ZoneErrorCodeNoVerifiedZones,
			fmt.Sprintf("No verified zones found for provider '%s' region '%s'", specInfo.ProviderName, specInfo.RegionName),
			"Verify that connection configs for this region have been properly verified. Check the connection verification status.",
			nil,
		)
	}

	// Prepare success result
	result := &model.AvailableZonesInfo{
		SpecId:           specId,
		ProviderName:     specInfo.ProviderName,
		RegionName:       specInfo.RegionName,
		CspSpecName:      specInfo.CspSpecName,
		CredentialHolder: credentialHolder,
		AllVerifiedZones: verifiedZones,
		HasZoneConcept:   true,
	}

	// Step 6: Narrow verified zones to those where the CSP actually offers the spec, using the
	// per-CSP availability checker (AWS DescribeInstanceTypeOfferings, Alibaba
	// DescribeAvailableResource, Azure/GCP/Tencent, ...). Applies to every provider — the helper
	// falls back to all verified zones when there is no checker or no per-zone data, so providers
	// without a checker keep the previous behavior. This makes the region zone list from
	// cloudinfo.yaml accurate (previously only Alibaba was filtered, so e.g. AWS listed zones that
	// do not actually offer the instance type).
	availableZones, unavailableZones, filterErr := filterZonesBySpecAvailability(ctx, specInfo.ProviderName, specInfo.CspSpecName, specInfo.RegionName, verifiedZones)
	if filterErr != nil {
		// Log warning but don't fail - return all verified zones
		log.Warn().Err(filterErr).
			Str("specId", specId).
			Str("provider", specInfo.ProviderName).
			Msg("Zone availability filtering failed, returning all verified zones")
		result.AvailableZones = verifiedZones
	} else if len(availableZones) == 0 {
		return nil, makeError(
			ZoneErrorCodeNoZonesAfterFiltering,
			fmt.Sprintf("Spec '%s' is not available in any verified zone for region '%s'", specInfo.CspSpecName, specInfo.RegionName),
			"This spec may not be available in the specified region. Try a different spec or region.",
			nil,
		)
	} else {
		result.AvailableZones = availableZones
		result.UnavailableZones = unavailableZones
	}

	result.QueryDurationMs = time.Since(startTime).Milliseconds()

	log.Debug().
		Str("specId", specId).
		Str("provider", specInfo.ProviderName).
		Str("region", specInfo.RegionName).
		Int("availableZones", len(result.AvailableZones)).
		Int64("durationMs", result.QueryDurationMs).
		Msg("Successfully queried available zones for spec")

	return result, nil
}

// filterZonesBySpecAvailability narrows verifiedZones to those where the CSP actually offers the
// spec, using the provider-agnostic availability dispatcher (csp.CheckAvailability), which routes
// to the per-CSP checker (AWS DescribeInstanceTypeOfferings, Alibaba DescribeAvailableResource,
// Azure/GCP/Tencent, ...). This is the single accurate source of per-AZ instance availability.
//
// The dispatcher path is:
//   - Faster: single-region API call vs. global sweep
//   - Cached: 5-min TTL + singleflight, so a prior ReviewSpecImagePair call
//     for the same (region, instanceType) is reused at zero cost
//   - Consistent: same data source as provisioning/review and the popup's pair-review section
//
// When there is no checker for the provider, the checker errored, or it returned no per-zone
// breakdown, this falls back to "all verified zones available" (best-effort) so we never hide
// zones the user might actually be able to use.
func filterZonesBySpecAvailability(ctx context.Context, provider string, cspSpecName string, regionName string, verifiedZones []string) (availableZones []string, unavailableZones []string, err error) {
	availability := cspcheck.CheckAvailability(ctx, model.AvailabilityQuery{
		Provider:     csp.ResolveCloudPlatform(provider),
		Region:       regionName,
		InstanceType: cspSpecName,
	})

	// No trustworthy per-zone data: missing/errored checker, or a checker that reports the spec
	// as available without a per-zone breakdown. Fall back to all verified zones (best-effort).
	// (When the spec is genuinely not offered anywhere, Available is false with empty Zones, and
	// we intentionally do NOT fall back — the caller then reports "not available in any zone".)
	noPerZoneData := availability.Source == "none" ||
		strings.HasSuffix(availability.Source, ":error") ||
		(availability.Available && len(availability.Zones) == 0)
	if noPerZoneData {
		log.Warn().
			Str("source", availability.Source).
			Str("reason", availability.Reason).
			Str("provider", provider).
			Str("instanceType", cspSpecName).
			Str("region", regionName).
			Msg("availability dispatcher returned no per-zone data; treating all verified zones as available")
		return verifiedZones, nil, nil
	}

	// Build the set of zones the CSP currently reports as available
	// (WithStock + at least one supported disk).
	cspZoneSet := make(map[string]bool)
	for _, z := range availability.Zones {
		if z.Available {
			cspZoneSet[z.ZoneId] = true
		}
	}

	// Filter verified zones based on CSP availability.
	for _, zone := range verifiedZones {
		if cspZoneSet[zone] {
			availableZones = append(availableZones, zone)
		} else {
			unavailableZones = append(unavailableZones, zone)
		}
	}

	return availableZones, unavailableZones, nil
}
