# CB-Tumblebug Migration Guide from v0.11.9 to v0.11.12

This document provides a comprehensive guide for migrating from CB-Tumblebug v0.11.9 to v0.11.12.

## Table of Contents
- [Breaking Changes](#breaking-changes)
- [New Features](#new-features)
- [API Changes](#api-changes)
- [Data Model Updates](#data-model-updates)
- [Performance Improvements](#performance-improvements)
- [Bug Fixes](#bug-fixes)
- [Migration Steps](#migration-steps)

## Breaking Changes

### ⚠️ **Minor API Response Format Changes**

Unlike v0.11.8 to v0.11.9 which had major breaking changes, v0.11.9 to v0.11.12 introduces minimal breaking changes. Most existing API client code will continue to work without modifications.

### 1. Object Storage API Response Format

**Impact:** New Object Storage APIs return XML format instead of JSON for AWS S3 compatibility.

#### **XML Response Format:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
<Owner>
<ID>aws-ap-northeast-2</ID>
<DisplayName>aws-ap-northeast-2</DisplayName>
</Owner>
<Buckets>
</Buckets>
</ListAllMyBucketsResult>
```

**Migration Required:** Update client code that consumes Object Storage APIs to handle XML parsing instead of JSON.

### 2. ⚠️ **Command Status API Routing Update**

**Impact:** API endpoint change to resolve routing conflicts.

**Issue Resolved:** The previous routing conflict between `/commandStatus/{index}` and `/commandStatus/clear` has been resolved by changing the endpoint.

**API Change:**
```diff
- DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/clear
+ DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatusAll
```

**Migration Required:** Update all client code that uses the clear command status endpoint.

**Migration Steps:**
```bash
# ✅ New endpoint (v0.11.12+)
curl -X DELETE "/ns/default/mci/my-mci/vm/vm-001/commandStatusAll"

# ❌ Old endpoint (deprecated)
curl -X DELETE "/ns/default/mci/my-mci/vm/vm-001/commandStatus/clear"
```

## New Features

### 1. 🚀 **Complete Object Storage Management**

**Description:** Full-featured Object Storage (S3-compatible) management with comprehensive bucket and object operations.

**New API Endpoints:**
```http
GET    /resources/objectStorage                                          # List all buckets
GET    /resources/objectStorage/{objectStorageName}                      # Get bucket details
POST   /resources/objectStorage/{objectStorageName}                      # Create bucket
DELETE /resources/objectStorage/{objectStorageName}                      # Delete bucket
GET    /resources/objectStorage/{objectStorageName}/{objectKey}          # Get object
PUT    /resources/objectStorage/{objectStorageName}/{objectKey}          # Upload object
DELETE /resources/objectStorage/{objectStorageName}/{objectKey}          # Delete object
GET    /resources/objectStorage/presigned/download/{objectStorageName}/{objectKey}  # Generate download URL
GET    /resources/objectStorage/presigned/upload/{objectStorageName}/{objectKey}    # Generate upload URL
GET    /resources/objectStorage/{objectStorageName}/cors                 # Get CORS configuration
PUT    /resources/objectStorage/{objectStorageName}/cors                 # Set CORS configuration
GET    /resources/objectStorage/{objectStorageName}/location             # Get bucket location
GET    /resources/objectStorage/{objectStorageName}/versioning           # Get versioning status
PUT    /resources/objectStorage/{objectStorageName}/versioning           # Set versioning
GET    /resources/objectStorage/{objectStorageName}/versions             # List object versions
GET    /resources/objectStorage/{objectStorageName}/versions/{objectKey} # List specific object versions
```

**Key Features:**
- **S3-Compatible API**: Full AWS S3 API compatibility for seamless integration
- **Presigned URLs**: Secure temporary access URLs for uploads and downloads
- **CORS Management**: Cross-Origin Resource Sharing configuration
- **Versioning Support**: Object versioning for backup and rollback capabilities
- **Multi-Provider Support**: Works across AWS, Azure, GCP, and other cloud providers

### 2. Enhanced K8s Management Stability

**Description:** Significant improvements to Kubernetes cluster management with circuit breaker patterns and enhanced error handling.

**Key Features:**
- **Circuit Breaker Implementation**: Prevents cascade failures during K8s operations
- **Enhanced Error Recovery**: Better error handling and retry mechanisms
- **Improved Documentation**: Fixed K8s API documentation errors
- **Multi-Filtering Support**: Advanced filtering capabilities for K8s resources
- **Version Updates**: Updated Alibaba Cloud Kubernetes cluster versions

### 3. 🔧 **Improved MCI Provisioning Reliability**

**Description:** Enhanced MCI (Multi-Cloud Infrastructure) provisioning with better failure handling and status management.

**Key Features:**
- **All-VM-Failure Detection**: Proper handling when all VMs in an MCI fail to provision
- **Status Consistency**: Improved MCI status reporting and consistency
- **Enhanced Error Handling**: Better error propagation and user feedback
- **Parallel Processing**: Improved performance for large-scale deployments

### 4. Remote Command Status and History

**Description:** New feature to track and manage remote command execution status and history.

**New API Endpoints:**
```http
GET    /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus        # List command status records
DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus        # Delete command status by criteria
GET    /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index} # Get specific command status
DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index} # Delete specific command status
DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatusAll     # Clear all command status
```

**Key Features:**
- **Command History Tracking**: Track execution history of remote commands
- **Status Monitoring**: Real-time status updates for long-running commands
- **Selective Cleanup**: Delete specific command histories or clear all

### 5. Enhanced Price Information

**Description:** Integration with CB-Spider v0.11.12 for simplified price information retrieval.

**Benefits:**
- **Accurate Pricing**: Real-time pricing information from cloud providers
- **Cost Estimation**: Better cost estimation for resource planning
- **Multi-Provider Pricing**: Unified pricing information across different CSPs

## API Changes

### 🔥 **NEW: Complete Object Storage Management APIs**

**CRITICAL:** New comprehensive Object Storage management capabilities with S3-compatible APIs.

#### **Basic Object Storage Operations:**

##### **1. List All Buckets:**
```http
GET /resources/objectStorage?provider=aws&region=ap-northeast-2
```

**Response (XML):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
<Owner>
  <ID>aws-ap-northeast-2</ID>
  <DisplayName>aws-ap-northeast-2</DisplayName>
</Owner>
<Buckets>
  <Bucket>
    <Name>my-test-bucket</Name>
    <CreationDate>2025-01-02T10:30:00.000Z</CreationDate>
  </Bucket>
</Buckets>
</ListAllMyBucketsResult>
```

##### **2. Create Bucket:**
```http
POST /resources/objectStorage/my-unique-bucket-name
Content-Type: application/json

{
  "provider": "aws",
  "region": "ap-northeast-2"
}
```

##### **3. Upload Object:**
```http
PUT /resources/objectStorage/my-bucket/test-file.txt
Content-Type: text/plain

File content goes here...
```

##### **4. Generate Presigned URLs:**
```http
GET /resources/objectStorage/presigned/download/my-bucket/test-file.txt?expires=3600
```

**Response (XML):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<PresignedURLResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
<PresignedURL>https://my-bucket.s3.amazonaws.com/test-file.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&amp;X-Amz-Expires=3600...</PresignedURL>
<Expires>3600</Expires>
<Method>GET</Method>
</PresignedURLResult>
```

#### **Advanced Object Storage Features:**

##### **5. CORS Configuration:**
```http
PUT /resources/objectStorage/my-bucket/cors
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<CORSConfiguration>
  <CORSRule>
    <AllowedOrigin>*</AllowedOrigin>
    <AllowedMethod>GET</AllowedMethod>
    <AllowedMethod>POST</AllowedMethod>
    <AllowedHeader>*</AllowedHeader>
  </CORSRule>
</CORSConfiguration>
```

##### **6. Versioning Management:**
```http
PUT /resources/objectStorage/my-bucket/versioning
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<VersioningConfiguration>
  <Status>Enabled</Status>
</VersioningConfiguration>
```

### 🔥 **NEW: Remote Command Status Management APIs**

**VERIFIED:** These are actual new API endpoints confirmed in the swagger.json specification.

**⚠️ IMPORTANT API ROUTING NOTE:**
Due to Echo router behavior, ensure you use the **exact** `/clear` endpoint for clearing all command status. The router may interpret "clear" as an index parameter if not handled carefully.

#### **Command Status Tracking:**
```http
GET /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus
```

**Response:**
```json
{
  "commandStatuses": [
    {
      "commandId": "cmd-12345",
      "command": "sudo apt-get update",
      "status": "completed",
      "result": "Package lists updated successfully",
      "startTime": "2025-01-02T10:30:00Z",
      "endTime": "2025-01-02T10:31:00Z",
      "exitCode": 0
    }
  ]
}
```

#### **Get Specific Command Status:**
```http
GET /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index}
```

#### **⚠️ Clear All Command History (Updated endpoint):**
```http
DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatusAll
```

#### **Delete Specific Command Status:**
```http
DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index}
```

#### **Delete Command Status by Criteria:**
```http
DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus?status=completed&older_than=24h
```

### Enhanced MCI Dynamic Review

**Description:** Improved MCI dynamic review with better validation and cost estimation.

**Enhanced Features:**
- Better resource availability validation
- Improved cost estimation accuracy
- Enhanced error reporting and suggestions
- Integration with latest pricing information

## Data Model Updates

### 1. New Object Storage Models

**Added comprehensive Object Storage data structures:**

```go
// New in v0.11.12
type ListAllMyBucketsResult struct {
    Owner   Owner    `xml:"Owner"`
    Buckets []Bucket `xml:"Buckets>Bucket"`
}

type Bucket struct {
    Name         string `xml:"Name"`
    CreationDate string `xml:"CreationDate"`
}

type Owner struct {
    ID          string `xml:"ID"`
    DisplayName string `xml:"DisplayName"`
}

type PresignedURLResult struct {
    PresignedURL string `xml:"PresignedURL"`
    Expires      int    `xml:"Expires"`
    Method       string `xml:"Method"`
}

type CORSConfiguration struct {
    CORSRules []CORSRule `xml:"CORSRule"`
}

type CORSRule struct {
    AllowedOrigins []string `xml:"AllowedOrigin"`
    AllowedMethods []string `xml:"AllowedMethod"`
    AllowedHeaders []string `xml:"AllowedHeader"`
    MaxAgeSeconds  int      `xml:"MaxAgeSeconds,omitempty"`
}

type VersioningConfiguration struct {
    Status string `xml:"Status"` // "Enabled" or "Suspended"
}
```

### 2. Enhanced Command Status Models

**Updated command execution tracking structures:**

```go
// Enhanced in v0.11.12
type CommandStatusInfo struct {
    CommandId   string    `json:"commandId"`
    Command     string    `json:"command"`
    Status      string    `json:"status"`      // "running", "completed", "failed"
    Result      string    `json:"result"`
    StartTime   time.Time `json:"startTime"`
    EndTime     *time.Time `json:"endTime,omitempty"`
    ExitCode    *int      `json:"exitCode,omitempty"`
    Error       string    `json:"error,omitempty"`
}

type CommandStatusListResponse struct {
    CommandStatuses []CommandStatusInfo `json:"commandStatuses"`
    TotalCount      int                 `json:"totalCount"`
}
```

### 3. Updated Credential Configuration

**Enhanced credential management for Object Storage:**

```yaml
# template.credentials.yaml - New Object Storage section
credentials:
  aws:
    # ... existing fields
    object_storage:
      access_key_id: "YOUR_ACCESS_KEY"
      secret_access_key: "YOUR_SECRET_KEY"
      region: "us-east-1"
  
  azure:
    # ... existing fields  
    object_storage:
      account_name: "yourstorageaccount"
      account_key: "YOUR_ACCOUNT_KEY"
      
  gcp:
    # ... existing fields
    object_storage:
      type: "service_account"
      project_id: "your-project-id"
      private_key: "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"
```

## Performance Improvements

### 1. Enhanced K8s Management Performance

**Description:** Significant performance improvements in Kubernetes cluster operations.

**Improvements:**
- **Circuit Breaker Pattern**: Prevents resource exhaustion during high-load scenarios
- **Parallel Processing**: Concurrent operations for faster cluster management
- **Optimized API Calls**: Reduced redundant API calls to cloud providers
- **Enhanced Caching**: Improved caching mechanisms for frequently accessed data

### 2. Improved MCI Provisioning Speed

**Description:** Faster and more reliable MCI provisioning with enhanced parallel processing.

**Key Improvements:**
- **Parallel VM Creation**: Improved parallel processing for large-scale deployments
- **Early Failure Detection**: Faster detection and handling of provisioning failures
- **Optimized Status Checking**: Reduced overhead in status monitoring
- **Enhanced Resource Management**: Better resource allocation and cleanup

### 3. Reduced Command Execution Overhead

**Description:** More efficient remote command execution with status tracking.

**Benefits:**
- **Asynchronous Processing**: Non-blocking command execution
- **Efficient Status Updates**: Optimized status tracking mechanisms
- **Reduced Memory Usage**: Better memory management for command history
- **Faster Response Times**: Improved API response times for command operations

## Bug Fixes

### 1. MCI Provisioning All-Failed Status Fix

**Issue:** When all VMs in an MCI failed to provision, the system would remain in a pending state instead of properly reporting failure.

**Fix:** Enhanced failure detection and status reporting to immediately return error status when all VMs fail.

#### Before (v0.11.9):
```bash
# MCI would stay in "Creating" status indefinitely when all VMs failed
GET /ns/default/mci/my-mci
# Response: {"status": "Creating", "targetStatus": "Running"}
```

#### After (v0.11.12):
```bash
# MCI properly reports failure when all VMs fail
GET /ns/default/mci/my-mci  
# Response: {"status": "Failed", "targetStatus": "Complete"}
```

### 2. K8s API Documentation Errors

**Issue:** Various documentation errors and missing API endpoint descriptions in Kubernetes management.

**Fix:** Comprehensive documentation updates and API specification corrections.

### 3. Object Storage API Error Handling

**Issue:** Inconsistent error handling in Object Storage operations leading to unclear error messages.

**Fix:** Enhanced error handling with proper HTTP status codes and detailed error messages.

### 4. Enhanced Circuit Breaker Implementation

**Issue:** K8s operations could cause cascade failures under high load conditions.

**Fix:** Implemented circuit breaker pattern to prevent system overload and improve stability.

## Migration Steps

### Step 1: 🔥 **NEW - Object Storage Integration**

**IMPORTANT:** New Object Storage capabilities require credential configuration updates.

#### **1.1 Update Credential Configuration:**
```yaml
# Update conf/template.credentials.yaml
credentials:
  aws:
    access_key_id: "YOUR_ACCESS_KEY"
    secret_access_key: "YOUR_SECRET_KEY"
    region: "us-east-1"
    # NEW: Add Object Storage configuration
    object_storage:
      access_key_id: "YOUR_S3_ACCESS_KEY"      # Can be same as above
      secret_access_key: "YOUR_S3_SECRET_KEY"  # Can be same as above
      region: "us-east-1"                      # Can be same as above
```

#### **1.2 Test Object Storage APIs:**
```bash
# Test bucket listing
curl -X GET "http://localhost:1323/resources/objectStorage?provider=aws&region=ap-northeast-2"

# Test bucket creation
curl -X POST "http://localhost:1323/resources/objectStorage/my-test-bucket-12345" \
  -H "Content-Type: application/json" \
  -d '{"provider": "aws", "region": "ap-northeast-2"}'

# Test object upload
echo "Hello World" | curl -X PUT "http://localhost:1323/resources/objectStorage/my-test-bucket-12345/hello.txt" \
  -H "Content-Type: text/plain" \
  --data-binary @-
```

### Step 2: Update API Client Libraries

**Unlike v0.11.8→v0.11.9, most existing client code will continue to work.**

#### **2.1 Add Object Storage Support (Optional):**
```go
// Add Object Storage client support
type ObjectStorageClient struct {
    BaseURL string
    Client  *http.Client
}

func (c *ObjectStorageClient) ListBuckets(provider, region string) (*ListAllMyBucketsResult, error) {
    resp, err := c.Client.Get(fmt.Sprintf("%s/resources/objectStorage?provider=%s&region=%s", 
        c.BaseURL, provider, region))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Parse XML response
    var result ListAllMyBucketsResult
    if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

#### **2.2 Add Command Status Tracking (Optional):**
```go
// Add command status tracking
func (c *TumblebugClient) GetCommandStatus(nsId, mciId, vmId string) (*CommandStatusListResponse, error) {
    resp, err := c.Client.Get(fmt.Sprintf("%s/ns/%s/mci/%s/vm/%s/commandStatus", 
        c.BaseURL, nsId, mciId, vmId))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result CommandStatusListResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

### Step 3: Test Enhanced MCI Provisioning

#### **3.1 Test Improved Failure Handling:**
```bash
# Test MCI creation with invalid specifications to verify proper failure reporting
curl -X POST "http://localhost:1323/ns/default/mciDynamic" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-failure-handling",
    "subGroups": [
      {
        "name": "invalid-sg",
        "specId": "invalid-spec-id",
        "imageId": "invalid-image-id"
      }
    ]
  }'

# Verify that the MCI status properly reports failure instead of staying in pending
curl -X GET "http://localhost:1323/ns/default/mci/test-failure-handling"
# Should return status: "Failed" instead of hanging in "Creating"
```

#### **3.2 Test Command Status Tracking:**
```bash
# Execute a command and track its status
curl -X POST "http://localhost:1323/ns/default/cmd/mci/my-mci" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "sleep 30 && echo Hello World"
  }'

# Check all command status records for a VM
curl -X GET "http://localhost:1323/ns/default/mci/my-mci/vm/vm-001/commandStatus"

# Get specific command status by index
curl -X GET "http://localhost:1323/ns/default/mci/my-mci/vm/vm-001/commandStatus/0"

# Delete specific command status by index
curl -X DELETE "http://localhost:1323/ns/default/mci/my-mci/vm/vm-001/commandStatus/0"

# Clear all command history when done
curl -X DELETE "http://localhost:1323/ns/default/mci/my-mci/vm/vm-001/commandStatusAll"
```

### Step 4: Update Monitoring and Alerting

#### **4.1 Update Health Checks:**
```bash
# Add Object Storage health checks
curl -X GET "http://localhost:1323/resources/objectStorage?provider=aws&region=ap-northeast-2"

# Add command status monitoring
curl -X GET "http://localhost:1323/ns/default/mci/my-mci/vm/vm-001/commandStatus"
```

#### **4.2 Update Error Monitoring:**
```bash
# Monitor for proper failure status reporting
curl -X GET "http://localhost:1323/ns/default/mci" | jq '.mcis[] | select(.status == "Failed")'
```

### Step 5: Validate Enhanced Features

#### **5.1 Test K8s Stability Improvements:**
```bash
# Test K8s cluster operations with circuit breaker protection
curl -X POST "http://localhost:1323/ns/default/k8sCluster" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-k8s-stability",
    "k8sNodeGroupList": [
      {
        "name": "worker-nodes",
        "desiredNodeSize": "3"
      }
    ]
  }'
```

#### **5.2 Test Object Storage Features:**
```bash
# Test full Object Storage workflow
# 1. Create bucket
curl -X POST "http://localhost:1323/resources/objectStorage/test-bucket-$(date +%s)" \
  -H "Content-Type: application/json" \
  -d '{"provider": "aws", "region": "ap-northeast-2"}'

# 2. Upload object
echo "Test content" | curl -X PUT "http://localhost:1323/resources/objectStorage/test-bucket-$(date +%s)/test.txt" \
  -H "Content-Type: text/plain" \
  --data-binary @-

# 3. Generate presigned URL
curl -X GET "http://localhost:1323/resources/objectStorage/presigned/download/test-bucket-$(date +%s)/test.txt?expires=3600"
```

## Post-Migration Verification

### 1. ✅ **Verify Core Functionality**

**Test that existing functionality continues to work:**

```bash
# Test existing MCI operations
curl -X GET "http://localhost:1323/ns/default/mci"

# Test existing VM operations  
curl -X GET "http://localhost:1323/ns/default/mci/my-mci/vm"

# Test existing resource operations
curl -X GET "http://localhost:1323/ns/default/resources/spec"
```

### 2. ✅ **Test New Object Storage Features**

```bash
# Comprehensive Object Storage testing
./scripts/test-object-storage.sh
```

### 3. ✅ **Verify Enhanced Error Handling**

- ✅ **Test MCI provisioning failure scenarios**
- ✅ **Verify proper status reporting for failed operations**  
- ✅ **Confirm K8s stability under load**
- ✅ **Test command execution status tracking**

### 4. ✅ **Monitor Performance Improvements**

- ✅ **Monitor MCI provisioning times** (should be faster)
- ✅ **Check K8s operation stability** (should be more reliable)
- ✅ **Verify command execution efficiency** (should be more responsive)

## Conclusion

CB-Tumblebug v0.11.12 introduces **significant new capabilities** while maintaining **excellent backward compatibility**. Unlike the major breaking changes in v0.11.8→v0.11.9, this release focuses on new features and stability improvements.

### 🚀 **Major New Capabilities in v0.11.12:**

1. **🔥 Complete Object Storage Management:** Full S3-compatible Object Storage APIs with bucket management, object operations, presigned URLs, CORS, and versioning
2. **Enhanced K8s Stability:** Circuit breaker patterns, improved error handling, and better performance
3. **Improved MCI Reliability:** Better failure detection, enhanced status reporting, and faster provisioning
4. **Remote Command Tracking:** New command status and history management capabilities
5. **Enhanced Price Information:** Integration with CB-Spider v0.11.12 for accurate pricing

### ✅ **Migration Impact:**

- **🟢 Minimal Breaking Changes:** Most existing API client code will continue to work
- **🟢 Backward Compatibility:** Existing workflows and integrations remain functional
- **🟡 New XML Responses:** Object Storage APIs return XML format for S3 compatibility
- **🔥 New Feature Integration:** Optional adoption of Object Storage and command tracking features

### 📞 **Support:**

For support or questions regarding this migration, please refer to the CB-Tumblebug GitHub repository or contact the development team.

---

**Last Updated:** 2025-01-02  
**Document Version:** 1.0  
**CB-Tumblebug Version Range:** v0.11.9 → v0.11.12
