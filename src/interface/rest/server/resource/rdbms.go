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

// Package resource is to handle REST API for resource
package resource

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestGetRDBMSSupport godoc
// @ID GetRDBMSSupport
// @Summary Get CSP RDBMS capability support information
// @Description Get CSP support metadata and capabilities for RDBMS engines (e.g., mysql, mariadb, postgresql)
// @Description Query parameters can be filtered by providerName, regionName, or dbEngine.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param providerName query string true "Provider Name (e.g., aws, gcp, azure, ncp)" example(aws) default(aws)
// @Param regionName query string true "Region Name (e.g., ap-northeast-2)" example(ap-northeast-2) default(ap-northeast-2)
// @Param dbEngine query string true "DB Engine Name (e.g., mysql, mariadb, postgresql)" Enums(mysql, mariadb, postgresql) example(mysql) default(mysql)
// @Success 200 {object} model.RDBMSSupportResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /rdbms/support [get]
func RestGetRDBMSSupport(c echo.Context) error {
	providerName := c.QueryParam("providerName")
	if providerName == "" {
		providerName = c.QueryParam("provider")
	}

	regionName := c.QueryParam("regionName")
	if regionName == "" {
		regionName = c.QueryParam("region")
	}

	dbEngine := c.QueryParam("dbEngine")

	result, err := resource.GetRDBMSSupport(providerName, regionName, dbEngine)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get RDBMS support info (provider: '%s', region: '%s', dbEngine: '%s')", providerName, regionName, dbEngine)
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}
