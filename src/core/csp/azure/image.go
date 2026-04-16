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

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

const (
	defaultPublisherWorkers = 8
	maxRetries              = 4
	DEBUG_AZURE_IMAGE       = false // Set to true for detailed timing and progress debugging
)

// PublisherFilterConfig represents the publisher filtering configuration from YAML
type PublisherFilterConfig struct {
	PublisherFiltering struct {
		Enabled  bool   `yaml:"enabled"`
		Strategy string `yaml:"strategy"` // "whitelist" or "blacklist"
	} `yaml:"publisherFiltering"`
	WhitelistedPublishers []string `yaml:"whitelistedPublishers"`
	BlacklistedPatterns   []string `yaml:"blacklistedPatterns"`
	Debug                 struct {
		LogFilteredPublishers bool `yaml:"logFilteredPublishers"`
		LogIncludedPublishers bool `yaml:"logIncludedPublishers"`
	} `yaml:"debug"`
}

// publisherFilterCache holds the loaded publisher filter configuration
var (
	publisherFilterMutex sync.RWMutex
	publisherFilterCache *PublisherFilterConfig
	publisherFilterErr   error
)

// loadPublisherFilters loads the publisher filter configuration from assets/azure-publisher-filters.yaml
func loadPublisherFilters() (*PublisherFilterConfig, error) {
	publisherFilterMutex.RLock()
	if publisherFilterCache != nil {
		defer publisherFilterMutex.RUnlock()
		return publisherFilterCache, publisherFilterErr
	}
	publisherFilterMutex.RUnlock()

	// Try to find assets/azure-publisher-filters.yaml
	assetsPaths := []string{
		"assets/azure-publisher-filters.yaml",
		"../assets/azure-publisher-filters.yaml",
		"../../assets/azure-publisher-filters.yaml",
	}

	var config *PublisherFilterConfig
	var data []byte
	var err error

	for _, path := range assetsPaths {
		if data, err = os.ReadFile(path); err == nil {
			if DEBUG_AZURE_IMAGE {
				log.Debug().Str("path", path).Msg("[AzureImage] Publisher filter config loaded")
			}
			break
		}
	}

	if err != nil {
		if DEBUG_AZURE_IMAGE {
			log.Warn().Err(err).Msg("[AzureImage] Publisher filter config not found, using default whitelist")
		}
		// Return default config if file not found
		config = getDefaultPublisherConfig()
	} else {
		config = &PublisherFilterConfig{}
		if yamlErr := yaml.Unmarshal(data, config); yamlErr != nil {
			log.Error().Err(yamlErr).Msg("[AzureImage] Failed to parse publisher filter config")
			config = getDefaultPublisherConfig()
		}
	}

	publisherFilterMutex.Lock()
	publisherFilterCache = config
	publisherFilterErr = nil
	publisherFilterMutex.Unlock()

	return config, nil
}

// getDefaultPublisherConfig returns default whitelist configuration
func getDefaultPublisherConfig() *PublisherFilterConfig {
	config := &PublisherFilterConfig{}
	config.PublisherFiltering.Enabled = true
	config.PublisherFiltering.Strategy = "whitelist"
	config.WhitelistedPublishers = []string{
		// Microsoft Official
		"Microsoft",
		"MicrosoftWindowsServer",
		"MicrosoftWindowsDesktop",
		"Microsoft-DSNode",
		"MicrosoftVisualStudio",
		"microsoft-ads",
		// Linux Distributions
		"Canonical",
		"RedHat",
		"SUSE",
		"CIQ",
		"Credativ",
		"AlmaLinux",
		"Oracle",
		"Kinvolk",
		"OpenLogic",
	}
	config.Debug.LogFilteredPublishers = false
	config.Debug.LogIncludedPublishers = false
	return config
}

