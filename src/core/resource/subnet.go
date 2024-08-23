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
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// CreateSubnet accepts subnet creation request, creates and returns an TB vNet object
func CreateSubnet(nsId string, vNetId string, req TbSubnetReq, objectOnly bool) (TbVNetInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := TbVNetInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(vNetId)
	if err != nil {
		temp := TbVNetInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = validate.Struct(req)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			temp := TbVNetInfo{}
			return temp, err
		}

		temp := TbVNetInfo{}
		return temp, err
	}

	parentResourceType := common.StrVNet
	resourceType := common.StrSubnet

	check, err := CheckResource(nsId, parentResourceType, vNetId)

	if !check {
		temp := TbVNetInfo{}
		err := fmt.Errorf("The vNet " + vNetId + " does not exist.")
		return temp, err
	}

	if err != nil {
		temp := TbVNetInfo{}
		err := fmt.Errorf("Failed to check the existence of the vNet " + vNetId + ".")
		return temp, err
	}

	check, err = CheckChildResource(nsId, resourceType, vNetId, req.Name)

	if check {
		temp := TbVNetInfo{}
		err := fmt.Errorf("The subnet " + req.Name + " already exists.")
		return temp, err
	}

	if err != nil {
		temp := TbVNetInfo{}
		err := fmt.Errorf("Failed to check the existence of the subnet " + req.Name + ".")
		return temp, err
	}

	vNetKey := common.GenResourceKey(nsId, common.StrVNet, vNetId)
	vNetKeyValue, _ := kvstore.GetKv(vNetKey)
	oldVNet := TbVNetInfo{}
	err = json.Unmarshal([]byte(vNetKeyValue.Value), &oldVNet)
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldVNet, err
	}

	if objectOnly == false { // then, call CB-Spider CreateSubnet API
		requestBody := SpiderSubnetReqInfoWrapper{}
		requestBody.ConnectionName = oldVNet.ConnectionName
		requestBody.ReqInfo.Name = common.GenUid()
		requestBody.ReqInfo.IPv4_CIDR = req.IPv4_CIDR

		url := fmt.Sprintf("%s/vpc/%s/subnet", common.SpiderRestUrl, oldVNet.CspVNetName)

		client := resty.New().SetCloseConnection(true)

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(requestBody).
			SetResult(&SpiderVPCInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Post(url)

		if err != nil {
			log.Error().Err(err).Msg("")
			content := TbVNetInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			log.Error().Err(err).Msg("")
			content := TbVNetInfo{}
			return content, err
		}

	}

	log.Info().Msg("POST CreateSubnet")
	SubnetKey := common.GenChildResourceKey(nsId, common.StrSubnet, vNetId, req.Name)
	Val, _ := json.Marshal(req)

	err = kvstore.Put(SubnetKey, string(Val))
	if err != nil {
		temp := TbVNetInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	newVNet := TbVNetInfo{}
	newVNet = oldVNet

	jsonBody, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	tbSubnetInfo := TbSubnetInfo{}
	err = json.Unmarshal(jsonBody, &tbSubnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	tbSubnetInfo.Id = req.Name
	tbSubnetInfo.Name = req.Name
	tbSubnetInfo.IdFromCsp = req.IdFromCsp

	newVNet.SubnetInfoList = append(newVNet.SubnetInfoList, tbSubnetInfo)
	Val, _ = json.Marshal(newVNet)

	err = kvstore.Put(vNetKey, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return oldVNet, err
	}

	return newVNet, nil
}
