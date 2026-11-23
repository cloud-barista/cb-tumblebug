# CB-Tumblebug MCP server.
#
# Exposes the CB-Tumblebug REST API to MCP clients over Streamable HTTP (stateless) at
# http://<host>:8000/mcp. Run with `uv run ./tb-mcp.py`; configuration is read from the
# environment (see README.md). Checks: `make mcp-check`, `make mcp-scenarios`.

import asyncio
import os
import requests
import json
import logging
import re
import functools
import inspect
import base64
import math
import time
import socket
from typing import Dict, List, Optional, Any, Union
from datetime import datetime, timedelta
from fastmcp import FastMCP
from fastmcp.tools.tool import ToolResult
from mcp.types import TextContent

# This server utilizes fastmcp (https://github.com/jlowin/fastmcp)

# Configure logging - Reduce noise from HTTP connections and MCP protocol details
# Set logging level via environment variable (default: INFO)
log_level = os.environ.get("MCP_LOG_LEVEL", "INFO").upper()
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
TUMBLEBUG_CREDENTIAL_HOLDER = os.environ.get("TUMBLEBUG_CREDENTIAL_HOLDER", "")
host = os.environ.get("MCP_SERVER_HOST", "0.0.0.0") 
port = int(os.environ.get("MCP_SERVER_PORT", "8000"))

# Output startup information for debugging using logger instead of print
logger.info(f"Tumblebug API URL: {TUMBLEBUG_API_BASE_URL}")
logger.info(f"Username: {TUMBLEBUG_USERNAME}")
logger.info(f"Password configured: {'Yes' if TUMBLEBUG_PASSWORD else 'No'}")
logger.info(f"Logging level: {log_level}")
logger.info(f"MCP Server will start on {host}:{port}")

# Sent to the client at initialize; most clients place it in the model's system prompt.
SERVER_INSTRUCTIONS = "\n\n".join([
    "CB-Tumblebug provisions and operates multi-cloud infrastructure. An Infra is a set of "
    "node groups, each a number of identical nodes on one provider and region.",
    "Typical flow: recommend_vm_spec -> search_images(matched_spec_id=...) -> "
    "review_infra_dynamic_request -> show the plan and its cost to the user -> "
    "create_infra_dynamic(force_create=True, confirm=True). Afterwards: "
    "execute_command_infra, diagnose_infra, terminate_infra, release_resources.",
    "get_usage_guide(topic) holds the worked examples: provision, initscript, commands, "
    "gpu, recovery, cleanup, services, detail.",
    "Tools that create, change or delete take confirm. Pass confirm=True only after the "
    "user agreed; without it the call refuses and describes what it would do.",
    "Tools that change state also take session_id, which journals the call so that "
    "resume_session(session_id) recovers the work after a disconnect, and idempotency_key, "
    "with which a retry returns the first result instead of acting twice.",
    'The namespace defaults to "default".',
])

# mask_error_details hides internal exception text from clients; it is still logged here.
mcp = FastMCP("cb-tumblebug", instructions=SERVER_INSTRUCTIONS, mask_error_details=True)


# MCP_READ_ONLY=true serves only the tools that change nothing, so an exploratory agent is
# safe by construction rather than by instruction.
READ_ONLY_MODE = os.environ.get("MCP_READ_ONLY", "").strip().lower() in ("1", "true", "yes")


# Tools that mix reads and writes behind an `action` argument: name -> the read actions
READ_ACTION_TOOLS = {}


def _compact_schema(parameters):
    """Publish an optional argument as its plain type instead of "X or null, default null".

    The listing is paid for on every session and optional arguments are most of it. Input
    is still validated against the function signature, so an explicit null is accepted.
    """
    for prop in ((parameters or {}).get("properties") or {}).values():
        options = prop.get("anyOf")
        if not isinstance(options, list) or prop.get("default", "") is not None:
            continue
        kept = [o for o in options if o != {"type": "null"}]
        if len(kept) == len(options):
            continue
        del prop["anyOf"], prop["default"]
        if len(kept) == 1:
            prop.update(kept[0])
        else:
            prop["anyOf"] = kept


def tool(*d_args, redact=True, mutating=False, read_only=None, additive=False,
         read_actions=None, **d_kwargs):
    """Register an MCP tool that returns its payload exactly once.

    FastMCP renders a structured return value twice - escaped inside content[0].text and
    again as structuredContent - so every response costs the caller double. Measured on
    get_infra: a 237 KB payload arrived as a 478 KB response, the two halves identical.

    A ToolResult carrying only content is the one shape that emits a single representation;
    output_schema=None and a str return annotation both still duplicate. The wrapper
    serialises whatever the tool returns and re-declares the return type, since FastMCP
    decides how to render from that annotation.

    redact=False exempts a tool from secret stripping; only a tool whose entire purpose is
    to hand back a secret should use it.

    mutating=True adds two things a caller should not have to remember: the call is recorded
    in the session journal so work survives a lost connection, and an idempotency_key makes
    a retry return the first result instead of acting twice. Recording is automatic because
    leaving it to the model is why the previous attempt captured almost nothing.

    read_only defaults to "not mutating". Pass read_only=False on a tool that is not
    journaled as mutating but can still change something through one of its arguments.
    Read-only tools are announced as such (readOnlyHint), which is what lets a client
    run them without asking. additive=True marks a tool that only creates; the other
    tools that take confirm are announced as destructive.

    read_actions names the values of the tool's `action` argument that only read. Those
    calls are not journaled, and under MCP_READ_ONLY the tool stays published and refuses
    every other action.
    """
    is_read_only = (not mutating) if read_only is None else read_only
    read_actions = tuple(a.lower() for a in (read_actions or ()))

    def decorate(fn):
        base = inspect.signature(fn)

        @functools.wraps(fn)
        def wrapper(*args, **kwargs):
            started = time.monotonic()
            own = fn.__code__.co_varnames[:fn.__code__.co_argcount]
            session_id = (kwargs.get("session_id") if "session_id" in own
                          else kwargs.pop("session_id", None))
            idem_key = None
            if "idempotency_key" not in own:
                idem_key = kwargs.pop("idempotency_key", None)
            reading = bool(read_actions) and str(
                kwargs.get("action") or "").strip().lower() in read_actions
            dry_run = "dry_run" in base.parameters and bool(
                kwargs.get("dry_run", base.parameters["dry_run"].default))
            if READ_ONLY_MODE and not is_read_only and not reading:
                return ToolResult(content=[TextContent(type="text", text=json.dumps(
                    _as_structured_error({"error": {
                        "message": f"Refused: this server is read-only and {fn.__name__} "
                                   f"with this action changes state",
                        "code": "READ_ONLY"}})))], is_error=True)

            if idem_key:
                cached = _state_get(f"{STATE_PREFIX}/idempotency/{fn.__name__}/{idem_key}")
                if cached is not None:
                    cached["_idempotent_replay"] = True
                    return ToolResult(content=[TextContent(
                        type="text",
                        text=json.dumps(cached, ensure_ascii=False, default=str))])

            payload = fn(*args, **kwargs)
            if isinstance(payload, ToolResult):
                return payload
            # Redaction is applied here rather than per tool so a newly added tool cannot
            # leak by omission. get_ssh_private_key opts out deliberately.
            if redact:
                payload = _redact_secrets(payload)
            failure = _as_structured_error(payload)

            if mutating and not reading and not dry_run:
                code = failure["error"]["code"] if failure else None
                if idem_key and not failure:
                    _state_put(f"{STATE_PREFIX}/idempotency/{fn.__name__}/{idem_key}",
                               payload, ttl_seconds=IDEMPOTENCY_TTL_SECONDS)
                # A refusal changed nothing, so it is not part of what the session did.
                if session_id and code not in _NO_EFFECT_CODES:
                    _journal(session_id, {
                        "tool": fn.__name__,
                        "arguments": _journal_arguments(kwargs),
                        "outcome": code or "ok",
                        "at": datetime.now().isoformat(timespec="seconds"),
                    })
            if failure is not None:
                logger.info("tool=%s ms=%d code=%s outcome=error", fn.__name__,
                            int((time.monotonic() - started) * 1000),
                            failure.get("error", {}).get("code"))
                return ToolResult(
                    content=[TextContent(type="text",
                                         text=json.dumps(failure, ensure_ascii=False, default=str))],
                    is_error=True)
            text = payload if isinstance(payload, str) else json.dumps(
                payload, ensure_ascii=False, default=str)
            # One line per call: name, duration, response size, outcome. Without it there is
            # no way to see which tools are slow or expensive except by measuring by hand.
            logger.info("tool=%s ms=%d bytes=%d outcome=ok",
                        fn.__name__, int((time.monotonic() - started) * 1000), len(text))
            return ToolResult(content=[TextContent(type="text", text=text)])

        # inspect.signature stops unwrapping at an explicit __signature__, so this is what
        # FastMCP reads: the original parameters with the return type it needs.
        # Journal metadata, on the tools that are journaled. A tool that already declares
        # one of these keeps its own (resume_session takes a session_id as its subject).
        wanted = ["session_id", "idempotency_key"] if mutating else []
        extra = [inspect.Parameter(name, inspect.Parameter.KEYWORD_ONLY,
                                   default=None, annotation=Optional[str])
                 for name in wanted if name not in base.parameters]
        wrapper.__signature__ = base.replace(
            parameters=list(base.parameters.values()) + extra,
            return_annotation=ToolResult)
        # pydantic builds the input schema from __annotations__, not from __signature__,
        # so the added parameters have to appear in both.
        wrapper.__annotations__ = dict(getattr(fn, "__annotations__", {}))
        for param in extra:
            wrapper.__annotations__[param.name] = param.annotation
        wrapper.__annotations__["return"] = ToolResult
        if is_read_only:
            d_kwargs.setdefault("annotations", {"readOnlyHint": True})
        elif additive:
            d_kwargs.setdefault("annotations", {"destructiveHint": False})
        elif "confirm" in base.parameters:
            d_kwargs.setdefault("annotations", {"destructiveHint": True})
        if read_actions:
            READ_ACTION_TOOLS[fn.__name__] = read_actions
        if is_read_only or read_actions or not READ_ONLY_MODE:
            mcp.tool(*d_args, **d_kwargs)(wrapper)

        # Register the wrapper with FastMCP but leave the plain function in the module
        # namespace. Tools call each other internally, and those callers want the dict the
        # function returns - handing them a ToolResult breaks `"error" in result` and
        # `result.get(...)` at a distance, in code that never mentions MCP.
        return fn
    return decorate


# ============================================================================
# Response shaping
#
# Reads are tiered so the caller chooses how much it pays for. "summary" is the
# default because the full objects are dominated by data nobody asked for: a
# single node measured 84,978 B, 94% of it command history.
# ============================================================================

# ---------------------------------------------------------------------------
# Structured errors
#
# A backend failure used to come back as an ordinary result: isError stayed false
# and the body carried {"error": ...}. Anything checking the protocol flag - an
# agent deciding whether to retry, a test asserting on a call - concluded the call
# had worked. Failures are now flagged and carry a code the caller can branch on.
# ---------------------------------------------------------------------------

_ERROR_CODE_PATTERNS = (
    # A refused-pending-review is a workflow gate, not an upstream failure: the caller
    # should run the review, not retry or report a server problem.
    ("over budget", "BUDGET_EXCEEDED"),
    ("not delegated", "ZONE_NOT_DELEGATED"),
    ("was not confirmed", "NEEDS_CONFIRMATION"),
    ("requires prior validation", "REVIEW_REQUIRED"),
    ("review_step_skipped", "REVIEW_REQUIRED"),
    ("rate limit", "RATE_LIMITED"),
    ("too many requests", "RATE_LIMITED"),
    ("does not exist", "NOT_FOUND"),
    ("not found", "NOT_FOUND"),
    ("not valid namespace", "NOT_FOUND"),
    ("already exists", "ALREADY_EXISTS"),
    ("database exists", "ALREADY_EXISTS"),
    ("is not allowed for", "CONFLICT"),
    ("resourceinuse", "CONFLICT"),
    ("currently referenced", "CONFLICT"),
    ("still referenced", "CONFLICT"),
    ("dependencyviolation", "CONFLICT"),
    # A rejected argument is the caller's to fix; reporting it as an upstream failure
    # invites a retry of the same call.
    ("unknown kind", "INVALID_ARGUMENT"),
    ("unknown topic", "INVALID_ARGUMENT"),
    ("unknown action", "INVALID_ARGUMENT"),
    ("unsupported action", "INVALID_ARGUMENT"),
    ("invalid resource type", "INVALID_ARGUMENT"),
    (" is required", "INVALID_ARGUMENT"),
    ("expected one of", "INVALID_ARGUMENT"),
    ("not supported", "UNSUPPORTED"),
    ("timeout", "TIMEOUT"),
    ("timed out", "TIMEOUT"),
    ("connection error", "UPSTREAM_UNREACHABLE"),
    ("connection refused", "UPSTREAM_UNREACHABLE"),
    ("connection reset", "UPSTREAM_UNREACHABLE"),
    ("no route to host", "UPSTREAM_UNREACHABLE"),
    ("permission", "FORBIDDEN"),
    ("unauthorized", "FORBIDDEN"),
)

_ERROR_NEXT_STEPS = {
    "BUDGET_EXCEEDED": ["choose a cheaper spec or fewer nodes",
                        "request_budget_increase(reason=...) and retry with budget_ack"],
    "ZONE_NOT_DELEGATED": ["get_dns_hosted_zones() lists usable_domains",
                           "ask the user which of their domains to use"],
    "NEEDS_CONFIRMATION": ["ask the user to confirm, then repeat the call with confirm=True"],
    "REVIEW_REQUIRED": ["run review_infra_dynamic_request() with the same arguments",
                        "fix anything it reports, then retry with force_create=True"],
    "RATE_LIMITED": ["wait a few seconds and retry",
                     "batch reads instead of calling per resource"],
    "NOT_FOUND": ["list the parent resource to check the id"],
    "ALREADY_EXISTS": ["read the existing resource instead of creating it"],
    "INVALID_ARGUMENT": ["correct the argument named in the message and call again"],
    "READ_ONLY": ["only the reading tools and actions work on this endpoint"],
    "CONFLICT": ["the resource is busy or not in a state that allows this - read its state, "
                 "wait, and try again"],
    "TIMEOUT": ["retry", "the operation may still be running - check status before retrying"],
    "UPSTREAM_UNREACHABLE": ["check CB-Tumblebug is reachable", "retry"],
    "UNSUPPORTED": ["this provider or option does not support the request as given - the "
                    "message says what does"],
    "FORBIDDEN": ["check credentials"],
}


# Outcomes that are known to have changed nothing
_NO_EFFECT_CODES = ("NEEDS_CONFIRMATION", "REVIEW_REQUIRED", "INVALID_ARGUMENT", "NOT_FOUND",
                    "ALREADY_EXISTS", "BUDGET_EXCEEDED", "READ_ONLY", "ZONE_NOT_DELEGATED",
                    "RATE_LIMITED", "UNSUPPORTED", "FORBIDDEN")


def _journal_arguments(kwargs):
    """Call arguments as they may be stored: no secrets, no bulk."""
    out = {}
    for name, value in kwargs.items():
        if name == "confirm" or value is None:
            continue
        if name in ("credential", "content") \
                or any(hint in name.lower().replace("_", "") for hint in _SECRET_FIELD_HINTS):
            out[name] = _REDACTED
            continue
        value = _redact_secrets(value)
        text = value if isinstance(value, str) else json.dumps(value, default=str)
        out[name] = value if len(text) <= 300 else text[:300] + "..."
    return out


_STATUS_CODES = {401: "FORBIDDEN", 403: "FORBIDDEN", 404: "NOT_FOUND", 409: "CONFLICT",
                 422: "UNSUPPORTED", 429: "RATE_LIMITED", 503: "RATE_LIMITED", 504: "TIMEOUT"}


def _classify_error(message):
    low = str(message).lower()
    # How the server reports a missing resource record
    if re.fullmatch(r"cannot get (the )?\w+ \S+\.", low):
        return "NOT_FOUND"
    for needle, code in _ERROR_CODE_PATTERNS:
        if needle in low:
            return code
    return "UPSTREAM_ERROR"


def _as_structured_error(payload):
    """Return a structured error when the payload represents a failure, else None."""
    if not isinstance(payload, dict):
        return None
    if payload.get("ok") is False and isinstance(payload.get("error"), dict):
        return payload  # already structured
    raw = payload.get("error")
    if raw is None:
        return None
    if isinstance(raw, dict):
        message = raw.get("message") or json.dumps(raw, ensure_ascii=False, default=str)
        detail = {k: v for k, v in raw.items() if k != "message"}
    else:
        message = str(raw)
        detail = {}
    for k, v in payload.items():
        if k != "error":
            detail[k] = v
    # A code the tool set itself wins over one guessed from the wording, and the wording
    # wins over the HTTP status, which the server uses loosely.
    own = detail.pop("code", None)
    status = detail.pop("status", None)
    code = own if own in _ERROR_NEXT_STEPS else _classify_error(message)
    if code == "UPSTREAM_ERROR" and detail:
        code = _classify_error(json.dumps(detail, default=str))
    if code == "UPSTREAM_ERROR":
        code = _STATUS_CODES.get(status, code)
    how = detail.pop("how_to_proceed", None)
    err = {"code": code, "message": message}
    if detail:
        err["detail"] = detail
    nxt = [how] if how else _ERROR_NEXT_STEPS.get(code)
    if nxt:
        err["next"] = nxt
    return {"ok": False, "error": err}


DETAIL_LEVELS = ("minimal", "summary", "full")

# Substring match, lowercased: field names that must never reach the caller.
# Keys are handed out by name only; the value is fetched deliberately elsewhere.
_SECRET_FIELD_HINTS = ("privatekey", "secretkey", "clientsecret", "password",
                       "credentialvalue", "accesskey", "apikey", "token")
_REDACTED = "<redacted>"


def _redact_secrets(obj):
    """Strip secret-looking values at any depth. Applied to every response."""
    if isinstance(obj, dict):
        out = {}
        for k, v in obj.items():
            if isinstance(k, str) and any(h in k.lower() for h in _SECRET_FIELD_HINTS):
                out[k] = _REDACTED if v else v
            else:
                out[k] = _redact_secrets(v)
        return out
    if isinstance(obj, list):
        return [_redact_secrets(v) for v in obj]
    return obj


def _collapse_command_status(entries):
    """Command history as a pointer instead of a transcript."""
    if not isinstance(entries, list) or not entries:
        return {"count": 0}
    last = entries[-1]
    out = {"count": len(entries), "last_status": last.get("status"),
           "last_request_id": last.get("xRequestId")}
    running = sum(1 for e in entries if e.get("status") in ("Handling", "Queued"))
    if running:
        out["handling"] = running
    return {k: v for k, v in out.items() if v not in (None, "")}


_NODE_MINIMAL = ("id", "status")
# Addresses appear under both spellings: the full record says publicIP, the status view
# publicIp. uid is what manage_labels addresses a node by.
_NODE_SUMMARY = ("id", "uid", "name", "status", "publicIP", "privateIP", "publicIp", "privateIp",
                 "sshPort", "nodeUserName",
                 "specId", "imageId", "connectionName", "nodeGroupId", "vNetId", "subnetId",
                 "securityGroupIds", "sshKeyId", "region", "systemMessage", "createdTime")
# Kept at summary only when they say something: most nodes have no failure and no
# init script message.
_NODE_SUMMARY_IF_SET = ("initScriptStatus", "initScriptMessage", "failure")

# A 10,000-node Infra is several megabytes even at summary. Past this many nodes a read
# returns counts plus the nodes that need attention.
NODE_LIST_LIMIT = int(os.environ.get("MCP_NODE_LIST_LIMIT", "50"))


_EMPTY = (None, "", [], {})


def _shape_node(node, detail):
    if not isinstance(node, dict) or detail == "full":
        return node
    if detail == "minimal":
        return {k: node[k] for k in _NODE_MINIMAL if k in node}
    out = {k: node[k] for k in _NODE_SUMMARY if node.get(k) not in _EMPTY}
    if out.get("name") == out.get("id"):
        out.pop("name", None)
    if out.get("status") == "Running":
        out.pop("systemMessage", None)
    for k in _NODE_SUMMARY_IF_SET:
        if node.get(k):
            out[k] = node[k]
    label = {k: v for k, v in (node.get("label") or {}).items() if v != node.get("uid")}
    if label:
        out["label"] = label
    if node.get("commandStatus"):
        out["commandStatus"] = _collapse_command_status(node["commandStatus"])
    return out


def _cap_nodes(nodes, limit):
    """Pick the nodes worth listing from a large Infra, and count the rest.

    Nodes that are not Running come first: they are what a caller is looking for when
    something went wrong, and they are the ones a plain head of the list would hide.
    """
    by_status, by_group = {}, {}
    for n in nodes:
        status = n.get("status") or "Undefined"
        by_status[status] = by_status.get(status, 0) + 1
        group = n.get("nodeGroupId") or (n.get("id") or "").rsplit("-", 1)[0]
        by_group[group] = by_group.get(group, 0) + 1
    attention = [n for n in nodes if (n.get("status") or "") != "Running"]
    running = [n for n in nodes if (n.get("status") or "") == "Running"]
    shown = (attention + running)[:limit]
    largest = sorted(by_group.items(), key=lambda kv: -kv[1])[:30]
    return shown, {
        "total": len(nodes),
        "shown": len(shown),
        "byStatus": by_status,
        "nodeGroupCount": len(by_group),
        "largestNodeGroups": dict(largest),
        "note": ("node list capped; nodes that are not Running are listed first. "
                 "get_nodegroups(nodegroup_id=...) lists a group's node ids; detail='full' "
                 "returns every node"),
    }


_INFRA_DROP_AT_SUMMARY = ("newNodeList", "configureCloudAdaptiveNetwork", "systemLabel",
                          "resourceType", "targetStatus", "targetAction", "cluster",
                          "postScript")


def _shape_infra(infra, detail):
    if not isinstance(infra, dict) or detail == "full":
        return infra
    if detail == "minimal":
        out = {k: infra[k] for k in ("id", "status") if k in infra}
    else:
        out = {k: v for k, v in infra.items()
               if k not in _INFRA_DROP_AT_SUMMARY and v not in _EMPTY}
        if out.get("name") == out.get("id"):
            out.pop("name", None)
        # sys.* labels repeat the id, name and namespace
        label = {k: v for k, v in (infra.get("label") or {}).items() if not k.startswith("sys.")}
        out.pop("label", None)
        if label:
            out["label"] = label
        if isinstance(infra.get("postScriptResults"), list):
            out["postScriptResults"] = [
                {"phase": ph.get("phase"), "target": ph.get("target"), "status": ph.get("status"),
                 "failedNodes": [r.get("nodeId") for r in
                                 ((ph.get("results") or {}).get("results") or []) if r.get("error")]}
                for ph in infra["postScriptResults"]]
    if isinstance(infra.get("statusCount"), dict):
        out["statusCount"] = {k: v for k, v in infra["statusCount"].items() if v}
    if "node" in infra:
        nodes = infra.get("node") or []
        if NODE_LIST_LIMIT and len(nodes) > NODE_LIST_LIMIT:
            nodes, out["nodeListCapped"] = _cap_nodes(nodes, NODE_LIST_LIMIT)
        out["node"] = [_shape_node(n, detail) for n in nodes]
    return out


# Fields that repeat on every resource and say nothing about it.
_RESOURCE_BULK = ("connectionConfig", "keyValueList", "addtionalDetails")


def _slim_resource(obj):
    """Drop the per-resource boilerplate from a record returned by a write."""
    if not isinstance(obj, dict):
        return obj
    return {k: v for k, v in obj.items() if k not in _RESOURCE_BULK}


def _project(obj, fields):
    """Keep only the named top-level fields; applied after tiering."""
    if not fields:
        return obj
    keep = [f.strip() for f in fields.split(",") if f.strip()]
    if not keep:
        return obj
    if isinstance(obj, list):
        return [_project(o, fields) for o in obj]
    if isinstance(obj, dict):
        return {k: v for k, v in obj.items() if k in keep}
    return obj


_SUBNET_SUMMARY = ("id", "ipv4_CIDR", "zone", "status")


def _summarize_resource(item, summary_keys):
    """One resource at summary: the named fields that are set, subnets reduced to a line."""
    out = {k: item[k] for k in summary_keys if item.get(k) not in _EMPTY}
    if out.get("name") == out.get("id"):
        out.pop("name", None)
    # An image reports its state under another name than every other resource
    if "imageStatus" in out:
        out.setdefault("status", out.pop("imageStatus"))
    if isinstance(out.get("subnetInfoList"), list):
        out["subnetInfoList"] = [{k: sub[k] for k in _SUBNET_SUMMARY if sub.get(k)}
                                 for sub in out["subnetInfoList"] if isinstance(sub, dict)]
    return out


def _shape_resource_list(payload, list_key, detail, fields, summary_keys):
    """Tier a `{list_key: [...]}` resource listing."""
    if not isinstance(payload, dict) or list_key not in payload:
        return _redact_secrets(payload)
    items = payload.get(list_key) or []
    if detail == "full":
        shaped = items
    elif detail == "minimal":
        shaped = [{k: it.get(k) for k in ("id",) if k in it} for it in items if isinstance(it, dict)]
    else:
        shaped = [_summarize_resource(it, summary_keys) for it in items if isinstance(it, dict)]
    out = dict(payload)
    out[list_key] = _project(shaped, fields)
    return _redact_secrets(out)


def _tail(text, lines):
    """Last N lines of a captured stream, with a marker when anything was dropped."""
    if not isinstance(text, str) or not text:
        return text
    parts = text.splitlines()
    if len(parts) <= lines:
        return text
    return f"...[{len(parts) - lines} earlier lines omitted]...\n" + "\n".join(parts[-lines:])


def _truncation_note(rec, streams, kept_lines):
    """Record truncation as a field, not only inside the text.

    The in-text marker disappears the moment a caller greps the output, and absence then
    reads as "the string is not there" rather than "you were shown only the tail". A
    structured flag survives filtering.
    """
    total = 0
    for value in (streams or {}).values():
        if isinstance(value, str):
            total += len(value.splitlines())
    if total > kept_lines:
        rec["truncated"] = True
        rec["total_output_lines"] = total
        rec["kept_output_lines"] = kept_lines
    return rec


def _grep_stream(text, pattern, context):
    """Lines matching pattern, each with a little surrounding context.

    What an install log is actually asked for is one region of it - the credentials block a
    DevStack prints at the end, the line that failed. The tail cannot reach either when the
    log is megabytes long, and returning the whole thing costs more than the session.
    """
    if not isinstance(text, str) or not text:
        return text
    lines = text.splitlines()
    try:
        matcher = re.compile(pattern, re.IGNORECASE)
    except re.error:
        matcher = re.compile(re.escape(pattern), re.IGNORECASE)
    keep = set()
    hits = 0
    for i, line in enumerate(lines):
        if matcher.search(line):
            hits += 1
            keep.update(range(max(0, i - context), min(len(lines), i + context + 1)))
    if not hits:
        return f"[no line matched {pattern!r} in {len(lines)} lines]"
    out, last = [], None
    for i in sorted(keep):
        if last is not None and i != last + 1:
            out.append(f"... [{i - last - 1} lines skipped]")
        out.append(lines[i])
        last = i
    return "\n".join(out)


def _trim_command_record(rec, tail_lines, contains=None, context_lines=3):
    """A command record without the full transcript.

    Install scripts produce 100 KB records; the tail is what diagnoses a failure.
    """
    if not isinstance(rec, dict):
        return rec
    out = dict(rec)
    streams = {}

    def shrink(value):
        if contains:
            return _grep_stream(value, contains, context_lines)
        return _tail(value, tail_lines)

    for k in ("stdout", "stderr"):
        v = rec.get(k)
        if isinstance(v, dict):
            streams.update({f"{k}:{ik}": iv for ik, iv in v.items()})
            out[k] = {ik: shrink(iv) for ik, iv in v.items()}
        else:
            streams[k] = v
            out[k] = shrink(v)
    if not contains:
        _truncation_note(out, streams, tail_lines)
    for k in ("commandRequested", "commandExecuted"):
        v = rec.get(k)
        if isinstance(v, str) and len(v) > 500:
            out[k] = v[:500] + f"...[{len(v) - 500} chars omitted]"
    if out.get("commandExecuted") == out.get("commandRequested"):
        out.pop("commandExecuted", None)
    return {k: v for k, v in out.items() if v not in _EMPTY}


def _drop_command_output(rec):
    """Metadata only - the caller can fetch this record by index when it matters."""
    if not isinstance(rec, dict):
        return rec
    out = {k: v for k, v in rec.items() if k not in ("stdout", "stderr") and v not in _EMPTY}
    if out.get("commandExecuted") == out.get("commandRequested"):
        out.pop("commandExecuted", None)

    def clip(v):
        if isinstance(v, str) and len(v) > 200:
            return v[:200] + f"...[{len(v) - 200} chars omitted]"
        return v

    # Bodies are clipped too: a base64-encoded installer is a single 86 KB "command".
    for k in ("commandRequested", "commandExecuted", "command"):
        v = rec.get(k)
        if isinstance(v, list):
            out[k] = [clip(x) for x in v]
        elif v is not None:
            out[k] = clip(v)
    out["output"] = "omitted; list_node_command_status(index=...) or detail='full'"
    return out


def _require_confirmation(confirm, what, consequence):
    """Refuse a destructive call that was not explicitly confirmed.

    Returning a structured refusal rather than relying on the description: an instruction to
    "ask the user first" is advice a model may skip, while a required flag cannot be skipped.
    """
    if confirm:
        return None
    return {"error": {
        "message": f"Refused: {what} was not confirmed",
        "code": "NEEDS_CONFIRMATION",
        "consequence": consequence,
        "how_to_proceed": "ask the user, then repeat the call with confirm=True",
    }}


def _confirm_creation_plan(node_groups, name, confirm):
    """Refuse to build until a human has seen exactly what will be built.

    review_infra_dynamic_request answers "can this be created". It does not answer "did
    anyone agree to create it". An agent that reviews successfully will otherwise proceed
    straight to provisioning, and the first a person hears of the shape and price of their
    infrastructure is the bill.

    The refusal carries the plan itself, so the confirmation the user gives is informed:
    what machines, on which providers, in which regions, at what hourly cost.
    """
    if confirm:
        return None

    rows, hourly, unpriced = [], 0.0, 0
    for group in node_groups or []:
        if not isinstance(group, dict):
            continue
        spec_id = group.get("specId") or group.get("spec_id") or "?"
        try:
            size = max(1, int(group.get("nodeGroupSize") or group.get("node_group_size") or 1))
        except (TypeError, ValueError):
            size = 1
        parts = str(spec_id).split("+")
        provider = parts[0] if parts else "?"
        region = parts[1] if len(parts) > 1 else "?"
        cost = _spec_cost(spec_id)
        if cost is None:
            unpriced += size
        else:
            hourly += cost * size
        rows.append({
            "nodeGroup": group.get("name") or "-",
            "nodes": size,
            "provider": provider,
            "region": region,
            "spec": parts[-1] if parts else spec_id,
            "image": str(group.get("imageId") or group.get("image_id") or "-")[-40:],
            "usd_per_hour": "unpriced" if cost is None else round(cost * size, 4),
        })

    total_nodes = sum(r["nodes"] for r in rows)
    return {"error": {
        "message": (f"Refused: creating '{name}' was not confirmed. "
                    f"{total_nodes} node(s) across {len({r['provider'] for r in rows})} "
                    f"provider(s), about ${round(hourly, 4)}/hour"
                    + (f" plus {unpriced} node(s) with no published price" if unpriced else "")),
        "code": "NEEDS_CONFIRMATION",
        "plan": rows,
        "totals": {"nodes": total_nodes,
                   "usd_per_hour": round(hourly, 4),
                   "usd_per_month_estimate": round(hourly * 24 * 30, 2),
                   "nodes_without_a_published_price": unpriced},
        "cost_caveat": ("compute list price only - excludes egress, storage IOPS, NAT/LB "
                        "hourly and licensing"),
        "how_to_proceed": "show this plan to the user; if they agree, repeat the call with confirm=True",
    }}


def _validate_detail(detail):
    d = (detail or "summary").lower()
    return d if d in DETAIL_LEVELS else "summary"


# Shared HTTP session: connection pooling + retry on transient connection errors
_session = requests.Session()
_session.mount("http://", requests.adapters.HTTPAdapter(
    pool_maxsize=20,
    max_retries=requests.adapters.Retry(total=3, connect=2, read=0, backoff_factor=0.8,
                                        allowed_methods=["GET"],
                                        # 429: CB-Tumblebug rate-limits bursts of reads
                                        status_forcelist=[429, 502, 503, 504],
                                        # hand back the last answer instead of raising
                                        raise_on_status=False)))
_session.mount("https://", requests.adapters.HTTPAdapter(pool_maxsize=20))

def _request_headers(extra=None, credential_holder=None):
    """Headers every call carries: the MCP marker MapUI shows, and the credential holder."""
    headers = {"X-Request-Source": "mcp", **(extra or {})}
    holder = credential_holder or TUMBLEBUG_CREDENTIAL_HOLDER
    if holder:
        headers["x-credential-holder"] = holder
    return headers


# Helper function: API request wrapper
def api_request(method, endpoint, json_data=None, params=None, files=None, headers=None,
                timeout_override=None, credential_holder=None):
    """One call to CB-Tumblebug: the decoded body, or {"error": ...} when it failed."""
    method = method.upper()
    if method not in ("GET", "POST", "PUT", "DELETE"):
        return {"error": f"unsupported method {method}"}
    # Remote commands run long; everything else answers within ten minutes.
    timeout = timeout_override or ((60, 1200) if "/cmd/infra/" in endpoint else (60, 600))
    config = {"auth": (TUMBLEBUG_USERNAME, TUMBLEBUG_PASSWORD), "timeout": timeout,
              "headers": _request_headers(headers, credential_holder)}
    if params:
        config["params"] = params
    if json_data is not None and method != "GET":
        # With files the fields travel as multipart form data
        config["data" if files else "json"] = json_data
    if files:
        config["files"] = files

    try:
        response = _session.request(method, f"{TUMBLEBUG_API_BASE_URL}{endpoint}", **config)
    except requests.exceptions.Timeout:
        return {"error": {
            "message": f"timed out after {timeout[1] // 60} minutes - the operation may "
                       f"still be running on the server",
            "code": "TIMEOUT"}}
    except requests.exceptions.ConnectionError as exc:
        logger.error(f"connection error: {exc}")
        return {"error": {"message": "connection error - CB-Tumblebug is not reachable",
                          "code": "UPSTREAM_UNREACHABLE"}}
    except requests.RequestException as exc:
        logger.error(f"request failed: {exc}")
        return {"error": str(exc)}

    if response.status_code >= 400:
        try:
            body = response.json()
        except ValueError:
            body = None
        message = body.get("message") if isinstance(body, dict) else None
        logger.debug(f"{method} {endpoint} -> {response.status_code}")
        return {"error": {"message": message or response.text[:500]
                          or f"HTTP {response.status_code}",
                          "status": response.status_code}}
    if not response.text:
        return {"message": "Success (No content)"}
    try:
        return response.json()
    except ValueError:
        return {"error": "Invalid JSON response", "raw_response": response.text[:500]}

# Safely test the API only if in development mode
if os.environ.get("MCP_ENV") == "development":
    try:
        logger.info("Testing API connection...")
        test_result = api_request("GET", "/ns")
        logger.info("API test successful")
    except Exception as e:
        logger.error(f"API test failed: {str(e)}")

# Helper: the image's OS descriptor in one lowercase string
def _image_os_text(img: Dict) -> str:
    return " ".join(str(img.get(k, "")) for k in ("osType", "osDistribution", "osPlatform")).lower()


#####################################
# Runtime CSP registration
#####################################


