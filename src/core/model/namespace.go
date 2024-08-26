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

// Package model is to handle object of CB-Tumblebug
package model

type NsReq struct {
	Name        string `json:"name" example:"default"`
	Description string `json:"description" example:"Description for this namespace"`
}

// swagger:response NsInfo
type NsInfo struct {
	Id   string `json:"id" example:"default"`
	Name string `json:"name" example:"default"`
	// uuid is universally unique identifier for the resource
	Uuid        string `json:"uuid,omitempty"`
	Description string `json:"description" example:"Description for this namespace"`
}
