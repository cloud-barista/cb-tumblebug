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
	"fmt"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestGetControlInfra godoc
// @ID GetControlInfra
// @Summary Control the lifecycle of Infra
// @Description Control the lifecycle of an Infra. Actions fall into three groups:
// @Description
// @Description **Lifecycle (normal operation):**
// @Description - `suspend` / `resume` / `reboot`: power-cycle every Node.
// @Description - `terminate`: terminate every Node (Infra metadata is kept; call DELETE to remove).
// @Description - `refine`: delete Nodes whose status is `Failed` or `Undefined` from Infra metadata
// @Description   (no CSP-side termination is issued for those Nodes).
// @Description
// @Description **Hold gate (only valid right after `POST /infra` with option=hold):**
// @Description - `continue`: signal the holding goroutine to proceed with provisioning.
// @Description - `withdraw`: signal the holding goroutine to cancel provisioning.
// @Description - These actions only work while a holding goroutine is alive in memory.
// @Description   After a server restart they will fail; use `reconcile`/`abort` instead.
// @Description
// @Description **Crash recovery (Infra stuck after server restart or partial failure):**
// @Description - `reconcile`: forward-recover. For each transient Node, query Spider for the real
// @Description   CSP status and absorb CSP-side orphan VMs (created before the crash but not
// @Description   recorded in TB). Nodes that cannot be matched on the CSP are marked `Failed`
// @Description   so a subsequent `refine` can remove them. No new Spider create calls are issued.
// @Description - `abort`: backward-recover. Force-terminate every non-final Node in parallel
// @Description   (with orphan rescue) and sweep any `Failed` remnants via `refine`. The final
// @Description   DELETE call is left to the operator.
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param action query string true "Action to apply to the Infra" Enums(suspend, resume, reboot, terminate, refine, continue, withdraw, reconcile, abort)
// @Param force query string false "Force control to skip checking controllable status" Enums(false, true)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/control/infra/{infraId} [get]
func RestGetControlInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	action := c.QueryParam("action")
	force := c.QueryParam("force")
	forceOption := false
	if force == "true" {
		forceOption = true
	}
	returnObj := model.SimpleMsg{}

	switch action {
	case "suspend", "resume", "reboot", "terminate", "refine",
		"continue", "withdraw", "reconcile", "abort":
		resultString, err := infra.HandleInfraAction(nsId, infraId, action, forceOption)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, returnObj)
		}
		returnObj.Message = resultString
		return clientManager.EndRequestWithLog(c, err, returnObj)
	default:
		err := fmt.Errorf("'action' should be one of these: suspend, resume, reboot, terminate, refine, continue, withdraw, reconcile, abort")
		return clientManager.EndRequestWithLog(c, err, returnObj)
	}
}

// RestGetControlInfraNode godoc
// @ID GetControlInfraNode
// @Summary Control the lifecycle of node (suspend, resume, reboot, terminate)
// @Description Control the lifecycle of node (suspend, resume, reboot, terminate)
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Param action query string true "Action to Infra" Enums(suspend, resume, reboot, terminate)
// @Param force query string false "Force control to skip checking controllable status" Enums(false, true)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/control/infra/{infraId}/node/{nodeId} [get]
func RestGetControlInfraNode(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")

	action := c.QueryParam("action")
	force := c.QueryParam("force")
	forceOption := false
	if force == "true" {
		forceOption = true
	}

	returnObj := model.SimpleMsg{}

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		resultString, err := infra.HandleInfraNodeAction(nsId, infraId, nodeId, action, forceOption)
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

// RestPostInfraNodeSnapshot godoc
// @ID PostInfraNodeSnapshot
// @Summary Snapshot node and create a Custom Image Object using the Snapshot
// @Description Snapshot node and create a Custom Image Object using the Snapshot
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param snapshotReq body model.SnapshotReq true "Request body to create node snapshot"
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Success 200 {object} model.ImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/node/{nodeId}/snapshot [post]
func RestPostInfraNodeSnapshot(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")

	req := &model.SnapshotReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := infra.CreateNodeSnapshot(nsId, infraId, nodeId, *req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "Failed to create a snapshot"})
	}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostInfraSnapshot godoc
// @ID PostInfraSnapshot
// @Summary Create snapshots for all nodegroups in Infra (one node per nodegroup in parallel)
// @Description Create snapshots for the first running node in each nodegroup of an Infra in parallel
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param snapshotReq body model.SnapshotReq true "Request body to create Infra snapshots"
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Success 200 {object} model.InfraSnapshotResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/snapshot [post]
func RestPostInfraSnapshot(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.SnapshotReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := infra.CreateInfraSnapshot(nsId, infraId, *req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "Failed to create Infra snapshots"})
	}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostBuildAgnosticImage godoc
// @ID PostBuildAgnosticImage
// @Summary Build agnostic custom images by creating Infra, executing commands, and taking snapshots
// @Description Creates an Infra infrastructure, executes post-deployment commands, creates snapshots from each nodegroup, and optionally cleans up the Infra. This is a complete workflow for building CSP-agnostic custom images.
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param buildReq body model.BuildAgnosticImageReq true "Request body to build agnostic images"
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.BuildAgnosticImageResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/buildAgnosticImage [post]
func RestPostBuildAgnosticImage(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.BuildAgnosticImageReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "Invalid request body"})
	}

	result, err := infra.BuildAgnosticImage(nsId, *req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "Failed to build agnostic images"})
	}
	return clientManager.EndRequestWithLog(c, err, result)
}
