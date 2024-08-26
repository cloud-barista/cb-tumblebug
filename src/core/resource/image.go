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
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"

	validator "github.com/go-playground/validator/v10"
)

// TbImageReqStructLevelValidation func is for Validation
func TbImageReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbImageReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// ConvertSpiderImageToTumblebugImage accepts an Spider image object, converts to and returns an TB image object
func ConvertSpiderImageToTumblebugImage(spiderImage model.SpiderImageInfo) (model.TbImageInfo, error) {
	if spiderImage.IId.NameId == "" {
		err := fmt.Errorf("ConvertSpiderImageToTumblebugImage failed; spiderImage.IId.NameId == EmptyString")
		emptyTumblebugImage := model.TbImageInfo{}
		return emptyTumblebugImage, err
	}

	tumblebugImage := model.TbImageInfo{}
	//tumblebugImage.Id = spiderImage.IId.NameId

	spiderKeyValueListName := common.LookupKeyValueList(spiderImage.KeyValueList, "Name")
	if len(spiderKeyValueListName) > 0 {
		tumblebugImage.Name = spiderKeyValueListName
	} else {
		tumblebugImage.Name = spiderImage.IId.NameId
	}

	tumblebugImage.CspImageId = spiderImage.IId.NameId
	tumblebugImage.CspImageName = common.LookupKeyValueList(spiderImage.KeyValueList, "Name")
	tumblebugImage.Description = common.LookupKeyValueList(spiderImage.KeyValueList, "Description")
	tumblebugImage.CreationDate = common.LookupKeyValueList(spiderImage.KeyValueList, "CreationDate")
	tumblebugImage.GuestOS = spiderImage.GuestOS
	tumblebugImage.Status = spiderImage.Status
	tumblebugImage.KeyValueList = spiderImage.KeyValueList

	return tumblebugImage, nil
}

// RegisterImageWithId accepts image creation request, creates and returns an TB image object
func RegisterImageWithId(nsId string, u *model.TbImageReq, update bool, RDBonly bool) (model.TbImageInfo, error) {

	content := model.TbImageInfo{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	resourceType := model.StrImage
	if !RDBonly {
		check, err := CheckResource(nsId, resourceType, u.Name)
		if !update {
			if check {
				err := fmt.Errorf("The image " + u.Name + " already exists.")
				return content, err
			}
		}
		if err != nil {
			err := fmt.Errorf("Failed to check the existence of the image " + u.Name + ".")
			return content, err
		}
	}

	res, err := LookupImage(u.ConnectionName, u.CspImageId)
	if err != nil {
		log.Trace().Err(err).Msg("")
		return content, err
	}
	if res.IId.NameId == "" {
		err := fmt.Errorf("CB-Spider returned empty IId.NameId without Error: %s", u.ConnectionName)
		log.Error().Err(err).Msgf("Cannot LookupImage %s %v", u.CspImageId, res)
		return content, err
	}

	content, err = ConvertSpiderImageToTumblebugImage(res)
	if err != nil {
		log.Error().Err(err).Msg("")
		//err := fmt.Errorf("an error occurred while converting Spider image info to Tumblebug image info.")
		return content, err
	}
	content.Namespace = nsId
	content.ConnectionName = u.ConnectionName
	content.Id = u.Name
	content.Name = u.Name
	content.AssociatedObjectList = []string{}

	if !RDBonly {
		Key := common.GenResourceKey(nsId, resourceType, content.Id)
		Val, _ := json.Marshal(content)
		err = kvstore.Put(Key, string(Val))
		if err != nil {
			log.Error().Err(err).Msg("")
			return content, err
		}
	}

	// "INSERT INTO `image`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	// Attempt to insert the new record
	_, err = model.ORM.Insert(content)
	if err != nil {
		if update {
			// If insert fails and update is true, attempt to update the existing record
			_, updateErr := model.ORM.Update(content, &model.TbSpecInfo{Namespace: content.Namespace, Id: content.Id})
			if updateErr != nil {
				log.Error().Err(updateErr).Msg("Error updating spec after insert failure")
				return content, updateErr
			} else {
				log.Trace().Msg("SQL: Update success after insert failure")
			}
		} else {
			log.Error().Err(err).Msg("Error inserting spec and update flag is false")
			return content, err
		}
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return content, nil
}

// RegisterImageWithInfo accepts image creation request, creates and returns an TB image object
func RegisterImageWithInfo(nsId string, content *model.TbImageInfo, update bool) (model.TbImageInfo, error) {

	resourceType := model.StrImage

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbImageInfo{}, err
	}
	err = common.CheckString(content.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbImageInfo{}, err
	}
	check, err := CheckResource(nsId, resourceType, content.Name)

	if !update {
		if check {
			err := fmt.Errorf("The image " + content.Name + " already exists.")
			return model.TbImageInfo{}, err
		}
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the image " + content.Name + ".")
		return model.TbImageInfo{}, err
	}

	content.Namespace = nsId
	//content.Id = common.GenUid()
	content.Id = content.Name
	content.AssociatedObjectList = []string{}

	log.Info().Msg("PUT registerImage")
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbImageInfo{}, err
	}

	// "INSERT INTO `image`(`namespace`, `id`, ...) VALUES ('nsId', 'content.Id', ...);
	_, err = model.ORM.Insert(content)
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Trace().Msg("SQL: Insert success")
	}

	return *content, nil
}

