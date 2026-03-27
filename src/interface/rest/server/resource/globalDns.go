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
)

// RestPutGlobalDnsRecord godoc
// @ID PutGlobalDnsRecord
// @Summary Update GlobalDns Record
// @Description Update (UPSERT) a DNS record for a domain in Route53. (Testing Required / Under Development)
// @Description Choose at least one of these three IP source methods:
// @Description 1. MCI ID (mciId): Fetch Public IPs of all VMs in the MCI.
// @Description 2. Label Selector (labelSelector): Fetch IPs of matching resources.
// @Description 3. Manual IP Values (values): Manually provide IP addresses.
// @Description If multiple methods are used, IPs will be merged and deduplicated.
// @Tags [Utility] Global DNS Management
// @Accept  json
// @Produce  json
// @Param globalDnsRecordReq body model.GlobalDnsRecordReq true "Details for record update"
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/record [put]
func RestPutGlobalDnsRecord(c echo.Context) error {
	req := &model.GlobalDnsRecordReq{}
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	resp, err := resource.UpdateGlobalDnsRecord(req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}

// RestGetGlobalDnsRecord godoc
// @ID GetGlobalDnsRecord
// @Summary Get GlobalDns Record
// @Description Get DNS records for a domain from Route53
// @Tags [Utility] Global DNS Management
// @Accept  json
// @Produce  json
// @Param domainName query string true "Domain Name"
// @Param recordName query string false "Record Name (Prefix search)"
// @Param recordType query string false "Record Type"
// @Success 200 {object} model.RestGetGlobalDnsRecordResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/record [get]
func RestGetGlobalDnsRecord(c echo.Context) error {
	domainName := c.QueryParam("domainName")
	if domainName == "" {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "domainName is required"})
	}

	recordName := c.QueryParam("recordName")
	recordType := c.QueryParam("recordType")

	resp, err := resource.GetGlobalDnsRecord(domainName, recordName, recordType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}
