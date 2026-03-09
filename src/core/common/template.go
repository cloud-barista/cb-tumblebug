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
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	"github.com/rs/zerolog/log"
)

// CreateMciDynamicTemplate creates a new MCI Dynamic Template
func CreateMciDynamicTemplate(nsId string, req *model.MciDynamicTemplateReq) (model.MciDynamicTemplateInfo, error) {
	emptyResult := model.MciDynamicTemplateInfo{}

	err := CheckString(req.Name)
	if err != nil {
		log.Error().Err(err).Msg("invalid template name")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if template already exists
	key := GenTemplateKey(nsId, "mci", req.Name)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if exists {
		return emptyResult, fmt.Errorf("template '%s' already exists in namespace '%s'", req.Name, nsId)
	}

	now := time.Now().Format(time.RFC3339)
	templateInfo := model.MciDynamicTemplateInfo{
		ResourceType:  model.StrMCI,
		Id:            req.Name,
		Name:          req.Name,
		Description:   req.Description,
		Source:        "user",
		CreatedAt:     now,
		UpdatedAt:     now,
		MciDynamicReq: req.MciDynamicReq,
	}

	val, err := json.Marshal(templateInfo)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to store template in ETCD")
		return emptyResult, err
	}

	return templateInfo, nil
}

// GetMciDynamicTemplate retrieves an MCI Dynamic Template by ID
func GetMciDynamicTemplate(nsId string, templateId string) (model.MciDynamicTemplateInfo, error) {
	emptyResult := model.MciDynamicTemplateInfo{}

	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	key := GenTemplateKey(nsId, "mci", templateId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if !exists {
		return emptyResult, fmt.Errorf("template '%s' not found in namespace '%s'", templateId, nsId)
	}

	result := model.MciDynamicTemplateInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal template info")
		return emptyResult, err
	}

	return result, nil
}

// ListMciDynamicTemplate lists all MCI Dynamic Templates in a namespace
// filterKeyword is optional; if non-empty, only templates whose Name or Description
// contains the keyword (case-insensitive) are returned.
func ListMciDynamicTemplate(nsId string, filterKeyword string) ([]model.MciDynamicTemplateInfo, error) {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return nil, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	key := GenTemplateKey(nsId, "mci", "")
	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var templates []model.MciDynamicTemplateInfo
	keyword := strings.ToLower(strings.TrimSpace(filterKeyword))
	for _, v := range keyValue {
		tempObj := model.MciDynamicTemplateInfo{}
		err = json.Unmarshal([]byte(v.Value), &tempObj)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal template")
			continue
		}
		if keyword != "" {
			nameLower := strings.ToLower(tempObj.Name)
			descLower := strings.ToLower(tempObj.Description)
			if !strings.Contains(nameLower, keyword) && !strings.Contains(descLower, keyword) {
				continue
			}
		}
		templates = append(templates, tempObj)
	}

	return templates, nil
}

// UpdateMciDynamicTemplate updates an existing MCI Dynamic Template
func UpdateMciDynamicTemplate(nsId string, templateId string, req *model.MciDynamicTemplateReq) (model.MciDynamicTemplateInfo, error) {
	emptyResult := model.MciDynamicTemplateInfo{}

	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Get existing template
	existing, err := GetMciDynamicTemplate(nsId, templateId)
	if err != nil {
		return emptyResult, err
	}

	// Update fields
	now := time.Now().Format(time.RFC3339)
	existing.Name = req.Name
	existing.Description = req.Description
	existing.UpdatedAt = now
	existing.MciDynamicReq = req.MciDynamicReq

	key := GenTemplateKey(nsId, "mci", templateId)
	val, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to update template in ETCD")
		return emptyResult, err
	}

	return existing, nil
}

// DeleteMciDynamicTemplate deletes an MCI Dynamic Template
func DeleteMciDynamicTemplate(nsId string, templateId string) error {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if template exists
	key := GenTemplateKey(nsId, "mci", templateId)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if !exists {
		return fmt.Errorf("template '%s' not found in namespace '%s'", templateId, nsId)
	}

	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete template from ETCD")
		return err
	}

	return nil
}

