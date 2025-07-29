# Information: This file is part of the Tumblebug MCP server implementation.
# Run with the following command:
# uv run --with fastmcp,requests fastmcp run --transport sse ./src/interface/mcp/tb-mcp.py:mcp
# this server will be exposed to the MCP interface at http://127.0.0.1:8000/sse by default.

# Configuration example in Claude Desktop
# Note that Claude Desktop does not fully support SSE transport yet. 
# So, the example utilizes mcp-remote (https://www.npmjs.com/package/mcp-remote).
# mcp-remote this not part of this project, you need to check https://github.com/geelen/mcp-remote for your security.
# {
#   "mcpServers": {
#     "tumblebug": {
#       "command": "npx",
#       "args": [
#         "mcp-remote",
#         "http://127.0.0.1:8000/sse"
#       ]
#     }
#   }
# }

# Configuration example in VS Code.
# Note that VS Code does support SSE transport directly.
# "servers": {
#   "tumblebug": {
#     "type": "sse",
#     "url": "http://127.0.0.1:8000/sse"
#   },
# }

# For testing, you can use the Model Context Protocol Inspector.
# https://modelcontextprotocol.io/docs/tools/inspector


import os
import requests
import json
import logging
from typing import Dict, List, Optional, Any
from mcp.server.fastmcp import FastMCP

# This server utilizes mcp.server.fastmcp (https://github.com/modelcontextprotocol/python-sdk)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Tumblebug API basic settings
TUMBLEBUG_API_BASE_URL = os.environ.get("TUMBLEBUG_API_BASE_URL", "http://localhost:1323/tumblebug")
TUMBLEBUG_USERNAME = os.environ.get("TUMBLEBUG_USERNAME", "default")
TUMBLEBUG_PASSWORD = os.environ.get("TUMBLEBUG_PASSWORD", "default")
host = os.environ.get("MCP_SERVER_HOST", "0.0.0.0") 
port = int(os.environ.get("MCP_SERVER_PORT", "8000"))

# Output startup information for debugging using logger instead of print
logger.info(f"Tumblebug API URL: {TUMBLEBUG_API_BASE_URL}")
logger.info(f"Username: {TUMBLEBUG_USERNAME}")
logger.info(f"Password configured: {'Yes' if TUMBLEBUG_PASSWORD else 'No'}")

# Initialize MCP server
mcp = FastMCP(name="cb-tumblebug", host=host, port=port)

# Helper function: API request wrapper
def api_request(method, endpoint, json_data=None, params=None, files=None, headers=None):
    url = f"{TUMBLEBUG_API_BASE_URL}{endpoint}"
    
    # Request configuration
    request_config = {
        "auth": (TUMBLEBUG_USERNAME, TUMBLEBUG_PASSWORD),
        "timeout": 600000  # 600 seconds (10 minutes) timeout
    }
    
    # Add parameters according to method
    if params:
        request_config["params"] = params
    if json_data and method.lower() in ["post", "put"]:
        request_config["json"] = json_data
    if files:
        request_config["files"] = files
    if headers:
        request_config["headers"] = headers
    
    logger.debug(f"Request: {method} {url}")
    if json_data and logger.isEnabledFor(logging.DEBUG):
        logger.debug(f"Request data: {json.dumps(json_data, indent=2, ensure_ascii=False)[:200]}...")
    
    try:
        if method.lower() == "get":
            response = requests.get(url, **request_config)
        elif method.lower() == "post":
            response = requests.post(url, **request_config)
        elif method.lower() == "put":
            response = requests.put(url, **request_config)
        elif method.lower() == "delete":
            response = requests.delete(url, **request_config)
        else:
            return {"error": f"Unsupported method: {method}"}
        
        logger.debug(f"Response status: {response.status_code}")
        
        response.raise_for_status()
        
        if response.text:
            try:
                return json.loads(response.text)
            except json.JSONDecodeError:
                logger.error(f"Failed to parse response as JSON: {response.text[:200]}")
                return {"error": "Invalid JSON response", "raw_response": response.text[:500]}
        else:
            return {"message": "Success (No content)"}
            
    except requests.RequestException as e:
        # Output error information for debugging
        logger.error(f"API request error: {str(e)}")
        if hasattr(e, 'response') and e.response is not None:
            logger.error(f"Status code: {e.response.status_code}")
            logger.error(f"Response text: {e.response.text[:200]}")
        
        error_response = None
        
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_response = json.loads(e.response.text)
            except Exception:
                error_response = {"message": e.response.text}
        
        if error_response:
            return {"error": error_response}
        else:
            return {"error": str(e)}

# Safely test the API only if in development mode
if os.environ.get("MCP_ENV") == "development":
    try:
        logger.info("Testing API connection...")
        test_result = api_request("GET", "/ns")
        logger.info("API test successful")
    except Exception as e:
        logger.error(f"API test failed: {str(e)}")

#####################################
# Namespace Management
#####################################

# Tool: Get all namespaces
@mcp.tool()
def get_namespaces() -> Dict:
    """Get list of namespaces"""
    result = api_request("GET", "/ns")
    if "output" in result:
        return {"namespaces": result["output"]}
    return result

