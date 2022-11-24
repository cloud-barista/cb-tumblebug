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

// RestPostInstallMonitorAgentToMcis godoc
// @Summary Install monitoring agent (CB-Dragonfly agent) to MCIS
// @Description Install monitoring agent (CB-Dragonfly agent) to MCIS
// @Tags [Infra service] MCIS Resource monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param mcisInfo body mcis.McisCmdReq true "Details for an MCIS object"
// @Success 200 {object} mcis.AgentInstallContentWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/monitoring/install/mcis/{mcisId} [post]
func RestPostInstallMonitorAgentToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	// mcisTmpSystemLabel := mcis.DefaultSystemLabel
	content, err := mcis.InstallMonitorAgentToMcis(nsId, mcisId, common.StrMCIS, req)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}

// RestGetMonitorData godoc
// @Summary Get monitoring data of specified MCIS for specified monitoring metric (cpu, memory, disk, network)
// @Description Get monitoring data of specified MCIS for specified monitoring metric (cpu, memory, disk, network)
// @Tags [Infra service] MCIS Resource monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param metric path string true "Metric type: cpu, memory, disk, network"
// @Success 200 {object} mcis.MonResultSimpleResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/monitoring/mcis/{mcisId}/metric/{metric} [get]
func RestGetMonitorData(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	metric := c.Param("metric")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.GetMonitoringData(nsId, mcisId, metric)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}
