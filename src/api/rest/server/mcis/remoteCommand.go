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
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostCmdMcis godoc
// @Summary Send a command to specified MCIS
// @Description Send a command to specified MCIS
// @Tags [Infra service] MCIS Remote command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param mcisCmdReq body mcis.McisCmdReq true "MCIS Command Request"
// @Param subGroupId query string false "subGroupId to apply the command only for VMs in subGroup of MCIS" default(g1)
// @Param vmId query string false "vmId to apply the command only for a VM in MCIS" default(g1-1)
// @Success 200 {object} mcis.McisSshCmdResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cmd/mcis/{mcisId} [post]
func RestPostCmdMcis(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	subGroupId := c.QueryParam("subGroupId")
	vmId := c.QueryParam("vmId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	resultArray, err := mcis.RemoteCommandToMcis(nsId, mcisId, subGroupId, vmId, req)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content := mcis.McisSshCmdResult{}

	for _, v := range resultArray {
		content.Results = append(content.Results, v)
	}

	common.PrintJsonPretty(content)

	return common.EndRequestWithLog(c, reqID, nil, content)

}

// RestSetBastionNodes godoc
// @Summary Set bastion nodes for a VM
// @Description Set bastion nodes for a VM
// @Tags [Infra service] MCIS Remote command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{targetVmId}/bastion/{bastionVmId} [put]
func RestSetBastionNodes(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	targetVmId := c.Param("targetVmId")
	bastionVmId := c.Param("bastionVmId")

	content, err := mcis.SetBastionNodes(nsId, mcisId, targetVmId, bastionVmId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetBastionNodes godoc
// @Summary Get bastion nodes for a VM
// @Description Get bastion nodes for a VM
// @Tags [Infra service] MCIS Remote command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Success 200 {object} []mcir.BastionNode
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{targetVmId}/bastion [get]
func RestGetBastionNodes(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	targetVmId := c.Param("targetVmId")

	content, err := mcis.GetBastionNodes(nsId, mcisId, targetVmId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestRemoveBastionNodes godoc
// @Summary Remove a bastion VM from all vNets
// @Description Remove a bastion VM from all vNets
// @Tags [Infra service] MCIS Remote command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/bastion/{bastionVmId} [delete]
func RestRemoveBastionNodes(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	bastionVmId := c.Param("bastionVmId")

	content, err := mcis.RemoveBastionNodes(nsId, mcisId, bastionVmId)
	return common.EndRequestWithLog(c, reqID, err, content)
}
