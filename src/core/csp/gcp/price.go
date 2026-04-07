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

package gcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// gcpPricingSubPages lists the GCP Compute Engine pricing sub-page paths.
// Each sub-page contains an AF_initDataCallback JS block with all-region pricing
// for that machine family category.
var gcpPricingSubPages = []string{
	"https://cloud.google.com/products/compute/pricing/general-purpose?hl=en",
	"https://cloud.google.com/products/compute/pricing/compute-optimized?hl=en",
	"https://cloud.google.com/products/compute/pricing/memory-optimized?hl=en",
	"https://cloud.google.com/products/compute/pricing/storage-optimized?hl=en",
	"https://cloud.google.com/products/compute/pricing/accelerator-optimized?hl=en",
}

// vmPriceEntry holds a single VM pricing entry parsed from the pricing page.
type vmPriceEntry struct {
	MachineType string
	VCPU        string
	MemoryGiB   string
	OnDemand    float64
}

// GCPPriceCache holds all parsed GCP pricing data, keyed by region.
// Key: region code (e.g., "us-central1")
// Value: map of machine type name -> vmPriceEntry
type GCPPriceCache struct {
	mu   sync.RWMutex
	data map[string]map[string]vmPriceEntry
}

// regex patterns compiled once
var (
	// regionPattern matches "RegionDisplayName (region-code)" in the JSON data
	regionPattern = regexp.MustCompile(`([A-Z][a-zA-Z,\s]+)\s*\(([a-z]+-[a-z]+-?\w*)\)`)
	// machineNamePattern matches a machine type name in a <p> tag.
	// Used as the first step to find machine entries; vCPU/Memory are searched
	// separately within a limited forward window to prevent cross-machine spanning.
	machineNamePattern = regexp.MustCompile(`<p>([a-z][a-z0-9]+-[a-z]+-[a-z0-9-]+)</p>`)
	// vcpuStdPattern matches standard vCPU: <p>32</p>
	vcpuStdPattern = regexp.MustCompile(`<p>(\d+(?:\.\d+)?)</p>`)
	// vcpuAccelPattern matches accelerator vCPU: <p>vCPUs: 32</p>
	vcpuAccelPattern = regexp.MustCompile(`<p>vCPUs:\s*(\d+)</p>`)
	// memPattern matches memory: <p>128 GiB</p> or <p>3,968GB</p>
	memPattern = regexp.MustCompile(`<p>([0-9.,]+)\s*(?:GiB|GB)</p>`)
	// memAccelPattern matches accelerator memory: <p>Memory: 128GiB</p>
	memAccelPattern = regexp.MustCompile(`<p>Memory:\s*([0-9.,]+)\s*(?:GiB|GB)</p>`)
	// pricePattern matches "$X.XXXXXX / 1 hour"
	pricePattern = regexp.MustCompile(`\$([0-9.]+)\s*/\s*1\s*hour`)
	// spanTagPattern matches opening <span ...> tags that some machine families
	// (e.g., c4a-*-lssd, g2-standard-32) wrap around vCPU/Memory cell values.
	// These must be stripped before regex matching to prevent match failures.
	spanTagPattern = regexp.MustCompile(`<span[^>]*>`)
	// afCallbackPattern extracts the AF_initDataCallback JS object
	afCallbackPattern = regexp.MustCompile(`AF_initDataCallback\((\{.*?\})\);`)

	// maxMachineFieldWindow is the maximum chars to look ahead from a machine name
	// for vCPU and memory fields. This prevents matching fields from the next machine.
	maxMachineFieldWindow = 600
)

