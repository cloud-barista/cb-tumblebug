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
	fmt.Println("[POST Spec requested action: " + action)

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
// @Success 200 {object} mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec/{specId} [put]
func RestPutSpec(c echo.Context) error {
	nsId := c.Param("nsId")
	specId := c.Param("specId")
	fmt.Printf("RestPutSpec called; nsId: %s, specId: %s \n", nsId, specId) // for debug

	u := &mcir.TbSpecInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		if specId != u.Id {
			err := fmt.Errorf("URL param " + specId + " and JSON param " + u.Id + " does not match.")
			common.CBLog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusBadRequest, &mapA)
		}
	*/

	updatedSpec, err := mcir.UpdateSpec(nsId, *u)
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
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param connectionName body RestLookupSpecRequest true "Specify connectionName"
// @Param specName path string true "Spec name"
// @Success 200 {object} mcir.SpiderSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /lookupSpec [get]
func RestLookupSpec(c echo.Context) error {

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Lookup spec]" + u.CspSpecName)
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
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param connectionName body RestLookupSpecRequest true "Specify connectionName"
// @Success 200 {object} mcir.SpiderSpecList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /lookupSpecs [get]
func RestLookupSpecList(c echo.Context) error {

	//type JsonTemplate struct {
	//	ConnectionName string
	//}

	u := &RestLookupSpecRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Get Region List]")
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

	connConfigCount, specCount, err := mcir.FetchSpecs(nsId)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{
			"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{
		"message": "Fetched " + fmt.Sprint(specCount) + " specs (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return c.JSON(http.StatusCreated, &mapA) //content)
}

// Response structure for RestFilterSpecs
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

	content, err = mcir.SortSpecs(content, "mem_GiB", "descending")
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
	// Obsolete function. This is just for Swagger.
	return nil
}

// Response structure for RestGetAllSpec
type RestGetAllSpecResponse struct {
	Spec []mcir.TbSpecInfo `json:"spec"`
}

// RestGetAllSpec godoc
// @Summary List all specs
// @Description List all specs
// @Tags [MCIR] Spec management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllSpecResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/spec [get]
func RestGetAllSpec(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
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
	// Obsolete function. This is just for Swagger.
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
	// Obsolete function. This is just for Swagger.
	return nil
}
