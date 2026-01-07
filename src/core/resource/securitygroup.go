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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"encoding/json"
	"fmt"
	"time"

	"reflect"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// parsePort parses a port string to int, returns -1 if invalid
func parsePort(s string) int {
	if s == "*" || s == "-1" {
		return 0
	}
	var p int
	_, err := fmt.Sscanf(s, "%d", &p)
	if err != nil {
		return -1
	}
	return p
}

// SecurityGroupReqStructLevelValidation is a function to validate 'SecurityGroupReq' object.
func SecurityGroupReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.SecurityGroupReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// SpiderSecurityRuleInfo → FirewallRuleInfo
func ConvertSpiderToFirewallRuleInfo(s model.SpiderSecurityRuleInfo) model.FirewallRuleInfo {
	ports := s.FromPort
	if s.FromPort != s.ToPort {
		ports = s.FromPort + "-" + s.ToPort
	}
	if strings.EqualFold(s.IPProtocol, "ICMP") || strings.EqualFold(s.IPProtocol, "ALL") {
		ports = ""
	}
	// if one of FromPort or ToPort is empty or "-1", set ports to empty string
	if s.FromPort == "" || s.ToPort == "" || s.FromPort == "-1" || s.ToPort == "-1" {
		ports = ""
	}

	return model.FirewallRuleInfo{
		Port:      ports,
		Protocol:  s.IPProtocol,
		Direction: s.Direction,
		CIDR:      s.CIDR,
	}
}

// FirewallRuleInfo → SpiderSecurityRuleInfo
func ConvertTbToSpiderSecurityRuleInfo(t model.FirewallRuleInfo) model.SpiderSecurityRuleInfo {
	var from, to string

	if strings.EqualFold(t.Protocol, "ALL") || strings.EqualFold(t.Protocol, "ICMP") {
		from, to = "-1", "-1"
	} else if t.Port == "" || t.Port == "-1" { // if Port is empty or "-1", set both from, to to "-1"
		from, to = "-1", "-1"
	} else {
		from, to = parsePortsToFromTo(t.Port)
	}
	return model.SpiderSecurityRuleInfo{
		FromPort:   from,
		ToPort:     to,
		IPProtocol: t.Protocol,
		Direction:  t.Direction,
		CIDR:       t.CIDR,
	}
}

// ConvertFirewallRuleRequestObjToInfoObjs converts a FirewallRuleReq object to a slice of FirewallRuleInfo objects.
// It handles single ports, port ranges, and multiple ports/ranges in a comma-separated format.
func ConvertFirewallRuleRequestObjToInfoObjs(req model.FirewallRuleReq) []model.FirewallRuleInfo {
	var infos []model.FirewallRuleInfo
	seperator := ","
	ports := strings.Split(req.Ports, seperator)

	for _, port := range ports {
		infos = append(infos, model.FirewallRuleInfo{
			Port:      port,
			Protocol:  req.Protocol,
			Direction: req.Direction,
			CIDR:      req.CIDR,
		})
	}
	return infos
}

// parsePortsToFromTo parses a port string to FromPort and ToPort
func parsePortsToFromTo(ports string) (string, string) {
	parts := strings.Split(ports, ",")
	if len(parts) == 0 {
		return "", ""
	}
	p := strings.TrimSpace(parts[0])
	if strings.Contains(p, "-") {
		rangeParts := strings.SplitN(p, "-", 2)
		if len(rangeParts) == 2 {
			return strings.TrimSpace(rangeParts[0]), strings.TrimSpace(rangeParts[1])
		}
	}
	return p, p
}

