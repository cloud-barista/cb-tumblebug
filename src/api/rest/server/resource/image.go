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
	"net/http"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
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
// @Param imageInfo body model.TbImageInfo false "Specify details of a image object (cspResourceName, guestOS, description, ...) manually"
// @Param imageId body model.TbImageReq false "Specify name, connectionName and cspResourceId to register an image object automatically"
// @Param update query boolean false "Force update to existing image object" default(false)
// @Success 200 {object} model.TbImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image [post]
func RestPostImage(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
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
		u := &model.TbImageInfo{}
		if err := c.Bind(u); err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content, err := resource.RegisterImageWithInfo(nsId, u, update)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else if action == "registerWithId" {
		log.Debug().Msg("[Registering Image with ID]")
		u := &model.TbImageReq{}
		if err := c.Bind(u); err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
		content, err := resource.RegisterImageWithId(nsId, u, update, false)
		return common.EndRequestWithLog(c, reqID, err, content)
	} else {
		err := fmt.Errorf("You must specify: action=registerWithInfo or action=registerWithId")
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

}

// RestPutImage godoc
// @ID PutImage
// @Summary Update image
// @Description Update image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param imageInfo body model.TbImageInfo true "Details for an image object"
// @Param nsId path string true "Namespace ID" default(default)
// @Param imageId path string true "Image ID ({providerName}+{regionName}+{imageName})"
// @Success 200 {object} model.TbImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [put]
func RestPutImage(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	resourceId := c.Param("imageId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	u := &model.TbImageInfo{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := resource.UpdateImage(nsId, resourceId, *u, false)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// Request structure for RestLookupImage
type RestLookupImageRequest struct {
	ConnectionName string `json:"connectionName"`
	CspResourceId  string `json:"cspResourceId"`
}

// RestLookupImage godoc
// @ID LookupImage
// @Summary Lookup image
// @Description Lookup image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param lookupImageReq body RestLookupImageRequest true "Specify connectionName & cspResourceId"
// @Success 200 {object} model.SpiderImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupImage [post]
func RestLookupImage(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestLookupImageRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	log.Debug().Msg("[Lookup image]: " + u.CspResourceId)
	content, err := resource.LookupImage(u.ConnectionName, u.CspResourceId)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestLookupImageList godoc
// @ID LookupImageList
// @Summary Lookup image list
// @Description Lookup image list
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param lookupImagesReq body common.TbConnectionName true "Specify connectionName"
// @Success 200 {object} model.SpiderImageList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /lookupImages [post]
func RestLookupImageList(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestLookupImageRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	log.Debug().Msg("[Lookup images]")
	content, err := resource.LookupImageList(u.ConnectionName)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestFetchImages godoc
// @ID FetchImages
// @Summary Fetch images
// @Description Fetch images
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/fetchImages [post]
func RestFetchImages(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")

	u := &RestLookupImageRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	var connConfigCount, imageCount uint
	var err error

	if u.ConnectionName == "" {
		connConfigCount, imageCount, err = resource.FetchImagesForAllConnConfigs(nsId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
	} else {
		connConfigCount = 1
		imageCount, err = resource.FetchImagesForConnConfig(u.ConnectionName, nsId)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, err, nil)
		}
	}

	content := map[string]string{
		"message": "Fetched " + fmt.Sprint(imageCount) + " images (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetImage godoc
// @ID GetImage
// @Summary Get image
// @Description Get image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param imageId path string true "Image ID ({providerName}+{regionName}+{imageName})"
// @Success 200 {object} model.TbImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image/{imageId} [get]
func RestGetImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllImage
type RestGetAllImageResponse struct {
	Image []model.TbImageInfo `json:"image"`
}

// RestGetAllImage godoc
// @ID GetAllImage
// @Summary List all images or images' ID
// @Description List all images or images' ID
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex:guestOS)"
// @Param filterVal query string false "Field value for filtering (ex: Ubuntu18.04)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllImageResponse,[ID]=model.IdList} "Different return structures by the given option param"
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
// @Param nsId path string true "Namespace ID" default(default)
// @Param imageId path string true "Image ID ({providerName}+{regionName}+{imageName})"
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
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/image [delete]
func RestDelAllImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestSearchImage
type RestSearchImageRequest struct {
	Keywords []string `json:"keywords"`
}

// RestSearchImage godoc
// @ID SearchImage
// @Summary Search image
// @Description Search image
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param keywords body RestSearchImageRequest true "Keywords"
// @Success 200 {object} RestGetAllImageResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/searchImage [post]
func RestSearchImage(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")

	u := &RestSearchImageRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	content, err := resource.SearchImage(nsId, u.Keywords...)
	result := RestGetAllImageResponse{}
	result.Image = content
	return common.EndRequestWithLog(c, reqID, err, result)
}
