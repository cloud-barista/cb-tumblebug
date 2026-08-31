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
	"errors"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
)

// RestPutInfraNodeGroupDynamic godoc
// @ID ReplaceFailedNodeGroup
// @Summary Re-create a NodeGroup whose Nodes all failed, with corrected settings
// @Description Replace a NodeGroup that could not be provisioned, using the same name.
// @Description
// @Description Some failures cannot be retried: an image the CSP does not have, a root disk too
// @Description small for the flavor. Re-sending the same request fails identically — the request
// @Description has to change. This clears the failed Nodes and creates the NodeGroup again from
// @Description the corrected request, so its record carries the new values too.
// @Description
// @Description The request body is the same shape as adding a NodeGroup, so it can be validated
// @Description first with POST /ns/{nsId}/infra/{infraId}/nodeGroupDynamicReview.
// @Description
// @Description Refused rather than guessed when clearing could destroy something:
// @Description - 409 if any Node of the group is not failed — add a new NodeGroup instead
// @Description - 409 if a failed Node still names a CSP resource — run action=reconcile to rescue
// @Description   it or action=refine to remove it first
// @Description - 400 if the request carries a different specId; instance type decides cost,
// @Description   performance and availability together, so changing it is a new NodeGroup
// @Description
// @Description A zone in the body is ignored: pinning one derives a zone-scoped shared VNet, which
// @Description would place the new Nodes in a separate VPC from the rest of the Infra.
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeGroupId path string true "NodeGroup ID to replace" default(g1)
// @Param nodeGroupReq body model.AddNodeGroupDynamicReq true "Corrected NodeGroup request; name must match the path"
// @Success 200 {object} model.ReplaceNodeGroupResult
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/infra/{infraId}/nodeGroupDynamic/{nodeGroupId} [put]
func RestPutInfraNodeGroupDynamic(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeGroupId := c.Param("nodeGroupId")

	req := &model.AddNodeGroupDynamicReq{}
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := infra.ReplaceFailedNodeGroup(c.Request().Context(), nsId, infraId, nodeGroupId, req)
	if err != nil {
		switch {
		case errors.Is(err, infra.ErrNodeGroupInUse), errors.Is(err, infra.ErrNodeGroupHasCspResource):
			return c.JSON(http.StatusConflict, model.SimpleMsg{Message: err.Error()})
		case errors.Is(err, infra.ErrSpecChanged), errors.Is(err, infra.ErrNodeGroupNameMismatch):
			return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
		case errors.Is(err, infra.ErrNodeGroupNotFound):
			return c.JSON(http.StatusNotFound, model.SimpleMsg{Message: err.Error()})
		}
		// The failed nodes may already be cleared; return what happened alongside.
		if result != nil {
			return c.JSON(http.StatusInternalServerError, result)
		}
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}
