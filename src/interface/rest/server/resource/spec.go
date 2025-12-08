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
	"strconv"
	"strings"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
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
// @Param action query string true "registeringMethod" Enums(registerWithInfo, registerWithCspResourceId)
// @Param nsId path string true "Namespace ID" default(system)
// @Param specInfo body model.SpecInfo false "Specify details of a spec object (vCPU, memoryGiB, ...) manually"
// @Param specReq body model.SpecReq false "Specify n(ame, connectionName, cspSpecName) to register a spec object automatically"
// @Param update query boolean false "Force update to existing spec object" default(false)
// @Success 200 {object} model.SpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec [post]
func RestPostSpec(c echo.Context) error {

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
		u := &model.SpecInfo{}
		if err := c.Bind(u); err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content, err := resource.RegisterSpecWithInfo(nsId, u, update)
		return clientManager.EndRequestWithLog(c, err, content)

	} else { // if action == "registerWithCspResourceId" { // The default mode.
		log.Debug().Msg("[Registering Spec with cspSpecName]")
		u := &model.SpecReq{}
		if err := c.Bind(u); err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content, err := resource.RegisterSpecWithCspResourceId(nsId, u, update)
		return clientManager.EndRequestWithLog(c, err, content)

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
// @Param specInfo body model.SpecInfo true "Details for an spec object"
// @Param nsId path string true "Namespace ID" default(system)
// @Param specId path string true "Spec ID ({providerName}+{regionName}+{cspSpecName})"
// @Success 200 {object} model.SpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [put]
func RestPutSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	specId := c.Param("resourceId")
	specId = strings.ReplaceAll(specId, " ", "+")
	specId = strings.ReplaceAll(specId, "%2B", "+")

	u := &model.SpecInfo{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.UpdateSpec(nsId, specId, *u)
	return clientManager.EndRequestWithLog(c, err, content)
}

// Request structure for RestLookupSpec
type RestLookupSpecRequest struct {
	ConnectionName string `json:"connectionName"`
	CspResourceId  string `json:"cspResourceId"`
}

// RestLookupSpec godoc
// @ID LookupSpec
// @Summary Lookup spec (for debugging purposes)
// @Description Lookup spec (for debugging purposes)
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param lookupSpecReq body RestLookupSpecRequest true "Specify connectionName & cspSpecNameS"
// @Success 200 {object} model.SpiderSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupSpec [post]
func RestLookupSpec(c echo.Context) error {

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	fmt.Println("[Lookup spec]: " + u.CspResourceId)
	content, err := resource.LookupSpec(u.ConnectionName, u.CspResourceId)
	return clientManager.EndRequestWithLog(c, err, content)

}

// RestLookupSpecList godoc
// @ID LookupSpecList
// @Summary Lookup spec list (for debugging purposes)
// @Description Lookup spec list (for debugging purposes)
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param lookupSpecsReq body common.TbConnectionName true "Specify connectionName"
// @Success 200 {object} model.SpiderSpecList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupSpecs [post]
func RestLookupSpecList(c echo.Context) error {

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msg("[Lookup specs]")
	content, err := resource.LookupSpecList(u.ConnectionName)
	return clientManager.EndRequestWithLog(c, err, content)

}

// RestFetchSpecs godoc
// @ID FetchSpecs
// @Summary Fetch specs from CSPs and register them in the system.
// @Description Fetch specs from CSPs and register them in the system.
// @Description
// @Description **Provider Selection Options:**
// @Description - `targetProviders`: Specify exact providers to fetch (e.g., ["aws", "gcp"]). When set, only these providers are processed and `excludedProviders` is ignored.
// @Description - `excludedProviders`: Specify providers to skip (e.g., ["azure"]). Only used when `targetProviders` is not set.
// @Description - `regionAgnosticProviders`: Providers where specs are shared across regions (e.g., ["gcp", "tencent"]). Only one region will be fetched per provider.
// @Description
// @Description **Note:** `regionAgnosticProviders` should only contain providers that are also in `targetProviders` (or not excluded).
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param fetchOption body model.SpecFetchOption true "Fetch option"
// @Success 202 {object} resource.FetchSpecsAsyncResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /fetchSpecs [post]
func RestFetchSpecs(c echo.Context) error {
	nsId := model.SystemCommonNs

	reqBody := &model.SpecFetchOption{}
	if err := c.Bind(reqBody); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.FetchSpecsForAllConnConfigs(nsId, reqBody)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, err, content)
}

