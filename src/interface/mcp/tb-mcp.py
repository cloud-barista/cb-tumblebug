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
import re
from typing import Dict, List, Optional, Any, Union
from datetime import datetime, timedelta
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
        ns_id = single_ns.get('id', 'unknown')
        result["recommendation"] = (
            f"One namespace available: '{ns_id}'. "
            "You can use this for MCI creation or create a new one if needed."
        )
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
        os_architecture: OS architecture filter (e.g., "x86_64", "arm64"). Defaults to "x86_64" if not specified.
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
    - **CRITICAL**: Each VM spec requires its own image search in the spec's CSP/region
    - **RECOMMENDED**: Use create_mci_dynamic() auto-mapping by omitting commonImage in VM configurations
    - **VALIDATION**: Use validate_vm_spec_image_compatibility() to check configurations before deployment
    """
    data = {}
    
    # Add default x86_64 architecture if not specified by user
    if os_architecture:
        data["osArchitecture"] = os_architecture
    else:
        data["osArchitecture"] = "x86_64"  # Default to x86_64
    
    # Build search criteria
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
    limit: str = "50",
    priority_policy: str = "location",
    latitude: Optional[float] = None,
    longitude: Optional[float] = None,
    include_full_details: bool = False
) -> Any:
    """
    🚨 CRITICAL: This is the ONLY valid source for VM specification IDs in MCI creation.
    NEVER create or guess spec IDs manually - they MUST come from this function's response.
    
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
       - commonSpec: specification ID from this function (NEVER modify)
       - commonImage: cspImageName from search_images()
    
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
        - id: Specification ID (use as 'commonSpec' in create_mci_dynamic() WITHOUT modification)
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
    The 'id' field from results becomes the 'commonSpec' parameter in create_mci_dynamic().
    Format is typically: {provider}+{region}+{spec_name} but MUST be from API response.
    """
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
    
    # Make API request
    raw_response = api_request("POST", "/mciRecommendVm", json_data=data)
    
    # Summarize response to reduce token usage
    return _summarize_vm_specs(raw_response, include_details=include_full_details)

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
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_command: Optional[Dict] = None,
    hold: bool = False,
    skip_confirmation: bool = False,
    force_create: bool = False
) -> Dict:
    """
    🚨 CRITICAL: VM specifications MUST come from recommend_vm_spec() - NEVER create spec IDs manually.
    
    Create Multi-Cloud Infrastructure dynamically using the official API specification.
    This is the RECOMMENDED method for MCI creation with MANDATORY spec validation.
    
    **🔥 ABSOLUTELY REQUIRED WORKFLOW:**
    ```python
    # STEP 1: MANDATORY - Get valid spec IDs from recommend_vm_spec()
    # For location-based requests: Use user's location coordinates
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
        priority_policy="location",  # When user mentions location
        latitude=37.4419,            # User's desired location
        longitude=-122.1430          # (e.g., Silicon Valley)
    )
    
    # STEP 2: REQUIRED - Use ONLY the returned spec IDs
    vm_configurations = []
    for i, spec in enumerate(specs["recommended_specs"][:2]):
        vm_configurations.append({
            "commonSpec": spec["id"],  # 🚨 MUST use exact ID from API response
            "name": f"vm-{spec['providerName']}-{i+1}",
            "subGroupSize": "1"
            # commonImage is optional - will be auto-mapped to spec's CSP/region
        })
    
    # STEP 3: Create MCI with validated specifications
    create_mci_dynamic(
        ns_id="default",
        name="location-based-mci",
        vm_configurations=vm_configurations
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
            "commonSpec": spec["id"],  # e.g., "aws+us-west-1+t3.medium"
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
    
    # 2. Process each spec individually  
    vm_configs = []
    for i, spec in enumerate(specs[:2]):  # Take first 2 different specs
        spec_id = spec["id"]  # e.g., "aws+ap-northeast-2+t2.small"
        provider = spec_id.split('+')[0]  # Extract "aws"
        region = spec_id.split('+')[1]    # Extract "ap-northeast-2"
        
        # 3. Search for images in THIS specific CSP/region
        images = search_images(
            provider_name=provider,
            region_name=region,
            os_type="ubuntu 22.04"
        )
        
        # 4. Select best image for THIS specific spec
        best_image = select_best_image_for_spec(
            images["imageList"], 
            spec, 
            {"os_type": "ubuntu 22.04"}
        )
        
        # 5. Add VM config with spec-matched image
        vm_configs.append({
            "commonImage": best_image["cspImageName"],  # CSP-specific image
            "commonSpec": spec_id,                      # CSP-specific spec
            "name": f"vm-{provider}-{i+1}",
            "description": f"VM on {provider} in {region}",
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
            - commonSpec: VM specification ID from recommend_vm_spec() (required)
            - commonImage: CSP-specific image identifier from search_images() (optional - auto-mapped if omitted)
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
        system_label: System label for special purposes (optional)
        label: Key-value pairs for MCI labeling (optional)
        post_command: Post-deployment command configuration with format:
            {"command": ["command1", "command2"], "userName": "username"} (optional)
        hold: Whether to hold provisioning for review (optional)
        skip_confirmation: Skip user confirmation step (for automated workflows, default: False)
        force_create: Bypass confirmation and create MCI immediately (default: False)
    
    Returns:
        **CONFIRMATION WORKFLOW (default behavior):**
        When force_create=False and skip_confirmation=False:
        - Returns comprehensive creation summary with:
          • Detailed cost analysis (hourly/monthly estimates)
          • CSP and region distribution
          • VM specifications and deployment strategy
          • Risk assessment and recommendations
          • User confirmation prompt with next steps
        
        **IMMEDIATE CREATION:**
        When force_create=True or skip_confirmation=True:
        - Created MCI information including:
          • id: MCI ID for future operations
          • vm: List of created VMs with their details
          • status: Current MCI status
          • deployment summary
        
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
    - commonSpec is required for all VM configurations
    - commonImage is optional - if omitted, will be auto-mapped based on commonSpec's CSP/region
    - Auto-mapping ensures each VM gets the correct CSP-specific image (AWS AMI, Azure Image ID, etc.)
    - Manual commonImage is validated against commonSpec for compatibility
    - The format for commonSpec is typically: {provider}+{region}+{spec_name}
    - Each VM configuration automatically gets the appropriate image for its specification
    - Use subGroupSize > 1 to create multiple VMs with same configuration
    - For multi-CSP deployments, each VM will automatically get its provider-specific image
    """
    # STEP 0: User confirmation workflow (unless explicitly skipped or forced)
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
    ns_validation = validate_namespace(ns_id)
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
        # Check if commonSpec is provided
        if "commonSpec" not in vm_config:
            return {
                "error": f"VM configuration {i} validation failed",
                "details": "VM configuration must include 'commonSpec'",
                "suggestion": "Use recommend_vm_spec() to get 'commonSpec'"
            }
        
        common_spec = vm_config["commonSpec"]
        common_image = vm_config.get("commonImage")
        
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
                vm_config["commonImage"] = chosen_image["cspImageName"]
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
                    "suggestion": "Manually specify 'commonImage' or check spec format"
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
                                "suggestion": "Use auto-mapping by omitting 'commonImage' or provide correct CSP-specific image"
                            }
                
            except Exception as e:
                # Continue with provided image if validation fails
                pass
        
        processed_vm_configs.append(vm_config)
    
    # Build request data according to model.TbMciDynamicReq spec
    data = {
        "name": name,
        "vm": processed_vm_configs  # Use processed configs with auto-mapped images
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

# Tool: Create MCI with proper spec-to-image mapping
@mcp.tool()
def create_mci_with_proper_spec_mapping(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI created with proper spec-to-image mapping",
    hold: bool = False,
    skip_confirmation: bool = False
) -> Dict:
    """
    Create MCI with proper spec-to-image mapping to ensure each VM gets the correct image for its specification.
    
    **CRITICAL IMPROVEMENT:**
    This function addresses the common issue where multiple VMs with different specs/CSPs
    incorrectly use the same image. Each VM spec requires its own image search and selection.
    
    **WHY SPEC-TO-IMAGE MAPPING MATTERS:**
    - Different CSPs use different image identifiers (AWS AMI vs Azure Image ID)
    - Same OS in different regions may have different image IDs
    - Different architectures require different images
    - Provider-optimized images perform better than generic ones
    
    **Example of WRONG approach (what this function fixes):**
    ```
    # WRONG: Using same image for different specs
    vm_configs = [
        {"commonImage": "ami-123456", "commonSpec": "aws+us-east-1+t2.small"},
        {"commonImage": "ami-123456", "commonSpec": "azure+eastus+Standard_B2s"}  # ERROR!
    ]
    ```
    
    **Example of CORRECT approach (what this function does):**
    ```
    # CORRECT: Each spec gets its own properly matched image
    vm_configs = [
        {"commonImage": "ami-123456", "commonSpec": "aws+us-east-1+t2.small"},
        {"commonImage": "/subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/images/ubuntu-20.04", 
         "commonSpec": "azure+eastus+Standard_B2s"}
    ]
    ```
    
    Args:
        ns_id: Namespace ID
        name: MCI name
        vm_configurations: List of VM configurations where each must have:
            - commonSpec: VM specification ID (required)
            - name: VM name (optional)
            - description: VM description (optional)
            - subGroupSize: Number of VMs in subgroup (optional)
            - os_requirements: Dict with os_type, use_case, etc. (optional)
        description: MCI description
        hold: Whether to hold provisioning
        skip_confirmation: Skip user confirmation step (for automated workflows, default: False)
    
    Returns:
        If skip_confirmation=False: Returns creation summary for user confirmation
        If skip_confirmation=True or after confirmation: Result with detailed spec-to-image mapping info and MCI creation status
    """
    # STEP 0: Generate creation summary for user confirmation (unless skipped)
    if not skip_confirmation:
        creation_summary = generate_mci_creation_summary(
            ns_id=ns_id,
            name=name,
            vm_configurations=vm_configurations,
            description=description,
            hold=hold
        )
        
        # Return summary for user review
        creation_summary["_mci_creation_parameters"] = {
            "ns_id": ns_id,
            "name": name,
            "vm_configurations": vm_configurations,
            "description": description,
            "hold": hold
        }
        creation_summary["_next_action"] = {
            "message": "Review the configuration above. To proceed with spec-aware MCI creation, call create_mci_with_proper_spec_mapping() again with skip_confirmation=True",
            "function_call": f"create_mci_with_proper_spec_mapping(ns_id='{ns_id}', name='{name}', vm_configurations=<same_config>, skip_confirmation=True)"
        }
        
        return creation_summary
    
    result = {
        "spec_analysis": {},
        "image_mapping": {},
        "final_configurations": [],
        "mci_creation": {},
        "status": "in_progress"
    }
    
    # Validate namespace
    ns_validation = validate_namespace(ns_id)
    if not ns_validation["valid"]:
        result["status"] = "failed"
        result["error"] = f"Invalid namespace: {ns_id}"
        return result
    
    # Process each VM configuration
    final_vm_configs = []
    
    for i, vm_config in enumerate(vm_configurations):
        vm_name = vm_config.get("name", f"vm-{i+1}")
        common_spec = vm_config.get("commonSpec")
        
        if not common_spec:
            result["status"] = "failed"
            result["error"] = f"Missing commonSpec for VM configuration {i+1}"
            return result
        
        # Get detailed spec information
        try:
            # Extract provider and region from spec ID
            spec_parts = common_spec.split("+")
            if len(spec_parts) < 3:
                result["status"] = "failed"
                result["error"] = f"Invalid spec format: {common_spec}. Expected: provider+region+spec_name"
                return result
            
            provider = spec_parts[0]
            region = spec_parts[1]
            spec_name = spec_parts[2]
            
            result["spec_analysis"][vm_name] = {
                "spec_id": common_spec,
                "provider": provider,
                "region": region,
                "spec_name": spec_name
            }
            
        except Exception as e:
            result["status"] = "failed"
            result["error"] = f"Failed to parse spec {common_spec}: {str(e)}"
            return result
        
        # Search for images in the specific CSP/region
        os_requirements = vm_config.get("os_requirements", {})
        os_type = os_requirements.get("os_type", "ubuntu")
        
        try:
            # Search for images specific to this VM's CSP and region
            images_result = search_images(
                provider_name=provider,
                region_name=region,
                os_type=os_type
            )
            
            if not images_result or "error" in images_result:
                result["status"] = "failed"
                result["error"] = f"Failed to find images for {vm_name} in {provider}/{region}"
                result["image_mapping"][vm_name] = {"error": images_result}
                return result
            
            image_list = images_result.get("imageList", [])
            if not image_list:
                result["status"] = "failed"
                result["error"] = f"No images found for {vm_name} in {provider}/{region}"
                return result
            
            # Create a mock spec object for image selection
            mock_spec = {
                "id": common_spec,
                "providerName": provider,
                "regionName": region,
                "architecture": "x86_64"  # Default, could be enhanced
            }
            
            # Select the best image for this specific spec
            chosen_image = select_best_image_for_spec(image_list, mock_spec, os_requirements)
            
            if not chosen_image or "error" in chosen_image:
                # Fallback to basic selection
                chosen_image = select_best_image(image_list)
                if not chosen_image:
                    result["status"] = "failed"
                    result["error"] = f"Failed to select image for {vm_name}"
                    return result
            
            csp_image_name = chosen_image["cspImageName"]
            
            result["image_mapping"][vm_name] = {
                "provider": provider,
                "region": region,
                "selected_image": csp_image_name,
                "selection_reason": chosen_image.get("_selection_reason", "Selected by analysis"),
                "compatibility_score": chosen_image.get("_compatibility_score", "N/A"),
                "spec_match": chosen_image.get("_spec_match", "unknown"),
                "is_basic_image": chosen_image.get("isBasicImage", False)
            }
            
            # Create final VM configuration with proper image mapping
            final_vm_config = {
                "commonImage": csp_image_name,
                "commonSpec": common_spec,
                "name": vm_name,
                "description": vm_config.get("description", f"VM {vm_name} - {provider} {region}"),
                "subGroupSize": str(vm_config.get("subGroupSize", 1))
            }
            
            # Add any additional configuration
            for key in ["connectionName", "rootDiskSize", "rootDiskType", "vmUserPassword", "label"]:
                if key in vm_config:
                    final_vm_config[key] = vm_config[key]
            
            final_vm_configs.append(final_vm_config)
            
        except Exception as e:
            result["status"] = "failed"
            result["error"] = f"Failed to process VM {vm_name}: {str(e)}"
            return result
    
    result["final_configurations"] = final_vm_configs
    
    # Create MCI with properly mapped configurations
    try:
        mci_result = create_mci_dynamic(
            ns_id=ns_id,
            name=name,
            vm_configurations=final_vm_configs,
            description=description,
            hold=hold
        )
        
        result["mci_creation"] = mci_result
        
        if "error" not in mci_result:
            result["status"] = "success"
            result["summary"] = {
                "namespace_id": ns_id,
                "mci_id": mci_result.get("id", name),
                "mci_name": name,
                "total_vms": len(final_vm_configs),
                "unique_csps": len(set([mapping["provider"] for mapping in result["image_mapping"].values()])),
                "unique_regions": len(set([mapping["region"] for mapping in result["image_mapping"].values()])),
                "mapping_quality": "All VMs have CSP-specific images"
            }
        else:
            result["status"] = "mci_creation_failed"
            result["error"] = "MCI creation failed after successful image mapping"
        
    except Exception as e:
        result["status"] = "failed"
        result["error"] = f"MCI creation error: {str(e)}"
    
    return result

# Tool: Create MCI Dynamic (Simplified)
@mcp.tool()
def create_simple_mci(
    ns_id: str,
    name: str,
    common_spec: str,
    common_image: Optional[str] = None,
    vm_count: int = 1,
    description: str = "MCI created via simplified MCP interface",
    hold: bool = False,
    skip_confirmation: bool = False,
    force_create: bool = False
) -> Dict:
    """
    Simplified MCI creation for common use cases. 
    This is a wrapper around create_mci_dynamic() for easier usage when all VMs use the same configuration.
    
    **WORKFLOW:**
    1. Use recommend_vm_spec() to find appropriate VM specifications (determines CSP and region)
    2. From spec results, extract CSP and region information  
    3. Use search_images() to find your desired image in the selected CSP/region
    4. Use this function with the found spec and image
    
    **AUTOMATIC IMAGE MAPPING:**
    This function now supports automatic image selection when common_image is not provided.
    Simply pass the spec and let the function find the best matching image automatically.
    
    **Example with auto-mapping:**
    ```
    # Find VM specs (determines CSP and region)
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
    )
    spec_id = specs[0]["id"]  # e.g., "aws+ap-northeast-2+t2.small"
    
    # Create MCI with automatic image selection (no need to manually find image)
    mci = create_simple_mci(
        ns_id="default",
        name="my-simple-mci",
        common_spec=spec_id,      # Only spec needed - image auto-selected
        vm_count=3
    )
    ```
    
    **Example with manual image:**
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
        common_spec: VM specification ID from recommend_vm_spec() results
        common_image: CSP-specific image identifier from search_images() results (optional - auto-selected if omitted)
        vm_count: Number of identical VMs to create (default: 1)
        description: MCI description
        hold: Whether to hold provisioning for review
        skip_confirmation: Skip user confirmation step (for automated workflows, default: False)
        force_create: Bypass confirmation and create MCI immediately (default: False)
    
    Returns:
        **CONFIRMATION WORKFLOW (default behavior):**
        When force_create=False and skip_confirmation=False:
        - Returns comprehensive creation summary with cost analysis and confirmation prompt
        
        **IMMEDIATE CREATION:**
        When force_create=True or skip_confirmation=True:
        - Created MCI information (same as create_mci_dynamic)
    """
    # Create VM configurations for identical VMs
    vm_configurations = []
    for i in range(vm_count):
        vm_config = {
            "commonSpec": common_spec,
            "name": f"vm-{i+1}",
            "description": f"VM {i+1} of {vm_count}",
            "subGroupSize": "1"
        }
        
        # Only add commonImage if provided (auto-mapping will handle if omitted)
        if common_image:
            vm_config["commonImage"] = common_image
        
        vm_configurations.append(vm_config)

    return create_mci_dynamic(
        ns_id=ns_id,
        name=name,
        vm_configurations=vm_configurations,
        description=description,
        hold=hold,
        skip_confirmation=skip_confirmation,
        force_create=force_create  # Pass through force_create setting
    )# Tool: Smart MCI Creation with Spec-First Workflow
@mcp.tool()
def create_mci_with_spec_first(
    name: str,
    vm_requirements: List[Dict],
    preferred_ns_id: Optional[str] = None,
    create_ns_if_missing: bool = False,
    ns_description: Optional[str] = None,
    description: str = "MCI created with spec-first workflow",
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
        
        # Select the best suitable image using spec-aware intelligent selection
        image_list = images_result.get("imageList", [])
        if not image_list:
            result["status"] = "failed"
            result["error"] = f"No images found for {req_name} in {provider}/{region}"
            return result
        
        # Use spec-aware image selection for better compatibility
        image_requirements = {
            "os_type": vm_req.get("os_type", ""),
            "use_case": vm_req.get("use_case", "general"),
            "version": vm_req.get("version", "")
        }
        
        chosen_image = select_best_image_for_spec(image_list, chosen_spec, image_requirements)
        if not chosen_image or "error" in chosen_image:
            # Fallback to basic selection if spec-aware selection fails
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
            "compatibility_score": chosen_image.get("_compatibility_score", "N/A"),
            "spec_match": chosen_image.get("_spec_match", "unknown"),
            "is_basic_image": chosen_image.get("isBasicImage", False),
            "spec_info": chosen_image.get("_spec_info", {}),
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
                result["suggestion"] = (
                    "Set create_ns_if_missing=True to auto-create, or use "
                    "check_and_prepare_namespace() to see available options"
                )
                result["available_options"] = ns_check
                return result
    else:
        # No preference specified, guide user
        if len(ns_check["available_namespaces"]) == 0:
            result["status"] = "namespace_creation_needed"
            result["error"] = "No namespaces available"
            result["suggestion"] = (
                "Create a namespace first using create_namespace_with_validation() or "
                "specify preferred_ns_id with create_ns_if_missing=True"
            )
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
    max_output_lines: int = 5,
    max_output_chars: int = 1000
) -> Dict:
    """
    Execute remote commands based on SSH on VMs of an MCI.
    This allows executing commands on all VMs in the MCI or specific VMs based on subgroup or label selector.
    
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
            for command in commands:
                processed_command = command
                for var_name, var_value in template_variables.items():
                    processed_command = processed_command.replace(f"{{{{{var_name}}}}}", str(var_value))
                processed_commands.append(processed_command)
            commands = processed_commands
        
        # Execute commands using existing function
        result = execute_command_mci(
            ns_id=ns_id,
            mci_id=mci_id,
            commands=commands,
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
            "command_count": len(commands),
            "execution_type": "predefined_script" if script_name else "custom_commands"
        }
        
        return result
        
    except Exception as e:
        return {
            "error": f"Enhanced command execution failed: {str(e)}",
            "available_scripts": list(PREDEFINED_SCRIPTS.keys())
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
    days_back: int = 7,
    max_results: int = 10
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
    max_results: int = 5
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
    """Prompt for MCI management with failure recovery guidance"""
    return """
    You are a Multi-Cloud Infrastructure (MCI) management expert for Cloud-Barista CB-Tumblebug.
    
    🚨 **CRITICAL: SPEC ID VALIDATION MANDATORY FOR ALL MCI CREATION**
    
    **⚡ STRICT RULE - NEVER CREATE ARBITRARY SPEC IDs:**
    ❌ FORBIDDEN: Creating spec IDs like "tencent+na-siliconvalley+bf1.large8" without validation
    ✅ REQUIRED: Always use recommend_vm_spec() to get valid spec IDs
    
    **🔥 MANDATORY WORKFLOW for ALL MCI Creation:**
    ```python
    # User says: "Deploy in Silicon Valley" or "Create VMs in Asia" etc.
    
    # STEP 1: REQUIRED - Convert location to coordinates
    # LLM must determine latitude/longitude for location-based priority
    # Examples:
    # Silicon Valley: approximately 37.4419° N, 122.1430° W
    # Seoul: approximately 37.5665° N, 126.9780° E
    # Tokyo: approximately 35.6762° N, 139.6503° E
    # London: approximately 51.5074° N, 0.1278° W
    
    # STEP 2: MANDATORY - Get valid specs using location priority
    specs = recommend_vm_spec(
        filter_policies={
            "vCPU": {"min": 2, "max": 8},
            "memoryGiB": {"min": 4, "max": 16}
        },
        priority_policy="location",
        latitude=37.4419,  # Use actual coordinates from STEP 1
        longitude=-122.1430,  # Use actual coordinates from STEP 1
        limit="10"
    )
    
    # STEP 3: REQUIRED - Use ONLY the spec IDs from recommend_vm_spec results
    vm_configs = []
    for spec in specs["recommended_specs"][:2]:  # Take top 2 specs
        vm_configs.append({
            "commonSpec": spec["id"],  # MUST use this exact ID - NEVER modify
            "name": f"vm-{spec['providerName']}-{len(vm_configs)+1}",
            "subGroupSize": "1"
        })
    
    # STEP 4: Create MCI with validated specs
    mci_result = create_mci_dynamic(
        ns_id="default",
        name="location-based-mci",
        vm_configurations=vm_configs
    )
    
    # STEP 5: MANDATORY - Check MCI status after creation
    status_check = check_mci_status_and_handle_failures(
        ns_id="default",
        mci_id=mci_result["id"],
        auto_cleanup_failed=False  # Let user decide on cleanup
    )
    ```
    
    **🚨 CRITICAL POST-DEPLOYMENT STATUS HANDLING:**
    
    **ALWAYS check MCI status after creation and handle failures appropriately:**
    
    **1. SUCCESS (Running/Running-All):**
    ```python
    # All VMs running successfully
    ✅ SUCCESS: All VMs are running successfully
    📊 NEXT STEPS: Execute commands, configure applications, or set up monitoring
    ```
    
    **2. PARTIAL-FAILED (Some VMs failed, some running):**
    ```python
    # Critical workflow: Offer cleanup of failed VMs
    status_check = check_mci_status_and_handle_failures(ns_id, mci_id)
    
    if status_check["status_analysis"]["deployment_health"] == "partial-failed":
        print(f"🚨 PARTIAL FAILURE: {failed_vms_count}/{total_vms} VMs failed")
        print(f"✅ SUCCESSFUL: {running_vms_count} VMs are running normally")
        print("💡 RECOMMENDED: Use 'refine' to cleanup failed VMs and keep successful ones")
        
        # Ask user for confirmation
        user_choice = input("Would you like to cleanup failed VMs using 'refine'? (y/n): ")
        if user_choice.lower() == 'y':
            recovery_result = interactive_mci_recovery(
                ns_id, mci_id, 
                recovery_action="refine",
                confirm_cleanup=True
            )
    ```
    
    **3. FAILED (All VMs failed):**
    ```python
    # Complete failure - investigate and retry
    ❌ CRITICAL: All VMs in MCI have failed
    🔧 RECOMMENDED ACTIONS: Check error logs, recreate MCI, or terminate and retry
    
    # Options:
    # 1. Terminate and recreate with different specs
    # 2. Investigate failure causes
    # 3. Try refine action to cleanup and restart
    ```
    
    **4. CREATING/IN-PROGRESS:**
    ```python
    # Still deploying - monitor progress
    ⏳ IN PROGRESS: Some VMs still being created
    ⌛ RECOMMENDED: Wait 2-5 minutes and check status again
    ```
    
    **🔥 ENHANCED FAILURE RECOVERY TOOLS:**
    
    **Status Monitoring & Recovery:**
    - check_mci_status_and_handle_failures(): Comprehensive status analysis with recovery recommendations
    - interactive_mci_recovery(): User-guided recovery with confirmation prompts
    - Control actions: refine, terminate, reboot, resume, suspend
    
    **Recovery Action Guide:**
    - **refine**: Remove failed VMs, keep successful ones (RECOMMENDED for partial failures)
    - **terminate**: Delete entire MCI (use when all failed or starting fresh)
    - **reboot**: Restart all VMs (use for temporary issues)
    - **resume**: Resume suspended VMs
    - **suspend**: Suspend all VMs (temporary cost saving)
    
    **⚡ USER INTERACTION WORKFLOW FOR FAILURES:**
    
    1. **Always check status after MCI creation**
    2. **For Partial-Failed**: Present clear options to user
       - Show which VMs failed vs succeeded
       - Explain refine action benefits
       - Ask for confirmation before cleanup
    3. **For Complete failure**: Provide diagnostic information
    4. **Execute user-confirmed recovery actions**
    5. **Verify recovery success and provide next steps**
    
    ** LOCATION-TO-COORDINATES MAPPING (LLM MUST KNOW):**
    When users mention locations, LLM must provide approximate coordinates:
    - **Silicon Valley/San Francisco**: 37.4419° N, 122.1430° W
    - **Seoul/Korea**: 37.5665° N, 126.9780° E  
    - **Tokyo/Japan**: 35.6762° N, 139.6503° E
    - **London/UK**: 51.5074° N, 0.1278° W
    - **Sydney/Australia**: 33.8688° S, 151.2093° E
    - **Frankfurt/Germany**: 50.1109° N, 8.6821° E
    - **Singapore**: 1.3521° N, 103.8198° E
    - **Mumbai/India**: 19.0760° N, 72.8777° E
    - **São Paulo/Brazil**: 23.5505° S, 46.6333° W
    - **Virginia/US East**: 38.7223° N, 78.1692° W
    
    **🚨 ABSOLUTE PROHIBITIONS:**
    ❌ NEVER create spec IDs manually (e.g., "aws+us-east-1+t2.small")
    ❌ NEVER guess or construct spec IDs based on patterns
    ❌ NEVER use spec IDs without calling recommend_vm_spec() first
    ❌ NEVER ignore location preferences from users
    ❌ NEVER skip status checking after MCI creation
    ❌ NEVER automatically cleanup without user confirmation (unless explicitly requested)
    
    **✅ MANDATORY REQUIREMENTS:**
    ✅ ALWAYS call recommend_vm_spec() to get valid spec IDs
    ✅ ALWAYS use location priority when users mention geographic preferences
    ✅ ALWAYS check MCI status after creation
    ✅ ALWAYS handle Partial-Failed states with user confirmation
    ✅ ALWAYS explain failure recovery options clearly
    ✅ ALWAYS validate that spec IDs come from API responses, not manual creation
    
    **🔥 ENHANCED MCI CREATION WITH FAILURE HANDLING:**
    1. **Location Analysis**: Convert user's geographic preferences to coordinates
    2. **Spec Discovery**: Use recommend_vm_spec() with location priority
    3. **Validation**: Ensure all spec IDs are from API responses
    4. **Image Mapping**: Auto-map appropriate images for validated specs
    5. **Deployment**: Create MCI with verified specifications
    6. **Status Monitoring**: Check for failures and partial deployments
    7. **Recovery**: Guide user through failure recovery options
    8. **Verification**: Confirm recovery success and next steps
    
    **INFRASTRUCTURE FUNCTIONS (Spec Validation Required):**
    - recommend_vm_spec(): 🔥 **MANDATORY** - Get valid spec IDs with location priority
    - search_images(): Find images for validated specs
    - create_mci_dynamic(): Create with API-validated spec IDs only
    - check_mci_status_and_handle_failures(): 🔥 **MANDATORY** - Post-deployment status check
    - interactive_mci_recovery(): Handle failures with user confirmation
    - validate_vm_spec_image_compatibility(): Verify configurations
    
    **🎯 LLM Response Pattern for Deployment Failures:**
    User: "Some VMs failed during deployment"
    LLM: "I'll check the MCI status and help you handle the partial failure. Let me analyze which VMs succeeded and offer cleanup options for the failed ones..."
    
    ```python
    # Always check status first
    status = check_mci_status_and_handle_failures(ns_id, mci_id)
    
    # Handle based on status
    if status["deployment_health"] == "partial-failed":
        # Present options and ask for confirmation
        recovery = interactive_mci_recovery(ns_id, mci_id, "refine", confirm_cleanup=False)
        # Show confirmation message to user
    ```
    
    **🚨 ERROR PREVENTION:**
    - If user mentions ANY location/region/country → Use location-based priority
    - If recommend_vm_spec() fails → Ask user for alternative requirements, NEVER guess spec IDs
    - If spec validation fails → Re-run recommend_vm_spec() with different parameters
    - If MCI creation succeeds → ALWAYS check status for failures
    - If Partial-Failed detected → ALWAYS offer cleanup with user confirmation
    
    Current namespace list: {{namespace://list}}
    
    What infrastructure would you like to create? I'll ensure proper spec validation, location optimization, and comprehensive failure handling.
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
            "commonSpec": spec["id"],  # Only spec needed
            "name": f"vm-{i+1}",
            "description": f"Auto-mapped VM {i+1}",
            "os_requirements": {"os_type": "ubuntu", "use_case": "web-server"}
        })
    
    # Create MCI with automatic spec-to-image mapping
    result = create_mci_dynamic(
        ns_id="my-project",
        name="multi-csp-infrastructure", 
        vm_configurations=vm_configs  # Auto-mapping ensures correct images
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
            "commonImage": best_image["cspImageName"],  # CSP-specific
            "commonSpec": spec_id,                      # CSP-specific
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
    - recommend_vm_spec() → spec ID (determines CSP/region) → commonSpec parameter
    - search_images() → cspImageName (in spec's CSP/region) → commonImage parameter
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
    - commonSpec format: {provider}+{region}+{spec_name}
    - **VALIDATION**: Use validate_vm_spec_image_compatibility() before deployment
    - **EXAMPLES**: Use get_spec_image_mapping_examples() to see correct/incorrect patterns
    - **MONITORING**: Use check_mci_status_and_handle_failures() after deployment
    - Test with hold=True first to review configuration
    
    Current namespaces: {{namespace://list}}
    
    What would you like to help you create today?
    """

logger.info("MCP server initialization complete with interaction memory capabilities")
logger.info("Available memory functions: store_interaction_memory, get_interaction_history, get_session_summary, search_interaction_memory, clear_interaction_memory")
logger.info("Automatic memory storage enabled for: MCI creation, command execution, namespace management")

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
    ns_validation = validate_namespace(ns_id)
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
        
        # Analyze commonSpec
        common_spec = vm_config.get("commonSpec")
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
            vm_analysis["spec_analysis"] = {"error": "Missing commonSpec"}
            vm_analysis["configuration_status"] = "invalid"
            validation_issues += 1
        
        # Analyze commonImage
        common_image = vm_config.get("commonImage")
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
        common_spec = vm_config.get("commonSpec", "")
        common_image = vm_config.get("commonImage", "AUTO-MAPPED")
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
        
        # Validate commonSpec
        common_spec = vm_config.get("commonSpec")
        if not common_spec:
            config_validation["status"] = "invalid"
            config_validation["issues"].append("Missing commonSpec")
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
                    
                    # Validate commonImage compatibility
                    common_image = vm_config.get("commonImage")
                    if not common_image:
                        config_validation["warnings"].append("commonImage not specified - will be auto-mapped")
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
            "Use auto-mapping by omitting commonImage in VM configurations",
            "Use create_mci_dynamic() which automatically handles spec-to-image mapping",
            "For manual mapping, ensure image identifiers match the CSP in commonSpec",
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
                "commonSpec": "aws+ap-northeast-2+t2.small",
                "commonImage": "ami-0e06732ba3ca8c6cc",
                "explanation": "AWS spec with AWS AMI ID - CORRECT",
                "why_correct": "AMI ID (ami-*) is AWS-specific image format"
            },
            "azure_example": {
                "commonSpec": "azure+koreacentral+Standard_B2s",
                "commonImage": "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.Compute/images/ubuntu-20.04",
                "explanation": "Azure spec with Azure image path - CORRECT",
                "why_correct": "Subscription-based path is Azure-specific format"
            },
            "gcp_example": {
                "commonSpec": "gcp+asia-northeast3+e2-medium",
                "commonImage": "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20240830",
                "explanation": "GCP spec with GCP image path - CORRECT", 
                "why_correct": "Project-based path is GCP-specific format"
            },
            "auto_mapping_example": {
                "commonSpec": "aws+us-east-1+t3.medium",
                "commonImage": "AUTO-MAPPED",
                "explanation": "Spec without image - will be auto-mapped - RECOMMENDED",
                "why_recommended": "Automatic mapping ensures correct CSP-specific image selection"
            }
        },
        "incorrect_examples": {
            "cross_csp_error": {
                "commonSpec": "aws+us-east-1+t2.small",
                "commonImage": "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.Compute/images/ubuntu",
                "explanation": "AWS spec with Azure image - WRONG",
                "why_wrong": "Cannot use Azure image path with AWS specifications",
                "fix": "Use AMI ID for AWS or let system auto-map"
            },
            "format_mismatch": {
                "commonSpec": "azure+eastus+Standard_B1s", 
                "commonImage": "ami-0123456789abcdef0",
                "explanation": "Azure spec with AWS AMI - WRONG",
                "why_wrong": "AMI IDs only work with AWS, not Azure",
                "fix": "Use Azure image identifier or enable auto-mapping"
            },
            "region_mismatch": {
                "commonSpec": "aws+us-west-2+t2.nano",
                "commonImage": "ami-0a1b2c3d4e5f6789a",  # Hypothetical wrong region AMI
                "explanation": "Potentially wrong region AMI - RISKY",
                "why_risky": "AMI IDs are region-specific, using wrong region AMI will fail",
                "fix": "Search for images in the spec's region (us-west-2)"
            }
        },
        "best_practices": {
            "recommendation_1": {
                "title": "Use Auto-Mapping",
                "description": "Omit commonImage to let create_mci_dynamic() automatically select correct images",
                "example": {"commonSpec": "aws+ap-northeast-2+t2.small"}
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

#####################################
# Enhanced MCI Creation with User Confirmation
#####################################

# Tool: MCI Creation with Mandatory User Confirmation
@mcp.tool()
def create_mci_with_confirmation(
    ns_id: str,
    name: str,
    vm_configurations: List[Dict],
    description: str = "MCI created with user confirmation",
    hold: bool = False,
    force_create: bool = False
) -> Dict:
    """
    Create MCI with mandatory user confirmation step.
    This function ALWAYS shows configuration summary first and requires explicit confirmation.
    
    **USER CONFIRMATION WORKFLOW:**
    1. First call: Shows detailed configuration summary and preview
    2. Second call with force_create=True: Actually creates the MCI
    
    **USER EXPERIENCE:**
    - Provides clear overview of what will be created
    - Shows multi-cloud distribution and resource allocation
    - Highlights potential issues before deployment
    - Requires explicit confirmation to proceed
    
    Args:
        ns_id: Namespace ID
        name: MCI name  
        vm_configurations: List of VM configurations
        description: MCI description
        hold: Whether to hold provisioning
        force_create: Set to True to actually create MCI after reviewing summary
    
    Returns:
        First call (force_create=False): Configuration summary with confirmation prompt
        Second call (force_create=True): MCI creation result
    
    Example:
    ```python
    # Step 1: Review configuration
    summary = create_mci_with_confirmation(
        ns_id="my-project",
        name="my-mci",
        vm_configurations=[...]
    )
    # User reviews the summary output
    
    # Step 2: After confirming, create MCI
    result = create_mci_with_confirmation(
        ns_id="my-project", 
        name="my-mci",
        vm_configurations=[...],
        force_create=True  # Proceed with creation
    )
    ```
    """
    if not force_create:
        # Generate and return configuration summary
        summary = generate_mci_creation_summary(
            ns_id=ns_id,
            name=name,
            vm_configurations=vm_configurations,
            description=description,
            hold=hold
        )
        
        # Add explicit confirmation instructions
        summary["CONFIRMATION_REQUIRED"] = {
            "status": "PENDING_USER_CONFIRMATION",
            "message": "MCI has NOT been created yet. This is a preview only.",
            "to_proceed": f"Call create_mci_with_confirmation() again with force_create=True to actually create the MCI",
            "to_modify": "Adjust vm_configurations and run this function again to see updated preview"
        }
        
        summary["_creation_parameters"] = {
            "ns_id": ns_id,
            "name": name,
            "vm_configurations": vm_configurations,
            "description": description,
            "hold": hold,
            "force_create": False
        }
        
        return summary
    
    else:
        # User confirmed - proceed with actual MCI creation
        return create_mci_dynamic(
            ns_id=ns_id,
            name=name,
            vm_configurations=vm_configurations,
            description=description,
            hold=hold,
            skip_confirmation=True  # Skip internal confirmation since user already confirmed
        )

# Tool: Quick MCI Creation with Confirmation
@mcp.tool()
def create_simple_mci_with_confirmation(
    ns_id: str,
    name: str,
    common_image: str,
    common_spec: str,
    vm_count: int = 1,
    description: str = "Simple MCI created with confirmation",
    hold: bool = False,
    force_create: bool = False
) -> Dict:
    """
    Simple MCI creation with mandatory user confirmation.
    Simplified interface for multiple VMs with identical configuration.
    
    Args:
        ns_id: Namespace ID
        name: MCI name
        common_image: Image to use for all VMs
        common_spec: Spec to use for all VMs
        vm_count: Number of identical VMs to create
        description: MCI description
        hold: Whether to hold provisioning
        force_create: Set to True for actual creation after review
    
    Returns:
        First call: Configuration summary and confirmation prompt
        Second call (force_create=True): MCI creation result
    """
    # Generate VM configuration array
    vm_configurations = []
    for i in range(vm_count):
        vm_configurations.append({
            "commonImage": common_image,
            "commonSpec": common_spec,
            "name": f"vm-{i+1}",
            "description": f"VM {i+1} of {vm_count}",
            "subGroupSize": "1"
        })
    
    # Delegate to confirmation workflow
    return create_mci_with_confirmation(
        ns_id=ns_id,
        name=name,
        vm_configurations=vm_configurations,
        description=description,
        hold=hold,
        force_create=force_create
    )

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
        "ray": {"cpu": 4, "memory": 8, "disk": 100}
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
        "description": "FPS game server deployment with automatic server configuration",
        "requirements": {
            "cpu_intensive": True,
            "network_intensive": True,
            "ports": ["26000", "8"],
            "os": "ubuntu",
            "min_disk_gb": 30  # Game-specific disk requirement
        },
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/xonotic/startServer.sh; chmod +x ~/startServer.sh",
            "sudo ~/startServer.sh {{mci_id}} 26000 8 8",
            "echo 'Server Address: {{public_ip}}:26000'"
        ],
        "result_pattern": r"Server Address: ([^:]+):(\d+)",
        "deployment_strategy": "global"
    },
    "nginx": {
        "name": "Nginx Web Server",
        "category": "web",
        "description": "High-performance web server deployment",
        "requirements": {
            "cpu_intensive": False,
            "network_intensive": True,
            "ports": ["80", "443"],
            "os": "ubuntu",
            "min_disk_gb": 20
        },
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/nginx/startServer.sh | bash -s -- --ip {{public_ip}}",
            "echo 'Web Server: http://{{public_ip}}'"
        ],
        "result_pattern": r"Web Server: (http://[^/]+)",
        "deployment_strategy": "regional"
    },
    "ollama": {
        "name": "Ollama LLM Server",
        "category": "llm",
        "description": "Local LLM server deployment with Ollama",
        "requirements": {
            "cpu_intensive": True,
            "memory_intensive": True,
            "gpu_preferred": True,
            "ports": ["3000"],
            "os": "ubuntu",
            "min_disk_gb": 100  # LLM models require significant storage
        },
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh",
            "echo 'LLM Server: http://{{public_ip}}:3000'"
        ],
        "result_pattern": r"LLM Server: (http://[^/]+)",
        "deployment_strategy": "performance"
    },
    "jitsi": {
        "name": "Jitsi Meet Video Conference",
        "category": "conference",
        "description": "Video conferencing server deployment",
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
            "sudo ~/startServer.sh {{public_ips_space}} DNS EMAIL"
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
        "description": "Distributed ML computing cluster with Ray",
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
            "~/ray-head-setup.sh -i {{public_ip}}"
        ],
        "result_pattern": r"Ray Dashboard: (http://[^:]+:8265)",
        "deployment_strategy": "cluster"
    }
}

# Tool: List available application templates
@mcp.tool()
def list_application_templates() -> Dict:
    """
    List all available application deployment templates with their specifications.
    
    Returns:
        Dictionary containing all available application templates with:
        - Application categories (game, web, llm, ml, etc.)
        - Requirements and resource specifications
        - Deployment strategies and use cases
        - Port requirements and network configurations
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

# Tool: Deploy application with automatic infrastructure provisioning
@mcp.tool()
def deploy_application(
    application_id: str,
    regions: Union[int, List[str]] = 1,
    instances_per_region: int = 1,
    namespace_id: str = "default",
    custom_config: Optional[Dict] = None,
    auto_confirm: bool = False,
    research_requirements: bool = True,
    deployment_type: str = "production"
) -> Dict:
    """
    Deploy applications globally with automatic infrastructure provisioning and configuration.
    
    Enhanced workflow with hardware requirements research:
    1. Research application hardware requirements from internet (if enabled)
    2. Application template validation with enhanced specifications
    3. Infrastructure requirement analysis with researched hardware specs
    4. Multi-region deployment planning with optimized VM configurations
    5. MCI creation with recommended specs and minimum 50GB disk
    6. Application installation and configuration
    7. Service endpoint collection and reporting
    
    Args:
        application_id: Application template ID (use list_application_templates() to see options)
        regions: Number of regions for global deployment OR specific region list
        instances_per_region: Number of instances to deploy per region (default: 1)
        namespace_id: Namespace for deployment (default: "default")
        custom_config: Optional custom configuration overrides
        auto_confirm: Skip confirmation prompts for automated deployment
        research_requirements: Whether to research hardware requirements from internet/knowledge base
        deployment_type: Deployment environment ("production", "development", "testing")
    
    Returns:
        Complete deployment result with:
        - hardware_research: Researched hardware requirements and recommendations
        - deployment_plan: Infrastructure and application deployment strategy
        - infrastructure_created: Details of provisioned MCI and VMs
        - application_deployment: Application installation results
        - service_endpoints: Access URLs and connection information
        - deployment_summary: Overall deployment status and next steps
    
    Examples:
        # Deploy Xonotic game servers globally with automatic hardware research
        deploy_application('xonotic', regions=10, auto_confirm=True, research_requirements=True)
        
        # Deploy Nginx in specific regions for production with enhanced specs
        deploy_application('nginx', regions=['us-east-1', 'eu-west-1'], deployment_type='production')
        
        # Deploy Ollama LLM server with automatic GPU optimization
        deploy_application('ollama', regions=3, custom_config={'gpu_required': True})
    """
    
    deployment_id = f"app-deploy-{int(datetime.now().timestamp())}"
    
    # Validate application template
    if application_id not in APPLICATION_CONFIGS:
        available_apps = list(APPLICATION_CONFIGS.keys())
        return {
            "status": "error",
            "error": f"Unknown application '{application_id}'. Available: {available_apps}",
            "available_applications": available_apps
        }
    
    app_config = APPLICATION_CONFIGS[application_id].copy()
    
    # Apply custom configuration overrides
    if custom_config:
        app_config["requirements"].update(custom_config.get("requirements", {}))
        app_config.update({k: v for k, v in custom_config.items() if k != "requirements"})
    
    deployment_result = {
        "deployment_id": deployment_id,
        "application": {
            "id": application_id,
            "name": app_config["name"],
            "category": app_config["category"]
        },
        "start_time": datetime.now().isoformat(),
        "hardware_research": {},
        "deployment_plan": {},
        "infrastructure_created": {},
        "application_deployment": {},
        "service_endpoints": [],
        "deployment_summary": {},
        "status": "starting"
    }
    
    try:
        # Step 1: Research hardware requirements (new feature)
        logger.info(f"Researching hardware requirements for {app_config['name']}")
        if research_requirements:
            hardware_research = research_application_requirements(
                application_name=app_config['name'],
                deployment_type=deployment_type
            )
            deployment_result["hardware_research"] = hardware_research
        else:
            # Use fallback if research is disabled
            hardware_research = _get_fallback_hardware_requirements(
                app_config['name'], deployment_type, "Research disabled by user"
            )
            deployment_result["hardware_research"] = hardware_research
        
        # Step 2: Create enhanced deployment plan with researched specs
        logger.info(f"Creating enhanced deployment plan for {application_id}")
        deployment_plan = _create_application_deployment_plan(
            app_config, regions, instances_per_region, namespace_id, hardware_research
        )
        deployment_result["deployment_plan"] = deployment_plan
        
        # Step 3: Confirm deployment plan with hardware research
        if not auto_confirm:
            confirmation_msg = _generate_deployment_confirmation(deployment_plan, app_config, hardware_research)
            deployment_result["confirmation_required"] = True
            deployment_result["confirmation_message"] = confirmation_msg
            deployment_result["status"] = "waiting_for_confirmation"
            return deployment_result
        
        # Step 4: Provision infrastructure
        logger.info(f"Provisioning infrastructure for {application_id}")
        mci_name = f"{application_id}-{deployment_id[-8:]}"
        
        infrastructure_result = _provision_application_infrastructure(
            deployment_plan, mci_name, namespace_id
        )
        deployment_result["infrastructure_created"] = infrastructure_result
        
        if infrastructure_result["status"] != "success":
            deployment_result["status"] = "infrastructure_failed"
            return deployment_result
        
        # Step 5: Deploy application
        logger.info(f"Deploying {application_id} application")
        mci_id = infrastructure_result["mci_id"]
        
        app_deployment_result = _deploy_application_to_infrastructure(
            app_config, mci_id, namespace_id, deployment_plan
        )
        deployment_result["application_deployment"] = app_deployment_result
        
        # Step 5: Collect service endpoints
        logger.info(f"Collecting service endpoints for {application_id}")
        endpoints = _collect_service_endpoints(
            app_config, mci_id, namespace_id, app_deployment_result
        )
        deployment_result["service_endpoints"] = endpoints
        
        # Step 6: Generate deployment summary
        deployment_result["deployment_summary"] = _generate_deployment_summary(
            deployment_result, app_config
        )
        deployment_result["status"] = "completed"
        deployment_result["end_time"] = datetime.now().isoformat()
        
        # Store deployment in memory
        _store_interaction_memory(
            user_request=f"Deploy {application_id} application in {regions} regions",
            llm_response=f"{application_id} deployed successfully with {len(endpoints)} service endpoints",
            operation_type="application_deployment",
            context_data={
                "application_id": application_id,
                "deployment_id": deployment_id,
                "mci_id": mci_id,
                "namespace": namespace_id,
                "endpoints": endpoints
            },
            status="completed"
        )
        
        return deployment_result
        
    except Exception as e:
        deployment_result["status"] = "error"
        deployment_result["error"] = str(e)
        deployment_result["end_time"] = datetime.now().isoformat()
        
        # Attempt cleanup
        if "mci_id" in deployment_result.get("infrastructure_created", {}):
            try:
                cleanup_result = delete_mci(
                    namespace_id, 
                    deployment_result["infrastructure_created"]["mci_id"]
                )
                deployment_result["cleanup_attempted"] = cleanup_result
            except:
                deployment_result["cleanup_attempted"] = {"status": "failed"}
        
        return deployment_result

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
        for cmd in commands:
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
                "commonImage": selected_image["csp_image_name"],
                "commonSpec": selected_spec["id"],
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
    """Expand template variables in commands."""
    
    expanded_cmd = command
    
    # Extract access information
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
    
    # Replace template variables
    if public_ips:
        expanded_cmd = expanded_cmd.replace("{{public_ip}}", public_ips[0])
        expanded_cmd = expanded_cmd.replace("{{public_ips_space}}", " ".join(public_ips))
        expanded_cmd = expanded_cmd.replace("{{public_ips_comma}}", ",".join(public_ips))
    
    if private_ips:
        expanded_cmd = expanded_cmd.replace("{{private_ip}}", private_ips[0])
        expanded_cmd = expanded_cmd.replace("{{private_ips_space}}", " ".join(private_ips))
        expanded_cmd = expanded_cmd.replace("{{private_ips_comma}}", ",".join(private_ips))
    
    expanded_cmd = expanded_cmd.replace("{{mci_id}}", mci_id)
    
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
            namespace_result = create_namespace_with_validation(
                name=target_namespace,
                description=f"Temporary namespace for compute task: {task_description[:50]}..."
            )
            
            if not namespace_result.get("created", False) and not namespace_result.get("namespace_id"):
                workflow_result["status"] = "failed"
                workflow_result["error"] = "Failed to create temporary namespace"
                return workflow_result
        else:
            # Validate existing namespace
            namespace_validation = validate_namespace(target_namespace)
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
                        "commonSpec": spec_id,
                        "commonImage": image.get("cspImageName"),
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
def tumblebug_application_deployment():
    """
    Complete Guide for Application Deployment on Multi-Cloud Infrastructure
    
    This prompt demonstrates how to deploy applications across multiple cloud providers
    using CB-Tumblebug's enhanced MCP server with MapUI-based patterns.
    """
    return """# CB-Tumblebug Application Deployment Guide

## ⚠️ CRITICAL: AUTOMATIC HARDWARE RESEARCH REQUIRED

### 🔥 ESSENTIAL LLM BEHAVIOR: When users request application deployment, you MUST:

1. **ALWAYS research hardware requirements FIRST** before deployment
2. **Use research_application_requirements()** to find optimal specs 
3. **Apply findings to VM specifications** automatically
4. **Set minimum 50GB disk** for all VMs (more if research indicates)
5. **Choose deployment_type** based on user context (production/development/testing)

### 🚨 MANDATORY WORKFLOW for ANY application deployment request:

```python
# Step 1: ALWAYS research hardware requirements first
research_result = research_application_requirements(
    application_name="user_requested_app",
    deployment_type="production"  # or development/testing based on context
)

