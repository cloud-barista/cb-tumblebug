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
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	"github.com/go-resty/resty/v2"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"reflect"

	validator "github.com/go-playground/validator/v10"

	"github.com/rs/zerolog/log"
)

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

// getResourceConnectionName extracts the connection name for a given resource.
// This function is used to group resources by their CSP connection for semaphore-based processing.
func getResourceConnectionName(nsId, resourceType, resourceId string) (string, error) {
	// For performance, try to extract connection name from resourceId pattern first
	// Many resources follow the pattern: {connectionName}-{resourceName}
	parts := strings.Split(resourceId, "-")
	if len(parts) >= 2 {
		// Quick validation: check if the first part looks like a connection name
		potentialConnName := parts[0]
		if len(potentialConnName) > 0 && potentialConnName != "shared" {
			return potentialConnName, nil
		}
	}

	// Fall back to KV store lookup if pattern extraction fails
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		// If KV lookup fails, use pattern-based fallback
		if len(parts) >= 2 {
			return parts[0], nil
		}
		return "unknown", err
	}

	// Parse the JSON value to extract connection name
	switch resourceType {
	case model.StrSpec:
		var resource model.TbSpecInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrImage:
		var resource model.TbImageInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrCustomImage:
		var resource model.TbCustomImageInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrSSHKey:
		var resource model.TbSshKeyInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrSecurityGroup:
		var resource model.TbSecurityGroupInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrVNet:
		var resource model.TbVNetInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrSubnet:
		var resource model.TbSubnetInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	case model.StrDataDisk:
		var resource model.TbDataDiskInfo
		err = json.Unmarshal([]byte(keyValue.Value), &resource)
		if err != nil {
			return "unknown", err
		}
		return resource.ConnectionName, nil

	default:
		// For unsupported resource types, use pattern-based extraction
		if len(parts) >= 2 {
			return parts[0], nil
		}
		return "unknown", fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func init() {

	validate = validator.New()

	// register function to get tag name from json tags.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// register validation for 'Tb*Req'
	// NOTE: only have to register a non-pointer type for 'Tb*Req', validator
	// internally dereferences during it's type checks.
	validate.RegisterStructValidation(TbDataDiskReqStructLevelValidation, model.TbDataDiskReq{})
	validate.RegisterStructValidation(TbImageReqStructLevelValidation, model.TbImageReq{})
	validate.RegisterStructValidation(TbCustomImageReqStructLevelValidation, model.TbCustomImageReq{})
	validate.RegisterStructValidation(TbSecurityGroupReqStructLevelValidation, model.TbSecurityGroupReq{})
	validate.RegisterStructValidation(TbSpecReqStructLevelValidation, model.TbSpecReq{})
	validate.RegisterStructValidation(TbSshKeyReqStructLevelValidation, model.TbSshKeyReq{})
	validate.RegisterStructValidation(TbVNetReqStructLevelValidation, model.TbVNetReq{})
}

// DelAllResources deletes all TB Resource objects of the given resourceType.
func DelAllResources(nsId string, resourceType string, subString string, forceFlag string) (model.IdList, error) {
	var resultList []string
	var mutex sync.Mutex  // Protect shared slice access
	var wg sync.WaitGroup // Synchronize all goroutines

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.IdList{IdList: resultList}, err
	}

	resourceIdList, err := ListResourceId(nsId, resourceType)
	if err != nil {
		return model.IdList{IdList: resultList}, err
	}

	if len(resourceIdList) == 0 {
		errString := fmt.Sprintf("There is no %s resource in %s", resourceType, nsId)
		err := fmt.Errorf(errString)
		log.Error().Err(err).Msg("")
		return model.IdList{IdList: resultList}, err
	}

	// Channel to capture errors
	errChan := make(chan error, len(resourceIdList))
	var errChanClosed int32 // atomic flag to track if channel is closed

	// Group resources by CSP connection to apply per-CSP semaphore
	connectionGroups := make(map[string][]string)

	// Group resources by their connection configuration
	for _, resourceId := range resourceIdList {
		// Check if the resourceId matches the subString criteria
		if subString != "" && !strings.Contains(resourceId, subString) {
			continue
		}

		// Get connection name for this resource (optimized to reduce KV calls)
		connectionName, err := getResourceConnectionName(nsId, resourceType, resourceId)
		if err != nil {
			log.Warn().Err(err).Str("resourceId", resourceId).Msg("Failed to get connection name, using default group")
			connectionName = "unknown"
		}

		connectionGroups[connectionName] = append(connectionGroups[connectionName], resourceId)
	}

	// Create semaphores for each connection (limit concurrent operations per CSP)
	const maxConcurrentPerCSP = 20
	connectionSemaphores := make(map[string]chan struct{})
	totalResources := 0
	for connectionName := range connectionGroups {
		connectionSemaphores[connectionName] = make(chan struct{}, maxConcurrentPerCSP)
		totalResources += len(connectionGroups[connectionName])
		log.Info().Msgf("Connection %s: %d resources", connectionName, len(connectionGroups[connectionName]))
	}

	log.Info().Msgf("Starting deletion of %d resources across %d connections", totalResources, len(connectionGroups))

	// Process ALL connection groups in parallel (not sequentially!)
	for connectionName, resourceIds := range connectionGroups {
		// Pre-increment WaitGroup counter for all resources in this connection group
		for range resourceIds {
			wg.Add(1)
		}

		// Launch a goroutine for each connection group to process in parallel
		go func(connName string, resourceList []string, semaphore chan struct{}) {
			log.Info().Msgf("Starting parallel deletion for connection %s with %d resources (max concurrent: %d)",
				connName, len(resourceList), maxConcurrentPerCSP)

			// Process each resource in this connection group
			for _, resourceId := range resourceList {
				// Launch a goroutine for each resource deletion
				go func(resourceId string) {
					defer wg.Done()

					// Acquire semaphore (limit concurrent operations for this CSP)
					semaphore <- struct{}{}
					defer func() { <-semaphore }() // Release semaphore when done

					startTime := time.Now()
					log.Debug().Msgf("Starting deletion of %s:%s (connection: %s)", resourceType, resourceId, connName)

					// Minimal random sleep to avoid thundering herd (reduced significantly)
					common.RandomSleep(0, 1)

					// Attempt to delete the resource
					deleteStatus := "[Done] "
					errString := ""

					err := DelResource(nsId, resourceType, resourceId, forceFlag)
					if err != nil {
						deleteStatus = "[Failed] "
						errString = " (" + err.Error() + ")"

						// Safe error channel send - check if channel is still open
						if atomic.LoadInt32(&errChanClosed) == 0 {
							select {
							case errChan <- err:
								// Successfully sent error to channel
							case <-time.After(10 * time.Millisecond):
								// Channel is likely blocked, skip sending
							default:
								// Channel is full, skip sending
							}
						}
					}

					// Safely append the result to resultList using mutex
					mutex.Lock()
					resultList = append(resultList, deleteStatus+resourceType+": "+resourceId+errString)
					mutex.Unlock()

					elapsedTime := time.Since(startTime)
					log.Debug().Str("connectionName", connName).Str("resourceId", resourceId).
						Str("status", deleteStatus).Dur("elapsed", elapsedTime).Msg("Resource deletion completed")
				}(resourceId)
			}
		}(connectionName, resourceIds, connectionSemaphores[connectionName])
	}

	// Wait for all goroutines to complete
	log.Info().Msgf("Waiting for %d resource deletion tasks to complete", totalResources)
	wg.Wait()
	log.Info().Msgf("All %d resource deletion tasks completed", totalResources)

	// Safely close the error channel with atomic flag
	if atomic.CompareAndSwapInt32(&errChanClosed, 0, 1) {
		close(errChan)
	}

	// Collect any errors from the error channel
	for err := range errChan {
		if err != nil {
			log.Info().Err(err).Msg("error deleting resource")
		}
	}

	log.Info().Msgf("DelAllResources completed. Total results: %d", len(resultList))
	for i, result := range resultList {
		log.Debug().Msgf("Result %d: %s", i, result)
	}

	// Sort the results for consistent output ordering
	sort.Strings(resultList)

	// Create a simple response without mutex to avoid JSON serialization issues
	response := model.IdList{
		IdList: resultList,
	}

	return response, nil
}

