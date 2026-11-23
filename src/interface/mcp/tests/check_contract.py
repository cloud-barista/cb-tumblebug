#!/usr/bin/env python3
"""Tier 1 contract checks for the CB-Tumblebug MCP server.

Needs no cluster and no cloud. It imports tb-mcp.py, reads the registered tools, and
checks them against the repository:

  golden      the published tool set matches tests/tools.golden.json
  requests    every request a tool sends (path, method, query names, body keys) exists
              in src/interface/rest/docs/swagger.yaml; each tool needs a sample call
  read-only   a tool announced as read-only sends nothing that changes state
  budget      an over-budget creation is refused; an approval covers one plan, once
  journal     the session journal records changes only, and never a secret
  schema-size tools/list and each tool schema stay within budget
  param-docs  every parameter is documented, and nothing undocumented is described
  stale-refs  prose and prompts name only tools that exist
  plus: shadowed definitions, undefined calls, runnable scripts, secret redaction,
  the error contract and duplicate tools.

Usage:
    python3 check_contract.py            # verify
    python3 check_contract.py --update   # rewrite the golden file after an intended change
    python3 check_contract.py --requests # also print each tool's recorded requests
"""

import argparse
import ast
import asyncio
import importlib.util
import inspect
import json
import logging
import os
import re
import sys

import yaml

HERE = os.path.dirname(os.path.abspath(__file__))
MCP_DIR = os.path.dirname(HERE)
REPO = os.path.abspath(os.path.join(MCP_DIR, "..", "..", ".."))
SOURCE = os.path.join(MCP_DIR, "tb-mcp.py")
GOLDEN = os.path.join(HERE, "tools.golden.json")
SWAGGER = os.path.join(REPO, "src", "interface", "rest", "docs", "swagger.yaml")

# A tool description is paid for on every session; long prose belongs in get_usage_guide().
MAX_SCHEMA_BYTES = 4000
MAX_TOOLS_LIST_BYTES = 90_000

failures = []
notes = []


def fail(check, message):
    failures.append(f"[{check}] {message}")


def load_module(read_only=False):
    """Import tb-mcp.py without starting a server."""
    logging.disable(logging.CRITICAL)
    os.environ["MCP_READ_ONLY"] = "true" if read_only else "false"
    name = "tb_mcp_ro" if read_only else "tb_mcp"
    spec = importlib.util.spec_from_file_location(name, SOURCE)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module, asyncio.run(module.mcp.list_tools())


def source_tree():
    with open(SOURCE) as fh:
        return ast.parse(fh.read())


# ---------------------------------------------------------------------------
# golden
# ---------------------------------------------------------------------------

def tool_fingerprint(tools):
    """Names, parameters and description size; not the wording, which changes constantly."""
    out = {}
    for t in sorted(tools, key=lambda x: x.name):
        params = t.parameters or {}
        out[t.name] = {
            "required": sorted(params.get("required") or []),
            "properties": sorted((params.get("properties") or {}).keys()),
            "description_bytes": len(t.description or ""),
        }
    return out


def check_golden(fingerprint, update):
    if update:
        with open(GOLDEN, "w") as fh:
            json.dump(fingerprint, fh, indent=1, sort_keys=True)
            fh.write("\n")
        notes.append(f"golden file rewritten with {len(fingerprint)} tools")
        return
    if not os.path.exists(GOLDEN):
        fail("golden", f"{GOLDEN} is missing - create it with --update")
        return
    with open(GOLDEN) as fh:
        expected = json.load(fh)

    missing = sorted(set(expected) - set(fingerprint))
    added = sorted(set(fingerprint) - set(expected))
    if missing:
        fail("golden", f"tools disappeared: {', '.join(missing)}")
    if added:
        fail("golden", f"tools added without updating the golden file: {', '.join(added)}")
    for name in sorted(set(expected) & set(fingerprint)):
        was, now = expected[name], fingerprint[name]
        if was["required"] != now["required"]:
            fail("golden", f"{name}: required params {was['required']} -> {now['required']}")
        gone = sorted(set(was["properties"]) - set(now["properties"]))
        if gone:
            fail("golden", f"{name}: parameters removed: {', '.join(gone)}")


# ---------------------------------------------------------------------------
# OpenAPI document
# ---------------------------------------------------------------------------

class Api:
    """The REST API as swagger.yaml declares it."""

    def __init__(self, path=SWAGGER):
        with open(path) as fh:
            self.doc = yaml.safe_load(fh)
        self.routes = []
        for template, item in (self.doc.get("paths") or {}).items():
            clean = template[len("/tumblebug"):] if template.startswith("/tumblebug/") else template
            self.routes.append((clean.strip("/").split("/"), clean, item))

    def resolve(self, schema):
        """Follow $ref and fold allOf into one object schema."""
        seen = 0
        while isinstance(schema, dict) and "$ref" in schema and seen < 20:
            name = schema["$ref"].rsplit("/", 1)[-1]
            schema = (self.doc.get("components", {}).get("schemas", {}).get(name) or {})
            seen += 1
        if isinstance(schema, dict) and "allOf" in schema:
            merged = {k: v for k, v in schema.items() if k != "allOf"}
            props = dict(merged.get("properties") or {})
            required = list(merged.get("required") or [])
            for part in schema["allOf"]:
                part = self.resolve(part)
                props.update(part.get("properties") or {})
                required += part.get("required") or []
                # The converter wraps a scalar $ref as "type: object" + allOf; the
                # referenced type is the real one.
                if part.get("type") and (part["type"] != "object" or "type" not in merged):
                    merged["type"] = part["type"]
                for key in ("additionalProperties", "enum", "items"):
                    if key in part and key not in merged:
                        merged[key] = part[key]
            if props:
                merged["properties"] = props
            if required:
                merged["required"] = required
            schema = merged
        return schema if isinstance(schema, dict) else {}

    def match(self, path):
        """The declared route for a concrete path; the most literal template wins."""
        segments = path.strip("/").split("/")
        best, best_score = None, -1
        for template_segments, template, item in self.routes:
            if len(template_segments) != len(segments):
                continue
            score = 0
            for want, got in zip(template_segments, segments):
                if want.startswith("{") and want.endswith("}"):
                    continue
                if want != got:
                    score = -1
                    break
                score += 1
            if score > best_score:
                best, best_score = (template, item), score
        return best

    def operation(self, method, path):
        found = self.match(path)
        if not found:
            return None, None
        template, item = found
        return template, item.get(method.lower())

    @staticmethod
    def query_names(item, operation):
        params = list(item.get("parameters") or []) + list(operation.get("parameters") or [])
        return {p.get("name") for p in params if p.get("in") == "query"}

    def body_schema(self, operation):
        content = (operation.get("requestBody") or {}).get("content") or {}
        for media in ("application/json", "multipart/form-data",
                      "application/x-www-form-urlencoded"):
            if media in content:
                return self.resolve(content[media].get("schema") or {})
        for value in content.values():
            return self.resolve(value.get("schema") or {})
        return None

    def response_schema(self, operation):
        for code in ("200", "201", "202"):
            content = ((operation.get("responses") or {}).get(code) or {}).get("content") or {}
            if "application/json" in content:
                return content["application/json"].get("schema") or {}
        return None

    def example(self, schema, option=None, depth=0):
        """A value shaped like the schema, so a tool can run past its first read."""
        schema = self.resolve(schema)
        props = schema.get("properties")
        if props and all(k.startswith("[") for k in props):
            # One endpoint, several shapes selected by ?option=
            key = f"[{(option or 'default').upper()}]"
            chosen = props.get(key) or props.get("[DEFAULT]") or next(iter(props.values()))
            return self.example(chosen, option, depth)
        if "example" in schema and not isinstance(schema["example"], (dict, list)):
            return schema["example"]
        if schema.get("enum"):
            return schema["enum"][0]
        kind = schema.get("type")
        if props or kind == "object":
            if depth > 5:
                return {}
            return {k: self.example(v, option, depth + 1) for k, v in (props or {}).items()}
        if kind == "array":
            return [self.example(schema.get("items") or {}, option, depth + 1)] if depth <= 5 else []
        if kind == "integer":
            return 1
        if kind == "number":
            return 1.0
        if kind == "boolean":
            return True
        return "x"

    def check_body(self, schema, value, where, problems):
        """Keys the schema does not declare, missing required keys and wrong basic types."""
        schema = self.resolve(schema)
        props = schema.get("properties")
        extra = schema.get("additionalProperties")
        kind = schema.get("type")
        if isinstance(value, dict):
            if kind not in (None, "object"):
                problems.append(f"{where} is an object, the API expects {kind}")
                return
            if props is None:
                if isinstance(extra, dict):
                    for key, item in value.items():
                        self.check_body(extra, item, f"{where}.{key}", problems)
                return
            for key, item in value.items():
                if key in props:
                    self.check_body(props[key], item, f"{where}.{key}", problems)
                elif not extra:
                    problems.append(f"{where}.{key} is not a field the API declares")
            for key in schema.get("required") or []:
                if key not in value:
                    problems.append(f"{where}.{key} is required by the API and was not sent")
        elif isinstance(value, list):
            if kind not in (None, "array"):
                problems.append(f"{where} is a list, the API expects {kind}")
                return
            for i, item in enumerate(value[:3]):
                self.check_body(schema.get("items") or {}, item, f"{where}[{i}]", problems)
        elif value is not None and kind:
            ok = {"string": str, "boolean": bool, "integer": int, "number": (int, float),
                  "array": list, "object": dict}.get(kind)
            if ok and not isinstance(value, ok) or (kind in ("integer", "number")
                                                    and isinstance(value, bool)):
                problems.append(f"{where} is {type(value).__name__}, the API expects {kind}")


