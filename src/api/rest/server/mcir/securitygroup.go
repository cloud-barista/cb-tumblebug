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
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostSecurityGroup godoc
// @Summary Create Security Group
// @Description Create Security Group
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option: [required params for register] connectionName, name, vNetId, cspSecurityGroupId" Enums(register)
// @Param securityGroupReq body mcir.TbSecurityGroupReq true "Details for an securityGroup object"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [post]
func RestPostSecurityGroup(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &mcir.TbSecurityGroupReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.CreateSecurityGroup(nsId, u, optionFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

/*
	function RestPutSecurityGroup not yet implemented

// RestPutSecurityGroup godoc
// @Summary Update Security Group
// @Description Update Security Group
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param securityGroupInfo body mcir.TbSecurityGroupInfo true "Details for an securityGroup object"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [put]
*/
func RestPutSecurityGroup(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetSecurityGroup godoc
// @Summary Get Security Group
// @Description Get Security Group
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param securityGroupId path string true "Security Group ID"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [get]
func RestGetSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllSecurityGroup
type RestGetAllSecurityGroupResponse struct {
	SecurityGroup []mcir.TbSecurityGroupInfo `json:"securityGroup"`
}

// RestGetAllSecurityGroup godoc
// @Summary List all Security Groups or Security Groups' ID
// @Description List all Security Groups or Security Groups' ID
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: systemLabel)"
// @Param filterVal query string false "Field value for filtering (ex: Registered from CSP resource)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSecurityGroupResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [get]
func RestGetAllSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelSecurityGroup godoc
// @Summary Delete Security Group
// @Description Delete Security Group
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param securityGroupId path string true "Security Group ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [delete]
func RestDelSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllSecurityGroup godoc
// @Summary Delete all Security Groups
// @Description Delete all Security Groups
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [delete]
func RestDelAllSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
