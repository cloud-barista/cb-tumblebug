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

package alibaba

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

const (
	defaultAlibabaWorkerCount      = 5
	defaultAlibabaRequestInterval  = 200 // ms
	defaultAlibabaMaxRetry         = 3
	defaultAlibabaRetryBaseBackoff = 1200 // ms
)

// FetchVMPricesByRegion fetches Alibaba ECS instance prices directly from Alibaba APIs.
//
// Optimization focus for TB spec cost/hour use-case:
// - Uses DescribeInstanceTypes once to get candidate specs.
// - Calls DescribePrice per instanceType WITHOUT systemDiskCategory sweep.
// - Extracts only the instanceType component price (cost/hour).
// - Treats PriceNotFound as data-gap (skip), not hard failure.
func FetchVMPricesByRegion(ctx context.Context, region string) (model.SpiderCloudPrice, error) {
	return FetchVMPricesByRegionFiltered(ctx, region, nil)
}

// FetchVMPricesByRegionFiltered fetches Alibaba ECS instance prices with optional
// instance-type filtering. When targetInstanceTypes is non-empty, only those instance
// types are queried via DescribePrice.
func FetchVMPricesByRegionFiltered(ctx context.Context, region string, targetInstanceTypes map[string]struct{}) (model.SpiderCloudPrice, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		return model.SpiderCloudPrice{}, fmt.Errorf("region is empty")
	}

	accessKeyID, accessKeySecret, err := getAlibabaCreds(ctx)
	if err != nil {
		return model.SpiderCloudPrice{}, fmt.Errorf("failed to get Alibaba credentials: %w", err)
	}

	ecsClient, err := ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return model.SpiderCloudPrice{}, fmt.Errorf("failed to create Alibaba ECS client for region %s: %w", region, err)
	}

	instanceTypes, err := listInstanceTypes(ecsClient, region)
	if err != nil {
		return model.SpiderCloudPrice{}, err
	}
	if len(instanceTypes) == 0 {
		return model.SpiderCloudPrice{}, nil
	}

	if len(targetInstanceTypes) > 0 {
		filtered := make([]string, 0, len(instanceTypes))
		for _, t := range instanceTypes {
			if _, ok := targetInstanceTypes[t]; ok {
				filtered = append(filtered, t)
			}
		}
		instanceTypes = filtered
		if len(instanceTypes) == 0 {
			log.Info().Msgf("Alibaba direct pricing: region=%s targetSpecNames=%d matchedInstanceTypes=0", region, len(targetInstanceTypes))
			return model.SpiderCloudPrice{}, nil
		}
	}

	workerCount := parsePositiveEnvInt("TB_ALIBABA_PRICE_WORKERS", defaultAlibabaWorkerCount)
	requestInterval := time.Duration(parsePositiveEnvInt("TB_ALIBABA_PRICE_INTERVAL_MS", defaultAlibabaRequestInterval)) * time.Millisecond

	type result struct {
		item   model.SpiderPrice
		ok     bool
		reason string
	}

	jobs := make(chan string, len(instanceTypes))
	results := make(chan result, len(instanceTypes))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for instanceType := range jobs {
				item, ok, reason := fetchSingleInstanceTypePrice(ecsClient, region, instanceType, requestInterval)
				results <- result{item: item, ok: ok, reason: reason}
			}
		}(i)
	}

	for _, t := range instanceTypes {
		jobs <- t
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	priceList := make([]model.SpiderPrice, 0, len(instanceTypes))
	skippedExpected := 0
	skippedUnexpected := 0
	for res := range results {
		if !res.ok {
			if res.reason == "expected_skip" {
				skippedExpected++
			} else {
				skippedUnexpected++
			}
			continue
		}
		priceList = append(priceList, res.item)
	}

	if len(targetInstanceTypes) > 0 {
		log.Info().Msgf("Alibaba direct pricing: region=%s instanceTypes=%d targetSpecNames=%d priced=%d skippedExpected=%d skippedUnexpected=%d",
			region, len(instanceTypes), len(targetInstanceTypes), len(priceList), skippedExpected, skippedUnexpected)
	} else {
		log.Info().Msgf("Alibaba direct pricing: region=%s instanceTypes=%d priced=%d skippedExpected=%d skippedUnexpected=%d",
			region, len(instanceTypes), len(priceList), skippedExpected, skippedUnexpected)
	}

	return model.SpiderCloudPrice{PriceList: priceList}, nil
}