# Tool: Get specific namespace
@mcp.tool()
def get_namespace(ns_id: str) -> Dict:
    """Get specific namespace"""
    return api_request("GET", f"/ns/{ns_id}")

# Tool: Create namespace
@mcp.tool()
def create_namespace(name: str, description: Optional[str] = None) -> Dict:
    """
    Create a new namespace
    
    Args:
        name: Name of the namespace to create
        description: Description of the namespace (optional)
    
    Returns:
        Created namespace information
    """
    data = {
        "name": name,
        "description": description or f"Namespace {name}"
    }
    return api_request("POST", "/ns", json_data=data)

# Tool: Update namespace
@mcp.tool()
def update_namespace(ns_id: str, description: str) -> Dict:
    """
    Update existing namespace information
    
    Args:
        ns_id: Namespace ID to update
        description: New namespace description
    
    Returns:
        Updated namespace information
    """
    data = {
        "description": description
    }
    return api_request("PUT", f"/ns/{ns_id}", json_data=data)

# Tool: Delete namespace
@mcp.tool()
def delete_namespace(ns_id: str) -> Dict:
    """
    Delete a specific namespace
    
    Args:
        ns_id: Namespace ID to delete
    
    Returns:
        Deletion result
    """
    return api_request("DELETE", f"/ns/{ns_id}")


#####################################
# Connection Management
#####################################

# Tool: Get all cloud connections
@mcp.tool()
def get_connections() -> Dict:
    """
    Get all registered cloud connections
    
    Returns:
        List of cloud connections
    """
    params = {
        "filterVerified": "true",
        "filterRegionRepresentative": "true"
    }
    return api_request("GET", "/connConfig", params=params)

# Tool: Get connections with options
@mcp.tool()
def get_connections_with_options(filter_verified: bool = True, filter_region_representative: bool = True) -> Dict:
    """
    Get all registered cloud connections with filtering options
    
    Args:
        filter_verified: Whether to filter by verified connections
        filter_region_representative: Whether to filter by representative regions
    
    Returns:
        List of cloud connections
    """
    params = {}
    if filter_verified:
        params["filterVerified"] = "true"
    if filter_region_representative:
        params["filterRegionRepresentative"] = "true"
    
    return api_request("GET", "/connConfig", params=params)

# Tool: Get specific cloud connection
@mcp.tool()
def get_connection(conn_config_name: str) -> Dict:
    """
    Get specific cloud connection
    
    Args:
        conn_config_name: Connection configuration name
    
    Returns:
        Cloud connection information
    """
    return api_request("GET", f"/connConfig/{conn_config_name}")

#####################################
# Resource Management
#####################################

# Tool: Get VNet resources for a specific namespace
@mcp.tool()
def get_vnets(ns_id: str) -> Dict:
    """
    Get VNet resources for a specific namespace. You can think of VNet as a virtual network or Virtual Private Cloud (VPC).
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        List of VNet resources
    """
    return api_request("GET", f"/ns/{ns_id}/resources/vNet")

# Tool: Get SecurityGroup resources for a specific namespace
@mcp.tool()
def get_security_groups(ns_id: str) -> Dict:
    """
    Get SecurityGroup resources for a specific namespace
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        List of SecurityGroup resources
    """
    return api_request("GET", f"/ns/{ns_id}/resources/securityGroup")

# Tool: Get SSHKey resources for a specific namespace
@mcp.tool()
def get_ssh_keys(ns_id: str) -> Dict:
    """
    Get SSHKey resources for a specific namespace
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        List of SSHKey resources
    """
    return api_request("GET", f"/ns/{ns_id}/resources/sshKey")


# Tool: Release resources
@mcp.tool()
def release_resources(ns_id: str) -> Dict:
    """
    Release all shared resources for a specific namespace
    This includes VNet, SecurityGroup, and SSHKey resources.
    This operation is irreversible and should be used with caution.
    In general, it is recommended to release resources after all related MCIs have been deleted.
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        Resource release result
    """
    return api_request("DELETE", f"/ns/{ns_id}/sharedResources")

# Tool: Resource overview
@mcp.tool()
def resource_overview() -> Dict:
    """
    Get overview of all resources from CSPs (Cloud Service Providers).
    This includes VNet, SecurityGroup, SSHKey, and other resources.
    This operation provides a summary of resources across all namespaces.
    This is not used in general operations. Will be used to check resources managed by CSPs's management console.
    
    Returns:
        Resource overview information
    """
    return api_request("GET", "/inspectResourcesOverview")

# # Tool: Register CSP resources
# @mcp.tool()
# def register_csp_resources(ns_id: str, mci_flag: str = "n") -> Dict:
#     """
#     Register CSP resources
    
#     Args:
#         ns_id: Namespace ID
#         mci_flag: MCI flag (y/n)
    
#     Returns:
#         Registration result
#     """
#     data = {
#         "mciName": "csp",
#         "nsId": ns_id
#     }
#     return api_request("POST", f"/registerCspResourcesAll?mciFlag={mci_flag}", json_data=data)

#####################################
# MCI Management (Multi-Cloud Infrastructure)
#####################################

