/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package resource

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	alibabaPricing "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	awsPricing "github.com/cloud-barista/cb-tumblebug/src/core/csp/aws"
	azurePricing "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"
	gcpPricing "github.com/cloud-barista/cb-tumblebug/src/core/csp/gcp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

// filterConnConfigsByProvider keeps targetProviders only; otherwise drops excludedProviders.
func filterConnConfigsByProvider(configs []model.ConnConfig, targetProviders, excludedProviders []string) []model.ConnConfig {
	if len(targetProviders) == 0 && len(excludedProviders) == 0 {
		return configs
	}
	matches := func(provider string, list []string) bool {
		for _, p := range list {
			if strings.EqualFold(provider, p) {
				return true
			}
		}
		return false
	}
	filtered := make([]model.ConnConfig, 0, len(configs))
	for _, c := range configs {
		if len(targetProviders) > 0 {
			if matches(c.ProviderName, targetProviders) {
				filtered = append(filtered, c)
			}
			continue
		}
		if !matches(c.ProviderName, excludedProviders) {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// FetchPriceForAllConnConfigs gets all conn configs from Spider, lookups all Price for each region of conn config,
// and saves into TB Price objects. This implementation uses parallel processing with concurrency control and retries failed connections once.
func FetchPriceForAllConnConfigs(option *model.PriceFetchOption) (connConfigCount uint, priceCount uint, err error) {
	// Get connection configurations
	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get connection config list")
		return 0, 0, err
	}

	if option == nil {
		option = &model.PriceFetchOption{}
	}
	targetConfigs := filterConnConfigsByProvider(connConfigs.Connectionconfig, option.TargetProviders, option.ExcludedProviders)

	// Skip processing if no connections found
	if len(targetConfigs) == 0 {
		log.Info().Msg("No connection configurations found")
		return 0, 0, nil
	}

	connConfigCount = uint(len(targetConfigs))
	log.Info().Msgf("Starting parallel price fetching for %d connections", connConfigCount)

	startTime := time.Now()

	// Separate CSPs that have a direct pricing path from the Spider-based fallback.
	// GCP: direct because Spider's GCP pricing scraper is broken.
	// Azure: direct because it is faster and avoids Spider's per-region HTTP overhead.
	// AWS: direct because a single global Pricing API query covers all regions at once,
	//      eliminating the N×Spider round-trips that dominate the legacy path.
	var gcpConfigs []model.ConnConfig
	var azureConfigs []model.ConnConfig
	var awsConfigs []model.ConnConfig
	var alibabaConfigs []model.ConnConfig
	var otherConfigs []model.ConnConfig
	for _, c := range targetConfigs {
		switch {
		case strings.EqualFold(c.ProviderName, csp.GCP):
			gcpConfigs = append(gcpConfigs, c)
		case strings.EqualFold(c.ProviderName, csp.Azure):
			azureConfigs = append(azureConfigs, c)
		case strings.EqualFold(c.ProviderName, csp.AWS):
			awsConfigs = append(awsConfigs, c)
		case strings.EqualFold(c.ProviderName, csp.Alibaba):
			alibabaConfigs = append(alibabaConfigs, c)
		default:
			otherConfigs = append(otherConfigs, c)
		}
	}

	var totalSuccessCount uint
	var allErrors []string

	// Run provider groups concurrently (up to 3 at a time) and aggregate results safely.
	type providerTask struct {
		name string
		run  func() (uint, []string)
	}
	type providerFetchResult struct {
		provider string
		success  uint
		errors   []string
		elapsed  time.Duration
	}

	tasks := make([]providerTask, 0, 5)
	// Alibaba is placed first: it has the strictest API rate limits and takes the
	// longest per-region due to mandatory inter-call intervals. Starting it immediately
	// (in the initial semaphore burst) avoids it becoming the tail that extends the
	// overall price-fetch duration.
	if len(alibabaConfigs) > 0 {
		tasks = append(tasks, providerTask{
			name: csp.Alibaba,
			run: func() (uint, []string) {
				return fetchAlibabaPricesDirect(alibabaConfigs)
			},
		})
	}
	if len(gcpConfigs) > 0 {
		tasks = append(tasks, providerTask{
			name: csp.GCP,
			run: func() (uint, []string) {
				return fetchGCPPricesDirect(gcpConfigs)
			},
		})
	}
	if len(azureConfigs) > 0 {
		tasks = append(tasks, providerTask{
			name: csp.Azure,
			run: func() (uint, []string) {
				return fetchAzurePricesDirect(azureConfigs)
			},
		})
	}
	if len(awsConfigs) > 0 {
		tasks = append(tasks, providerTask{
			name: csp.AWS,
			run: func() (uint, []string) {
				return fetchAWSPricesDirect(awsConfigs)
			},
		})
	}
	if len(otherConfigs) > 0 {
		tasks = append(tasks, providerTask{
			name: "OTHER(SPIDER)",
			run: func() (uint, []string) {
				// Keep existing behavior for Spider path.
				sortConnectionsByCSPRotation(otherConfigs)
				return fetchPricesViaSpider(otherConfigs)
			},
		})
	}

	if len(tasks) > 0 {
		maxProviderConcurrent := 3
		semaphore := make(chan struct{}, maxProviderConcurrent)
		resultChan := make(chan providerFetchResult, len(tasks))
		var wg sync.WaitGroup

		for _, task := range tasks {
			t := task
			wg.Add(1)
			semaphore <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-semaphore }()

				startedAt := time.Now()
				result := providerFetchResult{provider: t.name}

				defer func() {
					if r := recover(); r != nil {
						result.errors = append(result.errors, fmt.Sprintf("provider task panic: %v", r))
					}
					result.elapsed = time.Since(startedAt)
					resultChan <- result
				}()

				success, errs := t.run()
				result.success = success
				if len(errs) > 0 {
					prefixed := make([]string, 0, len(errs))
					for _, e := range errs {
						prefixed = append(prefixed, fmt.Sprintf("[%s] %s", t.name, e))
					}
					result.errors = append(result.errors, prefixed...)
				}
			}()
		}

		go func() {
			wg.Wait()
			close(resultChan)
		}()

		for result := range resultChan {
			totalSuccessCount += result.success
			allErrors = append(allErrors, result.errors...)
			log.Info().Msgf("Provider price fetch completed: provider=%s success=%d errors=%d elapsed=%s",
				result.provider, result.success, len(result.errors), result.elapsed)
		}
	}

	// Report any errors
	if len(allErrors) > 0 {
		log.Warn().Msgf("Encountered %d errors while fetching prices after retries", len(allErrors))
		if len(allErrors) == int(connConfigCount) {
			return connConfigCount, priceCount, fmt.Errorf("all connections failed: %s",
				allErrors[0])
		}
	}

	// Final cleanup
	runtime.GC()

	log.Info().Msgf("Completed price fetching in %s. Successfully fetched prices from %d/%d connections",
		time.Since(startTime),
		totalSuccessCount,
		connConfigCount)

	return connConfigCount, priceCount, nil
}

