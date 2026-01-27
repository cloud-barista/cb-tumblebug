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
// @Param subnetReq body model.SubnetReq true "Details for an Subnet object"
// @Success 200 {object} model.SubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [post]
func RestPostSubnet(c echo.Context) error {

	// [Input]
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

	reqt := &model.SubnetReq{}
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

// RestGetSubnet godoc
// @ID GetSubnet
// @Summary Get Subnet
// @Description Get Subnet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param subnetId path string true "Subnet ID"
// @Success 200 {object} model.SubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [get]
func RestGetSubnet(c echo.Context) error {

	// [Input]
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
	resp, err := resource.GetSubnet(nsId, vNetId, subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, resp)
}

// Response structure for RestGetAllSubnet
type RestGetAllSubnetResponse struct {
	SubnetInfoList []model.SubnetInfo `json:"subnetInfoList"`
}

// RestGetListSubnet godoc
// @ID GetAllSubnet
// @Summary List all subnets
// @Description List all subnets
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} RestGetAllSubnetResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet [get]
func RestGetListSubnet(c echo.Context) error {

	// [Input]
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

	// [Process]
	ret, err := resource.ListSubnet(nsId, vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	resp := RestGetAllSubnetResponse{}
	resp.SubnetInfoList = ret

	return c.JSON(http.StatusOK, resp)
}

/* function RestPutSubnet not yet implemented
// RestPutSubnet godoc
// @ID PutSubnet
// @Summary Update Subnet
// @Description Update Subnet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param subnetInfo body model.SubnetInfo true "Details for an Subnet object"
// @Success 200 {object} model.SubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId} [put]
func RestPutSubnet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}
*/

/*
// Response structure for RestGetAllSubnet
type RestGetAllSubnetResponse struct {
	Subnet []model.SubnetInfo `json:"subnet"`
}

*/

// RestDelSubnet godoc
// @ID DelSubnet
// @Summary Delete Subnet (supporting actions: refine, force)
// @Description Delete Subnet
// @Description - refine: delete a subnet `object` if there's no resource on CSP or no inforamation on Spider
// @Description - force: force: delete a subnet `resource` on a CSP regardless of the current resource status (e.g., attempt to delete even if in use)
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param subnetId path string true "Subnet ID"
// @Param action query string false "Action" Enums(refine, force)
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

	paramAction := c.QueryParam("action")
	action, vaild := resource.ParseNetworkAction(paramAction)
	if !vaild {
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Err(errMsg).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	var resp model.SimpleMsg
	var err error
	switch action {
	case resource.ActionNone, resource.ActionForce:
		// [Process]
		resp, err = resource.DeleteSubnet(nsId, vNetId, subnetId, action.String())
		if err != nil {
			log.Error().Err(err).Msg("")
			return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
		}
	case resource.ActionRefine:
		// [Process]
		resp, err = resource.RefineSubnet(nsId, vNetId, subnetId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
		}
	default:
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Err(errMsg).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
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

// RestPostRegisterSubnet godoc
// @ID PostRegisterSubnet
// @Summary Register Subnet (created in CSP)
// @Description Register Subnet, which was created in CSP
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param subnetReq body model.RegisterSubnetReq true "Details for an Subnet object"
// @Success 200 {object} model.SubnetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/registerCspResource/vNet/{vNetId}/subnet [post]
func RestPostRegisterSubnet(c echo.Context) error {

	// [Input]
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

	reqt := &model.RegisterSubnetReq{}
	if err := c.Bind(reqt); err != nil {
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	resp, err := resource.RegisterSubnet(nsId, vNetId, reqt)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusCreated, resp)
}

// RestDeleteDeregisterSubnet godoc
// @ID DeleteDeregisterSubnet
// @Summary Deregister Subnet (created in CSP)
// @Description Deregister Subnet, which was created in CSP
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param subnetId path string true "Subnet ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/deregisterResource/vNet/{vNetId}/subnet/{subnetId} [delete]
func RestDeleteDeregisterSubnet(c echo.Context) error {

	// [Input]
	// nsId and vNetId will be checked inside of the DeregisterVNet function
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
	resp, err := resource.DeregisterSubnet(nsId, vNetId, subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusCreated, resp)
}
