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

// Package mcir is to handle REST API for mcir
package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostDataDisk godoc
// @Summary Create Data Disk
// @Description Create Data Disk
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option: " Enums(register)
// @Param dataDiskInfo body mcir.TbDataDiskReq true "Details for an Data Disk object"
// @Success 200 {object} mcir.TbDataDiskInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk [post]
func RestPostDataDisk(c echo.Context) error {
	fmt.Println("[POST DataDisk]")

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &mcir.TbDataDiskReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	content, err := mcir.CreateDataDisk(nsId, u, optionFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

// RestPutDataDisk godoc
// @Summary Upsize Data Disk
// @Description Upsize Data Disk
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param dataDiskUpsizeReq body mcir.TbDataDiskUpsizeReq true "Request body to upsize the dataDisk"
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param dataDiskId path string true "DataDisk ID"
// @Success 200 {object} mcir.TbDataDiskInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk/{dataDiskId} [put]
func RestPutDataDisk(c echo.Context) error {
	nsId := c.Param("nsId")
	dataDiskId := c.Param("resourceId")

	u := &mcir.TbDataDiskUpsizeReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	updatedDataDisk, err := mcir.UpsizeDataDisk(nsId, dataDiskId, u)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{
			"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusOK, updatedDataDisk)
}

// RestGetDataDisk godoc
// @Summary Get Data Disk
// @Description Get Data Disk
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param dataDiskId path string true "Data Disk ID"
// @Success 200 {object} mcir.TbDataDiskInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk/{dataDiskId} [get]
func RestGetDataDisk(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response struct for RestGetAllDataDisk
type RestGetAllDataDiskResponse struct {
	DataDisk []mcir.TbDataDiskInfo `json:"dataDisk"`
}

// RestGetAllDataDisk godoc
// @Summary List all Data Disks or Data Disks' ID
// @Description List all Data Disks or Data Disks' ID
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: systemLabel)"
// @Param filterVal query string false "Field value for filtering (ex: Registered from CSP resource)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllDataDiskResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk [get]
func RestGetAllDataDisk(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelDataDisk godoc
// @Summary Delete Data Disk
// @Description Delete Data Disk
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param dataDiskId path string true "Data Disk ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk/{dataDiskId} [delete]
func RestDelDataDisk(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllDataDisk godoc
// @Summary Delete all Data Disks
// @Description Delete all Data Disks
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk [delete]
func RestDelAllDataDisk(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestPutVmDataDisk godoc
// @Summary Attach/Detach available dataDisk
// @Description Attach/Detach available dataDisk
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param attachDetachDataDiskReq body mcir.TbAttachDetachDataDiskReq false "Request body to attach/detach dataDisk"
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string true "Option for MCIS" Enums(attach, detach)
// @Success 200 {object} mcis.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/dataDisk [put]
func RestPutVmDataDisk(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	option := c.QueryParam("option")

	u := &mcir.TbAttachDetachDataDiskReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	switch option {
	case mcis.AttachDataDisk:
		fallthrough
	case mcis.DetachDataDisk:
		result, err := mcis.AttachDetachDataDisk(nsId, mcisId, vmId, option, u.DataDiskId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		// common.PrintJsonPretty(result)

		return c.JSON(http.StatusOK, result)

	default:
		mapA := map[string]string{"message": fmt.Sprintf("Supported options: %s, %s, %s", mcis.AttachDataDisk, mcis.DetachDataDisk, mcis.AvailableDataDisk)}
		return c.JSON(http.StatusNotFound, &mapA)
	}
	return nil
}

// RestGetVmDataDisk godoc
// @Summary Get available dataDisks for a VM
// @Description Get available dataDisks for a VM
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllDataDiskResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId}/dataDisk [get]
func RestGetVmDataDisk(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")
	optionFlag := c.QueryParam("option")

	result, err := mcis.GetAvailableDataDisks(nsId, mcisId, vmId, optionFlag)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	var content interface{}
	if optionFlag == "id" {
		content = common.IdList{
			IdList: result.([]string),
		}
	} else {
		content = RestGetAllDataDiskResponse{
			DataDisk: result.([]mcir.TbDataDiskInfo),
		}
	}

	return c.JSON(http.StatusOK, &content)
}
