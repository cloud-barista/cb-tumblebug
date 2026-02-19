# Remote Command Execution and File Transfer

Comprehensive guide for executing remote commands and transferring files to MCI VMs via secure SSH with TOFU (Trust On First Use) verification.

## 📑 Table of Contents

1. [Overview](#overview)
2. [Key Concepts](#key-concepts)
3. [Architecture](#architecture)
4. [Real-Time Streaming (SSE)](#real-time-streaming-sse)
5. [SSH Host Key Verification (TOFU)](#ssh-host-key-verification-tofu)
6. [API Reference](#api-reference)
7. [Usage Examples](#usage-examples)

---

## Overview

### What is Remote Command Execution?

**Remote Command Execution** is a core feature of CB-Tumblebug that allows users to execute shell commands on VMs within an MCI (Multi-Cloud Infrastructure). Commands are sent via SSH through a **Bastion Host** for security, supporting parallel execution across multiple VMs.

### What is File Transfer?

**File Transfer** enables uploading files from the Tumblebug server to target VMs via SCP (Secure Copy Protocol). Like remote commands, file transfers also use the Bastion Host architecture.

### Why Use These Features?

**Problem:**
- Manually SSH-ing into each VM across multiple clouds is tedious and error-prone.
- Direct SSH access to VMs in private subnets is not possible without a jump host.
- Trusting SSH host keys manually for dozens of VMs is impractical.

**Solution:**
- **Unified API**: Execute commands on multiple VMs with a single API call.
- **Bastion Architecture**: Secure access to private VMs via a designated jump host.
- **TOFU Security**: Automatic SSH host key verification to prevent MITM attacks.
- **Parallel Execution**: Commands run concurrently across all target VMs.

---

## Key Concepts

### Gradual Target Selection

Tumblebug supports **multi-level target selection** for precise command execution:

```mermaid
graph TB
    subgraph "Selection Levels"
        L1[Level 1: Entire MCI]
        L2[Level 2: SubGroup]
        L3[Level 3: Specific VM]
        L4[Level 4: Label Selector]
    end
    
    L1 --> |"All VMs"| ALL[g1-1, g1-2, g2-1, g2-2, g3-1]
    L2 --> |"subGroupId=g1"| SG[g1-1, g1-2]
    L3 --> |"vmId=g1-1"| VM[g1-1]
    L4 --> |"role=worker"| LBL[g1-2, g2-1, g3-1]
    
    style L1 fill:#e1f5ff
    style L2 fill:#fff4e1
    style L3 fill:#ffe1f5
    style L4 fill:#e1ffe1
```

| Level | Parameter | Example | Target VMs |
|-------|-----------|---------|------------|
| **MCI** | (none) | `/cmd/mci/mci01` | All VMs in MCI |
| **SubGroup** | `subGroupId` | `?subGroupId=g1` | All VMs in SubGroup g1 |
| **VM** | `vmId` | `?vmId=g1-1` | Only VM g1-1 |
| **Label** | `labelSelector` | `?labelSelector=role=worker` | VMs with matching label |

**Label Selector Examples:**
- `role=worker` - VMs with role=worker label
- `env=prod,tier=backend` - VMs matching both labels
- `sys.id=g1-1` - System label matching (VM ID)

### Bastion Host (Jump Host)

A **Bastion Host** is a VM with a public IP that acts as a gateway for SSH connections to other VMs in the MCI. All remote commands and file transfers are routed through the Bastion.

```
┌─────────────────────────────────────────────────────────────────┐
│                           MCI                                   │
│  ┌─────────────────┐                                            │
│  │  Bastion Host   │◄─────── Public IP (SSH Entry Point)        │
│  │  (Jump Host)    │                                            │
│  └────────┬────────┘                                            │
│           │ SSH Tunnel (via Private Network)                    │
│           ▼                                                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   Target VM 1   │  │   Target VM 2   │  │   Target VM 3   │  │
│  │  (Private IP)   │  │  (Private IP)   │  │  (Private IP)   │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Key Points:**
- Each VM in an MCI is assigned a Bastion Host automatically.
- The Bastion can be any VM in the MCI with a public IP.
- SSH connections: `User → Bastion (Public IP) → Target VM (Private IP)`

### Command Execution Flow

1. User sends command request to Tumblebug API
2. Tumblebug resolves Bastion Host for target VM
3. SSH connection established: Tumblebug → Bastion → Target VM
4. Command executed on Target VM
5. Output (stdout/stderr) returned to user

---

## Architecture

### Remote Command Execution Flow

```mermaid
sequenceDiagram
    actor User
    participant API as Tumblebug API
    participant Bastion as Bastion Host
    participant Target as Target VM
    participant KV as KV Store
    
    User->>API: POST /cmd/mci/{mciId}
    
    Note over API,KV: Resolve Bastion & Target Info
    API->>KV: Get VM Info
    KV-->>API: VM Details (IP, SSH Key)
    API->>KV: Get Bastion Assignment
    KV-->>API: Bastion VM Info
    
    Note over API,Target: SSH Connection via Bastion
    API->>Bastion: SSH Connect (TOFU Verify)
    Bastion-->>API: Host Key Verified
    
    API->>Bastion: Tunnel to Target
    Bastion->>Target: SSH Connect (TOFU Verify)
    Target-->>Bastion: Host Key Verified
    
    Note over API,Target: Command Execution
    API->>Target: Execute Command
    Target-->>API: stdout/stderr
    
    API-->>User: Command Result
```

### File Transfer Flow

```mermaid
sequenceDiagram
    actor User
    participant API as Tumblebug API
    participant Bastion as Bastion Host
    participant Target as Target VM
    
    User->>API: POST /file/mci/{mciId}
    Note right of User: multipart/form-data<br/>(file + targetPath)
    
    API->>Bastion: SSH Connect (TOFU)
    Bastion-->>API: Connected
    
    API->>Target: SCP via Bastion
    Note over API,Target: File data streamed<br/>through SSH tunnel
    
    Target-->>API: Transfer Complete
    API-->>User: Success Response
```

### Parallel Execution Architecture

When targeting multiple VMs, commands execute in parallel:

```mermaid
graph TB
    API[Tumblebug API]
    
    subgraph "Parallel Goroutines"
        G1[Goroutine 1]
        G2[Goroutine 2]
        G3[Goroutine 3]
    end
    
    subgraph "MCI VMs"
        VM1[VM-1]
        VM2[VM-2]
        VM3[VM-3]
    end
    
    API --> G1
    API --> G2
    API --> G3
    
    G1 -->|SSH via Bastion| VM1
    G2 -->|SSH via Bastion| VM2
    G3 -->|SSH via Bastion| VM3
    
    G1 --> Result1[Result 1]
    G2 --> Result2[Result 2]
    G3 --> Result3[Result 3]
    
    Result1 --> Aggregate[Aggregated Response]
    Result2 --> Aggregate
    Result3 --> Aggregate
```

---

## Real-Time Streaming (SSE)

### Overview

By default, the remote command API operates in **synchronous mode** — the HTTP response is returned only after all VMs finish execution. For long-running commands across many VMs, this means the client must wait with no visibility into progress.

**Async mode with SSE (Server-Sent Events)** solves this by:
1. Returning an `HTTP 202 Accepted` immediately with an `xRequestId`
2. Executing commands in the background
3. Streaming real-time events (status changes, stdout/stderr lines, completion) via an SSE endpoint

### Sync vs Async Mode

| Feature | Sync Mode (default) | Async Mode (`?async=true`) |
|---------|--------------------|--------------------------|
| Response | Waits for all VMs to finish | Returns `202 Accepted` immediately |
| Output | Full result in response body | Real-time events via SSE stream |
| Progress visibility | None until completion | Live status updates per VM |
| Use case | Short commands, scripted workflows | Long-running commands, interactive UIs |

### Architecture

```mermaid
sequenceDiagram
    actor Client
    participant API as Tumblebug API
    participant Broker as CommandLogBroker
    participant VMs as Target VMs
    
    Client->>API: POST /cmd/mci/{mciId}?async=true
    Note right of Client: x-request-id: cmd-mci01-1234
    API-->>Client: 202 Accepted {xRequestId}
    
    Note over API,VMs: Background Execution
    API->>Broker: Create session (xRequestId)
    API->>VMs: SSH Connect & Execute (parallel)
    
    Client->>API: GET /stream/cmd/mci/{mciId}?xRequestId=cmd-mci01-1234
    API->>Broker: Subscribe(xRequestId)
    Broker-->>API: Replay buffered events
    API-->>Client: SSE: CommandStatus (Queued)
    
    VMs-->>API: stdout line
    API->>Broker: Publish(CommandLog)
    Broker-->>API: Forward to subscriber
    API-->>Client: SSE: CommandLog {line}
    
    VMs-->>API: Command complete
    API->>Broker: Publish(CommandStatus: Completed)
    API-->>Client: SSE: CommandStatus (Completed)
    
    Note over API,VMs: All VMs finished
    API->>Broker: Publish(CommandDone)
    API-->>Client: SSE: CommandDone {summary}
    Note over Client: Stream ends
```

### SSE Event Types

The SSE stream delivers three types of events, each as a JSON object in `data:` frames:

#### 1. `CommandStatus`

Sent when a VM's command execution status changes (e.g., Queued → Handling → Completed).

```json
{
  "type": "CommandStatus",
  "vmId": "g1-1",
  "commandIndex": 1,
  "timestamp": "2024-01-15T10:30:01Z",
  "status": {
    "mciId": "mci01",
    "vmId": "g1-1",
    "status": "Handling",
    "command": "echo hello && hostname"
  }
}
```

**Status Transitions:**
```mermaid
stateDiagram-v2
    [*] --> Queued: Command registered
    Queued --> Handling: SSH session started
    Handling --> Completed: All commands succeeded
    Handling --> Failed: Command error
    Handling --> Timeout: Context deadline exceeded
    Handling --> Cancelled: User cancelled
    Handling --> Interrupted: Execution interrupted
```

#### 2. `CommandLog`

Sent for each line of stdout/stderr output from an SSH session, in real-time.

```json
{
  "type": "CommandLog",
  "vmId": "g1-1",
  "commandIndex": 1,
  "timestamp": "2024-01-15T10:30:02Z",
  "log": {
    "stream": "stdout",
    "line": "Hello World",
    "lineNumber": 1
  }
}
```

| Field | Description |
|-------|-------------|
| `log.stream` | `"stdout"` or `"stderr"` |
| `log.line` | Output line content |
| `log.lineNumber` | Sequential line number per stream per VM |

#### 3. `CommandDone`

The **terminal event** — sent once when all VMs have finished. The SSE stream closes after this event.

```json
{
  "type": "CommandDone",
  "timestamp": "2024-01-15T10:30:45Z",
  "summary": {
    "totalVms": 5,
    "completedVms": 4,
    "failedVms": 1,
    "elapsedSeconds": 44
  }
}
```

If the command failed before reaching any VMs (e.g., preprocessing error), the `error` field is included:

```json
{
  "type": "CommandDone",
  "timestamp": "2024-01-15T10:30:01Z",
  "summary": {
    "totalVms": 0,
    "completedVms": 0,
    "failedVms": 0,
    "elapsedSeconds": 0,
    "error": "built-in function GetPublicIP error: no VM found (ID: /ns/default/mci/mci01/vm/g1-1)"
  }
}
```

### CommandLogBroker (Internal)

The broker manages SSE event distribution using an in-memory per-request session:

- **Ring Buffer**: Up to 10,000 events per request, enabling late-joining clients to replay history
- **Non-blocking Publish**: Slow subscribers don't block the command execution pipeline
- **Auto-cleanup**: Sessions are removed 30 seconds after `CommandDone`
- **Late Join**: Clients connecting after execution starts receive all buffered events, then live updates

---

## SSH Host Key Verification (TOFU)

### What is TOFU?

**TOFU (Trust On First Use)** is a security model where:
1. **First Connection**: The SSH host key is stored and trusted.
2. **Subsequent Connections**: The stored key is compared with the presented key.
3. **Mismatch Detection**: If keys don't match, the connection is rejected (possible MITM attack).

This is the same model used by the `ssh` command-line tool (`~/.ssh/known_hosts`).

### How TOFU Works in Tumblebug

```mermaid
stateDiagram-v2
    [*] --> CheckStoredKey: SSH Connection Attempt
    
    CheckStoredKey --> FirstConnection: No stored key
    CheckStoredKey --> VerifyKey: Key exists
    
    FirstConnection --> StoreKey: Trust & Store
    StoreKey --> AllowConnection: Key Saved
    
    VerifyKey --> AllowConnection: Keys Match ✓
    VerifyKey --> RejectConnection: Keys Mismatch ✗
    
    AllowConnection --> [*]: SSH Session Established
    RejectConnection --> [*]: Error: SshHostKeyMismatchError
```

### Host Key Storage

Host keys are stored in the `VmInfo` structure:

```go
type VmInfo struct {
    // ... other fields ...
    
    // SshHostKeyInfo contains SSH host key for TOFU verification
    SshHostKeyInfo *SshHostKeyInfo `json:"sshHostKeyInfo,omitempty"`
}

type SshHostKeyInfo struct {
    HostKey     string  // Base64-encoded public key
    KeyType     string  // ssh-rsa, ssh-ed25519, ecdsa-sha2-nistp256
    Fingerprint string  // SHA256 fingerprint
    FirstUsedAt string  // Timestamp of first trust (RFC3339)
}
```

### Handling Host Key Changes

When a VM is recreated (terminated and created again), its SSH host key changes. This is legitimate but will trigger a TOFU verification failure.

**Resolution Steps:**
1. Verify the key change is expected (VM was recreated, not compromised)
2. Reset the stored host key via API
3. Next connection will store the new key (TOFU)

```mermaid
sequenceDiagram
    actor User
    participant API as Tumblebug API
    participant VM as Target VM
    
    Note over User,VM: After VM Recreation
    
    User->>API: POST /cmd/mci/{mciId}
    API->>VM: SSH Connect
    VM-->>API: New Host Key
    API-->>User: Error: Host Key Mismatch
    
    Note over User: Verify change is legitimate
    
    User->>API: DELETE /vm/{vmId}/sshHostKey
    API-->>User: Key Reset Success
    
    User->>API: POST /cmd/mci/{mciId}
    API->>VM: SSH Connect
    VM-->>API: Host Key
    Note over API: TOFU: Store new key
    API->>VM: Execute Command
    VM-->>API: Result
    API-->>User: Success
```

### Independent Key Management: Bastion vs Target

Both Bastion and Target VMs have their own SSH host keys, managed independently:

| VM Role | Key Storage Location | Verification |
|---------|---------------------|--------------|
| Bastion | `bastion.SshHostKeyInfo` | TOFU on first jump |
| Target | `target.SshHostKeyInfo` | TOFU on final connection |

This ensures:
- Bastion compromise is detected if its key changes
- Target compromise is detected if its key changes
- Each VM's security is independently verified

---

## API Reference

### Execute Command on MCI

Execute commands on VMs within an MCI.

```
POST /tumblebug/ns/{nsId}/cmd/mci/{mciId}
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `subGroupId` | string | Target specific subgroup only |
| `vmId` | string | Target specific VM only |
| `labelSelector` | string | Filter VMs by label (e.g., `role=worker`) |
| `async` | bool | Set to `true` for async mode with SSE streaming |

**Request Body:**
```json
{
  "userName": "cb-user",
  "command": ["echo 'Hello World'", "hostname", "uname -a"]
}
```

**Response:**
```json
{
  "results": [
    {
      "mciId": "mci01",
      "vmId": "g1-1",
      "vmIp": "10.0.1.5",
      "command": {
        "0": "echo 'Hello World'",
        "1": "hostname",
        "2": "uname -a"
      },
      "stdout": {
        "0": "Hello World",
        "1": "g1-1",
        "2": "Linux g1-1 5.15.0-generic x86_64 GNU/Linux"
      },
      "stderr": {},
      "error": ""
    }
  ]
}
```

**Async Response (when `?async=true`):**

Returns `HTTP 202 Accepted` immediately:
```json
{
  "xRequestId": "cmd-mci01-1234567890",
  "message": "Command execution started. Use GET /tumblebug/ns/{nsId}/stream/cmd/mci/{mciId}?xRequestId={xRequestId} for real-time streaming."
}
```

### Stream Command Execution Logs (SSE)

Subscribe to real-time command execution events via Server-Sent Events.

```
GET /tumblebug/ns/{nsId}/stream/cmd/mci/{mciId}?xRequestId={xRequestId}
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `xRequestId` | string | **Required.** The request ID returned from `POST /cmd/mci/{mciId}?async=true` |

**Response:** `Content-Type: text/event-stream`

The response is an SSE stream. Each event is delivered as:
```
data: {"type":"CommandStatus","vmId":"g1-1", ...}

data: {"type":"CommandLog","vmId":"g1-1","log":{"stream":"stdout","line":"Hello","lineNumber":1}, ...}

data: {"type":"CommandDone","summary":{"totalVms":3,"completedVms":3,"failedVms":0,"elapsedSeconds":12}}
```

The stream ends after the `CommandDone` event is sent. A keepalive comment (`: keepalive`) is sent every 15 seconds to prevent proxy/load-balancer timeouts.

**Notes:**
- The SSE endpoint requires **BasicAuth** (same credentials as other Tumblebug APIs)
- Standard `EventSource` API does not support custom headers; use `fetch()` with `ReadableStream` for browser clients
- If the client connects after execution has started, buffered events are replayed first

### Transfer File to MCI

Upload a file to VMs within an MCI.

```
POST /tumblebug/ns/{nsId}/file/mci/{mciId}
Content-Type: multipart/form-data
```

**Form Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `file` | file | File to upload |
| `targetPath` | string | Destination directory on VM |
| `subGroupId` | string | Target specific subgroup (optional) |
| `vmId` | string | Target specific VM (optional) |

**Response:**
```json
{
  "results": [
    {
      "mciId": "mci01",
      "vmId": "g1-1",
      "vmIp": "10.0.1.5",
      "stdout": {
        "0": "File transfer successful: /home/cb-user/config.yaml"
      },
      "stderr": {},
      "error": ""
    }
  ]
}
```

### Get SSH Host Key Info

Retrieve stored SSH host key information for a VM.

```
GET /tumblebug/ns/{nsId}/mci/{mciId}/vm/{vmId}/sshHostKey
```

**Response (key exists):**
```json
{
  "hostKey": "AAAAC3NzaC1lZDI1NTE5AAAAI...",
  "keyType": "ssh-ed25519",
  "fingerprint": "SHA256:abcdef123456...",
  "firstUsedAt": "2024-01-15T10:30:00Z"
}
```

**Response (no key stored yet):**
```json
{}
```

### Reset SSH Host Key

Reset the stored SSH host key for a VM. Use when a VM has been legitimately recreated.

```
DELETE /tumblebug/ns/{nsId}/mci/{mciId}/vm/{vmId}/sshHostKey
```

**Response:**
```json
{
  "message": "SSH host key for VM 'g1-1' has been reset. The next SSH connection will store the new host key (TOFU)."
}
```

---

## Usage Examples

### Example 1: Execute Command on All VMs

```bash
curl -X POST "http://localhost:1323/tumblebug/ns/default/cmd/mci/mci01" \
  -H "Content-Type: application/json" \
  -d '{
    "userName": "cb-user",
    "command": ["df -h", "free -m"]
  }'
```

### Example 2: Execute Command on Specific VM

```bash
curl -X POST "http://localhost:1323/tumblebug/ns/default/cmd/mci/mci01?vmId=g1-1" \
  -H "Content-Type: application/json" \
  -d '{
    "userName": "cb-user",
    "command": ["systemctl status nginx"]
  }'
```

### Example 3: Execute Command on VMs with Label

```bash
curl -X POST "http://localhost:1323/tumblebug/ns/default/cmd/mci/mci01?labelSelector=role=worker" \
  -H "Content-Type: application/json" \
  -d '{
    "userName": "cb-user",
    "command": ["docker ps"]
  }'
```

### Example 4: Transfer File to All VMs

```bash
curl -X POST "http://localhost:1323/tumblebug/ns/default/file/mci/mci01" \
  -F "file=@./config.yaml" \
  -F "targetPath=/home/cb-user/"
```

### Example 5: Handle Host Key Mismatch

When you receive an error like:
```
SSH host key mismatch for VM 'g1-1'. Stored key (ssh-ed25519, SHA256:abc...) 
does not match received key (ssh-ed25519, SHA256:xyz...). 
This could indicate a MITM attack or the VM was recreated.
```

**Step 1:** Verify the change is legitimate (check if VM was recently recreated)

**Step 2:** Reset the host key
```bash
curl -X DELETE "http://localhost:1323/tumblebug/ns/default/mci/mci01/vm/g1-1/sshHostKey"
```

**Step 3:** Retry the command (new key will be stored via TOFU)
```bash
curl -X POST "http://localhost:1323/tumblebug/ns/default/cmd/mci/mci01?vmId=g1-1" \
  -H "Content-Type: application/json" \
  -d '{
    "userName": "cb-user",
    "command": ["hostname"]
  }'
```

### Example 6: Check Stored Host Key

```bash
curl -X GET "http://localhost:1323/tumblebug/ns/default/mci/mci01/vm/g1-1/sshHostKey"
```

### Example 7: Async Command with SSE Streaming

**Step 1:** Start command execution in async mode

```bash
curl -X POST "http://localhost:1323/tumblebug/ns/default/cmd/mci/mci01?async=true" \
  -H "Content-Type: application/json" \
  -H "x-request-id: my-cmd-001" \
  -u default:default \
  -d '{
    "command": ["apt update", "apt install -y nginx"],
    "timeoutMinutes": 10
  }'
```

Response:
```json
{
  "xRequestId": "my-cmd-001",
  "message": "Command execution started. Use GET /tumblebug/ns/{nsId}/stream/cmd/mci/{mciId}?xRequestId={xRequestId} for real-time streaming."
}
```

**Step 2:** Connect to SSE stream for real-time logs

```bash
curl -N "http://localhost:1323/tumblebug/ns/default/stream/cmd/mci/mci01?xRequestId=my-cmd-001" \
  -H "Accept: text/event-stream" \
  -u default:default
```

Output (SSE stream):
```
: connected to stream for xRequestId=my-cmd-001

data: {"type":"CommandStatus","vmId":"g1-1","commandIndex":1,"timestamp":"...","status":{"status":"Queued"}}

data: {"type":"CommandStatus","vmId":"g1-1","commandIndex":1,"timestamp":"...","status":{"status":"Handling"}}

data: {"type":"CommandLog","vmId":"g1-1","commandIndex":1,"timestamp":"...","log":{"stream":"stdout","line":"Hit:1 http://archive.ubuntu.com/ubuntu jammy InRelease","lineNumber":1}}

data: {"type":"CommandLog","vmId":"g1-1","commandIndex":1,"timestamp":"...","log":{"stream":"stdout","line":"Reading package lists...","lineNumber":2}}

data: {"type":"CommandStatus","vmId":"g1-1","commandIndex":1,"timestamp":"...","status":{"status":"Completed"}}

data: {"type":"CommandDone","timestamp":"...","summary":{"totalVms":3,"completedVms":3,"failedVms":0,"elapsedSeconds":45}}
```

### Example 8: Async Command with SSE (JavaScript/Browser)

```javascript
// Step 1: Start async command
const response = await fetch('http://localhost:1323/tumblebug/ns/default/cmd/mci/mci01?async=true', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Basic ' + btoa('default:default'),
    'x-request-id': 'my-cmd-001'
  },
  body: JSON.stringify({
    command: ['echo hello', 'hostname'],
    timeoutMinutes: 5
  })
});

