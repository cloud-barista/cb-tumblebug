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

// Package infra is to handle REST API for infra
package infra

import (
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestPostNLB godoc
// @ID PostNLB
// @Summary Create NLB
// @Description Create NLB
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param option query string false "Option: [required params for register] connectionName, name, cspResourceId" Enums(register)
// @Param nlbReq body model.NLBReq true "Details of the NLB object"
// @Success 200 {object} model.NLBInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb [post]
func RestPostNLB(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	optionFlag := c.QueryParam("option")

	u := &model.NLBReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.CreateNLB(nsId, infraId, u, optionFlag)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPostMcNLB godoc
// @ID PostMcNLB
// @Summary Create a special purpose Infra for NLB and depoly and setting SW NLB
// @Description Create a special purpose Infra for NLB and depoly and setting SW NLB
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbReq body model.NLBReq true "Details of the NLB object"
// @Success 200 {object} model.McNlbInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/mcSwNlb [post]
func RestPostMcNLB(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	u := &model.NLBReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.CreateMcSwNlb(nsId, infraId, u, "")
	return clientManager.EndRequestWithLog(c, err, content)
}

/*
	function RestPutNLB not yet implemented

// RestPutNLB godoc
// @ID PutNLB
// @Summary Update NLB
// @Description Update NLB
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Param nlbInfo body model.NLBInfo true "Details of the NLB object"
// @Success 200 {object} model.NLBInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb/{nlbId} [put]
*/
func RestPutNLB(c echo.Context) error {
	// nsId := c.Param("nsId")
	// infraId := c.Param("infraId")

	return nil
}

// RestGetNLB godoc
// @ID GetNLB
// @Summary Get NLB
// @Description Get NLB
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Success 200 {object} model.NLBInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb/{nlbId} [get]
func RestGetNLB(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	resourceId := c.Param("resourceId")

	res, err := infra.GetNLB(nsId, infraId, resourceId)
	return clientManager.EndRequestWithLog(c, err, res)
}

// Response structure for RestGetAllNLB
type RestGetAllNLBResponse struct {
	NLB []model.NLBInfo `json:"nlb"`
}

// RestGetAllNLB godoc
// @ID GetAllNLB
// @Summary List all NLBs or NLBs' ID
// @Description List all NLBs or NLBs' ID
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: cspResourceName)"
// @Param filterVal query string false "Field value for filtering (ex: default-alibaba-ap-northeast-1-vpc)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllNLBResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb [get]
func RestGetAllNLB(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	optionFlag := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")

	if optionFlag == "id" {
		content := model.IdList{}
		var err error
		content.IdList, err = infra.ListNLBId(nsId, infraId)
		return clientManager.EndRequestWithLog(c, err, content)
	} else {

		resourceList, err := infra.ListNLB(nsId, infraId, filterKey, filterVal)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}

		var content struct {
			NLB []model.NLBInfo `json:"nlb"`
		}

		content.NLB = resourceList.([]model.NLBInfo) // type assertion (interface{} -> array)
		return clientManager.EndRequestWithLog(c, err, content)
	}
}

// RestDelNLB godoc
// @ID DelNLB
// @Summary Delete NLB
// @Description Delete NLB
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbId path string true "NLB ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb/{nlbId} [delete]
func RestDelNLB(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	resourceId := c.Param("resourceId")

	forceFlag := c.QueryParam("force")

	err := infra.DelNLB(nsId, infraId, resourceId, forceFlag)
	content := map[string]string{"message": "The NLB " + resourceId + " has been deleted"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestDelAllNLB godoc
// @ID DelAllNLB
// @Summary Delete all NLBs
// @Description Delete all NLBs
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb [delete]
func RestDelAllNLB(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	forceFlag := c.QueryParam("force")
	subString := c.QueryParam("match")

	content, err := infra.DelAllNLB(nsId, infraId, subString, forceFlag)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetNLBHealth godoc
// @ID GetNLBHealth
// @Summary Get NLB Health
// @Description Get NLB Health
// @Tags [Infra Resource] NLB Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Success 200 {object} model.NLBInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb/{nlbId}/healthz [get]
func RestGetNLBHealth(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	resourceId := c.Param("resourceId")

	content, err := infra.GetNLBHealth(nsId, infraId, resourceId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// The REST APIs below are for dev/test only

// RestAddNLBVMs godoc
// @ID AddNLBVMs
// @Summary Add VMs to NLB
// @Description Add VMs to NLB
// @Tags [Infra Resource] NLB Management (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Param nlbAddRemoveVMReq body model.NLBAddRemoveVMReq true "VMs to add to NLB"
// @Success 200 {object} model.NLBInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb/{nlbId}/vm [post]
func RestAddNLBVMs(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	resourceId := c.Param("resourceId")

	u := &model.NLBAddRemoveVMReq{}
	if err := c.Bind(u); err != nil {
		return err
	}
	content, err := infra.AddNLBVMs(nsId, infraId, resourceId, u)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestRemoveNLBVMs godoc
// @ID RemoveNLBVMs
// @Summary Delete VMs from NLB
// @Description Delete VMs from NLB
// @Tags [Infra Resource] NLB Management (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Param nlbAddRemoveVMReq body model.NLBAddRemoveVMReq true "Select VMs to remove from NLB"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nlb/{nlbId}/vm [delete]
func RestRemoveNLBVMs(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	resourceId := c.Param("resourceId")

	u := &model.NLBAddRemoveVMReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	err := infra.RemoveNLBVMs(nsId, infraId, resourceId, u)
	content := map[string]string{"message": "Removed VMs from the NLB " + resourceId}
	return clientManager.EndRequestWithLog(c, err, content)
}