# ---------------------------------------------------------------------------
# requests - run every tool against a recording HTTP layer
# ---------------------------------------------------------------------------

SPEC = "aws+ap-northeast-2+t3.small"
IMAGE = "ami-0123456789abcdef0"
GROUPS = [{"name": "g1", "specId": SPEC, "imageId": IMAGE, "nodeGroupSize": 2,
           "rootDiskSize": 50, "label": {"role": "web"}}]
POST_SCRIPT = {"phases": [{"nodeGroupId": "g1", "command": ["echo ok"]}]}
INIT_SCRIPT = {"templateId": "initscript-nginx", "env": {"SITE_TITLE": "demo"}}
RULES = [{"Ports": "80,443", "Protocol": "TCP"}]

NS, INFRA, NODE, GROUP = "default", "infra1", "g1-1", "g1"
TARGET = {"ns_id": NS, "infra_id": INFRA}

# Sample calls per tool. A tool without one fails the check: add its sample here.
SAMPLES = {
    # connections, credentials, runtime CSP registration
    "get_connections": [{}, {"detail": "minimal", "provider": "aws", "credential_holder": "admin"},
                        {"name": "aws-ap-northeast-2"}],
    "list_credential_holders": [{}, {"holder_id": "admin"}],
    "register_csp_definition": [{"provider_name": "openstack-site01",
                                 "driver": "openstack-driver-v1.0.so",
                                 "cloud_platform": "openstack", "description": "demo",
                                 "regions": {"RegionOne": {"id": "RegionOne", "zone": ["nova"],
                                                           "location": {"display": "Seoul",
                                                                        "latitude": 37.5,
                                                                        "longitude": 127.0}}}}],
    "unregister_csp_definition": [{"provider_name": "openstack-site01", "confirm": True}],
    "register_csp_credential": [{"provider_name": "openstack-site01",
                                 "credential": {"IdentityEndpoint": "http://10.0.0.1/identity/v3",
                                                "Username": "admin", "Password": "pw",
                                                "DomainName": "Default", "ProjectID": "p"}}],
    "fetch_assets_for_provider": [{"provider_names": ["x"]}],
    "inspect_csp_resources": [{"connection_name": "aws-ap-northeast-2"},
                              {"connection_name": "aws-ap-northeast-2", "resource_type": "audit"}],
    "register_csp_resources": [{"ns_id": NS, "connection_name": "aws-ap-northeast-2",
                                "resource_type": "vNet", "confirm": True}],

    # planning
    "get_search_options": [{"kind": "spec"}, {"kind": "image"}],
    "recommend_vm_spec": [
        {"filter_policies": {"vCPU": {"min": 2, "max": 4}, "memoryGiB": {"min": 4},
                             "providerName": "aws", "regionName": "ap-northeast-2"}, "limit": 5},
        {"filter_policies": {"acceleratorCount": {"min": 1}, "acceleratorModel": "nvidia"},
         "priority_policy": "location", "latitude": 37.5, "longitude": 127.0},
        {"priority_policy": "performance", "detail": "full"}],
    "search_images": [
        {"matched_spec_id": SPEC, "os_type": "ubuntu 22.04"},
        {"provider_name": "aws", "region_name": "ap-northeast-2", "is_gpu_image": True,
         "os_architecture": "x86_64", "max_results": 5},
        {"matched_spec_id": SPEC, "is_kubernetes_image": True,
         "include_basic_image_only": True, "detail_search_keys": ["tensorflow"]},
        {"custom_images_of": NS, "detail": "full"}],
    "get_spec_options": [{"spec_id": SPEC, "image_id": IMAGE, "refresh": True}],
    "review_spec_image_pair": [{"spec_id": SPEC, "image_id": IMAGE, "root_disk_type": "gp3",
                                "zone": "ap-northeast-2a"}],
    "get_provisioning_risk": [{"spec_id": SPEC}, {"spec_id": SPEC, "csp_image_name": IMAGE},
                              {"spec_id": SPEC, "csp_image_name": IMAGE, "kind": "detailed"}],
    "recommend_alternative_node_config": [
        {"source_spec_id": SPEC, "target_provider": "gcp", "target_region": "asia-northeast3",
         "source_image_id": IMAGE, "os_type": "ubuntu 22.04"}],

    # provisioning
    "review_infra_dynamic_request": [
        {"ns_id": NS, "name": "infra2", "node_groups": GROUPS, "label": {"env": "demo"},
         "description": "demo", "post_script": POST_SCRIPT, "init_script": INIT_SCRIPT,
         "hold": True, "vnet_template_id": "vt", "sg_template_id": "st", "system_label": "x"}],
    "create_infra_dynamic": [
        {"ns_id": NS, "name": "infra2", "node_groups": GROUPS, "label": {"env": "demo"},
         "description": "demo", "post_script": POST_SCRIPT, "post_script_async": True,
         "init_script": {"script": "#!/bin/bash\necho ok\n"}, "hold": True,
         "vnet_template_id": "vt", "sg_template_id": "st", "system_label": "x",
         "policy_on_partial_failure": "refine", "force_create": True, "confirm": True}],
    "add_nodegroup_dynamic": [
        {"spec_id": SPEC, "image_id": IMAGE, "name": "g2", "node_group_size": 2,
         "description": "demo", "root_disk_type": "gp3", "root_disk_size": 50,
         "connection_name": "aws-ap-northeast-2", "zone": "ap-northeast-2a",
         "vnet_template_id": "vt", "sg_template_id": "st", "label": {"role": "web"},
         "post_script": POST_SCRIPT, "post_script_async": True, "init_script": INIT_SCRIPT,
         "distribute_subnets": True, "confirm": True, **TARGET}],
    "review_nodegroup_dynamic": [
        {"spec_id": SPEC, "image_id": IMAGE, "name": "g2", "node_group_size": 2,
         "root_disk_type": "gp3", "root_disk_size": 50, "connection_name": "aws-ap-northeast-2",
         "zone": "ap-northeast-2a", "post_script": POST_SCRIPT, "init_script": INIT_SCRIPT,
         **TARGET}],
    "scale_out_nodegroup": [{"nodegroup_id": GROUP, "add_count": 2, "confirm": True, **TARGET}],
    "list_infra_templates": [{}, {"template_id": "t1"}],
    "provision_infra_from_template": [{"ns_id": NS, "template_id": "t1", "name": "infra2",
                                       "description": "demo", "confirm": True}],
    "manage_infra_template": [
        {"action": "create", "ns_id": NS,
         "template": {"name": "t1", "description": "demo",
                      "infraDynamicReq": {"name": "x", "nodeGroups": GROUPS}}},
        {"action": "update", "ns_id": NS, "template_id": "t1",
         "template": {"name": "t1", "infraDynamicReq": {"name": "x", "nodeGroups": GROUPS}}},
        {"action": "save_from_infra", "ns_id": NS, "infra_id": INFRA},
        {"action": "delete", "ns_id": NS, "template_id": "t1", "confirm": True}],
    "list_init_script_templates": [{}, {"template_id": "t1"}, {"keyword": "nginx"}],
    "manage_init_script_template": [
        {"action": "create",
         "template": {"name": "t1", "description": "demo", "env": {"K": "v"},
                      "scripts": [{"name": "s", "script": "#!/bin/bash\necho ok\n"}]}},
        {"action": "update", "template_id": "t1",
         "template": {"name": "t1", "scripts": [{"name": "s", "script": "#!/bin/bash\necho ok\n",
                                                 "timeoutSec": 60, "continueOnError": True}]}},
        {"action": "delete", "template_id": "t1", "confirm": True}],
    "get_init_script_status": [TARGET, {"detail": "full", "status": "fail", **TARGET}],
    "get_platform_services": [{"messages": 5}],

    # reading and operating an Infra
    "get_infra_list": [{"ns_id": NS}, {"ns_id": NS, "detail": "full", "option": "status"},
                       {"option": "id"}],
    "get_infra": [TARGET, {"detail": "full", **TARGET}],
    "get_nodegroups": [TARGET, {"nodegroup_id": GROUP, **TARGET}],
    "get_infra_associated_resources": [TARGET],
    "diagnose_infra": [TARGET],
    "control_infra": [{"action": "suspend", **TARGET},
                      {"action": "terminate", "force": True, "confirm": True, **TARGET}],
    "control_node": [{"node_id": NODE, "action": "reboot", "force": True, **TARGET},
                     {"node_id": NODE, "action": "delete", "confirm": True, **TARGET}],
    "retry_failed_nodes": [
        TARGET,
        {"dry_run": False, "confirm": True, "node_ids": [NODE], "zone": "ap-northeast-2b",
         "assume_resolved": True, "attempts_per_node": 3, "interval_seconds": 30,
         "keep_failed_nodes": True, **TARGET}],
    "terminate_infra": [{"confirm": True, **TARGET}],
    "release_resources": [{"ns_id": NS, "dry_run": True}, {"ns_id": NS, "confirm": True}],
    "manage_bastion": [{"action": "get", "target_node_id": NODE, **TARGET},
                       {"action": "set", "target_node_id": NODE, "bastion_node_id": "g2-1",
                        "bastion_infra_id": "infra2", "bastion_ns_id": "ns2", **TARGET},
                       {"action": "remove", "bastion_node_id": "g2-1", "confirm": True, **TARGET}],

    # remote commands and files
    "execute_command_infra": [
        {"commands": ["hostname"], "node_id": NODE, "timeout_minutes": 5,
         "env": {"K": "v"}, **TARGET},
        {"commands": ["hostname"], "nodegroup_id": GROUP, "run_async": True, **TARGET},
        {"script_name": "system_info", "label_selector": "role=web", **TARGET},
        {"template_id": "initscript-nginx", **TARGET}],
    "get_command_result": [{"request_id": "r1", "infra_id": INFRA}],
    "manage_command_task": [{"action": "list", **TARGET}, {"action": "get", "task_id": "t", **TARGET},
                            {"action": "cancel", "task_id": "t", **TARGET}],
    "list_node_command_status": [{"node_id": NODE, **TARGET},
                                 {"node_id": NODE, "index": 1, **TARGET},
                                 {"node_id": NODE, "status": ["Completed"], "request_id": "r1",
                                  "page": 1, "size": 5, **TARGET}],
    "clear_all_node_command_status": [{"node_id": NODE, "confirm": True, **TARGET}],
    "list_predefined_scripts": [{}, {"name": "system_info"}],
    "transfer_file_infra": [{"target_path": "/tmp", "content": "hello\n",
                             "file_name": "hello.txt", "node_id": NODE, **TARGET}],
    "download_file_from_node": [{"node_id": NODE, "source_path": "/etc/hostname", **TARGET}],
    "discover_service_endpoints": [{"probe": False}],

    # resources and services
    "get_resources": [{"kind": "vNet"}, {"kind": "securityGroup", "detail": "full"},
                      {"kind": "sshKey", "resource_id": "k1"}, {"kind": "rdbms"},
                      {"kind": "objectStorage"}, {"kind": "nlb"}, {"kind": "customImage"},
                      {"kind": "dataDisk"}],
    "manage_resource": [{"action": "delete", "kind": "vNet", "resource_id": "v1", "confirm": True},
                        {"action": "delete", "kind": "securityGroup", "resource_id": "sg1",
                         "confirm": True},
                        {"action": "restore", "kind": "sshKey", "resource_id": "k1"},
                        {"action": "restore", "kind": "vNet", "resource_id": "v1"}],
    "add_security_group_rules": [{"ns_id": NS, "sg_id": "sg1", "rules": RULES}],
    "remove_security_group_rules": [{"ns_id": NS, "sg_id": "sg1", "rules": RULES, "confirm": True}],
    "get_ssh_private_key": [{"ns_id": NS, "key_id": "k1", "reason": "check"}],
    "manage_labels": [{"action": "get", "label_type": "infra", "uid": "u1"},
                      {"action": "set", "label_type": "infra", "uid": "u1", "labels": {"role": "web"}},
                      {"action": "remove", "label_type": "infra", "uid": "u1", "key": "role"},
                      {"action": "find", "label_type": "node", "labels": "role=web"},
                      {"action": "sync_from_csp", "label_type": "node", "uid": "u1"}],
    "create_custom_image": [{"name": "img1", "description": "demo", **TARGET},
                            {"name": "img1", "node_id": NODE, **TARGET}],
    "manage_object_storage": [
        {"action": "support"}, {"action": "list"},
        {"action": "create", "bucket_id": "demo-bucket-1", "description": "demo",
         "connection_name": "aws-ap-northeast-2"},
        {"action": "get", "bucket_id": "demo-bucket-1"},
        {"action": "list_objects", "bucket_id": "demo-bucket-1"},
        {"action": "presigned_url", "bucket_id": "demo-bucket-1", "object_key": "logs/a b.txt",
         "operation": "upload", "expires_seconds": 600},
        {"action": "delete_object", "bucket_id": "demo-bucket-1", "object_key": "logs/a b.txt",
         "confirm": True},
        {"action": "set_versioning", "bucket_id": "demo-bucket-1", "versioning": True},
        {"action": "set_cors", "bucket_id": "demo-bucket-1",
         "cors_rules": [{"allowedOrigin": ["*"], "allowedMethod": ["GET", "PUT"],
                         "allowedHeader": ["*"], "exposeHeader": ["ETag"],
                         "maxAgeSeconds": 3000}]},
        {"action": "set_cors", "bucket_id": "demo-bucket-1", "cors_rules": []},
        {"action": "empty", "bucket_id": "demo-bucket-1", "confirm": True},
        {"action": "delete", "bucket_id": "demo-bucket-1", "force": True, "confirm": True}],
    "manage_nlb": [
        {"action": "list", **TARGET},
        {"action": "create", "nodegroup_id": GROUP, "port": 80, "target_port": 8080, **TARGET},
        {"action": "create", "scope": "GLOBAL", "protocol": "HTTP", "confirm": True, **TARGET},
        {"action": "get", "nlb_id": GROUP, **TARGET},
        {"action": "health", "nlb_id": GROUP, **TARGET},
        {"action": "delete", "nlb_id": GROUP, "confirm": True, **TARGET}],
    "manage_k8s_cluster": [
        {"action": "info"}, {"action": "versions", "spec_id": SPEC}, {"action": "list"},
        {"action": "get", "cluster_id": "k1"},
        {"action": "create", "cluster_id": "k1", "spec_id": SPEC, "version": "1.31",
         "nodegroup_name": "ng1", "desired_node_size": 2, "max_node_size": 4,
         "root_disk_type": "gp3", "root_disk_size": 50, "confirm": True},
        {"action": "add_nodegroup", "cluster_id": "k1", "spec_id": SPEC,
         "nodegroup_name": "ng2", "confirm": True},
        {"action": "scale", "cluster_id": "k1", "nodegroup_name": "ng1", "desired_node_size": 3,
         "max_node_size": 5},
        {"action": "kubeconfig", "cluster_id": "k1", "confirm": True},
        {"action": "delete_nodegroup", "cluster_id": "k1", "nodegroup_name": "ng2",
         "confirm": True},
        {"action": "delete", "cluster_id": "k1", "confirm": True}],
    "manage_rdbms": [
        {"action": "support"}, {"action": "list"},
        {"action": "capability", "connection_name": "aws-ap-northeast-2"},
        {"action": "validate", "rdbms_id": "db1", "connection_name": "aws-ap-northeast-2",
         "vnet_id": "v1", "subnet_ids": ["s1", "s2"], "security_group_ids": ["sg1"],
         "admin_user_name": "dbadmin", "admin_user_password": "pw",
         "options": {"dbInstanceSpec": "db.t3.micro", "storageSize": 20}},
        {"action": "create", "rdbms_id": "db1", "connection_name": "aws-ap-northeast-2",
         "vnet_id": "v1", "subnet_ids": ["s1", "s2"], "security_group_ids": ["sg1"],
         "admin_user_name": "dbadmin", "admin_user_password": "pw", "public_access": True,
         "confirm": True},
        {"action": "get", "rdbms_id": "db1"}, {"action": "reconcile", "rdbms_id": "db1"},
        {"action": "list_databases", "rdbms_id": "db1", "admin_user_name": "dbadmin",
         "admin_user_password": "pw"},
        {"action": "create_database", "rdbms_id": "db1", "database_name": "app",
         "admin_user_name": "dbadmin", "admin_user_password": "pw"},
        {"action": "delete_database", "rdbms_id": "db1", "database_name": "app",
         "admin_user_name": "dbadmin", "admin_user_password": "pw", "confirm": True},
        {"action": "secure_transport", "rdbms_id": "db1", "admin_user_name": "dbadmin",
         "admin_user_password": "pw"},
        {"action": "delete", "rdbms_id": "db1", "force": True, "confirm": True}],
    "get_dns_hosted_zones": [{}],
    "manage_dns_record": [
        {"action": "get", "domain_name": "example.com", "record_name": "www.example.com"},
        {"action": "set", "domain_name": "example.com", "record_name": "www.example.com",
         "infra_id": INFRA, "confirm": True},
        {"action": "set", "domain_name": "example.com", "record_name": "www.example.com",
         "infra_id": INFRA, "label_selector": "role=web", "routing_policy": "geoproximity",
         "confirm": True},
        {"action": "set", "domain_name": "example.com", "ips": ["1.2.3.4"], "confirm": True},
        {"action": "delete", "domain_name": "example.com", "record_name": "www.example.com",
         "set_identifier": "id-1", "confirm": True}],

    # namespaces, sessions, spend limits, guidance
    "get_namespaces": [{}, {"ns_id": NS}],
    "manage_namespace": [{"action": "create", "name": "ns2", "description": "demo"},
                         {"action": "update", "name": "ns2", "description": "demo"},
                         {"action": "delete", "name": "ns2", "confirm": True}],
    "resume_session": [{"session_id": "s1"}],
    "clear_session": [{"session_id": "s1", "confirm": True}],
    "get_budget_status": [{}],
    "request_budget_increase": [{"request_id": "req-1", "reason": "demo"}],
    "get_usage_guide": [{"topic": "provision"}],
}