# Tool: Get MCI list
@mcp.tool()
def get_mci_list(ns_id: str) -> Dict:
    """
    Get list of MCIs (Multi-Cloud Infrastructures) for a specific namespace.
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        List of MCIs
    """
    return api_request("GET", f"/ns/{ns_id}/mci?option=status")

# Tool: Get MCI list with options
@mcp.tool()
def get_mci_list_with_options(ns_id: str, option: str = "status") -> Dict:
    """
    Get list of MCIs (Multi-Cloud Infrastructures) for a specific namespace with options.
    With options, you can specify whether to filter by ID or status.
    Status infrormation is about VMs status in MCI.
    
    Args:
        ns_id: Namespace ID
        option: Query option (id or status)
    
    Returns:
        List of MCIs
    """
    if option not in ["id", "status"]:
        option = "status"
    return api_request("GET", f"/ns/{ns_id}/mci?option={option}")

# Tool: Get MCI details
@mcp.tool()
def get_mci(ns_id: str, mci_id: str) -> Dict:
    """
    Get details of a specific MCI
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
    
    Returns:
        MCI information
    """
    return api_request("GET", f"/ns/{ns_id}/mci/{mci_id}")

# Tool: Get MCI access information
@mcp.tool()
def get_mci_access_info(ns_id: str, mci_id: str, show_ssh_key: bool = False) -> Dict:
    """
    Get access information for an MCI.
    This includes SSH key information if requested.
    Needs to check user's opinion whether to show SSH key or not.
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        show_ssh_key: Whether to show SSH key
    
    Returns:
        MCI access information
    """
    option = "showSshKey" if show_ssh_key else "accessinfo"
    return api_request("GET", f"/ns/{ns_id}/mci/{mci_id}?option=accessinfo&accessInfoOption={option}")

# Tool: Get subgroups list
@mcp.tool()
def get_subgroups(ns_id: str, mci_id: str) -> Dict:
    """
    Get list of subgroups for a specific MCI
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
    
    Returns:
        List of subgroups
    """
    return api_request("GET", f"/ns/{ns_id}/mci/{mci_id}/subgroup")

# Tool: Get VM list
@mcp.tool()
def get_vms(ns_id: str, mci_id: str, subgroup_id: str) -> Dict:
    """
    Get list of VMs for a specific subgroup
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        subgroup_id: Subgroup ID
    
    Returns:
        List of VMs
    """
    return api_request("GET", f"/ns/{ns_id}/mci/{mci_id}/subgroup/{subgroup_id}")

# Tool: Get image search options
@mcp.tool()
def get_image_search_options(ns_id: str = "system") -> Dict:
    """
    Get all available options for image search fields.
    This provides example values for various search parameters that can be used in search_images().
    
    Use this function first to understand what search criteria are available,
    then use search_images() to find specific images based on your requirements.
    
    Args:
        ns_id: Namespace ID (typically "system" for system images)
    
    Returns:
        Available search options including:
        - osArchitecture: Available OS architectures (e.g., "x86_64", "arm64")
        - osType: Available OS types (e.g., "ubuntu 22.04", "centos 7", "windows server 2019")
        - providerName: Available cloud providers (e.g., "aws", "azure", "gcp")
        - regionName: Available regions (e.g., "ap-northeast-2", "us-east-1")
    """
    return api_request("GET", f"/ns/{ns_id}/resources/searchImageOptions")

# Tool: Search images
@mcp.tool()
def search_images(
    ns_id: str = "system",
    os_architecture: Optional[str] = None,
    os_type: Optional[str] = None,
    provider_name: Optional[str] = None,
    region_name: Optional[str] = None,
    guest_os: Optional[str] = None
) -> Dict:
    """
    Search for available images based on specific criteria.
    
    This is a critical function for MCI creation workflow:
    1. First call get_image_search_options() to see available search parameters
    2. Use this function to search for images matching your requirements
    3. From the results, identify the 'cspImageName' of your desired image
    4. Use the 'cspImageName' as 'commonImage' parameter in create_mci_dynamic()
    
    Example workflow:
    1. search_images(provider_name="aws", region_name="ap-northeast-2", os_type="ubuntu 22.04")
    2. From results, find an image and note its 'cspImageName' (e.g., "ami-0e06732ba3ca8c6cc")
    3. create_mci_dynamic(commonImage="ami-0e06732ba3ca8c6cc", ...)
    
    Args:
        ns_id: Namespace ID (typically "system" for system images)
        os_architecture: OS architecture filter (e.g., "x86_64", "arm64")
        os_type: OS type filter (e.g., "ubuntu 22.04", "centos 7", "windows server 2019")
        provider_name: Cloud provider filter (e.g., "aws", "azure", "gcp")
        region_name: Region filter (e.g., "ap-northeast-2", "us-east-1", "koreacentral")
        guest_os: Guest OS filter (alternative to os_type)
    
    Returns:
        Search results containing:
        - imageList: List of matching images
        - Each image includes:
          - cspImageName: CSP-specific image identifier (CRITICAL for MCI creation)
          - description: Image description
          - guestOS: Guest operating system
          - architecture: OS architecture
          - creationDate: Image creation date
          
    Important: The 'cspImageName' from search results becomes the 'commonImage' 
    parameter when creating MCIs via create_mci_dynamic().
    """
    data = {}
    
    # Build search criteria
    if os_architecture:
        data["osArchitecture"] = os_architecture
    if os_type:
        data["osType"] = os_type
    if provider_name:
        data["providerName"] = provider_name
    if region_name:
        data["regionName"] = region_name
    if guest_os:
        data["guestOS"] = guest_os
    
    return api_request("POST", f"/ns/{ns_id}/resources/searchImage", json_data=data)

