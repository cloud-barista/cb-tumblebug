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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

type ObjectStorageStatus string

const (
	// CRUD operations
	ObjectStorageOnConfiguring ObjectStorageStatus = "Configuring" // The object storage is being configured.
	ObjectStorageOnDeleting    ObjectStorageStatus = "Deleting"    // The object storage is being deleted.

	// Available status
	ObjectStorageAvailable ObjectStorageStatus = "Available" // The object storage is fully created and ready for use.

	// Error Handling
	ObjectStorageError              ObjectStorageStatus = "Error"              // An error occurred during a CRUD operation.
	ObjectStorageErrorOnConfiguring ObjectStorageStatus = "ErrorOnConfiguring" // An error occurred during the configuring operation.
	ObjectStorageErrorOnDeleting    ObjectStorageStatus = "ErrorOnDeleting"    // An error occurred during the deleting operation.
)

// ========== Resource APIs: Object Storage ==========

// spiderListBucketRes represents the response structure from Spider for listing S3 buckets
type spiderListBucketRes struct {
	Owner   spiderOwner   `xml:"Owner" json:"owner"`
	Buckets spiderBuckets `xml:"Buckets" json:"buckets"`
}

// spiderOwner represents the owner information in S3 bucket list response
type spiderOwner struct {
	ID          string `xml:"ID" json:"ID" example:"aws-ap-northeast-2"`
	DisplayName string `xml:"DisplayName" json:"DisplayName" example:"aws-ap-northeast-2"`
}

// spiderBucket represents a single bucket in S3 bucket list response
type spiderBucket struct {
	Name         string `xml:"Name" json:"Name" example:"spider-test-bucket"`
	CreationDate string `xml:"CreationDate" json:"CreationDate" example:"2025-09-04T04:18:06Z"`
}

// spiderBuckets represents the collection of buckets in S3 bucket list response
type spiderBuckets struct {
	Bucket []spiderBucket `xml:"Bucket" json:"Bucket"`
}

// spiderGetBucketInfoRes represents a single bucket in S3 bucket list response
type spiderGetBucketInfoRes struct {
	Name         string         `xml:"Name" json:"Name" example:"spider-test-bucket"`
	Prefix       string         `xml:"Prefix" json:"Prefix" example:""`
	Marker       string         `xml:"Marker" json:"Marker" example:""`
	MaxKeys      int            `xml:"MaxKeys" json:"MaxKeys" example:"1000"`
	IsTruncated  bool           `xml:"IsTruncated" json:"IsTruncated" example:"false"`
	CreationDate string         `xml:"CreationDate" json:"CreationDate" example:"2025-09-04T04:18:06Z"`
	Contents     []spiderObject `xml:"Contents" json:"Contents"`
}

// spiderObject represents a single object in the S3 bucket
type spiderObject struct {
	Key          string `xml:"Key" json:"Key" example:"test-object.txt"`
	LastModified string `xml:"LastModified" json:"LastModified" example:"2025-09-04T04:18:06Z"`
	ETag         string `xml:"ETag" json:"ETag" example:"9b2cf535f27731c974343645a3985328"`
	Size         int64  `xml:"Size" json:"Size" example:"1024"`
	StorageClass string `xml:"StorageClass" json:"StorageClass" example:"STANDARD"`
}

// spiderObjectStorageCreateRequest represents the request structure to create an S3 bucket in Spider
type spiderObjectStorageCreateRequest struct {
	BucketName     string `xml:"BucketName" json:"BucketName" validate:"required" example:"globally-unique-bucket-name-12345"`
	ConnectionName string `xml:"ConnectionName" json:"ConnectionName" validate:"required" example:"aws-ap-northeast-2"`
}

type spiderObjectStorageLocationResponse struct {
	LocationConstraint string `xml:"LocationConstraint" json:"LocationConstraint" example:"ap-northeast-2"`
}

