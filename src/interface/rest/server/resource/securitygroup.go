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
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
)

// RestPostSecurityGroup godoc
// @ID PostSecurityGroup
// @Summary Create Security Group
// @Description Create Security Group
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option: [required params for register] connectionName, name, vNetId, cspResourceId" Enums(register)
// @Param securityGroupReq body model.TbSecurityGroupReq true "Details for an securityGroup object"
// @Success 200 {object} model.TbSecurityGroupInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [post]
func RestPostSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &model.TbSecurityGroupReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.CreateSecurityGroup(nsId, u, optionFlag)
	return clientManager.EndRequestWithLog(c, err, content)
}

/*
	function RestPutSecurityGroup not yet implemented

// RestPutSecurityGroup godoc
// @ID PutSecurityGroup
// @Summary Update Security Group
// @Description Update Security Group
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param securityGroupInfo body model.TbSecurityGroupInfo true "Details for an securityGroup object"
// @Success 200 {object} model.TbSecurityGroupInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [put]
*/
func RestPutSecurityGroup(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetSecurityGroup godoc
// @ID GetSecurityGroup
// @Summary Get Security Group
// @Description Get Security Group
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param securityGroupId path string true "Security Group ID"
// @Success 200 {object} model.TbSecurityGroupInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [get]
func RestGetSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllSecurityGroup
type RestGetAllSecurityGroupResponse struct {
	SecurityGroup []model.TbSecurityGroupInfo `json:"securityGroup"`
}

// RestGetAllSecurityGroup godoc
// @ID GetAllSecurityGroup
// @Summary List all Security Groups or Security Groups' ID
// @Description List all Security Groups or Security Groups' ID
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: systemLabel)"
// @Param filterVal query string false "Field value for filtering (ex: Registered from CSP resource)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSecurityGroupResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [get]
func RestGetAllSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelSecurityGroup godoc
// @ID DelSecurityGroup
// @Summary Delete Security Group
// @Description Delete Security Group
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param securityGroupId path string true "Security Group ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [delete]
func RestDelSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllSecurityGroup godoc
// @ID DelAllSecurityGroup
// @Summary Delete all Security Groups
// @Description Delete all Security Groups
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [delete]
func RestDelAllSecurityGroup(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
