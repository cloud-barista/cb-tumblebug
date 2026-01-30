#!/bin/bash

# Kubernetes Control Plane Setup Script
# This script installs and configures a Kubernetes control plane node on Ubuntu
# Designed for unattended execution via SSH or pipe (curl | bash)
# Based on https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/

set -e

# Default values
NODE_IP=""                # IP for API server binding (private IP recommended, auto-detected if not provided)
EXTERNAL_IP=""            # External/Public IP for cert SAN and external access (optional)
POD_NETWORK_CIDR="10.244.0.0/16"  # Pod network CIDR (default for Flannel)
K8S_VERSION="1.35"        # Kubernetes version (1.35, 1.34, 1.33, etc.)

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -i|--ip)
            NODE_IP="$2"
            shift 2
            ;;
        -e|--external-ip)
            EXTERNAL_IP="$2"
            shift 2
            ;;
        -p|--pod-cidr)
            POD_NETWORK_CIDR="$2"
            shift 2
            ;;
        -v|--version)
            K8S_VERSION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Kubernetes Control Plane Setup Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo "   or: curl -fsSL <url> | bash -s -- [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -i, --ip           Node IP for API server binding (default: auto-detect private IP)"
            echo "  -e, --external-ip  External/Public IP for cert SAN (default: auto-detect public IP)"
            echo "  -p, --pod-cidr     Pod network CIDR (default: 10.244.0.0/16)"
            echo "  -v, --version      Kubernetes version (default: 1.35)"
            echo "  -h, --help         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Auto-detect IPs"
            echo "  $0 -i 10.0.0.1                        # Specify node IP"
            echo "  $0 -i 10.0.0.1 -e 54.1.2.3            # Specify both IPs"
            echo "  curl -fsSL <url> | bash -s -- -v 1.34 # Pipe execution"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Auto-detect NODE_IP (private IP) if not provided
if [ -z "$NODE_IP" ]; then
    # Get the primary private IP (first non-loopback IP)
    NODE_IP=$(hostname -I | awk '{print $1}')
    if [ -z "$NODE_IP" ]; then
        echo "ERROR: Could not detect private IP address"
        echo "Please specify with -i option"
        exit 1
    fi
fi

# Auto-detect EXTERNAL_IP (public IP) if not provided
if [ -z "$EXTERNAL_IP" ]; then
    EXTERNAL_IP=$(curl -s --connect-timeout 5 http://checkip.amazonaws.com || \
                  curl -s --connect-timeout 5 https://ipinfo.io/ip || \
                  curl -s --connect-timeout 5 https://api.ipify.org || \
                  echo "")
fi

# Build cert SANs list
CERT_SANS="$NODE_IP"
if [ -n "$EXTERNAL_IP" ] && [ "$EXTERNAL_IP" != "$NODE_IP" ]; then
    CERT_SANS="$NODE_IP,$EXTERNAL_IP"
fi

echo "==== Kubernetes Control Plane Setup ===="
echo "Kubernetes Version: $K8S_VERSION"
echo "Node IP (binding): $NODE_IP"
echo "External IP (cert SAN): ${EXTERNAL_IP:-N/A}"
echo "Pod Network CIDR: $POD_NETWORK_CIDR"
echo "========================================"
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

# Initialize Kubernetes control plane
echo "Initializing Kubernetes control plane..."
sudo kubeadm init \
  --apiserver-advertise-address=${NODE_IP} \
  --pod-network-cidr=${POD_NETWORK_CIDR} \
  --apiserver-cert-extra-sans=${CERT_SANS} \
  2>&1 | tee ~/kubeadm-init.log

# Check if initialization succeeded
if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "ERROR: Kubernetes control plane initialization failed"
    echo "Check logs at: ~/kubeadm-init.log"
    exit 1
fi

# Set up kubeconfig for the current user
echo "Setting up kubeconfig..."
mkdir -p $HOME/.kube
sudo cp -f /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# Install Flannel CNI plugin
echo "Installing Flannel CNI plugin..."
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml > /dev/null

# Wait for the node to be ready
echo "Waiting for node to be ready..."
for i in {1..30}; do
    if kubectl get nodes 2>/dev/null | grep -q "Ready"; then
        break
    fi
    sleep 2
done

# Extract join command for workers
echo "Extracting worker join command..."
JOIN_COMMAND=$(sudo kubeadm token create --print-join-command)
echo "$JOIN_COMMAND" > ~/k8s-worker-join-command.txt
chmod 600 ~/k8s-worker-join-command.txt

# Generate external kubeconfig
echo "Generating external kubeconfig..."
EXTERNAL_KUBECONFIG=~/kubeconfig-external.yaml
cp $HOME/.kube/config "$EXTERNAL_KUBECONFIG"

# Determine which IP to use for external access
ACCESS_IP="${EXTERNAL_IP:-$NODE_IP}"

# Replace internal API server address with external access IP
CURRENT_SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
sed -i "s|${CURRENT_SERVER}|https://${ACCESS_IP}:6443|g" "$EXTERNAL_KUBECONFIG"
chmod 600 "$EXTERNAL_KUBECONFIG"

# Check cluster status
echo ""
echo "Checking cluster status..."
kubectl get nodes -o wide
echo ""

# Print success message with structured output for easy parsing
if kubectl get nodes 2>/dev/null | grep -q "Ready"; then
    echo ""
    echo "========================================"
    echo "SUCCESS: Kubernetes control plane setup complete!"
    echo "========================================"
    echo ""
    echo "[K8S_CONTROL_PLANE_IP]"
    echo "$ACCESS_IP"
    echo ""
    echo "[K8S_NODE_IP]"
    echo "$NODE_IP"
    echo ""
    echo "[K8S_JOIN_COMMAND]"
    cat ~/k8s-worker-join-command.txt
    echo ""
    echo "[K8S_KUBECONFIG_BASE64]"
    base64 -w 0 "$EXTERNAL_KUBECONFIG"
    echo ""
    echo ""
    echo "========================================"
    echo "Quick Reference"
    echo "========================================"
    echo ""
    echo "Worker Join Command (saved to ~/k8s-worker-join-command.txt):"
    echo "  $(cat ~/k8s-worker-join-command.txt)"
    echo ""
    echo "External Kubeconfig (saved to ~/kubeconfig-external.yaml):"
    echo "  1. Copy to local: scp user@${ACCESS_IP}:~/kubeconfig-external.yaml ./kubeconfig.yaml"
    echo "  2. Use: export KUBECONFIG=./kubeconfig.yaml && kubectl get nodes"
    echo ""
    echo "Kubectl on this node:"
    echo "  kubectl get nodes"
    echo "  kubectl get pods -A"
    echo ""
    exit 0
else
    echo "WARNING: Node is not ready yet. Please check with 'kubectl get nodes'"
    exit 1
fi
