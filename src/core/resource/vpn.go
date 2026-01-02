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
	"math"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	terrariumModel "github.com/cloud-barista/mc-terrarium/pkg/api/rest/model"
	"github.com/rs/zerolog/log"
)

const (
	minWaitDuration = 10 * time.Second
	maxWaitDuration = 120 * time.Second
)

// supportedCspForVpn is a map of supported CSPs for VPN connections.
// The keys are the hub CSPs, and the values are slices of supported spoke CSPs.
var supportedCspForVpn = map[string][]string{
	csp.AWS: {csp.Azure, csp.GCP, csp.Alibaba, csp.Tencent, csp.IBM},
	// csp.Azure:   {csp.AWS, csp.GCP},
	// csp.GCP:     {csp.AWS, csp.Azure},
	// csp.Alibaba: {csp.AWS},
	// csp.Tencent: {csp.AWS},
	// csp.IBM:     {csp.AWS},
}

// validCspPairsForVpn is a map of valid CSP pairs for VPN connections.
var validCspPairsForVpn = func() map[string]map[string]bool {
	cspPairs := make(map[string]map[string]bool)

	// Initialize maps for each CSP pair
	for hub, spokes := range supportedCspForVpn {
		if cspPairs[hub] == nil {
			cspPairs[hub] = make(map[string]bool)
		}

		// Add valid pairs for current hub CSP
		for _, spoke := range spokes {
			// Add forward direction (hub -> spoke)
			cspPairs[hub][spoke] = true

			// Add reverse direction (spoke -> hub)
			if cspPairs[spoke] == nil {
				cspPairs[spoke] = make(map[string]bool)
			}
			cspPairs[spoke][hub] = true
		}
	}

	return cspPairs
}()

func IsValidCspPairForVpn(csp1, csp2 string) (bool, error) {
	valid, exists := validCspPairsForVpn[csp1][csp2]
	if !exists || !valid {
		return false, fmt.Errorf("currently not supported, VPN between %s and %s", csp1, csp2)
	}
	return true, nil
}

// GetSiteToSiteVPN returns a site-to-site VPN
func GetAllSiteToSiteVPN(nsId string, mciId string) (model.VpnInfoList, error) {

	var emptyRet model.VpnInfoList
	var vpnInfoList model.VpnInfoList
	vpnInfoList.VpnInfoList = []model.VpnInfo{}
	var err error = nil
	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrVPN

	// Set vpnKeyPrefix for the site-to-site VPN objects
	vpnKeyPrefix := "/ns/" + nsId + "/resources/" + resourceType

	// Read the stored VPN info
	vpnKvs, err := kvstore.GetKvList(vpnKeyPrefix)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	for _, vpnKv := range vpnKvs {
		tempVpnInfo := model.VpnInfo{}
		err = json.Unmarshal([]byte(vpnKv.Value), &tempVpnInfo)
		if err != nil {
			log.Warn().Err(err).Msg("")
		}

		vpnInfoList.VpnInfoList = append(vpnInfoList.VpnInfoList, tempVpnInfo)
	}

	return vpnInfoList, nil
}

// GetAllIDsOfSiteToSiteVPN returns a list of site-to-site VPN IDs
func GetAllIDsOfSiteToSiteVPN(nsId string, mciId string) (model.VpnIdList, error) {

	var emptyRet model.VpnIdList
	var vpnIdList model.VpnIdList
	vpnIdList.VpnIdList = []string{}
	var err error = nil

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrVPN

	// Set vpnKeyPrefix for the site-to-site VPN objects
	vpnKeyPrefix := "/ns/" + nsId + "/resources/" + resourceType

	// Read the stored VPN info
	vpnKvs, err := kvstore.GetKvList(vpnKeyPrefix)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	for _, vpnKv := range vpnKvs {
		tempVpnInfo := model.VpnInfo{}
		err = json.Unmarshal([]byte(vpnKv.Value), &tempVpnInfo)
		if err != nil {
			log.Warn().Err(err).Msg("")
		}

		vpnIdList.VpnIdList = append(vpnIdList.VpnIdList, tempVpnInfo.Id)
	}

	return vpnIdList, nil
}

