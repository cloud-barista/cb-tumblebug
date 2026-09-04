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
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// executeWithTimeoutRetry executes a function and retries once after 1 minute if a timeout error occurs
func executeWithTimeoutRetry(fn func() error, operationName string, resourceName string) error {
	err := fn()
	if err == nil {
		return nil
	}

	// Check if it's a timeout error - retry once after waiting
	errStr := err.Error()
	if strings.Contains(errStr, "Client.Timeout") || strings.Contains(errStr, "context deadline exceeded") {
		log.Warn().Msgf("%s timeout for '%s', waiting 1 minute before retry...", operationName, resourceName)
		time.Sleep(1 * time.Minute)

		// Retry once
		retryErr := fn()
		if retryErr != nil {
			log.Error().Err(retryErr).Msgf("%s retry also failed for '%s'", operationName, resourceName)
			return retryErr
		}
		log.Info().Msgf("%s retry succeeded for '%s'", operationName, resourceName)
		return nil
	}

	return err
}

// SpecReqStructLevelValidation is a function to validate 'SpecReq' object.
func SpecReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.SpecReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// ConvertSpiderSpecToTumblebugSpec accepts an Spider spec object, converts to and returns an TB spec object
func ConvertSpiderSpecToTumblebugSpec(connConfig model.ConnConfig, spiderSpec model.SpiderSpecInfo) (model.SpecInfo, error) {
	if spiderSpec.Name == "" {
		err := fmt.Errorf("failed convertSpiderSpecToTumblebugSpec. spiderSpec.Name is empty")
		emptyTumblebugSpec := model.SpecInfo{}
		return emptyTumblebugSpec, err
	}

	providerName := connConfig.ProviderName

	tumblebugSpec := model.SpecInfo{}

	tumblebugSpec.Name = spiderSpec.Name
	tumblebugSpec.CspSpecName = spiderSpec.Name
	tumblebugSpec.Uid = common.GenUid()
	tumblebugSpec.RegionName = spiderSpec.Region
	tumblebugSpec.RegionLatitude = connConfig.RegionDetail.Location.Latitude
	tumblebugSpec.RegionLongitude = connConfig.RegionDetail.Location.Longitude
	// log.Debug().Msgf("Region coordinates for spec %s: (%f, %f)", tumblebugSpec.CspSpecName, tumblebugSpec.RegionLatitude, tumblebugSpec.RegionLongitude)
	tumblebugSpec.ProviderName = providerName

	// For Azure, filter out Gen1-only VM families
	if csp.ResolveCloudPlatform(providerName) == csp.Azure {
		// TODO: needs to be merged with a general ignore filtering method
		if isAzureGen1OnlySpec(tumblebugSpec.CspSpecName, spiderSpec.KeyValueList) {
			err := fmt.Errorf("skipping Azure Gen1-only VM family spec: %s", tumblebugSpec.CspSpecName)
			emptyTumblebugSpec := model.SpecInfo{}
			return emptyTumblebugSpec, err
		}
	}

	tempUint64, _ := strconv.ParseUint(spiderSpec.VCpu.Count, 10, 16)
	tumblebugSpec.VCPU = uint16(tempUint64)
	tempFloat64, _ := strconv.ParseFloat(spiderSpec.MemSizeMiB, 32)
	tumblebugSpec.MemoryGiB = float32(tempFloat64 / 1024)
	tempFloat64, _ = strconv.ParseFloat(spiderSpec.DiskSizeGB, 32)
	tumblebugSpec.DiskSizeGB = float32(tempFloat64)
	if rootDiskSizeInt, err := strconv.Atoi(spiderSpec.DiskSizeGB); err == nil {
		tumblebugSpec.RootDiskSize = rootDiskSizeInt
	}

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

		// Correct manufacturer when Spider reports incorrect info (e.g., IBM labels Gaudi/MI300X as NVIDIA)
		tumblebugSpec.AcceleratorModel = normalizeAcceleratorModel(tumblebugSpec.AcceleratorModel)

		// GCP fallback: Spider cannot determine memory for GPU types whose accelerator type name
		// does not contain a "gb" suffix (e.g., "nvidia-l4", "nvidia-tesla-a100"). Extract the raw
		// guestAcceleratorType from Details and look it up in a static table.
		if tumblebugSpec.AcceleratorMemoryGB == 0 && csp.ResolveCloudPlatform(tumblebugSpec.ProviderName) == csp.GCP {
			if accelType := extractGCPAcceleratorType(tumblebugSpec.Details); accelType != "" {
				if memGB, ok := gcpAcceleratorMemoryGBByType[accelType]; ok {
					tumblebugSpec.AcceleratorMemoryGB = memGB
				}
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

// gcpAcceleratorMemoryGBByType maps GCP guestAcceleratorType values (as returned by the GCP
// Compute API) to the GPU memory size in GB per GPU unit. This is needed because the GCP API
// only returns the type name string (e.g., "nvidia-l4"), not the memory size, and CB-Spider
// cannot infer memory for types whose name lacks a "gb" suffix.
var gcpAcceleratorMemoryGBByType = map[string]float32{
	"nvidia-l4":           24,  // NVIDIA L4 - 24 GB GDDR6
	"nvidia-l40s":         48,  // NVIDIA L40S - 48 GB GDDR6
	"nvidia-tesla-t4":     16,  // NVIDIA T4 - 16 GB GDDR6
	"nvidia-tesla-k80":    12,  // NVIDIA K80 - 12 GB GDDR5 per GPU unit
	"nvidia-tesla-p4":     8,   // NVIDIA P4 - 8 GB GDDR5
	"nvidia-tesla-p100":   16,  // NVIDIA P100 - 16 GB HBM2
	"nvidia-tesla-v100":   16,  // NVIDIA V100 - 16 GB HBM2
	"nvidia-tesla-a100":   40,  // NVIDIA A100 SXM4 40 GB HBM2 (a2-highgpu / a2-megagpu families)
	"nvidia-b200":         192, // NVIDIA B200 - 192 GB HBM3e
	"nvidia-rtx-pro-6000": 96,  // NVIDIA RTX PRO 6000 Blackwell - 96 GB GDDR7
}

// extractGCPAcceleratorType parses the "Accelerators" key from GCP spec Details and returns
// the raw guestAcceleratorType string (e.g., "nvidia-l4"). GCP stores this as a comma-separated
// key:value pair inside curly braces, e.g. "{guestAcceleratorCount:1,guestAcceleratorType:nvidia-l4}".
func extractGCPAcceleratorType(details []model.KeyValue) string {
	for _, kv := range details {
		if kv.Key != "Accelerators" {
			continue
		}
		// Strip surrounding braces and split on commas
		val := strings.Trim(kv.Value, "{}")
		for _, part := range strings.Split(val, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "guestAcceleratorType:") {
				return strings.ToLower(strings.TrimPrefix(part, "guestAcceleratorType:"))
			}
		}
	}
	return ""
}

// normalizeAcceleratorModel corrects the manufacturer prefix based on known GPU model names.
// Spider drivers may report incorrect manufacturer info (e.g., IBM labels Intel Gaudi and AMD MI300X as NVIDIA).
// Models not listed here are left unchanged and trusted as-is.
func normalizeAcceleratorModel(model string) string {
	upper := strings.ToUpper(model)

	// Intel GPU models (order matters: longer/more specific first)
	for _, m := range []string{"GAUDI3", "GAUDI2", "GAUDI"} {
		if strings.Contains(upper, m) {
			return "Intel " + m
		}
	}

	// AMD GPU models
	for _, m := range []string{
		"RADEON PRO V710", "RADEON PRO V620",
		"MI300X", "MI300A", "MI300", "MI250X", "MI250", "MI210", "MI100", "INSTINCT",
	} {
		if strings.Contains(upper, m) {
			return "AMD " + m
		}
	}

	return model
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

	switch csp.ResolveCloudPlatform(providerName) {
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
		// ref: https://learn.microsoft.com/ko-kr/azure/virtual-machines/sizes/overview?tabs=breakdownseries%2Cgeneralsizelist%2Ccomputesizelist%2Cmemorysizelist%2Cstoragesizelist%2Cgpusizelist%2Cfpgasizelist%2Chpcsizelist#compute-optimized
		// According to Azure naming convention: lowercase 'p' indicates ARM CPU (Microsoft Cobalt or Ampere Altra)
		// Examples: Standard_B2pls_v2 (Ampere Altra), Standard_Dpsv6 (Microsoft Cobalt)
		//
		// Key insight: Azure uses ONLY lowercase 'p' for ARM architecture
		// - lowercase 'p' = ARM64 (e.g., B2ps, D2ps, E2ps)
		// - uppercase 'P' = x86-64 with different meaning (e.g., Promo, NP-series, PB-series)

		// Simple and future-proof: check for lowercase 'p' in spec name
		if strings.Contains(cspSpecName, "p") {
			return string(model.ARM64)
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

	case csp.KT:
		return string(model.X86_64)

	case csp.NCP:
		return string(model.X86_64)

	case csp.NHN:
		return string(model.X86_64)

	default:
		// For unknown CSPs
		return string(model.X86_64)
	}
	return string(model.ArchitectureUnknown)
}

// RegisterSpecWithCspResourceId accepts spec creation request, creates and returns an TB spec object
func RegisterSpecWithCspResourceId(nsId string, u *model.SpecReq, update bool) (model.SpecInfo, error) {

	content := model.SpecInfo{}

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

	content, err = ConvertSpiderSpecToTumblebugSpec(connConfig, res)
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
func RegisterSpecWithInfo(nsId string, content *model.SpecInfo, update bool) (model.SpecInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.SpecInfo{}
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
			updateResult := model.ORM.Model(&model.SpecInfo{}).
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
func RegisterSpecWithInfoInBulk(specList []model.SpecInfo) error {
	// In PostgreSQL, use session_replication_role instead of PRAGMA
	model.ORM.Exec("SET session_replication_role = 'replica'")

	// Batch size - PostgreSQL can handle larger batches
	batchSize := 100

	uniqueSpecs := make(map[string]model.SpecInfo)
	for _, spec := range specList {
		key := spec.Namespace + ":" + spec.Id
		uniqueSpecs[key] = spec
	}
	dedupedSpecList := make([]model.SpecInfo, 0, len(uniqueSpecs))
	for _, spec := range uniqueSpecs {
		dedupedSpecList = append(dedupedSpecList, spec)
	}

	total := len(dedupedSpecList)
	for i := 0; i < total; i += batchSize {
		end := min(i+batchSize, total)
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
    DELETE FROM spec_infos
    WHERE ctid NOT IN (
        SELECT MIN(ctid)
        FROM spec_infos
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

// specLookupGroup collapses concurrent identical spec lookups into a single DB read. Large infra
// provisioning fans out thousands of nodes that share only a handful of specs; without this, each
// node issued its own query and the burst exhausted the DB connection pool.
var specLookupGroup singleflight.Group

// GetSpec accepts namespace Id and specKey(Id,CspResourceName,...), and returns the TB spec object
func GetSpec(nsId string, specKey string) (model.SpecInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.SpecInfo{}, err
	}

	log.Debug().Msg("[Get spec] " + specKey)

	// make comparison case-insensitive
	nsId = strings.ToLower(nsId)
	specKey = strings.ToLower(specKey)

	// Deduplicate concurrent identical lookups so a provisioning burst hits the DB once per spec.
	v, err, _ := specLookupGroup.Do(nsId+"\x00"+specKey, func() (interface{}, error) {
		// ex: tencent+ap-jakarta+ubuntu22.04
		var spec model.SpecInfo
		result := model.ORM.Where("LOWER(namespace) = ? AND LOWER(id) = ?", nsId, specKey).First(&spec)
		if result.Error == nil {
			return spec, nil
		}
		// Only a real "record not found" means the spec is absent; any other error (e.g. a
		// connection-pool timeout under load) must be surfaced as-is so callers can retry
		// instead of mistaking a transient DB failure for a missing spec.
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.SpecInfo{}, fmt.Errorf("spec lookup by id failed for %s: %w", specKey, result.Error)
		}

		// ex: spec-487zeit5
		result = model.ORM.Where("LOWER(namespace) = ? AND LOWER(csp_spec_name) = ?", nsId, specKey).First(&spec)
		if result.Error == nil {
			return spec, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.SpecInfo{}, fmt.Errorf("spec lookup by cspSpecName failed for %s: %w", specKey, result.Error)
		}

		return model.SpecInfo{}, fmt.Errorf("The specKey %s not found by any of ID, CspSpecName", specKey)
	})
	if err != nil {
		return model.SpecInfo{}, err
	}
	return v.(model.SpecInfo), nil
}

// Retrieve field-to-column mapping information for the model
func getColumnMapping(modelType any) map[string]string {
	stmt := &gorm.Statement{DB: model.ORM}
	stmt.Parse(modelType)

	mapping := make(map[string]string)
	for _, field := range stmt.Schema.Fields {
		mapping[field.Name] = field.DBName
	}

	return mapping
}

// FilterSpecsByRange accepts criteria ranges for filtering, and returns the list of filtered TB spec objects
func FilterSpecsByRange(nsId string, filter model.FilterSpecsByRangeRequest, orderBy string) ([]model.SpecInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return nil, err
	}

	// Start building the query using field names as database column names
	query := model.ORM.Where("namespace = ?", nsId)

	specColumnMapping := getColumnMapping(&model.SpecInfo{})
	// Change field names to start with lowercase (GORM convention)
	val := reflect.ValueOf(filter)
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		modelFieldName := field.Name

		// Skip Limit field as it's not a database column
		if modelFieldName == "Limit" {
			continue
		}

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

				// Define fields that require LIKE search for partial matching only
				// Use LIKE search sparingly due to performance impact on indexing
				likeSearchFields := []string{
					"AcceleratorModel", // e.g., "NVIDIA H100" -> search with "NVIDIA"
					"Description",      // Description text partial search
					// Note: AcceleratorType removed - uses exact matching for better performance
					// since values are typically standardized (GPU, TPU, etc.)
				}

				// Fields matched by AND-ed substrings: every whitespace/comma separated token
				// must appear somewhere in the value. A spec id is a compound of provider,
				// region and CSP name (e.g. "aws+eu-south-1+m5.metal"), so callers reach for
				// fragments of it rather than the whole string - "metal 5" should find
				// m5.metal without the caller reconstructing the full id.
				//
				// This does not cost an index: the surrounding LOWER() already prevents the
				// btree on (id, namespace) from being used, so exact match was scanning the
				// table too. Measured on 73k specs: LOWER(id) = ? took 211 ms, two AND-ed
				// LIKEs took 31 ms (fewer rows survive to be materialised).
				tokenSubstringFields := []string{
					"Id",          // e.g. "metal 5" -> aws+eu-south-1+m5.metal
					"CspSpecName", // e.g. "metal" -> m5.metal, m6i.metal, ...
				}

				// Check if current field requires LIKE search
				useLikeSearch := slices.Contains(likeSearchFields, modelFieldName)

				if slices.Contains(tokenSubstringFields, modelFieldName) {
					// Every token must match; an empty token list means no filter at all.
					tokens := strings.FieldsFunc(cleanValue, func(c rune) bool {
						return c == ',' || c == ' '
					})
					matched := 0
					for _, token := range tokens {
						token = strings.TrimSpace(token)
						if token == "" {
							continue
						}
						query = query.Where("LOWER("+dbFieldName+") LIKE ?", "%"+token+"%")
						matched++
					}
					if matched > 0 {
						log.Info().Msgf("Filtering by %s (SUBSTRING AND): %v", dbFieldName, tokens)
					}
				} else if useLikeSearch {
					// For LIKE search, use the original single value (don't support multiple values for LIKE)
					query = query.Where("LOWER("+dbFieldName+") LIKE ?", "%"+cleanValue+"%")
					log.Info().Msgf("Filtering by %s (LIKE): %s", dbFieldName, cleanValue)
				} else {
					// Check if the value contains multiple items separated by comma or space
					var values []string
					for _, item := range strings.FieldsFunc(cleanValue, func(c rune) bool {
						return c == ',' || c == ' '
					}) {
						if trimmed := strings.TrimSpace(item); trimmed != "" {
							values = append(values, trimmed)
						}
					}

					// For exact match fields, support multiple values
					if len(values) == 1 {
						// Single value - use exact match
						query = query.Where("LOWER("+dbFieldName+") = ?", values[0])
						log.Info().Msgf("Filtering by %s (SINGLE): %s", dbFieldName, values[0])
					} else if len(values) > 1 {
						// Multiple values - use IN clause
						query = query.Where("LOWER("+dbFieldName+") IN ?", values)
						log.Info().Msgf("Filtering by %s (MULTIPLE): %v", dbFieldName, values)
					}
					// Note: if len(values) == 0 (empty after parsing), no filter is applied for this field
					// This allows filtering to be skipped when only whitespace/separators are provided
				}
			}
		}
	}

	startTime := time.Now()

	var specs []model.SpecInfo

	// Apply ORDER BY if specified
	if orderBy != "" {
		query = query.Order(orderBy)
		// log.Debug().Msgf("Applying ORDER BY: %s", orderBy)
	}

	// Apply limit if specified and greater than 0
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
		// log.Debug().Msgf("Applying LIMIT: %d", filter.Limit)
	}

	// Check the query before executing (only in debug mode to avoid performance impact)
	// query = query.Debug()
	result := query.Find(&specs)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to execute query")
		return nil, result.Error
	}

	elapsedTime := time.Since(startTime)
	log.Info().
		Int("resultCount", len(specs)).
		Int("limitApplied", filter.Limit).
		Dur("elapsedTime", elapsedTime).
		Msg("ORM:session.Find(&specs)")

	return specs, nil
}

