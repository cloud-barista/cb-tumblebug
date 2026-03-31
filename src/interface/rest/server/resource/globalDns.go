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
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
)

// classifyDnsError maps error messages to appropriate HTTP status codes.
func classifyDnsError(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "at least one"),
		strings.Contains(msg, "only one"),
		strings.Contains(msg, "requires MCI or Label"),
		strings.Contains(msg, "no records provided"):
		return http.StatusBadRequest
	case strings.Contains(msg, "no hosted zone found"),
		strings.Contains(msg, "no matching records found"),
		strings.Contains(msg, "no VMs with public IP"),
		strings.Contains(msg, "no IP addresses found"):
		return http.StatusNotFound
	case strings.Contains(msg, "VAULT_TOKEN is not set"):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// RestPutGlobalDnsRecord godoc
// @ID PutGlobalDnsRecord
// @Summary Update GlobalDns Record
// @Description Update (UPSERT) a DNS record for a domain in Route53.
// @Description Supports two routing policies: "simple" (default) and "geoproximity" (location-based).
// @Description Choose exactly one IP source method in 'setBy':
// @Description 1. MCI ID (mciId): Fetch Public IPs of all VMs in the MCI.
// @Description 2. Label Selector (labelSelector): Fetch IPs of matching resources.
// @Description 3. Manual IP Values (values): Manually provide IP addresses (simple routing only).
// @Tags [Utility] Global DNS Management
// @Accept  json
// @Produce  json
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/record [put]
func RestPutGlobalDnsRecord(c echo.Context) error {
	ctx := c.Request().Context()

	log.Debug().Msg("[DNS-REST] PUT /resources/globalDns/record called")
	req := &model.GlobalDnsRecordReq{}
	if err := c.Bind(req); err != nil {
		log.Error().Err(err).Msg("[DNS-REST] Failed to bind request body")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	log.Debug().Str("domainName", req.DomainName).Str("recordName", req.RecordName).Str("recordType", req.RecordType).Str("routingPolicy", req.RoutingPolicy).Msg("[DNS-REST] Request parsed")

	resp, err := resource.UpdateGlobalDnsRecord(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("[DNS-REST] UpdateGlobalDnsRecord failed")
		return c.JSON(classifyDnsError(err), model.SimpleMsg{Message: err.Error()})
	}

	log.Debug().Str("response", resp.Message).Msg("[DNS-REST] UpdateGlobalDnsRecord succeeded")
	return c.JSON(http.StatusOK, resp)
}

// RestGetGlobalDnsRecord godoc
// @ID GetGlobalDnsRecord
// @Summary Get GlobalDns Record
// @Description Get DNS records for a domain from Route53. Includes routing policy and geoproximity info.
// @Tags [Utility] Global DNS Management
// @Accept  json
// @Produce  json
// @Param domainName query string true "Domain Name"
// @Param recordName query string false "Record Name (Prefix search)"
// @Param recordType query string false "Record Type"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Success 200 {object} model.RestGetGlobalDnsRecordResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/record [get]
func RestGetGlobalDnsRecord(c echo.Context) error {
	ctx := c.Request().Context()
	domainName := c.QueryParam("domainName")
	recordName := c.QueryParam("recordName")
	recordType := c.QueryParam("recordType")
	log.Debug().Str("domainName", domainName).Str("recordName", recordName).Str("recordType", recordType).Msg("[DNS-REST] GET /resources/globalDns/record called")

	if domainName == "" {
		log.Warn().Msg("[DNS-REST] domainName query parameter is missing")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "domainName is required"})
	}

	resp, err := resource.GetGlobalDnsRecord(ctx, domainName, recordName, recordType)
	if err != nil {
		log.Error().Err(err).Msg("[DNS-REST] GetGlobalDnsRecord failed")
		return c.JSON(classifyDnsError(err), model.SimpleMsg{Message: err.Error()})
	}

	log.Debug().Int("recordCount", len(resp.Record)).Msg("[DNS-REST] GetGlobalDnsRecord succeeded")
	return c.JSON(http.StatusOK, resp)
}

