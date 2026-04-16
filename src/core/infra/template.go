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
	"context"
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// CreateInfraDynamicFromTemplate creates an Infra from a template with overrides
func CreateInfraDynamicFromTemplate(ctx context.Context, nsId string, templateId string, applyReq *model.TemplateApplyReq, option string) (*model.InfraInfo, error) {

	// Get the template
	templateInfo, err := common.GetInfraDynamicTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get template '%s'", templateId)
		return nil, fmt.Errorf("failed to get template '%s': %w", templateId, err)
	}

	// Create a copy of InfraDynamicReq from the template
	infraReq := templateInfo.InfraDynamicReq

	// Apply overrides (Phase 1: name and description only)
	infraReq.Name = applyReq.Name
	if applyReq.Description != "" {
		infraReq.Description = applyReq.Description
	}

	// Call the existing CreateInfraDynamic function
	result, err := CreateInfraDynamic(ctx, nsId, &infraReq, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create Infra from template")
		return nil, err
	}

	return result, nil
}

// ExtractAndCreateTemplate extracts an Infra configuration and creates a template from it
func ExtractAndCreateTemplate(nsId string, infraId string, templateName string) (model.InfraDynamicTemplateInfo, error) {
	emptyResult := model.InfraDynamicTemplateInfo{}

	// Extract Infra configuration
	infraDynamicReq, err := ExtractInfraDynamicReqFromInfraInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to extract Infra config from '%s'", infraId)
		return emptyResult, fmt.Errorf("failed to extract Infra config: %w", err)
	}

	// Source info
	source := fmt.Sprintf("infra:%s/%s", nsId, infraId)
	description := fmt.Sprintf("Template extracted from Infra '%s' in namespace '%s'", infraId, nsId)

	// Create the template using the common package
	result, err := common.CreateInfraDynamicTemplateWithReq(nsId, templateName, description, source, infraDynamicReq)
	if err != nil {
		log.Error().Err(err).Msg("failed to create template from Infra")
		return emptyResult, err
	}

	return result, nil
}
