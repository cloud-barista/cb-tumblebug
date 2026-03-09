#!/usr/bin/env bash
# ==============================================================================
# unseal-openbao.sh — Unseal OpenBao after container restart (persistent mode)
# ==============================================================================
#
# In persistent mode, OpenBao starts in a sealed state after every restart.
# This script checks the API for the actual state and acts accordingly:
#
#   Not initialized  -> guide user to run init.sh
#   Already unsealed  -> nothing to do
#   Sealed            -> read unseal key from init output and unseal
#
# Based on mc-terrarium's unseal-openbao.sh, adapted for cb-tumblebug deployment.
#
# Usage:
#   ./init/unseal-openbao.sh
#
# ==============================================================================

set -euo pipefail

# Resolve project root (parent of this script's directory)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_DIR=$(cd "$SCRIPT_DIR/.." && pwd)

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
INIT_OUTPUT="${PROJECT_DIR}/secrets/openbao-init.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}[unseal-openbao]${NC} VAULT_ADDR=${VAULT_ADDR}"

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
    echo -e "${YELLOW}[unseal-openbao]${NC} OpenBao is not yet initialized."
    echo "  Run 'make init-openbao' (or './init/init-openbao.sh') to initialize."
    exit 1
fi

# State: Already unsealed
if [ "$SEALED" = "False" ] || [ "$SEALED" = "false" ]; then
    echo -e "${GREEN}[unseal-openbao]${NC} OpenBao is already unsealed. Nothing to do."
    exit 0
fi

# State: Sealed — need unseal key from init output
if [ ! -f "$INIT_OUTPUT" ]; then
    echo -e "${RED}[unseal-openbao]${NC} OpenBao is sealed but ${INIT_OUTPUT} not found." >&2
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

echo -e "${YELLOW}[unseal-openbao]${NC} Unsealing..."

UNSEAL_RESPONSE=$(curl -sf -X POST "${VAULT_ADDR}/v1/sys/unseal" \
    -H "Content-Type: application/json" \
    -d "{\"key\": \"${UNSEAL_KEY}\"}")

SEALED=$(echo "$UNSEAL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('sealed', True))" 2>/dev/null || echo "true")

if [ "$SEALED" = "False" ] || [ "$SEALED" = "false" ]; then
    echo -e "${GREEN}[unseal-openbao]${NC} OpenBao is now unsealed and ready!"
else
    echo -e "${RED}[unseal-openbao]${NC} Unseal may require additional keys." >&2
    echo "  Check status: curl ${VAULT_ADDR}/v1/sys/seal-status" >&2
    exit 1
fi
