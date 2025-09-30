# CB-Tumblebug API Changes Report

**Analysis Date**: 2025-09-30 15:07:46
**Version Comparison**: v0.11.8 → v0.11.13

## 📊 Summary

| Change Type | Count |
|-------------|--------|
| Modified Endpoints | 8 |
| Endpoint Changes | 4 |
| Removed Endpoints | 1 |
| New Endpoints | 25 |
| New Schemas | 0 |
| Modified Schemas | 0 |
| New Categories | 2 |

⚠️ **BREAKING CHANGES DETECTED**: 5 potential breaking changes

## 🔄 Version Information

- **Old Version**: v0.11.8
- **New Version**: v0.11.13

## 🔧 Modified Endpoints (Critical Changes)

**Total Modified**: 8 endpoints

### PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/autoscaleSize

#### 🔧 Parameter Changes
**Modified Parameters:**
- `changeK8sNodeGroupAutoscaleSizeReq` (body)
  - **Description**: Details of the TbChangeK8sNodeGroupAutoscaleSizeReq object → Details of the ChangeK8sNodeGroupAutoscaleSizeReq object

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/autoscaleSize (old)
+++ PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/autoscaleSize (new)
@@ Changes in PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/autoscaleSize @@
   parameter: changeK8sNodeGroupAutoscaleSizeReq (body)
-    description: "Details of the TbChangeK8sNodeGroupAutoscaleSizeReq object"
+    description: "Details of the ChangeK8sNodeGroupAutoscaleSizeReq object"
```
</details>

---

### POST /recommendSpec

#### 📖 Description Changed
- **Old**: Recommend specs for configuring an infrastructure (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234
- **New**: Recommend specs for configuring an infrastructure (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234 Get available options by /recommendSpecOptions for filtering and prioritizing specs in RecommendSpec API

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- POST /recommendSpec (old)
+++ POST /recommendSpec (new)
@@ Changes in POST /recommendSpec @@
-  description: "Recommend specs for configuring an infrastructure (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234"
+  description: "Recommend specs for configuring an infrastructure (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234 Get available options by /recommendSpecOptions ... (truncated for diff)"
```
</details>

---

### POST /ns/{nsId}/resources/spec

#### 🔧 Parameter Changes
**Modified Parameters:**
- `specInfo` (body)
  - **Schema changes**: Added Properties:
  + regionLatitude:number
  + regionLongitude:number

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- POST /ns/{nsId}/resources/spec (old)
+++ POST /ns/{nsId}/resources/spec (new)
@@ Changes in POST /ns/{nsId}/resources/spec @@
   parameter: specInfo (body)
+    regionLatitude: number
+    regionLongitude: number
```
</details>

---

### PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/upgrade

#### 🔧 Parameter Changes
**Modified Parameters:**
- `upgradeK8sClusterReq` (body)
  - **Description**: Details of the TbUpgradeK8sClusterReq object → Details of the UpgradeK8sClusterReq object

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/upgrade (old)
+++ PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/upgrade (new)
@@ Changes in PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/upgrade @@
   parameter: upgradeK8sClusterReq (body)
-    description: "Details of the TbUpgradeK8sClusterReq object"
+    description: "Details of the UpgradeK8sClusterReq object"
```
</details>

---

### POST /ns/{nsId}/resources/filterSpecsByRange

#### 📖 Description Changed
- **Old**: Filter specs by range
- **New**: Filter specs by range. Use limit field to control the maximum number of results. If limit is 0 or not specified, returns all matching results.

#### 🔧 Parameter Changes
**Modified Parameters:**
- `specRangeFilter` (body)
  - **Schema changes**: Added Properties:
  + regionLatitude:number
  + limit:integer
  + regionLongitude:number
  - **Description**: Filter for range-filtering specs → Filter for range-filtering specs (limit: 0 for all results, >0 for limited results)

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- POST /ns/{nsId}/resources/filterSpecsByRange (old)
+++ POST /ns/{nsId}/resources/filterSpecsByRange (new)
@@ Changes in POST /ns/{nsId}/resources/filterSpecsByRange @@
-  description: "Filter specs by range"
+  description: "Filter specs by range. Use limit field to control the maximum number of results. If limit is 0 or not specified, returns all matching results."
   parameter: specRangeFilter (body)
