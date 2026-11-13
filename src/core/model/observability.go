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

package model

import "time"

const (
	ObservabilitySourceExternal = "external"
	ObservabilitySourceManaged  = "managed"

	ObservabilityStatusActive = "Active"
	ObservabilityStatusError  = "Error"
)

// ObservabilityInfo is the OpenTelemetry endpoint that Nodes send metrics and logs to, plus the Grafana that
// shows them. Managed stacks (provisioned by CB-Tumblebug) also record the resources to clean up.
type ObservabilityInfo struct {
	// OtlpEndpoint is the OTLP/HTTP base URL agents export to (paths /v1/metrics and /v1/logs are appended)
	OtlpEndpoint string `json:"otlpEndpoint" example:"https://obs.example.com"`
	// WriteToken is the bearer token agents send; masked unless showSecrets=true
	WriteToken string `json:"writeToken,omitempty" example:"tk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	// CaCert is the PEM certificate agents must trust (empty for a publicly trusted certificate)
	CaCert string `json:"caCert,omitempty"`

	GrafanaUrl           string `json:"grafanaUrl,omitempty" example:"https://obs.example.com"`
	GrafanaAdminUser     string `json:"grafanaAdminUser,omitempty" example:"admin"`
	GrafanaAdminPassword string `json:"grafanaAdminPassword,omitempty"`

	Source                  string   `json:"source" example:"managed" enums:"external,managed"`
	ManagedNsId             string   `json:"managedNsId,omitempty" example:"system"`
	ManagedInfraId          string   `json:"managedInfraId,omitempty" example:"obs"`
	ManagedVNetId           string   `json:"managedVNetId,omitempty"`
	ManagedSecurityGroupIds []string `json:"managedSecurityGroupIds,omitempty"`
	ManagedSshKeyId         string   `json:"managedSshKeyId,omitempty"`
	ManagedDnsDomain        string   `json:"managedDnsDomain,omitempty" example:"example.com"`
	ManagedDnsRecord        string   `json:"managedDnsRecord,omitempty" example:"obs-a1b.example.com"`
	RetentionDays           int      `json:"retentionDays,omitempty" example:"15"`
	// Readers are the Grafana viewer accounts created through the reader API
	Readers []string `json:"readers,omitempty"`

	Status        string    `json:"status" example:"Active" enums:"Active,Error"`
	StatusMessage string    `json:"statusMessage,omitempty"`
	RegisteredAt  time.Time `json:"registeredAt"`
}

// ObservabilityRegisterReq registers an existing OTLP endpoint (and optionally its Grafana).
type ObservabilityRegisterReq struct {
	OtlpEndpoint         string `json:"otlpEndpoint" validate:"required" example:"https://obs.example.com"`
	WriteToken           string `json:"writeToken,omitempty" example:"tk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	CaCert               string `json:"caCert,omitempty"`
	GrafanaUrl           string `json:"grafanaUrl,omitempty" example:"https://grafana.example.com"`
	GrafanaAdminUser     string `json:"grafanaAdminUser,omitempty" example:"admin"`
	GrafanaAdminPassword string `json:"grafanaAdminPassword,omitempty"`
}

// ObservabilityStackReq provisions the observability stack (OTel Collector, Prometheus, Loki, Grafana) from an
// Infra template and registers it.
type ObservabilityStackReq struct {
	NsId        string `json:"nsId,omitempty" example:"system" default:"system"`
	TemplateId  string `json:"templateId,omitempty" example:"infra-obs" default:"infra-obs"`
	Name        string `json:"name,omitempty" example:"obs" default:"obs"`
	Description string `json:"description,omitempty" example:"observability stack for CB-Tumblebug nodes"`
	// DomainName is a Route53 hosted zone; set to get a DNS name and a Let's Encrypt certificate
	DomainName string `json:"domainName,omitempty" example:"example.com"`
	// RecordName under DomainName; default obs-<3 random chars>
	RecordName string `json:"recordName,omitempty" example:"obs-a1b"`
	// RetentionDays for metrics and logs (default 15)
	RetentionDays int `json:"retentionDays,omitempty" example:"15"`
}

// ObservabilityReaderReq creates a Grafana viewer account on the managed stack.
type ObservabilityReaderReq struct {
	Name     string `json:"name" validate:"required" example:"shson"`
	Password string `json:"password,omitempty" example:"generated when empty"`
}

// ObservabilityReaderInfo is returned once with the password.
type ObservabilityReaderInfo struct {
	Name       string `json:"name" example:"shson"`
	Password   string `json:"password,omitempty"`
	GrafanaUrl string `json:"grafanaUrl" example:"https://obs.example.com"`
	Note       string `json:"note,omitempty"`
}

// ObservabilityTestResult is the outcome of sending a test metric and log through the endpoint.
type ObservabilityTestResult struct {
	Success   bool   `json:"success"`
	MetricsOk bool   `json:"metricsOk"`
	LogsOk    bool   `json:"logsOk"`
	ElapsedMs int64  `json:"elapsedMs"`
	Message   string `json:"message,omitempty"`
}

// ObservabilityEnv is the startup registration read from TB_OBSERVABILITY_* environment variables.
type ObservabilityEnv struct {
	OtlpEndpoint         string
	WriteToken           string
	CaCertFile           string
	GrafanaUrl           string
	GrafanaAdminUser     string
	GrafanaAdminPassword string
}

var ObservabilityEnvConfig ObservabilityEnv
