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

type RestListObjectStorageResponse struct {
	ObjectStorage []model.ObjectStorageInfo `json:"objectStorage"`
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
// @Success 200 {object} JSONResult{[DEFAULT]=RestListObjectStorageResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/objectStorage [get]
func RestListObjectStorages(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// // RestListObjectStorages godoc
// // @ID ListObjectStorages
// // @Summary List object storages (buckets)
// // @Description Get the list of object storages (buckets)
// // @Tags [Infra Resource] Object Storage Management
// // @Accept json
// // @Produce json
// // @Param nsId path string true "Namespace ID" default(default)
// // @Success 200 {object} model.ObjectStorageListResponse "OK"
// // @Failure 400 {object} model.SimpleMsg "Bad Request"
// // @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// // @Router /ns/{nsId}/resources/objectStorage [get]
// func RestListObjectStorages(c echo.Context) error {

// 	// [Input]
// 	nsId := c.Param("nsId")
// 	if nsId == "" {
// 		err := fmt.Errorf("nsId is required")
// 		log.Warn().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// [Process]
// 	result, err := resource.ListObjectStorages(nsId, req)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
// 	}

// 	return c.JSON(http.StatusOK, result)
// }

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
// @Success 200 {object} model.SimpleMsg
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
	msg := fmt.Sprintf("The object storage '%s' has been deleted.", osId)
	simpleMsg := model.SimpleMsg{
		Message: msg,
	}

	return c.JSON(http.StatusOK, simpleMsg)
}

// /*
//  * Object Storage Operations - Versioning
//  */

// // Note: The xmlns attribute and root element name may not be accurately
// // represented in Swagger UI due to XML rendering limitations.

// // <?xml version="1.0" encoding="UTF-8"?>
// // <VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// //   <Status>Enabled</Status>
// // </VersioningConfiguration>

// type VersioningConfiguration struct {
// 	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
// 	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
// 	Status string `xml:"Status" json:"status" example:"Enabled"`
// }

// // RestGetObjectStorageVersioningLagacy godoc
// // @ID GetObjectStorageVersioningLagacy
// // @Summary (To be deprecated) Get versioning status of an object storage (bucket)
// // @Description (To be deprecated) Get versioning status of an object storage (bucket)
// // @Description
// // @Description **Important Notes:**
// // @Description - The actual response will be XML format with root element `VersioningConfiguration`
// // @Description
// // @Description **Actual XML Response Example:**
// // @Description ```xml
// // @Description <?xml version="1.0" encoding="UTF-8"?>
// // @Description <VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// // @Description   <Status>Enabled</Status>
// // @Description </VersioningConfiguration>
// // @Description ```
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Success 200 {object} VersioningConfiguration "OK"
// // @Router /resources/objectStorage/{objectStorageName}/versioning [get]
// func GetObjectStorageVersioningLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Source path pattern with * to capture objectStorageName
// 	sourcePattern := "/resources/objectStorage/*/versioning"
// 	// Target path pattern using $1 for captured objectStorageName
// 	targetPattern := "/s3/$1?versioning"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

// // RestSetObjectStorageVersioningLagacy godoc
// // @ID SetObjectStorageVersioningLagacy
// // @Summary (To be deprecated) Set versioning status of an object storage (bucket)
// // @Description (To be deprecated) Set versioning status of an object storage (bucket)
// // @Description
// // @Description **Important Notes:**
// // @Description - The request body must be XML format with root element `VersioningConfiguration`
// // @Description - The `Status` field can be either `Enabled` or `Suspended`
// // @Description
// // @Description **Request Body Example:**
// // @Description ```xml
// // @Description <?xml version="1.0" encoding="UTF-8"?>
// // @Description <VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// // @Description   <Status>Enabled</Status>
// // @Description </VersioningConfiguration>
// // @Description ```
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Param reqBody body VersioningConfiguration true "Versioning Configuration"
// // @Success 200 "OK"
// // @Router /resources/objectStorage/{objectStorageName}/versioning [put]
// func SetObjectStorageVersioningLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}
// 	// Source path pattern with * to capture objectStorageName
// 	sourcePattern := "/resources/objectStorage/*/versioning"
// 	// Target path pattern using $1 for captured objectStorageName
// 	targetPattern := "/s3/$1?versioning"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

// // Note: The xmlns attribute and root element name may not be accurately
// // represented in Swagger UI due to XML rendering limitations.

// // <?xml version="1.0" encoding="UTF-8"?>
// // <ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// //   <Name>spider-test-bucket</Name>
// //   <Prefix></Prefix>
// //   <KeyMarker></KeyMarker>
// //   <VersionIdMarker></VersionIdMarker>
// //   <NextKeyMarker></NextKeyMarker>
// //   <NextVersionIdMarker></NextVersionIdMarker>
// //   <MaxKeys>1000</MaxKeys>
// //   <IsTruncated>false</IsTruncated>
// //   <Version>
// //     <Key>test-file.txt</Key>
// //     <VersionId>yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ</VersionId>
// //     <IsLatest>true</IsLatest>
// //     <LastModified>2025-09-04T04:24:12Z</LastModified>
// //     <ETag>23228a38faecd0591107818c7281cece</ETag>
// //     <Size>23</Size>
// //     <StorageClass>STANDARD</StorageClass>
// //     <Owner>
// //       <ID>aws-config01</ID>
// //       <DisplayName>aws-config01</DisplayName>
// //     </Owner>
// //   </Version>
// // </ListVersionsResult>

// type ListVersionsResult struct {
// 	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
// 	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
// 	Name                string  `xml:"Name" json:"name" example:"spider-test-bucket"`
// 	Prefix              string  `xml:"Prefix" json:"prefix" example:""`
// 	KeyMarker           string  `xml:"KeyMarker" json:"keyMarker" example:""`
// 	VersionIdMarker     string  `xml:"VersionIdMarker" json:"versionIdMarker" example:""`
// 	NextKeyMarker       string  `xml:"NextKeyMarker" json:"nextKeyMarker" example:""`
// 	NextVersionIdMarker string  `xml:"NextVersionIdMarker" json:"nextVersionIdMarker" example:""`
// 	MaxKeys             int     `xml:"MaxKeys" json:"maxKeys" example:"1000"`
// 	IsTruncated         bool    `xml:"IsTruncated" json:"isTruncated" example:"false"`
// 	Version             Version `xml:"Version" json:"version"`
// }

// type Version struct {
// 	Key          string `xml:"Key" json:"key" example:"test-file.txt"`
// 	VersionId    string `xml:"VersionId" json:"versionId" example:"yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ"`
// 	IsLatest     bool   `xml:"IsLatest" json:"isLatest" example:"true"`
// 	LastModified string `xml:"LastModified" json:"lastModified" example:"2025-09-04T04:24:12Z"`
// 	ETag         string `xml:"ETag" json:"etag" example:"23228a38faecd0591107818c7281cece"`
// 	Size         int    `xml:"Size" json:"size" example:"23"`
// 	StorageClass string `xml:"StorageClass" json:"storageClass" example:"STANDARD"`
// 	Owner        Owner  `xml:"Owner" json:"owner"`
// }

// // RestListObjectVersionsLagacy godoc
// // @ID ListObjectVersionsLagacy
// // @Summary (To be deprecated) List object versions in an object storage (bucket)
// // @Description (To be deprecated) List object versions in an object storage (bucket)
// // @Description
// // @Description **Important Notes:**
// // @Description - The actual response will be XML format with root element `ListVersionsResult`
// // @Description
// // @Description **Actual XML Response Example:**
// // @Description ```xml
// // @Description <?xml version="1.0" encoding="UTF-8"?>
// // @Description <ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// // @Description   <Name>spider-test-bucket</Name>
// // @Description   <Prefix></Prefix>
// // @Description   <KeyMarker></KeyMarker>
// // @Description   <VersionIdMarker></VersionIdMarker>
// // @Description   <NextKeyMarker></NextKeyMarker>
// // @Description   <NextVersionIdMarker></NextVersionIdMarker>
// // @Description   <MaxKeys>1000</MaxKeys>
// // @Description   <IsTruncated>false</IsTruncated>
// // @Description   <Version>
// // @Description     <Key>test-file.txt</Key>
// // @Description     <VersionId>yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ</VersionId>
// // @Description     <IsLatest>true</IsLatest>
// // @Description     <LastModified>2025-09-04T04:24:12Z</LastModified>
// // @Description     <ETag>23228a38faecd0591107818c7281cece</ETag>
// // @Description     <Size>23</Size>
// // @Description     <StorageClass>STANDARD</StorageClass>
// // @Description     <Owner>
// // @Description       <ID>aws-config01</ID>
// // @Description       <DisplayName>aws-config01</DisplayName>
// // @Description     </Owner>
// // @Description   </Version>
// // @Description </ListVersionsResult>
// // @Description ```
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Success 200 {object} ListVersionsResult "OK"
// // @Router /resources/objectStorage/{objectStorageName}/versions [get]
// func ListObjectVersionsLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Source path pattern with * to capture objectStorageName
// 	sourcePattern := "/resources/objectStorage/*/versions"
// 	// Target path pattern using $1 for captured objectStorageName
// 	targetPattern := "/s3/$1?versions"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

// // RestDeleteVersionedObjectLagacy godoc
// // @ID DeleteVersionedObjectLagacy
// // @Summary (To be deprecated) Delete a specific version of an object in an object storage (bucket)
// // @Description (To be deprecated) Delete a specific version of an object in an object storage (bucket)
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param objectKey path string true "Object Key" default(test-file.txt)
// // @Param versionId query string true "Version ID" default(yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Success 204 "No Content"
// // @Router /resources/objectStorage/{objectStorageName}/versions/{objectKey} [delete]
// func DeleteVersionedObjectLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}
// 	// Validate objectKey parameter
// 	objectKey := c.Param("objectKey")
// 	if objectKey == "" {
// 		err := fmt.Errorf("%s", "objectKey is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}
// 	// Validate versionId parameter
// 	versionId := c.QueryParam("versionId")
// 	if versionId == "" {
// 		err := fmt.Errorf("%s", "versionId is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Source path pattern with * to capture objectStorageName and objectKey
// 	sourcePattern := "/resources/objectStorage/*/versions/*?versionId=*"
// 	// Target path pattern using $1 for captured objectStorageName, $2 for objectKey, and $3 for versionId
// 	targetPattern := "/s3/$1/$2?versionId=$3"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

// /*
//  * Object Storage Operations - CORS
//  */

// // Note: The xmlns attribute and root element name may not be accurately
// // represented in Swagger UI due to XML rendering limitations.

// // <?xml version="1.0" encoding="UTF-8"?>
// // <CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// //   <CORSRule>
// //     <AllowedOrigin>*</AllowedOrigin>
// //     <AllowedMethod>GET</AllowedMethod>
// //     <AllowedMethod>PUT</AllowedMethod>
// //     <AllowedMethod>POST</AllowedMethod>
// //     <AllowedMethod>DELETE</AllowedMethod>
// //     <AllowedHeader>*</AllowedHeader>
// //     <ExposeHeader>ETag</ExposeHeader>
// //     <ExposeHeader>x-amz-server-side-encryption</ExposeHeader>
// //     <ExposeHeader>x-amz-request-id</ExposeHeader>
// //     <ExposeHeader>x-amz-id-2</ExposeHeader>
// //     <MaxAgeSeconds>3000</MaxAgeSeconds>
// //   </CORSRule>
// // </CORSConfiguration>

// type CORSRule struct {
// 	AllowedOrigin []string `xml:"AllowedOrigin" json:"allowedOrigin" example:"*"`
// 	AllowedMethod []string `xml:"AllowedMethod" json:"allowedMethod" example:"GET"`
// 	AllowedHeader []string `xml:"AllowedHeader" json:"allowedHeader" example:"*"`
// 	ExposeHeader  []string `xml:"ExposeHeader" json:"exposeHeader" example:"ETag"`
// 	MaxAgeSeconds int      `xml:"MaxAgeSeconds" json:"maxAgeSeconds" example:"3000"`
// }

// type CORSConfiguration struct {
// 	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
// 	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
// 	CORSRule []CORSRule `xml:"CORSRule" json:"corsRule"`
// }

// type Error struct {
// 	Code      string `xml:"Code" json:"code" example:"NoSuchCORSConfiguration"`
// 	Message   string `xml:"Message" json:"message" example:"The CORS configuration does not exist"`
// 	Resource  string `xml:"Resource" json:"resource" example:"/example-bucket"`
// 	RequestId string `xml:"RequestId" json:"requestId" example:"656c76696e6727732072657175657374"`
// }

// // RestGetObjectStorageCORSLagacy
// // @ID GetObjectStorageCORSLagacy
// // @Summary (To be deprecated) Get CORS configuration of an object storage (bucket)
// // @Description (To be deprecated) Get CORS configuration of an object storage (bucket)
// // @Description
// // @Description **Important Notes:**
// // @Description - The actual response will be XML format with root element `CORSConfiguration`
// // @Description
// // @Description **Actual XML Response Example:**
// // @Description ```xml
// // @Description <?xml version="1.0" encoding="UTF-8"?>
// // @Description <CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// // @Description   <CORSRule>
// // @Description     <AllowedOrigin>*</AllowedOrigin>
// // @Description     <AllowedMethod>GET</AllowedMethod>
// // @Description     <AllowedMethod>PUT</AllowedMethod>
// // @Description     <AllowedMethod>POST</AllowedMethod>
// // @Description     <AllowedMethod>DELETE</AllowedMethod>
// // @Description     <AllowedHeader>*</AllowedHeader>
// // @Description     <ExposeHeader>ETag</ExposeHeader>
// // @Description     <ExposeHeader>x-amz-server-side-encryption</ExposeHeader>
// // @Description     <ExposeHeader>x-amz-request-id</ExposeHeader>
// // @Description     <ExposeHeader>x-amz-id-2</ExposeHeader>
// // @Description     <MaxAgeSeconds>3000</MaxAgeSeconds>
// // @Description   </CORSRule>
// // @Description </CORSConfiguration>
// // @Description ```
// // @Description
// // @Description **Error Response Example (if CORS not configured):**
// // @Description ```xml
// // @Description <?xml version="1.0" encoding="UTF-8"?>
// // @Description <Error>
// // @Description   <Code>NoSuchCORSConfiguration</Code>
// // @Description   <Message>The CORS configuration does not exist</Message>
// // @Description   <Resource>/example-bucket</Resource>
// // @Description   <RequestId>656c76696e6727732072657175657374</RequestId>
// // @Description </Error>
// // @Description ```
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Success 200 {object} CORSConfiguration "OK"
// // @Failure 404 {object} Error "Not Found"
// // @Router /resources/objectStorage/{objectStorageName}/cors [get]
// func GetObjectStorageCORSLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Source path pattern with * to capture objectStorageName
// 	sourcePattern := "/resources/objectStorage/*/cors"
// 	// Target path pattern using $1 for captured objectStorageName
// 	targetPattern := "/s3/$1?cors"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

// // RestSetObjectStorageCORSLagacy godoc
// // @ID SetObjectStorageCORSLagacy
// // @Summary (To be deprecated) Set CORS configuration of an object storage (bucket)
// // @Description (To be deprecated) Set CORS configuration of an object storage (bucket)
// // @Description
// // @Description **Important Notes:**
// // @Description - The CORS configuration must be provided in the request body in XML format.
// // @Description - The actual request body should have root element `CORSConfiguration`
// // @Description
// // @Description **Actual XML Request Body Example:**
// // @Description ```xml
// // @Description <?xml version="1.0" encoding="UTF-8"?>
// // @Description <CORSConfiguration>
// // @Description   <CORSRule>
// // @Description     <AllowedOrigin>https://example.com</AllowedOrigin>
// // @Description     <AllowedOrigin>https://app.example.com</AllowedOrigin>
// // @Description     <AllowedMethod>GET</AllowedMethod>
// // @Description     <AllowedMethod>PUT</AllowedMethod>
// // @Description     <AllowedHeader>Content-Type</AllowedHeader>
// // @Description     <AllowedHeader>Authorization</AllowedHeader>
// // @Description     <ExposeHeader>ETag</ExposeHeader>
// // @Description     <MaxAgeSeconds>1800</MaxAgeSeconds>
// // @Description   </CORSRule>
// // @Description   <CORSRule>
// // @Description     <AllowedOrigin>*</AllowedOrigin>
// // @Description     <AllowedMethod>GET</AllowedMethod>
// // @Description     <MaxAgeSeconds>300</MaxAgeSeconds>
// // @Description   </CORSRule>
// // @Description </CORSConfiguration>
// // @Description ```
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Param reqBody body CORSConfiguration true "CORS Configuration in XML format"
// // @Success 200 "OK"
// // @Router /resources/objectStorage/{objectStorageName}/cors [put]
// func SetObjectStorageCORSLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Source path pattern with * to capture objectStorageName
// 	sourcePattern := "/resources/objectStorage/*/cors"
// 	// Target path pattern using $1 for captured objectStorageName
// 	targetPattern := "/s3/$1?cors"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

// // RestDeleteObjectStorageCORSLagacy godoc
// // @ID DeleteObjectStorageCORSLagacy
// // @Summary (To be deprecated) Delete CORS configuration of an object storage (bucket)
// // @Description (To be deprecated) Delete CORS configuration of an object storage (bucket)
// // @Tags [Infra Resource] Object Storage Management
// // @Accept xml
// // @Produce xml
// // @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// // @Param credential header string true "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name)." default(aws-ap-northeast-2)
// // @Success 204 "No Content"
// // @Router /resources/objectStorage/{objectStorageName}/cors [delete]
// func DeleteObjectStorageCORSLagacy(c echo.Context) error {

// 	// Validate objectStorageName parameter
// 	objectStorageName := c.Param("objectStorageName")
// 	if objectStorageName == "" {
// 		err := fmt.Errorf("%s", "objectStorageName is required")
// 		log.Error().Err(err).Msg("")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Validate credential header
// 	credentialHeader := c.Request().Header.Get("credential")
// 	err := validateCredential(credentialHeader)
// 	if err != nil {
// 		log.Error().Err(err).Msg("invalid credential header")
// 		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
// 	}

// 	// Source path pattern with * to capture objectStorageName
// 	sourcePattern := "/resources/objectStorage/*/cors"
// 	// Target path pattern using $1 for captured objectStorageName
// 	targetPattern := "/s3/$1?cors"

// 	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
// 	return proxyHandler(c)
// }

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
// @Param expiry query int false "Expiration time in seconds" default(3600)
// @Success 200 {object} model.PresignedUrlResponse "OK"
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

	// Parse expiry from query parameter (default: 3600 seconds)
	expiryStr := c.QueryParam("expiry")
	expirySeconds := 3600 // default
	if expiryStr != "" {
		if parsed, err := fmt.Sscanf(expiryStr, "%d", &expirySeconds); err != nil || parsed != 1 {
			log.Warn().Str("expiry", expiryStr).Msg("Invalid expiry parameter, using default 3600")
			expirySeconds = 3600
		}
	}

	// [Process]
	result, err := resource.GeneratePresignedURL(nsId, osId, objectKey, time.Duration(expirySeconds)*time.Second, operation)
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
// @Success 200 {object} model.ListObjectResponse "OK - Returns object storage info with contents"
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