func fetchSingleInstanceTypePrice(client *ecs.Client, region, instanceType string, requestInterval time.Duration) (model.SpiderPrice, bool, string) {
	// Per-call pacing to reduce API burst/rate-limit pressure.
	time.Sleep(requestInterval)

	buildReq := func(systemDiskCategory string) *ecs.DescribePriceRequest {
		req := ecs.CreateDescribePriceRequest()
		req.Scheme = "https"
		req.RegionId = region
		req.ResourceType = "instance"
		req.InstanceType = instanceType
		req.Period = requests.NewInteger(1)
		req.PriceUnit = "Hour"
		if strings.TrimSpace(systemDiskCategory) != "" {
			req.SystemDiskCategory = systemDiskCategory
		}
		return req
	}

	// First try without disk category for minimum API overhead.
	resp, err := callDescribePriceWithRetry(client, buildReq(""))

	// Some regions/specs require explicit disk category even for instance pricing.
	if err != nil && strings.Contains(err.Error(), "ErrorCode: InvalidSystemDiskCategory.ValueNotSupported") {
		fallbackCategories := []string{"cloud_essd", "cloud_efficiency"}
		for _, cat := range fallbackCategories {
			resp, err = callDescribePriceWithRetry(client, buildReq(cat))
			if err == nil {
				break
			}
		}
	}

	if err != nil {
		if isAlibabaExpectedSkipError(err) {
			return model.SpiderPrice{}, false, "expected_skip"
		}
		log.Warn().Msgf("Alibaba direct pricing: region=%s instanceType=%s err=%v", region, instanceType, err)
		return model.SpiderPrice{}, false, "unexpected_error"
	}

	if resp == nil {
		return model.SpiderPrice{}, false, "unexpected_error"
	}

	currency := strings.TrimSpace(resp.PriceInfo.Price.Currency)
	if currency == "" {
		currency = "CNY"
	}

	instancePrice := 0.0
	if resp.PriceInfo.Price.DetailInfos.DetailInfo != nil {
		for _, detail := range resp.PriceInfo.Price.DetailInfos.DetailInfo {
			if strings.EqualFold(detail.Resource, "instanceType") {
				instancePrice = detail.TradePrice
				break
			}
		}
	}
	if instancePrice <= 0 {
		instancePrice = resp.PriceInfo.Price.TradePrice
	}
	if instancePrice <= 0 {
		return model.SpiderPrice{}, false, "expected_skip"
	}

	return model.SpiderPrice{
		ProductInfo: model.SpiderProductInfo{VMSpecName: instanceType},
		PriceInfo: model.SpiderPriceInfo{
			OnDemand: model.SpiderOnDemand{
				PricingId: "NA",
				Unit:      "Hour",
				Currency:  currency,
				Price:     strconv.FormatFloat(instancePrice, 'f', -1, 64),
			},
		},
	}, true, "ok"
}

func isAlibabaExpectedSkipError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "ErrorCode: PriceNotFound") ||
		strings.Contains(errStr, "ErrorCode: InvalidInstanceType.NotFound") ||
		strings.Contains(errStr, "ErrorCode: InvalidInstanceType.ValueNotSupported") ||
		strings.Contains(errStr, "ErrorCode: InvalidSystemDiskCategory.ValueNotSupported") ||
		strings.Contains(errStr, "ErrorCode: InvalidParameter") ||
		strings.Contains(errStr, "Message: The specified InstanceType beyond the permitted range")
}

