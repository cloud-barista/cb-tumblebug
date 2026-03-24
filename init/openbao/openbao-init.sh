#!/usr/bin/env bash
# ==============================================================================
# # openbao-init.sh — One-time initialization for OpenBao persistent mode
# ==============================================================================
#
# This script initializes a fresh OpenBao instance, saves the unseal keys
# and root token, then automatically unseals the server.
#
# Prerequisites:
#   - OpenBao container must be running in persistent mode (sealed state)
#   - VAULT_ADDR must be reachable from the host (OpenBao endpoint)
#
# Output:
#   - secrets/openbao-init.json  (unseal keys + root token — keep this safe!)
#   - .env updated with VAULT_TOKEN
#
# Usage:
#   ./openbao-init.sh
#
# ==============================================================================

#
# Help handling
if [[ "${1:-}" == "-h" ]] || [[ "${1:-}" == "--help" ]]; then
    echo "OpenBao Initialization Script"
    echo ""
    echo "Usage: openbao-init.sh [ENV_FILE] [INIT_OUTPUT]"
    echo ""
    echo "Initializes a fresh OpenBao instance, saves the unseal keys"
    echo "and root token, then automatically unseals the server."
    echo ""
    echo "Arguments:"
    echo "  ENV_FILE        Path to .env file to update with VAULT_TOKEN (default: ../.env)"
    echo "  INIT_OUTPUT     Path to save initialization secrets (default: ./secrets/openbao-init.json)"
    exit 0
fi

set -euo pipefail

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color


if [ -z "${ENV_FILE:-}" ]; then
    if [ "$#" -ge 1 ]; then
        ENV_FILE="$1"
    else
        # ── Robust Root Discovery ───────────────────────────────────────────
        # Try to find the project root using git, fallback to searching for go.mod or .git upwards
        PROJECT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
        
        # If git root is found, check typical locations
        if [ -f "${PROJECT_ROOT}/.env" ]; then
            ENV_FILE="${PROJECT_ROOT}/.env"
        elif [ -f "${PROJECT_ROOT}/deployments/docker-compose/.env" ]; then
            ENV_FILE="${PROJECT_ROOT}/deployments/docker-compose/.env"
        fi
    fi
fi

# ── Validate ENV_FILE ───────────────────────────────────────────────
if [ -z "${ENV_FILE:-}" ] || [ ! -f "${ENV_FILE}" ]; then
    echo -e "${RED}Error: .env file not found.${NC}"
    echo "Please provide the path to your .env file:"
    echo "  1. As an argument: ./openbao-init.sh /path/to/.env"
    echo "  2. As an environment variable: ENV_FILE=/path/to/.env ./openbao-init.sh"
    echo ""
    echo "Typically, it is located at the project root or in deployments/docker-compose/.env"
    exit 1
fi

# Define INIT_OUTPUT relative to this script's directory for portability
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
if [ -z "${INIT_OUTPUT:-}" ]; then
    if [ "$#" -ge 2 ]; then
        INIT_OUTPUT="$2"
    else
        INIT_OUTPUT="${SCRIPT_DIR}/secrets/openbao-init.json"
    fi
fi

echo -e "${YELLOW}[openbao-init]${NC} VAULT_ADDR=${VAULT_ADDR}"

# ── Pre-flight checks ───────────────────────────────────────────────

# Wait for OpenBao to be reachable
echo -n "Waiting for OpenBao to be reachable..."
for i in $(seq 1 30); do
    if curl -sf "${VAULT_ADDR}/v1/sys/health" -o /dev/null 2>/dev/null || \
       curl -sf "${VAULT_ADDR}/v1/sys/seal-status" -o /dev/null 2>/dev/null; then
        echo -e " ${GREEN}OK${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ "$i" -eq 30 ]; then
        echo -e " ${RED}FAILED${NC}"
        echo "Error: OpenBao is not reachable at ${VAULT_ADDR}" >&2
        exit 1
    fi
done

# Check if already initialized
INIT_STATUS=$(curl -sf "${VAULT_ADDR}/v1/sys/seal-status" | grep -o '"initialized":[a-z]*' | cut -d: -f2)
if [ "$INIT_STATUS" = "true" ]; then
    echo -e "${YELLOW}[openbao-init]${NC} OpenBao is already initialized."
    echo "  If you need to re-initialize, destroy the volume first:"
    echo "    docker compose down -v"
    exit 0
fi

# ── Initialize ───────────────────────────────────────────────────────

echo -e "${YELLOW}[openbao-init]${NC} Initializing OpenBao (1 key share, threshold 1)..."

# Initialize with 1 key share and threshold 1 for simplicity.
# For production, increase key_shares and key_threshold.
INIT_RESPONSE=$(curl -sf -X POST "${VAULT_ADDR}/v1/sys/init" \
    -H "Content-Type: application/json" \
    -d '{"secret_shares": 1, "secret_threshold": 1}')

if [ -z "$INIT_RESPONSE" ]; then
    echo -e "${RED}Error: Initialization failed — empty response${NC}" >&2
    exit 1
fi

