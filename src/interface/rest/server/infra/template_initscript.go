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

package infra

import (
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostInitScriptTemplate godoc
// @ID PostInitScriptTemplate
// @Summary Create an init script Template
// @Description Create a reusable init script Template. An init script runs on a Node at first boot
// @Description (delivered as cloud-init user data) and is referenced from `initScript.templateId` or
// @Description `initScript.scripts[].templateId` of an Infra or NodeGroup request.
// @Description
// @Description Templates in the `system` namespace are shared with every namespace; a template with the
// @Description same id in the request namespace takes precedence. The script is run by bash with the
// @Description `TB_*` node identity variables and the template/request `env` exported.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateReq body model.InitScriptTemplateReq true "init script Template request"
// @Success 200 {object} model.InitScriptTemplateInfo "Successfully created template"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or template name"
// @Failure 409 {object} model.SimpleMsg "Template already exists"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/template/initScript [post]
func RestPostInitScriptTemplate(c echo.Context) error {
	nsId := c.Param("nsId")
	req := &model.InitScriptTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	result, err := common.CreateInitScriptTemplate(nsId, req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetInitScriptTemplate godoc
// @ID GetInitScriptTemplate
// @Summary Get an init script Template
// @Description Get an init script Template from the given namespace (no fallback to `system`).
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param templateId path string true "Template ID" default(initscript-hello)
// @Success 200 {object} model.InitScriptTemplateInfo "Template information"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/template/initScript/{templateId} [get]
func RestGetInitScriptTemplate(c echo.Context) error {
	result, err := common.GetInitScriptTemplate(c.Param("nsId"), c.Param("templateId"))
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetAllInitScriptTemplate godoc
// @ID GetAllInitScriptTemplate
// @Summary List all init script Templates
// @Description List init script Templates in a namespace. Shared templates live in the `system` namespace.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param filterKeyword query string false "Keyword to filter templates by name or description"
// @Success 200 {object} model.InitScriptTemplateListResponse "List of templates"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/template/initScript [get]
func RestGetAllInitScriptTemplate(c echo.Context) error {
	result, err := common.ListInitScriptTemplate(c.Param("nsId"), c.QueryParam("filterKeyword"))
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.InitScriptTemplateListResponse{Templates: result})
}

// RestPutInitScriptTemplate godoc
// @ID PutInitScriptTemplate
// @Summary Update an init script Template
// @Description Replace the script, env and description of an init script Template. Infras created earlier keep
// @Description the content they were created with; the change applies to Infras and NodeGroups created afterwards.
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Param templateReq body model.InitScriptTemplateReq true "init script Template request"
// @Success 200 {object} model.InitScriptTemplateInfo "Updated template information"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/template/initScript/{templateId} [put]
func RestPutInitScriptTemplate(c echo.Context) error {
	req := &model.InitScriptTemplateReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	result, err := common.UpdateInitScriptTemplate(c.Param("nsId"), c.Param("templateId"), req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestDeleteInitScriptTemplate godoc
// @ID DeleteInitScriptTemplate
// @Summary Delete an init script Template
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param templateId path string true "Template ID"
// @Success 200 {object} model.SimpleMsg "Template deleted successfully"
// @Failure 404 {object} model.SimpleMsg "Template not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/template/initScript/{templateId} [delete]
func RestDeleteInitScriptTemplate(c echo.Context) error {
	templateId := c.Param("templateId")
	if err := common.DeleteInitScriptTemplate(c.Param("nsId"), templateId); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "Template '" + templateId + "' has been deleted"})
}

// RestDeleteAllInitScriptTemplate godoc
// @ID DeleteAllInitScriptTemplate
// @Summary Delete all init script Templates
// @Tags [MC-Infra] Infra Template Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg "All templates deleted successfully"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/template/initScript [delete]
func RestDeleteAllInitScriptTemplate(c echo.Context) error {
	if err := common.DeleteAllInitScriptTemplate(c.Param("nsId")); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "All init script templates have been deleted"})
}
