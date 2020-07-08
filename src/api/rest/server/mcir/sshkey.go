package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostSshKey godoc
// @Summary Create SSH Key
// @Description Create SSH Key
// @Tags SSH Key
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param sshKeyInfo body mcir.TbSshKeyReq true "Details for an SSH Key object"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [post]
func RestPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbSshKeyReq{}
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

/* function RestPutSshKey not yet implemented
// RestPutSshKey godoc
// @Summary Update SSH Key
// @Description Update SSH Key
// @Tags SSH Key
// @Accept  json
// @Produce  json
// @Param sshKeyInfo body mcir.TbSshKeyInfo true "Details for an SSH Key object"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [put]
*/
func RestPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetSshKey godoc
// @Summary Get SSH Key
// @Description Get SSH Key
// @Tags SSH Key
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param sshKeyId path string true "SSH Key ID"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [get]
func RestGetSshKey(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
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
	*/
	return nil
}

// Response struct for RestGetAllSshKey
type RestGetAllSshKeyResponse struct {
	SshKey []mcir.TbSshKeyInfo `json:"sshKey"`
}

// RestGetAllSshKey godoc
// @Summary List all SSH Keys
// @Description List all SSH Keys
// @Tags SSH Key
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllSshKeyResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [get]
func RestGetAllSshKey(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
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
	*/
	return nil
}

// RestDelSshKey godoc
// @Summary Delete SSH Key
// @Description Delete SSH Key
// @Tags SSH Key
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param sshKeyId path string true "SSH Key ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [delete]
func RestDelSshKey(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
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
	*/
	return nil
}

// RestDelAllSshKey godoc
// @Summary Delete all SSH Keys
// @Description Delete all SSH Keys
// @Tags SSH Key
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [delete]
func RestDelAllSshKey(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
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
	*/
	return nil
}
