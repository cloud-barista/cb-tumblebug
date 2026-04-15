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

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricetypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// awsPriceItem is a minimal representation of an AWS Pricing API product JSON string.
// Each element in GetProductsOutput.PriceList is a JSON string with this structure.
type awsPriceItem struct {
	Product struct {
		ProductFamily string `json:"productFamily"`
		Attributes    struct {
			RegionCode      string `json:"regionCode"`
			InstanceType    string `json:"instanceType"`
			OperatingSystem string `json:"operatingSystem"`
			Tenancy         string `json:"tenancy"`
			PreInstalledSw  string `json:"preInstalledSw"`
			CapacitystStatus string `json:"capacitystatus"`
		} `json:"attributes"`
	} `json:"product"`
	Terms struct {
		OnDemand map[string]awsOfferTerm `json:"OnDemand"`
	} `json:"terms"`
}

type awsOfferTerm struct {
	Sku             string                      `json:"sku"`
	PriceDimensions map[string]awsPriceDimension `json:"priceDimensions"`
}

type awsPriceDimension struct {
	RateCode     string            `json:"rateCode"`
	Unit         string            `json:"unit"`
	PricePerUnit map[string]string `json:"pricePerUnit"`
}

// priceEntry stores the best (lowest) price for a (region, instanceType) pair.
type priceEntry struct {
	regionCode   string
	instanceType string
	pricingID    string
	unit         string
	currency     string
	price        float64
}

// specKey is used to deduplicate pricing entries by region+instanceType.
type specKey struct{ region, instanceType string }

// FetchAllVMPrices fetches EC2 Linux OnDemand pricing for ALL AWS regions in a single
// paginated query to the AWS Pricing API (no regionCode filter), then groups results by
// region code. This eliminates the N-Spider-calls overhead of the legacy per-region path.
//
// Only one call chain is made regardless of how many AWS regions are configured in CB-TB.
// Returns map[regionCode] → SpiderCloudPrice, compatible with the existing BulkUpdateSpec path.
func FetchAllVMPrices(ctx context.Context) (map[string]model.SpiderCloudPrice, error) {
	accessKey, secretKey, err := getAWSCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS pricing: cannot get credentials: %w", err)
	}

	// AWS Pricing API is only available at these 3 endpoints;
	// "us-east-1" works for all AWS regions' pricing data.
	cfg := awssdk.Config{
		Region:      "us-east-1",
		Credentials: awscreds.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}
	client := pricing.NewFromConfig(cfg)

	// Filters mirror what Spider's AwsPriceInfoHandler applies, except we deliberately
	// omit the regionCode filter to retrieve all regions in a single pass.
	input := &pricing.GetProductsInput{
		ServiceCode: awssdk.String("AmazonEC2"),
		Filters: []pricetypes.Filter{
			{
				Type:  pricetypes.FilterTypeTermMatch,
				Field: awssdk.String("marketoption"),
				Value: awssdk.String("OnDemand"),
			},
			{
				Type:  pricetypes.FilterTypeTermMatch,
				Field: awssdk.String("operatingSystem"),
				Value: awssdk.String("Linux"),
			},
			{
				Type:  pricetypes.FilterTypeTermMatch,
				Field: awssdk.String("tenancy"),
				Value: awssdk.String("Shared"),
			},
			{
				Type:  pricetypes.FilterTypeTermMatch,
				Field: awssdk.String("preInstalledSw"),
				Value: awssdk.String("NA"),
			},
			{
				Type:  pricetypes.FilterTypeTermMatch,
				Field: awssdk.String("capacitystatus"),
				Value: awssdk.String("Used"),
			},
			{
				Type:  pricetypes.FilterTypeTermMatch,
				Field: awssdk.String("currentGeneration"),
				Value: awssdk.String("Yes"),
			},
		},
		MaxResults: awssdk.Int32(100),
	}

	// Keep the lowest price per (regionCode, instanceType).
	bestByKey := map[specKey]priceEntry{}

	paginator := pricing.NewGetProductsPaginator(client, input)
	pageCount := 0
	parseErrors := 0

	for paginator.HasMorePages() {
		page, pageErr := paginator.NextPage(ctx)
		if pageErr != nil {
			return nil, fmt.Errorf("AWS pricing API error fetching page %d: %w", pageCount+1, pageErr)
		}
		pageCount++

		for _, jsonStr := range page.PriceList {
			entry, ok := parseAWSPriceItem(jsonStr)
			if !ok {
				parseErrors++
				continue
			}

			k := specKey{entry.regionCode, entry.instanceType}
			if prev, found := bestByKey[k]; !found || entry.price < prev.price {
				bestByKey[k] = entry
			}
		}
	}

	log.Info().
		Int("pages", pageCount).
		Int("uniqueRegionSpecPairs", len(bestByKey)).
		Int("parseErrors", parseErrors).
		Msg("AWS: pricing global fetch complete")

	return buildSpiderCloudPriceMap(bestByKey), nil
}