# Tools that answer from local state or text and send nothing to CB-Tumblebug.
LOCAL_ONLY = {"get_usage_guide", "list_predefined_scripts", "resume_session", "clear_session",
              "request_budget_increase"}

# POST endpoints that only read or validate; a read-only tool may send these.
READ_POSTS = {
    "/recommendSpec",
    "/ns/{nsId}/resources/searchImage",
    "/specImagePairReview",
    "/ns/{nsId}/infraDynamicReview",
    "/ns/{nsId}/infra/{infraId}/nodeGroupDynamicReview",
    "/ns/{nsId}/infra/{infraId}/retryFailedNodesReview",
    "/recommendAlternativeNodeConfig",
    "/inspectResources",
    "/auditCspResources",
    "/availableRegionZonesForSpec",
    "/availableRegionZonesForSpecList",
    "/ns/{nsId}/downloadFile/infra/{infraId}/node/{nodeId}",
    "/ns/{nsId}/resources/rdbms/validate",
}

def _public_key_pem():
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric import rsa
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    return key.public_key().public_bytes(
        serialization.Encoding.PEM,
        serialization.PublicFormat.SubjectPublicKeyInfo).decode()


# Responses a tool needs to get past its own checks, where the generated example is not enough.
RESPONSE_OVERRIDES = {
    ("GET", "/credential/publicKey"):
        lambda _params: {"publicKey": _public_key_pem(), "publicKeyTokenId": "token-1"},
}