// LookupImageList accepts Spider conn config,
// lookups and returns the list of all images in the region of conn config
// in the form of the list of Spider image objects
func LookupImageList(connConfig string) (model.SpiderImageList, error) {

	if connConfig == "" {
		content := model.SpiderImageList{}
		err := fmt.Errorf("LookupImage() called with empty connConfig.")
		log.Error().Err(err).Msg("")
		return content, err
	}

	url := model.SpiderRestUrl + "/vmimage"

	// Create Req body
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderImageList{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Get(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.SpiderImageList{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	log.Debug().Msg(string(resp.Body()))

	fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := model.SpiderImageList{}
		return content, err
	}

	temp := resp.Result().(*model.SpiderImageList)
	return *temp, nil

}

// LookupImage accepts Spider conn config and CSP image ID, lookups and returns the Spider image object
func LookupImage(connConfig string, imageId string) (model.SpiderImageInfo, error) {

	if connConfig == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("LookupImage() called with empty connConfig.")
		log.Error().Err(err).Msg("")
		return content, err
	} else if imageId == "" {
		content := model.SpiderImageInfo{}
		err := fmt.Errorf("LookupImage() called with empty imageId.")
		log.Error().Err(err).Msg("")
		return content, err
	}

	client := resty.New()
	client.SetTimeout(2 * time.Minute)
	url := model.SpiderRestUrl + "/vmimage/" + url.QueryEscape(imageId)
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfig
	callResult := model.SpiderImageInfo{}

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
		log.Trace().Err(err).Msg("")
		return callResult, err
	}

	return callResult, nil
}

// FetchImagesForAllConnConfigs gets all conn configs from Spider, lookups all images for each region of conn config, and saves into TB image objects
func FetchImagesForConnConfig(connConfig string, nsId string) (imageCount uint, err error) {
	log.Debug().Msg("FetchImagesForConnConfig(" + connConfig + ")")

	spiderImageList, err := LookupImageList(connConfig)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, err
	}

	for _, spiderImage := range spiderImageList.Image {
		tumblebugImage, err := ConvertSpiderImageToTumblebugImage(spiderImage)
		if err != nil {
			log.Error().Err(err).Msg("")
			return 0, err
		}

		tumblebugImageId := connConfig + "-" + ToNamingRuleCompatible(tumblebugImage.Name)

		check, err := CheckResource(nsId, model.StrImage, tumblebugImageId)
		if check {
			log.Info().Msgf("The image %s already exists in TB; continue", tumblebugImageId)
			continue
		} else if err != nil {
			log.Info().Msgf("Cannot check the existence of %s in TB; continue", tumblebugImageId)
			continue
		} else {
			tumblebugImage.Name = tumblebugImageId
			tumblebugImage.ConnectionName = connConfig

			_, err := RegisterImageWithInfo(nsId, &tumblebugImage, true)
			if err != nil {
				log.Error().Err(err).Msg("")
				return 0, err
			}
			imageCount++
		}
	}
	return imageCount, nil
}

// FetchImagesForAllConnConfigs gets all conn configs from Spider, lookups all images for each region of conn config, and saves into TB image objects
func FetchImagesForAllConnConfigs(nsId string) (connConfigCount uint, imageCount uint, err error) {

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, 0, err
	}

	connConfigs, err := common.GetConnConfigList(model.DefaultCredentialHolder, true, true)
	if err != nil {
		log.Error().Err(err).Msg("")
		return 0, 0, err
	}

	for _, connConfig := range connConfigs.Connectionconfig {
		temp, _ := FetchImagesForConnConfig(connConfig.ConfigName, nsId)
		imageCount += temp
		connConfigCount++
	}
	return connConfigCount, imageCount, nil
}

