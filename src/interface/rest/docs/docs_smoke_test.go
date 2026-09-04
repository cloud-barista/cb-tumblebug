package docs

import (
	"strings"
	"testing"

	"github.com/swaggo/swag"
	"sigs.k8s.io/yaml"
)

func TestRegisteredDocServesOpenAPI(t *testing.T) {
	j, err := swag.ReadDoc(swag.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j, `"openapi":"3.0.1"`) {
		t.Fatalf("not openapi 3.0.1: %.120s", j)
	}
	y, err := yaml.JSONToYAML([]byte(j)) // what echo-swagger serves at doc.yaml
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(y), "url: /tumblebug") {
		t.Fatal("servers url missing")
	}
}
