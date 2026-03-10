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

// Package infra is to manage multi-cloud infra
package infra

import (
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// CreateMciDynamicFromTemplate creates an MCI from a template with overrides
func CreateMciDynamicFromTemplate(reqID string, nsId string, templateId string, applyReq *model.TemplateApplyReq, option string) (*model.MciInfo, error) {

	// Get the template
	templateInfo, err := common.GetMciDynamicTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get template '%s'", templateId)
		return nil, fmt.Errorf("failed to get template '%s': %w", templateId, err)
	}

	// Create a copy of MciDynamicReq from the template
	mciReq := templateInfo.MciDynamicReq

	// Apply overrides (Phase 1: name and description only)
	mciReq.Name = applyReq.Name
	if applyReq.Description != "" {
		mciReq.Description = applyReq.Description
	}

	// Call the existing CreateMciDynamic function
	result, err := CreateMciDynamic(reqID, nsId, &mciReq, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create MCI from template")
		return nil, err
	}

	return result, nil
}

// ExtractAndCreateTemplate extracts an MCI configuration and creates a template from it
func ExtractAndCreateTemplate(nsId string, mciId string, templateName string) (model.MciDynamicTemplateInfo, error) {
	emptyResult := model.MciDynamicTemplateInfo{}

	// Extract MCI configuration
	mciDynamicReq, err := ExtractMciDynamicReqFromMciInfo(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to extract MCI config from '%s'", mciId)
		return emptyResult, fmt.Errorf("failed to extract MCI config: %w", err)
	}

	// Source info
	source := fmt.Sprintf("mci:%s/%s", nsId, mciId)
	description := fmt.Sprintf("Template extracted from MCI '%s' in namespace '%s'", mciId, nsId)

	// Create the template using the common package
	result, err := common.CreateMciDynamicTemplateWithReq(nsId, templateName, description, source, mciDynamicReq)
	if err != nil {
		log.Error().Err(err).Msg("failed to create template from MCI")
		return emptyResult, err
	}

	return result, nil
}
