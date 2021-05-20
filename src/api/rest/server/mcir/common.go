package mcir

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
)

func RestDelAllResources(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	forceFlag := c.QueryParam("force")

	err := mcir.DelAllResources(nsId, resourceType, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	mapA := map[string]string{"message": "All " + resourceType + "s has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelResource(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	resourceId := c.Param("resourceId")

	forceFlag := c.QueryParam("force")

	err := mcir.DelResource(nsId, resourceType, resourceId, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + resourceId + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestGetAllResources(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	resourceList, err := mcir.ListResource(nsId, resourceType)
	if err != nil {
		mapA := map[string]string{"message": "Failed to list " + resourceType + "s."}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	switch resourceType {
	case common.StrImage:
		var content struct {
			Image []mcir.TbImageInfo `json:"image"`
		}

		if resourceList == nil {
			return c.JSON(http.StatusOK, &content)
		}

		// When err == nil && resourceList != nil
		content.Image = resourceList.([]mcir.TbImageInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	case common.StrSecurityGroup:
		var content struct {
			SecurityGroup []mcir.TbSecurityGroupInfo `json:"securityGroup"`
		}

		if resourceList == nil {
			return c.JSON(http.StatusOK, &content)
		}

		// When err == nil && resourceList != nil
		content.SecurityGroup = resourceList.([]mcir.TbSecurityGroupInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	case common.StrSpec:
		var content struct {
			Spec []mcir.TbSpecInfo `json:"spec"`
		}

		if resourceList == nil {
			return c.JSON(http.StatusOK, &content)
		}

		// When err == nil && resourceList != nil
		content.Spec = resourceList.([]mcir.TbSpecInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	case common.StrSSHKey:
		var content struct {
			SshKey []mcir.TbSshKeyInfo `json:"sshKey"`
		}

		if resourceList == nil {
			return c.JSON(http.StatusOK, &content)
		}

		// When err == nil && resourceList != nil
		content.SshKey = resourceList.([]mcir.TbSshKeyInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	case common.StrVNet:
		var content struct {
			VNet []mcir.TbVNetInfo `json:"vNet"`
		}

		if resourceList == nil {
			return c.JSON(http.StatusOK, &content)
		}

		// When err == nil && resourceList != nil
		content.VNet = resourceList.([]mcir.TbVNetInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	default:
		return c.JSON(http.StatusOK, nil)

	}
	return c.JSON(http.StatusOK, nil)
}

func RestGetResource(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	resourceId := c.Param("resourceId")

	res, err := mcir.GetResource(nsId, resourceType, resourceId)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find " + resourceType + " " + resourceId}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

func RestCheckResource(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")

	exists, err := mcir.CheckResource(nsId, resourceType, resourceId)

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

// RestTestAddObjectAssociation is a REST API call handling function
// to test "mcir.UpdateAssociatedObjectList" function with "add" argument.
func RestTestAddObjectAssociation(c echo.Context) error {

	nsId := c.Param("nsId")
	//resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/testAddObjectAssociation/:resourceType/:resourceId
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")

	vmKeyList, err := mcir.UpdateAssociatedObjectList(nsId, resourceType, resourceId, common.StrAdd, "/test/vm/key")

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	//mapA := map[string]int8{"inUseCount": inUseCount}
	return c.JSON(http.StatusOK, vmKeyList)
}

// RestTestDeleteObjectAssociation is a REST API call handling function
// to test "mcir.UpdateAssociatedObjectList" function with "delete" argument.
func RestTestDeleteObjectAssociation(c echo.Context) error {

	nsId := c.Param("nsId")
	//resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/testDeleteObjectAssociation/:resourceType/:resourceId
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")

	vmKeyList, err := mcir.UpdateAssociatedObjectList(nsId, resourceType, resourceId, common.StrDelete, "/test/vm/key")

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	//mapA := map[string]int8{"inUseCount": inUseCount}
	return c.JSON(http.StatusOK, vmKeyList)
}

// RestTestGetAssociatedObjectCount is a REST API call handling function
// to test "mcir.GetAssociatedObjectCount" function.
func RestTestGetAssociatedObjectCount(c echo.Context) error {

	nsId := c.Param("nsId")
	//resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/testGetAssociatedObjectCount/:resourceType/:resourceId
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")

	associatedObjectCount, err := mcir.GetAssociatedObjectCount(nsId, resourceType, resourceId)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	mapA := map[string]int{"associatedObjectCount": associatedObjectCount}
	return c.JSON(http.StatusOK, &mapA)
}
