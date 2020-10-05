package common

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

func RestCheckNs(c echo.Context) error {

	/*
		nsId := c.Param("nsId")

		exists, err := common.CheckNs(nsId)

		type JsonTemplate struct {
			Exists bool `json:exists`
		}
		content := JsonTemplate{}
		content.Exists = exists

		if err != nil {
			common.CBLog.Error(err)
			//mapA := common.SimpleMsg{err.Error()}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return c.JSON(http.StatusNotFound, &content)
		}

		return c.JSON(http.StatusOK, &content)
	*/

	if err := Validate(c, []string{"nsId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	exists, err := common.CheckNs(c.Param("nsId"))
	if err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	return SendExistence(c, http.StatusOK, exists)
}

// RestDelAllNs godoc
// @Summary Delete all namespaces
// @Description Delete all namespaces
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns [delete]
func RestDelAllNs(c echo.Context) error {

	/*
		err := common.DelAllNs()
		if err != nil {
			common.CBLog.Error(err)
			mapA := common.SimpleMsg{err.Error()}
			return c.JSON(http.StatusConflict, &mapA)
		}

		mapA := common.SimpleMsg{"All namespaces has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/

	err := common.DelAllNs()
	if err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	return SendMessage(c, http.StatusOK, "All namespaces has been deleted")
}

// RestDelNs godoc
// @Summary Delete namespace
// @Description Delete namespace
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId} [delete]
func RestDelNs(c echo.Context) error {

	/*
		id := c.Param("nsId")

		err := common.DelNs(id)
		if err != nil {
			common.CBLog.Error(err)
			mapA := common.SimpleMsg{err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := common.SimpleMsg{"The ns has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/

	if err := Validate(c, []string{"nsId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	err := common.DelNs(c.Param("nsId"))
	if err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	return SendMessage(c, http.StatusOK, "The ns has been deleted")
}

// Response structure for RestGetAllNs
type RestGetAllNsResponse struct {
	//Name string     `json:"name"`
	Ns []common.NsInfo `json:"ns"`
}

// RestGetAllNs godoc
// @Summary List all namespaces
// @Description List all namespaces
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Success 200 {object} RestGetAllNsResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns [get]
func RestGetAllNs(c echo.Context) error {

	var content RestGetAllNsResponse

	nsList, err := common.ListNs()
	if err != nil {
		//mapA := common.SimpleMsg{"Failed to list namespaces."}
		//return c.JSON(http.StatusNotFound, &mapA)
		return SendMessage(c, http.StatusOK, "Failed to list namespaces.")
	}

	if nsList == nil {
		//return c.JSON(http.StatusOK, &content)
		return Send(c, http.StatusOK, content)
	}

	// When err == nil && resourceList != nil
	content.Ns = nsList
	//return c.JSON(http.StatusOK, &content)
	return Send(c, http.StatusOK, content)

}

// RestGetNs godoc
// @Summary Get namespace
// @Description Get namespace
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId} [get]
func RestGetNs(c echo.Context) error {
	//id := c.Param("nsId")
	if err := Validate(c, []string{"nsId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	res, err := common.GetNs(c.Param("nsId"))
	if err != nil {
		//mapA := common.SimpleMsg{"Failed to find the namespace " + id}
		//return c.JSON(http.StatusNotFound, &mapA)
		return SendMessage(c, http.StatusOK, "Failed to find the namespace "+c.Param("nsId"))
	} else {
		//return c.JSON(http.StatusOK, &res)
		return Send(c, http.StatusOK, res)
	}
}

// RestPostNs godoc
// @Summary Create namespace
// @Description Create namespace
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param nsReq body common.NsReq true "Details for a new namespace"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns [post]
func RestPostNs(c echo.Context) error {

	u := &common.NsReq{}
	if err := c.Bind(u); err != nil {
		//return err
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	fmt.Println("[Creating Ns]")
	content, err := common.CreateNs(u)
	if err != nil {
		//common.CBLog.Error(err)
		////mapA := common.SimpleMsg{"Failed to create the ns " + u.Name}
		//mapA := common.SimpleMsg{err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}
	//return c.JSON(http.StatusCreated, content)
	return Send(c, http.StatusOK, content)

}

/* function RestPutNs not yet implemented
// RestPutNs godoc
// @Summary Update namespace
// @Description Update namespace
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param namespace body common.NsInfo true "Details to update existing namespace"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId} [put]
*/
func RestPutNs(c echo.Context) error {
	return nil
}
