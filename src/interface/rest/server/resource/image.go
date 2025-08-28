/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package resource is to handle REST API for resource
package resource

import (
	"fmt"
	"strconv"
	"strings"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostImage godoc
// @ID PostImage
// @Summary Register image
// @Description Register image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param action query string true "registeringMethod" Enums(registerWithInfo, registerWithId)
// @Param nsId path string true "Namespace ID" default(system)
// @Param imageInfo body model.ImageInfo false "Specify details of a image object (cspResourceName, guestOS, description, ...) manually"
// @Param imageReq body model.ImageReq false "Specify (name, connectionName, cspImageName) to register an image object automatically"
// @Param update query boolean false "Force update to existing image object" default(false)
// @Success 200 {object} model.ImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image [post]
func RestPostImage(c echo.Context) error {

	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	updateStr := c.QueryParam("update")
	update, err := strconv.ParseBool(updateStr)
	if err != nil {
		update = false
	}

	log.Debug().Msg("[POST Image] (action: " + action + ")")
	/*
		if action == "create" {
			log.Debug().Msg("[Creating Image]")
			content, _ := createImage(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else */
	if action == "registerWithInfo" {
		log.Debug().Msg("[Registering Image with info]")
		u := &model.ImageInfo{}
		if err := c.Bind(u); err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content, err := resource.RegisterImageWithInfo(nsId, u, update)
		return clientManager.EndRequestWithLog(c, err, content)
	} else if action == "registerWithId" {
		log.Debug().Msg("[Registering Image with ID]")
		u := &model.ImageReq{}
		if err := c.Bind(u); err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content, err := resource.RegisterImageWithId(nsId, u, update, false)
		return clientManager.EndRequestWithLog(c, err, content)
	} else {
		err := fmt.Errorf("You must specify: action=registerWithInfo or action=registerWithId")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

}

// RestPutImage godoc
// @ID PutImage
// @Summary Update image
// @Description Update image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param imageInfo body model.ImageInfo true "Details for an image object"
// @Param nsId path string true "Namespace ID" default(system)
// @Param imageId path string true "Image ID ({providerName}+{regionName}+{cspImageName})"
// @Success 200 {object} model.ImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [put]
func RestPutImage(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceId := c.Param("imageId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	u := &model.ImageInfo{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.UpdateImage(nsId, resourceId, *u, false)

	return clientManager.EndRequestWithLog(c, err, content)
}

// Request structure for RestLookupImage
type RestLookupImageRequest struct {
	ConnectionName string `json:"connectionName"`
	CspImageName   string `json:"cspImageName"`
}

// RestLookupImage godoc
// @ID LookupImage
// @Summary Lookup image (for debugging purposes)
// @Description Lookup image (for debugging purposes)
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param lookupImageReq body RestLookupImageRequest true "Specify connectionName, cspImageName"
// @Success 200 {object} model.SpiderImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupImage [post]
func RestLookupImage(c echo.Context) error {

	u := &RestLookupImageRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msg("[Lookup image]: " + u.CspImageName)
	content, err := resource.LookupImage(u.ConnectionName, u.CspImageName)
	return clientManager.EndRequestWithLog(c, err, content)

}

// RestLookupImageList godoc
// @ID LookupImageList
// @Summary Lookup image list (for debugging purposes)
// @Description Lookup image list (for debugging purposes)
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param lookupImagesReq body common.TbConnectionName true "Specify connectionName"
// @Success 200 {object} model.SpiderImageList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupImages [post]
func RestLookupImageList(c echo.Context) error {

	u := &RestLookupImageRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msg("[Lookup images]")
	content, err := resource.LookupImageList(u.ConnectionName)
	return clientManager.EndRequestWithLog(c, err, content)

}

// RestFetchImages godoc
// @ID FetchImages
// @Summary Fetch images for regions of each CSP synchronously
// @Description Fetch images waiting for completion
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param fetchOption body model.ImageFetchOption true "Fetch option"
// @Success 202 {object} resource.FetchImagesAsyncResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /fetchImages [post]
func RestFetchImages(c echo.Context) error {
	nsId := model.SystemCommonNs

	reqBody := &model.ImageFetchOption{}
	if err := c.Bind(reqBody); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.FetchImagesForAllConnConfigs(nsId, reqBody)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, err, content)
}

// RestFetchImagesAsync godoc
// @ID FetchImagesAsync
// @Summary Fetch images asynchronously
// @Description Fetch images in the background without waiting for completion
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param fetchOption body model.ImageFetchOption true "Fetch option"
// @Success 202 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /fetchImagesAsync [post]
func RestFetchImagesAsync(c echo.Context) error {
	nsId := model.SystemCommonNs

	reqBody := &model.ImageFetchOption{}
	if err := c.Bind(reqBody); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	err := resource.FetchImagesForAllConnConfigsAsync(nsId, reqBody)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content := map[string]string{
		"message": "Started fetching images in the background. Check server logs for progress."}

	return c.JSON(202, content)
}

// RestGetFetchImagesAsyncResult godoc
// @ID GetFetchImagesAsyncResult
// @Summary Get result of asynchronous image fetching
// @Description Get detailed results from the last asynchronous image fetch operation
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Success 200 {object} resource.FetchImagesAsyncResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /fetchImagesResult [get]
func RestGetFetchImagesAsyncResult(c echo.Context) error {
	nsId := model.SystemCommonNs

	result, err := resource.GetFetchImagesAsyncResult(nsId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestUpdateImagesFromAsset godoc
// @ID UpdateImagesFromAsset
// @Summary Update images from cloudimage.csv asset file
// @Description Update image information based on the cloudimage.csv asset file
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Success 202 {object} resource.FetchImagesAsyncResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /updateImagesFromAsset [post]
func RestUpdateImagesFromAsset(c echo.Context) error {

	nsId := model.SystemCommonNs
	result, err := resource.UpdateImagesFromAsset(nsId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestGetImage godoc
// @ID GetImage
// @Summary Get image
// @Description GetImage returns an image object if there are matched images for the given namespace and imageKey(Id, CspResourceName)
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param imageId path string true "(Note: imageId param will be refined in next release, enabled for temporal support) This param accepts vaious input types as Image Key: cspImageName"
// @Success 200 {object} model.ImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [get]
func RestGetImage(c echo.Context) error {
	nsId := c.Param("nsId")
	imageKey := c.Param("imageId")
	imageKey = strings.ReplaceAll(imageKey, " ", "+")
	imageKey = strings.ReplaceAll(imageKey, "%2B", "+")

	content, err := resource.GetImage(nsId, imageKey)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetAllImage godoc
// @ID GetAllImage
// @Summary List all images or images' ID
// @Description List all images or images' ID
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex:guestOS)"
// @Param filterVal query string false "Field value for filtering (ex: Ubuntu18.04)"
// @Success 200 {object} JSONResult{[DEFAULT]=model.SearchImageResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image [get]
func RestGetAllImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelImage godoc
// @ID DelImage
// @Summary Delete image
// @Description Delete image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param imageId path string true "Image ID ({providerName}+{regionName}+{cspImageName})"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [delete]
func RestDelImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllImage godoc
// @ID DelAllImage
// @Summary Delete all images
// @Description Delete all images
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image [delete]
func RestDelAllImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestSearchImage godoc
// @ID SearchImage
// @Summary Search image
// @Description Search image
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param condition body model.SearchImageRequest true "condition"
// @Success 200 {object} model.SearchImageResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/searchImage [post]
func RestSearchImage(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &model.SearchImageRequest{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, cnt, err := resource.SearchImage(nsId, *u)

	result := model.SearchImageResponse{}
	result.ImageCount = cnt
	result.ImageList = content
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestSearchImageOptions godoc
// @ID SearchImageOptions
// @Get available image search request options
// @Description Get all available options for image search fields
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Success 200 {object} model.SearchImageRequestOptions
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/searchImageOptions [get]
func RestSearchImageOptions(c echo.Context) error {

	content, err := resource.SearchImageOptions()
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, nil, content)
}