// CreateObjectStorage creates a new object storage (bucket) in the specified namespace
func CreateObjectStorage(nsId string, req model.ObjectStorageCreateRequest) (model.ObjectStorageInfo, error) {

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

	// todo ? restore the tag list later
	// objStrgInfo.TagList = req.TagList

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

	// 6. Set and store status to the key-value store
	objStrgInfo.Status = string(ObjectStorageOnConfiguring)
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
			nil,
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
					return emptyRet, err
				}
				log.Warn().Msgf("Conflict detected for bucket name %s, retrying... (%d/%d)", spReq.BucketName, retryCount, maxRetries)
				continue
			} else {
				log.Error().Err(err).Msg("")
				return emptyRet, err
			}
		}

		log.Debug().Msgf("[Response from Spider] Creating a object storage (No response body): %+v", spResp)
		break
	}
	// Set the final values after successful creation
	objStrgInfo.Uid = uid

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
		nil,
		clientManager.SetUseBody(spGetBucketInfoReq),
		&spGetBucketInfoReq,
		&spGetBucketInfoRes,
		clientManager.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("[Response from Spider] Getting the created object storage info: %+v", spGetBucketInfoRes)

	// 9. Set the object storage info
	// TODO: Set CspResourceName and CspResourceId if available from Spider response
	// objStrgInfo.CspResourceName = spGetBucketInfoRes.IId.NameId
	// objStrgInfo.CspResourceId = spGetBucketInfoRes.IId.SystemId
	objStrgInfo.Prefix = spGetBucketInfoRes.Prefix
	objStrgInfo.Marker = spGetBucketInfoRes.Marker
	objStrgInfo.MaxKeys = spGetBucketInfoRes.MaxKeys
	objStrgInfo.IsTruncated = spGetBucketInfoRes.IsTruncated
	objStrgInfo.CreationDate = spGetBucketInfoRes.CreationDate

	var contents []model.Object
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

	// 10. Store the object storage info to the key-value store
	objStrgInfo.Status = string(ObjectStorageAvailable)
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
		model.LabelStatus:          objStrgInfo.Status,
		model.LabelDescription:     objStrgInfo.Description,
		model.LabelConnectionName:  objStrgInfo.ConnectionName,
	}

	err = label.CreateOrUpdateLabel(model.StrObjectStorage, objStrgInfo.Uid, objStrgKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 13. Return the object storage info
	return objStrgInfo, nil
}

// // ListObjectStorages retrieves the list of object storages (buckets) from the specified namespace
// func ListObjectStorages(nsId string) (model.ObjectStorageListResponse, error) {

// 	var emptyRet model.ObjectStorageListResponse

// 	// 1. Validate input parameters
// 	err := common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = validate.Struct(req)
// 	if err != nil {
// 		if _, ok := err.(*validator.InvalidValidationError); ok {
// 			log.Error().Err(err).Msg("")
// 			return emptyRet, err
// 		}
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	_, err = common.GetConnConfig(req.ConnectionName)
// 	if err != nil {
// 		err = fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// 2. Call Spider API to list the object storages
// 	client := clientManager.NewHttpClient()
// 	method := "GET"
// 	spReq := clientManager.NoBody
// 	spResp := spiderListBucketRes{}

// 	url := fmt.Sprintf("%s/s3?ConnectionName=%s", model.SpiderRestUrl, req.ConnectionName)
// 	log.Debug().Msgf("[Request to Spider] Listing S3 buckets (url: %s)", url)

// 	_, err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(spReq),
// 		&spReq,
// 		&spResp,
// 		clientManager.ShortDuration,
// 	)

// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("[Response from Spider] Listing S3 buckets: %+v", spResp)

// 	// 3. Convert spiderListBucketRes to model.ObjectStorageListResponse
// 	var buckets model.Buckets
// 	for _, spBucket := range spResp.Buckets.Bucket {
// 		bucket := model.Bucket{
// 			Name:         spBucket.Name,
// 			CreationDate: spBucket.CreationDate,
// 		}
// 		buckets.Bucket = append(buckets.Bucket, bucket)
// 	}

// 	listResp := model.ObjectStorageListResponse{
// 		Owner: model.Owner{
// 			ID:          spResp.Owner.ID,
// 			DisplayName: spResp.Owner.DisplayName,
// 		},
// 		Buckets: buckets,
// 	}

// 	return listResp, nil
// }

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
		nil,
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
	newObjStrgInfo.Prefix = spResp.Prefix
	newObjStrgInfo.Marker = spResp.Marker
	newObjStrgInfo.MaxKeys = spResp.MaxKeys
	newObjStrgInfo.IsTruncated = spResp.IsTruncated
	newObjStrgInfo.CreationDate = spResp.CreationDate

	var contents []model.Object
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

func DeleteObjectStorage(nsId, osId string) error {

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

	// 4. Set and store status
	objStrgInfo.Status = string(ObjectStorageOnDeleting)
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

	// 5. Call Spider API to delete the object storage
	client := clientManager.NewHttpClient()
	method := "DELETE"
	spReq := clientManager.NoBody
	spResp := clientManager.NoBody

	url := fmt.Sprintf("%s/s3/%s?ConnectionName=%s", model.SpiderRestUrl, uid, connName)

	maxRetries := 2
	t := uint64(3)
	for i := 0; i <= maxRetries; i++ {

		log.Debug().Msgf("[Request to Spider] Deleting the object storage (url: %s)", url)

		_, err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(spReq),
			&spReq,
			&spResp,
			clientManager.ShortDuration,
		)

		if err == nil {
			break
		}

		if i < maxRetries {
			log.Warn().Err(err).Msgf("Failed to delete object storage, retrying... (%d/%d)", i+1, maxRetries)
			// Sleep for a while before retrying
			time.Sleep(time.Duration(t) * time.Second)
		} else {
			log.Error().Err(err).Msgf("Failed to delete object storage after %d retries", maxRetries)
			return err
		}
	}

	log.Debug().Msgf("[Response from Spider] Deleting the object storage (No response body): %+v", spResp)

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
		nil,
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
		nil,
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
