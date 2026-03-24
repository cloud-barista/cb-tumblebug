# OpenBao server configuration for MC-Terrarium
#
# This configuration uses file-based persistent storage.
# Data survives container restarts via the Docker volume (openbao-data).
#
# Reference: https://openbao.org/docs/configuration/

# Persistent storage backend
storage "file" {
  path = "/openbao/data"
}

# TCP listener (TLS disabled for local development)
listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

# API address for self-reference
api_addr = "http://0.0.0.0:8200"

# Disable mlock for container compatibility
# (IPC_LOCK capability handles memory protection instead)
disable_mlock = true

# Enable Web UI (requires UI-enabled binary)
ui = true
