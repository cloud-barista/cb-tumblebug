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
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TbSpecReqStructLevelValidation is a function to validate 'TbSpecReq' object.
func TbSpecReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbSpecReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// ConvertSpiderSpecToTumblebugSpec accepts an Spider spec object, converts to and returns an TB spec object
func ConvertSpiderSpecToTumblebugSpec(providerName string, spiderSpec model.SpiderSpecInfo) (model.TbSpecInfo, error) {
	if spiderSpec.Name == "" {
		err := fmt.Errorf("convertSpiderSpecToTumblebugSpec failed; spiderSpec.Name == \"\" ")
		emptyTumblebugSpec := model.TbSpecInfo{}
		return emptyTumblebugSpec, err
	}

	tumblebugSpec := model.TbSpecInfo{}

	tumblebugSpec.Name = spiderSpec.Name
	tumblebugSpec.CspSpecName = spiderSpec.Name
	tumblebugSpec.RegionName = spiderSpec.Region
	tumblebugSpec.ProviderName = providerName

	tempUint64, _ := strconv.ParseUint(spiderSpec.VCpu.Count, 10, 16)
	tumblebugSpec.VCPU = uint16(tempUint64)
	tempFloat64, _ := strconv.ParseFloat(spiderSpec.MemSizeMiB, 32)
	tumblebugSpec.MemoryGiB = float32(tempFloat64 / 1024)
	tempFloat64, _ = strconv.ParseFloat(spiderSpec.DiskSizeGB, 32)
	tumblebugSpec.DiskSizeGB = float32(tempFloat64)

	tumblebugSpec.Details = spiderSpec.KeyValueList

	// Extract Architecture based on CSP
	tumblebugSpec.Architecture = extractArchitecture(tumblebugSpec.ProviderName, tumblebugSpec.Details, tumblebugSpec.CspSpecName)
	if tumblebugSpec.Architecture == string(model.ArchitectureUnknown) {
		log.Debug().Msgf("(%s) architecture for spec %s: %s", tumblebugSpec.ProviderName, tumblebugSpec.CspSpecName, tumblebugSpec.Architecture)
	}

	// GPU(Accelerator) information conversion
	if len(spiderSpec.Gpu) > 0 {
		// Set AcceleratorType to "gpu" when GPU exists
		tumblebugSpec.AcceleratorType = "gpu"

		// Use the first GPU information
		firstGpu := spiderSpec.Gpu[0]

		// Combine Mfr and Model to form AcceleratorModel
		if firstGpu.Mfr != "" && firstGpu.Model != "" {
			// Check if Model already starts with Mfr to avoid duplication
			if strings.HasPrefix(firstGpu.Model, firstGpu.Mfr) {
				// Model already includes Mfr, so just use Model
				tumblebugSpec.AcceleratorModel = firstGpu.Model
			} else {
				// Model doesn't include Mfr, so combine them
				tumblebugSpec.AcceleratorModel = firstGpu.Mfr + " " + firstGpu.Model
			}
		} else if firstGpu.Model != "" {
			tumblebugSpec.AcceleratorModel = firstGpu.Model
		} else if firstGpu.Mfr != "" {
			tumblebugSpec.AcceleratorModel = firstGpu.Mfr
		}

		// Convert GPU count
		if firstGpu.Count != "" && firstGpu.Count != "-1" {
			tempCount, err := strconv.ParseUint(firstGpu.Count, 10, 8)
			if err == nil {
				tumblebugSpec.AcceleratorCount = uint8(tempCount)
			}
		}

		// Convert GPU memory size
		if firstGpu.MemSizeGB != "" && firstGpu.MemSizeGB != "-1" {
			tempMemory, err := strconv.ParseFloat(firstGpu.MemSizeGB, 32)
			if err == nil {
				tumblebugSpec.AcceleratorMemoryGB = float32(tempMemory)
			}
		}

		// Log if there are multiple GPUs defined
		if len(spiderSpec.Gpu) > 1 {
			log.Warn().Msgf("Spec %s has multiple GPUs defined (%d GPUs). Only using the first GPU information.",
				spiderSpec.Name, len(spiderSpec.Gpu))
		}
	}

	return tumblebugSpec, nil
}

