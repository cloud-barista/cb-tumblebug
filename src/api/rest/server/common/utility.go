package common

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

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
