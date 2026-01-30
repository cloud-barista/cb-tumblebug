#!/bin/bash

# WireGuard Mesh VPN Setup Script for Multi-Cloud Kubernetes
# Creates a full mesh VPN network between nodes
#
# Usage:
#   On each node, run with all nodes' information:
#   ./setup-wireguard-mesh.sh \
#     --nodes "node1_public_ip:node1_wg_ip,node2_public_ip:node2_wg_ip,..."
#
# Example (3 nodes):
#   ./setup-wireguard-mesh.sh \
#     --nodes "54.1.1.1:10.200.0.1,35.2.2.2:10.200.0.2,13.3.3.3:10.200.0.3"

set -e

# Default values
WG_INTERFACE="wg0"
WG_PORT="51820"
WG_NETWORK="10.200.0.0/24"
NODES=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--nodes)
            NODES="$2"
            shift 2
            ;;
        -p|--port)
            WG_PORT="$2"
            shift 2
            ;;
        -i|--interface)
            WG_INTERFACE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 --nodes \"pub_ip1:wg_ip1,pub_ip2:wg_ip2,...\""
            echo ""
            echo "Options:"
            echo "  -n, --nodes      Comma-separated list of public_ip:wireguard_ip pairs"
            echo "  -p, --port       WireGuard UDP port (default: 51820)"
            echo "  -i, --interface  WireGuard interface name (default: wg0)"
            echo ""
            echo "Example:"
            echo "  $0 --nodes \"54.1.1.1:10.200.0.1,35.2.2.2:10.200.0.2,13.3.3.3:10.200.0.3\""
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$NODES" ]; then
    echo "ERROR: --nodes parameter is required"
    echo "Use --help for usage information"
    exit 1
fi

# Install WireGuard
install_wireguard() {
    echo "Installing WireGuard..."
    if command -v apt-get &>/dev/null; then
        sudo DEBIAN_FRONTEND=noninteractive apt-get update
        sudo DEBIAN_FRONTEND=noninteractive apt-get install -y wireguard wireguard-tools
    elif command -v yum &>/dev/null; then
        sudo yum install -y epel-release
        sudo yum install -y wireguard-tools
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y wireguard-tools
    else
        echo "ERROR: Unsupported package manager"
        exit 1
    fi
}

# Generate or load WireGuard keys
setup_keys() {
    local key_dir="/etc/wireguard"
    sudo mkdir -p "$key_dir"

    if [ ! -f "$key_dir/privatekey" ]; then
        echo "Generating WireGuard keys..."
        wg genkey | sudo tee "$key_dir/privatekey" > /dev/null
        sudo chmod 600 "$key_dir/privatekey"
        sudo cat "$key_dir/privatekey" | wg pubkey | sudo tee "$key_dir/publickey" > /dev/null
    fi

    PRIVATE_KEY=$(sudo cat "$key_dir/privatekey")
    PUBLIC_KEY=$(sudo cat "$key_dir/publickey")

    echo "This node's public key: $PUBLIC_KEY"
}

