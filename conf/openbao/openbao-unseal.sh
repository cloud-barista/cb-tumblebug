#!/usr/bin/env bash
# ==============================================================================
# # openbao-unseal.sh — Unseal OpenBao after container restart (persistent mode)
# ==============================================================================
#
# In persistent mode, OpenBao starts in a sealed state after every restart.
# This script checks the API for the actual state and acts accordingly:
#
#   Not initialized   → guide user to run openbao-register-creds.sh
#   Already unsealed  → nothing to do
#   Sealed            → read unseal key from init output and unseal
#
# Usage:
#   ./openbao-unseal.sh
# ==============================================================================
#
# Help handling
if [[ "${1:-}" == "-h" ]] || [[ "${1:-}" == "--help" ]]; then
    echo "OpenBao Unseal Script"
    echo ""
    echo "Usage: openbao-unseal.sh [ENV_FILE] [INIT_OUTPUT]"
    echo ""
    echo "Unseals a previously initialized OpenBao instance using"
    echo "stored unseal keys and provides usage hints."
    echo ""
    echo "Arguments:"
    echo "  ENV_FILE        Path to .env file to update (default: ../.env)"
    echo "  INIT_OUTPUT     Path to initialization secrets (default: ./secrets/openbao-init.json)"
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
    echo "  1. As an argument: ./openbao-unseal.sh /path/to/.env"
    echo "  2. As an environment variable: ENV_FILE=/path/to/.env ./openbao-unseal.sh"
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

echo -e "${YELLOW}[openbao-unseal]${NC} VAULT_ADDR=${VAULT_ADDR}"

# ── Wait for OpenBao to be reachable ─────────────────────────────────

echo -n "Waiting for OpenBao to be reachable..."
for i in $(seq 1 30); do
    if curl -sf "${VAULT_ADDR}/v1/sys/seal-status" -o /dev/null 2>/dev/null; then
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

# ── Check current state via API (source of truth) ────────────────────

SEAL_STATUS=$(curl -sf "${VAULT_ADDR}/v1/sys/seal-status")
INITIALIZED=$(echo "$SEAL_STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('initialized', False))" 2>/dev/null || echo "false")
SEALED=$(echo "$SEAL_STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('sealed', True))" 2>/dev/null || echo "true")

# State: Not initialized
if [ "$INITIALIZED" = "False" ] || [ "$INITIALIZED" = "false" ]; then
    echo -e "${YELLOW}[openbao-unseal]${NC} OpenBao is not yet initialized."
    echo "  Run 'make init' from project root to initialize OpenBao,"
    echo "  unseal it, and register CSP credentials."
    echo ""
    SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
    bash "${SCRIPT_DIR}/openbao-register-creds.sh" --help
    exit 1
fi

# State: Already unsealed
if [ "$SEALED" = "False" ] || [ "$SEALED" = "false" ]; then
    echo -e "${GREEN}[openbao-unseal]${NC} OpenBao is already unsealed. Nothing to do."
    exit 0
fi

# State: Sealed — need unseal key from init output
if [ ! -f "$INIT_OUTPUT" ]; then
    echo -e "${RED}[openbao-unseal]${NC} OpenBao is sealed but ${INIT_OUTPUT} not found." >&2
    echo "  Cannot unseal without the key file." >&2
    exit 1
fi

# ── Unseal ───────────────────────────────────────────────────────────

UNSEAL_KEY=$(python3 -c "import json; print(json.load(open('${INIT_OUTPUT}'))['keys'][0])" 2>/dev/null \
    || grep -o '"keys":\["[^"]*"' "$INIT_OUTPUT" | sed 's/"keys":\["//' | sed 's/"$//')

if [ -z "$UNSEAL_KEY" ]; then
    echo -e "${RED}Error: Could not extract unseal key from ${INIT_OUTPUT}${NC}" >&2
    exit 1
fi

echo -e "${YELLOW}[openbao-unseal]${NC} Unsealing..."

UNSEAL_RESPONSE=$(curl -sf -X POST "${VAULT_ADDR}/v1/sys/unseal" \
    -H "Content-Type: application/json" \
    -d "{\"key\": \"${UNSEAL_KEY}\"}")

SEALED=$(echo "$UNSEAL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('sealed', True))" 2>/dev/null || echo "true")

if [ "$SEALED" = "False" ] || [ "$SEALED" = "false" ]; then
    echo -e "${GREEN}[openbao-unseal]${NC} OpenBao is now unsealed and ready!"
else
    echo -e "${RED}[openbao-unseal]${NC} Unseal may require additional keys." >&2
    echo "  Check status: curl ${VAULT_ADDR}/v1/sys/seal-status" >&2
    exit 1
fi
