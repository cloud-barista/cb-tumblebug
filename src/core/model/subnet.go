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

// TbSubnetReq is a struct that represents TB subnet object.
type TbSubnetReq struct { // Tumblebug
	Name        string `json:"name" validate:"required" example:"subnet00"`
	IPv4_CIDR   string `json:"ipv4_CIDR" validate:"required" example:"10.0.1.0/24"`
	Zone        string `json:"zone,omitempty"`
	Description string `json:"description,omitempty" example:"subnet00 managed by CB-Tumblebug"`
	// todo: restore the tag list later
	// TagList     []KeyValue `json:"tagList,omitempty"`
}

type TbRegisterSubnetReq struct {
	ConnectionName string `json:"connectionName" validate:"required"`
	CspResourceId  string `json:"cspResourceId" validate:"required"`
	Name           string `json:"name" validate:"required"`
	Zone           string `json:"zone,omitempty"`
	Description    string `json:"description,omitempty"`
}

// TbSubnetInfo is a struct that represents TB subnet object.
type TbSubnetInfo struct { // Tumblebug
	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name           string `json:"name" example:"aws-ap-southeast-1"`
	ConnectionName string `json:"connectionName"`
	// CspVNetHandlingId is identifier to handle CSP vNet resource
	CspVNetHandlingId string `json:"cspVNetHandlingId,omitempty" example:"we12fawefadf1221edcf"`
	// CspVNetId is vNet resource identifier managed by CSP
	CspVNetId    string        `json:"cspResourceId,omitempty" example:"csp-45eb41e14121c550a"`
	Status       string        `json:"status"`
	IPv4_CIDR    string        `json:"ipv4_CIDR"`
	Zone         string        `json:"zone,omitempty"`
	BastionNodes []BastionNode `json:"bastionNodes,omitempty"`
	KeyValueList []KeyValue    `json:"keyValueList,omitempty"`
	Description  string        `json:"description"`
	// todo: restore the tag list later
	// TagList        []KeyValue    `json:"tagList,omitempty"`
}
