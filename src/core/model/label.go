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

// LabelableResource is an interface for resources that support labels.
type LabelableResource interface {
	GetLabels() map[string]string
	SetLabels(labels map[string]string)
}

// LabelInfo represents the label-related information for a resource.
type LabelInfo struct {
	ResourceKey string            `json:"resourceKey"`
	Labels      map[string]string `json:"labels"`
}

// Label is a struct to handle labels
type Label struct {
	Labels map[string]string `json:"labels"`
}

// SystemLabelInfo is a struct to return LabelTypes and System label Keys
type SystemLabelInfo struct {
	LabelTypes   []string          `json:"labelTypes"`
	SystemLabels map[string]string `json:"systemLabels"`
}

const (
	LabelManager         string = "sys.manager"
	LabelNamespace       string = "sys.namespace"
	LabelLabelType       string = "sys.labelType"
	LabelId              string = "sys.id"
	LabelName            string = "sys.name"
	LabelUid             string = "sys.uid"
	LabelCspResourceId   string = "sys.cspResourceId"
	LabelCspResourceName string = "sys.cspResourceName"
	LabelMciId           string = "sys.mciId"
	LabelMciName         string = "sys.mciName"
	LabelMciUid          string = "sys.mciUid"
	LabelMciDescription  string = "sys.mciDescription"
	LabelSubGroupId      string = "sys.subGroupId"
	LabelCreatedTime     string = "sys.createdTime"
	LabelConnectionName  string = "sys.connectionName"
	LabelDescription     string = "sys.description"
	LabelRegistered      string = "sys.registered"
	LabelPurpose         string = "sys.purpose"
	LabelDeploymentType  string = "sys.deploymentType"
	LabelDiskType        string = "sys.diskType"
	LabelDiskSize        string = "sys.diskSize"
	LabelVersion         string = "sys.version"
	LabelVNetId          string = "sys.vNetId"
	LabelIpv4_CIDR       string = "sys.ipv4_CIDR"
	LabelZone            string = "sys.zone"
	LabelStatus          string = "sys.status"
	LabelCspVNetId       string = "sys.cspVNetId"
	LabelCspVNetName     string = "sys.cspVNetName"
	LabelCidr            string = "sys.cidr"
	LabelSubnetId        string = "sys.subnetId"
)

// GetLabelConstantsMap returns a map with label-related system constants as keys and their example values.
func GetLabelConstantsMap() map[string]string {
	return map[string]string{
		LabelManager:         "cb-tumblebug",
		LabelNamespace:       "default",
		LabelLabelType:       StrMCI,
		LabelId:              "mci-1234",
		LabelName:            "mci-1234",
		LabelUid:             "wef12awefadf1221edcf",
		LabelCspResourceId:   "csp-vm-1234",
		LabelCspResourceName: "csp-vm-1234",
		LabelMciId:           "mci-1234",
		LabelSubGroupId:      "sg-1234",
		LabelCreatedTime:     "2021-01-01T00:00:00Z",
		LabelConnectionName:  "connection-1234",
		LabelDescription:     "Description",
		LabelRegistered:      "true",
		LabelPurpose:         "testing",
		LabelDeploymentType:  "vm",
		LabelDiskType:        "HDD",
		LabelDiskSize:        "10",
		LabelVersion:         "1.0",
		LabelVNetId:          "vnet-1234",
		LabelIpv4_CIDR:       "10.0.0.0/24",
		LabelZone:            "zone-1",
		LabelStatus:          "Running",
		LabelCspVNetId:       "csp-vnet-1234",
		LabelCspVNetName:     "csp-vnet-1234",
		LabelCidr:            "10.0.0.0/24",
	}
}

// GetLabelTypes returns a list of label types.
func GetLabelTypes() []string {
	return []string{
		StrVNet,
		StrSubnet,
		StrDataDisk,
		StrNLB,
		StrVM,
		StrMCI,
		StrSubGroup,
		StrK8s,
		StrKubernetes,
		StrContainer,
		StrNamespace,
	}
}

// CB-Spider models

type SpiderTagAddRequest struct {
	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        struct {
		ResourceType string   `json:"ResourceType" validate:"required" example:"VPC"`
		ResourceName string   `json:"ResourceName" validate:"required" example:"vpc-01"`
		Tag          KeyValue `json:"Tag" validate:"required"`
	} `json:"ReqInfo" validate:"required"`
}

type SpiderTagRemoveRequest struct {
	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        struct {
		ResourceType string `json:"ResourceType" validate:"required" example:"VPC"`
		ResourceName string `json:"ResourceName" validate:"required" example:"vpc-01"`
	} `json:"ReqInfo" validate:"required"`
}
