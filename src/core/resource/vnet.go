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
	"net"
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
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// NetworkStatus represents the status of a network resource.
type NetworkStatus string

const (

	// CRUD operations
	NetworkOnConfiguring NetworkStatus = "Configuring" // Resources are being configured.
	NetworkOnReading     NetworkStatus = "Reading"     // The network information is being read.
	NetworkOnUpdating    NetworkStatus = "Updating"    // The network is being updated.
	NetworkOnDeleting    NetworkStatus = "Deleting"    // The network is being deleted.
	// NetworkOnRefinining  NetworkStatus = "Refining"    // The network is being refined.

	// Register/deregister operations
	NetworkOnRegistering   NetworkStatus = "Registering"  // The network is being registered.
	NetworkOnDeregistering NetworkStatus = "Dergistering" // The network is being registered.

	// NetworkAvailable status
	NetworkAvailable NetworkStatus = "Available" // The network is fully created and ready for use.

	// In Use status
	NetworkInUse NetworkStatus = "InUse" // The network is currently in use.

	// Unknwon status
	NetworkUnknown NetworkStatus = "Unknown" // The network status is unknown.

	// NetworkError Handling
	NetworkError              NetworkStatus = "Error"              // An error occurred during a CRUD operation.
	NetworkErrorOnConfiguring NetworkStatus = "ErrorOnConfiguring" // An error occurred during the configuring operation.
	NetworkErrorOnReading     NetworkStatus = "ErrorOnReading"     // An error occurred during the reading operation.
	NetworkErrorOnUpdating    NetworkStatus = "ErrorOnUpdating"    // An error occurred during the updating operation.
	NetworkErrorOnDeleting    NetworkStatus = "ErrorOnDeleting"    // An error occurred during the deleting operation.
	NetworkErrorOnRegistering NetworkStatus = "ErrorOnRegistering" // An error occurred during the registering operation.
)

type NetworkAction string

const (
	ActionNone        NetworkAction = ""
	ActionRefine      NetworkAction = "refine"
	ActionForce       NetworkAction = "force"
	ActionWithSubnets NetworkAction = "withsubnets"
	// add additional actions here
)

var (
	stringToNetworkAction = map[string]NetworkAction{
		"":            ActionNone,
		"refine":      ActionRefine,
		"force":       ActionForce,
		"withsubnets": ActionWithSubnets,
	}

	actionsToDeleteSubnet = map[NetworkAction]bool{
		ActionRefine: true,
		ActionForce:  true,
		// add additional actions here
	}

	actionsToDeleteVNet = map[NetworkAction]bool{
		ActionRefine:      true,
		ActionForce:       true,
		ActionWithSubnets: true,
		// add additional actions here
	}
)

func ParseNetworkAction(s string) (NetworkAction, bool) {
	action, ok := stringToNetworkAction[strings.ToLower(s)]
	return action, ok
}

func (na NetworkAction) String() string {
	return string(na)
}

func (na NetworkAction) IsValidToDeleteSubnet() bool {
	return actionsToDeleteSubnet[na]
}

func (na NetworkAction) IsValidToDeleteVNet() bool {
	return actionsToDeleteVNet[na]
}

// VNetReqStructLevelValidation is a function to validate 'VNetReq' object.
func VNetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.VNetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

