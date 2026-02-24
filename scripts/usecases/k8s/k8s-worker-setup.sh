#!/bin/bash

# Kubernetes Worker Node Setup Script
# This script installs and configures a Kubernetes worker node on Ubuntu
# Designed for unattended execution via SSH or pipe (curl | bash)
#
# GPU Support:
#   - Automatically detects NVIDIA GPU and adds appropriate labels
#   - For GPU nodes, run installCudaDriver.sh BEFORE this script
#   - GPU Operator on control plane will configure GPU device plugin
#
# Remote execution (CB-MapUI / CB-Tumblebug API):
#   This script is designed for non-interactive SSH execution.
#   All prompts are suppressed using DEBIAN_FRONTEND, needrestart config, etc.

set -e

# ============================================================
# Non-interactive mode for SSH remote execution
# ============================================================
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1

# Disable needrestart interactive prompts (Ubuntu 22.04+)
if [ -d /etc/needrestart/conf.d ]; then
    echo "\$nrconf{restart} = 'a';" | sudo tee /etc/needrestart/conf.d/99-autorestart.conf > /dev/null 2>&1 || true
fi

# ============================================================
# Fix dpkg/apt state (cleanup from previous failed installations)
# ============================================================
echo "Cleaning up any interrupted package operations..."

# Wait for any existing apt/dpkg locks to be released
wait_for_apt() {
    local max_wait=60
    local waited=0
    while sudo fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
          sudo fuser /var/lib/apt/lists/lock >/dev/null 2>&1 || \
          sudo fuser /var/cache/apt/archives/lock >/dev/null 2>&1; do
        if [ $waited -ge $max_wait ]; then
            echo "Warning: apt lock still held after ${max_wait}s, attempting to proceed..."
            break
        fi
        echo "Waiting for apt lock to be released... ($waited/${max_wait}s)"
        sleep 5
        waited=$((waited + 5))
    done
}

wait_for_apt

# Kill any stuck apt/dpkg processes (only if they are hanging)
# Note: Use pgrep to find apt/dpkg processes and force-kill them if necessary
sudo pgrep -f "apt-get|dpkg" | xargs -r sudo kill -9 2>/dev/null || true
sleep 2

# Remove stale lock files
sudo rm -f /var/lib/dpkg/lock-frontend 2>/dev/null || true
sudo rm -f /var/lib/dpkg/lock 2>/dev/null || true
sudo rm -f /var/cache/apt/archives/lock 2>/dev/null || true
sudo rm -f /var/lib/apt/lists/lock 2>/dev/null || true

# Reconfigure any partially installed packages
sudo dpkg --configure -a 2>/dev/null || true

# Fix any broken dependencies
sudo DEBIAN_FRONTEND=noninteractive apt-get -f install -y \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" 2>/dev/null || true

echo "Package system cleanup complete."

# Default values
NODE_IP=""                # IP address for this node (optional, auto-detected)
CONTROL_PLANE_IP=""       # IP address of the control plane (required)
JOIN_TOKEN=""             # Join token from control plane (required)
JOIN_CA_HASH=""           # CA cert hash from control plane (required)
K8S_VERSION="1.35"        # Kubernetes version (must match control plane)
GPU_DETECTED=false        # Will be set to true if NVIDIA GPU found

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

# Auto-detect NVIDIA GPU
if lspci 2>/dev/null | grep -qi nvidia; then
    GPU_DETECTED=true
    GPU_INFO=$(lspci | grep -i nvidia | head -1)
fi

echo "==== Kubernetes Worker Node Setup ===="
echo "Kubernetes Version: $K8S_VERSION"
echo "Node IP: ${NODE_IP:-$DETECTED_IP (auto-detected)}"
echo "Control Plane: $CONTROL_PLANE_IP"
if [ "$GPU_DETECTED" = true ]; then
    echo "GPU Detected: $GPU_INFO"
fi
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
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" \
    ca-certificates curl gnupg lsb-release > /dev/null

# Add Docker's official GPG key
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add Docker repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" \
    containerd.io > /dev/null

# Configure containerd to use systemd cgroup driver
echo "Configuring containerd..."
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null

# Enable SystemdCgroup
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

# If NVIDIA GPU detected & nvidia-ctk available, re-configure nvidia runtime
# (containerd config default above resets any previous nvidia-ctk settings)
if [ "$GPU_DETECTED" = true ] && command -v nvidia-ctk &>/dev/null; then
    echo "  Re-applying NVIDIA container runtime config..."
    # --set-as-default: makes nvidia the default runtime (required for GPU Operator validator pods)
    sudo nvidia-ctk runtime configure --runtime=containerd --set-as-default 2>/dev/null || true
fi

sudo systemctl restart containerd
sudo systemctl enable containerd

# Install Kubernetes components
echo "Installing Kubernetes components (version $K8S_VERSION)..."
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq apt-transport-https > /dev/null

# Add Kubernetes apt repository
curl -fsSL https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/Release.key | sudo gpg --dearmor --yes -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list > /dev/null

sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" \
    kubelet kubeadm kubectl > /dev/null
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
    if [ "$GPU_DETECTED" = true ]; then
        echo "SUCCESS: GPU Worker node joined the cluster!"
    else
        echo "SUCCESS: Worker node joined the cluster!"
    fi
    echo "========================================"
    echo ""
    echo "[K8S_WORKER_IP]"
    echo "$WORKER_IP"
    echo ""
    echo "[K8S_CONTROL_PLANE]"
    echo "$CONTROL_PLANE_IP"
    echo ""
    if [ "$GPU_DETECTED" = true ]; then
        echo "[K8S_WORKER_TYPE]"
        echo "gpu"
        echo ""
        echo "[K8S_GPU_INFO]"
        echo "$GPU_INFO"
        echo ""
    fi
    echo "========================================"
    echo "Quick Reference"
    echo "========================================"
    echo ""
    echo "Verify on control plane:"
    echo "  kubectl get nodes"
    echo "  kubectl get pods -A"
    if [ "$GPU_DETECTED" = true ]; then
        echo ""
        echo "GPU Node Verification (on control plane):"
        echo "  kubectl describe node <node-name> | grep nvidia"
        echo "  kubectl get pods -n gpu-operator"
        echo ""
        echo "Note: GPU resources will be available after GPU Operator"
        echo "      finishes configuring this node (~2-5 minutes)"
    fi
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