// DeleteAllMciDynamicTemplate deletes all MCI Dynamic Templates in a namespace
func DeleteAllMciDynamicTemplate(nsId string) error {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	templates, err := ListMciDynamicTemplate(nsId, "")
	if err != nil {
		return err
	}

	for _, t := range templates {
		err := DeleteMciDynamicTemplate(nsId, t.Id)
		if err != nil {
			log.Error().Err(err).Msgf("failed to delete template '%s'", t.Id)
			return err
		}
	}

	return nil
}

// CreateMciDynamicTemplateFromMci creates a template from an existing MCI
func CreateMciDynamicTemplateFromMci(nsId string, mciId string, templateName string) (model.MciDynamicTemplateInfo, error) {
	emptyResult := model.MciDynamicTemplateInfo{}

	err := CheckString(templateName)
	if err != nil {
		log.Error().Err(err).Msg("invalid template name")
		return emptyResult, err
	}

	// Check if template already exists
	key := GenTemplateKey(nsId, "mci", templateName)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if exists {
		return emptyResult, fmt.Errorf("template '%s' already exists in namespace '%s'", templateName, nsId)
	}

	// This function is called from infra package, so we need to provide the extraction logic
	// The caller should pass the extracted MciDynamicReq
	return emptyResult, fmt.Errorf("use CreateMciDynamicTemplateWithReq instead")
}

// CreateMciDynamicTemplateWithReq creates a template from an MciDynamicReq (used for extraction from existing MCI)
func CreateMciDynamicTemplateWithReq(nsId string, templateName string, description string, source string, mciDynamicReq *model.MciDynamicReq) (model.MciDynamicTemplateInfo, error) {
	emptyResult := model.MciDynamicTemplateInfo{}

	err := CheckString(templateName)
	if err != nil {
		log.Error().Err(err).Msg("invalid template name")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if template already exists
	key := GenTemplateKey(nsId, "mci", templateName)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if exists {
		return emptyResult, fmt.Errorf("template '%s' already exists in namespace '%s'", templateName, nsId)
	}

	now := time.Now().Format(time.RFC3339)
	templateInfo := model.MciDynamicTemplateInfo{
		ResourceType:  model.StrMCI,
		Id:            templateName,
		Name:          templateName,
		Description:   description,
		Source:        source,
		CreatedAt:     now,
		UpdatedAt:     now,
		MciDynamicReq: *mciDynamicReq,
	}

	val, err := json.Marshal(templateInfo)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to store template in ETCD")
		return emptyResult, err
	}

	return templateInfo, nil
}

// =====================================================================
// vNet Template CRUD Functions
// =====================================================================

// CreateVNetTemplate creates a new vNet Template
func CreateVNetTemplate(nsId string, req *model.VNetTemplateReq) (model.VNetTemplateInfo, error) {
	emptyResult := model.VNetTemplateInfo{}

	err := CheckString(req.Name)
	if err != nil {
		log.Error().Err(err).Msg("invalid template name")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if template already exists
	key := GenTemplateKey(nsId, "vNet", req.Name)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if exists {
		return emptyResult, fmt.Errorf("vNet template '%s' already exists in namespace '%s'", req.Name, nsId)
	}

	now := time.Now().Format(time.RFC3339)
	templateInfo := model.VNetTemplateInfo{
		ResourceType: model.StrVNet,
		Id:           req.Name,
		Name:         req.Name,
		Description:  req.Description,
		Source:       "user",
		CreatedAt:    now,
		UpdatedAt:    now,
		VNetReq:      req.VNetReq,
	}

	val, err := json.Marshal(templateInfo)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal vNet template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to store vNet template in ETCD")
		return emptyResult, err
	}

	return templateInfo, nil
}

