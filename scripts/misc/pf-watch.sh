#!/usr/bin/env bash
#
# Resilient kubectl port-forward.
#   - detached  : setsid → survives the launching shell / an ephemeral SSH session
#   - self-healing: a retry loop reconnects ~2s after a drop (long request, pod rollout)
# Each forward runs in its own session; its PGID is appended to <pidfile> so `stop` can tear
# down exactly the recorded forwards (loop + its child kubectl) by process group — no fragile
# process-name matching (which would also match the caller and kill it).
#
#   start <pidfile> <kubectl port-forward args...>
#   stop  <pidfile>

set -euo pipefail
KUBECTL="${KUBECTL:-kubectl}"

action="${1:-}"; pidfile="${2:-}"
[ -n "$action" ] && [ -n "$pidfile" ] || { echo "usage: $0 start|stop <pidfile> [args]" >&2; exit 2; }

case "$action" in
  start)
    shift 2
    setsid bash -c 'while :; do "$0" port-forward "$@" >/dev/null 2>&1; sleep 2; done' \
      "$KUBECTL" "$@" </dev/null >/dev/null 2>&1 &
    echo $! >> "$pidfile"
    ;;
  stop)
    [ -f "$pidfile" ] || exit 0
    while read -r p; do
      [ -n "$p" ] || continue
      kill -- -"$p" 2>/dev/null || kill "$p" 2>/dev/null || true   # kill the process group, then the leader
    done < "$pidfile"
    rm -f "$pidfile"
    ;;
  *) echo "unknown action: $action" >&2; exit 2 ;;
esac