// UpdateSpec accepts to-be TB spec objects,
// updates and returns the updated TB spec objects
func UpdateSpec(nsId string, specId string, fieldsToUpdate model.SpecInfo) (model.SpecInfo, error) {

	result := model.ORM.Model(&model.SpecInfo{}).
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
	var caseClause strings.Builder
	caseClause.WriteString("CASE id ")
	args := make([]any, 0, len(updates)*2)

	for specId, price := range updates {
		caseClause.WriteString("WHEN ? THEN CAST(? AS NUMERIC) ")
		args = append(args, specId, price)
	}
	caseClause.WriteString("END")

	// Execute with proper casting
	result := model.ORM.Model(&model.SpecInfo{}).
		Where("namespace = ? AND id IN ?", nsId, specIds).
		Update("cost_per_hour", gorm.Expr(caseClause.String(), args...))

	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}

// isAzureGen1OnlySpec checks if the given Azure VM spec supports only Hyper-V Gen1.
// It first consults the HyperVGenerations field from the Azure API (authoritative),
// then falls back to name-based heuristics when that field is absent.
func isAzureGen1OnlySpec(specName string, keyValueList []model.KeyValue) bool {
	if specName == "" {
		return false
	}

	// Prefer the authoritative HyperVGenerations field returned by the Azure API.
	for _, kv := range keyValueList {
		if kv.Key == "HyperVGenerations" {
			v := strings.TrimSpace(kv.Value)
			supportsV1 := strings.Contains(v, "V1")
			supportsV2 := strings.Contains(v, "V2")
			return supportsV1 && !supportsV2
		}
	}

	// Fallback: name-based heuristics for specs missing the HyperVGenerations field.
	lowerSpecName := strings.ToLower(specName)

	var family string
	if strings.HasPrefix(lowerSpecName, "standard_") {
		remaining := lowerSpecName[9:]
		if len(remaining) > 0 {
			family = string(remaining[0])
		}
	} else if strings.HasPrefix(lowerSpecName, "basic_") {
		remaining := lowerSpecName[6:]
		if len(remaining) > 0 {
			family = string(remaining[0])
		}
	} else {
		if len(lowerSpecName) > 0 {
			family = string(lowerSpecName[0])
		}
	}

	switch family {
	case "a":
		// All A-series VMs are Gen1-only, including Av2 (Standard_A1_v2 etc.).
		// The "_v2" suffix denotes a size revision, NOT Hyper-V Generation 2.
		return true
	case "d":
		// D-family with digit+s pattern (e.g., Standard_D2s_v3) supports Gen2.
		for i := 0; i < len(lowerSpecName)-1; i++ {
			if lowerSpecName[i] >= '0' && lowerSpecName[i] <= '9' && lowerSpecName[i+1] == 's' {
				return false
			}
		}
		return true
	}

	return false
}

