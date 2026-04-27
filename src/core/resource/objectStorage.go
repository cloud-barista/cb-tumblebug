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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// This file lists the structs and functions as followings:
// - Structs for Spider request/response for object storage
// - Functions for object storages (buckets)
// - Functions for object storages (buckets) CORS configuration
// - Functions for object storages (buckets) versioning configuration
// - Functions for objects (files) in the object storages (buckets)

type ObjectStorageVersioningOption string

const (
	ObjectStorageVersioningEnabled     ObjectStorageVersioningOption = "Enabled"     // Versioning is enabled for the object storage.
	ObjectStorageVersioningSuspended   ObjectStorageVersioningOption = "Suspended"   // Versioning is suspended for the object storage.
	ObjectStorageVersioningUnversioned ObjectStorageVersioningOption = "Unversioned" // Versioning is not enabled for the object storage.
)

// ========== Resource APIs: Object Storage ==========

// spiderListBucketRes represents the JSON response from Spider for listing S3 buckets
// Matches Spider's ListAllMyBucketsResultJSON
type spiderListBucketRes struct {
	Owner   spiderOwner   `json:"Owner"`
	Buckets spiderBuckets `json:"Buckets"`
}

// spiderOwner represents the owner information in S3 bucket list response
type spiderOwner struct {
	ID          string `json:"ID" example:"aws-ap-northeast-2"`
	DisplayName string `json:"DisplayName" example:"aws-ap-northeast-2"`
}

// spiderBucket represents a single bucket in the JSON bucket list response
// Matches Spider's BucketJSON (IId-based, not Name-based)
type spiderBucket struct {
	IId          spiderBucketIID `json:"IId"`
	CreationDate string          `json:"CreationDate" example:"2025-09-04T04:18:06Z"`
}

// spiderBuckets represents the collection of buckets in S3 bucket list response
type spiderBuckets struct {
	Bucket []spiderBucket `json:"Bucket"`
}

type spiderBucketIID struct {
	NameId   string `json:"NameId"`
	SystemId string `json:"SystemId"`
}

// spiderGetBucketInfoRes represents the JSON response from Spider for a single bucket
// Matches Spider's ListBucketResultJSON (no Name, no CreationDate in JSON schema)
type spiderGetBucketInfoRes struct {
	IId         spiderBucketIID `json:"IId"`
	Prefix      string          `json:"Prefix" example:""`
	Marker      string          `json:"Marker" example:""`
	MaxKeys     int             `json:"MaxKeys" example:"1000"`
	IsTruncated bool            `json:"IsTruncated" example:"false"`
	Contents    []spiderObject  `json:"Contents"`
}

// spiderObjectStorageCreateRequest represents the request structure to create an S3 bucket in Spider
type spiderObjectStorageCreateRequest struct {
	BucketName     string `json:"BucketName" validate:"required" example:"globally-unique-bucket-name-12345"`
	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-ap-northeast-2"`
}

type spiderObjectStorageLocationResponse struct {
	LocationConstraint string `json:"LocationConstraint" example:"ap-northeast-2"`
}

// spiderObject represents a single object in the S3 bucket
type spiderObject struct {
	Key          string `json:"Key" example:"test-object.txt"`
	LastModified string `json:"LastModified" example:"2025-09-04T04:18:06Z"`
	ETag         string `json:"ETag" example:"9b2cf535f27731c974343645a3985328"`
	Size         int64  `json:"Size" example:"1024"`
	StorageClass string `json:"StorageClass" example:"STANDARD"`
}

// spiderPreSignedUrlResponse represents the response structure from Spider for generating a presigned URL
type spiderPreSignedUrlResponse struct {
	Expires      int64  `json:"Expires" example:"1693824000"`
	Method       string `json:"Method" example:"GET"`
	PreSignedURL string `json:"PreSignedURL" example:"https://example.com/presigned-url"`
}

// spiderGetCORSResponse represents the CORS rules for an S3 bucket
type spiderGetCORSResponse struct {
	CORSRule []spiderCorsRule `json:"CORSRule"`
}

// spiderSetCorsRequest represents the request structure to set CORS configuration for an S3 bucket in Spider
type spiderSetCorsRequest struct {
	CORSRule []spiderCorsRule `json:"CORSRule" validate:"required"`
}

// spiderCorsRule represents a single CORS rule in the set CORS request
type spiderCorsRule struct {
	AllowedOrigin []string `json:"AllowedOrigin" example:"*"`
	AllowedMethod []string `json:"AllowedMethod" example:"GET"`
	AllowedHeader []string `json:"AllowedHeader" example:"*"`
	ExposeHeader  []string `json:"ExposeHeader" example:"ETag"`
	MaxAgeSeconds int      `json:"MaxAgeSeconds" example:"3000"`
}

// spiderSetVersioningRequest represents the request structure to set versioning configuration for an S3 bucket in Spider
type spiderSetVersioningRequest struct {
	Status string `json:"Status" validate:"required" example:"Enabled"` // Possible values: "Enabled", "Suspended"
}

// spiderGetVersioningResponse represents the response structure from Spider for versioning configuration
type spiderGetVersioningResponse struct {
	Status string `json:"Status" example:"Enabled"` // Possible values: "Enabled", "Suspended"
}

// spiderListObjectsVersionsResponse represents the JSON response from Spider for listing object versions
// Matches Spider's ListVersionsResultJSON (no Name in JSON schema)
type spiderListObjectsVersionsResponse struct {
	IId                 spiderBucketIID       `json:"IId"`
	Prefix              string                `json:"Prefix" example:""`
	KeyMarker           string                `json:"KeyMarker" example:""`
	VersionIdMarker     string                `json:"VersionIdMarker" example:""`
	NextKeyMarker       string                `json:"NextKeyMarker" example:""`
	NextVersionIdMarker string                `json:"NextVersionIdMarker" example:""`
	MaxKeys             int                   `json:"MaxKeys" example:"1000"`
	IsTruncated         bool                  `json:"IsTruncated" example:"false"`
	Version             []spiderObjectVersion `json:"Version"`
	DeleteMarker        []spiderObjectVersion `json:"DeleteMarker"`
}

