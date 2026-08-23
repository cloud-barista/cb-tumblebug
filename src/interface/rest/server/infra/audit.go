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
	"strings"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/labstack/echo/v4"
)

func auditOptionsFromQuery(c echo.Context) infra.AuditOptions {
	return infra.AuditOptions{
		Remediate:      strings.EqualFold(c.QueryParam("remediate"), "true"),
		CleanResiduals: c.QueryParam("cleanResiduals"),
	}
}

// RestPostInfraAudit godoc
// @ID PostInfraAudit
// @Summary Audit an Infra against the CSP truth (direct SDK) and optionally remediate orphans
// @Description Lists VMs directly from each CSP used by the Infra (never via CB-Spider metadata) and classifies them:
// @Description - `trackedAlive`: TB node exists and the CSP VM is alive (expected)
// @Description - `ghostAlive`: TB recorded the node as Terminated/Failed but the CSP VM is still alive
// @Description - `trackedGone`: TB node has a CSP id but the CSP has no such VM
// @Description - `untrackedAlive`: a CSP VM of this Infra (by name/uid or sys.infraId tag) that TB never recorded
// @Description - `residuals`: NIC / public IP / disk / ENI left behind (attributed = named after a node of this Infra)
// @Description
// @Description `remediate=true` terminates ghost/untracked VMs directly at the CSP. `cleanResiduals=attributed|all` deletes residuals.
// @Tags [Infra] Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param remediate query boolean false "Terminate ghost/untracked VMs at the CSP" default(false)
// @Param cleanResiduals query string false "Delete residual sub-resources" Enums(none,attributed,all) default(none)
// @Success 200 {object} infra.AuditResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/infra/{infraId}/audit [post]
func RestPostInfraAudit(c echo.Context) error {
	result, err := infra.AuditInfra(c.Param("nsId"), c.Param("infraId"), auditOptionsFromQuery(c))
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestAuditCspResourcesRequest selects the connection to audit.
type RestAuditCspResourcesRequest struct {
	ConnectionName string `json:"connectionName" example:"aws-ap-northeast-2"`
}

// RestPostAuditCspResources godoc
// @ID PostAuditCspResources
// @Summary Audit every TB-managed VM at a connection's CSP region against all TB records (all namespaces)
// @Description Lists VMs directly from the CSP and reports TB-managed VMs (tb-uid names or sys.manager tag) that no TB record tracks, plus residual sub-resources.
// @Description `remediate=true` terminates untracked VMs; `cleanResiduals=attributed|all` deletes residuals.
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param auditReq body RestAuditCspResourcesRequest true "Connection to audit"
// @Param remediate query boolean false "Terminate untracked TB-managed VMs" default(false)
// @Param cleanResiduals query string false "Delete residual sub-resources" Enums(none,attributed,all) default(none)
// @Success 200 {object} infra.AuditResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /auditCspResources [post]
func RestPostAuditCspResources(c echo.Context) error {
	req := &RestAuditCspResourcesRequest{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	result, err := infra.AuditConnection(req.ConnectionName, auditOptionsFromQuery(c))
	return clientManager.EndRequestWithLog(c, err, result)
}
