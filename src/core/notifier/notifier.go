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

// Package notifier manages the notification channel (ntfy) that VMs publish to and CB-Tumblebug subscribes to.
package notifier

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/managedset"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

const (
	storeKey         = "/system/notifier"
	recentMessageMax = 200
	tokenLength      = 32 // ntfy token: "tk_" + 29 chars
)

// Handler consumes received messages of a kind.
type Handler func(msg model.NotifierMessage)

type state struct {
	mu        sync.RWMutex
	info      *model.NotifierInfo
	cancel    context.CancelFunc
	urlErrors map[string]string
	recent    []model.NotifierMessage
	dirty     bool // LastMessageIds changed since the last save
	handlers  map[string][]Handler
	waiters   map[chan model.NotifierMessage]struct{}
}

var s = &state{handlers: map[string][]Handler{}, waiters: map[chan model.NotifierMessage]struct{}{}}

func init() {
	infra.NotifierProvider = Current
	RegisterHandler("initScript", func(msg model.NotifierMessage) {
		if msg.NsId == "" || msg.InfraId == "" || msg.NodeId == "" {
			return
		}
		var r model.InitScriptResult
		if err := json.Unmarshal([]byte(msg.Message), &r); err != nil || r.Status == "" {
			return
		}
		r.ReportedAt = msg.Time.UTC().Format(time.RFC3339)
		if err := infra.RecordInitScriptResult(msg.NsId, msg.InfraId, msg.NodeId, r); err != nil {
			log.Warn().Err(err).Str("nodeId", msg.NodeId).Msg("notifier: could not record init script result")
		}
	})
}

// Init loads the stored notifier, seeds it from TB_NOTIFIER_* when none is stored, and starts the subscriber(s).
func Init() {
	info, err := load()
	if err != nil {
		log.Error().Err(err).Msg("notifier: failed to load stored notifier")
		return
	}
	if info == nil {
		info = fromEnv()
		if info == nil {
			return
		}
		if err := save(info); err != nil {
			log.Error().Err(err).Msg("notifier: failed to save notifier seeded from env")
			return
		}
		log.Info().Strs("urls", info.EffectiveUrls()).Str("topic", info.Topic).Msg("notifier: registered from environment")
	}
	normalizeInfo(info)
	s.mu.Lock()
	s.info = cloneInfo(info)
	s.mu.Unlock()
	startSubscriber(info)
}

func fromEnv() *model.NotifierInfo {
	env := model.NotifierEnvConfig
	if env.Url == "" || env.Topic == "" {
		return nil
	}
	cleanUrls, err := validateEndpoints(splitUrlList(env.Url), env.Topic)
	if err != nil {
		log.Error().Err(err).Msg("notifier: TB_NOTIFIER_NTFY_URL/TOPIC rejected")
		return nil
	}
	info := &model.NotifierInfo{
		Type: model.NotifierTypeNtfy, Url: cleanUrls[0], Urls: cleanUrls, Topic: env.Topic,
		WriteToken: env.WriteToken, ReadToken: env.ReadToken,
		Source: model.NotifierSourceExternal, Status: model.NotifierStatusActive, RegisteredAt: time.Now(),
	}
	if env.CaCertFile != "" {
		pem, err := os.ReadFile(env.CaCertFile)
		if err != nil {
			log.Error().Err(err).Msg("notifier: cannot read TB_NOTIFIER_NTFY_CA_CERT_FILE")
			return nil
		}
		info.CaCert = string(pem)
	}
	return info
}

// RegisterHandler subscribes fn to received messages whose kind matches.
func RegisterHandler(kind string, fn Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[kind] = append(s.handlers[kind], fn)
}

// topicPattern is the ntfy topic rule; the topic is appended to the server URL as a path segment.
var topicPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// validateEndpoint checks the server URL (see managedset.ValidateServerUrl) and the topic, which is appended
// to the URL as a path segment.
func validateEndpoint(rawUrl, topic string) (string, error) {
	cleanUrl, err := managedset.ValidateServerUrl(rawUrl)
	if err != nil {
		return "", err
	}
	if !topicPattern.MatchString(topic) {
		return "", fmt.Errorf("invalid topic %q: use 1-64 letters, digits, '_' or '-'", topic)
	}
	return cleanUrl, nil
}

