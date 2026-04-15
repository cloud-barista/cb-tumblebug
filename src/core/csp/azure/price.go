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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

const azureRetailPricesEndpoint = "https://prices.azure.com/api/retail/prices"

type azureRetailPage struct {
	Items        []azureRetailItem `json:"Items"`
	NextPageLink string            `json:"NextPageLink"`
}

type azureRetailItem struct {
	ServiceName   string  `json:"serviceName"`
	ProductName   string  `json:"productName"`
	SkuName       string  `json:"skuName"`
	ArmSkuName    string  `json:"armSkuName"`
	SkuID         string  `json:"skuId"`
	CurrencyCode  string  `json:"currencyCode"`
	RetailPrice   float64 `json:"retailPrice"`
	UnitOfMeasure string  `json:"unitOfMeasure"`
}

// FetchVMPricesByRegion fetches Azure VM prices directly from Azure Retail Prices API.
// It returns only fields required by cb-tumblebug's current spec price update flow.
func FetchVMPricesByRegion(region string) (model.SpiderCloudPrice, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		return model.SpiderCloudPrice{}, fmt.Errorf("region is empty")
	}

	filter := fmt.Sprintf("serviceName eq 'Virtual Machines' and priceType eq 'Consumption' and armRegionName eq '%s'", region)
	nextURL := azureRetailPricesEndpoint + "?$filter=" + url.QueryEscape(filter)

	client := &http.Client{Timeout: 30 * time.Second}

	// Keep the lowest non-negative unit price per ArmSkuName.
	// This mirrors Spider's dedupe intent while returning only the minimal data needed by TB.
	bestBySpec := map[string]azureRetailItem{}

	for nextURL != "" {
		page, err := fetchAzureRetailPage(client, nextURL)
		if err != nil {
			return model.SpiderCloudPrice{}, err
		}

		for _, item := range page.Items {
			if !strings.EqualFold(item.ServiceName, "Virtual Machines") {
				continue
			}
			if strings.TrimSpace(item.ArmSkuName) == "" {
				continue
			}

			// Keep Linux regular VM prices only (same intent as Spider path).
			if strings.Contains(item.ProductName, "Windows") ||
				strings.Contains(item.ProductName, "Cloud Services") ||
				strings.Contains(item.ProductName, "CloudServices") {
				continue
			}
			if strings.Contains(item.SkuName, "Low Priority") || strings.Contains(item.SkuName, "Spot") {
				continue
			}

			if prev, ok := bestBySpec[item.ArmSkuName]; ok {
				if item.RetailPrice >= prev.RetailPrice {
					continue
				}
			}
			bestBySpec[item.ArmSkuName] = item
		}

		nextURL = strings.TrimSpace(page.NextPageLink)
	}

	if len(bestBySpec) == 0 {
		return model.SpiderCloudPrice{}, nil
	}

	keys := make([]string, 0, len(bestBySpec))
	for spec := range bestBySpec {
		keys = append(keys, spec)
	}
	sort.Strings(keys)

	priceList := make([]model.SpiderPrice, 0, len(keys))
	for _, spec := range keys {
		item := bestBySpec[spec]
		priceList = append(priceList, model.SpiderPrice{
			ProductInfo: model.SpiderProductInfo{VMSpecName: spec},
			PriceInfo: model.SpiderPriceInfo{
				OnDemand: model.SpiderOnDemand{
					PricingId:   item.SkuID,
					Unit:        strings.TrimPrefix(item.UnitOfMeasure, "1 "),
					Currency:    item.CurrencyCode,
					Price:       strconv.FormatFloat(item.RetailPrice, 'f', 6, 64),
					Description: "NA",
				},
			},
		})
	}

	return model.SpiderCloudPrice{PriceList: priceList}, nil
}

func fetchAzureRetailPage(client *http.Client, pageURL string) (azureRetailPage, error) {
	const maxAttempts = 4

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodGet, pageURL, nil)
		if err != nil {
			return azureRetailPage{}, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			break
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return azureRetailPage{}, readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("azure retail prices temporary failure (status=%d, body=%s)", resp.StatusCode, truncateText(string(body), 200))
			if attempt < maxAttempts {
				wait := time.Duration(attempt) * time.Second
				if retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After")); retryAfter != "" {
					if sec, convErr := strconv.Atoi(retryAfter); convErr == nil && sec > 0 {
						wait = time.Duration(sec) * time.Second
					}
				}
				time.Sleep(wait)
				continue
			}
			break
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return azureRetailPage{}, fmt.Errorf("azure retail prices returned status=%d (body=%s)", resp.StatusCode, truncateText(string(body), 200))
		}

		var page azureRetailPage
		if err := json.Unmarshal(body, &page); err != nil {
			return azureRetailPage{}, fmt.Errorf("failed to parse azure retail prices response (status=%d, body=%s): %w", resp.StatusCode, truncateText(string(body), 200), err)
		}

		return page, nil
	}

	if lastErr != nil {
		return azureRetailPage{}, lastErr
	}

	return azureRetailPage{}, fmt.Errorf("failed to fetch azure retail prices page")
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
