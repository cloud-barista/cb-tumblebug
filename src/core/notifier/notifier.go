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
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

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
	mu       sync.RWMutex
	info     *model.NotifierInfo
	cancel   context.CancelFunc
	recent   []model.NotifierMessage
	handlers map[string][]Handler
	waiters  map[chan model.NotifierMessage]struct{}
}

var s = &state{handlers: map[string][]Handler{}, waiters: map[chan model.NotifierMessage]struct{}{}}

// Init loads the stored notifier, seeds it from TB_NOTIFIER_* when none is stored, and starts the subscriber.
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
		log.Info().Str("url", info.Url).Str("topic", info.Topic).Msg("notifier: registered from environment")
	}
	s.mu.Lock()
	s.info = info
	s.mu.Unlock()
	startSubscriber(info)
}

func fromEnv() *model.NotifierInfo {
	env := model.NotifierEnvConfig
	if env.Url == "" || env.Topic == "" {
		return nil
	}
	info := &model.NotifierInfo{
		Type: model.NotifierTypeNtfy, Url: strings.TrimRight(env.Url, "/"), Topic: env.Topic,
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

// Register stores an external notifier and starts subscribing to it.
func Register(req model.NotifierRegisterReq) (*model.NotifierInfo, error) {
	if _, err := url.ParseRequestURI(req.Url); err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	info := &model.NotifierInfo{
		Type: model.NotifierTypeNtfy, Url: strings.TrimRight(req.Url, "/"), Topic: req.Topic,
		WriteToken: req.WriteToken, ReadToken: req.ReadToken, CaCert: req.CaCert,
		Source: model.NotifierSourceExternal, RegisteredAt: time.Now(),
	}
	return register(info)
}

func register(info *model.NotifierInfo) (*model.NotifierInfo, error) {
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
	s.info = info
	s.mu.Unlock()
	startSubscriber(info)
	return masked(info), nil
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
	if s.info == nil {
		return nil
	}
	c := *s.info
	return &c
}

// Unregister stops the subscriber and removes the stored notifier.
func Unregister() error {
	stopSubscriber()
	s.mu.Lock()
	s.info = nil
	s.recent = nil
	s.mu.Unlock()
	if err := kvstore.Delete(storeKey); err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	return nil
}

// Publish sends a message of the given kind to the topic and returns the ntfy message id.
func Publish(kind string, data json.RawMessage) (string, error) {
	info := Current()
	if info == nil {
		return "", errors.New("no notifier is registered")
	}
	body := map[string]any{"kind": kind, "time": time.Now().UTC().Format(time.RFC3339)}
	if len(data) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(data, &extra); err != nil {
			return "", fmt.Errorf("data must be a JSON object: %w", err)
		}
		for k, v := range extra {
			if k != "kind" {
				body[k] = v
			}
		}
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, info.Url+"/"+info.Topic, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	if info.WriteToken != "" {
		req.Header.Set("Authorization", "Bearer "+info.WriteToken)
	}
	client, err := httpClient(info, 15*time.Second)
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
		return "", fmt.Errorf("ntfy publish failed: %s %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.Id, nil
}

// Test publishes a probe and waits until the subscriber receives it.
func Test(timeout time.Duration) model.NotifierTestResult {
	start := time.Now()
	nonce := randomToken(12)
	ch := addWaiter()
	defer removeWaiter(ch)
	id, err := Publish("test", json.RawMessage(fmt.Sprintf(`{"nonce":"%s"}`, nonce)))
	if err != nil {
		return model.NotifierTestResult{Success: false, ElapsedMs: time.Since(start).Milliseconds(), Message: err.Error()}
	}
	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			if strings.Contains(msg.Message, nonce) {
				return model.NotifierTestResult{Success: true, MessageId: id, ElapsedMs: time.Since(start).Milliseconds()}
			}
		case <-deadline:
			return model.NotifierTestResult{Success: false, MessageId: id, ElapsedMs: time.Since(start).Milliseconds(),
				Message: "published but not received by the subscriber within " + timeout.String()}
		}
	}
}

// RecentMessages returns the most recent received messages, newest first.
func RecentMessages(limit int) []model.NotifierMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.recent) {
		limit = len(s.recent)
	}
	out := make([]model.NotifierMessage, 0, limit)
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

func health(info *model.NotifierInfo) error {
	client, err := httpClient(info, 10*time.Second)
	if err != nil {
		return err
	}
	resp, err := client.Get(info.Url + "/v1/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}

func httpClient(info *model.NotifierInfo, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if info.CaCert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(info.CaCert)) {
			return nil, errors.New("caCert is not a valid PEM certificate")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool}
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func startSubscriber(info *model.NotifierInfo) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go subscribe(ctx, *info)
}

func stopSubscriber() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// subscribe streams the topic as JSON lines and reconnects with backoff. It stops when ctx is cancelled.
func subscribe(ctx context.Context, info model.NotifierInfo) {
	backoff := 2 * time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := streamOnce(ctx, &info)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			backoff = 2 * time.Second
		}
		if err != nil {
			log.Warn().Err(err).Str("topic", info.Topic).Msg("notifier: subscription interrupted, reconnecting")
			SetStatus(model.NotifierStatusError, err.Error())
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

func streamOnce(ctx context.Context, info *model.NotifierInfo) error {
	u := info.Url + "/" + info.Topic + "/json"
	if info.LastMessageId != "" {
		u += "?since=" + url.QueryEscape(info.LastMessageId)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if info.ReadToken != "" {
		req.Header.Set("Authorization", "Bearer "+info.ReadToken)
	}
	client, err := httpClient(info, 0)
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
	SetStatus(model.NotifierStatusActive, "")
	log.Info().Str("url", info.Url).Str("topic", info.Topic).Msg("notifier: subscribed")

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
		info.LastMessageId = ev.Id
		dispatch(msg)
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return errors.New("stream closed by server")
}

func dispatch(msg model.NotifierMessage) {
	s.mu.Lock()
	if s.info != nil {
		s.info.LastMessageId = msg.Id
		_ = save(s.info)
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
	return &info, nil
}

func save(info *model.NotifierInfo) error {
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return kvstore.Put(storeKey, string(raw))
}

func masked(info *model.NotifierInfo) *model.NotifierInfo {
	if info == nil {
		return nil
	}
	c := *info
	c.WriteToken = maskToken(c.WriteToken)
	c.ReadToken = maskToken(c.ReadToken)
	return &c
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
