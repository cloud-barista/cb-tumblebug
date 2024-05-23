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
	"github.com/go-resty/resty/v2"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestGetSitesInMcis godoc
// @Summary Get sites in MCIS
// @Description Get sites in MCIS
// @Tags [VPN] Sites in MCIS (under development)
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

	// Newly create the SitesInfo structure
	sitesInfo := model.NewSiteInfo(nsId, mcisId)

	for _, vm := range mcisInfo.Vm {
		providerName := vm.ConnectionConfig.ProviderName
		if providerName == "" {
			log.Warn().Msgf("Provider name is empty for VM ID: %s", vm.Id)
			continue
		}

		vNetId := vm.VNetId
		if vNetId == "" {
			log.Warn().Msgf("VNet ID is empty for VM ID: %s", vm.Id)
			continue
		}

		// Add or update the site detail in the map based on the provider
		providerName = strings.ToLower(providerName)

		// Use vNetId as the site ID
		if _, exists := sitesInfo.Sites[providerName][vm.VNetId]; !exists {

			var site = model.SiteDetail{}
			site.CSP = vm.ConnectionConfig.ProviderName
			site.Region = vm.CspViewVmDetail.Region.Region

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

			case "gcp":
				// Set vNet ID
				site.VNet = vm.CspViewVmDetail.VpcIID.SystemId

			default:
				log.Warn().Msgf("Unsupported provider name: %s", providerName)
			}

			if site != (model.SiteDetail{}) {
				sitesInfo.Sites[providerName][vm.VNetId] = site
				sitesInfo.Count++
			}
		}
	}

	return sitesInfo, nil
}

