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
package mci

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mci"
	"github.com/labstack/echo/v4"
)

// RestRecommendVm godoc
// @ID RecommendVm
// @Summary Recommend MCI plan (filter and priority)
// @Description Recommend MCI plan (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param deploymentPlan body mci.DeploymentPlan false "Recommend MCI plan (filter and priority)"
// @Success 200 {object} []mcir.TbSpecInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /mciRecommendVm [post]
func RestRecommendVm(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := common.SystemCommonNs

	u := &mci.DeploymentPlan{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mci.RecommendVm(nsId, *u)
	return common.EndRequestWithLog(c, reqID, err, content)
}

type RestPostMciRecommendResponse struct {
	//VmReq          []TbVmRecommendReq    `json:"vmReq"`
	VmRecommend    []mci.TbVmRecommendInfo `json:"vmRecommend"`
	PlacementAlgo  string                  `json:"placementAlgo"`
	PlacementParam []common.KeyValue       `json:"placementParam"`
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
// 	// @Param mciRecommendReq body mci.MciRecommendReq true "Details for an MCI object"
// 	// @Success 200 {object} RestPostMciRecommendResponse
// 	// @Failure 404 {object} common.SimpleMsg
// 	// @Failure 500 {object} common.SimpleMsg
// 	// @Router /ns/{nsId}/mci/recommend [post]
// 	nsId := c.Param("nsId")

// 	req := &mci.MciRecommendReq{}
// 	if err := c.Bind(req); err != nil {
// 		return err
// 	}

// 	result, err := mci.CorePostMciRecommend(nsId, req)
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
