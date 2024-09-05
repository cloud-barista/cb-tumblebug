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
	"time"
)

type DiskStatus string

const (
	DiskCreating  DiskStatus = "Creating"
	DiskAvailable DiskStatus = "Available"
	DiskAttached  DiskStatus = "Attached"
	DiskDeleting  DiskStatus = "Deleting"
	DiskError     DiskStatus = "Error"
)

// TbAttachDetachDataDiskReq is a wrapper struct to create JSON body of 'Attach/Detach disk request'
type TbAttachDetachDataDiskReq struct {
	DataDiskId string `json:"dataDiskId" validate:"required"`
}

// SpiderDiskAttachDetachReqWrapper is a wrapper struct to create JSON body of 'Attach/Detach disk request'
type SpiderDiskAttachDetachReqWrapper struct {
	ConnectionName string
	ReqInfo        SpiderDiskAttachDetachReq
}

// SpiderDiskAttachDetachReq is a struct to create JSON body of 'Attach/Detach disk request'
type SpiderDiskAttachDetachReq struct {
	VMName string
}

// SpiderDiskUpsizeReqWrapper is a wrapper struct to create JSON body of 'Upsize disk request'
type SpiderDiskUpsizeReqWrapper struct {
	ConnectionName string
	ReqInfo        SpiderDiskUpsizeReq
}

// SpiderDiskUpsizeReq is a struct to create JSON body of 'Upsize disk request'
type SpiderDiskUpsizeReq struct {
	Size string // "", "default", "50", "1000"  # (GB)
}

// SpiderDiskReqInfoWrapper is a wrapper struct to create JSON body of 'Get disk request'
type SpiderDiskReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderDiskInfo
}

// SpiderDiskInfo is a struct to create JSON body of 'Get disk request'
type SpiderDiskInfo struct {
	// Fields for request
	Name  string
	CSPid string

	// Fields for both request and response
	DiskType string // "", "SSD(gp2)", "Premium SSD", ...
	DiskSize string // "", "default", "50", "1000"  # (GB)

	// Fields for response
	IId IID // {NameId, SystemId}

	Status  DiskStatus // DiskCreating | DiskAvailable | DiskAttached | DiskDeleting | DiskError
	OwnerVM IID        // When the Status is DiskAttached

	CreatedTime  time.Time
	KeyValueList []KeyValue
}

// TbDataDiskReq is a struct to handle 'Register dataDisk' request toward CB-Tumblebug.
type TbDataDiskReq struct {
	Name           string `json:"name" validate:"required" example:"aws-ap-southeast-1-datadisk"`
	ConnectionName string `json:"connectionName" validate:"required" example:"aws-ap-southeast-1"`
	DiskType       string `json:"diskType" example:"default"`
	DiskSize       string `json:"diskSize" validate:"required" example:"77" default:"100"`
	Description    string `json:"description,omitempty"`

	// Fields for "Register existing dataDisk" feature
	// CspResourceId is required to register object from CSP (option=register)
	CspResourceId string `json:"cspResourceId"`
}

// TbDataDiskVmReq is a struct to handle 'Provisioning dataDisk to VM' request toward CB-Tumblebug.
type TbDataDiskVmReq struct {
	Name        string `json:"name" validate:"required" example:"aws-ap-southeast-1-datadisk"`
	DiskType    string `json:"diskType" example:"default"`
	DiskSize    string `json:"diskSize" validate:"required" example:"77" default:"100"`
	Description string `json:"description,omitempty"`
}

// TbDataDiskInfo is a struct that represents TB dataDisk object.
type TbDataDiskInfo struct {

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceHandlingName is identifier to handle CSP resource
	CspResourceHandlingName string `json:"cspResourceHandlingName,omitempty" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name                 string     `json:"name" example:"aws-ap-southeast-1"`
	ConnectionName       string     `json:"connectionName,omitempty" example:"aws-ap-southeast-1"`
	DiskType             string     `json:"diskType" example:"standard"`
	DiskSize             string     `json:"diskSize" example:"77"`
	Status               DiskStatus `json:"status" example:"Available"` // Available, Unavailable, Attached, ...
	AssociatedObjectList []string   `json:"associatedObjectList" example:"/ns/default/mci/mci01/vm/aws-ap-southeast-1-1"`
	CreatedTime          time.Time  `json:"createdTime,omitempty" example:"2022-10-12T05:09:51.05Z"`
	KeyValueList         []KeyValue `json:"keyValueList,omitempty"`
	Description          string     `json:"description,omitempty" example:"Available"`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	IsAutoGenerated bool `json:"isAutoGenerated,omitempty"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel,omitempty" example:"Managed by CB-Tumblebug" default:""`
}

// TbDataDiskUpsizeReq is a struct to handle 'Upsize dataDisk' request toward CB-Tumblebug.
type TbDataDiskUpsizeReq struct {
	DiskSize    string `json:"diskSize" validate:"required"`
	Description string `json:"description"`
}
