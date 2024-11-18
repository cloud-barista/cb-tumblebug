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
	"os"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	terrariumModel "github.com/cloud-barista/mc-terrarium/pkg/api/rest/model"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

const (
	minWaitDuration = 10 * time.Second
	maxWaitDuration = 120 * time.Second
)

var validCspSetForVPN = map[string]bool{
	"aws,gcp":   true,
	"gcp,aws":   true,
	"gcp,azure": true,
	"azure,gcp": true,
	// "azure,alibaba": true,
	// "alibaba,azure": true,
	// "nhn,ncp":       true,
	// "ncp,nhn":       true,

	// Add more CSP sets here
}

func IsValidCspSetForVPN(csp1, csp2 string) (bool, error) {
	if !validCspSetForVPN[csp1+","+csp2] {
		return false, fmt.Errorf("currently not supported, VPN between %s and %s", csp1, csp2)
	}
	return true, nil
}

func whichCspSetForVPN(csp1, csp2 string) string {
	return csp1 + "," + csp2
}

// GetSiteToSiteVPN returns a site-to-site VPN
func GetAllSiteToSiteVPN(nsId string, mciId string) (model.VpnInfoList, error) {

	var emptyRet model.VpnInfoList
	var vpnInfoList model.VpnInfoList
	vpnInfoList.VpnInfoList = []model.VPNInfo{}
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
		tempVpnInfo := model.VPNInfo{}
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
		tempVpnInfo := model.VPNInfo{}
		err = json.Unmarshal([]byte(vpnKv.Value), &tempVpnInfo)
		if err != nil {
			log.Warn().Err(err).Msg("")
		}

		vpnIdList.VpnIdList = append(vpnIdList.VpnIdList, tempVpnInfo.Id)
	}

	return vpnIdList, nil
}

