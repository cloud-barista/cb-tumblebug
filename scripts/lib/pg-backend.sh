#!/bin/bash

# Shared PostgreSQL access backend for restore-assets.sh / backup-assets.sh
# Backends: docker (compose container), kubectl (k8s pod), direct (TCP via local pg tools), ssh (remote docker/kubectl over SSH)
#
# Selection: ASSETS_PG_BACKEND=docker|kubectl|direct|ssh (default: auto-detect)
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
# ssh backend:
#   TB_SSH_TARGET          (user@host, e.g. cb-user@52.78.93.192)
#   TB_SSH_KEY             (optional SSH private key path)
#   TB_SSH_PORT            (optional SSH port, default: 22)

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

# --- ssh backend (remote docker or kubectl over SSH) --------------------------
# Env:
#   TB_SSH_TARGET  (required for ssh backend, e.g. cb-user@52.78.93.192)
#   TB_SSH_KEY     (optional path to SSH private key)
#   TB_SSH_PORT    (optional SSH port, default: 22)

_pg_ssh() {
    local opts=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=12 -o ServerAliveInterval=30 -o ServerAliveCountMax=3)
    [ -n "$TB_SSH_PORT" ] && opts+=(-p "$TB_SSH_PORT")
    [ -n "$TB_SSH_KEY" ]  && opts+=(-i "$TB_SSH_KEY")
    ssh "${opts[@]}" "$TB_SSH_TARGET" "$@"
}

_pg_ssh_check() {
    [ -n "$TB_SSH_TARGET" ] || return 1
    command -v ssh >/dev/null 2>&1 || return 1

    local probe
    probe=$(_pg_ssh "
        for d in 'docker' 'sudo -n docker'; do
            names=\$({ \$d ps --format '{{.Names}}' 2>/dev/null || true; })
            if [ -n \"\$names\" ]; then
                for c in '${TB_POSTGRES_CONTAINER:-}' 'cb-tumblebug-postgres' 'mc-infra-manager-postgres'; do
                    [ -n \"\$c\" ] || continue
                    if printf '%s\n' \"\$names\" | grep -Fxq \"\$c\"; then
                        echo \"docker|\$d|\$c\"
                        exit 0
                    fi
                done
            fi
        done
        for k in 'kubectl' 'sudo -n kubectl'; do
            found=\$({ \$k get pod -A -l '${TB_POSTGRES_POD_SELECTOR:-app=cb-tumblebug-postgres}' -o jsonpath='{.items[0].metadata.namespace}|{.items[0].metadata.name}' 2>/dev/null || true; })
            if [ -n \"\$found\" ] && [ \"\$found\" != '|' ]; then
                echo \"kubectl|\$k|\$found\"
                exit 0
            fi
        done
    " 2>/dev/null || true)

    [ -n "$probe" ] || return 1
    IFS='|' read -r PG_SSH_MODE PG_SSH_CLI PG_SSH_ARG1 PG_SSH_ARG2 <<< "$probe"
    case "$PG_SSH_MODE" in
        docker)
            PG_CONTAINER="$PG_SSH_ARG1"
            [ -n "$PG_CONTAINER" ] ;;
        kubectl)
            PG_K8S_NS="$PG_SSH_ARG1"
            PG_POD="$PG_SSH_ARG2"
            [ -n "$PG_K8S_NS" ] && [ -n "$PG_POD" ] ;;
        *) return 1 ;;
    esac
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
        ssh)
            _pg_ssh_check || { echo "Error: ssh backend unavailable (cannot reach PostgreSQL container/pod on ${TB_SSH_TARGET:-<unset>})." >&2; exit 1; }
            PG_BACKEND=ssh ;;
        auto)
            local tb_host="${TUMBLEBUG_SERVER%%:*}"
            if [ -n "$TB_SSH_TARGET" ] && _pg_ssh_check; then
                PG_BACKEND=ssh
            elif [ -n "$tb_host" ] && [ "$tb_host" != "localhost" ] && [ "$tb_host" != "127.0.0.1" ]; then
                if [ -n "$TB_POSTGRES_ENDPOINT" ] && _pg_direct_check; then
                    PG_BACKEND=direct
                else
                    echo "Error: TUMBLEBUG_SERVER points to remote host '$tb_host', refusing to overwrite local PostgreSQL database." >&2
                    echo "  Use 'make init HOST=$tb_host' (SSH tunnel + remote DB restore) or set TB_POSTGRES_ENDPOINT." >&2
                    exit 1
                fi
            elif _pg_docker_check; then PG_BACKEND=docker
            elif _pg_kubectl_check; then PG_BACKEND=kubectl
            elif [ -n "$TB_POSTGRES_ENDPOINT" ] && _pg_direct_check; then PG_BACKEND=direct
            else
                echo "Error: no PostgreSQL access backend available." >&2
                echo "  - docker:  start the compose stack (make up) or set TB_POSTGRES_CONTAINER" >&2
                echo "  - kubectl: set TB_K8S_NAMESPACE / TB_POSTGRES_POD(_SELECTOR)" >&2
                echo "  - ssh:     run 'make init HOST=<ip>' or set ASSETS_PG_BACKEND=ssh TB_SSH_TARGET=user@host" >&2
                echo "  - direct:  set TB_POSTGRES_ENDPOINT (and install postgresql client tools)" >&2
                exit 1
            fi ;;
        *)
            echo "Error: unknown ASSETS_PG_BACKEND '$ASSETS_PG_BACKEND' (docker|kubectl|direct|ssh)" >&2
            exit 1 ;;
    esac
}

