# CB-Tumblebug MCP Server

A Model Context Protocol (MCP) server implementation for Cloud-Barista CB-Tumblebug, enabling AI assistants like Claude to interact with multi-cloud infrastructure management capabilities.

## Overview

The CB-Tumblebug MCP Server provides a standardized interface for AI assistants to:
- Manage cloud namespaces and resources
- Create and control Multi-Cloud Infrastructure (MCI)
- Search for cloud images and VM specifications
- Execute remote commands on cloud instances
- Monitor and manage cloud resources across multiple providers

**Disclaimer**: This is a proof-of-concept implementation. Use at your own risk and ensure proper security measures are in place before deploying in any environment with sensitive data or production workloads.

## 🔄 Transport Protocol Information

This MCP server uses **Streamable HTTP** transport, which is the current recommended protocol for MCP implementations. The previous SSE (Server-Sent Events) transport has been deprecated in the MCP specification.

### Transport Architecture

```
TB-MCP Server (Streamable HTTP) ←→ AI Assistants
                ↑
        Direct connection (VS Code)
                ↑
        Proxy Bridge (Claude Desktop)
```

**Key Points:**
- **Primary Transport**: Streamable HTTP (`http://127.0.0.1:8000/mcp`)
- **VS Code**: Direct connection supported
- **Claude Desktop**: Requires proxy bridge (`mcp-simple-proxy.py`) due to remote server limitations
- **Protocol Standard**: MCP specification compliant

## 🏗️ Architecture

For detailed system architecture and component interactions, see our comprehensive architecture diagrams:

📋 **[Complete Architecture Documentation](./architecture.md)** - Detailed system diagrams and protocol flows

