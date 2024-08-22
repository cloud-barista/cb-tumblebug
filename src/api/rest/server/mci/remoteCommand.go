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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mci"
	"github.com/labstack/echo/v4"
)

// RestPostCmdMci godoc
// @ID PostCmdMci
// @Summary Send a command to specified MCI
// @Description Send a command to specified MCI
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param mciCmdReq body mci.MciCmdReq true "MCI Command Request"
// @Param subGroupId query string false "subGroupId to apply the command only for VMs in subGroup of MCI" default(g1)
// @Param vmId query string false "vmId to apply the command only for a VM in MCI" default(g1-1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} mci.MciSshCmdResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cmd/mci/{mciId} [post]
func RestPostCmdMci(c echo.Context) error {
	// reqID, idErr := common.StartRequestWithLog(c)
	// if idErr != nil {
	// 	return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	// }
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subGroupId := c.QueryParam("subGroupId")
	vmId := c.QueryParam("vmId")

	req := &mci.MciCmdReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	output, err := mci.RemoteCommandToMci(nsId, mciId, subGroupId, vmId, req)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	result := mci.MciSshCmdResult{}

	for _, v := range output {
		result.Results = append(result.Results, v)
	}

	common.PrintJsonPretty(result)

	return c.JSON(http.StatusOK, result)

	// return common.EndRequestWithLog(c, reqID, err, result)

}

// RestSetBastionNodes godoc
// @ID SetBastionNodes
// @Summary Set bastion nodes for a VM
// @Description Set bastion nodes for a VM
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{targetVmId}/bastion/{bastionVmId} [put]
func RestSetBastionNodes(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	targetVmId := c.Param("targetVmId")
	bastionVmId := c.Param("bastionVmId")

	content, err := mci.SetBastionNodes(nsId, mciId, targetVmId, bastionVmId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetBastionNodes godoc
// @ID GetBastionNodes
// @Summary Get bastion nodes for a VM
// @Description Get bastion nodes for a VM
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Success 200 {object} []mcir.BastionNode
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{targetVmId}/bastion [get]
func RestGetBastionNodes(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	targetVmId := c.Param("targetVmId")

	content, err := mci.GetBastionNodes(nsId, mciId, targetVmId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestRemoveBastionNodes godoc
// @ID RemoveBastionNodes
// @Summary Remove a bastion VM from all vNets
// @Description Remove a bastion VM from all vNets
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/bastion/{bastionVmId} [delete]
func RestRemoveBastionNodes(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	bastionVmId := c.Param("bastionVmId")

	content, err := mci.RemoveBastionNodes(nsId, mciId, bastionVmId)
	return common.EndRequestWithLog(c, reqID, err, content)
}
