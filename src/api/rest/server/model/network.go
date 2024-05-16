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

// Package mcis is to handle REST API for mcis
package model

// type ResponseText struct {
// 	Success bool   `json:"success" example:"true"`
// 	Text    string `json:"text" example:"Any text"`
// }

// type ResponseList struct {
// 	Success bool          `json:"success" example:"true"`
// 	List    []interface{} `json:"list"`
// }

//	type ResponseObject struct {
//		Success bool                   `json:"success" example:"true"`
//		Object  map[string]interface{} `json:"object"`
//	}
type Response struct {
	Success bool                   `json:"success" example:"true"`
	Text    string                 `json:"text" example:"Any text"`
	Detail  string                 `json:"details,omitempty" example:"Any details"`
	Object  map[string]interface{} `json:"object,omitempty"`
	List    []interface{}          `json:"list,omitempty"`
}

type TfVarsGcpAwsVpnTunnel struct {
	ResourceGroupId   string `json:"resource-group-id,omitempty" default:"" example:""`
	AwsRegion         string `json:"aws-region" validate:"required" default:"ap-northeast-2" example:"ap-northeast-2"`
	AwsVpcId          string `json:"aws-vpc-id" validate:"required" example:"vpc-xxxxx"`
	AwsSubnetId       string `json:"aws-subnet-id" validate:"required" example:"subnet-xxxxx"`
	GcpRegion         string `json:"gcp-region" validate:"required" default:"asia-northeast3" example:"asia-northeast3"`
	GcpVpcNetworkName string `json:"gcp-vpc-network-name" validate:"required" default:"vpc01" example:"vpc01"`
	// GcpBgpAsn                   string `json:"gcp-bgp-asn" default:"65530"`
}

type RestPostVpnGcpToAwsRequest struct {
	TfVars TfVarsGcpAwsVpnTunnel `json:"tfVars"`
}

// type TfVarsGcpAzureVpnTunnel struct {
// 	ResourceGroupId             string `json:"resource-group-id,omitempty" default:"" example:""`
// 	AzureRegion                 string `json:"azure-region" default:"koreacentral" example:"koreacentral"`
// 	AzureResourceGroupName      string `json:"azure-resource-group-name" default:"tofu-rg-01" example:"tofu-rg-01"`
// 	AzureVirtualNetworkName     string `json:"azure-virtual-network-name" default:"tofu-azure-vnet" example:"tofu-azure-vnet"`
// 	AzureGatewaySubnetCidrBlock string `json:"azure-gateway-subnet-cidr-block" default:"192.168.130.0/24" example:"192.168.130.0/24"`
// 	GcpRegion                   string `json:"gcp-region" default:"asia-northeast3" example:"asia-northeast3"`
// 	GcpVpcNetworkName           string `json:"gcp-vpc-network-name" default:"tofu-gcp-vpc" example:"tofu-gcp-vpc"`
// 	// AzureBgpAsn				 	string `json:"azure-bgp-asn" default:"65515"`
// 	// GcpBgpAsn                   string `json:"gcp-bgp-asn" default:"65534"`
// 	// AzureSubnetName             string `json:"azure-subnet-name" default:"tofu-azure-subnet-0"`
// 	// GcpVpcSubnetworkName    string `json:"gcp-vpc-subnetwork-name" default:"tofu-gcp-subnet-1"`
// }
