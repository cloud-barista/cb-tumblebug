package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

// fakeStack answers like nginx in front of the gateway and Grafana: OTLP posts are accepted with the right token,
// and Grafana's datasource proxies return whatever nonce was last posted.
func fakeStack(t *testing.T, token string) *httptest.Server {
	t.Helper()
	var lastBody string
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/"):
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			b := make([]byte, 8192)
			n, _ := r.Body.Read(b)
			lastBody += string(b[:n])
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/health":
			w.Write([]byte(`{"database":"ok"}`))
		case strings.HasPrefix(r.URL.Path, "/api/datasources/proxy/"):
			u, p, ok := r.BasicAuth()
			if !ok || u != "admin" || p != "pw" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			q := r.URL.Query().Get("query")
			for _, part := range strings.Split(lastBody, "tbtest") {
				if len(part) >= 10 && strings.Contains(q, "tbtest"+part[:10]) {
					w.Write([]byte(`{"status":"success","data":{"result":[{"value":"tbtest` + part[:10] + `"}]}}`))
					return
				}
			}
			w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
		case r.URL.Path == "/api/admin/users" && r.Method == http.MethodPost:
			w.Write([]byte(`{"id":7,"message":"User created"}`))
		case r.URL.Path == "/api/users/lookup":
			w.Write([]byte(`{"id":7,"login":"viewer1"}`))
		case r.URL.Path == "/api/admin/users/7" && r.Method == http.MethodDelete:
			w.Write([]byte(`{"message":"User deleted"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRegisterTestAndReaders(t *testing.T) {
	restore := kvstore.SetTestStore(kvtest.NewMemoryStore())
	defer restore()
	defer Unregister()
	srv := fakeStack(t, "tk_test")
	defer srv.Close()

	if _, err := Register(model.ObservabilityRegisterReq{OtlpEndpoint: "http://169.254.169.254"}); err == nil {
		t.Fatal("metadata address must be rejected")
	}
	info, err := Register(model.ObservabilityRegisterReq{OtlpEndpoint: srv.URL + "/", WriteToken: "tk_test", GrafanaUrl: srv.URL, GrafanaAdminPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	if info.WriteToken == "tk_test" || info.GrafanaAdminPassword == "pw" || info.OtlpEndpoint != srv.URL || info.GrafanaAdminUser != "admin" {
		t.Fatalf("registered info wrong or unmasked: %+v", info)
	}
	if stored, _ := load(); stored == nil || stored.WriteToken != "tk_test" || stored.Status != model.ObservabilityStatusActive {
		t.Fatalf("stored info wrong: %+v", stored)
	}
	res := Test(10 * time.Second)
	if !res.Success || !res.MetricsOk || !res.LogsOk {
		t.Fatalf("round trip failed: %+v", res)
	}
	reader, err := AddReader(model.ObservabilityReaderReq{Name: "Viewer1"})
	if err != nil || reader.Name != "viewer1" || reader.Password == "" || reader.GrafanaUrl != srv.URL {
		t.Fatalf("add reader: %+v %v", reader, err)
	}
	if list, _ := ListReaders(); len(list) != 1 || list[0] != "viewer1" {
		t.Fatalf("readers = %v", list)
	}
	if _, err := AddReader(model.ObservabilityReaderReq{Name: "admin"}); err == nil {
		t.Fatal("admin must be rejected")
	}
	if err := RemoveReader("viewer1"); err != nil {
		t.Fatal(err)
	}
	if list, _ := ListReaders(); len(list) != 0 {
		t.Fatalf("readers after remove = %v", list)
	}
	if err := Unregister(); err != nil || Current() != nil {
		t.Fatal("unregister failed")
	}
	// a wrong token is refused by the gateway and reported by the test
	if _, err := Register(model.ObservabilityRegisterReq{OtlpEndpoint: srv.URL, WriteToken: "bad", GrafanaUrl: srv.URL, GrafanaAdminPassword: "pw"}); err != nil {
		t.Fatal(err)
	}
	if res := Test(2 * time.Second); res.Success || !strings.Contains(res.Message, "401") {
		t.Fatalf("bad token must fail the test: %+v", res)
	}
}

func TestMaskSecret(t *testing.T) {
	for in, want := range map[string]string{"": "", "abc": "***", "tk_abcdefghij": "tk_*******hij"} {
		if got := maskSecret(in); got != want {
			t.Fatalf("maskSecret(%q) = %q, want %q", in, got, want)
		}
	}
}