# Tool: Register a CSP definition at runtime
@tool(mutating=True)
def register_csp_definition(
    provider_name: str,
    driver: str,
    regions: Dict,
    cloud_platform: str = "",
    description: str = ""
) -> Dict:
    """
    Register a new cloud provider at runtime, without restarting CB-Tumblebug.

    Use this for a cloud that came into existence after the server started - most often
    an OpenStack deployed onto a VM that CB-Tumblebug provisioned. The definition is
    persisted and survives a restart.

    This is step 1 of 3. Follow with register_csp_credential(), then
    fetch_assets_for_provider().

    Args:
        provider_name: Name for this provider, e.g. "openstack-site01". Becomes the
            provider in connection names, so keep it unique and descriptive.
        driver: CB-Spider driver file, e.g. "openstack-driver-v1.0.so"
        regions: Region map. Each region needs at least one zone:
            {"RegionOne": {"id": "RegionOne", "zone": ["nova"],
                           "location": {"display": "Europe (Milan)",
                                        "latitude": 45.4, "longitude": 9.1}}}
            The location is what places the cloud on the CB-MapUI map.
        cloud_platform: Base platform this is an instance of, e.g. "openstack".
            REQUIRED when provider_name is not itself a base platform - CB-Spider picks
            its driver from this. Omit only when registering a base platform.
        description: Human-readable description, e.g. "DevStack on 15.161.132.237"

    Returns:
        Registration result. On success, register_csp_credential() is the next call.
    """
    csp_detail = {
        "driver": driver,
        "regions": {},
    }
    if cloud_platform:
        csp_detail["cloudPlatform"] = cloud_platform
    if description:
        csp_detail["description"] = description

    # Accept both the cloudinfo.yaml spelling ("zone", "id") and the JSON model spelling
    # ("zones", "regionId"), because the snippet users paste comes from the YAML.
    for region_name, region in (regions or {}).items():
        zones = region.get("zone") or region.get("zones") or []
        if isinstance(zones, str):
            zones = [zones]
        entry = {
            "regionId": region.get("id") or region.get("regionId") or region_name,
            "zones": zones,
        }
        if region.get("description"):
            entry["description"] = region["description"]
        if region.get("location"):
            entry["location"] = region["location"]
        csp_detail["regions"][region_name] = entry

    return api_request("POST", f"/cloudInfo/{provider_name}", json_data=csp_detail)


# Tool: Remove a runtime-registered CSP definition
@tool(mutating=True)
def unregister_csp_definition(provider_name: str, confirm: bool = False) -> Dict:
    """
    Remove a CSP definition that was registered at runtime.

    Providers that came from assets/cloudinfo.yaml cannot be removed this way - they
    would return on the next restart. Edit the file for those.

    Args:
        provider_name: Provider to remove, e.g. "openstack-site01"
        confirm: Must be True to proceed - infra already running on this provider can no
            longer be managed

    Returns:
        The removal result; remainingConnections lists connection configs left behind.
    """
    guard = _require_confirmation(confirm, "unregistering a CSP definition", "infra already running on this provider becomes unmanageable from CB-Tumblebug")
    if guard:
        return guard
    result = api_request("DELETE", f"/cloudInfo/{provider_name}")

    # The definition goes; the connection configs built from its credential do not, because
    # the server exposes no way to delete one. Saying only "Removed" leaves the caller
    # believing in a clean slate that a later get_connections() contradicts.
    if isinstance(result, dict) and "error" not in result:
        leftovers = api_request("GET", "/connConfig")
        stale = [c.get("configName") for c in (leftovers or {}).get("connectionconfig") or []
                 if c.get("providerName") == provider_name] if isinstance(leftovers, dict) else []
        if stale:
            result["remainingConnections"] = stale
            result["_note"] = (
                f"{len(stale)} connection config(s) for {provider_name} remain - CB-Tumblebug "
                f"has no endpoint that deletes one. They point at an endpoint that may no "
                f"longer exist, and will be overwritten if you register this provider again "
                f"under the same name. Cached specs and images survive too.")
    return result


def _encrypt_credential_values(public_key_pem: str, credential: Dict) -> tuple:
    """
    Encrypt credential values the way CB-Tumblebug's /credential endpoint expects.

    Each value is AES-256-CBC encrypted under one per-call key, with a fresh IV
    prepended to its ciphertext; that AES key is then RSA-OAEP(SHA-256) wrapped with the
    server's public key. Mirrors init/init.py, which is the reference implementation.

    Returns (encrypted values by key, base64 RSA-wrapped AES key).
    """
    import base64
    import os as _os

    from cryptography.hazmat.primitives import hashes, padding as sym_padding, serialization
    from cryptography.hazmat.primitives.asymmetric import padding as asym_padding
    from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

    aes_key = _os.urandom(32)

    encrypted = {}
    for name, value in credential.items():
        iv = _os.urandom(16)
        padder = sym_padding.PKCS7(algorithms.AES.block_size).padder()
        padded = padder.update(str(value).encode()) + padder.finalize()
        encryptor = Cipher(algorithms.AES(aes_key), modes.CBC(iv)).encryptor()
        ciphertext = encryptor.update(padded) + encryptor.finalize()
        # The server splits the IV back off the front, so order matters here.
        encrypted[name] = base64.b64encode(iv + ciphertext).decode()

    wrapped = serialization.load_pem_public_key(public_key_pem.encode()).encrypt(
        aes_key,
        asym_padding.OAEP(
            mgf=asym_padding.MGF1(algorithm=hashes.SHA256()),
            algorithm=hashes.SHA256(),
            label=None,
        ),
    )
    return encrypted, base64.b64encode(wrapped).decode()


# Tool: Register credentials for a CSP
@tool(mutating=True)
def register_csp_credential(
    provider_name: str,
    credential: Dict,
    credential_holder: str = "admin"
) -> Dict:
    """
    Register credentials for a cloud provider and create its connection configs.

    This is step 2 of 3, after register_csp_definition(). On success CB-Tumblebug builds
    one connection per region and verifies each; the response reports which verified.

    Args:
        provider_name: Provider the credentials belong to, e.g. "openstack-site01"
        credential: Key/value credentials as the provider expects them. For OpenStack:
            {"IdentityEndpoint": "http://<ip>/identity/v3", "Username": "admin",
             "Password": "<pw>", "DomainName": "Default", "ProjectID": "<id>"}
            For AWS: {"ClientId": "...", "ClientSecret": "..."}
        credential_holder: Credential holder name (default: "admin")

    Returns:
        Registration result including per-connection verification status. When a
        connection reports verified=false, read verifiedMessage - it distinguishes an
        expired secret from a permission problem from an unreachable endpoint. For a
        self-hosted OpenStack the usual cause is the API port being closed in the
        security group of the VM hosting it.
    """
    if not credential:
        return {"error": "credential is required",
                "hint": "For OpenStack: IdentityEndpoint, Username, Password, DomainName, ProjectID"}

    # Credentials are never sent in the clear: the server hands out a short-lived RSA
    # public key, each value is AES-encrypted, and the AES key travels RSA-wrapped.
    key_response = api_request("GET", "/credential/publicKey")
    public_key = (key_response or {}).get("publicKey")
    token_id = (key_response or {}).get("publicKeyTokenId")
    if not public_key or not token_id:
        return {"error": "Could not obtain the credential encryption key",
                "response": key_response,
                "hint": "Is CB-Tumblebug reachable and initialized?"}

    try:
        encrypted_values, encrypted_aes_key = _encrypt_credential_values(public_key, credential)
    except ImportError as e:
        return {"error": "The 'cryptography' package is required to encrypt credentials",
                "detail": str(e),
                "hint": "pip install cryptography (already pinned in the MCP server image)"}
    except Exception as e:
        return {"error": f"Failed to encrypt credentials: {e}"}

    data = {
        "credentialHolder": credential_holder,
        "providerName": provider_name,
        "credentialKeyValueList": [{"key": k, "value": v} for k, v in encrypted_values.items()],
        "publicKeyTokenId": token_id,
        "encryptedClientAesKeyByPublicKey": encrypted_aes_key,
    }
    # Verification dials every region of the provider, so allow well past the default.
    result = api_request("POST", "/credential", json_data=data, timeout_override=(60, 900))

    # The server answers with every connection it knows about - 129 KB when a dozen
    # providers are registered, of which one screenful concerns the credential just
    # written. Keep the new provider's connections and count the rest.
    if isinstance(result, dict) and isinstance(result.get("allConnections"), dict):
        every = result["allConnections"].get("connectionconfig") or []
        mine = [c for c in every if c.get("providerName") == provider_name]
        result["connections"] = [
            {"configName": c.get("configName"), "region": (c.get("regionZoneInfo") or {}).get("assignedRegion"),
             "zone": (c.get("regionZoneInfo") or {}).get("assignedZone"),
             "verified": c.get("verified"), "verifiedMessage": c.get("verifiedMessage")}
            for c in mine]
        result["verifiedCount"] = sum(1 for c in mine if c.get("verified"))
        result.pop("allConnections", None)
        result["_note"] = (f"{len(mine)} connection(s) for {provider_name}; "
                           f"{len(every) - len(mine)} pre-existing connections omitted "
                           f"(get_connections() lists them)")
        if mine and not result["verifiedCount"]:
            result["nextStep"] = ("No connection verified. Read verifiedMessage, then check the "
                                  "endpoint is reachable from CB-Spider before retrying.")
    return result


# Tool: Fetch specs and images for specific providers
@tool(mutating=True)
def fetch_assets_for_provider(provider_names: List[str]) -> Dict:
    """
    Fetch VM specs and images for named providers only.

    This is step 3 of 3. Until it runs, a newly registered provider has an empty
    catalog: recommend_vm_spec() and search_images() return nothing for it, and
    create_infra_dynamic() has no spec or image to reference.

    Scoping to the new provider is what makes this usable interactively. Loading every
    registered provider walks all connections and takes 10-40 minutes; one small
    provider such as a DevStack finishes in seconds.

    Args:
        provider_names: Providers to fetch, e.g. ["openstack-site01"]. Use the names
            from get_connections() or the provider list. Must not be empty - passing
            nothing would fetch every provider.

    Returns:
        Fetch result. Afterwards, call recommend_vm_spec() / search_images() to see
        what the provider actually offers; a DevStack typically exposes only a handful.
    """
    if not provider_names:
        return {
            "error": "provider_names is required",
            "reason": "An empty list would fetch every registered provider, which takes 10-40 minutes.",
            "hint": "Pass the provider you just registered, e.g. ['openstack-site01'].",
        }

    # Only VERIFIED connections are fetched from. Without this check an unverified
    # provider returns an empty result in a couple of seconds and looks like a silent
    # success, when the real problem is that the cloud could not be reached.
    unverified, verified_count, probed = [], 0, False
    try:
        all_conns = api_request("GET", "/connConfig", params={"filterVerified": "false"})
        wanted = {p.lower() for p in provider_names}
        for conn in (all_conns or {}).get("connectionconfig", []):
            if str(conn.get("providerName", "")).lower() not in wanted:
                continue
            if conn.get("verified"):
                verified_count += 1
            else:
                unverified.append({
                    "configName": conn.get("configName"),
                    "verifiedMessage": conn.get("verifiedMessage") or "(no detail)",
                })
        probed = True
    except Exception:
        pass  # Advisory only; never block the fetch on this probe.

    if probed and verified_count == 0:
        if not unverified:
            # The provider is defined but has no connections at all, which means step 2
            # was skipped. Fetching would return nothing and look like a success.
            return {
                "error": "These providers have no connections yet",
                "reason": "A CSP definition alone does not create connections; credentials do. "
                          "Assets are fetched per connection, so there is nothing to fetch from.",
                "providers": provider_names,
                "hint": "Call register_csp_credential() for each provider first, confirm the "
                        "connection reports verified, then retry this call.",
            }
        return {
            "error": "None of the requested providers has a verified connection",
            "reason": "Assets are fetched only from verified connections, so this would return nothing.",
            "unverifiedConnections": unverified,
            "hint": "Read verifiedMessage above. For a self-hosted OpenStack the usual cause is "
                    "the API port (80) being closed in the security group of the VM hosting it - "
                    "fix that, re-run register_csp_credential(), then retry this call.",
        }

    params = {"providers": ",".join(provider_names)}
    # Even one provider can take a few minutes when it has many regions.
    result = api_request("GET", "/loadAssets", params=params, timeout_override=(60, 1800))

    summary = {"fetchedProviders": provider_names, "result": result}
    # /loadAssets answers {"output": null} on success, which reads as "nothing happened".
    # Count what actually landed so the caller knows whether step 3 achieved anything.
    counts = {}
    try:
        specs = recommend_vm_spec(filter_policies={"providerName": provider_names[0]}, limit=200)
        counts["specs"] = specs.get("count", 0)
        images = search_images(provider_name=provider_names[0], include_basic_image_only=False,
                               max_results=200)
        counts["images"] = images.get("imageCount", 0)
    except Exception as exc:
        counts["note"] = f"could not count the catalog: {exc}"
    summary["fetched"] = counts
    if counts and not any(v for k, v in counts.items() if k != "note"):
        summary["warning"] = ("The provider's catalog is still empty. Its connection may be "
                              "verified but returning no flavors or images.")
    if unverified:
        summary["warning"] = "Some connections of these providers are unverified and were skipped"
        summary["unverifiedConnections"] = unverified
    summary["nextStep"] = ("Call recommend_vm_spec() and search_images() for this provider to see "
                           "what it offers before create_infra_dynamic().")
    return summary


#####################################
# Connection Management
#####################################

# Tool: Cloud connections
@tool()
def get_connections(name: Optional[str] = None, detail: str = "summary",
                    provider: Optional[str] = None, fields: Optional[str] = None,
                    filter_verified: bool = True, filter_region_representative: bool = True,
                    credential_holder: Optional[str] = None) -> Dict:
    """
    List the registered cloud connections, or fetch one by name.

    A connection is one provider region reached with one credential; its name is what
    connectionName means elsewhere.

    Args:
        name: Return just this connection in full, e.g. "aws-ap-northeast-2"
        detail: "summary" (default: names grouped by provider) | "minimal" (a flat list
            of names) | "full"
        provider: Only connections of this provider, e.g. "aws"
        fields: Comma-separated allowlist of fields to keep
        filter_verified: Only connections that passed verification (default True)
        filter_region_representative: One connection per region (default True)
        credential_holder: Only connections of this credential holder

    Returns:
        connections: provider -> connection names, and count. With detail "minimal" or
        "full": connectionconfig.
    """
    if name:
        return api_request("GET", f"/connConfig/{name}")
    # Both flags are sent explicitly: the server defaults filterVerified to true when omitted
    params = {
        "filterVerified": "true" if filter_verified else "false",
        "filterRegionRepresentative": "true" if filter_region_representative else "false",
    }
    if credential_holder:
        params["filterCredentialHolder"] = credential_holder
    result = api_request("GET", "/connConfig", params=params)
    if not isinstance(result, dict) or "connectionconfig" not in result:
        return result
    items = result.get("connectionconfig") or []
    if provider:
        want = provider.lower()
        items = [c for c in items if str(c.get("providerName", "")).lower() == want]

    detail = _validate_detail(detail)
    if detail == "full":
        shaped = items
    elif detail == "minimal":
        shaped = [c.get("configName") for c in items]
    else:
        # One name per connection, grouped by provider: the name already carries the region.
        grouped: Dict[str, List[str]] = {}
        for c in items:
            grouped.setdefault(str(c.get("providerName")), []).append(c.get("configName"))
        return {"connections": grouped, "count": len(items)}
    return {"connectionconfig": _project(shaped, fields), "count": len(shaped)}


#####################################
# Resource Management
#####################################


# Helper: normalize firewall rule dicts to the API's field names
def _normalize_node_groups(node_groups: List[Dict]) -> List[Dict]:
    """
    Coerce a node-group list into the shapes model.CreateNodeGroupDynamicReq accepts.

    The server unmarshals rootDiskSize and nodeGroupSize as integers and rejects the
    request outright when they arrive as strings ("cannot unmarshal string into Go struct
    field ... of type int"). Callers - LLMs especially - naturally quote them, so convert
    here rather than surfacing a 400 they cannot act on.
    """
    INT_FIELDS = ("rootDiskSize", "nodeGroupSize")
    normalized = []
    for group in (node_groups or []):
        g = dict(group)
        if "subGroupSize" in g and "nodeGroupSize" not in g:
            g["nodeGroupSize"] = g.pop("subGroupSize")
        for field in INT_FIELDS:
            if field in g and g[field] not in (None, ""):
                try:
                    g[field] = int(str(g[field]).strip())
                except (TypeError, ValueError):
                    pass  # leave it; the server will report a clearer error than we can
        normalized.append(g)
    return normalized


def _normalize_firewall_rules(rules: List[Dict]) -> List[Dict]:
    """Map any reasonable spelling of a rule onto the four keys the server reads.

    A rule whose ports land in no recognised key used to reach the CSP as port -1, which
    AWS rejects with "TCP/UDP (from) port (-1) out of range" - a message that says nothing
    about the key being wrong. Unrecognised port keys are now refused here instead.
    """
    aliases = {"ports": "Ports", "port": "Ports", "protocol": "Protocol",
               "ipprotocol": "Protocol", "direction": "Direction", "cidr": "CIDR",
               "source": "CIDR", "sourcecidr": "CIDR"}
    normalized = []
    for rule in rules:
        r = {}
        low = {k.lower(): v for k, v in rule.items()}
        for k, v in rule.items():
            r[aliases.get(k.lower(), k)] = v
        # fromPort/toPort is how CB-Tumblebug reports rules back, so it is the form an
        # LLM most often echoes; fold it into the range spelling the write path expects.
        if "Ports" not in r and ("fromport" in low or "toport" in low):
            start = str(low.get("fromport", low.get("toport")))
            end = str(low.get("toport", low.get("fromport")))
            r["Ports"] = start if start == end else f"{start}-{end}"
            for key in list(r):
                if key.lower() in ("fromport", "toport"):
                    del r[key]
        r.setdefault("Direction", "inbound")
        r.setdefault("CIDR", "0.0.0.0/0")
        protocol = str(r.get("Protocol", "")).upper()
        if "Ports" not in r and protocol not in ("ALL", "ICMP", ""):
            raise ValueError(
                f"rule {rule!r} names no port. Use Ports (\"80\", \"80,443\", \"8000-9000\") "
                f"or fromPort/toPort; only Protocol ALL and ICMP may omit it.")
        if "Ports" in r:
            r["Ports"] = str(r["Ports"])
        normalized.append(r)
    return normalized


def _slim_sg_update(result):
    """A rule change answers with the whole SecurityGroup record; keep the rule set."""
    if not isinstance(result, dict) or "error" in result:
        return result
    result = {k: v for k, v in result.items() if k != "previous"}
    record = result.get("updated")
    if isinstance(record, dict):
        result["updated"] = {k: record[k] for k in ("id", "connectionName", "firewallRules")
                             if k in record}
    return result

def _add_firewall_rules(ns_id, sg_id, rules):
    """Add the rules the SecurityGroup lacks; the server refuses a set holding a duplicate."""
    path = f"/ns/{ns_id}/resources/securityGroup/{sg_id}"
    current = api_request("GET", path)
    if not isinstance(current, dict) or "error" in current:
        return current
    have = {(str(r.get("Port", "")), str(r.get("Protocol", "")).upper(),
             str(r.get("Direction", "")).lower(), r.get("CIDR"))
            for r in current.get("firewallRules") or [] if isinstance(r, dict)}
    missing, present = [], []
    for rule in rules:
        for port in [x.strip() for x in str(rule.get("Ports", "")).split(",")]:
            key = (port, str(rule.get("Protocol", "")).upper(),
                   str(rule.get("Direction", "")).lower(), rule.get("CIDR"))
            one = {**rule, "Ports": port} if "Ports" in rule else dict(rule)
            (present if key in have else missing).append(one)
    if not missing:
        return {"id": sg_id, "message": "every rule is already present",
                "updated": {k: current[k] for k in ("id", "connectionName", "firewallRules")
                            if k in current}}
    result = _slim_sg_update(
        api_request("POST", f"{path}/rules", json_data={"firewallRules": missing}))
    if present and isinstance(result, dict) and "error" not in result:
        result["alreadyPresent"] = [f"{r.get('Protocol')}/{r.get('Ports', '')}" for r in present]
    return result


# Tool: Add SecurityGroup firewall rules
@tool(mutating=True)
def add_security_group_rules(ns_id: str, sg_id: str, rules: List[Dict]) -> Dict:
    """
    Add firewall rules to a SecurityGroup.

    Common use case: Infra-created SecurityGroups open SSH(22) only — open service
    ports (e.g., 80/443) after installing applications like web servers.

    Args:
        ns_id: Namespace ID
        sg_id: SecurityGroup ID (e.g., "<infraId>-<nodeGroupId>"; see get_resources(kind='securityGroup'))
        rules: List of rules. Each rule:
            - Ports: Port(s) as string — single, list, or ranges (e.g., "80", "80,443", "8000-9000")
            - Protocol: "TCP" | "UDP" | "ICMP" | "ALL" (required)
            - Direction: "inbound" | "outbound" (default: "inbound")
            - CIDR: Allowed IP range (default: "0.0.0.0/0")
            Example: [{"Ports": "80,443", "Protocol": "TCP"}]

    Returns:
        The SecurityGroup's resulting rule set. Rules it already had are skipped and
        listed in alreadyPresent.
    """
    try:
        return _add_firewall_rules(ns_id, sg_id, _normalize_firewall_rules(rules))
    except ValueError as exc:
        return {"error": str(exc)}


# Tool: Remove SecurityGroup firewall rules
@tool(mutating=True)
def remove_security_group_rules(ns_id: str, sg_id: str, rules: List[Dict], confirm: bool = False) -> Dict:
    """
    Remove firewall rules from a SecurityGroup (rules must match existing ones).

    Args:
        ns_id: Namespace ID
        sg_id: SecurityGroup ID
        rules: List of rules to remove — same format as add_security_group_rules()
        confirm: Must be True to proceed - removing a rule can cut off SSH or
            application traffic to running nodes

    Returns:
        Update result with the SecurityGroup's resulting rule set
    """
    guard = _require_confirmation(confirm, "removing security group rules", "may cut off SSH or application traffic to running nodes")
    if guard:
        return guard
    try:
        data = {"firewallRules": _normalize_firewall_rules(rules)}
    except ValueError as exc:
        return {"error": str(exc)}
    return _slim_sg_update(
        api_request("DELETE", f"/ns/{ns_id}/resources/securityGroup/{sg_id}/rules", json_data=data))


# Tool: Provisioning risk, in one place
@tool()
def get_provisioning_risk(spec_id: str, csp_image_name: Optional[str] = None,
                          kind: Optional[str] = None) -> Dict:
    """
    Historical provisioning risk for a spec, optionally paired with an image.

    Three related questions used to be three separate tools; `kind` selects between them.

    Args:
        spec_id: Spec ID, e.g. "aws+ap-northeast-2+t2.small"
        csp_image_name: CSP image identifier. Required for "summary" and "detailed".
        kind: "summary"  - risk level for this spec+image pair (default with an image)
              "detailed" - per-factor breakdown and recommendations
              "history"  - raw success/failure log for the spec (default without an image)

    Returns:
        The requested risk view, or an error naming what is missing.
    """
    kind = (kind or ("summary" if csp_image_name else "history")).strip().lower()
    if kind not in ("summary", "detailed", "history"):
        return {"error": f"unknown kind {kind!r}; expected summary, detailed or history"}
    if kind in ("summary", "detailed") and not csp_image_name:
        return {"error": f"csp_image_name is required for kind={kind!r}"}

    if kind == "history":
        result = api_request("GET", f"/provisioning/log/{spec_id}")
        # 204 No Content means nothing has been recorded for this spec yet
        if isinstance(result, dict) and result.get("message") == "Success (No content)":
            return {"message": "No provisioning history recorded for this spec", "specId": spec_id}
        return result
    if kind == "summary":
        return api_request("GET", f"/provisioning/risk/{spec_id}",
                           params={"cspImageName": csp_image_name})
    return api_request("GET", "/provisioning/risk/detailed",
                       params={"specId": spec_id, "cspImageName": csp_image_name})


# ---------------------------------------------------------------------------
# Infra templates - a saved configuration is the fastest way to stand up a demo
# ---------------------------------------------------------------------------

@tool()
def list_infra_templates(ns_id: str = "system", template_id: Optional[str] = None,
                         detail: str = "summary") -> Dict:
    """
    List saved Infra templates, or fetch one by id.

    A template is a stored node_groups configuration, so a known-good demo setup can be
    replayed without rebuilding the spec and image selection each time.

    Args:
        ns_id: Namespace ID. Defaults to "system", where init loads the shipped templates.
        template_id: Return just this template, with its node groups
        detail: For the list: "summary" (default) gives one line per template - node
            groups, node count, providers; "full" returns every template's node groups.

    Returns:
        The template list, or the single template.
    """
    if template_id:
        return api_request("GET", f"/ns/{ns_id}/template/infra/{template_id}")
    result = api_request("GET", f"/ns/{ns_id}/template/infra")
    templates = result.get("templates") if isinstance(result, dict) else None
    if _validate_detail(detail) == "full" or not isinstance(templates, list):
        return result
    rows = []
    for t in templates:
        req = t.get("infraDynamicReq") or {}
        groups = req.get("nodeGroups") or []
        nodes = 0
        for g in groups:
            try:
                nodes += max(1, int(g.get("nodeGroupSize") or 1))
            except (TypeError, ValueError):
                nodes += 1
        rows.append({
            "id": t.get("id"),
            "description": t.get("description"),
            "nodeGroups": len(groups),
            "nodes": nodes,
            "providers": sorted({str(g.get("specId") or "").split("+")[0] for g in groups} - {""}),
            "hasPostScript": bool((req.get("postScript") or {}).get("phases")),
            "hasInitScript": bool(req.get("initScript")
                                  or any(g.get("initScript") for g in groups)),
        })
    return {"templates": rows, "count": len(rows),
            "note": "node groups: list_infra_templates(template_id=...)"}


def _read_infra_template(ns_id, template_id):
    """A template by id; the server looks in the namespace first, then in "system"."""
    found = api_request("GET", f"/ns/{ns_id}/template/infra/{template_id}")
    if isinstance(found, dict) and "error" in found and ns_id != "system":
        shipped = api_request("GET", f"/ns/system/template/infra/{template_id}")
        if isinstance(shipped, dict) and "error" not in shipped:
            return shipped
    return found


@tool(mutating=True, additive=True)
def provision_infra_from_template(ns_id: str, template_id: str, name: str,
                                  description: Optional[str] = None,
                                  budget_ack: Optional[str] = None,
                                  confirm: bool = False) -> Dict:
    """
    Create an Infra from a saved template.

    The template supplies the node groups and scripts; only the new Infra's name is needed.
    Without confirm the call refuses and returns the template's machines and hourly cost.

    Args:
        ns_id: Namespace for the new Infra. A template of that namespace is used first,
            then one shipped in "system".
        template_id: Template to instantiate (see list_infra_templates)
        name: Name for the new Infra
        description: Optional description
        budget_ack: Approval id from request_budget_increase(), to pass a spend limit once
        confirm: Set True once a person has seen the plan and agreed to it

    Returns:
        The created Infra: status, statusCount and its nodes with their addresses.
    """
    template = _read_infra_template(ns_id, template_id)
    if not isinstance(template, dict) or "error" in template:
        return template
    groups = (template.get("infraDynamicReq") or {}).get("nodeGroups") or []
    unconfirmed = _confirm_creation_plan(groups, name, confirm)
    if unconfirmed:
        return unconfirmed
    over_budget = _budget_refusal(groups, budget_ack)
    if over_budget:
        return over_budget
    body: Dict[str, Any] = {"name": name}
    if description:
        body["description"] = description
    result = api_request("POST", f"/ns/{ns_id}/infra/template/{template_id}", json_data=body,
                         timeout_override=(60, 3600))
    if not isinstance(result, dict) or "error" in result:
        return result
    _record_spend(groups)
    result = _shape_infra(result, "summary")
    if result.get("creationErrors") or (result.get("statusCount") or {}).get("countFailed"):
        result["next"] = ("some nodes failed: diagnose_infra() "
                          "explains why and what can be retried")
    return result


@tool(mutating=True)
def manage_infra_template(action: str, ns_id: str = "system",
                          template_id: Optional[str] = None,
                          template: Optional[Dict] = None,
                          infra_id: Optional[str] = None,
                          confirm: bool = False) -> Dict:
    """
    Create, update or delete an Infra template, or save a running Infra as one.

    Args:
        action: "create" | "update" | "delete" | "save_from_infra"
        ns_id: Namespace ID. Defaults to "system", where init loads the shipped templates.
            For "save_from_infra" it is the namespace of the Infra, and of the new template.
        template_id: Required for update and delete; the new template's name for
            "save_from_infra" (default "<infra_id>-template")
        template: Body for create and update: {"name": ..., "description": ...,
            "infraDynamicReq": {"name": ..., "nodeGroups": [...]}}
        infra_id: Infra to copy the configuration from, for "save_from_infra"
        confirm: Must be True for action="delete". Ask the user first - a shipped template
            is not recreated until the next init.

    Returns:
        The stored template, or the deletion result.
    """
    action = (action or "").strip().lower()
    if action == "create":
        if not template:
            return {"error": "template is required for action='create'"}
        return api_request("POST", f"/ns/{ns_id}/template/infra", json_data=template)
    if action == "update":
        if not (template_id and template):
            return {"error": "template_id and template is required for action='update'"}
        return api_request("PUT", f"/ns/{ns_id}/template/infra/{template_id}", json_data=template)
    if action == "save_from_infra":
        if not infra_id:
            return {"error": "infra_id is required for action='save_from_infra'"}
        config = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/configCopy")
        if not isinstance(config, dict) or "error" in config:
            return config
        return api_request("POST", f"/ns/{ns_id}/template/infra", json_data={
            "name": template_id or f"{infra_id}-template",
            "description": f"Saved from Infra {infra_id}",
            "infraDynamicReq": config})
    if action == "delete":
        if not template_id:
            return {"error": "template_id is required for action='delete'"}
        guard = _require_confirmation(
            confirm, f"deleting template {template_id!r}",
            "a shipped template is not recreated until the next init")
        if guard:
            return guard
        return api_request("DELETE", f"/ns/{ns_id}/template/infra/{template_id}")
    return {"error": f"unknown action {action!r}; expected one of create, update, delete, "
                     f"save_from_infra"}


# ---------------------------------------------------------------------------
# Global DNS - what makes a demo reachable by name instead of by IP
# ---------------------------------------------------------------------------

@tool()
def get_dns_hosted_zones(check_delegation: bool = True) -> Dict:
    """
    List the DNS hosted zones, and say which of them the public internet can actually reach.

    A zone existing in the account is not the same as the domain pointing at it. Writing a
    record into an undelegated zone succeeds, reads back correctly, and resolves for nobody.
    Each zone is therefore probed with a public NS lookup.

    Args:
        check_delegation: Probe each zone (default). Set False to skip the lookups.

    Returns:
        Zones with `delegated` true/false/null, and `usable_domains` - the ones worth using.
        Ask the user which domain to use rather than picking one; a DNS record is public and
        the choice is theirs.
    """
    result = api_request("GET", "/resources/globalDns/hostedZone")
    if not isinstance(result, dict) or not check_delegation:
        return result
    zones = result.get("hostedZones") or result.get("hostedZone") or []
    usable, unusable = [], []
    for zone in zones:
        if not isinstance(zone, dict):
            continue
        name = str(zone.get("name") or "").rstrip(".")
        if not name or name == "local":
            continue
        delegated = _zone_is_delegated(name)
        zone["delegated"] = delegated
        if delegated:
            usable.append(name)
        elif delegated is False:
            zone["warning"] = ("the domain is not delegated to this zone - records written "
                               "here resolve for nobody")
            unusable.append(name)
    result["usable_domains"] = usable
    if unusable:
        result["unusable_domains"] = unusable
    result["_choose"] = ("ask the user which domain to use before writing a record - it is "
                         "public and it is their domain")
    return result


# ---------------------------------------------------------------------------
# Command tasks - a long bootstrap needs to be observable and interruptible
# ---------------------------------------------------------------------------

@tool(read_only=False, read_actions=("list", "get"))
def manage_command_task(action: str, ns_id: str, infra_id: str,
                        task_id: Optional[str] = None, detail: str = "summary") -> Dict:
    """
    List, inspect or cancel remote-command tasks on an Infra.

    A long bootstrap started with post_script_async or a long timeout keeps running on the
    server; this is how to see what is in flight and stop it.

    Args:
        action: "list" | "get" | "cancel"
        ns_id: Namespace ID
        infra_id: Infra ID
        task_id: Required for "get" and "cancel"; taken from the task list
        detail: For "list": "summary" (default) shows the running and queued tasks and
            counts the rest; "full" lists every task

    Returns:
        The tasks in flight with statusCount, one task, or the cancellation result.
    """
    action = (action or "").strip().lower()
    if action == "list":
        result = api_request("GET", f"/ns/{ns_id}/cmd/infra/{infra_id}/task")
        if _validate_detail(detail) == "full" or not isinstance(result, dict):
            return result
        tasks = result.get("tasks")
        if not isinstance(tasks, list):
            return result
        # A listing answers "what is running"; finished tasks are counted, not listed.
        counts: Dict[str, int] = {}
        for task in tasks:
            state = task.get("status") or "Unknown"
            counts[state] = counts.get(state, 0) + 1
        active = [t for t in tasks if t.get("status") in ("Handling", "Queued")]
        rows = [{k: t[k] for k in ("taskId", "nodeId", "status", "startedAt", "elapsedSeconds",
                                   "command") if t.get(k) not in _EMPTY}
                for t in (_drop_command_output(t) for t in active[:NODE_LIST_LIMIT])]
        return {"tasks": rows, "statusCount": counts,
                "note": "running and queued tasks only; detail='full' lists every task"}
    if not task_id:
        return {"error": f"task_id is required for action={action!r}"}
    if action == "get":
        return api_request("GET", f"/ns/{ns_id}/cmd/infra/{infra_id}/task/{task_id}")
    if action == "cancel":
        return api_request("POST", f"/ns/{ns_id}/cmd/infra/{infra_id}/task/{task_id}/cancel")
    return {"error": f"unknown action {action!r}; expected list, get or cancel"}


# ---------------------------------------------------------------------------
# File download - reading a file a script wrote is often the only way to recover
# output that a dropped SSH channel lost
# ---------------------------------------------------------------------------

@tool()
def download_file_from_node(ns_id: str, infra_id: str, node_id: str, source_path: str,
                            max_bytes: int = 65536) -> Dict:
    """
    Read a file from a node, through the bastion.

    Returns the file's text inline, so it is for configuration and output files rather than
    archives. Content is truncated at max_bytes and binary files are reported rather than
    dumped. The server refuses files over 200 MB.

    This is the reliable way to recover output a command produced but did not return - for
    example a setup script that saved its registration details to a file before the SSH
    channel dropped.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Node ID
        source_path: Absolute path on the node, e.g. "/opt/stack/cb-registration-info.txt"
        max_bytes: Maximum number of bytes to return inline (default 65536)

    Returns:
        fileName, size, truncated, and either content or a note that the file is binary.
    """
    url = f"{TUMBLEBUG_API_BASE_URL}/ns/{ns_id}/downloadFile/infra/{infra_id}/node/{node_id}"
    try:
        resp = _session.post(url, json={"sourcePath": source_path},
                             headers=_request_headers(),
                             auth=(TUMBLEBUG_USERNAME, TUMBLEBUG_PASSWORD), timeout=(60, 600))
    except Exception as e:
        return {"error": f"download failed: {e}"}
    if resp.status_code >= 400:
        try:
            message = resp.json().get("message")
        except Exception:
            message = None
        return {"error": {"message": message or resp.text[:300] or f"HTTP {resp.status_code}",
                          "status": resp.status_code}}

    raw = resp.content
    disposition = resp.headers.get("Content-Disposition", "")
    match = re.search(r'filename="?([^";]+)"?', disposition)
    name = match.group(1) if match else os.path.basename(source_path)
    out: Dict[str, Any] = {"fileName": name, "size": len(raw),
                           "truncated": len(raw) > max_bytes}
    body = raw[:max_bytes]
    try:
        out["content"] = body.decode("utf-8")
    except UnicodeDecodeError:
        out["content"] = None
        out["note"] = "binary file - not returned inline; fetch it from MapUI instead"
    return out


# ---------------------------------------------------------------------------
# Shared state
#
# The MCP server runs stateless_http=True, which promises any replica can serve any
# request. Session data used to live in a function attribute, so the promise was false: it
# leaked between callers (one global list, no key), vanished on restart, and only appeared
# to work because replicas=1 was quietly holding it together.
#
# State therefore lives in etcd, keyed explicitly. Reached over etcd's HTTP v3 gateway so
# no gRPC client is needed - `requests` is already a dependency.
# ---------------------------------------------------------------------------

