# CB-Tumblebug High-Scale Node Provisioning Architecture

Visually analyzing the advanced architecture and optimization techniques for high-scale Node provisioning in CB-Tumblebug.

## 🏗️ Overall Architecture

This diagram illustrates the high-level architecture of CB-Tumblebug's provisioning system. It separates the core logic from the optimization layer, ensuring that high-scale requests are handled efficiently before reaching the Cloud Service Providers (CSPs) via CB-Spider. Key components include the **Infra Controller**, **Provisioning Engine**, and a dedicated **Optimization Layer** for rate limiting and concurrency control.

```mermaid
graph TB
    subgraph "Client Layer"
        API[REST API Request]
        MCP[MCP Tool]
        WEB[Web Dashboard]
    end
    
    subgraph "CB-Tumblebug Core"
        ROUTER[API Router]
        CTRL[Infra Controller]
        PROV[Provisioning Engine]
        CACHE[Cache Layer]
        HIST[History Manager]
    end
    
    subgraph "Optimization Layer"
        RATE[Rate Limiter]
        PARALLEL[Parallel Processor]
        MUTEX[Concurrency Control]
        MONITOR[Status Monitor]
    end
    
    subgraph "CB-Spider Layer"
        SPIDER[CB-Spider]
        CSP1[AWS APIs]
        CSP2[Azure APIs]
        CSP3[GCP APIs]
        CSP4[Alibaba APIs]
    end
    
    API --> ROUTER
    MCP --> ROUTER
    WEB --> ROUTER
    
    ROUTER --> CTRL
    CTRL --> PROV
    PROV --> RATE
    RATE --> PARALLEL
    PARALLEL --> MUTEX
    MUTEX --> SPIDER
    
    PROV --> CACHE
    PROV --> HIST
    MONITOR --> CACHE
    
    SPIDER --> CSP1
    SPIDER --> CSP2
    SPIDER --> CSP3
    SPIDER --> CSP4
    
    style API fill:#e1f5fe
    style RATE fill:#fff3e0
    style PARALLEL fill:#f3e5f5
    style MUTEX fill:#e8f5e8
    style HIST fill:#fce4ec
```

## 🚀 Hierarchical Rate Limiting System

To prevent API throttling from CSPs, we implement a **3-level rate limiting system**. This ensures that while we maximize parallelism across different CSPs, we carefully control the request rate within specific regions and for individual Nodes, adhering to provider-specific limits (e.g., stricter limits for NCP compared to AWS).

```mermaid
graph TD
    subgraph "Level 1: CSP Parallel Processing"
        CSP_AWS[AWS Nodes<br/>Unlimited Parallel]
        CSP_AZURE[Azure Nodes<br/>Unlimited Parallel]
        CSP_GCP[GCP Nodes<br/>Unlimited Parallel]
        CSP_NCP[NCP Nodes<br/>Unlimited Parallel]
    end
    
    subgraph "Level 2: Region Rate Limiting per CSP"
        subgraph "AWS Regions"
            AWS_R1[us-east-1<br/>Max 30 Regions]
            AWS_R2[us-west-2<br/>Semaphore Control]
            AWS_R3[eu-west-1<br/>...]
        end
        
        subgraph "NCP Regions (Stricter)"
            NCP_R1[kr-central-1<br/>Max 5 Regions]
            NCP_R2[kr-central-2<br/>Stricter Limits]
        end
    end
    
    subgraph "Level 3: Node Rate Limiting per Region"
        subgraph "AWS Region Nodes"
            AWS_VM1[Node-1<br/>Max 20 Nodes/Region]
            AWS_VM2[Node-2<br/>Concurrent Control]
            AWS_VM3[Node-N<br/>...]
        end
        
        subgraph "NCP Region Nodes (Conservative)"
            NCP_VM1[Node-1<br/>Max 15 Nodes/Region]
            NCP_VM2[Node-2<br/>Conservative Limits]
            NCP_VM3[Node-N<br/>...]
        end
    end
    
    CSP_AWS --> AWS_R1
    CSP_AWS --> AWS_R2
    CSP_AWS --> AWS_R3
    
    CSP_NCP --> NCP_R1
    CSP_NCP --> NCP_R2
    
    AWS_R1 --> AWS_VM1
    AWS_R1 --> AWS_VM2
    AWS_R1 --> AWS_VM3
    
    NCP_R1 --> NCP_VM1
    NCP_R1 --> NCP_VM2
    NCP_R1 --> NCP_VM3
    
    style CSP_AWS fill:#ff9800
    style CSP_NCP fill:#f44336
    style AWS_R1 fill:#ffeb3b
    style NCP_R1 fill:#e91e63
    style AWS_VM1 fill:#4caf50
    style NCP_VM1 fill:#9c27b0
```

