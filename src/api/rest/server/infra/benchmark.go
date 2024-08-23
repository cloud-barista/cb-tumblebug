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
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/labstack/echo/v4"
)

// RestPostInstallBenchmarkAgentToMci godoc
// @ID PostInstallBenchmarkAgentToMci
// @Summary Install the benchmark agent to specified MCI
// @Description Install the benchmark agent to specified MCI
// @Tags [MC-Infra] MCI Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param mciCmdReq body infra.MciCmdReq true "MCI Command Request"
// @Param option query string false "Option for checking update" Enums(update)
// @Success 200 {object} infra.MciSshCmdResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/installBenchmarkAgent/mci/{mciId} [post]
func RestPostInstallBenchmarkAgentToMci(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	option := c.QueryParam("option")

	req := &infra.MciCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	resultArray, err := infra.InstallBenchmarkAgentToMci(nsId, mciId, req, option)
	if err != nil {
		common.EndRequestWithLog(c, reqID, err, nil)
	}

	content := infra.MciSshCmdResult{}
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
// @Summary Run MCI benchmark for all performance metrics and return results
// @Description Run MCI benchmark for all performance metrics and return results
// @Tags [MC-Infra] MCI Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param hostIP body RestGetAllBenchmarkRequest true "Host IP address to benchmark"
// @Success 200 {object} infra.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmarkAll/mci/{mciId} [post]
func RestGetAllBenchmark(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &RestGetAllBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := infra.RunAllBenchmarks(nsId, mciId, req.Host)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetLatencyBenchmark godoc
// @ID GetLatencyBenchmark
// @Summary Run MCI benchmark for network latency
// @Description Run MCI benchmark for network latency
// @Tags [MC-Infra] MCI Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(system)
// @Param mciId path string true "MCI ID" default(probe)
// @Success 200 {object} infra.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmarkLatency/mci/{mciId} [get]
func RestGetBenchmarkLatency(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	content, err := infra.RunLatencyBenchmark(nsId, mciId, "")
	return common.EndRequestWithLog(c, reqID, err, content)
}

type RestGetBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetBenchmark godoc
// @ID GetBenchmark
// @Summary Run MCI benchmark for a single performance metric and return results
// @Description Run MCI benchmark for a single performance metric and return results
// @Tags [MC-Infra] MCI Performance Benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciId path string true "MCI ID" default(mci01)
// @Param hostIP body RestGetBenchmarkRequest true "Host IP address to benchmark"
// @Param action query string true "Benchmark Action to MCI" Enums(install, init, cpus, cpum, memR, memW, fioR, fioW, dbR, dbW, rtt, mrtt, clean)
// @Success 200 {object} infra.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmark/mci/{mciId} [post]
func RestGetBenchmark(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	action := c.QueryParam("action")

	req := &RestGetBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := infra.CoreGetBenchmark(nsId, mciId, action, req.Host)
	return common.EndRequestWithLog(c, reqID, err, content)
}