// logSpecsToIgnoreInfo logs the specs to ignore information in structured format
// This is more suitable for containerized environments than creating files
func logSpecsToIgnoreInfo(provider string, specsToIgnore map[string]map[string][]string, globalIgnoreSpecs map[string][]string) {
	// Prepare structured data for logging
	data := model.SpecsToIgnoreData{
		LastUpdated:          time.Now(),
		Description:          "Specs that should be ignored during availability checks. Global specs are unavailable in all regions, region-specific specs are unavailable only in specific regions.",
		GlobalIgnoreSpecs:    make(map[string][]string),
		RegionSpecificIgnore: make(map[string]map[string][]string),
	}

	// Prepare global ignore specs for logging
	for cspProvider, specs := range globalIgnoreSpecs {
		if len(specs) > 0 {
			sort.Strings(specs)
			data.GlobalIgnoreSpecs[cspProvider] = specs
		}
	}

	// Prepare region-specific ignore specs for logging
	for cspProvider, regions := range specsToIgnore {
		if data.RegionSpecificIgnore[cspProvider] == nil {
			data.RegionSpecificIgnore[cspProvider] = make(map[string][]string)
		}

		for region, specs := range regions {
			if len(specs) > 0 {
				sort.Strings(specs)
				data.RegionSpecificIgnore[cspProvider][region] = specs
			}
		}
	}

	// Log the specs to ignore information in structured format
	globalCount := 0
	regionalCount := 0

	// Count global specs
	if globalSpecs, exists := data.GlobalIgnoreSpecs[provider]; exists {
		globalCount = len(globalSpecs)
	}

	// Count regional specs
	if regions, exists := data.RegionSpecificIgnore[provider]; exists {
		for _, specs := range regions {
			regionalCount += len(specs)
		}
	}

	// Create detailed log entry with all specs information
	logEvent := log.Info().
		Str("provider", provider).
		Int("globalIgnoreCount", globalCount).
		Int("regionSpecificCount", regionalCount).
		Time("timestamp", data.LastUpdated)

	// Add global specs to log if any
	if globalSpecs, exists := data.GlobalIgnoreSpecs[provider]; exists && len(globalSpecs) > 0 {
		logEvent = logEvent.Strs("globalIgnoreSpecs", globalSpecs)
	}

	// Add regional specs to log if any
	if regions, exists := data.RegionSpecificIgnore[provider]; exists && len(regions) > 0 {
		for region, specs := range regions {
			if len(specs) > 0 {
				logEvent = logEvent.Strs(fmt.Sprintf("regionIgnoreSpecs_%s", region), specs)
			}
		}
	}

	// Log the complete information
	logEvent.Msg("Specs to ignore information logged for container environment")

	// Also log a summary for easy monitoring
	log.Debug().
		Str("provider", provider).
		Interface("specsToIgnoreData", data).
		Msg("Complete specs to ignore data structure")
}