class FakeResponse:
    def __init__(self, payload, status=200):
        self.status_code = status
        # Round-trip through JSON: examples in the document carry YAML dates.
        self.text = json.dumps(payload, default=str) if payload is not None else ""
        self.content = self.text.encode()
        self.headers = {"Content-Type": "application/json"}

    def json(self):
        return json.loads(self.text) if self.text else None

    def raise_for_status(self):
        return None


class FakeEtcd:
    """The few etcd v3 gateway calls the server uses, kept in memory."""

    def __init__(self):
        self.store = {}

    @staticmethod
    def _text(value):
        import base64
        return base64.b64decode(value).decode() if value else ""

    def handle(self, path, body):
        import base64
        key = self._text(body.get("key"))
        if path.endswith("/lease/grant"):
            return {"ID": "1"}
        if path.endswith("/kv/put"):
            self.store[key] = body.get("value") or ""
            return {}
        end = self._text(body.get("range_end"))
        hit = sorted(k for k in self.store if (key <= k < end if end else k == key))
        if path.endswith("/kv/deleterange"):
            for k in hit:
                del self.store[k]
            return {"deleted": str(len(hit))}
        return {"kvs": [{"key": base64.b64encode(k.encode()).decode(), "value": self.store[k]}
                        for k in hit]}

    def put_json(self, key, value):
        import base64
        self.store[key] = base64.b64encode(json.dumps(value).encode()).decode()

    def get_json(self, key):
        return json.loads(self._text(self.store[key])) if key in self.store else None


