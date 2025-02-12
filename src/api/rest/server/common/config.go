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
	"github.com/rs/zerolog/log"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// RestInitConfig godoc
// @ID InitConfig
// @Summary Init config
// @Description Init config
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param configId path string true "Config ID"
// @Success 200 {object} model.ConfigInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /config/{configId} [delete]
func RestInitConfig(c echo.Context) error {

	if err := Validate(c, []string{"configId"}); err != nil {
		log.Error().Err(err).Msg("")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	err := common.InitConfig(c.Param("configId"))
	if err != nil {
		err := fmt.Errorf("Failed to init the config " + c.Param("configId"))
		return clientManager.EndRequestWithLog(c, err, nil)
	} else {
		// return SendMessage(c, http.StatusOK, "The config "+c.Param("configId")+" has been initialized.")
		content := map[string]string{"message": "The config " + c.Param("configId") + " has been initialized."}
		return clientManager.EndRequestWithLog(c, err, content)
	}
}

// RestGetConfig godoc
// @ID GetConfig
// @Summary Get config
// @Description Get config
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param configId path string true "Config ID"
// @Success 200 {object} model.ConfigInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /config/{configId} [get]
func RestGetConfig(c echo.Context) error {

	if err := Validate(c, []string{"configId"}); err != nil {
		log.Error().Err(err).Msg("")
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	content, err := common.GetConfig(c.Param("configId"))
	if err != nil {
		err := fmt.Errorf("Failed to find the config " + c.Param("configId"))
		return clientManager.EndRequestWithLog(c, err, nil)
	} else {
		return clientManager.EndRequestWithLog(c, err, content)
	}
}

// Response structure for RestGetAllConfig
type RestGetAllConfigResponse struct {
	//Name string     `json:"name"`
	Config []model.ConfigInfo `json:"config"`
}

// RestGetAllConfig godoc
// @ID GetAllConfig
// @Summary List all configs
// @Description List all configs
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} RestGetAllConfigResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /config [get]
func RestGetAllConfig(c echo.Context) error {

	var content RestGetAllConfigResponse

	configList, err := common.ListConfig()
	content.Config = configList
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPostConfig godoc
// @ID PostConfig
// @Summary Create or Update config
// @Description Create or Update config (TB_SPIDER_REST_URL, TB_DRAGONFLY_REST_URL, ...)
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param config body model.ConfigReq true "Key and Value for configuration"
// @Success 200 {object} model.ConfigInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /config [post]
func RestPostConfig(c echo.Context) error {

	u := &model.ConfigReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msg("[Creating or Updating Config]")
	content, err := common.UpdateConfig(u)
	return clientManager.EndRequestWithLog(c, err, content)

}

// RestInitAllConfig godoc
// @ID InitAllConfig
// @Summary Init all configs
// @Description Init all configs
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /config [delete]
func RestInitAllConfig(c echo.Context) error {

	err := common.InitAllConfig()
	content := map[string]string{
		"message": "All configs has been initialized"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetRequest godoc
// @ID GetRequest
// @Summary Get request details
// @Description Get details of a specific request
// @Tags [Admin] API Request Management
// @Accept  json
// @Produce  json
// @Param reqId path string true "Request ID acquired from X-Request-ID header"
// @Success 200 {object} clientManager.RequestDetails
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /request/{reqId} [get]
func RestGetRequest(c echo.Context) error {
	reqId := c.Param("reqId")

	if details, ok := clientManager.RequestMap.Load(reqId); ok {
		return Send(c, http.StatusOK, details)
	}

	return SendMessage(c, http.StatusNotFound, "Request ID not found")
}

// RestGetAllRequests godoc
// @ID GetAllRequests
// @Summary Get all requests
// @Description Get details of all requests with optional filters.
// @Tags [Admin] API Request Management
// @Accept  json
// @Produce  json
// @Param status query string false "Filter by request status (Handling, Error, Success)"
// @Param method query string false "Filter by HTTP method (GET, POST, etc.)"
// @Param url query string false "Filter by request URL"
// @Param time query string false "Filter by time in minutes from now (to get recent requests)"
// @Param savefile query string false "Option to save the results to a file (set 'true' to activate)"
// @Success 200 {object} map[string][]clientManager.RequestDetails
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

	var allRequests []clientManager.RequestDetails

	// Filtering the requests
	clientManager.RequestMap.Range(func(key, value interface{}) bool {
		if details, ok := value.(clientManager.RequestDetails); ok {
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
		cbTumblebugRoot := os.Getenv("TB_ROOT_PATH")
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

	return Send(c, http.StatusOK, map[string][]clientManager.RequestDetails{"requests": allRequests})
}

// RestDeleteRequest godoc
// @ID DeleteRequest
// @Summary Delete a specific request's details
// @Description Delete details of a specific request
// @Tags [Admin] API Request Management
// @Accept  json
// @Produce  json
// @Param reqId path string true "Request ID to delete"
// @Success 200 {object} model.SimpleMsg
// @Router /request/{reqId} [delete]
func RestDeleteRequest(c echo.Context) error {
	reqId := c.Param("reqId")

	if _, ok := clientManager.RequestMap.Load(reqId); ok {
		clientManager.RequestMap.Delete(reqId)
		return SendMessage(c, http.StatusOK, "Request deleted successfully")
	}

	return SendMessage(c, http.StatusNotFound, "Request ID not found")
}

// RestDeleteAllRequests godoc
// @ID DeleteAllRequests
// @Summary Delete all requests' details
// @Description Delete details of all requests
// @Tags [Admin] API Request Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.SimpleMsg
// @Router /requests [delete]
func RestDeleteAllRequests(c echo.Context) error {
	clientManager.RequestMap.Range(func(key, value interface{}) bool {
		clientManager.RequestMap.Delete(key)
		return true
	})

	return SendMessage(c, http.StatusOK, "All requests deleted successfully")
}
