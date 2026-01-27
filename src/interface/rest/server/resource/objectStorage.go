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
package resource

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ========== Resource APIs: Object Storage ==========

// RestGetObjectStorageSupport godoc
// @ID GetObjectStorageSupport
// @Summary Get CSP support information for object storage features
// @Description Get CSP support information for object storage features (CORS, Versioning)
// @Description If cspType query parameter is provided, returns support information for that specific CSP
// @Description If cspType is not provided, returns support information for all CSPs
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param cspType query string false "CSP Type (e.g., aws, gcp, azure, alibaba, tencent, ibm, openstack, ncp, nhn, kt)" Enums(aws, gcp, azure, alibaba, tencent, ibm, openstack, ncp, nhn, kt)
// @Success 200 {object} model.ObjectStorageSupportResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /objectStorage/support [get]
func RestGetObjectStorageSupport(c echo.Context) error {

	// [Input]
	cspType := c.QueryParam("cspType")

	// [Process]
	result, err := resource.GetObjectStorageSupport(cspType)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get object storage support information")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, result)
}

// RestCreateObjectStorage godoc
// @ID CreateObjectStorage
// @Summary Create an object storage (bucket)
// @Description Create an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param reqBody body model.ObjectStorageCreateRequest true "Object Storage Create Request"
// @Success 200 {object} model.ObjectStorageInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 409 {object} model.SimpleMsg "Conflict"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage [put]
func RestCreateObjectStorage(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	req := model.ObjectStorageCreateRequest{}
	if err := c.Bind(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind request body to ObjectStorageCreateRequest")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	// Perform the operation
	result, err := resource.CreateObjectStorage(nsId, req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return c.JSON(http.StatusConflict, model.SimpleMsg{Message: err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestListObjectStorages godoc
// @ID ListObjectStorages
// @Summary List object storages (buckets)
// @Description Get the list of object storages (buckets)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: cspResourceName)"
// @Param filterVal query string false "Field value for filtering (ex: default-alibaba-ap-northeast-1-vpc)"
// @Success 200 {object} JSONResult{[DEFAULT]=model.ObjectStorageListResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/objectStorage [get]
func RestListObjectStorages(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestGetObjectStorage godoc
// @ID GetObjectStorage
// @Summary Get details of an object storage (bucket)
// @Description Get details of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 200 {object} model.ObjectStorageInfo "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Router /ns/{nsId}/resources/objectStorage/{osId} [get]
func RestGetObjectStorage(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	result, err := resource.GetObjectStorage(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestCheckObjectStorage godoc
// @ID CheckObjectStorage
// @Summary Check existence of an object storage (bucket)
// @Description Check existence of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 200 "OK"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Router /ns/{nsId}/resources/objectStorage/{osId} [head]
func RestCheckObjectStorageExistance(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	exists, err := resource.CheckObjectStorageExistence(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	if !exists {
		return c.JSON(http.StatusNotFound, model.SimpleMsg{Message: "Object Storage does not exist"})
	}

	return c.NoContent(http.StatusOK)
}

// RestGetObjectStorageLocation godoc
// @ID GetObjectStorageLocation
// @Summary Get the location of an object storage (bucket)
// @Description Get the location of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 200 {object} model.ObjectStorageLocationResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/location [get]
func RestGetObjectStorageLocation(c echo.Context) error {

	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	result, err := resource.GetObjectStorageLocation(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// RestDeleteObjectStorage godoc
// @ID RestDeleteObjectStorage
// @Summary Delete an object storage (bucket)
// @Description Delete an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 204 "No Content"
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/objectStorage/{osId} [delete]
func RestDeleteObjectStorage(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	err := resource.DeleteObjectStorage(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete object storage")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.NoContent(http.StatusNoContent)
}

/*
 * Object Storage management - CORS
 */

// RestSetObjectStorageCORS godoc
// @ID SetObjectStorageCORS
// @Summary Set CORS configuration of an object storage (bucket)
// @Description Set CORS configuration of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Param reqBody body model.ObjectStorageSetCorsRequest true "CORS Configuration Request"
// @Success 200 "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/cors [put]
func RestSetObjectStorageCORS(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	req := model.ObjectStorageSetCorsRequest{}
	if err := c.Bind(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind request body to SetCorsConfigurationRequest")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	err := resource.SetObjectStorageCorsConfigurations(nsId, osId, req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to set CORS configuration")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.NoContent(http.StatusOK)
}

// RestGetObjectStorageCORS godoc
// @ID GetObjectStorageCORS
// @Summary Get CORS configuration of an object storage (bucket)
// @Description Get CORS configuration of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 200 {object} model.ObjectStorageGetCorsResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/cors [get]
func RestGetObjectStorageCORS(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	result, err := resource.GetObjectStorageCorsConfigurations(nsId, osId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{Message: err.Error()})
		}
		log.Error().Err(err).Msg("Failed to get CORS configuration")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, result)
}

// RestDeleteObjectStorageCORS godoc
// @ID DeleteObjectStorageCORS
// @Summary Delete CORS configuration of an object storage (bucket)
// @Description Delete all CORS rules of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 204 "No Content"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/cors [delete]
func RestDeleteObjectStorageCORS(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	err := resource.DeleteObjectStorageCorsConfigurations(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete CORS configuration")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.NoContent(http.StatusNoContent)
}

/*
 * Object Storage Management - Versioning
 */

// RestSetObjectStorageVersioning godoc
// @ID SetObjectStorageVersioning
// @Summary Set versioning configuration of an object storage (bucket)
// @Description Set versioning configuration of an object storage (bucket)
// @Description
// @Description **Note: **
// @Description - Versioning options: "Enabled", "Suspended", "Unversioned"
// @Description
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Param reqBody body model.ObjectStorageSetVersioningRequest true "Versioning Configuration Request"
// @Success 200 "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/versioning [put]
func RestSetObjectStorageVersioning(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	req := model.ObjectStorageSetVersioningRequest{}
	if err := c.Bind(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind request body to ObjectStorageSetVersioningRequest")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	err := resource.SetObjectStorageVersioning(nsId, osId, req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to set versioning configuration")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.NoContent(http.StatusOK)
}

// RestGetObjectStorageVersioning godoc
// @ID GetObjectStorageVersioning
// @Summary Get versioning configuration of an object storage (bucket)
// @Description Get versioning configuration of an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 200 {object} model.ObjectStorageGetVersioningResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/versioning [get]
func RestGetObjectStorageVersioning(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	result, err := resource.GetObjectStorageVersioning(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get versioning configuration")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, result)
}

// RestListObjectVersions godoc
// @ID ListObjectVersions
// @Summary List object versions in an object storage (bucket)
// @Description List all versions of objects in an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Success 200 {object} model.ObjectStorageListObjectVersionsResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/versions [get]
func RestListObjectVersions(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	result, err := resource.ListObjectVersions(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list object versions")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, result)
}

// RestDeleteVersionedObject godoc
// @ID DeleteVersionedObject
// @Summary Delete a specific version of an object
// @Description Delete a specific version of an object in an object storage (bucket)
// @Description
// @Description **Note: **
// @Description - If no version is specified, we will define how it behaves and update it when necessary.
// @Description
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Param objectKey path string true "Object Key"
// @Param versionId query string true "Version ID"
// @Success 204 "No Content"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/versions/{objectKey} [delete]
func RestDeleteVersionedObject(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("objectKey is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	versionId := c.QueryParam("versionId")
	if versionId == "" {
		err := fmt.Errorf("versionId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	err := resource.DeleteVersionedObject(nsId, osId, objectKey, versionId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete versioned object")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.NoContent(http.StatusNoContent)
}

/*
 * Object operations
 */

// RestGeneratePresignedURL godoc
// @ID GeneratePresignedURL
// @Summary Generate a presigned URL for uploading or downloading an object
// @Description Generate a presigned URL for uploading  or downloading an object to an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The generated presigned URL can be used to upload the object directly without further authentication
// @Description - The expiration time is specified in seconds (default: 3600 seconds)
// @Description
// @Description **Example Usage: Upload**
// @Description ```bash
// @Description # Using the presigned URL to upload a file
// @Description curl -i -H "Content-Type: text/plain" -X PUT "<PRESIGNED_URL>" --data-binary "@local-file.txt"
// @Description ```
// @Description
// @Description **Example Usage: download**
// @Description ```bash
// @Description # Using the presigned URL to download a file
// @Description curl -X GET "<PRESIGNED_URL>" -o downloaded-file.txt
// @Description ```
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Param objectKey path string true "Object Key"
// @Param operation query string false "Operation type" Enums(upload, download)
// @Param expires query int false "Expiration time in seconds" default(3600)
// @Success 200 {object} model.ObjectStoragePresignedUrlResponse "OK"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/object/{objectKey} [get]
func RestGeneratePresignedURL(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("objectKey is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	operation := c.QueryParam("operation")
	if operation == "" {
		err := fmt.Errorf("operation is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Parse expires from query parameter (default: 3600 seconds)
	expiresStr := c.QueryParam("expires")
	expiresSeconds := 3600 // default
	if expiresStr != "" {
		if parsed, err := fmt.Sscanf(expiresStr, "%d", &expiresSeconds); err != nil || parsed != 1 {
			log.Warn().Str("expires", expiresStr).Msg("Invalid expires parameter, using default 3600")
			expiresSeconds = 3600
		}
	}

	// [Process]
	result, err := resource.GeneratePresignedURL(nsId, osId, objectKey, time.Duration(expiresSeconds)*time.Second, operation)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to generate presigned %s URL", operation)
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, result)
}

// RestListDataObjects godoc
// @ID ListDataObjects
// @Summary List objects in an object storage (bucket)
// @Description List all objects in an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// // @Param prefix query string false "Filter objects by prefix" default()
// // @Param maxKeys query int false "Maximum number of keys to return" default(1000)
// @Success 200 {object} model.ObjectStorageListObjectsResponse "OK - Returns object storage info with contents"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/object [get]
func RestListDataObjects(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// // Optional query parameters
	// prefix := c.QueryParam("prefix")
	// maxKeysStr := c.QueryParam("maxKeys")
	// maxKeys := 1000 // default
	// if maxKeysStr != "" {
	// 	if parsed, err := fmt.Sscanf(maxKeysStr, "%d", &maxKeys); err != nil || parsed != 1 {
	// 		log.Warn().Str("maxKeys", maxKeysStr).Msg("Invalid maxKeys parameter, using default 1000")
	// 		maxKeys = 1000
	// 	}
	// }

	// [Process]
	result, err := resource.ListDataObjects(nsId, osId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.JSON(http.StatusOK, result)
}

// RestGetDataObjectInfo godoc
// @ID GetDataObjectInfo
// @Summary Get object info from an object storage (bucket)
// @Description Get object info from an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - This API retrieves the metadata of an object without downloading the actual content
// @Description - Returns metadata in response headers (Content-Length, Content-Type, ETag, Last-Modified)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Param objectKey path string true "Object Key"
// @Success 200 "OK - Object metadata returned in headers"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/object/{objectKey} [head]
func RestGetDataObjectInfo(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("objectKey is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	result, err := resource.GetDataObject(nsId, osId, objectKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output] Set metadata in response headers
	if result.ETag != "" {
		c.Response().Header().Set("ETag", result.ETag)
	}
	if result.LastModified != "" {
		c.Response().Header().Set("Last-Modified", result.LastModified)
	}

	return c.NoContent(http.StatusOK)
}

// RestDeleteDataObject godoc
// @ID DeleteDataObject
// @Summary Delete an object from an object storage (bucket)
// @Description Delete an object from an object storage (bucket)
// @Tags [Infra Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param nsId path string true "Namespace ID" default(default)
// @Param osId path string true "Object Storage ID" default(os01)
// @Param objectKey path string true "Object Key"
// @Success 204 "No Content"
// @Failure 400 {object} model.SimpleMsg "Bad Request"
// @Failure 404 {object} model.SimpleMsg "Not Found"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /ns/{nsId}/resources/objectStorage/{osId}/object/{objectKey} [delete]
func RestDeleteDataObject(c echo.Context) error {

	// [Input]
	nsId := c.Param("nsId")
	if nsId == "" {
		err := fmt.Errorf("nsId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	osId := c.Param("osId")
	if osId == "" {
		err := fmt.Errorf("osId is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("objectKey is required")
		log.Warn().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// [Process]
	err := resource.DeleteDataObject(nsId, osId, objectKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete object")
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	// [Output]
	return c.NoContent(http.StatusNoContent)
}
