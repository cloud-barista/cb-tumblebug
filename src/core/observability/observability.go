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

// Package observability keeps the OpenTelemetry endpoint (and Grafana) that Nodes send metrics and logs to.
// CB-Tumblebug exports it to every init script as TB_OTEL_*; the initscript-otel-agent template installs the
// agent. The stack itself can be provisioned by CB-Tumblebug (otel_set.go) or registered from elsewhere.
package observability

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/managedset"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

const storeKey = "/system/observability"

type state struct {
	mu   sync.RWMutex
	info *model.ObservabilityInfo
}

var s = &state{}

func init() {
	infra.ObservabilityProvider = Current
}

// Init loads the stored stack, or seeds it from TB_OBSERVABILITY_* when none is stored.
func Init() {
	info, err := load()
	if err != nil {
		log.Error().Err(err).Msg("observability: failed to load stored stack")
		return
	}
	if info == nil {
		info = fromEnv()
		if info == nil {
			return
		}
		if err := save(info); err != nil {
			log.Error().Err(err).Msg("observability: failed to save stack seeded from env")
			return
		}
		log.Info().Str("otlp", info.OtlpEndpoint).Msg("observability: registered from environment")
	}
	s.mu.Lock()
	s.info = info
	s.mu.Unlock()
}

func fromEnv() *model.ObservabilityInfo {
	env := model.ObservabilityEnvConfig
	if env.OtlpEndpoint == "" {
		return nil
	}
	info, err := newInfo(model.ObservabilityRegisterReq{
		OtlpEndpoint: env.OtlpEndpoint, WriteToken: env.WriteToken, GrafanaUrl: env.GrafanaUrl,
		GrafanaAdminUser: env.GrafanaAdminUser, GrafanaAdminPassword: env.GrafanaAdminPassword,
	})
	if err != nil {
		log.Error().Err(err).Msg("observability: TB_OBSERVABILITY_OTLP_ENDPOINT rejected")
		return nil
	}
	if env.CaCertFile != "" {
		pem, err := os.ReadFile(env.CaCertFile)
		if err != nil {
			log.Error().Err(err).Msg("observability: cannot read TB_OBSERVABILITY_CA_CERT_FILE")
			return nil
		}
		info.CaCert = string(pem)
	}
	info.Status = model.ObservabilityStatusActive
	return info
}

func newInfo(req model.ObservabilityRegisterReq) (*model.ObservabilityInfo, error) {
	endpoint, err := managedset.ValidateServerUrl(req.OtlpEndpoint)
	if err != nil {
		return nil, fmt.Errorf("otlpEndpoint: %w", err)
	}
	grafana := ""
	if req.GrafanaUrl != "" {
		if grafana, err = managedset.ValidateServerUrl(req.GrafanaUrl); err != nil {
			return nil, fmt.Errorf("grafanaUrl: %w", err)
		}
	}
	if strings.ContainsAny(req.WriteToken, "'\n\r") {
		return nil, errors.New("writeToken must not contain quotes or newlines")
	}
	return &model.ObservabilityInfo{
		OtlpEndpoint: endpoint, WriteToken: req.WriteToken, CaCert: req.CaCert,
		GrafanaUrl: grafana, GrafanaAdminUser: managedset.FirstNonEmpty(req.GrafanaAdminUser, "admin"), GrafanaAdminPassword: req.GrafanaAdminPassword,
		Source: model.ObservabilitySourceExternal, RegisteredAt: time.Now(),
	}, nil
}

// Register stores an existing OTLP endpoint after checking that it answers.
func Register(req model.ObservabilityRegisterReq) (*model.ObservabilityInfo, error) {
	info, err := newInfo(req)
	if err != nil {
		return nil, err
	}
	return register(info)
}

func register(info *model.ObservabilityInfo) (*model.ObservabilityInfo, error) {
	if err := health(info); err != nil {
		return nil, fmt.Errorf("observability endpoint check failed: %w", err)
	}
	info.Status = model.ObservabilityStatusActive
	info.StatusMessage = ""
	if err := save(info); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.info = info
	s.mu.Unlock()
	return masked(info), nil
}

// Get returns the registered stack with secrets masked.
func Get() *model.ObservabilityInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return masked(s.info)
}

// Current returns a copy of the registered stack with secrets, or nil.
func Current() *model.ObservabilityInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.info == nil {
		return nil
	}
	c := *s.info
	return &c
}

// Unregister forgets the stack; Nodes created afterwards get no TB_OTEL_* variables.
func Unregister() error {
	s.mu.Lock()
	s.info = nil
	s.mu.Unlock()
	if err := kvstore.Delete(storeKey); err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	return nil
}

// SetStatus records an operational status on the stored stack.
func SetStatus(status, message string) {
	updateInfo(func(i *model.ObservabilityInfo) { i.Status, i.StatusMessage = status, message })
}

func updateInfo(fn func(*model.ObservabilityInfo)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info == nil {
		return
	}
	fn(s.info)
	_ = save(s.info)
}