// DelResource deletes the TB Resource object
func DelResource(nsId string, resourceType string, resourceId string, forceFlag string) error {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, err := CheckResource(nsId, resourceType, resourceId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	keyValue, _ := kvstore.GetKv(key)
	// In CheckResource() above, calling 'kvstore.GetKv()' and checking err parts exist.
	// So, in here, we don't need to check whether keyValue == nil or err != nil.

	/* Disabled the deletion protection feature
	associatedList, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
	if len(associatedList) == 0 {
		// continue
	} else {
		errString := " [Failed]" + " Associated with [" + strings.Join(associatedList[:], ", ") + "]"
		err := fmt.Errorf(errString)
		log.Error().Err(err).Msg("")
		return err
	}
	*/

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	var childResources interface{}

	var url string
	uid := ""

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	switch resourceType {
	case model.StrImage:
		// delete image info
		err := kvstore.Delete(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
		result := model.ORM.Delete(&model.TbImageInfo{}, "namespace = ? AND id = ?", nsId, resourceId)
		if result.Error != nil {
			fmt.Println(result.Error.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}

		return nil
	case model.StrCustomImage:
		temp := model.TbCustomImageInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/myimage/" + temp.CspResourceName
		uid = temp.Uid

		/*
			// delete image info
			err := kvstore.Delete(key)
			if err != nil {
				log.Error().Err(err).Msg("")
				return err
			}

			// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
			_, err = model.ORM.Delete(&TbCustomImageInfo{Namespace: nsId, Id: resourceId})
			if err != nil {
				fmt.Println(err.Error())
			} else {
				log.Debug().Msg("Data deleted successfully..")
			}

			return nil
		*/
	case model.StrSpec:
		// delete spec info

		//get related recommend spec
		//keyValue, err := kvstore.GetKv(key)
		// content := TbSpecInfo{}
		// err := json.Unmarshal([]byte(keyValue.Value), &content)
		// if err != nil {
		// 	log.Error().Err(err).Msg("")
		// 	return err
		// }

		// err = kvstore.Delete(key)
		// if err != nil {
		// 	log.Error().Err(err).Msg("")
		// 	return err
		// }

		// "DELETE FROM `spec` WHERE `id` = '" + resourceId + "';"
		result := model.ORM.Delete(&model.TbCustomImageInfo{}, "namespace = ? AND id = ?", nsId, resourceId)
		if result.Error != nil {
			fmt.Println(result.Error.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}

		return nil
	case model.StrSSHKey:
		temp := model.TbSshKeyInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/keypair/" + temp.CspResourceName
		uid = temp.Uid

	case model.StrVNet:
		temp := model.TbVNetInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/vpc/" + temp.CspResourceName
		childResources = temp.SubnetInfoList
		uid = temp.Uid

	case model.StrSecurityGroup:
		temp := model.TbSecurityGroupInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/securitygroup/" + temp.CspResourceName
		uid = temp.Uid

	case model.StrDataDisk:
		temp := model.TbDataDiskInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = model.SpiderRestUrl + "/disk/" + temp.CspResourceName
		uid = temp.Uid
	/*
		case "subnet":
			temp := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspResourceId
		case "publicIp":
			temp := publicIpInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			requestBody.ConnectionName = temp.ConnectionName
			url = common.TB_SPIDER_REST_URL + "/publicip/" + temp.CspPublicIpName
		case "vNic":
			temp := vNicInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			requestBody.ConnectionName = temp.ConnectionName
			url = common.TB_SPIDER_REST_URL + "/vnic/" + temp.CspVNicName
	*/
	default:
		err := fmt.Errorf("invalid resourceType")
		return err
	}

	if forceFlag == "true" {
		url += "?force=true"
	}
	var callResult interface{}
	client := resty.New()
	method := "DELETE"
	//client.SetTimeout(60 * time.Second)

	log.Debug().Msg("Sending DELETE request to " + url)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	log.Debug().Msg("Deleting request finished from " + url)

	if strings.EqualFold(resourceType, model.StrVNet) {
		// var subnetKeys []string
		subnets := childResources.([]model.TbSubnetInfo)
		for _, v := range subnets {
			subnetKey := common.GenChildResourceKey(nsId, model.StrSubnet, resourceId, v.Id)
			// subnetKeys = append(subnetKeys, subnetKey)
			err = kvstore.Delete(subnetKey)
			if err != nil {
				log.Error().Err(err).Msg("")
				// return err
			}

			err = label.DeleteLabelObject(resourceType, v.Uid)
			if err != nil {
				log.Error().Err(err).Msg("")
			}

		}
	} else if strings.EqualFold(resourceType, model.StrCustomImage) {
		// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
		result := model.ORM.Delete(&model.TbSpecInfo{}, "namespace = ? AND id = ?", nsId, resourceId)
		if result.Error != nil {
			fmt.Println(result.Error.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}
	}

	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = label.DeleteLabelObject(resourceType, uid)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	return nil
}

// DelEleInSlice delete an element from slice by index
//   - arr: the reference of slice
//   - index: the index of element will be deleted
func DelEleInSlice(arr interface{}, index int) {
	vField := reflect.ValueOf(arr)
	value := vField.Elem()
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		result := reflect.AppendSlice(value.Slice(0, index), value.Slice(index+1, value.Len()))
		value.Set(result)
	}
}

// ListResourceId returns the list of TB Resource object IDs of given resourceType
func ListResourceId(nsId string, resourceType string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) {
		// continue
	} else {
		err = fmt.Errorf("invalid resource type")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	key := "/ns/" + nsId + "/resources/"
	keyValue, err := kvstore.GetKvList(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	/* if keyValue == nil, then for-loop below will not be executed, and the empty array will be returned in `resourceList` placeholder.
	if keyValue == nil {
		err = fmt.Errorf("ListResourceId(); %s is empty.", key)
		log.Error().Err(err).Msg("")
		return nil, err
	}
	*/

	var resourceList []string
	for _, v := range keyValue {
		trimmedString := strings.TrimPrefix(v.Key, (key + resourceType + "/"))
		// prevent malformed key (if key for resource id includes '/', the key does not represent resource ID)
		if !strings.Contains(trimmedString, "/") {
			resourceList = append(resourceList, trimmedString)
		}
	}

	return resourceList, nil

}

// ListResource returns the list of TB Resource objects of given resourceType
func ListResource(nsId string, resourceType string, filterKey string, filterVal string) (interface{}, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) {
		// continue
	} else {
		errString := "Cannot list " + resourceType + "s."
		err := fmt.Errorf(errString)
		return nil, err
	}

	//log.Debug().Msg("[Get] " + resourceType + " list")
	key := "/ns/" + nsId + "/resources/" + resourceType
	//log.Debug().Msg(key)

	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		switch resourceType {
		case model.StrImage:
			res := []model.TbImageInfo{}
			for _, v := range keyValue {

				tempObj := model.TbImageInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrCustomImage:
			res := []model.TbCustomImageInfo{}
			for _, v := range keyValue {

				tempObj := model.TbCustomImageInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}

				// Update TB CustomImage object's 'status' field
				// Just calling GetResource(customImage) once will update TB CustomImage object's 'status' field
				newObj, err := GetResource(nsId, model.StrCustomImage, tempObj.Id)
				// do not return here to gather whole list. leave error message in the return body.
				if newObj != nil {
					tempObj = newObj.(model.TbCustomImageInfo)
				} else {
					tempObj.Id = tempObj.Id
				}
				if err != nil {
					log.Error().Err(err).Msg("")
					tempObj.Description = err.Error()
					tempObj.Status = "Error"
				}

				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrSecurityGroup:
			res := []model.TbSecurityGroupInfo{}
			for _, v := range keyValue {
				tempObj := model.TbSecurityGroupInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrSpec:
			res := []model.TbSpecInfo{}
			for _, v := range keyValue {
				tempObj := model.TbSpecInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrSSHKey:
			res := []model.TbSshKeyInfo{}
			for _, v := range keyValue {
				tempObj := model.TbSshKeyInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrVNet:
			res := []model.TbVNetInfo{}
			for _, v := range keyValue {
				tempObj := model.TbVNetInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		case model.StrDataDisk:
			res := []model.TbDataDiskInfo{}
			for _, v := range keyValue {
				tempObj := model.TbDataDiskInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}

				// Update TB DataDisk object's 'status' field
				// Just calling GetResource(dataDisk) once will update TB DataDisk object's 'status' field
				newObj, err := GetResource(nsId, model.StrDataDisk, tempObj.Id)
				if err != nil {
					log.Error().Err(err).Msg("")
					tempObj.Status = "Failed"
					tempObj.SystemMessage = err.Error()
				} else {
					tempObj = newObj.(model.TbDataDiskInfo)
				}

				// Check the JSON body inclues both filterKey and filterVal strings. (assume key and value)
				if filterKey != "" {
					// If not inclues both, do not append current item to the list result.
					itemValueForCompare := strings.ToLower(v.Value)
					if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) && strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
						continue
					}
				}
				res = append(res, tempObj)
			}
			return res, nil
		}

	} else { //return empty object according to resourceType
		switch resourceType {
		case model.StrImage:
			return []model.TbImageInfo{}, nil
		case model.StrCustomImage:
			return []model.TbCustomImageInfo{}, nil
		case model.StrSecurityGroup:
			return []model.TbSecurityGroupInfo{}, nil
		case model.StrSpec:
			return []model.TbSpecInfo{}, nil
		case model.StrSSHKey:
			return []model.TbSshKeyInfo{}, nil
		case model.StrVNet:
			return []model.TbVNetInfo{}, nil
		case model.StrDataDisk:
			return []model.TbDataDiskInfo{}, nil
		}
	}

	err = fmt.Errorf("Some exceptional case happened. Please check the references of " + common.GetFuncName())
	return nil, err // if interface{} == nil, make err be returned. Should not come this part if there is no err.
}

// GetAssociatedObjectCount returns the number of Resource's associated Tumblebug objects
func GetAssociatedObjectCount(nsId string, resourceType string, resourceId string) (int, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}
	check, err := CheckResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return -1, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		inUseCount := int(gjson.Get(keyValue.Value, "associatedObjectList.#").Int())
		return inUseCount, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return -1, err
}

// GetAssociatedObjectList returns the list of Resource's associated Tumblebug objects
func GetAssociatedObjectList(nsId string, resourceType string, resourceId string) ([]string, error) {

	var result []string

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	check, err := CheckResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return nil, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != (kvstore.KeyValue{}) {

		type stringList struct {
			AssociatedObjectList []string `json:"associatedObjectList"`
		}
		res := stringList{}
		err = json.Unmarshal([]byte(keyValue.Value), &res)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		result = res.AssociatedObjectList

		return result, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return nil, err
}

// UpdateAssociatedObjectList adds or deletes the objectKey (currently, vmKey) to/from TB object's associatedObjectList
func UpdateAssociatedObjectList(nsId string, resourceType string, resourceId string, cmd string, objectKey string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// err = common.CheckString(resourceId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return nil, err
	// }
	/*
		check, err := CheckResource(nsId, resourceType, resourceId)

		if !check {
			errString := "The " + resourceType + " " + resourceId + " does not exist."
			err := fmt.Errorf(errString)
			return -1, err
		}

		if err != nil {
			log.Error().Err(err).Msg("")
			return -1, err
		}
	*/
	log.Trace().Msg("[Set count] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if keyValue != (kvstore.KeyValue{}) {
		objList, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
		switch cmd {
		case model.StrAdd:
			for _, v := range objList {
				if v == objectKey {
					errString := objectKey + " is already associated with " + resourceType + " " + resourceId + "."
					err = fmt.Errorf(errString)
					return nil, err
				}
			}
			var anyJson map[string]interface{}
			json.Unmarshal([]byte(keyValue.Value), &anyJson)
			if anyJson["associatedObjectList"] == nil {
				arrayToBe := []string{objectKey}

				anyJson["associatedObjectList"] = arrayToBe
			} else { // anyJson["associatedObjectList"] != nil
				arrayAsIs := anyJson["associatedObjectList"].([]interface{})

				arrayToBe := append(arrayAsIs, objectKey)

				anyJson["associatedObjectList"] = arrayToBe
			}
			updatedJson, _ := json.Marshal(anyJson)

			keyValue.Value = string(updatedJson)
		case model.StrDelete:
			var foundKey int
			var foundVal string
			for k, v := range objList {
				if v == objectKey {
					foundKey = k
					foundVal = v
					break
				}
			}
			if foundVal == "" {
				errString := "Cannot find the associated object " + objectKey + "."
				err = fmt.Errorf(errString)
				return nil, err
			} else {
				keyValue.Value, err = sjson.Delete(keyValue.Value, "associatedObjectList."+strconv.Itoa(foundKey))
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}
			}
		}

		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}
		err = kvstore.Put(key, keyValue.Value)
		if err != nil {
			log.Error().Err(err).Msg("")
			return nil, err
		}

		result, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
		return result, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return nil, err
}

// GetResource returns the requested TB Resource object
func GetResource(nsId string, resourceType string, resourceId string) (interface{}, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// err = common.CheckString(resourceId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return nil, err
	// }
	check, err := CheckResource(nsId, resourceType, resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if !check {
		errString := fmt.Sprintf("The %s %s does not exist.", resourceType, resourceId)
		err := fmt.Errorf(errString)
		return nil, err
	}

	log.Trace().Msg("[Get resource] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := kvstore.GetKv(key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		switch resourceType {
		case model.StrImage:
			res := model.TbImageInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrCustomImage:
			res := model.TbCustomImageInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

			// Update TB CustomImage object's 'status' field
			url := fmt.Sprintf("%s/myimage/%s", model.SpiderRestUrl, res.CspResourceName)

			client := resty.New().SetCloseConnection(true)
			client.SetAllowGetMethodPayload(true)

			connectionName := model.SpiderConnectionName{
				ConnectionName: res.ConnectionName,
			}

			req := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(connectionName).
				SetResult(&model.SpiderMyImageInfo{}) // or SetResult(AuthSuccess{}).
				//SetError(&AuthError{}).       // or SetError(AuthError{}).

			resp, err := req.Get(url)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

			switch {
			case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
				err := fmt.Errorf(string(resp.Body()))
				log.Error().Err(err).Msg("")
				return nil, err
			}

			updatedSpiderMyImage := resp.Result().(*model.SpiderMyImageInfo)
			res.Status = updatedSpiderMyImage.Status
			UpdateResourceObject(nsId, model.StrCustomImage, res)

			return res, nil
		case model.StrSecurityGroup:
			res := model.TbSecurityGroupInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrSpec:
			res := model.TbSpecInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrSSHKey:
			res := model.TbSshKeyInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrVNet:
			res := model.TbVNetInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case model.StrDataDisk:
			res := model.TbDataDiskInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return res, err
			}

			// Update TB DataDisk object's 'status' field
			url := fmt.Sprintf("%s/disk/%s", model.SpiderRestUrl, res.CspResourceName)

			client := resty.New().SetCloseConnection(true)
			client.SetAllowGetMethodPayload(true)

			connectionName := model.SpiderConnectionName{
				ConnectionName: res.ConnectionName,
			}

			req := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(connectionName).
				SetResult(&model.SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
				//SetError(&AuthError{}).       // or SetError(AuthError{}).

			resp, err := req.Get(url)
			if err != nil {
				log.Error().Err(err).Msg("")
				return res, err
			}

			fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
			switch {
			case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
				err := fmt.Errorf(string(resp.Body()))
				fmt.Println("body: ", string(resp.Body()))
				log.Error().Err(err).Msg("")
				return res, err
			}

			updatedSpiderDisk := resp.Result().(*model.SpiderDiskInfo)
			res.Status = updatedSpiderDisk.Status
			fmt.Printf("res.Status: %s \n", res.Status) // for debug
			UpdateResourceObject(nsId, model.StrDataDisk, res)

			return res, nil
		}

		//return true, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return nil, err
}

// GenSpecMapKey generates a SpecMap key for storing or accessing data in a map
func GenSpecMapKey(region, specName string) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", region, specName))
}

// GenResourceKey generates a Resource key for concatenating providerName, regionName, zoneName, resourceName
func GetProviderRegionZoneResourceKey(providerName, regionName, zoneName, resourceName string) string {

	div := "+"

	if regionName == "" && zoneName == "" {
		return strings.ToLower(fmt.Sprintf("%s%s%s", providerName, div, resourceName))
	}

	if zoneName == "" {
		return strings.ToLower(fmt.Sprintf("%s%s%s%s%s", providerName, div, regionName, div, resourceName))
	}

	return strings.ToLower(fmt.Sprintf("%s%s%s%s%s%s%s", providerName, div, regionName, div, zoneName, div, resourceName))
}

// ResolveProviderRegionZoneResourceKey resolves the Resource key into providerName, regionName, zoneName, resourceName
func ResolveProviderRegionZoneResourceKey(key string) (providerName string, regionName string, zoneName string, resourceName string, err error) {

	div := "+"

	split := strings.Split(key, div)

	if len(split) == 1 {
		return "", "", "", "", fmt.Errorf("ResourceKey dose not contain div(%s)", div)
	}

	if len(split) == 2 {
		return split[0], "", "", split[1], nil
	}

	if len(split) == 3 {
		return split[0], split[1], "", split[2], nil
	}

	return split[0], split[1], split[2], split[3], nil
}

// CheckResource returns the existence of the TB Resource resource in bool form.
func CheckResource(nsId string, resourceType string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("failed to check resource, the given nsId is null")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("failed to check resource, the given resourceType is null")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("failed to check resource, the given resourceId is null")
		return false, err
	}

	// Check resourceType's validity
	if strings.EqualFold(resourceType, model.StrImage) ||
		strings.EqualFold(resourceType, model.StrCustomImage) ||
		strings.EqualFold(resourceType, model.StrSSHKey) ||
		strings.EqualFold(resourceType, model.StrSpec) ||
		strings.EqualFold(resourceType, model.StrVNet) ||
		strings.EqualFold(resourceType, model.StrVPN) ||
		strings.EqualFold(resourceType, model.StrSqlDB) ||
		strings.EqualFold(resourceType, model.StrObjectStorage) ||
		strings.EqualFold(resourceType, model.StrSecurityGroup) ||
		strings.EqualFold(resourceType, model.StrDataDisk) {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	// err = common.CheckString(resourceId)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return false, err
	// }

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		return true, nil
	}
	return false, nil

}

// CheckChildResource returns the existence of the TB Resource resource in bool form.
func CheckChildResource(nsId string, resourceType string, parentResourceId string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckResource failed; nsId given is null.")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return false, err
	} else if parentResourceId == "" {
		err := fmt.Errorf("CheckResource failed; parentResourceId given is null.")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("CheckResource failed; resourceId given is null.")
		return false, err
	}

	var parentResourceType string
	// Check resourceType's validity
	if strings.EqualFold(resourceType, model.StrSubnet) {
		parentResourceType = model.StrVNet
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(parentResourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	fmt.Printf("[Check child resource] %s, %s, %s", resourceType, parentResourceId, resourceId)

	key := common.GenResourceKey(nsId, parentResourceType, parentResourceId)
	key += "/" + resourceType + "/" + resourceId

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		return true, nil
	}
	return false, nil

}

/*
func convertSpiderResourceToTumblebugResource(resourceType string, i interface{}) (interface{}, error) {
	if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return nil, err
	}

	// Check resourceType's validity
	if resourceType == model.StrImage ||
		resourceType == model.StrSSHKey ||
		resourceType == model.StrSpec ||
		resourceType == model.StrVNet ||
		resourceType == model.StrSecurityGroup {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return nil, err
	}

}
*/

// https://stackoverflow.com/questions/45139954/dynamic-struct-as-parameter-golang

type IdNameOnly struct {
	Id   string
	Name string
}

// GetIdFromStruct accepts any struct for argument, and returns value of the field 'Id'
func GetIdFromStruct(u interface{}) (string, error) {
	jsonInByteStream, err := json.Marshal(u)
	if err != nil {
		return "", err
	}

	idStruct := IdNameOnly{}
	json.Unmarshal(jsonInByteStream, &idStruct)

	return idStruct.Id, nil
}

// GetNameFromStruct accepts any struct for argument, and returns value of the field 'Name'
func GetNameFromStruct(u interface{}) (string, error) {
	jsonInByteStream, err := json.Marshal(u)
	if err != nil {
		return "", err
	}

	idStruct := IdNameOnly{}
	json.Unmarshal(jsonInByteStream, &idStruct)

	return idStruct.Name, nil
}

// LoadAssets is to register common resources from asset files (../assets/*.csv)
func LoadAssets() (*model.IdList, error) {

	regiesteredIds := &model.IdList{}

	// Check common namespace. Create one if not.
	_, err := common.GetNs(model.SystemCommonNs)
	if err != nil {
		nsReq := model.NsReq{}
		nsReq.Name = model.SystemCommonNs
		nsReq.Description = "Namespace for common resources"
		_, nsErr := common.CreateNs(&nsReq)
		if nsErr != nil {
			log.Error().Err(nsErr).Msg("")
			return regiesteredIds, nsErr
		}
	}

	startTime := time.Now()

	reqBodySpecFetchOption := &model.SpecFetchOption{}

	resultFetchSpecsForAllConnConfigs, err := FetchSpecsForAllConnConfigs(model.SystemCommonNs, reqBodySpecFetchOption)
	if err != nil {
		log.Error().Err(err).Msg("FetchImagesForAllConnConfigs failed")
	}
	elapsedFetchSpec := time.Since(startTime)
	log.Debug().Msgf("resultFetchSpecsForAllConnConfigs.RegisteredSpecs: %+v elapsed: [%s]", resultFetchSpecsForAllConnConfigs.RegisteredSpecs, elapsedFetchSpec)

	startTime = time.Now()
	err = UpdateSpecsFromAsset(model.SystemCommonNs)
	if err != nil {
		log.Error().Err(err).Msg("UpdateSpecsFromAsset failed")
	}
	elapsedUpdateSpec := time.Since(startTime)
	log.Info().Msgf("UpdateSpecsFromAsset. Elapsed [%s]", elapsedUpdateSpec)

	startTime = time.Now()

	reqBodyImageFetchOption := &model.ImageFetchOption{}
	reqBodyImageFetchOption.ExcludedProviders = []string{csp.Azure}
	reqBodyImageFetchOption.RegionAgnosticProviders = []string{csp.GCP, csp.Tencent}
	resultFetchImagesForAllConnConfigs, err := FetchImagesForAllConnConfigs(model.SystemCommonNs, reqBodyImageFetchOption)
	if err != nil {
		log.Error().Err(err).Msg("FetchImagesForAllConnConfigs failed")
	}
	elapsedFetchImg := time.Since(startTime)
	log.Debug().Msgf("resultFetchImagesForAllConnConfigs.RegisteredImages: %+v elapsed: [%s]", resultFetchImagesForAllConnConfigs.RegisteredImages, elapsedFetchImg)

	startTime = time.Now()
	resultUpdateImagesFromAsset, err := UpdateImagesFromAsset(model.SystemCommonNs)
	if err != nil {
		log.Error().Err(err).Msg("UpdateImagesFromAsset failed")
	}
	log.Debug().Msgf("resultUpdateImagesFromAsset: %+v", resultUpdateImagesFromAsset)

	elapsedUpdateImg := time.Since(startTime)

	// Force garbage collection for large cleanup
	runtime.GC()

	// waitSpecImg.Wait()
	// sort.Strings(regiesteredIds.IdList)
	//log.Info().Msgf("Registered Common Resources %d", len(regiesteredIds.IdList))

	log.Info().Msgf("Fetched Spec List. Elapsed [%s]", elapsedFetchSpec)
	log.Info().Msgf("Updated Spec List. Elapsed [%s]", elapsedUpdateSpec)
	log.Info().Msgf("Fetched Image List. Elapsed [%s]", elapsedFetchImg)
	log.Info().Msgf("Updated Image List. Elapsed [%s]", elapsedUpdateImg)

	// FetchPriceForAllConnConfigs is called to update the prices of all specs
	log.Info().Msgf("FetchPriceForAllConnConfigs is called to update the prices of all specs")
	// FetchPriceForAllConnConfigs() will be called in the end of this function in background
	//go FetchPriceForAllConnConfigs()

	return regiesteredIds, nil
}

// CreateSharedResource is to register default resource from asset files (../assets/*.csv)
func CreateSharedResource(nsId string, resType string, connectionName string) error {

	// Check 'nsId' namespace.
	_, err := common.GetNs(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	var resList []string
	if resType == "all" {
		resList = append(resList, model.StrVNet)
		resList = append(resList, model.StrSSHKey)
		resList = append(resList, model.StrSecurityGroup)
	} else {
		resList = append(resList, resType)
	}

	// TODO: This is a temporary solution. need to be changed after the policy is decided.
	credentialHolder := model.DefaultCredentialHolder

	// Read default resources from file and create objects

	connectionList, err := common.GetConnConfigList(credentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot GetConnConfig")
		return err
	}
	sliceIndex := -1
	provider := ""
	for i, connConfig := range connectionList.Connectionconfig {
		if connConfig.ConfigName == connectionName {
			log.Info().Msgf("[%d] connectionName: %s", i, connectionName)
			sliceIndex = i
			provider = strings.ToLower(connConfig.ProviderName)
		}
	}
	if sliceIndex == -1 {
		err := fmt.Errorf("cannot find the connection config: %s", connectionName)
		log.Error().Err(err).Msg("Failed to CreateSharedResource")
		return err
	}
	sliceIndex = (sliceIndex % 254) + 1

	//resourceName := connectionName
	// Default resource name has this pattern (nsId + "-shared-" + connectionName)
	resourceName := nsId + model.StrSharedResourceName + connectionName
	description := "Generated Default Resource"

	for _, resType := range resList {
		if strings.EqualFold(resType, model.StrVNet) {
			log.Debug().Msg(model.StrVNet)

			reqTmp := model.TbVNetReq{}
			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			// set isolated private address space for each cloud region (10.i.0.0/16)
			reqTmp.CidrBlock = "10." + strconv.Itoa(sliceIndex) + ".0.0/16"

			// Consist 2 subnets (10.i.0.0/18, 10.i.64.0/18)
			// Reserve spaces for tentative 2 subnets (10.i.128.0/18, 10.i.192.0/18)
			zones, length, _ := GetFirstNZones(connectionName, 2)
			subnetName := reqTmp.Name
			subnetCidr := "10." + strconv.Itoa(sliceIndex) + ".0.0/18"
			subnet := model.TbSubnetReq{Name: subnetName, IPv4_CIDR: subnetCidr}
			if length > 0 {
				subnet.Zone = zones[0]
			}
			reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)

			subnetName = reqTmp.Name + "-01"
			subnetCidr = "10." + strconv.Itoa(sliceIndex) + ".64.0/18"
			subnet = model.TbSubnetReq{Name: subnetName, IPv4_CIDR: subnetCidr}
			if length > 1 {
				subnet.Zone = zones[1]
				// ref IBM VPC Network structure: https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc&locale=en
				// IBM VPC Network requires Address Prefix setup for each zone. (but there is limitation in CB-Spider implementation.)
				// So, we will create 2 subnets in a zone within the limited address space.
				if provider == csp.IBM {
					subnet.Zone = zones[0]
				}
			}
			reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)

			common.PrintJsonPretty(reqTmp)

			resultInfo, err := CreateVNet(nsId, &reqTmp)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create vNet")
				return err
			}
			common.PrintJsonPretty(resultInfo)
		} else if strings.EqualFold(resType, model.StrSecurityGroup) {
			log.Debug().Msg(model.StrSecurityGroup)

			reqTmp := model.TbSecurityGroupReq{}

			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			reqTmp.VNetId = resourceName

			// open all firewall for default securityGroup
			var ruleList []model.TbFirewallRuleReq
			rule := model.TbFirewallRuleReq{Ports: "1-65535", Protocol: "tcp", Direction: "inbound", CIDR: "0.0.0.0/0"}
			ruleList = append(ruleList, rule)
			rule = model.TbFirewallRuleReq{Ports: "1-65535", Protocol: "udp", Direction: "inbound", CIDR: "0.0.0.0/0"}
			ruleList = append(ruleList, rule)
			// CloudIt only offers tcp, udp Protocols
			if !strings.EqualFold(provider, "cloudit") {
				rule = model.TbFirewallRuleReq{Protocol: "icmp", Direction: "inbound", CIDR: "0.0.0.0/0"}
				ruleList = append(ruleList, rule)
			}

			common.PrintJsonPretty(ruleList)
			reqTmp.FirewallRules = &ruleList

			common.PrintJsonPretty(reqTmp)

			resultInfo, err := CreateSecurityGroup(nsId, &reqTmp, "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to create SecurityGroup")
				return err
			}
			common.PrintJsonPretty(resultInfo)

		} else if strings.EqualFold(resType, model.StrSSHKey) {
			log.Debug().Msg(model.StrSSHKey)

			reqTmp := model.TbSshKeyReq{}

			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			common.PrintJsonPretty(reqTmp)

			_, err := CreateSshKey(nsId, &reqTmp, "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to create SshKey")
				return err
			}
			// common.PrintJsonPretty(resultInfo)
		} else {
			return errors.New("Not valid option (provide sg, sshkey, vnet, or all)")
		}
	}

	return nil
}

