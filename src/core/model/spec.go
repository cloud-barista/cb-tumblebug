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

// TbSpecReq is a struct to handle 'Register spec' request toward CB-Tumblebug.
type TbSpecReq struct {
	// Name is human-readable string to represent the object, used to generate Id
	Name           string `json:"name" validate:"required"`
	ConnectionName string `json:"connectionName" validate:"required"`
	// CspSpecName is name of the spec given by CSP
	CspSpecName string `json:"cspSpecName" validate:"required"`
	Description string `json:"description"`
}

// TbSpecInfo is a struct that represents TB spec object.
type TbSpecInfo struct { // Tumblebug
	// Id is unique identifier for the object
	Id string `json:"id" example:"aws+ap-southeast+csp-06eb41e14121c550a" gorm:"primaryKey"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	// CspSpecName is name of the spec given by CSP
	CspSpecName string `json:"cspSpecName,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name           string `json:"name" example:"aws-ap-southeast-1"`
	Namespace      string `json:"namespace,omitempty" example:"default" gorm:"primaryKey"`
	ConnectionName string `json:"connectionName,omitempty"`
	ProviderName   string `json:"providerName,omitempty"`
	RegionName     string `json:"regionName,omitempty"`
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

// FilterSpecsByRangeRequest is for 'FilterSpecsByRange'
type FilterSpecsByRangeRequest struct {
	Id                  string `json:"id"`
	Name                string `json:"name"`
	ConnectionName      string `json:"connectionName"`
	ProviderName        string `json:"providerName"`
	RegionName          string `json:"regionName"`
	CspSpecName         string `json:"cspSpecName"`
	InfraType           string `json:"infraType"`
	Architecture        string `json:"architecture"`
	OsType              string `json:"osType"`
	VCPU                Range  `json:"vCPU"`
	MemoryGiB           Range  `json:"memoryGiB"`
	DiskSizeGB          Range  `json:"diskSizeGB"`
	MaxTotalStorageTiB  Range  `json:"maxTotalStorageTiB"`
	NetBwGbps           Range  `json:"netBwGbps"`
	AcceleratorModel    string `json:"acceleratorModel"`
	AcceleratorCount    Range  `json:"acceleratorCount"`
	AcceleratorMemoryGB Range  `json:"acceleratorMemoryGB"`
	AcceleratorType     string `json:"acceleratorType"`
	CostPerHour         Range  `json:"costPerHour"`
	Description         string `json:"description"`
	EvaluationStatus    string `json:"evaluationStatus"`
	EvaluationScore01   Range  `json:"evaluationScore01"`
	EvaluationScore02   Range  `json:"evaluationScore02"`
	EvaluationScore03   Range  `json:"evaluationScore03"`
	EvaluationScore04   Range  `json:"evaluationScore04"`
	EvaluationScore05   Range  `json:"evaluationScore05"`
	EvaluationScore06   Range  `json:"evaluationScore06"`
	EvaluationScore07   Range  `json:"evaluationScore07"`
	EvaluationScore08   Range  `json:"evaluationScore08"`
	EvaluationScore09   Range  `json:"evaluationScore09"`
	EvaluationScore10   Range  `json:"evaluationScore10"`
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
