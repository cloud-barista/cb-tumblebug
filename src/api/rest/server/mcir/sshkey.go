package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
	"github.com/labstack/echo/v4"
)

// MCIS API Proxy: SshKey
func RestPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbSshKeyInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST SshKey requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating SshKey]")
			content, _ := CreateSshKey(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else if action == "register" {
			fmt.Println("[Registering SshKey]")
			content, _ := registerSshKey(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST SshKey")
	fmt.Println("[Creating SshKey]")
	//content, responseCode, _, err := CreateSshKey(nsId, u)
	content, err := mcir.CreateSshKey(nsId, u)
	if err != nil {
		common.CBLog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SshKey"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		*/
		//return c.JSON(res.StatusCode, res)
		//body, _ := ioutil.ReadAll(res.Body)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

/*
func RestGetSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "sshKey"

	id := c.Param("sshKeyId")

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
func RestGetAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "sshKey"

	var content struct {
		SshKey []TbSshKeyInfo `json:"sshKey"`
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
	content.SshKey = resourceList.([]TbSshKeyInfo) // type assertion (interface{} -> array)
	return c.JSON(http.StatusOK, &content)
}
*/

/*
func RestDelSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "sshKey"
	id := c.Param("sshKeyId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delSshKey(nsId, id, forceFlag)

	err := DelResource(nsId, resourceType, id, forceFlag)
	if err != nil {
		common.CBLog.Error(err)

		//mapA := map[string]string{"message": "Failed to delete the sshKey"}
		//return c.JSON(http.StatusFailedDependency, &mapA)

		//return c.JSONBlob(responseCode, body)
		return c.JSON(http.StatusFailedDependency, err)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
	//return c.JSON(http.StatusOK, body)
}
*/

/*
func RestDelAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "sshKey"
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