-    description: "Filter for range-filtering specs"
+    description: "Filter for range-filtering specs (limit: 0 for all results, >0 for limited results)"
+    regionLatitude: number
+    limit: integer
+    regionLongitude: number
```
</details>

---

### GET /ns/{nsId}/resources/image/{imageId}

#### 📖 Description Changed
- **Old**: GetImage returns an image object if there are matched images for the given namespace and imageKey(Id, CspResourceName)
- **New**: GetImage returns an image object if there are matched images for the given namespace and imageKey(imageId)

#### 🔧 Parameter Changes
**Modified Parameters:**
- `imageId` (path)
  - **Description**: (Note: imageId param will be refined in next release, enabled for temporal support) This param accepts vaious input types as Image Key: cspImageName → (Note: imageId param will be refined in next release, enabled for temporal support) This param accepts several input forms: 1) provider+imageId, 2) provider+region+imageId, 3) imageId. For exact matching, use provider+imageId form.

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- GET /ns/{nsId}/resources/image/{imageId} (old)
+++ GET /ns/{nsId}/resources/image/{imageId} (new)
@@ Changes in GET /ns/{nsId}/resources/image/{imageId} @@
-  description: "GetImage returns an image object if there are matched images for the given namespace and imageKey(Id, CspResourceName)"
+  description: "GetImage returns an image object if there are matched images for the given namespace and imageKey(imageId)"
   parameter: imageId (path)
-    description: "(Note: imageId param will be refined in next release, enabled for temporal support) This param accepts vaious input types as Image Key: cspImageName"
+    description: "(Note: imageId param will be refined in next release, enabled for temporal support) This param accepts several input forms: 1) provider+imageId, 2) provider+region+imageId, 3) imageId. For exact matching, use provider+imageId form."
```
</details>

---

### PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/onAutoscaling

#### 🔧 Parameter Changes
**Modified Parameters:**
- `setK8sNodeGroupAutoscalingReq` (body)
  - **Description**: Details of the TbSetK8sNodeGroupAutoscalingReq object → Details of the SetK8sNodeGroupAutoscalingReq object

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/onAutoscaling (old)
+++ PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/onAutoscaling (new)
@@ Changes in PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/onAutoscaling @@
   parameter: setK8sNodeGroupAutoscalingReq (body)
-    description: "Details of the TbSetK8sNodeGroupAutoscalingReq object"
+    description: "Details of the SetK8sNodeGroupAutoscalingReq object"
```
</details>

---

### PUT /ns/{nsId}/resources/spec/{specId}

#### 🔧 Parameter Changes
**Modified Parameters:**
- `specInfo` (body)
  - **Schema changes**: Added Properties:
  + regionLatitude:number
  + regionLongitude:number

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- PUT /ns/{nsId}/resources/spec/{specId} (old)
+++ PUT /ns/{nsId}/resources/spec/{specId} (new)
@@ Changes in PUT /ns/{nsId}/resources/spec/{specId} @@
   parameter: specInfo (body)
+    regionLatitude: number
+    regionLongitude: number
```
</details>

---

## 🔄 Endpoint Changes (Path/Method/Content)

These endpoints have been modified in various ways:

### POST /ns/{nsId}/mci/{mciId}/vmDynamic → POST /ns/{nsId}/mci/{mciId}/subGroupDynamic
**Change Type**: Path & Operation ID
**Similarity Score**: 0.91