func validateEndpoints(rawUrls []string, topic string) ([]string, error) {
	if len(rawUrls) == 0 {
		return nil, errors.New("at least one server url is required")
	}
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range rawUrls {
		u, err := validateEndpoint(raw, topic)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[u]; !dup {
			seen[u] = struct{}{}
			out = append(out, u)
		}
	}
	return out, nil
}

func splitUrlList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// Register stores an external notifier (one or more ntfy URLs sharing a topic) and starts subscribing to it.
func Register(req model.NotifierRegisterReq) (*model.NotifierInfo, error) {
	var rawUrls []string
	if req.Url != "" {
		rawUrls = append(rawUrls, splitUrlList(req.Url)...)
	}
	for _, u := range req.Urls {
		rawUrls = append(rawUrls, splitUrlList(u)...)
	}
	cleanUrls, err := validateEndpoints(rawUrls, req.Topic)
	if err != nil {
		return nil, err
	}
	info := &model.NotifierInfo{
		Type: model.NotifierTypeNtfy, Url: cleanUrls[0], Urls: cleanUrls, Topic: req.Topic,
		WriteToken: req.WriteToken, ReadToken: req.ReadToken, CaCert: req.CaCert,
		Source: model.NotifierSourceExternal, RegisteredAt: time.Now(),
	}
	return register(info)
}

func register(info *model.NotifierInfo) (*model.NotifierInfo, error) {
	normalizeInfo(info)
	if err := health(info); err != nil {
		return nil, fmt.Errorf("ntfy health check failed: %w", err)
	}
	info.Status = model.NotifierStatusActive
	info.StatusMessage = ""
	stopSubscriber()
	if err := save(info); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.info = cloneInfo(info)
	s.mu.Unlock()
	startSubscriber(info)
	return masked(info), nil
}

func normalizeInfo(info *model.NotifierInfo) {
	if info == nil {
		return
	}
	urls := info.EffectiveUrls()
	if len(urls) > 0 {
		info.Urls = urls
		info.Url = urls[0]
	}
}

func cloneInfo(info *model.NotifierInfo) *model.NotifierInfo {
	if info == nil {
		return nil
	}
	c := *info
	if info.Urls != nil {
		c.Urls = append([]string(nil), info.Urls...)
	}
	if info.ManagedVNetIds != nil {
		c.ManagedVNetIds = append([]string(nil), info.ManagedVNetIds...)
	}
	if info.ManagedSecurityGroupIds != nil {
		c.ManagedSecurityGroupIds = append([]string(nil), info.ManagedSecurityGroupIds...)
	}
	if info.ManagedSshKeyIds != nil {
		c.ManagedSshKeyIds = append([]string(nil), info.ManagedSshKeyIds...)
	}
	if info.ManagedDnsRecords != nil {
		c.ManagedDnsRecords = append([]string(nil), info.ManagedDnsRecords...)
	}
	if info.Readers != nil {
		c.Readers = append([]string(nil), info.Readers...)
	}
	if info.LastMessageIds != nil {
		c.LastMessageIds = make(map[string]string, len(info.LastMessageIds))
		for k, v := range info.LastMessageIds {
			c.LastMessageIds[k] = v
		}
	}
	return &c
}

// Get returns the registered notifier with tokens masked, or nil.
func Get() *model.NotifierInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return masked(s.info)
}

// Current returns the registered notifier with secrets, or nil.
func Current() *model.NotifierInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneInfo(s.info)
}

// Unregister stops the subscriber(s) and removes the stored notifier.
func Unregister() error {
	stopSubscriber()
	s.mu.Lock()
	s.info = nil
	s.urlErrors = nil
	s.recent = nil
	s.mu.Unlock()
	if err := kvstore.Delete(storeKey); err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	return nil
}

