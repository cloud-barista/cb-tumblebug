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
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/apierr"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	ktcsp "github.com/cloud-barista/cb-tumblebug/src/core/csp/kt"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

type NetworkAction string

const (
	ActionNone        NetworkAction = ""
	ActionForce       NetworkAction = "force"
	ActionWithSubnets NetworkAction = "withsubnets"
	// add additional actions here
)

var (
	stringToNetworkAction = map[string]NetworkAction{
		"":            ActionNone,
		"force":       ActionForce,
		"withsubnets": ActionWithSubnets,
	}
)

func ParseNetworkAction(s string) (NetworkAction, bool) {
	action, ok := stringToNetworkAction[strings.ToLower(s)]
	return action, ok
}

func (na NetworkAction) String() string {
	return string(na)
}

// VNetReqStructLevelValidation is a function to validate 'VNetReq' object.
func VNetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.VNetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field any, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

func ValidateVNetReq(vNetReq *model.VNetReq) error {
	log.Debug().Msg("ValidateVNetReq")
	log.Trace().Msgf("vNetReq: %+v", vNetReq)

	// * 1. Validates that each struct fields follows the rules in its 'validate' tags.
	err := validate.Struct(vNetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return err
		}
		return err
	}

	// * 2. Validates that the vNet has at least one subnet.
	if len(vNetReq.SubnetInfoList) == 0 {
		err := fmt.Errorf("at least one subnet is required")
		log.Error().Err(err).Msg("")
		return err
	}

	// * 3. Validates that each subnet's zone is valid in the region.
	// Resolve the region detail from the connection's config (its actual provider/region),
	// not by splitting the connection name.
	connConfig, err := common.GetConnConfig(vNetReq.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	provider := connConfig.ProviderName
	regionDetail := connConfig.RegionDetail
	if regionDetail.RegionName == "" && len(regionDetail.Zones) == 0 {
		err := fmt.Errorf("invalid region/zone for connection: %s", vNetReq.ConnectionName)
		log.Error().Err(err).Msg("")
		return err
	}

	// Check if each subnet's zone is included in the region's zone list
	zones := regionDetail.Zones
	for _, subnetInfo := range vNetReq.SubnetInfoList {
		if subnetInfo.Zone != "" {
			if !ContainsZone(zones, subnetInfo.Zone) {
				err := fmt.Errorf("invalid zone: %s", subnetInfo.Zone)
				log.Error().Err(err).Msg("")
				return err
			}
		}
	}

	// GCP has no VPC-level CIDR: its subnets carry their own CIDRs and CB-Spider rejects a VPC
	// CIDR ("GCP VPC does not support IPv4_CIDR"), so the vNet CidrBlock is legitimately empty.
	// The CIDR-based checks below (availability + parent/child network structure) assume a
	// parseable vNet CIDR, so skip them in that case and let CB-Spider validate the subnets.
	// Verified: CB-Spider creates a GCP VPC successfully with an empty vNet CIDR.
	noVNetCidr := vNetReq.CidrBlock == "" && csp.ResolveCloudPlatform(provider) == csp.GCP

	// * 4. Validates that the CIDR block of the vNet and subnets are available for use in the CSP.
	// e.g., in available CIDR Blocks, not in the reserved CIDR Blocks, and etc.
	if !noVNetCidr {
		ok, err := IsAvailableForUseInCSP(vNetReq, provider)
		if !ok {
			if err != nil {
				err2 := fmt.Errorf("CIDR block is not available for use in the CSP (provider: %s): %w", provider, err)
				log.Error().Err(err2).Msg("")
				return err2
			} else {
				err := fmt.Errorf("CIDR block is not available for use in the CSP (provider: %s)", provider)
				log.Error().Err(err).Msg("")
				return err
			}
		}
	}

	// * 5. Validates that the CIDR block of the vNet and subnets are valid
	if !noVNetCidr {
		// A network object for validation
		var network netutil.Network
		var subnets []netutil.Network

		network = netutil.Network{
			CidrBlock: vNetReq.CidrBlock,
		}

		for _, subnetInfo := range vNetReq.SubnetInfoList {
			subnet := netutil.Network{
				CidrBlock: subnetInfo.IPv4_CIDR,
			}
			subnets = append(subnets, subnet)
		}
		network.Subnets = subnets
		log.Trace().Msgf("network: %+v", network)

		// Validate the network object
		if err := netutil.ValidateNetwork(network); err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	}

	return nil
}

func ContainsZone(zones []string, zone string) bool {
	return slices.Contains(zones, zone)
}