// CreateSecurityGroup accepts SG creation request, creates and returns an TB SG object
func CreateSecurityGroup(nsId string, u *model.SecurityGroupReq, option string) (model.SecurityGroupInfo, error) {

	resourceType := model.StrSecurityGroup

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.SecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// if option == "register" {
	// 	mockFirewallRule := model.SpiderSecurityRuleInfo{
	// 		FromPort:   "22",
	// 		ToPort:     "22",
	// 		IPProtocol: "tcp",
	// 		Direction:  "inbound",
	// 		CIDR:       "0.0.0.0/0",
	// 	}

	// 	*u.FirewallRules = append(*u.FirewallRules, mockFirewallRule)
	// }

	if option != "register" {
		err = validate.Var(u.FirewallRules, "required")
		if err != nil {
			temp := model.SecurityGroupInfo{}
			if _, ok := err.(*validator.InvalidValidationError); ok {
				log.Err(err).Msg("")
				return temp, err
			}
			return temp, err
		}
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			temp := model.SecurityGroupInfo{}
			return temp, err
		}

		temp := model.SecurityGroupInfo{}
		return temp, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		temp := model.SecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup " + u.Name + " already exists.")
		return temp, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.SecurityGroupInfo{}
		err := fmt.Errorf("Cannot create securityGroup")
		return content, err
	}

	uid := common.GenUid()

	// TODO: Need to be improved
	// Avoid retrieving vNet info if option == register
	// Assign random temporal ID to u.VNetId
	if option == "register" && u.VNetId == "not defined" {
		resourceList, err := ListResource(nsId, model.StrVNet, "", "")

		if err != nil {
			log.Error().Err(err).Msg("")
			err := fmt.Errorf("Cannot list vNet Ids for securityGroup")
			return model.SecurityGroupInfo{}, err
		}

		var content struct {
			VNet []model.VNetInfo `json:"vNet"`
		}
		content.VNet = resourceList.([]model.VNetInfo) // type assertion (interface{} -> array)

		if len(content.VNet) == 0 {
			errString := "There is no " + model.StrVNet + " resource in " + nsId
			err := fmt.Errorf(errString)
			log.Error().Err(err).Msg("")
			return model.SecurityGroupInfo{}, err
		}

		// Assign random temporal ID to u.VNetId (should be in the same Connection with SG)
		for _, r := range content.VNet {
			if r.ConnectionName == u.ConnectionName {
				u.VNetId = r.Id
			}
		}
	}

	vNetInfo := model.VNetInfo{}
	tempInterface, err := GetResource(nsId, model.StrVNet, u.VNetId)
	if err != nil {
		err := fmt.Errorf("Failed to get the VNetInfo " + u.VNetId + ".")
		return model.SecurityGroupInfo{}, err
	}
	err = common.CopySrcToDest(&tempInterface, &vNetInfo)
	if err != nil {
		err := fmt.Errorf("Failed to get the VNetInfo-CopySrcToDest() " + u.VNetId + ".")
		return model.SecurityGroupInfo{}, err
	}

	requestBody := model.SpiderSecurityReqInfoWrapper{}
	requestBody.ConnectionName = u.ConnectionName
	requestBody.ReqInfo.Name = uid
	requestBody.ReqInfo.VPCName = vNetInfo.CspResourceName
	requestBody.ReqInfo.CSPId = u.CspResourceId

	// requestBody.ReqInfo.SecurityRules = u.FirewallRules
	if u.FirewallRules != nil {
		for _, v := range *u.FirewallRules {
			expandedRules := ConvertFirewallRuleRequestObjToInfoObjs(v)

			for _, rule := range expandedRules {

				if !strings.EqualFold(rule.Protocol, "ICMP") && !strings.EqualFold(rule.Protocol, "ALL") {
					if !isValidPorts(rule.Port) {
						err := fmt.Errorf("invalid port range in rule: %v", rule)
						return model.SecurityGroupInfo{}, err
					}
				}

				spiderSecurityRuleInfo := ConvertTbToSpiderSecurityRuleInfo(rule)

				requestBody.ReqInfo.SecurityRules = append(requestBody.ReqInfo.SecurityRules, spiderSecurityRuleInfo)
			}
		}
	}

	var callResult model.SpiderSecurityInfo

	client := clientManager.NewHttpClient()
	client.SetAllowGetMethodPayload(true)

	var url string
	var method string
	if option == "register" && u.CspResourceId == "" {
		url = fmt.Sprintf("%s/securitygroup/%s", model.SpiderRestUrl, u.Name)
		method = "GET"
	} else if option == "register" && u.CspResourceId != "" {
		url = fmt.Sprintf("%s/regsecuritygroup", model.SpiderRestUrl)
		method = "POST"
	} else { // option != "register"
		url = fmt.Sprintf("%s/securitygroup", model.SpiderRestUrl)
		method = "POST"
	}

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.SecurityGroupInfo{}, err
	}

	tempSpiderSecurityInfo := &callResult

	content := model.SecurityGroupInfo{}
	content.ResourceType = resourceType
	content.Id = u.Name
	content.Name = u.Name
	content.Uid = uid
	content.ConnectionName = u.ConnectionName
	content.VNetId = u.VNetId
	content.CspResourceId = tempSpiderSecurityInfo.IId.SystemId
	content.CspResourceName = tempSpiderSecurityInfo.IId.NameId
	content.Description = u.Description
	content.KeyValueList = tempSpiderSecurityInfo.KeyValueList
	content.AssociatedObjectList = []string{}
	content.ConnectionConfig, err = common.GetConnConfig(content.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	// content.FirewallRules = tempSpiderSecurityInfo.SecurityRules
	tempTbFirewallRules := []model.FirewallRuleInfo{}
	for _, v := range tempSpiderSecurityInfo.SecurityRules {
		tempTbFirewallRule := ConvertSpiderToFirewallRuleInfo(v)
		tempTbFirewallRules = append(tempTbFirewallRules, tempTbFirewallRule)
	}
	content.FirewallRules = tempTbFirewallRules

	if option == "register" && u.CspResourceId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if option == "register" && u.CspResourceId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrSecurityGroup,
		model.LabelId:              content.Id,
		model.LabelName:            content.Name,
		model.LabelUid:             content.Uid,
		model.LabelVNetId:          content.VNetId,
		model.LabelCspResourceId:   content.CspResourceId,
		model.LabelCspResourceName: content.CspResourceName,
		model.LabelDescription:     content.Description,
		model.LabelConnectionName:  content.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrSecurityGroup, uid, Key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	return content, nil
}

