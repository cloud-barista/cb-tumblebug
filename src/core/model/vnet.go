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

// TbVNetReq is a struct to handle 'Create vNet' request toward CB-Tumblebug.
type TbVNetReq struct { // Tumblebug
	Name           string        `json:"name" validate:"required" example:"vnet00"`
	ConnectionName string        `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
	CidrBlock      string        `json:"cidrBlock" example:"10.0.0.0/16"`
	SubnetInfoList []TbSubnetReq `json:"subnetInfoList"`
	Description    string        `json:"description" example:"vnet00 managed by CB-Tumblebug"`
	// todo: restore the tag list later
	// TagList        []KeyValue    `json:"tagList,omitempty"`
}

// TbRegisterVNetReq TbRegisterVNetReq contains the information needed to register a vNet
// that has already been created via another external method.
type TbRegisterVNetReq struct {
	ConnectionName string `json:"connectionName" validate:"required"`
	CspResourceId  string `json:"cspResourceId" validate:"required"`
	Name           string `json:"name" validate:"required"`
	Description    string `json:"description,omitempty"`
}

// TbVNetInfo is a struct that represents TB vNet object.
type TbVNetInfo struct { // Tumblebug
	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name                 string         `json:"name" example:"aws-ap-southeast-1"`
	ConnectionName       string         `json:"connectionName"`
	CidrBlock            string         `json:"cidrBlock"`
	SubnetInfoList       []TbSubnetInfo `json:"subnetInfoList"`
	Description          string         `json:"description"`
	Status               string         `json:"status"`
	KeyValueList         []KeyValue     `json:"keyValueList,omitempty"`
	AssociatedObjectList []string       `json:"associatedObjectList"`
	IsAutoGenerated      bool           `json:"isAutoGenerated"`
	// todo: restore the tag list later
	// TagList              []KeyValue     `json:"tagList,omitempty"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

// BastionNode is a struct that represents TB BastionNode object.
type BastionNode struct {
	MciId string `json:"mciId"`
	VmId  string `json:"vmId"`
}

// VNetDesignRequest is a struct to handle the utility function, DesignVNet()
type VNetDesignRequest struct {
	TargetPrivateNetwork string      `json:"targetPrivateNetwork"`
	SupernettingEnabled  string      `json:"supernettingEnabled"`
	CspRegions           []CspRegion `json:"cspRegions"`
}

type CspRegion struct {
	ConnectionName string       `json:"connectionName"`
	NeededVNets    []NeededVNet `json:"neededVNets"`
}

type NeededVNet struct {
	SubnetCount         int    `json:"subnetCount"`
	SubnetSize          int    `json:"subnetSize"`
	ZoneSelectionMethod string `json:"zoneSelectionMethod"`
}

type VNetDesignResponse struct {
	RootNetworkCIDR string      `json:"rootNetworkCIDR,omitempty"` // in case of supernetting enabled
	VNetReqList     []TbVNetReq `json:"vNetReqList"`
}
