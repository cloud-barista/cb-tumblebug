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

// Package mci is to handle REST API for mci
package resource

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// // RestGetSitesInMci godoc
// // @ID GetSitesInMci
// // @Summary Get sites in MCI
// // @Description Get sites in MCI
// // @Tags [Infra Resource] Object Storage Management (under development)
// // @Accept  json
// // @Produce  json
// // @Param nsId path string true "Namespace ID" default(default)
// // @Param mciId path string true "MCI ID" default(mci01)
// // @Success 200 {object} model.SitesInfo "OK"
// // @Failure 400 {object} model.SimpleMsg "Bad Request"
// // @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// // @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// // @Router /ns/{nsId}/mci/{mciId}/site [get]
// func RestGetSitesInMci(c echo.Context) error {

// 	nsId := c.Param("nsId")
// 	err := common.CheckString(nsId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}

// 	mciId := c.Param("mciId")
// 	err = common.CheckString(mciId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid mciId (%s)", mciId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}

// 	SitesInfo, err := ExtractSitesInfoFromMciInfo(nsId, mciId)
// 	if err != nil {
// 		log.Err(err).Msg("")
// 		res := model.SimpleMsg{
// 			Message: err.Error(),
// 		}
// 		return c.JSON(http.StatusInternalServerError, res)
// 	}

// 	return c.JSON(http.StatusOK, SitesInfo)
// }

// func ExtractSitesInfoFromMciInfo(nsId, mciId string) (*model.SitesInfo, error) {
// 	// Get MCI info
// 	mciInfo, err := infra.GetMciInfo(nsId, mciId)
// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return nil, err
// 	}

// 	// A map to check if the VPC (site) is already extracted and added or not.
// 	checkedVpcs := make(map[string]bool)

// 	// Newly create the SitesInfo structure
// 	sitesInfo := model.NewSiteInfo(nsId, mciId)

// 	sitesInAws := []model.SiteDetail{}
// 	sitesInAzure := []model.SiteDetail{}
// 	sitesInGcp := []model.SiteDetail{}

// 	for _, vm := range mciInfo.Vm {

// 		vNetId := vm.VNetId
// 		if vNetId == "" {
// 			log.Warn().Msgf("VNet ID is empty for VM ID: %s", vm.Id)
// 			continue
// 		}

// 		if _, exists := checkedVpcs[vNetId]; exists {
// 			continue
// 		}
// 		checkedVpcs[vNetId] = true

// 		providerName := vm.ConnectionConfig.ProviderName
// 		if providerName == "" {
// 			log.Warn().Msgf("Provider name is empty for VM ID: %s", vm.Id)
// 			continue
// 		}

// 		// Create and set a site details
// 		var site = model.SiteDetail{}
// 		site.CSP = vm.ConnectionConfig.ProviderName
// 		site.Region = vm.Region.Region

// 		// Lowercase the provider name
// 		providerName = strings.ToLower(providerName)

// 		switch providerName {
// 		case "aws":

// 			// Get vNet info
// 			resourceType := "vNet"
// 			resourceId := vm.VNetId
// 			result, err := resource.GetResource(nsId, resourceType, resourceId)
// 			if err != nil {
// 				log.Warn().Msgf("Failed to get the VNet info for ID: %s", resourceId)
// 				continue
// 			}
// 			vNetInfo := result.(model.TbVNetInfo)

// 			// Get the last subnet
// 			subnetCount := len(vNetInfo.SubnetInfoList)
// 			if subnetCount == 0 {
// 				log.Warn().Msgf("No subnets found for VNet ID: %s", vNetId)
// 				continue
// 			}
// 			lastSubnet := vNetInfo.SubnetInfoList[subnetCount-1]

// 			// Set VNet and the last subnet IDs
// 			site.VNet = vm.CspVNetId
// 			site.Subnet = lastSubnet.CspResourceId

// 			// Set connection name
// 			site.ConnectionName = vm.ConnectionName

// 			sitesInAws = append(sitesInAws, site)

// 		case "azure":
// 			// Parse vNet and resource group names
// 			parts := strings.Split(vm.CspVNetId, "/")
// 			log.Debug().Msgf("parts: %+v", parts)
// 			if len(parts) < 9 {
// 				log.Warn().Msgf("Invalid VNet ID format for Azure VM ID: %s", vm.Id)
// 				continue
// 			}
// 			parsedResourceGroupName := parts[4]
// 			parsedVirtualNetworkName := parts[8]

// 			// Set VNet and resource group names
// 			site.VNet = parsedVirtualNetworkName
// 			site.ResourceGroup = parsedResourceGroupName

