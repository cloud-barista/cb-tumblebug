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

// Package mci is to handle REST API for mci
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
// @Description Recommend specs for configuring an infrastructure (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param recommendSpecReq body model.RecommendSpecReq false "Conditions for recommending specs (filter and priority)"
// @Success 200 {object} []model.TbSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /recommendSpec [post]
func RestRecommendSpec(c echo.Context) error {

	nsId := model.SystemCommonNs

	u := &model.RecommendSpecReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.RecommendSpec(nsId, *u)
	return clientManager.EndRequestWithLog(c, err, content)
}

type RestPostMciRecommendResponse struct {
	//VmReq          []TbVmRecommendReq    `json:"vmReq"`
	VmRecommend    []model.TbVmRecommendInfo `json:"vmRecommend"`
	PlacementAlgo  string                    `json:"placementAlgo"`
	PlacementParam []model.KeyValue          `json:"placementParam"`
}

// RestPostMciRecommend godoc
// @Deprecated
// func RestPostMciRecommend(c echo.Context) error {
// 	// @Summary Get MCI recommendation
// 	// @Description Get MCI recommendation
// 	// @Tags [MC-Infra] MCI Provisioning and Management
// 	// @Accept  json
// 	// @Produce  json
// 	// @Param nsId path string true "Namespace ID" default(default)
// 	// @Param mciRecommendReq body model.MciRecommendReq true "Details for an MCI object"
// 	// @Success 200 {object} RestPostMciRecommendResponse
// 	// @Failure 404 {object} model.SimpleMsg
// 	// @Failure 500 {object} model.SimpleMsg
// 	// @Router /ns/{nsId}/mci/recommend [post]
// 	nsId := c.Param("nsId")

// 	req := &model.MciRecommendReq{}
// 	if err := c.Bind(req); err != nil {
// 		return err
// 	}

// 	result, err := model.CorePostMciRecommend(nsId, req)
// 	if err != nil {
// 		mapA := map[string]string{"message": err.Error()}
// 		return c.JSON(http.StatusInternalServerError, &mapA)
// 	}

// 	content := RestPostMciRecommendResponse{}
// 	content.VmRecommend = result
// 	content.PlacementAlgo = req.PlacementAlgo
// 	content.PlacementParam = req.PlacementParam

// 	common.PrintJsonPretty(content)

// 	return c.JSON(http.StatusCreated, content)
// }