#### 📍 Path Change
```diff
- POST /ns/{nsId}/mci/{mciId}/vmDynamic
+ POST /ns/{nsId}/mci/{mciId}/subGroupDynamic
```

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- POST /ns/{nsId}/mci/{mciId}/vmDynamic
+++ POST /ns/{nsId}/mci/{mciId}/subGroupDynamic
@@ -1,7 +1,7 @@
 {
   "description": "Dynamically add new virtual machines to an existing MCI using common specifications and automated resource management. This endpoint provides elastic scaling capabilities for running MCIs: **Dynamic ... (truncated for diff)",
   "method": "POST",
-  "operationId": "PostMciVmDynamic",
+  "operationId": "PostMciSubGroupDynamic",
   "parameters": [
     {
       "in": "path",
@@ -22,7 +22,7 @@
       "type": null
     }
   ],
-  "path": "/ns/{nsId}/mci/{mciId}/vmDynamic",
+  "path": "/ns/{nsId}/mci/{mciId}/subGroupDynamic",
   "responses": [
     "200",
     "400",
```
</details>

---

### DELETE /ns/{nsId}/resources/objectStorage/{objectStorageId} → DELETE /resources/objectStorage/{objectStorageName}
**Change Type**: Path & Summary & Description & Parameters & Responses
**Similarity Score**: 0.77

#### 📍 Path Change
```diff
- DELETE /ns/{nsId}/resources/objectStorage/{objectStorageId}
+ DELETE /resources/objectStorage/{objectStorageName}
```

#### 📖 Summary Change
- **Old**: Delete a Object Storage
- **New**: Delete an object storage (bucket)

#### 📝 Description Change
- **Old**: Delete a Object Storage
- **New**: Delete an object storage (bucket)

#### 🔧 Parameters Changed
- **Old Parameter Count**: 2
- **New Parameter Count**: 2

#### 📤 Responses Changed
- **Old Response Codes**: 200, 400, 500, 503
- **New Response Codes**: 204

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- DELETE /ns/{nsId}/resources/objectStorage/{objectStorageId}
+++ DELETE /resources/objectStorage/{objectStorageName}
@@ -1,30 +1,27 @@
 {
-  "description": "Delete a Object Storage",
+  "description": "Delete an object storage (bucket)",
   "method": "DELETE",
   "operationId": "DeleteObjectStorage",
   "parameters": [
     {
       "in": "path",
-      "name": "nsId",
+      "name": "objectStorageName",
       "required": true,
       "type": "string"
     },
     {
-      "in": "path",
-      "name": "objectStorageId",
+      "in": "header",
+      "name": "credential",
       "required": true,
       "type": "string"
     }
   ],
-  "path": "/ns/{nsId}/resources/objectStorage/{objectStorageId}",
+  "path": "/resources/objectStorage/{objectStorageName}",
   "responses": [
-    "200",
-    "400",
-    "500",
-    "503"
+    "204"
   ],
-  "summary": "Delete a Object Storage",
+  "summary": "Delete an object storage (bucket)",
   "tags": [
-    "[Infra Resource] Object Storage Management (under development)"
+    "[Resource] Object Storage Management"
   ]
 }
```
</details>

---

### GET /ns/{nsId}/resources/objectStorage/{objectStorageId} → GET /resources/objectStorage/{objectStorageName}/cors
**Change Type**: Path & Summary & Description & Operation ID & Parameters & Responses
**Similarity Score**: 0.70

#### 📍 Path Change
```diff
- GET /ns/{nsId}/resources/objectStorage/{objectStorageId}
+ GET /resources/objectStorage/{objectStorageName}/cors
```

#### 📖 Summary Change
- **Old**: Get resource info of a Object Storage
- **New**: Get CORS configuration of an object storage (bucket)

#### 📝 Description Change
- **Old**: Get resource info of a Object Storage
- **New**: Get CORS configuration of an object storage (bucket) **Important Notes:** - The actual response will be XML format with root element `CORSConfiguration` **Actual XML Response Example:** `[XML Example]` [XML Tag] <CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"... (truncated)

#### 🔧 Parameters Changed
- **Old Parameter Count**: 3
- **New Parameter Count**: 2

#### 📤 Responses Changed
- **Old Response Codes**: 200, 400, 500, 503
- **New Response Codes**: 200, 404

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- GET /ns/{nsId}/resources/objectStorage/{objectStorageId}
+++ GET /resources/objectStorage/{objectStorageName}/cors
@@ -1,36 +1,28 @@
 {
-  "description": "Get resource info of a Object Storage",
+  "description": "Get CORS configuration of an object storage (bucket) **Important Notes:** - The actual response will be XML format with root element `CORSConfiguration` **Actual XML Response Example:** [XML Example] <?xml... (truncated for diff)",
   "method": "GET",
-  "operationId": "GetObjectStorage",
+  "operationId": "GetObjectStorageCORS",
   "parameters": [
     {
       "in": "path",
-      "name": "nsId",
+      "name": "objectStorageName",
       "required": true,
       "type": "string"
     },
     {
-      "in": "path",
-      "name": "objectStorageId",
+      "in": "header",
+      "name": "credential",
       "required": true,
-      "type": "string"
-    },
-    {
-      "in": "query",
-      "name": "detail",
-      "required": null,
       "type": "string"
     }
   ],
-  "path": "/ns/{nsId}/resources/objectStorage/{objectStorageId}",
+  "path": "/resources/objectStorage/{objectStorageName}/cors",
   "responses": [
     "200",
-    "400",
-    "500",
-    "503"
+    "404"
   ],
-  "summary": "Get resource info of a Object Storage",
+  "summary": "Get CORS configuration of an object storage (bucket)",
   "tags": [
-    "[Infra Resource] Object Storage Management (under development)"
+    "[Resource] Object Storage Management"
   ]
 }
```
</details>

---

### GET /ns/{nsId}/resources/objectStorage → GET /resources/objectStorage/{objectStorageName}/location
**Change Type**: Path & Summary & Description & Operation ID & Parameters & Responses
**Similarity Score**: 0.62

#### 📍 Path Change
```diff
- GET /ns/{nsId}/resources/objectStorage
+ GET /resources/objectStorage/{objectStorageName}/location
```

#### 📖 Summary Change
- **Old**: Get all Object Storages (TBD)
- **New**: Get the location of an object storage (bucket)

#### 📝 Description Change
- **Old**: Get all Object Storages (TBD)
- **New**: Get the location of an object storage (bucket) **Important Notes:** - The actual response will be XML format with root element `LocationConstraint` **Actual XML Response Example:** `[XML Example]` [XML Tag] [XML Tag]ap-... (truncated)

#### 🔧 Parameters Changed
- **Old Parameter Count**: 2
- **New Parameter Count**: 2

#### 📤 Responses Changed
- **Old Response Codes**: 200, 400, 500, 503
- **New Response Codes**: 200

<details>
<summary>🔍 View Detailed Diff</summary>

```diff
--- GET /ns/{nsId}/resources/objectStorage
+++ GET /resources/objectStorage/{objectStorageName}/location
@@ -1,30 +1,27 @@
 {
-  "description": "Get all Object Storages (TBD)",
+  "description": "Get the location of an object storage (bucket) **Important Notes:** - The actual response will be XML format with root element `LocationConstraint` **Actual XML Response Example:** [XML Example] <?xml vers... (truncated for diff)",
   "method": "GET",
-  "operationId": "GetAllObjectStorage",
+  "operationId": "GetObjectStorageLocation",
   "parameters": [
     {
       "in": "path",
-      "name": "nsId",
+      "name": "objectStorageName",
       "required": true,
       "type": "string"
     },
     {
-      "in": "query",
-      "name": "option",
-      "required": null,
+      "in": "header",
+      "name": "credential",
+      "required": true,
       "type": "string"
     }
   ],
-  "path": "/ns/{nsId}/resources/objectStorage",
+  "path": "/resources/objectStorage/{objectStorageName}/location",
   "responses": [
-    "200",
-    "400",
-    "500",
-    "503"
+    "200"
   ],
-  "summary": "Get all Object Storages (TBD)",
+  "summary": "Get the location of an object storage (bucket)",
   "tags": [
-    "[Infra Resource] Object Storage Management (under development)"
+    "[Resource] Object Storage Management"
   ]
 }
```
</details>

---

**⚠️ Migration Required**: Update client code to adapt to these changes

## ❌ Removed Endpoints (Breaking Changes)

### POST /ns/{nsId}/resources/objectStorage

- **Summary**: Create a Object Storages
- **Tags**: [Infra Resource] Object Storage Management (under development)
- **Parameters**: 3
- **Request Body**: No
- **Response Codes**: 200, 400, 500, 503

<details>
<summary>🔍 View Removed Endpoint Details</summary>

```json
{
  "method": "POST",
  "path": "/ns/{nsId}/resources/objectStorage",
  "summary": "Create a Object Storages",
  "description": "Create a Object Storages\n\nSupported CSPs: AWS, Azure\n- Note - `connectionName` example: aws-ap-northeast-2, azure-koreacentral\n\n- Note - Please check the `requiredCSPResource` property which includes CSP specific values.\n\n- Note - You can find the API usage examples on this link, https://github.com/cloud-barista/mc-terrarium/discussions/117\n",
  "operationId": "PostObjectStorage",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "description": "Request body to create a Object Storage",
      "name": "objectStorageReq",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/model.RestPostObjectStorageRequest"
      }
    },
    {
      "enum": [
        "retry"
      ],
      "type": "string",
      "description": "Action",
      "name": "action",
      "in": "query"
    }
  ],
  "responses": [
    "200",
    "400",
    "500",
    "503"
  ],
  "tags": [
    "[Infra Resource] Object Storage Management (under development)"
  ]
}
```
</details>

**⚠️ Migration Required**: These endpoints are no longer available

## ➕ New Endpoints

### PUT /resources/objectStorage/{objectStorageName}/cors

- **Summary**: Set CORS configuration of an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 3
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "PUT",
  "path": "/resources/objectStorage/{objectStorageName}/cors",
  "summary": "Set CORS configuration of an object storage (bucket)",
  "description": "Set CORS configuration of an object storage (bucket)\\n\\n**Important Notes:**\\n- The CORS configuration must be provided in the request body in XML format.\\n- The actual request body should have root element `CORSConfiguration`\\n\\n**Actual XML Request Body Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<CORSConfiguration>\\n<CORSRule>\\n<AllowedOrigin>https://example.com</AllowedOrigin>\\n<AllowedOrigin>https://app.example.com</AllowedOrigin>\\n<AllowedMethod>GET</AllowedMethod>\\n<AllowedMethod>PUT</A... (truncated)",
  "operationId": "SetObjectStorageCORS",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    },
    {
      "description": "CORS Configuration in XML format",
      "name": "reqBody",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/resource.CORSConfiguration"
      }
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### GET /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index}

- **Summary**: Get a specific command status by index for a VM
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 5
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index}",
  "summary": "Get a specific command status by index for a VM",
  "description": "Get a specific command status record by index for a VM",
  "operationId": "GetVmCommandStatus",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "g1-1",
      "description": "VM ID",
      "name": "vmId",
      "in": "path",
      "required": true
    },
    {
      "type": "integer",
      "default": 1,
      "description": "Command Index",
      "name": "index",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### GET /ns/{nsId}/mci/{mciId}/vm/{vmId}/handlingCount

- **Summary**: Get count of currently handling commands for a VM
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/ns/{nsId}/mci/{mciId}/vm/{vmId}/handlingCount",
  "summary": "Get count of currently handling commands for a VM",
  "description": "Get the number of commands currently in 'Handling' status for a specific VM. Optimized for frequent polling.",
  "operationId": "GetVmHandlingCommandCount",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "g1-1",
      "description": "VM ID",
      "name": "vmId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### POST /availableRegionZonesForSpecList

- **Summary**: Get available regions and zones for multiple specs
- **Tags**: [Infra Resource] Spec Management
- **Parameters**: 1
- **Request Body**: No
- **Response Codes**: 200, 400, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "POST",
  "path": "/availableRegionZonesForSpecList",
  "summary": "Get available regions and zones for multiple specs",
  "description": "Query the availability for multiple specs in parallel and return batch results",
  "operationId": "GetAvailableRegionZonesForSpecList",
  "parameters": [
    {
      "description": "Batch spec availability request",
      "name": "batchAvailabilityReq",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/model.GetAvailableRegionZonesListRequest"
      }
    }
  ],
  "responses": [
    "200",
    "400",
    "500"
  ],
  "tags": [
    "[Infra Resource] Spec Management"
  ]
}
```
</details>