class Recorder:
    """Stands in for requests.Session: records each call and answers from the schema."""

    def __init__(self, api, base_url, state_url):
        self.api = api
        self.base_url = base_url.rstrip("/")
        self.state_url = state_url.rstrip("/")
        self.calls = []
        self.etcd = FakeEtcd()

    def mount(self, *_args, **_kwargs):
        return None

    def post(self, url, **kwargs):
        return self.request("POST", url, **kwargs)

    def get(self, url, **kwargs):
        return self.request("GET", url, **kwargs)

    def request(self, method, url, **kwargs):
        if url.startswith(self.state_url):
            return FakeResponse(self.etcd.handle(url, kwargs.get("json") or {}))
        path, _, inline = url[len(self.base_url):].partition("?")
        params = dict(kwargs.get("params") or {})
        for pair in filter(None, inline.split("&")):
            key, _, value = pair.partition("=")
            params[key] = value
        body = kwargs.get("json")
        if body is None and isinstance(kwargs.get("data"), dict):
            body = dict(kwargs["data"])
        if kwargs.get("files"):
            body = dict(body or {}, **{name: "<file>" for name in kwargs["files"]})
        self.calls.append({"method": method.upper(), "path": path, "params": params,
                           "body": body, "multipart": bool(kwargs.get("files"))})
        template, operation = self.api.operation(method, path)
        override = RESPONSE_OVERRIDES.get((method.upper(), template))
        if override is not None:
            return FakeResponse(override(params) if callable(override) else override)
        schema = self.api.response_schema(operation) if operation else None
        if schema is None:
            return FakeResponse({})
        return FakeResponse(self.api.example(schema, params.get("option")))


def check_requests(module, tools, api, show):
    """Each tool's requests must be ones the REST API declares."""
    recorder = Recorder(api, module.TUMBLEBUG_API_BASE_URL, module.ETCD_ENDPOINT)
    module._session = recorder
    module._zone_is_delegated = lambda *_a, **_k: True
    module.time.sleep = lambda *_a, **_k: None
    read_only = {t.name for t in tools
                 if getattr(getattr(t, "annotations", None), "readOnlyHint", False)}

    total = 0
    for tool in sorted(tools, key=lambda t: t.name):
        samples = SAMPLES.get(tool.name)
        if not samples:
            fail("requests", f"{tool.name} has no sample call - add one to SAMPLES in "
                             f"check_contract.py")
            continue
        fn = getattr(module, tool.name)
        sent = 0
        for sample in samples:
            start = len(recorder.calls)
            try:
                fn(**sample)
            except Exception as exc:
                fail("requests", f"{tool.name}({_brief(sample)}) raised "
                                 f"{type(exc).__name__}: {exc}")
                continue
            calls = recorder.calls[start:]
            sent += len(calls)
            for call in calls:
                total += 1
                label = f"{tool.name}: {call['method']} {call['path']}"
                if show:
                    print(f"  {label}  ?{sorted(call['params'])}  "
                          f"body={_brief(call['body'])}")
                template, operation = api.operation(call["method"], call["path"])
                if template is None:
                    fail("requests", f"{label} - no such path in swagger.yaml")
                    continue
                if operation is None:
                    fail("requests", f"{label} - {template} does not take {call['method']}")
                    continue
                item = api.match(call["path"])[1]
                unknown = sorted(set(call["params"]) - api.query_names(item, operation))
                if unknown:
                    fail("requests", f"{label} - query parameter(s) the API does not "
                                     f"declare: {', '.join(unknown)}")
                schema = api.body_schema(operation)
                if call["body"] is not None and schema is not None:
                    problems = []
                    api.check_body(schema, call["body"], "body", problems)
                    for problem in problems:
                        fail("requests", f"{label} - {problem}")
                elif call["body"] is not None and schema is None:
                    fail("requests", f"{label} - sends a body the API does not declare")
                if tool.name in read_only and call["method"] != "GET" \
                        and template not in READ_POSTS:
                    fail("read-only", f"{label} - {tool.name} is announced as read-only "
                                      f"but sends {call['method']} {template}")
        if not sent and tool.name not in LOCAL_ONLY:
            fail("requests", f"{tool.name}: no sample reached the API - the sample is "
                             f"refused before it sends anything")
        if sent and tool.name in LOCAL_ONLY:
            fail("requests", f"{tool.name} is listed in LOCAL_ONLY but sent {sent} request(s)")
    stale = sorted(set(SAMPLES) - {t.name for t in tools})
    if stale:
        fail("requests", f"SAMPLES names tools that no longer exist: {', '.join(stale)}")
    notes.append(f"requests checked against swagger.yaml: {total} across {len(tools)} tools")


def _brief(value, limit=90):
    text = json.dumps(value, default=str)
    return text if len(text) <= limit else text[:limit] + "..."


# ---------------------------------------------------------------------------
# static checks
# ---------------------------------------------------------------------------

def check_schema_budget(tools):
    listing = json.dumps([
        {"name": t.name, "description": t.description, "inputSchema": t.parameters}
        for t in tools])
    if len(listing) > MAX_TOOLS_LIST_BYTES:
        fail("schema-size", f"tools/list is {len(listing)} B, budget {MAX_TOOLS_LIST_BYTES} B")
    for t in tools:
        size = len(json.dumps({"name": t.name, "description": t.description,
                               "inputSchema": t.parameters}))
        if size > MAX_SCHEMA_BYTES:
            fail("schema-size", f"{t.name} schema is {size} B, budget {MAX_SCHEMA_BYTES} B "
                                f"- move prose to get_usage_guide()")
    notes.append(f"tools/list is {len(listing)} B across {len(tools)} tools")


