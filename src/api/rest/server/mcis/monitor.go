package mcis

import (

	"net/http"


	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcis"
	"github.com/labstack/echo/v4"
)



// RestPostInstallMonitorAgentToMcis godoc
// @Summary InstallMonitorAgent MCIS
// @Description InstallMonitorAgent MCIS
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param nsId path string true "MCIS ID"
// @Param mcisInfo body mcis.McisCmdReq true "Details for an MCIS object"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/install/mcis/{mcisId} [post]
func RestPostInstallMonitorAgentToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.InstallMonitorAgentToMcis(nsId, mcisId, req)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}
