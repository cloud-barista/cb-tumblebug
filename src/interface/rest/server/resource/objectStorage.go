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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// createSpiderProxyHandler creates a handler function that proxies requests to Spider API
// sourcePattern: the source URL pattern with wildcards (*)
// targetPattern: the target URL pattern with wildcards ($1, $2, etc.)
func createSpiderProxyHandler(sourcePattern string, targetPattern string) echo.HandlerFunc {

	// Parse Spider endpoint URL
	spURL, err := url.Parse(model.SpiderRestUrl)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse Spider endpoint URL")
		return func(c echo.Context) error {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Server configuration error",
			})
		}
	}

	// Return a handler that processes the request and forwards it to Tumblebug
	return func(c echo.Context) error {
		// Create the rewrite rule based on the source and target patterns
		rewriteRules := map[string]string{
			sourcePattern: targetPattern,
		}

		log.Debug().Msgf("Incoming request: %s %s", c.Request().Method, c.Request().URL.String())
		log.Debug().Msgf("[Proxy] Request to %s with rewrite rule: %s -> %s", spURL.String(), sourcePattern, targetPattern)
		// log.Debug().Msgf("Proxying with rewrite rule: %s -> %s", sourcePattern, targetPattern)

		// Use the existing Proxy middleware
		proxyMiddleware := middlewares.Proxy(middlewares.ProxyConfig{
			URL:     spURL,
			Rewrite: rewriteRules,
			ModifyResponse: func(res *http.Response) error {
				resBytes, err := io.ReadAll(res.Body)
				if err != nil {
					return err
				}

				log.Debug().Msgf("[Proxy] Response from %s", res.Request.URL)
				log.Trace().Msgf("[Proxy] Response body: %s", string(resBytes))

				res.Body = io.NopCloser(bytes.NewReader(resBytes))
				return nil
			},
		})

		// Create a handler that will be wrapped by the proxy middleware
		handler := echo.HandlerFunc(func(c echo.Context) error {
			return nil // This will never be called because the proxy middleware will handle the request
		})

		// Apply the proxy middleware to the handler
		return proxyMiddleware(handler)(c)
	}
}

func validate(provider, region string) error {

	providers, err := common.GetProviderList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get provider list")
		return err
	}

	found := false
	for _, p := range providers.IdList {
		if p == provider {
			// Valid provider found, break the loop
			found = true
			break
		}
	}
	if !found {
		err := fmt.Errorf("invalid provider: %s", provider)
		log.Error().Err(err).Msg("Invalid provider")
		return err
	}

	// Validate region
	log.Debug().Msgf("Requested region: %s", region)
	_, err = common.GetRegion(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get region info")
		return fmt.Errorf("invalid region: %s", region)
	}

	// Validate connection config existence
	connectionName := fmt.Sprintf("%s-%s", provider, region)
	_, err = common.GetConnConfig(connectionName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get connection config")
		return fmt.Errorf("invalid connection config: %s", connectionName)
	}

	return nil
}

// ========== Resource APIs: Object Storage ==========
// Owner represents the owner information in S3 bucket list response
type Owner struct {
	ID          string `xml:"ID" json:"id" example:"aws-ap-northeast-2"`
	DisplayName string `xml:"DisplayName" json:"displayName" example:"aws-ap-northeast-2"`
}

// Bucket represents a single bucket in S3 bucket list response
type Bucket struct {
	Name         string `xml:"Name" json:"name" example:"spider-test-bucket"`
	CreationDate string `xml:"CreationDate" json:"creationDate" example:"2025-09-04T04:18:06Z"`
}

// Buckets represents the collection of buckets in S3 bucket list response
type Buckets struct {
	Bucket []Bucket `xml:"Bucket" json:"bucket"`
}

type Object struct {
	Key string `xml:"Key" json:"key" example:"test-object.txt"`
}

/*
 * Object Storage Operations
 */

