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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/go-resty/resty/v2"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// CacheItem is a struct to store cached item
type CacheItem[T any] struct {
	Response  T
	ExpiresAt time.Time
}

// clientCache is a map for cache items of intenal calls
var clientCache = sync.Map{}

// clientRequestCounter is a map for request counters of intenal calls
var clientRequestCounter = sync.Map{}

const (
	// VeryShortDuration is a duration for very short-term cache
	VeryShortDuration = 1 * time.Second
	// ShortDuration is a duration for short-term cache
	ShortDuration = 2 * time.Second
	// MediumDuration is a duration for medium-term cache
	MediumDuration = 5 * time.Second
	// LongDuration is a duration for long-term cache
	LongDuration = 10 * time.Second
)

// NoBody is a constant for empty body
const NoBody = "NOBODY"

// SetUseBody returns false if the given body is NoBody
func SetUseBody(requestBody interface{}) bool {
	if str, ok := requestBody.(string); ok {
		return str != NoBody
	}
	return true
}

// limitConcurrentRequests limits the number of Concurrent requests to the given limit
func limitConcurrentRequests(requestKey string, limit int) bool {
	count, _ := clientRequestCounter.LoadOrStore(requestKey, 0)
	currentCount := count.(int)

	if currentCount >= limit {
		fmt.Printf("[%d] requests for %s \n", currentCount, requestKey)
		return false
	}

	clientRequestCounter.Store(requestKey, currentCount+1)
	return true
}

// requestDone decreases the request counter
func requestDone(requestKey string) {
	count, _ := clientRequestCounter.Load(requestKey)
	if count == nil {
		return
	}
	currentCount := count.(int)

	if currentCount > 0 {
		clientRequestCounter.Store(requestKey, currentCount-1)
	}
}

// ExecuteHttpRequest performs the HTTP request and fills the result (var requestBody interface{} = nil for empty body)
func ExecuteHttpRequest[B any, T any](
	client *resty.Client,
	method string,
	url string,
	headers map[string]string,
	useBody bool, // New parameter to specify if body should be used
	body *B,
	result *T, // Generic type
	cacheDuration time.Duration,
) error {

	// Generate cache key for GET method only
	requestKey := ""
	if method == "GET" {

		if useBody {
			// Serialize the body to JSON
			bodyString, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("JSON marshaling failed: %w", err)
			}
			// Create cache key using both URL and body
			requestKey = fmt.Sprintf("%s_%s_%s", method, url, string(bodyString))
		} else {
			// Create cache key using only URL
			requestKey = fmt.Sprintf("%s_%s", method, url)
		}

		if item, found := clientCache.Load(requestKey); found {
			// Ensure safe type assertion
			cachedItem, ok := item.(CacheItem[T])
			if !ok {
				log.Error().Msgf("Type assertion failed for cache item: expected CacheItem[%T], got %T", *result, item)
				clientCache.Delete(requestKey) // Delete invalid cache item
				return fmt.Errorf("type assertion failed for cache item")
			}

			if time.Now().Before(cachedItem.ExpiresAt) {
				log.Trace().Msgf("Cache hit! Expires: %v", time.Now().Sub(cachedItem.ExpiresAt))
				*result = cachedItem.Response
				//val := reflect.ValueOf(result).Elem()
				//cachedVal := reflect.ValueOf(cachedItem.Response)
				//val.Set(cachedVal)

				return nil
			} else {
				//log.Trace().Msg("Cache item expired!")
				clientCache.Delete(requestKey)
			}
		}

		// Limit the number of concurrent requests
		concurrencyLimit := 10
		retryWait := 5 * time.Second
		retryLimit := 3
		retryCount := 0
		// try to wait for the upcoming cached result when sending queue is full
		for {
			if !limitConcurrentRequests(requestKey, concurrencyLimit) {
				if retryCount >= retryLimit {
					log.Debug().Msgf("Too many same requests: %s\n", requestKey)
					return fmt.Errorf("Too many same requests: %s", requestKey)
				}
				time.Sleep(retryWait)

				if item, found := clientCache.Load(requestKey); found {
					cachedItem, ok := item.(CacheItem[T])
					if !ok {
						log.Error().Msgf("Type assertion failed for cache item while waiting: expected CacheItem[%T], got %T", *result, item)
						clientCache.Delete(requestKey) // Delete invalid cache item
						return fmt.Errorf("type assertion failed for cache item while waiting")
					}
					*result = cachedItem.Response
					// release the request count for parallel requests limit
					requestDone(requestKey)
					log.Debug().Msg("Got the cached result while waiting")
					return nil
				}
				retryCount++
			} else {
				break
			}
		}
	}

	// Perform the HTTP request using Resty
	//client.SetDebug(true)
	// SetAllowGetMethodPayload should be set to true for GET method to allow payload
	// NOTE: Need to removed when cb-spider api is stopped to use GET method with payload
	client.SetAllowGetMethodPayload(true)
	req := client.R().SetHeader("Content-Type", "application/json").SetResult(result)

	if headers != nil {
		req = req.SetHeaders(headers)
	}

	if useBody {
		req = req.SetBody(body)
	}

	var resp *resty.Response
	var err error

	// Execute HTTP method based on the given type
	switch method {
	case "GET":
		resp, err = req.Get(url)
	case "POST":
		resp, err = req.Post(url)
	case "PUT":
		resp, err = req.Put(url)
	case "DELETE":
		resp, err = req.Delete(url)
	default:
		return fmt.Errorf("Unsupported rest method: %s", method)
	}

	if err != nil {
		if method == "GET" {
			requestDone(requestKey)
		}
		return fmt.Errorf("[Error from: %s] Message: %s", url, err.Error())
	}

	if resp.IsError() {
		if method == "GET" {
			requestDone(requestKey)
		}
		return fmt.Errorf("[Error from: %s] Status code: %s, Message: %s", url, resp.Status(), resp.Body())
	}

	// Update the cache for GET method only
	if method == "GET" {

		//val := reflect.ValueOf(result).Elem()
		//newCacheItem := val.Interface()

		// release the request count for parallel requests limit
		requestDone(requestKey)

		// Check if result is nil
		if result == nil {
			log.Trace().Msg("Fesult is nil, not caching")
		} else {
			clientCache.Store(requestKey, CacheItem[T]{Response: *result, ExpiresAt: time.Now().Add(cacheDuration)})
			//log.Trace().Msg("Cached successfully!")
		}
	}

	return nil
}