ETCD_ENDPOINT = os.environ.get("ETCD_ENDPOINT", "http://cb-tumblebug-etcd:2379")
STATE_PREFIX = "/mcp"
JOURNAL_LIMIT = 50          # entries kept per session; oldest are dropped
IDEMPOTENCY_TTL_SECONDS = 24 * 3600


def _b64(text):
    return base64.b64encode(str(text).encode()).decode()


def _unb64(text):
    return base64.b64decode(text).decode() if text else ""


def _etcd(path, body, timeout=5):
    """One etcd call. State is best-effort: a store outage must not fail the tool itself."""
    try:
        resp = _session.post(f"{ETCD_ENDPOINT}/v3/{path}", json=body, timeout=timeout)
        if resp.status_code >= 400:
            logger.warning(f"etcd {path} -> {resp.status_code}: {resp.text[:160]}")
            return None
        return resp.json()
    except Exception as e:
        logger.warning(f"etcd {path} unavailable: {e}")
        return None


def _state_put(key, value, ttl_seconds=None):
    body = {"key": _b64(key), "value": _b64(json.dumps(value, ensure_ascii=False, default=str))}
    if ttl_seconds:
        lease = _etcd("lease/grant", {"TTL": ttl_seconds})
        if lease and lease.get("ID"):
            body["lease"] = lease["ID"]
    return _etcd("kv/put", body) is not None


def _state_get(key):
    got = _etcd("kv/range", {"key": _b64(key)})
    if not got or not got.get("kvs"):
        return None
    try:
        return json.loads(_unb64(got["kvs"][0].get("value")))
    except Exception:
        return None


def _state_range(prefix, limit=0):
    """Every key under a prefix, in key order."""
    end = prefix[:-1] + chr(ord(prefix[-1]) + 1)
    body = {"key": _b64(prefix), "range_end": _b64(end)}
    if limit:
        body["limit"] = str(limit)
    got = _etcd("kv/range", body)
    out = []
    for kv in (got or {}).get("kvs") or []:
        try:
            out.append((_unb64(kv.get("key")), json.loads(_unb64(kv.get("value")))))
        except Exception:
            continue
    return out


def _state_delete_prefix(prefix):
    end = prefix[:-1] + chr(ord(prefix[-1]) + 1)
    return _etcd("kv/deleterange", {"key": _b64(prefix), "range_end": _b64(end)}) is not None


def _session_key(session_id, suffix):
    safe = re.sub(r"[^A-Za-z0-9_.:-]", "_", str(session_id))[:120] or "unkeyed"
    return f"{STATE_PREFIX}/session/{safe}/{suffix}"


def _journal(session_id, entry):
    """Append to a session's journal, capped so a long run cannot grow without bound."""
    if not session_id:
        return
    stamp = datetime.now().strftime("%Y%m%dT%H%M%S%f")
    _state_put(_session_key(session_id, f"journal/{stamp}"), entry)
    existing = _state_range(_session_key(session_id, "journal/"))
    for key, _ in existing[:-JOURNAL_LIMIT] if len(existing) > JOURNAL_LIMIT else []:
        _etcd("kv/deleterange", {"key": _b64(key)})


# ---------------------------------------------------------------------------
# Spend limits
#
# Enforced here rather than in CB-Tumblebug because the subject being limited is the LLM
# reaching the platform through MCP. People using MapUI or the REST API are deliberately
# unaffected.
#
# Unpriced specs are reported, never refused: every spec of a self-hosted OpenStack has
# costPerHour = -1, so refusing on price would block the primary demo. A guardrail that
# breaks the main use case is worse than no guardrail.
#
# costPerHour is a compute list price. It excludes egress, storage IOPS, NAT/LB hourly and
# licensing, so this bounds runaway agents - it is not a billing system.
# ---------------------------------------------------------------------------

BUDGET_POLICY_KEY = f"{STATE_PREFIX}/policy/budget"
BUDGET_REQUEST_PREFIX = f"{STATE_PREFIX}/budget/requests/"
BUDGET_CREATED_PREFIX = f"{STATE_PREFIX}/budget/created/"
BUDGET_APPROVAL_TTL_SECONDS = 3600

DEFAULT_BUDGET_POLICY = {
    "enabled": False,          # off until an operator sets limits
    "per_creation_usd_per_hour": 10.0,
    "per_day_created_usd_per_hour": 100.0,
    "concurrent_running_usd_per_hour": 50.0,
}

_spec_cost_cache = {}


def _budget_policy():
    stored = _state_get(BUDGET_POLICY_KEY)
    policy = dict(DEFAULT_BUDGET_POLICY)
    if isinstance(stored, dict):
        policy.update(stored)
    # A limit that is not a usable number would compare false and let everything through
    for key, default in DEFAULT_BUDGET_POLICY.items():
        value = policy.get(key)
        if key != "enabled" and not (isinstance(value, (int, float))
                                     and not isinstance(value, bool)
                                     and math.isfinite(value) and value >= 0):
            logger.warning(f"budget: stored {key}={value!r} is unusable, using {default}")
            policy[key] = default
    return policy


def _spec_cost(spec_id):
    """Hourly list price for a spec, or None when the provider does not publish one."""
    if spec_id in _spec_cost_cache:
        return _spec_cost_cache[spec_id]
    info = api_request("GET", f"/ns/system/resources/spec/{spec_id}")
    cost = None
    if isinstance(info, dict):
        raw = info.get("costPerHour")
        try:
            value = float(raw)
            cost = value if value >= 0 else None
        except (TypeError, ValueError):
            cost = None
    _spec_cost_cache[spec_id] = cost
    return cost


def _price_node_groups(node_groups):
    """Hourly cost of a planned Infra, and how much of it could not be priced."""
    total, unpriced = 0.0, 0
    for group in node_groups or []:
        if not isinstance(group, dict):
            continue
        size = group.get("nodeGroupSize") or group.get("node_group_size") or 1
        try:
            size = max(1, int(size))
        except (TypeError, ValueError):
            size = 1
        cost = _spec_cost(group.get("specId") or group.get("spec_id") or "")
        if cost is None:
            unpriced += size
        else:
            total += cost * size
    return round(total, 4), unpriced


def _running_spend():
    """Hourly cost of every node that is or may become billed, across all namespaces.

    Nodes on hold and in transition count: they were admitted against the limits, and
    continuing a held Infra is not checked again. Terminated nodes and creations that
    failed before the provider returned an instance do not.
    Read from the stored Infra list, whose nodes carry their spec with its price.
    """
    total, unpriced = 0.0, 0
    namespaces = (api_request("GET", "/ns", params={"option": "id"}) or {}).get("output") \
        or [DEFAULT_NAMESPACE]
    for ns_id in namespaces:
        listing = api_request("GET", f"/ns/{ns_id}/infra")
        for infra in (listing or {}).get("infra") or []:
            for node in infra.get("node") or []:
                status = str(node.get("status", "")).lower()
                if status.startswith("terminated") or (
                        status == "failed" and not node.get("cspResourceId")):
                    continue
                cost = (node.get("spec") or {}).get("costPerHour")
                if not isinstance(cost, (int, float)):
                    cost = _spec_cost(node.get("specId") or "")
                if cost is None or cost < 0:
                    unpriced += 1
                else:
                    total += cost
    return round(total, 4), unpriced


def _created_today():
    stamp = datetime.now().strftime("%Y-%m-%d")
    entries = _state_range(f"{BUDGET_CREATED_PREFIX}{stamp}/")
    return round(sum(float(v.get("usd_per_hour", 0)) for _, v in entries), 4)


def _record_spend(node_groups):
    """Count a creation that went through against today's total."""
    if not _budget_policy().get("enabled"):
        return
    planned, _ = _price_node_groups(node_groups)
    if planned > 0:
        stamp = datetime.now()
        key = f"{BUDGET_CREATED_PREFIX}{stamp.strftime('%Y-%m-%d')}/{stamp.strftime('%H%M%S%f')}"
        _state_put(key, {"usd_per_hour": planned}, ttl_seconds=48 * 3600)


def _budget_refusal(node_groups, budget_ack=None):
    """A refusal when the plan breaks a spend limit, else None. Records nothing.

    An approval covers the amount the administrator saw: it is spent only by a plan that
    costs no more than the request it was granted for.
    """
    policy = _budget_policy()
    if not policy.get("enabled"):
        return None

    planned, unpriced = _price_node_groups(node_groups)
    running, running_unpriced = _running_spend()
    today = _created_today()

    breaches = []
    if planned > policy["per_creation_usd_per_hour"]:
        breaches.append(f"this creation ${planned}/h exceeds the "
                        f"${policy['per_creation_usd_per_hour']}/h per-creation limit")
    if today + planned > policy["per_day_created_usd_per_hour"]:
        breaches.append(f"today's creations would reach ${round(today + planned, 4)}/h, over the "
                        f"${policy['per_day_created_usd_per_hour']}/h daily limit")
    if running + planned > policy["concurrent_running_usd_per_hour"]:
        breaches.append(f"running total would reach ${round(running + planned, 4)}/h, over the "
                        f"${policy['concurrent_running_usd_per_hour']}/h concurrent limit")
    if not breaches:
        return None

    record = _state_get(f"{BUDGET_REQUEST_PREFIX}{budget_ack}") if budget_ack else None
    record = record if isinstance(record, dict) else None
    ack_note = None
    if record:
        status = record.get("status")
        covered = record.get("planned_usd_per_hour")
        if status == "approved" and isinstance(covered, (int, float)) and planned <= covered + 1e-6:
            record["status"] = "used"
            record["used_at"] = datetime.now().isoformat(timespec="seconds")
            _state_put(f"{BUDGET_REQUEST_PREFIX}{budget_ack}", record,
                       ttl_seconds=BUDGET_APPROVAL_TTL_SECONDS)
            logger.info(f"budget: proceeding under approval {budget_ack}")
            return None
        if status == "approved":
            ack_note = (f"{budget_ack} was approved for ${covered}/h and this plan is "
                        f"${planned}/h, so a new request was filed")
        elif status != "pending":
            ack_note = f"{budget_ack} is {status}; an approval works once"
    elif budget_ack:
        ack_note = f"{budget_ack} is not a known request"

    # A pending request is updated in place, so retrying does not pile up requests.
    pending = bool(record) and record.get("status") == "pending"
    token = budget_ack if pending else f"req-{datetime.now().strftime('%Y%m%d%H%M%S%f')}"
    entry = {
        "status": "pending",
        "requested_at": datetime.now().isoformat(timespec="seconds"),
        "planned_usd_per_hour": planned,
        "unpriced_nodes": unpriced,
        "breaches": breaches,
        "node_groups": node_groups,
    }
    if pending and record.get("reason"):
        entry["reason"] = record["reason"]
    _state_put(f"{BUDGET_REQUEST_PREFIX}{token}", entry, ttl_seconds=7 * 24 * 3600)

    refusal = {
        "message": "Refused: over budget. " + "; ".join(breaches),
        "code": "BUDGET_EXCEEDED",
        "request_id": token,
        "planned_usd_per_hour": planned,
        "running_usd_per_hour": running,
        "created_today_usd_per_hour": today,
        "unpriced_nodes": unpriced,
        "unpriced_running_nodes": running_unpriced,
        "note": ("unpriced nodes count as 0 - self-hosted providers publish no price. "
                 "costPerHour is a compute list price and excludes egress, storage IOPS, "
                 "NAT/LB and licensing."),
        "how_to_proceed": ("choose something cheaper, or have an administrator approve "
                           f"{token} and retry the same plan with budget_ack='{token}'"),
    }
    if ack_note:
        refusal["budget_ack"] = ack_note
    return {"error": refusal}


@tool()
def get_budget_status() -> Dict:
    """
    Current spend limits and what has been used against them.

    Limits apply to work done through MCP only; MapUI and direct REST use are not affected.

    Returns:
        The policy, current running spend, today's creations, and any pending requests.
    """
    policy = _budget_policy()
    running, running_unpriced = _running_spend()
    pending = [{"request_id": k.rsplit("/", 1)[-1], **{f: v.get(f) for f in
                ("status", "planned_usd_per_hour", "requested_at", "breaches")}}
               for k, v in _state_range(BUDGET_REQUEST_PREFIX)
               if isinstance(v, dict) and v.get("status") == "pending"]
    return {
        "policy": policy,
        "enforced": bool(policy.get("enabled")),
        "running_usd_per_hour": running,
        "running_nodes_without_a_published_price": running_unpriced,
        "created_today_usd_per_hour": _created_today(),
        "pending_requests": pending[-10:],
        "note": ("costPerHour is a compute list price: it excludes egress, storage IOPS, "
                 "NAT/LB hourly and licensing. Unpriced specs count as 0."),
    }


@tool(read_only=False)  # annotates an approval request
def request_budget_increase(request_id: str, reason: str) -> Dict:
    """
    Ask an administrator to approve a creation that a spend limit refused.

    The agent can ask; it cannot grant. Approval happens outside MCP, and it covers the
    plan that was refused: retry that same plan with budget_ack once it is approved.

    Args:
        request_id: The request_id carried by the BUDGET_EXCEEDED refusal
        reason: Why the spend is justified. Recorded for the administrator to read.

    Returns:
        The request's status and how it gets approved.
    """
    key = f"{BUDGET_REQUEST_PREFIX}{request_id}"
    record = _state_get(key)
    if not isinstance(record, dict):
        return {"error": f"budget request {request_id!r} was not found; repeat the refused "
                         f"creation to get a current request_id"}
    record["reason"] = reason
    if record.get("status") == "pending":
        _state_put(key, record, ttl_seconds=7 * 24 * 3600)
    return {
        "request_id": request_id,
        "status": record.get("status"),
        "planned_usd_per_hour": record.get("planned_usd_per_hour"),
        "administrator_action": f"make mcp-budget-approve ID={request_id}",
        "then": f"retry the same plan with budget_ack='{request_id}'",
    }


# ---------------------------------------------------------------------------
# Namespaces
#
# "default" always exists - CB-Tumblebug creates it at initialisation - so tools default to
# it and the model never has to look one up or choose. Seven tools collapsed to two: what
# they cost was turns spent deciding, more than bytes.
# ---------------------------------------------------------------------------

DEFAULT_NAMESPACE = os.environ.get("MCP_DEFAULT_NAMESPACE", "default")


@tool()
def get_namespaces(ns_id: Optional[str] = None) -> Dict:
    """
    List namespaces, or fetch one by id.

    Args:
        ns_id: Return just this namespace instead of the whole list

    Returns:
        The namespace list, or the single namespace.
    """
    if ns_id:
        return api_request("GET", f"/ns/{ns_id}")
    return api_request("GET", "/ns")


@tool(mutating=True)
def manage_namespace(action: str, name: Optional[str] = None,
                     description: Optional[str] = None, confirm: bool = False) -> Dict:
    """
    Create, update or delete a namespace.

    Most work needs none of this: tools already use the "default" namespace, which
    CB-Tumblebug always provides. Reach for this only when a separate one is wanted.

    Args:
        action: "create" | "update" | "delete"
        name: Namespace name or id
        description: Description, for create and update
        confirm: Must be True for delete - it removes everything recorded in the namespace

    Returns:
        The namespace, with created=false when it already existed, or the deletion result.
    """
    action = (action or "").strip().lower()
    if not name:
        return {"error": "name is required"}
    if action == "create":
        existing = api_request("GET", f"/ns/{name}")
        if isinstance(existing, dict) and "error" not in existing:
            return {"created": False, "namespace": existing}
        created = api_request("POST", "/ns", json_data={"name": name,
                                                        "description": description or ""})
        if not isinstance(created, dict) or "error" in created:
            return created
        return {"created": True, "namespace": created}
    if action == "update":
        if not description:
            return {"error": "description is required for action='update'"}
        return api_request("PUT", f"/ns/{name}",
                           json_data={"name": name, "description": description})
    if action == "delete":
        guard = _require_confirmation(
            confirm, f"deleting namespace {name}",
            "removes the namespace and every resource recorded in it")
        if guard:
            return guard
        return api_request("DELETE", f"/ns/{name}")
    return {"error": f"unknown action {action!r}; expected one of create, update, delete"}


# ---------------------------------------------------------------------------
# Session continuity
# ---------------------------------------------------------------------------

@tool()
def resume_session(session_id: str) -> Dict:
    """
    Recover what a session was doing, after a disconnect or a restart.

    Every call that changed something is journaled under the session_id passed to it, so
    this reports what was actually done. Refusals and rejected arguments are not recorded.

    Args:
        session_id: The identifier used on earlier calls

    Returns:
        last_actions (tool, action, target, outcome, time), and unfinished: the calls
        whose outcome is unknown because they failed or timed out partway.
    """
    entries = [v for _, v in _state_range(_session_key(session_id, "journal/"))]
    if not entries:
        return {"session_id": session_id, "known": False,
                "message": "no journal for this session id",
                "hint": "pass the same session_id on later calls to build one"}
    actions = []
    for entry in entries:
        args = entry.get("arguments") or {}
        target = next((args[k] for k in ("rdbms_id", "cluster_id", "bucket_id", "sg_id",
                                         "resource_id", "template_id", "nlb_id")
                       if args.get(k)), None)
        if not target and args.get("infra_id"):
            target = "/".join(filter(None, (args["infra_id"], args.get("node_id")
                                            or args.get("nodegroup_id"))))
        target = target or args.get("name") or args.get("ns_id")
        row = {"tool": entry.get("tool"), "action": args.get("action"), "target": target,
               "outcome": entry.get("outcome"), "at": entry.get("at")}
        actions.append({k: v for k, v in row.items() if v})
    unfinished = [a for a in actions if a.get("outcome") != "ok"]
    out = {"session_id": session_id, "known": True, "actions_recorded": len(actions),
           "last_actions": actions[-15:]}
    if unfinished:
        out["unfinished"] = unfinished[-5:]
        out["next"] = ("the unfinished calls may have partly taken effect: read their targets "
                       "with get_infra() or get_resources() before repeating them")
    return out


@tool(mutating=True)
def clear_session(session_id: str, confirm: bool = False) -> Dict:
    """
    Discard a session's journal.

    Args:
        session_id: Session to clear
        confirm: Must be True to proceed

    Returns:
        Confirmation of the removal.
    """
    guard = _require_confirmation(confirm, "clearing a session journal",
                                  "the record of what this session did is lost")
    if guard:
        return guard
    _state_delete_prefix(_session_key(session_id, ""))
    return {"session_id": session_id, "cleared": True}


# ---------------------------------------------------------------------------
# Consolidated reads
#
# Several tools differed only by one word in the path and were otherwise identical, which
# made the model choose between indistinguishable options and cost a schema each. One tool
# with a `kind` does the same work, and a new resource type becomes a new enum value rather
# than a new tool.
# ---------------------------------------------------------------------------

_RESOURCE_KINDS = {
    "vNet": ("vNet", ("id", "name", "connectionName", "cidrBlock", "status",
                      "subnetInfoList", "associatedObjectList", "deletionRequestedAt")),
    "securityGroup": ("securityGroup", ("id", "name", "connectionName", "vNetId",
                                        "description", "firewallRules", "associatedObjectList",
                                        "status", "deletionRequestedAt")),
    "sshKey": ("sshKey", ("id", "name", "connectionName", "username", "verifiedUsername",
                          "fingerprint", "associatedObjectList", "description",
                          "status", "deletionRequestedAt")),
    "dataDisk": ("dataDisk", ("id", "name", "connectionName", "diskType", "diskSize",
                              "status", "associatedObjectList",
                              "deletionRequestedAt", "systemMessage")),
    "customImage": ("customImage", ("id", "name", "connectionName", "cspImageName", "status",
                                    "description", "osType", "sourceNodeUid",
                                    "imageStatus", "deletionRequestedAt", "systemMessage")),
    "objectStorage": ("objectStorage", ("id", "name", "connectionName", "status",
                                        "description", "creationDate", "systemMessage")),
    "rdbms": ("rdbms", ("id", "name", "connectionName", "status", "dbEngine",
                        "dbEngineVersion", "dbInstanceSpec", "endpoint", "publicAccess",
                        "vNetId", "storageSize", "systemMessage", "deletionRequestedAt")),
    "nlb": ("nlb", ("id", "name", "connectionName", "type", "scope", "listener",
                    "targetGroup", "status", "location")),
}


@tool()
def get_resources(kind: str, ns_id: str = DEFAULT_NAMESPACE,
                  resource_id: Optional[str] = None, detail: str = "summary",
                  fields: Optional[str] = None) -> Dict:
    """
    List resources of one kind in a namespace, or fetch one by id.

    A resource showing deletionRequestedAt was asked to be deleted and the provider has
    not confirmed it yet; manage_resource() retries, restores or purges it.

    Args:
        kind: "vNet" | "securityGroup" | "sshKey" | "dataDisk" | "customImage" |
              "objectStorage" | "rdbms" | "nlb"
        ns_id: Namespace ID (defaults to the standing namespace)
        resource_id: Return just this resource (not for "nlb", which belongs to an Infra)
        detail: "minimal" | "summary" (default) | "full"
        fields: Comma-separated allowlist of fields to keep

    Returns:
        The resource list under its own key, e.g. {"vNet": [...]}, or the one resource.
        Private keys are redacted; get_ssh_private_key returns one deliberately.
    """
    entry = _RESOURCE_KINDS.get(str(kind).strip())
    if not entry:
        return {"error": f"unknown kind {kind!r}; expected one of "
                         f"{', '.join(sorted(_RESOURCE_KINDS))}"}
    path, summary_keys = entry
    level = _validate_detail(detail)
    if resource_id:
        if path == "nlb":
            return {"error": "resource_id is not supported for kind 'nlb'; use "
                             "manage_nlb(action='get')"}
        one = api_request("GET", f"/ns/{ns_id}/resources/{path}/{resource_id}")
        if not isinstance(one, dict) or "error" in one or level == "full":
            return one
        if level == "minimal":
            return {"id": one.get("id"), "status": one.get("status")}
        return _project(_summarize_resource(one, summary_keys), fields)
    return _shape_resource_list(
        api_request("GET", f"/ns/{ns_id}/resources/{path}"), path, level, fields, summary_keys)


@tool(mutating=True, read_actions=("get", "find"))
def manage_labels(action: str, label_type: str, uid: Optional[str] = None,
                  labels: Optional[Union[Dict, str]] = None, key: Optional[str] = None,
                  detail: str = "summary") -> Dict:
    """
    Read, set, remove or sync resource labels, and find resources by label.

    Labels are what label_selector targets when a remote command should reach a whole tier,
    so a role label placed at creation is usually the one that matters later.

    Args:
        action: "get" | "set" | "remove" | "find" | "sync_from_csp"
        label_type: Resource type, e.g. "infra", "node", "vNet"
        uid: Resource uid - required for get, set, remove and sync_from_csp. get_infra()
            lists it for the Infra and for each node.
        labels: Key-value pairs, e.g. {"role": "control"}. For "find" a selector string
            such as "role=control,tier=web" is accepted as well, and a mapping given
            there is joined into one.
        key: Label key to delete, for "remove"
        detail: For "find": "summary" (default) trims each match; "full" returns the records

    Returns:
        The labels, the matching resources, or the result of the change.
    """
    action = (action or "").strip().lower()
    if action == "find":
        if not labels:
            return {"error": "labels (a selector string) is required for action='find'"}
        selector = labels if isinstance(labels, str) else ",".join(
            f"{k}={v}" for k, v in labels.items())
        result = api_request("GET", f"/resources/{label_type}",
                             params={"labelSelector": selector})
        matches = result.get("results") if isinstance(result, dict) else None
        level = _validate_detail(detail)
        if level == "full" or not isinstance(matches, list):
            return result
        shape = {"node": _shape_node, "infra": _shape_infra}.get(label_type)
        return {"results": [shape(m, level) if shape else _slim_resource(m) for m in matches],
                "count": len(matches)}
    if not uid:
        return {"error": f"uid is required for action={action!r}"}
    if action == "get":
        return api_request("GET", f"/label/{label_type}/{uid}")
    if action == "set":
        if not labels or not isinstance(labels, dict):
            return {"error": "labels (a key-value mapping) is required for action='set'"}
        # The body is model.Label; a bare mapping is accepted with 200 and sets nothing.
        return api_request("PUT", f"/label/{label_type}/{uid}",
                           json_data={"labels": {str(k): str(v) for k, v in labels.items()}})
    if action == "remove":
        if not key:
            return {"error": "key is required for action='remove'"}
        return api_request("DELETE", f"/label/{label_type}/{uid}/{key}")
    if action == "sync_from_csp":
        return api_request("PUT", f"/mergeCSPLabel/{label_type}/{uid}")
    return {"error": f"unknown action {action!r}; expected get, set, remove, find "
                     f"or sync_from_csp"}


@tool()
def get_search_options(kind: str) -> Dict:
    """
    The filter and priority vocabulary accepted by search_images and recommend_spec.

    Args:
        kind: "image" | "spec"

    Returns:
        Valid field names and values for that search.
    """
    kind = (kind or "").strip().lower()
    if kind == "image":
        return api_request("GET", "/ns/system/resources/searchImageOptions")
    if kind == "spec":
        return api_request("GET", "/recommendSpecOptions")
    return {"error": f"unknown kind {kind!r}; expected image or spec"}


# Helper: node addresses in the access-info shape, read from stored state.
#
# This was an MCP tool. It was removed because an LLM does not need it: commands, file
# transfer and downloads all run server-side with CB-Tumblebug holding the key, and the
# addresses it returned are already in get_infra - identical values, arriving sooner.
# Its one distinguishing feature, a live CSP lookup per node, cost ~34 s against a slow
# provider and produced the same answer.
#
# Internal callers only ever wanted the addresses, so they read stored state here.
def _infra_access_info(ns_id: str, infra_id: str, show_ssh_key: bool = False) -> Dict:
    infra = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}")
    if not isinstance(infra, dict) or "error" in infra:
        return infra if isinstance(infra, dict) else {"error": "could not read infra"}
    groups = {}
    for node in infra.get("node") or []:
        gid = node.get("nodeGroupId") or "default"
        groups.setdefault(gid, []).append({
            "nodeId": node.get("id"),
            "publicIP": node.get("publicIP"),
            "privateIP": node.get("privateIP"),
            "sshPort": node.get("sshPort"),
            "nodeUserName": node.get("nodeUserName") or "cb-user",
        })
    return {
        "InfraId": infra_id,
        "InfraNodeGroupAccessInfo": [
            {"NodeGroupId": gid, "NodeAccessInfo": nodes} for gid, nodes in groups.items()
        ],
    }


@tool()
def get_command_result(request_id: str, ns_id: str = DEFAULT_NAMESPACE,
                       infra_id: Optional[str] = None, tail_lines: int = 20,
                       contains: Optional[str] = None, context_lines: int = 3) -> Dict:
    """
    Outcome of a command started with run_async=True.

    Polls rather than streams. A live feed of a long install is tens of thousands of tokens
    of progress output that nothing reads; the finished result carries the same information
    trimmed to what diagnoses a failure.

    Args:
        request_id: The request_id returned when the command was started
        ns_id: Namespace ID
        infra_id: Infra ID. Given, per-node records are included.
        tail_lines: Lines of output kept per node (default 20)
        contains: Return only the lines matching this regex, with context_lines around
            each, instead of the tail. Use it to reach into a long install log - e.g.
            contains="IdentityEndpoint|ProjectID" for a DevStack's credentials block.
        context_lines: Lines kept either side of a match (default 3)

    Returns:
        status, statusCount and per-node output. Poll while status is Handling or Pending;
        CompletedWithError means at least one node did not finish cleanly.
    """
    record = api_request("GET", f"/request/{request_id}")
    out: Dict[str, Any] = {"request_id": request_id}
    accepted = isinstance(record, dict) and "error" not in record
    if accepted:
        # The stored request repeats the headers and body that were sent; what matters
        # is whether the server took it.
        out["request"] = {k: record[k] for k in ("status", "startTime", "errorResponse")
                          if record.get(k)}

    if infra_id:
        nodes, counts = [], {}
        listing = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}")
        for node in (listing or {}).get("node") or []:
            for entry in (node.get("commandStatus") or []):
                if entry.get("xRequestId") != request_id:
                    continue
                state = entry.get("status") or "Unknown"
                counts[state] = counts.get(state, 0) + 1
                nodes.append({"nodeId": node.get("id"),
                              **_trim_command_record(entry, tail_lines, contains, context_lines)})
        still_running = sum(v for k, v in counts.items() if k in ("Handling", "Queued"))
        failed = sum(v for k, v in counts.items()
                     if k not in ("Handling", "Queued", "Completed"))
        if len(nodes) > NODE_LIST_LIMIT:
            # Keep the records that did not finish cleanly; the rest are in statusCount.
            nodes = ([n for n in nodes if n.get("status") != "Completed"]
                     + [n for n in nodes if n.get("status") == "Completed"])[:NODE_LIST_LIMIT]
            out["nodesCapped"] = True
        out["nodes"] = nodes
        out["statusCount"] = counts
        out["running"] = still_running
        if still_running:
            out["status"] = "Handling"
            out["hint"] = "still running - poll this again rather than re-issuing the command"
        elif nodes:
            out["status"] = "CompletedWithError" if failed else "Completed"
        elif accepted:
            # An accepted command appears on the nodes a moment after the call returns.
            out["status"] = "Pending"
            out["hint"] = "accepted but not on any node yet - poll again in a few seconds"
        else:
            out["status"] = "Unknown"
    return out


def _zone_is_delegated(zone, timeout=4):
    """Is this hosted zone reachable from the public internet?

    A hosted zone can exist in the account while the registrar still points the domain
    somewhere else. Records then write successfully, read back correctly, and resolve for
    nobody - which is indistinguishable from working unless something asks a public
    resolver. One NS query settles it.

    Returns True, False, or None when the check itself could not be made.
    """
    import struct
    name = zone.rstrip(".")
    query = struct.pack(">HHHHHH", 0x1234, 0x0100, 1, 0, 0, 0)
    for part in name.split("."):
        query += bytes([len(part)]) + part.encode()
    query += b"\x00" + struct.pack(">HH", 2, 1)  # NS, IN
    for resolver in ("8.8.8.8", "1.1.1.1"):
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.settimeout(timeout)
        try:
            sock.sendto(query, (resolver, 53))
            data, _ = sock.recvfrom(2048)
            rcode = data[3] & 0x0F
            answers = struct.unpack(">H", data[6:8])[0]
            return rcode == 0 and answers > 0
        except Exception:
            continue
        finally:
            sock.close()
    return None


# Tool: Global DNS records, including pointing a name at a whole Infra
@tool(mutating=True, read_actions=("get",))
def manage_dns_record(action: str, domain_name: str, record_name: Optional[str] = None,
                      record_type: str = "A", ttl: int = 300,
                      routing_policy: str = "simple",
                      infra_id: Optional[str] = None,
                      ns_id: str = DEFAULT_NAMESPACE,
                      label_selector: Optional[str] = None,
                      ips: Optional[List[str]] = None,
                      set_identifier: Optional[str] = None,
                      confirm: bool = False) -> Dict:
    """
    Read, set or delete a global DNS record.

    A record can be pointed at an Infra rather than at addresses. CB-Tumblebug then resolves
    the node addresses itself and keeps the record as one name for the whole deployment -
    which is the point when the nodes sit in ten regions and their IPs are not memorable.

    One source must be given for "set":
      infra_id        every running node of that Infra
      label_selector  nodes matching a label; with infra_id, only that Infra's nodes
      ips             explicit addresses (not combined with the other two)

    Args:
        action: "get" | "set" | "delete"
        domain_name: Hosted zone, e.g. "cloud-barista.io" (see get_dns_hosted_zones)
        record_name: Full record name, e.g. "oss-korea.cloud-barista.io". Defaults to the
            zone apex.
        record_type: "A" (default) | "AAAA" | "CNAME" | "TXT"
        ttl: Seconds, default 300
        routing_policy: "simple" (default) | "geoproximity" | "weighted".
            geoproximity sends each client to the nearest node and needs an Infra or label
            source, because it reads each node's location - it cannot work from bare IPs.
        infra_id: Source Infra for "set"
        ns_id: Namespace of that Infra
        label_selector: Label filter, e.g. "role=web"
        ips: Explicit addresses for "set"
        set_identifier: For "delete": remove only this entry of a weighted or geoproximity
            record (setIdentifier from "get"). Omitted, every entry of the name goes.
        confirm: Must be True for "set" and "delete" - DNS changes are visible to everyone

    Returns:
        The records, or the result of the change.
    """
    action = (action or "").strip().lower()
    if action == "get":
        params = {"domainName": domain_name}
        if record_name:
            params["recordName"] = record_name
        return api_request("GET", "/resources/globalDns/record", params=params)

    guard = _require_confirmation(
        confirm, f"changing DNS for {record_name or domain_name}",
        "the record is public; clients will follow it as soon as it propagates")
    if guard:
        return guard

    if action == "delete":
        body = {"domainName": domain_name,
                "recordName": record_name or domain_name,
                "recordType": record_type}
        if set_identifier:
            body["setIdentifier"] = set_identifier
        return api_request("DELETE", "/resources/globalDns/record", json_data=body)

    if action != "set":
        return {"error": f"unknown action {action!r}; expected get, set or delete"}

    # Writing into an undelegated zone reports success and resolves for nobody. Catch it
    # here rather than letting the caller discover it from a browser that never connects.
    if _zone_is_delegated(domain_name) is False:
        return {"error": {
            "message": f"Refused: '{domain_name}' is not delegated to this hosted zone",
            "code": "ZONE_NOT_DELEGATED",
            "detail": {"domain": domain_name},
            "how_to_proceed": ("the zone exists in the account but the registrar points the "
                               "domain elsewhere, so a record here resolves for nobody. Run "
                               "get_dns_hosted_zones() and ask the user to pick one of "
                               "usable_domains."),
        }}

    # infra_id and label_selector combine (the selector is scoped to the Infra); ips stands alone.
    if bool(infra_id or label_selector) == bool(ips):
        return {"error": "give one source: infra_id and/or label_selector, or ips"}
    if routing_policy == "geoproximity" and ips:
        return {"error": "geoproximity needs an Infra or label source - it reads each "
                         "node's location, which bare IPs do not carry"}

    set_by: Dict[str, Any] = {}
    if infra_id and not label_selector:
        set_by["infra"] = {"nsId": ns_id, "infraId": infra_id}
    elif label_selector:
        # Scope the selector to one Infra when both are given, matching how MapUI does it.
        selector = (f"sys.infraId={infra_id},{label_selector}" if infra_id else label_selector)
        set_by["label"] = {"nsId": ns_id, "labelSelector": selector}
    else:
        set_by["ips"] = ips

    body = {"domainName": domain_name,
            "recordName": record_name or domain_name,
            "recordType": record_type,
            "ttl": int(ttl),
            "routingPolicy": routing_policy,
            "setBy": set_by}
    return api_request("PUT", "/resources/globalDns/record", json_data=body)