// isPublisherAllowed checks if a publisher should be included based on current filtering rules
func isPublisherAllowed(publisher string) bool {
	config, err := loadPublisherFilters()
	if err != nil || !config.PublisherFiltering.Enabled {
		return true // If filtering is disabled or config not available, allow all
	}

	publisherLower := strings.ToLower(publisher)

	if config.PublisherFiltering.Strategy == "whitelist" {
		// Whitelist strategy: only allow publishers in the list
		for _, allowed := range config.WhitelistedPublishers {
			if strings.EqualFold(publisher, allowed) {
				if config.Debug.LogIncludedPublishers {
					log.Debug().Str("publisher", publisher).Msg("[AzureImage:DEBUG] Publisher included (whitelist)")
				}
				return true
			}
		}
		if config.Debug.LogFilteredPublishers {
			log.Debug().Str("publisher", publisher).Msg("[AzureImage:DEBUG] Publisher excluded (not in whitelist)")
		}
		return false
	} else if config.PublisherFiltering.Strategy == "blacklist" {
		// Blacklist strategy: exclude publishers matching patterns
		for _, pattern := range config.BlacklistedPatterns {
			patternLower := strings.ToLower(pattern)
			if matchPattern(publisherLower, patternLower) {
				if config.Debug.LogFilteredPublishers {
					log.Debug().Str("publisher", publisher).Str("pattern", pattern).Msg("[AzureImage:DEBUG] Publisher excluded (blacklist pattern)")
				}
				return false
			}
		}
		if config.Debug.LogIncludedPublishers {
			log.Debug().Str("publisher", publisher).Msg("[AzureImage:DEBUG] Publisher included (not blacklisted)")
		}
		return true
	}

	return true // Default to allowing if strategy is unknown
}

// matchPattern performs simple wildcard pattern matching (* means any characters)
func matchPattern(text string, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return strings.Contains(text, pattern)
	}

	parts := strings.Split(pattern, "*")
	currentPos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		pos := strings.Index(text[currentPos:], part)
		if pos == -1 {
			return false
		}

		if i == 0 && pos != 0 {
			// First part must match from start
			return false
		}

		currentPos += pos + len(part)
	}

	// If pattern ends with *, rest is ok
	// Otherwise, text must end here
	if !strings.HasSuffix(pattern, "*") && currentPos != len(text) {
		return false
	}

	return true
}

// GetImage fetches a specific Azure VM image by URN format (publisher:offer:sku:version).
// Used for lookups of pre-identified images without full enumeration.
func GetImage(ctx context.Context, region, urn string) (model.SpiderImageInfo, error) {
	startTime := time.Now()
	if DEBUG_AZURE_IMAGE {
		log.Info().Str("region", region).Str("imageURN", urn).Msg("[AzureImage:DEBUG] GetImage started")
	}

	if strings.TrimSpace(region) == "" {
		return model.SpiderImageInfo{}, fmt.Errorf("region is required")
	}
	if strings.TrimSpace(urn) == "" {
		return model.SpiderImageInfo{}, fmt.Errorf("image URN is required")
	}

	parts := strings.Split(urn, ":")
	if len(parts) != 4 {
		return model.SpiderImageInfo{}, fmt.Errorf("invalid Azure image URN format: expected 'publisher:offer:sku:version', got '%s'", urn)
	}

	publisher, offer, sku, version := parts[0], parts[1], parts[2], parts[3]
	for _, s := range []string{publisher, offer, sku, version} {
		if strings.TrimSpace(s) == "" {
			return model.SpiderImageInfo{}, fmt.Errorf("invalid Azure image URN format: empty component in '%s'", urn)
		}
	}

	creds, err := getCreds(ctx)
	if err != nil {
		return model.SpiderImageInfo{}, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return model.SpiderImageInfo{}, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := armcompute.NewVirtualMachineImagesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return model.SpiderImageInfo{}, fmt.Errorf("failed to create Azure VM image client: %w", err)
	}

	getResp, err := callWithRetry(ctx, "GetImage", func() (armcompute.VirtualMachineImagesClientGetResponse, error) {
		return client.Get(ctx, region, publisher, offer, sku, version, nil)
	})
	if err != nil {
		return model.SpiderImageInfo{}, fmt.Errorf("failed to get Azure image %s in region %s: %w", urn, region, err)
	}

	image := buildSpiderImageInfo(region, publisher, offer, sku, version, getResp)

	if DEBUG_AZURE_IMAGE {
		elapsed := time.Since(startTime)
		log.Info().Str("region", region).Str("imageURN", urn).Dur("elapsed", elapsed).Msg("[AzureImage:DEBUG] GetImage completed")
	}

	return image, nil
}

