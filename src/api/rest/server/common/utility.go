package common

import (
	"errors"
	"fmt"
	"net/http"
	"encoding/json"

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

func RestGetConnConfigList(c echo.Context) error {

	fmt.Println("[Get ConnConfig List]")
	content, err := common.GetConnConfigList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

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

// func RestGetObjectList is a rest api wrapper for GetObjectList.
// RestGetObjectList godoc
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
func RestGetObjectList(c echo.Context) error {
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

// func RestGetObjectValue is a rest api wrapper for GetObjectValue.
// RestGetObjectValue godoc
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
func RestGetObjectValue(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Get Tumblebug Object Value] with Key: %s \n", parentKey)

	content, err := common.GetObjectValue(parentKey)
	if err != nil || content == "" {
		return SendMessage(c, http.StatusOK, "Cannot find [" + parentKey+ "] object")
	}
	
	var contentJSON map[string]interface{}
	json.Unmarshal([]byte(content), &contentJSON)

	return c.JSON(http.StatusOK, &contentJSON)
}

// func RestDeleteObjectValue is a rest api wrapper for DeleteObject.
// RestDeleteObjectValue godoc
// @Summary Delete value of an object
// @Description Delete value of an object
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
		return SendMessage(c, http.StatusOK, "Cannot find [" + parentKey+ "] object")
	}

	err = common.DeleteObject(parentKey)
	if err != nil {
		return SendMessage(c, http.StatusOK, "Cannot delete [" + parentKey+ "] object")
	}

	return SendMessage(c, http.StatusOK, "The object has been deleted")
}

// func RestDeleteObjects is a rest api wrapper for DeleteObjects.
// RestDeleteObjects godoc
// @Summary Delete objects
// @Description Delete objects
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