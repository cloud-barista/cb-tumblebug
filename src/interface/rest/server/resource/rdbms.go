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
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/apierr"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestGetRDBMSSupport godoc
// @ID GetRDBMSSupport
// @Summary Get CSP RDBMS capability support information
// @Description Get CSP support metadata and capabilities for a single connection (resolved from
// @Description providerName+regionName) and DB engine. This is computed live per request, so
// @Description providerName/regionName/dbEngine are all required to avoid a slow fan-out across
// @Description every registered connection and every supported engine.
// @Description
// @Description Some fields (e.g. storageTypeOptions, storageSizeRange) may be a fixed/approximate
// @Description reference value rather than a live, current value for a given CSP. Check
// @Description notes.staticFields for which field names that applies to right now, and why; a
// @Description field not listed there is live.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param providerName query string true "Provider Name (e.g., aws, gcp, azure, ncp)" example(aws) default(aws)
// @Param regionName query string true "Region Name (e.g., ap-northeast-2)" example(ap-northeast-2) default(ap-northeast-2)
// @Param dbEngine query string true "DB Engine Name" Enums(mysql, mariadb, postgresql) example(mysql) default(mysql)
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
	if providerName == "" {
		err := fmt.Errorf("providerName is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	regionName := c.QueryParam("regionName")
	if regionName == "" {
		regionName = c.QueryParam("region")
	}
	if regionName == "" {
		err := fmt.Errorf("regionName is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	dbEngine := c.QueryParam("dbEngine")
	if dbEngine == "" {
		err := fmt.Errorf("dbEngine is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := resource.GetRDBMSSupport(providerName, regionName, dbEngine)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get RDBMS support info (provider: '%s', region: '%s', dbEngine: '%s')", providerName, regionName, dbEngine)
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestPostRDBMS godoc
// @ID PostRDBMS
// @Summary Create an RDBMS instance
// @Description Create a managed RDBMS instance and wait for it to become Available (provisioning
// @Description can take several minutes).
// @Description
// @Description Call GET /tumblebug/rdbms/support first to discover valid dbEngineVersion/
// @Description dbInstanceSpec/storageType/storageSize values and Requires*/Supports* constraints
// @Description for the target connectionName. Alternatively, set autoFillDefaults=true to have
// @Description those four fields filled from the first supported option reported — this is a
// @Description capability-valid pick, not a cost/performance recommendation.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param reqBody body model.RDBMSCreateRequest true "RDBMS Create Request"
// @Success 200 {object} model.RDBMSInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 409 {object} model.SimpleMsg "Conflict"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms [post]
func RestPostRDBMS(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	req := model.RDBMSCreateRequest{}
	if err := c.Bind(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind request body to RDBMSCreateRequest")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := resource.CreateRDBMS(ctx, nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create RDBMS")
		return c.JSON(apierr.Code(err), model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestListRDBMSs godoc
// @ID ListRDBMSs
// @Summary List RDBMS instances
// @Description Get the list of RDBMS instances registered in the namespace
// @Description
// @Description **Filtering with filterKey and filterVal:**
// @Description Both parameters perform a case-insensitive substring match against the stored JSON of each resource.
// @Description A resource is included in the result only when its JSON contains **both** the filterKey string and the filterVal string.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: connectionName)"
// @Param filterVal query string false "Field value for filtering (ex: aws-ap-northeast-2)"
// @Success 200 {object} JSONResult{[DEFAULT]=model.RDBMSListResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms [get]
func RestListRDBMSs(c echo.Context) error {
	// This is a dummy function for Swagger. Actual handling is done by RestGetAllResources.
	return nil
}

// RestGetRDBMS godoc
// @ID GetRDBMS
// @Summary Get details of an RDBMS instance
// @Description Get details of an RDBMS instance, always refreshed live
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param rdbmsId path string true "RDBMS ID" default(rdbms-01)
// @Success 200 {object} model.RDBMSInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/{rdbmsId} [get]
func RestGetRDBMS(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	rdbmsId := c.Param("rdbmsId")
	if rdbmsId == "" {
		err := fmt.Errorf("rdbmsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := resource.GetRDBMS(nsId, rdbmsId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get RDBMS")
		return c.JSON(apierr.Code(err), model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestDeleteRDBMS godoc
// @ID DeleteRDBMS
// @Summary Delete an RDBMS instance
// @Description Delete an RDBMS instance. Use option=force to delete despite DeletionProtection.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param rdbmsId path string true "RDBMS ID" default(rdbms-01)
// @Param option query string false "Option" Enums(force)
// @Success 204 "No Content"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/{rdbmsId} [delete]
func RestDeleteRDBMS(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	rdbmsId := c.Param("rdbmsId")
	if rdbmsId == "" {
		err := fmt.Errorf("rdbmsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	force := c.QueryParam("option") == "force"

	if err := resource.DeleteRDBMS(nsId, rdbmsId, force); err != nil {
		log.Error().Err(err).Msg("Failed to delete RDBMS")
		return c.JSON(apierr.Code(err), model.SimpleMsg{Message: err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