// ListImages fetches Azure VM images directly from Azure ARM API.
// It returns one latest version per (publisher, offer, sku), following Spider's behavior.
func ListImages(ctx context.Context, region string) ([]model.SpiderImageInfo, error) {
	startTime := time.Now()
	if DEBUG_AZURE_IMAGE {
		log.Info().Str("region", region).Msg("[AzureImage:DEBUG] ListImages started")
	}

	if strings.TrimSpace(region) == "" {
		return nil, fmt.Errorf("region is required")
	}

	credStartTime := time.Now()
	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}
	if DEBUG_AZURE_IMAGE {
		log.Debug().Dur("duration", time.Since(credStartTime)).Msg("[AzureImage:DEBUG] getCreds completed")
	}

	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := armcompute.NewVirtualMachineImagesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure VM image client: %w", err)
	}

	pubStartTime := time.Now()
	pubResp, err := callWithRetry(ctx, "ListPublishers", func() (armcompute.VirtualMachineImagesClientListPublishersResponse, error) {
		return client.ListPublishers(ctx, region, nil)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list publishers in region %s: %w", region, err)
	}
	if DEBUG_AZURE_IMAGE {
		log.Debug().Str("region", region).Dur("duration", time.Since(pubStartTime)).Msg("[AzureImage:DEBUG] ListPublishers API call completed")
	}

	publishers := make([]string, 0, len(pubResp.VirtualMachineImageResourceArray))
	for _, p := range pubResp.VirtualMachineImageResourceArray {
		if p != nil && p.Name != nil && *p.Name != "" {
			publishers = append(publishers, *p.Name)
		}
	}

	if len(publishers) == 0 {
		return nil, fmt.Errorf("no publishers found in region %s", region)
	}

	if DEBUG_AZURE_IMAGE {
		log.Info().Str("region", region).Int("publisherCount", len(publishers)).Msg("[AzureImage:DEBUG] Publishers fetched")
	}

	jobs := make(chan string, len(publishers))
	imagesCh := make(chan []model.SpiderImageInfo, len(publishers))

	var workerWG sync.WaitGroup
	var errCount int64

	workerCount := defaultPublisherWorkers
	if len(publishers) < workerCount {
		workerCount = len(publishers)
	}

	if DEBUG_AZURE_IMAGE {
		log.Debug().Str("region", region).Int("workerCount", workerCount).Int("publisherCount", len(publishers)).Msg("[AzureImage:DEBUG] Starting worker pool")
	}

	for w := 0; w < workerCount; w++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for publisher := range jobs {
				if DEBUG_AZURE_IMAGE {
					log.Debug().Str("region", region).Str("publisher", publisher).Msg("[AzureImage:DEBUG] Processing publisher")
				}
				pubStartTime := time.Now()
				items, e := listPublisherImages(ctx, client, region, publisher)
				if e != nil {
					atomic.AddInt64(&errCount, 1)
					log.Warn().Err(e).Str("region", region).Str("publisher", publisher).Msg("[AzureImage] publisher listing failed")
					continue
				}
				if DEBUG_AZURE_IMAGE {
					log.Debug().Str("region", region).Str("publisher", publisher).Int("imageCount", len(items)).Dur("duration", time.Since(pubStartTime)).Msg("[AzureImage:DEBUG] Publisher processing completed")
				}
				if len(items) > 0 {
					imagesCh <- items
				}
			}
		}()
	}

	// Filter publishers based on configuration and add to jobs channel
	allowedCount := 0
	filteredCount := 0
	selectedPublishers := make([]string, 0, len(publishers))
	filteredPublishers := make([]string, 0, len(publishers))
	for _, p := range publishers {
		if isPublisherAllowed(p) {
			jobs <- p
			allowedCount++
			selectedPublishers = append(selectedPublishers, p)
		} else {
			filteredCount++
			filteredPublishers = append(filteredPublishers, p)
		}
	}
	close(jobs)

	if DEBUG_AZURE_IMAGE {
		log.Info().
			Str("region", region).
			Int("totalPublishers", len(publishers)).
			Int("selectedPublishersCount", len(selectedPublishers)).
			Strs("selectedPublishers", selectedPublishers).
			Msg("[AzureImage] Publisher fetch plan")

		log.Info().
			Str("region", region).
			Int("totalPublishers", len(publishers)).
			Int("filteredPublishersCount", len(filteredPublishers)).
			Strs("filteredPublishers", filteredPublishers).
			Msg("[AzureImage] Publisher filtered summary")
	}

	if DEBUG_AZURE_IMAGE && filteredCount > 0 {
		log.Info().Str("region", region).Int("totalPublishers", len(publishers)).Int("allowedPublishers", allowedCount).Int("filteredPublishers", filteredCount).Msg("[AzureImage:DEBUG] Publisher filtering summary")
	}

	workerWG.Wait()
	close(imagesCh)

	result := make([]model.SpiderImageInfo, 0, 1024)
	for batch := range imagesCh {
		result = append(result, batch...)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("failed to fetch Azure images in region %s (all publishers failed)", region)
	}

	if errCount > 0 {
		log.Warn().
			Str("region", region).
			Int64("failedPublishers", errCount).
			Int("imageCount", len(result)).
			Msg("[AzureImage] Completed with partial publisher failures")
	}

	totalElapsed := time.Since(startTime)
	log.Info().
		Str("region", region).
		Int("imageCount", len(result)).
		Int("publisherCount", len(publishers)).
		Int("workerCount", workerCount).
		Dur("totalDuration", totalElapsed).
		Msg("[AzureImage] Direct image listing completed")

	if DEBUG_AZURE_IMAGE {
		log.Info().Str("region", region).Int("imageCount", len(result)).Dur("totalDuration", totalElapsed).Msg("[AzureImage:DEBUG] ListImages finished")
	}

	return result, nil
}

