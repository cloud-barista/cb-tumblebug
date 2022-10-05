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
// @Summary Create Custom Image
// @Description Create Custom Image
// @Tags [Infra resource] MCIR Custom Image management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option: " Enums(register)
// @Param customImageInfo body mcir.TbCustomImageReq true "Details for an Custom Image object"
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
