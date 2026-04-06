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

// Package resource is to handle REST API for resource
package resource

import (
	"fmt"
	"net/http"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostVNetTemplate godoc
// @ID PostVNetTemplate
// @Summary Create a vNet Template
// @Description Create a reusable vNet Template. Templates store vNet creation
// @Description request configurations that can be applied later to create vNets with consistent settings.
// @Description
// @Description **Template Contents:**
// @Description - Connection name (cloud provider and region)
// @Description - CIDR block configuration
// @Description - Subnet definitions (names, CIDR blocks, zones)
// @Description - Description
// @Description
// @Description Templates can be created manually with desired vNet configurations.
// @Tags [Infra Resource] vNet Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateReq body model.VNetTemplateReq true "vNet Template request"
// @Success 200 {object} model.VNetTemplateInfo "Successfully created template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or template name"
// @Failure 409 {object} model.SimpleMsg "Template already exists"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/vNet [post]
func RestPostVNetTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	req := &model.VNetTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.CreateVNetTemplate(nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to create vNet template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetVNetTemplate godoc
// @ID GetVNetTemplate
// @Summary Get a vNet Template
// @Description Retrieve a specific vNet Template by ID.
// @Tags [Infra Resource] vNet Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.VNetTemplateInfo "Template information"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/vNet/{templateId} [get]
func RestGetVNetTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	result, err := common.GetVNetTemplate(nsId, templateId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetAllVNetTemplate godoc
// @ID GetAllVNetTemplate
// @Summary List all vNet Templates
// @Description List all vNet Templates in a namespace.
// @Description Optionally filter by keyword matching against template name or description (case-insensitive).
// @Tags [Infra Resource] vNet Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param filterKeyword query string false "Keyword to filter templates by name or description"
// @Success 200 {object} model.VNetTemplateListResponse "List of templates"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/vNet [get]
func RestGetAllVNetTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	filterKeyword := c.QueryParam("filterKeyword")

	result, err := common.ListVNetTemplate(nsId, filterKeyword)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	response := model.VNetTemplateListResponse{Templates: result}
	return clientManager.EndRequestWithLog(c, nil, response)
}

// RestPutVNetTemplate godoc
// @ID PutVNetTemplate
// @Summary Update a vNet Template
// @Description Update an existing vNet Template.
// @Tags [Infra Resource] vNet Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Param templateReq body model.VNetTemplateReq true "vNet Template request"
// @Success 200 {object} model.VNetTemplateInfo "Updated template information"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/vNet/{templateId} [put]
func RestPutVNetTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	req := &model.VNetTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.UpdateVNetTemplate(nsId, templateId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to update vNet template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteVNetTemplate godoc
// @ID DeleteVNetTemplate
// @Summary Delete a vNet Template
// @Description Delete a specific vNet Template.
// @Tags [Infra Resource] vNet Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.SimpleMsg "Template deleted successfully"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/vNet/{templateId} [delete]
func RestDeleteVNetTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	err := common.DeleteVNetTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete vNet template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "vNet template '" + templateId + "' has been deleted"})
}

// RestDeleteAllVNetTemplate godoc
// @ID DeleteAllVNetTemplate
// @Summary Delete all vNet Templates
// @Description Delete all vNet Templates in a namespace.
// @Tags [Infra Resource] vNet Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg "All templates deleted successfully"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/vNet [delete]
func RestDeleteAllVNetTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	err := common.DeleteAllVNetTemplate(nsId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete all vNet templates")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "All vNet templates have been deleted"})
}

