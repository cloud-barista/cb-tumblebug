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
	"net/http"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostInfraAutopilotReview godoc
// @ID PostInfraAutopilotReview
// @Summary Pre-flight review for autopilot infra provisioning
// @Description Review and validate an InfraAutopilotReq without creating any resources.
// @Description For each NodeSpec in the request, the endpoint resolves candidate specs (via RecommendSpec),
// @Description finds matching images, and runs ReviewSpecImagePair on each candidate.
// @Description The result contains per-NodeSpec candidate reviews, validity flags, risk levels, cost estimates,
// @Description suggested zones/disks, and an overall feasibility summary.
// @Description
// @Description **Use Cases:**
// @Description - Validate autopilot requests before committing to provisioning
// @Description - Estimate cost ranges for planned infra
// @Description - Identify specs with no valid candidates early
// @Description - Check availability zones and disk recommendations
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraAutopilotReq body model.InfraAutopilotReq true "Autopilot infra request to review"
// @Success 200 {object} model.InfraAutopilotReviewResult "Pre-flight review result with per-NodeSpec candidate details and overall feasibility summary"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or missing required fields"
// @Failure 404 {object} model.SimpleMsg "Namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal server error during review"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use"
// @Router /ns/{nsId}/infraAutopilotReview [post]
func RestPostInfraAutopilotReview(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")

	req := &model.InfraAutopilotReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for infra autopilot review")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ReviewInfraAutopilot(ctx, nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to review infra autopilot request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostInfraAutopilot godoc
// @ID PostInfraAutopilot
// @Summary Create infra using declarative autopilot provisioning
// @Description Create a multi-cloud infra using the autopilot engine, which automatically resolves
// @Description candidate specs and images for each NodeSpec, reviews each pair for compatibility
// @Description and availability, then provisions node groups until the DesiredCount is satisfied
// @Description or all candidates are exhausted.
// @Description
// @Description **Key Features:**
// @Description - Automatic spec and image resolution per NodeSpec
// @Description - Pre-flight ReviewSpecImagePair validation before each provisioning attempt
// @Description - Respects MaxPerLocation and PlacementPolicy for geographic distribution
// @Description - Configurable retry limits via AutopilotPolicy.MaxAttemptsPerSpec
// @Description - Returns detailed attempt history with success/failure reasons
// @Description
// @Description **Provisioning Strategy:**
// @Description 1. For each NodeSpec, call RecommendSpec with the provided SpecFilter
// @Description 2. For each candidate spec, search for a matching image using ImageRequirement
// @Description 3. Run ReviewSpecImagePair; skip invalid pairs
// @Description 4. Apply suggested zone/disk overrides from the review
// @Description 5. Provision a node group and subtract from remaining count
// @Description 6. Continue until DesiredCount fulfilled or candidates exhausted
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraAutopilotReq body model.InfraAutopilotReq true "Autopilot infra creation request"
// @Success 200 {object} model.InfraAutopilotResult "Created infra with autopilot metadata including per-NodeSpec results and attempt history"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or missing required fields"
// @Failure 404 {object} model.SimpleMsg "Namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal server error during provisioning"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use"
// @Router /ns/{nsId}/infraAutopilot [post]
func RestPostInfraAutopilot(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")

	req := &model.InfraAutopilotReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for infra autopilot")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateInfraAutopilot(ctx, nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to create infra via autopilot")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestGetInfraAutopilotStatus godoc
// @ID GetInfraAutopilotStatus
// @Summary Get autopilot provisioning status for an infra
// @Description Returns a lightweight status snapshot of an infra created by autopilot,
// @Description including overall infra status and per-NodeSpec provisioning progress.
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID"
// @Success 200 {object} model.InfraAutopilotStatus "Autopilot provisioning status"
// @Failure 404 {object} model.SimpleMsg "Namespace or infra not found"
// @Failure 500 {object} model.SimpleMsg "Internal server error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/infraAutopilot/{infraId}/status [get]
func RestGetInfraAutopilotStatus(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	result, err := infra.GetInfraAutopilotStatus(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get autopilot status for infra '%s'", infraId)
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}
