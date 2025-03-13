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
	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	// CspImageName is name of the image given by CSP
	CspImageName string `json:"cspImageName,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name           string `json:"name" example:"aws-ap-southeast-1"`
	Namespace      string `json:"namespace,omitempty" example:"default"` // required to save in RDB
	ConnectionName string `json:"connectionName,omitempty"`
	InfraType      string `json:"infraType,omitempty"` // vm|k8s|kubernetes|container, etc.
	Description    string `json:"description,omitempty"`
	CreationDate   string `json:"creationDate,omitempty"`
	GuestOS        string `json:"guestOS,omitempty"` // Windows7, Ubuntu etc.

	Architecture        string  `json:"architecture" example:"x86_64" description:"The architecture of the operating system of the image."`        // arm64, x86_64 etc.
	Platform            string  `json:"platform" example:"Linux/UNIX" description:"The platform of the operating system of the image."`            // Linux/UNIX, Windows, NA
	Distribution        string  `json:"distribution" example:"Ubuntu 22.04~" description:"The distribution of the operating system of the image."` // Ubuntu 22.04~, CentOS 8 etc.
	RootDiskType      string  `json:"rootDiskType" example:"HDD" description:"The type of the OS disk of for the VM being created."`           // ebs, HDD, etc.
	RootDiskMinSizeGB float32 `json:"rootDiskMinSizeGB" example:"50" description:"The (minimum) OS disk size in GB for the VM being created."` // 10, 50, 100 etc.

	Status               string     `json:"status,omitempty"` // available, unavailable
	KeyValueList         []KeyValue `json:"keyValueList,omitempty"`
	AssociatedObjectList []string   `json:"associatedObjectList,omitempty"`
	IsAutoGenerated      bool       `json:"isAutoGenerated,omitempty"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel,omitempty" example:"Managed by CB-Tumblebug" default:""`
}

// SpiderImageList is struct for Spider Image List
type SpiderImageList struct {
	Image []SpiderImageInfo `json:"image"`
}
