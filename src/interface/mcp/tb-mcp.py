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

# IMPORTANT POLICY: install_mon_agent
# By default, monitoring agent installation is set to "no" unless explicitly requested by user.
# When install_mon_agent="no" (default), the parameter is omitted from API requests to reduce overhead.
# Only when user explicitly requests install_mon_agent="yes", it will be included in the API call.


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
# Namespace Helper Functions
#####################################

def select_best_image(image_list: List[Dict]) -> Dict:
    """
    Select the best image from search results based on priority:
    1. isBasicImage: true (highest priority)
    2. General/basic OS images (determined by LLM analysis)
    3. Fallback to first available image
    
    Args:
        image_list: List of image dictionaries from search_images() result
    
    Returns:
        Selected image dictionary with selection reasoning
    """
    if not image_list:
        return None
    
    # Priority 1: Find images with isBasicImage: true
    basic_images = [img for img in image_list if img.get("isBasicImage", False)]
    if basic_images:
        selected = basic_images[0]
        selected["_selection_reason"] = "isBasicImage: true (highest priority)"
        return selected
    
    # Priority 2: Use LLM-based analysis to find the most suitable basic OS image
    # Create a prompt for the MCP client to analyze and select the best image
    image_analysis_data = []
    for i, img in enumerate(image_list):
        analysis_item = {
            "index": i,
            "name": img.get("name", ""),
            "description": img.get("description", ""),
            "guestOS": img.get("guestOS", ""),
            "cspImageName": img.get("cspImageName", "")
        }
        image_analysis_data.append(analysis_item)
    
    # Since this is a helper function, we'll implement a simple heuristic-based approach
    # that can still be intelligent without hardcoded patterns
    def calculate_image_suitability_score(image):
        """
        Calculate suitability score based on intelligent analysis of image metadata
        """
        name = image.get("name", "").lower()
        description = image.get("description", "").lower()
        guest_os = image.get("guestOS", "").lower()
        
        # Combine all text for analysis
        combined_text = f"{name} {description} {guest_os}"
        
        score = 0
        selection_reasons = []
        
        # Boost score for standard OS indicators
        os_indicators = ["ubuntu", "centos", "amazon", "rhel", "debian", "suse", "windows"]
        for indicator in os_indicators:
            if indicator in combined_text:
                score += 10
                selection_reasons.append(f"Standard OS: {indicator}")
                break
        
        # Boost score for basic/official indicators
        basic_indicators = ["official", "standard", "base", "minimal", "lts"]
        for indicator in basic_indicators:
            if indicator in combined_text:
                score += 5
                selection_reasons.append(f"Basic image indicator: {indicator}")
        
        # Reduce score for specialized software indicators
        specialized_indicators = [
            "gpu", "cuda", "nvidia",  # GPU/ML
            "docker", "kubernetes", "k8s",  # Container platforms
            "lamp", "wordpress", "drupal",  # Web applications
            "mysql", "postgres", "mongodb", "elastic",  # Databases
            "hadoop", "spark", "kafka",  # Big data
            "jenkins", "gitlab", "bamboo",  # CI/CD
            "tensorflow", "pytorch", "jupyter",  # ML frameworks
            "nginx", "apache", "tomcat", "jboss",  # Web servers
            "node", "python", "ruby", "go", "java", "dotnet"  # Runtime environments
        ]
        
        for indicator in specialized_indicators:
            if indicator in combined_text:
                score -= 3
                selection_reasons.append(f"Specialized software detected: {indicator}")
        
        # Prefer shorter names (usually more basic)
        if len(name) < 30:
            score += 2
            selection_reasons.append("Concise name (likely basic)")
        elif len(name) > 60:
            score -= 2
            selection_reasons.append("Long name (possibly specialized)")
        
        # Boost score for recent year indicators (more up-to-date)
        current_year = "2024"
        recent_years = ["2024", "2023", "2022"]
        for year in recent_years:
            if year in combined_text:
                score += 1
                selection_reasons.append(f"Recent version: {year}")
                break
        
        return score, selection_reasons
    
    # Score all images
    scored_images = []
    for img in image_list:
        score, reasons = calculate_image_suitability_score(img)
        scored_images.append((img, score, reasons))
    
    # Sort by score (highest first)
    scored_images.sort(key=lambda x: x[1], reverse=True)
    
    # Return the best scored image
    if scored_images:
        best_image, best_score, reasons = scored_images[0]
        best_image["_selection_reason"] = f"Best general OS image (score: {best_score})"
        best_image["_analysis_details"] = reasons
        return best_image
    
    # Fallback: return first image if no scoring worked
    fallback_image = image_list[0]
    fallback_image["_selection_reason"] = "Fallback to first available image"
    return fallback_image

