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

// Package infra is to handle REST API for infra
package infra

import (
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestRecommendSpec godoc
// @ID RecommendSpec
// @Summary Recommend specs for configuring an infrastructure (filter and priority)
// @Description Recommend specs for configuring an infrastructure (filter and priority)
// @Description Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234
// @Description Get available options by /recommendSpecOptions for filtering and prioritizing specs in RecommendSpec API
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param recommendSpecReq body model.RecommendSpecReq false "Conditions for recommending specs (filter and priority)"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID to filter specs by the holder's available providers (default: system default holder which shows all providers)"
// @Success 200 {object} []model.SpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /recommendSpec [post]
func RestRecommendSpec(c echo.Context) error {

	nsId := model.SystemCommonNs

	u := &model.RecommendSpecReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	ctx := c.Request().Context()
	content, err := infra.RecommendSpec(ctx, nsId, *u)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestRecommendSpecOptions godoc
// @ID RecommendSpecOptions
// @Summary Get options for RecommendSpec API
// @Description Get available options for filtering and prioritizing specs in RecommendSpec API
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.RecommendSpecRequestOptions
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /recommendSpecOptions [get]
func RestRecommendSpecOptions(c echo.Context) error {

	nsId := model.SystemCommonNs

	u := &model.RecommendSpecReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.RecommendSpecOptions(nsId)
	return clientManager.EndRequestWithLog(c, err, content)
}

type RestPostInfraRecommendResponse struct {
	//VmReq          []VmRecommendReq    `json:"vmReq"`
	VmRecommend    []model.VmRecommendInfo `json:"vmRecommend"`
	PlacementAlgo  string                  `json:"placementAlgo"`
	PlacementParam []model.KeyValue        `json:"placementParam"`
}

// RestPostInfraRecommend godoc
// @Deprecated
// func RestPostInfraRecommend(c echo.Context) error {
// 	// @Summary Get Infra recommendation
// 	// @Description Get Infra recommendation
// 	// @Tags [MC-Infra] Infra Provisioning and Management
// 	// @Accept  json
// 	// @Produce  json
// 	// @Param nsId path string true "Namespace ID" default(default)
// 	// @Param infraRecommendReq body model.InfraRecommendReq true "Details for an Infra object"
// 	// @Success 200 {object} RestPostInfraRecommendResponse
// 	// @Failure 404 {object} model.SimpleMsg
// 	// @Failure 500 {object} model.SimpleMsg
// 	// @Router /ns/{nsId}/infra/recommend [post]
// 	nsId := c.Param("nsId")

// 	req := &model.InfraRecommendReq{}
// 	if err := c.Bind(req); err != nil {
// 		return err
// 	}

// 	result, err := model.CorePostInfraRecommend(nsId, req)
// 	if err != nil {
// 		mapA := map[string]string{"message": err.Error()}
// 		return c.JSON(http.StatusInternalServerError, &mapA)
// 	}

// 	content := RestPostInfraRecommendResponse{}
// 	content.VmRecommend = result
// 	content.PlacementAlgo = req.PlacementAlgo
// 	content.PlacementParam = req.PlacementParam

// 	common.PrintJsonPretty(content)

// 	return c.JSON(http.StatusCreated, content)
// }
