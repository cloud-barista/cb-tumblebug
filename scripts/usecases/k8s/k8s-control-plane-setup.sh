#!/bin/bash

# Kubernetes Control Plane Setup Script
# This script installs and configures a Kubernetes control plane node on Ubuntu
# Designed for unattended execution via SSH or pipe (curl | bash)
# Based on https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/
#
# Modes:
#   Standard:  Basic K8s control plane with Flannel CNI
#   llm-d:     Adds Gateway API, LeaderWorkerSet, Helm, NVIDIA GPU Operator
#              for distributed LLM inference deployments
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
NODE_IP=""                # IP for API server binding (private IP recommended, auto-detected if not provided)
EXTERNAL_IP=""            # External/Public IP for cert SAN and external access (optional)
POD_NETWORK_CIDR="10.244.0.0/16"  # Pod network CIDR (default for Flannel)
K8S_VERSION="1.35"        # Kubernetes version (1.35, 1.34, 1.33, etc.)
LLMD_MODE=false           # Enable llm-d components (Gateway API, LeaderWorkerSet, GPU Operator)

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
        --llm-d|--llmd)
            LLMD_MODE=true
            shift
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
            echo "      --llm-d        Enable llm-d mode: install Gateway API, LeaderWorkerSet,"
            echo "                     Helm, and NVIDIA GPU Operator for distributed LLM inference"
            echo "  -h, --help         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Standard K8s setup"
            echo "  $0 --llm-d                            # K8s + llm-d components"
            echo "  $0 -i 10.0.0.1 -e 54.1.2.3            # Specify both IPs"
            echo "  curl -fsSL <url> | bash -s -- --llm-d # Pipe execution with llm-d"
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
if [ "$LLMD_MODE" = true ]; then
    echo "Mode: llm-d (Gateway API + LeaderWorkerSet + GPU Operator)"
else
    echo "Mode: Standard"
fi
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
if lspci 2>/dev/null | grep -qi nvidia && command -v nvidia-ctk &>/dev/null; then
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

# ============================================================
# Check if Kubernetes is already initialized
# ============================================================
K8S_ALREADY_INITIALIZED=false

if [ -f /etc/kubernetes/admin.conf ]; then
    echo ""
    echo "=========================================="
    echo "Kubernetes cluster already initialized!"
    echo "=========================================="
    K8S_ALREADY_INITIALIZED=true
    
    # Ensure kubeconfig is set up
    if [ ! -f "$HOME/.kube/config" ]; then
        echo "Setting up kubeconfig..."
        mkdir -p $HOME/.kube
        sudo cp -f /etc/kubernetes/admin.conf $HOME/.kube/config
        sudo chown $(id -u):$(id -g) $HOME/.kube/config
    fi
    
    # Check cluster status
    if kubectl get nodes &>/dev/null; then
        echo "Cluster is accessible. Current nodes:"
        kubectl get nodes
    else
        echo "Warning: Cluster exists but kubectl cannot connect"
    fi
    echo ""
    
    if [ "$LLMD_MODE" = true ]; then
        echo "Proceeding to install llm-d components on existing cluster..."
    else
        echo "Nothing to do. Cluster is already set up."
        echo ""
        echo "To add llm-d components, run:"
        echo "  $0 --llm-d"
        echo ""
        # Still output the join command for convenience
        KUBEADM_JOIN_CMD=$(kubeadm token create --print-join-command 2>/dev/null || cat ~/k8s-worker-join-command.txt 2>/dev/null || echo "")
        if [ -n "$KUBEADM_JOIN_CMD" ]; then
            echo "[K8S_JOIN_COMMAND]"
            echo "$KUBEADM_JOIN_CMD"
        fi
        exit 0
    fi
fi

# Only run kubeadm init if not already initialized
if [ "$K8S_ALREADY_INITIALIZED" = false ]; then
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
fi  # End of K8S_ALREADY_INITIALIZED check