func listPublisherImages(
	ctx context.Context,
	client *armcompute.VirtualMachineImagesClient,
	region, publisher string,
) ([]model.SpiderImageInfo, error) {
	startTime := time.Now()
	if DEBUG_AZURE_IMAGE {
		log.Debug().Str("region", region).Str("publisher", publisher).Msg("[AzureImage:DEBUG] listPublisherImages started")
	}

	offersStartTime := time.Now()
	offersResp, err := callWithRetry(ctx, "ListOffers", func() (armcompute.VirtualMachineImagesClientListOffersResponse, error) {
		return client.ListOffers(ctx, region, publisher, nil)
	})
	if err != nil {
		return nil, err
	}
	offerCount := len(offersResp.VirtualMachineImageResourceArray)
	if DEBUG_AZURE_IMAGE {
		log.Debug().Str("region", region).Str("publisher", publisher).Int("offerCount", offerCount).Dur("duration", time.Since(offersStartTime)).Msg("[AzureImage:DEBUG] ListOffers completed")
	}

	out := make([]model.SpiderImageInfo, 0, 256)
	var skuCount int64
	var successCount int64

	for _, offerItem := range offersResp.VirtualMachineImageResourceArray {
		if offerItem == nil || offerItem.Name == nil || *offerItem.Name == "" {
			continue
		}
		offer := *offerItem.Name

		skuResp, skuErr := callWithRetry(ctx, "ListSKUs", func() (armcompute.VirtualMachineImagesClientListSKUsResponse, error) {
			return client.ListSKUs(ctx, region, publisher, offer, nil)
		})
		if skuErr != nil {
			log.Warn().Err(skuErr).Str("publisher", publisher).Str("offer", offer).Msg("[AzureImage] list SKUs failed")
			continue
		}

		for _, skuItem := range skuResp.VirtualMachineImageResourceArray {
			if skuItem == nil || skuItem.Name == nil || *skuItem.Name == "" {
				continue
			}
			sku := *skuItem.Name
			atomic.AddInt64(&skuCount, 1)

			versionResp, verErr := callWithRetry(ctx, "ListVersions", func() (armcompute.VirtualMachineImagesClientListResponse, error) {
				orderBy := "name desc"
				top := int32(1)
				return client.List(ctx, region, publisher, offer, sku, &armcompute.VirtualMachineImagesClientListOptions{
					Orderby: &orderBy,
					Top:     &top,
				})
			})
			if verErr != nil {
				log.Warn().Err(verErr).Str("publisher", publisher).Str("offer", offer).Str("sku", sku).Msg("[AzureImage] list versions failed")
				continue
			}

			if len(versionResp.VirtualMachineImageResourceArray) == 0 {
				continue
			}

			latest := versionResp.VirtualMachineImageResourceArray[0]
			if latest == nil {
				continue
			}

			version := ""
			if latest.Name != nil {
				version = *latest.Name
			}
			if version == "" && latest.ID != nil {
				parts := strings.Split(*latest.ID, "/")
				if len(parts) > 0 {
					version = parts[len(parts)-1]
				}
			}
			if version == "" {
				continue
			}

			getResp, getErr := callWithRetry(ctx, "GetImage", func() (armcompute.VirtualMachineImagesClientGetResponse, error) {
				return client.Get(ctx, region, publisher, offer, sku, version, nil)
			})
			if getErr != nil {
				log.Warn().Err(getErr).Str("publisher", publisher).Str("offer", offer).Str("sku", sku).Str("version", version).Msg("[AzureImage] get image detail failed")
				continue
			}

			out = append(out, buildSpiderImageInfo(region, publisher, offer, sku, version, getResp))
			atomic.AddInt64(&successCount, 1)
		}
	}

	if DEBUG_AZURE_IMAGE {
		elapsed := time.Since(startTime)
		log.Debug().Str("region", region).Str("publisher", publisher).Int64("skuCount", skuCount).Int64("successCount", successCount).Int("outputImageCount", len(out)).Dur("duration", elapsed).Msg("[AzureImage:DEBUG] listPublisherImages completed")
	}

	return out, nil
}

