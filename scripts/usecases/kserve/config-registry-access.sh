#!/bin/bash

# Registry Access Configuration (run on ALL cluster nodes)
# Allows containerd to pull from the plain-HTTP private registry (localhost:30500).
# Only edits the registry config_path line and adds a hosts.toml drop-in, so
# existing containerd settings (e.g., nvidia runtime) are preserved.
# Note: restarts containerd; avoid running while an image pull is in progress.

set -e

REGISTRY_HOST="localhost:30500"

while [[ $# -gt 0 ]]; do
    case $1 in
        --registry) REGISTRY_HOST="$2"; shift 2 ;;
        *) echo "Usage: $0 [--registry localhost:30500]"; exit 1 ;;
    esac
done

echo "==== Registry Access Setup (${REGISTRY_HOST}) ===="

CHANGED=false

# Enable certs.d drop-in directory support in containerd
if grep -q 'config_path = ""' /etc/containerd/config.toml 2>/dev/null; then
    sudo sed -i 's|config_path = ""|config_path = "/etc/containerd/certs.d"|' /etc/containerd/config.toml
    CHANGED=true
fi

# Plain-HTTP host config for the registry
HOSTS_DIR="/etc/containerd/certs.d/${REGISTRY_HOST}"
if [ ! -f "${HOSTS_DIR}/hosts.toml" ]; then
    sudo mkdir -p "${HOSTS_DIR}"
    cat <<EOF | sudo tee "${HOSTS_DIR}/hosts.toml" > /dev/null
server = "http://${REGISTRY_HOST}"

[host."http://${REGISTRY_HOST}"]
  capabilities = ["pull", "resolve", "push"]
  skip_verify = true
EOF
    CHANGED=true
fi

if [ "$CHANGED" = true ]; then
    sudo systemctl restart containerd
    echo "  ✓ containerd configured and restarted"
else
    echo "  ✓ Already configured (no restart needed)"
fi

echo ""
echo "This node can now pull images referenced as ${REGISTRY_HOST}/<image>:<tag>"
exit 0
