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
	"fmt"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostInfraPolicy godoc
// @ID PostInfraPolicy
// @Summary Create Infra Automation policy
// @Description Create Infra Automation policy
// @Tags [MC-Infra] Infra Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param infraPolicyReq body model.InfraPolicyReq true "Details for an Infra automation policy request"
// @Success 200 {object} model.InfraPolicyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/policy/infra/{infraId} [post]
func RestPostInfraPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.InfraPolicyReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.CreateInfraPolicy(nsId, infraId, req)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetInfraPolicy godoc
// @ID GetInfraPolicy
// @Summary Get Infra Policy
// @Description Get Infra Policy
// @Tags [MC-Infra] Infra Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Success 200 {object} model.InfraPolicyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/policy/infra/{infraId} [get]
func RestGetInfraPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	result, err := infra.GetInfraPolicyObject(nsId, infraId)
	if err != nil {
		errorMessage := fmt.Errorf("Error to find InfraPolicyObject : " + infraId + "ERROR : " + err.Error())
		return clientManager.EndRequestWithLog(c, errorMessage, nil)
	}

	if result.Id == "" {
		errorMessage := fmt.Errorf("Failed to find InfraPolicyObject : " + infraId)
		return clientManager.EndRequestWithLog(c, errorMessage, nil)
	}
	return clientManager.EndRequestWithLog(c, err, result)
}

// Response structure for RestGetAllInfraPolicy
type RestGetAllInfraPolicyResponse struct {
	InfraPolicy []model.InfraPolicyInfo `json:"infraPolicy"`
}

// RestGetAllInfraPolicy godoc
// @ID GetAllInfraPolicy
// @Summary List all Infra policies
// @Description List all Infra policies
// @Tags [MC-Infra] Infra Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} RestGetAllInfraPolicyResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/policy/infra [get]
func RestGetAllInfraPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	log.Debug().Msg("[Get Infra Policy List]")

	result, err := infra.GetAllInfraPolicyObject(nsId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content := RestGetAllInfraPolicyResponse{}
	content.InfraPolicy = result
	return clientManager.EndRequestWithLog(c, err, content)
}

/*
	function RestPutInfraPolicy not yet implemented

// RestPutInfraPolicy godoc
// @ID PutInfraPolicy
// @Summary Update Infra Policy
// @Description Update Infra Policy
// @Tags [MC-Infra] Infra Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param infraInfo body InfraPolicyInfo true "Details for an Infra Policy object"
// @Success 200 {object} InfraPolicyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/policy/infra/{infraId} [put]
*/
func RestPutInfraPolicy(c echo.Context) error {
	return nil
}

// DelInfraPolicy godoc
// @ID DelInfraPolicy
// @Summary Delete Infra Policy
// @Description Delete Infra Policy
// @Tags [MC-Infra] Infra Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/policy/infra/{infraId} [delete]
func RestDelInfraPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	err := infra.DelInfraPolicy(nsId, infraId)
	result := map[string]string{"message": "Deleted the Infra Policy info"}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestDelAllInfraPolicy godoc
// @ID DelAllInfraPolicy
// @Summary Delete all Infra policies
// @Description Delete all Infra policies
// @Tags [MC-Infra] Infra Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/policy/infra [delete]
func RestDelAllInfraPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	result, err := infra.DelAllInfraPolicy(nsId)
	return clientManager.EndRequestWithLog(c, err, result)
}