# Tool: Recommend VM spec
@mcp.tool()
def recommend_vm_spec(
    filter_policies: Dict[str, Any] = None,
    limit: str = "200",
    priority_policy: str = "location",
    latitude: Optional[float] = None,
    longitude: Optional[float] = None
) -> Any:
    """
    Recommend VM specifications for MCI creation.
    This function works together with search_images() to provide complete MCI creation parameters.
    
    **WORKFLOW INTEGRATION:**
    1. Use search_images() to find suitable images → get 'cspImageName'
    2. Use this function to find appropriate specs → get specification ID  
    3. Use both values in create_mci_dynamic():
       - commonImage: cspImageName from search_images()
       - commonSpec: specification ID from this function
    
    **Example Usage:**
    ```
    # Find specs for AWS in specific region
    specs = recommend_vm_spec(
        filter_policies={
            "ProviderName": "aws",
            "vCPU": {"min": 2, "max": 4},
            "memoryGiB": {"min": 4}
        },
        priority_policy="cost"
    )
    
    # From results, pick a spec ID like "aws+ap-northeast-2+t2.small"
    # Use it in create_mci_dynamic(commonSpec="aws+ap-northeast-2+t2.small")
    ```
    
    Args:
        filter_policies: Filter criteria including:
                        - vCPU: {"min": 2, "max": 8} for CPU requirements
                        - memoryGiB: {"min": 4, "max": 16} for memory requirements
                        - ProviderName: "aws", "azure", "gcp" for specific provider
                        - CspSpecName: Specific CSP spec name
                        - RegionName: Specific region
        limit: Maximum number of recommendations (default: "200")
        priority_policy: Optimization strategy:
                        - "cost": Prioritize lower cost
                        - "performance": Prioritize higher performance
                        - "location": Prioritize proximity to coordinates
        latitude: Latitude for location-based priority
        longitude: Longitude for location-based priority
    
    Returns:
        Recommended VM specifications including:
        - id: Specification ID (use as 'commonSpec' in create_mci_dynamic())
        - vCPU: Number of virtual CPUs
        - memoryGiB: Memory in GB
        - costPerHour: Estimated hourly cost
        - providerName: Cloud provider
        - regionName: Region name
        
    **CRITICAL for MCI Creation:**
    The 'id' field from results becomes the 'commonSpec' parameter in create_mci_dynamic().
    Format is typically: {provider}+{region}+{spec_name} (e.g., "aws+ap-northeast-2+t2.small")
    """
    # Configure filter policies according to API spec
    if filter_policies is None:
        filter_policies = {}
    
    policies = []
    for metric, values in filter_policies.items():
        condition_operations = []
        
        # Handle different types of filter values
        if isinstance(values, dict):
            # Handle min/max range filters
            if "min" in values and values["min"] is not None:
                condition_operations.append({"operand": str(values["min"]), "operator": ">="})
            if "max" in values and values["max"] is not None:
                condition_operations.append({"operand": str(values["max"]), "operator": "<="})
        else:
            # Handle exact match filters (for strings like ProviderName)
            if values is not None:
                condition_operations.append({"operand": str(values), "operator": "=="})
        
        # Only add the policy if there are conditions
        if condition_operations:
            policies.append({
                "metric": metric,
                "condition": condition_operations
            })
    
    # Configure priority policy according to API spec
    priority_policies = []
    if priority_policy and priority_policy != "none":
        priority_config = {
            "metric": priority_policy,
            "weight": "1.0"
        }
        
        # Add location parameters if specified
        if priority_policy == "location" and latitude is not None and longitude is not None:
            priority_config["parameter"] = [
                {"key": "coordinateClose", "val": [f"{latitude}/{longitude}"]}
            ]
        
        priority_policies.append(priority_config)
    
    # Build the request data according to model.DeploymentPlan
    data = {
        "filter": {
            "policy": policies
        },
        "limit": str(limit),
        "priority": {
            "policy": priority_policies
        }
    }
    
    return api_request("POST", "/mciRecommendVm", json_data=data)

# # Tool: Create MCI (Traditional method)
# @mcp.tool()
# def create_mci(
#     ns_id: str,
#     name: str,
#     description: str = "Created via MCP",
#     vm_config: List[Dict] = None,
#     install_mon_agent: str = "no",
#     post_command: Optional[Dict] = None,
#     hold: bool = False
# ) -> Dict:
#     """
#     Create Multi-Cloud Infrastructure using traditional method with detailed VM configurations.
#     For easier MCI creation, consider using create_mci_dynamic() instead.
    
#     Args:
#         ns_id: Namespace ID
#         name: MCI name
#         description: MCI description
#         vm_config: Detailed VM configuration list with specific resource IDs
#         install_mon_agent: Whether to install monitoring agent (yes/no)
#         post_command: Post-deployment command configuration
#         hold: Whether to hold provisioning
    