// Global variable to cache the ignore config
var (
	ignoreConfig     *model.CloudSpecIgnoreConfig
	ignoreConfigOnce sync.Once
	ignoreConfigErr  error
)

// loadCloudSpecIgnoreConfig loads the cloudspec_ignore.yaml file using Viper
func loadCloudSpecIgnoreConfig() (*model.CloudSpecIgnoreConfig, error) {
	ignoreConfigOnce.Do(func() {
		// Create a new Viper instance for the ignore config
		ignoreViper := viper.New()

		// Add possible config paths using centralized helper
		common.SetupViperPaths(ignoreViper)
		ignoreViper.SetConfigName("cloudspec_ignore")
		ignoreViper.SetConfigType("yaml")

		// Try to read the config file
		err := ignoreViper.ReadInConfig()
		if err != nil {
			log.Warn().Err(err).Msg("Could not load cloudspec_ignore.yaml, no spec filtering will be applied")
			ignoreConfigErr = err
			return
		}

		log.Debug().Str("path", ignoreViper.ConfigFileUsed()).Msg("Found cloudspec_ignore.yaml")

		// Manual extraction to handle Viper's type conversion issues
		var config model.CloudSpecIgnoreConfig

		// Extract global patterns
		if globalPatternsRaw := ignoreViper.Get("global.patterns"); globalPatternsRaw != nil {
			if patterns, ok := globalPatternsRaw.([]any); ok {
				for _, pattern := range patterns {
					if str, ok := pattern.(string); ok {
						config.Global.Patterns = append(config.Global.Patterns, str)
					}
				}
			}
		}

		// Extract CSP-specific patterns
		config.CSPs = make(map[string]model.CSPIgnorePatterns)
		if cspsRaw := ignoreViper.Get("csps"); cspsRaw != nil {
			if cspsMap, ok := cspsRaw.(map[string]any); ok {
				for cspName, cspDataRaw := range cspsMap {
					if cspData, ok := cspDataRaw.(map[string]any); ok {
						var cspConfig model.CSPIgnorePatterns

						// Extract description
						if desc, exists := cspData["description"]; exists {
							if descStr, ok := desc.(string); ok {
								cspConfig.Description = descStr
							}
						}

						// Extract global_patterns with proper type handling
						if globalPatternsRaw, exists := cspData["global_patterns"]; exists && globalPatternsRaw != nil {
							if patterns, ok := globalPatternsRaw.([]any); ok {
								for _, pattern := range patterns {
									if str, ok := pattern.(string); ok {
										cspConfig.GlobalPatterns = append(cspConfig.GlobalPatterns, str)
									}
								}
							}
						}

						// Extract regions
						if regionsRaw, exists := cspData["regions"]; exists && regionsRaw != nil {
							if regionsMap, ok := regionsRaw.(map[string]any); ok {
								cspConfig.Regions = make(map[string]model.RegionIgnorePatterns)
								for regionName, regionDataRaw := range regionsMap {
									var regionConfig model.RegionIgnorePatterns

									// New format: direct array under region name
									if regionPatterns, ok := regionDataRaw.([]any); ok {
										for _, pattern := range regionPatterns {
											if str, ok := pattern.(string); ok {
												regionConfig.Patterns = append(regionConfig.Patterns, str)
											}
										}
									}

									cspConfig.Regions[regionName] = regionConfig
								}
							}
						}

						config.CSPs[cspName] = cspConfig
					}
				}
			}
		}

		ignoreConfig = &config
		log.Info().Msg("Successfully loaded cloudspec_ignore.yaml")

		// Debug: Print loaded config structure
		log.Debug().
			Int("globalPatterns", len(config.Global.Patterns)).
			Interface("globalPatterns", config.Global.Patterns).
			Msg("Loaded global patterns")

	})

	return ignoreConfig, ignoreConfigErr
}

