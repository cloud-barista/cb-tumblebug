package embedded

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"testing"
)

// TestDashboardsTarGz checks that every embedded dashboard is valid JSON with a uid that matches its file name.
func TestDashboardsTarGz(t *testing.T) {
	gz, err := gzip.NewReader(bytes.NewReader(DashboardsTarGz()))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	n := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		var d struct {
			Uid    string `json:"uid"`
			Panels []any  `json:"panels"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", h.Name, err)
		}
		if d.Uid+".json" != h.Name || len(d.Panels) == 0 {
			t.Fatalf("%s: uid %q, %d panels", h.Name, d.Uid, len(d.Panels))
		}
		n++
	}
	if n < 4 {
		t.Fatalf("expected at least 4 dashboards, got %d", n)
	}
}
