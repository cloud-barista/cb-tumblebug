# CB-TB MCP Server Architecture Diagram

## Overall System Architecture

```mermaid
graph TB
    subgraph "AI Assistants Layer"
        direction LR
        Claude[Claude Desktop]
        VSCode[VS Code Copilot]
        MCPInspector[MCP Inspector]
    end
    
    subgraph "MCP Protocol Layer"
        direction LR
        ProxyBridge[mcp-simple-proxy.py<br/>stdio → Streamable HTTP Bridge]
        DirectHTTP[Direct Streamable HTTP<br/>Connection]
    end
    
    subgraph "Docker Environment"
        direction TB
        
        subgraph "TB-MCP Container"
            TBMCPCode[TB-MCP Server<br/>FastMCP + Streamable HTTP :8000]
        end
        
        subgraph "Core Services"
            direction LR
            TBCore[CB-Tumblebug<br/>:1323]
            Spider[CB-Spider<br/>:1024]
        end
        
        subgraph "Data Storage"
            direction LR
            ETCD[ETCD<br/>:2379]
            Postgres[PostgreSQL<br/>:5432<br/>External Access]
        end
    end
    
    subgraph "Cloud Providers"
        direction LR
        AWS[AWS]
        Azure[Azure]
        GCP[GCP]
        Others[Others...]
    end
    
    %% Main AI Assistant Flows
    Claude -->|MCP stdio| ProxyBridge
    VSCode -->|MCP Streamable HTTP| DirectHTTP
    MCPInspector -->|MCP Streamable HTTP| DirectHTTP
    
    %% Proxy Bridge Flow
    ProxyBridge -->|Streamable HTTP :8000/mcp| TBMCPCode
    DirectHTTP -->|Streamable HTTP :8000/mcp| TBMCPCode
    
    %% Internal Service Flow
    TBMCPCode -->|REST API| TBCore
    TBCore -->|REST API| Spider
    TBCore -->|gRPC| ETCD
    TBCore -->|SQL| Postgres
    
    %% Cloud Integration
    Spider -->|Cloud APIs| AWS
    Spider -->|Cloud APIs| Azure
    Spider -->|Cloud APIs| GCP
    Spider -->|Cloud APIs| Others
    
    %% Styling for clarity
    classDef aiLayer fill:#e3f2fd,stroke:#1976d2,stroke-width:2px
    classDef mcpLayer fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px
    classDef dockerLayer fill:#e8f5e8,stroke:#388e3c,stroke-width:2px
    classDef cloudLayer fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    classDef primaryFlow stroke:#d32f2f,stroke-width:3px
    classDef proxyFlow stroke:#ff6f00,stroke-width:3px
    
    class Claude,VSCode,MCPInspector aiLayer
    class ProxyBridge,DirectHTTP mcpLayer
    class TBMCPCode,TBCore,Spider,ETCD,Postgres dockerLayer
    class AWS,Azure,GCP,Others cloudLayer
```

## MCP Protocol Flow (Streamable HTTP Transport)

```mermaid
sequenceDiagram
    participant C as Claude Desktop
    participant P as mcp-simple-proxy.py
    participant TB as TB-MCP.py (FastMCP)
    participant API as CB-TB API
    participant CSP as Cloud Providers
    
    Note over C,P: Claude Desktop Setup
    C->>P: Initialize stdio MCP connection
    P->>TB: Connect via Streamable HTTP to :8000/mcp
    
    Note over TB,API: MCP Server Initialization
    TB->>TB: Load MCP tools & prompts
    TB->>API: Test API connectivity
    
    Note over C,CSP: User Request Flow
    C->>P: User request: "Create AWS VM"
    P->>TB: MCP JSON-RPC over Streamable HTTP
    TB->>TB: Execute tool: create_mci_dynamic()
    
    TB->>API: POST /ns/{ns}/mciDynamic
    API->>CSP: Provision VM resources
    CSP-->>API: Resource creation response
    API-->>TB: MCI creation result
    TB-->>P: MCP response over Streamable HTTP
    P-->>C: Display result to user via stdio
    
    Note over C,CSP: Error Handling & Retry
    alt API Error
        API-->>TB: Error response
        TB->>TB: Retry with different CSP/region
        TB->>API: POST with alternative config
        API->>CSP: Retry provision
    end
```

## VS Code Copilot Direct Integration

```mermaid
sequenceDiagram
    participant VS as VS Code Copilot
    participant TB as TB-MCP.py (FastMCP)
    participant API as CB-TB API
    
    Note over VS,TB: Streamable HTTP Connection
    VS->>TB: Streamable HTTP connection to :8000/mcp
    TB->>TB: Initialize MCP tools
    
    Note over VS,API: Tool Execution
    VS->>TB: MCP tool request
    TB->>TB: Execute Python function
    TB->>API: REST API call
    API-->>TB: API response
    TB-->>VS: MCP response via Streamable HTTP
```

## PostgreSQL MCP Server Direct Database Access

