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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// JSONResult is a dummy struct for Swagger annotations.
type JSONResult struct {
	//Code    int          `json:"code" `
	//Message string       `json:"message"`
	//Data    interface{}  `json:"data"`
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention Need to be revised.

// RestGetMci godoc
// @ID GetMci
// @Summary Get MCI object (option: status, accessInfo, vmId)
// @Description Get MCI object (option: status, accessInfo, vmId)
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param option query string false "Option" Enums(default, id, status, accessinfo)
// @Param filterKey query string false "(For option=id) Field key for filtering (ex: connectionName)"
// @Param filterVal query string false "(For option=id) Field value for filtering (ex: aws-ap-northeast-2)"
// @Param accessInfoOption query string false "(For option=accessinfo) accessInfoOption (showSshKey)"
// @success 200 {object} JSONResult{[DEFAULT]=model.TbMciInfo,[ID]=model.IdList,[STATUS]=model.MciStatusInfo,[AccessInfo]=model.MciAccessInfo} "Different return structures by the given action param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId} [get]
func RestGetMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	option := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")
	accessInfoOption := c.QueryParam("accessInfoOption")

	if option == "id" {
		content := model.IdList{}
		var err error
		content.IdList, err = infra.ListVmByFilter(nsId, mciId, filterKey, filterVal)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if option == "status" {

		result, err := infra.GetMciStatus(nsId, mciId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}

		var content struct {
			Result *model.MciStatusInfo `json:"status"`
		}
		content.Result = result

		return common.EndRequestWithLog(c, reqID, err, content)

	} else if option == "accessinfo" {

		result, err := infra.GetMciAccessInfo(nsId, mciId, accessInfoOption)
		return common.EndRequestWithLog(c, reqID, err, result)

	} else {

		result, err := infra.GetMciInfo(nsId, mciId)
		return common.EndRequestWithLog(c, reqID, err, result)

	}
}

// RestGetAllMciResponse is a response structure for RestGetAllMci
type RestGetAllMciResponse struct {
	Mci []model.TbMciInfo `json:"mci"`
}

// RestGetAllMciStatusResponse is a response structure for RestGetAllMciStatus
type RestGetAllMciStatusResponse struct {
	Mci []model.MciStatusInfo `json:"mci"`
}

// RestGetAllMci godoc
// @ID GetAllMci
// @Summary List all MCIs or MCIs' ID
// @Description List all MCIs or MCIs' ID
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id, simple, status)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllMciResponse,[SIMPLE]=RestGetAllMciResponse,[ID]=model.IdList,[STATUS]=RestGetAllMciStatusResponse} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci [get]
func RestGetAllMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	if option == "id" {
		// return MCI IDs
		content := model.IdList{}
		var err error
		content.IdList, err = infra.ListMciId(nsId)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if option == "status" {
		// return MCI Status objects (diffent with MCI objects)
		result, err := infra.ListMciStatus(nsId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content := RestGetAllMciStatusResponse{}
		content.Mci = result
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if option == "simple" {
		// MCI in simple (without VM information)
		result, err := infra.ListMciInfo(nsId, option)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content := RestGetAllMciResponse{}
		content.Mci = result
		return common.EndRequestWithLog(c, reqID, err, content)
	} else {
		// MCI in detail (with status information)
		result, err := infra.ListMciInfo(nsId, "status")
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content := RestGetAllMciResponse{}
		content.Mci = result
		return common.EndRequestWithLog(c, reqID, err, content)
	}
}

/*
	function RestPutMci not yet implemented

// RestPutMci godoc
// @ID PutMci
// @Summary Update MCI
// @Description Update MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param mciInfo body TbMciInfo true "Details for an MCI object"
// @Success 200 {object} TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId} [put]
func RestPutMci(c echo.Context) error {
	return nil
}
*/

// RestDelMci godoc
// @ID DelMci
// @Summary Delete MCI
// @Description Delete MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param option query string false "Option for delete MCI (support force delete)" Enums(terminate,force)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId} [delete]
func RestDelMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	option := c.QueryParam("option")

	content, err := infra.DelMci(nsId, mciId, option)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestDelAllMci godoc
// @ID DelAllMci
// @Summary Delete all MCIs
// @Description Delete all MCIs
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option for delete all MCIs (support force object delete, terminate before delete)" Enums(force, terminate)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci [delete]
func RestDelAllMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	message, err := infra.DelAllMci(nsId, option)
	result := model.SimpleMsg{Message: message}
	return common.EndRequestWithLog(c, reqID, err, result)
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention needs to be revised.

// RestGetMciVm godoc
// @ID GetMciVm
// @Summary Get VM in specified MCI
// @Description Get VM in specified MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string false "Option for MCI" Enums(default, status, idsInDetail)
// @success 200 {object} JSONResult{[DEFAULT]=model.TbVmInfo,[STATUS]=model.TbVmStatusInfo,[IDNAME]=model.TbIdNameInDetailInfo} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId} [get]
func RestGetMciVm(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	option := c.QueryParam("option")

	switch option {
	case "status":
		result, err := infra.GetMciVmStatus(nsId, mciId, vmId)
		return common.EndRequestWithLog(c, reqID, err, result)

	case "idsInDetail":
		result, err := infra.GetVmIdNameInDetail(nsId, mciId, vmId)
		return common.EndRequestWithLog(c, reqID, err, result)

	default:
		result, err := infra.ListVmInfo(nsId, mciId, vmId)
		return common.EndRequestWithLog(c, reqID, err, result)
	}
}

/* RestPutMciVm function not yet implemented
// RestPutSshKey godoc
// @ID PutSshKey
// @Summary Update MCI
// @Description Update MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param vmInfo body model.TbVmInfo true "Details for an VM object"
// @Success 200 {object} model.TbVmInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId} [put]
func RestPutMciVm(c echo.Context) error {
	return nil
}
*/

// RestDelMciVm godoc
// @ID DelMciVm
// @Summary Delete VM in specified MCI
// @Description Delete VM in specified MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string false "Option for delete VM (support force delete)" Enums(force)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId} [delete]
func RestDelMciVm(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")
	option := c.QueryParam("option")

	err := infra.DelMciVm(nsId, mciId, vmId, option)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("Failed to delete the VM info")
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	result := map[string]string{"message": "Deleted the VM info"}
	return common.EndRequestWithLog(c, reqID, err, result)
}

// RestGetMciGroupVms godoc
// @ID GetMciGroupVms
// @Summary List VMs with a SubGroup label in a specified MCI
// @Description List VMs with a SubGroup label in a specified MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param subgroupId path string true "subGroup ID" default(g1)
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/subgroup/{subgroupId} [get]
func RestGetMciGroupVms(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subgroupId := c.Param("subgroupId")
	//option := c.QueryParam("option")

	content := model.IdList{}
	var err error
	content.IdList, err = infra.ListVmBySubGroup(nsId, mciId, subgroupId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetMciGroupIds godoc
// @ID GetMciGroupIds
// @Summary List SubGroup IDs in a specified MCI
// @Description List SubGroup IDs in a specified MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/subgroup [get]
func RestGetMciGroupIds(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	//option := c.QueryParam("option")

	content := model.IdList{}
	var err error
	content.IdList, err = infra.ListSubGroupId(nsId, mciId)
	return common.EndRequestWithLog(c, reqID, err, content)
}
