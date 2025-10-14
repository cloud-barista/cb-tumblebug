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

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestGetControlMci godoc
// @ID GetControlMci
// @Summary Control the lifecycle of MCI (refine, suspend, resume, reboot, terminate)
// @Description Control the lifecycle of MCI (refine, suspend, resume, reboot, terminate)
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param action query string true "Action to MCI" Enums(suspend, resume, reboot, terminate, refine, continue, withdraw)
// @Param force query string false "Force control to skip checking controllable status" Enums(false, true)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/control/mci/{mciId} [get]
func RestGetControlMci(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	action := c.QueryParam("action")
	force := c.QueryParam("force")
	forceOption := false
	if force == "true" {
		forceOption = true
	}
	returnObj := model.SimpleMsg{}

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" || action == "refine" || action == "continue" || action == "withdraw" {

		resultString, err := infra.HandleMciAction(nsId, mciId, action, forceOption)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, returnObj)
		}
		returnObj.Message = resultString
		return clientManager.EndRequestWithLog(c, err, returnObj)

	} else {
		err := fmt.Errorf("'action' should be one of these: suspend, resume, reboot, terminate, refine, continue, withdraw")
		return clientManager.EndRequestWithLog(c, err, returnObj)
	}
}

// RestGetControlMciVm godoc
// @ID GetControlMciVm
// @Summary Control the lifecycle of VM (suspend, resume, reboot, terminate)
// @Description Control the lifecycle of VM (suspend, resume, reboot, terminate)
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param action query string true "Action to MCI" Enums(suspend, resume, reboot, terminate)
// @Param force query string false "Force control to skip checking controllable status" Enums(false, true)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/control/mci/{mciId}/vm/{vmId} [get]
func RestGetControlMciVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")
	force := c.QueryParam("force")
	forceOption := false
	if force == "true" {
		forceOption = true
	}

	returnObj := model.SimpleMsg{}

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		resultString, err := infra.HandleMciVmAction(nsId, mciId, vmId, action, forceOption)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, returnObj)
		}
		returnObj.Message = resultString
		return clientManager.EndRequestWithLog(c, err, returnObj)

	} else {
		err := fmt.Errorf("'action' should be one of these: suspend, resume, reboot, terminate, refine")
		return clientManager.EndRequestWithLog(c, err, returnObj)
	}
}

// RestPostMciVmSnapshot godoc
// @ID PostMciVmSnapshot
// @Summary Snapshot VM and create a Custom Image Object using the Snapshot
// @Description Snapshot VM and create a Custom Image Object using the Snapshot
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param snapshotReq body model.SnapshotReq true "Request body to create VM snapshot"
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} model.ImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/snapshot [post]
func RestPostMciVmSnapshot(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	req := &model.SnapshotReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := infra.CreateVmSnapshot(nsId, mciId, vmId, *req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "Failed to create a snapshot"})
	}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciSnapshot godoc
// @ID PostMciSnapshot
// @Summary Create snapshots for all subgroups in MCI (one VM per subgroup in parallel)
// @Description Create snapshots for the first running VM in each subgroup of an MCI in parallel
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param snapshotReq body model.SnapshotReq true "Request body to create MCI snapshots"
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Success 200 {object} model.MciSnapshotResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/snapshot [post]
func RestPostMciSnapshot(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &model.SnapshotReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := infra.CreateMciSnapshot(nsId, mciId, *req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "Failed to create MCI snapshots"})
	}
	return clientManager.EndRequestWithLog(c, err, result)
}