// fetchGCPPricesDirect fetches GCP VM pricing directly from Google's pricing pages
// and updates spec prices in bulk for all GCP connection configs.
// This bypasses cb-spider, which has a broken GCP pricing scraper.
func fetchGCPPricesDirect(gcpConfigs []model.ConnConfig) (successCount uint, errors []string) {
	log.Info().Msgf("GCP: directly fetching pricing from Google for %d connections", len(gcpConfigs))
	gcpStart := time.Now()
	maxConcurrent := 8

	// Fetch all GCP prices at once (5 HTTP requests for 5 sub-pages)
	priceCache, err := gcpPricing.FetchAllGCPPrices()
	if err != nil {
		errMsg := fmt.Sprintf("GCP direct pricing fetch failed: %v", err)
		log.Error().Msg(errMsg)
		for _, c := range gcpConfigs {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %s", c.ConfigName, errMsg))
		}
		return 0, errors
	}

	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	type connResult struct {
		ConnName string
		Err      error
	}
	resultChan := make(chan connResult, len(gcpConfigs))

	for _, connConfig := range gcpConfigs {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(config model.ConnConfig) {
			defer wg.Done()
			defer func() { <-semaphore }()

			region := config.RegionDetail.RegionName
			priceData := priceCache.GetPriceForRegion(region)
			if len(priceData.PriceList) == 0 {
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("No GCP prices found for region %s", region)}
				return
			}

			// Build batch updates (same logic as FetchPriceForConnConfig)
			batchUpdates := make(map[string]float32, len(priceData.PriceList))
			for i := range priceData.PriceList {
				price := priceData.PriceList[i]
				priceFloat, parseErr := strconv.ParseFloat(price.PriceInfo.OnDemand.Price, 32)
				if parseErr != nil {
					log.Warn().Msgf("GCP direct: failed to parse price %q for spec %s: %v",
						price.PriceInfo.OnDemand.Price, price.ProductInfo.VMSpecName, parseErr)
					continue
				}
				priceFloat = float64(common.ConvertToBaseCurrency(float32(priceFloat), price.PriceInfo.OnDemand.Currency))
				specKey := GetProviderRegionZoneResourceKey(
					config.ProviderName,
					config.RegionDetail.RegionName,
					"",
					price.ProductInfo.VMSpecName)
				batchUpdates[specKey] = float32(priceFloat)
			}

			if len(batchUpdates) > 0 {
				_, dbErr := BulkUpdateSpec(model.SystemCommonNs, batchUpdates)
				if dbErr != nil {
					resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error updating GCP prices for %s: %w", config.ConfigName, dbErr)}
					return
				}
			}

			log.Debug().Msgf("GCP direct: updated %d prices for %s (%s)", len(batchUpdates), config.ConfigName, region)
			resultChan <- connResult{ConnName: config.ConfigName, Err: nil}
		}(connConfig)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Err != nil {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %v", result.ConnName, result.Err))
			continue
		}
		successCount++
	}

	log.Info().Msgf("GCP direct pricing completed in %s: %d/%d connections succeeded",
		time.Since(gcpStart), successCount, len(gcpConfigs))
	return successCount, errors
}

