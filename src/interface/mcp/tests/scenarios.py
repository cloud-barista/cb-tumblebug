#!/usr/bin/env python3
"""Tier 2 - real requests through the MCP endpoint.

Every step goes through MCP tools only, the way an agent would. By default nothing
billable is created: discovery, a reviewed plan, the refusal gates, metadata round trips
and reads of the Infras that already exist.

    make k-port-forward                         # prerequisite: the MCP endpoint on :8080
    python3 scenarios.py                        # no spend
    python3 scenarios.py --only plan            # one scenario
    python3 scenarios.py --spend                # adds a one-node lifecycle (a few cents)
    python3 scenarios.py --spend --only services      # load balancers, image, database
    python3 scenarios.py --spend --only kubernetes    # a managed Kubernetes cluster

--spend creates one small VM, runs commands on it, moves a file through a bucket with
presigned URLs, labels it, opens and closes a port, scales the group out by one, deletes
that node, and terminates everything, including on failure. MAX_USD_PER_HOUR (default 0.2)
is checked against the reviewed plan first.

"services" (about 20 minutes) and "kubernetes" (about 30 minutes) run only when named:
they create a managed database, load balancers and a cluster, which cost more than a VM.

Exit code is non-zero when a check fails.
"""

import argparse
import json
import os
import secrets
import sys
import time
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from mcp_client import McpClient, McpError  # noqa: E402

NS = os.environ.get("MCP_TEST_NS", "default")
PROVIDER = os.environ.get("MCP_TEST_PROVIDER", "aws")
REGION = os.environ.get("MCP_TEST_REGION", "ap-northeast-2")
MAX_USD_PER_HOUR = float(os.environ.get("MAX_USD_PER_HOUR", "0.2"))
MAX_TOOLS_LIST_BYTES = 90_000
# Everything this script creates starts with it; two runs at once need different ones
PREFIX = os.environ.get("MCP_TEST_PREFIX", "mcpt")


class Run:
    """Shared state: the client, the checks that failed, and what a scenario learned."""

    def __init__(self, client):
        self.mcp = client
        self.failed = []
        self.skipped = []
        self.spec = None
        self.image = None
        self.groups = None

    def check(self, what, ok, detail=""):
        mark = "pass" if ok else "FAIL"
        print(f"      [{mark}] {what}" + (f" - {detail}" if detail and not ok else ""))
        if not ok:
            self.failed.append(what)
        return ok

    def call(self, tool, args=None, note=""):
        return self.mcp.call(tool, args, note)

    def expect_code(self, tool, args, code, note=""):
        """A call that must be refused with this error code."""
        ok, payload = self.call(tool, args, note or f"expects {code}")
        got = (payload.get("error") or {}).get("code") if isinstance(payload, dict) else None
        self.check(f"{tool} refuses with {code}", (not ok) and got == code, f"got {got}")
        return payload


def scenario_discovery(run):
    """What the server publishes, and that the cheap reads answer."""
    mcp = run.mcp
    run.check("server instructions are published", bool(mcp.server.get("instructions")))
    run.check(f"tools/list is within {MAX_TOOLS_LIST_BYTES} B",
              mcp.tools_list_bytes <= MAX_TOOLS_LIST_BYTES, f"{mcp.tools_list_bytes} B")
    undocumented = [t["name"] for t in mcp.tools if not (t.get("description") or "").strip()]
    run.check("every tool has a description", not undocumented, str(undocumented))
    read_only = [t for t in mcp.tools if (t.get("annotations") or {}).get("readOnlyHint")]
    run.check("read-only tools are announced", len(read_only) >= 20, f"{len(read_only)}")
    with open(os.path.join(HERE, "tools.golden.json")) as fh:
        golden = set(json.load(fh))
    served = {t["name"] for t in mcp.tools}
    run.check("the served tool set matches tools.golden.json", served == golden,
              f"difference: {sorted(served ^ golden)[:6]}")

    prompts = mcp.list_prompts()
    run.check("prompts are published", len(prompts) >= 3, f"{len(prompts)}")
    if prompts:
        text = json.dumps(mcp.get_prompt(prompts[0]["name"]))
        run.check("a prompt renders", len(text) > 200)

    ok, namespaces = run.call("get_namespaces")
    run.check("the test namespace exists",
              ok and any(n.get("id") == NS for n in namespaces.get("ns") or []))
    ok, connections = run.call("get_connections", {"provider": PROVIDER})
    names = (connections.get("connections") or {}).get(PROVIDER) or []
    run.check(f"{PROVIDER} has verified connections", ok and bool(names),
              "CB-Tumblebug is not initialized with credentials (make k-init)")
    ok, guide = run.call("get_usage_guide", {"topic": "detail"})
    for topic in ("provision", "initscript", "commands", "gpu", "recovery", "cleanup",
                  "services"):
        ok, guide = run.call("get_usage_guide", {"topic": topic})
        run.check(f"usage guide '{topic}' is served", ok and len(guide.get("guide", "")) > 300)
    run.call("get_search_options", {"kind": "spec"})
    run.call("get_search_options", {"kind": "image"})
    ok, scripts = run.call("list_predefined_scripts")
    first = next(iter(scripts.get("scripts") or {}), None)
    if first:
        ok, script = run.call("list_predefined_scripts", {"name": first})
        run.check("a predefined script lists its commands", ok and bool(script.get("commands")))
    run.call("get_platform_services")
    run.call("get_budget_status")
    run.call("list_credential_holders")
    run.call("list_infra_templates")
    run.call("list_init_script_templates")
    run.call("get_dns_hosted_zones", {"check_delegation": False})
    run.call("manage_object_storage", {"action": "support"})
    run.call("manage_k8s_cluster", {"action": "info"})
    ok, out = run.call("manage_rdbms", {"action": "support"})
    run.check("manage_rdbms reports what each provider offers", ok and bool(out.get("supports")))
    ok, out = run.call("inspect_csp_resources", {"connection_name": f"{PROVIDER}-{REGION}",
                                                 "resource_type": "audit"})
    run.check("the provider audit answers", ok and "clean" in out, str(out)[:200])
    for kind in ("vNet", "securityGroup", "sshKey", "customImage", "objectStorage", "nlb"):
        ok, _ = run.call("get_resources", {"kind": kind, "ns_id": NS})
        run.check(f"get_resources({kind}) answers", ok)
    return bool(names)


