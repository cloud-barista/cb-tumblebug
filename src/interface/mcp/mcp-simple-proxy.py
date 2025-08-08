#!/usr/bin/env python3
"""
Simple TB-MCP Proxy - Minimal stdio bridge for Claude Desktop
"""
from fastmcp import FastMCP
from fastmcp.server.proxy import ProxyClient

# Create a simple proxy to TB-MCP server
proxy = FastMCP.as_proxy(
    ProxyClient("http://127.0.0.1:8000/mcp"),
    name="TB-MCP Bridge"
)

if __name__ == "__main__":
    proxy.run()  # stdio transport
