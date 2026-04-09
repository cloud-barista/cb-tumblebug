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

// Package infra is to handle REST API for infra operations
package infra

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestPostVpnHealthCheck godoc
// @ID PostVpnHealthCheck
// @Summary Check the health of a site-to-site VPN by bidirectional ping test
// @Description Perform a bidirectional ping test on a site-to-site VPN using existing MCI VMs and return the results.
// @Description
// @Description It finds VMs that belong to the VPN's two sites and runs ping tests
// @Description in both directions (site1→site2 and site2→site1) via private IP.
// @Description The VPN is considered healthy only when both directions succeed.
// @Description
// @Description A retry strategy is used with configurable interval and max attempts
// @Description (default: 15s interval, 20 attempts).
// @Tags [Infra Resource] Site-to-site VPN Management (preview)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param healthCheckReq body model.VpnHealthCheckRequest true "Health check options"
// @Success 200 {object} model.VpnHealthCheckResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/mci/{mciId}/vpn/{vpnId}/health [post]
func RestPostVpnHealthCheck(c echo.Context) error {

	nsId := c.Param("nsId")
	if err := common.CheckString(nsId); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: fmt.Sprintf("invalid nsId (%s)", nsId)})
	}
	mciId := c.Param("mciId")
	if err := common.CheckString(mciId); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: fmt.Sprintf("invalid mciId (%s)", mciId)})
	}
	vpnId := c.Param("vpnId")
	if err := common.CheckString(vpnId); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: fmt.Sprintf("invalid vpnId (%s)", vpnId)})
	}

	// Bind request body
	req := new(model.VpnHealthCheckRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Delegate to core function
	resp, err := infra.CheckVpnHealth(c.Request().Context(), nsId, mciId, vpnId, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}