// extractArchitecture extracts architecture information based on CSP-specific logic
func extractArchitecture(providerName string, details []model.KeyValue, cspSpecName string) string {

	// FYI model.OSArchitecture is defined in src/core/model/OSArchitecture.go
	// 	const (
	// 	ARM32          OSArchitecture = "arm32"
	// 	ARM64          OSArchitecture = "arm64"
	// 	ARM64_MAC      OSArchitecture = "arm64_mac"
	// 	X86_32         OSArchitecture = "x86_32"
	// 	X86_64         OSArchitecture = "x86_64"
	// 	X86_32_MAC     OSArchitecture = "x86_32_mac"
	// 	X86_64_MAC     OSArchitecture = "x86_64_mac"
	// 	S390X          OSArchitecture = "s390x"
	// 	ArchitectureNA OSArchitecture = "NA"
	// )

	switch providerName {
	case csp.AWS:
		// For AWS, look for ProcessorInfo and extract SupportedArchitectures from its value
		archInfo := common.LookupKeyValueList(details, "ProcessorInfo")
		if archInfo != "" {
			// Parse the SupportedArchitectures from ProcessorInfo value
			// Examples:
			// "{SupportedArchitectures:[arm64],SustainedClockSpeedInGhz:2.6}"
			// "{SupportedArchitectures:[x86_64_mac],SustainedClockSpeedInGhz:3.2}"
			// "{SupportedArchitectures:[i386,x86_64],SustainedClockSpeedInGhz:2.5}"

			if strings.Contains(archInfo, "arm64_mac") {
				return string(model.ARM64_MAC)
			} else if strings.Contains(archInfo, "x86_64_mac") {
				return string(model.X86_64_MAC)
			} else if strings.Contains(archInfo, "arm64") {
				return string(model.ARM64)
			} else if strings.Contains(archInfo, "x86_64") {
				return string(model.X86_64)
			} else if strings.Contains(archInfo, "i386") {
				return string(model.X86_32)
			} else {
				return archInfo
			}
		}
		// Fallback: check instance name patterns
		// if strings.HasPrefix(cspSpecName, "mac1") {
		// 	return string(model.X86_64_MAC)
		// } else if strings.HasPrefix(cspSpecName, "mac2") {
		// 	return string(model.ARM64_MAC)
		// }

	case csp.Alibaba:
		// For Alibaba, CpuArchitecture is a direct key
		archInfo := strings.ToLower(common.LookupKeyValueList(details, "CpuArchitecture"))
		if archInfo != "" {
			if strings.Contains(archInfo, strings.ToLower("ARM")) {
				return string(model.ARM64)
			} else if strings.Contains(archInfo, strings.ToLower("X86")) {
				return string(model.X86_64)
			} else {
				return archInfo
			}
		}

	case csp.IBM:
		// For IBM, look for VcpuArchitecture and extract the actual value
		archInfo := common.LookupKeyValueList(details, "VcpuArchitecture")
		if archInfo != "" {
			// Parse the value from "{type:fixed,value:amd64}"
			if strings.Contains(archInfo, "s390x") {
				return string(model.S390X)
			} else if strings.Contains(archInfo, "amd64") {
				return string(model.X86_64)
			} else {
				return archInfo
			}
		}

	case csp.Tencent:
		// ref: https://www.tencentcloud.com/document/product/213/11518
		patterns := []string{
			"sr1.", // Standard ARM (Ampere Altra)
		}

		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(cspSpecName), strings.ToLower(pattern)) {
				return string(model.ARM64)
			}
		}
		return string(model.X86_64)

	case csp.Azure:
		// Azure doesn't provide architecture in details, use instance name patterns
		// Check for ARM-specific patterns
		patterns := []string{
			"Ep", "Dp",
		}

		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(cspSpecName), strings.ToLower(pattern)) {
				return string(model.ARM64)
			}
		}
		return string(model.X86_64)

	case csp.GCP:
		// ref: https://cloud.google.com/compute/docs/cpu-platforms
		// GCP doesn't provide architecture in details, use instance name patterns
		// Check for ARM-specific patterns
		patterns := []string{
			"t2a", "c2a",
		}

		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(cspSpecName), strings.ToLower(pattern)) {
				return string(model.ARM64)
			}
		}
		return string(model.X86_64)

	case csp.KTCloud:
		return string(model.X86_64)

	case csp.NCP:
		return string(model.X86_64)

	case csp.NHNCloud:
		return string(model.X86_64)

	default:
		// For unknown CSPs
		return string(model.X86_64)
	}
	return string(model.ArchitectureUnknown)
}

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
	client := resty.New()
	client.SetTimeout(10 * time.Minute)
	url := model.SpiderRestUrl + "/vmspec"
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig

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

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	url := model.SpiderRestUrl + "/vmspec/" + specName
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderSpecInfo{}

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
		log.Error().Err(err).Msg("")
		return callResult, err
	}

	return callResult, nil
}

