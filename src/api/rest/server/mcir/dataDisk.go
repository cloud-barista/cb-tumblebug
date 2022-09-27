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

/* UpdateDataDisk not implemented
// RestPutDataDisk godoc
// @Summary Update Data Disk
// @Description Update Data Disk
// @Tags [Infra resource] MCIR Data Disk management
// @Accept  json
// @Produce  json
// @Param dataDiskInfo body mcir.TbDataDiskInfo true "Details for an Data Disk object"
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param dataDiskId path string true "DataDisk ID"
// @Success 200 {object} mcir.TbDataDiskInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk/{dataDiskId} [put]
func RestPutDataDisk(c echo.Context) error {
	nsId := c.Param("nsId")
	dataDiskId := c.Param("resourceId")

	u := &mcir.TbDataDiskInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	updatedDataDisk, err := mcir.UpdateDataDisk(nsId, dataDiskId, *u)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{
			"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusOK, updatedDataDisk)
}
*/

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
