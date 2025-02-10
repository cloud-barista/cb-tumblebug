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

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
)

// RestPostCustomImage godoc
// @ID PostCustomImage
// @Summary Register existing Custom Image in a CSP
// @Description Register existing Custom Image in a CSP (option=register)
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string true "Option: " Enums(register)
// @Param customImageRegisterReq body model.TbCustomImageReq true "Request to Register existing Custom Image in a CSP"
// @Success 200 {object} model.TbCustomImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/customImage [post]
func RestPostCustomImage(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	if optionFlag != "register" {
		err := fmt.Errorf("POST customImage can be called only with 'option=register'")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	u := &model.TbCustomImageReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.RegisterCustomImageWithId(nsId, u)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetCustomImage godoc
// @ID GetCustomImage
// @Summary Get customImage
// @Description Get customImage
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param customImageId path string true "customImage ID"
// @Success 200 {object} model.TbCustomImageInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/customImage/{customImageId} [get]
func RestGetCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllCustomImage
type RestGetAllCustomImageResponse struct {
	CustomImage []model.TbCustomImageInfo `json:"customImage"`
}

// RestGetAllCustomImage godoc
// @ID GetAllCustomImage
// @Summary List all customImages or customImages' ID
// @Description List all customImages or customImages' ID
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex:guestOS)"
// @Param filterVal query string false "Field value for filtering (ex: Ubuntu18.04)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllCustomImageResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/customImage [get]
func RestGetAllCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelCustomImage godoc
// @ID DelCustomImage
// @Summary Delete customImage
// @Description Delete customImage
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param customImageId path string true "customImage ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/customImage/{customImageId} [delete]
func RestDelCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllCustomImage godoc
// @ID DelAllCustomImage
// @Summary Delete all customImages
// @Description Delete all customImages
// @Tags [Infra Resource] Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/customImage [delete]
func RestDelAllCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
