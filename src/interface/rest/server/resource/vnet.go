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

// RestPostVNet godoc
// @ID PostVNet
// @Summary Create VNet
// @Description Create a new VNet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetReq body model.VNetReq false "Details for an VNet object"
// @Success 201 {object} model.VNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [post]
func RestPostVNet(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// Create vNet
	// [Input] Bind the request body
	reqt := &model.VNetReq{}
	if err := c.Bind(reqt); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Validation] Validate the request
	err = resource.ValidateVNetReq(reqt)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process] Create new vNet
	resp, err := resource.CreateVNet(nsId, reqt)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output] Return the created vNet info
	return c.JSON(http.StatusCreated, resp)
}

/*
	function RestPutVNet not yet implemented

// RestPutVNet godoc
// @ID PutVNet
// @Summary Update VNet
// @Description Update VNet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param vNetInfo body model.VNetInfo true "Details for an VNet object"
// @Success 200 {object} model.VNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [put]
*/
// func RestPutVNet(c echo.Context) error {
// 	//nsId := c.Param("nsId")

// 	return nil
// }

// RestGetVNet godoc
// @ID GetVNet
// @Summary Get VNet
// @Description Get VNet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} model.VNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [get]
func RestGetVNet(c echo.Context) error {
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

	// [Process]
	resp, err := resource.GetVNet(nsId, vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, resp)
}

// Response structure for RestGetAllVNet
type RestGetAllVNetResponse struct {
	VNet []model.VNetInfo `json:"vNet"`
}

// RestGetAllVNet godoc
// @ID GetAllVNet
// @Summary List all VNets or VNets' ID
// @Description List all VNets or VNets' ID
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: cspResourceName)"
// @Param filterVal query string false "Field value for filtering (ex: default-alibaba-ap-northeast-1-vpc)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllVNetResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [get]
func RestGetAllVNet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelVNet godoc
// @ID DelVNet
// @Summary Delete VNet (supporting actions: withsubnet, refine, force)
// @Description Delete VNet
// @Description - withsubnets: delete VNet and its subnets
// @Description - refine: delete information of VNet and its subnets if there's no info/resource in Spider/CSP
// @Description - force: delete VNet and its subnets regardless of the status of info/resource in Spider/CSP
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param action query string false "Action" Enums(withsubnets,refine,force)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [delete]
func RestDelVNet(c echo.Context) error {
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

	actionParam := c.QueryParam("action")
	action, valid := resource.ParseNetworkAction(actionParam)
	if !valid {
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})

	}

	var resp model.SimpleMsg
	var err error

	switch action {
	case resource.ActionNone, resource.ActionWithSubnets, resource.ActionForce:
		// [Process]
		resp, err = resource.DeleteVNet(nsId, vNetId, action.String())
		if err != nil {
			log.Error().Err(err).Msg("")
			return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
		}
	case resource.ActionRefine:
		// [Process]
		resp, err = resource.RefineVNet(nsId, vNetId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
		}
	default:
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, resp)
}

// RestDelAllVNet godoc
// @ID DelAllVNet
// @Summary Delete all VNets
// @Description Delete all VNets
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.ResourceDeleteResults
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [delete]
func RestDelAllVNet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestPostRegisterVNet godoc
// @ID PostRegisterVNet
// @Summary Register VNet (created in CSP)
// @Description Register the VNet, which was created in CSP
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetRegisterReq body model.RegisterVNetReq true "Inforamation required to register the VNet created externally"
// @Success 201 {object} model.VNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/registerCspResource/vNet [post]
func RestPostRegisterVNet(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// action := c.QueryParam("action")
	// if action == "" {
	// 	// Error
	// 	return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "action is required"})
	// } else if action != "" && action != "register" {
	// 	// Error
	// 	return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "action '" + action + "' not supported"})
	// }

	// Register vNet if the action is 'register'
	// [Input] Bind the request body
	reqt := &model.RegisterVNetReq{}
	if err := c.Bind(reqt); err != nil {
		log.Warn().Err(err).Msgf("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process] Register the VNet created externally
	resp, err := resource.RegisterVNet(nsId, reqt)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output] Return the registered vNet info
	return c.JSON(http.StatusCreated, resp)
}

// RestDeleteDeregisterVNet godoc
// @ID DeleteDeregisterVNet
// @Summary Deregister VNet (created in CSP)
// @Description Deregister the VNet, which was created in CSP
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Param withSubnets query string false "Delete subnets as well" Enums(true,false)
// @Success 201 {object} model.VNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/deregisterResource/vNet/{vNetId} [delete]
func RestDeleteDeregisterVNet(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	vNetId := c.Param("vNetId")
	err = common.CheckString(vNetId)
	if err != nil {
		errMsg := fmt.Errorf("invalid vNetId (%s)", vNetId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	withSubnets := c.QueryParam("withSubnets")
	if withSubnets != "" && withSubnets != "true" && withSubnets != "false" {
		errMsg := fmt.Errorf("invalid option, withSubnets (%s)", withSubnets)
		log.Warn().Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}
	if withSubnets == "" {
		withSubnets = "false"
	}

	// [Process] Deregister the VNet created externally
	resp, err := resource.DeregisterVNet(nsId, vNetId, withSubnets)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output] Return the deregistered result
	return c.JSON(http.StatusCreated, resp)
}
