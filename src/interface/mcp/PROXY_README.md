# TB-MCP Proxy Bridge

Transport bridge utilities for CB-Tumblebug MCP server integration with Claude Desktop and other MCP clients.

## Overview

These proxy utilities enable Claude Desktop to connect to the TB-MCP SSE server by bridging HTTP/SSE transport to stdio transport, providing seamless AI assistant integration.

## 🔀 Transport Bridging Architecture

```
Claude Desktop (stdio) ←→ Proxy Bridge (stdio/SSE) ←→ TB-MCP Server (HTTP/SSE)
```

The proxy acts as a transport bridge, allowing:
- **Claude Desktop**: Connects via stdio (standard input/output)
- **TB-MCP Server**: Runs on HTTP with SSE (Server-Sent Events) transport
- **Bridge**: Handles protocol translation between stdio and SSE

## 📁 Available Proxy Files

### 1. `mcp-simple-proxy.py` (Recommended for most users)
**Minimal, reliable proxy for standard use cases**

```python
#!/usr/bin/env python3
from fastmcp import FastMCP
from fastmcp.server.proxy import ProxyClient

# Create a simple proxy to TB-MCP server
proxy = FastMCP.as_proxy(
    ProxyClient("http://127.0.0.1:8000/mcp"),
    name="TB-MCP Bridge"
)

if __name__ == "__main__":
    proxy.run()  # stdio transport
```

**Features:**
- ✅ Minimal configuration
- ✅ Automatic session management
- ✅ Error handling via FastMCP defaults
- ✅ Perfect for Claude Desktop integration

### 2. `mcp-remote-proxy.py` (Enhanced with logging)
**Enhanced proxy with detailed logging and configuration**

**Features:**
- ✅ Comprehensive logging
- ✅ Environment variable configuration
- ✅ Detailed startup information
- ✅ Better error messages

### 3. `mcp-advanced-proxy.py` (Development/Debugging)
**Advanced proxy with explicit session management**

**Features:**
- ✅ Explicit session management
- ✅ Custom client factory patterns
- ✅ Detailed debugging information
- ✅ Multiple configuration options

## 🚀 Quick Start

### Step 1: Ensure TB-MCP Server is Running

```bash
# Check if TB-MCP server is accessible
curl http://127.0.0.1:8000/sse

# If not running, start it via Docker Compose
cd /path/to/cb-tumblebug
docker compose up cb-tumblebug-mcp-server
```

### Step 2: Test the Proxy

```bash
# Test the simple proxy
cd /path/to/cb-tumblebug
uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
```

You should see:
```
╭─ FastMCP 2.0 ──────────────────────────────────────────────╮
│    🖥️  Server name:     TB-MCP Bridge                       │
│    📦 Transport:       STDIO                               │
│    🏎️  FastMCP version: 2.11.2                              │
│    🤝 MCP version:     1.12.4                              │
╰────────────────────────────────────────────────────────────╯
```

## 🔧 Claude Desktop Configuration

Add this configuration to your Claude Desktop `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "tumblebug": {
      "command": "uv",
      "args": [
        "run",
        "--with",
        "fastmcp",
        "/path/to/cb-tumblebug/src/interface/mcp/mcp-simple-proxy.py"
      ],
      "env": {
        "TB_MCP_URL": "http://127.0.0.1:8000/sse"
      }
    }
  }
}
```

**Replace `/path/to/cb-tumblebug` with your actual CB-Tumblebug installation path.**

## ⚙️ Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TB_MCP_URL` | `http://127.0.0.1:8000/sse` | TB-MCP server endpoint |
| `PROXY_NAME` | `TB-MCP Bridge` | Proxy server name |

### Example with custom configuration:

```bash
TB_MCP_URL=http://remote-server:8000/sse \
PROXY_NAME="Remote TB-MCP" \
uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
```

## 🔍 Testing and Debugging

### 1. Test TB-MCP Server Connection

```bash
# Check if TB-MCP server responds
curl -i http://127.0.0.1:8000/sse

# Expected: HTTP response (even if "Not Found" - server is running)
```

### 2. Test Proxy with MCP Inspector

```bash
# Install and run MCP Inspector
npx @modelcontextprotocol/inspector

# Then in another terminal, start the proxy:
uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
```

### 3. Debugging Connection Issues

If you see connection errors:

1. **Check TB-MCP server status:**
   ```bash
   docker compose ps cb-tumblebug-mcp-server
   docker compose logs cb-tumblebug-mcp-server
   ```

2. **Verify network connectivity:**
   ```bash
   curl -v http://127.0.0.1:8000/sse
   ```

3. **Check firewall/port settings:**
   ```bash
   netstat -tlnp | grep :8000
   ```

## 🏗️ How It Works

### Transport Bridging Process

1. **Claude Desktop** sends MCP requests via stdio (JSON-RPC)
2. **Proxy Bridge** receives stdio input and converts to HTTP/SSE format
3. **TB-MCP Server** processes the request and responds via SSE
4. **Proxy Bridge** converts SSE response back to stdio format
5. **Claude Desktop** receives the response via stdout

### Session Management

- **Fresh Sessions**: Each request creates a new isolated session (recommended)
- **Session Isolation**: Prevents context mixing between concurrent requests
- **Automatic Reconnection**: Proxy handles connection failures gracefully

## 🔒 Security Considerations

- **Local Connection**: Default configuration uses localhost only
- **No Authentication**: Proxy inherits TB-MCP server authentication
- **Network Exposure**: Be cautious when exposing to external networks
- **Process Isolation**: Each proxy run is isolated

## 🐛 Troubleshooting

### Common Issues

#### "ModuleNotFoundError: No module named 'fastmcp'"
**Solution:** Use `uv run --with fastmcp` to automatically install dependencies

#### "Could not connect to TB-MCP server"
**Solution:** Ensure TB-MCP server is running on the correct port

#### "Connection refused"
**Solution:** Check if port 8000 is available and not blocked

#### Proxy hangs or doesn't respond
**Solution:** 
1. Stop proxy (Ctrl+C)
2. Restart TB-MCP server
3. Restart proxy

### Getting Help

1. Check TB-MCP server logs: `docker compose logs cb-tumblebug-mcp-server`
2. Test direct connection: `curl http://127.0.0.1:8000/sse`
3. Verify proxy startup logs for errors
4. Use the enhanced proxy for detailed debugging information

## 📚 References

- [FastMCP Proxy Documentation](https://gofastmcp.com/servers/proxy)
- [MCP Transport Bridging](https://gofastmcp.com/servers/proxy#transport-bridging)
- [Claude Desktop MCP Configuration](https://docs.anthropic.com/claude/docs/claude-desktop)
- [CB-Tumblebug MCP Server](../README.md)

---

This proxy bridge enables seamless integration between Claude Desktop and CB-Tumblebug's multi-cloud infrastructure management capabilities.
