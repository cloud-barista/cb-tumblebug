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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostMci godoc
// @ID PostMci
// @Summary Create MCI
// @Description Create MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciReq true "Details for an MCI object"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci [post]
func RestPostMci(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.TbMciReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	option := "create"
	result, err := infra.CreateMci(nsId, req, option)
	return common.EndRequestWithLog(c, err, result)
}

// RestPostRegisterCSPNativeVM godoc
// @ID PostRegisterCSPNativeVM
// @Summary Register existing VM in a CSP to Cloud-Barista MCI
// @Description Register existing VM in a CSP to Cloud-Barista MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciReq true "Details for an MCI object with existing CSP VM ID"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/registerCspVm [post]
func RestPostRegisterCSPNativeVM(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.TbMciReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	option := "register"
	result, err := infra.CreateMci(nsId, req, option)
	return common.EndRequestWithLog(c, err, result)
}

// RestPostSystemMci godoc
// @ID PostSystemMci
// @Summary Create System MCI Dynamically for Special Purpose in NS:system
// @Description Create System MCI Dynamically for Special Purpose
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param option query string false "Option for the purpose of system MCI" Enums(probe)
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /systemMci [post]
func RestPostSystemMci(c echo.Context) error {

	option := c.QueryParam("option")

	req := &model.TbMciDynamicReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateSystemMciDynamic(option)
	return common.EndRequestWithLog(c, err, result)
}

// RestPostMciDynamic godoc
// @ID PostMciDynamic
// @Summary Create MCI Dynamically
// @Description Create MCI Dynamically from common spec and image
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciDynamicReq true "Request body to provision MCI dynamically. Must include commonSpec and commonImage info of each VM request.(ex: {name: mci01,vm: [{commonImage: aws+ap-northeast-2+ubuntu22.04,commonSpec: aws+ap-northeast-2+t2.small}]} ) You can use /mciRecommendVm and /mciDynamicCheckRequest to get it) Check the guide: https://github.com/cloud-barista/cb-tumblebug/discussions/1570"
// @Param option query string false "Option for MCI creation" Enums(hold)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mciDynamic [post]
func RestPostMciDynamic(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.TbMciDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return common.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciDynamic(reqID, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create MCI dynamically")
		return common.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostMciVmDynamic godoc
// @ID PostMciVmDynamic
// @Summary Create VM Dynamically and add it to MCI
// @Description Create VM Dynamically and add it to MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmReq body model.TbVmDynamicReq true "Details for Vm dynamic request"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vmDynamic [post]
func RestPostMciVmDynamic(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &model.TbVmDynamicReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciVmDynamic(nsId, mciId, req)
	return common.EndRequestWithLog(c, err, result)
}

// RestPostMciDynamicCheckRequest godoc
// @ID PostMciDynamicCheckRequest
// @Summary Check available ConnectionConfig list for creating MCI Dynamically
// @Description Check available ConnectionConfig list before create MCI Dynamically from common spec and image
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param mciReq body model.MciConnectionConfigCandidatesReq true "Details for MCI dynamic request information"
// @Success 200 {object} model.CheckMciDynamicReqInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /mciDynamicCheckRequest [post]
func RestPostMciDynamicCheckRequest(c echo.Context) error {

	req := &model.MciConnectionConfigCandidatesReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CheckMciDynamicReq(req)
	return common.EndRequestWithLog(c, err, result)
}

// RestPostMciVm godoc
// @ID PostMciVm
// @Summary Create and add homogeneous VMs(subGroup) to a specified MCI (Set subGroupSize for multiple VMs)
// @Description Create and add homogeneous VMs(subGroup) to a specified MCI (Set subGroupSize for multiple VMs)
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmReq body model.TbVmReq true "Details for VMs(subGroup)"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm [post]
func RestPostMciVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	vmInfoData := &model.TbVmReq{}
	if err := c.Bind(vmInfoData); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}
	result, err := infra.CreateMciGroupVm(nsId, mciId, vmInfoData, true)
	return common.EndRequestWithLog(c, err, result)
}

// RestPostMciSubGroupScaleOut godoc
// @ID PostMciSubGroupScaleOut
// @Summary ScaleOut subGroup in specified MCI
// @Description ScaleOut subGroup in specified MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param subgroupId path string true "subGroup ID" default(g1)
// @Param vmReq body model.TbScaleOutSubGroupReq true "subGroup scaleOut request"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/subgroup/{subgroupId} [post]
func RestPostMciSubGroupScaleOut(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subgroupId := c.Param("subgroupId")

	scaleOutReq := &model.TbScaleOutSubGroupReq{}
	if err := c.Bind(scaleOutReq); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ScaleOutMciSubGroup(nsId, mciId, subgroupId, scaleOutReq.NumVMsToAdd)
	return common.EndRequestWithLog(c, err, result)
}
