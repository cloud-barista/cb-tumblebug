#!/usr/bin/env python3
"""stdio bridge to the CB-Tumblebug MCP server, for clients that only speak stdio.

TB_MCP_URL    endpoint (default http://127.0.0.1:8000/mcp)
TB_MCP_TOKEN  bearer token, when the endpoint asks for one (make k-mcp-token)
"""
import asyncio
import os

from fastmcp import Client, FastMCP

try:
    from fastmcp.server.providers.proxy import ProxyClient
except ImportError:  # older FastMCP
    from fastmcp.server.proxy import ProxyClient

URL = os.environ.get("TB_MCP_URL", "http://127.0.0.1:8000/mcp")
TOKEN = os.environ.get("TB_MCP_TOKEN") or None


def upstream_instructions():
    """The server's own instructions, which a proxy does not pass on by itself."""
    async def read():
        async with Client(URL, auth=TOKEN) as client:
            return client.initialize_result.instructions
    try:
        return asyncio.run(read())
    except Exception:
        return None


proxy = FastMCP.as_proxy(ProxyClient(URL, auth=TOKEN), name="TB-MCP Bridge",
                         instructions=upstream_instructions())

if __name__ == "__main__":
    proxy.run()
