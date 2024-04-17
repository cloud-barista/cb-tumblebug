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

// Package mcir is to manage multi-cloud infra resource
package mcir

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	//uuid "github.com/google/uuid"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/go-resty/resty/v2"

	// CB-Store
	cbstore_utils "github.com/cloud-barista/cb-store/utils"
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
	validate.RegisterStructValidation(TbDataDiskReqStructLevelValidation, TbDataDiskReq{})
	validate.RegisterStructValidation(TbImageReqStructLevelValidation, TbImageReq{})
	validate.RegisterStructValidation(TbCustomImageReqStructLevelValidation, TbCustomImageReq{})
	validate.RegisterStructValidation(TbSecurityGroupReqStructLevelValidation, TbSecurityGroupReq{})
	validate.RegisterStructValidation(TbSpecReqStructLevelValidation, TbSpecReq{})
	validate.RegisterStructValidation(TbSshKeyReqStructLevelValidation, TbSshKeyReq{})
	validate.RegisterStructValidation(TbVNetReqStructLevelValidation, TbVNetReq{})
}

// DelAllResources deletes all TB MCIR object of given resourceType
func DelAllResources(nsId string, resourceType string, subString string, forceFlag string) (common.IdList, error) {

	deletedResources := common.IdList{}
	deleteStatus := ""

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
		errString := "There is no " + resourceType + " resource in " + nsId
		err := fmt.Errorf(errString)
		log.Error().Err(err).Msg("")
		return deletedResources, err
	}

	for _, v := range resourceIdList {
		// if subString is provided, check the resourceId contains the subString.
		if subString == "" || strings.Contains(v, subString) {

			deleteStatus = "[Done] "
			errString := ""

			err := DelResource(nsId, resourceType, v, forceFlag)
			if err != nil {
				deleteStatus = "[Failed] "
				errString = " (" + err.Error() + ")"
			}
			deletedResources.IdList = append(deletedResources.IdList, deleteStatus+resourceType+": "+v+errString)
		}
	}
	return deletedResources, nil
}

