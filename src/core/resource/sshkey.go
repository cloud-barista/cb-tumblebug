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

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// TbSshKeyReqStructLevelValidation is a function to validate 'TbSshKeyReq' object.
func TbSshKeyReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbSshKeyReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// CreateSshKey accepts SSH key creation request, creates and returns an TB sshKey object
func CreateSshKey(nsId string, u *model.TbSshKeyReq, option string) (model.TbSshKeyInfo, error) {

	emptyObj := model.TbSshKeyInfo{}

	resourceType := model.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	uuid := common.GenUid()

	if option == "register" { // fields validation
		errs := []error{}
		// errs = append(errs, validate.Var(u.Username, "required"))
		// errs = append(errs, validate.Var(u.PrivateKey, "required"))

		for _, err := range errs {
			if err != nil {
				if _, ok := err.(*validator.InvalidValidationError); ok {
					log.Err(err).Msg("")
					return emptyObj, err
				}
				return emptyObj, err
			}
		}
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		err := fmt.Errorf("The sshKey %s already exists.", u.Name)
		return emptyObj, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the sshKey %s.", u.Name)
		return emptyObj, err
	}

	requestBody := model.SpiderKeyPairReqInfoWrapper{}
	requestBody.ConnectionName = u.ConnectionName
	requestBody.ReqInfo.Name = uuid
	requestBody.ReqInfo.CSPId = u.CspSshKeyId

	var tempSpiderKeyPairInfo *model.SpiderKeyPairInfo

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderKeyPairInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response

	var url string
	if option == "register" && u.CspSshKeyId == "" {
		url = fmt.Sprintf("%s/keypair/%s", model.SpiderRestUrl, u.Name)
		resp, err = req.Get(url)
	} else if option == "register" && u.CspSshKeyId != "" {
		url = fmt.Sprintf("%s/regkeypair", model.SpiderRestUrl)
		resp, err = req.Post(url)
	} else { // option != "register"
		url = fmt.Sprintf("%s/keypair", model.SpiderRestUrl)
		resp, err = req.Post(url)
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return emptyObj, err
	}

	fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		fmt.Println("body: ", string(resp.Body()))
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	tempSpiderKeyPairInfo = resp.Result().(*model.SpiderKeyPairInfo)

	content := model.TbSshKeyInfo{}
	//content.Id = common.GenUid()
	content.Id = u.Name
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.Uuid = uuid
	content.CspSshKeyId = tempSpiderKeyPairInfo.IId.SystemId
	content.CspSshKeyName = tempSpiderKeyPairInfo.IId.NameId
	content.Fingerprint = tempSpiderKeyPairInfo.Fingerprint
	content.Username = tempSpiderKeyPairInfo.VMUserID
	content.PublicKey = tempSpiderKeyPairInfo.PublicKey
	content.PrivateKey = tempSpiderKeyPairInfo.PrivateKey
	content.Description = u.Description
	content.KeyValueList = tempSpiderKeyPairInfo.KeyValueList
	content.AssociatedObjectList = []string{}

	if option == "register" {
		if u.CspSshKeyId == "" {
			content.SystemLabel = "Registered from CB-Spider resource"
		} else if u.CspSshKeyId != "" {
			content.SystemLabel = "Registered from CSP resource"
		}

		// Rewrite fields again
		// content.Fingerprint = u.Fingerprint
		content.Username = u.Username
		content.PublicKey = u.PublicKey
		content.PrivateKey = u.PrivateKey
	}

	log.Info().Msg("PUT CreateSshKey")
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrSSHKey, uuid, Key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	return content, nil
}

// UpdateSshKey accepts to-be TB sshKey objects,
// updates and returns the updated TB sshKey objects
func UpdateSshKey(nsId string, sshKeyId string, fieldsToUpdate model.TbSshKeyInfo) (model.TbSshKeyInfo, error) {

	emptyObj := model.TbSshKeyInfo{}

	resourceType := model.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if len(fieldsToUpdate.Id) > 0 {
		err := fmt.Errorf("You should not specify 'id' in the JSON request body.")
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	check, err := CheckResource(nsId, resourceType, sshKeyId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The sshKey %s does not exist.", sshKeyId)
		return emptyObj, err
	}

	tempInterface, err := GetResource(nsId, resourceType, sshKeyId)
	if err != nil {
		err := fmt.Errorf("Failed to get the sshKey %s.", sshKeyId)
		return emptyObj, err
	}
	asIsSshKey := model.TbSshKeyInfo{}
	err = common.CopySrcToDest(&tempInterface, &asIsSshKey)
	if err != nil {
		err := fmt.Errorf("Failed to CopySrcToDest() %s.", sshKeyId)
		return emptyObj, err
	}

	// Update specified fields only
	toBeSshKey := asIsSshKey
	toBeSshKeyJSON, _ := json.Marshal(fieldsToUpdate)
	err = json.Unmarshal(toBeSshKeyJSON, &toBeSshKey)

	log.Info().Msg("PUT UpdateSshKey")
	Key := common.GenResourceKey(nsId, resourceType, toBeSshKey.Id)
	Val, _ := json.Marshal(toBeSshKey)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	keyValue, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In UpdateSshKey(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	fmt.Printf("<%s> \n %s \n", keyValue.Key, keyValue.Value)

	return toBeSshKey, nil
}
