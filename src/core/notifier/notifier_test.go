package notifier

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

// fakeNtfy serves the subset of the ntfy API the notifier uses: health, publish, JSON stream.
type fakeNtfy struct {
	mu       sync.Mutex
	seq      int
	subs     map[chan string]struct{}
	writeTok string
	readTok  string
	bodies   []string
}

func newFakeNtfy() *fakeNtfy {
	return &fakeNtfy{subs: map[chan string]struct{}{}, writeTok: "tk_w0000000000000000000000000000", readTok: "tk_r0000000000000000000000000000"}
}

func (f *fakeNtfy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/v1/health":
		fmt.Fprint(w, `{"healthy":true}`)
	case r.Method == http.MethodPost:
		if r.Header.Get("Authorization") != "Bearer "+f.writeTok {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		f.mu.Lock()
		f.seq++
		f.bodies = append(f.bodies, sb.String())
		line, _ := json.Marshal(map[string]any{"id": fmt.Sprintf("m%d", f.seq), "time": time.Now().Unix(), "event": "message", "topic": "t", "message": sb.String()})
		for ch := range f.subs {
			ch <- string(line)
		}
		f.mu.Unlock()
		w.Write(line)
	case strings.HasSuffix(r.URL.Path, "/json"):
		if r.Header.Get("Authorization") != "Bearer "+f.readTok {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		ch := make(chan string, 16)
		f.mu.Lock()
		f.subs[ch] = struct{}{}
		f.mu.Unlock()
		defer func() { f.mu.Lock(); delete(f.subs, ch); f.mu.Unlock() }()
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"event":"open"}`)
		w.(http.Flusher).Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case line := <-ch:
				fmt.Fprintln(w, line)
				w.(http.Flusher).Flush()
			}
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestMain(m *testing.M) {
	if err := kvstore.InitializeStore(kvtest.NewMemoryStore()); err != nil {
		panic(err)
	}
	m.Run()
}

func TestRegisterPublishRoundTrip(t *testing.T) {
	fake := newFakeNtfy()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	defer Unregister()

	info, err := Register(model.NotifierRegisterReq{Url: srv.URL + "/", Topic: "t", WriteToken: "tk_w0000000000000000000000000000", ReadToken: "tk_r0000000000000000000000000000"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if strings.Contains(info.WriteToken, "0000") || !strings.HasPrefix(info.WriteToken, "tk_w00") {
		t.Fatalf("token not masked: %q", info.WriteToken)
	}
	if got := Current(); got == nil || got.Url != srv.URL || got.Source != model.NotifierSourceExternal {
		t.Fatalf("unexpected stored notifier: %+v", got)
	}

	received := make(chan model.NotifierMessage, 1)
	RegisterHandler("initScript", func(msg model.NotifierMessage) { received <- msg })

	// Wait for the subscriber to connect before publishing.
	deadline := time.Now().Add(3 * time.Second)
	for {
		fake.mu.Lock()
		n := len(fake.subs)
		fake.mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	result := Test(3 * time.Second)
	if !result.Success {
		t.Fatalf("round-trip test failed: %+v", result)
	}

	id, err := Publish("initScript", json.RawMessage(`{"nsId":"ns1","infraId":"i1","nodeId":"n1","status":"ok"}`))
	if err != nil || id == "" {
		t.Fatalf("publish: id=%q err=%v", id, err)
	}
	select {
	case msg := <-received:
		if msg.Kind != "initScript" || msg.NsId != "ns1" || msg.InfraId != "i1" || msg.NodeId != "n1" {
			t.Fatalf("handler got unexpected message: %+v", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not receive the published message")
	}
	if recent := RecentMessages(10); len(recent) < 2 || recent[0].Kind != "initScript" {
		t.Fatalf("unexpected recent messages: %+v", recent)
	}
	// LastMessageId is persisted by saveLoop on its own schedule
	saveBy := time.Now().Add(5 * time.Second)
	for {
		stored, _ := load()
		if stored != nil && stored.LastMessageId == id {
			break
		}
		if time.Now().After(saveBy) {
			t.Fatalf("lastMessageId not persisted: %+v", stored)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := Unregister(); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	if Get() != nil {
		t.Fatal("notifier still present after unregister")
	}
	if _, err := Publish("x", nil); err == nil {
		t.Fatal("publish should fail without a notifier")
	}
}

func TestRegisterRejectsUnreachable(t *testing.T) {
	if _, err := Register(model.NotifierRegisterReq{Url: "http://127.0.0.1:1", Topic: "t"}); err == nil {
		t.Fatal("expected health check failure")
	}
}

func TestExtractCA(t *testing.T) {
	out := "noise\n" + caBegin + "\n-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n" + caEnd + "\n"
	ca, err := extractCA(out)
	if err != nil || !strings.HasPrefix(ca, "-----BEGIN CERTIFICATE-----") || !strings.HasSuffix(ca, "-----END CERTIFICATE-----\n") {
		t.Fatalf("ca=%q err=%v", ca, err)
	}
	if _, err := extractCA("nothing"); err == nil {
		t.Fatal("expected error without markers")
	}
	if tok := newAccessToken(); len(tok) != tokenLength || !strings.HasPrefix(tok, "tk_") {
		t.Fatalf("bad token %q", tok)
	}
}

func TestInstallScriptSyntax(t *testing.T) {
	for _, fqdn := range []string{"", "ntfy.example.com"} {
		script := withEnv(map[string]string{
			"TB_NTFY_IP": "1.2.3.4", "TB_NTFY_FQDN": fqdn, "TB_NTFY_TOPIC": "tb-test",
			"TB_NTFY_WRITE_TOKEN": newAccessToken(), "TB_NTFY_READ_TOKEN": newAccessToken(),
			"TB_NTFY_WRITER_HASH": "$2a$10$abc", "TB_NTFY_READER_HASH": "$2a$10$def",
		}, installScript())
		cmd := exec.Command("bash", "-n")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("install script (fqdn=%q) has a syntax error: %v\n%s", fqdn, err, out)
		}
		if !strings.Contains(script, "export TB_NTFY_WRITER_HASH='$2a$10$abc'") || !strings.Contains(script, caBegin) {
			t.Fatalf("rendered script is missing the environment prefix or CA markers")
		}
	}
	if sc, err := exec.LookPath("shellcheck"); err == nil {
		if out, err := exec.Command(sc, "-S", "warning", "scripts/install-ntfy.sh").CombinedOutput(); err != nil {
			t.Fatalf("shellcheck: %v\n%s", err, out)
		}
	}
}

func TestReaderNameValidation(t *testing.T) {
	for _, ok := range []string{"shson", "team-a_1", "ab"} {
		if !readerNamePattern.MatchString(ok) {
			t.Fatalf("%q should be a valid reader name", ok)
		}
	}
	for _, bad := range []string{"", "a", "Shson", "with space", "-lead", strings.Repeat("x", 33)} {
		if readerNamePattern.MatchString(bad) {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}