# Save init response (contains unseal keys + root token)
mkdir -p "$(dirname "$INIT_OUTPUT")"
echo "$INIT_RESPONSE" | python3 -m json.tool > "$INIT_OUTPUT" 2>/dev/null \
    || echo "$INIT_RESPONSE" > "$INIT_OUTPUT"
chmod 600 "$INIT_OUTPUT"

# Extract values
UNSEAL_KEY=$(echo "$INIT_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['keys'][0])" 2>/dev/null \
    || echo "$INIT_RESPONSE" | grep -o '"keys":\["[^"]*"' | sed 's/"keys":\["//' | sed 's/"$//')
ROOT_TOKEN=$(echo "$INIT_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['root_token'])" 2>/dev/null \
    || echo "$INIT_RESPONSE" | grep -o '"root_token":"[^"]*"' | sed 's/"root_token":"//' | sed 's/"$//')

echo -e "${GREEN}[openbao-init]${NC} Initialization successful!"
echo "  Init output saved to: ${INIT_OUTPUT}"
echo ""
echo -e "  ${YELLOW}⚠ IMPORTANT: Keep ${INIT_OUTPUT} safe — it contains your unseal key and root token.${NC}"

# ── Unseal ───────────────────────────────────────────────────────────

echo -e "${YELLOW}[openbao-init]${NC} Unsealing OpenBao..."

UNSEAL_RESPONSE=$(curl -sf -X POST "${VAULT_ADDR}/v1/sys/unseal" \
    -H "Content-Type: application/json" \
    -d "{\"key\": \"${UNSEAL_KEY}\"}")

SEALED=$(echo "$UNSEAL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('sealed', True))" 2>/dev/null || echo "true")

if [ "$SEALED" = "False" ] || [ "$SEALED" = "false" ]; then
    echo -e "${GREEN}[openbao-init]${NC} OpenBao is now unsealed and ready!"
else
    echo -e "${RED}[openbao-init]${NC} Unseal may have failed. Check: curl ${VAULT_ADDR}/v1/sys/seal-status" >&2
fi

# ── Update .env ──────────────────────────────────────────────────────
# Write VAULT_TOKEN (unified token for OpenTofu vault provider, host API calls,
# and docker-compose.yaml injection).
# Note: BAO_TOKEN was replaced by VAULT_TOKEN for OpenTofu compatibility.

if [ -f "${ENV_FILE}" ]; then
    # Update VAULT_TOKEN
    if grep -q "^VAULT_TOKEN=" "${ENV_FILE}"; then
        sed -i "s|^VAULT_TOKEN=.*|VAULT_TOKEN=${ROOT_TOKEN}|" "${ENV_FILE}"
    else
        echo "VAULT_TOKEN=${ROOT_TOKEN}" >> "${ENV_FILE}"
    fi
    # Ensure VAULT_ADDR is present
    if ! grep -q "^VAULT_ADDR=" "${ENV_FILE}"; then
        echo "VAULT_ADDR=${VAULT_ADDR}" >> "${ENV_FILE}"
    fi
    echo -e "${GREEN}[openbao-init]${NC} Updated VAULT_TOKEN in ${ENV_FILE}"
else
    mkdir -p "$(dirname "${ENV_FILE}")"
    cat > "${ENV_FILE}" <<EOF
# OpenBao / OpenTofu vault provider token
VAULT_TOKEN=${ROOT_TOKEN}

# OpenBao endpoint (for host-side usage)
VAULT_ADDR=${VAULT_ADDR}
EOF
    echo -e "${GREEN}[openbao-init]${NC} Created ${ENV_FILE} with VAULT_TOKEN"
fi

# ── Enable KV v2 secret engine ───────────────────────────────────────

echo -e "${YELLOW}[openbao-init]${NC} Verifying KV v2 secret engine at secret/..."

# In persistent mode, the 'secret/' KV v2 mount is enabled by default.
# Verify it exists.
MOUNTS=$(curl -sf -H "X-Vault-Token: ${ROOT_TOKEN}" "${VAULT_ADDR}/v1/sys/mounts" 2>/dev/null || echo "{}")
if echo "$MOUNTS" | grep -q '"secret/"'; then
    echo -e "${GREEN}[openbao-init]${NC} KV v2 secret engine is available at secret/"
else
    echo -e "${YELLOW}[openbao-init]${NC} Enabling KV v2 secret engine at secret/..."
    curl -sf -X POST "${VAULT_ADDR}/v1/sys/mounts/secret" \
        -H "X-Vault-Token: ${ROOT_TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{"type": "kv", "options": {"version": "2"}}' > /dev/null
    echo -e "${GREEN}[openbao-init]${NC} KV v2 enabled at secret/"
fi

# ── Summary ──────────────────────────────────────────────────────────

echo ""
echo "============================================================"
echo "  OpenBao initialization complete!"
echo "============================================================"
echo "  Init File  : ${INIT_OUTPUT}"
echo "  (Please keep this file safe. It contains your Root Token and Unseal Key)"
echo ""
echo "  After container restart, run:"
echo "    ./openbao-unseal.sh"
echo "=================================================================="