// fetchAzurePricesDirect fetches Azure VM pricing directly from Azure Retail Prices API
// and updates spec prices in bulk for all Azure connection configs.
func fetchAzurePricesDirect(azureConfigs []model.ConnConfig) (successCount uint, errors []string) {
	log.Info().Msgf("Azure: directly fetching pricing from Azure Retail Prices API for %d connections", len(azureConfigs))
	azureStart := time.Now()
	maxConcurrent := 8
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	type connResult struct {
		ConnName string
		Err      error
	}
	resultChan := make(chan connResult, len(azureConfigs))

	for _, connConfig := range azureConfigs {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(config model.ConnConfig) {
			defer wg.Done()
			defer func() { <-semaphore }()

			region := config.RegionDetail.RegionName
			priceData, fetchErr := azurePricing.FetchNodePricesByRegion(region)
			if fetchErr != nil {
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error fetching Azure prices for connection %s: %w", config.ConfigName, fetchErr)}
				return
			}
			if len(priceData.PriceList) == 0 {
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("No Azure prices found for region %s", region)}
				return
			}

			batchUpdates := make(map[string]float32, len(priceData.PriceList))
			for i := range priceData.PriceList {
				price := priceData.PriceList[i]
				priceFloat, parseErr := strconv.ParseFloat(price.PriceInfo.OnDemand.Price, 32)
				if parseErr != nil {
					log.Warn().Msgf("Azure direct: failed to parse price %q for spec %s: %v",
						price.PriceInfo.OnDemand.Price, price.ProductInfo.VMSpecName, parseErr)
					continue
				}
				priceFloat = float64(common.ConvertToBaseCurrency(float32(priceFloat), price.PriceInfo.OnDemand.Currency))
				specKey := GetProviderRegionZoneResourceKey(
					config.ProviderName,
					config.RegionDetail.RegionName,
					"",
					price.ProductInfo.VMSpecName)
				batchUpdates[specKey] = float32(priceFloat)
			}

			if len(batchUpdates) > 0 {
				_, dbErr := BulkUpdateSpec(model.SystemCommonNs, batchUpdates)
				if dbErr != nil {
					resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error updating Azure prices for %s: %w", config.ConfigName, dbErr)}
					return
				}
			}

			log.Debug().Msgf("Azure direct: updated %d prices for %s (%s)", len(batchUpdates), config.ConfigName, region)
			resultChan <- connResult{ConnName: config.ConfigName, Err: nil}
		}(connConfig)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Err != nil {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %v", result.ConnName, result.Err))
			continue
		}
		successCount++
	}

	log.Info().Msgf("Azure direct pricing completed in %s: %d/%d connections succeeded",
		time.Since(azureStart), successCount, len(azureConfigs))
	return successCount, errors
}

