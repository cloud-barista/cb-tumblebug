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

// CreatePlaceholderSshKey creates a placeholder SSH key for CSPs that do not manage
// SSH keys as independent resources (e.g., GCP). It delegates actual creation to
// CreateSshKey (Spider abstracts SSH key API even for GCP), then modifies the
// returned object to mark it as a placeholder. The user can later update this
// SSH key via ComplementSshKey API to set username and privateKey.
func CreatePlaceholderSshKey(nsId string, connectionName string, vmName string, vmUid string) (model.SshKeyInfo, error) {
	emptyObj := model.SshKeyInfo{}
	resourceType := model.StrSSHKey

	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	placeholderName := common.ChangeIdString(fmt.Sprintf("%s-ssh-placeholder-%s", connectionName, vmUid))

	// Check if the placeholder SSH key already exists
	check, err := CheckResource(nsId, resourceType, placeholderName)
	if err != nil {
		return emptyObj, fmt.Errorf("failed to check existence of placeholder SSH key: %w", err)
	}
	if check {
		// Already exists, return the existing one
		existingRes, err := GetResource(nsId, resourceType, placeholderName)
		if err != nil {
			return emptyObj, fmt.Errorf("failed to get existing placeholder SSH key: %w", err)
		}
		existingKey := model.SshKeyInfo{}
		if err := common.CopySrcToDest(&existingRes, &existingKey); err != nil {
			return emptyObj, fmt.Errorf("failed to convert existing placeholder SSH key: %w", err)
		}
		log.Info().Msgf("Placeholder SSH key '%s' already exists, reusing it", placeholderName)
		return existingKey, nil
	}

	// Create a real SSH key through Spider (Spider abstracts SSH key API even for GCP)
	req := model.SshKeyReq{
		Name:           placeholderName,
		ConnectionName: connectionName,
		Description:    fmt.Sprintf("Auto-generated placeholder for GCP VM '%s'. Update via ComplementSshKey API.", vmName),
	}
	content, err := CreateSshKey(nsId, &req, "")
	if err != nil {
		// Handle race condition: another goroutine may have created the same placeholder concurrently
		if strings.Contains(err.Error(), "already exists") {
			log.Warn().Msgf("Placeholder SSH key '%s' was created concurrently; fetching existing resource", placeholderName)
			existingRes, getErr := GetResource(nsId, resourceType, placeholderName)
			if getErr != nil {
				return emptyObj, fmt.Errorf("placeholder SSH key '%s' already exists but failed to retrieve: %w", placeholderName, getErr)
			}
			existingKey := model.SshKeyInfo{}
			if copyErr := common.CopySrcToDest(&existingRes, &existingKey); copyErr != nil {
				return emptyObj, fmt.Errorf("failed to convert existing placeholder SSH key after concurrent creation: %w", copyErr)
			}
			return existingKey, nil
		}
		return emptyObj, fmt.Errorf("failed to create placeholder SSH key through Spider: %w", err)
	}

	// Mark the created SSH key as a placeholder
	content.IsAutoGenerated = true
	content.SystemLabel = "Placeholder SSH key (GCP). Use ComplementSshKey API to set username and privateKey."

	// Save the modified info back to kvstore
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	if err := kvstore.Put(Key, string(Val)); err != nil {
		log.Error().Err(err).Msgf("Failed to update placeholder SSH key '%s'", placeholderName)
		return emptyObj, err
	}

	// Add placeholder-specific labels (merged with existing labels created by CreateSshKey)
	placeholderLabels := map[string]string{
		model.LabelPlaceholder:        "true",
		model.LabelRequiresComplement: "true",
	}
	if err := label.CreateOrUpdateLabel(model.StrSSHKey, content.Uid, Key, placeholderLabels); err != nil {
		log.Warn().Err(err).Msgf("Failed to add placeholder labels for SSH key '%s'", placeholderName)
	}

	log.Info().Msgf("Created placeholder SSH key '%s' for GCP VM '%s'", placeholderName, vmName)
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

// ComplementSshKey enable remote command execution for registered SSH keys
// by updating username and privateKey required for SSH authentication
func ComplementSshKey(nsId string, sshKeyId string, req model.SshKeyComplementReq) (model.SshKeyInfo, error) {
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

	log.Info().Msg("PUT ComplementSshKey")
	Key := common.GenResourceKey(nsId, resourceType, sshKey.Id)
	Val, _ := json.Marshal(sshKey)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	return sshKey, nil
}
