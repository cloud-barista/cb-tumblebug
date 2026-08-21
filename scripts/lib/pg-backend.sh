#!/bin/bash

# Shared PostgreSQL access backend for restore-assets.sh / backup-assets.sh
# Backends: docker (compose container), kubectl (k8s pod), direct (TCP via local pg tools)
#
# Selection: ASSETS_PG_BACKEND=docker|kubectl|direct (default: auto-detect in that order)
#
# Common env:
#   TB_POSTGRES_USER       (default: tumblebug)
#   TB_POSTGRES_DATABASE   (default: tumblebug)
# docker backend:
#   TB_POSTGRES_CONTAINER  (default: auto-detect cb-tumblebug-postgres | mc-infra-manager-postgres)
# kubectl backend:
#   TB_K8S_NAMESPACE       (default: default)
#   TB_POSTGRES_POD        (default: auto-detect via TB_POSTGRES_POD_SELECTOR)
#   TB_POSTGRES_POD_SELECTOR (default: app=cb-tumblebug-postgres)
# direct backend:
#   TB_POSTGRES_ENDPOINT   (host:port, default: localhost:5432)
#   TB_POSTGRES_PASSWORD   (default: tumblebug)

PG_USER="${TB_POSTGRES_USER:-tumblebug}"
PG_DB="${TB_POSTGRES_DATABASE:-tumblebug}"

pg_validate_identifier() {
    if ! echo "$1" | grep -Eq '^[a-zA-Z_][a-zA-Z0-9_]*$'; then
        echo "Error: Invalid identifier '$1' — only letters, digits, and underscores are allowed." >&2
        exit 1
    fi
}
pg_validate_identifier "$PG_USER"
pg_validate_identifier "$PG_DB"

# --- docker backend -----------------------------------------------------------

_pg_docker_detect_container() {
    if [ -n "$TB_POSTGRES_CONTAINER" ]; then
        PG_CONTAINER="$TB_POSTGRES_CONTAINER"
    elif docker ps --format "{{.Names}}" 2>/dev/null | grep -Fxq "cb-tumblebug-postgres"; then
        PG_CONTAINER="cb-tumblebug-postgres"
    elif docker ps --format "{{.Names}}" 2>/dev/null | grep -Fxq "mc-infra-manager-postgres"; then
        PG_CONTAINER="mc-infra-manager-postgres"
    else
        PG_CONTAINER=""
    fi
    [ -n "$PG_CONTAINER" ]
}

_pg_docker_check() {
    command -v docker >/dev/null 2>&1 || return 1
    _pg_docker_detect_container || return 1
    docker ps --format "{{.Names}}" | grep -Fxq "$PG_CONTAINER"
}

# --- kubectl backend ----------------------------------------------------------

_pg_kubectl_detect_pod() {
    PG_K8S_NS="${TB_K8S_NAMESPACE:-default}"
    if [ -n "$TB_POSTGRES_POD" ]; then
        PG_POD="$TB_POSTGRES_POD"
        [ -n "$PG_POD" ]
        return
    fi

    local selector="${TB_POSTGRES_POD_SELECTOR:-app=cb-tumblebug-postgres}"
    PG_POD=$(kubectl get pod -n "$PG_K8S_NS" -l "$selector" \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

    # The Helm chart installs into its own namespace (cb-tumblebug), not "default", so a
    # bare `make backup-assets` used to report "no backend available" while the pod was
    # running. Look across namespaces before giving up.
    if [ -z "$PG_POD" ]; then
        local found
        found=$(kubectl get pod -A -l "$selector" \
            -o jsonpath='{.items[0].metadata.namespace} {.items[0].metadata.name}' 2>/dev/null || true)
        if [ -n "$found" ]; then
            PG_K8S_NS="${found%% *}"
            PG_POD="${found##* }"
        fi
    fi
    [ -n "$PG_POD" ]
}

_pg_kubectl_check() {
    command -v kubectl >/dev/null 2>&1 || return 1
    _pg_kubectl_detect_pod || return 1
    kubectl get pod -n "$PG_K8S_NS" "$PG_POD" >/dev/null 2>&1
}

# --- direct backend -----------------------------------------------------------

_pg_direct_check() {
    command -v psql >/dev/null 2>&1 || return 1
    PG_HOST="${TB_POSTGRES_ENDPOINT%%:*}"
    PG_PORT="${TB_POSTGRES_ENDPOINT##*:}"
    PG_HOST="${PG_HOST:-localhost}"
    [ "$PG_PORT" = "$PG_HOST" ] && PG_PORT=5432
    export PGPASSWORD="${TB_POSTGRES_PASSWORD:-tumblebug}"
    psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c "SELECT 1;" >/dev/null 2>&1
}

# --- backend selection --------------------------------------------------------

# Sets PG_BACKEND and backend-specific vars; exits with guidance on failure
pg_backend_init() {
    case "${ASSETS_PG_BACKEND:-auto}" in
        docker)
            _pg_docker_check || { echo "Error: docker backend unavailable (no running PostgreSQL container; set TB_POSTGRES_CONTAINER)." >&2; exit 1; }
            PG_BACKEND=docker ;;
        kubectl)
            _pg_kubectl_check || { echo "Error: kubectl backend unavailable (no pod matched; set TB_POSTGRES_POD or TB_POSTGRES_POD_SELECTOR/TB_K8S_NAMESPACE)." >&2; exit 1; }
            PG_BACKEND=kubectl ;;
        direct)
            [ -n "$TB_POSTGRES_ENDPOINT" ] || TB_POSTGRES_ENDPOINT="localhost:5432"
            _pg_direct_check || { echo "Error: direct backend unavailable (psql missing or cannot reach $TB_POSTGRES_ENDPOINT)." >&2; exit 1; }
            PG_BACKEND=direct ;;
        auto)
            if _pg_docker_check; then PG_BACKEND=docker
            elif _pg_kubectl_check; then PG_BACKEND=kubectl
            elif [ -n "$TB_POSTGRES_ENDPOINT" ] && _pg_direct_check; then PG_BACKEND=direct
            else
                echo "Error: no PostgreSQL access backend available." >&2
                echo "  - docker:  start the compose stack (make up) or set TB_POSTGRES_CONTAINER" >&2
                echo "  - kubectl: set TB_K8S_NAMESPACE / TB_POSTGRES_POD(_SELECTOR)" >&2
                echo "  - direct:  set TB_POSTGRES_ENDPOINT (and install postgresql client tools)" >&2
                exit 1
            fi ;;
        *)
            echo "Error: unknown ASSETS_PG_BACKEND '$ASSETS_PG_BACKEND' (docker|kubectl|direct)" >&2
            exit 1 ;;
    esac
}

