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
)

// RestGetControlMci godoc
// @ID GetControlMci
// @Summary Control the lifecycle of MCI (refine, suspend, resume, reboot, terminate)
// @Description Control the lifecycle of MCI (refine, suspend, resume, reboot, terminate)
// @Tags [MC-Infra] MCI Control lifecycle
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param action query string true "Action to MCI" Enums(suspend, resume, reboot, terminate, refine, continue, withdraw)
// @Param force query string false "Force control to skip checking controllable status" Enums(false, true)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/control/mci/{mciId} [get]
func RestGetControlMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	action := c.QueryParam("action")
	force := c.QueryParam("force")
	forceOption := false
	if force == "true" {
		forceOption = true
	}
	returnObj := common.SimpleMsg{}

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" || action == "refine" || action == "continue" || action == "withdraw" {

		resultString, err := mci.HandleMciAction(nsId, mciId, action, forceOption)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, returnObj)
		}
		returnObj.Message = resultString
		return common.EndRequestWithLog(c, reqID, err, returnObj)

	} else {
		err := fmt.Errorf("'action' should be one of these: suspend, resume, reboot, terminate, refine, continue, withdraw")
		return common.EndRequestWithLog(c, reqID, err, returnObj)
	}
}

// RestGetControlMciVm godoc
// @ID GetControlMciVm
// @Summary Control the lifecycle of VM (suspend, resume, reboot, terminate)
// @Description Control the lifecycle of VM (suspend, resume, reboot, terminate)
// @Tags [MC-Infra] MCI Control lifecycle
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param action query string true "Action to MCI" Enums(suspend, resume, reboot, terminate)
// @Param force query string false "Force control to skip checking controllable status" Enums(false, true)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/control/mci/{mciId}/vm/{vmId} [get]
func RestGetControlMciVm(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")
	force := c.QueryParam("force")
	forceOption := false
	if force == "true" {
		forceOption = true
	}

	returnObj := common.SimpleMsg{}

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		resultString, err := mci.HandleMciVmAction(nsId, mciId, vmId, action, forceOption)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, returnObj)
		}
		returnObj.Message = resultString
		return common.EndRequestWithLog(c, reqID, err, returnObj)

	} else {
		err := fmt.Errorf("'action' should be one of these: suspend, resume, reboot, terminate, refine")
		return common.EndRequestWithLog(c, reqID, err, returnObj)
	}
}

// RestPostMciVmSnapshot godoc
// @ID PostMciVmSnapshot
// @Summary Snapshot VM and create a Custom Image Object using the Snapshot
// @Description Snapshot VM and create a Custom Image Object using the Snapshot
// @Tags [Infra resource] Snapshot and Custom Image Management
// @Accept  json
// @Produce  json
// @Param vmSnapshotReq body mci.TbVmSnapshotReq true "Request body to create VM snapshot"
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} mcir.TbCustomImageInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/snapshot [post]
func RestPostMciVmSnapshot(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	u := &mci.TbVmSnapshotReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	result, err := mci.CreateVmSnapshot(nsId, mciId, vmId, u.Name)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, common.SimpleMsg{Message: "Failed to create a snapshot"})
	}
	return common.EndRequestWithLog(c, reqID, err, result)
}
