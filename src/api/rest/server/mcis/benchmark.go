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

// Package mcis is to handle REST API for mcis
package mcis

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostInstallBenchmarkAgentToMcis godoc
// @ID PostInstallBenchmarkAgentToMcis
// @Summary Install the benchmark agent to specified MCIS
// @Description Install the benchmark agent to specified MCIS
// @Tags [Infra service] MCIS Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param mcisCmdReq body mcis.McisCmdReq true "MCIS Command Request"
// @Param option query string false "Option for checking update" Enums(update)
// @Success 200 {object} mcis.McisSshCmdResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/installBenchmarkAgent/mcis/{mcisId} [post]
func RestPostInstallBenchmarkAgentToMcis(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	option := c.QueryParam("option")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	resultArray, err := mcis.InstallBenchmarkAgentToMcis(nsId, mcisId, req, option)
	if err != nil {
		common.EndRequestWithLog(c, reqID, err, nil)
	}

	content := mcis.McisSshCmdResult{}
	for _, v := range resultArray {
		content.Results = append(content.Results, v)
	}

	return common.EndRequestWithLog(c, reqID, err, content)

}

// Request struct for RestGetAllBenchmark
type RestGetAllBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetAllBenchmark godoc
// @ID GetAllBenchmark
// @Summary Run MCIS benchmark for all performance metrics and return results
// @Description Run MCIS benchmark for all performance metrics and return results
// @Tags [Infra service] MCIS Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param hostIP body RestGetAllBenchmarkRequest true "Host IP address to benchmark"
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmarkAll/mcis/{mcisId} [post]
func RestGetAllBenchmark(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &RestGetAllBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.RunAllBenchmarks(nsId, mcisId, req.Host)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetLatencyBenchmark godoc
// @ID GetLatencyBenchmark
// @Summary Run MCIS benchmark for network latency
// @Description Run MCIS benchmark for network latency
// @Tags [Infra service] MCIS Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system-purpose-common-ns)
// @Param mcisId path string true "MCIS ID" default(probe)
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmarkLatency/mcis/{mcisId} [get]
func RestGetBenchmarkLatency(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	content, err := mcis.RunLatencyBenchmark(nsId, mcisId, "")
	return common.EndRequestWithLog(c, reqID, err, content)
}

type RestGetBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetBenchmark godoc
// @ID GetBenchmark
// @Summary Run MCIS benchmark for a single performance metric and return results
// @Description Run MCIS benchmark for a single performance metric and return results
// @Tags [Infra service] MCIS Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param mcisId path string true "MCIS ID" default(mcis01)
// @Param hostIP body RestGetBenchmarkRequest true "Host IP address to benchmark"
// @Param action query string true "Benchmark Action to MCIS" Enums(install, init, cpus, cpum, memR, memW, fioR, fioW, dbR, dbW, rtt, mrtt, clean)
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmark/mcis/{mcisId} [post]
func RestGetBenchmark(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	action := c.QueryParam("action")

	req := &RestGetBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.CoreGetBenchmark(nsId, mcisId, action, req.Host)
	return common.EndRequestWithLog(c, reqID, err, content)
}
