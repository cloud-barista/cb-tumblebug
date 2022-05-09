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

// RestPostConfigureCloudAdaptiveNetworkToMcis godoc
// @Summary Configure Cloud Adaptive Network (cb-network agent) to MCIS
// @Description Configure Cloud Adaptive Network (cb-network agent) to MCIS
// @Tags [Infra service] MCIS Cloud Adaptive Network (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param networkReq body mcis.NetworkReq true "Details for the network request body"
// @Success 200 {object} mcis.AgentInstallContentWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/network/mcis/{mcisId} [post]
func RestPostConfigureCloudAdaptiveNetworkToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	netReq := &mcis.NetworkReq{}
	if err := c.Bind(netReq); err != nil {
		return err
	}

	contents, err := mcis.ConfigureCloudAdaptiveNetwork(nsId, mcisId, netReq)

	if err != nil {
		common.CBLog.Error(err)
		return err

	}

	return c.JSON(http.StatusOK, contents)
}

// RestPostInjectCloudInformationForCloudAdaptiveNetwork godoc
// @Summary Inject Cloud Information For Cloud Adaptive Network
// @Description Inject Cloud Information For Cloud Adaptive Network
// @Tags [Infra service] MCIS Cloud Adaptive Network (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param networkReq body mcis.NetworkReq true "Details for the network request body"
// @Success 200 {object} mcis.AgentInstallContentWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/network/mcis/{mcisId} [put]
func RestPutInjectCloudInformationForCloudAdaptiveNetwork(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	netReq := &mcis.NetworkReq{}
	if err := c.Bind(netReq); err != nil {
		return err
	}

	contents, err := mcis.InjectCloudInformationForCloudAdaptiveNetwork(nsId, mcisId, netReq)

	if err != nil {
		common.CBLog.Error(err)
		return err

	}

	return c.JSON(http.StatusOK, contents)
}
