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
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// SubnetReqStructLevelValidation is a function to validate 'SubnetReq' object.
func SubnetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.SubnetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

func ValidateSubnetReq(subnetReq *model.SubnetReq, existingVNet model.VNetInfo) error {
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
	// TODO: Update the validation logic
	// It's a temporary validation logic due to the connection name pattern
	// Split the connection name into provider and region/zone
	parts := strings.SplitN(existingVNet.ConnectionName, "-", 2)
	provider := parts[0]
	regionZone := parts[1]

	// Get the region list
	regionsObj, err := common.GetRegions(provider)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// Try to match and get the region detail
	var regionDetail model.RegionDetail
	for _, region := range regionsObj.Regions {
		exists := strings.HasPrefix(regionZone, region.RegionName)
		if exists {
			regionDetail = region
			break
		}
	}

	// Check if the region detail exists or not
	if regionDetail.RegionName == "" && len(regionDetail.Zones) == 0 {
		err := fmt.Errorf("invalid region/zone: %s", regionZone)
		log.Error().Err(err).Msg("")
		return err
	}

	// Validate the zone
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
func CreateSubnet(nsId string, vNetId string, subnetReq *model.SubnetReq) (model.SubnetInfo, error) {
	log.Info().Msg("CreateSubnet")

	log.Debug().Msgf("nsId: %s", nsId)
	log.Debug().Msgf("vNetId: %s", vNetId)
	log.Debug().Msgf("subnetReq: %+v", subnetReq)

	// subnet objects
	var emptyRet model.SubnetInfo
	var vNetInfo model.VNetInfo
	var subnetInfo model.SubnetInfo
	var err error = nil

	/*
	 *	Validate the input parameters
	 */

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

	subnetInfo.Id = subnetReq.Name
	subnetInfo.Name = subnetReq.Name

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

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
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
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
	subnetInfo.ConnectionConfig, err = common.GetConnConfig(subnetInfo.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}
	subnetInfo.CspVNetId = vNetInfo.CspResourceId
	subnetInfo.CspVNetName = vNetInfo.CspResourceName

	/*
	 *	Create a subnet
	 */

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

	client := clientManager.NewHttpClient()
	method := "POST"
	var spResp spiderVPCInfo

	// API to create a subnet
	url := fmt.Sprintf("%s/vpc/%s/subnet", model.SpiderRestUrl, vNetInfo.CspResourceName)

	log.Debug().Msgf("[Request to Spider] Creating Subnet (url: %s, request body: %+v)", url, spReqt)

	// Clean up the object when something goes wrong
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

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Creating Subnet (response body: %+v)", spResp)

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

	// [Set and store status]
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
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrSubnet,
		model.LabelId:              subnetInfo.Id,
		model.LabelName:            subnetInfo.Name,
		model.LabelUid:             subnetInfo.Uid,
		model.LabelCspResourceId:   subnetInfo.CspResourceId,
		model.LabelCspResourceName: subnetInfo.CspResourceName,
		model.LabelIpv4_CIDR:       subnetInfo.IPv4_CIDR,
		model.LabelZone:            subnetInfo.Zone,
		model.LabelStatus:          subnetInfo.Status,
		model.LabelVNetId:          vNetInfo.Id,
		model.LabelCspVNetId:       vNetInfo.CspResourceId,
		model.LabelCspVNetName:     vNetInfo.CspResourceName,
		model.LabelDescription:     subnetInfo.Description,
		model.LabelConnectionName:  subnetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrSubnet, uid, subnetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return subnetInfo, nil
}

// GetSubnet
func GetSubnet(nsId string, vNetId string, subnetId string) (model.SubnetInfo, error) {
	log.Info().Msg("GetSubnet")

	// subnet objects
	var emptyRet model.SubnetInfo
	var subnetInfo model.SubnetInfo

	/*
	 *	Validate the input parameters
	 */

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

	/*
	 *	Get the subnet info
	 */

	// Set the resource type
	// parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Set a key for the subnet object
	// vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// // Read the saved vNet info
	// vNetKv, err := kvstore.GetKv(vNetKey)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }
	// if vNetKv == (kvstore.KeyValue{}) {
	// 	err := fmt.Errorf("does not exist, vNet: %s", vNetId)
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }
	// // vNet object
	// err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }

	// Read the stored subnet info
	subnetKeyValue, exists, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
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

	// [Via Spider] Get a subnet
	client := clientManager.NewHttpClient()
	method := "GET"

	// API to get a subnet
	url := fmt.Sprintf("%s/vpc/%s/subnet/%s", model.SpiderRestUrl, subnetInfo.CspVNetName, subnetInfo.CspResourceName)
	queryParams := "?ConnectionName=" + subnetInfo.ConnectionName
	url += queryParams

	spReqt := clientManager.NoBody

	log.Debug().Msgf("[Request to Spider] Getting Subnet (url: %s, request body: %+v)", url, spReqt)

	var spResp spiderSubnetInfo

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Getting Subnet (response body: %+v)", spResp)

	if err != nil {
		log.Warn().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the subnet object with the response from the Spider
	subnetInfo.CspResourceId = spResp.IId.SystemId
	subnetInfo.CspResourceName = spResp.IId.NameId
	subnetInfo.IPv4_CIDR = spResp.IPv4_CIDR
	subnetInfo.Zone = spResp.Zone
	subnetInfo.KeyValueList = spResp.KeyValueList
	// TODO: restore the tag list later
	// subnetInfo.TagList = spResp.TagList

	// TODO: Check if it's required or not to save the subnet object

	return subnetInfo, nil
}

// ListSubnet
func ListSubnet(nsId string, vNetId string) ([]model.SubnetInfo, error) {
	log.Info().Msg("ListSubnet")

	// subnet objects
	var emptyRet []model.SubnetInfo
	var subnetInfoList []model.SubnetInfo

	/*
	 *	Validate the input parameters
	 */

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

	/*
	 *	Get the subnet info list
	 */

	// Use the GetVNet function to get the subnets info
	vNetInfo, err := GetVNet(nsId, vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	subnetInfoList = append(subnetInfoList, vNetInfo.SubnetInfoList...)

	return subnetInfoList, nil
}

// DeleteSubnet deletes and returns the result
func DeleteSubnet(nsId string, vNetId string, subnetId string, actionParam string) (model.SimpleMsg, error) {
	log.Info().Msg("DeleteSubnet")

	// subnet objects
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	/*
	 *	Validate the input parameters
	 */

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

	action, vaild := ParseNetworkAction(actionParam)

	// Validate options: withSubnets
	if !vaild {
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Msgf(errMsg.Error())
		return emptyRet, errMsg
	}

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Set a key for the subnet object
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// Read the stored vNet info
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// vNet object
	var vNetInfo model.VNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored subnet info
	subnetKeyValue, exists, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, subnet: %s", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// subnet object
	var subnetInfo model.SubnetInfo
	err = json.Unmarshal([]byte(subnetKeyValue.Value), &subnetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Todo: Check if the subnet is being used by any resouces, such as virtual machines, gateways, etc.
	// Check if the vNet has subnets or not
	if action == ActionNone && subnetInfo.Status == string(NetworkInUse) {
		err := fmt.Errorf("the subnet (%s) is in-use, may have any resources", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	/*
	 *	Delete the subnet
	 */

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
	url := fmt.Sprintf("%s/vpc/%s/subnet/%s", model.SpiderRestUrl, subnetInfo.CspVNetName, subnetInfo.CspResourceName)
	queryParams := ""
	if action == ActionForce {
		queryParams = "?force=true"
	}
	url += queryParams

	log.Debug().Msgf("[Request to Spider] Deleting Subnet (url: %s, request body: %+v)", url, spReqt)

	var spResp spiderBooleanInfoResp

	client := clientManager.NewHttpClient()
	method := "DELETE"

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Deleting Subnet (response body: %+v)", spResp)

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
		err := fmt.Errorf("failed to delete the subnet (%s)", subnetInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Verify deletion by checking subnet status after deletion request
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		log.Debug().Msgf("Waiting 3 seconds (attempt %d/%d) before checking subnet deletion status via GetSubnet", i+1, maxRetries)
		time.Sleep(3 * time.Second)

		// Use GetSubnet to check if subnet still exists
		log.Debug().Msgf("Checking if subnet (%s) still exists", subnetInfo.Id)
		_, checkErr := GetSubnet(nsId, vNetId, subnetId)

		// If we get an error (subnet not found), it means deletion was successful
		if checkErr != nil {
			log.Info().Msgf("Confirmed subnet (%s) deletion", subnetInfo.Id)
			break
		}

		// If this was the last attempt and subnet still exists, log warning but continue
		if i == maxRetries-1 {
			log.Warn().Msgf("Subnet (%s) may still exist in CSP after %d attempts, but proceeding with local cleanup", subnetInfo.Id, maxRetries)
		}
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
	// 	model.LabelManager:  model.StrManager,
	// 	"namespace": nsId,
	// }
	err = label.DeleteLabelObject(model.StrSubnet, subnetInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("subnet (%s) has been deleted", subnetId)

	// [Output] the message
	ret.Message = fmt.Sprintf("the subnet (%s) has been deleted", subnetId)

	return ret, nil
}

func RefineSubnet(nsId string, vNetId string, subnetId string) (model.SimpleMsg, error) {
	log.Info().Msg("RefineSubnet")

	/*
	 *	[NOTE]
	 *	"Refine" operates based on information managed by Tumblebug.
	 *	Based on this information, it checks whether there is information/resource in Spider/CSP.
	 *	It removes the information managed by Tumblebug if there's no information/resource.
	 */

	// subnet objects
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg
	var vNetInfo model.VNetInfo
	var subnetInfo model.SubnetInfo

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	/*
	 *	Validate the input parameters
	 */

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
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
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

	// Read the stored subnet info
	subnetKeyValue, exists, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
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

	/*
	 *	Check and refine the subnet info
	 */

	// [Via Spider] Get the subnet
	client := clientManager.NewHttpClient()
	method := "GET"

	// API to get a subnet
	url := fmt.Sprintf("%s/vpc/%s/subnet/%s", model.SpiderRestUrl, subnetInfo.CspVNetName, subnetInfo.CspResourceName)
	queryParams := "?ConnectionName=" + subnetInfo.ConnectionName
	url += queryParams

	spReqt := clientManager.NoBody

	log.Debug().Msgf("[Request to Spider] Refining Subnet (url: %s, request body: %+v)", url, spReqt)

	var spResp spiderSubnetInfo

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Refining Subnet (response body: %+v)", spResp)

	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }
	if err == nil {
		// [Output]
		err := fmt.Errorf("may not be refined, subnet info (id: %s) exists", subnetId)
		log.Warn().Err(err).Msg("")
		ret.Message = err.Error()
		return ret, err
	}

	/*
	 *	Delete the subnet info in case of the subnet does not exist
	 */

	// Delete the saved the subnet info
	err = kvstore.Delete(subnetKey)
	if err != nil {
		log.Warn().Err(err).Msg("")
		// return emptyRet, err
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
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Warn().Err(err).Msg("")
		// return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Warn().Err(err).Msg("")
		// return emptyRet, err
	}

	err = label.DeleteLabelObject(model.StrSubnet, subnetInfo.Uid)
	if err != nil {
		log.Warn().Err(err).Msg("")
	}

	// Get and check the subnet info still exists or not
	_, exists, err = kvstore.GetKv(subnetKey)
	if err != nil {
		log.Warn().Err(err).Msg("")
	}
	if exists {
		err := fmt.Errorf("fail to refine the subnet info (id: %s)", subnetId)
		ret.Message = err.Error()
		return ret, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the subnet info (%s) has been refined", subnetId)

	return ret, nil
}

func RegisterSubnet(nsId string, vNetId string, subnetReq *model.RegisterSubnetReq) (model.SubnetInfo, error) {
	log.Info().Msg("RegisterSubnet")

	// subnet objects
	var emptyRet model.SubnetInfo
	var vNetInfo model.VNetInfo
	var subnetInfo model.SubnetInfo
	var err error = nil

	/*
	 *	Validate the input parameters
	 */

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

	// Set the subnet object
	subnetInfo.Id = subnetReq.Name
	subnetInfo.Name = subnetReq.Name

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet
	subnetInfo.ResourceType = resourceType

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
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
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
	subnetInfo.CspVNetName = vNetInfo.CspResourceName

	/*
	 *	Register a subnet
	 */

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

	client := clientManager.NewHttpClient()
	method := "POST"
	var spResp spiderSubnetInfo

	// API to register a subnet from CSP
	url := fmt.Sprintf("%s/regsubnet", model.SpiderRestUrl)
	// [Note] Spider doesn't provide "GET /vpc{VPCName}/subnet" API

	log.Debug().Msgf("[Request to Spider] Registering Subnet (url: %s, request body: %+v)", url, spReqt)

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

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Registering Subnet (response body: %+v)", spResp)

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
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrSubnet,
		model.LabelId:              subnetInfo.Id,
		model.LabelName:            subnetInfo.Name,
		model.LabelUid:             subnetInfo.Uid,
		model.LabelCspResourceId:   subnetInfo.CspResourceId,
		model.LabelCspResourceName: subnetInfo.CspResourceName,
		model.LabelIpv4_CIDR:       subnetInfo.IPv4_CIDR,
		model.LabelZone:            subnetInfo.Zone,
		model.LabelStatus:          subnetInfo.Status,
		model.LabelVNetId:          vNetInfo.Id,
		model.LabelCspVNetId:       vNetInfo.CspResourceId,
		model.LabelCspVNetName:     vNetInfo.CspResourceName,
		model.LabelDescription:     subnetInfo.Description,
		model.LabelConnectionName:  subnetInfo.ConnectionName,
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
	log.Info().Msg("DeregisterSubnet")

	// subnet objects
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	/*
	 *	Validate the input parameters
	 */

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

	// Set the resource type
	parentResourceType := model.StrVNet
	resourceType := model.StrSubnet

	// Set a key for the subnet object
	vNetKey := common.GenResourceKey(nsId, parentResourceType, vNetId)
	subnetKey := common.GenChildResourceKey(nsId, resourceType, vNetId, subnetId)

	// Read the saved vNet info
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// vNet object
	var vNetInfo model.VNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored subnet info
	subnetKv, exists, err := kvstore.GetKv(subnetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, subnet: %s", subnetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// subnet object
	var subnetInfo model.SubnetInfo
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

	/*
	 *	Deregister the subnet
	 */

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
	spReqt.ReqInfo.VPCName = subnetInfo.CspVNetName

	// API to deregister subnet
	url := fmt.Sprintf("%s/regsubnet/%s", model.SpiderRestUrl, subnetInfo.CspResourceName)

	log.Debug().Msgf("[Request to Spider] Deregistering Subnet (url: %s, request body: %+v)", url, spReqt)

	var spResp spiderBooleanInfoResp

	client := clientManager.NewHttpClient()
	method := "DELETE"

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Deregistering Subnet (response body: %+v)", spResp)

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
	// 	model.LabelManager:  model.StrManager,
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
 * The following functions are used for Designing VNets
 */

// GetFirstNZones returns the first N zones of the given connection
func GetFirstNZones(connectionName string, firstN int) ([]string, int, error) {

	// TODO: Update the validation logic
	// It's a temporary validation logic due to the connection name pattern

	// Splite the connectionName into provider and region
	parts := strings.SplitN(connectionName, "-", 2)
	provider := parts[0]
	regionZone := parts[1]

	// Get the region details
	regionsObj, err := common.GetRegions(provider)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, 0, err
	}

	// Try to match and get the region detail
	var regionDetail model.RegionDetail
	for _, region := range regionsObj.Regions {
		exists := strings.HasPrefix(regionZone, region.RegionName)
		if exists {
			regionDetail = region
			break
		}
	}
	// Check if the region detail exists or not
	if regionDetail.RegionName == "" && len(regionDetail.Zones) == 0 {
		err := fmt.Errorf("invalid region/zone: %s", regionZone)
		log.Error().Err(err).Msg("")
		return nil, 0, err
	}

	// Get the first N zones
	zones := regionDetail.Zones
	length := len(zones)
	if length > firstN {
		zones = zones[:firstN]
		length = firstN
	}

	if len(zones) == 0 {
		return nil, 0, nil
	}

	return zones, length, nil
}

// FindSubnetByZone finds a subnet that matches the specified zone from a VNet.
// If zone is empty or no matching subnet is found, returns the first (default) subnet.
// This is useful for VM placement in a specific zone when the user explicitly specifies one.
//
// Parameters:
//   - nsId: Namespace ID
//   - vNetId: VNet ID to search subnets in
//   - zone: Target zone to find matching subnet (empty string means use default)
//
// Returns:
//   - subnetId: The ID of the matching or default subnet
//   - subnetZone: The actual zone of the selected subnet
//   - error: Error if VNet doesn't exist or has no subnets
func FindSubnetByZone(nsId string, vNetId string, zone string) (subnetId string, subnetZone string, err error) {
	log.Debug().Msgf("FindSubnetByZone: nsId=%s, vNetId=%s, zone=%s", nsId, vNetId, zone)

	// Get VNet info to access subnet list
	vNetInfo, err := GetVNet(nsId, vNetId)
	if err != nil {
		return "", "", fmt.Errorf("failed to get VNet '%s': %w", vNetId, err)
	}

	if len(vNetInfo.SubnetInfoList) == 0 {
		return "", "", fmt.Errorf("VNet '%s' has no subnets", vNetId)
	}

	// If zone is specified, try to find a matching subnet
	if zone != "" {
		for _, subnet := range vNetInfo.SubnetInfoList {
			if subnet.Zone == zone {
				log.Info().Msgf("Found subnet '%s' matching zone '%s'", subnet.Id, zone)
				return subnet.Id, subnet.Zone, nil
			}
		}
		log.Warn().Msgf("No subnet found matching zone '%s', using default subnet", zone)
	}

	// Return the first (default) subnet if no zone specified or no match found
	defaultSubnet := vNetInfo.SubnetInfoList[0]
	log.Debug().Msgf("Using default subnet '%s' (zone: '%s')", defaultSubnet.Id, defaultSubnet.Zone)
	return defaultSubnet.Id, defaultSubnet.Zone, nil
}
