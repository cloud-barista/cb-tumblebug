#!/bin/bash

# A wrapper to run initialization scripts with a single password prompt

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "CB-Tumblebug Initialization"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
read -s -p "Enter the password for credentials.yaml.enc: " MULTI_INIT_PWD
echo ""
export MULTI_INIT_PWD

# 1. OpenBao
if [ -f "$SCRIPT_DIR/openbao/openbao-register-creds.sh" ]; then
    OPENBAO_SH="$SCRIPT_DIR/openbao/openbao-register-creds.sh"
elif [ -f "$SCRIPT_DIR/../../openbao/openbao-register-creds.sh" ]; then
    # When executed within cm-beetle
    OPENBAO_SH="$SCRIPT_DIR/../../openbao/openbao-register-creds.sh"
else
    echo "Error: Cannot find openbao-register-creds.sh"
    exit 1
fi

echo ""
echo "Step 1. Registering credentials to OpenBao..."
chmod +x "$OPENBAO_SH" 2>/dev/null || true
bash "$OPENBAO_SH"
if [ $? -ne 0 ]; then exit 1; fi

# 2. Tumblebug
if [ -f "$SCRIPT_DIR/init.sh" ]; then
    echo ""
    echo "Step 2. Registering credentials to Tumblebug..."
    chmod +x "$SCRIPT_DIR/init.sh" 2>/dev/null || true
    bash "$SCRIPT_DIR/init.sh"
    if [ $? -ne 0 ]; then exit 1; fi
else
    echo "Error: Cannot find init.sh"
    exit 1
fi

echo ""
echo "Initialization completed successfully."