// GetVNetTemplate retrieves a vNet Template by ID
func GetVNetTemplate(nsId string, templateId string) (model.VNetTemplateInfo, error) {
	emptyResult := model.VNetTemplateInfo{}

	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	key := GenTemplateKey(nsId, "vNet", templateId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if !exists {
		return emptyResult, fmt.Errorf("vNet template '%s' not found in namespace '%s'", templateId, nsId)
	}

	result := model.VNetTemplateInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal vNet template info")
		return emptyResult, err
	}

	return result, nil
}

// ListVNetTemplate lists all vNet Templates in a namespace
// filterKeyword is optional; if non-empty, only templates whose Name or Description
// contains the keyword (case-insensitive) are returned.
func ListVNetTemplate(nsId string, filterKeyword string) ([]model.VNetTemplateInfo, error) {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return nil, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	key := GenTemplateKey(nsId, "vNet", "")
	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var templates []model.VNetTemplateInfo
	keyword := strings.ToLower(strings.TrimSpace(filterKeyword))
	for _, v := range keyValue {
		tempObj := model.VNetTemplateInfo{}
		err = json.Unmarshal([]byte(v.Value), &tempObj)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal vNet template")
			continue
		}
		if keyword != "" {
			nameLower := strings.ToLower(tempObj.Name)
			descLower := strings.ToLower(tempObj.Description)
			if !strings.Contains(nameLower, keyword) && !strings.Contains(descLower, keyword) {
				continue
			}
		}
		templates = append(templates, tempObj)
	}

	return templates, nil
}

// UpdateVNetTemplate updates an existing vNet Template
func UpdateVNetTemplate(nsId string, templateId string, req *model.VNetTemplateReq) (model.VNetTemplateInfo, error) {
	emptyResult := model.VNetTemplateInfo{}

	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Get existing template
	existing, err := GetVNetTemplate(nsId, templateId)
	if err != nil {
		return emptyResult, err
	}

	// Update fields
	now := time.Now().Format(time.RFC3339)
	existing.Name = req.Name
	existing.Description = req.Description
	existing.UpdatedAt = now
	existing.VNetReq = req.VNetReq

	key := GenTemplateKey(nsId, "vNet", templateId)
	val, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal vNet template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to update vNet template in ETCD")
		return emptyResult, err
	}

	return existing, nil
}

// DeleteVNetTemplate deletes a vNet Template
func DeleteVNetTemplate(nsId string, templateId string) error {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if template exists
	key := GenTemplateKey(nsId, "vNet", templateId)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if !exists {
		return fmt.Errorf("vNet template '%s' not found in namespace '%s'", templateId, nsId)
	}

	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete vNet template from ETCD")
		return err
	}

	return nil
}

// DeleteAllVNetTemplate deletes all vNet Templates in a namespace
func DeleteAllVNetTemplate(nsId string) error {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	templates, err := ListVNetTemplate(nsId, "")
	if err != nil {
		return err
	}

	for _, t := range templates {
		err := DeleteVNetTemplate(nsId, t.Id)
		if err != nil {
			log.Error().Err(err).Msgf("failed to delete vNet template '%s'", t.Id)
			return err
		}
	}

	return nil
}

// =====================================================================
// SecurityGroup Template CRUD Functions
// =====================================================================

// CreateSecurityGroupTemplate creates a new SecurityGroup Template
func CreateSecurityGroupTemplate(nsId string, req *model.SecurityGroupTemplateReq) (model.SecurityGroupTemplateInfo, error) {
	emptyResult := model.SecurityGroupTemplateInfo{}

	err := CheckString(req.Name)
	if err != nil {
		log.Error().Err(err).Msg("invalid template name")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if template already exists
	key := GenTemplateKey(nsId, "securityGroup", req.Name)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if exists {
		return emptyResult, fmt.Errorf("securityGroup template '%s' already exists in namespace '%s'", req.Name, nsId)
	}

	now := time.Now().Format(time.RFC3339)
	templateInfo := model.SecurityGroupTemplateInfo{
		ResourceType:     model.StrSecurityGroup,
		Id:               req.Name,
		Name:             req.Name,
		Description:      req.Description,
		Source:           "user",
		CreatedAt:        now,
		UpdatedAt:        now,
		SecurityGroupReq: req.SecurityGroupReq,
	}

	val, err := json.Marshal(templateInfo)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal securityGroup template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to store securityGroup template in ETCD")
		return emptyResult, err
	}

	return templateInfo, nil
}