// fetchAWSPricesDirect fetches AWS EC2 VM pricing directly from the AWS Pricing API
// using a single global query (no per-region filter), then distributes the results to
// all AWS connection configs via BulkUpdateSpec.
//
// Compared to the Spider path this eliminates:
//   - N×HTTP round-trips to cb-spider (one per region/connection)
//   - Spider's JSON marshaling/unmarshaling overhead
//   - Competition for the shared Spider concurrency pool
func fetchAWSPricesDirect(awsConfigs []model.ConnConfig) (successCount uint, errors []string) {
	log.Info().Msgf("AWS: directly fetching pricing from AWS Pricing API for %d connections", len(awsConfigs))
	awsStart := time.Now()
	maxConcurrent := 8

	// One global API call fetches pricing for ALL AWS regions at once. Bound it with a
	// timeout: without one, a hung/slow AWS Pricing API call blocks this goroutine
	// indefinitely and AWS specs silently stay unpriced while the other providers finish.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	priceMap, err := awsPricing.FetchAllNodePrices(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("AWS direct pricing fetch failed: %v", err)
		log.Error().Msg(errMsg)
		for _, c := range awsConfigs {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %s", c.ConfigName, errMsg))
		}
		return 0, errors
	}

	// Distribute cached results to each AWS connection config in parallel.
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	type connResult struct {
		ConnName string
		Err      error
	}
	resultChan := make(chan connResult, len(awsConfigs))

	for _, connConfig := range awsConfigs {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(config model.ConnConfig) {
			defer wg.Done()
			defer func() { <-semaphore }()

			region := config.RegionDetail.RegionName
			priceData, found := priceMap[region]
			if !found || len(priceData.PriceList) == 0 {
				log.Warn().Msgf("AWS direct: no prices found for region %s (connection %s)", region, config.ConfigName)
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("No AWS prices found for region %s", region)}
				return
			}

			batchUpdates := make(map[string]float32, len(priceData.PriceList))
			for i := range priceData.PriceList {
				price := priceData.PriceList[i]
				priceFloat, parseErr := strconv.ParseFloat(price.PriceInfo.OnDemand.Price, 32)
				if parseErr != nil {
					log.Warn().Msgf("AWS direct: failed to parse price %q for spec %s: %v",
						price.PriceInfo.OnDemand.Price, price.ProductInfo.VMSpecName, parseErr)
					continue
				}
				priceFloat = float64(common.ConvertToBaseCurrency(float32(priceFloat), price.PriceInfo.OnDemand.Currency))
				specKey := GetProviderRegionZoneResourceKey(
					config.ProviderName,
					config.RegionDetail.RegionName,
					"",
					price.ProductInfo.VMSpecName)
				batchUpdates[specKey] = float32(priceFloat)
			}

			if len(batchUpdates) > 0 {
				_, dbErr := BulkUpdateSpec(model.SystemCommonNs, batchUpdates)
				if dbErr != nil {
					resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error updating AWS prices for %s: %w", config.ConfigName, dbErr)}
					return
				}
			}

			log.Debug().Msgf("AWS direct: updated %d prices for %s (%s)", len(batchUpdates), config.ConfigName, region)
			resultChan <- connResult{ConnName: config.ConfigName, Err: nil}
		}(connConfig)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Err != nil {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %v", result.ConnName, result.Err))
			continue
		}
		successCount++
	}

	log.Info().Msgf("AWS direct pricing completed in %s: %d/%d connections succeeded",
		time.Since(awsStart), successCount, len(awsConfigs))
	return successCount, errors
}

