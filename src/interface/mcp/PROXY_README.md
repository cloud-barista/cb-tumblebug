# TB-MCP Proxy Bridge

`mcp-simple-proxy.py` lets an MCP client that only speaks stdio, such as Claude Desktop,
use the CB-Tumblebug MCP server, which speaks Streamable HTTP.

```
Claude Desktop (stdio) <-> mcp-simple-proxy.py <-> TB-MCP server (Streamable HTTP, /mcp)
```

The bridge passes on the server's tools, prompts and instructions unchanged. It keeps no
state of its own: every tool call is a request to the server.

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `TB_MCP_URL` | `http://127.0.0.1:8000/mcp` | The MCP endpoint |
| `TB_MCP_TOKEN` | unset | Bearer token, where the endpoint asks for one |

| Deployment | `TB_MCP_URL` | `TB_MCP_TOKEN` |
|------------|--------------|----------------|
| Docker Compose | `http://127.0.0.1:8000/mcp` (the default) | not needed |
| Kubernetes (`make k-up`, `make k-port-forward`) | `http://localhost:8080/mcp` | `make k-mcp-token` (valid 30 days by default) |

## Quick start

```bash
# 1. The server answers (405 or 406 to a plain GET means it is up; 401 means it wants a token)
curl -i http://127.0.0.1:8000/mcp

# 2. The bridge starts
uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
```

`uv` installs FastMCP on first use. The bridge then waits on stdin; stop it with Ctrl+C.

## Claude Desktop

Add to `claude_desktop_config.json`:

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
        "TB_MCP_URL": "http://localhost:8080/mcp",
        "TB_MCP_TOKEN": "<token>"
      }
    }
  }
}
```

Leave `env` out for the Docker Compose endpoint. WSL and Windows variants of the command
are in the [MCP README](./README.md#-claude-desktop-proxy-bridge).

## Testing

```bash
# Drive the bridge with MCP Inspector
npx @modelcontextprotocol/inspector uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
```

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `401 Unauthorized` on every call | The endpoint wants a token and `TB_MCP_TOKEN` is unset, or the token expired: `make k-mcp-token` |
| `Client failed to connect` | The server is not running at `TB_MCP_URL`: `docker compose ps cb-tumblebug-mcp-server`, or `make k-port-forward` |
| `tools/list` is empty | The bridge started but could not reach the server; see the two rows above |
| `ModuleNotFoundError: No module named 'fastmcp'` | Start it with `uv run --with fastmcp`, as above |

The token gives whoever holds it every tool the server publishes, including the ones that
create and delete cloud resources. Keep the client configuration file private, and point
the bridge at a server started with `MCP_READ_ONLY=true` when reads are all it needs.

## References

- [FastMCP proxy servers](https://gofastmcp.com/servers/proxy)
- [CB-Tumblebug MCP server](./README.md)