# Tool: Advanced image selection with context analysis
@mcp.tool()
def select_best_image_with_context(
    image_list: List[Dict], 
    use_case: str = "general",
    requirements: Optional[str] = None
) -> Dict:
    """
    Advanced image selection using contextual analysis for specific use cases.
    This function provides more sophisticated image selection than the basic select_best_image helper.
    
    Args:
        image_list: List of image dictionaries from search_images() result
        use_case: Type of use case ("general", "web-server", "database", "development", "production")
        requirements: Additional requirements or preferences in natural language
    
    Returns:
        Selected image with detailed analysis and reasoning
    """
    if not image_list:
        return {"error": "No images provided for selection"}
    
    # Priority 1: Always prefer isBasicImage: true
    basic_images = [img for img in image_list if img.get("isBasicImage", False)]
    if basic_images:
        selected = basic_images[0]
        return {
            "selected_image": selected,
            "csp_image_name": selected.get("cspImageName"),
            "selection_reason": "isBasicImage: true (highest priority)",
            "confidence": "high",
            "use_case_match": "excellent"
        }
    
    # Priority 2: Context-aware analysis
    analysis_results = []
    
    for img in image_list:
        name = img.get("name", "").lower()
        description = img.get("description", "").lower()
        guest_os = img.get("guestOS", "").lower()
        combined_text = f"{name} {description} {guest_os}"
        
        score = 0
        reasons = []
        
        # Base OS recognition
        if any(os in combined_text for os in ["ubuntu", "centos", "amazon", "rhel", "debian"]):
            score += 15
            reasons.append("Standard Linux distribution")
        elif "windows" in combined_text:
            score += 15
            reasons.append("Windows operating system")
        
        # Use case specific scoring
        if use_case == "web-server":
            if any(term in combined_text for term in ["nginx", "apache", "web"]):
                score += 10
                reasons.append("Web server optimized")
            if any(term in combined_text for term in ["lamp", "lemp"]):
                score += 5
                reasons.append("Web stack included")
        elif use_case == "database":
            if any(term in combined_text for term in ["mysql", "postgres", "mongodb"]):
                score += 10
                reasons.append("Database software included")
        elif use_case == "development":
            if any(term in combined_text for term in ["dev", "development", "sdk"]):
                score += 5
                reasons.append("Development environment")
        elif use_case == "production":
            if any(term in combined_text for term in ["production", "stable", "lts"]):
                score += 10
                reasons.append("Production ready")
        
        # General quality indicators
        if any(term in combined_text for term in ["official", "standard", "base"]):
            score += 8
            reasons.append("Official/standard image")
        
        if any(term in combined_text for term in ["minimal", "clean"]):
            score += 5
            reasons.append("Minimal installation")
        
        # Avoid over-specialized images for general use
        if use_case == "general":
            specialized_terms = ["gpu", "cuda", "docker", "kubernetes", "hadoop", "spark"]
            if any(term in combined_text for term in specialized_terms):
                score -= 10
                reasons.append("Specialized software detected (may not be suitable for general use)")
        
        # Name length consideration
        if len(name) < 40:
            score += 2
            reasons.append("Concise name")
        
        analysis_results.append({
            "image": img,
            "score": score,
            "reasons": reasons,
            "combined_text": combined_text[:100] + "..." if len(combined_text) > 100 else combined_text
        })
    
    # Sort by score and select the best
    analysis_results.sort(key=lambda x: x["score"], reverse=True)
    
    if analysis_results:
        best = analysis_results[0]
        confidence = "high" if best["score"] >= 20 else "medium" if best["score"] >= 10 else "low"
        
        return {
            "selected_image": best["image"],
            "csp_image_name": best["image"].get("cspImageName"),
            "selection_reason": f"Best match for {use_case} use case (score: {best['score']})",
            "analysis_details": best["reasons"],
            "confidence": confidence,
            "use_case_match": "excellent" if best["score"] >= 25 else "good" if best["score"] >= 15 else "fair",
            "alternative_options": [
                {
                    "image_name": alt["image"].get("name", ""),
                    "score": alt["score"],
                    "reasons": alt["reasons"][:2]  # Top 2 reasons
                }
                for alt in analysis_results[1:3]  # Show top 2 alternatives
            ]
        }
    
    # Fallback
    fallback = image_list[0]
    return {
        "selected_image": fallback,
        "csp_image_name": fallback.get("cspImageName"),
        "selection_reason": "Fallback to first available image",
        "confidence": "low",
        "use_case_match": "unknown"
    }

#####################################
# Namespace Helper Functions
#####################################

# Helper function: Check and manage namespace for MCI operations
@mcp.tool()
def check_and_prepare_namespace(preferred_ns_id: Optional[str] = None) -> Dict:
    """
    Check available namespaces and help user select or create one for MCI operations.
    This function provides intelligent namespace management by:
    1. Listing existing namespaces
    2. Suggesting namespace selection if available
    3. Offering to create a new namespace if none exist or user prefers
    
    Args:
        preferred_ns_id: Preferred namespace ID to check (optional)
    
    Returns:
        Namespace management guidance including:
        - available_namespaces: List of existing namespaces
        - recommendation: Suggested action
        - preferred_namespace: Information about preferred namespace if specified
    """
    # Get all existing namespaces
    ns_result = get_namespaces()
    
    if "error" in ns_result:
        return {
            "error": "Failed to retrieve namespaces",
            "details": ns_result["error"],
            "recommendation": "Please check your connection to Tumblebug API"
        }
    
    available_namespaces = ns_result.get("namespaces", [])
    
    result = {
        "available_namespaces": available_namespaces,
        "total_count": len(available_namespaces)
    }
    
    # If preferred namespace is specified, check if it exists
    if preferred_ns_id:
        preferred_exists = any(ns.get("id") == preferred_ns_id for ns in available_namespaces)
        if preferred_exists:
            result["preferred_namespace"] = {
                "id": preferred_ns_id,
                "exists": True,
                "status": "ready_to_use"
            }
            result["recommendation"] = f"Preferred namespace '{preferred_ns_id}' exists and is ready to use for MCI creation."
        else:
            result["preferred_namespace"] = {
                "id": preferred_ns_id,
                "exists": False,
                "status": "needs_creation"
            }
            result["recommendation"] = f"Preferred namespace '{preferred_ns_id}' does not exist. You can create it using create_namespace() function."
    
    # Provide guidance based on available namespaces
    if len(available_namespaces) == 0:
        result["recommendation"] = "No namespaces found. You need to create a namespace first using create_namespace() before creating MCI."
        result["suggested_action"] = "create_namespace"
    elif len(available_namespaces) == 1:
        single_ns = available_namespaces[0]
        result["recommendation"] = f"One namespace available: '{single_ns.get('id', 'unknown')}'. You can use this for MCI creation or create a new one if needed."
        result["suggested_namespace"] = single_ns.get("id", "unknown")
        result["suggested_action"] = "use_existing_or_create_new"
    else:
        result["recommendation"] = f"Multiple namespaces available ({len(available_namespaces)}). Please select one for MCI creation or create a new one."
        result["suggested_action"] = "select_existing_or_create_new"
        result["namespace_options"] = [ns.get("id", "unknown") for ns in available_namespaces]
    
    return result

