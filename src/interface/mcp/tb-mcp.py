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
import functools
import inspect
import base64
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

# Initialize MCP server
# mask_error_details=True: hide internal exception details from MCP clients
# (only generic error messages are returned). Errors are still logged server-side.
mcp = FastMCP("cb-tumblebug", mask_error_details=True)


def tool(*d_args, redact=True, mutating=False, **d_kwargs):
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
    """
    def decorate(fn):
        @functools.wraps(fn)
        def wrapper(*args, **kwargs):
            started = time.monotonic()
            own = fn.__code__.co_varnames[:fn.__code__.co_argcount]
            session_id = (kwargs.get("session_id") if "session_id" in own
                          else kwargs.pop("session_id", None))
            idem_key = None
            if mutating and "idempotency_key" not in own:
                idem_key = kwargs.pop("idempotency_key", None)

            if idem_key:
                cached = _state_get(f"{STATE_PREFIX}/idempotency/{fn.__name__}/{idem_key}")
                if cached is not None:
                    cached["_idempotent_replay"] = True
                    return ToolResult(content=[TextContent(
                        type="text",
                        text=json.dumps(cached, ensure_ascii=False, default=str))])

            payload = fn(*args, **kwargs)

            if mutating:
                failed = isinstance(payload, dict) and "error" in payload
                if idem_key and not failed:
                    _state_put(f"{STATE_PREFIX}/idempotency/{fn.__name__}/{idem_key}",
                               payload, ttl_seconds=IDEMPOTENCY_TTL_SECONDS)
                if session_id:
                    _journal(session_id, {
                        "tool": fn.__name__,
                        "arguments": {k: v for k, v in kwargs.items() if k != "confirm"},
                        "outcome": "error" if failed else "ok",
                        "summary": str(payload)[:400],
                        "at": datetime.now().isoformat(timespec="seconds"),
                    })
            if isinstance(payload, ToolResult):
                return payload
            # Redaction is applied here rather than per tool so a newly added tool cannot
            # leak by omission. get_ssh_private_key opts out deliberately.
            if redact:
                payload = _redact_secrets(payload)
            failure = _as_structured_error(payload)
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
        base = inspect.signature(fn)
        # A tool that already declares one of these keeps its own (resume_session takes a
        # session_id as its subject, not as journal metadata).
        wanted = ["session_id"] + (["idempotency_key"] if mutating else [])
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
    ("timeout", "TIMEOUT"),
    ("timed out", "TIMEOUT"),
    ("connection", "UPSTREAM_UNREACHABLE"),
    ("not supported", "UNSUPPORTED"),
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
    "TIMEOUT": ["retry", "the operation may still be running - check status before retrying"],
    "UPSTREAM_UNREACHABLE": ["check CB-Tumblebug is reachable", "retry"],
    "UNSUPPORTED": ["this provider or option is not implemented"],
    "FORBIDDEN": ["check credentials"],
}


def _classify_error(message):
    low = str(message).lower()

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
    # The review gate reports its marker in the detail body rather than the message.
    code = _classify_error(message)
    if code == "UPSTREAM_ERROR" and detail:
        code = _classify_error(json.dumps(detail, default=str))
    err = {"code": code, "message": message}
    if detail:
        err["detail"] = detail
    nxt = _ERROR_NEXT_STEPS.get(code)
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
    """Command history as a pointer instead of a transcript.

    Full stdout/stderr of every command ever run lives here - DevStack installs
    write 100 KB records - and it is returned on every read of the node.
    """
    if not isinstance(entries, list) or not entries:
        return {"count": 0}
    last = entries[-1]
    return {
        "count": len(entries),
        "last_status": last.get("status"),
        "last_request_id": last.get("xRequestId"),
        "last_started": last.get("startedTime"),
        "handling": sum(1 for e in entries if e.get("status") == "Handling"),
        "hint": "full output: get_command_result(request_id=...) or list_node_command_status(detail='full')",
    }


_NODE_MINIMAL = ("id", "status")
_NODE_SUMMARY = ("id", "name", "status", "publicIP", "privateIP", "sshPort", "nodeUserName",
                 "specId", "imageId", "connectionName", "nodeGroupId", "vNetId", "subnetId",
                 "securityGroupIds", "sshKeyId", "region", "systemMessage", "createdTime")


def _shape_node(node, detail):
    if not isinstance(node, dict):
        return node
    if detail == "full":
        out = dict(node)
    else:
        keys = _NODE_MINIMAL if detail == "minimal" else _NODE_SUMMARY
        out = {k: node[k] for k in keys if k in node}
        if detail == "summary" and node.get("label"):
            out["label"] = node["label"]
    if detail != "full" and "commandStatus" in node:
        out["commandStatus"] = _collapse_command_status(node.get("commandStatus"))
    return out


_INFRA_DROP_AT_SUMMARY = ("newNodeList", "configureCloudAdaptiveNetwork", "systemLabel",
                          "resourceType", "targetStatus", "targetAction")


def _shape_infra(infra, detail):
    if not isinstance(infra, dict):
        return infra
    node_key = "vm" if "vm" in infra else ("node" if "node" in infra else None)
    if detail == "full":
        out = dict(infra)
    else:
        out = {k: v for k, v in infra.items() if k not in _INFRA_DROP_AT_SUMMARY}
    if node_key:
        out[node_key] = [_shape_node(n, detail) for n in (infra.get(node_key) or [])]
    return out


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
        shaped = [{k: it[k] for k in summary_keys if k in it} for it in items if isinstance(it, dict)]
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
    return out


def _drop_command_output(rec):
    """Metadata only - the caller can fetch this record by index when it matters."""
    if not isinstance(rec, dict):
        return rec
    out = {k: v for k, v in rec.items() if k not in ("stdout", "stderr")}

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
    out["output"] = "omitted; get_node_command_status(index=...) or detail='full'"
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

# mcp = FastMCP(name="cb-tumblebug", host=host, port=port)

# Shared HTTP session: connection pooling + retry on transient connection errors
_session = requests.Session()
_session.mount("http://", requests.adapters.HTTPAdapter(
    pool_maxsize=20,
    max_retries=requests.adapters.Retry(total=3, connect=2, read=0, backoff_factor=0.8,
                                        allowed_methods=["GET"],
                                        # 429: CB-Tumblebug rate-limits bursts of reads
                                        status_forcelist=[429, 502, 503, 504])))
_session.mount("https://", requests.adapters.HTTPAdapter(pool_maxsize=20))

# Helper function: API request wrapper
def api_request(method, endpoint, json_data=None, params=None, files=None, headers=None, timeout_override=None, credential_holder=None):
    url = f"{TUMBLEBUG_API_BASE_URL}{endpoint}"
    
    # Enhanced request configuration with improved timeout handling
    # Special handling for remote command execution endpoints
    default_timeout = (60, 600)  # 10 minutes default
    
    # Extended timeout for remote command execution (up to 20 minutes)
    if "/cmd/infra/" in endpoint or timeout_override:
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
    if json_data and method.lower() in ["post", "put", "delete"]:
        # With files, send fields as multipart form data (json= would be discarded)
        request_config["data" if files else "json"] = json_data
    if files:
        request_config["files"] = files
    
    # Build headers with credential holder support
    request_headers = {
        # CB-Tumblebug records request headers, so this marks every call that arrived
        # through MCP. MapUI reads it to show which requests came from an agent rather than
        # from someone clicking - the point being visible during a demonstration, where
        # nobody is touching the map and things happen anyway.
        "X-Request-Source": "mcp",
    }
    if headers:
        request_headers.update(headers)
    
    # Add x-credential-holder header (per-request override > env default)
    effective_holder = credential_holder or TUMBLEBUG_CREDENTIAL_HOLDER
    if effective_holder:
        request_headers["x-credential-holder"] = effective_holder
    
    if request_headers:
        request_config["headers"] = request_headers
    
    logger.debug(f"Request: {method} {url}")
    if json_data and logger.isEnabledFor(logging.DEBUG):
        logger.debug(f"Request data: {json.dumps(json_data, indent=2, ensure_ascii=False)[:200]}...")
    
    try:
        if method.lower() not in ("get", "post", "put", "delete"):
            return {"error": f"Unsupported method: {method}"}
        response = _session.request(method.upper(), url, **request_config)
        
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
        if "/cmd/infra/" in endpoint:
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
    if "ns" in result:
        return {"namespaces": result["ns"]}
    return result


# Helper function: Internal get namespace (used by both get_namespace tool and other functions)
def _internal_get_namespace(ns_id: str) -> Dict:
    """Internal helper function to get specific namespace"""
    return api_request("GET", f"/ns/{ns_id}")


# Helper function: Internal create namespace (used by both create_namespace tool and other functions)
def _internal_create_namespace(name: str, description: Optional[str] = None) -> Dict:
    """Internal helper function to create a new namespace"""
    data = {
        "name": name,
        "description": description or f"Namespace {name}"
    }
    return api_request("POST", "/ns", json_data=data)





#####################################
# Image Selection & Analysis
#####################################

# Helper function: Select best image for specific VM spec
# Helper: combined lowercase OS descriptor from current ImageInfo fields (guestOS was removed from the API)
def _image_os_text(img: Dict) -> str:
    return " ".join(str(img.get(k, "")) for k in ("osType", "osDistribution", "osPlatform")).lower()

def select_best_image_for_spec(
    image_list: List[Dict],
    vm_spec: Dict,
    requirements: Optional[Dict] = None
) -> Dict:
    """
    Select best image for a specific VM specification.
    This function considers CSP, region, architecture, and other spec-specific requirements.
    
    **CRITICAL for Multi-VM Infra Creation:**
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
        guest_os = _image_os_text(img)
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
    guest_os = _image_os_text(image)
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
            "osInfo": _image_os_text(img),
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
        guest_os = _image_os_text(image)
        
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
@tool()
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
        guest_os = _image_os_text(img)
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


# Helper function: does this namespace exist? (used by several tools)
def _internal_validate_namespace(ns_id: str) -> Dict:
    """Internal helper function to validate if a namespace exists"""
    try:
        ns_info = _internal_get_namespace(ns_id)
        if "error" in ns_info:
            # Distinguish transport/server failures from a genuinely missing namespace
            if ns_info.get("error_type") in ("connection_error", "timeout"):
                return {
                    "valid": False,
                    "namespace_id": ns_id,
                    "error": "Could not reach CB-Tumblebug to validate the namespace",
                    "details": ns_info,
                    "suggestion": "Check server availability and retry — do NOT create the namespace based on this result"
                }
            return {
                "valid": False,
                "namespace_id": ns_id,
                "error": "Namespace does not exist",
                "suggestion": f"Create namespace '{ns_id}' using manage_namespace(action='create') or choose from existing namespaces using manage_namespace(action='create')"
            }
        
        return {
            "valid": True,
            "namespace_id": ns_id,
            "namespace_info": ns_info,
            "status": "ready_for_infra_creation"
        }
    except Exception as e:
        return {
            "valid": False,
            "namespace_id": ns_id,
            "error": f"Failed to validate namespace: {str(e)}",
            "suggestion": "Check your connection and try again"
        }


# Helper: create a namespace unless it exists. Backs the create_namespace tool.
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
            "suggestion": "You can use this existing namespace for Infra creation"
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
            "status": "ready_for_infra_creation",
            "message": f"Namespace '{name}' created successfully and ready for Infra creation"
        }
    except Exception as e:
        return {
            "created": False,
            "namespace_id": name,
            "error": f"Failed to create namespace: {str(e)}",
            "suggestion": "Please check your input and connection"
        }

# Helper function: Create namespace with validation
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
        confirm: Must be True to proceed. Ask the user first - this is the only
            thing standing between a request and the destructive action.

    Returns:
        Removal result
        confirm: must be True to proceed - the provider and its credentials stop being usable
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
        specs = recommend_vm_spec(filter_policies={"providerName": provider_names[0]}, limit="200")
        counts["specs"] = len(specs.get("summarized_specs") or specs.get("specList") or [])
        images = search_images(provider_name=provider_names[0], include_all=True, max_results=200)
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

# Tool: Get all cloud connections
@tool()
def get_connections(detail: str = "summary", provider: Optional[str] = None,
                    fields: Optional[str] = None, filter_verified: bool = True,
                    filter_region_representative: bool = True,
                    credential_holder: Optional[str] = None) -> Dict:
    """
    Get all registered cloud connections

    Args:
        detail: "minimal" (configName only) | "summary" (default) | "full"
        provider: optional provider name filter, e.g. "aws"
        fields: optional comma-separated allowlist of fields to keep
        filter_verified: only connections that passed verification
        filter_region_representative: one representative region per provider
        credential_holder: only connections of this credential holder

    Returns:
        List of cloud connections
    """
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
        # regionDetail is 282 of the 652 bytes per entry - coordinates and the zone list,
        # which a caller picking a connection does not need.
        shaped = [{"configName": c.get("configName"),
                   "providerName": c.get("providerName"),
                   "regionName": (c.get("regionDetail") or {}).get("regionId")
                                 or c.get("regionZoneInfoName")}
                  for c in items]
    return {"connectionconfig": _project(shaped, fields), "count": len(shaped)}

# Tool: Get specific cloud connection
@tool()
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
        Update result with the SecurityGroup's resulting rule set
    """
    try:
        data = {"firewallRules": _normalize_firewall_rules(rules)}
    except ValueError as exc:
        return {"error": str(exc)}
    return api_request("POST", f"/ns/{ns_id}/resources/securityGroup/{sg_id}/rules", json_data=data)

# Tool: Remove SecurityGroup firewall rules
@tool(mutating=True)
def remove_security_group_rules(ns_id: str, sg_id: str, rules: List[Dict], confirm: bool = False) -> Dict:
    """
    Remove firewall rules from a SecurityGroup (rules must match existing ones).

    Args:
        ns_id: Namespace ID
        sg_id: SecurityGroup ID
        rules: List of rules to remove — same format as add_security_group_rules()
        confirm: Must be True to proceed. Ask the user first - this is the only
            thing standing between a request and the destructive action.

    Returns:
        Update result with the SecurityGroup's resulting rule set
        confirm: must be True to proceed - removing rules can cut off SSH to running nodes
    """
    guard = _require_confirmation(confirm, "removing security group rules", "may cut off SSH or application traffic to running nodes")
    if guard:
        return guard
    try:
        data = {"firewallRules": _normalize_firewall_rules(rules)}
    except ValueError as exc:
        return {"error": str(exc)}
    return api_request("DELETE", f"/ns/{ns_id}/resources/securityGroup/{sg_id}/rules", json_data=data)



# Tool: Provisioning risk, in one place
@tool()
def get_provisioning_risk(spec_id: str, csp_image_name: Optional[str] = None,
                          kind: str = "summary") -> Dict:
    """
    Historical provisioning risk for a spec, optionally paired with an image.

    Three related questions used to be three separate tools; `kind` selects between them.

    Args:
        spec_id: Spec ID, e.g. "aws+ap-northeast-2+t2.small"
        csp_image_name: CSP image identifier. Required for "summary" and "detailed".
        kind: "summary" (default) - risk level for this spec+image pair
              "detailed"          - per-factor breakdown and recommendations
              "history"           - raw success/failure log for the spec

    Returns:
        The requested risk view, or an error naming what is missing.
    """
    kind = (kind or "summary").strip().lower()
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
def list_infra_templates(ns_id: str = "system", template_id: Optional[str] = None) -> Dict:
    """
    List saved Infra templates, or fetch one by id.

    A template is a stored node_groups configuration, so a known-good demo setup can be
    replayed without rebuilding the spec and image selection each time.

    Args:
        ns_id: Namespace ID. Defaults to "system", where init loads the shipped templates.
        template_id: Return just this template instead of the whole list

    Returns:
        The template list, or the single template.
    """
    if template_id:
        return api_request("GET", f"/ns/{ns_id}/template/infra/{template_id}")
    return api_request("GET", f"/ns/{ns_id}/template/infra")


@tool(mutating=True)
def provision_infra_from_template(ns_id: str, template_id: str, name: str,
                                  description: Optional[str] = None, budget_ack: Optional[str] = None, confirm: bool = False) -> Dict:
    """
    Create an Infra from a saved template.

    The template supplies the node groups; only the new Infra's name is needed. Cost is not
    reviewed on this path, so check the template with list_infra_templates() first if the
    spend matters.

    Args:
        ns_id: Namespace ID
        template_id: Template to instantiate (see list_infra_templates)
        name: Name for the new Infra
        description: Optional description
        budget_ack: Approval id from request_budget_increase(), to proceed past a
            spend limit. Single-use.
        confirm: Must be True to proceed. Ask the user first - this is the only
            thing standing between a request and the destructive action.

    Returns:
        The created Infra.
    """
    guard = _require_confirmation(
        confirm, f"creating infrastructure from template {template_id!r}",
        "the template decides the machines and their cost; read it with list_infra_templates first")
    if guard:
        return guard
    over_budget = _check_budget(None, budget_ack)
    if over_budget:
        return over_budget
    body: Dict[str, Any] = {"name": name}
    if description:
        body["description"] = description
    return api_request("POST", f"/ns/{ns_id}/infra/template/{template_id}", json_data=body)


@tool(mutating=True)
def manage_infra_template(action: str, ns_id: str = "system",
                          template_id: Optional[str] = None,
                          template: Optional[Dict] = None,
                          confirm: bool = False) -> Dict:
    """
    Create, update or delete an Infra template.

    Args:
        action: "create" | "update" | "delete"
        ns_id: Namespace ID. Defaults to "system", where init loads the shipped templates.
        template_id: Required for update and delete
        template: Template body for create and update
        confirm: Must be True for action="delete". Ask the user first - a shipped template
            is not recreated until the next init.

    Returns:
        The server's response.
    """
    action = (action or "").strip().lower()
    if action == "create":
        if not template:
            return {"error": "template body is required for action='create'"}
        return api_request("POST", f"/ns/{ns_id}/template/infra", json_data=template)
    if action == "update":
        if not (template_id and template):
            return {"error": "template_id and template are required for action='update'"}
        return api_request("PUT", f"/ns/{ns_id}/template/infra/{template_id}", json_data=template)
    if action == "delete":
        if not template_id:
            return {"error": "template_id is required for action='delete'"}
        guard = _require_confirmation(
            confirm, f"deleting template {template_id!r}",
            "a shipped template is not recreated until the next init")
        if guard:
            return guard
        return api_request("DELETE", f"/ns/{ns_id}/template/infra/{template_id}")
    return {"error": f"unknown action {action!r}; expected create, update or delete"}


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

@tool()
def manage_command_task(action: str, ns_id: str, infra_id: str,
                        task_id: Optional[str] = None, detail: str = "summary") -> Dict:
    """
    List, inspect or cancel remote-command tasks on an Infra.

    A long bootstrap started with post_command_async or a long timeout keeps running on the
    server; this is how to see what is in flight and stop it.

    Args:
        action: "list" | "get" | "cancel"
        ns_id: Namespace ID
        infra_id: Infra ID
        confirm: Must be True to proceed. Ask the user first - this destroys running nodes.
        task_id: Required for "get" and "cancel"
        detail: "summary" (default) trims the command text in listings; "full" keeps it

    Returns:
        The task list, one task, or the cancellation result.
    """
    action = (action or "").strip().lower()
    if action == "list":
        result = api_request("GET", f"/ns/{ns_id}/cmd/infra/{infra_id}/task")
        if _validate_detail(detail) == "full" or not isinstance(result, dict):
            return result
        tasks = result.get("tasks")
        if not isinstance(tasks, list):
            return result
        # A listing answers "what is running"; the command bodies belong to action="get".
        out = dict(result)
        out["tasks"] = [_drop_command_output(t) for t in tasks]
        return out
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
                             auth=(TUMBLEBUG_USERNAME, TUMBLEBUG_PASSWORD), timeout=(60, 600))
    except Exception as e:
        return {"error": f"download failed: {e}"}
    if resp.status_code >= 400:
        try:
            return {"error": resp.json().get("message", resp.text[:300])}
        except Exception:
            return {"error": f"HTTP {resp.status_code}: {resp.text[:300]}"}

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
    """Hourly cost of everything currently running, summed from node specs.

    Deliberately not option=status: that view omits specId, so every node would look
    unpriced and the concurrent limit would never fire. The full listing is large, but it is
    read inside the server and never reaches the caller's context.
    """
    total, unpriced = 0.0, 0
    listing = api_request("GET", f"/ns/{DEFAULT_NAMESPACE}/infra")
    for infra in (listing or {}).get("infra") or []:
        for node in infra.get("vm") or infra.get("node") or []:
            if str(node.get("status", "")).lower().startswith("terminated"):
                continue
            cost = _spec_cost(node.get("specId") or "")
            if cost is None:
                unpriced += 1
            else:
                total += cost
    return round(total, 4), unpriced


def _created_today():
    stamp = datetime.now().strftime("%Y-%m-%d")
    entries = _state_range(f"{BUDGET_CREATED_PREFIX}{stamp}/")
    return round(sum(float(v.get("usd_per_hour", 0)) for _, v in entries), 4)


def _record_created(amount):
    stamp = datetime.now().strftime("%Y-%m-%d")
    key = f"{BUDGET_CREATED_PREFIX}{stamp}/{datetime.now().strftime('%H%M%S%f')}"
    _state_put(key, {"usd_per_hour": amount}, ttl_seconds=48 * 3600)


def _consume_approval(token):
    """An approval is single-use: spend it or it stays unspent."""
    if not token:
        return False
    key = f"{BUDGET_REQUEST_PREFIX}{token}"
    record = _state_get(key)
    if not isinstance(record, dict) or record.get("status") != "approved":
        return False
    record["status"] = "used"
    record["used_at"] = datetime.now().isoformat(timespec="seconds")
    _state_put(key, record, ttl_seconds=BUDGET_APPROVAL_TTL_SECONDS)
    return True


def _check_budget(node_groups, budget_ack=None):
    """Return a refusal when a planned Infra breaks a limit, else None."""
    policy = _budget_policy()
    if not policy.get("enabled"):
        return None
    if _consume_approval(budget_ack):
        logger.info(f"budget: proceeding under approval {budget_ack}")
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
        _record_created(planned)
        return None

    token = f"req-{datetime.now().strftime('%Y%m%d%H%M%S%f')}"
    _state_put(f"{BUDGET_REQUEST_PREFIX}{token}", {
        "status": "pending",
        "requested_at": datetime.now().isoformat(timespec="seconds"),
        "planned_usd_per_hour": planned,
        "unpriced_nodes": unpriced,
        "breaches": breaches,
        "node_groups": node_groups,
    }, ttl_seconds=7 * 24 * 3600)

    return {"error": {
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
        "how_to_proceed": ("an administrator approves this request out of band, then retry "
                           f"with budget_ack='{token}'"),
    }}


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


@tool()
def request_budget_increase(reason: str, request_id: Optional[str] = None) -> Dict:
    """
    Ask an administrator to allow a creation that a limit refused.

    The agent can ask; it cannot grant. Approval happens outside MCP - an administrator
    writes it directly - so no tool call can approve its own request.

    Args:
        reason: Why the spend is justified. Recorded for the administrator to read.
        request_id: The request_id returned by the refusal. Omit to raise a new one.

    Returns:
        The request and how to check on it.
    """
    token = request_id or f"req-{datetime.now().strftime('%Y%m%d%H%M%S%f')}"
    key = f"{BUDGET_REQUEST_PREFIX}{token}"
    record = _state_get(key) or {"status": "pending",
                                 "requested_at": datetime.now().isoformat(timespec="seconds")}
    record["reason"] = reason
    record["status"] = record.get("status", "pending")
    _state_put(key, record, ttl_seconds=7 * 24 * 3600)
    return {
        "request_id": token,
        "status": record["status"],
        "message": "recorded; an administrator must approve it out of band",
        "administrator_action": f"make mcp-budget-approve ID={token}",
        "then": f"retry the creation with budget_ack='{token}'",
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
        The server's response. Creating a namespace that exists reports it rather than failing.
    """
    action = (action or "").strip().lower()
    if not name:
        return {"error": "name is required"}
    if action == "create":
        return _internal_create_namespace_with_validation(name, description)
    if action == "update":
        if not description:
            return {"error": "description is required for action='update'"}
        return api_request("PUT", f"/ns/{name}",
                           json_data={"name": name, "description": description})
    if action == "delete":
        guard = _require_confirmation(
            confirm, "deleting namespace",
            "removes the namespace and every resource recorded in it")
        if guard:
            return guard
        return api_request("DELETE", f"/ns/{name}")
    return {"error": f"unknown action {action!r}; expected create, update or delete"}


# ---------------------------------------------------------------------------
# Session continuity
# ---------------------------------------------------------------------------

@tool()
def resume_session(session_id: str) -> Dict:
    """
    Recover what a session was doing, after a disconnect or a restart.

    Every mutating call is journaled automatically under the session_id passed to it, so this
    reports real actions rather than whatever the model remembered to write down. Kept small
    on purpose: the point is to re-orient, not to replay a transcript.

    Args:
        session_id: The identifier used on earlier calls

    Returns:
        created_resources, in_flight (calls that errored or never reported), last_actions,
        and a suggested next step.
    """
    entries = [v for _, v in _state_range(_session_key(session_id, "journal/"))]
    if not entries:
        return {"session_id": session_id, "known": False,
                "message": "no journal for this session id",
                "hint": "pass the same session_id on later calls to build one"}

    created, failed = [], []
    for e in entries:
        args = e.get("arguments") or {}
        target = args.get("name") or args.get("infra_id") or args.get("ns_id")
        if e.get("outcome") == "error":
            failed.append({"tool": e.get("tool"), "target": target, "at": e.get("at")})
        elif e.get("tool") in ("create_infra_dynamic", "provision_infra_from_template",
                               "add_nodegroup_dynamic", "manage_namespace"):
            created.append({"tool": e.get("tool"), "target": target, "at": e.get("at")})

    last = entries[-1]
    return {
        "session_id": session_id,
        "known": True,
        "actions_recorded": len(entries),
        "created_resources": created[-10:],
        "in_flight": failed[-5:],
        "last_action": {"tool": last.get("tool"), "outcome": last.get("outcome"),
                        "at": last.get("at")},
        "suggested_next_step": (
            "verify the resources listed in created_resources with get_infra_list(), then "
            "continue" if created else
            "nothing was created yet; restate the goal and start from review"),
    }


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
                      "subnetInfoList", "associatedObjectList")),
    "securityGroup": ("securityGroup", ("id", "name", "connectionName", "vNetId",
                                        "description", "firewallRules", "associatedObjectList",
                                        "status", "deletionRequestedAt")),
    "sshKey": ("sshKey", ("id", "name", "connectionName", "username", "verifiedUsername",
                          "fingerprint", "associatedObjectList", "description",
                          "status", "deletionRequestedAt")),
    "dataDisk": ("dataDisk", ("id", "name", "connectionName", "diskType", "diskSize",
                              "status", "associatedObjectList",
                              "deletionRequestedAt", "systemMessage")),
    "customImage": ("customImage", ("id", "name", "connectionName", "status",
                                    "description", "associatedObjectList",
                                    "imageStatus", "deletionRequestedAt", "systemMessage")),
    "objectStorage": ("objectStorage", ("id", "name", "connectionName", "status",
                                        "description")),
    "nlb": ("nlb", ("id", "name", "connectionName", "vNetId", "listener", "status")),
}


