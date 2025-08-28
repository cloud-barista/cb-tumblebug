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

// RestPostFirewallRules godoc
// @ID PostFirewallRules
// @Summary Add new FirewallRules to existing rules
// @Description Add new FirewallRules: Add the provided firewall rules to the existing rules in the Security Group.
// @Description This API will only add new rules without deleting or modifying existing ones.
// @Description If a rule with identical properties already exists, it will be skipped to avoid duplicates.
// @Description
// @Description Usage:
// @Description Use this API to add new firewall rules to a Security Group while preserving existing rules.
// @Description - Only new rules that don't already exist will be added.
// @Description - Existing rules remain unchanged.
// @Description - If an identical rule already exists, it will be skipped.
// @Description
// @Description Notes:
// @Description - "Ports" field supports single port ("22"), port range ("80-100"), and multiple ports/ranges ("22,80-100,443").
// @Description - The valid port number range is 0 to 65535 (inclusive).
// @Description - "Protocol" can be TCP, UDP, ICMP, ALL, etc. (as supported by the cloud provider).
// @Description - "Direction" must be either "inbound" or "outbound".
// @Description - "CIDR" is the allowed IP range.
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param securityGroupId path string true "Security Group ID"
// @Param firewallRuleReq body model.SecurityGroupUpdateReq true "FirewallRules to add (only firewallRules field is used)"
// @Success 200 {object} model.SecurityGroupUpdateResponse "Updated Security Group info with added firewall rules"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [post]
func RestPostFirewallRules(c echo.Context) error {

	nsId := c.Param("nsId")
	securityGroupId := c.Param("securityGroupId")

	u := &model.SecurityGroupUpdateReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert FirewallRuleReq to FirewallRuleInfo for addition
	var rulesToAdd []model.FirewallRuleInfo
	for _, ruleReq := range u.FirewallRules {
		// Convert each rule request to info object(s)
		ruleInfos := resource.ConvertFirewallRuleRequestObjToInfoObjs(ruleReq)
		rulesToAdd = append(rulesToAdd, ruleInfos...)
	}

	sgInfo, err := resource.CreateFirewallRules(nsId, securityGroupId, rulesToAdd, false)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert SecurityGroupInfo to SecurityGroupUpdateResponse for consistency
	response := model.SecurityGroupUpdateResponse{
		Id:       sgInfo.Id,
		Name:     sgInfo.Name,
		Success:  true,
		Message:  "Successfully added new firewall rules",
		Updated:  sgInfo,
		Previous: sgInfo, // Since we don't have the previous state, use current state
	}

	return clientManager.EndRequestWithLog(c, nil, response)
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
// @Summary Delete specific FirewallRules (Replace with remaining rules)
// @Description Delete specific FirewallRules: Remove specified rules from the Security Group while keeping other existing rules.
// @Description This API will remove only the specified rules from the Security Group, leaving all other rules intact.
// @Description
// @Description Usage:
// @Description Use this API to remove specific firewall rules from a Security Group. Only the rules matching the provided criteria will be deleted.
// @Description - Rules that exactly match the provided Direction, Protocol, Port, and CIDR will be removed.
// @Description - All other existing rules will remain unchanged.
// @Description
// @Description Notes:
// @Description - "Ports" field supports single port ("22"), port range ("80-100"), and multiple ports/ranges ("22,80-100,443").
// @Description - "Protocol" can be TCP, UDP, ICMP, ALL, etc. (as supported by the cloud provider).
// @Description - "Direction" must be either "inbound" or "outbound".
// @Description - "CIDR" is the allowed IP range.
// @Tags [Infra Resource] Security Group Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param securityGroupId path string true "Security Group ID"
// @Param firewallRuleReq body model.SecurityGroupUpdateReq true "FirewallRules to delete (only firewallRules field is used)"
// @Success 200 {object} model.SecurityGroupUpdateResponse "Updated Security Group info after rule deletion"
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId}/rules [delete]
func RestDelFirewallRules(c echo.Context) error {

	nsId := c.Param("nsId")
	securityGroupId := c.Param("securityGroupId")

	u := &model.SecurityGroupUpdateReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert FirewallRuleReq to FirewallRuleInfo for deletion
	var rulesToDelete []model.FirewallRuleInfo
	for _, ruleReq := range u.FirewallRules {
		// Convert each rule request to info object(s)
		ruleInfos := resource.ConvertFirewallRuleRequestObjToInfoObjs(ruleReq)
		rulesToDelete = append(rulesToDelete, ruleInfos...)
	}

	sgInfo, err := resource.DeleteFirewallRules(nsId, securityGroupId, rulesToDelete)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Convert SecurityGroupInfo to SecurityGroupUpdateResponse for consistency
	response := model.SecurityGroupUpdateResponse{
		Id:       sgInfo.Id,
		Name:     sgInfo.Name,
		Success:  true,
		Message:  "Successfully deleted specified firewall rules",
		Updated:  sgInfo,
		Previous: sgInfo, // Since we don't have the previous state, use current state
	}

	return clientManager.EndRequestWithLog(c, nil, response)
}
