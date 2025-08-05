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
package infra

import (
	"net/http"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostMci godoc
// @ID PostMci
// @Summary Create MCI
// @Description Create MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciReq true "Details for an MCI object"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci [post]
func RestPostMci(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.TbMciReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	option := "create"
	result, err := infra.CreateMci(nsId, req, option)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostRegisterCSPNativeVM godoc
// @ID PostRegisterCSPNativeVM
// @Summary Register existing VM in a CSP to Cloud-Barista MCI
// @Description Register existing VM in a CSP to Cloud-Barista MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciReq true "Details for an MCI object with existing CSP VM ID"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/registerCspVm [post]
func RestPostRegisterCSPNativeVM(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.TbMciReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	option := "register"
	result, err := infra.CreateMci(nsId, req, option)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostSystemMci godoc
// @ID PostSystemMci
// @Summary Create System MCI Dynamically for Special Purpose in NS:system
// @Description Create System MCI Dynamically for Special Purpose
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param option query string false "Option for the purpose of system MCI" Enums(probe)
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /systemMci [post]
func RestPostSystemMci(c echo.Context) error {

	option := c.QueryParam("option")

	req := &model.TbMciDynamicReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateSystemMciDynamic(option)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciDynamic godoc
// @ID PostMciDynamic
// @Summary Create MCI Dynamically
// @Description Create MCI Dynamically from common spec and image
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciDynamicReq true "Request body to provision MCI dynamically. Must include commonSpec and commonImage info of each VM request. Example multi-cloud setup: {\"name\":\"mc-infra\",\"description\":\"Multi-cloud infrastructure\",\"vm\":[{\"name\":\"aws-workers\",\"subGroupSize\":\"3\",\"commonSpec\":\"aws+ap-northeast-2+t3.nano\",\"commonImage\":\"ami-01f71f215b23ba262\",\"rootDiskSize\":\"50\",\"label\":{\"role\":\"worker\",\"csp\":\"aws\"}},{\"name\":\"azure-head\",\"subGroupSize\":\"2\",\"commonSpec\":\"azure+koreasouth+standard_b1s\",\"commonImage\":\"Canonical:0001-com-ubuntu-server-jammy:22_04-lts:22.04.202505210\",\"rootDiskSize\":\"50\",\"label\":{\"role\":\"head\",\"csp\":\"azure\"}},{\"name\":\"gcp-test\",\"subGroupSize\":\"1\",\"commonSpec\":\"gcp+asia-northeast3+g1-small\",\"commonImage\":\"https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20250712\",\"rootDiskSize\":\"50\",\"label\":{\"role\":\"test\",\"csp\":\"gcp\"}}}]. Use /mciRecommendVm and /mciDynamicCheckRequest for resource discovery. Guide: https://github.com/cloud-barista/cb-tumblebug/discussions/1570"
// @Param option query string false "Option for MCI creation" Enums(hold)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mciDynamic [post]
func RestPostMciDynamic(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.TbMciDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciDynamic(reqID, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create MCI dynamically")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostMciDynamicReview godoc
// @ID PostMciDynamicReview
// @Summary Review and Validate MCI Dynamic Request
// @Description Review and validate MCI dynamic request comprehensively before actual provisioning.
// @Description This endpoint performs comprehensive validation of MCI dynamic creation requests without actually creating resources.
// @Description It checks resource availability, validates specifications and images, estimates costs, and provides detailed recommendations.
// @Description
// @Description **Key Features:**
// @Description - Validates all VM specifications and images against CSP availability
// @Description - Provides cost estimation (including partial estimates when some costs are unknown)
// @Description - Identifies potential configuration issues and warnings
// @Description - Recommends optimization strategies
// @Description - Shows provider and region distribution
// @Description - Non-invasive validation (no resources are created)
// @Description
// @Description **Review Status:**
// @Description - `Ready`: All VMs can be created successfully
// @Description - `Warning`: VMs can be created but with configuration warnings
// @Description - `Error`: Critical errors prevent MCI creation
// @Description
// @Description **Use Cases:**
// @Description - Pre-validation before expensive MCI creation
// @Description - Cost estimation and planning
// @Description - Configuration optimization
// @Description - Multi-cloud resource planning
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.TbMciDynamicReq true "Request body to review MCI dynamic provisioning. Must include commonSpec and commonImage info of each VM request. Same format as /mciDynamic endpoint. (ex: {name: mci01, vm: [{commonImage: aws+ap-northeast-2+ubuntu22.04, commonSpec: aws+ap-northeast-2+t2.small}]})"
// @Param option query string false "Option for MCI creation review (same as actual creation)" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking"
// @Success 200 {object} model.ReviewMciDynamicReqInfo "Comprehensive review result with validation status, cost estimation, and recommendations"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or parameters"
// @Failure 404 {object} model.SimpleMsg "Namespace not found or invalid"
// @Failure 500 {object} model.SimpleMsg "Internal server error during validation"
// @Router /ns/{nsId}/mciDynamicReview [post]
func RestPostMciDynamicReview(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.TbMciDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for MCI dynamic review")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ReviewMciDynamicReq(reqID, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to review MCI dynamic request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostMciVmDynamic godoc
// @ID PostMciVmDynamic
// @Summary Create VM Dynamically and add it to MCI
// @Description Create VM Dynamically and add it to MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmReq body model.TbVmDynamicReq true "Details for Vm dynamic request"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vmDynamic [post]
func RestPostMciVmDynamic(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &model.TbVmDynamicReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciVmDynamic(nsId, mciId, req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciDynamicCheckRequest godoc
// @ID PostMciDynamicCheckRequest
// @Summary Check available ConnectionConfig list for creating MCI Dynamically
// @Description Check available ConnectionConfig list before create MCI Dynamically from common spec and image
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param mciReq body model.MciConnectionConfigCandidatesReq true "Details for MCI dynamic request information"
// @Success 200 {object} model.CheckMciDynamicReqInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /mciDynamicCheckRequest [post]
func RestPostMciDynamicCheckRequest(c echo.Context) error {

	req := &model.MciConnectionConfigCandidatesReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CheckMciDynamicReq(req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciVm godoc
// @ID PostMciVm
// @Summary Create and add homogeneous VMs(subGroup) to a specified MCI (Set subGroupSize for multiple VMs)
// @Description Create and add homogeneous VMs(subGroup) to a specified MCI (Set subGroupSize for multiple VMs)
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param vmReq body model.TbVmReq true "Details for VMs(subGroup)"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/vm [post]
func RestPostMciVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	vmInfoData := &model.TbVmReq{}
	if err := c.Bind(vmInfoData); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	result, err := infra.CreateMciGroupVm(nsId, mciId, vmInfoData, true)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciSubGroupScaleOut godoc
// @ID PostMciSubGroupScaleOut
// @Summary ScaleOut subGroup in specified MCI
// @Description ScaleOut subGroup in specified MCI
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param subgroupId path string true "subGroup ID" default(g1)
// @Param vmReq body model.TbScaleOutSubGroupReq true "subGroup scaleOut request"
// @Success 200 {object} model.TbMciInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/mci/{mciId}/subgroup/{subgroupId} [post]
func RestPostMciSubGroupScaleOut(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subgroupId := c.Param("subgroupId")

	scaleOutReq := &model.TbScaleOutSubGroupReq{}
	if err := c.Bind(scaleOutReq); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ScaleOutMciSubGroup(nsId, mciId, subgroupId, scaleOutReq.NumVMsToAdd)
	return clientManager.EndRequestWithLog(c, err, result)
}