# Helper function: Validate namespace exists
@mcp.tool() 
def validate_namespace(ns_id: str) -> Dict:
    """
    Validate if a namespace exists and provide its details
    
    Args:
        ns_id: Namespace ID to validate
    
    Returns:
        Validation result with namespace details or error
    """
    try:
        ns_info = get_namespace(ns_id)
        if "error" in ns_info:
            return {
                "valid": False,
                "namespace_id": ns_id,
                "error": "Namespace does not exist",
                "suggestion": f"Create namespace '{ns_id}' using create_namespace() or choose from existing namespaces using check_and_prepare_namespace()"
            }
        
        return {
            "valid": True,
            "namespace_id": ns_id,
            "namespace_info": ns_info,
            "status": "ready_for_mci_creation"
        }
    except Exception as e:
        return {
            "valid": False,
            "namespace_id": ns_id,
            "error": f"Failed to validate namespace: {str(e)}",
            "suggestion": "Check your connection and try again"
        }

# Helper function: Create namespace with validation
@mcp.tool()
def create_namespace_with_validation(name: str, description: Optional[str] = None) -> Dict:
    """
    Create a new namespace with validation and confirmation
    
    Args:
        name: Name of the namespace to create
        description: Description of the namespace (optional)
    
    Returns:
        Creation result with validation status
    """
    # First check if namespace already exists
    validation = validate_namespace(name)
    if validation["valid"]:
        return {
            "created": False,
            "namespace_id": name,
            "message": f"Namespace '{name}' already exists",
            "existing_info": validation["namespace_info"],
            "suggestion": "You can use this existing namespace for MCI creation"
        }
    
    # Create the namespace
    try:
        result = create_namespace(name, description)
        if "error" in result:
            return {
                "created": False,
                "namespace_id": name,
                "error": result["error"],
                "suggestion": "Please check the namespace name and try again"
            }
        
        # Validate the created namespace
        validation = validate_namespace(name)
        
        return {
            "created": True,
            "namespace_id": name,
            "namespace_info": result,
            "validation": validation,
            "status": "ready_for_mci_creation",
            "message": f"Namespace '{name}' created successfully and ready for MCI creation"
        }
    except Exception as e:
        return {
            "created": False,
            "namespace_id": name,
            "error": f"Failed to create namespace: {str(e)}",
            "suggestion": "Please check your input and connection"
        }


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

# Tool: Check resource exists
@mcp.tool()
def check_resource_exists(ns_id: str, resource_type: str, resource_id: str) -> Dict:
    """
    Check if a specific resource exists in the namespace.
    This is useful for validating resources before using them in MCI creation.
    
    Args:
        ns_id: Namespace ID
        resource_type: Type of resource (e.g., "vNet", "securityGroup", "sshKey", "image", "spec")
        resource_id: Resource ID to check
    
    Returns:
        Resource existence information
    """
    return api_request("GET", f"/ns/{ns_id}/checkResource/{resource_type}/{resource_id}")

# Tool: Get all specs in namespace
@mcp.tool()
def get_specs(ns_id: str) -> Dict:
    """
    Get all VM specifications available in the namespace.
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        List of VM specifications
    """
    return api_request("GET", f"/ns/{ns_id}/resources/spec")

# Tool: Get all images in namespace  
@mcp.tool()
def get_images(ns_id: str) -> Dict:
    """
    Get all images available in the namespace.
    
    Args:
        ns_id: Namespace ID
    
    Returns:
        List of images
    """
    return api_request("GET", f"/ns/{ns_id}/resources/image")

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

