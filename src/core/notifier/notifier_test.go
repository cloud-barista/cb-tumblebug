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
	closed   bool
	writeTok string
	readTok  string
	bodies   []string
}

func newFakeNtfy() *fakeNtfy {
	return &fakeNtfy{subs: map[chan string]struct{}{}, writeTok: "tk_w0000000000000000000000000000", readTok: "tk_r0000000000000000000000000000"}
}

func (f *fakeNtfy) closeStreams() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	for ch := range f.subs {
		close(ch)
		delete(f.subs, ch)
	}
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
		if f.closed {
			f.mu.Unlock()
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
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
			case line, ok := <-ch:
				if !ok {
					return
				}
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
		if out, err := exec.Command(sc, "-S", "warning", "../embedded/scripts/install-ntfy.sh").CombinedOutput(); err != nil {
			t.Fatalf("shellcheck: %v\n%s", err, out)
		}
	}
}

func TestMultiUrlRegisterSubscribeAndFailover(t *testing.T) {
	fake1, fake2 := newFakeNtfy(), newFakeNtfy()
	srv1 := httptest.NewServer(fake1)
	srv2 := httptest.NewTLSServer(fake2)
	defer srv1.Close()
	defer srv2.Close()
	defer fake1.closeStreams()
	defer fake2.closeStreams()
	defer Unregister()

	pemBlock := "-----BEGIN CERTIFICATE-----\n" + encodePEMBase64(srv2.Certificate().Raw) + "-----END CERTIFICATE-----\n"

	info, err := Register(model.NotifierRegisterReq{
		Urls:       []string{srv1.URL, srv2.URL},
		Topic:      "t",
		WriteToken: fake1.writeTok,
		ReadToken:  fake1.readTok,
		CaCert:     pemBlock,
	})
	if err != nil {
		t.Fatalf("register multi-url: %v", err)
	}
	if len(info.Urls) != 2 || info.Url != srv1.URL {
		t.Fatalf("unexpected urls: %+v", info)
	}

	// Wait for both subscribers to connect.
	deadline := time.Now().Add(3 * time.Second)
	for {
		fake1.mu.Lock()
		n1 := len(fake1.subs)
		fake1.mu.Unlock()
		fake2.mu.Lock()
		n2 := len(fake2.subs)
		fake2.mu.Unlock()
		if (n1 > 0 && n2 > 0) || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	res := Test(3 * time.Second)
	if !res.Success || !strings.Contains(res.Message, "2/2 endpoints ok") {
		t.Fatalf("multi-endpoint round-trip test failed: %+v", res)
	}

	// Simulate a China VM pushing directly to srv2 while a Korea VM pushes to srv1.
	got := make(chan string, 4)
	RegisterHandler("multiCheck", func(msg model.NotifierMessage) { got <- msg.NodeId })

	cur := Current()
	for i, target := range []string{srv1.URL, srv2.URL} {
		payload := []byte(fmt.Sprintf(`{"kind":"multiCheck","nsId":"ns","infraId":"inf","nodeId":"node-%d"}`, i+1))
		if _, err := publishToUrl(cur, target, payload); err != nil {
			t.Fatalf("direct publish to %s: %v", target, err)
		}
	}

	seenNodes := map[string]bool{}
	for len(seenNodes) < 2 {
		select {
		case nid := <-got:
			seenNodes[nid] = true
		case <-time.After(3 * time.Second):
			t.Fatalf("did not receive messages from both servers, got=%v", seenNodes)
		}
	}

	// Wait for per-URL cursors to flush and verify independent LastMessageIds.
	saveBy := time.Now().Add(5 * time.Second)
	for {
		stored, _ := load()
		if stored != nil && stored.LastMessageIds[srv1.URL] != "" && stored.LastMessageIds[srv2.URL] != "" {
			break
		}
		if time.Now().After(saveBy) {
			t.Fatalf("per-url lastMessageIds not persisted: %+v", stored)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Shut down srv1: notifier must remain Active (degraded 1/2) and Publish must fail over to srv2.
	fake1.closeStreams()
	srv1.CloseClientConnections()
	srv1.Close()
	deadline = time.Now().Add(3 * time.Second)
	for {
		c := Current()
		if c != nil && strings.Contains(c.StatusMessage, "degraded (1/2 active)") {
			if c.Status != model.NotifierStatusActive {
				t.Fatalf("expected Active status while 1/2 endpoints remain up, got %+v", c)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected degraded status message after srv1 close, got %+v", Current())
		}
		time.Sleep(30 * time.Millisecond)
	}
	if id, err := Publish("multiCheck", json.RawMessage(`{"nsId":"ns","infraId":"inf","nodeId":"node-failover"}`)); err != nil || id == "" {
		t.Fatalf("publish failover to srv2 failed: id=%q err=%v", id, err)
	}
}

func TestCanUsePublicDnsAndSortNodes(t *testing.T) {
	cnAlibaba := &model.NodeInfo{Id: "ntfy-cn-1", SpecId: "alibaba+cn-shanghai+ecs.e-c1m1.large"}
	cnTencent := &model.NodeInfo{Id: "ntfy-tc-1", ConnectionName: "tencent-ap-shanghai"}
	hkAlibaba := &model.NodeInfo{Id: "ntfy-hk-1", SpecId: "alibaba+cn-hongkong+ecs.e-c1m1.large"}
	krAws := &model.NodeInfo{Id: "ntfy-kr-1", SpecId: "aws+ap-northeast-2+t3.micro"}

	if canUsePublicDns(cnAlibaba) || canUsePublicDns(cnTencent) {
		t.Fatal("Mainland China regions must not be selected for Route53/Let's Encrypt HTTP-01")
	}
	if !canUsePublicDns(hkAlibaba) || !canUsePublicDns(krAws) {
		t.Fatal("Hong Kong and Korea regions must allow Route53/Let's Encrypt")
	}

	// Simulate alphabetical GetInfraInfo order ("ntfy-cn-1" before "ntfy-kr-1")
	nodes := []*model.NodeInfo{cnAlibaba, krAws}
	sortNodesForNotifier(nodes)
	if nodes[0].Id != "ntfy-kr-1" || nodes[1].Id != "ntfy-cn-1" {
		t.Fatalf("expected non-China node sorted first, got [%s, %s]", nodes[0].Id, nodes[1].Id)
	}
}

func encodePEMBase64(der []byte) string {
	const table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	col := 0
	writeChar := func(ch byte) {
		b.WriteByte(ch)
		col++
		if col == 64 {
			b.WriteByte('\n')
			col = 0
		}
	}
	for i := 0; i < len(der); i += 3 {
		rem := len(der) - i
		var v uint32
		v |= uint32(der[i]) << 16
		if rem > 1 {
			v |= uint32(der[i+1]) << 8
		}
		if rem > 2 {
			v |= uint32(der[i+2])
		}
		writeChar(table[(v>>18)&0x3F])
		writeChar(table[(v>>12)&0x3F])
		if rem > 1 {
			writeChar(table[(v>>6)&0x3F])
		} else {
			writeChar('=')
		}
		if rem > 2 {
			writeChar(table[v&0x3F])
		} else {
			writeChar('=')
		}
	}
	if col > 0 {
		b.WriteByte('\n')
	}
	return b.String()
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

func TestValidateEndpoint(t *testing.T) {
	ok := map[string]string{
		"https://ntfy.example.com":   "https://ntfy.example.com",
		"https://ntfy.example.com/":  "https://ntfy.example.com",
		"http://10.0.0.5:8080/ntfy/": "http://10.0.0.5:8080/ntfy",
		"https://192.168.1.10":       "https://192.168.1.10",
		"https://[fd00::1]:443":      "https://[fd00::1]:443",
	}
	for in, want := range ok {
		got, err := validateEndpoint(in, "tb-topic_1")
		if err != nil || got != want {
			t.Fatalf("%s: got %q, %v (want %q)", in, got, err, want)
		}
	}
	bad := []struct{ u, topic string }{
		{"file:///etc/passwd", "t"},
		{"gopher://host", "t"},
		{"https://user:pw@host", "t"},
		{"https://host/?x=1", "t"},
		{"https://host/#frag", "t"},
		{"http://169.254.169.254/latest/meta-data", "t"},
		{"http://[fe80::1]", "t"},
		{"http://0.0.0.0", "t"},
		{"http://metadata.google.internal", "t"},
		{"https://host", "bad/topic"},
		{"https://host", "../admin"},
		{"https://host", ""},
		{"https://host", strings.Repeat("a", 65)},
		{"not a url", "t"},
	}
	for _, c := range bad {
		if _, err := validateEndpoint(c.u, c.topic); err == nil {
			t.Fatalf("%s / %q must be rejected", c.u, c.topic)
		}
	}
}
