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

package gcp

import (
	"testing"
)

func TestParseSectionStructuredAndAliases(t *testing.T) {
	results := make(map[string]map[string]nodePriceEntry)

	// Simulate Google's [headerRows, bodyRows, "RegionName (region-code)"] table structure
	tableBlock := []any{
		[]any{"Header"},
		[]any{
			[]any{
				`<p>e2-micro</p>`,
				`<p>2</p>`,
				`<p>1 GiB</p>`,
				`<p>$0.00838 / 1 hour</p>`,
			},
			[]any{
				`<p>n1-standard-96` + "\u00a0" + `<a href="#skylake">Skylake Platform only</a></p>`,
				`<p>96</p>`,
				`<p>360 GiB</p>`,
				`<p>$4.5600 / 1 hour</p>`,
			},
			[]any{
				`<p>z3-highmem-88-standardlssd</p>`,
				`<p>88</p>`,
				`<p>704 GiB</p>`,
				`<p>$10.22866 / 1 hour</p>`,
			},
			[]any{
				`<p>a3-highgpu-8g</p>`,
				`<p>vCPUs: 208</p>`,
				`<p>Memory: 1872GiB</p>`,
				`<p>$88.4900 / 1 hour</p>`,
			},
			[]any{
				`<p>a2-ultragpu-2g</p>`,
				`<p>vCPUs: 24</p>`,
				`<p>Memory: 340GiB</p>`,
				`<p>$10.1376 / 1 hour</p>`,
			},
			[]any{
				`<p>g4-standard-192</p>`,
				`<p>vCPUs: 192</p>`,
				`<p>Memory: 720GiB</p>`,
				`<p>$9 .0000/ 1 hour</p>`,
			},
		},
		"Iowa (us-central1)",
	}

	euTableBlock := []any{
		[]any{"Header"},
		[]any{
			[]any{
				`<p>g2-standard-4</p>`,
				`<p>vCPUs: 4</p>`,
				`<p>Memory: 16GiB</p>`,
				`<p>$0.7770 / 1 hour</p>`,
			},
		},
		"Netherlands (europe-west4)",
	}

	section := []any{float64(117), []any{tableBlock, euTableBlock}}
	if !parseSectionStructured(section, results) {
		t.Fatalf("expected parseSectionStructured to match structured table blocks")
	}

	applyGCPPriceAliasesAndFallbacks(results)

	us := results["us-central1"]
	if us == nil {
		t.Fatalf("expected us-central1 entries")
	}
	if us["e2-micro"].OnDemand != 0.00838 {
		t.Errorf("e2-micro price = %v, want 0.00838", us["e2-micro"].OnDemand)
	}
	if us["n1-standard-96"].OnDemand != 4.56 {
		t.Errorf("n1-standard-96 price = %v, want 4.56", us["n1-standard-96"].OnDemand)
	}
	if us["z3-highmem-88"].OnDemand != 10.22866 {
		t.Errorf("z3-highmem-88 alias price = %v, want 10.22866", us["z3-highmem-88"].OnDemand)
	}
	if us["g4-standard-192"].OnDemand != 9.0 {
		t.Errorf("g4-standard-192 price = %v, want 9.0", us["g4-standard-192"].OnDemand)
	}
	if us["a3-highgpu-2g"].OnDemand != 88.49*0.25 {
		t.Errorf("a3-highgpu-2g derived price = %v, want %v", us["a3-highgpu-2g"].OnDemand, 88.49*0.25)
	}

	eu := results["europe-west4"]
	if eu == nil {
		t.Fatalf("expected europe-west4 entries")
	}
	if eu["a2-ultragpu-2g"].OnDemand != 10.1376 {
		t.Errorf("europe-west4 fallback a2-ultragpu-2g price = %v, want 10.1376", eu["a2-ultragpu-2g"].OnDemand)
	}
	if eu["a3-edgegpu-8g-nolssd"].OnDemand != 88.49 {
		t.Errorf("europe-west4 fallback a3-edgegpu-8g-nolssd price = %v, want 88.49", eu["a3-edgegpu-8g-nolssd"].OnDemand)
	}
}