def check_resource_kinds(module, api):
    """Every kind get_resources() offers must map to a declared route."""
    kinds = getattr(module, "_RESOURCE_KINDS", None)
    if not kinds:
        fail("resource-kinds", "_RESOURCE_KINDS is missing")
        return
    for kind, (path, _fields) in kinds.items():
        template, operation = api.operation("GET", f"/ns/default/resources/{path}")
        if template is None or operation is None or "{" in template.rsplit("/", 1)[-1]:
            fail("resource-kinds", f"kind {kind!r} maps to /ns/{{nsId}}/resources/{path}, "
                                   f"which swagger.yaml does not declare")


def tool_functions(tree):
    """Top-level functions registered with @tool, with the decorator's keywords."""
    out = {}
    for node in tree.body:
        if not isinstance(node, ast.FunctionDef):
            continue
        for dec in node.decorator_list:
            if isinstance(dec, ast.Call) and getattr(dec.func, "id", None) == "tool":
                out.setdefault(node.name, []).append(
                    (node, {k.arg: ast.literal_eval(k.value) for k in dec.keywords
                            if isinstance(k.value, ast.Constant)}))
    return out


def check_no_shadowed_tools(tree):
    for name, entries in sorted(tool_functions(tree).items()):
        if len(entries) > 1:
            fail("shadowed", f"{name} is defined {len(entries)} times, at lines "
                             f"{[n.lineno for n, _ in entries]}; the last one wins")


def check_no_undefined_calls(tree):
    """A call must name something that exists; Python only notices when the line runs."""
    import builtins
    defined = set(dir(builtins))
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            defined.add(node.name)
        elif isinstance(node, ast.Assign):
            for target in node.targets:
                for leaf in ast.walk(target):
                    if isinstance(leaf, ast.Name):
                        defined.add(leaf.id)
        elif isinstance(node, (ast.AnnAssign, ast.NamedExpr)) and isinstance(node.target, ast.Name):
            defined.add(node.target.id)
        elif isinstance(node, (ast.Import, ast.ImportFrom)):
            for alias in node.names:
                defined.add((alias.asname or alias.name).split(".")[0])
        elif isinstance(node, ast.arg):
            defined.add(node.arg)
        elif isinstance(node, (ast.For, ast.comprehension)):
            for leaf in ast.walk(node.target):
                if isinstance(leaf, ast.Name):
                    defined.add(leaf.id)
        elif isinstance(node, ast.ExceptHandler) and node.name:
            defined.add(node.name)
        elif isinstance(node, ast.withitem) and isinstance(node.optional_vars, ast.Name):
            defined.add(node.optional_vars.id)
    missing = {}
    for node in ast.walk(tree):
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) \
                and node.func.id not in defined:
            missing.setdefault(node.func.id, []).append(node.lineno)
    for name, lines in sorted(missing.items()):
        fail("undefined", f"{name}() is called at line(s) {lines} but defined nowhere")


def check_no_stale_tool_references(tree, tools):
    """Docstrings, guides and prompts are the model's map; they must name real tools."""
    published = {t.name for t in tools}
    internal = {node.name for node in ast.walk(tree)
                if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))}
    verbs = sorted({name.split("_")[0] for name in published if "_" in name})
    pattern = re.compile(r"(?<![\w.])((?:%s)_[a-z0-9_]+)\(" % "|".join(verbs))
    private = re.compile(r"(?<![\w.])(_[a-z][a-z0-9_]+)\(")
    stale, leaked = {}, {}
    for node in ast.walk(tree):
        if not isinstance(node, ast.Constant) or not isinstance(node.value, str):
            continue
        for name in pattern.findall(node.value):
            if name not in published and name not in internal:
                stale.setdefault(name, set()).add(node.lineno)
        for name in private.findall(node.value):
            if name in internal:
                leaked.setdefault(name, set()).add(node.lineno)
    for name, lines in sorted(stale.items()):
        fail("stale-refs", f"{name}() is named in text at line(s) {sorted(lines)[:6]} "
                           f"but no such tool exists")
    for name, lines in sorted(leaked.items()):
        fail("stale-refs", f"{name}() is an internal helper named in text at line(s) "
                           f"{sorted(lines)[:6]}; a caller cannot use it")


def check_prompts(module, tools):
    """Every prompt must render, and must not point at a tool that is not published."""
    try:
        prompts = asyncio.run(module.mcp.list_prompts())
    except Exception as exc:
        fail("prompts", f"could not list prompts: {exc}")
        return
    published = {t.name for t in tools}
    verbs = sorted({name.split("_")[0] for name in published if "_" in name})
    pattern = re.compile(r"(?<![\w.])((?:%s)_[a-z0-9_]+)\(" % "|".join(verbs))
    for prompt in prompts:
        fn = getattr(module, prompt.name, None)
        try:
            text = fn() if callable(fn) else ""
        except Exception as exc:
            fail("prompts", f"{prompt.name} failed to render: {exc}")
            continue
        unknown = sorted({name for name in pattern.findall(str(text)) if name not in published})
        if unknown:
            fail("prompts", f"{prompt.name} names tools that are not published: "
                            f"{', '.join(unknown)}")
    notes.append(f"{len(prompts)} prompts checked")


def check_parameter_docs(tree, tools):
    """Each parameter reaches the model with a description, and Args names only real ones."""
    injected = {"session_id", "idempotency_key"}
    for tool in tools:
        for name, prop in ((tool.parameters or {}).get("properties") or {}).items():
            if name not in injected and not ((prop or {}).get("description") or "").strip():
                fail("param-docs", f"{tool.name}.{name} has no description; document it "
                                   f"under Args:, one parameter per line")
    for name, entries in tool_functions(tree).items():
        node = entries[-1][0]
        doc = ast.get_docstring(node) or ""
        section = re.search(r"\n\s*Args:\n(.*?)(?:\n\s*Returns:|\Z)", doc, re.S)
        if not section:
            continue
        lines = [line for line in section.group(1).splitlines() if line.strip()]
        if not lines:
            continue
        indent = min(len(line) - len(line.lstrip()) for line in lines)
        described = {m.group(1) for line in lines
                     if len(line) - len(line.lstrip()) == indent
                     for m in [re.match(r"\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:\([^)]*\))?:", line)]
                     if m}
        real = {a.arg for a in node.args.args + node.args.kwonlyargs}
        for extra in sorted(described - real):
            fail("param-docs", f"{name} documents '{extra}', which is not one of its parameters")
    for name, entries in tool_functions(tree).items():
        doc = ast.get_docstring(entries[-1][0]) or ""
        returns = doc.split("Returns:", 1)[1] if "Returns:" in doc else ""
        stray = re.findall(r"\n\s+(confirm|dry_run|force):", returns)
        if stray:
            fail("param-docs", f"{name} describes {stray[0]} under Returns:, where it is "
                               f"dropped from the schema")