// GetSecurityGroupTemplate retrieves a SecurityGroup Template by ID
func GetSecurityGroupTemplate(nsId string, templateId string) (model.SecurityGroupTemplateInfo, error) {
	emptyResult := model.SecurityGroupTemplateInfo{}

	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return emptyResult, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	key := GenTemplateKey(nsId, "securityGroup", templateId)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}
	if !exists {
		return emptyResult, fmt.Errorf("securityGroup template '%s' not found in namespace '%s'", templateId, nsId)
	}

	result := model.SecurityGroupTemplateInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal securityGroup template info")
		return emptyResult, err
	}

	return result, nil
}

// ListSecurityGroupTemplate lists all SecurityGroup Templates in a namespace
// filterKeyword is optional; if non-empty, only templates whose Name or Description
// contains the keyword (case-insensitive) are returned.
func ListSecurityGroupTemplate(nsId string, filterKeyword string) ([]model.SecurityGroupTemplateInfo, error) {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	// Check if namespace exists
	check, err := CheckNs(nsId)
	if !check {
		return nil, fmt.Errorf("namespace '%s' does not exist", nsId)
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	key := GenTemplateKey(nsId, "securityGroup", "")
	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	var templates []model.SecurityGroupTemplateInfo
	keyword := strings.ToLower(strings.TrimSpace(filterKeyword))
	for _, v := range keyValue {
		tempObj := model.SecurityGroupTemplateInfo{}
		err = json.Unmarshal([]byte(v.Value), &tempObj)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal securityGroup template")
			continue
		}
		if keyword != "" {
			nameLower := strings.ToLower(tempObj.Name)
			descLower := strings.ToLower(tempObj.Description)
			if !strings.Contains(nameLower, keyword) && !strings.Contains(descLower, keyword) {
				continue
			}
		}
		templates = append(templates, tempObj)
	}

	return templates, nil
}

// UpdateSecurityGroupTemplate updates an existing SecurityGroup Template
func UpdateSecurityGroupTemplate(nsId string, templateId string, req *model.SecurityGroupTemplateReq) (model.SecurityGroupTemplateInfo, error) {
	emptyResult := model.SecurityGroupTemplateInfo{}

	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResult, err
	}

	// Get existing template
	existing, err := GetSecurityGroupTemplate(nsId, templateId)
	if err != nil {
		return emptyResult, err
	}

	// Update fields
	now := time.Now().Format(time.RFC3339)
	existing.Name = req.Name
	existing.Description = req.Description
	existing.UpdatedAt = now
	existing.SecurityGroupReq = req.SecurityGroupReq

	key := GenTemplateKey(nsId, "securityGroup", templateId)
	val, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal securityGroup template info")
		return emptyResult, err
	}

	err = kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("failed to update securityGroup template in ETCD")
		return emptyResult, err
	}

	return existing, nil
}

// DeleteSecurityGroupTemplate deletes a SecurityGroup Template
func DeleteSecurityGroupTemplate(nsId string, templateId string) error {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = CheckString(templateId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if template exists
	key := GenTemplateKey(nsId, "securityGroup", templateId)
	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if !exists {
		return fmt.Errorf("securityGroup template '%s' not found in namespace '%s'", templateId, nsId)
	}

	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete securityGroup template from ETCD")
		return err
	}

	return nil
}

// DeleteAllSecurityGroupTemplate deletes all SecurityGroup Templates in a namespace
func DeleteAllSecurityGroupTemplate(nsId string) error {
	err := CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	templates, err := ListSecurityGroupTemplate(nsId, "")
	if err != nil {
		return err
	}

	for _, t := range templates {
		err := DeleteSecurityGroupTemplate(nsId, t.Id)
		if err != nil {
			log.Error().Err(err).Msgf("failed to delete securityGroup template '%s'", t.Id)
			return err
		}
	}

	return nil
}