// 			// Get vNet info
// 			resourceType := "vNet"
// 			resourceId := vm.VNetId
// 			result, err := resource.GetResource(nsId, resourceType, resourceId)
// 			if err != nil {
// 				log.Warn().Msgf("Failed to get the VNet info for ID: %s", resourceId)
// 				continue
// 			}
// 			vNetInfo := result.(model.TbVNetInfo)

// 			// Get the last subnet CIDR block
// 			subnetCount := len(vNetInfo.SubnetInfoList)
// 			if subnetCount == 0 {
// 				log.Warn().Msgf("No subnets found for VNet ID: %s", vNetId)
// 				continue
// 			}
// 			lastSubnet := vNetInfo.SubnetInfoList[subnetCount-1]
// 			lastSubnetCidr := lastSubnet.IPv4_CIDR

// 			// (Currently unsafe) Calculate the next subnet CIDR block
// 			nextCidr, err := netutil.NextSubnet(lastSubnetCidr, vNetInfo.CidrBlock)
// 			if err != nil {
// 				log.Warn().Msgf("Failed to get the next subnet CIDR")
// 			}

// 			// Set the site detail
// 			site.GatewaySubnetCidr = nextCidr

// 			// Set connection name
// 			site.ConnectionName = vm.ConnectionName

// 			sitesInAzure = append(sitesInAzure, site)

// 		case "gcp":
// 			// Set vNet ID
// 			site.VNet = vm.CspVNetId

// 			// Set connection name
// 			site.ConnectionName = vm.ConnectionName

// 			sitesInGcp = append(sitesInGcp, site)

// 		default:
// 			log.Warn().Msgf("Unsupported provider name: %s", providerName)
// 		}

// 		sitesInfo.Count++
// 	}

// 	sitesInfo.Sites.Aws = sitesInAws
// 	sitesInfo.Sites.Azure = sitesInAzure
// 	sitesInfo.Sites.Gcp = sitesInGcp

// 	return sitesInfo, nil
// }