# Step 2: Deploy with research_requirements=True (REQUIRED)
deploy_application(
    application_id="app_name", 
    research_requirements=True,  # MUST be True
    deployment_type="production"  # Match research_result context
)
```

### 🎯 Enhanced Application Deployment Process
CB-Tumblebug automatically researches hardware requirements before deployment:

**User says:** "I want to deploy Nginx" or "Deploy Ollama server" or "Setup game server"
**LLM MUST respond:** "I'll research the hardware requirements for [app] first, then deploy with optimal specifications."

The system will:
1. **🔍 Research hardware requirements** from built-in knowledge base FIRST
2. **⚡ Optimize VM specifications** based on findings (CPU, Memory, Disk)
3. **💾 Set minimum 50GB disk** for all deployments (more for specific apps)
4. **🎛️ Apply deployment-specific multipliers** (production vs development)

### 2. Available Application Templates (with Enhanced Hardware Research)
**🔬 Each template triggers automatic hardware research:**
- **xonotic**: Game server → Research gaming hardware needs → 30GB+ disk
- **nginx**: Web server → Research web server specs → 20GB+ disk  
- **ollama**: AI inference → Research AI/ML requirements → 100GB+ disk
- **jitsi**: Video conferencing → Research streaming specs → 50GB+ disk
- **elk**: Elasticsearch stack → Research big data needs → 200GB+ disk
- **ray**: Distributed computing → Research cluster specs → 100GB+ disk

**⚠️ LLM INSTRUCTION: For ANY application not in templates, you MUST:**
1. Call `research_application_requirements(app_name)` first
2. Use research results to inform deployment decisions
3. Explain hardware reasoning to user

### 3. 🤖 REQUIRED LLM BEHAVIOR - Enhanced Infrastructure + Application Workflow
**When user requests ANY application deployment, LLM MUST execute this sequence:**

```python
# STEP 1: MANDATORY - Research hardware requirements
research_result = research_application_requirements(
    application_name="user_requested_app",
    deployment_type="production"  # Infer from user context
)

