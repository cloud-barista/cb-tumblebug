package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
	"github.com/labstack/echo/v4"
)

// MCIS API Proxy: VNet
func RestPostVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbVNetInfo{}
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

func RestPutVNet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

/*
func RestGetVNet(c echo.Context) error {

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
}
*/

/*
func RestGetAllVNet(c echo.Context) error {

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
}
*/

/*
func RestDelVNet(c echo.Context) error {

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
}
*/

/*
func RestDelAllVNet(c echo.Context) error {

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
}
*/
