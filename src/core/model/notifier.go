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
type NotifierInfo struct {
	Type       string `json:"type" example:"ntfy"`
	Url        string `json:"url" example:"https://3.35.10.20"`
	Topic      string `json:"topic" example:"tb-a1b2c3d4"`
	WriteToken string `json:"writeToken,omitempty" example:"tk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	ReadToken  string `json:"readToken,omitempty" example:"tk_yyyyyyyyyyyyyyyyyyyyyyyyyyyyy"`
	// CaCert is the PEM certificate VMs and CB-Tumblebug trust for a self-signed server.
	CaCert string `json:"caCert,omitempty"`

	Source         string `json:"source" example:"managed" enums:"external,managed"`
	ManagedNsId    string `json:"managedNsId,omitempty" example:"system"`
	ManagedInfraId string `json:"managedInfraId,omitempty" example:"ntfy"`
	// Resources created for the managed Infra; removed together with it on DELETE ?deleteInfra=true.
	ManagedVNetId           string   `json:"managedVNetId,omitempty"`
	ManagedSecurityGroupIds []string `json:"managedSecurityGroupIds,omitempty"`
	ManagedSshKeyId         string   `json:"managedSshKeyId,omitempty"`
	// Route53 record created for the managed Infra (empty when a self-signed certificate is used).
	ManagedDnsDomain string `json:"managedDnsDomain,omitempty" example:"example.com"`
	ManagedDnsRecord string `json:"managedDnsRecord,omitempty" example:"ntfy.example.com"`
	// Readers are read-only ntfy user accounts created through POST /notifier/reader.
	Readers []string `json:"readers,omitempty"`

	Status        string    `json:"status" example:"Active" enums:"Active,Error"`
	StatusMessage string    `json:"statusMessage,omitempty"`
	LastMessageId string    `json:"lastMessageId,omitempty"`
	RegisteredAt  time.Time `json:"registeredAt"`
}

// NotifierRegisterReq registers an existing ntfy server.
type NotifierRegisterReq struct {
	Url        string `json:"url" validate:"required" example:"https://ntfy.example.com"`
	Topic      string `json:"topic" validate:"required" example:"tb-a1b2c3d4"`
	WriteToken string `json:"writeToken,omitempty" example:"tk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	ReadToken  string `json:"readToken,omitempty" example:"tk_yyyyyyyyyyyyyyyyyyyyyyyyyyyyy"`
	CaCert     string `json:"caCert,omitempty"`
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
