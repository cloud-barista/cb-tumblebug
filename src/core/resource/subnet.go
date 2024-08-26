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
	"strconv"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// TbSubnetReqStructLevelValidation is a function to validate 'TbSubnetReq' object.
func TbSubnetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbSubnetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// The spiderXxx structs are used to call the Spider REST API
// Ref:
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/api-runtime/rest-runtime/VPC-SubnetRest.go
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

// Synchronized the request body with the Spider API
type spiderSubnetRegisterReq struct {
	ConnectionName string // Connection name for the cloud provider
	ReqInfo        spiderSubnetRegisterReqInfo
}

type spiderSubnetRegisterReqInfo struct {
	Name    string // Name of the Subnet
	Zone    string // Zone of the Subnet
	VPCName string // VPC Name
	CSPId   string // CSP ID of the Subnet
}

type spiderSubnetUnregisterReq struct {
	ConnectionName string // Connection name for the cloud provider
	ReqInfo        spiderSubnetUnregisterReqInfo
}

type spiderSubnetUnregisterReqInfo struct {
	VPCName string // VPC Name
}

type spiderSubnetAddReq struct {
	ConnectionName  string // Connection name for the cloud provider
	IDTransformMode string // ON | OFF, default is ON
	ReqInfo         spiderSubnetAddReqInfo
}

type spiderSubnetAddReqInfo struct {
	Name      string // Name of the Subnet
	Zone      string // Zone of the Subnet
	IPv4_CIDR string // CIDR block of the Subnet
	TagList   []model.KeyValue
}

type spiderSubnetRemoveReq struct {
	ConnectionName string // Connection name for the cloud provider
}

type spiderCspSubnetRemoveReq struct {
	ConnectionName string // Connection name for the cloud provider
}

// The spiderSubnetInfo struct is a union of the properties in
// Spider's 'subnetRegisterReq', 'req' in AddSubnet(), and 'SubnetInfo' structs.
type spiderSubnetInfo struct {
	IId          model.IID        // {NameId, SystemId}
	Name         string           // Name of the Subnet
	Zone         string           // Zone of the Subnet
	IPv4_CIDR    string           // CIDR block of the Subnet
	TagList      []model.KeyValue // List of key-value tags for the Subnet
	KeyValueList []model.KeyValue // List of key-value pairs indicating CSP-side response
}

// CreateSubnet accepts subnet creation request, creates and returns an TB vNet object
func CreateSubnet(nsId string, vNetId string, req model.TbSubnetReq, objectOnly bool) (model.TbVNetInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.TbVNetInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(vNetId)
	if err != nil {
		temp := model.TbVNetInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = validate.Struct(req)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			temp := model.TbVNetInfo{}
			return temp, err
		}

		temp := model.TbVNetInfo{}
		return temp, err
	}

	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	check, err := CheckResource(nsId, parentResourceType, vNetId)

	if !check {
		temp := model.TbVNetInfo{}
		err := fmt.Errorf("The vNet " + vNetId + " does not exist.")
		return temp, err
	}

	if err != nil {
		temp := model.TbVNetInfo{}
		err := fmt.Errorf("Failed to check the existence of the vNet " + vNetId + ".")
		return temp, err
	}

	check, err = CheckChildResource(nsId, resourceType, vNetId, req.Name)

	if check {
		temp := model.TbVNetInfo{}
		err := fmt.Errorf("The subnet " + req.Name + " already exists.")
		return temp, err
	}

	if err != nil {
		temp := model.TbVNetInfo{}
		err := fmt.Errorf("Failed to check the existence of the subnet " + req.Name + ".")
		return temp, err
	}

	uuid := common.GenUid()

	vNetKey := common.GenResourceKey(nsId, model.StrVNet, vNetId)
	vNetKeyValue, _ := kvstore.GetKv(vNetKey)
	oldVNet := model.TbVNetInfo{}
	err = json.Unmarshal([]byte(vNetKeyValue.Value), &oldVNet)
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldVNet, err
	}

	if objectOnly == false { // then, call CB-Spider CreateSubnet API
		requestBody := spiderSubnetAddReq{}
		requestBody.ConnectionName = oldVNet.ConnectionName
		requestBody.ReqInfo.Name = uuid
		requestBody.ReqInfo.IPv4_CIDR = req.IPv4_CIDR

		url := fmt.Sprintf("%s/vpc/%s/subnet", model.SpiderRestUrl, oldVNet.CspVNetName)

		client := resty.New().SetCloseConnection(true)

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(requestBody).
			SetResult(&spiderVpcInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Post(url)

		if err != nil {
			log.Error().Err(err).Msg("")
			content := model.TbVNetInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			log.Error().Err(err).Msg("")
			content := model.TbVNetInfo{}
			return content, err
		}

	}

	log.Info().Msg("POST CreateSubnet")
	SubnetKey := common.GenChildResourceKey(nsId, model.StrSubnet, vNetId, req.Name)
	Val, _ := json.Marshal(req)

	err = kvstore.Put(SubnetKey, string(Val))
	if err != nil {
		temp := model.TbVNetInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	newVNet := model.TbVNetInfo{}
	newVNet = oldVNet

	jsonBody, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	tbSubnetInfo := model.TbSubnetInfo{}
	err = json.Unmarshal(jsonBody, &tbSubnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	tbSubnetInfo.Id = req.Name
	tbSubnetInfo.Name = req.Name
	tbSubnetInfo.Uuid = uuid
	tbSubnetInfo.IdFromCsp = req.IdFromCsp

	newVNet.SubnetInfoList = append(newVNet.SubnetInfoList, tbSubnetInfo)
	Val, _ = json.Marshal(newVNet)

	err = kvstore.Put(vNetKey, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldVNet, err
	}
	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrSubnet, uuid, vNetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.TbVNetInfo{}, err
	}

	return newVNet, nil
}
