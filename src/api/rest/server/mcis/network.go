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

// Package mcis is to handle REST API for mcis
package mcis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/api/rest/server/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	terrariumModel "github.com/cloud-barista/mc-terrarium/pkg/api/rest/model"
	"github.com/go-resty/resty/v2"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestGetSitesInMcis godoc
// @ID GetSitesInMcis
// @Summary Get sites in MCIS
// @Description Get sites in MCIS
// @Tags [VPN] Sites in MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Success 200 {object} model.SitesInfo "OK"
// @Failure 400 {object} common.SimpleMsg "Bad Request"
// @Failure 500 {object} common.SimpleMsg "Internal Server Error"
// @Failure 503 {object} common.SimpleMsg "Service Unavailable"
// @Router /ns/{nsId}/mcis/{mcisId}/site [get]
func RestGetSitesInMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}

		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	SitesInfo, err := ExtractSitesInfoFromMcisInfo(nsId, mcisId)
	if err != nil {
		log.Err(err).Msg("")
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	return c.JSON(http.StatusOK, SitesInfo)
}

func ExtractSitesInfoFromMcisInfo(nsId, mcisId string) (*model.SitesInfo, error) {
	// Get MCIS info
	mcisInfo, err := mcis.GetMcisInfo(nsId, mcisId)
	if err != nil {
		log.Err(err).Msg("")
		return nil, err
	}

	// A map to check if the VPC (site) is already extracted and added or not.
	checkedVpcs := make(map[string]bool)

	// Newly create the SitesInfo structure
	sitesInfo := model.NewSiteInfo(nsId, mcisId)

	sitesInAws := []model.SiteDetail{}
	sitesInAzure := []model.SiteDetail{}
	sitesInGcp := []model.SiteDetail{}

	for _, vm := range mcisInfo.Vm {

		vNetId := vm.VNetId
		if vNetId == "" {
			log.Warn().Msgf("VNet ID is empty for VM ID: %s", vm.Id)
			continue
		}

		if _, exists := checkedVpcs[vNetId]; exists {
			continue
		}
		checkedVpcs[vNetId] = true

		providerName := vm.ConnectionConfig.ProviderName
		if providerName == "" {
			log.Warn().Msgf("Provider name is empty for VM ID: %s", vm.Id)
			continue
		}

		// Create and set a site details
		var site = model.SiteDetail{}
		site.CSP = vm.ConnectionConfig.ProviderName
		site.Region = vm.CspViewVmDetail.Region.Region

		// Lowercase the provider name
		providerName = strings.ToLower(providerName)

		switch providerName {
		case "aws":

			// Get vNet info
			resourceType := "vNet"
			resourceId := vm.VNetId
			result, err := mcir.GetResource(nsId, resourceType, resourceId)
			if err != nil {
				log.Warn().Msgf("Failed to get the VNet info for ID: %s", resourceId)
				continue
			}
			vNetInfo := result.(mcir.TbVNetInfo)

			// Get the last subnet
			subnetCount := len(vNetInfo.SubnetInfoList)
			lastSubnet := vNetInfo.SubnetInfoList[subnetCount-1]
			lastSubnetIdFromCSP := lastSubnet.IdFromCsp

			// Set VNet and the last subnet IDs
			site.VNet = vm.CspViewVmDetail.VpcIID.SystemId
			site.Subnet = lastSubnetIdFromCSP

			sitesInAws = append(sitesInAws, site)

		case "azure":
			// Parse vNet and resource group names
			parts := strings.Split(vm.CspViewVmDetail.VpcIID.SystemId, "/")
			log.Debug().Msgf("parts: %+v", parts)
			parsedResourceGroupName := parts[4]
			parsedVirtualNetworkName := parts[8]

			// Set VNet and resource group names
			site.VNet = parsedVirtualNetworkName
			site.ResourceGroup = parsedResourceGroupName

			// Get vNet info
			resourceType := "vNet"
			resourceId := vm.VNetId
			result, err := mcir.GetResource(nsId, resourceType, resourceId)
			if err != nil {
				log.Warn().Msgf("Failed to get the VNet info for ID: %s", resourceId)
				continue
			}
			vNetInfo := result.(mcir.TbVNetInfo)

			// Get the last subnet CIDR block
			subnetCount := len(vNetInfo.SubnetInfoList)
			lastSubnet := vNetInfo.SubnetInfoList[subnetCount-1]
			lastSubnetCidr := lastSubnet.IPv4_CIDR

			// (Currently unsafe) Calculate the next subnet CIDR block
			nextCidr, err := netutil.NextSubnet(lastSubnetCidr, vNetInfo.CidrBlock)
			if err != nil {
				log.Warn().Msgf("Failed to get the next subnet CIDR")
			}

			// Set the site detail
			site.GatewaySubnetCidr = nextCidr

			sitesInAzure = append(sitesInAzure, site)

		case "gcp":
			// Set vNet ID
			site.VNet = vm.CspViewVmDetail.VpcIID.SystemId

			sitesInGcp = append(sitesInGcp, site)

		default:
			log.Warn().Msgf("Unsupported provider name: %s", providerName)
		}

		sitesInfo.Count++
	}

	sitesInfo.Sites.Aws = sitesInAws
	sitesInfo.Sites.Azure = sitesInAzure
	sitesInfo.Sites.Gcp = sitesInGcp

	return sitesInfo, nil
}

