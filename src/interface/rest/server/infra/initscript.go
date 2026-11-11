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
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/labstack/echo/v4"
)

// RestGetInfraInitScript godoc
// @ID GetInfraInitScript
// @Summary Get init script results of all Nodes in an Infra
// @Description Counts and per-Node results of the first-boot init script, as reported by the Nodes through the
// @Description notifier. A Node is `pending` until its first report, `initializing` while steps run (the current step is in
// @Description `step`), then `ok` or `fail`; `notApplicable` when its NodeGroup has no init script or reporting is off.
// @Description Results are read from a per-Node record, so the call stays cheap for large Infras.
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param option query string false "summary returns the counts only" Enums(default, summary)
// @Param status query string false "Only list Nodes with this status" Enums(initializing, ok, fail, pending, notApplicable)
// @Success 200 {object} model.InfraInitScriptStatus "Init script results"
// @Failure 404 {object} model.SimpleMsg "Infra not found"
// @Failure 500 {object} model.SimpleMsg "Internal error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/infra/{infraId}/initScript [get]
func RestGetInfraInitScript(c echo.Context) error {
	result, err := infra.GetInfraInitScriptStatus(c.Param("nsId"), c.Param("infraId"), c.QueryParam("option"), c.QueryParam("status"))
	return clientManager.EndRequestWithLog(c, err, result)
}