func IsAvailableForUseInCSP(vNetReq *model.VNetReq, provider string) (bool, error) {

	// * 1. Check if the provider info exists
	csp, ok := common.RuntimeCloudNetworkInfo.CSPs[provider]
	if !ok {
		log.Warn().Msgf("skip validation, no CSP info for provider: %s", provider)
		return true, nil
	}

	// * 2. Check if the input CIDR block is valid.
	// Input the CIDR block of the vNet
	vNetCidrBlock := vNetReq.CidrBlock
	// Parse IPNet
	_, vNetIpNet, err := net.ParseCIDR(vNetCidrBlock)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR block format (%s): %v", vNetCidrBlock, err)
	}
	vNetPrefixLength, _ := vNetIpNet.Mask.Size()

	// * 3. Check if the CIDR block of the vNet is available for use in the CSP
	if csp.AvailableCIDRBlocks != nil {

		// Check if the CIDR block is in the available CIDR blocks
		isAvailable := false
		for _, availableCidrBlockDetail := range csp.AvailableCIDRBlocks {

			// Parse IPNet
			_, availableIpNet, err := net.ParseCIDR(availableCidrBlockDetail.CIDRBlock)
			if err != nil {
				return false, fmt.Errorf("invalid CIDR block format (%s): %v", availableCidrBlockDetail.CIDRBlock, err)
			}

			// Its available if the CIDR blocks are the same
			if vNetIpNet.String() == availableIpNet.String() {
				isAvailable = true
				break
			}

			// 1. Available CIDR block must include the input CIDR block
			// 2. Network mask of the available CIDR block must be less than the input CIDR block
			PrefixLengthOfAvailableCidrBlock, _ := availableIpNet.Mask.Size()

			if availableIpNet.Contains(vNetIpNet.IP) && PrefixLengthOfAvailableCidrBlock < vNetPrefixLength {
				isAvailable = true
				break
			}
		}

		if !isAvailable {
			err := fmt.Errorf("vNet CIDR block %s is not available for use in the CSP (provider: %s)", vNetCidrBlock, provider)
			log.Error().Err(err).Msg("")
			return false, err
		}

		log.Debug().Msgf("[Network Validation Success] vNet CIDR block %s is available for use in the CSP (provider: %s)", vNetCidrBlock, provider)
	}

	// * 4. Check if the prefix length of the vNet CIDR block is in range of CSP's vNet prefix length
	// Note: GCP does not have vNet (VPC) CIDR block requirement, so skip the prefix length check
	if csp.VNet != nil {
		vNetPrefixMin := csp.VNet.PrefixLength.Min
		vNetPrefixMax := csp.VNet.PrefixLength.Max

		if !(vNetPrefixLength >= vNetPrefixMin && vNetPrefixLength <= vNetPrefixMax) {
			err := fmt.Errorf("vNet CIDR block %s is not valid (provider: %s, prefix min: %d, prefix max: %d)", vNetCidrBlock, provider, vNetPrefixMin, vNetPrefixMax)
			return false, err
		}
		log.Debug().Msgf("[Network Validation Success] vNet CIDR block %s is valid (provider: %s, prefix min: %d, prefix max: %d)", vNetCidrBlock, provider, vNetPrefixMin, vNetPrefixMax)
	}

	// * 5. Check if the vNet CIDR block is in the reserved CIDR blocks
	// * For the time being, just make a warning log
	if csp.ReservedCIDRBlocks != nil {
		for _, reservedCidrBlockDetail := range csp.ReservedCIDRBlocks {
			// Parse IPNet
			_, reservedIpNet, err := net.ParseCIDR(reservedCidrBlockDetail.CIDRBlock)
			if err != nil {
				return false, fmt.Errorf("invalid CIDR block format (%s): %v", reservedIpNet, err)
			}

			// It's not available if the CIDR blocks are the same
			if vNetIpNet.String() == reservedIpNet.String() {
				log.Warn().Msgf("vNet CIDR block %s is in the reserved CIDR blocks (provider: %s)", vNetCidrBlock, provider)
				// return false, err
			}

			// Check if the vNet CIDR block is in the reserved CIDR blocks
			if reservedIpNet.Contains(vNetIpNet.IP) {
				log.Warn().Msgf("vNet CIDR block %s is in the reserved CIDR blocks (provider: %s)", vNetCidrBlock, provider)
				// return false, err
			}
		}

		log.Debug().Msgf("[Network Validation Success] vNet CIDR block %s is not in the reserved CIDR blocks (provider: %s)", vNetCidrBlock, provider)
	}

	// * 6. Check if the CIDR block of the subnet is
	// subnet of the vNet CIDR block and
	// available for use in the CSP.

	// Get the CIDR block of the subnets
	// subnetCidrBlocks := make([]string, len(vNetReq.SubnetInfoList))
	if csp.Subnet != nil {
		for _, subnetInfo := range vNetReq.SubnetInfoList {

			// * 6-1. Check if the subnet CIDR block is available for use in the CSP
			subnetCidrBlock := subnetInfo.IPv4_CIDR
			// Parse IPNet
			_, subnetIpNet, err := net.ParseCIDR(subnetCidrBlock)
			if err != nil {
				return false, fmt.Errorf("invalid subnet CIDR block format (%s): %v", subnetIpNet, err)
			}

			// 1. Available CIDR block must include the input CIDR block
			// 2. Network mask of the available CIDR block must be less than the input CIDR block
			subnetPrefixLength, _ := subnetIpNet.Mask.Size()

			if !(vNetIpNet.Contains(subnetIpNet.IP) && vNetPrefixLength < subnetPrefixLength) {
				err := fmt.Errorf("subnet CIDR block %s is not valid for vNet CIDR block: %s", subnetCidrBlock, vNetCidrBlock)
				log.Error().Err(err).Msg("")
				return false, err
			}

			// * 6-2. Check if the prefix length of the subnet CIDR block is in range of CSP's subnet prefix length
			subnetPrefixMin := csp.Subnet.PrefixLength.Min
			subnetPrefixMax := csp.Subnet.PrefixLength.Max
			if !(subnetPrefixLength >= subnetPrefixMin && subnetPrefixLength <= subnetPrefixMax) {
				err := fmt.Errorf("subnet CIDR block %s is not valid (provider: %s, prefix min: %d, prefix max: %d)", subnetCidrBlock, provider, subnetPrefixMin, subnetPrefixMax)
				log.Error().Err(err).Msg("")
				return false, err
			}
		}

		log.Debug().Msgf("[Network Validation Success] subnet CIDR block %s is valid (provider: %s, prefix min: %d, prefix max: %d)", vNetCidrBlock, provider, csp.Subnet.PrefixLength.Min, csp.Subnet.PrefixLength.Max)
	}

	log.Info().Msgf("[Network Validation Completed] Everything is valid (provider: %s)", provider)

	// TODO: Validate the VPN in the VPN request section.

	return true, nil
}

// The spiderXxx structs are used to call the Spider REST API
// Ref:
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/api-runtime/rest-runtime/VPC-SubnetRest.go
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

// Synchronized the request body with the Spider API

// ConnectionRequest represents the request body for common use.
type spiderConnectionRequest struct {
	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
}

