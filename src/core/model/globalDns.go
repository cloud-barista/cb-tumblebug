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
	DomainName string `json:"domainName" validate:"required" example:"example.com" description:"Managed Domain Name in Route53"`
	RecordName string `json:"recordName" example:"mci.example.com" description:"Record Name (FQDN) to update"`
	RecordType string `json:"recordType" example:"A" enums:"A,AAAA,CNAME,TXT" description:"DNS Record Type"`
	TTL        int64  `json:"ttl" example:"300" description:"Time To Live (seconds)"`

	// --- IP Source Selection ---
	SetBy GlobalDnsIPSource `json:"setBy" validate:"required" description:"IP source selection (Choose exactly one)"`
}

// GlobalDnsIPSource defines the source for IP addresses.
type GlobalDnsIPSource struct {
	Mci   *GlobalDnsMciSource   `json:"mci,omitempty" description:"(Method 1) MCI ID source"`
	Label *GlobalDnsLabelSource `json:"label,omitempty" description:"(Method 2) Label Selector source"`
	Ips   []string              `json:"ips,omitempty" example:"[\"1.2.3.4\"]" description:"(Method 3) Manual IP addresses"`
}

// GlobalDnsMciSource defines MCI ID and its namespace.
type GlobalDnsMciSource struct {
	NsId  string `json:"nsId" validate:"required" example:"default" description:"Namespace ID"`
	MciId string `json:"mciId" validate:"required" example:"mci-01" description:"MCI ID"`
}

// GlobalDnsLabelSource defines Label Selector and its namespace.
type GlobalDnsLabelSource struct {
	NsId          string `json:"nsId" validate:"required" example:"default" description:"Namespace ID"`
	LabelSelector string `json:"labelSelector" validate:"required" example:"app=nginx" description:"Label Selector (e.g., app=nginx)"`
}

// GlobalDnsRecordInfo is a struct to handle DNS record information.
type GlobalDnsRecordInfo struct {
	Name   string   `json:"name" example:"mci.example.com"`
	Type   string   `json:"type" example:"A"`
	TTL    int64    `json:"ttl" example:"300"`
	Values []string `json:"values" example:"[\"1.2.3.4\"]"`
}

// RestGetGlobalDnsRecordResponse is a struct to handle 'Get globalDns record' response toward CB-Tumblebug.
type RestGetGlobalDnsRecordResponse struct {
	Record []GlobalDnsRecordInfo `json:"record"`
}