def check_predefined_scripts(module, tools):
    """Every catalogued script can be run, and what it downloads exists in this repository."""
    scripts = getattr(module, "PREDEFINED_SCRIPTS", None)
    if not scripts:
        fail("scripts", "PREDEFINED_SCRIPTS is missing or empty")
        return
    runner = next((t for t in tools
                   if "script_name" in (t.parameters.get("properties") or {})), None)
    if runner is None:
        fail("scripts", f"no tool takes script_name, so none of the {len(scripts)} "
                        f"predefined scripts can be run")
        return
    have_repo = os.path.isdir(os.path.join(REPO, "scripts"))
    fetched = 0
    for name, config in scripts.items():
        commands = config.get("commands")
        if not commands or not all(isinstance(c, str) and c.strip() for c in commands):
            fail("scripts", f"{name} has no usable commands")
            continue
        if not config.get("description"):
            fail("scripts", f"{name} has no description")
        used = {m for c in commands for m in re.findall(r"\{\{(\w+)\}\}", c)}
        stale = sorted(set(config.get("defaults") or {}) - used)
        if stale:
            fail("scripts", f"{name} has defaults for placeholders it does not use: {stale}")
        for path in re.findall(r"raw\.githubusercontent\.com/cloud-barista/cb-tumblebug/main/"
                               r"(scripts/[A-Za-z0-9_./-]+)", " ".join(commands)):
            fetched += 1
            if have_repo and not os.path.exists(os.path.join(REPO, path)):
                fail("scripts", f"{name} downloads {path}, which is not in this repository")
    where = "found in the repository" if have_repo else "not checked (scripts/ is not mounted)"
    notes.append(f"{len(scripts)} predefined scripts runnable via {runner.name}; "
                 f"{fetched} script downloads {where}")


def check_internal_calls_return_data(module, tools):
    """Tools call each other; the module-level name must stay the plain function."""
    for tool in tools:
        fn = getattr(module, tool.name, None)
        if fn is None:
            fail("internal-calls", f"{tool.name} is not present in the module namespace")
        elif inspect.signature(fn).return_annotation is module.ToolResult:
            fail("internal-calls", f"{tool.name} in the module namespace returns ToolResult")


def check_secret_redaction(module, tree):
    sample = {
        "id": "key-1",
        "privateKey": "-----BEGIN RSA PRIVATE KEY-----\nAAAA",
        "nested": [{"clientSecret": "s3cr3t", "keep": "visible"}],
        "credentialHolder": "admin",
    }
    got = module._redact_secrets(sample)
    blob = json.dumps(got)
    if "BEGIN RSA PRIVATE KEY" in blob or "s3cr3t" in blob:
        fail("secrets", f"_redact_secrets left a secret in place: {blob}")
    if got.get("credentialHolder") != "admin" or got["nested"][0]["keep"] != "visible":
        fail("secrets", "_redact_secrets removed a field that is not a secret")
    opted_out = sorted(name for name, entries in tool_functions(tree).items()
                       if entries[-1][1].get("redact") is False)
    if opted_out != ["get_ssh_private_key"]:
        fail("secrets", f"unexpected redaction opt-outs: {opted_out}")


def check_error_contract(module):
    cases = [
        ({"error": {"message": "The infra nope does not exist."}}, "NOT_FOUND"),
        ({"error": {"message": "CreateNs(); The namespace x already exists."}}, "ALREADY_EXISTS"),
        ({"error": "connection refused"}, "UPSTREAM_UNREACHABLE"),
        ({"error": "something odd happened"}, "UPSTREAM_ERROR"),
        ({"error": "unknown action 'x'; expected one of a, b"}, "INVALID_ARGUMENT"),
        ({"error": {"message": "anything", "code": "NEEDS_CONFIRMATION"}}, "NEEDS_CONFIRMATION"),
    ]
    for payload, expected in cases:
        got = module._as_structured_error(payload)
        if got is None:
            fail("errors", f"{payload} was not recognised as a failure")
        elif got["error"]["code"] != expected:
            fail("errors", f"{payload} -> {got['error']['code']}, expected {expected}")
        elif got.get("ok") is not False:
            fail("errors", f"{payload} -> missing ok=false")
    for ok_payload in ({"id": "infra-1"}, {"message": "Success (No content)"}, ["a", "b"]):
        if module._as_structured_error(ok_payload) is not None:
            fail("errors", f"{ok_payload} was wrongly treated as a failure")


def check_no_duplicate_tools(tools):
    """Same parameters plus a shared name prefix is a choice the model should not have."""
    seen = {}
    for t in tools:
        params = t.parameters or {}
        key = (tuple(sorted((params.get("properties") or {}).keys())),
               tuple(sorted(params.get("required") or [])))
        seen.setdefault(key, []).append(t.name)
    for key, names in seen.items():
        if len(names) < 2 or not key[0]:
            continue
        names = sorted(names)
        related = sorted({f"{a}/{b}" for a in names for b in names
                          if a != b and b.startswith(a + "_")})
        if related:
            fail("duplicates", "same parameters and a shared name prefix: " + ", ".join(related))


def check_annotations(tools, tree):
    """A tool that takes confirm must not be announced as read-only."""
    for tool in tools:
        hints = getattr(tool, "annotations", None)
        props = (tool.parameters or {}).get("properties") or {}
        if getattr(hints, "readOnlyHint", False) and "confirm" in props:
            fail("annotations", f"{tool.name} takes confirm but is announced as read-only")


