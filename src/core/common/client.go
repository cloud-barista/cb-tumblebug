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
	"fmt"

	"github.com/go-resty/resty/v2"
)

func ExecuteHttpRequest(
	client *resty.Client,
	method string,
	url string,
	headers map[string]string,
	body interface{},
	result interface{}) error {

	req := client.R().SetResult(result)

	if headers != nil {
		req = req.SetHeaders(headers)
	}

	if body != nil {
		req = req.SetBody(body)
	}

	var resp *resty.Response
	var err error

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
		return fmt.Errorf("unsupported method: %s", method)
	}

	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("API error: %s", resp.Status())
	}

	return nil
}
