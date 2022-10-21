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
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// JSONResult is a dummy struct for Swagger annotations.
type JSONResult struct {
	//Code    int          `json:"code" `
	//Message string       `json:"message"`
	//Data    interface{}  `json:"data"`
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention Need to be revised.

// RestGetMcis godoc
// @Summary Get MCIS object (option: status, accessInfo, vmId)
// @Description Get MCIS object (option: status, accessInfo, vmId)
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param option query string false "Option" Enums(default, id, status, accessinfo)
// @Param filterKey query string false "(For option=id) Field key for filtering (ex: connectionName)"
// @Param filterVal query string false "(For option=id) Field value for filtering (ex: aws-ap-northeast-2)"
// @Param accessInfoOption query string false "(For option=accessinfo) accessInfoOption (showSshKey)"
// @success 200 {object} JSONResult{[DEFAULT]=mcis.TbMcisInfo,[ID]=common.IdList,[STATUS]=mcis.McisStatusInfo,[AccessInfo]=mcis.McisAccessInfo} "Different return structures by the given action param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [get]
func RestGetMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	option := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")
	accessInfoOption := c.QueryParam("accessInfoOption")

	if option == "id" {
		content := common.IdList{}
		var err error
		content.IdList, err = mcis.ListVmByFilter(nsId, mcisId, filterKey, filterVal)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		return c.JSON(http.StatusOK, &content)
	} else if option == "status" {

		result, err := mcis.GetMcisStatus(nsId, mcisId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		var content struct {
			Result *mcis.McisStatusInfo `json:"status"`
		}
		content.Result = result

		common.PrintJsonPretty(content)
		return c.JSON(http.StatusOK, &content)

	} else if option == "accessinfo" {

		result, err := mcis.GetMcisAccessInfo(nsId, mcisId, accessInfoOption)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		common.PrintJsonPretty(result)
		return c.JSON(http.StatusOK, &result)

	} else {

		result, err := mcis.GetMcisInfo(nsId, mcisId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		common.PrintJsonPretty(*result)
		return c.JSON(http.StatusOK, result)

	}
}

// RestGetAllMcisResponse is a response structure for RestGetAllMcis
type RestGetAllMcisResponse struct {
	Mcis []mcis.TbMcisInfo `json:"mcis"`
}

// RestGetAllMcisStatusResponse is a response structure for RestGetAllMcisStatus
type RestGetAllMcisStatusResponse struct {
	Mcis []mcis.McisStatusInfo `json:"mcis"`
}

// RestGetAllMcis godoc
// @Summary List all MCISs or MCISs' ID
// @Description List all MCISs or MCISs' ID
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option" Enums(id, simple, status)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllMcisResponse,[SIMPLE]=RestGetAllMcisResponse,[ID]=common.IdList,[STATUS]=RestGetAllMcisStatusResponse} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [get]
func RestGetAllMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	option := c.QueryParam("option")
	fmt.Println("[Get MCIS List requested with option: " + option)

	if option == "id" {
		// return MCIS IDs
		content := common.IdList{}
		var err error
		content.IdList, err = mcis.ListMcisId(nsId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		return c.JSON(http.StatusOK, &content)
	} else if option == "status" {
		// return MCIS Status objects (diffent with MCIS objects)
		result, err := mcis.GetMcisStatusAll(nsId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}
		content := RestGetAllMcisStatusResponse{}
		content.Mcis = result
		return c.JSON(http.StatusOK, &content)
	} else if option == "simple" {
		// MCIS in simple (without VM information)
		result, err := mcis.CoreGetAllMcis(nsId, option)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}
		content := RestGetAllMcisResponse{}
		content.Mcis = result
		return c.JSON(http.StatusOK, &content)
	} else {
		// MCIS in detail (with status information)
		result, err := mcis.CoreGetAllMcis(nsId, "status")
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}
		content := RestGetAllMcisResponse{}
		content.Mcis = result
		return c.JSON(http.StatusOK, &content)
	}

}

/*
	function RestPutMcis not yet implemented

// RestPutMcis godoc
// @Summary Update MCIS
// @Description Update MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param mcisInfo body TbMcisInfo true "Details for an MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [put]
func RestPutMcis(c echo.Context) error {
	return nil
}
*/

// RestDelMcis godoc
// @Summary Delete MCIS
// @Description Delete MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param option query string false "Option for delete MCIS (support force delete)" Enums(terminate,force)
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [delete]
func RestDelMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	option := c.QueryParam("option")

	output, err := mcis.DelMcis(nsId, mcisId, option)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, output)
}

// RestDelAllMcis godoc
// @Summary Delete all MCISs
// @Description Delete all MCISs
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option for delete MCIS (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [delete]
func RestDelAllMcis(c echo.Context) error {
	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	result, err := mcis.DelAllMcis(nsId, option)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": result}
	return c.JSON(http.StatusOK, &mapA)
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention needs to be revised.

// RestGetMcisVm godoc
// @Summary Get VM in specified MCIS
// @Description Get VM in specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string false "Option for MCIS" Enums(default, status, idsInDetail)
// @success 200 {object} JSONResult{[DEFAULT]=mcis.TbVmInfo,[STATUS]=mcis.TbVmStatusInfo,[IDNAME]=mcis.TbIdNameInDetailInfo} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [get]
func RestGetMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	option := c.QueryParam("option")

	switch option {
	case "status":
		result, err := mcis.CoreGetMcisVmStatus(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		common.PrintJsonPretty(*result)

		return c.JSON(http.StatusOK, result)

	case "idsInDetail":
		result, err := mcis.GetVmIdNameInDetail(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		return c.JSON(http.StatusOK, result)

	default:
		result, err := mcis.CoreGetMcisVmInfo(nsId, mcisId, vmId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		common.PrintJsonPretty(*result)

		return c.JSON(http.StatusOK, result)
	}
}

/* RestPutMcisVm function not yet implemented
// RestPutSshKey godoc
// @Summary Update MCIS
// @Description Update MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param vmInfo body mcis.TbVmInfo true "Details for an VM object"
// @Success 200 {object} mcis.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [put]
func RestPutMcisVm(c echo.Context) error {
	return nil
}
*/

// RestDelMcisVm godoc
// @Summary Delete VM in specified MCIS
// @Description Delete VM in specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string false "Option for delete VM (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [delete]
func RestDelMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")
	option := c.QueryParam("option")

	err := mcis.DelMcisVm(nsId, mcisId, vmId, option)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the VM info"}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": "Deleted the VM info"}
	return c.JSON(http.StatusOK, &mapA)
}

// RestGetMcisGroupVms godoc
// @Summary List VMs with a SubGroup label in a specified MCIS
// @Description List VMs with a SubGroup label in a specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param subgroupId path string true "subGroup ID" default(g1)
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/subgroup/{subgroupId} [get]
func RestGetMcisGroupVms(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	subgroupId := c.Param("subgroupId")
	//option := c.QueryParam("option")

	content := common.IdList{}
	var err error
	content.IdList, err = mcis.ListMcisGroupVms(nsId, mcisId, subgroupId)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	return c.JSON(http.StatusOK, &content)

}

// RestGetMcisGroupIds godoc
// @Summary List SubGroup IDs in a specified MCIS
// @Description List SubGroup IDs in a specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/subgroup [get]
func RestGetMcisGroupIds(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	//option := c.QueryParam("option")

	content := common.IdList{}
	var err error
	content.IdList, err = mcis.ListSubGroupId(nsId, mcisId)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	return c.JSON(http.StatusOK, &content)

}
