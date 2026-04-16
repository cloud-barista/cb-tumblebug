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

// Package infra is to handle REST API for infra
package infra

import (
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestPostInstallMonitorAgentToInfra godoc
// @ID PostInstallMonitorAgentToInfra
// @Summary Install monitoring agent (CB-Dragonfly agent) to Infra
// @Description Install monitoring agent (CB-Dragonfly agent) to Infra
// @Tags [MC-Infra] Infra Resource Monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param infraInfo body model.InfraCmdReq true "Details for an Infra object"
// @Success 200 {object} model.AgentInstallContentWrapper
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/monitoring/install/infra/{infraId} [post]
func RestPostInstallMonitorAgentToInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.InfraCmdReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	// infraTmpSystemLabel := model.DefaultSystemLabel
	content, err := infra.InstallMonitorAgentToInfra(nsId, infraId, model.StrInfra, req)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPutMonitorAgentStatusInstalled godoc
// @ID PutMonitorAgentStatusInstalled
// @Summary Set monitoring agent (CB-Dragonfly agent) installation status installed (for Windows node only)
// @Description Set monitoring agent (CB-Dragonfly agent) installation status installed (for Windows node only)
// @Tags [MC-Infra] Infra Resource Monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(node01)
// @Success 200 {object} model.NodeInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/monitoring/status/infra/{infraId}/node/{nodeId} [put]
func RestPutMonitorAgentStatusInstalled(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")

	// infraTmpSystemLabel := model.DefaultSystemLabel
	err := infra.SetMonitoringAgentStatusInstalled(nsId, infraId, nodeId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.GetNodeObject(nsId, infraId, nodeId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetMonitorData godoc
// @ID GetMonitorData
// @Summary Get monitoring data of specified Infra for specified monitoring metric (cpu, memory, disk, network)
// @Description Get monitoring data of specified Infra for specified monitoring metric (cpu, memory, disk, network)
// @Tags [MC-Infra] Infra Resource Monitor (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param metric path string true "Metric type: cpu, memory, disk, network"
// @Success 200 {object} model.MonResultSimpleResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/monitoring/infra/{infraId}/metric/{metric} [get]
func RestGetMonitorData(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	metric := c.Param("metric")

	req := &model.InfraCmdReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.GetMonitoringData(nsId, infraId, metric)
	return clientManager.EndRequestWithLog(c, err, content)
}