def scenario_plan(run):
    """Spec, image and a reviewed plan - the path every creation depends on."""
    ok, out = run.call("recommend_vm_spec", {
        "filter_policies": {"providerName": PROVIDER, "regionName": REGION,
                            "vCPU": {"min": 1, "max": 2}, "memoryGiB": {"min": 1, "max": 4}},
        "priority_policy": "cost", "limit": 5}, f"{PROVIDER} {REGION}")
    specs = out.get("specs") or []
    if not run.check("recommend_vm_spec returns priced specs",
                     ok and specs and specs[0].get("costPerHour", -1) >= 0, str(out)[:200]):
        return False
    run.spec = specs[0]["id"]
    print(f"         -> {run.spec} ${specs[0]['costPerHour']}/h")

    ok, out = run.call("search_images", {"matched_spec_id": run.spec, "os_type": "ubuntu 22.04",
                                         "max_results": 3})
    images = out.get("imageList") or []
    if not run.check("search_images returns an image for the spec", ok and bool(images)):
        return False
    run.image = images[0]["cspImageName"]
    print(f"         -> {run.image}")

    ok, out = run.call("get_spec_options", {"spec_id": run.spec, "image_id": run.image})
    run.check("get_spec_options reports availability and disk choices",
              ok and "status" in (out.get("availability") or {}) and "disk" in out)
    ok, out = run.call("review_spec_image_pair", {"spec_id": run.spec, "image_id": run.image})
    run.check("review_spec_image_pair accepts the pair", ok and out.get("isValid") is True,
              str(out.get("message")))
    run.call("get_provisioning_risk", {"spec_id": run.spec, "csp_image_name": run.image})
    other = "gcp" if PROVIDER != "gcp" else "aws"
    ok, out = run.call("recommend_alternative_node_config",
                       {"source_spec_id": run.spec, "target_provider": other}, f"-> {other}")
    run.check("recommend_alternative_node_config proposes candidates",
              ok and bool(out.get("candidates")))

    run.groups = [{"name": "web", "specId": run.spec, "imageId": run.image,
                   "nodeGroupSize": 1, "label": {"role": "web"}}]
    plan = {"ns_id": NS, "name": f"{PREFIX}-plan", "node_groups": run.groups}
    ok, review = run.call("review_infra_dynamic_request", plan, "the plan")
    run.check("the plan is viable", ok and review.get("creationViable") is True,
              str(review.get("overallMessage")))
    run.check("the review counts machines", review.get("totalNodeCount") == 1)
    run.check("the review stays small", run.mcp.trace[-1]["bytes"] < 6000,
              f"{run.mcp.trace[-1]['bytes']} B")
    return bool(review.get("creationViable"))