@tool()
def get_resources(kind: str, ns_id: str = DEFAULT_NAMESPACE, detail: str = "summary",
                  fields: Optional[str] = None) -> Dict:
    """
    List resources of one kind in a namespace.

    Args:
        kind: "vNet" | "securityGroup" | "sshKey" | "dataDisk" | "customImage" |
              "objectStorage" | "nlb"
        ns_id: Namespace ID (defaults to the standing namespace)
        detail: "minimal" | "summary" (default) | "full"
        fields: Comma-separated allowlist of fields to keep

    Returns:
        The resource list under its own key, e.g. {"vNet": [...]}.
        Private keys are redacted; get_ssh_private_key returns one deliberately.
    """
    entry = _RESOURCE_KINDS.get(kind) or _RESOURCE_KINDS.get(str(kind).strip())
    if not entry:
        return {"error": f"unknown kind {kind!r}; expected one of "
                         f"{', '.join(sorted(_RESOURCE_KINDS))}"}
    path, summary_keys = entry
    return _shape_resource_list(
        api_request("GET", f"/ns/{ns_id}/resources/{path}"), path,
        _validate_detail(detail), fields, summary_keys)


@tool(mutating=True)
def manage_labels(action: str, label_type: str, uid: Optional[str] = None,
                  labels: Optional[Union[Dict, str]] = None, key: Optional[str] = None) -> Dict:
    """
    Read, set, remove or sync resource labels, and find resources by label.

    Labels are what label_selector targets when a remote command should reach a whole tier,
    so a role label placed at creation is usually the one that matters later.

    Args:
        action: "get" | "set" | "remove" | "find" | "sync_from_csp"
        label_type: Resource type, e.g. "infra", "node", "vNet"
        uid: Resource uid - required for get, set, remove and sync_from_csp
        labels: Key-value pairs, e.g. {"role": "control"}. For "find" a selector string
            such as "role=control,tier=web" is accepted as well, and a mapping given
            there is joined into one.
        key: Label key to delete, for "remove"

    Returns:
        The labels, the matching resources, or the result of the change.
    """
    action = (action or "").strip().lower()
    if action == "find":
        if not labels:
            return {"error": "labels (a selector string) is required for action='find'"}
        selector = labels if isinstance(labels, str) else ",".join(
            f"{k}={v}" for k, v in labels.items())
        return api_request("GET", f"/resources/{label_type}",
                           params={"labelSelector": selector})
    if not uid:
        return {"error": f"uid is required for action={action!r}"}
    if action == "get":
        return api_request("GET", f"/label/{label_type}/{uid}")
    if action == "set":
        if not labels:
            return {"error": "labels is required for action='set'"}
        return api_request("PUT", f"/label/{label_type}/{uid}", json_data=labels)
    if action == "remove":
        if not key:
            return {"error": "key is required for action='remove'"}
        return api_request("DELETE", f"/label/{label_type}/{uid}/{key}")
    if action == "sync_from_csp":
        return api_request("PUT", f"/mergeCSPLabel/{label_type}/{uid}")
    return {"error": f"unknown action {action!r}; expected get, set, remove, find "
                     f"or sync_from_csp"}


@tool()
def get_handling_command_count(ns_id: str = DEFAULT_NAMESPACE, infra_id: str = "",
                               node_id: Optional[str] = None) -> Dict:
    """
    How many remote commands are still running.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Narrow to one node; omit for the whole Infra

    Returns:
        The in-flight command count, and a caveat about when it can be trusted.
    """
    if not infra_id:
        return {"error": "infra_id is required"}
    if node_id:
        result = api_request("GET",
                             f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}/handlingCount")
    else:
        result = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/handlingCount")

    # A command is not counted until the server has registered it, which lags the request by
    # tens of seconds. Polling this immediately after issuing one reads 0 and looks like
    # "finished" - it means "not started yet". Say so rather than letting the caller guess.
    if isinstance(result, dict):
        result["caveat"] = ("0 can mean 'not registered yet' as well as 'finished'. Right "
                            "after issuing a command, confirm with list_node_command_status "
                            "or get_command_result before concluding it is done.")
    return result


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
    for node in (infra.get("vm") or infra.get("node") or []):
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
        status, and per-node output once finished. Poll while status is Handling.
    """
    record = api_request("GET", f"/request/{request_id}")
    out: Dict[str, Any] = {"request_id": request_id}
    if isinstance(record, dict) and "error" not in record:
        out["request"] = _trim_command_record(record, tail_lines, contains, context_lines)

    if infra_id:
        nodes, still_running = [], 0
        listing = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}")
        for node in (listing or {}).get("vm") or (listing or {}).get("node") or []:
            for entry in (node.get("commandStatus") or []):
                if entry.get("xRequestId") != request_id:
                    continue
                if entry.get("status") == "Handling":
                    still_running += 1
                nodes.append({"nodeId": node.get("id"),
                              **_trim_command_record(entry, tail_lines, contains, context_lines)})
        out["nodes"] = nodes
        out["running"] = still_running
        out["status"] = "Handling" if still_running else ("Completed" if nodes else "Unknown")
        if still_running:
            out["hint"] = "still running - poll this again rather than re-issuing the command"
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
@tool(mutating=True)
def manage_dns_record(action: str, domain_name: str, record_name: Optional[str] = None,
                      record_type: str = "A", ttl: int = 300,
                      routing_policy: str = "simple",
                      infra_id: Optional[str] = None,
                      ns_id: str = DEFAULT_NAMESPACE,
                      label_selector: Optional[str] = None,
                      ips: Optional[List[str]] = None,
                      confirm: bool = False) -> Dict:
    """
    Read, set or delete a global DNS record.

    A record can be pointed at an Infra rather than at addresses. CB-Tumblebug then resolves
    the node addresses itself and keeps the record as one name for the whole deployment -
    which is the point when the nodes sit in ten regions and their IPs are not memorable.

    Exactly one source must be given for "set":
      infra_id        every running node of that Infra
      label_selector  nodes matching a label, scoped to infra_id when both are given
      ips             explicit addresses

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

    sources = [bool(infra_id), bool(label_selector), bool(ips)]
    if sum(sources) != 1:
        return {"error": "give exactly one source: infra_id, label_selector or ips"}
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

1. Pick specs.  recommend_vm_spec(filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
   priority_policy="cost")  ->  summarized_specs[i]["id"], e.g. "aws+ap-northeast-2+t2.small".
   Use the id exactly as returned. It encodes provider, region and instance type, and an
   edited or hand-written id fails at the CSP.

2. Pick an image per spec.  search_images(matched_spec_id=spec_id, os_type="ubuntu 22.04",
   include_basic_image_only=True)  ->  imageList[i]["cspImageName"].
   Every spec needs its own image: image ids are CSP- and region-specific, so one image
   cannot be reused across providers or regions.

3. Build node_groups.
   [{"specId": spec_id, "imageId": csp_image_name, "name": "web-1", "nodeGroupSize": 1,
     "label": {"role": "web"}}]
   The label matters later - label_selector on a remote command targets it.

4. Review.  review_infra_dynamic_request(ns_id, name, node_groups)
   Check overallStatus and creationViable. Fix what it reports and review again.
   estimatedCost is a compute list price: it excludes egress, storage IOPS, NAT/LB
   hourly and licensing.

5. Create.  create_infra_dynamic(ns_id, name, node_groups, force_create=True)
   force_create is what distinguishes an intentional provision from a review.

Bootstrapping at creation time: pass post_commands with one phase per tier and
post_command_async=True for anything slow. Nodes bill from Running, so a synchronous
wait costs money and can hit proxy timeouts.""",

    "commands": """Running commands on nodes.

Each element of `commands` is a separate SSH session. Shell state does not survive
between elements:
    WRONG  ["IFACE=$(ip route | ...)", "echo $IFACE"]      # second sees an empty IFACE
    RIGHT  ["IFACE=$(ip route | ...) && echo $IFACE"]

Exit status is the last command's. A trailing echo masks a failure - `installer.sh; echo done`
reports success no matter what the installer did. Use && between steps, or set -o pipefail.

Targeting: node_id for one node, nodegroup_id for a NodeGroup, label_selector for a role
across NodeGroups ("role=node"). At most one applies.

Long-running work: pass timeout_minutes (server default 30, max 120). Output is retrieved
with list_node_command_status() or get_node_command_status(index=...); by default only the
newest record and any that did not complete carry their output.

Predefined scripts are listed by list_predefined_scripts() and cover common setup tasks.""",

    "gpu": """Serving an LLM on a GPU node.

Choosing the machine. Filtering on acceleratorCount alone is not enough - it returns AMD
cards, and the vLLM/KServe stack is CUDA-only, so an AMD node fails after provisioning:

    recommend_vm_spec(filter_policies={"providerName": "aws", "regionName": "eu-central-1",
                                       "acceleratorCount": {"min": 1},
                                       "acceleratorModel": "nvidia"},
                      priority_policy="cost")

In eu-central-1 that is the difference between g4ad.xlarge (AMD, $0.47/h, will not run the
model) and g4dn.xlarge (NVIDIA T4, $0.658/h, will). The cheaper one is not cheaper if it
cannot do the job. get_search_options("spec") lists the filter vocabulary, including
acceleratorModel, acceleratorType and acceleratorMemoryGB.

Choosing the image. search_images(is_gpu_image=True) returns purpose-built images with the
driver already present. The clean-base-image default steps aside automatically when that
flag is set.

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

Check progress with get_command_result or list_node_command_status; step 3 and 4 are long
enough that a synchronous wait can hit a client timeout.""",

    "recovery": """When something fails partway through.

Creation is not all-or-nothing. policyOnPartialFailure defaults to "continue", so a request
for ten nodes can leave nine running and one Failed, and the Infra sits at
Partial-Running:9 (R:9/10).

Clear the wreckage, keep the rest:
    control_infra(action='refine')     removes the Failed nodes; the Infra returns to Running

Replace what was lost:
    review_nodegroup_dynamic(...)      check the substitute first
    add_nodegroup_dynamic(...)         add it under a NEW name

Use a new name. A node group keeps its shared resources (security group, key pair) after the
node dies, and those are keyed by Infra and node-group name without the region. Reusing the
old name in a different region attaches a security group belonging to the first region, and
creation fails with "Security Group does not exist in connection ...".

Why a review can pass and creation still fail: the review checks that the spec and image
exist and are priced. It cannot check account quota, live stock, or faults internal to the
provider. Expect these and treat them as a retry signal, not a bug:
    VcpuLimitExceeded ...              quota - ask for an increase, or use another family
    no zone with stock ... right now   capacity - another spec in the region, or another region
    provider-specific 4xx              try a different provider for that location

Probing cheaply: review_spec_image_pair(spec_id, image_id) validates one pair in well under
a second, so several candidates can be tested before committing to a full plan.

Long commands: pass run_async=True to execute_command_infra and follow with
get_command_result(request_id=...). A dropped connection then costs nothing.""",

    "detail": """Controlling response size.

Read tools take `detail`:
    minimal  ids and status only
    summary  the default - identifiers, status, addresses, and counts
    full     everything the server returned, secrets still redacted

and `fields`, a comma-separated allowlist applied after tiering:
    get_infra(ns_id, infra_id, fields="id,status,node")

Command history is collapsed at summary to {count, last_status, last_request_id}; fetch the
transcript deliberately rather than carrying it in every read.

Secrets are stripped from every response. get_ssh_private_key() is the one deliberate
exception and puts the key into the transcript.""",
}


@tool()
def get_usage_guide(topic: str) -> Dict:
    """
    Long-form guidance kept out of the tool schemas.

    Tool descriptions carry only what is needed to call them correctly; worked examples and
    workflow notes live here so they cost nothing until asked for.

    Args:
        topic: "provision" | "commands" | "gpu" | "recovery" | "detail"

    Returns:
        The guide text, or the list of topics if the name is unknown.
    """
    text = _USAGE_GUIDES.get((topic or "").strip().lower())
    if text is None:
        return {"error": f"unknown topic {topic!r}", "topics": sorted(_USAGE_GUIDES)}
    return {"topic": topic, "guide": text}


# Tool: Get an SSH private key (explicitly, because it lands in the transcript)
@tool(redact=False)
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
        key_id: SSH key resource ID (see get_ssh_keys)
        reason: why the key is needed - recorded in the server log for audit

    Returns:
        The key resource including privateKey, or an error
    """
    logger.warning(f"get_ssh_private_key: ns={ns_id} key={key_id} reason={reason!r}")
    result = api_request("GET", f"/ns/{ns_id}/resources/sshKey/{key_id}")
    if isinstance(result, dict) and "error" not in result:
        result["_warning"] = "private key material is now in this transcript"
    return result


# Tool: Release resources
@tool(mutating=True)
def release_resources(ns_id: str, confirm: bool = False, dry_run: bool = False) -> Dict:
    """
    Reclaim shared resources in a namespace that nothing is using.

    The server decides per resource, not per namespace: it lists the VNets, SecurityGroups
    and SSHKeys, checks each one's associated objects, and deletes only those with none.
    Anything a running Infra still references is preserved. So this is safe to run with
    other Infras present - it will simply skip their resources - and it is the normal way
    to clean up after terminating an Infra, which leaves its security group and key behind.

    Args:
        ns_id: Namespace ID
        confirm: Must be True to proceed. Ask the user first.
        dry_run: Report what would be released without deleting anything. Cheap, and the
            honest way to show the user the list before asking for confirm.

    Returns:
        Per-resource results with a success/failure count. Resources still in use are not
        listed, because they were never candidates.
    """
    if not confirm and not dry_run:
        return {
            "error": "Refused: releasing shared resources was not confirmed",
            "requirement": "Set confirm=True to proceed, or dry_run=True to preview first",
            "note": ("Only resources with no associated objects are removed; anything a "
                     "running Infra references is left alone."),
        }

    params = {"dryRun": "true"} if dry_run else None
    result = api_request("DELETE", f"/ns/{ns_id}/sharedResources", params=params)

    if isinstance(result, dict) and "error" not in result:
        result["_note"] = (
            "preview only - nothing was deleted; re-run with confirm=True to apply"
            if dry_run else
            "resources still referenced by a running Infra were skipped, not failed")
    return result

# Tool: Check resource exists
@tool()
def check_resource_exists(ns_id: str, resource_type: str, resource_id: str) -> Dict:
    """
    Check if a specific resource exists in the namespace.
    This is useful for validating resources before using them in Infra creation.
    
    Args:
        ns_id: Namespace ID
        resource_type: Type of resource (e.g., "vNet", "securityGroup", "sshKey", "image", "spec")
        resource_id: Resource ID to check

    Returns:
        {"exists": true|false}
    """
    return api_request("GET", f"/ns/{ns_id}/checkResource/{resource_type}/{resource_id}")


# # Tool: Register CSP resources
# @mcp.tool()
# def register_csp_resources(ns_id: str, infra_flag: str = "n") -> Dict:
#     """
#     Register CSP resources
    
#     Args:
#         ns_id: Namespace ID
#         infra_flag: Infra flag (y/n)
    
#     Returns:
#         Registration result
#     """
#     data = {
#         "infraName": "csp",
#         "nsId": ns_id
#     }
#     return api_request("POST", f"/registerCspResourcesAll?infraFlag={infra_flag}", json_data=data)

#####################################
# Infra Management (Multi-Cloud Infrastructure)
#####################################

# Tool: Get Infra list
@tool()
def get_infra_list(ns_id: str, detail: str = "summary",
                   fields: Optional[str] = None, option: str = "status") -> Dict:
    """
    Get list of Infras (Multi-Cloud Infrastructures) for a specific namespace.
    
    Args:
        ns_id: Namespace ID
        detail: "summary" (default) or "full". Summary drops the per-node command
            history, which is where nearly all the bytes are.
        fields: Comma-separated top-level fields to keep, e.g. "id,status". Applied
            after detail.
        option: Server-side view: "status" (default), "id" for ids only, or "" for the
            full records.

    Returns:
        List of Infras
    """
    detail = _validate_detail(detail)
    if option not in ("id", "simple", "status"):
        option = "status"
    result = api_request("GET", f"/ns/{ns_id}/infra?option={option}")
    if not isinstance(result, dict) or "infra" not in result:
        return result
    out = dict(result)
    out["infra"] = [_shape_infra(i, detail) for i in (result.get("infra") or [])]
    return _project(_redact_secrets(out), fields)

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
#
# A bastion is the SSH jump host CB-Tumblebug tunnels through to reach a node. One is
# auto-assigned when a node has a public IP, so most Infras never need these tools.
#
# They matter when a node's "public" address is not reachable from CB-Tumblebug. The
# clearest case is a VM inside a self-hosted OpenStack: its floating IP is private space
# routable only from the machine running that OpenStack. Registering that machine as the
# bastion is the only way remote commands reach the VM.

# Tool: Set a bastion node for a target node
@tool(mutating=True)
def set_bastion_node(
    ns_id: str,
    infra_id: str,
    target_node_id: str,
    bastion_node_id: str,
    bastion_infra_id: str = "",
    bastion_ns_id: str = ""
) -> Dict:
    """
    Register a bastion (SSH jump host) for a node, so remote commands can reach it.

    Needed when the target's own address is not reachable from CB-Tumblebug. For a VM in
    a self-hosted OpenStack, the bastion is the VM that hosts that OpenStack - it is the
    only machine that can route to the tenant's floating IPs.

    The bastion may live in a different Infra, and even a different namespace, from its
    target; pass bastion_infra_id (and bastion_ns_id) for that.

    A manually registered bastion takes precedence over any auto-assigned one, including
    an auto-assignment that pointed the node at itself.

    Args:
        ns_id: Namespace of the TARGET node
        infra_id: Infra of the TARGET node
        target_node_id: Node that needs to be reached, e.g. "g1-1"
        bastion_node_id: Node to jump through, e.g. "g9-1". Must have an address
            CB-Tumblebug can reach, and must be able to route to the target.
        bastion_infra_id: Bastion's Infra, when different from the target's
        bastion_ns_id: Bastion's namespace, when different from the target's

    Returns:
        Registration result. Verify with a simple execute_command_infra() such as
        "hostname" before running anything long.
    """
    path = f"/ns/{ns_id}/infra/{infra_id}/node/{target_node_id}/bastion"
    if bastion_ns_id and bastion_infra_id:
        path = f"{path}/{bastion_ns_id}/{bastion_infra_id}/{bastion_node_id}"
    elif bastion_infra_id:
        path = f"{path}/{bastion_infra_id}/{bastion_node_id}"
    else:
        path = f"{path}/{bastion_node_id}"
    result = api_request("PUT", path)
    # This endpoint answers with a bare JSON string; wrap it so the tool keeps its
    # declared Dict return type and the caller gets a usable next step.
    if isinstance(result, str):
        return {
            "message": result,
            "alreadyRegistered": "already exists" in result,
            "nextStep": "Confirm with get_bastion_nodes(), then run a simple command "
                        "(hostname) on the target to prove it is reachable.",
        }
    return result


# Tool: Get bastion nodes for a target node
@tool()
def get_bastion_nodes(ns_id: str, infra_id: str, target_node_id: str) -> Dict:
    """
    List the bastion nodes registered for a node.

    Useful when remote commands to a node time out: if the only entry is the node
    itself, CB-Tumblebug will dial it directly, which fails whenever its address is not
    reachable from here.

    Args:
        ns_id: Namespace of the target node
        infra_id: Infra of the target node
        target_node_id: Node to inspect, e.g. "g1-1"

    Returns:
        Registered bastions, each with its namespace, Infra and node id
    """
    result = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/node/{target_node_id}/bastion")
    # This endpoint answers with a bare JSON array; wrap it so the tool's return type
    # stays a Dict and callers get the self-bastion hint alongside the list.
    if isinstance(result, list):
        is_self_only = (
            len(result) == 1
            and result[0].get("nodeId") == target_node_id
            and result[0].get("infraId") == infra_id
        )
        return {
            "bastionNodes": result,
            "count": len(result),
            "selfBastionOnly": is_self_only,
            "note": ("The only bastion is the node itself, so CB-Tumblebug will dial it "
                     "directly. That works only if the node's own address is reachable from "
                     "CB-Tumblebug; if commands time out, register a reachable jump host with "
                     "set_bastion_node().") if is_self_only else None,
        }
    return result


# Tool: Remove a bastion node
@tool(mutating=True)
def remove_bastion_node( ns_id: str, infra_id: str, bastion_node_id: str, bastion_infra_id: str = "", bastion_ns_id: str = "" , confirm: bool = False) -> Dict:
    """
    Remove a bastion registration from an Infra.

    Args:
        ns_id: Namespace of the Infra holding the registration
        infra_id: Infra holding the registration
        bastion_node_id: Bastion node to unregister
        bastion_infra_id: Bastion's Infra, when it differs from infra_id
        bastion_ns_id: Bastion's namespace, when it differs from ns_id
        confirm: Must be True to proceed. Ask the user first - this is the only
            thing standing between a request and the destructive action.

    Returns:
        Removal result
        confirm: must be True to proceed - nodes behind this bastion lose their route
    """
    guard = _require_confirmation(confirm, "removing a bastion", "remote commands to nodes served by this bastion will fail until another is set")
    if guard:
        return guard
    path = f"/ns/{ns_id}/infra/{infra_id}/bastion"
    if bastion_ns_id and bastion_infra_id:
        path = f"{path}/{bastion_ns_id}/{bastion_infra_id}/{bastion_node_id}"
    elif bastion_infra_id:
        path = f"{path}/{bastion_infra_id}/{bastion_node_id}"
    else:
        path = f"{path}/{bastion_node_id}"
    result = api_request("DELETE", path)
    if isinstance(result, str):
        return {"message": result}
    return result



# Tool: Get nodegroups list
@tool()
def get_nodegroups(ns_id: str, infra_id: str) -> Dict:
    """
    Get list of nodegroups for a specific Infra
    
    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
    
    Returns:
        List of nodegroups
    """
    return api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/nodegroup")

# Tool: Get nodes in a nodegroup
@tool()
def get_nodes_in_nodegroup(ns_id: str, infra_id: str, nodegroup_id: str) -> Dict:
    """
    Get list of nodes for a specific nodegroup in an Infra.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        nodegroup_id: NodeGroup ID

    Returns:
        List of nodes in the nodegroup
    """
    return api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/nodegroup/{nodegroup_id}")

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


