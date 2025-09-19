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

// Package mci is to handle REST API for mci
package model

// Owner represents the owner information in S3 bucket list response
type Owner struct {
	ID          string `xml:"ID" json:"id"`
	DisplayName string `xml:"DisplayName" json:"displayName"`
}

// Bucket represents a single bucket in S3 bucket list response
type Bucket struct {
	Name         string `xml:"Name" json:"name"`
	CreationDate string `xml:"CreationDate" json:"creationDate"`
}

// Buckets represents the collection of buckets in S3 bucket list response
type Buckets struct {
	Bucket []Bucket `xml:"Bucket" json:"buckets"`
}

// type RestPostObjectStorageRequest struct {
// 	Name                string                              `json:"name" validate:"required" example:"objectstorage01"`
// 	ConnectionName      string                              `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
// 	CSP                 string                              `json:"csp" validate:"required" example:"aws"`
// 	Region              string                              `json:"region" validate:"required" example:"ap-northeast-2"`
// 	RequiredCSPResource RequiredCSPResourceForObjectStorage `json:"requiredCSPResource,omitempty"`
// }

// type RequiredCSPResourceForObjectStorage struct {
// 	// AWS   RequiredAWSResourceForObjectStorage   `json:"aws,omitempty"`
// 	Azure RequiredAzureResourceForObjectStorage `json:"azure,omitempty"`
// }

// // type RequiredAWSResourceForObjectStorage struct {
// // 	VNetID    string `json:"vNetID,omitempty" example:"vpc-xxxxx"`
// // 	Subnet1ID string `json:"subnet1ID,omitempty" example:"subnet-xxxx"`
// // 	Subnet2ID string `json:"subnet2ID,omitempty" example:"subnet-xxxx in different AZ"`
// // }

// type RequiredAzureResourceForObjectStorage struct {
// 	ResourceGroup string `json:"resourceGroup,omitempty" example:"koreacentral"`
// }

// type ObjectStorageInfo struct {
// 	// ResourceType is the type of the resource
// 	ResourceType     string     `json:"resourceType"`
// 	ConnectionName   string     `json:"connectionName"`
// 	ConnectionConfig ConnConfig `json:"connectionConfig"`
// 	// Id is unique identifier for the object
// 	Id string `json:"id" example:"sqldb01"`
// 	// Uid is universally unique identifier for the object, used for labelSelector
// 	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
// 	// Name is human-readable string to represent the object
// 	Name string `json:"name" example:"sqldb01"`
// 	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
// 	CspResourceName string `json:"cspResourceName,omitempty" example:"we12fawefadf1221edcf"`
// 	// CspResourceId is resource identifier managed by CSP
// 	CspResourceId string      `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`
// 	Status        string      `json:"status"`
// 	Description   string      `json:"description"`
// 	Details       interface{} `json:"details"`
// }
