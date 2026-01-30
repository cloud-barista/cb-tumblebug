#!/bin/bash

# Kubernetes Worker Node Setup Script
# This script installs and configures a Kubernetes worker node on Ubuntu
# Designed for unattended execution via SSH or pipe (curl | bash)

set -e

# Default values
NODE_IP=""                # IP address for this node (optional, auto-detected)
CONTROL_PLANE_IP=""       # IP address of the control plane (required)
JOIN_TOKEN=""             # Join token from control plane (required)
JOIN_CA_HASH=""           # CA cert hash from control plane (required)
K8S_VERSION="1.35"        # Kubernetes version (must match control plane)

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -i|--ip)
            NODE_IP="$2"
            shift 2
            ;;
        -c|--control-plane)
            CONTROL_PLANE_IP="$2"
            shift 2
            ;;
        -t|--token)
            JOIN_TOKEN="$2"
            shift 2
            ;;
        --hash)
            JOIN_CA_HASH="$2"
            shift 2
            ;;
        -v|--version)
            K8S_VERSION="$2"
            shift 2
            ;;
        -j|--join-command)
            # Parse full join command: "kubeadm join IP:6443 --token TOKEN --discovery-token-ca-cert-hash sha256:HASH"
            FULL_JOIN_COMMAND="$2"
            # Extract control plane IP (with port)
            CONTROL_PLANE_IP=$(echo "$FULL_JOIN_COMMAND" | grep -oP '(?<=join )[0-9.:]+')
            # Extract token
            JOIN_TOKEN=$(echo "$FULL_JOIN_COMMAND" | grep -oP '(?<=--token )[a-z0-9.]+')
            # Extract CA hash
            JOIN_CA_HASH=$(echo "$FULL_JOIN_COMMAND" | grep -oP '(?<=--discovery-token-ca-cert-hash )sha256:[a-f0-9]+')
            shift 2
            ;;
        -h|--help)
            echo "Kubernetes Worker Node Setup Script"
            echo ""
            echo "Usage: $0 -j \"JOIN_COMMAND\" [OPTIONS]"
            echo "   or: $0 -c CONTROL_IP:PORT -t TOKEN --hash CA_HASH [OPTIONS]"
            echo "   or: curl -fsSL <url> | bash -s -- -j \"JOIN_COMMAND\""
            echo ""
            echo "Options:"
            echo "  -j, --join-command  Full kubeadm join command from control plane (recommended)"
            echo "  -c, --control-plane Control plane IP:PORT"
            echo "  -t, --token         Join token"
            echo "      --hash          CA certificate hash (sha256:...)"
            echo "  -v, --version       Kubernetes version (default: 1.35)"
            echo "  -i, --ip            Node IP (optional, for multi-interface environments)"
            echo "  -h, --help          Show this help message"
            echo ""
            echo "Examples:"
            echo "  # Using full join command (recommended)"
            echo "  $0 -j \"kubeadm join 10.0.0.1:6443 --token abc.123 --discovery-token-ca-cert-hash sha256:xyz\""
            echo ""
            echo "  # Using separate parameters"
            echo "  $0 -c 10.0.0.1:6443 -t abc.123 --hash sha256:xyz"
            echo ""
            echo "  # Pipe execution"
            echo "  curl -fsSL <url> | bash -s -- -j \"kubeadm join ...\""
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$CONTROL_PLANE_IP" ] || [ -z "$JOIN_TOKEN" ] || [ -z "$JOIN_CA_HASH" ]; then
    echo "ERROR: Missing required parameters"
    echo ""
    echo "Required: -j \"JOIN_COMMAND\" or -c CONTROL_IP -t TOKEN --hash CA_HASH"
    echo ""
    echo "Get the join command from control plane output, or run on control plane:"
    echo "  cat ~/k8s-worker-join-command.txt"
    echo ""
    echo "Use -h or --help for usage information"
    exit 1
fi

# Auto-detect NODE_IP if not provided
DETECTED_IP=$(hostname -I | awk '{print $1}')

echo "==== Kubernetes Worker Node Setup ===="
echo "Kubernetes Version: $K8S_VERSION"
echo "Node IP: ${NODE_IP:-$DETECTED_IP (auto-detected)}"
echo "Control Plane: $CONTROL_PLANE_IP"
echo "====================================="
echo

# Disable swap (required for Kubernetes)
echo "Disabling swap..."
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# Load required kernel modules
echo "Loading kernel modules..."
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# Set required sysctl parameters
echo "Setting sysctl parameters..."
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system > /dev/null 2>&1

# Install containerd
echo "Installing containerd..."
sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ca-certificates curl gnupg lsb-release > /dev/null

# Add Docker's official GPG key
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add Docker repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq containerd.io > /dev/null

# Configure containerd to use systemd cgroup driver
echo "Configuring containerd..."
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null

# Enable SystemdCgroup
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

sudo systemctl restart containerd
sudo systemctl enable containerd

# Install Kubernetes components
echo "Installing Kubernetes components (version $K8S_VERSION)..."
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq apt-transport-https > /dev/null

# Add Kubernetes apt repository
curl -fsSL https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/Release.key | sudo gpg --dearmor --yes -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list > /dev/null

sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq kubelet kubeadm kubectl > /dev/null
sudo apt-mark hold kubelet kubeadm kubectl > /dev/null

# Join the Kubernetes cluster
echo "Joining Kubernetes cluster..."

# Build join command with optional --node-ip
JOIN_CMD="sudo kubeadm join ${CONTROL_PLANE_IP} --token ${JOIN_TOKEN} --discovery-token-ca-cert-hash ${JOIN_CA_HASH}"
if [ -n "$NODE_IP" ]; then
    JOIN_CMD="$JOIN_CMD --node-ip ${NODE_IP}"
fi

eval $JOIN_CMD 2>&1 | tee ~/kubeadm-join.log

# Check if join succeeded
if [ ${PIPESTATUS[0]} -eq 0 ]; then
    WORKER_IP="${NODE_IP:-$DETECTED_IP}"
    echo ""
    echo "========================================"
    echo "SUCCESS: Worker node joined the cluster!"
    echo "========================================"
    echo ""
    echo "[K8S_WORKER_IP]"
    echo "$WORKER_IP"
    echo ""
    echo "[K8S_CONTROL_PLANE]"
    echo "$CONTROL_PLANE_IP"
    echo ""
    echo "========================================"
    echo "Quick Reference"
    echo "========================================"
    echo ""
    echo "Verify on control plane:"
    echo "  kubectl get nodes"
    echo "  kubectl get pods -A"
    echo ""
    exit 0
else
    echo ""
    echo "========================================"
    echo "ERROR: Failed to join Kubernetes cluster"
    echo "========================================"
    echo ""
    echo "Please check:"
    echo "  - Control plane is running and accessible at $CONTROL_PLANE_IP"
    echo "  - Token and CA hash are correct (tokens expire after 24h)"
    echo "  - Network connectivity (ping $CONTROL_PLANE_IP)"
    echo "  - Firewall allows ports: 6443, 10250"
    echo ""
    echo "To regenerate join command on control plane:"
    echo "  kubeadm token create --print-join-command"
    echo ""
    echo "Logs: ~/kubeadm-join.log"
    exit 1
fi
