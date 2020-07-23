package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostImage godoc
// @Summary Register image
// @Description Register image
// @Tags Image
// @Accept  json
// @Produce  json
// @Param registeringMethod query string true "registerWithInfo or registerWithId"
// @Param nsId path string true "Namespace ID"
// @Param imageInfo body mcir.TbImageInfo false "Details for an image object"
// @Param imageId body mcir.TbImageReq false "name, connectionName and cspImageId"
// @Success 200 {object} mcir.TbImageInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/image [post]
func RestPostImage(c echo.Context) error {

	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	fmt.Println("[POST Image requested action: " + action)
	/*
		if action == "create" {
			fmt.Println("[Creating Image]")
			content, _ := createImage(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else */
	if action == "registerWithInfo" {
		fmt.Println("[Registering Image with info]")
		u := &mcir.TbImageInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := mcir.RegisterImageWithInfo(nsId, u)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)
	} else if action == "registerWithId" {
		fmt.Println("[Registering Image with ID]")
		u := &mcir.TbImageReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		//content, responseCode, body, err := RegisterImageWithId(nsId, u)
		content, err := mcir.RegisterImageWithId(nsId, u)
		if err != nil {
			common.CBLog.Error(err)
			//fmt.Println("body: ", string(body))
			//return c.JSONBlob(responseCode, body)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)
	} else {
		mapA := map[string]string{"message": "You must specify: action=registerWithInfo or action=registerWithId"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

/* function RestPutImage not yet implemented
// RestPutImage godoc
// @Summary Update image
// @Description Update image
// @Tags Image
// @Accept  json
// @Produce  json
// @Param imageInfo body mcir.TbImageInfo true "Details for an image object"
// @Success 200 {object} mcir.TbImageInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [put]
*/
func RestPutImage(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetImage godoc
// @Summary Get image
// @Description Get image
// @Tags Image
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param imageId path string true "Image ID"
// @Success 200 {object} mcir.TbImageInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [get]
func RestGetImage(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")

		resourceType := "image"

		id := c.Param("imageId")

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

// Response structure for RestGetAllImage
type RestGetAllImageResponse struct {
	Image []mcir.TbImageInfo `json:"image"`
}

// RestGetAllImage godoc
// @Summary List all images
// @Description List all images
// @Tags Image
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllImageResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/image [get]
func RestGetAllImage(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")

		resourceType := "image"

		var content struct {
			Image []TbImageInfo `json:"image"`
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
		content.Image = resourceList.([]TbImageInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	*/
	return nil
}

// RestDelImage godoc
// @Summary Delete image
// @Description Delete image
// @Tags Image
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param imageId path string true "Image ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [delete]
func RestDelImage(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")
		resourceType := "image"
		id := c.Param("imageId")
		forceFlag := c.QueryParam("force")

		err := DelResource(nsId, resourceType, id, forceFlag)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/
	return nil
}

// RestDelAllImage godoc
// @Summary Delete all images
// @Description Delete all images
// @Tags Image
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/image [delete]
func RestDelAllImage(c echo.Context) error {
	// Obsolete function. This is just for Swagger.
	/*
		nsId := c.Param("nsId")
		resourceType := "image"
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
