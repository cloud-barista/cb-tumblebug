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

import (
	"encoding/json"
	"time"
)

const (
	NotifierTypeNtfy = "ntfy"

	NotifierSourceExternal = "external"
	NotifierSourceManaged  = "managed"

	NotifierStatusActive = "Active"
	NotifierStatusError  = "Error"
)

// NotifierInfo is the registered notification channel (one per CB-Tumblebug).
// VMs publish to it with WriteToken; CB-Tumblebug subscribes with ReadToken.
// When provisioned from a multi-NodeGroup Infra template (e.g. infra-ntfy-global), Urls holds every ntfy
// server endpoint sharing Topic and tokens; Url is always the first entry for single-endpoint callers.
type NotifierInfo struct {
	Type       string   `json:"type" example:"ntfy"`
	Url        string   `json:"url" example:"https://3.35.10.20:9443"`
	Urls       []string `json:"urls,omitempty" example:"https://ntfy.example.com,https://47.103.10.20:9443"`
	Topic      string   `json:"topic" example:"tb-a1b2c3d4"`
	WriteToken string   `json:"writeToken,omitempty" example:"tk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	ReadToken  string   `json:"readToken,omitempty" example:"tk_yyyyyyyyyyyyyyyyyyyyyyyyyyyyy"`
	// CaCert is the PEM certificate bundle VMs and CB-Tumblebug trust for self-signed server endpoints.
	CaCert string `json:"caCert,omitempty"`

	Source         string `json:"source" example:"managed" enums:"external,managed"`
	ManagedNsId    string `json:"managedNsId,omitempty" example:"system"`
	ManagedInfraId string `json:"managedInfraId,omitempty" example:"ntfy"`
	// Resources created for the managed Infra; removed together with it on DELETE ?deleteInfra=true.
	ManagedVNetId           string   `json:"managedVNetId,omitempty"`
	ManagedVNetIds          []string `json:"managedVNetIds,omitempty"`
	ManagedSecurityGroupIds []string `json:"managedSecurityGroupIds,omitempty"`
	ManagedSshKeyId         string   `json:"managedSshKeyId,omitempty"`
	ManagedSshKeyIds        []string `json:"managedSshKeyIds,omitempty"`
	// Route53 record(s) created for DNS-eligible managed nodes (empty when only self-signed certificates are used).
	ManagedDnsDomain  string   `json:"managedDnsDomain,omitempty" example:"example.com"`
	ManagedDnsRecord  string   `json:"managedDnsRecord,omitempty" example:"ntfy.example.com"`
	ManagedDnsRecords []string `json:"managedDnsRecords,omitempty"`
	// Readers are read-only ntfy user accounts created through POST /notifier/reader.
	Readers []string `json:"readers,omitempty"`

	Status         string            `json:"status" example:"Active" enums:"Active,Error"`
	StatusMessage  string            `json:"statusMessage,omitempty"`
	LastMessageId  string            `json:"lastMessageId,omitempty"`
	LastMessageIds map[string]string `json:"lastMessageIds,omitempty"`
	RegisteredAt   time.Time         `json:"registeredAt"`
}

// EffectiveUrls returns the deduplicated list of server URLs for this notifier, falling back to Url when Urls is empty.
func (n *NotifierInfo) EffectiveUrls() []string {
	if n == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(u string) {
		if u == "" {
			return
		}
		if _, dup := seen[u]; !dup {
			seen[u] = struct{}{}
			out = append(out, u)
		}
	}
	for _, u := range n.Urls {
		add(u)
	}
	if len(out) == 0 {
		add(n.Url)
	}
	return out
}

// NotifierRegisterReq registers one or more existing ntfy servers sharing the same topic and tokens.
type NotifierRegisterReq struct {
	Url        string   `json:"url,omitempty" example:"https://ntfy.example.com"`
	Urls       []string `json:"urls,omitempty" example:"https://ntfy.example.com,https://47.103.10.20:9443"`
	Topic      string   `json:"topic" validate:"required" example:"tb-a1b2c3d4"`
	WriteToken string   `json:"writeToken,omitempty" example:"tk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	ReadToken  string   `json:"readToken,omitempty" example:"tk_yyyyyyyyyyyyyyyyyyyyyyyyyyyyy"`
	CaCert     string   `json:"caCert,omitempty"`
}

// NotifierSetReq provisions an ntfy server from an Infra template and registers it.
// With DomainName (a Route53 hosted zone managed by CB-Tumblebug) an A record is created and a
// Let's Encrypt certificate is issued, so phones and browsers can subscribe without a custom CA.
type NotifierSetReq struct {
	NsId        string `json:"nsId,omitempty" example:"system" default:"system"`
	TemplateId  string `json:"templateId,omitempty" example:"infra-ntfy" default:"infra-ntfy"`
	Name        string `json:"name,omitempty" example:"ntfy" default:"ntfy"`
	Description string `json:"description,omitempty" example:"ntfy server for CB-Tumblebug notifications"`
	DomainName  string `json:"domainName,omitempty" example:"example.com"`
	// RecordName is the host part under DomainName. Empty picks a free "ntfy-<random>" name so that
	// repeated creations never reuse a name (Let's Encrypt limits certificates per exact name).
	RecordName string `json:"recordName,omitempty" example:"ntfy-a1b"`
}

// NotifierReaderReq creates a read-only ntfy user on the managed server.
type NotifierReaderReq struct {
	Name     string `json:"name" validate:"required" example:"shson"`
	Password string `json:"password,omitempty" example:"generated when empty"`
}

// NotifierReaderInfo is returned once at creation; the password is not stored by CB-Tumblebug.
type NotifierReaderInfo struct {
	Name      string `json:"name" example:"shson"`
	Password  string `json:"password,omitempty"`
	ServerUrl string `json:"serverUrl" example:"https://ntfy.example.com"`
	Topic     string `json:"topic" example:"tb-a1b2c3d4"`
	CaCert    string `json:"caCert,omitempty"`
	Note      string `json:"note,omitempty"`
}

// NotifierPublishReq is a message CB-Tumblebug publishes on behalf of the caller.
type NotifierPublishReq struct {
	Kind string          `json:"kind" validate:"required" example:"custom"`
	Data json.RawMessage `json:"data,omitempty" swaggertype:"object"`
}

// NotifierMessage is a message received from the notifier topic.
// Kind, NsId, InfraId, NodeGroupId, NodeId are filled when the body is JSON with those fields.
type NotifierMessage struct {
	Id          string    `json:"id"`
	Time        time.Time `json:"time"`
	Kind        string    `json:"kind,omitempty"`
	NsId        string    `json:"nsId,omitempty"`
	InfraId     string    `json:"infraId,omitempty"`
	NodeGroupId string    `json:"nodeGroupId,omitempty"`
	NodeId      string    `json:"nodeId,omitempty"`
	Message     string    `json:"message"`
}

// NotifierTestResult is the round-trip check result.
type NotifierTestResult struct {
	Success   bool   `json:"success"`
	MessageId string `json:"messageId,omitempty"`
	ElapsedMs int64  `json:"elapsedMs"`
	Message   string `json:"message,omitempty"`
}

// NotifierEnv holds TB_NOTIFIER_* values used to seed the notifier at startup.
type NotifierEnv struct {
	Url        string
	Topic      string
	WriteToken string
	ReadToken  string
	CaCertFile string
}

// NotifierEnvConfig is set from environment variables in main.
var NotifierEnvConfig NotifierEnv
