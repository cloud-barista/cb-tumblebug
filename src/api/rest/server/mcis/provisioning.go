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

// Package mcis is to handle REST API for mcis
package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostMcis godoc
// @Summary Create MCIS
// @Description Create MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisReq body TbMcisReq true "Details for an MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [post]
func RestPostMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	option := "create"
	result, err := mcis.CreateMcis(nsId, req, option)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostRegisterCSPNativeVM godoc
// @Summary Register existing VM in a CSP to Cloud-Barista MCIS
// @Description Register existing VM in a CSP to Cloud-Barista MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisReq body TbMcisReq true "Details for an MCIS object with existing CSP VM ID"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/registerCspVm [post]
func RestPostRegisterCSPNativeVM(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	option := "register"
	result, err := mcis.CreateMcis(nsId, req, option)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostSystemMcis godoc
// @Summary Create System MCIS Dynamically for Special Purpose in NS:system-purpose-common-ns
// @Description Create System MCIS Dynamically for Special Purpose
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param option query string false "Option for the purpose of system MCIS" Enums(probe)
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /systemMcis [post]
func RestPostSystemMcis(c echo.Context) error {

	option := c.QueryParam("option")

	req := &mcis.TbMcisDynamicReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CreateSystemMcisDynamic(option)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisDynamic godoc
// @Summary Create MCIS Dynamically
// @Description Create MCIS Dynamically from common spec and image
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisReq body TbMcisDynamicReq true "Details for MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcisDynamic [post]
func RestPostMcisDynamic(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisDynamicReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CreateMcisDynamic(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisVmDynamic godoc
// @Summary Create VM Dynamically and add it to MCIS
// @Description Create VM Dynamically and add it to MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmReq body TbVmDynamicReq true "Details for Vm dynamic request"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vmDynamic [post]
func RestPostMcisVmDynamic(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.TbVmDynamicReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CreateMcisVmDynamic(nsId, mcisId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisDynamicCheckRequest godoc
// @Summary Check available ConnectionConfig list for creating MCIS Dynamically
// @Description Check available ConnectionConfig list before create MCIS Dynamically from common spec and image
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param mcisReq body McisConnectionConfigCandidatesReq true "Details for MCIS dynamic request information"
// @Success 200 {object} CheckMcisDynamicReqInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /mcisDynamicCheckRequest [post]
func RestPostMcisDynamicCheckRequest(c echo.Context) error {

	req := &mcis.McisConnectionConfigCandidatesReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CheckMcisDynamicReq(req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusOK, result)
}

// RestPostMcisVm godoc
// @Summary Create and add homogeneous VMs(subGroup) to a specified MCIS (Set subGroupSize for multiple VMs)
// @Description Create and add homogeneous VMs(subGroup) to a specified MCIS (Set subGroupSize for multiple VMs)
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmReq body mcis.TbVmReq true "Details for VMs(subGroup)"
// @Success 200 {object} mcis.TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm [post]
func RestPostMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	vmInfoData := &mcis.TbVmReq{}
	if err := c.Bind(vmInfoData); err != nil {
		return err
	}
	common.PrintJsonPretty(*vmInfoData)

	result, err := mcis.CreateMcisGroupVm(nsId, mcisId, vmInfoData, true)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisSubGroupScaleOut godoc
// @Summary ScaleOut subGroup in specified MCIS
// @Description ScaleOut subGroup in specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param subgroupId path string true "subGroup ID" default(g1)
// @Param vmReq body mcis.TbScaleOutSubGroupReq true "subGroup scaleOut request"
// @Success 200 {object} mcis.TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/subgroup/{subgroupId} [post]
func RestPostMcisSubGroupScaleOut(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	subgroupId := c.Param("subgroupId")

	scaleOutReq := &mcis.TbScaleOutSubGroupReq{}
	if err := c.Bind(scaleOutReq); err != nil {
		return err
	}

	result, err := mcis.ScaleOutMcisSubGroup(nsId, mcisId, subgroupId, scaleOutReq.NumVMsToAdd)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}
