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

// Package embedded holds scripts compiled into the cb-tumblebug binary and executed on VMs
// (first-boot init, service installers). Keep them here, not under the repository-level scripts/
// directory, which is for operator-run tooling; go:embed cannot reach outside this package.
package embedded

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"io/fs"
	"path"
)

//go:embed scripts
var scripts embed.FS

//go:embed dashboards
var dashboards embed.FS

// InitScript is the first-boot wrapper that CB-Tumblebug delivers as VM user data.
func InitScript() string { return read("init-script.sh") }

// InstallNtfy installs and configures a self-hosted ntfy server for the notifier.
func InstallNtfy() string { return read("install-ntfy.sh") }

// InstallOtelStack installs the observability stack (OTel Collector gateway, Prometheus, Loki, Grafana, nginx).
func InstallOtelStack() string { return read("install-otel-stack.sh") }

// InstallOtelAgent installs the OTel Collector agent on a node; the stack serves it at /dl/install-agent.sh.
func InstallOtelAgent() string { return read("install-otel-agent.sh") }

// DashboardsTarGz packs the Grafana dashboards (dashboards/*.json) so the stack installer can unpack them
// into Grafana's provisioning directory with one command.
func DashboardsTarGz() []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entries, _ := fs.ReadDir(dashboards, "dashboards")
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := dashboards.ReadFile("dashboards/" + e.Name())
		if err != nil {
			panic("embedded dashboard missing: " + e.Name())
		}
		_ = tw.WriteHeader(&tar.Header{Name: e.Name(), Mode: 0644, Size: int64(len(b))})
		_, _ = tw.Write(b)
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func read(name string) string {
	b, err := scripts.ReadFile("scripts/" + name)
	if err != nil {
		panic("embedded script missing: " + name) // build-time asset; cannot happen for a correctly built binary
	}
	return string(b)
}
