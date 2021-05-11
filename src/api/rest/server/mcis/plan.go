package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestRecommendVm godoc
// @Summary RestRecommendVm specs by range
// @Description RestRecommendVm specs by range
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param deploymentPlan body mcis.DeploymentPlan false "RestRecommendVm for range-filtering specs"
// @Success 200 {object} []mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/testRecommendVm [post]
func RestRecommendVm(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcis.DeploymentPlan{}
	if err := c.Bind(u); err != nil {
		return err
	}

	content, err := mcis.RecommendVm(nsId, *u)

	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	// result := RestFilterSpecsResponse{}
	// result.Spec = content
	return c.JSON(http.StatusOK, &content)
}
