#!/bin/bash
#
# MC-Terrarium Initialization Script
#
# Orchestrates the full initialization sequence:
#   1. Initialize OpenBao (one-time: generate unseal key + root token)
#   2. Unseal OpenBao (if already initialized but sealed after restart)
#   3. Register CSP credentials from ~/.cloud-barista/credentials.yaml.enc
#
# This script is designed to be called both standalone and from external
# projects (e.g., cb-tumblebug) for unified Cloud-Barista initialization.
#
# Usage:
#   ./openbao-register-creds.sh                        # Full registration (interactive)
#   ./openbao-register-creds.sh -y                     # Non-interactive (auto-confirm)
#   ./openbao-register-creds.sh --key-file PATH        # Run with key file
#
# Prerequisites:
#   - OpenBao container running
#     docker compose up -d openbao
#   - ~/.cloud-barista/credentials.yaml.enc (for credential registration)
#
# Environment Variables:
#   VAULT_ADDR          OpenBao endpoint (default: http://localhost:8200)
#   VAULT_TOKEN       OpenBao root token (auto-set by openbao-init.sh)

# Check for help option
if [[ "${1:-}" == "-h" ]] || [[ "${1:-}" == "--help" ]]; then
    echo "MC-Terrarium Initialization Script"
    echo ""
    echo "Usage: deployments/docker-compose/openbao/openbao-register-creds.sh [OPTIONS]"
    echo ""
    echo "This script initializes MC-Terrarium by setting up OpenBao"
    echo "and registering CSP credentials from Cloud-Barista shared credentials."
    echo ""
    echo "Options:"
    echo "  -h, --help                    Show this help message"
    echo "  -y, --yes                     Automatically answer yes to prompts"
    echo "  --key-file PATH               Path to decryption key file"
    echo ""
    echo "Examples:"
    echo "  ./openbao-register-creds.sh                     # Run all steps (default)"
    echo "  ./openbao-register-creds.sh -y                  # Run without confirmation"
    echo "  ./openbao-register-creds.sh --key-file PATH     # Run with key file"
    echo ""
    echo "Environment Variables:"
    echo "  VAULT_ADDR              OpenBao endpoint (default: http://localhost:8200)"
    echo "  VAULT_TOKEN             OpenBao root token (auto-set by openbao-init.sh)"
    echo ""
    exit 0
fi

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Change to the script directory
pushd "$SCRIPT_DIR" > /dev/null

# Python version check
REQUIRED_VERSION="3.8.0"

PYTHON_VERSION=$(python3 --version | cut -d' ' -f2)
echo "Detected Python version: $PYTHON_VERSION"
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)
PYTHON_PATCH=$(echo $PYTHON_VERSION | cut -d. -f3)

REQUIRED_MAJOR=3
REQUIRED_MINOR=8
REQUIRED_PATCH=0

if [[ $PYTHON_MAJOR -gt $REQUIRED_MAJOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -gt $REQUIRED_MINOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -eq $REQUIRED_MINOR && $PYTHON_PATCH -ge $REQUIRED_PATCH ]]; then
    echo "Python version is sufficient."
else
    echo "This script requires Python $REQUIRED_MAJOR.$REQUIRED_MINOR.$REQUIRED_PATCH or higher."
    echo "  * Upgrade command by uv: uv python install $REQUIRED_MAJOR.$REQUIRED_MINOR"
    exit 1
fi

# Ensure uv is installed
echo
echo "Checking for uv..."
if ! command -v uv &> /dev/null; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "uv is not installed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "uv is an extremely fast Python package installer and resolver."
    echo "It's required for this project to manage Python dependencies efficiently."
    echo ""
    echo "You can install it using one of these methods:"
    echo ""
    echo "Option 1: Direct install (recommended)"
    echo ""
    echo -e "\033[4;94mcurl -LsSf https://astral.sh/uv/install.sh | sh\033[0m"
    echo ""
    echo "Option 2: Visit the installation page"
    echo ""
    echo -e "\033[4;94mhttps://github.com/astral-sh/uv#installation\033[0m"
    echo ""
    echo "After installation, reload your shell environment with:"
    echo ""
    echo -e "\033[4;94msource ~/.bashrc\033[0m"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 1
fi

# Record start time
START_TIME=$(date +%s)

echo
uv run openbao-register-creds.py "$@"

# Elapsed time
END_TIME=$(date +%s)
ELAPSED_TIME=$((END_TIME - START_TIME))
ELAPSED_MINUTES=$((ELAPSED_TIME / 60))
echo "Elapsed time: ${ELAPSED_TIME}s ($ELAPSED_MINUTES minutes)"

echo
echo "Cleaning up the venv and uv.lock files..."
rm -rf .venv
rm -rf uv.lock

echo
echo "Environment cleanup complete."

# Return to the original directory
popd > /dev/null
