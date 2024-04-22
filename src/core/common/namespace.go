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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	//"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	//"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
)

type NsReq struct {
	Name        string `json:"name" example:"ns01"`
	Description string `json:"description" example:"Description for this namespace"`
}

// swagger:response NsInfo
type NsInfo struct {
	Id          string `json:"id" example:"ns01"`
	Name        string `json:"name" example:"ns01"`
	Description string `json:"description" example:"Description for this namespace"`
}

func NsValidation() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			nsId := c.Param("nsId")
			if nsId == "" {
				return next(c)
			}

			err := CheckString(nsId)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "The first character of name must be a lowercase letter, and all following characters must be a dash, lowercase letter, or digit, except the last character, which cannot be a dash.")
			}

			check, err := CheckNs(nsId)

			if !check || err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "Not valid namespace")
			}
			return next(c)
		}
	}
}

func CreateNs(u *NsReq) (NsInfo, error) {
	err := CheckString(u.Name)
	if err != nil {
		temp := NsInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	check, err := CheckNs(u.Name)

	if check {
		temp := NsInfo{}
		err := fmt.Errorf("CreateNs(); The namespace " + u.Name + " already exists.")
		return temp, err
	}

	if err != nil {
		temp := NsInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	content := NsInfo{}
	//content.Id = GenUid()
	content.Id = u.Name
	content.Name = u.Name
	content.Description = u.Description

	Key := "/ns/" + content.Id
	Val, _ := json.Marshal(content)
	err = CBStore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}
	keyValue, _ := CBStore.Get(Key)
	fmt.Println("CreateNs: Key: " + keyValue.Key + "\nValue: " + keyValue.Value)
	return content, nil
}

// UpdateNs is func to update namespace info
func UpdateNs(id string, u *NsReq) (NsInfo, error) {

	res := NsInfo{}
	emptyInfo := NsInfo{}

	err := CheckString(id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}
	check, err := CheckNs(id)

	if !check {
		errString := "The namespace " + id + " does not exist."
		err := fmt.Errorf(errString)
		return emptyInfo, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}

	key := "/ns/" + id
	keyValue, err := CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}

	res.Id = id
	res.Name = u.Name
	res.Description = u.Description

	Key := "/ns/" + id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	Val, err := json.Marshal(res)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}
	err = CBStore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}
	keyValue, err = CBStore.Get(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}
	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyInfo, err
	}
	return res, nil
}

func GetNs(id string) (NsInfo, error) {

	res := NsInfo{}

	err := CheckString(id)
	if err != nil {
		temp := NsInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, err := CheckNs(id)

	if !check {
		errString := "The namespace " + id + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return res, err
	}

	if err != nil {
		temp := NsInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	log.Debug().Msg("[Get namespace] " + id)
	key := "/ns/" + id
	log.Debug().Msg(key)

	keyValue, err := CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return res, err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		log.Error().Err(err).Msg("")
		return res, err
	}
	return res, nil
}

func ListNs() ([]NsInfo, error) {
	log.Debug().Msg("[List namespace]")
	key := "/ns"
	log.Debug().Msg(key)

	keyValue, err := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		res := []NsInfo{}
		for _, v := range keyValue {
			tempObj := NsInfo{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			res = append(res, tempObj)
		}
		return res, nil
		//return true, nil
	}
	return nil, nil // When err == nil && keyValue == nil
}

func AppendIfMissing(slice []string, i string) []string {
	for _, ele := range slice {
		if ele == i {
			return slice
		}
	}
	return append(slice, i)
}

func ListNsId() ([]string, error) {

	key := "/ns"

	var nsList []string

	// Implementation Option 1
	// keyValue, _ := CBStore.GetList(key, true)

	// r, _ := regexp.Compile("/ns/[a-z]([-a-z0-9]*[a-z0-9])?$")

	// for _, v := range keyValue {

	// 	if v.Key == "" {
	// 		continue
	// 	}

	// 	filtered := r.FindString(v.Key)

	// 	if filtered != v.Key {
	// 		continue
	// 	} else {
	// 		trimmedString := strings.TrimPrefix(v.Key, "/ns/")
	// 		nsList = AppendIfMissing(nsList, trimmedString)
	// 	}
	// }
	// EOF of Implementation Option 1

	// Implementation Option 2
	keyValue, err := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		for _, v := range keyValue {
			trimmedString := strings.TrimPrefix(v.Key, "/ns/")
			nsList = append(nsList, trimmedString)
		}
	}
	// EOF of Implementation Option 2

	return nsList, nil

}

func DelNs(id string) error {

	err := CheckString(id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	check, err := CheckNs(id)

	if !check {
		errString := "The namespace " + id + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	log.Debug().Msg("[Delete ns] " + id)
	key := "/ns/" + id
	log.Debug().Msg(key)

	mcisList := GetChildIdList(key + "/mcis")
	imageList := GetChildIdList(key + "/resources/image")
	vNetList := GetChildIdList(key + "/resources/vNet")
	//subnetList := GetChildIdList(key + "/resources/subnet")
	//publicIpList := GetChildIdList(key + "/resources/publicIp")
	securityGroupList := GetChildIdList(key + "/resources/securityGroup")
	specList := GetChildIdList(key + "/resources/spec")
	sshKeyList := GetChildIdList(key + "/resources/sshKey")
	//vNicList := GetChildIdList(key + "/resources/vNic")

	if len(mcisList)+
		len(imageList)+
		len(vNetList)+
		//len(subnetList)
		len(securityGroupList)+
		len(specList)+
		len(sshKeyList) > 0 {
		errString := "Cannot delete NS " + id + ", which is not empty. There exists at least one MCIS or one of resources."
		errString += " \n len(mcisList): " + strconv.Itoa(len(mcisList))
		errString += " \n len(imageList): " + strconv.Itoa(len(imageList))
		errString += " \n len(vNetList): " + strconv.Itoa(len(vNetList))
		//errString += " \n len(publicIpList): " + strconv.Itoa(len(publicIpList))
		errString += " \n len(securityGroupList): " + strconv.Itoa(len(securityGroupList))
		errString += " \n len(specList): " + strconv.Itoa(len(specList))
		errString += " \n len(sshKeyList): " + strconv.Itoa(len(sshKeyList))
		//errString += " \n len(subnetList): " + strconv.Itoa(len(subnetList))
		//errString += " \n len(vNicList): " + strconv.Itoa(len(vNicList))

		err := fmt.Errorf(errString)
		log.Error().Err(err).Msg("")
		return err
	}

	// delete ns info
	err = CBStore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

func DelAllNs() error {

	nsIdList, err := ListNsId()
	if err != nil {
		return err
	}

	if len(nsIdList) == 0 {
		return nil
	}

	for _, v := range nsIdList {
		err := DelNs(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckNs(id string) (bool, error) {

	if id == "" {
		err := fmt.Errorf("CheckNs failed; nsId given is null.")
		return false, err
	}

	err := CheckString(id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	key := "/ns/" + id

	keyValue, _ := CBStore.Get(key)
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}
