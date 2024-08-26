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

// Package resource is to handle REST API for resource
package resource

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostSpec godoc
// @ID PostSpec
// @Summary Register spec
// @Description Register spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param action query string true "registeringMethod" Enums(registerWithInfo, registerWithCspSpecName)
// @Param nsId path string true "Namespace ID" default(system)
// @Param specInfo body model.TbSpecInfo false "Specify details of a spec object (vCPU, memoryGiB, ...) manually"
// @Param specName body model.TbSpecReq false "Specify name, connectionName and cspSpecName to register a spec object automatically"
// @Param update query boolean false "Force update to existing spec object" default(false)
// @Success 200 {object} model.TbSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec [post]
func RestPostSpec(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	updateStr := c.QueryParam("update")
	update, err := strconv.ParseBool(updateStr)
	if err != nil {
		update = false
	}
	log.Debug().Msg("[POST Spec] (action: " + action + ")")

	if action == "registerWithInfo" { // `RegisterSpecWithInfo` will be deprecated in Cappuccino.
		log.Debug().Msg("[Registering Spec with info]")
		u := &model.TbSpecInfo{}
		if err := c.Bind(u); err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content, err := resource.RegisterSpecWithInfo(nsId, u, update)
		return common.EndRequestWithLog(c, reqID, err, content)

	} else { // if action == "registerWithCspSpecName" { // The default mode.
		log.Debug().Msg("[Registering Spec with CspSpecName]")
		u := &model.TbSpecReq{}
		if err := c.Bind(u); err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content, err := resource.RegisterSpecWithCspSpecName(nsId, u, update)
		return common.EndRequestWithLog(c, reqID, err, content)

	} /* else {
		mapA := map[string]string{"message": "LookupSpec(specRequest) failed."}
		return c.JSON(http.StatusFailedDependency, &mapA)
	} */

}

// RestPutSpec godoc
// @ID PutSpec
// @Summary Update spec
// @Description Update spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param specInfo body model.TbSpecInfo true "Details for an spec object"
// @Param nsId path string true "Namespace ID" default(system)
// @Param specId path string true "Spec ID ({providerName}+{regionName}+{specName})"
// @Success 200 {object} model.TbSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [put]
func RestPutSpec(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	specId := c.Param("resourceId")
	specId = strings.ReplaceAll(specId, " ", "+")
	specId = strings.ReplaceAll(specId, "%2B", "+")

	u := &model.TbSpecInfo{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := resource.UpdateSpec(nsId, specId, *u)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// Request structure for RestLookupSpec
type RestLookupSpecRequest struct {
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
}

// RestLookupSpec godoc
// @ID LookupSpec
// @Summary Lookup spec
// @Description Lookup spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param lookupSpecReq body RestLookupSpecRequest true "Specify connectionName & cspSpecName"
// @Success 200 {object} model.SpiderSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupSpec [post]
func RestLookupSpec(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	fmt.Println("[Lookup spec]: " + u.CspSpecName)
	content, err := resource.LookupSpec(u.ConnectionName, u.CspSpecName)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestLookupSpecList godoc
// @ID LookupSpecList
// @Summary Lookup spec list
// @Description Lookup spec list
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param lookupSpecsReq body common.TbConnectionName true "Specify connectionName"
// @Success 200 {object} model.SpiderSpecList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupSpecs [post]
func RestLookupSpecList(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	log.Debug().Msg("[Lookup specs]")
	content, err := resource.LookupSpecList(u.ConnectionName)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestFetchSpecs godoc
// @ID FetchSpecs
// @Summary Fetch specs
// @Description Fetch specs
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/fetchSpecs [post]
func RestFetchSpecs(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	var connConfigCount, specCount uint
	var err error

	if u.ConnectionName == "" {
		connConfigCount, specCount, err = resource.FetchSpecsForAllConnConfigs(nsId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
	} else {
		connConfigCount = 1
		specCount, err = resource.FetchSpecsForConnConfig(u.ConnectionName, nsId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
	}

	content := map[string]string{
		"message": "Fetched " + fmt.Sprint(specCount) + " specs (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestFilterSpecsResponse is Response structure for RestFilterSpecs
type RestFilterSpecsResponse struct {
	Spec []model.TbSpecInfo `json:"spec"`
}

// RestFilterSpecsByRange godoc
// @ID FilterSpecsByRange
// @Summary Filter specs by range
// @Description Filter specs by range
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param specRangeFilter body model.FilterSpecsByRangeRequest false "Filter for range-filtering specs"
// @Success 200 {object} RestFilterSpecsResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/filterSpecsByRange [post]
func RestFilterSpecsByRange(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")

	u := &model.FilterSpecsByRangeRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	log.Debug().Msg("[Filter specs]")
	content, err := resource.FilterSpecsByRange(nsId, *u)
	result := RestFilterSpecsResponse{}
	result.Spec = content
	return common.EndRequestWithLog(c, reqID, err, result)
}

// RestGetSpec godoc
// @ID GetSpec
// @Summary Get spec
// @Description Get spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param specId path string true "Spec ID ({providerName}+{regionName}+{specName})"
// @Success 200 {object} model.TbSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [get]
func RestGetSpec(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	specId := c.Param("resourceId")
	// make " " and "+" to be "+" (web utilizes "+" for " " in URL)
	specId = strings.ReplaceAll(specId, " ", "+")
	specId = strings.ReplaceAll(specId, "%2B", "+")

	log.Debug().Msg("[Get spec]" + specId)
	result, err := resource.GetSpec(nsId, specId)
	return common.EndRequestWithLog(c, reqID, err, result)
}

// RestDelSpec godoc
// @ID DelSpec
// @Summary Delete spec
// @Description Delete spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param specId path string true "Spec ID ({providerName}+{regionName}+{specName})"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [delete]
func RestDelSpec(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
