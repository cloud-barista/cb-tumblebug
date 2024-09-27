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

	"reflect"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// TbSecurityGroupReqStructLevelValidation is a function to validate 'TbSecurityGroupReq' object.
func TbSecurityGroupReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbSecurityGroupReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// CreateSecurityGroup accepts SG creation request, creates and returns an TB SG object
func CreateSecurityGroup(nsId string, u *model.TbSecurityGroupReq, option string) (model.TbSecurityGroupInfo, error) {

	resourceType := model.StrSecurityGroup

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.TbSecurityGroupInfo{}
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
			temp := model.TbSecurityGroupInfo{}
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
			temp := model.TbSecurityGroupInfo{}
			return temp, err
		}

		temp := model.TbSecurityGroupInfo{}
		return temp, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		temp := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup " + u.Name + " already exists.")
		return temp, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
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
			return model.TbSecurityGroupInfo{}, err
		}

		var content struct {
			VNet []model.TbVNetInfo `json:"vNet"`
		}
		content.VNet = resourceList.([]model.TbVNetInfo) // type assertion (interface{} -> array)

		if len(content.VNet) == 0 {
			errString := "There is no " + model.StrVNet + " resource in " + nsId
			err := fmt.Errorf(errString)
			log.Error().Err(err).Msg("")
			return model.TbSecurityGroupInfo{}, err
		}

		// Assign random temporal ID to u.VNetId (should be in the same Connection with SG)
		for _, r := range content.VNet {
			if r.ConnectionName == u.ConnectionName {
				u.VNetId = r.Id
			}
		}
	}

	vNetInfo := model.TbVNetInfo{}
	tempInterface, err := GetResource(nsId, model.StrVNet, u.VNetId)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo " + u.VNetId + ".")
		return model.TbSecurityGroupInfo{}, err
	}
	err = common.CopySrcToDest(&tempInterface, &vNetInfo)
	if err != nil {
		err := fmt.Errorf("Failed to get the TbVNetInfo-CopySrcToDest() " + u.VNetId + ".")
		return model.TbSecurityGroupInfo{}, err
	}

	requestBody := model.SpiderSecurityReqInfoWrapper{}
	requestBody.ConnectionName = u.ConnectionName
	requestBody.ReqInfo.Name = uid
	requestBody.ReqInfo.VPCName = vNetInfo.CspResourceName
	requestBody.ReqInfo.CSPId = u.CspResourceId

	// requestBody.ReqInfo.SecurityRules = u.FirewallRules
	if u.FirewallRules != nil {
		for _, v := range *u.FirewallRules {
			jsonBody, err := json.Marshal(v)
			if err != nil {
				log.Error().Err(err).Msg("")
			}

			spiderSecurityRuleInfo := model.SpiderSecurityRuleInfo{}
			err = json.Unmarshal(jsonBody, &spiderSecurityRuleInfo)
			if err != nil {
				log.Error().Err(err).Msg("")
			}

			requestBody.ReqInfo.SecurityRules = append(requestBody.ReqInfo.SecurityRules, spiderSecurityRuleInfo)
		}
	}

	var tempSpiderSecurityInfo *model.SpiderSecurityInfo

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&model.SpiderSecurityInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response

	var url string
	if option == "register" && u.CspResourceId == "" {
		url = fmt.Sprintf("%s/securitygroup/%s", model.SpiderRestUrl, u.Name)
		resp, err = req.Get(url)
	} else if option == "register" && u.CspResourceId != "" {
		url = fmt.Sprintf("%s/regsecuritygroup", model.SpiderRestUrl)
		resp, err = req.Post(url)
	} else { // option != "register"
		url = fmt.Sprintf("%s/securitygroup", model.SpiderRestUrl)
		resp, err = req.Post(url)
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
		return content, err
	}

	tempSpiderSecurityInfo = resp.Result().(*model.SpiderSecurityInfo)

	content := model.TbSecurityGroupInfo{}
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

	// content.FirewallRules = tempSpiderSecurityInfo.SecurityRules
	tempTbFirewallRules := []model.TbFirewallRuleInfo{}
	for _, v := range tempSpiderSecurityInfo.SecurityRules {
		tempTbFirewallRule := model.TbFirewallRuleInfo(v) // type casting
		tempTbFirewallRules = append(tempTbFirewallRules, tempTbFirewallRule)
	}
	content.FirewallRules = tempTbFirewallRules

	if option == "register" && u.CspResourceId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if option == "register" && u.CspResourceId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	log.Info().Msg("PUT CreateSecurityGroup")
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
func CreateFirewallRules(nsId string, securityGroupId string, req []model.TbFirewallRuleInfo, objectOnly bool) (model.TbSecurityGroupInfo, error) {
	// Which one would be better, 'req model.TbFirewallRuleInfo' vs. 'req model.TbFirewallRuleInfo' ?

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.TbSecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(securityGroupId)
	if err != nil {
		temp := model.TbSecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// Validate each TbFirewallRule
	for i, v := range req {
		err = validate.Struct(v)
		if err != nil {

			if _, ok := err.(*validator.InvalidValidationError); ok {
				log.Err(err).Msg("")
				temp := model.TbSecurityGroupInfo{}
				return temp, err
			}

			temp := model.TbSecurityGroupInfo{}
			return temp, err
		}

		req[i].IPProtocol = strings.ToUpper(req[i].IPProtocol)
		req[i].Direction = strings.ToLower(req[i].Direction)
	}

	parentResourceType := model.StrSecurityGroup

	check, err := CheckResource(nsId, parentResourceType, securityGroupId)

	if !check {
		temp := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup %s does not exist.", securityGroupId)
		return temp, err
	}

	if err != nil {
		temp := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("Failed to check the existence of the securityGroup %s.", securityGroupId)
		return temp, err
	}

	securityGroupKey := common.GenResourceKey(nsId, model.StrSecurityGroup, securityGroupId)
	securityGroupKeyValue, _ := kvstore.GetKv(securityGroupKey)
	oldSecurityGroup := model.TbSecurityGroupInfo{}
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
				err := fmt.Errorf("One of submitted firewall rules already exists in the SG %s.", securityGroupId)
				return oldSecurityGroup, err
			}
		}
	}

	var tempSpiderSecurityInfo *model.SpiderSecurityInfo

	if objectOnly == false { // then, call CB-Spider CreateSecurityRule API
		requestBody := model.SpiderSecurityRuleReqInfoWrapper{}
		requestBody.ConnectionName = oldSecurityGroup.ConnectionName
		for _, newRule := range req {
			requestBody.ReqInfo.RuleInfoList = append(requestBody.ReqInfo.RuleInfoList, model.SpiderSecurityRuleInfo(newRule)) // Is this really works?
		}

		url := fmt.Sprintf("%s/securitygroup/%s/rules", model.SpiderRestUrl, oldSecurityGroup.CspResourceName)

		client := resty.New().SetCloseConnection(true)

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(requestBody).
			SetResult(&model.SpiderSecurityInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Post(url)

		if err != nil {
			log.Error().Err(err).Msg("")
			content := model.TbSecurityGroupInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			log.Error().Err(err).Msg("")
			content := model.TbSecurityGroupInfo{}
			return content, err
		}

		tempSpiderSecurityInfo = resp.Result().(*model.SpiderSecurityInfo)

	}

	log.Info().Msg("POST CreateFirewallRule")

	newSecurityGroup := model.TbSecurityGroupInfo{}
	newSecurityGroup = oldSecurityGroup
	newSecurityGroup.FirewallRules = nil

	for _, newSpiderSecurityRule := range tempSpiderSecurityInfo.SecurityRules {
		newSecurityGroup.FirewallRules = append(newSecurityGroup.FirewallRules, model.TbFirewallRuleInfo(newSpiderSecurityRule))
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
	// content := model.TbSecurityGroupInfo{}
	// err = json.Unmarshal([]byte(keyValue.Value), &content)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return err
	// }
	return newSecurityGroup, nil
}

// DeleteFirewallRules accepts firewallRule creation request, creates and returns an TB securityGroup object
func DeleteFirewallRules(nsId string, securityGroupId string, req []model.TbFirewallRuleInfo) (model.TbSecurityGroupInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.TbSecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(securityGroupId)
	if err != nil {
		temp := model.TbSecurityGroupInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	// Validate each TbFirewallRule
	for i, v := range req {
		err = validate.Struct(v)
		if err != nil {

			if _, ok := err.(*validator.InvalidValidationError); ok {
				log.Err(err).Msg("")
				temp := model.TbSecurityGroupInfo{}
				return temp, err
			}

			temp := model.TbSecurityGroupInfo{}
			return temp, err
		}

		req[i].IPProtocol = strings.ToUpper(req[i].IPProtocol)
		req[i].Direction = strings.ToLower(req[i].Direction)
	}

	parentResourceType := model.StrSecurityGroup

	check, err := CheckResource(nsId, parentResourceType, securityGroupId)

	if !check {
		temp := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup %s does not exist.", securityGroupId)
		return temp, err
	}

	if err != nil {
		temp := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("Failed to check the existence of the securityGroup %s.", securityGroupId)
		return temp, err
	}

	securityGroupKey := common.GenResourceKey(nsId, model.StrSecurityGroup, securityGroupId)
	securityGroupKeyValue, _ := kvstore.GetKv(securityGroupKey)
	oldSecurityGroup := model.TbSecurityGroupInfo{}
	err = json.Unmarshal([]byte(securityGroupKeyValue.Value), &oldSecurityGroup)
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	// Return error if one or more of provided rules does not exist.
	oldSGsFirewallRules := oldSecurityGroup.FirewallRules

	found_flag := false

	rulesToDelete := []model.TbFirewallRuleInfo{}

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
			requestBody.ReqInfo.RuleInfoList = append(requestBody.ReqInfo.RuleInfoList, model.SpiderSecurityRuleInfo(v))
		}
	}

	url := fmt.Sprintf("%s/securitygroup/%s/rules", model.SpiderRestUrl, oldSecurityGroup.CspResourceName)

	client := resty.New().SetCloseConnection(true)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		SetResult(&SpiderDeleteSecurityRulesResp{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Delete(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
		return content, err
	}

	spiderDeleteSecurityRulesResp = resp.Result().(*SpiderDeleteSecurityRulesResp)

	if spiderDeleteSecurityRulesResp.Result != "true" {
		err := fmt.Errorf("Failed to delete Security Group rules with CB-Spider.")
		log.Error().Err(err).Msg("")
		return oldSecurityGroup, err
	}

	requestBody2 := model.SpiderConnectionName{}
	requestBody2.ConnectionName = oldSecurityGroup.ConnectionName

	var tempSpiderSecurityInfo *model.SpiderSecurityInfo

	url = fmt.Sprintf("%s/securitygroup/%s", model.SpiderRestUrl, oldSecurityGroup.CspResourceName)

	client = resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	resp, err = client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody2).
		SetResult(&model.SpiderSecurityInfo{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Get(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	// fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := model.TbSecurityGroupInfo{}
		return content, err
	}

	tempSpiderSecurityInfo = resp.Result().(*model.SpiderSecurityInfo)

	log.Info().Msg("DELETE FirewallRule")

	newSecurityGroup := model.TbSecurityGroupInfo{}
	newSecurityGroup = oldSecurityGroup
	newSecurityGroup.FirewallRules = nil
	for _, newSpiderSecurityRule := range tempSpiderSecurityInfo.SecurityRules {
		newSecurityGroup.FirewallRules = append(newSecurityGroup.FirewallRules, model.TbFirewallRuleInfo(newSpiderSecurityRule))
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
	// content := model.TbSecurityGroupInfo{}
	// err = json.Unmarshal([]byte(keyValue.Value), &content)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return err
	// }
	return newSecurityGroup, nil
}