## ⚡ Advanced Parallel Processing Flow

This flow demonstrates how a massive Infra creation request (e.g., 1000+ Nodes) is broken down. Requests are grouped by CSP and Region, allowing for **unlimited parallel processing** at the CSP level, while enforcing **semaphores** at the Region and Node levels to maintain stability and prevent resource exhaustion.

```mermaid
flowchart TD
    START[Infra Creation Request<br/>1000+ Nodes] --> GROUP[Node Grouping by CSP & Region]
    
    GROUP --> CSP_GROUP{CSP Grouping}
    
    CSP_GROUP --> AWS_FLOW[AWS Processing<br/>300 Nodes]
    CSP_GROUP --> AZURE_FLOW[Azure Processing<br/>250 Nodes]
    CSP_GROUP --> GCP_FLOW[GCP Processing<br/>200 Nodes]
    CSP_GROUP --> NCP_FLOW[NCP Processing<br/>250 Nodes]
    
    subgraph "AWS Parallel Processing"
        AWS_FLOW --> AWS_SEM[Region Semaphore<br/>Max 10 Regions]
        AWS_SEM --> AWS_R1[us-east-1<br/>120 Nodes]
        AWS_SEM --> AWS_R2[us-west-2<br/>100 Nodes]
        AWS_SEM --> AWS_R3[eu-west-1<br/>80 Nodes]
        
        AWS_R1 --> AWS_VM_SEM1[Node Semaphore<br/>Max 30 Nodes]
        AWS_R2 --> AWS_VM_SEM2[Node Semaphore<br/>Max 30 Nodes]
        AWS_R3 --> AWS_VM_SEM3[Node Semaphore<br/>Max 30 Nodes]
    end
    
    subgraph "NCP Conservative Processing"
        NCP_FLOW --> NCP_SEM[Region Semaphore<br/>Max 5 Regions]
        NCP_SEM --> NCP_R1[kr-central-1<br/>150 Nodes]
        NCP_SEM --> NCP_R2[kr-central-2<br/>100 Nodes]
        
        NCP_R1 --> NCP_VM_SEM1[Node Semaphore<br/>Max 15 Nodes]
        NCP_R2 --> NCP_VM_SEM2[Node Semaphore<br/>Max 15 Nodes]
    end
    
    AWS_VM_SEM1 --> AWS_RESULT[AWS Results]
    NCP_VM_SEM1 --> NCP_RESULT[NCP Results]
    
    AWS_RESULT --> COLLECT[Result Collection<br/>Thread-Safe Channels]
    NCP_RESULT --> COLLECT
    
    COLLECT --> STATUS_AGG[Status Aggregation<br/>Mutex Protected]
    STATUS_AGG --> FINAL[Final Infra Status<br/>Success/Partial/Failed]
    
    style START fill:#e3f2fd
    style AWS_FLOW fill:#ff9800
    style NCP_FLOW fill:#f44336
    style COLLECT fill:#4caf50
    style FINAL fill:#9c27b0
```

## 🎯 Intelligent Status Management

Status tracking is handled by the **Node Status Agent** (`NodeStatusAgent`), a background daemon that continuously polls CSP Node statuses using a priority-aware scheduler and per-CSP rate limiters — completely decoupled from the API request path.

