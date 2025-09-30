#!/usr/bin/env python3
"""
CB-Tumblebug API Change Analyzer

This script analyzes differences between two versions of CB-Tumblebug API
by comparing their Swagger/OpenAPI specifications. It generates detailed
reports highlighting breaking changes, new endpoints, and modifications.

Features:
- Local git repository analysis (preferred)
- GitHub repository fallback
- Comprehensive diff generation
- Breaking change detection
- Markdown report generation
"""

import os
import sys
import json
import argparse
import subprocess
import tempfile
from datetime import datetime
from typing import Dict, List, Set, Tuple, Any
import difflib

# Try to import requests for GitHub fallback, but make it optional
try:
    import requests
    HAS_REQUESTS = True
except ImportError:
    HAS_REQUESTS = False

class SwaggerDiffAnalyzer:
    """
    Analyzes differences between two Swagger/OpenAPI specifications.
    
    This class provides comprehensive analysis of API changes including:
    - Endpoint additions, removals, and modifications
    - Schema changes and property modifications
    - Breaking change detection
    - Detailed diff generation
    """
    
    def __init__(self, old_version=None, new_version=None):
        """Initialize the analyzer with version information"""
        self.old_spec = None
        self.new_spec = None
        self.old_version_name = old_version  # Version name provided by user
        self.new_version_name = new_version  # Version name provided by user
        # Dictionary to store all detected changes
        self.changes = {
            'version_change': [],
            'path_changes': [],
            'new_endpoints': [],
            'removed_endpoints': [],
            'modified_endpoints': [],
            'new_schemas': [],
            'removed_schemas': [],
            'modified_schemas': [],
            'new_categories': []
        }

    def get_swagger_spec(self, version: str) -> Dict:
        """
        Get swagger spec from local git or GitHub repository.
        
        Priority order:
        1. Local git repository (preferred for speed and accuracy)
        2. GitHub repository (fallback when local git fails)
        """
        # Try to get from local git repository first
        local_spec = self.fetch_local_git_swagger_spec(version)
        if local_spec:
            return local_spec
        
        # Fallback to GitHub if local git fails
        print(f"Local git not available, fetching from GitHub...")
        return self.fetch_swagger_spec(version)

    def fetch_local_git_swagger_spec(self, version: str) -> Dict:
        """
        Fetch swagger spec from local git repository using git show command.
        
        This method is preferred over GitHub API because:
        - Faster access to local files
        - Works offline
        - Includes uncommitted changes when comparing with working directory
        """
        try:
            # Find git repository root
            script_dir = os.path.dirname(os.path.abspath(__file__))
            repo_root = self.find_git_root(script_dir)
            if not repo_root:
                return None
            
            print(f"Fetching swagger spec for {version} from local git...")
            
            # Try different possible paths for swagger.json
            possible_paths = [
                "src/api/rest/docs/swagger.json",      # newer versions
                "src/interface/rest/docs/swagger.json" # older versions (v0.11.8)
            ]
            
            # Check if version is a branch/tag that exists
            try:
                subprocess.run(['git', 'show', f'{version}:README.md'], 
                             cwd=repo_root, capture_output=True, check=True)
            except subprocess.CalledProcessError:
                print(f"Version {version} not found in local git, trying GitHub...")
                return None
            
            # Try each possible path until we find swagger.json
            for path in possible_paths:
                try:
                    # Use git show to get file content from specific version
                    result = subprocess.run(['git', 'show', f'{version}:{path}'], 
                                          cwd=repo_root, capture_output=True, text=True, check=True)
                    return json.loads(result.stdout)
                except subprocess.CalledProcessError:
                    continue
                except json.JSONDecodeError:
                    continue
            
            return None
            
        except Exception as e:
            if hasattr(self, 'verbose') and self.verbose:
                print(f"Local git fetch failed: {e}")
            return None

    def find_git_root(self, start_path: str) -> str:
        """
        Find the root directory of the git repository.
        Traverses up the directory tree looking for .git directory.
        """
        current_path = start_path
        while current_path != '/':
            if os.path.exists(os.path.join(current_path, '.git')):
                return current_path
            current_path = os.path.dirname(current_path)
        return None
        
    def fetch_swagger_spec(self, version: str) -> Dict:
        """
        Fetch swagger spec from GitHub repository as fallback.
        Used when local git repository is not available.
        """
        if not HAS_REQUESTS:
            print(f"Error: requests library not available. Install with: pip install requests")
            print(f"Or use local files with --local-old and --local-new options")
            sys.exit(1)
            
        base_url = "https://raw.githubusercontent.com/cloud-barista/cb-tumblebug"
        
        # Try different possible paths for swagger.json
        possible_paths = [
            "src/api/rest/docs/swagger.json",      # newer versions
            "src/interface/rest/docs/swagger.json" # older versions (v0.11.8)
        ]
        
        print(f"Fetching swagger spec for {version}...")
        
        # Try each path until we find the swagger.json file
        for path in possible_paths:
            url = f"{base_url}/{version}/{path}"
            try:
                response = requests.get(url, timeout=30)
                if response.status_code == 200:
                    return response.json()
            except requests.RequestException:
                continue
            except json.JSONDecodeError:
                continue
        
        # If all paths failed
        print(f"Error: Could not find swagger.json for {version} in any of these paths:")
        for path in possible_paths:
            print(f"  - {path}")
        sys.exit(1)

    def load_local_swagger_spec(self, file_path: str) -> Dict:
        """Load swagger spec from local file"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                return json.load(f)
        except FileNotFoundError:
            print(f"Error: File not found: {file_path}")
            sys.exit(1)
        except json.JSONDecodeError as e:
            print(f"Error parsing JSON file {file_path}: {e}")
            sys.exit(1)

    def analyze_all_changes(self):
        """
        Perform comprehensive analysis of all API changes.
        This is the main analysis orchestrator that coordinates all
        change detection activities.
        """
        print(f"Analyzing version changes...")
        self.analyze_version_changes()
        print(f"Analyzing endpoint changes...")
        self.analyze_endpoint_changes()
        print(f"Analyzing schema changes...")
        self.analyze_schema_changes()
        print(f"Analyzing new categories...")
        self.analyze_new_categories()

    def analyze_version_changes(self):
        """Analyze version information changes using user-provided version names"""
        # Use version names provided by user
        old_version = self.old_version_name if self.old_version_name else 'unknown'
        new_version = self.new_version_name if self.new_version_name else 'unknown'
        
        # Record as change if versions are different
        if old_version != new_version:
            self.changes['version_change'] = [old_version, new_version]
        else:
            self.changes['version_change'] = [old_version, new_version]

    def find_moved_endpoints(self, old_paths: Set[Tuple[str, str]], new_paths: Set[Tuple[str, str]]) -> List[Dict]:
        """Find endpoints that have been moved to different paths or significantly changed"""
        moved_endpoints = []
        
        # Get truly new and removed endpoints
        truly_new = new_paths - old_paths
        truly_removed = old_paths - new_paths
        
        # For each removed endpoint, try to find a matching new endpoint
        for old_path, old_method in truly_removed.copy():
            old_endpoint = self.old_spec['paths'][old_path][old_method.lower()]
            old_signature = self.get_endpoint_signature(old_endpoint)
            
            for new_path, new_method in truly_new.copy():
                # Allow different HTTP methods for more comprehensive detection
                new_endpoint = self.new_spec['paths'][new_path][new_method.lower()]
                new_signature = self.get_endpoint_signature(new_endpoint)
                
                # Calculate similarity score based on operational similarity
                similarity = self.calculate_endpoint_similarity(old_path, old_method, old_endpoint, 
                                                             new_path, new_method, new_endpoint)
                
                # If similarity is high enough, consider it a moved/changed endpoint
                if similarity > 0.6:  # Lowered threshold for better detection
                    # Get detailed differences
                    endpoint_diff = self.get_detailed_endpoint_diff(
                        old_path, old_method, old_endpoint,
                        new_path, new_method, new_endpoint
                    )
                    
                    moved_endpoints.append({
                        'old_path': old_path,
                        'new_path': new_path,
                        'old_method': old_method,
                        'new_method': new_method,
                        'similarity': similarity,
                        'diff_details': endpoint_diff,
                        'old_endpoint': old_endpoint,
                        'new_endpoint': new_endpoint
                    })
                    
                    # Remove from new/removed lists
                    truly_removed.discard((old_path, old_method))
                    truly_new.discard((new_path, new_method))
                    break
        
        return moved_endpoints

    def get_endpoint_signature(self, endpoint: Dict) -> Dict:
        """Generate a signature for an endpoint to help identify moves"""
        return {
            'summary': endpoint.get('summary', ''),
            'operationId': endpoint.get('operationId', ''),
            'tags': tuple(sorted(endpoint.get('tags', []))),
            'request_body_schema': self.get_request_body_schema_ref(endpoint),
            'response_schemas': self.get_response_schemas(endpoint),
            'param_count': len(endpoint.get('parameters', [])),
            'param_names': tuple(sorted([p.get('name', '') for p in endpoint.get('parameters', [])]))
        }

    def get_request_body_schema_ref(self, endpoint: Dict) -> str:
        """Extract schema reference from request body"""
        request_body = endpoint.get('requestBody', {})
        content = request_body.get('content', {})
        
        for media_type, media_content in content.items():
            schema = media_content.get('schema', {})
            if '$ref' in schema:
                return schema['$ref']
            elif schema.get('type') == 'array' and 'items' in schema and '$ref' in schema['items']:
                return schema['items']['$ref']
        
        return ""

    def get_response_schemas(self, endpoint: Dict) -> List[str]:
        """Extract schema references from responses"""
        schemas = []
        responses = endpoint.get('responses', {})
        
        for code, response in responses.items():
            content = response.get('content', {})
            for media_type, media_content in content.items():
                schema = media_content.get('schema', {})
                if '$ref' in schema:
                    schemas.append(schema['$ref'])
                elif schema.get('type') == 'array' and 'items' in schema and '$ref' in schema['items']:
                    schemas.append(schema['items']['$ref'])
        
        return sorted(schemas)

    def calculate_endpoint_similarity(self, old_path: str, old_method: str, old_endpoint: Dict,
                                    new_path: str, new_method: str, new_endpoint: Dict) -> float:
        """Calculate comprehensive similarity between two endpoints"""
        score = 0.0
        total_weight = 0.0
        
        # Path similarity (weight: 1.5)
        path_sim = self.path_similarity(old_path, new_path)
        score += path_sim * 1.5
        total_weight += 1.5
        
        # Method similarity (weight: 1.0)
        method_sim = 1.0 if old_method == new_method else 0.3  # Some similarity for different methods
        score += method_sim * 1.0
        total_weight += 1.0
        
        # Summary similarity (weight: 2.0)
        old_summary = old_endpoint.get('summary', '')
        new_summary = new_endpoint.get('summary', '')
        if old_summary and new_summary:
            summary_sim = self.string_similarity(old_summary, new_summary)
            score += summary_sim * 2.0
            total_weight += 2.0
        
        # OperationId similarity (weight: 1.5)
        old_op_id = old_endpoint.get('operationId', '')
        new_op_id = new_endpoint.get('operationId', '')
        if old_op_id and new_op_id:
            op_id_sim = self.string_similarity(old_op_id, new_op_id)
            score += op_id_sim * 1.5
            total_weight += 1.5
        
        # Tags similarity (weight: 1.0)
        old_tags = set(old_endpoint.get('tags', []))
        new_tags = set(new_endpoint.get('tags', []))
        if old_tags or new_tags:
            tags_sim = len(old_tags & new_tags) / len(old_tags | new_tags) if old_tags | new_tags else 0
            score += tags_sim * 1.0
            total_weight += 1.0
        
        return score / total_weight if total_weight > 0 else 0.0

    def path_similarity(self, path1: str, path2: str) -> float:
        """Calculate similarity between two paths"""
        # Remove path parameters for comparison
        clean_path1 = self.clean_path_for_comparison(path1)
        clean_path2 = self.clean_path_for_comparison(path2)
        
        # Direct string similarity
        return self.string_similarity(clean_path1, clean_path2)

    def clean_path_for_comparison(self, path: str) -> str:
        """Clean path by replacing parameters with placeholders"""
        import re
        # Replace {param} with generic placeholder
        return re.sub(r'\{[^}]+\}', '{param}', path)

    def get_detailed_endpoint_diff(self, old_path: str, old_method: str, old_endpoint: Dict,
                                 new_path: str, new_method: str, new_endpoint: Dict) -> Dict:
        """Get detailed differences between two endpoints"""
        diff_info = {
            'path_changed': old_path != new_path,
            'method_changed': old_method != new_method,
            'summary_changed': old_endpoint.get('summary') != new_endpoint.get('summary'),
            'description_changed': old_endpoint.get('description') != new_endpoint.get('description'),
            'operation_id_changed': old_endpoint.get('operationId') != new_endpoint.get('operationId'),
            'tags_changed': set(old_endpoint.get('tags', [])) != set(new_endpoint.get('tags', [])),
            'parameters_changed': self.parameters_changed(old_endpoint, new_endpoint),
            'responses_changed': self.responses_changed(old_endpoint, new_endpoint),
            'request_body_changed': self.request_body_changed(old_endpoint, new_endpoint),
            'raw_diff': self.generate_endpoint_raw_diff(old_path, old_method, old_endpoint,
                                                      new_path, new_method, new_endpoint)
        }
        
        return diff_info

    def parameters_changed(self, old_endpoint: Dict, new_endpoint: Dict) -> bool:
        """Check if parameters have changed"""
        old_params = old_endpoint.get('parameters', [])
        new_params = new_endpoint.get('parameters', [])
        
        # Compare parameter signatures
        old_param_sigs = [(p.get('name'), p.get('in'), p.get('type')) for p in old_params]
        new_param_sigs = [(p.get('name'), p.get('in'), p.get('type')) for p in new_params]
        
        return sorted(old_param_sigs) != sorted(new_param_sigs)

    def responses_changed(self, old_endpoint: Dict, new_endpoint: Dict) -> bool:
        """Check if responses have changed"""
        old_responses = old_endpoint.get('responses', {})
        new_responses = new_endpoint.get('responses', {})
        
        return old_responses.keys() != new_responses.keys()

    def request_body_changed(self, old_endpoint: Dict, new_endpoint: Dict) -> bool:
        """Check if request body has changed"""
        old_body = old_endpoint.get('requestBody', {})
        new_body = new_endpoint.get('requestBody', {})
        
        if not old_body and not new_body:
            return False
        if bool(old_body) != bool(new_body):
            return True
            
        # Compare schema references
        old_schema_ref = self.get_request_body_schema_ref(old_endpoint)
        new_schema_ref = self.get_request_body_schema_ref(new_endpoint)
        
        return old_schema_ref != new_schema_ref

    def generate_endpoint_raw_diff(self, old_path: str, old_method: str, old_endpoint: Dict,
                                 new_path: str, new_method: str, new_endpoint: Dict) -> str:
        """Generate raw diff showing the actual changes"""
        import json
        from textwrap import indent
        
        # Create simplified representations for comparison
        old_repr = {
            'path': old_path,
            'method': old_method.upper(),
            'summary': old_endpoint.get('summary', ''),
            'description': self._safe_description_for_diff(old_endpoint.get('description', '')),
            'operationId': old_endpoint.get('operationId', ''),
            'tags': old_endpoint.get('tags', []),
            'parameters': [
                {
                    'name': p.get('name'),
                    'in': p.get('in'),
                    'type': p.get('type'),
                    'required': p.get('required')
                } for p in old_endpoint.get('parameters', [])
            ],
            'responses': list(old_endpoint.get('responses', {}).keys())
        }
        
        new_repr = {
            'path': new_path,
            'method': new_method.upper(),
            'summary': new_endpoint.get('summary', ''),
            'description': self._safe_description_for_diff(new_endpoint.get('description', '')),
            'operationId': new_endpoint.get('operationId', ''),
            'tags': new_endpoint.get('tags', []),
            'parameters': [
                {
                    'name': p.get('name'),
                    'in': p.get('in'),
                    'type': p.get('type'),
                    'required': p.get('required')
                } for p in new_endpoint.get('parameters', [])
            ],
            'responses': list(new_endpoint.get('responses', {}).keys())
        }
        
        # Generate JSON diff
        old_json = json.dumps(old_repr, indent=2, sort_keys=True, ensure_ascii=False)
        new_json = json.dumps(new_repr, indent=2, sort_keys=True, ensure_ascii=False)
        
        old_lines = old_json.splitlines()
        new_lines = new_json.splitlines()
        
        diff_lines = []
        for line in difflib.unified_diff(old_lines, new_lines, 
                                       fromfile=f"{old_method.upper()} {old_path}",
                                       tofile=f"{new_method.upper()} {new_path}",
                                       lineterm=''):
            diff_lines.append(line)
        
        return '\n'.join(diff_lines)

    def _safe_description_for_diff(self, description: str) -> str:
        """Make description safe for diff display by handling XML and long content"""
        if not description:
            return ""
        
        # Truncate very long descriptions
        if len(description) > 200:
            description = description[:200] + "... (truncated for diff)"
        
        # Replace XML code blocks that could interfere with markdown rendering
        description = description.replace('```xml', '[XML Example]')
        description = description.replace('```', '')
        
        # Replace newlines that could break JSON
        description = description.replace('\n', ' ')
        description = description.replace('\r', ' ')
        
        # Clean up multiple spaces
        import re
        description = re.sub(r'\s+', ' ', description).strip()
        
        return description

    def has_breaking_changes(self, endpoint_change: Dict) -> bool:
        """Determine if an endpoint change is breaking"""
        changes = endpoint_change.get('changes', {})
        
        # Parameter removal is always breaking
        param_changes = changes.get('parameter_changes', {})
        if param_changes.get('removed_params'):
            return True
        
        # Required parameter addition is breaking
        if param_changes.get('added_params'):
            for param in param_changes['added_params']:
                if param.get('required', False):
                    return True
        
        # Request body schema changes can be breaking
        if changes.get('request_body_changes'):
            return True
        
        # Response schema changes might be breaking (especially removing fields)
        if changes.get('response_changes'):
            return True
        
        return False

    def string_similarity(self, s1: str, s2: str) -> float:
        """Calculate similarity between two strings"""
        if not s1 or not s2:
            return 0.0
        
        # Use SequenceMatcher for string similarity
        matcher = difflib.SequenceMatcher(None, s1.lower(), s2.lower())
        return matcher.ratio()

    def analyze_endpoint_changes(self):
        """Analyze changes in API endpoints"""
        old_paths = set()
        new_paths = set()
        
        # Extract all endpoints
        for path, methods in self.old_spec.get('paths', {}).items():
            for method in methods.keys():
                if method.upper() in ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']:
                    old_paths.add((path, method.upper()))
        
        for path, methods in self.new_spec.get('paths', {}).items():
            for method in methods.keys():
                if method.upper() in ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']:
                    new_paths.add((path, method.upper()))
        
        # Find moved endpoints first
        moved_endpoints = self.find_moved_endpoints(old_paths, new_paths)
        moved_old_paths = {(ep['old_path'], ep['old_method']) for ep in moved_endpoints}
        moved_new_paths = {(ep['new_path'], ep['new_method']) for ep in moved_endpoints}
        
        # Store path changes
        self.changes['path_changes'] = moved_endpoints
        
        # Find truly new and removed endpoints (excluding moved ones)
        truly_new = new_paths - old_paths - moved_new_paths
        truly_removed = old_paths - new_paths - moved_old_paths
        
        self.changes['new_endpoints'] = list(truly_new)
        self.changes['removed_endpoints'] = list(truly_removed)
        
        # Find modified endpoints (same path/method, different content)
        common_endpoints = (old_paths & new_paths) - moved_old_paths
        
        for path, method in common_endpoints:
            old_endpoint = self.old_spec['paths'][path][method.lower()]
            new_endpoint = self.new_spec['paths'][path][method.lower()]
            
            changes = self.analyze_endpoint_modifications(path, method, old_endpoint, new_endpoint)
            if changes:
                self.changes['modified_endpoints'].append({
                    'path': path,
                    'method': method,
                    'changes': changes
                })

    def analyze_endpoint_modifications(self, path: str, method: str, old_endpoint: Dict, new_endpoint: Dict) -> Dict:
        """Analyze detailed modifications in an endpoint"""
        changes = {}
        
        # Summary changes
        old_summary = old_endpoint.get('summary', '')
        new_summary = new_endpoint.get('summary', '')
        if old_summary != new_summary:
            changes['summary'] = {'old': old_summary, 'new': new_summary}
        
        # Description changes
        old_desc = old_endpoint.get('description', '')
        new_desc = new_endpoint.get('description', '')
        if old_desc != new_desc:
            changes['description'] = {'old': old_desc, 'new': new_desc}
        
        # Parameter changes
        param_changes = self.analyze_parameter_changes(
            old_endpoint.get('parameters', []),
            new_endpoint.get('parameters', [])
        )
        if param_changes:
            changes['parameters'] = param_changes
        
        # Request Body changes
        request_body_changes = self.analyze_request_body_changes(
            old_endpoint.get('requestBody', {}),
            new_endpoint.get('requestBody', {})
        )
        if request_body_changes:
            changes['request_body'] = request_body_changes
        
        # Response changes
        response_changes = self.analyze_response_changes(
            old_endpoint.get('responses', {}),
            new_endpoint.get('responses', {})
        )
        if response_changes:
            changes['responses'] = response_changes
        
        # Tags changes
        old_tags = set(old_endpoint.get('tags', []))
        new_tags = set(new_endpoint.get('tags', []))
        if old_tags != new_tags:
            changes['tags'] = {
                'added': list(new_tags - old_tags),
                'removed': list(old_tags - new_tags)
            }
        
        return changes

    def analyze_parameter_changes(self, old_params: List[Dict], new_params: List[Dict]) -> Dict:
        """Analyze changes in parameters"""
        changes = {}
        
        # Create parameter maps for comparison
        old_param_map = {f"{p.get('name', '')}:{p.get('in', '')}": p for p in old_params}
        new_param_map = {f"{p.get('name', '')}:{p.get('in', '')}": p for p in new_params}
        
        old_keys = set(old_param_map.keys())
        new_keys = set(new_param_map.keys())
        
        # Added parameters
        added_params = new_keys - old_keys
        if added_params:
            changes['added'] = [new_param_map[key] for key in added_params]
        
        # Removed parameters
        removed_params = old_keys - new_keys
        if removed_params:
            changes['removed'] = [old_param_map[key] for key in removed_params]
        
        # Modified parameters
        common_params = old_keys & new_keys
        modified_params = []
        for key in common_params:
            old_param = old_param_map[key]
            new_param = new_param_map[key]
            
            param_diff = {}
            
            # Check required status
            if old_param.get('required', False) != new_param.get('required', False):
                param_diff['required'] = {
                    'old': old_param.get('required', False),
                    'new': new_param.get('required', False)
                }
            
            # Check type changes in schema with full resolution
            old_schema = old_param.get('schema', {})
            new_schema = new_param.get('schema', {})
            schema_diff = self.analyze_schema_diff(old_schema, new_schema, self.old_spec, self.new_spec)
            if schema_diff:
                param_diff['schema'] = schema_diff
            
            # Check description changes
            if old_param.get('description', '') != new_param.get('description', ''):
                param_diff['description'] = {
                    'old': old_param.get('description', ''),
                    'new': new_param.get('description', '')
                }
            
            if param_diff:
                param_diff['name'] = old_param.get('name', '')
                param_diff['in'] = old_param.get('in', '')
                modified_params.append(param_diff)
        
        if modified_params:
            changes['modified'] = modified_params
        
        return changes

    def analyze_request_body_changes(self, old_body: Dict, new_body: Dict) -> Dict:
        """Analyze changes in request body"""
        changes = {}
        
        # If one is empty and other is not
        if not old_body and new_body:
            changes['added'] = new_body
            return changes
        elif old_body and not new_body:
            changes['removed'] = old_body
            return changes
        elif not old_body and not new_body:
            return changes
        
        # Check required status
        if old_body.get('required', False) != new_body.get('required', False):
            changes['required'] = {
                'old': old_body.get('required', False),
                'new': new_body.get('required', False)
            }
        
        # Check description changes
        if old_body.get('description', '') != new_body.get('description', ''):
            changes['description'] = {
                'old': old_body.get('description', ''),
                'new': new_body.get('description', '')
            }
        
        # Analyze content changes
        old_content = old_body.get('content', {})
        new_content = new_body.get('content', {})
        
        content_changes = self.analyze_content_changes(old_content, new_content)
        if content_changes:
            changes['content'] = content_changes
        
        return changes

    def analyze_response_changes(self, old_responses: Dict, new_responses: Dict) -> Dict:
        """Analyze changes in responses"""
        changes = {}
        
        old_codes = set(old_responses.keys())
        new_codes = set(new_responses.keys())
        
        # Added response codes
        added_codes = new_codes - old_codes
        if added_codes:
            changes['added_codes'] = {code: new_responses[code] for code in added_codes}
        
        # Removed response codes
        removed_codes = old_codes - new_codes
        if removed_codes:
            changes['removed_codes'] = {code: old_responses[code] for code in removed_codes}
        
        # Modified response codes
        common_codes = old_codes & new_codes
        modified_responses = {}
        
        for code in common_codes:
            old_response = old_responses[code]
            new_response = new_responses[code]
            
            response_diff = {}
            
            # Check description changes
            if old_response.get('description', '') != new_response.get('description', ''):
                response_diff['description'] = {
                    'old': old_response.get('description', ''),
                    'new': new_response.get('description', '')
                }
            
            # Check content changes
            old_content = old_response.get('content', {})
            new_content = new_response.get('content', {})
            
            content_changes = self.analyze_content_changes(old_content, new_content)
            if content_changes:
                response_diff['content'] = content_changes
            
            # Check headers changes
            old_headers = old_response.get('headers', {})
            new_headers = new_response.get('headers', {})
            if old_headers != new_headers:
                response_diff['headers'] = {
                    'old': old_headers,
                    'new': new_headers
                }
            
            if response_diff:
                modified_responses[code] = response_diff
        
        if modified_responses:
            changes['modified_codes'] = modified_responses
        
        return changes

    def analyze_content_changes(self, old_content: Dict, new_content: Dict) -> Dict:
        """Analyze changes in content (media types and schemas)"""
        changes = {}
        
        old_types = set(old_content.keys())
        new_types = set(new_content.keys())
        
        # Added media types
        added_types = new_types - old_types
        if added_types:
            changes['added_media_types'] = {mt: new_content[mt] for mt in added_types}
        
        # Removed media types
        removed_types = old_types - new_types
        if removed_types:
            changes['removed_media_types'] = {mt: old_content[mt] for mt in removed_types}
        
        # Modified media types
        common_types = old_types & new_types
        modified_types = {}
        
        for media_type in common_types:
            old_media = old_content[media_type]
            new_media = new_content[media_type]
            
            media_diff = {}
            
            # Check schema changes with full resolution
            old_schema = old_media.get('schema', {})
            new_schema = new_media.get('schema', {})
            
            schema_diff = self.analyze_schema_diff(old_schema, new_schema, self.old_spec, self.new_spec)
            if schema_diff:
                media_diff['schema'] = schema_diff
            
            # Check examples changes
            if old_media.get('examples', {}) != new_media.get('examples', {}):
                media_diff['examples'] = {
                    'old': old_media.get('examples', {}),
                    'new': new_media.get('examples', {})
                }
            
            if media_diff:
                modified_types[media_type] = media_diff
        
        if modified_types:
            changes['modified_media_types'] = modified_types
        
        return changes

    def resolve_schema_ref(self, schema: Dict, spec: Dict) -> Dict:
        """Resolve schema $ref to actual schema definition"""
        if not schema:
            return {}
        
        if '$ref' in schema:
            ref_path = schema['$ref']
            # Handle #/definitions/model.Something format
            if ref_path.startswith('#/definitions/'):
                schema_name = ref_path.replace('#/definitions/', '')
                definitions = spec.get('definitions', {})
                if schema_name in definitions:
                    return definitions[schema_name]
            # Handle #/components/schemas/Something format
            elif ref_path.startswith('#/components/schemas/'):
                schema_name = ref_path.replace('#/components/schemas/', '')
                schemas = spec.get('components', {}).get('schemas', {})
                if schema_name in schemas:
                    return schemas[schema_name]
        
        return schema

    def get_schema_from_content(self, content: Dict, spec: Dict) -> Dict:
        """Extract schema from content (request/response body)"""
        for media_type, media_content in content.items():
            schema = media_content.get('schema', {})
            if schema:
                return self.resolve_schema_ref(schema, spec)
        return {}

    def analyze_schema_diff(self, old_schema: Dict, new_schema: Dict, old_spec: Dict = None, new_spec: Dict = None) -> Dict:
        """Analyze differences in schema definitions with full resolution"""
        changes = {}
        
        # Resolve references to get actual schema definitions
        if old_spec:
            old_schema = self.resolve_schema_ref(old_schema, old_spec)
        if new_spec:
            new_schema = self.resolve_schema_ref(new_schema, new_spec)
        
        if not old_schema and not new_schema:
            return changes
        
        # Handle reference changes (show both ref and resolved changes)
        if old_schema.get('$ref') != new_schema.get('$ref'):
            changes['reference'] = {
                'old': old_schema.get('$ref'),
                'new': new_schema.get('$ref')
            }
        
        # Type changes
        if old_schema.get('type') != new_schema.get('type'):
            changes['type'] = {
                'old': old_schema.get('type'),
                'new': new_schema.get('type')
            }
        
        # Properties changes (for object types)
        if old_schema.get('type') == 'object' or new_schema.get('type') == 'object' or \
           ('properties' in old_schema) or ('properties' in new_schema):
            old_props = old_schema.get('properties', {})
            new_props = new_schema.get('properties', {})
            
            prop_changes = self.analyze_properties_changes(old_props, new_props, old_spec, new_spec)
            if prop_changes:
                changes['properties'] = prop_changes
        
        # Required fields changes
        old_required = set(old_schema.get('required', []))
        new_required = set(new_schema.get('required', []))
        if old_required != new_required:
            changes['required'] = {
                'added': list(new_required - old_required),
                'removed': list(old_required - new_required)
            }
        
        # Array items changes
        if old_schema.get('type') == 'array' or new_schema.get('type') == 'array':
            old_items = old_schema.get('items', {})
            new_items = new_schema.get('items', {})
            if old_items != new_items:
                items_diff = self.analyze_schema_diff(old_items, new_items, old_spec, new_spec)
                if items_diff:
                    changes['items'] = items_diff
        
        # Description changes
        if old_schema.get('description') != new_schema.get('description'):
            changes['description'] = {
                'old': old_schema.get('description', ''),
                'new': new_schema.get('description', '')
            }
        
        # Format changes
        if old_schema.get('format') != new_schema.get('format'):
            changes['format'] = {
                'old': old_schema.get('format'),
                'new': new_schema.get('format')
            }
        
        # Enum changes
        old_enum = old_schema.get('enum', [])
        new_enum = new_schema.get('enum', [])
        if set(old_enum) != set(new_enum):
            changes['enum'] = {
                'added': list(set(new_enum) - set(old_enum)),
                'removed': list(set(old_enum) - set(new_enum)),
                'old': old_enum,
                'new': new_enum
            }
        
        # Validation constraints changes
        validation_fields = ['minimum', 'maximum', 'minLength', 'maxLength', 'pattern']
        for field in validation_fields:
            if old_schema.get(field) != new_schema.get(field):
                changes[field] = {
                    'old': old_schema.get(field),
                    'new': new_schema.get(field)
                }
        
        return changes

    def analyze_properties_changes(self, old_props: Dict, new_props: Dict, old_spec: Dict = None, new_spec: Dict = None) -> Dict:
        """Analyze changes in object properties with deep schema resolution"""
        changes = {}
        
        old_keys = set(old_props.keys())
        new_keys = set(new_props.keys())
        
        # Added properties
        added_props = new_keys - old_keys
        if added_props:
            added_details = {}
            for prop in added_props:
                prop_schema = self.resolve_schema_ref(new_props[prop], new_spec) if new_spec else new_props[prop]
                added_details[prop] = {
                    'type': prop_schema.get('type', 'unknown'),
                    'description': prop_schema.get('description', ''),
                    'required': False  # Will be determined by parent required array
                }
            changes['added'] = added_details
        
        # Removed properties
        removed_props = old_keys - new_keys
        if removed_props:
            removed_details = {}
            for prop in removed_props:
                prop_schema = self.resolve_schema_ref(old_props[prop], old_spec) if old_spec else old_props[prop]
                removed_details[prop] = {
                    'type': prop_schema.get('type', 'unknown'),
                    'description': prop_schema.get('description', ''),
                    'was_required': False  # Will be determined by parent required array
                }
            changes['removed'] = removed_details
        
        # Modified properties
        common_props = old_keys & new_keys
        modified_props = {}
        
        for prop in common_props:
            old_prop = old_props[prop]
            new_prop = new_props[prop]
            
            prop_diff = self.analyze_schema_diff(old_prop, new_prop, old_spec, new_spec)
            if prop_diff:
                modified_props[prop] = prop_diff
        
        if modified_props:
            changes['modified'] = modified_props
        
        return changes
        
        return changes

    def analyze_schema_changes(self):
        """Analyze changes in data schemas"""
        old_schemas = set(self.old_spec.get('components', {}).get('schemas', {}).keys())
        new_schemas = set(self.new_spec.get('components', {}).get('schemas', {}).keys())
        
        self.changes['new_schemas'] = list(new_schemas - old_schemas)
        self.changes['removed_schemas'] = list(old_schemas - new_schemas)
        
        # Check for modified schemas
        common_schemas = old_schemas & new_schemas
        for schema_name in common_schemas:
            old_schema = self.old_spec['components']['schemas'][schema_name]
            new_schema = self.new_spec['components']['schemas'][schema_name]
            
            if old_schema != new_schema:
                self.changes['modified_schemas'].append(schema_name)

    def analyze_new_categories(self):
        """Analyze new API categories (tags)"""
        old_tags = set()
        new_tags = set()
        
        for path, methods in self.old_spec.get('paths', {}).items():
            for method, spec in methods.items():
                if isinstance(spec, dict) and 'tags' in spec:
                    old_tags.update(spec['tags'])
        
        for path, methods in self.new_spec.get('paths', {}).items():
            for method, spec in methods.items():
                if isinstance(spec, dict) and 'tags' in spec:
                    new_tags.update(spec['tags'])
        
        self.changes['new_categories'] = list(new_tags - old_tags)

    def get_endpoint_details(self, path: str, method: str, spec: Dict) -> Dict:
        """Get detailed information about an endpoint"""
        endpoint = spec['paths'][path][method.lower()]
        
        details = {
            'path': path,
            'method': method,
            'summary': endpoint.get('summary', ''),
            'tags': endpoint.get('tags', []),
            'parameters': len(endpoint.get('parameters', [])),
            'has_request_body': 'requestBody' in endpoint,
            'response_codes': list(endpoint.get('responses', {}).keys())
        }
        
        return details

    def _safe_text_for_json(self, text: str) -> str:
        """Convert text to be JSON-safe, handling XML blocks and newlines"""
        if not text:
            return ""
        
        # Truncate very long descriptions to prevent huge JSON output
        if len(text) > 500:
            text = text[:500] + "... (truncated)"
        
        # Replace problematic characters that could break JSON
        text = text.replace('\n', '\\n')
        text = text.replace('\r', '\\r')
        text = text.replace('\t', '\\t')
        text = text.replace('"', '\\"')
        
        # Handle XML blocks that could cause markdown issues
        text = text.replace('```xml', '```\\nxml')
        text = text.replace('```', '```\\n')
        
        return text

    def _safe_description_for_markdown(self, description: str) -> str:
        """Make description safe for markdown display by handling XML blocks and special characters"""
        if not description:
            return "N/A"
        
        # Truncate very long descriptions
        if len(description) > 300:
            description = description[:300] + "... (truncated)"
        
        # Replace XML code blocks that could interfere with markdown rendering
        description = description.replace('```xml', '`[XML Example]`')
        description = description.replace('```', '`')
        
        # Replace specific XML patterns that cause issues
        import re
        # Remove actual XML content that spans multiple lines
        description = re.sub(r'<\?xml[^>]*>.*?</[^>]+>', '[XML Content]', description, flags=re.DOTALL)
        # Remove single XML tags
        description = re.sub(r'<[^>]+>', '[XML Tag]', description)
        
        # Clean up newlines and replace with proper markdown line breaks
        description = description.replace('\n\n', ' ').replace('\n', ' ')
        
        # Clean up multiple spaces
        description = re.sub(r'\s+', ' ', description).strip()
        
        return description

    def generate_comprehensive_endpoint_diff(self, path: str, method: str, old_endpoint: Dict, 
                                           new_endpoint: Dict, changes: Dict) -> str:
        """
        Generate comprehensive diff focusing on the actual changes detected.
        
        This method creates detailed, unified diff format showing:
        - Parameter additions, removals, and modifications
        - Schema property changes
        - Request/response body changes
        - Description and metadata changes
        """
        diff_lines = []
        
        # Header with clear identification
        diff_lines.append(f"--- {method.upper()} {path} (old)")
        diff_lines.append(f"+++ {method.upper()} {path} (new)")
        diff_lines.append(f"@@ Changes in {method.upper()} {path} @@")
        
        # Show specific changes based on what was detected
        if 'summary' in changes:
            diff_lines.append(f"-  summary: \"{changes['summary']['old']}\"")
            diff_lines.append(f"+  summary: \"{changes['summary']['new']}\"")
        
        if 'description' in changes:
            old_desc = self._safe_description_for_diff(changes['description']['old'])
            new_desc = self._safe_description_for_diff(changes['description']['new'])
            diff_lines.append(f"-  description: \"{old_desc}\"")
            diff_lines.append(f"+  description: \"{new_desc}\"")
        
        if 'parameters' in changes:
            param_changes = changes['parameters']
            
            if param_changes.get('added'):
                for param in param_changes['added']:
                    param_str = f"{param.get('name', 'unknown')} ({param.get('in', 'unknown')})"
                    diff_lines.append(f"+  parameter: {param_str}")
                    if param.get('description'):
                        diff_lines.append(f"+    description: \"{param['description']}\"")
            
            if param_changes.get('removed'):
                for param in param_changes['removed']:
                    param_str = f"{param.get('name', 'unknown')} ({param.get('in', 'unknown')})"
                    diff_lines.append(f"-  parameter: {param_str}")
            
            if param_changes.get('modified'):
                for param in param_changes['modified']:
                    param_name = param.get('name', 'unknown')
                    param_in = param.get('in', 'unknown')
                    diff_lines.append(f"   parameter: {param_name} ({param_in})")
                    if 'description' in param:
                        diff_lines.append(f"-    description: \"{param['description']['old']}\"")
                        diff_lines.append(f"+    description: \"{param['description']['new']}\"")
                    if 'schema' in param:
                        schema_changes = param['schema']
                        # Handle properties changes
                        if 'properties' in schema_changes:
                            prop_changes = schema_changes['properties']
                            
                            if 'added' in prop_changes:
                                for prop_name, prop_info in prop_changes['added'].items():
                                    prop_type = prop_info.get('type', 'unknown')
                                    prop_desc = prop_info.get('description', '')
                                    desc_str = f" // {prop_desc}" if prop_desc else ""
                                    diff_lines.append(f"+    {prop_name}: {prop_type}{desc_str}")
                            
                            if 'removed' in prop_changes:
                                for prop_name, prop_info in prop_changes['removed'].items():
                                    prop_type = prop_info.get('type', 'unknown')
                                    prop_desc = prop_info.get('description', '')
                                    desc_str = f" // {prop_desc}" if prop_desc else ""
                                    diff_lines.append(f"-    {prop_name}: {prop_type}{desc_str}")
                            
                            if 'modified' in prop_changes:
                                for prop_name, prop_change in prop_changes['modified'].items():
                                    diff_lines.append(f"     {prop_name}: modified")
                                    if 'type' in prop_change:
                                        diff_lines.append(f"-      type: {prop_change['type']['old']}")
                                        diff_lines.append(f"+      type: {prop_change['type']['new']}")
                        
                        # Legacy handling
                        if 'added_properties' in schema_changes:
                            for prop_name, prop_type in schema_changes['added_properties'].items():
                                diff_lines.append(f"+    schema.properties.{prop_name}: {prop_type}")
                        if 'removed_properties' in schema_changes:
                            for prop_name, prop_type in schema_changes['removed_properties'].items():
                                diff_lines.append(f"-    schema.properties.{prop_name}: {prop_type}")
                        if 'modified_properties' in schema_changes:
                            for prop_name, prop_change in schema_changes['modified_properties'].items():
                                diff_lines.append(f"     schema.properties.{prop_name}:")
                                if 'type' in prop_change:
                                    diff_lines.append(f"-      type: {prop_change['type']['old']}")
                                    diff_lines.append(f"+      type: {prop_change['type']['new']}")
        
        if 'request_body' in changes:
            rb_changes = changes['request_body']
            if 'added' in rb_changes:
                diff_lines.append(f"+  requestBody: added")
            if 'removed' in rb_changes:
                diff_lines.append(f"-  requestBody: removed")
            
            # Handle content changes which contain schema changes
            if 'content' in rb_changes:
                content_changes = rb_changes['content']
                if 'modified_media_types' in content_changes:
                    for media_type, media_changes in content_changes['modified_media_types'].items():
                        if 'schema' in media_changes:
                            diff_lines.append(f"   requestBody ({media_type}) schema changes:")
                            schema_changes = media_changes['schema']
                            
                            # Handle properties changes
                            if 'properties' in schema_changes:
                                prop_changes = schema_changes['properties']
                                
                                if 'added' in prop_changes:
                                    for prop_name, prop_info in prop_changes['added'].items():
                                        prop_type = prop_info.get('type', 'unknown')
                                        prop_desc = prop_info.get('description', '')
                                        desc_str = f" ({prop_desc})" if prop_desc else ""
                                        diff_lines.append(f"+    {prop_name}: {prop_type}{desc_str}")
                                
                                if 'removed' in prop_changes:
                                    for prop_name, prop_info in prop_changes['removed'].items():
                                        prop_type = prop_info.get('type', 'unknown')
                                        prop_desc = prop_info.get('description', '')
                                        desc_str = f" ({prop_desc})" if prop_desc else ""
                                        diff_lines.append(f"-    {prop_name}: {prop_type}{desc_str}")
                                
                                if 'modified' in prop_changes:
                                    for prop_name, prop_change in prop_changes['modified'].items():
                                        diff_lines.append(f"     {prop_name}: modified")
                                        if 'type' in prop_change:
                                            diff_lines.append(f"-      type: {prop_change['type']['old']}")
                                            diff_lines.append(f"+      type: {prop_change['type']['new']}")
                                        if 'description' in prop_change:
                                            diff_lines.append(f"-      description: {prop_change['description']['old']}")
                                            diff_lines.append(f"+      description: {prop_change['description']['new']}")
            
            # Legacy handling for direct schema changes
            if 'schema' in rb_changes:
                diff_lines.append(f"   requestBody schema changes:")
                schema_changes = rb_changes['schema']
                if 'added_properties' in schema_changes:
                    for prop_name, prop_info in schema_changes['added_properties'].items():
                        prop_type = prop_info.get('type', 'unknown') if isinstance(prop_info, dict) else str(prop_info)
                        diff_lines.append(f"+    schema.properties.{prop_name}: {prop_type}")
                if 'removed_properties' in schema_changes:
                    for prop_name, prop_info in schema_changes['removed_properties'].items():
                        prop_type = prop_info.get('type', 'unknown') if isinstance(prop_info, dict) else str(prop_info)
                        diff_lines.append(f"-    schema.properties.{prop_name}: {prop_type}")
                if 'modified_properties' in schema_changes:
                    for prop_name, prop_change in schema_changes['modified_properties'].items():
                        diff_lines.append(f"     schema.properties.{prop_name}:")
                        if 'type' in prop_change:
                            diff_lines.append(f"-      type: {prop_change['type']['old']}")
                            diff_lines.append(f"+      type: {prop_change['type']['new']}")
        
        if 'responses' in changes:
            resp_changes = changes['responses']
            if resp_changes.get('added_codes'):
                for code in resp_changes['added_codes']:
                    diff_lines.append(f"+  response: {code}")
            if resp_changes.get('removed_codes'):
                for code in resp_changes['removed_codes']:
                    diff_lines.append(f"-  response: {code}")
        
        # If no specific changes were detected, show basic endpoint comparison
        if len(diff_lines) <= 3:  # Only headers
            return self.generate_simple_endpoint_diff(old_endpoint, new_endpoint, path, method)
        
        return '\n'.join(diff_lines)

    def generate_simple_endpoint_diff(self, old_endpoint: Dict, new_endpoint: Dict, 
                                    path: str, method: str) -> str:
        """Generate simple endpoint diff when comprehensive diff is not available"""
        diff_lines = []
        
        # Header
        diff_lines.append(f"--- {method.upper()} {path} (old)")
        diff_lines.append(f"+++ {method.upper()} {path} (new)")
        diff_lines.append(f"@@ Endpoint comparison @@")
        
        # Compare basic fields
        old_summary = old_endpoint.get('summary', '')
        new_summary = new_endpoint.get('summary', '')
        if old_summary != new_summary:
            diff_lines.append(f"-  summary: \"{old_summary}\"")
            diff_lines.append(f"+  summary: \"{new_summary}\"")
        
        old_desc = self._safe_description_for_diff(old_endpoint.get('description', ''))
        new_desc = self._safe_description_for_diff(new_endpoint.get('description', ''))
        if old_desc != new_desc:
            diff_lines.append(f"-  description: \"{old_desc}\"")
            diff_lines.append(f"+  description: \"{new_desc}\"")
        
        # Compare parameter count and basic info
        old_params = old_endpoint.get('parameters', [])
        new_params = new_endpoint.get('parameters', [])
        if len(old_params) != len(new_params):
            diff_lines.append(f"-  parameters: {len(old_params)} parameters")
            diff_lines.append(f"+  parameters: {len(new_params)} parameters")
        
        # Compare responses
        old_responses = list(old_endpoint.get('responses', {}).keys())
        new_responses = list(new_endpoint.get('responses', {}).keys())
        if sorted(old_responses) != sorted(new_responses):
            diff_lines.append(f"-  responses: [{', '.join(sorted(old_responses))}]")
            diff_lines.append(f"+  responses: [{', '.join(sorted(new_responses))}]")
        
        # Compare operation ID
        old_op_id = old_endpoint.get('operationId', '')
        new_op_id = new_endpoint.get('operationId', '')
        if old_op_id != new_op_id:
            diff_lines.append(f"-  operationId: \"{old_op_id}\"")
            diff_lines.append(f"+  operationId: \"{new_op_id}\"")
        
        # If no differences found, show a basic structure comparison
        if len(diff_lines) <= 3:
            diff_lines.append("   Endpoint structure:")
            diff_lines.append(f"   - Summary: {new_summary}")
            diff_lines.append(f"   - Parameters: {len(new_params)}")
            diff_lines.append(f"   - Responses: {len(new_responses)}")
            diff_lines.append("   (No significant structural changes detected)")
        
        return '\n'.join(diff_lines)

    def generate_markdown_report(self, output_file: str = None) -> str:
        """Generate comprehensive markdown report"""
        if output_file is None:
            # Generate filename using user-provided version names
            old_version = self.old_version_name if self.old_version_name else 'unknown'
            new_version = self.new_version_name if self.new_version_name else 'unknown'
            
            # Set docs directory path relative to script location
            script_dir = os.path.dirname(os.path.abspath(__file__))
            docs_dir = os.path.join(script_dir, '..', '..', 'docs')
            docs_dir = os.path.normpath(docs_dir)  # Normalize path
            
            # Create docs directory if it doesn't exist
            try:
                os.makedirs(docs_dir, exist_ok=True)
            except OSError as e:
                print(f"Error: Failed to create documentation directory '{docs_dir}': {e}", file=sys.stderr)
                raise
            
            # Generate filename in format: apidiff_version_version.md
            output_file = os.path.join(docs_dir, f"apidiff_{old_version}_{new_version}.md")
        
        old_version = self.changes['version_change'][0] if self.changes['version_change'] else self.old_version_name or 'unknown'
        new_version = self.changes['version_change'][1] if self.changes['version_change'] else self.new_version_name or 'unknown'
        
        report = []
        report.append(f"# CB-Tumblebug API Changes Report")
        report.append(f"")
        report.append(f"**Analysis Date**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        report.append(f"**Version Comparison**: {old_version} → {new_version}")
        report.append(f"")
        
        # Summary
        report.append(f"## 📊 Summary")
        report.append(f"")
        report.append(f"| Change Type | Count |")
        report.append(f"|-------------|--------|")
        report.append(f"| Modified Endpoints | {len(self.changes['modified_endpoints'])} |")
        report.append(f"| Endpoint Changes | {len(self.changes['path_changes'])} |")
        report.append(f"| Removed Endpoints | {len(self.changes['removed_endpoints'])} |")
        report.append(f"| New Endpoints | {len(self.changes['new_endpoints'])} |")
        report.append(f"| New Schemas | {len(self.changes['new_schemas'])} |")
        report.append(f"| Modified Schemas | {len(self.changes['modified_schemas'])} |")
        report.append(f"| New Categories | {len(self.changes['new_categories'])} |")
        report.append(f"")
        
        # Breaking Changes Assessment - more sophisticated counting
        breaking_changes_count = len(self.changes['removed_endpoints'])  # Always breaking
        
        # Count path changes as potentially breaking (if path or method changed)
        for change in self.changes['path_changes']:
            diff_details = change.get('diff_details', {})
            if (diff_details.get('path_changed') or 
                diff_details.get('method_changed') or 
                diff_details.get('parameters_changed') or
                diff_details.get('request_body_changed')):
                breaking_changes_count += 1
        
        # Count modified endpoints that have breaking changes
        for endpoint in self.changes['modified_endpoints']:
            if self.has_breaking_changes(endpoint):
                breaking_changes_count += 1
        
        if breaking_changes_count > 0:
            report.append(f"⚠️ **BREAKING CHANGES DETECTED**: {breaking_changes_count} potential breaking changes")
        else:
            report.append(f"✅ **NO BREAKING CHANGES**: Only additions and non-breaking modifications")
        
        report.append(f"")
        
        # Version Changes
        if self.changes['version_change']:
            report.append(f"## 🔄 Version Information")
            report.append(f"")
            report.append(f"- **Old Version**: {old_version}")
            report.append(f"- **New Version**: {new_version}")
            report.append(f"")

        # 1. MODIFIED ENDPOINTS (Most Important - moved to top)
        if self.changes['modified_endpoints']:
            report.append(f"## 🔧 Modified Endpoints (Critical Changes)")
            report.append(f"")
            report.append(f"**Total Modified**: {len(self.changes['modified_endpoints'])} endpoints")
            report.append(f"")
            
            for endpoint_change in self.changes['modified_endpoints']:
                path = endpoint_change['path']
                method = endpoint_change['method']
                changes = endpoint_change['changes']
                
                report.append(f"### {method} {path}")
                report.append(f"")
                
                # Summary/Description changes
                if 'summary' in changes:
                    report.append(f"#### 📝 Summary Changed")
                    report.append(f"- **Old**: {changes['summary']['old']}")
                    report.append(f"- **New**: {changes['summary']['new']}")
                    report.append(f"")
                
                if 'description' in changes:
                    report.append(f"#### 📖 Description Changed")
                    old_desc = self._safe_description_for_markdown(changes['description']['old'])
                    new_desc = self._safe_description_for_markdown(changes['description']['new'])
                    report.append(f"- **Old**: {old_desc}")
                    report.append(f"- **New**: {new_desc}")
                    report.append(f"")
                
                # Parameter changes
                if 'parameters' in changes:
                    param_changes = changes['parameters']
                    report.append(f"#### 🔧 Parameter Changes")
                    
                    if 'added' in param_changes:
                        report.append(f"**Added Parameters:**")
                        for param in param_changes['added']:
                            required = " *(required)*" if param.get('required', False) else " *(optional)*"
                            report.append(f"- `{param.get('name', 'unknown')}` ({param.get('in', 'unknown')}){required}")
                            if param.get('description'):
                                report.append(f"  - Description: {param['description']}")
                        report.append(f"")
                    
                    if 'removed' in param_changes:
                        report.append(f"**Removed Parameters:** ⚠️ BREAKING CHANGE")
                        for param in param_changes['removed']:
                            required = " *(was required)*" if param.get('required', False) else " *(was optional)*"
                            report.append(f"- `{param.get('name', 'unknown')}` ({param.get('in', 'unknown')}){required}")
                        report.append(f"")
                    
                    if 'modified' in param_changes:
                        report.append(f"**Modified Parameters:**")
                        for param in param_changes['modified']:
                            param_name = param.get('name', 'unknown')
                            param_in = param.get('in', 'unknown')
                            report.append(f"- `{param_name}` ({param_in})")
                            
                            if 'required' in param:
                                old_req = param['required']['old']
                                new_req = param['required']['new']
                                if new_req and not old_req:
                                    report.append(f"  - **Required status**: Optional → Required ⚠️ BREAKING CHANGE")
                                elif old_req and not new_req:
                                    report.append(f"  - **Required status**: Required → Optional")
                            
                            if 'schema' in param:
                                report.append(f"  - **Schema changes**: {self.format_schema_changes(param['schema'])}")
                            
                            if 'description' in param:
                                report.append(f"  - **Description**: {param['description']['old']} → {param['description']['new']}")
                        report.append(f"")
                
                # Request Body changes
                if 'request_body' in changes:
                    rb_changes = changes['request_body']
                    report.append(f"#### 📤 Request Body Changes")
                    
                    if 'added' in rb_changes:
                        report.append(f"**Request Body Added** (new requirement)")
                        report.append(f"")
                    
                    if 'removed' in rb_changes:
                        report.append(f"**Request Body Removed** ⚠️ BREAKING CHANGE")
                        report.append(f"")
                    
                    if 'required' in rb_changes:
                        old_req = rb_changes['required']['old']
                        new_req = rb_changes['required']['new']
                        if new_req and not old_req:
                            report.append(f"**Required status**: Optional → Required ⚠️ BREAKING CHANGE")
                        elif old_req and not new_req:
                            report.append(f"**Required status**: Required → Optional")
                        report.append(f"")
                    
                    if 'content' in rb_changes:
                        report.append(f"**Content Changes**:")
                        report.append(self.format_content_changes(rb_changes['content']))
                        report.append(f"")
                
                # Response changes
                if 'responses' in changes:
                    resp_changes = changes['responses']
                    report.append(f"#### 📥 Response Changes")
                    
                    if 'added_codes' in resp_changes:
                        report.append(f"**New Response Codes:**")
                        for code in resp_changes['added_codes'].keys():
                            desc = resp_changes['added_codes'][code].get('description', '')
                            report.append(f"- `{code}`: {desc}")
                        report.append(f"")
                    
                    if 'removed_codes' in resp_changes:
                        report.append(f"**Removed Response Codes:** ⚠️ BREAKING CHANGE")
                        for code in resp_changes['removed_codes'].keys():
                            desc = resp_changes['removed_codes'][code].get('description', '')
                            report.append(f"- `{code}`: {desc}")
                        report.append(f"")
                    
                    if 'modified_codes' in resp_changes:
                        report.append(f"**Modified Response Codes:**")
                        for code, code_changes in resp_changes['modified_codes'].items():
                            report.append(f"- **{code}**:")
                            
                            if 'description' in code_changes:
                                report.append(f"  - Description: {code_changes['description']['old']} → {code_changes['description']['new']}")
                            
                            if 'content' in code_changes:
                                report.append(f"  - Content Changes:")
                                report.append(self.format_content_changes(code_changes['content'], indent=4))
                        report.append(f"")
                
                # Tags changes
                if 'tags' in changes:
                    tag_changes = changes['tags']
                    if tag_changes.get('added') or tag_changes.get('removed'):
                        report.append(f"#### 🏷️ Tag Changes")
                        if tag_changes.get('added'):
                            report.append(f"**Added Tags**: {', '.join(tag_changes['added'])}")
                        if tag_changes.get('removed'):
                            report.append(f"**Removed Tags**: {', '.join(tag_changes['removed'])}")
                        report.append(f"")
                
                # Add detailed diff for modified endpoints
                old_endpoint = self.old_spec.get('paths', {}).get(path, {}).get(method.lower(), {})
                new_endpoint = self.new_spec.get('paths', {}).get(path, {}).get(method.lower(), {})
                
                if old_endpoint and new_endpoint:
                    report.append(f"<details>")
                    report.append(f"<summary>🔍 View Detailed Diff</summary>")
                    report.append(f"")
                    report.append(f"```diff")
                    
                    # Generate comprehensive diff for modified endpoint
                    raw_diff = self.generate_comprehensive_endpoint_diff(path, method, old_endpoint, 
                                                                       new_endpoint, changes)
                    if raw_diff and raw_diff.strip():
                        # Limit diff size to avoid overly long reports
                        diff_lines = raw_diff.split('\n')
                        if len(diff_lines) > 100:
                            report.append('\n'.join(diff_lines[:50]))
                            report.append(f"... (diff truncated, {len(diff_lines) - 100} more lines)")
                            report.append('\n'.join(diff_lines[-50:]))
                        else:
                            report.append(raw_diff)
                    else:
                        # Fallback: generate simple diff from the endpoint data
                        simple_diff = self.generate_simple_endpoint_diff(old_endpoint, new_endpoint, path, method)
                        report.append(simple_diff)
                    
                    report.append(f"```")
                    report.append(f"</details>")
                    report.append(f"")
                
                report.append(f"---")
                report.append(f"")

        # 2. PATH CHANGES (Endpoint moves/modifications)
        if self.changes['path_changes']:
            report.append(f"## 🔄 Endpoint Changes (Path/Method/Content)")
            report.append(f"")
            report.append(f"These endpoints have been modified in various ways:")
            report.append(f"")
            
            for change in self.changes['path_changes']:
                diff_details = change.get('diff_details', {})
                old_endpoint = change.get('old_endpoint', {})
                new_endpoint = change.get('new_endpoint', {})
                
                # Generate change summary
                changes_summary = []
                if diff_details.get('path_changed'):
                    changes_summary.append("Path")
                if diff_details.get('method_changed'):
                    changes_summary.append("HTTP Method")
                if diff_details.get('summary_changed'):
                    changes_summary.append("Summary")
                if diff_details.get('description_changed'):
                    changes_summary.append("Description")
                if diff_details.get('operation_id_changed'):
                    changes_summary.append("Operation ID")
                if diff_details.get('parameters_changed'):
                    changes_summary.append("Parameters")
                if diff_details.get('responses_changed'):
                    changes_summary.append("Responses")
                if diff_details.get('request_body_changed'):
                    changes_summary.append("Request Body")
                
                change_type = " & ".join(changes_summary) if changes_summary else "Modified"
                
                report.append(f"### {change['old_method'].upper()} {change['old_path']} → {change['new_method'].upper()} {change['new_path']}")
                report.append(f"**Change Type**: {change_type}")
                report.append(f"**Similarity Score**: {change['similarity']:.2f}")
                report.append(f"")
                
                # Show path change if applicable
                if diff_details.get('path_changed'):
                    report.append(f"#### 📍 Path Change")
                    report.append(f"```diff")
                    report.append(f"- {change['old_method'].upper()} {change['old_path']}")
                    report.append(f"+ {change['new_method'].upper()} {change['new_path']}")
                    report.append(f"```")
                    report.append(f"")
                
                # Show method change if applicable
                if diff_details.get('method_changed'):
                    report.append(f"#### 🔧 HTTP Method Change")
                    report.append(f"- **Old Method**: {change['old_method'].upper()}")
                    report.append(f"- **New Method**: {change['new_method'].upper()}")
                    report.append(f"")
                
                # Show summary change if applicable
                if diff_details.get('summary_changed'):
                    report.append(f"#### 📖 Summary Change")
                    report.append(f"- **Old**: {old_endpoint.get('summary', 'N/A')}")
                    report.append(f"- **New**: {new_endpoint.get('summary', 'N/A')}")
                    report.append(f"")
                
                # Show description change if applicable
                if diff_details.get('description_changed'):
                    report.append(f"#### 📝 Description Change")
                    old_desc = self._safe_description_for_markdown(old_endpoint.get('description', ''))
                    new_desc = self._safe_description_for_markdown(new_endpoint.get('description', ''))
                    report.append(f"- **Old**: {old_desc}")
                    report.append(f"- **New**: {new_desc}")
                    report.append(f"")
                
                # Show parameters change if applicable
                if diff_details.get('parameters_changed'):
                    report.append(f"#### 🔧 Parameters Changed")
                    old_params = old_endpoint.get('parameters', [])
                    new_params = new_endpoint.get('parameters', [])
                    report.append(f"- **Old Parameter Count**: {len(old_params)}")
                    report.append(f"- **New Parameter Count**: {len(new_params)}")
                    report.append(f"")
                
                # Show responses change if applicable
                if diff_details.get('responses_changed'):
                    report.append(f"#### 📤 Responses Changed")
                    old_responses = list(old_endpoint.get('responses', {}).keys())
                    new_responses = list(new_endpoint.get('responses', {}).keys())
                    report.append(f"- **Old Response Codes**: {', '.join(sorted(old_responses))}")
                    report.append(f"- **New Response Codes**: {', '.join(sorted(new_responses))}")
                    report.append(f"")
                
                # Add collapsible raw diff section
                raw_diff = diff_details.get('raw_diff', '')
                if raw_diff:
                    report.append(f"<details>")
                    report.append(f"<summary>🔍 View Detailed Diff</summary>")
                    report.append(f"")
                    report.append(f"```diff")
                    # Limit diff size to avoid overly long reports
                    diff_lines = raw_diff.split('\n')
                    if len(diff_lines) > 100:
                        report.append('\n'.join(diff_lines[:50]))
                        report.append(f"... (diff truncated, {len(diff_lines) - 100} more lines)")
                        report.append('\n'.join(diff_lines[-50:]))
                    else:
                        report.append(raw_diff)
                    report.append(f"```")
                    report.append(f"</details>")
                    report.append(f"")
                
                report.append(f"---")
                report.append(f"")
            
            report.append(f"**⚠️ Migration Required**: Update client code to adapt to these changes")
            report.append(f"")

        # 3. REMOVED ENDPOINTS (Breaking Changes)
        if self.changes['removed_endpoints']:
            report.append(f"## ❌ Removed Endpoints (Breaking Changes)")
            report.append(f"")
            
            for path, method in self.changes['removed_endpoints']:
                details = self.get_endpoint_details(path, method, self.old_spec)
                report.append(f"### {method} {path}")
                report.append(f"")
                report.append(f"- **Summary**: {details['summary']}")
                report.append(f"- **Tags**: {', '.join(details['tags'])}")
                report.append(f"- **Parameters**: {details['parameters']}")
                report.append(f"- **Request Body**: {'Yes' if details['has_request_body'] else 'No'}")
                report.append(f"- **Response Codes**: {', '.join(details['response_codes'])}")
                report.append(f"")
                
                # Add detailed endpoint structure for removed endpoints
                endpoint_data = self.old_spec.get('paths', {}).get(path, {}).get(method.lower(), {})
                if endpoint_data:
                    report.append(f"<details>")
                    report.append(f"<summary>🔍 View Removed Endpoint Details</summary>")
                    report.append(f"")
                    report.append(f"```json")
                    # Create a clean JSON representation
                    endpoint_json = {
                        "method": method,
                        "path": path,
                        "summary": endpoint_data.get('summary', ''),
                        "description": endpoint_data.get('description', ''),
                        "operationId": endpoint_data.get('operationId', ''),
                        "parameters": endpoint_data.get('parameters', []),
                        "responses": list(endpoint_data.get('responses', {}).keys()),
                        "tags": endpoint_data.get('tags', [])
                    }
                    report.append(json.dumps(endpoint_json, indent=2, ensure_ascii=False))
                    report.append(f"```")
                    report.append(f"</details>")
                    report.append(f"")
            
            report.append(f"**⚠️ Migration Required**: These endpoints are no longer available")
            report.append(f"")

        # 4. NEW ENDPOINTS
        if self.changes['new_endpoints']:
            report.append(f"## ➕ New Endpoints")
            report.append(f"")
            
            for path, method in self.changes['new_endpoints']:
                details = self.get_endpoint_details(path, method, self.new_spec)
                report.append(f"### {method} {path}")
                report.append(f"")
                report.append(f"- **Summary**: {details['summary']}")
                report.append(f"- **Tags**: {', '.join(details['tags'])}")
                report.append(f"- **Parameters**: {details['parameters']}")
                report.append(f"- **Request Body**: {'Yes' if details['has_request_body'] else 'No'}")
                report.append(f"- **Response Codes**: {', '.join(details['response_codes'])}")
                report.append(f"")
                
                # Add detailed endpoint structure for new endpoints
                endpoint_data = self.new_spec.get('paths', {}).get(path, {}).get(method.lower(), {})
                if endpoint_data:
                    report.append(f"<details>")
                    report.append(f"<summary>🔍 View New Endpoint Details</summary>")
                    report.append(f"")
                    report.append(f"```json")
                    # Create a clean JSON representation
                    endpoint_json = {
                        "method": method,
                        "path": path,
                        "summary": endpoint_data.get('summary', ''),
                        "description": self._safe_text_for_json(endpoint_data.get('description', '')),
                        "operationId": endpoint_data.get('operationId', ''),
                        "parameters": endpoint_data.get('parameters', []),
                        "responses": list(endpoint_data.get('responses', {}).keys()),
                        "tags": endpoint_data.get('tags', [])
                    }
                    report.append(json.dumps(endpoint_json, indent=2, ensure_ascii=False))
                    report.append(f"```")
                    report.append(f"</details>")
                    report.append(f"")
                    report.append(f"---")
                    report.append(f"")

        # 5. SCHEMA CHANGES
        if self.changes['new_schemas'] or self.changes['modified_schemas']:
            report.append(f"## 📋 Schema Changes")
            report.append(f"")
            
            if self.changes['new_schemas']:
                report.append(f"### New Schemas")
                for schema in self.changes['new_schemas']:
                    report.append(f"- `{schema}`")
                report.append(f"")
            
            if self.changes['modified_schemas']:
                report.append(f"### Modified Schemas")
                for schema in self.changes['modified_schemas']:
                    report.append(f"- `{schema}` ⚠️ May affect data structures")
                report.append(f"")

        # 6. NEW CATEGORIES
        if self.changes['new_categories']:
            report.append(f"## 🆕 New API Categories")
            report.append(f"")
            for category in self.changes['new_categories']:
                report.append(f"- `{category}`")
            report.append(f"")

        # MIGRATION GUIDE
        report.append(f"## 🚀 Migration Guide")
        report.append(f"")
        
        if breaking_changes_count > 0:
            report.append(f"### ⚠️ Breaking Changes Summary")
            report.append(f"")
            if self.changes['removed_endpoints']:
                report.append(f"1. **Removed Endpoints**: {len(self.changes['removed_endpoints'])} endpoints no longer available")
            if self.changes['path_changes']:
                # Count breaking path changes
                breaking_path_changes = sum(1 for change in self.changes['path_changes'] 
                                          if change.get('diff_details', {}).get('path_changed') or 
                                             change.get('diff_details', {}).get('method_changed'))
                report.append(f"2. **Endpoint Changes**: {breaking_path_changes} endpoints with breaking changes")
            if self.changes['modified_endpoints']:
                breaking_mod_endpoints = sum(1 for endpoint in self.changes['modified_endpoints'] 
                                           if self.has_breaking_changes(endpoint))
                report.append(f"3. **Modified Endpoints**: {breaking_mod_endpoints} endpoints with breaking changes")
            report.append(f"")
            
            report.append(f"### 📋 Migration Steps")
            report.append(f"")
            report.append(f"1. **Review Endpoint Changes**: Check each changed endpoint's detailed modifications above")
            report.append(f"2. **Update Client Code**: Adapt to path/method/parameter/request body changes")
            report.append(f"3. **Update Path References**: Change old paths to new paths where applicable")
            report.append(f"4. **Update HTTP Methods**: Change HTTP methods where endpoints switched methods")
            report.append(f"5. **Remove Deprecated Calls**: Stop using removed endpoints")
            report.append(f"6. **Test Thoroughly**: Validate all API integrations")
            report.append(f"")
        else:
            report.append(f"✅ **No Breaking Changes**: This update only adds new functionality")
            report.append(f"")
            
        # RECOMMENDATIONS
        report.append(f"## 💡 Recommendations")
        report.append(f"")
        
        if self.changes['new_endpoints']:
            report.append(f"- **Explore New Features**: {len(self.changes['new_endpoints'])} new endpoints available")
        
        if self.changes['modified_endpoints']:
            report.append(f"- **Priority Testing**: Focus testing on {len(self.changes['modified_endpoints'])} modified endpoints")
        
        if breaking_changes_count > 0:
            report.append(f"- **Staged Migration**: Consider gradual migration to minimize risk")
            report.append(f"- **Backup Plan**: Keep old version available during transition")
        
        report.append(f"")
        
        # Write report to file
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write('\n'.join(report))
        
        return output_file

    def format_schema_changes(self, schema_changes: Dict, indent: int = 0) -> str:
        """Format schema changes for display with detailed breakdown"""
        result = []
        indent_str = " " * indent
        
        # Reference changes (show both old and new schema names)
        if 'reference' in schema_changes:
            old_ref = schema_changes['reference']['old'] or 'inline'
            new_ref = schema_changes['reference']['new'] or 'inline'
            result.append(f"{indent_str}Schema Reference: {old_ref} → {new_ref}")
        
        # Type changes
        if 'type' in schema_changes:
            result.append(f"{indent_str}Type: {schema_changes['type']['old']} → {schema_changes['type']['new']} ⚠️ BREAKING")
        
        # Description changes
        if 'description' in schema_changes:
            old_desc = schema_changes['description']['old'][:50] + "..." if len(schema_changes['description']['old']) > 50 else schema_changes['description']['old']
            new_desc = schema_changes['description']['new'][:50] + "..." if len(schema_changes['description']['new']) > 50 else schema_changes['description']['new']
            result.append(f"{indent_str}Description: {old_desc} → {new_desc}")
        
        # Format changes
        if 'format' in schema_changes:
            result.append(f"{indent_str}Format: {schema_changes['format']['old']} → {schema_changes['format']['new']}")
        
        # Enum changes
        if 'enum' in schema_changes:
            enum_changes = schema_changes['enum']
            if enum_changes.get('added'):
                result.append(f"{indent_str}Enum added: {enum_changes['added']}")
            if enum_changes.get('removed'):
                result.append(f"{indent_str}Enum removed: {enum_changes['removed']} ⚠️ BREAKING")
        
        # Validation constraint changes
        validation_fields = ['minimum', 'maximum', 'minLength', 'maxLength', 'pattern']
        for field in validation_fields:
            if field in schema_changes:
                old_val = schema_changes[field]['old']
                new_val = schema_changes[field]['new']
                result.append(f"{indent_str}{field.capitalize()}: {old_val} → {new_val}")
        
        # Properties changes (detailed breakdown)
        if 'properties' in schema_changes:
            props = schema_changes['properties']
            
            if 'added' in props:
                added_details = []
                for prop_name, prop_info in props['added'].items():
                    prop_type = prop_info.get('type', 'unknown')
                    prop_desc = prop_info.get('description', '')
                    desc_part = f" ({prop_desc[:30]}...)" if len(prop_desc) > 30 else f" ({prop_desc})" if prop_desc else ""
                    added_details.append(f"{prop_name}:{prop_type}{desc_part}")
                result.append(f"{indent_str}Added Properties:")
                for detail in added_details:
                    result.append(f"{indent_str}  + {detail}")
            
            if 'removed' in props:
                removed_details = []
                for prop_name, prop_info in props['removed'].items():
                    prop_type = prop_info.get('type', 'unknown')
                    prop_desc = prop_info.get('description', '')
                    desc_part = f" ({prop_desc[:30]}...)" if len(prop_desc) > 30 else f" ({prop_desc})" if prop_desc else ""
                    removed_details.append(f"{prop_name}:{prop_type}{desc_part}")
                result.append(f"{indent_str}Removed Properties: ⚠️ BREAKING")
                for detail in removed_details:
                    result.append(f"{indent_str}  - {detail}")
            
            if 'modified' in props:
                result.append(f"{indent_str}Modified Properties:")
                for prop_name, prop_changes in props['modified'].items():
                    result.append(f"{indent_str}  ~ {prop_name}:")
                    prop_details = self.format_schema_changes(prop_changes, indent + 4)
                    if prop_details != "Schema structure changed":
                        result.append(f"{prop_details}")
                    else:
                        result.append(f"{indent_str}    Structure changed")
        
        # Required field changes
        if 'required' in schema_changes:
            req = schema_changes['required']
            if req.get('added'):
                result.append(f"{indent_str}New Required Fields: {req['added']} ⚠️ BREAKING")
            if req.get('removed'):
                result.append(f"{indent_str}No Longer Required: {req['removed']}")
        
        # Array items changes
        if 'items' in schema_changes:
            result.append(f"{indent_str}Array Items Changed:")
            items_details = self.format_schema_changes(schema_changes['items'], indent + 2)
            result.append(f"{items_details}")
        
        return '\n'.join(result) if result else "Schema structure changed"

    def format_content_changes(self, content_changes: Dict, indent: int = 0) -> str:
        """Format content changes for display"""
        result = []
        indent_str = " " * indent
        
        if 'added_media_types' in content_changes:
            types = list(content_changes['added_media_types'].keys())
            result.append(f"{indent_str}Added media types: {', '.join(types)}")
        
        if 'removed_media_types' in content_changes:
            types = list(content_changes['removed_media_types'].keys())
            result.append(f"{indent_str}Removed media types: {', '.join(types)} ⚠️ BREAKING")
        
        if 'modified_media_types' in content_changes:
            for media_type, changes in content_changes['modified_media_types'].items():
                result.append(f"{indent_str}{media_type}:")
                if 'schema' in changes:
                    schema_desc = self.format_schema_changes(changes['schema'], indent + 2)
                    result.append(f"{indent_str}  Schema: {schema_desc}")
                if 'examples' in changes:
                    result.append(f"{indent_str}  Examples changed")
        
        return '\n'.join(result) if result else "Content structure changed"

def main():
    """
    Main entry point for the API change analyzer.
    
    Handles command line arguments, orchestrates the analysis process,
    and generates the final report with summary statistics.
    """
    # Set up command line argument parsing
    parser = argparse.ArgumentParser(description="Analyze CB-Tumblebug API changes between versions")
    parser.add_argument('old_version', help='Old version (e.g., v0.11.8)')
    parser.add_argument('new_version', help='New version (e.g., main or v0.11.10)')
    parser.add_argument('-o', '--output', help='Output markdown file (default: auto-generated)')
    parser.add_argument('--local-old', help='Path to local old swagger.json file')
    parser.add_argument('--local-new', help='Path to local new swagger.json file')
    parser.add_argument('-v', '--verbose', action='store_true', help='Verbose output')
    
    args = parser.parse_args()
    
    # Display welcome message
    print(f"CB-Tumblebug API Change Analyzer")
    print(f"{'='*50}")
    
    # Initialize analyzer with user-provided version names
    
    # Pass user-provided version names to analyzer
    analyzer = SwaggerDiffAnalyzer(old_version=args.old_version, new_version=args.new_version)
    analyzer.verbose = args.verbose
    
    # Load specifications
    if args.local_old:
        print(f"Loading old spec from local file: {args.local_old}")
        analyzer.old_spec = analyzer.load_local_swagger_spec(args.local_old)
    else:
        analyzer.old_spec = analyzer.get_swagger_spec(args.old_version)
    
    if args.local_new:
        print(f"Loading new spec from local file: {args.local_new}")
        analyzer.new_spec = analyzer.load_local_swagger_spec(args.local_new)
    else:
        analyzer.new_spec = analyzer.get_swagger_spec(args.new_version)
    
    # Analyze changes
    print(f"Analyzing changes between {args.old_version} and {args.new_version}...")
    analyzer.analyze_all_changes()
    
    # Generate report
    print(f"Generating detailed report...")
    output_file = analyzer.generate_markdown_report(args.output)
    
    # Summary output
    changes = analyzer.changes
    print(f"\nAnalysis Complete!")
    print(f"Report saved to: {output_file}")
    print(f"\nSummary:")
    print(f"  Path Changes: {len(changes['path_changes'])}")
    print(f"  New Endpoints: {len(changes['new_endpoints'])}")
    print(f"  Removed Endpoints: {len(changes['removed_endpoints'])}")
    print(f"  Modified Endpoints: {len(changes['modified_endpoints'])}")
    print(f"  New Schemas: {len(changes['new_schemas'])}")
    print(f"  Modified Schemas: {len(changes['modified_schemas'])}")
    print(f"  New Categories: {len(changes['new_categories'])}")
    
    breaking_changes = len(changes['removed_endpoints']) + len(changes['path_changes']) + len(changes['modified_endpoints'])
    
    if breaking_changes > 0:
        print(f"\n⚠️  BREAKING CHANGES DETECTED: {breaking_changes} potential breaking changes")
        print(f"   Please review the migration guide in the report!")
    else:
        print(f"\n✅ NO BREAKING CHANGES: Only additions and improvements")

if __name__ == '__main__':
    main()
