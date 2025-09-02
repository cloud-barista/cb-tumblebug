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

// Package model is to handle object of CB-Tumblebug
package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// SpiderSpecInfo is a struct to create JSON body of 'Get spec request'
type SpiderSpecInfo struct {
	// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VMSpecHandler.go

	Region     string          `json:"Region" validate:"required" example:"us-east-1"` // Region where the VM spec is available
	Name       string          `json:"Name" validate:"required" example:"t2.micro"`    // Name of the VM spec
	VCpu       SpiderVCpuInfo  `json:"VCpu" validate:"required"`                       // CPU details of the VM spec
	MemSizeMiB string          `json:"MemSizeMib" validate:"required" example:"1024"`  // Memory size in MiB
	DiskSizeGB string          `json:"DiskSizeGB" validate:"required" example:"8"`     // Disk size in GB, "-1" when not applicable
	Gpu        []SpiderGpuInfo `json:"Gpu,omitempty" validate:"omitempty"`             // GPU details if available

	KeyValueList []KeyValue `json:"KeyValueList,omitempty" validate:"omitempty"` // Additional key-value pairs for the VM spec
}

// SpiderVCpuInfo is a struct to handle vCPU Info from CB-Spider.
type SpiderVCpuInfo struct {
	Count    string `json:"Count" validate:"required" example:"2"`                 // Number of VCpu, "-1" when not applicable
	ClockGHz string `json:"ClockGHz,omitempty" validate:"omitempty" example:"2.5"` // Clock speed in GHz, "-1" when not applicable
}

// SpiderGpuInfo is a struct to handle GPU Info from CB-Spider.
type SpiderGpuInfo struct {
	Count          string `json:"Count" validate:"required" example:"2"`                      // Number of GPUs, "-1" when not applicable
	Mfr            string `json:"Mfr,omitempty" validate:"omitempty" example:"NVIDIA"`        // Manufacturer of the GPU, NA when not applicable
	Model          string `json:"Model,omitempty" validate:"omitempty" example:"Tesla K80"`   // Model of the GPU, NA when not applicable
	MemSizeGB      string `json:"MemSizeGB,omitempty" validate:"omitempty" example:"12"`      // Memory size of the GPU in GB, "-1" when not applicable
	TotalMemSizeGB string `json:"TotalMemSizeGB,omitempty" validate:"omitempty" example:"24"` // Total Memory size of the GPU in GB, "-1" when not applicable
}

// SpiderCloudPrice represents the pricing information for a specific cloud provider.
type SpiderCloudPrice struct {
	// Meta       SpiderMeta `json:"Meta" validate:"required" description:"Metadata information about the price data"`
	// CloudName  string     `json:"CloudName" validate:"required" example:"AWS"`        // Name of the cloud provider
	// RegionName string     `json:"RegionName" validate:"required" example:"us-east-1"` // Name of the region

	PriceList []SpiderPrice `json:"PriceList" validate:"required" description:"List of prices"` // List of prices for different services/products
}

// SpiderMeta contains metadata information about the price data.
type SpiderMeta struct {
	Version     string `json:"Version" validate:"required" example:"1.0"`        // Version of the pricing data
	Description string `json:"Description,omitempty" example:"Cloud price data"` // Description of the pricing data
}

// SpiderPrice represents the price information for a specific product.
type SpiderPrice struct {
	// ZoneName    string            `json:"ZoneName,omitempty" example:"us-east-1a"`                                     // Name of the zone
	ProductInfo SpiderProductInfo `json:"ProductInfo" validate:"required" description:"Information about the product"` // Information about the product
	PriceInfo   SpiderPriceInfo   `json:"PriceInfo" validate:"required" description:"Pricing details of the product"`  // Pricing details of the product
}

// ProductInfo represents the product details.
type SpiderProductInfo struct {
	// ProductId  string         `json:"ProductId" validate:"required" example:"prod-123"`                           // ID of the product
	VMSpecInfo SpiderSpecInfoForNameOnly `json:"VMSpecInfo" validate:"required" description:"Information about the VM spec"` // Information about the VM spec
	// Description string         `json:"Description,omitempty" example:"General purpose instance"`                   // Description of the product
	// CSPProductInfo interface{}    `json:"CSPProductInfo" validate:"required" description:"Additional product info"`   // Additional product information specific to CSP
}

