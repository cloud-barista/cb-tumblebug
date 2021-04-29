package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/beego/beego/v2/core/validation"
	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

type Existence struct {
	Exists bool `json:"exists"`
}

func SendExistence(c echo.Context, httpCode int, existence bool) error {
	return c.JSON(httpCode, Existence{Exists: existence})
}

type Status struct {
	Message string `json:"message"`
}

func SendMessage(c echo.Context, httpCode int, msg string) error {
	return c.JSON(httpCode, Status{Message: msg})
}

func Send(c echo.Context, httpCode int, json interface{}) error {
	return c.JSON(httpCode, json)
}

func Validate(c echo.Context, params []string) error {
	valid := validation.Validation{}

	for _, name := range params {
		valid.Required(c.Param(name), name)
	}

	if valid.HasErrors() {
		for _, err := range valid.Errors {
			return errors.New(fmt.Sprintf("[%s]%s", err.Key, err.Error()))
		}
	}
	return nil
}

func RestGetHealth(c echo.Context) error {
	return c.String(http.StatusOK, "The API server of CB-Tumblebug is alive.")
}

// RestGetConnConfig func is a rest api wrapper for GetConnConfig.
// RestGetConnConfig godoc
// @Summary Get registered ConnConfig info
// @Description Get registered ConnConfig info
// @Tags Admin
// @Accept  json
// @Produce  json
// @Success 200 {object} common.ConnConfig
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /connConfig/{connConfigName} [get]
func RestGetConnConfig(c echo.Context) error {

	connConfigName := c.Param("connConfigName")

	fmt.Println("[Get ConnConfig for name]" + connConfigName)
	content, err := common.GetConnConfig(connConfigName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}
	return c.JSON(http.StatusOK, &content)

}

// RestGetConnConfigList func is a rest api wrapper for GetConnConfigList.
// RestGetConnConfigList godoc
// @Summary List all registered ConnConfig
// @Description List all registered ConnConfig
// @Tags Admin
// @Accept  json
// @Produce  json
// @Success 200 {object} common.ConnConfigList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /connConfig [get]
func RestGetConnConfigList(c echo.Context) error {

	fmt.Println("[Get ConnConfig List]")
	content, err := common.GetConnConfigList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// RestGetRegion func is a rest api wrapper for GetRegion.
// RestGetRegion godoc
// @Summary Get registered region info
// @Description Get registered region info
// @Tags Admin
// @Accept  json
// @Produce  json
// @Param regionName path string true "Name of region to retrieve"
// @Success 200 {object} common.Region
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /region/{regionName} [get]
func RestGetRegion(c echo.Context) error {

	regionName := c.Param("regionName")

	fmt.Println("[Get Region for name]" + regionName)
	content, err := common.GetRegion(regionName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// RestGetRegionList func is a rest api wrapper for GetRegionList.
// RestGetRegionList godoc
// @Summary List all registered regions
// @Description List all registered regions
// @Tags Admin
// @Accept  json
// @Produce  json
// @Success 200 {object} common.RegionList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /region [get]
func RestGetRegionList(c echo.Context) error {

	fmt.Println("[Get Region List]")
	content, err := common.GetRegionList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// ObjectList struct consists of object IDs
type ObjectList struct {
	Object []string `json:"object"`
}

// func RestGetObjects is a rest api wrapper for GetObjectList.
// RestGetObjects godoc
// @Summary List all objects for a given key
// @Description List all objects for a given key
// @Tags Admin
// @Accept  json
// @Produce  json
// @Param key query string true "retrieve objects by key"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /objects [get]
func RestGetObjects(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Get Tumblebug Object List] with Key: %s \n", parentKey)

	content := common.GetObjectList(parentKey)

	objectList := ObjectList{}
	for i, v := range content {
		fmt.Printf("[Obj: %d] %s \n", i, v)
		objectList.Object = append(objectList.Object, v)
	}
	return c.JSON(http.StatusOK, &objectList)
}

// func RestGetObject is a rest api wrapper for GetObject.
// RestGetObject godoc
// @Summary Get value of an object
// @Description Get value of an object
// @Tags Admin
// @Accept  json
// @Produce  json
// @Param key query string true "get object value by key"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /object [get]
func RestGetObject(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Get Tumblebug Object Value] with Key: %s \n", parentKey)

	content, err := common.GetObjectValue(parentKey)
	if err != nil || content == "" {
		return SendMessage(c, http.StatusOK, "Cannot find ["+parentKey+"] object")
	}

	var contentJSON map[string]interface{}
	json.Unmarshal([]byte(content), &contentJSON)

	return c.JSON(http.StatusOK, &contentJSON)
}

// func RestDeleteObject is a rest api wrapper for DeleteObject.
// RestDeleteObject godoc
// @Summary Delete an object
// @Description Delete an object
// @Tags Admin
// @Accept  json
// @Produce  json
// @Param key query string true "delete object value by key"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /object [delete]
func RestDeleteObject(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Delete Tumblebug Object] with Key: %s \n", parentKey)

	content, err := common.GetObjectValue(parentKey)
	if err != nil || content == "" {
		return SendMessage(c, http.StatusOK, "Cannot find ["+parentKey+"] object")
	}

	err = common.DeleteObject(parentKey)
	if err != nil {
		return SendMessage(c, http.StatusOK, "Cannot delete ["+parentKey+"] object")
	}

	return SendMessage(c, http.StatusOK, "The object has been deleted")
}

// func RestDeleteObjects is a rest api wrapper for DeleteObjects.
// RestDeleteObjects godoc
// @Summary Delete child objects along with the given object
// @Description Delete child objects along with the given object
// @Tags Admin
// @Accept  json
// @Produce  json
// @Param key query string true "Delete child objects based on the given key string"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /objects [delete]
func RestDeleteObjects(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Delete Tumblebug child Objects] with Key: %s \n", parentKey)

	err := common.DeleteObjects(parentKey)
	if err != nil {
		return SendMessage(c, http.StatusOK, "Cannot delete  objects")
	}

	return SendMessage(c, http.StatusOK, "Objects have been deleted")
}