func buildSpiderImageInfo(
	region, publisher, offer, sku, version string,
	resp armcompute.VirtualMachineImagesClientGetResponse,
) model.SpiderImageInfo {
	urn := fmt.Sprintf("%s:%s:%s:%s", publisher, offer, sku, version)

	info := model.SpiderImageInfo{
		IId:            model.IID{NameId: urn, SystemId: urn},
		Name:           urn,
		OSArchitecture: model.ArchitectureNA,
		OSPlatform:     model.PlatformNA,
		OSDistribution: urn,
		OSDiskType:     "default",
		OSDiskSizeGB:   "-1",
		ImageStatus:    model.ImageAvailable,
		KeyValueList: []model.KeyValue{
			{Key: "Location", Value: region},
			{Key: "Publisher", Value: publisher},
			{Key: "Offer", Value: offer},
			{Key: "SKU", Value: sku},
			{Key: "Version", Value: version},
		},
	}

	if resp.VirtualMachineImage.ID != nil {
		info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "ID", Value: *resp.VirtualMachineImage.ID})
	}

	if resp.VirtualMachineImage.Properties != nil {
		props := resp.VirtualMachineImage.Properties

		if props.Architecture != nil {
			info.OSArchitecture = normalizeArch(string(*props.Architecture))
		}
		if props.OSDiskImage != nil && props.OSDiskImage.OperatingSystem != nil {
			info.OSPlatform = normalizePlatform(string(*props.OSDiskImage.OperatingSystem))
		}
		if props.HyperVGeneration != nil {
			info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "HyperVGeneration", Value: string(*props.HyperVGeneration)})
		}
		if props.Plan != nil {
			if props.Plan.Publisher != nil {
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "PlanPublisher", Value: *props.Plan.Publisher})
			}
			if props.Plan.Product != nil {
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "PlanProduct", Value: *props.Plan.Product})
			}
			if props.Plan.Name != nil {
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "PlanName", Value: *props.Plan.Name})
			}
		}
		if len(props.Features) > 0 {
			featurePairs := make([]string, 0, len(props.Features))
			for _, feature := range props.Features {
				if feature == nil {
					continue
				}

				name := ""
				value := ""
				if feature.Name != nil {
					name = strings.TrimSpace(*feature.Name)
				}
				if feature.Value != nil {
					value = strings.TrimSpace(*feature.Value)
				}
				if name == "" && value == "" {
					continue
				}

				if value == "" {
					featurePairs = append(featurePairs, name)
				} else if name == "" {
					featurePairs = append(featurePairs, value)
				} else {
					featurePairs = append(featurePairs, fmt.Sprintf("%s=%s", name, value))
				}
			}

			if len(featurePairs) > 0 {
				info.KeyValueList = append(info.KeyValueList,
					model.KeyValue{Key: "Features", Value: strings.Join(featurePairs, ", ")},
					model.KeyValue{Key: "FeatureCount", Value: fmt.Sprintf("%d", len(featurePairs))},
				)
			}
		}
		if len(props.DataDiskImages) > 0 {
			dataDiskLUNs := make([]string, 0, len(props.DataDiskImages))
			for _, disk := range props.DataDiskImages {
				if disk == nil || disk.Lun == nil {
					continue
				}
				dataDiskLUNs = append(dataDiskLUNs, fmt.Sprintf("%d", *disk.Lun))
			}

			if len(dataDiskLUNs) > 0 {
				info.KeyValueList = append(info.KeyValueList,
					model.KeyValue{Key: "DataDiskImageLUNs", Value: strings.Join(dataDiskLUNs, ",")},
					model.KeyValue{Key: "DataDiskImageCount", Value: fmt.Sprintf("%d", len(dataDiskLUNs))},
				)
			} else {
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "DataDiskImageCount", Value: fmt.Sprintf("%d", len(props.DataDiskImages))})
			}

			if raw, err := json.Marshal(props.DataDiskImages); err == nil {
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "DataDiskImages", Value: string(raw)})
			}
		}
		if props.ImageDeprecationStatus != nil {
			if props.ImageDeprecationStatus.ImageState != nil {
				state := string(*props.ImageDeprecationStatus.ImageState)
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "ImageDeprecationState", Value: state})
				info.ImageStatus = mapAzureDeprecationStateToImageStatus(state)
			} else {
				info.ImageStatus = model.ImageUnavailable
			}

			if props.ImageDeprecationStatus.AlternativeOption != nil {
				if raw, err := json.Marshal(props.ImageDeprecationStatus.AlternativeOption); err == nil {
					info.KeyValueList = append(info.KeyValueList, model.KeyValue{Key: "ImageAlternativeOption", Value: string(raw)})
				}
			}
			if props.ImageDeprecationStatus.ScheduledDeprecationTime != nil {
				info.KeyValueList = append(info.KeyValueList, model.KeyValue{
					Key:   "ImageScheduledDeprecationTime",
					Value: props.ImageDeprecationStatus.ScheduledDeprecationTime.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	return info
}

func mapAzureDeprecationStateToImageStatus(state string) model.ImageStatus {
	s := strings.ToLower(strings.TrimSpace(state))
	switch s {
	case "active":
		return model.ImageAvailable
	case "deprecated", "scheduledfordeprecation":
		return model.ImageDeprecated
	default:
		return model.ImageUnavailable
	}
}

func normalizeArch(v string) model.OSArchitecture {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case "arm64", "aarch64":
		return model.ARM64
	case "x64", "x86_64", "amd64":
		return model.X86_64
	default:
		return model.ArchitectureNA
	}
}

func normalizePlatform(v string) model.OSPlatform {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case "linux":
		return model.Linux_UNIX
	case "windows":
		return model.Windows
	default:
		return model.PlatformNA
	}
}

func isTransientAzureError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "http2: client connection lost") ||
		strings.Contains(s, "too many requests") ||
		strings.Contains(s, "statuscode=429") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "temporary") ||
		strings.Contains(s, "eof")
}

func callWithRetry[T any](ctx context.Context, op string, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		resp, err := fn()
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isTransientAzureError(err) || i == maxRetries-1 {
			break
		}

		backoff := time.Duration(1<<i) * 500 * time.Millisecond
		log.Warn().Err(err).Str("operation", op).Dur("backoff", backoff).Msg("[AzureImage] transient error, retrying")
		time.Sleep(backoff)
	}

	return zero, fmt.Errorf("%s failed after retries: %w", op, lastErr)
}
