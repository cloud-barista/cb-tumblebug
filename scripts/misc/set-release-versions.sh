#!/usr/bin/env bash
#
# Interactively set the pinned release image versions of the core cb-tumblebug
# components across BOTH deployment modes (docker compose + Helm/k8s).
#
# For each component it shows the current version and the most recent published
# tags from Docker Hub, lets you pick one or type a custom version (e.g. a
# not-yet-published staging tag for cb-tumblebug), then rewrites the version in
# the files that actually pin it:
#
#   docker-compose*.yaml                          (image: cloudbaristaorg/<c>:<ver>)
#   deployments/helm/cb-tumblebug/values.yaml     (images.<c>)
#   deployments/helm/cb-tumblebug/Chart.yaml      (appVersion — cb-tumblebug only)
#
# Usage: make set-versions   (or: bash scripts/misc/set-release-versions.sh)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VALUES="$REPO_ROOT/deployments/helm/cb-tumblebug/values.yaml"
CHART="$REPO_ROOT/deployments/helm/cb-tumblebug/Chart.yaml"

# Components (display order) and their Docker Hub repositories.
COMPONENTS=(cb-tumblebug cb-spider cb-mapui mc-terrarium)
declare -A DHREPO=(
  [cb-tumblebug]=cloudbaristaorg/cb-tumblebug
  [cb-spider]=cloudbaristaorg/cb-spider
  [cb-mapui]=cloudbaristaorg/cb-mapui
  [mc-terrarium]=cloudbaristaorg/mc-terrarium
)

compose_files() { ls "$REPO_ROOT"/docker-compose*.yaml 2>/dev/null; }

# current_version <dockerRepo> — source of truth is values.yaml.
current_version() {
  grep -oE "$1:[0-9][A-Za-z0-9._-]*" "$VALUES" | head -1 | sed "s#$1:##"
}

# The MCP server image is published in lockstep with cb-tumblebug (same version tag —
# see .github/workflows/continuous-delivery-mcp.yaml), so its pin tracks cb-tumblebug's.
MCP_REPO=cloudbaristaorg/cb-tumblebug-mcp-server
current_mcp() { grep -oE "$MCP_REPO:[0-9][A-Za-z0-9._-]*" "$VALUES" | head -1 | sed "s#$MCP_REPO:##"; }
current_appversion() { grep -oE '^appVersion:[^#]*' "$CHART" | head -1 | sed -E 's#appVersion:[[:space:]]*"?([^" ]*)"?.*#\1#'; }

# fetch_tags <dockerRepo> — recent semver tags (x.y.z[-suffix]), newest first.
fetch_tags() {
  curl -fsSL "https://hub.docker.com/v2/repositories/$1/tags?page_size=50&ordering=last_updated" 2>/dev/null \
    | python3 -c "import json,sys,re
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
tags=[t.get('name','') for t in d.get('results',[])]
sem=[t for t in tags if re.match(r'^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?\$', t)]
print('\n'.join(sem[:8]))" 2>/dev/null || true
}

# apply_component <dockerRepo> <newVer> — rewrite one component's pinned tag in compose+values.
# Matches only a versioned tag ("<repo>:<digit>...") so cb-tumblebug never touches
# cb-tumblebug-mcp-server. '#' delimiter avoids clashing with the '/' in the repo.
apply_component() {
  # shellcheck disable=SC2046
  sed -i -E "s#($1):[0-9][A-Za-z0-9._-]*#\1:$2#g" $(compose_files) "$VALUES"
}

echo "Core component release versions (source of truth: helm values.yaml)"
for c in "${COMPONENTS[@]}"; do
  printf "  %-14s %s\n" "$c" "$(current_version "${DHREPO[$c]}")"
done