# Tool: Long-form usage guidance, fetched on demand
_USAGE_GUIDES = {
    "provision": """Provisioning an Infra, end to end.

1. Pick specs.  recommend_vm_spec(filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4},
   "providerName": "aws", "regionName": "ap-northeast-2"}, priority_policy="cost")
   ->  specs[i]["id"], e.g. "aws+ap-northeast-2+t3.small". Use the id exactly as returned:
   it encodes provider, region and instance type, and an edited id fails at the CSP.

2. Pick an image per spec.  search_images(matched_spec_id=spec_id, os_type="ubuntu 22.04")
   ->  imageList[0]["cspImageName"]. Image ids are specific to a provider and region, so
   every spec needs its own.

3. Build node_groups, one object per group of identical nodes:
   [{"name": "web", "specId": spec_id, "imageId": csp_image_name, "nodeGroupSize": 2,
     "label": {"role": "web"}}]
   Nodes are named "<name>-1", "<name>-2", ... The label matters later: label_selector on
   a remote command targets it. Optional per group: rootDiskType and rootDiskSize (GB),
   zone, distributeSubnets (spread the nodes over zones), connectionName, initScript.
   Valid zones and disk values for a spec: get_spec_options(spec_id).

4. Review.  review_infra_dynamic_request(ns_id, name, node_groups)
   Check creationViable and each nodeReviews entry. specValidation.quota.fitCount is how
   many more nodes of that spec the account can start; warnings carry the spec's past
   failure rate. Fix what it reports and review again.

5. Show the user the node groups and estimatedCost, and get their agreement.

6. Create.  create_infra_dynamic(ns_id, name, node_groups, force_create=True, confirm=True)
   force_create says the review passed, confirm says a person agreed. Without confirm the
   call refuses and returns the plan with its hourly cost.

estimatedCost is a compute list price: it excludes egress, storage, NAT/LB hours and
licensing. A review can pass and creation still fail on live stock or a provider fault;
see get_usage_guide('recovery').

Bootstrapping. post_script runs over SSH once the nodes are up:
    {"phases": [{"nodeGroupId": "web", "command": ["sudo apt-get install -y nginx"]}]}
The call waits for it and returns postScriptStatus. For anything slow pass
post_script_async=True and poll get_infra() until postScriptStatus is terminal.
init_script runs inside the node at first boot and needs no SSH: get_usage_guide('initscript').

Large or slow creations. The call answers when every node is up. hold=True registers the
Infra without provisioning; control_infra(action='continue') then starts it.

A saved configuration: list_infra_templates() and provision_infra_from_template().
manage_infra_template(action='save_from_infra') turns a running Infra into a template.""",

    "initscript": """First-boot scripts, the notifier and observability.

Two ways to configure a node, and they differ in who runs the script:
    post_script   CB-Tumblebug connects over SSH after the node is up. Ordered phases,
                  per-phase targets, output returned. Needs the node to be reachable.
    init_script   The node runs it itself at first boot, from its user data. No SSH, so
                  it works behind a private address and scales to thousands of nodes.
                  The node reports the result through the notifier.

init_script shapes (Infra-level argument, or "initScript" inside one node group):
    {"templateId": "initscript-nginx", "env": {"SITE_TITLE": "demo"}}
    {"script": "#!/bin/bash\\nset -e\\napt-get install -y jq"}
    {"scripts": [{"name": "setup", "script": "#!/bin/bash\\n...", "timeoutSec": 300}]}
    {"mode": "none"}                       no init script, not even the default one
A script starts with its shebang line. Steps run in order as root; the first failure
stops the run unless that step sets continueOnError. User data is limited (about 16 KB
on most providers); the review reports the size.

Templates: list_init_script_templates() lists them; with template_id it returns the
script text. The same template can run later over SSH -
execute_command_infra(template_id=...) - which is how to re-run it on nodes that failed.

Results: get_init_script_status(ns_id, infra_id) gives counts per state;
detail='full', status='fail' names the nodes and the failing step.
    ok / fail        the node reported
    initializing     the script is running
    pending          not reported yet (still booting)
    noReport         Running but silent past the timeout: user data did not run, or the
                     node cannot reach the notifier
    notApplicable    no init script, or reporting is off

Reports need a notifier. get_platform_services() shows whether one is registered; when
none is, every node is notApplicable and the script still runs - only the report is
missing. The same call shows the observability stack (OTLP endpoint, Grafana URL); the
initscript-otel-agent template installs the agent that sends a node's metrics and logs
there.""",

    "commands": """Running commands on nodes.

Each element of `commands` is a separate SSH session. Shell state does not survive
between elements:
    WRONG  ["IFACE=$(ip route | ...)", "echo $IFACE"]      # second sees an empty IFACE
    RIGHT  ["IFACE=$(ip route | ...) && echo $IFACE"]

Exit status is the last command's. A trailing echo masks a failure - `installer.sh; echo done`
reports success no matter what the installer did. Use && between steps, or set -o pipefail.

Targeting: node_id for one node, nodegroup_id for a NodeGroup, label_selector for a role
across NodeGroups ("role=worker"). Without any, the command runs on every node.

The result carries nodes: {total, succeeded, failed}; a failure on one node does not set
isError, so read it. A node that has only just reached Running, or was just rebooted, can
refuse SSH for up to a minute ("SSH authentication failed", "connection refused") while
its login user and key are set up - retry rather than report a failure.

Long-running work: pass run_async=True and follow with get_command_result(request_id).
A dropped connection then costs nothing. timeout_minutes raises the per-command limit
(server default 30, max 120). manage_command_task(action='list') shows what is still
running and can cancel it.

Output: each stream is cut to tail_lines; `contains` returns the lines matching a regex
instead, which reaches a block in the middle of a long log. Earlier commands of a node:
list_node_command_status(), and index=... for one record with its output.

Values the server fills in on each node, usable inside a command:
    $$Func(GetPublicIP(target=this))                      this node's public address
    $$Func(GetPublicIP(target=this, prefix=http://, postfix=:8000/v1))
    $$Func(GetPublicIPs(separator=' '))                   every node's public address
    $$Func(GetPrivateIPs(separator=',', label='role=worker'))   nodes matching a label
    $$Func(GetInfraId())    $$Func(GetNodeId())
{{public_ip}}, {{public_ips_space}}, {{public_ips_comma}} and {{private_ips_space}} are
filled by this server before sending, from the whole Infra.

Predefined scripts: list_predefined_scripts(), run with script_name. Files:
transfer_file_infra() writes text to nodes, download_file_from_node() reads one back.
After installing a service open its port: add_security_group_rules().""",

    "gpu": """Serving an LLM on a GPU node.

Choosing the machine. Filtering on acceleratorCount alone is not enough - it returns AMD
cards, and the vLLM/KServe stack is CUDA-only, so an AMD node fails after provisioning:

    recommend_vm_spec(filter_policies={"providerName": "aws", "regionName": "eu-central-1",
                                       "acceleratorCount": {"min": 1},
                                       "acceleratorModel": "nvidia"},
                      priority_policy="cost")

In eu-central-1 that is the difference between g4ad.xlarge (AMD, will not run the model)
and g4dn.xlarge (NVIDIA T4, will). Filter on acceleratorMemoryGB for the memory the model
needs: an acceleratorCount of 0 marks a fractional slice of a card.
get_search_options("spec") lists the filter vocabulary.

Choosing the image. search_images(matched_spec_id=..., is_gpu_image=True) returns images
with the driver already present.

Sizing the model to the card. A T4 has 16 GB: a 1.5B-7B model in half precision fits.
Qwen/Qwen2.5-1.5B-Instruct is a safe small choice; 7B needs quantisation or an L4/A10.

Two ways to serve. Pick the simple one unless a cluster is wanted for its own sake.

A. Straight on the VM - no Kubernetes. Fewer moving parts, much faster to stand up:
    1. gpu_driver_install     installs the NVIDIA driver, --no-reboot (a reboot mid-command
                              drops the SSH channel and reads as a failure)
    2. gpu_node_reboot        then wait about a minute
    3. gpu_driver_check       nvidia-smi must show the card before going further
    4. vllm_install           vLLM onto the VM
    5. vllm_serve             model_id=Qwen/Qwen2.5-1.5B-Instruct, listens on :8000
    6. open_webui_vllm        vllm_url=http://<node-ip>:8000/v1, UI on :80
    Open ports 80 (and 8000 if the API should be reachable) in the security group.

B. Through Kubernetes and KServe. Worth it when the cluster is already there, or when
   several models and autoscaling are in play:
    1. k8s_control_plane            (CNI of your choice)
    2. k8s_worker_join              on each GPU worker
    3. k8s_kserve_stack             StorageClass, GPU Operator, cert-manager, KServe (10-20 min)
    4. serve_vllm_model             model_id=Qwen/Qwen2.5-1.5B-Instruct, isvc_name=llm
    5. k8s_open_webui               node_port=30080; it auto-discovers the served model
    6. publish_nodeport_via_host    only when the node sits behind a private address

Run the long steps with run_async=True and follow them with get_command_result();
discover_service_endpoints() finds the model endpoints that are answering.""",

    "recovery": """When something fails partway through.

Creation is not all-or-nothing. policy_on_partial_failure defaults to "continue", so a
request for ten nodes can leave nine running and one Failed, and the Infra sits at
Partial-Running.

See what happened:
    diagnose_infra(ns_id, infra_id)    counts, the failed nodes with the provider's
                                       message, script outcomes and the retry plan

Retry in place - same node group, same configuration:
    retry_failed_nodes(ns_id, infra_id)                 preview: what is retriable and why
    retry_failed_nodes(..., dry_run=False, confirm=True)
The server classifies each failure. Capacity and transient faults are retried; quota and
configuration faults are held back until fixed (then pass node_ids with
assume_resolved=True). zone moves the replacements to another zone of the same VNet.

Clear the wreckage, keep the rest:
    control_infra(action='refine')     removes the Failed nodes; the Infra returns to Running

Replace what was lost with something else:
    recommend_alternative_node_config(source_spec_id, target_provider)
                                       closest spec and images in another cloud or region
    review_nodegroup_dynamic(...)      check the substitute first
    add_nodegroup_dynamic(...)         add it under a NEW name
One node misbehaving: control_node(action='reboot'), or 'delete' to drop it.

Use a new name. A node group keeps its security group and key pair after its nodes die,
and those are keyed by Infra and node-group name without the region. Reusing the name in
another region attaches a security group that belongs to the first one.

Why a review can pass and creation still fail: the review checks that the spec and image
exist, their price and the account's quota. It cannot see live stock or faults inside
the provider. Treat these as a retry signal, not a bug:
    no zone with stock right now       another zone, spec or region
    provider-specific 4xx              another provider for that location

An Infra stuck mid-provisioning after a server restart: control_infra(action='reconcile')
adopts what exists at the CSP; 'abort' terminates what is unfinished.

Probing cheaply: review_spec_image_pair(spec_id, image_id) validates one pair in well
under a second, and get_spec_options(spec_id, refresh=True) asks the provider whether the
spec can be started right now.""",

    "cleanup": """Removing what was created.

    terminate_infra(ns_id, infra_id, confirm=True)
                                       terminates every node and deletes the Infra
    release_resources(ns_id, dry_run=True)
                                       lists the VNets, security groups and keys nothing
                                       uses any more; confirm=True removes only those

An Infra's VNet, security groups and SSH key outlive it on purpose - the next Infra in
that region reuses them - so a namespace is clean only after both steps.

A deletion the provider has not confirmed keeps the record: get_resources() shows it with
deletionRequestedAt. manage_resource(action='delete') tries again; action='restore'
cancels the deletion when the resource is still wanted.

Checking for leftovers at the provider: inspect_csp_resources(connection_name) compares
CB-Tumblebug's records with what the CSP reports for that region.

Other things that are billed until removed: database instances (manage_rdbms; delete them
before the VNet they live in), buckets (manage_object_storage), load balancers
(manage_nlb; a global one is its own "<infra>-nlb" Infra), managed Kubernetes clusters
(manage_k8s_cluster), custom images (manage_resource) and DNS records
(manage_dns_record).""",

    "services": """Beyond virtual machines.

Object storage.  manage_object_storage(action='create', bucket_id=name,
connection_name='aws-ap-northeast-2'). Data does not pass through this server:
action='presigned_url' with operation='upload' or 'download' returns a time-limited URL,
and the node uses it directly, e.g.
    execute_command_infra(commands=['curl -fsS -T /var/log/syslog "<url>"'], node_id=...)
which moves data without putting cloud credentials on the node. The answer names the
HTTP method (PUT for an upload) and, at some providers, requiredHeaders: send each one
(`curl -H 'x-ms-blob-type: BlockBlob' ...` on Azure) or the upload is refused. Object keys
may contain "/".

Load balancer.  manage_nlb(action='create', nodegroup_id='web', port=80) puts the
provider's NLB in front of one node group; action='health' lists healthy and unhealthy
targets, which takes a few minutes to settle, and the address is listener.dnsName.
scope='GLOBAL' fronts every node of an Infra that spans regions or clouds: it builds a
billed HAProxy Infra "<infra_id>-nlb", opens the listener port on it and returns its
addresses. HAProxy reaches the nodes on their public addresses, so open the target port
in their security groups. terminate_infra("<infra_id>-nlb") removes it.

One name for the deployment.  get_dns_hosted_zones() then manage_dns_record(action='set',
infra_id=...) points a record at an Infra's nodes; routing_policy='geoproximity' sends
each client to the nearest one.

Managed Kubernetes.  manage_k8s_cluster(action='create', cluster_id=name, spec_id=...).
action='info' says what each provider supports, 'versions' lists Kubernetes versions for
the spec's region. Creation takes 10-20 minutes; poll action='get' until status is Active.
Where info reports nodegroups_on_creation false (AWS, Alibaba) the cluster starts with no
nodes: add them with action='add_nodegroup' once it is Active. action='kubeconfig' returns
a file kubectl can use as is; where the provider relies on a credential plugin it holds a
short-lived token instead. To remove a cluster, delete its node groups first where the
provider asks for it (the delete answers CONFLICT), then the cluster. kubeadm on plain VMs
is the other route: the k8s_* predefined scripts.

Custom images.  create_custom_image(ns_id, infra_id, name) snapshots a configured node;
search_images(custom_images_of=ns_id) finds it for the next Infra in that region.

Managed database.  An instance joins a VNet that exists, so create the Infra first and
take its VNet, two subnets in different zones and a security group from
get_resources(kind='vNet') and get_infra_associated_resources():
    manage_rdbms(action='validate', rdbms_id='appdb', connection_name='aws-ap-northeast-2',
                 vnet_id=..., subnet_ids=[..., ...], security_group_ids=[...],
                 admin_user_name='dbadmin', admin_user_password=...)
then action='create' with confirm=True. action='support' says what each provider needs
(some reject the admin name "admin", fix it to "root", or ignore security groups);
action='capability' lists versions and instance specs. Open the database port (3306) in
the security group for the VNet's address range, then reach the endpoint from a node of
that VNet, passing the password through env so it stays out of the command line:
    execute_command_infra(commands=["mysql -h <host> -u dbadmin -e 'SHOW DATABASES'"],
                          env={"MYSQL_PWD": "<pw>"}, node_id=...)
The database actions of manage_rdbms work through the provider's API or a public
endpoint; a private instance on AWS or IBM answers UNSUPPORTED and is managed with SQL
from a node, as above.

Cleaning up.  Delete databases and clusters before release_resources(): they live in
the Infra's VNet, and the release is refused while they do.

Not covered by this server: site-to-site VPN and the autopilot provisioning API. They are
available in MapUI and the REST API.""",

    "detail": """Controlling response size.

Read tools take `detail`:
    minimal  ids and status only
    summary  the default - identifiers, status, addresses, and counts
    full     everything the server returned, secrets still redacted

and `fields`, a comma-separated allowlist applied after tiering:
    get_infra(ns_id, infra_id, fields="id,status,node")

Command history is collapsed at summary to {count, last_status, last_request_id}; fetch the
transcript deliberately rather than carrying it in every read.

Large Infras: past 50 nodes a summary read lists the nodes that are not Running first,
then fills up to the cap, and reports the rest as counts in nodeListCapped (byStatus,
largestNodeGroups). Per-node lists in other tools are capped the same way.
get_nodegroups() gives every node id of a group.

Secrets are stripped from every response. get_ssh_private_key() is the one deliberate
exception and puts the key into the transcript.

A failed call sets isError and carries {"error": {"code", "message", "next"}}. Codes:
NEEDS_CONFIRMATION (ask the user, repeat with confirm=True), REVIEW_REQUIRED,
INVALID_ARGUMENT, NOT_FOUND, ALREADY_EXISTS, BUDGET_EXCEEDED, RATE_LIMITED, TIMEOUT,
UPSTREAM_ERROR, UPSTREAM_UNREACHABLE, READ_ONLY.""",
}


@tool()
def get_usage_guide(topic: str) -> Dict:
    """
    Long-form guidance kept out of the tool schemas.

    Tool descriptions carry only what is needed to call them correctly; worked examples and
    workflow notes live here so they cost nothing until asked for.

    Args:
        topic: "provision" | "initscript" | "commands" | "gpu" | "recovery" | "cleanup" |
            "services" | "detail"

    Returns:
        The guide text, or the list of topics if the name is unknown.
    """
    text = _USAGE_GUIDES.get((topic or "").strip().lower())
    if text is None:
        return {"error": f"unknown topic {topic!r}; expected one of "
                         f"{', '.join(sorted(_USAGE_GUIDES))}"}
    return {"topic": topic, "guide": text}


# Tool: Get an SSH private key (explicitly, because it lands in the transcript)
@tool(redact=False, read_only=False)  # hands out a secret; never auto-approved
def get_ssh_private_key(ns_id: str, key_id: str, reason: str) -> Dict:
    """
    Return the private key material of one SSH key resource.

    Every other tool redacts private keys, because the agent does not need them: remote
    commands and file transfers run through CB-Tumblebug, which holds the key server-side.
    Use this only when a human asked for the key itself, for example to SSH by hand.

    The key will appear in the conversation transcript and in any logs of it. Prefer
    downloading it from MapUI when that is an option.

    Args:
        ns_id: Namespace ID
        key_id: SSH key resource ID (see get_resources(kind='sshKey'))
        reason: why the key is needed - recorded in the server log for audit

    Returns:
        The key resource including privateKey, or an error
    """
    logger.warning(f"get_ssh_private_key: ns={ns_id} key={key_id} reason={reason!r}")
    result = api_request("GET", f"/ns/{ns_id}/resources/sshKey/{key_id}")
    if isinstance(result, dict) and "error" not in result:
        result = {k: result[k] for k in ("id", "connectionName", "username", "verifiedUsername",
                                         "fingerprint", "privateKey") if result.get(k)}
        result["_warning"] = "private key material is now in this transcript"
    return result


def _managed_service_resources(ns_id):
    """Shared resources that clusters and databases use; the server does not track them."""
    used = {}

    def claim(kind, ids, owner):
        for rid in ids:
            if rid:
                used.setdefault((kind, rid), owner)
    clusters = api_request("GET", f"/ns/{ns_id}/k8sCluster")
    for c in (clusters.get("K8sClusterInfo") or [] if isinstance(clusters, dict) else []):
        owner, net = f"k8sCluster {c.get('id')}", c.get("network") or {}
        claim("vNet", [net.get("vNetId")], owner)
        claim("securityGroup", net.get("securityGroupIds") or [], owner)
        claim("sshKey", [g.get("sshKeyId") for g in c.get("k8sNodeGroupList") or []], owner)
    databases = api_request("GET", f"/ns/{ns_id}/resources/rdbms")
    for d in (databases.get("rdbms") or [] if isinstance(databases, dict) else []):
        owner = f"rdbms {d.get('id')}"
        claim("vNet", [d.get("vNetId")], owner)
        claim("securityGroup", d.get("securityGroupIds") or [], owner)
    return used


# Tool: Release resources
@tool(mutating=True)
def release_resources(ns_id: str, confirm: bool = False, dry_run: bool = False) -> Dict:
    """
    Delete the shared resources of a namespace that nothing uses any more.

    Terminating an Infra leaves its VNet, SecurityGroups and SSH keys behind; this removes
    those, and skips whatever a remaining Infra still references. A VNet that a managed
    Kubernetes cluster or database lives in blocks the release until that is deleted.

    Args:
        ns_id: Namespace ID
        confirm: Must be True to delete. Ask the user first.
        dry_run: True lists what would be deleted and deletes nothing

    Returns:
        Per-resource results with success and failure counts. Resources in use are not
        listed.
    """
    if not dry_run:
        guard = _require_confirmation(
            confirm, f"releasing unused shared resources in {ns_id}",
            "VNets, security groups and SSH keys that nothing uses are deleted; "
            "dry_run=True lists them first")
        if guard:
            return guard

    path = f"/ns/{ns_id}/sharedResources"
    in_use = _managed_service_resources(ns_id)
    if in_use:
        preview = api_request("DELETE", path, params={"dryRun": "true"})
        blocked = {}
        for entry in (preview.get("results") or [] if isinstance(preview, dict) else []):
            owner = in_use.get((entry.get("resourceType"), entry.get("resourceId")))
            if owner:
                entry["inUseBy"] = owner
                blocked[f"{entry.get('resourceType')} {entry.get('resourceId')}"] = owner
        if blocked and not dry_run:
            return {"error": {
                "message": f"nothing was released: {len(blocked)} candidate(s) are used by "
                           f"a managed cluster or database",
                "code": "CONFLICT", "blocked": blocked,
                "how_to_proceed": "delete those services first, or remove single leftovers "
                                  "with manage_resource(action='delete')",
            }}
        if dry_run:
            if blocked and isinstance(preview, dict):
                preview["_note"] = ("preview only; entries with inUseBy block the release "
                                    "until that service is deleted")
                return preview
            result = preview
        else:
            result = api_request("DELETE", path)
    else:
        result = api_request("DELETE", path, params={"dryRun": "true"} if dry_run else None)

    if isinstance(result, dict) and "error" not in result:
        result["_note"] = (
            "preview only - nothing was deleted; re-run with confirm=True to apply"
            if dry_run else
            "resources still referenced by a running Infra were skipped, not failed")
    return result


#####################################
# Infra Management (Multi-Cloud Infrastructure)
#####################################

# Tool: Get Infra list
@tool()
def get_infra_list(ns_id: str = DEFAULT_NAMESPACE, detail: str = "summary",
                   fields: Optional[str] = None, option: str = "summary") -> Dict:
    """
    List the Infras of a namespace.

    Args:
        ns_id: Namespace ID
        detail: "summary" (default) - one line per Infra: status, node counts, labels.
            "full" also lists each Infra's nodes.
        fields: Comma-separated fields to keep per Infra, e.g. "id,status"
        option: "summary" (default) - the server's stored view, cheap to read;
            "status" - the state of every node checked at the provider now;
            "id" - Infra ids only

    Returns:
        infra: the Infras. With option="id": output, the ids. get_infra() reads one in full.
    """
    detail = _validate_detail(detail)
    if option not in ("summary", "status", "id"):
        return {"error": f"unknown option {option!r}; expected one of summary, status, id"}
    result = api_request("GET", f"/ns/{ns_id}/infra",
                         params=None if option == "summary" else {"option": option})
    if not isinstance(result, dict) or "infra" not in result:
        return result
    rows = []
    for infra in result.get("infra") or []:
        row = _shape_infra(infra, detail)
        if detail != "full" and isinstance(row, dict) and isinstance(row.get("node"), list):
            # A listing answers "what is there"; get_infra() has the nodes.
            row["nodeCount"] = len(infra.get("node") or [])
            del row["node"]
            row.pop("nodeListCapped", None)
        rows.append(_project(row, fields))
    return {"infra": rows, "count": len(rows)}

# Tool: Get Infra details
@tool()
def get_infra(ns_id: str, infra_id: str, detail: str = "summary",
              fields: Optional[str] = None) -> Dict:
    """
    Get details of a specific Infra

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        detail: "summary" (default) or "full". Summary drops the per-node command
            history, which is where nearly all the bytes are.
        fields: Comma-separated top-level fields to keep, e.g. "id,status". Applied
            after detail.

    Returns:
        Infra information
    """
    detail = _validate_detail(detail)
    result = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}")
    if not isinstance(result, dict) or "error" in result:
        return result
    return _project(_redact_secrets(_shape_infra(result, detail)), fields)

#####################################
# Bastion Management
#####################################

# Tool: Bastion (SSH jump host) of a node
@tool(mutating=True, read_actions=("get",))
def manage_bastion(action: str, ns_id: str, infra_id: str,
                   target_node_id: Optional[str] = None,
                   bastion_node_id: Optional[str] = None,
                   bastion_infra_id: str = "", bastion_ns_id: str = "",
                   confirm: bool = False) -> Dict:
    """
    List, set or remove the bastion (SSH jump host) CB-Tumblebug reaches a node through.

    One is assigned automatically when a node has a public address, so most Infras never
    need this. It matters when a node's own address cannot be reached from CB-Tumblebug:
    for a VM inside a self-hosted OpenStack, the bastion is the machine hosting that
    OpenStack, the only one that can route to the tenant's floating IPs.

    Args:
        action: "get" (bastions of target_node_id) | "set" | "remove"
        ns_id: Namespace of the target node
        infra_id: Infra of the target node
        target_node_id: Node that needs to be reached, e.g. "g1-1". For get and set.
        bastion_node_id: Node to jump through. For set and remove.
        bastion_infra_id: Bastion's Infra, when different from infra_id
        bastion_ns_id: Bastion's namespace, when different from ns_id
        confirm: Must be True for "remove" - nodes behind the bastion lose their route

    Returns:
        get: bastionNodes; selfBastionOnly true means the node is dialled directly.
        set / remove: the server's message.
    """
    action = (action or "").strip().lower()
    if action not in ("get", "set", "remove"):
        return {"error": f"unknown action {action!r}; expected one of get, set, remove"}
    if action in ("get", "set") and not target_node_id:
        return {"error": f"target_node_id is required for action={action!r}"}
    if action in ("set", "remove") and not bastion_node_id:
        return {"error": f"bastion_node_id is required for action={action!r}"}

    if action == "get":
        result = api_request(
            "GET", f"/ns/{ns_id}/infra/{infra_id}/node/{target_node_id}/bastion")
        if not isinstance(result, list):
            return result
        self_only = (len(result) == 1 and result[0].get("nodeId") == target_node_id
                     and result[0].get("infraId") == infra_id)
        out = {"bastionNodes": result, "count": len(result), "selfBastionOnly": self_only}
        if self_only:
            out["note"] = ("the node is dialled directly; if commands time out, set a "
                           "bastion that can reach it")
        return out

    suffix = bastion_node_id
    if bastion_ns_id and bastion_infra_id:
        suffix = f"{bastion_ns_id}/{bastion_infra_id}/{bastion_node_id}"
    elif bastion_infra_id:
        suffix = f"{bastion_infra_id}/{bastion_node_id}"
    if action == "set":
        result = api_request(
            "PUT", f"/ns/{ns_id}/infra/{infra_id}/node/{target_node_id}/bastion/{suffix}")
        if isinstance(result, str):
            return {"message": result, "alreadyRegistered": "already exists" in result,
                    "next": "run a simple command (hostname) on the target to prove it is "
                            "reachable"}
        return result
    guard = _require_confirmation(
        confirm, f"removing bastion {bastion_node_id}",
        "remote commands to nodes served by this bastion fail until another is set")
    if guard:
        return guard
    result = api_request("DELETE", f"/ns/{ns_id}/infra/{infra_id}/bastion/{suffix}")
    return {"message": result} if isinstance(result, str) else result


# Tool: Node groups of an Infra and their node ids
@tool()
def get_nodegroups(ns_id: str, infra_id: str, nodegroup_id: Optional[str] = None) -> Dict:
    """
    List the node group ids of an Infra, or the node ids of one group.

    get_infra() shows nodes with their group, but caps the list on a large Infra; this
    gives every id.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        nodegroup_id: Return the node ids of this group instead of the group ids

    Returns:
        output: the ids.
    """
    path = f"/ns/{ns_id}/infra/{infra_id}/nodegroup"
    return api_request("GET", f"{path}/{nodegroup_id}" if nodegroup_id else path)


# Tool: Get Infra associated resources
@tool()
def get_infra_associated_resources(ns_id: str, infra_id: str) -> Dict:
    """
    Get associated resource IDs for a given Infra.
    This function returns all resources (VNet, SecurityGroup, SSHKey, etc.) that are used by the Infra.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID

    Returns:
        Associated resource information including:
        - vNetIds: List of VNet IDs used by the Infra
        - securityGroupIds: List of Security Group IDs
        - sshKeyIds: List of SSH Key IDs
        - imageIds: List of Image IDs
        - specIds: List of Spec IDs
        - And other resource associations
    """
    return api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/associatedResources")


_IMAGE_SUMMARY = ("cspImageName", "osType", "osDistribution", "osArchitecture", "isBasicImage",
                  "isGPUImage", "isKubernetesImage", "osDiskSizeGB", "creationDate")


def _shape_image(image, detail):
    if detail == "full" or not isinstance(image, dict):
        return image
    row = {k: image[k] for k in _IMAGE_SUMMARY if image.get(k) not in (None, "", False, 0, -1)}
    if image.get("imageStatus") not in (None, "", "Available"):
        row["imageStatus"] = image["imageStatus"]
    description = str(image.get("description") or "")
    if description and description != row.get("osDistribution"):
        row["description"] = description[:120]
    return row


# Tool: Search images
@tool()
def search_images(
    matched_spec_id: Optional[str] = None,
    os_type: Optional[str] = None,
    os_architecture: Optional[str] = None,
    provider_name: Optional[str] = None,
    region_name: Optional[str] = None,
    is_gpu_image: Optional[bool] = None,
    is_kubernetes_image: Optional[bool] = None,
    include_basic_image_only: Optional[bool] = None,
    detail_search_keys: Optional[List[str]] = None,
    max_results: Union[int, str] = 10,
    custom_images_of: Optional[str] = None,
    detail: str = "summary"
) -> Dict:
    """Search VM images, normally constrained to a chosen spec.

    Pass matched_spec_id with a spec id from recommend_vm_spec(): provider, region and
    architecture are then matched for you, which is the reliable way to get an image that
    boots on that spec. Use the returned cspImageName as imageId.
    Vocabulary: get_search_options(kind='image').

    Args:
        matched_spec_id: Spec id to match provider, region and architecture against
        os_type: OS name and version, space-separated terms all match, e.g. "ubuntu 22.04"
        os_architecture: "x86_64" or "arm64". Taken from the spec when matched_spec_id is given.
        provider_name: Provider to search, e.g. "aws" - an alternative to matched_spec_id
        region_name: Region to search, used with provider_name
        is_gpu_image: True for images with GPU drivers preinstalled - use it for
            accelerator nodes
        is_kubernetes_image: True for images usable as managed Kubernetes nodes
        include_basic_image_only: Plain OS images only. Defaults to True unless a GPU or
            Kubernetes image is asked for.
        detail_search_keys: Keywords matched against the image details, e.g. ["sql 2022"]
        max_results: Maximum images to return (default 10)
        custom_images_of: Namespace ID - search that namespace's custom images (snapshots)
            instead of the public catalog
        detail: "summary" (default) | "full" for the whole image records

    Returns:
        imageList with cspImageName, osType, osDistribution and osArchitecture, best match
        first. search_status "widened" means the exact filters matched nothing and were
        relaxed; "no_results" means nothing matched at all.
    """
    if include_basic_image_only is None and not (is_gpu_image or is_kubernetes_image
                                                 or custom_images_of or detail_search_keys):
        include_basic_image_only = True
    try:
        max_results = max(1, int(max_results))
    except (TypeError, ValueError):
        max_results = 10

    data: Dict[str, Any] = {"maxResults": max_results}
    if matched_spec_id:
        data["matchedSpecId"] = matched_spec_id
    if os_type:
        data["osType"] = os_type
    if os_architecture:
        data["osArchitecture"] = os_architecture
    elif not matched_spec_id:
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

    path = f"/ns/{custom_images_of or 'system'}/resources/searchImage"
    params = {"customImage": "true"} if custom_images_of else None
    response = api_request("POST", path, json_data=data, params=params)
    if not isinstance(response, dict) or "error" in response:
        return response
    images = response.get("imageList") or []
    status = None

    if not images and (provider_name or region_name or matched_spec_id) and not custom_images_of:
        # Some providers publish images with an empty osType and none marked basic, so
        # look again without those filters and match os_type against the whole descriptor.
        probe = {k: v for k, v in data.items() if k not in ("osType", "includeBasicImageOnly")}
        probe["includeBasicImageOnly"] = False
        probe["maxResults"] = max(max_results, 50)
        found = api_request("POST", path, json_data=probe)
        candidates = (found.get("imageList") or []) if isinstance(found, dict) else []
        wanted = str(os_type or "").lower().split()
        narrowed = [i for i in candidates if all(w in _image_os_text(i) for w in wanted)]
        images = (narrowed or candidates)[:max_results]
        if images:
            status = "widened"

    level = _validate_detail(detail)
    out: Dict[str, Any] = {"imageList": [_shape_image(i, level) for i in images],
                           "imageCount": len(images)}
    if status == "widened":
        out["search_status"] = "widened"
        out["note"] = ("the exact filters matched nothing, so basic-image-only was dropped and "
                       "os_type was matched against osDistribution too - check osDistribution "
                       "before using one")
    elif not images:
        out["search_status"] = "no_results"
        out["next"] = ("drop the version from os_type, try another os_type, or pick another "
                       "spec with recommend_vm_spec(); get_search_options(kind='image') lists "
                       "the valid values")
    return out


_SPEC_SUMMARY = ("id", "cspSpecName", "providerName", "regionName", "vCPU", "memoryGiB",
                 "costPerHour")


def _shape_spec(spec, detail="summary"):
    """A spec reduced to what a choice is made on."""
    if detail == "full" or not isinstance(spec, dict):
        return spec
    row = {k: spec.get(k) for k in _SPEC_SUMMARY}
    if spec.get("architecture") and spec["architecture"] != "x86_64":
        row["architecture"] = spec["architecture"]
    if spec.get("acceleratorModel"):
        # Count 0 with a model is a fractional GPU: acceleratorMemoryGB is the share you get.
        for key in ("acceleratorModel", "acceleratorType", "acceleratorCount",
                    "acceleratorMemoryGB"):
            if spec.get(key) is not None:
                row[key] = spec[key]
    status = spec.get("availabilityStatus")
    if status and status != "Available":
        row["availabilityStatus"] = status
        if spec.get("availabilityReason"):
            row["availabilityReason"] = spec["availabilityReason"]
    return row


def _demote_unpriced(specs):
    """Under a cost priority a spec with no published price must not rank as cheapest."""
    def unpriced(spec):
        try:
            return float(spec.get("costPerHour", -1)) < 0
        except (TypeError, ValueError):
            return True
    priced = [s for s in specs if not unpriced(s)]
    unknown = [s for s in specs if unpriced(s)]
    note = None
    if unknown and priced:
        note = (f"{len(unknown)} of {len(specs)} specs publish no price (costPerHour -1) and "
                f"are listed last; they are not necessarily cheap")
    elif unknown:
        where = sorted({f"{s.get('providerName')}/{s.get('regionName')}" for s in unknown})
        note = (f"no matching spec in {', '.join(where)} publishes a price, so none can be "
                f"ranked by cost - try another region or provider, or treat the spend as unknown")
    return priced + unknown, note


# Tool: Recommend VM spec
@tool()
def recommend_vm_spec(
    filter_policies: Optional[Dict[str, Any]] = None,
    priority_policy: str = "cost",
    limit: Union[int, str] = 10,
    latitude: Optional[float] = None,
    longitude: Optional[float] = None,
    detail: str = "summary"
) -> Dict:
    """Recommend VM specs matching resource, location or cost preferences.

    This is the only source of spec ids: pass the returned id unchanged to search_images()
    and create_infra_dynamic(). Filter vocabulary: get_search_options(kind='spec').

    Args:
        filter_policies: Field -> constraint. A range is {"min": .., "max": ..}, anything
            else is an exact match:
            {"vCPU": {"min": 2, "max": 8}, "memoryGiB": {"min": 4},
             "providerName": "aws", "regionName": "ap-northeast-2"}
            GPU: {"acceleratorModel": "nvidia", "acceleratorMemoryGB": {"min": 16}} - filter
            on the memory needed, since a card's name is also reported by fractional slices
            of it. architecture defaults to "x86_64".
        priority_policy: "cost" (default) | "location" | "performance" | "random"
        limit: Maximum specs to return (default 10)
        latitude: Reference latitude, required for priority_policy "location"
        longitude: Reference longitude, required for priority_policy "location"
        detail: "summary" (default) | "full" for every spec field

    Returns:
        specs: id, cspSpecName, providerName, regionName, vCPU, memoryGiB, costPerHour
        (-1 = price unknown), accelerator fields when present, and availabilityStatus when
        the spec is known to be sold out or over quota.
    """
    priority_policy = (priority_policy or "cost").strip().lower()
    if priority_policy not in ("cost", "location", "performance", "random"):
        return {"error": f"unknown priority_policy {priority_policy!r}; expected one of "
                         f"cost, location, performance, random"}
    if priority_policy == "location" and (latitude is None or longitude is None):
        return {"error": "latitude and longitude is required for priority_policy='location'"}
    try:
        limit = max(1, int(limit))
    except (TypeError, ValueError):
        limit = 10

    filters = dict(filter_policies or {})
    if not any(str(k).lower() == "architecture" for k in filters):
        filters["architecture"] = "x86_64"
    policies = []
    for metric, value in filters.items():
        conditions = []
        if isinstance(value, dict):
            if value.get("min") is not None:
                conditions.append({"operand": str(value["min"]), "operator": ">="})
            if value.get("max") is not None:
                conditions.append({"operand": str(value["max"]), "operator": "<="})
        elif value is not None:
            conditions.append({"operand": str(value), "operator": "=="})
        if conditions:
            policies.append({"metric": metric, "condition": conditions})

    priority: Dict[str, Any] = {"metric": priority_policy, "weight": 1.0}
    if priority_policy == "location":
        priority["parameter"] = [{"key": "coordinateClose", "val": [f"{latitude}/{longitude}"]}]
    result = api_request("POST", "/recommendSpec", json_data={
        "filter": {"policy": policies}, "limit": limit, "priority": {"policy": [priority]}})
    if isinstance(result, dict):
        if "error" in result:
            return result
        # An error from this endpoint can arrive as a bare {"message": ...}
        return {"error": result.get("message") or "unexpected answer from recommendSpec"}
    specs = result if isinstance(result, list) else []

    note = None
    if priority_policy == "cost":
        specs, note = _demote_unpriced(specs)
    level = _validate_detail(detail)
    out: Dict[str, Any] = {"specs": [_shape_spec(s, level) for s in specs], "count": len(specs)}
    if note:
        out["pricing_note"] = note
    if not specs:
        out["next"] = ("nothing matched: loosen the filters, or check the vocabulary with "
                       "get_search_options(kind='spec')")
    return out