// RestDeleteGlobalDnsRecord godoc
// @ID DeleteGlobalDnsRecord
// @Summary Delete GlobalDns Record
// @Description Delete DNS record(s) from Route53. If setIdentifier is provided, deletes only that specific record.
// @Description If setIdentifier is empty, deletes all records matching the name and type.
// @Tags [Utility] Global DNS Management
// @Accept  json
// @Produce  json
// @Param globalDnsDeleteReq body model.GlobalDnsDeleteReq true "Details for record deletion"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/record [delete]
func RestDeleteGlobalDnsRecord(c echo.Context) error {
	ctx := c.Request().Context()
	log.Debug().Msg("[DNS-REST] DELETE /resources/globalDns/record called")
	req := &model.GlobalDnsDeleteReq{}
	if err := c.Bind(req); err != nil {
		log.Error().Err(err).Msg("[DNS-REST] Failed to bind delete request body")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	log.Debug().Str("domainName", req.DomainName).Str("recordName", req.RecordName).Str("setIdentifier", req.SetIdentifier).Msg("[DNS-REST] Delete request parsed")

	resp, err := resource.DeleteGlobalDnsRecord(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("[DNS-REST] DeleteGlobalDnsRecord failed")
		return c.JSON(classifyDnsError(err), model.SimpleMsg{Message: err.Error()})
	}

	log.Debug().Str("response", resp.Message).Msg("[DNS-REST] DeleteGlobalDnsRecord succeeded")
	return c.JSON(http.StatusOK, resp)
}

// RestGetHostedZones godoc
// @ID GetHostedZones
// @Summary List Hosted Zones
// @Description List all hosted zones available in Route53
// @Tags [Utility] Global DNS Management
// @Produce  json
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Success 200 {object} model.RestGetHostedZonesResponse
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/hostedZone [get]
func RestGetHostedZones(c echo.Context) error {
	ctx := c.Request().Context()
	log.Debug().Msg("[DNS-REST] GET /resources/globalDns/hostedZone called")

	resp, err := resource.ListHostedZones(ctx)
	if err != nil {
		log.Error().Err(err).Msg("[DNS-REST] ListHostedZones failed")
		return c.JSON(classifyDnsError(err), model.SimpleMsg{Message: err.Error()})
	}

	log.Debug().Int("count", len(resp.HostedZones)).Msg("[DNS-REST] ListHostedZones succeeded")
	return c.JSON(http.StatusOK, resp)
}

// RestBulkDeleteGlobalDnsRecord godoc
// @ID BulkDeleteGlobalDnsRecord
// @Summary Bulk Delete GlobalDns Records
// @Description Delete multiple DNS records from Route53 in a single request.
// @Description Records are grouped by domain and submitted as a single ChangeBatch per domain for efficiency.
// @Tags [Utility] Global DNS Management
// @Accept  json
// @Produce  json
// @Param globalDnsBulkDeleteReq body model.GlobalDnsBulkDeleteReq true "List of records to delete"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Success 200 {object} model.GlobalDnsBulkDeleteResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /resources/globalDns/records [delete]
func RestBulkDeleteGlobalDnsRecord(c echo.Context) error {
	ctx := c.Request().Context()
	log.Debug().Msg("[DNS-REST] DELETE /resources/globalDns/records called")
	req := &model.GlobalDnsBulkDeleteReq{}
	if err := c.Bind(req); err != nil {
		log.Error().Err(err).Msg("[DNS-REST] Failed to bind bulk delete request body")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	if len(req.Records) == 0 {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "no records provided"})
	}
	log.Debug().Int("count", len(req.Records)).Msg("[DNS-REST] Bulk delete request parsed")

	resp, err := resource.BulkDeleteGlobalDnsRecords(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("[DNS-REST] BulkDeleteGlobalDnsRecords failed")
		return c.JSON(classifyDnsError(err), model.SimpleMsg{Message: err.Error()})
	}

	log.Debug().Int("succeeded", resp.Succeeded).Int("failed", resp.Failed).Msg("[DNS-REST] BulkDeleteGlobalDnsRecords completed")
	return c.JSON(http.StatusOK, resp)
}