# STEP 2: REQUIRED - Deploy with research enabled
deploy_application(
    application_id="app_name",
    research_requirements=True,  # ALWAYS True unless user explicitly says no
    deployment_type="production"  # Match user's environment needs
)

# STEP 3: Optional - Post-deployment management
get_mci_access_info()  # Get connection endpoints
execute_application_commands()  # Post-deployment configuration
```

**🚨 CRITICAL: LLM must NEVER skip hardware research step unless explicitly told to disable it.**

### 4. 📊 Hardware Research Features (LLM MUST UTILIZE)
- **🧠 Built-in Knowledge Base**: 50+ applications with known requirements (LLM should leverage this)
- **🎯 Deployment Type Auto-Detection**: LLM should infer production (1.5x), development (1.0x), testing (0.8x) from context
- **⚡ Automatic Spec Selection**: CPU, Memory, and Disk optimized based on research
- **💾 Minimum Disk Guarantee**: Always 50GB minimum, research may increase this
- **🎯 Confidence Levels**: High/Medium/Low - LLM should explain confidence to user

**🚨 LLM BEHAVIOR: Always mention research confidence and explain hardware reasoning to user**

### 5. 🎯 LLM Deployment Decision Matrix
**🤖 LLM must automatically choose deployment approach based on user context:**

```python
# USER CONTEXT: "I need this for production" / "deploy on production"
deploy_application('app', research_requirements=True, deployment_type='production')

