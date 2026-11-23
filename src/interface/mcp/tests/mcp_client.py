#!/usr/bin/env python3
"""Minimal MCP client (Streamable HTTP) shared by scenarios.py and bench.py.

Standard library only, so the scripts run on a bare host.

    MCP_URL    endpoint (default http://localhost:8080/mcp, the gateway port-forward)
    MCP_TOKEN  bearer token; when unset one is minted with `make k-mcp-token`
"""

import json
import os
import subprocess
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", "..", ".."))
URL = os.environ.get("MCP_URL", "http://localhost:8080/mcp")
PROTOCOL_VERSION = "2025-06-18"


def mint_token():
    """A dev JWT for the MCP route, or None when auth is off."""
    if os.environ.get("MCP_TOKEN"):
        return os.environ["MCP_TOKEN"]
    try:
        out = subprocess.run(["make", "-s", "k-mcp-token"], capture_output=True,
                             text=True, cwd=REPO, timeout=30).stdout
    except Exception:
        return None
    for word in out.split():
        if word.startswith("eyJ"):
            return word.strip()
    return None


class McpError(Exception):
    pass


class McpClient:
    """Calls tools and keeps a trace of every call."""

    def __init__(self, session_id="harness", url=None, token=None, pacing=0.0, verbose=True):
        self.url = url or URL
        self.session_id = session_id
        self.token = token if token is not None else mint_token()
        self.pacing = pacing
        self.verbose = verbose
        self.trace = []
        self._transport = None
        self._next_id = 0
        raw, headers = self._post("initialize", {
            "protocolVersion": PROTOCOL_VERSION, "capabilities": {},
            "clientInfo": {"name": "tb-mcp-harness", "version": "1"}})
        self._transport = headers.get("Mcp-Session-Id") or headers.get("mcp-session-id")
        self.server = self._result(raw)
        self.tools, self.tools_list_bytes = self.list_tools()
        # Only journaled tools take session_id; the others would reject it.
        self._journaled = {t["name"] for t in self.tools
                           if "session_id" in (t.get("inputSchema") or {}).get("properties", {})}

    def _post(self, method, params, timeout=1800):
        self._next_id += 1
        body = {"jsonrpc": "2.0", "id": self._next_id, "method": method, "params": params}
        req = urllib.request.Request(self.url, data=json.dumps(body).encode(), method="POST")
        req.add_header("Content-Type", "application/json")
        req.add_header("Accept", "application/json, text/event-stream")
        if self.token:
            req.add_header("Authorization", f"Bearer {self.token}")
        if self._transport:
            req.add_header("Mcp-Session-Id", self._transport)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.read().decode(), dict(resp.headers)
        except urllib.error.HTTPError as exc:
            raise McpError(f"HTTP {exc.code} from {self.url}: {exc.read().decode()[:200]}") from exc
        except urllib.error.URLError as exc:
            raise McpError(f"cannot reach {self.url}: {exc.reason}") from exc

    @staticmethod
    def _result(raw):
        """The JSON-RPC result from a plain or an SSE-framed body."""
        for line in raw.splitlines():
            line = line.strip()
            if line.startswith("data:"):
                line = line[5:].strip()
            if not line.startswith("{"):
                continue
            try:
                message = json.loads(line)
            except ValueError:
                continue
            if "result" in message:
                return message["result"]
            if "error" in message:
                raise McpError(f"protocol error: {message['error']}")
        raise McpError(f"no JSON-RPC result in response: {raw[:200]}")

    def list_tools(self):
        raw, _ = self._post("tools/list", {})
        return self._result(raw).get("tools") or [], len(raw)

    def list_prompts(self):
        raw, _ = self._post("prompts/list", {})
        return self._result(raw).get("prompts") or []

    def get_prompt(self, name, arguments=None):
        raw, _ = self._post("prompts/get", {"name": name, "arguments": arguments or {}})
        return self._result(raw)

    def call_raw(self, tool, args=None, timeout=1800):
        """One tools/call; returns the raw response body."""
        raw, _ = self._post("tools/call", {"name": tool, "arguments": args or {}}, timeout)
        return raw

    def call(self, tool, args=None, note="", timeout=1800):
        """Call a tool. Returns (ok, payload); payload is the decoded JSON text content."""
        args = dict(args or {})
        if tool in self._journaled:
            args.setdefault("session_id", self.session_id)
        started = time.time()
        raw = self.call_raw(tool, args, timeout)
        elapsed = round(time.time() - started, 1)
        result = self._result(raw)
        text = "".join(part.get("text", "") for part in result.get("content") or []
                       if part.get("type") == "text")
        try:
            payload = json.loads(text)
        except ValueError:
            payload = {"_text": text}
        ok = not result.get("isError")
        code = ""
        if not ok and isinstance(payload, dict) and isinstance(payload.get("error"), dict):
            code = payload["error"].get("code") or ""
        self.trace.append({"tool": tool, "ok": ok, "code": code, "bytes": len(raw),
                           "secs": elapsed, "note": note})
        if self.verbose:
            marker = "ok " if ok else f"ERR {code}"
            print(f"    {marker:22} {tool:38} {elapsed:>6}s {len(raw):>8}B  {note}")
        if self.pacing:
            time.sleep(self.pacing)
        return ok, payload