// CreateSiteToSiteVPN creates a site-to-site VPN via Terrarium
func CreateSiteToSiteVPN(nsId string, mciId string, vpnReq *model.RestPostVpnRequest, retry string) (model.VPNInfo, error) {

	// VPN objects
	var emptyRet model.VPNInfo
	var vpnInfo model.VPNInfo
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
	ok, err := IsValidCspSetForVPN(vpnReq.Site1.CSP, vpnReq.Site2.CSP)
	if !ok {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Ensure vpnReq.Site1 and vpnReq.Site2 are in alphabetical order by CSP
	if vpnReq.Site1.CSP > vpnReq.Site2.CSP {
		vpnReq.Site1, vpnReq.Site2 = vpnReq.Site2, vpnReq.Site1
	}

	// Set the resource type
	resourceType := model.StrVPN

	// Set the vpn object in advance
	uid := common.GenUid()
	vpnInfo.ResourceType = resourceType
	vpnInfo.Name = vpnReq.Name
	vpnInfo.Id = vpnReq.Name
	vpnInfo.Uid = uid
	vpnInfo.Description = "VPN between " + vpnReq.Site1.CSP + " and " + vpnReq.Site2.CSP

	site1VPNGatewayInfo := model.VPNGatewayInfo{}
	site1VPNGatewayInfo.ConnectionName = vpnReq.Site1.ConnectionName
	site1VPNGatewayInfo.ConnectionConfig, err = common.GetConnConfig(site1VPNGatewayInfo.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	site2VPNGatewayInfo := model.VPNGatewayInfo{}
	site2VPNGatewayInfo.ConnectionName = vpnReq.Site2.ConnectionName
	site2VPNGatewayInfo.ConnectionConfig, err = common.GetConnConfig(site2VPNGatewayInfo.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	vpnInfo.VPNGatewayInfo = []model.VPNGatewayInfo{site1VPNGatewayInfo, site2VPNGatewayInfo}

	// Set a vpnKey for the site-to-site VPN object
	vpnKey := common.GenResourceKey(nsId, resourceType, vpnInfo.Id)
	// Check if the vpn already exists or not
	exists, err := CheckResource(nsId, resourceType, vpnInfo.Id)
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
		vpnKv, err := kvstore.GetKv(vpnKey)
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
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	// Set Terrarium endpoint
	epTerrarium := model.TerrariumRestUrl

	// Set a terrarium ID
	trId := vpnInfo.Uid

	cspSet := whichCspSetForVPN(vpnReq.Site1.CSP, vpnReq.Site2.CSP)

	// Check the CSPs of the sites
	switch cspSet {
	case "aws,gcp":

		if !retried {
			// Issue a terrarium
			method := "POST"
			url := fmt.Sprintf("%s/tr", epTerrarium)
			reqTr := new(terrariumModel.TerrariumInfo)
			reqTr.Id = trId
			reqTr.Description = "VPN between GCP and AWS"

			resTrInfo := new(terrariumModel.TerrariumInfo)

			err = common.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				common.SetUseBody(*reqTr),
				reqTr,
				resTrInfo,
				common.VeryShortDuration,
			)

			if err != nil {
				log.Err(err).Msg("")
				return emptyRet, err
			}

			log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
			log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

			// init env
			method = "POST"
			url = fmt.Sprintf("%s/tr/%s/vpn/gcp-aws/env", epTerrarium, trId)
			requestBody := common.NoBody
			resTerrariumEnv := new(model.Response)

			err = common.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				common.SetUseBody(requestBody),
				&requestBody,
				resTerrariumEnv,
				common.VeryShortDuration,
			)

			if err != nil {
				log.Err(err).Msg("")
				return emptyRet, err
			}

			log.Debug().Msgf("resInit: %+v", resTerrariumEnv.Message)
			log.Trace().Msgf("resInit: %+v", resTerrariumEnv.Detail)
		}

		// generate infracode
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/vpn/gcp-aws/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfGcpAwsVpnRequest)

		if vpnReq.Site1.CSP == "aws" {
			// Site1 is AWS
			reqInfracode.TfVars.AwsRegion = vpnReq.Site1.Region
			reqInfracode.TfVars.AwsVpcId = vpnReq.Site1.VNet
			reqInfracode.TfVars.AwsSubnetId = vpnReq.Site1.Subnet
			// Site2 is GCP
			reqInfracode.TfVars.GcpRegion = vpnReq.Site2.Region
			reqInfracode.TfVars.GcpVpcNetworkName = vpnReq.Site2.VNet
		} else {
			// Site2 is AWS
			reqInfracode.TfVars.AwsRegion = vpnReq.Site2.Region
			reqInfracode.TfVars.AwsVpcId = vpnReq.Site2.VNet
			reqInfracode.TfVars.AwsSubnetId = vpnReq.Site2.Subnet
			// Site1 is GCP
			reqInfracode.TfVars.GcpRegion = vpnReq.Site1.Region
			reqInfracode.TfVars.GcpVpcNetworkName = vpnReq.Site1.VNet
		}

		resInfracode := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

		// check the infracode by plan
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-aws/plan", epTerrarium, trId)
		requestBody := common.NoBody
		resPlan := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(requestBody),
			&requestBody,
			resPlan,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resPlan: %+v", resPlan.Message)
		log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

		// apply
		// wait until the task is completed
		// or response immediately with requestId as it is a time-consuming task
		// and provide seperate api to check the status
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-aws", epTerrarium, trId)
		requestBody = common.NoBody
		resApply := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(requestBody),
			&requestBody,
			resApply,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resApply: %+v", resApply.Message)
		log.Trace().Msgf("resApply: %+v", resApply.Detail)

		/*
		 * [Via Terrarium] Retrieve the VPN info recursively until the VPN is created
		 */

		// Recursively call the function to get the VPN info
		// An expected completion duration is 15 minutes
		expectedCompletionDuration := 15 * time.Minute

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()

		ret, err := retrieveEnrichmentsInfoInTerrarium(ctx, trId, "vpn/gcp-aws", expectedCompletionDuration)
		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		// Set the VPN info
		var trVpnInfo terrariumModel.OutputGcpAwsVpnInfo
		jsonData, err := json.Marshal(ret.Object)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		err = json.Unmarshal(jsonData, &trVpnInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		vpnInfo.VPNGatewayInfo[0].CspResourceId = trVpnInfo.AWS.VpnGateway.ID
		vpnInfo.VPNGatewayInfo[0].CspResourceName = trVpnInfo.AWS.VpnGateway.Name
		vpnInfo.VPNGatewayInfo[0].Details = trVpnInfo.AWS
		vpnInfo.VPNGatewayInfo[1].CspResourceId = trVpnInfo.GCP.HaVpnGateway.ID
		vpnInfo.VPNGatewayInfo[1].CspResourceName = trVpnInfo.GCP.HaVpnGateway.Name
		vpnInfo.VPNGatewayInfo[1].Details = trVpnInfo.GCP

	case "azure,gcp":

		if !retried {
			// issue a terrarium
			method := "POST"
			url := fmt.Sprintf("%s/tr", epTerrarium)
			reqTr := new(terrariumModel.TerrariumInfo)
			reqTr.Id = trId
			reqTr.Description = "VPN between GCP and Azure"

			resTrInfo := new(terrariumModel.TerrariumInfo)

			err = common.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				common.SetUseBody(*reqTr),
				reqTr,
				resTrInfo,
				common.VeryShortDuration,
			)

			if err != nil {
				log.Err(err).Msg("")
				return emptyRet, err
			}

			log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
			log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

			// init env
			method = "POST"
			url = fmt.Sprintf("%s/tr/%s/vpn/gcp-azure/env", epTerrarium, trId)
			requestBody := common.NoBody
			resTerrariumEnv := new(model.Response)

			err = common.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				common.SetUseBody(requestBody),
				&requestBody,
				resTerrariumEnv,
				common.VeryShortDuration,
			)

			if err != nil {
				log.Err(err).Msg("")
				return emptyRet, err
			}

			log.Debug().Msgf("resInit: %+v", resTerrariumEnv.Message)
			log.Trace().Msgf("resInit: %+v", resTerrariumEnv.Detail)
		}

		// generate infracode
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/vpn/gcp-azure/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfGcpAzureVpnRequest)

		if vpnReq.Site1.CSP == "azure" {
			// Site1 is Azure
			reqInfracode.TfVars.AzureRegion = vpnReq.Site1.Region
			reqInfracode.TfVars.AzureVirtualNetworkName = vpnReq.Site1.VNet
			reqInfracode.TfVars.AzureResourceGroupName = vpnReq.Site1.ResourceGroup
			reqInfracode.TfVars.AzureGatewaySubnetCidrBlock = vpnReq.Site1.GatewaySubnetCidr
			// Site2 is GCP
			reqInfracode.TfVars.GcpRegion = vpnReq.Site2.Region
			reqInfracode.TfVars.GcpVpcNetworkName = vpnReq.Site2.VNet
		} else {
			// Site1 is GCP
			reqInfracode.TfVars.GcpRegion = vpnReq.Site1.Region
			reqInfracode.TfVars.GcpVpcNetworkName = vpnReq.Site1.VNet
			// site2 is Azure
			reqInfracode.TfVars.AzureRegion = vpnReq.Site2.Region
			reqInfracode.TfVars.AzureVirtualNetworkName = vpnReq.Site2.VNet
			reqInfracode.TfVars.AzureResourceGroupName = vpnReq.Site2.ResourceGroup
			reqInfracode.TfVars.AzureGatewaySubnetCidrBlock = vpnReq.Site2.GatewaySubnetCidr
		}

		resInfracode := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

		// check the infracode by plan
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-azure/plan", epTerrarium, trId)
		requestBody := common.NoBody
		resPlan := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(requestBody),
			&requestBody,
			resPlan,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		log.Debug().Msgf("resPlan: %+v", resPlan.Message)
		log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

		// apply
		// wait until the task is completed
		// or response immediately with requestId as it is a time-consuming task
		// and provide seperate api to check the status
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-azure", epTerrarium, trId)
		requestBody = common.NoBody
		resApply := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(requestBody),
			&requestBody,
			resApply,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		log.Debug().Msgf("resApply: %+v", resApply.Message)
		log.Trace().Msgf("resApply: %+v", resApply.Detail)

		/*
		 * [Via Terrarium] Retrieve the VPN info recursively until the VPN is created
		 */

		// Recursively call the function to get the VPN info
		// An expected completion duration is 15 minutes
		expectedCompletionDuration := 30 * time.Minute

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()

		ret, err := retrieveEnrichmentsInfoInTerrarium(ctx, trId, "vpn/gcp-azure", expectedCompletionDuration)
		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		// Set the VPN info
		var trVpnInfo terrariumModel.OutputGcpAzureVpnInfo
		jsonData, err := json.Marshal(ret.Object)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		err = json.Unmarshal(jsonData, &trVpnInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		vpnInfo.VPNGatewayInfo[0].CspResourceId = trVpnInfo.Azure.VirtualNetworkGateway.ID
		vpnInfo.VPNGatewayInfo[0].CspResourceName = trVpnInfo.Azure.VirtualNetworkGateway.Name
		vpnInfo.VPNGatewayInfo[0].Details = trVpnInfo.Azure
		vpnInfo.VPNGatewayInfo[1].CspResourceId = trVpnInfo.GCP.HaVpnGateway.ID
		vpnInfo.VPNGatewayInfo[1].CspResourceName = trVpnInfo.GCP.HaVpnGateway.Name
		vpnInfo.VPNGatewayInfo[1].Details = trVpnInfo.GCP

	default:
		log.Warn().Msgf("not valid CSP set: %s", cspSet)
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
	vpnKv, err := kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vpnKv == (kvstore.KeyValue{}) {
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
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	// Set Terrarium endpoint
	epTerrarium := model.TerrariumRestUrl

	requestBody := common.NoBody
	resRetrieving := new(model.Response)

	// Set the start time and the expected end time
	startTime := time.Now()
	expectedEndTime := startTime.Add(expectedCompletionDuration)

	// Set wait duration
	currentWaitDuration := maxWaitDuration

	for {
		err := common.ExecuteHttpRequest(
			client,
			"GET",
			fmt.Sprintf("%s/tr/%s/%s?detail=refined", epTerrarium, trId, enrichments),
			nil,
			common.SetUseBody(requestBody),
			&requestBody,
			resRetrieving,
			common.VeryShortDuration,
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

// GetSiteToSiteVPN returns a site-to-site VPN via Terrarium
func GetSiteToSiteVPN(nsId string, mciId string, vpnId string, detail string) (model.VPNInfo, error) {

	var emptyRet model.VPNInfo
	var vpnInfo model.VPNInfo
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
	vpnKv, err := kvstore.GetKv(vpnKey)
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
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := vpnInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		common.VeryShortDuration,
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
	requestBody = common.NoBody
	resResourceInfo := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resResourceInfo,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	switch enrichments {
	case "vpn/gcp-aws":
		var trVpnInfo terrariumModel.OutputGcpAwsVpnInfo
		jsonData, err := json.Marshal(resResourceInfo.Object)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		err = json.Unmarshal(jsonData, &trVpnInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		vpnInfo.VPNGatewayInfo[0].CspResourceId = trVpnInfo.AWS.VpnGateway.ID
		vpnInfo.VPNGatewayInfo[0].CspResourceName = trVpnInfo.AWS.VpnGateway.Name
		vpnInfo.VPNGatewayInfo[0].Details = trVpnInfo.AWS
		vpnInfo.VPNGatewayInfo[1].CspResourceId = trVpnInfo.GCP.HaVpnGateway.ID
		vpnInfo.VPNGatewayInfo[1].CspResourceName = trVpnInfo.GCP.HaVpnGateway.Name
		vpnInfo.VPNGatewayInfo[1].Details = trVpnInfo.GCP
	case "vpn/gcp-azure":
		var trVpnInfo terrariumModel.OutputGcpAzureVpnInfo
		jsonData, err := json.Marshal(resResourceInfo.Object)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		err = json.Unmarshal(jsonData, &trVpnInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		vpnInfo.VPNGatewayInfo[0].CspResourceId = trVpnInfo.Azure.VirtualNetworkGateway.ID
		vpnInfo.VPNGatewayInfo[0].CspResourceName = trVpnInfo.Azure.VirtualNetworkGateway.Name
		vpnInfo.VPNGatewayInfo[0].Details = trVpnInfo.Azure
		vpnInfo.VPNGatewayInfo[1].CspResourceId = trVpnInfo.GCP.HaVpnGateway.ID
		vpnInfo.VPNGatewayInfo[1].CspResourceName = trVpnInfo.GCP.HaVpnGateway.Name
		vpnInfo.VPNGatewayInfo[1].Details = trVpnInfo.GCP
	default:
		log.Warn().Msgf("not valid enrichments: %s", enrichments)
		return emptyRet, fmt.Errorf("not valid enrichments: %s", enrichments)
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
	vpnKv, err = kvstore.GetKv(vpnKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vpnKv == (kvstore.KeyValue{}) {
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
	var vpnInfo model.VPNInfo
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
	vpnKv, err := kvstore.GetKv(vpnKey)
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
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := vpnInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
	enrichments := resTrInfo.Enrichments

	// delete enrichments
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s/%s", epTerrarium, trId, enrichments)
	requestBody = common.NoBody
	resDeleteEnrichments := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDeleteEnrichments,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Message)
	log.Trace().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Detail)

	// delete env
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s/%s/env", epTerrarium, trId, enrichments)
	requestBody = common.NoBody
	resDeleteEnv := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDeleteEnv,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteEnv: %+v", resDeleteEnv.Message)
	log.Trace().Msgf("resDeleteEnv: %+v", resDeleteEnv.Detail)

	// delete terrarium
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody = common.NoBody
	resDeleteTr := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDeleteTr,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteTr: %+v", resDeleteTr.Message)
	log.Trace().Msgf("resDeleteTr: %+v", resDeleteTr.Detail)

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

	res := model.SimpleMsg{
		Message: resDeleteTr.Message,
	}

	return res, nil
}

// GetRequestStatusOfSiteToSiteVpn checks the status of a specific request
func GetRequestStatusOfSiteToSiteVpn(nsId string, mciId string, vpnId string, reqId string) (model.Response, error) {

	var emptyRet model.Response
	var vpnInfo model.VPNInfo
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
	vpnKv, err := kvstore.GetKv(vpnKey)
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
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := vpnInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		common.VeryShortDuration,
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
	reqReqStatus := common.NoBody
	resReqStatus := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(reqReqStatus),
		&reqReqStatus,
		resReqStatus,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("resReqStatus: %+v", resReqStatus.Detail)

	return *resReqStatus, nil
}
