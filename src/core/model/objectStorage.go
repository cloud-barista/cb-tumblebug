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
	ID          string `json:"id" example:"aws-ap-northeast-2"`
	DisplayName string `json:"displayName" example:"aws-ap-northeast-2"`
}

// Buckets represents the collection of buckets in S3 bucket list response
type Buckets struct {
	Bucket []Bucket `json:"bucket"`
}

// Bucket represents a single bucket in S3 bucket list response
type Bucket struct {
	Name         string `json:"name" example:"spider-test-bucket"`
	CreationDate string `json:"creationDate" example:"2025-09-04T04:18:06Z"`
}

type Object struct {
	Key          string `json:"key" example:"test-object.txt"`
	LastModified string `json:"lastModified" example:"2025-09-04T04:18:06Z"`
	ETag         string `json:"eTag" example:"9b2cf535f27731c974343645a3985328"`
	Size         int64  `json:"size" example:"1024"`
	StorageClass string `json:"storageClass" example:"STANDARD"`
}

// ListBucketResponse represents the response structure for listing S3 buckets
type ListBucketResponse struct {
	Owner   Owner   `json:"owner"`
	Buckets Buckets `json:"buckets"`
}

type ObjectStorageCreateRequest struct {
	BucketName     string `json:"bucketName" validate:"required" example:"os01"`
	ConnectionName string `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
	Description    string `json:"description" example:"this bucket is managed by CB-Tumblebug"`
}

type ObjectStorageInfo struct {
	// ResourceType is the type of this resource
	ResourceType string `json:"resourceType" example:"ObjectStorage"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"globally-unique-bucket-name-12345"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty" example:""`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:""`

	// Variables for management of Object Storage resource in CB-Tumblebug
	ConnectionName   string     `json:"connectionName"`
	ConnectionConfig ConnConfig `json:"connectionConfig"`
	Description      string     `json:"description" example:"this object storage is managed by CB-Tumblebug"`
	Status           string     `json:"status"`

	// Name is human-readable string to represent the object
	Name         string   `json:"name" example:"globally-unique-bucket-name-12345"`
	Prefix       string   `json:"prefix,omitempty" example:""`
	Marker       string   `json:"marker,omitempty" example:""`
	MaxKeys      int      `json:"maxKeys,omitempty" example:"1000"`
	IsTruncated  bool     `json:"isTruncated,omitempty" example:"false"`
	CreationDate string   `json:"creationDate,omitempty" example:"2025-09-04T04:18:06Z"`
	Contents     []Object `json:"contents,omitempty"`
}

// ObjectStorageLocationResponse represents the response structure for object storage location
type ObjectStorageLocationResponse struct {
	LocationConstraint string `json:"locationConstraint" example:"ap-northeast-2"`
}

// PresignedUrlResponse represents the response structure for presigned URL generation
type PresignedUrlResponse struct {
	Expires      int64  `json:"expires" example:"1693824000"`
	Method       string `json:"method" example:"GET"`
	PreSignedURL string `json:"presignedURL" example:"https://example.com/presigned-url"`
}

// ListObjectResponse represents the response structure for listing objects in a bucket
type ListObjectResponse struct {
	Objects []Object `json:"objects"`
}