func ValidateVNetReq(vNetReq *model.VNetReq) error {
	log.Debug().Msg("ValidateVNetReq")
	log.Debug().Msgf("vNetReq: %+v", vNetReq)

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

	// * 3. Validates that each subnet's zone is valid in the region
	// TODO: Update the validation logic
	// It's a temporary validation logic due to the connection name pattern

	// Split the connection name into provider and region/zone
	parts := strings.SplitN(vNetReq.ConnectionName, "-", 2)
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

	// * 4. Validates that the CIDR block of the vNet and subnets are available for use in the CSP.
	// e.g., in available CIDR Blocks, not in the reserved CIDR Blocks, and etc.
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

	// * 5. Validates that the CIDR block of the vNet and subnets are valid
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
	log.Debug().Msgf("network: %+v", network)

	// Validate the network object
	err = netutil.ValidateNetwork(network)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

func ContainsZone(zones []string, zone string) bool {
	for _, z := range zones {
		if z == zone {
			return true
		}
	}
	return false
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
	// Note: GCP does not have VPC network CIDR block so skip the prefix length check
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
				err := fmt.Errorf("vNet CIDR block %s is in the reserved CIDR blocks (provider: %s)", vNetCidrBlock, provider)
				log.Warn().Msgf(err.Error())
				// return false, err
			}

			// Check if the vNet CIDR block is in the reserved CIDR blocks
			if reservedIpNet.Contains(vNetIpNet.IP) {
				err := fmt.Errorf("vNet CIDR block %s is in the reserved CIDR blocks (provider: %s)", vNetCidrBlock, provider)
				log.Warn().Msgf(err.Error())
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

// VPCRegisterRequest represents the request body for registering a VPC.
type spiderVPCRegisterRequest struct {
	ConnectionName string                       `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        spiderVPCRegisterRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderVPCRegisterRequestInfo struct {
	Name  string `json:"Name" validate:"required" example:"vpc-01"`
	CSPId string `json:"CSPId" validate:"required" example:"csp-vpc-1234"`
}

// CreateVPCRequest represents the request body for creating a VPC.
type spiderCreateVPCRequest struct {
	ConnectionName  string                     `json:"ConnectionName" validate:"required" example:"aws-connection"`
	IDTransformMode string                     `json:"IDTransformMode,omitempty" validate:"omitempty" example:"ON"` // ON: transform CSP ID, OFF: no-transform CSP ID
	ReqInfo         spiderCreateVPCRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderCreateVPCRequestInfo struct {
	Name           string                       `json:"Name" validate:"required" example:"vpc-01"`
	IPv4_CIDR      string                       `json:"IPv4_CIDR" validate:"omitempty"` // Some CSPs unsupported VPC CIDR
	SubnetInfoList []spiderAddSubnetRequestInfo `json:"SubnetInfoList" validate:"required"`
	TagList        []model.KeyValue             `json:"TagList,omitempty" validate:"omitempty"`
}

// type spiderListVPCReq struct {
// 	ConnectionName string `json:"ConnectionName" query:"ConnectionName" example:"aws-connection"`
// }

// type spiderListVPCResponse struct {
// 	Result []spiderVPCInfo `json:"vpc" validate:"required" description:"A list of VPC information"`
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
func CreateVNet(nsId string, vNetReq *model.VNetReq) (model.VNetInfo, error) {
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
	vNetInfo.ConnectionConfig, err = common.GetConnConfig(vNetInfo.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}
	vNetInfo.Description = vNetReq.Description
	// todo: restore the tag list later
	// vNetInfo.TagList = vNetReq.TagList

	// Note: Set subnetInfoList in vNetInfo in advance
	//       since each subnet uid must be consistent
	for _, subnetInfo := range vNetReq.SubnetInfoList {
		vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList, model.SubnetInfo{
			ResourceType: model.StrSubnet,
			Id:           subnetInfo.Name,
			Name:         subnetInfo.Name,
			Uid:          common.GenUid(),
			IPv4_CIDR:    subnetInfo.IPv4_CIDR,
			Zone:         subnetInfo.Zone,
			// todo: restore the tag list later
			// TagList:   subnetInfo.TagList,
		})
	}

	log.Debug().Msgf("vNetInfo(initial): %+v", vNetInfo)

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetInfo.Id)
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

	// [Set and store status]
	vNetInfo.Status = string(NetworkOnConfiguring)
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
	spReqt.ReqInfo.IPv4_CIDR = vNetReq.CidrBlock

	// Note: Use the subnets in the vNetInfo object (instead of the vNetReq object)
	//       since each subnet uid must be consistent
	for _, subnetInfo := range vNetInfo.SubnetInfoList {
		spReqt.ReqInfo.SubnetInfoList = append(spReqt.ReqInfo.SubnetInfoList, spiderAddSubnetRequestInfo{
			Name:      subnetInfo.Uid,
			IPv4_CIDR: subnetInfo.IPv4_CIDR,
			Zone:      subnetInfo.Zone,
			// todo: restore the tag list later
			// TagList:   subnetInfo.TagList,
		})
	}

	log.Debug().Msgf("spReqt: %+v", spReqt)

	client := resty.New()
	method := "POST"
	var spResp spiderVPCInfo

	// API to create a vNet
	url := fmt.Sprintf("%s/vpc", model.SpiderRestUrl)

	log.Debug().Msgf("[Request to Spider] Creating VPC (url: %s, request body: %+v)", url, spReqt)

	// Cleanup object when something goes wrong
	defer func() {
		// Only if this operation fails, the vNet will be deleted
		if err != nil && vNetInfo.Status == string(NetworkOnConfiguring) {
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

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Creating VPC (response body: %+v)", spResp)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspResourceId = spResp.IId.SystemId
	vNetInfo.CspResourceName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList
	// todo: restore the tag list later
	// vNetInfo.TagList = spResp.TagList

	// Note: Check one by one and update the vNet object with the response from the Spider
	//       since the order may differ different between slices
	for _, spSubnetInfo := range spResp.SubnetInfoList {
		for i, tbSubnetInfo := range vNetInfo.SubnetInfoList {
			if tbSubnetInfo.Uid == spSubnetInfo.IId.NameId {
				vNetInfo.SubnetInfoList[i].ResourceType = model.StrSubnet
				vNetInfo.SubnetInfoList[i].ConnectionName = vNetInfo.ConnectionName
				vNetInfo.SubnetInfoList[i].CspVNetId = spResp.IId.SystemId
				vNetInfo.SubnetInfoList[i].CspVNetName = spResp.IId.NameId
				vNetInfo.SubnetInfoList[i].Status = string(NetworkAvailable)
				vNetInfo.SubnetInfoList[i].CspResourceId = spSubnetInfo.IId.SystemId
				vNetInfo.SubnetInfoList[i].CspResourceName = spSubnetInfo.IId.NameId
				vNetInfo.SubnetInfoList[i].KeyValueList = spSubnetInfo.KeyValueList
				vNetInfo.SubnetInfoList[i].Zone = spSubnetInfo.Zone
				vNetInfo.SubnetInfoList[i].IPv4_CIDR = spSubnetInfo.IPv4_CIDR
				// todo: restore the tag list later
				// vNetInfo.SubnetInfoList[i].TagList = spSubnetInfo.TagList
			}
		}
	}

	// [Set and store status]
	if len(vNetInfo.SubnetInfoList) == 0 {
		vNetInfo.Status = string(NetworkAvailable)
	} else if len(vNetInfo.SubnetInfoList) > 0 {
		vNetInfo.Status = string(NetworkInUse)
	} else {
		vNetInfo.Status = string(NetworkUnknown)
		log.Warn().Msgf("The status of the vNet (%s) is unknown", vNetInfo.Id)
	}

	log.Debug().Msgf("vNetInfo(filled): %+v", vNetInfo)

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
			model.LabelStatus:          subnetInfo.Status,
			model.LabelDescription:     subnetInfo.Description,
			model.LabelZone:            subnetInfo.Zone,
			model.LabelVNetId:          vNetInfo.Id,
			model.LabelConnectionName:  vNetInfo.ConnectionName,
		}
		err = label.CreateOrUpdateLabel(model.StrSubnet, subnetInfo.Uid, subnetKey, labels)
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
		model.LabelStatus:          vNetInfo.Status,
		model.LabelDescription:     vNetInfo.Description,
		model.LabelConnectionName:  vNetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrVNet, vNetInfo.Uid, vNetKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return vNetInfo, nil
}

func GetVNet(nsId string, vNetId string) (model.VNetInfo, error) {
	log.Info().Msg("GetVNet")

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

	log.Debug().Msgf("vNetInfo: %+v", vNetInfo)

	/*
	 *	Get vNet info
	 */

	// [Via Spider] Get a vNet and subnets
	client := resty.New()
	method := "GET"
	spReqt := clientManager.NoBody
	var spResp spiderVPCInfo

	// API to create a vNet
	url := fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, vNetInfo.CspResourceName)
	queryParams := "?ConnectionName=" + vNetInfo.ConnectionName
	url += queryParams

	log.Debug().Msgf("[Request to Spider] Getting VPC (url: %s, request body: %+v)", url, spReqt)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Getting VPC (response body: %+v)", spResp)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspResourceId = spResp.IId.SystemId
	vNetInfo.CspResourceName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList
	// todo: restore the tag list later
	// vNetInfo.TagList = spResp.TagList

	// TODO: Check if it's required or not to save the vNet object
	// val, err := json.Marshal(vNetInfo)
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }

	// err = kvstore.Put(vNetKey, string(val))
	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }

	return vNetInfo, nil
}

// DeleteVNet accepts vNet creation request, creates and returns an TB vNet object
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
		log.Warn().Msgf(errMsg.Error())
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
	log.Debug().Msgf("subnetsKv: %+v", subnetsKv)

	// normal case: action == ""
	if action == ActionNone && len(subnetsKv) > 0 {
		err := fmt.Errorf("the vNet (%s) is in-use, may have subnets", vNetId)
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
		log.Warn().Msgf(err.Error())
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

	// [Set and store status]
	vNetInfo.Status = string(NetworkOnDeleting)
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
	// Sleep and retry if the vNet deletion fails
	for i := range trials {
		if i > 0 {
			log.Warn().Msgf("Retrying to delete vNet (%s) after %d seconds...", vNetId, seconds)
		}
		// Sleep for a while before retrying
		time.Sleep(time.Duration(seconds) * time.Second)

		log.Debug().Msgf("[Request to Spider] Deleting VPC (url: %s, request body: %+v)", url, spReqt)

		var spResp spiderBooleanInfoResp

		client := resty.New()
		method := "DELETE"

		err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(spReqt),
			&spReqt,
			&spResp,
			clientManager.MediumDuration,
		)

		log.Debug().Msgf("[Response from Spider] Deleting VPC (response body: %+v)", spResp)

		if err != nil {
			log.Error().Err(err).Msg("")
			continue
		}
		ok, err = strconv.ParseBool(spResp.Result)
		if err != nil {
			log.Error().Err(err).Msg("")
			continue
		}
		if ok {
			break
		}
	}

	// Finally, check if the vNet deletion was successful
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !ok {
		err := fmt.Errorf("failed to delete the vNet (%s)", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
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

func RefineVNet(nsId string, vNetId string) (model.SimpleMsg, error) {
	log.Info().Msg("RefineVNet")

	/*
	 *	[NOTE]
	 *	"Refine" operates based on information managed by Tumblebug.
	 *	Based on this information, it checks whether there is information/resource in Spider/CSP.
	 *	It removes the information managed by Tumblebug if there's no information/resource.
	 */

	// vNet object
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg
	var vNetInfo model.VNetInfo

	// Set the resource type
	resourceType := model.StrVNet

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

	/*
	 *	Check and refine the info of vNet and associated subnets
	 */

	// [Via Spider] Get a vNet
	client := resty.New()
	method := "GET"
	spReqt := clientManager.NoBody
	var spResp spiderVPCInfo

	// API to get a vNet
	url := fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, vNetInfo.CspResourceName)
	queryParams := "?ConnectionName=" + vNetInfo.ConnectionName
	url += queryParams

	log.Debug().Msgf("[Request to Spider] Refining VPC (url: %s, request body: %+v)", url, spReqt)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Refining VPC (response body: %+v)", spResp)

	// if err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return emptyRet, err
	// }

	if err == nil {
		err = fmt.Errorf("may not be refined, vNet info (id: %s) exists", vNetId)
		log.Warn().Err(err).Msg("")
		log.Info().Msgf("try to refine subnets once")

		// Read the stored subnets
		subnetKvList, err2 := kvstore.GetKvList(vNetKey + "/subnet")
		if err2 != nil {
			log.Warn().Err(err2).Msg("")
			return emptyRet, err2
		}

		for _, subnetKv := range subnetKvList {
			subnetInfo := model.SubnetInfo{}
			err2 = json.Unmarshal([]byte(subnetKv.Value), &subnetInfo)
			if err2 != nil {
				log.Warn().Err(err2).Msg("")
				// return emptyRet, err
			}
			log.Debug().Msgf("subnetInfo: %+v", subnetInfo)

			_, err2 := RefineSubnet(nsId, vNetId, subnetInfo.Id)
			if err2 != nil {
				log.Warn().Err(err2).Msg("")
				// return emptyRet, err
			}
		}

		// [Output]
		ret.Message = err.Error()
		return ret, err
	}

	/*
	 * In case of the VPC info/resource does not exist in Spider/CSP
	 * delete the information of vNet and subnets from the key-value stores
	 */

	// Delete subnet objects from the key-value store
	// Read the stored subnets
	subnetKvList, err := kvstore.GetKvList(vNetKey + "/subnet")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	for _, subnetKv := range subnetKvList {
		// Save the subnet object into the key-value store
		err = kvstore.Delete(subnetKv.Key)
		if err != nil {
			log.Warn().Err(err).Msg("")
			// return emptyRet, err
		}

		err = label.DeleteLabelObject(model.StrSubnet, vNetInfo.Uid)
		if err != nil {
			log.Warn().Err(err).Msg("")
			// return emptyRet, err
		}
	}

	// Delete the saved the vNet info
	err = kvstore.Delete(vNetKey)
	if err != nil {
		log.Warn().Err(err).Msg("")
		// return emptyRet, err
	}

	// Remove label info using DeleteLabelObject
	// labels := map[string]string{
	// 	"sys.manager":  model.StrManager,
	// 	"namespace": nsId,
	// }
	err = label.DeleteLabelObject(model.StrVNet, vNetInfo.Uid)
	if err != nil {
		log.Warn().Err(err).Msg("")
		// return emptyRet, err
	}

	// Get and check the subnet info still exists or not
	vNetKv, exists, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Warn().Err(err).Msg("")
		// return emptyRet, err
	}
	if exists {
		err := fmt.Errorf("fail to refine the vNet info (%s)", vNetKv)
		ret.Message = err.Error()
		return ret, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the vNet info (%s) has been refined", vNetId)

	log.Info().Msgf("vNet (%s) has been refined", vNetId)
	return ret, nil
}

// RegisterVNet accepts vNet registration request, register and returns an TB vNet object
func RegisterVNet(nsId string, vNetRegisterReq *model.RegisterVNetReq) (model.VNetInfo, error) {
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

	// [Set and store status]
	vNetInfo.Status = string(NetworkOnRegistering)
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

	client := resty.New()
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

	log.Debug().Msgf("[Request to Spider] Registering VPC (url: %s, request body: %+v)", url, spReqt)

	// Clean up the vNet object when something goes wrong
	defer func() {
		// Only if this operation fails, the vNet will be deleted
		if err != nil && vNetInfo.Status == string(NetworkOnRegistering) {
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

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Registering VPC (response body: %+v)", spResp)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspResourceId = spResp.IId.SystemId
	vNetInfo.CspResourceName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList
	// todo: restore the tag list later
	// vNetInfo.TagList = spResp.TagList

	if vNetRegisterReq.CspResourceId != "" {
		vNetInfo.SystemLabel = "Registered from CSP resource"
	} else if vNetRegisterReq.CspResourceId == "" {
		vNetInfo.SystemLabel = "Registered from CB-Spider resource"
	}

	// Note: Check one by one and update the vNet object with the response from the Spider
	//       since the order may differ different between slices
	for i, spSubnetInfo := range spResp.SubnetInfoList {
		subnetInfo := model.SubnetInfo{
			Id:              fmt.Sprintf("reg-subnet-%02d", i+1),
			Name:            fmt.Sprintf("reg-subnet-%02d", i+1),
			Uid:             common.GenUid(),
			ConnectionName:  vNetInfo.ConnectionName,
			Status:          string(NetworkUnknown),
			CspResourceId:   spSubnetInfo.IId.SystemId,
			CspResourceName: spSubnetInfo.IId.NameId,
			CspVNetId:       spResp.IId.SystemId,
			CspVNetName:     spResp.IId.NameId,
			KeyValueList:    spSubnetInfo.KeyValueList,
			Zone:            spSubnetInfo.Zone,
			IPv4_CIDR:       spSubnetInfo.IPv4_CIDR,
			// todo: restore the tag list later
			// TagList:        spSubnetInfo.TagList,
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
			model.LabelStatus:          subnetInfo.Status,
			model.LabelDescription:     subnetInfo.Description,
			model.LabelZone:            subnetInfo.Zone,
			model.LabelVNetId:          vNetInfo.Id,
			model.LabelConnectionName:  vNetInfo.ConnectionName,
		}
		err = label.CreateOrUpdateLabel(model.StrSubnet, subnetInfo.Uid, subnetKey, labels)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

	}

	log.Debug().Msgf("vNetInfo: %+v", vNetInfo)

	// [Set and store status]
	vNetInfo.Status = string(NetworkAvailable)
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
		model.LabelStatus:          vNetInfo.Status,
		model.LabelDescription:     vNetInfo.Description,
		model.LabelConnectionName:  vNetInfo.ConnectionName,
	}
	err = label.CreateOrUpdateLabel(model.StrVNet, vNetInfo.Uid, vNetKey, labels)
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
		log.Warn().Msgf(errMsg.Error())
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
	log.Debug().Msgf("subnetsKv: %+v", subnetsKv)

	if withSubnets == "false" && len(subnetsKv) > 0 {
		err := fmt.Errorf("the vNet (%s) is in-use, may have subnets", vNetId)
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

	// Set status to 'Deleting'
	vNetInfo.Status = string(NetworkOnDeleting)
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

	log.Debug().Msgf("[Request to Spider] Deregistering VPC (url: %s, request body: %+v)", url, spReqt)

	var spResp spiderBooleanInfoResp

	client := resty.New()
	method := "DELETE"

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		clientManager.MediumDuration,
	)

	log.Debug().Msgf("[Response from Spider] Deregistering VPC (response body: %+v)", spResp)

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
		err := fmt.Errorf("failed to deregister the vNet (%s)", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
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

				// Convert string to integer and check if it's valid
				subnetCount, err := strconv.Atoi(vnet.SubnetCount)
				if err != nil {
					log.Error().Err(err).Msg("Failed to convert SubnetCount to integer")
					return model.VNetDesignResponse{}, err
				}

				hostsPerSubent, err := strconv.Atoi(vnet.HostsPerSubnet)
				if err != nil {
					log.Error().Err(err).Msg("Failed to convert HostsPerSubnet to integer")
					return model.VNetDesignResponse{}, err
				}

				useFirstNZones, err := strconv.Atoi(vnet.UseFirstNZones)
				if err != nil {
					log.Error().Err(err).Msg("Failed to convert UseFirstNZones to integer")
					return model.VNetDesignResponse{}, err
				}

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