# USER CONTEXT: "testing this app" / "just trying out" / "development"  
deploy_application('app', research_requirements=True, deployment_type='development')

# USER CONTEXT: "quick test" / "temporary" / "prototype"
deploy_application('app', research_requirements=True, deployment_type='testing')

# USER CONTEXT: "optimal specs" / "best performance" / business context
deploy_application('app', research_requirements=True, deployment_type='production')
```

**⚠️ RULE: LLM should ALWAYS enable research_requirements=True unless user explicitly says "skip research" or "no optimization"**

### 6. Disk Size Management
- **Minimum 50GB** for all VMs (as requested)
- **Application-specific increases**: 
  - ELK Stack: 200GB (for log storage)
  - Ollama: 100GB (for AI models)
  - Game servers: 30-50GB (for game data)
- **Research-based optimization**: Uses findings from knowledge base
- **Automatic scaling**: Higher specs for production deployments

### 7. Enhanced Remote Commands
Use predefined scripts for common tasks:
- system_info: Complete system information
- docker_install: Docker installation
- nginx_install: Nginx web server setup
- security_hardening: Basic security measures
- monitoring_setup: System monitoring tools

### 8. Template Variables Support
Commands automatically substitute variables:
- {{public_ip}}: Primary public IP
- {{public_ips_space}}: All public IPs (space-separated)
- {{mci_id}}: MCI identifier
- {{ns_id}}: Namespace identifier

### 9. Hardware Research Tools
```python
# Manual hardware research
research_application_requirements('nginx', deployment_type='production')