// spiderObjectVersion represents a single object version in the S3 bucket
type spiderObjectVersion struct {
	Key          string      `json:"Key,omitempty" example:"test-object.txt"`
	VersionId    string      `json:"VersionId,omitempty" example:"3/L4kqtJlcpXroDTDmJ+rmSpXd3aIbrC"`
	IsLatest     bool        `json:"IsLatest,omitempty" example:"true"`
	LastModified string      `json:"LastModified,omitempty" example:"2025-09-04T04:18:06Z"`
	ETag         string      `json:"ETag,omitempty" example:"9b2cf535f27731c974343645a3985328"`
	Size         int64       `json:"Size,omitempty" example:"1024"`
	StorageClass string      `json:"StorageClass,omitempty" example:"STANDARD"`
	Owner        spiderOwner `json:"Owner,omitempty"`
}

// spiderS3JSONHeaders sets Accept header to ensure Spider returns JSON responses (not XML)
var spiderS3JSONHeaders = map[string]string{
	"Accept": "application/json",
}

// checkObjectKey validates the object key (file name) for S3 operations
func checkObjectKey(objectKey string) error {
	if objectKey == "" {
		return fmt.Errorf("objectKey cannot be empty")
	}

	// S3 object key length validation (max 1024 characters)
	if len(objectKey) > 1024 {
		return fmt.Errorf("objectKey length exceeds maximum of 1024 characters")
	}

	// Check for invalid characters
	// AWS S3 recommends avoiding: backslash (\), control characters (0x00-0x1F, 0x7F)
	for i, r := range objectKey {
		// Control characters
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("objectKey contains invalid control character at position %d", i)
		}
		// Backslash
		if r == '\\' {
			return fmt.Errorf("objectKey contains backslash (\\) which should be avoided")
		}
	}

	// Check for problematic patterns
	if objectKey[0] == '/' {
		return fmt.Errorf("objectKey should not start with slash (/)")
	}
	if objectKey[len(objectKey)-1] == '/' {
		return fmt.Errorf("objectKey should not end with slash (/)")
	}

	return nil
}

var cspSupportingObjectStorage = map[string]bool{
	csp.AWS:       true,
	csp.Azure:     false, // TODO: to be supported
	csp.GCP:       true,
	csp.Alibaba:   true,
	csp.Tencent:   true,
	csp.IBM:       true,
	csp.OpenStack: true,
	csp.NCP:       true,
	csp.NHN:       true,
	csp.KT:        true,
}

func isObjectStorageSupported(cspType string) bool {
	cspType = csp.ResolveCloudPlatform(cspType)
	supported, exists := cspSupportingObjectStorage[cspType]
	if !exists {
		return false
	}
	return supported
}

var cspSupportingObjectStorageCors = map[string]bool{
	csp.AWS:       true,
	csp.Azure:     false, // TODO: to be decided when Azure object storage is supported
	csp.GCP:       true,
	csp.Alibaba:   true,
	csp.Tencent:   true,
	csp.IBM:       true,
	csp.OpenStack: true,
	csp.NCP:       false,
	csp.NHN:       false,
	csp.KT:        true,
}

func isObjectStorageCorsSupported(cspType string) bool {
	cspType = csp.ResolveCloudPlatform(cspType)
	supported, exists := cspSupportingObjectStorageCors[cspType]
	if !exists {
		return false
	}
	return supported
}

var cspSupportingObjectStorageVersioning = map[string]bool{
	csp.AWS:       true,
	csp.Azure:     false, // TODO: to be decided when Azure object storage is supported
	csp.GCP:       true,
	csp.Alibaba:   true,
	csp.Tencent:   true,
	csp.IBM:       true,
	csp.OpenStack: false,
	csp.NCP:       false,
	csp.NHN:       false,
	csp.KT:        true,
}

func isObjectStorageVersioningSupported(cspType string) bool {
	cspType = csp.ResolveCloudPlatform(cspType)
	supported, exists := cspSupportingObjectStorageVersioning[cspType]
	if !exists {
		return false
	}
	return supported
}

// GetObjectStorageSupport retrieves the CSP support information for object storage features
// If cspType is provided, it returns support information for that specific CSP
// If cspType is empty, it returns support information for all CSPs
func GetObjectStorageSupport(cspType string) (model.ObjectStorageSupportResponse, error) {
	var response model.ObjectStorageSupportResponse

	// If cspType is specified, return support for that CSP only
	if cspType != "" {
		cspType = strings.ToLower(cspType)

		// Check if the CSP exists in the support map
		isCorsSupported, corsExists := cspSupportingObjectStorageCors[cspType]
		isVersioningSupported, versioningExists := cspSupportingObjectStorageVersioning[cspType]

		if !corsExists && !versioningExists {
			return response, fmt.Errorf("unknown CSP type: %s", cspType)
		}

		response.ResourceType = model.StrObjectStorage
		response.Supports = map[string]model.ObjectStorageFeatureSupport{
			cspType: {
				Cors:       isCorsSupported,
				Versioning: isVersioningSupported,
			},
		}
		return response, nil
	}

	// Return support information for all CSPs
	allSupports := make(map[string]model.ObjectStorageFeatureSupport)

	// Iterate through all CSPs in the CORS support map
	for _, providerName := range csp.AllCSPs {
		isCorsSupported := cspSupportingObjectStorageCors[providerName]
		isVersioningSupported := cspSupportingObjectStorageVersioning[providerName]

		allSupports[providerName] = model.ObjectStorageFeatureSupport{
			Cors:       isCorsSupported,
			Versioning: isVersioningSupported,
		}
	}

	response.ResourceType = model.StrObjectStorage
	response.Supports = allSupports
	return response, nil
}

/*
 * Functions for object storages (buckets)
 */