// CreateFirewallRules accepts firewallRule creation request, creates and returns an TB securityGroup object
func CreateFirewallRules(nsId string, securityGroupId string, req []model.FirewallRuleInfo, objectOnly bool) (model.SecurityGroupInfo, error) {
	// Which one would be better, 'req model.FirewallRuleInfo' vs. 'req model.FirewallRuleInfo' ?

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.SecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(securityGroupId)
	if err != nil {
		temp := model.SecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// Validate each TbFirewallRule
	for i, v := range req {
		err = validate.Struct(v)
		if err != nil {

			if _, ok := err.(*validator.InvalidValidationError); ok {
				log.Err(err).Msg("")
				temp := model.SecurityGroupInfo{}
				return temp, err
			}

			temp := model.SecurityGroupInfo{}
			return temp, err
		}

		req[i].Protocol = strings.ToUpper(req[i].Protocol)
		req[i].Direction = strings.ToLower(req[i].Direction)
	}

	parentResourceType := model.StrSecurityGroup

	check, err := CheckResource(nsId, parentResourceType, securityGroupId)

	if !check {
		temp := model.SecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup %s does not exist.", securityGroupId)
		return temp, err
	}

	if err != nil {
		temp := model.SecurityGroupInfo{}
		err := fmt.Errorf("Failed to check the existence of the securityGroup %s.", securityGroupId)
		return temp, err
	}

	securityGroupKey := common.GenResourceKey(nsId, model.StrSecurityGroup, securityGroupId)
	securityGroupKeyValue, _, _ := kvstore.GetKv(securityGroupKey)
	oldSecurityGroup := model.SecurityGroupInfo{}
	err = json.Unmarshal([]byte(securityGroupKeyValue.Value), &oldSecurityGroup)
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	// Return error if the exactly same rule already exists.
	oldSGsFirewallRules := oldSecurityGroup.FirewallRules

	for _, oldRule := range oldSGsFirewallRules {

		for _, newRule := range req {
			if reflect.DeepEqual(oldRule, newRule) {
				err := fmt.Errorf("one of submitted firewall rules already exists in the SG %s.", securityGroupId)
				return oldSecurityGroup, err
			}
		}
	}

	var tempSpiderSecurityInfo *model.SpiderSecurityInfo

	if !objectOnly { // then, call CB-Spider CreateSecurityRule API
		requestBody := model.SpiderSecurityRuleReqInfoWrapper{}
		requestBody.ConnectionName = oldSecurityGroup.ConnectionName
		for _, newRule := range req {
			spRule := ConvertTbToSpiderSecurityRuleInfo(newRule)
			requestBody.ReqInfo.RuleInfoList = append(requestBody.ReqInfo.RuleInfoList, model.SpiderSecurityRuleInfo(spRule))
		}

		url := fmt.Sprintf("%s/securitygroup/%s/rules", model.SpiderRestUrl, oldSecurityGroup.CspResourceName)
		method := "POST"
		var callResult model.SpiderSecurityInfo

		client := clientManager.NewHttpClient()

		_, err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			&callResult,
			clientManager.MediumDuration,
		)

		if err != nil {
			log.Error().Err(err).Msg("")
			return model.SecurityGroupInfo{}, err
		}

		tempSpiderSecurityInfo = &callResult
	}

	log.Info().Msg("POST CreateFirewallRule")

	newSecurityGroup := model.SecurityGroupInfo{}
	newSecurityGroup = oldSecurityGroup
	newSecurityGroup.FirewallRules = nil

	for _, newSpiderSecurityRule := range tempSpiderSecurityInfo.SecurityRules {
		newSecurityGroup.FirewallRules = append(newSecurityGroup.FirewallRules, ConvertSpiderToFirewallRuleInfo(newSpiderSecurityRule))
	}
	Val, _ := json.Marshal(newSecurityGroup)

	err = kvstore.Put(securityGroupKey, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	// securityGroupKey := common.GenResourceKey(nsId, model.StrSecurityGroup, securityGroupId)
	// keyValue, _ := kvstore.GetKv(securityGroupKey)
	//
	//
	// content := model.SecurityGroupInfo{}
	// err = json.Unmarshal([]byte(keyValue.Value), &content)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return err
	// }
	return newSecurityGroup, nil
}

// DeleteFirewallRules accepts firewallRule deletion request, deletes specified rules and returns an TB securityGroup object
func DeleteFirewallRules(nsId string, securityGroupId string, req []model.FirewallRuleInfo) (model.SecurityGroupInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.SecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(securityGroupId)
	if err != nil {
		temp := model.SecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// Validate each TbFirewallRule
	for i, v := range req {
		err = validate.Struct(v)
		if err != nil {

			if _, ok := err.(*validator.InvalidValidationError); ok {
				log.Err(err).Msg("")
				temp := model.SecurityGroupInfo{}
				return temp, err
			}

			temp := model.SecurityGroupInfo{}
			return temp, err
		}

		req[i].Protocol = strings.ToUpper(req[i].Protocol)
		req[i].Direction = strings.ToLower(req[i].Direction)
	}

	parentResourceType := model.StrSecurityGroup

	check, err := CheckResource(nsId, parentResourceType, securityGroupId)

	if !check {
		temp := model.SecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup %s does not exist.", securityGroupId)
		return temp, err
	}

	if err != nil {
		temp := model.SecurityGroupInfo{}
		err := fmt.Errorf("Failed to check the existence of the securityGroup %s.", securityGroupId)
		return temp, err
	}

	securityGroupKey := common.GenResourceKey(nsId, model.StrSecurityGroup, securityGroupId)
	securityGroupKeyValue, _, _ := kvstore.GetKv(securityGroupKey)
	oldSecurityGroup := model.SecurityGroupInfo{}
	err = json.Unmarshal([]byte(securityGroupKeyValue.Value), &oldSecurityGroup)
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	// Return error if one or more of provided rules does not exist.
	oldSGsFirewallRules := oldSecurityGroup.FirewallRules

	found_flag := false

	rulesToDelete := []model.FirewallRuleInfo{}

	for _, oldRule := range oldSGsFirewallRules {

		for _, newRule := range req {
			if reflect.DeepEqual(oldRule, newRule) {
				found_flag = true
				rulesToDelete = append(rulesToDelete, newRule)
				continue
			}
		}
	}

	type SpiderDeleteSecurityRulesResp struct {
		Result string
	}

	var spiderDeleteSecurityRulesResp *SpiderDeleteSecurityRulesResp

	requestBody := model.SpiderSecurityRuleReqInfoWrapper{}
	requestBody.ConnectionName = oldSecurityGroup.ConnectionName

	if found_flag == false {
		err := fmt.Errorf("Any of submitted firewall rules does not exist in the SG %s.", securityGroupId)
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	} else {
		for _, v := range rulesToDelete {
			spRule := ConvertTbToSpiderSecurityRuleInfo(v)
			requestBody.ReqInfo.RuleInfoList = append(requestBody.ReqInfo.RuleInfoList, spRule)
		}
	}

	url := fmt.Sprintf("%s/securitygroup/%s/rules", model.SpiderRestUrl, oldSecurityGroup.CspResourceName)
	method := "DELETE"
	var callResult SpiderDeleteSecurityRulesResp

	client := clientManager.NewHttpClient()

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.SecurityGroupInfo{}, err
	}

	spiderDeleteSecurityRulesResp = &callResult

	if spiderDeleteSecurityRulesResp.Result != "true" {
		err := fmt.Errorf("Failed to delete Security Group rules with CB-Spider.")
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	requestBody2 := model.SpiderConnectionName{}
	requestBody2.ConnectionName = oldSecurityGroup.ConnectionName

	var callResult2 model.SpiderSecurityInfo

	url = fmt.Sprintf("%s/securitygroup/%s", model.SpiderRestUrl, oldSecurityGroup.CspResourceName)
	method = "GET"

	client = clientManager.NewHttpClient()
	client.SetAllowGetMethodPayload(true)

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody2),
		&requestBody2,
		&callResult2,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.SecurityGroupInfo{}, err
	}

	tempSpiderSecurityInfo := &callResult2

	log.Info().Msg("DELETE FirewallRule")

	newSecurityGroup := model.SecurityGroupInfo{}
	newSecurityGroup = oldSecurityGroup
	newSecurityGroup.FirewallRules = nil
	for _, newSpiderSecurityRule := range tempSpiderSecurityInfo.SecurityRules {
		newSecurityGroup.FirewallRules = append(newSecurityGroup.FirewallRules, ConvertSpiderToFirewallRuleInfo(newSpiderSecurityRule))
	}
	Val, _ := json.Marshal(newSecurityGroup)

	err = kvstore.Put(securityGroupKey, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	// securityGroupKey := common.GenResourceKey(nsId, model.StrSecurityGroup, securityGroupId)
	// keyValue, _ := kvstore.GetKv(securityGroupKey)
	//
	//
	// content := model.SecurityGroupInfo{}
	// err = json.Unmarshal([]byte(keyValue.Value), &content)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return err
	// }
	return newSecurityGroup, nil
}

// GetSecurityGroup retrieves a security group object from the key-value store
func GetSecurityGroup(nsId string, securityGroupId string) (model.SecurityGroupInfo, error) {
	sg := model.SecurityGroupInfo{}

	tempInterface, err := GetResource(nsId, model.StrSecurityGroup, securityGroupId)
	if err != nil {
		err := fmt.Errorf("Failed to get the SecurityGroupInfo " + securityGroupId + ".")
		return sg, err
	}
	err = common.CopySrcToDest(&tempInterface, &sg)
	if err != nil {
		err := fmt.Errorf("failed to CopySrcToDest() %s", securityGroupId)
		return sg, err
	}
	return sg, nil
}

// UpdateFirewallRules updates the firewall rules of a security group
func UpdateFirewallRules(nsId string, securityGroupId string, desiredRules []model.FirewallRuleReq) (model.SecurityGroupUpdateResponse, error) {

	// extend the FirewallRuleReq to FirewallRuleInfos
	desiredRulesInfos := []model.FirewallRuleInfo{}
	for _, rule := range desiredRules {
		expandedRules := ConvertFirewallRuleRequestObjToInfoObjs(rule)
		desiredRulesInfos = append(desiredRulesInfos, expandedRules...)
	}

	for _, rule := range desiredRulesInfos {
		if !strings.EqualFold(rule.Protocol, "ICMP") {
			if !isValidPorts(rule.Port) {
				err := fmt.Errorf("invalid port range in rule: %v", rule)
				return model.SecurityGroupUpdateResponse{
					Id:      securityGroupId,
					Name:    securityGroupId, // fallback to ID if name not available
					Success: false,
					Message: fmt.Sprintf("Invalid port range in rule: %v", rule),
				}, err
			}
		}
	}

	previousSg, err := GetSecurityGroup(nsId, securityGroupId)
	if err != nil {
		return model.SecurityGroupUpdateResponse{
			Id:      securityGroupId,
			Name:    securityGroupId,
			Success: false,
			Message: fmt.Sprintf("Failed to get security group: %v", err),
		}, err
	}
	currentRules := previousSg.FirewallRules

	toAdd, toDelete := diffFirewallRules(currentRules, desiredRulesInfos)

	// add log info for debugging
	log.Info().Msgf("Current rules: %v", currentRules)
	log.Info().Msgf("Desired rules: %v", desiredRulesInfos)
	log.Info().Msgf("Rules to add: %v", toAdd)
	log.Info().Msgf("Rules to delete: %v", toDelete)

	isSensitiveProvider := strings.EqualFold(previousSg.ConnectionConfig.ProviderName, csp.NCP)

	var deleteErrors []string
	if len(toDelete) > 0 {

		if isSensitiveProvider {
			// Process each rule deletion sequentially for sensitive provider for better stability
			log.Info().Msg("Using sequential deletion for sensitive provider")
			for _, ruleToDelete := range toDelete {
				_, err := DeleteFirewallRules(nsId, securityGroupId, []model.FirewallRuleInfo{ruleToDelete})
				// wait for seconds before next
				time.Sleep(5 * time.Second)
				// If deletion fails, log the error and continue with next rule
				if err != nil {
					log.Info().Err(err).Msgf("Failed to delete firewall rule: %v. Continuing with next rule.", ruleToDelete)
					deleteErrors = append(deleteErrors, fmt.Sprintf("Delete rule failed (%s-%s-%s-%s): %v",
						ruleToDelete.Direction, ruleToDelete.Protocol, ruleToDelete.Port, ruleToDelete.CIDR, err))
				}
			}
		} else {
			// Process all rules at once for other providers
			log.Info().Msg("Using batch deletion for non-sensitive provider")
			_, err := DeleteFirewallRules(nsId, securityGroupId, toDelete)
			if err != nil {
				log.Info().Err(err).Msg("Failed to delete some firewall rules. Continuing with adding new rules.")
				deleteErrors = append(deleteErrors, fmt.Sprintf("Delete failed: %v", err))
			}
		}
	}

	var addErrors []string
	if len(toAdd) > 0 {

		if isSensitiveProvider {
			// Process each rule addition sequentially for sensitive provider for better stability
			log.Info().Msg("Using sequential addition for sensitive provider")
			for _, ruleToAdd := range toAdd {
				_, err := CreateFirewallRules(nsId, securityGroupId, []model.FirewallRuleInfo{ruleToAdd}, false)
				// wait for seconds before next
				time.Sleep(5 * time.Second)
				if err != nil {
					addErrors = append(addErrors, fmt.Sprintf("Add rule failed (%s-%s-%s-%s): %v",
						ruleToAdd.Direction, ruleToAdd.Protocol, ruleToAdd.Port, ruleToAdd.CIDR, err))
					log.Info().Err(err).Msgf("Failed to add firewall rule: %v. Continuing with next rule.", ruleToAdd)
				}
			}
		} else {
			// Process all rules at once for other providers
			log.Info().Msg("Using batch addition for non-sensitive provider")
			_, err := CreateFirewallRules(nsId, securityGroupId, toAdd, false)
			if err != nil {
				addErrors = append(addErrors, fmt.Sprintf("Add failed: %v", err))
			}
		}

		// If there were any add errors, return the error response
		if len(addErrors) > 0 {
			errorMessage := "Failed to add firewall rules"
			if len(deleteErrors) > 0 {
				errorMessage += "; Also failed to delete some rules"
			}
			return model.SecurityGroupUpdateResponse{
				Id:       securityGroupId,
				Name:     previousSg.Name,
				Success:  false,
				Message:  errorMessage + ": " + strings.Join(addErrors, "; "),
				Previous: previousSg,
			}, fmt.Errorf("failed to add firewall rules: %s", strings.Join(addErrors, "; "))
		}
	}

	updatedSg, err := GetSecurityGroup(nsId, securityGroupId)
	if err != nil {
		return model.SecurityGroupUpdateResponse{
			Id:       securityGroupId,
			Name:     previousSg.Name,
			Success:  false,
			Message:  fmt.Sprintf("Failed to get updated security group: %v", err),
			Previous: previousSg,
		}, err
	}

	// Determine success status and message
	success := len(addErrors) == 0 && len(deleteErrors) == 0
	var message string
	if success {
		message = "Security group rules updated successfully"
	} else {
		var allErrors []string
		allErrors = append(allErrors, deleteErrors...)
		allErrors = append(allErrors, addErrors...)
		message = "Partial success with warnings: " + strings.Join(allErrors, "; ")
	}

	resp := model.SecurityGroupUpdateResponse{
		Id:       securityGroupId,
		Name:     updatedSg.Name,
		Success:  success,
		Message:  message,
		Updated:  updatedSg,
		Previous: previousSg,
	}
	return resp, nil
}

// isValidPorts checks if a Ports string is valid (all ranges/values 0~65535)
func isValidPorts(ports string) bool {
	ranges := parsePortsToRanges(ports)
	if len(ranges) == 0 {
		return false
	}
	for _, r := range ranges {
		if r[0] < 0 || r[0] > 65535 || r[1] < 0 || r[1] > 65535 || r[0] > r[1] {
			return false
		}
	}
	return true
}

// diffFirewallRules applies simplified AWS rule update logic:
// - If Direction and IPProtocol are the same, and port range overlaps, delete old and add new.
// - If Direction, IPProtocol, FromPort, ToPort are the same but CIDR is different, delete old and add new.
// - If all fields are the same, do nothing.
// - Otherwise, add new rule.
func diffFirewallRules(current, desired []model.FirewallRuleInfo) (toAdd, toDelete []model.FirewallRuleInfo) {
	usedCurrent := make([]bool, len(current))
	for _, d := range desired {
		matched := false
		for i, c := range current {
			if strings.EqualFold(d.Direction, c.Direction) && strings.EqualFold(d.Protocol, c.Protocol) {
				if portsOverlap(d.Port, c.Port) {
					if d.Port == c.Port {
						if d.CIDR != c.CIDR {
							toDelete = append(toDelete, c)
							toAdd = append(toAdd, d)
							usedCurrent[i] = true
							matched = true
							break
						} else {
							usedCurrent[i] = true
							matched = true
							break
						}
					} else {
						toDelete = append(toDelete, c)
						toAdd = append(toAdd, d)
						usedCurrent[i] = true
						matched = true
						break
					}
				}
			}
		}
		if !matched {
			toAdd = append(toAdd, d)
		}
	}
	for i, c := range current {
		if !usedCurrent[i] {
			toDelete = append(toDelete, c)
		}
	}
	// remove duplicates in toAdd
	uniqueAdd := make(map[string]model.FirewallRuleInfo)
	for _, rule := range toAdd {
		key := fmt.Sprintf("%s-%s-%s-%s", rule.Direction, rule.Protocol, rule.Port, rule.CIDR)
		if _, exists := uniqueAdd[key]; !exists {
			uniqueAdd[key] = rule
		}
	}
	toAdd = make([]model.FirewallRuleInfo, 0, len(uniqueAdd))
	for _, rule := range uniqueAdd {
		toAdd = append(toAdd, rule)
	}
	// remove duplicates in toDelete
	uniqueDelete := make(map[string]model.FirewallRuleInfo)
	for _, rule := range toDelete {
		key := fmt.Sprintf("%s-%s-%s-%s", rule.Direction, rule.Protocol, rule.Port, rule.CIDR)
		if _, exists := uniqueDelete[key]; !exists {
			uniqueDelete[key] = rule
		}
	}
	toDelete = make([]model.FirewallRuleInfo, 0, len(uniqueDelete))
	for _, rule := range uniqueDelete {
		// skip if the rule is Protocol:ALL outbound 0.0.0.0/0 (it is the default rule)
		if strings.EqualFold(rule.Direction, "outbound") && rule.CIDR == "0.0.0.0/0" && strings.EqualFold(rule.Protocol, "ALL") {
			continue
		}
		toDelete = append(toDelete, rule)
	}

	return toAdd, toDelete
}

// portsOverlap checks if two port ranges overlap
func portsOverlap(portsA, portsB string) bool {
	rangesA := parsePortsToRanges(portsA)
	rangesB := parsePortsToRanges(portsB)
	for _, ra := range rangesA {
		for _, rb := range rangesB {
			if ra[0] <= rb[1] && ra[1] >= rb[0] {
				return true
			}
		}
	}
	return false
}

// parsePortsToRanges parses a string of ports (e.g., "22, 80-90, 443") into a slice of port ranges.
func parsePortsToRanges(ports string) [][2]int {
	var result [][2]int
	for _, p := range strings.Split(ports, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "-") {
			rangeParts := strings.SplitN(p, "-", 2)
			from := parsePort(rangeParts[0])
			to := parsePort(rangeParts[1])
			if from != -1 && to != -1 {
				result = append(result, [2]int{from, to})
			}
		} else {
			v := parsePort(p)
			if v != -1 {
				result = append(result, [2]int{v, v})
			}
		}
	}
	return result
}