---

### DELETE /resources/objectStorage/{objectStorageName}/versions/{objectKey}

- **Summary**: Delete a specific version of an object in an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 204

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "DELETE",
  "path": "/resources/objectStorage/{objectStorageName}/versions/{objectKey}",
  "summary": "Delete a specific version of an object in an object storage (bucket)",
  "description": "Delete a specific version of an object in an object storage (bucket)",
  "operationId": "DeleteVersionedObject",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "test-file.txt",
      "description": "Object Key",
      "name": "objectKey",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "yb4PgjnFVD2LfRZHXBjjsHBkQRHlu.TZ",
      "description": "Version ID",
      "name": "versionId",
      "in": "query",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "204"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatusAll

- **Summary**: Clear all command status records for a VM
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "DELETE",
  "path": "/ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatusAll",
  "summary": "Clear all command status records for a VM",
  "description": "Delete all command status records for a VM",
  "operationId": "ClearAllVmCommandStatus",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "g1-1",
      "description": "VM ID",
      "name": "vmId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### GET /ns/{nsId}/mci/{mciId}/handlingCount

- **Summary**: Get count of currently handling commands for all VMs in MCI
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 3
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/ns/{nsId}/mci/{mciId}/handlingCount",
  "summary": "Get count of currently handling commands for all VMs in MCI",
  "description": "Get the number of commands currently in 'Handling' status for all VMs in an MCI. Returns per-VM counts and total count.",
  "operationId": "GetMciHandlingCommandCount",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### POST /ns/{nsId}/updateExistingSpecListByAvailableRegionZones