# The request both the review and the creation send (model.InfraDynamicReq)
def _infra_request_body(name, node_groups, description="", system_label="", label=None,
                        post_script=None, init_script=None, policy_on_partial_failure="",
                        vnet_template_id="", sg_template_id=""):
    body: Dict[str, Any] = {"name": name, "nodeGroups": node_groups}
    if description:
        body["description"] = description
    if policy_on_partial_failure:
        body["policyOnPartialFailure"] = policy_on_partial_failure
    if system_label:
        body["systemLabel"] = system_label
    if label:
        body["label"] = label
    if post_script and post_script.get("phases"):
        body["postScript"] = post_script
    if init_script:
        body["initScript"] = init_script
    if vnet_template_id:
        body["vNetTemplateId"] = vnet_template_id
    if sg_template_id:
        body["sgTemplateId"] = sg_template_id
    return body


def _check_node_groups(node_groups):
    """Refuse a plan the server would reject for a missing spec or image."""
    if not isinstance(node_groups, list) or not node_groups:
        return {"error": "node_groups is required: a non-empty list of node group objects"}
    for i, group in enumerate(node_groups):
        if not isinstance(group, dict):
            return {"error": f"node_groups[{i}] is required to be an object"}
        if not group.get("specId"):
            return {"error": f"node_groups[{i}].specId is required; take the id from "
                             f"recommend_vm_spec()"}
        if not group.get("imageId"):
            return {"error": f"node_groups[{i}].imageId is required; take cspImageName from "
                             f"search_images(matched_spec_id={group['specId']!r})"}
    return None


_REVIEW_VALIDATION_KEYS = ("resourceId", "isAvailable", "status", "message", "quota")


def _slim_review(result):
    """A review reduced to what decides the plan: verdicts, cost, quota and findings."""
    out = {k: result[k] for k in ("overallStatus", "overallMessage", "creationViable",
                                  "estimatedCost", "policyOnPartialFailure")
           if result.get(k) not in (None, "")}
    reviews, nodes = [], 0
    for review in result.get("nodeReviews") or []:
        try:
            nodes += max(1, int(review.get("nodeGroupSize") or 1))
        except (TypeError, ValueError):
            nodes += 1
        row = {k: review[k] for k in ("nodeName", "nodeGroupSize", "status", "canCreate",
                                      "estimatedCost", "connectionName")
               if review.get(k) not in (None, "")}
        for key in ("specValidation", "imageValidation"):
            checked = review.get(key)
            if isinstance(checked, dict):
                row[key] = {k: checked[k] for k in _REVIEW_VALIDATION_KEYS
                            if checked.get(k) not in (None, "", {})}
        for key in ("errors", "warnings", "info"):
            if review.get(key):
                row[key] = review[key]
        reviews.append(row)
    out["nodeReviews"] = reviews
    # totalNodeCount counts machines; the server's own figure counts node groups.
    out["nodeGroupCount"], out["totalNodeCount"] = len(reviews), nodes
    if result.get("recommendations"):
        out["recommendations"] = result["recommendations"]
    summary = result.get("resourceSummary") or {}
    for key in ("providerNames", "regionNames"):
        if summary.get(key):
            out[key] = summary[key]
    return out


# Tool: Review Infra Dynamic Request (Pre-validation)
@tool()
def review_infra_dynamic_request(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_script: Optional[Dict] = None,
    hold: bool = False,
    policy_on_partial_failure: str = "continue",
    vnet_template_id: str = "",
    sg_template_id: str = "",
    init_script: Optional[Dict] = None,
    detail: str = "summary"
) -> Dict:
    """Validate an Infra configuration and estimate its cost before creating it.

    Run this before create_infra_dynamic(), with the same arguments. It resolves every spec
    and image against the CSP and reports availability, quota headroom, past failure rate,
    init script size and hourly cost. Nothing is created.
    Details: get_usage_guide('provision').

    Args:
        ns_id: Namespace ID
        name: Infra name
        node_groups: Node group objects - see create_infra_dynamic()
        description: Infra description
        policy_on_partial_failure: "continue" (default) | "rollback" | "refine"
        vnet_template_id: VNet template ID, Infra-level default
        sg_template_id: Security group template ID, Infra-level default
        system_label: Reserved system label; leave empty unless told otherwise
        label: Key-value pairs applied to the Infra
        post_script: Bootstrap to run once the nodes are up: {"phases": [...]}
        init_script: First-boot script for every node - see create_infra_dynamic()
        hold: Review the request as one that will be created on hold
        detail: "summary" (default) | "full" for the server's whole answer

    Returns:
        overallStatus ("Ready" | "Warning" | "Error"), creationViable, estimatedCost,
        totalNodeCount, and per node group in nodeReviews: canCreate, the spec and image
        checks (specValidation.quota.fitCount is how many more nodes of that spec the
        account can start), errors, warnings and info. `next` says what to do.
    """
    groups = _normalize_node_groups(node_groups)
    problem = _check_node_groups(groups)
    if problem:
        return problem
    body = _infra_request_body(name, groups, description, system_label, label, post_script,
                               init_script, policy_on_partial_failure, vnet_template_id,
                               sg_template_id)
    result = api_request("POST", f"/ns/{ns_id}/infraDynamicReview", json_data=body,
                         params={"option": "hold"} if hold else None)
    if not isinstance(result, dict) or "error" in result:
        return result
    if _validate_detail(detail) != "full":
        result = _slim_review(result)
    if result.get("creationViable"):
        result["next"] = ("show the user the node groups and estimatedCost; when they agree, "
                          "call create_infra_dynamic() with the same arguments plus "
                          "force_create=True, confirm=True")
    else:
        result["next"] = ("fix what nodeReviews[].errors report and review again; "
                          "recommend_alternative_node_config() finds a substitute for a spec "
                          "that cannot be provisioned")
    return result


# Tool: Create Infra dynamically
@tool(mutating=True, additive=True)
def create_infra_dynamic(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_script: Optional[Dict] = None,
    post_script_async: bool = False,
    hold: bool = False,
    force_create: bool = False,
    policy_on_partial_failure: str = "continue",
    vnet_template_id: str = "",
    sg_template_id: str = "",
    init_script: Optional[Dict] = None,
    budget_ack: Optional[str] = None,
    confirm: bool = False
) -> Dict:
    """Create a Multi-Cloud Infrastructure.

    Review first: review_infra_dynamic_request() with the same arguments, then this with
    force_create=True and confirm=True. Without them it refuses and says what is missing.
    specId and imageId must be copied verbatim from recommend_vm_spec() and
    search_images(matched_spec_id=...). Worked example: get_usage_guide('provision').

    Args:
        ns_id: Namespace ID
        name: Infra name
        node_groups: One object per node group: specId and imageId (required), name,
            nodeGroupSize (default 1), label ({"role": "web"} lets label_selector reach the
            group later), rootDiskType, rootDiskSize (GB), zone, connectionName,
            distributeSubnets, vNetTemplateId, sgTemplateId, initScript.
        description: Infra description
        system_label: Reserved system label; leave empty unless told otherwise
        label: Key-value pairs applied to the Infra
        post_script: Commands run over SSH once the nodes are up:
            {"phases": [{"nodeGroupId": "web", "command": ["..."]}]}. A phase targets one of
            nodeGroupId | nodeId | labelSelector, or every node when none is given.
        post_script_async: True returns when the nodes are up and runs post_script in the
            background; poll get_infra() until postScriptStatus is terminal.
        init_script: First-boot script run by the node itself, no SSH:
            {"templateId": "initscript-nginx", "env": {...}}, {"script": "#!/bin/bash\\n..."}
            or {"mode": "none"}. Details: get_usage_guide('initscript').
        hold: Register the Infra but do not provision it yet. control_infra() with
            action "continue" starts it, "withdraw" cancels it.
        force_create: Set True once the review has passed
        confirm: Set True once a person has seen the plan and agreed to it
        policy_on_partial_failure: "continue" (default; keep what succeeded) | "rollback"
            (remove everything) | "refine" (remove only the failed nodes)
        vnet_template_id: VNet template ID, Infra-level default
        sg_template_id: Security group template ID, Infra-level default
        budget_ack: Approval id from request_budget_increase(), to pass a spend limit once

    Returns:
        The created Infra: status, statusCount and its nodes with their addresses.
    """
    groups = _normalize_node_groups(node_groups)
    problem = _check_node_groups(groups)
    if problem:
        return problem
    if not force_create:
        return {"error": {
            "message": "Infra creation requires prior validation: run "
                       "review_infra_dynamic_request() with the same arguments first",
            "code": "REVIEW_REQUIRED",
            "how_to_proceed": ("review, fix what it reports, show the user the result, then "
                               "repeat this call with force_create=True, confirm=True"),
        }}
    unconfirmed = _confirm_creation_plan(groups, name, confirm)
    if unconfirmed:
        return unconfirmed
    over_budget = _budget_refusal(groups, budget_ack)
    if over_budget:
        return over_budget

    body = _infra_request_body(name, groups, description, system_label, label, post_script,
                               init_script, policy_on_partial_failure, vnet_template_id,
                               sg_template_id)
    params = {}
    if hold:
        params["option"] = "hold"
    if post_script_async and body.get("postScript"):
        params["postScriptAsync"] = "true"
    # Provisioning answers when every node is up, which takes minutes for a large Infra.
    result = api_request("POST", f"/ns/{ns_id}/infraDynamic", json_data=body,
                         params=params or None, timeout_override=(60, 3600))
    if not isinstance(result, dict) or "error" in result:
        return result

    _record_spend(groups)
    result = _shape_infra(result, "summary")
    if hold:
        result["next"] = ("held before provisioning: control_infra(action='continue') starts "
                          "it, 'withdraw' cancels it")
    elif result.get("creationErrors") or (result.get("statusCount") or {}).get("countFailed"):
        result["next"] = ("some nodes failed: diagnose_infra() "
                          "explains why and what can be retried")
    return result

# Tool: Review Spec-Image Pair
@tool()
def review_spec_image_pair(spec_id: str, image_id: str, root_disk_type: str = "",
                           zone: str = "") -> Dict:
    """
    Check one spec and image pair without building a whole Infra request.

    Much faster than review_infra_dynamic_request(), so several candidates can be tested
    before committing to a plan.

    Args:
        spec_id: Spec id, e.g. "aws+ap-northeast-2+t3.nano"
        image_id: CSP image id (cspImageName from search_images)
        root_disk_type: Root disk type to check with the pair
        zone: Zone to check availability in

    Returns:
        isValid, message, estimatedCost, availability (status and per-zone detail),
        suggestedZone, suggestedSystemDisk, and info, warnings and errors.
    """
    data = {"specId": spec_id, "imageId": image_id}
    if root_disk_type:
        data["rootDiskType"] = root_disk_type
    if zone:
        data["zone"] = zone
    result = api_request("POST", "/specImagePairReview", json_data=data)
    if isinstance(result, dict) and "error" not in result:
        # The full spec and image records are most of the answer and add nothing to it.
        if isinstance(result.get("specDetails"), dict):
            result["specDetails"] = _shape_spec(result["specDetails"])
        if isinstance(result.get("imageDetails"), dict):
            result["imageDetails"] = _shape_image(result["imageDetails"], "summary")
    return result

# The request both the node group review and the addition send
def _nodegroup_request_body(spec_id, image_id, name, node_group_size, description,
                            root_disk_type, root_disk_size, connection_name, zone,
                            post_script, init_script, vnet_template_id="", sg_template_id="",
                            label=None, distribute_subnets=False):
    body: Dict[str, Any] = {"specId": spec_id, "imageId": image_id,
                            "nodeGroupSize": int(node_group_size)}
    optional = {"name": name, "description": description, "rootDiskType": root_disk_type,
                "rootDiskSize": int(root_disk_size or 0), "connectionName": connection_name,
                "zone": zone, "vNetTemplateId": vnet_template_id,
                "sgTemplateId": sg_template_id, "label": label, "initScript": init_script}
    body.update({k: v for k, v in optional.items() if v})
    if distribute_subnets:
        body["distributeSubnets"] = True
    if post_script and post_script.get("phases"):
        body["postScript"] = post_script
    return body


# Tool: Add NodeGroup to existing Infra dynamically
@tool(mutating=True, additive=True)
def add_nodegroup_dynamic(
    ns_id: str,
    infra_id: str,
    spec_id: str,
    image_id: str,
    name: str = "",
    node_group_size: int = 1,
    description: str = "",
    root_disk_type: str = "",
    root_disk_size: int = 0,
    connection_name: str = "",
    zone: str = "",
    vnet_template_id: str = "",
    sg_template_id: str = "",
    label: Optional[Dict[str, str]] = None,
    post_script: Optional[Dict] = None,
    post_script_async: bool = False,
    init_script: Optional[Dict] = None,
    distribute_subnets: bool = False,
    budget_ack: Optional[str] = None,
    confirm: bool = False
) -> Dict:
    """
    Add a new NodeGroup to an existing Infra.

    The new nodes can use another spec, image, region or provider; the existing ones are
    not touched. To add more nodes to a group that exists, use scale_out_nodegroup().
    Check the group with review_nodegroup_dynamic() first.

    Args:
        ns_id: Namespace ID
        infra_id: Infra to add the node group to
        spec_id: Spec id from recommend_vm_spec()
        image_id: cspImageName from search_images(matched_spec_id=spec_id)
        name: Node group name; its nodes are named "<name>-1", "<name>-2", ...
        node_group_size: Number of nodes (default 1)
        description: Node group description
        root_disk_type: Root disk type; "" for the provider default
        root_disk_size: Root disk size in GB; 0 for the provider default
        connection_name: Connection to use instead of the spec's own
        zone: Availability zone, e.g. "ap-northeast-2a"
        vnet_template_id: VNet template ID
        sg_template_id: Security group template ID
        label: Key-value pairs; label_selector on a remote command targets them
        post_script: Commands run over SSH on the new nodes once they are up:
            {"phases": [{"command": ["..."]}]}
        post_script_async: True returns when the nodes are up and runs post_script in the
            background
        init_script: First-boot script for the new nodes - see create_infra_dynamic()
        distribute_subnets: Spread the new nodes across the VNet's subnets (zones)
        budget_ack: Approval id from request_budget_increase(), to pass a spend limit once
        confirm: Set True once a person has agreed to the added machines and their cost

    Returns:
        The Infra after the addition; addedNodeIds lists the new nodes.
    """
    body = _nodegroup_request_body(spec_id, image_id, name, node_group_size, description,
                                   root_disk_type, root_disk_size, connection_name, zone,
                                   post_script, init_script, vnet_template_id, sg_template_id,
                                   label, distribute_subnets)
    plan = [{"name": name or "-", "nodeGroupSize": node_group_size,
             "specId": spec_id, "imageId": image_id}]
    unconfirmed = _confirm_creation_plan(plan, f"node group {name or '(unnamed)'} in {infra_id}",
                                         confirm)
    if unconfirmed:
        return unconfirmed
    over_budget = _budget_refusal(plan, budget_ack)
    if over_budget:
        return over_budget

    params = {"postScriptAsync": "true"} if post_script_async and "postScript" in body else None
    result = api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/nodeGroupDynamic",
                         json_data=body, params=params, timeout_override=(60, 3600))
    if not isinstance(result, dict) or "error" in result:
        return result
    _record_spend(plan)
    added = result.get("newNodeList") or [
        n.get("id") for n in (result.get("node") or []) if name and n.get("nodeGroupId") == name]
    result = _shape_infra(result, "summary")
    if added:
        result["addedNodeIds"] = added
    return result


# Tool: Review NodeGroup Dynamic Request
@tool()
def review_nodegroup_dynamic(
    ns_id: str,
    infra_id: str,
    spec_id: str,
    image_id: str,
    name: str = "",
    node_group_size: int = 1,
    description: str = "",
    root_disk_type: str = "",
    root_disk_size: int = 0,
    connection_name: str = "",
    zone: str = "",
    post_script: Optional[Dict] = None,
    init_script: Optional[Dict] = None
) -> Dict:
    """
    Check a node group before adding it to an existing Infra with add_nodegroup_dynamic().

    The same checks as review_infra_dynamic_request(), for one group. Nothing is created.

    Args:
        ns_id: Namespace ID
        infra_id: Infra the node group would join
        spec_id: Spec id from recommend_vm_spec()
        image_id: cspImageName from search_images(matched_spec_id=spec_id)
        name: Node group name
        node_group_size: Number of nodes (default 1)
        description: Node group description
        root_disk_type: Root disk type; "" for the provider default
        root_disk_size: Root disk size in GB; 0 for the provider default
        connection_name: Connection to use instead of the spec's own
        zone: Availability zone
        post_script: Bootstrap to validate (same shape as add_nodegroup_dynamic)
        init_script: First-boot script to validate (same shape as add_nodegroup_dynamic)

    Returns:
        canCreate, status, estimatedCost, the spec and image checks with quota, and
        errors, warnings and info.
    """
    body = _nodegroup_request_body(spec_id, image_id, name, node_group_size, description,
                                   root_disk_type, root_disk_size, connection_name, zone,
                                   post_script, init_script)
    return api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/nodeGroupDynamicReview",
                       json_data=body)

# Tool: Delete Infra
@tool(mutating=True)
def terminate_infra(ns_id: str, infra_id: str, confirm: bool = False) -> Dict:
    """
    Terminate every node of an Infra at the provider and delete the Infra.

    Irreversible. Removing only the CB-Tumblebug record is deliberately not offered: it
    leaves billable instances running with nothing tracking them. If the API reports that
    termination is still in progress, wait and call again.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        confirm: Must be True to proceed. Ask the user first.

    Returns:
        The deletion result, and leftover_resources: the security groups, key pair and
        VNet the Infra used, which outlive it.
    """
    guard = _require_confirmation(
        confirm, f"terminating Infra {infra_id}",
        "every node in this Infra is terminated at the provider; this cannot be undone")
    if guard:
        return guard
    associated = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/associatedResources")
    # option=terminate ends the instances before the records go; option=force is not offered
    result = api_request("DELETE", f"/ns/{ns_id}/infra/{infra_id}",
                         params={"option": "terminate"}, timeout_override=(60, 1800))
    if not isinstance(result, dict) or "error" in result:
        return result
    if isinstance(associated, dict) and "error" not in associated:
        result["leftover_resources"] = {k: associated.get(k) or [] for k in
                                        ("vNetIds", "securityGroupIds", "sshKeyIds")}
    result["next"] = ("release_resources(ns_id, dry_run=True) lists what nothing uses any "
                      "more; confirm=True removes only those")
    return result


_INFRA_ACTIONS = ("suspend", "resume", "reboot", "terminate", "refine", "continue",
                  "withdraw", "reconcile", "abort")


# Tool: Control Infra
@tool(mutating=True)
def control_infra(ns_id: str, infra_id: str, action: str, force: bool = False,
                  confirm: bool = False) -> Dict:
    """
    Apply a lifecycle action to every node of an Infra.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        action: "suspend" | "resume" | "reboot" | "terminate" (ends the instances, keeps
            the Infra record) | "refine" (drop Failed nodes from the Infra) |
            "continue" / "withdraw" (start or cancel an Infra created with hold) |
            "reconcile" / "abort" (recover an Infra stuck mid-provisioning: adopt what
            exists at the CSP, or terminate what is unfinished)
        force: Apply even when the Infra is in an in-progress state
        confirm: Must be True for "terminate" and "abort". Ask the user first.

    Returns:
        The action result; nodeResults lists the nodes it failed on.
    """
    action = (action or "").strip().lower()
    if action not in _INFRA_ACTIONS:
        return {"error": f"unknown action {action!r}; expected one of {', '.join(_INFRA_ACTIONS)}"}
    if action in ("terminate", "abort"):
        guard = _require_confirmation(
            confirm, f"{action} on Infra {infra_id}",
            "instances are terminated at the provider; this cannot be undone")
        if guard:
            return guard
    params = {"action": action}
    if force:
        params["force"] = "true"
    return api_request("GET", f"/ns/{ns_id}/control/infra/{infra_id}", params=params,
                       timeout_override=(60, 1800))

#####################################
# Node-level control and scaling
#####################################

# Tool: Control a single node
@tool(mutating=True)
def control_node(ns_id: str, infra_id: str, node_id: str, action: str,
                 force: bool = False, confirm: bool = False) -> Dict:
    """
    Control one node of an Infra: suspend, resume, reboot, terminate or delete.

    control_infra() acts on every node; this acts on one. "terminate" ends the instance at
    the provider and keeps the node record as Terminated. "delete" terminates the instance
    and then removes the node from the Infra; if the provider has not confirmed the
    termination yet it answers "retry shortly" and nothing is removed.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Node ID, e.g. "g1-1"
        action: "suspend" | "resume" | "reboot" | "terminate" | "delete"
        force: For suspend, resume, reboot and terminate: proceed even when the node is
            in an in-progress state. Ignored for "delete".
        confirm: Must be True for "terminate" and "delete". Ask the user first.

    Returns:
        The node's action result (success, status, message).
    """
    action = (action or "").strip().lower()
    if action not in ("suspend", "resume", "reboot", "terminate", "delete"):
        return {"error": f"unknown action {action!r}; expected one of suspend, resume, "
                         f"reboot, terminate, delete"}
    if action in ("terminate", "delete"):
        guard = _require_confirmation(
            confirm, f"{'terminating' if action == 'terminate' else 'deleting'} node {node_id}",
            "the instance is terminated at the provider; this cannot be undone")
        if guard:
            return guard
    if action == "delete":
        # The server's option=force skips the termination and drops only the record, which
        # leaves a billable instance nothing tracks. It is deliberately not offered here.
        result = api_request("DELETE", f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}")
        if isinstance(result, dict) and "error" in result:
            # The termination is issued before the record goes; when the provider has not
            # confirmed it within the call, the record stays and a second call finishes it.
            view = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}",
                               params={"option": "status"})
            state = view.get("status") if isinstance(view, dict) else None
            if isinstance(state, dict):
                state = state.get("status")
            if state in ("Terminating", "Terminated"):
                return {"error": {
                    "message": f"node {node_id} is {state} at the provider and its record is "
                               f"still listed; nothing else was removed",
                    "code": "TIMEOUT",
                    "how_to_proceed": "call again in a few seconds to remove the record",
                }}
        return result
    params = {"action": action}
    if force:
        params["force"] = "true"
    return api_request("GET", f"/ns/{ns_id}/control/infra/{infra_id}/node/{node_id}",
                       params=params)


# Tool: Add nodes to an existing NodeGroup
@tool(mutating=True, additive=True)
def scale_out_nodegroup(ns_id: str, infra_id: str, nodegroup_id: str, add_count: int = 1,
                        budget_ack: Optional[str] = None, confirm: bool = False) -> Dict:
    """
    Add nodes to an existing NodeGroup, using the group's own spec, image and settings.

    For nodes of a different spec or image, add_nodegroup_dynamic() creates a new group.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        nodegroup_id: NodeGroup to grow, e.g. "g1"
        add_count: Number of nodes to add (default 1)
        budget_ack: Approval id from request_budget_increase(), to proceed past a spend limit
        confirm: Confirms a person agreed to the added machines and their cost.

    Returns:
        The Infra after the addition; addedNodeIds lists the new nodes.
    """
    try:
        add_count = int(add_count)
    except (TypeError, ValueError):
        return {"error": "add_count must be a positive integer"}
    if add_count < 1:
        return {"error": "add_count must be a positive integer"}

    # Price the addition from a node already in the group.
    members = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/nodegroup/{nodegroup_id}")
    if not isinstance(members, dict) or "error" in members:
        return members
    node_ids = members.get("output") or []
    if not node_ids:
        return {"error": f"NodeGroup {nodegroup_id!r} does not exist in Infra {infra_id!r}"}
    sample = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/node/{node_ids[0]}")
    plan = [{"name": nodegroup_id, "nodeGroupSize": add_count,
             "specId": (sample or {}).get("specId"), "imageId": (sample or {}).get("imageId")}]
    unconfirmed = _confirm_creation_plan(plan, f"{add_count} more node(s) in {nodegroup_id}", confirm)
    if unconfirmed:
        return unconfirmed
    over_budget = _budget_refusal(plan, budget_ack)
    if over_budget:
        return over_budget

    result = api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/nodegroup/{nodegroup_id}",
                         json_data={"numNodesToAdd": add_count}, timeout_override=(60, 3600))
    if not isinstance(result, dict) or "error" in result:
        return result
    _record_spend(plan)
    before = set(node_ids)
    added = [n.get("id") for n in (result.get("node") or [])
             if n.get("nodeGroupId") == nodegroup_id and n.get("id") not in before]
    result = _shape_infra(result, "summary")
    result["addedNodeIds"] = added
    return result


#####################################
# Failure recovery
#####################################

# Tool: Retry failed nodes
@tool(mutating=True, additive=True)
def retry_failed_nodes(ns_id: str, infra_id: str, dry_run: bool = True,
                       node_ids: Optional[List[str]] = None, attempts_per_node: int = 0,
                       interval_seconds: int = 0, zone: Optional[str] = None,
                       assume_resolved: bool = False, keep_failed_nodes: bool = False,
                       confirm: bool = False) -> Dict:
    """
    Re-create the Failed nodes of an Infra with their original configuration.

    The server classifies each failure first: a capacity or transient fault is worth
    retrying, a quota or configuration fault is not until someone fixes it. dry_run=True
    (the default) returns that plan - which nodes would be retried, why the others would
    not, and the hourly cost - without creating anything.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        dry_run: True (default) previews the plan. False performs the retry.
        node_ids: Failed nodes to retry. Omit for every node the server considers retriable.
        attempts_per_node: Tries per node before giving up (0 = server default). Capacity
            comes and goes, so a few attempts can succeed where one fails.
        interval_seconds: Pause between attempts on one node (0 = server default 30, max 600)
        zone: Place the replacements in this zone instead of the original one. Applies to
            the nodes named in node_ids.
        assume_resolved: Retry nodes the classifier ruled out, asserting the blocking
            condition (a quota, for instance) has been fixed. Applies to node_ids.
        keep_failed_nodes: Keep the original Failed records after a successful replacement
        confirm: Required when dry_run=False - the retry creates billable machines.

    Returns:
        As a preview: retriableCount, notRetriableCount, costPerHourIfAll and a plan per
        node (action, reason, escalation). Otherwise succeededCount, failedCount and
        per-node results.
    """
    body: Dict[str, Any] = {"targets": []}
    for node_id in node_ids or []:
        target: Dict[str, Any] = {"nodeId": node_id}
        if zone:
            target["zone"] = zone
        if assume_resolved:
            target["assumeResolved"] = True
        body["targets"].append(target)
    if (zone or assume_resolved) and not node_ids:
        return {"error": "node_ids is required when zone or assume_resolved is given"}
    if attempts_per_node:
        body["attemptsPerNode"] = int(attempts_per_node)
    if interval_seconds:
        body["intervalSeconds"] = int(interval_seconds)
    if keep_failed_nodes:
        body["keepFailedNodes"] = True

    if dry_run:
        review = api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/retryFailedNodesReview",
                             json_data=body)
        if not isinstance(review, dict) or "error" in review:
            return review
        plans = review.get("plans") or []
        keep = ("nodeId", "nodeGroupId", "action", "reason", "escalation", "provider", "region",
                "zone", "specId", "costPerHour", "siblingZone", "siblingRunningCount",
                "assumeResolvedHelps")
        out = {k: v for k, v in review.items() if k != "plans"}
        out["plans"] = [{k: p[k] for k in keep if p.get(k) not in (None, "", 0, False)}
                        for p in plans[:NODE_LIST_LIMIT]]
        if len(plans) > NODE_LIST_LIMIT:
            out["plansCapped"] = {"total": len(plans), "shown": NODE_LIST_LIMIT}
        if review.get("retriableCount"):
            out["next"] = "show the plan to the user, then call again with dry_run=False, confirm=True"
        return out

    guard = _require_confirmation(
        confirm, f"retrying failed nodes of {infra_id}",
        "replacement machines are created and billed")
    if guard:
        return guard
    result = api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/retryFailedNodes",
                         json_data=body, timeout_override=(60, 3600))
    if isinstance(result, dict) and isinstance(result.get("results"), list) \
            and len(result["results"]) > NODE_LIST_LIMIT:
        # Keep the nodes that still failed; the counts cover the rest.
        rows = result["results"]
        rows = ([r for r in rows if not r.get("succeeded")] + [r for r in rows if r.get("succeeded")])
        result = dict(result)
        result["results"] = rows[:NODE_LIST_LIMIT]
        result["resultsCapped"] = True
    return result


# Tool: Find a substitute spec and image elsewhere
@tool()
def recommend_alternative_node_config(source_spec_id: str, target_provider: str,
                                      target_region: Optional[str] = None,
                                      source_image_id: Optional[str] = None,
                                      os_type: Optional[str] = None,
                                      limit: int = 3) -> Dict:
    """
    Find the closest spec, with matching images, in another provider or region.

    Use it when a spec cannot be provisioned where it was planned - no stock, no quota - or
    to move a node group to another cloud. Each candidate carries a spec id and image ids
    that can go straight into review_infra_dynamic_request().

    Args:
        source_spec_id: Spec to match, exact id from recommend_vm_spec()
        target_provider: Provider to search, e.g. "gcp"
        target_region: Limit the search to one region (default: every region of the provider)
        source_image_id: Image used with the source spec; its OS decides the image search
        os_type: OS to search images for, e.g. "ubuntu 22.04", overriding the source image's
        limit: Number of candidate specs (default 3)

    Returns:
        candidates, each with the spec, how it compares with the source, and images.
    """
    body: Dict[str, Any] = {"sourceSpecId": source_spec_id,
                            "targetProviderName": target_provider,
                            "specCandidateLimit": max(1, int(limit or 3)),
                            "imageAlternativeLimit": 2}
    if target_region:
        body["targetRegionName"] = target_region
    if source_image_id:
        body["sourceImageId"] = source_image_id
    if os_type:
        body["osType"] = os_type
    result = api_request("POST", "/recommendAlternativeNodeConfig", json_data=body)
    if not isinstance(result, dict) or "error" in result:
        return result

    def spec_row(spec):
        if not isinstance(spec, dict):
            return spec
        return {k: spec[k] for k in ("id", "providerName", "regionName", "cspSpecName", "vCPU",
                                     "memoryGiB", "costPerHour", "architecture",
                                     "acceleratorModel", "acceleratorCount",
                                     "acceleratorMemoryGB") if spec.get(k) not in (None, "", 0)}

    def image_row(image):
        if not isinstance(image, dict):
            return image
        return {k: image[k] for k in ("cspImageName", "osType", "osArchitecture",
                                      "isBasicImage", "description") if image.get(k) is not None}

    out: Dict[str, Any] = {"sourceSpec": spec_row(result.get("sourceSpec")), "candidates": []}
    for cand in result.get("candidates") or []:
        row = {}
        for key, value in cand.items():
            if isinstance(value, dict) and "cspSpecName" in value:
                row[key] = spec_row(value)
            elif isinstance(value, list) and value and isinstance(value[0], dict) \
                    and "cspImageName" in value[0]:
                row[key] = [image_row(v) for v in value]
            elif isinstance(value, dict) and "cspImageName" in value:
                row[key] = image_row(value)
            else:
                row[key] = value
        out["candidates"].append(row)
    return out


# Tool: Zones and disks usable with a spec
@tool()
def get_spec_options(spec_id: str, image_id: Optional[str] = None,
                     refresh: bool = False) -> Dict:
    """
    What a node group may ask for with this spec: zones, root disk types and sizes.

    These are the valid values for zone, rootDiskType and rootDiskSize in a node group,
    plus whether the provider can start the spec right now.

    Args:
        spec_id: Exact spec id from recommend_vm_spec()
        image_id: Image the node will boot; adds the image's minimum root disk size
        refresh: True asks the provider now instead of using the cached availability

    Returns:
        availability (status: Available | SoldOut | QuotaExceeded | Unsupported |
        Unverified, with availableZones, unavailableZones and quota), and disk
        (defaultRootDiskType, rootDiskSelectable, and per disk type its size range).
    """
    spec = api_request("GET", f"/ns/system/resources/spec/{spec_id}",
                       params={"forceRefresh": "true"} if refresh else None)
    if not isinstance(spec, dict) or "error" in spec:
        return spec
    availability = {
        "status": spec.get("availabilityStatus") or "Unverified",
        "reason": spec.get("availabilityReason"),
        "availableZones": spec.get("availableZones"),
        "unavailableZones": spec.get("unavailableZones"),
        "quota": spec.get("availabilityQuota"),
        "checkedAt": spec.get("availabilityCheckedAt"),
    }
    out: Dict[str, Any] = {
        "spec": _shape_spec(spec),
        "availability": {k: v for k, v in availability.items() if v not in (None, "", [], {})},
    }
    disk = api_request("GET", f"/ns/system/resources/spec/{spec_id}/diskOptions",
                       params={"imageId": image_id} if image_id else None)
    if not isinstance(disk, dict) or "error" in disk:
        out["disk"] = disk
        return out
    types = [t for t in disk.get("diskTypes") or [] if isinstance(t, dict)]
    usable = [t for t in types if t.get("rootDisk") and t.get("available") is not False]
    out["disk"] = {k: disk[k] for k in ("supported", "rootDiskSelectable", "defaultRootDiskType",
                                        "imageMinRootDiskSizeGB", "note", "liveCheckHint")
                   if disk.get(k) not in (None, "", [], {})}
    out["disk"]["rootDiskTypes"] = [
        {k: t[k] for k in ("diskType", "description", "rootDiskSizeGB", "note") if t.get(k)}
        for t in usable]
    unusable = [t.get("diskType") for t in types if t not in usable]
    if unusable:
        out["disk"]["notUsableAsRootDisk"] = unusable
    return out


#####################################
# Init script (first-boot) and platform services
#####################################

_INIT_SCRIPT_STATES = ("initializing", "ok", "fail", "pending", "noReport", "notApplicable")


# Tool: Init script results of an Infra
@tool()
def get_init_script_status(ns_id: str, infra_id: str, detail: str = "summary",
                           status: Optional[str] = None) -> Dict:
    """
    First-boot (init script) results reported by the nodes of an Infra.

    The init script runs inside each node from its user data, so the result arrives as a
    report through the notifier, not over SSH. States: initializing, ok, fail, pending
    (not reported yet), noReport (Running but silent past the timeout) and notApplicable
    (no init script, or reporting off - which is every node while no notifier is
    registered; see get_platform_services()).

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        detail: "summary" (default) - counts per state. "full" - one entry per node with
            its step results, capped; narrow it with status.
        status: With detail="full", only nodes in this state, e.g. "fail"

    Returns:
        Counts per state, and with detail="full" the node entries.
    """
    if status and status not in _INIT_SCRIPT_STATES:
        return {"error": f"unknown status {status!r}; expected one of "
                         f"{', '.join(_INIT_SCRIPT_STATES)}"}
    if _validate_detail(detail) != "full" and not status:
        result = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/initScript",
                             params={"option": "summary"})
    else:
        result = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/initScript",
                             params={"status": status} if status else None)
        nodes = result.get("nodes") if isinstance(result, dict) else None
        if isinstance(nodes, list) and len(nodes) > NODE_LIST_LIMIT:
            result = dict(result)
            result["nodes"] = nodes[:NODE_LIST_LIMIT]
            result["nodesCapped"] = {"total": len(nodes), "shown": NODE_LIST_LIMIT}
    if isinstance(result, dict) and "error" not in result and result.get("total") \
            and result.get("notApplicable") == result.get("total"):
        result["note"] = ("no node reports: either no notifier is registered "
                          "(get_platform_services()) or the init script is switched off")
    return result


