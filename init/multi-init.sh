#!/bin/bash

# A wrapper to run initialization scripts with a single password prompt

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "CB-Tumblebug Initialization"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Skip the password prompt when a decryption key source already exists
# (MULTI_INIT_PWD env, ~/.cloud-barista/.tmp_enc_key, or non-interactive stdin)
if [ -n "$MULTI_INIT_PWD" ]; then
    echo "Using password from MULTI_INIT_PWD environment variable."
elif [ -f "$HOME/.cloud-barista/.tmp_enc_key" ]; then
    echo "Using decryption key file: ~/.cloud-barista/.tmp_enc_key"
elif [ -t 0 ]; then
    read -s -p "Enter the password for credentials.yaml.enc: " MULTI_INIT_PWD
    echo ""
else
    echo "Warning: no password source available in non-interactive mode (set MULTI_INIT_PWD or ~/.cloud-barista/.tmp_enc_key)."
fi
export MULTI_INIT_PWD

# 1. Step 1 script execution code is deprecated (to be removed) for operational simplicity:
#    CB-Tumblebug server registers credentials to OpenBao automatically during Step 2.
#
# if [ -f "$SCRIPT_DIR/openbao/openbao-register-creds.sh" ]; then
#     OPENBAO_SH="$SCRIPT_DIR/openbao/openbao-register-creds.sh"
# elif [ -f "$SCRIPT_DIR/../../openbao/openbao-register-creds.sh" ]; then
#     # When executed within cm-beetle
#     OPENBAO_SH="$SCRIPT_DIR/../../openbao/openbao-register-creds.sh"
# else
#     echo "Error: Cannot find openbao-register-creds.sh"
#     exit 1
# fi
#
# echo ""
# echo "Step 1. Registering credentials to OpenBao..."
# chmod +x "$OPENBAO_SH" 2>/dev/null || true
# bash "$OPENBAO_SH"
# if [ $? -ne 0 ]; then exit 1; fi

# 2. Tumblebug
# Extra arguments are forwarded to init.py (e.g., -y for headless runs)
if [ -f "$SCRIPT_DIR/init.sh" ]; then
    echo ""
    echo "Step 2. Registering credentials to Tumblebug..."
    chmod +x "$SCRIPT_DIR/init.sh" 2>/dev/null || true
    bash "$SCRIPT_DIR/init.sh" "$@"
    if [ $? -ne 0 ]; then exit 1; fi
else
    echo "Error: Cannot find init.sh"
    exit 1
fi

echo ""
echo "Initialization completed successfully."