declare -A NEW
for c in "${COMPONENTS[@]}"; do
  repo="${DHREPO[$c]}"; cur="$(current_version "$repo")"
  echo
  echo "== $c  (current: $cur) =="
  mapfile -t tags < <(fetch_tags "$repo")
  if [ "${#tags[@]}" -gt 0 ]; then
    echo "  recent published tags:"
    for i in 0 1 2; do
      [ -n "${tags[$i]:-}" ] || break
      mark=""; [ "${tags[$i]}" = "$cur" ] && mark="  (current)"
      printf "   %d) %s%s\n" "$((i+1))" "${tags[$i]}" "$mark"
    done
  else
    echo "  (could not fetch tags from Docker Hub — offline? enter a version manually)"
  fi
  if [ "$c" = "cb-tumblebug" ]; then
    echo "   c) custom version  (allowed even if not yet published — e.g. a staging tag)"
    echo "   note: cb-tumblebug-mcp-server (currently $(current_mcp)) is kept at the same version (CI lockstep)"
  else
    echo "   c) custom version"
  fi
  echo "   s) skip (keep $cur)"
  read -rp "  choose [1-3/c/s]: " ans
  case "$ans" in
    1|2|3) sel="${tags[$((ans-1))]:-}"
           if [ -z "$sel" ]; then echo "  no such tag — keeping $cur"; NEW[$c]="$cur"; else NEW[$c]="$sel"; fi ;;
    c|C)   read -rp "  custom version: " v; NEW[$c]="${v:-$cur}" ;;
    *)     NEW[$c]="$cur"; echo "  keeping $cur" ;;
  esac
done

echo
# cb-tumblebug-mcp-server and Chart appVersion always follow the effective cb-tumblebug
# version (lockstep). Using the effective version repairs existing drift even when the
# cb-tumblebug tag itself is kept.
TB_FINAL="${NEW[cb-tumblebug]}"

echo "Planned changes:"
changed=0
for c in "${COMPONENTS[@]}"; do
  cur="$(current_version "${DHREPO[$c]}")"; nv="${NEW[$c]}"
  if [ "$nv" != "$cur" ]; then printf "  %-14s %s -> %s\n" "$c" "$cur" "$nv"; changed=1; fi
done
if [ "$(current_mcp)" != "$TB_FINAL" ]; then
  printf "  %-14s %s -> %s  (tracks cb-tumblebug)\n" "mcp-server" "$(current_mcp)" "$TB_FINAL"; changed=1
fi
if [ "$(current_appversion)" != "$TB_FINAL" ]; then
  printf "  %-14s %s -> %s  (Chart appVersion)\n" "appVersion" "$(current_appversion)" "$TB_FINAL"; changed=1
fi
if [ "$changed" = 0 ]; then echo "  (nothing to change)"; exit 0; fi

echo
read -rp "Apply to docker-compose*.yaml + helm values + Chart appVersion? [y/N]: " ok
case "$ok" in [yY]*) ;; *) echo "Aborted (no files changed)."; exit 0 ;; esac

for c in "${COMPONENTS[@]}"; do
  cur="$(current_version "${DHREPO[$c]}")"; nv="${NEW[$c]}"
  [ "$nv" = "$cur" ] && continue
  apply_component "${DHREPO[$c]}" "$nv"
  printf "  ✔ %-14s -> %s\n" "$c" "$nv"
done
# Lockstep dependents of cb-tumblebug (no-ops if already equal to TB_FINAL).
if [ "$(current_mcp)" != "$TB_FINAL" ]; then
  apply_component "$MCP_REPO" "$TB_FINAL"; printf "  ✔ %-14s -> %s\n" "mcp-server" "$TB_FINAL"
fi
if [ "$(current_appversion)" != "$TB_FINAL" ]; then
  sed -i -E "s#^appVersion:.*#appVersion: \"$TB_FINAL\"#" "$CHART"; printf "  ✔ %-14s -> %s\n" "appVersion" "$TB_FINAL"
fi

echo
echo "Done. Review the diff before committing:"
echo "  git diff -- docker-compose*.yaml deployments/helm/cb-tumblebug/"