def scenario_gates(run):
    """Refusals, error codes, and metadata round trips that create nothing billable."""
    groups = run.groups or [{"name": "web", "specId": f"{PROVIDER}+{REGION}+none",
                             "imageId": "none"}]
    plan = {"ns_id": NS, "name": f"{PREFIX}-gate", "node_groups": groups}
    run.expect_code("create_infra_dynamic", plan, "REVIEW_REQUIRED")
    refusal = run.expect_code("create_infra_dynamic", {**plan, "force_create": True},
                              "NEEDS_CONFIRMATION")
    detail = (refusal.get("error") or {}).get("detail") or {}
    run.check("the refusal carries the plan and its cost",
              bool(detail.get("plan")) and "usd_per_hour" in (detail.get("totals") or {}))
    run.expect_code("create_infra_dynamic",
                    {**plan, "node_groups": [{"specId": groups[0]["specId"]}]},
                    "INVALID_ARGUMENT")
    run.expect_code("terminate_infra", {"ns_id": NS, "infra_id": f"{PREFIX}-none"},
                    "NEEDS_CONFIRMATION")
    run.expect_code("control_infra", {"ns_id": NS, "infra_id": f"{PREFIX}-none",
                                      "action": "terminate"}, "NEEDS_CONFIRMATION")
    run.expect_code("control_infra", {"ns_id": NS, "infra_id": f"{PREFIX}-none",
                                      "action": "restart"}, "INVALID_ARGUMENT")
    run.expect_code("control_node", {"ns_id": NS, "infra_id": f"{PREFIX}-none",
                                     "node_id": "g1-1", "action": "delete"},
                    "NEEDS_CONFIRMATION")
    run.expect_code("get_infra", {"ns_id": NS, "infra_id": f"{PREFIX}-none"}, "NOT_FOUND")
    run.expect_code("diagnose_infra", {"ns_id": NS, "infra_id": f"{PREFIX}-none"}, "NOT_FOUND")
    run.expect_code("execute_command_infra", {"ns_id": NS, "infra_id": f"{PREFIX}-none",
                                              "commands": ["hostname"]}, "NOT_FOUND")
    run.expect_code("get_resources", {"kind": "volume"}, "INVALID_ARGUMENT")
    run.expect_code("recommend_vm_spec", {"priority_policy": "location"}, "INVALID_ARGUMENT")
    run.expect_code("manage_resource", {"action": "delete", "kind": "vNet",
                                        "resource_id": f"{PREFIX}-none"}, "NEEDS_CONFIRMATION")
    run.expect_code("manage_object_storage", {"action": "create", "bucket_id": "Bad_Name",
                                              "connection_name": f"{PROVIDER}-{REGION}"},
                    "INVALID_ARGUMENT")
    run.expect_code("manage_k8s_cluster", {"action": "create", "cluster_id": f"{PREFIX}-k8s",
                                           "spec_id": groups[0]["specId"]},
                    "NEEDS_CONFIRMATION")
    run.expect_code("manage_nlb", {"action": "create", "scope": "GLOBAL", "ns_id": NS,
                                   "infra_id": f"{PREFIX}-none"}, "NEEDS_CONFIRMATION")
    run.expect_code("request_budget_increase", {"request_id": "req-none", "reason": "test"},
                    "NOT_FOUND")
    run.expect_code("manage_object_storage", {"action": "get", "bucket_id": f"{PREFIX}-none"},
                    "NOT_FOUND")
    run.expect_code("manage_rdbms", {"action": "get", "rdbms_id": f"{PREFIX}-none"}, "NOT_FOUND")
    run.expect_code("manage_rdbms", {"action": "create", "rdbms_id": f"{PREFIX}-db"},
                    "INVALID_ARGUMENT")
    run.expect_code("release_resources", {"ns_id": NS}, "NEEDS_CONFIRMATION")

    # Namespace round trip, with an idempotent retry
    name = f"{PREFIX}-ns"
    key = f"{PREFIX}-{int(time.time())}"
    ok, first = run.call("manage_namespace", {"action": "create", "name": name,
                                              "description": "scenario", "idempotency_key": key})
    run.check("a namespace is created", ok and first.get("created") is True, str(first)[:160])
    ok, again = run.call("manage_namespace", {"action": "create", "name": name,
                                              "description": "scenario", "idempotency_key": key},
                         "same idempotency_key")
    run.check("a retry with the same idempotency_key replays the first result",
              ok and again.get("_idempotent_replay") is True and again.get("created") is True)
    run.expect_code("manage_namespace", {"action": "delete", "name": name}, "NEEDS_CONFIRMATION")
    ok, _ = run.call("manage_namespace", {"action": "delete", "name": name, "confirm": True})
    run.check("the namespace is deleted", ok)

    # Infra template round trip
    if run.groups:
        template = f"{PREFIX}-template"
        run.call("manage_infra_template", {"action": "delete", "ns_id": NS,
                                           "template_id": template, "confirm": True},
                 "clear a leftover")
        ok, _ = run.call("manage_infra_template", {
            "action": "create", "ns_id": NS,
            "template": {"name": template, "description": "scenario",
                         "infraDynamicReq": {"name": "x", "nodeGroups": run.groups}}})
        run.check("an Infra template is stored", ok)
        ok, listed = run.call("list_infra_templates", {"ns_id": NS})
        run.check("the template is listed with its node count",
                  ok and any(t.get("id") == template and t.get("nodes") == 1
                             for t in listed.get("templates") or []))
        refusal = run.expect_code("provision_infra_from_template",
                                  {"ns_id": NS, "template_id": template,
                                   "name": f"{PREFIX}-from-template"}, "NEEDS_CONFIRMATION")
        totals = ((refusal.get("error") or {}).get("detail") or {}).get("totals") or {}
        run.check("the template is priced before it is used",
                  totals.get("nodes") == 1 and totals.get("usd_per_hour", 0) > 0, str(totals))
        ok, _ = run.call("manage_infra_template", {"action": "delete", "ns_id": NS,
                                                   "template_id": template, "confirm": True})
        run.check("the template is deleted", ok)

    # Init script template round trip
    script = f"{PREFIX}-initscript"
    run.call("manage_init_script_template", {"action": "delete", "ns_id": NS,
                                             "template_id": script, "confirm": True},
             "clear a leftover")
    ok, _ = run.call("manage_init_script_template", {
        "action": "create", "ns_id": NS,
        "template": {"name": script, "description": "scenario", "env": {"GREETING": "hello"},
                     "scripts": [{"name": "greet",
                                  "script": "#!/bin/bash\necho $GREETING > /tmp/greeting\n"}]}})
    run.check("an init script template is stored", ok)
    ok, got = run.call("list_init_script_templates", {"ns_id": NS, "template_id": script})
    run.check("the init script template returns its script",
              ok and "GREETING" in json.dumps(got))
    ok, _ = run.call("manage_init_script_template", {"action": "delete", "ns_id": NS,
                                                     "template_id": script, "confirm": True})
    run.check("the init script template is deleted", ok)

    # The journal holds what changed something, and none of the refusals
    ok, journal = run.call("resume_session", {"session_id": run.mcp.session_id})
    actions = journal.get("last_actions") or []
    run.check("the session journal recorded the changes",
              ok and any(a.get("tool") == "manage_namespace" for a in actions))
    run.check("refusals are not journaled",
              not any(a.get("outcome") in ("NEEDS_CONFIRMATION", "REVIEW_REQUIRED",
                                           "INVALID_ARGUMENT") for a in actions))
    run.call("clear_session", {"session_id": run.mcp.session_id, "confirm": True})
    return True