#     Returns:
#         Created MCI information
#     """
#     if vm_config is None:
#         vm_config = []
    
#     data = {
#         "name": name,
#         "description": description,
#         "installMonAgent": install_mon_agent,
#         "vm": vm_config
#     }
    
#     if post_command:
#         data["postCommand"] = post_command
    
#     url = f"/ns/{ns_id}/mci"
#     if hold:
#         url += "?option=hold"
    
#     return api_request("POST", url, json_data=data)

# Tool: Create MCI dynamically (Recommended method)
@mcp.tool()
def create_mci_dynamic(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI created dynamically via MCP",
    install_mon_agent: str = "no",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_command: Optional[Dict] = None,
    hold: bool = False
) -> Dict:
    """
    Create Multi-Cloud Infrastructure dynamically using the official API specification.
    This is the RECOMMENDED method for MCI creation as it automatically handles resource selection.
    
    **CRITICAL WORKFLOW:**
    1. Use get_image_search_options() to see available search parameters
    2. Use search_images() to find suitable images based on your criteria
    3. From search results, identify the 'cspImageName' of your desired image
    4. Use recommend_vm_spec() to find appropriate VM specifications
    5. Create VM configurations with 'cspImageName' as 'commonImage' and spec ID as 'commonSpec'
    
    **Example workflow:**
    ```
    # 1. Get search options
    options = get_image_search_options()
    
    # 2. Search for Ubuntu 22.04 images in AWS ap-northeast-2
    images = search_images(
        provider_name="aws", 
        region_name="ap-northeast-2", 
        os_type="ubuntu 22.04"
    )
    
    # 3. From results, pick a cspImageName (e.g., "ami-0e06732ba3ca8c6cc")
    # 4. Get VM specs
    specs = recommend_vm_spec(filter_policies={"ProviderName": "aws"})
    
    # 5. Create VM configurations
    vm_configs = [
        {
            "commonImage": "ami-0e06732ba3ca8c6cc",  # From search_images()
            "commonSpec": "aws+ap-northeast-2+t2.small",  # From recommend_vm_spec()
            "name": "vm-1",
            "description": "First VM",
            "subGroupSize": "1"
        },
        {
            "commonImage": "ami-0e06732ba3ca8c6cc",
            "commonSpec": "aws+ap-northeast-2+t2.medium", 
            "name": "vm-2",
            "description": "Second VM with different spec",
            "subGroupSize": "1"
        }
    ]
    
    # 6. Create MCI
    mci = create_mci_dynamic(
        ns_id="default",
        name="my-mci",
        vm_configurations=vm_configs
    )
    ```
    
    Args:
        ns_id: Namespace ID
        name: MCI name (required)
        vm_configurations: List of VM configuration dictionaries (required). Each VM config should include:
            - commonImage: CSP-specific image identifier from search_images() (required)
            - commonSpec: VM specification ID from recommend_vm_spec() (required)
            - name: VM name or subGroup name (optional)
            - description: VM description (optional)
            - subGroupSize: Number of VMs in subgroup, default "1" (optional)
            - connectionName: Specific connection name to use (optional)
            - rootDiskSize: Root disk size in GB, default "default" (optional)
            - rootDiskType: Root disk type, default "default" (optional)
            - vmUserPassword: VM user password (optional)
            - label: Key-value pairs for VM labeling (optional)
        description: MCI description (optional)
        install_mon_agent: Whether to install monitoring agent ("yes"/"no", default "no")
        system_label: System label for special purposes (optional)
        label: Key-value pairs for MCI labeling (optional)
        post_command: Post-deployment command configuration with format:
            {"command": ["command1", "command2"], "userName": "username"} (optional)
        hold: Whether to hold provisioning for review (optional)
    
    Returns:
        Created MCI information including:
        - id: MCI ID for future operations
        - vm: List of created VMs with their details
        - status: Current MCI status
        
    **Important Notes:**
    - commonImage must be a valid cspImageName from search_images() results
    - commonSpec must be a valid specification ID from recommend_vm_spec() results
    - The format for commonSpec is typically: {provider}+{region}+{spec_name}
    - Each VM configuration in vm_configurations must have commonImage and commonSpec
    - Use subGroupSize > 1 to create multiple VMs with same configuration
    """
    # Validate required VM configuration fields
    for i, vm_config in enumerate(vm_configurations):
        if "commonImage" not in vm_config or "commonSpec" not in vm_config:
            raise ValueError(f"VM configuration {i} must include both 'commonImage' and 'commonSpec'")
    
    # Build request data according to model.TbMciDynamicReq spec
    data = {
        "name": name,
        "vm": vm_configurations
    }
    
    # Add optional fields
    if description:
        data["description"] = description
    if install_mon_agent:
        data["installMonAgent"] = install_mon_agent
    if system_label:
        data["systemLabel"] = system_label
    if label:
        data["label"] = label
    if post_command:
        data["postCommand"] = post_command
    
    url = f"/ns/{ns_id}/mciDynamic"
    if hold:
        url += "?option=hold"
    
    return api_request("POST", url, json_data=data)