// FetchAllGCPPrices fetches all 5 GCP pricing sub-pages and returns
// a GCPPriceCache containing region -> machine_type -> price mappings.
func FetchAllGCPPrices() (*GCPPriceCache, error) {
	cache := &GCPPriceCache{
		data: make(map[string]map[string]vmPriceEntry),
	}

	type fetchResult struct {
		url  string
		body string
		err  error
	}

	results := make(chan fetchResult, len(gcpPricingSubPages))
	client := &http.Client{Timeout: 3 * time.Minute}

	// Fetch all sub-pages in parallel
	for _, pageURL := range gcpPricingSubPages {
		go func(url string) {
			body, err := fetchPage(client, url)
			results <- fetchResult{url: url, body: body, err: err}
		}(pageURL)
	}

	// Collect results and parse
	var fetchErrors []string
	for i := 0; i < len(gcpPricingSubPages); i++ {
		r := <-results
		if r.err != nil {
			fetchErrors = append(fetchErrors, fmt.Sprintf("%s: %v", r.url, r.err))
			continue
		}

		pageName := extractPageName(r.url)
		entries, err := parsePricingPage(r.body)
		if err != nil {
			fetchErrors = append(fetchErrors, fmt.Sprintf("parse %s: %v", pageName, err))
			continue
		}

		// Merge into cache
		cache.mu.Lock()
		for region, machines := range entries {
			if cache.data[region] == nil {
				cache.data[region] = make(map[string]vmPriceEntry, len(machines))
			}
			for mt, entry := range machines {
				cache.data[region][mt] = entry
			}
		}
		cache.mu.Unlock()

		sampleMachineCount := 0
		for _, machines := range entries {
			sampleMachineCount += len(machines)
			break // just count one region for per-region count
		}
		log.Info().Msgf("GCP pricing: parsed %s - %d regions, ~%d machine types/region",
			pageName, len(entries), sampleMachineCount)
	}

	if len(fetchErrors) == len(gcpPricingSubPages) {
		return nil, fmt.Errorf("all GCP pricing page fetches failed: %s", strings.Join(fetchErrors, "; "))
	}
	if len(fetchErrors) > 0 {
		log.Warn().Msgf("GCP pricing: %d/%d pages had errors: %s",
			len(fetchErrors), len(gcpPricingSubPages), strings.Join(fetchErrors, "; "))
	}

	totalRegions := len(cache.data)
	totalEntries := 0
	for _, machines := range cache.data {
		totalEntries += len(machines)
	}
	log.Info().Msgf("GCP pricing: total %d regions, %d machine-type entries", totalRegions, totalEntries)

	return cache, nil
}

// GetPriceForRegion returns a SpiderCloudPrice containing all machine type prices
// for a specific region. This is compatible with the existing cb-tumblebug price processing.
func (c *GCPPriceCache) GetPriceForRegion(region string) model.SpiderCloudPrice {
	c.mu.RLock()
	machines, ok := c.data[region]
	c.mu.RUnlock()

	if !ok || len(machines) == 0 {
		return model.SpiderCloudPrice{}
	}

	priceList := make([]model.SpiderPrice, 0, len(machines))
	for _, entry := range machines {
		priceList = append(priceList, model.SpiderPrice{
			ProductInfo: model.SpiderProductInfo{
				VMSpecName: entry.MachineType,
			},
			PriceInfo: model.SpiderPriceInfo{
				OnDemand: model.SpiderOnDemand{
					PricingId: "GCP-Direct",
					Unit:      "Hour",
					Currency:  "USD",
					Price:     fmt.Sprintf("%f", entry.OnDemand),
				},
			},
		})
	}

	return model.SpiderCloudPrice{PriceList: priceList}
}

// Regions returns a sorted list of all region codes in the cache.
func (c *GCPPriceCache) Regions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	regions := make([]string, 0, len(c.data))
	for r := range c.data {
		regions = append(regions, r)
	}
	sort.Strings(regions)
	return regions
}

// fetchPage downloads a page body with appropriate headers.
func fetchPage(client *http.Client, url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body %s: %w", url, err)
	}
	return string(body), nil
}

// extractPageName extracts a short name from the URL for logging.
func extractPageName(url string) string {
	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		name := strings.Split(parts[i], "?")[0]
		if name != "" {
			return name
		}
	}
	return url
}