// RestPostVpnGcpToAws godoc
// @Summary Create VPN tunnels between GCP and AWS (Note - Streaming JSON response)
// @Description Create VPN tunnels between GCP and AWS (Note - Streaming JSON response)
// @Tags [VPN] GCP-AWS VPN tunnel (under development)
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
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId}/gcp-aws [post]
func RestPostVpnGcpToAws(c echo.Context) error {

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
	vpnReq := new(model.RestPostVpnGcpToAwsRequest)
	if err := c.Bind(vpnReq); err != nil {
		err2 := fmt.Errorf("invalid request format, %v", err)
		log.Warn().Err(err).Msg("invalid request format")
		res := common.SimpleMsg{
			Message: err2.Error(),
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

	rgId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

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
	log.Debug().Msgf("resReadyz: %+v", resReadyz.Text)

	// Flush a response
	res := common.SimpleMsg{
		Message: resReadyz.Text,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	// init terrarium
	method = "POST"
	url = fmt.Sprintf("%s/rg/%s/vpn/gcp-aws/terrarium", epTerrarium, rgId)
	requestBody = common.NoBody
	resInitTerrarium := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resInitTerrarium,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resInit: %+v", resInitTerrarium.Text)
	log.Trace().Msgf("resInit: %+v", resInitTerrarium.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resInitTerrarium.Text,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	// infracode
	method = "POST"
	url = fmt.Sprintf("%s/rg/%s/vpn/gcp-aws/infracode", epTerrarium, rgId)
	reqInfracode := *vpnReq
	resInfracode := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(reqInfracode),
		&reqInfracode,
		resInfracode,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resInfracode: %+v", resInfracode.Text)
	log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resInfracode.Text,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	// plan
	method = "POST"
	url = fmt.Sprintf("%s/rg/%s/vpn/gcp-aws/plan", epTerrarium, rgId)
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

	log.Debug().Msgf("resPlan: %+v", resPlan.Text)
	log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resPlan.Text,
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
	url = fmt.Sprintf("%s/rg/%s/vpn/gcp-aws", epTerrarium, rgId)
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

	log.Debug().Msgf("resApply: %+v", resApply.Text)
	log.Trace().Msgf("resApply: %+v", resApply.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resApply.Text,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	return nil
}

// RestDeleteVpnGcpToAws godoc
// @Summary Delete VPN tunnels between GCP and AWS (Note - Streaming JSON response)
// @Description Delete VPN tunnels between GCP and AWS (Note - Streaming JSON response)
// @Tags [VPN] GCP-AWS VPN tunnel (under development)
// @Accept  json
// @Produce  json-stream
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Success 200 {object} common.SimpleMsg "OK"
// @Failure 400 {object} common.SimpleMsg "Bad Request"
// @Failure 500 {object} common.SimpleMsg "Internal Server Error"
// @Failure 503 {object} common.SimpleMsg "Service Unavailable"
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId}/gcp-aws [delete]
func RestDeleteVpnGcpToAws(c echo.Context) error {

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

	rgId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

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
	log.Debug().Msgf("resReadyz: %+v", resReadyz.Text)
	log.Trace().Msgf("resReadyz: %+v", resReadyz.Detail)

	// Flush a response
	res := common.SimpleMsg{
		Message: resReadyz.Text,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	// delete
	method = "DELETE"
	url = fmt.Sprintf("%s/rg/%s/vpn/gcp-aws", epTerrarium, rgId)
	requestBody = common.NoBody
	resDelete := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDelete,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := common.SimpleMsg{Message: err.Error()}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resDelete: %+v", resDelete.Text)
	log.Trace().Msgf("resDelete: %+v", resDelete.Detail)

	// Flush a response
	res = common.SimpleMsg{
		Message: resDelete.Text,
	}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	return nil
}

// RestPutVpnGcpToAws godoc
// @Summary (To be provided) Update VPN tunnels between GCP and AWS
// @Description Update VPN tunnels between GCP and AWS
// @Tags [VPN] GCP-AWS VPN tunnel (under development)
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
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId}/gcp-aws [put]
func RestPutVpnGcpToAws(c echo.Context) error {

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
	// rgId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// // check readyz
	// method := "GET"
	// url := fmt.Sprintf("%s/readyz", epTerrarium)
	// requestBody := common.NoBody
	// resReadyz := new(model.ResponseText)

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
	// 	Message: resReadyz.Text,
	// }
	// if err := enc.Encode(res); err != nil {
	// 	return err
	// }
	// c.Response().Flush()

	// return nil
}

// RestGetVpnGcpToAws godoc
// @Summary Get resource info of VPN tunnels between GCP and AWS
// @Description Update VPN tunnels between GCP and AWS
// @Tags [VPN] GCP-AWS VPN tunnel (under development)
// @Accept  json
// @Produce  json-stream
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param detail query string false "Resource info by detail (refined, raw)" default(refined)
// @Success 200 {object} model.Response "OK"
// @Failure 400 {object} model.Response "Bad Request"
// @Failure 500 {object} model.Response "Internal Server Error"
// @Failure 503 {object} model.Response "Service Unavailable"
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId}/gcp-aws [get]
func RestGetVpnGcpToAws(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
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

	rgId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

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
			Text:    err.Error(),
		}
		return c.JSON(http.StatusServiceUnavailable, res)
	}
	log.Debug().Msgf("resReadyz: %+v", resReadyz.Text)

	// Get resource info
	method = "GET"
	url = fmt.Sprintf("%s/rg/%s/vpn/gcp-aws?detail=%s", epTerrarium, rgId, detail)
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
			Text:    err.Error(),
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
			Text:    fmt.Sprintf("invalid detail option (%s)", detail),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
}

// RestGetRequestStatusOfGcpAwsVpn godoc
// @Summary Check the status of a specific request by its ID
// @Description Check the status of a specific request by its ID
// @Tags [VPN] GCP-AWS VPN tunnel (under development)
// @Accept  json
// @Produce  json-stream
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param vpnId path string true "VPN ID" default(vpn01)
// @Param requestId path string true "Request ID"
// @Success 200 {object} model.Response "OK"
// @Failure 400 {object} model.Response "Bad Request"
// @Failure 500 {object} model.Response "Internal Server Error"
// @Failure 503 {object} model.Response "Service Unavailable"
// @Router /ns/{nsId}/mcis/{mcisId}/vpn/{vpnId}/gcp-aws/request/{requestId} [get]
func RestGetRequestStatusOfGcpAwsVpn(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("invalid request, namespace ID (nsId: %s) is required", nsId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	mcisId := c.Param("mcisId")
	if mcisId == "" {
		err := fmt.Errorf("invalid request, MCIS ID (mcisId: %s) is required", mcisId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	vpnId := c.Param("vpnId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, VPN ID (vpnId: %s) is required", vpnId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	reqId := c.Param("requestId")
	if vpnId == "" {
		err := fmt.Errorf("invalid request, request ID (requestId: %s) is required", reqId)
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("API_USERNAME")
	apiPass := os.Getenv("API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	rgId := fmt.Sprintf("%s-%s-%s", nsId, mcisId, vpnId)

	// set endpoint
	epTerrarium := common.TerrariumRestUrl

	// Get resource info
	method := "GET"
	url := fmt.Sprintf("%s/rg/%s/vpn/gcp-aws/request/%s", epTerrarium, rgId, reqId)
	requestBody := common.NoBody
	resReqStatus := new(model.Response)

	err := common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resReqStatus,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	log.Debug().Msgf("resReqStatus: %+v", resReqStatus.Detail)

	return c.JSON(http.StatusOK, resReqStatus)
}
