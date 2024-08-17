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
package mci

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mci"
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
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param option query string false "Option" Enums(default, id, status, accessinfo)
// @Param filterKey query string false "(For option=id) Field key for filtering (ex: connectionName)"
// @Param filterVal query string false "(For option=id) Field value for filtering (ex: aws-ap-northeast-2)"
// @Param accessInfoOption query string false "(For option=accessinfo) accessInfoOption (showSshKey)"
// @success 200 {object} JSONResult{[DEFAULT]=mci.TbMciInfo,[ID]=common.IdList,[STATUS]=mci.MciStatusInfo,[AccessInfo]=mci.MciAccessInfo} "Different return structures by the given action param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
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
		content := common.IdList{}
		var err error
		content.IdList, err = mci.ListVmByFilter(nsId, mciId, filterKey, filterVal)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if option == "status" {

		result, err := mci.GetMciStatus(nsId, mciId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}

		var content struct {
			Result *mci.MciStatusInfo `json:"status"`
		}
		content.Result = result

		return common.EndRequestWithLog(c, reqID, err, content)

	} else if option == "accessinfo" {

		result, err := mci.GetMciAccessInfo(nsId, mciId, accessInfoOption)
		return common.EndRequestWithLog(c, reqID, err, result)

	} else {

		result, err := mci.GetMciInfo(nsId, mciId)
		return common.EndRequestWithLog(c, reqID, err, result)

	}
}

// RestGetAllMciResponse is a response structure for RestGetAllMci
type RestGetAllMciResponse struct {
	Mci []mci.TbMciInfo `json:"mci"`
}

// RestGetAllMciStatusResponse is a response structure for RestGetAllMciStatus
type RestGetAllMciStatusResponse struct {
	Mci []mci.MciStatusInfo `json:"mci"`
}

// RestGetAllMci godoc
// @ID GetAllMci
// @Summary List all MCIs or MCIs' ID
// @Description List all MCIs or MCIs' ID
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id, simple, status)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllMciResponse,[SIMPLE]=RestGetAllMciResponse,[ID]=common.IdList,[STATUS]=RestGetAllMciStatusResponse} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
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
		content := common.IdList{}
		var err error
		content.IdList, err = mci.ListMciId(nsId)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if option == "status" {
		// return MCI Status objects (diffent with MCI objects)
		result, err := mci.ListMciStatus(nsId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content := RestGetAllMciStatusResponse{}
		content.Mci = result
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if option == "simple" {
		// MCI in simple (without VM information)
		result, err := mci.ListMciInfo(nsId, option)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content := RestGetAllMciResponse{}
		content.Mci = result
		return common.EndRequestWithLog(c, reqID, err, content)
	} else {
		// MCI in detail (with status information)
		result, err := mci.ListMciInfo(nsId, "status")
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
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param mciInfo body TbMciInfo true "Details for an MCI object"
// @Success 200 {object} TbMciInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId} [put]
func RestPutMci(c echo.Context) error {
	return nil
}
*/

// RestDelMci godoc
// @ID DelMci
// @Summary Delete MCI
// @Description Delete MCI
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param option query string false "Option for delete MCI (support force delete)" Enums(terminate,force)
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId} [delete]
func RestDelMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	option := c.QueryParam("option")

	content, err := mci.DelMci(nsId, mciId, option)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestDelAllMci godoc
// @ID DelAllMci
// @Summary Delete all MCIs
// @Description Delete all MCIs
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option for delete MCI (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci [delete]
func RestDelAllMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	result, err := mci.DelAllMci(nsId, option)
	return common.EndRequestWithLog(c, reqID, err, result)
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention needs to be revised.

// RestGetMciVm godoc
// @ID GetMciVm
// @Summary Get VM in specified MCI
// @Description Get VM in specified MCI
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string false "Option for MCI" Enums(default, status, idsInDetail)
// @success 200 {object} JSONResult{[DEFAULT]=mci.TbVmInfo,[STATUS]=mci.TbVmStatusInfo,[IDNAME]=mci.TbIdNameInDetailInfo} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
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
		result, err := mci.GetMciVmStatus(nsId, mciId, vmId)
		return common.EndRequestWithLog(c, reqID, err, result)

	case "idsInDetail":
		result, err := mci.GetVmIdNameInDetail(nsId, mciId, vmId)
		return common.EndRequestWithLog(c, reqID, err, result)

	default:
		result, err := mci.ListVmInfo(nsId, mciId, vmId)
		return common.EndRequestWithLog(c, reqID, err, result)
	}
}

/* RestPutMciVm function not yet implemented
// RestPutSshKey godoc
// @ID PutSshKey
// @Summary Update MCI
// @Description Update MCI
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param vmInfo body mci.TbVmInfo true "Details for an VM object"
// @Success 200 {object} mci.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId} [put]
func RestPutMciVm(c echo.Context) error {
	return nil
}
*/

// RestDelMciVm godoc
// @ID DelMciVm
// @Summary Delete VM in specified MCI
// @Description Delete VM in specified MCI
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string false "Option for delete VM (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
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

	err := mci.DelMciVm(nsId, mciId, vmId, option)
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
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param subgroupId path string true "subGroup ID" default(g1)
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
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

	content := common.IdList{}
	var err error
	content.IdList, err = mci.ListVmBySubGroup(nsId, mciId, subgroupId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetMciGroupIds godoc
// @ID GetMciGroupIds
// @Summary List SubGroup IDs in a specified MCI
// @Description List SubGroup IDs in a specified MCI
// @Tags [MC-Infra] MCI Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/subgroup [get]
func RestGetMciGroupIds(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	//option := c.QueryParam("option")

	content := common.IdList{}
	var err error
	content.IdList, err = mci.ListSubGroupId(nsId, mciId)
	return common.EndRequestWithLog(c, reqID, err, content)
}