func callDescribePriceWithRetry(client *ecs.Client, req *ecs.DescribePriceRequest) (*ecs.DescribePriceResponse, error) {
	maxRetry := parsePositiveEnvInt("TB_ALIBABA_PRICE_MAX_RETRY", defaultAlibabaMaxRetry)
	baseBackoff := parsePositiveEnvInt("TB_ALIBABA_PRICE_BACKOFF_MS", defaultAlibabaRetryBaseBackoff)

	var lastErr error
	for attempt := 0; attempt < maxRetry; attempt++ {
		resp, err := client.DescribePrice(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		errStr := err.Error()
		isThrottling := strings.Contains(errStr, "ErrorCode: Throttling") || strings.Contains(errStr, "TooManyRequests")
		isTransientNetwork := isAlibabaRetryableTransientNetworkError(errStr)
		if isThrottling || isTransientNetwork {
			// For network glitches (connection reset/timeout/EOF), retry at most once.
			if isTransientNetwork && attempt >= 1 {
				break
			}

			jitter := rand.Intn(400)
			sleepMs := baseBackoff*(attempt+1) + jitter
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)
			continue
		}

		// Non-throttling errors are not retried aggressively.
		break
	}

	return nil, lastErr
}

func isAlibabaRetryableTransientNetworkError(errStr string) bool {
	msg := strings.ToLower(strings.TrimSpace(errStr))
	if msg == "" {
		return false
	}

	return strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "tls handshake timeout") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, " eof") ||
		strings.Contains(msg, "read: connection")
}

func listInstanceTypes(client *ecs.Client, region string) ([]string, error) {
	req := ecs.CreateDescribeInstanceTypesRequest()
	req.Scheme = "https"
	req.RegionId = region
	req.MaxResults = requests.NewInteger(100)

	types := make([]string, 0, 1000)
	nextToken := ""

	for {
		if nextToken != "" {
			req.NextToken = nextToken
		}

		resp, err := client.DescribeInstanceTypes(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list Alibaba instance types for region %s: %w", region, err)
		}

		for _, t := range resp.InstanceTypes.InstanceType {
			name := strings.TrimSpace(t.InstanceTypeId)
			if name != "" {
				types = append(types, name)
			}
		}

		nextToken = strings.TrimSpace(resp.NextToken)
		if nextToken == "" {
			break
		}
	}

	return types, nil
}

func getAlibabaCreds(ctx context.Context) (accessKeyID, accessKeySecret string, err error) {
	path := csp.BuildSecretPath(ctx, "alibaba")
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return "", "", err
	}

	accessKeyID = csp.GetString(data, "ALIBABA_CLOUD_ACCESS_KEY_ID")
	accessKeySecret = csp.GetString(data, "ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	if accessKeyID == "" || accessKeySecret == "" {
		return "", "", fmt.Errorf("Alibaba credentials incomplete at %s", path)
	}

	return accessKeyID, accessKeySecret, nil
}

func parsePositiveEnvInt(name string, defaultValue int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultValue
	}
	return n
}

