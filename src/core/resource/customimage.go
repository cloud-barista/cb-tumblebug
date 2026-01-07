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
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// ImageReqStructLevelValidation func is for Validation
func CustomImageReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.CustomImageReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// RegisterCustomImageWithInfo accepts customimage registration request, creates and returns an TB customimage object
func RegisterCustomImageWithInfo(nsId string, content model.ImageInfo) (model.ImageInfo, error) {

	resourceType := model.StrCustomImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}
	err = common.CheckString(content.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}
	check, err := CheckResource(nsId, resourceType, content.Name)

	if check {
		err := fmt.Errorf("The customImage " + content.Name + " already exists.")
		return model.ImageInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the customImage " + content.Name + ".")
		return model.ImageInfo{}, err
	}

	// Set required fields based on new structure
	content.ResourceType = resourceType
	content.Namespace = nsId
	content.Id = content.Name

	// Generate Uid if not set
	if content.Uid == "" {
		content.Uid = common.GenUid()
	}

	// "INSERT INTO `custom_image`(`namespace`, `provider_name`, `csp_image_name`, ...) VALUES ('nsId', 'content.ProviderName', 'content.CspImageName', ...);
	result := model.ORM.Create(&content)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to insert custom image to database")
		return model.ImageInfo{}, result.Error
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
	client := clientManager.NewHttpClient()
	client.SetTimeout(2 * time.Minute)
	url := model.SpiderRestUrl + "/myimage/" + url.QueryEscape(myImageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig

	_, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
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
func ConvertSpiderMyImageToTumblebugCustomImage(connConfig model.ConnConfig, spiderMyImage model.SpiderMyImageInfo) (model.ImageInfo, error) {
	if spiderMyImage.IId.NameId == "" {
		err := fmt.Errorf("ConvertSpiderMyImageToTumblebugCustomImage failed; spiderMyImage.IId.NameId == \"\" ")
		return model.ImageInfo{}, err
	}

	// Extract name from KeyValueList or use IId.NameId
	imageName := common.LookupKeyValueList(spiderMyImage.KeyValueList, "Name")
	if len(imageName) == 0 {
		imageName = spiderMyImage.IId.NameId
	}

	// Extract description from KeyValueList
	description := common.LookupKeyValueList(spiderMyImage.KeyValueList, "Description")

	tumblebugCustomImage := model.ImageInfo{
		// CustomImage-specific fields
		ResourceType: model.StrCustomImage,
		CspImageId:   spiderMyImage.IId.SystemId,
		SourceVmUid:  "", // This should be filled by caller if available

		// Composite primary key fields
		ProviderName: connConfig.ProviderName,
		CspImageName: spiderMyImage.IId.NameId,

		// Array field
		RegionList: []string{connConfig.RegionDetail.RegionName},

		// Identifiers
		Name:           imageName,
		ConnectionName: connConfig.ConfigName,

		// Time fields
		CreationDate: spiderMyImage.CreatedTime.Format(time.RFC3339),

		// Status
		ImageStatus: model.ImageStatus(spiderMyImage.Status),

		// Additional information
		Details:     spiderMyImage.KeyValueList,
		Description: description,
	}

	return tumblebugCustomImage, nil
}

// RegisterCustomImageWithId accepts customimage creation request, creates and returns an TB customimage object
func RegisterCustomImageWithId(nsId string, u *model.CustomImageReq) (model.ImageInfo, error) {

	resourceType := model.StrCustomImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	err = validate.Struct(u)
	if err != nil {

		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return model.ImageInfo{}, err
		}

		return model.ImageInfo{}, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		err := fmt.Errorf("The customimage " + u.Name + " already exists.")
		return model.ImageInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the customimage " + u.Name + ".")
		return model.ImageInfo{}, err
	}

	client := clientManager.NewHttpClient()
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

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	// Get connection config for provider and region information
	connConfig, err := common.GetConnConfig(u.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig: " + err.Error())
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	// Create ImageInfo based on new structure
	content := model.ImageInfo{
		// CustomImage-specific fields
		ResourceType: resourceType,
		CspImageId:   callResult.IId.SystemId,
		SourceVmUid:  "", // Not available for registered images

		// Composite primary key fields
		Namespace:    nsId,
		ProviderName: connConfig.ProviderName,
		CspImageName: callResult.IId.NameId,

		// Array field
		RegionList: []string{connConfig.RegionDetail.RegionName},

		// Identifiers
		Id:             u.Name,
		Uid:            common.GenUid(),
		Name:           u.Name,
		ConnectionName: u.ConnectionName,

		// Time fields
		FetchedTime:  time.Now().Format(time.RFC3339),
		CreationDate: callResult.CreatedTime.Format(time.RFC3339),

		// Status
		ImageStatus: model.ImageStatus(callResult.Status),

		// Additional information
		Details:     callResult.KeyValueList,
		Description: u.Description,
	}

	// Set system label based on registration source
	if u.CspResourceId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else {
		content.SystemLabel = "Registered from CSP resource"
	}

	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	// Insert into database
	result := model.ORM.Create(&content)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to insert custom image to database")
		return content, result.Error
	}

	return content, nil
}
