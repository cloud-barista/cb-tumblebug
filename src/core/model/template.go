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

// [Template for MCI Dynamic Provisioning]

// MciDynamicTemplateInfo is struct for MCI Dynamic Template information stored in ETCD
type MciDynamicTemplateInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType" example:"mci"`

	// Id is unique identifier for the template
	Id string `json:"id" example:"my-template"`

	// Name is human-readable string to represent the template
	Name string `json:"name" example:"my-template"`

	// Description of the template
	Description string `json:"description" example:"3-tier web application template"`

	// Source indicates where this template was created from
	// - "user": manually created by user
	// - "mci:{nsId}/{mciId}": extracted from an existing MCI
	Source string `json:"source" example:"user"`

	// CreatedAt is the creation timestamp
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`

	// UpdatedAt is the last update timestamp
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`

	// MciDynamicReq is the template body (MCI dynamic request)
	MciDynamicReq MciDynamicReq `json:"mciDynamicReq"`
}

// MciDynamicTemplateReq is struct for creating/updating an MCI Dynamic Template
type MciDynamicTemplateReq struct {
	// Name is the template ID and name
	Name string `json:"name" validate:"required" example:"my-template"`

	// Description of the template
	Description string `json:"description" example:"3-tier web application template"`

	// MciDynamicReq is the template body (MCI dynamic request configuration)
	MciDynamicReq MciDynamicReq `json:"mciDynamicReq" validate:"required"`
}

// TemplateApplyReq is struct for applying a template to create an MCI
// Phase 1: Only name and description overrides are supported
type TemplateApplyReq struct {
	// Name for the new MCI to be created from the template
	Name string `json:"name" validate:"required" example:"my-new-mci"`

	// Description for the new MCI (optional, overrides template description)
	Description string `json:"description" example:"MCI created from template"`
}

// MciDynamicTemplateListResponse is struct for listing MCI Dynamic Templates
type MciDynamicTemplateListResponse struct {
	Templates []MciDynamicTemplateInfo `json:"templates"`
}

// [Template for vNet Resource]

// VNetPolicy defines a CSP-agnostic intent for VNet creation.
// The policy is converted to a CSP-specific VNetReq at provisioning time,
// respecting per-CSP constraints automatically (e.g. IBM single-subnet, NCP same-zone).
type VNetPolicy struct {
	// CidrBlock for the VPC. Use "auto" to assign a unique /16 block automatically
	// (based on connection index: 10.{i}.0.0/16), or specify an explicit CIDR.
	// For CSPs that do not support VPC-level CIDR (e.g. GCP), this field is ignored.
	CidrBlock string `json:"cidrBlock" example:"auto"`

	// SubnetCount is the desired number of subnets.
	// CSP-specific caps apply automatically:
	//   IBM  → always capped to 1 (VPC/subnet architecture limitation)
	//   Others → up to 2 subnets are supported; values > 2 are capped to 2
	SubnetCount int `json:"subnetCount" example:"2"`

	// MultiZone requests that subnets be spread across different availability zones
	// when the region has more than one zone.
	// Set to false to place all subnets in the same zone (required for some workloads).
	// NCP → always forced to false (all subnets must reside in the same zone).
	MultiZone bool `json:"multiZone" example:"true"`
}

// VNetTemplateInfo is struct for vNet Template information stored in ETCD
type VNetTemplateInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType" example:"vNet"`

	// Id is unique identifier for the template
	Id string `json:"id" example:"my-vnet-template"`

	// Name is human-readable string to represent the template
	Name string `json:"name" example:"my-vnet-template"`

	// Description of the template
	Description string `json:"description" example:"Standard 3-subnet VPC template"`

	// Source indicates where this template was created from
	// - "user": manually created by user
	Source string `json:"source" example:"user"`

	// CreatedAt is the creation timestamp
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`

	// UpdatedAt is the last update timestamp
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`

	// VNetPolicy is a CSP-agnostic VNet intent (policy mode).
	// Mutually exclusive with VNetReq.
	// Used for dynamic provisioning where CSP-specific details are auto-resolved.
	VNetPolicy *VNetPolicy `json:"vNetPolicy,omitempty"`

	// VNetReq is the raw VNet creation request (raw mode).
	// Mutually exclusive with VNetPolicy.
	// Used when precise control over CIDR and subnet layout is required.
	VNetReq *VNetReq `json:"vNetReq,omitempty"`
}

// VNetTemplateReq is struct for creating/updating a vNet Template
type VNetTemplateReq struct {
	// Name is the template ID and name
	Name string `json:"name" validate:"required" example:"my-vnet-template"`

	// Description of the template
	Description string `json:"description" example:"Standard 3-subnet VPC template"`

	// VNetPolicy is a CSP-agnostic VNet intent (policy mode).
	// Mutually exclusive with VNetReq. Exactly one must be provided.
	VNetPolicy *VNetPolicy `json:"vNetPolicy,omitempty"`

	// VNetReq is the raw VNet creation request (raw mode).
	// Mutually exclusive with VNetPolicy. Exactly one must be provided.
	VNetReq *VNetReq `json:"vNetReq,omitempty"`
}

// VNetTemplateApplyReq is struct for applying a vNet template to create a vNet
// Phase 1: Only name and description overrides are supported
type VNetTemplateApplyReq struct {
	// Name for the new vNet to be created from the template
	Name string `json:"name" validate:"required" example:"my-new-vnet"`

	// Description for the new vNet (optional, overrides template description)
	Description string `json:"description" example:"vNet created from template"`
}

// VNetTemplateListResponse is struct for listing vNet Templates
type VNetTemplateListResponse struct {
	Templates []VNetTemplateInfo `json:"templates"`
}

// [Template for SecurityGroup Resource]

// SecurityGroupTemplateInfo is struct for SecurityGroup Template information stored in ETCD
type SecurityGroupTemplateInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType" example:"securityGroup"`

	// Id is unique identifier for the template
	Id string `json:"id" example:"my-sg-template"`

	// Name is human-readable string to represent the template
	Name string `json:"name" example:"my-sg-template"`

	// Description of the template
	Description string `json:"description" example:"Standard web server security group template"`

	// Source indicates where this template was created from
	// - "user": manually created by user
	Source string `json:"source" example:"user"`

	// CreatedAt is the creation timestamp
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`

	// UpdatedAt is the last update timestamp
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`

	// SecurityGroupReq is the template body (SecurityGroup creation request)
	SecurityGroupReq SecurityGroupReq `json:"securityGroupReq"`
}

// SecurityGroupTemplateReq is struct for creating/updating a SecurityGroup Template
type SecurityGroupTemplateReq struct {
	// Name is the template ID and name
	Name string `json:"name" validate:"required" example:"my-sg-template"`

	// Description of the template
	Description string `json:"description" example:"Standard web server security group template"`

	// SecurityGroupReq is the template body (SecurityGroup creation request configuration)
	SecurityGroupReq SecurityGroupReq `json:"securityGroupReq" validate:"required"`
}

// SecurityGroupTemplateApplyReq is struct for applying a SecurityGroup template
// Phase 1: Only name and description overrides are supported
type SecurityGroupTemplateApplyReq struct {
	// Name for the new SecurityGroup to be created from the template
	Name string `json:"name" validate:"required" example:"my-new-sg"`

	// Description for the new SecurityGroup (optional, overrides template description)
	Description string `json:"description" example:"SecurityGroup created from template"`
}

// SecurityGroupTemplateListResponse is struct for listing SecurityGroup Templates
type SecurityGroupTemplateListResponse struct {
	Templates []SecurityGroupTemplateInfo `json:"templates"`
}
