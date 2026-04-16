# Cloud-Agnostic Image

Comprehensive guide for automated CSP-agnostic custom image creation using CB-Tumblebug

## 📑 Table of Contents

1. [Overview](#overview)
2. [Key Features](#key-features)
3. [Workflow Architecture](#workflow-architecture)
4. [API Reference](#api-reference)
5. [Usage Scenarios](#usage-scenarios)

---

## Overview

### What is Cloud-Agnostic Image?

**Cloud-Agnostic Image** is a high-level workflow in CB-Tumblebug that automates the entire lifecycle of creating custom machine images (snapshots) across multiple cloud providers. It handles infrastructure provisioning, software installation (via post-commands), snapshot creation, and resource cleanup in a single API call.

### Why Use Cloud-Agnostic Image?

**Problem:**
- Creating custom images manually involves multiple steps: Provision Node → Install Software → Verify → Stop Node → Create Image → Delete Node.
- Doing this across multiple clouds (AWS, Azure, GCP, etc.) requires different tools and procedures.
- Managing the timing (waiting for Node to be ready, waiting for image to be available) is complex and error-prone.

**Solution:**
- **One-Click Automation**: Define your infrastructure and software once, and Tumblebug handles the rest.
- **CSP Agnostic**: Works uniformly across all supported cloud providers.
- **Resource Efficiency**: Automatically cleans up expensive compute resources after the image is secured.

### Key Highlights

✅ **End-to-End Automation**: From empty state to ready-to-use custom image in one request.
✅ **Parallel Processing**: Creates snapshots for multiple Nodes (per NodeGroup) simultaneously.
✅ **Smart Cleanup**: Automatically terminates temporary Nodes only after images are confirmed "Available".
✅ **Error Handling**: Uses "Refine" policy to handle partial provisioning failures gracefully.
✅ **Status Tracking**: Monitors image creation progress and ensures availability before cleanup.

---

## Key Features

### 1. Automated Workflow

The system executes a strictly ordered sequence of operations:

1. **Provisioning**: Creates a temporary Infra (Multi-Cloud Infrastructure) based on your specifications.
2. **Configuration**: Executes post-deployment commands (e.g., `apt install nginx`) to set up the software environment.
3. **Snapshotting**: Triggers CSP-native snapshot mechanisms for each running Node.
4. **Verification**: Actively polls image status until it transitions to `Available`.
5. **Cleanup**: Terminates the temporary Infra to prevent unnecessary costs (optional but recommended).

### 2. Parallel Snapshot Creation

- Identifies the first running Node in each NodeGroup.
- Executes snapshot requests in parallel across different providers.
- Uses provider-specific semaphores to prevent API rate limiting.

### 3. Safety Mechanisms

- **Wait-for-Available**: The system does not delete the source Node until the created image is fully registered and available.
- **Partial Failure Handling**: If some Nodes fail to provision, the system proceeds with snapshotting the successful ones.
- **Cleanup Protection**: If snapshot creation fails completely, cleanup can still be enforced to avoid zombie resources.

---

## Workflow Architecture

### 1. Overall Execution Sequence

The following sequence diagram illustrates the interaction between the user, Tumblebug components, and the underlying CB-Spider layer.

```mermaid
sequenceDiagram
    actor User
    participant API as Tumblebug API
    participant Infra as Infra Manager
    participant Snap as Snapshot Manager
    participant Spider as CB-Spider
    
    User->>API: POST /buildAgnosticImage
    
    rect rgb(230, 240, 255)
        Note over API,Infra: Phase 1: Provisioning
        API->>Infra: CreateInfraDynamic (Policy="refine")
        Infra->>Spider: Create Nodes
        Spider-->>Infra: Nodes Created
        Infra-->>API: Infra Info
        
        API->>Infra: GetInfraStatus
        Infra-->>API: Running Count
    end
    
    rect rgb(255, 245, 230)
        Note over API,Snap: Phase 2: Snapshotting
        API->>Snap: CreateInfraSnapshot
        
        par Parallel per NodeGroup
            Snap->>Spider: Create Image (Node 1)
            Snap->>Spider: Create Image (Node 2)
        end
        
        Spider-->>Snap: Image IDs
        Snap-->>API: Snapshot Results
    end
    
    rect rgb(255, 255, 230)
        Note over API,Snap: Phase 3: Verification
        loop Until Available or Timeout
            API->>Snap: Check Image Status
        end
    end
    
    rect rgb(255, 230, 230)
        Note over API,Infra: Phase 4: Cleanup
        API->>Infra: DelInfra (Terminate)
    end
    
    API-->>User: Final Result
```

### 2. Smart Snapshot Strategy

Tumblebug optimizes the snapshot process by selecting representative Nodes and managing API concurrency limits per provider.

```mermaid
flowchart TD
    Start([Start Snapshot Process]) --> GetVMs[Get All Nodes in Infra]
    
    subgraph Selection [Target Selection]
        GetVMs --> GroupSG[Group by NodeGroup]
        GroupSG --> FilterRunning[Filter Running Nodes]
        FilterRunning --> SelectOne[Select First Running Node / per NodeGroup]
    end
    
    subgraph Concurrency [Provider-Aware Parallelism]
        SelectOne --> GroupProv[Group by Provider]
        
        GroupProv --> AWS[AWS Tasks]
        GroupProv --> Azure[Azure Tasks]
        GroupProv --> GCP[GCP Tasks]
        
        AWS --> SemAWS{Semaphore Limit: 3}
        Azure --> SemAzure{Semaphore Limit: 3}
        GCP --> SemGCP{Semaphore Limit: 3}
    end
    
    SemAWS --> ExecAWS[Execute Snapshot]
    SemAzure --> ExecAzure[Execute Snapshot]
    SemGCP --> ExecGCP[Execute Snapshot]
    
    ExecAWS --> Collect[Collect Results]
    ExecAzure --> Collect
    ExecGCP --> Collect
    
    Collect --> End([Return Results])
    
    style Selection fill:#e1f5fe
    style Concurrency fill:#fff3e0
    style SelectOne stroke:#d32f2f,stroke-width:2px
```

### 3. Verification and Cleanup Logic

The system ensures images are usable before destroying the source infrastructure.

```mermaid
stateDiagram-v2
    [*] --> SnapshotCreated
    
    state "Waiting for Availability" as Waiting {
        SnapshotCreated --> InitialSleep: Sleep 15s
        InitialSleep --> CheckStatus
        
        CheckStatus --> AllAvailable: Yes
        CheckStatus --> AnyPending: No
        
        AnyPending --> CheckTimeout: > 10 mins?
        CheckTimeout --> SleepLoop: No
        SleepLoop --> CheckStatus: Sleep 10s
        
        CheckTimeout --> TimeoutWarning: Yes
    }
    
    AllAvailable --> CleanupDecision
    TimeoutWarning --> CleanupDecision
    
    state "Cleanup Phase" as Cleanup {
        CleanupDecision --> TerminateInfra: Cleanup=true
        CleanupDecision --> KeepInfra: Cleanup=false
        
        TerminateInfra --> Result
        KeepInfra --> Result
    }
    
    Result --> [*]
```

### State Transitions

| Stage | Description | Typical Duration |
|-------|-------------|------------------|
| **Provisioning** | Creating Nodes and installing software | 2 - 10 mins |
| **Snapshotting** | Triggering CSP snapshot APIs | 1 - 5 mins |
| **Waiting** | Waiting for cloud provider to finalize image | 5 - 20 mins |
| **Cleanup** | Terminating resources | 1 - 3 mins |

### 4. Lifecycle: Build Once, Deploy Many

Once a custom image is created, it becomes a reusable asset within Tumblebug. You can use the generated `imageId` to spawn multiple identical Node instances, enabling rapid scaling and consistent deployments.

```mermaid
flowchart TD
    subgraph BuildPhase [Phase 1: Build]
        SourceInfra[Source Infra] -->|Snapshot| CustomImg[Custom Image / ID: nginx-custom-image-g1]
        SourceInfra -.->|Cleanup| Terminated[Terminated]
    end

    subgraph DeployPhase [Phase 2: Deploy & Scale]
        CustomImg -->|Reference by imageId| NewInfra1[Production Infra 1]
        CustomImg -->|Reference by imageId| NewInfra2[Production Infra 2]
        CustomImg -->|Reference by imageId| ScaleOut[Scale Out Existing Infra]
        
        NewInfra1 --> VM1[Node Instance 1]
        NewInfra1 --> VM2[Node Instance 2]
        NewInfra2 --> VM3[Node Instance 3]
    end
    
    style CustomImg fill:#fff9c4,stroke:#fbc02d,stroke-width:2px
    style BuildPhase fill:#f5f5f5
    style DeployPhase fill:#e3f2fd
```

**How to Reuse:**
Simply use the `imageId` returned from the build process in your standard Infra creation request:

```json
{
  "node": [
    {
      "imageId": "nginx-custom-image-g1",
      "specId": "aws-t3-small",
      "name": "prod-node-01"
    }
  ]
}
```

---

## API Reference

### Create Agnostic Image

**Endpoint:** `POST /ns/{nsId}/buildAgnosticImage`

**Request Body:**
```json
{
  "sourceInfraReq": {
    "name": "build-image-infra",
    "nodeGroups": [
      {
        "nodeGroupSize": "1",
        "name": "base-group",
        "imageId": "ubuntu-22.04",
        "specId": "aws-t3-small",
        "vmUserPassword": "mypassword"
      }
    ],
    "postCommand": {
      "command": [
        "sudo apt-get update",
        "sudo apt-get install -y nginx"
      ]
    }
  },
  "snapshotReq": {
    "name": "nginx-custom-image",
    "description": "Ubuntu 22.04 with Nginx pre-installed"
  },
  "cleanupInfraAfterSnapshot": true
}
```

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `sourceInfraReq` | object | Yes | - | Standard Infra creation request with Node specs and post-commands |
| `snapshotReq` | object | Yes | - | Configuration for the resulting images (name, description) |
| `cleanupInfraAfterSnapshot` | boolean | No | `true` | Whether to delete the Infra after successful image creation |

**Response:** `200 OK`
```json
{
  "namespace": "default",
  "infraId": "build-image-infra",
  "infraStatus": "Terminated",
  "infraCleanedUp": true,
  "totalDuration": "12m45s",
  "message": "Successfully created 1 custom images from Infra build-image-infra and cleaned up infrastructure",
  "snapshotResult": {
    "infraId": "build-image-infra",
    "successCount": 1,
    "failCount": 0,
    "results": [
      {
        "nodeGroupId": "g1",
        "nodeId": "base-group-01",
        "status": "Success",
        "imageId": "nginx-custom-image-g1",
        "imageInfo": { ... }
      }
    ]
  }
}
```

---

## Usage Scenarios

### 1. Golden Image Pipeline
Create a standardized "Golden Image" with security patches and compliance tools pre-installed.
- **Input**: Base OS image (e.g., Ubuntu 22.04)
- **Post-Command**: Security hardening scripts, monitoring agent installation
- **Output**: Hardened custom image ready for production deployment

### 2. Application Pre-baking
Pre-install complex application stacks to reduce boot time for scaling groups.
- **Input**: Base OS
- **Post-Command**: `docker install`, `git clone app`, `npm install`
- **Output**: Application-ready image that starts serving traffic immediately upon boot

### 3. Cross-Cloud Replication
(Requires running the workflow for each CSP)
- Define one `BuildAgnosticImage` request structure.
- Change only the `specId` and `imageId` for the target cloud (AWS, Azure, GCP).
- Execute to get functionally identical images across different clouds.

---

## Testing based on GUI

### request
<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/e42694a2-950f-4b8d-bc0d-168f7a4bfe21" />

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/b2d84160-a38f-49c7-94b3-40172437d4c4" />

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/93f967fb-d183-46a4-b758-57c6e321b4b4" />

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/a1f5e718-2297-4190-a92c-aba2bc47ac83" />

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/1837d75d-54f7-44d4-94c4-a93c1e2163c0" />

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/a9a71477-f17f-4579-b2ef-6bc3f5ef8456" />

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/a74869d3-43df-4c55-a189-437879fb7544" />



### result

<img width="1169" height="762" alt="image" src="https://github.com/user-attachments/assets/e9f32f56-9297-4715-8ebd-be7e9a14e1b4" />

<img width="1259" height="346" alt="image" src="https://github.com/user-attachments/assets/2bf4be70-ef14-4ac2-9b16-6c2579e8c689" />

<img width="1170" height="762" alt="image" src="https://github.com/user-attachments/assets/17ffc077-0eac-45d1-ab91-75abe9e7409d" />

<img width="1241" height="762" alt="image" src="https://github.com/user-attachments/assets/2c9bc80b-bba5-47f7-85b4-e2d0693b36f1" />

<img width="1126" height="762" alt="image" src="https://github.com/user-attachments/assets/72059e89-9b56-43c2-aab0-f1a27add88de" />

<img width="1129" height="762" alt="image" src="https://github.com/user-attachments/assets/4c6acb3d-7213-447b-bf28-148f851800d4" />

<img width="637" height="762" alt="image" src="https://github.com/user-attachments/assets/786e7d36-e0e9-4311-bd5a-f25b565c720c" />

<img width="663" height="762" alt="image" src="https://github.com/user-attachments/assets/a0b73846-df0e-42d4-b013-6a94f08bf070" />

<img width="931" height="762" alt="image" src="https://github.com/user-attachments/assets/beb1396c-0ea8-4ac8-8794-9103537d65cc" />

<img width="931" height="762" alt="image" src="https://github.com/user-attachments/assets/b546138d-5458-4e35-8158-cbc37de8d15e" />

<img width="851" height="762" alt="image" src="https://github.com/user-attachments/assets/9e992eea-9f8f-4db8-9493-b149c86a34c1" />

<img width="850" height="762" alt="image" src="https://github.com/user-attachments/assets/a52cac17-3c9d-47af-b06c-aa2d87b122b4" />
