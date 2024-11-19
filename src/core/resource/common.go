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
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
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
	deletedResources := model.IdList{}
	var mutex sync.Mutex  // Protect shared slice access
	var wg sync.WaitGroup // Synchronize all goroutines

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	resourceIdList, err := ListResourceId(nsId, resourceType)
	if err != nil {
		return deletedResources, err
	}

	if len(resourceIdList) == 0 {
		errString := fmt.Sprintf("There is no %s resource in %s", resourceType, nsId)
		err := fmt.Errorf(errString)
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	// Channel to capture errors
	errChan := make(chan error, len(resourceIdList))

	// Process each resourceId concurrently
	for _, v := range resourceIdList {
		// Increment WaitGroup counter
		wg.Add(1)

		// Launch a goroutine for each resource deletion
		go func(resourceId string) {
			defer wg.Done()
			common.RandomSleep(0, len(resourceIdList)/10)

			// Check if the resourceId matches the subString criteria
			if subString != "" && !strings.Contains(resourceId, subString) {
				return
			}

			// Attempt to delete the resource
			deleteStatus := "[Done] "
			errString := ""

			err := DelResource(nsId, resourceType, resourceId, forceFlag)
			if err != nil {
				deleteStatus = "[Failed] "
				errString = " (" + err.Error() + ")"
				errChan <- err // Send error to the error channel
			}

			// Safely append the result to deletedResources.IdList using mutex
			mutex.Lock()
			deletedResources.IdList = append(deletedResources.IdList, deleteStatus+resourceType+": "+resourceId+errString)
			mutex.Unlock()
		}(v) // Pass loop variable as an argument to avoid race conditions
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan) // Close the error channel

	// Collect any errors from the error channel
	for err := range errChan {
		if err != nil {
			log.Info().Err(err).Msg("error deleting resource")
		}
	}

	return deletedResources, nil
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
		_, err = model.ORM.Delete(&model.TbImageInfo{Namespace: nsId, Id: resourceId})
		if err != nil {
			fmt.Println(err.Error())
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
		_, err = model.ORM.Delete(&model.TbSpecInfo{Namespace: nsId, Id: resourceId})
		if err != nil {
			fmt.Println(err.Error())
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

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	if resourceType == model.StrVNet {
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
	} else if resourceType == model.StrCustomImage {
		// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
		_, err = model.ORM.Delete(&model.TbCustomImageInfo{Namespace: nsId, Id: resourceId})
		if err != nil {
			fmt.Println(err.Error())
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

	if resourceType == model.StrImage ||
		resourceType == model.StrCustomImage ||
		resourceType == model.StrSSHKey ||
		resourceType == model.StrSpec ||
		resourceType == model.StrVNet ||
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" ||
		resourceType == model.StrSecurityGroup ||
		resourceType == model.StrDataDisk {
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

	if resourceType == model.StrImage ||
		resourceType == model.StrCustomImage ||
		resourceType == model.StrSSHKey ||
		resourceType == model.StrSpec ||
		resourceType == model.StrVNet ||
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" ||
		resourceType == model.StrSecurityGroup ||
		resourceType == model.StrDataDisk {
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

			fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
			switch {
			case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
				err := fmt.Errorf(string(resp.Body()))
				fmt.Println("body: ", string(resp.Body()))
				log.Error().Err(err).Msg("")
				return nil, err
			}

			updatedSpiderMyImage := resp.Result().(*model.SpiderMyImageInfo)
			res.Status = updatedSpiderMyImage.Status
			fmt.Printf("res.Status: %s \n", res.Status) // for debug
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
	if resourceType == model.StrImage ||
		resourceType == model.StrCustomImage ||
		resourceType == model.StrSSHKey ||
		resourceType == model.StrSpec ||
		resourceType == model.StrVNet ||
		resourceType == model.StrVPN ||
		resourceType == model.StrSqlDB ||
		resourceType == model.StrObjectStorage ||
		resourceType == model.StrSecurityGroup ||
		resourceType == model.StrDataDisk {
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
	if resourceType == model.StrSubnet {
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
func LoadAssets() (model.IdList, error) {

	regiesteredIds := model.IdList{}
	regiesteredStatus := ""

	// WaitGroups for goroutine
	// var waitSpecImg sync.WaitGroup
	var wait sync.WaitGroup

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

	connectionList, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot GetConnConfigList")
		return regiesteredIds, err
	}
	if len(connectionList.Connectionconfig) == 0 {
		log.Error().Err(err).Msg("No registered connection config")
		return regiesteredIds, err
	}

	elapsedVerifyConnections := time.Now().Sub(startTime)
	log.Info().Msgf("Verified all connections. Elapsed [%s]", elapsedVerifyConnections)
	startTime = time.Now()

	// LookupSpecList and LookupImageList of all connections in parallel
	var specMap sync.Map

	tmpSpecList := []model.TbSpecInfo{}
	tmpImageList := []model.TbImageInfo{}

	// ignoreConnectionMap is used to store connection names that failed to lookup specs
	var ignoreConnectionMap sync.Map
	// validRepresentativeConnectionMap is used to store connection names that valid representative connection
	var validRepresentativeConnectionMap sync.Map

	startTime = time.Now()
	var wg sync.WaitGroup
	for _, connConfig := range connectionList.Connectionconfig {
		wg.Add(1)
		go func(connConfig model.ConnConfig) {
			defer wg.Done()
			specsInConnection, err := LookupSpecList(connConfig.ConfigName)
			if err != nil {
				log.Error().Err(err).Msgf("Cannot LookupSpecList in %s", connConfig.ConfigName)
				ignoreConnectionMap.Store(connConfig.ConfigName, err)
				return
			}
			log.Info().Msgf("[%s] #Spec: %d", connConfig.ConfigName, len(specsInConnection.Vmspec))
			validRepresentativeConnectionMap.Store(connConfig.ProviderName+"-"+connConfig.RegionDetail.RegionName, connConfig)
			for _, spec := range specsInConnection.Vmspec {
				spiderSpec := spec
				//log.Info().Msgf("Found spec in the map: %s", spiderSpec.Name)
				tumblebugSpec, errConvert := ConvertSpiderSpecToTumblebugSpec(spiderSpec)
				if errConvert != nil {
					log.Error().Err(errConvert).Msg("Cannot ConvertSpiderSpecToTumblebugSpec")
				} else {
					key := GetProviderRegionZoneResourceKey(connConfig.ProviderName, connConfig.RegionDetail.RegionName, "", spec.Name)
					tumblebugSpec.Namespace = model.SystemCommonNs
					tumblebugSpec.Name = key
					tumblebugSpec.Id = key
					tumblebugSpec.ConnectionName = connConfig.ConfigName
					tumblebugSpec.ProviderName = strings.ToLower(connConfig.ProviderName)
					tumblebugSpec.RegionName = connConfig.RegionDetail.RegionName
					tumblebugSpec.InfraType = "vm" // default value
					tumblebugSpec.SystemLabel = "auto-gen"
					tumblebugSpec.CostPerHour = 99999999.9
					tumblebugSpec.EvaluationScore01 = -99.9

					// instead of connConfig.RegionName, spec.Region will be used in the future
					//log.Info().Msgf("specMap.Store(%s, spec)", key)
					specMap.Store(key, tumblebugSpec)
					tmpSpecList = append(tmpSpecList, tumblebugSpec)
				}
			}
		}(connConfig)
	}

	// LookupImageList of all connections in parallel takes too long time
	// disable it for now

	// for i := len(connectionList.Connectionconfig) - 1; i >= 0; i-- {
	// 	connConfig := connectionList.Connectionconfig[i]
	// 	wg.Add(1)
	// 	go func(cc model.ConnConfig) {
	// 		defer wg.Done()
	// 		imagesInConnection, err := LookupImageList(cc.ConfigName)
	// 		if err != nil {
	// 			log.Error().Err(err).Msgf("Cannot LookupImageList in %s", cc.ConfigName)
	// 		} else {
	// 			log.Info().Msgf("[%s] #Image: %d", cc.ConfigName, len(imagesInConnection.Image))
	// 		}
	// 	}(connConfig)
	// }
	wg.Wait()

	elapsedLookupSpecList := time.Now().Sub(startTime)
	log.Info().Msgf("Lookup Spec List is complete. Elapsed [%s]", elapsedLookupSpecList)
	startTime = time.Now()

	// specMap.Range(func(key, value interface{}) bool {
	// 	specInfo := value.(model.TbSpecInfo)
	// 	//log.Info().Msgf("specMap.Range: %s value: %v", key, specInfo)

	// 	_, errRegisterSpec := RegisterSpecWithInfo(model.SystemCommonNs, &specInfo, true)
	// 	if errRegisterSpec != nil {
	// 		log.Info().Err(errRegisterSpec).Msg("RegisterSpec WithInfo failed")
	// 	}
	// 	return true
	// })

	err = RegisterSpecWithInfoInBulk(tmpSpecList)
	if err != nil {
		log.Info().Err(err).Msg("RegisterSpec WithInfo failed")
	}
	tmpSpecList = nil

	elapsedRegisterSpecs := time.Now().Sub(startTime)
	log.Info().Msgf("Registerd the Specs. Elapsed [%s]", elapsedRegisterSpecs)
	startTime = time.Now()

	err = RemoveDuplicateSpecsInSQL()
	if err != nil {
		log.Error().Err(err).Msg("RemoveDuplicateSpecsInSQL failed")
	}
	elapsedRemoveDuplicateSpecsInSQL := time.Now().Sub(startTime)
	log.Info().Msgf("Remove Duplicate Specs In SQL. Elapsed [%s]", elapsedRemoveDuplicateSpecsInSQL)
	startTime = time.Now()

	// Read common specs and register spec objects
	file, fileErr := os.Open("../assets/cloudspec.csv")
	if fileErr != nil {
		log.Error().Err(fileErr).Msg("")
		return regiesteredIds, fileErr
	}
	defer file.Close()

	rdr := csv.NewReader(bufio.NewReader(file))
	rowsSpec, _ := rdr.ReadAll()

	// expending rows with "all" connectionName into each region
	// "all" means the values in the row are applicable to all connectionNames in a CSP
	newRowsSpec := make([][]string, 0, len(rowsSpec))
	for _, row := range rowsSpec {
		if row[1] == "all" {
			for _, connConfig := range connectionList.Connectionconfig {
				if strings.EqualFold(connConfig.ProviderName, row[0]) {
					newRow := make([]string, len(row))
					copy(newRow, row)
					newRow[1] = connConfig.RegionDetail.RegionName
					newRowsSpec = append(newRowsSpec, newRow)
					//log.Info().Msgf("Expended row: %s", newRow)
				}
			}
		} else {
			newRowsSpec = append(newRowsSpec, row)
		}
	}
	rowsSpec = newRowsSpec

	// Read common specs and register spec objects
	file, fileErr = os.Open("../assets/cloudimage.csv")
	if fileErr != nil {
		log.Error().Err(fileErr).Msg("")
		return regiesteredIds, fileErr
	}
	defer file.Close()

	rdr = csv.NewReader(bufio.NewReader(file))
	rowsImg, _ := rdr.ReadAll()

	// expending rows with "all" connectionName into each region
	// "all" means the values in the row are applicable to all connectionNames in a CSP
	newRowsImg := make([][]string, 0, len(rowsImg))
	for _, row := range rowsImg {
		if row[1] == "all" {
			for _, connConfig := range connectionList.Connectionconfig {
				if strings.EqualFold(connConfig.ProviderName, row[0]) {
					newRow := make([]string, len(row))
					copy(newRow, row)
					newRow[1] = connConfig.RegionDetail.RegionName
					newRowsImg = append(newRowsImg, newRow)
					//log.Info().Msgf("Expended row: %s", newRow)
				}
			}
		} else {
			newRowsImg = append(newRowsImg, row)
		}
	}
	rowsImg = newRowsImg

	// waitSpecImg.Add(1)
	//go func(rowsSpec [][]string) {
	// defer waitSpecImg.Done()
	//lenSpecs := len(rowsSpec[1:])
	for i, row := range rowsSpec[1:] {
		// wait.Add(1)
		// go func(i int, row []string, lenSpecs int) {
		// 	defer wait.Done()
		// 	common.RandomSleep(0, lenSpecs/20)

		specReqTmp := model.TbSpecReq{}
		// 0	providerName
		// 1	regionName
		// 2	cspResourceId
		// 3	CostPerHour
		// 4	evaluationScore01
		// 5	evaluationScore02
		// 6	evaluationScore03
		// 7	evaluationScore04
		// 8	evaluationScore05
		// 9	evaluationScore06
		// 10	evaluationScore07
		// 11	evaluationScore08
		// 12	evaluationScore09
		// 13	evaluationScore10
		// 14	rootDiskType
		// 15	rootDiskSize
		// 17	acceleratorModel
		// 18	acceleratorCount
		// 19	acceleratorMemoryGB
		// 20	acceleratorDetails
		// 21	infraType

		providerName := strings.ToLower(row[0])
		regionName := strings.ToLower(row[1])
		specReqTmp.CspSpecName = row[2]
		rootDiskType := row[14]
		rootDiskSize := row[15]
		acceleratorType := row[16]
		acceleratorModel := row[17]
		acceleratorCount := 0
		if s, err := strconv.Atoi(row[18]); err == nil {
			acceleratorCount = s
		}
		acceleratorMemoryGB := 0.0
		if s, err := strconv.ParseFloat(row[19], 32); err == nil {
			acceleratorMemoryGB = s
		}
		description := row[20]
		infraType := strings.ToLower(row[21])

		specReqTmp.Name = GetProviderRegionZoneResourceKey(providerName, regionName, "", specReqTmp.CspSpecName)

		//get connetion for lookup (if regionName is "all", use providerName only)
		validRepresentativeConnectionMapKey := providerName + "-" + regionName
		connectionForLookup, ok := validRepresentativeConnectionMap.Load(validRepresentativeConnectionMapKey)
		if ok {
			specReqTmp.ConnectionName = connectionForLookup.(model.ConnConfig).ConfigName

			_, ignoreCase := ignoreConnectionMap.Load(specReqTmp.ConnectionName)
			if !ignoreCase {
				// Give a name for spec object by combining ConnectionName and CspResourceId
				// To avoid naming-rule violation, modify the string

				// specReqTmp.Name = specReqTmp.ConnectionName + "-" + specReqTmp.CspResourceId
				// specReqTmp.Name = ToNamingRuleCompatible(specReqTmp.Name)
				specInfoId := specReqTmp.Name

				specReqTmp.Description = "Common Spec Resource"

				regiesteredStatus = ""

				var errRegisterSpec error

				log.Trace().Msgf("[%d] register Common Spec: %s", i, specReqTmp.Name)

				// Register Spec object
				searchKey := GetProviderRegionZoneResourceKey(providerName, regionName, "", specReqTmp.CspSpecName)
				value, ok := specMap.Load(searchKey)
				if ok {
					// spiderSpec := value.(SpiderSpecInfo)
					// //log.Info().Msgf("Found spec in the map: %s", spiderSpec.Name)
					// tumblebugSpec, errConvert := ConvertSpiderSpecToTumblebugSpec(spiderSpec)
					// if errConvert != nil {
					// 	log.Error().Err(errConvert).Msg("Cannot ConvertSpiderSpecToTumblebugSpec")
					// }

					// tumblebugSpec.Name = specInfoId
					// tumblebugSpec.ConnectionName = specReqTmp.ConnectionName
					// // _, errRegisterSpec = RegisterSpecWithInfo(model.SystemCommonNs, &tumblebugSpec, true)
					// // if errRegisterSpec != nil {
					// // 	log.Info().Err(errRegisterSpec).Msg("RegisterSpec WithInfo failed")
					// // }
					specInfo := value.(model.TbSpecInfo)

					// Update registered Spec object with givn info from asset file
					// Update registered Spec object with Cost info
					costPerHour, err2 := strconv.ParseFloat(strings.ReplaceAll(row[3], " ", ""), 32)
					if err2 != nil {
						log.Error().Msgf("Not valid CostPerHour value in the asset: %s", specInfoId)
						costPerHour = 99999999.9
					}
					evaluationScore01, err2 := strconv.ParseFloat(strings.ReplaceAll(row[4], " ", ""), 32)
					if err2 != nil {
						log.Error().Msgf("Not valid evaluationScore01 value in the asset: %s", specInfoId)
						evaluationScore01 = -99.9
					}
					expandedInfraType := expandInfraType(infraType)

					specInfo.ProviderName = providerName
					specInfo.RegionName = regionName
					specInfo.CostPerHour = float32(costPerHour)
					specInfo.RootDiskType = rootDiskType
					specInfo.RootDiskSize = rootDiskSize
					specInfo.AcceleratorType = acceleratorType
					specInfo.AcceleratorModel = acceleratorModel
					specInfo.AcceleratorCount = uint8(acceleratorCount)
					specInfo.AcceleratorMemoryGB = float32(acceleratorMemoryGB)
					specInfo.Description = description
					specInfo.EvaluationScore01 = float32(evaluationScore01)
					specInfo.SystemLabel = "from-assets"
					specInfo.InfraType = expandedInfraType

					// _, err3 := UpdateSpec(model.SystemCommonNs, specInfoId, specInfo)
					// if err3 != nil {
					// 	log.Error().Err(err3).Msg("UpdateSpec failed")
					// 	regiesteredStatus += "  [Failed] " + err3.Error()
					// }

					tmpSpecList = append(tmpSpecList, specInfo)

					//fmt.Printf("[%d] Registered Common Spec\n", i)
					//common.PrintJsonPretty(updatedSpecInfo)

				} else {
					errRegisterSpec = fmt.Errorf("Not Found spec from the fetched spec list: %s", searchKey)
					log.Trace().Msgf(errRegisterSpec.Error())
					// _, errRegisterSpec = RegisterSpecWithCspResourceId(model.SystemCommonNs, &specReqTmp, true)
					// if errRegisterSpec != nil {
					// 	log.Error().Err(errRegisterSpec).Msg("RegisterSpec WithCspResourceId failed")
					// }
					regiesteredStatus += "  [Failed] " + errRegisterSpec.Error()
				}

				regiesteredIds.AddItem(model.StrSpec + ": " + specInfoId + regiesteredStatus)
				// }(i, row, lenSpecs)
			}
		}
	}
	// 	wait.Wait()
	// }(rowsSpec)

	log.Info().Msgf("tmpSpecList %d", len(tmpSpecList))

	err = RegisterSpecWithInfoInBulk(tmpSpecList)
	if err != nil {
		log.Info().Err(err).Msg("RegisterSpec WithInfo failed")
	}
	tmpSpecList = nil

	// elapsedRegisterUpdatedSpecs := time.Now().Sub(startTime)
	// log.Info().Msgf("Registerd Updated Specs. Elapsed [%s]", elapsedRegisterUpdatedSpecs)
	// startTime = time.Now()

	err = RemoveDuplicateSpecsInSQL()
	if err != nil {
		log.Error().Err(err).Msg("RemoveDuplicateSpecsInSQL failed")
	}
	// elapsedRemoveDuplicateSpecsInSQLUpdated := time.Now().Sub(startTime)
	// log.Info().Msgf("Remove Duplicate Specs In SQL. Elapsed [%s]", elapsedRemoveDuplicateSpecsInSQLUpdated)
	// startTime = time.Now()

	elapsedUpdateSpec := time.Now().Sub(startTime)
	log.Info().Msgf("Updated the registered Specs according to the asset file. Elapsed [%s]", elapsedUpdateSpec)
	startTime = time.Now()

	// // waitSpecImg.Add(1)
	// go func(rowsImg [][]string) {
	// 	// defer waitSpecImg.Done()
	lenImages := len(rowsImg[1:])
	for i, row := range rowsImg[1:] {
		wait.Add(1)
		// fmt.Printf("[%d] i, row := range rowsImg[1:] %s\n", i, row)
		// goroutine
		go func(i int, row []string, lenImages int) {
			defer wait.Done()

			imageReqTmp := model.TbImageReq{}
			// row0: ProviderName
			// row1: regionName
			// row2: cspResourceId
			// row3: OsType
			// row4: description
			// row5: supportedInstance
			// row6: infraType
			providerName := strings.ToLower(row[0])
			regionName := strings.ToLower(row[1])
			imageReqTmp.CspImageName = row[2]
			osType := strings.ReplaceAll(row[3], " ", "")
			description := row[4]
			infraType := strings.ToLower(row[6])

			// Give a name for spec object by combining ConnectionName and OsType
			imageReqTmp.Name = GetProviderRegionZoneResourceKey(providerName, regionName, "", osType)

			//get connetion for lookup (if regionName is "all", use providerName only)
			validRepresentativeConnectionMapKey := providerName + "-" + regionName
			connectionForLookup, ok := validRepresentativeConnectionMap.Load(validRepresentativeConnectionMapKey)
			if ok {
				imageReqTmp.ConnectionName = connectionForLookup.(model.ConnConfig).ConfigName

				_, ignoreCase := ignoreConnectionMap.Load(imageReqTmp.ConnectionName)
				if !ignoreCase {
					// RandomSleep for safe parallel executions
					common.RandomSleep(0, lenImages/8)

					// To avoid naming-rule violation, modify the string
					// imageReqTmp.Name = imageReqTmp.ConnectionName + "-" + osType
					// imageReqTmp.Name = ToNamingRuleCompatible(imageReqTmp.Name)
					imageInfoId := imageReqTmp.Name
					imageReqTmp.Description = "Common Image Resource"

					log.Trace().Msgf("[%d] register Common Image: %s", i, imageReqTmp.Name)

					// Register Spec object
					regiesteredStatus = ""

					tmpImageInfo, err1 := GetImageInfoFromLookupImage(model.SystemCommonNs, imageReqTmp)
					if err1 != nil {
						log.Info().Msgf("lookup failure, Provider: %s, Region: %s, CspImageName: %s Error: %s", providerName, regionName, imageReqTmp.CspImageName, err1.Error())
						regiesteredStatus += "  [Failed] " + err1.Error()
					} else {
						// Update registered image object with OsType info
						expandedInfraType := expandInfraType(infraType)

						tmpImageInfo.GuestOS = osType
						tmpImageInfo.Description = description
						tmpImageInfo.InfraType = expandedInfraType

						tmpImageList = append(tmpImageList, tmpImageInfo)

					}

					// _, err1 := RegisterImageWithId(model.SystemCommonNs, &imageReqTmp, true, true)
					// if err1 != nil {
					// 	log.Info().Msgf("Provider: %s, Region: %s, CspResourceId: %s Error: %s", providerName, regionName, imageReqTmp.CspImageName, err1.Error())
					// 	regiesteredStatus += "  [Failed] " + err1.Error()
					// } else {
					// 	// Update registered image object with OsType info
					// 	expandedInfraType := expandInfraType(infraType)
					// 	imageUpdateRequest := model.TbImageInfo{
					// 		GuestOS:     osType,
					// 		Description: description,
					// 		InfraType:   expandedInfraType,
					// 	}
					// 	_, err2 := UpdateImage(model.SystemCommonNs, imageInfoId, imageUpdateRequest, true)
					// 	if err2 != nil {
					// 		log.Error().Err(err2).Msg("UpdateImage failed")
					// 		regiesteredStatus += "  [Failed] " + err2.Error()
					// 	}
					// }

					//regiesteredStatus = strings.Replace(regiesteredStatus, "\\", "", -1)
					regiesteredIds.AddItem(model.StrImage + ": " + imageInfoId + regiesteredStatus)
				}
			}
		}(i, row, lenImages)
	}
	wait.Wait()
	// }(rowsImg)

	log.Info().Msgf("tmpImageList %d", len(tmpImageList))

	err = RegisterImageWithInfoInBulk(tmpImageList)
	if err != nil {
		log.Info().Err(err).Msg("RegisterImage WithInfo failed")
	}
	tmpImageList = nil

	// elapsedRegisterUpdatedImages := time.Now().Sub(startTime)
	// log.Info().Msgf("Registerd Updated Images. Elapsed [%s]", elapsedRegisterUpdatedImages)
	// startTime = time.Now()

	err = RemoveDuplicateImagesInSQL()
	if err != nil {
		log.Error().Err(err).Msg("RemoveDuplicateImagesInSQL failed")
	}
	// elapsedRemoveDuplicateImagesInSQLUpdated := time.Now().Sub(startTime)
	// log.Info().Msgf("Remove Duplicate Images In SQL. Elapsed [%s]", elapsedRemoveDuplicateImagesInSQLUpdated)
	// startTime = time.Now()

	elapsedUpdateImg := time.Now().Sub(startTime)

	// waitSpecImg.Wait()
	// sort.Strings(regiesteredIds.IdList)
	log.Info().Msgf("Registered Common Resources %d", len(regiesteredIds.IdList))

	log.Info().Msgf("Verified all connections. Elapsed [%s]", elapsedVerifyConnections)
	log.Info().Msgf("Lookup Spec List is complete. Elapsed [%s]", elapsedLookupSpecList)
	log.Info().Msgf("Remove Duplicate Specs In SQL. Elapsed [%s]", elapsedRemoveDuplicateSpecsInSQL)
	log.Info().Msgf("Updated the registered Specs according to the asset file. Elapsed [%s]", elapsedUpdateSpec)
	log.Info().Msgf("Updated the registered Images according to the asset file. Elapsed [%s]", elapsedUpdateImg)

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
		resList = append(resList, "vnet")
		resList = append(resList, "sshkey")
		resList = append(resList, "sg")
	} else {
		resList = append(resList, strings.ToLower(resType))
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
		err := fmt.Errorf("Cannot find the connection config: %s", connectionName)
		log.Error().Err(err).Msg("Failed to CreateSharedResource")
		return err
	}
	sliceIndex = (sliceIndex % 254) + 1

	//resourceName := connectionName
	// Default resource name has this pattern (nsId + "-shared-" + connectionName)
	resourceName := nsId + model.StrSharedResourceName + connectionName
	description := "Generated Default Resource"

	for _, resType := range resList {
		if resType == "vnet" {
			log.Debug().Msg("vnet")

			reqTmp := model.TbVNetReq{}
			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			// set isolated private address space for each cloud region (10.i.0.0/16)
			reqTmp.CidrBlock = "10." + strconv.Itoa(sliceIndex) + ".0.0/16"
			if strings.EqualFold(provider, "cloudit") {
				// CLOUDIT: the list of subnets that can be created is
				// 10.0.4.0/22,10.0.8.0/22,10.0.12.0/22,10.0.28.0/22,10.0.32.0/22,
				// 10.0.36.0/22,10.0.40.0/22,10.0.44.0/22,10.0.48.0/22,10.0.52.0/22,
				// 10.0.56.0/22,10.0.60.0/22,10.0.64.0/22,10.0.68.0/22,10.0.72.0/22,
				// 10.0.76.0/22,10.0.80.0/22,10.0.84.0/22,10.0.88.0/22,10.0.92.0/22,
				// 10.0.96.0/22,10.0.100.0/22,10.0.104.0/22,10.0.108.0/22,10.0.112.0/22,
				// 10.0.116.0/22,10.0.120.0/22,10.0.124.0/22,10.0.132.0/22,10.0.136.0/22,
				// 10.0.140.0/22,10.0.144.0/22,10.0.148.0/22,10.0.152.0/22,10.0.156.0/22,
				// 10.0.160.0/22,10.0.164.0/22,10.0.168.0/22,10.0.172.0/22,10.0.176.0/22,
				// 10.0.180.0/22,10.0.184.0/22,10.0.188.0/22,10.0.192.0/22,10.0.196.0/22,
				// 10.0.200.0/22,10.0.204.0/22,10.0.208.0/22,10.0.212.0/22,10.0.216.0/22,
				// 10.0.220.0/22,10.0.224.0/22,10.0.228.0/22,10.0.232.0/22,10.0.236.0/22,
				// 10.0.240.0/22,10.0.244.0/22,10.0.248.0/22

				// temporally assign 10.0.40.0/22 until new policy.
				reqTmp.CidrBlock = "10.0.40.0/22"
			}

			// Consist 2 subnets (10.i.0.0/18, 10.i.64.0/18)
			// Reserve spaces for tentative 2 subnets (10.i.128.0/18, 10.i.192.0/18)
			subnetName := reqTmp.Name
			subnetCidr := "10." + strconv.Itoa(sliceIndex) + ".0.0/18"
			subnet := model.TbSubnetReq{Name: subnetName, IPv4_CIDR: subnetCidr}
			reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)

			subnetName = reqTmp.Name + "-01"
			subnetCidr = "10." + strconv.Itoa(sliceIndex) + ".64.0/18"
			subnet = model.TbSubnetReq{Name: subnetName, IPv4_CIDR: subnetCidr}
			reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)

			common.PrintJsonPretty(reqTmp)

			resultInfo, err := CreateVNet(nsId, &reqTmp)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create vNet")
				return err
			}
			common.PrintJsonPretty(resultInfo)
		} else if resType == "sg" || resType == "securitygroup" {
			log.Debug().Msg("sg")

			reqTmp := model.TbSecurityGroupReq{}

			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			reqTmp.VNetId = resourceName

			// open all firewall for default securityGroup
			rule := model.TbFirewallRuleInfo{FromPort: "1", ToPort: "65535", IPProtocol: "tcp", Direction: "inbound", CIDR: "0.0.0.0/0"}
			var ruleList []model.TbFirewallRuleInfo
			ruleList = append(ruleList, rule)
			rule = model.TbFirewallRuleInfo{FromPort: "1", ToPort: "65535", IPProtocol: "udp", Direction: "inbound", CIDR: "0.0.0.0/0"}
			ruleList = append(ruleList, rule)
			// CloudIt only offers tcp, udp Protocols
			if !strings.EqualFold(provider, "cloudit") {
				rule = model.TbFirewallRuleInfo{FromPort: "-1", ToPort: "-1", IPProtocol: "icmp", Direction: "inbound", CIDR: "0.0.0.0/0"}
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

		} else if resType == "sshkey" {
			log.Debug().Msg("sshkey")

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

// DelAllSharedResources deletes all Default securityGroup, sshKey, vNet objects
func DelAllSharedResources(nsId string) (model.IdList, error) {

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

	if resourceType == model.StrSpec {
		specInfo, err := GetSpec(nsId, resourceId)
		if err != nil {
			return "", err
		}
		return specInfo.CspSpecName, nil
	}
	if resourceType == model.StrImage {
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
