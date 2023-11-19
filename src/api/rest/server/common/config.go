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

// Package common is to handle REST API for common funcitonalities
package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

// RestInitConfig godoc
// @Summary Init config
// @Description Init config
// @Tags [Admin] System environment
// @Accept  json
// @Produce  json
// @Param configId path string true "Config ID"
// @Success 200 {object} common.ConfigInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /config/{configId} [delete]
func RestInitConfig(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	if err := Validate(c, []string{"configId"}); err != nil {
		common.CBLog.Error(err)
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	err := common.InitConfig(c.Param("configId"))
	if err != nil {
		err := fmt.Errorf("Failed to init the config " + c.Param("configId"))
		return common.EndRequestWithLog(c, reqID, err, nil)
	} else {
		return SendMessage(c, http.StatusOK, "The config "+c.Param("configId")+" has been initialized.")
		content := map[string]string{"message": "The config " + c.Param("configId") + " has been initialized."}
		return common.EndRequestWithLog(c, reqID, err, content)
	}
}

// RestGetConfig godoc
// @Summary Get config
// @Description Get config
// @Tags [Admin] System environment
// @Accept  json
// @Produce  json
// @Param configId path string true "Config ID"
// @Success 200 {object} common.ConfigInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /config/{configId} [get]
func RestGetConfig(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	if err := Validate(c, []string{"configId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	content, err := common.GetConfig(c.Param("configId"))
	if err != nil {
		err := fmt.Errorf("Failed to find the config " + c.Param("configId"))
		return common.EndRequestWithLog(c, reqID, err, nil)
	} else {
		return common.EndRequestWithLog(c, reqID, err, content)
	}
}

// Response structure for RestGetAllConfig
type RestGetAllConfigResponse struct {
	//Name string     `json:"name"`
	Config []common.ConfigInfo `json:"config"`
}

// RestGetAllConfig godoc
// @Summary List all configs
// @Description List all configs
// @Tags [Admin] System environment
// @Accept  json
// @Produce  json
// @Success 200 {object} RestGetAllConfigResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /config [get]
func RestGetAllConfig(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	var content RestGetAllConfigResponse

	configList, err := common.ListConfig()
	content.Config = configList
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestPostConfig godoc
// @Summary Create or Update config
// @Description Create or Update config (SPIDER_REST_URL, DRAGONFLY_REST_URL, ...)
// @Tags [Admin] System environment
// @Accept  json
// @Produce  json
// @Param config body common.ConfigReq true "Key and Value for configuration"
// @Success 200 {object} common.ConfigInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /config [post]
func RestPostConfig(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	u := &common.ConfigReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	fmt.Println("[Creating or Updating Config]")
	content, err := common.UpdateConfig(u)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestInitAllConfig godoc
// @Summary Init all configs
// @Description Init all configs
// @Tags [Admin] System environment
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /config [delete]
func RestInitAllConfig(c echo.Context) error {
	reqID := common.StartRequestWithLog(c)
	err := common.InitAllConfig()
	content := map[string]string{
		"message": "All configs has been initialized"}
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetRequest godoc
// @Summary Get request details
// @Description Get details of a specific request
// @Tags [Admin] Request tracking
// @Accept  json
// @Produce  json
// @Param reqId path string true "Request ID acquired from X-Request-ID header"
// @Success 200 {object} common.RequestDetails
// @Failure 404 {object} SimpleMsg
// @Failure 500 {object} SimpleMsg
// @Router /request/{reqId} [get]
func RestGetRequest(c echo.Context) error {
	reqId := c.Param("reqId")

	if details, ok := common.RequestMap.Load(reqId); ok {
		return Send(c, http.StatusOK, details)
	}

	return SendMessage(c, http.StatusNotFound, "Request ID not found")
}

// RestGetAllRequests godoc
// @Summary Get all requests
// @Description Get details of all requests with optional filters.
// @Tags [Admin] Request tracking
// @Accept  json
// @Produce  json
// @Param status query string false "Filter by request status (Handling, Error, Success)"
// @Param method query string false "Filter by HTTP method (GET, POST, etc.)"
// @Param url query string false "Filter by request URL"
// @Param time query string false "Filter by time in minutes from now (to get recent requests)"
// @Param savefile query string false "Option to save the results to a file (set 'true' to activate)"
// @Success 200 {object} map[string][]common.RequestDetails
// @Router /requests [get]
func RestGetAllRequests(c echo.Context) error {
	// Filter parameters
	statusFilter := strings.ToLower(c.QueryParam("status"))
	methodFilter := strings.ToLower(c.QueryParam("method"))
	urlFilter := strings.ToLower(c.QueryParam("url"))
	timeFilter := c.QueryParam("time") // in minutes

	var timeLimit time.Time
	if minutes, err := strconv.Atoi(timeFilter); err == nil {
		timeLimit = time.Now().Add(-time.Duration(minutes) * time.Minute)
	}

	var allRequests []common.RequestDetails

	// Filtering the requests
	common.RequestMap.Range(func(key, value interface{}) bool {
		if details, ok := value.(common.RequestDetails); ok {
			if (statusFilter == "" || strings.ToLower(details.Status) == statusFilter) &&
				(methodFilter == "" || strings.ToLower(details.RequestInfo.Method) == methodFilter) &&
				(urlFilter == "" || strings.Contains(strings.ToLower(details.RequestInfo.URL), urlFilter)) &&
				(timeFilter == "" || details.StartTime.After(timeLimit)) {
				allRequests = append(allRequests, details)
			}
		}
		return true
	})

	// Option to save the result to a file
	if c.QueryParam("savefile") == "true" {
		cbTumblebugRoot := os.Getenv("CBTUMBLEBUG_ROOT")
		logPath := filepath.Join(cbTumblebugRoot, "log", "request_log_"+time.Now().Format("20060102_150405")+".log")
		file, err := os.Create(logPath)
		if err != nil {
			return SendMessage(c, http.StatusInternalServerError, "Failed to create log file")
		}
		defer file.Close()

		// Write each request detail in a new line
		for _, detail := range allRequests {
			jsonLine, _ := json.Marshal(detail)
			file.Write(jsonLine)
			file.WriteString("\n")
		}
	}

	return Send(c, http.StatusOK, map[string][]common.RequestDetails{"requests": allRequests})
}

// RestDeleteRequest godoc
// @Summary Delete a specific request's details
// @Description Delete details of a specific request
// @Tags [Admin] Request tracking
// @Accept  json
// @Produce  json
// @Param reqId path string true "Request ID to delete"
// @Success 200 {object} SimpleMsg
// @Router /request/{reqId} [delete]
func RestDeleteRequest(c echo.Context) error {
	reqId := c.Param("reqId")

	if _, ok := common.RequestMap.Load(reqId); ok {
		common.RequestMap.Delete(reqId)
		return SendMessage(c, http.StatusOK, "Request deleted successfully")
	}

	return SendMessage(c, http.StatusNotFound, "Request ID not found")
}

// RestDeleteAllRequests godoc
// @Summary Delete all requests' details
// @Description Delete details of all requests
// @Tags [Admin] Request tracking
// @Accept  json
// @Produce  json
// @Success 200 {object} SimpleMsg
// @Router /requests [delete]
func RestDeleteAllRequests(c echo.Context) error {
	common.RequestMap.Range(func(key, value interface{}) bool {
		common.RequestMap.Delete(key)
		return true
	})

	return SendMessage(c, http.StatusOK, "All requests deleted successfully")
}