// UpdateFirewallRulesBatch updates firewall rules for multiple security groups
func UpdateFirewallRulesBatch(nsId string, securityGroupIds []string, desiredRules []model.FirewallRuleReq) model.RestWrapperSecurityGroupUpdateResponse {
	var responses []model.SecurityGroupUpdateResponse
	var successCount, failedCount int

	for _, sgId := range securityGroupIds {
		response, err := UpdateFirewallRules(nsId, sgId, desiredRules)
		if err != nil {
			// Even if there's an error, we still want to include the response
			if response.Id == "" {
				response.Id = sgId
				response.Name = sgId
				response.Success = false
				response.Message = fmt.Sprintf("Update failed: %v", err)
			}
			failedCount++
		} else {
			if response.Success {
				successCount++
			} else {
				failedCount++
			}
		}
		responses = append(responses, response)
	}

	summary := model.UpdateSummary{
		Total:      len(securityGroupIds),
		Success:    successCount,
		Failed:     failedCount,
		AllSuccess: failedCount == 0,
	}

	return model.RestWrapperSecurityGroupUpdateResponse{
		Response: responses,
		Summary:  summary,
	}
}

// UpdateMultipleFirewallRules updates firewall rules for multiple security groups with parallel processing
func UpdateMultipleFirewallRules(nsId string, securityGroupIds []string, desiredRules []model.FirewallRuleReq) model.RestWrapperSecurityGroupUpdateResponse {
	// Handle empty input
	if len(securityGroupIds) == 0 {
		return model.RestWrapperSecurityGroupUpdateResponse{
			Response: []model.SecurityGroupUpdateResponse{},
			Summary: model.UpdateSummary{
				Total:      0,
				Success:    0,
				Failed:     0,
				AllSuccess: true,
			},
		}
	}

	// Use channels for parallel processing
	type result struct {
		response model.SecurityGroupUpdateResponse
		index    int
	}

	// Create buffered channels
	resultChan := make(chan result, len(securityGroupIds))

	// Launch goroutines for parallel processing
	for i, sgId := range securityGroupIds {
		go func(index int, securityGroupId string) {
			// Add defer to handle any panics in goroutines
			defer func() {
				if r := recover(); r != nil {
					log.Error().Msgf("Panic in UpdateFirewallRules for SG %s: %v", securityGroupId, r)
					resultChan <- result{
						response: model.SecurityGroupUpdateResponse{
							Id:      securityGroupId,
							Name:    securityGroupId,
							Success: false,
							Message: fmt.Sprintf("Internal error occurred: %v", r),
						},
						index: index,
					}
				}
			}()

			response, err := UpdateFirewallRules(nsId, securityGroupId, desiredRules)
			if err != nil {
				// Even if there's an error, we still want to include the response
				if response.Id == "" {
					response.Id = securityGroupId
					response.Name = securityGroupId
					response.Success = false
					response.Message = fmt.Sprintf("Update failed: %v", err)
				}
			}
			resultChan <- result{response: response, index: index}
		}(i, sgId)
	}

	// Collect results maintaining original order
	responses := make([]model.SecurityGroupUpdateResponse, len(securityGroupIds))
	var successCount, failedCount int

	// Wait for all goroutines to complete
	for i := 0; i < len(securityGroupIds); i++ {
		res := <-resultChan
		responses[res.index] = res.response

		if res.response.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	// Close the channel
	close(resultChan)

	summary := model.UpdateSummary{
		Total:      len(securityGroupIds),
		Success:    successCount,
		Failed:     failedCount,
		AllSuccess: failedCount == 0,
	}

	log.Info().Msgf("UpdateMultipleFirewallRules completed: %d total, %d success, %d failed",
		summary.Total, summary.Success, summary.Failed)

	return model.RestWrapperSecurityGroupUpdateResponse{
		Response: responses,
		Summary:  summary,
	}
}