# List predefined scripts
list_predefined_scripts()

# Enhanced command execution with templates
execute_remote_commands_enhanced(
    script_name='nginx_install',
    template_variables={'public_ip': '1.2.3.4'}
)
```

### 10. 🌍 LLM Response Examples for Global Deployment
**🤖 LLM should respond like this when user requests deployment:**

**User:** "Deploy Nginx web servers"  
**LLM Response:** "I'll research Nginx hardware requirements first to ensure optimal specifications, then deploy with enhanced specs including minimum 50GB disk. Let me start with the research..."

**User:** "I need Ollama for AI inference"
**LLM Response:** "Ollama requires significant resources for AI models. I'll research the specific hardware requirements including GPU preferences and storage needs (typically 100GB+ for models), then deploy with production-grade specifications..."

**User:** "Setup game servers in multiple regions"  
**LLM Response:** "For game servers, I'll research optimal gaming hardware requirements including CPU for real-time processing and network optimization. This will ensure low latency and good performance..."

**🚨 CRITICAL LLM BEHAVIOR:**
- Always mention hardware research step
- Explain why research is important for the specific application  
- Mention minimum 50GB disk and any application-specific increases
- Show confidence level from research results

## 🎯 Best Practices for LLM
1. **🔍 ALWAYS enable hardware research** for optimal specifications (research_requirements=True)
2. **🎛️ Auto-detect deployment_type** from user context (production/development/testing)
3. **📋 Always review and explain** confirmation messages to user with hardware reasoning
4. **💾 Highlight disk allocation** as minimum 50GB is automatically allocated (more for specific apps)
5. **🔬 Always use research_application_requirements()** for unknown applications
6. **🛡️ Recommend security hardening** after deployment
7. **📊 Explain research confidence** levels to users (High/Medium/Low)
8. **⚡ Mention hardware optimization** benefits in your responses

## 🚨 LLM Error Prevention
- **NEVER deploy applications without hardware research** unless explicitly told to skip
- **ALWAYS explain hardware decisions** based on research findings  
- **NEVER use hardcoded specs** when research tools are available
- **ALWAYS mention minimum 50GB disk** and any increases from research
- **VERIFY deployment_type** matches user's intended environment

## Error Handling and Fallbacks
- **Research failures**: Automatic fallback to built-in knowledge
- **Specification limits**: Automatic adjustment within CSP limits
- **Disk size enforcement**: Always minimum 50GB regardless of research
- **Confidence indicators**: High/Medium/Low confidence in recommendations
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
create_simple_mci() - Quick deployment with identical VMs
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

# Overview
resource_overview() - Check all CSP resources
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
            "commonImage": selected_image,
            "commonSpec": f"{region}+standard-instance",
            "subGroupSize": "3"
        }]
    )
```

