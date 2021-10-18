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

// Package mcir is to handle REST API for mcir
package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostSpec godoc
// @Summary Register spec
// @Description Register spec
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param registeringMethod query string true "registerWithInfo or else"
// @Param nsId path string true "Namespace ID"
// @Param specInfo body mcir.TbSpecInfo false "Details for an spec object"
// @Param specName body mcir.TbSpecReq false "name, connectionName and cspSpecName"
// @Success 200 {object} mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec [post]
func RestPostSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	fmt.Println("[POST Spec] (action: " + action + ")")

	if action == "registerWithInfo" { // `RegisterSpecWithInfo` will be deprecated in Cappuccino.
		fmt.Println("[Registering Spec with info]")
		u := &mcir.TbSpecInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := mcir.RegisterSpecWithInfo(nsId, u)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}
		return c.JSON(http.StatusCreated, content)

	} else { // if action == "registerWithCspSpecName" { // The default mode.
		fmt.Println("[Registering Spec with CspSpecName]")
		u := &mcir.TbSpecReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := mcir.RegisterSpecWithCspSpecName(nsId, u)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}
		return c.JSON(http.StatusCreated, content)

	} /* else {
		mapA := map[string]string{"message": "LookupSpec(specRequest) failed."}
		return c.JSON(http.StatusFailedDependency, &mapA)
	} */

}

// RestPutSpec godoc
// @Summary Update spec
// @Description Update spec
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param specInfo body mcir.TbSpecInfo true "Details for an spec object"
// @Param nsId path string true "Namespace ID"
// @Param specId path string true "Spec ID"
// @Success 200 {object} mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [put]
func RestPutSpec(c echo.Context) error {
	nsId := c.Param("nsId")
	specId := c.Param("resourceId")
	fmt.Printf("RestPutSpec called; nsId: %s, specId: %s \n", nsId, specId) // for debug

	u := &mcir.TbSpecInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	updatedSpec, err := mcir.UpdateSpec(nsId, specId, *u)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{
			"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusOK, updatedSpec)
}

// Request structure for RestLookupSpec
type RestLookupSpecRequest struct {
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
}

// RestLookupSpec godoc
// @Summary Lookup spec
// @Description Lookup spec
// @Tags [Admin] Cloud environment management
// @Accept  json
// @Produce  json
// @Param lookupSpecReq body RestLookupSpecRequest true "Specify connectionName & cspSpecName"
// @Success 200 {object} mcir.SpiderSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /lookupSpec [post]
func RestLookupSpec(c echo.Context) error {
	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Lookup spec]: " + u.CspSpecName)
	content, err := mcir.LookupSpec(u.ConnectionName, u.CspSpecName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// RestLookupSpecList godoc
// @Summary Lookup spec list
// @Description Lookup spec list
// @Tags [Admin] Cloud environment management
// @Accept  json
// @Produce  json
// @Param lookupSpecsReq body common.TbConnectionName true "Specify connectionName"
// @Success 200 {object} mcir.SpiderSpecList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /lookupSpecs [post]
func RestLookupSpecList(c echo.Context) error {

	//type JsonTemplate struct {
	//	ConnectionName string
	//}

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Lookup specs]")
	content, err := mcir.LookupSpecList(u.ConnectionName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// RestFetchSpecs godoc
// @Summary Fetch specs
// @Description Fetch specs
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/fetchSpecs [post]
func RestFetchSpecs(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	var connConfigCount, specCount uint
	var err error

	if u.ConnectionName == "" {
		connConfigCount, specCount, err = mcir.FetchSpecsForAllConnConfigs(nsId)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}
	} else {
		connConfigCount = 1
		specCount, err = mcir.FetchSpecsForConnConfig(u.ConnectionName, nsId)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}
	}

	mapA := map[string]string{
		"message": "Fetched " + fmt.Sprint(specCount) + " specs (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return c.JSON(http.StatusCreated, &mapA) //content)
}

// RestFilterSpecsResponse is Response structure for RestFilterSpecs
type RestFilterSpecsResponse struct {
	Spec []mcir.TbSpecInfo `json:"spec"`
}

// RestFilterSpecs godoc
// @Summary Filter specs
// @Description Filter specs
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param specFilter body mcir.TbSpecInfo false "Filter for filtering specs"
// @Success 200 {object} RestFilterSpecsResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/filterSpecs [post]
func RestFilterSpecs(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbSpecInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Filter specs]")
	content, err := mcir.FilterSpecs(nsId, *u)

	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	result := RestFilterSpecsResponse{}
	result.Spec = content
	return c.JSON(http.StatusOK, &result)
}

// RestFilterSpecsByRange godoc
// @Summary Filter specs by range
// @Description Filter specs by range
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param specRangeFilter body mcir.FilterSpecsByRangeRequest false "Filter for range-filtering specs"
// @Success 200 {object} RestFilterSpecsResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/filterSpecsByRange [post]
func RestFilterSpecsByRange(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.FilterSpecsByRangeRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Filter specs]")
	content, err := mcir.FilterSpecsByRange(nsId, *u)

	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	result := RestFilterSpecsResponse{}
	result.Spec = content
	return c.JSON(http.StatusOK, &result)
}

func RestTestSortSpecs(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbSpecInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Filter specs]")
	content, err := mcir.FilterSpecs(nsId, *u)

	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	content, err = mcir.SortSpecs(content, "memGiB", "descending")
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	result := RestFilterSpecsResponse{}
	result.Spec = content
	return c.JSON(http.StatusOK, &result)
}

// RestGetSpec godoc
// @Summary Get spec
// @Description Get spec
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param specId path string true "Spec ID"
// @Success 200 {object} mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [get]
func RestGetSpec(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllSpec
type RestGetAllSpecResponse struct {
	Spec []mcir.TbSpecInfo `json:"spec"`
}

// RestGetAllSpec godoc
// @Summary List all specs or specs' ID
// @Description List all specs or specs' ID
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSpecResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec [get]
func RestGetAllSpec(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelSpec godoc
// @Summary Delete spec
// @Description Delete spec
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param specId path string true "Spec ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [delete]
func RestDelSpec(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllSpec godoc
// @Summary Delete all specs
// @Description Delete all specs
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec [delete]
func RestDelAllSpec(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
