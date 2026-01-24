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
	"io"
	"net/http"
	"strconv"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// convertSshCmdResultForAPI converts internal SshCmdResult to API-friendly format
func convertSshCmdResultForAPI(internal []model.SshCmdResult) model.MciSshCmdResultForAPI {
	apiResults := make([]model.SshCmdResultForAPI, len(internal))

	for i, result := range internal {
		apiResult := model.SshCmdResultForAPI{
			MciId:   result.MciId,
			VmId:    result.VmId,
			VmIp:    result.VmIp,
			Command: result.Command,
			Stdout:  result.Stdout,
			Stderr:  result.Stderr,
		}

		// Convert error to string for JSON serialization
		if result.Err != nil {
			apiResult.Error = result.Err.Error()
		}

		apiResults[i] = apiResult
	}

	return model.MciSshCmdResultForAPI{
		Results: apiResults,
	}
}

// RestPostCmdMci godoc
// @ID PostCmdMci
// @Summary Send a command to specified MCI
// @Description Send a command to specified MCI. Use query parameters to target specific subGroup or VM.
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param mciCmdReq body model.MciCmdReq true "MCI Command Request"
// @Param subGroupId query string false "subGroupId to apply the command only for VMs in subGroup of MCI" default(g1)
// @Param vmId query string false "vmId to apply the command only for a VM in MCI" default(g1-1)
// @Param labelSelector query string false "Target VM Label selector query. Example: sys.id=g1-1,role=worker"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.MciSshCmdResultForAPI
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/cmd/mci/{mciId} [post]
func RestPostCmdMci(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subGroupId := c.QueryParam("subGroupId")
	vmId := c.QueryParam("vmId")
	//Label selector query. Example: env=production,tier=backend
	labelSelector := c.QueryParam("labelSelector")

	// Get X-Request-ID header
	xRequestId := c.Request().Header.Get("X-Request-Id")
	if xRequestId == "" {
		xRequestId = c.Request().Header.Get("x-request-id") // fallback for lowercase
	}
	if xRequestId == "" {
		xRequestId = common.GenUid() // Generate if not provided
	}

	req := &model.MciCmdReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	output, err := infra.RemoteCommandToMci(nsId, mciId, subGroupId, vmId, labelSelector, req, xRequestId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert internal result to API-friendly format
	result := convertSshCmdResultForAPI(output)

	common.PrintJsonPretty(result)

	return c.JSON(http.StatusOK, result)

}

// RestPostFileToMci godoc
// @ID PostFileToMci
// @Summary Transfer a file to specified MCI
// @Description Transfer a file to specified MCI to the specified path.
// @Description The file size should be less than 10MB.
// @Description Not for gerneral file transfer but for specific purpose (small configuration files).
// @Tags [MC-Infra] MCI Remote Command
// @Accept  multipart/form-data
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param subGroupId query string false "subGroupId to apply the file transfer only for VMs in subGroup of MCI" default(g1)
// @Param vmId query string false "vmId to apply the file transfer only for a VM in MCI" default(g1-1)
// @Param path formData string true "Target path where the file will be stored" default(/home/cb-user/)
// @Param file formData file true "The file to be uploaded (Max 10MB)"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.MciSshCmdResultForAPI
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/transferFile/mci/{mciId} [post]
func RestPostFileToMci(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subGroupId := c.QueryParam("subGroupId")
	vmId := c.QueryParam("vmId")
	targetPath := c.FormValue("path")

	if targetPath == "" {
		err := fmt.Errorf("target path is required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Validate the file
	file, err := c.FormFile("file")
	if err != nil {
		err = fmt.Errorf("failed to read the file %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// File size validation
	fileSizeLimit := int64(10 * 1024 * 1024) // (10MB limit)
	if file.Size > fileSizeLimit {
		err := fmt.Errorf("file too large, max size is %v", fileSizeLimit)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Open the file and read it into memory
	src, err := file.Open()
	if err != nil {
		err = fmt.Errorf("failed to open the file %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	defer src.Close()

	// Read the file into memory
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		err = fmt.Errorf("failed to read the file %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Call the TransferFileToMci function
	output, err := infra.TransferFileToMci(nsId, mciId, subGroupId, vmId, fileBytes, file.Filename, targetPath)
	if err != nil {
		err = fmt.Errorf("failed to transfer file to mci %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert internal result to API-friendly format
	result := convertSshCmdResultForAPI(output)

	// Return the result
	return clientManager.EndRequestWithLog(c, nil, result)
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
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{targetVmId}/bastion/{bastionVmId} [put]
func RestSetBastionNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	targetVmId := c.Param("targetVmId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.SetBastionNodes(nsId, mciId, targetVmId, bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
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
// @Success 200 {object} []model.BastionNode
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{targetVmId}/bastion [get]
func RestGetBastionNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	targetVmId := c.Param("targetVmId")

	content, err := infra.GetBastionNodes(nsId, mciId, targetVmId)
	return clientManager.EndRequestWithLog(c, err, content)
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
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/bastion/{bastionVmId} [delete]
func RestRemoveBastionNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.RemoveBastionNodes(nsId, mciId, bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetVmCommandStatus godoc
// @ID GetVmCommandStatus
// @Summary Get a specific command status by index for a VM
// @Description Get a specific command status record by index for a VM
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param index path int true "Command Index" default(1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.CommandStatusInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index} [get]
func RestGetVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return clientManager.EndRequestWithLog(c, fmt.Errorf("invalid index parameter: %s", indexStr), nil)
	}

	commandStatus, err := infra.GetCommandStatusInfo(nsId, mciId, vmId, index)
	return clientManager.EndRequestWithLog(c, err, commandStatus)
}

// RestListVmCommandStatus godoc
// @ID ListVmCommandStatus
// @Summary List command status records for a VM with filtering
// @Description List command status records for a VM with various filtering options
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param status query []string false "Filter by command execution status (can specify multiple)" Enums(Queued,Handling,Completed,Failed,Timeout)
// @Param xRequestId query string false "Filter by X-Request-ID"
// @Param commandContains query string false "Filter commands containing this text"
// @Param startTimeFrom query string false "Filter commands started from this time (RFC3339 format)"
// @Param startTimeTo query string false "Filter commands started until this time (RFC3339 format)"
// @Param indexFrom query int false "Filter commands from this index (inclusive)"
// @Param indexTo query int false "Filter commands to this index (inclusive)"
// @Param limit query int false "Limit the number of results returned" default(50)
// @Param offset query int false "Number of results to skip" default(0)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.CommandStatusListResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus [get]
func RestListVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	// Parse query parameters for filtering
	filter := &model.CommandStatusFilter{}

	// Parse status array
	statusParams := c.QueryParams()["status"]
	if len(statusParams) > 0 {
		var statuses []model.CommandExecutionStatus
		for _, s := range statusParams {
			statuses = append(statuses, model.CommandExecutionStatus(s))
		}
		filter.Status = statuses
	}

	filter.XRequestId = c.QueryParam("xRequestId")
	filter.CommandContains = c.QueryParam("commandContains")
	filter.StartTimeFrom = c.QueryParam("startTimeFrom")
	filter.StartTimeTo = c.QueryParam("startTimeTo")

	if indexFromStr := c.QueryParam("indexFrom"); indexFromStr != "" {
		if indexFrom, err := strconv.Atoi(indexFromStr); err == nil {
			filter.IndexFrom = indexFrom
		}
	}

	if indexToStr := c.QueryParam("indexTo"); indexToStr != "" {
		if indexTo, err := strconv.Atoi(indexToStr); err == nil {
			filter.IndexTo = indexTo
		}
	}

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	} else {
		filter.Limit = 50 // Default limit
	}

	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	result, err := infra.ListCommandStatusInfo(nsId, mciId, vmId, filter)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestDeleteVmCommandStatus godoc
// @ID DeleteVmCommandStatus
// @Summary Delete a specific command status by index for a VM
// @Description Delete a specific command status record by index for a VM
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param index path int true "Command Index" default(1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index} [delete]
func RestDeleteVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return clientManager.EndRequestWithLog(c, fmt.Errorf("invalid index parameter: %s", indexStr), nil)
	}

	err = infra.DeleteCommandStatusInfo(nsId, mciId, vmId, index)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{Message: fmt.Sprintf("Command status with index %d deleted successfully", index)}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteVmCommandStatusByCriteria godoc
// @ID DeleteVmCommandStatusByCriteria
// @Summary Delete multiple command status records by criteria for a VM
// @Description Delete multiple command status records for a VM based on filtering criteria
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param status query []string false "Filter by command execution status (can specify multiple)" Enums(Queued,Handling,Completed,Failed,Timeout)
// @Param xRequestId query string false "Filter by X-Request-ID"
// @Param commandContains query string false "Filter commands containing this text"
// @Param startTimeFrom query string false "Filter commands started from this time (RFC3339 format)"
// @Param startTimeTo query string false "Filter commands started until this time (RFC3339 format)"
// @Param indexFrom query int false "Filter commands from this index (inclusive)"
// @Param indexTo query int false "Filter commands to this index (inclusive)"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus [delete]
func RestDeleteVmCommandStatusByCriteria(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	// Parse query parameters for filtering
	filter := &model.CommandStatusFilter{}

	// Parse status array
	statusParams := c.QueryParams()["status"]
	if len(statusParams) > 0 {
		var statuses []model.CommandExecutionStatus
		for _, s := range statusParams {
			statuses = append(statuses, model.CommandExecutionStatus(s))
		}
		filter.Status = statuses
	}

	filter.XRequestId = c.QueryParam("xRequestId")
	filter.CommandContains = c.QueryParam("commandContains")
	filter.StartTimeFrom = c.QueryParam("startTimeFrom")
	filter.StartTimeTo = c.QueryParam("startTimeTo")

	if indexFromStr := c.QueryParam("indexFrom"); indexFromStr != "" {
		if indexFrom, err := strconv.Atoi(indexFromStr); err == nil {
			filter.IndexFrom = indexFrom
		}
	}

	if indexToStr := c.QueryParam("indexTo"); indexToStr != "" {
		if indexTo, err := strconv.Atoi(indexToStr); err == nil {
			filter.IndexTo = indexTo
		}
	}

	deletedCount, err := infra.DeleteCommandStatusInfoByCriteria(nsId, mciId, vmId, filter)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{Message: fmt.Sprintf("Deleted %d command status records", deletedCount)}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestClearAllVmCommandStatus godoc
// @ID ClearAllVmCommandStatus
// @Summary Clear all command status records for a VM
// @Description Delete all command status records for a VM
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatusAll [delete]
func RestClearAllVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	deletedCount, err := infra.ClearAllCommandStatusInfo(nsId, mciId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{Message: fmt.Sprintf("Cleared %d command status records", deletedCount)}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetVmHandlingCommandCount godoc
// @ID GetVmHandlingCommandCount
// @Summary Get count of currently handling commands for a VM
// @Description Get the number of commands currently in 'Handling' status for a specific VM. Optimized for frequent polling.
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.HandlingCommandCountResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/handlingCount [get]
func RestGetVmHandlingCommandCount(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	handlingCount, err := infra.GetHandlingCommandCount(nsId, mciId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.HandlingCommandCountResponse{
		VmId:          vmId,
		HandlingCount: handlingCount,
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetMciHandlingCommandCount godoc
// @ID GetMciHandlingCommandCount
// @Summary Get count of currently handling commands for all VMs in MCI
// @Description Get the number of commands currently in 'Handling' status for all VMs in an MCI. Returns per-VM counts and total count.
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.MciHandlingCommandCountResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/handlingCount [get]
func RestGetMciHandlingCommandCount(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	vmHandlingCounts, totalHandlingCount, err := infra.GetMciHandlingCommandCount(nsId, mciId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.MciHandlingCommandCountResponse{
		MciId:              mciId,
		VmHandlingCounts:   vmHandlingCounts,
		TotalHandlingCount: totalHandlingCount,
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetVmSshHostKey godoc
// @ID GetVmSshHostKey
// @Summary Get SSH host key information for a VM
// @Description Get the stored SSH host key information for a specific VM. This is used for TOFU (Trust On First Use) verification.
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} model.SshHostKeyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/sshHostKey [get]
func RestGetVmSshHostKey(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	result, err := infra.GetVmSshHostKey(nsId, mciId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteVmSshHostKey godoc
// @ID DeleteVmSshHostKey
// @Summary Reset SSH host key for a VM
// @Description Reset the stored SSH host key for a specific VM. This should be used when the VM's host key has legitimately changed (e.g., after VM recreation) and you trust the new key. The next SSH connection will store the new host key (TOFU).
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/sshHostKey [delete]
func RestDeleteVmSshHostKey(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	err := infra.ResetVmSshHostKey(nsId, mciId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{
		Message: fmt.Sprintf("SSH host key for VM '%s' has been reset. The next SSH connection will store the new host key (TOFU).", vmId),
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetMciExecutionTasks godoc
// @ID GetMciExecutionTasks
// @Summary List execution tasks for an MCI
// @Description List all running and completed execution tasks for a specific MCI. These tasks can be cancelled if still in progress. The task list is based on persistent VM command status records.
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param status query string false "Filter by command status (Queued, Handling, Completed, Failed, Timeout, Cancelled, Interrupted). If not specified, returns all statuses." Enums(Queued, Handling, Completed, Failed, Timeout, Cancelled, Interrupted)
// @Success 200 {object} model.ExecutionTaskListResponse
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/cmd/mci/{mciId}/task [get]
func RestGetMciExecutionTasks(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	statusFilter := c.QueryParam("status")

	// Convert status filter
	var statusSlice []model.CommandExecutionStatus
	if statusFilter != "" {
		statusSlice = []model.CommandExecutionStatus{model.CommandExecutionStatus(statusFilter)}
	}
	// If no filter specified, statusSlice remains nil -> returns all statuses

	// Get tasks from persistent CommandStatusInfo (this is the source of truth)
	result, err := infra.GetMciActiveCommands(nsId, mciId, statusSlice)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetExecutionTask godoc
// @ID GetExecutionTask
// @Summary Get a specific execution task
// @Description Get detailed information about a specific execution task by taskId
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param taskId path string true "Task ID (format: xRequestId:vmId:index)"
// @Success 200 {object} model.ExecutionTaskListResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/cmd/mci/{mciId}/task/{taskId} [get]
func RestGetExecutionTask(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	taskId := c.Param("taskId")

	// Get all active commands and filter by taskId
	// Empty nsId/mciId will scan all namespaces/MCIs (for global route support)
	result, err := infra.GetMciActiveCommands(nsId, mciId, nil)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Filter tasks by taskId (exact match)
	var filteredTasks []model.ExecutionTask
	for _, task := range result.Tasks {
		if task.TaskId == taskId {
			filteredTasks = append(filteredTasks, task)
			break // TaskId is unique, no need to continue
		}
	}

	if len(filteredTasks) == 0 {
		return clientManager.EndRequestWithLog(c, fmt.Errorf("task not found: %s", taskId), nil)
	}

	return clientManager.EndRequestWithLog(c, nil, &model.ExecutionTaskListResponse{
		Tasks: filteredTasks,
		Total: len(filteredTasks),
	})
}

// RestCancelExecutionTask godoc
// @ID CancelExecutionTask
// @Summary Cancel an execution task
// @Description Cancel a running execution task by task ID. This will send a cancellation signal to the task and update the VM command status.
// @Tags [MC-Infra] MCI Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param taskId path string true "Task ID"
// @Param body body model.CancelTaskRequest false "Optional cancellation reason"
// @Success 200 {object} model.CancelTaskResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/cmd/mci/{mciId}/task/{taskId}/cancel [post]
func RestCancelExecutionTask(c echo.Context) error {
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	taskId := c.Param("taskId")

	// Parse optional cancel request body
	req := &model.CancelTaskRequest{}
	c.Bind(req) // Ignore error - body is optional

	// Find the task by taskId from task list
	taskList, err := infra.GetMciActiveCommands(nsId, mciId, nil)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Search for the task with matching taskId
	var targetTask *model.ExecutionTask
	for _, task := range taskList.Tasks {
		if task.TaskId == taskId {
			targetTask = &task
			break
		}
	}

	if targetTask == nil {
		return clientManager.EndRequestWithLog(c, fmt.Errorf("task not found: %s", taskId), nil)
	}

	// Cancel the task using the retrieved information from the task itself
	// Use targetTask.NsId and targetTask.MciId to support global route (/tumblebug/task/:taskId/cancel)
	response, err := infra.CancelMciCommand(targetTask.NsId, targetTask.MciId, targetTask.VmId, targetTask.XRequestId, targetTask.CommandIndex, req.Reason)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, response)
}
