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

// RestPostSubnet godoc
// @Summary Create Subnet
// @Description Create Subnet
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param subnetReq body mcir.TbSubnetReq true "Details for an Subnet object"
// @Success 200 {object} mcir.TbSubnetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [post]
func RestPostSubnet(c echo.Context) error {

	nsId := c.Param("nsId")
	vNetId := c.Param("vNetId")

	u := &mcir.TbSubnetReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[POST Subnet]")
	//fmt.Println("[Creating Subnet]")
	//content, responseCode, body, err := CreateSubnet(nsId, u)
	content, err := mcir.CreateSubnet(nsId, vNetId, *u, false)
	if err != nil {
		common.CBLog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a subnet"}
		*/
		//return c.JSONBlob(responseCode, body)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

/* function RestPutSubnet not yet implemented
// RestPutSubnet godoc
// @Summary Update Subnet
// @Description Update Subnet
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param subnetInfo body mcir.TbSubnetInfo true "Details for an Subnet object"
// @Success 200 {object} mcir.TbSubnetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [put]
func RestPutSubnet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}
*/

/*
// RestGetSubnet godoc
// @Summary Get Subnet
// @Description Get Subnet
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param subnetId path string true "Subnet ID"
// @Success 200 {object} mcir.TbSubnetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [get]
func RestGetSubnet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllSubnet
type RestGetAllSubnetResponse struct {
	Subnet []mcir.TbSubnetInfo `json:"subnet"`
}

// RestGetAllSubnet godoc
// @Summary List all Subnets or Subnets' ID
// @Description List all Subnets or Subnets' ID
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSubnetResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [get]
func RestGetAllSubnet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
*/

// RestDelSubnet godoc
// @Summary Delete Subnet
// @Description Delete Subnet
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param subnetId path string true "Subnet ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [delete]
func RestDelSubnet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

/*
// RestDelAllSubnet godoc
// @Summary Delete all Subnets
// @Description Delete all Subnets
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [delete]
func RestDelAllSubnet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
*/
