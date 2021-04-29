package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostInstallMonitorAgentToMcis godoc
// @Summary InstallMonitorAgent MCIS
// @Description InstallMonitorAgent MCIS
// @Tags [MCIS] Resource monitor
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param mcisInfo body mcis.McisCmdReq true "Details for an MCIS object"
// @Success 200 {object} mcis.AgentInstallContentWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/monitoring/install/mcis/{mcisId} [post]
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

// RestGetMonitorData godoc
// @Summary GetMonitorData MCIS
// @Description GetMonitorData MCIS
// @Tags [MCIS] Resource monitor
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param metric path string true "Metric type: cpu, memory, disk, network"
// @Success 200 {object} mcis.MonResultSimpleResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/monitoring/mcis/{mcisId}/metric/{metric} [get]
func RestGetMonitorData(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	metric := c.Param("metric")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.GetMonitoringData(nsId, mcisId, metric)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}