# Tool: Search images
@tool()
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
    max_results: Optional[Union[int, str]] = None,
    include_all: bool = False
) -> Dict:
    """Search VM images, optionally constrained to a chosen spec.

    Pass matched_spec_id with a spec id from recommend_vm_spec() and the provider, region
    and architecture filters are applied for you - this is the reliable way to get an image
    that will actually boot on that spec. Use the returned cspImageName as imageId.
    Vocabulary: get_search_options(kind='image'). Worked example: get_usage_guide('provision').

    Args:
        ns_id: Namespace ID (default "system")
        matched_spec_id: Spec id to match provider/region/architecture against
        provider_name: Provider to search, e.g. "aws". An explicit alternative to
            matched_spec_id; give region_name with it.
        region_name: Region to search, e.g. "ap-northeast-2". Used with provider_name.
        os_type: e.g. "ubuntu 22.04", "centos"
        os_architecture: e.g. "x86_64", "arm64"
        include_basic_image_only: Clean base images only. Defaults to True unless a GPU or
            Kubernetes image was asked for - an unfiltered search returns over a megabyte.
        is_gpu_image: Purpose-built GPU images. Use this for an accelerator node instead of
            widening the search and picking by hand.
        is_kubernetes_image: Purpose-built Kubernetes node images.
        max_results: Maximum images to return (default 20)
        include_all: Disable the defaults above and return everything. Rarely wanted.
        detail: "summary" (default) or "full"
        detail_search_keys: Extra provider-specific key/value filters applied to the
            image details, e.g. {"Architecture": "x86_64"}.

    Returns:
        imageList with cspImageName, osType, osDistribution, osArchitecture, description.
        Pass cspImageName as imageId to create_infra_dynamic(). Read osDistribution to tell
        images apart - a self-hosted OpenStack leaves name and osType empty and puts the
        only readable name ("jammy-server-cloudimg-amd64") there.
        When the exact search finds nothing the filters are relaxed automatically and
        search_status comes back "widened".
    """
    # Unfiltered, this returns every image the provider publishes: measured at 1,169,312 B
    # for one AWS region - larger than any other response in the server, and useless besides.
    # Narrowing to clean base images gives 2 results and 5,554 B, which is what almost every
    # caller wants. GPU and Kubernetes nodes have their own purpose-built images, selected
    # with is_gpu_image / is_kubernetes_image rather than by widening the search.
    if not include_all:
        if include_basic_image_only is None and not (is_gpu_image or is_kubernetes_image):
            include_basic_image_only = True
        if max_results is None:
            max_results = 20
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
        # Before reporting nothing, look again without the filters that most often hide a
        # real image. A self-hosted OpenStack publishes images with an empty osType and
        # none marked basic, so the default search returns zero while Glance holds a
        # perfectly good Ubuntu - and the model has no way to learn that from a bare
        # "no results". Match os_type against the whole OS descriptor instead.
        widened = None
        if provider_name or region_name or matched_spec_id:
            probe = {k: v for k, v in data.items()
                     if k not in ("osType", "includeBasicImageOnly", "maxResults")}
            probe["includeBasicImageOnly"] = False
            probe["maxResults"] = max_results or 50
            found = api_request("POST", f"/ns/{ns_id}/resources/searchImage", json_data=probe)
            candidates = (found or {}).get("imageList") or [] if isinstance(found, dict) else []
            os_matched = True
            if os_type:
                wanted = str(os_type).lower().split()
                narrowed = [i for i in candidates
                            if all(w in _image_os_text(i) for w in wanted)]
                os_matched = bool(narrowed)
                candidates = narrowed or candidates
            if candidates:
                widened = candidates

        if widened:
            response["imageList"] = widened
            response["imageCount"] = len(widened)
            response["search_status"] = "widened"
            note = ("The exact search matched nothing, so basic-image-only was dropped and "
                    "os_type was matched against osDistribution as well as osType. Identify "
                    "an image by osDistribution - some providers leave name and osType empty "
                    "and repeat the CSP id there.")
            if os_type and not os_matched:
                note += (f" Nothing mentions {os_type!r}, so this is EVERY image the provider "
                         f"publishes, not a filtered set - check osDistribution before using one.")
            response["_note"] = note
            return response

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
                "Consider using get_search_options(kind='image') to see available parameters"
            ]
        else:
            # Traditional search suggestions
            guidance["alternative_suggestions"] = [
                "Try broadening search criteria (remove version numbers from os_type)",
                "Try searching in different regions or providers", 
                "Try setting include_basic_image_only=True",
                "Use get_search_options(kind='image') to see available search parameters"
            ]
            
            guidance["next_steps"] = [
                "Broaden your search criteria or try different OS types",
                "Check available providers/regions with get_search_options(kind='image')",
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
            "Verify provider/region availability with get_search_options(kind='image')"
        ]
        
        response["llm_guidance"] = guidance
        
        # Log helpful guidance for debugging
        logger.info(f"💡 LLM Guidance: {guidance['message']}")
        if matched_spec_id:
            logger.info(f"🔧 Suggestion: Try recommend_vm_spec() for alternative specifications")
        else:
            logger.info(f"🔧 Suggestion: {guidance['alternative_suggestions'][0] if guidance['alternative_suggestions'] else 'Broaden search criteria'}")
    
    return response


# Tool: Recommend VM spec
@tool()
def recommend_vm_spec(
    filter_policies: Dict[str, Any] = None,
    limit: Union[str, int] = "50",
    priority_policy: str = "location",
    latitude: Optional[float] = None,
    longitude: Optional[float] = None,
    include_full_details: bool = False
) -> Any:
    """Recommend VM specs matching resource, location, or cost preferences.

    This is the only supported source of spec ids: the returned spec["id"] must be passed
    to search_images() and create_infra_dynamic() unchanged. Filter and priority vocabulary:
    get_search_options(kind='spec'). Worked example: get_usage_guide('provision').

    Args:
        filter_policies: Field -> constraint, e.g.
            {"vCPU": {"min": 2, "max": 8}, "memoryGiB": {"min": 4},
             "providerName": "aws", "regionName": "ap-northeast-2"}
            For a GPU, filter on the memory you need rather than the card's name:
            {"acceleratorModel": "l4", "acceleratorMemoryGB": {"min": 16}}. The name alone
            does not say how much of the card you get - AWS g6f.large reports "NVIDIA L4"
            and costs a quarter of g6.xlarge because it is a 3 GB slice of one, and sorting
            by cost puts it first. acceleratorCount 0 marks such a slice.
            Full vocabulary: get_search_options(kind='spec').
        priority_policy: "location" | "cost" | "performance" | "random"
        latitude: Reference latitude. Required when priority_policy is "location".
        longitude: Reference longitude. Required when priority_policy is "location".
        limit: Maximum number of specs to return (string)
        include_full_details: Return every spec field instead of the summary
            (default False; the full form is much larger).

    Returns:
        summarized_specs: list of specs with id, providerName, regionName, vCPU,
        memoryGiB, costPerHour (-1 means price unknown).
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
            "weight": 1.0
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
        "limit": int(limit),
        "priority": {
            "policy": priority_policies
        }
    }
    
    # Make API request
    raw_response = api_request("POST", "/recommendSpec", json_data=data)

    # Summarize response to reduce token usage
    summary = _summarize_vm_specs(raw_response, include_details=include_full_details)
    return _demote_unpriced(summary, priority_policy)

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

# Helper function: Internal Infra dynamic validation (used by both review and create functions)
def _internal_review_infra_dynamic(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "Infra created dynamically via MCP",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_commands: Optional[List[Dict]] = None,
    post_command_async: bool = False,
    hold: bool = False,
    policy_on_partial_failure: str = "continue",
    vnet_template_id: str = "",
    sg_template_id: str = ""
) -> Dict:
    """Internal helper function to review Infra dynamic configuration"""
    # Build request data according to model.InfraDynamicReq spec
    data = {
        "name": name,
        "description": description,
        "nodeGroups": _normalize_node_groups(node_groups),
        "policyOnPartialFailure": policy_on_partial_failure
    }
    
    # Add optional parameters
    if system_label:
        data["systemLabel"] = system_label
    if label:
        data["label"] = label
    if post_commands:
        data["postCommands"] = post_commands
    if post_command_async:
        data["postCommandAsync"] = True
    if vnet_template_id:
        data["vNetTemplateId"] = vnet_template_id
    if sg_template_id:
        data["sgTemplateId"] = sg_template_id

    # Make API request to review endpoint (hold is a query option, not a body field)
    url = f"/ns/{ns_id}/infraDynamicReview"
    if hold:
        url += "?option=hold"
    result = api_request("POST", url, json_data=data)
    
    # 🔍 ENHANCED: Add historical risk analysis for each VM configuration
    if isinstance(result, dict) and "nodeReviews" in result:
        enhanced_risk_analysis = []
        
        for i, vm_review in enumerate(result["nodeReviews"]):
            vm_config = node_groups[i] if i < len(node_groups) else {}
            spec_id = vm_config.get("specId")
            image_name = vm_config.get("imageId")
            
            # Get historical risk analysis for this spec
            risk_analysis = {}
            if spec_id:
                try:
                    # Get basic risk analysis
                    risk_result = get_provisioning_risk(spec_id, image_name, kind="summary")
                    if "error" not in risk_result:
                        risk_analysis["basic_risk"] = risk_result
                        
                        # Add detailed risk if high or medium risk detected
                        risk_level = risk_result.get("riskLevel", "unknown")
                        if risk_level in ["high", "medium"]:
                            detailed_risk = get_provisioning_risk(spec_id, image_name, kind="detailed")
                            if "error" not in detailed_risk:
                                risk_analysis["detailed_risk"] = detailed_risk
                        
                        # Get provisioning history for context
                        history = get_provisioning_risk(spec_id, kind="history")
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
            result["issues"].append(f"{high_risk_count} VM(s) have HIGH provisioning failure risk based on historical data")
            
            if "recommendations" not in result:
                result["recommendations"] = []
            result["recommendations"].append("Consider using alternative specs with lower failure rates")
            result["recommendations"].append("Use get_provisioning_risk(kind='detailed') for specific guidance on high-risk VMs")
    
    # Enhance result with additional guidance based on review results
    if isinstance(result, dict):
        # 🚀 ENHANCED: Analyze nodeReviews for specific guidance and reconfiguration needs
        vm_reviews = result.get("nodeReviews", [])
        failed_vms = []
        provisioning_issues = []
        reconfiguration_needed = False
        
        # Analyze each VM review for specific issues
        for i, vm_review in enumerate(vm_reviews):
            vm_config = node_groups[i] if i < len(node_groups) else {}
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
            result["_guidance"] = "✅ Validation passed! You can proceed with create_infra_dynamic() using the same parameters."
            result["_next_step"] = f"create_infra_dynamic(ns_id='{ns_id}', name='{name}', node_groups=<same_configurations>)"
            
            # Add warnings for hold mode or other special cases
            if hold:
                result["_guidance"] += "\n⚠️ Note: VMs requiring manual deployment will need completion after hold mode."
        else:
            result["_guidance"] = "❌ Validation failed. Please address the issues before proceeding with Infra creation."
            
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
                result["_guidance"] += "\n  2. Update node_groups with working specs and run review again"
                result["_guidance"] += "\n  3. Consider using hold=True for providers requiring manual deployment"
                
                # Add specific provider guidance from review results
                for issue in provisioning_issues:
                    if "hold" in issue["issue"].lower():
                        result["_guidance"] += f"\n  • For {issue['provider']}: Set hold=True in create_infra_dynamic()"
                    elif "not available" in issue["issue"].lower():
                        result["_guidance"] += f"\n  • For {issue['provider']}: Replace with alternative provider (AWS, Azure, GCP, etc.)"
            
            result["_next_step"] = "Fix the reported issues and run review_infra_dynamic_request() again."
        
        # A review that says "viable" is routinely read as "this will succeed". It cannot
        # be: quota, live stock and provider-internal faults are only discoverable by asking
        # the provider to create something. Three real runs passed review and then failed on
        # exactly those. Stating the scope turns a surprise into a planned retry.
        result["_validation_scope"] = {
            "checked": ["spec exists and is offered in the region",
                        "image exists and is usable with that spec",
                        "list price"],
            "not_checked": ["account quota for the instance family",
                            "live capacity or stock in the target zones",
                            "provider-internal errors raised at creation time"],
            "implication": ("creationViable=true means nothing was found wrong, not that "
                            "creation will succeed - be ready to substitute a spec, region "
                            "or provider and retry"),
        }

        # Node groups are not nodes. The server reports totalNodeCount as the number of
        # groups - 2 for a "1 control + 4 workers" cluster - while pricing the same request
        # correctly at five machines. Counting from nodeGroupSize here keeps the two halves
        # of the answer agreeing; the server side is reported separately.
        def _true_node_count(reviews):
            total = 0
            for review in reviews or []:
                try:
                    size = int(review.get("nodeGroupSize") or 1)
                except (TypeError, ValueError):
                    size = 1
                total += max(1, size)
            return total

        node_total = _true_node_count(vm_reviews)

        # Add summary of nodeReviews for LLM reference
        if vm_reviews:
            result["_vm_summary"] = {
                "total_vms": node_total,
                "total_node_groups": len(vm_reviews),
                "successful_vms": len([vm for vm in vm_reviews if vm.get("canCreate", True)]),
                "failed_vms": len([vm for vm in vm_reviews if not vm.get("canCreate", True)]),
                "providers_used": list(set([vm.get("providerName", "") for vm in vm_reviews if vm.get("providerName")])),
                "review_details": "Check nodeReviews field for detailed per-VM validation results"
            }
        
        # Add enhanced summary with corrected VM count and cost information
        if "totalNodeCount" in result and "estimatedCost" in result:
            vm_count = node_total or result["totalNodeCount"]
            estimated_cost = result.get("estimatedCost", "Cost estimation unavailable")
            result["_deployment_summary"] = {
                "total_vms_to_deploy": vm_count,
                "estimated_hourly_cost": estimated_cost,
                "estimated_monthly_cost": f"~${float(estimated_cost.replace('$', '').replace('/hour', '')) * 24 * 30:.2f}/month" if "$" in estimated_cost and "/hour" in estimated_cost else "Estimate unavailable",
                "node_groups": len(vm_reviews),
                "note": ("total_vms_to_deploy sums nodeGroupSize across groups. The server's "
                         "own totalNodeCount reports the number of groups, not machines - "
                         "its cost estimate is per machine and correct.")
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
🎯 **Infra Creation Plan Validated Successfully!**

📊 **Deployment Summary:**
• Total VMs: {result.get('totalNodeCount', 'N/A')}
• Estimated Cost: {result.get('estimatedCost', 'N/A')}
• Status: All configurations validated ✅

💰 **Cost Information:**
• Hourly: {result.get('estimatedCost', 'N/A')}
• Monthly (estimated): {result.get('_deployment_summary', {}).get('estimated_monthly_cost', 'N/A')}

🔧 **Specifications:**
{_format_vm_summary_for_user(vm_reviews)}

**Would you like to proceed with creating this Infra infrastructure?**
- ✅ **Yes** - I'll create the Infra with these specifications
- ❌ **No** - I want to modify the configuration first
- 📋 **Details** - Show me more detailed validation results

*Reply with your choice to continue.*
""",
                "confirmation_required": True,
                "proceed_command": "create_infra_dynamic(..., force_create=True)"
            }
        elif creation_viable and overall_status == "Warning":
            result["_llm_guidance"] = {
                "status": "READY_WITH_WARNINGS",
                "message": "⚠️ Configuration has warnings but can proceed.",
                "next_action": "SHOW_WARNINGS_AND_ASK_CONFIRMATION",
                "user_prompt": f"""
⚠️ **Infra Creation Plan - Warnings Detected**

📊 **Deployment Summary:**
• Total VMs: {result.get('totalNodeCount', 'N/A')}
• Estimated Cost: {result.get('estimatedCost', 'N/A')}
• Status: Can create with warnings ⚠️

⚠️ **Warnings Found:**
{_format_warnings_for_user(vm_reviews)}

💰 **Cost Information:**
• Hourly: {result.get('estimatedCost', 'N/A')}
• Monthly (estimated): {result.get('_deployment_summary', {}).get('estimated_monthly_cost', 'N/A')}

**Despite the warnings, would you like to proceed with creating this Infra?**
- ✅ **Yes** - Proceed anyway (warnings are acceptable)
- ❌ **No** - I want to fix the warnings first
- 📋 **Details** - Show me detailed warning information

*Please review the warnings and let me know your decision.*
""",
                "confirmation_required": True,
                "proceed_command": "create_infra_dynamic(..., force_create=True)"
            }
        else:
            result["_llm_guidance"] = {
                "status": "CANNOT_CREATE",
                "message": "❌ Configuration has errors that must be fixed before creation.",
                "next_action": "SHOW_ERRORS_AND_GUIDE_FIXES",
                "user_prompt": f"""
❌ **Infra Creation Plan - Errors Must Be Fixed**

📊 **Review Results:**
• Total VMs: {result.get('totalNodeCount', 'N/A')}
• Status: Cannot create due to errors ❌

🚫 **Errors Found:**
{_format_errors_for_user(vm_reviews)}

🔧 **Recommended Actions:**
1. Use `recommend_vm_spec()` to get alternative VM specifications
2. Check `search_images()` for compatible images in different regions
3. Verify CSP provider availability and quotas
4. Update node_groups and run review again

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
            "Always run review_infra_dynamic_request() before create_infra_dynamic()",
            "Address all critical issues (errors) before deployment",
            "Consider optimization suggestions for better performance",
            "Use hold=True in create_infra_dynamic() for manual review if needed",
            "NodeGroup sizes are automatically calculated for accurate VM counts and costs"
        ]
    
    return result

# Tool: Review Infra Dynamic Request (Pre-validation)
@tool()
def review_infra_dynamic_request(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "Infra created dynamically via MCP",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_commands: Optional[List[Dict]] = None,
    post_command_async: bool = False,
    hold: bool = False,
    policy_on_partial_failure: str = "continue",
    vnet_template_id: str = "",
    sg_template_id: str = ""
) -> Dict:
    """Validate an Infra configuration and estimate its cost before creating it.

    Run this before create_infra_dynamic(). It resolves every spec and image against the
    CSP, reports per-node availability, estimates hourly cost, and flags configurations
    that cannot be provisioned. Same arguments as create_infra_dynamic().
    Details: get_usage_guide('provision').

    Args:
        ns_id: Namespace ID
        name: Infra name
        node_groups: NodeGroup dicts - see create_infra_dynamic()
        description: Infra description
        policy_on_partial_failure: "continue" (default) | "rollback" | "refine"
        vnet_template_id: VNet template ID, Infra-level default
        sg_template_id: Security group template ID, Infra-level default
        system_label: Reserved system label; leave empty unless told otherwise.
        label: Key-value pairs applied to the Infra (enables labelSelector targeting later).
        post_commands: Bootstrap phases to run once the nodes are up.
        post_command_async: Return as soon as the nodes exist and let post_commands run
            in the background (default False).
        hold: Create the records but leave provisioning paused, to be released later.

    Returns:
        overallStatus ("Ready" | "Warning" | "Error"), creationViable, estimatedCost,
        per-node validation in nodeReviews, and resourceSummary. Fix anything reported as
        an error and re-review until creationViable is true.
    """
    return _internal_review_infra_dynamic(
        ns_id=ns_id,
        name=name,
        node_groups=node_groups,
        description=description,
        system_label=system_label,
        label=label,
        post_commands=post_commands,
        post_command_async=post_command_async,
        hold=hold,
        policy_on_partial_failure=policy_on_partial_failure,
        vnet_template_id=vnet_template_id,
        sg_template_id=sg_template_id
    )

# # Tool: Create Infra (Traditional method)
# @mcp.tool()
# def create_infra(
#     ns_id: str,
#     name: str,
#     description: str = "Created via MCP",
#     vm_config: List[Dict] = None,
#     post_command: Optional[Dict] = None,
#     hold: bool = False
# ) -> Dict:
#     """
#     Create Multi-Cloud Infrastructure using traditional method with detailed VM configurations.
#     For easier Infra creation, consider using create_infra_dynamic() instead.
    
#     Args:
#         ns_id: Namespace ID
#         name: Infra name
#         description: Infra description
#         vm_config: Detailed VM configuration list with specific resource IDs
#         post_command: Post-deployment command configuration
#         hold: Whether to hold provisioning
    
#     Returns:
#         Created Infra information
#     """
#     if vm_config is None:
#         vm_config = []
    
#     data = {
#         "name": name,
#         "description": description,
#         "nodeGroups": vm_config
#     }
    
#     if post_command:
#         data["postCommand"] = post_command
    
#     url = f"/ns/{ns_id}/infra"
#     if hold:
#         url += "?option=hold"
    
#     return api_request("POST", url, json_data=data)

# Tool: Create Infra dynamically (Recommended method)
@tool(mutating=True)
def create_infra_dynamic(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "Infra created dynamically via MCP",
    system_label: str = "",
    label: Optional[Dict[str, str]] = None,
    post_commands: Optional[List[Dict]] = None,
    post_command_async: bool = False,
    hold: bool = False,
    skip_confirmation: bool = False,
    force_create: bool = False,
    policy_on_partial_failure: str = "continue",
    vnet_template_id: str = "",
    sg_template_id: str = ""
, budget_ack: Optional[str] = None, confirm: bool = False) -> Dict:
    """Create a Multi-Cloud Infrastructure.

    Two steps: call review_infra_dynamic_request() first, then this with force_create=True.
    Without force_create it returns the review requirement instead of provisioning.

    specId and imageId must be copied verbatim from recommend_vm_spec() and search_images(),
    and must come from the same provider and region - a guessed or edited id fails at the CSP.
    Worked example: get_usage_guide('provision').

    Args:
        ns_id: Namespace ID
        name: Infra name
        node_groups: NodeGroup dicts (maps to the server's nodeGroups). Per entry:
            specId (required; exact from recommend_vm_spec, "provider+region+type"),
            imageId (required; exact cspImageName from search_images),
            name, description, nodeGroupSize (int, default 1), connectionName,
            rootDiskSize (int GB, 0 = CSP default), rootDiskType, zone,
            vNetTemplateId, sgTemplateId, os_requirements,
            label: key-value pairs. A role label here is what label_selector targets
            later, so {"role":"control"} / {"role":"node"} lets one remote command
            reach a whole tier.
        description: Infra description
        system_label: System label for special purposes
        label: Key-value pairs for Infra labeling
        post_commands: Bootstrap phases, each targeted by at most one of
            nodeGroupId | nodeId | labelSelector. Omit userName so the server resolves the
            verified account per node. Phases run in order; a failed one skips the rest by
            default. Cumulative timeoutMinutes must stay under 120. Outcome comes back as
            postCommandStatus / postCommandResults. Worked example:
            get_usage_guide('provision').
        post_command_async: Return once nodes are provisioned and run the commands in the
            background. Recommended for long bootstraps - nodes bill from Running and a
            synchronous wait can hit proxy timeouts. Poll get_infra() until
            postCommandStatus is terminal, or stream using postCommandRequestId.
        hold: Hold provisioning for review
        skip_confirmation: Skip the confirmation step
        force_create: Confirms the configuration was reviewed
        confirm: Confirms a person has seen the plan and agreed to it. Both are required:
            force_create says the machine checks passed, confirm says a human said yes.
        policy_on_partial_failure: "continue" (default) | "rollback" | "refine"
        vnet_template_id: VNet template ID, Infra-level default
        sg_template_id: Security group template ID, Infra-level default
        budget_ack: Approval id from request_budget_increase(), to proceed past a
            spend limit. Single-use.

    Returns:
        Without force_create: the review requirement and next steps.
        With force_create: the created Infra - id, node list, status, deployment summary.
    """
    unconfirmed = _confirm_creation_plan(node_groups, name, confirm)
    if unconfirmed:
        return unconfirmed
    over_budget = _check_budget(node_groups, budget_ack)
    if over_budget:
        return over_budget
    
    # CRITICAL: Enforce explicit review step before Infra creation
    if not force_create and not skip_confirmation:
        return {
            "error": "❌ Infra creation requires prior validation - MANDATORY WORKFLOW",
            "message": "🚨 STOP: You MUST run review_infra_dynamic_request() first before creating Infra!",
            "workflow_violation": "REVIEW_STEP_SKIPPED",
            "required_workflow": [
                "1. 🔍 MANDATORY: Call review_infra_dynamic_request() with the same parameters",
                "2. 📋 EXAMINE: Review validation results, cost estimates, and any warnings/errors", 
                "3. 🔧 FIX: Address any validation issues if needed",
                "4. ✅ CREATE: Call create_infra_dynamic() with force_create=True to proceed"
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
                "message_to_user": "I need to validate this Infra configuration first to ensure it will work and show you the cost estimates. Let me run the review step.",
                "next_function_call": f"review_infra_dynamic_request(ns_id='{ns_id}', name='{name}', node_groups=<same_configurations>)",
                "after_review": "After review completes, I'll show you the results and ask for confirmation before creating the infrastructure."
            },
            "example_code": f'''
# STEP 1: Review first (MANDATORY)
review_result = review_infra_dynamic_request(
    ns_id="{ns_id}",
    name="{name}",
    node_groups=node_groups
)

# STEP 2: Check results and create if valid
if review_result.get("overallStatus") == "Ready":
    infra = create_infra_dynamic(
        ns_id="{ns_id}",
        name="{name}",
        node_groups=node_groups,
        force_create=True
    )
''',
            "critical_note": "🚨 This error is intentional to enforce proper workflow. Do NOT bypass this step."
        }
    
    # STEP 1: User confirmation workflow (unless explicitly skipped or forced)
    if not skip_confirmation and not force_create:
        # Generate comprehensive creation summary with cost analysis
        creation_summary = generate_infra_creation_summary(
            ns_id=ns_id,
            name=name,
            node_groups=node_groups,
            description=description,
            hold=hold
        )
        
        # Add creation parameters for easy re-execution
        creation_summary["_CREATION_PARAMETERS"] = {
            "ns_id": ns_id,
            "name": name,
            "node_groups": node_groups,
            "description": description,
            "system_label": system_label,
            "label": label,
            "post_commands": post_commands,
            "hold": hold
        }
        
        # Add clear next action instructions
        creation_summary["_NEXT_ACTION"] = {
            "action_required": "USER_CONFIRMATION",
            "message": "📋 Please review the Infra creation plan above, including cost estimates and deployment strategy.",
            "to_proceed": {
                "description": "After reviewing, call this function again with force_create=True to proceed with deployment",
                "function_call": f"create_infra_dynamic(ns_id='{ns_id}', name='{name}', node_groups=<same_configurations>, force_create=True)",
                "alternative": "Or use skip_confirmation=True if you want to skip future confirmations"
            },
            "to_modify": {
                "description": "To modify the configuration, adjust node_groups and run this function again",
                "options": [
                    "Modify vm specs, images, or counts in node_groups",
                    "Change namespace, description, or other parameters",
                    "Add or remove VMs from the configuration"
                ]
            }
        }
        
        # Return summary without creating Infra
        return creation_summary
    
    # STEP 1: Validate namespace first
    ns_validation = _internal_validate_namespace(ns_id)
    if not ns_validation["valid"]:
        return {
            "error": f"Namespace '{ns_id}' validation failed",
            "details": ns_validation.get("error", "Unknown error"),
            "suggestion": ns_validation.get("suggestion", ""),
            "namespace_guidance": "Use manage_namespace(action='create') to see available namespaces or manage_namespace(action='create') to create a new one"
        }
    
    # Validate required VM configuration fields and auto-map images if needed
    processed_vm_configs = []
    for i, vm_config in enumerate(node_groups):
        vm_config = dict(vm_config)  # Work on a copy — don't mutate the caller's dicts
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
    
    # Build request data according to model.InfraDynamicReq spec
    # (strip internal "_"-prefixed bookkeeping keys and non-API fields from each node group)
    data = {
        "name": name,
        "nodeGroups": _normalize_node_groups([
            {k: v for k, v in cfg.items() if not k.startswith("_") and k != "os_requirements"}
            for cfg in processed_vm_configs
        ]),
        "policyOnPartialFailure": policy_on_partial_failure
    }
    
    # Add optional fields
    if description:
        data["description"] = description
    # Only include installMonAgent if explicitly set to "yes"
    if system_label:
        data["systemLabel"] = system_label
    if label:
        data["label"] = label
    if post_commands:
        data["postCommands"] = post_commands
    if post_command_async:
        data["postCommandAsync"] = True
    if vnet_template_id:
        data["vNetTemplateId"] = vnet_template_id
    if sg_template_id:
        data["sgTemplateId"] = sg_template_id
    
    url = f"/ns/{ns_id}/infraDynamic"
    if hold:
        url += "?option=hold"
    
    result = api_request("POST", url, json_data=data)
    
    # Store interaction in memory for future reference
    context_data = {
        "namespace_id": ns_id,
        "infra_name": name,
        "vm_count": len(node_groups),
        "hold": hold
    }
    
    if "error" not in result:
        # Add namespace info to result for reference
        result["namespace_info"] = {
            "namespace_id": ns_id,
            "validation": "passed"
        }
        
        # Store successful Infra creation with auto-mapping details
        auto_mapped_count = sum(1 for vm in processed_vm_configs if vm.get("_auto_mapped_image", False))
        context_data["infra_id"] = result.get("id", name)
        context_data["auto_mapped_images"] = auto_mapped_count
        context_data["total_vms"] = len(processed_vm_configs)
        context_data["spec_to_image_mapping"] = "applied"
        
        _store_interaction_memory(
            user_request=f"Create Infra '{name}' with {len(processed_vm_configs)} VM configurations using spec-aware image selection",
            llm_response=f"Successfully created Infra '{name}' (ID: {result.get('id', name)}) with proper spec-to-image mapping in namespace '{ns_id}'. Auto-mapped {auto_mapped_count}/{len(processed_vm_configs)} images.",
            operation_type="infra_creation",
            context_data=context_data,
            status="completed"
        )
    else:
        # Store failed Infra creation
        _store_interaction_memory(
            user_request=f"Create Infra '{name}' with {len(processed_vm_configs)} VM configurations",
            llm_response=f"Failed to create Infra '{name}': {result.get('error', 'Unknown error')}",
            operation_type="infra_creation",
            context_data=context_data,
            status="failed"
        )
    
    return result

# Note: create_infra_with_proper_spec_mapping has been removed.
# Use create_infra_dynamic with proper spec-to-image mapping workflow instead.

# Note: create_infra_with_spec_first has been removed.
# Use create_infra_dynamic with spec-first workflow as described in prompts.

# Note: create_infra_with_namespace_management has been removed.
# Use create_infra_dynamic with namespace management workflow as described in prompts.

# Tool: Review Spec-Image Pair
@tool()
def review_spec_image_pair(
    spec_id: str,
    image_id: str
) -> Dict:
    """
    Lightweight validation of a specId + imageId pair without creating any infrastructure.
    
    Use this to quickly check if a spec and image combination is valid and available
    before building full Infra configurations. Much faster than review_infra_dynamic_request().
    
    **Workflow:**
    ```python
    # Quick check before building full Infra config
    result = review_spec_image_pair(
        spec_id="aws+ap-northeast-2+t3.nano",
        image_id="ami-01f71f215b23ba262"
    )
    if result.get("isValid"):
        # Proceed to build full Infra configuration
        ...
    ```

    Args:
        spec_id: VM specification ID (e.g., "aws+ap-northeast-2+t3.nano") (REQUIRED)
        image_id: CSP-specific image ID (e.g., "ami-01f71f215b23ba262") (REQUIRED)

    Returns:
        Validation result including:
        - isValid: Whether the spec+image pair is valid and available
        - status / message: Overall review outcome
        - specDetails: Spec details (vCPU, memory, cost)
        - imageDetails: Image details (OS, architecture)
        - availability, specValidation, imageValidation: Detailed checks
        - estimatedCost (string), warnings, errors, suggestedZone
    """
    data = {
        "specId": spec_id,
        "imageId": image_id
    }
    return api_request("POST", "/specImagePairReview", json_data=data)

# Tool: Add NodeGroup to existing Infra dynamically
@tool(mutating=True)
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
    post_commands: Optional[List[Dict]] = None,
    post_command_async: bool = False
) -> Dict:
    """
    Add a new NodeGroup of VMs to an existing Infra dynamically.
    
    Use this to scale out an existing Infra by adding more VMs with a new spec/image.
    The NodeGroup will be added to the existing Infra without affecting other VMs.
    
    **Example:**
    ```python
    # Add 3 worker VMs to existing Infra
    result = add_nodegroup_dynamic(
        ns_id="default",
        infra_id="my-infra",
        spec_id="aws+ap-northeast-2+t3.medium",
        image_id="ami-0c02fb55956c7d316",
        name="worker-group",
        node_group_size=3,
        description="Worker nodes"
    )
    ```
    
    Args:
        ns_id: Namespace ID (REQUIRED)
        infra_id: Infra ID to add the nodegroup to (REQUIRED)
        spec_id: VM specification ID from recommend_vm_spec() (REQUIRED)
        image_id: CSP-specific image ID from search_images() (REQUIRED)
        name: NodeGroup name (optional)
        node_group_size: Number of VMs in the nodegroup (int, default 1)
        description: NodeGroup description (optional)
        root_disk_type: Root disk type (optional)
        root_disk_size: Root disk size in GB (int, 0 for CSP default)
        connection_name: Specific connection name (optional)
        zone: Availability zone (optional, e.g., "ap-northeast-2a")
        vnet_template_id: VNet template ID (optional)
        sg_template_id: Security group template ID (optional)
        label: Key-value pairs for labeling (optional; enables labelSelector targeting later)
        post_command_async: Return as soon as the nodes exist and let post_commands run
            in the background (default False).
        post_commands: Bootstrap phases for the NEW nodes (scoped to this nodeGroup), e.g.
            [{"command": ["curl ... | bash"], "timeoutMinutes": 10, "continueOnError": False}]
    
    Returns:
        Updated Infra information including the newly added NodeGroup
    """
    data = {
        "specId": spec_id,
        "imageId": image_id,
        "nodeGroupSize": node_group_size
    }
    if name:
        data["name"] = name
    if description:
        data["description"] = description
    if root_disk_type:
        data["rootDiskType"] = root_disk_type
    if root_disk_size:
        data["rootDiskSize"] = root_disk_size
    if connection_name:
        data["connectionName"] = connection_name
    if zone:
        data["zone"] = zone
    if vnet_template_id:
        data["vNetTemplateId"] = vnet_template_id
    if sg_template_id:
        data["sgTemplateId"] = sg_template_id
    if label:
        data["label"] = label
    # Bootstrap commands run on the NEW nodes only (phases without an explicit
    # target are scoped to this nodeGroup by the server)
    if post_commands:
        data["postCommands"] = post_commands
    if post_command_async:
        data["postCommandAsync"] = True
    
    return api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/nodeGroupDynamic", json_data=data)

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
    post_commands: Optional[List[Dict]] = None,
    post_command_async: bool = False
) -> Dict:
    """
    Review/validate a NodeGroup configuration before adding it to an existing Infra.
    
    Similar to review_infra_dynamic_request but for a single NodeGroup being added
    to an already-running Infra. Use this before add_nodegroup_dynamic().
    
    Args:
        ns_id: Namespace ID (REQUIRED)
        infra_id: Infra ID (REQUIRED)
        spec_id: VM specification ID (REQUIRED)
        image_id: CSP-specific image ID (REQUIRED)
        name: NodeGroup name (optional)
        node_group_size: Number of VMs (int, default 1)
        description: NodeGroup description (optional)
        root_disk_type: Root disk type (optional)
        root_disk_size: Root disk size in GB (int, 0 for CSP default)
        connection_name: Specific connection name (optional)
        zone: Availability zone (optional)
        post_commands: Bootstrap phases to validate (same shape as add_nodegroup_dynamic)
        post_command_async: Whether the bootstrap would run in the background
    
    Returns:
        Validation result including spec/image availability, cost estimation,
        and compatibility with the existing Infra
    """
    data = {
        "specId": spec_id,
        "imageId": image_id,
        "nodeGroupSize": node_group_size
    }
    if name:
        data["name"] = name
    if description:
        data["description"] = description
    if root_disk_type:
        data["rootDiskType"] = root_disk_type
    if root_disk_size:
        data["rootDiskSize"] = root_disk_size
    if connection_name:
        data["connectionName"] = connection_name
    if zone:
        data["zone"] = zone
    # Review accepts the same body as add_nodegroup_dynamic, so a reviewed request
    # can be submitted unchanged (bootstrap phases are validated here too)
    if post_commands:
        data["postCommands"] = post_commands
    if post_command_async:
        data["postCommandAsync"] = True

    return api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/nodeGroupDynamicReview", json_data=data)