# Tool: Init script templates
@tool()
def list_init_script_templates(ns_id: str = "system", template_id: Optional[str] = None,
                               keyword: Optional[str] = None) -> Dict:
    """
    List init script templates, or fetch one with its script text.

    A template is a named first-boot script with default variables, referenced from a
    node group as init_script={"templateId": "<id>", "env": {...}} or run later over SSH
    as a post_script phase with templateId.

    Args:
        ns_id: Namespace ID. Defaults to "system", where the shipped templates live.
        template_id: Return this template in full, script text included
        keyword: Filter the list by keyword

    Returns:
        One line per template (id, description, step names, variables), or the template.
    """
    if template_id:
        return api_request("GET", f"/ns/{ns_id}/template/initScript/{template_id}")
    result = api_request("GET", f"/ns/{ns_id}/template/initScript",
                         params={"filterKeyword": keyword} if keyword else None)
    templates = result.get("templates") if isinstance(result, dict) else None
    if not isinstance(templates, list):
        return result
    return {"templates": [{
        "id": t.get("id"),
        "description": t.get("description"),
        "steps": [s.get("name") or f"step-{i + 1}" for i, s in enumerate(t.get("scripts") or [])],
        "env": t.get("env") or {},
    } for t in templates], "count": len(templates)}


# Tool: Create, update or delete an init script template
@tool(mutating=True)
def manage_init_script_template(action: str, ns_id: str = "default",
                                template_id: Optional[str] = None,
                                template: Optional[Dict] = None,
                                confirm: bool = False) -> Dict:
    """
    Create, update or delete an init script template.

    Args:
        action: "create" | "update" | "delete"
        ns_id: Namespace ID that owns the template
        template_id: Required for update and delete
        template: Body for create and update: {"name": "...", "description": "...",
            "env": {"KEY": "default"}, "scripts": [{"name": "step", "script":
            "#!/bin/bash\\n...", "timeoutSec": 0, "continueOnError": false}]}.
            Each script starts with its shebang line.
        confirm: Must be True for action="delete". Ask the user first.

    Returns:
        The stored template, or the deletion result.
    """
    action = (action or "").strip().lower()
    if action == "create":
        if not template:
            return {"error": "template body is required for action='create'"}
        return api_request("POST", f"/ns/{ns_id}/template/initScript", json_data=template)
    if action == "update":
        if not (template_id and template):
            return {"error": "template_id and template are required for action='update'"}
        return api_request("PUT", f"/ns/{ns_id}/template/initScript/{template_id}",
                           json_data=template)
    if action == "delete":
        if not template_id:
            return {"error": "template_id is required for action='delete'"}
        guard = _require_confirmation(
            confirm, f"deleting init script template {template_id!r}",
            "node groups that reference it can no longer be created")
        if guard:
            return guard
        return api_request("DELETE", f"/ns/{ns_id}/template/initScript/{template_id}")
    return {"error": f"unknown action {action!r}; expected create, update or delete"}


# Tool: Notifier and observability state
@tool()
def get_platform_services(messages: int = 0) -> Dict:
    """
    State of the services CB-Tumblebug runs for its nodes.

    notifier: where nodes report their init script result. With none registered,
    get_init_script_status() shows every node as notApplicable.
    observability: the OTLP endpoint and Grafana that node metrics and logs go to.

    Read-only: registering or provisioning either service is done in MapUI or the REST API.

    Args:
        messages: Include this many of the newest notifier messages (0 = none, max 50)

    Returns:
        ready, plus notifier and observability with registered, status and endpoints.
        Tokens and passwords are redacted.
    """
    def service(path, keep):
        info = api_request("GET", path)
        if not isinstance(info, dict) or "error" in info:
            return {"registered": False, "error": (info or {}).get("error") if isinstance(info, dict) else "unreadable"}
        # Nothing registered answers 204, which api_request turns into a bare message.
        found = {k: info[k] for k in keep if info.get(k)}
        return {"registered": True, **found} if found else {"registered": False}

    ready = api_request("GET", "/readyz")
    out: Dict[str, Any] = {
        "ready": isinstance(ready, dict) and "error" not in ready,
        "notifier": service("/notifier", (
            "status", "statusMessage", "source", "type", "url", "urls", "topic",
            "managedNsId", "managedInfraId", "registeredAt")),
        "observability": service("/observability", (
            "status", "statusMessage", "source", "otlpEndpoint", "grafanaUrl",
            "retentionDays", "managedNsId", "managedInfraId", "registeredAt")),
    }
    try:
        wanted = max(0, min(50, int(messages or 0)))
    except (TypeError, ValueError):
        wanted = 0
    if wanted and out["notifier"].get("registered"):
        recent = api_request("GET", "/notifier/messages", params={"limit": wanted})
        out["notifier"]["recentMessages"] = recent if isinstance(recent, list) else []
    return out


#####################################
# Resources that exist at the CSP but are not managed yet
#####################################

_INSPECT_TYPES = ("vNet", "subnet", "securityGroup", "sshKey", "node")


# Tool: Compare CB-Tumblebug's records with the CSP
@tool()
def inspect_csp_resources(connection_name: str, resource_type: str = "node") -> Dict:
    """
    Compare what CB-Tumblebug manages with what exists at the CSP, for one connection.

    Shows resources found at the provider that CB-Tumblebug does not track - left over
    from a failed run, or created outside it. Read-only. Listing a busy account can take
    minutes.

    Args:
        connection_name: Connection to inspect, e.g. "aws-ap-northeast-2"
        resource_type: "node" (default) | "vNet" | "subnet" | "securityGroup" | "sshKey"
            list what the provider has and CB-Tumblebug does not, whatever its state - a
            terminated machine the provider still lists is included. "audit" checks only
            machines CB-Tumblebug created, by their state at the provider: the check to
            run after a termination.

    Returns:
        Counts (onTumblebug, onCspTotal, onCspOnly) and the resources found only at the
        CSP. For "audit": clean, and when not clean ghostAlive (recorded as gone, still
        running), untrackedAlive (running, no record) and residuals (leftover parts).
    """
    if resource_type == "audit":
        report = api_request("POST", "/auditCspResources",
                             json_data={"connectionName": connection_name},
                             timeout_override=(60, 900))
        if not isinstance(report, dict) or "error" in report:
            return report
        out = {"connectionName": connection_name, "clean": report.get("clean"),
               "summary": {k: v for k, v in (report.get("summary") or {}).items() if v}}
        for conn in report.get("connections") or []:
            for key in ("error", "ghostAlive", "untrackedAlive", "trackedGone", "residuals"):
                found = conn.get(key)
                if found:
                    out[key] = found[:NODE_LIST_LIMIT] if isinstance(found, list) else found
        return out
    if resource_type not in _INSPECT_TYPES:
        return {"error": f"unknown resource_type {resource_type!r}; expected one of "
                         f"{', '.join(_INSPECT_TYPES)}, audit"}
    result = api_request("POST", "/inspectResources",
                         json_data={"connectionName": connection_name,
                                    "resourceType": resource_type},
                         timeout_override=(60, 900))
    if not isinstance(result, dict) or "error" in result:
        return result
    resources = result.get("resources") or {}
    csp_only = ((resources.get("onCspOnly") or {}).get("info")) or []
    out = {
        "connectionName": result.get("connectionName"),
        "resourceType": result.get("resourceType"),
        "overview": result.get("resourceOverview"),
        "onCspOnly": csp_only[:NODE_LIST_LIMIT],
    }
    if len(csp_only) > NODE_LIST_LIMIT:
        out["onCspOnlyCapped"] = {"total": len(csp_only), "shown": NODE_LIST_LIMIT}
    if result.get("systemMessage"):
        out["systemMessage"] = result["systemMessage"]
    return out


# Tool: Bring existing CSP resources under management
@tool(mutating=True, additive=True)
def register_csp_resources(ns_id: str, connection_name: str,
                           resource_type: Optional[str] = None,
                           infra_name_prefix: str = "reg",
                           one_infra: bool = True, confirm: bool = False) -> Dict:
    """
    Register resources that already exist at a CSP into a namespace, for one connection.

    After registration CB-Tumblebug manages them like its own: deleting a registered
    resource through CB-Tumblebug deletes the real one. In a shared account this also
    picks up resources that belong to other people - run inspect_csp_resources() first
    and show the user what would be taken in.

    Args:
        ns_id: Namespace that will own the registered resources
        connection_name: Connection whose region is scanned, e.g. "aws-ap-northeast-2"
        resource_type: Limit to "vNet", "securityGroup", "sshKey" or "node". A type needs
            the ones before it: securityGroup needs vNet, node needs securityGroup and
            sshKey. Omit to register all of them.
        infra_name_prefix: Prefix of the Infra that registered nodes are placed in
        one_infra: True (default) puts the registered nodes in one Infra; False creates
            one Infra per node.
        confirm: Must be True to proceed. Ask the user first.

    Returns:
        Counts and results per resource type.
    """
    guard = _require_confirmation(
        confirm, f"registering the CSP resources of {connection_name}",
        "every matching resource at the provider becomes managed, and deletable, through "
        "CB-Tumblebug - including ones created by others in a shared account")
    if guard:
        return guard
    params = {"infraFlag": "y" if one_infra else "n"}
    if resource_type:
        if resource_type not in ("vNet", "securityGroup", "sshKey", "node"):
            return {"error": f"unknown resource_type {resource_type!r}; expected one of "
                             f"vNet, securityGroup, sshKey, node"}
        params["option"] = resource_type
    result = api_request("POST", "/registerCspResources", params=params,
                         json_data={"connectionName": connection_name, "nsId": ns_id,
                                    "infraNamePrefix": infra_name_prefix},
                         timeout_override=(60, 1800))
    if not isinstance(result, dict) or "error" in result:
        return result
    # Per connection: the counts, the message, and a sample of what was registered.
    out = {k: result.get(k) for k in ("registerationOverview", "elapsedTime") if k in result}
    out["connections"] = []
    for entry in result.get("registerationResult") or []:
        ids = ((entry.get("registerationOutputs") or {}).get("output")) or []
        row = {"connectionName": entry.get("connectionName"),
               "overview": entry.get("registerationOverview"),
               "registered": ids[:NODE_LIST_LIMIT], "registeredCount": len(ids)}
        if entry.get("systemMessage"):
            row["systemMessage"] = entry["systemMessage"]
        out["connections"].append(row)
    return out


#####################################
# Single resources: delete, restore
#####################################

_MANAGED_RESOURCE_KINDS = ("vNet", "securityGroup", "sshKey", "dataDisk", "customImage")


# Tool: Delete one resource, or cancel a deletion that is still pending
@tool(mutating=True)
def manage_resource(action: str, kind: str, resource_id: str,
                    ns_id: str = DEFAULT_NAMESPACE, confirm: bool = False) -> Dict:
    """
    Delete one resource, or cancel a deletion the provider has not confirmed yet.

    release_resources() cleans up everything a namespace no longer uses; this acts on one
    resource. A resource whose deletion is unconfirmed keeps its record and shows
    deletionRequestedAt in get_resources(): "delete" tries again, "restore" cancels the
    deletion and returns it to service.

    Args:
        action: "delete" | "restore"
        kind: "vNet" | "securityGroup" | "sshKey" | "dataDisk" | "customImage"
            ("restore" is not available for customImage)
        resource_id: Resource ID
        ns_id: Namespace ID
        confirm: Must be True for "delete". Ask the user first.

    Returns:
        The server's message.
    """
    action = (action or "").strip().lower()
    if action not in ("delete", "restore"):
        return {"error": f"unknown action {action!r}; expected one of delete, restore"}
    if kind not in _MANAGED_RESOURCE_KINDS:
        return {"error": f"unknown kind {kind!r}; expected one of "
                         f"{', '.join(_MANAGED_RESOURCE_KINDS)}"}
    path = f"/ns/{ns_id}/resources/{kind}/{resource_id}"
    if action == "restore":
        if kind == "customImage":
            return {"error": "restore is not supported for kind 'customImage'"}
        return api_request("PUT", f"{path}/restore")
    guard = _require_confirmation(
        confirm, f"deleting {kind} {resource_id}",
        "the resource is deleted at the provider; anything still using it loses it")
    if guard:
        return guard
    holders = []
    owner = _managed_service_resources(ns_id).get((kind, resource_id))
    if owner:
        holders.append(owner)
    if kind == "vNet":
        groups = api_request("GET", f"/ns/{ns_id}/resources/securityGroup")
        holders += [f"securityGroup {g.get('id')}"
                    for g in (groups.get("securityGroup") or [] if isinstance(groups, dict) else [])
                    if isinstance(g, dict) and g.get("vNetId") == resource_id]
    if holders:
        # The server would remove the subnets and then fail on the network itself
        return {"error": {
            "message": f"{kind} {resource_id} is still used by {len(holders)} resource(s); "
                       f"nothing was deleted",
            "code": "CONFLICT", "usedBy": holders[:20],
            "how_to_proceed": "delete those first; release_resources() removes everything "
                              "unused in the right order",
        }}
    # A VNet goes together with its subnets
    return api_request("DELETE", path, params={"action": "withsubnets"} if kind == "vNet" else None)


#####################################
# Custom images (snapshots)
#####################################

# Tool: Snapshot nodes into custom images
@tool(mutating=True, additive=True)
def create_custom_image(ns_id: str, infra_id: str, name: str,
                        node_id: Optional[str] = None, description: str = "") -> Dict:
    """
    Snapshot a node - or one node of every node group - into a reusable custom image.

    The image keeps what was installed on the node, so later nodes start ready. Use it
    with search_images(custom_images_of=ns_id) and pass its cspImageName as imageId; it
    works only in the provider and region it was taken in. Snapshots are billed storage.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        name: Image name. Without node_id each image is named "<name>-<nodeGroupId>".
        node_id: Snapshot just this node. Omit for one image per node group.
        description: Image description

    Returns:
        The image, or per node group the image id and status. An image is usable when
        get_resources(kind='customImage') shows it Available.
    """
    body = {"name": name}
    if description:
        body["description"] = description
    path = f"/ns/{ns_id}/infra/{infra_id}"
    if node_id:
        path += f"/node/{node_id}"
    result = api_request("POST", f"{path}/snapshot", json_data=body,
                         timeout_override=(60, 3600))
    if isinstance(result, dict) and isinstance(result.get("results"), list):
        result = dict(result)
        result["results"] = [{k: r[k] for k in ("nodeGroupId", "nodeId", "imageId", "status",
                                                "error") if r.get(k)}
                             for r in result["results"]]
    elif isinstance(result, dict) and "error" not in result:
        result = _summarize_resource(result, _RESOURCE_KINDS["customImage"][1] + ("cspImageId",))
        result["next"] = ("the image is usable once get_resources(kind='customImage') shows "
                          "status Available; pass its id as imageId")
    return result


#####################################
# Object storage (buckets)
#####################################

# Tool: Buckets and their objects
@tool(mutating=True, read_actions=("support", "list", "get", "list_objects"))
def manage_object_storage(action: str, ns_id: str = DEFAULT_NAMESPACE,
                          bucket_id: Optional[str] = None,
                          connection_name: Optional[str] = None,
                          object_key: Optional[str] = None,
                          operation: str = "download", expires_seconds: int = 3600,
                          description: str = "", versioning: Optional[bool] = None,
                          cors_rules: Optional[List[Dict]] = None, force: bool = False,
                          confirm: bool = False) -> Dict:
    """
    Create and delete object storage buckets, list objects, and hand out presigned URLs.

    Object data never passes through here: "presigned_url" returns a time-limited URL the
    holder uses directly - `curl -T file "<url>"` to upload, `curl -o file "<url>"` to
    download - which is how a node moves data without holding cloud credentials.

    Args:
        action: "support" (what each provider offers) | "list" | "get" | "create" |
            "list_objects" | "presigned_url" | "delete_object" | "set_versioning" |
            "set_cors" | "empty" (delete every object, keep the bucket) | "delete"
        ns_id: Namespace ID
        bucket_id: Bucket id. For "create" it is the bucket name: 3-63 characters of
            lowercase letters, digits and hyphens, globally unique at most providers.
        connection_name: For "create": where the bucket lives, e.g. "aws-ap-northeast-2"
        object_key: Object name inside the bucket, e.g. "logs/node-1.tar.gz"
        operation: For "presigned_url": "download" (default) | "upload"
        expires_seconds: For "presigned_url": lifetime of the URL (default 3600)
        description: For "create": bucket description
        versioning: For "set_versioning": True keeps old versions of overwritten and
            deleted objects, False suspends that
        cors_rules: For "set_cors": rules a browser needs to use the bucket directly, e.g.
            [{"allowedOrigin": ["*"], "allowedMethod": ["GET", "PUT"],
            "allowedHeader": ["*"], "exposeHeader": ["ETag"], "maxAgeSeconds": 3000}];
            [] removes them
        force: For "delete": True empties a bucket that still holds objects, then deletes it
        confirm: Must be True for "delete", "empty" and "delete_object". Ask the user first.

    Returns:
        The bucket (cspResourceName is its name at the provider; versioning and cors when
        set), the object list, the URL with its method and any requiredHeaders the request
        must carry, or the result of the change.
    """
    action = (action or "").strip().lower()
    base = f"/ns/{ns_id}/resources/objectStorage"
    if action == "support":
        return api_request("GET", "/objectStorage/support")
    if action == "list":
        return get_resources("objectStorage", ns_id)
    if action == "create":
        if not (bucket_id and connection_name):
            return {"error": "bucket_id and connection_name is required for action='create'"}
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]{1,61}[a-z0-9]", bucket_id):
            return {"error": "bucket_id is required to be 3-63 characters of lowercase "
                             "letters, digits and hyphens"}
        body = {"bucketName": bucket_id, "connectionName": connection_name}
        if description:
            body["description"] = description
        return _shape_bucket(api_request("PUT", base, json_data=body))

    if action not in ("get", "list_objects", "presigned_url", "delete_object",
                      "set_versioning", "set_cors", "empty", "delete"):
        return {"error": f"unknown action {action!r}; expected one of support, list, get, "
                         f"create, list_objects, presigned_url, delete_object, "
                         f"set_versioning, set_cors, empty, delete"}
    if not bucket_id:
        return {"error": f"bucket_id is required for action={action!r}"}
    bucket = f"{base}/{bucket_id}"
    if action == "get":
        found = _shape_bucket(api_request("GET", bucket))
        if isinstance(found, dict) and "error" not in found:
            state = api_request("GET", f"{bucket}/versioning")
            if isinstance(state, dict) and state.get("status"):
                found["versioning"] = state["status"]
            rules = api_request("GET", f"{bucket}/cors")
            if isinstance(rules, dict) and rules.get("corsRule"):
                found["cors"] = rules["corsRule"]
        return found
    if action == "set_versioning":
        if versioning is None:
            return {"error": "versioning is required for action='set_versioning'"}
        return api_request("PUT", f"{bucket}/versioning",
                           json_data={"status": "Enabled" if versioning else "Suspended"})
    if action == "set_cors":
        if cors_rules is None:
            return {"error": "cors_rules is required for action='set_cors'"}
        if not cors_rules:
            return api_request("DELETE", f"{bucket}/cors")
        return api_request("PUT", f"{bucket}/cors", json_data={"corsRule": list(cors_rules)})
    if action == "empty":
        guard = _require_confirmation(confirm, f"emptying bucket {bucket_id}",
                                      "every object in the bucket is deleted")
        if guard:
            return guard
        return api_request("DELETE", bucket, params={"option": "empty"})
    if action == "list_objects":
        result = api_request("GET", f"{bucket}/object")
        objects = result.get("objects") if isinstance(result, dict) else None
        if isinstance(objects, list) and len(objects) > 200:
            result = {"objects": objects[:200],
                      "objectsCapped": {"total": len(objects), "shown": 200}}
        return result
    if action == "delete":
        guard = _require_confirmation(
            confirm, f"deleting bucket {bucket_id}",
            "the bucket goes, and with force=True every object in it")
        if guard:
            return guard
        return api_request("DELETE", bucket, params={"option": "force"} if force else None)

    if not object_key:
        return {"error": f"object_key is required for action={action!r}"}
    from urllib.parse import quote
    target = f"{bucket}/object/{quote(object_key, safe='')}"
    if action == "delete_object":
        guard = _require_confirmation(confirm, f"deleting object {object_key}",
                                      "the object cannot be recovered unless versioning is on")
        if guard:
            return guard
        return api_request("DELETE", target)
    if operation not in ("download", "upload"):
        return {"error": f"unknown operation {operation!r}; expected one of download, upload"}
    return api_request("POST", f"{target}/presignedUrl",
                       params={"operation": operation, "expires": int(expires_seconds)})


def _shape_bucket(bucket):
    """A bucket record without what repeats its id or reports nothing wrong."""
    if not isinstance(bucket, dict) or "error" in bucket:
        return bucket
    out = {k: v for k, v in _slim_resource(bucket).items()
           if k not in ("resourceType", "maxKeys", "contents") and v not in _EMPTY}
    if isinstance(bucket.get("contents"), list) and bucket["contents"]:
        out["objectCount"] = len(bucket["contents"])
    for dup, of in (("name", "id"), ("cspResourceId", "cspResourceName"), ("uid", "cspResourceName")):
        if out.get(dup) == out.get(of):
            out.pop(dup, None)
    conditions = out.get("conditions")
    if isinstance(conditions, list) and all(
            isinstance(c, dict) and c.get("status") == "True" for c in conditions):
        out.pop("conditions")
    return out


#####################################
# Load balancers
#####################################

def _shape_nlb(nlb):
    if not isinstance(nlb, dict) or "error" in nlb:
        return nlb
    low = {k[:1].lower() + k[1:]: v for k, v in nlb.items()}
    out = {k: low[k] for k in ("id", "connectionName", "type", "scope", "status",
                               "description", "createdTime", "location")
           if low.get(k) not in _EMPTY}
    for part, keys in (("listener", ("protocol", "port", "dnsName", "ip")),
                       ("targetGroup", ("protocol", "port", "nodeGroupId", "nodes")),
                       ("healthChecker", ("protocol", "port", "interval", "timeout", "threshold"))):
        value = low.get(part)
        if isinstance(value, dict):
            out[part] = {k: value[k] for k in keys if value.get(k) not in _EMPTY}
    return out


def _global_nlb_result(ns_id, infra_id, made, port, protocol):
    """Open the listener port on the HAProxy hosts and report where clients connect."""
    host = made.get("mcNlbHostInfo") or {}
    nodes = [n for n in host.get("node") or [] if isinstance(n, dict)]
    groups = sorted({sg for n in nodes for sg in n.get("securityGroupIds") or []})
    rule = {"Ports": str(port), "Protocol": "UDP" if protocol.upper() == "UDP" else "TCP",
            "Direction": "inbound", "CIDR": "0.0.0.0/0"}
    unopened = [sg for sg in groups
                if "error" in (_add_firewall_rules(ns_id, sg, [rule]) or {})]
    log = (made.get("deploymentLog") or {}).get("results") or []
    failed = [r.get("nodeId") for r in log if isinstance(r, dict) and r.get("err")]
    out = {"hostInfraId": host.get("id") or f"{infra_id}-nlb",
           "addresses": [f"{n.get('publicIP')}:{port}" for n in nodes if n.get("publicIP")],
           "hosts": [{k: n[k] for k in ("id", "status", "publicIP", "specId") if n.get(k)}
                     for n in nodes]}
    if failed:
        out["installFailedOn"] = failed
    if unopened:
        out["listenerNotOpenedOn"] = unopened
    out["next"] = (f"clients use any of addresses; target nodes must accept the target port "
                   f"from the hosts; terminate_infra('{out['hostInfraId']}') removes it")
    return out


# Tool: Network load balancers of an Infra
@tool(mutating=True, read_actions=("list", "get", "health"))
def manage_nlb(action: str, ns_id: str, infra_id: str, nlb_id: Optional[str] = None,
               nodegroup_id: Optional[str] = None, port: int = 80,
               target_port: Optional[int] = None, protocol: str = "TCP",
               scope: str = "REGION", confirm: bool = False) -> Dict:
    """
    Put a load balancer in front of an Infra's nodes.

    scope "REGION" creates the provider's own NLB in front of one node group, which lives
    in one region. scope "GLOBAL" covers every node of the Infra, wherever it runs: it
    builds a billed HAProxy Infra named "<infra_id>-nlb" and opens the listener port on
    it. It reaches the nodes on their public addresses, so target_port must be open on
    their SecurityGroups.

    Args:
        action: "list" | "get" | "health" | "create" | "delete"
        ns_id: Namespace ID
        infra_id: Infra whose nodes are the targets
        nlb_id: NLB id, for get, health and delete. A regional NLB takes the id of its
            node group.
        nodegroup_id: For "create" with scope "REGION": the node group to balance
        port: Listener port (default 80)
        target_port: Port on the nodes (default: the listener port)
        protocol: "TCP" (default) | "UDP"; scope "GLOBAL" also takes "HTTP"
        scope: "REGION" (default) | "GLOBAL"
        confirm: Must be True for "create" with scope "GLOBAL" and for "delete"

    Returns:
        The NLB with its listener address (listener.dnsName), the list, or healthyNodes
        and unhealthyNodes. A global NLB returns its client addresses and host Infra.
    """
    action = (action or "").strip().lower()
    scope = (scope or "REGION").strip().upper()
    base = f"/ns/{ns_id}/infra/{infra_id}/nlb"
    if action == "list":
        result = api_request("GET", base)
        if isinstance(result, dict) and isinstance(result.get("nlb"), list):
            result = {"nlb": [_shape_nlb(n) for n in result["nlb"]]}
        return result
    if action == "create":
        if scope not in ("REGION", "GLOBAL"):
            return {"error": f"unknown scope {scope!r}; expected one of REGION, GLOBAL"}
        spec = {"protocol": protocol.upper(), "port": str(port)}
        target = {"protocol": protocol.upper(), "port": str(target_port or port)}
        body = {"type": "PUBLIC", "scope": scope, "listener": spec, "targetGroup": target,
                "healthChecker": {"interval": 10, "timeout": 10, "threshold": 3}}
        if scope == "GLOBAL":
            guard = _require_confirmation(
                confirm, f"creating a global load balancer for {infra_id}",
                f"a new billed Infra '{infra_id}-nlb' running HAProxy is created")
            if guard:
                return guard
            made = api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/mcSwNlb",
                               json_data=body, timeout_override=(60, 3600))
            if not isinstance(made, dict) or "error" in made:
                return made
            return _global_nlb_result(ns_id, infra_id, made, port, protocol)
        if not nodegroup_id:
            return {"error": "nodegroup_id is required for action='create' with scope "
                             "'REGION'"}
        target["nodeGroupId"] = nodegroup_id
        created = _shape_nlb(api_request("POST", base, json_data=body,
                                         timeout_override=(60, 1800)))
        if isinstance(created, dict) and "error" not in created:
            created["next"] = ("targets pass their health checks a few minutes after "
                               "creation: manage_nlb(action='health'); clients use "
                               "listener.dnsName")
        return created
    if action not in ("get", "health", "delete"):
        return {"error": f"unknown action {action!r}; expected one of list, get, health, "
                         f"create, delete"}
    if not nlb_id:
        return {"error": f"nlb_id is required for action={action!r}"}
    if action == "get":
        return _shape_nlb(api_request("GET", f"{base}/{nlb_id}"))
    if action == "health":
        health = api_request("GET", f"{base}/{nlb_id}/healthz")
        if not isinstance(health, dict) or "error" in health:
            return health
        low = {k.lower(): v or [] for k, v in health.items()}
        return {"healthyNodes": low.get("healthynodes", []),
                "unhealthyNodes": low.get("unhealthynodes", []),
                "allNodes": low.get("allnodes", [])}
    guard = _require_confirmation(confirm, f"deleting NLB {nlb_id}",
                                  "traffic through this load balancer stops")
    if guard:
        return guard
    return api_request("DELETE", f"{base}/{nlb_id}", timeout_override=(60, 1800))


#####################################
# Managed Kubernetes clusters
#####################################

_K8S_NODEGROUP_SUMMARY = ("name", "status", "specId", "imageId", "desiredNodeSize",
                          "minNodeSize", "maxNodeSize", "onAutoScaling")


def _shape_k8s_cluster(cluster, detail="summary"):
    if detail == "full" or not isinstance(cluster, dict) or "error" in cluster:
        return cluster
    out = {k: cluster[k] for k in ("id", "connectionName", "version", "status", "description",
                                   "systemMessage", "createdTime", "cspResourceName")
           if cluster.get(k) not in _EMPTY}
    network = cluster.get("network")
    if isinstance(network, dict):
        out["network"] = {k: network[k] for k in ("vNetId", "subnetIds", "securityGroupIds")
                          if network.get(k) not in _EMPTY}
    endpoint = (cluster.get("accessInfo") or {}).get("endpoint")
    if endpoint:
        out["endpoint"] = endpoint
    out["k8sNodeGroupList"] = [
        {k: g[k] for k in _K8S_NODEGROUP_SUMMARY if g.get(k) not in (None, "")}
        for g in cluster.get("k8sNodeGroupList") or [] if isinstance(g, dict)]
    label = {k: v for k, v in (cluster.get("label") or {}).items() if not k.startswith("sys.")}
    if label:
        out["label"] = label
    return out


def _standalone_kubeconfig(text, token):
    """Swap a credential-plugin user for a bearer token, so the file works on its own."""
    try:
        import yaml
        config = yaml.safe_load(text)
        for user in config.get("users") or []:
            if "exec" in (user.get("user") or {}):
                user["user"] = {"token": token}
        return yaml.safe_dump(config, sort_keys=False)
    except Exception:
        return None


# Tool: Managed Kubernetes clusters (EKS, GKE, AKS and the like)
@tool(mutating=True, read_actions=("info", "versions", "list", "get"))
def manage_k8s_cluster(action: str, ns_id: str = DEFAULT_NAMESPACE,
                       cluster_id: Optional[str] = None, spec_id: Optional[str] = None,
                       image_id: str = "default", version: Optional[str] = None,
                       nodegroup_name: Optional[str] = None, desired_node_size: int = 1,
                       min_node_size: int = 1, max_node_size: int = 3,
                       root_disk_type: str = "", root_disk_size: int = 0,
                       detail: str = "summary", budget_ack: Optional[str] = None,
                       confirm: bool = False) -> Dict:
    """
    Create and operate a provider-managed Kubernetes cluster.

    This is the CSP's own service (EKS, GKE, AKS, ...), billed for its control plane as
    well as its nodes and taking 10-20 minutes to create. Where "info" reports
    nodegroups_on_creation false (AWS, Alibaba), "create" makes the control plane only
    and the first node group is added once it is Active. A cluster built on plain VMs
    with kubeadm is a different path: create_infra_dynamic() plus the k8s_* scripts in
    list_predefined_scripts().

    Args:
        action: "info" (what each provider supports; spec_id narrows it to that
            provider) | "versions" (needs spec_id) |
            "list" | "get" | "create" | "add_nodegroup" | "scale" (set a node group's
            sizes) | "delete_nodegroup" | "kubeconfig" | "delete"
        ns_id: Namespace ID
        cluster_id: Cluster id; for "create" the name of the new cluster
        spec_id: Node spec from recommend_vm_spec(); it also decides provider and region
        image_id: Node image. "default" lets the provider choose, and is required where
            "info" reports that node images cannot be designated.
        version: Kubernetes version from "versions"; omitted, the provider's default
        nodegroup_name: Node group name
        desired_node_size: Nodes to run (default 1)
        min_node_size: Autoscaling lower bound (default 1)
        max_node_size: Autoscaling upper bound (default 3)
        root_disk_type: Node root disk type; "" for the provider default
        root_disk_size: Node root disk size in GB; 0 for the provider default
        detail: "summary" (default) | "full"
        budget_ack: Approval id from request_budget_increase(), to pass a spend limit once
        confirm: Must be True for create, add_nodegroup, delete_nodegroup, delete and
            kubeconfig (it returns credentials). Ask the user first.

    Returns:
        The cluster with its node groups and status ("Active" when usable), the list, the
        supported versions, the resized node group, or the kubeconfig text.
    """
    action = (action or "").strip().lower()
    actions = ("info", "versions", "list", "get", "create", "add_nodegroup", "scale",
               "delete_nodegroup", "kubeconfig", "delete")
    if action not in actions:
        return {"error": f"unknown action {action!r}; expected one of {', '.join(actions)}"}
    level = _validate_detail(detail)
    base = f"/ns/{ns_id}/k8sCluster"
    if action == "info":
        info = api_request("GET", "/k8sClusterInfo")
        providers = info.get("k8s_cluster") if isinstance(info, dict) else None
        provider = str(spec_id or "").split("+")[0]
        if provider and isinstance(providers, dict) and provider in providers:
            return {"k8s_cluster": {provider: providers[provider]}}
        return info
    if action == "versions":
        parts = str(spec_id or "").split("+")
        if len(parts) < 3:
            return {"error": "spec_id is required for action='versions' "
                             "(provider and region are read from it)"}
        return api_request("GET", "/availableK8sVersion",
                           params={"providerName": parts[0], "regionName": parts[1]})
    if action == "list":
        result = api_request("GET", base)
        clusters = None
        if isinstance(result, dict):
            clusters = result.get("K8sClusterInfo", result.get("cluster"))
        if isinstance(clusters, list):
            return {"cluster": [_shape_k8s_cluster(c, level) for c in clusters],
                    "count": len(clusters)}
        return result

    if not cluster_id:
        return {"error": f"cluster_id is required for action={action!r}"}
    sizes = {"desiredNodeSize": int(desired_node_size), "minNodeSize": int(min_node_size),
             "maxNodeSize": int(max_node_size)}
    if not sizes["minNodeSize"] <= sizes["desiredNodeSize"] <= sizes["maxNodeSize"]:
        return {"error": "min_node_size <= desired_node_size <= max_node_size is required"}
    disk = {}
    if root_disk_type:
        disk["rootDiskType"] = root_disk_type
    if root_disk_size:
        disk["rootDiskSize"] = int(root_disk_size)

    if action == "get":
        return _shape_k8s_cluster(api_request("GET", f"{base}/{cluster_id}"), level)
    if action == "scale":
        if not nodegroup_name:
            return {"error": "nodegroup_name is required for action='scale'"}
        scaled = api_request(
            "PUT", f"{base}/{cluster_id}/k8sNodeGroup/{nodegroup_name}/autoscaleSize",
            json_data=sizes, timeout_override=(60, 1800))
        if level == "full" or not isinstance(scaled, dict) or "error" in scaled:
            return scaled
        out = {k: scaled[k] for k in _K8S_NODEGROUP_SUMMARY if scaled.get(k) not in (None, "")}
        out["nodeCount"] = len(scaled.get("k8sNodes") or [])
        return out

    if action in ("create", "add_nodegroup"):
        if not spec_id:
            return {"error": f"spec_id is required for action={action!r}"}
        if action == "add_nodegroup" and not nodegroup_name:
            return {"error": "nodegroup_name is required for action='add_nodegroup'"}
        plan = [{"name": nodegroup_name or cluster_id, "nodeGroupSize": sizes["desiredNodeSize"],
                 "specId": spec_id, "imageId": image_id}]
        # Some providers create the control plane alone and take node groups afterwards
        with_nodes = True
        if action == "create":
            answer = api_request("GET", "/checkK8sNodeGroupsOnK8sCreation",
                                 params={"providerName": spec_id.split("+")[0]})
            with_nodes = not (isinstance(answer, dict)
                              and str(answer.get("result")).lower() == "false")
        if with_nodes:
            unconfirmed = _confirm_creation_plan(plan, f"Kubernetes cluster {cluster_id}", confirm)
            if unconfirmed:
                unconfirmed["error"]["cost_caveat"] = (
                    "node list price only - the managed control plane is billed on top")
                return unconfirmed
            over_budget = _budget_refusal(plan, budget_ack)
            if over_budget:
                return over_budget
        else:
            guard = _require_confirmation(
                confirm, f"creating Kubernetes cluster {cluster_id}",
                "a managed control plane is created and billed until it is deleted; this "
                "provider takes node groups only after the cluster is Active")
            if guard:
                return guard
        body = {"specId": spec_id, "imageId": image_id, **sizes, **disk}
        if action == "create":
            body["name"] = cluster_id
            if nodegroup_name:
                body["nodeGroupName"] = nodegroup_name
            if version:
                body["version"] = version
            result = api_request("POST", f"/ns/{ns_id}/k8sClusterDynamic", json_data=body,
                                 timeout_override=(60, 3600))
        else:
            body["name"] = nodegroup_name
            result = api_request("POST", f"{base}/{cluster_id}/k8sNodeGroupDynamic",
                                 json_data=body, timeout_override=(60, 3600))
        if not isinstance(result, dict) or "error" in result:
            return result
        result = _shape_k8s_cluster(result, level)
        if with_nodes:
            _record_spend(plan)
        if action == "create" and level != "full":
            result["next"] = (
                "poll action='get' until status is Active (10-20 minutes)" + (
                    "" if with_nodes else
                    "; then action='add_nodegroup' with nodegroup_name and spec_id - no "
                    "node group was created with the cluster"))
        return result

    what = {"delete_nodegroup": (f"deleting node group {nodegroup_name}",
                                 "its nodes and the workloads on them are removed"),
            "delete": (f"deleting Kubernetes cluster {cluster_id}",
                       "the cluster and everything running in it is removed"),
            "kubeconfig": (f"reading the kubeconfig of {cluster_id}",
                           "cluster-admin credentials are placed in this transcript")}[action]
    guard = _require_confirmation(confirm, *what)
    if guard:
        return guard
    if action == "kubeconfig":
        found = api_request("GET", f"{base}/{cluster_id}/kubeconfig")
        text = found.get("kubeconfig") if isinstance(found, dict) else None
        if isinstance(text, str) and "exec:" in text:
            issued = api_request("GET", f"{base}/{cluster_id}/token")
            status = ((issued if isinstance(issued, dict) else {}).get("execCredential")
                      or {}).get("status") or {}
            standalone = status.get("token") and _standalone_kubeconfig(text, status["token"])
            if standalone:
                return {"kubeconfig": standalone,
                        "credentialExpires": status.get("expirationTimestamp"),
                        "note": "holds a short-lived token in place of the provider's "
                                "credential plugin; read it again once it expires"}
        return found
    if action == "delete_nodegroup":
        if not nodegroup_name:
            return {"error": "nodegroup_name is required for action='delete_nodegroup'"}
        return api_request("DELETE", f"{base}/{cluster_id}/k8sNodeGroup/{nodegroup_name}",
                           timeout_override=(60, 3600))
    result = api_request("DELETE", f"{base}/{cluster_id}", timeout_override=(60, 3600))
    refused = result.get("error") if isinstance(result, dict) else None
    if isinstance(refused, dict) and "nodegroups attached" in str(refused.get("message")).lower():
        return {"error": {
            "message": f"cluster {cluster_id} still has node groups; nothing was deleted",
            "code": "CONFLICT",
            "how_to_proceed": "action='delete_nodegroup' for each, wait until action='get' "
                              "lists none, then delete the cluster",
        }}
    return result


