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

// CloudInfo is structure for cloud information
type CloudInfo struct {
	CSPs map[string]CSPDetail `mapstructure:"cloud" json:"csps"`
}

// CSPDetail is structure for CSP information
type CSPDetail struct {
	Description string                  `mapstructure:"description" json:"description"`
	Driver      string                  `mapstructure:"driver" json:"driver"`
	Links       []string                `mapstructure:"link" json:"links,omitempty"`
	Regions     map[string]RegionDetail `mapstructure:"region" json:"regions"`
}

// RegionDetail is structure for region information
type RegionDetail struct {
	RegionId    string   `mapstructure:"id" json:"regionId"`
	RegionName  string   `mapstructure:"regionName" json:"regionName"`
	Description string   `mapstructure:"description" json:"description"`
	Location    Location `mapstructure:"location" json:"location"`
	Zones       []string `mapstructure:"zone" json:"zones"`
}

// RegionList is structure for region list
type RegionList struct {
	Regions []RegionDetail `mapstructure:"regions" json:"regions"`
}

// Location is structure for location information
type Location struct {
	Display   string  `mapstructure:"display" json:"display"`
	Latitude  float64 `mapstructure:"latitude" json:"latitude"`
	Longitude float64 `mapstructure:"longitude" json:"longitude"`
}

type Credential struct {
	Credentialholder map[string]map[string]map[string]string `yaml:"credentialholder"`
}

// ExtractPatternsInfo is structure for extraction patterns information
type ExtractPatternsInfo struct {
	ExtractPatterns ExtractPatterns `mapstructure:"extractionPatterns" json:"extraction_patterns"`
}

// ExtractPatterns is structure for extraction patterns
type ExtractPatterns struct {
	OSType      map[string]OSTypeDetail `mapstructure:"osType" json:"os_type"`
	GPUPatterns []string                `mapstructure:"gpuPatterns" json:"gpu_patterns"`
	K8sPatterns []string                `mapstructure:"k8sPatterns" json:"k8s_patterns"`
}

// OSTypeDetail is structure for OS type detail information
type OSTypeDetail struct {
	Name            string           `mapstructure:"name" json:"name"`
	Versions        []string         `mapstructure:"versions" json:"versions"`
	DefaultVersion  string           `mapstructure:"defaultVersion" json:"default_version"`
	Patterns        []string         `mapstructure:"patterns" json:"patterns"`
	BasicImageRules *BasicImageRules `mapstructure:"basicImageRules" json:"basicImageRules,omitempty"`
}

// BasicImageRules defines rules for identifying basic OS images
// Basic images are clean, official OS installations without additional software or customization
type BasicImageRules struct {
	Common      PatternSet            `mapstructure:"common" json:"common"`
	CspSpecific map[string]PatternSet `mapstructure:"cspSpecific" json:"cspSpecific,omitempty"`
}

// PatternSet defines include and exclude patterns for basic image detection
type PatternSet struct {
	Include []string `mapstructure:"include" json:"include,omitempty"`
	Exclude []string `mapstructure:"exclude" json:"exclude,omitempty"`
}

/*
 * NetworkInfo
 */

// NetworkInfo is structure for network information
type CloudNetworkInfo struct {
	CSPs map[string]CSPNetworkDetail `mapstructure:"network" json:"csps"`
}

// CSPNetworkDetail is structure for CSP network information
type CSPNetworkDetail struct {
	Description         string            `mapstructure:"description" json:"description"`
	Links               []string          `mapstructure:"link" json:"links,omitempty"`
	AvailableCIDRBlocks []CIDRBlockDetail `mapstructure:"available-cidr-blocks" json:"availableCidrBlocks"`
	ReservedCIDRBlocks  []CIDRBlockDetail `mapstructure:"reserved-cidr-blocks" json:"reservedCidrBlocks,omitempty"`
	VNet                *VNetDetail       `mapstructure:"vnet" json:"vnet,omitempty"`
	Subnet              *SubnetDetail     `mapstructure:"subnet" json:"subnet,omitempty"`
	VPN                 *VPNDetail        `mapstructure:"vpn" json:"vpn,omitempty"`
}