def scenario_existing(run):
    """Reads of whatever Infras are already there."""
    ok, listing = run.call("get_infra_list", {"ns_id": NS, "option": "id"})
    ids = [i for i in (listing.get("output") or []) if isinstance(i, str)][:3]
    if not ids:
        print("      (no Infra in the namespace; nothing to read)")
        return True
    for infra_id in ids:
        target = {"ns_id": NS, "infra_id": infra_id}
        ok, infra = run.call("get_infra", target)
        run.check(f"get_infra({infra_id}) answers", ok)
        run.check(f"get_infra({infra_id}) summary stays small",
                  run.mcp.trace[-1]["bytes"] < 40_000, f"{run.mcp.trace[-1]['bytes']} B")
        ok, health = run.call("diagnose_infra", target)
        run.check(f"diagnose_infra({infra_id}) reports health", ok and "health" in health)
        run.call("get_nodegroups", target)
        run.call("get_infra_associated_resources", target)
        run.call("get_init_script_status", target)
        run.call("manage_command_task", {"action": "list", **target})
        run.call("manage_nlb", {"action": "list", **target})
    return True


def _hourly(review):
    for word in str(review.get("estimatedCost") or "").replace("$", " ").replace("/", " ").split():
        try:
            return float(word)
        except ValueError:
            continue
    return 0.0


def bucket_round_trip(run, target):
    """A bucket, an upload from the node through a presigned URL, and the same object back."""
    bucket = f"{PREFIX}-{int(time.time())}"
    ok, made = run.call("manage_object_storage", {
        "action": "create", "ns_id": NS, "bucket_id": bucket,
        "connection_name": f"{PROVIDER}-{REGION}", "description": "MCP scenario run"})
    if not run.check("a bucket is created", ok, json.dumps(made)[:300]):
        return
    bucket = made.get("id") or bucket
    where = {"ns_id": NS, "bucket_id": bucket}
    key = "scenario/hello.txt"
    try:
        ok, up = run.call("manage_object_storage", {
            **where, "action": "presigned_url", "object_key": key, "operation": "upload"})
        url = up.get("presignedURL")
        if not run.check("an upload URL is issued", ok and bool(url), json.dumps(up)[:300]):
            return
        headers = " ".join(f"-H '{k}: {v}'" for k, v in (up.get("requiredHeaders") or {}).items())
        ok, sent = run.call("execute_command_infra", {**target, "commands": [
            f"echo hello-from-node > /tmp/up.txt && curl -fsS -X PUT {headers} "
            f"-T /tmp/up.txt '{url}' && echo upload-ok"]}, "node uploads with the URL")
        run.check("the node uploads through the presigned URL",
                  ok and "upload-ok" in json.dumps(sent), json.dumps(sent)[:300])
        ok, listed = run.call("manage_object_storage", {**where, "action": "list_objects"})
        run.check("the object is listed", ok and key in json.dumps(listed))
        ok, down = run.call("manage_object_storage", {
            **where, "action": "presigned_url", "object_key": key, "operation": "download"})
        ok, got = run.call("execute_command_infra", {**target, "commands": [
            f"curl -fsS '{down.get('presignedURL')}'"]}, "node downloads with the URL")
        run.check("the object reads back unchanged",
                  ok and "hello-from-node" in json.dumps(got), json.dumps(got)[:300])
        ok, _ = run.call("manage_object_storage", {**where, "action": "delete_object",
                                                   "object_key": key, "confirm": True})
        run.check("the object is deleted", ok)
    finally:
        ok, gone = run.call("manage_object_storage", {**where, "action": "delete",
                                                      "force": True, "confirm": True})
        run.check("the bucket is deleted", ok, json.dumps(gone)[:300])


