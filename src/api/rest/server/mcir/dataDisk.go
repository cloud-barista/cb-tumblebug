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
	"strconv"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mci"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostDataDisk godoc
// @ID PostDataDisk
// @Summary Create Data Disk
// @Description Create Data Disk
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option: " Enums(register)
// @Param dataDiskInfo body mcir.TbDataDiskReq true "Details for an Data Disk object"
// @Success 200 {object} mcir.TbDataDiskInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk [post]
func RestPostDataDisk(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &mcir.TbDataDiskReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.CreateDataDisk(nsId, u, optionFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestPutDataDisk godoc
// @ID PutDataDisk
// @Summary Upsize Data Disk
// @Description Upsize Data Disk
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param dataDiskUpsizeReq body mcir.TbDataDiskUpsizeReq true "Request body to upsize the dataDisk"
// @Param nsId path string true "Namespace ID" default(default)
// @Param dataDiskId path string true "DataDisk ID"
// @Success 200 {object} mcir.TbDataDiskInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk/{dataDiskId} [put]
func RestPutDataDisk(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")
	dataDiskId := c.Param("resourceId")

	u := &mcir.TbDataDiskUpsizeReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.UpsizeDataDisk(nsId, dataDiskId, u)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetDataDisk godoc
// @ID GetDataDisk
// @Summary Get Data Disk
// @Description Get Data Disk
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
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
// @ID GetAllDataDisk
// @Summary List all Data Disks or Data Disks' ID
// @Description List all Data Disks or Data Disks' ID
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
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
// @ID DelDataDisk
// @Summary Delete Data Disk
// @Description Delete Data Disk
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param dataDiskId path string true "Data Disk ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk/{dataDiskId} [delete]
func RestDelDataDisk(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllDataDisk godoc
// @ID DelAllDataDisk
// @Summary Delete all Data Disks
// @Description Delete all Data Disks
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/dataDisk [delete]
func RestDelAllDataDisk(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestPutVmDataDisk godoc
// @ID PutVmDataDisk
// @Summary Attach/Detach available dataDisk
// @Description Attach/Detach available dataDisk
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param attachDetachDataDiskReq body mcir.TbAttachDetachDataDiskReq false "Request body to attach/detach dataDisk"
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Param option query string true "Option for MCI" Enums(attach, detach)
// @Param force query string false "Force to attach/detach even if VM info is not matched" Enums(true, false)
// @Success 200 {object} mci.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/dataDisk [put]
func RestPutVmDataDisk(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	option := c.QueryParam("option")
	forceStr := c.QueryParam("force")
	forceBool := false
	var err error
	if forceStr != "" {
		forceBool, err = strconv.ParseBool(forceStr)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, fmt.Errorf("Invalid force value: %s", forceStr), nil)
		}
	}

	u := &mcir.TbAttachDetachDataDiskReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	switch option {
	case common.AttachDataDisk:
		fallthrough
	case common.DetachDataDisk:
		result, err := mci.AttachDetachDataDisk(nsId, mciId, vmId, option, u.DataDiskId, forceBool)
		return common.EndRequestWithLog(c, reqID, err, result)

	default:
		err := fmt.Errorf("Supported options: %s, %s, %s", common.AttachDataDisk, common.DetachDataDisk, common.AvailableDataDisk)
		return common.EndRequestWithLog(c, reqID, err, nil)
	}
}

// RestPostVmDataDisk godoc
// @ID PostVmDataDisk
// @Summary Provisioning (Create and attach) dataDisk
// @Description Provisioning (Create and attach) dataDisk
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param dataDiskInfo body mcir.TbDataDiskVmReq true "Details for an Data Disk object"
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} mci.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/dataDisk [post]
func RestPostVmDataDisk(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")

	u := &mcir.TbDataDiskVmReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	result, err := mci.ProvisionDataDisk(nsId, mciId, vmId, u)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	return common.EndRequestWithLog(c, reqID, err, result)
}

// RestGetVmDataDisk godoc
// @ID GetVmDataDisk
// @Summary Get available dataDisks for a VM
// @Description Get available dataDisks for a VM
// @Tags [Infra Resource] Data Disk Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmId path string true "VM ID" default(g1-1)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllDataDiskResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm/{vmId}/dataDisk [get]
func RestGetVmDataDisk(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	vmId := c.Param("vmId")
	optionFlag := c.QueryParam("option")

	result, err := mci.GetAvailableDataDisks(nsId, mciId, vmId, optionFlag)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
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

	return common.EndRequestWithLog(c, reqID, err, content)
}