# Detect current node from the node list
detect_current_node() {
    local all_ips=$(ip addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1')
    local public_ip=$(curl -s --connect-timeout 5 http://checkip.amazonaws.com || \
                      curl -s --connect-timeout 5 https://ipinfo.io/ip || \
                      curl -s --connect-timeout 5 https://api.ipify.org)

    IFS=',' read -ra NODE_ARRAY <<< "$NODES"

    for i in "${!NODE_ARRAY[@]}"; do
        IFS=':' read -r node_pub_ip node_wg_ip <<< "${NODE_ARRAY[$i]}"

        # Check if this is our public IP
        if [ "$node_pub_ip" = "$public_ip" ]; then
            CURRENT_INDEX=$i
            CURRENT_PUB_IP=$node_pub_ip
            CURRENT_WG_IP=$node_wg_ip
            return 0
        fi

        # Check if this IP is assigned to this machine
        if echo "$all_ips" | grep -q "^${node_pub_ip}$"; then
            CURRENT_INDEX=$i
            CURRENT_PUB_IP=$node_pub_ip
            CURRENT_WG_IP=$node_wg_ip
            return 0
        fi
    done

    echo "ERROR: Could not detect current node from the provided node list"
    echo "Current machine IPs: $all_ips"
    echo "Detected public IP: $public_ip"
    exit 1
}

# Collect public keys from all nodes (interactive step)
collect_peer_keys() {
    echo ""
    echo "=========================================="
    echo "IMPORTANT: Key Exchange Required"
    echo "=========================================="
    echo ""
    echo "This node's information:"
    echo "  Public IP: $CURRENT_PUB_IP"
    echo "  WireGuard IP: $CURRENT_WG_IP"
    echo "  Public Key: $PUBLIC_KEY"
    echo ""
    echo "To complete the mesh setup, you need to:"
    echo "1. Run this script on ALL other nodes first"
    echo "2. Collect their public keys"
    echo "3. Create a key file at /etc/wireguard/peer_keys with format:"
    echo "   public_ip:wireguard_ip:public_key"
    echo ""
    echo "Example /etc/wireguard/peer_keys file:"
    echo "---"

    IFS=',' read -ra NODE_ARRAY <<< "$NODES"
    for i in "${!NODE_ARRAY[@]}"; do
        IFS=':' read -r node_pub_ip node_wg_ip <<< "${NODE_ARRAY[$i]}"
        if [ $i -eq $CURRENT_INDEX ]; then
            echo "$node_pub_ip:$node_wg_ip:$PUBLIC_KEY  # This node"
        else
            echo "$node_pub_ip:$node_wg_ip:<PUBLIC_KEY_FROM_NODE_$i>"
        fi
    done
    echo "---"
    echo ""

    # Check if peer keys file exists
    if [ -f /etc/wireguard/peer_keys ]; then
        echo "Found /etc/wireguard/peer_keys - using existing keys"
        return 0
    fi

    # Auto-generate peer_keys with placeholder for this node
    echo "Creating /etc/wireguard/peer_keys template..."
    sudo touch /etc/wireguard/peer_keys

    for i in "${!NODE_ARRAY[@]}"; do
        IFS=':' read -r node_pub_ip node_wg_ip <<< "${NODE_ARRAY[$i]}"
        if [ $i -eq $CURRENT_INDEX ]; then
            echo "$node_pub_ip:$node_wg_ip:$PUBLIC_KEY" | sudo tee -a /etc/wireguard/peer_keys > /dev/null
        fi
    done

    echo ""
    echo "Template created. Please add other nodes' public keys to /etc/wireguard/peer_keys"
    echo "Then run this script again."
    exit 0
}

# Generate WireGuard configuration
generate_config() {
    echo "Generating WireGuard configuration..."

    local config_file="/etc/wireguard/${WG_INTERFACE}.conf"

    # Start with interface section
    sudo tee "$config_file" > /dev/null << EOF
[Interface]
PrivateKey = $PRIVATE_KEY
Address = $CURRENT_WG_IP/24
ListenPort = $WG_PORT
SaveConfig = false

# Enable IP forwarding via PostUp
PostUp = sysctl -w net.ipv4.ip_forward=1
PostUp = iptables -A FORWARD -i %i -j ACCEPT
PostUp = iptables -A FORWARD -o %i -j ACCEPT
PostDown = iptables -D FORWARD -i %i -j ACCEPT
PostDown = iptables -D FORWARD -o %i -j ACCEPT

EOF

    # Add peer sections
    while IFS=: read -r peer_pub_ip peer_wg_ip peer_pubkey; do
        # Skip empty lines and comments
        [[ -z "$peer_pub_ip" || "$peer_pub_ip" =~ ^# ]] && continue

        # Skip current node
        if [ "$peer_pub_ip" = "$CURRENT_PUB_IP" ]; then
            continue
        fi

        # Skip if public key is placeholder
        if [[ "$peer_pubkey" =~ ^"<" ]]; then
            echo "WARNING: Skipping peer $peer_pub_ip - no public key provided"
            continue
        fi

        echo "Adding peer: $peer_pub_ip ($peer_wg_ip)"

        sudo tee -a "$config_file" > /dev/null << EOF
[Peer]
# Node: $peer_pub_ip
PublicKey = $peer_pubkey
AllowedIPs = $peer_wg_ip/32
Endpoint = $peer_pub_ip:$WG_PORT
PersistentKeepalive = 25

EOF
    done < /etc/wireguard/peer_keys

    sudo chmod 600 "$config_file"
}

# Start WireGuard
start_wireguard() {
    echo "Starting WireGuard..."

    # Stop existing interface if running
    sudo wg-quick down "$WG_INTERFACE" 2>/dev/null || true

    # Start interface
    sudo wg-quick up "$WG_INTERFACE"

    # Enable on boot
    sudo systemctl enable wg-quick@${WG_INTERFACE}

    echo ""
    echo "WireGuard status:"
    sudo wg show "$WG_INTERFACE"
}

# Test connectivity
test_connectivity() {
    echo ""
    echo "Testing connectivity to peers..."

    while IFS=: read -r peer_pub_ip peer_wg_ip peer_pubkey; do
        [[ -z "$peer_pub_ip" || "$peer_pub_ip" =~ ^# ]] && continue
        [ "$peer_pub_ip" = "$CURRENT_PUB_IP" ] && continue
        [[ "$peer_pubkey" =~ ^"<" ]] && continue

        echo -n "  Ping $peer_wg_ip ($peer_pub_ip): "
        if ping -c 1 -W 3 "$peer_wg_ip" &>/dev/null; then
            echo "OK"
        else
            echo "FAILED (may need time to establish connection)"
        fi
    done < /etc/wireguard/peer_keys
}

# Main execution
main() {
    echo "==== WireGuard Mesh VPN Setup ===="
    echo "Nodes: $NODES"
    echo "Port: $WG_PORT"
    echo "=================================="
    echo ""

    install_wireguard
    setup_keys
    detect_current_node

    echo ""
    echo "Detected current node:"
    echo "  Index: $CURRENT_INDEX"
    echo "  Public IP: $CURRENT_PUB_IP"
    echo "  WireGuard IP: $CURRENT_WG_IP"

    # Check if we have peer keys
    if [ ! -f /etc/wireguard/peer_keys ]; then
        collect_peer_keys
    fi

    # Verify we have keys for all peers
    local peer_count=$(grep -v "^#" /etc/wireguard/peer_keys | grep -v "^$" | wc -l)
    IFS=',' read -ra NODE_ARRAY <<< "$NODES"
    local expected_count=${#NODE_ARRAY[@]}

    if [ "$peer_count" -lt "$expected_count" ]; then
        echo ""
        echo "WARNING: peer_keys has $peer_count entries, expected $expected_count"
        echo "Please ensure all nodes' public keys are added to /etc/wireguard/peer_keys"
        collect_peer_keys
    fi

    generate_config
    start_wireguard
    test_connectivity

    echo ""
    echo "==== WireGuard Setup Complete ===="
    echo ""
    echo "This node's WireGuard IP: $CURRENT_WG_IP"
    echo ""
    echo "For Kubernetes, use WireGuard IPs:"
    echo "  Control Plane: ./k8s-control-plane-setup.sh -i $CURRENT_WG_IP"
    echo "  Worker: ./k8s-worker-setup.sh -j \"kubeadm join <control-plane-wg-ip>:6443 ...\""
    echo ""
}

main "$@"
