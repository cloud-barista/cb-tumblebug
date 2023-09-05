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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// CacheItem is a struct to store cached item
type CacheItem[T any] struct {
	Response  T
	ExpiresAt time.Time
}

var clientCache = sync.Map{}

const (
	// ShortDuration is a duration for short-term cache
	ShortDuration = 2 * time.Second
	// MediumDuration is a duration for medium-term cache
	MediumDuration = 5 * time.Second
	// LongDuration is a duration for long-term cache
	LongDuration = 10 * time.Second
)

// ExecuteHttpRequest performs the HTTP request and fills the result (var requestBody interface{} = nil for empty body)
func ExecuteHttpRequest[B any, T any](
	client *resty.Client,
	method string,
	url string,
	headers map[string]string,
	body *B,
	result *T, // Generic type
	cacheDuration time.Duration,
) error {

	// Generate cache key for GET method only
	cacheKey := ""
	if method == "GET" {

		if body != nil {
			// Serialize the body to JSON
			bodyString, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("JSON marshaling failed: %w", err)
			}
			// Create cache key using both URL and body
			cacheKey = fmt.Sprintf("%s_%s_%s", method, url, string(bodyString))
		} else {
			// Create cache key using only URL
			cacheKey = fmt.Sprintf("%s_%s", method, url)
		}

		if item, found := clientCache.Load(cacheKey); found {
			cachedItem := item.(CacheItem[T]) // Generic type
			if time.Now().Before(cachedItem.ExpiresAt) {
				fmt.Println("Cache hit! Expires: ", time.Now().Sub(cachedItem.ExpiresAt))
				*result = cachedItem.Response
				//val := reflect.ValueOf(result).Elem()
				//cachedVal := reflect.ValueOf(cachedItem.Response)
				//val.Set(cachedVal)

				return nil
			} else {
				fmt.Println("Cache item expired!")
				clientCache.Delete(cacheKey)
			}
		}
	}

	// Perform the HTTP request using Resty
	client.SetDebug(true)
	// SetAllowGetMethodPayload should be set to true for GET method to allow payload
	// NOTE: Need to removed when cb-spider api is stopped to use GET method with payload
	client.SetAllowGetMethodPayload(true)
	req := client.R().SetHeader("Content-Type", "application/json").SetResult(result)

	if headers != nil {
		req = req.SetHeaders(headers)
	}

	if body != nil {
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
		return fmt.Errorf("[Error from: %s] Message: %s", url, err.Error())
	}

	if resp.IsError() {
		return fmt.Errorf("[Error from: %s] Status code: %s", url, resp.Status())
	}

	// Update the cache for GET method only
	if method == "GET" {

		//val := reflect.ValueOf(result).Elem()
		//newCacheItem := val.Interface()

		// Check if result is nil
		if result == nil {
			fmt.Println("Warning: result is nil, not caching.")
		} else {
			clientCache.Store(cacheKey, CacheItem[T]{Response: *result, ExpiresAt: time.Now().Add(cacheDuration)})
			fmt.Println("Cached successfully!")
		}
	}

	return nil
}
