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

// Package mcis is to handle REST API for mcis
package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestRecommendVm godoc
// @Summary Recommend MCIS plan (filter and priority)
// @Description Recommend MCIS plan (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234
// @Tags [Infra service] MCIS Provisioning management
// @Accept  json
// @Produce  json
// @Param deploymentPlan body mcis.DeploymentPlan false "Recommend MCIS plan (filter and priority)"
// @Success 200 {object} []mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /mcisRecommendVm [post]
func RestRecommendVm(c echo.Context) error {

	nsId := common.SystemCommonNs

	u := &mcis.DeploymentPlan{}
	if err := c.Bind(u); err != nil {
		return err
	}

	content, err := mcis.RecommendVm(nsId, *u)

	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	// result := RestFilterSpecsResponse{}
	// result.Spec = content
	return c.JSON(http.StatusOK, &content)
}

type RestPostMcisRecommendResponse struct {
	//VmReq          []TbVmRecommendReq    `json:"vmReq"`
	VmRecommend    []mcis.TbVmRecommendInfo `json:"vmRecommend"`
	PlacementAlgo  string                   `json:"placementAlgo"`
	PlacementParam []common.KeyValue        `json:"placementParam"`
}

// RestPostMcisRecommend godoc
// @Deprecated
func RestPostMcisRecommend(c echo.Context) error {
	// @Summary Get MCIS recommendation
	// @Description Get MCIS recommendation
	// @Tags [Infra service] MCIS Provisioning management
	// @Accept  json
	// @Produce  json
	// @Param nsId path string true "Namespace ID" default(ns01)
	// @Param mcisRecommendReq body mcis.McisRecommendReq true "Details for an MCIS object"
	// @Success 200 {object} RestPostMcisRecommendResponse
	// @Failure 404 {object} common.SimpleMsg
	// @Failure 500 {object} common.SimpleMsg
	// @Router /ns/{nsId}/mcis/recommend [post]
	nsId := c.Param("nsId")

	req := &mcis.McisRecommendReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CorePostMcisRecommend(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	content := RestPostMcisRecommendResponse{}
	content.VmRecommend = result
	content.PlacementAlgo = req.PlacementAlgo
	content.PlacementParam = req.PlacementParam

	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}