// SearchImage accepts arbitrary number of keywords, and returns the list of matched TB image objects
func SearchImage(nsId string, keywords ...string) ([]model.TbImageInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	tempList := []model.TbImageInfo{}

	//sqlQuery := "SELECT * FROM `image` WHERE `namespace`='" + nsId + "'"
	sqlQuery := model.ORM.Where("Namespace = ?", nsId)

	for _, keyword := range keywords {
		keyword = ToNamingRuleCompatible(keyword)
		//sqlQuery += " AND `name` LIKE '%" + keyword + "%'"
		sqlQuery = sqlQuery.And("Name LIKE ?", "%"+keyword+"%")
	}

	err = sqlQuery.Find(&tempList)
	if err != nil {
		log.Error().Err(err).Msg("")
		return tempList, err
	}
	return tempList, nil
}

// UpdateImage accepts to-be TB image objects,
// updates and returns the updated TB image objects
func UpdateImage(nsId string, imageId string, fieldsToUpdate model.TbImageInfo, RDBonly bool) (model.TbImageInfo, error) {
	if !RDBonly {

		resourceType := model.StrImage
		temp := model.TbImageInfo{}
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}

		if len(fieldsToUpdate.Namespace) > 0 {
			err := fmt.Errorf("You should not specify 'namespace' in the JSON request body.")
			log.Error().Err(err).Msg("")
			return temp, err
		}

		if len(fieldsToUpdate.Id) > 0 {
			err := fmt.Errorf("You should not specify 'id' in the JSON request body.")
			log.Error().Err(err).Msg("")
			return temp, err
		}

		check, err := CheckResource(nsId, resourceType, imageId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}

		if !check {
			err := fmt.Errorf("The image " + imageId + " does not exist.")
			return temp, err
		}

		tempInterface, err := GetResource(nsId, resourceType, imageId)
		if err != nil {
			err := fmt.Errorf("Failed to get the image " + imageId + ".")
			return temp, err
		}
		asIsImage := model.TbImageInfo{}
		err = common.CopySrcToDest(&tempInterface, &asIsImage)
		if err != nil {
			err := fmt.Errorf("Failed to CopySrcToDest() " + imageId + ".")
			return temp, err
		}

		// Update specified fields only
		toBeImage := asIsImage
		toBeImageJSON, _ := json.Marshal(fieldsToUpdate)
		err = json.Unmarshal(toBeImageJSON, &toBeImage)

		Key := common.GenResourceKey(nsId, resourceType, toBeImage.Id)
		Val, _ := json.Marshal(toBeImage)
		err = kvstore.Put(Key, string(Val))
		if err != nil {
			log.Error().Err(err).Msg("")
			return temp, err
		}

	}
	// "UPDATE `image` SET `id`='" + imageId + "', ... WHERE `namespace`='" + nsId + "' AND `id`='" + imageId + "';"
	_, err := model.ORM.Update(&fieldsToUpdate, &model.TbSpecInfo{Namespace: nsId, Id: imageId})
	if err != nil {
		log.Error().Err(err).Msg("")
		return fieldsToUpdate, err
	} else {
		log.Trace().Msg("SQL: Update success")
	}

	return fieldsToUpdate, nil
}

// GetImage accepts namespace Id and imageKey(Id,CspImageId,GuestOS,...), and returns the TB image object
func GetImage(nsId string, imageKey string) (model.TbImageInfo, error) {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("Invalid namespace ID")
		return model.TbImageInfo{}, err
	}

	log.Debug().Msg("[Get image] " + imageKey)

	// make comparison case-insensitive
	nsId = strings.ToLower(nsId)
	imageKey = strings.ToLower(imageKey)

	// ex: tencent+ap-jakarta+ubuntu22.04
	image := model.TbImageInfo{Namespace: nsId, Id: imageKey}
	has, err := model.ORM.Where("LOWER(Namespace) = ? AND LOWER(Id) = ?", nsId, imageKey).Get(&image)
	if err != nil {
		log.Info().Err(err).Msgf("Failed to get image %s by ID", imageKey)
	}
	if has {
		return image, nil
	}

	// ex: img-487zeit5
	image = model.TbImageInfo{Namespace: nsId, CspImageId: imageKey}
	has, err = model.ORM.Where("LOWER(Namespace) = ? AND LOWER(CspImageId) = ?", nsId, imageKey).Get(&image)
	if err != nil {
		log.Info().Err(err).Msgf("Failed to get image %s by CspImageId", imageKey)
	}
	if has {
		return image, nil
	}

	// ex: Ubuntu22.04
	image = model.TbImageInfo{Namespace: nsId, GuestOS: imageKey}
	has, err = model.ORM.Where("LOWER(Namespace) = ? AND LOWER(GuestOS) LIKE ?", nsId, imageKey).Get(&image)
	if err != nil {
		log.Info().Err(err).Msgf("Failed to get image %s by GuestOS type", imageKey)
	}
	if has {
		return image, nil
	}

	return model.TbImageInfo{}, fmt.Errorf("The imageKey %s not found by any of ID, CspImageId, GuestOS", imageKey)
}
