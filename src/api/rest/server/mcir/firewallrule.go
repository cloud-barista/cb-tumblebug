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

type TbFirewallRulesWrapper struct {
	FirewallRules []mcir.TbFirewallRuleInfo `json:"firewallRules"` // validate:"required"`
}

// RestPostFirewallRules godoc
// @ID PostFirewallRules
// @Summary Create FirewallRules
// @Description Create FirewallRules
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param firewallRuleReq body TbFirewallRulesWrapper true "FirewallRules to create"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [post]
func RestPostFirewallRules(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	securityGroupId := c.Param("securityGroupId")

	u := &TbFirewallRulesWrapper{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.CreateFirewallRules(nsId, securityGroupId, *&u.FirewallRules, false)
	return common.EndRequestWithLog(c, reqID, err, content)
}

/* function RestPutFirewallRules not yet implemented
// RestPutFirewallRules godoc
// @ID PutFirewallRules
// @Summary Update FirewallRules
// @Description Update FirewallRules
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param firewallRuleInfo body mcir.TbFirewallRulesInfo true "FirewallRules to update"
// @Success 200 {object} mcir.TbFirewallRulesInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [put]
func RestPutFirewallRules(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}
*/

/*
// RestGetFirewallRules godoc
// @ID GetFirewallRules
// @Summary Get FirewallRules
// @Description Get FirewallRules
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param firewallRuleReq body TbFirewallRulesWrapper true "FirewallRules to lookup"
// @Success 200 {object} mcir.TbFirewallRulesInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [get]
func RestGetFirewallRules(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllFirewallRules
type RestGetAllFirewallRulesResponse struct {
	FirewallRules []mcir.TbFirewallRulesInfo `json:"firewallRules"`
}
*/

// RestDelFirewallRules godoc
// @ID DelFirewallRules
// @Summary Delete FirewallRules
// @Description Delete FirewallRules
// @Tags [Infra resource] MCIR Security group management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param firewallRuleReq body TbFirewallRulesWrapper true "FirewallRules to delete"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [delete]
func RestDelFirewallRules(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	securityGroupId := c.Param("securityGroupId")

	u := &TbFirewallRulesWrapper{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.DeleteFirewallRules(nsId, securityGroupId, *&u.FirewallRules)
	return common.EndRequestWithLog(c, reqID, err, content)
}
