package common

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/astaxie/beego/validation"
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

func Healthy(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World! This is Cloud-Barista CB-Tumblebug")
}

func RestGetConnConfig(c echo.Context) error {

	connConfigName := c.Param("connConfigName")

	fmt.Println("[Get ConnConfig for name]" + connConfigName)
	content, err := common.GetConnConfig(connConfigName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusFailedDependency, []byte(err.Error()))
	}
	return c.JSON(http.StatusOK, &content)

}

func RestGetConnConfigList(c echo.Context) error {

	fmt.Println("[Get ConnConfig List]")
	content, err := common.GetConnConfigList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusFailedDependency, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

func RestGetRegion(c echo.Context) error {

	regionName := c.Param("regionName")

	fmt.Println("[Get Region for name]" + regionName)
	content, err := common.GetRegion(regionName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusFailedDependency, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

func RestGetRegionList(c echo.Context) error {

	fmt.Println("[Get Region List]")
	content, err := common.GetRegionList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusFailedDependency, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}
