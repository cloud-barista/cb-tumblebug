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

// Package mci is to handle REST API for mci
package infra

import (
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostMciPolicy godoc
// @ID PostMciPolicy
// @Summary Create MCI Automation policy
// @Description Create MCI Automation policy
// @Tags [MC-Infra] MCI Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param mciPolicyReq body model.MciPolicyReq true "Details for an MCI automation policy request"
// @Success 200 {object} model.MciPolicyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/policy/mci/{mciId} [post]
func RestPostMciPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &model.MciPolicyReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.CreateMciPolicy(nsId, mciId, req)
	return common.EndRequestWithLog(c, err, content)
}

// RestGetMciPolicy godoc
// @ID GetMciPolicy
// @Summary Get MCI Policy
// @Description Get MCI Policy
// @Tags [MC-Infra] MCI Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Success 200 {object} model.MciPolicyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/policy/mci/{mciId} [get]
func RestGetMciPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	result, err := infra.GetMciPolicyObject(nsId, mciId)
	if err != nil {
		errorMessage := fmt.Errorf("Error to find MciPolicyObject : " + mciId + "ERROR : " + err.Error())
		return common.EndRequestWithLog(c, errorMessage, nil)
	}

	if result.Id == "" {
		errorMessage := fmt.Errorf("Failed to find MciPolicyObject : " + mciId)
		return common.EndRequestWithLog(c, errorMessage, nil)
	}
	return common.EndRequestWithLog(c, err, result)
}

// Response structure for RestGetAllMciPolicy
type RestGetAllMciPolicyResponse struct {
	MciPolicy []model.MciPolicyInfo `json:"mciPolicy"`
}

// RestGetAllMciPolicy godoc
// @ID GetAllMciPolicy
// @Summary List all MCI policies
// @Description List all MCI policies
// @Tags [MC-Infra] MCI Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} RestGetAllMciPolicyResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/policy/mci [get]
func RestGetAllMciPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	log.Debug().Msg("[Get MCI Policy List]")

	result, err := infra.GetAllMciPolicyObject(nsId)
	if err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	content := RestGetAllMciPolicyResponse{}
	content.MciPolicy = result
	return common.EndRequestWithLog(c, err, content)
}

/*
	function RestPutMciPolicy not yet implemented

// RestPutMciPolicy godoc
// @ID PutMciPolicy
// @Summary Update MCI Policy
// @Description Update MCI Policy
// @Tags [MC-Infra] MCI Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param mciInfo body MciPolicyInfo true "Details for an MCI Policy object"
// @Success 200 {object} MciPolicyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/policy/mci/{mciId} [put]
*/
func RestPutMciPolicy(c echo.Context) error {
	return nil
}

// DelMciPolicy godoc
// @ID DelMciPolicy
// @Summary Delete MCI Policy
// @Description Delete MCI Policy
// @Tags [MC-Infra] MCI Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/policy/mci/{mciId} [delete]
func RestDelMciPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	err := infra.DelMciPolicy(nsId, mciId)
	result := map[string]string{"message": "Deleted the MCI Policy info"}
	return common.EndRequestWithLog(c, err, result)
}

// RestDelAllMciPolicy godoc
// @ID DelAllMciPolicy
// @Summary Delete all MCI policies
// @Description Delete all MCI policies
// @Tags [MC-Infra] MCI Orchestration Management (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/policy/mci [delete]
func RestDelAllMciPolicy(c echo.Context) error {

	nsId := c.Param("nsId")
	result, err := infra.DelAllMciPolicy(nsId)
	return common.EndRequestWithLog(c, err, result)
}