const { xRequestId } = await response.json();

// Step 2: Connect to SSE stream (using fetch + ReadableStream for BasicAuth support)
const sseResponse = await fetch(
  `http://localhost:1323/tumblebug/ns/default/stream/cmd/mci/mci01?xRequestId=${xRequestId}`,
  {
    headers: {
      'Accept': 'text/event-stream',
      'Authorization': 'Basic ' + btoa('default:default')
    }
  }
);

const reader = sseResponse.body.getReader();
const decoder = new TextDecoder();
let buffer = '';

while (true) {
  const { done, value } = await reader.read();
  if (done) break;

  buffer += decoder.decode(value, { stream: true });

  let boundary;
  while ((boundary = buffer.indexOf('\n\n')) !== -1) {
    const message = buffer.substring(0, boundary);
    buffer = buffer.substring(boundary + 2);

    for (const line of message.split('\n')) {
      if (line.startsWith('data: ')) {
        const event = JSON.parse(line.substring(6));
        console.log(`[${event.type}]`, event.vmId || '', event);

        if (event.type === 'CommandDone') {
          console.log('All VMs finished:', event.summary);
        }
      }
    }
  }
}
```

---

## Security Considerations

### Why Not InsecureIgnoreHostKey?

Using `InsecureIgnoreHostKey` (accepting any host key) would make SSH vulnerable to **Man-in-the-Middle (MITM) attacks**:

```
[Attacker Scenario without TOFU]
User → Attacker's Fake VM → Real VM
      ↑                    
      Attacker intercepts all traffic
      and can modify commands/data
```

With TOFU:
- First connection establishes trust
- Subsequent connections verify the trusted key
- Any key change triggers an alert

### Best Practices

1. **Verify Key Changes**: Before resetting a host key, confirm the VM was legitimately recreated.
2. **Monitor Alerts**: Unexpected host key mismatches may indicate security issues.
3. **Use Label Selectors**: Target commands precisely to minimize exposure.
4. **Review Command Output**: Check stderr for any security-related warnings.