// SpiderSpecInfoForNameOnly is a struct to create JSON body of SpiderSpecInfoForNameOnly
type SpiderSpecInfoForNameOnly struct {
	// Region     string          `json:"Region" validate:"required" example:"us-east-1"` // Region where the VM spec is available
	Name string `json:"Name" validate:"required" example:"t2.micro"` // Name of the VM spec
	// VCpu       SpiderVCpuInfo  `json:"VCpu" validate:"required"`                       // CPU details of the VM spec
	// MemSizeMiB string          `json:"MemSizeMib" validate:"required" example:"1024"`  // Memory size in MiB
	// DiskSizeGB string          `json:"DiskSizeGB" validate:"required" example:"8"`     // Disk size in GB, "-1" when not applicable
	// Gpu        []SpiderGpuInfo `json:"Gpu,omitempty" validate:"omitempty"`             // GPU details if available

	// KeyValueList []KeyValue `json:"KeyValueList,omitempty" validate:"omitempty"` // Additional key-value pairs for the VM spec
}

// PriceInfo represents the pricing details for a product.
type SpiderPriceInfo struct {
	OnDemand SpiderOnDemand `json:"OnDemand" validate:"required" description:"Ondemand pricing details"` // Ondemand pricing details
	// CSPPriceInfo interface{}    `json:"CSPPriceInfo" validate:"required" description:"Additional price info"` // Additional price information specific to CSP
}

// OnDemand represents the OnDemand pricing details.
type SpiderOnDemand struct {
	PricingId   string `json:"PricingId" validate:"required" example:"price-123"`    // ID of the pricing policy
	Unit        string `json:"Unit" validate:"required" example:"Hour"`              // Unit of the pricing (e.g., per hour)
	Currency    string `json:"Currency" validate:"required" example:"USD"`           // Currency of the pricing
	Price       string `json:"Price" validate:"required" example:"0.02"`             // Price in the specified currency per unit
	Description string `json:"Description,omitempty" example:"Pricing for t2.micro"` // Description of the pricing policy
}

type SpiderPriceInfoHandler interface {
	ListProductFamily(regionName string) ([]string, error)
	GetPriceInfo(productFamily string, regionName string, filterList []KeyValue) (string, error) // return string: json format
}

// SpecReq is a struct to handle 'Register spec' request toward CB-Tumblebug.
type SpecReq struct {
	// Name is human-readable string to represent the object, used to generate Id
	Name           string `json:"name" validate:"required"`
	ConnectionName string `json:"connectionName" validate:"required"`
	// CspSpecName is name of the spec given by CSP
	CspSpecName string `json:"cspSpecName" validate:"required"`
	Description string `json:"description"`
}

