#!/usr/bin/env bash
# ==============================================================================
# init-openbao.sh — One-time initialization for OpenBao persistent mode
# ==============================================================================
#
# This script initializes a fresh OpenBao instance, saves the unseal keys
# and root token, then automatically unseals the server.
#
# Based on mc-terrarium's init-openbao.sh, adapted for cb-tumblebug deployment.
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
#   ./init/init-openbao.sh
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

echo -e "${YELLOW}[init-openbao]${NC} VAULT_ADDR=${VAULT_ADDR}"

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
    echo -e "${YELLOW}[init-openbao]${NC} OpenBao is already initialized."
    echo "  If you need to re-initialize, destroy the volume first:"
    echo "    docker compose down -v"
    exit 0
fi

# ── Initialize ───────────────────────────────────────────────────────

echo -e "${YELLOW}[init-openbao]${NC} Initializing OpenBao (1 key share, threshold 1)..."

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

echo -e "${GREEN}[init-openbao]${NC} Initialization successful!"
echo "  Init output saved to: ${INIT_OUTPUT}"
echo ""
echo -e "  ${YELLOW}WARNING: Keep ${INIT_OUTPUT} safe — it contains your unseal key and root token.${NC}"

# ── Unseal ───────────────────────────────────────────────────────────

echo -e "${YELLOW}[init-openbao]${NC} Unsealing OpenBao..."

UNSEAL_RESPONSE=$(curl -sf -X POST "${VAULT_ADDR}/v1/sys/unseal" \
    -H "Content-Type: application/json" \
    -d "{\"key\": \"${UNSEAL_KEY}\"}")

SEALED=$(echo "$UNSEAL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('sealed', True))" 2>/dev/null || echo "true")

if [ "$SEALED" = "False" ] || [ "$SEALED" = "false" ]; then
    echo -e "${GREEN}[init-openbao]${NC} OpenBao is now unsealed and ready!"
else
    echo -e "${RED}[init-openbao]${NC} Unseal may have failed. Check: curl ${VAULT_ADDR}/v1/sys/seal-status" >&2
fi

# ── Update .env ──────────────────────────────────────────────────────

ENV_FILE="${PROJECT_DIR}/.env"

if [ -f "$ENV_FILE" ]; then
    # Update VAULT_TOKEN
    if grep -q "^VAULT_TOKEN=" "$ENV_FILE"; then
        sed -i "s|^VAULT_TOKEN=.*|VAULT_TOKEN=${ROOT_TOKEN}|" "$ENV_FILE"
    else
        echo "VAULT_TOKEN=${ROOT_TOKEN}" >> "$ENV_FILE"
    fi
    # Ensure VAULT_ADDR is present
    if ! grep -q "^VAULT_ADDR=" "$ENV_FILE"; then
        echo "VAULT_ADDR=${VAULT_ADDR}" >> "$ENV_FILE"
    fi
    echo -e "${GREEN}[init-openbao]${NC} Updated VAULT_TOKEN in .env"
else
    cat > "$ENV_FILE" <<EOF
# OpenBao / OpenTofu vault provider token
VAULT_TOKEN=${ROOT_TOKEN}

# OpenBao endpoint (for host-side usage)
VAULT_ADDR=${VAULT_ADDR}

# MC-Terrarium API credentials
TERRARIUM_API_USERNAME=default
TERRARIUM_API_PASSWORD=default
EOF
    echo -e "${GREEN}[init-openbao]${NC} Created .env with VAULT_TOKEN"
fi

# ── Enable KV v2 secret engine ───────────────────────────────────────

echo -e "${YELLOW}[init-openbao]${NC} Verifying KV v2 secret engine at secret/..."

# In persistent mode, the 'secret/' KV v2 mount is enabled by default.
# Verify it exists.
MOUNTS=$(curl -sf -H "X-Vault-Token: ${ROOT_TOKEN}" "${VAULT_ADDR}/v1/sys/mounts" 2>/dev/null || echo "{}")
if echo "$MOUNTS" | grep -q '"secret/"'; then
    echo -e "${GREEN}[init-openbao]${NC} KV v2 secret engine is available at secret/"
else
    echo -e "${YELLOW}[init-openbao]${NC} Enabling KV v2 secret engine at secret/..."
    curl -sf -X POST "${VAULT_ADDR}/v1/sys/mounts/secret" \
        -H "X-Vault-Token: ${ROOT_TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{"type": "kv", "options": {"version": "2"}}' > /dev/null
    echo -e "${GREEN}[init-openbao]${NC} KV v2 enabled at secret/"
fi

# ── Summary ──────────────────────────────────────────────────────────

echo ""
echo "============================================================"
echo "  OpenBao initialization complete!"
echo "============================================================"
echo "  Root Token : ${ROOT_TOKEN}"
echo "  Unseal Key : ${UNSEAL_KEY}"
echo "  Init File  : ${INIT_OUTPUT}"
echo ""
echo "  After container restart, run:"
echo "    ./init/unseal-openbao.sh"
echo "============================================================"
