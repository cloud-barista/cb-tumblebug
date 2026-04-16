/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package infra is to handle REST API for infra
package infra

import (
	"fmt"
	"net/http"
	"time"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestPostInfra godoc
// @ID PostInfra
// @Summary Create Infra (Multi-Cloud Infrastructure)
// @Description Create Infra with detailed node specifications and resource configuration.
// @Description This endpoint creates a complete multi-cloud infrastructure by:
// @Description 1. **node Provisioning**: Creates nodes across multiple cloud providers using predefined specs and images
// @Description 2. **Resource Management**: Automatically handles VPC/VNet, security groups, SSH keys, and network configuration
// @Description 3. **Status Tracking**: Monitors node creation progress and handles failures based on policy settings
// @Description 4. **Post-Deployment**: Optionally installs monitoring agents and executes custom commands
// @Description
// @Description **Key Features:**
// @Description - Multi-cloud node deployment with heterogeneous configurations
// @Description - Automatic resource dependency management (VPC → Security Group → node)
// @Description - Built-in failure handling with configurable policies (continue/rollback/refine)
// @Description - Optional CB-Dragonfly monitoring agent installation
// @Description - Post-deployment command execution support
// @Description - Real-time status updates and progress tracking
// @Description
// @Description **node Lifecycle:**
// @Description 1. Creating → Running (successful deployment)
// @Description 2. Creating → Failed (deployment error, handled by failure policy)
// @Description 3. Running → Terminated (manual or policy-driven cleanup)
// @Description
// @Description **Failure Policies:**
// @Description - `continue`: Keep successful nodes, mark failed ones for later refinement
// @Description - `rollback`: Delete entire Infra if any node fails (all-or-nothing)
// @Description - `refine`: Automatically clean up failed nodes, keep successful ones
// @Description
// @Description **Resource Requirements:**
// @Description - Valid node specifications (must exist in system namespace)
// @Description - Valid images (must be available in target CSP regions)
// @Description - Sufficient CSP quotas and permissions
// @Description - Network connectivity between components
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID for resource isolation" default(default)
// @Param infraReq body model.InfraReq true "Infra creation request with node specifications, networking, and deployment options"
// @Success 200 {object} model.InfraInfo "Created Infra information with node details, status, and resource mapping"
// @Failure 400 {object} model.SimpleMsg "Invalid request parameters or missing required fields"
// @Failure 404 {object} model.SimpleMsg "Namespace not found or specified resources unavailable"
// @Failure 409 {object} model.SimpleMsg "Infra name already exists in namespace"
// @Failure 500 {object} model.SimpleMsg "Internal server error during Infra creation or CSP communication failure"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra [post]
func RestPostInfra(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")

	req := &model.InfraReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	option := "create"
	result, err := infra.CreateInfra(ctx, nsId, req, option, false)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostRegisterCSPNativeNode godoc
// @ID PostRegisterCSPNativeNode
// @Summary Register Existing CSP nodes into Cloud-Barista Infra
// @Description Import and register pre-existing virtual machines from cloud service providers into CB-Tumblebug management.
// @Description This endpoint allows you to bring existing CSP resources under CB-Tumblebug control without recreating them:
// @Description
// @Description **Registration Process:**
// @Description 1. **Discovery**: Validates that the specified node exists in the target CSP
// @Description 2. **Metadata Import**: Retrieves node configuration, network settings, and current status
// @Description 3. **Resource Mapping**: Creates CB-Tumblebug resource objects that reference the existing CSP resources
// @Description 4. **Status Synchronization**: Aligns CB-Tumblebug status with actual CSP node state
// @Description 5. **Management Integration**: Enables CB-Tumblebug operations on the registered nodes
// @Description
// @Description **Supported node States:**
// @Description - Running nodes (most common use case)
// @Description - Stopped nodes (will be registered with current state)
// @Description - nodes with attached storage and network interfaces
// @Description
// @Description **Resource Compatibility:**
// @Description - node must exist in a supported CSP (AWS, Azure, GCP, etc.)
// @Description - Network resources (VPC, subnets, security groups) will be discovered and mapped
// @Description - Storage volumes and attached disks will be registered automatically
// @Description - SSH keys and security configurations will be imported
// @Description
// @Description **Post-Registration Capabilities:**
// @Description - Standard CB-Tumblebug node lifecycle operations (start, stop, terminate)
// @Description - Monitoring agent installation (if CB-Dragonfly is configured)
// @Description - Command execution and automation
// @Description - Integration with other CB-Tumblebug Infras
// @Description
// @Description **Important Notes:**
// @Description - Registration does not modify the existing node configuration
// @Description - Original CSP billing and resource management still applies
// @Description - CB-Tumblebug provides additional management layer and automation
// @Description - Ensure proper CSP credentials and permissions are configured
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID for organizing registered resources" default(default)
// @Param infraReq body model.InfraReq true "Infra registration request containing existing CSP node IDs and connection details"
// @Success 200 {object} model.InfraInfo "Registered Infra information with imported node details and current status"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or missing required CSP node identifiers"
// @Failure 404 {object} model.SimpleMsg "Specified nodes not found in target CSP or namespace doesn't exist"
// @Failure 409 {object} model.SimpleMsg "node already registered or Infra name conflicts"
// @Failure 500 {object} model.SimpleMsg "CSP communication error or registration process failure"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/registerCspNode [post]
func RestPostRegisterCSPNativeNode(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")

	req := &model.InfraReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	option := "register"
	result, err := infra.CreateInfra(ctx, nsId, req, option, false)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostSystemInfra godoc
// @ID PostSystemInfra
// @Summary Create System Infra for CB-Tumblebug Internal Operations
// @Description Create specialized Infra instances for CB-Tumblebug system operations and infrastructure probing.
// @Description This endpoint provisions system-level infrastructure that supports CB-Tumblebug's internal functions:
// @Description
// @Description **System Infra Types:**
// @Description - `probe`: Creates lightweight nodes for network connectivity testing and CSP capability discovery
// @Description - `monitor`: Deploys monitoring infrastructure for system health and performance tracking
// @Description - `test`: Provisions test environments for validating CSP integrations and features
// @Description
// @Description **Probe Infra Features:**
// @Description - **Connectivity Testing**: Validates network paths between different CSP regions
// @Description - **Latency Measurement**: Measures inter-region and inter-provider network performance
// @Description - **Feature Discovery**: Tests CSP-specific capabilities and service availability
// @Description - **Resource Validation**: Verifies that CB-Tumblebug can successfully provision resources
// @Description
// @Description **System Namespace:**
// @Description - All system Infras are created in the special `system` namespace
// @Description - Isolated from user workloads and regular Infra operations
// @Description - Managed automatically by CB-Tumblebug internal processes
// @Description - May be used for background maintenance and monitoring tasks
// @Description
// @Description **Automatic Configuration:**
// @Description - Uses optimized node specifications for system tasks (typically minimal resources)
// @Description - Automatically selects appropriate regions and providers based on probe requirements
// @Description - Configures necessary network access and security policies
// @Description - Deploys with minimal attack surface and security hardening
// @Description
// @Description **Lifecycle Management:**
// @Description - System Infras may be automatically created, updated, or destroyed by CB-Tumblebug
// @Description - Typically short-lived for specific system tasks
// @Description - Resource cleanup is handled automatically
// @Description - Status and results are logged for system administrators
// @Description
// @Description **Use Cases:**
// @Description - Infrastructure health checks and validation
// @Description - Performance benchmarking across cloud providers
// @Description - Automated testing of new CSP integrations
// @Description - Network topology discovery and optimization
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param option query string false "System Infra type: 'probe' for connectivity testing, 'monitor' for system monitoring" Enums(probe,monitor,test)
// @Param infraReq body model.InfraDynamicReq false "Optional Infra configuration. If not provided, system defaults will be used"
// @Success 200 {object} model.InfraInfo "Created system Infra with specialized configuration and status"
// @Failure 400 {object} model.SimpleMsg "Invalid system option or malformed request"
// @Failure 403 {object} model.SimpleMsg "Insufficient permissions for system Infra creation"
// @Failure 500 {object} model.SimpleMsg "System Infra creation failed or CSP integration error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /systemInfra [post]
func RestPostSystemInfra(c echo.Context) error {

	option := c.QueryParam("option")

	req := &model.InfraDynamicReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateSystemInfraDynamic(option)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostInfraDynamic godoc
// @ID PostInfraDynamic
// @Summary Create Infra Dynamically with Intelligent Resource Selection
// @Description Create multi-cloud infrastructure dynamically using common specifications and images with automatic resource discovery and optimization.
// @Description This is the **recommended approach** for Infra creation, providing simplified configuration with powerful automation:
// @Description
// @Description **Dynamic Resource Creation:**
// @Description 1. **Automatic Resource Discovery**: Validates and selects optimal node specifications and images from common namespace
// @Description 2. **Intelligent Network Setup**: Creates VNets, subnets, security groups, and SSH keys automatically per provider
// @Description 3. **Cross-Cloud Orchestration**: Coordinates node provisioning across multiple cloud providers simultaneously
// @Description 4. **Dependency Management**: Handles resource creation order and inter-dependencies automatically
// @Description 5. **Failure Recovery**: Implements configurable failure policies for robust deployment
// @Description
// @Description **Key Advantages Over Static Infra:**
// @Description - **Simplified Configuration**: Use common spec/image IDs instead of provider-specific resources
// @Description - **Automatic Resource Management**: No need to pre-create VNets, security groups, or SSH keys
// @Description - **Multi-Cloud Optimization**: Intelligent placement and configuration across providers
// @Description - **Built-in Best Practices**: Security groups, network isolation, and access controls applied automatically
// @Description - **Scalable Architecture**: Supports large-scale deployments with optimized resource utilization
// @Description
// @Description **Configuration Process:**
// @Description 1. **Resource Discovery**: Use `/recommendSpec` to find suitable node specifications
// @Description 2. **Image Selection**: Use system namespace to discover compatible images
// @Description 3. **Request Validation**: Use `/infraDynamicCheckRequest` to validate configuration before deployment
// @Description 4. **Optional Preview**: Use `/infraDynamicReview` to estimate costs and review configuration
// @Description 5. **Deployment**: Submit Infra dynamic request with failure policy and deployment options
// @Description
// @Description **Failure Policies (PolicyOnPartialFailure):**
// @Description - **`continue`** (default): Create Infra with successful nodes, failed nodes remain for manual refinement
// @Description - **`rollback`**: Delete entire Infra if any node fails (all-or-nothing deployment)
// @Description - **`refine`**: Automatically clean up failed nodes, keep successful ones (recommended for large deployments)
// @Description
// @Description **Deployment Options:**
// @Description - **`hold`**: Create Infra object but hold node provisioning for manual approval
// @Description - **Normal**: Proceed with immediate node provisioning after resource creation
// @Description
// @Description **Multi-Cloud Example Configuration:**
// @Description ```json
// @Description {
// @Description   "name": "multi-cloud-web-tier",
// @Description   "description": "Web application across AWS, Azure, and GCP",
// @Description   "policyOnPartialFailure": "refine",
// @Description   "nodeGroups": [
// @Description     {
// @Description       "name": "aws-web-servers",
// @Description       "nodeGroupSize": "3",
// @Description       "specId": "aws+us-east-1+t3.medium",
// @Description       "imageId": "ami-0abcdef1234567890",
// @Description       "rootDiskSize": "100",
// @Description       "label": {"tier": "web", "provider": "aws"}
// @Description     },
// @Description     {
// @Description       "name": "azure-api-servers",
// @Description       "nodeGroupSize": "2",
// @Description       "specId": "azure+eastus+Standard_B2s",
// @Description       "imageId": "Canonical:0001-com-ubuntu-server-jammy:22_04-lts",
// @Description       "label": {"tier": "api", "provider": "azure"}
// @Description     }
// @Description   ]
// @Description }
// @Description ```
// @Description
// @Description **Performance Considerations:**
// @Description - node provisioning occurs in parallel across providers
// @Description - Network resources are created concurrently where possible
// @Description - Large deployments (>10 nodes) automatically use optimized batching
// @Description - Built-in rate limiting prevents CSP API throttling
// @Description
// @Description **Monitoring and Post-Deployment:**
// @Description - Optional CB-Dragonfly monitoring agent installation
// @Description - Custom post-deployment command execution
// @Description - Real-time status tracking and progress updates
// @Description - Automatic resource labeling and metadata management
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID for resource organization and isolation" default(default)
// @Param infraReq body model.InfraDynamicReq true "Dynamic Infra request with common specifications. Must include specId and imageId for each node group. See description for detailed example."
// @Param option query string false "Deployment option: 'hold' to create Infra without immediate node provisioning" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking and correlation across API calls"
// @Param x-credential-holder header string false "Credential holder ID to select which credentials to use for provisioning (default: system default holder)"
// @Success 200 {object} model.InfraInfo "Successfully created Infra with node deployment status, resource mappings, and configuration details"
// @Failure 400 {object} model.SimpleMsg "Invalid request format, missing required fields, or unsupported configuration"
// @Failure 404 {object} model.SimpleMsg "Namespace not found, specified specs/images unavailable, or CSP resources inaccessible"
// @Failure 409 {object} model.SimpleMsg "Infra name already exists or resource naming conflicts detected"
// @Failure 500 {object} model.SimpleMsg "Internal deployment error, CSP API failures, or resource creation timeouts"
// @Router /ns/{nsId}/infraDynamic [post]
func RestPostInfraDynamic(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.InfraDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateInfraDynamic(ctx, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create Infra dynamically")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostInfraDynamicReview godoc
// @ID PostInfraDynamicReview
// @Summary Review and Validate Infra Dynamic Request
// @Description Review and validate Infra dynamic request comprehensively before actual provisioning.
// @Description This endpoint performs comprehensive validation of Infra dynamic creation requests without actually creating resources.
// @Description It checks resource availability, validates specifications and images, estimates costs, and provides detailed recommendations.
// @Description
// @Description **Key Features:**
// @Description - Validates all node specifications and images against CSP availability
// @Description - Provides cost estimation (including partial estimates when some costs are unknown)
// @Description - Identifies potential configuration issues and warnings
// @Description - Recommends optimization strategies
// @Description - Shows provider and region distribution
// @Description - Non-invasive validation (no resources are created)
// @Description
// @Description **Review Status:**
// @Description - `Ready`: All nodes can be created successfully
// @Description - `Warning`: nodes can be created but with configuration warnings
// @Description - `Error`: Critical errors prevent Infra creation
// @Description
// @Description **Use Cases:**
// @Description - Pre-validation before expensive Infra creation
// @Description - Cost estimation and planning
// @Description - Configuration optimization
// @Description - Multi-cloud resource planning
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraReq body model.InfraDynamicReq true "Request body to review Infra dynamic provisioning. Must include specId and imageId info of each node request. Same format as /infraDynamic endpoint. (ex: {name: infra01, nodeGroups: [{imageId: aws+ap-northeast-2+ubuntu22.04, specId: aws+ap-northeast-2+t2.small}]})"
// @Param option query string false "Option for Infra creation review (same as actual creation)" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID to select which credentials to use for review (default: system default holder)"
// @Success 200 {object} model.ReviewInfraDynamicReqInfo "Comprehensive review result with validation status, cost estimation, and recommendations"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or parameters"
// @Failure 404 {object} model.SimpleMsg "Namespace not found or invalid"
// @Failure 500 {object} model.SimpleMsg "Internal server error during validation"
// @Router /ns/{nsId}/infraDynamicReview [post]
func RestPostInfraDynamicReview(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.InfraDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for Infra dynamic review")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ReviewInfraDynamicReq(ctx, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to review Infra dynamic request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostInfraNodeGroupDynamic godoc
// @ID PostInfraNodeGroupDynamic
// @Summary Add node Dynamically to Existing Infra
// @Description Dynamically add new virtual machines to an existing Infra using common specifications and automated resource management.
// @Description This endpoint provides elastic scaling capabilities for running Infras:
// @Description
// @Description **Dynamic node Addition Process:**
// @Description 1. **Infra Validation**: Verifies target Infra exists and is in a valid state for expansion
// @Description 2. **Resource Discovery**: Resolves common spec and image to provider-specific resources
// @Description 3. **Network Integration**: Automatically configures new nodes to use existing Infra network resources
// @Description 4. **NodeGroup Management**: Creates new nodegroups or expands existing ones based on configuration
// @Description 5. **Status Synchronization**: Updates Infra status and metadata to reflect new node additions
// @Description
// @Description **Integration with Existing Infrastructure:**
// @Description - **Network Reuse**: New nodes automatically join existing VNets and security groups
// @Description - **SSH Key Sharing**: Uses existing SSH keys for consistent access management
// @Description - **Monitoring Integration**: New nodes inherit monitoring configuration from parent Infra
// @Description - **Label Propagation**: Applies Infra-level labels and policies to new nodes
// @Description - **Resource Consistency**: Maintains naming conventions and resource organization
// @Description
// @Description **Scaling Scenarios:**
// @Description - **Horizontal Scaling**: Add more instances to handle increased workload
// @Description - **Multi-Region Expansion**: Deploy nodes in new regions while maintaining Infra cohesion
// @Description - **Provider Diversification**: Add nodes from different cloud providers for redundancy
// @Description - **Workload Specialization**: Deploy nodes with different specifications for specific tasks
// @Description
// @Description **Configuration Requirements:**
// @Description - `specId`: Must specify valid node specification from system namespace
// @Description - `imageId`: Must specify valid image compatible with target provider/region
// @Description - `name`: Becomes nodegroup name; nodes will be named with sequential suffixes
// @Description - `nodeGroupSize`: Number of identical nodes to create (default: 1)
// @Description
// @Description **Network and Security:**
// @Description - New nodes automatically inherit security group rules from existing Infra
// @Description - Network connectivity to existing nodes is established automatically
// @Description - Firewall rules and access policies are applied consistently
// @Description - SSH access is configured using existing key pairs
// @Description
// @Description **Example Use Cases:**
// @Description - Scale out web tier during traffic spikes
// @Description - Add GPU instances for machine learning workloads
// @Description - Deploy edge nodes in additional geographic regions
// @Description - Add specialized storage or database nodes to existing application stack
// @Description
// @Description **Post-Addition Operations:**
// @Description - New nodes are immediately available for standard Infra operations
// @Description - Can be individually managed or grouped with existing nodegroups
// @Description - Monitoring and logging are automatically configured
// @Description - Application deployment and configuration management can proceed immediately
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target Infra" default(default)
// @Param infraId path string true "Infra ID to which new nodes will be added" default(infra01)
// @Param nodeGroupReq body model.CreateNodeGroupDynamicReq true "NodeGroup dynamic request specifying specId, imageId, and scaling parameters"
// @Param x-credential-holder header string false "Credential holder ID to select which credentials to use for provisioning (default: system default holder)"
// @Success 200 {object} model.InfraInfo "Updated Infra information including newly added nodes and current status"
// @Failure 400 {object} model.SimpleMsg "Invalid node request or incompatible configuration parameters"
// @Failure 404 {object} model.SimpleMsg "Target Infra not found or specified resources unavailable"
// @Failure 409 {object} model.SimpleMsg "NodeGroup name conflicts or Infra in incompatible state"
// @Failure 500 {object} model.SimpleMsg "node creation failed or network integration error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/infra/{infraId}/nodeGroupDynamic [post]
func RestPostInfraNodeGroupDynamic(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.CreateNodeGroupDynamicReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateInfraNodeGroupDynamic(ctx, nsId, infraId, req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostInfraDynamicNodeGroupNodeReview godoc
// @ID PostInfraDynamicNodeGroupNodeReview
// @Summary Review node Dynamic Addition Request for Existing Infra
// @Description Review and validate a node dynamic addition request for an existing Infra before actual provisioning.
// @Description This endpoint provides comprehensive validation for adding new nodes to existing Infras without actually creating resources.
// @Description It checks resource availability, validates specifications and images, estimates costs, and provides detailed recommendations.
// @Description
// @Description **Key Features:**
// @Description - Validates node specification and image against CSP availability
// @Description - Checks compatibility with existing Infra configuration
// @Description - Provides cost estimation for the new node addition
// @Description - Identifies potential configuration issues and warnings
// @Description - Recommends optimization strategies
// @Description - Non-invasive validation (no resources are created)
// @Description
// @Description **Review Status:**
// @Description - `Ready`: node can be added successfully
// @Description - `Warning`: node can be added but with configuration warnings
// @Description - `Error`: Critical errors prevent node addition
// @Description
// @Description **Infra Integration Validation:**
// @Description - Ensures target Infra exists and is in a compatible state
// @Description - Validates network integration possibilities
// @Description - Checks resource naming conflicts
// @Description - Verifies security group and SSH key compatibility
// @Description
// @Description **Use Cases:**
// @Description - Pre-validation before expensive node addition operations
// @Description - Cost estimation for scaling decisions
// @Description - Configuration optimization before deployment
// @Description - Risk assessment for node addition to existing infrastructure
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target Infra" default(default)
// @Param infraId path string true "Infra ID to which the node will be added" default(infra01)
// @Param nodeGroupReq body model.CreateNodeGroupDynamicReq true "Request body to review node dynamic addition. Must include specId and imageId info. (ex: {name: web-servers, specId: aws+ap-northeast-2+t2.small, imageId: aws+ap-northeast-2+ubuntu22.04, nodeGroupSize: 2})"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID to select which credentials to use for review (default: system default holder)"
// @Success 200 {object} model.ReviewNodeGroupDynamicReqInfo "Comprehensive node addition review result with validation status, cost estimation, and recommendations"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or parameters"
// @Failure 404 {object} model.SimpleMsg "Target Infra not found or namespace not found"
// @Failure 500 {object} model.SimpleMsg "Internal server error during validation"
// @Router /ns/{nsId}/infra/{infraId}/nodeGroupDynamicReview [post]
func RestPostInfraDynamicNodeGroupNodeReview(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.CreateNodeGroupDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for Node dynamic addition review")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Validate that target Infra exists
	_, err := infra.GetInfraInfo(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msgf("target Infra not found: %s/%s", nsId, infraId)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ReviewSingleNodeGroupDynamicReq(ctx, nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to review Node dynamic addition request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostSpecImagePairReview godoc
// @ID PostSpecImagePairReview
// @Summary Review Spec and Image Pair Compatibility
// @Description Validate whether a spec and image pair is compatible for node provisioning.
// @Description This lightweight API checks:
// @Description - Spec availability in DB and CSP
// @Description - Image availability in DB and CSP (auto-registers if found in CSP but not in DB)
// @Description - Cost estimation based on spec
// @Description
// @Description **Use Cases:**
// @Description - Quick validation before node creation
// @Description - Pre-check for dynamic provisioning
// @Description - Verify custom image IDs entered by user
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param specImagePair body model.SpecImagePairReviewReq true "Spec and Image pair to review"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Success 200 {object} model.SpecImagePairReviewResult "Review result with validation status and details"
// @Failure 400 {object} model.SimpleMsg "Invalid request format"
// @Failure 500 {object} model.SimpleMsg "Internal server error"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /specImagePairReview [post]
func RestPostSpecImagePairReview(c echo.Context) error {
	ctx := c.Request().Context()

	req := &model.SpecImagePairReviewReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for spec-image pair review")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if req.SpecId == "" || req.ImageId == "" {
		err := fmt.Errorf("specId and imageId are required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ReviewSpecImagePair(ctx, req.SpecId, req.ImageId)
	if err != nil {
		log.Error().Err(err).Msg("failed to review spec-image pair")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostInfraDynamicCheckRequest godoc
// @ID PostInfraDynamicCheckRequest
// @Summary (Deprecated) Check Resource Availability for Dynamic Infra Creation
// @Description **⚠️ DEPRECATED: This endpoint is deprecated and will be removed in a future version. Please use `/infraDynamicReview` instead for comprehensive validation and cost estimation.**
// @Description
// @Description Validate resource availability and discover optimal connection configurations before creating Infra dynamically.
// @Description This endpoint provides comprehensive resource validation and connection discovery for Infra planning:
// @Description
// @Description **Resource Validation Process:**
// @Description 1. **Specification Analysis**: Validates that requested common specs exist and are accessible
// @Description 2. **Provider Discovery**: Identifies available cloud providers and regions for each specification
// @Description 3. **Connectivity Assessment**: Tests connection configurations and CSP API accessibility
// @Description 4. **Quota Verification**: Checks available quotas and resource limits where possible
// @Description 5. **Compatibility Matrix**: Generates matrix of viable spec-provider-region combinations
// @Description
// @Description **Connection Configuration Discovery:**
// @Description - **Available Providers**: Lists all configured cloud providers (AWS, Azure, GCP, etc.)
// @Description - **Active Regions**: Shows available regions per provider with connectivity status
// @Description - **Specification Mapping**: Maps common specs to provider-specific instance types
// @Description - **Image Compatibility**: Validates image availability across different providers/regions
// @Description - **Network Capabilities**: Identifies supported network features and configurations
// @Description
// @Description **Pre-Deployment Validation:**
// @Description - **Resource Existence**: Confirms all specified resources exist in system namespace
// @Description - **Permission Verification**: Validates CSP credentials and required permissions
// @Description - **API Connectivity**: Tests connection to CSP APIs and service endpoints
// @Description - **Dependency Resolution**: Identifies any missing dependencies or prerequisites
// @Description
// @Description **Optimization Recommendations:**
// @Description - **Cost-Effective Regions**: Suggests regions with lower pricing for specified resources
// @Description - **Performance Optimization**: Recommends regions with better network performance
// @Description - **Availability Zone**: Identifies optimal AZ distribution for high availability
// @Description - **Resource Bundling**: Suggests efficient resource combinations and groupings
// @Description
// @Description **Output Information:**
// @Description - **Connection Candidates**: List of viable connection configurations
// @Description - **Provider Capabilities**: Detailed capabilities matrix per provider
// @Description - **Resource Status**: Real-time availability status for each requested resource
// @Description - **Recommendation Summary**: Actionable recommendations for optimal deployment
// @Description
// @Description **Use Cases:**
// @Description - Pre-validate Infra configuration before expensive deployment operations
// @Description - Discover optimal provider/region combinations for cost or performance
// @Description - Troubleshoot resource availability issues during Infra planning
// @Description - Generate connection configuration templates for standardized deployments
// @Description - Assess infrastructure capacity and planning constraints
// @Description
// @Description **Integration Workflow:**
// @Description 1. Use this endpoint to validate and discover connection options
// @Description 2. Review recommendations and adjust specifications if needed
// @Description 3. Use `/infraDynamicReview` for detailed cost estimation and final validation
// @Description 4. Proceed with `/infraDynamic` using validated configuration
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param infraReq body model.InfraConnectionConfigCandidatesReq true "Resource check request containing common specifications to validate"
// @Success 200 {object} model.CheckInfraDynamicReqInfo "Resource availability matrix with connection candidates, provider capabilities, and optimization recommendations"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or malformed specification identifiers"
// @Failure 404 {object} model.SimpleMsg "Specified common specifications not found in system namespace"
// @Failure 500 {object} model.SimpleMsg "CSP connectivity issues or internal validation service errors"
// @Deprecated
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /infraDynamicCheckRequest [post]
func RestPostInfraDynamicCheckRequest(c echo.Context) error {
	ctx := c.Request().Context()

	req := &model.InfraConnectionConfigCandidatesReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CheckInfraDynamicReq(ctx, req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostInfraNode godoc
// @ID PostInfraNode
// @Summary Add Homogeneous node NodeGroup to Existing Infra
// @Description Create and add a group of identical virtual machines (nodegroup) to an existing Infra using detailed specifications.
// @Description This endpoint provides precise control over node configuration and placement within existing infrastructure:
// @Description
// @Description **NodeGroup Creation Process:**
// @Description 1. **Infra Integration**: Validates target Infra exists and can accommodate new nodes
// @Description 2. **Resource Validation**: Verifies all specified resources (specs, images, networks) exist and are accessible
// @Description 3. **Homogeneous Deployment**: Creates multiple identical nodes with consistent configuration
// @Description 4. **Network Integration**: Integrates new nodes with existing Infra networking and security policies
// @Description 5. **Group Management**: Establishes nodegroup for collective management and operations
// @Description
// @Description **Detailed Configuration Control:**
// @Description - **Specific Resource References**: Uses exact resource IDs rather than common specifications
// @Description - **Network Placement**: Precise control over VNet, subnet, and security group assignment
// @Description - **Storage Configuration**: Detailed disk configuration including type, size, and performance tiers
// @Description - **Instance Customization**: Full control over node specifications, images, and metadata
// @Description - **Security Settings**: Explicit security group and SSH key configuration
// @Description
// @Description **NodeGroup Benefits:**
// @Description - **Collective Operations**: Perform operations on entire nodegroup simultaneously
// @Description - **Homogeneous Scaling**: All nodes in nodegroup share identical configuration
// @Description - **Simplified Management**: Single configuration template for multiple nodes
// @Description - **Consistent Naming**: Automatic sequential naming (e.g., web-1, web-2, web-3)
// @Description - **Group Policies**: Apply scaling, monitoring, and lifecycle policies at nodegroup level
// @Description
// @Description **Use Cases:**
// @Description - **Application Tiers**: Deploy multiple instances of web servers, application servers, or databases
// @Description - **Load Distribution**: Create multiple identical nodes for load balancing scenarios
// @Description - **High Availability**: Deploy redundant instances across availability zones
// @Description - **Batch Processing**: Create worker nodes for distributed computing workloads
// @Description - **Development Environments**: Provision identical development or testing instances
// @Description
// @Description **Configuration Requirements:**
// @Description - **Resource IDs**: Must specify exact resource identifiers (not common specs)
// @Description - **Network Configuration**: VNet, subnet, and security group must exist and be compatible
// @Description - **SSH Keys**: Must specify valid SSH key pairs for access management
// @Description - **Image Compatibility**: Specified image must be available in target region
// @Description - **Quota Validation**: Sufficient CSP quotas must be available for all requested nodes
// @Description
// @Description **NodeGroup Size Considerations:**
// @Description - **Small Groups (1-5 nodes)**: Fast deployment, minimal resource contention
// @Description - **Medium Groups (6-20 nodes)**: Optimized parallel deployment with resource batching
// @Description - **Large Groups (21+ nodes)**: Advanced deployment strategies to avoid CSP rate limits
// @Description - **Resource Limits**: Respects CSP quotas and CB-Tumblebug configuration limits
// @Description
// @Description **Post-Deployment Integration:**
// @Description - NodeGroup becomes integral part of parent Infra
// @Description - All nodes inherit Infra-level monitoring and management policies
// @Description - Can be scaled out further or individual nodes can be managed separately
// @Description - Supports all standard CB-Tumblebug node lifecycle operations
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target Infra" default(default)
// @Param infraId path string true "Infra ID to which the node nodegroup will be added" default(infra01)
// @Param nodeGroupReq body model.CreateNodeGroupReq true "Detailed node nodegroup specification including exact resource IDs, networking, and scaling parameters"
// @Success 200 {object} model.InfraInfo "Updated Infra information including newly created node nodegroup with individual node details and status"
// @Failure 400 {object} model.SimpleMsg "Invalid node request, missing required resources, or configuration conflicts"
// @Failure 404 {object} model.SimpleMsg "Target Infra not found, specified resources unavailable, or namespace inaccessible"
// @Failure 409 {object} model.SimpleMsg "NodeGroup name conflicts, resource allocation conflicts, or Infra state incompatible with expansion"
// @Failure 500 {object} model.SimpleMsg "node provisioning failed, network configuration error, or CSP API communication failure"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/node [post]
func RestPostInfraNode(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	nodeInfoData := &model.CreateNodeGroupReq{}
	if err := c.Bind(nodeInfoData); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	result, err := infra.CreateInfraGroupNode(ctx, nsId, infraId, nodeInfoData, true)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostInfraNodeGroupScaleOut godoc
// @ID PostInfraNodeGroupScaleOut
// @Summary Scale Out Existing NodeGroup in Infra
// @Description Horizontally scale an existing node nodegroup by adding more identical instances for increased capacity.
// @Description This endpoint provides elastic scaling capabilities for running application tiers:
// @Description
// @Description **Scale-Out Process:**
// @Description 1. **NodeGroup Validation**: Verifies target nodegroup exists and is in scalable state
// @Description 2. **Template Replication**: Uses existing node configuration as template for new instances
// @Description 3. **Resource Allocation**: Ensures sufficient CSP quotas and network resources
// @Description 4. **Parallel Deployment**: Deploys multiple new nodes simultaneously for faster scaling
// @Description 5. **Integration**: Seamlessly integrates new nodes into existing nodegroup and Infra
// @Description
// @Description **Configuration Inheritance:**
// @Description - **node Specifications**: New nodes inherit exact specifications from existing nodegroup members
// @Description - **Network Settings**: Automatically placed in same VNet, subnet, and security groups
// @Description - **SSH Keys**: Use same SSH key pairs for consistent access management
// @Description - **Monitoring**: Inherit monitoring agent configuration and policies
// @Description - **Labels and Metadata**: Propagate all labels and metadata from parent nodegroup
// @Description
// @Description **Scaling Scenarios:**
// @Description - **Traffic Spikes**: Quickly add capacity during high-demand periods
// @Description - **Seasonal Scaling**: Scale out for predictable demand increases
// @Description - **Performance Optimization**: Add instances to reduce per-node resource utilization
// @Description - **Geographic Expansion**: Scale existing workloads to handle broader user base
// @Description - **Fault Tolerance**: Increase redundancy by adding more instances
// @Description
// @Description **Intelligent Scaling:**
// @Description - **Sequential Naming**: New nodes follow established naming pattern (e.g., web-4, web-5, web-6)
// @Description - **Load Distribution**: New nodes are distributed optimally across availability zones
// @Description - **Resource Efficiency**: Reuses existing network and security infrastructure
// @Description - **Minimal Disruption**: Scaling occurs without affecting existing node operations
// @Description - **Consistent Configuration**: Ensures all nodes in nodegroup remain homogeneous
// @Description
// @Description **Operational Benefits:**
// @Description - **Zero Downtime**: Existing nodes continue running during scale-out operation
// @Description - **Immediate Availability**: New nodes are ready for traffic as soon as deployment completes
// @Description - **Unified Management**: All nodes (old and new) managed through single nodegroup
// @Description - **Policy Consistency**: All scaling and management policies apply uniformly
// @Description - **Monitoring Integration**: New nodes automatically included in existing monitoring dashboards
// @Description
// @Description **Scale-Out Considerations:**
// @Description - **CSP Quotas**: Verifies sufficient instance, network, and storage quotas
// @Description - **Region Capacity**: Ensures target region has capacity for requested instance types
// @Description - **Network Limits**: Validates that VNet can accommodate additional nodes
// @Description - **Cost Impact**: Additional nodes incur proportional CSP billing costs
// @Description - **Application Readiness**: Applications should be designed to handle additional instances
// @Description
// @Description **Post-Scale Operations:**
// @Description - New nodes immediately participate in nodegroup operations
// @Description - Can be individually managed while maintaining nodegroup membership
// @Description - Support for further scaling operations (scale-out or scale-in)
// @Description - Ready for application deployment and load balancer integration
// @Description
// @Description **Best Practices:**
// @Description - Monitor application performance before and after scaling
// @Description - Ensure load balancers are configured to include new instances
// @Description - Verify application clustering and session management handle new instances
// @Description - Consider database connection limits and other resource constraints
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target Infra and nodegroup" default(default)
// @Param infraId path string true "Infra ID containing the nodegroup to scale" default(infra01)
// @Param nodegroupId path string true "NodeGroup ID to scale out (must exist and contain at least one node)" default(g1)
// @Param nodeGroupReq body model.ScaleOutNodeGroupReq true "Scale-out request specifying the number of additional nodes to create"
// @Success 200 {object} model.InfraInfo "Updated Infra information with scaled nodegroup showing all nodes including newly added instances"
// @Failure 400 {object} model.SimpleMsg "Invalid scale-out request, insufficient quotas, or invalid node count"
// @Failure 404 {object} model.SimpleMsg "Target Infra or nodegroup not found, or namespace inaccessible"
// @Failure 409 {object} model.SimpleMsg "NodeGroup in incompatible state for scaling or resource conflicts detected"
// @Failure 500 {object} model.SimpleMsg "node provisioning failed, network configuration error, or CSP capacity limitations"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nodegroup/{nodegroupId} [post]
func RestPostInfraNodeGroupScaleOut(c echo.Context) error {
	ctx := c.Request().Context()

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodegroupId := c.Param("nodegroupId")

	scaleOutReq := &model.ScaleOutNodeGroupReq{}
	if err := c.Bind(scaleOutReq); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ScaleOutInfraNodeGroup(ctx, nsId, infraId, nodegroupId, scaleOutReq.NumNodesToAdd)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetProvisioningLog godoc
// @ID GetProvisioningLog
// @Summary Get Provisioning History Log for node Specification
// @Description Retrieve detailed provisioning history for a specific node specification including success/failure patterns and risk analysis.
// @Description This endpoint provides comprehensive insights into provisioning reliability:
// @Description
// @Description **Historical Data Includes:**
// @Description - Success and failure counts with timestamps
// @Description - CSP-specific error messages and failure patterns
// @Description - Image compatibility tracking across different attempts
// @Description - Failure rate analysis and risk assessment
// @Description - Regional and provider-specific reliability metrics
// @Description
// @Description **Use Cases:**
// @Description - **Pre-deployment Risk Assessment**: Check if a spec has historical failures before creating Infra
// @Description - **Troubleshooting**: Analyze failure patterns to identify root causes
// @Description - **Capacity Planning**: Understand reliability patterns for different specs and regions
// @Description - **Cost Optimization**: Avoid specs with high failure rates that waste resources
// @Description
// @Description **Response Details:**
// @Description - `failureCount`: Total number of provisioning failures
// @Description - `successCount`: Number of successes (only tracked after failures occur)
// @Description - `failureImages`: List of CSP images that failed with this spec
// @Description - `successImages`: List of CSP images that succeeded with this spec
// @Description - `failureMessages`: Detailed error messages from CSP
// @Description - `lastUpdated`: Timestamp of most recent provisioning attempt
// @Tags [Admin] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param specId path string true "node Specification ID (format: provider+region+spec_name, e.g., aws+ap-northeast-2+t2.micro)"
// @Success 200 {object} model.ProvisioningLog "Provisioning history log with success/failure statistics and detailed analytics"
// @Success 204 "No provisioning history found for the specified node specification"
// @Failure 400 {object} model.SimpleMsg "Invalid specification ID format or missing required parameters"
// @Failure 500 {object} model.SimpleMsg "Internal server error while retrieving provisioning history"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /provisioning/log/{specId} [get]
func RestGetProvisioningLog(c echo.Context) error {
	specId := c.Param("specId")

	if specId == "" {
		return clientManager.EndRequestWithLog(c,
			echo.NewHTTPError(http.StatusBadRequest, "specId parameter is required"), nil)
	}

	result, err := infra.GetProvisioningLog(specId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if result == nil {
		// No provisioning history found - return 204 No Content
		return c.NoContent(http.StatusNoContent)
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestDeleteProvisioningLog godoc
// @ID DeleteProvisioningLog
// @Summary Delete Provisioning History Log
// @Description Remove all provisioning history data for a specific node specification.
// @Description This operation permanently deletes historical failure and success records:
// @Description
// @Description **Warning**: This action is irreversible and will remove:
// @Description - All failure and success statistics
// @Description - Historical error messages and troubleshooting data
// @Description - Risk analysis baseline for future deployments
// @Description - Failure pattern analysis data
// @Description
// @Description **When to Use:**
// @Description - **Data Cleanup**: Remove outdated or irrelevant provisioning history
// @Description - **Fresh Start**: Clear history after infrastructure changes that resolve previous issues
// @Description - **Privacy Compliance**: Remove logs containing sensitive error information
// @Description - **Storage Management**: Clean up logs to manage kvstore space
// @Description
// @Description **Impact on System:**
// @Description - Future risk analysis for this spec will have no historical baseline
// @Description - Infra review process will not show historical warnings for this spec
// @Description - Provisioning reliability metrics will be reset to zero
// @Tags [Admin] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param specId path string true "node Specification ID to delete history for (format: provider+region+spec_name)"
// @Success 200 {object} model.SimpleMsg "Provisioning history successfully deleted"
// @Success 204 "No provisioning history found to delete"
// @Failure 400 {object} model.SimpleMsg "Invalid specification ID format"
// @Failure 500 {object} model.SimpleMsg "Internal server error while deleting provisioning history"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /provisioning/log/{specId} [delete]
func RestDeleteProvisioningLog(c echo.Context) error {
	specId := c.Param("specId")

	if specId == "" {
		return clientManager.EndRequestWithLog(c,
			echo.NewHTTPError(http.StatusBadRequest, "specId parameter is required"), nil)
	}

	// Check if log exists first
	existingLog, err := infra.GetProvisioningLog(specId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	if existingLog == nil {
		// No log exists - return 204 No Content
		return c.NoContent(http.StatusNoContent)
	}

	// Delete the log
	err = infra.DeleteProvisioningLog(specId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{
		Message: "Provisioning history successfully deleted for spec: " + specId,
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestAnalyzeProvisioningRisk godoc
// @ID AnalyzeProvisioningRisk
// @Summary Analyze Provisioning Risk for Spec and Image Combination
// @Description Evaluate the likelihood of provisioning failure based on historical data for a specific node specification and image combination.
// @Description This endpoint provides intelligent risk assessment to help prevent deployment failures:
// @Description
// @Description **Risk Analysis Factors:**
// @Description - Historical failure rate for the node specification
// @Description - Image-specific compatibility with the spec
// @Description - Recent failure patterns and trends
// @Description - Cross-reference of spec+image combination success rates
// @Description
// @Description **Risk Levels:**
// @Description - `high`: Very likely to fail (>80% failure rate or image-specific failures)
// @Description - `medium`: Moderate risk (50-80% failure rate or mixed results)
// @Description - `low`: Low risk (<50% failure rate or no previous failures)
// @Description - `unknown`: Insufficient data for analysis
// @Description
// @Description **Recommended Actions by Risk Level:**
// @Description - **High Risk**: Consider alternative specs or images, verify CSP quotas and permissions
// @Description - **Medium Risk**: Proceed with caution, have backup plans ready
// @Description - **Low Risk**: Safe to proceed with normal deployment
// @Description
// @Description **Integration Points:**
// @Description - Automatically called during Infra review process
// @Description - Can be used in CI/CD pipelines for deployment validation
// @Description - Helpful for capacity planning and resource selection
// @Tags [Admin] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param specId path string true "node Specification ID (format: provider+region+spec_name)"
// @Param cspImageName query string true "CSP-specific image name/ID to analyze compatibility"
// @Success 200 {object} object{riskLevel=string,riskMessage=string,analysis=object} "Risk analysis result with level, message, and detailed analysis"
// @Failure 400 {object} model.SimpleMsg "Invalid parameters or missing required query parameters"
// @Failure 500 {object} model.SimpleMsg "Internal server error during risk analysis"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /provisioning/risk/{specId} [get]
func RestAnalyzeProvisioningRisk(c echo.Context) error {
	specId := c.Param("specId")
	cspImageName := c.QueryParam("cspImageName")

	if specId == "" {
		return clientManager.EndRequestWithLog(c,
			echo.NewHTTPError(http.StatusBadRequest, "specId parameter is required"), nil)
	}

	if cspImageName == "" {
		return clientManager.EndRequestWithLog(c,
			echo.NewHTTPError(http.StatusBadRequest, "cspImageName query parameter is required"), nil)
	}

	riskLevel, riskMessage, err := infra.AnalyzeProvisioningRisk(specId, cspImageName)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Get additional analysis data
	provisioningLog, _ := infra.GetProvisioningLog(specId)

	result := map[string]interface{}{
		"riskLevel":   riskLevel,
		"riskMessage": riskMessage,
		"analysis": map[string]interface{}{
			"specId":       specId,
			"cspImageName": cspImageName,
			"hasHistory":   provisioningLog != nil,
		},
	}

	if provisioningLog != nil {
		totalAttempts := provisioningLog.FailureCount + provisioningLog.SuccessCount
		failureRate := float64(0)
		if totalAttempts > 0 {
			failureRate = float64(provisioningLog.FailureCount) / float64(totalAttempts)
		}

		result["analysis"].(map[string]interface{})["failureCount"] = provisioningLog.FailureCount
		result["analysis"].(map[string]interface{})["successCount"] = provisioningLog.SuccessCount
		result["analysis"].(map[string]interface{})["totalAttempts"] = totalAttempts
		result["analysis"].(map[string]interface{})["failureRate"] = failureRate
		result["analysis"].(map[string]interface{})["lastUpdated"] = provisioningLog.LastUpdated
		result["analysis"].(map[string]interface{})["imageInFailureList"] = false
		result["analysis"].(map[string]interface{})["imageInSuccessList"] = false

		// Check if this specific image has history
		for _, img := range provisioningLog.FailureImages {
			if img == cspImageName {
				result["analysis"].(map[string]interface{})["imageInFailureList"] = true
				break
			}
		}

		for _, img := range provisioningLog.SuccessImages {
			if img == cspImageName {
				result["analysis"].(map[string]interface{})["imageInSuccessList"] = true
				break
			}
		}
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}

// RestAnalyzeProvisioningRiskDetailed godoc
// @ID AnalyzeProvisioningRiskDetailed
// @Summary Analyze Detailed Provisioning Risk with Spec and Image Breakdown
// @Description Provides comprehensive risk analysis with separate assessments for node specification and image risks, plus actionable recommendations.
// @Description This endpoint offers enhanced risk analysis by separating spec-level and image-level risk factors:
// @Description
// @Description **Risk Analysis Breakdown:**
// @Description - **Spec Risk**: Analyzes whether the node specification itself has compatibility or resource issues
// @Description - **Image Risk**: Evaluates the track record of the specific image with this spec
// @Description - **Overall Risk**: Combines both factors to determine the primary risk source
// @Description - **Recommendations**: Provides actionable guidance based on risk analysis
// @Description
// @Description **Spec Risk Factors:**
// @Description - Number of different images that failed with this spec (indicates spec-level issues)
// @Description - Overall failure rate across all images
// @Description - Success/failure ratio with various images
// @Description
// @Description **Image Risk Factors:**
// @Description - Previous success/failure history of this specific image with this spec
// @Description - Whether this is a new, untested combination
// @Description
// @Description **Recommendation Types:**
// @Description - Change node specification (when spec is the primary risk factor)
// @Description - Try different image (when image is the primary risk factor)
// @Description - Monitor deployment closely (for new combinations or medium risk)
// @Description - Proceed with confidence (for low-risk combinations)
// @Tags [Admin] Provisioning History and Analytics
// @Accept json
// @Produce json
// @Param specId query string true "node specification ID (e.g., 'gcp+europe-north1+f1-micro')"
// @Param cspImageName query string true "CSP-specific image name (e.g., 'ami-0c02fb55956c7d316' for AWS)"
// @Success 200 {object} model.RiskAnalysis "Detailed risk analysis with spec, image, and overall risk assessments plus recommendations"
// @Failure 400 {object} model.SimpleMsg "Bad Request - Missing or invalid parameters"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /tumblebug/provisioning/risk/detailed [get]
func RestAnalyzeProvisioningRiskDetailed(c echo.Context) error {
	// Get query parameters
	specId := c.QueryParam("specId")
	cspImageName := c.QueryParam("cspImageName")

	// Validate required parameters
	if specId == "" {
		err := fmt.Errorf("specId parameter is required")
		log.Error().Err(err).Msg("Missing required parameter")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	if cspImageName == "" {
		err := fmt.Errorf("cspImageName parameter is required")
		log.Error().Err(err).Msg("Missing required parameter")
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}

	log.Debug().Msgf("REST API - Analyzing detailed provisioning risk for spec: %s, image: %s", specId, cspImageName)

	// Analyze detailed provisioning risk
	riskAnalysis, err := infra.AnalyzeProvisioningRiskDetailed(specId, cspImageName)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to analyze detailed provisioning risk for spec: %s", specId)
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, riskAnalysis)
}

// RestRecordProvisioningEvent godoc
// @ID RecordProvisioningEvent
// @Summary Record Manual Provisioning Event
// @Description Manually record a provisioning success or failure event for historical tracking and analysis.
// @Description This endpoint allows external systems or manual processes to contribute to provisioning history:
// @Description
// @Description **Use Cases:**
// @Description - **External Provisioning Tools**: Record events from non-CB-Tumblebug provisioning systems
// @Description - **Manual Testing**: Log results from manual deployment tests
// @Description - **Migration**: Import historical data from other systems
// @Description - **Integration**: Connect with CI/CD pipelines for comprehensive tracking
// @Description
// @Description **Event Types:**
// @Description - **Success Events**: Only recorded if previous failures exist for the spec
// @Description - **Failure Events**: Always recorded to build failure pattern database
// @Description
// @Description **Data Quality:**
// @Description - Provide accurate timestamps for proper chronological analysis
// @Description - Include detailed error messages for failure events
// @Description - Use consistent spec ID and image name formats
// @Description
// @Description **Impact on System:**
// @Description - Contributes to risk analysis algorithms
// @Description - Affects future Infra review recommendations
// @Description - Builds historical baseline for reliability metrics
// @Tags [Admin] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param provisioningEvent body model.ProvisioningEvent true "Provisioning event details with success/failure information"
// @Success 200 {object} model.SimpleMsg "Provisioning event successfully recorded"
// @Failure 400 {object} model.SimpleMsg "Invalid event data or missing required fields"
// @Failure 500 {object} model.SimpleMsg "Internal server error while recording event"
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /provisioning/event [post]
func RestRecordProvisioningEvent(c echo.Context) error {
	req := &model.ProvisioningEvent{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Validate required fields
	if req.SpecId == "" {
		return clientManager.EndRequestWithLog(c,
			echo.NewHTTPError(http.StatusBadRequest, "specId is required"), nil)
	}

	if req.CspImageName == "" {
		return clientManager.EndRequestWithLog(c,
			echo.NewHTTPError(http.StatusBadRequest, "cspImageName is required"), nil)
	}

	// Set timestamp if not provided
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	err := infra.RecordProvisioningEvent(req)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := model.SimpleMsg{
		Message: "Provisioning event successfully recorded",
	}

	return clientManager.EndRequestWithLog(c, nil, result)
}