// DelResource deletes the TB MCIR object
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
	keyValue, _ := common.CBStore.Get(key)
	// In CheckResource() above, calling 'CBStore.Get()' and checking err parts exist.
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

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	switch resourceType {
	case common.StrImage:
		// delete image info
		err := common.CBStore.Delete(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
		_, err = common.ORM.Delete(&TbImageInfo{Namespace: nsId, Id: resourceId})
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}

		return nil
	case common.StrCustomImage:
		temp := TbCustomImageInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = common.SpiderRestUrl + "/myimage/" + temp.CspCustomImageName

		/*
			// delete image info
			err := common.CBStore.Delete(key)
			if err != nil {
				log.Error().Err(err).Msg("")
				return err
			}

			// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
			_, err = common.ORM.Delete(&TbCustomImageInfo{Namespace: nsId, Id: resourceId})
			if err != nil {
				fmt.Println(err.Error())
			} else {
				log.Debug().Msg("Data deleted successfully..")
			}

			return nil
		*/
	case common.StrSpec:
		// delete spec info

		//get related recommend spec
		//keyValue, err := common.CBStore.Get(key)
		content := TbSpecInfo{}
		err := json.Unmarshal([]byte(keyValue.Value), &content)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CBStore.Delete(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		// "DELETE FROM `spec` WHERE `id` = '" + resourceId + "';"
		_, err = common.ORM.Delete(&TbSpecInfo{Namespace: nsId, Id: resourceId})
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}

		return nil
	case common.StrSSHKey:
		temp := TbSshKeyInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = common.SpiderRestUrl + "/keypair/" + temp.CspSshKeyName
	case common.StrVNet:
		temp := TbVNetInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = common.SpiderRestUrl + "/vpc/" + temp.CspVNetName
		childResources = temp.SubnetInfoList
	case common.StrSecurityGroup:
		temp := TbSecurityGroupInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = common.SpiderRestUrl + "/securitygroup/" + temp.CspSecurityGroupName
	case common.StrDataDisk:
		temp := TbDataDiskInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = common.SpiderRestUrl + "/disk/" + temp.CspDataDiskName
	/*
		case "subnet":
			temp := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSubnetId
		case "publicIp":
			temp := publicIpInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			requestBody.ConnectionName = temp.ConnectionName
			url = common.SPIDER_REST_URL + "/publicip/" + temp.CspPublicIpName
		case "vNic":
			temp := vNicInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			requestBody.ConnectionName = temp.ConnectionName
			url = common.SPIDER_REST_URL + "/vnic/" + temp.CspVNicName
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
		common.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	if resourceType == common.StrVNet {
		// var subnetKeys []string
		subnets := childResources.([]TbSubnetInfo)
		for _, v := range subnets {
			subnetKey := common.GenChildResourceKey(nsId, common.StrSubnet, resourceId, v.Id)
			// subnetKeys = append(subnetKeys, subnetKey)
			err = common.CBStore.Delete(subnetKey)
			if err != nil {
				log.Error().Err(err).Msg("")
				// return err
			}
		}
	} else if resourceType == common.StrCustomImage {
		// "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
		_, err = common.ORM.Delete(&TbCustomImageInfo{Namespace: nsId, Id: resourceId})
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("Data deleted successfully..")
		}
	}

	err = common.CBStore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	return nil
}

// DelChildResource deletes the TB MCIR object
func DelChildResource(nsId string, resourceType string, parentResourceId string, resourceId string, forceFlag string) error {

	var parentResourceType string
	switch resourceType {
	case common.StrSubnet:
		parentResourceType = common.StrVNet
	default:
		err := fmt.Errorf("Not valid child resource type.")
		return err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(parentResourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	check, err := CheckResource(nsId, parentResourceType, parentResourceId)

	if !check {
		errString := "The " + parentResourceType + " " + parentResourceId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	check, err = CheckChildResource(nsId, resourceType, parentResourceId, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	parentResourceKey := common.GenResourceKey(nsId, parentResourceType, parentResourceId)
	log.Debug().Msg("parentResourceKey: " + parentResourceKey)

	childResourceKey := common.GenChildResourceKey(nsId, resourceType, parentResourceId, resourceId)
	log.Debug().Msg("childResourceKey: " + childResourceKey)

	parentKeyValue, _ := common.CBStore.Get(parentResourceKey)

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	var url string

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	switch resourceType {
	case common.StrSubnet:
		temp := TbVNetInfo{}
		err = json.Unmarshal([]byte(parentKeyValue.Value), &temp)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		requestBody.ConnectionName = temp.ConnectionName
		url = fmt.Sprintf("%s/vpc/%s/subnet/%s", common.SpiderRestUrl, temp.Name, resourceId)
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
		common.ShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CBStore.Delete(childResourceKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Delete the child element in parent resources' array
	switch resourceType {
	case common.StrSubnet:
		oldVNet := TbVNetInfo{}
		err = json.Unmarshal([]byte(parentKeyValue.Value), &oldVNet)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		newVNet := TbVNetInfo{}
		newVNet = oldVNet

		var subnetIndex int
		subnetIndex = -1
		for i, v := range newVNet.SubnetInfoList {
			if v.Name == resourceId {
				subnetIndex = i
				break
			}
		}

		if subnetIndex != -1 {
			DelEleInSlice(&newVNet.SubnetInfoList, subnetIndex)
		} else {
			err := fmt.Errorf("Failed to find and delete subnet %s in vNet %s.", resourceId, parentResourceId)
			log.Error().Err(err).Msg("")
		}

		Val, _ := json.Marshal(newVNet)
		err = common.CBStore.Put(parentResourceKey, string(Val))
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
		// default:
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

// ListResourceId returns the list of TB MCIR object IDs of given resourceType
func ListResourceId(nsId string, resourceType string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if resourceType == common.StrImage ||
		resourceType == common.StrCustomImage ||
		resourceType == common.StrSSHKey ||
		resourceType == common.StrSpec ||
		resourceType == common.StrVNet ||
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" ||
		resourceType == common.StrSecurityGroup ||
		resourceType == common.StrDataDisk {
		// continue
	} else {
		err = fmt.Errorf("invalid resource type")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	key := "/ns/" + nsId + "/resources/"
	keyValue, err := common.CBStore.GetList(key, true)

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

// ListResource returns the list of TB MCIR objects of given resourceType
func ListResource(nsId string, resourceType string, filterKey string, filterVal string) (interface{}, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if resourceType == common.StrImage ||
		resourceType == common.StrCustomImage ||
		resourceType == common.StrSSHKey ||
		resourceType == common.StrSpec ||
		resourceType == common.StrVNet ||
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" ||
		resourceType == common.StrSecurityGroup ||
		resourceType == common.StrDataDisk {
		// continue
	} else {
		errString := "Cannot list " + resourceType + "s."
		err := fmt.Errorf(errString)
		return nil, err
	}

	log.Debug().Msg("[Get] " + resourceType + " list")
	key := "/ns/" + nsId + "/resources/" + resourceType
	log.Debug().Msg(key)

	keyValue, err := common.CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		switch resourceType {
		case common.StrImage:
			res := []TbImageInfo{}
			for _, v := range keyValue {

				tempObj := TbImageInfo{}
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
		case common.StrCustomImage:
			res := []TbCustomImageInfo{}
			for _, v := range keyValue {

				tempObj := TbCustomImageInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}

				// Update TB CustomImage object's 'status' field
				// Just calling GetResource(customImage) once will update TB CustomImage object's 'status' field
				newObj, err := GetResource(nsId, common.StrCustomImage, tempObj.Id)
				// do not return here to gather whole list. leave error message in the return body.
				if newObj != nil {
					tempObj = newObj.(TbCustomImageInfo)
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
		case common.StrSecurityGroup:
			res := []TbSecurityGroupInfo{}
			for _, v := range keyValue {
				tempObj := TbSecurityGroupInfo{}
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
		case common.StrSpec:
			res := []TbSpecInfo{}
			for _, v := range keyValue {
				tempObj := TbSpecInfo{}
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
		case common.StrSSHKey:
			res := []TbSshKeyInfo{}
			for _, v := range keyValue {
				tempObj := TbSshKeyInfo{}
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
		case common.StrVNet:
			res := []TbVNetInfo{}
			for _, v := range keyValue {
				tempObj := TbVNetInfo{}
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
		case common.StrDataDisk:
			res := []TbDataDiskInfo{}
			for _, v := range keyValue {
				tempObj := TbDataDiskInfo{}
				err = json.Unmarshal([]byte(v.Value), &tempObj)
				if err != nil {
					log.Error().Err(err).Msg("")
					return nil, err
				}

				// Update TB DataDisk object's 'status' field
				// Just calling GetResource(dataDisk) once will update TB DataDisk object's 'status' field
				newObj, err := GetResource(nsId, common.StrDataDisk, tempObj.Id)
				if err != nil {
					log.Error().Err(err).Msg("")
					tempObj.Status = "Failed"
					tempObj.SystemMessage = err.Error()
				} else {
					tempObj = newObj.(TbDataDiskInfo)
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
		case common.StrImage:
			return []TbImageInfo{}, nil
		case common.StrCustomImage:
			return []TbCustomImageInfo{}, nil
		case common.StrSecurityGroup:
			return []TbSecurityGroupInfo{}, nil
		case common.StrSpec:
			return []TbSpecInfo{}, nil
		case common.StrSSHKey:
			return []TbSshKeyInfo{}, nil
		case common.StrVNet:
			return []TbVNetInfo{}, nil
		case common.StrDataDisk:
			return []TbDataDiskInfo{}, nil
		}
	}

	err = fmt.Errorf("Some exceptional case happened. Please check the references of " + common.GetFuncName())
	return nil, err // if interface{} == nil, make err be returned. Should not come this part if there is no err.
}

// GetAssociatedObjectCount returns the number of MCIR's associated Tumblebug objects
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

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return -1, err
	}
	if keyValue != nil {
		inUseCount := int(gjson.Get(keyValue.Value, "associatedObjectList.#").Int())
		return inUseCount, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return -1, err
}

// GetAssociatedObjectList returns the list of MCIR's associated Tumblebug objects
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

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {

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

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
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
	log.Debug().Msg("[Set count] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	if keyValue != nil {
		objList, _ := GetAssociatedObjectList(nsId, resourceType, resourceId)
		switch cmd {
		case common.StrAdd:
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
		case common.StrDelete:
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
		err = common.CBStore.Put(key, keyValue.Value)
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

// GetResource returns the requested TB MCIR object
func GetResource(nsId string, resourceType string, resourceId string) (interface{}, error) {

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

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		switch resourceType {
		case common.StrImage:
			res := TbImageInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case common.StrCustomImage:
			res := TbCustomImageInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}

			// Update TB CustomImage object's 'status' field
			url := fmt.Sprintf("%s/myimage/%s", common.SpiderRestUrl, res.CspCustomImageName)

			client := resty.New().SetCloseConnection(true)
			client.SetAllowGetMethodPayload(true)

			connectionName := common.SpiderConnectionName{
				ConnectionName: res.ConnectionName,
			}

			req := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(connectionName).
				SetResult(&SpiderMyImageInfo{}) // or SetResult(AuthSuccess{}).
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

			updatedSpiderMyImage := resp.Result().(*SpiderMyImageInfo)
			res.Status = updatedSpiderMyImage.Status
			fmt.Printf("res.Status: %s \n", res.Status) // for debug
			UpdateResourceObject(nsId, common.StrCustomImage, res)

			return res, nil
		case common.StrSecurityGroup:
			res := TbSecurityGroupInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case common.StrSpec:
			res := TbSpecInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case common.StrSSHKey:
			res := TbSshKeyInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case common.StrVNet:
			res := TbVNetInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			return res, nil
		case common.StrDataDisk:
			res := TbDataDiskInfo{}
			err = json.Unmarshal([]byte(keyValue.Value), &res)
			if err != nil {
				log.Error().Err(err).Msg("")
				return res, err
			}

			// Update TB DataDisk object's 'status' field
			url := fmt.Sprintf("%s/disk/%s", common.SpiderRestUrl, res.CspDataDiskName)

			client := resty.New().SetCloseConnection(true)
			client.SetAllowGetMethodPayload(true)

			connectionName := common.SpiderConnectionName{
				ConnectionName: res.ConnectionName,
			}

			req := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(connectionName).
				SetResult(&SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
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

			updatedSpiderDisk := resp.Result().(*SpiderDiskInfo)
			res.Status = updatedSpiderDisk.Status
			fmt.Printf("res.Status: %s \n", res.Status) // for debug
			UpdateResourceObject(nsId, common.StrDataDisk, res)

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

// CheckResource returns the existence of the TB MCIR resource in bool form.
func CheckResource(nsId string, resourceType string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckResource failed; nsId given is null.")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("CheckResource failed; resourceId given is null.")
		return false, err
	}

	// Check resourceType's validity
	if resourceType == common.StrImage ||
		resourceType == common.StrCustomImage ||
		resourceType == common.StrSSHKey ||
		resourceType == common.StrSpec ||
		resourceType == common.StrVNet ||
		resourceType == common.StrSecurityGroup ||
		resourceType == common.StrDataDisk {
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

	err = common.CheckString(resourceId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

// CheckChildResource returns the existence of the TB MCIR resource in bool form.
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
	if resourceType == common.StrSubnet {
		parentResourceType = common.StrVNet
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

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if keyValue != nil {
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
	if resourceType == common.StrImage ||
		resourceType == common.StrSSHKey ||
		resourceType == common.StrSpec ||
		resourceType == common.StrVNet ||
		resourceType == common.StrSecurityGroup {
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

// LoadCommonResource is to register common resources from asset files (../assets/*.csv)
func LoadCommonResource() (common.IdList, error) {

	regiesteredIds := common.IdList{}
	regiesteredStatus := ""

	// WaitGroups for goroutine
	var waitSpecImg sync.WaitGroup
	var wait sync.WaitGroup

	// Check common namespace. Create one if not.
	_, err := common.GetNs(common.SystemCommonNs)
	if err != nil {
		nsReq := common.NsReq{}
		nsReq.Name = common.SystemCommonNs
		nsReq.Description = "Namespace for common resources"
		_, nsErr := common.CreateNs(&nsReq)
		if nsErr != nil {
			log.Error().Err(nsErr).Msg("")
			return regiesteredIds, nsErr
		}
	}

	connectionList, err := common.GetConnConfigList()
	if err != nil {
		log.Error().Err(err).Msg("Cannot GetConnConfigList")
		return regiesteredIds, err
	}
	if len(connectionList.Connectionconfig) == 0 {
		log.Error().Err(err).Msg("No registered connection config")
		return regiesteredIds, err
	}

	// LookupSpecList and LookupImageList of all connections in parallel
	var specMap sync.Map
	// ignoreConnectionMap is used to store connection names that failed to lookup specs
	var ignoreConnectionMap sync.Map
	startTime := time.Now()
	var wg sync.WaitGroup
	for _, connConfig := range connectionList.Connectionconfig {
		wg.Add(1)
		go func(connConfig common.ConnConfig) {
			defer wg.Done()
			specsInConnection, err := LookupSpecList(connConfig.ConfigName)
			if err != nil {
				log.Error().Err(err).Msgf("Cannot LookupSpecList in %s", connConfig.ConfigName)
				ignoreConnectionMap.Store(connConfig.ConfigName, err)
				return
			}
			log.Info().Msgf("[%s] #Spec: %d", connConfig.ConfigName, len(specsInConnection.Vmspec))
			for _, spec := range specsInConnection.Vmspec {
				key := GenSpecMapKey(connConfig.RegionName, spec.Name)
				// instead of connConfig.RegionName, spec.Region will be used in the future
				//log.Info().Msgf("specMap.Store(%s, spec)", key)
				specMap.Store(key, spec)
			}
		}(connConfig)
	}

	// LookupImageList of all connections in parallel takes too long time
	// disable it for now

	// for i := len(connectionList.Connectionconfig) - 1; i >= 0; i-- {
	// 	connConfig := connectionList.Connectionconfig[i]
	// 	wg.Add(1)
	// 	go func(cc common.ConnConfig) {
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
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)

	log.Info().Msgf("LookupSpecList in parallel: %s", totalDuration)

	// Read common specs and register spec objects
	file, fileErr := os.Open("../assets/cloudspec.csv")
	defer file.Close()
	if fileErr != nil {
		log.Error().Err(fileErr).Msg("")
		return regiesteredIds, fileErr
	}

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
					newRow[1] = connConfig.ConfigName
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
	defer file.Close()
	if fileErr != nil {
		log.Error().Err(fileErr).Msg("")
		return regiesteredIds, fileErr
	}

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
					newRow[1] = connConfig.ConfigName
					newRowsImg = append(newRowsImg, newRow)
					//log.Info().Msgf("Expended row: %s", newRow)
				}
			}
		} else {
			newRowsImg = append(newRowsImg, row)
		}
	}
	rowsImg = newRowsImg

	waitSpecImg.Add(1)
	go func(rowsSpec [][]string) {
		defer waitSpecImg.Done()
		//lenSpecs := len(rowsSpec[1:])
		for i, row := range rowsSpec[1:] {
			// wait.Add(1)
			// go func(i int, row []string, lenSpecs int) {
			// 	defer wait.Done()
			// 	common.RandomSleep(0, lenSpecs/20)

			specReqTmp := TbSpecReq{}
			// 0	providerName
			// 1	connectionName
			// 2	cspSpecName
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

			providerName := row[0]
			specReqTmp.ConnectionName = row[1]
			specReqTmp.CspSpecName = row[2]

			_, ok := ignoreConnectionMap.Load(specReqTmp.ConnectionName)
			if !ok {
				// Give a name for spec object by combining ConnectionName and CspSpecName
				// To avoid naming-rule violation, modify the string
				specReqTmp.Name = specReqTmp.ConnectionName + "-" + specReqTmp.CspSpecName
				specReqTmp.Name = ToNamingRuleCompatible(specReqTmp.Name)
				specInfoId := specReqTmp.Name
				rootDiskType := row[14]
				rootDiskSize := row[15]
				specReqTmp.Description = "Common Spec Resource"

				regionName := specReqTmp.ConnectionName // assume regionName is the same as connectionName
				regiesteredStatus = ""

				var errRegisterSpec error

				log.Trace().Msgf("[%d] register Common Spec: %s", i, specReqTmp.Name)

				// Register Spec object
				searchKey := GenSpecMapKey(regionName, specReqTmp.CspSpecName)
				value, ok := specMap.Load(searchKey)
				if ok {
					spiderSpec := value.(SpiderSpecInfo)
					//log.Info().Msgf("Found spec in the map: %s", spiderSpec.Name)
					tumblebugSpec, errConvert := ConvertSpiderSpecToTumblebugSpec(spiderSpec)
					if errConvert != nil {
						log.Error().Err(errConvert).Msg("Cannot ConvertSpiderSpecToTumblebugSpec")
					}

					tumblebugSpec.Name = specInfoId
					tumblebugSpec.ConnectionName = specReqTmp.ConnectionName
					_, errRegisterSpec = RegisterSpecWithInfo(common.SystemCommonNs, &tumblebugSpec, true)
					if errRegisterSpec != nil {
						log.Info().Err(errRegisterSpec).Msg("RegisterSpec WithInfo failed")
					}

				} else {
					errRegisterSpec = fmt.Errorf("Not Found spec from the fetched spec list: %s", searchKey)
					log.Info().Msgf(errRegisterSpec.Error())
					// _, errRegisterSpec = RegisterSpecWithCspSpecName(common.SystemCommonNs, &specReqTmp, true)
					// if errRegisterSpec != nil {
					// 	log.Error().Err(errRegisterSpec).Msg("RegisterSpec WithCspSpecName failed")
					// }
				}

				if errRegisterSpec != nil {
					regiesteredStatus += "  [Failed] " + errRegisterSpec.Error()
				} else {
					// Update registered Spec object with givn info from asset file
					// Update registered Spec object with Cost info
					costPerHour, err2 := strconv.ParseFloat(strings.ReplaceAll(row[3], " ", ""), 32)
					if err2 != nil {
						log.Error().Err(err2).Msg("Not valid CostPerHour value in the asset")
						costPerHour = 99999999.9
					}
					evaluationScore01, err2 := strconv.ParseFloat(strings.ReplaceAll(row[4], " ", ""), 32)
					if err2 != nil {
						log.Error().Err(err2).Msg("Not valid evaluationScore01 value in the asset")
						evaluationScore01 = -99.9
					}
					specUpdateRequest :=
						TbSpecInfo{
							ProviderName:      providerName,
							RegionName:        regionName,
							CostPerHour:       float32(costPerHour),
							RootDiskType:      rootDiskType,
							RootDiskSize:      rootDiskSize,
							EvaluationScore01: float32(evaluationScore01),
						}

					_, err3 := UpdateSpec(common.SystemCommonNs, specInfoId, specUpdateRequest)
					if err3 != nil {
						log.Error().Err(err3).Msg("UpdateSpec failed")
						regiesteredStatus += "  [Failed] " + err3.Error()
					}
					//fmt.Printf("[%d] Registered Common Spec\n", i)
					//common.PrintJsonPretty(updatedSpecInfo)
				}

				regiesteredIds.AddItem(common.StrSpec + ": " + specInfoId + regiesteredStatus)
				// }(i, row, lenSpecs)
			}
		}
		wait.Wait()
	}(rowsSpec)

	waitSpecImg.Add(1)
	go func(rowsImg [][]string) {
		defer waitSpecImg.Done()
		lenImages := len(rowsImg[1:])
		for i, row := range rowsImg[1:] {
			wait.Add(1)
			// fmt.Printf("[%d] i, row := range rowsImg[1:] %s\n", i, row)
			// goroutine
			go func(i int, row []string, lenImages int) {
				defer wait.Done()

				imageReqTmp := TbImageReq{}
				// row0: ProviderName
				// row1: connectionName
				// row2: cspImageId
				// row3: OsType
				imageReqTmp.ConnectionName = row[1]
				_, ok := ignoreConnectionMap.Load(imageReqTmp.ConnectionName)
				if !ok {
					// RandomSleep for safe parallel executions
					common.RandomSleep(0, lenImages/8)

					imageReqTmp.CspImageId = row[2]
					osType := strings.ReplaceAll(row[3], " ", "")
					// Give a name for spec object by combining ConnectionName and OsType
					// To avoid naming-rule violation, modify the string
					imageReqTmp.Name = imageReqTmp.ConnectionName + "-" + osType
					imageReqTmp.Name = ToNamingRuleCompatible(imageReqTmp.Name)
					imageInfoId := imageReqTmp.Name
					imageReqTmp.Description = "Common Image Resource"

					log.Trace().Msgf("[%d] register Common Image: %s", i, imageReqTmp.Name)

					// Register Spec object
					regiesteredStatus = ""

					_, err1 := RegisterImageWithId(common.SystemCommonNs, &imageReqTmp, true)
					if err1 != nil {
						log.Info().Msg(err1.Error())
						regiesteredStatus += "  [Failed] " + err1.Error()
					} else {
						// Update registered image object with OsType info
						imageUpdateRequest := TbImageInfo{GuestOS: osType}
						_, err2 := UpdateImage(common.SystemCommonNs, imageInfoId, imageUpdateRequest)
						if err2 != nil {
							log.Error().Err(err2).Msg("UpdateImage failed")
							regiesteredStatus += "  [Failed] " + err2.Error()
						}
					}

					//regiesteredStatus = strings.Replace(regiesteredStatus, "\\", "", -1)
					regiesteredIds.AddItem(common.StrImage + ": " + imageInfoId + regiesteredStatus)
				}
			}(i, row, lenImages)
		}
		wait.Wait()
	}(rowsImg)

	waitSpecImg.Wait()
	sort.Strings(regiesteredIds.IdList)
	log.Info().Msgf("Registered Common Resources %d", len(regiesteredIds.IdList))

	return regiesteredIds, nil
}

// LoadDefaultResource is to register default resource from asset files (../assets/*.csv)
func LoadDefaultResource(nsId string, resType string, connectionName string) error {

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

	// Read default resources from file and create objects

	connectionList, err := common.GetConnConfigList()
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
		log.Error().Err(err).Msg("Failed to LoadDefaultResource")
		return err
	}
	sliceIndex = (sliceIndex % 254) + 1

	//resourceName := connectionName
	// Default resource name has this pattern (nsId + "-systemdefault-" + connectionName)
	resourceName := nsId + common.StrDefaultResourceName + connectionName
	description := "Generated Default Resource"

	for _, resType := range resList {
		if resType == "vnet" {
			log.Debug().Msg("vnet")

			reqTmp := TbVNetReq{}
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
			subnet := TbSubnetReq{Name: subnetName, IPv4_CIDR: subnetCidr}
			reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)

			subnetName = reqTmp.Name + "-01"
			subnetCidr = "10." + strconv.Itoa(sliceIndex) + ".64.0/18"
			subnet = TbSubnetReq{Name: subnetName, IPv4_CIDR: subnetCidr}
			reqTmp.SubnetInfoList = append(reqTmp.SubnetInfoList, subnet)

			common.PrintJsonPretty(reqTmp)

			resultInfo, err := CreateVNet(nsId, &reqTmp, "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to create vNet")
				return err
			}
			common.PrintJsonPretty(resultInfo)
		} else if resType == "sg" || resType == "securitygroup" {
			log.Debug().Msg("sg")

			reqTmp := TbSecurityGroupReq{}

			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			reqTmp.VNetId = resourceName

			// open all firewall for default securityGroup
			rule := TbFirewallRuleInfo{FromPort: "1", ToPort: "65535", IPProtocol: "tcp", Direction: "inbound", CIDR: "0.0.0.0/0"}
			var ruleList []TbFirewallRuleInfo
			ruleList = append(ruleList, rule)
			rule = TbFirewallRuleInfo{FromPort: "1", ToPort: "65535", IPProtocol: "udp", Direction: "inbound", CIDR: "0.0.0.0/0"}
			ruleList = append(ruleList, rule)
			// CloudIt only offers tcp, udp Protocols
			if !strings.EqualFold(provider, "cloudit") {
				rule = TbFirewallRuleInfo{FromPort: "-1", ToPort: "-1", IPProtocol: "icmp", Direction: "inbound", CIDR: "0.0.0.0/0"}
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

			reqTmp := TbSshKeyReq{}

			reqTmp.ConnectionName = connectionName
			reqTmp.Name = resourceName
			reqTmp.Description = description

			common.PrintJsonPretty(reqTmp)

			resultInfo, err := CreateSshKey(nsId, &reqTmp, "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to create SshKey")
				return err
			}
			common.PrintJsonPretty(resultInfo)
		} else {
			return errors.New("Not valid option (provide sg, sshkey, vnet, or all)")
		}
	}

	return nil
}

// DelAllDefaultResources deletes all Default securityGroup, sshKey, vNet objects
func DelAllDefaultResources(nsId string) (common.IdList, error) {

	output := common.IdList{}
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return output, err
	}

	matchedSubstring := nsId + common.StrDefaultResourceName

	list, err := DelAllResources(nsId, common.StrSecurityGroup, matchedSubstring, "false")
	if err != nil {
		log.Error().Err(err).Msg("")
		output.IdList = append(output.IdList, err.Error())
	}
	output.IdList = append(output.IdList, list.IdList...)

	list, err = DelAllResources(nsId, common.StrSSHKey, matchedSubstring, "false")
	if err != nil {
		log.Error().Err(err).Msg("")
		output.IdList = append(output.IdList, err.Error())
	}
	output.IdList = append(output.IdList, list.IdList...)

	list, err = DelAllResources(nsId, common.StrVNet, matchedSubstring, "false")
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

	// Replace all non-alphanumeric characters with '-'
	nonAlphanumericRegex := regexp.MustCompile(`[^a-z0-9]+`)
	rawName = nonAlphanumericRegex.ReplaceAllString(rawName, "-")

	// Remove leading and trailing '-' from the result string
	trimLeadingTrailingDashRegex := regexp.MustCompile(`^-+|-+$`)
	rawName = trimLeadingTrailingDashRegex.ReplaceAllString(rawName, "")

	return rawName
}

// UpdateResourceObject is func to update the resource object
func UpdateResourceObject(nsId string, resourceType string, resourceObject interface{}) {
	resourceId, err := GetIdFromStruct(resourceObject)
	fmt.Printf("in UpdateResourceObject; extracted resourceId: %s \n", resourceId) // for debug
	if resourceId == "" || err != nil {
		fmt.Printf("in UpdateResourceObject; failed to extract resourceId. \n") // for debug
		return
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)

	// Check existence of the key. If no key, no update.
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
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
			err = common.CBStore.Put(key, string(newJSON))
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
		err = common.CBStore.Put(key, string(val))
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

}

/*
func AreEqualJSON(s1, s2 string) (bool, error) {
	var o1 interface{}
	var o2 interface{}

	var err error
	err = json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 1 :: %s", err.Error())
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 2 :: %s", err.Error())
	}

	return reflect.DeepEqual(o1, o2), nil
}
*/