# Tool: Create MCI Dynamic (Simplified)
@mcp.tool()
def create_simple_mci(
    ns_id: str,
    name: str,
    common_image: str,
    common_spec: str,
    vm_count: int = 1,
    description: str = "MCI created via simplified MCP interface",
    install_mon_agent: str = "no",
    hold: bool = False
) -> Dict:
    """
    Simplified MCI creation for common use cases. 
    This is a wrapper around create_mci_dynamic() for easier usage when all VMs use the same configuration.
    
    **WORKFLOW:**
    1. Use search_images() to find your desired image and get its cspImageName
    2. Use recommend_vm_spec() to find appropriate VM specifications
    3. Use this function with the found image and spec
    
    **Example:**
    ```
    # Find an Ubuntu image
    images = search_images(provider_name="aws", region_name="ap-northeast-2", os_type="ubuntu")
    image_name = images[0]["cspImageName"]  # e.g., "ami-0e06732ba3ca8c6cc"
    
    # Get VM specs
    specs = recommend_vm_spec(filter_policies={"ProviderName": "aws"})
    spec_id = specs[0]["id"]  # e.g., "aws+ap-northeast-2+t2.small"
    
    # Create MCI with 3 identical VMs
    mci = create_simple_mci(
        ns_id="default",
        name="my-simple-mci",
        common_image=image_name,
        common_spec=spec_id,
        vm_count=3
    )
    ```
    
    Args:
        ns_id: Namespace ID
        name: MCI name
        common_image: CSP-specific image identifier from search_images() results
        common_spec: VM specification ID from recommend_vm_spec() results
        vm_count: Number of identical VMs to create (default: 1)
        description: MCI description
        install_mon_agent: Whether to install monitoring agent ("yes"/"no")
        hold: Whether to hold provisioning for review
    
    Returns:
        Created MCI information (same as create_mci_dynamic)
    """
    # Create VM configurations for identical VMs
    vm_configurations = []
    for i in range(vm_count):
        vm_config = {
            "commonImage": common_image,
            "commonSpec": common_spec,
            "name": f"vm-{i+1}",
            "description": f"VM {i+1} of {vm_count}",
            "subGroupSize": "1"
        }
        vm_configurations.append(vm_config)
    
    return create_mci_dynamic(
        ns_id=ns_id,
        name=name,
        vm_configurations=vm_configurations,
        description=description,
        install_mon_agent=install_mon_agent,
        hold=hold
    )

# Tool: Delete MCI
@mcp.tool()
def delete_mci(ns_id: str, mci_id: str) -> Dict:
    """
    Delete an MCI.
    This operation will terminate all VMs in the MCI and delete the MCI.
    This operation is irreversible and should be used with caution.
    This operation requires confirmation from the user.
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
    
    Returns:
        Deletion result
    """
    return api_request("DELETE", f"/ns/{ns_id}/mci/{mci_id}?option=terminate")

# Tool: Control MCI
@mcp.tool()
def control_mci(ns_id: str, mci_id: str, action: str) -> Dict:
    """
    Control an MCI. Control action (refine, suspend, resume, reboot, terminate, continue, withdraw, etc.)
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        action: Control action (refine, suspend, resume, reboot, terminate, continue, withdraw, etc.)
    
    Returns:
        Control result
    """
    valid_actions = ["refine", "suspend", "resume", "reboot", "terminate", "continue", "withdraw"]
    if action not in valid_actions:
        return {"error": f"Unsupported action: {action}. Supported actions: {', '.join(valid_actions)}"}
    
    return api_request("GET", f"/ns/{ns_id}/control/mci/{mci_id}?action={action}")

#####################################
# NLB Management (Network Load Balancer)
#####################################

# # Tool: Create Multi-Cloud NLB
# @mcp.tool()
# def create_mc_nlb(ns_id: str, mci_id: str, port: int = 80, type: str = "PUBLIC", scope: str = "REGION") -> Dict:
#     """
#     Create Multi-Cloud NLB
    
#     Args:
#         ns_id: Namespace ID
#         mci_id: MCI ID
#         port: Port number
#         type: NLB type (PUBLIC/PRIVATE)
#         scope: NLB scope (REGION)
    
#     Returns:
#         Created MC-NLB information
#     """
#     data = {
#         "type": type,
#         "scope": scope,
#         "listener": {
#             "Protocol": "TCP",
#             "Port": str(port)
#         },
#         "targetGroup": {
#             "Protocol": "TCP",
#             "Port": str(port)
#         },
#         "HealthChecker": {
#             "Interval": "default",
#             "Timeout": "default",
#             "Threshold": "default"
#         }
#     }
    
#     return api_request("POST", f"/ns/{ns_id}/mci/{mci_id}/mcSwNlb", json_data=data)

# # Tool: Create Regional NLB
# @mcp.tool()
# def create_region_nlb(
#     ns_id: str, 
#     mci_id: str, 
#     subgroup_id: str, 
#     port: int = 80, 
#     type: str = "PUBLIC", 
#     scope: str = "REGION"
# ) -> Dict:
#     """
#     Create Regional NLB
    
#     Args:
#         ns_id: Namespace ID
#         mci_id: MCI ID
#         subgroup_id: Subgroup ID
#         port: Port number
#         type: NLB type (PUBLIC/PRIVATE)
#         scope: NLB scope (REGION)
    