// CreateObjectStorage creates a new object storage (bucket) in the specified namespace
func CreateObjectStorage(ctx context.Context, nsId string, req model.ObjectStorageCreateRequest) (model.ObjectStorageInfo, error) {

	var emptyRet model.ObjectStorageInfo
	var objStrgInfo model.ObjectStorageInfo

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = validate.Struct(req)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = common.CheckString(req.BucketName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	_, err = common.GetConnConfig(req.ConnectionName)
	if err != nil {
		err = fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the input CSP is supported for object storage
	cspType := strings.Split(req.ConnectionName, "-")[0]
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Set the resource type
	resourceType := model.StrObjectStorage

	// 3. Set the object storage info in advance
	objStrgInfo.ResourceType = resourceType
	objStrgInfo.Name = req.BucketName
	objStrgInfo.Id = req.BucketName
	// objStrgInfo.Uid = uid             // Set this below, before call to Spider for retry if conflict occurs
	// objStrgInfo.CspResourceName = ""  // Set this after creation
	// objStrgInfo.CspResourceId = ""    // Set this after creation
	objStrgInfo.ConnectionName = req.ConnectionName
	objStrgInfo.ConnectionConfig, err = common.GetConnConfig(req.ConnectionName)
	if err != nil {
		err = fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	objStrgInfo.Description = req.Description

	// 4. Set a objectStorageKey for the object storage info
	objStrgKey := common.GenResourceKey(nsId, resourceType, objStrgInfo.Id)

	// 5. Check if the objectStorage already exists or not
	exists, err := CheckResource(nsId, resourceType, objStrgInfo.Id)
	if exists {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("already exists, object storage: %s", objStrgInfo.Id)
		return emptyRet, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the object storage (%s) exists or not", objStrgInfo.Id)
		return emptyRet, err
	}

	// 6. [Conditions] Mark as not ready (creating) before calling Spider API
	model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreating, "Object storage creation in progress")
	model.SetCondition(&objStrgInfo.Conditions, model.ConditionSynced, model.ConditionFalse, model.ReasonCreating, "")
	objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
	val, err := json.Marshal(objStrgInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(objStrgKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 7. Call Spider API to create the object storage and retry if conflict occurs

	client := clientManager.NewHttpClient()
	method := "PUT"
	spResp := clientManager.NoBody

	var uid string
	var spReq spiderObjectStorageCreateRequest

	maxRetries := 5
	retryCount := 0

	for {
		uid = common.GenUid()
		objStrgInfo.Uid = uid // Set uid immediately so failed-state metadata always records the attempted CSP resource name
		spReq = spiderObjectStorageCreateRequest{}
		spReq.ConnectionName = req.ConnectionName
		spReq.BucketName = uid

		log.Debug().Msgf("spReqt: %+v", spReq)
		// defer function to handle failure case

		url := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, spReq.BucketName, spReq.ConnectionName)
		log.Debug().Msgf("[Request to Spider] Creating a object storage (url: %s, request body: %+v)", url, spReq)

		restyResp, err := clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			spiderS3JSONHeaders,
			clientManager.SetUseBody(spReq),
			&spReq,
			&spResp,
			clientManager.ShortDuration,
		)

		if err != nil {
			if restyResp != nil && restyResp.StatusCode() == http.StatusConflict {
				retryCount++
				if retryCount >= maxRetries {
					err = fmt.Errorf("failed to create object storage after %d retries", maxRetries)
					log.Error().Err(err).Msg("")
					// [Conditions] Creation failed → mark as Failed to prevent stuck state
					model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreationFailed, err.Error())
					objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
					objStrgInfo.SystemMessage = err.Error()
					failVal, marshalErr := json.Marshal(objStrgInfo)
					if marshalErr == nil {
						_ = kvstore.Put(objStrgKey, string(failVal))
					}
					return emptyRet, err
				}
				log.Warn().Msgf("Conflict detected for bucket name %s, retrying... (%d/%d)", spReq.BucketName, retryCount, maxRetries)
				continue
			} else {
				log.Error().Err(err).Msg("")
				// [Conditions] Creation failed → mark as Failed to prevent stuck state
				model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreationFailed, err.Error())
				objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
				objStrgInfo.SystemMessage = err.Error()
				failVal, marshalErr := json.Marshal(objStrgInfo)
				if marshalErr == nil {
					_ = kvstore.Put(objStrgKey, string(failVal))
				}
				return emptyRet, err
			}
		}

		log.Debug().Msgf("[Response from Spider] Creating a object storage (No response body): %+v", spResp)
		break
	}
	// objStrgInfo.Uid is already set at the start of each loop iteration

	// 8. Call Spider API to get the created object storage info
	// Currently, there is no specific response body from Spider for object storage creation.

	client = clientManager.NewHttpClient()
	method = "GET"
	spGetBucketInfoReq := clientManager.NoBody
	spGetBucketInfoRes := spiderGetBucketInfoRes{}
	url := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, objStrgInfo.Uid, req.ConnectionName)
	log.Debug().Msgf("[Request to Spider] Getting the created object storage info (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spGetBucketInfoReq),
		&spGetBucketInfoReq,
		&spGetBucketInfoRes,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		// [Conditions] Creation failed (GET after PUT) → mark as Failed to prevent stuck state
		model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreationFailed, err.Error())
		objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
		objStrgInfo.SystemMessage = err.Error()
		failVal, marshalErr := json.Marshal(objStrgInfo)
		if marshalErr == nil {
			_ = kvstore.Put(objStrgKey, string(failVal))
		}
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Getting the created object storage info: %+v", spGetBucketInfoRes)

	// 9. Set the object storage info
	objStrgInfo.CspResourceName = spGetBucketInfoRes.IId.NameId
	objStrgInfo.CspResourceId = spGetBucketInfoRes.IId.SystemId
	objStrgInfo.Prefix = spGetBucketInfoRes.Prefix
	objStrgInfo.Marker = spGetBucketInfoRes.Marker
	objStrgInfo.MaxKeys = spGetBucketInfoRes.MaxKeys
	objStrgInfo.IsTruncated = spGetBucketInfoRes.IsTruncated

	contents := make([]model.Object, 0)
	for _, spObj := range spGetBucketInfoRes.Contents {
		obj := model.Object{
			Key:          spObj.Key,
			LastModified: spObj.LastModified,
			ETag:         spObj.ETag,
			Size:         spObj.Size,
			StorageClass: spObj.StorageClass,
		}
		contents = append(contents, obj)
	}
	objStrgInfo.Contents = contents

	// 10. [Conditions] Creation succeeded → mark as ready and synced
	model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonAvailable, "")
	model.SetCondition(&objStrgInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "")
	objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
	objStrgInfo.SystemMessage = ""
	val, err = json.Marshal(objStrgInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(objStrgKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 11. Check if the object storage is stored or not
	storedObjStrgInfo, exists, err := kvstore.GetKv(objStrgKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err = fmt.Errorf("not found after creation, object storage: %s", objStrgInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(storedObjStrgInfo.Value), &objStrgInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 12. Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrObjectStorage,
		model.LabelId:              objStrgInfo.Id,
		model.LabelName:            objStrgInfo.Name,
		model.LabelUid:             objStrgInfo.Uid,
		model.LabelCspResourceId:   objStrgInfo.CspResourceId,
		model.LabelCspResourceName: objStrgInfo.CspResourceName,
		model.LabelDescription:     objStrgInfo.Description,
		model.LabelConnectionName:  objStrgInfo.ConnectionName,
	}

	err = label.CreateOrUpdateLabel(ctx, model.StrObjectStorage, objStrgInfo.Uid, objStrgKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 13. Return the object storage info
	return objStrgInfo, nil
}

// GetObjectStorage retrieves the object storage (bucket) information from the specified namespace
func GetObjectStorage(nsId, osId string) (model.ObjectStorageInfo, error) {
	var emptyRet model.ObjectStorageInfo

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}
	oldObjStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := oldObjStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	connName := oldObjStrgInfo.ConnectionName
	uid := oldObjStrgInfo.Uid

	// 3. Call Spider API to get the object storage info
	client := clientManager.NewHttpClient()
	method := "GET"
	spReq := clientManager.NoBody
	spResp := spiderGetBucketInfoRes{}
	url := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, uid, connName)
	log.Debug().Msgf("[Request to Spider] Getting the object storage info (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Getting the object storage info: %+v", spResp)

	// 4. Compare saved and retrieved info and update the object storage info

	// Deep copy by marshalling and unmarshalling
	data, _ := json.Marshal(oldObjStrgInfo)
	var newObjStrgInfo model.ObjectStorageInfo
	_ = json.Unmarshal(data, &newObjStrgInfo)

	// Set the retrieved values
	newObjStrgInfo.CspResourceName = spResp.IId.NameId
	newObjStrgInfo.CspResourceId = spResp.IId.SystemId
	newObjStrgInfo.Prefix = spResp.Prefix
	newObjStrgInfo.Marker = spResp.Marker
	newObjStrgInfo.MaxKeys = spResp.MaxKeys
	newObjStrgInfo.IsTruncated = spResp.IsTruncated

	contents := make([]model.Object, 0)
	for _, spObj := range spResp.Contents {
		obj := model.Object{
			Key:          spObj.Key,
			LastModified: spObj.LastModified,
			ETag:         spObj.ETag,
			Size:         spObj.Size,
			StorageClass: spObj.StorageClass,
		}
		contents = append(contents, obj)
	}
	newObjStrgInfo.Contents = contents

	// Check chanages and update if necessary
	if isObjStrgInfoUpdated(oldObjStrgInfo, newObjStrgInfo) {
		// Update the object storage info in the key-value store
		objStrgKey := common.GenResourceKey(nsId, resourceType, newObjStrgInfo.Id)
		val, err := json.Marshal(newObjStrgInfo)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal new object storage info")
			return emptyRet, err
		}
		err = kvstore.Put(objStrgKey, string(val))
		if err != nil {
			log.Error().Err(err).Msg("Failed to update object storage info in kvstore")
			return emptyRet, err
		}
	}

	// 5. Return the object storage info

	return newObjStrgInfo, nil
}

