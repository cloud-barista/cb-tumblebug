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

// Package mcis is to handle REST API for mcis
package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostMcis godoc
// @Summary Create MCIS
// @Description Create MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisReq body TbMcisReq true "Details for an MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [post]
func RestPostMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CreateMcis(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	//fmt.Printf("%+v\n", *result)
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisDynamic godoc
// @Summary Create MCIS Dynamically
// @Description Create MCIS Dynamically from common spec and image
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisReq body TbMcisDynamicReq true "Details for MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcisDynamic [post]
func RestPostMcisDynamic(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisDynamicReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CreateMcisDynamic(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	//fmt.Printf("%+v\n", *result)
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisVm godoc
// @Summary Create VM in specified MCIS
// @Description Create VM in specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmReq body mcis.TbVmReq true "Details for an VM object"
// @Success 200 {object} mcis.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm [post]
func RestPostMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	vmInfoData := &mcis.TbVmInfo{}
	if err := c.Bind(vmInfoData); err != nil {
		return err
	}
	common.PrintJsonPretty(*vmInfoData)

	result, err := mcis.CorePostMcisVm(nsId, mcisId, vmInfoData)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisVmGroup godoc
// @Summary Create multiple VMs by VM group in specified MCIS
// @Description Create multiple VMs by VM group in specified MCIS
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vmReq body mcis.TbVmReq true "Details for VM Group"
// @Success 200 {object} mcis.TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vmgroup [post]
func RestPostMcisVmGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	vmInfoData := &mcis.TbVmReq{}
	if err := c.Bind(vmInfoData); err != nil {
		return err
	}
	common.PrintJsonPretty(*vmInfoData)

	result, err := mcis.CreateMcisGroupVm(nsId, mcisId, vmInfoData)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}
