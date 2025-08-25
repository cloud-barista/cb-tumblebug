# Information: This file is part of the Tumblebug MCP server implementation.
# Run with the following command:
# uv run ./tb-mcp.py
# this server will be exposed to the MCP interface at http://127.0.0.1:8000/sse by default.

# Configuration example in Claude Desktop
# Note that Claude Desktop does not fully support streamable HTTP transport yet. (SSE transport was deprecated)
# So, the example utilizes mcp-simple-proxy.py based on the (https://gofastmcp.com/integrations/claude-desktop#remote-servers).
# cb-tumblebug/src/interface/mcp/mcp-simple-proxy.py
# In case of you are using WSL, the configuration would look like this:
# {
#   "mcpServers": {
#     "tumblebug": {
#       "command": "wsl.exe",
#       "args": [
#         "bash",
#         "-c",
#         "/home/shson/.local/bin/uv run --with fastmcp /home/shson/go/src/github.com/cloud-barista/cb-tumblebug/src/interface/mcp/mcp-simple-proxy.py"
#       ]
#     }
#   }
# }
# In case of the source code is in Windows, the configuration would look like this:
# {
#   "mcpServers": {
#     "tumblebug": {
#       "command": "uv",
#       "args": [
#         "run",
#         "--with",
#         "fastmcp",
#         "{Path to the mcp-simple-proxy.py}"
#       ]
#     }
#   }
# }

# Configuration example in VS Code.
# Note that VS Code does support streamable HTTP transport directly.
# "servers": {
#   "tumblebug": {
#     "type": "http",
#     "url": "http://127.0.0.1:8000/mcp"
#   },
# }

# For testing, you can use the Model Context Protocol Inspector.
# https://modelcontextprotocol.io/docs/tools/inspector



import os
import requests
import json
import logging
import re
from typing import Dict, List, Optional, Any, Union
from datetime import datetime, timedelta
from fastmcp import FastMCP

# This server utilizes fastmcp (https://github.com/jlowin/fastmcp)

# Configure logging - Reduce noise from HTTP connections and MCP protocol details
# Set logging level via environment variable (default: INFO)
log_level = os.environ.get("MCP_LOG_LEVEL", "DEBUG").upper()
log_level_value = getattr(logging, log_level, logging.INFO)

logging.basicConfig(
    level=log_level_value,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Reduce logging noise from external libraries (only if not in DEBUG mode)
if log_level_value > logging.DEBUG:
    # Completely suppress uvicorn access logs
    logging.getLogger("uvicorn.access").setLevel(logging.CRITICAL)
    logging.getLogger("uvicorn.access").disabled = True
    
    # Suppress other noisy loggers
    logging.getLogger("mcp.server.streamable_http").setLevel(logging.CRITICAL)
    logging.getLogger("mcp.server.streamable_http_manager").setLevel(logging.CRITICAL)
    logging.getLogger("mcp.server.lowlevel.server").setLevel(logging.CRITICAL)
    logging.getLogger("fastmcp").setLevel(logging.WARNING)
    logging.getLogger("uvicorn").setLevel(logging.WARNING)
    logging.getLogger("uvicorn.error").setLevel(logging.WARNING)
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logging.getLogger("requests").setLevel(logging.WARNING)
    
    # Override uvicorn's default access logger
    uvicorn_access = logging.getLogger("uvicorn.access")
    uvicorn_access.handlers.clear()
    uvicorn_access.propagate = False

    # Only show important MCP events
    class MCPRequestFilter(logging.Filter):
        def filter(self, record):
            message = record.getMessage()
            if "Processing request of type CallToolRequest" in message:
                # Extract tool name from the message if available
                try:
                    # Look for tool name patterns in the message
                    if "name" in message:
                        import re
                        tool_match = re.search(r"'name':\s*'([^']+)'", message)
                        if tool_match:
                            tool_name = tool_match.group(1)
                            record.msg = f"🔧 Calling tool: {tool_name}"
                            record.args = ()  # Clear args to prevent formatting mismatch
                            return True
                except:
                    pass
                # Fallback to original message
                record.msg = "🔧 Processing MCP tool request"
                record.args = ()  # Clear args to prevent formatting mismatch
                return True
            return False
    
    mcp_logger = logging.getLogger("mcp.server.lowlevel.server")
    mcp_logger.setLevel(logging.INFO)  # Allow INFO level for important events
    mcp_logger.addFilter(MCPRequestFilter())

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
logger.info(f"Logging level: {log_level}")
logger.info(f"MCP Server will start on {host}:{port}")

# Initialize MCP server
mcp = FastMCP("cb-tumblebug")

# mcp = FastMCP(name="cb-tumblebug", host=host, port=port)

# Helper function: API request wrapper
def api_request(method, endpoint, json_data=None, params=None, files=None, headers=None, timeout_override=None):
    url = f"{TUMBLEBUG_API_BASE_URL}{endpoint}"
    
    # Enhanced request configuration with improved timeout handling
    # Special handling for remote command execution endpoints
    default_timeout = (60, 600)  # 10 minutes default
    
    # Extended timeout for remote command execution (up to 20 minutes)
    if "/cmd/mci/" in endpoint or timeout_override:
        extended_timeout = timeout_override or (60, 1200)  # 20 minutes for command execution
        request_config = {
            "auth": (TUMBLEBUG_USERNAME, TUMBLEBUG_PASSWORD),
            "timeout": extended_timeout
        }
        logger.info(f"Using extended timeout for remote command: {extended_timeout[1]/60} minutes")
    else:
        request_config = {
            "auth": (TUMBLEBUG_USERNAME, TUMBLEBUG_PASSWORD),
            "timeout": default_timeout
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
            
    except requests.exceptions.Timeout as e:
        timeout_duration = request_config["timeout"][1] / 60  # Convert to minutes
        logger.error(f"Request timeout after {timeout_duration} minutes: {str(e)}")
        
        # Special message for remote command timeouts
        if "/cmd/mci/" in endpoint:
            return {
                "error": f"Remote command execution timeout after {timeout_duration} minutes",
                "error_type": "command_execution_timeout",
                "suggestion": "The remote commands are taking longer than expected. This can happen with complex installations, large downloads, or system updates. Consider breaking commands into smaller batches or checking VM resources."
            }
        else:
            return {
                "error": f"Request timeout - operation took longer than {timeout_duration} minutes",
                "error_type": "timeout",
                "suggestion": "Try breaking down the operation into smaller steps or check resource availability"
            }
    except requests.exceptions.ConnectionError as e:
        logger.error(f"Connection error: {str(e)}")
        return {
            "error": "Connection error - unable to reach CB-Tumblebug server",
            "error_type": "connection_error",
            "suggestion": "Check if CB-Tumblebug server is running and accessible"
        }
    except requests.RequestException as e:
        # Enhanced error handling for different response codes
        logger.error(f"API request error: {str(e)}")
        if hasattr(e, 'response') and e.response is not None:
            logger.error(f"Status code: {e.response.status_code}")
            logger.error(f"Response text: {e.response.text[:200]}")
            
            # Handle specific error cases
            if e.response.status_code == 400:
                try:
                    error_data = json.loads(e.response.text)
                    if "rollback completed successfully" in str(error_data):
                        return {
                            "error": "Resource creation failed and was rolled back",
                            "error_type": "resource_creation_failed",
                            "details": error_data,
                            "suggestion": "Check resource quotas, network settings, or try a different region/provider"
                        }
                except json.JSONDecodeError:
                    pass
        
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

# Helper function: Internal get namespaces (used by both get_namespaces tool and other functions)
def _internal_get_namespaces() -> Dict:
    """Internal helper function to get namespaces"""
    result = api_request("GET", "/ns")
    if "output" in result:
        return {"namespaces": result["output"]}
    return result

# Tool: Get all namespaces
@mcp.tool()
def get_namespaces() -> Dict:
    """Get list of namespaces"""
    return _internal_get_namespaces()

# Helper function: Internal get namespace (used by both get_namespace tool and other functions)
def _internal_get_namespace(ns_id: str) -> Dict:
    """Internal helper function to get specific namespace"""
    return api_request("GET", f"/ns/{ns_id}")

# Tool: Get specific namespace
@mcp.tool()
def get_namespace(ns_id: str) -> Dict:
    """Get specific namespace"""
    return _internal_get_namespace(ns_id)

# Helper function: Internal create namespace (used by both create_namespace tool and other functions)
def _internal_create_namespace(name: str, description: Optional[str] = None) -> Dict:
    """Internal helper function to create a new namespace"""
    data = {
        "name": name,
        "description": description or f"Namespace {name}"
    }
    return api_request("POST", "/ns", json_data=data)

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
    return _internal_create_namespace(name, description)

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
# Image Selection & Analysis
#####################################

# Helper function: Select best image for specific VM spec
def select_best_image_for_spec(
    image_list: List[Dict],
    vm_spec: Dict,
    requirements: Optional[Dict] = None
) -> Dict:
    """
    Select best image for a specific VM specification.
    This function considers CSP, region, architecture, and other spec-specific requirements.
    
    **CRITICAL for Multi-VM MCI Creation:**
    Each VM spec requires its own image selection because:
    - Different CSPs use different image formats (AMI vs Image ID vs etc.)
    - Same OS in different regions may have different image IDs
    - Architecture differences require different images
    - Provider-optimized images perform better than generic ones
    
    Args:
        image_list: List of image dictionaries from search_images() result
        vm_spec: VM specification dictionary containing:
            - id: spec ID (e.g., "aws+ap-northeast-2+t2.small")
            - providerName: CSP provider
            - regionName: region
            - architecture: CPU architecture
            - vCPU, memoryGiB: resource specs
        requirements: Additional requirements:
            - os_type: preferred OS (e.g., "ubuntu", "centos")
            - use_case: workload type ("web-server", "database", etc.)
            - version: specific version requirements
    
    Returns:
        Selected image with detailed spec-compatibility analysis including:
        - cspImageName: Provider-specific image identifier
        - _selection_reason: Why this image was chosen
        - _compatibility_score: How well it matches the spec
        - _spec_info: Spec details used for selection
    """
    if not image_list:
        return {"error": "No images provided for selection"}
    
    if not vm_spec:
        return {"error": "VM specification is required"}
    
    provider = vm_spec.get("providerName", "")
    region = vm_spec.get("regionName", "")
    architecture = vm_spec.get("architecture", "")
    vcpu = vm_spec.get("vCPU", 0)
    memory = vm_spec.get("memoryGiB", 0)
    
    # Extract provider and region from spec ID if not directly available
    spec_id = vm_spec.get("id", "")
    if spec_id and ("+" in spec_id):
        parts = spec_id.split("+")
        if len(parts) >= 3:
            provider = provider or parts[0]
            region = region or parts[1]
    
    requirements = requirements or {}
    preferred_os = requirements.get("os_type", "").lower()
    use_case = requirements.get("use_case", "general")
    
    # Priority 1: Always prefer isBasicImage: true if available and compatible
    basic_images = [img for img in image_list if img.get("isBasicImage", False)]
    if basic_images:
        # Check if basic images are compatible with the spec
        for img in basic_images:
            compatibility_score, compatibility_reasons = _analyze_image_spec_compatibility(img, vm_spec, requirements)
            if compatibility_score >= 80:  # High compatibility threshold
                return {
                    **img,
                    "cspImageName": img.get("cspImageName"),
                    "_selection_reason": f"isBasicImage: true with high spec compatibility ({compatibility_score}%)",
                    "_compatibility_score": compatibility_score,
                    "_compatibility_reasons": compatibility_reasons,
                    "_spec_info": {
                        "provider": provider,
                        "region": region,
                        "architecture": architecture,
                        "resources": f"{vcpu}vCPU, {memory}GB RAM"
                    },
                    "_confidence": "high",
                    "_spec_match": "excellent"
                }
    
    # Priority 2: Comprehensive spec-aware analysis
    analysis_results = []
    
    for img in image_list:
        compatibility_score, compatibility_reasons = _analyze_image_spec_compatibility(img, vm_spec, requirements)
        
        # Additional scoring based on image metadata
        name = img.get("name", "").lower()
        description = img.get("description", "").lower()
        guest_os = img.get("guestOS", "").lower()
        combined_text = f"{name} {description} {guest_os}"
        
        metadata_score = 0
        metadata_reasons = []
        
        # OS preference matching
        if preferred_os:
            if preferred_os in combined_text:
                metadata_score += 20
                metadata_reasons.append(f"Matches preferred OS: {preferred_os}")
            elif any(variant in combined_text for variant in [preferred_os[:3], preferred_os.split()[0]]):
                metadata_score += 10
                metadata_reasons.append(f"Related to preferred OS: {preferred_os}")
        
        # Architecture matching
        if architecture and architecture.lower() in combined_text:
            metadata_score += 15
            metadata_reasons.append(f"Architecture match: {architecture}")
        
        # Use case specific scoring
        if use_case == "web-server":
            if any(term in combined_text for term in ["nginx", "apache", "web", "lamp", "lemp"]):
                metadata_score += 10
                metadata_reasons.append("Web server optimized")
        elif use_case == "database":
            if any(term in combined_text for term in ["mysql", "postgres", "mongodb", "database"]):
                metadata_score += 10
                metadata_reasons.append("Database optimized")
        elif use_case == "development":
            if any(term in combined_text for term in ["dev", "development", "sdk", "tools"]):
                metadata_score += 8
                metadata_reasons.append("Development environment")
        
        # Version and stability indicators
        if any(term in combined_text for term in ["lts", "stable", "production"]):
            metadata_score += 5
            metadata_reasons.append("Stable/LTS version")
        
        total_score = compatibility_score + metadata_score
        
        analysis_results.append({
            "image": img,
            "total_score": total_score,
            "compatibility_score": compatibility_score,
            "metadata_score": metadata_score,
            "compatibility_reasons": compatibility_reasons,
            "metadata_reasons": metadata_reasons,
            "provider": provider,
            "region": region
        })
    
    # Sort by total score (descending)
    analysis_results.sort(key=lambda x: x["total_score"], reverse=True)
    
    if not analysis_results:
        # Fallback to first available image
        fallback_img = image_list[0]
        return {
            **fallback_img,
            "cspImageName": fallback_img.get("cspImageName"),
            "_selection_reason": "Fallback selection - first available image",
            "_compatibility_score": 0,
            "_spec_info": {"provider": provider, "region": region},
            "_confidence": "low",
            "_spec_match": "unknown"
        }
    
    best_result = analysis_results[0]
    best_img = best_result["image"]
    
    return {
        **best_img,
        "cspImageName": best_img.get("cspImageName"),
        "_selection_reason": f"Best spec-aware match (score: {best_result['total_score']})",
        "_compatibility_score": best_result["compatibility_score"],
        "_metadata_score": best_result["metadata_score"],
        "_compatibility_reasons": best_result["compatibility_reasons"],
        "_metadata_reasons": best_result["metadata_reasons"],
        "_spec_info": {
            "provider": provider,
            "region": region,
            "architecture": architecture,
            "resources": f"{vcpu}vCPU, {memory}GB RAM"
        },
        "_analysis_details": [f"Analyzed {len(analysis_results)} images", f"Top score: {best_result['total_score']}"],
        "_confidence": "high" if best_result["total_score"] >= 60 else "medium" if best_result["total_score"] >= 30 else "low",
        "_spec_match": "excellent" if best_result["compatibility_score"] >= 80 else "good" if best_result["compatibility_score"] >= 60 else "fair"
    }

# Helper function: Analyze image-spec compatibility
def _analyze_image_spec_compatibility(image: Dict, vm_spec: Dict, requirements: Optional[Dict] = None) -> tuple:
    """
    Analyze compatibility between an image and VM specification.
    
    Returns:
        Tuple of (compatibility_score, reasons_list)
    """
    score = 0
    reasons = []
    
    provider = vm_spec.get("providerName", "")
    region = vm_spec.get("regionName", "")
    architecture = vm_spec.get("architecture", "")
    
    # Extract from spec ID if needed
    spec_id = vm_spec.get("id", "")
    if spec_id and "+" in spec_id:
        parts = spec_id.split("+")
        if len(parts) >= 3:
            provider = provider or parts[0]
            region = region or parts[1]
    
    image_name = image.get("name", "").lower()
    image_desc = image.get("description", "").lower()
    guest_os = image.get("guestOS", "").lower()
    combined_text = f"{image_name} {image_desc} {guest_os}"
    
    # Provider compatibility (critical)
    if provider:
        # Provider-specific image naming patterns
        provider_patterns = {
            "aws": ["ami-", "amazon", "aws"],
            "azure": ["microsoft", "azure", "windows"],
            "gcp": ["google", "gcp", "debian", "ubuntu"],
            "alibaba": ["alibaba", "aliyun"],
            "tencent": ["tencent", "centos"]
        }
        
        if provider.lower() in provider_patterns:
            if any(pattern in combined_text for pattern in provider_patterns[provider.lower()]):
                score += 30
                reasons.append(f"Provider-optimized image for {provider}")
            elif provider.lower() == "aws" and image.get("cspImageName", "").startswith("ami-"):
                score += 35
                reasons.append("AWS AMI format confirmed")
        else:
            score += 20  # Default compatibility for unknown providers
            reasons.append("General compatibility assumed")
    
    # Region compatibility
    if region:
        region_lower = region.lower()
        if region_lower in combined_text:
            score += 15
            reasons.append(f"Region-specific image: {region}")
        elif any(geo in combined_text for geo in ["us", "eu", "asia", "ap-"]):
            score += 8
            reasons.append("Regional optimization detected")
    
    # Architecture compatibility
    if architecture:
        arch_lower = architecture.lower()
        if arch_lower in combined_text:
            score += 20
            reasons.append(f"Architecture match: {architecture}")
        elif "x86" in arch_lower and any(x86_variant in combined_text for x86_variant in ["x86", "amd64", "64-bit"]):
            score += 15
            reasons.append("x86 architecture compatibility")
        elif "arm" in arch_lower and "arm" in combined_text:
            score += 15
            reasons.append("ARM architecture compatibility")
    
    # OS type compatibility from requirements
    if requirements and requirements.get("os_type"):
        preferred_os = requirements["os_type"].lower()
        if preferred_os in combined_text:
            score += 15
            reasons.append(f"OS requirement satisfied: {preferred_os}")
    
    # Basic image bonus
    if image.get("isBasicImage", False):
        score += 10
        reasons.append("Official basic image")
    
    return min(score, 100), reasons  # Cap at 100%

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

@mcp.tool()
def check_and_prepare_namespace(preferred_ns_id: Optional[str] = None) -> Dict:
    """
    Check available namespaces and help user select or create one for MCI operations.
    This function provides intelligent namespace management by:
    1. Using "default" namespace as default choice if no preference specified
    2. Creating "default" namespace if it doesn't exist
    3. Listing existing namespaces for user selection
    4. Offering to create a new namespace if needed
    
    Args:
        preferred_ns_id: Preferred namespace ID to check (optional, defaults to "default")
    
    Returns:
        Namespace management guidance including:
        - available_namespaces: List of existing namespaces
        - recommendation: Suggested action
        - preferred_namespace: Information about preferred namespace
        - default_namespace_status: Status of default namespace handling
    """
    # Use "default" as preferred namespace if none specified
    if preferred_ns_id is None:
        preferred_ns_id = "default"
    
    # Get all existing namespaces
    ns_result = _internal_get_namespaces()
    
    if "error" in ns_result:
        return {
            "error": "Failed to retrieve namespaces",
            "details": ns_result["error"],
            "recommendation": "Please check your connection to Tumblebug API"
        }
    
    available_namespaces = ns_result.get("namespaces", [])
    
    result = {
        "available_namespaces": available_namespaces,
        "total_count": len(available_namespaces),
        "using_default": preferred_ns_id == "default"
    }
    
    # Check if preferred namespace (or "default") exists
    preferred_exists = any(ns.get("id") == preferred_ns_id for ns in available_namespaces)
    
    if preferred_exists:
        result["preferred_namespace"] = {
            "id": preferred_ns_id,
            "exists": True,
            "status": "ready_to_use"
        }
        result["recommendation"] = f"Namespace '{preferred_ns_id}' exists and is ready to use for MCI creation."
        
        if preferred_ns_id == "default":
            result["default_namespace_status"] = "exists_and_ready"
    else:
        result["preferred_namespace"] = {
            "id": preferred_ns_id,
            "exists": False,
            "status": "needs_creation"
        }
        
        if preferred_ns_id == "default":
            # Automatically create "default" namespace if it doesn't exist
            try:
                create_result = _internal_create_namespace_with_validation(
                    name="default",
                    description="Default namespace for MCI operations"
                )
                
                if "error" not in create_result:
                    result["preferred_namespace"]["exists"] = True
                    result["preferred_namespace"]["status"] = "created_automatically"
                    result["default_namespace_status"] = "created_automatically"
                    result["recommendation"] = "Default namespace 'default' was created automatically and is ready to use for MCI creation."
                else:
                    result["default_namespace_status"] = "creation_failed"
                    result["recommendation"] = f"Failed to create default namespace: {create_result.get('error', 'Unknown error')}. Please create it manually or use a different namespace."
                    
            except Exception as e:
                result["default_namespace_status"] = "creation_error"
                result["recommendation"] = f"Error creating default namespace: {str(e)}. Please create it manually or use a different namespace."
        else:
            result["recommendation"] = f"Preferred namespace '{preferred_ns_id}' does not exist. You can create it using create_namespace() function."
    
    # Provide additional guidance based on available namespaces (only if not using default or default creation failed)
    if not (preferred_ns_id == "default" and result.get("preferred_namespace", {}).get("exists", False)):
        if len(available_namespaces) == 0:
            if preferred_ns_id != "default":
                result["additional_guidance"] = "No namespaces found. Consider using 'default' namespace or create a new one."
            result["suggested_action"] = "create_namespace"
        elif len(available_namespaces) == 1:
            single_ns = available_namespaces[0]
            ns_id = single_ns.get('id', 'unknown')
            if preferred_ns_id != ns_id:
                result["additional_guidance"] = f"One namespace available: '{ns_id}'. You can use this or continue with '{preferred_ns_id}'."
            result["suggested_namespace"] = single_ns.get("id", "unknown")
            result["suggested_action"] = "use_existing_or_create_new"
        else:
            result["additional_guidance"] = f"Multiple namespaces available ({len(available_namespaces)}). You can select one or continue with '{preferred_ns_id}'."
            result["suggested_action"] = "select_existing_or_create_new"
            result["namespace_options"] = [ns.get("id", "unknown") for ns in available_namespaces]
    
    return result

# Helper function: Internal validate namespace (used by both validate_namespace tool and other functions)
def _internal_validate_namespace(ns_id: str) -> Dict:
    """Internal helper function to validate if a namespace exists"""
    try:
        ns_info = _internal_get_namespace(ns_id)
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
    return _internal_validate_namespace(ns_id)

# Helper function: Internal create namespace with validation (used by both create_namespace_with_validation tool and other functions)
def _internal_create_namespace_with_validation(name: str, description: Optional[str] = None) -> Dict:
    """Internal helper function to create namespace with validation"""
    # First check if namespace already exists
    validation = _internal_validate_namespace(name)
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
        result = _internal_create_namespace(name, description)
        if "error" in result:
            return {
                "created": False,
                "namespace_id": name,
                "error": result["error"],
                "suggestion": "Please check the namespace name and try again"
            }
        
        # Validate the created namespace
        validation = _internal_validate_namespace(name)
        
        # Store namespace creation in memory
        _store_interaction_memory(
            user_request=f"Create namespace '{name}' with description '{description or 'N/A'}'",
            llm_response=f"Successfully created namespace '{name}'",
            operation_type="namespace_management",
            context_data={"namespace_id": name, "description": description},
            status="completed"
        )
        
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
    return _internal_create_namespace_with_validation(name, description)


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
def release_resources(ns_id: str, force_release: bool = False) -> Dict:
    """
    Release all shared resources for a specific namespace.
    This includes VNet, SecurityGroup, and SSHKey resources.
    This operation is irreversible and should be used with caution.
    
    ⚠️  IMPORTANT: Only use this when no MCIs exist in the namespace or you're sure the resources are not needed.
    
    **When to use:**
    - After deleting the last MCI in a namespace
    - When completely cleaning up a namespace
    - When you're certain no future MCIs will reuse these resources
    
    **When NOT to use:**
    - Other MCIs exist in the same namespace
    - You plan to create new MCIs that could reuse network configurations
    - You're unsure about resource dependencies
    
    Args:
        ns_id: Namespace ID
        force_release: Safety flag to prevent accidental deletion (must be True to proceed)
    
    Returns:
        Resource release result with safety checks and guidance
    """
    # Safety check: require explicit confirmation
    if not force_release:
        return {
            "error": "Force release flag required for safety",
            "requirement": "Set force_release=True to proceed with shared resources deletion",
            "warning": "⚠️  This will permanently delete ALL shared resources (VNet, SecurityGroup, SSHKey) in the namespace",
            "safety_checklist": [
                "✓ Confirm no MCIs exist in this namespace",
                "✓ Confirm no other infrastructure depends on these resources", 
                "✓ Confirm you won't need these network configurations for future MCIs",
                "✓ Have backups of any important configurations"
            ],
            "check_command": f"get_mci_list('{ns_id}') # Verify no MCIs exist before proceeding"
        }
    
    # Pre-deletion checks
    try:
        # Check for existing MCIs
        mci_list_result = api_request("GET", f"/ns/{ns_id}/mci")
        if isinstance(mci_list_result, dict) and "mci" in mci_list_result and len(mci_list_result["mci"]) > 0:
            active_mcis = [mci.get("id", "unknown") for mci in mci_list_result["mci"]]
            return {
                "error": "Cannot release resources while MCIs exist",
                "active_mcis": active_mcis,
                "recommendation": "Delete all MCIs first using delete_mci()",
                "safety_note": "Shared resources are being used by active MCIs"
            }
        
        # Get current resources for reporting
        try:
            vnets_result = api_request("GET", f"/ns/{ns_id}/resources/vNet")
            security_groups_result = api_request("GET", f"/ns/{ns_id}/resources/securityGroup")
            ssh_keys_result = api_request("GET", f"/ns/{ns_id}/resources/sshKey")
            
            vnets = vnets_result.get("vNet", []) if isinstance(vnets_result, dict) else []
            security_groups = security_groups_result.get("securityGroup", []) if isinstance(security_groups_result, dict) else []
            ssh_keys = ssh_keys_result.get("sshKey", []) if isinstance(ssh_keys_result, dict) else []
            
            resources_to_delete = {
                "vNets": [vnet.get("id", "unknown") for vnet in vnets] if vnets else [],
                "securityGroups": [sg.get("id", "unknown") for sg in security_groups] if security_groups else [],
                "sshKeys": [key.get("id", "unknown") for key in ssh_keys] if ssh_keys else []
            }
        except Exception as e:
            resources_to_delete = {"note": f"Could not enumerate resources before deletion: {str(e)}"}
        
    except Exception as e:
        return {
            "error": f"Pre-deletion checks failed: {str(e)}",
            "recommendation": "Check namespace status and try again"
        }
    
    # Proceed with deletion
    result = api_request("DELETE", f"/ns/{ns_id}/sharedResources")
    
    # Enhance result with context information
    if isinstance(result, dict) and "error" not in result:
        # Store deletion in memory
        _store_interaction_memory(
            user_request=f"Release shared resources in namespace '{ns_id}'",
            llm_response=f"Successfully released shared resources in namespace '{ns_id}'",
            operation_type="resource_management",
            context_data={
                "namespace_id": ns_id, 
                "operation": "release_shared_resources",
                "resources_deleted": resources_to_delete
            },
            status="completed"
        )
        
        result["deletion_summary"] = {
            "namespace": ns_id,
            "resources_deleted": resources_to_delete,
            "status": "All shared resources have been permanently deleted",
            "note": "This namespace is now clean and ready for new MCI deployments with fresh resources"
        }
    
    return result

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
    
    **CRITICAL for osType discovery:** Use this function to see all available OS types and versions
    that can be used in the search_images() osType parameter.
    
    **Workflow:**
    1. Call this function to discover available osType values
    2. Use search_images() with either:
       - Simple OS name from osType list (e.g., "ubuntu", "centos")
       - Full OS + version combination (e.g., "ubuntu 22.04", "centos 7")
    
    Args:
        ns_id: Namespace ID (typically "system" for system images)
    
    Returns:
        Available search options including:
        - osType: Available OS types and versions (e.g., ["ubuntu", "ubuntu 22.04", "centos", "centos 7", "windows server 2019"])
        - osArchitecture: Available OS architectures (e.g., ["x86_64", "arm64"])
        - providerName: Available cloud providers (e.g., ["aws", "azure", "gcp", "ncp"])
        - regionName: Available regions (e.g., ["ap-northeast-2", "us-east-1", "koreacentral"])
    """
    return api_request("GET", f"/ns/{ns_id}/resources/searchImageOptions")

# Tool: Search images
@mcp.tool()
def search_images(
    ns_id: str = "system",
    matched_spec_id: Optional[str] = None,
    os_type: Optional[str] = None,
    os_architecture: Optional[str] = None,
    provider_name: Optional[str] = None,
    region_name: Optional[str] = None,
    is_gpu_image: Optional[bool] = None,
    is_kubernetes_image: Optional[bool] = None,
    include_basic_image_only: Optional[bool] = None,
    detail_search_keys: Optional[List[str]] = None,
    max_results: Optional[Union[int, str]] = None
) -> Dict:
    """
    Search for available images based on specific criteria.
    
    **SIMPLIFIED WORKFLOW with MatchedSpecId:**
    When you have a specific VM spec, simply use:
    1. search_images(matched_spec_id="aws+ap-northeast-2+t2.small", os_type="ubuntu")
    2. The system automatically applies provider, region, architecture from the spec
    3. Get compatible images filtered for that specific spec
    
    **TRADITIONAL WORKFLOW:**
    1. First call get_image_search_options() to see available search parameters
    2. Use this function to search for images matching your requirements
    3. From the results, identify the 'cspImageName' of your desired image
    4. Use the 'cspImageName' as 'imageId' parameter in create_mci_dynamic()
    
    Example workflows:
    A) **RECOMMENDED - With MatchedSpecId (simple OS type):**
       search_images(matched_spec_id="aws+ap-northeast-2+t2.small", os_type="ubuntu")
       
    B) **RECOMMENDED - With MatchedSpecId (OS + version):**
       search_images(matched_spec_id="aws+ap-northeast-2+t2.small", os_type="ubuntu 22.04")
       
    C) **Advanced - With detail search keys:**
       search_images(matched_spec_id="aws+ap-northeast-2+t2.small", detail_search_keys=["tensorflow", "2.17"])
       
    D) **Traditional approach (simple OS type):**
       search_images(provider_name="aws", region_name="ap-northeast-2", os_type="centos")
       
    E) **Traditional approach (OS + version):**
       search_images(provider_name="aws", region_name="ap-northeast-2", os_type="ubuntu 22.04")
       
    F) **Basic images only:**
       search_images(provider_name="aws", region_name="ap-northeast-2", include_basic_image_only=True)
    
    Args:
        ns_id: Namespace ID (typically "system" for system images)
        matched_spec_id: VM spec ID for automatic provider/region/architecture mapping (RECOMMENDED)
        os_type: OS type filter with flexible options:
                - Simple OS name: "ubuntu", "centos", "windows", "debian", "rhel"
                - Full OS + version: "ubuntu 22.04", "centos 7", "windows server 2019"
                - Use get_image_search_options() to see all available osType values
        os_architecture: OS architecture filter (auto-applied when using matched_spec_id)
        provider_name: Cloud provider filter (auto-applied when using matched_spec_id)
        region_name: Region filter (auto-applied when using matched_spec_id)
        is_gpu_image: Filter for GPU-optimized images (images ready for GPU usage)
        is_kubernetes_image: Filter for Kubernetes-specialized images
        include_basic_image_only: Return only basic OS distributions without additional applications
        detail_search_keys: Keywords for detailed search (space-separated for AND condition)
        max_results: Maximum number of images to return
    
    Returns:
        Enhanced search results with LLM guidance for empty results:
        - imageList: List of matching images compatible with the spec (if matched_spec_id used)
        - imageCount: Total number of images found
        - search_status: Status indicator ("success" or "no_results") 
        - llm_guidance: Structured guidance for LLM agents when no images are found, including:
          - status: Detailed status description
          - message: Human-readable description of the result
          - search_criteria_used: List of criteria that were applied
          - alternative_suggestions: Specific alternative search strategies
          - next_steps: Recommended actions (especially recommend_vm_spec() for different specs)
          - common_solutions: General troubleshooting approaches
        - Each image (when found) includes:
          - cspImageName: CSP-specific image identifier (CRITICAL for MCI creation)
          - description: Image description
          - guestOS: Guest operating system
          - architecture: OS architecture
          - creationDate: Image creation date
          - isBasicImage: Boolean indicating if this is a basic OS image (PRIORITY indicator)
          
    Important: 
    - **NEW**: Use matched_spec_id for automatic spec-compatible filtering
    - **CSP-specific filtering**: NCP specs automatically filter by CorrespondingImageIds
    - **Detail search**: Use detail_search_keys for advanced keyword-based searching
    - **Basic images**: Use include_basic_image_only=True for clean OS installations only
    - The 'cspImageName' from search results becomes the 'imageId' parameter when creating MCIs
    - For optimal image selection, use select_best_image() helper function
    - **CRITICAL**: Each VM spec requires its own image search for proper compatibility
    - **LLM GUIDANCE**: Empty results include structured guidance to prevent "Canceled" responses
    - **SPEC ALTERNATIVES**: When matched_spec_id yields no results, guides to use recommend_vm_spec()
    """
    # Handle type conversion for max_results (MCP client may send string)
    if max_results is not None:
        if isinstance(max_results, str):
            try:
                max_results = int(max_results)
            except ValueError:
                logger.warning(f"Invalid max_results value: {max_results}, using default")
                max_results = None
    
    # Build request data according to model.SearchImageRequest spec
    data = {}
    
    # Add MatchedSpecId for automatic spec-based filtering
    if matched_spec_id:
        data["matchedSpecId"] = matched_spec_id
    
    # Add other search criteria
    if os_type:
        data["osType"] = os_type
    
    # Add default x86_64 architecture if not specified by user and no matched_spec_id
    if os_architecture:
        data["osArchitecture"] = os_architecture
    elif not matched_spec_id:  # Only set default if not using matched_spec_id
        data["osArchitecture"] = "x86_64"
    
    if provider_name:
        data["providerName"] = provider_name
    
    if region_name:
        data["regionName"] = region_name
    
    if is_gpu_image is not None:
        data["isGPUImage"] = is_gpu_image
    
    if is_kubernetes_image is not None:
        data["isKubernetesImage"] = is_kubernetes_image
    
    if include_basic_image_only is not None:
        data["includeBasicImageOnly"] = include_basic_image_only
    
    if detail_search_keys:
        data["detailSearchKeys"] = detail_search_keys
    
    if max_results is not None:
        data["maxResults"] = max_results
    
    # Debug log the request data
    logger.debug(f"🔍 SearchImage request data: {json.dumps(data, indent=2)}")
    
    # Make API request
    try:
        response = api_request("POST", f"/ns/{ns_id}/resources/searchImage", json_data=data)
        
        # Debug log the response
        logger.debug(f"🔍 SearchImage API response type: {type(response)}")
        if isinstance(response, dict):
            logger.debug(f"🔍 SearchImage response keys: {list(response.keys())}")
            if "imageList" in response:
                logger.debug(f"🔍 SearchImage found {len(response['imageList'])} images")
        
        # Validate response structure
        if not isinstance(response, dict):
            logger.error(f"Invalid response format: expected dict, got {type(response)}")
            return {
                "error": "Invalid response format from API",
                "imageList": [],
                "imageCount": 0
            }
        
        # Handle API error responses
        if "error" in response:
            logger.error(f"API error in search_images: {response.get('error', 'Unknown error')}")
            return response
        
        # Ensure imageList exists in response
        if "imageList" not in response:
            response["imageList"] = []
        
        # Ensure imageCount exists in response
        if "imageCount" not in response:
            response["imageCount"] = len(response.get("imageList", []))
            
    except Exception as e:
        logger.error(f"Exception in search_images: {str(e)}")
        return {
            "error": f"Failed to search images: {str(e)}",
            "imageList": [],
            "imageCount": 0
        }
    
    # Add helper information for optimal image selection
    if "imageList" in response and response["imageList"]:
        # Add context about matched spec filtering
        if matched_spec_id:
            logger.info(f"🎯 Found {len(response['imageList'])} images compatible with spec: {matched_spec_id}")
        
        # Add basic image prioritization hint
        basic_images = [img for img in response["imageList"] if img.get("isBasicImage", False)]
        if basic_images:
            logger.info(f"💡 Found {len(basic_images)} basic images (highest priority) out of {len(response['imageList'])} total images")
        
        # Add first few images preview for LLM context
        preview_count = min(3, len(response["imageList"]))
        logger.info(f"📋 First {preview_count} images preview:")
        for i, img in enumerate(response["imageList"][:preview_count]):
            basic_marker = " 🌟" if img.get("isBasicImage", False) else ""
            logger.info(f"  {i+1}. {img.get('cspImageName', 'N/A')}: {img.get('description', 'No description')[:80]}...{basic_marker}")
    else:
        # Enhanced handling for empty results with structured LLM guidance
        search_criteria = []
        if matched_spec_id:
            search_criteria.append(f"matched_spec_id={matched_spec_id}")
        if os_type:
            search_criteria.append(f"os_type={os_type}")
        if provider_name:
            search_criteria.append(f"provider={provider_name}")
        if region_name:
            search_criteria.append(f"region={region_name}")
        
        criteria_str = ", ".join(search_criteria) if search_criteria else "specified criteria"
        
        # Log when no images are found
        if matched_spec_id:
            logger.warning(f"⚠️ No images found for spec: {matched_spec_id}")
        else:
            logger.warning(f"⚠️ No images found for {criteria_str}")
        
        # Add structured guidance for LLM agents to prevent "Canceled: Canceled" responses
        response["search_status"] = "no_results"
        response["imageList"] = []
        response["imageCount"] = 0
        
        # Generate specific guidance based on search context
        guidance = {
            "status": "no_images_found",
            "message": f"No images found matching criteria: {criteria_str}",
            "search_criteria_used": search_criteria
        }
        
        # Provide specific suggestions based on matched_spec_id usage
        if matched_spec_id:
            # Extract provider and region from spec_id for targeted suggestions
            spec_parts = matched_spec_id.split('+') if '+' in matched_spec_id else []
            
            guidance["alternative_suggestions"] = [
                "Try using recommend_vm_spec() to get alternative VM specifications",
                "Use a different spec_id from your previous recommend_vm_spec() results",
                f"Try searching without matched_spec_id using traditional parameters",
                "Try setting include_basic_image_only=True for basic OS images only"
            ]
            
            if len(spec_parts) >= 2:
                spec_provider = spec_parts[0]
                spec_region = spec_parts[1]
                guidance["alternative_suggestions"].extend([
                    f"Try provider_name='{spec_provider}' and region_name='{spec_region}' instead of matched_spec_id",
                    f"Search for images in different regions for provider '{spec_provider}'"
                ])
            
            guidance["next_steps"] = [
                "Call recommend_vm_spec() to get different VM specifications",
                "Select alternative spec_id from recommend_vm_spec() results",
                "Try broader search criteria without version-specific OS requirements",
                "Consider using get_image_search_options() to see available parameters"
            ]
        else:
            # Traditional search suggestions
            guidance["alternative_suggestions"] = [
                "Try broadening search criteria (remove version numbers from os_type)",
                "Try searching in different regions or providers", 
                "Try setting include_basic_image_only=True",
                "Use get_image_search_options() to see available search parameters"
            ]
            
            guidance["next_steps"] = [
                "Broaden your search criteria or try different OS types",
                "Check available providers/regions with get_image_search_options()",
                "Consider using matched_spec_id with recommend_vm_spec() for better results"
            ]
        
        # Add OS-specific suggestions
        if os_type and "22.04" in str(os_type):
            guidance["alternative_suggestions"].insert(0, "Try searching with 'ubuntu 20.04' or just 'ubuntu' for broader results")
        elif os_type and "ubuntu" in str(os_type).lower():
            guidance["alternative_suggestions"].insert(0, "Try searching with 'centos', 'debian', or 'rhel' as alternative OS types")
        
        guidance["common_solutions"] = [
            "Use recommend_vm_spec() first to get valid specifications",
            "Try include_basic_image_only=True parameter",
            "Search without version numbers in os_type (e.g., 'ubuntu' instead of 'ubuntu 22.04')",
            "Verify provider/region availability with get_image_search_options()"
        ]
        
        response["llm_guidance"] = guidance
        
        # Log helpful guidance for debugging
        logger.info(f"💡 LLM Guidance: {guidance['message']}")
        if matched_spec_id:
            logger.info(f"🔧 Suggestion: Try recommend_vm_spec() for alternative specifications")
        else:
            logger.info(f"🔧 Suggestion: {guidance['alternative_suggestions'][0] if guidance['alternative_suggestions'] else 'Broaden search criteria'}")
    
    return response

# Tool: Get VM spec recommendation options
@mcp.tool()
def get_recommend_spec_options() -> Dict:
    """
    Get available options for VM spec recommendations including filter metrics, priority options, and example values.
    
    **IMPORTANT: Always call this function BEFORE using recommend_vm_spec() to:**
    - Check available filter metrics and valid operators
    - Get actual values for providers, regions, architectures, etc.
    - See example policies and parameter formats
    - Understand priority options and their required parameters
    
    This helps prevent failures in recommend_vm_spec() due to invalid parameters.
    
    Returns:
        Dictionary containing:
        - filter.availableMetrics: List of metrics you can filter by
        - filter.availableValues: Actual values for providers, regions, etc.
        - filter.examplePolicies: Example filter configurations
        - priority.availableMetrics: Priority options (cost, performance, location, latency, random)
        - priority.examplePolicies: Example priority configurations
        - priority.parameterOptions: Required parameters for location/latency priorities
        - limit: Suggested limit values
    """
    return api_request("GET", "/recommendSpecOptions")

# Tool: Recommend VM spec
@mcp.tool()
def recommend_vm_spec(
    filter_policies: Dict[str, Any] = None,
    limit: Union[str, int] = "50",
    priority_policy: str = "location",
    latitude: Optional[float] = None,
    longitude: Optional[float] = None,
    include_full_details: bool = False
) -> Any:
    """
    ⚠️  IMPORTANT PREREQUISITE: Call get_recommend_spec_options() FIRST!
    
    🚨 CRITICAL: This is the ONLY valid source for VM specification IDs in MCI creation.
    NEVER create or guess spec IDs manually - they MUST come from this function's response.
    
    **MANDATORY WORKFLOW:**
    1. 🔥 Call get_recommend_spec_options() FIRST to get:
       - Available filter metrics and valid operators
       - Actual values for providers, regions, architectures
       - Example policies and parameter formats
       - Valid priority options and required parameters
    2. Use the returned information to build valid filter_policies
    3. Then call this function with validated parameters
    
    **Common failure prevention (use get_recommend_spec_options()):**
    - ❌ Using invalid metric names in filter_policies
    - ❌ Using non-existent provider names, regions, or architectures  
    - ❌ Wrong operator formats or parameter structures
    - ❌ Invalid priority policy configurations
    
    Recommend VM specifications for MCI creation with location-based priority support.
    
    **🔥 MANDATORY FOR LOCATION-BASED REQUESTS:**
    When users mention ANY geographic location (country, city, region), you MUST:
    1. Determine the approximate latitude/longitude coordinates for that location using your geographic knowledge
    2. Use priority_policy="location" with those coordinates
    3. Use the returned spec IDs exactly as provided - NEVER modify them
    4. Explain your coordinate reasoning briefly to the user
    
    **🌍 GEOGRAPHIC COORDINATE ANALYSIS:**
    - Apply your knowledge of world geography to determine appropriate coordinates
    - Use major metropolitan area coordinates for regional requests
    - Consider time zones and geographic proximity when selecting coordinates
    - Explain coordinate selection reasoning to users for transparency
    
    **WORKFLOW INTEGRATION:**
    1. Use this function to find specs with location priority → get specification IDs
    2. Use search_images() to find suitable images for the specs' CSPs/regions
    3. Use both values in create_mci_dynamic():
       - specId: specification ID from this function (NEVER modify)
       - imageId: cspImageName from search_images()
    
    **Example for Location-Based Request:**
    ```python
    # User: "Deploy servers in Silicon Valley"
    specs = recommend_vm_spec(
        filter_policies={
            "vCPU": {"min": 2, "max": 8},
            "memoryGiB": {"min": 4, "max": 16}
        },
        priority_policy="location",
        latitude=37.4419,   # Silicon Valley coordinates
        longitude=-122.1430,
        limit="10"
    )
    
    # Use ONLY the returned spec IDs:
    for spec in specs["recommended_specs"]:
        spec_id = spec["id"]  # e.g., "aws+us-west-1+t3.medium"
        # Use this exact spec_id in create_mci_dynamic() - NEVER modify
    ```
    
    Args:
        filter_policies: Filter criteria including:
                        - vCPU: {"min": 2, "max": 8} for CPU requirements
                        - memoryGiB: {"min": 4, "max": 16} for memory requirements
                        - ProviderName: "aws", "azure", "gcp" for specific provider
                        - CspSpecName: Specific CSP spec name
                        - RegionName: Specific region
                        - Architecture: CPU architecture (defaults to "x86_64" if not specified)
        limit: Maximum number of recommendations (default: "50")
        priority_policy: Optimization strategy:
                        - "location": 🔥 REQUIRED for geographic requests - Prioritize proximity to coordinates
                        - "cost": Prioritize lower cost
                        - "performance": Prioritize higher performance
        latitude: 🔥 REQUIRED for location priority - Latitude coordinate for the desired location
        longitude: 🔥 REQUIRED for location priority - Longitude coordinate for the desired location
        include_full_details: Whether to include detailed technical specifications (default: False)
    
    Returns:
        🚨 CRITICAL: Use the returned spec IDs exactly as provided
        Recommended VM specifications including:
        - id: Specification ID (use as 'specId' in create_mci_dynamic() WITHOUT modification)
        - vCPU: Number of virtual CPUs
        - memoryGiB: Memory in GB
        - costPerHour: Estimated hourly cost (if -1, pricing information is unavailable)
        - providerName: Cloud provider
        - regionName: Region name
        
    **🚨 SPEC ID VALIDATION RULES:**
    ✅ ALWAYS use the exact 'id' field from this function's response
    ✅ NEVER modify, concatenate, or reconstruct spec IDs
    ✅ If no specs match requirements, adjust filter_policies and try again
    ❌ NEVER create spec IDs like "tencent+na-siliconvalley+bf1.large8" manually
    ❌ NEVER guess spec formats based on patterns
    
    **CRITICAL for MCI Creation:**
    The 'id' field from results becomes the 'specId' parameter in create_mci_dynamic().
    Format is typically: {provider}+{region}+{spec_name} but MUST be from API response.
    """
    # Handle type conversion for limit (API expects string but MCP client may send int)
    if isinstance(limit, int):
        limit = str(limit)
    elif limit is None:
        limit = "50"  # Default value
    
    # Configure filter policies according to API spec
    if filter_policies is None:
        filter_policies = {}
    
    # Add default x86_64 architecture filter if not specified by user
    if "Architecture" not in filter_policies and "architecture" not in filter_policies:
        filter_policies["Architecture"] = "x86_64"
    
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
    
    # Build the request data according to model.RecommendSpecReq
    data = {
        "filter": {
            "policy": policies
        },
        "limit": str(limit),
        "priority": {
            "policy": priority_policies
        }
    }
    
    # Make API request
    raw_response = api_request("POST", "/recommendSpec", json_data=data)
    
    # Summarize response to reduce token usage
    return _summarize_vm_specs(raw_response, include_details=include_full_details)

# Helper functions for formatting user-friendly review results
def _format_vm_summary_for_user(vm_reviews: List[Dict]) -> str:
    """Format VM review summary for user display"""
    if not vm_reviews:
        return "No VM configurations provided"
    
    summary_lines = []
    for i, vm_review in enumerate(vm_reviews, 1):
        provider = vm_review.get("providerName", "Unknown")
        region = vm_review.get("regionName", "Unknown")
        status = vm_review.get("status", "Unknown")
        can_create = vm_review.get("canCreate", False)
        
        status_icon = "✅" if can_create else "❌"
        summary_lines.append(f"  {i}. {provider} ({region}) - {status} {status_icon}")
    
    return "\n".join(summary_lines)

def _format_warnings_for_user(vm_reviews: List[Dict]) -> str:
    """Format VM warnings for user display"""
    warning_lines = []
    for i, vm_review in enumerate(vm_reviews, 1):
        if vm_review.get("status") == "Warning":
            provider = vm_review.get("providerName", "Unknown")
            message = vm_review.get("message", "No details available")
            warning_lines.append(f"  {i}. {provider}: {message}")
    
    return "\n".join(warning_lines) if warning_lines else "No warnings found"

def _format_errors_for_user(vm_reviews: List[Dict]) -> str:
    """Format VM errors for user display"""
    error_lines = []
    for i, vm_review in enumerate(vm_reviews, 1):
        if vm_review.get("status") == "Error" or not vm_review.get("canCreate", True):
            provider = vm_review.get("providerName", "Unknown")
            message = vm_review.get("message", "No details available")
            error_lines.append(f"  {i}. {provider}: {message}")
    
    return "\n".join(error_lines) if error_lines else "No errors found"

# Helper function: Internal MCI dynamic validation (used by both review and create functions)
def _internal_review_mci_dynamic(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI created dynamically via MCP",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_command: Optional[Dict] = None,
    hold: bool = False
) -> Dict:
    """Internal helper function to review MCI dynamic configuration"""
    # Build request data according to model.TbMciDynamicReq spec
    data = {
        "name": name,
        "description": description,
        "subGroups": vm_configurations
    }
    
    # Add optional parameters
    if system_label:
        data["systemLabel"] = system_label
    if label:
        data["label"] = label
    if post_command:
        data["postCommand"] = post_command
    if hold:
        data["hold"] = hold
    
    # Make API request to review endpoint
    url = f"/ns/{ns_id}/mciDynamicReview"
    result = api_request("POST", url, json_data=data)
    
    # 🔍 ENHANCED: Add historical risk analysis for each VM configuration
    if isinstance(result, dict) and "vmReviews" in result:
        enhanced_risk_analysis = []
        
        for i, vm_review in enumerate(result["vmReviews"]):
            vm_config = vm_configurations[i] if i < len(vm_configurations) else {}
            spec_id = vm_config.get("specId")
            image_name = vm_config.get("imageId")
            
            # Get historical risk analysis for this spec
            risk_analysis = {}
            if spec_id:
                try:
                    # Get basic risk analysis
                    risk_result = analyze_provisioning_risk(spec_id, image_name)
                    if "error" not in risk_result:
                        risk_analysis["basic_risk"] = risk_result
                        
                        # Add detailed risk if high or medium risk detected
                        risk_level = risk_result.get("riskLevel", "unknown")
                        if risk_level in ["high", "medium"]:
                            detailed_risk = get_detailed_provisioning_risk(spec_id, image_name)
                            if "error" not in detailed_risk:
                                risk_analysis["detailed_risk"] = detailed_risk
                        
                        # Get provisioning history for context
                        history = get_provisioning_history(spec_id)
                        if "error" not in history:
                            risk_analysis["history"] = history
                            
                except Exception as e:
                    risk_analysis["risk_analysis_error"] = f"Could not analyze risk: {str(e)}"
            
            # Add risk analysis to VM review
            vm_review["historical_risk_analysis"] = risk_analysis
            enhanced_risk_analysis.append({
                "vm_index": i,
                "spec_id": spec_id,
                "risk_level": risk_analysis.get("basic_risk", {}).get("riskLevel", "unknown"),
                "failure_rate": risk_analysis.get("basic_risk", {}).get("failureRate", "N/A"),
                "recommendations": risk_analysis.get("basic_risk", {}).get("recommendations", [])
            })
        
        # Add overall risk summary
        result["overall_risk_assessment"] = {
            "risk_summary": enhanced_risk_analysis,
            "high_risk_vms": [r for r in enhanced_risk_analysis if r["risk_level"] == "high"],
            "medium_risk_vms": [r for r in enhanced_risk_analysis if r["risk_level"] == "medium"],
            "risk_guidance": "Check individual VM risk analysis for detailed recommendations"
        }
        
        # Update overall validation status based on risk analysis
        high_risk_count = len([r for r in enhanced_risk_analysis if r["risk_level"] == "high"])
        if high_risk_count > 0:
            if "issues" not in result:
                result["issues"] = []
            result["issues"].append(f"⚠️ {high_risk_count} VM(s) have HIGH provisioning failure risk based on historical data")
            
            if "recommendations" not in result:
                result["recommendations"] = []
            result["recommendations"].append("Consider using alternative specs with lower failure rates")
            result["recommendations"].append("Use get_detailed_provisioning_risk() for specific guidance on high-risk VMs")
    
    # Enhance result with additional guidance based on review results
    if isinstance(result, dict):
        # 🚀 ENHANCED: Analyze vmReviews for specific guidance and reconfiguration needs
        vm_reviews = result.get("vmReviews", [])
        failed_vms = []
        provisioning_issues = []
        reconfiguration_needed = False
        
        # Analyze each VM review for specific issues
        for i, vm_review in enumerate(vm_reviews):
            vm_config = vm_configurations[i] if i < len(vm_configurations) else {}
            can_create = vm_review.get("canCreate", True)
            errors = vm_review.get("errors", [])
            warnings = vm_review.get("warnings", [])
            provider_name = vm_review.get("providerName", "")
            
            if not can_create:
                failed_vms.append({
                    "vm_index": i,
                    "vm_name": vm_config.get("name", f"vm-{i}"),
                    "provider": provider_name,
                    "errors": errors,
                    "warnings": warnings,
                    "spec_id": vm_config.get("specId", "")
                })
                
                # Check for specific provisioning issues that need reconfiguration
                for error in errors:
                    if any(keyword in error.lower() for keyword in ["not available", "cannot be provisioned", "not supported"]):
                        provisioning_issues.append({
                            "vm_index": i,
                            "provider": provider_name,
                            "issue": error,
                            "spec_id": vm_config.get("specId", "")
                        })
                        reconfiguration_needed = True
        
        # Add enhanced analysis results
        if failed_vms or provisioning_issues:
            result["_analysis"] = {
                "failed_vms": failed_vms,
                "provisioning_issues": provisioning_issues,
                "reconfiguration_needed": reconfiguration_needed,
                "total_failed": len(failed_vms)
            }
        
        # Provide specific guidance based on validation results
        creation_viable = result.get("creationViable", False)
        if creation_viable:
            result["_guidance"] = "✅ Validation passed! You can proceed with create_mci_dynamic() using the same parameters."
            result["_next_step"] = f"create_mci_dynamic(ns_id='{ns_id}', name='{name}', vm_configurations=<same_configurations>)"
            
            # Add warnings for hold mode or other special cases
            if hold:
                result["_guidance"] += "\n⚠️ Note: VMs requiring manual deployment will need completion after hold mode."
        else:
            result["_guidance"] = "❌ Validation failed. Please address the issues before proceeding with MCI creation."
            
            # Provide specific guidance for failed VMs
            if failed_vms:
                result["_guidance"] += f"\n🚫 {len(failed_vms)} VM(s) cannot be created due to configuration or provider issues:"
                
                for failed_vm in failed_vms:
                    vm_errors = '; '.join(failed_vm['errors'])
                    result["_guidance"] += f"\n  • VM {failed_vm['vm_index']} ({failed_vm['provider']}): {vm_errors}"
            
            # Provide reconfiguration guidance
            if reconfiguration_needed:
                result["_guidance"] += "\n\n💡 RECONFIGURATION NEEDED:"
                result["_guidance"] += "\n  1. Use recommend_vm_spec() to find alternative specifications from different providers"
                result["_guidance"] += "\n  2. Update vm_configurations with working specs and run review again"
                result["_guidance"] += "\n  3. Consider using hold=True for providers requiring manual deployment"
                
                # Add specific provider guidance from review results
                for issue in provisioning_issues:
                    if "hold" in issue["issue"].lower():
                        result["_guidance"] += f"\n  • For {issue['provider']}: Set hold=True in create_mci_dynamic()"
                    elif "not available" in issue["issue"].lower():
                        result["_guidance"] += f"\n  • For {issue['provider']}: Replace with alternative provider (AWS, Azure, GCP, etc.)"
            
            result["_next_step"] = "Fix the reported issues and run review_mci_dynamic_request() again."
        
        # Add summary of vmReviews for LLM reference
        if vm_reviews:
            result["_vm_summary"] = {
                "total_vms": len(vm_reviews),
                "successful_vms": len([vm for vm in vm_reviews if vm.get("canCreate", True)]),
                "failed_vms": len([vm for vm in vm_reviews if not vm.get("canCreate", True)]),
                "providers_used": list(set([vm.get("providerName", "") for vm in vm_reviews if vm.get("providerName")])),
                "review_details": "Check vmReviews field for detailed per-VM validation results"
            }
        
        # Add enhanced summary with corrected VM count and cost information
        if "totalVmCount" in result and "estimatedCost" in result:
            vm_count = result["totalVmCount"]
            estimated_cost = result.get("estimatedCost", "Cost estimation unavailable")
            result["_deployment_summary"] = {
                "total_vms_to_deploy": vm_count,
                "estimated_hourly_cost": estimated_cost,
                "estimated_monthly_cost": f"~${float(estimated_cost.replace('$', '').replace('/hour', '')) * 24 * 30:.2f}/month" if "$" in estimated_cost and "/hour" in estimated_cost else "Estimate unavailable",
                "note": "VM count includes all VMs in SubGroups (subGroupSize considered)"
            }
        
        # 🎯 ENHANCED: Add LLM-friendly user interaction guidance
        creation_viable = result.get("creationViable", False)
        overall_status = result.get("overallStatus", "Unknown")
        
        if creation_viable and overall_status == "Ready":
            result["_llm_guidance"] = {
                "status": "READY_TO_CREATE",
                "message": "✅ Configuration validated successfully! All VMs can be created.",
                "next_action": "ASK_USER_CONFIRMATION",
                "user_prompt": f"""
🎯 **MCI Creation Plan Validated Successfully!**

📊 **Deployment Summary:**
• Total VMs: {result.get('totalVmCount', 'N/A')}
• Estimated Cost: {result.get('estimatedCost', 'N/A')}
• Status: All configurations validated ✅

💰 **Cost Information:**
• Hourly: {result.get('estimatedCost', 'N/A')}
• Monthly (estimated): {result.get('_deployment_summary', {}).get('estimated_monthly_cost', 'N/A')}

🔧 **Specifications:**
{_format_vm_summary_for_user(vm_reviews)}

**Would you like to proceed with creating this MCI infrastructure?**
- ✅ **Yes** - I'll create the MCI with these specifications
- ❌ **No** - I want to modify the configuration first
- 📋 **Details** - Show me more detailed validation results

*Reply with your choice to continue.*
""",
                "confirmation_required": True,
                "proceed_command": "create_mci_dynamic(..., force_create=True)"
            }
        elif creation_viable and overall_status == "Warning":
            result["_llm_guidance"] = {
                "status": "READY_WITH_WARNINGS",
                "message": "⚠️ Configuration has warnings but can proceed.",
                "next_action": "SHOW_WARNINGS_AND_ASK_CONFIRMATION",
                "user_prompt": f"""
⚠️ **MCI Creation Plan - Warnings Detected**

📊 **Deployment Summary:**
• Total VMs: {result.get('totalVmCount', 'N/A')}
• Estimated Cost: {result.get('estimatedCost', 'N/A')}
• Status: Can create with warnings ⚠️

⚠️ **Warnings Found:**
{_format_warnings_for_user(vm_reviews)}

💰 **Cost Information:**
• Hourly: {result.get('estimatedCost', 'N/A')}
• Monthly (estimated): {result.get('_deployment_summary', {}).get('estimated_monthly_cost', 'N/A')}

**Despite the warnings, would you like to proceed with creating this MCI?**
- ✅ **Yes** - Proceed anyway (warnings are acceptable)
- ❌ **No** - I want to fix the warnings first
- 📋 **Details** - Show me detailed warning information

*Please review the warnings and let me know your decision.*
""",
                "confirmation_required": True,
                "proceed_command": "create_mci_dynamic(..., force_create=True)"
            }
        else:
            result["_llm_guidance"] = {
                "status": "CANNOT_CREATE",
                "message": "❌ Configuration has errors that must be fixed before creation.",
                "next_action": "SHOW_ERRORS_AND_GUIDE_FIXES",
                "user_prompt": f"""
❌ **MCI Creation Plan - Errors Must Be Fixed**

📊 **Review Results:**
• Total VMs: {result.get('totalVmCount', 'N/A')}
• Status: Cannot create due to errors ❌

🚫 **Errors Found:**
{_format_errors_for_user(vm_reviews)}

🔧 **Recommended Actions:**
1. Use `recommend_vm_spec()` to get alternative VM specifications
2. Check `search_images()` for compatible images in different regions
3. Verify CSP provider availability and quotas
4. Update vm_configurations and run review again

**I'll help you fix these issues. Would you like me to:**
- 🔄 **Find alternatives** - Search for different VM specs/images
- 📋 **Show details** - Display detailed error information
- 🛠️ **Guide fixes** - Step-by-step troubleshooting

*Let me know how you'd like to proceed with fixing these issues.*
""",
                "confirmation_required": False,
                "proceed_command": "Fix errors first, then re-run review"
            }
        
        # Add workflow recommendations
        result["_workflow_tips"] = [
            "Always run review_mci_dynamic_request() before create_mci_dynamic()",
            "Address all critical issues (errors) before deployment",
            "Consider optimization suggestions for better performance",
            "Use hold=True in create_mci_dynamic() for manual review if needed",
            "SubGroup sizes are automatically calculated for accurate VM counts and costs"
        ]
    
    return result

# Tool: Review MCI Dynamic Request (Pre-validation)
@mcp.tool()
def review_mci_dynamic_request(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI created dynamically via MCP",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_command: Optional[Dict] = None,
    hold: bool = False
) -> Dict:
    """
    🔍 MANDATORY FIRST STEP: Review and validate MCI Dynamic Request before creation.
    
    **🚨 CRITICAL: ALWAYS CALL THIS BEFORE create_mci_dynamic():**
    This tool performs comprehensive validation and MUST be used as the first step in MCI creation:
    
    **✅ REQUIRED WORKFLOW:**
    ```python
    # STEP 1: 🔍 MANDATORY - Review configuration first
    review_result = review_mci_dynamic_request(
        ns_id="my-project",
        name="web-app-cluster", 
        vm_configurations=vm_configs  # From recommend_vm_spec()
    )
    
    # STEP 2: 📋 Check validation results
    if review_result.get("overallStatus") == "Ready":
        # ✅ Safe to proceed with MCI creation
        mci = create_mci_dynamic(
            ns_id="my-project",
            name="web-app-cluster",
            vm_configurations=vm_configs,
            force_create=True  # Required after review
        )
    else:
        # ❌ Fix validation issues first
        print("Validation issues:", review_result.get("vmReviews", []))
        # Modify vm_configurations and re-run review
    ```
    
    **🔬 COMPREHENSIVE VALIDATION CHECKS:**
    - ✅ VM specification validity and availability
    - ✅ Image compatibility with specifications
    - ✅ Resource quotas and regional availability  
    - ✅ Network configuration validation
    - ✅ Security and access requirements
    - ✅ Cross-CSP compatibility analysis
    - ✅ Accurate cost estimation (SubGroup-aware)
    - ✅ Total VM count calculation (includes SubGroup multipliers)
    - ✅ Risk assessment and optimization recommendations
    
    **COST & VM COUNT CALCULATION:**
    - Total VM count considers SubGroup sizes (e.g., subGroupSize="3" = 3 VMs)
    - Cost estimation multiplied by actual VM count per SubGroup
    - Example: SubGroup with 3 VMs @ $0.10/hour = $0.30/hour total
    
    Args:
        ns_id: Namespace ID for MCI deployment
        name: MCI name (must be unique within namespace)
        vm_configurations: List of VM configuration dictionaries. Each VM config should include:
            - specId: VM specification ID from recommend_vm_spec() (REQUIRED)
            - imageId: CSP-specific image identifier (optional - auto-mapped if omitted)
            - name: VM or subGroup name (optional)
            - description: VM description (optional)
            - subGroupSize: Number of VMs in subgroup (default "1") - affects total VM count and cost
            - connectionName: Specific connection name (optional)
            - rootDiskSize: Root disk size in GB (optional)
            - rootDiskType: Root disk type (optional)
            - vmUserPassword: VM user password (optional)
            - label: Key-value pairs for VM labeling (optional)
        description: MCI description
        system_label: System label for special purposes
        label: Key-value pairs for MCI labeling
        post_command: Post-deployment command configuration with format:
            {"command": ["command1", "command2"], "userName": "username"}
        hold: Whether to hold provisioning for review
    
    Returns:
        **🎯 COMPREHENSIVE VALIDATION RESULTS:**
        
        **📊 MAIN STATUS FIELDS:**
        - overallStatus: "Ready" | "Warning" | "Error" - Overall validation status
        - overallMessage: Summary of validation results  
        - creationViable: Boolean - Whether MCI can be created
        - estimatedCost: String - Cost estimate (e.g., "$0.0837/hour")
        - totalVmCount: Number - Total VMs to be created (includes SubGroup sizes)
        - vmReviews: Array - Detailed validation for each VM configuration
        
        **🎯 LLM GUIDANCE FIELDS:**
        - _llm_guidance: Object with user interaction instructions including:
          • status: "READY_TO_CREATE" | "READY_WITH_WARNINGS" | "CANNOT_CREATE"
          • message: Human-readable status description
          • user_prompt: Ready-to-display message for user confirmation
          • confirmation_required: Boolean indicating if user input needed
          • proceed_command: Next function call to execute
        
        **📊 DETAILED VM VALIDATION:**
        Each vmReview includes:
        - status: "Ready" | "Warning" | "Error"
        - message: Validation result description
        - canCreate: Boolean - Whether this VM can be created
        - specValidation: Spec availability and details
        - imageValidation: Image compatibility and details
        - estimatedCost: Per-VM cost estimation
        
        **✅ READY TO CREATE EXAMPLE:**
        ```json
        {
            "overallStatus": "Ready",
            "overallMessage": "All VMs can be created successfully",
            "creationViable": true,
            "estimatedCost": "$0.0837/hour",
            "totalVmCount": 2,
            "_llm_guidance": {
                "status": "READY_TO_CREATE",
                "user_prompt": "Configuration validated! Would you like to proceed?",
                "confirmation_required": true
            }
        }
        ```
        
        **⚠️ WARNINGS DETECTED EXAMPLE:**
        ```json
        {
            "overallStatus": "Warning", 
            "creationViable": true,
            "_llm_guidance": {
                "status": "READY_WITH_WARNINGS",
                "user_prompt": "Warnings found but can proceed. Continue anyway?",
                "confirmation_required": true
            }
        }
        ```
        
        **❌ ERRORS FOUND EXAMPLE:**
        **❌ ERRORS FOUND EXAMPLE:**
        ```json
        {
            "overallStatus": "Error",
            "creationViable": false, 
            "_llm_guidance": {
                "status": "CANNOT_CREATE",
                "user_prompt": "Errors must be fixed before proceeding. Would you like help?",
                "confirmation_required": false
            }
        }
        ```
        
        **🔄 NEXT STEPS:**
        1. **If Ready**: Display user_prompt from _llm_guidance, get confirmation, then call create_mci_dynamic(force_create=True)
        2. **If Warnings**: Show warnings in user_prompt, get user decision, then proceed if approved
        3. **If Errors**: Display errors, offer to help fix issues, do NOT proceed to creation
    """
    return _internal_review_mci_dynamic(
        ns_id=ns_id,
        name=name,
        vm_configurations=vm_configurations,
        description=description,
        system_label=system_label,
        label=label,
        post_command=post_command,
        hold=hold
    )

# # Tool: Create MCI (Traditional method)
# @mcp.tool()
# def create_mci(
#     ns_id: str,
#     name: str,
#     description: str = "Created via MCP",
#     vm_config: List[Dict] = None,
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
#         "subGroups": vm_config
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
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_command: Optional[Dict] = None,
    hold: bool = False,
    skip_confirmation: bool = False,
    force_create: bool = False
) -> Dict:
    """
    🚨 CRITICAL: MANDATORY TWO-STEP WORKFLOW - Review THEN Create
    
    Create Multi-Cloud Infrastructure dynamically with REQUIRED pre-validation step.
    
    **🔥 ABSOLUTELY REQUIRED WORKFLOW:**
    ```python
    # STEP 1: Get VM specifications from recommend_vm_spec()
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
        priority_policy="location",
        latitude=37.4419,
        longitude=-122.1430
    )
    
    # STEP 2: Search and select images for each spec
    vm_configurations = []
    for i, spec in enumerate(specs["recommended_specs"][:2]):
        spec_id = spec["id"]  # e.g., "aws+ap-northeast-2+t2.small"
        
        # 2.1: Search for compatible images using matched_spec_id
        images = search_images(
            matched_spec_id=spec_id,     # Auto-applies provider/region/arch
            os_type="ubuntu",            # or "centos", "windows", etc.
            include_basic_image_only=True  # Prefer clean OS installations
        )
        
        # 2.2: Select best image for this specific spec
        if images.get("imageList"):
            best_image = select_best_image_for_spec(
                images["imageList"], 
                spec, 
                {"os_type": "ubuntu", "prefer_basic": True}
            )
            selected_image_id = best_image["cspImageName"]
        else:
            raise Exception(f"No compatible images found for spec {spec_id}")
        
        # 2.3: Create VM configuration with REQUIRED imageId
        vm_configurations.append({
            "specId": spec_id,                    # 🚨 REQUIRED
            "imageId": selected_image_id,         # 🚨 REQUIRED - CSP-specific image ID
            "name": f"vm-{spec['providerName']}-{i+1}",
            "subGroupSize": "1"
        })
    
    # STEP 3: 🔍 MANDATORY REVIEW STEP - Always review first!
    review_result = review_mci_dynamic_request(
        ns_id="default",
        name="my-mci",
        vm_configurations=vm_configurations
    )
    
    # STEP 4: Check review results and fix any issues
    if review_result.get("overallStatus") != "Ready":
        # Fix configuration issues based on review feedback
        # Re-run review until status is "Ready"
    
    # STEP 5: 🚀 Create MCI only after successful review
    mci = create_mci_dynamic(
        ns_id="default",
        name="my-mci",
        vm_configurations=vm_configurations,
        force_create=True  # Required to bypass review enforcement
    )
    ```
    
    **🚨 SPEC ID VALIDATION RULES:**
    ❌ FORBIDDEN: Manual spec IDs like "tencent+na-siliconvalley+bf1.large8"
    ❌ FORBIDDEN: Guessing spec formats based on CSP patterns
    ❌ FORBIDDEN: Modifying spec IDs from recommend_vm_spec() results
    ✅ REQUIRED: Use exact spec["id"] from recommend_vm_spec() response
    ✅ REQUIRED: Call recommend_vm_spec() before every MCI creation
    ✅ REQUIRED: Use location priority when user mentions geographic preferences
    
    **IMPORTANT: Each VM spec requires its own image selection because:**
    - Different CSPs use different image formats (AMI vs Image ID vs etc.)
    - Same OS in different regions may have different image IDs  
    - Cross-CSP image references will cause deployment failures
    
    **Example workflow for location-based MCI (Silicon Valley):**
    ```
    # 1. User says: "Deploy servers in Silicon Valley"
    # 2. LLM determines coordinates: 37.4419, -122.1430
    # 3. Get location-optimized specs
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
        priority_policy="location",
        latitude=37.4419,
        longitude=-122.1430,
        limit="5"
    )
    
    # 4. Use returned spec IDs exactly as provided
    vm_configs = []
    for spec in specs["recommended_specs"]:
        vm_configs.append({
            "specId": spec["id"],  # e.g., "aws+us-west-1+t3.medium"
            "name": f"vm-{spec['regionName']}-{len(vm_configs)+1}",
            "description": f"VM in {spec['regionName']} near Silicon Valley"
        })
    
    # 5. Create MCI with location-optimized specs
    result = create_mci_dynamic(
        ns_id="my-project",
        name="silicon-valley-mci",
        vm_configurations=vm_configs
    )
    ```
    
    # 2. **SIMPLIFIED IMAGE SELECTION with MatchedSpecId**
    vm_configs = []
    for i, spec in enumerate(specs[:2]):  # Take first 2 different specs
        spec_id = spec["id"]  # e.g., "aws+ap-northeast-2+t2.small"
        
        # 3. SIMPLIFIED: Search for compatible images using MatchedSpecId
        images = search_images(
            matched_spec_id=spec_id,     # 🆕 Auto-applies provider/region/arch
            os_type="ubuntu"             # Just specify OS preference
        )
        
        # 4. Select best image for THIS specific spec
        best_image = select_best_image_for_spec(
            images["imageList"], 
            spec, 
            {"os_type": "ubuntu"}
        )
        
        # 5. Add VM config with spec-matched image
        vm_configs.append({
            "imageId": best_image["cspImageName"],  # CSP-specific image
            "specId": spec_id,                      # CSP-specific spec
            "name": f"vm-{spec['providerName']}-{i+1}",
            "description": f"VM on {spec['providerName']} in {spec['regionName']}",
            "subGroupSize": "1"
        })
    
    # 6. Create MCI with properly mapped images
    mci = create_mci_dynamic(
        ns_id="default",
        name="multi-csp-mci",
        vm_configurations=vm_configs
    )
    ```
    
    Args:
        ns_id: Namespace ID
        name: MCI name (required)
        vm_configurations: List of VM configuration dictionaries (required). Each VM config should include:
            
            **CRITICAL REQUIREMENTS FOR MCI DYNAMIC:**
            - specId: EXACT spec ID from recommend_vm_spec() API response (REQUIRED)
                * Must use full spec ID like "aws+ap-northeast-2+t2.small"
                * Do NOT modify or truncate the spec ID
                * Each spec ID is tied to specific CSP provider and region
            
            - imageId: EXACT cspImageName from search_images() API response (REQUIRED)
                * Must use exact "cspImageName" field value from image search results
                * CSP-specific image identifier (e.g., "ami-0c02fb55956c7d316" for AWS)
                * MUST match the same CSP provider and region as specId
                * CANNOT be omitted - imageId is mandatory for all VM configurations
                * Example workflow:
                  1. Use recommend_vm_spec() to get spec ID "aws+ap-northeast-2+t2.small"
                  2. Extract provider "aws" and region "ap-northeast-2" from spec ID
                  3. Use search_images(matched_spec_id=spec_id, os_type="ubuntu") to get compatible images
                  4. Use exact "cspImageName" from search results (e.g., "ami-0c02fb55956c7d316")
                  5. Set specId="aws+ap-northeast-2+t2.small", imageId="ami-0c02fb55956c7d316"
            
            **Other Configuration Options:**
            - name: VM name or subGroup name (optional)
            - description: VM description (optional)
            - subGroupSize: Number of VMs in subgroup, default "1" (optional)
            - connectionName: Specific connection name to use (optional)
            - rootDiskSize: Root disk size in GB, default "default" (optional)
            - rootDiskType: Root disk type, default "default" (optional)
            - vmUserPassword: VM user password (optional)
            - label: Key-value pairs for VM labeling (optional)
            - os_requirements: Dict with os_type, use_case for auto image selection (optional)
        
        description: MCI description (optional)
        
    **MANDATORY VALIDATION WORKFLOW FOR MCI DYNAMIC:**
    1. Always use recommend_vm_spec() first to get valid spec IDs
    2. Extract CSP provider and region from each spec ID (format: "provider+region+instance_type")
    3. Use search_images() with matching provider/region to get compatible images
    4. Use EXACT spec ID in specId and EXACT cspImageName in imageId
    5. Validate that spec and image are from same CSP provider and region
        system_label: System label for special purposes (optional)
        label: Key-value pairs for MCI labeling (optional)
        post_command: Post-deployment command configuration with format:
            {"command": ["command1", "command2"], "userName": "username"} (optional)
        hold: Whether to hold provisioning for review (optional)
        skip_confirmation: Skip user confirmation step (for automated workflows, default: False)
        force_create: Bypass confirmation and create MCI immediately (default: False)
    
    Returns:
        **REVIEW ENFORCEMENT (default behavior):**
        When force_create=False (default):
        - Returns error requiring review_mci_dynamic_request() to be called first
        - Provides guidance on proper workflow steps
        - Includes next step instructions
        
        **MCI CREATION (after review):**
        When force_create=True (after successful review):
        - Created MCI information including:
          • id: MCI ID for future operations
          • vm: List of created VMs with their details  
          • status: Current MCI status
          • deployment summary
        
        **WORKFLOW ENFORCEMENT:**
        This function enforces the two-step process:
        1. review_mci_dynamic_request() - Validates configuration and estimates costs
        2. create_mci_dynamic(force_create=True) - Actually creates the infrastructure
        
    **USER CONFIRMATION WORKFLOW:**
    ```python
    # Step 1: Review configuration and cost estimates
    summary = create_mci_dynamic(
        ns_id="my-project",
        name="my-mci",
        vm_configurations=[...]
    )
    # User reviews detailed summary with cost estimates
    
    # Step 2: After confirmation, create MCI
    result = create_mci_dynamic(
        ns_id="my-project",
        name="my-mci", 
        vm_configurations=[...],
        force_create=True  # Proceed with creation
    )
    ```
        
    **Important Notes:**
    - specId is REQUIRED for all VM configurations - cannot be omitted
    - imageId is REQUIRED for all VM configurations - cannot be omitted
    - Both specId and imageId must be from compatible CSP provider and region
    - specId format: {provider}+{region}+{spec_name} (e.g., "aws+ap-northeast-2+t2.small")
    - imageId format: CSP-specific image identifier (e.g., "ami-0c02fb55956c7d316" for AWS)
    - Use matched_spec_id in search_images() for automatic compatibility filtering
    - Use subGroupSize > 1 to create multiple VMs with same configuration
    - For multi-CSP deployments, each VM needs its own provider-specific image
    """
    
    # CRITICAL: Enforce explicit review step before MCI creation
    if not force_create and not skip_confirmation:
        return {
            "error": "❌ MCI creation requires prior validation - MANDATORY WORKFLOW",
            "message": "🚨 STOP: You MUST run review_mci_dynamic_request() first before creating MCI!",
            "workflow_violation": "REVIEW_STEP_SKIPPED",
            "required_workflow": [
                "1. 🔍 MANDATORY: Call review_mci_dynamic_request() with the same parameters",
                "2. 📋 EXAMINE: Review validation results, cost estimates, and any warnings/errors", 
                "3. 🔧 FIX: Address any validation issues if needed",
                "4. ✅ CREATE: Call create_mci_dynamic() with force_create=True to proceed"
            ],
            "why_review_required": [
                "🛡️ Prevents expensive deployment failures",
                "💰 Provides accurate cost estimation before spending money",
                "🔍 Validates VM specifications and image compatibility",
                "⚠️ Identifies potential issues before infrastructure creation",
                "📊 Shows detailed deployment plan for informed decisions"
            ],
            "llm_instructions": {
                "immediate_action": "CALL_REVIEW_FUNCTION",
                "message_to_user": "I need to validate this MCI configuration first to ensure it will work and show you the cost estimates. Let me run the review step.",
                "next_function_call": f"review_mci_dynamic_request(ns_id='{ns_id}', name='{name}', vm_configurations=<same_configurations>)",
                "after_review": "After review completes, I'll show you the results and ask for confirmation before creating the infrastructure."
            },
            "example_code": f'''
# STEP 1: Review first (MANDATORY)
review_result = review_mci_dynamic_request(
    ns_id="{ns_id}",
    name="{name}",
    vm_configurations=vm_configurations
)

# STEP 2: Check results and create if valid
if review_result.get("overallStatus") == "Ready":
    mci = create_mci_dynamic(
        ns_id="{ns_id}",
        name="{name}",
        vm_configurations=vm_configurations,
        force_create=True
    )
''',
            "critical_note": "🚨 This error is intentional to enforce proper workflow. Do NOT bypass this step."
        }
    
    # STEP 1: User confirmation workflow (unless explicitly skipped or forced)
    if not skip_confirmation and not force_create:
        # Generate comprehensive creation summary with cost analysis
        creation_summary = generate_mci_creation_summary(
            ns_id=ns_id,
            name=name,
            vm_configurations=vm_configurations,
            description=description,
            hold=hold
        )
        
        # Add creation parameters for easy re-execution
        creation_summary["_CREATION_PARAMETERS"] = {
            "ns_id": ns_id,
            "name": name,
            "vm_configurations": vm_configurations,
            "description": description,
            "system_label": system_label,
            "label": label,
            "post_command": post_command,
            "hold": hold
        }
        
        # Add clear next action instructions
        creation_summary["_NEXT_ACTION"] = {
            "action_required": "USER_CONFIRMATION",
            "message": "📋 Please review the MCI creation plan above, including cost estimates and deployment strategy.",
            "to_proceed": {
                "description": "After reviewing, call this function again with force_create=True to proceed with deployment",
                "function_call": f"create_mci_dynamic(ns_id='{ns_id}', name='{name}', vm_configurations=<same_configurations>, force_create=True)",
                "alternative": "Or use skip_confirmation=True if you want to skip future confirmations"
            },
            "to_modify": {
                "description": "To modify the configuration, adjust vm_configurations and run this function again",
                "options": [
                    "Modify vm specs, images, or counts in vm_configurations",
                    "Change namespace, description, or other parameters",
                    "Add or remove VMs from the configuration"
                ]
            }
        }
        
        # Return summary without creating MCI
        return creation_summary
    
    # STEP 1: Validate namespace first
    ns_validation = _internal_validate_namespace(ns_id)
    if not ns_validation["valid"]:
        return {
            "error": f"Namespace '{ns_id}' validation failed",
            "details": ns_validation.get("error", "Unknown error"),
            "suggestion": ns_validation.get("suggestion", ""),
            "namespace_guidance": "Use check_and_prepare_namespace() to see available namespaces or create_namespace_with_validation() to create a new one"
        }
    
    # Validate required VM configuration fields and auto-map images if needed
    processed_vm_configs = []
    for i, vm_config in enumerate(vm_configurations):
        # Check if specId is provided
        if "specId" not in vm_config:
            return {
                "error": f"VM configuration {i} validation failed",
                "details": "VM configuration must include 'specId'",
                "suggestion": "Use recommend_vm_spec() to get 'specId'"
            }
        
        common_spec = vm_config["specId"]
        common_image = vm_config.get("imageId")
        
        # Auto-map image if not provided or validate existing mapping
        if not common_image:
            # Auto-map: Find appropriate image for this spec
            try:
                # Extract CSP and region from spec
                spec_parts = common_spec.split("+")
                if len(spec_parts) < 3:
                    return {
                        "error": f"Invalid spec format in VM config {i}: {common_spec}",
                        "details": "Expected format: provider+region+spec_name"
                    }
                
                provider = spec_parts[0]
                region = spec_parts[1]
                
                # Search for images in this specific CSP/region with default x86_64 architecture
                os_requirements = vm_config.get("os_requirements", {})
                os_type = os_requirements.get("os_type", "ubuntu")
                
                images_result = search_images(
                    provider_name=provider,
                    region_name=region,
                    os_type=os_type,
                    os_architecture="x86_64"  # Default architecture
                )
                
                if not images_result or "error" in images_result:
                    return {
                        "error": f"Failed to find images for VM {i} in {provider}/{region}",
                        "details": images_result.get("error", "Unknown error")
                    }
                
                image_list = images_result.get("imageList", [])
                if not image_list:
                    return {
                        "error": f"No images available for VM {i} in {provider}/{region}",
                        "suggestion": f"Check image availability in {provider} {region} or try different os_type"
                    }
                
                # Select best image for this specific spec
                mock_spec = {
                    "id": common_spec,
                    "providerName": provider,
                    "regionName": region,
                    "architecture": "x86_64"  # Default to x86_64 architecture
                }
                
                chosen_image = select_best_image_for_spec(image_list, mock_spec, os_requirements)
                if not chosen_image or "error" in chosen_image:
                    # Fallback to basic selection
                    chosen_image = select_best_image(image_list)
                    if not chosen_image:
                        return {
                            "error": f"Failed to select appropriate image for VM {i}",
                            "suggestion": "Check image search parameters or try manual image selection"
                        }
                
                # Use the auto-selected image
                vm_config["imageId"] = chosen_image["cspImageName"]
                vm_config["_auto_mapped_image"] = True
                vm_config["_image_selection_info"] = {
                    "provider": provider,
                    "region": region,
                    "selection_reason": chosen_image.get("_selection_reason", "Auto-selected"),
                    "compatibility_score": chosen_image.get("_compatibility_score", "N/A")
                }
                
            except Exception as e:
                return {
                    "error": f"Auto image mapping failed for VM {i}",
                    "details": str(e),
                    "suggestion": "Manually specify 'imageId' or check spec format"
                }
                
        else:
            # Validate existing image mapping
            try:
                spec_parts = common_spec.split("+")
                if len(spec_parts) >= 2:
                    spec_provider = spec_parts[0].lower()
                    
                    # Basic CSP-image format validation
                    image_lower = common_image.lower()
                    is_valid_mapping = True
                    validation_warning = None
                    
                    if spec_provider == "aws" and not image_lower.startswith("ami-"):
                        validation_warning = f"AWS spec with non-AMI image: {common_image}"
                        is_valid_mapping = False
                    elif spec_provider == "azure" and "microsoft" not in image_lower and "/subscriptions/" not in image_lower:
                        validation_warning = f"Azure spec with potentially incompatible image: {common_image}"
                    elif spec_provider == "gcp" and "google" not in image_lower and "projects/" not in image_lower:
                        validation_warning = f"GCP spec with potentially incompatible image: {common_image}"
                    
                    if validation_warning:
                        vm_config["_image_validation_warning"] = validation_warning
                        if not is_valid_mapping:
                            return {
                                "error": f"Invalid image-spec mapping for VM {i}",
                                "details": validation_warning,
                                "suggestion": "Use auto-mapping by omitting 'imageId' or provide correct CSP-specific image"
                            }
                
            except Exception as e:
                # Continue with provided image if validation fails
                pass
        
        processed_vm_configs.append(vm_config)
    
    # Build request data according to model.TbMciDynamicReq spec
    data = {
        "name": name,
        "subGroups": processed_vm_configs  # Use processed configs with auto-mapped images
    }
    
    # Add optional fields
    if description:
        data["description"] = description
    # Only include installMonAgent if explicitly set to "yes"
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
    
    # Store interaction in memory for future reference
    context_data = {
        "namespace_id": ns_id,
        "mci_name": name,
        "vm_count": len(vm_configurations),
        "hold": hold
    }
    
    if "error" not in result:
        # Add namespace info to result for reference
        result["namespace_info"] = {
            "namespace_id": ns_id,
            "validation": "passed"
        }
        
        # Store successful MCI creation with auto-mapping details
        auto_mapped_count = sum(1 for vm in processed_vm_configs if vm.get("_auto_mapped_image", False))
        context_data["mci_id"] = result.get("id", name)
        context_data["auto_mapped_images"] = auto_mapped_count
        context_data["total_vms"] = len(processed_vm_configs)
        context_data["spec_to_image_mapping"] = "applied"
        
        _store_interaction_memory(
            user_request=f"Create MCI '{name}' with {len(processed_vm_configs)} VM configurations using spec-aware image selection",
            llm_response=f"Successfully created MCI '{name}' (ID: {result.get('id', name)}) with proper spec-to-image mapping in namespace '{ns_id}'. Auto-mapped {auto_mapped_count}/{len(processed_vm_configs)} images.",
            operation_type="mci_creation",
            context_data=context_data,
            status="completed"
        )
    else:
        # Store failed MCI creation
        _store_interaction_memory(
            user_request=f"Create MCI '{name}' with {len(processed_vm_configs)} VM configurations",
            llm_response=f"Failed to create MCI '{name}': {result.get('error', 'Unknown error')}",
            operation_type="mci_creation",
            context_data=context_data,
            status="failed"
        )
    
    return result

# Note: create_mci_with_proper_spec_mapping has been removed.
# Use create_mci_dynamic with proper spec-to-image mapping workflow instead.

# Note: create_mci_with_spec_first has been removed.
# Use create_mci_dynamic with spec-first workflow as described in prompts.

# Note: create_mci_with_namespace_management has been removed.
# Use create_mci_dynamic with namespace management workflow as described in prompts.

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
        Deletion result with guidance about optional shared resources cleanup
    """
    # Get MCI associated resources before deletion for guidance
    try:
        associated_resources = get_mci_associated_resources(ns_id, mci_id)
    except:
        associated_resources = None
    
    # Delete the MCI
    result = api_request("DELETE", f"/ns/{ns_id}/mci/{mci_id}?option=terminate")
    
    # Add shared resources cleanup guidance to the result
    if isinstance(result, dict) and "error" not in result:
        # Store deletion in memory
        _store_interaction_memory(
            user_request=f"Delete MCI '{mci_id}' in namespace '{ns_id}'",
            llm_response=f"Successfully deleted MCI '{mci_id}'",
            operation_type="mci_management",
            context_data={"namespace_id": ns_id, "mci_id": mci_id, "operation": "delete"},
            status="completed"
        )
        
        # Add shared resources cleanup guidance
        result["next_steps_guidance"] = {
            "mci_deletion": "✅ MCI deletion completed successfully",
            "optional_cleanup": {
                "title": "🔧 Optional: Clean up shared resources",
                "description": "Your MCI used shared resources (VNet, SecurityGroup, SSHKey) that remain in the namespace. These can be reused by future MCIs or cleaned up if no longer needed.",
                "when_to_cleanup": [
                    "This was the last MCI in the namespace",
                    "You won't create new MCIs that could reuse these network configurations",
                    "You want to completely clean up the namespace"
                ],
                "when_to_keep": [
                    "Other MCIs exist in the same namespace",
                    "You plan to create new MCIs soon",
                    "You want to keep network configurations for future use"
                ],
                "cleanup_command": f"release_resources('{ns_id}', force_release=True)",
                "check_command": f"get_mci_list('{ns_id}') # Check if other MCIs exist",
                "warning": "⚠️  Shared resources cleanup is IRREVERSIBLE and affects the entire namespace"
            }
        }
        
        # Add associated resources information if available
        if associated_resources and isinstance(associated_resources, dict) and "error" not in associated_resources:
            result["next_steps_guidance"]["associated_resources"] = {
                "description": "The deleted MCI was using these shared resources:",
                "vNets": associated_resources.get("vNetIds", []),
                "securityGroups": associated_resources.get("securityGroupIds", []),
                "sshKeys": associated_resources.get("sshKeyIds", []),
                "note": "These resources remain available for future MCIs unless explicitly released"
            }
    
    return result

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
# Memory & Interaction History Management
#####################################

# Helper function: Store user interaction in memory
def _store_interaction_memory(
    user_request: str,
    llm_response: str,
    operation_type: str,
    context_data: Optional[Dict] = None,
    status: str = "completed"
) -> bool:
    """
    Store user interaction in knowledge graph memory for future LLM sessions.
    
    Args:
        user_request: The user's original request
        llm_response: The LLM's response or action taken
        operation_type: Type of operation (e.g., "mci_creation", "namespace_management", "command_execution")
        context_data: Additional context like namespace_id, mci_id, etc.
        status: Status of the operation ("completed", "failed", "in_progress")
    
    Returns:
        Boolean indicating if storage was successful
    """
    try:
        from datetime import datetime
        
        # Create timestamp
        timestamp = datetime.now().isoformat()
        
        # Create user entity if not exists
        user_entity_name = f"User_{timestamp.split('T')[0].replace('-', '_')}"
        
        # Create interaction entity
        interaction_id = f"interaction_{timestamp.replace(':', '_').replace('-', '_').replace('.', '_')}"
        
        # Prepare observations for user entity
        user_observations = [
            f"Made request on {timestamp}: {user_request[:200]}{'...' if len(user_request) > 200 else ''}",
            f"Operation type: {operation_type}",
            f"Status: {status}"
        ]
        
        # Add context data to observations
        if context_data:
            for key, value in context_data.items():
                user_observations.append(f"{key}: {value}")
        
        # Prepare observations for interaction entity
        interaction_observations = [
            f"User request: {user_request}",
            f"LLM response summary: {llm_response[:500]}{'...' if len(llm_response) > 500 else ''}",
            f"Operation type: {operation_type}",
            f"Status: {status}",
            f"Timestamp: {timestamp}"
        ]
        
        # Store entities in memory
        try:
            # Use local memory storage for now
            # In production, this would use MCP memory tools
            if not hasattr(_store_interaction_memory, '_local_memory'):
                _store_interaction_memory._local_memory = []
            
            _store_interaction_memory._local_memory.append({
                "user_entity": user_entity_name,
                "interaction_id": interaction_id,
                "user_observations": user_observations,
                "interaction_observations": interaction_observations,
                "timestamp": timestamp,
                "operation_type": operation_type,
                "status": status
            })
            
        except Exception as mem_error:
            print(f"Memory storage error: {str(mem_error)}")
            return False
        
        return True
        
    except Exception as e:
        print(f"Warning: Failed to store interaction in memory: {str(e)}")
        return False

# Helper function: Retrieve interaction history
def _get_interaction_history(
    operation_type: Optional[str] = None,
    days_back: int = 7,
    max_results: int = 10
) -> Dict:
    """
    Retrieve recent interaction history from memory.
    
    Args:
        operation_type: Filter by operation type (optional)
        days_back: How many days back to search (default: 7)
        max_results: Maximum number of results to return (default: 10)
    
    Returns:
        Dictionary with interaction history and summary
    """
    try:
        from datetime import datetime, timedelta
        
        # Search for recent interactions
        search_query = "Interaction"
        if operation_type:
            search_query += f" {operation_type}"
        
        # Use local memory storage for search
        interactions = []
        
        if hasattr(_store_interaction_memory, '_local_memory'):
            cutoff_date = datetime.now() - timedelta(days=days_back)
            
            for memory_item in _store_interaction_memory._local_memory:
                try:
                    interaction_time = datetime.fromisoformat(memory_item["timestamp"])
                    if interaction_time >= cutoff_date:
                        # Filter by operation type if specified
                        if not operation_type or memory_item.get("operation_type") == operation_type:
                            interactions.append({
                                "id": memory_item["interaction_id"],
                                "timestamp": memory_item["timestamp"],
                                "observations": memory_item["interaction_observations"],
                                "operation_type": memory_item.get("operation_type", "unknown"),
                                "status": memory_item.get("status", "unknown")
                            })
                except (ValueError, KeyError):
                    continue
        
        # Sort by timestamp (newest first)
        interactions.sort(key=lambda x: x["timestamp"], reverse=True)
        
        return {
            "interactions": interactions[:max_results],
            "total_found": len(interactions),
            "search_period_days": days_back,
            "operation_type_filter": operation_type
        }
        
    except Exception as e:
        return {
            "error": f"Failed to retrieve interaction history: {str(e)}",
            "interactions": [],
            "total_found": 0
        }

# Helper function: Create session summary
def _create_session_summary() -> Dict:
    """
    Create a comprehensive summary of current session activities.
    
    Returns:
        Dictionary with session summary and key insights
    """
    try:
        # Get recent interactions
        history = _get_interaction_history(days_back=1, max_results=20)
        
        if not history.get("interactions"):
            return {
                "summary": "No recent interactions found",
                "key_operations": [],
                "recommendations": ["Start by checking available namespaces", "Review cloud connections"]
            }
        
        # Analyze interaction patterns
        operation_types = {}
        namespaces_used = set()
        mcis_created = []
        errors_encountered = []
        
        for interaction in history["interactions"]:
            observations = interaction.get("observations", [])
            
            # Extract operation type
            op_type = next((obs.split(": ")[1] for obs in observations if obs.startswith("Operation type:")), "unknown")
            operation_types[op_type] = operation_types.get(op_type, 0) + 1
            
            # Extract namespace information
            namespace_obs = next((obs for obs in observations if "namespace" in obs.lower()), None)
            if namespace_obs:
                namespaces_used.add(namespace_obs)
            
            # Extract MCI information
            mci_obs = next((obs for obs in observations if "mci" in obs.lower()), None)
            if mci_obs:
                mcis_created.append(mci_obs)
            
            # Check for errors
            status_obs = next((obs for obs in observations if obs.startswith("Status:")), None)
            if status_obs and "failed" in status_obs.lower():
                errors_encountered.append(interaction["id"])
        
        # Generate recommendations
        recommendations = []
        if "mci_creation" in operation_types:
            recommendations.append("Monitor created MCIs for optimal resource usage")
        if errors_encountered:
            recommendations.append("Review error logs and retry failed operations")
        if len(namespaces_used) > 3:
            recommendations.append("Consider consolidating resources into fewer namespaces")
        
        return {
            "summary": f"Found {len(history['interactions'])} recent interactions",
            "operation_summary": operation_types,
            "namespaces_used": list(namespaces_used),
            "mcis_info": mcis_created,
            "errors_count": len(errors_encountered),
            "key_insights": [
                f"Most common operation: {max(operation_types.keys(), key=operation_types.get) if operation_types else 'none'}",
                f"Success rate: {((len(history['interactions']) - len(errors_encountered)) / len(history['interactions']) * 100):.1f}%" if history['interactions'] else "No data"
            ],
            "recommendations": recommendations
        }
        
    except Exception as e:
        return {
            "error": f"Failed to create session summary: {str(e)}",
            "summary": "Unable to analyze session data"
        }

#####################################
# Command & File Management
#####################################

# Helper function: Summarize VM spec response
def _summarize_vm_specs(specs_response: Any, include_details: bool = False) -> Dict:
    """
    Summarize VM spec recommendations to reduce token usage while preserving essential information.
    
    Args:
        specs_response: Raw response from recommend_vm_spec API
        include_details: Whether to include detailed technical specifications
    
    Returns:
        Dictionary with summarized specs and metadata
    """
    if not specs_response:
        return {
            "summarized_specs": [],
            "total_count": 0,
            "details_included": include_details,
            "summary_applied": True
        }
    
    # Handle different response formats
    specs_list = []
    if isinstance(specs_response, list):
        specs_list = specs_response
    elif isinstance(specs_response, dict):
        if "result" in specs_response:
            specs_list = specs_response["result"] or []
        else:
            specs_list = [specs_response]
    
    summarized_specs = []
    
    for spec in specs_list:
        if not isinstance(spec, dict):
            continue
            
        # Extract essential information
        summarized_spec = {
            "id": spec.get("id", ""),
            "name": spec.get("name", ""),
            "providerName": spec.get("providerName", ""),
            "regionName": spec.get("regionName", ""),
            "architecture": spec.get("architecture", ""),
            "vCPU": spec.get("vCPU", 0),
            "memoryGiB": spec.get("memoryGiB", 0),
            "costPerHour": spec.get("costPerHour", -1),
            "cspSpecName": spec.get("cspSpecName", ""),
            "connectionName": spec.get("connectionName", "")
        }
        
        # Add GPU information if available
        if spec.get("acceleratorModel"):
            summarized_spec["acceleratorModel"] = spec.get("acceleratorModel")
        if spec.get("acceleratorType"):
            summarized_spec["acceleratorType"] = spec.get("acceleratorType")
        if spec.get("acceleratorCount"):
            summarized_spec["acceleratorCount"] = spec.get("acceleratorCount")
        
        # Add disk information if available
        if spec.get("diskSizeGB", -1) > 0:
            summarized_spec["diskSizeGB"] = spec.get("diskSizeGB")
        if spec.get("rootDiskType"):
            summarized_spec["rootDiskType"] = spec.get("rootDiskType")
        if spec.get("rootDiskSize") and spec.get("rootDiskSize") != "-1":
            summarized_spec["rootDiskSize"] = spec.get("rootDiskSize")
        
        # Include evaluation scores if they are meaningful (not -1)
        evaluation_scores = {}
        for i in range(1, 11):
            score_key = f"evaluationScore{i:02d}"
            score_value = spec.get(score_key, -1)
            if score_value != -1:
                evaluation_scores[score_key] = score_value
        
        if evaluation_scores:
            summarized_spec["evaluationScores"] = evaluation_scores
        
        # Include detailed specs only if requested
        if include_details and "details" in spec:
            # Categorize details for better readability
            details = spec["details"]
            if isinstance(details, list):
                categorized_details = {
                    "compute": {},
                    "storage": {},
                    "network": {},
                    "general": {}
                }
                
                for detail in details:
                    key = detail.get("key", "")
                    value = detail.get("value", "")
                    
                    # Categorize based on key names
                    if any(keyword in key.lower() for keyword in ["cpu", "vcpu", "processor", "core"]):
                        categorized_details["compute"][key] = value
                    elif any(keyword in key.lower() for keyword in ["ebs", "storage", "disk", "nvme"]):
                        categorized_details["storage"][key] = value
                    elif any(keyword in key.lower() for keyword in ["network", "bandwidth", "interface"]):
                        categorized_details["network"][key] = value
                    else:
                        categorized_details["general"][key] = value
                
                # Only include non-empty categories
                detail_categories = {k: v for k, v in categorized_details.items() if v}
                if detail_categories:
                    summarized_spec["technicalDetails"] = detail_categories
        
        summarized_specs.append(summarized_spec)
    
    return {
        "summarized_specs": summarized_specs,
        "total_count": len(summarized_specs),
        "details_included": include_details,
        "summary_applied": True,
        "note": (
            "Technical details have been summarized to reduce token usage. "
            "Use include_details=True parameter to get full specifications if needed."
        )
    }

# Helper function: Summarize command output
def _summarize_command_output(output: str, max_lines: int = 5, max_chars: int = 1000) -> Dict:
    """
    Summarize command output to reduce token usage while preserving important information.
    
    Args:
        output: Raw command output
        max_lines: Maximum number of lines to show from start and end
        max_chars: Maximum character limit for the output
    
    Returns:
        Dictionary with summarized output and metadata
    """
    if not output:
        return {
            "summary": "",
            "truncated": False,
            "original_length": 0,
            "lines_count": 0
        }
    
    original_length = len(output)
    lines = output.split('\n')
    total_lines = len(lines)
    
    # If output is short enough, return as-is
    if original_length <= max_chars and total_lines <= max_lines * 2:
        return {
            "summary": output.strip(),
            "truncated": False,
            "original_length": original_length,
            "lines_count": total_lines
        }
    
    # Create summary with first and last lines
    if total_lines > max_lines * 2:
        first_lines = lines[:max_lines]
        last_lines = lines[-max_lines:]
        
        summary_parts = []
        summary_parts.extend(first_lines)
        summary_parts.append(f"... [truncated {total_lines - (max_lines * 2)} lines] ...")
        summary_parts.extend(last_lines)
        
        summary = '\n'.join(summary_parts)
    else:
        summary = output
    
    # If still too long, truncate by characters
    if len(summary) > max_chars:
        half_chars = (max_chars - 50) // 2  # Reserve space for truncation message
        summary = (
            summary[:half_chars] + 
            f"\n... [truncated {original_length - max_chars} characters] ...\n" + 
            summary[-half_chars:]
        )
    
    return {
        "summary": summary.strip(),
        "truncated": True,
        "original_length": original_length,
        "lines_count": total_lines
    }

# Tool: Execute remote command to VMs in MCI
@mcp.tool()
def execute_command_mci(
    ns_id: str, 
    mci_id: str, 
    commands: List[str], 
    subgroup_id: Optional[str] = None, 
    vm_id: Optional[str] = None,
    label_selector: Optional[str] = None,
    summarize_output: bool = True,
    max_output_lines: Union[int, str] = 5,
    max_output_chars: Union[int, str] = 1000
) -> Dict:
    """
    Execute remote commands based on SSH on VMs of an MCI.
    This allows executing commands on all VMs in the MCI or specific VMs based on subgroup or label selector.
    
    🚨 **CRITICAL PERFORMANCE WARNING:**
    Remote command execution can take significant time depending on:
    - Command complexity and execution time
    - Number of VMs in MCI (commands run on all VMs sequentially)
    - Network latency to target VMs
    - Application installation and configuration time
    
    **Expected Response Times:**
    - Simple commands (ls, ps, etc.): 10-30 seconds
    - Package installation (apt install): 1-5 minutes
    - Application deployment scripts: 5-15 minutes
    - Complex setups (databases, clusters): 10-20 minutes
    - Large software downloads: Up to 20+ minutes
    
    **LLM Usage Guidelines:**
    1. ⏰ Inform users about potential delays before execution
    2. 📋 Break complex deployments into smaller command batches
    3. 🔄 Use verification commands to check progress
    4. ⚡ Consider summarize_output=True for large outputs
    5. 🎯 Group related commands to minimize API calls
    6. 🚨 NEVER send empty commands - Always validate command content before execution
    
    **Output Summarization:**
    By default, command outputs (stdout/stderr) are summarized to reduce token usage:
    - Shows first and last N lines of output
    - Truncates long outputs with clear indicators
    - Preserves important context while reducing size
    - Full output metadata is included for reference
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        commands: List of commands to execute
        subgroup_id: Subgroup ID (optional)
        vm_id: VM ID (optional)
        label_selector: Label selector (optional)
        summarize_output: Whether to summarize long outputs to reduce token usage (default: True)
        max_output_lines: Maximum lines to show from start/end of output (default: 5)
        max_output_chars: Maximum characters per output field (default: 1000)
    
    Returns:
        Command execution result with summarized outputs (if enabled):
        - results: List of VM execution results
        - Each result includes: mciId, vmId, vmIp, command, stdout, stderr, err
        - When summarized: stdout/stderr include summary info and truncation indicators
        - output_summary: Metadata about output summarization
    """
    # Handle type conversion for numeric parameters (MCP client may send strings)
    if isinstance(max_output_lines, str):
        try:
            max_output_lines = int(max_output_lines)
        except ValueError:
            max_output_lines = 5  # Default value
    
    if isinstance(max_output_chars, str):
        try:
            max_output_chars = int(max_output_chars)
        except ValueError:
            max_output_chars = 1000  # Default value
    
    # 🚨 CRITICAL: Validate commands before execution
    if not commands or len(commands) == 0:
        return {
            "error": "Empty command list provided",
            "message": "At least one command must be specified for execution",
            "suggestion": "Provide meaningful commands like 'ls -la', 'ps aux', 'df -h', etc."
        }
    
    # Check for empty or whitespace-only commands
    valid_commands = []
    for cmd in commands:
        if not cmd or not cmd.strip():
            return {
                "error": f"Empty or whitespace-only command detected: '{cmd}'",
                "message": "All commands must contain actual executable content",
                "suggestion": "Remove empty commands and provide meaningful command strings"
            }
        valid_commands.append(cmd.strip())
    
    if len(valid_commands) == 0:
        return {
            "error": "No valid commands found after filtering",
            "message": "All provided commands were empty or contained only whitespace",
            "suggestion": "Provide meaningful commands with actual content"
        }
    
    data = {
        "command": valid_commands  # Use validated commands
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
    
    result = api_request("POST", url, json_data=data)
    
    # Apply output summarization if enabled
    if summarize_output and "results" in result:
        total_original_size = 0
        total_summarized_size = 0
        summarization_applied = False
        
        for vm_result in result["results"]:
            # Summarize stdout for each command
            if "stdout" in vm_result:
                if isinstance(vm_result["stdout"], list):
                    summarized_stdout = []
                    for i, stdout_item in enumerate(vm_result["stdout"]):
                        if isinstance(stdout_item, str):
                            summary_info = _summarize_command_output(
                                stdout_item, max_output_lines, max_output_chars
                            )
                            total_original_size += summary_info["original_length"]
                            total_summarized_size += len(summary_info["summary"])
                            if summary_info["truncated"]:
                                summarization_applied = True
                            summarized_stdout.append({
                                "command_index": i,
                                "output": summary_info["summary"],
                                "truncated": summary_info["truncated"],
                                "original_length": summary_info["original_length"],
                                "lines_count": summary_info["lines_count"]
                            })
                        else:
                            summarized_stdout.append(stdout_item)
                    vm_result["stdout"] = summarized_stdout
                elif isinstance(vm_result["stdout"], str):
                    summary_info = _summarize_command_output(
                        vm_result["stdout"], max_output_lines, max_output_chars
                    )
                    total_original_size += summary_info["original_length"]
                    total_summarized_size += len(summary_info["summary"])
                    if summary_info["truncated"]:
                        summarization_applied = True
                    vm_result["stdout"] = {
                        "output": summary_info["summary"],
                        "truncated": summary_info["truncated"],
                        "original_length": summary_info["original_length"],
                        "lines_count": summary_info["lines_count"]
                    }
            
            # Summarize stderr for each command
            if "stderr" in vm_result:
                if isinstance(vm_result["stderr"], list):
                    summarized_stderr = []
                    for i, stderr_item in enumerate(vm_result["stderr"]):
                        if isinstance(stderr_item, str):
                            summary_info = _summarize_command_output(
                                stderr_item, max_output_lines, max_output_chars
                            )
                            total_original_size += summary_info["original_length"]
                            total_summarized_size += len(summary_info["summary"])
                            if summary_info["truncated"]:
                                summarization_applied = True
                            summarized_stderr.append({
                                "command_index": i,
                                "output": summary_info["summary"],
                                "truncated": summary_info["truncated"],
                                "original_length": summary_info["original_length"],
                                "lines_count": summary_info["lines_count"]
                            })
                        else:
                            summarized_stderr.append(stderr_item)
                    vm_result["stderr"] = summarized_stderr
                elif isinstance(vm_result["stderr"], str):
                    summary_info = _summarize_command_output(
                        vm_result["stderr"], max_output_lines, max_output_chars
                    )
                    total_original_size += summary_info["original_length"]
                    total_summarized_size += len(summary_info["summary"])
                    if summary_info["truncated"]:
                        summarization_applied = True
                    vm_result["stderr"] = {
                        "output": summary_info["summary"],
                        "truncated": summary_info["truncated"],
                        "original_length": summary_info["original_length"],
                        "lines_count": summary_info["lines_count"]
                    }
        
        # Add summarization metadata
        result["output_summary"] = {
            "summarization_enabled": True,
            "summarization_applied": summarization_applied,
            "total_original_size": total_original_size,
            "total_summarized_size": total_summarized_size,
            "size_reduction_percent": round(
                ((total_original_size - total_summarized_size) / total_original_size * 100) 
                if total_original_size > 0 else 0, 2
            ),
            "max_lines_per_output": max_output_lines,
            "max_chars_per_output": max_output_chars,
            "note": "Output has been summarized to reduce token usage. Use summarize_output=False to get full output."
        }
    else:
        result["output_summary"] = {
            "summarization_enabled": False,
            "note": "Full output returned without summarization."
        }
    
    # Store command execution in memory
    context_data = {
        "namespace_id": ns_id,
        "mci_id": mci_id,
        "commands": commands,
        "vm_count": len(result.get("results", [])),
        "summarize_output": summarize_output
    }
    
    if subgroup_id:
        context_data["subgroup_id"] = subgroup_id
    if vm_id:
        context_data["vm_id"] = vm_id
    if label_selector:
        context_data["label_selector"] = label_selector
    
    # Determine success status
    success_count = 0
    total_count = len(result.get("results", []))
    
    for vm_result in result.get("results", []):
        if not vm_result.get("err"):
            success_count += 1
    
    status = "completed" if success_count == total_count else "partial_failure" if success_count > 0 else "failed"
    
    _store_interaction_memory(
        user_request=f"Execute commands {commands} on MCI '{mci_id}' in namespace '{ns_id}'",
        llm_response=f"Command execution {status}: {success_count}/{total_count} VMs successful",
        operation_type="command_execution",
        context_data=context_data,
        status=status
    )
    
    return result

# PREDEFINED_SCRIPTS: Enhanced remote command scripts based on MapUI patterns
PREDEFINED_SCRIPTS = {
    "system_info": {
        "commands": [
            "echo '=== System Information ==='",
            "uname -a",
            "cat /etc/os-release",
            "echo '=== Memory Info ==='",
            "free -h",
            "echo '=== Disk Info ==='", 
            "df -h",
            "echo '=== Network Info ==='",
            "ip addr show",
            "echo '=== Process Info ==='",
            "ps aux | head -20"
        ],
        "description": "Comprehensive system information collection"
    },
    "docker_install": {
        "commands": [
            "echo 'Installing Docker...'",
            "sudo apt-get update",
            "sudo apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release",
            "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg",
            "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
            "sudo apt-get update",
            "sudo apt-get install -y docker-ce docker-ce-cli containerd.io",
            "sudo systemctl start docker",
            "sudo systemctl enable docker",
            "sudo usermod -aG docker $USER",
            "docker --version"
        ],
        "description": "Install Docker on Ubuntu/Debian systems"
    },
    "nginx_install": {
        "commands": [
            "echo 'Installing Nginx...'",
            "sudo apt-get update",
            "sudo apt-get install -y nginx",
            "sudo systemctl start nginx",
            "sudo systemctl enable nginx",
            "sudo ufw allow 'Nginx Full'",
            "echo 'Nginx Status:'",
            "sudo systemctl status nginx",
            "echo 'Access URL: http://{{public_ip}}'"
        ],
        "description": "Install and configure Nginx web server"
    },
    "node_install": {
        "commands": [
            "echo 'Installing Node.js...'",
            "curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -",
            "sudo apt-get install -y nodejs",
            "echo 'Node.js version:'",
            "node --version",
            "echo 'NPM version:'",
            "npm --version"
        ],
        "description": "Install Node.js 18.x LTS"
    },
    "python_install": {
        "commands": [
            "echo 'Installing Python development environment...'",
            "sudo apt-get update",
            "sudo apt-get install -y python3 python3-pip python3-venv python3-dev",
            "echo 'Python version:'",
            "python3 --version",
            "echo 'Pip version:'",
            "pip3 --version"
        ],
        "description": "Install Python 3 development environment"
    },
    "firewall_setup": {
        "commands": [
            "echo 'Setting up UFW firewall...'",
            "sudo ufw --force reset",
            "sudo ufw default deny incoming",
            "sudo ufw default allow outgoing",
            "sudo ufw allow ssh",
            "sudo ufw allow 80/tcp",
            "sudo ufw allow 443/tcp",
            "sudo ufw --force enable",
            "sudo ufw status"
        ],
        "description": "Configure basic UFW firewall rules"
    },
    "security_hardening": {
        "commands": [
            "echo 'Applying basic security hardening...'",
            "sudo apt-get update && sudo apt-get upgrade -y",
            "sudo apt-get install -y fail2ban",
            "sudo systemctl start fail2ban",
            "sudo systemctl enable fail2ban",
            "echo 'Setting up automatic security updates...'",
            "sudo apt-get install -y unattended-upgrades",
            "echo 'Disabling root SSH login...'",
            "sudo sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config",
            "sudo systemctl reload sshd"
        ],
        "description": "Apply basic security hardening measures"
    },
    "monitoring_setup": {
        "commands": [
            "echo 'Installing monitoring tools...'",
            "sudo apt-get update",
            "sudo apt-get install -y htop iotop nethogs ncdu",
            "echo 'Installing netdata...'",
            "bash <(curl -Ss https://my-netdata.io/kickstart.sh) --dont-wait",
            "echo 'Monitoring dashboard: http://{{public_ip}}:19999'"
        ],
        "description": "Install system monitoring tools and Netdata dashboard"
    }
}

# Tool: Enhanced remote command execution with predefined scripts
@mcp.tool()
def execute_remote_commands_enhanced(
    ns_id: str,
    mci_id: str,
    script_name: Optional[str] = None,
    custom_commands: Optional[List[str]] = None,
    template_variables: Optional[Dict[str, str]] = None,
    subgroup_id: Optional[str] = None,
    vm_id: Optional[str] = None,
    label_selector: Optional[str] = None,
    summarize_output: bool = True
) -> Dict:
    """
    Execute enhanced remote commands on MCI VMs with predefined scripts and template variable support.
    Based on MapUI patterns for comprehensive application deployment and management.
    
    🚨 **CRITICAL PERFORMANCE WARNING:**
    Enhanced remote command execution can take significant time:
    - Predefined scripts may include complex installations
    - Security hardening can take 5-10 minutes
    - Monitoring setup with multiple tools: 10-15 minutes
    - System updates and package installations: 5-20 minutes
    
    **LLM MUST inform users about expected delays before execution.**
    
    **LLM Usage Guidelines:**
    1. 🚨 NEVER send empty commands - Always validate command content before execution
    2. ⏰ Inform users about potential delays before execution
    3. 📋 Break complex deployments into smaller command batches
    4. 🎯 Group related commands to minimize API calls
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID
        script_name: Name of predefined script to execute (optional)
        custom_commands: List of custom commands to execute (optional)
        template_variables: Variables to substitute in commands (e.g., {"public_ip": "1.2.3.4"})
        subgroup_id: Target specific subgroup (optional)
        vm_id: Target specific VM (optional)
        label_selector: Target VMs by label selector (optional)
        summarize_output: Whether to summarize long output (default: True)
    
    Returns:
        Enhanced command execution results with template variable substitution
    """
    try:
        # Validate input
        if not script_name and not custom_commands:
            return {
                "error": "Either script_name or custom_commands must be provided",
                "available_scripts": list(PREDEFINED_SCRIPTS.keys())
            }
        
        # Prepare commands
        commands = []
        script_description = ""
        
        if script_name:
            if script_name not in PREDEFINED_SCRIPTS:
                return {
                    "error": f"Script '{script_name}' not found",
                    "available_scripts": list(PREDEFINED_SCRIPTS.keys())
                }
            
            script_config = PREDEFINED_SCRIPTS[script_name]
            commands = script_config["commands"].copy()
            script_description = script_config["description"]
        
        if custom_commands:
            commands.extend(custom_commands)
        
        # 🚨 CRITICAL: Validate commands before execution
        if not commands or len(commands) == 0:
            return {
                "error": "No commands to execute",
                "message": "Either provide a valid script_name or non-empty custom_commands",
                "available_scripts": list(PREDEFINED_SCRIPTS.keys()),
                "suggestion": "Use predefined scripts or provide meaningful custom commands"
            }
        
        # Check for empty or whitespace-only commands
        valid_commands = []
        for cmd in commands:
            if not cmd or not cmd.strip():
                return {
                    "error": f"Empty or whitespace-only command detected: '{cmd}'",
                    "message": "All commands must contain actual executable content",
                    "suggestion": "Remove empty commands and provide meaningful command strings"
                }
            valid_commands.append(cmd.strip())
        
        if len(valid_commands) == 0:
            return {
                "error": "No valid commands found after filtering",
                "message": "All provided commands were empty or contained only whitespace",
                "suggestion": "Provide meaningful commands with actual content"
            }
        
        # Get MCI access info for template variables
        if template_variables is None:
            template_variables = {}
        
        # Auto-populate common template variables
        try:
            access_info = get_mci_access_info(ns_id, mci_id, show_ssh_key=False)
            if "accessInfo" in access_info:
                public_ips = []
                private_ips = []
                
                for access in access_info["accessInfo"]:
                    if "publicIP" in access:
                        public_ips.append(access["publicIP"])
                    if "privateIP" in access:
                        private_ips.append(access["privateIP"])
                
                # Set default template variables
                if public_ips and "public_ip" not in template_variables:
                    template_variables["public_ip"] = public_ips[0]
                if "public_ips_space" not in template_variables:
                    template_variables["public_ips_space"] = " ".join(public_ips)
                if "public_ips_comma" not in template_variables:
                    template_variables["public_ips_comma"] = ",".join(public_ips)
                if "private_ips_space" not in template_variables:
                    template_variables["private_ips_space"] = " ".join(private_ips)
                if "mci_id" not in template_variables:
                    template_variables["mci_id"] = mci_id
                if "ns_id" not in template_variables:
                    template_variables["ns_id"] = ns_id
        except Exception as e:
            # Continue without auto-populated variables
            pass
        
        # Apply template variable substitution
        if template_variables:
            processed_commands = []
            for command in valid_commands:  # Use valid_commands instead of commands
                processed_command = command
                for var_name, var_value in template_variables.items():
                    processed_command = processed_command.replace(f"{{{{{var_name}}}}}", str(var_value))
                processed_commands.append(processed_command)
            final_commands = processed_commands
        else:
            final_commands = valid_commands  # Use valid_commands instead of commands
        
        # Execute commands using existing function
        result = execute_command_mci(
            ns_id=ns_id,
            mci_id=mci_id,
            commands=final_commands,  # Use final_commands
            subgroup_id=subgroup_id,
            vm_id=vm_id,
            label_selector=label_selector,
            summarize_output=summarize_output
        )
        
        # Add enhanced metadata
        result["enhanced_execution"] = {
            "script_name": script_name,
            "script_description": script_description,
            "template_variables_applied": template_variables,
            "command_count": len(final_commands),  # Use final_commands
            "execution_type": "predefined_script" if script_name else "custom_commands"
        }
        
        return result
        
    except Exception as e:
        return {
            "error": f"Enhanced command execution failed: {str(e)}",
            "available_scripts": list(PREDEFINED_SCRIPTS.keys())
        }


# Tool: Analyze provisioning risk for spec and image combination
@mcp.tool()
def analyze_provisioning_risk(
    spec_id: str,
    image_name: Optional[str] = None
) -> Dict:
    """
    Analyze the likelihood of provisioning failure based on historical data for a specific VM specification and image combination.
    
    **CRITICAL for MCI Creation Planning:**
    This tool provides intelligent risk assessment to help prevent deployment failures by analyzing:
    - Historical failure rates for the VM specification
    - Image-specific compatibility with the spec
    - Recent failure patterns and trends
    - Cross-reference of spec+image combination success rates
    
    **Risk Levels and Recommended Actions:**
    - **High Risk**: Very likely to fail (>80% failure rate)
      - 🚨 Consider alternative specs or images
      - ✅ Verify CSP quotas and permissions
      - 💡 Review error patterns for root cause
    
    - **Medium Risk**: Moderate risk (50-80% failure rate)
      - ⚠️ Proceed with caution
      - 🛠️ Have backup plans ready
      - 📊 Monitor deployment closely
    
    - **Low Risk**: Low risk (<50% failure rate)
      - ✅ Safe to proceed with normal deployment
      - 📈 Good historical success rate
    
    - **Unknown**: Insufficient historical data
      - 📊 No previous deployment history available
      - 🎯 Proceed with standard best practices
    
    **LLM Integration Guidance:**
    - Use this tool BEFORE creating MCI with specific specs
    - If high risk detected, guide user to select alternative specs
    - Provide risk-based recommendations for deployment strategy
    - Suggest fallback options for high-risk configurations
    
    Args:
        spec_id: VM specification ID (e.g., "aws+ap-northeast-2+t2.small")
        image_name: Optional image name for combined risk analysis
    
    Returns:
        Risk analysis results including:
        - overall_risk: Risk level (high/medium/low/unknown)
        - risk_score: Numeric risk score (0-100)
        - failure_rate: Historical failure percentage
        - recommendations: Specific actions based on risk level
        - historical_context: Background on previous failures
        - alternative_suggestions: Recommended alternatives if high risk
    """
    try:
        url = f"/provisioning/risk/{spec_id}"
        params = {}
        if image_name:
            params["imageName"] = image_name
        
        result = api_request("GET", url, params=params)
        
        # Store interaction for future reference
        store_interaction_memory(
            user_request=f"Analyze provisioning risk for spec '{spec_id}'" + (f" with image '{image_name}'" if image_name else ""),
            llm_response=f"Risk analysis completed - Level: {result.get('riskLevel', 'unknown')}",
            operation_type="risk_analysis",
            context_data={"spec_id": spec_id, "image_name": image_name, "risk_level": result.get('riskLevel')},
            status="completed"
        )
        
        return result
        
    except Exception as e:
        return {
            "error": f"Failed to analyze provisioning risk: {str(e)}",
            "spec_id": spec_id,
            "suggestion": "Check if spec ID format is correct (provider+region+spec_name)"
        }


# Tool: Get detailed provisioning risk analysis
@mcp.tool()
def get_detailed_provisioning_risk(
    spec_id: str,
    image_name: Optional[str] = None
) -> Dict:
    """
    Get comprehensive provisioning risk analysis with separate spec and image risk assessment.
    
    **Advanced Risk Analysis Features:**
    This tool provides detailed breakdown of risk factors for informed decision-making:
    
    **Spec-Specific Risk Analysis:**
    - Historical performance of the VM specification
    - Provider and region-specific reliability patterns
    - Resource availability and quota considerations
    - Performance characteristics and limitations
    
    **Image-Specific Risk Analysis:**
    - Image compatibility with the specification
    - Historical success rates for image deployments
    - Known compatibility issues and workarounds
    - Image freshness and support status
    
    **Combined Risk Assessment:**
    - Interaction effects between spec and image
    - Historical data for exact spec+image combination
    - Cross-validation of compatibility factors
    - Predictive risk modeling based on similar combinations
    
    **Use Cases for LLM:**
    - **Pre-deployment Validation**: Comprehensive check before MCI creation
    - **Alternative Planning**: Detailed analysis to guide spec/image selection
    - **Troubleshooting**: Understanding why certain combinations fail
    - **Cost Optimization**: Avoiding high-risk combinations that waste resources
    
    Args:
        spec_id: VM specification ID for detailed analysis
        image_name: Optional image name for combined detailed analysis
    
    Returns:
        Detailed risk analysis including:
        - spec_risk: Detailed spec-specific risk assessment
        - image_risk: Image-specific risk analysis (if image provided)
        - combined_risk: Overall risk when using spec+image together
        - detailed_recommendations: Specific actionable recommendations
        - risk_factors: Breakdown of individual risk contributors
        - mitigation_strategies: Specific steps to reduce risk
        - alternative_options: Suggested safer alternatives
    """
    try:
        url = f"/tumblebug/provisioning/risk/detailed"
        params = {"specId": spec_id}
        if image_name:
            params["imageName"] = image_name
        
        result = api_request("GET", url, params=params)
        
        # Store detailed analysis for future reference
        store_interaction_memory(
            user_request=f"Get detailed provisioning risk analysis for spec '{spec_id}'" + (f" with image '{image_name}'" if image_name else ""),
            llm_response=f"Detailed risk analysis completed - Spec Risk: {result.get('specRisk', {}).get('riskLevel', 'unknown')}, Overall: {result.get('overallRisk', {}).get('riskLevel', 'unknown')}",
            operation_type="detailed_risk_analysis",
            context_data={"spec_id": spec_id, "image_name": image_name, "analysis_type": "detailed"},
            status="completed"
        )
        
        return result
        
    except Exception as e:
        return {
            "error": f"Failed to get detailed provisioning risk analysis: {str(e)}",
            "spec_id": spec_id,
            "suggestion": "Verify spec ID format and try again"
        }


# Tool: Get provisioning history for spec
@mcp.tool()
def get_provisioning_history(
    spec_id: str
) -> Dict:
    """
    Retrieve detailed provisioning history for a specific VM specification including success/failure patterns.
    
    **Historical Insights for Better Decision Making:**
    This tool provides comprehensive historical data to understand deployment patterns:
    
    **What You'll Learn:**
    - Success and failure counts with timestamps
    - CSP-specific error messages and failure patterns
    - Image compatibility tracking across attempts
    - Regional and provider-specific reliability metrics
    - Seasonal or temporal failure patterns
    
    **Key Metrics Provided:**
    - **Failure Rate**: Percentage of failed deployments
    - **Success Count**: Number of successful deployments (tracked after failures)
    - **Failure Images**: List of images that have failed with this spec
    - **Error Patterns**: Common error messages and their frequency
    - **Time Analysis**: When failures typically occur
    
    **LLM Decision Support:**
    Use this data to:
    - **Validate Spec Choice**: Ensure spec has acceptable success rate
    - **Avoid Problematic Images**: Skip images with known compatibility issues
    - **Plan Deployment Timing**: Avoid peak failure periods if patterns exist
    - **Set Expectations**: Inform users about likely deployment success
    - **Prepare Fallbacks**: Have alternatives ready for high-failure specs
    
    Args:
        spec_id: VM specification ID to analyze history for
    
    Returns:
        Historical data including:
        - failure_count: Total number of provisioning failures
        - success_count: Number of successes (tracked after failures occur)
        - failure_rate: Calculated failure percentage
        - failure_images: List of images that failed with this spec
        - error_messages: Common error patterns and frequencies
        - last_failure: Most recent failure timestamp and details
        - first_failure: When problems first appeared
        - reliability_trend: Whether failures are increasing or decreasing
    """
    try:
        url = f"/provisioning/log/{spec_id}"
        
        result = api_request("GET", url)
        
        # Store history query for context
        store_interaction_memory(
            user_request=f"Get provisioning history for spec '{spec_id}'",
            llm_response=f"History retrieved - Failures: {result.get('failureCount', 0)}, Successes: {result.get('successCount', 0)}",
            operation_type="provisioning_history",
            context_data={"spec_id": spec_id, "failure_count": result.get('failureCount', 0)},
            status="completed"
        )
        
        return result
        
    except Exception as e:
        return {
            "error": f"Failed to retrieve provisioning history: {str(e)}",
            "spec_id": spec_id,
            "suggestion": "Check if spec ID exists and has deployment history"
        }


# Tool: Get risk-based MCI reconfiguration guidance
@mcp.tool()
def get_mci_risk_mitigation_guidance(
    vm_configurations: List[Dict],
    risk_analysis_results: Optional[Dict] = None
) -> Dict:
    """
    Provide intelligent guidance for reconfiguring MCI based on historical risk analysis.
    
    **When to Use This Tool:**
    - After review_mci_dynamic_request() shows high-risk VMs
    - When historical analysis indicates potential deployment failures
    - To optimize MCI configuration for better reliability
    - Before finalizing MCI creation with problematic specs
    
    **Risk Mitigation Strategies:**
    This tool provides specific guidance for:
    - **High-Risk Specs**: Alternative spec recommendations
    - **Image Compatibility**: Better image selection strategies  
    - **Provider Issues**: Regional or CSP-specific alternatives
    - **Resource Optimization**: Cost vs. reliability trade-offs
    
    **LLM Decision Support:**
    Use the output to:
    1. **Automatic Reconfiguration**: Generate new VM configurations
    2. **User Consultation**: Present risks and alternatives to users
    3. **Fallback Planning**: Prepare backup deployment strategies
    4. **Progressive Deployment**: Start with low-risk VMs first
    
    Args:
        vm_configurations: Original VM configurations that may have risks
        risk_analysis_results: Optional risk analysis from review_mci_dynamic_request()
    
    Returns:
        Mitigation guidance including:
        - risk_summary: Overview of identified risks
        - vm_specific_guidance: Per-VM recommendations
        - alternative_configurations: Suggested safer alternatives
        - deployment_strategies: Risk-aware deployment approaches
        - fallback_options: Backup plans if primary configs fail
        - reconfiguration_steps: Step-by-step guidance for LLM
    """
    try:
        guidance = {
            "risk_summary": {
                "total_vms": len(vm_configurations),
                "analyzed_vms": 0,
                "high_risk_vms": [],
                "medium_risk_vms": [],
                "low_risk_vms": [],
                "unknown_risk_vms": []
            },
            "vm_specific_guidance": [],
            "alternative_configurations": [],
            "deployment_strategies": {},
            "fallback_options": {},
            "reconfiguration_steps": []
        }
        
        # Analyze each VM configuration
        for i, vm_config in enumerate(vm_configurations):
            spec_id = vm_config.get("specId")
            image_name = vm_config.get("imageId")
            
            vm_guidance = {
                "vm_index": i,
                "original_spec": spec_id,
                "original_image": image_name,
                "risk_level": "unknown",
                "issues": [],
                "recommendations": [],
                "alternatives": []
            }
            
            if spec_id:
                guidance["risk_summary"]["analyzed_vms"] += 1
                
                # Get risk analysis for this specific VM
                try:
                    risk_result = analyze_provisioning_risk(spec_id, image_name)
                    if "error" not in risk_result:
                        risk_level = risk_result.get("riskLevel", "unknown")
                        vm_guidance["risk_level"] = risk_level
                        vm_guidance["failure_rate"] = risk_result.get("failureRate", "N/A")
                        vm_guidance["risk_score"] = risk_result.get("riskScore", 0)
                        
                        # Categorize by risk level
                        guidance["risk_summary"][f"{risk_level}_risk_vms"].append(i)
                        
                        # Generate specific recommendations based on risk level
                        if risk_level == "high":
                            vm_guidance["issues"].append("High historical failure rate")
                            vm_guidance["recommendations"].extend([
                                "Consider using alternative VM specification",
                                "Try different image if image-specific",
                                "Check CSP quotas and permissions",
                                "Consider different region or provider"
                            ])
                            
                            # Try to suggest alternatives
                            spec_parts = spec_id.split("+")
                            if len(spec_parts) >= 3:
                                provider = spec_parts[0]
                                region = spec_parts[1]
                                
                                vm_guidance["alternatives"].extend([
                                    f"Try smaller instance in same region: {provider}+{region}+<smaller_instance>",
                                    f"Try same instance in different region: {provider}+<different_region>+{spec_parts[2]}",
                                    "Use recommend_vm_spec() to find reliable alternatives"
                                ])
                        
                        elif risk_level == "medium":
                            vm_guidance["recommendations"].extend([
                                "Proceed with caution - monitor deployment closely", 
                                "Have backup plans ready",
                                "Consider deployment during low-traffic periods"
                            ])
                        
                        elif risk_level == "low":
                            vm_guidance["recommendations"].append("Safe to proceed with this configuration")
                        
                        # Get historical context
                        history = get_provisioning_history(spec_id)
                        if "error" not in history:
                            failure_count = history.get("failureCount", 0)
                            if failure_count > 0:
                                vm_guidance["historical_context"] = {
                                    "total_failures": failure_count,
                                    "failure_images": history.get("failureImages", []),
                                    "common_errors": history.get("errorMessages", [])
                                }
                
                except Exception as e:
                    vm_guidance["issues"].append(f"Could not analyze risk: {str(e)}")
            
            guidance["vm_specific_guidance"].append(vm_guidance)
        
        # Generate deployment strategies based on overall risk profile
        high_risk_count = len(guidance["risk_summary"]["high_risk_vms"])
        medium_risk_count = len(guidance["risk_summary"]["medium_risk_vms"])
        
        if high_risk_count > 0:
            guidance["deployment_strategies"]["recommended"] = "staged_deployment"
            guidance["deployment_strategies"]["explanation"] = "Deploy low-risk VMs first, then address high-risk VMs"
            guidance["deployment_strategies"]["steps"] = [
                "1. Create MCI with only low and medium risk VMs first",
                "2. Test and validate the initial deployment", 
                "3. Research alternatives for high-risk VMs",
                "4. Add high-risk VMs using recommended alternatives"
            ]
            
            guidance["fallback_options"]["primary"] = "alternative_specs"
            guidance["fallback_options"]["backup"] = "manual_resource_creation"
            
        elif medium_risk_count > 0:
            guidance["deployment_strategies"]["recommended"] = "cautious_deployment" 
            guidance["deployment_strategies"]["explanation"] = "Deploy with monitoring and quick rollback capability"
            
        else:
            guidance["deployment_strategies"]["recommended"] = "standard_deployment"
            guidance["deployment_strategies"]["explanation"] = "All VMs have low or unknown risk - proceed normally"
        
        # Generate step-by-step reconfiguration guidance for LLM
        if high_risk_count > 0 or medium_risk_count > 0:
            guidance["reconfiguration_steps"] = [
                "1. Use recommend_vm_spec() to find alternative specifications for high-risk VMs",
                "2. Filter specs by same provider/region but different instance types",
                "3. Run review_mci_dynamic_request() again with new configurations",
                "4. Compare risk levels between original and alternative configurations",
                "5. Proceed with configuration that has acceptable risk levels"
            ]
        
        # Generate alternative configurations automatically
        for vm_guidance in guidance["vm_specific_guidance"]:
            if vm_guidance["risk_level"] == "high":
                original_config = vm_configurations[vm_guidance["vm_index"]]
                spec_id = original_config.get("specId", "")
                
                if spec_id:
                    spec_parts = spec_id.split("+")
                    if len(spec_parts) >= 3:
                        provider = spec_parts[0]
                        region = spec_parts[1]
                        
                        # Create alternative configuration template
                        alt_config = original_config.copy()
                        alt_config["_alternative_note"] = f"Alternative for high-risk spec {spec_id}"
                        alt_config["_suggestion"] = f"Use recommend_vm_spec() with filter: provider={provider}, region={region}"
                        
                        guidance["alternative_configurations"].append({
                            "original_vm_index": vm_guidance["vm_index"],
                            "alternative_template": alt_config,
                            "search_criteria": {
                                "provider": provider,
                                "region": region,
                                "priority": "cost"  # or "performance" based on needs
                            }
                        })
        
        return guidance
        
    except Exception as e:
        return {
            "error": f"Failed to generate risk mitigation guidance: {str(e)}",
            "suggestion": "Ensure VM configurations are properly formatted"
        }

# Tool: List available predefined scripts
@mcp.tool()
def list_predefined_scripts() -> Dict:
    """
    List all available predefined scripts for enhanced remote command execution.
    
    Returns:
        Dictionary containing all available predefined scripts with descriptions
    """
    scripts_info = {}
    
    for script_name, script_config in PREDEFINED_SCRIPTS.items():
        scripts_info[script_name] = {
            "description": script_config["description"],
            "command_count": len(script_config["commands"]),
            "commands_preview": script_config["commands"][:3] + ["..."] if len(script_config["commands"]) > 3 else script_config["commands"]
        }
    
    return {
        "predefined_scripts": scripts_info,
        "total_scripts": len(PREDEFINED_SCRIPTS),
        "usage_note": "Use execute_remote_commands_enhanced() with script_name parameter to execute these scripts",
        "template_variables_supported": [
            "{{public_ip}}", "{{public_ips_space}}", "{{public_ips_comma}}", 
            "{{private_ips_space}}", "{{mci_id}}", "{{ns_id}}"
        ]
    }

# Tool: Store interaction in memory
@mcp.tool()
def store_interaction_memory(
    user_request: str,
    llm_response: str,
    operation_type: str,
    context_data: Optional[Dict] = None,
    status: str = "completed"
) -> Dict:
    """
    Store user interaction in memory for future LLM sessions.
    This enables new LLM instances to understand previous work context.
    
    Args:
        user_request: The user's original request
        llm_response: The LLM's response or action taken
        operation_type: Type of operation (e.g., "mci_creation", "namespace_management", "command_execution")
        context_data: Additional context like namespace_id, mci_id, etc.
        status: Status of the operation ("completed", "failed", "in_progress")
    
    Returns:
        Dictionary with storage result and interaction ID
    """
    success = _store_interaction_memory(user_request, llm_response, operation_type, context_data, status)
    
    return {
        "success": success,
        "message": "Interaction stored in memory" if success else "Failed to store interaction",
        "operation_type": operation_type,
        "status": status,
        "timestamp": datetime.now().isoformat()
    }

# Tool: Get interaction history
@mcp.tool()
def get_interaction_history(
    operation_type: Optional[str] = None,
    days_back: Union[int, str] = 7,
    max_results: Union[int, str] = 10
) -> Dict:
    """
    Retrieve recent interaction history from memory.
    Useful for new LLM sessions to understand previous work context.
    
    Args:
        operation_type: Filter by operation type (optional)
        days_back: How many days back to search (default: 7)
        max_results: Maximum number of results to return (default: 10)
    
    Returns:
        Dictionary with interaction history and analysis
    """
    # Handle type conversion for numeric parameters (MCP client may send strings)
    if isinstance(days_back, str):
        try:
            days_back = int(days_back)
        except ValueError:
            days_back = 7  # Default value
    
    if isinstance(max_results, str):
        try:
            max_results = int(max_results)
        except ValueError:
            max_results = 10  # Default value
    
    return _get_interaction_history(operation_type, days_back, max_results)

# Tool: Get session summary
@mcp.tool()
def get_session_summary() -> Dict:
    """
    Get a comprehensive summary of current session activities.
    Provides context for new LLM sessions about recent work.
    
    Returns:
        Dictionary with session summary, patterns, and recommendations
    """
    return _create_session_summary()

# Tool: Search interaction memory
@mcp.tool()
def search_interaction_memory(
    search_term: str,
    max_results: Union[int, str] = 5
) -> Dict:
    """
    Search through stored interactions for specific terms or contexts.
    Helps LLMs find relevant previous work quickly.
    
    Args:
        search_term: Term to search for in interactions
        max_results: Maximum number of results (default: 5)
    
    Returns:
        Dictionary with matching interactions and relevance scores
    """
    # Handle type conversion for max_results (MCP client may send string)
    if isinstance(max_results, str):
        try:
            max_results = int(max_results)
        except ValueError:
            max_results = 5  # Default value
    
    try:
        results = []
        
        if hasattr(_store_interaction_memory, '_local_memory'):
            for memory_item in _store_interaction_memory._local_memory:
                relevance_score = 0
                search_lower = search_term.lower()
                
                # Search in user request
                if search_lower in str(memory_item.get("user_observations", [])).lower():
                    relevance_score += 2
                
                # Search in LLM response
                if search_lower in str(memory_item.get("interaction_observations", [])).lower():
                    relevance_score += 1
                
                # Search in operation type
                if search_lower in memory_item.get("operation_type", "").lower():
                    relevance_score += 3
                
                if relevance_score > 0:
                    results.append({
                        "interaction_id": memory_item["interaction_id"],
                        "timestamp": memory_item["timestamp"],
                        "operation_type": memory_item.get("operation_type", "unknown"),
                        "status": memory_item.get("status", "unknown"),
                        "relevance_score": relevance_score,
                        "preview": str(memory_item.get("user_observations", [])[:1])[:100] + "..."
                    })
        
        # Sort by relevance score
        results.sort(key=lambda x: x["relevance_score"], reverse=True)
        
        return {
            "search_term": search_term,
            "results": results[:max_results],
            "total_found": len(results),
            "search_successful": True
        }
        
    except Exception as e:
        return {
            "search_term": search_term,
            "results": [],
            "total_found": 0,
            "search_successful": False,
            "error": str(e)
        }

# Tool: Clear interaction memory
@mcp.tool()
def clear_interaction_memory(
    confirm: bool = False,
    days_older_than: Optional[int] = None
) -> Dict:
    """
    Clear stored interaction memory (use with caution).
    
    Args:
        confirm: Must be True to actually clear memory
        days_older_than: Only clear interactions older than N days (optional)
    
    Returns:
        Dictionary with clearing result
    """
    if not confirm:
        return {
            "cleared": False,
            "message": "Memory not cleared. Set confirm=True to actually clear memory.",
            "warning": "This will remove all stored interaction history."
        }
    
    try:
        if hasattr(_store_interaction_memory, '_local_memory'):
            original_count = len(_store_interaction_memory._local_memory)
            
            if days_older_than:
                from datetime import datetime, timedelta
                cutoff_date = datetime.now() - timedelta(days=days_older_than)
                
                # Keep only recent interactions
                _store_interaction_memory._local_memory = [
                    item for item in _store_interaction_memory._local_memory
                    if datetime.fromisoformat(item["timestamp"]) >= cutoff_date
                ]
                
                cleared_count = original_count - len(_store_interaction_memory._local_memory)
                
                return {
                    "cleared": True,
                    "message": f"Cleared {cleared_count} interactions older than {days_older_than} days",
                    "remaining_count": len(_store_interaction_memory._local_memory)
                }
            else:
                # Clear all memory
                _store_interaction_memory._local_memory = []
                
                return {
                    "cleared": True,
                    "message": f"Cleared all {original_count} stored interactions",
                    "remaining_count": 0
                }
        else:
            return {
                "cleared": True,
                "message": "No memory to clear",
                "remaining_count": 0
            }
            
    except Exception as e:
        return {
            "cleared": False,
            "message": f"Failed to clear memory: {str(e)}",
            "error": str(e)
        }

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
    """Prompt for MCI management with comprehensive create_mci_dynamic usage patterns"""
    return """
    You are a Multi-Cloud Infrastructure (MCI) management expert for Cloud-Barista CB-Tumblebug.
    
    🚨 **CRITICAL: ONLY USE create_mci_dynamic FOR ALL MCI CREATION**
    
    All MCI creation MUST use `create_mci_dynamic` with proper workflow patterns. 
    Other MCI creation tools have been deprecated.
    
    **🔥 MANDATORY WORKFLOW FOR create_mci_dynamic:**
    
    **PATTERN 1: SPEC-FIRST WORKFLOW (RECOMMENDED)**
    ```python
    # Step 1: Find VM specifications first (determines CSP and region)
    specs = recommend_vm_spec(
        filter_policies={
            "vCPU": {"min": 2, "max": 8},
            "memoryGiB": {"min": 4, "max": 16}
        },
        priority_policy="cost"  # or "performance", "location"
    )
    
    # Step 2: For each spec, find compatible images in same CSP/region
    vm_configs = []
    for spec in specs["recommended_specs"][:2]:  # Use multiple specs for multi-CSP
        spec_id = spec["id"]  # e.g., "aws+us-east-1+t3.medium"
        
        # Extract CSP and region from spec ID
        provider, region, instance_type = spec_id.split("+")
        
        # Step 3: Search for images in the SAME CSP/region as the spec
        images = search_images(
            provider_name=provider,  # Must match spec's provider
            region_name=region,      # Must match spec's region
            os_type="ubuntu 22.04"
        )
        
        # Step 4: Select best image for this specific spec
        # Use intelligent image selection instead of arbitrary first choice
        best_image = select_best_image_for_spec(
            images["imageList"], spec, {"os_type": "ubuntu 22.04"}
        )
        # Alternative: best_image = select_best_image(images["imageList"])
        
        # Step 5: Create VM config with spec-matched image
        vm_configs.append({
            "specId": spec_id,                          # Exact spec ID from API
            "imageId": best_image["cspImageName"],      # Intelligently selected image
            "name": f"vm-{provider}-{len(vm_configs)+1}",
            "description": f"VM on {provider} in {region}",
            "subGroupSize": "1"
        })
    
    # Step 6: Create MCI with properly mapped configurations
    mci = create_mci_dynamic(
        ns_id="my-project",
        name="multi-csp-mci",
        vm_configurations=vm_configs
    )
    ```
    
    **PATTERN 2: AUTO-MAPPING WORKFLOW (SIMPLER)**
    ```python
    # Let create_mci_dynamic auto-map images for specs
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
    )
    
    vm_configs = []
    for spec in specs["recommended_specs"][:2]:
        vm_configs.append({
            "specId": spec["id"],  # REQUIRED: Exact spec ID
            "name": f"vm-{spec['providerName']}-{len(vm_configs)+1}",
            "os_requirements": {"os_type": "ubuntu", "use_case": "web-server"}
            # imageId omitted - will be auto-mapped to compatible image
        })
    
    mci = create_mci_dynamic(
        ns_id="my-project",
        name="auto-mapped-mci",
        vm_configurations=vm_configs  # Auto-mapping ensures compatibility
    )
    ```
    
    **PATTERN 3: LOCATION-BASED WORKFLOW**
    ```python
    # User says: "Deploy in Silicon Valley"
    
    # Step 1: Convert location to coordinates
    latitude, longitude = 37.4419, -122.1430  # Silicon Valley coordinates
    
    # Step 2: Get location-optimized specs
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
        priority_policy="location",
        latitude=latitude,
        longitude=longitude
    )
    
    # Step 3: Create MCI with location-optimized specs
    mci = create_mci_dynamic(
        ns_id="production",
        name="silicon-valley-mci",
        vm_configurations=[
            {"specId": spec["id"], "name": f"vm-{spec['regionName']}-{i+1}"}
            for i, spec in enumerate(specs["recommended_specs"][:3])
        ]
    )
    ```
    
    **🔍 PATTERN 4: RISK-AWARE WORKFLOW (NEW - ENHANCED)**
    ```python
    # CRITICAL: Always analyze historical risk before MCI creation
    
    # Step 1: Prepare initial VM configurations
    vm_configs = [
        {"specId": "aws+us-east-1+t2.small", "name": "web-server"},
        {"specId": "azure+eastus+Standard_B2s", "name": "api-server"}
    ]
    
    # Step 2: Review configurations with risk analysis
    review_result = review_mci_dynamic_request(
        ns_id="production",
        name="web-application",
        vm_configurations=vm_configs
    )
    
    # Step 3: Check for high-risk VMs and get mitigation guidance
    if review_result.get("overall_risk_assessment", {}).get("high_risk_vms"):
        guidance = get_mci_risk_mitigation_guidance(vm_configs)
        
        # Step 4: Reconfigure based on risk analysis
        for alt_config in guidance["alternative_configurations"]:
            vm_index = alt_config["original_vm_index"]
            search_criteria = alt_config["search_criteria"]
            
            # Find safer alternatives
            safer_specs = recommend_vm_spec(
                filter_policies={
                    "ProviderName": search_criteria["provider"],
                    "RegionName": search_criteria["region"],
                    "vCPU": {"min": 1, "max": 4}
                },
                priority_policy="cost"
            )
            
            # Replace high-risk spec with safer alternative
            if safer_specs["recommended_specs"]:
                vm_configs[vm_index]["specId"] = safer_specs["recommended_specs"][0]["id"]
        
        # Step 5: Re-review with updated configurations
        final_review = review_mci_dynamic_request(
            ns_id="production",
            name="web-application",
            vm_configurations=vm_configs
        )
    
    # Step 6: Create MCI only after acceptable risk level
    if final_review.get("creationViable", False):
        mci = create_mci_dynamic(
            ns_id="production",
            name="web-application",
            vm_configurations=vm_configs
        )
    ```
    
    **🔧 PATTERN 5: PROGRESSIVE RISK MITIGATION**
    ```python
    # For high-risk scenarios, deploy in stages
    
    # Step 1: Separate VMs by risk level
    guidance = get_mci_risk_mitigation_guidance(original_vm_configs)
    low_risk_configs = []
    high_risk_configs = []
    
    for i, vm_guidance in enumerate(guidance["vm_specific_guidance"]):
        if vm_guidance["risk_level"] in ["low", "unknown"]:
            low_risk_configs.append(original_vm_configs[i])
        else:
            high_risk_configs.append(original_vm_configs[i])
    
    # Step 2: Deploy low-risk VMs first
    if low_risk_configs:
        stable_mci = create_mci_dynamic(
            ns_id="production",
            name="stable-infrastructure",
            vm_configurations=low_risk_configs
        )
    
    # Step 3: Research and deploy high-risk VMs with alternatives
    for high_risk_config in high_risk_configs:
        risk_analysis = analyze_provisioning_risk(
            high_risk_config["specId"]
        )
        
        if risk_analysis.get("riskLevel") == "high":
            # Get detailed analysis and alternatives
            detailed_risk = get_detailed_provisioning_risk(
                high_risk_config["specId"]
            )
            # Apply recommendations from detailed analysis
        
        # Add to MCI after risk mitigation
        enhanced_vm = create_mci_vm_dynamic(
            ns_id="production",
            mci_id="stable-infrastructure",
            req=modified_high_risk_config
        )
    ```
    
    # Step 3: Use specs with auto-mapping or manual image selection
    vm_configs = []
    for spec in specs["recommended_specs"][:2]:
        vm_configs.append({
            "specId": spec["id"],
            "name": f"vm-{spec['regionName']}-{len(vm_configs)+1}",
            "description": f"VM near Silicon Valley in {spec['regionName']}"
        })
    
    mci = create_mci_dynamic(
        ns_id="location-project",
        name="silicon-valley-mci",
        vm_configurations=vm_configs
    )
    ```
    
    **PATTERN 4: NAMESPACE MANAGEMENT WORKFLOW**
    ```python
    # Check namespace first
    ns_check = check_and_prepare_namespace("my-project")
    
    # Create namespace if needed
    if not ns_check["namespace_exists"]:
        create_namespace_with_validation("my-project", "Project namespace")
    
    # Then create MCI
    mci = create_mci_dynamic(
        ns_id="my-project",
        name="managed-mci",
        vm_configurations=vm_configs
    )
    ```
    
    **PATTERN 5: CONFIRMATION WORKFLOW**
    ```python
    # Preview configuration first
    preview = create_mci_dynamic(
        ns_id="my-project",
        name="preview-mci",
        vm_configurations=vm_configs,
        skip_confirmation=False  # Returns preview only
    )
    
    # User reviews preview, then confirms
    mci = create_mci_dynamic(
        ns_id="my-project",
        name="confirmed-mci",
        vm_configurations=vm_configs,
        force_create=True  # Actually creates after confirmation
    )
    ```
    
    **PATTERN 6: COMPREHENSIVE REVIEW WORKFLOW (MANDATORY)**
    ```python
    # Step 1: Pre-creation validation
    validation = review_mci_dynamic_request(
        ns_id="my-project",
        name="web-application",
        vm_configurations=vm_configs
    )
    
    # Step 2: CRITICAL - Comprehensive review result analysis
    # YOU MUST analyze ALL aspects of the review result before proceeding
    
    # 2A. Overall Validation Status
    creation_viable = validation.get("creationViable", False)
    validation_passed = validation.get("validation_passed", False)
    
    # 2B. VM-Level Analysis
    vm_reviews = validation.get("vmReviews", [])
    for i, vm_review in enumerate(vm_reviews):
        vm_name = vm_configs[i].get("name", f"VM-{i+1}")
        print(f"\\n🔍 VM Review: {vm_name}")
        
        # Check individual VM status
        vm_viable = vm_review.get("viable", False)
        vm_issues = vm_review.get("issues", [])
        
        if not vm_viable:
            print(f"❌ VM {vm_name} has issues:")
            for issue in vm_issues:
                print(f"   • {issue}")
        else:
            print(f"✅ VM {vm_name} configuration is valid")
        
        # Analyze spec-image compatibility
        spec_image_compat = vm_review.get("specImageCompatibility", {})
        if spec_image_compat:
            compat_score = spec_image_compat.get("compatibility_score", 0)
            compat_issues = spec_image_compat.get("issues", [])
            
            if compat_score < 80:
                print(f"⚠️  VM {vm_name} spec-image compatibility: {compat_score}%")
                for issue in compat_issues:
                    print(f"   • Compatibility issue: {issue}")
    
    # 2C. Risk Assessment Analysis  
    risk_assessment = validation.get("overall_risk_assessment", {})
    if risk_assessment:
        risk_level = risk_assessment.get("overall_risk", "unknown")
        high_risk_vms = risk_assessment.get("high_risk_vms", [])
        risk_factors = risk_assessment.get("risk_factors", [])
        
        print(f"\\n🎯 Risk Assessment: {risk_level.upper()}")
        
        if high_risk_vms:
            print("⚠️  High-risk VMs detected:")
            for high_risk_vm in high_risk_vms:
                vm_name = high_risk_vm.get("vm_name", "Unknown")
                risk_reasons = high_risk_vm.get("risk_reasons", [])
                print(f"   • {vm_name}: {', '.join(risk_reasons)}")
        
        if risk_factors:
            print("📊 Overall risk factors:")
            for factor in risk_factors:
                print(f"   • {factor}")
    
    # 2D. Cost and Resource Analysis
    estimated_cost = validation.get("estimatedCost", {})
    if estimated_cost:
        total_cost = estimated_cost.get("totalCostPerHour", 0)
        cost_breakdown = estimated_cost.get("vmCosts", [])
        
        print(f"\\n💰 Cost Analysis: ${total_cost:.3f}/hour")
        for cost_item in cost_breakdown:
            vm_name = cost_item.get("vmName", "Unknown")
            vm_cost = cost_item.get("costPerHour", 0)
            print(f"   • {vm_name}: ${vm_cost:.3f}/hour")
    
    # 2E. Network and Security Analysis
    network_analysis = validation.get("networkAnalysis", {})
    if network_analysis:
        network_complexity = network_analysis.get("complexity", "unknown")
        security_groups = network_analysis.get("securityGroups", [])
        
        print(f"\\n🔒 Network Complexity: {network_complexity}")
        if security_groups:
            print("Security groups to be created:")
            for sg in security_groups:
                print(f"   • {sg}")
    
    # Step 3: Decision making based on comprehensive analysis
    print("\\n📋 REVIEW SUMMARY:")
    print(f"Creation Viable: {'✅ YES' if creation_viable else '❌ NO'}")
    print(f"Validation Passed: {'✅ YES' if validation_passed else '❌ NO'}")
    
    # Step 4: Handle different review outcomes
    if not creation_viable:
        print("\\n🚨 CRITICAL ISSUES DETECTED - Creation not recommended")
        print("Required actions before proceeding:")
        
        issues = validation.get("issues", [])
        for issue in issues:
            print(f"   • Fix: {issue}")
        
        # Don't proceed with creation - user must fix issues first
        return validation
        
    elif not validation_passed:
        print("\\n⚠️  VALIDATION WARNINGS - Proceed with caution")
        
        # Ask user for confirmation before proceeding
        user_wants_to_proceed = input("Do you want to proceed despite warnings? (yes/no): ")
        if user_wants_to_proceed.lower() != 'yes':
            return validation
    
    else:
        print("\\n✅ ALL VALIDATIONS PASSED - Safe to proceed")
    
    # Step 5: Only proceed with creation after thorough review
    print("\\n🚀 Proceeding with MCI creation...")
    mci = create_mci_dynamic(
        ns_id="my-project",
        name="web-application",
        vm_configurations=vm_configs
    )
    
    # Step 6: Post-creation status monitoring
    print("\\n📊 Monitoring deployment progress...")
    status = check_mci_status_and_handle_failures(
        ns_id="my-project",
        mci_id=mci["id"],
        detailed_analysis=True
    )
    ```
    
    **🔍 MANDATORY REVIEW RESULT ANALYSIS CHECKLIST:**
    
    Before proceeding with any MCI creation, you MUST analyze and report:
    
    ✅ **Overall Status:**
    - [ ] creationViable (true/false)
    - [ ] validation_passed (true/false) 
    - [ ] Total estimated cost per hour
    
    ✅ **Per-VM Analysis:**
    - [ ] Each VM's viable status
    - [ ] Individual VM issues and warnings
    - [ ] Spec-image compatibility scores
    - [ ] Resource allocation validation
    
    ✅ **Risk Assessment:**
    - [ ] Overall risk level (low/medium/high)
    - [ ] Identification of high-risk VMs
    - [ ] Specific risk factors and mitigation suggestions
    - [ ] Historical failure analysis results
    
    ✅ **Resource Planning:**
    - [ ] Network complexity analysis
    - [ ] Security group requirements
    - [ ] Resource dependencies and conflicts
    - [ ] Multi-region deployment considerations
    
    ✅ **User Communication:**
    - [ ] Clear explanation of any issues found
    - [ ] Specific steps to resolve problems
    - [ ] Cost implications and resource usage
    - [ ] Risk trade-offs and recommendations
    
    **❌ NEVER proceed with create_mci_dynamic() until:**
    - All critical issues are resolved
    - User understands and accepts any risks
    - Cost implications are clearly communicated
    - Alternative configurations are considered if needed
    
    **🔑 CRITICAL VM CONFIGURATION REQUIREMENTS:**
    
    **specId (ALWAYS REQUIRED):**
    - MUST be exact spec ID from recommend_vm_spec() results
    - Format: "{provider}+{region}+{instance_type}" (e.g., "aws+us-east-1+t3.medium")
    - ❌ NEVER manually create spec IDs
    - ✅ ALWAYS get from recommend_vm_spec() API
    
    **imageId (RECOMMENDED):**
    - Should be exact cspImageName from search_images() results
    - Must be compatible with specId's CSP/region
    - If omitted: Auto-mapped by create_mci_dynamic
    - Provider-specific formats:
      * AWS: "ami-0123456789abcdef0"
      * Azure: "/subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/images/ubuntu-20.04"
      * GCP: "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20240307a"
    
    **🔄 COMPLETE CREATE_MCI_DYNAMIC WORKFLOW:**
    
    **1. PREPARATION PHASE:**
    ```python
    # A. Check/create namespace
    ns_result = check_and_prepare_namespace(preferred_ns_id)
    
    # B. Get VM specifications (determines CSP and region)
    specs = recommend_vm_spec(
        filter_policies=user_requirements,
        priority_policy="cost|performance|location",
        latitude=lat,  # if location-based
        longitude=lon  # if location-based
    )
    ```
    
    **2. CONFIGURATION PHASE:**
    ```python
    # A. For each spec, build VM configuration
    vm_configs = []
    for spec in specs["recommended_specs"]:
        spec_id = spec["id"]
        
        # B. Extract CSP info from spec
        provider, region, instance = spec_id.split("+")
        
        # C. Find compatible images (optional but recommended)
        images = search_images(
            provider_name=provider,
            region_name=region,
            os_type=desired_os
        )
        
        # D. Create VM config
        vm_config = {
            "specId": spec_id,  # Required: exact spec ID
            "imageId": images["imageList"][0]["cspImageName"],  # Optional
            "name": f"vm-{provider}-{vm_index}",
            "description": f"VM on {provider} in {region}",
            "subGroupSize": "1"
        }
        vm_configs.append(vm_config)
    ```
    
    **3. CREATION PHASE:**
    ```python
    # A. Validate configuration (optional but recommended)
    validation = review_mci_dynamic_request(ns_id, name, vm_configs)
    
    # B. Create MCI
    mci = create_mci_dynamic(
        ns_id=target_namespace,
        name=mci_name,
        vm_configurations=vm_configs,
        description="Multi-CSP infrastructure",
        hold=False,  # Set True to hold for review
        skip_confirmation=False,  # Set True for automated workflows
        force_create=False  # Set True after user confirmation
    )
    ```
    
    **4. POST-CREATION PHASE (MANDATORY):**
    ```python
    # A. Check deployment status
    status = check_mci_status_and_handle_failures(
        ns_id=target_namespace,
        mci_id=mci["id"],
        auto_cleanup_failed=False
    )
    
    # B. Handle different outcomes
    if status["deployment_health"] == "healthy":
        print("✅ All VMs deployed successfully!")
    elif status["deployment_health"] == "partial-failed":
        # Offer cleanup of failed VMs
        recovery = interactive_mci_recovery(
            ns_id, mci["id"], 
            recovery_action="refine"
        )
    elif status["deployment_health"] == "critical":
        print("❌ All VMs failed - investigate and retry")
    ```
    
    **🚨 CRITICAL SPEC-TO-IMAGE MAPPING RULES:**
    
    **Why Proper Mapping Matters:**
    - AWS uses AMI IDs, Azure uses Image IDs, GCP uses Image URIs
    - Same OS in different regions has different image identifiers
    - Cross-CSP image references cause deployment failures
    
    **Correct Mapping Pattern:**
    ```python
    # ✅ CORRECT: Each VM gets spec-matched image
    vm_configs = [
        {
            "specId": "aws+us-east-1+t3.medium",
            "imageId": "ami-0123456789abcdef0"  # AWS AMI in us-east-1
        },
        {
            "specId": "azure+eastus+Standard_B2s", 
            "imageId": "/subscriptions/.../images/ubuntu-20.04"  # Azure Image in eastus
        }
    ]
    ```
    
    **Wrong Mapping Pattern:**
    ```python
    # ❌ WRONG: Using same image for different CSPs
    vm_configs = [
        {
            "specId": "aws+us-east-1+t3.medium",
            "imageId": "ami-0123456789abcdef0"
        },
        {
            "specId": "azure+eastus+Standard_B2s",
            "imageId": "ami-0123456789abcdef0"  # ERROR: AWS AMI for Azure spec
        }
    ]
    ```
    
    **📍 LOCATION-TO-COORDINATES MAPPING:**
    When users mention locations, use these coordinates for location-based priority:
    - **Silicon Valley**: 37.4419° N, 122.1430° W
    - **Seoul**: 37.5665° N, 126.9780° E
    - **Tokyo**: 35.6762° N, 139.6503° E
    - **London**: 51.5074° N, 0.1278° W
    - **Sydney**: 33.8688° S, 151.2093° E
    - **Frankfurt**: 50.1109° N, 8.6821° E
    - **Singapore**: 1.3521° N, 103.8198° E
    - **Mumbai**: 19.0760° N, 72.8777° E
    - **Virginia**: 38.7223° N, 78.1692° W
    
    **� FAILURE RECOVERY STRATEGIES:**
    
    **Partial-Failed State:**
    - Some VMs succeeded, some failed
    - Use 'refine' action to cleanup failed VMs
    - Preserve successful infrastructure
    - Continue with working VMs
    
    **Complete Failure State:**
    - All VMs failed
    - Investigate error messages
    - Try different specs/regions
    - Consider terminate and recreate
    
    **🚨 ABSOLUTE REQUIREMENTS:**
    1. ✅ ALWAYS use recommend_vm_spec() for spec IDs
    2. ✅ ALWAYS check MCI status after creation
    3. ✅ ALWAYS handle failures with user confirmation
    4. ✅ ALWAYS use location priority for geographic requests
    5. ❌ NEVER create spec IDs manually
    6. ❌ NEVER skip failure recovery workflows
    7. ❌ NEVER use cross-CSP image references
    
    Current namespace list: {{namespace://list}}
    
    What MCI would you like to create? I'll guide you through the proper create_mci_dynamic workflow with spec validation and failure handling.
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

# Prompt: Context awareness for new LLM sessions
@mcp.prompt()
def context_awareness_prompt() -> str:
    """Prompt for new LLM sessions to understand previous work context"""
    return """
    You are a Cloud-Barista CB-Tumblebug expert who has access to interaction memory from previous sessions.
    
    🚨 **CRITICAL: HARDWARE RESEARCH REQUIRED FOR ALL APPLICATION DEPLOYMENTS**
    
    **⚡ MANDATORY BEHAVIOR: When using context from previous sessions for application deployments:**
    1. **ALWAYS research hardware requirements** for any new applications (even if similar deployed before)
    2. **Apply current hardware research standards** (minimum 50GB disk, optimized specs)
    3. **Use deployment_type** inferred from user context and historical patterns
    4. **Enable research_requirements=True** for all deployments unless explicitly told not to
    5. **Explain hardware research benefits** to users based on improved deployment context
    
    **🔥 Enhanced Context Workflow with Hardware Research:**
    ```python
    # 1. Check previous work context
    history = get_interaction_history(operation_type="mci_creation")
    
    # 2. For NEW application deployments, always research (regardless of history)
    research_result = research_application_requirements(
        application_name="user_requested_app",
        deployment_type="production"  # infer from context + history
    )
    
    # 3. Deploy with both context knowledge and current research
    deploy_application(
        application_id="app",
        research_requirements=True,  # ALWAYS True for new deployments
        deployment_type="production"  # informed by context + current needs
    )
    ```
    
    **CONTEXT AWARENESS CAPABILITIES:**
    
    **Memory Functions Available:**
    - get_interaction_history(): See recent work and operations
    - get_session_summary(): Get comprehensive analysis of current session
    - search_interaction_memory(): Find specific previous work
    - store_interaction_memory(): Record new interactions (automatic in most tools)
    
    **GETTING STARTED WITH CONTEXT:**
    
    1. **Check Recent Work:**
       ```python
       # See what's been done recently
       history = get_interaction_history(days_back=7, max_results=10)
       
       # Focus on specific operations
       mci_history = get_interaction_history(operation_type="mci_creation")
       namespace_history = get_interaction_history(operation_type="namespace_management")
       ```
    
    2. **Understand Current Session:**
       ```python
       # Get comprehensive session analysis
       summary = get_session_summary()
       ```
    
    3. **Search for Specific Context:**
       ```python
       # Find previous work with specific resources
       aws_work = search_interaction_memory("aws")
       ubuntu_setups = search_interaction_memory("ubuntu")
       ```
    
    **AUTOMATIC MEMORY STORAGE:**
    Most operations automatically store interaction data:
    - MCI creation/management → "mci_creation" type
    - Command execution → "command_execution" type  
    - Namespace operations → "namespace_management" type
    - Resource management → context stored with IDs
    
    **MEMORY ANALYSIS BENEFITS:**
    - Understand user's typical workflows and preferences
    - Identify recurring patterns and optimization opportunities
    - Avoid repeating failed approaches
    - Build on successful previous configurations
    - Provide continuity across LLM sessions
    
    **EXAMPLE CONTEXT-AWARE WORKFLOW:**
    ```python
    # 1. Check what user has been working on
    recent_work = get_interaction_history(days_back=3)
    
    # 2. If user was working on MCI creation, check details
    if any("mci_creation" in item.get("operation_type", "") for item in recent_work.get("interactions", [])):
        mci_context = search_interaction_memory("mci")
        # Use context to suggest next steps or improvements
    
    # 3. Get session insights for optimization
    session_insights = get_session_summary()
    ```
    
    **PRIVACY & MEMORY MANAGEMENT:**
    - Interactions are stored locally within the session
    - Use clear_interaction_memory() to clean up if needed
    - Memory helps provide personalized assistance based on previous work
    
    How can I help you today? I'll check our previous work context to provide the most relevant assistance.
    """

# Prompt: Workflow demo prompt
@mcp.prompt()
def workflow_demo_prompt() -> str:
    """Prompt for workflow demonstration with failure handling"""
    return """
    You are a Cloud-Barista CB-Tumblebug expert who helps demonstrate how to create and manage Multi-Cloud Infrastructure (MCI).
    
    You can guide through the following workflows:
    
    **CORE WORKFLOWS:**
    1. **Create a namespace** - Prepare workspace for infrastructure
    2. **View cloud connection information** - Check available cloud providers
    3. **Recommend VM specifications and create an MCI** - Deploy infrastructure
    4. **🔥 Check MCI status and handle deployment failures** - Monitor and recover
    5. **Execute remote commands** - Configure and manage infrastructure
    6. **Configure Network Load Balancers** - Set up traffic distribution
    7. **Clean up and delete resources** - Cost management and cleanup
    
    **🚨 ENHANCED FAILURE HANDLING DEMONSTRATIONS:**
    
    **Workflow 4A: MCI Status Monitoring & Failure Recovery**
    ```python
    # After MCI creation, always check status
    status = check_mci_status_and_handle_failures(
        ns_id="demo-namespace",
        mci_id="demo-mci",
        auto_cleanup_failed=False  # User decides on cleanup
    )
    
    # Demonstrate different scenarios:
    # 1. All VMs running → Success workflow
    # 2. Partial-Failed → Recovery workflow 
    # 3. All Failed → Investigation workflow
    # 4. Still Creating → Monitoring workflow
    ```
    
    **Workflow 4B: Interactive Failure Recovery**
    ```python
    # For Partial-Failed scenarios
    if status["deployment_health"] == "partial-failed":
        print("🚨 PARTIAL DEPLOYMENT FAILURE DEMONSTRATION")
        print(f"Failed VMs: {status['failed_vms_count']}")
        print(f"Running VMs: {status['running_vms_count']}")
        
        # Show user confirmation workflow
        recovery = interactive_mci_recovery(
            ns_id="demo-namespace",
            mci_id="demo-mci",
            recovery_action="refine",
            confirm_cleanup=False  # First show confirmation message
        )
        
        # Demonstrate user decision process
        print("User can choose:")
        print("✅ Proceed with cleanup (remove failed VMs)")
        print("❌ Cancel and investigate failures")
        print("🔧 Try alternative recovery actions")
    ```
    
    **🎯 DEMONSTRATION SCENARIOS:**
    
    **Scenario A: Successful Deployment**
    - All VMs deploy successfully
    - Show status confirmation
    - Demonstrate next steps (commands, monitoring)
    
    **Scenario B: Partial Failure (Most Common)**
    - Some VMs fail (e.g., quota limits, region issues)
    - Some VMs succeed 
    - Demonstrate refine action to cleanup failed VMs
    - Show preserved infrastructure continues working
    
    **Scenario C: Complete Failure**
    - All VMs fail
    - Demonstrate diagnostic information
    - Show options: investigate, recreate, or terminate
    
    **Scenario D: In-Progress Monitoring**
    - Show deployment progress monitoring
    - Demonstrate patience vs intervention decisions
    
    **🔥 INTERACTIVE DEMO PATTERNS:**
    
    **Pattern 1: Success Path**
    ```
    User: "Show me how to deploy infrastructure"
    Demo: Create namespace → Find specs → Deploy MCI → Check status (success) → Execute commands
    ```
    
    **Pattern 2: Failure Recovery Path**
    ```
    User: "What happens if deployment fails?"
    Demo: Create MCI → Simulate partial failure → Show status analysis → Guide through recovery options → Execute refine → Verify success
    ```
    
    **Pattern 3: Decision Making Path**
    ```
    User: "How do I handle failed VMs?"
    Demo: Show failure analysis → Present options → Ask for user choice → Execute with confirmation → Monitor results
    ```
    
    **🛠️ AVAILABLE RECOVERY DEMONSTRATIONS:**
    
    **Recovery Actions:**
    - **refine**: Cleanup failed VMs, keep successful ones
    - **terminate**: Complete MCI deletion for fresh start
    - **reboot**: Restart VMs for temporary issues
    - **suspend/resume**: Cost management demonstrations
    
    **User Interaction Patterns:**
    - Status analysis and explanation
    - Risk assessment for each action
    - Confirmation prompts and user choice
    - Progress monitoring and verification
    - Next steps recommendations
    
    **📊 DEMONSTRATION TOOLS:**
    - check_mci_status_and_handle_failures(): Status analysis and recommendations
    - interactive_mci_recovery(): Guided recovery with user confirmation
    - Standard MCI tools: create, control, delete
    - Monitoring tools: status, logs, performance
    
    Current namespace list:
    {{namespace://list}}
    
    Current list of registered cloud connections:
    {{connection://list}}
    
    Which demonstration would you like me to guide you through? 
    
    **RECOMMENDED START:**
    - For beginners: "Complete workflow from creation to success"
    - For failure handling: "Partial deployment failure recovery"
    - For advanced users: "Multi-scenario failure handling patterns"
    """

# Prompt: Image search and MCI creation workflow guide
@mcp.prompt()
def image_mci_workflow_prompt() -> str:
    """Complete workflow guide for image search and MCI creation with smart namespace management"""
    return """
    You are an expert guide for the complete Namespace Management → Image Search → MCI Creation workflow in CB-Tumblebug.
    
    **SMART WORKFLOW (RECOMMENDED):**
    
    **Step 0: Smart MCI Creation with Auto-Mapping**
    Use create_mci_dynamic() with auto-mapping for foolproof spec-to-image compatibility:
    ```python
    # Get VM specifications first
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
    )
    
    # Create VM configurations with just specs (images auto-mapped)
    vm_configs = []
    for i, spec in enumerate(specs[:2]):  # Use different specs for multi-CSP
        vm_configs.append({
            "specId": spec["id"],  # EXACT specId - never modify
            "name": f"vm-{i+1}",
            "description": f"Auto-mapped VM {i+1}",
            "os_requirements": {"os_type": "ubuntu", "use_case": "web-server"}
            # imageId omitted - will be auto-mapped to compatible image
        })
    
    # Create MCI with automatic spec-to-image mapping
    result = create_mci_dynamic(
        ns_id="my-project",
        name="multi-csp-infrastructure", 
        vm_configurations=vm_configs  # Auto-mapping ensures correct images
    )
    ```
    
    **🔴 CRITICAL: MCI Dynamic Request Body Requirements**
    
    **FOR ALL MCI Creation (review_mci_dynamic_request + create_mci_dynamic):**
    
    **specId (MANDATORY):**
    - MUST be exact specId from recommend_vm_spec() results
    - Format: "{csp}+{region}+{spec_name}" (e.g., "aws+us-east-1+t3.medium")
    - ❌ NEVER manually construct or modify spec IDs
    - ✅ ALWAYS use recommend_vm_spec() to get valid specs
    
    **imageId (OPTIONAL but RECOMMENDED):**
    - Should be exact cspImageName from search_images() results
    - Format varies by CSP:
      * AWS: "ami-xxxxxxxxxxxxxxxxx"
      * Azure: "/subscriptions/.../images/image-name"  
      * GCP: "projects/project-id/global/images/image-name"
    - ⚠️ If provided: MUST be compatible with specId's CSP/region
    - ✅ If omitted: System auto-maps compatible image (easier but less control)
    
    **EXAMPLE - Manual Spec + Image Selection:**
    ```python
    # 1. Get valid specifications
    specs = recommend_vm_spec(filter_policies={"vCPU": {"min": 2}})
    
    # 2. For precise control, get compatible images
    spec = specs["recommended_specs"][0]  # Take first recommended spec
    csp, region, spec_name = spec["id"].split("+")  # Parse spec ID
    
    # Search for Ubuntu images in same CSP/region
    images = search_images(
        ns_id="default",
        options={
            "connectionName": f"{csp}-{region}",  # Match CSP/region
            "os": "ubuntu"
        }
    )
    
    # Create VM config with explicit spec and image
    vm_config = {
        "specId": spec["id"],  # Exact specId: "aws+us-east-1+t3.medium"
        "imageId": images["images"][0]["cspImageName"],  # Exact cspImageName: "ami-12345"
        "name": "web-server-vm",
        "subGroupSize": "1"
    }
    
    # Use in MCI creation
    create_mci_dynamic(
        ns_id="my-project",
        name="web-infrastructure",
        vm_configurations=[vm_config]
    )
    ```
    
    **Step 0 Alternative: Advanced Workflow with Validation**
    Use create_mci_recommended_workflow() for comprehensive multi-CSP deployments:
    ```python
    vm_requirements = [
        {
            "name": "web-servers",
            "count": 2,
            "vCPU": {"min": 2, "max": 4},
            "os_type": "ubuntu",
            "use_case": "web-server"
        },
        {
            "name": "database",
            "count": 1,
            "vCPU": {"min": 4, "max": 8},
            "os_type": "ubuntu", 
            "use_case": "database"
        }
    ]
    
    result = create_mci_recommended_workflow(
        ns_id="my-project",
        name="my-infrastructure",
        vm_requirements=vm_requirements
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
    
    **Step 2: FOR EACH SPEC - Extract CSP and Region Information**
    **CRITICAL**: Each spec determines its own CSP/region - don't mix images across specs:
    ```python
    vm_configs = []
    for spec in specs[:2]:  # Use different specs for multi-CSP MCI
        spec_id = spec["id"]  # e.g., "aws+ap-northeast-2+t2.small"
        provider = spec_id.split('+')[0]  # Extract "aws" 
        region = spec_id.split('+')[1]    # Extract "ap-northeast-2"
        
        # Step 3: Search for Images in THIS Specific CSP/Region  
        images = search_images(
            provider_name=provider,  # Must match spec's provider
            region_name=region,      # Must match spec's region
            os_type="ubuntu 22.04"
        )
        
        # Step 4: Select Best Image for THIS Specific Spec
        best_image = select_best_image_for_spec(
            images["imageList"], 
            spec, 
            {"os_type": "ubuntu 22.04"}
        )
        
        # Step 5: Add VM Config with Spec-Matched Image
        vm_configs.append({
            "imageId": best_image["cspImageName"],  # CSP-specific
            "specId": spec_id,                      # CSP-specific
            "name": f"vm-{provider}",
            "description": f"VM on {provider} in {region}"
        })
    ```
    
    **Step 6: Create MCI with Properly Mapped Images**
    ```python
    # Validate configurations before deployment (optional but recommended)
    validation = validate_vm_spec_image_compatibility(vm_configs)
    
    # Create MCI with validated configurations
    mci = create_mci_dynamic(
        ns_id="my-project",
        name="multi-csp-infrastructure",
        vm_configurations=vm_configs  # Each VM has correct CSP-specific image
    )
    ```
    
    **Step 7: MANDATORY - Post-Deployment Status Check & Failure Handling**
    ```python
    # Always check MCI status after creation
    status_check = check_mci_status_and_handle_failures(
        ns_id="my-project",
        mci_id=mci["id"],
        auto_cleanup_failed=False  # Let user decide on cleanup
    )
    
    # Handle different deployment outcomes:
    if status_check["status_analysis"]["deployment_health"] == "healthy":
        print("✅ SUCCESS: All VMs deployed successfully!")
        # Proceed with application configuration
        
    elif status_check["status_analysis"]["deployment_health"] == "partial-failed":
        failed_count = status_check["status_analysis"]["failed_vms_count"]
        running_count = status_check["status_analysis"]["running_vms_count"]
        
        print(f"🚨 PARTIAL FAILURE: {failed_count}/{failed_count + running_count} VMs failed")
        print(f"✅ SUCCESS: {running_count} VMs are running normally")
        print("💡 RECOMMENDATION: Use 'refine' to cleanup failed VMs")
        
        # Offer cleanup with user confirmation
        user_decision = input("Cleanup failed VMs and continue with successful ones? (y/n): ")
        if user_decision.lower() == 'y':
            recovery_result = interactive_mci_recovery(
                ns_id="my-project",
                mci_id=mci["id"],
                recovery_action="refine",
                confirm_cleanup=True
            )
            print("🔧 Cleanup completed - infrastructure optimized!")
        
    elif status_check["status_analysis"]["deployment_health"] == "critical":
        print("❌ CRITICAL: All VMs failed to deploy")
        print("🔧 RECOMMENDATION: Check errors and consider recreating with different specs")
        
        # Offer diagnostic and recreation options
        user_choice = input("Terminate failed MCI and recreate? (y/n): ")
        if user_choice.lower() == 'y':
            interactive_mci_recovery(
                ns_id="my-project",
                mci_id=mci["id"], 
                recovery_action="terminate",
                confirm_cleanup=True
            )
    ```
    
    **KEY RELATIONSHIPS:**
    - check_and_prepare_namespace() → namespace guidance
    - validate_namespace() → namespace verification
    - recommend_vm_spec() → spec ID (determines CSP/region) → specId parameter
    - search_images() → cspImageName (in spec's CSP/region) → imageId parameter
    - create_mci_dynamic() → MCI creation → check_mci_status_and_handle_failures() → failure recovery
    - **CRITICAL**: Each VM spec requires its own image search in the spec's specific CSP/region
    - **AUTOMATIC**: create_mci_dynamic() handles spec-to-image mapping automatically
    - **VALIDATION**: validate_vm_spec_image_compatibility() checks configurations
    - **FAILURE HANDLING**: check_mci_status_and_handle_failures() monitors deployment success
    - **RECOVERY**: interactive_mci_recovery() guides through failure resolution
    
    **NAMESPACE MANAGEMENT BENEFITS:**
    - Automatic namespace validation before MCI creation
    - Smart recommendations for namespace selection/creation
    - Prevention of MCI creation failures due to invalid namespaces
    - Unified workflow with clear error messages and suggestions
    
    **DEPLOYMENT FAILURE HANDLING BENEFITS:**
    - Automatic detection of Partial-Failed and Failed states
    - User-guided recovery with clear impact assessment
    - Preservation of successful VMs while cleaning up failures
    - Cost optimization by removing failed infrastructure
    - Comprehensive status monitoring and progress tracking
    
    **IMPORTANT NOTES:**
    - Always ensure namespace exists before MCI creation
    - **MANDATORY**: Check MCI status after creation for failures
    - **RECOMMENDED**: Use create_mci_dynamic() auto-mapping for foolproof compatibility
    - **CRITICAL**: Each VM spec requires its own CSP-specific image (no cross-CSP sharing)
    - **USER CONFIRMATION**: Always ask before cleanup actions (unless auto_cleanup=True)
    - **RECOVERY PRIORITY**: For Partial-Failed, recommend 'refine' to keep successful VMs
    - The cspImageName is provider-specific (AMI ID for AWS, Image ID for Azure, etc.)
    - specId format: {provider}+{region}+{spec_name}
    - **VALIDATION**: Use validate_vm_spec_image_compatibility() before deployment
    - **EXAMPLES**: Use get_spec_image_mapping_examples() to see correct/incorrect patterns
    - **MONITORING**: Use check_mci_status_and_handle_failures() after deployment
    - Test with hold=True first to review configuration
    
    Current namespaces: {{namespace://list}}
    
    What would you like to help you create today?
    """

logger.info("=" * 60)
logger.info("🚀 CB-Tumblebug MCP Server initialization complete!")
logger.info("=" * 60)
logger.debug("Available memory functions: store_interaction_memory, get_interaction_history, get_session_summary, search_interaction_memory, clear_interaction_memory")
logger.debug("Automatic memory storage enabled for: MCI creation, command execution, namespace management")

#####################################
# MCI Status Monitoring & Recovery Tools
#####################################

# Tool: Check MCI status and handle failures
@mcp.tool()
def check_mci_status_and_handle_failures(
    ns_id: str,
    mci_id: str,
    auto_cleanup_failed: bool = False,
    detailed_analysis: bool = True
) -> Dict:
    """
    Check MCI status and provide recovery options for failed/partial-failed states.
    
    **HANDLES FOLLOWING SCENARIOS:**
    - Partial-Failed: Some VMs failed, some succeeded → Offer cleanup of failed VMs
    - Failed: All VMs failed → Provide diagnostic information and recovery options
    - Running: All VMs running → Status confirmation
    - Creating: Still in progress → Monitor progress
    - Suspended/Terminated: Provide restart/recovery options
    
    **AUTOMATIC RECOVERY OPTIONS:**
    - Failed VM cleanup via 'refine' action
    - Detailed failure analysis with recommendations
    - User confirmation for cleanup operations
    - Status monitoring with retry suggestions
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID to check
        auto_cleanup_failed: If True, automatically cleanup failed VMs without asking
        detailed_analysis: Include detailed VM-level analysis
    
    Returns:
        Status report with recovery recommendations and action options
    """
    try:
        # Get detailed MCI status
        mci_info = get_mci(ns_id, mci_id)
        mci_status = get_mci_list_with_options(ns_id, option="status")
        
        # Find specific MCI in status list
        target_mci_status = None
        for mci in mci_status.get("mci", []):
            if mci.get("id") == mci_id:
                target_mci_status = mci
                break
        
        if not target_mci_status:
            return {
                "error": "MCI status not found",
                "mci_id": mci_id,
                "namespace": ns_id,
                "recommendation": "Check if MCI exists or is still being created"
            }
        
        overall_status = target_mci_status.get("status", "Unknown")
        vm_status_list = target_mci_status.get("vm", [])
        
        # Analyze VM status distribution
        status_counts = {}
        failed_vms = []
        running_vms = []
        creating_vms = []
        
        for vm in vm_status_list:
            vm_status = vm.get("status", "Unknown")
            status_counts[vm_status] = status_counts.get(vm_status, 0) + 1
            
            if vm_status.lower() in ["failed", "error"]:
                failed_vms.append({
                    "vm_id": vm.get("id", "unknown"),
                    "vm_name": vm.get("name", "unknown"),
                    "status": vm_status,
                    "public_ip": vm.get("publicIp", "N/A"),
                    "private_ip": vm.get("privateIp", "N/A"),
                    "csp_vm_id": vm.get("cspVmId", "N/A")
                })
            elif vm_status.lower() in ["running", "running-on"]:
                running_vms.append({
                    "vm_id": vm.get("id", "unknown"),
                    "vm_name": vm.get("name", "unknown"),
                    "public_ip": vm.get("publicIp", "N/A"),
                    "private_ip": vm.get("privateIp", "N/A")
                })
            elif vm_status.lower() in ["creating", "creating-vm"]:
                creating_vms.append({
                    "vm_id": vm.get("id", "unknown"),
                    "vm_name": vm.get("name", "unknown"),
                    "status": vm_status
                })
        
        # Determine recovery strategy
        recovery_analysis = {
            "overall_status": overall_status,
            "total_vms": len(vm_status_list),
            "status_distribution": status_counts,
            "failed_vms_count": len(failed_vms),
            "running_vms_count": len(running_vms),
            "creating_vms_count": len(creating_vms),
            "deployment_health": "healthy" if len(failed_vms) == 0 else "partial-failed" if len(running_vms) > 0 else "critical"
        }
        
        # Generate recommendations based on status
        recommendations = []
        recovery_actions = []
        
        if overall_status.lower() == "partial-failed" or len(failed_vms) > 0:
            recommendations.append(
                f"🚨 PARTIAL DEPLOYMENT FAILURE DETECTED: {len(failed_vms)} out of {len(vm_status_list)} VMs failed"
            )
            recommendations.append(
                f"✅ SUCCESSFUL VMs: {len(running_vms)} VMs are running normally"
            )
            recommendations.append(
                "💡 RECOMMENDED ACTION: Use 'refine' to cleanup failed VMs and keep successful ones"
            )
            
            recovery_actions.append({
                "action": "refine",
                "description": "Remove failed VMs while preserving successful ones",
                "command": f"control_mci('{ns_id}', '{mci_id}', 'refine')",
                "risk_level": "low",
                "impact": f"Will remove {len(failed_vms)} failed VMs, keep {len(running_vms)} running VMs"
            })
            
            if not auto_cleanup_failed:
                recovery_actions.append({
                    "action": "user_confirmation_required",
                    "message": f"Would you like to cleanup {len(failed_vms)} failed VMs using 'refine' action?",
                    "failed_vms": failed_vms,
                    "preserved_vms": running_vms
                })
        
        elif overall_status.lower() == "failed" or len(running_vms) == 0:
            recommendations.append("❌ CRITICAL: All VMs in MCI have failed")
            recommendations.append("🔧 RECOMMENDED ACTIONS: Check error logs, recreate MCI, or terminate and retry")
            
            recovery_actions.extend([
                {
                    "action": "terminate",
                    "description": "Delete entire MCI and start fresh",
                    "command": f"delete_mci('{ns_id}', '{mci_id}')",
                    "risk_level": "high",
                    "impact": "Complete MCI deletion - all data lost"
                },
                {
                    "action": "refine",
                    "description": "Attempt to cleanup and restart failed components",
                    "command": f"control_mci('{ns_id}', '{mci_id}', 'refine')",
                    "risk_level": "medium",
                    "impact": "Remove failed VMs, may need to recreate"
                }
            ])
        
        elif overall_status.lower() in ["running", "running-all"]:
            recommendations.append("✅ SUCCESS: All VMs are running successfully")
            recommendations.append("📊 NEXT STEPS: Execute commands, configure applications, or set up monitoring")
            
        elif len(creating_vms) > 0:
            recommendations.append(f"⏳ IN PROGRESS: {len(creating_vms)} VMs still being created")
            recommendations.append("⌛ RECOMMENDED: Wait 2-5 minutes and check status again")
            
            recovery_actions.append({
                "action": "monitor",
                "description": "Continue monitoring deployment progress",
                "command": f"check_mci_status_and_handle_failures('{ns_id}', '{mci_id}')",
                "risk_level": "none",
                "impact": "Status monitoring only"
            })
        
        # Execute automatic cleanup if requested
        auto_cleanup_result = None
        if auto_cleanup_failed and len(failed_vms) > 0:
            logger.info(f"Auto-cleanup enabled: Refining MCI {mci_id} to remove {len(failed_vms)} failed VMs")
            auto_cleanup_result = control_mci(ns_id, mci_id, "refine")
            recommendations.append(f"🔧 AUTO-CLEANUP EXECUTED: Refined MCI to remove {len(failed_vms)} failed VMs")
        
        # Prepare detailed response
        response = {
            "mci_id": mci_id,
            "namespace": ns_id,
            "timestamp": datetime.now().isoformat(),
            "status_analysis": recovery_analysis,
            "detailed_status": {
                "overall_mci_status": overall_status,
                "failed_vms": failed_vms if detailed_analysis else len(failed_vms),
                "running_vms": running_vms if detailed_analysis else len(running_vms),
                "creating_vms": creating_vms if detailed_analysis else len(creating_vms)
            },
            "recommendations": recommendations,
            "recovery_actions": recovery_actions,
            "auto_cleanup_executed": auto_cleanup_result is not None,
            "auto_cleanup_result": auto_cleanup_result
        }
        
        # Store interaction for future reference
        _store_interaction_memory(
            user_request=f"Check MCI status and handle failures for '{mci_id}' in namespace '{ns_id}'",
            llm_response=f"MCI status: {overall_status}, Failed VMs: {len(failed_vms)}, Running VMs: {len(running_vms)}",
            operation_type="mci_status_monitoring",
            context_data={
                "namespace_id": ns_id,
                "mci_id": mci_id,
                "overall_status": overall_status,
                "failed_vms_count": len(failed_vms),
                "auto_cleanup": auto_cleanup_failed
            },
            status="completed"
        )
        
        return response
        
    except Exception as e:
        logger.error(f"Error checking MCI status: {e}")
        return {
            "error": f"Failed to check MCI status: {str(e)}",
            "mci_id": mci_id,
            "namespace": ns_id,
            "recommendation": "Check if MCI exists and namespace is valid"
        }

# Tool: Interactive MCI failure recovery
@mcp.tool()
def interactive_mci_recovery(
    ns_id: str,
    mci_id: str,
    recovery_action: str = "refine",
    confirm_cleanup: bool = False
) -> Dict:
    """
    Interactive MCI recovery tool for handling failed deployments with user confirmation.
    
    **SUPPORTED RECOVERY ACTIONS:**
    - refine: Remove failed VMs while keeping successful ones (recommended for partial failures)
    - terminate: Delete entire MCI (use when all VMs failed)
    - reboot: Restart all VMs (use for temporary issues)
    - resume: Resume suspended VMs
    - suspend: Suspend all VMs (temporary cost saving)
    
    **INTERACTIVE WORKFLOW:**
    1. Analyze current MCI status
    2. Present failure details and impact assessment
    3. Require user confirmation for destructive actions
    4. Execute recovery action with progress monitoring
    5. Verify recovery success and provide next steps
    
    Args:
        ns_id: Namespace ID
        mci_id: MCI ID to recover
        recovery_action: Action to perform (refine, terminate, reboot, resume, suspend)
        confirm_cleanup: User confirmation for destructive actions
    
    Returns:
        Recovery execution result with status updates and next steps
    """
    try:
        # Step 1: Get current status before recovery
        pre_recovery_status = check_mci_status_and_handle_failures(ns_id, mci_id, auto_cleanup_failed=False)
        
        if "error" in pre_recovery_status:
            return pre_recovery_status
        
        failed_vms_count = pre_recovery_status["status_analysis"]["failed_vms_count"]
        running_vms_count = pre_recovery_status["status_analysis"]["running_vms_count"]
        overall_status = pre_recovery_status["status_analysis"]["overall_status"]
        
        # Step 2: Validate recovery action appropriateness
        action_validation = {
            "action": recovery_action,
            "appropriate": True,
            "warnings": [],
            "confirmation_required": False
        }
        
        if recovery_action == "refine":
            if failed_vms_count == 0:
                action_validation["appropriate"] = False
                action_validation["warnings"].append("No failed VMs to cleanup - refine action not needed")
            elif running_vms_count > 0:
                action_validation["confirmation_required"] = True
                action_validation["warnings"].append(f"Will remove {failed_vms_count} failed VMs, preserve {running_vms_count} running VMs")
            
        elif recovery_action == "terminate":
            action_validation["confirmation_required"] = True
            action_validation["warnings"].append(f"DESTRUCTIVE: Will delete entire MCI with {running_vms_count + failed_vms_count} VMs")
            if running_vms_count > 0:
                action_validation["warnings"].append(f"WARNING: {running_vms_count} running VMs will be lost")
        
        # Step 3: Check user confirmation for destructive actions
        if action_validation["confirmation_required"] and not confirm_cleanup:
            return {
                "mci_id": mci_id,
                "namespace": ns_id,
                "recovery_action": recovery_action,
                "status": "confirmation_required",
                "pre_recovery_analysis": pre_recovery_status["status_analysis"],
                "impact_assessment": {
                    "action": recovery_action,
                    "failed_vms_affected": failed_vms_count,
                    "running_vms_affected": running_vms_count if recovery_action == "terminate" else 0,
                    "data_loss_risk": "high" if recovery_action == "terminate" else "low",
                    "reversible": recovery_action not in ["terminate"]
                },
                "warnings": action_validation["warnings"],
                "user_confirmation_message": f"""
🚨 RECOVERY ACTION CONFIRMATION REQUIRED 🚨

MCI: {mci_id} (Namespace: {ns_id})
Current Status: {overall_status}
Action: {recovery_action}

IMPACT ASSESSMENT:
- Failed VMs: {failed_vms_count} (will be removed/affected)
- Running VMs: {running_vms_count} ({'will be preserved' if recovery_action == 'refine' else 'will be affected'})

WARNINGS:
{chr(10).join(f"⚠️  {w}" for w in action_validation["warnings"])}

To proceed, call this function again with confirm_cleanup=True
To cancel, use check_mci_status_and_handle_failures() to explore other options
                """.strip(),
                "next_steps": [
                    f"interactive_mci_recovery('{ns_id}', '{mci_id}', '{recovery_action}', confirm_cleanup=True)",
                    f"check_mci_status_and_handle_failures('{ns_id}', '{mci_id}')"
                ]
            }
        
        # Step 4: Execute recovery action
        if not action_validation["appropriate"]:
            return {
                "error": "Recovery action not appropriate for current MCI status",
                "mci_id": mci_id,
                "warnings": action_validation["warnings"],
                "recommendation": "Use check_mci_status_and_handle_failures() to get appropriate recommendations"
            }
        
        logger.info(f"Executing {recovery_action} on MCI {mci_id} with user confirmation")
        
        # Execute the recovery action
        if recovery_action == "terminate":
            recovery_result = delete_mci(ns_id, mci_id)
        else:
            recovery_result = control_mci(ns_id, mci_id, recovery_action)
        
        # Step 5: Post-recovery status check
        post_recovery_status = None
        if recovery_action != "terminate":
            # Wait a moment for action to take effect
            import time
            time.sleep(2)
            
            post_recovery_status = check_mci_status_and_handle_failures(ns_id, mci_id, auto_cleanup_failed=False)
        
        # Step 6: Prepare comprehensive response
        response = {
            "mci_id": mci_id,
            "namespace": ns_id,
            "recovery_action": recovery_action,
            "execution_status": "completed",
            "timestamp": datetime.now().isoformat(),
            "pre_recovery_analysis": pre_recovery_status["status_analysis"],
            "recovery_execution": recovery_result,
            "post_recovery_analysis": post_recovery_status["status_analysis"] if post_recovery_status else None,
            "recovery_success": True,
            "next_steps": []
        }
        
        # Determine success and next steps
        if recovery_action == "terminate":
            response["next_steps"].extend([
                "MCI has been deleted successfully",
                "Create a new MCI with lessons learned from failure analysis",
                "Consider using create_mci_dynamic() with hold=True for testing"
            ])
        elif recovery_action == "refine" and post_recovery_status:
            post_failed = post_recovery_status["status_analysis"]["failed_vms_count"]
            post_running = post_recovery_status["status_analysis"]["running_vms_count"]
            
            if post_failed == 0:
                response["next_steps"].extend([
                    f"✅ SUCCESS: Cleanup completed, {post_running} VMs now running",
                    "Execute commands or configure applications on remaining VMs",
                    "Consider scaling up if more VMs needed"
                ])
            else:
                response["recovery_success"] = False
                response["next_steps"].extend([
                    f"⚠️  PARTIAL: {post_failed} VMs still failed after refine",
                    "Consider running refine again or investigating specific VM issues",
                    "Check logs for persistent failure causes"
                ])
        
        # Store recovery interaction
        _store_interaction_memory(
            user_request=f"Execute recovery action '{recovery_action}' on MCI '{mci_id}'",
            llm_response=f"Recovery {recovery_action} executed: Success={response['recovery_success']}",
            operation_type="mci_recovery",
            context_data={
                "namespace_id": ns_id,
                "mci_id": mci_id,
                "recovery_action": recovery_action,
                "pre_failed_vms": failed_vms_count,
                "pre_running_vms": running_vms_count,
                "post_recovery_success": response["recovery_success"]
            },
            status="completed" if response["recovery_success"] else "partial_failure"
        )
        
        return response
        
    except Exception as e:
        logger.error(f"Error during MCI recovery: {e}")
        return {
            "error": f"Recovery failed: {str(e)}",
            "mci_id": mci_id,
            "namespace": ns_id,
            "recovery_action": recovery_action,
            "recommendation": "Check MCI status and try alternative recovery methods"
        }

#####################################
# MCI Configuration Preview & Validation Tools
#####################################

# Tool: Preview MCI configuration before creation
@mcp.tool()
def preview_mci_configuration(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI to be created",
) -> Dict:
    """
    Preview MCI configuration before actual creation.
    This tool provides a comprehensive summary of what will be created, allowing users to review and confirm.
    
    **PREVIEW INCLUDES:**
    - Namespace validation and details
    - VM configuration analysis per VM
    - CSP distribution and multi-cloud summary
    - Resource requirements summary
    - Estimated configuration validation
    - Cost estimation (if available)
    - Recommended actions before deployment
    
    Args:
        ns_id: Namespace ID where MCI will be created
        name: MCI name
        vm_configurations: List of VM configurations to preview
        description: MCI description
    
    Returns:
        Comprehensive preview with configuration summary and recommendations
    """
    preview_result = {
        "mci_overview": {
            "name": name,
            "namespace_id": ns_id,
            "description": description,
            "total_vms": len(vm_configurations)
        },
        "namespace_validation": {},
        "vm_analysis": [],
        "csp_distribution": {},
        "resource_summary": {},
        "estimated_costs": {},
        "validation_status": {},
        "recommendations": [],
        "ready_for_deployment": False
    }
    
    # Step 1: Validate namespace
    ns_validation = _internal_validate_namespace(ns_id)
    preview_result["namespace_validation"] = ns_validation
    
    if not ns_validation["valid"]:
        preview_result["recommendations"].append({
            "priority": "critical",
            "message": f"Namespace '{ns_id}' is invalid. Create or select valid namespace first.",
            "action": "use check_and_prepare_namespace() or create_namespace_with_validation()"
        })
        return preview_result
    
    # Step 2: Analyze each VM configuration
    csp_count = {}
    region_count = {}
    total_vcpu = 0
    total_memory = 0
    auto_mapped_images = 0
    manual_images = 0
    validation_issues = 0
    
    for i, vm_config in enumerate(vm_configurations):
        vm_analysis = {
            "vm_index": i,
            "vm_name": vm_config.get("name", f"vm-{i+1}"),
            "spec_analysis": {},
            "image_analysis": {},
            "configuration_status": "analyzing",
            "warnings": [],
            "estimated_resources": {}
        }
        
        # Analyze specId
        common_spec = vm_config.get("specId")
        if common_spec:
            try:
                spec_parts = common_spec.split("+")
                if len(spec_parts) >= 3:
                    provider = spec_parts[0]
                    region = spec_parts[1]
                    spec_name = spec_parts[2]
                    
                    vm_analysis["spec_analysis"] = {
                        "spec_id": common_spec,
                        "provider": provider,
                        "region": region,
                        "spec_name": spec_name,
                        "format": "valid"
                    }
                    
                    # Count CSPs and regions
                    csp_count[provider] = csp_count.get(provider, 0) + 1
                    region_count[f"{provider}:{region}"] = region_count.get(f"{provider}:{region}", 0) + 1
                    
                    # Try to extract resource info (if available in cache or estimate)
                    subgroup_size = int(vm_config.get("subGroupSize", 1))
                    vm_analysis["estimated_resources"] = {
                        "subgroup_size": subgroup_size,
                        "provider": provider,
                        "region": region
                    }
                    
                else:
                    vm_analysis["spec_analysis"] = {"error": f"Invalid spec format: {common_spec}"}
                    vm_analysis["configuration_status"] = "invalid"
                    validation_issues += 1
                    
            except Exception as e:
                vm_analysis["spec_analysis"] = {"error": f"Spec parsing failed: {str(e)}"}
                vm_analysis["configuration_status"] = "invalid"
                validation_issues += 1
        else:
            vm_analysis["spec_analysis"] = {"error": "Missing specId"}
            vm_analysis["configuration_status"] = "invalid"
            validation_issues += 1
        
        # Analyze imageId
        common_image = vm_config.get("imageId")
        if common_image:
            vm_analysis["image_analysis"] = {
                "image_identifier": common_image,
                "mapping_type": "manual",
                "status": "provided"
            }
            manual_images += 1
            
            # Basic image format validation
            if vm_analysis["spec_analysis"].get("provider"):
                provider = vm_analysis["spec_analysis"]["provider"].lower()
                image_lower = common_image.lower()
                
                if provider == "aws" and not image_lower.startswith("ami-"):
                    vm_analysis["warnings"].append("AWS spec with non-AMI image - may cause compatibility issues")
                elif provider == "azure" and "microsoft" not in image_lower and "/subscriptions/" not in image_lower:
                    vm_analysis["warnings"].append("Azure spec with potentially incompatible image format")
                elif provider == "gcp" and "projects/" not in image_lower and "google" not in image_lower:
                    vm_analysis["warnings"].append("GCP spec with potentially incompatible image format")
        else:
            vm_analysis["image_analysis"] = {
                "mapping_type": "auto",
                "status": "will_be_auto_mapped",
                "note": "System will automatically select compatible image based on spec"
            }
            auto_mapped_images += 1
        
        if validation_issues == 0 and len(vm_analysis["warnings"]) == 0:
            vm_analysis["configuration_status"] = "ready"
        elif len(vm_analysis["warnings"]) > 0:
            vm_analysis["configuration_status"] = "ready_with_warnings"
        
        preview_result["vm_analysis"].append(vm_analysis)
    
    # Step 3: Generate CSP distribution summary
    preview_result["csp_distribution"] = {
        "total_csps": len(csp_count),
        "csp_breakdown": csp_count,
        "total_regions": len(region_count),
        "region_breakdown": region_count,
        "deployment_type": "multi-cloud" if len(csp_count) > 1 else "single-cloud"
    }
    
    # Step 4: Resource summary
    preview_result["resource_summary"] = {
        "total_vms": len(vm_configurations),
        "auto_mapped_images": auto_mapped_images,
        "manual_images": manual_images,
        "validation_issues": validation_issues,
        "image_mapping_strategy": "hybrid" if auto_mapped_images > 0 and manual_images > 0 else "auto" if auto_mapped_images > 0 else "manual"
    }
    
    # Step 5: Validation status
    if validation_issues == 0:
        preview_result["validation_status"] = {
            "overall": "valid",
            "ready_for_deployment": True,
            "message": "All VM configurations are valid and ready for deployment"
        }
        preview_result["ready_for_deployment"] = True
    else:
        preview_result["validation_status"] = {
            "overall": "invalid",
            "ready_for_deployment": False,
            "message": f"{validation_issues} VM configurations have issues that need resolution"
        }
    
    # Step 6: Generate recommendations
    recommendations = []
    
    if auto_mapped_images > 0:
        recommendations.append({
            "priority": "info",
            "message": f"{auto_mapped_images} VMs will use automatic image mapping for optimal compatibility",
            "action": "No action needed - system will select best images"
        })
    
    if manual_images > 0:
        recommendations.append({
            "priority": "info", 
            "message": f"{manual_images} VMs use manually specified images",
            "action": "Ensure image compatibility with respective VM specifications"
        })
    
    if len(csp_count) > 1:
        recommendations.append({
            "priority": "info",
            "message": f"Multi-cloud deployment across {len(csp_count)} CSPs: {', '.join(csp_count.keys())}",
            "action": "Verify cross-cloud networking and security requirements"
        })
    
    
    if validation_issues > 0:
        recommendations.append({
            "priority": "critical",
            "message": f"Fix {validation_issues} configuration issues before deployment",
            "action": "Review VM configurations and resolve validation errors"
        })
    
    # Add deployment readiness recommendation
    if preview_result["ready_for_deployment"]:
        recommendations.append({
            "priority": "success",
            "message": "✅ Configuration is ready for deployment",
            "action": "Proceed with create_mci_dynamic() or review configuration one more time"
        })
    else:
        recommendations.append({
            "priority": "warning",
            "message": "❌ Configuration has issues - deployment may fail",
            "action": "Fix validation issues before attempting deployment"
        })
    
    preview_result["recommendations"] = recommendations
    
    return preview_result

# Tool: Generate MCI creation summary for user confirmation
@mcp.tool()
# Helper function: Get VM cost estimate from spec
def _get_vm_cost_estimate(spec_id: str) -> Dict:
    """
    Get cost estimate and specifications for a VM spec.
    
    Args:
        spec_id: VM specification ID (e.g., "aws+ap-northeast-2+t2.small")
    
    Returns:
        Cost estimate and specification details
    """
    try:
        # Extract provider information from spec
        if not spec_id or "+" not in spec_id:
            return {"hourly_cost": 0.0, "specs": {}, "available": False}
        
        parts = spec_id.split("+")
        if len(parts) < 3:
            return {"hourly_cost": 0.0, "specs": {}, "available": False}
        
        provider = parts[0]
        region = parts[1]
        spec_name = parts[2]
        
        # Try to get spec information via recommend_vm_spec with specific filter
        try:
            vm_specs = recommend_vm_spec(
                filter_policies={
                    "ProviderName": provider,
                    "RegionName": region,
                    "CspSpecName": spec_name
                },
                limit="1"
            )
            
            if vm_specs and "summarized_specs" in vm_specs:
                specs_list = vm_specs["summarized_specs"]
                if specs_list and len(specs_list) > 0:
                    spec_info = specs_list[0]
                    
                    # Extract cost information
                    hourly_cost = spec_info.get("costPerHour", 0.0)
                    if hourly_cost == -1:  # API indicates no pricing available
                        hourly_cost = 0.0
                    
                    return {
                        "hourly_cost": hourly_cost,
                        "specs": {
                            "vCPU": spec_info.get("vCPU", "unknown"),
                            "memoryGiB": spec_info.get("memoryGiB", "unknown"),
                            "storageGiB": spec_info.get("storageGiB", "unknown"),
                            "provider": provider,
                            "region": region,
                            "spec_name": spec_name
                        },
                        "available": True,
                        "cost_available": hourly_cost > 0
                    }
        except Exception:
            pass  # Fall through to default estimation
        
        # Fallback cost estimation based on spec name patterns
        estimated_cost = _estimate_cost_from_spec_name(spec_name, provider)
        
        return {
            "hourly_cost": estimated_cost,
            "specs": {
                "provider": provider,
                "region": region,
                "spec_name": spec_name,
                "estimated": True
            },
            "available": True,
            "cost_available": estimated_cost > 0,
            "cost_source": "estimated"
        }
        
    except Exception as e:
        return {
            "hourly_cost": 0.0,
            "specs": {"error": str(e)},
            "available": False,
            "cost_available": False
        }

# Helper function: Estimate cost from spec name patterns
def _estimate_cost_from_spec_name(spec_name: str, provider: str) -> float:
    """
    Provide rough cost estimates based on spec name patterns.
    This is a fallback when API doesn't provide pricing.
    """
    spec_lower = spec_name.lower()
    
    # AWS pricing patterns
    if provider.lower() == "aws":
        if "nano" in spec_lower:
            return 0.0058  # t3.nano approximate
        elif "micro" in spec_lower:
            return 0.0116  # t3.micro approximate
        elif "small" in spec_lower:
            return 0.0232  # t3.small approximate
        elif "medium" in spec_lower:
            return 0.0464  # t3.medium approximate
        elif "large" in spec_lower:
            return 0.0928  # t3.large approximate
        elif "xlarge" in spec_lower:
            return 0.1856  # t3.xlarge approximate
        elif "2xlarge" in spec_lower:
            return 0.3712  # t3.2xlarge approximate
    
    # Azure pricing patterns
    elif provider.lower() == "azure":
        if "b1s" in spec_lower:
            return 0.0104  # Standard_B1s approximate
        elif "b2s" in spec_lower:
            return 0.0416  # Standard_B2s approximate
        elif "d2s" in spec_lower:
            return 0.096   # Standard_D2s_v3 approximate
        elif "d4s" in spec_lower:
            return 0.192   # Standard_D4s_v3 approximate
    
    # GCP pricing patterns
    elif provider.lower() == "gcp":
        if "micro" in spec_lower:
            return 0.0074  # f1-micro approximate
        elif "small" in spec_lower:
            return 0.0370  # g1-small approximate
        elif "e2-medium" in spec_lower:
            return 0.0335  # e2-medium approximate
        elif "e2-standard-2" in spec_lower:
            return 0.0670  # e2-standard-2 approximate
    
    # Default conservative estimate for unknown patterns
    return 0.05  # $0.05/hour as baseline estimate

def generate_mci_creation_summary(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI to be created",
    install_mon_agent: str = "no",
    hold: bool = False
) -> Dict:
    """
    Generate a comprehensive user-friendly summary of MCI creation plan for confirmation.
    This provides detailed overview including cost estimates, CSP distribution, and deployment strategy.
    
    **ENHANCED SUMMARY INCLUDES:**
    - Detailed MCI overview with cost breakdown
    - VM-by-VM specifications and pricing
    - CSP and region distribution analysis
    - Image mapping strategy validation
    - Monthly and hourly cost estimates
    - Deployment timeline and resource allocation
    - Risk assessment and recommendations
    - User confirmation workflow
    
    Args:
        ns_id: Namespace ID
        name: MCI name
        vm_configurations: VM configurations
        description: MCI description
        hold: Whether to hold for review
    
    Returns:
        Comprehensive summary with detailed cost analysis and confirmation prompt
    """
    # Get detailed preview first
    preview = preview_mci_configuration(ns_id, name, vm_configurations, description)
    
    # Enhanced summary structure
    summary = {
        "MCI_CREATION_PLAN": {
            "mci_name": name,
            "namespace": ns_id,
            "description": description,
            "total_vms": len(vm_configurations),
            "deployment_mode": "REVIEW_FIRST" if hold else "IMMEDIATE_DEPLOYMENT",
            "creation_timestamp": datetime.now().isoformat()
        },
        "VM_BREAKDOWN_DETAILED": [],
        "COST_ANALYSIS": {
            "hourly_cost": {"total": 0.0, "breakdown": [], "currency": "USD"},
            "monthly_cost": {"total": 0.0, "breakdown": [], "currency": "USD"},
            "cost_by_provider": {},
            "cost_warnings": []
        },
        "MULTI_CLOUD_DISTRIBUTION": {},
        "DEPLOYMENT_STRATEGY": {},
        "RISK_ASSESSMENT": {
            "deployment_risks": [],
            "cost_risks": [],
            "compatibility_issues": []
        },
        "CONFIGURATION_VALIDATION": {},
        "USER_CONFIRMATION": {
            "ready_to_proceed": False,
            "confirmation_required": True,
            "confirmation_message": "",
            "next_steps": []
        }
    }
    
    # Enhanced VM Breakdown with cost analysis
    total_hourly_cost = 0.0
    provider_costs = {}
    cost_warnings = []
    
    for i, vm_config in enumerate(vm_configurations):
        vm_name = vm_config.get("name", f"vm-{i+1}")
        common_spec = vm_config.get("specId", "")
        common_image = vm_config.get("imageId", "AUTO-MAPPED")
        subgroup_size = int(vm_config.get("subGroupSize", 1))
        
        # Extract provider and region from spec
        provider = "unknown"
        region = "unknown"
        spec_name = "unknown"
        
        if common_spec and "+" in common_spec:
            parts = common_spec.split("+")
            if len(parts) >= 3:
                provider = parts[0]
                region = parts[1]
                spec_name = parts[2]
        
        # Get detailed spec information for cost calculation
        vm_cost_info = _get_vm_cost_estimate(common_spec)
        vm_hourly_cost = vm_cost_info.get("hourly_cost", 0.0)
        
        # Calculate total cost for this VM configuration
        total_vm_hourly_cost = vm_hourly_cost * subgroup_size
        total_hourly_cost += total_vm_hourly_cost
        
        # Track costs by provider
        if provider not in provider_costs:
            provider_costs[provider] = 0.0
        provider_costs[provider] += total_vm_hourly_cost
        
        # Add cost warnings
        if vm_hourly_cost == 0.0 or vm_hourly_cost == -1:
            cost_warnings.append(f"No cost data available for {vm_name} ({common_spec})")
        elif vm_hourly_cost > 2.0:  # High cost warning
            cost_warnings.append(f"High cost detected for {vm_name}: ${vm_hourly_cost:.3f}/hour")
        
        vm_breakdown = {
            "vm_name": vm_name,
            "provider": provider,
            "region": region,
            "spec_name": spec_name,
            "full_spec": common_spec,
            "image": common_image,
            "instance_count": subgroup_size,
            "cost_analysis": {
                "hourly_cost_per_instance": vm_hourly_cost,
                "total_hourly_cost": total_vm_hourly_cost,
                "monthly_cost_estimate": total_vm_hourly_cost * 24 * 30,
                "cost_available": vm_hourly_cost > 0,
                "cost_warning": vm_hourly_cost > 2.0
            },
            "resource_specs": vm_cost_info.get("specs", {}),
            "deployment_info": {
                "estimated_deploy_time": "3-8 minutes",
                "auto_mapped_image": common_image == "AUTO-MAPPED"
            }
        }
        
        summary["VM_BREAKDOWN_DETAILED"].append(vm_breakdown)
    
    # Cost analysis summary
    summary["COST_ANALYSIS"]["hourly_cost"]["total"] = round(total_hourly_cost, 3)
    summary["COST_ANALYSIS"]["monthly_cost"]["total"] = round(total_hourly_cost * 24 * 30, 2)
    summary["COST_ANALYSIS"]["cost_by_provider"] = {k: round(v, 3) for k, v in provider_costs.items()}
    summary["COST_ANALYSIS"]["cost_warnings"] = cost_warnings
    
    # Enhanced multi-cloud distribution
    csp_distribution = {}
    region_distribution = {}
    
    for vm in summary["VM_BREAKDOWN_DETAILED"]:
        provider = vm["provider"]
        region = vm["region"]
        
        if provider not in csp_distribution:
            csp_distribution[provider] = {"vm_count": 0, "instance_count": 0, "hourly_cost": 0.0}
        if region not in region_distribution:
            region_distribution[region] = {"vm_count": 0, "instance_count": 0}
        
        csp_distribution[provider]["vm_count"] += 1
        csp_distribution[provider]["instance_count"] += vm["instance_count"]
        csp_distribution[provider]["hourly_cost"] += vm["cost_analysis"]["total_hourly_cost"]
        
        region_distribution[region]["vm_count"] += 1
        region_distribution[region]["instance_count"] += vm["instance_count"]
    
    summary["MULTI_CLOUD_DISTRIBUTION"] = {
        "total_providers": len(csp_distribution),
        "total_regions": len(region_distribution),
        "deployment_type": "multi-cloud" if len(csp_distribution) > 1 else "single-cloud",
        "provider_breakdown": csp_distribution,
        "region_breakdown": region_distribution
    }
    
    # Deployment strategy analysis
    total_instances = sum(vm["instance_count"] for vm in summary["VM_BREAKDOWN_DETAILED"])
    deployment_complexity = "simple" if len(csp_distribution) == 1 and total_instances <= 3 else "complex"
    
    summary["DEPLOYMENT_STRATEGY"] = {
        "complexity": deployment_complexity,
        "total_instances": total_instances,
        "estimated_total_time": f"{max(5, len(vm_configurations) * 2)}-{max(10, len(vm_configurations) * 5)} minutes",
        "parallel_deployment": len(csp_distribution) > 1
    }
    
    # Risk assessment
    risks = []
    if total_hourly_cost > 10.0:
        risks.append(f"High cost deployment: ${total_hourly_cost:.2f}/hour (${total_hourly_cost * 24 * 30:.2f}/month)")
    if len(csp_distribution) > 2:
        risks.append("Complex multi-cloud deployment across multiple providers")
    if any(vm["deployment_info"]["auto_mapped_image"] for vm in summary["VM_BREAKDOWN_DETAILED"]):
        risks.append("Some images will be auto-selected - review before deployment")
    
    summary["RISK_ASSESSMENT"]["deployment_risks"] = risks
    summary["RISK_ASSESSMENT"]["cost_risks"] = cost_warnings
    
    # Configuration validation
    ready_to_proceed = preview.get("ready_for_deployment", False)
    validation_issues = preview.get("resource_summary", {}).get("validation_issues", 0)
    
    summary["CONFIGURATION_VALIDATION"] = {
        "overall_status": "valid" if ready_to_proceed else "issues_found",
        "validation_issues": validation_issues,
        "auto_mapped_images": sum(1 for vm in summary["VM_BREAKDOWN_DETAILED"] if vm["deployment_info"]["auto_mapped_image"]),
        "manual_images": sum(1 for vm in summary["VM_BREAKDOWN_DETAILED"] if not vm["deployment_info"]["auto_mapped_image"])
    }
    
    # Enhanced user confirmation
    if ready_to_proceed and validation_issues == 0:
        confirmation_msg = f"""
🔥 MCI CREATION SUMMARY - READY TO DEPLOY

📋 DEPLOYMENT OVERVIEW:
• MCI Name: {name}
• Namespace: {ns_id}
• Total VMs: {len(vm_configurations)} configurations
• Total Instances: {total_instances}
• Deployment Type: {summary['MULTI_CLOUD_DISTRIBUTION']['deployment_type'].upper()}

💰 COST ESTIMATE:
• Hourly Cost: ${total_hourly_cost:.3f} USD
• Monthly Estimate: ${total_hourly_cost * 24 * 30:.2f} USD
• Cost by Provider: {', '.join(f'{k}: ${v:.3f}/h' for k, v in provider_costs.items())}

🌏 MULTI-CLOUD DISTRIBUTION:
• Providers: {', '.join(csp_distribution.keys())}
• Regions: {', '.join(region_distribution.keys())}

⚡ DEPLOYMENT INFO:
• Estimated Time: {summary['DEPLOYMENT_STRATEGY']['estimated_total_time']}
• Complexity: {deployment_complexity.upper()}
• Mode: {'HOLD FOR REVIEW' if hold else 'IMMEDIATE DEPLOYMENT'}

❗ IMPORTANT NOTES:
{chr(10).join(f'• {risk}' for risk in risks) if risks else '• No significant risks identified'}

✅ Ready to proceed with MCI creation!
"""
        
        next_steps = [
            "✅ Approve: Call create_mci_dynamic() with skip_confirmation=True to proceed",
            "📝 Review: Use hold=True to create but hold for manual review",
            "✏️ Modify: Adjust vm_configurations if needed and re-run this summary"
        ]
        
        summary["USER_CONFIRMATION"]["ready_to_proceed"] = True
        
    else:
        confirmation_msg = f"""
⚠️ MCI CONFIGURATION HAS ISSUES

❌ DEPLOYMENT BLOCKED:
• Found {validation_issues} configuration issue(s)
• Manual review and fixes required before deployment

🔧 REQUIRED ACTIONS:
• Review and fix configuration issues
• Validate spec-image compatibility
• Re-run this summary after fixes
"""
        
        next_steps = [
            "🔧 Fix: Address configuration issues first",
            "🔍 Validate: Use validate_vm_spec_image_compatibility() for detailed analysis",
            "🔄 Retry: Re-run this summary after fixes"
        ]
        
        summary["USER_CONFIRMATION"]["ready_to_proceed"] = False
    
    summary["USER_CONFIRMATION"]["confirmation_message"] = confirmation_msg.strip()
    summary["USER_CONFIRMATION"]["next_steps"] = next_steps
    
    return summary
    
    # Multi-cloud distribution
    csp_dist = preview.get("csp_distribution", {})
    summary["MULTI_CLOUD_DISTRIBUTION"] = {
        "deployment_type": csp_dist.get("deployment_type", "unknown"),
        "total_csps": csp_dist.get("total_csps", 0),
        "csp_breakdown": csp_dist.get("csp_breakdown", {}),
        "regions": list(csp_dist.get("region_breakdown", {}).keys())
    }
    
    # Configuration status
    validation = preview.get("validation_status", {})
    resource_summary = preview.get("resource_summary", {})
    
    summary["CONFIGURATION_STATUS"] = {
        "overall_status": validation.get("overall", "unknown"),
        "auto_mapped_images": resource_summary.get("auto_mapped_images", 0),
        "manual_images": resource_summary.get("manual_images", 0),
        "validation_issues": resource_summary.get("validation_issues", 0),
    }
    
    # Resource estimate
    total_vm_instances = sum(int(vm.get("subGroupSize", 1)) for vm in vm_configurations)
    summary["RESOURCE_ESTIMATE"] = {
        "total_vm_instances": total_vm_instances,
        "unique_configurations": len(vm_configurations),
        "multi_cloud_deployment": csp_dist.get("total_csps", 0) > 1,
        "estimated_deployment_time": f"{2 + len(vm_configurations)}~{5 + len(vm_configurations) * 2} minutes"
    }
    
    # Important notes from recommendations
    important_notes = []
    for rec in preview.get("recommendations", []):
        if rec.get("priority") in ["critical", "warning"]:
            important_notes.append(f"WARNING: {rec.get('message', '')}")
        elif rec.get("priority") == "info" and "multi-cloud" in rec.get("message", "").lower():
            important_notes.append(f"INFO: {rec.get('message', '')}")
        elif rec.get("priority") == "success":
            important_notes.append(f"SUCCESS: {rec.get('message', '')}")
    
    summary["IMPORTANT_NOTES"] = important_notes
    
    # User confirmation
    ready_to_proceed = preview.get("ready_for_deployment", False)
    
    if ready_to_proceed:
        confirmation_msg = f"""
READY TO CREATE MCI '{name}'

Your multi-cloud infrastructure is configured and ready for deployment:
- {len(vm_configurations)} VM configuration(s) across {csp_dist.get('total_csps', 0)} cloud provider(s)
- {total_vm_instances} total VM instance(s) will be created
- Deployment mode: {'Review first (hold=True)' if hold else 'Immediate deployment'}

Do you want to proceed with MCI creation?
"""
        next_steps = [
            "Confirm: Proceed with create_mci_dynamic()",
            "Review: Use hold=True to review before deployment",
            "Modify: Adjust VM configurations if needed"
        ]
    else:
        validation_issues = resource_summary.get("validation_issues", 0)
        confirmation_msg = f"""
MCI CONFIGURATION HAS ISSUES

Cannot proceed with deployment due to {validation_issues} configuration issue(s):
Please review and fix the issues identified in the validation report.
"""
        next_steps = [
            "Fix configuration issues first",
            "Use validate_vm_spec_image_compatibility() for detailed analysis",
            "Re-run this summary after fixes"
        ]
    
    summary["USER_CONFIRMATION"] = {
        "ready_to_proceed": ready_to_proceed,
        "confirmation_message": confirmation_msg.strip(),
        "next_steps": next_steps
    }
    
    return summary

#####################################
# Spec-to-Image Mapping Validation Tools
#####################################

# Tool: Validate VM configuration spec-image compatibility
@mcp.tool()
def validate_vm_spec_image_compatibility(vm_configurations: List[Dict]) -> Dict:
    """
    Validate that VM configurations have proper spec-to-image mapping.
    This tool helps identify potential compatibility issues before MCI creation.
    
    **VALIDATION CHECKS:**
    - Spec format validation (provider+region+spec_name)
    - Image format validation for each CSP
    - Cross-reference CSP in spec vs image identifier
    - Region compatibility checks where possible
    
    Args:
        vm_configurations: List of VM configurations to validate
    
    Returns:
        Validation results with detailed compatibility analysis
    """
    validation_result = {
        "overall_status": "checking",
        "total_configurations": len(vm_configurations),
        "valid_configurations": 0,
        "validation_details": [],
        "recommendations": []
    }
    
    csp_image_patterns = {
        "aws": {"required_patterns": ["ami-"], "forbidden_patterns": ["microsoft", "/subscriptions/", "projects/"]},
        "azure": {"required_patterns": ["microsoft", "/subscriptions/"], "forbidden_patterns": ["ami-", "projects/"]},
        "gcp": {"required_patterns": ["projects/", "google"], "forbidden_patterns": ["ami-", "/subscriptions/"]},
        "alibaba": {"required_patterns": ["m-"], "forbidden_patterns": ["ami-", "/subscriptions/"]},
        "tencent": {"required_patterns": ["img-"], "forbidden_patterns": ["ami-", "/subscriptions/"]}
    }
    
    for i, vm_config in enumerate(vm_configurations):
        config_validation = {
            "vm_index": i,
            "vm_name": vm_config.get("name", f"vm-{i+1}"),
            "status": "valid",
            "issues": [],
            "warnings": [],
            "spec_analysis": {},
            "image_analysis": {}
        }
        
        # Validate specId
        common_spec = vm_config.get("specId")
        if not common_spec:
            config_validation["status"] = "invalid"
            config_validation["issues"].append("Missing specId")
        else:
            try:
                spec_parts = common_spec.split("+")
                if len(spec_parts) < 3:
                    config_validation["status"] = "invalid"
                    config_validation["issues"].append(f"Invalid spec format: {common_spec}")
                else:
                    provider = spec_parts[0].lower()
                    region = spec_parts[1]
                    spec_name = spec_parts[2]
                    
                    config_validation["spec_analysis"] = {
                        "provider": provider,
                        "region": region,
                        "spec_name": spec_name,
                        "format": "valid"
                    }
                    
                    # Validate imageId compatibility
                    common_image = vm_config.get("imageId")
                    if not common_image:
                        config_validation["warnings"].append("imageId not specified - will be auto-mapped")
                    else:
                        image_lower = common_image.lower()
                        image_valid = False
                        
                        if provider in csp_image_patterns:
                            patterns = csp_image_patterns[provider]
                            
                            # Check required patterns
                            has_required = any(pattern in image_lower for pattern in patterns["required_patterns"])
                            has_forbidden = any(pattern in image_lower for pattern in patterns["forbidden_patterns"])
                            
                            if has_required and not has_forbidden:
                                image_valid = True
                                config_validation["image_analysis"] = {
                                    "compatibility": "valid",
                                    "provider_match": True,
                                    "image_identifier": common_image
                                }
                            elif has_forbidden:
                                config_validation["status"] = "invalid"
                                config_validation["issues"].append(
                                    f"Image {common_image} appears to be for different CSP than spec {provider}"
                                )
                                config_validation["image_analysis"] = {
                                    "compatibility": "invalid",
                                    "provider_match": False,
                                    "detected_issue": "Cross-CSP image reference"
                                }
                            elif not has_required:
                                config_validation["warnings"].append(
                                    f"Image {common_image} doesn't match expected {provider} patterns"
                                )
                                config_validation["image_analysis"] = {
                                    "compatibility": "warning",
                                    "provider_match": "uncertain",
                                    "suggestion": f"Expected patterns for {provider}: {patterns['required_patterns']}"
                                }
                        else:
                            config_validation["warnings"].append(f"Unknown provider {provider} - cannot validate image pattern")
                            
            except Exception as e:
                config_validation["status"] = "invalid"
                config_validation["issues"].append(f"Spec parsing error: {str(e)}")
        
        validation_result["validation_details"].append(config_validation)
        
        if config_validation["status"] == "valid":
            validation_result["valid_configurations"] += 1
    
    # Overall status determination
    if validation_result["valid_configurations"] == validation_result["total_configurations"]:
        validation_result["overall_status"] = "all_valid"
    elif validation_result["valid_configurations"] > 0:
        validation_result["overall_status"] = "partially_valid"
    else:
        validation_result["overall_status"] = "all_invalid"
    
    # Generate recommendations
    if validation_result["overall_status"] != "all_valid":
        validation_result["recommendations"] = [
            "Use auto-mapping by omitting imageId in VM configurations",
            "Use create_mci_dynamic() which automatically handles spec-to-image mapping",
            "For manual mapping, ensure image identifiers match the CSP in specId",
            "AWS: use AMI IDs (ami-xxxxxx), Azure: use Microsoft images or subscription paths, GCP: use project paths"
        ]
    
    return validation_result

# Tool: Get spec-image mapping examples  
@mcp.tool()
def get_spec_image_mapping_examples() -> Dict:
    """
    Get examples of correct and incorrect spec-to-image mappings.
    This helps understand the importance of proper CSP-specific image selection.
    
    Returns:
        Examples showing correct and incorrect mappings with explanations
    """
    return {
        "correct_examples": {
            "aws_example": {
                "specId": "aws+ap-northeast-2+t2.small",
                "imageId": "ami-0e06732ba3ca8c6cc",
                "explanation": "AWS spec with AWS AMI ID - CORRECT",
                "why_correct": "AMI ID (ami-*) is AWS-specific image format"
            },
            "azure_example": {
                "specId": "azure+koreacentral+Standard_B2s",
                "imageId": "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.Compute/images/ubuntu-20.04",
                "explanation": "Azure spec with Azure image path - CORRECT",
                "why_correct": "Subscription-based path is Azure-specific format"
            },
            "gcp_example": {
                "specId": "gcp+asia-northeast3+e2-medium",
                "imageId": "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20240830",
                "explanation": "GCP spec with GCP image path - CORRECT", 
                "why_correct": "Project-based path is GCP-specific format"
            },
            "auto_mapping_example": {
                "specId": "aws+us-east-1+t3.medium",
                "imageId": "AUTO-MAPPED",
                "explanation": "Spec without image - will be auto-mapped - RECOMMENDED",
                "why_recommended": "Automatic mapping ensures correct CSP-specific image selection"
            }
        },
        "incorrect_examples": {
            "cross_csp_error": {
                "specId": "aws+us-east-1+t2.small",
                "imageId": "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.Compute/images/ubuntu",
                "explanation": "AWS spec with Azure image - WRONG",
                "why_wrong": "Cannot use Azure image path with AWS specifications",
                "fix": "Use AMI ID for AWS or let system auto-map"
            },
            "format_mismatch": {
                "specId": "azure+eastus+Standard_B1s", 
                "imageId": "ami-0123456789abcdef0",
                "explanation": "Azure spec with AWS AMI - WRONG",
                "why_wrong": "AMI IDs only work with AWS, not Azure",
                "fix": "Use Azure image identifier or enable auto-mapping"
            },
            "region_mismatch": {
                "specId": "aws+us-west-2+t2.nano",
                "imageId": "ami-0a1b2c3d4e5f6789a",  # Hypothetical wrong region AMI
                "explanation": "Potentially wrong region AMI - RISKY",
                "why_risky": "AMI IDs are region-specific, using wrong region AMI will fail",
                "fix": "Search for images in the spec's region (us-west-2)"
            }
        },
        "best_practices": {
            "recommendation_1": {
                "title": "Use Auto-Mapping",
                "description": "Omit imageId to let create_mci_dynamic() automatically select correct images",
                "example": {"specId": "aws+ap-northeast-2+t2.small"}
            },
            "recommendation_2": {
                "title": "Spec-First Workflow",
                "description": "Choose spec first, then search images in that spec's CSP/region",
                "workflow": [
                    "1. recommend_vm_spec() -> get spec ID",
                    "2. Extract CSP/region from spec ID", 
                    "3. search_images(provider=csp, region=region)",
                    "4. Use cspImageName from search results"
                ]
            },
            "recommendation_3": {
                "title": "Use Validation Tools", 
                "description": "Validate configurations before deployment",
                "tools": ["validate_vm_spec_image_compatibility()", "create_mci_with_proper_spec_mapping()"]
            }
        },
        "common_mistakes": [
            "Using same image for different CSPs (AWS AMI with Azure spec)",
            "Hard-coding image IDs without considering regions",
            "Not validating spec-image compatibility before deployment",
            "Mixing public and private image references",
            "Using outdated or deprecated image IDs"
        ]
    }

# Note: create_mci_with_confirmation has been removed.
# Use create_mci_dynamic with force_create parameter for confirmation workflow.

#####################################
# Application Deployment & UseCase Management System
#####################################

# Tool: Research application hardware requirements from internet
@mcp.tool()
def research_application_requirements(
    application_name: str,
    version: Optional[str] = None,
    deployment_type: str = "production"
) -> Dict:
    """
    Research application hardware requirements from internet sources.
    This tool searches for official documentation and community recommendations
    to determine optimal hardware specifications for application deployment.
    
    Args:
        application_name: Name of the application to research
        version: Specific version if needed (optional)
        deployment_type: Type of deployment ("production", "development", "testing")
    
    Returns:
        Hardware requirements research results with recommendations
    """
    try:
        # Construct search queries for hardware requirements
        base_queries = [
            f"{application_name} system requirements hardware specifications",
            f"{application_name} minimum requirements CPU memory disk",
            f"{application_name} server requirements production deployment",
            f"{application_name} recommended hardware specs"
        ]
        
        if version:
            base_queries.append(f"{application_name} {version} system requirements")
        
        # Add deployment type specific queries
        if deployment_type == "production":
            base_queries.append(f"{application_name} production server requirements")
        elif deployment_type == "development":
            base_queries.append(f"{application_name} development environment requirements")
        
        research_results = {
            "application_name": application_name,
            "version": version,
            "deployment_type": deployment_type,
            "search_queries": base_queries,
            "requirements_found": {},
            "recommendations": {},
            "sources": [],
            "status": "success"
        }
        
        # Search for requirements from multiple sources
        all_search_results = []
        
        # Since web search is not available, use built-in knowledge base
        # This will be enhanced when web search tools are available
        logger.info(f"Researching requirements for {application_name} using built-in knowledge")
        
        # Try to extract requirements from application name patterns
        extracted_requirements = _analyze_application_by_name(application_name, deployment_type)
        research_results["requirements_found"] = extracted_requirements
        
        # Generate recommendations based on findings
        recommendations = _generate_hardware_recommendations(extracted_requirements, deployment_type)
        research_results["recommendations"] = recommendations
        
        # Add sources information
        research_results["sources"] = [result["query"] for result in all_search_results]
        research_results["total_sources_checked"] = len(all_search_results)
        
        return research_results
        
    except Exception as e:
        # If internet search fails, return fallback recommendations
        return _get_fallback_hardware_requirements(application_name, deployment_type, str(e))

# Helper function: Analyze application by name when web search is not available
def _analyze_application_by_name(app_name: str, deployment_type: str) -> Dict:
    """Analyze application requirements based on name patterns and built-in knowledge."""
    
    app_lower = app_name.lower()
    
    # Built-in knowledge base for common applications
    knowledge_base = {
        # Web servers
        "nginx": {"cpu": 2, "memory": 2, "disk": 20, "category": "web"},
        "apache": {"cpu": 2, "memory": 2, "disk": 20, "category": "web"},
        "httpd": {"cpu": 2, "memory": 2, "disk": 20, "category": "web"},
        
        # Databases
        "mysql": {"cpu": 2, "memory": 4, "disk": 100, "category": "database"},
        "postgresql": {"cpu": 2, "memory": 4, "disk": 100, "category": "database"},
        "postgres": {"cpu": 2, "memory": 4, "disk": 100, "category": "database"},
        "mongodb": {"cpu": 2, "memory": 4, "disk": 100, "category": "database"},
        "redis": {"cpu": 2, "memory": 4, "disk": 50, "category": "cache"},
        "memcached": {"cpu": 1, "memory": 2, "disk": 20, "category": "cache"},
        
        # Search and Analytics
        "elasticsearch": {"cpu": 4, "memory": 8, "disk": 200, "category": "search"},
        "kibana": {"cpu": 2, "memory": 4, "disk": 50, "category": "visualization"},
        "logstash": {"cpu": 2, "memory": 4, "disk": 50, "category": "processing"},
        "elk": {"cpu": 4, "memory": 8, "disk": 200, "category": "stack"},
        
        # Container platforms
        "docker": {"cpu": 2, "memory": 4, "disk": 100, "category": "container"},
        "kubernetes": {"cpu": 4, "memory": 8, "disk": 100, "category": "orchestration"},
        "k8s": {"cpu": 4, "memory": 8, "disk": 100, "category": "orchestration"},
        
        # CI/CD
        "jenkins": {"cpu": 2, "memory": 4, "disk": 100, "category": "ci_cd"},
        "gitlab": {"cpu": 4, "memory": 8, "disk": 200, "category": "ci_cd"},
        "github": {"cpu": 2, "memory": 4, "disk": 100, "category": "ci_cd"},
        
        # Games
        "xonotic": {"cpu": 2, "memory": 2, "disk": 30, "category": "game"},
        "minecraft": {"cpu": 2, "memory": 4, "disk": 50, "category": "game"},
        "csgo": {"cpu": 4, "memory": 4, "disk": 50, "category": "game"},
        "tf2": {"cpu": 2, "memory": 4, "disk": 40, "category": "game"},
        
        # AI/ML
        "ollama": {"cpu": 4, "memory": 8, "disk": 100, "category": "ai"},
        "tensorflow": {"cpu": 4, "memory": 8, "disk": 100, "category": "ai"},
        "pytorch": {"cpu": 4, "memory": 8, "disk": 100, "category": "ai"},
        "jupyter": {"cpu": 2, "memory": 4, "disk": 50, "category": "ai"},
        
        # Communication
        "jitsi": {"cpu": 4, "memory": 8, "disk": 50, "category": "communication"},
        "zoom": {"cpu": 4, "memory": 8, "disk": 50, "category": "communication"},
        "slack": {"cpu": 2, "memory": 4, "disk": 50, "category": "communication"},
        
        # Distributed computing
        "ray": {"cpu": 4, "memory": 8, "disk": 100, "category": "distributed"},
        "spark": {"cpu": 4, "memory": 8, "disk": 100, "category": "distributed"},
        "hadoop": {"cpu": 4, "memory": 8, "disk": 200, "category": "distributed"},
        
        # Monitoring
        "prometheus": {"cpu": 2, "memory": 4, "disk": 100, "category": "monitoring"},
        "grafana": {"cpu": 2, "memory": 4, "disk": 50, "category": "monitoring"},
        "netdata": {"cpu": 1, "memory": 2, "disk": 20, "category": "monitoring"}
    }
    
    # Find matching application
    matched_app = None
    for app_key, specs in knowledge_base.items():
        if app_key in app_lower or app_lower in app_key:
            matched_app = specs
            break
    
    # Default specs if no match found
    if not matched_app:
        matched_app = {"cpu": 2, "memory": 4, "disk": 50, "category": "general"}
    
    # Apply deployment type multipliers
    multipliers = {"production": 1.5, "development": 1.0, "testing": 0.8}
    multiplier = multipliers.get(deployment_type, 1.0)
    
    requirements = {
        "cpu_cores": {
            "min": matched_app["cpu"],
            "recommended": max(2, int(matched_app["cpu"] * multiplier))
        },
        "memory_gb": {
            "min": matched_app["memory"],
            "recommended": max(4, int(matched_app["memory"] * multiplier))
        },
        "disk_gb": {
            "min": max(50, matched_app["disk"]),  # Minimum 50GB as requested
            "recommended": max(50, int(matched_app["disk"] * multiplier))
        },
        "additional_requirements": [
            f"Application category: {matched_app['category']}",
            f"Deployment type: {deployment_type}",
            f"Multiplier applied: {multiplier}"
        ],
        "confidence": "medium"  # Built-in knowledge is medium confidence
    }
    
    return requirements

# Helper function: Analyze hardware requirements from search results
def _analyze_hardware_requirements(search_results: List[Dict], app_name: str) -> Dict:
    """Extract hardware requirements from search results using pattern matching."""
    
    requirements = {
        "cpu_cores": {"min": None, "recommended": None},
        "memory_gb": {"min": None, "recommended": None},
        "disk_gb": {"min": None, "recommended": None},
        "additional_requirements": [],
        "confidence": "low"
    }
    
    # Common patterns for extracting requirements
    cpu_patterns = [
        r'(\d+)[\s\-]*core[s]?',
        r'(\d+)[\s\-]*cpu[s]?',
        r'(\d+)[\s\-]*processor[s]?',
        r'cpu:?\s*(\d+)',
        r'minimum.*?(\d+).*?core',
        r'recommended.*?(\d+).*?core'
    ]
    
    memory_patterns = [
        r'(\d+)[\s\-]*gb[\s\-]*ram',
        r'(\d+)[\s\-]*gb[\s\-]*memory',
        r'memory:?\s*(\d+)[\s\-]*gb',
        r'ram:?\s*(\d+)[\s\-]*gb',
        r'minimum.*?(\d+).*?gb.*?ram',
        r'recommended.*?(\d+).*?gb.*?ram'
    ]
    
    disk_patterns = [
        r'(\d+)[\s\-]*gb[\s\-]*disk',
        r'(\d+)[\s\-]*gb[\s\-]*storage',
        r'storage:?\s*(\d+)[\s\-]*gb',
        r'disk:?\s*(\d+)[\s\-]*gb',
        r'minimum.*?(\d+).*?gb.*?disk',
        r'(\d+)[\s\-]*gb[\s\-]*free[\s\-]*space'
    ]
    
    # Extract values from all search results
    cpu_values = []
    memory_values = []
    disk_values = []
    
    for result in search_results:
        content = result.get("content", "").lower()
        
        # Extract CPU values
        for pattern in cpu_patterns:
            matches = re.findall(pattern, content, re.IGNORECASE)
            cpu_values.extend([int(m) for m in matches if m.isdigit() and 1 <= int(m) <= 128])
        
        # Extract memory values
        for pattern in memory_patterns:
            matches = re.findall(pattern, content, re.IGNORECASE)
            memory_values.extend([int(m) for m in matches if m.isdigit() and 1 <= int(m) <= 512])
        
        # Extract disk values
        for pattern in disk_patterns:
            matches = re.findall(pattern, content, re.IGNORECASE)
            disk_values.extend([int(m) for m in matches if m.isdigit() and 1 <= int(m) <= 10000])
    
    # Process extracted values
    if cpu_values:
        requirements["cpu_cores"]["min"] = min(cpu_values)
        requirements["cpu_cores"]["recommended"] = max(cpu_values) if len(cpu_values) > 1 else min(cpu_values) * 2
        requirements["confidence"] = "high" if len(cpu_values) >= 2 else "medium"
    
    if memory_values:
        requirements["memory_gb"]["min"] = min(memory_values)
        requirements["memory_gb"]["recommended"] = max(memory_values) if len(memory_values) > 1 else min(memory_values) * 2
        if requirements["confidence"] != "high":
            requirements["confidence"] = "high" if len(memory_values) >= 2 else "medium"
    
    if disk_values:
        requirements["disk_gb"]["min"] = min(disk_values)
        requirements["disk_gb"]["recommended"] = max(disk_values) if len(disk_values) > 1 else min(disk_values)
        if requirements["confidence"] == "low":
            requirements["confidence"] = "medium"
    
    return requirements

# Helper function: Generate hardware recommendations
def _generate_hardware_recommendations(requirements: Dict, deployment_type: str) -> Dict:
    """Generate final hardware recommendations based on research results."""
    
    recommendations = {
        "cpu_cores": 2,  # Default
        "memory_gb": 4,  # Default
        "disk_gb": 50,   # Minimum default
        "deployment_multiplier": 1.0,
        "reasoning": []
    }
    
    # Apply deployment type multipliers
    multipliers = {
        "production": 1.5,
        "development": 1.0,
        "testing": 0.8
    }
    
    multiplier = multipliers.get(deployment_type, 1.0)
    recommendations["deployment_multiplier"] = multiplier
    
    # Process CPU recommendations
    if requirements["cpu_cores"]["recommended"]:
        recommendations["cpu_cores"] = max(2, int(requirements["cpu_cores"]["recommended"] * multiplier))
        recommendations["reasoning"].append(f"CPU: Based on research, using {requirements['cpu_cores']['recommended']} cores with {deployment_type} multiplier")
    elif requirements["cpu_cores"]["min"]:
        recommendations["cpu_cores"] = max(2, int(requirements["cpu_cores"]["min"] * multiplier * 1.5))
        recommendations["reasoning"].append(f"CPU: Based on minimum requirement {requirements['cpu_cores']['min']} cores, increased for {deployment_type}")
    else:
        recommendations["reasoning"].append("CPU: Using default 2 cores (no specific requirements found)")
    
    # Process Memory recommendations
    if requirements["memory_gb"]["recommended"]:
        recommendations["memory_gb"] = max(4, int(requirements["memory_gb"]["recommended"] * multiplier))
        recommendations["reasoning"].append(f"Memory: Based on research, using {requirements['memory_gb']['recommended']}GB with {deployment_type} multiplier")
    elif requirements["memory_gb"]["min"]:
        recommendations["memory_gb"] = max(4, int(requirements["memory_gb"]["min"] * multiplier * 1.5))
        recommendations["reasoning"].append(f"Memory: Based on minimum requirement {requirements['memory_gb']['min']}GB, increased for {deployment_type}")
    else:
        recommendations["reasoning"].append("Memory: Using default 4GB (no specific requirements found)")
    
    # Process Disk recommendations (minimum 50GB as requested)
    if requirements["disk_gb"]["recommended"]:
        recommendations["disk_gb"] = max(50, int(requirements["disk_gb"]["recommended"] * multiplier))
        recommendations["reasoning"].append(f"Disk: Based on research, using {requirements['disk_gb']['recommended']}GB with {deployment_type} multiplier")
    elif requirements["disk_gb"]["min"]:
        recommendations["disk_gb"] = max(50, int(requirements["disk_gb"]["min"] * multiplier * 1.5))
        recommendations["reasoning"].append(f"Disk: Based on minimum requirement {requirements['disk_gb']['min']}GB, increased for {deployment_type}")
    else:
        recommendations["disk_gb"] = 50
        recommendations["reasoning"].append("Disk: Using minimum 50GB (no specific requirements found)")
    
    return recommendations

# Helper function: Fallback hardware requirements
def _get_fallback_hardware_requirements(app_name: str, deployment_type: str, error_msg: str) -> Dict:
    """Provide fallback recommendations when internet search fails."""
    
    # Application-specific fallbacks based on common patterns
    app_fallbacks = {
        "nginx": {"cpu": 2, "memory": 2, "disk": 20},
        "apache": {"cpu": 2, "memory": 2, "disk": 20},
        "mysql": {"cpu": 2, "memory": 4, "disk": 100},
        "postgresql": {"cpu": 2, "memory": 4, "disk": 100},
        "mongodb": {"cpu": 2, "memory": 4, "disk": 100},
        "redis": {"cpu": 2, "memory": 4, "disk": 50},
        "elasticsearch": {"cpu": 4, "memory": 8, "disk": 200},
        "kibana": {"cpu": 2, "memory": 4, "disk": 50},
        "logstash": {"cpu": 2, "memory": 4, "disk": 50},
        "docker": {"cpu": 2, "memory": 4, "disk": 100},
        "kubernetes": {"cpu": 4, "memory": 8, "disk": 100},
        "jenkins": {"cpu": 2, "memory": 4, "disk": 100},
        "gitlab": {"cpu": 4, "memory": 8, "disk": 200},
        "xonotic": {"cpu": 2, "memory": 2, "disk": 30},
        "minecraft": {"cpu": 2, "memory": 4, "disk": 50},
        "ollama": {"cpu": 4, "memory": 8, "disk": 100},
        "jitsi": {"cpu": 4, "memory": 8, "disk": 50},
        "ray": {"cpu": 4, "memory": 8, "disk": 100},
        # New MapUI applications
        "vllm": {"cpu": 4, "memory": 8, "disk": 100},
        "nvidia": {"cpu": 2, "memory": 4, "disk": 50},
        "openwebui": {"cpu": 2, "memory": 4, "disk": 100},
        "westward": {"cpu": 2, "memory": 4, "disk": 50},
        "weavescope": {"cpu": 2, "memory": 4, "disk": 50},
        "elk": {"cpu": 4, "memory": 8, "disk": 200},
        "cross_nat": {"cpu": 1, "memory": 2, "disk": 30},
        "stress": {"cpu": 2, "memory": 2, "disk": 30}
    }
    
    # Try to find application in fallbacks
    app_lower = app_name.lower()
    fallback = None
    
    for app_key, specs in app_fallbacks.items():
        if app_key in app_lower or app_lower in app_key:
            fallback = specs
            break
    
    # Default fallback if no specific match
    if not fallback:
        fallback = {"cpu": 2, "memory": 4, "disk": 50}
    
    # Apply deployment type multiplier
    multipliers = {"production": 1.5, "development": 1.0, "testing": 0.8}
    multiplier = multipliers.get(deployment_type, 1.0)
    
    recommendations = {
        "cpu_cores": max(2, int(fallback["cpu"] * multiplier)),
        "memory_gb": max(4, int(fallback["memory"] * multiplier)),
        "disk_gb": max(50, int(fallback["disk"] * multiplier)),  # Minimum 50GB as requested
        "deployment_multiplier": multiplier,
        "reasoning": [
            f"Internet search failed: {error_msg}",
            f"Using fallback recommendations for {app_name}",
            f"Applied {deployment_type} deployment multiplier: {multiplier}"
        ]
    }
    
    return {
        "application_name": app_name,
        "deployment_type": deployment_type,
        "search_queries": ["fallback_used"],
        "requirements_found": {
            "cpu_cores": {"min": fallback["cpu"], "recommended": fallback["cpu"]},
            "memory_gb": {"min": fallback["memory"], "recommended": fallback["memory"]},
            "disk_gb": {"min": fallback["disk"], "recommended": fallback["disk"]},
            "confidence": "fallback"
        },
        "recommendations": recommendations,
        "sources": ["fallback_database"],
        "status": "fallback_used",
        "error": error_msg
    }

# Predefined application configurations based on MapUI patterns
APPLICATION_CONFIGS = {
    "xonotic": {
        "name": "Xonotic Game Server",
        "category": "game",
        "description": "FPS game server deployment with automatic server configuration using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "network_intensive": True,
            "ports": ["26000", "8"],
            "os": "ubuntu",
            "min_disk_gb": 30  # Game-specific disk requirement
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/xonotic/startServer.sh; chmod +x ~/startServer.sh",
            "sudo ~/startServer.sh Cloud-Barista-$$Func(GetMciId()) 26000 8 8",
            "echo '$$Func(GetPublicIP(target=this,postfix=:26000))'"
        ],
        "result_pattern": r"Server Address: ([^:]+):(\d+)",
        "deployment_strategy": "global"
    },
    "nginx": {
        "name": "Nginx Web Server",
        "category": "web",
        "description": "High-performance web server deployment using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": False,
            "network_intensive": True,
            "ports": ["80", "443"],
            "os": "ubuntu",
            "min_disk_gb": 20
        },
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/nginx/startServer.sh | bash -s -- --ip $$Func(GetPublicIP(target=this))",
            "echo '$$Func(GetPublicIP(target=this, prefix=http://))'"
        ],
        "result_pattern": r"Web Server: (http://[^/]+)",
        "deployment_strategy": "regional"
    },
    "ollama": {
        "name": "Ollama LLM Server",
        "category": "llm",
        "description": "Local LLM server deployment with Ollama using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "gpu_preferred": True,
            "ports": ["3000"],
            "os": "ubuntu",
            "min_disk_gb": 300  # LLM models require significant storage
        },
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh",
            "echo '$$Func(GetPublicIP(target=this, prefix=http://, postfix=:3000))'"
        ],
        "result_pattern": r"LLM Server: (http://[^/]+)",
        "deployment_strategy": "performance"
    },
    "jitsi": {
        "name": "Jitsi Meet Video Conference",
        "category": "conference",
        "description": "Video conferencing server deployment using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "network_intensive": True,
            "bandwidth_intensive": True,
            "ports": ["443", "80", "10000"],
            "os": "ubuntu",
            "min_disk_gb": 50
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/jitsi/startServer.sh",
            "chmod +x ~/startServer.sh",
            "sudo ~/startServer.sh $$Func(GetPublicIPs(separator=' ')) DNS EMAIL"
        ],
        "result_pattern": r"Jitsi Server: (https://[^/]+)",
        "deployment_strategy": "regional"
    },
    "elk": {
        "name": "ELK Stack",
        "category": "observability",
        "description": "Elasticsearch, Logstash, and Kibana stack deployment",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "storage_intensive": True,
            "ports": ["9200", "5601", "5044"],
            "os": "ubuntu",
            "min_disk_gb": 200  # ELK requires significant storage for logs
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/elastic-stack/startELK.sh",
            "chmod +x ~/startELK.sh",
            "sudo ~/startELK.sh"
        ],
        "result_pattern": r"Kibana: (http://[^:]+:5601)",
        "deployment_strategy": "centralized"
    },
    "ray": {
        "name": "Ray ML Cluster",
        "category": "ml",
        "description": "Distributed ML computing cluster with Ray using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "network_intensive": True,
            "cluster": True,
            "ports": ["8265", "10001"],
            "os": "ubuntu",
            "min_disk_gb": 100
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/ray/ray-head-setup.sh",
            "chmod +x ~/ray-head-setup.sh",
            "~/ray-head-setup.sh -i $$Func(GetPublicIP(target=this))"
        ],
        "result_pattern": r"Ray Dashboard: (http://[^:]+:8265)",
        "deployment_strategy": "cluster"
    },
    "vllm": {
        "name": "vLLM Server",
        "category": "llm",
        "description": "High-performance LLM inference server with vLLM using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "gpu_preferred": True,
            "ports": ["5000"],
            "os": "ubuntu",
            "min_disk_gb": 100
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/llmServer.py",
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/startServer.sh; chmod +x ~/startServer.sh",
            "~/startServer.sh --ip $$Func(GetPublicIPs(separator=' ')) --port 5000 --token 1024 --model tiiuae/falcon-7b-instruct"
        ],
        "result_pattern": r"vLLM Server: (http://[^:]+:5000)",
        "deployment_strategy": "performance"
    },
    "nvidia_driver": {
        "name": "NVIDIA CUDA Driver",
        "category": "driver",
        "description": "NVIDIA CUDA driver installation for GPU computing",
        "requirements": {
            "gpu_required": True,
            "cpu_intensive": False,
            "memory_intensive": False,
            "ports": [],
            "os": "ubuntu",
            "min_disk_gb": 50
        },
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/installCudaDriver.sh | sh",
            "nvidia-smi",
            "echo 'NVIDIA Driver installed successfully'"
        ],
        "result_pattern": r"NVIDIA Driver installed successfully",
        "deployment_strategy": "performance"
    },
    "ollama_pull": {
        "name": "Ollama Model Pull (Dynamic Selection)",
        "category": "llm",
        "description": "Pull LLM models for Ollama using dynamic model selection from ollama.com. Use get_ollama_model_discovery_guide() to find latest models, then deploy_ollama_pull_with_models() for custom deployment.",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "gpu_preferred": True,
            "ports": ["3000"],
            "os": "ubuntu",
            "min_disk_gb": 150
        },
        "commands": [
            "# This is a fallback command - prefer using deploy_ollama_pull_with_models() with user-selected models",
            "OLLAMA_HOST=0.0.0.0:3000 ollama pull $$Func(AssignTask(task='Ask user to visit https://ollama.com/search for latest models'))",
            "echo 'Visit https://ollama.com/search to discover and select models'",
            "echo 'Then use deploy_ollama_pull_with_models() for custom deployment'"
        ],
        "result_pattern": r"Visit https://ollama.com/search",
        "deployment_strategy": "performance",
        "recommended_workflow": "Use get_ollama_model_discovery_guide() then deploy_ollama_pull_with_models()"
    },
    "openwebui": {
        "name": "Open WebUI",
        "category": "web",
        "description": "Web interface for LLM models with Open WebUI using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "network_intensive": True,
            "ports": ["80", "3000"],
            "os": "ubuntu",
            "min_disk_gb": 100
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh; chmod +x ~/deployOpenWebUI.sh",
            "sudo ~/deployOpenWebUI.sh \"$$Func(GetPublicIPs(target=this, separator=;, prefix=http://, postfix=:3000))\"",
            "echo '$$Func(GetPublicIP(target=this, prefix=http://))'"
        ],
        "result_pattern": r"Open WebUI: (http://[^/]+)",
        "deployment_strategy": "regional"
    },
    "ray_worker": {
        "name": "Ray Worker Node",
        "category": "ml",
        "description": "Ray worker node to join existing Ray cluster using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "network_intensive": True,
            "cluster": True,
            "ports": ["10001"],
            "os": "ubuntu",
            "min_disk_gb": 100
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/ray/ray-worker-setup.sh",
            "chmod +x ~/ray-worker-setup.sh",
            "~/ray-worker-setup.sh -i $$Func(GetPublicIP(target=this)) -h $$Func(GetPublicIP(target=mc-ray.g1-1))"
        ],
        "result_pattern": r"Ray Worker connected to head node",
        "deployment_strategy": "cluster"
    },
    "westward": {
        "name": "Westward Game",
        "category": "game",
        "description": "Westward strategy game server deployment",
        "requirements": {
            "cpu_intensive": True,
            "network_intensive": True,
            "ports": ["80", "443"],
            "os": "ubuntu",
            "min_disk_gb": 50
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setgame.sh",
            "chmod +x ~/setgame.sh; sudo ~/setgame.sh",
            "echo 'Westward Game Server Ready'"
        ],
        "result_pattern": r"Westward Game Server Ready",
        "deployment_strategy": "regional"
    },
    "weavescope": {
        "name": "Weave Scope",
        "category": "monitoring",
        "description": "Container monitoring and visualization with Weave Scope using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "network_intensive": True,
            "ports": ["4040"],
            "os": "ubuntu",
            "min_disk_gb": 50
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/weavescope/startServer.sh",
            "chmod +x ~/startServer.sh",
            "sudo ~/startServer.sh $$Func(GetPublicIPs(separator=' ')) $$Func(GetPrivateIPs(separator=' '))"
        ],
        "result_pattern": r"Weave Scope: (http://[^:]+:4040)",
        "deployment_strategy": "centralized"
    },
    "cross_nat": {
        "name": "Cross-Cloud NAT Setup",
        "category": "network",
        "description": "Setup cross-cloud NAT for multi-cloud networking using CB-Tumblebug built-in functions",
        "requirements": {
            "cpu_intensive": False,
            "memory_intensive": False,
            "network_intensive": True,
            "ports": [],
            "os": "ubuntu",
            "min_disk_gb": 30
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setup-cross-cloud-nat.sh",
            "chmod +x ~/setup-cross-cloud-nat.sh",
            "~/setup-cross-cloud-nat.sh pub=$$Func(GetPublicIPs(target=this)) priv=$$Func(GetPrivateIPs(target=this))"
        ],
        "result_pattern": r"Cross-cloud NAT setup completed",
        "deployment_strategy": "global"
    },
    "stress_test": {
        "name": "System Stress Test",
        "category": "testing",
        "description": "CPU stress testing for performance validation",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": False,
            "network_intensive": False,
            "ports": [],
            "os": "ubuntu",
            "min_disk_gb": 30
        },
        "commands": [
            "sudo apt install -y stress > /dev/null; stress -c 16 -t 60",
            "echo 'Stress test completed - 16 cores for 60 seconds'",
            ""
        ],
        "result_pattern": r"Stress test completed",
        "deployment_strategy": "regional"
    }
}

# Tool: Get Ollama model discovery guide
@mcp.tool()
def get_ollama_model_discovery_guide(
    use_case: Optional[str] = None,
    category: Optional[str] = None
) -> Dict:
    """
    Provide guidance for discovering latest Ollama models dynamically.
    
    Instead of static model lists, this tool provides LLM with instructions 
    on how to find the most current model information from official sources.
    
    **CRITICAL LLM Instructions:**
    1. Visit https://ollama.com/search to find latest models
    2. Use search filters on ollama.com for specific categories
    3. Check model tags, sizes, and descriptions on model pages
    4. Verify model names before deployment
    5. Consider hardware requirements based on model size
    
    **Model Discovery Workflow:**
    1. Browse https://ollama.com/search for current models
    2. Filter by category (code, chat, embedding, vision, etc.)
    3. Check individual model pages for details and tags
    4. Note model variants (7B, 13B, 70B, etc.)
    5. Select appropriate models for deployment
    
    **Popular Categories to Search:**
    - Code: deepseek-coder, qwen-coder, codellama, starcoder
    - Chat: llama, qwen, gemma, mistral, phi
    - Reasoning: deepseek-r1, qwen-plus, o1-mini
    - Vision: llava, moondream, bakllava
    - Embedding: nomic-embed, mxbai-embed
    - Lightweight: gemma2, phi3, tinyllama
    
    Args:
        use_case: Optional use case hint (e.g., "coding", "chat", "research")
        category: Optional category filter hint
    
    Returns:
        Discovery guidance with search strategies and model selection tips
    """
    
    discovery_guide = {
        "model_discovery_instructions": {
            "primary_source": "https://ollama.com/search",
            "search_strategy": [
                "1. Open https://ollama.com/search in browser",
                "2. Use search bar for specific terms (e.g., 'code', 'chat', 'vision')",
                "3. Browse categories using filters on the left sidebar",
                "4. Click on model names to see detailed information",
                "5. Check 'Tags' tab on model pages for available versions",
                "6. Note memory requirements and model sizes"
            ],
            "verification_steps": [
                "1. Verify model names exactly as shown on ollama.com",
                "2. Check if model supports desired capabilities",
                "3. Review model size vs available hardware",
                "4. Test with 'ollama pull <model_name>' to validate"
            ]
        },
        
        "search_categories": {
            "code_development": {
                "popular_patterns": ["deepseek-coder", "qwen-coder", "codellama", "starcoder", "codegemma"],
                "search_terms": ["code", "programming", "coder", "developer"],
                "considerations": "Look for models specifically trained on code datasets"
            },
            "conversational_ai": {
                "popular_patterns": ["llama", "qwen", "gemma", "mistral", "phi"],
                "search_terms": ["chat", "instruct", "assistant"],
                "considerations": "Check if model is instruction-tuned for better chat performance"
            },
            "reasoning_tasks": {
                "popular_patterns": ["deepseek-r1", "qwen-plus", "o1", "reasoning"],
                "search_terms": ["reasoning", "think", "r1", "analysis"],
                "considerations": "Models optimized for step-by-step reasoning and problem solving"
            },
            "multimodal_vision": {
                "popular_patterns": ["llava", "moondream", "bakllava", "vision"],
                "search_terms": ["vision", "image", "multimodal", "visual"],
                "considerations": "Requires models that can process both text and images"
            },
            "embeddings": {
                "popular_patterns": ["nomic-embed", "mxbai-embed", "all-minilm"],
                "search_terms": ["embed", "embedding", "similarity"],
                "considerations": "Specialized for vector embeddings and semantic search"
            }
        },
        
        "model_selection_guidelines": {
            "size_considerations": {
                "1B-3B": "Lightweight, fast inference, limited capabilities",
                "7B-8B": "Good balance of performance and resource usage",
                "13B-14B": "Better quality, moderate resource requirements", 
                "30B+": "High quality, requires significant RAM/VRAM",
                "70B+": "Best quality, enterprise-grade hardware needed"
            },
            "hardware_matching": {
                "8GB RAM": "Up to 7B models (quantized)",
                "16GB RAM": "Up to 13B models comfortably",
                "32GB RAM": "Up to 30B models",
                "64GB+ RAM": "70B+ models possible"
            },
            "use_case_optimization": {
                "development_team": "Mix of code and chat models (e.g., deepseek-coder + qwen)",
                "chat_service": "Instruction-tuned models with good conversation flow",
                "research": "Latest reasoning models and specialized tools",
                "production": "Stable, well-tested models with consistent performance"
            }
        },
        
        "current_trends": {
            "2024_2025_popular": [
                "DeepSeek R1 series (reasoning)",
                "Qwen 2.5 series (multilingual)",
                "Llama 3.x series (general purpose)",
                "Gemma 2 series (efficient)",
                "Phi-4 series (small but capable)"
            ],
            "emerging_categories": [
                "Reasoning-optimized models",
                "Code-specific fine-tunes", 
                "Multimodal capabilities",
                "Efficiency-focused variants"
            ]
        }
    }
    
    # Customize response based on use case
    if use_case:
        use_case_lower = use_case.lower()
        if "code" in use_case_lower or "programming" in use_case_lower:
            discovery_guide["recommended_focus"] = "code_development"
            discovery_guide["specific_guidance"] = "Focus on models with 'coder' or 'code' in name, check for code-specific training data"
        elif "chat" in use_case_lower or "conversation" in use_case_lower:
            discovery_guide["recommended_focus"] = "conversational_ai"
            discovery_guide["specific_guidance"] = "Look for 'instruct' or 'chat' variants, test conversation quality"
        elif "research" in use_case_lower or "analysis" in use_case_lower:
            discovery_guide["recommended_focus"] = "reasoning_tasks"
            discovery_guide["specific_guidance"] = "Prioritize reasoning models and latest research releases"
    
    discovery_guide["dynamic_search_examples"] = [
        {
            "scenario": "Find latest code models",
            "steps": [
                "1. Go to https://ollama.com/search",
                "2. Search 'coder' or 'code'",
                "3. Sort by recent or popularity",
                "4. Check model sizes and capabilities",
                "5. Select 2-3 models for different use cases"
            ]
        },
        {
            "scenario": "Discover new reasoning models",
            "steps": [
                "1. Search 'reasoning' or 'r1' on ollama.com",
                "2. Check release dates for newest models",
                "3. Read model descriptions for reasoning capabilities",
                "4. Compare performance claims",
                "5. Test with sample reasoning tasks"
            ]
        }
    ]
    
    return discovery_guide

# Tool: Deploy Ollama with custom model selection
@mcp.tool()
def deploy_ollama_pull_with_models(
    ns_id: str,
    mci_name: str,
    selected_models: List[str],
    vm_configurations: Optional[List[Dict]] = None,
    description: str = "Ollama deployment with custom model selection"
) -> Dict:
    """
    Deploy Ollama with user-selected models from dynamic discovery.
    
    **UPDATED Workflow:**
    1. User calls get_ollama_model_discovery_guide() to learn how to find models
    2. User visits https://ollama.com/search to discover latest models
    3. User selects desired models from ollama.com
    4. This function deploys Ollama and pulls the selected models
    
    **Model Discovery Process:**
    - LLM should guide user to browse https://ollama.com/search
    - User can search by categories (code, chat, vision, etc.)
    - User selects specific model names from ollama.com
    - This tool validates and deploys the selected models
    
    **Model Distribution:**
    - If multiple VMs: Models are distributed using CB-Tumblebug's AssignTask function
    - If single VM: All models are downloaded to that VM
    
    Args:
        ns_id: Namespace ID
        mci_name: Name for the MCI
        selected_models: List of model names from ollama.com (e.g., ['llama3.3:latest', 'deepseek-r1'])
        vm_configurations: Optional VM configs (if not provided, will create optimized config)
        description: Deployment description
    
    Returns:
        Deployment result with model distribution information
    
    Example:
    ```python
    # After user discovers models from ollama.com/search
    result = deploy_ollama_pull_with_models(
        ns_id="ai-project",
        mci_name="ollama-cluster", 
        selected_models=["llama3.3:latest", "deepseek-r1:latest", "qwen2.5:14b"],
        description="Custom LLM deployment with user-discovered models"
    )
    ```
    """
    if not selected_models:
        return {
            "status": "error",
            "error": "No models selected. Please use get_ollama_model_discovery_guide() to learn how to find models on ollama.com"
        }
    
    # Validate model names (basic check)
    for model in selected_models:
        if not isinstance(model, str) or len(model.strip()) == 0:
            return {
                "status": "error", 
                "error": f"Invalid model name: {model}. Visit https://ollama.com/search to get valid model names."
            }
    
    try:
        # If no VM configurations provided, create optimized ones based on model count
        if not vm_configurations:
            # Get VM specifications for LLM workload
            specs = recommend_vm_spec(
                filter_policies={
                    "vCPU": {"min": 4, "max": 16},
                    "memoryGiB": {"min": 8, "max": 32}
                },
                priority_policy="performance",
                limit="10"
            )
            
            if not specs or not specs.get("recommended_specs"):
                return {"status": "error", "error": "Failed to get VM specifications"}
            
            # Create VM configurations based on model count
            model_count = len(selected_models)
            vm_count = min(model_count, 4)  # Max 4 VMs for distribution
            
            vm_configurations = []
            for i in range(vm_count):
                vm_configurations.append({
                    "specId": specs["recommended_specs"][i % len(specs["recommended_specs"])]["id"],
                    "name": f"ollama-vm-{i+1}",
                    "description": f"Ollama VM {i+1} for LLM models",
                    "subGroupSize": "1"
                })
        
        # Create the MCI first
        mci_result = create_mci_dynamic(
            ns_id=ns_id,
            name=mci_name,
            vm_configurations=vm_configurations,
            description=description
        )
        
        if mci_result.get("status") != "success":
            return {
                "status": "error",
                "error": "Failed to create MCI",
                "details": mci_result
            }
        
        # Prepare model pull commands with user-selected models
        models_str = ", ".join(selected_models)
        
        ollama_commands = [
            # First install Ollama
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh",
            
            # Pull selected models using AssignTask for distribution
            f"OLLAMA_HOST=0.0.0.0:3000 ollama pull $$Func(AssignTask(task='{models_str}'))",
            
            # Show access information and list installed models
            "echo '$$Func(GetPublicIP(target=this, prefix=http://, postfix=:3000))'",
            "OLLAMA_HOST=0.0.0.0:3000 ollama list"
        ]
        
        # Execute the commands
        execution_result = execute_command_mci(
            ns_id=ns_id,
            mci_id=mci_name,
            commands=ollama_commands
        )
        
        return {
            "status": "success",
            "mci_created": mci_result,
            "selected_models": selected_models,
            "model_count": len(selected_models),
            "vm_count": len(vm_configurations),
            "model_distribution": f"Models distributed across {len(vm_configurations)} VMs using AssignTask",
            "deployment_commands": ollama_commands,
            "execution_result": execution_result,
            "access_instructions": [
                "1. Wait for all models to download (may take several minutes)",
                "2. Access Ollama API at the provided HTTP endpoints", 
                "3. Use 'ollama list' to verify model installation",
                "4. Each VM may have different models based on distribution"
            ],
            "next_steps": [
                "Deploy Open WebUI for web interface",
                "Configure load balancing for multiple VMs",
                "Set up model usage monitoring",
                "Test model inference capabilities"
            ]
        }
        
    except Exception as e:
        return {
            "status": "error",
            "error": f"Deployment failed: {str(e)}",
            "selected_models": selected_models
        }

# Tool: List available application templates
@mcp.tool()
def list_application_templates() -> Dict:
    """
    List predefined applications with optimized usecase commands (Strategy A).
    
    🚨 CRITICAL: This tool identifies applications that have tested, optimized deployment scripts.
    
    **LLM Decision Logic:**
    1. Call this tool FIRST when user requests application deployment
    2. If application found in results → Use Strategy A (exact usecase commands)
    3. If application NOT found → Use Strategy B (LLM-generated commands)
    
    **Applications with Predefined Scripts:**
    - xonotic: Game server with specific port configuration
    - nginx: Web server with optimized startup script
    - ollama: LLM server with model deployment
    - jitsi: Video conference with domain configuration
    - elk: Elasticsearch, Logstash, Kibana stack
    - ray: Distributed ML computing cluster
    - vllm: High-performance LLM inference server
    - nvidia_driver: NVIDIA CUDA driver installation
    - ollama_pull: Pull specific LLM models for Ollama
    - openwebui: Web interface for LLM models
    - ray_worker: Ray worker nodes for cluster
    - westward: Westward strategy game server
    - weavescope: Container monitoring and visualization
    - cross_nat: Cross-cloud NAT networking setup
    - stress_test: CPU stress testing utility
    
    **For Predefined Applications - Use Exact Commands:**
    ✅ Use the exact wget/curl commands from APPLICATION_CONFIGS
    ✅ Follow the specific parameter patterns ({{mci_id}}, {{public_ip}}, etc.)
    ✅ DO NOT modify or recreate these scripts
    
    Returns:
        Dictionary containing predefined applications with:
        - Application categories and descriptions
        - Deployment strategies and requirements
        - Port configurations and network requirements
        - Cost estimates and location recommendations
    """
    try:
        templates = {}
        categories = {}
        
        for app_id, config in APPLICATION_CONFIGS.items():
            category = config["category"]
            if category not in categories:
                categories[category] = []
            
            categories[category].append({
                "id": app_id,
                "name": config["name"],
                "description": config["description"],
                "requirements": config["requirements"],
                "deployment_strategy": config["deployment_strategy"],
                "ports": config["requirements"].get("ports", [])
            })
            
            templates[app_id] = {
                "name": config["name"],
                "category": category,
                "description": config["description"],
                "requirements": config["requirements"],
                "deployment_strategy": config["deployment_strategy"],
                "recommended_locations": _get_recommended_locations(config["deployment_strategy"]),
                "estimated_cost_per_instance": _estimate_app_cost(config["requirements"])
            }
        
        return {
            "status": "success",
            "total_templates": len(templates),
            "categories": categories,
            "templates": templates,
            "usage_examples": [
                "deploy_application('xonotic', regions=10, description='Global game servers')",
                "deploy_application('nginx', regions=['us-east-1', 'eu-west-1'], instances_per_region=2)",
                "deploy_application('ollama', regions=3, gpu_preferred=True)"
            ]
        }
        
    except Exception as e:
        return {"status": "error", "error": str(e)}

# Tool: Get application deployment guides and commands
@mcp.tool()
def get_application_deployment_guide(
    application_name: str,
    deployment_type: str = "production"
) -> Dict:
    """
    Get REFERENCE deployment guide for applications (Reference Only for Strategy B).
    
    🚨 IMPORTANT: This provides REFERENCE INFORMATION ONLY
    
    **Usage Strategy:**
    - For APPLICATION_CONFIGS apps (xonotic, nginx, ollama, jitsi, elk, ray): IGNORE this tool completely
    - For general apps (mongodb, jenkins, redis, etc.): Use hardware specs only, generate your own commands
    
    **What to Use:**
    ✅ Hardware requirements (cpu_cores, memory_gb, disk_gb)
    ✅ VM spec filters (vCPU, memoryGiB constraints)
    ✅ Priority policy (cost, performance)
    
    **What to Ignore:**
    ❌ Installation commands (generate your own based on LLM knowledge)
    ❌ Verification commands (create appropriate ones)
    ❌ Generic deployment workflow (use the 9-step workflow instead)
    
    **Correct LLM Workflow:**
    1. Check list_application_templates() first
    2. If app in APPLICATION_CONFIGS → Use Strategy A (ignore this tool)
    3. If app NOT in APPLICATION_CONFIGS → Use Strategy B (use hardware specs only)
    
    Args:
        application_name: Name of the application to deploy
        deployment_type: Deployment environment ("production", "development", "testing")
    
    Returns:
        Reference guide with hardware requirements and generic examples:
        - Hardware requirements and VM spec recommendations (USE THESE)
        - Generic installation commands (REFERENCE ONLY - Generate your own)
        - Basic verification examples (REFERENCE ONLY - Create appropriate ones)
        - Expected ports and access patterns (USE FOR REFERENCE)
    """
    app_lower = application_name.lower()
    
    # Application-specific deployment guides
    deployment_guides = {
        "nginx": {
            "name": "Nginx Web Server",
            "category": "web",
            "description": "High-performance web server deployment",
            "hardware_requirements": {
                "cpu_cores": {"min": 1, "recommended": 2},
                "memory_gb": {"min": 2, "recommended": 4},
                "disk_gb": {"min": 20, "recommended": 50}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "cost",
            "installation_commands": [
                "sudo apt-get update -y",
                "sudo apt-get install -y nginx",
                "sudo systemctl start nginx",
                "sudo systemctl enable nginx",
                "sudo systemctl status nginx",
                "echo 'Web Server accessible at: http://{{public_ip}}'"
            ],
            "verification_commands": [
                "curl -I http://{{public_ip}}",
                "sudo systemctl is-active nginx"
            ],
            "expected_ports": ["80", "443"],
            "access_pattern": "http://{{public_ip}}"
        },
        "apache": {
            "name": "Apache Web Server",
            "category": "web",
            "description": "Apache HTTP server deployment",
            "hardware_requirements": {
                "cpu_cores": {"min": 1, "recommended": 2},
                "memory_gb": {"min": 2, "recommended": 4},
                "disk_gb": {"min": 20, "recommended": 50}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "cost",
            "installation_commands": [
                "sudo apt-get update -y",
                "sudo apt-get install -y apache2",
                "sudo systemctl start apache2",
                "sudo systemctl enable apache2",
                "sudo systemctl status apache2",
                "echo 'Apache Server accessible at: http://{{public_ip}}'"
            ],
            "verification_commands": [
                "curl -I http://{{public_ip}}",
                "sudo systemctl is-active apache2"
            ],
            "expected_ports": ["80", "443"],
            "access_pattern": "http://{{public_ip}}"
        },
        "mysql": {
            "name": "MySQL Database Server",
            "category": "database",
            "description": "MySQL database server deployment",
            "hardware_requirements": {
                "cpu_cores": {"min": 2, "recommended": 4},
                "memory_gb": {"min": 4, "recommended": 8},
                "disk_gb": {"min": 100, "recommended": 200}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "performance",
            "installation_commands": [
                "sudo apt-get update -y",
                "sudo apt-get install -y mysql-server",
                "sudo systemctl start mysql",
                "sudo systemctl enable mysql",
                "sudo mysql_secure_installation",
                "echo 'MySQL Server running on: {{public_ip}}:3306'"
            ],
            "verification_commands": [
                "sudo systemctl is-active mysql",
                "sudo mysql -e 'SELECT VERSION();'"
            ],
            "expected_ports": ["3306"],
            "access_pattern": "mysql://{{public_ip}}:3306"
        },
        "postgresql": {
            "name": "PostgreSQL Database Server",
            "category": "database",
            "description": "PostgreSQL database server deployment",
            "hardware_requirements": {
                "cpu_cores": {"min": 2, "recommended": 4},
                "memory_gb": {"min": 4, "recommended": 8},
                "disk_gb": {"min": 100, "recommended": 200}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 8}
            },
            "priority": "performance",
            "installation_commands": [
                "sudo apt-get update -y",
                "sudo apt-get install -y postgresql postgresql-contrib",
                "sudo systemctl start postgresql",
                "sudo systemctl enable postgresql",
                "sudo -u postgres psql -c \"SELECT version();\"",
                "echo 'PostgreSQL Server running on: {{public_ip}}:5432'"
            ],
            "verification_commands": [
                "sudo systemctl is-active postgresql",
                "sudo -u postgres psql -c 'SELECT version();'"
            ],
            "expected_ports": ["5432"],
            "access_pattern": "postgresql://{{public_ip}}:5432"
        },
        "docker": {
            "name": "Docker Container Runtime",
            "category": "container",
            "description": "Docker container runtime deployment",
            "hardware_requirements": {
                "cpu_cores": {"min": 2, "recommended": 4},
                "memory_gb": {"min": 4, "recommended": 8},
                "disk_gb": {"min": 50, "recommended": 100}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "cost",
            "installation_commands": [
                "sudo apt-get update -y",
                "sudo apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release",
                "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg",
                "echo \"deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
                "sudo apt-get update -y",
                "sudo apt-get install -y docker-ce docker-ce-cli containerd.io",
                "sudo systemctl start docker",
                "sudo systemctl enable docker",
                "sudo usermod -aG docker $USER",
                "echo 'Docker installed successfully on: {{public_ip}}'"
            ],
            "verification_commands": [
                "sudo docker --version",
                "sudo docker run hello-world"
            ],
            "expected_ports": ["2376"],
            "access_pattern": "docker://{{public_ip}}:2376"
        },
        "node": {
            "name": "Node.js Runtime",
            "category": "runtime",
            "description": "Node.js application runtime deployment",
            "hardware_requirements": {
                "cpu_cores": {"min": 2, "recommended": 4},
                "memory_gb": {"min": 2, "recommended": 4},
                "disk_gb": {"min": 50, "recommended": 100}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "cost",
            "installation_commands": [
                "sudo apt-get update -y",
                "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -",
                "sudo apt-get install -y nodejs",
                "node --version",
                "npm --version",
                "echo 'Node.js installed on: {{public_ip}}'"
            ],
            "verification_commands": [
                "node --version",
                "npm --version"
            ],
            "expected_ports": ["3000", "8080"],
            "access_pattern": "http://{{public_ip}}:3000"
        },
        "python": {
            "name": "Python Development Environment",
            "category": "runtime",
            "description": "Python development environment setup",
            "hardware_requirements": {
                "cpu_cores": {"min": 2, "recommended": 4},
                "memory_gb": {"min": 2, "recommended": 4},
                "disk_gb": {"min": 50, "recommended": 100}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "cost",
            "installation_commands": [
                "sudo apt-get update -y",
                "sudo apt-get install -y python3 python3-pip python3-venv",
                "python3 --version",
                "pip3 --version",
                "python3 -m venv ~/venv",
                "echo 'Python environment ready on: {{public_ip}}'"
            ],
            "verification_commands": [
                "python3 --version",
                "pip3 --version"
            ],
            "expected_ports": ["8000", "5000"],
            "access_pattern": "ssh://{{public_ip}}"
        }
    }
    
    # Find matching application
    matched_app = None
    for app_key, config in deployment_guides.items():
        if app_key in app_lower or app_lower in app_key:
            matched_app = config.copy()
            break
    
    # Default guide for unknown applications
    if not matched_app:
        matched_app = {
            "name": f"{application_name} (Generic)",
            "category": "general",
            "description": f"Generic deployment guide for {application_name}",
            "hardware_requirements": {
                "cpu_cores": {"min": 2, "recommended": 4},
                "memory_gb": {"min": 4, "recommended": 8},
                "disk_gb": {"min": 50, "recommended": 100}
            },
            "vm_spec_filter": {
                "vCPU": {"min": 2},
                "memoryGiB": {"min": 4}
            },
            "priority": "cost",
            "installation_commands": [
                "sudo apt-get update -y",
                f"# Install {application_name} - customize these commands",
                f"# sudo apt-get install -y {application_name.lower()}",
                f"echo '{application_name} installation completed on: {{{{public_ip}}}}'"
            ],
            "verification_commands": [
                f"# Verify {application_name} installation",
                f"# systemctl is-active {application_name.lower()}"
            ],
            "expected_ports": ["80", "443"],
            "access_pattern": "http://{{public_ip}}"
        }
    
    # Apply deployment type adjustments
    if deployment_type == "production":
        # Increase resources for production
        for resource in ["cpu_cores", "memory_gb", "disk_gb"]:
            if resource in matched_app["hardware_requirements"]:
                matched_app["hardware_requirements"][resource]["recommended"] = int(
                    matched_app["hardware_requirements"][resource]["recommended"] * 1.5
                )
    
    # Generate comprehensive deployment guide
    deployment_guide = {
        "application_info": {
            "name": matched_app["name"],
            "category": matched_app["category"],
            "description": matched_app["description"],
            "deployment_type": deployment_type
        },
        "hardware_requirements": matched_app["hardware_requirements"],
        "deployment_workflow": {
            "step_1_namespace": {
                "title": "Check there is existing Namespace such as default, create if not with a proper naming",
                "description": "Ensure there is the namespace (default), create if not with a proper naming, if the default namespace exist use it for deployment",
                "tools_to_use": ["check_and_prepare_namespace", "create_namespace_with_validation"],
                "example": "check_and_prepare_namespace('default')"
            },
            "step_2_vm_specs": {
                "title": "Get VM Specifications",
                "description": "Find appropriate VM specifications based on application requirements",
                "tools_to_use": ["recommend_vm_spec"],
                "filter_policies": matched_app["vm_spec_filter"],
                "priority_policy": matched_app["priority"],
                "example": f"recommend_vm_spec(filter_policies={matched_app['vm_spec_filter']}, priority_policy='{matched_app['priority']}')"
            },
            "step_3_validation": {
                "title": "Validate MCI Configuration",
                "description": "Review MCI configuration before creation",
                "tools_to_use": ["review_mci_dynamic_request"],
                "example": "review_mci_dynamic_request(ns_id='my-app-ns', name='my-app-mci', vm_configurations=vm_configs)"
            },
            "step_4_mci_creation": {
                "title": "Create MCI Infrastructure",
                "description": "Create the multi-cloud infrastructure",
                "tools_to_use": ["create_mci_dynamic"],
                "example": "create_mci_dynamic(ns_id='my-app-ns', name='my-app-mci', vm_configurations=vm_configs)"
            },
            "step_5_application_deployment": {
                "title": "Deploy Application",
                "description": "Install and configure the application using remote commands",
                "tools_to_use": ["execute_command_mci"],
                "commands": matched_app["installation_commands"],
                "example": "execute_command_mci(ns_id='my-app-ns', mci_id='my-app-mci', commands=installation_commands)"
            },
            "step_6_verification": {
                "title": "Verify Deployment",
                "description": "Verify the application is running correctly",
                "tools_to_use": ["execute_command_mci", "get_mci_access_info"],
                "commands": matched_app["verification_commands"],
                "example": "execute_command_mci(ns_id='my-app-ns', mci_id='my-app-mci', commands=verification_commands)"
            },
            "step_7_access_info": {
                "title": "Collect Access Information",
                "description": "Get endpoints and access information",
                "tools_to_use": ["get_mci_access_info"],
                "expected_access": matched_app["access_pattern"],
                "expected_ports": matched_app["expected_ports"],
                "example": "get_mci_access_info(ns_id='my-app-ns', mci_id='my-app-mci')"
            }
        },
        "commands": {
            "installation": matched_app["installation_commands"],
            "verification": matched_app["verification_commands"]
        },
        "expected_results": {
            "ports": matched_app["expected_ports"],
            "access_pattern": matched_app["access_pattern"],
            "service_endpoints": f"Application will be accessible via {matched_app['access_pattern']}"
        },
        "troubleshooting": {
            "common_issues": [
                "Check if the application service is running: systemctl status <service>",
                "Verify ports are open: sudo netstat -tlnp | grep <port>",
                "Check logs: journalctl -u <service> -f",
                "Verify firewall rules: sudo ufw status"
            ],
            "useful_commands": [
                "ps aux | grep <application>",
                "sudo systemctl restart <service>",
                "tail -f /var/log/<application>.log",
                "curl -I http://{{public_ip}}"
            ]
        }
    }
    
    return deployment_guide
    

# Tool: Execute custom commands on deployed applications
@mcp.tool()
def execute_application_commands(
    mci_id: str,
    commands: List[str],
    namespace_id: str = "default",
    target_selection: Optional[Dict] = None
) -> Dict:
    """
    Execute custom commands on deployed application infrastructure.
    
    🚨 **EXECUTION TIME WARNING:**
    Application commands may take considerable time depending on:
    - Command complexity and processing requirements
    - Application-specific operations (database operations, file processing, etc.)
    - Network operations and data transfers
    - Multiple VM coordination
    
    **LLM should inform users about potential delays before executing application commands.**
    
    **LLM Usage Guidelines:**
    1. 🚨 NEVER send empty commands - Always validate command content before execution
    2. ⏰ Inform users about potential delays before execution
    3. 📋 Break complex operations into smaller command batches
    4. 🎯 Use template variables for dynamic command generation
    
    Args:
        mci_id: MCI ID where application is deployed
        commands: List of commands to execute (supports template variables)
        namespace_id: Namespace ID (default: "default")
        target_selection: Optional target selection:
            - type: "mci" | "subgroup" | "vm"
            - target_id: specific subgroup or VM ID (if applicable)
    
    Template variables supported in commands:
        - {{public_ip}}: Public IP of target VM
        - {{private_ip}}: Private IP of target VM
        - {{mci_id}}: MCI ID
        - {{public_ips_space}}: All public IPs separated by space
        - {{public_ips_comma}}: All public IPs separated by comma
    
    Returns:
        Command execution results with expanded templates
    """
    try:
        # 🚨 CRITICAL: Validate commands before execution
        if not commands or len(commands) == 0:
            return {
                "status": "error",
                "error": "Empty command list provided",
                "message": "At least one command must be specified for execution",
                "suggestion": "Provide meaningful commands for application management"
            }
        
        # Check for empty or whitespace-only commands
        valid_commands = []
        for cmd in commands:
            if not cmd or not cmd.strip():
                return {
                    "status": "error",
                    "error": f"Empty or whitespace-only command detected: '{cmd}'",
                    "message": "All commands must contain actual executable content",
                    "suggestion": "Remove empty commands and provide meaningful command strings"
                }
            valid_commands.append(cmd.strip())
        
        if len(valid_commands) == 0:
            return {
                "status": "error",
                "error": "No valid commands found after filtering",
                "message": "All provided commands were empty or contained only whitespace",
                "suggestion": "Provide meaningful commands with actual content"
            }
        
        # Get MCI access info for template expansion
        access_info = get_mci_access_info(namespace_id, mci_id, show_ssh_key=False)
        
        if access_info.get("status") != "success":
            return {
                "status": "error",
                "error": "Failed to get MCI access information",
                "details": access_info
            }
        
        # Expand command templates
        expanded_commands = []
        for cmd in valid_commands:  # Use valid_commands instead of commands
            expanded_cmd = _expand_command_templates(cmd, access_info, mci_id)
            expanded_commands.append(expanded_cmd)
        
        # Execute commands
        execute_result = execute_command_mci(
            namespace_id=namespace_id,
            mci_id=mci_id,
            commands=expanded_commands,
            **{k: v for k, v in (target_selection or {}).items() if k != "type"}
        )
        
        return {
            "status": "success",
            "original_commands": commands,
            "expanded_commands": expanded_commands,
            "execution_result": execute_result,
            "template_variables_used": _extract_used_templates(commands)
        }
        
    except Exception as e:
        return {
            "status": "error",
            "error": str(e)
        }

# Helper function: Create application deployment plan
def _create_application_deployment_plan(
    app_config: Dict, 
    regions: Union[int, List[str]], 
    instances_per_region: int,
    namespace_id: str,
    hardware_research: Optional[Dict] = None
) -> Dict:
    """Create detailed deployment plan for application with enhanced hardware specifications."""
    
    # Determine target regions
    if isinstance(regions, int):
        # Auto-select regions based on deployment strategy
        strategy = app_config.get("deployment_strategy", "regional")
        target_regions = _select_optimal_regions(regions, strategy)
    else:
        target_regions = regions
    
    # Calculate total instances
    total_instances = len(target_regions) * instances_per_region
    
    # Determine VM specifications based on requirements and hardware research
    vm_requirements = _translate_app_requirements_to_vm_specs(
        app_config["requirements"], hardware_research
    )
    
    # Determine disk size with minimum 50GB and application-specific requirements
    min_disk_gb = max(
        50,  # Minimum 50GB as requested
        app_config["requirements"].get("min_disk_gb", 50),
        hardware_research.get("recommendations", {}).get("disk_gb", 50) if hardware_research else 50
    )
    
    deployment_plan = {
        "application_config": app_config,
        "hardware_research_applied": hardware_research is not None,
        "deployment_strategy": {
            "type": app_config.get("deployment_strategy", "regional"),
            "target_regions": target_regions,
            "instances_per_region": instances_per_region,
            "total_instances": total_instances
        },
        "infrastructure_requirements": {
            "vm_specifications": vm_requirements,
            "disk_requirements": {
                "min_disk_gb": min_disk_gb,
                "disk_type": "default",
                "research_based": hardware_research is not None
            },
            "network_requirements": {
                "ports": app_config["requirements"].get("ports", []),
                "bandwidth_intensive": app_config["requirements"].get("bandwidth_intensive", False)
            },
            "estimated_cost": {
                "per_instance_hourly": _estimate_app_cost(app_config["requirements"]),
                "total_hourly": _estimate_app_cost(app_config["requirements"]) * total_instances,
                "estimated_monthly": _estimate_app_cost(app_config["requirements"]) * total_instances * 24 * 30
            }
        },
        "deployment_commands": app_config["commands"],
        "expected_endpoints": total_instances
    }
    
    # Add hardware research details if available
    if hardware_research:
        deployment_plan["hardware_research_summary"] = {
            "cpu_recommendation": hardware_research.get("recommendations", {}).get("cpu_cores", 2),
            "memory_recommendation": hardware_research.get("recommendations", {}).get("memory_gb", 4),
            "disk_recommendation": hardware_research.get("recommendations", {}).get("disk_gb", 50),
            "research_confidence": hardware_research.get("requirements_found", {}).get("confidence", "medium"),
            "sources_checked": hardware_research.get("total_sources_checked", 0)
        }
    
    return deployment_plan

# Helper function: Select optimal regions for deployment
def _select_optimal_regions(count: int, strategy: str) -> List[str]:
    """Select optimal regions based on deployment strategy."""
    
    # Define region priorities based on strategy
    strategy_regions = {
        "global": [
            "us-east-1", "eu-west-1", "ap-northeast-2", "ap-southeast-1", 
            "us-west-2", "eu-central-1", "ap-south-1", "sa-east-1",
            "ap-northeast-1", "ca-central-1"
        ],
        "performance": [
            "us-east-1", "eu-west-1", "ap-northeast-2", "us-west-2"
        ],
        "regional": [
            "us-east-1", "eu-west-1", "ap-northeast-2"
        ],
        "centralized": ["us-east-1"],
        "cluster": ["us-east-1"]  # Single region for cluster deployments
    }
    
    available_regions = strategy_regions.get(strategy, strategy_regions["regional"])
    return available_regions[:min(count, len(available_regions))]

# Helper function: Translate app requirements to VM specs
def _translate_app_requirements_to_vm_specs(requirements: Dict, hardware_research: Optional[Dict] = None) -> Dict:
    """Translate application requirements to VM specification filters with hardware research integration."""
    
    vm_filter = {"Architecture": "x86_64"}  # Default to x86_64
    
    # Get research-based recommendations if available
    research_cpu = None
    research_memory = None
    
    if hardware_research and hardware_research.get("recommendations"):
        research_cpu = hardware_research["recommendations"].get("cpu_cores")
        research_memory = hardware_research["recommendations"].get("memory_gb")
    
    # CPU requirements - use research data if available, otherwise fallback to original logic
    if research_cpu:
        # Use researched CPU requirements with some buffer
        min_cpu = max(2, research_cpu)
        max_cpu = min(32, research_cpu * 2)
        vm_filter["vCPU"] = {"min": min_cpu, "max": max_cpu}
    elif requirements.get("cpu_intensive"):
        vm_filter["vCPU"] = {"min": 4, "max": 16}
    else:
        vm_filter["vCPU"] = {"min": 2, "max": 8}
    
    # Memory requirements - use research data if available, otherwise fallback to original logic
    if research_memory:
        # Use researched memory requirements with some buffer
        min_memory = max(4, research_memory)
        max_memory = min(64, research_memory * 2)
        vm_filter["memoryGiB"] = {"min": min_memory, "max": max_memory}
    elif requirements.get("memory_intensive"):
        vm_filter["memoryGiB"] = {"min": 8, "max": 32}
    else:
        vm_filter["memoryGiB"] = {"min": 4, "max": 16}
    
    # GPU requirements
    priority_policy = "cost"
    if requirements.get("gpu_preferred") or requirements.get("gpu_required"):
        priority_policy = "performance"
        # Note: GPU filtering would need additional CB-TB support
    
    # If we have high-confidence research, prioritize performance
    if hardware_research and hardware_research.get("requirements_found", {}).get("confidence") == "high":
        priority_policy = "performance"
    
    return {
        "filter_policies": vm_filter,
        "priority_policy": priority_policy,
        "research_applied": hardware_research is not None,
        "research_confidence": hardware_research.get("requirements_found", {}).get("confidence", "none") if hardware_research else "none"
    }

# Helper function: Provision application infrastructure
def _provision_application_infrastructure(
    deployment_plan: Dict,
    mci_name: str,
    namespace_id: str
) -> Dict:
    """Provision infrastructure for application deployment."""
    
    try:
        strategy = deployment_plan["deployment_strategy"]
        target_regions = strategy["target_regions"]
        instances_per_region = strategy["instances_per_region"]
        vm_requirements = deployment_plan["infrastructure_requirements"]["vm_specifications"]
        disk_requirements = deployment_plan["infrastructure_requirements"].get("disk_requirements", {})
        
        # Get disk size (minimum 50GB as requested)
        disk_size = str(disk_requirements.get("min_disk_gb", 50))
        
        # Get VM specifications
        vm_specs_result = recommend_vm_spec(
            filter_policies=vm_requirements["filter_policies"],
            priority_policy=vm_requirements["priority_policy"],
            limit="50"
        )
        
        if vm_specs_result.get("status") != "success":
            return {
                "status": "error",
                "error": "Failed to get VM specifications",
                "details": vm_specs_result
            }
        
        # Get suitable images
        images_result = search_images(
            provider_name=None,  # Search all providers
            os_type="ubuntu 22.04"  # Default to Ubuntu 22.04
        )
        
        if images_result.get("status") != "success":
            return {
                "status": "error", 
                "error": "Failed to search for images",
                "details": images_result
            }
        
        # Create VM configurations for each region
        vm_configurations = []
        available_specs = vm_specs_result.get("summarized_specs", [])
        available_images = images_result.get("image_list", [])
        
        for i, region in enumerate(target_regions):
            # Find specs and images for this region
            region_specs = [s for s in available_specs if region in s.get("region_name", "")]
            region_images = [img for img in available_images if region in img.get("region", "")]
            
            if not region_specs or not region_images:
                continue
            
            # Select best spec and image for this region
            selected_spec = region_specs[0]  # First spec (sorted by priority)
            selected_image = region_images[0]  # First available image
            
            # Create VM configuration with enhanced disk settings
            vm_config = {
                "name": f"app-vm-{region}-{i+1}",
                "imageId": selected_image["csp_image_name"],
                "specId": selected_spec["id"],
                "description": f"Application VM in {region}",
                "subGroupSize": str(instances_per_region),
                "rootDiskSize": disk_size,  # Set disk size (minimum 50GB)
                "rootDiskType": "default"
            }
            
            vm_configurations.append(vm_config)
        
        if not vm_configurations:
            return {
                "status": "error",
                "error": "No suitable VM configurations found for target regions"
            }
        
        # Create MCI
        mci_result = create_mci_dynamic(
            ns_id=namespace_id,
            name=mci_name,
            vm_configurations=vm_configurations,
            description=f"Infrastructure for {deployment_plan['application_config']['name']}",
        )
        
        return {
            "status": "success",
            "mci_id": mci_name,
            "mci_result": mci_result,
            "vm_configurations": vm_configurations,
            "total_vms": sum(int(vm["subGroupSize"]) for vm in vm_configurations)
        }
        
    except Exception as e:
        return {
            "status": "error",
            "error": str(e)
        }

# Helper function: Deploy application to infrastructure
def _deploy_application_to_infrastructure(
    app_config: Dict,
    mci_id: str,
    namespace_id: str,
    deployment_plan: Dict
) -> Dict:
    """Deploy application to provisioned infrastructure."""
    
    try:
        # Wait for MCI to be ready
        import time
        max_wait_time = 300  # 5 minutes
        wait_interval = 10   # Check every 10 seconds
        waited_time = 0
        
        while waited_time < max_wait_time:
            mci_status = get_mci(namespace_id, mci_id)
            if mci_status.get("status") == "success":
                mci_data = mci_status.get("mci", {})
                if mci_data.get("status") == "running":
                    break
            
            time.sleep(wait_interval)
            waited_time += wait_interval
        
        if waited_time >= max_wait_time:
            return {
                "status": "error",
                "error": "MCI did not reach running state within timeout period"
            }
        
        # Get MCI access info for command expansion
        access_info = get_mci_access_info(namespace_id, mci_id, show_ssh_key=False)
        
        # Execute deployment commands
        deployment_commands = app_config["commands"]
        expanded_commands = []
        
        for cmd in deployment_commands:
            expanded_cmd = _expand_command_templates(cmd, access_info, mci_id)
            expanded_commands.append(expanded_cmd)
        
        # Execute commands on MCI
        execution_result = execute_command_mci(
            namespace_id=namespace_id,
            mci_id=mci_id,
            commands=expanded_commands
        )
        
        return {
            "status": "success",
            "original_commands": deployment_commands,
            "expanded_commands": expanded_commands,
            "execution_result": execution_result,
            "deployment_time": datetime.now().isoformat()
        }
        
    except Exception as e:
        return {
            "status": "error",
            "error": str(e)
        }

# Helper function: Expand command templates
def _expand_command_templates(command: str, access_info: Dict, mci_id: str) -> str:
    """
    Expand template variables in commands with MapUI compatibility.
    
    Note: CB-Tumblebug built-in functions ($$Func(...)) are processed by the CB-Tumblebug server
    during command execution, so they should be passed through as-is without modification.
    
    Built-in functions include:
    - $$Func(GetPublicIP(target=this))
    - $$Func(GetPublicIPs(separator=' '))
    - $$Func(GetMciId())
    - $$Func(AssignTask(task='...'))
    - etc.
    """
    
    expanded_cmd = command
    
    # Skip processing if command contains CB-Tumblebug built-in functions
    if "$$Func(" in command:
        return command  # Return as-is for CB-Tumblebug to process
    
    # Extract access information for manual template variables
    access_data = access_info.get("access_info", {})
    
    # Get public and private IPs
    public_ips = []
    private_ips = []
    
    for subgroup in access_data.get("mci_subgroup_access_info", []):
        for vm in subgroup.get("vm_access_info_array", []):
            if vm.get("public_ip"):
                public_ips.append(vm["public_ip"])
            if vm.get("private_ip"):
                private_ips.append(vm["private_ip"])
    
    # Process only manual template variables ({{...}})
    if public_ips:
        expanded_cmd = expanded_cmd.replace("{{public_ip}}", public_ips[0])
        expanded_cmd = expanded_cmd.replace("{{public_ips_space}}", " ".join(public_ips))
        expanded_cmd = expanded_cmd.replace("{{public_ips_comma}}", ",".join(public_ips))
        # MapUI-style semicolon separator with port
        expanded_cmd = expanded_cmd.replace("{{public_ips_semicolon_with_port}}", 
                                          ";".join([f"http://{ip}:3000" for ip in public_ips]))
    
    if private_ips:
        expanded_cmd = expanded_cmd.replace("{{private_ip}}", private_ips[0])
        expanded_cmd = expanded_cmd.replace("{{private_ips_space}}", " ".join(private_ips))
        expanded_cmd = expanded_cmd.replace("{{private_ips_comma}}", ",".join(private_ips))
    
    # MCI ID replacement
    expanded_cmd = expanded_cmd.replace("{{mci_id}}", mci_id)
    
    # Special placeholder for Ray head IP (for worker nodes)
    # This would need to be set by the deployment context
    expanded_cmd = expanded_cmd.replace("{{ray_head_ip}}", public_ips[0] if public_ips else "RAY_HEAD_IP_PLACEHOLDER")
    
    return expanded_cmd

# Helper function: Collect service endpoints
def _collect_service_endpoints(
    app_config: Dict,
    mci_id: str,
    namespace_id: str,
    deployment_result: Dict
) -> List[Dict]:
    """Collect service endpoints from deployment results."""
    
    endpoints = []
    
    try:
        # Get execution results
        execution_result = deployment_result.get("execution_result", {})
        
        if execution_result.get("status") == "success":
            results = execution_result.get("results", [])
            result_pattern = app_config.get("result_pattern")
            
            # Extract endpoints from command results
            for result in results:
                result_text = result.get("result", "")
                
                # Look for endpoint patterns
                if result_pattern and result_text:
                    import re
                    matches = re.findall(result_pattern, result_text)
                    for match in matches:
                        if isinstance(match, tuple):
                            endpoint_url = f"{match[0]}:{match[1]}"
                        else:
                            endpoint_url = match
                        
                        endpoints.append({
                            "type": "service_endpoint",
                            "url": endpoint_url,
                            "vm_id": result.get("vm_id", "unknown"),
                            "description": f"{app_config['name']} service endpoint"
                        })
                
                # Also look for common URL patterns
                import re
                url_patterns = [
                    r'(https?://[^\s]+)',
                    r'Server Address: ([^\s]+)',
                    r'Access URL: ([^\s]+)',
                    r'Endpoint: ([^\s]+)'
                ]
                
                for pattern in url_patterns:
                    matches = re.findall(pattern, result_text)
                    for match in matches:
                        if match not in [ep["url"] for ep in endpoints]:
                            endpoints.append({
                                "type": "detected_endpoint",
                                "url": match,
                                "vm_id": result.get("vm_id", "unknown"),
                                "description": f"Auto-detected endpoint for {app_config['name']}"
                            })
        
        # If no specific endpoints found, create default ones based on access info
        if not endpoints:
            access_info = get_mci_access_info(namespace_id, mci_id, show_ssh_key=False)
            access_data = access_info.get("access_info", {})
            
            for subgroup in access_data.get("mci_subgroup_access_info", []):
                for vm in subgroup.get("vm_access_info_array", []):
                    if vm.get("public_ip"):
                        # Create default endpoint based on common ports
                        ports = app_config["requirements"].get("ports", ["80"])
                        for port in ports:
                            endpoints.append({
                                "type": "default_endpoint",
                                "url": f"http://{vm['public_ip']}:{port}",
                                "vm_id": vm.get("vm_id", "unknown"),
                                "description": f"{app_config['name']} on port {port}"
                            })
    
    except Exception as e:
        logger.error(f"Error collecting endpoints: {e}")
    
    return endpoints

# Helper function: Generate deployment summary
def _generate_deployment_summary(deployment_result: Dict, app_config: Dict) -> Dict:
    """Generate comprehensive deployment summary."""
    
    infrastructure = deployment_result.get("infrastructure_created", {})
    deployment = deployment_result.get("application_deployment", {})
    endpoints = deployment_result.get("service_endpoints", [])
    
    summary = {
        "deployment_status": deployment_result["status"],
        "application_name": app_config["name"],
        "category": app_config["category"],
        "total_instances": infrastructure.get("total_vms", 0),
        "service_endpoints": len(endpoints),
        "deployment_time": deployment_result.get("end_time"),
        "next_steps": [],
        "access_information": endpoints
    }
    
    # Add next steps based on application type
    if deployment_result["status"] == "completed":
        summary["next_steps"] = [
            f"✅ {app_config['name']} has been successfully deployed",
            f"🌐 {len(endpoints)} service endpoints are available",
            "🔗 Use the provided URLs to access your services",
            "📊 Monitor service health through CB-Tumblebug dashboard",
            "🛠️ Use execute_application_commands() for additional configuration"
        ]
        
        if app_config["category"] == "game":
            summary["next_steps"].append("🎮 Share server addresses with players")
        elif app_config["category"] == "web":
            summary["next_steps"].append("🌍 Configure DNS for production use")
        elif app_config["category"] == "llm":
            summary["next_steps"].append("🤖 Pull LLM models using appropriate commands")
    
    return summary

# Helper function: Estimate application cost
def _estimate_app_cost(requirements: Dict) -> float:
    """Estimate hourly cost based on application requirements."""
    
    base_cost = 0.10  # $0.10/hour base
    
    if requirements.get("cpu_intensive"):
        base_cost += 0.05
    if requirements.get("memory_intensive"):
        base_cost += 0.05
    if requirements.get("gpu_preferred"):
        base_cost += 0.30
    if requirements.get("bandwidth_intensive"):
        base_cost += 0.02
    
    return round(base_cost, 3)

# Helper function: Get recommended locations for deployment strategy
def _get_recommended_locations(strategy: str) -> List[str]:
    """Get recommended locations based on deployment strategy."""
    
    location_strategies = {
        "global": ["Global distribution across 6+ continents", "US, EU, Asia-Pacific, South America"],
        "regional": ["Regional deployment in major markets", "US, EU, Asia"],
        "performance": ["High-performance regions", "US East, EU West, Asia Northeast"],
        "centralized": ["Single region deployment", "US East"],
        "cluster": ["Co-located cluster deployment", "Single region for optimal networking"]
    }
    
    return location_strategies.get(strategy, ["Regional deployment"])

# Helper function: Generate deployment confirmation message
def _generate_deployment_confirmation(deployment_plan: Dict, app_config: Dict, hardware_research: Optional[Dict] = None) -> str:
    """Generate user-friendly deployment confirmation message with hardware research information."""
    
    strategy = deployment_plan["deployment_strategy"]
    requirements = deployment_plan["infrastructure_requirements"]
    disk_req = requirements.get("disk_requirements", {})
    
    # Hardware research section
    hardware_section = ""
    if hardware_research:
        research_summary = deployment_plan.get("hardware_research_summary", {})
        confidence = research_summary.get("research_confidence", "medium")
        hardware_section = f"""
**Hardware Research Results:**
- CPU Recommendation: {research_summary.get('cpu_recommendation', 2)} cores
- Memory Recommendation: {research_summary.get('memory_recommendation', 4)} GB
- Disk Requirement: {research_summary.get('disk_recommendation', 50)} GB (minimum: {disk_req.get('min_disk_gb', 50)} GB)
- Research Confidence: {confidence.title()}
- Sources Checked: {research_summary.get('sources_checked', 'Built-in knowledge')}
"""
    else:
        hardware_section = """
**Hardware Configuration:**
- Using default specifications (no research performed)
- Disk Size: {disk_req.get('min_disk_gb', 50)} GB minimum
""".format(disk_req=disk_req)
    
    msg = f"""
🚀 **{app_config['name']} Deployment Plan**

**Application Details:**
- Type: {app_config['category'].title()} Application
- Description: {app_config['description']}
{hardware_section}
**Deployment Strategy:**
- Regions: {', '.join(strategy['target_regions'])}
- Instances per region: {strategy['instances_per_region']}
- Total instances: {strategy['total_instances']}

**Resource Requirements:**
- Estimated cost: ${requirements['estimated_cost']['total_hourly']:.3f}/hour
- Monthly estimate: ${requirements['estimated_cost']['estimated_monthly']:.2f}
- Network ports: {', '.join(requirements['network_requirements']['ports'])}
- Disk size per VM: {disk_req.get('min_disk_gb', 50)} GB

**Next Steps:**
1. Infrastructure will be provisioned with researched specifications
2. Application will be deployed using predefined scripts
3. Service endpoints will be collected and reported

⚠️ **Important:** This will create billable cloud resources. 
Are you sure you want to proceed with this deployment?

To confirm, call this function again with auto_confirm=True
"""
    
    return msg.strip()

# Helper function: Extract used template variables
def _extract_used_templates(commands: List[str]) -> List[str]:
    """Extract template variables used in commands."""
    
    import re
    template_pattern = r'\{\{([^}]+)\}\}'
    used_templates = set()
    
    for cmd in commands:
        matches = re.findall(template_pattern, cmd)
        used_templates.update(matches)
    
    return sorted(list(used_templates))

#####################################
# Enhanced Remote Command Execution
#####################################

# Tool: Execute computational task with automatic infrastructure provisioning
@mcp.tool()
def execute_compute_task(
    task_description: str,
    computation_requirements: Optional[Dict] = None,
    cleanup_after_completion: bool = True,
    namespace_id: Optional[str] = None,
    create_temporary_namespace: bool = False
) -> Dict:
    """
    Execute computational tasks by automatically provisioning infrastructure,
    running the computation, collecting results, and cleaning up resources.
    
    🚨 **EXTENDED EXECUTION TIME WARNING:**
    This function involves MULTIPLE time-consuming operations:
    - Infrastructure provisioning: 3-8 minutes
    - Environment setup and software installation: 5-15 minutes
    - Actual computation execution: Variable (minutes to hours)
    - Result collection and processing: 1-3 minutes
    - Resource cleanup: 2-5 minutes
    
    **Total expected time: 15-30+ minutes for complete workflow**
    **LLM MUST inform users about the extended duration before execution.**
    
    This function provides a complete compute-as-a-service workflow:
    1. Analyze computational requirements
    2. Use existing or create temporary namespace
    3. Deploy computation environment
    4. Execute the requested task
    5. Collect and return results
    6. Clean up compute resources (preserving namespace unless temporary)
    
    Args:
        task_description: Natural language description of the computational task
        computation_requirements: Optional requirements specification:
            - cpu_intensive: bool (default: False)
            - memory_intensive: bool (default: False) 
            - parallel_processing: bool (default: False)
            - estimated_duration: str (e.g., "5 minutes", "1 hour")
            - preferred_os: str (default: "ubuntu")
            - required_software: List[str] (e.g., ["python", "numpy", "scipy"])
        cleanup_after_completion: Whether to delete compute resources after completion
        namespace_id: Specific namespace to use (default: "default")
        create_temporary_namespace: Whether to create a temporary namespace for isolation
    
    Returns:
        Complete workflow result including:
        - task_analysis: Analysis of computational requirements
        - infrastructure_created: Details of provisioned resources
        - computation_results: Results of the computational task
        - cleanup_status: Resource cleanup information
        - execution_summary: Overall workflow summary
    
    Example:
    ```python
    # Use default namespace
    result = execute_compute_task(
        task_description="Calculate pi using Monte Carlo method with 1 million samples"
    )
    
    # Use temporary namespace for isolation
    result = execute_compute_task(
        task_description="Calculate pi using Monte Carlo method with 1 million samples",
        create_temporary_namespace=True
    )
    ```
    """
    workflow_id = f"compute-{int(datetime.now().timestamp())}"
    
    # Determine namespace strategy
    if create_temporary_namespace:
        target_namespace = f"compute-task-{workflow_id}"
        is_temporary_namespace = True
    else:
        target_namespace = namespace_id or "default"
        is_temporary_namespace = False
    
    workflow_result = {
        "workflow_id": workflow_id,
        "task_description": task_description,
        "namespace_strategy": {
            "namespace_id": target_namespace,
            "is_temporary": is_temporary_namespace,
            "will_be_deleted": is_temporary_namespace and cleanup_after_completion
        },
        "start_time": datetime.now().isoformat(),
        "task_analysis": {},
        "infrastructure_created": {},
        "computation_results": {},
        "cleanup_status": {},
        "execution_summary": {},
        "status": "starting"
    }
    
    try:
        # STEP 1: Analyze computational requirements
        workflow_result["status"] = "analyzing_requirements"
        task_analysis = _analyze_compute_requirements(task_description, computation_requirements)
        workflow_result["task_analysis"] = task_analysis
        
        # STEP 2: Prepare namespace
        workflow_result["status"] = "preparing_namespace"
        if is_temporary_namespace:
            # Create temporary namespace
            namespace_result = _internal_create_namespace_with_validation(
                name=target_namespace,
                description=f"Temporary namespace for compute task: {task_description[:50]}..."
            )
            
            if not namespace_result.get("created", False) and not namespace_result.get("namespace_id"):
                workflow_result["status"] = "failed"
                workflow_result["error"] = "Failed to create temporary namespace"
                return workflow_result
        else:
            # Validate existing namespace
            namespace_validation = _internal_validate_namespace(target_namespace)
            if not namespace_validation["valid"]:
                workflow_result["status"] = "failed"
                workflow_result["error"] = f"Namespace '{target_namespace}' is not valid or accessible"
                return workflow_result
        
        # STEP 3: Provision infrastructure based on requirements
        workflow_result["status"] = "provisioning_infrastructure"
        infrastructure_result = _provision_compute_infrastructure(
            target_namespace,
            task_analysis,
            workflow_id
        )
        workflow_result["infrastructure_created"] = infrastructure_result
        
        if infrastructure_result.get("status") != "success":
            workflow_result["status"] = "failed"
            workflow_result["error"] = "Failed to provision infrastructure"
            return workflow_result
        
        # STEP 4: Deploy computation environment and execute task
        workflow_result["status"] = "executing_computation"
        computation_result = _execute_computation_on_infrastructure(
            target_namespace,
            infrastructure_result["mci_id"],
            task_description,
            task_analysis
        )
        workflow_result["computation_results"] = computation_result
        
        # STEP 5: Cleanup resources if requested
        if cleanup_after_completion:
            workflow_result["status"] = "cleaning_up"
            cleanup_result = _cleanup_compute_resources(
                target_namespace, 
                delete_namespace=is_temporary_namespace
            )
            workflow_result["cleanup_status"] = cleanup_result
        
        # STEP 6: Generate execution summary
        workflow_result["status"] = "completed"
        workflow_result["end_time"] = datetime.now().isoformat()
        workflow_result["execution_summary"] = _generate_compute_execution_summary(workflow_result)
        
        # Store execution in memory for future reference
        _store_interaction_memory(
            user_request=f"Execute compute task: {task_description}",
            llm_response=f"Compute task completed successfully with results: {computation_result.get('summary', 'N/A')}",
            operation_type="compute_task_execution",
            context_data={
                "workflow_id": workflow_id,
                "namespace": target_namespace,
                "namespace_preserved": not is_temporary_namespace,
                "cleanup_performed": cleanup_after_completion
            },
            status="completed"
        )
        
        return workflow_result
        
    except Exception as e:
        workflow_result["status"] = "error"
        workflow_result["error"] = str(e)
        workflow_result["end_time"] = datetime.now().isoformat()
        
        # Attempt emergency cleanup
        if cleanup_after_completion:
            try:
                emergency_cleanup = _cleanup_compute_resources(
                    target_namespace,
                    delete_namespace=is_temporary_namespace
                )
                workflow_result["emergency_cleanup"] = emergency_cleanup
            except:
                workflow_result["emergency_cleanup"] = {"status": "failed", "message": "Emergency cleanup failed"}
        
        return workflow_result

# Helper function: Analyze computational requirements
def _analyze_compute_requirements(task_description: str, requirements: Optional[Dict] = None) -> Dict:
    """
    Analyze computational task to determine infrastructure requirements.
    """
    requirements = requirements or {}
    
    # Default analysis
    analysis = {
        "task_type": "general_computation",
        "cpu_intensive": requirements.get("cpu_intensive", False),
        "memory_intensive": requirements.get("memory_intensive", False),
        "parallel_processing": requirements.get("parallel_processing", False),
        "estimated_duration": requirements.get("estimated_duration", "unknown"),
        "preferred_os": requirements.get("preferred_os", "ubuntu"),
        "required_software": requirements.get("required_software", ["python"]),
        "infrastructure_requirements": {}
    }
    
    # Analyze task description for hints
    task_lower = task_description.lower()
    
    # Detect computation type
    if any(keyword in task_lower for keyword in ["machine learning", "ml", "deep learning", "neural network"]):
        analysis["task_type"] = "machine_learning"
        analysis["cpu_intensive"] = True
        analysis["memory_intensive"] = True
        analysis["required_software"].extend(["numpy", "scipy", "scikit-learn"])
    
    elif any(keyword in task_lower for keyword in ["monte carlo", "simulation", "statistical"]):
        analysis["task_type"] = "simulation"
        analysis["cpu_intensive"] = True
        analysis["required_software"].extend(["numpy", "scipy"])
    
    elif any(keyword in task_lower for keyword in ["matrix", "linear algebra", "mathematical"]):
        analysis["task_type"] = "mathematical_computation"
        analysis["cpu_intensive"] = True
        analysis["memory_intensive"] = True
        analysis["required_software"].extend(["numpy", "scipy"])
    
    elif any(keyword in task_lower for keyword in ["data analysis", "data processing", "csv", "dataset"]):
        analysis["task_type"] = "data_processing"
        analysis["memory_intensive"] = True
        analysis["required_software"].extend(["pandas", "numpy"])
    
    # Determine infrastructure requirements with default x86_64 architecture
    if analysis["cpu_intensive"] and analysis["memory_intensive"]:
        analysis["infrastructure_requirements"] = {
            "vm_spec_filter": {"vCPU": {"min": 4, "max": 8}, "memoryGiB": {"min": 8, "max": 16}, "Architecture": "x86_64"},
            "priority_policy": "performance"
        }
    elif analysis["cpu_intensive"]:
        analysis["infrastructure_requirements"] = {
            "vm_spec_filter": {"vCPU": {"min": 2, "max": 4}, "memoryGiB": {"min": 4, "max": 8}, "Architecture": "x86_64"},
            "priority_policy": "performance"
        }
    elif analysis["memory_intensive"]:
        analysis["infrastructure_requirements"] = {
            "vm_spec_filter": {"vCPU": {"min": 2, "max": 4}, "memoryGiB": {"min": 8, "max": 16}, "Architecture": "x86_64"},
            "priority_policy": "cost"
        }
    else:
        analysis["infrastructure_requirements"] = {
            "vm_spec_filter": {"vCPU": {"min": 1, "max": 2}, "memoryGiB": {"min": 2, "max": 4}, "Architecture": "x86_64"},
            "priority_policy": "cost"
        }
    
    return analysis

# Helper function: Provision compute infrastructure
def _provision_compute_infrastructure(namespace: str, task_analysis: Dict, workflow_id: str) -> Dict:
    """
    Provision infrastructure based on computational requirements.
    Automatically retries with different CSP/region/spec/image combinations on failure.
    """
    max_retry_attempts = 3
    spec_requirements = task_analysis.get("infrastructure_requirements", {})
    
    # Get multiple VM specifications for retry options
    vm_specs = recommend_vm_spec(
        filter_policies=spec_requirements.get("vm_spec_filter", {}),
        priority_policy=spec_requirements.get("priority_policy", "cost"),
        limit="20"  # Get more options for retry
    )
    
    if not vm_specs or len(vm_specs.get("summarized_specs", [])) == 0:
        return {"status": "failed", "error": "No suitable VM specifications found"}
    
    available_specs = vm_specs["summarized_specs"]
    retry_log = []
    
    for attempt in range(max_retry_attempts):
        if attempt >= len(available_specs):
            break
            
        try:
            current_spec = available_specs[attempt]
            spec_id = current_spec["id"]
            
            # Extract provider and region from spec
            provider = spec_id.split('+')[0] if '+' in spec_id else "aws"
            region = spec_id.split('+')[1] if '+' in spec_id and len(spec_id.split('+')) > 1 else "us-east-1"
            
            retry_log.append({
                "attempt": attempt + 1,
                "spec_id": spec_id,
                "provider": provider,
                "region": region,
                "status": "attempting"
            })
            
            # Find suitable images for this specification with default x86_64 architecture
            images = search_images(
                provider_name=provider,
                region_name=region,
                os_type=task_analysis.get("preferred_os", "ubuntu"),
                os_architecture="x86_64"  # Default architecture
            )
            
            if not images or len(images.get("imageList", [])) == 0:
                retry_log[-1]["status"] = "failed"
                retry_log[-1]["error"] = f"No images found for {provider} in {region}"
                continue
            
            # Try multiple images if first one fails
            image_list = images["imageList"][:3]  # Try up to 3 images
            
            for image_attempt, image in enumerate(image_list):
                try:
                    # Create MCI configuration
                    vm_configurations = [{
                        "specId": spec_id,
                        "imageId": image.get("cspImageName"),
                        "name": f"compute-vm-{workflow_id}",
                        "description": f"Compute VM for task execution - {workflow_id}",
                        "subGroupSize": "1"
                    }]
                    
                    # Attempt MCI creation
                    mci_result = create_mci_dynamic(
                        ns_id=namespace,
                        name=f"compute-mci-{workflow_id}-attempt{attempt+1}",
                        vm_configurations=vm_configurations,
                        description=f"Compute infrastructure for task: {workflow_id} (Attempt {attempt+1})",
                        skip_confirmation=True  # Skip confirmation for automated workflow
                    )
                    
                    # Check if MCI creation was successful
                    if "error" not in mci_result and mci_result.get("id"):
                        retry_log[-1]["status"] = "success"
                        retry_log[-1]["image_used"] = image.get("cspImageName")
                        retry_log[-1]["image_attempt"] = image_attempt + 1
                        
                        return {
                            "status": "success",
                            "mci_id": mci_result.get("id"),
                            "vm_spec": current_spec,
                            "vm_image": image.get("cspImageName"),
                            "provider": provider,
                            "region": region,
                            "attempt_number": attempt + 1,
                            "total_attempts": attempt + 1,
                            "retry_log": retry_log
                        }
                    else:
                        error_msg = mci_result.get("error", "Unknown MCI creation error")
                        if image_attempt == len(image_list) - 1:  # Last image attempt
                            retry_log[-1]["status"] = "failed"
                            retry_log[-1]["error"] = f"MCI creation failed with all images: {error_msg}"
                        
                except Exception as e:
                    if image_attempt == len(image_list) - 1:  # Last image attempt
                        retry_log[-1]["status"] = "failed"
                        retry_log[-1]["error"] = f"Exception during MCI creation: {str(e)}"
                    continue
                    
        except Exception as e:
            retry_log[-1]["status"] = "failed"
            retry_log[-1]["error"] = f"Specification processing failed: {str(e)}"
            continue
    
    # All attempts failed
    return {
        "status": "failed",
        "error": "All infrastructure provisioning attempts failed",
        "total_attempts": len(retry_log),
        "retry_log": retry_log,
        "available_specs_count": len(available_specs)
    }

# Helper function: Execute computation on infrastructure
def _execute_computation_on_infrastructure(
    namespace: str,
    mci_id: str,
    task_description: str,
    task_analysis: Dict
) -> Dict:
    """
    Execute computational task on provisioned infrastructure.
    """
    try:
        # Wait for MCI to be ready
        max_wait_attempts = 30
        for attempt in range(max_wait_attempts):
            mci_status = get_mci(namespace, mci_id)
            if mci_status.get("status") == "Running":
                break
            elif attempt == max_wait_attempts - 1:
                return {"status": "failed", "error": "MCI failed to reach Running state"}
            # Wait 10 seconds before next check
            import time
            time.sleep(10)
        
        # Install required software
        software_packages = task_analysis.get("required_software", ["python"])
        install_commands = _generate_software_installation_commands(software_packages)
        
        if install_commands:
            install_result = execute_command_mci(
                ns_id=namespace,
                mci_id=mci_id,
                commands=install_commands,
                summarize_output=True
            )
        
        # Generate computation script based on task description
        computation_script = _generate_computation_script(task_description, task_analysis)
        
        # Execute the computation
        execution_result = execute_command_mci(
            ns_id=namespace,
            mci_id=mci_id,
            commands=[computation_script],
            summarize_output=True
        )
        
        return {
            "status": "success",
            "computation_output": execution_result,
            "script_generated": computation_script,
            "software_installed": software_packages
        }
        
    except Exception as e:
        return {"status": "failed", "error": str(e)}

# Helper function: Generate software installation commands
def _generate_software_installation_commands(packages: List[str]) -> List[str]:
    """
    Generate commands to install required software packages.
    """
    commands = [
        "sudo apt-get update -y"
    ]
    
    # System packages
    system_packages = []
    python_packages = []
    
    for package in packages:
        if package in ["python", "python3"]:
            system_packages.append("python3")
            system_packages.append("python3-pip")
        elif package in ["numpy", "scipy", "pandas", "scikit-learn", "matplotlib"]:
            python_packages.append(package)
        elif package == "git":
            system_packages.append("git")
        elif package == "curl":
            system_packages.append("curl")
    
    # Install system packages
    if system_packages:
        commands.append(f"sudo apt-get install -y {' '.join(set(system_packages))}")
    
    # Install Python packages
    if python_packages:
        commands.append(f"pip3 install {' '.join(python_packages)}")
    
    return commands

# Helper function: Generate computation script
def _generate_computation_script(task_description: str, task_analysis: Dict) -> str:
    """
    Generate Python script based on task description and analysis.
    """
    task_lower = task_description.lower()
    task_type = task_analysis.get("task_type", "general_computation")
    
    if "monte carlo" in task_lower and "pi" in task_lower:
        return """python3 -c "
import random
import time

def calculate_pi_monte_carlo(samples=1000000):
    inside_circle = 0
    print(f'Starting Monte Carlo simulation with {samples:,} samples...')
    start_time = time.time()
    
    for i in range(samples):
        x = random.uniform(-1, 1)
        y = random.uniform(-1, 1)
        if x*x + y*y <= 1:
            inside_circle += 1
        
        if (i + 1) % 100000 == 0:
            current_pi = (inside_circle / (i + 1)) * 4
            print(f'Progress: {i+1:,}/{samples:,} samples, Current Pi estimate: {current_pi:.6f}')
    
    pi_estimate = (inside_circle / samples) * 4
    end_time = time.time()
    
    print(f'\\nFinal Results:')
    print(f'Samples used: {samples:,}')
    print(f'Points inside circle: {inside_circle:,}')
    print(f'Pi estimate: {pi_estimate:.10f}')
    print(f'Actual Pi: 3.1415926536')
    print(f'Error: {abs(pi_estimate - 3.1415926536):.10f}')
    print(f'Execution time: {end_time - start_time:.2f} seconds')
    
    return pi_estimate

result = calculate_pi_monte_carlo()
print(f'\\nCOMPUTATION COMPLETED: Pi = {result}')
"
"""
    
    elif "factorial" in task_lower:
        return """python3 -c "
import math
import sys

def calculate_factorial(n):
    if n < 0:
        return 'Factorial is not defined for negative numbers'
    elif n == 0 or n == 1:
        return 1
    else:
        result = math.factorial(n)
        return result

# Extract number from task description or use default
numbers = [int(s) for s in '{task_description}'.split() if s.isdigit()]
n = numbers[0] if numbers else 10

print(f'Calculating factorial of {n}...')
result = calculate_factorial(n)
print(f'Result: {n}! = {result}')
print(f'COMPUTATION COMPLETED: {n}! = {result}')
".format(task_description=task_description)
"""
    
    elif "fibonacci" in task_lower:
        return """python3 -c "
def fibonacci_sequence(n):
    if n <= 0:
        return []
    elif n == 1:
        return [0]
    elif n == 2:
        return [0, 1]
    
    fib_seq = [0, 1]
    for i in range(2, n):
        fib_seq.append(fib_seq[i-1] + fib_seq[i-2])
    
    return fib_seq

# Extract number from task description or use default
numbers = [int(s) for s in '{task_description}'.split() if s.isdigit()]
n = numbers[0] if numbers else 10

print(f'Calculating first {n} Fibonacci numbers...')
result = fibonacci_sequence(n)
print(f'Fibonacci sequence: {result}')
print(f'COMPUTATION COMPLETED: First {n} Fibonacci numbers calculated')
".format(task_description=task_description)
"""
    
    else:
        # Generic computation script
        return """python3 -c "
print('=== COMPUTATION TASK EXECUTION ===')
print('Task: {task_description}')
print('Starting computation...')

# Basic computation example
import time
import math

start_time = time.time()

# Perform a simple computational task as example
result = sum(i**2 for i in range(1000000))
mathematical_result = math.sqrt(result)

end_time = time.time()

print(f'Sample computation completed:')
print(f'Sum of squares (1 to 1,000,000): {result}')
print(f'Square root of result: {mathematical_result:.6f}')
print(f'Execution time: {end_time - start_time:.2f} seconds')
print('COMPUTATION COMPLETED')
".format(task_description=task_description)
"""

# Helper function: Cleanup compute resources
def _cleanup_compute_resources(namespace: str, delete_namespace: bool = False) -> Dict:
    """
    Clean up compute resources created for computation task.
    
    Args:
        namespace: Namespace containing the compute resources
        delete_namespace: Whether to delete the namespace itself (only for temporary namespaces)
    
    Returns:
        Cleanup result with details of what was cleaned up
    """
    cleanup_result = {
        "namespace": namespace,
        "namespace_type": "temporary" if delete_namespace else "persistent",
        "mci_deleted": False,
        "resources_released": False,
        "namespace_deleted": False,
        "status": "starting"
    }
    
    try:
        # Get list of MCIs in namespace
        mci_list = get_mci_list(namespace)
        if "mci" in mci_list:
            for mci in mci_list["mci"]:
                mci_id = mci.get("id")
                if mci_id and "compute" in mci_id.lower():  # Only delete compute-related MCIs
                    try:
                        delete_result = delete_mci(namespace, mci_id)
                        cleanup_result["mci_deleted"] = True
                    except:
                        pass
        
        # Release shared resources only if deleting namespace
        if delete_namespace:
            try:
                release_result = release_resources(namespace)
                cleanup_result["resources_released"] = True
            except:
                pass
            
            # Delete namespace only if it was temporary
            try:
                ns_delete_result = delete_namespace(namespace)
                cleanup_result["namespace_deleted"] = True
            except:
                pass
        
        cleanup_result["status"] = "completed"
        return cleanup_result
        
    except Exception as e:
        cleanup_result["status"] = "failed"
        cleanup_result["error"] = str(e)
        return cleanup_result

# Helper function: Generate execution summary
def _generate_compute_execution_summary(workflow_result: Dict) -> Dict:
    """
    Generate summary of compute task execution.
    """
    start_time = workflow_result.get("start_time")
    end_time = workflow_result.get("end_time")
    
    # Calculate duration if both times available
    duration = "unknown"
    if start_time and end_time:
        try:
            start_dt = datetime.fromisoformat(start_time.replace('Z', '+00:00'))
            end_dt = datetime.fromisoformat(end_time.replace('Z', '+00:00'))
            duration_seconds = (end_dt - start_dt).total_seconds()
            duration = f"{duration_seconds:.1f} seconds"
        except:
            pass
    
    return {
        "workflow_id": workflow_result.get("workflow_id"),
        "task_completed": workflow_result.get("status") == "completed",
        "total_duration": duration,
        "infrastructure_provisioned": workflow_result.get("infrastructure_created", {}).get("status") == "success",
        "computation_executed": workflow_result.get("computation_results", {}).get("status") == "success",
        "resources_cleaned": workflow_result.get("cleanup_status", {}).get("status") == "completed",
        "task_type": workflow_result.get("task_analysis", {}).get("task_type", "unknown")
    }

# Tool: Quick computational tasks with predefined scripts
@mcp.tool()
def quick_compute(
    task_type: str,
    parameters: Optional[Dict] = None,
    cleanup: bool = True,
    namespace_id: Optional[str] = None,
    use_temporary_namespace: bool = False
) -> Dict:
    """
    Execute predefined computational tasks quickly without detailed analysis.
    
    Args:
        task_type: Type of computation ("pi_monte_carlo", "factorial", "fibonacci", "matrix_multiply")
        parameters: Task-specific parameters (e.g., {"samples": 1000000, "number": 10})
        cleanup: Whether to cleanup compute resources after computation
        namespace_id: Specific namespace to use (default: "default")
        use_temporary_namespace: Whether to create temporary namespace for isolation
    
    Returns:
        Computation results and execution summary
    """
    predefined_tasks = {
        "pi_monte_carlo": "Calculate pi using Monte Carlo method with {} samples",
        "factorial": "Calculate factorial of {}",
        "fibonacci": "Generate first {} Fibonacci numbers",
        "matrix_multiply": "Perform matrix multiplication of {}x{} matrices"
    }
    
    if task_type not in predefined_tasks:
        return {"error": f"Unknown task type: {task_type}. Available: {list(predefined_tasks.keys())}"}
    
    # Set default parameters
    params = parameters or {}
    if task_type == "pi_monte_carlo":
        samples = params.get("samples", 1000000)
        task_description = predefined_tasks[task_type].format(samples)
        requirements = {"cpu_intensive": True, "required_software": ["python"]}
    elif task_type == "factorial":
        number = params.get("number", 10)
        task_description = predefined_tasks[task_type].format(number)
        requirements = {"cpu_intensive": False, "required_software": ["python"]}
    elif task_type == "fibonacci":
        count = params.get("count", 20)
        task_description = predefined_tasks[task_type].format(count)
        requirements = {"cpu_intensive": False, "required_software": ["python"]}
    elif task_type == "matrix_multiply":
        size = params.get("size", 100)
        task_description = predefined_tasks[task_type].format(size, size)
        requirements = {"cpu_intensive": True, "memory_intensive": True, "required_software": ["python", "numpy"]}
    
    return execute_compute_task(
        task_description=task_description,
        computation_requirements=requirements,
        cleanup_after_completion=cleanup,
        namespace_id=namespace_id,
        create_temporary_namespace=use_temporary_namespace
    )


# ===== MCP Server Prompts =====
# These prompts help users understand and effectively use the TB-MCP server capabilities
# Based on MapUI patterns for comprehensive cloud infrastructure management

@mcp.prompt()
def mci_creation_workflow():
    """
    Complete MCI Creation Workflow Guide with LLM Interaction Patterns
    
    This comprehensive prompt enforces the correct workflow for creating Multi-Cloud Infrastructure (MCI)
    and provides detailed LLM behavior guidelines for proper user interaction.
    ALL MCI creation MUST follow this workflow to prevent deployment failures.
    VM configurations MUST include both specId and imageId - no exceptions.
    """
    return """# 🚨 COMPLETE MCI Creation Workflow & LLM Interaction Guide

## ⚠️ CRITICAL: ALWAYS Follow This Exact Workflow

This comprehensive guide covers both the technical workflow and LLM behavior patterns
for creating Multi-Cloud Infrastructure (MCI) with proper user interaction.

### 🔄 STEP-BY-STEP PROCESS (NO SHORTCUTS ALLOWED):

⚠️ **IMPORTANT**: VM configurations MUST include both `specId` AND `imageId` - no exceptions!

📋 **OS TYPE SPECIFICATION OPTIONS:**
- Simple OS name: `"ubuntu"`, `"centos"`, `"windows"`, `"debian"`, `"rhel"`
- OS with specific version: `"ubuntu 22.04"`, `"centos 7"`, `"windows server 2019"`, `"debian 11"`
- Use `get_image_search_options()` to see all available osType values

#### STEP 0: 🔧 GET AVAILABLE OPTIONS (MANDATORY FIRST STEP)
```python
# 0.1: ALWAYS get available options before recommend_vm_spec
spec_options = get_recommend_spec_options()
# This returns:
# - Available filter metrics (vCPU, memoryGiB, providerName, regionName, etc.)
# - Valid values for providers, regions, architectures
# - Example policies and parameter formats
# - Priority options and their required parameters

# 0.2: Use the returned information to build valid filter_policies
# ✅ Check availableMetrics for valid metric names
# ✅ Check availableValues for valid provider/region names
# ✅ Use examplePolicies as templates
# ✅ Verify priority options and parameter formats
```

#### STEP 1: 🔍 MANDATORY REVIEW PHASE
```python
# 1.1: Get VM specifications (using validated parameters from step 0)
specs = recommend_vm_spec(
    filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},  # Use validated metric names
    priority_policy="location",  # or "cost" or "performance" (check spec_options for available)
    latitude=37.4419,           # if location-based
    longitude=-122.1430         # if location-based
)

# 1.2: Search and select images for each spec
vm_configurations = []
for i, spec in enumerate(specs["recommended_specs"][:2]):
    spec_id = spec["id"]  # e.g., "aws+ap-northeast-2+t2.small"
    
    # 1.2.1: Search for compatible images using matched_spec_id
    images = search_images(
        matched_spec_id=spec_id,     # Auto-applies provider/region/arch
        os_type="ubuntu 22.04",      # Can be simple OS ("ubuntu") or OS+version ("ubuntu 22.04", "centos 7", "windows server 2019")
        include_basic_image_only=True  # Prefer clean OS installations
    )
    
    # 1.2.2: Select best image for this specific spec
    if images.get("imageList"):
        best_image = select_best_image_for_spec(
            images["imageList"], 
            spec, 
            {"os_type": "ubuntu 22.04", "prefer_basic": True}  # Can specify exact OS version
        )
        selected_image_id = best_image["cspImageName"]
    else:
        # Fallback: try without basic_image_only filter
        images = search_images(matched_spec_id=spec_id, os_type="ubuntu 22.04")
        if images.get("imageList"):
            selected_image_id = images["imageList"][0]["cspImageName"]
        else:
            raise Exception(f"No compatible images found for spec {spec_id}")
    
    # 1.2.3: Create VM configuration with required imageId
    vm_configurations.append({
        "specId": spec_id,                    # MUST use exact ID from API
        "imageId": selected_image_id,         # 🚨 REQUIRED - CSP-specific image ID
        "name": f"vm-{spec['providerName']}-{i+1}",
        "subGroupSize": "1"
    })

# 1.3: 🔍 MANDATORY REVIEW - Always do this first!
review_result = review_mci_dynamic_request(
    ns_id="default",
    name="my-infrastructure",
    vm_configurations=vm_configurations
)
```

#### STEP 2: 📋 ANALYZE REVIEW RESULTS
```python
# 2.1: Check overall status
if review_result.get("overallStatus") == "Ready":
    print("✅ Configuration validated - Safe to proceed")
    creation_viable = True
elif review_result.get("overallStatus") == "Warning":
    print("⚠️ Warnings detected - Review before proceeding")
    # Check vmReviews for specific warnings
    creation_viable = True  # Can proceed with caution
elif review_result.get("overallStatus") == "Error":
    print("❌ Errors detected - Must fix before proceeding") 
    creation_viable = False
    # Fix issues in vm_configurations and re-run review

# 2.2: Review cost estimates
print(f"💰 Estimated cost: {review_result.get('estimatedCost')}")
print(f"🖥️ Total VMs: {review_result.get('totalVmCount')}")
```

#### STEP 3: 🚀 MCI CREATION (Only After Successful Review)
```python
# 3.1: Create MCI only if review passed
if creation_viable:
    mci_result = create_mci_dynamic(
        ns_id="default",
        name="my-infrastructure",
        vm_configurations=vm_configurations,  # Already includes specId + imageId
        force_create=True  # 🚨 REQUIRED after review
    )
    print(f"✅ MCI created: {mci_result.get('id')}")
else:
    print("❌ Cannot create MCI - fix validation issues first")
```

## 🚫 FORBIDDEN PATTERNS:

### ❌ NEVER Do This:
```python
# DON'T: Skip review step
create_mci_dynamic(ns_id="default", name="test", vm_configurations=[...])
# This will return an error requiring review first

# DON'T: Create spec IDs manually
vm_config = {"specId": "aws+us-east-1+t2.small"}  # Manual creation
# Always use recommend_vm_spec() results

# DON'T: Skip image selection step
vm_config = {
    "specId": spec["id"],
    # Missing imageId - THIS WILL FAIL
    "name": "my-vm"
}

# DON'T: Use imageId from different CSP/region than specId
vm_config = {
    "specId": "aws+us-east-1+t2.small",
    "imageId": "ami-azure-image-id"  # Mismatched CSP - WILL FAIL
}

# DON'T: Use vague OS specifications when specific versions are needed
vm_config = {
    "specId": spec["id"],
    "imageId": "generic-ubuntu-image"  # Be specific about OS version
}
# DO: Use specific OS versions like "ubuntu 22.04" for better image matching

# DON'T: Ignore review results
review = review_mci_dynamic_request(...)
create_mci_dynamic(..., force_create=True)  # Without checking review
```

### ✅ ALWAYS Do This:
```python
# DO: Follow the complete three-step process
specs = recommend_vm_spec(...)
vm_configs = []
for spec in specs["recommended_specs"]:
    # REQUIRED: Search for compatible images with specific OS version
    images = search_images(matched_spec_id=spec["id"], os_type="ubuntu 22.04")  # Specify exact OS+version
    selected_image = images["imageList"][0]["cspImageName"]
    
    vm_configs.append({
        "specId": spec["id"],
        "imageId": selected_image,  # 🚨 REQUIRED
        "name": f"vm-{spec['providerName']}-1"
    })

review = review_mci_dynamic_request(ns_id="default", name="test", vm_configurations=vm_configs)
if review["overallStatus"] == "Ready":
    create_mci_dynamic(ns_id="default", name="test", vm_configurations=vm_configs, force_create=True)

# DO: Always include imageId in VM configurations
vm_config = {
    "specId": spec["id"],      # From recommend_vm_spec()
    "imageId": image["cspImageName"],  # From search_images() - REQUIRED
    "name": "my-vm"
}
```

## 📊 UNDERSTANDING REVIEW RESULTS:

### Key Fields to Check:
- `overallStatus`: "Ready" | "Warning" | "Error"
- `creationViable`: Boolean - can create MCI?
- `estimatedCost`: Cost per hour (e.g., "$0.0837/hour")
- `totalVmCount`: Total VMs including SubGroup sizes
- `vmReviews`: Per-VM validation details

### Decision Matrix:
- **"Ready" + creationViable: true** → ✅ Proceed with creation
- **"Warning" + creationViable: true** → ⚠️ Review warnings, then proceed
- **"Error" + creationViable: false** → ❌ Fix errors, re-run review

## 🎯 LLM IMPLEMENTATION GUIDELINES:

1. **NEVER skip the review step** - Always call review_mci_dynamic_request() first
2. **NEVER omit imageId** - VM configurations MUST include both specId and imageId
3. **ALWAYS use matched_spec_id** in search_images() for compatibility
4. **SPECIFY OS version when needed** - Use os_type with versions like "ubuntu 22.04", "centos 7", "windows server 2019"
5. **ALWAYS check overallStatus** before proceeding to creation
6. **ALWAYS use force_create=True** in create_mci_dynamic() after review
7. **EXPLAIN the complete process** to users when they request MCI creation
8. **SHOW cost estimates** from review results before creation
9. **FIX validation issues** if overallStatus is "Error" before creating

## 🤖 CRITICAL LLM USER INTERACTION RULES:

### ✅ REQUIRED USER INTERACTION PATTERN:
```
User: "Create an MCI with 2 VMs"

LLM Response:
"I'll help you create an MCI with 2 VMs. Let me first validate the configuration and show you the cost estimates before proceeding.

[Calls review_mci_dynamic_request()]

✅ Configuration validated! Here's your deployment plan:
💰 Cost: $0.16/hour (~$115.20/month)
🖥️ VMs: 2 virtual machines (AWS us-east-1, GCP us-central1)

Would you like me to proceed with creating this infrastructure?
- Reply 'Yes' to create the MCI
- Reply 'No' to cancel
- Reply 'Details' for more information"

[Waits for user confirmation]

User: "Yes"

LLM: [Calls create_mci_dynamic(force_create=True)]
"✅ MCI 'user-mci-12345' created successfully!"
```

### ❌ FORBIDDEN USER INTERACTION PATTERNS:
```
User: "Create an MCI"

❌ WRONG LLM Response:
"Creating MCI now..."
[Calls create_mci_dynamic() directly]  // Will fail with error!

❌ WRONG LLM Response:
"MCI created successfully!"
[No review, no cost disclosure, no user confirmation]
```

### 📊 MANDATORY USER INFORMATION DISCLOSURE:
**Before ANY MCI creation, LLM MUST show user:**
1. 💰 **Cost estimates** (hourly and monthly)
2. 🖥️ **VM specifications** (count, providers, regions)  
3. ⚠️ **Any warnings** or potential issues
4. ✅ **Explicit confirmation request**

**Never create infrastructure without user seeing costs first!**

## 📋 REQUIRED VM CONFIGURATION FORMAT:

```python
vm_configuration = {
    "specId": "aws+ap-northeast-2+t2.small",      # From recommend_vm_spec() - REQUIRED
    "imageId": "ami-0c02fb55956c7d316",           # From search_images() - REQUIRED
    "name": "my-vm-1",                            # VM name - REQUIRED
    "subGroupSize": "1"                           # Number of VMs - REQUIRED
}
```

This workflow prevents expensive deployment failures and ensures reliable infrastructure creation.

## 🤖 DETAILED LLM BEHAVIOR GUIDE FOR MCI CREATION

### 🎯 MANDATORY LLM BEHAVIOR WHEN USER REQUESTS MCI CREATION:

#### STEP 1: 🚨 ALWAYS START WITH REVIEW (NEVER SKIP)
```python
# ✅ CORRECT: Always call review first
review_result = review_mci_dynamic_request(
    ns_id="default",
    name="user-requested-mci",
    vm_configurations=vm_configurations
)

# ❌ WRONG: Never call create_mci_dynamic directly
# create_mci_dynamic(...)  # This will return an error!
```

#### STEP 2: 📊 ANALYZE REVIEW RESULTS AND INFORM USER
Based on `review_result._llm_guidance.status`:

**🟢 "READY_TO_CREATE":**
```python
# Display the user_prompt from _llm_guidance directly to user
user_message = review_result["_llm_guidance"]["user_prompt"]
print(user_message)  # Shows cost, specs, asks for confirmation

# Wait for user response: "Yes", "No", or "Details"
user_response = get_user_input()

if user_response.lower() in ["yes", "y", "proceed", "create"]:
    # User approved - proceed with creation
    mci = create_mci_dynamic(..., force_create=True)
    print(f"✅ MCI '{mci['id']}' created successfully!")
elif user_response.lower() in ["no", "n", "cancel"]:
    print("❌ MCI creation cancelled by user.")
else:
    # Show detailed information
    print("📋 Detailed validation results:", review_result["vmReviews"])
```

**🟡 "READY_WITH_WARNINGS":**
```python
# Show warnings and ask user to decide
warnings_message = review_result["_llm_guidance"]["user_prompt"]
print(warnings_message)  # Displays warnings and asks for decision

user_response = get_user_input()
if user_response.lower() in ["yes", "proceed"]:
    mci = create_mci_dynamic(..., force_create=True)
    print("⚠️ MCI created with warnings noted.")
else:
    print("Configuration needs adjustment. Let me help fix the warnings.")
    # Guide user through fixing warnings
```

**🔴 "CANNOT_CREATE":**
```python
# Show errors and offer help
error_message = review_result["_llm_guidance"]["user_prompt"]
print(error_message)  # Explains errors and offers help options

# DO NOT proceed to create_mci_dynamic
# Instead, help user fix the issues:

print("Let me help you fix these issues:")
print("1. Finding alternative VM specifications...")
alternative_specs = recommend_vm_spec(different_filter_policies)
print("2. Checking image availability in other regions...")
# Guide user through problem resolution
```

### 🎯 REQUIRED LLM RESPONSES TO USERS:

#### ✅ When Review Shows "Ready":
```
"Great! I've validated your MCI configuration and everything looks good:

💰 Cost: $X.XX/hour (approximately $Y.YY/month)  
🖥️ VMs: N virtual machines across M providers
✅ Status: All configurations validated

The infrastructure will be created with these specifications:
[Display VM summary from review]

Would you like me to proceed with creating this MCI infrastructure?
- Reply 'Yes' to create the infrastructure
- Reply 'No' to cancel  
- Reply 'Details' for more information"
```

#### ⚠️ When Review Shows Warnings:
```
"I've reviewed your MCI configuration. It can be created but there are some warnings:

⚠️ Warnings Found:
[Display warnings from review]

💰 Cost: $X.XX/hour  
🖥️ VMs: N virtual machines

Despite these warnings, the infrastructure can still be created. 
Would you like to proceed anyway, or should I help you address the warnings first?
- Reply 'Proceed' to create with warnings
- Reply 'Fix' to address warnings first  
- Reply 'Details' for more information"
```

#### ❌ When Review Shows Errors:
```
"I've reviewed your MCI configuration and found issues that must be fixed:

❌ Errors Found:
[Display errors from review]

I cannot create the infrastructure with these errors. Let me help you fix them:

Would you like me to:
1. Find alternative VM specifications  
2. Search for images in different regions
3. Check different cloud providers
4. Show detailed error information

Which option would you prefer? (Reply with the number or description)"
```

### 🚨 CRITICAL LLM RULES:

1. **NEVER** call `create_mci_dynamic()` without calling `review_mci_dynamic_request()` first
2. **ALWAYS** show the user the cost estimates and configuration details before creating
3. **ALWAYS** wait for explicit user confirmation before proceeding to creation
4. **NEVER** assume user consent - always ask for confirmation
5. **ALWAYS** use the `_llm_guidance.user_prompt` field for user messages
6. **NEVER** proceed with creation if `creationViable: false`
7. **ALWAYS** help users fix errors instead of just reporting them

### 📊 LLM RESPONSE TEMPLATES:

**User Request:** "Create an MCI with 2 VMs in AWS"

**LLM Response Pattern:**
```
"I'll help you create an MCI with 2 VMs in AWS. Let me first validate the configuration and check costs.

[Calls review_mci_dynamic_request()]

[Based on review results, uses appropriate response template above]
```

**NEVER:**
```
"I'll create an MCI for you."
[Calls create_mci_dynamic() directly]  // ❌ WRONG!
```

This ensures users are always informed about costs and configuration before infrastructure is created.
"""

@mcp.prompt()
def tumblebug_application_deployment():
    """
    Application Deployment Strategy and Workflow Guide
    
    This prompt guides LLMs to use different approaches based on application type:
    - Known applications (APPLICATION_CONFIGS): Use existing usecase commands
    - General applications: LLM generates deployment commands based on reference guides
    """
    return """# CB-Tumblebug Application Deployment Guide

## 🚀 DEPLOYMENT STRATEGY: Intelligent Application-Specific Approach

### 🎯 LLM DECISION MATRIX: Choose Deployment Method Based on Application Type

**🤖 LLM MUST first determine the application type and use appropriate strategy:**

#### 📋 STEP 1: Application Type Detection
```python
# 1. Check if application is in predefined APPLICATION_CONFIGS
list_application_templates()  # Get available predefined applications

# 2. If found in APPLICATION_CONFIGS → Use Strategy A (Usecase Commands)
# 3. If NOT found → Use Strategy B (LLM-Generated Commands)
```

### 🎯 STRATEGY A: Predefined Applications (APPLICATION_CONFIGS)

**📋 For applications in APPLICATION_CONFIGS (xonotic, nginx, ollama, jitsi, elk, ray):**

#### ✅ USE EXISTING USECASE COMMANDS - DO NOT REINVENT
```python
# EXAMPLE: Xonotic Game Server
execute_command_mci(ns_id, mci_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/xonotic/startServer.sh; chmod +x ~/startServer.sh",
    "sudo ~/startServer.sh {{mci_id}} 26000 8 8",
    "echo 'Server Address: {{public_ip}}:26000'"
])

# EXAMPLE: Nginx Web Server  
execute_command_mci(ns_id, mci_id, [
    "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/nginx/startServer.sh | bash -s -- --ip {{public_ip}}",
    "echo 'Web Server: http://{{public_ip}}'"
])

# EXAMPLE: Ollama LLM Server (Basic Setup)
execute_command_mci(ns_id, mci_id, [
    "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh",
    "echo 'LLM Server: http://{{public_ip}}:3000'"
])

# 🚀 SPECIAL CASE: Ollama Model Deployment with Dynamic Selection
# For ollama_pull requests, use NEW ENHANCED WORKFLOW:

# Step 1: Guide user to discover latest models
discovery_guide = get_ollama_model_discovery_guide(
    use_case="coding",  # or "chat", "research", etc.
    category="code"     # optional category filter
)

# Step 2: Instruct user to visit ollama.com/search
print("🔍 Please visit https://ollama.com/search to find latest models")
print("📋 Browse categories and select models you need")
print("✅ Note down exact model names for deployment")

# Step 3: Deploy with user-selected models
result = deploy_ollama_pull_with_models(
    ns_id="namespace",
    mci_name="ollama-cluster",
    selected_models=["llama3.3:latest", "deepseek-r1:latest", "qwen2.5:14b"],  # User-selected
    description="Custom LLM deployment with latest models"
)

# EXAMPLE: Jitsi Meet Conference
execute_command_mci(ns_id, mci_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/jitsi/startServer.sh",
    "chmod +x ~/startServer.sh",
    "sudo ~/startServer.sh {{public_ips_space}} DNS EMAIL"
])

# EXAMPLE: ELK Stack
execute_command_mci(ns_id, mci_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/elastic-stack/startELK.sh",
    "chmod +x ~/startELK.sh",
    "sudo ~/startELK.sh"
])

# EXAMPLE: Ray ML Cluster
execute_command_mci(ns_id, mci_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/ray/ray-head-setup.sh",
    "chmod +x ~/ray-head-setup.sh",
    "~/ray-head-setup.sh -i {{public_ip}}"
])
```

**🚨 CRITICAL for Strategy A:**
- **ALWAYS use the exact usecase commands from APPLICATION_CONFIGS**
- **DO NOT modify or recreate these scripts**
- **These are tested, optimized deployment scripts**
- **Follow the exact command patterns shown above**

### 🎯 STRATEGY B: General Applications (LLM-Generated Commands)

**🧠 For applications NOT in APPLICATION_CONFIGS:**

#### ✅ LLM MUST GENERATE SMART DEPLOYMENT COMMANDS
```python
# 1. Get reference guide (for hardware requirements only)
guide = get_application_deployment_guide("application_name", "production")

# 2. Use guide ONLY for hardware specs, NOT for exact commands
vm_spec_filter = guide["vm_spec_filter"]  # Use this for VM specifications
installation_commands = guide["commands"]["installation"]  # IGNORE - Generate your own

# 3. LLM generates intelligent deployment commands based on application knowledge
```

**🤖 LLM Command Generation Examples:**

```python
# Example 1: MongoDB deployment (LLM-generated)
execute_command_mci(ns_id, mci_id, [
    "sudo apt-get update -y",
    "wget -qO - https://www.mongodb.org/static/pgp/server-6.0.asc | sudo apt-key add -",
    "echo 'deb [ arch=amd64,arm64 ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/6.0 multiverse' | sudo tee /etc/apt/sources.list.d/mongodb-org-6.0.list",
    "sudo apt-get update -y",
    "sudo apt-get install -y mongodb-org",
    "sudo systemctl start mongod",
    "sudo systemctl enable mongod",
    "sudo systemctl status mongod",
    "echo 'MongoDB Server: {{public_ip}}:27017'"
])

# Example 2: Jenkins CI/CD (LLM-generated)
execute_command_mci(ns_id, mci_id, [
    "sudo apt-get update -y",
    "sudo apt-get install -y openjdk-11-jdk",
    "wget -q -O - https://pkg.jenkins.io/debian/jenkins.io.key | sudo apt-key add -",
    "sudo sh -c 'echo deb http://pkg.jenkins.io/debian-stable binary/ > /etc/apt/sources.list.d/jenkins.list'",
    "sudo apt-get update -y",
    "sudo apt-get install -y jenkins",
    "sudo systemctl start jenkins",
    "sudo systemctl enable jenkins",
    "sudo cat /var/lib/jenkins/secrets/initialAdminPassword",
    "echo 'Jenkins Server: http://{{public_ip}}:8080'"
])

# Example 3: Redis Cache (LLM-generated)
execute_command_mci(ns_id, mci_id, [
    "sudo apt-get update -y",
    "sudo apt-get install -y redis-server",
    "sudo sed -i 's/bind 127.0.0.1/bind 0.0.0.0/' /etc/redis/redis.conf",
    "sudo systemctl restart redis-server",
    "sudo systemctl enable redis-server",
    "redis-cli ping",
    "echo 'Redis Server: {{public_ip}}:6379'"
])

# Example 4: Apache Kafka (LLM-generated)
execute_command_mci(ns_id, mci_id, [
    "sudo apt-get update -y",
    "sudo apt-get install -y openjdk-11-jdk",
    "wget https://downloads.apache.org/kafka/2.8.2/kafka_2.13-2.8.2.tgz",
    "tar -xzf kafka_2.13-2.8.2.tgz",
    "cd kafka_2.13-2.8.2",
    "bin/zookeeper-server-start.sh config/zookeeper.properties &",
    "sleep 10",
    "bin/kafka-server-start.sh config/server.properties &",
    "echo 'Kafka Server: {{public_ip}}:9092'"
])
```

**🧠 LLM Command Generation Guidelines:**
1. **Think about the application's typical installation process**
2. **Consider package managers (apt, yum, snap, docker)**
3. **Include service startup and enablement**
4. **Add configuration for network access if needed**
5. **Include verification commands**
6. **Provide clear access information**
7. **Use your knowledge of the application's standard deployment**

### 🔄 UNIFIED 9-STEP DEPLOYMENT WORKFLOW

**🚨 REGARDLESS of Strategy A or B, ALWAYS follow this workflow:**

#### Step 1: 📖 Application Type Detection
```python
# Check if predefined application exists
templates = list_application_templates()

# If not predefined, get reference guide for hardware specs only
guide = get_application_deployment_guide("application_name", "production")
```

#### Step 2: 🏗️ Prepare Namespace
```python
check_and_prepare_namespace("my-app-namespace")
```

#### Step 3: 🎯 Get VM Specifications
```python
# Use hardware requirements from APPLICATION_CONFIGS or deployment guide
specs = recommend_vm_spec(
    filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
    priority_policy="cost"
)
```

#### Step 4: 🔧 Build VM Configurations
```python
vm_configurations = [{
    "specId": spec["id"],
    "name": f"vm-{app_name}-1",
    "description": f"VM for {app_name}",
    "subGroupSize": "1"
}]
```

#### Step 5: ✅ Validate Configuration (MANDATORY)
```python
review = review_mci_dynamic_request(ns_id, name, vm_configurations)
```

#### Step 6: 🚀 Create Infrastructure
```python
mci = create_mci_dynamic(ns_id, name, vm_configurations, force_create=True)
```

#### Step 7: 📦 Install Application
```python
# Strategy A: Use predefined APPLICATION_CONFIGS commands
# Strategy B: Use LLM-generated intelligent commands
execute_command_mci(ns_id, mci_id, deployment_commands)
```

#### Step 8: 🔍 Verify Deployment
```python
# Generate appropriate verification commands
execute_command_mci(ns_id, mci_id, verification_commands)
```

#### Step 9: 📋 Collect Access Information
```python
access_info = get_mci_access_info(ns_id, mci_id)
```

### 🎯 LLM BEHAVIOR REQUIREMENTS

**🚨 CRITICAL LLM Decision Rules:**
1. **ALWAYS check list_application_templates() first**
2. **IF application in templates → Use Strategy A (exact usecase commands)**
3. **IF application NOT in templates → Use Strategy B (LLM-generated commands)**
4. **Use get_application_deployment_guide() as REFERENCE ONLY for Strategy B**
5. **NEVER skip validation step**
6. **ALWAYS complete all 9 steps**

### 📋 Example Decision Process:

```
User: "Deploy xonotic game server"
LLM Decision: 
1. Check templates → xonotic found in APPLICATION_CONFIGS
2. Use Strategy A → Execute exact usecase commands
3. Commands: wget startServer.sh; sudo ~/startServer.sh {{mci_id}} 26000 8 8

User: "Deploy MongoDB database"  
LLM Decision:
1. Check templates → mongodb NOT in APPLICATION_CONFIGS
2. Use Strategy B → Generate intelligent deployment commands
3. Commands: LLM creates MongoDB installation script (as shown above)

User: "Deploy nginx web server"
LLM Decision:
1. Check templates → nginx found in APPLICATION_CONFIGS  
2. Use Strategy A → Use exact nginx usecase script
3. Commands: curl startServer.sh | bash -s -- --ip {{public_ip}}
```

### 🚨 CRITICAL: Remote Command Execution Time Warnings

**⏰ IMPORTANT PERFORMANCE CONSIDERATIONS:**

Remote command execution via CB-Tumblebug API can take **significantly longer** than expected:

#### ⏱️ **Expected Response Times:**
- **Simple commands** (ls, ps, whoami): 10-30 seconds
- **Package updates** (apt update): 1-3 minutes  
- **Software installation** (apt install nginx): 2-5 minutes
- **Application deployment scripts**: 5-15 minutes
- **Complex setups** (databases, Docker, clusters): 10-20 minutes
- **Large downloads/compilations**: **Up to 20+ minutes**

#### 🚨 **LLM MUST INFORM USERS:**

**Before executing remote commands, ALWAYS tell users:**

```
⏰ IMPORTANT: Remote command execution may take several minutes to complete.
   - Simple installations: 2-5 minutes
   - Complex applications: 10-20 minutes
   - Please be patient during the deployment process.
   
🔄 The system will provide progress updates when commands complete.
```

#### 🎯 **Best Practices for LLMs:**

1. **🗣️ Set Expectations:** Always warn users about potential delays
2. **📦 Batch Commands:** Group related commands to minimize API calls
3. **🔍 Use Verification:** Add simple verification commands to check progress
4. **📊 Enable Summarization:** Use `summarize_output=True` for large outputs
5. **⚡ Break Down Complex:** Split large deployments into smaller batches

#### 📋 **Example User Communication:**

```python
# ✅ GOOD: Inform user before execution
print("⏰ Starting deployment - this may take 5-10 minutes...")
print("📦 Installing application packages and dependencies...")

result = execute_command_mci(ns_id, mci_id, installation_commands)

print("✅ Deployment completed! Checking service status...")
```

```python
# ❌ BAD: No warning about timing
result = execute_command_mci(ns_id, mci_id, installation_commands)
# User may think system is frozen
```

### 🎯 SUCCESS METRICS

**✅ Successful deployment must:**
1. Follow correct strategy (A or B) based on application type
2. Complete all 9 workflow steps
3. Pass MCI configuration validation
4. Execute appropriate installation commands
5. Verify deployment success
6. Provide clear access information

**🚨 This intelligent approach ensures:**
- **Predefined apps**: Use tested, optimized scripts
- **General apps**: LLM creativity with reliable workflow
- **All apps**: Consistent validation and verification

### 📚 DEPLOYMENT GUIDES: Reference Information Only

**🔍 About get_application_deployment_guide():**

This tool provides **REFERENCE INFORMATION ONLY** for Strategy B (general applications):

```python
# ✅ Correct usage for Strategy B
guide = get_application_deployment_guide("mongodb", "production")
vm_spec_filter = guide["vm_spec_filter"]  # Use for VM specifications
# installation_commands = guide["commands"]["installation"]  # IGNORE - Generate your own

# ❌ Wrong usage
# Do NOT copy commands directly from deployment guides for general apps
# Do NOT use deployment guides for APPLICATION_CONFIGS apps
```

**📋 Deployment Guide Contents:**
- **Hardware Requirements**: CPU, memory, disk specifications (USE THIS)
- **VM Spec Filters**: Filter policies for recommend_vm_spec() (USE THIS)
- **Installation Commands**: Generic examples (REFERENCE ONLY - Don't copy)
- **Verification Commands**: Basic verification examples (REFERENCE ONLY)

**🎯 LLM Strategy for Deployment Guides:**
1. **Strategy A Apps**: Ignore deployment guides completely - use APPLICATION_CONFIGS commands
2. **Strategy B Apps**: Use hardware specs from guides, generate your own installation commands
3. **Unknown Apps**: Use generic deployment guide as starting reference only

**⚠️ IMPORTANT NOTES:**
- Deployment guides are NOT optimized, tested scripts
- They provide generic installation patterns for reference
- LLM should use domain knowledge to create better commands
- Always prefer APPLICATION_CONFIGS commands when available

## 🚨 ERROR HANDLING AND RECOVERY

### Common Error Scenarios and Solutions

#### 1. **Resource Creation Failures and Rollbacks**
```
Error: "rollback completed successfully after errors in resource preparation"
```

**🔍 Diagnosis:**
- Infrastructure provisioning failed during resource creation
- CB-Tumblebug performed automatic rollback
- Common causes: quota limits, network issues, region availability

**✅ LLM Recovery Actions:**
1. **Check Resource Availability:**
   ```python
   # Verify cloud connections and quotas
   connections = get_connections_with_options(filter_verified=True)
   
   # Try different regions or providers
   specs = recommend_vm_spec(
       filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
       priority_policy="cost"  # Try cost-optimized specs
   )
   ```

2. **Simplify Configuration:**
   ```python
   # Reduce VM count or specs
   vm_configurations = [{
       "specId": "smaller_spec_id",  # Use smaller instance
       "subGroupSize": "1"               # Start with single VM
   }]
   ```

3. **Validate Before Retry:**
   ```python
   # Always validate before retrying
   review = review_mci_dynamic_request(ns_id, name, vm_configurations)
   ```

#### 2. **Timeout Errors (10+ minute operations)**
```
Error: "Request timeout - operation took longer than 10 minutes"
```

**🔍 Diagnosis:**
- Very long MCI creation times exceed client connection timeout
- Complex multi-region deployments or large cluster operations
- Resource contention or CSP-side delays

**✅ LLM Recovery Actions:**
1. **Check MCI Status:**
   ```python
   # Check if MCI was partially created despite timeout
   mci_list = get_mci_list_with_options(ns_id, option="status")
   
   # Look for the MCI name in results
   for mci in mci_list.get("mci", []):
       if mci["name"] == "your-mci-name":
           print(f"MCI Status: {mci['status']}")
   ```

2. **Retry with Simpler Configuration:**
   ```python
   # Start with single region/provider
   vm_configurations = [{
       "specId": single_region_spec,
       "subGroupSize": "1"
   }]
   ```

3. **Use Staged Deployment:**
   ```python
   # Deploy incrementally
   # Phase 1: Core infrastructure
   create_mci_dynamic(ns_id, "core-mci", [core_vm])
   
   # Phase 2: Additional resources
   create_mci_dynamic(ns_id, "additional-mci", [additional_vms])
   ```

#### 3. **Connection Errors**
```
Error: "Connection error - unable to reach CB-Tumblebug server"
```

**✅ LLM Recovery Actions:**
1. **Verify Service Status:**
   ```python
   # Test basic connectivity
   try:
       namespaces = get_namespaces()
       print("✅ Connection restored")
   except:
       print("❌ Still cannot connect")
   ```

2. **Inform User:**
   - Explain the connection issue clearly
   - Suggest checking CB-Tumblebug server status
   - Provide retry instructions

#### 4. **Validation Failures**
```
Error: "MCI configuration validation failed"
```

**✅ LLM Recovery Actions:**
1. **Analyze Validation Results:**
   ```python
   review = review_mci_dynamic_request(ns_id, name, vm_configurations)
   
   # Check each VM's validation status
   for vm in review.get("vm_validations", []):
       if vm.get("issues"):
           print(f"VM {vm['vm_index']}: {vm['issues']}")
   ```

2. **Fix Common Issues:**
   - Use exact spec IDs from recommend_vm_spec()
   - Ensure compatible image-spec combinations
   - Verify resource quotas and limits

### 🎯 LLM Error Communication Guidelines

**✅ DO:**
- Explain errors in user-friendly terms
- Provide specific recovery steps
- Offer alternative approaches
- Show what was attempted and why it failed

**❌ DON'T:**
- Simply repeat technical error messages
- Give up after first failure
- Ignore timeout or connection issues
- Skip validation steps to "save time"

**📝 Error Response Template:**
```
I encountered a [specific error type] while [operation attempted]. 

🔍 **What happened:** [User-friendly explanation]

🛠️ **I'm trying these solutions:**
1. [First recovery action]
2. [Second recovery action]
3. [Alternative approach]

⏳ **Please wait while I resolve this...**
```

## STEP-BY-STEP APPLICATION DEPLOYMENT WORKFLOW

### Step 1: Get Application Deployment Guide
```python
# First, get detailed deployment guide for the application
guide = get_application_deployment_guide("nginx", "production")
# This provides hardware requirements, installation commands, and verification steps
```

### Step 2: Create or Validate Namespace
```python
# Ensure proper namespace exists
namespace_result = check_and_prepare_namespace("my-app")
# Or create new one: create_namespace_with_validation("my-app-production")
```

### Step 3: Get VM Specifications (Using Application Requirements)
```python
# Use hardware requirements from deployment guide
specs = recommend_vm_spec(
    filter_policies=guide["deployment_workflow"]["step_2_vm_specs"]["filter_policies"],
    priority_policy=guide["deployment_workflow"]["step_2_vm_specs"]["priority_policy"]
)
```

### Step 4: Build VM Configuration
```python
# Create VM configurations using recommended specs
vm_configurations = []
for i, spec in enumerate(specs["recommended_specs"][:2]):
    vm_configurations.append({
        "specId": spec["id"],  # Use exact spec ID from API
        "name": f"app-vm-{i+1}",
        "description": f"VM for {application_name} in {spec['regionName']}",
        "subGroupSize": "1"
        # imageId: Auto-mapped based on spec
    })
```

### Step 5: Validate MCI Configuration (MANDATORY)
```python
# Always validate before creating MCI
review_result = review_mci_dynamic_request(
    ns_id="my-app",
    name="my-app-mci",
    vm_configurations=vm_configurations
)

# Check validation results
if review_result.get("summary", {}).get("validationPassed", False):
    print("✅ Configuration validated - safe to proceed")
else:
    print("❌ Validation failed - fix issues first")
    # Handle validation errors
```

### Step 6: Create MCI Infrastructure
```python
# Create the infrastructure
mci_result = create_mci_dynamic(
    ns_id="my-app",
    name="my-app-mci",
    vm_configurations=vm_configurations,
    description="Infrastructure for my application",
    force_create=True  # Skip confirmation since we validated
)
```

### Step 7: Deploy Application Using Remote Commands
```python
# Get installation commands from deployment guide
installation_commands = guide["commands"]["installation"]

# Execute installation commands
deployment_result = execute_command_mci(
    ns_id="my-app",
    mci_id="my-app-mci",  # From mci_result
    commands=installation_commands
)
```

### Step 8: Verify Deployment
```python
# Get verification commands from deployment guide
verification_commands = guide["commands"]["verification"]

# Verify installation
verification_result = execute_command_mci(
    ns_id="my-app",
    mci_id="my-app-mci",
    commands=verification_commands
)
```

### Step 9: Collect Access Information
```python
# Get access information and endpoints
access_info = get_mci_access_info("my-app", "my-app-mci", show_ssh_key=False)

# Display access URLs using the pattern from deployment guide
access_pattern = guide["expected_results"]["access_pattern"]
# e.g., "http://{{public_ip}}" becomes "http://1.2.3.4"
```

## 🔧 SUPPORTED APPLICATIONS WITH DEPLOYMENT GUIDES

The following applications have detailed deployment guides available:

### **Web Servers**
- **nginx**: High-performance web server (recommended specs: 2 CPU, 4GB RAM)
- **apache**: Apache HTTP server (recommended specs: 2 CPU, 4GB RAM)

### **Databases**
- **mysql**: MySQL database server (recommended specs: 2 CPU, 4GB RAM, 100GB disk)
- **postgresql**: PostgreSQL database (recommended specs: 2 CPU, 8GB RAM, 100GB disk)

### **Development Tools**
- **docker**: Container runtime (recommended specs: 2 CPU, 4GB RAM, 100GB disk)
- **node**: Node.js development environment (recommended specs: 2 CPU, 4GB RAM)
- **python**: Python development environment (recommended specs: 2 CPU, 4GB RAM)

### **For Unknown Applications**
```python
# For applications not in the guide, get generic deployment template
guide = get_application_deployment_guide("my-custom-app", "production")
# Provides generic deployment workflow with customizable commands
```

## 💡 WHY THIS APPROACH IS BETTER

### ✅ **Advantages of Step-by-Step Approach:**
1. **Reliability**: Each step can be verified before proceeding
2. **Debugging**: Easy to identify and fix issues at each stage
3. **Flexibility**: Customize each step based on specific requirements
4. **Transparency**: User sees exactly what's happening
5. **Error Recovery**: Can retry individual steps without full redeployment

### ❌ **Problems with Automated deploy_application():**
1. **Black Box**: Difficult to debug when something fails
2. **All-or-Nothing**: Single failure breaks entire deployment
3. **Less Flexible**: Hard to customize for specific needs
4. **Complex Rollback**: Difficult to clean up partial deployments

## 🚨 CRITICAL LLM BEHAVIOR REQUIREMENTS

### When User Requests Application Deployment:

1. **ALWAYS use get_application_deployment_guide() FIRST**
2. **Follow the 9-step workflow above**
3. **NEVER skip the validation step (Step 5)**
4. **Use exact spec IDs from recommend_vm_spec()**
5. **Execute installation commands from the deployment guide**
6. **Verify deployment before declaring success**

### Example User Request Handling:
```
User: "Deploy nginx web server"

LLM Response:
"I'll deploy nginx using the reliable step-by-step approach:
1. First, let me get the nginx deployment guide...
2. I'll create/validate the namespace...
3. Get optimal VM specifications for nginx...
4. Validate the MCI configuration...
5. Create the infrastructure...
6. Install nginx using remote commands...
7. Verify the deployment...
8. Provide access information..."
```

### Example Commands Flow:
```python
# 1. Get deployment guide
guide = get_application_deployment_guide("nginx", "production")

# 2. Prepare namespace
check_and_prepare_namespace("web-servers")

# 3. Get VM specs
specs = recommend_vm_spec(
    filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
    priority_policy="cost"
)

# 4. Build VM config
vm_configs = [{
    "specId": specs["recommended_specs"][0]["id"],
    "name": "nginx-vm-1",
    "description": "Nginx web server VM"
}]

# 5. Validate
review = review_mci_dynamic_request("web-servers", "nginx-mci", vm_configs)

# 6. Create MCI (if validation passed)
mci = create_mci_dynamic("web-servers", "nginx-mci", vm_configs, force_create=True)

# 7. Install nginx
execute_command_mci("web-servers", "nginx-mci", [
    "sudo apt-get update -y",
    "sudo apt-get install -y nginx",
    "sudo systemctl start nginx",
    "sudo systemctl enable nginx",
    "echo 'Web Server accessible at: http://{{public_ip}}'"
])

# 8. Verify
execute_command_mci("web-servers", "nginx-mci", [
    "curl -I http://{{public_ip}}",
    "sudo systemctl is-active nginx"
])

# 9. Get access info
access_info = get_mci_access_info("web-servers", "nginx-mci")
```

## 🎯 SUCCESS METRICS

### A successful application deployment should:
1. ✅ Complete all 9 steps without errors
2. ✅ Pass MCI configuration validation
3. ✅ Successfully execute installation commands
4. ✅ Pass verification commands
5. ✅ Provide clear access information to user
6. ✅ Include troubleshooting guidance

This approach ensures reliable, debuggable, and maintainable application deployments.

## 🚀 SPECIAL CASE: Dynamic LLM Model Discovery for Ollama

### 🔍 NEW APPROACH: Real-Time Model Discovery (Recommended for Ollama)

When users request Ollama deployment, use the NEW dynamic discovery workflow:

#### Step 1: Provide Model Discovery Guidance
```python
# Give user instructions for finding latest models
discovery_guide = get_ollama_model_discovery_guide(
    use_case="coding",  # or user's specific use case
    category="code"     # or user's preferred category
)

# Show the discovery instructions to user
print("🔍 To find the latest Ollama models:")
print("1. Visit https://ollama.com/search")
print("2. Browse categories or search for specific model types")
print("3. Note down exact model names you want to deploy")
print("4. Consider model sizes vs your hardware requirements")
```

#### Step 2: User Model Selection Workflow
```python
# Guide user through selection process
print("📋 Example searches on ollama.com:")
print("- Search 'coder' for programming models")
print("- Search 'chat' for conversational models") 
print("- Search 'reasoning' for advanced reasoning models")
print("- Browse by size: 7B, 13B, 70B variants")

print("✅ Please select 2-5 models and provide their exact names")
print("Example selections:")
print("- ['llama3.3:latest', 'deepseek-r1:latest', 'qwen2.5:14b']")
print("- ['codellama:latest', 'mistral:latest', 'gemma2:9b']")
```

#### Step 3: Deploy with User-Selected Models
```python
# Once user provides model selections
selected_models = ["llama3.3:latest", "deepseek-r1:latest", "qwen2.5:14b"]  # User input

# Deploy using the enhanced tool
result = deploy_ollama_pull_with_models(
    ns_id="ai-workspace",
    mci_name="ollama-cluster",
    selected_models=selected_models,
    description="Custom LLM deployment with latest models from ollama.com"
)
```

#### Step 4: Alternative - Fallback to APPLICATION_CONFIGS
```python
# If user prefers not to browse ollama.com, use APPLICATION_CONFIGS fallback
# This will show guidance to visit ollama.com but use predefined workflow
execute_command_mci(ns_id, mci_id, [
    APPLICATION_CONFIGS["ollama_pull"]["commands"]  # Shows ollama.com guidance
])
```

### 🎯 Why Dynamic Discovery is Better:

#### ✅ **Advantages:**
1. **Always Current**: Gets latest models released on ollama.com
2. **User Choice**: User selects exactly what they need
3. **Flexibility**: Supports any model available on ollama.com
4. **Discovery Learning**: User learns about available options
5. **No Maintenance**: No need to update static model lists

#### 📋 **LLM Behavior for Ollama Requests:**
```
User: "Deploy Ollama with latest code models"

LLM Response:
"I'll help you deploy Ollama with the latest models. Let me guide you through 
discovering the most current models available:

🔍 Step 1: Model Discovery
I'll provide guidance on finding the latest models...
[calls get_ollama_model_discovery_guide()]

📋 Step 2: Your Selection
Please visit https://ollama.com/search and:
- Search for 'coder' or 'code' for programming models
- Note model sizes (7B, 13B, etc.) for your hardware
- Select 2-5 models you'd like to deploy

✅ Step 3: Deployment
Once you provide the model names, I'll deploy them using
deploy_ollama_pull_with_models() with your selections.

This ensures you get the very latest models available!"
```

### 🚨 CRITICAL: No More Static Model Lists
- ❌ **Avoid**: Hardcoded model lists that become outdated
- ✅ **Use**: Dynamic discovery through ollama.com
- ✅ **Guide**: Users to make informed selections
- ✅ **Deploy**: Exactly what users want from latest available models

This approach ensures users always get access to the newest and most relevant LLM models for their specific use cases.
"""

@mcp.prompt() 
def tumblebug_infrastructure_management():
    """
    Complete Guide for Multi-Cloud Infrastructure Management
    
    This prompt explains CB-Tumblebug's infrastructure management capabilities
    including namespace management, MCI operations, and resource optimization.
    """
    return """# CB-Tumblebug Infrastructure Management Guide

## Core Infrastructure Operations

### 1. Namespace Management
Namespaces organize your cloud resources:
```
create_namespace("production") - Create production environment
create_namespace("staging") - Create staging environment  
get_namespaces() - List all namespaces
delete_namespace("test") - Clean up test environment
```

### 2. MCI (Multi-Cloud Infrastructure) Lifecycle
```
# Creation Methods
create_mci_dynamic() - Full control with VM configurations
create_mci_dynamic() - Flexible MCI creation with custom VM configurations
recommend_vm_spec() - Find optimal VM specifications
search_images() - Find suitable OS images

# Management  
get_mci_list("production") - List infrastructure
get_mci("production", "web-servers") - Get detailed info
control_mci("production", "web-servers", "suspend") - Control operations
delete_mci("production", "web-servers") - Clean up
```

### 3. Resource Discovery
```
# Cloud Providers
get_connections() - Available cloud connections
get_connections_with_options() - Filtered connections

# Infrastructure Components
get_vnets("production") - Virtual networks
get_security_groups("production") - Security groups  
get_ssh_keys("production") - SSH key pairs
```

### 4. VM Specification Optimization
```
# Find optimal specs based on requirements
recommend_vm_spec(
    filter_policies={
        "vCPU": {"min": 2, "max": 8},
        "memoryGiB": {"min": 4, "max": 16},
        "ProviderName": "aws"
    },
    priority_policy="cost"  # or "performance" or "location"
)
```

### 5. Image Selection
```
# Search for OS images
search_images(
    provider_name="aws",
    region_name="ap-northeast-2", 
    os_type="ubuntu 22.04"
)
```

### 6. Access and Connectivity
```
get_mci_access_info("production", "web-servers") - SSH and IP info
execute_command_mci() - Run commands remotely
transfer_file_mci() - File transfer operations
```

### 7. Resource Cleanup
```
# Gradual cleanup
delete_mci("test", "temp-servers") - Remove specific MCI
release_resources("test") - Remove shared resources (VNet, etc.)
delete_namespace("test") - Complete cleanup

```

## Advanced Patterns

### 1. Multi-Region Deployment
```python
# Deploy across multiple regions
regions = ["aws+ap-northeast-2", "azure+koreacentral", "gcp+asia-northeast3"]
for region in regions:
    create_mci_dynamic(
        ns_id="global-app",
        name=f"app-{region.split('+')[0]}",
        vm_configurations=[{
            "imageId": selected_image,
            "specId": f"{region}+standard-instance",
            "subGroupSize": "3"
        }]
    )
```

### 2. Environment Separation
```
# Development
create_namespace("dev")
create_mci_dynamic("dev", "test-app", [{"imageId": image, "specId": "t2.micro", "name": "test-vm"}])

# Staging  
create_namespace("staging")
create_mci_dynamic("staging", "staging-app", [{"imageId": image, "specId": "t2.small", "name": "staging-vm"}])

# Production
create_namespace("production") 
create_mci_dynamic("production", "prod-app", complex_config)
```

### 3. Cost Optimization
```
# Find cost-effective options
specs = recommend_vm_spec(
    filter_policies={"ProviderName": "aws"},
    priority_policy="cost"
)

# Use spot instances or burstable types
vm_config = {
    "specId": "aws+ap-northeast-2+t3.micro",
    "subGroupSize": "5"
}
```

## Monitoring and Troubleshooting
1. Use get_mci_list_with_options() for status monitoring
2. Check get_mci() for detailed VM information  
3. Review execute_command_mci() outputs for issues
4. Use get_mci_access_info() for connectivity verification

## Security Best Practices
1. Use separate namespaces for different environments
2. Configure security groups appropriately
3. Manage SSH keys securely
4. Regular security updates via remote commands
5. Monitor access patterns and resource usage
"""

@mcp.prompt()
def tumblebug_usecase_examples():
    """
    Real-world Use Case Examples for CB-Tumblebug
    
    This prompt provides practical examples of common deployment scenarios
    using CB-Tumblebug's enhanced capabilities.
    """
    return """# CB-Tumblebug Use Case Examples

## 1. Gaming Infrastructure: Global Game Server Deployment

### Scenario: Deploy Xonotic Game Servers Worldwide
```
User: "I want to deploy Xonotic game servers in 10 regions for global players"

Solution:
1. deploy_application(
     ns_id="gaming",
     app_name="xonotic", 
     regions=10,
     deployment_strategy="global"
   )
   
2. Result: Automatic infrastructure provisioning + game server installation
3. Players connect to nearest server via provided IP addresses
```

### Gaming Infrastructure Features:
- Low-latency server placement
- Automatic game server configuration  
- Player capacity scaling
- Global load distribution

## 2. Web Application: Multi-Cloud Load Balancing

### Scenario: Deploy Web Application with Geographic Distribution
```
User: "Deploy Nginx web servers in AWS Seoul and Azure Korea Central"

Solution:
1. create_mci_dynamic("web-app", "nginx-aws", [{"imageId": aws_image, "specId": "t3.medium", "name": "nginx-aws-vm"}])
2. create_mci_dynamic("web-app", "nginx-azure", [{"imageId": azure_image, "specId": "Standard_B2s", "name": "nginx-azure-vm"}]) 
3. execute_remote_commands_enhanced(script_name="nginx_install")
4. Configure DNS load balancing
```

### Web Application Benefits:
- Geographic redundancy
- Improved user experience
- Disaster recovery capability
- Cost optimization across providers

## 3. AI/ML Workload: Distributed AI Inference

### Scenario: Deploy Ollama AI Service for Global AI Applications  
```
User: "I need Ollama AI inference service on powerful GPU instances"

Solution:
1. search_images(os_type="ubuntu 22.04") - Find compatible images
2. recommend_vm_spec(
     filter_policies={"gpu": true, "memoryGiB": {"min": 16}},
     priority_policy="performance"
   )
3. deploy_application(app_name="ollama", regions=3, vm_requirements={"gpu": true})
```

### AI/ML Infrastructure:
- GPU-optimized instances
- Model deployment automation
- API endpoint configuration
- Scaling based on demand

## 4. Development Environment: Team Development Infrastructure

### Scenario: Create Development Environment for Team
```
User: "Set up development infrastructure with Docker and monitoring"

Workflow:
1. create_namespace("dev-team")
2. create_mci_dynamic("dev-team", "dev-servers", [{"imageId": ubuntu_image, "specId": "t3.large", "name": f"dev-vm-{i}", "subGroupSize": "1"} for i in range(1, 6)])
3. execute_remote_commands_enhanced(script_name="docker_install")
4. execute_remote_commands_enhanced(script_name="monitoring_setup") 
5. execute_remote_commands_enhanced(script_name="security_hardening")
```

### Development Features:
- Containerized development
- Team collaboration tools
- Monitoring and logging
- Security hardening

## 5. Data Analytics: ELK Stack Deployment

### Scenario: Deploy Elasticsearch, Logstash, Kibana for Log Analytics
```
User: "Deploy ELK stack for centralized logging"

Solution:
1. deploy_application(
     app_name="elk",
     regions=1, 
     vm_requirements={"memoryGiB": {"min": 8}, "cpu": {"min": 4}}
   )
2. Configure log shipping from applications
3. Set up Kibana dashboards
```

### Analytics Infrastructure:
- Centralized log collection
- Real-time data processing  
- Interactive dashboards
- Scalable storage

## 6. Video Conferencing: Jitsi Meet Platform

### Scenario: Deploy Video Conferencing for Organization
```
User: "Deploy Jitsi Meet for our organization's video conferencing"

Solution:
1. deploy_application(app_name="jitsi", regions=2)
2. Configure domain and SSL certificates
3. Set up user authentication
4. Monitor performance and scaling
```

### Video Conferencing Features:
- Self-hosted video platform
- High-quality video/audio
- Multi-region deployment
- Privacy and security control

## 7. High-Performance Computing: Ray Cluster

### Scenario: Deploy Distributed Computing with Ray
```
User: "Create Ray cluster for distributed machine learning"

Solution:
1. deploy_application(
     app_name="ray",
     regions=1,
     vm_count=5,
     vm_requirements={"cpu": {"min": 8}, "memoryGiB": {"min": 16}}
   )
2. Configure Ray head and worker nodes
3. Deploy ML workloads
```

### HPC Benefits:
- Distributed computing power
- Automatic scaling
- Resource optimization
- ML/AI workload support

## Common Patterns Across Use Cases

### 1. Infrastructure Preparation
```
1. create_namespace() - Environment isolation
2. search_images() - Find suitable OS images  
3. recommend_vm_spec() - Optimize instance selection
4. get_connections() - Verify cloud provider availability
```

### 2. Application Deployment
```
1. deploy_application() - Automated deployment
2. get_mci_access_info() - Collect endpoints
3. execute_application_commands() - Post-deployment configuration
4. execute_remote_commands_enhanced() - Ongoing management
```

### 3. Monitoring and Management
```
1. get_mci_list() - Monitor infrastructure
2. execute_command_mci() - Health checks
3. control_mci() - Scaling operations
```

### 4. Cleanup and Optimization
```
1. delete_mci() - Remove unused infrastructure
2. release_resources() - Clean shared resources
3. delete_namespace() - Complete environment cleanup
```

## Best Practices Summary
1. **Start Small**: Use create_mci_dynamic() for testing
2. **Scale Gradually**: Use deploy_application() for production
3. **Monitor Resources**: Regular health checks and cost monitoring  
4. **Security First**: Apply security hardening from day one
5. **Document Endpoints**: Keep track of service URLs and IPs
6. **Plan Cleanup**: Regular resource cleanup to control costs

## 🔍 MCI Validation and Quality Assurance Workflow

### MANDATORY Pre-Validation Process

**✅ ALWAYS Follow This Workflow for MCI Creation:**

```python
# STEP 1: Search for appropriate VM specifications
specs = recommend_vm_spec(
    filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
    priority_policy="cost"  # or "performance" or "location"
)

# STEP 2: Build VM configurations using ONLY returned spec IDs
vm_configurations = []
for i, spec in enumerate(specs["recommended_specs"][:2]):
    vm_configurations.append({
        "specId": spec["id"],  # 🚨 CRITICAL: Use exact ID from API
        "name": f"vm-{spec['providerName']}-{i+1}",
        "description": f"VM in {spec['regionName']}",
        "subGroupSize": "1"
        # imageId: Optional - auto-mapped if omitted
    })

# STEP 3: MANDATORY - Review configuration before creation
review_result = review_mci_dynamic_request(
    ns_id="my-project",
    name="web-app-cluster", 
    vm_configurations=vm_configurations,
    description="Production web application cluster"
)

# STEP 4: Check validation results
if review_result.get("summary", {}).get("validationPassed", False):
    print("✅ Configuration validated successfully!")
    print(f"Estimated cost: {review_result.get('estimated_cost', 'N/A')}")
    print(f"Deployment time: {review_result.get('deployment_time_estimate', 'N/A')}")
    
    # STEP 5: Proceed with MCI creation
    mci_result = create_mci_dynamic(
        ns_id="my-project",
        name="web-app-cluster",
        vm_configurations=vm_configurations,
        force_create=True  # Skip confirmation since we already reviewed
    )
    
else:
    print("❌ Validation failed!")
    print("Issues to fix:")
    for vm_validation in review_result.get("vm_validations", []):
        if vm_validation.get("issues"):
            print(f"VM {vm_validation['vm_index']}: {vm_validation['issues']}")
```

### Automatic Pre-Validation in create_mci_dynamic()

**The create_mci_dynamic() function now includes automatic pre-validation:**

1. **Automatic Review**: Every call to create_mci_dynamic() automatically runs review_mci_dynamic_request()
2. **Validation Gate**: MCI creation is blocked if critical validation issues are found
3. **Enhanced Feedback**: Detailed validation results guide you to fix configuration problems
4. **Smart Recovery**: Clear guidance on how to address validation failures

### Enhanced Error Prevention

**🚨 CRITICAL VALIDATIONS PERFORMED:**

✅ **VM Specification Validation**
- Ensures specId IDs are valid and available
- Verifies specifications exist in target CSP/region
- Validates resource quotas and limits

✅ **Image Compatibility Validation**  
- Checks image-spec compatibility across CSPs
- Validates architecture compatibility (x86_64, ARM)
- Ensures images are available in target regions

✅ **Resource Availability Validation**
- Verifies sufficient compute quotas
- Checks network resource availability
- Validates storage capacity and types

✅ **Cost and Time Estimation**
- Provides hourly and monthly cost estimates
- Estimates deployment completion time
- Identifies potential cost optimization opportunities

✅ **Security and Compliance Validation**
- Validates SSH key requirements
- Checks security group configurations
- Ensures network security policies

### Example: Validated MCI Creation Workflow

**Recommended Pattern for Error-Resilient MCI Creation:**

1. **Get VM Specifications**: Use recommend_vm_spec() to get valid spec IDs
2. **Build Configurations**: Create VM configs using exact spec IDs from API
3. **Validate Configuration**: Use review_mci_dynamic_request() to check setup
4. **Handle Validation Results**: Address any issues before proceeding
5. **Create Infrastructure**: Use create_mci_dynamic() with force_create=True

**Example Usage Pattern:**
- Call recommend_vm_spec() with your requirements
- Use returned spec["id"] values in vm_configurations
- Run review_mci_dynamic_request() to validate
- Check validation_passed status before proceeding
- Use create_mci_dynamic() with validated configurations

### LLM Integration Guidelines

**When working with MCI creation, LLMs should:**

1. **Always Use Validation**: Never skip the review_mci_dynamic_request() step
2. **Interpret Results**: Analyze validation results and explain issues to users
3. **Provide Guidance**: Offer specific steps to fix validation failures
4. **Optimize Configurations**: Suggest improvements based on validation feedback
5. **Estimate Costs**: Present cost implications clearly to users
6. **Plan Deployments**: Use validation insights to optimize deployment strategies

### Validation Result Analysis

**Understanding Validation Responses:**

```python
{
    "validation_passed": true,  # Overall validation status
    "summary": {
        "validationPassed": true,
        "totalVms": 2,
        "totalErrors": 0,      # Critical issues (blocks creation)
        "totalWarnings": 1,    # Recommendations for optimization  
        "totalInfo": 2         # General information and tips
    },
    "vm_validations": [        # Per-VM validation details
        {
            "vm_index": 0,
            "status": "valid",
            "spec_info": {...},  # VM specification details
            "image_info": {...}, # Image mapping details
            "issues": [],        # Critical problems (must fix)
            "warnings": [],      # Optimization suggestions
            "info": [           # General information
                "Custom root disk type configured: gp3"
            ]
        }
    ],
    "estimated_cost": "~$0.15/hour (~$108/month)",
    "deployment_time_estimate": "3-5 minutes",
    "optimization_suggestions": [...]
}
```

**Response to Validation Results:**

- ✅ **validation_passed: true** → Safe to proceed with create_mci_dynamic()
- ❌ **validation_passed: false** → Must fix issues before creation
- ⚠️ **warnings > 0** → Consider optimization suggestions
- ℹ️ **info messages** → Informational, no action required

This comprehensive validation system ensures reliable, cost-effective, and properly configured MCI deployments.

## 🕒 CRITICAL REMINDER: Remote Command Execution Timing

### 🚨 **MANDATORY USER WARNINGS for ALL LLMs**

**Before executing ANY remote commands, LLMs MUST inform users:**

```
⏰ IMPORTANT TIMING NOTICE:
• Remote command execution can take 5-20+ minutes
• Complex deployments may require up to 20 minutes
• Please be patient during the process
• Progress will be reported when commands complete

📊 Typical timing expectations:
- Simple commands: 10-30 seconds
- Package installation: 2-5 minutes  
- Application deployment: 5-15 minutes
- Complex setups: 10-20+ minutes
```

### 🎯 **LLM Best Practices for Command Execution:**

1. **⚠️ Always warn users first** before any execute_command_mci() call
2. **📦 Batch related commands** to minimize API calls
3. **🔍 Add verification steps** to check progress
4. **📊 Use summarize_output=True** for large outputs
5. **💡 Explain what's happening** during long operations

### 🔧 **Technical Implementation Notes:**

- **API timeout extended to 20 minutes** for remote commands
- **Automatic output summarization** to manage response size
- **Enhanced error handling** for timeout scenarios
- **Progress indicators** in command responses

**🎯 Remember: Setting proper expectations prevents user frustration and ensures smooth deployment experiences.**
"""

@mcp.prompt()
def vm_spec_recommendation_guide():
    """
    VM Specification Recommendation Best Practices Guide
    
    This prompt ensures LLMs always use get_recommend_spec_options() before recommend_vm_spec()
    to prevent failures from invalid parameters and guide proper usage of the recommendation system.
    """
    return """# 🔧 VM Specification Recommendation Guide

## 🚨 MANDATORY WORKFLOW: Options First, Then Recommendation

### ⚠️ CRITICAL: ALWAYS Call get_recommend_spec_options() FIRST

**NEVER call recommend_vm_spec() without first calling get_recommend_spec_options()!**

This prevents common failures from:
- ❌ Invalid metric names in filter policies
- ❌ Non-existent provider names, regions, architectures
- ❌ Wrong operator formats or parameter structures
- ❌ Invalid priority policy configurations

### 🔄 STEP-BY-STEP WORKFLOW:

#### STEP 1: Get Available Options
```python
# ALWAYS start with this call
spec_options = get_recommend_spec_options(ns_id="system")

# This returns comprehensive information:
# - filter.availableMetrics: All metrics you can filter by
# - filter.availableValues: Actual values for providers, regions, etc.
# - filter.examplePolicies: Pre-built example filter configurations
# - priority.availableMetrics: Available priority options
# - priority.examplePolicies: Example priority configurations
# - priority.parameterOptions: Required parameters for location/latency
# - limit: Suggested limit values
```

#### STEP 2: Analyze Available Options
```python
# Extract key information for building requests
available_metrics = spec_options["filter"]["availableMetrics"]
# Example: ["id", "providerName", "regionName", "cspSpecName", "architecture", 
#           "vCPU", "memoryGiB", "costPerHour", "acceleratorType", ...]

available_providers = spec_options["filter"]["availableValues"]["providerName"]
# Example: ["alibaba", "aws", "azure", "gcp", "ibm", "kt", "ncp", "nhn", "tencent"]

available_regions = spec_options["filter"]["availableValues"]["regionName"]
# Example: ["us-east-1", "ap-northeast-2", "eu-west-1", ...]

priority_options = spec_options["priority"]["availableMetrics"]
# Example: ["cost", "performance", "location", "latency", "random"]
```

#### STEP 3: Build Valid Filter Policies
```python
# ✅ Use ONLY metrics from availableMetrics
# ✅ Use ONLY providers from availableValues.providerName
# ✅ Use ONLY regions from availableValues.regionName

# Example 1: Basic resource filtering
filter_policies = {
    "policy": [
        {
            "metric": "vCPU",  # ✅ From availableMetrics
            "condition": [
                {"operator": ">=", "operand": "2"},
                {"operator": "<=", "operand": "8"}
            ]
        },
        {
            "metric": "memoryGiB",  # ✅ From availableMetrics
            "condition": [
                {"operator": ">=", "operand": "4"}
            ]
        },
        {
            "metric": "providerName",  # ✅ From availableMetrics
            "condition": [
                {"operator": "=", "operand": "aws"}  # ✅ From availableValues
            ]
        }
    ]
}

# Example 2: Using example policies as templates
example_policy = spec_options["filter"]["examplePolicies"][0]
# Modify the example as needed for your requirements
```

#### STEP 4: Configure Priority with Valid Options
```python
# ✅ Use ONLY priority metrics from availableMetrics
priority_policy = {
    "policy": [
        {
            "metric": "cost",  # ✅ From priority.availableMetrics
            "weight": "1.0"
        }
    ]
}

# OR for location-based priority:
priority_policy = {
    "policy": [
        {
            "metric": "location",  # ✅ From priority.availableMetrics
            "parameter": [
                {
                    "key": "coordinateClose",  # ✅ From parameterOptions
                    "val": ["37.5665/126.9780"]  # Seoul coordinates
                }
            ],
            "weight": "1.0"
        }
    ]
}
```

#### STEP 5: Call recommend_vm_spec with Validated Parameters
```python
# Now safely call recommend_vm_spec with validated parameters
specs = recommend_vm_spec(
    filter_policies=filter_policies,
    priority_policy=priority_policy,
    limit="10"  # ✅ From suggested limit values
)
```

### 🎯 COMMON USE CASES WITH EXAMPLES:

#### 💰 Cost-Optimized Specs
```python
# 1. Get options
spec_options = get_recommend_spec_options()

# 2. Build cost-focused filter
filter_policies = {
    "policy": [
        {
            "metric": "costPerHour",
            "condition": [{"operator": "<=", "operand": "0.50"}]
        },
        {
            "metric": "vCPU", 
            "condition": [{"operator": ">=", "operand": "2"}]
        }
    ]
}

# 3. Set cost priority
priority_policy = {"policy": [{"metric": "cost", "weight": "1.0"}]}

# 4. Get recommendations
specs = recommend_vm_spec(
    filter_policies=filter_policies,
    priority_policy=priority_policy,
    limit="5"
)
```

#### 🌍 Location-Based Specs
```python
# 1. Get options
spec_options = get_recommend_spec_options()

# 2. Build location-focused filter
filter_policies = {
    "policy": [
        {
            "metric": "vCPU",
            "condition": [
                {"operator": ">=", "operand": "4"},
                {"operator": "<=", "operand": "16"}
            ]
        }
    ]
}

# 3. Set location priority with coordinates
priority_policy = {
    "policy": [
        {
            "metric": "location",
            "parameter": [
                {
                    "key": "coordinateClose", 
                    "val": ["35.6762/139.6503"]  # Tokyo coordinates
                }
            ],
            "weight": "1.0"
        }
    ]
}

# 4. Get recommendations
specs = recommend_vm_spec(
    filter_policies=filter_policies,
    priority_policy=priority_policy,
    limit="10"
)
```

#### 🎮 GPU-Enabled Specs
```python
# 1. Get options
spec_options = get_recommend_spec_options()

# 2. Build GPU-focused filter
filter_policies = {
    "policy": [
        {
            "metric": "acceleratorType",
            "condition": [{"operator": "=", "operand": "gpu"}]
        },
        {
            "metric": "memoryGiB",
            "condition": [{"operator": ">=", "operand": "16"}]
        }
    ]
}

# 3. Set performance priority
priority_policy = {"policy": [{"metric": "performance", "weight": "1.0"}]}

# 4. Get recommendations
specs = recommend_vm_spec(
    filter_policies=filter_policies,
    priority_policy=priority_policy,
    limit="5"
)
```

### 🚨 ERROR PREVENTION CHECKLIST:

Before calling recommend_vm_spec(), verify:
- ✅ All metric names exist in availableMetrics
- ✅ All provider names exist in availableValues.providerName  
- ✅ All region names exist in availableValues.regionName
- ✅ All architecture values exist in availableValues.architecture
- ✅ Priority metric exists in priority.availableMetrics
- ✅ Required parameters are provided for location/latency priorities
- ✅ Limit value is within suggested range

### 💡 LLM BEST PRACTICES:

1. **Always explain the two-step process** to users
2. **Show the available options** before asking for preferences
3. **Validate user inputs** against available values
4. **Use example policies** as starting points
5. **Explain coordinate requirements** for location-based priorities
6. **Handle errors gracefully** by re-checking options

This workflow ensures reliable VM specification recommendations and prevents API failures.
"""

if __name__ == "__main__":
    # Map our log level to FastMCP/uvicorn log level
    fastmcp_log_level = "warning" if log_level_value > logging.DEBUG else "info"
    
    mcp.run(
        transport="http",
        host=host,
        port=port,
        path="/mcp",
        log_level=fastmcp_log_level,
    )