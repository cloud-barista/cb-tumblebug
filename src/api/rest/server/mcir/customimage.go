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

// Package mcir is to handle REST API for mcir
package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostCustomImage godoc
// @Summary Register existing Custom Image in a CSP
// @Description Register existing Custom Image in a CSP (option=register)
// @Tags [Infra resource] Snapshot and Custom Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string true "Option: " Enums(register)
// @Param customImageRegisterReq body mcir.TbCustomImageReq true "Request to Register existing Custom Image in a CSP"
// @Success 200 {object} mcir.TbCustomImageInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/customImage [post]
func RestPostCustomImage(c echo.Context) error {
	fmt.Println("[POST CustomImage]")

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	if optionFlag != "register" {
		mapA := map[string]string{"message": "POST customImage can be called only with 'option=register'"}
		return c.JSON(http.StatusNotAcceptable, &mapA)
	}

	u := &mcir.TbCustomImageReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	content, err := mcir.RegisterCustomImageWithId(nsId, u)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

// RestGetCustomImage godoc
// @Summary Get customImage
// @Description Get customImage
// @Tags [Infra resource] Snapshot and Custom Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param customImageId path string true "customImage ID"
// @Success 200 {object} mcir.TbCustomImageInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/customImage/{customImageId} [get]
func RestGetCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response structure for RestGetAllCustomImage
type RestGetAllCustomImageResponse struct {
	CustomImage []mcir.TbCustomImageInfo `json:"customImage"`
}

// RestGetAllCustomImage godoc
// @Summary List all customImages or customImages' ID
// @Description List all customImages or customImages' ID
// @Tags [Infra resource] Snapshot and Custom Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex:guestOS)"
// @Param filterVal query string false "Field value for filtering (ex: Ubuntu18.04)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllCustomImageResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/customImage [get]
func RestGetAllCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelCustomImage godoc
// @Summary Delete customImage
// @Description Delete customImage
// @Tags [Infra resource] Snapshot and Custom Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param customImageId path string true "customImage ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/customImage/{customImageId} [delete]
func RestDelCustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllCustomImage godoc
// @Summary Delete all customImages
// @Description Delete all customImages
// @Tags [Infra resource] Snapshot and Custom Image Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/customImage [delete]
func RestDelAllCustomcustomImage(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