def scenario_lifecycle(run):
    """Create one node, operate it, grow and shrink the group, and remove everything."""
    if not run.groups:
        run.check("a reviewed plan is available for the lifecycle", False, "run 'plan' first")
        return False
    infra = f"{PREFIX}-life"
    target = {"ns_id": NS, "infra_id": infra}
    _clear_leftover(run, target)
    plan = {"ns_id": NS, "name": infra, "node_groups": run.groups,
            "label": {"purpose": "mcp-scenario"}, "description": "MCP scenario run"}
    ok, review = run.call("review_infra_dynamic_request", plan)
    cost = _hourly(review)
    if not run.check(f"the plan costs under ${MAX_USD_PER_HOUR}/h",
                     ok and review.get("creationViable") and 0 < cost * 2 <= MAX_USD_PER_HOUR,
                     f"${cost}/h per node"):
        return False

    created = False
    try:
        ok, made = run.call("create_infra_dynamic",
                            {**plan, "force_create": True, "confirm": True}, "CREATE")
        created = ok or "already exists" in json.dumps(made)
        if not run.check("the Infra is created", ok, json.dumps(made)[:300]):
            return False
        nodes = made.get("node") or []
        run.check("the node is Running", bool(nodes) and nodes[0].get("status") == "Running",
                  str([n.get("status") for n in nodes]))
        node_id = nodes[0]["id"] if nodes else "web-1"
        uid = made.get("uid")

        ok, read = run.call("get_infra", target)
        run.check("get_infra shows the node with an address",
                  ok and bool((read.get("node") or [{}])[0].get("publicIP")))
        ok, health = run.call("diagnose_infra", target)
        run.check("diagnose_infra reports healthy", ok and health.get("health") == "healthy",
                  str(health.get("health")))
        run.call("get_nodegroups", {**target, "nodegroup_id": "web"})
        ok, used = run.call("get_infra_associated_resources", target)
        groups = used.get("securityGroupIds") or []
        run.call("get_init_script_status", target)

        # A node that just came up can refuse SSH for a moment
        for attempt in range(4):
            ok, out = run.call("execute_command_infra",
                               {**target, "commands": ["hostname && echo scenario-ok"]},
                               f"attempt {attempt + 1}")
            if ok and (out.get("nodes") or {}).get("succeeded") == 1:
                break
            time.sleep(20)
        run.check("a command runs on the node",
                  ok and "scenario-ok" in json.dumps(out.get("results")), json.dumps(out)[:300])

        ok, started = run.call("execute_command_infra",
                               {**target, "commands": ["sleep 3; echo async-ok"],
                                "run_async": True})
        request_id = started.get("request_id")
        run.check("an async command returns a request id", ok and bool(request_id))
        state = {}
        for _ in range(12):
            time.sleep(5)
            ok, state = run.call("get_command_result", {"request_id": request_id, **target})
            if state.get("status") not in ("Handling", "Pending"):
                break
        run.check("the async command completes", state.get("status") == "Completed"
                  and "async-ok" in json.dumps(state), json.dumps(state)[:300])
        ok, history = run.call("list_node_command_status", {**target, "node_id": node_id})
        run.check("the node's command history lists both commands",
                  ok and len(history.get("commands") or []) >= 2)
        run.call("manage_command_task", {"action": "list", **target})

        ok, _ = run.call("transfer_file_infra", {**target, "target_path": "/tmp",
                                                 "file_name": "scenario.txt",
                                                 "content": "written through MCP\n"})
        ok, back = run.call("download_file_from_node", {**target, "node_id": node_id,
                                                        "source_path": "/tmp/scenario.txt"})
        run.check("a file written to the node reads back unchanged",
                  ok and back.get("content") == "written through MCP\n", str(back)[:200])

        bucket_round_trip(run, target)

        if uid:
            ok, _ = run.call("manage_labels", {"action": "set", "label_type": "infra",
                                               "uid": uid, "labels": {"scenario": "yes"}})
            ok, labels = run.call("manage_labels", {"action": "get", "label_type": "infra",
                                                    "uid": uid})
            run.check("a label is set and read back", ok and "scenario" in json.dumps(labels))
            ok, found = run.call("manage_labels", {"action": "find", "label_type": "infra",
                                                   "labels": "scenario=yes"})
            run.check("the Infra is found by its label", ok and infra in json.dumps(found))
            run.call("manage_labels", {"action": "remove", "label_type": "infra",
                                       "uid": uid, "key": "scenario"})

        if groups:
            rule = [{"Ports": "8080", "Protocol": "TCP"}]
            ok, opened = run.call("add_security_group_rules",
                                  {"ns_id": NS, "sg_id": groups[0], "rules": rule})
            run.check("a firewall rule is added", ok and "8080" in json.dumps(opened))
            ok, _ = run.call("remove_security_group_rules",
                             {"ns_id": NS, "sg_id": groups[0], "rules": rule, "confirm": True})
            run.check("the firewall rule is removed", ok)
            run.call("get_resources", {"kind": "securityGroup", "ns_id": NS,
                                       "resource_id": groups[0]})

        run.expect_code("scale_out_nodegroup", {**target, "nodegroup_id": "web"},
                        "NEEDS_CONFIRMATION")
        ok, grown = run.call("scale_out_nodegroup",
                             {**target, "nodegroup_id": "web", "add_count": 1, "confirm": True},
                             "SCALE OUT")
        added = grown.get("addedNodeIds") or []
        run.check("the node group grows by one", ok and len(added) == 1, json.dumps(grown)[:300])
        if added:
            for _ in range(8):
                ok, gone = run.call("control_node", {**target, "node_id": added[0],
                                                     "action": "delete", "confirm": True})
                if ok or (gone.get("error") or {}).get("code") != "TIMEOUT":
                    break
                time.sleep(10)
            run.check("the added node is deleted", ok, json.dumps(gone)[:300])

        template = f"{PREFIX}-saved"
        ok, _ = run.call("manage_infra_template", {"action": "save_from_infra", "ns_id": NS,
                                                   "infra_id": infra, "template_id": template})
        run.check("the Infra is saved as a template", ok)
        run.call("manage_infra_template", {"action": "delete", "ns_id": NS,
                                           "template_id": template, "confirm": True})
    finally:
        if created:
            run.call("release_resources", {"ns_id": NS, "dry_run": True})
            _teardown(run, target)
    run.expect_code("get_infra", target, "NOT_FOUND", "gone")
    return True


def wait_for(run, tool, args, done, tries=40, pause=30):
    """Poll a read until done(ok, payload) holds."""
    ok, out = False, {}
    for _ in range(tries):
        ok, out = run.call(tool, args)
        if done(ok, out):
            break
        time.sleep(pause)
    return ok, out


def _code(payload):
    return (payload.get("error") or {}).get("code") if isinstance(payload, dict) else None


def _serves_http(address, tries=12, pause=15):
    for _ in range(tries):
        try:
            with urllib.request.urlopen(f"http://{address}/", timeout=10) as reply:
                if reply.status == 200:
                    return True
        except OSError:
            pass
        time.sleep(pause)
    return False


def _teardown(run, target):
    """Terminate the Infra and release what it leaves behind."""
    done, code = {}, None
    for _ in range(6):
        ok, done = run.call("terminate_infra", {**target, "confirm": True}, "TERMINATE")
        code = _code(done)
        if ok or code == "NOT_FOUND":
            break
        time.sleep(20)
    run.check("the Infra is terminated", ok or code == "NOT_FOUND", json.dumps(done)[:300])
    time.sleep(10)
    ok, freed = run.call("release_resources", {"ns_id": NS, "confirm": True}, "RELEASE")
    run.check("shared resources are released",
              ok and not freed.get("failedCount"), json.dumps(freed)[:300])


def _clear_leftover(run, target):
    """Remove the Infra an interrupted run left behind, so this one starts clean."""
    ok, _ = run.call("get_infra", {**target, "detail": "minimal"}, "leftover from an earlier run?")
    if ok:
        _teardown(run, target)