// RestFetchPrice godoc
// @ID FetchPrice
// @Summary Fetch price from all CSP connections and update the price information for associated specs in the system.
// @Description Fetch price from all CSP connections and update the price information for associated specs in the system.
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /fetchPrice [post]
func RestFetchPrice(c echo.Context) error {

	connConfigCount, _, err := resource.FetchPriceForAllConnConfigs()
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content := map[string]string{
		"message": "Fetched prices (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestFilterSpecsResponse is Response structure for RestFilterSpecs
type RestFilterSpecsResponse struct {
	Spec []model.SpecInfo `json:"spec"`
}

// RestFilterSpecsByRange godoc
// @ID FilterSpecsByRange
// @Summary Filter specs by range
// @Description Filter specs by range. Use limit field to control the maximum number of results. If limit is 0 or not specified, returns all matching results.
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param specRangeFilter body model.FilterSpecsByRangeRequest false "Filter for range-filtering specs (limit: 0 for all results, >0 for limited results)"
// @Success 200 {object} RestFilterSpecsResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/filterSpecsByRange [post]
func RestFilterSpecsByRange(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &model.FilterSpecsByRangeRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msg("[Filter specs]")
	content, err := resource.FilterSpecsByRange(nsId, *u, "")
	result := RestFilterSpecsResponse{}
	result.Spec = content
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetSpec godoc
// @ID GetSpec
// @Summary Get spec
// @Description Get spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param specId path string true "Spec ID ({providerName}+{regionName}+{cspSpecName})"
// @Success 200 {object} model.SpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [get]
func RestGetSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	specId := c.Param("resourceId")
	// make " " and "+" to be "+" (web utilizes "+" for " " in URL)
	specId = strings.ReplaceAll(specId, " ", "+")
	specId = strings.ReplaceAll(specId, "%2B", "+")

	log.Debug().Msg("[Get spec]" + specId)
	result, err := resource.GetSpec(nsId, specId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestDelSpec godoc
// @ID DelSpec
// @Summary Delete spec
// @Description Delete spec
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param specId path string true "Spec ID ({providerName}+{regionName}+{cspSpecName})"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [delete]
func RestDelSpec(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestGetAvailableRegionZonesForSpec godoc
// @ID GetAvailableRegionZonesForSpec
// @Summary Get available regions and zones for a specific spec
// @Description Query the availability of a specific spec across all regions/zones
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param availabilityReq body model.GetAvailableRegionZonesRequest true "Spec availability request"
// @Success 200 {object} model.SpecAvailabilityInfo
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /availableRegionZonesForSpec [post]
func RestGetAvailableRegionZonesForSpec(c echo.Context) error {
	u := &model.GetAvailableRegionZonesRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if u.Provider == "" || u.CspSpecName == "" {
		err := fmt.Errorf("provider and cspSpecName are required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msgf("[Get Spec Availability] Provider: %s, CspSpecName: %s", u.Provider, u.CspSpecName)

	content, err := resource.GetAvailableRegionZonesForSpec(u.Provider, u.CspSpecName)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetAvailableRegionZonesForSpecList godoc
// @ID GetAvailableRegionZonesForSpecList
// @Summary Get available regions and zones for multiple specs
// @Description Query the availability for multiple specs in parallel and return batch results
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param batchAvailabilityReq body model.GetAvailableRegionZonesListRequest true "Batch spec availability request"
// @Success 200 {object} model.SpecAvailabilityBatchResult
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /availableRegionZonesForSpecList [post]
func RestGetAvailableRegionZonesForSpecList(c echo.Context) error {
	u := &model.GetAvailableRegionZonesListRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if u.Provider == "" || len(u.CspSpecNames) == 0 {
		err := fmt.Errorf("provider and cspSpecNames are required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msgf("[Get Batch Spec Availability] Provider: %s, Specs: %d", u.Provider, len(u.CspSpecNames))

	content, err := resource.GetAvailableRegionZonesForSpecList(u.Provider, u.CspSpecNames)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestUpdateExistingSpecListByAvailableRegionZones godoc
// @ID UpdateExistingSpecListByAvailableRegionZones
// @Summary Clean up unavailable specs from database
// @Description Query all specs for a specific provider across all regions, check their availability, and remove specs that are not available in their respective regions
// @Tags [Infra Resource] Spec Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param cleanupReq body model.UpdateSpecListByAvailabilityRequest true "Spec cleanup request"
// @Success 200 {object} model.SpecCleanupResult
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/updateExistingSpecListByAvailableRegionZones [post]
func RestUpdateExistingSpecListByAvailableRegionZones(c echo.Context) error {
	nsId := c.Param("nsId")

	u := &model.UpdateSpecListByAvailabilityRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if u.Provider == "" {
		err := fmt.Errorf("provider is required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msgf("[Cleanup Specs] Namespace: %s, Provider: %s", nsId, u.Provider)

	content, err := resource.UpdateExistingSpecListByAvailableRegionZones(nsId, u.Provider)
	return clientManager.EndRequestWithLog(c, err, content)
}
