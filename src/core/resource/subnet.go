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
	"strings"

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

	// Validate zone in each subnet
	parts := strings.SplitN(existingVNet.ConnectionName, "-", 2)
	provider := parts[0]
	region := parts[1]
	regionDetail, err := common.GetRegion(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	zones := regionDetail.Zones

	if subnetReq.Zone != "" {
		if !ContainsZone(zones, subnetReq.Zone) {
			err := fmt.Errorf("invalid zone: %s", subnetReq.Zone)
			log.Error().Err(err).Msg("")
			return err
		}
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

type spiderAddSubnetRequest struct {
	ConnectionName  string                     `json:"ConnectionName" validate:"required" example:"aws-connection"`
	IDTransformMode string                     `json:"IDTransformMode,omitempty" validate:"omitempty" example:"ON"` // ON: transform CSP ID, OFF: no-transform CSP ID
	ReqInfo         spiderAddSubnetRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderAddSubnetRequestInfo struct {
	Name      string           `json:"Name" validate:"required" example:"subnet-01"`
	Zone      string           `json:"Zone,omitempty" validate:"omitempty" example:"us-east-1b"` // target zone for the subnet, if not specified, it will be created in the same zone as the Connection.
	IPv4_CIDR string           `json:"IPv4_CIDR" validate:"required" example:"10.0.12.0/22"`
	TagList   []model.KeyValue `json:"TagList,omitempty" validate:"omitempty"`
}

// SubnetRegisterRequest represents the request body for registering a subnet.
type spiderSubnetRegisterRequest struct {
	ConnectionName string                          `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        spiderSubnetRegisterRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderSubnetRegisterRequestInfo struct {
	Name    string `json:"Name" validate:"required" example:"subnet-01"`
	Zone    string `json:"Zone,omitempty" validate:"omitempty" example:"us-east-1a"`
	VPCName string `json:"VPCName" validate:"required" example:"vpc-01"`
	CSPId   string `json:"CSPId" validate:"required" example:"csp-subnet-1234"`
}

type spiderSubnetUnregisterRequest struct {
	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        struct {
		VPCName string `json:"VPCName" validate:"required" example:"vpc-01"`
	} `json:"ReqInfo" validate:"required"`
}

type spiderSubnetRemoveReq struct {
	ConnectionName string // Connection name for the cloud provider
}

// type spiderCspSubnetRemoveReq struct {
// 	ConnectionName string // Connection name for the cloud provider
// }

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
func CreateSubnet(nsId string, vNetId string, subnetReq *model.TbSubnetReq) (model.TbSubnetInfo, error) {
	log.Info().Msg("CreateSubnet")

	log.Debug().Msgf("nsId: %s", nsId)
	log.Debug().Msgf("vNetId: %s", vNetId)
	log.Debug().Msgf("subnetReq: %+v", subnetReq)

	// subnet objects
	var emptyRet model.TbSubnetInfo
	var vNetInfo model.TbVNetInfo
	var subnetInfo model.TbSubnetInfo
	var err error = nil
	subnetInfo.Id = subnetReq.Name
	subnetInfo.Name = subnetReq.Name

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = validate.Struct(subnetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the subnet already exists
	exists, err := CheckChildResource(nsId, resourceType, vNetId, subnetInfo.Id)
	if exists {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("already exists, subnet: %s", subnetInfo.Id)
		return emptyRet, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the subnet (%s) exists or not", subnetInfo.Id)
		return emptyRet, err
	}

	// Set vNet and subnet keys
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetInfo.Id)

	// Read the saved vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// vNet object
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Validate that the requested subnet can be added to the vNet
	err = ValidateSubnetReq(subnetReq, vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set subnet object
	uid := common.GenUid()
	subnetInfo.Uid = uid
	subnetInfo.ResourceType = resourceType
	subnetInfo.ConnectionName = vNetInfo.ConnectionName
	subnetInfo.CspVNetId = vNetInfo.CspResourceId
	subnetInfo.CspVNetHandlingId = vNetInfo.CspResourceName

	// Set status to 'Configuring'
	subnetInfo.Status = string(NetworkOnConfiguring)
	// Save the status
	val, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(subnetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Add subnet
	spReqt := spiderAddSubnetRequest{}
	spReqt.ConnectionName = vNetInfo.ConnectionName
	spReqt.IDTransformMode = "OFF"
	spReqt.ReqInfo.Name = subnetInfo.Uid
	spReqt.ReqInfo.Zone = subnetReq.Zone
	spReqt.ReqInfo.IPv4_CIDR = subnetReq.IPv4_CIDR
	// todo: restore the tag list later
	// spReqt.ReqInfo.TagList = subnetReq.TagList

	client := resty.New()
	method := "POST"
	var spResp spiderVPCInfo

	// API to create a subnet
	url := fmt.Sprintf("%s/vpc/%s/subnet", model.SpiderRestUrl, vNetInfo.CspResourceName)

	// Defer function to ensure cleanup object
	defer func() {
		// Only if this operation fails, the subnet will be deleted
		if err != nil && subnetInfo.Status == string(NetworkOnConfiguring) {
			if subnetInfo.CspResourceId == "" { // Delete the saved the subnet info
				log.Warn().Msgf("failed to create subnet, cleaning up the subnet info: %v", subnetInfo.Id)
				deleteErr := kvstore.Delete(subnetKey)
				if deleteErr != nil {
					log.Warn().Err(deleteErr).Msgf("failed to delete the subnet info: %v from kvstore", subnetInfo.Id)
				}
			}
			// todo: check if the following operation is obviously required or not
			// else { // Delete the subnet from the CSP
			// 	// [Via Spider] Delete the subnet
			// 	_, deleteErr := DeleteSubnet(nsId, vNetId, subnetInfo.Id)
			// 	if deleteErr != nil {
			// 		log.Warn().Err(err).Msgf("failed to delete the subnet: %v from CSP", subnetInfo.Id)
			// 	}
			// }
		}
	}()

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
		return emptyRet, err
	}

	// Search the requested subnet in the response from the Spider
	for _, spSubnetInfo := range spResp.SubnetInfoList {
		if subnetInfo.Uid == spSubnetInfo.IId.NameId {
			// Set the subnet object with the response from the Spider
			subnetInfo.CspResourceId = spSubnetInfo.IId.SystemId
			subnetInfo.CspResourceName = spSubnetInfo.IId.NameId
			subnetInfo.IPv4_CIDR = spSubnetInfo.IPv4_CIDR
			subnetInfo.Zone = spSubnetInfo.Zone
			// todo: restore the tag list later
			// subnetInfo.TagList = spSubnetInfo.TagList
			subnetInfo.KeyValueList = spSubnetInfo.KeyValueList
			break
		}
	}

	// Set status to 'Available'
	subnetInfo.Status = string(NetworkAvailable)

	log.Debug().Msgf("subnetInfo: %+v", subnetInfo)

	// Save subnet object into the key-value store
	subnetObj, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(subnetKey, string(subnetObj))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
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
	vNetObj, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(vNetObj))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"sys.manager":         model.StrManager,
		"sys.namespace":       nsId,
		"sys.labelType":       model.StrSubnet,
		"sys.id":              subnetInfo.Id,
		"sys.name":            subnetInfo.Name,
		"sys.uid":             subnetInfo.Uid,
		"sys.cspResourceId":   subnetInfo.CspResourceId,
		"sys.cspResourceName": subnetInfo.CspResourceName,
		"sys.ipv4_CIDR":       subnetInfo.IPv4_CIDR,
		"sys.zone":            subnetInfo.Zone,
		"sys.status":          subnetInfo.Status,
		"sys.vNetId":          vNetInfo.Id,
		"sys.cspvNetId":       vNetInfo.CspResourceId,
		"sys.cspvNetName":     vNetInfo.CspResourceName,
		"sys.description":     subnetInfo.Description,
		"sys.connectionName":  subnetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrSubnet, uid, subnetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return subnetInfo, nil
}

// GetSubnet
func GetSubnet(nsId string, vNetId string, subnetId string) (model.TbSubnetInfo, error) {

	// subnet objects
	var emptyRet model.TbSubnetInfo
	var subnetInfo model.TbSubnetInfo

	// Set the resource type
	// parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Todo: Change the below code when Spider provides the API to get the subnet info from CSP
	// Set a key for the subnet object
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// Read the stored subnet info
	subnetKeyValue, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if subnetKeyValue == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, subnet: %s", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// subnet object
	err = json.Unmarshal([]byte(subnetKeyValue.Value), &subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return subnetInfo, nil
}

// ListSubnet
func ListSubnet(nsId string, vNetId string) ([]model.TbSubnetInfo, error) {

	// subnet objects
	var emptyRet []model.TbSubnetInfo
	var subnetInfoList []model.TbSubnetInfo

	// Set the resource type
	// parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)
	subnetPrefixKey := vNetKey + "/subnet"
	// Read the stored subnets
	subnetsKv, err := kvstore.GetKvList(subnetPrefixKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("subnetsKv: %+v", subnetsKv)

	// subnet object
	for _, kv := range subnetsKv {
		var subnetInfo model.TbSubnetInfo
		err = json.Unmarshal([]byte(kv.Value), &subnetInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		subnetInfoList = append(subnetInfoList, subnetInfo)
	}

	return subnetInfoList, nil
}

// DeleteSubnet deletes and returns the result
func DeleteSubnet(nsId string, vNetId string, subnetId string) (model.SimpleMsg, error) {

	// subnet objects
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set a key for the subnet object
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// Read the stored vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// vNet object
	var vNetInfo model.TbVNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored subnet info
	subnetKeyValue, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if subnetKeyValue == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, subnet: %s", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// subnet object
	var subnetInfo model.TbSubnetInfo
	err = json.Unmarshal([]byte(subnetKeyValue.Value), &subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Todo: Check if the subnet is being used by any resouces, such as virtual machines, gateways, etc.
	// Check if the vNet has subnets or not
	if subnetInfo.Status == string(NetworkInUse) {
		err := fmt.Errorf("the subnet (%s) is in-use, may have any resources", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set status to 'Deleting'
	subnetInfo.Status = string(NetworkOnDeleting)
	// Save the status
	val, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(subnetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Delete the subnet
	spReqt := spiderSubnetRemoveReq{}
	spReqt.ConnectionName = subnetInfo.ConnectionName

	// API to delete a subnet
	url := fmt.Sprintf("%s/vpc/%s/subnet/%s", model.SpiderRestUrl, subnetInfo.CspVNetHandlingId, subnetInfo.CspResourceName)

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
		return emptyRet, err
	}
	ok, err := strconv.ParseBool(spResp.Result)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !ok {
		err := fmt.Errorf("failed to delete the subnet (%s)", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Delete the saved the subnet info
	err = kvstore.Delete(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
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
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	// labels := map[string]string{
	// 	"sys.manager":  model.StrManager,
	// 	"namespace": nsId,
	// }
	err = label.DeleteLabelObject(model.StrSubnet, subnetInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the subnet (%s) has been deleted", subnetId)

	return ret, nil
}

func RegisterSubnet(nsId string, vNetId string, subnetReq *model.TbRegisterSubnetReq) (model.TbSubnetInfo, error) {

	// subnet objects
	var emptyRet model.TbSubnetInfo
	var vNetInfo model.TbVNetInfo
	var subnetInfo model.TbSubnetInfo
	var err error = nil

	// Set the subnet object
	subnetInfo.Id = subnetReq.Name
	subnetInfo.Name = subnetReq.Name

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet
	subnetInfo.ResourceType = resourceType

	// Validate the input parameters
	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = validate.Struct(subnetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the subnet already exists
	exists, err := CheckChildResource(nsId, resourceType, vNetId, subnetInfo.Id)
	if exists {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("already exists, subnet: %s", subnetInfo.Id)
		return emptyRet, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the subnet (%s) exists or not", subnetInfo.Id)
		return emptyRet, err
	}

	// Set vNet and subnet keys
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetInfo.Id)

	// Read the saved vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// vNet object
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set subnet object
	uid := common.GenUid()
	subnetInfo.Uid = uid
	subnetInfo.ConnectionName = vNetInfo.ConnectionName
	subnetInfo.CspVNetId = vNetInfo.CspResourceId
	subnetInfo.CspVNetHandlingId = vNetInfo.CspResourceName

	// Set status to 'Registering'
	subnetInfo.Status = string(NetworkOnRegistering)
	// Save the status
	val, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(subnetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Register subnet
	spReqt := spiderSubnetRegisterRequest{}
	spReqt.ConnectionName = subnetReq.ConnectionName
	spReqt.ReqInfo.Name = uid
	spReqt.ReqInfo.VPCName = vNetInfo.CspResourceName
	spReqt.ReqInfo.Zone = subnetReq.Zone
	spReqt.ReqInfo.CSPId = subnetReq.CspResourceId

	client := resty.New()
	method := "POST"
	var spResp spiderSubnetInfo

	// API to register a subnet from CSP
	url := fmt.Sprintf("%s/regsubnet", model.SpiderRestUrl)
	// [Note] Spider doesn't provide "GET /vpc{VPCName}/subnet" API

	// Defer function to ensure cleanup object
	defer func() {
		// Only if this operation fails, the subnet will be deleted
		if err != nil && subnetInfo.Status == string(NetworkOnRegistering) {
			if subnetInfo.CspResourceId == "" { // Delete the saved the subnet info
				log.Warn().Msgf("failed to create subnet, cleaning up the subnet info: %v", subnetInfo.Id)
				deleteErr := kvstore.Delete(subnetKey)
				if deleteErr != nil {
					log.Warn().Err(deleteErr).Msgf("failed to delete the subnet info: %v from kvstore", subnetInfo.Id)
				}
			}
			// todo: check if the following operation is obviously required or not
			// else { // Delete the subnet from the CSP
			// 	// [Via Spider] Delete the subnet
			// 	_, deregisterErr := DeregisterSubnet(nsId, vNetId, subnetInfo.Id)
			// 	if deregisterErr != nil {
			// 		log.Warn().Err(err).Msgf("failed to deregister the subnet: %v from CSP", subnetInfo.Id)
			// 	}
			// }
		}
	}()

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
		return emptyRet, err
	}

	// Set the subbet object with the response from the Spider
	subnetInfo.CspResourceId = spResp.IId.SystemId
	subnetInfo.CspResourceName = spResp.IId.NameId
	subnetInfo.IPv4_CIDR = spResp.IPv4_CIDR
	subnetInfo.Zone = spResp.Zone
	subnetInfo.KeyValueList = spResp.KeyValueList
	// todo: restore the tag list later
	// subnetInfo.TagList = spResp.TagList

	// Set status to 'Available'
	subnetInfo.Status = string(NetworkAvailable)

	log.Debug().Msgf("subnetInfo: %+v", subnetInfo)

	// Save subnet object into the key-value store
	subnetObj, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(subnetKey, string(subnetObj))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
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
	vNetObj, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(vNetObj))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"sys.manager":         model.StrManager,
		"sys.namespace":       nsId,
		"sys.labelType":       model.StrSubnet,
		"sys.id":              subnetInfo.Id,
		"sys.name":            subnetInfo.Name,
		"sys.uid":             subnetInfo.Uid,
		"sys.cspResourceId":   subnetInfo.CspResourceId,
		"sys.cspResourceName": subnetInfo.CspResourceName,
		"sys.ipv4_CIDR":       subnetInfo.IPv4_CIDR,
		"sys.zone":            subnetInfo.Zone,
		"sys.status":          subnetInfo.Status,
		"sys.vNetId":          vNetInfo.Id,
		"sys.cspvNetId":       vNetInfo.CspResourceId,
		"sys.cspvNetName":     vNetInfo.CspResourceName,
		"sys.description":     subnetInfo.Description,
		"sys.connectionName":  subnetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrSubnet, uid, vNetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return subnetInfo, nil
}

// DeregisterSubnet deregister subnet and returns the result
func DeregisterSubnet(nsId string, vNetId string, subnetId string) (model.SimpleMsg, error) {

	// subnet objects
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set a key for the subnet object
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// Read the saved vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// vNet object
	var vNetInfo model.TbVNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored subnet info
	subnetKv, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if subnetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, subnet: %s", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// subnet object
	var subnetInfo model.TbSubnetInfo
	err = json.Unmarshal([]byte(subnetKv.Value), &subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Todo: Check if the subnet is being used by any resouces, such as virtual machines, gateways, etc.
	// Check if the vNet has subnets or not
	if subnetInfo.Status == string(NetworkInUse) {
		err := fmt.Errorf("the subnet (%s) is in-use, may have any resources", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set status to 'Derigistering'
	subnetInfo.Status = string(NetworkOnDeregistering)
	// Save the status
	val, err := json.Marshal(subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(subnetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Deregister the subnet
	spReqt := spiderSubnetUnregisterRequest{}
	spReqt.ConnectionName = subnetInfo.ConnectionName
	spReqt.ReqInfo.VPCName = subnetInfo.CspVNetHandlingId

	// API to deregister subnet
	url := fmt.Sprintf("%s/regsubne/%s", model.SpiderRestUrl, subnetInfo.CspResourceName)

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
		return emptyRet, err
	}
	ok, err := strconv.ParseBool(spResp.Result)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !ok {
		err := fmt.Errorf("failed to deregister the subnet (%s)", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Delete the saved the subnet info
	err = kvstore.Delete(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
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
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	// labels := map[string]string{
	// 	"sys.manager":  model.StrManager,
	// 	"namespace": nsId,
	// }

	err = label.DeleteLabelObject(model.StrSubnet, subnetInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the subnet (%s) has been deregistered", subnetId)

	return ret, nil
}

/*
The following functions are used for Designing VNets
*/

// GetFirstNZones returns the first N zones of the given connection
func GetFirstNZones(connectionName string, firstN int) ([]string, int, error) {

	// Splite the connectionName into provider and region
	parts := strings.SplitN(connectionName, "-", 2)
	provider := parts[0]
	region := parts[1]

	// Get the region details
	regionDetail, err := common.GetRegion(provider, region)
	if err != nil {
		return nil, 0, err
	}

	// Get the first N zones
	zones := regionDetail.Zones
	length := len(zones)
	if length > firstN {
		zones = zones[:firstN]
		length = firstN
	}

	if zones == nil || len(zones) == 0 {
		return nil, 0, nil
	}

	return zones, length, nil
}