// CIDRBlockDetail is structure for IP range information
type CIDRBlockDetail struct {
	CIDRBlock   string `mapstructure:"cidr-block" json:"cidrBlock"`
	Description string `mapstructure:"description" json:"description"`
}

// VNetDetail is structure for virtual network configuration
type VNetDetail struct {
	PrefixLength PrefixLengthDetail `mapstructure:"prefix-length" json:"prefixLength"`
}

// SubnetDetail is structure for subnet configuration
type SubnetDetail struct {
	PrefixLength PrefixLengthDetail `mapstructure:"prefix-length" json:"prefixLength"`
	ReservedIPs  ReservedIPsDetail  `mapstructure:"reserved-ips" json:"reservedIPs"`
}

// PrefixLengthDetail is structure for prefix length configuration
type PrefixLengthDetail struct {
	Min         int    `mapstructure:"min" json:"min,omitempty"`
	Max         int    `mapstructure:"max" json:"max,omitempty"`
	Description string `mapstructure:"description" json:"description"`
}

// ReservedIPsDetail is structure for reserved IPs configuration
type ReservedIPsDetail struct {
	Value       int    `mapstructure:"value" json:"value"`
	Description string `mapstructure:"description" json:"description"`
}

// VPNDetail is structure for VPN configuration
type VPNDetail struct {
	GatewaySubnet GatewaySubnetDetail `mapstructure:"gateway-subnet" json:"gatewaySubnet"`
}

// GatewaySubnetDetail is structure for gateway subnet configuration
type GatewaySubnetDetail struct {
	Required     bool               `mapstructure:"required" json:"required"`
	Name         string             `mapstructure:"name" json:"name"`
	Description  string             `mapstructure:"description" json:"description"`
	PrefixLength PrefixLengthDetail `mapstructure:"prefix-length" json:"prefixLength"`
}

/*
 * K8sClusterAssetInfo
 */

// K8sClusterAssetInfo is structure for kubernetes cluster information
type K8sClusterAssetInfo struct {
	CSPs map[string]K8sClusterDetail `mapstructure:"k8scluster" json:"k8s_cluster"`
}

type K8sClusterNodeGroupsOnCreation struct {
	Result string `json:"result" example:"true"`
}

type K8sClusterNodeImageDesignation struct {
	Result string `json:"result" example:"true"`
}

type K8sClusterRequiredSubnetCount struct {
	Result string `json:"result" example:"1"`
}

// K8sClusterDetail is structure for kubernetes cluster detail information
type K8sClusterDetail struct {
	NodeGroupsOnCreation bool                        `mapstructure:"nodeGroupsOnCreation" json:"nodegroups_on_creation"`
	NodeImageDesignation bool                        `mapstructure:"nodeImageDesignation" json:"node_image_designation"`
	RequiredSubnetCount  int                         `mapstructure:"requiredSubnetCount" json:"required_subnet_count"`
	NodeGroupNamingRule  string                      `mapstructure:"nodeGroupNamingRule" json:"nodegroup_naming_rule"`
	Version              []K8sClusterVersionDetail   `mapstructure:"version" json:"versions"`
	NodeImage            []K8sClusterNodeImageDetail `mapstructure:"nodeImage" json:"node_images"`
	RootDisk             []K8sClusterRootDiskDetail  `mapstructure:"rootDisk" json:"root_disks"`
}

// K8sClusterVersionDetail is structure for kubernetes cluster version detail information
type K8sClusterVersionDetail struct {
	Region    []string                           `mapstructure:"region" json:"region"`
	Available []K8sClusterVersionDetailAvailable `mapstructure:"available" json:"availables"`
}

// K8sClusterVersionDetailAvailable is structure for kubernetes cluster version detail's available information
type K8sClusterVersionDetailAvailable struct {
	Name string `mapstructure:"name" json:"name" example:"1.30"`
	Id   string `mapstructure:"id" json:"id" example:"1.30.1-aliyun.1"`
}

// K8sClusterNodeImageDetail is structure for kubernetes cluster node image detail information
type K8sClusterNodeImageDetail struct {
	Region    []string                             `mapstructure:"region" json:"region"`
	Available []K8sClusterNodeImageDetailAvailable `mapstructure:"available" json:"availables"`
}