// FetchSpecsForConnConfig lookups all specs for region of conn config, and saves into TB spec objects
func FetchSpecsForConnConfig(connConfigName string, nsId string) (uint, error) {
	log.Debug().Msg("FetchSpecsForConnConfig(" + connConfigName + ")")
	specCount := uint(0)

	connConfig, err := common.GetConnConfig(connConfigName)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot GetConnConfig in %s", connConfigName)
		return specCount, err
	}

	specsInConnection, err := LookupSpecList(connConfigName)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot LookupSpecList in %s", connConfigName)
		return specCount, err
	}

	for _, spec := range specsInConnection.Vmspec {
		spiderSpec := spec
		//log.Info().Msgf("Found spec in the map: %s", spiderSpec.Name)
		tumblebugSpec, errConvert := ConvertSpiderSpecToTumblebugSpec(connConfig.ProviderName, spiderSpec)
		if errConvert != nil {
			log.Error().Err(errConvert).Msg("Cannot ConvertSpiderSpecToTumblebugSpec")
		} else {
			key := GetProviderRegionZoneResourceKey(connConfig.ProviderName, connConfig.RegionDetail.RegionName, "", spec.Name)
			tumblebugSpec.Name = key
			tumblebugSpec.ConnectionName = connConfig.ConfigName
			tumblebugSpec.ProviderName = strings.ToLower(connConfig.ProviderName)
			tumblebugSpec.RegionName = connConfig.RegionDetail.RegionName
			tumblebugSpec.InfraType = "vm" // default value
			tumblebugSpec.SystemLabel = "auto-gen"
			tumblebugSpec.CostPerHour = -1
			tumblebugSpec.EvaluationScore01 = -99.9

			_, err := RegisterSpecWithInfo(nsId, &tumblebugSpec, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return 0, err
			}
			specCount++
		}

	}
	return specCount, nil
}

// FetchSpecsForAllConnConfigs gets all conn configs from Spider, lookups all specs for each region of conn config, and saves into TB spec objects
func FetchSpecsForAllConnConfigs(nsId string) (connConfigCount uint, specCount uint, err error) {

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, 0, err
	}

	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, 0, err
	}

	for _, connConfig := range connConfigs.Connectionconfig {
		temp, _ := FetchSpecsForConnConfig(connConfig.ConfigName, nsId)
		specCount += temp
		connConfigCount++
	}
	return connConfigCount, specCount, nil
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

// lastSpecFetchResult stores the result of the most recent fetch images operation
var lastSpecFetchResult struct {
	sync.RWMutex
	Result map[string]*FetchSpecsAsyncResult
}

func init() {
	lastSpecFetchResult.Result = make(map[string]*FetchSpecsAsyncResult)
}

func updateFetchSpecsProgress(nsId string, result *FetchSpecsAsyncResult) {
	lastSpecFetchResult.Lock()
	lastSpecFetchResult.Result[nsId] = result
	lastSpecFetchResult.Unlock()
}

// isSpecFetchInProgress checks if there's an ongoing image fetch operation for the given namespace
func isSpecFetchInProgress(nsId string) bool {
	lastSpecFetchResult.RLock()
	defer lastSpecFetchResult.RUnlock()

	result, exists := lastSpecFetchResult.Result[nsId]
	if exists && result != nil && result.InProgress {
		return true
	}
	return false
}