// DeleteSharedResources deletes all Default securityGroup, sshKey, vNet objects
func DeleteSharedResources(nsId string) (model.IdList, error) {

	output := model.IdList{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}

	matchedSubstring := nsId + model.StrSharedResourceName

	list, err := DelAllResources(nsId, model.StrSecurityGroup, matchedSubstring, "false")
	if err != nil {
		log.Error().Err(err).Msg("")
		output.IdList = append(output.IdList, err.Error())
	}
	output.IdList = append(output.IdList, list.IdList...)

	list, err = DelAllResources(nsId, model.StrSSHKey, matchedSubstring, "false")
	if err != nil {
		log.Error().Err(err).Msg("")
		output.IdList = append(output.IdList, err.Error())
	}
	output.IdList = append(output.IdList, list.IdList...)

	list, err = DelAllResources(nsId, model.StrVNet, matchedSubstring, "false")
	if err != nil {
		log.Error().Err(err).Msg("")
		output.IdList = append(output.IdList, err.Error())
	}
	output.IdList = append(output.IdList, list.IdList...)

	return output, nil
}

// ToNamingRuleCompatible function transforms a given string to match the regex pattern [a-z]([-a-z0-9]*[a-z0-9])?.
func ToNamingRuleCompatible(rawName string) string {
	// Convert all uppercase letters to lowercase
	rawName = strings.ToLower(rawName)

	// // Replace all non-alphanumeric characters with '-'
	// nonAlphanumericRegex := regexp.MustCompile(`[^a-z0-9]+`)
	// rawName = nonAlphanumericRegex.ReplaceAllString(rawName, "-")

	// // Remove leading and trailing '-' from the result string
	// trimLeadingTrailingDashRegex := regexp.MustCompile(`^-+|-+$`)
	// rawName = trimLeadingTrailingDashRegex.ReplaceAllString(rawName, "")

	return rawName
}