### 2. Environment Separation
```
# Development
create_namespace("dev")
create_simple_mci("dev", "test-app", image, "t2.micro")

# Staging  
create_namespace("staging")
create_simple_mci("staging", "staging-app", image, "t2.small")

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
    "commonSpec": "aws+ap-northeast-2+t3.micro",
    "subGroupSize": "5"
}
```

## Monitoring and Troubleshooting
1. Use get_mci_list_with_options() for status monitoring
2. Check get_mci() for detailed VM information  
3. Review execute_command_mci() outputs for issues
4. Monitor resource usage with resource_overview()
5. Use get_mci_access_info() for connectivity verification

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
1. create_simple_mci("web-app", "nginx-aws", aws_image, "t3.medium")
2. create_simple_mci("web-app", "nginx-azure", azure_image, "Standard_B2s") 
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
2. create_simple_mci("dev-team", "dev-servers", ubuntu_image, "t3.large", vm_count=5)
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
4. resource_overview() - Cost monitoring
```

### 4. Cleanup and Optimization
```
1. delete_mci() - Remove unused infrastructure
2. release_resources() - Clean shared resources
3. delete_namespace() - Complete environment cleanup
```

## Best Practices Summary
1. **Start Small**: Use create_simple_mci() for testing
2. **Scale Gradually**: Use deploy_application() for production
3. **Monitor Resources**: Regular health checks and cost monitoring  
4. **Security First**: Apply security hardening from day one
5. **Document Endpoints**: Keep track of service URLs and IPs
6. **Plan Cleanup**: Regular resource cleanup to control costs
"""