#####################################
# Managed databases (RDBMS)
#####################################

_RDBMS_SUMMARY = ("id", "name", "connectionName", "status", "endpoint", "publicAccess",
                  "dbEngine", "dbEngineVersion", "dbInstanceSpec", "storageType", "storageSize",
                  "highAvailability", "deletionProtection", "vNetId", "subnetIds",
                  "securityGroupIds", "description", "systemMessage", "deletionRequestedAt")
_RDBMS_ACTIONS = ("support", "capability", "list", "get", "validate", "create", "reconcile",
                  "delete", "list_databases", "create_database", "delete_database",
                  "secure_transport")


# Tool: Managed relational databases (RDS, Cloud SQL and the like)
@tool(mutating=True, read_actions=("support", "capability", "list", "get", "validate",
                                   "list_databases", "secure_transport"))
def manage_rdbms(action: str, ns_id: str = DEFAULT_NAMESPACE,
                 rdbms_id: Optional[str] = None, connection_name: Optional[str] = None,
                 vnet_id: Optional[str] = None, db_engine: str = "mysql",
                 admin_user_name: Optional[str] = None,
                 admin_user_password: Optional[str] = None,
                 subnet_ids: Optional[List[str]] = None,
                 security_group_ids: Optional[List[str]] = None,
                 public_access: bool = False, options: Optional[Dict] = None,
                 database_name: Optional[str] = None, force: bool = False,
                 detail: str = "summary", confirm: bool = False) -> Dict:
    """
    Create and operate a provider-managed MySQL or MariaDB instance.

    The instance joins a VNet that already exists in the namespace - normally the one an
    Infra in that region created - so its nodes reach the database by private address.
    Creation takes 5-15 minutes and is billed until deleted. Providers differ in what
    they need; details and a worked example: get_usage_guide('services').

    Args:
        action: "support" (what each provider offers) | "capability" (versions, instance
            specs and storage for connection_name) | "list" | "get" | "validate" (check a
            create request and see the defaults it would get) | "create" | "reconcile"
            (refresh the record from the provider) | "delete" | "list_databases" |
            "create_database" | "delete_database" | "secure_transport" (TLS state and CA)
        ns_id: Namespace ID
        rdbms_id: Instance id; for "create" and "validate" the name of the new instance
        connection_name: Where the instance runs, e.g. "aws-ap-northeast-2"
        vnet_id: VNet the instance joins; get_resources(kind='vNet') lists them
        db_engine: "mysql" (default) | "mariadb"
        admin_user_name: Admin login, for create and for the database and TLS actions
        admin_user_password: Admin password. It is not stored by CB-Tumblebug: keep it.
        subnet_ids: Subnets of the VNet for the instance (AWS needs two in different zones)
        security_group_ids: Security groups applied to the instance, where the provider
            takes them
        public_access: True gives the instance a public endpoint (default False)
        options: Other create fields: dbEngineVersion, dbInstanceSpec, storageType,
            storageSize, highAvailability, deletionProtection, backupRetentionDays,
            description. Left out, valid defaults are filled in.
        database_name: Logical database name, for create_database and delete_database
        force: For "delete": delete despite deletion protection
        detail: "summary" (default) | "full". For "capability", summary lists the
            smallest instance specs and full the provider's whole catalogue.
        confirm: Must be True for create, delete and delete_database. Ask the user first.

    Returns:
        The instance with its status ("Available" when usable) and endpoint, the list,
        the provider's capabilities, or the logical databases (also after creating or
        deleting one).
    """
    action = (action or "").strip().lower()
    if action not in _RDBMS_ACTIONS:
        return {"error": f"unknown action {action!r}; expected one of "
                         f"{', '.join(_RDBMS_ACTIONS)}"}
    base = f"/ns/{ns_id}/resources/rdbms"
    if action == "support":
        return api_request("GET", "/rdbms/support")
    if action == "list":
        return get_resources("rdbms", ns_id)
    if action == "capability":
        if not connection_name:
            return {"error": "connection_name is required for action='capability'"}
        connection = api_request("GET", f"/connConfig/{connection_name}")
        if not isinstance(connection, dict) or "error" in connection:
            return connection
        region = (connection.get("regionDetail") or {}).get("regionName") \
            or (connection.get("regionZoneInfo") or {}).get("assignedRegion")
        found = api_request("GET", "/rdbms/capability", params={
            "providerName": connection.get("providerName"), "regionName": region,
            "dbEngine": db_engine}, timeout_override=(60, 300))
        supports = found.get("supports") if isinstance(found, dict) else None
        if _validate_detail(detail) == "full" or not isinstance(supports, dict):
            return found

        def size(spec):
            try:
                return float(spec.get("vCpuCount") or 0), float(spec.get("memSizeMiB") or 0)
            except (TypeError, ValueError):
                return 0.0, 0.0
        specs = sorted((x for x in supports.get("dbInstanceSpecs") or [] if isinstance(x, dict)),
                       key=size)
        out = {k: v for k, v in supports.items()
               if k not in ("dbInstanceSpecs", "dbInstanceSpecOptions", "notes")
               and v not in _EMPTY}
        out["dbInstanceSpecs"] = [
            {k: v for k, v in (("name", x.get("name")), ("vCPU", x.get("vCpuCount")),
                               ("memMiB", x.get("memSizeMiB")))
             if v not in (None, "", "-1", -1)} for x in specs[:15]]
        total = len(specs) or len(supports.get("dbInstanceSpecOptions") or [])
        if total > 15:
            out["dbInstanceSpecCount"] = total
            out["note"] = "the 15 smallest instance specs are listed; detail='full' has all"
        if not specs and supports.get("dbInstanceSpecOptions"):
            out["dbInstanceSpecOptions"] = supports["dbInstanceSpecOptions"][:30]
        storage = (supports.get("notes") or {}).get("storageTypes") or []
        if storage:
            out["storageTypeNotes"] = {
                x.get("storageType"): " ".join(filter(None, (x.get("recommendationLevel"),
                                                             x.get("constraints"))))
                for x in storage if isinstance(x, dict)}
        return out

    if not rdbms_id:
        return {"error": f"rdbms_id is required for action={action!r}"}
    if action in ("validate", "create"):
        missing = [name for name, value in (("connection_name", connection_name),
                                            ("vnet_id", vnet_id),
                                            ("admin_user_name", admin_user_name),
                                            ("admin_user_password", admin_user_password))
                   if not value]
        if missing:
            return {"error": f"{', '.join(missing)} is required for action={action!r}"}
        body: Dict[str, Any] = {"autoFillDefaults": True, **(options or {})}
        body.update({"name": rdbms_id, "connectionName": connection_name, "vNetId": vnet_id,
                     "dbEngine": db_engine, "adminUserName": admin_user_name,
                     "adminUserPassword": admin_user_password,
                     "publicAccess": bool(public_access)})
        if subnet_ids:
            body["subnetIds"] = list(subnet_ids)
        if security_group_ids:
            body["securityGroupIds"] = list(security_group_ids)
        if action == "validate":
            return api_request("POST", f"{base}/validate", json_data=body)
        guard = _require_confirmation(
            confirm, f"creating database instance {rdbms_id}",
            "a managed database is created at the provider and billed until it is deleted; "
            "its price is not part of the spend limits")
        if guard:
            return guard
        created = api_request("POST", base, json_data=body, timeout_override=(60, 3600))
        if not isinstance(created, dict) or "error" in created:
            return created
        created = _summarize_resource(created, _RDBMS_SUMMARY)
        created["next"] = ("open the database port in a security group of the VNet, then "
                           "connect to endpoint from a node there")
        return created

    instance = f"{base}/{rdbms_id}"
    if action == "get":
        found = api_request("GET", instance)
        if not isinstance(found, dict) or "error" in found or _validate_detail(detail) == "full":
            return found
        return _summarize_resource(found, _RDBMS_SUMMARY)
    if action == "reconcile":
        return api_request("PUT", f"{instance}/reconcile")
    if action == "delete":
        guard = _require_confirmation(confirm, f"deleting database instance {rdbms_id}",
                                      "the instance and all data in it are removed")
        if guard:
            return guard
        return api_request("DELETE", instance, params={"option": "force"} if force else None,
                           timeout_override=(60, 3600))

    # The remaining actions log in to the instance as its admin
    if not (admin_user_name and admin_user_password):
        return {"error": f"admin_user_name and admin_user_password is required for "
                         f"action={action!r}"}
    login = {"X-Admin-User-Name": admin_user_name, "X-Admin-User-Password": admin_user_password}
    if action == "secure_transport":
        return api_request("GET", f"{instance}/secure-transport", headers=login)
    if action == "list_databases":
        return api_request("GET", f"{instance}/database", headers=login)
    if not database_name:
        return {"error": f"database_name is required for action={action!r}"}
    if action == "create_database":
        changed = api_request("POST", f"{instance}/database", headers=login,
                              json_data={"databaseName": database_name})
    else:
        guard = _require_confirmation(confirm, f"deleting database {database_name}",
                                      "the database and its tables are dropped")
        if guard:
            return guard
        changed = api_request("DELETE", f"{instance}/database/{database_name}", headers=login)
    if not isinstance(changed, dict) or "error" in changed:
        return changed
    # The server caches the list for two seconds
    time.sleep(2.2)
    listed = api_request("GET", f"{instance}/database", headers=login)
    if isinstance(listed, dict) and isinstance(listed.get("databases"), list):
        changed["databases"] = listed["databases"]
    return changed

#####################################
# Command & File Management
#####################################


# Tool: Execute remote command to VMs in Infra
@tool(mutating=True)
def execute_command_infra(
    ns_id: str,
    infra_id: str,
    commands: Optional[List[str]] = None,
    script_name: Optional[str] = None,
    template_variables: Optional[Dict[str, str]] = None,
    nodegroup_id: Optional[str] = None,
    node_id: Optional[str] = None,
    label_selector: Optional[str] = None,
    timeout_minutes: Optional[int] = None,
    run_async: bool = False,
    tail_lines: int = 20,
    contains: Optional[str] = None,
    template_id: Optional[str] = None,
    env: Optional[Dict[str, str]] = None
) -> Dict:
    """Run shell commands on the nodes of an Infra over SSH.

    Each entry in commands is its own SSH session, so shell state does not carry over: put
    dependent steps in one entry joined with &&. Without a target the command runs on
    every node. Details: get_usage_guide('commands').

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        commands: Shell commands. Each element is a separate SSH session.
        script_name: Run a predefined script, before any commands given.
            See list_predefined_scripts().
        template_variables: Values for {{placeholder}} in the script or commands.
            public_ip, public_ips_space, public_ips_comma, private_ips_space, infra_id and
            ns_id are filled in automatically when left unset.
        node_id: Run on one node only
        nodegroup_id: Run on one NodeGroup only
        label_selector: Run on nodes matching a label, e.g. "role=worker"
        timeout_minutes: Per-command timeout (server default 30, max 120)
        run_async: Return a request_id at once instead of waiting; follow up with
            get_command_result(). Use it for anything slow, so a dropped connection
            cannot lose the result.
        tail_lines: Lines of each output stream kept per node (default 20)
        contains: Keep only output lines matching this regex, with 3 lines of context,
            instead of the tail - reaches a block in the middle of a long log
        template_id: Run an init script template over SSH (as root) instead of commands;
            see list_init_script_templates()
        env: Environment variables exported to the commands or the template

    Returns:
        request_id, nodes {total, succeeded, failed} and per-node results with stdout,
        stderr and error. A node failing does not make the call an error: read nodes.
    """
    script = None
    if script_name:
        script = PREDEFINED_SCRIPTS.get(script_name)
        if not script:
            return {"error": f"unknown script_name {script_name!r}; expected one of "
                             f"{', '.join(sorted(PREDEFINED_SCRIPTS))}"}
        commands = list(script["commands"]) + list(commands or [])
        template_variables = {**script.get("defaults", {}), **(template_variables or {})}
        timeout_minutes = timeout_minutes or script.get("timeout_minutes")
    commands = [c.strip() for c in (commands or []) if isinstance(c, str) and c.strip()]
    if not commands and not template_id:
        return {"error": "commands is required (or script_name, or template_id)"}

    data: Dict[str, Any] = {}
    if commands:
        data["command"] = _substitute_template_variables(commands, template_variables,
                                                         ns_id, infra_id)
    if script:
        # An unfilled placeholder would reach the node as literal text
        unfilled = sorted({name for c in data["command"][:len(script["commands"])]
                           for name in re.findall(r"\{\{(\w+)\}\}", c)})
        if unfilled:
            return {"error": f"template_variables is required for {', '.join(unfilled)} "
                             f"(script {script_name}); list_predefined_scripts(name=...) "
                             f"describes them"}
    if template_id:
        data["templateId"] = template_id
    if env:
        data["env"] = {str(k): str(v) for k, v in env.items()}
    request_timeout = (60, 1920)
    if timeout_minutes:
        capped = max(1, min(int(timeout_minutes), 120))
        data["timeoutMinutes"] = capped
        request_timeout = (60, capped * 60 + 120)

    params = {}
    if nodegroup_id:
        params["nodeGroupId"] = nodegroup_id
    if node_id:
        params["nodeId"] = node_id
    if label_selector:
        params["labelSelector"] = label_selector
    # The id lets the output be fetched again if this call is cut off.
    request_id = f"mcp-cmd-{infra_id}-{int(time.time() * 1000)}"
    url = f"/ns/{ns_id}/cmd/infra/{infra_id}"

    if run_async:
        params["async"] = "true"
        started = api_request("POST", url, json_data=data, params=params,
                              headers={"x-request-id": request_id},
                              timeout_override=(60, 120))
        if not isinstance(started, dict) or "error" in started:
            return started
        request_id = started.get("xRequestId") or request_id
        return {"started": True, "request_id": request_id,
                "next": f"get_command_result(request_id='{request_id}', "
                        f"ns_id='{ns_id}', infra_id='{infra_id}') for the outcome"}

    result = api_request("POST", url, json_data=data, params=params or None,
                         headers={"x-request-id": request_id},
                         timeout_override=request_timeout)
    if not isinstance(result, dict) or "results" not in result:
        return result

    rows = []
    for item in result.get("results") or []:
        row = _trim_command_record(
            {k: item[k] for k in ("nodeId", "nodeIp", "stdout", "stderr", "error")
             if item.get(k)}, tail_lines, contains)
        for stream in ("stdout", "stderr"):
            if isinstance(row.get(stream), dict):
                row[stream] = {k: v for k, v in row[stream].items() if v}
                if not row[stream]:
                    del row[stream]
        rows.append(row)
    failed = [r for r in rows if r.get("error")]
    out: Dict[str, Any] = {
        "request_id": request_id,
        "nodes": {"total": len(rows), "succeeded": len(rows) - len(failed),
                  "failed": len(failed)},
    }
    if len(rows) > NODE_LIST_LIMIT:
        rows = (failed + [r for r in rows if not r.get("error")])[:NODE_LIST_LIMIT]
        out["resultsCapped"] = "failed nodes are listed first; narrow the target to see others"
    out["results"] = rows
    errors = " ".join(str(r.get("error")) for r in failed).lower()
    if any(sign in errors for sign in ("authentication failed", "unable to authenticate",
                                       "connection refused")):
        out["hint"] = ("a node that was just created or rebooted refuses SSH until its login "
                       "user and key are in place, usually under a minute - retry before "
                       "treating this as a failure")
    if script and script.get("ports"):
        out["ports"] = script["ports"]
        out["next"] = ("the service listens on these ports: open the ones that must be "
                       "reachable with add_security_group_rules()")
    return out

# Predefined scripts, mirroring the CB-MapUI usecase menus. {{name}} placeholders are filled
# from template_variables, then from "defaults"; "ports" are what the service listens on.
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
            "ps aux | head -20",
        ],
        "description": "Comprehensive system information collection",
    },
    "devstack_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/1.installDevStack.sh -o /tmp/installDevStack.sh && bash /tmp/installDevStack.sh --csp-name {{csp_name}} --latitude {{latitude}} --longitude {{longitude}} --location \"{{location}}\"",
        ],
        "description": "Install OpenStack (DevStack) on a BARE-METAL node. Requires a *.metal spec: nested KVM needs hardware virtualization, and ordinary instances fall back to slow QEMU emulation. Takes 20-40 minutes, so pass timeout_minutes=120. Safe to re-run: it attaches to an install in progress instead of starting a second one. Prints credentials.yaml/cloudinfo.yaml snippets at the end - feed those to register_csp_definition() and register_csp_credential().",
        "defaults": {"csp_name": "openstack-$$Func(GetInfraId())-$$Func(GetNodeId())", "latitude": "$$Func(GetLocationLatitude())", "longitude": "$$Func(GetLocationLongitude())", "location": "$$Func(GetLocationDisplay())"},
        "ports": ["80/tcp", "6080/tcp", "8774/tcp", "9292/tcp", "9696/tcp", "8776/tcp", "9876/tcp"],
        "min_disk_gb": 100,
        "timeout_minutes": 120,
    },
    "devstack_registration_info": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/2.getRegistrationInfo.sh -o /tmp/getRegistrationInfo.sh && bash /tmp/getRegistrationInfo.sh --csp-name {{csp_name}} --latitude {{latitude}} --longitude {{longitude}} --location \"{{location}}\"",
        ],
        "description": "Re-print the registration snippets (Keystone endpoint, ProjectID, region/zone) for an already-installed DevStack.",
        "defaults": {"csp_name": "openstack-$$Func(GetInfraId())-$$Func(GetNodeId())", "latitude": "$$Func(GetLocationLatitude())", "longitude": "$$Func(GetLocationLongitude())", "location": "$$Func(GetLocationDisplay())"},
    },
    "devstack_update_endpoints": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/3.updateEndpoints.sh -o /tmp/updateEndpoints.sh && bash /tmp/updateEndpoints.sh --csp-name {{csp_name}}",
        ],
        "description": "Repoint the OpenStack service catalog after the host's public IP changed (stop/start, or no elastic IP). Re-register the credential afterwards.",
        "defaults": {"csp_name": "openstack-$$Func(GetInfraId())-$$Func(GetNodeId())"},
    },
    "devstack_clean": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/4.cleanDevStack.sh -o /tmp/cleanDevStack.sh && bash /tmp/cleanDevStack.sh",
        ],
        "description": "Tear down a failed or stale DevStack so it can be installed cleanly again.",
    },
    "k8s_control_plane": {
        "commands": [
            "CNI=$(echo \"{{k8s_cni}}\" | tr 'A-Z' 'a-z' | xargs); [ -z \"$CNI\" ] && CNI=flannel; curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/k8s/k8s-control-plane-setup.sh | bash -s -- --cni \"$CNI\"",
        ],
        "description": "Install a Kubernetes control plane. K8S_CNI selects the CNI: empty or 'flannel' (default), or 'cilium'. Needs working outbound access - verify the security group has an outbound rule first, since a stateful firewall keeps SSH alive while blocking every package download. Before joining workers, also open the cluster's inbound ports - see k8s_worker_join for the list.",
        "defaults": {"k8s_cni": "flannel"},
        "ports": ["6443/tcp"],
        "min_disk_gb": 50,
    },
    "k8s_get_join_command": {
        "commands": [
            "echo '[K8S_JOIN_COMMAND]'; sudo kubeadm token create --print-join-command",
        ],
        "description": "Print the kubeadm join command, to be handed to workers. Run on the CONTROL PLANE. Tokens expire (24h by default), so fetch this immediately before joining workers rather than reusing an old one. Use summarize_output=false so the command is not truncated.",
    },
    "k8s_worker_join": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/k8s/k8s-worker-setup.sh | bash -s -- -j \"{{join_command}}\"",
        ],
        "description": "Install the kubelet stack and join this node to an existing cluster. Run on the WORKER nodes (target them with node_id or label_selector - never the control plane, which is already a member). join_command comes from k8s_get_join_command. OPEN THE CLUSTER PORTS FIRST - a CB-Tumblebug security group allows only SSH, so the join times out at 'couldn't validate the identity of the API Server'. On the control plane's group allow inbound TCP 6443,2379-2380,10250,10257,10259,4240 and UDP 8472; on the workers' group TCP 10250,4240,30000-32767 and UDP 8472; source is the node subnet (get_infra shows the private addresses). 4240 and 8472 are Cilium's health and VXLAN ports - without them nodes join and then stay NotReady.",
    },
    "k8s_storage_default": {
        "commands": [
            "kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.31/deploy/local-path-storage.yaml",
            "kubectl patch storageclass local-path -p '{\"metadata\":{\"annotations\":{\"storageclass.kubernetes.io/is-default-class\":\"true\"}}}'",
            "kubectl get storageclass",
        ],
        "description": "Install local-path as the default StorageClass. Required in EVERY topology before deploying anything with a PVC (Open WebUI has one): nothing else installs a StorageClass unless the full KServe stack is deployed. Use this for MULTI-NODE clusters; for a single-node cluster use k8s_single_node_prep, which also removes the control-plane taint.",
    },
    "k8s_single_node_prep": {
        "commands": [
            "kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule- || true",
            "kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.31/deploy/local-path-storage.yaml",
            "kubectl patch storageclass local-path -p '{\"metadata\":{\"annotations\":{\"storageclass.kubernetes.io/is-default-class\":\"true\"}}}'",
            "kubectl get nodes -o wide",
            "kubectl get storageclass",
        ],
        "description": "Prepare a SINGLE-NODE cluster for workloads: remove the control-plane taint and install local-path as the default StorageClass. Without both, pods stay Pending forever with no error explaining why. Use this ONLY when the cluster has no workers - with workers joined, keep the taint (so the control plane stays reserved) and run k8s_storage_default instead.",
    },
    "gpu_driver_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/installGpuDriver.sh -o /tmp/installGpuDriver.sh && bash /tmp/installGpuDriver.sh --no-reboot",
        ],
        "description": "Install the NVIDIA driver on a GPU node. --no-reboot is used deliberately: the installer reboots by default, which drops the SSH channel mid-command and looks like a failure. Reboot separately afterwards with gpu_node_reboot, then confirm with gpu_driver_check.",
    },
    "gpu_node_reboot": {
        "commands": [
            "sudo systemctl reboot || sudo reboot",
        ],
        "description": "Reboot a node so a freshly installed GPU driver loads. The command drops the connection by design; wait about a minute, then run gpu_driver_check.",
    },
    "gpu_driver_check": {
        "commands": [
            "nvidia-smi || rocm-smi || echo 'no accelerator driver responded'",
        ],
        "description": "Confirm the accelerator driver is loaded and the card is visible. Run this before installing vLLM - vLLM on a node without a working driver fails much later and less clearly.",
    },
    "vllm_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployvLLM.sh -o /tmp/deployvLLM.sh && bash /tmp/deployvLLM.sh",
        ],
        "description": "Install vLLM on a GPU node, directly on the VM - no Kubernetes involved. Requires a working NVIDIA driver (gpu_driver_check). This is the simple path; the KServe route (k8s_kserve_stack) is only worth it when the cluster is wanted for other reasons.",
        "ports": ["8000/tcp"],
        "min_disk_gb": 100,
    },
    "vllm_serve": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/servevLLM.sh -o /tmp/servevLLM.sh && bash /tmp/servevLLM.sh --model '{{model_id}}' --port 8000",
            "curl -s -m 20 http://127.0.0.1:8000/v1/models | head -c 400",
        ],
        "description": "Serve a Hugging Face model with vLLM on port 8000, OpenAI-compatible. model_id is the HF repo, e.g. Qwen/Qwen2.5-1.5B-Instruct, which fits a 16 GB T4 in half precision. The model is downloaded on first run, so allow time. Verifies /v1/models afterwards.",
        "ports": ["8000/tcp"],
        "min_disk_gb": 100,
    },
    "ollama_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh",
        ],
        "description": "Install Ollama on a GPU node and serve it on port 3000 (not the 11434 default). Run gpu_driver_install first, or it falls back to CPU. Pull a model afterwards with ollama_pull. Open port 3000 to reach the API from outside.",
        "ports": ["3000/tcp"],
        "min_disk_gb": 50,
    },
    "ollama_pull": {
        "commands": [
            "export OLLAMA_HOST=127.0.0.1:3000; ollama pull {{model}} && ollama list",
        ],
        "description": "Download a model into an Ollama installed by ollama_install. OLLAMA_HOST is set because that install moves the server to port 3000 while the CLI still defaults to 11434 - without it every ollama command fails with 'could not connect to ollama server'. Size the model to the card: a 14B at Q4 is ~9 GB, a 32B ~20 GB. Set model, e.g. qwen2.5:14b.",
        "ports": ["3000/tcp"],
        "min_disk_gb": 50,
    },
    "open_webui_ollama": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh -o /tmp/deployOpenWebUI.sh && bash /tmp/deployOpenWebUI.sh ollama {{ollama_url}}",
            "sleep 20; curl -s -o /dev/null -w 'local check: %{http_code}\\n' http://127.0.0.1/",
        ],
        "description": "Run Open WebUI in Docker on port 80 against an Ollama backend. ollama_url is reached from inside the container, so for an Ollama on the same node use the Docker bridge address http://172.17.0.1:3000 - localhost there is the container itself. Open port 80 in the security group.",
        "ports": ["80/tcp"],
    },
    "open_webui_vllm": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh -o /tmp/deployOpenWebUI.sh && bash /tmp/deployOpenWebUI.sh vllm '{{vllm_url}}'",
            "sleep 10; curl -s -o /dev/null -w 'local check: %{http_code}\n' http://127.0.0.1/",
        ],
        "description": "Run Open WebUI in Docker on port 80, pointed at a vLLM endpoint. vllm_url is the OpenAI-compatible base, e.g. http://<gpu-node-ip>:8000/v1 - use the node's own address when the UI and vLLM share a node. Open port 80 in the security group to reach it.",
        "ports": ["80/tcp"],
    },
    "k8s_kserve_stack": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/deploy-kserve-stack.sh -o /tmp/deploy-kserve-stack.sh && bash /tmp/deploy-kserve-stack.sh",
        ],
        "description": "Install the model-serving stack on the control plane: a default StorageClass, Helm, the NVIDIA GPU Operator, cert-manager and KServe in RawDeployment mode. Takes 10-20 minutes. Required before serve_vllm_model. Skip it only if the cluster already has KServe.",
        "min_disk_gb": 80,
    },
    "serve_vllm_model": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/serve-vllm-model.sh -o /tmp/serve-vllm-model.sh && bash /tmp/serve-vllm-model.sh --model '{{model_id}}' --name '{{isvc_name}}'",
            "kubectl get isvc",
        ],
        "description": "Serve a Hugging Face model with vLLM through KServe. model_id is the HF repo, e.g. Qwen/Qwen2.5-1.5B-Instruct for a small one that fits a single T4. isvc_name is the InferenceService name Open WebUI will connect to. Needs an NVIDIA GPU node and k8s_kserve_stack already installed; AMD accelerators will not work, the stack is CUDA-based.",
        "defaults": {"isvc_name": "llm"},
        "ports": ["30800/tcp"],
        "min_disk_gb": 100,
    },
    "k8s_open_webui": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/deploy-open-webui-kserve.sh -o /tmp/deploy-open-webui.sh",
            "BACKEND='{{backend_url}}'; case \"$BACKEND\" in http*) ;; *) BACKEND='' ;; esac; if [ -n \"$BACKEND\" ]; then echo \"Connecting Open WebUI to: $BACKEND\"; bash /tmp/deploy-open-webui.sh --nodeport {{node_port}} --backend-url \"$BACKEND\"; else echo 'No backend URL given - the UI will come up with an empty model list.'; bash /tmp/deploy-open-webui.sh --nodeport {{node_port}}; fi",
            "kubectl get svc open-webui -o wide",
            "code=$(curl -s -m 8 -o /dev/null -w '%{http_code}' http://127.0.0.1:{{node_port}}/); if [ \"$code\" != 200 ]; then echo \"NodePort did not answer ($code); restarting kube-proxy\"; kubectl -n kube-system rollout restart ds/kube-proxy > /dev/null; kubectl -n kube-system rollout status ds/kube-proxy --timeout=180s > /dev/null; sleep 5; code=$(curl -s -m 8 -o /dev/null -w '%{http_code}' http://127.0.0.1:{{node_port}}/); fi; echo \"local NodePort check: $code\"",
        ],
        "description": "Deploy Open WebUI as a NodePort service and confirm it answers locally. node_port is typically 30080. Set backend_url to an OpenAI-compatible endpoint (or several joined by ';') - get it from discover_service_endpoints() first, otherwise the UI deploys with an empty model list and chat does not answer. Requires a default StorageClass: the data PVC is a hard precondition, and nothing installs one unless the KServe stack was deployed, so run k8s_single_node_prep first. KServe itself is NOT required - the script falls back cleanly when no InferenceService exists.",
        "defaults": {"backend_url": "", "node_port": "30080"},
        "ports": ["30080/tcp"],
    },
    "publish_nodeport_via_host": {
        "commands": [
            "set -e; IFACE=$(ip route show 0.0.0.0/0 | grep -oE 'dev [^ ]+' | cut -c5- | head -1); echo \"forwarding via $IFACE\"; sudo sysctl -w net.ipv4.ip_forward=1; sudo iptables -t nat -C PREROUTING -i \"$IFACE\" -p tcp --dport {{ext_port}} -j DNAT --to-destination {{target_ip}}:{{target_port}} 2>/dev/null || sudo iptables -t nat -A PREROUTING -i \"$IFACE\" -p tcp --dport {{ext_port}} -j DNAT --to-destination {{target_ip}}:{{target_port}}; sudo iptables -C FORWARD -p tcp -d {{target_ip}} --dport {{target_port}} -j ACCEPT 2>/dev/null || sudo iptables -A FORWARD -p tcp -d {{target_ip}} --dport {{target_port}} -j ACCEPT; sudo iptables -t nat -C POSTROUTING -d {{target_ip}} -p tcp --dport {{target_port}} -j MASQUERADE 2>/dev/null || sudo iptables -t nat -A POSTROUTING -d {{target_ip}} -p tcp --dport {{target_port}} -j MASQUERADE; echo '--- active rules ---'; sudo iptables -t nat -L PREROUTING -n -v --line-numbers | grep {{ext_port}}",
        ],
        "description": "Publish a service that lives behind a private address. RUN THIS ON THE HOST that owns the public IP and can route to target_ip - never on the target itself, where DNAT to its own unreachable address becomes an infinite loop. Kept as ONE command on purpose: CB-Tumblebug runs each entry of a command list in a SEPARATE SSH session, so a shell variable set in one entry is empty in the next. The MASQUERADE is required: with DNAT alone the target replies straight to the client and that reply never passes back through this host's conntrack. Also open ext_port in the host's security group AND target_port in the target's security group. Rules are added idempotently, so re-running is safe.",
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
            "docker --version",
        ],
        "description": "Install Docker on Ubuntu/Debian systems",
    },
    "node_install": {
        "commands": [
            "echo 'Installing Node.js...'",
            "curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -",
            "sudo apt-get install -y nodejs",
            "echo 'Node.js version:'",
            "node --version",
            "echo 'NPM version:'",
            "npm --version",
        ],
        "description": "Install Node.js 18.x LTS",
    },
    "python_install": {
        "commands": [
            "echo 'Installing Python development environment...'",
            "sudo apt-get update",
            "sudo apt-get install -y python3 python3-pip python3-venv python3-dev",
            "echo 'Python version:'",
            "python3 --version",
            "echo 'Pip version:'",
            "pip3 --version",
        ],
        "description": "Install Python 3 development environment",
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
            "sudo ufw status",
        ],
        "description": "Configure basic UFW firewall rules",
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
            "sudo systemctl reload sshd",
        ],
        "description": "Apply basic security hardening measures",
    },
    "k8s_cluster_status": {
        "commands": [
            "echo '=== Nodes ===' && kubectl get nodes -o wide && echo '' && echo '=== Pods ===' && kubectl get pods -A",
        ],
        "description": "Nodes and pods of a kubeadm cluster. Run on the control plane.",
    },
    "k8s_get_kubeconfig": {
        "commands": [
            "echo '[K8S_KUBECONFIG_BASE64]' && base64 -w 0 ~/kubeconfig-external.yaml && echo ''",
        ],
        "description": "Print the kubeconfig for kubectl from outside, base64-encoded after the [K8S_KUBECONFIG_BASE64] marker. Run on the control plane; open TCP 6443 to use it.",
        "ports": ["6443/tcp"],
    },
    "k8s_get_kubeconfig_external": {
        "commands": [
            "ADDR=\"{{k8s_external_ip}}\"; PORT=\"{{k8s_api_port}}\"; [ -z \"$PORT\" ] && PORT=6443; if [ -z \"$ADDR\" ]; then echo 'ERROR: K8S_EXTERNAL_IP is required'; exit 1; fi; SANS=$(sudo openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -ext subjectAltName | tr ',' '\\n' | sed -n -e 's/.*DNS://p' -e 's/.*IP Address://p' | tr -d ' ' | paste -sd, -); case \",$SANS,\" in *\",$ADDR,\"*) echo \"API server cert already covers $ADDR\";; *) echo \"Re-issuing API server cert with SAN $ADDR ...\"; ADV=$(sudo sed -n 's/.*--advertise-address=\\([0-9.]*\\).*/\\1/p' /etc/kubernetes/manifests/kube-apiserver.yaml | head -1); TS=$(date +%s); sudo mv /etc/kubernetes/pki/apiserver.crt /etc/kubernetes/pki/apiserver.crt.bak.$TS; sudo mv /etc/kubernetes/pki/apiserver.key /etc/kubernetes/pki/apiserver.key.bak.$TS; sudo kubeadm init phase certs apiserver --apiserver-advertise-address \"$ADV\" --apiserver-cert-extra-sans \"$SANS,$ADDR\" || { echo 'ERROR: cert re-issue failed'; exit 1; }; sudo mv /etc/kubernetes/manifests/kube-apiserver.yaml /tmp/kube-apiserver.yaml; sleep 8; sudo mv /tmp/kube-apiserver.yaml /etc/kubernetes/manifests/kube-apiserver.yaml; for i in $(seq 1 60); do kubectl get --raw /healthz >/dev/null 2>&1 && break; sleep 3; done; echo 'API server restarted with the new cert';; esac; KC=$HOME/kubeconfig-external.yaml; cp $HOME/.kube/config $KC; kubectl --kubeconfig=$KC config set-cluster \"$(kubectl --kubeconfig=$KC config view -o jsonpath='{.clusters[0].name}')\" --server=\"https://$ADDR:$PORT\" >/dev/null; chmod 600 $KC; kubectl --kubeconfig=$KC get nodes >/dev/null 2>&1 && echo \"Verified: https://$ADDR:$PORT is reachable from this node\" || echo \"NOTE: https://$ADDR:$PORT not reachable from this node - expected when the address is a NAT/port-forward entry point; forward $PORT to this node's :6443 and open it in the security group\"; echo ''; echo '[K8S_KUBECONFIG_BASE64]'; base64 -w 0 $KC; echo ''; printf '$$FILEPATH[Kubeconfig for %s:%s](%s)\\n' \"$ADDR\" \"$PORT\" \"$KC\"",
        ],
        "description": "Kubeconfig for an address the cluster was not created with (NAT, a port-forward, a nested cloud). Re-issues the API server certificate with k8s_external_ip added and prints the kubeconfig base64-encoded. Run on the control plane.",
        "defaults": {"k8s_api_port": "6443"},
        "ports": ["6443/tcp"],
    },
    "k8s_gpu_status": {
        "commands": [
            "echo '=== GPU Operator Pods ==='; kubectl get pods -n gpu-operator 2>/dev/null || echo '  (GPU Operator not installed or namespace not found)'; echo ''; echo '=== GPU Resources per Node ==='; kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpu' 2>/dev/null || echo '  (no GPU resources detected)'",
        ],
        "description": "GPU Operator pods and the GPUs each node offers. Run on the control plane.",
    },
    "kserve_status": {
        "commands": [
            "echo '=== InferenceServices ==='; kubectl get isvc 2>/dev/null || echo '  (KServe not installed)'; echo ''; echo '=== Predictor Pods ==='; kubectl get pods -l serving.kserve.io/inferenceservice -o wide 2>/dev/null; echo ''; echo '=== GPU Resources ==='; kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpu' 2>/dev/null; echo ''; echo '=== External APIs (per served LLM) ==='; APIS=$(kubectl get svc -o jsonpath='{range .items[*]}{.metadata.name}{\" \"}{.spec.ports[0].nodePort}{\"\\n\"}{end}' 2>/dev/null | grep -- '-api '); if [ -n \"$APIS\" ]; then echo \"$APIS\" | sed 's|\\(.*\\)-api \\(.*\\)|  \\1: http://$$Func(GetPublicIP(target=this)):\\2/openai/v1|'; else echo '  (not exposed; serve with --nodeport to expose)'; fi; echo ''; echo '=== Model Logs (tail) ==='; kubectl logs -l serving.kserve.io/inferenceservice --tail=5 2>/dev/null || echo '  (no model pods yet)'",
        ],
        "description": "InferenceServices, predictor pods and GPU allocation of a KServe cluster. Run on the control plane.",
    },
    "kserve_gpu_timeslice": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/config-gpu-timeslicing.sh -o /tmp/config-gpu-timeslicing.sh && NODE=\"{{gpu_timeslice_node}}\"; ARGS=(--replicas {{gpu_timeslice_replicas}}); [ -n \"$NODE\" ] && ARGS+=(--node \"$NODE\"); bash /tmp/config-gpu-timeslicing.sh \"${ARGS[@]}\"",
        ],
        "description": "Let several models share one GPU by time-slicing (no memory isolation, so cap each model's GPU share). Run on the control plane after k8s_kserve_stack. gpu_timeslice_node limits it to one node; empty slices every GPU node.",
        "defaults": {"gpu_timeslice_node": "", "gpu_timeslice_replicas": "2"},
    },
    "kserve_gpu_mig": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/config-gpu-mig.sh | bash -s -- --profile {{mig_profile}}",
        ],
        "description": "Partition A100/H100 GPUs into isolated MIG slices. Run on the control plane after k8s_kserve_stack.",
        "defaults": {"mig_profile": "all-3g.40gb"},
    },
    "ray_head": {
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/ray/ray-head-setup.sh",
            "chmod +x ~/ray-head-setup.sh",
            "~/ray-head-setup.sh -i $$Func(GetPublicIP(target=this))",
        ],
        "description": "Start a Ray head node (dashboard on port 8265).",
        "ports": ["8265/tcp", "6379/tcp", "10001/tcp"],
    },
    "ray_worker": {
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/ray/ray-worker-setup.sh",
            "chmod +x ~/ray-worker-setup.sh",
            "~/ray-worker-setup.sh -i $$Func(GetPublicIP(target=this)) -h $$Func(GetPublicIP(target={{head_node}}))",
        ],
        "description": "Join a node to a Ray cluster. head_node names the head as '<infraId>.<nodeId>', e.g. 'mc-ray.g1-1'.",
    },
    "xonotic_server": {
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/xonotic/startServer.sh; chmod +x ~/startServer.sh",
            "sudo ~/startServer.sh Cloud-Barista-$$Func(GetInfraId()) 26000 8 8",
            "echo '$$Func(GetPublicIP(target=this,postfix=:26000))'",
        ],
        "description": "Start a Xonotic game server on port 26000 - the classic multi-region latency demo.",
        "ports": ["26000/udp", "26000/tcp"],
    },
    "nginx_server": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/nginx/startServer.sh | bash -s -- --ip $$Func(GetPublicIP(target=this))",
            "which unzip || sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y unzip; f=$(ls /home/cb-user/*.zip /home/cb-user/*.tar.gz /home/cb-user/*.tgz /home/cb-user/*.tar.bz2 2>/dev/null | head -1); [ -n \"$f\" ] && case \"$f\" in *.zip) sudo unzip -o \"$f\" -d /var/www/html/ ;; *.tar.gz|*.tgz) sudo tar -xzf \"$f\" -C /var/www/html/ ;; *.tar.bz2) sudo tar -xjf \"$f\" -C /var/www/html/ ;; esac || echo 'No archive found in /home/cb-user/, skipping extraction.'",
            "echo 'Access to $$Func(GetPublicIP(target=this, prefix=http://))'",
        ],
        "description": "Install nginx and serve a page identifying the node. An archive placed in /home/cb-user (transfer_file_infra) is unpacked into the web root.",
        "ports": ["80/tcp"],
    },
    "jitsi": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/jitsi/startServer.sh | sudo bash -s -- {{dns_domain}} {{email_address}}",
        ],
        "description": "Install Jitsi Meet. dns_domain must already resolve to the node (manage_dns_record); email_address is used for the certificate.",
        "ports": ["80/tcp", "443/tcp", "10000/udp", "3478/udp", "5349/tcp"],
        "min_disk_gb": 30,
    },
    "netdata_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployNetdataMonitor.sh | sh",
            "echo '$$Func(GetPublicIP(target=this, prefix=http://, postfix=:19999))'",
        ],
        "description": "Install the Netdata live monitor (UI on port 19999, unauthenticated - restrict the CIDR).",
        "ports": ["19999/tcp"],
    },
    "netdata_status": {
        "commands": [
            "sudo systemctl status netdata --no-pager",
        ],
        "description": "Service state of Netdata.",
    },
    "weavescope": {
        "commands": [
            "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/weavescope/startServer.sh",
            "chmod +x ~/startServer.sh",
            "sudo ~/startServer.sh $$Func(GetPublicIPs(separator=' ')) $$Func(GetPrivateIPs(separator=' '))",
        ],
        "description": "Install Weave Scope to visualise the hosts of the Infra (UI on port 4040, unauthenticated - restrict the CIDR). Run on every node.",
        "ports": ["4040/tcp"],
    },
    "stress_test": {
        "commands": [
            "sudo apt install -y stress > /dev/null; stress -c 16 -t 60",
        ],
        "description": "Load 16 CPU workers for 60 seconds - useful to watch monitoring react.",
    },
    "cross_cloud_nat": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setup-cross-cloud-nat.sh -o ~/setup-cross-cloud-nat.sh && chmod +x ~/setup-cross-cloud-nat.sh",
            "sudo ~/setup-cross-cloud-nat.sh pub=$$Func(GetPublicIPs(separator=,)) priv=$$Func(GetPrivateIPs(separator=,))",
        ],
        "description": "Let the nodes of a multi-cloud Infra reach each other by private address, by NAT between each node's public and private IP. Run on every node.",
    },
    "wireguard_mesh": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/k8s/setup-wireguard-mesh.sh -o ~/setup-wireguard-mesh.sh && chmod +x ~/setup-wireguard-mesh.sh",
            "sudo ~/setup-wireguard-mesh.sh --nodes \"{{nodes_mapping}}\"",
        ],
        "description": "Build a WireGuard mesh between nodes in different clouds, e.g. before a cross-cloud Kubernetes cluster. nodes_mapping is 'public_ip:wg_ip' pairs, comma-separated ('54.1.1.1:10.200.0.1,35.2.2.2:10.200.0.2'). Run on every node; open UDP 51820.",
        "ports": ["51820/udp"],
    },
    "kolla_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/kolla/1.installKolla.sh -o /tmp/installKolla.sh && bash /tmp/installKolla.sh --csp-name openstack-$$Func(GetInfraId())-$$Func(GetNodeId()) --latitude $$Func(GetLocationLatitude()) --longitude $$Func(GetLocationLongitude()) --location \"$$Func(GetLocationDisplay())\"",
            "echo 'Kolla-Ansible installed. Horizon: $$Func(GetPublicIP(target=this, prefix=http://, postfix=/))'",
        ],
        "description": "Install OpenStack with Kolla-Ansible on a BARE-METAL node - closer to a production layout than DevStack. Takes 30-60 minutes: pass timeout_minutes=120 and run_async=True. Prints the registration snippets when it finishes.",
        "ports": ["80/tcp", "5000/tcp", "6080/tcp", "8774/tcp", "8776/tcp", "8780/tcp", "9292/tcp", "9696/tcp", "9876/tcp"],
        "min_disk_gb": 100,
    },
    "kolla_registration_info": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/kolla/2.getRegistrationInfo.sh -o /tmp/getKollaInfo.sh && bash /tmp/getKollaInfo.sh --csp-name openstack-$$Func(GetInfraId())-$$Func(GetNodeId()) --latitude $$Func(GetLocationLatitude()) --longitude $$Func(GetLocationLongitude()) --location \"$$Func(GetLocationDisplay())\"",
        ],
        "description": "Re-print the registration snippets (Keystone endpoint, ProjectID, region/zone) of a Kolla-Ansible OpenStack.",
    },
    "kolla_update_endpoints": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/kolla/3.updateEndpoints.sh -o /tmp/updateKollaEndpoints.sh && bash /tmp/updateKollaEndpoints.sh --csp-name openstack-$$Func(GetInfraId())-$$Func(GetNodeId())",
        ],
        "description": "Repoint the Kolla-Ansible service catalog after the host's public IP changed. Re-register the credential afterwards.",
    },
    "kolla_clean": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/kolla/4.cleanKolla.sh -o /tmp/cleanKolla.sh && bash /tmp/cleanKolla.sh",
        ],
        "description": "Tear down a failed or stale Kolla-Ansible deployment so it can be installed again.",
    },
    "mcp_registry_db": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/mcp/deploy-registry-db.sh | bash",
        ],
        "description": "Model-registry demo 1/6: PostgreSQL with a sample model catalog. Run on the control plane of a kubeadm cluster with a default StorageClass.",
    },
    "mcp_registry_backend": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/mcp/deploy-registry-backend.sh | bash",
        ],
        "description": "Model-registry demo 2/6: the REST backend. Run on the control plane.",
    },
    "mcp_registry_web": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/mcp/deploy-registry-web.sh | bash; echo ''; echo '[MODEL_REGISTRY_WEB]'; echo 'http://$$Func(GetPublicIP(target=this)):30902'",
        ],
        "description": "Model-registry demo 3/6: the web catalog on NodePort 30902. Run on the control plane.",
        "ports": ["30902/tcp"],
    },
    "mcp_servers": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/mcp/deploy-mcp-servers.sh | bash",
        ],
        "description": "Model-registry demo 4/6: the MCP adapters (catalog tools and read-only SQL). Run on the control plane.",
    },
    "mcp_agentgateway": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/mcp/deploy-agentgateway.sh | bash; echo ''; echo '[MCP_ENDPOINT (all tools)]'; echo 'http://$$Func(GetPublicIP(target=this)):30900/mcp'; echo '[MCP_ENDPOINT_REGISTRY (catalog view)]'; echo 'http://$$Func(GetPublicIP(target=this)):30900/mcp-registry'; echo '[MCP_ENDPOINT_SERVING (KServe view; needs step 8-opt)]'; echo 'http://$$Func(GetPublicIP(target=this)):30900/mcp-serving'; echo '[AGENTGATEWAY_UI]'; echo 'http://$$Func(GetPublicIP(target=this)):30901/ui/'; echo ''; echo 'Register in Claude Code:'; echo 'claude mcp add --transport http model-registry http://$$Func(GetPublicIP(target=this)):30900/mcp'; echo ''; echo 'Register in VS Code Copilot:'; echo 'Ctrl+Shift+P -> MCP: Add Server... -> HTTP -> paste an endpoint URL above, then use Copilot Chat in Agent mode'",
        ],
        "description": "Model-registry demo 5/6: agentgateway federating the adapters at http://<node>:30900/mcp. Run on the control plane.",
        "ports": ["30900/tcp", "30901/tcp"],
    },
    "mcp_e2e_test": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/mcp/test-mcp-e2e.sh | bash",
        ],
        "description": "Model-registry demo 6/6: call the tools through the gateway end to end. Run on the control plane.",
    },
    "mcp_status": {
        "commands": [
            "echo '=== MCP demo stack (namespace mcp-demo) ==='; kubectl -n mcp-demo get pods,svc 2>/dev/null || echo '  (mcp-demo namespace not found)'; echo ''; echo '=== External endpoints ==='; echo '  Web:  http://$$Func(GetPublicIP(target=this)):30902'; echo '  MCP:  http://$$Func(GetPublicIP(target=this)):30900/mcp (views: /mcp-registry, /mcp-serving)'; echo '  UI:   http://$$Func(GetPublicIP(target=this)):30901/ui/'; echo ''; echo '=== agentgateway logs (tail) ==='; kubectl -n mcp-demo logs deploy/agentgateway --tail=5 2>/dev/null || echo '  (agentgateway not deployed yet)'",
        ],
        "description": "Pods, services and external endpoints of the model-registry demo. Run on the control plane.",
    },
}

