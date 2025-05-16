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

// SpiderImageReqInfoWrapper is a wrapper struct to create JSON body of 'Get image request'
type SpiderImageReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderImageInfo
}

type OSArchitecture string

const (
	ARM32          OSArchitecture = "arm32"
	ARM64          OSArchitecture = "arm64"
	ARM64_MAC      OSArchitecture = "arm64_mac"
	X86_32         OSArchitecture = "x86_32"
	X86_64         OSArchitecture = "x86_64"
	X86_32_MAC     OSArchitecture = "x86_32_mac"
	X86_64_MAC     OSArchitecture = "x86_64_mac"
	ArchitectureNA OSArchitecture = "NA"
)

type OSPlatform string

const (
	Linux_UNIX OSPlatform = "Linux/UNIX"
	Windows    OSPlatform = "Windows"
	PlatformNA OSPlatform = "NA"
)

type ImageStatus string

const (
	ImageAvailable   ImageStatus = "Available"
	ImageUnavailable ImageStatus = "Unavailable"
	ImageNA          ImageStatus = "NA"
)

// SpiderImageInfo represents the information of an Image.
type SpiderImageInfo struct {
	IId IID `json:"IId" description:"The ID of the image."` // {NameId, SystemId}, {ami-00aa5a103ddf4509f, ami-00aa5a103ddf4509f}

	Name           string         `json:"Name" example:"ami-00aa5a103ddf4509f" description:"The name of the image."`                                   // ami-00aa5a103ddf4509f
	OSArchitecture OSArchitecture `json:"OSArchitecture" example:"x86_64" description:"The architecture of the operating system of the image."`        // arm64, x86_64 etc.
	OSPlatform     OSPlatform     `json:"OSPlatform" example:"Linux/UNIX" description:"The platform of the operating system of the image."`            // Linux/UNIX, Windows, NA
	OSDistribution string         `json:"OSDistribution" example:"Ubuntu 22.04~" description:"The distribution of the operating system of the image."` // Ubuntu 22.04~, CentOS 8 etc.
	OSDiskType     string         `json:"OSDiskType" example:"HDD" description:"The type of the OS disk of for the VM being created."`                 // ebs, HDD, etc.
	OSDiskSizeGB   string         `json:"OSDiskSizeGB" example:"50" description:"The (minimum) OS disk size in GB for the VM being created."`          // 10, 50, 100 etc.

	ImageStatus ImageStatus `json:"ImageStatus" example:"Available" description:"The status of the image, e.g., Available or Unavailable."` // Available, Unavailable

	KeyValueList []KeyValue `json:"KeyValueList,omitempty" validate:"omitempty" description:"A list of key-value pairs associated with the image."`
}

// TbImageReq is a struct to handle 'Register image' request toward CB-Tumblebug.
type TbImageReq struct {
	Name           string `json:"name" validate:"required"`
	ConnectionName string `json:"connectionName" validate:"required"`
	CspImageName   string `json:"cspImageName" validate:"required"`
	Description    string `json:"description"`
}

// TbImageInfo is a struct that represents TB image object.
type TbImageInfo struct {
	// Composite primary key
	Namespace    string `json:"namespace" example:"default" gorm:"primaryKey"`
	ProviderName string `json:"providerName" gorm:"primaryKey"`
	CspImageName string `json:"cspImageName" example:"csp-06eb41e14121c550a" gorm:"primaryKey" description:"The name of the CSP image for querying image information."`

	// Array field for supporting multiple regions
	RegionList []string `json:"regionList" gorm:"type:text;serializer:json"`

	Id  string `json:"id" example:"aws-ap-southeast-1"`
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	Name           string `json:"name" example:"aws-ap-southeast-1"`
	ConnectionName string `json:"connectionName,omitempty"`
	InfraType      string `json:"infraType,omitempty"` // vm|k8s|kubernetes|container, etc.

	FetchedTime  string `json:"fetchedTime,omitempty"`
	CreationDate string `json:"creationDate,omitempty"`

	IsGPUImage        bool `json:"isGPUImage,omitempty" gorm:"column:is_gpu_image" enum:"true|false" default:"false" description:"Whether the image is GPU-enabled or not."`
	IsKubernetesImage bool `json:"isKubernetesImage,omitempty" gorm:"column:is_kubernetes_image" enum:"true|false" default:"false" description:"Whether the image is Kubernetes-enabled or not."`

	OSType string `json:"osType,omitempty" gorm:"column:os_type" example:"ubuntu 22.04" description:"Simplified OS name and version string"`

	OSArchitecture OSArchitecture `json:"osArchitecture" gorm:"column:os_architecture" example:"x86_64" description:"The architecture of the operating system of the image."`        // arm64, x86_64 etc.
	OSPlatform     OSPlatform     `json:"osPlatform" gorm:"column:os_platform" example:"Linux/UNIX" description:"The platform of the operating system of the image."`                // Linux/UNIX, Windows, NA
	OSDistribution string         `json:"osDistribution" gorm:"column:os_distribution" example:"Ubuntu 22.04~" description:"The distribution of the operating system of the image."` // Ubuntu 22.04~, CentOS 8 etc.
	OSDiskType     string         `json:"osDiskType" gorm:"column:os_disk_type" example:"HDD" description:"The type of the OS disk of for the VM being created."`                    // ebs, HDD, etc.
	OSDiskSizeGB   float64        `json:"osDiskSizeGB" gorm:"column:os_disk_size_gb" example:"50" description:"The (minimum) OS disk size in GB for the VM being created."`          // 10, 50, 100 etc.
	ImageStatus    ImageStatus    `json:"imageStatus" example:"Available" description:"The status of the image, e.g., Available or Unavailable."`                                    // Available, Unavailable

	KeyValueList []KeyValue `json:"keyValueList" gorm:"type:text;serializer:json"`
	SystemLabel  string     `json:"systemLabel,omitempty" example:"Managed by CB-Tumblebug" default:""`
	Description  string     `json:"description,omitempty"`
}

// ImageFetchOption is struct for Image Fetch Options
type ImageFetchOption struct {
	// providers need to be excluded from the image fetching operation (ex: ["azure"])
	ExcludedProviders []string `json:"excludedProviders,omitempty" example:"azure" description:"Providers to be excluded from the image fetching operation."`

	// providers that are not region-specific (ex: ["gcp"])
	RegionAgnosticProviders []string `json:"regionAgnosticProviders,omitempty" example:"gcp,tencent" description:"Providers that are not region-specific."`
}

// SearchImageRequest is struct for Search Image Request
type SearchImageRequest struct {
	ProviderName      string   `json:"providerName" example:"aws"`
	RegionName        string   `json:"regionName" example:"us-east-1"`
	OSType            string   `json:"osType" example:"ubuntu 22.04" description:"Simplified OS name and version string. Space-separated for AND condition"`
	IsGPUImage        *bool    `json:"isGPUImage" example:"false"`
	IsKubernetesImage *bool    `json:"isKubernetesImage" example:"false"`
	DetailSearchKeys  []string `json:"detailSearchKeys" example:"sql,2022" description:"Keywords for searching images in detail"`
}

// SearchImageResponse is struct for Search Image Request
type SearchImageResponse struct {
	Count     int           `json:"count"`
	ImageList []TbImageInfo `json:"imageList"`
}

// SpiderImageList is struct for Spider Image List
type SpiderImageList struct {
	Image []SpiderImageInfo `json:"image"`
}