```mermaid
sequenceDiagram
    participant AI as AI Assistant<br/>(Claude/VS Code)
    participant PGMCP as PostgreSQL MCP Server<br/>modelcontextprotocol/server-postgres
    participant PG as CB-TB PostgreSQL<br/>:5432
    
    Note over AI,PGMCP: PostgreSQL MCP Setup
    AI->>PGMCP: Initialize MCP connection (stdio)
    PGMCP->>PG: Connect to PostgreSQL :5432
    
    Note over AI,PG: Direct Database Analysis
    AI->>PGMCP: "Show me AWS specs with >4 vCPUs"
    PGMCP->>PG: SELECT * FROM spec WHERE provider='aws' AND vcpu > 4
    PG-->>PGMCP: Query results
    PGMCP-->>AI: Formatted spec data
    
    AI->>PGMCP: "Compare Ubuntu images across regions"
    PGMCP->>PG: SELECT region, COUNT(*) FROM image WHERE os_type LIKE '%ubuntu%' GROUP BY region
    PG-->>PGMCP: Regional image statistics
    PGMCP-->>AI: Analysis results
    
    Note over AI,PG: Advanced Analytics
    AI->>PGMCP: "Find cost-optimal specs for ML workloads"
    PGMCP->>PG: Complex JOIN query across spec and pricing tables
    PG-->>PGMCP: Optimized recommendations
    PGMCP-->>AI: ML-ready infrastructure suggestions
```

## Docker Compose Network Architecture

```mermaid
graph TB
    subgraph "External Network (bridge)"
        direction LR
        ExtPort8000[":8000<br/>MCP Streamable HTTP"]
        ExtPort1323[":1323<br/>TB API"]
        ExtPort1024[":1024<br/>Spider API"]
        ExtPort5432[":5432<br/>PostgreSQL<br/>External Access"]
    end
    
    subgraph "Internal Network"
        direction TB
        
        subgraph "MCP Container"
            MCPApp[TB-MCP.py<br/>+ FastMCP Server<br/>Streamable HTTP :8000/mcp]
            MCPEnv[Environment:<br/>CB-TB_API_BASE_URL=<br/>http://cb-tumblebug:1323/tumblebug]
        end
        
        subgraph "CB-TB Container"
            TBApp[CB-TB Core<br/>Go Application]
            TBConfig[Config:<br/>TB_SPIDER_REST_URL=<br/>http://cb-spider:1024/spider]
        end
        
        subgraph "Spider Container"
            SpiderApp[CB-Spider<br/>CSP Driver Manager]
        end
        
        subgraph "Data Layer"
            ETCDData[ETCD<br/>:2379<br/>Metadata]
            PostgresData[PostgreSQL<br/>:5432<br/>Specs & Images<br/>External Network Enabled]
        end
    end
    
    subgraph "External Clients"
        Client1[Claude Desktop<br/>+ mcp-simple-proxy.py]
        Client2[VS Code Copilot<br/>Direct HTTP]
        Client3[MCP Inspector<br/>Direct HTTP]
        Client4[PostgreSQL MCP Server<br/>modelcontextprotocol/server-postgres<br/>npx auto-execution]
    end
    
    %% External connections
    Client1 -->|stdio → HTTP proxy| ExtPort8000
    Client2 -->|Streamable HTTP| ExtPort8000
    Client3 -->|Streamable HTTP| ExtPort8000
    Client4 -->|SQL Queries<br/>Direct DB Access| ExtPort5432
    
    %% Port mapping
    ExtPort8000 --> MCPApp
    ExtPort1323 --> TBApp
    ExtPort1024 --> SpiderApp
    ExtPort5432 --> PostgresData
    
    %% Internal container communication
    MCPApp -->|REST API<br/>Container DNS| TBApp
    TBApp -->|REST API<br/>Container DNS| SpiderApp
    TBApp -->|gRPC| ETCDData
    TBApp -->|SQL| PostgresData
    
    %% External cloud access
    SpiderApp -->|HTTPS<br/>Cloud APIs| CloudProviders[Cloud Service Providers]
```

## MCP Tool Categories and API Mapping

```mermaid
mindmap
  root((CB-TB MCP Tools))
    Namespace Management
      get_namespaces
      create_namespace
      delete_namespace
      API_ns_endpoints
    
    Image and Spec Discovery
      get_image_search_options
      search_images
      recommend_vm_spec
      API_resources_searchImage
      API_mciRecommendVm
    
    MCI Management
      create_mci_dynamic
      control_mci
      delete_mci
      API_ns_mciDynamic
      API_ns_control_mci
    
    Remote Operations
      execute_command_mci
      transfer_file_mci
      API_ns_cmd_mci
      API_ns_transferFile_mci
    
    Resource Management
      get_vnets
      get_security_groups
      release_resources
      API_ns_resources
    
    Direct Database Access
      PostgreSQL_MCP_Server
      SQL_Queries_Specs_Images
      Advanced_Analytics
      Cost_Optimization_Analysis
```

## Configuration Flow

```mermaid
flowchart TD
    A[Docker Compose Start] --> B[Build MCP Container]
    B --> C[Set Environment Variables]
    C --> D[Initialize TB-MCP with FastMCP]
    D --> E[Load MCP Tools & Prompts]
    E --> F[Test CB-TB Connectivity]
    F --> G[Start Streamable HTTP Server on port 8000]
    
    G --> H{Client Connection Type}
    H -->|Claude Desktop| I[Uses stdio with mcp-simple-proxy.py]
    H -->|VS Code Copilot| J[Direct Streamable HTTP to port 8000/mcp]
    H -->|MCP Inspector| K[Direct Streamable HTTP to port 8000/mcp]
    
    I --> L[mcp-simple-proxy.py bridges stdio to HTTP]
    J --> M[Native Streamable HTTP support]
    K --> M
    
    L --> N[MCP JSON-RPC over Streamable HTTP]
    M --> N
    N --> O[Tool Execution in TB-MCP]
    O --> P[REST API calls to CB-TB]
    P --> Q[CB-TB processes request]
    Q --> R[CB-Spider manages CSP interactions]
    R --> S[Cloud Provider APIs]
```