pg_backend_describe() {
    case "$PG_BACKEND" in
        docker)  echo "docker (container: $PG_CONTAINER)" ;;
        kubectl) echo "kubectl (pod: $PG_K8S_NS/$PG_POD)" ;;
        direct)  echo "direct (endpoint: $PG_HOST:$PG_PORT)" ;;
        ssh)
            if [ "$PG_SSH_MODE" = "docker" ]; then
                echo "ssh ($TB_SSH_TARGET -> docker:$PG_CONTAINER)"
            else
                echo "ssh ($TB_SSH_TARGET -> kubectl:$PG_K8S_NS/$PG_POD)"
            fi ;;
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
        ssh)
            if [ "$PG_SSH_MODE" = "docker" ]; then
                printf '%s\n' "$sql" | _pg_ssh "$PG_SSH_CLI exec -i $PG_CONTAINER psql -U $PG_USER -d $db"
            else
                printf '%s\n' "$sql" | _pg_ssh "$PG_SSH_CLI exec -i -n $PG_K8S_NS $PG_POD -- psql -U $PG_USER -d $db"
            fi ;;
    esac
}

# pg_psql_value <db> <sql>  — tuples-only, unaligned: clean scalar/line output for scripts
pg_psql_value() {
    local db="$1" sql="$2"
    case "$PG_BACKEND" in
        docker)  docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$db" -tAc "$sql" ;;
        kubectl) kubectl exec -n "$PG_K8S_NS" "$PG_POD" -- psql -U "$PG_USER" -d "$db" -tAc "$sql" ;;
        direct)  psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" -tAc "$sql" ;;
        ssh)
            if [ "$PG_SSH_MODE" = "docker" ]; then
                printf '%s\n' "$sql" | _pg_ssh "$PG_SSH_CLI exec -i $PG_CONTAINER psql -U $PG_USER -d $db -tA"
            else
                printf '%s\n' "$sql" | _pg_ssh "$PG_SSH_CLI exec -i -n $PG_K8S_NS $PG_POD -- psql -U $PG_USER -d $db -tA"
            fi ;;
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
        ssh)
            local exec_prefix
            if [ "$PG_SSH_MODE" = "docker" ]; then
                exec_prefix="$PG_SSH_CLI exec -i $PG_CONTAINER"
            else
                exec_prefix="$PG_SSH_CLI exec -i -n $PG_K8S_NS $PG_POD --"
            fi
            _pg_ssh "$exec_prefix sh -c 'cat > $remote'" < "$dump"
            _pg_ssh "$exec_prefix pg_restore -U $PG_USER -d $db --data-only -v $remote"
            local rc=$?
            _pg_ssh "$exec_prefix rm -f $remote"
            return $rc ;;
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
        ssh)
            local exec_prefix
            if [ "$PG_SSH_MODE" = "docker" ]; then
                exec_prefix="$PG_SSH_CLI exec -i $PG_CONTAINER"
            else
                exec_prefix="$PG_SSH_CLI exec -i -n $PG_K8S_NS $PG_POD --"
            fi
            _pg_ssh "$exec_prefix pg_dump -U $PG_USER -d $db -F c --data-only -f $remote"
            _pg_ssh "$exec_prefix cat $remote" > "$out"
            local rc=$?
            _pg_ssh "$exec_prefix rm -f $remote"
            return $rc ;;
    esac
}