- **Summary**: Clean up unavailable specs from database
- **Tags**: [Infra Resource] Spec Management
- **Parameters**: 2
- **Request Body**: No
- **Response Codes**: 200, 400, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "POST",
  "path": "/ns/{nsId}/updateExistingSpecListByAvailableRegionZones",
  "summary": "Clean up unavailable specs from database",
  "description": "Query all specs for a specific provider across all regions, check their availability, and remove specs that are not available in their respective regions",
  "operationId": "UpdateExistingSpecListByAvailableRegionZones",
  "parameters": [
    {
      "type": "string",
      "default": "system",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "description": "Spec cleanup request",
      "name": "cleanupReq",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/model.UpdateSpecListByAvailabilityRequest"
      }
    }
  ],
  "responses": [
    "200",
    "400",
    "500"
  ],
  "tags": [
    "[Infra Resource] Spec Management"
  ]
}
```
</details>

---

### DELETE /resources/objectStorage/{objectStorageName}/cors

- **Summary**: Delete CORS configuration of an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 2
- **Request Body**: No
- **Response Codes**: 204

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "DELETE",
  "path": "/resources/objectStorage/{objectStorageName}/cors",
  "summary": "Delete CORS configuration of an object storage (bucket)",
  "description": "Delete CORS configuration of an object storage (bucket)",
  "operationId": "DeleteObjectStorageCORS",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "204"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### GET /recommendSpecOptions

- **Summary**: Get options for RecommendSpec API
- **Tags**: [MC-Infra] MCI Provisioning and Management
- **Parameters**: 0
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/recommendSpecOptions",
  "summary": "Get options for RecommendSpec API",
  "description": "Get available options for filtering and prioritizing specs in RecommendSpec API",
  "operationId": "RecommendSpecOptions",
  "parameters": [],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Provisioning and Management"
  ]
}
```
</details>

---

### POST /availableRegionZonesForSpec