// SpecInfo is a struct that represents TB spec object.
type SpecInfo struct { // Tumblebug
	// Id is unique identifier for the object
	Id string `json:"id" example:"aws+ap-southeast+csp-06eb41e14121c550a" gorm:"primaryKey"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	// CspSpecName is name of the spec given by CSP
	CspSpecName string `json:"cspSpecName,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name            string  `json:"name" example:"aws-ap-southeast-1"`
	Namespace       string  `json:"namespace,omitempty" example:"default" gorm:"primaryKey"`
	ConnectionName  string  `json:"connectionName,omitempty"`
	ProviderName    string  `json:"providerName,omitempty"`
	RegionName      string  `json:"regionName,omitempty"`
	RegionLatitude  float64 `json:"regionLatitude"`
	RegionLongitude float64 `json:"regionLongitude"`
	// InfraType can be one of vm|k8s|kubernetes|container, etc.
	InfraType             string   `json:"infraType,omitempty"`
	Architecture          string   `json:"architecture,omitempty" example:"x86_64"`
	OsType                string   `json:"osType,omitempty"`
	VCPU                  uint16   `json:"vCPU,omitempty"`
	MemoryGiB             float32  `json:"memoryGiB,omitempty"`
	DiskSizeGB            float32  `json:"diskSizeGB,omitempty"`
	MaxTotalStorageTiB    uint16   `json:"maxTotalStorageTiB,omitempty"`
	NetBwGbps             uint16   `json:"netBwGbps,omitempty"`
	AcceleratorModel      string   `json:"acceleratorModel,omitempty"`
	AcceleratorCount      uint8    `json:"acceleratorCount,omitempty"`
	AcceleratorMemoryGB   float32  `json:"acceleratorMemoryGB,omitempty"`
	AcceleratorType       string   `json:"acceleratorType,omitempty"`
	CostPerHour           float32  `json:"costPerHour,omitempty"`
	Description           string   `json:"description,omitempty"`
	OrderInFilteredResult uint16   `json:"orderInFilteredResult,omitempty"`
	EvaluationStatus      string   `json:"evaluationStatus,omitempty"`
	EvaluationScore01     float32  `json:"evaluationScore01"`
	EvaluationScore02     float32  `json:"evaluationScore02"`
	EvaluationScore03     float32  `json:"evaluationScore03"`
	EvaluationScore04     float32  `json:"evaluationScore04"`
	EvaluationScore05     float32  `json:"evaluationScore05"`
	EvaluationScore06     float32  `json:"evaluationScore06"`
	EvaluationScore07     float32  `json:"evaluationScore07"`
	EvaluationScore08     float32  `json:"evaluationScore08"`
	EvaluationScore09     float32  `json:"evaluationScore09"`
	EvaluationScore10     float32  `json:"evaluationScore10"`
	RootDiskType          string   `json:"rootDiskType"`
	RootDiskSize          string   `json:"rootDiskSize"`
	AssociatedObjectList  []string `json:"associatedObjectList,omitempty" gorm:"type:text;serializer:json"`
	IsAutoGenerated       bool     `json:"isAutoGenerated,omitempty"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string     `json:"systemLabel,omitempty" example:"Managed by CB-Tumblebug" default:""`
	Details     []KeyValue `json:"details" gorm:"type:text;serializer:json"`
}

// LatencyInfo is a struct that represents TB latency map object.
type LatencyInfo struct {
	// SourceRegion is the source region for latency measurement
	SourceRegion string `json:"sourceRegion" gorm:"primaryKey" example:"aws+us-east-1"`
	// TargetRegion is the target region for latency measurement
	TargetRegion string `json:"targetRegion" gorm:"primaryKey" example:"aws+us-west-2"`
	// LatencyMs is the latency in milliseconds between source and target regions
	LatencyMs float64 `json:"latencyMs" example:"70.5"`
	// MeasuredAt is the timestamp when the latency was measured
	MeasuredAt time.Time `json:"measuredAt"`
	// UpdatedAt is the timestamp when the record was last updated
	UpdatedAt time.Time `json:"updatedAt"`
}

// FilterSpecsByRangeRequest is for 'FilterSpecsByRange'
type FilterSpecsByRangeRequest struct {
	Id                  string  `json:"id"`
	Name                string  `json:"name"`
	ConnectionName      string  `json:"connectionName"`
	ProviderName        string  `json:"providerName"`
	RegionName          string  `json:"regionName"`
	RegionLatitude      float64 `json:"regionLatitude"`
	RegionLongitude     float64 `json:"regionLongitude"`
	CspSpecName         string  `json:"cspSpecName"`
	InfraType           string  `json:"infraType"`
	Architecture        string  `json:"architecture"`
	OsType              string  `json:"osType"`
	VCPU                Range   `json:"vCPU"`
	MemoryGiB           Range   `json:"memoryGiB"`
	DiskSizeGB          Range   `json:"diskSizeGB"`
	MaxTotalStorageTiB  Range   `json:"maxTotalStorageTiB"`
	NetBwGbps           Range   `json:"netBwGbps"`
	AcceleratorModel    string  `json:"acceleratorModel"`
	AcceleratorCount    Range   `json:"acceleratorCount"`
	AcceleratorMemoryGB Range   `json:"acceleratorMemoryGB"`
	AcceleratorType     string  `json:"acceleratorType"`
	CostPerHour         Range   `json:"costPerHour"`
	Description         string  `json:"description"`
	EvaluationStatus    string  `json:"evaluationStatus"`
	EvaluationScore01   Range   `json:"evaluationScore01"`
	EvaluationScore02   Range   `json:"evaluationScore02"`
	EvaluationScore03   Range   `json:"evaluationScore03"`
	EvaluationScore04   Range   `json:"evaluationScore04"`
	EvaluationScore05   Range   `json:"evaluationScore05"`
	EvaluationScore06   Range   `json:"evaluationScore06"`
	EvaluationScore07   Range   `json:"evaluationScore07"`
	EvaluationScore08   Range   `json:"evaluationScore08"`
	EvaluationScore09   Range   `json:"evaluationScore09"`
	EvaluationScore10   Range   `json:"evaluationScore10"`
	Limit               int     `json:"limit" example:"0" description:"Maximum number of results to return (0 for no limit - returns all results)"`
}

// SpiderSpecList is a struct to handle spec list from the CB-Spider's REST API response
type SpiderSpecList struct {
	Vmspec []SpiderSpecInfo `json:"vmspec"`
}

// Range struct is for 'FilterSpecsByRange'
type Range struct {
	Min float32 `json:"min"`
	Max float32 `json:"max"`
}

// SpecFetchOption is struct for Spec Fetch Options
type SpecFetchOption struct {
	// providers need to be excluded from the spec fetching operation (ex: ["azure"])
	ExcludedProviders []string `json:"excludedProviders,omitempty" example:"azure" description:"Providers to be excluded from the spec fetching operation."`

	// providers that are not region-specific (ex: ["gcp"])
	RegionAgnosticProviders []string `json:"regionAgnosticProviders,omitempty" example:"gcp,tencent" description:"Providers that are not region-specific."`
}

// RecommendSpecRequestOptions is struct for RecommendSpec Request Options
type RecommendSpecRequestOptions struct {
	// Filter options - available filtering fields and their example values
	Filter FilterOptionsInfo `json:"filter" description:"Available filtering options for specs"`

	// Priority options - available prioritization metrics and parameters
	Priority PriorityOptionsInfo `json:"priority" description:"Available prioritization options for specs"`

	// Limit options - example limit values
	Limit []string `json:"limit" example:"5,10,20,50" description:"Example limit values for result count"`
}

// FilterOptionsInfo provides available filter metrics and their example values
type FilterOptionsInfo struct {
	// Available metrics for filtering
	AvailableMetrics []string `json:"availableMetrics" example:"vCPU,memoryGiB,costPerHour,providerName,regionName,architecture" description:"Available metrics for filtering specs"`

	// Example filter policies
	ExamplePolicies []FilterConditionExample `json:"examplePolicies" description:"Example filter policies"`

	// Available values for each metric (for select fields)
	AvailableValues FilterAvailableValues `json:"availableValues" description:"Available values for select-type filter fields"`

	// Example limit values for performance optimization
	LimitExamples []string `json:"limitExamples" example:"0,50,100,200,500" description:"Example limit values for performance optimization"`
}

// FilterConditionExample provides example filter conditions
type FilterConditionExample struct {
	Metric      string             `json:"metric" example:"vCPU"`
	Description string             `json:"description" example:"Filter specs with 2-8 vCPUs"`
	Condition   []OperationExample `json:"condition"`
}

// OperationExample provides example operations
type OperationExample struct {
	Operator string `json:"operator" example:">="`
	Operand  string `json:"operand" example:"2"`
}

// FilterAvailableValues provides available values for filter fields
type FilterAvailableValues struct {
	// Basic identification fields
	Id             []string `json:"id,omitempty" description:"Available spec IDs"`
	Name           []string `json:"name,omitempty" description:"Available spec names"`
	ConnectionName []string `json:"connectionName,omitempty" description:"Available connection names"`

	// Provider and region information
	ProviderName []string `json:"providerName" description:"Available CSP provider names"`
	RegionName   []string `json:"regionName" description:"Available region names"`
	CspSpecName  []string `json:"cspSpecName,omitempty" description:"Available CSP spec names"`

	// Infrastructure specifications
	InfraType    []string `json:"infraType" description:"Available infrastructure types"`
	Architecture []string `json:"architecture" description:"Available architectures"`
	OsType       []string `json:"osType,omitempty" description:"Available OS types"`

	// Accelerator information
	AcceleratorModel []string `json:"acceleratorModel,omitempty" description:"Available accelerator models"`
	AcceleratorType  []string `json:"acceleratorType,omitempty" description:"Available accelerator types"`

	// Additional fields
	Description      []string `json:"description,omitempty" description:"Available descriptions"`
	EvaluationStatus []string `json:"evaluationStatus,omitempty" description:"Available evaluation statuses"`
}

// PriorityOptionsInfo provides available priority metrics and their parameters
type PriorityOptionsInfo struct {
	// Available metrics for prioritization
	AvailableMetrics []string `json:"availableMetrics" example:"cost,performance,location,latency,random" description:"Available metrics for prioritizing specs"`

	// Example priority policies
	ExamplePolicies []PriorityConditionExample `json:"examplePolicies" description:"Example priority policies"`

	// Parameter options for location and latency metrics
	ParameterOptions ParameterOptionsInfo `json:"parameterOptions" description:"Available parameter options for location and latency metrics"`
}

// PriorityConditionExample provides example priority conditions
type PriorityConditionExample struct {
	Metric      string                   `json:"metric" example:"cost"`
	Description string                   `json:"description" example:"Prioritize by lowest cost"`
	Weight      string                   `json:"weight" example:"1.0"`
	Parameter   []ParameterKeyValExample `json:"parameter,omitempty"`
}

// ParameterKeyValExample provides example parameter key-value pairs
type ParameterKeyValExample struct {
	Key         string   `json:"key" example:"coordinateClose"`
	Description string   `json:"description" example:"Find specs closest to given coordinate"`
	Val         []string `json:"val" example:"37.5665/126.9780"`
}

// ParameterOptionsInfo provides parameter options for location and latency metrics
type ParameterOptionsInfo struct {
	LocationParameters []ParameterOptionDetail `json:"locationParameters" description:"Available parameter options for location-based prioritization"`
	LatencyParameters  []ParameterOptionDetail `json:"latencyParameters" description:"Available parameter options for latency-based prioritization"`
}

// ParameterOptionDetail provides details for parameter options
type ParameterOptionDetail struct {
	Key         string   `json:"key" example:"coordinateClose"`
	Description string   `json:"description" example:"Find specs closest to given coordinate (latitude/longitude)"`
	Format      string   `json:"format" example:"latitude/longitude"`
	Example     []string `json:"example" example:"37.5665/126.9780,35.6762/139.6503"`
}

// StoreLatencyInfo stores latency information to database
func StoreLatencyInfo(sourceRegion, targetRegion string, latencyMs float64) error {
	if sourceRegion == "" || targetRegion == "" {
		return fmt.Errorf("source region and target region cannot be empty")
	}
	if latencyMs < 0 {
		return fmt.Errorf("latency cannot be negative: %f", latencyMs)
	}

	latencyInfo := LatencyInfo{
		SourceRegion: sourceRegion,
		TargetRegion: targetRegion,
		LatencyMs:    latencyMs,
		MeasuredAt:   time.Now(),
		UpdatedAt:    time.Now(),
	}

	result := ORM.Save(&latencyInfo)
	return result.Error
}

// GetLatencyInfo retrieves latency information from database
func GetLatencyInfo(sourceRegion, targetRegion string) (*LatencyInfo, error) {
	if sourceRegion == "" || targetRegion == "" {
		return nil, fmt.Errorf("source region and target region cannot be empty")
	}

	var latencyInfo LatencyInfo
	result := ORM.Where("source_region = ? AND target_region = ?", sourceRegion, targetRegion).First(&latencyInfo)
	if result.Error != nil {
		return nil, result.Error
	}
	return &latencyInfo, nil
}

// GetLatencyValue retrieves latency value between two regions (compatibility function)
func GetLatencyValue(sourceRegion, targetRegion string) (float64, error) {
	latencyInfo, err := GetLatencyInfo(sourceRegion, targetRegion)
	if err != nil {
		return 0, err
	}
	return latencyInfo.LatencyMs, nil
}

// BatchStoreLatencyInfo stores multiple latency records in a single transaction
func BatchStoreLatencyInfo(latencyData []LatencyInfo) error {
	if len(latencyData) == 0 {
		return nil
	}

	return ORM.Transaction(func(tx *gorm.DB) error {
		for _, data := range latencyData {
			data.MeasuredAt = time.Now()
			data.UpdatedAt = time.Now()
			if err := tx.Save(&data).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetAllLatencyInfo retrieves all latency information from database
func GetAllLatencyInfo() ([]LatencyInfo, error) {
	var latencyInfos []LatencyInfo
	result := ORM.Find(&latencyInfos)
	return latencyInfos, result.Error
}

// SpecRegionZoneInfo represents the available zones for a specific region
type SpecRegionZoneInfo struct {
	RegionName string   `json:"regionName" example:"ap-northeast-1"`
	Zones      []string `json:"zones" example:"ap-northeast-1a,ap-northeast-1b"`
}

// SpecAvailabilityInfo represents the availability information for a spec
type SpecAvailabilityInfo struct {
	Provider         string               `json:"provider" example:"alibaba"`
	CspSpecName      string               `json:"cspSpecName" example:"ecs.t5.large"`
	AvailableRegions []SpecRegionZoneInfo `json:"availableRegions"`
	QueryDurationMs  int64                `json:"queryDurationMs" example:"1250"`
	Success          bool                 `json:"success" example:"true"`
	ErrorMessage     string               `json:"errorMessage,omitempty" example:"Spec not available"`
}

// SpecAvailabilityBatchResult represents the batch query result for multiple specs
type SpecAvailabilityBatchResult struct {
	Provider          string                 `json:"provider" example:"alibaba"`
	SpecResults       []SpecAvailabilityInfo `json:"specResults"`
	TotalSpecs        int                    `json:"totalSpecs" example:"10"`
	SuccessfulQueries int                    `json:"successfulQueries" example:"8"`
	FailedQueries     int                    `json:"failedQueries" example:"2"`
	TotalDurationMs   int64                  `json:"totalDurationMs" example:"12500"`
	FastestQueryMs    int64                  `json:"fastestQueryMs" example:"850"`
	SlowestQueryMs    int64                  `json:"slowestQueryMs" example:"2100"`
	AverageQueryMs    int64                  `json:"averageQueryMs" example:"1250"`
}

// SpecCleanupResult represents the result of cleaning up unavailable specs
type SpecCleanupResult struct {
	Provider            string                      `json:"provider" example:"alibaba"`
	Region              string                      `json:"region" example:"ap-northeast-1"`
	TotalSpecsChecked   int                         `json:"totalSpecsChecked" example:"50"`
	SpecsToDelete       int                         `json:"specsToDelete" example:"5"`
	SpecsDeleted        int                         `json:"specsDeleted" example:"5"`
	CleanupDurationMs   int64                       `json:"cleanupDurationMs" example:"15000"`
	AvailabilityCheckMs int64                       `json:"availabilityCheckMs" example:"12500"`
	FailedDeletions     []string                    `json:"failedDeletions,omitempty" example:"ecs.t5.large"`
	AvailabilityResults SpecAvailabilityBatchResult `json:"availabilityResults"`
	// Detailed information about specs that were identified for deletion
	SpecsToIgnoreInfo *SpecsToIgnoreData `json:"specsToIgnoreInfo,omitempty"`
}

// SpecsToIgnoreData represents the structure for specs that should be ignored during availability checks
type SpecsToIgnoreData struct {
	LastUpdated          time.Time                      `json:"last_updated"`
	Description          string                         `json:"description"`
	GlobalIgnoreSpecs    map[string][]string            `json:"global_ignore_specs"`
	RegionSpecificIgnore map[string]map[string][]string `json:"region_specific_ignore"`
}

// Availability request structures for API handlers
type GetAvailableRegionZonesRequest struct {
	Provider    string `json:"provider" validate:"required" example:"alibaba"`
	CspSpecName string `json:"cspSpecName" validate:"required" example:"ecs.t5.large"`
}

type GetAvailableRegionZonesListRequest struct {
	Provider     string   `json:"provider" validate:"required" example:"alibaba"`
	CspSpecNames []string `json:"cspSpecNames" validate:"required" example:"ecs.t5.large,ecs.t5.medium"`
}

type UpdateSpecListByAvailabilityRequest struct {
	Provider string `json:"provider" validate:"required" example:"alibaba"`
}

// CloudSpecIgnoreConfig represents the structure of cloudspec_ignore.yaml
type CloudSpecIgnoreConfig struct {
	Global GlobalIgnorePatterns         `yaml:"global"`
	CSPs   map[string]CSPIgnorePatterns `yaml:"csps,omitempty"`
}

// GlobalIgnorePatterns represents global ignore patterns that apply to all CSPs
type GlobalIgnorePatterns struct {
	Patterns []string `yaml:"patterns"`
}

// CSPIgnorePatterns represents ignore patterns for a specific CSP
type CSPIgnorePatterns struct {
	Description    string                          `yaml:"description,omitempty"`
	GlobalPatterns []string                        `yaml:"global_patterns,omitempty"`
	Regions        map[string]RegionIgnorePatterns `yaml:"regions,omitempty"`
}

// RegionIgnorePatterns represents ignore patterns for a specific region within a CSP
type RegionIgnorePatterns struct {
	AdditionalPatterns []string `yaml:"additional_patterns,omitempty"`
}