// RequestInfo stores the essential details of an HTTP request.
type RequestInfo struct {
	Method string            `json:"method"`         // HTTP method (GET, POST, etc.), indicating the request's action type.
	URL    string            `json:"url"`            // The URL the request is made to.
	Header map[string]string `json:"header"`         // Key-value pairs of the request headers.
	Body   interface{}       `json:"body,omitempty"` // Optional: request body
}

// RequestDetails contains detailed information about an HTTP request and its processing status.
type RequestDetails struct {
	StartTime     time.Time   `json:"startTime"`     // The time when the request was received by the server.
	EndTime       time.Time   `json:"endTime"`       // The time when the request was fully processed.
	Status        string      `json:"status"`        // The current status of the request (e.g., "Handling", "Error", "Success").
	RequestInfo   RequestInfo `json:"requestInfo"`   // Extracted information about the request.
	ResponseData  interface{} `json:"responseData"`  // The data sent back in response to the request.
	ErrorResponse string      `json:"errorResponse"` // A message describing any error that occurred during request processing.
}

// RequestMap is a map for request details
var RequestMap = sync.Map{}

// ProgressInfo contains the progress information of a request.
type ProgressInfo struct {
	Title string      `json:"title"`
	Info  interface{} `json:"info"`
	Time  time.Time   `json:"time"`
}

// ExtractRequestInfo extracts necessary information from http.Request
func ExtractRequestInfo(r *http.Request) RequestInfo {
	headerInfo := make(map[string]string)
	for name, headers := range r.Header {
		headerInfo[name] = headers[0]
	}

	//var bodyString string
	var bodyObject interface{}
	if r.Body != nil { // Check if the body is not nil
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			//bodyString = string(bodyBytes)
			json.Unmarshal(bodyBytes, &bodyObject) // Try to unmarshal to a JSON object

			// Important: Write the body back for further processing
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	return RequestInfo{
		Method: r.Method,
		URL:    r.URL.String(),
		Header: headerInfo,
		Body:   bodyObject, // Use the JSON object if parsing was successful, otherwise it's nil
	}
}

// // StartRequestWithLog initializes request tracking details
// func StartRequestWithLog(c echo.Context) (string, error) {
// 	reqID := c.Request().Header.Get(echo.HeaderXRequestID)
// 	if reqID == "" {
// 		reqID = fmt.Sprintf("%d", time.Now().UnixNano())
// 	}
// 	if _, ok := RequestMap.Load(reqID); ok {
// 		return reqID, fmt.Errorf("the x-request-id is already in use")
// 	}

// 	details := RequestDetails{
// 		StartTime:   time.Now(),
// 		Status:      "Handling",
// 		RequestInfo: ExtractRequestInfo(c.Request()),
// 	}
// 	RequestMap.Store(reqID, details)
// 	return reqID, nil
// }

// EndRequestWithLog updates the request details and sends the final response.
func EndRequestWithLog(c echo.Context, err error, responseData interface{}) error {

	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	if v, ok := RequestMap.Load(reqID); ok {
		details := v.(RequestDetails)
		details.EndTime = time.Now()

		c.Response().Header().Set(echo.HeaderXRequestID, reqID)

		if err != nil {
			details.Status = "Error"
			details.ErrorResponse = err.Error()
			RequestMap.Store(reqID, details)
			if responseData == nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
			} else {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
			}
		}

		details.Status = "Success"
		details.ResponseData = responseData
		RequestMap.Store(reqID, details)
		return c.JSON(http.StatusOK, responseData)
	}

	return c.JSON(http.StatusNotFound, map[string]string{"message": "Invalid Request ID"})
}

// UpdateRequestProgress updates the handling status of the request.
func UpdateRequestProgress(reqID string, progressData interface{}) {
	if v, ok := RequestMap.Load(reqID); ok {
		details := v.(RequestDetails)

		var responseData []interface{}
		if details.ResponseData != nil {
			// Convert existing ResponseData to []interface{}
			responseData = details.ResponseData.([]interface{})
		}
		// Append the new progressData to the existing ResponseData
		responseData = append(responseData, progressData)
		details.ResponseData = responseData

		RequestMap.Store(reqID, details)
	}
}

// ForwardRequestToAny forwards the given request to the specified path
func ForwardRequestToAny(reqPath string, method string, requestBody interface{}) (interface{}, error) {
	client := resty.New()
	var callResult interface{}

	url := model.SpiderRestUrl + "/" + reqPath

	var requestBodyBytes []byte
	var ok bool
	if requestBodyBytes, ok = requestBody.([]byte); !ok {
		return nil, fmt.Errorf("requestBody is not []byte type")
	}

	var requestBodyMap map[string]interface{}
	err := json.Unmarshal(requestBodyBytes, &requestBodyMap)
	if err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	err = ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBodyMap),
		&requestBodyMap,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	return callResult, nil
}
