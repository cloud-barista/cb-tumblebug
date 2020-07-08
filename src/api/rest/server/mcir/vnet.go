package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostVNet godoc
// @Summary Create VNet
// @Description Create VNet
// @Tags VNet
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

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST VNet requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating VNet]")
			content, _ := CreateVNet(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else if action == "register" {
			fmt.Println("[Registering VNet]")
			content, _ := registerVNet(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST VNet")
	fmt.Println("[Creating VNet]")
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
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

/* function RestPutVNet not yet implemented
// RestPutVNet godoc
// @Summary Update VNet
// @Description Update VNet
// @Tags VNet
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
// @Tags VNet
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
	/*
		nsId := c.Param("nsId")

		resourceType := "vNet"

		id := c.Param("vNetId")

		res, err := GetResource(nsId, resourceType, id)
		if err != nil {
			mapA := map[string]string{"message": "Failed to find " + resourceType + " " + id}
			return c.JSON(http.StatusNotFound, &mapA)
		} else {
			return c.JSON(http.StatusOK, &res)
		}
	*/
	return nil
}

// Response structure for RestGetAllVNet
type RestGetAllVNetResponse struct {
	VNet []mcir.TbVNetInfo `json:"vNet"`
}

// RestGetAllVNet godoc
// @Summary List all VNets
// @Description List all VNets
// @Tags VNet
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllVNetResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [get]
func RestGetAllVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")

		resourceType := "vNet"

		var content struct {
			VNet []TbVNetInfo `json:"vNet"`
		}

		resourceList, err := ListResource(nsId, resourceType)
		if err != nil {
			mapA := map[string]string{"message": "Failed to list " + resourceType + "s."}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		if resourceList == nil {
			return c.JSON(http.StatusOK, &content)
		}

		// When err == nil && resourceList != nil
		content.VNet = resourceList.([]TbVNetInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	*/
	return nil
}

// RestDelVNet godoc
// @Summary Delete VNet
// @Description Delete VNet
// @Tags VNet
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param vNetId path string true "VNet ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet/{vNetId} [delete]
func RestDelVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")
		resourceType := "vNet"
		id := c.Param("vNetId")
		forceFlag := c.QueryParam("force")

		//responseCode, body, err := delVNet(nsId, id, forceFlag)

		err := DelResource(nsId, resourceType, id, forceFlag)
		if err != nil {
			common.CBLog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete the vNet"}
			//return c.JSONBlob(responseCode, body)
			return c.JSON(http.StatusFailedDependency, err)
		}

		mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/
	return nil
}

// RestDelAllVNet godoc
// @Summary Delete all VNets
// @Description Delete all VNets
// @Tags VNet
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/vNet [delete]
func RestDelAllVNet(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")
		resourceType := "vNet"
		forceFlag := c.QueryParam("force")

		err := DelAllResources(nsId, resourceType, forceFlag)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusConflict, &mapA)
		}

		mapA := map[string]string{"message": "All " + resourceType + "s has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/
	return nil
}