# Tool: Get MCI associated resources
@mcp.tool()
def get_mci_associated_resources(ns_id: str, mci_id: str) -> Dict:
    """
    Get associated resource IDs for a given MCI.
    This function returns all resources (VNet, SecurityGroup, SSHKey, etc.) that are used by the MCI.
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
    
    Returns:
        Associated resource information including:
        - vNetIds: List of VNet IDs used by the MCI
        - securityGroupIds: List of Security Group IDs
        - sshKeyIds: List of SSH Key IDs
        - imageIds: List of Image IDs
        - specIds: List of Spec IDs
        - And other resource associations
    """
    return api_request("GET", f"/ns/{ns_id}/mci/{mci_id}/associatedResources")

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
          - isBasicImage: Boolean indicating if this is a basic OS image (PRIORITY indicator)
          
    Important: 
    - The 'cspImageName' from search results becomes the 'commonImage' parameter when creating MCIs
    - For optimal image selection, use select_best_image() helper function which intelligently prioritizes:
      1. Images with isBasicImage: true (highest priority)
      2. General OS images through intelligent analysis of image metadata
      3. Fallback to first available image with detailed reasoning
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
        - costPerHour: Estimated hourly cost (if -1, pricing information is unavailable)
        - providerName: Cloud provider
        - regionName: Region name
        
    **PRICING INFORMATION:**
    When costPerHour is -1, it indicates that pricing information is not available 
    in the API response. In such cases, you may need to refer to the cloud provider's 
    official pricing documentation or use external pricing APIs for accurate costs.
    
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
    1. Use recommend_vm_spec() to find appropriate VM specifications (determines CSP and region)
    2. From spec results, identify target CSP and region
    3. Use search_images() to find suitable images in the selected CSP/region
    4. From search results, identify the 'cspImageName' of your desired image
    5. Create VM configurations with 'cspImageName' as 'commonImage' and spec ID as 'commonSpec'
    
    **Example workflow:**
    ```
    # 1. Get VM specs first (determines CSP and region)
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
    )
    
    # 2. Pick a suitable spec (e.g., "aws+ap-northeast-2+t2.small")
    chosen_spec = specs[0]["id"]  # e.g., "aws+ap-northeast-2+t2.small"
    provider = chosen_spec.split('+')[0]  # Extract "aws"
    region = chosen_spec.split('+')[1]    # Extract "ap-northeast-2"
    
    # 3. Search for images in the selected CSP/region
    images = search_images(
        provider_name=provider,
        region_name=region,
        os_type="ubuntu 22.04"
    )
    
    # 4. Select the best image (intelligent analysis with reasoning)
    best_image = select_best_image(images["imageList"])
    chosen_image = best_image["cspImageName"]
    
    # 5. Create VM configurations
    vm_configs = [
        {
            "commonImage": chosen_image,        # From Step 4
            "commonSpec": chosen_spec,          # From Step 2
            "name": "vm-1",
            "description": "First VM",
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
                          Note: Only requests "yes" when explicitly needed. Default behavior omits this parameter.
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
    # Validate namespace first
    ns_validation = validate_namespace(ns_id)
    if not ns_validation["valid"]:
        return {
            "error": f"Namespace '{ns_id}' validation failed",
            "details": ns_validation.get("error", "Unknown error"),
            "suggestion": ns_validation.get("suggestion", ""),
            "namespace_guidance": "Use check_and_prepare_namespace() to see available namespaces or create_namespace_with_validation() to create a new one"
        }
    
    # Validate required VM configuration fields
    for i, vm_config in enumerate(vm_configurations):
        if "commonImage" not in vm_config or "commonSpec" not in vm_config:
            return {
                "error": f"VM configuration {i} validation failed",
                "details": "VM configuration must include both 'commonImage' and 'commonSpec'",
                "suggestion": "Use search_images() to get 'commonImage' and recommend_vm_spec() to get 'commonSpec'"
            }
    
    # Build request data according to model.TbMciDynamicReq spec
    data = {
        "name": name,
        "vm": vm_configurations
    }
    
    # Add optional fields
    if description:
        data["description"] = description
    # Only include installMonAgent if explicitly set to "yes"
    if install_mon_agent and install_mon_agent.lower() == "yes":
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
    
    result = api_request("POST", url, json_data=data)
    
    # Add namespace info to result for reference
    if "error" not in result:
        result["namespace_info"] = {
            "namespace_id": ns_id,
            "validation": "passed"
        }
    
    return result

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
    1. Use recommend_vm_spec() to find appropriate VM specifications (determines CSP and region)
    2. From spec results, extract CSP and region information  
    3. Use search_images() to find your desired image in the selected CSP/region
    4. Use this function with the found spec and image
    
    **Example:**
    ```
    # Find VM specs first (determines CSP and region)
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
    )
    spec_id = specs[0]["id"]  # e.g., "aws+ap-northeast-2+t2.small"
    
    # Extract CSP and region from spec
    provider = spec_id.split('+')[0]  # "aws"
    region = spec_id.split('+')[1]    # "ap-northeast-2"
    
    # Find the best image in the selected CSP/region
    images = search_images(
        provider_name=provider, 
        region_name=region, 
        os_type="ubuntu"
    )
    
    # Automatically select the best image (intelligent analysis with detailed reasoning)
    best_image = select_best_image(images.get("imageList", []))
    image_name = best_image["cspImageName"]  # e.g., "ami-0e06732ba3ca8c6cc"
    
    # Optional: Check selection reasoning
    # selection_reason = best_image.get("_selection_reason", "")
    # analysis_details = best_image.get("_analysis_details", [])
    
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
        install_mon_agent: Whether to install monitoring agent ("yes"/"no", default "no")
                          Note: Only requests "yes" when explicitly needed. Default behavior omits this parameter.
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

