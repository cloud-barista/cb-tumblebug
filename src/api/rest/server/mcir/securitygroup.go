package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostSecurityGroup godoc
// @Summary Create Security Group
// @Description Create Security Group
// @Tags Security Group
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param securityGroupReq body mcir.TbSecurityGroupReq true "Details for an securityGroup object"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [post]
func RestPostSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbSecurityGroupReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST SecurityGroup requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating SecurityGroup]")
			content, _ := CreateSecurityGroup(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else if action == "register" {
			fmt.Println("[Registering SecurityGroup]")
			content, _ := registerSecurityGroup(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST SecurityGroup")
	fmt.Println("[Creating SecurityGroup]")
	//content, responseCode, _, err := CreateSecurityGroup(nsId, u)
	content, err := mcir.CreateSecurityGroup(nsId, u)
	if err != nil {
		common.CBLog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SecurityGroup"}
		*/
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

/* function RestPutSecurityGroup not yet implemented
// RestPutSecurityGroup godoc
// @Summary Update Security Group
// @Description Update Security Group
// @Tags Security Group
// @Accept  json
// @Produce  json
// @Param securityGroupInfo body mcir.TbSecurityGroupInfo true "Details for an securityGroup object"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [put]
*/
func RestPutSecurityGroup(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetSecurityGroup godoc
// @Summary Get Security Group
// @Description Get Security Group
// @Tags Security Group
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param securityGroupId path string true "Security Group ID"
// @Success 200 {object} mcir.TbSecurityGroupInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [get]
func RestGetSecurityGroup(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")

		resourceType := "securityGroup"

		id := c.Param("securityGroupId")

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

// Response structure for RestGetAllSecurityGroup
type RestGetAllSecurityGroupResponse struct {
	SecurityGroup []mcir.TbSecurityGroupInfo `json:"securityGroup"`
}

// RestGetAllSecurityGroup godoc
// @Summary List all Security Groups
// @Description List all Security Groups
// @Tags Security Group
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllSecurityGroupResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [get]
func RestGetAllSecurityGroup(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")

		resourceType := "securityGroup"

		var content struct {
			SecurityGroup []TbSecurityGroupInfo `json:"securityGroup"`
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
		content.SecurityGroup = resourceList.([]TbSecurityGroupInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	*/
	return nil
}

// RestDelSecurityGroup godoc
// @Summary Delete Security Group
// @Description Delete Security Group
// @Tags Security Group
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param securityGroupId path string true "Security Group ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup/{securityGroupId} [delete]
func RestDelSecurityGroup(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")
		resourceType := "securityGroup"
		id := c.Param("securityGroupId")
		forceFlag := c.QueryParam("force")

		//responseCode, body, err := delSecurityGroup(nsId, id, forceFlag)

		err := DelResource(nsId, resourceType, id, forceFlag)
		if err != nil {
			common.CBLog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete the securityGroup"}
			//return c.JSONBlob(responseCode, body)
			return c.JSON(http.StatusFailedDependency, err)
		}

		mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/
	return nil
}

// RestDelAllSecurityGroup godoc
// @Summary Delete all Security Groups
// @Description Delete all Security Groups
// @Tags Security Group
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/securityGroup [delete]
func RestDelAllSecurityGroup(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")
		resourceType := "securityGroup"
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