#     Returns:
#         Created NLB information
#     """
#     data = {
#         "type": type,
#         "scope": scope,
#         "listener": {
#             "Protocol": "TCP",
#             "Port": str(port)
#         },
#         "targetGroup": {
#             "Protocol": "TCP",
#             "Port": str(port),
#             "subGroupId": subgroup_id
#         },
#         "HealthChecker": {
#             "Interval": "default",
#             "Timeout": "default",
#             "Threshold": "default"
#         }
#     }
    
#     return api_request("POST", f"/ns/{ns_id}/mci/{mci_id}/nlb", json_data=data)

# # Tool: Delete NLB
# @mcp.tool()
# def delete_nlb(ns_id: str, mci_id: str, subgroup_id: str) -> Dict:
#     """
#     Delete NLB
    
#     Args:
#         ns_id: Namespace ID
#         mci_id: MCI ID
#         subgroup_id: Subgroup ID
    
#     Returns:
#         Deletion result
#     """
#     return api_request("DELETE", f"/ns/{ns_id}/mci/{mci_id}/nlb/{subgroup_id}")

#####################################
# Command & File Management
#####################################

# Tool: Execute remote command to VMs in MCI
@mcp.tool()
def execute_command_mci(
    ns_id: str, 
    mci_id: str, 
    commands: List[str], 
    subgroup_id: Optional[str] = None, 
    vm_id: Optional[str] = None,
    label_selector: Optional[str] = None
) -> Dict:
    """
    Execute remote commands based on SSH on VMs of an MCI.
    This allows executing commands on all VMs in the MCI or specific VMs based on subgroup or label selector.
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        commands: List of commands to execute
        subgroup_id: Subgroup ID (optional)
        vm_id: VM ID (optional)
        label_selector: Label selector (optional)
    
    Returns:
        Command execution result
    """
    data = {
        "command": commands
    }
    
    url = f"/ns/{ns_id}/cmd/mci/{mci_id}"
    params = {}
    
    if subgroup_id:
        params["subGroupId"] = subgroup_id
    if vm_id:
        params["vmId"] = vm_id
    if label_selector:
        params["labelSelector"] = label_selector
    
    if params:
        query_string = "&".join([f"{k}={v}" for k, v in params.items()])
        url += f"?{query_string}"
    
    return api_request("POST", url, json_data=data)

# Tool: Transfer file to VMs in MCI
@mcp.tool()
def transfer_file_mci(
    ns_id: str, 
    mci_id: str, 
    file_path: str, 
    target_path: str,
    subgroup_id: Optional[str] = None, 
    vm_id: Optional[str] = None
) -> Dict:
    """
    Transfer file to an MCI
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        file_path: Local file path to transfer
        target_path: Target path
        subgroup_id: Subgroup ID (optional)
        vm_id: VM ID (optional)
    
    Returns:
        File transfer result
    """
    url = f"/ns/{ns_id}/transferFile/mci/{mci_id}"
    params = {}
    
    if subgroup_id:
        params["subGroupId"] = subgroup_id
    if vm_id:
        params["vmId"] = vm_id
    
    if params:
        query_string = "&".join([f"{k}={v}" for k, v in params.items()])
        url += f"?{query_string}"
    
    # Open file
    try:
        with open(file_path, 'rb') as file:
            files = {'file': (os.path.basename(file_path), file)}
            data = {'path': target_path}
            
            # Request with multipart form data
            return api_request("POST", url, files=files, json_data=data)
    except Exception as e:
        return {"error": f"File transfer error: {str(e)}"}

# Tool: Get request by ID
@mcp.tool()
def get_request_by_id(request_id: str) -> Dict:
    """
    Get request by request ID
    
    Args:
        request_id: Request ID
    
    Returns:
        Request information
    """
    return api_request("GET", f"/request/{request_id}")

#####################################
# Prompts
#####################################

# Prompt: Namespace management prompt
@mcp.prompt()
def namespace_management_prompt() -> str:
    """Prompt for namespace management"""
    return """
    You are a namespace management expert for Cloud-Barista CB-Tumblebug.
    You can perform the following tasks:
    
    1. View list of namespaces
    2. View specific namespace information
    3. Create a new namespace
    4. Update namespace information
    5. Delete a namespace
    
    Perform appropriate actions according to the user's request and explain the results clearly.
    
    Current namespace list:
    {{namespace://list}}
    
    How can I help you?
    """

