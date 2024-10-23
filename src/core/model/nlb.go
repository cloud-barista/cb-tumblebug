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

const nlbPostfix = "-nlb"

// 2022-07-15 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/NLBHandler.go

// SpiderNLBReqInfoWrapper is a wrapper struct to create JSON body of 'Create NLB request'
type SpiderNLBReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderNLBReqInfo
}

// 2022-07-15 https://github.com/cloud-barista/cb-spider/blob/a3ee3030e4b956bcb27691203f286265ab3a926e/api-runtime/rest-runtime/CCMRest.go#L1895

// SpiderNLBReqInfo is a struct to create JSON body of 'Create NLB request'
type SpiderNLBReqInfo struct {
	Name    string
	VPCName string
	Type    string // PUBLIC(V) | INTERNAL
	Scope   string // REGION(V) | GLOBAL

	//------ Frontend

	Listener NLBListenerReq

	//------ Backend

	VMGroup       SpiderNLBSubGroupReq
	HealthChecker SpiderNLBHealthCheckerReq
}

type SpiderNLBHealthCheckerReq struct {
	Protocol  string `json:"protocol" example:"TCP"`      // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`           // Listener Port or 1-65535
	Interval  string `json:"interval" example:"default"`  // secs, Interval time between health checks.
	Timeout   string `json:"timeout" example:"default"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold string `json:"threshold" example:"default"` // num, The number of continuous health checks to change the VM status.
}

type TbNLBHealthCheckerReq struct {
	// Protocol  string `json:"protocol" example:"TCP"`      // TCP|HTTP|HTTPS
	// Port      string `json:"port" example:"22"`           // Listener Port or 1-65535

	Interval  string `json:"interval" example:"default"`  // secs, Interval time between health checks.
	Timeout   string `json:"timeout" example:"default"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold string `json:"threshold" example:"default"` // num, The number of continuous health checks to change the VM status.
}

type SpiderNLBSubGroupReq struct {
	Protocol string // TCP|HTTP|HTTPS
	Port     string // Listener Port or 1-65535
	VMs      []string
}

// SpiderNLBAddRemoveVMReqInfoWrapper is a wrapper struct to create JSON body of 'Add/Remove VMs to/from NLB' request
type SpiderNLBAddRemoveVMReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderNLBSubGroupReq
}

// SpiderNLBInfo is a struct to handle NLB information from the CB-Spider's REST API response
type SpiderNLBInfo struct {
	IId    IID // {NameId, SystemId}
	VpcIID IID // {NameId, SystemId}

	Type  string // PUBLIC(V) | INTERNAL
	Scope string // REGION(V) | GLOBAL

	//------ Frontend
	Listener SpiderNLBListenerInfo

	//------ Backend
	VMGroup       SpiderNLBSubGroupInfo
	HealthChecker SpiderNLBHealthCheckerInfo

	CreatedTime  time.Time
	KeyValueList []KeyValue
}

// NLBListenerReq is a struct to handle NLB Listener information of the CB-Spider's & CB-Tumblebug's REST API request
type NLBListenerReq struct { // for both Spider and Tumblebug
	Protocol string `json:"protocol" example:"TCP"` // TCP|UDP
	Port     string `json:"port" example:"80"`      // 1-65535
}

// SpiderNLBListenerInfo is a struct to handle NLB Listener information from the CB-Spider's REST API response
type SpiderNLBListenerInfo struct {
	Protocol string `json:"protocol" example:"TCP"` // TCP|UDP
	IP       string `json:"ip" example:""`          // Auto Generated and attached
	Port     string `json:"port" example:"80"`      // 1-65535
	DNSName  string `json:"dnsName" example:""`     // Optional, Auto Generated and attached

	CspID        string     `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []KeyValue `json:"keyValueList"`
}

// TbNLBListenerInfo is a struct to handle NLB Listener information from the CB-Tumblebug's REST API response
type TbNLBListenerInfo struct {
	Protocol string `json:"protocol" example:"TCP"`                                               // TCP|UDP
	IP       string `json:"ip" example:"x.x.x.x"`                                                 // Auto Generated and attached
	Port     string `json:"port" example:"80"`                                                    // 1-65535
	DNSName  string `json:"dnsName" example:"default-group-cd3.elb.ap-northeast-2.amazonaws.com"` // Optional, Auto Generated and attached

	KeyValueList []KeyValue `json:"keyValueList"`
}

// SpiderNLBSubGroupInfo is a struct from NLBSubGroupInfo from Spider
type SpiderNLBSubGroupInfo struct {
	Protocol string // TCP|UDP|HTTP|HTTPS
	Port     string // 1-65535
	VMs      *[]IID

	CspID        string     `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []KeyValue `json:"keyValueList"`
}