```mermaid
flowchart LR
    subgraph "Background: NodeStatusAgent"
        TICK[1s tick] --> DISPATCH[Dispatch eligible\nentries from StatusStore]
        DISPATCH --> RATE[Per-CSP rate limiter]
        RATE --> SPIDER[Spider /vmstatus]
        SPIDER --> STORE[StatusStore update]
    end

    subgraph "API request path (no CSP call)"
        REQ[GET /infra status] --> STORE
        STORE --> RESP[Fresh cached response]
    end

    style TICK fill:#e3f2fd
    style STORE fill:#fff3e0
    style RESP fill:#4caf50
```

**Poll priorities** drive how often each Node is re-checked:

| Priority | Interval | Assigned when |
|---|---|---|
| `PollUrgent` | ~5 s | Creating, Terminating, Rebooting, … |
| `PollHigh` | ~15 s | Running with pending TargetAction |
| `PollNormal` | ~5 min | Stable Running / Undefined |
| `PollRecover` | ~10 min | Suspended |
| `PollSkip` | never | Terminated, Failed (final states) |

The system also **intelligently skips CSP API calls** for Nodes in final states:

```mermaid
stateDiagram-v2
    [*] --> Creating: Infra Request
    Creating --> VMObjects: Create Node Objects
    VMObjects --> ResourcePrep: Prepare Resources
    ResourcePrep --> Provisioning: Start Provisioning
    
    Provisioning --> ParallelProcess: Rate-Limited Parallel
    ParallelProcess --> CSPCalls: CB-Spider Calls
    CSPCalls --> StatusCheck: Fetch Node Status
    StatusCheck --> StableCheck: Check Stable States
    
    StableCheck --> SkipCSP: Skip CSP Calls
    StableCheck --> CSPCall: Make CSP Call
    
    SkipCSP --> CacheReturn: Use Cached Status
    CSPCall --> UpdateCache: Update Cache
    UpdateCache --> CacheReturn
    
    CacheReturn --> Complete: All Nodes Processed
    Complete --> [*]
    
    note right of SkipCSP : Terminated, Failed,<br/>Suspended states<br/>skip CSP calls
    
    note right of ParallelProcess : CSP-aware rate<br/>limiting prevents<br/>API throttling
```

