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

// [Template for Infra Dynamic Provisioning]

// InfraDynamicTemplateInfo is struct for Infra Dynamic Template information stored in ETCD
type InfraDynamicTemplateInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType" example:"infra"`

	// Id is unique identifier for the template
	Id string `json:"id" example:"my-template"`

	// Name is human-readable string to represent the template
	Name string `json:"name" example:"my-template"`

	// Description of the template
	Description string `json:"description" example:"3-tier web application template"`

	// Source indicates where this template was created from
	// - "user": manually created by user
	// - "infra:{nsId}/{infraId}": extracted from an existing Infra
	Source string `json:"source" example:"user"`

	// CreatedAt is the creation timestamp
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`

	// UpdatedAt is the last update timestamp
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`

	// InfraDynamicReq is the template body (Infra dynamic request)
	InfraDynamicReq InfraDynamicReq `json:"infraDynamicReq"`
}

// InfraDynamicTemplateReq is struct for creating/updating an Infra Dynamic Template
type InfraDynamicTemplateReq struct {
	// Name is the template ID and name
	Name string `json:"name" validate:"required" example:"my-template"`

	// Description of the template
	Description string `json:"description" example:"3-tier web application template"`

	// InfraDynamicReq is the template body (Infra dynamic request configuration)
	InfraDynamicReq InfraDynamicReq `json:"infraDynamicReq" validate:"required"`
}

// TemplateApplyReq is struct for applying a template to create an Infra
// Phase 1: Only name and description overrides are supported
type TemplateApplyReq struct {
	// Name for the new Infra to be created from the template
	Name string `json:"name" validate:"required" example:"my-new-infra"`

	// Description for the new Infra (optional, overrides template description)
	Description string `json:"description" example:"Infra created from template"`
}

// InfraDynamicTemplateListResponse is struct for listing Infra Dynamic Templates
type InfraDynamicTemplateListResponse struct {
	Templates []InfraDynamicTemplateInfo `json:"templates"`
}

// [Template for vNet Resource]

// VNetPolicy defines a CSP-agnostic intent for VNet creation.
// The policy is converted to a CSP-specific VNetReq at provisioning time,
// respecting per-CSP constraints automatically (e.g. IBM single-subnet, NCP same-zone).
type VNetPolicy struct {
	// CidrBlock for the VPC. Use "auto" to assign a unique /16 block automatically
	// (based on connection index: 10.{i}.0.0/16), or specify an explicit CIDR.
	// Ignored for CSPs that have no VPC-level CIDR (e.g. GCP, whose subnets carry their own
	// CIDRs and whose VPC rejects a CIDR): no CIDR is assigned to the vNet for those CSPs.
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

	// Dedicated controls the VNet isolation model for dynamic provisioning.
	//   false (default) → shared VNet per connection ("{ns}-shared-{conn}"), reused across
	//                     Infras. Preferred because VPC/VNet is a scarce CSP resource.
	//   true            → dedicated VNet per Infra ("{infraId}-{conn}"), isolating each
	//                     Infra's network. Use only when isolation is required; dedicated
	//                     VNets consume the (limited) per-region VPC quota faster.
	Dedicated bool `json:"dedicated,omitempty" example:"false"`
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

// [Template for K8s Multi-Cluster Dynamic Provisioning]

// K8sClusterDynamicTemplateInfo is struct for K8s Cluster Dynamic Template information stored in ETCD
type K8sClusterDynamicTemplateInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType" example:"k8sCluster"`

	// Id is unique identifier for the template
	Id string `json:"id" example:"k8s-across-global"`

	// Name is human-readable string to represent the template
	Name string `json:"name" example:"k8s-across-global"`

	// Description of the template
	Description string `json:"description" example:"Multi-cloud K8s cluster template"`

	// Source indicates where this template was created from
	// - "user": manually created by user
	// - "k8sCluster:{nsId}/{k8sClusterId}": extracted from an existing K8sCluster
	Source string `json:"source" example:"user"`

	// CreatedAt is the creation timestamp
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`

	// UpdatedAt is the last update timestamp
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`

	// K8sMultiClusterDynamicReq is the template body (K8s multi-cluster dynamic request)
	K8sMultiClusterDynamicReq K8sMultiClusterDynamicReq `json:"k8sMultiClusterDynamicReq"`
}

// K8sClusterDynamicTemplateReq is struct for creating/updating a K8s Cluster Dynamic Template
type K8sClusterDynamicTemplateReq struct {
	// Name is the template ID and name
	Name string `json:"name" validate:"required" example:"k8s-across-global"`

	// Description of the template
	Description string `json:"description" example:"Multi-cloud K8s cluster template"`

	// K8sMultiClusterDynamicReq is the template body (K8s multi-cluster dynamic request configuration)
	K8sMultiClusterDynamicReq K8sMultiClusterDynamicReq `json:"k8sMultiClusterDynamicReq" validate:"required"`
}

// K8sClusterTemplateApplyReq is struct for applying a K8s Cluster template to provision multi-cluster
// Phase 1: Only namePrefix and description overrides are supported
type K8sClusterTemplateApplyReq struct {
	// NamePrefix for the new K8s clusters to be created from the template (maps to K8sMultiClusterDynamicReq.NamePrefix)
	NamePrefix string `json:"namePrefix" validate:"required" example:"my-k8s"`

	// Description for the new K8s clusters (optional, propagated to all clusters)
	Description string `json:"description" example:"K8s clusters created from template"`
}

// K8sClusterDynamicTemplateListResponse is struct for listing K8s Cluster Dynamic Templates
type K8sClusterDynamicTemplateListResponse struct {
	Templates []K8sClusterDynamicTemplateInfo `json:"templates"`
}