// RestPostSiteToSiteVpn godoc
// @ID PostSiteToSiteVpn
// @Summary Create a site-to-site VPN (Currently, GCP-AWS is supported)
// @Description Create a site-to-site VPN (Currently, GCP-AWS is supported)
// @Tags [VPN] Site-to-site VPN (under development)
// @Accept  json
// @Produce  json-stream
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param vpnReq body model.RestPostVpnRequest true "Sites info for VPN configuration"
// @Success 200 {object} common.SimpleMsg "OK"
// @Failure 400 {object} common.SimpleMsg "Bad Request"
// @Failure 500 {object} common.SimpleMsg "Internal Server Error"
// @Failure 503 {object} common.SimpleMsg "Service Unavailable"
// @Router /stream-response/ns/{nsId}/mcis/{mcisId}/vpn/{vpnId} [post]
func RestPostSiteToSiteVpn(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Bind the request body to RestPostVpnGcpToAwsRequest struct
	vpnReq := new(model.RestPostVpnRequest)
	if err := c.Bind(vpnReq); err != nil {
		err2 := fmt.Errorf("invalid request format, %v", err)
		log.Warn().Err(err).Msg("invalid request format")
		res := common.SimpleMsg{
			Message: err2.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Validate the VPN sites
	ok := isValidCspSet(vpnReq.Site1.CSP, vpnReq.Site2.CSP)
	if !ok {
		err := fmt.Errorf("currently not supported, VPN between %s and %s", vpnReq.Site1.CSP, vpnReq.Site2.CSP)
		log.Warn().Err(err).Msg("")
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Prepare for streaming response
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.Response().WriteHeader(http.StatusOK)
	enc := json.NewEncoder(c.Response())

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("API_USERNAME")
	apiPass := os.Getenv("API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// set endpoint
	epTerrarium := common.TerrariumRestUrl

	// check readyz
	method := "GET"
	url := fmt.Sprintf("%s/readyz", epTerrarium)
	requestBody := common.NoBody
	resReadyz := new(model.Response)

	err := common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resReadyz,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusServiceUnavailable, res)
	}
	log.Debug().Msgf("resReadyz: %+v", resReadyz.Message)

	// Flush a response
	res := common.SimpleMsg{
		Message: resReadyz.Message,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	cspSet := whichCspSet(vpnReq.Site1.CSP, vpnReq.Site2.CSP)

	// Check the CSPs of the sites
	switch cspSet {
	case "aws,gcp", "gcp,aws":

		// issue a terrarium
		method = "POST"
		url = fmt.Sprintf("%s/tr", epTerrarium)
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
		log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

		// Flush a response
		res = common.SimpleMsg{
			Message: "successully created a terrarium (trId: " + resTrInfo.Id + ")",
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

		// init env
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-aws/env", epTerrarium, trId)
		requestBody = common.NoBody
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resInit: %+v", resTerrariumEnv.Message)
		log.Trace().Msgf("resInit: %+v", resTerrariumEnv.Detail)

		// flush a response
		res = common.SimpleMsg{
			Message: resTerrariumEnv.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

		// generate infracode
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-aws/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfGcpAwsVpnRequest)

		if vpnReq.Site1.CSP == "aws" {
			reqInfracode.TfVars.AwsRegion = vpnReq.Site1.Region
			reqInfracode.TfVars.AwsVpcId = vpnReq.Site1.VNet
			reqInfracode.TfVars.AwsSubnetId = vpnReq.Site1.Subnet
			reqInfracode.TfVars.GcpRegion = vpnReq.Site2.Region
			reqInfracode.TfVars.GcpVpcNetworkName = vpnReq.Site2.VNet
		} else {
			reqInfracode.TfVars.AwsRegion = vpnReq.Site2.Region
			reqInfracode.TfVars.AwsVpcId = vpnReq.Site2.VNet
			reqInfracode.TfVars.AwsSubnetId = vpnReq.Site2.Subnet
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

		// Flush a response
		res = common.SimpleMsg{
			Message: resInfracode.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

		// check the infracode by plan
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-aws/plan", epTerrarium, trId)
		requestBody = common.NoBody
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resPlan: %+v", resPlan.Message)
		log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

		// Flush a response
		res = common.SimpleMsg{
			Message: resPlan.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resApply: %+v", resApply.Message)
		log.Trace().Msgf("resApply: %+v", resApply.Detail)

		// Flush a response
		res = common.SimpleMsg{
			Message: resApply.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()
	case "gcp,azure", "azure,gcp":
		// issue a terrarium
		method = "POST"
		url = fmt.Sprintf("%s/tr", epTerrarium)
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
		log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

		// Flush a response
		res = common.SimpleMsg{
			Message: "successully created a terrarium (trId: " + resTrInfo.Id + ")",
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

		// init env
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-azure/env", epTerrarium, trId)
		requestBody = common.NoBody
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resInit: %+v", resTerrariumEnv.Message)
		log.Trace().Msgf("resInit: %+v", resTerrariumEnv.Detail)

		// flush a response
		res = common.SimpleMsg{
			Message: resTerrariumEnv.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

		// generate infracode
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-azure/infracode", epTerrarium, trId)
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

		// Flush a response
		res = common.SimpleMsg{
			Message: resInfracode.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

		// check the infracode by plan
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/vpn/gcp-azure/plan", epTerrarium, trId)
		requestBody = common.NoBody
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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resPlan: %+v", resPlan.Message)
		log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

		// Flush a response
		res = common.SimpleMsg{
			Message: resPlan.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

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
			res := common.SimpleMsg{Message: err.Error()}
			return c.JSON(http.StatusInternalServerError, res)
		}

		log.Debug().Msgf("resApply: %+v", resApply.Message)
		log.Trace().Msgf("resApply: %+v", resApply.Detail)

		// Flush a response
		res = common.SimpleMsg{
			Message: resApply.Message,
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		c.Response().Flush()

	default:
		log.Warn().Msgf("not valid CSP set: %s", cspSet)
	}

	return nil
}

var validCspSet = map[string]bool{
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

func isValidCspSet(csp1, csp2 string) bool {
	return validCspSet[csp1+","+csp2]
}

func whichCspSet(csp1, csp2 string) string {
	return csp1 + "," + csp2
}

// RestDeleteSiteToSiteVpn godoc
// @ID DeleteSiteToSiteVpn
// @Summary Delete a site-to-site VPN (Currently, GCP-AWS is supported)
// @Description Delete a site-to-site VPN (Currently, GCP-AWS is supported)
// @Tags [VPN] Site-to-site VPN (under development)
// @Accept  json
// @Produce  json-stream
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Success 200 {object} common.SimpleMsg "OK"
// @Failure 400 {object} common.SimpleMsg "Bad Request"
// @Failure 500 {object} common.SimpleMsg "Internal Server Error"
// @Failure 503 {object} common.SimpleMsg "Service Unavailable"
// @Router /stream-response/ns/{nsId}/mcis/{mcisId}/vpn/{vpnId} [delete]
func RestDeleteSiteToSiteVpn(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Prepare for streaming response
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.Response().WriteHeader(http.StatusOK)
	enc := json.NewEncoder(c.Response())

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("API_USERNAME")
	apiPass := os.Getenv("API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// set endpoint
	epTerrarium := common.TerrariumRestUrl

	// check readyz
	method := "GET"
	url := fmt.Sprintf("%s/readyz", epTerrarium)
	requestBody := common.NoBody
	resReadyz := new(model.Response)

	err := common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resReadyz,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusServiceUnavailable, res)
	}
	log.Debug().Msgf("resReadyz: %+v", resReadyz.Message)
	log.Trace().Msgf("resReadyz: %+v", resReadyz.Detail)

	// Flush a response
	res := common.SimpleMsg{
		Message: resReadyz.Message,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	// Get the terrarium info
	method = "GET"
	url = fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody = common.NoBody
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
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
	enrichments := resTrInfo.Enrichments

	// Flush a response
	msg := fmt.Sprintf("successully got the terrarium (trId: %s) for the enrichment (%s)", resTrInfo.Id, enrichments)
	res = common.SimpleMsg{
		Message: msg,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

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
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Message)
	log.Trace().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resDeleteEnrichments.Message,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

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
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resDeleteEnv: %+v", resDeleteEnv.Message)
	log.Trace().Msgf("resDeleteEnv: %+v", resDeleteEnv.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resDeleteEnv.Message,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

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
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resDeleteTr: %+v", resDeleteTr.Message)
	log.Trace().Msgf("resDeleteTr: %+v", resDeleteTr.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resDeleteTr.Message,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	return nil
}

// RestPutSiteToSiteVpn godoc
// @ID PutSiteToSiteVpn
// @Summary (To be provided) Update a site-to-site VPN
// @Description (To be provided) Update a site-to-site VPN
// @Tags [VPN] Site-to-site VPN (under development)
// @Accept  json
// @Produce  json-stream
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param vpnReq body model.RestPostVpnGcpToAwsRequest true "Resources info for VPN tunnel configuration between GCP and AWS"
// @Success 200 {object} common.SimpleMsg "OK"
// @Failure 400 {object} common.SimpleMsg "Bad Request"
// @Failure 500 {object} common.SimpleMsg "Internal Server Error"
// @Failure 503 {object} common.SimpleMsg "Service Unavailable"
// @Router /stream-response/ns/{nsId}/mcis/{mcisId}/vpn/{vpnId} [put]
func RestPutSiteToSiteVpn(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := common.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Prepare for streaming response
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.Response().WriteHeader(http.StatusOK)
	enc := json.NewEncoder(c.Response())

	// Flush a response
	res := common.SimpleMsg{
		Message: "note - API to be provided",
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	return nil

	// Initialize resty client with basic auth
	// client := resty.New()
	// apiUser := os.Getenv("API_USERNAME")
	// apiPass := os.Getenv("API_PASSWORD")
	// client.SetBasicAuth(apiUser, apiPass)

	// epTerrarium := "http://localhost:8888/terrarium"
	// trId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// // check readyz
	// method := "GET"
	// url := fmt.Sprintf("%s/readyz", epTerrarium)
	// requestBody := common.NoBody
	// resReadyz := new(model.Response)

	// err := common.ExecuteHttpRequest(
	// 	client,
	// 	method,
	// 	url,
	// 	nil,
	// 	common.SetUseBody(requestBody),
	// 	&requestBody,
	// 	resReadyz,
	// 	common.VeryShortDuration,
	// )

	// if err != nil {
	// 	log.Err(err).Msg("")
	// 	res := common.SimpleMsg{
	// 		Message: err.Error(),
	// 	}
	// 	return c.JSON(http.StatusServiceUnavailable, res)
	// }
	// log.Debug().Msgf("resReadyz: %+v", resReadyz)

	// // Flush a response
	// res := common.SimpleMsg{
	// 	Message: resReadyz.Message,
	// }
	// if err := enc.Encode(res); err != nil {
	// 	return err
	// }
	// c.Response().Flush()

	// return nil
}

// RestGetSiteToSiteVpn godoc
// @ID GetSiteToSiteVpn
// @Summary Get resource info of a site-to-site VPN (Currently, GCP-AWS is supported)
// @Description Get resource info of a site-to-site VPN (Currently, GCP-AWS is supported)
// @Tags [VPN] Site-to-site VPN (under development)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param detail query string false "Resource info by detail (refined, raw)" default(refined)
// @Success 200 {object} model.Response "OK"
// @Failure 400 {object} model.Response "Bad Request"
// @Failure 500 {object} model.Response "Internal Server Error"
// @Failure 503 {object} model.Response "Service Unavailable"
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId} [get]
func RestGetSiteToSiteVpn(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Use this struct like the enum
	var DetailOptions = struct {
		Refined string
		Raw     string
	}{
		Refined: "refined",
		Raw:     "raw",
	}

	// valid detail options
	validDetailOptions := map[string]bool{
		DetailOptions.Refined: true,
		DetailOptions.Raw:     true,
	}

	detail := c.QueryParam("detail")
	detail = strings.ToLower(detail)

	if detail == "" || !validDetailOptions[detail] {
		err := fmt.Errorf("invalid detail (%s), use the default (%s)", detail, DetailOptions.Refined)
		log.Warn().Msg(err.Error())
		detail = DetailOptions.Refined
	}

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("API_USERNAME")
	apiPass := os.Getenv("API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// set endpoint
	epTerrarium := common.TerrariumRestUrl

	// check readyz
	method := "GET"
	url := fmt.Sprintf("%s/readyz", epTerrarium)
	requestBody := common.NoBody
	resReadyz := new(model.Response)

	err := common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resReadyz,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusServiceUnavailable, res)
	}
	log.Debug().Msgf("resReadyz: %+v", resReadyz.Message)

	// Get the terrarium info
	method = "GET"
	url = fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody = common.NoBody
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
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
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
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	switch detail {
	case DetailOptions.Refined:
		log.Debug().Msgf("resResourceInfo: %+v", resResourceInfo.Object)
		res := model.Response{
			Success: resResourceInfo.Success,
			Object:  resResourceInfo.Object,
		}
		return c.JSON(http.StatusOK, res)
	case DetailOptions.Raw:
		log.Debug().Msgf("resResourceInfo: %+v", resResourceInfo.List)
		res := model.Response{
			Success: resResourceInfo.Success,
			List:    resResourceInfo.List,
		}
		return c.JSON(http.StatusOK, res)
	default:
		log.Warn().Msgf("invalid detail option (%s)", detail)
		res := model.Response{
			Success: false,
			Message: fmt.Sprintf("invalid detail option (%s)", detail),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
}

// RestGetRequestStatusOfSiteToSiteVpn godoc
// @ID GetRequestStatusOfSiteToSiteVpn
// @Summary Check the status of a specific request by its ID
// @Description Check the status of a specific request by its ID
// @Tags [VPN] Site-to-site VPN (under development)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param requestId path string true "Request ID"
// @Success 200 {object} model.Response "OK"
// @Failure 400 {object} model.Response "Bad Request"
// @Failure 500 {object} model.Response "Internal Server Error"
// @Failure 503 {object} model.Response "Service Unavailable"
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId}/request/{requestId} [get]
func RestGetRequestStatusOfSiteToSiteVpn(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	reqId := c.Param("requestId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, request ID (requestId: %s) is required", reqId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("API_USERNAME")
	apiPass := os.Getenv("API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// set endpoint
	epTerrarium := common.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err := common.ExecuteHttpRequest(
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
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
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
		res := model.Response{
			Success: false,
			Message: err.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resReqStatus: %+v", resReqStatus.Detail)

	return c.JSON(http.StatusOK, resReqStatus)
}