> For the full NodeStatusAgent design (StatusStore, operation lock, startup scan, orphan rescue integration), see [Infra Resource Model and Lifecycle Management](./infra-resource-model-and-lifecycle-management.md#node-status-agent).

## 🔄 Advanced Caching & Memory Optimization

We utilize a **multi-layered caching strategy** for connection configurations and Node statuses. Combined with Go's **channel-based concurrency** and minimal mutex usage, this approach minimizes memory footprint and eliminates redundant network operations, ensuring high performance.

```mermaid
flowchart LR
    subgraph "Memory Management"
        CHAN[Channel-based<br/>Result Collection]
        SEM[Semaphore Pool<br/>Concurrency Control]
        MUTEX[Mutex Minimal<br/>Critical Sections Only]
    end
    
    subgraph "Caching Strategy"
        STATUS_CACHE[Node Status Cache<br/>Stable States Only]
        CONN_CACHE[Connection Config<br/>Cache]
        SPEC_CACHE[Spec Info<br/>Cache]
    end
    
    subgraph "Smart Skipping"
        STABLE_CHECK{Status Stable?}
        CSP_SKIP[Skip CSP Call]
        CSP_CALL[Make CSP Call]
        CACHE_UPDATE[Update Cache]
    end
    
    CHAN --> STATUS_CACHE
    SEM --> STABLE_CHECK
    
    STABLE_CHECK -->|Terminated/Failed/Suspended| CSP_SKIP
    STABLE_CHECK -->|Creating/Running| CSP_CALL
    
    CSP_SKIP --> STATUS_CACHE
    CSP_CALL --> CACHE_UPDATE
    CACHE_UPDATE --> STATUS_CACHE
    
    STATUS_CACHE --> FAST_RESPONSE[Fast Response<br/>No Network Delay]
    
    style CHAN fill:#e8f5e8
    style STATUS_CACHE fill:#fff3e0
    style CSP_SKIP fill:#4caf50
    style FAST_RESPONSE fill:#2196f3
```

## 📈 Provisioning History & Risk Analysis

The system learns from past deployments. By analyzing historical success and failure rates for specific Spec and Image combinations, the **Risk Analysis Engine** can predict potential failures and block or warn users about high-risk configurations before deployment begins, improving overall reliability.

```mermaid
flowchart TD
    subgraph "Event Recording"
        VM_CREATE[Node Creation Attempt]
        SUCCESS[Success Event]
        FAILURE[Failure Event]
        
        VM_CREATE --> SUCCESS
        VM_CREATE --> FAILURE
    end
    
    subgraph "History Storage"
        SUCCESS --> HIST_DB[(Provisioning History<br/>KV Store)]
        FAILURE --> HIST_DB
        
        HIST_DB --> SPEC_LOG[Spec-based Logs]
        HIST_DB --> IMAGE_LOG[Image-based Logs]
        HIST_DB --> COMBO_LOG[Combination Logs]
    end
    
    subgraph "Risk Analysis Engine"
        SPEC_LOG --> SPEC_RISK{Spec Risk<br/>Analysis}
        IMAGE_LOG --> IMAGE_RISK{Image Risk<br/>Analysis}
        COMBO_LOG --> COMBO_RISK{Combination Risk<br/>Analysis}
        
        SPEC_RISK --> HIGH_SPEC[High: 10+ image failures]
        SPEC_RISK --> MED_SPEC[Medium: 5+ image failures]
        SPEC_RISK --> LOW_SPEC[Low: Few failures]
        
        IMAGE_RISK --> HIGH_IMAGE[High: Previously failed<br/>with this spec]
        IMAGE_RISK --> MED_IMAGE[Medium: Mixed results]
        IMAGE_RISK --> LOW_IMAGE[Low: Previously succeeded]
        
        HIGH_SPEC --> BLOCK[Block Deployment]
        HIGH_IMAGE --> WARN[Warning + Monitoring]
        LOW_SPEC --> PROCEED[Safe to Proceed]
        LOW_IMAGE --> PROCEED
    end
    
    subgraph "Intelligent Decision"
        BLOCK --> ALTERNATIVE[Suggest Alternative<br/>Spec/Image]
        WARN --> MONITOR[Enhanced Monitoring]
        PROCEED --> NORMAL[Normal Deployment]
    end
    
    style VM_CREATE fill:#e3f2fd
    style FAILURE fill:#f44336
    style SUCCESS fill:#4caf50
    style HIGH_SPEC fill:#ff5722
    style HIGH_IMAGE fill:#e91e63
    style BLOCK fill:#d32f2f
```

## 🛡️ Failure Handling & Recovery Strategies

When failures occur, the system offers flexible recovery options. **'Continue'** ignores failures and proceeds, **'Rollback'** cleans up everything upon failure, and **'Refine'** allows users to keep successful Nodes and only clean up the failed ones for a retry, minimizing downtime.

```mermaid
flowchart TD
    Infra_START[Infra Creation Start] --> POLICY{Failure Policy}
    
    POLICY -->|continue| CONTINUE_FLOW[Continue Flow]
    POLICY -->|rollback| ROLLBACK_FLOW[Rollback Flow]
    POLICY -->|refine| REFINE_FLOW[Refine Flow]
    
    subgraph "Continue Strategy"
        CONTINUE_FLOW --> VM_PARALLEL[Parallel Node Creation]
        VM_PARALLEL --> SOME_FAIL{Some Nodes Failed?}
        SOME_FAIL -->|Yes| PARTIAL_Infra[Create Partial Infra]
        SOME_FAIL -->|No| FULL_Infra[Create Full Infra]
        PARTIAL_Infra --> MARK_FAILED[Mark Failed Nodes<br/>as StatusFailed]
        FULL_Infra --> SUCCESS_COMPLETE[Complete Success]
    end
    
    subgraph "Rollback Strategy"
        ROLLBACK_FLOW --> VM_CREATE_RB[Node Creation]
        VM_CREATE_RB --> ANY_FAIL{Any Node Failed?}
        ANY_FAIL -->|Yes| CLEANUP_ALL[Delete All Resources]
        ANY_FAIL -->|No| SUCCESS_RB[Complete Success]
        CLEANUP_ALL --> ROLLBACK_COMPLETE[Rollback Complete<br/>Infra Deleted]
    end
    
    subgraph "Refine Strategy"
        REFINE_FLOW --> VM_CREATE_RF[Node Creation]
        VM_CREATE_RF --> AUTO_CLEANUP[Auto Cleanup Failed Nodes]
        AUTO_CLEANUP --> CLEAN_Infra[Clean Infra<br/>Only Successful Nodes]
        CLEAN_Infra --> REFINE_COMPLETE[Refine Complete]
    end
    
    subgraph "Error Tracking"
        MARK_FAILED --> ERROR_LOG[Error Logging]
        CLEANUP_ALL --> ERROR_LOG
        AUTO_CLEANUP --> ERROR_LOG
        ERROR_LOG --> HIST_UPDATE[Update History]
        HIST_UPDATE --> RISK_UPDATE[Update Risk Analysis]
    end
    
    style CONTINUE_FLOW fill:#4caf50
    style ROLLBACK_FLOW fill:#f44336
    style REFINE_FLOW fill:#ff9800
    style PARTIAL_Infra fill:#ffeb3b
    style CLEANUP_ALL fill:#e91e63
    style AUTO_CLEANUP fill:#2196f3
```

## 🌐 Network & Connection Optimization

Network overhead is minimized by **caching connection configurations**. Instead of validating credentials and endpoints for every single Node request, the system reuses validated connection info, speeding up the initialization phase of massive deployments.

```mermaid
sequenceDiagram
    participant Client
    participant TB as CB-Tumblebug
    participant Cache
    participant Spider as CB-Spider
    participant CSP1 as AWS
    participant CSP2 as Azure
    participant CSP3 as GCP
    
    Client->>TB: Create Infra (1000 Nodes)
    TB->>TB: Group by CSP & Region
    
    par AWS Processing
        TB->>Cache: Check Connection Config
        Cache-->>TB: Cached Config
        TB->>Spider: Create 300 AWS Nodes
        Note over TB,Spider: Rate Limited:<br/>10 regions, 30 Nodes/region
        Spider->>CSP1: Parallel API Calls
        CSP1-->>Spider: Node Creation Results
    and Azure Processing
        TB->>Cache: Check Connection Config
        TB->>Spider: Create 250 Azure Nodes
        Note over TB,Spider: Rate Limited:<br/>8 regions, 25 Nodes/region
        Spider->>CSP2: Parallel API Calls
        CSP2-->>Spider: Node Creation Results
    and GCP Processing
        TB->>Cache: Check Connection Config
        TB->>Spider: Create 200 GCP Nodes
        Note over TB,Spider: Rate Limited:<br/>12 regions, 35 Nodes/region
        Spider->>CSP3: Parallel API Calls
        CSP3-->>Spider: Node Creation Results
    end
    
    Spider-->>TB: All Results
    TB->>TB: Status Aggregation<br/>(Thread-Safe)
    TB->>Cache: Cache Stable Statuses
    TB-->>Client: Infra Creation Complete
    
    Note over TB: Random delays prevent<br/>CSP API throttling
    Note over Cache: Stable states cached<br/>to avoid redundant calls
```

## 🔧 Resource Management & Cleanup

This flow ensures no resources are orphaned. The system **tracks all dynamically created resources** (VNets, Security Groups, SSH Keys). In case of failure or termination, cleanup is performed in **parallel** to speed up the teardown process.

```mermaid
flowchart TD
    subgraph "Resource Creation"
        DYNAMIC[Dynamic Infra Request]
        VALIDATE[Resource Validation]
        CREATE_RES[Create Missing Resources]
        
        DYNAMIC --> VALIDATE
        VALIDATE --> CREATE_RES
    end
    
    subgraph "Resource Tracking"
        CREATE_RES --> TRACK[Track Created Resources]
        TRACK --> VNET_TRACK[VNet Tracking]
        TRACK --> SSH_TRACK[SSH Key Tracking]
        TRACK --> SG_TRACK[Security Group Tracking]
    end
    
    subgraph "Failure Scenarios"
        VM_FAIL[Node Creation Failure]
        POLICY_CHECK{Cleanup Policy}
        
        VM_FAIL --> POLICY_CHECK
        POLICY_CHECK -->|Rollback| PARALLEL_CLEANUP[Parallel Resource Cleanup]
        POLICY_CHECK -->|Continue| KEEP_RESOURCES[Keep Resources]
        POLICY_CHECK -->|Refine| SELECTIVE_CLEANUP[Selective Cleanup]
    end
    
    subgraph "Parallel Cleanup Process"
        PARALLEL_CLEANUP --> CLEANUP_ORDER[Cleanup Order:<br/>SSH → SG → VNet]
        CLEANUP_ORDER --> SSH_DEL[Delete SSH Keys<br/>Parallel, Max 10]
        CLEANUP_ORDER --> SG_DEL[Delete Security Groups<br/>Parallel, Max 10]
        CLEANUP_ORDER --> VNET_DEL[Delete VNets<br/>Parallel, Max 10]
        
        SSH_DEL --> WAIT1[Wait 5 seconds]
        SG_DEL --> WAIT1
        WAIT1 --> VNET_DEL
    end
    
    VNET_TRACK --> VM_FAIL
    SSH_TRACK --> VM_FAIL
    SG_TRACK --> VM_FAIL
    
    VNET_DEL --> CLEANUP_COMPLETE[Cleanup Complete]
    KEEP_RESOURCES --> RESOURCES_KEPT[Resources Preserved<br/>for Future Use]
    SELECTIVE_CLEANUP --> PARTIAL_CLEANUP[Cleanup Failed Node<br/>Resources Only]
    
    style DYNAMIC fill:#e3f2fd
    style PARALLEL_CLEANUP fill:#f44336
    style CLEANUP_ORDER fill:#ff9800
    style CLEANUP_COMPLETE fill:#4caf50
```


## 📊 Performance Test Results

We have validated the architecture with large-scale provisioning tests. The following metrics demonstrate the system's capability to handle massive multi-cloud deployments.

| Metric | Value | Note |
| :--- | :---: | :--- |
| **Total Nodes** | **1,110** | 🚀 **Massive Scale** |
| **Regions Used** | **53** | 🌍 **Global Distribution** |
| CSPs Used | 8 | Multi-Cloud Coverage |
| Infras Running | 4 | Concurrent Operations |

> The successful provisioning of **1,110 Nodes** across **53 regions** validates the stability of the hierarchical rate limiting and parallel processing mechanisms.

<img width="1751" height="924" alt="image" src="https://github.com/user-attachments/assets/64d48c1c-0f3e-4d0c-8b12-63380d0e6df7" />

<br>

<img width="1781" height="915" alt="image" src="https://github.com/user-attachments/assets/69bbca53-aef5-4cad-8be4-d26488bcd86b" />


## 🎯 Key Optimization Benefits

### Performance Improvements
- **3-Level Rate Limiting**: Prevent API throttling with hierarchical control (CSP → Region → Node).
- **Node Status Agent**: Background daemon decouples CSP polling from API requests; priority-aware scheduler (PollUrgent → PollSkip) cuts unnecessary calls by 30–50%.
- **Parallel Processing**: Optimal performance with unlimited parallelization per CSP and limited parallelization per Region/Node.

### Reliability Enhancements
- **Failure History Analysis**: Risk prediction and blocking based on historical failure data.
- **Intelligent Recovery**: Flexible failure handling with Continue/Rollback/Refine policies.
- **Resource Tracking**: Complete rollback support by tracking dynamically created resources.

### Scalability Features
- **CSP-Aware Rate Limits**: Differentiated limits (e.g., NCP: 5 regions, 15 Nodes vs AWS: 30 regions, 20 Nodes).
- **Memory Optimization**: Memory efficiency with Channel-based result collection and minimal mutex usage.
- **Connection Pooling**: Minimize network overhead with connection config caching.

Through these optimization techniques, we have implemented an enterprise-grade multi-cloud infrastructure provisioning system capable of **stably and efficiently managing Infras with thousands of Nodes**.