# Tool: Find HTTP endpoints served by the nodes of matching Infras
@tool()
def discover_service_endpoints(
    ns_id: str = "default",
    name_filter: str = "",
    port: int = 8000,
    api_path: str = "/v1",
    probe_path: str = "/models",
    probe: bool = True
) -> Dict:
    """
    Find an HTTP service already running on the nodes of this namespace's Infras.

    Lists the Infras, keeps those whose name contains name_filter, collects their node
    addresses, and (when probe=True) calls GET <ip>:<port><api_path><probe_path> on each.
    Probing is what separates "a VM exists" from "something is answering there", so the
    returned URL is one that has actually responded.

    The defaults describe an OpenAI-compatible LLM server such as vLLM (port 8000,
    /v1/models) because that is the common case; point them anywhere else to find any
    other service.

    Args:
        ns_id: Namespace to search (default: "default")
        name_filter: Substring an Infra name must contain, case-insensitive. Empty
            (default) considers every Infra in the namespace.
        port: Port the service listens on (default: 8000)
        api_path: Base path of the API (default: "/v1")
        probe_path: Path appended to the base URL for the health probe
            (default: "/models", the OpenAI-compatible model listing)
        probe: Contact each candidate (default: True). Set False to build the URLs
            without reaching out.

    Returns:
        endpointUrl - ";"-joined confirmed endpoints; per-endpoint items when the probe
        answered an OpenAI-style {"data": [{"id": ...}]} listing; and every candidate
        that failed, with the reason. For Open WebUI, pass endpointUrl straight to the
        k8s_open_webui script as {{backend_url}}.
    """
    infra_list = api_request("GET", f"/ns/{ns_id}/infra", params={"option": "id"})
    all_infras = (infra_list or {}).get("output") or []
    if not all_infras:
        return {"error": f"No Infra found in namespace '{ns_id}'", "endpointUrl": ""}

    needle = (name_filter or "").lower()
    matched = [i for i in all_infras if needle in i.lower()] if needle else list(all_infras)
    if not matched:
        return {
            "error": f"No Infra name contains '{name_filter}'",
            "allInfras": all_infras,
            "hint": "Pass name_filter='' to consider every Infra, or give the exact name.",
            "endpointUrl": "",
        }

    confirmed, rejected = [], []
    for infra_id in matched:
        try:
            access = _infra_access_info(ns_id, infra_id, show_ssh_key=False)
            public_ips, private_ips = _extract_infra_ips(access)
        except Exception as e:
            rejected.append({"infra": infra_id, "reason": f"could not read access info: {e}"})
            continue

        if not public_ips and not private_ips:
            # Most often the nodes are stopped or terminated: access info still lists the
            # Infra but carries no addresses. Say so rather than dropping it silently.
            rejected.append({
                "infra": infra_id,
                "reason": "no node addresses returned - the nodes are probably not running",
                "hint": f"Check with get_infra('{ns_id}', '{infra_id}'); a Terminated or "
                        f"Suspended node has no reachable address.",
            })
            continue

        # A service reachable only on a private address is no use to a caller outside that
        # network, so prefer public and fall back only when there is none.
        for ip in (public_ips or private_ips):
            url = f"http://{ip}:{port}{api_path}"
            if not probe:
                confirmed.append({"infra": infra_id, "endpoint": url, "items": "(not probed)"})
                continue
            try:
                r = requests.get(f"{url}{probe_path}", timeout=(5, 15))
                if r.status_code != 200:
                    rejected.append({"infra": infra_id, "endpoint": url,
                                     "reason": f"{probe_path} returned HTTP {r.status_code}"})
                    continue
                # An OpenAI-style listing names what is being served; anything else just
                # confirms the endpoint answers.
                items = None
                try:
                    items = [m.get("id") for m in (r.json().get("data") or []) if m.get("id")]
                except Exception:
                    pass
                confirmed.append({"infra": infra_id, "endpoint": url,
                                  "items": items if items else f"HTTP {r.status_code}"})
            except Exception as e:
                rejected.append({"infra": infra_id, "endpoint": url,
                                 "reason": f"not reachable: {type(e).__name__}"})

    endpoint_url = ";".join(c["endpoint"] for c in confirmed)
    result = {
        "endpointUrl": endpoint_url,
        "confirmed": confirmed,
        "searchedInfras": matched,
        "rejected": rejected,
    }
    if endpoint_url:
        result["nextStep"] = ("Pass endpointUrl wherever the consumer expects the service - for "
                              "Open WebUI, as the {{backend_url}} variable of k8s_open_webui.")
    else:
        result["warning"] = "No endpoint answered. Every candidate is listed under rejected."
        result["hint"] = (f"Check the service is running and that port {port} is open in the "
                          f"security group of the Infra hosting it (add_security_group_rules), "
                          f"then re-run.")
    return result


# Tool: List available predefined scripts
@tool()
def list_predefined_scripts(name: Optional[str] = None) -> Dict:
    """
    The predefined scripts execute_command_infra(script_name=...) can run.

    The same setups CB-MapUI offers: GPU drivers, vLLM, Ollama, Open WebUI, Kubernetes on
    VMs, KServe, OpenStack (DevStack, Kolla), Ray, cross-cloud networking and demo services.

    Args:
        name: Return this script in full

    Returns:
        scripts: name -> what it does. With name: its commands, each {{placeholder}} and
        whether it is required, the ports the service listens on, and the disk it needs.
    """
    if name:
        script = PREDEFINED_SCRIPTS.get(name)
        if not script:
            return {"error": f"unknown name {name!r}; expected one of "
                             f"{', '.join(sorted(PREDEFINED_SCRIPTS))}"}
        automatic = _AUTO_TEMPLATE_VARS + ("infra_id", "ns_id")
        defaults = script.get("defaults", {})
        placeholders = {}
        for token in sorted({m for c in script["commands"]
                             for m in re.findall(r"\{\{(\w+)\}\}", c)}):
            if token in defaults:
                placeholders[token] = f"optional, default {defaults[token]!r}"
            elif token in automatic:
                placeholders[token] = "filled automatically"
            else:
                placeholders[token] = "required"
        out = {"name": name, "description": script["description"],
               "commands": script["commands"], "placeholders": placeholders}
        for key in ("ports", "min_disk_gb", "timeout_minutes"):
            if key in script:
                out[key] = script[key]
        return out
    def first_sentence(text):
        head = str(text).split(". ")[0].rstrip(".")
        return head if len(head) <= 140 else head[:137] + "..."
    return {"scripts": {k: first_sentence(v["description"]) for k, v in PREDEFINED_SCRIPTS.items()},
            "count": len(PREDEFINED_SCRIPTS),
            "next": "list_predefined_scripts(name=...) for a script's full notes, commands "
                    "and placeholders; execute_command_infra(script_name=...) runs it"}


# Tool: Put a file on nodes
@tool(mutating=True)
def transfer_file_infra(
    ns_id: str,
    infra_id: str,
    target_path: str,
    content: Optional[str] = None,
    file_name: Optional[str] = None,
    file_path: Optional[str] = None,
    nodegroup_id: Optional[str] = None,
    node_id: Optional[str] = None
) -> Dict:
    """
    Place a file on the nodes of an Infra (SCP through the bastion, 10 MB at most).

    Give the text as content - a configuration file, a script, a manifest. For anything
    binary or large, have the node fetch it from a URL with execute_command_infra().

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        target_path: Directory on the node that receives the file, e.g. "/home/cb-user"
        content: Text to write into the file
        file_name: Name of the file on the node; required with content
        file_path: Instead of content: a file on the MCP server's own filesystem
        nodegroup_id: Only the nodes of this NodeGroup
        node_id: Only this node

    Returns:
        nodes {total, succeeded, failed} and per-node results, failed nodes first. It is
        an error only when no node received the file.
    """
    if content is None and not file_path:
        return {"error": "content is required (with file_name), or file_path"}
    if content is not None and not file_name:
        return {"error": "file_name is required with content"}
    params = {}
    if nodegroup_id:
        params["nodeGroupId"] = nodegroup_id
    if node_id:
        params["nodeId"] = node_id
    try:
        if content is not None:
            payload = (os.path.basename(file_name), content.encode("utf-8"))
        else:
            with open(file_path, "rb") as fh:
                payload = (os.path.basename(file_path), fh.read())
    except OSError as exc:
        return {"error": f"could not read {file_path}: {exc}"}
    result = api_request("POST", f"/ns/{ns_id}/transferFile/infra/{infra_id}",
                         files={"file": payload}, json_data={"path": target_path},
                         params=params or None, timeout_override=(60, 1200))
    if not isinstance(result, dict) or not isinstance(result.get("results"), list):
        return result
    rows = [{k: r[k] for k in ("nodeId", "nodeIp", "stdout", "stderr", "error") if r.get(k)}
            for r in result["results"] if isinstance(r, dict)]
    failed = [r for r in rows if r.get("error")]
    if rows and len(failed) == len(rows):
        return {"error": {"message": f"the file reached none of the {len(rows)} node(s): "
                                     f"{failed[0]['error']}",
                          "failedNodes": [r.get("nodeId") for r in failed][:NODE_LIST_LIMIT]}}
    return {"nodes": {"total": len(rows), "succeeded": len(rows) - len(failed),
                      "failed": len(failed)},
            "results": (failed + [r for r in rows if not r.get("error")])[:NODE_LIST_LIMIT]}


# Tool: Command history of one node
@tool()
def list_node_command_status(
    ns_id: str,
    infra_id: str,
    node_id: str,
    index: Optional[int] = None,
    status: Optional[List[str]] = None,
    request_id: Optional[str] = None,
    page: Optional[int] = None,
    size: Optional[int] = None,
    detail: str = "summary",
    tail_lines: int = 20
) -> Dict:
    """
    Remote command records of one node, or one record by index.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Node ID, e.g. "g1-1"
        index: Return this one record with its output
        status: Only records in these states, e.g. ["Completed", "Failed"]
        request_id: Only records of this command request
        page: Page number (with size)
        size: Records per page
        detail: "summary" (default) keeps output only for the newest record and any that
            did not finish; "full" returns every record's output.
        tail_lines: Lines of stdout/stderr kept per record (default 20)

    Returns:
        commands: the records, oldest first.
    """
    base = f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}/commandStatus"
    if index is not None:
        record = api_request("GET", f"{base}/{index}")
        if _validate_detail(detail) == "full" or not isinstance(record, dict) \
                or "error" in record:
            return record
        return _trim_command_record(record, tail_lines)
    params = {}
    if status:
        params["status"] = status
    if request_id:
        params["xRequestId"] = request_id
    if size is not None:
        params["limit"] = size
    if page is not None:
        params["offset"] = (max(page, 1) - 1) * (size or 50)
    result = api_request("GET", base, params=params)
    if _validate_detail(detail) == "full" or not isinstance(result, dict):
        return result
    cmds = result.get("commands")
    if not isinstance(cmds, list):
        return result
    out = dict(result)
    out["commands"] = [
        _trim_command_record(c, tail_lines)
        if i >= len(cmds) - 1 or (isinstance(c, dict) and c.get("status") not in ("Completed", None))
        else _drop_command_output(c)
        for i, c in enumerate(cmds)
    ]
    return out


# Tool: Clear all node command status
@tool(mutating=True)
def clear_all_node_command_status( ns_id: str, infra_id: str, node_id: str , confirm: bool = False) -> Dict:
    """
    Clear (delete) all remote command execution status records for a specific node.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Node ID (e.g., g1-1)
        confirm: Must be True to proceed - the command history for this node is discarded,
            including the output of failed runs.

    Returns:
        Result of the clear operation
    """
    guard = _require_confirmation(confirm, "clearing command history", "the record of what ran on this node is lost, including failure output")
    if guard:
        return guard
    return api_request("DELETE", f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}/commandStatusAll")


# Tool: Health of an Infra and what to do about failures
@tool()
def diagnose_infra(ns_id: str, infra_id: str) -> Dict:
    """
    Health of an Infra and what to do about its failures.

    Checks the live node states, the provider's message for each Failed node, the post
    script and init script outcome, and the server's retry plan - which failed nodes can
    be re-created in place and why the others cannot. Changes nothing.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID

    Returns:
        status, health ("healthy" | "in_progress" | "degraded" | "failed"), statusCount,
        failedNodes, and when present postScript, initScript and retryPlan. `next` lists
        the calls that address what was found.
    """
    view = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}", params={"option": "status"})
    if not isinstance(view, dict) or "error" in view:
        return view
    status = view.get("status")
    if not isinstance(status, dict):
        status = view
    nodes = status.get("node") or []
    counts: Dict[str, int] = {}
    failed, in_progress = [], []
    for node in nodes:
        state = node.get("status") or "Undefined"
        counts[state] = counts.get(state, 0) + 1
        if state in ("Failed", "Undefined"):
            failed.append({k: node[k] for k in ("id", "status", "systemMessage") if node.get(k)})
        elif state not in ("Running", "Suspended", "Terminated"):
            in_progress.append(node.get("id"))
    running = counts.get("Running", 0)
    if failed:
        health = "degraded" if running else "failed"
    elif in_progress:
        health = "in_progress"
    else:
        health = "healthy"

    out: Dict[str, Any] = {"infraId": infra_id, "status": status.get("status"),
                           "health": health, "nodes": len(nodes), "statusCount": counts}
    steps = []
    if failed:
        out["failedNodes"] = failed[:NODE_LIST_LIMIT]
    if in_progress:
        out["inProgressNodes"] = in_progress[:NODE_LIST_LIMIT]
        steps.append("nodes are still changing state - check again in a minute")

    # Post script outcome: nodes can be Running while their bootstrap failed
    record = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}") if len(nodes) <= 1000 else None
    post_status = (record or {}).get("postScriptStatus") if isinstance(record, dict) else None
    if post_status and post_status != "None":
        phases = []
        for phase in record.get("postScriptResults") or []:
            if phase.get("status") in ("Completed", None):
                continue
            errors = [{"nodeId": r.get("nodeId"), "error": r.get("error")}
                      for r in ((phase.get("results") or {}).get("results") or [])
                      if r.get("error")]
            phases.append({"phase": phase.get("phase"), "target": phase.get("target"),
                           "status": phase.get("status"),
                           "failedNodes": errors[:NODE_LIST_LIMIT]})
        out["postScript"] = {"status": post_status}
        if phases:
            out["postScript"]["failedPhases"] = phases
        if post_status in ("Failed", "CompletedWithErrors", "Interrupted"):
            steps.append("the post script did not finish cleanly: re-run the failing phase's "
                         "commands with execute_command_infra() on those nodes")

    # First-boot script results, reported by the nodes themselves
    init = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/initScript",
                       params={"option": "summary"})
    if isinstance(init, dict) and "error" not in init \
            and init.get("total", 0) > init.get("notApplicable", 0):
        out["initScript"] = {k: init.get(k, 0) for k in
                             ("ok", "fail", "initializing", "pending", "noReport")}
        if init.get("fail") or init.get("noReport"):
            steps.append("get_init_script_status(detail='full', status='fail') names the "
                         "nodes whose init script failed and the failing step")

    if failed:
        review = api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/retryFailedNodesReview",
                             json_data={"targets": []})
        if isinstance(review, dict) and "error" not in review:
            out["retryPlan"] = {
                "retriable": review.get("retriableCount", 0),
                "notRetriable": review.get("notRetriableCount", 0),
                "costPerHourIfAll": review.get("costPerHourIfAll"),
                "plans": [{k: plan[k] for k in ("nodeId", "action", "reason", "escalation",
                                                "zone", "siblingZone") if plan.get(k)}
                          for plan in (review.get("plans") or [])[:NODE_LIST_LIMIT]],
            }
            if review.get("retriableCount"):
                steps.append("retry_failed_nodes() re-creates the retriable nodes in place "
                             "(preview first; dry_run=False, confirm=True runs it)")
        steps.append("control_infra(action='refine') drops the Failed nodes and keeps the rest")
        if not running:
            steps.append("nothing is running: recommend_alternative_node_config() finds "
                         "another spec or region, or terminate_infra() and start over")
    if steps:
        out["next"] = steps
    return out


# Helper function: Extract node IPs from an InfraAccessInfo response
def _extract_infra_ips(access_info: Dict) -> tuple:
    """Return (public_ips, private_ips) from GET .../infra/{id}?option=accessinfo."""
    public_ips, private_ips = [], []
    nodegroups = access_info.get("InfraNodeGroupAccessInfo") or access_info.get("infraNodeGroupAccessInfo") or []
    for ng in nodegroups:
        nodes = ng.get("NodeAccessInfo") or ng.get("nodeAccessInfo") or []
        for node in nodes:
            if node.get("publicIP"):
                public_ips.append(node["publicIP"])
            if node.get("privateIP"):
                private_ips.append(node["privateIP"])
    return public_ips, private_ips

# Helper function: Fill {{placeholder}} slots in predefined scripts and commands
_AUTO_TEMPLATE_VARS = ("public_ip", "public_ips_space", "public_ips_comma", "private_ips_space")

def _substitute_template_variables(commands: List[str], variables: Optional[Dict[str, str]],
                                   ns_id: str, infra_id: str) -> List[str]:
    """Replace {{name}} in each command. IP placeholders are resolved from the Infra."""
    values = dict(variables or {})
    values.setdefault("infra_id", infra_id)
    values.setdefault("ns_id", ns_id)

    joined = " ".join(commands)
    if any(f"{{{{{name}}}}}" in joined and name not in values for name in _AUTO_TEMPLATE_VARS):
        try:
            public_ips, private_ips = _extract_infra_ips(
                _infra_access_info(ns_id, infra_id, show_ssh_key=False))
            if public_ips:
                values.setdefault("public_ip", public_ips[0])
            values.setdefault("public_ips_space", " ".join(public_ips))
            values.setdefault("public_ips_comma", ",".join(public_ips))
            values.setdefault("private_ips_space", " ".join(private_ips))
        except Exception:
            pass  # leave the placeholder in place rather than failing the whole call

    for name, value in values.items():
        commands = [c.replace(f"{{{{{name}}}}}", str(value)) for c in commands]
    return commands


# Tool: Credential holders
@tool()
def list_credential_holders(holder_id: Optional[str] = None) -> Dict:
    """
    List the credential holders registered in the system, or fetch one.

    A holder is one set of CSP credentials (a team, an environment, a user), possibly for
    several providers. get_connections(credential_holder=...) lists a holder's connections.

    Args:
        holder_id: Return just this holder

    Returns:
        credentialHolderList: per holder its providers, connectionCount,
        verifiedConnectionCount and isDefault.
    """
    if holder_id:
        return api_request("GET", f"/credentialHolder/{holder_id}")
    return api_request("GET", "/credentialHolder")


#####################################
# Prompts
#####################################

def _prompt(task, *topics):
    """A task statement followed by the usage guides it relies on."""
    guides = "\n\n".join(f"## {topic}\n{_USAGE_GUIDES[topic]}" for topic in topics)
    return (f"{task}\n\nWork through the CB-Tumblebug MCP tools. Ask before anything that "
            f"creates, changes or deletes resources, and report what was actually done.\n\n"
            f"{guides}")


@mcp.prompt()
def provision_infrastructure(requirement: str = "") -> str:
    """Plan, review and create a multi-cloud Infra from a requirement"""
    return _prompt(
        f"Provision infrastructure for this requirement: {requirement or '(ask the user)'}\n"
        "Choose specs and images, review the plan, show the user the node groups and the "
        "hourly cost, and create it only after they agree.", "provision")


@mcp.prompt()
def run_remote_commands(infra_id: str = "", task: str = "") -> str:
    """Run commands or a predefined script on the nodes of an Infra"""
    return _prompt(
        f"On Infra {infra_id or '(ask which one)'}, do this: {task or '(ask the user)'}\n"
        "Check list_predefined_scripts() before writing commands by hand.", "commands")


@mcp.prompt()
def serve_llm_on_gpu(model: str = "") -> str:
    """Stand up a GPU node and serve an LLM on it"""
    return _prompt(
        f"Serve the model {model or '(ask the user which model)'} on a GPU node and report "
        "the endpoint that answers.", "gpu", "provision")


@mcp.prompt()
def diagnose_and_recover(infra_id: str = "") -> str:
    """Find out why an Infra is unhealthy and recover it"""
    return _prompt(
        f"Diagnose Infra {infra_id or '(ask which one)'} and propose a recovery. Start with "
        "diagnose_infra().", "recovery")


@mcp.prompt()
def clean_up(ns_id: str = "default") -> str:
    """Remove an Infra and everything it left behind"""
    return _prompt(
        f"List what exists in namespace {ns_id}, ask the user what to remove, remove it, "
        "and verify nothing billable is left.", "cleanup")


def _compact_published_schemas():
    try:
        registered = asyncio.run(mcp.list_tools())
    except RuntimeError:
        return  # imported inside a running event loop: leave the schemas as generated
    for entry in registered:
        _compact_schema(entry.parameters)


_compact_published_schemas()


if __name__ == "__main__":
    # Map our log level to FastMCP/uvicorn log level
    fastmcp_log_level = "warning" if log_level_value > logging.DEBUG else "info"

    mcp.run(
        transport="http",
        host=host,
        port=port,
        path="/mcp",
        log_level=fastmcp_log_level,
        # LB/gateway-friendly (aligned with the stateless MCP spec 2026-07-28)
        stateless_http=True,
    )