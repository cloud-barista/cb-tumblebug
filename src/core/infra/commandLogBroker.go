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

// Package infra is to manage multi-cloud infra
package infra

import (
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

const (
	// maxRingBufferSize is the maximum number of events kept in the ring buffer per request
	maxRingBufferSize = 10000

	// subscriberChannelSize is the buffered channel size for each subscriber
	subscriberChannelSize = 256

	// cleanupDelay is how long to keep the session after CommandDone before cleanup
	cleanupDelay = 30 * time.Second
)

// commandLogSession holds the ring buffer and subscribers for a single xRequestId
type commandLogSession struct {
	mu          sync.RWMutex
	events      []model.CommandStreamEvent // ring buffer
	subscribers map[chan model.CommandStreamEvent]struct{}
	done        bool // set to true when CommandDone is published
}

// CommandLogBroker manages SSE event distribution for remote command execution.
// It is keyed by xRequestId. Each request creates a session with a ring buffer
// and zero or more subscribers (SSE clients).
var commandLogBroker = struct {
	mu       sync.RWMutex
	sessions map[string]*commandLogSession
}{
	sessions: make(map[string]*commandLogSession),
}

// getOrCreateSession returns an existing session or creates a new one
func getOrCreateSession(xRequestId string) *commandLogSession {
	commandLogBroker.mu.RLock()
	session, exists := commandLogBroker.sessions[xRequestId]
	commandLogBroker.mu.RUnlock()

	if exists {
		return session
	}

	commandLogBroker.mu.Lock()
	defer commandLogBroker.mu.Unlock()

	// Double check after acquiring write lock
	if session, exists = commandLogBroker.sessions[xRequestId]; exists {
		return session
	}

	session = &commandLogSession{
		events:      make([]model.CommandStreamEvent, 0, 64),
		subscribers: make(map[chan model.CommandStreamEvent]struct{}),
	}
	commandLogBroker.sessions[xRequestId] = session

	log.Debug().Str("xRequestId", xRequestId).Msg("Created new command log session")
	return session
}

// PublishCommandEvent publishes an event to all subscribers of a given xRequestId.
// Events are also stored in the ring buffer for late-joining subscribers.
func PublishCommandEvent(xRequestId string, event model.CommandStreamEvent) {
	session := getOrCreateSession(xRequestId)

	session.mu.Lock()
	defer session.mu.Unlock()

	// Append to ring buffer (drop oldest if full)
	if len(session.events) >= maxRingBufferSize {
		// Drop the oldest 10% to avoid frequent shifts
		dropCount := maxRingBufferSize / 10
		session.events = session.events[dropCount:]
	}
	session.events = append(session.events, event)

	// Non-blocking send to all subscribers
	for ch := range session.subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber is slow; drop this event for them.
			// They can catch up via ring buffer replay if reconnected.
			log.Warn().Str("xRequestId", xRequestId).Msg("Subscriber channel full, dropping event")
		}
	}

	// If this is the terminal event, mark session done and schedule cleanup
	if event.Type == model.EventCommandDone {
		session.done = true
		go scheduleSessionCleanup(xRequestId)
	}
}

// SubscribeCommandEvents subscribes to events for a given xRequestId.
// It returns a channel of events and a cleanup function that MUST be called when done.
// If the session already has events in the ring buffer, they are replayed first.
func SubscribeCommandEvents(xRequestId string) (<-chan model.CommandStreamEvent, func()) {
	session := getOrCreateSession(xRequestId)

	ch := make(chan model.CommandStreamEvent, subscriberChannelSize)

	session.mu.Lock()

	// Replay buffered events
	for _, evt := range session.events {
		select {
		case ch <- evt:
		default:
			// Channel full during replay; skip oldest events
		}
	}

	// If session is already done, close channel immediately after replay
	if session.done {
		session.mu.Unlock()
		close(ch)
		return ch, func() {} // no-op cleanup since we didn't register
	}

	// Register subscriber
	session.subscribers[ch] = struct{}{}
	session.mu.Unlock()

	// Cleanup function to unregister
	cleanup := func() {
		session.mu.Lock()
		delete(session.subscribers, ch)
		session.mu.Unlock()
	}

	return ch, cleanup
}

// scheduleSessionCleanup removes a session after a delay, giving late subscribers time to drain
func scheduleSessionCleanup(xRequestId string) {
	time.Sleep(cleanupDelay)

	commandLogBroker.mu.Lock()
	defer commandLogBroker.mu.Unlock()

	if session, exists := commandLogBroker.sessions[xRequestId]; exists {
		session.mu.Lock()
		// Close all remaining subscriber channels
		for ch := range session.subscribers {
			close(ch)
			delete(session.subscribers, ch)
		}
		session.mu.Unlock()

		delete(commandLogBroker.sessions, xRequestId)
		log.Debug().Str("xRequestId", xRequestId).Msg("Cleaned up command log session")
	}
}

// CleanupCommandLogSession forces cleanup of a session (e.g., on server shutdown)
func CleanupCommandLogSession(xRequestId string) {
	commandLogBroker.mu.Lock()
	defer commandLogBroker.mu.Unlock()

	if session, exists := commandLogBroker.sessions[xRequestId]; exists {
		session.mu.Lock()
		for ch := range session.subscribers {
			close(ch)
			delete(session.subscribers, ch)
		}
		session.mu.Unlock()

		delete(commandLogBroker.sessions, xRequestId)
	}
}