Key architectural components:
- **[Overall System Architecture](./architecture.md#overall-system-architecture)** - Complete system overview with AI assistants, MCP protocol, and cloud providers
- **[Docker Compose Network](./architecture.md#docker-compose-network-architecture)** - Container communication and networking
- **[MCP Protocol Flows](./architecture.md#mcp-protocol-flow-sse-transport)** - Claude Desktop and VS Code integration patterns
- **[Configuration Flow](./architecture.md#configuration-flow)** - Server initialization and client connection types

## 🚀 Quick Start with Docker Compose (Recommended)

### Prerequisites

- Docker and Docker Compose installed
- CB-Tumblebug source code repository
- Basic understanding of multi-cloud infrastructure concepts

### Step 1: Enable MCP Server in Docker Compose

The MCP server is included as a **Proof of Concept (PoC)** service in the main `docker-compose.yaml` file but is commented out by default.

⚠️ **Security Warning**: This is a PoC implementation. Please review the code thoroughly before using in production environments. The MCP server may expose sensitive infrastructure management capabilities.

1. **Navigate to CB-Tumblebug root directory:**
   ```bash
   cd /path/to/cb-tumblebug
   # or if you have the alias configured:
   cdtb
   ```

2. **Review and configure the MCP server:**

   ⚠️ **IMPORTANT SECURITY NOTICE:**
   - **No public Docker image is available** - the service builds from source
   - **Review the source code** at `src/interface/mcp/tb-mcp.py` before deployment
   - **This is a Proof of Concept** - use appropriate security measures for production

   **Configuration location:** The MCP server configuration is located in the root `docker-compose.yaml` file.

   **Required configuration steps:**
   ```bash
   # 1. Review the MCP server source code
   cat src/interface/mcp/tb-mcp.py
   
   # 2. Edit the main docker-compose.yaml file
   nano docker-compose.yaml  # or use your preferred editor
   ```

   **MCP service configuration in `docker-compose.yaml`:**
   ```yaml
   # TB-MCP (Tumblebug Model Context Protocol Server)
   # Provides MCP interface for CB-Tumblebug to work with AI assistants
   cb-tumblebug-mcp-server:
     build:
       context: ./src/interface/mcp
       dockerfile: Dockerfile
     container_name: cb-tumblebug-mcp-server
     networks:
       - internal_network
       - external_network
     ports:
       - "8000:8000"  # Change if port 8000 is already in use
     environment:
       # Tumblebug API Connection
       - TUMBLEBUG_API_BASE_URL=http://cb-tumblebug:1323/tumblebug
       - TUMBLEBUG_USERNAME=default      # ⚠️ Change in production
       - TUMBLEBUG_PASSWORD=default      # ⚠️ Change in production
       # MCP Server Configuration
       - MCP_SERVER_HOST=0.0.0.0
       - MCP_SERVER_PORT=8000
       - PYTHONUNBUFFERED=1
     depends_on:
       - cb-tumblebug
     restart: unless-stopped
   ```

   **Security recommendations:**
   - Change default username/password for production use
   - Consider using environment files (.env) for sensitive data
   - Limit network access if not needed externally
   - Review firewall settings for port 8000

### Step 2: Launch the Environment

Start all services including the MCP server:

```bash
# Build and start all services (includes MCP server if configured)
make compose

# Alternative: Use docker-compose directly
docker-compose up -d
```

This command will:
- Build the CB-Tumblebug MCP server Docker image
- Start all required services (etcd, PostgreSQL, CB-Spider, CB-Tumblebug, MCP server)
- Configure networking between services

### Step 3: Verify MCP Server

Check if the MCP server is running:

```bash
# Check container status
docker compose ps

# View MCP server logs
docker compose logs -f cb-tumblebug-mcp-server

# Test MCP server endpoint (Streamable HTTP)
curl http://localhost:8000/mcp
```

The MCP server should be accessible at `http://localhost:8000/mcp` using Streamable HTTP transport.

## 🔧 Configuration

### Environment Variables

The MCP server can be configured using the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TUMBLEBUG_API_BASE_URL` | `http://cb-tumblebug:1323/tumblebug` | CB-Tumblebug API endpoint |
| `TUMBLEBUG_USERNAME` | `default` | CB-Tumblebug API username |
| `TUMBLEBUG_PASSWORD` | `default` | CB-Tumblebug API password |
| `MCP_SERVER_HOST` | `0.0.0.0` | MCP server bind address |
| `MCP_SERVER_PORT` | `8000` | MCP server port |

### Custom Configuration

To modify the configuration, edit the environment variables in the `docker-compose.yaml` file:

```yaml
environment:
  - TUMBLEBUG_API_BASE_URL=http://your-custom-endpoint:1323/tumblebug
  - TUMBLEBUG_USERNAME=your-username
  - TUMBLEBUG_PASSWORD=your-password
```

## 🧠 AI Assistant Integration

### Integration Methods Overview

The TB-MCP server supports two integration approaches depending on your AI assistant:

1. **Direct Connection (VS Code)**: Native Streamable HTTP support
2. **Proxy Bridge (Claude Desktop)**: stdio proxy for remote server limitations

For detailed protocol flows and integration patterns, see:
- **[MCP Protocol Flow (Streamable HTTP)](./architecture.md#mcp-protocol-flow-sse-transport)** - VS Code direct integration workflow
- **[Proxy Bridge Pattern](./PROXY_README.md)** - Claude Desktop proxy integration

### 🔧 VS Code MCP Extension (Direct Connection)

VS Code supports Streamable HTTP transport directly, allowing seamless connection to the TB-MCP server.

**Configuration in `.vscode/mcp.json`:**

```json
{
  "servers": {
    "tumblebug": {
      "type": "http",
      "url": "http://127.0.0.1:8000/mcp"
    }
  }
}
```

**Features:**
- ✅ Direct Streamable HTTP connection
- ✅ No proxy required
- ✅ Full MCP protocol support
- ✅ Real-time communication

### 🎭 Claude Desktop (Proxy Bridge)

Claude Desktop has limitations with remote server connections, so we use a proxy bridge to enable seamless integration.

**Step 1: Use the Proxy Bridge**

The `mcp-simple-proxy.py` acts as a stdio bridge:

```python
# mcp-simple-proxy.py - Transport bridge for Claude Desktop
from fastmcp import FastMCP
from fastmcp.server.proxy import ProxyClient

proxy = FastMCP.as_proxy(
    ProxyClient("http://127.0.0.1:8000/mcp"),
    name="TB-MCP Bridge"
)

if __name__ == "__main__":
    proxy.run()  # stdio transport
```

**Step 2: Claude Desktop Configuration**

Add to your `claude_desktop_config.json`:

**For Linux/macOS:**
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
      ]
    }
  }
}
```

**For WSL (Windows Subsystem for Linux):**
```json
{
  "mcpServers": {
    "tumblebug": {
      "command": "wsl.exe",
      "args": [
        "bash",
        "-c",
        "/home/username/.local/bin/uv run --with fastmcp /path/to/cb-tumblebug/src/interface/mcp/mcp-simple-proxy.py"
      ]
    }
  }
}
```

**Features:**
- ✅ Claude Desktop compatibility
- ✅ Streamable HTTP to stdio bridge
- ✅ Automatic dependency management via UV
- ✅ Session isolation

### 🔄 Integration Comparison

| Feature | VS Code (Direct) | Claude Desktop (Proxy) |
|---------|-----------------|------------------------|
| Connection | Direct HTTP | stdio via proxy |
| Setup Complexity | Simple | Moderate |
| Performance | Optimal | Good |
| Protocol | Streamable HTTP | Streamable HTTP → stdio |
| Dependencies | MCP Extension | UV + FastMCP |

## 📚 Core Capabilities

For a visual overview of all available tools and their API mappings, see:
**[MCP Tool Categories and API Mapping](./architecture.md#mcp-tool-categories-and-api-mapping)** - Complete tool organization and endpoint mapping

### 1. Namespace Management
- Create, read, update, delete namespaces
- List and manage namespace resources

### 2. Image and Specification Discovery
- Search cloud images across providers
- Get VM specification recommendations
- Filter by OS type, architecture, provider, region

### 3. Multi-Cloud Infrastructure (MCI) Management
- **Complete MCI Creation Workflow**:
  1. `get_image_search_options()` - Discover available search parameters
  2. `search_images()` - Find suitable OS images
  3. `recommend_vm_spec()` - Get optimal VM specifications
  4. `create_mci_dynamic()` - Create infrastructure with found parameters

- **MCI Operations**:
  - Control MCI lifecycle (suspend, resume, reboot, terminate)
  - Monitor MCI status and health
  - Scale infrastructure dynamically

### 4. Remote Operations
- Execute commands on cloud instances
- Transfer files to/from instances
- Manage instance configurations

### 5. Resource Management
- Manage VNets, Security Groups, SSH Keys
- View cloud connections and regions
- Resource cleanup and optimization


## 🔍 Testing and Debugging

### MCP Inspector (Official Testing Tool)

For testing MCP functionality with Streamable HTTP, use the official MCP Inspector:

```bash
# Test the TB-MCP server directly
npx @modelcontextprotocol/inspector http://localhost:8000/mcp

# Test the proxy bridge (in separate terminal)
cd /path/to/cb-tumblebug
uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
# Then connect MCP Inspector to stdio
```

### Container Logs and Debugging

```bash
# View detailed logs
docker compose logs -f cb-tumblebug-mcp-server

# Access container shell
docker compose exec cb-tumblebug-mcp-server sh

# Check FastMCP version
docker compose exec cb-tumblebug-mcp-server fastmcp version

# Test internal connectivity
docker compose exec cb-tumblebug-mcp-server curl http://cb-tumblebug:1323/tumblebug/readyz

# Test Streamable HTTP endpoint
curl -i http://localhost:8000/mcp
```

### Proxy Bridge Testing

```bash
# Test proxy bridge functionality
cd /path/to/cb-tumblebug

# Ensure TB-MCP server is running
curl http://127.0.0.1:8000/mcp

# Start proxy bridge
uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py

# In another terminal, test with MCP Inspector
npx @modelcontextprotocol/inspector
```

### Troubleshooting Common Issues

#### 1. **Connection Refused**
```bash
# Check if TB-MCP server is running
docker compose ps cb-tumblebug-mcp-server
docker compose logs cb-tumblebug-mcp-server

# Verify port accessibility
curl -v http://127.0.0.1:8000/mcp
```

#### 2. **Proxy Bridge Issues**
```bash
# Check UV installation
which uv
uv --version

# Test FastMCP installation
uv run --with fastmcp python -c "import fastmcp; print('FastMCP available')"

# Check proxy bridge syntax
uv run --with fastmcp python -m py_compile ./src/interface/mcp/mcp-simple-proxy.py
```

#### 3. **Claude Desktop Connection Issues**
- Ensure proxy bridge starts successfully
- Verify path in `claude_desktop_config.json` is correct
- Check Claude Desktop logs for error messages
- Test proxy independently with MCP Inspector

For network architecture and debugging reference, see:
**[Docker Compose Network Architecture](./architecture.md#docker-compose-network-architecture)** - Container communication patterns and port mappings

## 📦 Alternative Installation (Direct Python)

For development or testing purposes, you can run the MCP server directly:

### Prerequisites
- Python 3.12+
- UV package manager

### Installation Steps

1. **Install UV:**
   ```bash
   curl -LsSf https://astral.sh/uv/install.sh | sh
   ```

2. **Navigate to CB-Tumblebug Root:**
   ```bash
   cdtb
   ```

3. **Set environment variables:**
   ```bash
   export TUMBLEBUG_API_BASE_URL=http://localhost:1323/tumblebug
   export TUMBLEBUG_USERNAME=default
   export TUMBLEBUG_PASSWORD=default
   export MCP_SERVER_HOST=0.0.0.0
   export MCP_SERVER_PORT=8000
   ```

4. **Run the server:**
   ```bash
   uv run --with fastmcp,requests ./src/interface/mcp/tb-mcp.py
   ```

   This will start the TB-MCP server with Streamable HTTP transport on `http://127.0.0.1:8000/mcp`.

### Direct Python Configuration

When running directly, ensure CB-Tumblebug is accessible at the configured endpoint. The default configuration assumes CB-Tumblebug is running on `localhost:1323`.

**TB-MCP Server Access:**
- Direct: `http://127.0.0.1:8000/mcp` (Streamable HTTP)
- For Claude Desktop: Use proxy bridge (`mcp-simple-proxy.py`)

## 🔄 Proxy Bridge for Claude Desktop

Due to Claude Desktop's limitations with remote server connections, a proxy bridge is required. See **[Proxy Documentation](./PROXY_README.md)** for detailed setup instructions.

**Quick Setup:**

1. **Ensure TB-MCP server is running** (via Docker or direct Python)
2. **Test proxy bridge:**
   ```bash
   cd /path/to/cb-tumblebug
   uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py
   ```
3. **Configure Claude Desktop** with proxy bridge path

**Files provided:**
- `mcp-simple-proxy.py` - Minimal proxy (recommended)
- `mcp-advanced-proxy.py` - Enhanced proxy with logging
- `claude_desktop_config.json` - Configuration examples
- `PROXY_README.md` - Detailed proxy documentation

## 📁 Project Structure

```
src/interface/mcp/
├── tb-mcp.py                    # Main MCP server implementation
├── mcp-simple-proxy.py          # Proxy bridge for Claude Desktop
├── mcp-advanced-proxy.py        # Enhanced proxy with logging
├── mcp-remote-proxy.py          # Alternative proxy implementation
├── claude_desktop_config.json   # Claude Desktop configuration examples
├── .vscode/mcp.json            # VS Code MCP configuration
├── Dockerfile                   # Docker container configuration
├── README.md                    # This documentation
├── PROXY_README.md             # Detailed proxy documentation
└── architecture.md             # System architecture diagrams
```

## 🚀 Quick Start Guide

### Method 1: Docker Compose (Recommended)

1. **Start TB-MCP server:**
   ```bash
   cd /path/to/cb-tumblebug
   docker compose up cb-tumblebug-mcp-server
   ```

2. **For VS Code users:**
   - Add configuration to `.vscode/mcp.json`
   - Connect directly via Streamable HTTP

3. **For Claude Desktop users:**
   - Use proxy bridge: `uv run --with fastmcp ./src/interface/mcp/mcp-simple-proxy.py`
   - Configure Claude Desktop with proxy path

### Method 2: Direct Python

1. **Start TB-MCP server:**
   ```bash
   uv run --with fastmcp,requests ./src/interface/mcp/tb-mcp.py
   ```

2. **Follow integration steps above based on your AI assistant**

- **PoC Status**: This is a proof-of-concept implementation
- **Code Review**: Thoroughly review all code before production use
- **Network Security**: MCP server exposes infrastructure management capabilities
- **Authentication**: Default credentials should be changed in production
- **Access Control**: Implement proper access controls and monitoring
- **Firewall**: Restrict MCP server access to trusted networks only

## 🤝 Contributing

This MCP server is part of the Cloud-Barista CB-Tumblebug project. For contributions:

1. Review the codebase thoroughly
2. Follow Cloud-Barista development guidelines
3. Test changes extensively before submitting
4. Document security implications of any changes

## 📄 License

This project is licensed under the Apache License 2.0 - see the `LICENSE` file for details.

## 🔗 Related Projects

- [Cloud-Barista CB-Tumblebug](https://github.com/cloud-barista/cb-tumblebug)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [FastMCP (jlowin/fastmcp)](https://github.com/jlowin/fastmcp) - Primary MCP implementation
- [FastMCP (modelcontextprotocol/python-sdk)](https://github.com/modelcontextprotocol/python-sdk) - Official MCP SDK

## 📋 Additional Resources

- **[PROXY_README.md](./PROXY_README.md)** - Comprehensive proxy bridge documentation
- **[architecture.md](./architecture.md)** - System architecture and protocol flows
- **[FastMCP Documentation](https://gofastmcp.com)** - Official FastMCP guides
- **[MCP Inspector](https://modelcontextprotocol.io/docs/tools/inspector)** - Protocol testing tool

---

**Transport Protocol**: Streamable HTTP (MCP compliant)  
**AI Assistant Support**: VS Code (direct), Claude Desktop (proxy bridge)  
**License**: Apache License 2.0