- **Summary**: Get available regions and zones for a specific spec
- **Tags**: [Infra Resource] Spec Management
- **Parameters**: 1
- **Request Body**: No
- **Response Codes**: 200, 400, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "POST",
  "path": "/availableRegionZonesForSpec",
  "summary": "Get available regions and zones for a specific spec",
  "description": "Query the availability of a specific spec across all regions/zones",
  "operationId": "GetAvailableRegionZonesForSpec",
  "parameters": [
    {
      "description": "Spec availability request",
      "name": "availabilityReq",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/model.GetAvailableRegionZonesRequest"
      }
    }
  ],
  "responses": [
    "200",
    "400",
    "500"
  ],
  "tags": [
    "[Infra Resource] Spec Management"
  ]
}
```
</details>

---

### DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index}

- **Summary**: Delete a specific command status by index for a VM
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 5
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "DELETE",
  "path": "/ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus/{index}",
  "summary": "Delete a specific command status by index for a VM",
  "description": "Delete a specific command status record by index for a VM",
  "operationId": "DeleteVmCommandStatus",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "g1-1",
      "description": "VM ID",
      "name": "vmId",
      "in": "path",
      "required": true
    },
    {
      "type": "integer",
      "default": 1,
      "description": "Command Index",
      "name": "index",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### GET /resources/objectStorage/presigned/download/{objectStorageName}/{objectKey}

- **Summary**: Generate a presigned URL for downloading an object from a bucket
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/resources/objectStorage/presigned/download/{objectStorageName}/{objectKey}",
  "summary": "Generate a presigned URL for downloading an object from a bucket",
  "description": "Generate a presigned URL for downloading an object from a bucket\\n\\n**Important Notes:**\\n- The actual response will be XML format with root element `PresignedURLResult`\\n- The `expires` query parameter specifies the expiration time in seconds for the presigned URL (default: 3600 seconds)\\n- The generated presigned URL can be used to download the object directly without further authentication\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<PresignedURLResult xmlns=\\\"ht... (truncated)",
  "operationId": "GeneratePresignedDownloadURL",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "test-object.txt",
      "description": "Object Name",
      "name": "objectKey",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    },
    {
      "type": "integer",
      "default": 3600,
      "description": "Expiration time in seconds for the presigned URL",
      "name": "expires",
      "in": "query"
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### GET /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus

- **Summary**: List command status records for a VM with filtering
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 13
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus",
  "summary": "List command status records for a VM with filtering",
  "description": "List command status records for a VM with various filtering options",
  "operationId": "ListVmCommandStatus",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "g1-1",
      "description": "VM ID",
      "name": "vmId",
      "in": "path",
      "required": true
    },
    {
      "type": "array",
      "items": {
        "enum": [
          "Queued",
          "Handling",
          "Completed",
          "Failed",
          "Timeout"
        ],
        "type": "string"
      },
      "collectionFormat": "csv",
      "description": "Filter by command execution status (can specify multiple)",
      "name": "status",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter by X-Request-ID",
      "name": "xRequestId",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter commands containing this text",
      "name": "commandContains",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter commands started from this time (RFC3339 format)",
      "name": "startTimeFrom",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter commands started until this time (RFC3339 format)",
      "name": "startTimeTo",
      "in": "query"
    },
    {
      "type": "integer",
      "description": "Filter commands from this index (inclusive)",
      "name": "indexFrom",
      "in": "query"
    },
    {
      "type": "integer",
      "description": "Filter commands to this index (inclusive)",
      "name": "indexTo",
      "in": "query"
    },
    {
      "type": "integer",
      "default": 50,
      "description": "Limit the number of results returned",
      "name": "limit",
      "in": "query"
    },
    {
      "type": "integer",
      "default": 0,
      "description": "Number of results to skip",
      "name": "offset",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### GET /resources/objectStorage/{objectStorageName}/versions

- **Summary**: List object versions in an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 2
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/resources/objectStorage/{objectStorageName}/versions",
  "summary": "List object versions in an object storage (bucket)",
  "description": "List object versions in an object storage (bucket)\\n\\n**Important Notes:**\\n- The actual response will be XML format with root element `ListVersionsResult`\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<ListVersionsResult xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\">\\n<Name>spider-test-bucket</Name>\\n<Prefix></Prefix>\\n<KeyMarker></KeyMarker>\\n<VersionIdMarker></VersionIdMarker>\\n<NextKeyMarker></NextKeyMarker>\\n<NextVersionIdMarker></NextVersionIdMarker>\\n<MaxKeys>100... (truncated)",
  "operationId": "ListObjectVersions",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### PUT /resources/objectStorage/{objectStorageName}

- **Summary**: Create an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 2
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "PUT",
  "path": "/resources/objectStorage/{objectStorageName}",
  "summary": "Create an object storage (bucket)",
  "description": "Create an object storage (bucket)\\n\\n**Important Notes:**\\n- The `objectStorageName` must be globally unique across all existing buckets in the S3 compatible storage.\\n- The bucket namespace is shared by all users of the system.",
  "operationId": "CreateObjectStorage",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### DELETE /ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus

- **Summary**: Delete multiple command status records by criteria for a VM
- **Tags**: [MC-Infra] MCI Remote Command Status
- **Parameters**: 11
- **Request Body**: No
- **Response Codes**: 200, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "DELETE",
  "path": "/ns/{nsId}/mci/{mciId}/vm/{vmId}/commandStatus",
  "summary": "Delete multiple command status records by criteria for a VM",
  "description": "Delete multiple command status records for a VM based on filtering criteria",
  "operationId": "DeleteVmCommandStatusByCriteria",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "g1-1",
      "description": "VM ID",
      "name": "vmId",
      "in": "path",
      "required": true
    },
    {
      "type": "array",
      "items": {
        "enum": [
          "Queued",
          "Handling",
          "Completed",
          "Failed",
          "Timeout"
        ],
        "type": "string"
      },
      "collectionFormat": "csv",
      "description": "Filter by command execution status (can specify multiple)",
      "name": "status",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter by X-Request-ID",
      "name": "xRequestId",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter commands containing this text",
      "name": "commandContains",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter commands started from this time (RFC3339 format)",
      "name": "startTimeFrom",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Filter commands started until this time (RFC3339 format)",
      "name": "startTimeTo",
      "in": "query"
    },
    {
      "type": "integer",
      "description": "Filter commands from this index (inclusive)",
      "name": "indexFrom",
      "in": "query"
    },
    {
      "type": "integer",
      "description": "Filter commands to this index (inclusive)",
      "name": "indexTo",
      "in": "query"
    },
    {
      "type": "string",
      "description": "Custom request ID",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Remote Command Status"
  ]
}
```
</details>

