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

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostInfraDynamicTemplate godoc
// @ID PostInfraDynamicTemplate
// @Summary Create an Infra Dynamic Template
// @Description Create a reusable Infra Dynamic Template. Templates store Infra dynamic creation
// @Description request configurations that can be applied later to create Infras with consistent settings.
// @Description
// @Description **Template Contents:**
// @Description - node specifications (specId, imageId) for each nodegroup
// @Description - NodeGroup sizing and naming
// @Description - Network and disk configuration
// @Description - Post-deployment commands
// @Description - Monitoring agent options
// @Description
// @Description Templates can be created manually or extracted from existing Infras.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateReq body model.InfraDynamicTemplateReq true "Infra Dynamic Template request"
// @Success 200 {object} model.InfraDynamicTemplateInfo "Successfully created template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or template name"
// @Failure 409 {object} model.SimpleMsg "Template already exists"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/infra [post]
func RestPostInfraDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	req := &model.InfraDynamicTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.CreateInfraDynamicTemplate(nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to create Infra dynamic template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetInfraDynamicTemplate godoc
// @ID GetInfraDynamicTemplate
// @Summary Get an Infra Dynamic Template
// @Description Retrieve a specific Infra Dynamic Template by ID.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.InfraDynamicTemplateInfo "Template information"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/infra/{templateId} [get]
func RestGetInfraDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	result, err := common.GetInfraDynamicTemplate(nsId, templateId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetAllInfraDynamicTemplate godoc
// @ID GetAllInfraDynamicTemplate
// @Summary List all Infra Dynamic Templates
// @Description List all Infra Dynamic Templates in a namespace.
// @Description Optionally filter by keyword matching against template name or description (case-insensitive).
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param filterKeyword query string false "Keyword to filter templates by name or description"
// @Success 200 {object} model.InfraDynamicTemplateListResponse "List of templates"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/infra [get]
func RestGetAllInfraDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	filterKeyword := c.QueryParam("filterKeyword")

	result, err := common.ListInfraDynamicTemplate(nsId, filterKeyword)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	response := model.InfraDynamicTemplateListResponse{Templates: result}
	return clientManager.EndRequestWithLog(c, nil, response)
}

// RestPutInfraDynamicTemplate godoc
// @ID PutInfraDynamicTemplate
// @Summary Update an Infra Dynamic Template
// @Description Update an existing Infra Dynamic Template.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Param templateReq body model.InfraDynamicTemplateReq true "Infra Dynamic Template request"
// @Success 200 {object} model.InfraDynamicTemplateInfo "Updated template information"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/infra/{templateId} [put]
func RestPutInfraDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	req := &model.InfraDynamicTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.UpdateInfraDynamicTemplate(nsId, templateId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to update Infra dynamic template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteInfraDynamicTemplate godoc
// @ID DeleteInfraDynamicTemplate
// @Summary Delete an Infra Dynamic Template
// @Description Delete a specific Infra Dynamic Template.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.SimpleMsg "Template deleted successfully"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/infra/{templateId} [delete]
func RestDeleteInfraDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	err := common.DeleteInfraDynamicTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete Infra dynamic template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "Template '" + templateId + "' has been deleted"})
}

// RestDeleteAllInfraDynamicTemplate godoc
// @ID DeleteAllInfraDynamicTemplate
// @Summary Delete all Infra Dynamic Templates
// @Description Delete all Infra Dynamic Templates in a namespace.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg "All templates deleted successfully"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/infra [delete]
func RestDeleteAllInfraDynamicTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	err := common.DeleteAllInfraDynamicTemplate(nsId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete all Infra dynamic templates")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "All Infra dynamic templates have been deleted"})
}

// RestPostInfraDynamicFromTemplate godoc
// @ID PostInfraDynamicFromTemplate
// @Summary Create Infra from a Template
// @Description Create a new Infra by applying an Infra Dynamic Template.
// @Description The template provides the base node configuration, and the apply request
// @Description allows overriding the Infra name and description.
// @Description
// @Description **Override Behavior (Phase 1):**
// @Description - `name` (required): Name for the new Infra
// @Description - `description` (optional): Overrides the template's description
// @Description - All other configuration (specs, images, nodegroups) comes from the template
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID to apply"
// @Param applyReq body model.TemplateApplyReq true "Template apply request with Infra name and optional description"
// @Param option query string false "Deployment option: 'hold' to create Infra without immediate node provisioning" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking"
// @Success 200 {object} model.InfraInfo "Successfully created Infra from template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template or namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal deployment error"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/template/{templateId} [post]
func RestPostInfraDynamicFromTemplate(c echo.Context) error {
	ctx := c.Request().Context()
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")
	option := c.QueryParam("option")

	req := &model.TemplateApplyReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateInfraDynamicFromTemplate(ctx, nsId, templateId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create Infra from template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}