// K8sClusterNodeImageDetailAvailable is structure for kubernetes cluster node image detail's available information
type K8sClusterNodeImageDetailAvailable struct {
	Name string `mapstructure:"name" json:"name"`
	Id   string `mapstructure:"id" json:"id"`
}

// K8sClusterRootDiskDetail is structure for kubernetes cluster root disk detail information
type K8sClusterRootDiskDetail struct {
	Region []string                       `mapstructure:"region" json:"region"`
	Type   []K8sClusterRootDiskDetailType `mapstructure:"type" json:"type"`
	Size   K8sClusterRootDiskDetailSize   `mapstructure:"size" json:"size"`
}

// K8sClusterRootDiskDetailType is structure for kubernetes cluster root disk detail's type information
type K8sClusterRootDiskDetailType struct {
	Name string `mapstructure:"name" json:"name"`
	Id   string `mapstructure:"id" json:"id"`
}

// K8sClusterRootDiskDetailSize is structure for kubernetes cluster root disk detail's size information
type K8sClusterRootDiskDetailSize struct {
	Min uint `mapstructure:"min" json:"min"`
	Max uint `mapstructure:"max" json:"max"`
}

// RuntimeConfig is structure for global variable for cloud config
type RuntimeConfig struct {
	Cloud Cloud `yaml:"cloud"`
	Nlbsw Nlbsw `yaml:"nlbsw"`
}

// Cloud is structure for cloud settings per CSP
type Cloud struct {
	Common    CloudSetting `yaml:"common"`
	Aws       CloudSetting `yaml:"aws"`
	Azure     CloudSetting `yaml:"azure"`
	Gcp       CloudSetting `yaml:"gcp"`
	Alibaba   CloudSetting `yaml:"alibaba"`
	Tencent   CloudSetting `yaml:"tencent"`
	Ibm       CloudSetting `yaml:"ibm"`
	Ncp       CloudSetting `yaml:"ncp"`
	NHN       CloudSetting `yaml:"nhn"`
	Openstack CloudSetting `yaml:"openstack"`
	Cloudit   CloudSetting `yaml:"cloudit"`
}

// CloudSetting is structure for cloud settings per CSP in details
type CloudSetting struct {
	Enable     string            `yaml:"enable"`
	Nlb        NlbSetting        `yaml:"nlb"`
	K8sCluster K8sClusterSetting `yaml:"k8scluster"`
}

// NlbSetting is structure for NLB setting
type NlbSetting struct {
	Enable    string `yaml:"enable"`
	Interval  string `yaml:"interval"`
	Timeout   string `yaml:"timeout"`
	Threshold string `yaml:"threshold"`
}

// Nlbsw is structure for NLB setting
type Nlbsw struct {
	Sw                      string `yaml:"sw"`
	Version                 string `yaml:"version"`
	CommandNlbPrepare       string `yaml:"commandNlbPrepare"`
	CommandNlbDeploy        string `yaml:"commandNlbDeploy"`
	CommandNlbAddTargetNode string `yaml:"commandNlbAddTargetNode"`
	CommandNlbApplyConfig   string `yaml:"commandNlbApplyConfig"`
	NlbMciSpecId            string `yaml:"nlbMciSpecId"`
	NlbMciImageId           string `yaml:"nlbMciImageId"`
	NlbMciSubGroupSize      string `yaml:"nlbMciSubGroupSize"`
}

// K8sClusterSetting is structure for K8sCluster setting
type K8sClusterSetting struct {
	Enable string `yaml:"enable"`
}

// type DataDiskCmd string
const (
	AttachDataDisk    string = "attach"
	DetachDataDisk    string = "detach"
	AvailableDataDisk string = "available"
)

// swagger:request ConfigReq
type ConfigReq struct {
	Name  string `json:"name" example:"TB_SPIDER_REST_URL"`
	Value string `json:"value" example:"http://localhost:1024/spider"`
}

// swagger:response ConfigInfo
type ConfigInfo struct {
	Id    string `json:"id" example:"TB_SPIDER_REST_URL"`
	Name  string `json:"name" example:"TB_SPIDER_REST_URL"`
	Value string `json:"value" example:"http://localhost:1024/spider"`
}
