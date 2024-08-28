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

// Package resource is to handle REST API for resource
package resource

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostSubnet godoc
// @ID PostSubnet
// @Summary Create Subnet
// @Description Create Subnet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param subnetReq body model.TbSubnetReq true "Details for an Subnet object"
// @Success 200 {object} model.TbSubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [post]
func RestPostSubnet(c echo.Context) error {

	// [Input]
	// nsId and vNetId will be checked inside of the DeleteVNet function
	nsId := c.Param("nsId")
	if err := common.CheckString(nsId); err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	vNetId := c.Param("vNetId")
	if err := common.CheckString(vNetId); err != nil {
		errMsg := fmt.Errorf("invalid vNetId (%s)", vNetId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	reqt := model.TbSubnetReq{}
	if err := c.Bind(reqt); err != nil {
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	resp, err := resource.CreateSubnet(nsId, vNetId, reqt)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusCreated, resp)
}

/* function RestPutSubnet not yet implemented
// RestPutSubnet godoc
// @ID PutSubnet
// @Summary Update Subnet
// @Description Update Subnet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param subnetInfo body model.TbSubnetInfo true "Details for an Subnet object"
// @Success 200 {object} model.TbSubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param subnetId path string true "Subnet ID"
// @Success 200 {object} model.TbSubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [get]
func RestGetSubnet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllSubnet
type RestGetAllSubnetResponse struct {
	Subnet []model.TbSubnetInfo `json:"subnet"`
}

// RestGetAllSubnet godoc
// @ID GetAllSubnet
// @Summary List all Subnets or Subnets' ID
// @Description List all Subnets or Subnets' ID
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSubnetResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param subnetId path string true "Subnet ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [delete]
func RestDelSubnet(c echo.Context) error {

	// [Input]
	// nsId and vNetId will be checked inside of the DeleteVNet function
	nsId := c.Param("nsId")
	if err := common.CheckString(nsId); err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	vNetId := c.Param("vNetId")
	if err := common.CheckString(vNetId); err != nil {
		errMsg := fmt.Errorf("invalid vNetId (%s)", vNetId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}
	subnetId := c.Param("subnetId")
	if err := common.CheckString(subnetId); err != nil {
		errMsg := fmt.Errorf("invalid subnetId (%s)", subnetId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// [Process]
	resp, err := resource.DeleteSubnet(nsId, vNetId, subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusCreated, resp)
}

/*
// RestDelAllSubnet godoc
// @ID DelAllSubnet
// @Summary Delete all Subnets
// @Description Delete all Subnets
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [delete]
func RestDelAllSubnet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
*/
