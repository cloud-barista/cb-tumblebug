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

type TbFirewallRulesWrapper struct {
	FirewallRules []model.TbFirewallRuleInfo `json:"firewallRules"` // validate:"required"`
}

// RestPostFirewallRules godoc
// @ID PostFirewallRules
// @Summary Create FirewallRules
// @Description Create FirewallRules
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param securityGroupId path string true "Security Group ID"
// @Param firewallRuleReq body TbFirewallRulesWrapper true "FirewallRules to create"
// @Success 200 {object} model.TbSecurityGroupInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [post]
func RestPostFirewallRules(c echo.Context) error {

	nsId := c.Param("nsId")
	securityGroupId := c.Param("securityGroupId")

	u := &TbFirewallRulesWrapper{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.CreateFirewallRules(nsId, securityGroupId, *&u.FirewallRules, false)
	return clientManager.EndRequestWithLog(c, err, content)
}

/* function RestPutFirewallRules not yet implemented
// RestPutFirewallRules godoc
// @ID PutFirewallRules
// @Summary Update FirewallRules
// @Description Update FirewallRules
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param firewallRuleInfo body model.TbFirewallRulesInfo true "FirewallRules to update"
// @Success 200 {object} model.TbFirewallRulesInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param firewallRuleReq body TbFirewallRulesWrapper true "FirewallRules to lookup"
// @Success 200 {object} model.TbFirewallRulesInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [get]
func RestGetFirewallRules(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllFirewallRules
type RestGetAllFirewallRulesResponse struct {
	FirewallRules []model.TbFirewallRulesInfo `json:"firewallRules"`
}
*/

// RestDelFirewallRules godoc
// @ID DelFirewallRules
// @Summary Delete FirewallRules
// @Description Delete FirewallRules
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param securityGroupId path string true "Security Group ID"
// @Param firewallRuleReq body TbFirewallRulesWrapper true "FirewallRules to delete"
// @Success 200 {object} model.TbSecurityGroupInfo
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [delete]
func RestDelFirewallRules(c echo.Context) error {

	nsId := c.Param("nsId")
	securityGroupId := c.Param("securityGroupId")

	u := &TbFirewallRulesWrapper{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.DeleteFirewallRules(nsId, securityGroupId, *&u.FirewallRules)
	return clientManager.EndRequestWithLog(c, err, content)
}