def scenario_services(run):
    """Two web nodes behind both kinds of load balancer, a custom image, a managed database."""
    if not run.groups:
        run.check("a reviewed plan is available", False, "run 'plan' first")
        return False
    infra, image, database = f"{PREFIX}-svc", f"{PREFIX}-img", f"{PREFIX}-db"
    target = {"ns_id": NS, "infra_id": infra}
    _clear_leftover(run, target)
    groups = [{**run.groups[0], "nodeGroupSize": 2}]
    plan = {"ns_id": NS, "name": infra, "node_groups": groups,
            "description": "MCP scenario run"}
    ok, review = run.call("review_infra_dynamic_request", plan)
    if not run.check("the plan is viable", ok and review.get("creationViable") is True,
                     str(review.get("overallMessage"))):
        return False

    created = made_db = made_image = False
    try:
        ok, made = run.call("create_infra_dynamic",
                            {**plan, "force_create": True, "confirm": True}, "CREATE")
        created = ok or "already exists" in json.dumps(made)
        if not run.check("the Infra is created", ok, json.dumps(made)[:300]):
            return False
        nodes = made.get("node") or []
        first = nodes[0] if nodes else {}
        group, vnet = first.get("securityGroupIds", [None])[0], first.get("vNetId")

        for attempt in range(4):
            ok, out = run.call("execute_command_infra",
                               {**target, "script_name": "nginx_server"}, f"attempt {attempt + 1}")
            if ok and (out.get("nodes") or {}).get("failed") == 0:
                break
            time.sleep(20)
        run.check("the predefined script installs a web server on both nodes",
                  ok and (out.get("nodes") or {}).get("succeeded") == 2, json.dumps(out)[:300])
        ok, opened = run.call("add_security_group_rules", {
            "ns_id": NS, "sg_id": group, "rules": [{"Ports": "80", "Protocol": "TCP"}]})
        run.check("port 80 is opened", ok and '"80"' in json.dumps(opened))
        ok, again = run.call("add_security_group_rules", {
            "ns_id": NS, "sg_id": group, "rules": [{"Ports": "80", "Protocol": "TCP"}]})
        run.check("adding the same rule again is not an error", ok, json.dumps(again)[:200])

        # the provider's load balancer in front of the node group
        ok, nlb = run.call("manage_nlb", {**target, "action": "create",
                                          "nodegroup_id": "web", "port": 80}, "REGIONAL NLB")
        dns = (nlb.get("listener") or {}).get("dnsName")
        if run.check("a regional load balancer is created", ok and bool(dns),
                     json.dumps(nlb)[:300]):
            ok, health = wait_for(run, "manage_nlb",
                                  {**target, "action": "health", "nlb_id": "web"},
                                  lambda ok, h: ok and len(h.get("healthyNodes") or []) == 2,
                                  tries=20)
            run.check("both nodes pass its health check",
                      len(health.get("healthyNodes") or []) == 2, json.dumps(health)[:300])
            run.check("the load balancer serves the page", _serves_http(dns))
            run.expect_code("manage_nlb", {**target, "action": "delete", "nlb_id": "web"},
                            "NEEDS_CONFIRMATION")
            ok, _ = run.call("manage_nlb", {**target, "action": "delete", "nlb_id": "web",
                                            "confirm": True})
            run.check("the regional load balancer is deleted", ok)

        # HAProxy on its own Infra in front of every node
        run.expect_code("manage_nlb", {**target, "action": "create", "scope": "GLOBAL"},
                        "NEEDS_CONFIRMATION")
        ok, glb = run.call("manage_nlb", {**target, "action": "create", "scope": "GLOBAL",
                                          "port": 80, "confirm": True}, "GLOBAL NLB")
        addresses = glb.get("addresses") or []
        if run.check("a global load balancer is created with its port open",
                     ok and bool(addresses) and not glb.get("listenerNotOpenedOn"),
                     json.dumps(glb)[:300]):
            run.check("the global load balancer serves the page", _serves_http(addresses[0]))
        if ok or _code(glb) != "NEEDS_CONFIRMATION":
            ok, _ = run.call("terminate_infra", {"ns_id": NS, "infra_id": f"{infra}-nlb",
                                                 "confirm": True}, "TERMINATE NLB HOST")
            run.check("its host Infra is terminated", ok)

        # an image of a node, and a node started from it
        ok, snap = run.call("create_custom_image", {**target, "name": image,
                                                    "node_id": first.get("id")}, "SNAPSHOT")
        made_image = ok
        if run.check("a custom image is requested", ok, json.dumps(snap)[:300]):
            ok, ready = wait_for(run, "get_resources",
                                 {"kind": "customImage", "ns_id": NS, "resource_id": image},
                                 lambda ok, r: not ok or r.get("status") == "Available")
            run.check("the custom image becomes Available", ok and ready.get("status") == "Available",
                      json.dumps(ready)[:300])
            extra = {**target, "spec_id": run.spec, "image_id": image, "name": "fromimg",
                     "node_group_size": 1}
            ok, checked = run.call("review_nodegroup_dynamic", extra)
            run.check("the node group from the image passes review", ok, json.dumps(checked)[:300])
            ok, grown = run.call("add_nodegroup_dynamic", {**extra, "confirm": True},
                                 "NODE FROM IMAGE")
            run.check("a node group starts from the custom image", ok, json.dumps(grown)[:300])
            if ok:
                ok, out = wait_for(run, "execute_command_infra",
                                   {**target, "nodegroup_id": "fromimg",
                                    "commands": ["systemctl is-active nginx && echo web-ready"]},
                                   lambda ok, o: ok and (o.get("nodes") or {}).get("failed") == 0,
                                   tries=4, pause=20)
                run.check("the new node has what was installed before the snapshot",
                          ok and "web-ready" in json.dumps(out.get("results")),
                          json.dumps(out)[:300])
                ok, gone = wait_for(run, "control_node",
                                    {**target, "node_id": "fromimg-1", "action": "delete",
                                     "confirm": True},
                                    lambda ok, g: ok or _code(g) != "TIMEOUT", tries=8, pause=10)
                run.check("the node from the image is deleted", ok, json.dumps(gone)[:300])

        # a managed database in the Infra's network, reached from a node
        ok, net = run.call("get_resources", {"kind": "vNet", "ns_id": NS, "resource_id": vnet})
        subnets = [x.get("id") for x in net.get("subnetInfoList") or []]
        password = "Mcp" + secrets.token_hex(8) + "x9"
        request = {"ns_id": NS, "rdbms_id": database, "connection_name": f"{PROVIDER}-{REGION}",
                   "vnet_id": vnet, "subnet_ids": subnets, "security_group_ids": [group],
                   "admin_user_name": "dbadmin", "admin_user_password": password}
        ok, valid = run.call("manage_rdbms", {**request, "action": "validate"})
        run.check("the database request validates and the password is not echoed",
                  ok and password not in json.dumps(valid), json.dumps(valid)[:300])
        run.expect_code("manage_rdbms", {**request, "action": "create"}, "NEEDS_CONFIRMATION")
        ok, db = run.call("manage_rdbms", {**request, "action": "create", "confirm": True},
                          "CREATE DATABASE INSTANCE")
        made_db = ok or "already exists" in json.dumps(db)
        if run.check("a managed database is created", ok, json.dumps(db)[:300]):
            ok, db = wait_for(run, "manage_rdbms",
                              {"ns_id": NS, "action": "get", "rdbms_id": database},
                              lambda ok, d: not ok or d.get("status") == "Available")
            endpoint = str(db.get("endpoint") or "")
            run.check("the database becomes Available with an endpoint",
                      db.get("status") == "Available" and ":" in endpoint, json.dumps(db)[:300])
            host, _, port = endpoint.rpartition(":")
            run.call("add_security_group_rules", {"ns_id": NS, "sg_id": group, "rules": [
                {"Ports": port, "Protocol": "TCP", "CIDR": net.get("cidrBlock") or "10.0.0.0/8"}]})
            ok, out = run.call("execute_command_infra", {
                **target, "node_id": first.get("id"), "timeout_minutes": 5,
                "env": {"MYSQL_PWD": password},
                "commands": ["sudo apt-get install -y -qq mysql-client >/dev/null 2>&1; "
                             f"mysql -h {host} -P {port} -u dbadmin "
                             "-e 'create database if not exists app; select 42 as answer'"]},
                "node queries the database")
            run.check("a node reaches the database over the private network",
                      ok and "42" in json.dumps(out.get("results")), json.dumps(out)[:400])
            run.check("the password does not come back in the command result",
                      password not in json.dumps(out))
    finally:
        if made_db:
            ok, gone = run.call("manage_rdbms", {"ns_id": NS, "action": "delete",
                                                 "rdbms_id": database, "confirm": True},
                                "DELETE DATABASE INSTANCE")
            ok, state = wait_for(run, "manage_rdbms",
                                 {"ns_id": NS, "action": "get", "rdbms_id": database},
                                 lambda ok, d: not ok)
            run.check("the database is deleted", _code(state) == "NOT_FOUND",
                      json.dumps(state)[:300])
        if made_image:
            ok, _ = run.call("manage_resource", {"action": "delete", "kind": "customImage",
                                                 "resource_id": image, "ns_id": NS,
                                                 "confirm": True})
            run.check("the custom image is deleted", ok)
        if created:
            run.call("terminate_infra", {"ns_id": NS, "infra_id": f"{infra}-nlb",
                                         "confirm": True}, "host Infra, if still there")
            _teardown(run, target)
    return True