func isObjStrgInfoUpdated(oldObjStrgInfo, newObjStrgInfo model.ObjectStorageInfo) bool {
	if oldObjStrgInfo.CspResourceName != newObjStrgInfo.CspResourceName {
		return true
	}
	if oldObjStrgInfo.CspResourceId != newObjStrgInfo.CspResourceId {
		return true
	}
	if oldObjStrgInfo.Prefix != newObjStrgInfo.Prefix {
		return true
	}
	if oldObjStrgInfo.Marker != newObjStrgInfo.Marker {
		return true
	}
	if oldObjStrgInfo.MaxKeys != newObjStrgInfo.MaxKeys {
		return true
	}
	if oldObjStrgInfo.IsTruncated != newObjStrgInfo.IsTruncated {
		return true
	}
	if oldObjStrgInfo.CreationDate != newObjStrgInfo.CreationDate {
		return true
	}
	if len(oldObjStrgInfo.Contents) != len(newObjStrgInfo.Contents) {
		return true
	}
	for i, oldObj := range oldObjStrgInfo.Contents {
		newObj := newObjStrgInfo.Contents[i]
		if oldObj.Key != newObj.Key ||
			oldObj.LastModified != newObj.LastModified ||
			oldObj.ETag != newObj.ETag ||
			oldObj.Size != newObj.Size ||
			oldObj.StorageClass != newObj.StorageClass {
			return true
		}
	}
	return false
}

