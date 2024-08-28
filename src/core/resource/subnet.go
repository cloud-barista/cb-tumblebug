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
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
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

func ValidateSubnetReq(subnetReq *model.TbSubnetReq, existingVNet model.TbVNetInfo) error {
	log.Debug().Msg("ValidateSubnetReq")
	log.Debug().Msgf("Subnet: %+v", subnetReq)

	err := validate.Struct(subnetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return err
		}
		return err
	}

	// A network object for validation
	var network netutil.Network
	var subnets []netutil.Network

	network = netutil.Network{
		CidrBlock: existingVNet.CidrBlock,
	}
	for _, subnetInfo := range existingVNet.SubnetInfoList {
		subnet := netutil.Network{
			CidrBlock: subnetInfo.IPv4_CIDR,
		}
		subnets = append(subnets, subnet)
	}
	subnet := netutil.Network{
		CidrBlock: subnetReq.IPv4_CIDR,
	}
	subnets = append(subnets, subnet)

	network.Subnets = subnets
	log.Debug().Msgf("network: %+v", network)

	// Validate the network object
	err = netutil.ValidateNetwork(network)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// The spiderXxx structs are used to call the Spider REST API
// Ref:
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/api-runtime/rest-runtime/VPC-SubnetRest.go
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

// Synchronized the request body with the Spider API

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
	Zone         string           // Zone of the Subnet
	IPv4_CIDR    string           // CIDR block of the Subnet
	TagList      []model.KeyValue // List of key-value tags for the Subnet
	KeyValueList []model.KeyValue // List of key-value pairs indicating CSP-side response
	// Name         string           // Name of the Subnet
}

// CreateSubnet creates and returns the vNet object
func CreateSubnet(nsId string, vNetId string, subnetReq model.TbSubnetReq) (model.TbSubnetInfo, error) {

	// subnet objects
	var emptyResp model.TbSubnetInfo
	var vNetInfo model.TbVNetInfo
	var subnetInfo model.TbSubnetInfo
	subnetInfo.Id = subnetReq.Name
	subnetInfo.Name = subnetReq.Name

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	err = validate.Struct(subnetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyResp, err
		}

		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Check if the subnet already exists
	exists, err := CheckChildResource(nsId, resourceType, vNetId, subnetInfo.Id)
	if exists {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("already exists, subnet: %s", subnetInfo.Id)
		return emptyResp, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the subnet (%s) exists or not", subnetInfo.Id)
		return emptyResp, err
	}

	// Set vNet and subnet keys
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetInfo.Id)

	// Read the stored vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	// vNet object
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Validate that the requested subnet can be added to the vNet
	err = ValidateSubnetReq(&subnetReq, vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Set subnet object
	uuid := common.GenUid()
	subnetInfo.Uuid = uuid
	subnetInfo.ConnectionName = vNetInfo.ConnectionName
	subnetInfo.CspVNetId = vNetInfo.CspVNetId
	subnetInfo.CspVNetName = vNetInfo.CspVNetName

	// Set status to 'Configuring'
	subnetInfo.Status = string(NetworkOnConfiguring)
	// Save the status
	val, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	err = kvstore.Put(subnetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// [Via Spider] Add subnet
	spReqt := spiderSubnetAddReq{}
	spReqt.ConnectionName = vNetInfo.ConnectionName
	spReqt.IDTransformMode = "OFF"
	spReqt.ReqInfo.Name = uuid
	spReqt.ReqInfo.Zone = subnetReq.Zone
	spReqt.ReqInfo.IPv4_CIDR = subnetReq.IPv4_CIDR
	spReqt.ReqInfo.TagList = subnetReq.TagList

	client := resty.New()
	method := "POST"
	var spResp spiderSubnetInfo

	// API to create a vNet
	url := fmt.Sprintf("%s/vpc/%s/subnet", model.SpiderRestUrl, vNetInfo.CspVNetName)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Set the subent object with the response from the Spider
	subnetInfo.CspSubnetId = spResp.IId.SystemId
	subnetInfo.CspSubnetName = spResp.IId.NameId
	subnetInfo.IPv4_CIDR = spResp.IPv4_CIDR
	subnetInfo.Zone = spResp.Zone
	subnetInfo.TagList = spResp.TagList
	subnetInfo.KeyValueList = spResp.KeyValueList

	// Set status to 'Available'
	subnetInfo.Status = string(NetworkAvailable)

	log.Debug().Msgf("subnetInfo: %+v", subnetInfo)

	// Save subnet object into the key-value store
	value, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	err = kvstore.Put(subnetKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Update and save vNet object
	vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList, subnetInfo)

	if len(vNetInfo.SubnetInfoList) == 0 {
		vNetInfo.Status = string(NetworkAvailable)
	} else if len(vNetInfo.SubnetInfoList) > 0 {
		vNetInfo.Status = string(NetworkInUse)
	} else {
		vNetInfo.Status = string(NetworkUnknown)
		log.Warn().Msgf("The status of the vNet (%s) is unknown", vNetInfo.Id)
	}

	log.Debug().Msgf("vNetInfo: %+v", vNetInfo)

	// Save vNet object into the key-value store
	value, err = json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	err = kvstore.Put(vNetKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrSubnet, uuid, vNetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	return subnetInfo, nil
}

// DeleteSubnet deletes and returns the result
func DeleteSubnet(nsId string, vNetId string, subnetId string) (model.SimpleMsg, error) {

	// subnet objects
	var emptyResp model.SimpleMsg
	var resp model.SimpleMsg

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	err = common.CheckString(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Set a key for the subnet object
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// Read the stored vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	// vNet object
	var vNetInfo model.TbVNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Read the stored subnet info
	subnetKeyValue, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	if subnetKeyValue == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, subnet: %s", subnetId)
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	// subnet object
	var subnetInfo model.TbSubnetInfo
	err = json.Unmarshal([]byte(subnetKeyValue.Value), &subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Todo: Check if the subnet is being used by any resouces, such as virtual machines, gateways, etc.
	// Check if the vNet has subnets or not
	if subnetInfo.Status == string(NetworkInUse) {
		err := fmt.Errorf("the subnet (%s) is in-use, may have any resources", subnetId)
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Set status to 'Deleting'
	subnetInfo.Status = string(NetworkOnDeleting)
	// Save the status
	val, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	err = kvstore.Put(subnetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// [Via Spider] Delete the subnet
	spReqt := spiderSubnetRemoveReq{}
	spReqt.ConnectionName = subnetInfo.ConnectionName

	// API to delete a vNet
	url := fmt.Sprintf("%s/vpc/%s/subnet/%s", model.SpiderRestUrl, subnetInfo.CspVNetName, subnetInfo.CspSubnetName)

	var spResp spiderBooleanInfoResp

	client := resty.New()
	method := "DELETE"

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	ok, err := strconv.ParseBool(spResp.Result)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	if !ok {
		err := fmt.Errorf("failed to delete the subnet (%s)", subnetId)
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Delete the saved the subnet info
	err = kvstore.Delete(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// Update the vNet info
	for i, s := range vNetInfo.SubnetInfoList {
		if s.Id == subnetId {
			vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList[:i], vNetInfo.SubnetInfoList[i+1:]...)
			break
		}
	}
	if len(vNetInfo.SubnetInfoList) == 0 {
		vNetInfo.Status = string(NetworkAvailable)
	}

	// Save the updated vNet info
	val, err = json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyResp, err
	}

	// [Output] the message
	resp.Message = fmt.Sprintf("the subnet (%s) has been deleted", subnetId)

	return resp, nil
}
