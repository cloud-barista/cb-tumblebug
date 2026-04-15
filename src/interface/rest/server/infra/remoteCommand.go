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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// convertSshCmdResultForAPI converts internal SshCmdResult to API-friendly format
func convertSshCmdResultForAPI(internal []model.SshCmdResult) model.InfraSshCmdResultForAPI {
	apiResults := make([]model.SshCmdResultForAPI, len(internal))

	for i, result := range internal {
		apiResult := model.SshCmdResultForAPI{
			InfraId: result.InfraId,
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

	return model.InfraSshCmdResultForAPI{
		Results: apiResults,
	}
}

// RestPostCmdInfra godoc
// @ID PostCmdInfra
// @Summary Send a command to specified Infra
// @Description Send a command to specified Infra. Use query parameters to target specific nodeGroup or VM.
// @Description When async=true, returns immediately with xRequestId and streams results via SSE at GET /stream/ns/{nsId}/cmd/infra/{infraId}?xRequestId={xRequestId}
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param infraCmdReq body model.InfraCmdReq true "Infra Command Request"
// @Param nodeGroupId query string false "nodeGroupId to apply the command only for VMs in nodeGroup of Infra" default(g1)
// @Param vmId query string false "vmId to apply the command only for a VM in Infra" default(g1-1)
// @Param labelSelector query string false "Target VM Label selector query. Example: sys.id=g1-1,role=worker"
// @Param async query string false "If true, execute asynchronously and return xRequestId for SSE streaming" default(false)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.InfraSshCmdResultForAPI
// @Success 202 {object} map[string]string "Async mode: returns xRequestId"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/cmd/infra/{infraId} [post]
func RestPostCmdInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeGroupId := c.QueryParam("nodeGroupId")
	vmId := c.QueryParam("vmId")
	asyncMode := c.QueryParam("async") == "true"
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

	req := &model.InfraCmdReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if asyncMode {
		// Async mode: launch execution in background and return xRequestId immediately
		go func() {
			_, err := infra.RemoteCommandToInfra(nsId, infraId, nodeGroupId, vmId, labelSelector, req, xRequestId)
			if err != nil {
				log.Error().Err(err).Str("xRequestId", xRequestId).Msg("Async remote command execution failed")

				// Small delay to give SSE clients time to connect before publishing the terminal event.
				// Without this, the CommandDone event may be published before the client subscribes,
				// and while the ring buffer should replay it, this avoids timing edge cases.
				time.Sleep(500 * time.Millisecond)

				// Publish a CommandDone event with error info so SSE clients don't hang forever
				log.Info().Str("xRequestId", xRequestId).Msg("Publishing CommandDone (error) event for SSE subscribers")
				infra.PublishCommandEvent(xRequestId, model.CommandStreamEvent{
					Type:      model.EventCommandDone,
					Timestamp: time.Now().Format(time.RFC3339Nano),
					Summary: &model.CommandDoneSummary{
						TotalVms:       0,
						CompletedVms:   0,
						FailedVms:      0,
						ElapsedSeconds: 0,
						Error:          err.Error(),
					},
				})
			}
		}()

		c.Response().Header().Set("X-Request-Id", xRequestId)
		return c.JSON(http.StatusAccepted, map[string]string{
			"xRequestId": xRequestId,
			"message":    "Command execution started. Use GET /tumblebug/ns/{nsId}/stream/cmd/infra/{infraId}?xRequestId={xRequestId} for real-time streaming.",
		})
	}

	// Sync mode (default): execute and wait for result
	output, err := infra.RemoteCommandToInfra(nsId, infraId, nodeGroupId, vmId, labelSelector, req, xRequestId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert internal result to API-friendly format
	result := convertSshCmdResultForAPI(output)

	common.PrintJsonPretty(result)

	return c.JSON(http.StatusOK, result)

}

// RestPostFileToInfra godoc
// @ID PostFileToInfra
// @Summary Transfer a file to specified Infra
// @Description Transfer a file to specified Infra to the specified path.
// @Description The file size should be less than 10MB.
// @Description Not for gerneral file transfer but for specific purpose (small configuration files).
// @Tags [MC-Infra] Infra Remote Command
// @Accept  multipart/form-data
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeGroupId query string false "nodeGroupId to apply the file transfer only for VMs in nodeGroup of Infra" default(g1)
// @Param vmId query string false "vmId to apply the file transfer only for a VM in Infra" default(g1-1)
// @Param path formData string true "Target path where the file will be stored" default(/home/cb-user/)
// @Param file formData file true "The file to be uploaded (Max 10MB)"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.InfraSshCmdResultForAPI
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/transferFile/infra/{infraId} [post]
func RestPostFileToInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeGroupId := c.QueryParam("nodeGroupId")
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
	fileSizeLimit := int64(50 * 1024 * 1024) // (50MB limit)
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

	// Call the TransferFileToInfra function
	output, err := infra.TransferFileToInfra(nsId, infraId, nodeGroupId, vmId, fileBytes, file.Filename, targetPath)
	if err != nil {
		err = fmt.Errorf("failed to transfer file to infra %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert internal result to API-friendly format
	result := convertSshCmdResultForAPI(output)

	// Return the result
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestPostFileAndCmdToInfra godoc
// @ID PostFileAndCmdToInfra
// @Summary Transfer a file to Infra and optionally execute a command after transfer
// @Description Transfer a file to all targeted VMs in Infra via SCP, then optionally run a shell command on each VM where the transfer succeeded.
// @Description Useful for deploying files directly to privileged locations (e.g., nginx document root) in a single API call.
// @Description Example: upload index.html to /tmp and run "sudo mv /tmp/index.html /var/www/html/" as the post-transfer command.
// @Description The file size should be less than 50MB.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  multipart/form-data
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeGroupId query string false "NodeGroup ID to limit file transfer scope to VMs in a nodeGroup"
// @Param vmId query string false "VM ID to limit file transfer scope to a single VM"
// @Param path formData string true "Target directory path on the VM where the file will be stored" default(/tmp)
// @Param file formData file true "The file to be uploaded (Max 50MB)"
// @Param command formData string false "Shell command to execute on each VM after successful file transfer (e.g., sudo mv /tmp/index.html /var/www/html/)"
// @Param x-request-id header string false "Custom request ID"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Success 200 {object} model.InfraFileTransferAndCmdResultForAPI
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/transferFileAndCmd/infra/{infraId} [post]
func RestPostFileAndCmdToInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeGroupId := c.QueryParam("nodeGroupId")
	vmId := c.QueryParam("vmId")
	targetPath := c.FormValue("path")
	command := c.FormValue("command")

	if targetPath == "" {
		err := fmt.Errorf("target path is required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	file, err := c.FormFile("file")
	if err != nil {
		err = fmt.Errorf("failed to read the file: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	fileSizeLimit := int64(50 * 1024 * 1024) // 50MB
	if file.Size > fileSizeLimit {
		err := fmt.Errorf("file too large, max size is %v bytes", fileSizeLimit)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	src, err := file.Open()
	if err != nil {
		err = fmt.Errorf("failed to open the file: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	defer src.Close()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		err = fmt.Errorf("failed to read the file: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	output, err := infra.TransferFileAndCmdToInfra(nsId, infraId, nodeGroupId, vmId, fileBytes, file.Filename, targetPath, command)
	if err != nil {
		err = fmt.Errorf("failed to transfer file to infra: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	apiResult := model.InfraFileTransferAndCmdResultForAPI{
		FileTransferResults: convertSshCmdResultForAPI(output.FileTransferResults),
	}
	if len(output.CmdResults) > 0 {
		cmdResult := convertSshCmdResultForAPI(output.CmdResults)
		apiResult.CmdResults = &cmdResult
	}

	return clientManager.EndRequestWithLog(c, nil, apiResult)
}

// RestPostDownloadFileFromInfraVm godoc
// @ID PostDownloadFileFromInfraVm
// @Summary Download a file from a VM in Infra
// @Description Download a file from a specific VM in Infra via SCP through bastion host.
// @Description The file size should be less than 200MB.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  application/octet-stream,json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param fileDownloadReq body model.FileDownloadReq true "File download request"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {file} file "Downloaded file"
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/downloadFile/infra/{infraId}/vm/{vmId} [post]
func RestPostDownloadFileFromInfraVm(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	req := &model.FileDownloadReq{}
	if err := c.Bind(req); err != nil {
		err = fmt.Errorf("invalid request body: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if req.SourcePath == "" {
		err := fmt.Errorf("sourcePath is required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Download the file from the VM
	fileData, fileName, err := infra.DownloadFileFromInfraVm(nsId, infraId, vmId, req.SourcePath)
	if err != nil {
		err = fmt.Errorf("failed to download file from VM: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Sanitize fileName to prevent header injection
	safeFileName := path.Base(fileName)
	safeFileName = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '\r' || r == '\n' {
			return '_'
		}
		return r
	}, safeFileName)
	if safeFileName == "" || safeFileName == "." {
		safeFileName = "downloaded_file"
	}

	// Set response headers for file download
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeFileName))
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))

	log.Info().Msgf("Sending downloaded file: %s (%d bytes) from VM %s", fileName, len(fileData), vmId)

	return c.Blob(http.StatusOK, "application/octet-stream", fileData)
}

// RestSetBastionNodes godoc
// @ID SetBastionNodes
// @Summary Set bastion nodes for a VM
// @Description Set bastion nodes for a VM
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{targetVmId}/bastion/{bastionVmId} [put]
func RestSetBastionNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	targetVmId := c.Param("targetVmId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.SetBastionNodes(nsId, infraId, targetVmId, "", "", bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestSetBastionNodesWithInfra godoc
// @ID SetBastionNodesWithInfra
// @Summary Set bastion nodes for a VM using a bastion from another Infra (same namespace)
// @Description Set bastion nodes for a target VM, specifying a bastion VM that belongs to a different Infra within the same namespace (cross-Infra bastion). This allows, for example, an AWS VM to serve as a bastion for an OpenStack VM.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Target Infra ID" default(infra01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Param bastionInfraId path string true "Bastion Infra ID (may differ from target Infra)" default(infra-bastion)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{targetVmId}/bastion/{bastionInfraId}/{bastionVmId} [put]
func RestSetBastionNodesWithInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	targetVmId := c.Param("targetVmId")
	bastionInfraId := c.Param("bastionInfraId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.SetBastionNodes(nsId, infraId, targetVmId, "", bastionInfraId, bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestSetBastionNodesWithNs godoc
// @ID SetBastionNodesWithNs
// @Summary Set bastion nodes for a VM using a bastion from a different namespace and Infra
// @Description Set bastion nodes for a target VM, specifying a bastion VM that belongs to a different namespace and Infra (cross-namespace bastion). This allows, for example, a VM in a shared-services namespace to act as a bastion for VMs in other namespaces.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Target Namespace ID" default(default)
// @Param infraId path string true "Target Infra ID" default(infra01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Param bastionNsId path string true "Bastion Namespace ID (may differ from target namespace)" default(ns-bastion)
// @Param bastionInfraId path string true "Bastion Infra ID" default(infra-bastion)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{targetVmId}/bastion/{bastionNsId}/{bastionInfraId}/{bastionVmId} [put]
func RestSetBastionNodesWithNs(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	targetVmId := c.Param("targetVmId")
	bastionNsId := c.Param("bastionNsId")
	bastionInfraId := c.Param("bastionInfraId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.SetBastionNodes(nsId, infraId, targetVmId, bastionNsId, bastionInfraId, bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetBastionNodes godoc
// @ID GetBastionNodes
// @Summary Get bastion nodes for a VM
// @Description Get bastion nodes for a VM
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param targetVmId path string true "Target VM ID" default(g1-1)
// @Success 200 {object} []model.BastionNode
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{targetVmId}/bastion [get]
func RestGetBastionNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	targetVmId := c.Param("targetVmId")

	content, err := infra.GetBastionNodes(nsId, infraId, targetVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestRemoveBastionNodes godoc
// @ID RemoveBastionNodes
// @Summary Remove a bastion VM from all vNets
// @Description Remove a bastion VM from all vNets
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param bastionVmId path string true "Bastion VM ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/bastion/{bastionVmId} [delete]
func RestRemoveBastionNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.RemoveBastionNodes(nsId, infraId, "", "", bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestRemoveBastionNodesWithInfra godoc
// @ID RemoveBastionNodesWithInfra
// @Summary Remove a bastion VM (cross-Infra) from all vNets
// @Description Remove a specific cross-Infra bastion from all vNets of the target Infra
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Target Infra ID" default(infra01)
// @Param bastionInfraId path string true "Bastion Infra ID"
// @Param bastionVmId path string true "Bastion VM ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/bastion/{bastionInfraId}/{bastionVmId} [delete]
func RestRemoveBastionNodesWithInfra(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	bastionInfraId := c.Param("bastionInfraId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.RemoveBastionNodes(nsId, infraId, "", bastionInfraId, bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestRemoveBastionNodesWithNs godoc
// @ID RemoveBastionNodesWithNs
// @Summary Remove a bastion VM (cross-namespace) from all vNets
// @Description Remove a specific cross-namespace bastion from all vNets of the target Infra
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Target Infra ID" default(infra01)
// @Param bastionNsId path string true "Bastion Namespace ID"
// @Param bastionInfraId path string true "Bastion Infra ID"
// @Param bastionVmId path string true "Bastion VM ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/bastion/{bastionNsId}/{bastionInfraId}/{bastionVmId} [delete]
func RestRemoveBastionNodesWithNs(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	bastionNsId := c.Param("bastionNsId")
	bastionInfraId := c.Param("bastionInfraId")
	bastionVmId := c.Param("bastionVmId")

	content, err := infra.RemoveBastionNodes(nsId, infraId, bastionNsId, bastionInfraId, bastionVmId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetVmCommandStatus godoc
// @ID GetVmCommandStatus
// @Summary Get a specific command status by index for a VM
// @Description Get a specific command status record by index for a VM
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param index path int true "Command Index" default(1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.CommandStatusInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/commandStatus/{index} [get]
func RestGetVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return clientManager.EndRequestWithLog(c, fmt.Errorf("invalid index parameter: %s", indexStr), nil)
	}

	commandStatus, err := infra.GetCommandStatusInfo(nsId, infraId, vmId, index)
	return clientManager.EndRequestWithLog(c, err, commandStatus)
}

// RestListVmCommandStatus godoc
// @ID ListVmCommandStatus
// @Summary List command status records for a VM with filtering
// @Description List command status records for a VM with various filtering options
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
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
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/commandStatus [get]
func RestListVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
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

	result, err := infra.ListCommandStatusInfo(nsId, infraId, vmId, filter)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestDeleteVmCommandStatus godoc
// @ID DeleteVmCommandStatus
// @Summary Delete a specific command status by index for a VM
// @Description Delete a specific command status record by index for a VM
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param index path int true "Command Index" default(1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/commandStatus/{index} [delete]
func RestDeleteVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return clientManager.EndRequestWithLog(c, fmt.Errorf("invalid index parameter: %s", indexStr), nil)
	}

	err = infra.DeleteCommandStatusInfo(nsId, infraId, vmId, index)
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
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
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
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/commandStatus [delete]
func RestDeleteVmCommandStatusByCriteria(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
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

	deletedCount, err := infra.DeleteCommandStatusInfoByCriteria(nsId, infraId, vmId, filter)
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
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/commandStatusAll [delete]
func RestClearAllVmCommandStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	deletedCount, err := infra.ClearAllCommandStatusInfo(nsId, infraId, vmId)
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
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.HandlingCommandCountResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/handlingCount [get]
func RestGetVmHandlingCommandCount(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	handlingCount, err := infra.GetHandlingCommandCount(nsId, infraId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.HandlingCommandCountResponse{
		VmId:          vmId,
		HandlingCount: handlingCount,
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetInfraHandlingCommandCount godoc
// @ID GetInfraHandlingCommandCount
// @Summary Get count of currently handling commands for all VMs in Infra
// @Description Get the number of commands currently in 'Handling' status for all VMs in an Infra. Returns per-VM counts and total count.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.InfraHandlingCommandCountResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/handlingCount [get]
func RestGetInfraHandlingCommandCount(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	vmHandlingCounts, totalHandlingCount, err := infra.GetInfraHandlingCommandCount(nsId, infraId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.InfraHandlingCommandCountResponse{
		InfraId:            infraId,
		VmHandlingCounts:   vmHandlingCounts,
		TotalHandlingCount: totalHandlingCount,
	}
	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetVmSshHostKey godoc
// @ID GetVmSshHostKey
// @Summary Get SSH host key information for a VM
// @Description Get the stored SSH host key information for a specific VM. This is used for TOFU (Trust On First Use) verification.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} model.SshHostKeyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/sshHostKey [get]
func RestGetVmSshHostKey(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	result, err := infra.GetVmSshHostKey(nsId, infraId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteVmSshHostKey godoc
// @ID DeleteVmSshHostKey
// @Summary Reset SSH host key for a VM
// @Description Reset the stored SSH host key for a specific VM. This should be used when the VM's host key has legitimately changed (e.g., after VM recreation) and you trust the new key. The next SSH connection will store the new host key (TOFU).
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/vm/{vmId}/sshHostKey [delete]
func RestDeleteVmSshHostKey(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	vmId := c.Param("vmId")

	err := infra.ResetVmSshHostKey(nsId, infraId, vmId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{
		Message: fmt.Sprintf("SSH host key for VM '%s' has been reset. The next SSH connection will store the new host key (TOFU).", vmId),
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetInfraExecutionTasks godoc
// @ID GetInfraExecutionTasks
// @Summary List execution tasks for an Infra
// @Description List all running and completed execution tasks for a specific Infra. These tasks can be cancelled if still in progress. The task list is based on persistent VM command status records.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param status query string false "Filter by command status (Queued, Handling, Completed, Failed, Timeout, Cancelled, Interrupted). If not specified, returns all statuses." Enums(Queued, Handling, Completed, Failed, Timeout, Cancelled, Interrupted)
// @Success 200 {object} model.ExecutionTaskListResponse
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/cmd/infra/{infraId}/task [get]
func RestGetInfraExecutionTasks(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	statusFilter := c.QueryParam("status")

	// Convert status filter
	var statusSlice []model.CommandExecutionStatus
	if statusFilter != "" {
		statusSlice = []model.CommandExecutionStatus{model.CommandExecutionStatus(statusFilter)}
	}
	// If no filter specified, statusSlice remains nil -> returns all statuses

	// Get tasks from persistent CommandStatusInfo (this is the source of truth)
	result, err := infra.GetInfraActiveCommands(nsId, infraId, statusSlice)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetExecutionTask godoc
// @ID GetExecutionTask
// @Summary Get a specific execution task
// @Description Get detailed information about a specific execution task by taskId
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param taskId path string true "Task ID (format: xRequestId:vmId:index)"
// @Success 200 {object} model.ExecutionTaskListResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/cmd/infra/{infraId}/task/{taskId} [get]
func RestGetExecutionTask(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	taskId := c.Param("taskId")

	// Get all active commands and filter by taskId
	// Empty nsId/infraId will scan all namespaces/Infras (for global route support)
	result, err := infra.GetInfraActiveCommands(nsId, infraId, nil)
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
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param taskId path string true "Task ID"
// @Param body body model.CancelTaskRequest false "Optional cancellation reason"
// @Success 200 {object} model.CancelTaskResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/cmd/infra/{infraId}/task/{taskId}/cancel [post]
func RestCancelExecutionTask(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	taskId := c.Param("taskId")

	// Parse optional cancel request body
	req := &model.CancelTaskRequest{}
	c.Bind(req) // Ignore error - body is optional

	// Find the task by taskId from task list
	taskList, err := infra.GetInfraActiveCommands(nsId, infraId, nil)
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
	// Use targetTask.NsId and targetTask.InfraId to support global route (/tumblebug/task/:taskId/cancel)
	response, err := infra.CancelInfraCommand(targetTask.NsId, targetTask.InfraId, targetTask.VmId, targetTask.XRequestId, targetTask.CommandIndex, req.Reason)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return clientManager.EndRequestWithLog(c, nil, response)
}

// RestGetCmdInfraStream godoc
// @ID GetCmdInfraStream
// @Summary Stream real-time command execution logs via SSE
// @Description Subscribe to Server-Sent Events (SSE) for real-time command execution logs.
// @Description Use the xRequestId returned from POST /ns/{nsId}/cmd/infra/{infraId}?async=true to connect.
// @Description Events: CommandStatus (status transitions), CommandLog (stdout/stderr lines), CommandDone (terminal).
// @Tags [MC-Infra] Infra Remote Command
// @Produce text/event-stream
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param xRequestId query string true "Request ID from async command execution"
// @Success 200 {object} model.CommandStreamEvent "SSE stream of command events"
// @Failure 400 {object} model.SimpleMsg "Missing xRequestId"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/stream/cmd/infra/{infraId} [get]
func RestGetCmdInfraStream(c echo.Context) error {
	xRequestId := c.QueryParam("xRequestId")
	if xRequestId == "" {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "xRequestId query parameter is required"})
	}

	log.Info().Str("xRequestId", xRequestId).Msg("SSE stream client connected")

	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	c.Response().WriteHeader(http.StatusOK)
	c.Response().Flush()

	// Subscribe to the command log broker
	eventCh, cleanup := infra.SubscribeCommandEvents(xRequestId)
	defer cleanup()

	enc := json.NewEncoder(c.Response())
	clientGone := c.Request().Context().Done()

	// Send initial SSE comment as keepalive / connection confirmation
	fmt.Fprintf(c.Response(), ": connected to stream for xRequestId=%s\n\n", xRequestId)
	c.Response().Flush()

	// Keepalive ticker to prevent proxy/load-balancer timeouts
	keepaliveTicker := time.NewTicker(15 * time.Second)
	defer keepaliveTicker.Stop()

	eventCount := 0
	for {
		select {
		case <-clientGone:
			// Client disconnected
			log.Debug().Str("xRequestId", xRequestId).Int("eventsSent", eventCount).Msg("SSE client disconnected")
			return nil

		case event, ok := <-eventCh:
			if !ok {
				// Channel closed (session ended)
				// Send a final SSE comment before closing
				log.Info().Str("xRequestId", xRequestId).Int("eventsSent", eventCount).Msg("SSE stream channel closed, ending stream")
				fmt.Fprint(c.Response(), ": stream ended\n\n")
				c.Response().Flush()
				return nil
			}

			eventCount++
			// log.Debug().Str("xRequestId", xRequestId).Str("eventType", string(event.Type)).Int("eventCount", eventCount).Msg("Sending SSE event to client")

			// Write SSE format: "data: {json}\n\n"
			fmt.Fprint(c.Response(), "data: ")
			if err := enc.Encode(event); err != nil {
				log.Error().Err(err).Str("xRequestId", xRequestId).Msg("Failed to encode SSE event")
				return nil
			}
			fmt.Fprint(c.Response(), "\n")
			c.Response().Flush()

			// If this is the terminal event, end the stream
			if event.Type == model.EventCommandDone {
				log.Info().Str("xRequestId", xRequestId).Int("eventsSent", eventCount).Msg("SSE stream completed (CommandDone sent)")
				return nil
			}

		case <-keepaliveTicker.C:
			// Send SSE comment as keepalive
			fmt.Fprint(c.Response(), ": keepalive\n\n")
			c.Response().Flush()
		}
	}
}