// RestPostVNetFromTemplate godoc
// @ID PostVNetFromTemplate
// @Summary Create vNet from a Template
// @Description Create a new vNet by applying a vNet Template.
// @Description The template provides the base vNet configuration (connectionName, cidrBlock, subnets),
// @Description and the apply request allows overriding the vNet name and description.
// @Description
// @Description **Override Behavior (Phase 1):**
// @Description - `name` (required): Name for the new vNet
// @Description - `description` (optional): Overrides the template's description
// @Description - All other configuration (connectionName, cidrBlock, subnets) comes from the template
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID to apply"
// @Param applyReq body model.VNetTemplateApplyReq true "Template apply request with vNet name and optional description"
// @Success 200 {object} model.VNetInfo "Successfully created vNet from template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template or namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal resource creation error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/vNet/template/{templateId} [post]
func RestPostVNetFromTemplate(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	req := &model.VNetTemplateApplyReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Get the template
	template, err := common.GetVNetTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msg("failed to get vNet template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Policy-mode templates require connection context and are used for dynamic provisioning only
	if template.VNetPolicy != nil {
		err := fmt.Errorf("vNet template '%s' uses policy mode and cannot be applied directly; use dynamic provisioning (MCI) instead", templateId)
		log.Warn().Err(err).Msg("cannot apply policy-mode vNet template directly")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	if template.VNetReq == nil {
		err := fmt.Errorf("vNet template '%s' has no raw vNetReq defined", templateId)
		log.Error().Err(err).Msg("invalid vNet template state")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Build VNetReq from template with overrides
	vNetReq := *template.VNetReq
	vNetReq.Name = req.Name
	if req.Description != "" {
		vNetReq.Description = req.Description
	}

	// Create vNet using the template-derived request
	result, err := resource.CreateVNet(ctx, nsId, &vNetReq)
	if err != nil {
		log.Error().Err(err).Msg("failed to create vNet from template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// ====================================================================
// SecurityGroup Template Handlers
// ====================================================================

// RestPostSecurityGroupTemplate godoc
// @ID PostSecurityGroupTemplate
// @Summary Create a SecurityGroup Template
// @Description Create a reusable SecurityGroup Template. Templates store SecurityGroup creation
// @Description request configurations that can be applied later to create SecurityGroups with consistent settings.
// @Description
// @Description **Template Contents:**
// @Description - Connection name (cloud provider and region)
// @Description - vNet ID for the security group
// @Description - Firewall rules (ports, protocol, direction, CIDR)
// @Description - Description
// @Description
// @Description Templates can be created manually with desired SecurityGroup configurations.
// @Tags [Infra Resource] SecurityGroup Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateReq body model.SecurityGroupTemplateReq true "SecurityGroup Template request"
// @Success 200 {object} model.SecurityGroupTemplateInfo "Successfully created template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or template name"
// @Failure 409 {object} model.SimpleMsg "Template already exists"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/securityGroup [post]
func RestPostSecurityGroupTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	req := &model.SecurityGroupTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.CreateSecurityGroupTemplate(nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to create SecurityGroup template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetSecurityGroupTemplate godoc
// @ID GetSecurityGroupTemplate
// @Summary Get a SecurityGroup Template
// @Description Retrieve a specific SecurityGroup Template by ID.
// @Tags [Infra Resource] SecurityGroup Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.SecurityGroupTemplateInfo "Template information"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/securityGroup/{templateId} [get]
func RestGetSecurityGroupTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	result, err := common.GetSecurityGroupTemplate(nsId, templateId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetAllSecurityGroupTemplate godoc
// @ID GetAllSecurityGroupTemplate
// @Summary List all SecurityGroup Templates
// @Description List all SecurityGroup Templates in a namespace.
// @Description Optionally filter by keyword matching against template name or description (case-insensitive).
// @Tags [Infra Resource] SecurityGroup Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param filterKeyword query string false "Keyword to filter templates by name or description"
// @Success 200 {object} model.SecurityGroupTemplateListResponse "List of templates"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/securityGroup [get]
func RestGetAllSecurityGroupTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	filterKeyword := c.QueryParam("filterKeyword")

	result, err := common.ListSecurityGroupTemplate(nsId, filterKeyword)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	response := model.SecurityGroupTemplateListResponse{Templates: result}
	return clientManager.EndRequestWithLog(c, nil, response)
}

// RestPutSecurityGroupTemplate godoc
// @ID PutSecurityGroupTemplate
// @Summary Update a SecurityGroup Template
// @Description Update an existing SecurityGroup Template.
// @Tags [Infra Resource] SecurityGroup Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Param templateReq body model.SecurityGroupTemplateReq true "SecurityGroup Template request"
// @Success 200 {object} model.SecurityGroupTemplateInfo "Updated template information"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/securityGroup/{templateId} [put]
func RestPutSecurityGroupTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	req := &model.SecurityGroupTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := common.UpdateSecurityGroupTemplate(nsId, templateId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to update SecurityGroup template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteSecurityGroupTemplate godoc
// @ID DeleteSecurityGroupTemplate
// @Summary Delete a SecurityGroup Template
// @Description Delete a specific SecurityGroup Template.
// @Tags [Infra Resource] SecurityGroup Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.SimpleMsg "Template deleted successfully"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/securityGroup/{templateId} [delete]
func RestDeleteSecurityGroupTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	err := common.DeleteSecurityGroupTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete SecurityGroup template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "SecurityGroup template '" + templateId + "' has been deleted"})
}

// RestDeleteAllSecurityGroupTemplate godoc
// @ID DeleteAllSecurityGroupTemplate
// @Summary Delete all SecurityGroup Templates
// @Description Delete all SecurityGroup Templates in a namespace.
// @Tags [Infra Resource] SecurityGroup Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg "All templates deleted successfully"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/template/securityGroup [delete]
func RestDeleteAllSecurityGroupTemplate(c echo.Context) error {
	nsId := c.Param("nsId")

	err := common.DeleteAllSecurityGroupTemplate(nsId)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete all SecurityGroup templates")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "All SecurityGroup templates have been deleted"})
}

// RestPostSecurityGroupFromTemplate godoc
// @ID PostSecurityGroupFromTemplate
// @Summary Create SecurityGroup from a Template
// @Description Create a new SecurityGroup by applying a SecurityGroup Template.
// @Description The template provides the base SecurityGroup configuration (connectionName, vNetId, firewallRules),
// @Description and the apply request allows overriding the SecurityGroup name and description.
// @Description
// @Description **Override Behavior (Phase 1):**
// @Description - `name` (required): Name for the new SecurityGroup
// @Description - `description` (optional): Overrides the template's description
// @Description - All other configuration (connectionName, vNetId, firewallRules) comes from the template
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID to apply"
// @Param applyReq body model.SecurityGroupTemplateApplyReq true "Template apply request with SecurityGroup name and optional description"
// @Success 200 {object} model.SecurityGroupInfo "Successfully created SecurityGroup from template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template or namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal resource creation error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/securityGroup/template/{templateId} [post]
func RestPostSecurityGroupFromTemplate(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	templateId := c.Param("templateId")

	req := &model.SecurityGroupTemplateApplyReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Get the template
	template, err := common.GetSecurityGroupTemplate(nsId, templateId)
	if err != nil {
		log.Error().Err(err).Msg("failed to get SecurityGroup template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Build SecurityGroupReq from template with overrides
	sgReq := template.SecurityGroupReq
	sgReq.Name = req.Name
	if req.Description != "" {
		sgReq.Description = req.Description
	}

	// Create SecurityGroup using the template-derived request
	result, err := resource.CreateSecurityGroup(ctx, nsId, &sgReq, "")
	if err != nil {
		log.Error().Err(err).Msg("failed to create SecurityGroup from template")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}
