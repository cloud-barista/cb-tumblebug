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

// Package infra is to handle REST API for infra
package infra

import (
	"net/http"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostMciDynamicTemplate godoc
// @ID PostMciDynamicTemplate
// @Summary Create an MCI Dynamic Template
// @Description Create a reusable MCI Dynamic Template. Templates store MCI dynamic creation
// @Description request configurations that can be applied later to create MCIs with consistent settings.
// @Description
// @Description **Template Contents:**
// @Description - VM specifications (specId, imageId) for each subgroup
// @Description - Subgroup sizing and naming
// @Description - Network and disk configuration
// @Description - Post-deployment commands
// @Description - Monitoring agent options
// @Description
// @Description Templates can be created manually or extracted from existing MCIs.
// @Tags [MC-Infra] MCI Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateReq body model.MciDynamicTemplateReq true "MCI Dynamic Template request"
// @Success 200 {object} model.MciDynamicTemplateInfo "Successfully created template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or template name"
// @Failure 409 {object} model.SimpleMsg "Template already exists"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/mci [post]
func RestPostMciDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	req := &model.MciDynamicTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.CreateMciDynamicTemplate(nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to create MCI dynamic template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetMciDynamicTemplate godoc
// @ID GetMciDynamicTemplate
// @Summary Get an MCI Dynamic Template
// @Description Retrieve a specific MCI Dynamic Template by ID.
// @Tags [MC-Infra] MCI Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.MciDynamicTemplateInfo "Template information"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/mci/{templateId} [get]
func RestGetMciDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	result, err := common.GetMciDynamicTemplate(nsId, templateId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetAllMciDynamicTemplate godoc
// @ID GetAllMciDynamicTemplate
// @Summary List all MCI Dynamic Templates
// @Description List all MCI Dynamic Templates in a namespace.
// @Description Optionally filter by keyword matching against template name or description (case-insensitive).
// @Tags [MC-Infra] MCI Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param filterKeyword query string false "Keyword to filter templates by name or description"
// @Success 200 {object} model.MciDynamicTemplateListResponse "List of templates"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/mci [get]
func RestGetAllMciDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	filterKeyword := c.QueryParam("filterKeyword")

	result, err := common.ListMciDynamicTemplate(nsId, filterKeyword)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	response := model.MciDynamicTemplateListResponse{Templates: result}
	return clientManager.EndRequestWithLog(c, nil, response)
}

// RestPutMciDynamicTemplate godoc
// @ID PutMciDynamicTemplate
// @Summary Update an MCI Dynamic Template
// @Description Update an existing MCI Dynamic Template.
// @Tags [MC-Infra] MCI Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Param templateReq body model.MciDynamicTemplateReq true "MCI Dynamic Template request"
// @Success 200 {object} model.MciDynamicTemplateInfo "Updated template information"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/mci/{templateId} [put]
func RestPutMciDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	req := &model.MciDynamicTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.UpdateMciDynamicTemplate(nsId, templateId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to update MCI dynamic template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteMciDynamicTemplate godoc
// @ID DeleteMciDynamicTemplate
// @Summary Delete an MCI Dynamic Template
// @Description Delete a specific MCI Dynamic Template.
// @Tags [MC-Infra] MCI Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.SimpleMsg "Template deleted successfully"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/mci/{templateId} [delete]
func RestDeleteMciDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	err := common.DeleteMciDynamicTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete MCI dynamic template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "Template '" + templateId + "' has been deleted"})
}

// RestDeleteAllMciDynamicTemplate godoc
// @ID DeleteAllMciDynamicTemplate
// @Summary Delete all MCI Dynamic Templates
// @Description Delete all MCI Dynamic Templates in a namespace.
// @Tags [MC-Infra] MCI Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg "All templates deleted successfully"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/mci [delete]
func RestDeleteAllMciDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	err := common.DeleteAllMciDynamicTemplate(nsId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete all MCI dynamic templates")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "All MCI dynamic templates have been deleted"})
}

// RestPostMciDynamicFromTemplate godoc
// @ID PostMciDynamicFromTemplate
// @Summary Create MCI from a Template
// @Description Create a new MCI by applying an MCI Dynamic Template.
// @Description The template provides the base VM configuration, and the apply request
// @Description allows overriding the MCI name and description.
// @Description
// @Description **Override Behavior (Phase 1):**
// @Description - `name` (required): Name for the new MCI
// @Description - `description` (optional): Overrides the template's description
// @Description - All other configuration (specs, images, subgroups) comes from the template
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID to apply"
// @Param applyReq body model.TemplateApplyReq true "Template apply request with MCI name and optional description"
// @Param option query string false "Deployment option: 'hold' to create MCI without immediate VM provisioning" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking"
// @Success 200 {object} model.MciInfo "Successfully created MCI from template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template or namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal deployment error"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/mci/template/{templateId} [post]
func RestPostMciDynamicFromTemplate(c echo.Context) error {
	ctx := c.Request().Context()
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")
	option := c.QueryParam("option")

	req := &model.TemplateApplyReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciDynamicFromTemplate(ctx, nsId, templateId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create MCI from template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}
