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
)

// RestPostInstallMonitorAgentToMci godoc
// @ID PostInstallMonitorAgentToMci
// @Summary Install monitoring agent (CB-Dragonfly agent) to MCI
// @Description Install monitoring agent (CB-Dragonfly agent) to MCI
// @Tags [MC-Infra] MCI Resource Monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param mciInfo body model.MciCmdReq true "Details for an MCI object"
// @Success 200 {object} model.AgentInstallContentWrapper
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/monitoring/install/mci/{mciId} [post]
func RestPostInstallMonitorAgentToMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &model.MciCmdReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}
	// mciTmpSystemLabel := model.DefaultSystemLabel
	content, err := infra.InstallMonitorAgentToMci(nsId, mciId, model.StrMCI, req)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestPutMonitorAgentStatusInstalled godoc
// @ID PutMonitorAgentStatusInstalled
// @Summary Set monitoring agent (CB-Dragonfly agent) installation status installed (for Windows VM only)
// @Description Set monitoring agent (CB-Dragonfly agent) installation status installed (for Windows VM only)
// @Tags [MC-Infra] MCI Resource Monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(vm01)
// @Success 200 {object} model.TbVmInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/monitoring/status/mci/{mciId}/vm/{vmId} [put]
func RestPutMonitorAgentStatusInstalled(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	// mciTmpSystemLabel := model.DefaultSystemLabel
	err := infra.SetMonitoringAgentStatusInstalled(nsId, mciId, vmId)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	result, err := infra.ListVmInfo(nsId, mciId, vmId)
	return common.EndRequestWithLog(c, reqID, err, result)
}

// RestGetMonitorData godoc
// @ID GetMonitorData
// @Summary Get monitoring data of specified MCI for specified monitoring metric (cpu, memory, disk, network)
// @Description Get monitoring data of specified MCI for specified monitoring metric (cpu, memory, disk, network)
// @Tags [MC-Infra] MCI Resource Monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param metric path string true "Metric type: cpu, memory, disk, network"
// @Success 200 {object} model.MonResultSimpleResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/monitoring/mci/{mciId}/metric/{metric} [get]
func RestGetMonitorData(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	metric := c.Param("metric")

	req := &model.MciCmdReq{}
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := infra.GetMonitoringData(nsId, mciId, metric)
	return common.EndRequestWithLog(c, reqID, err, content)
}