// RestGetAllObjectStorage godoc
// @ID GetAllObjectStorage
// @Summary Get all Object Storages (TBD)
// @Description Get all Object Storages (TBD)
// @Tags [Infra Resource] Object Storage Management (under development)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(InfoList, IdList) default(IdList)
// @Success 200 {object} model.VpnInfoList "OK" /////////////
// @Success 200 {object} model.VpnIdList "OK" /////////////
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// @Router /ns/{nsId}/resources/objectStorage [get]
func RestGetAllObjectStorage(c echo.Context) error {

	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// mciId := c.Param("mciId")
	// err = common.CheckString(mciId)
	// if err != nil {
	// 	errMsg := fmt.Errorf("invalid mciId (%s)", mciId)
	// 	log.Warn().Err(err).Msgf(errMsg.Error())
	// 	return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	// }

	option := c.QueryParam("option")
	if option != "InfoList" && option != "IdList" && option != "" {
		errMsg := fmt.Errorf("invalid option (%s)", option)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// switch option {
	// case "InfoList":
	// 	vpnInfoList, err := resource.GetAllSiteToSiteVPN(nsId, mciId)
	// 	if err != nil {
	// 		log.Err(err).Msg("")
	// 		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	// 	}
	// 	return c.JSON(http.StatusOK, vpnInfoList)
	// case "IdList":
	// 	vpnIdList, err := resource.GetAllIDsOfSiteToSiteVPN(nsId, mciId)
	// 	if err != nil {
	// 		log.Err(err).Msg("")
	// 		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	// 	}
	// 	return c.JSON(http.StatusOK, vpnIdList)
	// default:
	// 	errMsg := fmt.Errorf("invalid option (%s)", option)
	// 	log.Warn().Err(err).Msgf(errMsg.Error())
	// 	return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	// }

	return c.JSON(http.StatusOK, model.VpnInfoList{})
}

// RestPostObjectStorage godoc
// @ID PostObjectStorage
// @Summary Create a Object Storages
// @Description Create a Object Storages
// @Description
// @Description Supported CSPs: AWS, Azure
// @Description - Note - `connectionName` example: aws-ap-northeast-2, azure-koreacentral
// @Description
// @Description - Note - Please check the `requiredCSPResource` property which includes CSP specific values.
// @Description
// @Description - Note - You can find the API usage examples on this link, https://github.com/cloud-barista/mc-terrarium/discussions/117
// @Description
// @Tags [Infra Resource] Object Storage Management (under development)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param objectStorageReq body model.RestPostObjectStorageRequest true "Request body to create a Object Storage"
// @Param action query string false "Action" Enums(retry)
// @Success 200 {object} model.ObjectStorageInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// @Router /ns/{nsId}/resources/objectStorage [post]
func RestPostObjectStorage(c echo.Context) error {

	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	action := c.QueryParam("action")
	if action != "retry" && action != "" {
		errMsg := fmt.Errorf("invalid action (%s)", action)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// Bind the request body to RestPostObjectStorageRequest struct
	objectStorageReq := new(model.RestPostObjectStorageRequest)
	if err := c.Bind(objectStorageReq); err != nil {
		log.Warn().Err(err).Msgf("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Validate the CSP is supported
	objectStorageReq.CSP = strings.ToLower(objectStorageReq.CSP)
	ok, err := resource.IsValidCspForObjectStorage(objectStorageReq.CSP)
	if !ok {
		log.Warn().Err(err).Msg("")
		res := model.SimpleMsg{
			Message: err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	err = common.CheckString(objectStorageReq.Name)
	if err != nil {
		errMsg := fmt.Errorf("invalid objectStorageName (%s)", objectStorageReq.Name)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	var resp model.ObjectStorageInfo
	resp, err = resource.CreateObjectStorage(nsId, objectStorageReq, action)
	if err != nil {
		log.Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resp)

}

// RestGetObjectStorage godoc
// @ID GetObjectStorage
// @Summary Get resource info of a Object Storage
// @Description Get resource info of a Object Storage
// @Tags [Infra Resource] Object Storage Management (under development)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param objectStorageId path string true "Object Storage ID" default(objectstorage01)
// @Param detail query string false "Resource info by detail (refined, raw)" default(refined)
// @Success 200 {object} model.ObjectStorageInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// @Router /ns/{nsId}/resources/objectStorage/{objectStorageId} [get]
func RestGetObjectStorage(c echo.Context) error {

	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	objectStorageId := c.Param("objectStorageId")
	err = common.CheckString(objectStorageId)
	if err != nil {
		errMsg := fmt.Errorf("invalid objectStorageId (%s)", objectStorageId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	// // Use this struct like the enum
	// var DetailOptions = struct {
	// 	Refined string
	// 	Raw     string
	// }{
	// 	Refined: "refined",
	// 	Raw:     "raw",
	// }

	// // valid detail options
	// validDetailOptions := map[string]bool{
	// 	DetailOptions.Refined: true,
	// 	DetailOptions.Raw:     true,
	// }

	// detail := c.QueryParam("detail")
	// detail = strings.ToLower(detail)

	// if detail == "" || !validDetailOptions[detail] {
	// 	err := fmt.Errorf("invalid detail (%s), use the default (%s)", detail, DetailOptions.Refined)
	// 	log.Warn().Msg(err.Error())
	// 	detail = DetailOptions.Refined
	// }

	var resp model.ObjectStorageInfo
	// currently, only support detail=refined
	detail := "refined"
	resp, err = resource.GetObjectStorage(nsId, objectStorageId, detail)
	if err != nil {
		log.Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

// RestDeleteObjectStorage godoc
// @ID DeleteObjectStorage
// @Summary Delete a Object Storage
// @Description Delete a Object Storage
// @Tags [Infra Resource] Object Storage Management (under development)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param objectStorageId path string true "Object Storage ID" default(objectstorage01)
// @Success 200 {object} model.SimpleMsg "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// @Router /ns/{nsId}/resources/objectStorage/{objectStorageId} [delete]
func RestDeleteObjectStorage(c echo.Context) error {

	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	objectStorageId := c.Param("objectStorageId")
	err = common.CheckString(objectStorageId)
	if err != nil {
		errMsg := fmt.Errorf("invalid objectStorageId (%s)", objectStorageId)
		log.Warn().Err(err).Msgf(errMsg.Error())
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
	}

	resp, err := resource.DeleteObjectStorage(nsId, objectStorageId)
	if err != nil {
		log.Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

// // RestPutObjectStorage godoc
// // @ID PutObjectStorage
// // @Summary (To be provided) Update the Object Storage
// // @Description (To be provided) Update the Object Storage
// // @Tags [Infra Resource] Object Storage Management (under development)
// // @Accept  json
// // @Produce  json-stream
// // @Param nsId path string true "Namespace ID" default(default)
//// // @Param mciId path string true "MCI ID" default(mci01)
// // @Param vpnId path string true "Object Storage ID" default(objectstorage01)
// // @Param vpnReq body model.RestPostVpnRequest true "Resources info for VPN tunnel configuration between GCP and AWS"
// // @Success 200 {object} model.SimpleMsg "OK"
// // @Failure 400 {object} model.SimpleMsg "Bad Request"
// // @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// // @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// // @Router /ns/{nsId}/resources/qqlDb/{ObjectStorageId} [put]
// func RestPutObjectStorage(c echo.Context) error {

// 	nsId := c.Param("nsId")
// 	err := common.CheckString(nsId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}

// 	mciId := c.Param("mciId")
// 	err = common.CheckString(mciId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid mciId (%s)", mciId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}

// 	vpnId := c.Param("vpnId")
// 	err = common.CheckString(vpnId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid vpnId (%s)", vpnId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}

// 	// Prepare for streaming response
// 	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
// 	c.Response().WriteHeader(http.StatusOK)
// 	enc := json.NewEncoder(c.Response())

// 	// Flush a response
// 	res := model.SimpleMsg{
// 		Message: "note - API to be provided",
// 	}
// 	if err := enc.Encode(res); err != nil {
// 		return err
// 	}
// 	c.Response().Flush()

// 	return nil

// 	// Initialize resty client with basic auth
// 	// client := resty.New()
// 	// apiUser := os.Getenv("TB_API_USERNAME")
// 	// apiPass := os.Getenv("TB_API_PASSWORD")
// 	// client.SetBasicAuth(apiUser, apiPass)

// 	// epTerrarium := "http://localhost:8055/terrarium"
// 	// trId := fmt.Sprintf("%s-%s-%s", nsId, mciId, vpnId)

// 	// // check readyz
// 	// method := "GET"
// 	// url := fmt.Sprintf("%s/readyz", epTerrarium)
// 	// requestBody := common.NoBody
// 	// resReadyz := new(model.Response)

// 	// err := common.ExecuteHttpRequest(
// 	// 	client,
// 	// 	method,
// 	// 	url,
// 	// 	nil,
// 	// 	common.SetUseBody(requestBody),
// 	// 	&requestBody,
// 	// 	resReadyz,
// 	// 	common.VeryShortDuration,
// 	// )

// 	// if err != nil {
// 	// 	log.Err(err).Msg("")
// 	// 	res := model.SimpleMsg{
// 	// 		Message: err.Error(),
// 	// 	}
// 	// 	return c.JSON(http.StatusServiceUnavailable, res)
// 	// }
// 	// log.Debug().Msgf("resReadyz: %+v", resReadyz)

// 	// // Flush a response
// 	// res := model.SimpleMsg{
// 	// 	Message: resReadyz.Message,
// 	// }
// 	// if err := enc.Encode(res); err != nil {
// 	// 	return err
// 	// }
// 	// c.Response().Flush()

// 	// return nil
// }

// // RestGetRequestStatusOfObjectStorage godoc
// // @ID GetRequestStatusOfObjectStorage
// // @Summary Check the status of a specific request by its ID
// // @Description Check the status of a specific request by its ID
// // @Tags [Infra Resource] Object Storage Management (under development)
// // @Accept  json
// // @Produce  json
// // @Param nsId path string true "Namespace ID" default(default)
// // @Param objectStorageId path string true "Object Storage ID" default(objectstorage01)
// // @Param requestId path string true "Request ID"
// // @Success 200 {object} model.Response "OK"
// // @Failure 400 {object} model.SimpleMsg "Bad Request"
// // @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// // @Failure 503 {object} model.SimpleMsg "Service Unavailable"
// // @Router /ns/{nsId}/resources/objectStorage/{objectStorageId}/request/{requestId} [get]
// func RestGetRequestStatusOfObjectStorage(c echo.Context) error {

// 	nsId := c.Param("nsId")
// 	err := common.CheckString(nsId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid nsId (%s)", nsId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}

// 	objectStorageId := c.Param("objectStorageId")
// 	err = common.CheckString(objectStorageId)
// 	if err != nil {
// 		errMsg := fmt.Errorf("invalid objectStorageId (%s)", objectStorageId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}
// 	reqId := c.Param("requestId")
// 	if reqId == "" {
// 		errMsg := fmt.Errorf("invalid reqId (%s)", reqId)
// 		log.Warn().Err(err).Msgf(errMsg.Error())
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errMsg.Error()})
// 	}
// 	reqId = strings.TrimSpace(reqId)

// 	var resp model.Response
// 	resp, err = resource.GetRequestStatusOfObjectStorage(nsId, objectStorageId, reqId)
// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
// 	}

// 	return c.JSON(http.StatusOK, resp)
// }