// UpdateSpecsFromAsset updates spec information based on cloudspec.csv asset file
func UpdateSpecsFromAsset(nsId string) (*FetchSpecsAsyncResult, error) {
	if nsId == "" {
		nsId = model.SystemCommonNs
	}

	startTime := time.Now()
	result := &FetchSpecsAsyncResult{
		NamespaceID: nsId,
		StartTime:   startTime,
		InProgress:  true,
	}

	// Open and read CSV file
	file, err := os.Open("../assets/cloudspec.csv")
	if err != nil {
		log.Error().Err(err).Msg("Failed to open cloudspec.csv")
		result.InProgress = false
		result.ElapsedTime = time.Since(startTime).String()
		return result, err
	}
	defer file.Close()

	rdr := csv.NewReader(bufio.NewReader(file))
	rows, err := rdr.ReadAll()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read cloudspec.csv")
		result.InProgress = false
		result.ElapsedTime = time.Since(startTime).String()
		return result, err
	}
	// row[0]	providerName
	// row[1]	regionName
	// row[2]	cspResourceId
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
	// row[17]	acceleratorModel
	// row[18]	acceleratorCount
	// row[19]	acceleratorMemoryGB
	// row[20]	acceleratorDetails
	// row[21]	infraType

	var specList []model.TbSpecInfo

	// Process each row (skip header)
	for i, row := range rows[1:] {

		// Parse CSV data directly into TbSpecInfo struct
		csvSpec := model.TbSpecInfo{
			Namespace:    nsId,
			ProviderName: strings.ToLower(row[0]),
			RegionName:   strings.ToLower(row[1]),
			CspSpecName:  row[2],
			SystemLabel:  model.StrFromAssets,
		}

		// Parse cost per hour with currency conversion
		if row[3] != "" {
			if cost, err := strconv.ParseFloat(row[3], 32); err == nil {
				currency := "USD"
				if row[4] != "" {
					currency = strings.ToUpper(row[4])
				}
				csvSpec.CostPerHour = common.ConvertToBaseCurrency(float32(cost), currency)
			}
		}

		// Parse evaluation scores (row[5] to row[14])
		if row[5] != "" {
			if score, err := strconv.ParseFloat(row[5], 32); err == nil {
				csvSpec.EvaluationScore01 = float32(score)
			}
		}
		if row[6] != "" {
			if score, err := strconv.ParseFloat(row[6], 32); err == nil {
				csvSpec.EvaluationScore02 = float32(score)
			}
		}
		if row[7] != "" {
			if score, err := strconv.ParseFloat(row[7], 32); err == nil {
				csvSpec.EvaluationScore03 = float32(score)
			}
		}
		if row[8] != "" {
			if score, err := strconv.ParseFloat(row[8], 32); err == nil {
				csvSpec.EvaluationScore04 = float32(score)
			}
		}
		if row[9] != "" {
			if score, err := strconv.ParseFloat(row[9], 32); err == nil {
				csvSpec.EvaluationScore05 = float32(score)
			}
		}
		if row[10] != "" {
			if score, err := strconv.ParseFloat(row[10], 32); err == nil {
				csvSpec.EvaluationScore06 = float32(score)
			}
		}
		if row[11] != "" {
			if score, err := strconv.ParseFloat(row[11], 32); err == nil {
				csvSpec.EvaluationScore07 = float32(score)
			}
		}
		if row[12] != "" {
			if score, err := strconv.ParseFloat(row[12], 32); err == nil {
				csvSpec.EvaluationScore08 = float32(score)
			}
		}
		if row[13] != "" {
			if score, err := strconv.ParseFloat(row[13], 32); err == nil {
				csvSpec.EvaluationScore09 = float32(score)
			}
		}
		if row[14] != "" {
			if score, err := strconv.ParseFloat(row[14], 32); err == nil {
				csvSpec.EvaluationScore10 = float32(score)
			}
		}

		// Parse disk and accelerator specifications
		if row[15] != "" {
			csvSpec.RootDiskType = row[15]
		}
		if row[16] != "" {
			csvSpec.RootDiskSize = row[16]
		}
		if row[17] != "" {
			csvSpec.AcceleratorModel = row[17]
		}
		if row[18] != "" {
			if count, err := strconv.Atoi(row[18]); err == nil {
				csvSpec.AcceleratorCount = uint8(count)
			}
		}
		if row[19] != "" {
			if memory, err := strconv.ParseFloat(row[19], 32); err == nil {
				csvSpec.AcceleratorMemoryGB = float32(memory)
			}
		}
		// row[20] is acceleratorDetails - not used currently
		if row[21] != "" {
			csvSpec.Description = row[21]
		}
		if len(row) > 22 && row[22] != "" {
			csvSpec.InfraType = strings.ToLower(row[22])
		}

		// Generate ID and Name
		csvSpec.Id = GetProviderRegionZoneResourceKey(csvSpec.ProviderName, csvSpec.RegionName, "", csvSpec.CspSpecName)
		csvSpec.Name = csvSpec.Id

		log.Debug().Msgf("Processing row %d: %s-%s-%s", i+1, csvSpec.ProviderName, csvSpec.RegionName, csvSpec.CspSpecName)

		// Check if spec already exists in database
		existingSpec, err := GetSpec(nsId, csvSpec.CspSpecName)
		if err == nil {
			// Existing spec found - merge with CSV data
			log.Info().Msgf("Found existing spec: %s, merging with CSV data", csvSpec.CspSpecName)
			mergedSpec := mergeSpecWithCSVData(existingSpec, csvSpec)
			specList = append(specList, mergedSpec)

		} else {
			// Spec not found in DB - try LookupSpec from CSP
			log.Debug().Msgf("Spec not found in DB: %s, trying LookupSpec", csvSpec.CspSpecName)

			// Create connection name for LookupSpec
			connectionName := csvSpec.ProviderName + "-" + csvSpec.RegionName

			lookupSpec, lookupErr := LookupSpec(connectionName, csvSpec.CspSpecName)
			if lookupErr != nil {
				log.Warn().Msgf("LookupSpec failed for %s-%s-%s: %s",
					csvSpec.ProviderName, csvSpec.RegionName, csvSpec.CspSpecName, lookupErr.Error())
				continue
			}

			// Convert Spider spec to Tumblebug spec and merge with CSV data
			log.Info().Msgf("Found spec via LookupSpec: %s, converting and merging", csvSpec.CspSpecName)

			tumblebugSpec, convertErr := ConvertSpiderSpecToTumblebugSpec(csvSpec.ProviderName, lookupSpec)
			if convertErr != nil {
				log.Warn().Msgf("Failed to convert Spider spec to Tumblebug spec for %s: %s",
					csvSpec.CspSpecName, convertErr.Error())
				continue
			}

			// Set basic information
			tumblebugSpec.Namespace = nsId
			tumblebugSpec.ConnectionName = connectionName
			tumblebugSpec.ProviderName = csvSpec.ProviderName
			tumblebugSpec.RegionName = csvSpec.RegionName
			tumblebugSpec.Id = csvSpec.Id
			tumblebugSpec.Name = csvSpec.Name

			// Merge with CSV data
			mergedSpec := mergeSpecWithCSVData(tumblebugSpec, csvSpec)
			specList = append(specList, mergedSpec)
		}
	}

	// Update database with bulk operation
	if len(specList) > 0 {
		err = RegisterSpecWithInfoInBulk(specList)
		if err != nil {
			log.Error().Err(err).Msg("RegisterSpecWithInfoInBulk failed")
			result.InProgress = false
			result.ElapsedTime = time.Since(startTime).String()
			return result, err
		}
		log.Info().Msgf("Updated %d specs from asset file", len(specList))
	} else {
		log.Warn().Msg("No specs were processed from the asset file")
	}

	result.InProgress = false
	result.ElapsedTime = time.Since(startTime).String()
	updateFetchSpecsProgress(nsId, result)

	return result, nil
}

