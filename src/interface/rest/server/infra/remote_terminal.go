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

package infra

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/websocket"
)

// TerminalTicketReq carries optional parameters when requesting a one-time SSH terminal ticket.
type TerminalTicketReq struct {
	UserName string `json:"userName" example:"cb-user"`
}

// TerminalTicketRes is returned by POST /ns/{nsId}/infra/{infraId}/node/{nodeId}/terminal/ticket.
type TerminalTicketRes struct {
	Ticket           string `json:"ticket"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type wsClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type wsServerMsg struct {
	Type       string `json:"type"`
	Data       string `json:"data,omitempty"`
	Message    string `json:"message,omitempty"`
	NodeId     string `json:"nodeId,omitempty"`
	User       string `json:"user,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
	ViaBastion string `json:"viaBastion,omitempty"`
}

func resolveTerminalIdleTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("TB_SSH_TERMINAL_IDLE_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Minute
}

func isAllowedWebSocketOrigin(originHeader string, reqHost string) bool {
	allowed := strings.TrimSpace(os.Getenv("TB_ALLOW_ORIGINS"))
	if allowed == "" || allowed == "*" {
		return true
	}
	if originHeader == "" {
		return true // non-browser / CLI WebSocket client
	}
	parsedOrigin, err := url.Parse(originHeader)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsedOrigin.Host, reqHost) {
		return true
	}
	for _, entry := range strings.Split(allowed, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "*" || strings.EqualFold(entry, originHeader) {
			return true
		}
	}
	return false
}

// RestPostNodeTerminalTicket godoc
// @ID PostNodeTerminalTicket
// @Summary Issue a one-time WebSocket ticket for an interactive Node SSH terminal
// @Description Validates access to the target Infra Node and returns a short-lived, single-use ticket for upgrading GET /ws/terminal to an interactive PTY shell.
// @Tags [MC-Infra] Infra Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Param req body TerminalTicketReq false "Optional SSH username override"
// @Param x-credential-holder header string false "Credential holder ID"
// @Success 200 {object} TerminalTicketRes
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/infra/{infraId}/node/{nodeId}/terminal/ticket [post]
func RestPostNodeTerminalTicket(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")

	var req TerminalTicketReq
	if c.Request().ContentLength > 0 {
		if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": fmt.Sprintf("invalid request body: %v", err)})
		}
	}

	holder := common.CredentialHolderFromContext(c.Request().Context())
	ticket, err := infra.IssueTerminalTicket(nsId, infraId, nodeId, req.UserName, holder, infra.DefaultTerminalTicketTTL)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "cannot find") {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"message": err.Error()})
	}

	return c.JSON(http.StatusOK, TerminalTicketRes{
		Ticket:           ticket.Ticket,
		ExpiresInSeconds: int(infra.DefaultTerminalTicketTTL.Seconds()),
	})
}

// RestGetNodeTerminalWs upgrades an HTTP GET request carrying a valid one-time ticket
// to a full-duplex WebSocket streaming an interactive SSH PTY session (direct or via bastion).
func RestGetNodeTerminalWs(c echo.Context) error {
	origin := c.Request().Header.Get("Origin")
	if !isAllowedWebSocketOrigin(origin, c.Request().Host) {
		return c.JSON(http.StatusForbidden, map[string]string{"message": "WebSocket origin not allowed"})
	}

	ticketToken := c.QueryParam("ticket")
	ticket, err := infra.ConsumeTerminalTicket(ticketToken)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": err.Error()})
	}

	cols, _ := strconv.Atoi(c.QueryParam("cols"))
	rows, _ := strconv.Atoi(c.QueryParam("rows"))
	if cols <= 0 {
		cols = 100
	}
	if rows <= 0 {
		rows = 30
	}

	wsServer := websocket.Server{
		Handshake: func(cfg *websocket.Config, req *http.Request) error {
			// Origin was already validated against TB_ALLOW_ORIGINS above.
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			serveTerminalWebSocket(ws, ticket, cols, rows)
		},
	}

	wsServer.ServeHTTP(c.Response(), c.Request())
	return nil
}