// parseAWSPriceItem unmarshals one element from GetProductsOutput.PriceList and extracts
// the (regionCode, instanceType, lowest OnDemand price) tuple.
// Returns (entry, true) on success, (zero, false) if the item should be skipped.
func parseAWSPriceItem(jsonStr string) (priceEntry, bool) {
	var item awsPriceItem
	if err := json.Unmarshal([]byte(jsonStr), &item); err != nil {
		return priceEntry{}, false
	}

	pf := item.Product.ProductFamily
	if pf != "Compute Instance" && pf != "Compute Instance (bare metal)" {
		return priceEntry{}, false
	}

	region := strings.TrimSpace(item.Product.Attributes.RegionCode)
	instanceType := strings.TrimSpace(item.Product.Attributes.InstanceType)
	if region == "" || instanceType == "" {
		return priceEntry{}, false
	}

	// Extract the lowest USD price across all OnDemand dimensions.
	var bestPrice float64 = -1
	var bestEntry priceEntry

	for _, term := range item.Terms.OnDemand {
		for _, dim := range term.PriceDimensions {
			usdStr, ok := dim.PricePerUnit["USD"]
			if !ok {
				continue
			}
			p, err := strconv.ParseFloat(strings.TrimSpace(usdStr), 64)
			if err != nil || p <= 0 {
				continue
			}

			unit := dim.Unit
			if strings.EqualFold(unit, "Hrs") {
				unit = "Hour"
			}

			if bestPrice < 0 || p < bestPrice {
				bestPrice = p
				bestEntry = priceEntry{
					regionCode:   region,
					instanceType: instanceType,
					pricingID:    dim.RateCode,
					unit:         unit,
					currency:     "USD",
					price:        p,
				}
			}
		}
	}

	if bestPrice <= 0 {
		return priceEntry{}, false
	}
	return bestEntry, true
}

// buildSpiderCloudPriceMap converts the flat bestByKey map into a map of
// regionCode → model.SpiderCloudPrice, sorted by instanceType for deterministic output.
func buildSpiderCloudPriceMap(bestByKey map[specKey]priceEntry) map[string]model.SpiderCloudPrice {
	// Group by region, collecting instance types in order.
	regionItems := map[string][]priceEntry{}
	for _, entry := range bestByKey {
		regionItems[entry.regionCode] = append(regionItems[entry.regionCode], entry)
	}

	result := make(map[string]model.SpiderCloudPrice, len(regionItems))
	for region, entries := range regionItems {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].instanceType < entries[j].instanceType
		})

		priceList := make([]model.SpiderPrice, len(entries))
		for i, e := range entries {
			priceList[i] = model.SpiderPrice{
				ProductInfo: model.SpiderProductInfo{
					VMSpecName: e.instanceType,
				},
				PriceInfo: model.SpiderPriceInfo{
					OnDemand: model.SpiderOnDemand{
						PricingId: e.pricingID,
						Unit:      e.unit,
						Currency:  e.currency,
						Price:     strconv.FormatFloat(e.price, 'f', -1, 64),
					},
				},
			}
		}
		result[region] = model.SpiderCloudPrice{PriceList: priceList}
	}
	return result
}