# ============================================================
# llm-d Mode: Install additional components
# Reference: https://llm-d.ai/docs/guide/Installation/prerequisites
# ============================================================
if [ "$LLMD_MODE" = true ]; then
    echo ""
    echo "=========================================="
    echo "Installing llm-d infrastructure components..."
    echo "=========================================="
    
    LLMD_INSTALL_ERRORS=0

    # ---- Tool Dependencies ----

    # Install Helm (v3.12+ required by llm-d)
    echo "Installing Helm..."
    if ! command -v helm &>/dev/null; then
        curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash > /dev/null 2>&1
    fi
    if command -v helm &>/dev/null; then
        echo "  ✓ Helm $(helm version --short 2>/dev/null | head -1)"
    else
        echo "  ✗ Helm installation failed"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # Install helmfile (v1.1+ required by llm-d)
    echo "Installing helmfile..."
    if ! command -v helmfile &>/dev/null; then
        HELMFILE_VERSION="1.1.3"
        HELMFILE_ARCH=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
        curl -fsSL -o /tmp/helmfile.tar.gz \
            "https://github.com/helmfile/helmfile/releases/download/v${HELMFILE_VERSION}/helmfile_${HELMFILE_VERSION}_linux_${HELMFILE_ARCH}.tar.gz" 2>/dev/null
        sudo tar -xzf /tmp/helmfile.tar.gz -C /usr/local/bin helmfile 2>/dev/null || true
        sudo chmod +x /usr/local/bin/helmfile 2>/dev/null || true
        rm -f /tmp/helmfile.tar.gz
    fi
    if command -v helmfile &>/dev/null; then
        echo "  ✓ helmfile $(helmfile version 2>/dev/null | head -1)"
    else
        echo "  ✗ helmfile installation failed"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # Install yq (v4+ required by llm-d)
    echo "Installing yq..."
    if ! command -v yq &>/dev/null; then
        YQ_ARCH=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
        sudo curl -fsSL -o /usr/local/bin/yq \
            "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_${YQ_ARCH}" 2>/dev/null
        sudo chmod +x /usr/local/bin/yq 2>/dev/null || true
    fi
    if command -v yq &>/dev/null; then
        echo "  ✓ yq $(yq --version 2>/dev/null | head -1)"
    else
        echo "  ✗ yq installation failed"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # Install helm-diff plugin (required by helmfile)
    echo "Installing helm-diff plugin..."
    helm plugin install https://github.com/databus23/helm-diff 2>/dev/null || true
    if helm plugin list 2>/dev/null | grep -q diff; then
        echo "  ✓ helm-diff plugin installed"
    else
        echo "  ⚠ helm-diff plugin may not be installed (helmfile may install it automatically)"
    fi

    # ---- Kubernetes CRDs ----

    # Install Gateway API CRDs v1.4.0 (required by llm-d v0.5+)
    # Ref: https://github.com/llm-d/llm-d/blob/main/guides/prereq/gateway-provider/README.md
    GATEWAY_API_VERSION="v1.4.0"
    GATEWAY_API_URL="https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/standard-install.yaml"
    echo "Installing Gateway API CRDs (${GATEWAY_API_VERSION})..."
    GATEWAY_INSTALLED=false
    for attempt in 1 2 3; do
        if kubectl apply -f "$GATEWAY_API_URL" 2>&1; then
            GATEWAY_INSTALLED=true
            break
        fi
        echo "  Retry ${attempt}/3..."
        sleep 3
    done
    if kubectl get crd gateways.gateway.networking.k8s.io &>/dev/null; then
        echo "  ✓ Gateway API CRDs verified"
    else
        echo "  ✗ Gateway API CRDs NOT found after installation"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # Install Gateway API Inference Extension CRDs v1.3.0 (InferencePool, etc.)
    # Ref: https://gateway-api-inference-extension.sigs.k8s.io/
    # Note: The Istio install step below may also install these via install-gateway-provider-dependencies.sh
    GAIE_VERSION="v1.3.0"
    echo "Installing Gateway API Inference Extension CRDs (${GAIE_VERSION})..."
    GAIE_INSTALLED=false
    for attempt in 1 2 3; do
        if kubectl apply -k "https://github.com/kubernetes-sigs/gateway-api-inference-extension/config/crd?ref=${GAIE_VERSION}" 2>&1; then
            GAIE_INSTALLED=true
            break
        fi
        echo "  Retry ${attempt}/3..."
        sleep 3
    done
    if kubectl get crd inferencepools.inference.networking.k8s.io &>/dev/null; then
        echo "  ✓ Gateway API Inference Extension CRDs verified (InferencePool)"
    else
        echo "  ⚠ Inference Extension CRDs not found yet (may be installed by gateway setup below)"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # Install LeaderWorkerSet CRD v0.7.0+ (for multi-host inference)
    LWS_VERSION="v0.7.0"
    LWS_URL="https://github.com/kubernetes-sigs/lws/releases/download/${LWS_VERSION}/manifests.yaml"
    echo "Installing LeaderWorkerSet CRD (${LWS_VERSION})..."
    LWS_INSTALLED=false
    for attempt in 1 2 3; do
        if kubectl apply --server-side -f "$LWS_URL" 2>&1; then
            LWS_INSTALLED=true
            break
        fi
        echo "  Retry ${attempt}/3..."
        sleep 3
    done
    if kubectl get crd leaderworkersets.leaderworkerset.x-k8s.io &>/dev/null; then
        echo "  ✓ LeaderWorkerSet CRD verified"
    else
        echo "  ✗ LeaderWorkerSet CRD NOT found after installation"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # Wait for LeaderWorkerSet controller to be ready
    echo "Waiting for LeaderWorkerSet controller..."
    LWS_READY=false
    for i in {1..30}; do
        if kubectl get pods -n lws-system 2>/dev/null | grep -q "Running"; then
            LWS_READY=true
            break
        fi
        sleep 2
    done
    if [ "$LWS_READY" = true ]; then
        echo "  ✓ LeaderWorkerSet controller is running"
    else
        echo "  ⚠ LeaderWorkerSet controller not yet ready (may take longer)"
    fi

    # ---- Prometheus Operator CRDs (PodMonitor, ServiceMonitor) ----
    # llm-d Helm charts (llm-d-modelservice, inferencepool) include PodMonitor/ServiceMonitor
    # resources for metrics collection. These CRDs must exist before helmfile apply.
    # We install only the CRDs (not the full Prometheus Operator stack).
    PROM_OP_VERSION="v0.82.2"
    PROM_CRD_BASE="https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/${PROM_OP_VERSION}/example/prometheus-operator-crd"
    echo "Installing Prometheus Operator CRDs (${PROM_OP_VERSION})..."
    PROM_CRDS_OK=true
    for crd_file in monitoring.coreos.com_podmonitors.yaml monitoring.coreos.com_servicemonitors.yaml; do
        if ! kubectl apply --server-side -f "${PROM_CRD_BASE}/${crd_file}" 2>&1; then
            echo "  ✗ Failed to install ${crd_file}"
            PROM_CRDS_OK=false
        fi
    done
    if kubectl get crd podmonitors.monitoring.coreos.com &>/dev/null && \
       kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null; then
        echo "  ✓ Prometheus Operator CRDs verified (PodMonitor, ServiceMonitor)"
    else
        echo "  ✗ Prometheus Operator CRDs NOT found after installation"
        LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
    fi

    # ---- Gateway Control Plane (Istio) ----
    # llm-d requires a Gateway implementation. Install Istio as the default.
    # Ref: https://github.com/llm-d/llm-d/blob/main/guides/prereq/gateway-provider/README.md

    echo "Installing Istio Gateway control plane..."
    ISTIO_ALREADY_RUNNING=false
    if kubectl get pods -n istio-system 2>/dev/null | grep -q "Running"; then
        ISTIO_ALREADY_RUNNING=true
        echo "  ✓ Istio already running in istio-system"
    else
        # Use llm-d's provided gateway installation if repo is cloned
        LLM_D_REPO=~/llm-d
        if [ ! -d "$LLM_D_REPO/.git" ]; then
            echo "  Cloning llm-d repo for gateway setup..."
            git clone --depth 1 https://github.com/llm-d/llm-d.git "$LLM_D_REPO" 2>&1 || true
        fi

        if [ -f "$LLM_D_REPO/guides/prereq/gateway-provider/install-gateway-provider-dependencies.sh" ]; then
            echo "  Installing gateway dependencies via llm-d script..."
            cd "$LLM_D_REPO/guides/prereq/gateway-provider"
            bash install-gateway-provider-dependencies.sh 2>&1 || true
            cd - > /dev/null
        fi

        if [ -f "$LLM_D_REPO/guides/prereq/gateway-provider/istio.helmfile.yaml" ]; then
            echo "  Installing Istio via helmfile..."
            cd "$LLM_D_REPO/guides/prereq/gateway-provider"
            helmfile apply -f istio.helmfile.yaml 2>&1 || {
                echo "  ⚠ Istio helmfile installation may have issues"
            }
            cd - > /dev/null
        else
            # Fallback: install Istio directly
            echo "  llm-d repo not available, installing Istio via istioctl..."
            if ! command -v istioctl &>/dev/null; then
                curl -fsSL https://istio.io/downloadIstio | sh - 2>/dev/null || true
                ISTIO_DIR=$(ls -d istio-* 2>/dev/null | sort -V | tail -1)
                if [ -n "$ISTIO_DIR" ]; then
                    export PATH="$PWD/$ISTIO_DIR/bin:$PATH"
                fi
            fi
            if command -v istioctl &>/dev/null; then
                istioctl install --set profile=minimal -y 2>&1 || true
            else
                echo "  ✗ Could not install Istio (istioctl not available)"
                LLMD_INSTALL_ERRORS=$((LLMD_INSTALL_ERRORS + 1))
            fi
        fi

        # Wait for Istio to be ready
        echo "Waiting for Istio control plane..."
        for i in {1..30}; do
            if kubectl get pods -n istio-system 2>/dev/null | grep -q "Running"; then
                ISTIO_ALREADY_RUNNING=true
                break
            fi
            sleep 2
        done
        if [ "$ISTIO_ALREADY_RUNNING" = true ]; then
            echo "  ✓ Istio Gateway control plane is running"
        else
            echo "  ⚠ Istio not yet ready (may take longer)"
        fi
    fi

    # ---- NVIDIA GPU Operator ----

    echo "Installing NVIDIA GPU Operator..."
    helm repo add nvidia https://helm.ngc.nvidia.com/nvidia 2>&1 || true
    helm repo update 2>&1 || true
    
    kubectl create namespace gpu-operator --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
    
    # Install GPU Operator (driver.enabled=false assumes driver is pre-installed on GPU nodes)
    helm upgrade --install gpu-operator nvidia/gpu-operator \
        --namespace gpu-operator \
        --set driver.enabled=false \
        --set toolkit.enabled=true \
        --set devicePlugin.enabled=true \
        --set mig.strategy=single \
        --wait --timeout 5m 2>&1 || {
            echo "  ⚠ GPU Operator installation in progress (may take a few minutes)"
        }
    echo "  ✓ NVIDIA GPU Operator configured"
    
    # ---- Final Re-verification ----
    # Some components may have been installed as side effects
    # (e.g., install-gateway-provider-dependencies.sh installs GAIE CRDs)
    # Re-check and correct the error count.

    if [ "$LLMD_INSTALL_ERRORS" -gt 0 ]; then
        echo ""
        echo "Running final re-verification of all components..."
        FINAL_ERRORS=0

        command -v helm &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))
        command -v helmfile &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))
        command -v yq &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))

        kubectl get crd gateways.gateway.networking.k8s.io &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))
        kubectl get crd inferencepools.inference.networking.k8s.io &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))
        kubectl get crd leaderworkersets.leaderworkerset.x-k8s.io &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))
        kubectl get crd podmonitors.monitoring.coreos.com &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))
        kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null || FINAL_ERRORS=$((FINAL_ERRORS + 1))

        if kubectl get pods -n istio-system 2>/dev/null | grep -q "Running"; then
            : # ok
        elif kubectl get pods -n kgateway 2>/dev/null | grep -q "Running"; then
            : # ok
        else
            FINAL_ERRORS=$((FINAL_ERRORS + 1))
        fi

        if [ "$FINAL_ERRORS" -lt "$LLMD_INSTALL_ERRORS" ]; then
            echo "  Some previously failed components were resolved by subsequent steps."
        fi
        LLMD_INSTALL_ERRORS=$FINAL_ERRORS
    fi

    # ---- Summary ----

    echo ""
    if [ "$LLMD_INSTALL_ERRORS" -gt 0 ]; then
        echo "=========================================="
        echo "WARNING: ${LLMD_INSTALL_ERRORS} llm-d component(s) failed to install"
        echo "=========================================="
        echo "Check network connectivity and re-run the script with --llm-d option."
        echo ""
    else
        echo "llm-d infrastructure components installed successfully."
        echo ""
        echo "Installed components:"
        echo "  - Helm, helmfile, yq (client tools)"
        echo "  - Gateway API CRDs v1.4.0"
        echo "  - Gateway API Inference Extension CRDs v1.3.0 (InferencePool)"
        echo "  - LeaderWorkerSet CRD v0.7.0"
        echo "  - Istio Gateway control plane"
        echo "  - NVIDIA GPU Operator"
    fi
