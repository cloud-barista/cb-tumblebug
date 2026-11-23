#!/usr/bin/env python3
"""Helper for `make mcp-budget*`: reads etcd output on stdin, prints what to store or show.

    policy    merge ENABLED / PER_CREATION / PER_DAY / CONCURRENT into the stored policy
    approve   mark a pending request approved (refuses any other state)
    show      list the policy and the requests from `etcdctl get --prefix -w json`
"""

import base64
import json
import math
import os
import sys

LIMITS = (("per_creation_usd_per_hour", "PER_CREATION"),
          ("per_day_created_usd_per_hour", "PER_DAY"),
          ("concurrent_running_usd_per_hour", "CONCURRENT"))
DEFAULTS = {"per_creation_usd_per_hour": 10.0, "per_day_created_usd_per_hour": 100.0,
            "concurrent_running_usd_per_hour": 50.0}


def limit(variable):
    """A limit from the environment: dollars per hour, finite and not negative."""
    given = os.environ[variable]
    try:
        value = float(given)
    except ValueError:
        value = math.nan
    # NaN compares false against everything, which would switch the limit off
    if not math.isfinite(value) or value < 0:
        sys.exit(f"{variable} must be a number of 0 or more, got {given!r}")
    return value


def switch(given, stored):
    if not given:
        return stored
    if given.lower() in ("1", "true", "yes", "on"):
        return True
    if given.lower() in ("0", "false", "no", "off"):
        return False
    sys.exit(f"ENABLED must be true or false, got {given!r}")


def policy(raw):
    """The stored policy with the given values changed; the rest keeps its setting."""
    stored = json.loads(raw) if raw.strip() else {}
    merged = {**DEFAULTS, **stored}
    merged["enabled"] = switch(os.environ.get("ENABLED", ""), stored.get("enabled", True))
    for key, variable in LIMITS:
        if os.environ.get(variable):
            merged[key] = limit(variable)
    print(json.dumps(merged))


def approve(raw):
    if not raw.strip():
        sys.exit("no such request")
    record = json.loads(raw)
    if record.get("status") != "pending":
        sys.exit(f"request is {record.get('status')}, not pending - an approval works once")
    if not isinstance(record.get("planned_usd_per_hour"), (int, float)):
        sys.exit("request carries no plan to approve")
    record["status"] = "approved"
    print(json.dumps(record))


def show(raw):
    entries = {}
    for kv in (json.loads(raw) if raw.strip() else {}).get("kvs") or []:
        key = base64.b64decode(kv["key"]).decode()
        try:
            entries[key] = json.loads(base64.b64decode(kv.get("value", "")).decode())
        except ValueError:
            continue
    stored = entries.get("/mcp/policy/budget")
    current = {**DEFAULTS, **(stored or {})}
    state = "enforced" if current.get("enabled") and stored else "off"
    print(f"Spend limits: {state}")
    for key, variable in LIMITS:
        print(f"  {variable:<13} ${current[key]}/h")
    requests = {k.rsplit("/", 1)[-1]: v for k, v in entries.items()
                if k.startswith("/mcp/budget/requests/")}
    print(f"Requests: {len(requests)}")
    for name, record in sorted(requests.items()):
        print(f"  {name}  {record.get('status', '?'):<9} ${record.get('planned_usd_per_hour', '?')}/h"
              f"  {record.get('requested_at', '')}")
        for line in record.get("breaches") or []:
            print(f"      - {line}")
        if record.get("reason"):
            print(f"      reason: {record['reason']}")
        for group in record.get("node_groups") or []:
            if isinstance(group, dict):
                print(f"      {group.get('name', '-')}: {group.get('nodeGroupSize', 1)} x "
                      f"{group.get('specId')}")
    if any(r.get("status") == "pending" for r in requests.values()):
        print("Approve with: make mcp-budget-approve ID=<request id>")


if __name__ == "__main__":
    actions = {"policy": policy, "approve": approve, "show": show}
    if len(sys.argv) != 2 or sys.argv[1] not in actions:
        sys.exit(__doc__)
    actions[sys.argv[1]](sys.stdin.read())