// Publish sends a message of the given kind to the first reachable notifier endpoint and returns the ntfy message id.
func Publish(kind string, data json.RawMessage) (string, error) {
	info := Current()
	if info == nil {
		return "", errors.New("no notifier is registered")
	}
	payload, err := buildPublishPayload(kind, data)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, targetUrl := range info.EffectiveUrls() {
		id, err := publishToUrl(info, targetUrl, payload)
		if err == nil {
			return id, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func buildPublishPayload(kind string, data json.RawMessage) ([]byte, error) {
	body := map[string]any{"kind": kind, "time": time.Now().UTC().Format(time.RFC3339)}
	if len(data) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(data, &extra); err != nil {
			return nil, fmt.Errorf("data must be a JSON object: %w", err)
		}
		for k, v := range extra {
			if k != "kind" {
				body[k] = v
			}
		}
	}
	return json.Marshal(body)
}

func publishToUrl(info *model.NotifierInfo, targetUrl string, payload []byte) (string, error) {
	req, err := http.NewRequest(http.MethodPost, targetUrl+"/"+info.Topic, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	if info.WriteToken != "" {
		req.Header.Set("Authorization", "Bearer "+info.WriteToken)
	}
	client, err := httpClientForUrl(info, targetUrl, 15*time.Second)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ntfy publish failed (%s): %s %s", targetUrl, resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.Id, nil
}

// Test publishes a probe to every registered endpoint URL and waits until the subscriber receives each one.
func Test(timeout time.Duration) model.NotifierTestResult {
	start := time.Now()
	info := Current()
	if info == nil {
		return model.NotifierTestResult{Success: false, Message: "no notifier is registered"}
	}
	urls := info.EffectiveUrls()
	ch := addWaiter()
	defer removeWaiter(ch)

	deadline := time.After(timeout)
	var ids []string
	for _, targetUrl := range urls {
		nonce := randomToken(12)
		payload, _ := buildPublishPayload("test", json.RawMessage(fmt.Sprintf(`{"nonce":"%s"}`, nonce)))
		id, err := publishToUrl(info, targetUrl, payload)
		if err != nil {
			return model.NotifierTestResult{Success: false, ElapsedMs: time.Since(start).Milliseconds(), Message: err.Error()}
		}
		ids = append(ids, id)
		received := false
		for !received {
			select {
			case msg := <-ch:
				if strings.Contains(msg.Message, nonce) {
					received = true
				}
			case <-deadline:
				return model.NotifierTestResult{
					Success: false, MessageId: strings.Join(ids, ","), ElapsedMs: time.Since(start).Milliseconds(),
					Message: fmt.Sprintf("published to %s but not received by the subscriber within %s", targetUrl, timeout),
				}
			}
		}
	}
	var note string
	if len(urls) > 1 {
		note = fmt.Sprintf("%d/%d endpoints ok", len(urls), len(urls))
	}
	return model.NotifierTestResult{
		Success: true, MessageId: strings.Join(ids, ","), ElapsedMs: time.Since(start).Milliseconds(), Message: note,
	}
}

// RecentMessages returns the most recent received messages, newest first.
func RecentMessages(limit int) []model.NotifierMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.recent) {
		limit = len(s.recent)
	}
	out := make([]model.NotifierMessage, 0, recentMessageMax)
	for i := len(s.recent) - 1; i >= len(s.recent)-limit; i-- {
		out = append(out, s.recent[i])
	}
	return out
}

// updateInfo applies fn to the stored notifier and persists it.
func updateInfo(fn func(*model.NotifierInfo)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info == nil {
		return
	}
	fn(s.info)
	_ = save(s.info)
}

// SetStatus records an operational status on the stored notifier.
func SetStatus(status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info == nil {
		return
	}
	s.info.Status = status
	s.info.StatusMessage = message
	_ = save(s.info)
}

func recordUrlStatus(targetUrl string, streamErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info == nil {
		return
	}
	if s.urlErrors == nil {
		s.urlErrors = map[string]string{}
	}
	if streamErr == nil {
		s.urlErrors[targetUrl] = ""
	} else {
		s.urlErrors[targetUrl] = streamErr.Error()
	}
	urls := s.info.EffectiveUrls()
	activeCount := 0
	var errs []string
	for _, u := range urls {
		if msg := s.urlErrors[u]; msg != "" {
			errs = append(errs, u+": "+msg)
		} else {
			activeCount++
		}
	}
	var nextStatus, nextMsg string
	if activeCount > 0 {
		nextStatus = model.NotifierStatusActive
		if len(errs) > 0 {
			nextMsg = fmt.Sprintf("degraded (%d/%d active): %s", activeCount, len(urls), strings.Join(errs, "; "))
		}
	} else {
		nextStatus = model.NotifierStatusError
		nextMsg = strings.Join(errs, "; ")
	}
	if s.info.Status != nextStatus || s.info.StatusMessage != nextMsg {
		s.info.Status = nextStatus
		s.info.StatusMessage = nextMsg
		_ = save(s.info)
	}
}

func health(info *model.NotifierInfo) error {
	for _, targetUrl := range info.EffectiveUrls() {
		client, err := httpClientForUrl(info, targetUrl, 10*time.Second)
		if err != nil {
			return err
		}
		resp, err := client.Get(targetUrl + "/v1/health")
		if err != nil {
			return fmt.Errorf("%s: %w", targetUrl, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: unexpected status %s", targetUrl, resp.Status)
		}
	}
	return nil
}

func httpClientForUrl(info *model.NotifierInfo, targetUrl string, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if info.CaCert != "" {
		pinned := x509.NewCertPool()
		if !pinned.AppendCertsFromPEM([]byte(info.CaCert)) {
			return nil, errors.New("caCert is not a valid PEM certificate")
		}
		pool := pinned
		if u, err := url.Parse(targetUrl); err == nil && u.Hostname() != "" && net.ParseIP(u.Hostname()) == nil {
			// Domain endpoints (e.g. Let's Encrypt on Node 1) validate against system CAs; custom domain CAs still work.
			if sysPool, sysErr := x509.SystemCertPool(); sysErr == nil && sysPool != nil {
				sysPool.AppendCertsFromPEM([]byte(info.CaCert))
				pool = sysPool
			}
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool}
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func startSubscriber(info *model.NotifierInfo) {
	ctx, cancel := context.WithCancel(context.Background())
	urls := info.EffectiveUrls()
	s.mu.Lock()
	s.cancel = cancel
	s.urlErrors = make(map[string]string, len(urls))
	s.mu.Unlock()
	snap := cloneInfo(info)
	for _, targetUrl := range urls {
		go subscribe(ctx, *snap, targetUrl)
	}
	go saveLoop(ctx)
}

func stopSubscriber() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// saveLoop persists LastMessageIds at most once per interval while messages arrive, and once more when the
// subscriber stops. Messages received after the last save are re-delivered on restart; handlers are idempotent.
func saveLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	flush := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.dirty && s.info != nil {
			if err := save(s.info); err == nil {
				s.dirty = false
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-ticker.C:
			flush()
		}
	}
}

// subscribe streams the topic from targetUrl as JSON lines and reconnects with backoff until ctx is cancelled.
func subscribe(ctx context.Context, info model.NotifierInfo, targetUrl string) {
	backoff := 2 * time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := streamOnce(ctx, &info, targetUrl)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			backoff = 2 * time.Second
		}
		if err != nil {
			log.Warn().Err(err).Str("url", targetUrl).Str("topic", info.Topic).Msg("notifier: subscription interrupted, reconnecting")
			recordUrlStatus(targetUrl, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

func lastMessageIdFor(targetUrl string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.info == nil {
		return ""
	}
	if id := s.info.LastMessageIds[targetUrl]; id != "" {
		return id
	}
	if len(s.info.EffectiveUrls()) == 1 && targetUrl == s.info.Url {
		return s.info.LastMessageId
	}
	return ""
}

func streamOnce(ctx context.Context, info *model.NotifierInfo, targetUrl string) error {
	u := targetUrl + "/" + info.Topic + "/json"
	if since := lastMessageIdFor(targetUrl); since != "" {
		u += "?since=" + url.QueryEscape(since)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if info.ReadToken != "" {
		req.Header.Set("Authorization", "Bearer "+info.ReadToken)
	}
	client, err := httpClientForUrl(info, targetUrl, 0)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("subscribe failed: %s %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	recordUrlStatus(targetUrl, nil)
	log.Info().Str("url", targetUrl).Str("topic", info.Topic).Msg("notifier: subscribed")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var ev struct {
			Id      string `json:"id"`
			Time    int64  `json:"time"`
			Event   string `json:"event"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil || ev.Event != "message" {
			continue
		}
		msg := model.NotifierMessage{Id: ev.Id, Time: time.Unix(ev.Time, 0).UTC(), Message: ev.Message}
		var meta struct {
			Kind        string `json:"kind"`
			NsId        string `json:"nsId"`
			InfraId     string `json:"infraId"`
			NodeGroupId string `json:"nodeGroupId"`
			NodeId      string `json:"nodeId"`
		}
		if json.Unmarshal([]byte(ev.Message), &meta) == nil {
			msg.Kind, msg.NsId, msg.InfraId, msg.NodeGroupId, msg.NodeId = meta.Kind, meta.NsId, meta.InfraId, meta.NodeGroupId, meta.NodeId
		}
		dispatchFrom(targetUrl, msg)
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return errors.New("stream closed by server")
}

func dispatchFrom(targetUrl string, msg model.NotifierMessage) {
	s.mu.Lock()
	if s.info != nil {
		// LastMessageIds is persisted by saveLoop, not per message: thousands of Nodes can report at once
		s.info.LastMessageId = msg.Id
		if s.info.LastMessageIds == nil {
			s.info.LastMessageIds = map[string]string{}
		}
		s.info.LastMessageIds[targetUrl] = msg.Id
		s.dirty = true
	}
	s.recent = append(s.recent, msg)
	if len(s.recent) > recentMessageMax {
		s.recent = s.recent[len(s.recent)-recentMessageMax:]
	}
	handlers := append([]Handler{}, s.handlers[msg.Kind]...)
	for ch := range s.waiters {
		select {
		case ch <- msg:
		default:
		}
	}
	s.mu.Unlock()
	for _, h := range handlers {
		h(msg)
	}
}

func addWaiter() chan model.NotifierMessage {
	ch := make(chan model.NotifierMessage, 16)
	s.mu.Lock()
	s.waiters[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func removeWaiter(ch chan model.NotifierMessage) {
	s.mu.Lock()
	delete(s.waiters, ch)
	s.mu.Unlock()
}

func load() (*model.NotifierInfo, error) {
	raw, exists, err := kvstore.Get(storeKey)
	if err != nil || !exists {
		return nil, err
	}
	var info model.NotifierInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, err
	}
	normalizeInfo(&info)
	return &info, nil
}

func save(info *model.NotifierInfo) error {
	normalizeInfo(info)
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return kvstore.Put(storeKey, string(raw))
}

func masked(info *model.NotifierInfo) *model.NotifierInfo {
	c := cloneInfo(info)
	if c == nil {
		return nil
	}
	c.WriteToken = maskToken(c.WriteToken)
	c.ReadToken = maskToken(c.ReadToken)
	return c
}

func maskToken(t string) string {
	if len(t) <= 6 {
		return strings.Repeat("*", len(t))
	}
	return t[:6] + strings.Repeat("*", len(t)-6)
}

// randomToken returns n lowercase alphanumeric characters.
func randomToken(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// newAccessToken returns an ntfy-format access token.
func newAccessToken() string {
	return "tk_" + randomToken(tokenLength-3)
}
