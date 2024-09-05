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
	"net/url"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// TbImageReqStructLevelValidation func is for Validation
func TbCustomImageReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbCustomImageReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// RegisterCustomImageWithInfo accepts customimage registration request, creates and returns an TB customimage object
func RegisterCustomImageWithInfo(nsId string, content model.TbCustomImageInfo) (model.TbCustomImageInfo, error) {

	resourceType := model.StrCustomImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbCustomImageInfo{}, err
	}
	err = common.CheckString(content.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbCustomImageInfo{}, err
	}
	check, err := CheckResource(nsId, resourceType, content.Name)

	if check {
		err := fmt.Errorf("The customImage " + content.Name + " already exists.")
		return model.TbCustomImageInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the customImage " + content.Name + ".")
		return model.TbCustomImageInfo{}, err
	}

	content.Namespace = nsId
	content.Id = content.Name
	content.AssociatedObjectList = []string{}

	log.Info().Msg("POST registerCustomImage")
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbCustomImageInfo{}, err
	}

	// "INSERT INTO `image`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	_, err = model.ORM.Insert(content)
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return content, nil
}

// LookupMyImage accepts Spider conn config and CSP myImage ID, lookups and returns the Spider image object
func LookupMyImage(connConfig string, myImageId string) (model.SpiderMyImageInfo, error) {

	if connConfig == "" {
		err := fmt.Errorf("LookupMyImage() called with empty connConfig.")
		log.Error().Err(err).Msg("")
		return model.SpiderMyImageInfo{}, err
	} else if myImageId == "" {
		err := fmt.Errorf("LookupMyImage() called with empty myImageId.")
		log.Error().Err(err).Msg("")
		return model.SpiderMyImageInfo{}, err
	}

	var callResult model.SpiderMyImageInfo
	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	url := model.SpiderRestUrl + "/myimage/" + url.QueryEscape(myImageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig

	err := common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.SpiderMyImageInfo{}
		return content, err
	}

	temp := callResult
	return temp, nil
}

// ConvertSpiderMyImageToTumblebugCustomImage accepts an Spider MyImage object, converts to and returns an TB customImage object
func ConvertSpiderMyImageToTumblebugCustomImage(spiderMyImage model.SpiderMyImageInfo) (model.TbCustomImageInfo, error) {
	if spiderMyImage.IId.NameId == "" {
		err := fmt.Errorf("ConvertSpiderMyImageToTumblebugCustomImage failed; spiderMyImage.IId.NameId == \"\" ")
		return model.TbCustomImageInfo{}, err
	}

	tumblebugCustomImage := model.TbCustomImageInfo{
		CspResourceId:        spiderMyImage.IId.SystemId,
		CspResourceName:      spiderMyImage.IId.NameId, // common.LookupKeyValueList(spiderMyImage.KeyValueList, "Name"),
		Description:          common.LookupKeyValueList(spiderMyImage.KeyValueList, "Description"),
		CreationDate:         spiderMyImage.CreatedTime,
		GuestOS:              "",
		Status:               spiderMyImage.Status,
		KeyValueList:         spiderMyImage.KeyValueList,
		AssociatedObjectList: []string{},
	}
	//tumblebugCustomImage.Id = spiderMyImage.IId.NameId

	spiderKeyValueListName := common.LookupKeyValueList(spiderMyImage.KeyValueList, "Name")
	if len(spiderKeyValueListName) > 0 {
		tumblebugCustomImage.Name = spiderKeyValueListName
	} else {
		tumblebugCustomImage.Name = spiderMyImage.IId.NameId
	}

	return tumblebugCustomImage, nil
}

// RegisterCustomImageWithId accepts customimage creation request, creates and returns an TB customimage object
func RegisterCustomImageWithId(nsId string, u *model.TbCustomImageReq) (model.TbCustomImageInfo, error) {

	resourceType := model.StrCustomImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbCustomImageInfo{}, err
	}

	err = validate.Struct(u)
	if err != nil {

		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return model.TbCustomImageInfo{}, err
		}

		return model.TbCustomImageInfo{}, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		err := fmt.Errorf("The customimage " + u.Name + " already exists.")
		return model.TbCustomImageInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the customimage " + u.Name + ".")
		return model.TbCustomImageInfo{}, err
	}

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	url := ""
	method := ""
	if u.CspResourceId == "" {
		url = fmt.Sprintf("%s/myimage/%s", model.SpiderRestUrl, u.Name)
		method = "GET"
	} else if u.CspResourceId != "" {
		url = fmt.Sprintf("%s/regmyimage", model.SpiderRestUrl)
		method = "POST"
	}
	requestBody := model.SpiderMyImageRegisterReq{
		ConnectionName: u.ConnectionName,
		ReqInfo: struct {
			Name  string
			CSPId string
		}{
			Name:  u.Name,
			CSPId: u.CspResourceId,
		},
	}
	callResult := model.SpiderMyImageInfo{}

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbCustomImageInfo{}, err
	}

	content := model.TbCustomImageInfo{
		Namespace:            nsId,
		Id:                   u.Name,
		Name:                 u.Name,
		ConnectionName:       u.ConnectionName,
		SourceVmId:           "",
		CspResourceId:        callResult.IId.SystemId,
		CspResourceName:      callResult.IId.NameId,
		Description:          u.Description,
		CreationDate:         callResult.CreatedTime,
		GuestOS:              "",
		Status:               callResult.Status,
		KeyValueList:         callResult.KeyValueList,
		AssociatedObjectList: []string{},
		IsAutoGenerated:      false,
	}

	if u.CspResourceId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if u.CspResourceId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}
	return content, nil
}