// parsePricingPage parses a single GCP pricing sub-page HTML and extracts
// all region -> machine_type -> price mappings.
// Returns: map[regionCode]map[machineType]vmPriceEntry
func parsePricingPage(html string) (map[string]map[string]vmPriceEntry, error) {
	// Find AF_initDataCallback block
	dataArray, err := extractDataArray(html)
	if err != nil {
		return nil, err
	}

	// data[0][2] contains the sections (machine families)
	topLevel, ok := dataArray.([]interface{})
	if !ok || len(topLevel) == 0 {
		return nil, fmt.Errorf("unexpected data format: top level not array")
	}
	firstItem, ok := topLevel[0].([]interface{})
	if !ok || len(firstItem) < 3 {
		return nil, fmt.Errorf("unexpected data format: data[0] too short")
	}
	sections, ok := firstItem[2].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: data[0][2] not array")
	}

	results := make(map[string]map[string]vmPriceEntry)

	for _, sec := range sections {
		secArr, ok := sec.([]interface{})
		if !ok || len(secArr) == 0 {
			continue
		}
		// Only process type=117 sections (pricing data sections)
		secType, _ := secArr[0].(float64)
		if secType != 117 {
			continue
		}

		// Convert section to string for regex processing
		secStr, err := json.Marshal(sec)
		if err != nil {
			continue
		}
		decoded := decodeUnicodeEscapes(string(secStr))

		parseSection(decoded, results)
	}

	return results, nil
}

// extractDataArray extracts and parses the JSON data array from the AF_initDataCallback block.
func extractDataArray(html string) (interface{}, error) {
	cbStart := strings.Index(html, "AF_initDataCallback(")
	if cbStart < 0 {
		return nil, fmt.Errorf("AF_initDataCallback not found")
	}

	cbMatch := afCallbackPattern.FindStringSubmatch(html[cbStart:])
	if cbMatch == nil {
		return nil, fmt.Errorf("AF_initDataCallback pattern not matched")
	}
	jsBlock := cbMatch[1]

	// Extract the data array from the JS object: {key: '...', hash: '...', data: [...]}
	dataIdx := strings.Index(jsBlock, "data:")
	if dataIdx < 0 {
		return nil, fmt.Errorf("data: field not found in AF_initDataCallback")
	}

	dataStart := strings.Index(jsBlock[dataIdx:], "[")
	if dataStart < 0 {
		return nil, fmt.Errorf("data array start not found")
	}
	dataStart += dataIdx

	// Find matching bracket using depth counting
	depth := 0
	end := dataStart
	for i := dataStart; i < len(jsBlock); i++ {
		switch jsBlock[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i + 1
				goto found
			}
		}
	}
	return nil, fmt.Errorf("matching bracket not found for data array")

found:
	dataStr := jsBlock[dataStart:end]

	var data interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return data, nil
}

// decodeUnicodeEscapes converts \u003c → <, \u003e → >, etc.
// json.Marshal produces these escapes, but we need literal < > for regex matching.
func decodeUnicodeEscapes(s string) string {
	s = strings.ReplaceAll(s, `\u003c`, "<")
	s = strings.ReplaceAll(s, `\u003e`, ">")
	s = strings.ReplaceAll(s, `\u0026`, "&")
	s = strings.ReplaceAll(s, `\"`, `"`)
	return s
}

// token represents a parsed element found in the section data.
type token struct {
	pos  int
	kind string // "REGION", "MACHINE", "PRICE"
	val  string // region code, or "machine|vcpu|mem", or price string
}

