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
	Links       []string                `mapstructure:"link" json:"links"`
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

// Location is structure for location information
type Location struct {
	Display   string  `mapstructure:"display" json:"display"`
	Latitude  float64 `mapstructure:"latitude" json:"latitude"`
	Longitude float64 `mapstructure:"longitude" json:"longitude"`
}

type Credential struct {
	Credentialholder map[string]map[string]map[string]string `yaml:"credentialholder"`
}

// K8sClusterInfo is structure for kubernetes cluster information
type K8sClusterInfo struct {
	CSPs map[string]K8sClusterDetail `mapstructure:"k8scluster" json:"k8s_cluster"`
}

type K8sClusterNodeGroupsOnCreation struct {
	Result string `json:"result" example:"true"`
}

// K8sClusterDetail is structure for kubernetes cluster detail information
type K8sClusterDetail struct {
	NodeGroupsOnCreation bool                        `mapstructure:"nodeGroupsOnCreation" json:"nodegroups_on_creation"`
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
	Nhncloud  CloudSetting `yaml:"nhncloud"`
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
	NlbMciCommonSpec        string `yaml:"nlbMciCommonSpec"`
	NlbMciCommonImage       string `yaml:"nlbMciCommonImage"`
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