def scenario_kubernetes(run):
    """A provider-managed Kubernetes cluster: create, node group, kubeconfig, scale, delete."""
    cluster = f"{PREFIX}-k8s"
    where = {"ns_id": NS, "cluster_id": cluster}
    ok, out = run.call("recommend_vm_spec", {
        "filter_policies": {"providerName": PROVIDER, "regionName": REGION,
                            "vCPU": {"min": 2, "max": 2}, "memoryGiB": {"min": 4, "max": 8}},
        "priority_policy": "cost", "limit": 3})
    specs = out.get("specs") or []
    if not run.check("a node spec is found", ok and bool(specs)):
        return False
    spec = specs[0]["id"]
    ok, info = run.call("manage_k8s_cluster", {"action": "info", "spec_id": spec})
    traits = (info.get("k8s_cluster") or {}).get(PROVIDER) or {}
    run.check("info is narrowed to the provider", ok and bool(traits), json.dumps(info)[:200])
    ok, versions = run.call("manage_k8s_cluster", {"action": "versions", "spec_id": spec})
    version = versions[min(1, len(versions) - 1)]["id"] if ok and versions else None
    sizes = {"desired_node_size": 1, "min_node_size": 1, "max_node_size": 2}
    request = {**where, "action": "create", "spec_id": spec, "nodegroup_name": "ng1", **sizes}
    if version:
        request["version"] = version

    created = False
    try:
        run.expect_code("manage_k8s_cluster", request, "NEEDS_CONFIRMATION")
        ok, made = run.call("manage_k8s_cluster", {**request, "confirm": True}, "CREATE CLUSTER")
        created = ok or "already exists" in json.dumps(made)
        if not run.check("the cluster is requested", ok, json.dumps(made)[:300]):
            return False
        run.check("the answer says what to do next", bool(made.get("next")))
        ok, state = wait_for(run, "manage_k8s_cluster", {**where, "action": "get"},
                             lambda ok, c: not ok or c.get("status") == "Active", tries=60)
        if not run.check("the cluster becomes Active", ok and state.get("status") == "Active",
                         json.dumps(state)[:300]):
            return False
        if not state.get("k8sNodeGroupList"):
            run.check("the provider creates no node group with the cluster",
                      traits.get("nodegroups_on_creation") is False)
            ok, state = run.call("manage_k8s_cluster", {
                **where, "action": "add_nodegroup", "spec_id": spec, "nodegroup_name": "ng1",
                **sizes, "confirm": True}, "ADD NODE GROUP")
            run.check("a node group is added", ok, json.dumps(state)[:300])
        ok, state = wait_for(
            run, "manage_k8s_cluster", {**where, "action": "get"},
            lambda ok, c: not ok or [g.get("status") for g in c.get("k8sNodeGroupList") or []]
            == ["Active"], tries=30)
        run.check("the node group is Active",
                  [g.get("status") for g in state.get("k8sNodeGroupList") or []] == ["Active"],
                  json.dumps(state)[:300])

        run.expect_code("manage_k8s_cluster", {**where, "action": "kubeconfig"},
                        "NEEDS_CONFIRMATION")
        ok, access = run.call("manage_k8s_cluster", {**where, "action": "kubeconfig",
                                                     "confirm": True})
        text = str(access.get("kubeconfig") or "")
        run.check("the kubeconfig names the API server and needs no credential plugin",
                  ok and "server:" in text and "exec:" not in text, text[:120])
        ok, scaled = run.call("manage_k8s_cluster", {
            **where, "action": "scale", "nodegroup_name": "ng1", "desired_node_size": 2,
            "min_node_size": 1, "max_node_size": 3}, "SCALE")
        run.check("the node group is resized", ok and scaled.get("desiredNodeSize") == 2,
                  json.dumps(scaled)[:300])
        ok, blocked = run.call("release_resources", {"ns_id": NS, "dry_run": True})
        run.check("the release preview marks the cluster's network as in use",
                  ok and "inUseBy" in json.dumps(blocked), json.dumps(blocked)[:300])
    finally:
        if created:
            ok, gone = run.call("manage_k8s_cluster", {**where, "action": "delete",
                                                       "confirm": True}, "DELETE CLUSTER")
            if not ok and _code(gone) == "CONFLICT":
                run.call("manage_k8s_cluster", {**where, "action": "delete_nodegroup",
                                                "nodegroup_name": "ng1", "confirm": True})
                wait_for(run, "manage_k8s_cluster", {**where, "action": "get"},
                         lambda ok, c: not ok or not c.get("k8sNodeGroupList"), tries=40)
                ok, gone = wait_for(run, "manage_k8s_cluster",
                                    {**where, "action": "delete", "confirm": True},
                                    lambda ok, g: ok or _code(g) != "CONFLICT", tries=10)
            run.check("the cluster is deleted", ok or _code(gone) == "NOT_FOUND",
                      json.dumps(gone)[:300])
            ok, state = wait_for(run, "manage_k8s_cluster", {**where, "action": "get"},
                                 lambda ok, c: not ok, tries=40)
            run.check("the cluster record is gone", _code(state) == "NOT_FOUND")
            ok, freed = wait_for(run, "release_resources", {"ns_id": NS, "confirm": True},
                                 lambda ok, f: ok and not f.get("failedCount"), tries=6,
                                 pause=60)
            run.check("shared resources are released", ok and not freed.get("failedCount"),
                      json.dumps(freed)[:300])
    return True