// shouldIgnoreSpec checks if a spec should be ignored based on the ignore configuration
func shouldIgnoreSpec(specName, providerName, regionName string) bool {
	config, err := loadCloudSpecIgnoreConfig()
	if err != nil || config == nil {
		// If config can't be loaded, don't ignore any specs
		return false
	}

	// Check global patterns first
	for _, pattern := range config.Global.Patterns {
		if matchesPattern(specName, pattern) {
			return true
		}
	}

	// Get CSP-specific patterns from the CSPs map
	// Use ResolveCloudPlatform to handle derived CSPs (e.g., openstack-new01 → openstack)
	cspPatterns, exists := config.CSPs[csp.ResolveCloudPlatform(providerName)]
	if !exists {
		return false
	}

	// Check CSP global patterns
	for _, pattern := range cspPatterns.GlobalPatterns {
		if matchesPattern(specName, pattern) {
			return true
		}
	}

	// Check region-specific patterns
	if regionPatterns, regionExists := cspPatterns.Regions[regionName]; regionExists {
		// Check direct patterns array
		for _, pattern := range regionPatterns.Patterns {
			if matchesPattern(specName, pattern) {
				return true
			}
		}
	}

	return false
}

// matchesPattern checks if a spec name matches a given pattern
// Supports basic wildcard matching with * and ?
func matchesPattern(specName, pattern string) bool {
	// Simple wildcard matching implementation
	// * matches any sequence of characters
	// ? matches any single character

	matched, err := filepath.Match(pattern, specName)
	if err != nil {
		log.Debug().Err(err).
			Str("spec", specName).
			Str("pattern", pattern).
			Msg("Error matching pattern, assuming no match")
		return false
	}

	return matched
}

// Error codes for GetAvailableZonesForSpec