// mergeSpecWithCSVData merges CSV spec data into existing spec (CSV data has priority for non-empty values)
func mergeSpecWithCSVData(existingSpec model.TbSpecInfo, csvSpec model.TbSpecInfo) model.TbSpecInfo {
	mergedSpec := existingSpec

	// Merge cost information (CSV priority for non-zero values)
	if csvSpec.CostPerHour != 0 {
		mergedSpec.CostPerHour = csvSpec.CostPerHour
	}

	// Merge evaluation scores (CSV priority for non-zero values)
	if csvSpec.EvaluationScore01 != 0 {
		mergedSpec.EvaluationScore01 = csvSpec.EvaluationScore01
	}
	if csvSpec.EvaluationScore02 != 0 {
		mergedSpec.EvaluationScore02 = csvSpec.EvaluationScore02
	}
	if csvSpec.EvaluationScore03 != 0 {
		mergedSpec.EvaluationScore03 = csvSpec.EvaluationScore03
	}
	if csvSpec.EvaluationScore04 != 0 {
		mergedSpec.EvaluationScore04 = csvSpec.EvaluationScore04
	}
	if csvSpec.EvaluationScore05 != 0 {
		mergedSpec.EvaluationScore05 = csvSpec.EvaluationScore05
	}
	if csvSpec.EvaluationScore06 != 0 {
		mergedSpec.EvaluationScore06 = csvSpec.EvaluationScore06
	}
	if csvSpec.EvaluationScore07 != 0 {
		mergedSpec.EvaluationScore07 = csvSpec.EvaluationScore07
	}
	if csvSpec.EvaluationScore08 != 0 {
		mergedSpec.EvaluationScore08 = csvSpec.EvaluationScore08
	}
	if csvSpec.EvaluationScore09 != 0 {
		mergedSpec.EvaluationScore09 = csvSpec.EvaluationScore09
	}
	if csvSpec.EvaluationScore10 != 0 {
		mergedSpec.EvaluationScore10 = csvSpec.EvaluationScore10
	}

	// Merge disk specifications (CSV priority for non-empty values)
	if csvSpec.RootDiskType != "" {
		mergedSpec.RootDiskType = csvSpec.RootDiskType
	}
	if csvSpec.RootDiskSize != "" {
		mergedSpec.RootDiskSize = csvSpec.RootDiskSize
	}

	// Merge accelerator specifications (CSV priority for non-empty/non-zero values)
	if csvSpec.AcceleratorModel != "" {
		mergedSpec.AcceleratorModel = csvSpec.AcceleratorModel
	}
	if csvSpec.AcceleratorCount != 0 {
		mergedSpec.AcceleratorCount = csvSpec.AcceleratorCount
	}
	if csvSpec.AcceleratorMemoryGB != 0 {
		mergedSpec.AcceleratorMemoryGB = csvSpec.AcceleratorMemoryGB
	}

	// Merge description and infrastructure type (CSV priority for non-empty values)
	if csvSpec.Description != "" {
		mergedSpec.Description = csvSpec.Description
	}
	if csvSpec.InfraType != "" {
		mergedSpec.InfraType = csvSpec.InfraType
	}

	// Always update SystemLabel to indicate data source
	mergedSpec.SystemLabel = model.StrFromAssets

	return mergedSpec
}

