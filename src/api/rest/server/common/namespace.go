package common

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)


func RestCheckNs(c echo.Context) error {

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
}

// RestDelAllNs godoc
// @Summary delete all RestDelAllNs
// @Description delete by json RestDelAllNs
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns [delete]
func RestDelAllNs(c echo.Context) error {

	err := common.DelAllNs()
	if err != nil {
		common.CBLog.Error(err)
		mapA := common.SimpleMsg{err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	mapA := common.SimpleMsg{"All namespaces has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

// RestDelNs godoc
// @Summary RestDelNs namespace
// @Description Delete namespace by json RestDelNs
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId} [delete]
func RestDelNs(c echo.Context) error {

	id := c.Param("nsId")

	err := common.DelNs(id)
	if err != nil {
		common.CBLog.Error(err)
		mapA :=  common.SimpleMsg{err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := common.SimpleMsg{"The ns has been deleted"} 
	return c.JSON(http.StatusOK, &mapA)
}


// Response structure for RestGetAllNs
type RestGetAllNsResponse struct {
	//Name string     `json:"name"`
	Ns []common.NsInfo `json:"ns"`
}

// RestGetAllNs godoc
// @Summary RestGetAllNs namespace
// @Description list namespace by json RestGetAllNs
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
		mapA := common.SimpleMsg{"Failed to list namespaces."}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	if nsList == nil {
		return c.JSON(http.StatusOK, &content)
	}

	// When err == nil && resourceList != nil
	content.Ns = nsList
	return c.JSON(http.StatusOK, &content)

}

// RestGetNs godoc
// @Summary RestGetNs namespace
// @Description Get namespace by json RestGetNs
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId} [get]
func RestGetNs(c echo.Context) error {
	id := c.Param("nsId")

	res, err := common.GetNs(id)
	if err != nil {
		mapA := common.SimpleMsg{"Failed to find the namespace " + id}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

// PostNs godoc
// @Summary Create namespace
// @Description Create namespace by json RestPostNs
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param namespace body common.NsInfo true "Post Ns"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns [post]
func RestPostNs(c echo.Context) error {

	u := &common.NsInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Creating Ns]")
	content, err := common.CreateNs(u)
	if err != nil {
		common.CBLog.Error(err)
		//mapA := common.SimpleMsg{"Failed to create the ns " + u.Name}
		mapA := common.SimpleMsg{err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)

}

// PutNs godoc
// @Summary Update namespace
// @Description Update namespace by json RestPutNs
// @Tags Namespace
// @Accept  json
// @Produce  json
// @Param namespace body common.NsInfo true "put Ns"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns [put]
func RestPutNs(c echo.Context) error {
	return nil
}