# Tool: Delete Infra
@tool(mutating=True)
def terminate_infra(ns_id: str, infra_id: str, confirm: bool = False) -> Dict:
    """
    Delete an Infra.
    This operation will terminate all VMs in the Infra and delete the Infra.

    Named for what it does: it terminates. Deleting the CB-Tumblebug record without
    terminating at the provider is deliberately not offered here - that path leaves billable
    instances running with nothing tracking them. Use MapUI or the REST API if a
    metadata-only removal is ever genuinely needed.

    Safety: always runs with option=terminate (CSP resources are terminated
    first). Forced metadata-only deletion is not available through this tool because it
    orphans running instances. If the API reports that termination is still in progress,
    wait a few minutes and call this tool again.
    This operation is irreversible and should be used with caution.
    This operation requires confirmation from the user.
    
    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        confirm: Must be True to proceed. Ask the user first - this is the only
            thing standing between a request and the destructive action.
    
    Returns:
        Deletion result with guidance about optional shared resources cleanup
    """
    guard = _require_confirmation(
        confirm, "terminating infrastructure",
        "every node in this Infra is terminated at the provider; this cannot be undone")
    if guard:
        return guard
    # Get Infra associated resources before deletion for guidance
    try:
        associated_resources = get_infra_associated_resources(ns_id, infra_id)
    except:
        associated_resources = None
    
    # Delete the Infra.
    # Always option=terminate: it terminates CSP nodes before removing CB-TB records.
    # option=force is deliberately NOT exposed here — it drops CB-TB metadata without
    # confirming CSP termination, leaving orphaned (billing) instances that also block
    # VNet/SecurityGroup cleanup. If deletion is rejected because termination is still
    # in progress, retry in a few minutes instead of forcing.
    result = api_request("DELETE", f"/ns/{ns_id}/infra/{infra_id}?option=terminate")
    
    # Add shared resources cleanup guidance to the result
    if isinstance(result, dict) and "error" not in result:
        # Store deletion in memory
        _store_interaction_memory(
            user_request=f"Delete Infra '{infra_id}' in namespace '{ns_id}'",
            llm_response=f"Successfully deleted Infra '{infra_id}'",
            operation_type="infra_management",
            context_data={"namespace_id": ns_id, "infra_id": infra_id, "operation": "delete"},
            status="completed"
        )
        
        # Add shared resources cleanup guidance
        result["next_steps_guidance"] = {
            "infra_deletion": "✅ Infra deletion completed successfully",
            "optional_cleanup": {
                "title": "🔧 Optional: Clean up shared resources",
                "description": "Your Infra used shared resources (VNet, SecurityGroup, SSHKey) that remain in the namespace. These can be reused by future Infras or cleaned up if no longer needed.",
                "when_to_cleanup": [
                    "This was the last Infra in the namespace",
                    "You won't create new Infras that could reuse these network configurations",
                    "You want to completely clean up the namespace"
                ],
                "when_to_keep": [
                    "Other Infras exist in the same namespace",
                    "You plan to create new Infras soon",
                    "You want to keep network configurations for future use"
                ],
                "cleanup_command": f"release_resources('{ns_id}', confirm=True)",
                "check_command": f"get_infra_list('{ns_id}') # Check if other Infras exist",
                "warning": "⚠️  Shared resources cleanup is IRREVERSIBLE and affects the entire namespace"
            }
        }
        
        # Add associated resources information if available
        if associated_resources and isinstance(associated_resources, dict) and "error" not in associated_resources:
            result["next_steps_guidance"]["associated_resources"] = {
                "description": "The deleted Infra was using these shared resources:",
                "vNets": associated_resources.get("vNetIds", []),
                "securityGroups": associated_resources.get("securityGroupIds", []),
                "sshKeys": associated_resources.get("sshKeyIds", []),
                "note": "These resources remain available for future Infras unless explicitly released"
            }
    
    return result

# Tool: Control Infra
@tool(mutating=True)
def control_infra(ns_id: str, infra_id: str, action: str, force: bool = False) -> Dict:
    """
    Control an Infra. Control action (refine, suspend, resume, reboot, terminate, continue, withdraw, reconcile, abort)

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        action: Control action (refine, suspend, resume, reboot, terminate, continue, withdraw, reconcile, abort)
        force: Force the action even if the Infra is in an in-progress state

    Returns:
        Control result
    """
    valid_actions = ["refine", "suspend", "resume", "reboot", "terminate", "continue", "withdraw", "reconcile", "abort"]
    if action not in valid_actions:
        return {"error": f"Unsupported action: {action}. Supported actions: {', '.join(valid_actions)}"}

    params = {"action": action}
    if force:
        params["force"] = "true"
    return api_request("GET", f"/ns/{ns_id}/control/infra/{infra_id}", params=params)

#####################################
# NLB Management (Network Load Balancer)
#####################################

# # Tool: Create Multi-Cloud NLB
# @mcp.tool()
# def create_mc_nlb(ns_id: str, infra_id: str, port: int = 80, type: str = "PUBLIC", scope: str = "REGION") -> Dict:
#     """
#     Create Multi-Cloud NLB
    
#     Args:
#         ns_id: Namespace ID
#         infra_id: Infra ID
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
    
#     return api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/mcSwNlb", json_data=data)

# # Tool: Create Regional NLB
# @mcp.tool()
# def create_region_nlb(
#     ns_id: str, 
#     infra_id: str, 
#     nodegroup_id: str, 
#     port: int = 80, 
#     type: str = "PUBLIC", 
#     scope: str = "REGION"
# ) -> Dict:
#     """
#     Create Regional NLB
    
#     Args:
#         ns_id: Namespace ID
#         infra_id: Infra ID
#         nodegroup_id: NodeGroup ID
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
#             "nodeGroupId": nodegroup_id
#         },
#         "HealthChecker": {
#             "Interval": "default",
#             "Timeout": "default",
#             "Threshold": "default"
#         }
#     }
    
#     return api_request("POST", f"/ns/{ns_id}/infra/{infra_id}/nlb", json_data=data)

# # Tool: Delete NLB
# @mcp.tool()
# def delete_nlb(ns_id: str, infra_id: str, nodegroup_id: str) -> Dict:
#     """
#     Delete NLB
    
#     Args:
#         ns_id: Namespace ID
#         infra_id: Infra ID
#         nodegroup_id: NodeGroup ID
    
#     Returns:
#         Deletion result
#     """
#     return api_request("DELETE", f"/ns/{ns_id}/infra/{infra_id}/nlb/{nodegroup_id}")

#####################################
# Memory & Interaction History Management
#####################################

# Helper function: Store user interaction in memory
def _store_interaction_memory(
    user_request: str,
    llm_response: str,
    operation_type: str,
    context_data: Optional[Dict] = None,
    status: str = "completed",
    **_ignored
) -> Dict:
    """Record an operation for the server log only.

    This used to append to a process-local list that every caller shared: it leaked one
    session's request text to the next, vanished on restart, and had no key. Nothing reads it
    back any more - session continuity is the wrapper's automatic journal in etcd, keyed by
    session_id - so what remains is a log line.
    """
    logger.info(f"operation: {operation_type} status={status} :: {str(user_request)[:200]}")
    return {"recorded": True}