---

### GET /resources/objectStorage

- **Summary**: List object storages (buckets)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 1
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/resources/objectStorage",
  "summary": "List object storages (buckets)",
  "description": "Get the list of all object storages (buckets)\\n\\n**Important Notes:**\\n- The actual response will be XML format with root element `ListAllMyBucketsResult`\\n- The response includes xmlns attribute: `xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\"`\\n- Swagger UI may show `resource.ListAllMyBucketsResult` due to rendering limitations\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<ListAllMyBucketsResult xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\">\\n<Owner>\\n<ID>aws-ap-... (truncated)",
  "operationId": "ListObjectStorages",
  "parameters": [
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### GET /resources/objectStorage/presigned/upload/{objectStorageName}/{objectKey}

- **Summary**: Generate a presigned URL for uploading an object to a bucket
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/resources/objectStorage/presigned/upload/{objectStorageName}/{objectKey}",
  "summary": "Generate a presigned URL for uploading an object to a bucket",
  "description": "Generate a presigned URL for uploading an object to a bucket\\n\\n**Important Notes:**\\n- The actual response will be XML format with root element `PresignedURLResult`\\n- The `expires` query parameter specifies the expiration time in seconds for the presigned URL (default: 3600 seconds)\\n- The generated presigned URL can be used to upload the object directly without further authentication\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<PresignedURLResult xmlns=\\\"http://s... (truncated)",
  "operationId": "GeneratePresignedUploadURL",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "test-object.txt",
      "description": "Object Name",
      "name": "objectKey",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    },
    {
      "type": "integer",
      "default": 3600,
      "description": "Expiration time in seconds for the presigned URL",
      "name": "expires",
      "in": "query"
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### POST /ns/{nsId}/mci/{mciId}/subGroupDynamicReview

- **Summary**: Review VM Dynamic Addition Request for Existing MCI
- **Tags**: [MC-Infra] MCI Provisioning and Management
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 200, 400, 404, 500

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "POST",
  "path": "/ns/{nsId}/mci/{mciId}/subGroupDynamicReview",
  "summary": "Review VM Dynamic Addition Request for Existing MCI",
  "description": "Review and validate a VM dynamic addition request for an existing MCI before actual provisioning.\\nThis endpoint provides comprehensive validation for adding new VMs to existing MCIs without actually creating resources.\\nIt checks resource availability, validates specifications and images, estimates costs, and provides detailed recommendations.\\n\\n**Key Features:**\\n- Validates VM specification and image against CSP availability\\n- Checks compatibility with existing MCI configuration\\n- Provides cost e... (truncated)",
  "operationId": "PostMciDynamicSubGroupVmReview",
  "parameters": [
    {
      "type": "string",
      "default": "default",
      "description": "Namespace ID containing the target MCI",
      "name": "nsId",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "mci01",
      "description": "MCI ID to which the VM will be added",
      "name": "mciId",
      "in": "path",
      "required": true
    },
    {
      "description": "Request body to review VM dynamic addition. Must include specId and imageId info. (ex: {name: web-servers, specId: aws+ap-northeast-2+t2.small, imageId: aws+ap-northeast-2+ubuntu22.04, subGroupSize: 2})",
      "name": "vmReq",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/model.CreateSubGroupDynamicReq"
      }
    },
    {
      "type": "string",
      "description": "Custom request ID for tracking",
      "name": "x-request-id",
      "in": "header"
    }
  ],
  "responses": [
    "200",
    "400",
    "404",
    "500"
  ],
  "tags": [
    "[MC-Infra] MCI Provisioning and Management"
  ]
}
```
</details>

---

### GET /resources/objectStorage/{objectStorageName}/versioning

- **Summary**: Get versioning status of an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 2
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/resources/objectStorage/{objectStorageName}/versioning",
  "summary": "Get versioning status of an object storage (bucket)",
  "description": "Get versioning status of an object storage (bucket)\\n\\n**Important Notes:**\\n- The actual response will be XML format with root element `VersioningConfiguration`\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<VersioningConfiguration xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\">\\n<Status>Enabled</Status>\\n</VersioningConfiguration>\\n```\\n",
  "operationId": "GetObjectStorageVersioning",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### GET /resources/objectStorage/{objectStorageName}

