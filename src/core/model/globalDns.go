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

// GlobalDnsRecordReq is a struct to handle 'Update globalDns record' request toward CB-Tumblebug.
// Users must choose EXACTLY ONE of the three IP source methods in 'setBy'.
type GlobalDnsRecordReq struct {
	// --- DNS Record Settings ---
	DomainName    string `json:"domainName" validate:"required" example:"example.com" description:"Managed Domain Name in Route53"`
	RecordName    string `json:"recordName" example:"infra.example.com" description:"Record Name (FQDN) to update"`
	RecordType    string `json:"recordType" example:"A" enums:"A,AAAA,CNAME,TXT" description:"DNS Record Type"`
	TTL           int64  `json:"ttl" example:"300" description:"Time To Live (seconds)"`
	RoutingPolicy string `json:"routingPolicy,omitempty" example:"simple" enums:"simple,geoproximity" description:"Routing policy (default: simple)"`

	// --- IP Source Selection ---
	SetBy GlobalDnsIPSource `json:"setBy" validate:"required" description:"IP source selection (Choose exactly one)"`
}

// GlobalDnsDeleteReq is a struct to handle 'Delete globalDns record' request.
type GlobalDnsDeleteReq struct {
	DomainName    string `json:"domainName" validate:"required" example:"example.com" description:"Managed Domain Name in Route53"`
	RecordName    string `json:"recordName" validate:"required" example:"infra.example.com" description:"Record Name (FQDN) to delete"`
	RecordType    string `json:"recordType" example:"A" enums:"A,AAAA,CNAME,TXT" description:"DNS Record Type"`
	SetIdentifier string `json:"setIdentifier,omitempty" example:"" description:"SetIdentifier for specific record (empty = delete all matching)"`
}

// GlobalDnsBulkDeleteReq is a struct to handle 'Bulk delete globalDns records' request.
type GlobalDnsBulkDeleteReq struct {
	Records []GlobalDnsDeleteReq `json:"records" validate:"required" description:"List of records to delete"`
}

// GlobalDnsBulkDeleteResult represents the result of one record deletion in a bulk operation.
type GlobalDnsBulkDeleteResult struct {
	RecordName    string `json:"recordName" example:"infra.example.com"`
	RecordType    string `json:"recordType" example:"A"`
	SetIdentifier string `json:"setIdentifier,omitempty"`
	Success       bool   `json:"success" example:"true"`
	Message       string `json:"message" example:"deleted successfully"`
}

// GlobalDnsBulkDeleteResponse is a struct to handle 'Bulk delete globalDns records' response.
type GlobalDnsBulkDeleteResponse struct {
	TotalRequested int                         `json:"totalRequested" example:"5"`
	Succeeded      int                         `json:"succeeded" example:"4"`
	Failed         int                         `json:"failed" example:"1"`
	Results        []GlobalDnsBulkDeleteResult `json:"results"`
}

// GlobalDnsIPSource defines the source for IP addresses.
type GlobalDnsIPSource struct {
	Infra *GlobalDnsInfraSource `json:"infra,omitempty" description:"(Method 1) Infra ID source"`
	Label *GlobalDnsLabelSource `json:"label,omitempty" description:"(Method 2) Label Selector source"`
	Ips   []string              `json:"ips,omitempty" example:"[\"1.2.3.4\"]" description:"(Method 3) Manual IP addresses"`
}

// GlobalDnsInfraSource defines Infra ID and its namespace.
type GlobalDnsInfraSource struct {
	NsId    string `json:"nsId" validate:"required" example:"default" description:"Namespace ID"`
	InfraId string `json:"infraId" validate:"required" example:"infra-01" description:"Infra ID"`
}

// GlobalDnsLabelSource defines Label Selector and its namespace.
type GlobalDnsLabelSource struct {
	NsId          string `json:"nsId" validate:"required" example:"default" description:"Namespace ID"`
	LabelSelector string `json:"labelSelector" validate:"required" example:"sys.infraId=infra-01,app=nginx" description:"Label Selector (e.g., sys.infraId=infra-01,app=nginx)"`
}

// GlobalDnsRecordInfo is a struct to handle DNS record information.
type GlobalDnsRecordInfo struct {
	Name          string   `json:"name" example:"infra.example.com"`
	Type          string   `json:"type" example:"A"`
	TTL           int64    `json:"ttl" example:"300"`
	Values        []string `json:"values" example:"[\"1.2.3.4\"]"`
	SetIdentifier string   `json:"setIdentifier,omitempty" example:""`
	RoutingPolicy string   `json:"routingPolicy,omitempty" example:"simple"`
	GeoLatitude   string   `json:"geoLatitude,omitempty" example:"37.56"`
	GeoLongitude  string   `json:"geoLongitude,omitempty" example:"126.97"`
}

// RestGetGlobalDnsRecordResponse is a struct to handle 'Get globalDns record' response toward CB-Tumblebug.
type RestGetGlobalDnsRecordResponse struct {
	Record []GlobalDnsRecordInfo `json:"record"`
}

// HostedZoneInfo is a struct to handle hosted zone information.
type HostedZoneInfo struct {
	ZoneId      string `json:"zoneId" example:"/hostedzone/Z1234567890"`
	Name        string `json:"name" example:"example.com."`
	RecordCount int64  `json:"recordCount" example:"10"`
}

// RestGetHostedZonesResponse is a struct to handle 'Get hosted zones' response.
type RestGetHostedZonesResponse struct {
	HostedZones []HostedZoneInfo `json:"hostedZones"`
}
