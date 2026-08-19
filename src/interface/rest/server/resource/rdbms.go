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
	"github.com/cloud-barista/cb-tumblebug/src/core/reconcile"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestGetRDBMSCapability godoc
// @ID GetRDBMSCapability
// @Summary Get live RDBMS capability details for a specific connection
// @Description Live capability query for one connection+dbEngine (calls CB-Spider), so all
// @Description three params are required. Call GET /tumblebug/rdbms/support first to see
// @Description which CSPs/engines are worth trying. Fields listed in notes.staticFields are
// @Description fixed/approximate rather than live. dbSpecs is a richer per-option catalog
// @Description (vCPU/memory/storage range) than dbSpecOptions; liveSupportedEngines is this
// @Description connection's live-verified engine list, independent of the dbEngine queried.
// @Description Both are best-effort and may be empty if their underlying live call failed.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param providerName query string true "Provider Name" Enums(aws, gcp, azure, alibaba, tencent, ibm, openstack, ncp, nhn, kt) example(aws)
// @Param regionName query string true "Region Name (e.g., ap-northeast-2)" example(ap-northeast-2) default(ap-northeast-2)
// @Param dbEngine query string true "DB Engine Name" Enums(mysql, mariadb) example(mysql) default(mysql)
// @Success 200 {object} model.RDBMSCapabilityResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /rdbms/capability [get]
func RestGetRDBMSCapability(c echo.Context) error {
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

	result, err := resource.GetRDBMSCapability(providerName, regionName, dbEngine)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get RDBMS capability info (provider: '%s', region: '%s', dbEngine: '%s')", providerName, regionName, dbEngine)
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestGetRDBMSSupport godoc
// @ID GetRDBMSSupport
// @Summary Get the static, CSP-wide RDBMS support matrix
// @Description Static per-CSP RDBMS reference (no CB-Spider call, so it cheaply covers every
// @Description CSP). Omit providerName for all CSPs. Use this to decide what to try before
// @Description GET /tumblebug/rdbms/capability, which gives live per-connection details.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param providerName query string false "Provider Name to filter to a single CSP; omit for all CSPs" Enums(aws, gcp, azure, alibaba, tencent, ibm, openstack, ncp, nhn, kt) example(aws)
// @Success 200 {object} model.RDBMSSupportResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /rdbms/support [get]
func RestGetRDBMSSupport(c echo.Context) error {
	providerName := c.QueryParam("providerName")
	if providerName == "" {
		providerName = c.QueryParam("provider")
	}

	result, err := resource.GetRDBMSSupport(providerName)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to get RDBMS support matrix (provider: '%s')", providerName)
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestValidateRDBMS godoc
// @ID ValidateRDBMS
// @Summary Validate an RDBMS create request without creating anything
// @Description Dry run of the same checks POST .../rdbms performs before provisioning
// @Description (network resolution, live CB-Spider capability checks,
// @Description assets/rdbmsinfo.yaml storage-type constraints) — no instance is created, no
// @Description CSP call is made to provision anything. Returns the resolved request
// @Description (autoFillDefaults applied, if set) so the caller can preview exactly what
// @Description POST .../rdbms would use.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param reqBody body model.RDBMSCreateRequest true "RDBMS Create Request"
// @Success 200 {object} model.RDBMSCreateRequest "OK — resolved request"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/validate [post]
func RestValidateRDBMS(c echo.Context) error {
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

	resolved, err := resource.ValidateRDBMSCreateRequest(nsId, req)
	if err != nil {
		// Always 400: ValidateRDBMSCreateRequest only ever fails on client-input/capability
		// validation (it never reaches the Spider provisioning call), unlike CreateRDBMS's
		// error, which can also come from downstream Spider/CSP calls (apierr.Code territory).
		log.Warn().Err(err).Msg("RDBMS create request validation failed")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resolved)
}

// RestPostRDBMS godoc
// @ID PostRDBMS
// @Summary Create an RDBMS instance
// @Description Create a managed RDBMS instance and wait for it to become Available
// @Description (can take several minutes). Call GET /tumblebug/rdbms/capability first to
// @Description discover valid dbEngineVersion/dbSpec/storageType/storageSize values,
// @Description or set autoFillDefaults=true to auto-pick a capability-valid (not necessarily
// @Description optimal) default for each.
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

// RestReconcileAllRDBMS godoc
// @ID ReconcileAllRDBMS
// @Summary Reconcile all RDBMS instances in a namespace
// @Description Compares Tumblebug metadata with actual CSP RDBMS status via Spider.
// @Description Restores status for alive resources and flags discrepancies.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.ResourceReconcileResults "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/reconcile [put]
func RestReconcileAllRDBMS(c echo.Context) error {
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := reconcile.GetManager().RunReconcileAll(c.Request().Context(), nsId, model.StrRDBMS, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to reconcile RDBMS instances")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestReconcileRDBMS godoc
// @ID ReconcileRDBMS
// @Summary Reconcile a single RDBMS instance
// @Description Compares Tumblebug metadata for a specific RDBMS instance with actual CSP status via Spider.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param rdbmsId path string true "RDBMS ID" default(rdbms-01)
// @Success 200 {object} model.SimpleMsg "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/{rdbmsId}/reconcile [put]
func RestReconcileRDBMS(c echo.Context) error {
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

	result, err := reconcile.GetManager().RunReconcile(c.Request().Context(), nsId, model.StrRDBMS, rdbmsId, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to reconcile RDBMS (%s)", rdbmsId)
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestPruneRDBMS godoc
// @ID PruneRDBMS
// @Summary Prune orphaned RDBMS metadata in a namespace
// @Description Purges Tumblebug metadata for RDBMS instances diagnosed as missing on CSP.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.ResourcePruneResults "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/reconcile/prune [post]
func RestPruneRDBMS(c echo.Context) error {
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := resource.PruneRDBMS(nsId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to prune RDBMS metadata")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestPostRDBMSDatabase godoc
// @ID PostRDBMSDatabase
// @Summary Create a logical database inside an RDBMS instance
// @Description Creates a logical database inside an Available RDBMS instance via CB-Spider.
// @Description adminUserPassword (the instance's admin login password) is required in the
// @Description request body and is never persisted by Tumblebug (see
// @Description docs/feature_guide/rdbms-management.md's Features section) — forwarded to
// @Description CB-Spider for this call only.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param rdbmsId path string true "RDBMS ID" default(rdbms-01)
// @Param reqBody body model.RDBMSDatabaseCreateReq true "Database Create Request"
// @Success 200 {object} model.RDBMSDatabaseInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/{rdbmsId}/database [post]
func RestPostRDBMSDatabase(c echo.Context) error {
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

	req := model.RDBMSDatabaseCreateReq{}
	if err := c.Bind(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind request body to RDBMSDatabaseCreateReq")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := resource.CreateRDBMSDatabase(nsId, rdbmsId, req)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create database in RDBMS '%s'", rdbmsId)
		return c.JSON(apierr.Code(err), model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestGetRDBMSDatabases godoc
// @ID GetRDBMSDatabases
// @Summary List logical databases inside an RDBMS instance
// @Description Lists logical databases inside an RDBMS instance via CB-Spider.
// @Description X-Admin-User-Password is optional here — CB-Spider's own database-test
// @Description results show list succeeding without one for at least some drivers; supply it
// @Description if the connection's driver requires direct SQL access (see
// @Description docs/feature_guide/rdbms-management.md's Features section).
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param rdbmsId path string true "RDBMS ID" default(rdbms-01)
// @Param X-Admin-User-Password header string false "RDBMS instance admin login password (optional for list; not persisted by Tumblebug)"
// @Success 200 {object} model.RDBMSDatabaseListResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/{rdbmsId}/database [get]
func RestGetRDBMSDatabases(c echo.Context) error {
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

	adminUserPassword := c.Request().Header.Get("X-Admin-User-Password")

	result, err := resource.ListRDBMSDatabases(nsId, rdbmsId, adminUserPassword)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to list databases in RDBMS '%s'", rdbmsId)
		return c.JSON(apierr.Code(err), model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestDeleteRDBMSDatabase godoc
// @ID DeleteRDBMSDatabase
// @Summary Delete a logical database inside an RDBMS instance
// @Description Deletes a logical database inside an RDBMS instance via CB-Spider.
// @Description X-Admin-User-Password is required — Tumblebug does not persist the RDBMS
// @Description instance's admin login password (see docs/feature_guide/rdbms-management.md's
// @Description Features section), so the caller must resupply it on every call.
// @Tags [Infra Resource] RDBMS Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param rdbmsId path string true "RDBMS ID" default(rdbms-01)
// @Param dbName path string true "Database Name" default(sampledb)
// @Param X-Admin-User-Password header string true "RDBMS instance admin login password (required; not persisted by Tumblebug)"
// @Success 204 "No Content"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/resources/rdbms/{rdbmsId}/database/{dbName} [delete]
func RestDeleteRDBMSDatabase(c echo.Context) error {
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
	dbName := c.Param("dbName")
	if dbName == "" {
		err := fmt.Errorf("dbName is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	adminUserPassword := c.Request().Header.Get("X-Admin-User-Password")
	if adminUserPassword == "" {
		err := fmt.Errorf("X-Admin-User-Password header is required; Tumblebug does not persist the RDBMS instance's admin login password (see docs/feature_guide/rdbms-management.md's Features section)")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	if err := resource.DeleteRDBMSDatabase(nsId, rdbmsId, dbName, adminUserPassword); err != nil {
		log.Error().Err(err).Msgf("Failed to delete database '%s' in RDBMS '%s'", dbName, rdbmsId)
		return c.JSON(apierr.Code(err), model.SimpleMsg{Message: err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