SCENARIOS = {"discovery": scenario_discovery, "plan": scenario_plan, "gates": scenario_gates,
             "existing": scenario_existing}
# These create billable resources and run only with --spend
SPENDING = {"lifecycle": scenario_lifecycle, "services": scenario_services,
            "kubernetes": scenario_kubernetes}


def main():
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--only", choices=sorted(SCENARIOS) + sorted(SPENDING))
    parser.add_argument("--spend", action="store_true",
                        help="also run the lifecycle scenario, which creates a real VM; "
                             "required for --only lifecycle, services and kubernetes")
    args = parser.parse_args()
    sys.stdout.reconfigure(line_buffering=True)  # progress stays visible when redirected

    try:
        client = McpClient(session_id=f"scenario-{int(time.time())}", pacing=0.3)
    except McpError as exc:
        print(f"{exc}\nstart the endpoint first:  make k-port-forward")
        return 2
    run = Run(client)

    chosen = [args.only] if args.only else list(SCENARIOS)
    if args.spend and not args.only:
        chosen.append("lifecycle")
    if args.only in SPENDING and not args.spend:
        print(f"the {args.only} scenario creates billable resources; pass --spend to run it")
        return 2
    if args.only in ("gates", "lifecycle", "services"):
        chosen.insert(0, "plan")

    ready = True
    for name in chosen:
        fn = SCENARIOS.get(name) or SPENDING[name]
        print(f"\n=== {name}: {fn.__doc__.strip().splitlines()[0]}")
        if name in ("plan", *SPENDING) and not ready:
            print("      skipped: no verified connection for the test provider")
            run.skipped.append(name)
            continue
        try:
            result = fn(run)
        except McpError as exc:
            run.check(f"{name} ran to the end", False, str(exc))
            result = False
        if name == "discovery":
            ready = bool(result)

    called = {t["tool"] for t in client.trace}
    served = {t["name"] for t in client.tools}
    total = sum(t["bytes"] for t in client.trace)
    print("\n" + "=" * 78)
    print(f"  {len(client.trace)} calls, {total} B of responses, "
          f"{len(called & served)}/{len(served)} tools exercised")
    if run.skipped:
        print(f"  skipped: {', '.join(run.skipped)}")
    untouched = sorted(served - called)
    if untouched:
        print(f"  not exercised ({len(untouched)}): {', '.join(untouched)}")
    if run.failed:
        print(f"\n  FAILED ({len(run.failed)}):")
        for what in run.failed:
            print(f"    - {what}")
        return 1
    print("\n  OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
