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

import "embed"

//go:embed scripts
var scripts embed.FS

// InitScript is the first-boot wrapper that CB-Tumblebug delivers as VM user data.
func InitScript() string { return read("init-script.sh") }

// InstallNtfy installs and configures a self-hosted ntfy server for the notifier.
func InstallNtfy() string { return read("install-ntfy.sh") }

func read(name string) string {
	b, err := scripts.ReadFile("scripts/" + name)
	if err != nil {
		panic("embedded script missing: " + name) // build-time asset; cannot happen for a correctly built binary
	}
	return string(b)
}