// CreateSiteToSiteVPN creates a site-to-site VPN via Terrarium
func CreateSiteToSiteVPN(nsId string, mciId string, vpnReq *model.RestPostVpnRequest, retry string) (model.VpnInfo, error) {

	// VPN objects
	var emptyRet model.VpnInfo
	var vpnInfo model.VpnInfo
	var err error = nil
	var retried bool = (retry == "retry")

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vpnReq.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Get each vnet info
	vNetInfo1, err := GetVNet(nsId, vpnReq.Site1.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	vNetInfo2, err := GetVNet(nsId, vpnReq.Site2.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the CSPs are valid for VPN
	ok, err := IsValidCspPairForVpn(vNetInfo1.ConnectionConfig.ProviderName, vNetInfo2.ConnectionConfig.ProviderName)
	if !ok {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// Ensure vnetInfo1 is the hub and vnetInfo2 is the spoke
	_, exists := supportedCspForVpn[vNetInfo1.ConnectionConfig.ProviderName]
	if !exists {
		vNetInfo1, vNetInfo2 = vNetInfo2, vNetInfo1
		vpnReq.Site1, vpnReq.Site2 = vpnReq.Site2, vpnReq.Site1
	}

	site1CspName := vNetInfo1.ConnectionConfig.ProviderName
	site2CspName := vNetInfo2.ConnectionConfig.ProviderName

	// Set the resource type
	resourceType := model.StrVPN

	// Set the vpn object in advance
	uid := common.GenUid()
	vpnInfo.ResourceType = resourceType
	vpnInfo.Name = vpnReq.Name
	vpnInfo.Id = vpnReq.Name
	vpnInfo.Uid = uid
	vpnInfo.Description = "VPN between " + site1CspName + " and " + site2CspName

	site1Detail := model.VpnSiteDetail{}
	site1Detail.ConnectionName = vNetInfo1.ConnectionName
	site1Detail.ConnectionConfig, err = common.GetConnConfig(site1Detail.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	site2Detail := model.VpnSiteDetail{}
	site2Detail.ConnectionName = vNetInfo2.ConnectionName
	site2Detail.ConnectionConfig, err = common.GetConnConfig(site2Detail.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	vpnInfo.VpnSites = []model.VpnSiteDetail{site1Detail, site2Detail}

	// Set a vpnKey for the site-to-site VPN object
	vpnKey := common.GenResourceKey(nsId, resourceType, vpnInfo.Id)
	// Check if the vpn already exists or not
	exists, err = CheckResource(nsId, resourceType, vpnInfo.Id)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the site-to-site VPN (%s) exists or not", vpnInfo.Id)
		return emptyRet, err
	}
	// For retry, read the stored VPN info if exists
	if exists {
		if !retried {
			log.Error().Err(err).Msg("")
			err := fmt.Errorf("already exists, site-to-site VPN: %s", vpnInfo.Id)
			return emptyRet, err
		}

		// Read the stored VPN info
		vpnKv, _, err := kvstore.GetKv(vpnKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		err = json.Unmarshal([]byte(vpnKv.Value), &vpnInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// [Set and store status]
	vpnInfo.Status = string(NetworkOnConfiguring)
	val, err := json.Marshal(vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vpnKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("vpnInfo(initial): %+v", vpnInfo)

	/*
	 * [Via Terrarium] Create a Site-to-Site VPN
	 */

	// Initialize resty client with basic auth
	client := clientManager.NewHttpClient()

	// Set Terrarium endpoint
	epTerrarium := model.TerrariumRestUrl

	// Set a terrarium ID
	trName := vpnInfo.Uid
	// init trId
	trId := trName

	// Check the CSPs of the sites
	switch site1CspName {
	case csp.AWS:

		if !retried {
			// Issue a terrarium
			method := "POST"
			url := fmt.Sprintf("%s/tr", epTerrarium)
			reqTr := new(terrariumModel.TerrariumCreationRequest)
			reqTr.Name = trName
			reqTr.Description = fmt.Sprintf("VPN between %s and %s", site1CspName, site2CspName)

			resTrInfo := new(terrariumModel.TerrariumInfo)

			err = clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(*reqTr),
				reqTr,
				resTrInfo,
				clientManager.VeryShortDuration,
			)

			if err != nil {
				log.Err(err).Msg("")
				return emptyRet, err
			}

			log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
			log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

			// set trId
			trId = resTrInfo.Id
		}

		// * Set request body to create site-to-site VPN as csp pair
		reqInfracode := new(terrariumModel.CreateAwsToSiteVpnRequest)

		log.Debug().Msgf("vNetInfo1: %+v", vNetInfo1)

		// Set Hub site (AWS)
		reqInfracode.VpnConfig.Aws.Region = vNetInfo1.ConnectionConfig.RegionDetail.RegionId
		reqInfracode.VpnConfig.Aws.VpcId = vNetInfo1.CspResourceId
		if len(vNetInfo1.SubnetInfoList) < 1 {
			log.Error().Msgf("No subnets found for VPC ID: %s", vNetInfo1.Id)
			return emptyRet, fmt.Errorf("no subnets found for VPC ID: %s", vNetInfo1.Id)
		}
		reqInfracode.VpnConfig.Aws.SubnetId = vNetInfo1.SubnetInfoList[0].CspResourceId
		reqInfracode.VpnConfig.Aws.BgpAsn = vpnReq.Site1.CspSpecificProperty.Aws.BgpAsn

		log.Debug().Msgf("vNetInfo2: %+v", vNetInfo2)

		// Set spoke site
		switch site2CspName {
		case csp.Azure:
			reqInfracode.VpnConfig.TargetCsp.Type = csp.Azure
			reqInfracode.VpnConfig.TargetCsp.Azure = new(terrariumModel.AzureConfig)
			reqInfracode.VpnConfig.TargetCsp.Azure.Region = vNetInfo2.ConnectionConfig.RegionDetail.RegionId
			reqInfracode.VpnConfig.TargetCsp.Azure.VirtualNetworkName = vNetInfo2.CspResourceName // * Azure uses CspResourceName
			reqInfracode.VpnConfig.TargetCsp.Azure.ResourceGroupName = vNetInfo2.ConnectionConfig.RegionDetail.RegionId
			reqInfracode.VpnConfig.TargetCsp.Azure.BgpAsn = vpnReq.Site2.CspSpecificProperty.Azure.BgpAsn
			reqInfracode.VpnConfig.TargetCsp.Azure.VpnSku = vpnReq.Site2.CspSpecificProperty.Azure.VpnSku

			// ! Warning: This is a temporary solution
			// Get the last subnet CIDR block
			subnetCount := len(vNetInfo2.SubnetInfoList)
			if subnetCount == 0 {
				log.Error().Msgf("No subnets found for VNet ID: %s", vNetInfo2.Id)
				return emptyRet, fmt.Errorf("no subnets found for VNet ID: %s", vNetInfo2.Id)
			}

			lastSubnet := vNetInfo2.SubnetInfoList[subnetCount-1]
			lastSubnetCidr := lastSubnet.IPv4_CIDR

			// Calculate the next subnet CIDR block
			nextCidr, err := netutil.NextSubnet(lastSubnetCidr, vNetInfo2.CidrBlock)
			if err != nil {
				log.Warn().Msgf("Failed to get the next subnet CIDR")
			}
			// Set the next subnet CIDR block as GatewaySubnet CIDR
			reqInfracode.VpnConfig.TargetCsp.Azure.GatewaySubnetCidr = nextCidr

		case csp.GCP:
			reqInfracode.VpnConfig.TargetCsp.Type = csp.GCP
			reqInfracode.VpnConfig.TargetCsp.Gcp = new(terrariumModel.GcpConfig)
			reqInfracode.VpnConfig.TargetCsp.Gcp.Region = vNetInfo2.ConnectionConfig.RegionDetail.RegionId
			reqInfracode.VpnConfig.TargetCsp.Gcp.VpcNetworkName = vNetInfo2.CspResourceId
			reqInfracode.VpnConfig.TargetCsp.Gcp.BgpAsn = vpnReq.Site2.CspSpecificProperty.Gcp.BgpAsn

		case csp.Alibaba:
			reqInfracode.VpnConfig.TargetCsp.Type = csp.Alibaba
			reqInfracode.VpnConfig.TargetCsp.Alibaba = new(terrariumModel.AlibabaConfig)
			reqInfracode.VpnConfig.TargetCsp.Alibaba.Region = vNetInfo2.ConnectionConfig.RegionDetail.RegionId
			reqInfracode.VpnConfig.TargetCsp.Alibaba.VpcId = vNetInfo2.CspResourceId
			if len(vNetInfo2.SubnetInfoList) == 1 {
				reqInfracode.VpnConfig.TargetCsp.Alibaba.VswitchId1 = vNetInfo2.SubnetInfoList[0].CspResourceId
				reqInfracode.VpnConfig.TargetCsp.Alibaba.VswitchId2 = vNetInfo2.SubnetInfoList[0].CspResourceId
			} else if len(vNetInfo2.SubnetInfoList) == 2 {
				reqInfracode.VpnConfig.TargetCsp.Alibaba.VswitchId1 = vNetInfo2.SubnetInfoList[0].CspResourceId
				reqInfracode.VpnConfig.TargetCsp.Alibaba.VswitchId2 = vNetInfo2.SubnetInfoList[1].CspResourceId
			}
			reqInfracode.VpnConfig.TargetCsp.Alibaba.BgpAsn = vpnReq.Site2.CspSpecificProperty.Alibaba.BgpAsn

		case csp.Tencent:
			reqInfracode.VpnConfig.TargetCsp.Type = csp.Tencent
			reqInfracode.VpnConfig.TargetCsp.Tencent = new(terrariumModel.TencentConfig)
			reqInfracode.VpnConfig.TargetCsp.Tencent.Region = vNetInfo2.ConnectionConfig.RegionDetail.RegionId
			reqInfracode.VpnConfig.TargetCsp.Tencent.VpcId = vNetInfo2.CspResourceId
			if len(vNetInfo2.SubnetInfoList) >= 1 {
				reqInfracode.VpnConfig.TargetCsp.Tencent.SubnetId = vNetInfo2.SubnetInfoList[0].CspResourceId
			}

		case csp.IBM:
			reqInfracode.VpnConfig.TargetCsp.Type = csp.IBM
			reqInfracode.VpnConfig.TargetCsp.Ibm = new(terrariumModel.IbmConfig)
			reqInfracode.VpnConfig.TargetCsp.Ibm.Region = vNetInfo2.ConnectionConfig.RegionDetail.RegionId
			reqInfracode.VpnConfig.TargetCsp.Ibm.VpcId = vNetInfo2.CspResourceId
			reqInfracode.VpnConfig.TargetCsp.Ibm.VpcCidr = vNetInfo2.CidrBlock
			if len(vNetInfo2.SubnetInfoList) >= 1 {
				reqInfracode.VpnConfig.TargetCsp.Ibm.SubnetId = vNetInfo2.SubnetInfoList[0].CspResourceId
			}

		default:
			err = fmt.Errorf("not supported, %s", site2CspName)
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		// * Create site-to-site VPN
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/vpn/aws-to-site", epTerrarium, trId)

		resInfracode := new(model.Response)

		err = clientManager.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			clientManager.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			clientManager.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resVpnCreation: %+v", resInfracode.Message)
		log.Trace().Msgf("resVpnCreation: %+v", resInfracode.Detail)

		// * Retrieve the VPN info recursively until the VPN is created
		// Recursively call the function to get the VPN info
		// An expected completion duration is 15 minutes
		expectedCompletionDuration := 15 * time.Minute

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()

		ret, err := retrieveEnrichmentsInfoInTerrarium(ctx, trId, "vpn/aws-to-site", expectedCompletionDuration)
		if err != nil {
			log.Err(err).Msg("")
		}

		// Set the VPN info
		jsonData, err := json.Marshal(ret.Object)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		// Use a map on unmarshaling the JSON data instead of terrariumModel.TerrariumInfo
		// for better data extraction
		var trVpnInfo map[string]interface{}
		err = json.Unmarshal(jsonData, &trVpnInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		log.Debug().Msgf("trVpnInfo: %v", trVpnInfo)

		// Extract the detail of CSPs' resources (NOTE: currently Terrarium supports AWS-to-site VPN)
		cspResources, exists := trVpnInfo[site1CspName].(map[string]interface{})
		if !exists {
			log.Error().Msgf("AWS resources not found in VPN info")
		}
		vpnInfo.VpnSites[0].ResourceDetails = extractResourceDetails(cspResources)

		cspResources2, exists2 := trVpnInfo[site2CspName].(map[string]interface{})
		if !exists2 {
			log.Error().Msgf("%s resources not found in VPN info", site2CspName)
		}
		vpnInfo.VpnSites[1].ResourceDetails = extractResourceDetails(cspResources2)

	default:
		log.Warn().Msgf("invalid CSP set: %s and %s", site1CspName, site2CspName)
	}

	// [Set and store status]
	vpnInfo.Status = string(NetworkAvailable)

	log.Debug().Msgf("vpnInfo(final): %+v", vpnInfo)

	value, err := json.Marshal(vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vpnKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the vpn info is stored
	vpnKv, exists, err := kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, vpn: %s", vpnInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(vpnKv.Value), &vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:     model.StrManager,
		model.LabelNamespace:   nsId,
		model.LabelLabelType:   model.StrVPN,
		model.LabelId:          vpnInfo.Id,
		model.LabelName:        vpnInfo.Name,
		model.LabelUid:         vpnInfo.Uid,
		model.LabelStatus:      vpnInfo.Status,
		model.LabelDescription: vpnInfo.Description,
	}
	err = label.CreateOrUpdateLabel(model.StrVPN, vpnInfo.Uid, vpnKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return vpnInfo, nil
}

func retrieveEnrichmentsInfoInTerrarium(ctx context.Context, trId string, enrichments string, expectedCompletionDuration time.Duration) (model.Response, error) {

	var emptyRet model.Response

	// Initialize resty client with basic auth
	client := clientManager.NewHttpClient()

	// Set Terrarium endpoint
	epTerrarium := model.TerrariumRestUrl

	requestBody := clientManager.NoBody
	resRetrieving := new(model.Response)

	// Set the start time and the expected end time
	startTime := time.Now()
	expectedEndTime := startTime.Add(expectedCompletionDuration)

	// Set wait duration
	currentWaitDuration := maxWaitDuration

	for {
		err := clientManager.ExecuteHttpRequest(
			client,
			"GET",
			fmt.Sprintf("%s/tr/%s/%s?detail=refined", epTerrarium, trId, enrichments),
			nil,
			clientManager.SetUseBody(requestBody),
			&requestBody,
			resRetrieving,
			clientManager.VeryShortDuration,
		)

		if err == nil {
			log.Info().Msgf("successfully retrieve the enrichments (%s)", enrichments)
			return *resRetrieving, nil
		}

		elapsedTime := time.Since(startTime)
		currentWaitDuration = calculateWaitDuration(elapsedTime, expectedCompletionDuration, expectedEndTime)

		minutes := int(elapsedTime.Minutes())
		seconds := int(elapsedTime.Seconds()) % 60
		log.Info().Msgf("[Elapsed time: %dm%ds] Creating enrichments (%s), retrying in %s...", minutes, seconds, enrichments, currentWaitDuration)

		select {
		case <-ctx.Done():
			log.Info().Msg("Context timeout reached.")
			return emptyRet, ctx.Err()
		case <-time.After(currentWaitDuration):
			continue
		}
	}
}

func calculateWaitDuration(elapsedTime time.Duration, expectedCompletionDuration time.Duration, expectedEndTime time.Time) time.Duration {
	if time.Now().After(expectedEndTime) {
		return minWaitDuration
	}

	progress := float64(elapsedTime) / float64(expectedCompletionDuration)
	k := 16.0  // Slope of the curve (higher, steeper)
	x0 := 0.65 // Inflection point position (65% point)

	sigmoid := 1.0 - (1.0 / (1.0 + math.Exp(-k*(progress-x0))))

	// guarantee minWaitDuration (10s),  e.g. math.Floor(10 + (120-10) * sigmoid)
	waitSeconds := math.Floor(minWaitDuration.Seconds() +
		(maxWaitDuration.Seconds()-minWaitDuration.Seconds())*sigmoid)

	return time.Duration(waitSeconds) * time.Second
}

// extractResourceDetails collects all resource details from a CSP's data map
func extractResourceDetails(cspData map[string]interface{}) []model.ResourceDetail {
	var details []model.ResourceDetail

	// Process top-level resources
	for _, value := range cspData {
		// Handle different types of values
		switch v := value.(type) {
		case map[string]interface{}:
			// Process map values
			resourceDetails := processResourceMap(v)
			details = append(details, resourceDetails...)

		case []interface{}:
			// Process array/list values
			resourceDetails := processResourceArray(v)
			details = append(details, resourceDetails...)
		}
	}

	return details
}

// processResourceMap extracts resource details from a map
func processResourceMap(resourceMap map[string]interface{}) []model.ResourceDetail {
	var details []model.ResourceDetail

	var resourceDetail model.ResourceDetail

	// Check if this resource has id fields
	if id, hasId := resourceMap["id"].(string); hasId {
		resourceDetail.CspResourceId = id
	}

	// Check if this resource has name fields
	if name, hasName := resourceMap["name"].(string); hasName {
		resourceDetail.CspResourceName = name
	}

	// Set the resource detail
	resourceDetail.CspResourceDetail = resourceMap

	details = append(details, resourceDetail)

	return details
}

// processResourceArray extracts resource details from an array/slice
func processResourceArray(array []interface{}) []model.ResourceDetail {
	var details []model.ResourceDetail

	for _, item := range array {
		// Process each item in the array
		if resourceMap, ok := item.(map[string]interface{}); ok {
			itemDetails := processResourceMap(resourceMap)
			details = append(details, itemDetails...)
		}
	}

	return details
}

// GetSiteToSiteVPN returns a site-to-site VPN via Terrarium
func GetSiteToSiteVPN(nsId string, mciId string, vpnId string, detail string) (model.VpnInfo, error) {

	var emptyRet model.VpnInfo
	var vpnInfo model.VpnInfo
	var err error = nil
	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vpnId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrVPN

	// Set a vpnKey for the site-to-site VPN object
	vpnKey := common.GenResourceKey(nsId, resourceType, vpnId)
	// Check if the VPN already exists or not
	exists, err := CheckResource(nsId, resourceType, vpnId)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the site-to-site VPN (%s) exists or not", vpnId)
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, site-to-site VPN: %s", vpnId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored VPN info
	vpnKv, _, err := kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(vpnKv.Value), &vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Initialize resty client with basic auth
	client := clientManager.NewHttpClient()

	trId := vpnInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := clientManager.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

	// e.g. "vpn/gcp-aws"
	enrichments := resTrInfo.Enrichments

	// Get resource info
	method = "GET"
	url = fmt.Sprintf("%s/tr/%s/%s?detail=%s", epTerrarium, trId, enrichments, detail)
	requestBody = clientManager.NoBody
	resResourceInfo := new(model.Response)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		resResourceInfo,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	jsonData, err := json.Marshal(resResourceInfo.Object)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	// Use a map on unmarshaling the JSON data instead of terrariumModel.TerrariumInfo
	// for better data extraction
	var trVpnInfo map[string]interface{}
	err = json.Unmarshal(jsonData, &trVpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	log.Debug().Msgf("trVpnInfo: %v", trVpnInfo)

	// Extract the detail of CSPs' resources (NOTE: currently Terrarium supports AWS-to-site VPN)
	for _, provider := range resTrInfo.Providers {
		if provider == csp.AWS {
			cspResources, exists := trVpnInfo[provider].(map[string]interface{})
			if !exists {
				log.Error().Msgf("AWS resources not found in VPN info")
			}
			vpnInfo.VpnSites[0].ResourceDetails = extractResourceDetails(cspResources)
		} else {
			cspResources, exists := trVpnInfo[provider].(map[string]interface{})
			if !exists {
				log.Error().Msgf("%s resources not found in VPN info", provider)
			}
			vpnInfo.VpnSites[1].ResourceDetails = extractResourceDetails(cspResources)
		}
	}

	log.Debug().Msgf("vpnInfo(final): %+v", vpnInfo)

	value, err := json.Marshal(vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vpnKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the vpn info is stored
	vpnKv, exists, err = kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, vpn: %s", vpnInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(vpnKv.Value), &vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return vpnInfo, nil
}

// DeleteSiteToSiteVPN deletes a site-to-site VPN via Terrarium
func DeleteSiteToSiteVPN(nsId string, mciId string, vpnId string) (model.SimpleMsg, error) {

	// VPN objects
	var emptyRet model.SimpleMsg
	var vpnInfo model.VpnInfo
	var err error = nil

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vpnId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrVPN

	// Set a vpnKey for the site-to-site VPN object
	vpnKey := common.GenResourceKey(nsId, resourceType, vpnId)
	// Check if the VPN already exists or not
	exists, err := CheckResource(nsId, resourceType, vpnId)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the site-to-site VPN (%s) exists or not", vpnId)
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, site-to-site VPN: %s", vpnId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored VPN info
	vpnKv, exists, err := kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(vpnKv.Value), &vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Set and store status]
	vpnInfo.Status = string(NetworkOnDeleting)
	val, err := json.Marshal(vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vpnKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Initialize resty client with basic auth
	client := clientManager.NewHttpClient()

	trId := vpnInfo.Uid

	// Set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := clientManager.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
	enrichments := resTrInfo.Enrichments

	// Delete aws-to-site VPN (enrichments example: "vpn/aws-to-site")
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s/%s", epTerrarium, trId, enrichments)
	requestBody = clientManager.NoBody
	resDeleteSiteToSiteVpn := new(model.Response)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		resDeleteSiteToSiteVpn,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteSiteToSiteVpn: %+v", resDeleteSiteToSiteVpn.Message)
	log.Trace().Msgf("resDeleteSiteToSiteVpn: %+v", resDeleteSiteToSiteVpn.Detail)

	// ! TBD

	// // delete terrarium
	// method = "DELETE"
	// url = fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	// requestBody = clientManager.NoBody
	// resDeleteTr := new(model.Response)

	// err = clientManager.ExecuteHttpRequest(
	// 	client,
	// 	method,
	// 	url,
	// 	nil,
	// 	clientManager.SetUseBody(requestBody),
	// 	&requestBody,
	// 	resDeleteTr,
	// 	clientManager.VeryShortDuration,
	// )

	// if err != nil {
	// 	log.Err(err).Msg("")
	// 	return emptyRet, err
	// }

	// log.Debug().Msgf("resDeleteTr: %+v", resDeleteTr.Message)
	// log.Trace().Msgf("resDeleteTr: %+v", resDeleteTr.Detail)

	// [Set and store status]
	err = kvstore.Delete(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Remove label info using DeleteLabelObject
	err = label.DeleteLabelObject(model.StrVPN, vpnInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	msg := fmt.Sprintf("successfully deleted the site-to-site VPN (%s)", vpnId)
	log.Debug().Msgf("msg: %s", msg)

	res := model.SimpleMsg{
		Message: msg,
	}

	return res, nil
}

// GetRequestStatusOfSiteToSiteVpn checks the status of a specific request
func GetRequestStatusOfSiteToSiteVpn(nsId string, mciId string, vpnId string, reqId string) (model.Response, error) {

	var emptyRet model.Response
	var vpnInfo model.VpnInfo
	var err error = nil

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vpnId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrVPN

	// Set a vpnKey for the site-to-site VPN object
	vpnKey := common.GenResourceKey(nsId, resourceType, vpnId)
	// Check if the vpn already exists or not
	exists, err := CheckResource(nsId, resourceType, vpnId)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the site-to-site VPN (%s) exists or not", vpnId)
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, site-to-site VPN: %s", vpnId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored VPN info
	vpnKv, _, err := kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(vpnKv.Value), &vpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Initialize resty client with basic auth
	client := clientManager.NewHttpClient()

	trId := vpnInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := clientManager.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
	enrichments := resTrInfo.Enrichments

	// Get resource info
	method = "GET"
	url = fmt.Sprintf("%s/tr/%s/%s/request/%s", epTerrarium, trId, enrichments, reqId)
	reqReqStatus := clientManager.NoBody
	resReqStatus := new(model.Response)

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(reqReqStatus),
		&reqReqStatus,
		resReqStatus,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("resReqStatus: %+v", resReqStatus.Detail)

	return *resReqStatus, nil
}