fi

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
    if [ "$LLMD_MODE" = true ]; then
        echo "SUCCESS: Kubernetes + llm-d setup complete!"
    else
        echo "SUCCESS: Kubernetes control plane setup complete!"
    fi
    echo "========================================"
    echo ""
    echo "[K8S_CONTROL_PLANE_IP]"
    echo "$ACCESS_IP"
    echo ""
    echo "[K8S_NODE_IP]"
    echo "$NODE_IP"
    echo ""
    if [ "$LLMD_MODE" = true ]; then
        echo "[K8S_MODE]"
        echo "llm-d"
        echo ""
    fi
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
    if [ "$LLMD_MODE" = true ]; then
        echo "For GPU Workers, first install NVIDIA driver:"
        echo "  ./installCudaDriver.sh --no-reboot"
        echo "  sudo reboot"
        echo ""
        echo "llm-d Deployment (after GPU workers join):"
        echo "  ./deploy-llm-d.sh --hf-token YOUR_HF_TOKEN"
        echo "  # Or for minimal setup (1 GPU):"
        echo "  ./deploy-llm-d.sh --replicas 1 --tp 1 --hf-token YOUR_HF_TOKEN"
        echo ""
    fi
    echo "External Kubeconfig (saved to ~/kubeconfig-external.yaml):"
    echo "  1. Copy to local: scp user@${ACCESS_IP}:~/kubeconfig-external.yaml ./kubeconfig.yaml"
    echo "  2. Use: export KUBECONFIG=./kubeconfig.yaml && kubectl get nodes"
    echo ""
    echo "Kubectl on this node:"
    echo "  kubectl get nodes"
    echo "  kubectl get pods -A"
    if [ "$LLMD_MODE" = true ]; then
        echo "  kubectl get gateway -A              # Gateway API resources"
        echo "  kubectl get inferencepool -A         # Inference Extension resources"
        echo "  kubectl get lws -A                   # LeaderWorkerSet resources"
        echo "  kubectl get pods -n istio-system      # Istio Gateway pods"
        echo "  kubectl get pods -n gpu-operator      # GPU Operator pods"
    fi
    echo ""
    exit 0
else
    echo "WARNING: Node is not ready yet. Please check with 'kubectl get nodes'"
    exit 1
fi
