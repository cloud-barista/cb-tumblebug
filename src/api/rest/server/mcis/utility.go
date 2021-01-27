package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

func RestCheckMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	//exists, _, err := mcis.LowerizeAndCheckMcis(nsId, mcisId)
	exists, err := mcis.CheckMcis(nsId, mcisId)

	type JsonTemplate struct {
		Exists bool `json:"exists"`
	}
	content := JsonTemplate{}
	content.Exists = exists

	if err != nil {
		common.CBLog.Error(err)
		//mapA := map[string]string{"message": err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return c.JSON(http.StatusNotFound, &content)
	}

	return c.JSON(http.StatusOK, &content)
}

func RestCheckVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	//exists, _, err := mcis.LowerizeAndCheckVm(nsId, mcisId, vmId)
	exists, err := mcis.CheckVm(nsId, mcisId, vmId)

	type JsonTemplate struct {
		Exists bool `json:"exists"`
	}
	content := JsonTemplate{}
	content.Exists = exists

	if err != nil {
		common.CBLog.Error(err)
		//mapA := map[string]string{"message": err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return c.JSON(http.StatusNotFound, &content)
	}

	return c.JSON(http.StatusOK, &content)
}