// fetchAlibabaPricesDirect fetches Alibaba VM pricing directly from Alibaba ECS API.
// It intentionally focuses on spec cost/hour only, avoiding Spider's disk-category sweep path.
func fetchAlibabaPricesDirect(alibabaConfigs []model.ConnConfig) (successCount uint, errors []string) {
	log.Info().Msgf("Alibaba: directly fetching pricing from Alibaba ECS API for %d connections", len(alibabaConfigs))
	alibabaStart := time.Now()

	maxConcurrent := 10
	if v := strings.TrimSpace(os.Getenv("TB_ALIBABA_REGION_CONCURRENCY")); v != "" {
		if n, parseErr := strconv.Atoi(v); parseErr == nil && n > 0 {
			maxConcurrent = n
		}
	}
	log.Info().Msgf("Alibaba direct pricing: regionConcurrency=%d", maxConcurrent)

	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	type connResult struct {
		ConnName string
		Err      error
	}
	resultChan := make(chan connResult, len(alibabaConfigs))

	for _, connConfig := range alibabaConfigs {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(config model.ConnConfig) {
			defer wg.Done()
			defer func() { <-semaphore }()

			region := config.RegionDetail.RegionName

			// Fetch only for spec names that actually exist in TB DB for this provider+region.
			// This avoids querying Alibaba prices for thousands of irrelevant instance types.
			var regionSpecs []model.SpecInfo
			specQueryErr := model.ORM.Select("csp_spec_name").
				Where("namespace = ? AND provider_name = ? AND region_name = ?",
					model.SystemCommonNs, config.ProviderName, region).
				Find(&regionSpecs).Error
			if specQueryErr != nil {
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error querying Alibaba spec names for connection %s: %w", config.ConfigName, specQueryErr)}
				return
			}

			targetSpecNames := make(map[string]struct{}, len(regionSpecs))
			for i := range regionSpecs {
				specName := strings.TrimSpace(regionSpecs[i].CspSpecName)
				if specName == "" {
					continue
				}
				targetSpecNames[specName] = struct{}{}
			}

			if len(targetSpecNames) == 0 {
				log.Info().Msgf("Alibaba direct: no target spec names found in TB DB for %s (%s), skipping", config.ConfigName, region)
				resultChan <- connResult{ConnName: config.ConfigName, Err: nil}
				return
			}

			priceData, fetchErr := alibabaPricing.FetchNodePricesByRegionFiltered(context.Background(), region, targetSpecNames)
			if fetchErr != nil {
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error fetching Alibaba prices for connection %s: %w", config.ConfigName, fetchErr)}
				return
			}
			if len(priceData.PriceList) == 0 {
				resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("No Alibaba prices found for region %s", region)}
				return
			}

			batchUpdates := make(map[string]float32, len(priceData.PriceList))
			for i := range priceData.PriceList {
				price := priceData.PriceList[i]
				priceFloat, parseErr := strconv.ParseFloat(price.PriceInfo.OnDemand.Price, 32)
				if parseErr != nil {
					log.Warn().Msgf("Alibaba direct: failed to parse price %q for spec %s: %v",
						price.PriceInfo.OnDemand.Price, price.ProductInfo.VMSpecName, parseErr)
					continue
				}

				priceFloat = float64(common.ConvertToBaseCurrency(float32(priceFloat), price.PriceInfo.OnDemand.Currency))
				specKey := GetProviderRegionZoneResourceKey(
					config.ProviderName,
					config.RegionDetail.RegionName,
					"",
					price.ProductInfo.VMSpecName)
				batchUpdates[specKey] = float32(priceFloat)
			}

			if len(batchUpdates) > 0 {
				_, dbErr := BulkUpdateSpec(model.SystemCommonNs, batchUpdates)
				if dbErr != nil {
					resultChan <- connResult{ConnName: config.ConfigName, Err: fmt.Errorf("Error updating Alibaba prices for %s: %w", config.ConfigName, dbErr)}
					return
				}
			}

			log.Debug().Msgf("Alibaba direct: updated %d prices for %s (%s)", len(batchUpdates), config.ConfigName, region)
			resultChan <- connResult{ConnName: config.ConfigName, Err: nil}
		}(connConfig)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	totalCount := len(alibabaConfigs)
	doneCount := 0
	for result := range resultChan {
		doneCount++
		if result.Err != nil {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %v", result.ConnName, result.Err))
		} else {
			successCount++
		}

		if doneCount%5 == 0 || doneCount == totalCount {
			elapsed := time.Since(alibabaStart)
			avgPerConn := elapsed / time.Duration(doneCount)
			remaining := totalCount - doneCount
			eta := avgPerConn * time.Duration(remaining)
			log.Info().Msgf(
				"Alibaba direct pricing progress: done=%d/%d success=%d errors=%d elapsed=%s eta=%s",
				doneCount, totalCount, successCount, len(errors), elapsed, eta,
			)
		}
	}

	log.Info().Msgf("Alibaba direct pricing completed in %s: %d/%d connections succeeded",
		time.Since(alibabaStart), successCount, len(alibabaConfigs))
	return successCount, errors
}