// FetchAvailableSpecListByRegion fetches Alibaba VM specs available in the given
// region using DescribeAvailableResource(Status=Available), then enriches them
// with spec details from DescribeInstanceTypes.
func FetchAvailableSpecListByRegion(ctx context.Context, region string, zoneID string) (model.SpiderSpecList, error) {
	region = strings.TrimSpace(region)
	zoneID = strings.TrimSpace(zoneID)
	if region == "" {
		return model.SpiderSpecList{}, fmt.Errorf("region is empty")
	}

	accessKeyID, accessKeySecret, err := getAlibabaCreds(ctx)
	if err != nil {
		return model.SpiderSpecList{}, fmt.Errorf("failed to get Alibaba credentials: %w", err)
	}

	ecsClient, err := ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return model.SpiderSpecList{}, fmt.Errorf("failed to create Alibaba ECS client for region %s: %w", region, err)
	}

	availableNames, err := listAvailableInstanceTypes(ecsClient, region, zoneID)
	if err != nil {
		return model.SpiderSpecList{}, err
	}
	if len(availableNames) == 0 {
		return model.SpiderSpecList{}, nil
	}

	details, err := listInstanceTypeDetails(ecsClient, region)
	if err != nil {
		return model.SpiderSpecList{}, err
	}

	out := model.SpiderSpecList{Vmspec: make([]model.SpiderSpecInfo, 0, len(availableNames))}
	for _, d := range details {
		name := strings.TrimSpace(d.InstanceTypeId)
		if name == "" {
			continue
		}
		if _, ok := availableNames[name]; !ok {
			continue
		}

		spec := model.SpiderSpecInfo{
			Region: region,
			Name:   name,
			VCpu: model.SpiderVCpuInfo{
				Count:    strconv.Itoa(d.CpuCoreCount),
				ClockGHz: "-1",
			},
			MemSizeMiB: strconv.FormatFloat(d.MemorySize*1024, 'f', 0, 64),
			DiskSizeGB: "-1",
			KeyValueList: []model.KeyValue{
				{Key: "CpuArchitecture", Value: d.CpuArchitecture},
			},
		}

		if d.LocalStorageCapacity > 0 {
			spec.DiskSizeGB = strconv.FormatFloat(float64(d.LocalStorageCapacity)*1.073741824, 'f', 0, 64)
		}

		if d.GPUAmount > 0 {
			gpuModel := "NA"
			gpuMfr := "NA"
			if strings.TrimSpace(d.GPUSpec) != "" {
				gpuModel = strings.ToUpper(strings.TrimSpace(d.GPUSpec))
				parts := strings.Fields(d.GPUSpec)
				if len(parts) > 0 {
					gpuMfr = strings.ToUpper(parts[0])
				}
			}

			gpuMem := "-1"
			totalGpuMem := "-1"
			if d.GPUMemorySize > 0 {
				gpuMem = strconv.FormatFloat(d.GPUMemorySize, 'f', 0, 64)
				totalGpuMem = strconv.FormatFloat(d.GPUMemorySize*float64(d.GPUAmount), 'f', 0, 64)
			}

			spec.Gpu = []model.SpiderGpuInfo{{
				Count:          strconv.Itoa(d.GPUAmount),
				Mfr:            gpuMfr,
				Model:          gpuModel,
				MemSizeGB:      gpuMem,
				TotalMemSizeGB: totalGpuMem,
			}}
		}

		out.Vmspec = append(out.Vmspec, spec)
	}

	sort.Slice(out.Vmspec, func(i, j int) bool {
		return out.Vmspec[i].Name < out.Vmspec[j].Name
	})

	log.Info().Msgf("Alibaba direct available spec fetch: region=%s zone=%s available=%d detailed=%d",
		region, zoneID, len(availableNames), len(out.Vmspec))

	return out, nil
}

func listAvailableInstanceTypes(client *ecs.Client, region string, zoneID string) (map[string]struct{}, error) {
	req := ecs.CreateDescribeAvailableResourceRequest()
	req.Scheme = "https"
	req.RegionId = region
	req.ResourceType = "instance"
	req.DestinationResource = "InstanceType"
	if zoneID != "" {
		req.ZoneId = zoneID
	}

	resp, err := client.DescribeAvailableResource(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list Alibaba available resources for region %s: %w", region, err)
	}

	available := make(map[string]struct{})
	for _, z := range resp.AvailableZones.AvailableZone {
		if zoneID != "" && !strings.EqualFold(strings.TrimSpace(z.ZoneId), zoneID) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(z.Status), "available") {
			continue
		}

		for _, r := range z.AvailableResources.AvailableResource {
			if !strings.EqualFold(strings.TrimSpace(r.Type), "instancetype") {
				continue
			}
			for _, s := range r.SupportedResources.SupportedResource {
				if !strings.EqualFold(strings.TrimSpace(s.Status), "available") {
					continue
				}
				name := strings.TrimSpace(s.Value)
				if name == "" {
					continue
				}
				available[name] = struct{}{}
			}
		}
	}

	return available, nil
}

func listInstanceTypeDetails(client *ecs.Client, region string) ([]ecs.InstanceType, error) {
	req := ecs.CreateDescribeInstanceTypesRequest()
	req.Scheme = "https"
	req.RegionId = region
	req.MaxResults = requests.NewInteger(100)

	out := make([]ecs.InstanceType, 0, 1000)
	nextToken := ""
	for {
		if nextToken != "" {
			req.NextToken = nextToken
		}

		resp, err := client.DescribeInstanceTypes(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list Alibaba instance type details for region %s: %w", region, err)
		}

		out = append(out, resp.InstanceTypes.InstanceType...)
		nextToken = strings.TrimSpace(resp.NextToken)
		if nextToken == "" {
			break
		}
	}

	return out, nil
}