// FetchPriceForAllConnConfigs gets all conn configs from Spider, lookups all Price for each region of conn config,
// and saves into TB Price objects. This implementation uses parallel processing with concurrency control and retries failed connections once.
func FetchPriceForAllConnConfigs() (connConfigCount uint, priceCount uint, err error) {
	// Get connection configurations
	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get connection config list")
		return 0, 0, err
	}

	// Skip processing if no connections found
	if len(connConfigs.Connectionconfig) == 0 {
		log.Info().Msg("No connection configurations found")
		return 0, 0, nil
	}

	connConfigCount = uint(len(connConfigs.Connectionconfig))
	log.Info().Msgf("Starting parallel price fetching for %d connections", connConfigCount)

	// Sort connections by CSP rotation for optimal parallel processing
	sortConnectionsByCSPRotation(connConfigs.Connectionconfig)

	startTime := time.Now()

	// Control concurrency with semaphore - limit concurrent connections
	maxConcurrent := 15 // Reduced from unlimited to 15 concurrent connections
	semaphore := make(chan struct{}, maxConcurrent)

	// Function to fetch prices for a single connection with retry
	fetchPricesWithRetry := func(config model.ConnConfig) error {
		// First attempt
		err := FetchPriceForConnConfig(config)

		// If failed, retry once after random sleep
		if err != nil {
			log.Warn().Err(err).Msgf("First attempt failed for connection %s, will retry",
				config.ConfigName)

			// if err message contains "not support", skip retry
			if strings.Contains(err.Error(), "not support") {
				log.Warn().Msgf("Skipping retry for connection %s due to unsupported error",
					config.ConfigName)
				return err
			}

			// Random sleep before retry
			common.RandomSleep(2, 5)
			err = FetchPriceForConnConfig(config)
		}

		// If still failed after retry
		if err != nil {
			log.Error().Err(err).Msgf("Failed to fetch prices for connection %s after retry",
				config.ConfigName)
			return err
		}

		return nil
	}

	// Process all connections in parallel with controlled concurrency
	var wg sync.WaitGroup
	resultChan := make(chan struct {
		ConnConfig model.ConnConfig
		Err        error
	}, len(connConfigs.Connectionconfig))

	for _, connConfig := range connConfigs.Connectionconfig {
		wg.Add(1)

		// Acquire semaphore slot before starting goroutine
		semaphore <- struct{}{}

		go func(config model.ConnConfig) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore slot when done

			// Simulate random sleep to avoid overwhelming the API
			common.RandomSleep(0, 10)

			// Fetch with retry
			err := fetchPricesWithRetry(config)

			// Force garbage collection after each connection to manage memory
			runtime.GC()

			// Send result back through channel
			resultChan <- struct {
				ConnConfig model.ConnConfig
				Err        error
			}{
				ConnConfig: config,
				Err:        err,
			}
		}(connConfig)
	}

	// Wait for all goroutines to complete and close the channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	var finalErrors []string
	var successCount uint

	for result := range resultChan {
		if result.Err != nil {
			errMsg := fmt.Sprintf("Error fetching prices for connection %s: %v",
				result.ConnConfig.ConfigName, result.Err)
			finalErrors = append(finalErrors, errMsg)
			continue
		}

		// Mark as successful and collect prices
		successCount++
		log.Debug().Msgf("Successfully processed connection: %s", result.ConnConfig.ConfigName)
	}

	// Report any errors
	if len(finalErrors) > 0 {
		log.Warn().Msgf("Encountered %d errors while fetching prices after retries", len(finalErrors))
		if len(finalErrors) == len(connConfigs.Connectionconfig) {
			return connConfigCount, priceCount, fmt.Errorf("all connections failed: %s",
				finalErrors[0])
		}
	}

	// Final cleanup
	runtime.GC()

	log.Info().Msgf("Completed price fetching in %s. Successfully fetched prices from %d/%d connections with max %d concurrent workers",
		time.Since(startTime),
		successCount,
		connConfigCount,
		maxConcurrent)

	return connConfigCount, priceCount, nil
}

// FetchPriceForConnConfig lookups all Price for region of conn config, processes them in batch
func FetchPriceForConnConfig(config model.ConnConfig) error {
	log.Debug().Msg("FetchPriceForConnConfig(" + config.ConfigName + ")")

	// Reuse existing LookupPriceList function
	priceInConnection, err := LookupPriceList(config)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot LookupPriceList in %s", config.ConfigName)
		return err
	}

	if len(priceInConnection.PriceList) == 0 {
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
				price.PriceInfo.OnDemand.Price, price.ProductInfo.VMSpecInfo.Name)

			// Early memory cleanup for failed items
			priceInConnection.PriceList[i].ProductInfo.VMSpecInfo = model.SpiderSpecInfoForNameOnly{}
			continue
		}

		// Apply currency conversion
		priceFloat = float64(common.ConvertToBaseCurrency(float32(priceFloat), price.PriceInfo.OnDemand.Currency))

		// Create spec key
		specKey := GetProviderRegionZoneResourceKey(
			config.ProviderName,
			config.RegionDetail.RegionName,
			"",
			price.ProductInfo.VMSpecInfo.Name)

		// Add to batch instead of individual update
		batchUpdates[specKey] = float32(priceFloat)
		processedCount++

		// Immediate memory cleanup after processing each item
		priceInConnection.PriceList[i].ProductInfo.VMSpecInfo = model.SpiderSpecInfoForNameOnly{}
	}

	// Release the original data slice immediately
	priceInConnection.PriceList = nil
	priceInConnection = model.SpiderCloudPrice{}

	// Perform batch update if we have data to update
	if len(batchUpdates) > 0 {
		updateCount, err := BulkUpdateSpec(model.SystemCommonNs, batchUpdates)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to batch update specs for %s", config.ConfigName)
			batchUpdates = nil
			return err
		}
		log.Debug().Msgf("Successfully updated %d specs for %s", updateCount, config.ConfigName)
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
	client := resty.New()
	client.SetTimeout(10 * time.Minute)
	url := model.SpiderRestUrl + "/priceinfo/vm/" + connConfig.RegionZoneInfo.AssignedRegion
	method := "POST"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig.ConfigName

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
		content := model.SpiderCloudPrice{}
		return content, err
	}

	temp := callResult
	return temp, nil
}