- **Summary**: Get details of an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 2
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "GET",
  "path": "/resources/objectStorage/{objectStorageName}",
  "summary": "Get details of an object storage (bucket)",
  "description": "Get details of an object storage (bucket)\\n\\n**Important Notes:**\\n- The actual response will be XML format with root element `ListBucketResult`\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<ListBucketResult xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\">\\n<Name>spider-test-bucket</Name>\\n<Prefix></Prefix>\\n<Marker></Marker>\\n<MaxKeys>1000</MaxKeys>\\n<IsTruncated>false</IsTruncated>\\n</ListBucketResult>\\n```\\n",
  "operationId": "GetObjectStorage",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### DELETE /resources/objectStorage/{objectStorageName}/{objectKey}

- **Summary**: Delete an object from a bucket
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 3
- **Request Body**: No
- **Response Codes**: 204

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "DELETE",
  "path": "/resources/objectStorage/{objectStorageName}/{objectKey}",
  "summary": "Delete an object from a bucket",
  "description": "Delete an object from a bucket",
  "operationId": "DeleteDataObject",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "test-object.txt",
      "description": "Object Name",
      "name": "objectKey",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    }
  ],
  "responses": [
    "204"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### POST /resources/objectStorage/{objectStorageName}

- **Summary**: **Delete** multiple objects from a bucket
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 4
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "POST",
  "path": "/resources/objectStorage/{objectStorageName}",
  "summary": "**Delete** multiple objects from a bucket",
  "description": "`Delete` multiple objects from a bucket\\n\\n**Important Notes:**\\n- The request body must contain the list of objects to delete in XML format\\n- The `delete` query parameter must be set to `true`\\n\\n**Request Body Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<Delete xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\">\\n<Object>\\n<Key>test-object1.txt</Key>\\n</Object>\\n<Object>\\n<Key>test-object2.txt</Key>\\n</Object>\\n</Delete>\\n```\\n\\n\\n**Actual XML Response Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"... (truncated)",
  "operationId": "DeleteMultipleDataObjects",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "boolean",
      "default": true,
      "description": "Delete",
      "name": "delete",
      "in": "query",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    },
    {
      "description": "List of objects to delete",
      "name": "reqBody",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/resource.Delete"
      }
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

### PUT /resources/objectStorage/{objectStorageName}/versioning

- **Summary**: Set versioning status of an object storage (bucket)
- **Tags**: [Resource] Object Storage Management
- **Parameters**: 3
- **Request Body**: No
- **Response Codes**: 200

<details>
<summary>🔍 View New Endpoint Details</summary>

```json
{
  "method": "PUT",
  "path": "/resources/objectStorage/{objectStorageName}/versioning",
  "summary": "Set versioning status of an object storage (bucket)",
  "description": "Set versioning status of an object storage (bucket)\\n\\n**Important Notes:**\\n- The request body must be XML format with root element `VersioningConfiguration`\\n- The `Status` field can be either `Enabled` or `Suspended`\\n\\n**Request Body Example:**\\n```\\n\\nxml\\n<?xml version=\\\"1.0\\\" encoding=\\\"UTF-8\\\"?>\\n<VersioningConfiguration xmlns=\\\"http://s3.amazonaws.com/doc/2006-03-01/\\\">\\n<Status>Enabled</Status>\\n</VersioningConfiguration>\\n```\\n",
  "operationId": "SetObjectStorageVersioning",
  "parameters": [
    {
      "type": "string",
      "default": "globally-unique-bucket-hctdx3",
      "description": "Object Storage Name",
      "name": "objectStorageName",
      "in": "path",
      "required": true
    },
    {
      "type": "string",
      "default": "aws-ap-northeast-2",
      "description": "This represents a credential or an access key ID. The required format is `{csp-region}` (i.e., the connection name).",
      "name": "credential",
      "in": "header",
      "required": true
    },
    {
      "description": "Versioning Configuration",
      "name": "reqBody",
      "in": "body",
      "required": true,
      "schema": {
        "$ref": "#/definitions/resource.VersioningConfiguration"
      }
    }
  ],
  "responses": [
    "200"
  ],
  "tags": [
    "[Resource] Object Storage Management"
  ]
}
```
</details>

---

## 🆕 New API Categories

- `[MC-Infra] MCI Remote Command Status`
- `[Resource] Object Storage Management`

## 🚀 Migration Guide

### ⚠️ Breaking Changes Summary

1. **Removed Endpoints**: 1 endpoints no longer available
2. **Endpoint Changes**: 4 endpoints with breaking changes
3. **Modified Endpoints**: 0 endpoints with breaking changes

### 📋 Migration Steps

1. **Review Endpoint Changes**: Check each changed endpoint's detailed modifications above
2. **Update Client Code**: Adapt to path/method/parameter/request body changes
3. **Update Path References**: Change old paths to new paths where applicable
4. **Update HTTP Methods**: Change HTTP methods where endpoints switched methods
5. **Remove Deprecated Calls**: Stop using removed endpoints
6. **Test Thoroughly**: Validate all API integrations

## 💡 Recommendations

- **Explore New Features**: 25 new endpoints available
- **Priority Testing**: Focus testing on 8 modified endpoints
- **Staged Migration**: Consider gradual migration to minimize risk
- **Backup Plan**: Keep old version available during transition