# Tool: Smart MCI Creation with Spec-First Workflow
@mcp.tool()
def create_mci_with_spec_first(
    name: str,
    vm_requirements: List[Dict],
    preferred_ns_id: Optional[str] = None,
    create_ns_if_missing: bool = False,
    ns_description: Optional[str] = None,
    description: str = "MCI created with spec-first workflow",
    install_mon_agent: str = "no",
    hold: bool = False
) -> Dict:
    """
    Smart MCI creation using spec-first workflow for optimal CSP/region selection.
    This function finds VM specifications first (which determines CSP and region), 
    then searches for compatible images in the selected CSP/region.
    
    **SPEC-FIRST WORKFLOW BENEFITS:**
    - Optimal CSP and region selection based on performance/cost requirements
    - Intelligent image selection using smart metadata analysis (no hardcoded patterns)
    - Automatic image compatibility with selected specifications
    - Reduced complexity in multi-CSP environments
    - Better resource optimization with detailed selection reasoning
    
    **Example Usage:**
    ```python
    # Create MCI with performance requirements
    result = create_mci_with_spec_first(
        name="web-servers",
        vm_requirements=[
            {
                "name": "web-server",
                "count": 2,
                "vCPU": {"min": 2, "max": 4},
                "memoryGiB": {"min": 4, "max": 8},
                "os_type": "ubuntu 22.04",
                "priority": "cost"
            },
            {
                "name": "database",
                "count": 1, 
                "vCPU": {"min": 4},
                "memoryGiB": {"min": 8},
                "os_type": "ubuntu 22.04",
                "priority": "performance"
            }
        ],
        preferred_ns_id="production",
        create_ns_if_missing=True
    )
    ```
    
    Args:
        name: MCI name
        vm_requirements: List of VM requirement dictionaries, each containing:
            - name: VM group name (required)
            - count: Number of VMs (default: 1)
            - vCPU: CPU requirements {"min": 2, "max": 8} (optional)
            - memoryGiB: Memory requirements {"min": 4, "max": 16} (optional)
            - os_type: Operating system (e.g., "ubuntu 22.04") (optional)
            - os_architecture: Architecture (e.g., "x86_64") (optional)
            - priority: "cost", "performance", or "location" (default: "cost")
            - provider_preference: Preferred CSP (e.g., "aws", "azure") (optional)
            - region_preference: Preferred region (optional)
        preferred_ns_id: Preferred namespace ID (optional)
        create_ns_if_missing: Whether to create namespace if it doesn't exist
        ns_description: Description for new namespace if created
        description: MCI description
        install_mon_agent: Whether to install monitoring agent ("yes"/"no", default "no")
                          Note: Only requests "yes" when explicitly needed. Default behavior omits this parameter.
        hold: Whether to hold provisioning
    
    Returns:
        Comprehensive result including:
        - namespace_management: Namespace handling results
        - spec_selection: Selected specifications for each VM group
        - image_selection: Selected images for each VM group
        - vm_configurations: Final VM configurations
        - mci_creation: MCI creation results
        - status: Overall operation status
    """
    result = {
        "namespace_management": {},
        "spec_selection": {},
        "image_selection": {},
        "vm_configurations": [],
        "mci_creation": {},
        "status": "in_progress"
    }
    
    # Step 1: Handle namespace management
    if preferred_ns_id:
        ns_validation = validate_namespace(preferred_ns_id)
        if ns_validation["valid"]:
            target_ns_id = preferred_ns_id
            result["namespace_management"]["action"] = "used_existing"
        else:
            if create_ns_if_missing:
                creation_result = create_namespace_with_validation(
                    preferred_ns_id, 
                    ns_description or f"Namespace for MCI {name}"
                )
                if creation_result.get("created") or creation_result.get("message"):
                    target_ns_id = preferred_ns_id
                    result["namespace_management"]["action"] = "created_new"
                    result["namespace_management"]["creation_result"] = creation_result
                else:
                    result["status"] = "failed"
                    result["error"] = "Failed to create namespace"
                    return result
            else:
                result["status"] = "namespace_selection_needed"
                result["error"] = f"Namespace '{preferred_ns_id}' does not exist"
                return result
    else:
        ns_check = check_and_prepare_namespace()
        if len(ns_check["available_namespaces"]) == 0:
            result["status"] = "namespace_creation_needed"
            result["error"] = "No namespaces available"
            return result
        elif len(ns_check["available_namespaces"]) == 1:
            target_ns_id = ns_check["suggested_namespace"]
            result["namespace_management"]["action"] = "used_only_available"
        else:
            result["status"] = "namespace_selection_needed"
            result["error"] = "Multiple namespaces available, specify preferred_ns_id"
            result["available_namespaces"] = ns_check["namespace_options"]
            return result
    
    result["namespace_management"]["final_namespace_id"] = target_ns_id
    
    # Step 2: Process each VM requirement (spec-first approach)
    vm_configs = []
    
    for req_idx, vm_req in enumerate(vm_requirements):
        req_name = vm_req.get("name", f"vm-group-{req_idx + 1}")
        vm_count = vm_req.get("count", 1)
        
        # Build filter policies for spec recommendation
        filter_policies = {}
        if "vCPU" in vm_req:
            filter_policies["vCPU"] = vm_req["vCPU"]
        if "memoryGiB" in vm_req:
            filter_policies["memoryGiB"] = vm_req["memoryGiB"]
        if "provider_preference" in vm_req:
            filter_policies["ProviderName"] = vm_req["provider_preference"]
        if "region_preference" in vm_req:
            filter_policies["RegionName"] = vm_req["region_preference"]
        
        # Step 2a: Find VM specifications
        priority = vm_req.get("priority", "cost")
        specs_result = recommend_vm_spec(
            filter_policies=filter_policies,
            priority_policy=priority,
            limit="10"
        )
        
        if not specs_result or "error" in specs_result:
            result["status"] = "failed"
            result["error"] = f"Failed to find specifications for {req_name}"
            result["spec_selection"][req_name] = {"error": specs_result}
            return result
        
        # Select the best spec
        if isinstance(specs_result, list) and len(specs_result) > 0:
            chosen_spec = specs_result[0]
        elif isinstance(specs_result, dict) and "result" in specs_result:
            chosen_spec = specs_result["result"][0] if specs_result["result"] else None
        else:
            result["status"] = "failed"
            result["error"] = f"Invalid spec recommendation response for {req_name}"
            return result
        
        if not chosen_spec:
            result["status"] = "failed"
            result["error"] = f"No suitable specifications found for {req_name}"
            return result
        
        spec_id = chosen_spec["id"]
        result["spec_selection"][req_name] = {
            "selected_spec": chosen_spec,
            "spec_id": spec_id
        }
        
        # Step 2b: Extract CSP and region from spec
        try:
            spec_parts = spec_id.split('+')
            provider = spec_parts[0]
            region = spec_parts[1]
        except (IndexError, AttributeError):
            result["status"] = "failed"
            result["error"] = f"Invalid spec ID format: {spec_id}"
            return result
        
        # Step 2c: Search for images in the selected CSP/region
        image_search_params = {
            "provider_name": provider,
            "region_name": region
        }
        
        if "os_type" in vm_req:
            image_search_params["os_type"] = vm_req["os_type"]
        if "os_architecture" in vm_req:
            image_search_params["os_architecture"] = vm_req["os_architecture"]
        
        images_result = search_images(**image_search_params)
        
        if not images_result or "error" in images_result:
            result["status"] = "failed"
            result["error"] = f"Failed to find images for {req_name} in {provider}/{region}"
            result["image_selection"][req_name] = {"error": images_result}
            return result
        
        # Select the best suitable image using intelligent selection
        image_list = images_result.get("imageList", [])
        if not image_list:
            result["status"] = "failed"
            result["error"] = f"No images found for {req_name} in {provider}/{region}"
            return result
        
        chosen_image = select_best_image(image_list)
        if not chosen_image:
            result["status"] = "failed" 
            result["error"] = f"Failed to select suitable image for {req_name}"
            return result
            
        csp_image_name = chosen_image["cspImageName"]
        
        result["image_selection"][req_name] = {
            "selected_image": chosen_image,
            "csp_image_name": csp_image_name,
            "provider": provider,
            "region": region,
            "selection_reason": chosen_image.get("_selection_reason", "Selected by intelligent analysis"),
            "is_basic_image": chosen_image.get("isBasicImage", False),
            "analysis_details": chosen_image.get("_analysis_details", [])
        }
        
        # Step 2d: Create VM configurations for this requirement
        for vm_idx in range(vm_count):
            vm_config = {
                "commonImage": csp_image_name,
                "commonSpec": spec_id,
                "name": f"{req_name}-{vm_idx + 1}" if vm_count > 1 else req_name,
                "description": f"VM {vm_idx + 1} of {vm_count} for {req_name}",
                "subGroupSize": "1"
            }
            vm_configs.append(vm_config)
    
    result["vm_configurations"] = vm_configs
    
    # Step 3: Create MCI with all configurations
    mci_result = create_mci_dynamic(
        ns_id=target_ns_id,
        name=name,
        vm_configurations=vm_configs,
        description=description,
        install_mon_agent=install_mon_agent,
        hold=hold
    )
    
    result["mci_creation"] = mci_result
    
    if "error" not in mci_result:
        result["status"] = "success"
        result["summary"] = {
            "namespace_id": target_ns_id,
            "mci_id": mci_result.get("id", name),
            "mci_name": name,
            "total_vms": len(vm_configs),
            "vm_groups": len(vm_requirements),
            "selected_csps": list(set([img["provider"] for img in result["image_selection"].values()])),
            "selected_regions": list(set([img["region"] for img in result["image_selection"].values()]))
        }
    else:
        result["status"] = "mci_creation_failed"
        result["error"] = "MCI creation failed after successful resource selection"
    
    return result

