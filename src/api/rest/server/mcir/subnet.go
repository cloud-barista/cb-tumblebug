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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostSubnet godoc
// @ID PostSubnet
// @Summary Create Subnet
// @Description Create Subnet
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param vNetId path string true "VNet ID"
// @Param subnetReq body mcir.TbSubnetReq true "Details for an Subnet object"
// @Success 200 {object} mcir.TbSubnetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [post]
func RestPostSubnet(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	vNetId := c.Param("vNetId")

	u := &mcir.TbSubnetReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	content, err := mcir.CreateSubnet(nsId, vNetId, *u, false)
	return common.EndRequestWithLog(c, reqID, err, content)
}

/* function RestPutSubnet not yet implemented
// RestPutSubnet godoc
// @ID PutSubnet
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
// @ID GetSubnet
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
// @ID GetAllSubnet
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
// @ID DelSubnet
// @Summary Delete Subnet
// @Description Delete Subnet
// @Tags [Infra resource] MCIR Network management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param vNetId path string true "VNet ID"
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
// @ID DelAllSubnet
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
