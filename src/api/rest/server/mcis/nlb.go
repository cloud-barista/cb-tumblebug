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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostNLB godoc
// @Summary Create NLB
// @Description Create NLB
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param option query string false "Option: [required params for register] connectionName, name, cspNLBId" Enums(register)
// @Param nlbReq body mcis.TbNLBReq true "Details of the NLB object"
// @Success 200 {object} mcis.TbNLBInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb [post]
func RestPostNLB(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	optionFlag := c.QueryParam("option")

	u := &mcis.TbNLBReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcis.CreateNLB(nsId, mcisId, u, optionFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestPostMcNLB godoc
// @Summary Create a special purpose MCIS for NLB and depoly and setting SW NLB
// @Description Create a special purpose MCIS for NLB and depoly and setting SW NLB
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbReq body mcis.TbNLBReq true "Details of the NLB object"
// @Success 200 {object} mcis.McNlbInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/mcSwNlb [post]
func RestPostMcNLB(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	u := &mcis.TbNLBReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcis.CreateMcSwNlb(nsId, mcisId, u, "")
	return common.EndRequestWithLog(c, reqID, err, content)
}

/*
	function RestPutNLB not yet implemented

// RestPutNLB godoc
// @Summary Update NLB
// @Description Update NLB
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Param nlbInfo body mcis.TbNLBInfo true "Details of the NLB object"
// @Success 200 {object} mcis.TbNLBInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId} [put]
*/
func RestPutNLB(c echo.Context) error {
	// nsId := c.Param("nsId")
	// mcisId := c.Param("mcisId")

	return nil
}

// RestGetNLB godoc
// @Summary Get NLB
// @Description Get NLB
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Success 200 {object} mcis.TbNLBInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId} [get]
func RestGetNLB(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	resourceId := c.Param("resourceId")

	res, err := mcis.GetNLB(nsId, mcisId, resourceId)
	return common.EndRequestWithLog(c, reqID, err, res)
}

// Response structure for RestGetAllNLB
type RestGetAllNLBResponse struct {
	NLB []mcis.TbNLBInfo `json:"nlb"`
}

// RestGetAllNLB godoc
// @Summary List all NLBs or NLBs' ID
// @Description List all NLBs or NLBs' ID
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: cspNLBName)"
// @Param filterVal query string false "Field value for filtering (ex: ns01-alibaba-ap-northeast-1-vpc)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllNLBResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb [get]
func RestGetAllNLB(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	optionFlag := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")

	if optionFlag == "id" {
		content := common.IdList{}
		var err error
		content.IdList, err = mcis.ListNLBId(nsId, mcisId)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else {

		resourceList, err := mcis.ListNLB(nsId, mcisId, filterKey, filterVal)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}

		var content struct {
			NLB []mcis.TbNLBInfo `json:"nlb"`
		}

		content.NLB = resourceList.([]mcis.TbNLBInfo) // type assertion (interface{} -> array)
		return common.EndRequestWithLog(c, reqID, err, content)
	}
}

// RestDelNLB godoc
// @Summary Delete NLB
// @Description Delete NLB
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbId path string true "NLB ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId} [delete]
func RestDelNLB(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	resourceId := c.Param("resourceId")

	forceFlag := c.QueryParam("force")

	err := mcis.DelNLB(nsId, mcisId, resourceId, forceFlag)
	content := map[string]string{"message": "The NLB " + resourceId + " has been deleted"}
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestDelAllNLB godoc
// @Summary Delete all NLBs
// @Description Delete all NLBs
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb [delete]
func RestDelAllNLB(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	forceFlag := c.QueryParam("force")
	subString := c.QueryParam("match")

	content, err := mcis.DelAllNLB(nsId, mcisId, subString, forceFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetNLBHealth godoc
// @Summary Get NLB Health
// @Description Get NLB Health
// @Tags [Infra resource] NLB management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Success 200 {object} mcis.TbNLBInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/healthz [get]
func RestGetNLBHealth(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	resourceId := c.Param("resourceId")

	content, err := mcis.GetNLBHealth(nsId, mcisId, resourceId)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// The REST APIs below are for dev/test only

// RestAddNLBVMs godoc
// @Summary Add VMs to NLB
// @Description Add VMs to NLB
// @Tags [Infra resource] NLB management (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Param nlbAddRemoveVMReq body mcis.TbNLBAddRemoveVMReq true "VMs to add to NLB"
// @Success 200 {object} mcis.TbNLBInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/vm [post]
func RestAddNLBVMs(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	resourceId := c.Param("resourceId")

	u := &mcis.TbNLBAddRemoveVMReq{}
	if err := c.Bind(u); err != nil {
		return err
	}
	content, err := mcis.AddNLBVMs(nsId, mcisId, resourceId, u)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestRemoveNLBVMs godoc
// @Summary Delete VMs from NLB
// @Description Delete VMs from NLB
// @Tags [Infra resource] NLB management (for developer)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param nlbId path string true "NLB ID" default(g1)
// @Param nlbAddRemoveVMReq body mcis.TbNLBAddRemoveVMReq true "VMs to add to NLB"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/nlb/{nlbId}/vm [delete]
func RestRemoveNLBVMs(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	resourceId := c.Param("resourceId")

	u := &mcis.TbNLBAddRemoveVMReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	err := mcis.RemoveNLBVMs(nsId, mcisId, resourceId, u)
	content := map[string]string{"message": "Removed VMs from the NLB " + resourceId}
	return common.EndRequestWithLog(c, reqID, err, content)
}