# Tool: Smart MCI Creation with Namespace Management
@mcp.tool()
def create_mci_with_namespace_management(
    name: str,
    vm_configurations: List[Dict],
    preferred_ns_id: Optional[str] = None,
    create_ns_if_missing: bool = False,
    ns_description: Optional[str] = None,
    description: str = "MCI created with smart namespace management",
    install_mon_agent: str = "no",
    hold: bool = False
) -> Dict:
    """
    Smart MCI creation with automatic namespace management.
    This function handles namespace selection/creation automatically before creating MCI.
    
    **INTELLIGENT WORKFLOW:**
    1. Check available namespaces
    2. If preferred_ns_id specified and exists → use it
    3. If preferred_ns_id specified but doesn't exist → create it (if create_ns_if_missing=True)
    4. If no preferred_ns_id → guide user to select from available or create new
    5. Create MCI once namespace is ready
    
    **Example Usage:**
    ```python
    # Auto-create namespace if it doesn't exist
    result = create_mci_with_namespace_management(
        name="my-infrastructure",
        vm_configurations=[...],
        preferred_ns_id="my-project",
        create_ns_if_missing=True,
        ns_description="Project namespace"
    )
    
    # Or let it guide namespace selection
    result = create_mci_with_namespace_management(
        name="my-infrastructure", 
        vm_configurations=[...]
    )
    ```
    
    Args:
        name: MCI name
        vm_configurations: List of VM configurations (same as create_mci_dynamic)
        preferred_ns_id: Preferred namespace ID (optional)
        create_ns_if_missing: Whether to create namespace if it doesn't exist (default: False)
        ns_description: Description for new namespace if created
        description: MCI description
        install_mon_agent: Whether to install monitoring agent ("yes"/"no", default "no")
                          Note: Only requests "yes" when explicitly needed. Default behavior omits this parameter.
        hold: Whether to hold provisioning
    
    Returns:
        Smart creation result including namespace management info and MCI creation result
    """
    result = {
        "namespace_management": {},
        "mci_creation": {},
        "status": "in_progress"
    }
    
    # Step 1: Check and prepare namespace
    ns_check = check_and_prepare_namespace(preferred_ns_id)
    result["namespace_management"]["check_result"] = ns_check
    
    target_ns_id = None
    
    # Step 2: Handle namespace selection/creation
    if preferred_ns_id:
        # User has a preference
        ns_validation = validate_namespace(preferred_ns_id)
        if ns_validation["valid"]:
            # Preferred namespace exists, use it
            target_ns_id = preferred_ns_id
            result["namespace_management"]["action"] = "used_existing_preferred"
            result["namespace_management"]["namespace_id"] = preferred_ns_id
        else:
            # Preferred namespace doesn't exist
            if create_ns_if_missing:
                # Create the preferred namespace
                creation_result = create_namespace_with_validation(
                    preferred_ns_id, 
                    ns_description or f"Namespace for MCI {name}"
                )
                result["namespace_management"]["creation_result"] = creation_result
                
                if creation_result.get("created") or creation_result.get("message"):
                    target_ns_id = preferred_ns_id
                    result["namespace_management"]["action"] = "created_preferred"
                    result["namespace_management"]["namespace_id"] = preferred_ns_id
                else:
                    result["status"] = "failed"
                    result["error"] = "Failed to create preferred namespace"
                    result["suggestion"] = f"Namespace '{preferred_ns_id}' could not be created. " + creation_result.get("suggestion", "")
                    return result
            else:
                result["status"] = "namespace_selection_needed"
                result["error"] = f"Preferred namespace '{preferred_ns_id}' does not exist"
                result["suggestion"] = f"Set create_ns_if_missing=True to auto-create, or use check_and_prepare_namespace() to see available options"
                result["available_options"] = ns_check
                return result
    else:
        # No preference specified, guide user
        if len(ns_check["available_namespaces"]) == 0:
            result["status"] = "namespace_creation_needed"
            result["error"] = "No namespaces available"
            result["suggestion"] = "Create a namespace first using create_namespace_with_validation() or specify preferred_ns_id with create_ns_if_missing=True"
            return result
        elif len(ns_check["available_namespaces"]) == 1:
            # Use the only available namespace
            target_ns_id = ns_check["suggested_namespace"]
            result["namespace_management"]["action"] = "used_only_available"
            result["namespace_management"]["namespace_id"] = target_ns_id
        else:
            # Multiple namespaces available, need user selection
            result["status"] = "namespace_selection_needed"
            result["suggestion"] = "Multiple namespaces available. Specify preferred_ns_id parameter to select one:"
            result["available_namespaces"] = ns_check["namespace_options"]
            return result
    
    # Step 3: Create MCI with selected/created namespace
    if target_ns_id:
        result["namespace_management"]["final_namespace_id"] = target_ns_id
        
        mci_result = create_mci_dynamic(
            ns_id=target_ns_id,
            name=name,
            vm_configurations=vm_configurations,
            description=description,
            install_mon_agent=install_mon_agent,
            hold=hold
        )
        
        result["mci_creation"] = mci_result
        
        if "error" not in mci_result:
            result["status"] = "success"
            result["summary"] = {
                "namespace_id": target_ns_id,
                "mci_id": mci_result.get("id", name),
                "mci_name": name,
                "vm_count": len(vm_configurations)
            }
        else:
            result["status"] = "mci_creation_failed"
            result["error"] = "MCI creation failed after successful namespace setup"
    else:
        result["status"] = "failed"
        result["error"] = "No valid namespace could be determined"
    
    return result

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
    You can perform comprehensive MCI operations with intelligent namespace management:
    
    **COMPLETE MCI CREATION WORKFLOW WITH SMART NAMESPACE MANAGEMENT:**
    1. **Namespace Check**: Use check_and_prepare_namespace() to see available namespaces
    2. **Namespace Setup**: Create new namespace if needed with create_namespace_with_validation()
    3. **Spec Selection**: Use recommend_vm_spec() to find optimal VM specifications (determines CSP and region)
    4. **Image Discovery**: Use search_images() to find suitable OS images in the selected CSP/region
    5. **Smart MCI Creation**: Use create_mci_with_namespace_management() for automated namespace handling
    6. **Management**: Monitor, control, and manage your infrastructure
    
    **SMART NAMESPACE MANAGEMENT FUNCTIONS:**
    - check_and_prepare_namespace(): Check available namespaces and get recommendations
    - validate_namespace(): Verify if a specific namespace exists
    - create_namespace_with_validation(): Create namespace with validation
    - create_mci_with_namespace_management(): Smart MCI creation with auto namespace handling
    - create_mci_with_spec_first(): Advanced MCI creation with spec-first workflow (RECOMMENDED for new requests)
    
    **TRADITIONAL MCI FUNCTIONS:**
    - recommend_vm_spec(): Find VM specs by requirements (returns spec ID, determines CSP/region)
    - search_images(): Find images by CSP/region/OS (returns cspImageName)
    - create_mci_dynamic(): Create infrastructure (needs cspImageName + spec ID + valid namespace)
    - control_mci(): Manage MCI lifecycle (suspend, resume, reboot, terminate)
    - execute_command(): Run commands on VMs
    - transfer_file(): Upload files to VMs
    
    **RECOMMENDED WORKFLOW EXAMPLE (SPEC-FIRST):**
    ```
    1. # Create MCI with requirements (spec-first approach)
       result = create_mci_with_spec_first(
           name="my-infrastructure",
           vm_requirements=[
               {
                   "name": "web-servers",
                   "count": 2,
                   "vCPU": {"min": 2, "max": 4},
                   "memoryGiB": {"min": 4},
                   "os_type": "ubuntu 22.04",
                   "priority": "cost"
               }
           ],
           preferred_ns_id="my-project",
           create_ns_if_missing=True
       )
    ```
    
    **TRADITIONAL WORKFLOW EXAMPLE:**
    ```
    1. # Check/create namespace manually
       ns_result = create_namespace_with_validation("my-project")
       
    2. # Find specifications first (determines CSP and region)
       specs = recommend_vm_spec(
           filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
       )
       chosen_spec = specs[0]["id"]  # e.g., "aws+ap-northeast-2+t2.small"
       
    3. # Extract CSP and region, then find images
       provider = chosen_spec.split('+')[0]  # "aws"
       region = chosen_spec.split('+')[1]    # "ap-northeast-2"
       images = search_images(
           provider_name=provider, 
           region_name=region, 
           os_type="ubuntu 22.04"
       )
       
    4. # Select the best image and create MCI
       best_image = select_best_image(images["imageList"])
       mci = create_mci_dynamic(
           ns_id="my-project",
           commonImage=best_image["cspImageName"],  # From step 4 (best image)
           commonSpec=chosen_spec                   # From step 2
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
    """Complete workflow guide for image search and MCI creation with smart namespace management"""
    return """
    You are an expert guide for the complete Namespace Management → Image Search → MCI Creation workflow in CB-Tumblebug.
    
    **SMART WORKFLOW (RECOMMENDED):**
    
    **Step 0: Smart Namespace Management**
    Use create_mci_with_namespace_management() for automated handling:
    ```python
    result = create_mci_with_namespace_management(
        name="my-infrastructure",
        vm_configurations=[{
            "commonImage": "ami-xxx",     # From image search
            "commonSpec": "aws+region+spec"  # From spec recommendation
        }],
        preferred_ns_id="my-project",     # Optional: preferred namespace
        create_ns_if_missing=True         # Auto-create if missing
    )
    ```
    
    **MANUAL STEP-BY-STEP WORKFLOW:**
    
    **Step 0: Namespace Preparation**
    Check and prepare namespace first:
    ```python
    # Check what's available
    ns_check = check_and_prepare_namespace("my-project")
    
    # Create if needed
    ns_result = create_namespace_with_validation("my-project", "My project namespace")
    ```
    
    **Step 1: Find VM Specifications (determines CSP and region)**
    Use recommend_vm_spec() to find appropriate specs based on requirements:
    ```python
    specs = recommend_vm_spec(
        filter_policies={
            "vCPU": {"min": 2, "max": 4},
            "memoryGiB": {"min": 4, "max": 8}
        },
        priority_policy="cost"
    )
    ```
    
    **Step 2: Extract CSP and Region Information**
    From spec results, identify target CSP and region:
    ```python
    chosen_spec = specs[0]["id"]  # e.g., "aws+ap-northeast-2+t2.small"
    provider = chosen_spec.split('+')[0]  # Extract "aws"
    region = chosen_spec.split('+')[1]    # Extract "ap-northeast-2"
    ```
    
    **Step 3: Search for Images in Selected CSP/Region**
    Use search_images() with the determined CSP and region:
    ```python
    images = search_images(
        provider_name=provider,
        region_name=region,
        os_type="ubuntu 22.04"
    )
    ```
    
    **Step 4: Identify Image Details**
    From search results, find the 'cspImageName' (e.g., "ami-0e06732ba3ca8c6cc")
    Use select_best_image() to choose the optimal image through intelligent analysis:
    1. Images with isBasicImage: true (highest priority)
    2. General OS images identified through smart metadata analysis
    3. Fallback to first available image with detailed reasoning
    This is the CRITICAL value needed for MCI creation.
    
    **Step 5: Create MCI (with validated namespace)**
    Use create_mci_dynamic() with the found values:
    ```python
    # Select the best image using intelligent selection
    best_image = select_best_image(images["imageList"])
    
    mci = create_mci_dynamic(
        ns_id="my-project",                          # Validated namespace
        name="my-infrastructure",
        vm_configurations=[{
            "commonImage": best_image["cspImageName"],   # From Step 4 (best image)
            "commonSpec": chosen_spec                    # From Step 2
        }]
    )
    ```
    
    **KEY RELATIONSHIPS:**
    - check_and_prepare_namespace() → namespace guidance
    - validate_namespace() → namespace verification
    - recommend_vm_spec() → spec ID (determines CSP/region) → commonSpec parameter
    - search_images() → cspImageName (in selected CSP/region) → commonImage parameter
    - All are required for successful MCI creation
    
    **NAMESPACE MANAGEMENT BENEFITS:**
    - Automatic namespace validation before MCI creation
    - Smart recommendations for namespace selection/creation
    - Prevention of MCI creation failures due to invalid namespaces
    - Unified workflow with clear error messages and suggestions
    
    **IMPORTANT NOTES:**
    - Always ensure namespace exists before MCI creation
    - Use smart functions for automated namespace handling
    - The cspImageName is provider-specific (AMI ID for AWS, Image ID for Azure, etc.)
    - commonSpec format: {provider}+{region}+{spec_name}
    - Test with hold=True first to review configuration
    
    Current namespaces: {{namespace://list}}
    
    What would you like to help you create today?
    """

logger.info("MCP server initialization complete")