# Prompt: MCI management prompt
@mcp.prompt()
def mci_management_prompt() -> str:
    """Prompt for MCI management"""
    return """
    You are a Multi-Cloud Infrastructure (MCI) management expert for Cloud-Barista CB-Tumblebug.
    You can perform comprehensive MCI operations including:
    
    **COMPLETE MCI CREATION WORKFLOW:**
    1. **Image Discovery**: Use get_image_search_options() and search_images() to find suitable OS images
    2. **Spec Selection**: Use recommend_vm_spec() to find optimal VM specifications  
    3. **MCI Creation**: Use create_mci_dynamic() with image and spec information
    4. **Management**: Monitor, control, and manage your infrastructure
    
    **Key Functions Available:**
    - get_image_search_options(): Discover available search parameters
    - search_images(): Find images by OS, provider, region (returns cspImageName)
    - recommend_vm_spec(): Find VM specs by requirements (returns spec ID)
    - create_mci_dynamic(): Create infrastructure (needs cspImageName + spec ID)
    - control_mci(): Manage MCI lifecycle (suspend, resume, reboot, terminate)
    - execute_command(): Run commands on VMs
    - transfer_file(): Upload files to VMs
    
    **IMPORTANT WORKFLOW EXAMPLE:**
    ```
    1. options = get_image_search_options()  # See available search criteria
    2. images = search_images(provider_name="aws", os_type="ubuntu 22.04")  
    3. specs = recommend_vm_spec(filter_policies={"ProviderName": "aws"})
    4. mci = create_mci_dynamic(
         commonImage="ami-xxx",      # From step 2: cspImageName
         commonSpec="aws+region+type" # From step 3: spec ID
       )
    ```
    
    Current namespace list: {{namespace://list}}
    
    What MCI operation would you like to perform?
    """

# Prompt: Resource management prompt
@mcp.prompt()
def resource_management_prompt() -> str:
    """Prompt for resource management"""
    return """
    You are a resource management expert for Cloud-Barista CB-Tumblebug.
    You can perform the following tasks:
    
    1. Manage Network (VNet) resources
    2. Manage Security Group resources
    3. Manage SSH keys
    4. Manage images and specifications
    5. Manage resource connections
    6. Register and release CSP resources
    
    Perform appropriate actions according to the user's request and explain the results clearly.
    
    Current namespace list:
    {{namespace://list}}
    
    How can I help you?
    """

# Prompt: Cloud connection management prompt
@mcp.prompt()
def connection_management_prompt() -> str:
    """Prompt for cloud connection management"""
    return """
    You are a cloud connection management expert for Cloud-Barista CB-Tumblebug.
    You can perform the following tasks:
    
    1. View list of registered cloud connections
    2. View specific cloud connection information
    3. View cloud region and location information
    
    Current list of registered cloud connections:
    {{connection://list}}
    
    How can I help you?
    """

# Prompt: Workflow demo prompt
@mcp.prompt()
def workflow_demo_prompt() -> str:
    """Prompt for workflow demonstration"""
    return """
    You are a Cloud-Barista CB-Tumblebug expert who helps demonstrate how to create and manage Multi-Cloud Infrastructure (MCI).
    
    You can guide through the following workflows:
    
    1. Create a namespace
    2. View cloud connection information
    3. Recommend VM specifications and create an MCI
    4. Check and control MCI status
    5. Execute remote commands
    6. Configure Network Load Balancers
    7. Clean up and delete resources
    
    Current namespace list:
    {{namespace://list}}
    
    Current list of registered cloud connections:
    {{connection://list}}
    
    Which demonstration would you like me to guide you through?
    """

# Prompt: Image search and MCI creation workflow guide
@mcp.prompt()
def image_mci_workflow_prompt() -> str:
    """Complete workflow guide for image search and MCI creation"""
    return """
    You are an expert guide for the complete Image Search → MCI Creation workflow in CB-Tumblebug.
    
    **STEP-BY-STEP WORKFLOW:**
    
    **Step 1: Discover Available Search Options**
    Use get_image_search_options() to understand available search parameters:
    - osArchitecture: "x86_64", "arm64"
    - osType: "ubuntu 22.04", "centos 7", "windows server 2019"
    - providerName: "aws", "azure", "gcp"
    - regionName: "ap-northeast-2", "us-east-1", "koreacentral"
    
    **Step 2: Search for Images**
    Use search_images() with specific criteria:
    ```
    search_images(
        provider_name="aws",
        region_name="ap-northeast-2", 
        os_type="ubuntu 22.04"
    )
    ```
    
    **Step 3: Identify Image Details**
    From search results, find the 'cspImageName' (e.g., "ami-0e06732ba3ca8c6cc")
    This is the CRITICAL value needed for MCI creation.
    
    **Step 4: Get VM Specifications**
    Use recommend_vm_spec() to find appropriate specs:
    ```
    recommend_vm_spec(
        filter_policies={"ProviderName": "aws", "vCPU": {"min": 2}}
    )
    ```
    
    **Step 5: Create MCI**
    Use create_mci_dynamic() with the found values:
    ```
    create_mci_dynamic(
        ns_id="default",
        name="my-infrastructure",
        common_image="ami-0e06732ba3ca8c6cc",  # From Step 3
        common_spec="aws+ap-northeast-2+t2.small",  # From Step 4
        vm_count=2
    )
    ```
    
    **KEY RELATIONSHIPS:**
    - search_images() → cspImageName → commonImage parameter
    - recommend_vm_spec() → specification ID → commonSpec parameter
    - Both are required for create_mci_dynamic()    
    **IMPORTANT NOTES:**
    - Always use search_images() before MCI creation
    - The cspImageName is provider-specific (AMI ID for AWS, Image ID for Azure, etc.)
    - commonSpec format: {provider}+{region}+{spec_name}
    - Test with hold=True first to review configuration
    
    Current namespaces: {{namespace://list}}
    
    What would you like to help you create today?
    """

logger.info("MCP server initialization complete")

