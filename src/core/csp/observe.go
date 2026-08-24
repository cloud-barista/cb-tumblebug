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

package csp

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Direct-SDK calls bypass CB-Spider, so they don't appear in the client's "Internal Call"
// logs. These helpers emit start/end logs in the same shape (provider, op, region, count,
// latency in ms) so all three interaction layers — inbound API, Spider calls, direct SDK —
// can be parsed and correlated the same way. Start/OK are Debug; failures are Warn.

type directCall struct {
	provider string
	op       string
	region   string
	count    int
	start    time.Time
}

// beginDirect logs the start of a direct-CSP-SDK operation and returns a handle to end it.
func beginDirect(provider, op, region string, count int) *directCall {
	dc := &directCall{provider: strings.ToLower(provider), op: op, region: region, count: count, start: time.Now()}
	log.Debug().
		Str("provider", dc.provider).Str("op", dc.op).Str("region", dc.region).Int("count", dc.count).
		Msg("Direct SDK Start")
	return dc
}

// end logs the completion of the operation with its latency; err switches OK -> Failed (Warn).
func (dc *directCall) end(err error) {
	latency := float64(time.Since(dc.start).Microseconds()) / 1000.0 // ms, matching the Spider client logs
	if err != nil {
		log.Warn().Err(err).
			Str("provider", dc.provider).Str("op", dc.op).Str("region", dc.region).Int("count", dc.count).
			Float64("latency", latency).Msg("Direct SDK Failed")
		return
	}
	log.Debug().
		Str("provider", dc.provider).Str("op", dc.op).Str("region", dc.region).Int("count", dc.count).
		Float64("latency", latency).Msg("Direct SDK OK")
}

// DirectSpan traces a direct-CSP-SDK operation started from a provider subpackage
// (pricing, image, ...) with the same start/end logs as the batch dispatch path.
// Usage: span := csp.BeginDirect(...); defer func() { span.End(retErr) }()
type DirectSpan struct{ dc *directCall }

// BeginDirect logs the start of a direct-CSP-SDK operation and returns a span to end it.
// op is a short verb ("pricing", "image-list", ...); region may be "global" for account-wide calls.
func BeginDirect(provider, op, region string, count int) *DirectSpan {
	return &DirectSpan{dc: beginDirect(provider, op, region, count)}
}

// End logs completion with latency; a non-nil err switches OK -> Failed (Warn).
func (s *DirectSpan) End(err error) { s.dc.end(err) }

// observeBatchVMStatus wraps a status func so every actual SDK call is traced uniformly.
func observeBatchVMStatus(provider, op string, fn BatchVMStatusFunc) BatchVMStatusFunc {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
		dc := beginDirect(provider, op, region, len(instanceIds))
		res, err := fn(ctx, region, instanceIds)
		dc.end(err)
		return res, err
	}
}

// observeBatchVMControl wraps a control func so every actual SDK call is traced uniformly.
func observeBatchVMControl(provider, op string, fn BatchVMControlFunc) BatchVMControlFunc {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
		dc := beginDirect(provider, op, region, len(instanceIds))
		res, err := fn(ctx, region, instanceIds)
		dc.end(err)
		return res, err
	}
}
