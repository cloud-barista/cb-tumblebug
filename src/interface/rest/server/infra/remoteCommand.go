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

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestPostCmdMci godoc
// @ID PostCmdMci
// @Summary Send a command to specified MCI
// @Description Send a command to specified MCI
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
// @Success 200 {object} model.MciSshCmdResult
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

	req := &model.MciCmdReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	output, err := infra.RemoteCommandToMci(nsId, mciId, subGroupId, vmId, labelSelector, req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.MciSshCmdResult{}

	result.Results = append(result.Results, output...)

	common.PrintJsonPretty(result)

	return c.JSON(http.StatusOK, result)

	// return clientManager.EndRequestWithLog(c, err, result)

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
// @Success 200 {object} model.MciSshCmdResult
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
	result, err := infra.TransferFileToMci(nsId, mciId, subGroupId, vmId, fileBytes, file.Filename, targetPath)
	if err != nil {
		err = fmt.Errorf("failed to transfer file to mci %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Return the result
	return clientManager.EndRequestWithLog(c, err, result)
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