// DeleteObjectStorage deletes the specified object storage (bucket) from the specified namespace.
// If empty is true, it first empties bucket contents. If force is true, Spider force-deletes bucket with contents.
func DeleteObjectStorage(nsId, osId string, force, empty bool) error {

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 2. Set the resource type
	resourceType := model.StrObjectStorage

	// 3. Get the object storage info to retrieve ConnectionName and Uid
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return err
	}

	// 4. [Conditions] Mark as not ready (deleting) before calling Spider API
	model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeleting, "Object storage deletion in progress")
	objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
	objStrgInfo.SystemMessage = ""
	objStrgKey := common.GenResourceKey(nsId, resourceType, objStrgInfo.Id)
	val, err := json.Marshal(objStrgInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = kvstore.Put(objStrgKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 5. Call Spider API to delete the object storage, then verify via GET.
	// If uid is empty, the CSP resource was never successfully created; skip Spider and go straight to metadata cleanup.
	if uid != "" {
		client := clientManager.NewHttpClient()

		deleteURL := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, uid, connName)
		if force {
			deleteURL = deleteURL + "&force=true"
		} else if empty {
			deleteURL = deleteURL + "&empty=true"
		}
		getURL := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, uid, connName)

		const maxDeleteAttempts = 2
		deleted := false

		for attempt := 1; attempt <= maxDeleteAttempts; attempt++ {
			// 5-1. DELETE
			spDelReq := clientManager.NoBody
			spDelResp := clientManager.NoBody
			log.Debug().Msgf("[Request to Spider] Deleting the object storage (url: %s, attempt: %d/%d)", deleteURL, attempt, maxDeleteAttempts)

			_, delErr := clientManager.ExecuteHttpRequest(
				client,
				"DELETE",
				deleteURL,
				spiderS3JSONHeaders,
				clientManager.SetUseBody(spDelReq),
				&spDelReq,
				&spDelResp,
				clientManager.ShortDuration,
			)

			// Spider returns 404 when the bucket does not exist on the CSP side → already gone.
			if delErr != nil && strings.Contains(delErr.Error(), "404") {
				log.Warn().Msgf("Spider returned 404 on DELETE for object storage %s (attempt %d/%d) — already deleted", uid, attempt, maxDeleteAttempts)
				deleted = true
				break
			}

			if delErr != nil {
				log.Warn().Err(delErr).Msgf("DELETE attempt %d/%d failed for object storage %s", attempt, maxDeleteAttempts, uid)
				// Fall through to GET to check the actual current state
			}

			// 5-2. GET to verify deletion
			spGetReq := clientManager.NoBody
			spGetResp := spiderGetBucketInfoRes{}
			log.Debug().Msgf("[Request to Spider] Verifying object storage deletion via GET (url: %s)", getURL)

			_, getErr := clientManager.ExecuteHttpRequest(
				client,
				"GET",
				getURL,
				spiderS3JSONHeaders,
				clientManager.SetUseBody(spGetReq),
				&spGetReq,
				&spGetResp,
				clientManager.ShortDuration,
			)

			if getErr != nil {
				// GET returned error → resource no longer exists → confirmed deleted
				log.Info().Msgf("[Response from Spider] Object storage %s confirmed deleted (GET returned: %v)", uid, getErr)
				deleted = true
				break
			}

			// GET succeeded → resource still exists on the CSP side
			if attempt < maxDeleteAttempts {
				log.Warn().Msgf("Object storage %s still exists after DELETE attempt %d/%d, retrying...", uid, attempt, maxDeleteAttempts)
				time.Sleep(3 * time.Second)
			}
		}

		if !deleted {
			err = fmt.Errorf("object storage %s still exists on CSP after %d delete attempts", uid, maxDeleteAttempts)
			log.Error().Err(err).Msg("")
			// [Conditions] Deletion failed → mark as Failed to prevent stuck state
			model.SetCondition(&objStrgInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeletionFailed, err.Error())
			objStrgInfo.Status = model.DeriveObjectStorageStatus(objStrgInfo.Conditions)
			objStrgInfo.SystemMessage = err.Error()
			failVal, marshalErr := json.Marshal(objStrgInfo)
			if marshalErr == nil {
				_ = kvstore.Put(objStrgKey, string(failVal))
			}
			return err
		}
	} else {
		log.Warn().Msgf("Object storage %s has no CSP resource (Uid is empty). Skipping Spider DELETE and removing metadata only.", osId)
	}

	// 6. Delete the object storage info from the key-value store
	err = kvstore.Delete(objStrgKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 7. Delete label info
	err = label.DeleteLabelObject(model.StrObjectStorage, objStrgInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// ReconcileObjectStorage detects and repairs discrepancies between Tumblebug metadata
// and the actual state of the CSP resource.
//
// Reconcile covers two main scenarios that arise from partial failures:
//  1. The CSP resource was never created (Uid is empty) but metadata was persisted
//     in a Failed state → the metadata is orphaned and is removed.
//  2. The CSP resource creation or deletion failed mid-way (Uid is set) but the bucket
//     no longer exists on the CSP side → the orphaned metadata is removed.
//
// If the CSP resource does exist and the metadata accurately reflects its state,
// no action is taken and "NoActionNeeded" is returned.
func ReconcileObjectStorage(nsId, osId string) (model.ObjectStorageReconcileResponse, error) {
	var emptyRet model.ObjectStorageReconcileResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	result := model.ObjectStorageReconcileResponse{
		ObjectStorageId: osId,
	}

	// 2. Fetch metadata from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		// Metadata not found – nothing to reconcile
		result.MetadataStatus = "NotFound"
		result.CspResourceStatus = "Skipped"
		result.Action = "NoActionNeeded"
		result.Message = fmt.Sprintf("No metadata found for object storage '%s'; nothing to reconcile", osId)
		log.Warn().Msgf("ReconcileObjectStorage: %s", result.Message)
		return result, nil
	}

	objStrgInfo := objStrgData.(model.ObjectStorageInfo)
	result.MetadataStatus = "Found"
	objStrgKey := common.GenResourceKey(nsId, resourceType, objStrgInfo.Id)

	// 3. If Uid is empty the CSP resource was never created; metadata is orphaned
	if objStrgInfo.Uid == "" {
		result.CspResourceStatus = "Skipped"
		log.Warn().Msgf("ReconcileObjectStorage: object storage '%s' has no CSP resource (Uid empty); removing orphaned metadata", osId)

		if delErr := kvstore.Delete(objStrgKey); delErr != nil {
			log.Error().Err(delErr).Msg("ReconcileObjectStorage: failed to delete orphaned metadata")
			return emptyRet, delErr
		}
		// Label cleanup is best-effort; Uid is empty so there is no label entry to clean.
		result.Action = "MetadataRemoved"
		result.Message = "Orphaned metadata removed: CSP resource was never created (Uid is empty)"
		return result, nil
	}

	// 4. Check whether the CSP resource actually exists via Spider HEAD
	connName := objStrgInfo.ConnectionName
	uid := objStrgInfo.Uid

	client := clientManager.NewHttpClient()
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody
	headURL := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, uid, connName)
	log.Debug().Msgf("[ReconcileObjectStorage] HEAD %s", headURL)

	_, headErr := clientManager.ExecuteHttpRequest(
		client,
		"HEAD",
		headURL,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if headErr == nil {
		// 5a. CSP resource exists — no corrective action required
		result.CspResourceStatus = "Exists"
		result.Action = "NoActionNeeded"
		result.Message = "CSP resource exists; metadata is consistent"
		log.Info().Msgf("ReconcileObjectStorage: bucket '%s' exists on CSP — no action needed", uid)
		return result, nil
	}

	// 5b. CSP resource does not exist (HEAD returned an error / 404)
	result.CspResourceStatus = "NotFound"
	log.Warn().Err(headErr).Msgf("ReconcileObjectStorage: bucket '%s' not found on CSP; removing orphaned metadata", uid)

	// Remove metadata from kvstore
	if delErr := kvstore.Delete(objStrgKey); delErr != nil {
		log.Error().Err(delErr).Msg("ReconcileObjectStorage: failed to delete orphaned metadata")
		return emptyRet, delErr
	}

	// Remove label — best-effort (non-fatal if label is already absent)
	if labelErr := label.DeleteLabelObject(model.StrObjectStorage, objStrgInfo.Uid); labelErr != nil {
		log.Warn().Err(labelErr).Msg("ReconcileObjectStorage: failed to delete label (non-fatal)")
	}

	result.Action = "MetadataRemoved"
	result.Message = fmt.Sprintf("Orphaned metadata removed: CSP resource '%s' does not exist", uid)
	return result, nil
}

// CheckObjectStorageExistence checks if the object storage exists in both the key-value store and Spider
func CheckObjectStorageExistence(nsId, osId string) (bool, error) {

	exists := false

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	// 2. Set the resource type
	resourceType := model.StrObjectStorage

	// 3. Check if the object storage exists
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to check existence, object storage: %s", osId)
		return false, err
	}

	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return false, err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 4. Call Spider API to verify existence if it exists in kvstore
	client := clientManager.NewHttpClient()
	method := "HEAD"
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody

	url := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, uid, connName)
	log.Debug().Msgf("[Request to Spider] Checking existence of the object storage (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msgf("object storage %s does not exist in Spider", osId)
		return false, nil
	}

	log.Debug().Msgf("[Response from Spider] Object storage %s exists", osId)

	// 5. Return existence as true
	exists = true

	return exists, nil
}