def check_spend_limits(module, api):
    """An over-budget creation is refused, an approval covers one plan once."""
    recorder = Recorder(api, module.TUMBLEBUG_API_BASE_URL, module.ETCD_ENDPOINT)
    module._session = recorder
    module._spec_cost_cache.clear()
    plan = dict(SAMPLES["create_infra_dynamic"][0])

    def created():
        return sum(1 for c in recorder.calls if c["path"].endswith("/infraDynamic"))

    def attempt(**extra):
        got = module._as_structured_error(module.create_infra_dynamic(**{**plan, **extra}))
        return (got or {}).get("error") or {}

    if attempt() or created() != 1:
        fail("budget", "a creation was refused although no spend limit is set")
    recorder.etcd.put_json(module.BUDGET_POLICY_KEY,
                           {"enabled": True, "per_creation_usd_per_hour": 0.5})
    refused = attempt()
    request_id = (refused.get("detail") or {}).get("request_id")
    if refused.get("code") != "BUDGET_EXCEEDED" or not request_id or created() != 1:
        fail("budget", f"an over-budget creation was not refused cleanly: {refused}")
        return
    key = f"{module.BUDGET_REQUEST_PREFIX}{request_id}"
    if attempt(budget_ack=request_id).get("code") != "BUDGET_EXCEEDED" or created() != 1:
        fail("budget", "a pending request was accepted as an approval")
    if len([k for k in recorder.etcd.store if k.startswith(module.BUDGET_REQUEST_PREFIX)]) != 1:
        fail("budget", "retrying a refused creation filed a second request")
    noted = module.request_budget_increase(request_id=request_id, reason="needed for the demo")
    if recorder.etcd.get_json(key).get("reason") != "needed for the demo" or "error" in noted:
        fail("budget", "request_budget_increase did not record the reason")

    record = recorder.etcd.get_json(key)
    record["status"] = "approved"
    recorder.etcd.put_json(key, record)
    bigger = [dict(plan["node_groups"][0], nodeGroupSize=50)]
    if attempt(budget_ack=request_id, node_groups=bigger).get("code") != "BUDGET_EXCEEDED" \
            or created() != 1:
        fail("budget", "an approval was spent on a larger plan than the one approved")
    if attempt(budget_ack=request_id) or created() != 2:
        fail("budget", "an approved request did not let the approved plan through")
    if recorder.etcd.get_json(key).get("status") != "used":
        fail("budget", "a spent approval was not marked used")
    if attempt(budget_ack=request_id).get("code") != "BUDGET_EXCEEDED" or created() != 2:
        fail("budget", "an approval worked twice")
    today = [k for k in recorder.etcd.store if k.startswith(module.BUDGET_CREATED_PREFIX)]
    if len(today) != 1:
        fail("budget", f"the approved creation was not counted once in today's total ({len(today)})")
    recorder.etcd.put_json(module.BUDGET_POLICY_KEY, {
        "enabled": True, "per_creation_usd_per_hour": float("nan"),
        "per_day_created_usd_per_hour": -1, "concurrent_running_usd_per_hour": "many"})
    policy = module._budget_policy()
    if any(policy[k] != v for k, v in module.DEFAULT_BUDGET_POLICY.items() if k != "enabled"):
        fail("budget", f"an unusable limit was kept instead of the default: {policy}")

    nodes = [{"status": "Running", "spec": {"costPerHour": 1.0}},
             {"status": "Failed", "spec": {"costPerHour": 2.0}},
             {"status": "Failed", "cspResourceId": "i-1", "spec": {"costPerHour": 4.0}},
             {"status": "Terminated", "spec": {"costPerHour": 8.0}},
             {"status": "Prepared", "spec": {"costPerHour": 16.0}}]
    real = module.api_request
    module.api_request = lambda method, path, **_: (
        {"output": ["ns1"]} if path == "/ns" else {"infra": [{"node": nodes}]})
    try:
        running, _ = module._running_spend()
    finally:
        module.api_request = real
    if running != 21.0:
        fail("budget", f"running spend is {running}; expected 21.0 - running, failed with an "
                       f"instance and on hold count, terminated and never-created do not")
    notes.append("spend limits: refusal, single request per plan, approval bound to the plan, "
                 "single use, unusable limits fall back, running spend by node state")


def check_journal(module, api):
    """The session journal keeps what changed something, without secrets."""
    recorder = Recorder(api, module.TUMBLEBUG_API_BASE_URL, module.ETCD_ENDPOINT)
    module._session = recorder
    tools = {t.name: t for t in asyncio.run(module.mcp.list_tools())}

    def run(name, arguments):
        return asyncio.run(tools[name].run({**arguments, "session_id": "s-journal"}))

    run("manage_namespace", {"action": "create", "name": "ns-journal"})
    run("manage_namespace", {"action": "delete", "name": "ns-journal"})  # refused: no confirm
    run("register_csp_credential", dict(SAMPLES["register_csp_credential"][0]))
    run("manage_labels", {"action": "get", "label_type": "infra", "uid": "u1"})
    stored = json.dumps([recorder.etcd.get_json(k) for k in recorder.etcd.store
                         if "/session/s-journal/" in k])
    entries = module.resume_session("s-journal").get("last_actions") or []
    if [e.get("tool") for e in entries] != ["manage_namespace", "register_csp_credential"]:
        fail("journal", f"expected the two changes to be journaled, got {entries}")
    if '"pw"' in stored or "Password" in stored:
        fail("journal", "a credential was written into the session journal")
    first = asyncio.run(tools["manage_namespace"].run(
        {"action": "create", "name": "ns-once", "idempotency_key": "k1"})).content[0].text
    again = asyncio.run(tools["manage_namespace"].run(
        {"action": "create", "name": "ns-once", "idempotency_key": "k1"})).content[0].text
    if "_idempotent_replay" in first or "_idempotent_replay" not in again:
        fail("journal", "an idempotency_key did not replay the first result")
    notes.append("journal: changes recorded, refusals and reads left out, secrets redacted, "
                 "idempotent replay")


def check_read_only_mode(tools, api):
    """MCP_READ_ONLY=true publishes the reading tools, and refuses every write."""
    module, served = load_module(read_only=True)
    mixed = set(module.READ_ACTION_TOOLS)
    expected = {t.name for t in tools
                if getattr(getattr(t, "annotations", None), "readOnlyHint", False)} | mixed
    got = {t.name for t in served}
    if got != expected:
        fail("read-only", f"MCP_READ_ONLY serves {len(got)} tools, expected the "
                          f"{len(expected)} reading ones; difference: "
                          f"{sorted(got ^ expected)[:8]}")
    recorder = Recorder(api, module.TUMBLEBUG_API_BASE_URL, module.ETCD_ENDPOINT)
    module._session = recorder
    for name in sorted(mixed):
        registered = next((t for t in served if t.name == name), None)
        if registered is None:
            continue
        for sample in SAMPLES.get(name, []):
            action = str(sample.get("action") or "").lower()
            start = len(recorder.calls)
            result = asyncio.run(registered.run(dict(sample)))
            text = "".join(getattr(part, "text", "") for part in result.content)
            sent = recorder.calls[start:]
            if action in module.READ_ACTION_TOOLS[name]:
                bad = [c for c in sent if c["method"] != "GET"
                       and api.operation(c["method"], c["path"])[0] not in READ_POSTS]
                if bad:
                    fail("read-only", f"{name}(action={action!r}) is declared a read action "
                                      f"but sends {bad[0]['method']} {bad[0]['path']}")
            elif sent or "READ_ONLY" not in text:
                fail("read-only", f"{name}(action={action!r}) was not refused under "
                                  f"MCP_READ_ONLY")
    notes.append(f"MCP_READ_ONLY=true serves {len(got)} of {len(tools)} tools and refuses "
                 f"the writing actions of {len(mixed)}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--update", action="store_true",
                        help="rewrite the golden file instead of checking it")
    parser.add_argument("--requests", action="store_true",
                        help="print the requests each tool sends")
    args = parser.parse_args()

    module, tools = load_module()
    tree = source_tree()
    api = Api()

    check_golden(tool_fingerprint(tools), args.update)
    check_schema_budget(tools)
    check_resource_kinds(module, api)
    check_no_shadowed_tools(tree)
    check_no_undefined_calls(tree)
    check_no_stale_tool_references(tree, tools)
    check_prompts(module, tools)
    check_predefined_scripts(module, tools)
    check_parameter_docs(tree, tools)
    check_internal_calls_return_data(module, tools)
    check_secret_redaction(module, tree)
    check_error_contract(module)
    check_no_duplicate_tools(tools)
    check_annotations(tools, tree)
    check_requests(module, tools, api, args.requests)
    check_spend_limits(module, api)
    check_journal(module, api)
    check_read_only_mode(tools, api)

    for note in notes:
        print(f"  note: {note}")
    if failures:
        print(f"\nFAILED ({len(failures)}):")
        for f in failures:
            print(f"  {f}")
        return 1
    print(f"\nOK - {len(tools)} tools passed all contract checks")
    return 0


if __name__ == "__main__":
    sys.exit(main())
