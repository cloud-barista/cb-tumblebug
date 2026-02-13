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
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// normalizePrivateKey normalizes private key format from various CSP sources.
// It handles Tencent Cloud's escaped newlines and ensures consistent format across providers.
func normalizePrivateKey(privateKey string, keyValueList []model.KeyValue) string {
	// If PrivateKey is already available and properly formatted, use it
	if privateKey != "" && strings.Contains(privateKey, "\n") {
		return privateKey
	}

	// Handle Tencent Cloud format with keyValueList containing escaped newlines
	if privateKey == "" && len(keyValueList) > 0 {
		for _, kv := range keyValueList {
			if kv.Key == "PrivateKey" {
				// Replace escaped newlines with actual newlines
				normalizedKey := strings.ReplaceAll(kv.Value, "\\n", "\n")
				log.Debug().
					Str("original", kv.Value[:min(50, len(kv.Value))]).
					Str("normalized", normalizedKey[:min(50, len(normalizedKey))]).
					Msg("Normalized private key from keyValueList")
				return normalizedKey
			}
		}
	}

	// Handle case where privateKey contains escaped newlines
	if strings.Contains(privateKey, "\\n") {
		normalizedKey := strings.ReplaceAll(privateKey, "\\n", "\n")
		log.Debug().
			Str("original", privateKey[:min(50, len(privateKey))]).
			Str("normalized", normalizedKey[:min(50, len(normalizedKey))]).
			Msg("Normalized private key from escaped format")
		return normalizedKey
	}

	return privateKey
}

// SshKeyReqStructLevelValidation is a function to validate 'SshKeyReq' object.
func SshKeyReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.SshKeyReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// CreateSshKey accepts SSH key creation request, creates and returns an TB sshKey object
func CreateSshKey(nsId string, u *model.SshKeyReq, option string) (model.SshKeyInfo, error) {

	emptyObj := model.SshKeyInfo{}

	resourceType := model.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	uid := common.GenUid()

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
	requestBody.ReqInfo.Name = uid
	requestBody.ReqInfo.CSPId = u.CspResourceId

	var tempSpiderKeyPairInfo model.SpiderKeyPairInfo

	client := clientManager.NewHttpClient()

	var url string
	var method string

	if option == "register" && u.CspResourceId == "" {
		// GET request with ConnectionName as query parameter
		url = fmt.Sprintf("%s/keypair/%s?ConnectionName=%s", model.SpiderRestUrl, u.Name, u.ConnectionName)
		method = "GET"

		requestBodyNoBody := clientManager.NoBody
		_, err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBodyNoBody),
			&requestBodyNoBody,
			&tempSpiderKeyPairInfo,
			clientManager.VeryShortDuration,
		)

	} else if option == "register" && u.CspResourceId != "" {
		url = fmt.Sprintf("%s/regkeypair", model.SpiderRestUrl)
		method = "POST"

		_, err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&tempSpiderKeyPairInfo,
			clientManager.VeryShortDuration,
		)

	} else { // option != "register"
		url = fmt.Sprintf("%s/keypair", model.SpiderRestUrl)
		method = "POST"

		_, err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&tempSpiderKeyPairInfo,
			clientManager.VeryShortDuration,
		)
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	content := model.SshKeyInfo{}
	content.ResourceType = resourceType
	content.Id = u.Name
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.Uid = uid
	content.CspResourceId = tempSpiderKeyPairInfo.IId.SystemId
	content.CspResourceName = tempSpiderKeyPairInfo.IId.NameId
	content.Fingerprint = tempSpiderKeyPairInfo.Fingerprint
	content.Username = tempSpiderKeyPairInfo.VMUserID
	content.PublicKey = tempSpiderKeyPairInfo.PublicKey

	// Normalize private key format from various CSP sources at storage time
	content.PrivateKey = normalizePrivateKey(tempSpiderKeyPairInfo.PrivateKey, tempSpiderKeyPairInfo.KeyValueList)

	content.Description = u.Description
	content.KeyValueList = tempSpiderKeyPairInfo.KeyValueList
	content.AssociatedObjectList = []string{}
	content.ConnectionConfig, err = common.GetConnConfig(content.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	if option == "register" {
		if u.CspResourceId == "" {
			content.SystemLabel = "Registered from CB-Spider resource"
		} else if u.CspResourceId != "" {
			content.SystemLabel = "Registered from CSP resource"
		}

		// Rewrite fields again
		// content.Fingerprint = u.Fingerprint
		content.Username = u.Username
		content.PublicKey = u.PublicKey
		// Normalize private key for register option as well
		content.PrivateKey = normalizePrivateKey(u.PrivateKey, content.KeyValueList)
	}

	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrSSHKey,
		model.LabelId:              content.Id,
		model.LabelName:            content.Name,
		model.LabelUid:             content.Uid,
		model.LabelCspResourceId:   content.CspResourceId,
		model.LabelCspResourceName: content.CspResourceName,
		model.LabelDescription:     content.Description,
		model.LabelConnectionName:  content.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrSSHKey, uid, Key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	return content, nil
}

// UpdateSshKey accepts to-be TB sshKey objects,
// updates and returns the updated TB sshKey objects
func UpdateSshKey(nsId string, sshKeyId string, update model.SshKeyUpdateReq) (model.SshKeyInfo, error) {

	emptyObj := model.SshKeyInfo{}

	resourceType := model.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
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
	asIsSshKey := model.SshKeyInfo{}
	err = common.CopySrcToDest(&tempInterface, &asIsSshKey)
	if err != nil {
		err := fmt.Errorf("Failed to CopySrcToDest() %s.", sshKeyId)
		return emptyObj, err
	}

	// Update specified fields only
	toBeSshKey := asIsSshKey
	updateBytes, err := json.Marshal(update)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal update fields")
		return emptyObj, err
	}
	err = json.Unmarshal(updateBytes, &toBeSshKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal update fields")
		return emptyObj, err
	}

	log.Info().Msg("PUT UpdateSshKey")
	Key := common.GenResourceKey(nsId, resourceType, toBeSshKey.Id)
	Val, _ := json.Marshal(toBeSshKey)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	keyValue, _, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In UpdateSshKey(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	fmt.Printf("<%s> \n %s \n", keyValue.Key, keyValue.Value)

	return toBeSshKey, nil
}

// ActivateSshKey activates remote command execution for registered SSH keys
// by updating username and privateKey required for SSH authentication
func ActivateSshKey(nsId string, sshKeyId string, req model.SshKeyActivateReq) (model.SshKeyInfo, error) {
	emptyObj := model.SshKeyInfo{}
	resourceType := model.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
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

	sshKey := model.SshKeyInfo{}
	err = common.CopySrcToDest(&tempInterface, &sshKey)
	if err != nil {
		err := fmt.Errorf("Failed to CopySrcToDest() %s.", sshKeyId)
		return emptyObj, err
	}

	// Update username and privateKey for remote command
	sshKey.Username = req.Username
	sshKey.PrivateKey = req.PrivateKey

	log.Info().Msg("PUT ActivateSshKey")
	Key := common.GenResourceKey(nsId, resourceType, sshKey.Id)
	Val, _ := json.Marshal(sshKey)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	return sshKey, nil
}