// GetObjectStorageLocation retrieves the location of the specified object storage (bucket)
func GetObjectStorageLocation(nsId, osId string) (model.ObjectStorageLocationResponse, error) {
	var emptyRet model.ObjectStorageLocationResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Call Spider API to get the object storage location
	client := clientManager.NewHttpClient()
	method := "GET"
	spReq := clientManager.NoBody
	spResp := spiderObjectStorageLocationResponse{}
	url := fmt.Sprintf("%s/s3/%s?location&ConnectionName=%s", model.SpiderRestUrl, uid, connName)
	log.Debug().Msgf("[Request to Spider] Getting the object storage location (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Getting the object storage location: %+v", spResp)

	// 4. Set and return the object storage location response
	locationResp := model.ObjectStorageLocationResponse{
		LocationConstraint: spResp.LocationConstraint,
	}

	return locationResp, nil
}

/*
 * Functions for object storages (buckets) CORS configuration
 */

// SetObjectStorageCorsConfigurations sets the CORS configuration for the specified object storage (bucket)
func SetObjectStorageCorsConfigurations(nsId, osId string, req model.ObjectStorageSetCorsRequest) error {

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if CORS configuration is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageCorsSupported(cspType) {
		err = fmt.Errorf("cors configuration is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Prepare the Spider CORS rules request body
	spCorsRules := spiderSetCorsRequest{}
	for _, rule := range req.CorsRule {
		spRule := spiderCorsRule{
			AllowedOrigin: rule.AllowedOrigin,
			AllowedMethod: rule.AllowedMethod,
			AllowedHeader: rule.AllowedHeader,
			ExposeHeader:  rule.ExposeHeader,
			MaxAgeSeconds: rule.MaxAgeSeconds,
		}
		spCorsRules.CORSRule = append(spCorsRules.CORSRule, spRule)
	}

	// 4. Call Spider API to set the object storage CORS configuration
	client := clientManager.NewHttpClient()
	method := "PUT"
	spReq := spCorsRules
	spResp := clientManager.NoBody
	url := fmt.Sprintf("%s/s3/%s?cors&ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	log.Debug().Msgf("[Request to Spider] Setting the object storage CORS configuration (url: %s, request body: %+v)", url, spReq)
	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Debug().Msgf("[Response from Spider] Setting the object storage CORS configuration (No response body): %+v", spResp)

	return nil
}

// GetObjectStorageCorsConfigurations retrieves the CORS configuration for the specified object storage (bucket)
func GetObjectStorageCorsConfigurations(nsId, osId string) (model.ObjectStorageGetCorsResponse, error) {
	var emptyRet model.ObjectStorageGetCorsResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if CORS configuration is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageCorsSupported(cspType) {
		err = fmt.Errorf("cors configuration is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Call Spider API to get the object storage CORS configuration
	client := clientManager.NewHttpClient()
	method := "GET"
	spReq := clientManager.NoBody
	spResp := spiderGetCORSResponse{}
	url := fmt.Sprintf("%s/s3/%s?cors&ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	log.Debug().Msgf("[Request to Spider] Getting the object storage CORS configuration (url: %s)", url)
	restyResponse, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		if restyResponse != nil && restyResponse.StatusCode() == http.StatusNotFound {
			// Return empty CORS configuration if not found
			err := fmt.Errorf("not found CORS configuration for object storage: %s", osId)
			log.Warn().Err(err).Msg(err.Error())
			return model.ObjectStorageGetCorsResponse{CorsRule: []model.CorsRule{}}, err
		}
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Getting the object storage CORS configuration: %+v", spResp)

	// 4. Set and return the object storage CORS configuration
	corsRules := make([]model.CorsRule, 0)
	for _, spRule := range spResp.CORSRule {
		rule := model.CorsRule{
			AllowedOrigin: spRule.AllowedOrigin,
			AllowedMethod: spRule.AllowedMethod,
			AllowedHeader: spRule.AllowedHeader,
			ExposeHeader:  spRule.ExposeHeader,
			MaxAgeSeconds: spRule.MaxAgeSeconds,
		}
		corsRules = append(corsRules, rule)
	}

	corsConfig := model.ObjectStorageGetCorsResponse{
		CorsRule: corsRules,
	}

	return corsConfig, nil
}

// DeleteObjectStorageCorsConfigurations deletes the CORS configuration for the specified object storage (bucket)
func DeleteObjectStorageCorsConfigurations(nsId, osId string) error {

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if CORS configuration is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageCorsSupported(cspType) {
		err = fmt.Errorf("cors configuration is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Call Spider API to delete the object storage CORS configuration
	client := clientManager.NewHttpClient()
	method := "DELETE"
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody
	url := fmt.Sprintf("%s/s3/%s?cors&ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	log.Debug().Msgf("[Request to Spider] Deleting the object storage CORS configuration (url: %s)", url)
	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Debug().Msgf("[Response from Spider] Deleting the object storage CORS configuration (No response body): %+v", spResp)

	return nil
}

/*
 * Functions for object storage versioning
 */

// SetObjectStorageVersioning sets the versioning configuration for the specified object storage (bucket)
func SetObjectStorageVersioning(nsId, osId string, req model.ObjectStorageSetVersioningRequest) error {

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if versioning is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageVersioningSupported(cspType) {
		err = fmt.Errorf("versioning is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Prepare the Spider versioning request body
	spVersioningReq := spiderSetVersioningRequest{
		Status: req.Status,
	}

	// 4. Call Spider API to set the object storage versioning configuration
	client := clientManager.NewHttpClient()
	method := "PUT"
	spReq := spVersioningReq
	spResp := clientManager.NoBody
	url := fmt.Sprintf("%s/s3/%s?versioning&ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	log.Debug().Msgf("[Request to Spider] Setting the object storage versioning configuration (url: %s, request body: %+v)", url, spReq)
	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Debug().Msgf("[Response from Spider] Setting the object storage versioning configuration (No response body): %+v", spResp)

	return nil
}

// GetObjectStorageVersioning retrieves the versioning configuration for the specified object storage (bucket)
func GetObjectStorageVersioning(nsId, osId string) (model.ObjectStorageGetVersioningResponse, error) {
	var emptyRet model.ObjectStorageGetVersioningResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if versioning is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageVersioningSupported(cspType) {
		err = fmt.Errorf("versioning is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Call Spider API to get the object storage versioning configuration
	client := clientManager.NewHttpClient()
	method := "GET"
	spReq := clientManager.NoBody
	spResp := spiderGetVersioningResponse{}
	url := fmt.Sprintf("%s/s3/%s?versioning&ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	log.Debug().Msgf("[Request to Spider] Getting the object storage versioning configuration (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Getting the object storage versioning configuration: %+v", spResp)

	// 4. Set and return the object storage versioning configuration
	versioningResp := model.ObjectStorageGetVersioningResponse{
		Status: spResp.Status,
	}

	return versioningResp, nil
}

// ListObjectVersions lists the versions of objects in the specified object storage (bucket)
func ListObjectVersions(nsId, osId string) (model.ObjectStorageListObjectVersionsResponse, error) {
	var emptyRet model.ObjectStorageListObjectVersionsResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if versioning is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageVersioningSupported(cspType) {
		err = fmt.Errorf("versioning is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Call Spider API to list object versions
	client := clientManager.NewHttpClient()
	method := "GET"
	spReq := clientManager.NoBody
	spResp := spiderListObjectsVersionsResponse{}
	url := fmt.Sprintf("%s/s3/%s?versions&ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	log.Debug().Msgf("[Request to Spider] Listing object versions (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Listing object versions: %+v", spResp)

	// 4. Set and return the list of object versions
	ret := model.ObjectStorageListObjectVersionsResponse{
		Name:                objStrgInfo.Name,
		Prefix:              spResp.Prefix,
		KeyMarker:           spResp.KeyMarker,
		VersionIdMarker:     spResp.VersionIdMarker,
		NextKeyMarker:       spResp.NextKeyMarker,
		NextVersionIdMarker: spResp.NextVersionIdMarker,
		MaxKeys:             spResp.MaxKeys,
		IsTruncated:         spResp.IsTruncated,
	}

	versions := make([]model.ObjectVersion, 0)
	for _, spVer := range spResp.Version {
		ver := model.ObjectVersion{
			Key:          spVer.Key,
			VersionId:    spVer.VersionId,
			IsLatest:     spVer.IsLatest,
			LastModified: spVer.LastModified,
			ETag:         spVer.ETag,
			Size:         spVer.Size,
			StorageClass: spVer.StorageClass,
		}

		owner := model.Owner{
			ID:          spVer.Owner.ID,
			DisplayName: spVer.Owner.DisplayName,
		}
		ver.Owner = owner

		versions = append(versions, ver)
	}
	ret.Version = versions

	deleteMarkers := make([]model.ObjectVersion, 0)
	for _, spDelMarker := range spResp.DeleteMarker {
		delMarker := model.ObjectVersion{
			Key:          spDelMarker.Key,
			VersionId:    spDelMarker.VersionId,
			IsLatest:     spDelMarker.IsLatest,
			LastModified: spDelMarker.LastModified,
		}

		owner := model.Owner{
			ID:          spDelMarker.Owner.ID,
			DisplayName: spDelMarker.Owner.DisplayName,
		}
		delMarker.Owner = owner

		deleteMarkers = append(deleteMarkers, delMarker)
	}
	ret.DeleteMarker = deleteMarkers

	return ret, nil
}

// DeleteVersionedObject deletes a specific version of an object in the specified object storage (bucket)
func DeleteVersionedObject(nsId, osId, objectKey, versionId string) error {
	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = checkObjectKey(objectKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 2. Get the object storage info from the key-value store
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if versioning is supported for the CSP type
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageVersioningSupported(cspType) {
		err = fmt.Errorf("versioning is not supported for CSP (%s)", cspType)
		log.Error().Err(err).Msg("")
		return err
	}

	objVersions, err := ListObjectVersions(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to list object versions for object storage: %s", osId)
		return err
	}

	// Check if the specified version of the object exists in Version or DeleteMarker
	found := false
	for _, ver := range objVersions.Version {
		if ver.Key == objectKey && ver.VersionId == versionId {
			found = true
			break
		}
	}
	if !found {
		for _, delMarker := range objVersions.DeleteMarker {
			if delMarker.Key == objectKey && delMarker.VersionId == versionId {
				found = true
				break
			}
		}
	}
	if !found {
		err = fmt.Errorf("not found, object key: %s with version ID: %s", objectKey, versionId)
		log.Error().Err(err).Msg("")
		return err
	}

	uid := objStrgInfo.Uid
	connName := objStrgInfo.ConnectionName

	// 3. Call Spider API to delete the specific version of the object
	client := clientManager.NewHttpClient()
	method := "DELETE"
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody
	url := fmt.Sprintf("%s/s3/%s/%s?versionId=%s&ConnectionName=%s", model.SpiderRestUrl, uid, objectKey, versionId, connName)

	log.Debug().Msgf("[Request to Spider] Deleting versioned object (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Debug().Msgf("[Response from Spider] Deleting versioned object (No response body): %+v", spResp)

	return nil
}

/*
 * Functions for objects (data)
 */

// ! IMPORTANT: To avoid data transfer overhead,
// ! Tumblebug will provide presigned URLs for uploading and downloading objects.
// ! The upload or download of objects is NOT handled directly by Tumblebug.

// GeneratePresignedURL generates a presigned URL for downloading or uploading an object
func GeneratePresignedURL(nsId, osId, objectKey string, expires time.Duration, operation string) (model.ObjectStoragePresignedUrlResponse, error) {
	var emptyRet model.ObjectStoragePresignedUrlResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = checkObjectKey(objectKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if operation != "download" && operation != "upload" {
		err = fmt.Errorf("invalid operation: %s, must be 'download' or 'upload'", operation)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info to retrieve ConnectionName and Uid
	resourceType := model.StrObjectStorage
	objStrgData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}
	objStrgInfo := objStrgData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := objStrgInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	connName := objStrgInfo.ConnectionName
	uid := objStrgInfo.Uid

	// 3. Call Spider API to generate the presigned URL
	client := clientManager.NewHttpClient()
	method := "GET"
	spReq := clientManager.NoBody
	spResp := spiderPreSignedUrlResponse{}

	url := fmt.Sprintf("%s/s3/presigned/%s/%s/%s?ConnectionName=%s&expires=%d",
		model.SpiderRestUrl, operation, uid, objectKey, connName, int64(expires.Seconds()))
	log.Debug().Msgf("[Request to Spider] Generating presigned URL (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Generating presigned URL: %+v", spResp)

	// 4. Return the presigned URL
	return model.ObjectStoragePresignedUrlResponse{
		Expires:      spResp.Expires,
		Method:       spResp.Method,
		PreSignedURL: spResp.PreSignedURL,
	}, nil
}

// ListDataObjects lists the objects in the specified object storage (bucket)
func ListDataObjects(nsId, osId string) (model.ObjectStorageListObjectsResponse, error) {
	var emptyRet model.ObjectStorageListObjectsResponse

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Get the object storage info by calling GetObjectStorage
	osInfo, err := GetObjectStorage(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}

	// 3. Return the list of objects
	res := model.ObjectStorageListObjectsResponse{}

	if osInfo.Contents == nil {
		res.Objects = []model.Object{}
	} else {
		res.Objects = osInfo.Contents
	}

	return res, nil
}

// GetDataObject retrieves a specific object from the specified object storage (bucket)
func GetDataObject(nsId, osId, objectKey string) (model.Object, error) {
	var emptyRet model.Object

	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = checkObjectKey(objectKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Check if the object storage exists
	resourceType := model.StrObjectStorage
	osData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return emptyRet, err
	}

	osInfo := osData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := osInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	connName := osInfo.ConnectionName
	uid := osInfo.Uid

	// 3. Call Spider API to get the object info
	client := clientManager.NewHttpClient()
	method := "HEAD"
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody

	url := fmt.Sprintf("%s/s3/%s/%s?ConnectionName=%s", model.SpiderRestUrl, uid, objectKey, connName)
	log.Debug().Msgf("[Request to Spider] Getting the object info (url: %s)", url)

	restyRes, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("[Response from Spider] Getting the object info (No response body): %+v", spResp)

	eTag := restyRes.Header().Get("ETag")
	lastModified := restyRes.Header().Get("Last-Modified")

	// 4. Since Spider does not return object metadata in the HEAD response,
	// we will return an empty object with just the key set.
	obj := model.Object{
		Key:          objectKey,
		ETag:         eTag,
		LastModified: lastModified,
	}

	return obj, nil
}

// DeleteDataObject deletes a specific object from the specified object storage (bucket)
func DeleteDataObject(nsId, osId, objectKey string) error {
	// 1. Validate input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = common.CheckString(osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	err = checkObjectKey(objectKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// 2. Get the object storage
	resourceType := model.StrObjectStorage
	osData, err := GetResource(nsId, resourceType, osId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, object storage: %s", osId)
		return err
	}
	osInfo := osData.(model.ObjectStorageInfo)

	// Check if the input CSP is supported for object storage
	cspType := osInfo.ConnectionConfig.ProviderName
	if !isObjectStorageSupported(cspType) {
		err = fmt.Errorf("object storage is not supported for CSP: %s", cspType)
		log.Error().Err(err).Msg("")
		return err
	}

	connName := osInfo.ConnectionName
	uid := osInfo.Uid

	// 3. Call Spider API to delete the object
	client := clientManager.NewHttpClient()
	method := "DELETE"
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody

	url := fmt.Sprintf("%s/s3/%s/%s?ConnectionName=%s", model.SpiderRestUrl, uid, objectKey, connName)
	log.Debug().Msgf("[Request to Spider] Deleting the object (url: %s)", url)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		spiderS3JSONHeaders,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Debug().Msgf("[Response from Spider] Deleting the object (No response body): %+v", spResp)

	return nil
}