// fetchPricesViaSpider fetches prices for non-GCP connections via cb-spider.
func fetchPricesViaSpider(configs []model.ConnConfig) (successCount uint, errors []string) {
	maxConcurrent := 15
	semaphore := make(chan struct{}, maxConcurrent)

	fetchPricesWithRetry := func(config model.ConnConfig) error {
		err := FetchPriceForConnConfig(config)
		if err != nil {
			log.Warn().Err(err).Msgf("First attempt failed for connection %s, will retry",
				config.ConfigName)
			if strings.Contains(err.Error(), "not support") {
				log.Warn().Msgf("Skipping retry for connection %s due to unsupported error",
					config.ConfigName)
				return err
			}
			common.RandomSleep(2*1000, 5*1000)
			err = FetchPriceForConnConfig(config)
		}
		if err != nil {
			log.Error().Err(err).Msgf("Failed to fetch prices for connection %s after retry",
				config.ConfigName)
			return err
		}
		return nil
	}

	var wg sync.WaitGroup
	type connResult struct {
		ConfigName string
		Err        error
	}
	resultChan := make(chan connResult, len(configs))

	for _, connConfig := range configs {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(config model.ConnConfig) {
			defer wg.Done()
			defer func() { <-semaphore }()
			common.RandomSleep(0, 10*1000)
			err := fetchPricesWithRetry(config)
			runtime.GC()
			resultChan <- connResult{ConfigName: config.ConfigName, Err: err}
		}(connConfig)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Err != nil {
			errors = append(errors, fmt.Sprintf("Error fetching prices for connection %s: %v",
				result.ConfigName, result.Err))
			continue
		}
		successCount++
	}
	return successCount, errors
}