// RegisterSpecWithCspResourceId accepts spec creation request, creates and returns an TB spec object
func RegisterSpecWithCspResourceId(nsId string, u *model.TbSpecReq, update bool) (model.TbSpecInfo, error) {

	content := model.TbSpecInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	connConfig, err := common.GetConnConfig(u.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot GetConnConfig in %s", u.ConnectionName)
		return content, err
	}

	res, err := LookupSpec(u.ConnectionName, u.CspSpecName)
	if err != nil {
		log.Error().Err(err).Msgf("cannot LookupSpec ConnectionName(%s), CspResourceId(%s)", u.ConnectionName, u.CspSpecName)
		return content, err
	}

	content, err = ConvertSpiderSpecToTumblebugSpec(connConfig.ProviderName, res)
	if err != nil {
		log.Error().Err(err).Msg("cannot RegisterSpecWithCspResourceId")
		return content, err
	}

	content.Namespace = nsId
	content.Id = u.Name
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.AssociatedObjectList = []string{}

	// "INSERT INTO `spec`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	result := model.ORM.Create(&content)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Cannot insert data to RDB")
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return content, nil
}

// RegisterSpecWithInfo accepts spec creation request, creates and returns an TB spec object
func RegisterSpecWithInfo(nsId string, content *model.TbSpecInfo, update bool) (model.TbSpecInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.TbSpecInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	content.Namespace = nsId
	content.Id = content.Name
	content.AssociatedObjectList = []string{}

	// "INSERT INTO `spec`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	// Attempt to insert the new record
	result := model.ORM.Create(content)
	if result.Error != nil {
		if update {
			updateResult := model.ORM.Model(&model.TbSpecInfo{}).
				Where("namespace = ? AND id = ?", content.Namespace, content.Id).
				Updates(content)

			if updateResult.Error != nil {
				log.Error().Err(updateResult.Error).Msg("Error updating spec after insert failure")
				return *content, updateResult.Error
			} else {
				log.Trace().Msg("SQL: Update success after insert failure")
			}
		} else {
			log.Error().Err(result.Error).Msg("Error inserting spec and update flag is false")
			return *content, result.Error
		}
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return *content, nil
}

// RegisterSpecWithInfoInBulk register a list of specs in bulk
func RegisterSpecWithInfoInBulk(specList []model.TbSpecInfo) error {
	// In PostgreSQL, use session_replication_role instead of PRAGMA
	model.ORM.Exec("SET session_replication_role = 'replica'")

	// Batch size - PostgreSQL can handle larger batches
	batchSize := 100

	uniqueSpecs := make(map[string]model.TbSpecInfo)
	for _, spec := range specList {
		key := spec.Namespace + ":" + spec.Id
		uniqueSpecs[key] = spec
	}
	dedupedSpecList := make([]model.TbSpecInfo, 0, len(uniqueSpecs))
	for _, spec := range uniqueSpecs {
		dedupedSpecList = append(dedupedSpecList, spec)
	}

	total := len(dedupedSpecList)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := dedupedSpecList[i:end]

		// Start transaction
		tx := model.ORM.Begin()
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to begin transaction")
			return tx.Error
		}

		// Use PostgreSQL's more concise UPSERT approach: UpdateAll: true
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}, {Name: "id"}},
			UpdateAll: true, // Automatically update all fields (no need to specify individual fields)
		}).CreateInBatches(&batch, len(batch))

		if result.Error != nil {
			tx.Rollback()
			log.Error().Err(result.Error).Msg("Error upserting specs in bulk")
			return result.Error
		}

		if err := tx.Commit().Error; err != nil {
			log.Error().Err(err).Msg("Failed to commit transaction")
			return err
		}

		// log.Info().Msgf("Bulk upsert success: batch %d-%d, affected: %d records",
		// 	i, end-1, result.RowsAffected)
	}

	// Re-enable foreign key constraints
	//model.ORM.Exec("SET session_replication_role = 'origin'")
	return nil
}

// RemoveDuplicateSpecsInSQL is to remove duplicate specs in db to refine batch insert duplicates
func RemoveDuplicateSpecsInSQL() error {
	// PostgreSQL deduplication query
	sqlStr := `
    DELETE FROM tb_spec_infos
    WHERE ctid NOT IN (
        SELECT MIN(ctid)
        FROM tb_spec_infos
        GROUP BY namespace, id
    );
    `

	result := model.ORM.Exec(sqlStr)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Error deleting duplicate specs")
		return result.Error
	}
	log.Info().Msg("Duplicate specs removed successfully")

	return nil
}