// parseSection extracts region -> machine -> price mappings from a section string
// and merges them into the results map.
func parseSection(sectionStr string, results map[string]map[string]vmPriceEntry) {
	// Strip <span> tags that some machine families (e.g., c4a-*-lssd) wrap around
	// vCPU/Memory values: <p><span style="...">16</span></p> → <p>16</p>
	sectionStr = spanTagPattern.ReplaceAllString(sectionStr, "")
	sectionStr = strings.ReplaceAll(sectionStr, "</span>", "")

	var tokens []token

	// Find regions: "CityName (region-code)"
	for _, m := range regionPattern.FindAllStringSubmatchIndex(sectionStr, -1) {
		name := strings.TrimSpace(sectionStr[m[2]:m[3]])
		code := sectionStr[m[4]:m[5]]
		// Skip false positives (e.g., "Compute Flexible CUD..." starts with capital)
		if len(name) > 2 && !strings.HasPrefix(name, "Compute") && !strings.HasPrefix(name, "Resource") {
			tokens = append(tokens, token{pos: m[0], kind: "REGION", val: code})
		}
	}

	// Find machine rows by first finding machine names, then searching a limited
	// forward window for vCPU and memory fields. This prevents the regex from
	// spanning across machine boundaries (e.g., spot entries without vCPU/Memory
	// stealing data from the next machine).
	for _, m := range machineNamePattern.FindAllStringSubmatchIndex(sectionStr, -1) {
		machine := sectionStr[m[2]:m[3]]
		nameEnd := m[1]

		// Search within a limited forward window for vCPU and memory
		windowEnd := nameEnd + maxMachineFieldWindow
		if windowEnd > len(sectionStr) {
			windowEnd = len(sectionStr)
		}
		window := sectionStr[nameEnd:windowEnd]

		var vcpu, mem string

		// Try accelerator format first: <p>vCPUs: N</p> ... <p>Memory: X GiB</p>
		if vc := vcpuAccelPattern.FindStringSubmatch(window); vc != nil {
			if mm := memAccelPattern.FindStringSubmatch(window); mm != nil {
				vcpu = vc[1]
				mem = mm[1] + " GiB"
			}
		}

		// Fall back to standard format: <p>N</p> ... <p>X GiB</p>
		if vcpu == "" {
			if vc := vcpuStdPattern.FindStringSubmatch(window); vc != nil {
				if mm := memPattern.FindStringSubmatch(window); mm != nil {
					vcpu = vc[1]
					mem = mm[1] + " GiB"
				}
			}
		}

		if vcpu != "" && mem != "" {
			tokens = append(tokens, token{pos: m[0], kind: "MACHINE", val: machine + "|" + vcpu + "|" + mem})
		}
	}

	// Find prices: $X.XXXXXX / 1 hour
	for _, m := range pricePattern.FindAllStringSubmatchIndex(sectionStr, -1) {
		price := sectionStr[m[2]:m[3]]
		tokens = append(tokens, token{pos: m[0], kind: "PRICE", val: price})
	}

	// Sort by position in the string
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].pos < tokens[j].pos })

	if len(tokens) == 0 {
		return
	}

	// Process tokens linearly:
	// Pattern: REGION ... [MACHINE PRICE PRICE PRICE PRICE PRICE]+ ... REGION ...
	// The first set of machine rows appears after the initial region list
	// (the last region in the initial list is the default region).
	// After that, each region block has: REGION, then machine rows with prices.
	var currentRegion string

	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch t.kind {
		case "REGION":
			currentRegion = t.val
			i++
		case "MACHINE":
			parts := strings.SplitN(t.val, "|", 3)
			if len(parts) != 3 || currentRegion == "" {
				i++
				continue
			}
			machineType, vcpu, mem := parts[0], parts[1], parts[2]

			// Collect the first price (on-demand) from subsequent PRICE tokens.
			// Each machine row is followed by 5 prices:
			// [Default, CUD-Flex-1yr, CUD-Flex-3yr, CUD-Res-1yr, CUD-Res-3yr]
			// We only need the first one (on-demand Default).
			j := i + 1
			var onDemandPrice float64
			priceFound := false
			for j < len(tokens) && tokens[j].kind == "PRICE" {
				if !priceFound {
					p := parseFloat(tokens[j].val)
					if p > 0 {
						onDemandPrice = p
						priceFound = true
					}
				}
				j++
			}

			if priceFound {
				if results[currentRegion] == nil {
					results[currentRegion] = make(map[string]vmPriceEntry)
				}
				// Keep the highest price per region+machine. The section data contains
				// multiple pricing tiers (on-demand, CUD-1yr, CUD-3yr) and CUD tables
				// may appear before on-demand. The on-demand price is always the highest.
				existing, exists := results[currentRegion][machineType]
				if !exists || onDemandPrice > existing.OnDemand {
					results[currentRegion][machineType] = vmPriceEntry{
						MachineType: machineType,
						VCPU:        vcpu,
						MemoryGiB:   mem,
						OnDemand:    onDemandPrice,
					}
				}
			}
			i = j
		default:
			i++
		}
	}
}

// parseFloat converts a string to float64, returning 0 on error.
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