// health checks that the OTLP/HTTP endpoint answers (any HTTP status: the gateway rejects GETs but is up)
// and, when a Grafana URL is set, that Grafana reports healthy.
func health(info *model.ObservabilityInfo) error {
	client, err := httpClient(info, 10*time.Second)
	if err != nil {
		return err
	}
	resp, err := client.Get(info.OtlpEndpoint + "/v1/metrics")
	if err != nil {
		return fmt.Errorf("otlp endpoint: %w", err)
	}
	resp.Body.Close()
	if info.GrafanaUrl != "" {
		resp, err := client.Get(info.GrafanaUrl + "/api/health")
		if err != nil {
			return fmt.Errorf("grafana: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("grafana: unexpected status %s", resp.Status)
		}
	}
	return nil
}

// Test sends one metric and one log line through the OTLP endpoint and, when Grafana admin access is
// available, waits until both can be queried back through Grafana's datasource proxy.
func Test(timeout time.Duration) model.ObservabilityTestResult {
	start := time.Now()
	info := Current()
	if info == nil {
		return model.ObservabilityTestResult{Message: "no observability stack is registered", ElapsedMs: 0}
	}
	nonce := "tbtest" + managedset.RandomToken(10)
	now := time.Now()
	nowNs := fmt.Sprintf("%d", now.UnixNano())
	metricBody := fmt.Sprintf(`{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"tb-test"}}]},"scopeMetrics":[{"metrics":[{"name":"tb_test_gauge","gauge":{"dataPoints":[{"asDouble":1,"timeUnixNano":"%s","attributes":[{"key":"nonce","value":{"stringValue":"%s"}}]}]}}]}]}]}`, nowNs, nonce)
	logBody := fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"tb-test"}},{"key":"tb.infra","value":{"stringValue":"tb-test"}}]},"scopeLogs":[{"logRecords":[{"timeUnixNano":"%s","severityText":"INFO","body":{"stringValue":"cb-tumblebug observability test %s"}}]}]}]}`, nowNs, nonce)
	if err := postOtlp(info, "/v1/metrics", metricBody); err != nil {
		return model.ObservabilityTestResult{Message: "metrics: " + err.Error(), ElapsedMs: time.Since(start).Milliseconds()}
	}
	if err := postOtlp(info, "/v1/logs", logBody); err != nil {
		return model.ObservabilityTestResult{Message: "logs: " + err.Error(), ElapsedMs: time.Since(start).Milliseconds()}
	}
	if info.GrafanaUrl == "" || info.GrafanaAdminPassword == "" {
		return model.ObservabilityTestResult{Success: true, MetricsOk: true, LogsOk: true, ElapsedMs: time.Since(start).Milliseconds(),
			Message: "endpoint accepted a metric and a log line (no Grafana admin access to verify storage)"}
	}
	deadline := time.Now().Add(timeout)
	metricsOk, logsOk := false, false
	var lastErr error
	for time.Now().Before(deadline) && !(metricsOk && logsOk) {
		if !metricsOk {
			q := url.QueryEscape(fmt.Sprintf(`tb_test_gauge{nonce="%s"}`, nonce))
			body, err := grafanaGet(info, "/api/datasources/proxy/uid/tb-prom/api/v1/query?query="+q)
			if err != nil {
				lastErr = err
			} else if strings.Contains(body, nonce) {
				metricsOk = true
			}
		}
		if !logsOk {
			q := url.QueryEscape(fmt.Sprintf(`{service_name="tb-test"} |= "%s"`, nonce))
			path := fmt.Sprintf("/api/datasources/proxy/uid/tb-loki/loki/api/v1/query_range?query=%s&start=%d&end=%d&limit=5",
				q, now.Add(-time.Minute).UnixNano(), time.Now().Add(time.Minute).UnixNano())
			body, err := grafanaGet(info, path)
			if err != nil {
				lastErr = err
			} else if strings.Contains(body, nonce) {
				logsOk = true
			}
		}
		if !(metricsOk && logsOk) {
			time.Sleep(2 * time.Second)
		}
	}
	res := model.ObservabilityTestResult{Success: metricsOk && logsOk, MetricsOk: metricsOk, LogsOk: logsOk, ElapsedMs: time.Since(start).Milliseconds()}
	if !res.Success {
		res.Message = fmt.Sprintf("sent, but not queryable through Grafana within %s (metrics %v, logs %v)", timeout, metricsOk, logsOk)
		if lastErr != nil {
			res.Message += ": " + lastErr.Error()
		}
	}
	return res
}

func postOtlp(info *model.ObservabilityInfo, path, body string) error {
	client, err := httpClient(info, 15*time.Second)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, info.OtlpEndpoint+path, bytes.NewReader([]byte(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if info.WriteToken != "" {
		req.Header.Set("Authorization", "Bearer "+info.WriteToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

// grafanaGet calls the Grafana HTTP API with the admin account.
func grafanaGet(info *model.ObservabilityInfo, path string) (string, error) {
	return grafanaDo(info, http.MethodGet, path, "")
}

func grafanaDo(info *model.ObservabilityInfo, method, path, body string) (string, error) {
	if info.GrafanaUrl == "" || info.GrafanaAdminPassword == "" {
		return "", errors.New("grafana admin access is not configured")
	}
	client, err := httpClient(info, 20*time.Second)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(method, info.GrafanaUrl+path, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(info.GrafanaAdminUser, info.GrafanaAdminPassword)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("grafana %s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(raw)))
	}
	return string(raw), nil
}

func httpClient(info *model.ObservabilityInfo, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if info.CaCert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(info.CaCert)) {
			return nil, errors.New("caCert is not a valid PEM certificate")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool}
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func load() (*model.ObservabilityInfo, error) {
	raw, exists, err := kvstore.Get(storeKey)
	if err != nil || !exists {
		return nil, err
	}
	var info model.ObservabilityInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func save(info *model.ObservabilityInfo) error {
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return kvstore.Put(storeKey, string(raw))
}

func masked(info *model.ObservabilityInfo) *model.ObservabilityInfo {
	if info == nil {
		return nil
	}
	c := *info
	c.WriteToken = maskSecret(c.WriteToken)
	c.GrafanaAdminPassword = maskSecret(c.GrafanaAdminPassword)
	return &c
}

func maskSecret(t string) string {
	if len(t) <= 6 {
		if t == "" {
			return ""
		}
		return "***"
	}
	return t[:3] + strings.Repeat("*", len(t)-6) + t[len(t)-3:]
}