type SpiderNLBHealthCheckerInfo struct {
	Protocol  string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`      // Listener Port or 1-65535
	Interval  int    `json:"interval" example:"10"`  // secs, Interval time between health checks.
	Timeout   int    `json:"timeout" example:"10"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold int    `json:"threshold" example:"3"`  // num, The number of continuous health checks to change the VM status.

	CspID        string     `json:"cspID"` // Optional, May be Used by Driver.
	KeyValueList []KeyValue `json:"keyValueList"`
}

type TbNLBHealthCheckerInfo struct {
	Protocol  string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port      string `json:"port" example:"22"`      // Listener Port or 1-65535
	Interval  int    `json:"interval" example:"10"`  // secs, Interval time between health checks.
	Timeout   int    `json:"timeout" example:"10"`   // secs, Waiting time to decide an unhealthy VM when no response.
	Threshold int    `json:"threshold" example:"3"`  // num, The number of continuous health checks to change the VM status.

	KeyValueList []KeyValue `json:"keyValueList"`
}

type SpiderNLBHealthInfoWrapper struct {
	Healthinfo SpiderNLBHealthInfo
}

type SpiderNLBHealthInfo struct {
	AllVMs       *[]IID
	HealthyVMs   *[]IID
	UnHealthyVMs *[]IID
}

type TbNLBTargetGroupReq struct {
	Protocol   string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port       string `json:"port" example:"80"`      // Listener Port or 1-65535
	SubGroupId string `json:"subGroupId" example:"g1"`
}

type TbNLBTargetGroupInfo struct {
	Protocol string `json:"protocol" example:"TCP"` // TCP|HTTP|HTTPS
	Port     string `json:"port" example:"80"`      // Listener Port or 1-65535

	SubGroupId string   `json:"subGroupId" example:"g1"`
	VMs        []string `json:"vms"`

	KeyValueList []KeyValue
}

// TbNLBReq is a struct to handle 'Create nlb' request toward CB-Tumblebug.
type TbNLBReq struct { // Tumblebug
	// Name           string `json:"name" validate:"required" example:"mc"`
	// ConnectionName string `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
	// VNetId         string `json:"vNetId" validate:"required" example:"default-shared-aws-ap-northeast-2"`

	Description string `json:"description"`
	// Existing NLB (used only for option=register)
	CspResourceId string `json:"cspResourceId"`

	Type  string `json:"type" validate:"required" enums:"PUBLIC,INTERNAL" example:"PUBLIC"` // PUBLIC(V) | INTERNAL
	Scope string `json:"scope" validate:"required" enums:"REGION,GLOBAL" example:"REGION"`  // REGION(V) | GLOBAL

	// Frontend
	Listener NLBListenerReq `json:"listener" validate:"required"`
	// Backend
	TargetGroup TbNLBTargetGroupReq `json:"targetGroup" validate:"required"`
	// HealthChecker
	HealthChecker TbNLBHealthCheckerReq `json:"healthChecker" validate:"required"`
}

// TbNLBInfo is a struct that represents TB nlb object.
type TbNLBInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name string `json:"name" example:"aws-ap-southeast-1"`

	ConnectionName   string     `json:"connectionName"`
	ConnectionConfig ConnConfig `json:"connectionConfig"`

	Type  string // PUBLIC(V) | INTERNAL
	Scope string // REGION(V) | GLOBAL

	//------ Frontend

	Listener TbNLBListenerInfo `json:"listener"`

	//------ Backend

	TargetGroup   TbNLBTargetGroupInfo   `json:"targetGroup"`
	HealthChecker TbNLBHealthCheckerInfo `json:"healthChecker"`

	CreatedTime time.Time

	Description          string     `json:"description"`
	Status               string     `json:"status"`
	KeyValueList         []KeyValue `json:"keyValueList"`
	AssociatedObjectList []string   `json:"associatedObjectList"`
	IsAutoGenerated      bool       `json:"isAutoGenerated"`
	Location             Location   `json:"location"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

type TbNLBHealthInfo struct { // Tumblebug
	AllVMs       []string
	HealthyVMs   []string
	UnHealthyVMs []string
}

// TbNLBAddRemoveVMReq is a struct to handle 'Add/Remove VMs to/from NLB' request toward CB-Tumblebug.
type TbNLBAddRemoveVMReq struct { // Tumblebug
	TargetGroup TbNLBTargetGroupInfo `json:"targetGroup"`
}

// McNlbInfo is a struct for response of CreateMcSwNlb
type McNlbInfo struct {
	MciAccessInfo *MciAccessInfo  `json:"mciAccessInfo"`
	McNlbHostInfo *TbMciInfo      `json:"mcNlbHostInfo"`
	DeploymentLog MciSshCmdResult `json:"deploymentLog"`
}