# Helper function: Retrieve interaction history
def _resume_session(
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
        history = _resume_session(days_back=1, max_results=20)
        
        if not history.get("interactions"):
            return {
                "summary": "No recent interactions found",
                "key_operations": [],
                "recommendations": ["Start by checking available namespaces", "Review cloud connections"]
            }
        
        # Analyze interaction patterns
        operation_types = {}
        namespaces_used = set()
        infras_created = []
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
            
            # Extract Infra information
            infra_obs = next((obs for obs in observations if "infra" in obs.lower()), None)
            if infra_obs:
                infras_created.append(infra_obs)
            
            # Check for errors
            status_obs = next((obs for obs in observations if obs.startswith("Status:")), None)
            if status_obs and "failed" in status_obs.lower():
                errors_encountered.append(interaction["id"])
        
        # Generate recommendations
        recommendations = []
        if "infra_creation" in operation_types:
            recommendations.append("Monitor created Infras for optimal resource usage")
        if errors_encountered:
            recommendations.append("Review error logs and retry failed operations")
        if len(namespaces_used) > 3:
            recommendations.append("Consider consolidating resources into fewer namespaces")
        
        return {
            "summary": f"Found {len(history['interactions'])} recent interactions",
            "operation_summary": operation_types,
            "namespaces_used": list(namespaces_used),
            "infras_info": infras_created,
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
def _demote_unpriced(summary, priority_policy):
    """Under a cost priority, a spec with no published price must not rank as cheapest.

    costPerHour = -1 means "unknown", and the server sorts it ahead of every real price.
    Asked for the cheapest option in us-west-1, the recommender returned an unpriced spec
    that also turned out to be unprovisionable - the caller only found out at review.
    Unknown is not cheap, so unpriced specs move to the end and say why.
    """
    if str(priority_policy or "").lower() != "cost":
        return summary
    if not isinstance(summary, dict):
        return summary
    specs = summary.get("summarized_specs")
    if not isinstance(specs, list):
        return summary

    def unpriced(spec):
        try:
            return float(spec.get("costPerHour", -1)) < 0
        except (TypeError, ValueError):
            return True

    priced = [x for x in specs if not unpriced(x)]
    unknown = [x for x in specs if unpriced(x)]
    for spec in unknown:
        spec["_note"] = "price not published by the provider - ranked last under a cost priority"
    summary["summarized_specs"] = priced + unknown
    if unknown and priced:
        summary["_pricing"] = (f"{len(unknown)} of {len(specs)} specs publish no price and were "
                               f"moved to the end; they are not necessarily cheap")
    elif unknown:
        # Nothing here can be compared on cost. Saying so is more useful than returning an
        # arbitrary pick: whole provider/region combinations publish no prices at all
        # (alibaba us-west-1 has 247 specs and not one price), and a caller asked for the
        # cheapest option should hear that before it commits to one.
        where = {f"{x.get('providerName')}/{x.get('regionName')}" for x in unknown}
        summary["_pricing"] = (
            f"none of the {len(unknown)} matching specs in {', '.join(sorted(where))} "
            f"publishes a price, so none can be ranked by cost. If cost matters, try another "
            f"region or provider; if this one is required, treat the spend as unknown.")
    return summary


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
        elif "message" in specs_response and "id" not in specs_response:
            # API error response — surface it instead of an empty spec
            return {
                "error": specs_response["message"],
                "summarized_specs": [],
                "total_count": 0,
                "details_included": include_details,
                "summary_applied": True,
            }
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
        
        # Accelerator. The model name alone does not say how much of that card you get:
        # AWS g6f.large reports "NVIDIA L4" like g6.xlarge does, but is a fractional slice
        # with 3 GB against the full card's 24 GB. Count and memory are therefore reported
        # whenever the spec has an accelerator at all - including count 0, which is what a
        # fractional slice carries and what an `if` on the value silently dropped.
        if spec.get("acceleratorModel"):
            summarized_spec["acceleratorModel"] = spec.get("acceleratorModel")
            summarized_spec["acceleratorCount"] = spec.get("acceleratorCount", 0)
            summarized_spec["acceleratorMemoryGB"] = spec.get("acceleratorMemoryGB", 0)
            if not spec.get("acceleratorCount"):
                summarized_spec["_accelerator_note"] = (
                    "count 0 means a fractional GPU - acceleratorMemoryGB is the share you "
                    "actually get, not the card's full memory")
        if spec.get("acceleratorType"):
            summarized_spec["acceleratorType"] = spec.get("acceleratorType")
        
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
    summarize_output: bool = True,
    max_output_lines: Union[int, str] = 5,
    max_output_chars: Union[int, str] = 1000,
    timeout_minutes: Optional[int] = None
, run_async: bool = False, contains: Optional[str] = None) -> Dict:
    """Run shell commands on the nodes of an Infra over SSH.

    Each entry in commands runs in its own SSH session, so shell state does not carry over:
    a variable set in one entry is empty in the next. Put dependent steps in a single entry
    joined with && or ;. Predefined scripts: list_predefined_scripts().
    Targeting and long-running commands: get_usage_guide("commands").

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        commands: Shell commands. Each element is a separate SSH session.
        script_name: Run a predefined script instead of (or before) commands.
            See list_predefined_scripts().
        template_variables: Values for {{placeholder}} in the script or commands.
            public_ip / public_ips_space / public_ips_comma / private_ips_space /
            infra_id / ns_id are filled in automatically when left unset.
        node_id: Run on one node only
        nodegroup_id: Run on one NodeGroup only
        label_selector: Run on nodes matching a label, e.g. "role=node"
        timeout_minutes: Per-command timeout (server default 30, max 120)
        contains: Keep only output lines matching this regex (with 3 lines of context)
        summarize_output: Trim each stream to max_output_lines/max_output_chars
            (default True). Ignored when contains is given.
        max_output_lines: Lines kept per stream when summarizing (default 5)
        max_output_chars: Characters kept per stream when summarizing (default 1000)
            instead of summarizing. Reaches a block a tail cannot, e.g. the credentials
            an installer prints in the middle of a long log.
        run_async: Return a request_id immediately instead of waiting. Use it for anything
            slow - a driver install, a package build - so a dropped connection cannot lose
            the result. Follow up with get_command_result(request_id=...).

    Returns:
        Per-node results with stdout, stderr and error, plus the request id used to
        retrieve output later.
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
    
    # A predefined script contributes its commands ahead of any given explicitly.
    if script_name:
        if script_name not in PREDEFINED_SCRIPTS:
            return {"error": f"Script '{script_name}' not found",
                    "available_scripts": sorted(PREDEFINED_SCRIPTS.keys())}
        commands = list(PREDEFINED_SCRIPTS[script_name]["commands"]) + list(commands or [])

    # 🚨 CRITICAL: Validate commands before execution
    if not commands or len(commands) == 0:
        return {
            "error": "Empty command list provided",
            "message": "At least one command must be specified for execution",
            "suggestion": "Provide meaningful commands like 'ls -la', 'ps aux', 'df -h', etc."
        }

    commands = _substitute_template_variables(commands, template_variables, ns_id, infra_id)

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

    # The server defaults to 30 minutes and kills the session at that point. Long
    # installs (a DevStack build runs 20-40 minutes) must raise it or they are cut off
    # mid-way, leaving a half-configured host.
    request_timeout = None
    if timeout_minutes:
        capped = max(1, min(int(timeout_minutes), 120))  # server clamps to 1..120
        data["timeoutMinutes"] = capped
        # Give the HTTP read a margin over the command's own budget.
        request_timeout = (60, capped * 60 + 120)

    url = f"/ns/{ns_id}/cmd/infra/{infra_id}"
    params = {}
    
    if nodegroup_id:
        params["nodeGroupId"] = nodegroup_id
    if node_id:
        params["nodeId"] = node_id
    if label_selector:
        params["labelSelector"] = label_selector

    if run_async:
        # The server runs the command in the background and answers immediately with an id.
        # Results are collected by polling, not by streaming: an SSE feed of a long install
        # is tens of thousands of tokens of progress output that nobody reads, and the same
        # information is available trimmed once the command finishes.
        params["async"] = "true"
        started = api_request("POST", url, json_data=data, params=params,
                              timeout_override=(60, 120))
        request_id = None
        if isinstance(started, dict):
            request_id = (started.get("xRequestId") or started.get("x-request-id")
                          or started.get("requestId"))
        if not request_id:
            return started if isinstance(started, dict) else {"error": "no request id returned"}
        return {
            "started": True,
            "request_id": request_id,
            "note": ("running in the background; the connection can drop without losing it"),
            "how_to_follow": [
                f"get_command_result(ns_id='{ns_id}', infra_id='{infra_id}', "
                f"request_id='{request_id}') for the outcome",
                f"list_node_command_status(ns_id='{ns_id}', infra_id='{infra_id}', "
                f"node_id=...) per node",
            ],
        }

    result = api_request("POST", url, json_data=data, params=params or None,
                         timeout_override=request_timeout)

    # A grep beats a summary when the caller already knows what it is looking for.
    if contains and isinstance(result, dict) and "results" in result:
        for node_result in result["results"]:
            for stream in ("stdout", "stderr"):
                value = node_result.get(stream)
                if isinstance(value, dict):
                    node_result[stream] = {k: (_grep_stream(v, contains, 3)
                                               if isinstance(v, str) else v)
                                           for k, v in value.items()}
                elif isinstance(value, str):
                    node_result[stream] = _grep_stream(value, contains, 3)
        result["_note"] = f"output filtered to lines matching {contains!r}"
        return result

    # Apply output summarization if enabled
    if summarize_output and "results" in result:
        total_original_size = 0
        total_summarized_size = 0
        summarization_applied = False

        def _summarize_entry(text):
            nonlocal total_original_size, total_summarized_size, summarization_applied
            info = _summarize_command_output(text, max_output_lines, max_output_chars)
            total_original_size += info["original_length"]
            total_summarized_size += len(info["summary"])
            if info["truncated"]:
                summarization_applied = True
            return {
                "output": info["summary"],
                "truncated": info["truncated"],
                "original_length": info["original_length"],
                "lines_count": info["lines_count"]
            }

        for vm_result in result["results"]:
            for stream in ("stdout", "stderr"):
                value = vm_result.get(stream)
                # Current API returns a map of command-index -> output
                if isinstance(value, dict):
                    vm_result[stream] = {
                        k: (_summarize_entry(v) if isinstance(v, str) else v)
                        for k, v in value.items()
                    }
                elif isinstance(value, list):
                    vm_result[stream] = [
                        (dict(_summarize_entry(item), command_index=i) if isinstance(item, str) else item)
                        for i, item in enumerate(value)
                    ]
                elif isinstance(value, str):
                    vm_result[stream] = _summarize_entry(value)

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
        "infra_id": infra_id,
        "commands": commands,
        "vm_count": len(result.get("results", [])),
        "summarize_output": summarize_output
    }
    
    if nodegroup_id:
        context_data["nodegroup_id"] = nodegroup_id
    if node_id:
        context_data["node_id"] = node_id
    if label_selector:
        context_data["label_selector"] = label_selector
    
    # Determine success status ("error" per node result; whole call may also be an error dict)
    success_count = 0
    total_count = len(result.get("results", []))

    for vm_result in result.get("results", []):
        if not vm_result.get("error"):
            success_count += 1

    if "results" not in result:
        status = "failed"
    else:
        status = "completed" if success_count == total_count else "partial_failure" if success_count > 0 else "failed"
    
    _store_interaction_memory(
        user_request=f"Execute commands {commands} on Infra '{infra_id}' in namespace '{ns_id}'",
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
    # --- Nested-cloud scenario -------------------------------------------------
    # Mirrors the CB-MapUI usecase menus ("☁️ OpenStack (DevStack)" and
    # "☸️ Kubernetes"), so an agent can run the same flows an operator clicks
    # through. Placeholders are filled by
    # execute_command_infra(template_variables={...}) using the
    # double-brace form - not the <NAME> form CB-MapUI uses in its own templates.
    "devstack_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/1.installDevStack.sh -o /tmp/installDevStack.sh && bash /tmp/installDevStack.sh --csp-name {{csp_name}} --latitude {{latitude}} --longitude {{longitude}} --location \"{{location}}\""
        ],
        "description": "Install OpenStack (DevStack) on a BARE-METAL node. Requires a *.metal spec: nested KVM needs hardware virtualization, and ordinary instances fall back to slow QEMU emulation. Takes 20-40 minutes, so pass timeout_minutes=120. Safe to re-run: it attaches to an install in progress instead of starting a second one. Prints credentials.yaml/cloudinfo.yaml snippets at the end - feed those to register_csp_definition() and register_csp_credential()."
    },
    "devstack_registration_info": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/2.getRegistrationInfo.sh -o /tmp/getRegistrationInfo.sh && bash /tmp/getRegistrationInfo.sh --csp-name {{csp_name}}"
        ],
        "description": "Re-print the registration snippets (Keystone endpoint, ProjectID, region/zone) for an already-installed DevStack."
    },
    "devstack_update_endpoints": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/3.updateEndpoints.sh -o /tmp/updateEndpoints.sh && bash /tmp/updateEndpoints.sh --csp-name {{csp_name}}"
        ],
        "description": "Repoint the OpenStack service catalog after the host's public IP changed (stop/start, or no elastic IP). Re-register the credential afterwards."
    },
    "devstack_clean": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/openstack/4.cleanDevStack.sh -o /tmp/cleanDevStack.sh && bash /tmp/cleanDevStack.sh"
        ],
        "description": "Tear down a failed or stale DevStack so it can be installed cleanly again."
    },
    "k8s_control_plane": {
        "commands": [
            "CNI=$(echo \"{{k8s_cni}}\" | tr 'A-Z' 'a-z' | xargs); [ -z \"$CNI\" ] && CNI=flannel; curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/k8s/k8s-control-plane-setup.sh | bash -s -- --cni \"$CNI\""
        ],
        "description": "Install a Kubernetes control plane. K8S_CNI selects the CNI: empty or 'flannel' (default), or 'cilium'. Needs working outbound access - verify the security group has an outbound rule first, since a stateful firewall keeps SSH alive while blocking every package download. Before joining workers, also open the cluster's inbound ports - see k8s_worker_join for the list."
    },
    "k8s_get_join_command": {
        "commands": [
            "echo '[K8S_JOIN_COMMAND]'; sudo kubeadm token create --print-join-command"
        ],
        "description": "Print the kubeadm join command, to be handed to workers. Run on the CONTROL PLANE. Tokens expire (24h by default), so fetch this immediately before joining workers rather than reusing an old one. Use summarize_output=false so the command is not truncated."
    },
    "k8s_worker_join": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/k8s/k8s-worker-setup.sh | bash -s -- -j \"{{join_command}}\""
        ],
        "description": "Install the kubelet stack and join this node to an existing cluster. Run on the WORKER nodes (target them with node_id or label_selector - never the control plane, which is already a member). join_command comes from k8s_get_join_command. OPEN THE CLUSTER PORTS FIRST - a CB-Tumblebug security group allows only SSH, so the join times out at 'couldn't validate the identity of the API Server'. On the control plane's group allow inbound TCP 6443,2379-2380,10250,10257,10259,4240 and UDP 8472; on the workers' group TCP 10250,4240,30000-32767 and UDP 8472; source is the node subnet (get_infra shows the private addresses). 4240 and 8472 are Cilium's health and VXLAN ports - without them nodes join and then stay NotReady."
    },
    "k8s_storage_default": {
        "commands": [
            "kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.31/deploy/local-path-storage.yaml",
            "kubectl patch storageclass local-path -p '{\"metadata\":{\"annotations\":{\"storageclass.kubernetes.io/is-default-class\":\"true\"}}}'",
            "kubectl get storageclass"
        ],
        "description": "Install local-path as the default StorageClass. Required in EVERY topology before deploying anything with a PVC (Open WebUI has one): nothing else installs a StorageClass unless the full KServe stack is deployed. Use this for MULTI-NODE clusters; for a single-node cluster use k8s_single_node_prep, which also removes the control-plane taint."
    },
    "k8s_single_node_prep": {
        "commands": [
            "kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule- || true",
            "kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.31/deploy/local-path-storage.yaml",
            "kubectl patch storageclass local-path -p '{\"metadata\":{\"annotations\":{\"storageclass.kubernetes.io/is-default-class\":\"true\"}}}'",
            "kubectl get nodes -o wide",
            "kubectl get storageclass"
        ],
        "description": "Prepare a SINGLE-NODE cluster for workloads: remove the control-plane taint and install local-path as the default StorageClass. Without both, pods stay Pending forever with no error explaining why. Use this ONLY when the cluster has no workers - with workers joined, keep the taint (so the control plane stays reserved) and run k8s_storage_default instead."
    },
    "gpu_driver_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/installGpuDriver.sh -o /tmp/installGpuDriver.sh && bash /tmp/installGpuDriver.sh --no-reboot"
        ],
        "description": "Install the NVIDIA driver on a GPU node. --no-reboot is used deliberately: the installer reboots by default, which drops the SSH channel mid-command and looks like a failure. Reboot separately afterwards with gpu_node_reboot, then confirm with gpu_driver_check.",
        "timeout_minutes": 30
    },
    "gpu_node_reboot": {
        "commands": ["sudo systemctl reboot || sudo reboot"],
        "description": "Reboot a node so a freshly installed GPU driver loads. The command drops the connection by design; wait about a minute, then run gpu_driver_check.",
        "timeout_minutes": 5
    },
    "gpu_driver_check": {
        "commands": ["nvidia-smi || rocm-smi || echo 'no accelerator driver responded'"],
        "description": "Confirm the accelerator driver is loaded and the card is visible. Run this before installing vLLM - vLLM on a node without a working driver fails much later and less clearly.",
        "timeout_minutes": 5
    },
    "vllm_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployvLLM.sh -o /tmp/deployvLLM.sh && bash /tmp/deployvLLM.sh"
        ],
        "description": "Install vLLM on a GPU node, directly on the VM - no Kubernetes involved. Requires a working NVIDIA driver (gpu_driver_check). This is the simple path; the KServe route (k8s_kserve_stack) is only worth it when the cluster is wanted for other reasons.",
        "timeout_minutes": 40
    },
    "vllm_serve": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/servevLLM.sh -o /tmp/servevLLM.sh && bash /tmp/servevLLM.sh --model '{{model_id}}' --port 8000",
            "curl -s -m 20 http://127.0.0.1:8000/v1/models | head -c 400"
        ],
        "description": "Serve a Hugging Face model with vLLM on port 8000, OpenAI-compatible. model_id is the HF repo, e.g. Qwen/Qwen2.5-1.5B-Instruct, which fits a 16 GB T4 in half precision. The model is downloaded on first run, so allow time. Verifies /v1/models afterwards.",
        "timeout_minutes": 60
    },
    "ollama_install": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh"
        ],
        "description": "Install Ollama on a GPU node and serve it on port 3000 (not the 11434 default). Run gpu_driver_install first, or it falls back to CPU. Pull a model afterwards with ollama_pull. Open port 3000 to reach the API from outside."
    },
    "ollama_pull": {
        "commands": [
            "export OLLAMA_HOST=127.0.0.1:3000; ollama pull {{model}} && ollama list"
        ],
        "description": "Download a model into an Ollama installed by ollama_install. OLLAMA_HOST is set because that install moves the server to port 3000 while the CLI still defaults to 11434 - without it every ollama command fails with 'could not connect to ollama server'. Size the model to the card: a 14B at Q4 is ~9 GB, a 32B ~20 GB. Set model, e.g. qwen2.5:14b."
    },
    "open_webui_ollama": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh -o /tmp/deployOpenWebUI.sh && bash /tmp/deployOpenWebUI.sh ollama {{ollama_url}}",
            "sleep 20; curl -s -o /dev/null -w 'local check: %{http_code}\\n' http://127.0.0.1/"
        ],
        "description": "Run Open WebUI in Docker on port 80 against an Ollama backend. ollama_url is reached from inside the container, so for an Ollama on the same node use the Docker bridge address http://172.17.0.1:3000 - localhost there is the container itself. Open port 80 in the security group."
    },
    "open_webui_vllm": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOpenWebUI.sh -o /tmp/deployOpenWebUI.sh && bash /tmp/deployOpenWebUI.sh vllm '{{vllm_url}}'",
            "sleep 10; curl -s -o /dev/null -w 'local check: %{http_code}\n' http://127.0.0.1/"
        ],
        "description": "Run Open WebUI in Docker on port 80, pointed at a vLLM endpoint. vllm_url is the OpenAI-compatible base, e.g. http://<gpu-node-ip>:8000/v1 - use the node's own address when the UI and vLLM share a node. Open port 80 in the security group to reach it.",
        "timeout_minutes": 20
    },
    "k8s_kserve_stack": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/deploy-kserve-stack.sh -o /tmp/deploy-kserve-stack.sh && bash /tmp/deploy-kserve-stack.sh"
        ],
        "description": "Install the model-serving stack on the control plane: a default StorageClass, Helm, the NVIDIA GPU Operator, cert-manager and KServe in RawDeployment mode. Takes 10-20 minutes. Required before serve_vllm_model. Skip it only if the cluster already has KServe.",
        "timeout_minutes": 30
    },
    "serve_vllm_model": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/serve-vllm-model.sh -o /tmp/serve-vllm-model.sh && bash /tmp/serve-vllm-model.sh --model '{{model_id}}' --name '{{isvc_name}}'",
            "kubectl get isvc"
        ],
        "description": "Serve a Hugging Face model with vLLM through KServe. model_id is the HF repo, e.g. Qwen/Qwen2.5-1.5B-Instruct for a small one that fits a single T4. isvc_name is the InferenceService name Open WebUI will connect to. Needs an NVIDIA GPU node and k8s_kserve_stack already installed; AMD accelerators will not work, the stack is CUDA-based.",
        "timeout_minutes": 60
    },
    "k8s_open_webui": {
        "commands": [
            "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/kserve/deploy-open-webui-kserve.sh -o /tmp/deploy-open-webui.sh",
            "BACKEND='{{backend_url}}'; case \"$BACKEND\" in http*) ;; *) BACKEND='' ;; esac; if [ -n \"$BACKEND\" ]; then echo \"Connecting Open WebUI to: $BACKEND\"; bash /tmp/deploy-open-webui.sh --nodeport {{node_port}} --backend-url \"$BACKEND\"; else echo 'No backend URL given - the UI will come up with an empty model list.'; bash /tmp/deploy-open-webui.sh --nodeport {{node_port}}; fi",
            "kubectl get svc open-webui -o wide",
            "code=$(curl -s -m 8 -o /dev/null -w '%{http_code}' http://127.0.0.1:{{node_port}}/); if [ \"$code\" != 200 ]; then echo \"NodePort did not answer ($code); restarting kube-proxy\"; kubectl -n kube-system rollout restart ds/kube-proxy > /dev/null; kubectl -n kube-system rollout status ds/kube-proxy --timeout=180s > /dev/null; sleep 5; code=$(curl -s -m 8 -o /dev/null -w '%{http_code}' http://127.0.0.1:{{node_port}}/); fi; echo \"local NodePort check: $code\""
        ],
        "description": "Deploy Open WebUI as a NodePort service and confirm it answers locally. node_port is typically 30080. Set backend_url to an OpenAI-compatible endpoint (or several joined by ';') - get it from discover_service_endpoints() first, otherwise the UI deploys with an empty model list and chat does not answer. Requires a default StorageClass: the data PVC is a hard precondition, and nothing installs one unless the KServe stack was deployed, so run k8s_single_node_prep first. KServe itself is NOT required - the script falls back cleanly when no InferenceService exists."
    },
    "publish_nodeport_via_host": {
        "commands": [
            "set -e; IFACE=$(ip route show 0.0.0.0/0 | grep -oE 'dev [^ ]+' | cut -c5- | head -1); echo \"forwarding via $IFACE\"; sudo sysctl -w net.ipv4.ip_forward=1; sudo iptables -t nat -C PREROUTING -i \"$IFACE\" -p tcp --dport {{ext_port}} -j DNAT --to-destination {{target_ip}}:{{target_port}} 2>/dev/null || sudo iptables -t nat -A PREROUTING -i \"$IFACE\" -p tcp --dport {{ext_port}} -j DNAT --to-destination {{target_ip}}:{{target_port}}; sudo iptables -C FORWARD -p tcp -d {{target_ip}} --dport {{target_port}} -j ACCEPT 2>/dev/null || sudo iptables -A FORWARD -p tcp -d {{target_ip}} --dport {{target_port}} -j ACCEPT; sudo iptables -t nat -C POSTROUTING -d {{target_ip}} -p tcp --dport {{target_port}} -j MASQUERADE 2>/dev/null || sudo iptables -t nat -A POSTROUTING -d {{target_ip}} -p tcp --dport {{target_port}} -j MASQUERADE; echo '--- active rules ---'; sudo iptables -t nat -L PREROUTING -n -v --line-numbers | grep {{ext_port}}"
        ],
        "description": "Publish a service that lives behind a private address. RUN THIS ON THE HOST that owns the public IP and can route to target_ip - never on the target itself, where DNAT to its own unreachable address becomes an infinite loop. Kept as ONE command on purpose: CB-Tumblebug runs each entry of a command list in a SEPARATE SSH session, so a shell variable set in one entry is empty in the next. The MASQUERADE is required: with DNAT alone the target replies straight to the client and that reply never passes back through this host's conntrack. Also open ext_port in the host's security group AND target_port in the target's security group. Rules are added idempotently, so re-running is safe."
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
        "usage_note": "Run one with execute_command_infra(script_name=...)",
        "template_variables_supported": [
            "{{public_ip}}", "{{public_ips_space}}", "{{public_ips_comma}}", 
            "{{private_ips_space}}", "{{infra_id}}", "{{ns_id}}"
        ]
    }






# Tool: Transfer file to VMs in Infra
@tool(mutating=True)
def transfer_file_infra(
    ns_id: str,
    infra_id: str,
    file_path: str,
    target_path: str,
    nodegroup_id: Optional[str] = None,
    node_id: Optional[str] = None
) -> Dict:
    """
    Transfer a file to nodes of an Infra via SCP.

    The file is read from the MCP SERVER's own filesystem, not from yours - an LLM client
    has no way to put a file there, so this is usable only for paths that already exist in
    the server container. To place content you are holding, write it with a heredoc through
    execute_command_infra; for anything binary or large, have the node fetch it from a URL.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        file_path: Path on the MCP server's filesystem - see the note above
        target_path: Destination path on the remote node
        nodegroup_id: NodeGroup ID to limit transfer to nodes in a nodegroup (optional)
        node_id: Node ID to transfer to a single node (optional)

    Returns:
        File transfer result
    """
    url = f"/ns/{ns_id}/transferFile/infra/{infra_id}"
    params = {}

    if nodegroup_id:
        params["nodeGroupId"] = nodegroup_id
    if node_id:
        params["nodeId"] = node_id

    # Open file
    try:
        with open(file_path, 'rb') as file:
            files = {'file': (os.path.basename(file_path), file)}
            data = {'path': target_path}

            # Request with multipart form data
            return api_request("POST", url, files=files, json_data=data, params=params or None)
    except Exception as e:
        return {"error": f"File transfer error: {str(e)}"}

# Tool: Get request by ID
@tool()
def get_request_by_id(request_id: str) -> Dict:
    """
    Get request by request ID

    Args:
        request_id: Request ID

    Returns:
        Request information
    """
    return api_request("GET", f"/request/{request_id}")


# Tool: List node command status
@tool()
def list_node_command_status(
    ns_id: str,
    infra_id: str,
    node_id: str,
    status: Optional[List[str]] = None,
    request_id: Optional[str] = None,
    page: Optional[int] = None,
    size: Optional[int] = None,
    detail: str = "summary",
    tail_lines: int = 20
) -> Dict:
    """
    List remote command execution status records for a specific node.

    Supports filtering by status, requestId, and pagination.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Node ID (e.g., g1-1)
        status: Filter by status values (e.g., ["Completed", "Failed"])
        request_id: Filter by x-request-id of the command execution
        page: Page number for pagination (converted to offset)
        size: Page size for pagination (max records to return)
        detail: "summary" (default) keeps output only for the newest record and any that
            did not finish; "full" returns every record's output.
        tail_lines: Lines of stdout/stderr kept per record (default 20)

    Returns:
        List of command status records for the node
    """
    params = {}
    if status:
        params["status"] = status
    if request_id:
        params["xRequestId"] = request_id
    if size is not None:
        params["limit"] = size
    if page is not None:
        params["offset"] = (max(page, 1) - 1) * (size or 50)
    result = api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}/commandStatus", params=params)
    if _validate_detail(detail) == "full" or not isinstance(result, dict):
        return _redact_secrets(result)
    cmds = result.get("commands")
    if not isinstance(cmds, list):
        return _redact_secrets(result)
    # Output is kept only where it is actually read: the newest record, and anything that
    # did not complete. Older successful commands keep their metadata and resultSummary.
    keep_output = set()
    for i, c in enumerate(cmds):
        if not isinstance(c, dict):
            continue
        if i >= len(cmds) - 1 or c.get("status") not in ("Completed", None):
            keep_output.add(i)
    out = dict(result)
    out["commands"] = [
        _trim_command_record(c, tail_lines) if i in keep_output else _drop_command_output(c)
        for i, c in enumerate(cmds)
    ]
    out["_note"] = (f"output kept for the newest record and any that did not complete, "
                    f"trimmed to {tail_lines} lines; detail='full' returns everything")
    return _redact_secrets(out)


# Tool: Get specific node command status by index
@tool()
def get_node_command_status(
    ns_id: str,
    infra_id: str,
    node_id: str,
    index: int
) -> Dict:
    """
    Get a specific remote command execution status record by index for a node.

    Args:
        ns_id: Namespace ID
        infra_id: Infra ID
        node_id: Node ID (e.g., g1-1)
        index: Command status record index

    Returns:
        Command status record details
    """
    return api_request("GET", f"/ns/{ns_id}/infra/{infra_id}/node/{node_id}/commandStatus/{index}")


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






# Tool: Check available region zones for a spec
@tool()
def check_available_region_zones_for_spec(
    provider: str,
    spec_name: str
) -> Dict:
    """
    Check available region zones for a specific VM specification.

    Only Alibaba Cloud is implemented server-side today; any other provider comes back
    "not supported yet". Do not reach for this to validate an AWS or Azure spec.

    Use this to verify which regions/zones can actually provision a given spec
    before attempting infra creation.

    Args:
        provider: Cloud provider name (e.g., "aws", "gcp", "azure")
        spec_name: CSP spec name to check (e.g., "t3.medium")

    Returns:
        Available region zones for the spec
    """
    data = {
        "provider": provider,
        "cspSpecName": spec_name
    }
    return api_request("POST", "/availableRegionZonesForSpec", json_data=data)


# Tool: Check available region zones for a list of specs
@tool()
def check_available_region_zones_for_spec_list(
    provider: str,
    spec_names: List[str]
) -> Dict:
    """
    Check available region zones for multiple VM specifications of one provider at once.

    Only Alibaba Cloud is implemented server-side today; any other provider comes back
    "not supported yet". Do not reach for this to validate an AWS or Azure spec.

    Args:
        provider: Cloud provider name (e.g., "aws")
        spec_names: List of CSP spec names (e.g., ["t3.medium", "m5.large"])

    Returns:
        Available region zones for each spec in the list
    """
    data = {"provider": provider, "cspSpecNames": spec_names}
    return api_request("POST", "/availableRegionZonesForSpecList", json_data=data)


# Tool: Update existing spec list by available region zones
@tool(mutating=True)
def update_existing_spec_list_by_available_region_zones(
    ns_id: str,
    provider: str
) -> Dict:
    """
    Update the availability status of existing specs in the namespace by checking

    Only Alibaba Cloud is implemented server-side today; any other provider comes back
    "not supported yet". Do not reach for this to validate an AWS or Azure spec.
    actual available region zones from the CSP.

    This refreshes which specs can be provisioned in which zones (and removes
    stale unavailable specs), helping avoid failures caused by outdated data.

    Args:
        ns_id: Namespace ID
        provider: Cloud provider name to refresh (e.g., "aws")

    Returns:
        Cleanup result (specsDeleted, specsToDelete, availabilityResults, ...)
    """
    return api_request("POST", f"/ns/{ns_id}/updateExistingSpecListByAvailableRegionZones", json_data={"provider": provider})


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

# Prompt: Infra management prompt
@mcp.prompt()
def infra_management_prompt() -> str:
    """Prompt for Infra management with comprehensive create_infra_dynamic usage patterns"""
    return """
    You are a Multi-Cloud Infrastructure (Infra) management expert for Cloud-Barista CB-Tumblebug.
    
    🚨 **CRITICAL: ONLY USE create_infra_dynamic FOR ALL Infra CREATION**
    
    All Infra creation MUST use `create_infra_dynamic` with proper workflow patterns. 
    Other Infra creation tools have been deprecated.
    
    **🔥 MANDATORY WORKFLOW FOR create_infra_dynamic:**
    
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
    for spec in specs["summarized_specs"][:2]:  # Use multiple specs for multi-CSP
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
            "nodeGroupSize": 1
        })
    
    # Step 6: Create Infra with properly mapped configurations
    infra = create_infra_dynamic(
        ns_id="my-project",
        name="multi-csp-infra",
        node_groups=vm_configs
    )
    ```
    
    **PATTERN 2: AUTO-MAPPING WORKFLOW (SIMPLER)**
    ```python
    # Let create_infra_dynamic auto-map images for specs
    specs = recommend_vm_spec(
        filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}}
    )
    
    vm_configs = []
    for spec in specs["summarized_specs"][:2]:
        vm_configs.append({
            "specId": spec["id"],  # REQUIRED: Exact spec ID
            "name": f"vm-{spec['providerName']}-{len(vm_configs)+1}",
            "os_requirements": {"os_type": "ubuntu", "use_case": "web-server"}
            # imageId omitted - will be auto-mapped to compatible image
        })
    
    infra = create_infra_dynamic(
        ns_id="my-project",
        name="auto-mapped-infra",
        node_groups=vm_configs  # Auto-mapping ensures compatibility
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
    
    # Step 3: Create Infra with location-optimized specs
    infra = create_infra_dynamic(
        ns_id="production",
        name="silicon-valley-infra",
        node_groups=[
            {"specId": spec["id"], "name": f"vm-{spec['regionName']}-{i+1}"}
            for i, spec in enumerate(specs["summarized_specs"][:3])
        ]
    )
    ```
    
    **🔍 PATTERN 4: RISK-AWARE WORKFLOW (NEW - ENHANCED)**
    ```python
    # CRITICAL: Always analyze historical risk before Infra creation
    
    # Step 1: Prepare initial VM configurations
    vm_configs = [
        {"specId": "aws+us-east-1+t2.small", "name": "web-server"},
        {"specId": "azure+eastus+Standard_B2s", "name": "api-server"}
    ]
    
    # Step 2: Review configurations with risk analysis
    review_result = review_infra_dynamic_request(
        ns_id="production",
        name="web-application",
        node_groups=vm_configs
    )
    
    # Step 3: Check for high-risk VMs and get mitigation guidance
    if review_result.get("overall_risk_assessment", {}).get("high_risk_vms"):
        guidance = get_provisioning_risk(vm_configs)
        
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
            if safer_specs["summarized_specs"]:
                vm_configs[vm_index]["specId"] = safer_specs["summarized_specs"][0]["id"]
        
        # Step 5: Re-review with updated configurations
        final_review = review_infra_dynamic_request(
            ns_id="production",
            name="web-application",
            node_groups=vm_configs
        )
    
    # Step 6: Create Infra only after acceptable risk level
    if final_review.get("creationViable", False):
        infra = create_infra_dynamic(
            ns_id="production",
            name="web-application",
            node_groups=vm_configs
        )
    ```
    
    **🔧 PATTERN 5: PROGRESSIVE RISK MITIGATION**
    ```python
    # For high-risk scenarios, deploy in stages
    
    # Step 1: Separate VMs by risk level
    guidance = get_provisioning_risk(original_vm_configs)
    low_risk_configs = []
    high_risk_configs = []
    
    for i, vm_guidance in enumerate(guidance["vm_specific_guidance"]):
        if vm_guidance["risk_level"] in ["low", "unknown"]:
            low_risk_configs.append(original_vm_configs[i])
        else:
            high_risk_configs.append(original_vm_configs[i])
    
    # Step 2: Deploy low-risk VMs first
    if low_risk_configs:
        stable_infra = create_infra_dynamic(
            ns_id="production",
            name="stable-infrastructure",
            node_groups=low_risk_configs
        )
    
    # Step 3: Research and deploy high-risk VMs with alternatives
    for high_risk_config in high_risk_configs:
        risk_analysis = get_provisioning_risk(
            high_risk_config["specId"], kind="summary"
        )
        
        if risk_analysis.get("riskLevel") == "high":
            # Get detailed analysis and alternatives
            detailed_risk = get_provisioning_risk(
                high_risk_config["specId"], kind="detailed"
            )
            # Apply recommendations from detailed analysis
        
        # Add to Infra after risk mitigation
        enhanced_vm = add_nodegroup_dynamic(
            ns_id="production",
            infra_id="stable-infrastructure",
            req=modified_high_risk_config
        )
    ```
    
    # Step 3: Use specs with auto-mapping or manual image selection
    vm_configs = []
    for spec in specs["summarized_specs"][:2]:
        vm_configs.append({
            "specId": spec["id"],
            "name": f"vm-{spec['regionName']}-{len(vm_configs)+1}",
            "description": f"VM near Silicon Valley in {spec['regionName']}"
        })
    
    infra = create_infra_dynamic(
        ns_id="location-project",
        name="silicon-valley-infra",
        node_groups=vm_configs
    )
    ```
    
    **PATTERN 4: NAMESPACE MANAGEMENT WORKFLOW**
    ```python
    # Check namespace first
    ns_check = manage_namespace(action='create', name="my-project")
    
    # Create namespace if needed
    if not ns_check["namespace_exists"]:
        manage_namespace(action='create'"my-project", "Project namespace")
    
    # Then create Infra
    infra = create_infra_dynamic(
        ns_id="my-project",
        name="managed-infra",
        node_groups=vm_configs
    )
    ```
    
    **PATTERN 5: CONFIRMATION WORKFLOW**
    ```python
    # Preview configuration first
    preview = create_infra_dynamic(
        ns_id="my-project",
        name="preview-infra",
        node_groups=vm_configs,
        skip_confirmation=False  # Returns preview only
    )
    
    # User reviews preview, then confirms
    infra = create_infra_dynamic(
        ns_id="my-project",
        name="confirmed-infra",
        node_groups=vm_configs,
        force_create=True  # Actually creates after confirmation
    )
    ```
    
    **PATTERN 6: COMPREHENSIVE REVIEW WORKFLOW (MANDATORY)**
    ```python
    # Step 1: Pre-creation validation
    validation = review_infra_dynamic_request(
        ns_id="my-project",
        name="web-application",
        node_groups=vm_configs
    )
    
    # Step 2: CRITICAL - Comprehensive review result analysis
    # YOU MUST analyze ALL aspects of the review result before proceeding
    
    # 2A. Overall Validation Status
    creation_viable = validation.get("creationViable", False)
    validation_passed = validation.get("validation_passed", False)
    
    # 2B. VM-Level Analysis
    vm_reviews = validation.get("nodeReviews", [])
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
    print("\\n🚀 Proceeding with Infra creation...")
    infra = create_infra_dynamic(
        ns_id="my-project",
        name="web-application",
        node_groups=vm_configs
    )
    
    # Step 6: Post-creation status monitoring
    print("\\n📊 Monitoring deployment progress...")
    status = check_infra_status_and_handle_failures(
        ns_id="my-project",
        infra_id=infra["id"],
        detailed_analysis=True
    )
    ```
    
    **🔍 MANDATORY REVIEW RESULT ANALYSIS CHECKLIST:**
    
    Before proceeding with any Infra creation, you MUST analyze and report:
    
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
    
    **❌ NEVER proceed with create_infra_dynamic() until:**
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
    - If omitted: Auto-mapped by create_infra_dynamic
    - Provider-specific formats:
      * AWS: "ami-0123456789abcdef0"
      * Azure: "/subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/images/ubuntu-20.04"
      * GCP: "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20240307a"
    
    **🔄 COMPLETE CREATE_Infra_DYNAMIC WORKFLOW:**
    
    **1. PREPARATION PHASE:**
    ```python
    # A. Check/create namespace
    ns_result = manage_namespace(action='create', name=preferred_ns_id)
    
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
    for spec in specs["summarized_specs"]:
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
            "nodeGroupSize": 1
        }
        vm_configs.append(vm_config)
    ```
    
    **3. CREATION PHASE:**
    ```python
    # A. Validate configuration (optional but recommended)
    validation = review_infra_dynamic_request(ns_id, name, vm_configs)
    
    # B. Create Infra
    infra = create_infra_dynamic(
        ns_id=target_namespace,
        name=infra_name,
        node_groups=vm_configs,
        description="Multi-CSP infrastructure",
        hold=False,  # Set True to hold for review
        skip_confirmation=False,  # Set True for automated workflows
        force_create=False  # Set True after user confirmation
    )
    ```
    
    **4. POST-CREATION PHASE (MANDATORY):**
    ```python
    # A. Check deployment status
    status = check_infra_status_and_handle_failures(
        ns_id=target_namespace,
        infra_id=infra["id"],
        auto_cleanup_failed=False
    )
    
    # B. Handle different outcomes
    if status["deployment_health"] == "healthy":
        print("✅ All VMs deployed successfully!")
    elif status["deployment_health"] == "partial-failed":
        # Offer cleanup of failed VMs
        recovery = interactive_infra_recovery(
            ns_id, infra["id"], 
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
    2. ✅ ALWAYS check Infra status after creation
    3. ✅ ALWAYS handle failures with user confirmation
    4. ✅ ALWAYS use location priority for geographic requests
    5. ❌ NEVER create spec IDs manually
    6. ❌ NEVER skip failure recovery workflows
    7. ❌ NEVER use cross-CSP image references
    
    Current namespace list: {{namespace://list}}
    
    What Infra would you like to create? I'll guide you through the proper create_infra_dynamic workflow with spec validation and failure handling.
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
    history = resume_session(operation_type="infra_creation")
    
    # 3. Deploy with both context knowledge and current research
    deploy_application(
        application_id="app",
        research_requirements=True,  # ALWAYS True for new deployments
        deployment_type="production"  # informed by context + current needs
    )
    ```
    
    **CONTEXT AWARENESS CAPABILITIES:**
    
    **Memory Functions Available:**
    - resume_session(): See recent work and operations
    - resume_session(): Get comprehensive analysis of current session
    - resume_session(): Find specific previous work
    - store_interaction_memory(): Record new interactions (automatic in most tools)
    
    **GETTING STARTED WITH CONTEXT:**
    
    1. **Check Recent Work:**
       ```python
       # See what's been done recently
       history = resume_session(days_back=7, max_results=10)
       
       # Focus on specific operations
       infra_history = resume_session(operation_type="infra_creation")
       namespace_history = resume_session(operation_type="namespace_management")
       ```
    
    2. **Understand Current Session:**
       ```python
       # Get comprehensive session analysis
       summary = resume_session()
       ```
    
    3. **Search for Specific Context:**
       ```python
       # Find previous work with specific resources
       aws_work = resume_session("aws")
       ubuntu_setups = resume_session("ubuntu")
       ```
    
    **AUTOMATIC MEMORY STORAGE:**
    Most operations automatically store interaction data:
    - Infra creation/management → "infra_creation" type
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
    recent_work = resume_session(days_back=3)
    
    # 2. If user was working on Infra creation, check details
    if any("infra_creation" in item.get("operation_type", "") for item in recent_work.get("interactions", [])):
        infra_context = resume_session("infra")
        # Use context to suggest next steps or improvements
    
    # 3. Get session insights for optimization
    session_insights = resume_session()
    ```
    
    **PRIVACY & MEMORY MANAGEMENT:**
    - Interactions are stored locally within the session
    - Use clear_session() to clean up if needed
    - Memory helps provide personalized assistance based on previous work
    
    How can I help you today? I'll check our previous work context to provide the most relevant assistance.
    """

# Prompt: Workflow demo prompt
@mcp.prompt()
def workflow_demo_prompt() -> str:
    """Prompt for workflow demonstration with failure handling"""
    return """
    You are a Cloud-Barista CB-Tumblebug expert who helps demonstrate how to create and manage Multi-Cloud Infrastructure (Infra).
    
    You can guide through the following workflows:
    
    **CORE WORKFLOWS:**
    1. **Create a namespace** - Prepare workspace for infrastructure
    2. **View cloud connection information** - Check available cloud providers
    3. **Recommend VM specifications and create an Infra** - Deploy infrastructure
    4. **🔥 Check Infra status and handle deployment failures** - Monitor and recover
    5. **Execute remote commands** - Configure and manage infrastructure
    6. **Configure Network Load Balancers** - Set up traffic distribution
    7. **Clean up and delete resources** - Cost management and cleanup
    
    **🚨 ENHANCED FAILURE HANDLING DEMONSTRATIONS:**
    
    **Workflow 4A: Infra Status Monitoring & Failure Recovery**
    ```python
    # After Infra creation, always check status
    status = check_infra_status_and_handle_failures(
        ns_id="demo-namespace",
        infra_id="demo-infra",
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
        recovery = interactive_infra_recovery(
            ns_id="demo-namespace",
            infra_id="demo-infra",
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
    Demo: Create namespace → Find specs → Deploy Infra → Check status (success) → Execute commands
    ```
    
    **Pattern 2: Failure Recovery Path**
    ```
    User: "What happens if deployment fails?"
    Demo: Create Infra → Simulate partial failure → Show status analysis → Guide through recovery options → Execute refine → Verify success
    ```
    
    **Pattern 3: Decision Making Path**
    ```
    User: "How do I handle failed VMs?"
    Demo: Show failure analysis → Present options → Ask for user choice → Execute with confirmation → Monitor results
    ```
    
    **🛠️ AVAILABLE RECOVERY DEMONSTRATIONS:**
    
    **Recovery Actions:**
    - **refine**: Cleanup failed VMs, keep successful ones
    - **terminate**: Complete Infra deletion for fresh start
    - **reboot**: Restart VMs for temporary issues
    - **suspend/resume**: Cost management demonstrations
    
    **User Interaction Patterns:**
    - Status analysis and explanation
    - Risk assessment for each action
    - Confirmation prompts and user choice
    - Progress monitoring and verification
    - Next steps recommendations
    
    **📊 DEMONSTRATION TOOLS:**
    - check_infra_status_and_handle_failures(): Status analysis and recommendations
    - interactive_infra_recovery(): Guided recovery with user confirmation
    - Standard Infra tools: create, control, delete
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

# Prompt: Image search and Infra creation workflow guide
@mcp.prompt()
def image_infra_workflow_prompt() -> str:
    """Complete workflow guide for image search and Infra creation with smart namespace management"""
    return """
    You are an expert guide for the complete Namespace Management → Image Search → Infra Creation workflow in CB-Tumblebug.
    
    **SMART WORKFLOW (RECOMMENDED):**
    
    **Step 0: Smart Infra Creation with Auto-Mapping**
    Use create_infra_dynamic() with auto-mapping for foolproof spec-to-image compatibility:
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
    
    # Create Infra with automatic spec-to-image mapping
    result = create_infra_dynamic(
        ns_id="my-project",
        name="multi-csp-infrastructure", 
        node_groups=vm_configs  # Auto-mapping ensures correct images
    )
    ```
    
    **🔴 CRITICAL: Infra Dynamic Request Body Requirements**
    
    **FOR ALL Infra Creation (review_infra_dynamic_request + create_infra_dynamic):**
    
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
    spec = specs["summarized_specs"][0]  # Take first recommended spec
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
        "nodeGroupSize": 1
    }
    
    # Use in Infra creation
    create_infra_dynamic(
        ns_id="my-project",
        name="web-infrastructure",
        node_groups=[vm_config]
    )
    ```
    
    **Step 0 Alternative: Advanced Workflow with Validation**
    Use get_usage_guide('provision') for comprehensive multi-CSP deployments:
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
    
    review = review_infra_dynamic_request(
        ns_id="my-project",
        name="my-infrastructure",
        node_groups=node_groups
    )
    ```
    
    **MANUAL STEP-BY-STEP WORKFLOW:**
    
    **Step 0: Namespace Preparation**
    Check and prepare namespace first:
    ```python
    # Check what's available
    ns_check = manage_namespace(action='create', name="my-project")
    
    # Create if needed
    ns_result = manage_namespace(action='create'"my-project", "My project namespace")
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
    for spec in specs[:2]:  # Use different specs for multi-CSP Infra
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
    
    **Step 6: Create Infra with Properly Mapped Images**
    ```python
    # Validate configurations before deployment (optional but recommended)
    validation = validate_vm_spec_image_compatibility(vm_configs)
    
    # Create Infra with validated configurations
    infra = create_infra_dynamic(
        ns_id="my-project",
        name="multi-csp-infrastructure",
        node_groups=vm_configs  # Each VM has correct CSP-specific image
    )
    ```
    
    **Step 7: MANDATORY - Post-Deployment Status Check & Failure Handling**
    ```python
    # Always check Infra status after creation
    status_check = check_infra_status_and_handle_failures(
        ns_id="my-project",
        infra_id=infra["id"],
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
            recovery_result = interactive_infra_recovery(
                ns_id="my-project",
                infra_id=infra["id"],
                recovery_action="refine",
                confirm_cleanup=True
            )
            print("🔧 Cleanup completed - infrastructure optimized!")
        
    elif status_check["status_analysis"]["deployment_health"] == "critical":
        print("❌ CRITICAL: All VMs failed to deploy")
        print("🔧 RECOMMENDATION: Check errors and consider recreating with different specs")
        
        # Offer diagnostic and recreation options
        user_choice = input("Terminate failed Infra and recreate? (y/n): ")
        if user_choice.lower() == 'y':
            interactive_infra_recovery(
                ns_id="my-project",
                infra_id=infra["id"], 
                recovery_action="terminate",
                confirm_cleanup=True
            )
    ```
    
    **KEY RELATIONSHIPS:**
    - manage_namespace(action='create') → namespace guidance
    - check_resource_exists() → namespace verification
    - recommend_vm_spec() → spec ID (determines CSP/region) → specId parameter
    - search_images() → cspImageName (in spec's CSP/region) → imageId parameter
    - create_infra_dynamic() → Infra creation → check_infra_status_and_handle_failures() → failure recovery
    - **CRITICAL**: Each VM spec requires its own image search in the spec's specific CSP/region
    - **AUTOMATIC**: create_infra_dynamic() handles spec-to-image mapping automatically
    - **VALIDATION**: validate_vm_spec_image_compatibility() checks configurations
    - **FAILURE HANDLING**: check_infra_status_and_handle_failures() monitors deployment success
    - **RECOVERY**: interactive_infra_recovery() guides through failure resolution
    
    **NAMESPACE MANAGEMENT BENEFITS:**
    - Automatic namespace validation before Infra creation
    - Smart recommendations for namespace selection/creation
    - Prevention of Infra creation failures due to invalid namespaces
    - Unified workflow with clear error messages and suggestions
    
    **DEPLOYMENT FAILURE HANDLING BENEFITS:**
    - Automatic detection of Partial-Failed and Failed states
    - User-guided recovery with clear impact assessment
    - Preservation of successful VMs while cleaning up failures
    - Cost optimization by removing failed infrastructure
    - Comprehensive status monitoring and progress tracking
    
    **IMPORTANT NOTES:**
    - Always ensure namespace exists before Infra creation
    - **MANDATORY**: Check Infra status after creation for failures
    - **RECOMMENDED**: Use create_infra_dynamic() auto-mapping for foolproof compatibility
    - **CRITICAL**: Each VM spec requires its own CSP-specific image (no cross-CSP sharing)
    - **USER CONFIRMATION**: Always ask before cleanup actions (unless auto_cleanup=True)
    - **RECOVERY PRIORITY**: For Partial-Failed, recommend 'refine' to keep successful VMs
    - The cspImageName is provider-specific (AMI ID for AWS, Image ID for Azure, etc.)
    - specId format: {provider}+{region}+{spec_name}
    - **VALIDATION**: Use validate_vm_spec_image_compatibility() before deployment
    - **EXAMPLES**: Use get_usage_guide('provision') to see the correct spec/image pairing
    - **MONITORING**: Use check_infra_status_and_handle_failures() after deployment
    - Test with hold=True first to review configuration
    
    Current namespaces: {{namespace://list}}
    
    What would you like to help you create today?
    """

logger.info("=" * 60)
logger.info("🚀 CB-Tumblebug MCP Server initialization complete!")
logger.info("=" * 60)
logger.debug("Available memory functions: store_interaction_memory, get_interaction_history, get_session_summary, search_interaction_memory, clear_interaction_memory")
logger.debug("Automatic memory storage enabled for: Infra creation, command execution, namespace management")

#####################################
# Infra Status Monitoring & Recovery Tools
#####################################

# Tool: Check Infra status and handle failures
@tool()
def check_infra_status_and_handle_failures(
    ns_id: str,
    infra_id: str,
    auto_cleanup_failed: bool = False,
    detailed_analysis: bool = True
) -> Dict:
    """
    Check Infra status and provide recovery options for failed/partial-failed states.
    
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
        infra_id: Infra ID to check
        auto_cleanup_failed: If True, automatically cleanup failed VMs without asking
        detailed_analysis: Include detailed VM-level analysis
    
    Returns:
        Status report with recovery recommendations and action options
    """
    try:
        # Get detailed Infra status
        infra_status = get_infra_list(ns_id, option="status")
        
        # Find specific Infra in status list
        target_infra_status = None
        for infra in infra_status.get("infra", []):
            if infra.get("id") == infra_id:
                target_infra_status = infra
                break
        
        if not target_infra_status:
            return {
                "error": "Infra status not found",
                "infra_id": infra_id,
                "namespace": ns_id,
                "recommendation": "Check if Infra exists or is still being created"
            }
        
        overall_status = target_infra_status.get("status", "Unknown")
        # Status is composite, e.g. "Running:2 (R:2/2)" — extract the base keyword for comparisons
        base_status = overall_status.split(":")[0].strip().lower()
        vm_status_list = target_infra_status.get("node", [])
        
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
                    "csp_resource_id": vm.get("cspResourceId", "N/A")
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

        # Post-deployment (bootstrap) command outcome: nodes can be Running while
        # their bootstrap failed, so surface it alongside node health
        post_status = None
        try:
            infra_detail = get_infra(ns_id, infra_id)
            if isinstance(infra_detail, dict):
                post_status = infra_detail.get("postCommandStatus")
                if post_status and post_status != "None":
                    recovery_analysis["post_command_status"] = post_status
                    failed_phases = [
                        {
                            "phase": ph.get("phase"),
                            "target": ph.get("target"),
                            "status": ph.get("status"),
                            "failed_nodes": [
                                {"nodeId": r.get("nodeId"), "error": r.get("error")}
                                for r in ((ph.get("results") or {}).get("results") or [])
                                if r.get("error")
                            ],
                        }
                        for ph in (infra_detail.get("postCommandResults") or [])
                        if ph.get("status") not in ("Completed", None)
                    ]
                    if failed_phases:
                        recovery_analysis["post_command_failed_phases"] = failed_phases
        except Exception as e:
            logger.debug(f"Could not read postCommandStatus: {e}")

        # Generate recommendations based on status
        recommendations = []
        recovery_actions = []

        if post_status in ("Failed", "CompletedWithErrors"):
            recommendations.append(
                f"⚠️ BOOTSTRAP ISSUE: post-deployment commands reported '{post_status}' "
                f"(nodes may be Running but not configured as intended)"
            )
            recommendations.append(
                "💡 Inspect 'postCommandResults' for the failing phase/nodes, then re-run the "
                "commands with execute_command_infra() targeting those nodes"
            )
        
        if base_status == "partial-failed" or len(failed_vms) > 0:
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
                "command": f"control_infra('{ns_id}', '{infra_id}', 'refine')",
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
        
        elif base_status == "failed" or (len(failed_vms) > 0 and len(running_vms) == 0 and len(creating_vms) == 0):
            recommendations.append("❌ CRITICAL: All VMs in Infra have failed")
            recommendations.append("🔧 RECOMMENDED ACTIONS: Check error logs, recreate Infra, or terminate and retry")
            
            recovery_actions.extend([
                {
                    "action": "terminate",
                    "description": "Delete entire Infra and start fresh",
                    "command": f"terminate_infra('{ns_id}', '{infra_id}', confirm=True)",
                    "risk_level": "high",
                    "impact": "Complete Infra deletion - all data lost"
                },
                {
                    "action": "refine",
                    "description": "Attempt to cleanup and restart failed components",
                    "command": f"control_infra('{ns_id}', '{infra_id}', 'refine')",
                    "risk_level": "medium",
                    "impact": "Remove failed VMs, may need to recreate"
                }
            ])
        
        elif base_status == "running":
            recommendations.append("✅ SUCCESS: All VMs are running successfully")
            recommendations.append("📊 NEXT STEPS: Execute commands, configure applications, or set up monitoring")
            
        elif len(creating_vms) > 0:
            recommendations.append(f"⏳ IN PROGRESS: {len(creating_vms)} VMs still being created")
            recommendations.append("⌛ RECOMMENDED: Wait 2-5 minutes and check status again")
            
            recovery_actions.append({
                "action": "monitor",
                "description": "Continue monitoring deployment progress",
                "command": f"check_infra_status_and_handle_failures('{ns_id}', '{infra_id}')",
                "risk_level": "none",
                "impact": "Status monitoring only"
            })
        
        # Execute automatic cleanup if requested
        auto_cleanup_result = None
        if auto_cleanup_failed and len(failed_vms) > 0:
            logger.info(f"Auto-cleanup enabled: Refining Infra {infra_id} to remove {len(failed_vms)} failed VMs")
            auto_cleanup_result = control_infra(ns_id, infra_id, "refine")
            recommendations.append(f"🔧 AUTO-CLEANUP EXECUTED: Refined Infra to remove {len(failed_vms)} failed VMs")
        
        # Prepare detailed response
        response = {
            "infra_id": infra_id,
            "namespace": ns_id,
            "timestamp": datetime.now().isoformat(),
            "status_analysis": recovery_analysis,
            "detailed_status": {
                "overall_infra_status": overall_status,
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
            user_request=f"Check Infra status and handle failures for '{infra_id}' in namespace '{ns_id}'",
            llm_response=f"Infra status: {overall_status}, Failed VMs: {len(failed_vms)}, Running VMs: {len(running_vms)}",
            operation_type="infra_status_monitoring",
            context_data={
                "namespace_id": ns_id,
                "infra_id": infra_id,
                "overall_status": overall_status,
                "failed_vms_count": len(failed_vms),
                "auto_cleanup": auto_cleanup_failed
            },
            status="completed"
        )
        
        return response
        
    except Exception as e:
        logger.error(f"Error checking Infra status: {e}")
        return {
            "error": f"Failed to check Infra status: {str(e)}",
            "infra_id": infra_id,
            "namespace": ns_id,
            "recommendation": "Check if Infra exists and namespace is valid"
        }

# Tool: Interactive Infra failure recovery
@tool(mutating=True)
def interactive_infra_recovery(
    ns_id: str,
    infra_id: str,
    recovery_action: str = "refine",
    confirm_cleanup: bool = False
) -> Dict:
    """
    Interactive Infra recovery tool for handling failed deployments with user confirmation.
    
    **SUPPORTED RECOVERY ACTIONS:**
    - refine: Remove failed VMs while keeping successful ones (recommended for partial failures)
    - terminate: Delete entire Infra (use when all VMs failed)
    - reboot: Restart all VMs (use for temporary issues)
    - resume: Resume suspended VMs
    - suspend: Suspend all VMs (temporary cost saving)
    
    **INTERACTIVE WORKFLOW:**
    1. Analyze current Infra status
    2. Present failure details and impact assessment
    3. Require user confirmation for destructive actions
    4. Execute recovery action with progress monitoring
    5. Verify recovery success and provide next steps
    
    Args:
        ns_id: Namespace ID
        infra_id: Infra ID to recover
        recovery_action: Action to perform (refine, terminate, reboot, resume, suspend)
        confirm_cleanup: User confirmation for destructive actions
    
    Returns:
        Recovery execution result with status updates and next steps
    """
    try:
        # Step 1: Get current status before recovery
        pre_recovery_status = check_infra_status_and_handle_failures(ns_id, infra_id, auto_cleanup_failed=False)
        
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
            action_validation["warnings"].append(f"DESTRUCTIVE: Will delete entire Infra with {running_vms_count + failed_vms_count} VMs")
            if running_vms_count > 0:
                action_validation["warnings"].append(f"WARNING: {running_vms_count} running VMs will be lost")
        
        # Step 3: Check user confirmation for destructive actions
        if action_validation["confirmation_required"] and not confirm_cleanup:
            return {
                "infra_id": infra_id,
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

Infra: {infra_id} (Namespace: {ns_id})
Current Status: {overall_status}
Action: {recovery_action}

IMPACT ASSESSMENT:
- Failed VMs: {failed_vms_count} (will be removed/affected)
- Running VMs: {running_vms_count} ({'will be preserved' if recovery_action == 'refine' else 'will be affected'})

WARNINGS:
{chr(10).join(f"⚠️  {w}" for w in action_validation["warnings"])}

To proceed, call this function again with confirm_cleanup=True
To cancel, use check_infra_status_and_handle_failures() to explore other options
                """.strip(),
                "next_steps": [
                    f"interactive_infra_recovery('{ns_id}', '{infra_id}', '{recovery_action}', confirm_cleanup=True)",
                    f"check_infra_status_and_handle_failures('{ns_id}', '{infra_id}')"
                ]
            }
        
        # Step 4: Execute recovery action
        if not action_validation["appropriate"]:
            return {
                "error": "Recovery action not appropriate for current Infra status",
                "infra_id": infra_id,
                "warnings": action_validation["warnings"],
                "recommendation": "Use check_infra_status_and_handle_failures() to get appropriate recommendations"
            }
        
        logger.info(f"Executing {recovery_action} on Infra {infra_id} with user confirmation")
        
        # Execute the recovery action
        if recovery_action == "terminate":
            recovery_result = terminate_infra(ns_id, infra_id, confirm=True)
        else:
            recovery_result = control_infra(ns_id, infra_id, recovery_action)
        
        # Step 5: Post-recovery status check
        post_recovery_status = None
        if recovery_action != "terminate":
            # Wait a moment for action to take effect
            import time
            time.sleep(2)
            
            post_recovery_status = check_infra_status_and_handle_failures(ns_id, infra_id, auto_cleanup_failed=False)
        
        # Step 6: Prepare comprehensive response (reflect actual recovery-call outcome)
        recovery_failed = isinstance(recovery_result, dict) and "error" in recovery_result
        response = {
            "infra_id": infra_id,
            "namespace": ns_id,
            "recovery_action": recovery_action,
            "execution_status": "failed" if recovery_failed else "completed",
            "timestamp": datetime.now().isoformat(),
            "pre_recovery_analysis": pre_recovery_status["status_analysis"],
            "recovery_execution": recovery_result,
            "post_recovery_analysis": post_recovery_status["status_analysis"] if post_recovery_status else None,
            "recovery_success": not recovery_failed,
            "next_steps": []
        }
        if recovery_failed:
            response["next_steps"].append("Recovery API call failed — inspect recovery_execution for details")
            return response
        
        # Determine success and next steps
        if recovery_action == "terminate":
            response["next_steps"].extend([
                "Infra has been deleted successfully",
                "Create a new Infra with lessons learned from failure analysis",
                "Consider using create_infra_dynamic() with hold=True for testing"
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
            user_request=f"Execute recovery action '{recovery_action}' on Infra '{infra_id}'",
            llm_response=f"Recovery {recovery_action} executed: Success={response['recovery_success']}",
            operation_type="infra_recovery",
            context_data={
                "namespace_id": ns_id,
                "infra_id": infra_id,
                "recovery_action": recovery_action,
                "pre_failed_vms": failed_vms_count,
                "pre_running_vms": running_vms_count,
                "post_recovery_success": response["recovery_success"]
            },
            status="completed" if response["recovery_success"] else "partial_failure"
        )
        
        return response
        
    except Exception as e:
        logger.error(f"Error during Infra recovery: {e}")
        return {
            "error": f"Recovery failed: {str(e)}",
            "infra_id": infra_id,
            "namespace": ns_id,
            "recovery_action": recovery_action,
            "recommendation": "Check Infra status and try alternative recovery methods"
        }

#####################################
# Infra Configuration Preview & Validation Tools
#####################################

# Tool: Preview Infra configuration before creation
@tool()
def preview_infra_configuration(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "Infra to be created",
) -> Dict:
    """
    Preview Infra configuration before actual creation.
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
        ns_id: Namespace ID where Infra will be created
        name: Infra name
        node_groups: List of VM configurations to preview
        description: Infra description
    
    Returns:
        Comprehensive preview with configuration summary and recommendations
    """
    preview_result = {
        "infra_overview": {
            "name": name,
            "namespace_id": ns_id,
            "description": description,
            "total_vms": len(node_groups)
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
            "action": "use manage_namespace(action='create') or manage_namespace(action='create')"
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
    
    for i, vm_config in enumerate(node_groups):
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
                    nodegroup_size = int(vm_config.get("nodeGroupSize", 1))
                    vm_analysis["estimated_resources"] = {
                        "nodegroup_size": nodegroup_size,
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
        "total_vms": len(node_groups),
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
            "action": "Proceed with create_infra_dynamic() or review configuration one more time"
        })
    else:
        recommendations.append({
            "priority": "warning",
            "message": "❌ Configuration has issues - deployment may fail",
            "action": "Fix validation issues before attempting deployment"
        })
    
    preview_result["recommendations"] = recommendations
    
    return preview_result

# Helper: VM cost estimate from a spec id. Not a tool - the leading underscore says so,
# and a stray @tool() here published a private helper in every client's schema listing.
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
                            "diskSizeGB": spec_info.get("diskSizeGB", "unknown"),
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

def generate_infra_creation_summary(
    ns_id: str,
    name: str,
    node_groups: List[Dict],
    description: str = "Infra to be created",
    install_mon_agent: str = "no",
    hold: bool = False
) -> Dict:
    """
    Generate a comprehensive user-friendly summary of Infra creation plan for confirmation.
    This provides detailed overview including cost estimates, CSP distribution, and deployment strategy.
    
    **ENHANCED SUMMARY INCLUDES:**
    - Detailed Infra overview with cost breakdown
    - VM-by-VM specifications and pricing
    - CSP and region distribution analysis
    - Image mapping strategy validation
    - Monthly and hourly cost estimates
    - Deployment timeline and resource allocation
    - Risk assessment and recommendations
    - User confirmation workflow
    
    Args:
        ns_id: Namespace ID
        name: Infra name
        node_groups: VM configurations
        description: Infra description
        hold: Whether to hold for review
    
    Returns:
        Comprehensive summary with detailed cost analysis and confirmation prompt
    """
    # Get detailed preview first
    preview = preview_infra_configuration(ns_id, name, node_groups, description)
    
    # Enhanced summary structure
    summary = {
        "Infra_CREATION_PLAN": {
            "infra_name": name,
            "namespace": ns_id,
            "description": description,
            "total_vms": len(node_groups),
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
    
    for i, vm_config in enumerate(node_groups):
        vm_name = vm_config.get("name", f"vm-{i+1}")
        common_spec = vm_config.get("specId", "")
        common_image = vm_config.get("imageId", "AUTO-MAPPED")
        nodegroup_size = int(vm_config.get("nodeGroupSize", 1))
        
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
        total_vm_hourly_cost = vm_hourly_cost * nodegroup_size
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
            "instance_count": nodegroup_size,
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
        "estimated_total_time": f"{max(5, len(node_groups) * 2)}-{max(10, len(node_groups) * 5)} minutes",
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
🔥 Infra CREATION SUMMARY - READY TO DEPLOY

📋 DEPLOYMENT OVERVIEW:
• Infra Name: {name}
• Namespace: {ns_id}
• Total VMs: {len(node_groups)} configurations
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

✅ Ready to proceed with Infra creation!
"""
        
        next_steps = [
            "✅ Approve: Call create_infra_dynamic() with skip_confirmation=True to proceed",
            "📝 Review: Use hold=True to create but hold for manual review",
            "✏️ Modify: Adjust node_groups if needed and re-run this summary"
        ]
        
        summary["USER_CONFIRMATION"]["ready_to_proceed"] = True
        
    else:
        confirmation_msg = f"""
⚠️ Infra CONFIGURATION HAS ISSUES

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
    total_vm_instances = sum(int(vm.get("nodeGroupSize", 1)) for vm in node_groups)
    summary["RESOURCE_ESTIMATE"] = {
        "total_vm_instances": total_vm_instances,
        "unique_configurations": len(node_groups),
        "multi_cloud_deployment": csp_dist.get("total_csps", 0) > 1,
        "estimated_deployment_time": f"{2 + len(node_groups)}~{5 + len(node_groups) * 2} minutes"
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
READY TO CREATE Infra '{name}'

Your multi-cloud infrastructure is configured and ready for deployment:
- {len(node_groups)} VM configuration(s) across {csp_dist.get('total_csps', 0)} cloud provider(s)
- {total_vm_instances} total VM instance(s) will be created
- Deployment mode: {'Review first (hold=True)' if hold else 'Immediate deployment'}

Do you want to proceed with Infra creation?
"""
        next_steps = [
            "Confirm: Proceed with create_infra_dynamic()",
            "Review: Use hold=True to review before deployment",
            "Modify: Adjust VM configurations if needed"
        ]
    else:
        validation_issues = resource_summary.get("validation_issues", 0)
        confirmation_msg = f"""
Infra CONFIGURATION HAS ISSUES

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
@tool()
def validate_vm_spec_image_compatibility(node_groups: List[Dict]) -> Dict:
    """
    Validate that VM configurations have proper spec-to-image mapping.
    This tool helps identify potential compatibility issues before Infra creation.
    
    **VALIDATION CHECKS:**
    - Spec format validation (provider+region+spec_name)
    - Image format validation for each CSP
    - Cross-reference CSP in spec vs image identifier
    - Region compatibility checks where possible
    
    Args:
        node_groups: List of VM configurations to validate
    
    Returns:
        Validation results with detailed compatibility analysis
    """
    validation_result = {
        "overall_status": "checking",
        "total_configurations": len(node_groups),
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
    
    for i, vm_config in enumerate(node_groups):
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
            "Use create_infra_dynamic() which automatically handles spec-to-image mapping",
            "For manual mapping, ensure image identifiers match the CSP in specId",
            "AWS: use AMI IDs (ami-xxxxxx), Azure: use Microsoft images or subscription paths, GCP: use project paths"
        ]
    
    return validation_result




    


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
    infra_name: str,
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
        node_groups = []
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
                "nodeGroupSize": str(instances_per_region),
                "rootDiskSize": disk_size,  # Set disk size (minimum 50GB)
                "rootDiskType": "default"
            }
            
            node_groups.append(vm_config)
        
        if not node_groups:
            return {
                "status": "error",
                "error": "No suitable VM configurations found for target regions"
            }
        
        # Create Infra
        infra_result = create_infra_dynamic(
            ns_id=namespace_id,
            name=infra_name,
            node_groups=node_groups,
            description=f"Infrastructure for {deployment_plan['application_config']['name']}",
        )
        
        return {
            "status": "success",
            "infra_id": infra_name,
            "infra_result": infra_result,
            "node_groups": node_groups,
            "total_vms": sum(int(vm["nodeGroupSize"]) for vm in node_groups)
        }
        
    except Exception as e:
        return {
            "status": "error",
            "error": str(e)
        }

# Helper function: Deploy application to infrastructure
def _deploy_application_to_infrastructure(
    app_config: Dict,
    infra_id: str,
    namespace_id: str,
    deployment_plan: Dict
) -> Dict:
    """Deploy application to provisioned infrastructure."""
    
    try:
        # Wait for Infra to be ready
        import time
        max_wait_time = 300  # 5 minutes
        wait_interval = 10   # Check every 10 seconds
        waited_time = 0
        
        while waited_time < max_wait_time:
            infra_status = get_infra(namespace_id, infra_id)
            if infra_status.get("status") == "success":
                infra_data = infra_status.get("infra", {})
                if infra_data.get("status") == "running":
                    break
            
            time.sleep(wait_interval)
            waited_time += wait_interval
        
        if waited_time >= max_wait_time:
            return {
                "status": "error",
                "error": "Infra did not reach running state within timeout period"
            }
        
        # Get Infra access info for command expansion
        access_info = _infra_access_info(namespace_id, infra_id, show_ssh_key=False)
        
        # Execute deployment commands
        deployment_commands = app_config["commands"]
        expanded_commands = []
        
        for cmd in deployment_commands:
            expanded_cmd = _expand_command_templates(cmd, access_info, infra_id)
            expanded_commands.append(expanded_cmd)
        
        # Execute commands on Infra
        execution_result = execute_command_infra(
            ns_id=namespace_id,
            infra_id=infra_id,
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

# Helper function: Expand command templates
def _expand_command_templates(command: str, access_info: Dict, infra_id: str) -> str:
    """
    Expand template variables in commands with MapUI compatibility.
    
    Note: CB-Tumblebug built-in functions ($$Func(...)) are processed by the CB-Tumblebug server
    during command execution, so they should be passed through as-is without modification.
    
    Built-in functions include:
    - $$Func(GetPublicIP(target=this))
    - $$Func(GetPublicIPs(separator=' '))
    - $$Func(GetInfraId())
    - $$Func(AssignTask(task='...'))
    - etc.
    """
    
    expanded_cmd = command
    
    # Skip processing if command contains CB-Tumblebug built-in functions
    if "$$Func(" in command:
        return command  # Return as-is for CB-Tumblebug to process
    
    # Extract node IPs for manual template variables
    public_ips, private_ips = _extract_infra_ips(access_info)
    
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
    
    # Infra ID replacement
    expanded_cmd = expanded_cmd.replace("{{infra_id}}", infra_id)
    
    # Special placeholder for Ray head IP (for worker nodes)
    # This would need to be set by the deployment context
    expanded_cmd = expanded_cmd.replace("{{ray_head_ip}}", public_ips[0] if public_ips else "RAY_HEAD_IP_PLACEHOLDER")
    
    return expanded_cmd

# Helper function: Collect service endpoints
def _collect_service_endpoints(
    app_config: Dict,
    infra_id: str,
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
            access_info = _infra_access_info(namespace_id, infra_id, show_ssh_key=False)
            access_data = access_info.get("access_info", {})
            
            for nodegroup in access_data.get("infra_nodegroup_access_info", []):
                for vm in nodegroup.get("vm_access_info_array", []):
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
            "🛠️ Use execute_command_infra() for additional configuration"
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







# ===== Credential Management Tools =====

# Tool: List Credential Holders
@tool()
def list_credential_holders() -> Dict:
    """
    List all credential holders registered in the system.
    
    A credential holder represents a set of CSP credentials (e.g., for different teams,
    environments, or users). Each holder can have credentials for multiple cloud providers.
    
    Use this to discover available credential holders before setting the
    x-credential-holder header via the credential_holder parameter in API calls.
    
    Returns:
        Dict with "credentialHolderList": list of holders, each with:
        - credentialHolder: Holder identifier (e.g., "admin", "dev-team")
        - providers: List of cloud providers with credentials (e.g., ["aws", "gcp", "azure"])
        - connectionCount: Total connection configurations
        - verifiedConnectionCount: Number of verified connections
        - isDefault: Whether this is the system default holder
    """
    return api_request("GET", "/credentialHolder")

# Tool: Get Credential Holder Details
@tool()
def get_credential_holder(holder_id: str) -> Dict:
    """
    Get detailed information about a specific credential holder.
    
    Args:
        holder_id: Credential holder identifier
    
    Returns:
        Detailed credential holder information including providers and connection counts
    """
    return api_request("GET", f"/credentialHolder/{holder_id}")


# ===== MCP Server Prompts =====
# These prompts help users understand and effectively use the TB-MCP server capabilities
# Based on MapUI patterns for comprehensive cloud infrastructure management

@mcp.prompt()
def infra_creation_workflow():
    """
    Complete Infra Creation Workflow Guide with LLM Interaction Patterns
    
    This comprehensive prompt enforces the correct workflow for creating Multi-Cloud Infrastructure (Infra)
    and provides detailed LLM behavior guidelines for proper user interaction.
    ALL Infra creation MUST follow this workflow to prevent deployment failures.
    VM configurations MUST include both specId and imageId - no exceptions.
    """
    return """# 🚨 COMPLETE Infra Creation Workflow & LLM Interaction Guide

## ⚠️ CRITICAL: ALWAYS Follow This Exact Workflow

This comprehensive guide covers both the technical workflow and LLM behavior patterns
for creating Multi-Cloud Infrastructure (Infra) with proper user interaction.

### 🔄 STEP-BY-STEP PROCESS (NO SHORTCUTS ALLOWED):

⚠️ **IMPORTANT**: VM configurations MUST include both `specId` AND `imageId` - no exceptions!

📋 **OS TYPE SPECIFICATION OPTIONS:**
- Simple OS name: `"ubuntu"`, `"centos"`, `"windows"`, `"debian"`, `"rhel"`
- OS with specific version: `"ubuntu 22.04"`, `"centos 7"`, `"windows server 2019"`, `"debian 11"`
- Use `get_search_options(kind='image')` to see all available osType values

#### STEP 0: 🔧 GET AVAILABLE OPTIONS (MANDATORY FIRST STEP)
```python
# 0.1: ALWAYS get available options before recommend_vm_spec
spec_options = get_search_options(kind='spec')
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
node_groups = []
for i, spec in enumerate(specs["summarized_specs"][:2]):
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
    node_groups.append({
        "specId": spec_id,                    # MUST use exact ID from API
        "imageId": selected_image_id,         # 🚨 REQUIRED - CSP-specific image ID
        "name": f"vm-{spec['providerName']}-{i+1}",
        "nodeGroupSize": 1
    })

# 1.3: 🔍 MANDATORY REVIEW - Always do this first!
review_result = review_infra_dynamic_request(
    ns_id="default",
    name="my-infrastructure",
    node_groups=node_groups
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
    # Check nodeReviews for specific warnings
    creation_viable = True  # Can proceed with caution
elif review_result.get("overallStatus") == "Error":
    print("❌ Errors detected - Must fix before proceeding") 
    creation_viable = False
    # Fix issues in node_groups and re-run review

# 2.2: Review cost estimates
print(f"💰 Estimated cost: {review_result.get('estimatedCost')}")
print(f"🖥️ Total VMs: {review_result.get('totalNodeCount')}")
```

#### STEP 3: 🚀 Infra CREATION (Only After Successful Review)
```python
# 3.1: Create Infra only if review passed
if creation_viable:
    infra_result = create_infra_dynamic(
        ns_id="default",
        name="my-infrastructure",
        node_groups=node_groups,  # Already includes specId + imageId
        force_create=True  # 🚨 REQUIRED after review
    )
    print(f"✅ Infra created: {infra_result.get('id')}")
else:
    print("❌ Cannot create Infra - fix validation issues first")
```

## 🚫 FORBIDDEN PATTERNS:

### ❌ NEVER Do This:
```python
# DON'T: Skip review step
create_infra_dynamic(ns_id="default", name="test", node_groups=[...])
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
review = review_infra_dynamic_request(...)
create_infra_dynamic(..., force_create=True)  # Without checking review
```

### ✅ ALWAYS Do This:
```python
# DO: Follow the complete three-step process
specs = recommend_vm_spec(...)
vm_configs = []
for spec in specs["summarized_specs"]:
    # REQUIRED: Search for compatible images with specific OS version
    images = search_images(matched_spec_id=spec["id"], os_type="ubuntu 22.04")  # Specify exact OS+version
    selected_image = images["imageList"][0]["cspImageName"]
    
    vm_configs.append({
        "specId": spec["id"],
        "imageId": selected_image,  # 🚨 REQUIRED
        "name": f"vm-{spec['providerName']}-1"
    })

review = review_infra_dynamic_request(ns_id="default", name="test", node_groups=vm_configs)
if review["overallStatus"] == "Ready":
    create_infra_dynamic(ns_id="default", name="test", node_groups=vm_configs, force_create=True)

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
- `creationViable`: Boolean - can create Infra?
- `estimatedCost`: Cost per hour (e.g., "$0.0837/hour")
- `totalNodeCount`: Total VMs including NodeGroup sizes
- `nodeReviews`: Per-VM validation details

### Decision Matrix:
- **"Ready" + creationViable: true** → ✅ Proceed with creation
- **"Warning" + creationViable: true** → ⚠️ Review warnings, then proceed
- **"Error" + creationViable: false** → ❌ Fix errors, re-run review

## 🎯 LLM IMPLEMENTATION GUIDELINES:

1. **NEVER skip the review step** - Always call review_infra_dynamic_request() first
2. **NEVER omit imageId** - VM configurations MUST include both specId and imageId
3. **ALWAYS use matched_spec_id** in search_images() for compatibility
4. **SPECIFY OS version when needed** - Use os_type with versions like "ubuntu 22.04", "centos 7", "windows server 2019"
5. **ALWAYS check overallStatus** before proceeding to creation
6. **ALWAYS use force_create=True** in create_infra_dynamic() after review
7. **EXPLAIN the complete process** to users when they request Infra creation
8. **SHOW cost estimates** from review results before creation
9. **FIX validation issues** if overallStatus is "Error" before creating

## 🤖 CRITICAL LLM USER INTERACTION RULES:

### ✅ REQUIRED USER INTERACTION PATTERN:
```
User: "Create an Infra with 2 VMs"

LLM Response:
"I'll help you create an Infra with 2 VMs. Let me first validate the configuration and show you the cost estimates before proceeding.

[Calls review_infra_dynamic_request()]

✅ Configuration validated! Here's your deployment plan:
💰 Cost: $0.16/hour (~$115.20/month)
🖥️ VMs: 2 virtual machines (AWS us-east-1, GCP us-central1)

Would you like me to proceed with creating this infrastructure?
- Reply 'Yes' to create the Infra
- Reply 'No' to cancel
- Reply 'Details' for more information"

[Waits for user confirmation]

User: "Yes"

LLM: [Calls create_infra_dynamic(force_create=True)]
"✅ Infra 'user-infra-12345' created successfully!"
```

### ❌ FORBIDDEN USER INTERACTION PATTERNS:
```
User: "Create an Infra"

❌ WRONG LLM Response:
"Creating Infra now..."
[Calls create_infra_dynamic() directly]  // Will fail with error!

❌ WRONG LLM Response:
"Infra created successfully!"
[No review, no cost disclosure, no user confirmation]
```

### 📊 MANDATORY USER INFORMATION DISCLOSURE:
**Before ANY Infra creation, LLM MUST show user:**
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
    "nodeGroupSize": 1                           # Number of VMs - REQUIRED
}
```

This workflow prevents expensive deployment failures and ensures reliable infrastructure creation.

## 🤖 DETAILED LLM BEHAVIOR GUIDE FOR Infra CREATION

### 🎯 MANDATORY LLM BEHAVIOR WHEN USER REQUESTS Infra CREATION:

#### STEP 1: 🚨 ALWAYS START WITH REVIEW (NEVER SKIP)
```python
# ✅ CORRECT: Always call review first
review_result = review_infra_dynamic_request(
    ns_id="default",
    name="user-requested-infra",
    node_groups=node_groups
)

# ❌ WRONG: Never call create_infra_dynamic directly
# create_infra_dynamic(...)  # This will return an error!
```

#### STEP 2: 📊 ANALYZE REVIEW RESULTS AND INFORM USER
Based on `review_result._llm_guidance.status`:

**🟢 "READY_TO_CREATE":**
```python
# Display the user_prompt from _llm_guidance directly to user
user_message = review_result["_llm_guidance"]["user_prompt"]
print(user_message)  # Shows cost, specs, asks for confirmation

# Wait for user response: "Yes", "No", or "Details"
user_response = ask_the_user()   # not a tool - prompt the human

if user_response.lower() in ["yes", "y", "proceed", "create"]:
    # User approved - proceed with creation
    infra = create_infra_dynamic(..., force_create=True)
    print(f"✅ Infra '{infra['id']}' created successfully!")
elif user_response.lower() in ["no", "n", "cancel"]:
    print("❌ Infra creation cancelled by user.")
else:
    # Show detailed information
    print("📋 Detailed validation results:", review_result["nodeReviews"])
```

**🟡 "READY_WITH_WARNINGS":**
```python
# Show warnings and ask user to decide
warnings_message = review_result["_llm_guidance"]["user_prompt"]
print(warnings_message)  # Displays warnings and asks for decision

user_response = ask_the_user()   # not a tool - prompt the human
if user_response.lower() in ["yes", "proceed"]:
    infra = create_infra_dynamic(..., force_create=True)
    print("⚠️ Infra created with warnings noted.")
else:
    print("Configuration needs adjustment. Let me help fix the warnings.")
    # Guide user through fixing warnings
```

**🔴 "CANNOT_CREATE":**
```python
# Show errors and offer help
error_message = review_result["_llm_guidance"]["user_prompt"]
print(error_message)  # Explains errors and offers help options

# DO NOT proceed to create_infra_dynamic
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
"Great! I've validated your Infra configuration and everything looks good:

💰 Cost: $X.XX/hour (approximately $Y.YY/month)  
🖥️ VMs: N virtual machines across M providers
✅ Status: All configurations validated

The infrastructure will be created with these specifications:
[Display VM summary from review]

Would you like me to proceed with creating this Infra infrastructure?
- Reply 'Yes' to create the infrastructure
- Reply 'No' to cancel  
- Reply 'Details' for more information"
```

#### ⚠️ When Review Shows Warnings:
```
"I've reviewed your Infra configuration. It can be created but there are some warnings:

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
"I've reviewed your Infra configuration and found issues that must be fixed:

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

1. **NEVER** call `create_infra_dynamic()` without calling `review_infra_dynamic_request()` first
2. **ALWAYS** show the user the cost estimates and configuration details before creating
3. **ALWAYS** wait for explicit user confirmation before proceeding to creation
4. **NEVER** assume user consent - always ask for confirmation
5. **ALWAYS** use the `_llm_guidance.user_prompt` field for user messages
6. **NEVER** proceed with creation if `creationViable: false`
7. **ALWAYS** help users fix errors instead of just reporting them

### 📊 LLM RESPONSE TEMPLATES:

**User Request:** "Create an Infra with 2 VMs in AWS"

**LLM Response Pattern:**
```
"I'll help you create an Infra with 2 VMs in AWS. Let me first validate the configuration and check costs.

[Calls review_infra_dynamic_request()]

[Based on review results, uses appropriate response template above]
```

**NEVER:**
```
"I'll create an Infra for you."
[Calls create_infra_dynamic() directly]  // ❌ WRONG!
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
list_infra_templates()  # Get available predefined applications

# 2. If found in APPLICATION_CONFIGS → Use Strategy A (Usecase Commands)
# 3. If NOT found → Use Strategy B (LLM-Generated Commands)
```

### 🎯 STRATEGY A: Predefined Applications (APPLICATION_CONFIGS)

**📋 For applications in APPLICATION_CONFIGS (xonotic, nginx, ollama, jitsi, elk, ray):**

#### ✅ USE EXISTING USECASE COMMANDS - DO NOT REINVENT
```python
# EXAMPLE: Xonotic Game Server
execute_command_infra(ns_id, infra_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/xonotic/startServer.sh; chmod +x ~/startServer.sh",
    "sudo ~/startServer.sh {{infra_id}} 26000 8 8",
    "echo 'Server Address: {{public_ip}}:26000'"
])

# EXAMPLE: Nginx Web Server  
execute_command_infra(ns_id, infra_id, [
    "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/nginx/startServer.sh | bash -s -- --ip {{public_ip}}",
    "echo 'Web Server: http://{{public_ip}}'"
])

# EXAMPLE: Ollama LLM Server (Basic Setup)
execute_command_infra(ns_id, infra_id, [
    "curl -fsSL https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/llm/deployOllama.sh | sh",
    "echo 'LLM Server: http://{{public_ip}}:3000'"
])

# Pulling specific Ollama models: ask the user which ones (model names change often,
# see https://ollama.com/search), then pull them with an ordinary remote command.
execute_command_infra(ns_id, infra_id, [
    "ollama pull llama3.3:latest && ollama pull qwen2.5:14b",
    "ollama list"
])

# EXAMPLE: Jitsi Meet Conference
execute_command_infra(ns_id, infra_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/jitsi/startServer.sh",
    "chmod +x ~/startServer.sh",
    "sudo ~/startServer.sh {{public_ips_space}} DNS EMAIL"
])

# EXAMPLE: ELK Stack
execute_command_infra(ns_id, infra_id, [
    "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/elastic-stack/startELK.sh",
    "chmod +x ~/startELK.sh",
    "sudo ~/startELK.sh"
])

# EXAMPLE: Ray ML Cluster
execute_command_infra(ns_id, infra_id, [
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
guide = get_usage_guide('provision'"application_name", "production")

# 2. Use guide ONLY for hardware specs, NOT for exact commands
vm_spec_filter = guide["vm_spec_filter"]  # Use this for VM specifications
installation_commands = guide["commands"]["installation"]  # IGNORE - Generate your own

# 3. LLM generates intelligent deployment commands based on application knowledge
```

**🤖 LLM Command Generation Examples:**

```python
# Example 1: MongoDB deployment (LLM-generated)
execute_command_infra(ns_id, infra_id, [
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
execute_command_infra(ns_id, infra_id, [
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
execute_command_infra(ns_id, infra_id, [
    "sudo apt-get update -y",
    "sudo apt-get install -y redis-server",
    "sudo sed -i 's/bind 127.0.0.1/bind 0.0.0.0/' /etc/redis/redis.conf",
    "sudo systemctl restart redis-server",
    "sudo systemctl enable redis-server",
    "redis-cli ping",
    "echo 'Redis Server: {{public_ip}}:6379'"
])

# Example 4: Apache Kafka (LLM-generated)
execute_command_infra(ns_id, infra_id, [
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
templates = list_infra_templates()

# If not predefined, get reference guide for hardware specs only
guide = get_usage_guide('provision'"application_name", "production")
```

#### Step 2: 🏗️ Prepare Namespace
```python
manage_namespace(action='create', name="my-app-namespace")
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
node_groups = [{
    "specId": spec["id"],
    "name": f"vm-{app_name}-1",
    "description": f"VM for {app_name}",
    "nodeGroupSize": 1
}]
```

#### Step 5: ✅ Validate Configuration (MANDATORY)
```python
review = review_infra_dynamic_request(ns_id, name, node_groups)
```

#### Step 6: 🚀 Create Infrastructure
```python
infra = create_infra_dynamic(ns_id, name, node_groups, force_create=True)
```

#### Step 7: 📦 Install Application
```python
# Strategy A: Use predefined APPLICATION_CONFIGS commands
# Strategy B: Use LLM-generated intelligent commands
execute_command_infra(ns_id, infra_id, deployment_commands)
```

#### Step 8: 🔍 Verify Deployment
```python
# Generate appropriate verification commands
execute_command_infra(ns_id, infra_id, verification_commands)
```

#### Step 9: 📋 Collect Access Information
```python
access_info = _infra_access_info(ns_id, infra_id)
```

### 🎯 LLM BEHAVIOR REQUIREMENTS

**🚨 CRITICAL LLM Decision Rules:**
1. **ALWAYS check list_infra_templates() first**
2. **IF application in templates → Use Strategy A (exact usecase commands)**
3. **IF application NOT in templates → Use Strategy B (LLM-generated commands)**
4. **Use get_usage_guide('provision') as REFERENCE ONLY for Strategy B**
5. **NEVER skip validation step**
6. **ALWAYS complete all 9 steps**

### 📋 Example Decision Process:

```
User: "Deploy xonotic game server"
LLM Decision: 
1. Check templates → xonotic found in APPLICATION_CONFIGS
2. Use Strategy A → Execute exact usecase commands
3. Commands: wget startServer.sh; sudo ~/startServer.sh {{infra_id}} 26000 8 8

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

result = execute_command_infra(ns_id, infra_id, installation_commands)

print("✅ Deployment completed! Checking service status...")
```

```python
# ❌ BAD: No warning about timing
result = execute_command_infra(ns_id, infra_id, installation_commands)
# User may think system is frozen
```

### 🎯 SUCCESS METRICS

**✅ Successful deployment must:**
1. Follow correct strategy (A or B) based on application type
2. Complete all 9 workflow steps
3. Pass Infra configuration validation
4. Execute appropriate installation commands
5. Verify deployment success
6. Provide clear access information

**🚨 This intelligent approach ensures:**
- **Predefined apps**: Use tested, optimized scripts
- **General apps**: LLM creativity with reliable workflow
- **All apps**: Consistent validation and verification

### 📚 DEPLOYMENT GUIDES: Reference Information Only

**🔍 About get_usage_guide('provision'):**

This tool provides **REFERENCE INFORMATION ONLY** for Strategy B (general applications):

```python
# ✅ Correct usage for Strategy B
guide = get_usage_guide('provision'"mongodb", "production")
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
   connections = get_connections(filter_verified=True)
   
   # Try different regions or providers
   specs = recommend_vm_spec(
       filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
       priority_policy="cost"  # Try cost-optimized specs
   )
   ```

2. **Simplify Configuration:**
   ```python
   # Reduce VM count or specs
   node_groups = [{
       "specId": "smaller_spec_id",  # Use smaller instance
       "nodeGroupSize": 1               # Start with single VM
   }]
   ```

3. **Validate Before Retry:**
   ```python
   # Always validate before retrying
   review = review_infra_dynamic_request(ns_id, name, node_groups)
   ```

#### 2. **Timeout Errors (10+ minute operations)**
```
Error: "Request timeout - operation took longer than 10 minutes"
```

**🔍 Diagnosis:**
- Very long Infra creation times exceed client connection timeout
- Complex multi-region deployments or large cluster operations
- Resource contention or CSP-side delays

**✅ LLM Recovery Actions:**
1. **Check Infra Status:**
   ```python
   # Check if Infra was partially created despite timeout
   infra_list = get_infra_list(ns_id, option="status")
   
   # Look for the Infra name in results
   for infra in infra_list.get("infra", []):
       if infra["name"] == "your-infra-name":
           print(f"Infra Status: {infra['status']}")
   ```

2. **Retry with Simpler Configuration:**
   ```python
   # Start with single region/provider
   node_groups = [{
       "specId": single_region_spec,
       "nodeGroupSize": 1
   }]
   ```

3. **Use Staged Deployment:**
   ```python
   # Deploy incrementally
   # Phase 1: Core infrastructure
   create_infra_dynamic(ns_id, "core-infra", [core_vm])
   
   # Phase 2: Additional resources
   create_infra_dynamic(ns_id, "additional-infra", [additional_vms])
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
Error: "Infra configuration validation failed"
```

**✅ LLM Recovery Actions:**
1. **Analyze Validation Results:**
   ```python
   review = review_infra_dynamic_request(ns_id, name, node_groups)
   
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
guide = get_usage_guide('provision'"nginx", "production")
# This provides hardware requirements, installation commands, and verification steps
```

### Step 2: Create or Validate Namespace
```python
# Ensure proper namespace exists
namespace_result = manage_namespace(action='create', name="my-app")
# Or create new one: manage_namespace(action='create'"my-app-production")
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
node_groups = []
for i, spec in enumerate(specs["summarized_specs"][:2]):
    node_groups.append({
        "specId": spec["id"],  # Use exact spec ID from API
        "name": f"app-vm-{i+1}",
        "description": f"VM for {application_name} in {spec['regionName']}",
        "nodeGroupSize": 1
        # imageId: Auto-mapped based on spec
    })
```

### Step 5: Validate Infra Configuration (MANDATORY)
```python
# Always validate before creating Infra
review_result = review_infra_dynamic_request(
    ns_id="my-app",
    name="my-app-infra",
    node_groups=node_groups
)

# Check validation results
if review_result.get("summary", {}).get("validationPassed", False):
    print("✅ Configuration validated - safe to proceed")
else:
    print("❌ Validation failed - fix issues first")
    # Handle validation errors
```

### Step 6: Create Infra Infrastructure
```python
# Create the infrastructure
infra_result = create_infra_dynamic(
    ns_id="my-app",
    name="my-app-infra",
    node_groups=node_groups,
    description="Infrastructure for my application",
    force_create=True  # Skip confirmation since we validated
)
```

### Step 7: Deploy Application Using Remote Commands
```python
# Get installation commands from deployment guide
installation_commands = guide["commands"]["installation"]

# Execute installation commands
deployment_result = execute_command_infra(
    ns_id="my-app",
    infra_id="my-app-infra",  # From infra_result
    commands=installation_commands
)
```

### Step 8: Verify Deployment
```python
# Get verification commands from deployment guide
verification_commands = guide["commands"]["verification"]

# Verify installation
verification_result = execute_command_infra(
    ns_id="my-app",
    infra_id="my-app-infra",
    commands=verification_commands
)
```

### Step 9: Collect Access Information
```python
# Get access information and endpoints
access_info = _infra_access_info("my-app", "my-app-infra", show_ssh_key=False)

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
guide = get_usage_guide('provision'"my-custom-app", "production")
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

1. **ALWAYS use get_usage_guide('provision') FIRST**
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
4. Validate the Infra configuration...
5. Create the infrastructure...
6. Install nginx using remote commands...
7. Verify the deployment...
8. Provide access information..."
```

### Example Commands Flow:
```python
# 1. Get deployment guide
guide = get_usage_guide('provision'"nginx", "production")

# 2. Prepare namespace
manage_namespace(action='create', name="web-servers")

# 3. Get VM specs
specs = recommend_vm_spec(
    filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
    priority_policy="cost"
)

# 4. Build VM config
vm_configs = [{
    "specId": specs["summarized_specs"][0]["id"],
    "name": "nginx-vm-1",
    "description": "Nginx web server VM"
}]

# 5. Validate
review = review_infra_dynamic_request("web-servers", "nginx-infra", vm_configs)

# 6. Create Infra (if validation passed)
infra = create_infra_dynamic("web-servers", "nginx-infra", vm_configs, force_create=True)

# 7. Install nginx
execute_command_infra("web-servers", "nginx-infra", [
    "sudo apt-get update -y",
    "sudo apt-get install -y nginx",
    "sudo systemctl start nginx",
    "sudo systemctl enable nginx",
    "echo 'Web Server accessible at: http://{{public_ip}}'"
])

# 8. Verify
execute_command_infra("web-servers", "nginx-infra", [
    "curl -I http://{{public_ip}}",
    "sudo systemctl is-active nginx"
])

# 9. Get access info
access_info = _infra_access_info("web-servers", "nginx-infra")
```

## 🎯 SUCCESS METRICS

### A successful application deployment should:
1. ✅ Complete all 9 steps without errors
2. ✅ Pass Infra configuration validation
3. ✅ Successfully execute installation commands
4. ✅ Pass verification commands
5. ✅ Provide clear access information to user
6. ✅ Include troubleshooting guidance

This approach ensures reliable, debuggable, and maintainable application deployments.

## 🚀 SPECIAL CASE: Dynamic LLM Model Discovery for Ollama

### 🔍 NEW APPROACH: Real-Time Model Discovery (Recommended for Ollama)

When users request Ollama deployment, use the NEW dynamic discovery workflow:

#### Step 1: Agree on the models with the user
```python
# Model names change often, so confirm them rather than guessing:
# point the user at https://ollama.com/search and take the exact names back.

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
    infra_name="ollama-cluster",
    selected_models=selected_models,
    description="Custom LLM deployment with latest models from ollama.com"
)
```

#### Step 4: Alternative - Fallback to APPLICATION_CONFIGS
```python
# If user prefers not to browse ollama.com, use APPLICATION_CONFIGS fallback
# This will show guidance to visit ollama.com but use predefined workflow
execute_command_infra(ns_id, infra_id, [
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
[asks the user which models to pull]

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
    including namespace management, Infra operations, and resource optimization.
    """
    return """# CB-Tumblebug Infrastructure Management Guide

## Core Infrastructure Operations

### 1. Namespace Management
Namespaces organize your cloud resources:
```
manage_namespace(action='create'"production") - Create production environment
manage_namespace(action='create'"staging") - Create staging environment  
get_namespaces() - List all namespaces
manage_namespace(action='delete'"test") - Clean up test environment
```

### 2. Infra (Multi-Cloud Infrastructure) Lifecycle
```
# Creation Methods
create_infra_dynamic() - Full control with VM configurations
create_infra_dynamic() - Flexible Infra creation with custom VM configurations
recommend_vm_spec() - Find optimal VM specifications
search_images() - Find suitable OS images

# Management  
get_infra_list("production") - List infrastructure
get_infra("production", "web-servers") - Get detailed info
control_infra("production", "web-servers", "suspend") - Control operations
terminate_infra("production", "web-servers", confirm=True) - Clean up
```

### 3. Resource Discovery
```
# Cloud Providers
get_connections() - Available cloud connections
get_connections() - Filtered connections

# Infrastructure Components
get_resources(kind='vNet'"production") - Virtual networks
get_resources(kind='securityGroup'"production") - Security groups  
get_resources(kind='sshKey'"production") - SSH key pairs
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
_infra_access_info("production", "web-servers") - SSH and IP info
execute_command_infra() - Run commands remotely
transfer_file_infra() - File transfer operations
```

### 7. Resource Cleanup
```
# Gradual cleanup
terminate_infra("test", "temp-servers", confirm=True) - Remove specific Infra
release_resources("test") - Remove shared resources (VNet, etc.)
manage_namespace(action='delete'"test") - Complete cleanup

```

## Advanced Patterns

### 1. Multi-Region Deployment
```python
# Deploy across multiple regions
regions = ["aws+ap-northeast-2", "azure+koreacentral", "gcp+asia-northeast3"]
for region in regions:
    create_infra_dynamic(
        ns_id="global-app",
        name=f"app-{region.split('+')[0]}",
        node_groups=[{
            "imageId": selected_image,
            "specId": f"{region}+standard-instance",
            "nodeGroupSize": 3
        }]
    )
```

### 2. Environment Separation
```
# Development
manage_namespace(action='create'"dev")
create_infra_dynamic("dev", "test-app", [{"imageId": image, "specId": "t2.micro", "name": "test-vm"}])

# Staging  
manage_namespace(action='create'"staging")
create_infra_dynamic("staging", "staging-app", [{"imageId": image, "specId": "t2.small", "name": "staging-vm"}])

# Production
manage_namespace(action='create'"production") 
create_infra_dynamic("production", "prod-app", complex_config)
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
    "nodeGroupSize": 5
}
```

## Monitoring and Troubleshooting
1. Use get_infra_list() for status monitoring
2. Check get_infra() for detailed VM information  
3. Review execute_command_infra() outputs for issues
4. Use _infra_access_info() for connectivity verification

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
1. create_infra_dynamic("web-app", "nginx-aws", [{"imageId": aws_image, "specId": "t3.medium", "name": "nginx-aws-vm"}])
2. create_infra_dynamic("web-app", "nginx-azure", [{"imageId": azure_image, "specId": "Standard_B2s", "name": "nginx-azure-vm"}]) 
3. execute_command_infra(script_name="nginx_install")
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
1. manage_namespace(action='create'"dev-team")
2. create_infra_dynamic("dev-team", "dev-servers", [{"imageId": ubuntu_image, "specId": "t3.large", "name": f"dev-vm-{i}", "nodeGroupSize": 1} for i in range(1, 6)])
3. execute_command_infra(script_name="docker_install")
4. execute_command_infra(script_name="monitoring_setup") 
5. execute_command_infra(script_name="security_hardening")
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
1. manage_namespace(action='create') - Environment isolation
2. search_images() - Find suitable OS images  
3. recommend_vm_spec() - Optimize instance selection
4. get_connections() - Verify cloud provider availability
```

### 2. Application Deployment
```
1. deploy_application() - Automated deployment
2. _infra_access_info() - Collect endpoints
3. execute_command_infra() - Post-deployment configuration
4. execute_command_infra() - Ongoing management
```

### 3. Monitoring and Management
```
1. get_infra_list() - Monitor infrastructure
2. execute_command_infra() - Health checks
3. control_infra() - Scaling operations
```

### 4. Cleanup and Optimization
```
1. terminate_infra() - Terminate unused infrastructure
2. release_resources() - Clean shared resources
3. manage_namespace(action='delete') - Complete environment cleanup
```

## Best Practices Summary
1. **Start Small**: Use create_infra_dynamic() for testing
2. **Scale Gradually**: Use deploy_application() for production
3. **Monitor Resources**: Regular health checks and cost monitoring  
4. **Security First**: Apply security hardening from day one
5. **Document Endpoints**: Keep track of service URLs and IPs
6. **Plan Cleanup**: Regular resource cleanup to control costs

## 🔍 Infra Validation and Quality Assurance Workflow

### MANDATORY Pre-Validation Process

**✅ ALWAYS Follow This Workflow for Infra Creation:**

```python
# STEP 1: Search for appropriate VM specifications
specs = recommend_vm_spec(
    filter_policies={"vCPU": {"min": 2}, "memoryGiB": {"min": 4}},
    priority_policy="cost"  # or "performance" or "location"
)

# STEP 2: Build VM configurations using ONLY returned spec IDs
node_groups = []
for i, spec in enumerate(specs["summarized_specs"][:2]):
    node_groups.append({
        "specId": spec["id"],  # 🚨 CRITICAL: Use exact ID from API
        "name": f"vm-{spec['providerName']}-{i+1}",
        "description": f"VM in {spec['regionName']}",
        "nodeGroupSize": 1
        # imageId: Optional - auto-mapped if omitted
    })

# STEP 3: MANDATORY - Review configuration before creation
review_result = review_infra_dynamic_request(
    ns_id="my-project",
    name="web-app-cluster", 
    node_groups=node_groups,
    description="Production web application cluster"
)

# STEP 4: Check validation results
if review_result.get("summary", {}).get("validationPassed", False):
    print("✅ Configuration validated successfully!")
    print(f"Estimated cost: {review_result.get('estimated_cost', 'N/A')}")
    print(f"Deployment time: {review_result.get('deployment_time_estimate', 'N/A')}")
    
    # STEP 5: Proceed with Infra creation
    infra_result = create_infra_dynamic(
        ns_id="my-project",
        name="web-app-cluster",
        node_groups=node_groups,
        force_create=True  # Skip confirmation since we already reviewed
    )
    
else:
    print("❌ Validation failed!")
    print("Issues to fix:")
    for vm_validation in review_result.get("vm_validations", []):
        if vm_validation.get("issues"):
            print(f"VM {vm_validation['vm_index']}: {vm_validation['issues']}")
```

### Automatic Pre-Validation in create_infra_dynamic()

**The create_infra_dynamic() function now includes automatic pre-validation:**

1. **Automatic Review**: Every call to create_infra_dynamic() automatically runs review_infra_dynamic_request()
2. **Validation Gate**: Infra creation is blocked if critical validation issues are found
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

### Example: Validated Infra Creation Workflow

**Recommended Pattern for Error-Resilient Infra Creation:**

1. **Get VM Specifications**: Use recommend_vm_spec() to get valid spec IDs
2. **Build Configurations**: Create VM configs using exact spec IDs from API
3. **Validate Configuration**: Use review_infra_dynamic_request() to check setup
4. **Handle Validation Results**: Address any issues before proceeding
5. **Create Infrastructure**: Use create_infra_dynamic() with force_create=True

**Example Usage Pattern:**
- Call recommend_vm_spec() with your requirements
- Use returned spec["id"] values in node_groups
- Run review_infra_dynamic_request() to validate
- Check validation_passed status before proceeding
- Use create_infra_dynamic() with validated configurations

### LLM Integration Guidelines

**When working with Infra creation, LLMs should:**

1. **Always Use Validation**: Never skip the review_infra_dynamic_request() step
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

- ✅ **validation_passed: true** → Safe to proceed with create_infra_dynamic()
- ❌ **validation_passed: false** → Must fix issues before creation
- ⚠️ **warnings > 0** → Consider optimization suggestions
- ℹ️ **info messages** → Informational, no action required

This comprehensive validation system ensures reliable, cost-effective, and properly configured Infra deployments.

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

1. **⚠️ Always warn users first** before any execute_command_infra() call
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
    
    This prompt ensures LLMs always use get_search_options(kind='spec') before recommend_vm_spec()
    to prevent failures from invalid parameters and guide proper usage of the recommendation system.
    """
    return """# 🔧 VM Specification Recommendation Guide

## 🚨 MANDATORY WORKFLOW: Options First, Then Recommendation

### ⚠️ CRITICAL: ALWAYS Call get_search_options(kind='spec') FIRST

**NEVER call recommend_vm_spec() without first calling get_search_options(kind='spec')!**

This prevents common failures from:
- ❌ Invalid metric names in filter policies
- ❌ Non-existent provider names, regions, architectures
- ❌ Wrong operator formats or parameter structures
- ❌ Invalid priority policy configurations

### 🔄 STEP-BY-STEP WORKFLOW:

#### STEP 1: Get Available Options
```python
# ALWAYS start with this call
spec_options = get_search_options(kind='spec'ns_id="system")

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
            "weight": 1.0
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
            "weight": 1.0
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
spec_options = get_search_options(kind='spec')

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
priority_policy = {"policy": [{"metric": "cost", "weight": 1.0}]}

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
spec_options = get_search_options(kind='spec')

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
            "weight": 1.0
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
spec_options = get_search_options(kind='spec')

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
priority_policy = {"policy": [{"metric": "performance", "weight": 1.0}]}

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

@mcp.prompt()
def infrastructure_scaling_and_management_guide():
    """
    Guide for scaling Infra infrastructure, managing labels, and using credential holders.
    Covers NodeGroup management, label-based resource organization, and multi-tenant credentials.
    """
    return """# Infrastructure Scaling, Labels & Credential Management

## 🔄 Scaling an Existing Infra with NodeGroups

### Adding VMs to an Existing Infra
Use `add_nodegroup_dynamic()` to add a new NodeGroup to a running Infra:

```python
# Step 1: Review the NodeGroup configuration first
review = review_nodegroup_dynamic(
    ns_id="default",
    infra_id="my-infra",
    spec_id="aws+ap-northeast-2+t3.medium",
    image_id="ami-0c02fb55956c7d316",
    node_group_size=3,
    name="worker-nodes"
)

# Step 2: If review passes, add the NodeGroup
if review.get("isValid") or review.get("canCreate"):
    result = add_nodegroup_dynamic(
        ns_id="default",
        infra_id="my-infra",
        spec_id="aws+ap-northeast-2+t3.medium",
        image_id="ami-0c02fb55956c7d316",
        node_group_size=3,
        name="worker-nodes",
        description="Worker nodes for batch processing"
    )
```

### Quick Spec-Image Validation
Use `review_spec_image_pair()` for fast validation without full Infra review:

```python
# Lightweight check if a spec+image pair works
check = review_spec_image_pair(
    spec_id="aws+ap-northeast-2+t3.nano",
    image_id="ami-01f71f215b23ba262"
)
if check.get("isValid"):
    print("Spec and image are compatible and available")
```

### NodeGroup Configuration Options
Each NodeGroup supports:
- `specId` / `imageId`: Required spec and image (from recommend_vm_spec / search_images)
- `nodeGroupSize`: Number of VMs (int, e.g., 3)
- `zone`: Specific availability zone (e.g., "ap-northeast-2a")
- `connectionName`: Specific CSP connection
- `rootDiskSize`: Disk size in GB (int, 0 for default)
- `vNetTemplateId` / `sgTemplateId`: Network/security templates
- `label`: Key-value labels for the NodeGroup

### Infra-Level Configuration
When creating an Infra:
- `policyOnPartialFailure`: "continue" (default), "rollback", or "refine"
  - continue: Keep successfully created VMs even if some fail
  - rollback: Delete all VMs if any fail
  - refine: Retry failed VMs with alternative specs
- `vNetTemplateId` / `sgTemplateId`: Infra-level defaults (NodeGroup values override)

## 🏷️ Label Management

Labels are key-value string pairs for organizing and filtering resources.

### Setting Labels
```python
# Add labels to an Infra
manage_labels(action='set'
    label_type="infra",
    uid="my-infra-uid",
    labels={"env": "production", "team": "backend", "app": "web-server"}
)

# Add labels to a node (VM)
manage_labels(action='set'
    label_type="node",
    uid="node-uid-123",
    labels={"role": "worker", "tier": "compute"}
)
```

### Finding Resources by Labels
```python
# Find all production Infras
production_resources = manage_labels(action='find'
    label_type="infra",
    labels="env=production"
)

# Find worker VMs in the backend team
workers = manage_labels(action='find'
    label_type="node",
    labels="role=worker,team=backend"
)
```

### Syncing CSP Labels
```python
# Pull labels set directly on CSP resources (e.g., via AWS Console)
merge_csp_resource_labels(label_type="node", uid="node-uid-123")
```

## 🔐 Credential Holder Management

Credential holders represent different sets of cloud provider credentials,
enabling multi-tenant or multi-environment deployments.

### Listing Available Credential Holders
```python
holders = list_credential_holders()
# Returns list of holders with their providers and connection counts
```

### Using a Specific Credential Holder
Set the `TUMBLEBUG_CREDENTIAL_HOLDER` environment variable, or use per-request override:
```python
# Environment variable (applies to all requests)
os.environ["TUMBLEBUG_CREDENTIAL_HOLDER"] = "dev-team"

# Per-request (not yet exposed as tool parameter - use env var)
```

### Workflow: Multi-Tenant Deployment
1. List available credential holders: `list_credential_holders()`
2. Choose the appropriate holder for your deployment
3. Set `TUMBLEBUG_CREDENTIAL_HOLDER` env var
4. Proceed with Infra creation workflow as normal
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
        # LB/gateway-friendly (aligned with the stateless MCP spec 2026-07-28)
        stateless_http=True,
    )