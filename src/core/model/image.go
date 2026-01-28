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
	ARM32               OSArchitecture = "arm32"
	ARM64               OSArchitecture = "arm64"
	ARM64_MAC           OSArchitecture = "arm64_mac"
	X86_32              OSArchitecture = "x86_32"
	X86_64              OSArchitecture = "x86_64"
	X86_32_MAC          OSArchitecture = "x86_32_mac"
	X86_64_MAC          OSArchitecture = "x86_64_mac"
	S390X               OSArchitecture = "s390x"
	ArchitectureNA      OSArchitecture = "NA"
	ArchitectureUnknown OSArchitecture = ""
)

type OSPlatform string

const (
	Linux_UNIX OSPlatform = "Linux/UNIX"
	Windows    OSPlatform = "Windows"
	PlatformNA OSPlatform = "NA"
)

type ImageStatus string

const (
	// ImageCreating indicates the image is being created (e.g., snapshot in progress)
	// This is a CB-Tumblebug managed state, independent of CB-Spider's status
	ImageCreating ImageStatus = "Creating"

	// ImageAvailable indicates the image is ready and can be used
	ImageAvailable ImageStatus = "Available"

	// ImageFailed indicates the image creation failed
	// This is a terminal state - no further status updates needed
	ImageFailed ImageStatus = "Failed"

	// ImageUnavailable indicates the image is temporarily unavailable
	// This may transition to Available or Failed
	ImageUnavailable ImageStatus = "Unavailable"

	// ImageDeleting indicates the image is being deleted
	ImageDeleting ImageStatus = "Deleting"

	// ImageDeprecated indicates the image is deprecated and should not be used
	ImageDeprecated ImageStatus = "Deprecated"

	// ImageNA indicates the status is not applicable or unknown
	ImageNA ImageStatus = "NA"
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

// ImageSummary is a lightweight struct containing essential image information for VmInfo
type ImageSummary struct {
	ResourceType   string         `json:"resourceType,omitempty" example:"image" description:"image or customImage"`
	CspImageName   string         `json:"cspImageName,omitempty" example:"ami-0123456789abcdef0"`
	OSType         string         `json:"osType" gorm:"column:os_type" example:"ubuntu 22.04" description:"Simplified OS name and version string"`
	OSArchitecture OSArchitecture `json:"osArchitecture,omitempty" example:"x86_64"`
	OSDistribution string         `json:"osDistribution,omitempty" example:"Ubuntu 22.04"`
}

// ImageReq is a struct to handle 'Register image' request toward CB-Tumblebug.
type ImageReq struct {
	Name           string `json:"name" validate:"required"`
	ConnectionName string `json:"connectionName" validate:"required"`
	CspImageName   string `json:"cspImageName" validate:"required"`
	Description    string `json:"description"`
}

// ImageInfo is a struct that represents TB image object.
type ImageInfo struct {

	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Composite primary key
	Namespace    string `json:"namespace" example:"default" gorm:"primaryKey"`
	ProviderName string `json:"providerName" gorm:"primaryKey"`
	CspImageName string `json:"cspImageName" example:"csp-06eb41e14121c550a" gorm:"primaryKey" description:"The name of the CSP image for querying image information."`

	// Array field for supporting multiple regions
	RegionList []string `json:"regionList" gorm:"type:text;serializer:json"`

	Id   string `json:"id" example:"aws-ap-southeast-1"`
	Uid  string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	Name string `json:"name" example:"aws-ap-southeast-1"`

	// CspImageId is resource identifier managed by CSP
	CspImageId string `json:"cspImageId,omitempty" example:"ami-0d399fba46a30a310"`
	// SourceVmUid is the UID of the source VM from which this image was created
	SourceVmUid string `json:"sourceVmUid" example:"wef12awefadf1221edcf"`
	// SourceCspImageName is the name of the source CSP image from which this image was created
	SourceCspImageName string `json:"sourceCspImageName" example:"csp-06eb41e14121c550a"`

	ConnectionName string `json:"connectionName"`
	InfraType      string `json:"infraType"` // vm|k8s|kubernetes|container, etc.

	FetchedTime  string `json:"fetchedTime"`
	CreationDate string `json:"creationDate"`

	IsGPUImage        bool `json:"isGPUImage" gorm:"column:is_gpu_image" enum:"true|false" default:"false" description:"Whether the image is GPU-enabled or not."`
	IsKubernetesImage bool `json:"isKubernetesImage" gorm:"column:is_kubernetes_image" enum:"true|false" default:"false" description:"Whether the image is Kubernetes-enabled or not."`
	IsBasicImage      bool `json:"isBasicImage" gorm:"column:is_basic_image" enum:"true|false" default:"false" description:"Whether the image is a basic OS image or not."`

	OSType string `json:"osType" gorm:"column:os_type" example:"ubuntu 22.04" description:"Simplified OS name and version string"`

	OSArchitecture OSArchitecture `json:"osArchitecture" gorm:"column:os_architecture" example:"x86_64" description:"The architecture of the operating system of the image."`        // arm64, x86_64 etc.
	OSPlatform     OSPlatform     `json:"osPlatform" gorm:"column:os_platform" example:"Linux/UNIX" description:"The platform of the operating system of the image."`                // Linux/UNIX, Windows, NA
	OSDistribution string         `json:"osDistribution" gorm:"column:os_distribution" example:"Ubuntu 22.04~" description:"The distribution of the operating system of the image."` // Ubuntu 22.04~, CentOS 8 etc.
	OSDiskType     string         `json:"osDiskType" gorm:"column:os_disk_type" example:"HDD" description:"The type of the OS disk of for the VM being created."`                    // ebs, HDD, etc.
	OSDiskSizeGB   float64        `json:"osDiskSizeGB" gorm:"column:os_disk_size_gb" example:"50" description:"The (minimum) OS disk size in GB for the VM being created."`          // 10, 50, 100 etc.
	ImageStatus    ImageStatus    `json:"imageStatus" example:"Available" description:"The status of the image, e.g., Available, Deprecated, NA."`                                   // Available, Deprecated, NA

	Details     []KeyValue `json:"details" gorm:"type:text;serializer:json"`
	SystemLabel string     `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`
	Description string     `json:"description"`

	// CommandHistory stores the status and history of remote commands executed on this VM
	CommandHistory []ImageSourceCommandHistory `json:"commandHistory" gorm:"type:text;serializer:json"`
}

// ImageSourceCommandHistory represents a single remote command execution record
type ImageSourceCommandHistory struct {
	// Index is sequential identifier for this command execution (1, 2, 3, ...)
	Index int `json:"index" example:"1"`
	// CommandExecuted is the actual SSH command executed on the VM (may be adjusted)
	CommandExecuted string `json:"commandExecuted" example:"ls -la"`
}

// ImageFetchOption is struct for Image Fetch Options
type ImageFetchOption struct {
	// Specific providers to target for the image fetching operation (ex: ["aws", "gcp"])
	// If specified, only these providers will be processed (excludedProviders will be ignored)
	TargetProviders []string `json:"targetProviders,omitempty" example:"aws,gcp" description:"Specific providers to target. If specified, only these providers will be processed."`

	// providers need to be excluded from the image fetching operation (ex: ["azure"])
	ExcludedProviders []string `json:"excludedProviders,omitempty" example:"azure" description:"Providers to be excluded from the image fetching operation."`

	// providers that are not region-specific (ex: ["gcp"])
	RegionAgnosticProviders []string `json:"regionAgnosticProviders,omitempty" example:"gcp,tencent" description:"Providers that are not region-specific."`
}

// SearchImageRequest is struct for Search Image Request
type SearchImageRequest struct {

	// MatchedSpecId is the ID of the matched spec.
	// If specified, only the images that match this spec will be returned.
	// This is useful when the user wants to search images that match a specific spec.
	MatchedSpecId string `json:"matchedSpecId,omitempty" example:"aws+ap-northeast-2+t2.small" description:"The ID of the matched spec. If specified, only the images that match this spec will be returned."`

	// Cloud Service Provider (ex: "aws", "azure", "gcp", etc.). Use GET /provider to get the list of available providers.
	ProviderName string `json:"providerName" example:"aws"`

	// Cloud Service Provider Region (ex: "us-east-1", "us-west-2", etc.). Use GET /provider/{providerName}/region to get the list of available regions.
	RegionName string `json:"regionName" example:"us-east-1"`

	// Simplified OS name and version string. Space-separated for AND condition (ex: "ubuntu 22.04", "windows 10", etc.).
	OSType string `json:"osType" example:"ubuntu 22.04" description:"Simplified OS name and version string. Space-separated for AND condition"`

	// The architecture of the operating system of the image. (ex: "x86_64", "arm64", etc.)
	OSArchitecture OSArchitecture `json:"osArchitecture" gorm:"column:os_architecture" example:"x86_64" description:"The architecture of the operating system of the image."`

	// Whether the image is ready for GPU usage or not.
	// In usual, true means the image is ready for GPU usage with GPU drivers and libraries installed.
	// If not specified, both true and false images will be included in the search results.
	// Even if the image is not ready for GPU usage, it can be used with GPU by installing GPU drivers and libraries manually.
	IsGPUImage *bool `json:"isGPUImage" example:"false"`

	// Whether the image is specialized image only for Kubernetes nodes.
	// If not specified, both true and false images will be included in the search results.
	// Images that are not specialized for Kubernetes also can be used as Kubernetes nodes. It depends on CSPs.
	IsKubernetesImage *bool `json:"isKubernetesImage" example:"false"`

	// Whether the image is registered by CB-Tumblebug asset file or not.
	IsRegisteredByAsset *bool `json:"isRegisteredByAsset" example:"false" description:"Whether the image is registered by asset or not."`

	// Whether the search results should include deprecated images or not.
	// If not specified, deprecated images will not be included in the search results.
	// In usual, deprecated images are not recommended to use, but they can be used if necessary.
	IncludeDeprecatedImage *bool `json:"includeDeprecatedImage" example:"false" description:"Include deprecated images in the search results."`

	// IncludeBasicImageOnly is to return basic OS distribution only without additional applications.
	// If true, the search results will include only the basic OS distribution without additional applications.
	// If false or not specified, the search results will include images with additional applications installed.
	IncludeBasicImageOnly *bool `json:"includeBasicImageOnly" example:"false" description:"Return basic OS distribution only without additional applications."`

	// MaxResults is the maximum number of images to be returned in the search results.
	// If not specified, all images will be returned.
	// If specified, the number of images returned will be limited to the specified value.
	MaxResults *int `json:"maxResults" example:"100" description:"Maximum number of images to be returned in the search results. If not specified, all images will be returned."`

	// Keywords for searching images in detail.
	// Space-separated for AND condition (ex: "sql 2022", "ubuntu 22.04", etc.).
	// Used for if the user wants to search images with specific keywords in their details.
	DetailSearchKeys []string `json:"detailSearchKeys" example:"tensorflow,2.17" description:"Keywords for searching images in detail"`
}

// SearchImageRequestOptions is struct for Search Image Request
type SearchImageRequestOptions struct {

	// MatchedSpecId is the ID of the matched spec.
	// If specified, only the images that match this spec will be returned.
	// This is useful when the user wants to search images that match a specific spec.
	MatchedSpecId []string `json:"matchedSpecId" example:"aws+ap-northeast-2+t2.small" description:"The ID of the matched spec. If specified, only the images that match this spec will be returned."`

	// Cloud Service Provider (ex: "aws", "azure", "gcp", etc.). Use GET /provider to get the list of available providers.
	ProviderName []string `json:"providerName"`

	// Cloud Service Provider Region (ex: "us-east-1", "us-west-2", etc.). Use GET /provider/{providerName}/region to get the list of available regions.
	RegionName []string `json:"regionName"`

	// Simplified OS name and version string. Space-separated for AND condition (ex: "ubuntu 22.04", "windows 10", etc.).
	OSType []string `json:"osType" description:"Simplified OS name and version string. Space-separated for AND condition"`

	// The architecture of the operating system of the image. (ex: "x86_64", "arm64", etc.)
	OSArchitecture []string `json:"osArchitecture" description:"The architecture of the operating system of the image."`

	// Whether the image is ready for GPU usage or not.
	// In usual, true means the image is ready for GPU usage with GPU drivers and libraries installed.
	// If not specified, both true and false images will be included in the search results.
	// Even if the image is not ready for GPU usage, it can be used with GPU by installing GPU drivers and libraries manually.
	IsGPUImage []bool `json:"isGPUImage" description:"Whether the image is ready for GPU usage or not."`

	// Whether the image is specialized image only for Kubernetes nodes.
	// If not specified, both true and false images will be included in the search results.
	// Images that are not specialized for Kubernetes also can be used as Kubernetes nodes. It depends on CSPs.
	IsKubernetesImage []bool `json:"isKubernetesImage" description:"Whether the image is specialized image only for Kubernetes nodes."`

	// Whether the image is registered by CB-Tumblebug asset file or not.
	IsRegisteredByAsset []bool `json:"isRegisteredByAsset" description:"Whether the image is registered by asset or not."`

	// Whether the search results should include deprecated images or not.
	// If not specified, deprecated images will not be included in the search results.
	// In usual, deprecated images are not recommended to use, but they can be used if necessary.
	IncludeDeprecatedImage []bool `json:"includeDeprecatedImage" description:"Include deprecated images in the search results."`

	// MaxResults is the maximum number of images to be returned in the search results.
	// If not specified, all images will be returned.
	// If specified, the number of images returned will be limited to the specified value.
	MaxResults []int `json:"maxResults" example:"100" description:"Maximum number of images to be returned in the search results. If not specified, all images will be returned."`

	// Keywords for searching images in detail.
	// Space-separated for AND condition (ex: "sql 2022", "ubuntu 22.04", etc.).
	// Used for if the user wants to search images with specific keywords in their details.
	DetailSearchKeys [][]string `json:"detailSearchKeys" description:"Keywords for searching images in detail"`
}

// SearchImageResponse is struct for Search Image Request
type SearchImageResponse struct {
	ImageCount int         `json:"imageCount"`
	ImageList  []ImageInfo `json:"imageList"`
}

// SpiderImageList is struct for Spider Image List
type SpiderImageList struct {
	Image []SpiderImageInfo `json:"image"`
}
