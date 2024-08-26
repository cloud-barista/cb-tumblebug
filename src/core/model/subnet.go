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
	Name         string     `json:"name,omitempty" validate:"required"`
	IdFromCsp    string     `json:"idFromCsp,omitempty"`
	IPv4_CIDR    string     `json:"ipv4_CIDR" validate:"required"`
	KeyValueList []KeyValue `json:"keyValueList,omitempty"`
	Description  string     `json:"description,omitempty"`
	Zone         string     `json:"zone,omitempty"`
	TagList      []KeyValue `json:"tagList,omitempty"`
}

// TbSubnetInfo is a struct that represents TB subnet object.
type TbSubnetInfo struct { // Tumblebug
	Id   string
	Name string `validate:"required"`
	// uuid is universally unique identifier for the resource
	Uuid         string
	IdFromCsp    string
	IPv4_CIDR    string `validate:"required"`
	BastionNodes []BastionNode
	KeyValueList []KeyValue
	Description  string
}
