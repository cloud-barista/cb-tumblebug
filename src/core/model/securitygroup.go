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

// SpiderSecurityReqInfoWrapper is a wrapper struct to create JSON body of 'Create security group request'
type SpiderSecurityReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderSecurityInfo
}

// SpiderSecurityRuleReqInfoWrapper is a wrapper struct to create JSON body of 'Create security rule'
type SpiderSecurityRuleReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderSecurityRuleReqInfoIntermediateWrapper
}

// SpiderSecurityRuleReqInfoIntermediateWrapper is a intermediate wrapper struct between SpiderSecurityRuleReqInfoWrapper and SpiderSecurityRuleInfo.
type SpiderSecurityRuleReqInfoIntermediateWrapper struct {
	RuleInfoList []SpiderSecurityRuleInfo
}

// SpiderSecurityRuleInfo is a struct to handle security group rule info from/to CB-Spider.
type SpiderSecurityRuleInfo struct {
	FromPort   string //`json:"fromPort"`
	ToPort     string //`json:"toPort"`
	IPProtocol string //`json:"ipProtocol"`
	Direction  string //`json:"direction"`
	CIDR       string
}

// SpiderSecurityRuleInfo is a struct to create JSON body of 'Create security group request'
type SpiderSecurityInfo struct {
	// Fields for request
	Name    string
	VPCName string
	CSPId   string

	// Fields for both request and response
	SecurityRules []SpiderSecurityRuleInfo

	// Fields for response
	IId          IID    // {NameId, SystemId}
	VpcIID       IID    // {NameId, SystemId}
	Direction    string // @todo userd??
	KeyValueList []KeyValue
}

// SpiderSecurityInfoList is a struct to handle 'List security group' response from CB-Spider.
type SpiderSecurityInfoList struct {
	SecurityGroup []SpiderSecurityInfo
}

// SecurityGroupReq is a struct to handle 'Create security group' request toward CB-Tumblebug.
type SecurityGroupReq struct { // Tumblebug
	Name           string             `json:"name" validate:"required"`
	ConnectionName string             `json:"connectionName" validate:"required"`
	VNetId         string             `json:"vNetId"` // Optional for registration: some CSPs (e.g., Azure, Tencent, NHN) don't bind SG to VPC
	Description    string             `json:"description"`
	FirewallRules  *[]FirewallRuleReq `json:"firewallRules"` // validate:"required"`

	// CspResourceId is required to register object from CSP (option=register)
	CspResourceId string `json:"cspResourceId" example:"required for option=register only. ex: csp-06eb41e14121c550a"`
}

// FirewallRuleReq is a struct to get a request for firewall rule info of CB-Tumblebug.
type FirewallRuleReq struct {
	// Ports is to get multiple ports or port ranges as a string (e.g. "22,900-1000,2000-3000")
	// This allows flexibility in specifying single ports or ranges in a comma-separated format.
	// This field is used to handle both single ports and port ranges in a unified way.
	// It can accept a single port (e.g. "22"), a range (e.g. "900-1000"), or multiple ports/ranges (e.g. "22,900-1000,2000-3000").
	Ports string `json:"Ports" example:"22,900-1000,2000-3000"`
	// Protocol is the protocol type for the rule (TCP, UDP, ICMP). Don't use ALL here.
	Protocol string `validate:"required" json:"Protocol" example:"TCP" enums:"TCP,UDP,ICMP"`
	// Direction is the direction of the rule (inbound or outbound)
	Direction string `validate:"required" json:"Direction" example:"inbound" enums:"inbound,outbound"`
	// CIDR is the allowed IP range (e.g. 0.0.0.0/0, 10.0.0/8)
	CIDR string `json:"CIDR" example:"0.0.0.0/0"`
}

// FirewallRuleInfo is a struct to handle firewall rule info of CB-Tumblebug.
type FirewallRuleInfo struct {
	// Port is the single port (e.g. "22") or port range (e.g. "1-65535") for the rule
	Port string `json:"Port" example:"1-65535"`
	// Protocol is the protocol type for the rule (TCP, UDP, ICMP, ALL)
	Protocol string `validate:"required" json:"Protocol" example:"TCP" enums:"TCP,UDP,ICMP,ALL"`
	// Direction is the direction of the rule (inbound or outbound)
	Direction string `validate:"required" json:"Direction" example:"inbound" enums:"inbound,outbound"`
	// CIDR is the allowed IP range (e.g. 0.0.0.0/0, 10.0.0/8)
	CIDR string `json:"CIDR" example:"0.0.0.0/0"`
}

// SecurityGroupInfo is a struct that represents TB security group object.
type SecurityGroupInfo struct {
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

	VNetId               string             `json:"vNetId"`
	Description          string             `json:"description"`
	FirewallRules        []FirewallRuleInfo `json:"firewallRules"`
	KeyValueList         []KeyValue         `json:"keyValueList"`
	AssociatedObjectList []string           `json:"associatedObjectList"`
	IsAutoGenerated      bool               `json:"isAutoGenerated"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Disabled for now
	//ResourceGroupName  string `json:"resourceGroupName"`
}

// SecurityGroupUpdateResponse is a struct to handle 'Update security group' response toward CB-Tumblebug.
type SecurityGroupUpdateResponse struct {
	Id       string            `json:"id"`
	Name     string            `json:"name"`
	Success  bool              `json:"success"`
	Message  string            `json:"message,omitempty"`
	Updated  SecurityGroupInfo `json:"updated,omitempty"`
	Previous SecurityGroupInfo `json:"previous,omitempty"`
}

// RestWrapperSecurityGroupUpdateResponse is a struct to handle 'Update security group' response toward CB-Tumblebug.
type RestWrapperSecurityGroupUpdateResponse struct {
	Response []SecurityGroupUpdateResponse `json:"response"`
	Summary  UpdateSummary                 `json:"summary"`
}

// UpdateSummary provides overall summary of the update operation
type UpdateSummary struct {
	Total      int  `json:"total"`
	Success    int  `json:"success"`
	Failed     int  `json:"failed"`
	AllSuccess bool `json:"allSuccess"`
}

// SecurityGroupUpdateReq is a struct to handle 'Update security group' request toward CB-Tumblebug.
type SecurityGroupUpdateReq struct {
	FirewallRules []FirewallRuleReq `json:"firewallRules"`
}