// ListAllMyBucketsResult represents the response structure for S3 ListAllMyBuckets operation
//
// The actual XML response will have the following structure:
// - Root element: ListAllMyBucketsResult
// - Namespace: xmlns="http://s3.amazonaws.com/doc/2006-03-01/"
// - Contains Owner and Buckets elements
//
// Example XML response:
// ```xml
// <?xml version="1.0" encoding="UTF-8"?>
// <ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
//
//	<Owner>
//	  <ID>aws-ap-northeast-2</ID>
//	  <DisplayName>aws-ap-northeast-2</DisplayName>
//	</Owner>
//	<Buckets>
//	</Buckets>
//
// </ListAllMyBucketsResult>
// ```
//
// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.
type ListAllMyBucketsResult struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
	// Owner information for the S3 account
	Owner Owner `xml:"Owner" json:"owner"`
	// Collection of buckets
	Buckets Buckets `xml:"Buckets" json:"buckets"`
}

// RestListObjectStorages godoc
// @ID ListObjectStorages
// @Summary List object storages (buckets)
// @Description Get the list of all object storages (buckets)
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `ListAllMyBucketsResult`
// @Description - The response includes xmlns attribute: `xmlns="http://s3.amazonaws.com/doc/2006-03-01/"`
// @Description - Swagger UI may show `resource.ListAllMyBucketsResult` due to rendering limitations
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Owner>
// @Description     <ID>aws-ap-northeast-2</ID>
// @Description     <DisplayName>aws-ap-northeast-2</DisplayName>
// @Description   </Owner>
// @Description   <Buckets>
// @Description   </Buckets>
// @Description </ListAllMyBucketsResult>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 {object} ListAllMyBucketsResult "OK"
// @Router /resources/objectStorage [get]
func ListObjectStorages(c echo.Context) error {

	// Validate provider
	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture provider and region
	sourcePattern := "/resources/objectStorage?provider=*&region=*"
	// Target path pattern using $1 for captured provider and $2 for captured region
	targetPattern := "/s3?ConnectionName=$1-$2"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestCreateObjectStorage godoc
// @ID CreateObjectStorage
// @Summary Create an object storage (bucket)
// @Description Create an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The `objectStorageName` must be globally unique across all existing buckets in the S3 compatible storage.
// @Description - The bucket namespace is shared by all users of the system.
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 "OK"
// @Router /resources/objectStorage/{objectStorageName} [put]
func CreateObjectStorage(c echo.Context) error {

	// Validate provider
	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.
type ListBucketResult struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
	Name        string `xml:"Name" json:"name" example:"spider-test-bucket"`
	Prefix      string `xml:"Prefix" json:"prefix" example:""`
	Marker      string `xml:"Marker" json:"marker" example:""`
	MaxKeys     int    `xml:"MaxKeys" json:"maxKeys" example:"1000"`
	IsTruncated bool   `xml:"IsTruncated" json:"isTruncated" example:"false"`
}

// RestGetObjectStorage godoc
// @ID GetObjectStorage
// @Summary Get details of an object storage (bucket)
// @Description Get details of an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `ListBucketResult`
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Name>spider-test-bucket</Name>
// @Description   <Prefix></Prefix>
// @Description   <Marker></Marker>
// @Description   <MaxKeys>1000</MaxKeys>
// @Description   <IsTruncated>false</IsTruncated>
// @Description </ListBucketResult>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 {object} ListBucketResult "OK"
// @Router /resources/objectStorage/{objectStorageName} [get]
func GetObjectStorage(c echo.Context) error {

	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestExistObjectStorage godoc
// @ID ExistObjectStorage
// @Summary Check existence of an object storage (bucket)
// @Description Check existence of an object storage (bucket)
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 "OK"
// @Failure 404 "Not Found"
// @Router /resources/objectStorage/{objectStorageName} [head]
func ExistObjectStorage(c echo.Context) error {

	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")
	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.

type LocationConstraint struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
}

// RestGetObjectStorageLocation godoc
// @ID GetObjectStorageLocation
// @Summary Get the location of an object storage (bucket)
// @Description Get the location of an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `LocationConstraint`
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">ap-northeast-2</LocationConstraint>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 {object} LocationConstraint "OK"
// @Router /resources/objectStorage/{objectStorageName}/location [get]
func GetObjectStorageLocation(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")
	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/location?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?location&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestDeleteObjectStorage godoc
// @ID DeleteObjectStorage
// @Summary Delete an object storage (bucket)
// @Description Delete an object storage (bucket)
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 204 "No Content"
// @Router /resources/objectStorage/{objectStorageName} [delete]
func DeleteObjectStorage(c echo.Context) error {

	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

/*
 * Object Storage Operations - Versioning
 */

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.

// <?xml version="1.0" encoding="UTF-8"?>
// <VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
//   <Status>Enabled</Status>
// </VersioningConfiguration>

type VersioningConfiguration struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
	Status string `xml:"Status" json:"status" example:"Enabled"`
}

// RestGetObjectStorageVersioning godoc
// @ID GetObjectStorageVersioning
// @Summary Get versioning status of an object storage (bucket)
// @Description Get versioning status of an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `VersioningConfiguration`
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Status>Enabled</Status>
// @Description </VersioningConfiguration>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 {object} VersioningConfiguration "OK"
// @Router /resources/objectStorage/{objectStorageName}/versioning [get]
func GetObjectStorageVersioning(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/versioning?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?versioning&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestSetObjectStorageVersioning godoc
// @ID SetObjectStorageVersioning
// @Summary Set versioning status of an object storage (bucket)
// @Description Set versioning status of an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The request body must be XML format with root element `VersioningConfiguration`
// @Description - The `Status` field can be either `Enabled` or `Suspended`
// @Description
// @Description **Request Body Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Status>Enabled</Status>
// @Description </VersioningConfiguration>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Param reqBody body VersioningConfiguration true "Versioning Configuration"
// @Success 200 "OK"
// @Router /resources/objectStorage/{objectStorageName}/versioning [put]
func SetObjectStorageVersioning(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/versioning?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?versioning&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.

// <?xml version="1.0" encoding="UTF-8"?>
// <ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
//   <Name>spider-test-bucket</Name>
//   <Prefix></Prefix>
//   <KeyMarker></KeyMarker>
//   <VersionIdMarker></VersionIdMarker>
//   <NextKeyMarker></NextKeyMarker>
//   <NextVersionIdMarker></NextVersionIdMarker>
//   <MaxKeys>1000</MaxKeys>
//   <IsTruncated>false</IsTruncated>
//   <Version>
//     <Key>test-file.txt</Key>
//     <VersionId>yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ</VersionId>
//     <IsLatest>true</IsLatest>
//     <LastModified>2025-09-04T04:24:12Z</LastModified>
//     <ETag>23228a38faecd0591107818c7281cece</ETag>
//     <Size>23</Size>
//     <StorageClass>STANDARD</StorageClass>
//     <Owner>
//       <ID>aws-config01</ID>
//       <DisplayName>aws-config01</DisplayName>
//     </Owner>
//   </Version>
// </ListVersionsResult>

type ListVersionsResult struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
	Name                string  `xml:"Name" json:"name" example:"spider-test-bucket"`
	Prefix              string  `xml:"Prefix" json:"prefix" example:""`
	KeyMarker           string  `xml:"KeyMarker" json:"keyMarker" example:""`
	VersionIdMarker     string  `xml:"VersionIdMarker" json:"versionIdMarker" example:""`
	NextKeyMarker       string  `xml:"NextKeyMarker" json:"nextKeyMarker" example:""`
	NextVersionIdMarker string  `xml:"NextVersionIdMarker" json:"nextVersionIdMarker" example:""`
	MaxKeys             int     `xml:"MaxKeys" json:"maxKeys" example:"1000"`
	IsTruncated         bool    `xml:"IsTruncated" json:"isTruncated" example:"false"`
	Version             Version `xml:"Version" json:"version"`
}

type Version struct {
	Key          string `xml:"Key" json:"key" example:"test-file.txt"`
	VersionId    string `xml:"VersionId" json:"versionId" example:"yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ"`
	IsLatest     bool   `xml:"IsLatest" json:"isLatest" example:"true"`
	LastModified string `xml:"LastModified" json:"lastModified" example:"2025-09-04T04:24:12Z"`
	ETag         string `xml:"ETag" json:"etag" example:"23228a38faecd0591107818c7281cece"`
	Size         int    `xml:"Size" json:"size" example:"23"`
	StorageClass string `xml:"StorageClass" json:"storageClass" example:"STANDARD"`
	Owner        Owner  `xml:"Owner" json:"owner"`
}

// RestListObjectVersions godoc
// @ID ListObjectVersions
// @Summary List object versions in an object storage (bucket)
// @Description List object versions in an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `ListVersionsResult`
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Name>spider-test-bucket</Name>
// @Description   <Prefix></Prefix>
// @Description   <KeyMarker></KeyMarker>
// @Description   <VersionIdMarker></VersionIdMarker>
// @Description   <NextKeyMarker></NextKeyMarker>
// @Description   <NextVersionIdMarker></NextVersionIdMarker>
// @Description   <MaxKeys>1000</MaxKeys>
// @Description   <IsTruncated>false</IsTruncated>
// @Description   <Version>
// @Description     <Key>test-file.txt</Key>
// @Description     <VersionId>yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ</VersionId>
// @Description     <IsLatest>true</IsLatest>
// @Description     <LastModified>2025-09-04T04:24:12Z</LastModified>
// @Description     <ETag>23228a38faecd0591107818c7281cece</ETag>
// @Description     <Size>23</Size>
// @Description     <StorageClass>STANDARD</StorageClass>
// @Description     <Owner>
// @Description       <ID>aws-config01</ID>
// @Description       <DisplayName>aws-config01</DisplayName>
// @Description     </Owner>
// @Description   </Version>
// @Description </ListVersionsResult>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 {object} ListVersionsResult "OK"
// @Router /resources/objectStorage/{objectStorageName}/versions [get]
func ListObjectVersions(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/versions?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?versions&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestDeleteVersionedObject godoc
// @ID DeleteVersionedObject
// @Summary Delete a specific version of an object in an object storage (bucket)
// @Description Delete a specific version of an object in an object storage (bucket)
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param objectKey path string true "Object Key" default(test-file.txt)
// @Param versionId query string true "Version ID" default(yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 204 "No Content"
// @Router /resources/objectStorage/{objectStorageName}/versions/{objectKey} [delete]
func DeleteVersionedObject(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("%s", "objectKey is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	versionId := c.QueryParam("versionId")
	if versionId == "" {
		err := fmt.Errorf("%s", "versionId is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, objectKey, provider, and region
	sourcePattern := "/resources/objectStorage/*/versions/*?versionId=*&provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for objectKey, $3 for versionId, $4 for provider, and $5 for region
	targetPattern := "/s3/$1/$2?versionId=$3&ConnectionName=$4-$5"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

/*
 * Object Storage Operations - CORS
 */

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.

// <?xml version="1.0" encoding="UTF-8"?>
// <CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
//   <CORSRule>
//     <AllowedOrigin>*</AllowedOrigin>
//     <AllowedMethod>GET</AllowedMethod>
//     <AllowedMethod>PUT</AllowedMethod>
//     <AllowedMethod>POST</AllowedMethod>
//     <AllowedMethod>DELETE</AllowedMethod>
//     <AllowedHeader>*</AllowedHeader>
//     <ExposeHeader>ETag</ExposeHeader>
//     <ExposeHeader>x-amz-server-side-encryption</ExposeHeader>
//     <ExposeHeader>x-amz-request-id</ExposeHeader>
//     <ExposeHeader>x-amz-id-2</ExposeHeader>
//     <MaxAgeSeconds>3000</MaxAgeSeconds>
//   </CORSRule>
// </CORSConfiguration>

type CORSRule struct {
	AllowedOrigin []string `xml:"AllowedOrigin" json:"allowedOrigin" example:"*"`
	AllowedMethod []string `xml:"AllowedMethod" json:"allowedMethod" example:"GET"`
	AllowedHeader []string `xml:"AllowedHeader" json:"allowedHeader" example:"*"`
	ExposeHeader  []string `xml:"ExposeHeader" json:"exposeHeader" example:"ETag"`
	MaxAgeSeconds int      `xml:"MaxAgeSeconds" json:"maxAgeSeconds" example:"3000"`
}

type CORSConfiguration struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
	CORSRule []CORSRule `xml:"CORSRule" json:"corsRule"`
}

type Error struct {
	Code      string `xml:"Code" json:"code" example:"NoSuchCORSConfiguration"`
	Message   string `xml:"Message" json:"message" example:"The CORS configuration does not exist"`
	Resource  string `xml:"Resource" json:"resource" example:"/example-bucket"`
	RequestId string `xml:"RequestId" json:"requestId" example:"656c76696e6727732072657175657374"`
}

// RestGetObjectStorageCORS
// @ID GetObjectStorageCORS
// @Summary Get CORS configuration of an object storage (bucket)
// @Description Get CORS configuration of an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `CORSConfiguration`
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <CORSRule>
// @Description     <AllowedOrigin>*</AllowedOrigin>
// @Description     <AllowedMethod>GET</AllowedMethod>
// @Description     <AllowedMethod>PUT</AllowedMethod>
// @Description     <AllowedMethod>POST</AllowedMethod>
// @Description     <AllowedMethod>DELETE</AllowedMethod>
// @Description     <AllowedHeader>*</AllowedHeader>
// @Description     <ExposeHeader>ETag</ExposeHeader>
// @Description     <ExposeHeader>x-amz-server-side-encryption</ExposeHeader>
// @Description     <ExposeHeader>x-amz-request-id</ExposeHeader>
// @Description     <ExposeHeader>x-amz-id-2</ExposeHeader>
// @Description     <MaxAgeSeconds>3000</MaxAgeSeconds>
// @Description   </CORSRule>
// @Description </CORSConfiguration>
// @Description ```
// @Description
// @Description **Error Response Example (if CORS not configured):**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <Error>
// @Description   <Code>NoSuchCORSConfiguration</Code>
// @Description   <Message>The CORS configuration does not exist</Message>
// @Description   <Resource>/example-bucket</Resource>
// @Description   <RequestId>656c76696e6727732072657175657374</RequestId>
// @Description </Error>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 {object} CORSConfiguration "OK"
// @Failure 404 {object} Error "Not Found"
// @Router /resources/objectStorage/{objectStorageName}/cors [get]
func GetObjectStorageCORS(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/cors?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?cors&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestSetObjectStorageCORS godoc
// @ID SetObjectStorageCORS
// @Summary Set CORS configuration of an object storage (bucket)
// @Description Set CORS configuration of an object storage (bucket)
// @Description
// @Description **Important Notes:**
// @Description - The CORS configuration must be provided in the request body in XML format.
// @Description - The actual request body should have root element `CORSConfiguration`
// @Description
// @Description **Actual XML Request Body Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <CORSConfiguration>
// @Description   <CORSRule>
// @Description     <AllowedOrigin>https://example.com</AllowedOrigin>
// @Description     <AllowedOrigin>https://app.example.com</AllowedOrigin>
// @Description     <AllowedMethod>GET</AllowedMethod>
// @Description     <AllowedMethod>PUT</AllowedMethod>
// @Description     <AllowedHeader>Content-Type</AllowedHeader>
// @Description     <AllowedHeader>Authorization</AllowedHeader>
// @Description     <ExposeHeader>ETag</ExposeHeader>
// @Description     <MaxAgeSeconds>1800</MaxAgeSeconds>
// @Description   </CORSRule>
// @Description   <CORSRule>
// @Description     <AllowedOrigin>*</AllowedOrigin>
// @Description     <AllowedMethod>GET</AllowedMethod>
// @Description     <MaxAgeSeconds>300</MaxAgeSeconds>
// @Description   </CORSRule>
// @Description </CORSConfiguration>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Param reqBody body CORSConfiguration true "CORS Configuration in XML format"
// @Success 200 "OK"
// @Router /resources/objectStorage/{objectStorageName}/cors [put]
func SetObjectStorageCORS(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/cors?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?cors&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestDeleteObjectStorageCORS godoc
// @ID DeleteObjectStorageCORS
// @Summary Delete CORS configuration of an object storage (bucket)
// @Description Delete CORS configuration of an object storage (bucket)
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 204 "No Content"
// @Router /resources/objectStorage/{objectStorageName}/cors [delete]
func DeleteObjectStorageCORS(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*/cors?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?cors&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

/*
 * Object operations
 */

// RestGetDataObjectInfo godoc
// @ID GetObjectInfoGetDataObjectInfo
// @Summary Get an object info from a bucket
// @Description Get an object info from a bucket
// @Description
// @Description **Important Notes:**
// @Description - The generated `Download file` link in Swagger UI may not work because this API get the object metadata only.
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce octet-stream
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param objectKey path string true "Object Name" default(test-object.txt)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 200 "OK"
// @Router /resources/objectStorage/{objectStorageName}/{objectKey} [head]
func GetDataObjectInfo(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("%s", "objectKey is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, objectKey, provider, and region
	sourcePattern := "/resources/objectStorage/*/*?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for objectKey, $3 for provider, and $4 for region
	targetPattern := "/s3/$1/$2?ConnectionName=$3-$4"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestDeleteDataObject godoc
// @ID DeleteDataObject
// @Summary Delete an object from a bucket
// @Description Delete an object from a bucket
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param objectKey path string true "Object Name" default(test-object.txt)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Success 204 "No Content"
// @Router /resources/objectStorage/{objectStorageName}/{objectKey} [delete]
func DeleteDataObject(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("%s", "objectKey is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, objectKey, provider, and region
	sourcePattern := "/resources/objectStorage/*/*?provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for objectKey, $3 for provider, and $4 for region
	targetPattern := "/s3/$1/$2?ConnectionName=$3-$4"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.

type Delete struct {
	Object []Object `xml:"Object" json:"object"`
}

type DeleteResult struct {
	// The xmlns attribute will be set to "http://s3.amazonaws.com/doc/2006-03-01/"
	// Xmlns string `xml:"xmlns,attr" json:"-" example:"http://s3.amazonaws.com/doc/2006-03-01/"`
	Deleted []Object `xml:"Deleted" json:"deleted"`
}

// RestDeleteMultipleDataObjects godoc
// @ID DeleteMultipleDataObjects
// @Summary **Delete** multiple objects from a bucket
// @Description `Delete` multiple objects from a bucket
// @Description
// @Description **Important Notes:**
// @Description - The request body must contain the list of objects to delete in XML format
// @Description - The `delete` query parameter must be set to `true`
// @Description
// @Description **Request Body Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <Delete xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Object>
// @Description     <Key>test-object1.txt</Key>
// @Description   </Object>
// @Description   <Object>
// @Description     <Key>test-object2.txt</Key>
// @Description   </Object>
// @Description </Delete>
// @Description ```
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <Deleted>
// @Description     <Key>test-object1.txt</Key>
// @Description   </Deleted>
// @Description   <Deleted>
// @Description     <Key>test-object2.txt</Key>
// @Description   </Deleted>
// @Description </DeleteResult>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept xml
// @Produce xml
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param delete query boolean true "Delete" default(true) enum(true)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Param reqBody body Delete true "List of objects to delete"
// @Success 200 {object} DeleteResult "OK"
// @Router /resources/objectStorage/{objectStorageName} [post]
func DeleteMultipleDataObjects(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	deleteParam := c.QueryParam("delete")
	if deleteParam != "true" {
		err := fmt.Errorf("%s", "delete query parameter must be true")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, provider, and region
	sourcePattern := "/resources/objectStorage/*?delete=true&provider=*&region=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for provider, and $3 for region
	targetPattern := "/s3/$1?delete&ConnectionName=$2-$3"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// Note: The xmlns attribute and root element name may not be accurately
// represented in Swagger UI due to XML rendering limitations.

type PresignedURLResult struct {
	PresignedURL string `xml:"PresignedURL" json:"presignedURL" example:">https://globally-unique-bucket-hctdx3.s3.dualstack.ap-southeast-2.amazonaws.com/test-file.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIA***EXAMPLE%2F20250904%2Fap-southeast-2%2Fs3%2Faws4_request&X-Amz-Date=20250904T061448Z&X-Amz-Expires=3600&X-Amz-SignedHeaders=host&X-Amz-Signature=***-signature"`
	Expires      int    `xml:"Expires" json:"expires" example:"3600"`
	Method       string `xml:"Method" json:"method" example:"GET"`
}

// RestGeneratePresignedDownloadURL godoc
// @ID GeneratePresignedDownloadURL
// @Summary Generate a presigned URL for downloading an object from a bucket
// @Description Generate a presigned URL for downloading an object from a bucket
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `PresignedURLResult`
// @Description - The `expires` query parameter specifies the expiration time in seconds for the presigned URL (default: 3600 seconds)
// @Description - The generated presigned URL can be used to download the object directly without further authentication
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <PresignedURLResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <PresignedURL>https://globally-unique-bucket-hctdx3.s3.dualstack.ap-southeast-2.amazonaws.com/test-file.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIA***EXAMPLE%2F20250904%2Fap-southeast-2%2Fs3%2Faws4_request&X-Amz-Date=20250904T061448Z&X-Amz-Expires=3600&X-Amz-SignedHeaders=host&X-Amz-Signature=***-signature</PresignedURL>
// @Description   <Expires>3600</Expires>
// @Description   <Method>GET</Method>
// @Description </PresignedURLResult>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param objectKey path string true "Object Name" default(test-object.txt)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Param expires query int false "Expiration time in seconds for the presigned URL" default(3600)
// @Success 200 {object} PresignedURLResult "OK"
// @Router /resources/objectStorage/presigned/download/{objectStorageName}/{objectKey} [get]
func GeneratePresignedDownloadURL(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("%s", "objectKey is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, objectKey, provider, and region
	sourcePattern := "/resources/objectStorage/presigned/download/*/*?provider=*&region=*&expires=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for objectKey, $3 for provider, and $4 for region
	targetPattern := "/s3/presigned/download/$1/$2?ConnectionName=$3-$4&expires=$5"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}

// RestGeneratePresignedUploadURL godoc
// @ID GeneratePresignedUploadURL
// @Summary Generate a presigned URL for uploading an object to a bucket
// @Description Generate a presigned URL for uploading an object to a bucket
// @Description
// @Description **Important Notes:**
// @Description - The actual response will be XML format with root element `PresignedURLResult`
// @Description - The `expires` query parameter specifies the expiration time in seconds for the presigned URL (default: 3600 seconds)
// @Description - The generated presigned URL can be used to upload the object directly without further authentication
// @Description
// @Description **Actual XML Response Example:**
// @Description ```xml
// @Description <?xml version="1.0" encoding="UTF-8"?>
// @Description <PresignedURLResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
// @Description   <PresignedURL>https://globally-unique-bucket-hctdx3.s3.dualstack.ap-southeast-2.amazonaws.com/test-file.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIA***EXAMPLE%2F20250904%2Fap-southeast-2%2Fs3%2Faws4_request&X-Amz-Date=20250904T061448Z&X-Amz-Expires=3600&X-Amz-SignedHeaders=host&X-Amz-Signature=***-signature</PresignedURL>
// @Description   <Expires>3600</Expires>
// @Description   <Method>PUT</Method>
// @Description </PresignedURLResult>
// @Description ```
// @Tags [Resource] Object Storage Management
// @Accept json
// @Produce json
// @Param objectStorageName path string true "Object Storage Name" default(globally-unique-bucket-hctdx3)
// @Param objectKey path string true "Object Name" default(test-object.txt)
// @Param provider query string true "Provider" default(aws)
// @Param region query string true "Region" default(ap-northeast-2)
// @Param expires query int false "Expiration time in seconds for the presigned URL" default(3600)
// @Success 200 {object} PresignedURLResult "OK"
// @Router /resources/objectStorage/presigned/upload/{objectStorageName}/{objectKey} [get]
func GeneratePresignedUploadURL(c echo.Context) error {
	objectStorageName := c.Param("objectStorageName")
	if objectStorageName == "" {
		err := fmt.Errorf("%s", "objectStorageName is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	objectKey := c.Param("objectKey")
	if objectKey == "" {
		err := fmt.Errorf("%s", "objectKey is required")
		log.Error().Err(err).Msg("")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	provider := c.QueryParam("provider")
	region := c.QueryParam("region")

	err := validate(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("invalid provider, region, or connection name")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	// Source path pattern with * to capture objectStorageName, objectKey, provider, and region
	sourcePattern := "/resources/objectStorage/presigned/upload/*/*?provider=*&region=*&expires=*"
	// Target path pattern using $1 for captured objectStorageName, $2 for objectKey, $3 for provider, and $4 for region
	targetPattern := "/s3/presigned/upload/$1/$2?ConnectionName=$3-$4&expires=$5"

	proxyHandler := createSpiderProxyHandler(sourcePattern, targetPattern)
	return proxyHandler(c)
}