// UpdateResourceObject is func to update the resource object
func UpdateResourceObject(nsId string, resourceType string, resourceObject interface{}) {
	resourceId, err := GetIdFromStruct(resourceObject)
	if resourceId == "" || err != nil {
		log.Debug().Msgf("in UpdateResourceObject; failed to extract resourceId") // for debug
		return
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	// Check existence of the key. If no key, no update.
	keyValue, err := kvstore.GetKv(key)
	if keyValue == (kvstore.KeyValue{}) || err != nil {
		return
	}

	/*
		// Implementation 1
		oldJSON := keyValue.Value
		newJSON, err := json.Marshal(resourceObject)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		isEqualJSON, err := AreEqualJSON(oldJSON, string(newJSON))
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		if !isEqualJSON {
			err = kvstore.Put(key, string(newJSON))
			if err != nil {
				log.Error().Err(err).Msg("")
			}
		}
	*/

	// Implementation 2
	var oldObject interface{}
	err = json.Unmarshal([]byte(keyValue.Value), &oldObject)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	if !reflect.DeepEqual(oldObject, resourceObject) {
		val, _ := json.Marshal(resourceObject)
		err = kvstore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

}

func expandInfraType(infraType string) string {
	expInfraTypeList := []string{}
	lowerInfraType := strings.ToLower(infraType)

	if strings.Contains(lowerInfraType, model.StrVM) {
		expInfraTypeList = append(expInfraTypeList, model.StrVM)
	}
	if strings.Contains(lowerInfraType, model.StrK8s) ||
		strings.Contains(lowerInfraType, model.StrKubernetes) ||
		strings.Contains(lowerInfraType, model.StrContainer) {
		expInfraTypeList = append(expInfraTypeList, model.StrK8s)
		expInfraTypeList = append(expInfraTypeList, model.StrKubernetes)
		expInfraTypeList = append(expInfraTypeList, model.StrContainer)
	}

	return strings.Join(expInfraTypeList, "|")
}

// GetCspResourceName is func to retrieve CSP native resource ID
func GetCspResourceName(nsId string, resourceType string, resourceId string) (string, error) {

	if strings.EqualFold(resourceType, model.StrSpec) {
		specInfo, err := GetSpec(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return specInfo.CspSpecName, nil
	}
	if strings.EqualFold(resourceType, model.StrImage) {
		imageInfo, err := GetImage(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return imageInfo.CspImageName, nil
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalidKey" {
		return "", fmt.Errorf("invalid nsId or resourceType or resourceId")
	}
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	if keyValue == (kvstore.KeyValue{}) {
		//log.Error().Err(err).Msg("")
		// if there is no matched value for the key, return empty string. Error will be handled in a parent function
		return "", fmt.Errorf("cannot find the key " + key)
	}

	switch resourceType {
	case model.StrCustomImage:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil
	case model.StrSSHKey:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil
	case model.StrVNet:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil // contains CspResourceId
	case model.StrSecurityGroup:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil
	case model.StrDataDisk:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceName, nil

	default:
		return "", fmt.Errorf("invalid resourceType")
	}
}

// GetCspResourceId is func to retrieve CSP native resource ID (SystemId)
func GetCspResourceId(nsId string, resourceType string, resourceId string) (string, error) {

	if strings.EqualFold(resourceType, model.StrSpec) {
		specInfo, err := GetSpec(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return specInfo.CspSpecName, nil // For Spec, name and id are the same
	}
	if strings.EqualFold(resourceType, model.StrImage) {
		imageInfo, err := GetImage(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return imageInfo.CspImageName, nil // For Image, name and id are the same
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalidKey" {
		return "", fmt.Errorf("invalid nsId or resourceType or resourceId")
	}
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	if keyValue == (kvstore.KeyValue{}) {
		//log.Error().Err(err).Msg("")
		// if there is no matched value for the key, return empty string. Error will be handled in a parent function
		return "", fmt.Errorf("cannot find the key " + key)
	}

	// need to handle subnet in a different way

	switch resourceType {
	case model.StrCustomImage:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil // Return CspResourceId instead of CspResourceName
	case model.StrSSHKey:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil
	case model.StrVNet:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil
	case model.StrSecurityGroup:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil
	case model.StrDataDisk:
		content := model.ResourceIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspResourceId, nil

	default:
		return "", fmt.Errorf("invalid resourceType")
	}
}

// GetCspResourceStatus retrieves resource status from CB-Spider for a specific connection and resource type
//
// This function queries CB-Spider to get comprehensive resource information including:
// - Resources managed by both Tumblebug and Spider (MappedList)
// - Resources only managed by Spider (OnlySpiderList)
// - Resources only existing in CSP (OnlyCSPList)
//
// Parameters:
//   - connConfig: Connection configuration name for the target CSP
//   - resourceType: Type of resource to query (e.g., model.StrVM, model.StrVNet, etc.)
//
// Returns:
//   - model.CspResourceStatusResponse: Structured response containing resource lists and metadata
//   - error: Error if the operation fails
//
// Example usage:
//
//	response, err := GetCspResourceStatus("aws-connection", model.StrVM)
//	if err != nil {
//	    log.Error().Err(err).Msg("Failed to get CSP resource status")
//	    return err
//	}
//
//	fmt.Printf("Found %d VMs mapped in Spider\n", len(response.AllList.MappedList))
//	fmt.Printf("Found %d VMs only in CSP\n", len(response.AllList.OnlyCSPList))
func GetCspResourceStatus(connConfig string, resourceType string) (model.CspResourceStatusResponse, error) {
	var response model.CspResourceStatusResponse

	// Initialize response with basic information
	response.ConnectionName = connConfig
	response.ResourceType = resourceType

	// Create HTTP client with connection close for efficiency
	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	// Create request body
	requestBody := model.CspResourceStatusRequest{
		ConnectionName: connConfig,
	}

	// Determine Spider API endpoint based on resource type
	var spiderRequestURL string
	var isSubnetResource bool = false

	switch resourceType {
	case model.StrNLB:
		spiderRequestURL = model.SpiderRestUrl + "/allnlb"
	case model.StrVM:
		spiderRequestURL = model.SpiderRestUrl + "/allvm"
	case model.StrVNet:
		spiderRequestURL = model.SpiderRestUrl + "/allvpc"
	case model.StrSubnet:
		// Subnet requires special handling via VPC info
		spiderRequestURL = model.SpiderRestUrl + "/allvpcinfo"
		isSubnetResource = true
	case model.StrSecurityGroup:
		spiderRequestURL = model.SpiderRestUrl + "/allsecuritygroup"
	case model.StrSSHKey:
		spiderRequestURL = model.SpiderRestUrl + "/allkeypair"
	case model.StrDataDisk:
		spiderRequestURL = model.SpiderRestUrl + "/alldisk"
	case model.StrCustomImage:
		spiderRequestURL = model.SpiderRestUrl + "/allmyimage"
	default:
		err := fmt.Errorf("unsupported resource type: %s", resourceType)
		response.Error = err.Error()
		return response, err
	}

	// Make HTTP request to CB-Spider
	var resp *resty.Response
	var err error

	if isSubnetResource {
		// For Subnet, use different endpoint and query parameter
		resp, err = client.R().
			SetHeader("Content-Type", "application/json").
			SetQueryParam("ConnectionName", connConfig).
			SetResult(&model.SpiderAllVpcInfoWrapper{}).
			Get(spiderRequestURL)
	} else {
		// For other resources, use standard body-based request
		resp, err = client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(requestBody).
			SetResult(&model.SpiderAllListWrapper{}).
			Get(spiderRequestURL)
	}

	if err != nil {
		log.Error().Err(err).Str("connection", connConfig).Str("resourceType", resourceType).
			Msg("Failed to request CB-Spider for resource status")
		response.Error = fmt.Sprintf("HTTP request failed: %v", err)
		return response, fmt.Errorf("failed to request CB-Spider: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode() >= 400 || resp.StatusCode() < 200 {
		errorMsg := string(resp.Body())
		log.Error().Int("statusCode", resp.StatusCode()).Str("connection", connConfig).
			Str("resourceType", resourceType).Str("response", errorMsg).
			Msg("CB-Spider returned error status")
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode(), errorMsg)
		return response, fmt.Errorf("CB-Spider error (HTTP %d): %s", resp.StatusCode(), errorMsg)
	}

	// Parse response from CB-Spider
	if isSubnetResource {
		// Special handling for Subnet resources
		vpcInfoResponse, ok := resp.Result().(*model.SpiderAllVpcInfoWrapper)
		if !ok {
			err := fmt.Errorf("failed to parse VPC info response from CB-Spider")
			response.Error = err.Error()
			return response, err
		}

		// Extract all subnet SystemIds from all VPCs
		var subnetList []model.SpiderNameIdSystemId
		// Check all three lists: MappedInfoList, OnlySpiderList, and OnlyCSPInfoList
		allVpcLists := [][]model.SpiderVpcInfo{
			vpcInfoResponse.AllListInfo.MappedInfoList,
			vpcInfoResponse.AllListInfo.OnlySpiderList,
			vpcInfoResponse.AllListInfo.OnlyCSPInfoList,
		}

		for _, vpcList := range allVpcLists {
			for _, vpc := range vpcList {
				for _, subnet := range vpc.SubnetInfoList {
					subnetList = append(subnetList, model.SpiderNameIdSystemId{
						NameId:   subnet.IId.NameId,
						SystemId: subnet.IId.SystemId,
					})
				}
			}
		}
		log.Debug().Interface("subnetList", subnetList).Msg("Extracted subnet list from VPC info")

		// For subnets, we only have OnlyCSPList since they're not managed directly by Spider
		response.AllList = model.SpiderAllList{
			MappedList:     []model.SpiderNameIdSystemId{},
			OnlySpiderList: []model.SpiderNameIdSystemId{},
			OnlyCSPList:    subnetList,
		}
	} else {
		// Standard handling for other resources
		spiderResponse, ok := resp.Result().(*model.SpiderAllListWrapper)
		if !ok {
			err := fmt.Errorf("failed to parse response from CB-Spider")
			response.Error = err.Error()
			return response, err
		}

		// Copy the AllList data to response
		response.AllList = spiderResponse.AllList
	}

	// Add success message with resource counts
	mappedCount := len(response.AllList.MappedList)
	spiderOnlyCount := len(response.AllList.OnlySpiderList)
	cspOnlyCount := len(response.AllList.OnlyCSPList)

	response.SystemMessage = fmt.Sprintf("Successfully retrieved %s resources from %s: %d mapped, %d spider-only, %d csp-only",
		resourceType, connConfig, mappedCount, spiderOnlyCount, cspOnlyCount)

	log.Info().Str("connection", connConfig).Str("resourceType", resourceType).
		Int("mapped", mappedCount).Int("spiderOnly", spiderOnlyCount).Int("cspOnly", cspOnlyCount).
		Msg("Successfully retrieved CSP resource status")

	return response, nil
}

// GetCspResourceStatusBatch retrieves resource status for multiple resource types in a single connection
//
// This is a convenience function that calls GetCspResourceStatus for multiple resource types
// and returns a map of results. This is useful when you need to check multiple resource types
// for the same connection configuration.
//
// Parameters:
//   - connConfig: Connection configuration name for the target CSP
//   - resourceTypes: List of resource types to query
//
// Returns:
//   - map[string]model.CspResourceStatusResponse: Map of resource type to response
//   - error: Error if any of the operations fail (returns first error encountered)
//
// Example usage:
//
//	resourceTypes := []string{model.StrVM, model.StrVNet, model.StrSecurityGroup}
//	responses, err := GetCspResourceStatusBatch("aws-connection", resourceTypes)
//	if err != nil {
//	    log.Error().Err(err).Msg("Failed to get batch CSP resource status")
//	    return err
//	}
//
//	for resourceType, response := range responses {
//	    fmt.Printf("%s: %s\n", resourceType, response.SystemMessage)
//	}
func GetCspResourceStatusBatch(connConfig string, resourceTypes []string) (map[string]model.CspResourceStatusResponse, error) {
	results := make(map[string]model.CspResourceStatusResponse)

	for _, resourceType := range resourceTypes {
		response, err := GetCspResourceStatus(connConfig, resourceType)
		if err != nil {
			log.Error().Err(err).Str("connection", connConfig).Str("resourceType", resourceType).
				Msg("Failed to get CSP resource status in batch operation")
			return results, fmt.Errorf("failed to get status for %s in %s: %w", resourceType, connConfig, err)
		}
		results[resourceType] = response
	}

	log.Info().Str("connection", connConfig).Int("resourceTypes", len(resourceTypes)).
		Msg("Successfully completed batch CSP resource status retrieval")

	return results, nil
}

// CheckAssociatedCspResourceExistence checks if a CB-TB resource's associated CSP resource exists in Spider and CSP
//
// This function takes a CB-TB resource and checks if its corresponding CSP resource exists in:
//   - CSP (Cloud Service Provider): Checks MappedList and OnlyCSPList
//   - Spider (CB-Spider): Checks MappedList and OnlySpiderList
//
// Parameters:
//   - nsId: Namespace ID of the CB-TB resource
//   - resourceType: Type of the CB-TB resource (e.g., model.StrVM, model.StrVNet, etc.)
//   - resourceId: ID of the CB-TB resource
//   - connConfig: Connection configuration name for the target CSP
//
// Returns:
//   - onCsp: true if the resource exists in CSP (either mapped or CSP-only)
//   - onSpider: true if the resource exists in Spider (either mapped or Spider-only)
//   - error: Error if the operation fails (connection errors, resource not found, etc.)
//
// Example usage:
//
//	onCsp, onSpider, err := CheckAssociatedCspResourceExistence("default", model.StrVM, "my-vm-01", "aws-connection")
//	if err != nil {
//	    log.Error().Err(err).Msg("Failed to check resource existence")
//	    return err
//	}
//
//	if onCsp && onSpider {
//	    fmt.Println("Resource exists in both CSP and Spider (mapped)")
//	} else if onCsp && !onSpider {
//	    fmt.Println("Resource exists only in CSP")
//	} else if !onCsp && onSpider {
//	    fmt.Println("Resource exists only in Spider")
//	} else {
//	    fmt.Println("Resource does not exist in either CSP or Spider")
//	}
func CheckAssociatedCspResourceExistence(nsId string, resourceType string, resourceId string, connConfig string) (onCsp bool, onSpider bool, err error) {
	// Initialize return values
	onCsp = false
	onSpider = false

	// Get the CSP resource ID/name from CB-TB resource
	cspResourceId, err := GetCspResourceId(nsId, resourceType, resourceId)
	if err != nil {
		log.Error().Err(err).Str("nsId", nsId).Str("resourceType", resourceType).
			Str("resourceId", resourceId).Msg("Failed to get CSP resource ID from CB-TB resource")
		return false, false, fmt.Errorf("failed to get CSP resource ID for %s/%s/%s: %w", nsId, resourceType, resourceId, err)
	}

	if cspResourceId == "" {
		log.Warn().Str("nsId", nsId).Str("resourceType", resourceType).
			Str("resourceId", resourceId).Msg("CSP resource ID is empty")
		return false, false, fmt.Errorf("CSP resource ID is empty for %s/%s/%s", nsId, resourceType, resourceId)
	}

	// Get CSP resource status from Spider
	response, err := GetCspResourceStatus(connConfig, resourceType)
	if err != nil {
		log.Error().Err(err).Str("connection", connConfig).Str("resourceType", resourceType).
			Msg("Failed to get CSP resource status")
		return false, false, fmt.Errorf("failed to get CSP resource status for %s/%s: %w", connConfig, resourceType, err)
	}

	// Check if the CSP resource exists in MappedList
	for _, resource := range response.AllList.MappedList {
		if resource.SystemId == cspResourceId {
			log.Debug().Str("cspResourceId", cspResourceId).Str("systemId", resource.SystemId).
				Msg("Found resource in MappedList")
			return true, true, nil // Mapped resources exist in both CSP and Spider
		}
	}

	// Check if the CSP resource exists in OnlyCSPList
	for _, resource := range response.AllList.OnlyCSPList {
		if resource.SystemId == cspResourceId {
			log.Debug().Str("cspResourceId", cspResourceId).Str("systemId", resource.SystemId).
				Msg("Found resource in OnlyCSPList")
			return true, false, nil // Exists only in CSP
		}
	}

	// Check if the CSP resource exists in OnlySpiderList
	for _, resource := range response.AllList.OnlySpiderList {
		if resource.SystemId == cspResourceId {
			log.Debug().Str("cspResourceId", cspResourceId).Str("systemId", resource.SystemId).
				Msg("Found resource in OnlySpiderList")
			return false, true, nil // Exists only in Spider
		}
	}

	// Resource not found in any list
	log.Debug().Str("cspResourceId", cspResourceId).Str("connection", connConfig).
		Str("resourceType", resourceType).Msg("Resource not found in any list")
	return false, false, nil
}
