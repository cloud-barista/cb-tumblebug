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

// Package mci is to handle REST API for mci
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

// RestPostMci godoc
// @ID PostMci
// @Summary Create MCI (Multi-Cloud Infrastructure)
// @Description Create MCI with detailed VM specifications and resource configuration.
// @Description This endpoint creates a complete multi-cloud infrastructure by:
// @Description 1. **VM Provisioning**: Creates VMs across multiple cloud providers using predefined specs and images
// @Description 2. **Resource Management**: Automatically handles VPC/VNet, security groups, SSH keys, and network configuration
// @Description 3. **Status Tracking**: Monitors VM creation progress and handles failures based on policy settings
// @Description 4. **Post-Deployment**: Optionally installs monitoring agents and executes custom commands
// @Description
// @Description **Key Features:**
// @Description - Multi-cloud VM deployment with heterogeneous configurations
// @Description - Automatic resource dependency management (VPC → Security Group → VM)
// @Description - Built-in failure handling with configurable policies (continue/rollback/refine)
// @Description - Optional CB-Dragonfly monitoring agent installation
// @Description - Post-deployment command execution support
// @Description - Real-time status updates and progress tracking
// @Description
// @Description **VM Lifecycle:**
// @Description 1. Creating → Running (successful deployment)
// @Description 2. Creating → Failed (deployment error, handled by failure policy)
// @Description 3. Running → Terminated (manual or policy-driven cleanup)
// @Description
// @Description **Failure Policies:**
// @Description - `continue`: Keep successful VMs, mark failed ones for later refinement
// @Description - `rollback`: Delete entire MCI if any VM fails (all-or-nothing)
// @Description - `refine`: Automatically clean up failed VMs, keep successful ones
// @Description
// @Description **Resource Requirements:**
// @Description - Valid VM specifications (must exist in system namespace)
// @Description - Valid images (must be available in target CSP regions)
// @Description - Sufficient CSP quotas and permissions
// @Description - Network connectivity between components
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID for resource isolation" default(default)
// @Param mciReq body model.MciReq true "MCI creation request with VM specifications, networking, and deployment options"
// @Success 200 {object} model.MciInfo "Created MCI information with VM details, status, and resource mapping"
// @Failure 400 {object} model.SimpleMsg "Invalid request parameters or missing required fields"
// @Failure 404 {object} model.SimpleMsg "Namespace not found or specified resources unavailable"
// @Failure 409 {object} model.SimpleMsg "MCI name already exists in namespace"
// @Failure 500 {object} model.SimpleMsg "Internal server error during MCI creation or CSP communication failure"
// @Router /ns/{nsId}/mci [post]
func RestPostMci(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.MciReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	option := "create"
	result, err := infra.CreateMci(nsId, req, option, false)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostRegisterCSPNativeVM godoc
// @ID PostRegisterCSPNativeVM
// @Summary Register Existing CSP VMs into Cloud-Barista MCI
// @Description Import and register pre-existing virtual machines from cloud service providers into CB-Tumblebug management.
// @Description This endpoint allows you to bring existing CSP resources under CB-Tumblebug control without recreating them:
// @Description
// @Description **Registration Process:**
// @Description 1. **Discovery**: Validates that the specified VM exists in the target CSP
// @Description 2. **Metadata Import**: Retrieves VM configuration, network settings, and current status
// @Description 3. **Resource Mapping**: Creates CB-Tumblebug resource objects that reference the existing CSP resources
// @Description 4. **Status Synchronization**: Aligns CB-Tumblebug status with actual CSP VM state
// @Description 5. **Management Integration**: Enables CB-Tumblebug operations on the registered VMs
// @Description
// @Description **Supported VM States:**
// @Description - Running VMs (most common use case)
// @Description - Stopped VMs (will be registered with current state)
// @Description - VMs with attached storage and network interfaces
// @Description
// @Description **Resource Compatibility:**
// @Description - VM must exist in a supported CSP (AWS, Azure, GCP, etc.)
// @Description - Network resources (VPC, subnets, security groups) will be discovered and mapped
// @Description - Storage volumes and attached disks will be registered automatically
// @Description - SSH keys and security configurations will be imported
// @Description
// @Description **Post-Registration Capabilities:**
// @Description - Standard CB-Tumblebug VM lifecycle operations (start, stop, terminate)
// @Description - Monitoring agent installation (if CB-Dragonfly is configured)
// @Description - Command execution and automation
// @Description - Integration with other CB-Tumblebug MCIs
// @Description
// @Description **Important Notes:**
// @Description - Registration does not modify the existing VM configuration
// @Description - Original CSP billing and resource management still applies
// @Description - CB-Tumblebug provides additional management layer and automation
// @Description - Ensure proper CSP credentials and permissions are configured
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID for organizing registered resources" default(default)
// @Param mciReq body model.MciReq true "MCI registration request containing existing CSP VM IDs and connection details"
// @Success 200 {object} model.MciInfo "Registered MCI information with imported VM details and current status"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or missing required CSP VM identifiers"
// @Failure 404 {object} model.SimpleMsg "Specified VMs not found in target CSP or namespace doesn't exist"
// @Failure 409 {object} model.SimpleMsg "VM already registered or MCI name conflicts"
// @Failure 500 {object} model.SimpleMsg "CSP communication error or registration process failure"
// @Router /ns/{nsId}/registerCspVm [post]
func RestPostRegisterCSPNativeVM(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &model.MciReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	option := "register"
	result, err := infra.CreateMci(nsId, req, option, false)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostSystemMci godoc
// @ID PostSystemMci
// @Summary Create System MCI for CB-Tumblebug Internal Operations
// @Description Create specialized MCI instances for CB-Tumblebug system operations and infrastructure probing.
// @Description This endpoint provisions system-level infrastructure that supports CB-Tumblebug's internal functions:
// @Description
// @Description **System MCI Types:**
// @Description - `probe`: Creates lightweight VMs for network connectivity testing and CSP capability discovery
// @Description - `monitor`: Deploys monitoring infrastructure for system health and performance tracking
// @Description - `test`: Provisions test environments for validating CSP integrations and features
// @Description
// @Description **Probe MCI Features:**
// @Description - **Connectivity Testing**: Validates network paths between different CSP regions
// @Description - **Latency Measurement**: Measures inter-region and inter-provider network performance
// @Description - **Feature Discovery**: Tests CSP-specific capabilities and service availability
// @Description - **Resource Validation**: Verifies that CB-Tumblebug can successfully provision resources
// @Description
// @Description **System Namespace:**
// @Description - All system MCIs are created in the special `system` namespace
// @Description - Isolated from user workloads and regular MCI operations
// @Description - Managed automatically by CB-Tumblebug internal processes
// @Description - May be used for background maintenance and monitoring tasks
// @Description
// @Description **Automatic Configuration:**
// @Description - Uses optimized VM specifications for system tasks (typically minimal resources)
// @Description - Automatically selects appropriate regions and providers based on probe requirements
// @Description - Configures necessary network access and security policies
// @Description - Deploys with minimal attack surface and security hardening
// @Description
// @Description **Lifecycle Management:**
// @Description - System MCIs may be automatically created, updated, or destroyed by CB-Tumblebug
// @Description - Typically short-lived for specific system tasks
// @Description - Resource cleanup is handled automatically
// @Description - Status and results are logged for system administrators
// @Description
// @Description **Use Cases:**
// @Description - Infrastructure health checks and validation
// @Description - Performance benchmarking across cloud providers
// @Description - Automated testing of new CSP integrations
// @Description - Network topology discovery and optimization
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param option query string false "System MCI type: 'probe' for connectivity testing, 'monitor' for system monitoring" Enums(probe,monitor,test)
// @Param mciReq body model.MciDynamicReq false "Optional MCI configuration. If not provided, system defaults will be used"
// @Success 200 {object} model.MciInfo "Created system MCI with specialized configuration and status"
// @Failure 400 {object} model.SimpleMsg "Invalid system option or malformed request"
// @Failure 403 {object} model.SimpleMsg "Insufficient permissions for system MCI creation"
// @Failure 500 {object} model.SimpleMsg "System MCI creation failed or CSP integration error"
// @Router /systemMci [post]
func RestPostSystemMci(c echo.Context) error {

	option := c.QueryParam("option")

	req := &model.MciDynamicReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateSystemMciDynamic(option)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciDynamic godoc
// @ID PostMciDynamic
// @Summary Create MCI Dynamically with Intelligent Resource Selection
// @Description Create multi-cloud infrastructure dynamically using common specifications and images with automatic resource discovery and optimization.
// @Description This is the **recommended approach** for MCI creation, providing simplified configuration with powerful automation:
// @Description
// @Description **Dynamic Resource Creation:**
// @Description 1. **Automatic Resource Discovery**: Validates and selects optimal VM specifications and images from common namespace
// @Description 2. **Intelligent Network Setup**: Creates VNets, subnets, security groups, and SSH keys automatically per provider
// @Description 3. **Cross-Cloud Orchestration**: Coordinates VM provisioning across multiple cloud providers simultaneously
// @Description 4. **Dependency Management**: Handles resource creation order and inter-dependencies automatically
// @Description 5. **Failure Recovery**: Implements configurable failure policies for robust deployment
// @Description
// @Description **Key Advantages Over Static MCI:**
// @Description - **Simplified Configuration**: Use common spec/image IDs instead of provider-specific resources
// @Description - **Automatic Resource Management**: No need to pre-create VNets, security groups, or SSH keys
// @Description - **Multi-Cloud Optimization**: Intelligent placement and configuration across providers
// @Description - **Built-in Best Practices**: Security groups, network isolation, and access controls applied automatically
// @Description - **Scalable Architecture**: Supports large-scale deployments with optimized resource utilization
// @Description
// @Description **Configuration Process:**
// @Description 1. **Resource Discovery**: Use `/recommendSpec` to find suitable VM specifications
// @Description 2. **Image Selection**: Use system namespace to discover compatible images
// @Description 3. **Request Validation**: Use `/mciDynamicCheckRequest` to validate configuration before deployment
// @Description 4. **Optional Preview**: Use `/mciDynamicReview` to estimate costs and review configuration
// @Description 5. **Deployment**: Submit MCI dynamic request with failure policy and deployment options
// @Description
// @Description **Failure Policies (PolicyOnPartialFailure):**
// @Description - **`continue`** (default): Create MCI with successful VMs, failed VMs remain for manual refinement
// @Description - **`rollback`**: Delete entire MCI if any VM fails (all-or-nothing deployment)
// @Description - **`refine`**: Automatically clean up failed VMs, keep successful ones (recommended for large deployments)
// @Description
// @Description **Deployment Options:**
// @Description - **`hold`**: Create MCI object but hold VM provisioning for manual approval
// @Description - **Normal**: Proceed with immediate VM provisioning after resource creation
// @Description
// @Description **Multi-Cloud Example Configuration:**
// @Description ```json
// @Description {
// @Description   "name": "multi-cloud-web-tier",
// @Description   "description": "Web application across AWS, Azure, and GCP",
// @Description   "policyOnPartialFailure": "refine",
// @Description   "vm": [
// @Description     {
// @Description       "name": "aws-web-servers",
// @Description       "subGroupSize": "3",
// @Description       "specId": "aws+us-east-1+t3.medium",
// @Description       "imageId": "ami-0abcdef1234567890",
// @Description       "rootDiskSize": "100",
// @Description       "label": {"tier": "web", "provider": "aws"}
// @Description     },
// @Description     {
// @Description       "name": "azure-api-servers",
// @Description       "subGroupSize": "2",
// @Description       "specId": "azure+eastus+Standard_B2s",
// @Description       "imageId": "Canonical:0001-com-ubuntu-server-jammy:22_04-lts",
// @Description       "label": {"tier": "api", "provider": "azure"}
// @Description     }
// @Description   ]
// @Description }
// @Description ```
// @Description
// @Description **Performance Considerations:**
// @Description - VM provisioning occurs in parallel across providers
// @Description - Network resources are created concurrently where possible
// @Description - Large deployments (>10 VMs) automatically use optimized batching
// @Description - Built-in rate limiting prevents CSP API throttling
// @Description
// @Description **Monitoring and Post-Deployment:**
// @Description - Optional CB-Dragonfly monitoring agent installation
// @Description - Custom post-deployment command execution
// @Description - Real-time status tracking and progress updates
// @Description - Automatic resource labeling and metadata management
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID for resource organization and isolation" default(default)
// @Param mciReq body model.MciDynamicReq true "Dynamic MCI request with common specifications. Must include specId and imageId for each VM group. See description for detailed example."
// @Param option query string false "Deployment option: 'hold' to create MCI without immediate VM provisioning" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking and correlation across API calls"
// @Success 200 {object} model.MciInfo "Successfully created MCI with VM deployment status, resource mappings, and configuration details"
// @Failure 400 {object} model.SimpleMsg "Invalid request format, missing required fields, or unsupported configuration"
// @Failure 404 {object} model.SimpleMsg "Namespace not found, specified specs/images unavailable, or CSP resources inaccessible"
// @Failure 409 {object} model.SimpleMsg "MCI name already exists or resource naming conflicts detected"
// @Failure 500 {object} model.SimpleMsg "Internal deployment error, CSP API failures, or resource creation timeouts"
// @Router /ns/{nsId}/mciDynamic [post]
func RestPostMciDynamic(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.MciDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciDynamic(reqID, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to create MCI dynamically")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostMciDynamicReview godoc
// @ID PostMciDynamicReview
// @Summary Review and Validate MCI Dynamic Request
// @Description Review and validate MCI dynamic request comprehensively before actual provisioning.
// @Description This endpoint performs comprehensive validation of MCI dynamic creation requests without actually creating resources.
// @Description It checks resource availability, validates specifications and images, estimates costs, and provides detailed recommendations.
// @Description
// @Description **Key Features:**
// @Description - Validates all VM specifications and images against CSP availability
// @Description - Provides cost estimation (including partial estimates when some costs are unknown)
// @Description - Identifies potential configuration issues and warnings
// @Description - Recommends optimization strategies
// @Description - Shows provider and region distribution
// @Description - Non-invasive validation (no resources are created)
// @Description
// @Description **Review Status:**
// @Description - `Ready`: All VMs can be created successfully
// @Description - `Warning`: VMs can be created but with configuration warnings
// @Description - `Error`: Critical errors prevent MCI creation
// @Description
// @Description **Use Cases:**
// @Description - Pre-validation before expensive MCI creation
// @Description - Cost estimation and planning
// @Description - Configuration optimization
// @Description - Multi-cloud resource planning
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param mciReq body model.MciDynamicReq true "Request body to review MCI dynamic provisioning. Must include specId and imageId info of each VM request. Same format as /mciDynamic endpoint. (ex: {name: mci01, vm: [{imageId: aws+ap-northeast-2+ubuntu22.04, specId: aws+ap-northeast-2+t2.small}]})"
// @Param option query string false "Option for MCI creation review (same as actual creation)" Enums(hold)
// @Param x-request-id header string false "Custom request ID for tracking"
// @Success 200 {object} model.ReviewMciDynamicReqInfo "Comprehensive review result with validation status, cost estimation, and recommendations"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or parameters"
// @Failure 404 {object} model.SimpleMsg "Namespace not found or invalid"
// @Failure 500 {object} model.SimpleMsg "Internal server error during validation"
// @Router /ns/{nsId}/mciDynamicReview [post]
func RestPostMciDynamicReview(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	req := &model.MciDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request for MCI dynamic review")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ReviewMciDynamicReq(reqID, nsId, req, option)
	if err != nil {
		log.Error().Err(err).Msg("failed to review MCI dynamic request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostMciVmDynamic godoc
// @ID PostMciVmDynamic
// @Summary Add VM Dynamically to Existing MCI
// @Description Dynamically add new virtual machines to an existing MCI using common specifications and automated resource management.
// @Description This endpoint provides elastic scaling capabilities for running MCIs:
// @Description
// @Description **Dynamic VM Addition Process:**
// @Description 1. **MCI Validation**: Verifies target MCI exists and is in a valid state for expansion
// @Description 2. **Resource Discovery**: Resolves common spec and image to provider-specific resources
// @Description 3. **Network Integration**: Automatically configures new VMs to use existing MCI network resources
// @Description 4. **Subgroup Management**: Creates new subgroups or expands existing ones based on configuration
// @Description 5. **Status Synchronization**: Updates MCI status and metadata to reflect new VM additions
// @Description
// @Description **Integration with Existing Infrastructure:**
// @Description - **Network Reuse**: New VMs automatically join existing VNets and security groups
// @Description - **SSH Key Sharing**: Uses existing SSH keys for consistent access management
// @Description - **Monitoring Integration**: New VMs inherit monitoring configuration from parent MCI
// @Description - **Label Propagation**: Applies MCI-level labels and policies to new VMs
// @Description - **Resource Consistency**: Maintains naming conventions and resource organization
// @Description
// @Description **Scaling Scenarios:**
// @Description - **Horizontal Scaling**: Add more instances to handle increased workload
// @Description - **Multi-Region Expansion**: Deploy VMs in new regions while maintaining MCI cohesion
// @Description - **Provider Diversification**: Add VMs from different cloud providers for redundancy
// @Description - **Workload Specialization**: Deploy VMs with different specifications for specific tasks
// @Description
// @Description **Configuration Requirements:**
// @Description - `specId`: Must specify valid VM specification from system namespace
// @Description - `imageId`: Must specify valid image compatible with target provider/region
// @Description - `name`: Becomes subgroup name; VMs will be named with sequential suffixes
// @Description - `subGroupSize`: Number of identical VMs to create (default: 1)
// @Description
// @Description **Network and Security:**
// @Description - New VMs automatically inherit security group rules from existing MCI
// @Description - Network connectivity to existing VMs is established automatically
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
// @Description - New VMs are immediately available for standard MCI operations
// @Description - Can be individually managed or grouped with existing subgroups
// @Description - Monitoring and logging are automatically configured
// @Description - Application deployment and configuration management can proceed immediately
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target MCI" default(default)
// @Param mciId path string true "MCI ID to which new VMs will be added" default(mci01)
// @Param vmReq body model.CreateSubGroupDynamicReq true "SubGroup dynamic request specifying specId, imageId, and scaling parameters"
// @Success 200 {object} model.MciInfo "Updated MCI information including newly added VMs and current status"
// @Failure 400 {object} model.SimpleMsg "Invalid VM request or incompatible configuration parameters"
// @Failure 404 {object} model.SimpleMsg "Target MCI not found or specified resources unavailable"
// @Failure 409 {object} model.SimpleMsg "Subgroup name conflicts or MCI in incompatible state"
// @Failure 500 {object} model.SimpleMsg "VM creation failed or network integration error"
// @Router /ns/{nsId}/mci/{mciId}/vmDynamic [post]
func RestPostMciVmDynamic(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	req := &model.CreateSubGroupDynamicReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateMciVmDynamic(nsId, mciId, req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciDynamicCheckRequest godoc
// @ID PostMciDynamicCheckRequest
// @Summary Check Resource Availability for Dynamic MCI Creation
// @Description Validate resource availability and discover optimal connection configurations before creating MCI dynamically.
// @Description This endpoint provides comprehensive resource validation and connection discovery for MCI planning:
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
// @Description - Pre-validate MCI configuration before expensive deployment operations
// @Description - Discover optimal provider/region combinations for cost or performance
// @Description - Troubleshoot resource availability issues during MCI planning
// @Description - Generate connection configuration templates for standardized deployments
// @Description - Assess infrastructure capacity and planning constraints
// @Description
// @Description **Integration Workflow:**
// @Description 1. Use this endpoint to validate and discover connection options
// @Description 2. Review recommendations and adjust specifications if needed
// @Description 3. Use `/mciDynamicReview` for detailed cost estimation and final validation
// @Description 4. Proceed with `/mciDynamic` using validated configuration
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param mciReq body model.MciConnectionConfigCandidatesReq true "Resource check request containing common specifications to validate"
// @Success 200 {object} model.CheckMciDynamicReqInfo "Resource availability matrix with connection candidates, provider capabilities, and optimization recommendations"
// @Failure 400 {object} model.SimpleMsg "Invalid request format or malformed specification identifiers"
// @Failure 404 {object} model.SimpleMsg "Specified common specifications not found in system namespace"
// @Failure 500 {object} model.SimpleMsg "CSP connectivity issues or internal validation service errors"
// @Router /mciDynamicCheckRequest [post]
func RestPostMciDynamicCheckRequest(c echo.Context) error {

	req := &model.MciConnectionConfigCandidatesReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CheckMciDynamicReq(req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciVm godoc
// @ID PostMciVm
// @Summary Add Homogeneous VM SubGroup to Existing MCI
// @Description Create and add a group of identical virtual machines (subgroup) to an existing MCI using detailed specifications.
// @Description This endpoint provides precise control over VM configuration and placement within existing infrastructure:
// @Description
// @Description **SubGroup Creation Process:**
// @Description 1. **MCI Integration**: Validates target MCI exists and can accommodate new VMs
// @Description 2. **Resource Validation**: Verifies all specified resources (specs, images, networks) exist and are accessible
// @Description 3. **Homogeneous Deployment**: Creates multiple identical VMs with consistent configuration
// @Description 4. **Network Integration**: Integrates new VMs with existing MCI networking and security policies
// @Description 5. **Group Management**: Establishes subgroup for collective management and operations
// @Description
// @Description **Detailed Configuration Control:**
// @Description - **Specific Resource References**: Uses exact resource IDs rather than common specifications
// @Description - **Network Placement**: Precise control over VNet, subnet, and security group assignment
// @Description - **Storage Configuration**: Detailed disk configuration including type, size, and performance tiers
// @Description - **Instance Customization**: Full control over VM specifications, images, and metadata
// @Description - **Security Settings**: Explicit security group and SSH key configuration
// @Description
// @Description **SubGroup Benefits:**
// @Description - **Collective Operations**: Perform operations on entire subgroup simultaneously
// @Description - **Homogeneous Scaling**: All VMs in subgroup share identical configuration
// @Description - **Simplified Management**: Single configuration template for multiple VMs
// @Description - **Consistent Naming**: Automatic sequential naming (e.g., web-1, web-2, web-3)
// @Description - **Group Policies**: Apply scaling, monitoring, and lifecycle policies at subgroup level
// @Description
// @Description **Use Cases:**
// @Description - **Application Tiers**: Deploy multiple instances of web servers, application servers, or databases
// @Description - **Load Distribution**: Create multiple identical VMs for load balancing scenarios
// @Description - **High Availability**: Deploy redundant instances across availability zones
// @Description - **Batch Processing**: Create worker nodes for distributed computing workloads
// @Description - **Development Environments**: Provision identical development or testing instances
// @Description
// @Description **Configuration Requirements:**
// @Description - **Resource IDs**: Must specify exact resource identifiers (not common specs)
// @Description - **Network Configuration**: VNet, subnet, and security group must exist and be compatible
// @Description - **SSH Keys**: Must specify valid SSH key pairs for access management
// @Description - **Image Compatibility**: Specified image must be available in target region
// @Description - **Quota Validation**: Sufficient CSP quotas must be available for all requested VMs
// @Description
// @Description **SubGroup Size Considerations:**
// @Description - **Small Groups (1-5 VMs)**: Fast deployment, minimal resource contention
// @Description - **Medium Groups (6-20 VMs)**: Optimized parallel deployment with resource batching
// @Description - **Large Groups (21+ VMs)**: Advanced deployment strategies to avoid CSP rate limits
// @Description - **Resource Limits**: Respects CSP quotas and CB-Tumblebug configuration limits
// @Description
// @Description **Post-Deployment Integration:**
// @Description - SubGroup becomes integral part of parent MCI
// @Description - All VMs inherit MCI-level monitoring and management policies
// @Description - Can be scaled out further or individual VMs can be managed separately
// @Description - Supports all standard CB-Tumblebug VM lifecycle operations
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target MCI" default(default)
// @Param mciId path string true "MCI ID to which the VM subgroup will be added" default(mci01)
// @Param vmReq body model.CreateSubGroupReq true "Detailed VM subgroup specification including exact resource IDs, networking, and scaling parameters"
// @Success 200 {object} model.MciInfo "Updated MCI information including newly created VM subgroup with individual VM details and status"
// @Failure 400 {object} model.SimpleMsg "Invalid VM request, missing required resources, or configuration conflicts"
// @Failure 404 {object} model.SimpleMsg "Target MCI not found, specified resources unavailable, or namespace inaccessible"
// @Failure 409 {object} model.SimpleMsg "SubGroup name conflicts, resource allocation conflicts, or MCI state incompatible with expansion"
// @Failure 500 {object} model.SimpleMsg "VM provisioning failed, network configuration error, or CSP API communication failure"
// @Router /ns/{nsId}/mci/{mciId}/vm [post]
func RestPostMciVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")

	vmInfoData := &model.CreateSubGroupReq{}
	if err := c.Bind(vmInfoData); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	result, err := infra.CreateMciGroupVm(nsId, mciId, vmInfoData, true)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostMciSubGroupScaleOut godoc
// @ID PostMciSubGroupScaleOut
// @Summary Scale Out Existing SubGroup in MCI
// @Description Horizontally scale an existing VM subgroup by adding more identical instances for increased capacity.
// @Description This endpoint provides elastic scaling capabilities for running application tiers:
// @Description
// @Description **Scale-Out Process:**
// @Description 1. **SubGroup Validation**: Verifies target subgroup exists and is in scalable state
// @Description 2. **Template Replication**: Uses existing VM configuration as template for new instances
// @Description 3. **Resource Allocation**: Ensures sufficient CSP quotas and network resources
// @Description 4. **Parallel Deployment**: Deploys multiple new VMs simultaneously for faster scaling
// @Description 5. **Integration**: Seamlessly integrates new VMs into existing subgroup and MCI
// @Description
// @Description **Configuration Inheritance:**
// @Description - **VM Specifications**: New VMs inherit exact specifications from existing subgroup members
// @Description - **Network Settings**: Automatically placed in same VNet, subnet, and security groups
// @Description - **SSH Keys**: Use same SSH key pairs for consistent access management
// @Description - **Monitoring**: Inherit monitoring agent configuration and policies
// @Description - **Labels and Metadata**: Propagate all labels and metadata from parent subgroup
// @Description
// @Description **Scaling Scenarios:**
// @Description - **Traffic Spikes**: Quickly add capacity during high-demand periods
// @Description - **Seasonal Scaling**: Scale out for predictable demand increases
// @Description - **Performance Optimization**: Add instances to reduce per-VM resource utilization
// @Description - **Geographic Expansion**: Scale existing workloads to handle broader user base
// @Description - **Fault Tolerance**: Increase redundancy by adding more instances
// @Description
// @Description **Intelligent Scaling:**
// @Description - **Sequential Naming**: New VMs follow established naming pattern (e.g., web-4, web-5, web-6)
// @Description - **Load Distribution**: New VMs are distributed optimally across availability zones
// @Description - **Resource Efficiency**: Reuses existing network and security infrastructure
// @Description - **Minimal Disruption**: Scaling occurs without affecting existing VM operations
// @Description - **Consistent Configuration**: Ensures all VMs in subgroup remain homogeneous
// @Description
// @Description **Operational Benefits:**
// @Description - **Zero Downtime**: Existing VMs continue running during scale-out operation
// @Description - **Immediate Availability**: New VMs are ready for traffic as soon as deployment completes
// @Description - **Unified Management**: All VMs (old and new) managed through single subgroup
// @Description - **Policy Consistency**: All scaling and management policies apply uniformly
// @Description - **Monitoring Integration**: New VMs automatically included in existing monitoring dashboards
// @Description
// @Description **Scale-Out Considerations:**
// @Description - **CSP Quotas**: Verifies sufficient instance, network, and storage quotas
// @Description - **Region Capacity**: Ensures target region has capacity for requested instance types
// @Description - **Network Limits**: Validates that VNet can accommodate additional VMs
// @Description - **Cost Impact**: Additional VMs incur proportional CSP billing costs
// @Description - **Application Readiness**: Applications should be designed to handle additional instances
// @Description
// @Description **Post-Scale Operations:**
// @Description - New VMs immediately participate in subgroup operations
// @Description - Can be individually managed while maintaining subgroup membership
// @Description - Support for further scaling operations (scale-out or scale-in)
// @Description - Ready for application deployment and load balancer integration
// @Description
// @Description **Best Practices:**
// @Description - Monitor application performance before and after scaling
// @Description - Ensure load balancers are configured to include new instances
// @Description - Verify application clustering and session management handle new instances
// @Description - Consider database connection limits and other resource constraints
// @Tags [MC-Infra] MCI Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID containing the target MCI and subgroup" default(default)
// @Param mciId path string true "MCI ID containing the subgroup to scale" default(mci01)
// @Param subgroupId path string true "SubGroup ID to scale out (must exist and contain at least one VM)" default(g1)
// @Param vmReq body model.ScaleOutSubGroupReq true "Scale-out request specifying the number of additional VMs to create"
// @Success 200 {object} model.MciInfo "Updated MCI information with scaled subgroup showing all VMs including newly added instances"
// @Failure 400 {object} model.SimpleMsg "Invalid scale-out request, insufficient quotas, or invalid VM count"
// @Failure 404 {object} model.SimpleMsg "Target MCI or subgroup not found, or namespace inaccessible"
// @Failure 409 {object} model.SimpleMsg "SubGroup in incompatible state for scaling or resource conflicts detected"
// @Failure 500 {object} model.SimpleMsg "VM provisioning failed, network configuration error, or CSP capacity limitations"
// @Router /ns/{nsId}/mci/{mciId}/subgroup/{subgroupId} [post]
func RestPostMciSubGroupScaleOut(c echo.Context) error {

	nsId := c.Param("nsId")
	mciId := c.Param("mciId")
	subgroupId := c.Param("subgroupId")

	scaleOutReq := &model.ScaleOutSubGroupReq{}
	if err := c.Bind(scaleOutReq); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.ScaleOutMciSubGroup(nsId, mciId, subgroupId, scaleOutReq.NumVMsToAdd)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetProvisioningLog godoc
// @ID GetProvisioningLog
// @Summary Get Provisioning History Log for VM Specification
// @Description Retrieve detailed provisioning history for a specific VM specification including success/failure patterns and risk analysis.
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
// @Description - **Pre-deployment Risk Assessment**: Check if a spec has historical failures before creating MCI
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
// @Tags [MC-Infra] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param specId path string true "VM Specification ID (format: provider+region+spec_name, e.g., aws+ap-northeast-2+t2.micro)"
// @Success 200 {object} model.ProvisioningLog "Provisioning history log with success/failure statistics and detailed analytics"
// @Success 204 "No provisioning history found for the specified VM specification"
// @Failure 400 {object} model.SimpleMsg "Invalid specification ID format or missing required parameters"
// @Failure 500 {object} model.SimpleMsg "Internal server error while retrieving provisioning history"
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
// @Description Remove all provisioning history data for a specific VM specification.
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
// @Description - MCI review process will not show historical warnings for this spec
// @Description - Provisioning reliability metrics will be reset to zero
// @Tags [MC-Infra] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param specId path string true "VM Specification ID to delete history for (format: provider+region+spec_name)"
// @Success 200 {object} model.SimpleMsg "Provisioning history successfully deleted"
// @Success 204 "No provisioning history found to delete"
// @Failure 400 {object} model.SimpleMsg "Invalid specification ID format"
// @Failure 500 {object} model.SimpleMsg "Internal server error while deleting provisioning history"
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
// @Description Evaluate the likelihood of provisioning failure based on historical data for a specific VM specification and image combination.
// @Description This endpoint provides intelligent risk assessment to help prevent deployment failures:
// @Description
// @Description **Risk Analysis Factors:**
// @Description - Historical failure rate for the VM specification
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
// @Description - Automatically called during MCI review process
// @Description - Can be used in CI/CD pipelines for deployment validation
// @Description - Helpful for capacity planning and resource selection
// @Tags [MC-Infra] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param specId path string true "VM Specification ID (format: provider+region+spec_name)"
// @Param cspImageName query string true "CSP-specific image name/ID to analyze compatibility"
// @Success 200 {object} object{riskLevel=string,riskMessage=string,analysis=object} "Risk analysis result with level, message, and detailed analysis"
// @Failure 400 {object} model.SimpleMsg "Invalid parameters or missing required query parameters"
// @Failure 500 {object} model.SimpleMsg "Internal server error during risk analysis"
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
// @Description Provides comprehensive risk analysis with separate assessments for VM specification and image risks, plus actionable recommendations.
// @Description This endpoint offers enhanced risk analysis by separating spec-level and image-level risk factors:
// @Description
// @Description **Risk Analysis Breakdown:**
// @Description - **Spec Risk**: Analyzes whether the VM specification itself has compatibility or resource issues
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
// @Description - Change VM specification (when spec is the primary risk factor)
// @Description - Try different image (when image is the primary risk factor)
// @Description - Monitor deployment closely (for new combinations or medium risk)
// @Description - Proceed with confidence (for low-risk combinations)
// @Tags [MCI] Provisioning History
// @Accept json
// @Produce json
// @Param specId query string true "VM specification ID (e.g., 'gcp+europe-north1+f1-micro')"
// @Param cspImageName query string true "CSP-specific image name (e.g., 'ami-0c02fb55956c7d316' for AWS)"
// @Success 200 {object} model.RiskAnalysis "Detailed risk analysis with spec, image, and overall risk assessments plus recommendations"
// @Failure 400 {object} model.SimpleMsg "Bad Request - Missing or invalid parameters"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
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
// @Description - Affects future MCI review recommendations
// @Description - Builds historical baseline for reliability metrics
// @Tags [MC-Infra] Provisioning History and Analytics
// @Accept  json
// @Produce  json
// @Param provisioningEvent body model.ProvisioningEvent true "Provisioning event details with success/failure information"
// @Success 200 {object} model.SimpleMsg "Provisioning event successfully recorded"
// @Failure 400 {object} model.SimpleMsg "Invalid event data or missing required fields"
// @Failure 500 {object} model.SimpleMsg "Internal server error while recording event"
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
