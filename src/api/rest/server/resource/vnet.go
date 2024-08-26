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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
)

// RestPostVNet godoc
// @ID PostVNet
// @Summary Create VNet
// @Description Create VNet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option: [required params for register] connectionName, name, cspVNetId" Enums(register)
// @Param vNetReq body model.TbVNetReq true "Details for an VNet object"
// @Success 200 {object} model.TbVNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [post]
func RestPostVNet(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	optionFlag := c.QueryParam("option")
	u := &model.TbVNetReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}
	content, err := resource.CreateVNet(nsId, u, optionFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
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
// @Param vNetInfo body model.TbVNetInfo true "Details for an VNet object"
// @Success 200 {object} model.TbVNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [put]
*/
func RestPutVNet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetVNet godoc
// @ID GetVNet
// @Summary Get VNet
// @Description Get VNet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} model.TbVNetInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [get]
func RestGetVNet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllVNet
type RestGetAllVNetResponse struct {
	VNet []model.TbVNetInfo `json:"vNet"`
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
// @Param filterKey query string false "Field key for filtering (ex: cspVNetName)"
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
// @Summary Delete VNet
// @Description Delete VNet
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [delete]
func RestDelVNet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
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
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [delete]
func RestDelAllVNet(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
