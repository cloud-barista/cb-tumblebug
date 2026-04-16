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

// RestPostInstallBenchmarkAgentToInfra godoc
// @ID PostInstallBenchmarkAgentToInfra
// @Summary Install the benchmark agent to specified Infra
// @Description Install the benchmark agent to specified Infra
// @Tags [MC-Infra] Infra Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param infraCmdReq body model.InfraCmdReq true "Infra Command Request"
// @Param option query string false "Option for checking update" Enums(update)
// @Success 200 {object} model.InfraSshCmdResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/installBenchmarkAgent/infra/{infraId} [post]
func RestPostInstallBenchmarkAgentToInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	option := c.QueryParam("option")

	req := &model.InfraCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	resultArray, err := infra.InstallBenchmarkAgentToInfra(nsId, infraId, req, option)
	if err != nil {
		clientManager.EndRequestWithLog(c, err, nil)
	}

	content := model.InfraSshCmdResult{}
	for _, v := range resultArray {
		content.Results = append(content.Results, v)
	}

	return clientManager.EndRequestWithLog(c, err, content)

}

// Request struct for RestGetAllBenchmark
type RestGetAllBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetAllBenchmark godoc
// @ID GetAllBenchmark
// @Summary Run Infra benchmark for all performance metrics and return results
// @Description Run Infra benchmark for all performance metrics and return results
// @Tags [MC-Infra] Infra Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param hostIP body RestGetAllBenchmarkRequest true "Host IP address to benchmark"
// @Success 200 {object} model.BenchmarkInfoArray
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/benchmarkAll/infra/{infraId} [post]
func RestGetAllBenchmark(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &RestGetAllBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := infra.RunAllBenchmarks(nsId, infraId, req.Host)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetLatencyBenchmark godoc
// @ID GetLatencyBenchmark
// @Summary Run Infra benchmark for network latency
// @Description Run Infra benchmark for network latency
// @Tags [MC-Infra] Infra Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param infraId path string true "Infra ID" default(probe)
// @Success 200 {object} model.BenchmarkInfoArray
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/benchmarkLatency/infra/{infraId} [get]
func RestGetBenchmarkLatency(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	content, err := infra.RunLatencyBenchmark(nsId, infraId, "")
	return clientManager.EndRequestWithLog(c, err, content)
}

type RestGetBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetBenchmark godoc
// @ID GetBenchmark
// @Summary Run Infra benchmark for a single performance metric and return results
// @Description Run Infra benchmark for a single performance metric and return results
// @Tags [MC-Infra] Infra Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param hostIP body RestGetBenchmarkRequest true "Host IP address to benchmark"
// @Param action query string true "Benchmark Action to Infra" Enums(install, init, cpus, cpum, memR, memW, fioR, fioW, dbR, dbW, rtt, mrtt, clean)
// @Success 200 {object} model.BenchmarkInfoArray
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/benchmark/infra/{infraId} [post]
func RestGetBenchmark(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	action := c.QueryParam("action")

	req := &RestGetBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := infra.CoreGetBenchmark(nsId, infraId, action, req.Host)
	return clientManager.EndRequestWithLog(c, err, content)
}
