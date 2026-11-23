# CB-Tumblebug MCP Server

A Model Context Protocol (MCP) server implementation for Cloud-Barista CB-Tumblebug, enabling AI assistants like Claude to interact with multi-cloud infrastructure management capabilities.

## Overview

The CB-Tumblebug MCP Server provides a standardized interface for AI assistants to:
- Manage cloud namespaces and resources
- Create and control Multi-Cloud Infrastructure (Infra)
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
- **Primary Transport**: Streamable HTTP (`http://127.0.0.1:8000/mcp`), running in
  **stateless mode** (no session continuity required — load-balancer/gateway
  friendly, aligned with the stateless MCP specification 2026-07-28)
- **VS Code**: Direct connection supported
- **Claude Desktop**: Requires proxy bridge (`mcp-simple-proxy.py`) due to remote server limitations
- **Protocol Standard**: MCP specification compliant

## 🏗️ Architecture

For detailed system architecture and component interactions, see our comprehensive architecture diagrams:

📋 **[Complete Architecture Documentation](./architecture.md)** - Detailed system diagrams and protocol flows

Key architectural components:
- **[Overall System Architecture](./architecture.md#overall-system-architecture)** - Complete system overview with AI assistants, MCP protocol, and cloud providers
- **[Docker Compose Network](./architecture.md#docker-compose-network-architecture)** - Container communication and networking
- **[MCP Protocol Flows](./architecture.md#mcp-protocol-flow-streamable-http-transport)** - Claude Desktop and VS Code integration patterns
- **[Configuration Flow](./architecture.md#configuration-flow)** - Server initialization and client connection types

## 🚀 Quick Start with Docker Compose (Recommended)

### Prerequisites

- Docker and Docker Compose installed
- CB-Tumblebug source code repository
- Basic understanding of multi-cloud infrastructure concepts

### Step 1: Review the MCP service

The MCP server is a service in the root `docker-compose.yaml` and is built from
`src/interface/mcp` when the stack starts. It exposes infrastructure management, so read
`tb-mcp.py` and decide who can reach port 8000 before running it anywhere shared.

```yaml
  cb-tumblebug-mcp-server:
    build:
      context: ./src/interface/mcp
      dockerfile: Dockerfile
    ports:
      - "8000:8000"          # change if 8000 is taken, or remove to keep it internal
    environment:
      - TUMBLEBUG_API_BASE_URL=http://cb-tumblebug:1323/tumblebug
      - TUMBLEBUG_USERNAME=${TB_API_USERNAME}
      - TUMBLEBUG_PASSWORD=${TB_API_PASSWORD}
      - MCP_SERVER_HOST=0.0.0.0
      - MCP_SERVER_PORT=8000
      - MCP_LOG_LEVEL=INFO
      # - MCP_READ_ONLY=true   # serve only the tools and actions that change nothing
```

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

## ☸️ Quick Start on Kubernetes (Helm)

The Helm chart ships the MCP server switched off. The `k-*` targets turn it on and keep
the choice across later `make k-up` runs.

```bash
make k-mcp-on              # deploy the MCP server
make k-agentgateway-on     # optional: agentgateway in front of it (enables MCP too)
make k-mcp-auth-on         # optional: require a JWT on the MCP route
make k-mcp-token           # mint a dev token (Authorization: Bearer <token>)
make k-port-forward        # http://localhost:8080/mcp through the gateway
make k-info                # endpoints and client configuration for what is enabled
make k-build-mcp           # run the local tb-mcp.py in the cluster (kind only)
```

`mcp.readOnly: true` in the chart values sets `MCP_READ_ONLY`.

### Spend limits

Limits apply to creations requested through MCP only. The agent can ask for an exception;
only an operator can grant one, with these targets, which write to etcd directly.

```bash
make mcp-budget                                  # policy and requests
make mcp-budget-set PER_CREATION=5 PER_DAY=50    # values not given keep their setting
make mcp-budget-approve ID=req-...               # approve one pending request
make mcp-budget-off                              # stop enforcing, keep the values
```

A creation over a limit is refused with `BUDGET_EXCEEDED` and a `request_id`. Once
approved, the agent repeats the same plan with `budget_ack=<request_id>`. An approval
covers the plan that was refused, once.

## 🔧 Configuration

### Environment Variables

The MCP server can be configured using the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TUMBLEBUG_API_BASE_URL` | `http://localhost:1323/tumblebug` | CB-Tumblebug API endpoint (Docker Compose and Helm set it to the `cb-tumblebug` service) |
| `TUMBLEBUG_USERNAME` | `default` | CB-Tumblebug API username |
| `TUMBLEBUG_PASSWORD` | `default` | CB-Tumblebug API password |
| `TUMBLEBUG_CREDENTIAL_HOLDER` | (empty) | Credential holder sent with every request; empty uses the server default |
| `MCP_SERVER_HOST` | `0.0.0.0` | MCP server bind address |
| `MCP_SERVER_PORT` | `8000` | MCP server port |
| `MCP_LOG_LEVEL` | `INFO` | Log level (`DEBUG`, `INFO`, `WARNING`, `ERROR`) |
| `MCP_DEFAULT_NAMESPACE` | `default` | Namespace used when a tool call names none |
| `MCP_READ_ONLY` | `false` | `true` serves only the tools that change nothing: no create, command or delete tools are published |
| `MCP_NODE_LIST_LIMIT` | `50` | Nodes listed by a summary read; past it the response carries counts and lists the nodes that are not Running first |
| `ETCD_ENDPOINT` | `http://cb-tumblebug-etcd:2379` | Store for the session journal, idempotency keys and spend limits (best effort: tools still work without it) |

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
- **[MCP Protocol Flow (Streamable HTTP)](./architecture.md#mcp-protocol-flow-streamable-http-transport)** - VS Code direct integration workflow
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

On Kubernetes the endpoint is the gateway and asks for a token (`make k-mcp-token`,
valid for 30 days by default):

```json
{
  "servers": {
    "tumblebug": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

A `401` with `ExpiredSignature` means the token ran out: mint a new one.

**Features:**
- ✅ Direct Streamable HTTP connection
- ✅ No proxy required
- ✅ Full MCP protocol support
- ✅ Real-time communication

### 🎭 Claude Desktop (Proxy Bridge)

Claude Desktop has limitations with remote server connections, so we use a proxy bridge to enable seamless integration.

**Step 1: Use the Proxy Bridge**

`mcp-simple-proxy.py` is a stdio bridge to the HTTP endpoint. It passes on the server's
tools, prompts and instructions, and reads two environment variables:

| Variable | Default | Meaning |
|----------|---------|---------|
| `TB_MCP_URL` | `http://127.0.0.1:8000/mcp` | The MCP endpoint (`http://localhost:8080/mcp` behind the Kubernetes gateway) |
| `TB_MCP_TOKEN` | unset | Bearer token, where the endpoint asks for one (`make k-mcp-token`) |

**Step 2: Claude Desktop Configuration**

Add to your `claude_desktop_config.json`:


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

**For Windows native:** Install `uv` first and use the following example.
```json
{
  "mcpServers": {
    "tumblebug": {
      "command": "uv",
      "args": [
        "run",
        "--with",
        "fastmcp",
        "C:\\Users\\admin\\Documents\\mcp-simple-proxy.py"
      ],
      "env": {
        "PYTHONPATH": ".",
        "PYTHONIOENCODING": "utf-8"
      }
    }
  }
}
```

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
      ],
      "env": {
        "TB_MCP_URL": "http://localhost:8080/mcp",
        "TB_MCP_TOKEN": "<token>"
      }
    }
  }
}
```

Leave `env` out for the Docker Compose endpoint, which needs neither.

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

The tool set follows the CB-Tumblebug REST API and the way CB-MapUI uses it. Long
explanations are kept out of the tool descriptions: `get_usage_guide(topic)` returns them
on demand (`provision`, `initscript`, `commands`, `gpu`, `recovery`, `cleanup`,
`services`, `detail`), and the server's `instructions` give the client the short version.

### 1. Plan
- `recommend_vm_spec()`, `search_images()`, `get_search_options()` - pick a spec and an image
- `get_spec_options()` - zones, root disk types and sizes, and whether the provider can
  start the spec right now
- `review_spec_image_pair()`, `get_provisioning_risk()`,
  `recommend_alternative_node_config()` - check one choice, or find a substitute
- `review_infra_dynamic_request()` - validate the whole plan: availability, quota headroom,
  past failure rate, init script size, hourly cost

### 2. Provision
- `create_infra_dynamic()` - create an Infra. It needs the review first (`force_create`)
  and a person's agreement (`confirm`); the refusal carries the plan and its cost.
- `add_nodegroup_dynamic()`, `review_nodegroup_dynamic()`, `scale_out_nodegroup()` - grow an Infra
- `list_infra_templates()`, `provision_infra_from_template()`, `manage_infra_template()` -
  replay a saved configuration, or save a running Infra as one
- `post_script` runs over SSH once the nodes are up; `init_script` runs inside each node at
  first boot. `list_init_script_templates()`, `manage_init_script_template()`,
  `get_init_script_status()` and `get_platform_services()` belong to the latter.

### 3. Operate
- `get_infra()`, `get_infra_list()`, `get_nodegroups()`, `diagnose_infra()` - state and health
- `control_infra()`, `control_node()` - suspend, resume, reboot, terminate, refine
- `execute_command_infra()`, `get_command_result()`, `list_node_command_status()`,
  `manage_command_task()` - remote commands, synchronous or in the background
- `list_predefined_scripts()` - the setups CB-MapUI offers (GPU drivers, vLLM, Ollama,
  Kubernetes, KServe, OpenStack, Ray, ...), with their placeholders and ports
- `transfer_file_infra()`, `download_file_from_node()` - write and read files on nodes
- `manage_labels()`, `add_security_group_rules()` (rules already present are skipped),
  `remove_security_group_rules()`, `manage_bastion()`, `discover_service_endpoints()`

### 4. Services around an Infra
- `manage_object_storage()` - buckets, object listing, presigned upload and download URLs,
  versioning and CORS
- `manage_nlb()` - a provider NLB in front of a node group, or a global one (HAProxy on its
  own Infra, listener port opened) for a whole Infra
- `manage_k8s_cluster()` - provider-managed Kubernetes (EKS, GKE, AKS, ...): cluster, node
  groups, scaling, and a kubeconfig that works without the provider's credential plugin
- `manage_rdbms()` - provider-managed MySQL or MariaDB in an Infra's network: capabilities,
  validation, instance, logical databases, TLS state
- `create_custom_image()` - snapshot a configured node into a reusable image
- `get_dns_hosted_zones()`, `manage_dns_record()` - one name for a deployment

### 5. Recover and clean up
- `retry_failed_nodes()` - re-create failed nodes in place; previews the server's plan by default
- `terminate_infra()`, `release_resources()` - remove an Infra and what it left behind.
  The release is refused while a managed cluster or database still lives in a network it
  would delete.
- `get_resources()`, `manage_resource()` - list resources, delete one, or cancel a
  deletion the provider has not confirmed
- `inspect_csp_resources()`, `register_csp_resources()` - find and adopt resources that exist
  at the CSP but are not managed; `resource_type="audit"` asks the provider whether any
  machine CB-Tumblebug created is still alive

### 6. Platform
- `get_namespaces()`, `manage_namespace()`, `get_connections()`, `list_credential_holders()`
- `register_csp_definition()`, `register_csp_credential()`, `fetch_assets_for_provider()`,
  `unregister_csp_definition()` - add a cloud at runtime, e.g. an OpenStack built on a VM
- `resume_session()`, `clear_session()`, `get_budget_status()`, `request_budget_increase()`

Site-to-site VPN and the autopilot provisioning API are not covered; use CB-MapUI or the
REST API for those.

### Safety properties
- Read-only tools are announced with `readOnlyHint`, so a client can run them without asking.
  `MCP_READ_ONLY=true` publishes only those, plus the reading actions of the `manage_*`
  tools; every write is refused with `READ_ONLY`.
- Tools that destroy or spend take `confirm` and refuse without it. A refused creation
  returns the machines and the hourly cost that would have been created.
- A failure sets `isError` and carries a code the caller can branch on
  (`NOT_FOUND`, `INVALID_ARGUMENT`, `NEEDS_CONFIRMATION`, `REVIEW_REQUIRED`,
  `BUDGET_EXCEEDED`, `CONFLICT`, `TIMEOUT`, ...) with the next step.
- Secret-looking fields are redacted from every response; `get_ssh_private_key()` is the one
  deliberate exception.
- Calls that changed something are journaled per `session_id` (in etcd), without secrets;
  `resume_session()` reads them back. `idempotency_key` makes a retry return the first result.
- Reads default to a summary (`detail="summary"`); `detail="full"` returns the raw records.


## 🔍 Testing and Debugging

### Automated checks

| Command | What it does | Needs |
|---------|--------------|-------|
| `make mcp-check` | Contract checks: the published tool set against `tests/tools.golden.json`, the size of `tools/list`, every request a tool sends (path, query names, body keys) against `src/interface/rest/docs/swagger.yaml`, read-only mode, spend limits, the session journal | Docker only |
| `make mcp-check-update` | Rewrites the golden file after an intended tool change | Docker only |
| `make mcp-scenarios` | Drives real requests through the MCP endpoint: discovery, a reviewed plan, the refusal gates, metadata round trips, reads of existing Infras. Creates nothing billable. | A running cluster (`make k-port-forward`) |
| `make mcp-scenarios ARGS="--spend"` | Adds a one-node lifecycle: create, command, file, bucket with presigned URLs, label, firewall rule, scale out, node delete, terminate, release | CSP credentials; costs a few cents |
| `make mcp-scenarios ARGS="--spend --only services"` | Two web nodes behind a regional and a global load balancer, a custom image and a node started from it, a managed database queried from a node | CSP credentials; about 20 minutes |
| `make mcp-scenarios ARGS="--spend --only kubernetes"` | A managed Kubernetes cluster: create, node group, kubeconfig, scale, delete | CSP credentials; about 30 minutes |
| `make mcp-bench` | Prints the byte size of the common responses against `tests/baseline.json` | A running cluster |

A new tool needs a sample call in `tests/check_contract.py`; the check fails until one is
added. After changing a REST model, `make swag` followed by `make mcp-check` shows which
tools still send the old shape. Details: [tests/README.md](./tests/README.md).

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
   uv run --with "fastmcp==3.4.5",requests,cryptography,pyyaml ./src/interface/mcp/tb-mcp.py
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
- `mcp-simple-proxy.py` - Minimal proxy
- `claude_desktop_config.json` - Configuration examples
- `PROXY_README.md` - Detailed proxy documentation

## 📁 Project Structure

```
src/interface/mcp/
├── tb-mcp.py                    # Main MCP server implementation
├── mcp-simple-proxy.py          # Proxy bridge for Claude Desktop
├── claude_desktop_config.json   # Claude Desktop configuration examples
├── Dockerfile                   # Docker container configuration
├── tests/
│   ├── check_contract.py        # make mcp-check: tool set, schema budget, requests vs the REST API
│   ├── scenarios.py             # make mcp-scenarios: real requests through the MCP endpoint
│   ├── bench.py                 # make mcp-bench: response sizes
│   ├── mcp_client.py            # Minimal MCP client used by the scripts above
│   ├── tools.golden.json        # The published tool set, compared by mcp-check
│   └── baseline.json            # Response sizes, compared by mcp-bench
├── README.md                    # This documentation
├── PROXY_README.md              # Detailed proxy documentation
└── architecture.md              # System architecture diagrams
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
   uv run --with "fastmcp==3.4.5",requests,cryptography,pyyaml ./src/interface/mcp/tb-mcp.py
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