// spiderVPCRegisterRequest represents the Spider API request body for registering a vNet (VPC in Spider).
type spiderVPCRegisterRequest struct {
	ConnectionName string                       `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        spiderVPCRegisterRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderVPCRegisterRequestInfo struct {
	Name  string `json:"Name" validate:"required" example:"vpc-01"`
	CSPId string `json:"CSPId" validate:"required" example:"csp-vpc-1234"`
}

// spiderCreateVPCRequest represents the Spider API request body for creating a vNet (VPC in Spider).
type spiderCreateVPCRequest struct {
	ConnectionName  string                     `json:"ConnectionName" validate:"required" example:"aws-connection"`
	IDTransformMode string                     `json:"IDTransformMode,omitempty" validate:"omitempty" example:"ON"` // ON: transform CSP ID, OFF: no-transform CSP ID
	ReqInfo         spiderCreateVPCRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderCreateVPCRequestInfo struct {
	Name           string                       `json:"Name" validate:"required" example:"vpc-01"`
	IPv4_CIDR      string                       `json:"IPv4_CIDR" validate:"omitempty"` // Some CSPs do not support vNet (VPC) CIDR
	SubnetInfoList []spiderAddSubnetRequestInfo `json:"SubnetInfoList" validate:"required"`
	TagList        []model.KeyValue             `json:"TagList,omitempty" validate:"omitempty"`
}

// type spiderListVPCReq struct {
// 	ConnectionName string `json:"ConnectionName" query:"ConnectionName" example:"aws-connection"`
// }

// type spiderListVPCResponse struct {
// 	Result []spiderVPCInfo `json:"vpc" validate:"required" description:"A list of vNet (VPC) information"`
// }

type spiderVpcDeleteReq struct {
	ConnectionName string // Connection name for the cloud provider
}

// type spiderCspVpcDeleteReq struct {
// 	ConnectionName string // Connection name for the cloud provider
// }

type spiderBooleanInfoResp struct {
	Result string // Result of the operation
}

// type spiderGetSGOwnerVPCRequest struct {
// 	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
// 	ReqInfo        struct {
// 		CSPId string `json:"CSPId" validate:"required" example:"csp-sg-1234"`
// 	} `json:"ReqInfo" validate:"required"`
// }

/*
	Based on polymorphism, the following Spider-related structs have been designed.
	The Spider API's requests/response bodies have been appropriately combined,
	and then `required` and `omitempty` have been appropriately assigned.
	Note - A separate struct can be created at any time
	if a conflict between `required` and `optional` is detected in a certain property.
*/

// [Note] Keep the combined structs for Spider API request bodies
// Given that API docs may not be clear about the required and optional properties currently.
// type spiderCreateVpcReq struct {
// 	spiderReqBase
// 	ReqInfo spiderVpcInfo `json:"ReqInfo" validate:"required"`
// }

// type spiderAddSubnetReq struct {
// 	spiderReqBase
// 	ReqInfo spiderSubnetInfo `json:"ReqInfo" validate:"required"`
// }

// type spiderReqBase struct {
// 	ConnectionName  string `json:"ConnectionName" validate:"required"` // Connection name for the cloud provider
// 	IDTransformMode string `json:"IDTransformMode,omitempty"`          // ID Transform mode, ON | OFF (default is ON)
// }

// [Note] Use the combined structs for Spider API response bodies
// The SpiderVpcInfo structure is a union of the properties in
// Spider's 'vpcRegisterReq', 'vpcCreateReq', and 'VPCInfo' structs.
type spiderVPCInfo struct {
	IId            model.IID          `json:"IId" validate:"required"` // {NameId, SystemId}
	IPv4_CIDR      string             `json:"IPv4_CIDR" validate:"required" example:"10.0.0.0/16" description:"The IPv4 CIDR block for the VPC"`
	SubnetInfoList []spiderSubnetInfo `json:"SubnetInfoList" validate:"required" description:"A list of subnet information associated with this VPC"`

	TagList      []model.KeyValue `json:"TagList,omitempty" validate:"omitempty" description:"A list of tags associated with this VPC"`
	KeyValueList []model.KeyValue `json:"KeyValueList,omitempty" validate:"omitempty" description:"Additional key-value pairs associated with this VPC"`
}

// CreateVNet accepts vNet creation request, creates and returns an TB vNet object
func CreateVNet(ctx context.Context, nsId string, vNetReq *model.VNetReq) (model.VNetInfo, error) {
	log.Info().Msg("CreateVNet")

	// vNet objects
	var emptyRet model.VNetInfo
	var vNetInfo model.VNetInfo
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
	err = validate.Struct(vNetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	for _, subnetInfo := range vNetReq.SubnetInfoList {
		err = common.CheckString(subnetInfo.Name)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Set the resource type
	resourceType := model.StrVNet
	childResourceType := model.StrSubnet

	// Set the vNet object in advance
	uid := common.GenUid()
	vNetInfo.ResourceType = resourceType
	vNetInfo.Name = vNetReq.Name
	vNetInfo.Id = vNetReq.Name
	vNetInfo.Uid = uid
	vNetInfo.ConnectionName = vNetReq.ConnectionName
	connConfig, err := common.GetConnConfig(vNetInfo.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig: %w", err)
		log.Error().Err(err).Msg("")
	}
	vNetInfo.ConnectionConfig = connConfig
	vNetInfo.Description = vNetReq.Description

	// Note: Set subnetInfoList in vNetInfo in advance
	//       since each subnet uid must be consistent
	for _, subnetInfo := range vNetReq.SubnetInfoList {
		vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList, model.SubnetInfo{
			ResourceType:     model.StrSubnet,
			Id:               subnetInfo.Name,
			Name:             subnetInfo.Name,
			Uid:              common.GenUid(),
			IPv4_CIDR:        subnetInfo.IPv4_CIDR,
			Zone:             subnetInfo.Zone,
			ConnectionConfig: connConfig,
		})
	}

	log.Trace().Msgf("vNetInfo(initial): %+v", vNetInfo)

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetInfo.Id)

	unlock := LockResourceCreation(nsId, resourceType, vNetInfo.Id)
	defer unlock()

	// Check if the vNet already exists or not
	exists, err := CheckResource(nsId, resourceType, vNetInfo.Id)
	if exists {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("already exists, vNet: %s", vNetInfo.Id)
		return emptyRet, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the vNet (%s) exists or not", vNetInfo.Id)
		return emptyRet, err
	}

	/*
	 *	Create vNet with at least one subnet
	 */

	// [Conditions] Mark VNet as not ready (creating) before calling Spider API
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreating, "VNet creation in progress")
	model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, model.ReasonCreating, "")
	model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionFalse, model.ReasonNoChildren, "")
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Create a vNet and subnets
	spReqt := spiderCreateVPCRequest{}
	spReqt.ConnectionName = vNetReq.ConnectionName
	spReqt.ReqInfo.Name = vNetInfo.Uid
	// ! The following CSPs actually doesn't support to set CIDR block. (GCP, IBM)
	// * However, Tumblebug "intentionally" manages the CIDR block for multi-cloud network management purpose.
	// NOTE: Since Spider has CIDR block processing logic, it is input regardless of CSP when calling the API.
	spReqt.ReqInfo.IPv4_CIDR = vNetReq.CidrBlock

	// Note: Use the subnets in the vNetInfo object (instead of the vNetReq object)
	//       since each subnet uid must be consistent
	for _, subnetInfo := range vNetInfo.SubnetInfoList {
		spReqt.ReqInfo.SubnetInfoList = append(spReqt.ReqInfo.SubnetInfoList, spiderAddSubnetRequestInfo{
			Name:      subnetInfo.Uid,
			IPv4_CIDR: subnetInfo.IPv4_CIDR,
			Zone:      subnetInfo.Zone,
		})
	}

	log.Trace().Msgf("spReqt: %+v", spReqt)

	client := clientManager.NewHttpClient()
	method := "POST"
	var spResp spiderVPCInfo

	// API to create a vNet
	url := fmt.Sprintf("%s/vpc", model.SpiderRestUrl)

	log.Debug().Msgf("[Request to Spider] Creating VPC: %s", url)

	// Cleanup object when something goes wrong
	defer func() {
		// Only if this operation fails, the vNet will be deleted
		if err != nil && vNetInfo.Status == model.NetworkStatusCreating {
			if vNetInfo.CspResourceId == "" { // Delete the saved the subnet info
				log.Warn().Msgf("failed to create vNet, cleaning up the vNet: %v", vNetInfo.Id)
				// Delete the subnets associated with the vNet
				for _, subnetInfo := range vNetInfo.SubnetInfoList {
					if subnetInfo.CspResourceId == "" {
						// Set a subnetKey for the subnet object
						subnetKey := common.GenChildResourceKey(nsId, childResourceType, vNetInfo.Id, subnetInfo.Id)
						deleteErr := kvstore.Delete(subnetKey)
						if deleteErr != nil {
							log.Warn().Err(deleteErr).Msgf("failed to delete the subnet: %v from kvstore", subnetInfo.Id)
						}
					}
				}
				// Delete the saved the vNet info
				deleteErr := kvstore.Delete(vNetKey)
				if deleteErr != nil {
					log.Warn().Err(deleteErr).Msgf("failed to delete the vNet: %v from kvstore", vNetInfo.Id)
				}
			}
			// todo: check if the following operation is obviously required or not
			// } else { // Delete the vNet from the CSP
			// 	// [Via Spider] Delete the vNet withSubnets == true
			// 	_, deleteErr := DeleteVNet(nsId, vNetInfo.Id, "true")
			// 	if deleteErr != nil {
			// 		log.Warn().Err(err).Msgf("failed to delete vNet: %v from CSP", vNetInfo.Id)
			// 	}
			// }
		}
	}()

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Trace().Msgf("[Response from Spider] Creating VPC: %+v", spResp)

	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to create vNet '%s'", vNetInfo.Id))
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspResourceId = spResp.IId.SystemId
	vNetInfo.CspResourceName = spResp.IId.NameId
	vNetInfo.KeyValueList = spResp.KeyValueList
	// ! The following CSPs actually doesn't support to set CIDR block. (GCP, IBM)
	// * However, Tumblebug "intentionally" manages the CIDR block entered by the user. It's for multi-cloud network management purpose.
	if vNetReq.CidrBlock != "" && (vNetInfo.ConnectionConfig.ProviderName == csp.GCP || vNetInfo.ConnectionConfig.ProviderName == csp.IBM) {
		vNetInfo.CidrBlock = vNetReq.CidrBlock
	} else {
		vNetInfo.CidrBlock = spResp.IPv4_CIDR
	}

	// Note: Check one by one and update the vNet object with the response from the Spider
	//       since the order may differ different between slices
	for _, spSubnetInfo := range spResp.SubnetInfoList {
		for i, tbSubnetInfo := range vNetInfo.SubnetInfoList {
			if tbSubnetInfo.Uid == spSubnetInfo.IId.NameId {
				vNetInfo.SubnetInfoList[i].ResourceType = model.StrSubnet
				vNetInfo.SubnetInfoList[i].ConnectionName = vNetInfo.ConnectionName
				vNetInfo.SubnetInfoList[i].CspVNetId = spResp.IId.SystemId
				vNetInfo.SubnetInfoList[i].CspVNetName = spResp.IId.NameId
				vNetInfo.SubnetInfoList[i].CspResourceId = spSubnetInfo.IId.SystemId
				vNetInfo.SubnetInfoList[i].CspResourceName = spSubnetInfo.IId.NameId
				vNetInfo.SubnetInfoList[i].KeyValueList = spSubnetInfo.KeyValueList
				vNetInfo.SubnetInfoList[i].Zone = spSubnetInfo.Zone
				vNetInfo.SubnetInfoList[i].IPv4_CIDR = spSubnetInfo.IPv4_CIDR

				// [Conditions] Subnet created successfully → mark as ready and synced
				model.SetCondition(&vNetInfo.SubnetInfoList[i].Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonAvailable, "")
				model.SetCondition(&vNetInfo.SubnetInfoList[i].Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "")
				vNetInfo.SubnetInfoList[i].Status = model.DeriveSubnetStatus(vNetInfo.SubnetInfoList[i].Conditions)
			}
		}
	}

	// [Conditions] VNet creation succeeded → mark as ready, synced, and update children status
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonAvailable, "")
	model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "")
	hasSubnets := len(vNetInfo.SubnetInfoList) > 0
	if hasSubnets {
		model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionTrue, model.ReasonAllReady, "")
	} else {
		model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionTrue, model.ReasonNoChildren, "")
	}
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	vNetInfo.SystemMessage = ""

	log.Debug().Msgf("VNet created in CSP: id=%s, cspId=%s, cidr=%s", vNetInfo.Id, vNetInfo.CspResourceId, vNetInfo.CidrBlock)
	log.Trace().Msgf("vNetInfo(filled): %+v", vNetInfo)

	// Store vNet object into the key-value store
	value, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store subnet objects into the key-value store
	for _, subnetInfo := range vNetInfo.SubnetInfoList {
		// Set a subnetKey for the subnet object
		subnetKey := common.GenChildResourceKey(nsId, childResourceType, vNetInfo.Id, subnetInfo.Id)
		value, err := json.Marshal(subnetInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		// Store the subnet object into the key-value store
		err = kvstore.Put(subnetKey, string(value))
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
			model.LabelCidr:            subnetInfo.IPv4_CIDR,
			model.LabelDescription:     subnetInfo.Description,
			model.LabelZone:            subnetInfo.Zone,
			model.LabelVNetId:          vNetInfo.Id,
			model.LabelConnectionName:  vNetInfo.ConnectionName,
		}
		err = label.CreateOrUpdateLabel(ctx, model.StrSubnet, subnetInfo.CspResourceName, subnetKey, labels)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Check if the vNet info is stored
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, vNet: %s", vNetInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrVNet,
		model.LabelId:              vNetInfo.Id,
		model.LabelName:            vNetInfo.Name,
		model.LabelUid:             vNetInfo.Uid,
		model.LabelCspResourceId:   vNetInfo.CspResourceId,
		model.LabelCspResourceName: vNetInfo.CspResourceName,
		model.LabelCidr:            vNetInfo.CidrBlock,
		model.LabelDescription:     vNetInfo.Description,
		model.LabelConnectionName:  vNetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(ctx, model.StrVNet, vNetInfo.Uid, vNetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Info().Msgf("VNet created: id=%s, cidr=%s, subnets=%d", vNetInfo.Id, vNetInfo.CidrBlock, len(vNetInfo.SubnetInfoList))
	return vNetInfo, nil
}

func GetVNet(nsId string, vNetId string) (model.VNetInfo, error) {
	log.Debug().Msg("GetVNet")

	// vNet object
	var emptyRet model.VNetInfo
	var vNetInfo model.VNetInfo

	/*
	 *	Validate the input parameters
	 */

	// Check the input parameters
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

	// Set the resource type
	resourceType := model.StrVNet
	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)

	// Read the stored vNet info
	keyValue, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if !exists {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Derive status from conditions stored in KV store
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	for i := range vNetInfo.SubnetInfoList {
		vNetInfo.SubnetInfoList[i].Status = model.DeriveSubnetStatus(vNetInfo.SubnetInfoList[i].Conditions)
	}

	log.Debug().Msgf("VNet retrieved: id=%s, status=%s", vNetInfo.Id, vNetInfo.Status)
	log.Trace().Msgf("vNetInfo: %+v", vNetInfo)

	return vNetInfo, nil
}

// markVNetDeleteFailed persists the vNet as Failed(DeletionFailed).
//
// `cause` is the original delete error; it populates both the Condition
// message and SystemMessage. The caller decides what error to return.
func markVNetDeleteFailed(nsId, vNetId, vNetKey string, vNetInfo *model.VNetInfo, cause error) {
	log.Error().Err(cause).Msg("")
	// [Conditions] Deletion failed → mark as Failed to prevent stuck state
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeletionFailed, cause.Error())
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	vNetInfo.SystemMessage = cause.Error()
	if failVal, marshalErr := json.Marshal(vNetInfo); marshalErr == nil {
		_ = kvstore.Put(vNetKey, string(failVal))
	}
}

// DeleteVNet accepts vNet creation request, creates and returns an TB vNet object
// VNetPresentOnCsp is the purge gate after a vNet DELETE. KT has no VPC of its own — Spider maps
// the account's default network — so the VPC always "exists"; there the gate asks whether any of
// the vNet's tiers (subnets) still exist instead.
func VNetPresentOnCsp(vNetInfo model.VNetInfo) (bool, error) {
	connConfig, err := common.GetConnConfig(vNetInfo.ConnectionName)
	if err == nil && strings.EqualFold(connConfig.ProviderName, csptypes.KT) {
		ctx := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, connConfig.CredentialHolder)
		tiers, terr := ktcsp.ListCustomTiers(ctx, connConfig.RegionZoneInfo.AssignedRegion, connConfig.RegionZoneInfo.AssignedZone)
		if terr != nil {
			return false, terr
		}
		for _, sn := range vNetInfo.SubnetInfoList {
			if _, ok := tiers[sn.CspResourceId]; ok {
				return true, nil
			}
			for id, name := range tiers {
				if name == sn.CspResourceName || id == sn.CspResourceName {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return ResourcePresentOnCsp(vNetInfo.ConnectionName, model.StrVNet, vNetInfo.CspResourceId, vNetInfo.CspResourceName)
}

func DeleteVNet(nsId string, vNetId string, actionParam string) (model.SimpleMsg, error) {
	log.Info().Msg("DeleteVNet")

	// vNet object
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	/*
	 *	Validate the input parameters
	 */

	// Check the input parameters
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

	action, valid := ParseNetworkAction(actionParam)
	if !valid {
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Msg(errMsg.Error())
		return emptyRet, errMsg
	}

	// Set the resource type
	resourceType := model.StrVNet

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)
	// Read the stored subnets
	subnetsKv, err := kvstore.GetKvList(vNetKey + "/subnet")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("Deleting VNet: %s (action: %s, subnets: %d)", vNetId, action, len(subnetsKv))
	log.Trace().Msgf("subnetsKv: %+v", subnetsKv)

	// normal case: action == ""
	if action == ActionNone && len(subnetsKv) > 0 {
		err := fmt.Errorf("cannot delete vNet (%s): has %d subnet(s); use action=withSubnets or action=force", vNetId, len(subnetsKv))
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the subnet delete action
	subnetDelAction := ActionNone
	switch action {
	case ActionNone, ActionWithSubnets:
		subnetDelAction = ActionNone
	case ActionForce:
		subnetDelAction = ActionForce
	default:
		err := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Msg(err.Error())
		return emptyRet, err
	}

	/*
	 *	Delete the vNet
	 */

	// First, delete the subnets associated with the vNet
	for _, kv := range subnetsKv {
		subnet := model.SubnetInfo{}
		err = json.Unmarshal([]byte(kv.Value), &subnet)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		_, err := DeleteSubnet(nsId, vNetId, subnet.Id, subnetDelAction.String())
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Read the stored vNet info, which includes the updated subnets
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

	// [Conditions] Mark VNet as not ready (deleting) before calling Spider API
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeleting, "VNet deletion in progress")
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	vNetInfo.SystemMessage = ""
	// Store the status
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Delete the vNet
	spReqt := spiderVpcDeleteReq{}
	spReqt.ConnectionName = vNetInfo.ConnectionName

	// API to delete a vNet
	url := fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, vNetInfo.CspResourceName)
	queryParam := ""
	if action == ActionForce {
		queryParam = "?force=true"
	}
	url += queryParam

	trials := 2
	seconds := uint64(3)
	ok := false

	// Retry loop: handles transient network/transport errors (e.g. 503, timeout)
	// and, less commonly, transient Result:false responses (e.g. rate-limiting).
	for i := range trials {
		// On the second attempt, wait before retrying to avoid hammering the CSP.
		if i > 0 {
			log.Warn().Msgf("Retrying to delete vNet (%s) after %d seconds...", vNetId, seconds)
			time.Sleep(time.Duration(seconds) * time.Second)
		}

		log.Debug().Msgf("[Request to Spider] Deleting VPC: %s", url)

		var spResp spiderBooleanInfoResp // response body: {"Result":"true"|"false"}

		client := clientManager.NewHttpClient()
		method := "DELETE"

		// restyResp is captured so HandleHttpResponse can wrap the HTTP status code for apierr.IsNotFound.
		restyResp, callErr := clientManager.ExecuteHttpRequest( // send DELETE request to Spider
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(spReqt),
			&spReqt,
			&spResp,
			clientManager.MediumDuration,
		)
		err = clientManager.HandleHttpResponse(restyResp, callErr) // normalize HTTP error into err

		log.Trace().Msgf("[Response from Spider] Deleting VPC: %+v", spResp)

		if err != nil {
			if apierr.IsNotFound(err) {
				// 404: vNet is already gone on the CSP side — treat as successful deletion.
				log.Info().Msgf("VPC (%s) not found on CSP, treating as already deleted", vNetId)
				ok = true // mark as deleted
				err = nil // clear error; not a failure
				break     // no further retries needed
			}
			// Other errors (5xx, transport failure): log and retry.
			log.Warn().Err(err).Msg("")
			continue
		}
		// Parse Spider's boolean result field ("true" or "false").
		ok, err = strconv.ParseBool(spResp.Result)
		if err != nil {
			// Unexpected response format — retry.
			log.Error().Err(err).Msg("")
			continue
		}
		if ok {
			break // Result:true — Spider confirmed deletion; exit loop immediately.
		}
		// Result:false — deletion rejected (e.g. subnet dependency); retry once.
	}

	// Check final result after all trials
	if err != nil {
		// A network/transport error persisted through all retries.
		markVNetDeleteFailed(nsId, vNetId, vNetKey, &vNetInfo, err)
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to delete vNet '%s'", vNetId))
	}
	if !ok {
		// Spider returned Result:false on every attempt — treat as a hard deletion failure.
		delErr := fmt.Errorf("failed to delete the vNet (%s)", vNetId)
		markVNetDeleteFailed(nsId, vNetId, vNetKey, &vNetInfo, delErr)
		return emptyRet, delErr
	}

	// The GET poll is an eventual-consistency wait only; the CSP enumeration is the purge
	// gate — a still-present vNet keeps the record for a later retry rather than orphaning it (issue #2685).
	verifyURL := fmt.Sprintf("%s/vpc/%s?ConnectionName=%s",
		model.SpiderRestUrl, vNetInfo.CspResourceName, vNetInfo.ConnectionName)
	PollResourceDeletedViaSpider(verifyURL, nil, DefaultPollMaxAttempts, DefaultPollInterval)
	if action != ActionForce {
		present, gateErr := VNetPresentOnCsp(vNetInfo)
		if gateErr != nil || present {
			cause := fmt.Errorf("vNet (%s) still exists on the CSP after DELETE; record retained — retry, or delete with action=force", vNetId)
			if gateErr != nil {
				cause = fmt.Errorf("vNet (%s) deletion unconfirmed: CSP existence check failed: %w", vNetId, gateErr)
			}
			markVNetDeleteFailed(nsId, vNetId, vNetKey, &vNetInfo, cause)
			return emptyRet, cause
		}
	}

	// Delete the saved the vNet info
	err = kvstore.Delete(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Remove label info using DeleteLabelObject
	// labels := map[string]string{
	// 	model.LabelManager:  model.StrManager,
	// 	"namespace": nsId,
	// }
	err = label.DeleteLabelObject(model.StrVNet, vNetInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the vNet (%s) has been deleted", vNetId)

	log.Info().Msgf("vNet (%s) has been deleted", vNetId)
	return ret, nil
}

// RegisterVNet accepts vNet registration request, register and returns an TB vNet object
func RegisterVNet(ctx context.Context, nsId string, vNetRegisterReq *model.RegisterVNetReq) (model.VNetInfo, error) {
	log.Info().Msg("RegisterVNet")

	// vNet objects
	var emptyRet model.VNetInfo
	var vNetInfo model.VNetInfo
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
	err = validate.Struct(vNetRegisterReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrVNet
	childResourceType := model.StrSubnet

	// Set the vNet object
	uid := common.GenUid()
	vNetInfo.ResourceType = resourceType
	vNetInfo.Id = vNetRegisterReq.Name
	vNetInfo.Name = vNetRegisterReq.Name
	vNetInfo.Uid = uid
	vNetInfo.ConnectionName = vNetRegisterReq.ConnectionName
	connectionConfig, err := common.GetConnConfig(vNetInfo.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	vNetInfo.ConnectionConfig = connectionConfig
	vNetInfo.Description = vNetRegisterReq.Description

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetRegisterReq.Name)
	// Check if the vNet already exists or not
	exists, err := CheckResource(nsId, resourceType, vNetRegisterReq.Name)
	if exists {
		err := fmt.Errorf("already exists, vNet: %s", vNetRegisterReq.Name)
		return emptyRet, err
	}
	if err != nil {
		err := fmt.Errorf("failed to check if the vNet (%s) exists or not", vNetRegisterReq.Name)
		return emptyRet, err
	}

	/*
	 *	Register vNet in the CSP, which has not been created by Tumblebug
	 */

	// [Conditions] Mark VNet as not ready (registering) before calling Spider API
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonRegistering, "VNet registration in progress")
	model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionFalse, model.ReasonRegistering, "")
	model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionFalse, model.ReasonNoChildren, "")
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	// Save the current operation status and the vNet object
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		return emptyRet, err
	}

	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		return emptyRet, err
	}

	// [Via Spider] Register vNet and subnets
	var spReqt = spiderVPCRegisterRequest{}
	spReqt.ConnectionName = vNetRegisterReq.ConnectionName
	spReqt.ReqInfo.Name = vNetInfo.Uid
	spReqt.ReqInfo.CSPId = vNetRegisterReq.CspResourceId

	client := clientManager.NewHttpClient()
	method := "POST"
	var spResp spiderVPCInfo

	// API to register a vNet from CSP
	url := fmt.Sprintf("%s/regvpc", model.SpiderRestUrl)

	// API to register a vNet from CB-Spider
	if spReqt.ReqInfo.CSPId == "" {
		url = fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, vNetInfo.Uid)
		queryParams := "?ConnectionName=" + vNetInfo.ConnectionName
		url += queryParams
		method = "GET"
		spReqt = spiderVPCRegisterRequest{}
	}

	log.Debug().Msgf("[Request to Spider] Registering VPC: %s", url)

	// Clean up the vNet object when something goes wrong
	defer func() {
		// Only if this operation fails, the vNet will be deleted
		if err != nil && vNetInfo.Status == model.NetworkStatusRegistering {
			if vNetInfo.CspResourceId == "" { // Delete the saved the vNet info
				log.Warn().Msgf("failed to create vNet, cleaning up the vNet info: %v, with associated subnets info", vNetInfo.Id)
				// Delete the subnets associated with the vNet
				for _, subnetInfo := range vNetInfo.SubnetInfoList {
					if subnetInfo.CspResourceId == "" {
						// Set a subnetKey for the subnet object
						subnetKey := common.GenChildResourceKey(nsId, childResourceType, vNetInfo.Id, subnetInfo.Id)
						deleteErr := kvstore.Delete(subnetKey)
						if deleteErr != nil {
							log.Warn().Err(deleteErr).Msgf("failed to delete the subnet info: %v from kvstore", subnetInfo.Id)
						}
					}
				}
				// Delete the saved the vNet info
				deleteErr := kvstore.Delete(vNetKey)
				if deleteErr != nil {
					log.Warn().Err(deleteErr).Msgf("failed to delete the vNet info: %v from kvstore", vNetInfo.Id)
				}
			}
			// todo: check if the following operation is obviously required or not
			// } else { // Delete the vNet from the CSP
			// 	// [Via Spider] Delete the vNet withSubnets == true
			// 	_, deleteErr := DeregisterVNet(nsId, vNetInfo.Id, "true")
			// 	if deleteErr != nil {
			// 		log.Warn().Err(err).Msgf("failed to delete vNet: %v from CSP", vNetInfo.Id)
			// 	}
			// }
		}
	}()

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Trace().Msgf("[Response from Spider] Registering VPC: %+v", spResp)

	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to register vNet '%s'", vNetInfo.Id))
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspResourceId = spResp.IId.SystemId
	vNetInfo.CspResourceName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList

	if vNetRegisterReq.CspResourceId != "" {
		vNetInfo.SystemLabel = "Registered from CSP resource"
	} else if vNetRegisterReq.CspResourceId == "" {
		vNetInfo.SystemLabel = "Registered from CB-Spider resource"
	}

	// Note: Check one by one and update the vNet object with the response from the Spider
	//       since the order may differ different between slices
	for _, spSubnetInfo := range spResp.SubnetInfoList {
		// [Conditions] Initialize subnet conditions as ready and synced for registered subnet
		subnetConditions := []model.Condition{}
		model.SetCondition(&subnetConditions, model.ConditionReady, model.ConditionTrue, model.ReasonAvailable, "")
		model.SetCondition(&subnetConditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "")
		subnetInfo := model.SubnetInfo{
			ResourceType:    model.StrSubnet,
			Id:              fmt.Sprintf("%s", spSubnetInfo.IId.NameId),
			Name:            fmt.Sprintf("%s", spSubnetInfo.IId.NameId),
			Uid:             common.GenUid(),
			ConnectionName:  vNetInfo.ConnectionName,
			Status:          model.NetworkStatusAvailable,
			Conditions:      subnetConditions,
			CspResourceId:   spSubnetInfo.IId.SystemId,
			CspResourceName: spSubnetInfo.IId.NameId,
			CspVNetId:       spResp.IId.SystemId,
			CspVNetName:     spResp.IId.NameId,
			KeyValueList:    spSubnetInfo.KeyValueList,
			Zone:            spSubnetInfo.Zone,
			IPv4_CIDR:       spSubnetInfo.IPv4_CIDR,
		}
		vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList, subnetInfo)

		// Set a subnetKey for the subnet object
		subnetKey := common.GenChildResourceKey(nsId, childResourceType, vNetInfo.Id, subnetInfo.Id)
		// Save the subnet object
		value, err := json.Marshal(subnetInfo)
		if err != nil {
			return emptyRet, err
		}
		err = kvstore.Put(subnetKey, string(value))
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
			model.LabelCidr:            subnetInfo.IPv4_CIDR,
			model.LabelDescription:     subnetInfo.Description,
			model.LabelZone:            subnetInfo.Zone,
			model.LabelVNetId:          vNetInfo.Id,
			model.LabelConnectionName:  vNetInfo.ConnectionName,
		}
		err = label.CreateOrUpdateLabel(ctx, model.StrSubnet, subnetInfo.CspResourceName, subnetKey, labels)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

	}

	log.Debug().Msgf("VNet registered: id=%s, cspId=%s, cidr=%s", vNetInfo.Id, vNetInfo.CspResourceId, vNetInfo.CidrBlock)
	log.Trace().Msgf("vNetInfo: %+v", vNetInfo)

	// [Conditions] VNet registration succeeded → mark as ready, synced, and update children status
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonAvailable, "")
	model.SetCondition(&vNetInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "")
	hasSubnets := len(vNetInfo.SubnetInfoList) > 0
	if hasSubnets {
		model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionTrue, model.ReasonAllReady, "")
	} else {
		model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionTrue, model.ReasonNoChildren, "")
	}
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	vNetInfo.SystemMessage = ""

	// Put vNet object into the key-value store
	value, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the vNet info is stored
	keyValue, exists, err := kvstore.GetKv(vNetKey)

	if !exists {
		err := fmt.Errorf("does not exist, vNet: %s", vNetRegisterReq.Name)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrVNet,
		model.LabelId:              vNetInfo.Id,
		model.LabelName:            vNetInfo.Name,
		model.LabelUid:             vNetInfo.Uid,
		model.LabelCspResourceId:   vNetInfo.CspResourceId,
		model.LabelCspResourceName: vNetInfo.CspResourceName,
		model.LabelCidr:            vNetInfo.CidrBlock,
		model.LabelDescription:     vNetInfo.Description,
		model.LabelConnectionName:  vNetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(ctx, model.StrVNet, vNetInfo.Uid, vNetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Info().Msgf("vNet (%s) has been registered", vNetInfo.Id)
	return vNetInfo, nil
}

// DeregisterVNet accepts vNet unregistration request, deregister and returns the result
func DeregisterVNet(nsId string, vNetId string, withSubnets string) (model.SimpleMsg, error) {
	log.Info().Msg("DeregisterVNet")

	// vNet object
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	/*
	 *	Validate the input parameters
	 */

	// Check the input parameters
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

	// Set the resource type
	resourceType := model.StrVNet

	// Validate options: withSubnets
	if withSubnets != "" && withSubnets != "true" && withSubnets != "false" {
		errMsg := fmt.Errorf("invalid option, withSubnets (%s)", withSubnets)
		log.Warn().Msg(errMsg.Error())
		return emptyRet, errMsg
	}
	if withSubnets == "" {
		withSubnets = "false"
	}

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)
	// Read the stored subnets
	subnetsKv, err := kvstore.GetKvList(vNetKey + "/subnet")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("Deregistering VNet: %s (withSubnets: %s, subnets: %d)", vNetId, withSubnets, len(subnetsKv))
	log.Trace().Msgf("subnetsKv: %+v", subnetsKv)

	if withSubnets == "false" && len(subnetsKv) > 0 {
		err := fmt.Errorf("cannot deregister vNet (%s): has %d subnet(s); set withSubnets=true", vNetId, len(subnetsKv))
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	/*
	 *	Deregister the vNet
	 */

	// Delete the subnets associated with the vNet
	for _, kv := range subnetsKv {
		subnet := model.SubnetInfo{}
		err = json.Unmarshal([]byte(kv.Value), &subnet)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		_, err := DeregisterSubnet(nsId, vNetId, subnet.Id)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

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

	// [Conditions] Mark VNet as not ready (deregistering) before calling Spider API
	model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeregistering, "VNet deregistration in progress")
	vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
	vNetInfo.SystemMessage = ""
	// Save the status
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Deregister the vNet
	spReqt := spiderConnectionRequest{}
	spReqt.ConnectionName = vNetInfo.ConnectionName

	// API to delete a vNet
	url := fmt.Sprintf("%s/regvpc/%s", model.SpiderRestUrl, vNetInfo.CspResourceName)

	log.Debug().Msgf("[Request to Spider] Deregistering VPC: %s", url)

	var spResp spiderBooleanInfoResp

	client := clientManager.NewHttpClient()
	method := "DELETE"

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)
	// restyResp is captured so HandleHttpResponse can wrap the error with the
	// HTTP status code; this lets apierr.IsNotFound use the status code
	// as a secondary signal when the error message alone is ambiguous.
	err = clientManager.HandleHttpResponse(restyResp, err)

	log.Trace().Msgf("[Response from Spider] Deregistering VPC: %+v", spResp)

	if err != nil {
		if apierr.IsNotFound(err) {
			// Resource already gone from Spider (not found)
			// Proceed with local cleanup (same as success path)
			log.Info().Msgf("VNet (%s) not found on Spider, proceeding with local cleanup", vNetInfo.Id)
		} else {
			log.Error().Err(err).Msg("")
			// [Conditions] Deregistration failed → mark as Failed to prevent stuck state
			model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeregisterFailed, err.Error())
			vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
			vNetInfo.SystemMessage = err.Error()
			failVal, marshalErr := json.Marshal(vNetInfo)
			if marshalErr == nil {
				_ = kvstore.Put(vNetKey, string(failVal))
			}
			return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to deregister vNet '%s'", vNetId))
		}
	} else {
		ok, err := strconv.ParseBool(spResp.Result)
		if err != nil {
			log.Error().Err(err).Msg("")
			// [Conditions] Deregistration failed → mark as Failed to prevent stuck state
			model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeregisterFailed, err.Error())
			vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
			vNetInfo.SystemMessage = err.Error()
			failVal, marshalErr := json.Marshal(vNetInfo)
			if marshalErr == nil {
				_ = kvstore.Put(vNetKey, string(failVal))
			}
			return emptyRet, err
		}
		if !ok {
			err := fmt.Errorf("failed to deregister the vNet (%s)", vNetId)
			log.Error().Err(err).Msg("")
			// [Conditions] Deregistration failed → mark as Failed to prevent stuck state
			model.SetCondition(&vNetInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeregisterFailed, err.Error())
			vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
			vNetInfo.SystemMessage = err.Error()
			failVal, marshalErr := json.Marshal(vNetInfo)
			if marshalErr == nil {
				_ = kvstore.Put(vNetKey, string(failVal))
			}
			return emptyRet, err
		}
	}

	// Delete the saved the vNet info
	err = kvstore.Delete(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Remove label info using DeleteLabelObject
	// labels := map[string]string{
	// 	model.LabelManager:  model.StrManager,
	// 	"namespace": nsId,
	// }
	err = label.DeleteLabelObject(model.StrVNet, vNetInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the vnet (%s) has been deregistered", vNetId)

	log.Info().Msgf("vNet (%s) has been deregistered", vNetId)
	return ret, nil
}

/*
 * The following functions are used for Designing VNets
 */

// DesignVNets accepts a VNet design request, designs and returns a VNet design response
func DesignVNets(reqt *model.VNetDesignRequest) (model.VNetDesignResponse, error) {
	log.Info().Msg("DesignVNets")

	var vNetDesignResp model.VNetDesignResponse
	var vNetReqList []model.VNetReq
	var allCIDRs []string

	baseIP, _, err := net.ParseCIDR(reqt.DesiredPrivateNetwork)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.VNetDesignResponse{}, err
	}

	nextAvailableIP := baseIP

	idx := 0
	for _, mcNetConf := range reqt.McNetConfigurations {
		for _, region := range mcNetConf.Regions {
			for k, vnet := range region.VNets {

				csp := mcNetConf.Csp
				region := region.Name
				connectionName := csp + "-" + region
				connectionName = strings.ToLower(connectionName)
				log.Debug().Msgf("CSP: %s, Region: %s", csp, region)
				log.Debug().Msgf("connectionName: %s", connectionName)

				// Use integer fields directly (already int type in model)
				subnetCount := vnet.SubnetCount
				hostsPerSubent := vnet.HostsPerSubnet
				useFirstNZones := vnet.UseFirstNZones

				// Design a vNet
				log.Debug().Msgf("CSP: %s, Region %s, VNet %02d:\n", mcNetConf.Csp, region, k+1)

				// Calculate CIDR blocks for vNet and subnets
				cidr, subnets, newNextAvailableIP, err := netutil.DeriveVNetAndSubnets(nextAvailableIP, hostsPerSubent, subnetCount)
				if err != nil {
					log.Warn().Msgf("Error calculating subnets: %v", err)
					continue
				}
				log.Debug().Msgf("vNet: %s", cidr)
				vNetReq := model.VNetReq{
					Name:           fmt.Sprintf("vnet%02d", idx),
					ConnectionName: connectionName,
					CidrBlock:      cidr,
					Description:    fmt.Sprintf("vnet%02d designed by util/vNet/design", idx),
				}

				log.Debug().Msgf("Subnets:")
				zones, length, err := GetFirstNZones(connectionName, useFirstNZones)
				if err != nil {
					log.Error().Err(err).Msg("")
				}

				for l, subnet := range subnets {
					subnetReq := model.SubnetReq{}
					subnetReq.IPv4_CIDR = subnet

					// Note - Depending on the input, a few more subnets can be created
					if l < subnetCount {
						subnetReq.Name = fmt.Sprintf("subnet%02d", l)
						subnetReq.Description = fmt.Sprintf("subnet%02d designed by util/vNet/design", l)
					} else {
						subnetReq.Name = fmt.Sprintf("subnet%02d-reserved", l)
						subnetReq.Description = fmt.Sprintf("subnet%02d-reserved designed by util/vNet/design", l)
					}

					// Zone selection method: firstNZones
					if length > 0 {
						subnetReq.Zone = zones[l%length]
					} else {
						subnetReq.Zone = ""
					}

					// Add the subnet to the vNet
					vNetReq.SubnetInfoList = append(vNetReq.SubnetInfoList, subnetReq)
				}
				nextAvailableIP = newNextAvailableIP

				// Keep all CIDRs for supernetting
				allCIDRs = append(allCIDRs, cidr)

				// Add the vNet to the list
				vNetReqList = append(vNetReqList, vNetReq)
				idx++
			}
		}
	}
	vNetDesignResp.VNetReqList = vNetReqList

	if reqt.SupernettingEnabled == "true" {
		supernet, err := netutil.CalculateSupernet(allCIDRs)
		if err != nil {
			log.Error().Err(err).Msg("")
			return model.VNetDesignResponse{}, err
		}
		log.Info().Msgf("Supernet of all vNets: %s", supernet)
		vNetDesignResp.RootNetworkCIDR = supernet
	}

	log.Info().Msgf("Designed %d vNets with supernetting enabled: %s", len(vNetReqList), vNetDesignResp.RootNetworkCIDR)
	return vNetDesignResp, nil
}

// PruneVNets purges Tumblebug metadata for all VNets in a namespace
// that were diagnosed as missing on CSP (SyncStateCspResourceMissing).
func PruneVNets(nsId string) (model.ResourcePruneResults, error) {
	err := common.CheckString(nsId)
	if err != nil {
		return model.ResourcePruneResults{}, err
	}

	resList, err := ListResource(nsId, model.StrVNet, "", "")
	if err != nil {
		return model.ResourcePruneResults{}, err
	}

	vnetList, ok := resList.([]model.VNetInfo)
	if !ok {
		return model.ResourcePruneResults{}, fmt.Errorf("unexpected type from ListResource")
	}

	pruneResults := model.ResourcePruneResults{
		Results: []model.ResourcePruneResult{},
	}

	for _, vNetInfo := range vnetList {
		vNetKey := common.GenResourceKey(nsId, model.StrVNet, vNetInfo.Id)
		subnetKvList, _ := kvstore.GetKvList(vNetKey + "/subnet")

		// The list above is a snapshot; re-read and use the current state from here on, since a
		// concurrent Reconcile may have already restored this VNet in the meantime.
		freshKv, exists, fErr := kvstore.GetKv(vNetKey)
		if fErr != nil || !exists {
			continue
		}
		if json.Unmarshal([]byte(freshKv.Value), &vNetInfo) != nil {
			continue
		}
		condSynced := model.GetCondition(vNetInfo.Conditions, model.ConditionSynced)
		// Only ReasonCspResourceMissing is Prune's concern — a broader Status==Failed check would also
		// match SpMetaMissing (CSP still alive) and delete metadata for a resource that isn't actually gone.
		vNetMissing := condSynced != nil && condSynced.Reason == model.ReasonCspResourceMissing

		if !vNetMissing {
			// Parent VNet is healthy, but a subnet can still be individually CspResourceMissing — prune only those.
			prunedAny := false
			for _, sKv := range subnetKvList {
				var sInfo model.SubnetInfo
				if json.Unmarshal([]byte(sKv.Value), &sInfo) != nil {
					continue
				}
				subnetKey := common.GenChildResourceKey(nsId, model.StrSubnet, vNetInfo.Id, sInfo.Id)

				// Re-verify this subnet's current state too, right before acting on it.
				freshSubKv, subExists, sfErr := kvstore.GetKv(subnetKey)
				if sfErr != nil || !subExists {
					continue
				}
				if json.Unmarshal([]byte(freshSubKv.Value), &sInfo) != nil {
					continue
				}
				sCondSynced := model.GetCondition(sInfo.Conditions, model.ConditionSynced)
				if sCondSynced == nil || sCondSynced.Reason != model.ReasonCspResourceMissing {
					continue
				}

				purgeSpiderSubnetMetadata(sInfo)
				if lErr := label.DeleteLabelObject(model.StrSubnet, sInfo.Uid); lErr != nil {
					log.Warn().Err(lErr).Msgf("failed to delete label during prune for subnet %s", sInfo.Id)
				}
				delErr := kvstore.Delete(subnetKey)
				res := model.ResourcePruneResult{
					ResourceType:   model.StrSubnet,
					ResourceId:     sInfo.Id,
					ConnectionName: sInfo.ConnectionName,
				}
				if delErr != nil {
					res.Success = false
					res.Error = delErr.Error()
					pruneResults.FailedCount++
				} else {
					res.Success = true
					res.Message = fmt.Sprintf("Orphaned metadata for subnet (%s) pruned successfully", sInfo.Id)
					pruneResults.SuccessCount++
					// Drop the pruned subnet from the parent's own embedded list too, or it lingers there as a stale entry.
					for i, s := range vNetInfo.SubnetInfoList {
						if s.Id == sInfo.Id {
							vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList[:i], vNetInfo.SubnetInfoList[i+1:]...)
							break
						}
					}
					prunedAny = true
				}
				pruneResults.TotalPruned++
				pruneResults.Results = append(pruneResults.Results, res)
			}
			if prunedAny {
				if len(vNetInfo.SubnetInfoList) > 0 {
					model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionTrue, model.ReasonAllReady, "")
				} else {
					model.SetCondition(&vNetInfo.Conditions, model.ConditionChildrenReady, model.ConditionTrue, model.ReasonNoChildren, "")
				}
				vNetInfo.Status = model.DeriveVNetStatus(vNetInfo.Conditions)
				if val, mErr := json.Marshal(vNetInfo); mErr == nil {
					if pErr := PutResourceObject(vNetKey, val); pErr != nil {
						log.Warn().Err(pErr).Msgf("failed to persist vNet %s after pruning its orphaned subnets", vNetInfo.Id)
					}
				}
			}
			continue
		}

		// Parent VNet itself is gone — purge children (subnets) before the parent, since the subnet-delete URL is VPC-scoped.
		for _, sKv := range subnetKvList {
			var sInfo model.SubnetInfo
			if json.Unmarshal([]byte(sKv.Value), &sInfo) == nil {
				purgeSpiderSubnetMetadata(sInfo)
				subnetKey := common.GenChildResourceKey(nsId, model.StrSubnet, vNetInfo.Id, sInfo.Id)
				_ = kvstore.Delete(subnetKey)
				_ = label.DeleteLabelObject(model.StrSubnet, sInfo.Uid)
			}
		}

		// Now the parent: purge Spider's own orphaned VPC IID too, or it's stranded forever once TB's record is gone.
		if vNetInfo.CspResourceName != "" {
			spForceDelReqt := spiderVpcDeleteReq{ConnectionName: vNetInfo.ConnectionName}
			forceDelURL := fmt.Sprintf("%s/vpc/%s?force=true", model.SpiderRestUrl, vNetInfo.CspResourceName)
			var spForceDelResp spiderBooleanInfoResp
			restyForceDelResp, forceDelErr := clientManager.ExecuteHttpRequest(
				clientManager.NewHttpClient(),
				"DELETE",
				forceDelURL,
				nil,
				clientManager.SetUseBody(spForceDelReqt),
				&spForceDelReqt,
				&spForceDelResp,
				clientManager.MediumDuration,
			)
			if forceDelErr = clientManager.HandleHttpResponse(restyForceDelResp, forceDelErr); forceDelErr != nil && !apierr.IsNotFound(forceDelErr) {
				log.Warn().Err(forceDelErr).Msgf("Prune: failed to purge Spider metadata for vNet %s (continuing)", vNetInfo.Id)
			}
		}

		// Remove VNet label
		if lErr := label.DeleteLabelObject(model.StrVNet, vNetInfo.Uid); lErr != nil {
			log.Warn().Err(lErr).Msgf("failed to delete label during prune for vNet %s", vNetInfo.Id)
		}

		// Delete VNet KV metadata
		delErr := kvstore.Delete(vNetKey)
		res := model.ResourcePruneResult{
			ResourceType:   model.StrVNet,
			ResourceId:     vNetInfo.Id,
			ConnectionName: vNetInfo.ConnectionName,
		}

		if delErr != nil {
			res.Success = false
			res.Error = delErr.Error()
			pruneResults.FailedCount++
		} else {
			res.Success = true
			res.Message = fmt.Sprintf("Orphaned metadata for vNet (%s) and its subnets pruned successfully", vNetInfo.Id)
			pruneResults.SuccessCount++
		}
		pruneResults.TotalPruned++
		pruneResults.Results = append(pruneResults.Results, res)
	}

	return pruneResults, nil
}

// purgeSpiderSubnetMetadata best-effort force-deletes one subnet's own Spider IID (a distinct resource, not auto-cleared with the parent VPC's).
func purgeSpiderSubnetMetadata(sInfo model.SubnetInfo) {
	if sInfo.CspVNetName == "" || sInfo.CspResourceName == "" {
		return
	}
	spReqt := spiderSubnetRemoveReq{ConnectionName: sInfo.ConnectionName}
	forceDelURL := fmt.Sprintf("%s/vpc/%s/subnet/%s?force=true", model.SpiderRestUrl, sInfo.CspVNetName, sInfo.CspResourceName)
	var spResp spiderBooleanInfoResp
	restyResp, forceDelErr := clientManager.ExecuteHttpRequest(
		clientManager.NewHttpClient(),
		"DELETE",
		forceDelURL,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)
	if forceDelErr = clientManager.HandleHttpResponse(restyResp, forceDelErr); forceDelErr != nil && !apierr.IsNotFound(forceDelErr) {
		log.Warn().Err(forceDelErr).Msgf("Prune: failed to purge Spider metadata for subnet %s (continuing)", sInfo.Id)
	}
}
