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

	// VNetReq is the template body (vNet creation request)
	VNetReq VNetReq `json:"vNetReq"`
}

// VNetTemplateReq is struct for creating/updating a vNet Template
type VNetTemplateReq struct {
	// Name is the template ID and name
	Name string `json:"name" validate:"required" example:"my-vnet-template"`

	// Description of the template
	Description string `json:"description" example:"Standard 3-subnet VPC template"`

	// VNetReq is the template body (vNet creation request configuration)
	VNetReq VNetReq `json:"vNetReq" validate:"required"`
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
