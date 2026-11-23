#!/usr/bin/env python3
"""Measure what a session pays to use the CB-Tumblebug MCP server.

Sizes are of the whole JSON-RPC response over the wire, which is what the model's context
is charged for. Token figures are bytes/4: the absolute number is approximate, the ratio
against the baseline is the point.

    make k-port-forward                 # prerequisite: the MCP endpoint on :8080
    python3 bench.py                    # table against baseline.json
    python3 bench.py --save-baseline    # record the current numbers as the new baseline
    BENCH_INFRA=my-infra python3 bench.py   # also size the reads of an existing Infra
"""

import argparse
import json
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from mcp_client import McpClient, McpError  # noqa: E402

BASELINE = os.path.join(HERE, "baseline.json")
NS = os.environ.get("BENCH_NS", "default")
INFRA = os.environ.get("BENCH_INFRA", "")
NODE = os.environ.get("BENCH_NODE", "")
PROVIDER = os.environ.get("MCP_TEST_PROVIDER", "aws")
REGION = os.environ.get("MCP_TEST_REGION", "ap-northeast-2")
# CB-Tumblebug rate-limits bursts of reads, and a throttled reply measures nothing.
PACING_SECONDS = float(os.environ.get("BENCH_PACING", "0.5"))

SPEC_FILTER = {"providerName": PROVIDER, "regionName": REGION,
               "vCPU": {"min": 2, "max": 4}, "memoryGiB": {"min": 4}}

# One entry per thing a session commonly does. Add rather than edit, so runs stay comparable.
CASES = [
    ("get_namespaces", "get_namespaces", {}),
    ("get_connections", "get_connections", {}),
    ("get_connections:minimal", "get_connections", {"detail": "minimal"}),
    ("get_usage_guide:provision", "get_usage_guide", {"topic": "provision"}),
    ("get_search_options:spec", "get_search_options", {"kind": "spec"}),
    ("recommend_vm_spec", "recommend_vm_spec", {"filter_policies": SPEC_FILTER, "limit": 5}),
    ("recommend_vm_spec:full", "recommend_vm_spec",
     {"filter_policies": SPEC_FILTER, "limit": 5, "detail": "full"}),
    ("list_infra_templates", "list_infra_templates", {}),
    ("list_init_script_templates", "list_init_script_templates", {}),
    ("list_predefined_scripts", "list_predefined_scripts", {}),
    ("get_infra_list", "get_infra_list", {"ns_id": NS}),
    ("get_resources:vNet", "get_resources", {"kind": "vNet", "ns_id": NS}),
    ("get_resources:securityGroup", "get_resources", {"kind": "securityGroup", "ns_id": NS}),
    ("get_resources:sshKey", "get_resources", {"kind": "sshKey", "ns_id": NS}),
    ("get_platform_services", "get_platform_services", {}),
]
INFRA_CASES = [
    ("get_infra", "get_infra", {"ns_id": NS, "infra_id": INFRA}),
    ("get_infra:minimal", "get_infra", {"ns_id": NS, "infra_id": INFRA, "detail": "minimal"}),
    ("get_infra:full", "get_infra", {"ns_id": NS, "infra_id": INFRA, "detail": "full"}),
    ("diagnose_infra", "diagnose_infra", {"ns_id": NS, "infra_id": INFRA}),
    ("manage_command_task:list", "manage_command_task",
     {"action": "list", "ns_id": NS, "infra_id": INFRA}),
]


def measure(client, tool, args):
    """Size one call, retrying past the rate limiter."""
    for attempt in range(4):
        started = time.time()
        raw = client.call_raw(tool, args)
        if "RATE_LIMITED" not in raw:
            return len(raw), round(time.time() - started, 2)
        time.sleep(1.5 * (attempt + 1))
    raise RuntimeError("rate limited after 4 attempts")


def plan_cases(client):
    """A spec, its image and the review of a one-node plan - sized on real ids."""
    ok, specs = client.call("recommend_vm_spec", {"filter_policies": SPEC_FILTER, "limit": 1})
    spec = ((specs.get("specs") or [{}])[0]).get("id") if ok else None
    if not spec:
        return []
    images = {"matched_spec_id": spec, "os_type": "ubuntu 22.04"}
    ok, found = client.call("search_images", images)
    image = ((found.get("imageList") or [{}])[0]).get("cspImageName") if ok else None
    cases = [("search_images", "search_images", images),
             ("get_spec_options", "get_spec_options", {"spec_id": spec})]
    if image:
        groups = [{"name": "g1", "specId": spec, "imageId": image, "nodeGroupSize": 2}]
        cases += [
            ("review_spec_image_pair", "review_spec_image_pair",
             {"spec_id": spec, "image_id": image}),
            ("review_infra_dynamic_request", "review_infra_dynamic_request",
             {"ns_id": NS, "name": "bench-plan", "node_groups": groups}),
            ("create_infra_dynamic:refusal", "create_infra_dynamic",
             {"ns_id": NS, "name": "bench-plan", "node_groups": groups, "force_create": True}),
        ]
    return cases


def main():
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--csv", help="also write the numbers to this file")
    parser.add_argument("--save-baseline", action="store_true")
    args = parser.parse_args()

    try:
        client = McpClient(session_id="bench", verbose=False)
    except McpError as exc:
        print(f"{exc}\nstart the endpoint first:  make k-port-forward")
        return 2

    baseline = {}
    if os.path.exists(BASELINE):
        with open(BASELINE) as fh:
            baseline = json.load(fh).get("cases", {})

    results = {"tools/list": client.tools_list_bytes,
               "instructions": len(client.server.get("instructions") or "")}
    cases = CASES + plan_cases(client)
    if INFRA:
        cases += INFRA_CASES
        if NODE:
            cases.append(("list_node_command_status", "list_node_command_status",
                          {"ns_id": NS, "infra_id": INFRA, "node_id": NODE}))

    print(f"{'case':34} {'bytes':>9} {'~tokens':>8} {'secs':>6}  {'vs baseline':>12}")

    def row(label, size, secs=""):
        was = baseline.get(label)
        delta = f"{(size - was) * 100 // was:+d}%" if was else "-"
        print(f"{label:34} {size:9} {size // 4:8} {secs:>6}  {delta:>12}")

    row("tools/list", results["tools/list"])
    row("instructions", results["instructions"])
    for label, tool, tool_args in cases:
        try:
            size, secs = measure(client, tool, tool_args)
        except Exception as exc:
            print(f"{label:34} {'ERROR':>9}  {exc}")
            continue
        results[label] = size
        row(label, size, secs)
        time.sleep(PACING_SECONDS)

    total = sum(results.values())
    print(f"{'TOTAL':34} {total:9} {total // 4:8}")

    if args.csv:
        with open(args.csv, "w") as fh:
            fh.write("case,bytes,tokens_est\n")
            for label, size in results.items():
                fh.write(f"{label},{size},{size // 4}\n")
        print(f"\nwrote {args.csv}")
    if args.save_baseline:
        with open(BASELINE, "w") as fh:
            json.dump({"note": "bytes of the full JSON-RPC response over the wire",
                       "cases": results}, fh, indent=1, sort_keys=True)
            fh.write("\n")
        print(f"\nbaseline saved to {BASELINE}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
