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

package infra

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestPostRetryFailedNodesReview godoc
// @ID RetryFailedNodesReview
// @Summary Review which failed Nodes can be retried
// @Description Classify every failed Node in the Infra and report whether re-creating it can succeed.
// @Description
// @Description Nothing is created. Each plan carries the classified CSP failure, the action
// @Description (retryInPlace or none), a reason, and the hourly cost of one replacement.
// @Description
// @Description A retry re-creates the Node with the identical configuration — same zone, subnet,
// @Description VNet, security group and key. CSP capacity shortages are transient, so the same
// @Description request is often accepted minutes later. Failures that retrying cannot fix
// @Description (account quota, an image the spec rejects, a malformed request) are reported with
// @Description the reason instead.
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param retryReq body model.RetryFailedNodesReq false "Optional filter and retry policy"
// @Success 200 {object} model.RetryFailedNodesReview
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/infra/{infraId}/retryFailedNodesReview [post]
func RestPostRetryFailedNodesReview(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.RetryFailedNodesReq{}
	// The body is optional: no body means "review every failed node".
	if err := c.Bind(req); err != nil {
		req = &model.RetryFailedNodesReq{}
	}

	result, err := infra.ReviewRetryFailedNodes(nsId, infraId, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostRetryFailedNodes godoc
// @ID RetryFailedNodes
// @Summary Retry failed Nodes with their original configuration
// @Description Re-create the retriable failed Nodes of the Infra, one Node at a time.
// @Description
// @Description Each replacement keeps the failed Node's exact configuration — zone, subnet, VNet,
// @Description security group, key, spec, image and root disk — so it stays on the same private
// @Description network as its siblings. Use attemptsPerNode with intervalSeconds to keep trying
// @Description while a CSP capacity shortage clears.
// @Description
// @Description Each target names one failed Node and the settings for it: an optional zone,
// @Description resolved to a subnet of that Node's own VNet, and assumeResolved to retry a Node
// @Description the classifier ruled out because the block was lifted outside CB-Tumblebug (a
// @Description quota granted, a permission fixed). An empty targets list retries every retriable
// @Description failed Node. The original failed Node record is removed once its replacement is
// @Description up, unless keepFailedNodes is set.
// @Description
// @Description A target's zone never leaves the Node's VNet: it selects a subnet of that zone, or
// @Description adds one, so the replacement keeps the VPC, security group and key. This is not the
// @Description zone field of a dynamic provisioning request, which builds a separate zone-scoped
// @Description VNet.
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param retryReq body model.RetryFailedNodesReq false "Optional filter and retry policy"
// @Success 200 {object} model.RetryFailedNodesResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/infra/{infraId}/retryFailedNodes [post]
func RestPostRetryFailedNodes(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.RetryFailedNodesReq{}
	if err := c.Bind(req); err != nil {
		req = &model.RetryFailedNodesReq{}
	}

	result, err := infra.RetryFailedNodes(c.Request().Context(), nsId, infraId, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}
