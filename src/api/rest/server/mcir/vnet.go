package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostVNet godoc
// @Summary Create VNet
// @Description Create VNet
// @Tags [MCIR] VNet management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param vNetReq body mcir.TbVNetReq true "Details for an VNet object"
// @Success 200 {object} mcir.TbVNetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [post]
func RestPostVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbVNetReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[POST VNet]")
	//fmt.Println("[Creating VNet]")
	//content, responseCode, body, err := CreateVNet(nsId, u)
	content, err := mcir.CreateVNet(nsId, u)
	if err != nil {
		common.CBLog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a vNet"}
		*/
		//return c.JSONBlob(responseCode, body)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

/* function RestPutVNet not yet implemented
// RestPutVNet godoc
// @Summary Update VNet
// @Description Update VNet
// @Tags [MCIR] VNet management
// @Accept  json
// @Produce  json
// @Param vNetInfo body mcir.TbVNetInfo true "Details for an VNet object"
// @Success 200 {object} mcir.TbVNetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [put]
*/
func RestPutVNet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetVNet godoc
// @Summary Get VNet
// @Description Get VNet
// @Tags [MCIR] VNet management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} mcir.TbVNetInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [get]
func RestGetVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	return nil
}

// Response structure for RestGetAllVNet
type RestGetAllVNetResponse struct {
	VNet []mcir.TbVNetInfo `json:"vNet"`
}

// RestGetAllVNet godoc
// @Summary List all VNets
// @Description List all VNets
// @Tags [MCIR] VNet management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllVNetResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [get]
func RestGetAllVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	return nil
}

// RestDelVNet godoc
// @Summary Delete VNet
// @Description Delete VNet
// @Tags [MCIR] VNet management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [delete]
func RestDelVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	return nil
}

// RestDelAllVNet godoc
// @Summary Delete all VNets
// @Description Delete all VNets
// @Tags [MCIR] VNet management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [delete]
func RestDelAllVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	return nil
}