pg_backend_describe() {
    case "$PG_BACKEND" in
        docker)  echo "docker (container: $PG_CONTAINER)" ;;
        kubectl) echo "kubectl (pod: $PG_K8S_NS/$PG_POD)" ;;
        direct)  echo "direct (endpoint: $PG_HOST:$PG_PORT)" ;;
    esac
}

# --- primitives ---------------------------------------------------------------

# pg_psql <db> <sql>
pg_psql() {
    local db="$1" sql="$2"
    case "$PG_BACKEND" in
        docker)  docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$db" -c "$sql" ;;
        kubectl) kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- psql -U "$PG_USER" -d "$db" -c "$sql" ;;
        direct)  psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" -c "$sql" ;;
    esac
}

# pg_psql_value <db> <sql>  — tuples-only, unaligned: clean scalar/line output for scripts
pg_psql_value() {
    local db="$1" sql="$2"
    case "$PG_BACKEND" in
        docker)  docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$db" -tAc "$sql" ;;
        kubectl) kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- psql -U "$PG_USER" -d "$db" -tAc "$sql" ;;
        direct)  psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" -tAc "$sql" ;;
    esac
}

# pg_restore_file <local-dump-file> <db>  (custom-format, data-only, not gzipped)
# Data-only so the schema always comes from the app's AutoMigrate, never from the
# (possibly older) dump — the target tables must already exist. Restore clears the
# target data first (see restore-assets.sh), so this only loads rows.
pg_restore_file() {
    local dump="$1" db="$2" remote="/tmp/tb_restore_$$.dump"
    case "$PG_BACKEND" in
        docker)
            docker cp "$dump" "$PG_CONTAINER:$remote"
            docker exec "$PG_CONTAINER" pg_restore -U "$PG_USER" -d "$db" --data-only -v "$remote"
            local rc=$?
            docker exec "$PG_CONTAINER" rm -f "$remote"
            return $rc ;;
        kubectl)
            kubectl cp "$dump" "$PG_K8S_NS/$PG_POD:$remote"
            kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- pg_restore -U "$PG_USER" -d "$db" --data-only -v "$remote"
            local rc=$?
            kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- rm -f "$remote"
            return $rc ;;
        direct)
            pg_restore -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" --data-only -v "$dump" ;;
    esac
}

# pg_dump_file <db> <local-out-file>  (custom-format, data-only)
# Data-only: the dump carries rows, not schema — schema is owned by the app's
# AutoMigrate. This keeps the dump schema-version-agnostic (see pg_restore_file).
pg_dump_file() {
    local db="$1" out="$2" remote="/tmp/tb_backup_$$.dump"
    case "$PG_BACKEND" in
        docker)
            docker exec "$PG_CONTAINER" pg_dump -U "$PG_USER" -d "$db" -F c --data-only -f "$remote"
            docker cp "$PG_CONTAINER:$remote" "$out"
            local rc=$?
            docker exec "$PG_CONTAINER" rm -f "$remote"
            return $rc ;;
        kubectl)
            kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- pg_dump -U "$PG_USER" -d "$db" -F c --data-only -f "$remote"
            kubectl cp "$PG_K8S_NS/$PG_POD:$remote" "$out"
            local rc=$?
            kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- rm -f "$remote"
            return $rc ;;
        direct)
            pg_dump -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" -F c --data-only -f "$out" ;;
    esac
}
