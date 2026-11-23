package managedset

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestBlockedAddr(t *testing.T) {
	blocked := []string{
		"169.254.169.254", "::ffff:169.254.169.254", "169.254.0.23", "fe80::1", "fe80::1%eth0",
		"0.0.0.0", "::", "100.100.100.200", "fd00:ec2::254", "fd20:ce::254",
	}
	for _, a := range blocked {
		if !blockedAddr(netip.MustParseAddr(a)) {
			t.Errorf("%s should be blocked", a)
		}
	}
	allowed := []string{"127.0.0.1", "::1", "10.0.0.5", "172.16.3.4", "192.168.0.10", "8.8.8.8", "100.100.100.201", "fd00::1", "2001:db8::1"}
	for _, a := range allowed {
		if blockedAddr(netip.MustParseAddr(a)) {
			t.Errorf("%s should be allowed", a)
		}
	}
}

func TestGuardDial(t *testing.T) {
	for _, a := range []string{"169.254.169.254:80", "[fe80::1%eth0]:443", "100.100.100.200:80", "[fd00:ec2::254]:80", "not-an-address"} {
		if err := guardDial("tcp", a, nil); err == nil {
			t.Errorf("dial to %s should be refused", a)
		}
	}
	for _, a := range []string{"127.0.0.1:8080", "10.0.0.5:443", "[2001:db8::1]:443"} {
		if err := guardDial("tcp", a, nil); err != nil {
			t.Errorf("dial to %s should be allowed: %v", a, err)
		}
	}
}

func TestValidateServerUrlHosts(t *testing.T) {
	for _, u := range []string{
		"http://169.254.169.254", "http://[::ffff:169.254.169.254]", "http://[fe80::1%25eth0]", "http://0.0.0.0:80",
		"http://100.100.100.200", "http://[fd00:ec2::254]", "http://metadata.google.internal", "http://METADATA",
	} {
		if _, err := ValidateServerUrl(u); err == nil {
			t.Errorf("%s should be rejected", u)
		}
	}
	for in, want := range map[string]string{
		"https://ntfy.example.com/":   "https://ntfy.example.com",
		"http://10.0.0.5:8080/base/":  "http://10.0.0.5:8080/base",
		"http://127.0.0.1:2586":       "http://127.0.0.1:2586",
		"https://[2001:db8::1]:8443/": "https://[2001:db8::1]:8443",
	} {
		got, err := ValidateServerUrl(in)
		if err != nil || got != want {
			t.Errorf("ValidateServerUrl(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestServerClient(t *testing.T) {
	var gotAuth, gotBody string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Write([]byte("target"))
	}))
	defer target.Close()
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-target":
			http.Redirect(w, r, target.URL+"/x", http.StatusTemporaryRedirect)
		case "/to-metadata":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
		}
	}))
	defer front.Close()

	c := NewServerClient(nil, 5*time.Second)
	c.hc.Transport.(*http.Transport).Proxy = nil

	// An ordinary redirect is followed with its method, body and headers, as before.
	resp, err := c.Do(context.Background(), http.MethodPost, front.URL+"/to-target", strings.NewReader("payload"),
		func(r *http.Request) { r.Header.Set("Authorization", "Bearer t") })
	if err != nil {
		t.Fatalf("redirect to an ordinary server failed: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(raw) != "target" || gotBody != "payload" || gotAuth != "Bearer t" {
		t.Errorf("redirected request arrived as body=%q auth=%q, response %q", gotBody, gotAuth, raw)
	}

	if _, err := c.Do(context.Background(), http.MethodGet, front.URL+"/to-metadata", nil, nil); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("redirect to a metadata address should be refused, got %v", err)
	}
	if _, err := c.Do(context.Background(), http.MethodGet, "http://169.254.169.254:1/", nil, nil); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("direct connection to a metadata address should be refused, got %v", err)
	}
}

func TestErrorBody(t *testing.T) {
	if got := ErrorBody([]byte("  short \n")); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := ErrorBody([]byte(strings.Repeat("x", 3000))); len(got) != errBodyMax {
		t.Errorf("long body kept %d bytes, want %d", len(got), errBodyMax)
	}
}
