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

import (
	"time"
)

type CustomImageStatus string

const (
	MyImageAvailable   CustomImageStatus = "Available"
	MyImageUnavailable CustomImageStatus = "Unavailable"
)

// SnapshotReq is a struct to handle 'Create Node snapshot' request toward CB-Tumblebug.
type SnapshotReq struct {
	Name        string `json:"name" example:"custom-image01" validate:"required"`
	Description string `json:"description" example:"Description about this custom image"`
}

type SpiderMyImageReq struct {
	ConnectionName string
	ReqInfo        struct {
		Name     string
		SourceNode string
	}
}

type SpiderMyImageInfo struct {
	IId IID // {NameId, SystemId}

	SourceNode IID

	Status CustomImageStatus // Available | Deleting

	CreatedTime  time.Time
	KeyValueList []KeyValue
}

type SpiderMyImageRegisterReq struct {
	ConnectionName string
	ReqInfo        struct {
		Name  string
		CSPId string // or, CSPid ?
	}
}

// CustomImageReq is a struct to handle a request for Create custom image (Node snapshot)
type CustomImageReq struct {
	// This field is for 'Register existing custom image'
	CspResourceId string `json:"cspResourceId"`

	ConnectionName string `json:"connectionName"`
	Name           string `json:"name" validate:"required"`
	SourceNodeId     string `json:"sourceNodeId"`
	Description    string `json:"description"`
}

// NodeSnapshotResult represents the result of creating a snapshot for a single Node
type NodeSnapshotResult struct {
	NodeGroupId string    `json:"nodeGroupId" example:"g1"`
	NodeId        string    `json:"nodeId" example:"g1-1"`
	NodeName      string    `json:"nodeName" example:"aws-ap-northeast-2-g1-1"`
	Status      string    `json:"status" example:"Success" enums:"Success,Failed"`
	ImageId     string    `json:"imageId,omitempty" example:"custom-image-g1"`
	ImageInfo   ImageInfo `json:"imageInfo,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// InfraSnapshotResult represents the result of creating snapshots for an entire Infra
type InfraSnapshotResult struct {
	InfraId      string             `json:"infraId" example:"infra01"`
	Namespace    string             `json:"namespace" example:"default"`
	SuccessCount int                `json:"successCount" example:"3"`
	FailCount    int                `json:"failCount" example:"0"`
	Results      []NodeSnapshotResult `json:"results"`
}

// BuildAgnosticImageReq is a struct to handle 'Build Agnostic Image' request
// This combines Infra creation and snapshot creation into a single workflow
type BuildAgnosticImageReq struct {
	// Infra configuration for creating the infrastructure
	SourceInfraReq InfraDynamicReq `json:"sourceInfraReq" validate:"required"`

	// Snapshot configuration for creating custom images
	SnapshotReq SnapshotReq `json:"snapshotReq" validate:"required"`

	// Whether to cleanup (terminate) Infra after snapshot creation
	CleanupInfraAfterSnapshot bool `json:"cleanupInfraAfterSnapshot" example:"true" default:"true"`
}

// BuildAgnosticImageResult represents the result of building agnostic images
type BuildAgnosticImageResult struct {
	// Infra information
	InfraId        string `json:"infraId" example:"infra01"`
	Namespace      string `json:"namespace" example:"default"`
	InfraStatus    string `json:"infraStatus" example:"Running"`
	InfraCleanedUp bool   `json:"infraCleanedUp" example:"true"`

	// Snapshot results
	SnapshotResult InfraSnapshotResult `json:"snapshotResult"`

	// Overall summary
	TotalDuration string `json:"totalDuration" example:"15m30s"`
	Message       string `json:"message" example:"Successfully created 3 custom images from Infra"`
}