func serveTerminalWebSocket(ws *websocket.Conn, ticket *infra.TerminalTicket, cols, rows int) {
	defer ws.Close()

	var writeMu sync.Mutex
	sendMsg := func(msg wsServerMsg) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return websocket.JSON.Send(ws, msg)
	}

	ctx, cancel := context.WithCancel(common.WithCredentialHolder(context.Background(), ticket.CredentialHolder))
	defer cancel()

	log.Info().
		Str("nsId", ticket.NsId).
		Str("infraId", ticket.InfraId).
		Str("nodeId", ticket.NodeId).
		Str("userName", ticket.UserName).
		Msg("[WebTerminal] Opening interactive SSH session")

	sess, err := infra.OpenNodeTerminalSession(
		ctx,
		ticket.NsId,
		ticket.InfraId,
		ticket.NodeId,
		ticket.UserName,
		cols,
		rows,
		func(status string) {
			_ = sendMsg(wsServerMsg{Type: "status", Message: status})
		},
	)
	if err != nil {
		_ = sendMsg(wsServerMsg{Type: "error", Message: err.Error()})
		return
	}
	defer sess.Close()

	_ = sendMsg(wsServerMsg{
		Type:       "connected",
		NodeId:     ticket.NodeId,
		User:       sess.TargetUser,
		Endpoint:   sess.TargetEndpoint,
		ViaBastion: sess.ViaBastion,
	})

	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())
	idleTimeout := resolveTerminalIdleTimeout()

	done := make(chan struct{})
	var doneOnce sync.Once
	signalDone := func() {
		doneOnce.Do(func() {
			close(done)
			cancel()
			_ = sess.Close()
		})
	}

	// Pump remote PTY output (stdout + stderr) as base64 frames so multi-byte UTF-8
	// sequences (e.g. Hangul) split across read boundaries reassemble cleanly in xterm.js.
	pumpPipe := func(r io.Reader) {
		if r == nil {
			return
		}
		buf := make([]byte, 8192)
		for {
			n, rErr := r.Read(buf)
			if n > 0 {
				encoded := base64.StdEncoding.EncodeToString(buf[:n])
				if sErr := sendMsg(wsServerMsg{Type: "output", Data: encoded}); sErr != nil {
					signalDone()
					return
				}
			}
			if rErr != nil {
				return
			}
		}
	}

	go pumpPipe(sess.Stdout)
	go pumpPipe(sess.Stderr)

	// Wait for remote shell exit
	go func() {
		waitErr := sess.Wait()
		exitMsg := "Session closed."
		if waitErr != nil && !errors.Is(waitErr, io.EOF) {
			exitMsg = fmt.Sprintf("Session exited (%v).", waitErr)
		}
		_ = sendMsg(wsServerMsg{Type: "exit", Message: exitMsg})
		signalDone()
		_ = ws.Close()
	}()

	// SSH keepalive + idle timeout watchdog
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				idleFor := time.Since(time.Unix(0, lastActivity.Load()))
				if idleFor >= idleTimeout {
					_ = sendMsg(wsServerMsg{
						Type:    "exit",
						Message: fmt.Sprintf("Session closed due to inactivity (%s).", idleTimeout),
					})
					signalDone()
					_ = ws.Close()
					return
				}
				_ = sess.KeepAlive()
			}
		}
	}()

	// Client -> SSH stdin / resize / ping loop
	for {
		var raw string
		if rErr := websocket.Message.Receive(ws, &raw); rErr != nil {
			signalDone()
			break
		}

		var msg wsClientMsg
		if jErr := json.Unmarshal([]byte(raw), &msg); jErr != nil {
			continue
		}

		switch msg.Type {
		case "input":
			lastActivity.Store(time.Now().UnixNano())
			if msg.Data != "" {
				if _, wErr := io.WriteString(sess.Stdin, msg.Data); wErr != nil {
					signalDone()
					return
				}
			}
		case "resize":
			lastActivity.Store(time.Now().UnixNano())
			_ = sess.Resize(msg.Cols, msg.Rows)
		case "ping":
			_ = sendMsg(wsServerMsg{Type: "pong"})
		}
	}

	log.Info().
		Str("nsId", ticket.NsId).
		Str("infraId", ticket.InfraId).
		Str("nodeId", ticket.NodeId).
		Msg("[WebTerminal] Interactive SSH session closed")
}