// Range struct is for 'FilterSpecsByRange'
type Range struct {
	Min float32 `json:"min"`
	Max float32 `json:"max"`
}

// GetSpec accepts namespace Id and specKey(Id,CspResourceName,...), and returns the TB spec object
func GetSpec(nsId string, specKey string) (model.TbSpecInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.TbSpecInfo{}, err
	}

	log.Debug().Msg("[Get spec] " + specKey)

	// make comparison case-insensitive
	nsId = strings.ToLower(nsId)
	specKey = strings.ToLower(specKey)

	// ex: tencent+ap-jakarta+ubuntu22.04
	var spec model.TbSpecInfo
	result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ?", nsId, specKey).First(&spec)
	if result.Error == nil {
		return spec, nil
	}

	// ex: spec-487zeit5
	result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(csp_spec_name) = ?", nsId, specKey).First(&spec)
	if result.Error == nil {
		return spec, nil
	}

	return model.TbSpecInfo{}, fmt.Errorf("The specKey %s not found by any of ID, CspSpecName", specKey)
}

// Retrieve field-to-column mapping information for the model
func getColumnMapping(modelType interface{}) map[string]string {
	stmt := &gorm.Statement{DB: model.ORM}
	stmt.Parse(modelType)

	mapping := make(map[string]string)
	for _, field := range stmt.Schema.Fields {
		mapping[field.Name] = field.DBName
	}

	return mapping
}

// FilterSpecsByRange accepts criteria ranges for filtering, and returns the list of filtered TB spec objects
func FilterSpecsByRange(nsId string, filter model.FilterSpecsByRangeRequest) ([]model.TbSpecInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return nil, err
	}

	// Start building the query using field names as database column names
	query := model.ORM.Where("namespace = ?", nsId)

	specColumnMapping := getColumnMapping(&model.TbSpecInfo{})
	// Change field names to start with lowercase (GORM convention)
	val := reflect.ValueOf(filter)
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		modelFieldName := field.Name

		dbFieldName, exists := specColumnMapping[modelFieldName]
		if !exists {
			log.Warn().Msgf("Field %s not found in the model", modelFieldName)
			return nil, fmt.Errorf("Field %s not found in the model", modelFieldName)
		}

		if value.Kind() == reflect.Struct {
			min := value.FieldByName("Min")
			max := value.FieldByName("Max")

			if min.IsValid() && !min.IsZero() {
				query = query.Where(dbFieldName+" >= ?", min.Interface())
			}
			if max.IsValid() && !max.IsZero() {
				query = query.Where(dbFieldName+" <= ?", max.Interface())
			}
		} else if value.IsValid() && !value.IsZero() {
			switch value.Kind() {
			case reflect.String:
				cleanValue := strings.ToLower(value.String())
				query = query.Where("LOWER("+dbFieldName+") LIKE ?", "%"+cleanValue+"%")
				log.Info().Msgf("Filtering by %s: %s", dbFieldName, cleanValue)
			}
		}
	}

	startTime := time.Now()

	var specs []model.TbSpecInfo

	// Check the query before executing
	query = query.Debug()
	result := query.Find(&specs)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to execute query")
		return nil, result.Error
	}

	elapsedTime := time.Since(startTime)
	log.Info().
		Dur("elapsedTime", elapsedTime).
		Msg("ORM:session.Find(&specs)")

	return specs, nil
}

// UpdateSpec accepts to-be TB spec objects,
// updates and returns the updated TB spec objects
func UpdateSpec(nsId string, specId string, fieldsToUpdate model.TbSpecInfo) (model.TbSpecInfo, error) {

	result := model.ORM.Model(&model.TbSpecInfo{}).
		Where("namespace = ? AND id = ?", nsId, specId).
		Updates(fieldsToUpdate)

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("")
		return fieldsToUpdate, result.Error
	} else {
		log.Trace().Msg("SQL: Update success")
	}

	return fieldsToUpdate, nil
}

// BulkUpdateSpec updates multiple specs with proper type casting
func BulkUpdateSpec(nsId string, updates map[string]float32) (int, error) {
	if len(updates) == 0 {
		return 0, nil
	}

	// Extract spec IDs for WHERE IN clause
	specIds := make([]string, 0, len(updates))
	for specId := range updates {
		specIds = append(specIds, specId)
	}

	// Build CASE statement with explicit CAST
	caseClause := "CASE id "
	args := make([]interface{}, 0, len(updates)*2)

	for specId, price := range updates {
		caseClause += "WHEN ? THEN CAST(? AS NUMERIC) "
		args = append(args, specId, price)
	}
	caseClause += "END"

	// Execute with proper casting
	result := model.ORM.Model(&model.TbSpecInfo{}).
		Where("namespace = ? AND id IN ?", nsId, specIds).
		Update("cost_per_hour", gorm.Expr(caseClause, args...))

	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}
