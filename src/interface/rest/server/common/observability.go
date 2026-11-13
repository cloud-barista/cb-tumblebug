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

package common

import (
	"net/http"
	"time"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/observability"
	"github.com/labstack/echo/v4"
)

// RestGetObservability godoc
// @ID GetObservability
// @Summary Get the registered observability stack
// @Description The OpenTelemetry endpoint (and Grafana) that Nodes send metrics and logs to. Secrets are masked
// @Description unless showSecrets=true. CB-Tumblebug exports it to init scripts as TB_OTEL_ENDPOINT/TB_OTEL_TOKEN/TB_OTEL_CA_B64;
// @Description the `initscript-otel-agent` template installs the agent on a Node.
// @Tags [Admin] System Configuration
// @Produce json
// @Param showSecrets query bool false "Return the write token and Grafana admin password unmasked"
// @Success 200 {object} model.ObservabilityInfo
// @Success 204 "no observability stack is registered"
// @Router /observability [get]
func RestGetObservability(c echo.Context) error {
	var info *model.ObservabilityInfo
	if c.QueryParam("showSecrets") == "true" {
		info = observability.Current()
	} else {
		info = observability.Get()
	}
	if info == nil {
		return c.NoContent(http.StatusNoContent) // nothing registered; 204 keeps browser consoles quiet
	}
	return c.JSON(http.StatusOK, info)
}

// RestPutObservability godoc
// @ID PutObservability
// @Summary Register an existing OTLP endpoint
// @Description Register an OTLP/HTTP endpoint (for example an existing OTel Collector or a hosted service) and,
// @Description optionally, the Grafana in front of it. grafanaAdminUser/Password enable the round-trip test and
// @Description viewer accounts. `otlpEndpoint` and `grafanaUrl` must be `http(s)://host[:port][/path]` without
// @Description credentials, query or fragment; link-local and metadata addresses are rejected.
// @Tags [Admin] System Configuration
// @Accept json
// @Produce json
// @Param req body model.ObservabilityRegisterReq true "Endpoint to register"
// @Success 200 {object} model.ObservabilityInfo
// @Failure 400 {object} model.SimpleMsg
// @Router /observability [put]
func RestPutObservability(c echo.Context) error {
	req := model.ObservabilityRegisterReq{}
	if err := c.Bind(&req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	if err := c.Validate(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	info, err := observability.Register(req)
	return clientManager.EndRequestWithLog(c, err, info)
}

// RestDeleteObservability godoc
// @ID DeleteObservability
// @Summary Unregister the observability stack
// @Description Removes the registration; Nodes created afterwards get no TB_OTEL_* variables. With
// @Description deleteInfra=true a stack provisioned by CB-Tumblebug is terminated together with its resources.
// @Tags [Admin] System Configuration
// @Produce json
// @Param deleteInfra query bool false "Also delete the managed Infra (default false)"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /observability [delete]
func RestDeleteObservability(c echo.Context) error {
	err := observability.DeleteStack(c.QueryParam("deleteInfra") == "true")
	return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "observability stack unregistered"})
}

// RestPostObservabilityStack godoc
// @ID PostObservabilityStack
// @Summary Provision an observability stack and register it
// @Description Creates one VM from an Infra template (default `infra-obs`, namespace `system`), installs nginx (TLS),
// @Description an OTel Collector gateway, Prometheus, Loki and Grafana with CB-Tumblebug dashboards, mirrors the agent
// @Description packages, and registers the stack. With `domainName` (a Route53 hosted zone) the stack gets a DNS name
// @Description and a Let's Encrypt certificate; otherwise a self-signed certificate is used and its CA is stored.
// @Description Takes several minutes.
// @Tags [Admin] System Configuration
// @Accept json
// @Produce json
// @Param req body model.ObservabilityStackReq false "Options (all optional)"
// @Success 200 {object} model.ObservabilityInfo
// @Failure 400 {object} model.SimpleMsg
// @Router /observability/stack [post]
func RestPostObservabilityStack(c echo.Context) error {
	req := model.ObservabilityStackReq{}
	if err := c.Bind(&req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	info, err := observability.CreateStack(c.Request().Context(), req)
	return clientManager.EndRequestWithLog(c, err, info)
}

// RestPostObservabilityTest godoc
// @ID PostObservabilityTest
// @Summary Send a test metric and log line through the stack
// @Description Sends one metric and one log line to the OTLP endpoint and, when Grafana admin access is configured,
// @Description waits until both can be queried back through Grafana.
// @Tags [Admin] System Configuration
// @Produce json
// @Param timeoutSec query int false "Seconds to wait for the data to become queryable (default 60)"
// @Success 200 {object} model.ObservabilityTestResult
// @Router /observability/test [post]
func RestPostObservabilityTest(c echo.Context) error {
	timeout := 60 * time.Second
	if v := c.QueryParam("timeoutSec"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil && d > 0 && d <= 10*time.Minute {
			timeout = d
		}
	}
	return c.JSON(http.StatusOK, observability.Test(timeout))
}

// RestPostObservabilityReader godoc
// @ID PostObservabilityReader
// @Summary Create a Grafana viewer account
// @Description Creates a read-only Grafana user on the stack. The password is returned once.
// @Tags [Admin] System Configuration
// @Accept json
// @Produce json
// @Param req body model.ObservabilityReaderReq true "Account to create"
// @Success 200 {object} model.ObservabilityReaderInfo
// @Failure 400 {object} model.SimpleMsg
// @Router /observability/reader [post]
func RestPostObservabilityReader(c echo.Context) error {
	req := model.ObservabilityReaderReq{}
	if err := c.Bind(&req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	info, err := observability.AddReader(req)
	return clientManager.EndRequestWithLog(c, err, info)
}

// RestGetObservabilityReaders godoc
// @ID GetObservabilityReaders
// @Summary List Grafana viewer accounts created through this API
// @Tags [Admin] System Configuration
// @Produce json
// @Success 200 {array} string
// @Router /observability/reader [get]
func RestGetObservabilityReaders(c echo.Context) error {
	readers, err := observability.ListReaders()
	return clientManager.EndRequestWithLog(c, err, readers)
}

// RestDeleteObservabilityReader godoc
// @ID DeleteObservabilityReader
// @Summary Delete a Grafana viewer account
// @Tags [Admin] System Configuration
// @Produce json
// @Param name path string true "Account name"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /observability/reader/{name} [delete]
func RestDeleteObservabilityReader(c echo.Context) error {
	err := observability.RemoveReader(c.Param("name"))
	return clientManager.EndRequestWithLog(c, err, model.SimpleMsg{Message: "reader removed"})
}