// FetchPriceForConnConfig lookups all Price for region of conn config, processes them in batch
func FetchPriceForConnConfig(config model.ConnConfig) error {
	log.Debug().Msg("Init fetching prices for connection: " + config.ConfigName)

	// Reuse existing LookupPriceList function
	priceInConnection, err := LookupPriceList(config)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot LookupPriceList in %s", config.ConfigName)
		return err
	}

	// To check GCP prices since it frequently shows unexpected results
	// if strings.EqualFold(csp.GCP, config.ProviderName) {
	// 	log.Debug().Msgf("GCP price %v", priceInConnection)
	// }

	if len(priceInConnection.PriceList) == 0 {
		log.Warn().Msgf("No prices found for connection %s",
			config.ConfigName)
		return nil
	}

	// Prepare batch updates map
	batchUpdates := make(map[string]float32, len(priceInConnection.PriceList))
	processedCount := 0

	for i := range priceInConnection.PriceList {
		price := priceInConnection.PriceList[i]

		priceFloat, err := strconv.ParseFloat(price.PriceInfo.OnDemand.Price, 32)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to parse price '%s' for spec '%s', skipping.",
				price.PriceInfo.OnDemand.Price, price.ProductInfo.VMSpecName)

			continue
		}

		// Apply currency conversion
		priceFloat = float64(common.ConvertToBaseCurrency(float32(priceFloat), price.PriceInfo.OnDemand.Currency))

		// Create spec key
		specKey := GetProviderRegionZoneResourceKey(
			config.ProviderName,
			config.RegionDetail.RegionName,
			"",
			price.ProductInfo.VMSpecName)

		// Add to batch instead of individual update
		batchUpdates[specKey] = float32(priceFloat)
		processedCount++

	}

	// Release the original data slice immediately
	priceInConnection.PriceList = nil
	priceInConnection = model.SpiderCloudPrice{}

	// Perform batch update if we have data to update
	if len(batchUpdates) > 0 {
		_, err := BulkUpdateSpec(model.SystemCommonNs, batchUpdates)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to batch update specs for %s", config.ConfigName)
			batchUpdates = nil
			return err
		}
		// log.Debug().Msgf("Successfully updated %d specs for %s", updateCount, config.ConfigName)
	}

	// Clear the batch map to help GC
	batchUpdates = nil

	if processedCount > 100 {
		runtime.GC()
	}

	log.Debug().Msgf("Processed %d price items from %s", processedCount, config.ConfigName)
	return nil
}

// Sort connections by CSP rotation to ensure different CSPs are processed in parallel
// Result: csp1-region1, csp2-region1, csp3-region1, csp1-region2, csp2-region2, csp3-region2, ...
func sortConnectionsByCSPRotation(configs []model.ConnConfig) {
	// Group by CSP provider
	cspGroups := make(map[string][]model.ConnConfig)
	for _, config := range configs {
		provider := config.ProviderName
		cspGroups[provider] = append(cspGroups[provider], config)
	}

	// Get sorted CSP names for consistent ordering
	cspNames := make([]string, 0, len(cspGroups))
	for cspName := range cspGroups {
		cspNames = append(cspNames, cspName)
	}
	sort.Strings(cspNames)

	// Find maximum number of regions in any CSP
	maxRegions := 0
	for _, configs := range cspGroups {
		if len(configs) > maxRegions {
			maxRegions = len(configs)
		}
	}

	// Rebuild the slice in rotation order
	rotatedConfigs := make([]model.ConnConfig, 0, len(configs))
	for regionIndex := 0; regionIndex < maxRegions; regionIndex++ {
		for _, cspName := range cspNames {
			if regionIndex < len(cspGroups[cspName]) {
				rotatedConfigs = append(rotatedConfigs, cspGroups[cspName][regionIndex])
			}
		}
	}

	// Copy back to original slice
	copy(configs, rotatedConfigs)
}

// LookupPriceList returns the list of all prices in the region of conn config
// in the form of the list of Spider price objects
func LookupPriceList(connConfig model.ConnConfig) (model.SpiderCloudPrice, error) {

	var callResult model.SpiderCloudPrice
	client := clientManager.NewHttpClient()
	client.SetTimeout(10 * time.Minute)
	url := model.SpiderRestUrl + "/priceinfo/vm/" +
		connConfig.RegionZoneInfo.AssignedRegion +
		"?ConnectionName=" + connConfig.ConfigName +
		"&simple=true"
	method := "GET"
	requestBody := clientManager.NoBody

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
		content := model.SpiderCloudPrice{}
		return content, err
	}

	temp := callResult
	return temp, nil
}
