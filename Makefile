default: ## Build the application ('make' without arguments)
	cd src/ && $(MAKE)

run: ## Run the built application
	cd src/ && $(MAKE) run

clean: ## Clean build artifacts
	cd src/ && $(MAKE) clean

swag swagger: ## Generate Swagger documentation
	cd src/ && $(MAKE) swag

# ===== Initialization =====
SHELL := /bin/bash

init: ## Run initialization sequence (`make init ARGS="-y"` for headless; `TARGET=k8s` or `make k-init` for Kubernetes)
ifeq ($(TARGET),k8s)
	@$(MAKE) k-init
else
	@chmod +x ./init/multi-init.sh 2>/dev/null || true
	@./init/multi-init.sh $(ARGS)
endif

init-profile: ## Maintainer-only: run make init with elapsed/memory profiling outputs under tmp/init-profile/
	@chmod +x ./scripts/misc/init-profile.sh 2>/dev/null || true
	@./scripts/misc/init-profile.sh

# ===== Docker Compose Commands =====
# docker-compose.yaml includes all services + OpenBao.
#
# Usage scenarios:
#   1) Fresh start:       make up → make init
#   2) Restart:           make up
#   3) Reset DB only:     make clean-db → make up → make init
#   4) Full reset:        make clean-all → make up → make init
prepare-volumes: ## Create bind-mount directories with correct ownership
	@echo "Preparing container-volume directories..."
	@mkdir -p \
		container-volume/cb-tumblebug-container/meta_db \
		container-volume/cb-tumblebug-container/log \
		container-volume/cb-spider-container/meta_db \
		container-volume/cb-spider-container/log \
		container-volume/etcd/data \
		container-volume/openbao-data \
		container-volume/mc-terrarium-container/.terrarium \
		2>/dev/null || \
	sudo mkdir -p \
		container-volume/cb-tumblebug-container/meta_db \
		container-volume/cb-tumblebug-container/log \
		container-volume/cb-spider-container/meta_db \
		container-volume/cb-spider-container/log \
		container-volume/etcd/data \
		container-volume/openbao-data \
		container-volume/mc-terrarium-container/.terrarium
	@# Fix ownership for mc-terrarium volume (container runs as appuser, uid 1000)
	@if [ "$$(stat -c '%u' container-volume/mc-terrarium-container/.terrarium 2>/dev/null)" != "$$(id -u)" ]; then \
		echo "Fixing ownership of mc-terrarium volume..."; \
		sudo chown -R $$(id -u):$$(id -g) container-volume/mc-terrarium-container/.terrarium; \
	fi
	@# Fix root-owned runtime artifacts in assets/spider/ (created by cb-spider container).
	@# These block Docker build context transfer if not readable by the current user.
	@if find assets/spider/ -mindepth 1 -uid 0 -a -not -readable -print -quit 2>/dev/null | grep -q .; then \
		echo "Fixing permissions on cb-spider runtime artifacts in assets/spider/..."; \
		sudo find assets/spider/ -mindepth 1 -uid 0 -exec chown $$(id -u):$$(id -g) {} +; \
	fi
	@echo "Prepared!"
# Note: OpenBao data dir ownership is fixed by entrypoint chown in docker-compose.yaml.

compose: prepare-volumes ## Start Docker Compose services (auto init/unseal OpenBao)
	@echo "Starting OpenBao..."
	@DOCKER_BUILDKIT=1 docker compose up -d openbao
	@if [ ! -f .env ] || ! grep -q '^VAULT_TOKEN=.\+' .env 2>/dev/null; then \
		echo "VAULT_TOKEN not found — running first-time OpenBao initialization..."; \
		bash init/openbao/openbao-init.sh; \
	fi
	@$(MAKE) unseal
	@echo "Starting all services..."
	@DOCKER_BUILDKIT=1 docker compose up --build

logs: ## Follow Docker Compose logs (docker compose logs -f)
	docker compose logs -f

compose-down: ## Stop Docker Compose services (docker compose down)
	@echo "Stopping Docker Compose services..."
	docker compose down

status: ## Show status of Docker Compose services (docker compose ps)
	@docker compose ps --format "table {{.Name}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"

ps: ## Show status of services (alias for status)
	@$(MAKE) status

# ===== Database Cleanup Commands =====
clean-db: compose-down ## Clean all database metadata (./init/cleanDB.sh)
	@echo "Running cleanDB script..."
	@chmod +x ./init/cleanDB.sh 2>/dev/null || true
	@./init/cleanDB.sh

clean-all: compose-down clean-db ## Full reset including OpenBao (requires re-init)
	@echo "Cleaning OpenBao data..."
	@sudo rm -rf container-volume/openbao-data
	@rm -f init/openbao/secrets/openbao-init.json
	@rm -f secrets/openbao-init.json  # legacy location (old openbao-init.sh)
	@sed -i 's/^VAULT_TOKEN=.*/VAULT_TOKEN=/' .env 2>/dev/null || true
	@echo "Cleaned! Run 'make up' then 'make init' to re-initialize."

# ===== Database Backup & Restore =====
backup-assets: ## Backup PostgreSQL database to assets directory for version control
	@chmod +x ./scripts/backup-assets.sh 2>/dev/null || true
	@./scripts/backup-assets.sh

restore-assets: ## Restore PostgreSQL database from assets backup (or FILE=<path>)
	@chmod +x ./scripts/restore-assets.sh 2>/dev/null || true
	@if [ -z "$(FILE)" ]; then \
		./scripts/restore-assets.sh; \
	else \
		./scripts/restore-assets.sh $(FILE); \
	fi

# ===== Utility Aliases =====
# TARGET=k8s switches up/init/down to the Kubernetes (Helm) deployment.
# Default (no TARGET) keeps the existing docker compose behavior unchanged.
up: ## Start all services (compose; `make up TARGET=k8s` or `make k-up` for Kubernetes)
ifeq ($(TARGET),k8s)
	@$(MAKE) k-up
else
	@$(MAKE) compose
endif

down: ## Quick stop (compose; `make down TARGET=k8s` or `make k-down` for Kubernetes)
ifeq ($(TARGET),k8s)
	@$(MAKE) k-down
else
	@$(MAKE) compose-down
endif

# ===== Kubernetes (Helm) Deployment =====
# Short k-* commands mirror the compose verbs:
#   up/down/init/ps/logs/clean-db+clean-all  ->  k-up/k-down/k-init/k-status(=k-ps)/k-logs/k-clean
# Works against any cluster in the current kubeconfig context (kind is only the
# local-dev fallback). Pin a context explicitly with: make k-up K8S_CONTEXT=<ctx>
K8S_NAMESPACE ?= cb-tumblebug
HELM_RELEASE ?= cb-tumblebug
HELM_CHART := deployments/helm/cb-tumblebug
K8S_INIT_PORT ?= 11323
K8S_CONTEXT ?=
KIND_CLUSTER_NAME ?= cb-tumblebug
KUBECTL := kubectl$(if $(K8S_CONTEXT), --context $(K8S_CONTEXT))
HELM := helm$(if $(K8S_CONTEXT), --kube-context $(K8S_CONTEXT))
# Persistent local toggles (survive future k-up runs; gitignored)
K8S_MCP_VALUES := deployments/helm/cb-tumblebug/values-mcp.yaml
MCP_VALUES_FLAG = $(if $(wildcard $(K8S_MCP_VALUES)),-f $(K8S_MCP_VALUES))
K8S_AGW_VALUES := deployments/helm/cb-tumblebug/values-agentgateway.yaml
AGW_VALUES_FLAG = $(if $(wildcard $(K8S_AGW_VALUES)),-f $(K8S_AGW_VALUES))
K8S_AGW_AUTH_VALUES := deployments/helm/cb-tumblebug/values-agentgateway-auth.yaml
AGW_AUTH_VALUES_FLAG = $(if $(wildcard $(K8S_AGW_AUTH_VALUES)),-f $(K8S_AGW_AUTH_VALUES))
# Local JWT signing key for MCP auth (private key never leaves this host)
MCP_AUTH_DIR := $(HOME)/.cloud-barista/mcp-auth
MCP_JWT_ISSUER ?= cb-tumblebug-dev
MCP_JWT_AUDIENCE ?= cb-tumblebug-mcp
MCP_TOKEN_TTL_HOURS ?= 720
K8S_GW_VALUES := deployments/helm/cb-tumblebug/values-gateway.yaml
GW_VALUES_FLAG = $(if $(wildcard $(K8S_GW_VALUES)),-f $(K8S_GW_VALUES))
ENVOY_GATEWAY_VERSION ?= v1.8.3
# Local-source dev images (compose `build:` equivalent): one state file per component
DEV_IMAGE_FLAGS = $(foreach f,$(wildcard deployments/helm/cb-tumblebug/values-dev-image-*.yaml),-f $(f))

k-up: ## Install/upgrade the stack on Kubernetes (creates a kind cluster if no cluster is reachable)
	@command -v helm >/dev/null || { echo "helm is required: https://helm.sh/docs/intro/install/"; exit 1; }
	@command -v kubectl >/dev/null || { echo "kubectl is required"; exit 1; }
	@if ! $(KUBECTL) cluster-info >/dev/null 2>&1; then \
		if command -v kind >/dev/null; then \
			echo "No reachable cluster. Creating kind cluster '$(KIND_CLUSTER_NAME)'..."; \
			kind create cluster --name $(KIND_CLUSTER_NAME); \
		else \
			echo "No reachable Kubernetes cluster and 'kind' is not installed."; \
			echo "Install kind (https://kind.sigs.k8s.io) or configure kubectl context."; \
			exit 1; \
		fi; \
	fi
	@# Show the install target; confirm when it is not a local kind cluster
	@ctx="$(K8S_CONTEXT)"; [ -n "$$ctx" ] || ctx=$$(kubectl config current-context 2>/dev/null); \
	server=$$($(KUBECTL) config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null); \
	echo "Target cluster: context '$$ctx' ($$server), namespace '$(K8S_NAMESPACE)'"; \
	case "$$ctx" in \
		kind-*) ;; \
		*) if [ -t 0 ]; then \
			read -p "This is NOT a kind cluster. Install into it? (y/N): " ans; \
			case "$$ans" in [yY]*) ;; *) echo "Aborted."; exit 1;; esac; \
		else \
			echo "WARNING: installing into a non-kind cluster (non-interactive mode; proceeding)"; \
		fi ;; \
	esac
	@# Preflight: PVCs need a default StorageClass unless storageClassName values are given
	@if ! echo '$(HELM_ARGS)' | grep -q storageClassName; then \
		if ! $(KUBECTL) get storageclass 2>/dev/null | grep -q '(default)'; then \
			echo "ERROR: this cluster has no default StorageClass — PVCs would stay Pending forever."; \
			echo "  Fix one of:"; \
			echo "   - mark a StorageClass as default (storageclass.kubernetes.io/is-default-class=true)"; \
			echo "   - pass explicit classes: make k-up HELM_ARGS=\"--set etcd.storageClassName=<sc> --set tumblebugPostgres.storageClassName=<sc> --set spiderPostgres.storageClassName=<sc> --set openbao.storageClassName=<sc> --set terrarium.storageClassName=<sc>\""; \
			exit 1; \
		fi; \
	fi
	@# Preflight: warn if every node is control-plane-tainted (common on single-node kubeadm)
	@total=$$($(KUBECTL) get nodes --no-headers 2>/dev/null | wc -l); \
	tainted=$$($(KUBECTL) get nodes -o jsonpath='{range .items[*]}{range .spec.taints[*]}{.key}:{.effect} {end}{"\n"}{end}' 2>/dev/null | grep -c 'node-role.kubernetes.io/\(control-plane\|master\):NoSchedule' || true); \
	if [ "$$total" -gt 0 ] && [ "$$tainted" -ge "$$total" ]; then \
		echo "WARNING: all $$total node(s) carry the control-plane NoSchedule taint — pods will stay Pending."; \
		echo "  Single-node cluster fix: kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule-"; \
	fi
	@if $(HELM) status $(HELM_RELEASE) -n $(K8S_NAMESPACE) >/dev/null 2>&1; then \
		rev=$$($(HELM) list -n $(K8S_NAMESPACE) --filter '^$(HELM_RELEASE)$$' 2>/dev/null | tail -1 | awk '{print $$3}'); \
		echo "Existing release found (revision $$rev) — upgrading in place (pods restart only if templates changed)..."; \
	else \
		echo "No existing release — fresh install into namespace '$(K8S_NAMESPACE)'..."; \
	fi
	@# --reset-values: state comes only from chart defaults + current flags
	@# (otherwise helm silently carries over user-supplied values from the previous revision)
	@$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(K8S_NAMESPACE) --create-namespace --timeout 10m \
		--reset-values $(MCP_VALUES_FLAG) $(AGW_VALUES_FLAG) $(AGW_AUTH_VALUES_FLAG) $(GW_VALUES_FLAG) $(DEV_IMAGE_FLAGS) $(HELM_ARGS) || \
		{ $(MAKE) --no-print-directory k-diagnose; exit 1; }
	@echo ""
	@echo "Waiting for pods to become Ready (timeout 10m)..."
	@$(KUBECTL) wait pods -l app.kubernetes.io/part-of=cb-tumblebug \
		--for=condition=Ready -n $(K8S_NAMESPACE) --timeout=600s || \
		{ $(MAKE) --no-print-directory k-diagnose; exit 1; }
	@$(MAKE) --no-print-directory k-status
	@echo ""
	@echo "Next: make k-init   (first deployment only — register credentials & load assets)"
	@echo ""
	@$(MAKE) --no-print-directory k-info

# Internal: dump why the deployment is unhealthy (used by k-up failure paths)
k-diagnose:
	@echo ""
	@echo "=== Deployment diagnostics (namespace $(K8S_NAMESPACE)) ==="
	@echo "--- Pods not Running/Completed:"
	@$(KUBECTL) get pods -n $(K8S_NAMESPACE) 2>/dev/null | grep -vE "Running|Completed" || echo "  (none)"
	@echo "--- PVC status (Pending = no usable StorageClass?):"
	@$(KUBECTL) get pvc -n $(K8S_NAMESPACE) 2>/dev/null || true
	@echo "--- Recent Warning events:"
	@$(KUBECTL) get events -n $(K8S_NAMESPACE) --field-selector type=Warning \
		--sort-by=.lastTimestamp 2>/dev/null | tail -12 || true
	@echo "--- openbao-init job logs (if any):"
	@$(KUBECTL) logs -n $(K8S_NAMESPACE) job/openbao-init --tail=10 2>/dev/null || echo "  (no job logs)"
	@echo "Check again later with: make k-status"

k-init: ## Run initialization against the Kubernetes deployment (port-forward + headless-capable init)
	@echo "Port-forwarding cb-tumblebug ($(K8S_INIT_PORT) -> 1323)..."
	@$(KUBECTL) port-forward -n $(K8S_NAMESPACE) svc/cb-tumblebug $(K8S_INIT_PORT):1323 >/dev/null 2>&1 & \
	PF_PID=$$!; \
	trap "kill $$PF_PID 2>/dev/null" EXIT; \
	sleep 2; \
	chmod +x ./init/multi-init.sh 2>/dev/null || true; \
	TUMBLEBUG_SERVER=localhost:$(K8S_INIT_PORT) \
	ASSETS_PG_BACKEND=kubectl \
	TB_K8S_NAMESPACE=$(K8S_NAMESPACE) \
	./init/multi-init.sh $(ARGS)

k-down: ## Uninstall the Helm release and wait for pods to terminate (PVCs/data are kept)
	$(HELM) uninstall $(HELM_RELEASE) -n $(K8S_NAMESPACE) || true
	@$(KUBECTL) delete job openbao-init -n $(K8S_NAMESPACE) --ignore-not-found 2>/dev/null || true
	@$(MAKE) --no-print-directory k-port-forward-stop
	@echo "Waiting for pods to terminate (timeout 2m)..."
	@$(KUBECTL) wait --for=delete pods -l app.kubernetes.io/part-of=cb-tumblebug \
		-n $(K8S_NAMESPACE) --timeout=120s 2>/dev/null || \
		{ echo "Some pods are still terminating — check later with: make k-status"; }
	@echo "Stopped. Data PVCs are kept (restart: make k-up). Full reset: make k-clean"

k-clean: ## Full K8s reset: uninstall + delete PVCs and OpenBao key Secret
	@$(HELM) uninstall $(HELM_RELEASE) -n $(K8S_NAMESPACE) 2>/dev/null || true
	@$(KUBECTL) delete job openbao-init -n $(K8S_NAMESPACE) --ignore-not-found 2>/dev/null || true
	$(KUBECTL) delete pvc --all -n $(K8S_NAMESPACE) 2>/dev/null || true
	@echo "Waiting for PVC deletion to complete (prevents volume reuse races on the next k-up)..."
	@$(KUBECTL) wait --for=delete pvc --all -n $(K8S_NAMESPACE) --timeout=180s 2>/dev/null || true
	$(KUBECTL) delete secret openbao-keys -n $(K8S_NAMESPACE) 2>/dev/null || true
	@$(MAKE) --no-print-directory k-port-forward-stop
	@echo "Cleaned. Run 'make k-up' then 'make k-init' to re-deploy."
	@echo "(a kind cluster is kept; remove with: kind delete cluster --name $(KIND_CLUSTER_NAME))"

k-port-forward: ## Start port-forwards for API (1323) and MapUI (1324); idempotent (restarts stale ones)
	@$(MAKE) --no-print-directory k-port-forward-stop
	@$(KUBECTL) port-forward -n $(K8S_NAMESPACE) svc/cb-tumblebug 1323:1323 >/dev/null 2>&1 &
	@$(KUBECTL) get svc cb-mapui -n $(K8S_NAMESPACE) >/dev/null 2>&1 && \
		{ $(KUBECTL) port-forward -n $(K8S_NAMESPACE) svc/cb-mapui 1324:1324 >/dev/null 2>&1 & } || true
	@sleep 2
	@echo "Port-forwards started:"
	@echo "  API / Swagger : http://localhost:1323/tumblebug/api"
	@echo "  MapUI         : http://localhost:1324"
	@echo "Stop with: make k-port-forward-stop"
	@$(KUBECTL) get svc cb-tumblebug-mcp-server -n $(K8S_NAMESPACE) >/dev/null 2>&1 && \
		echo "MCP is enabled — connection guide: make k-info" || true
	@$(KUBECTL) get svc -n envoy-gateway-system \
		-l gateway.envoyproxy.io/owning-gateway-name=cb-tumblebug-gateway -o name 2>/dev/null | grep -q . && \
		echo "Gateway entrypoint available: make k-gateway-forward (single URL for MapUI/API/MCP)" || true

k-mcp-on: ## Enable the MCP server (persists across future k-up runs)
	@printf 'mcp:\n  enabled: true\n' > $(K8S_MCP_VALUES)
	@echo "MCP server enabled (persisted in $(K8S_MCP_VALUES)). Applying..."
	@$(MAKE) --no-print-directory k-up

k-mcp-off: ## Disable the MCP server (also disables agentgateway, which depends on it)
	@rm -f $(K8S_MCP_VALUES) $(K8S_AGW_VALUES) $(K8S_AGW_AUTH_VALUES)
	@echo "MCP server (and agentgateway, if enabled) disabled. Applying..."
	@$(MAKE) --no-print-directory k-up

k-agentgateway-on: ## Enable agentgateway in front of the MCP server (enables MCP too; persistent)
	@printf 'mcp:\n  enabled: true\n' > $(K8S_MCP_VALUES)
	@printf 'agentgateway:\n  enabled: true\n' > $(K8S_AGW_VALUES)
	@echo "agentgateway + MCP enabled (persisted). Applying..."
	@$(MAKE) --no-print-directory k-up

k-info: ## Show access endpoints & LLM-client setup based on what is currently enabled
	@gw=$$($(KUBECTL) get svc -n envoy-gateway-system \
		-l gateway.envoyproxy.io/owning-gateway-name=cb-tumblebug-gateway -o name 2>/dev/null | head -1); \
	mcp=$$($(KUBECTL) get svc cb-tumblebug-mcp-server -n $(K8S_NAMESPACE) -o name 2>/dev/null); \
	agw=$$($(KUBECTL) get svc agentgateway -n $(K8S_NAMESPACE) -o name 2>/dev/null); \
	echo "=== Access guide (based on what is enabled) ==="; \
	if [ -n "$$gw" ]; then \
		routes="/ MapUI | /tumblebug API+Swagger"; \
		[ -n "$$mcp" ] && routes="$$routes | /mcp MCP"; \
		echo "Single entrypoint (recommended): make k-gateway-forward"; \
		echo "  -> http://localhost:8080 ($$routes)"; \
		if $(KUBECTL) get -n envoy-gateway-system $$gw -o jsonpath='{.spec.ports[*].port}' 2>/dev/null | grep -qw 443; then \
			echo "  -> https://localhost:8443 (same routes; self-signed cert — browser OK, MCP clients may reject)"; \
		fi; \
		echo "  Note: localhost http is safe here — kubectl port-forward tunnels traffic inside TLS to the cluster."; \
	else \
		echo "Per-service access: make k-port-forward  (API/Swagger :1323, MapUI :1324)"; \
	fi; \
	auth=0; $(KUBECTL) get configmap agentgateway-jwks -n $(K8S_NAMESPACE) >/dev/null 2>&1 && auth=1; \
	if [ -n "$$mcp" ]; then \
		if [ -n "$$gw" ]; then \
			$(MAKE) --no-print-directory k-mcp-client-info MCP_URL=http://localhost:8080/mcp MCP_AUTH=$$auth; \
			echo '  (after: make k-gateway-forward)'; \
		elif [ -n "$$agw" ]; then \
			$(MAKE) --no-print-directory k-mcp-client-info MCP_URL=http://localhost:3000/mcp MCP_AUTH=$$auth; \
			echo "  (after: kubectl port-forward -n $(K8S_NAMESPACE) svc/agentgateway 3000:3000)"; \
		else \
			$(MAKE) --no-print-directory k-mcp-client-info MCP_URL=http://localhost:8000/mcp MCP_AUTH=0; \
			echo "  (after: kubectl port-forward -n $(K8S_NAMESPACE) svc/cb-tumblebug-mcp-server 8000:8000)"; \
		fi; \
	fi

# Internal: print LLM-client connection snippets for a given MCP_URL (MCP_AUTH=1 adds JWT headers)
k-mcp-client-info:
	@echo ""
ifeq ($(MCP_AUTH),1)
	@echo "Connect your LLM client to $(MCP_URL) (streamable HTTP, JWT auth ON — mint: make k-mcp-token):"
	@echo '  VS Code / Copilot (.vscode/mcp.json):'
	@echo '      { "servers": { "cb-tumblebug": { "type": "http", "url": "$(MCP_URL)",'
	@echo '          "headers": { "Authorization": "Bearer <TOKEN>" } } } }'
	@echo '  Claude Code (CLI):'
	@echo '      TOKEN=$$(make -s k-mcp-token | grep -o "eyJ[A-Za-z0-9_.-]*")'
	@echo '      claude mcp add --transport http cb-tumblebug $(MCP_URL) --header "Authorization: Bearer $$TOKEN"'
	@echo '  Cursor (~/.cursor/mcp.json):'
	@echo '      { "mcpServers": { "cb-tumblebug": { "url": "$(MCP_URL)",'
	@echo '          "headers": { "Authorization": "Bearer <TOKEN>" } } } }'
	@echo '  MCP Inspector: Authentication > Bearer Token = <TOKEN> (token only, no "Bearer " prefix)'
else
	@echo "Connect your LLM client to $(MCP_URL) (streamable HTTP):"
	@echo '  VS Code / Copilot (.vscode/mcp.json):'
	@echo '      { "servers": { "cb-tumblebug": { "type": "http", "url": "$(MCP_URL)" } } }'
	@echo '  Claude Code (CLI):'
	@echo '      claude mcp add --transport http cb-tumblebug $(MCP_URL)'
	@echo '  Cursor (~/.cursor/mcp.json):'
	@echo '      { "mcpServers": { "cb-tumblebug": { "url": "$(MCP_URL)" } } }'
endif
	@echo '  Claude Desktop: run the stdio<->HTTP bridge ON YOUR DESKTOP (a Claude Desktop'
	@echo '      client limitation, same for compose) — see src/interface/mcp/README.md'
	@echo '      (mcp-simple-proxy.py + claude_desktop_config.json example)'
	@echo '  Test/debug with MCP Inspector:'
	@echo '      npx @modelcontextprotocol/inspector'
	@echo '        -> open the printed URL; Transport: "Streamable HTTP", URL: $(MCP_URL)'
	@echo '      quick CLI check: npx @modelcontextprotocol/inspector --cli $(MCP_URL) --method tools/list'
	@echo '  Security note: the MCP endpoint itself is UNAUTHENTICATED (PoC) — tools act with'
	@echo '      the TB API credentials embedded in the server. Locally it is reachable only'
	@echo '      via kubectl port-forward; add gateway-level auth before any external exposure.'

k-mcp-auth-on: ## Enable JWT auth on the MCP route (local key; mint tokens with k-mcp-token; persistent)
	@[ -f $(K8S_AGW_VALUES) ] || { echo "agentgateway is not enabled — run 'make k-agentgateway-on' first."; exit 1; }
	@mkdir -p $(MCP_AUTH_DIR) && chmod 700 $(MCP_AUTH_DIR)
	@if [ ! -f $(MCP_AUTH_DIR)/key.pem ]; then \
		echo "Generating RSA signing key ($(MCP_AUTH_DIR)/key.pem)..."; \
		openssl genrsa -out $(MCP_AUTH_DIR)/key.pem 2048 2>/dev/null && chmod 600 $(MCP_AUTH_DIR)/key.pem; \
	fi
	@openssl rsa -in $(MCP_AUTH_DIR)/key.pem -pubout -out $(MCP_AUTH_DIR)/pub.pem 2>/dev/null
	@n=$$(openssl rsa -pubin -in $(MCP_AUTH_DIR)/pub.pem -noout -modulus | cut -d= -f2 | xxd -r -p | base64 -w0 | tr '+/' '-_' | tr -d '='); \
	jwks="{\"keys\":[{\"kty\":\"RSA\",\"kid\":\"cb-tb\",\"use\":\"sig\",\"alg\":\"RS256\",\"n\":\"$$n\",\"e\":\"AQAB\"}]}"; \
	printf 'agentgateway:\n  auth:\n    enabled: true\n    jwks: '"'"'%s'"'"'\n' "$$jwks" > $(K8S_AGW_AUTH_VALUES)
	@echo "MCP JWT auth enabled (persisted). Applying..."
	@$(MAKE) --no-print-directory k-up
	@echo ""
	@echo "Mint a token with: make k-mcp-token"

k-mcp-auth-off: ## Disable JWT auth on the MCP route (key file is kept)
	@rm -f $(K8S_AGW_AUTH_VALUES)
	@echo "MCP JWT auth disabled. Applying..."
	@$(MAKE) --no-print-directory k-up

k-mcp-token: ## Mint a dev JWT for the MCP endpoint (MCP_TOKEN_TTL_HOURS=$(MCP_TOKEN_TTL_HOURS))
	@[ -f $(MCP_AUTH_DIR)/key.pem ] || { echo "No signing key — run 'make k-mcp-auth-on' first."; exit 1; }
	@b64url() { base64 -w0 | tr '+/' '-_' | tr -d '='; }; \
	now=$$(date +%s); exp=$$((now + $(MCP_TOKEN_TTL_HOURS)*3600)); \
	h=$$(printf '{"alg":"RS256","typ":"JWT","kid":"cb-tb"}' | b64url); \
	p=$$(printf '{"iss":"$(MCP_JWT_ISSUER)","aud":"$(MCP_JWT_AUDIENCE)","sub":"dev","iat":%s,"exp":%s}' "$$now" "$$exp" | b64url); \
	s=$$(printf '%s.%s' "$$h" "$$p" | openssl dgst -sha256 -sign $(MCP_AUTH_DIR)/key.pem -binary | b64url); \
	echo "Token (valid until $$(date -d @$$exp '+%Y-%m-%d %H:%M' 2>/dev/null || date -r $$exp)):"; \
	echo ""; \
	echo "$$h.$$p.$$s"; \
	echo ""; \
	echo "Use as: Authorization: Bearer <token>   (Inspector: paste token only, no 'Bearer ' prefix)"

k-agentgateway-off: ## Disable agentgateway (MCP server stays enabled)
	@rm -f $(K8S_AGW_VALUES) $(K8S_AGW_AUTH_VALUES)
	@echo "agentgateway disabled (MCP server kept). Applying..."
	@$(MAKE) --no-print-directory k-up

k-gateway-on: ## Enable the Gateway API entrypoint (/, /tumblebug, /mcp); installs Envoy Gateway on kind if missing
	@if ! $(KUBECTL) get crd gateways.gateway.networking.k8s.io >/dev/null 2>&1; then \
		ctx="$(K8S_CONTEXT)"; [ -n "$$ctx" ] || ctx=$$(kubectl config current-context 2>/dev/null); \
		case "$$ctx" in \
		kind-*) \
			echo "No Gateway API implementation found — installing Envoy Gateway $(ENVOY_GATEWAY_VERSION)..."; \
			$(HELM) install eg oci://docker.io/envoyproxy/gateway-helm --version $(ENVOY_GATEWAY_VERSION) \
				-n envoy-gateway-system --create-namespace --wait --timeout 6m ;; \
		*) \
			echo "No Gateway API implementation found in this cluster."; \
			echo "Install one first (e.g., Envoy Gateway):"; \
			echo "  helm install eg oci://docker.io/envoyproxy/gateway-helm --version $(ENVOY_GATEWAY_VERSION) -n envoy-gateway-system --create-namespace"; \
			exit 1 ;; \
		esac; \
	fi
	@printf 'gateway:\n  enabled: true\n' > $(K8S_GW_VALUES)
	@echo "Gateway entrypoint enabled (persisted). Applying..."
	@$(MAKE) --no-print-directory k-up
	@echo ""
	@echo "Single entrypoint: make k-gateway-forward   (http://localhost:8080 -> / mapui, /tumblebug API, /mcp MCP)"

k-gateway-off: ## Disable the Gateway API entrypoint (the implementation/controller is left installed)
	@rm -f $(K8S_GW_VALUES)
	@echo "Gateway entrypoint disabled. Applying..."
	@$(MAKE) --no-print-directory k-up

k-build-tb: ## Build local cb-tumblebug source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=cb-tumblebug
k-build-mapui: ## Build local cb-mapui source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=cb-mapui
k-build-mcp: ## Build local MCP server source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=mcp
k-build-sp: ## Build local cb-spider source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=cb-spider

k-build: ## Build LOCAL source and run it in the cluster (C=tb|mapui|mcp) — compose `--build` equivalent
	@case "$(C)" in \
		cb-tumblebug|tb)      canon="cb-tumblebug"; ctx="."; key="tumblebug"; deploy="cb-tumblebug" ;; \
		cb-mapui|mapui)       canon="cb-mapui"; ctx="../cb-mapui"; key="mapui"; deploy="cb-mapui" ;; \
		mcp)                  canon="mcp"; ctx="src/interface/mcp"; key="mcp"; deploy="cb-tumblebug-mcp-server" ;; \
		cb-spider|sp|spider)  canon="cb-spider"; ctx="../cb-spider"; key="spider"; deploy="cb-spider" ;; \
		*) echo "Usage: make k-build C=tb|mapui|mcp|sp   (or: make k-build-tb / k-build-mapui / k-build-mcp / k-build-sp)"; exit 1 ;; \
	esac; \
	ctx2="$$ctx"; key2="$$key"; deploy2="$$deploy"; \
	img="cloudbaristaorg/$$canon:local-dev"; \
	ctxk="$(K8S_CONTEXT)"; [ -n "$$ctxk" ] || ctxk=$$(kubectl config current-context 2>/dev/null); \
	case "$$ctxk" in kind-*) ;; *) \
		echo "k-build loads images via 'kind load' — current context '$$ctxk' is not kind."; \
		echo "For remote clusters push to a registry and set images.$$key2 instead."; exit 1 ;; \
	esac; \
	echo "Building $$img from $$ctx2 ..."; \
	docker build -t "$$img" "$$ctx2" && \
	kind load docker-image --name $(KIND_CLUSTER_NAME) "$$img" && \
	printf 'images:\n  %s: %s\n' "$$key2" "$$img" > deployments/helm/cb-tumblebug/values-dev-image-$$canon.yaml && \
	$(MAKE) --no-print-directory k-up && \
	echo "Restarting deploy/$$deploy2 to pick up the rebuilt image..." && \
	$(KUBECTL) rollout restart deploy/$$deploy2 -n $(K8S_NAMESPACE) && \
	$(KUBECTL) rollout status deploy/$$deploy2 -n $(K8S_NAMESPACE) --timeout=600s && \
	echo "Local build active for $$canon (persists across k-up). Revert: make k-build-off C=$$canon"

k-build-off: ## Revert to published images (C=tb|mapui|mcp for one; omit C for all)
	@case "$(C)" in \
		"") rm -f deployments/helm/cb-tumblebug/values-dev-image-*.yaml ;; \
		cb-tumblebug|tb)     rm -f deployments/helm/cb-tumblebug/values-dev-image-cb-tumblebug.yaml ;; \
		cb-mapui|mapui)      rm -f deployments/helm/cb-tumblebug/values-dev-image-cb-mapui.yaml ;; \
		mcp)                 rm -f deployments/helm/cb-tumblebug/values-dev-image-mcp.yaml ;; \
		cb-spider|sp|spider) rm -f deployments/helm/cb-tumblebug/values-dev-image-cb-spider.yaml ;; \
		*) echo "Unknown component: $(C) (tb|mapui|mcp|sp)"; exit 1 ;; \
	esac
	@echo "Reverting to published image(s). Applying..."
	@$(MAKE) --no-print-directory k-up

k-gateway-forward: ## Port-forward the gateway entrypoint to localhost:8080 (+8443 when TLS is on; idempotent)
	@pids=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && $$3 == "port-forward" && /envoy-gateway-system/{print $$1}' | xargs); \
	[ -z "$$pids" ] || kill $$pids 2>/dev/null || true
	@svc=$$($(KUBECTL) get svc -n envoy-gateway-system \
		-l gateway.envoyproxy.io/owning-gateway-name=cb-tumblebug-gateway -o name 2>/dev/null | head -1); \
	[ -n "$$svc" ] || { echo "Gateway service not found — run 'make k-gateway-on' first."; exit 1; }; \
	$(KUBECTL) port-forward -n envoy-gateway-system $$svc 8080:80 >/dev/null 2>&1 & \
	svc=$$($(KUBECTL) get svc -n envoy-gateway-system \
		-l gateway.envoyproxy.io/owning-gateway-name=cb-tumblebug-gateway -o name 2>/dev/null | head -1); \
	if $(KUBECTL) get -n envoy-gateway-system $$svc -o jsonpath='{.spec.ports[*].port}' 2>/dev/null | grep -qw 443; then \
		$(KUBECTL) port-forward -n envoy-gateway-system $$svc 8443:443 >/dev/null 2>&1 & \
		https=" / https://localhost:8443"; \
	fi; \
	sleep 2; \
	echo "Gateway entrypoint: http://localhost:8080$$https  (/ mapui, /tumblebug API, /mcp MCP)"

K8S_TOKEN_FILE ?= $(HOME)/.cloud-barista/k8s-admin.token

k-token: ## Create an admin token file for K8s UIs like Headlamp (cluster-admin; local dev use)
	@$(KUBECTL) create serviceaccount headlamp-admin -n kube-system 2>/dev/null || true
	@$(KUBECTL) create clusterrolebinding headlamp-admin --clusterrole=cluster-admin \
		--serviceaccount=kube-system:headlamp-admin 2>/dev/null || true
	@printf '%s\n' \
		'apiVersion: v1' \
		'kind: Secret' \
		'metadata:' \
		'  name: headlamp-admin-token' \
		'  namespace: kube-system' \
		'  annotations:' \
		'    kubernetes.io/service-account.name: headlamp-admin' \
		'type: kubernetes.io/service-account-token' | $(KUBECTL) apply -f - >/dev/null
	@mkdir -p $(dir $(K8S_TOKEN_FILE))
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		tok=$$($(KUBECTL) get secret headlamp-admin-token -n kube-system -o jsonpath='{.data.token}' 2>/dev/null | base64 -d); \
		[ -n "$$tok" ] && { printf '%s' "$$tok" > $(K8S_TOKEN_FILE); break; }; \
		sleep 1; \
	done
	@[ -s $(K8S_TOKEN_FILE) ] || { echo "Failed to obtain token — is the cluster reachable?"; exit 1; }
	@chmod 600 $(K8S_TOKEN_FILE)
	@$(KUBECTL) --token="$$(cat $(K8S_TOKEN_FILE))" get ns >/dev/null 2>&1 && \
		echo "Admin token saved: $(K8S_TOKEN_FILE) (verified)" || \
		echo "Admin token saved: $(K8S_TOKEN_FILE) (verification failed — check cluster)"
	@echo "Use it to log in to Headlamp or other K8s UIs."
	@echo "Note: cluster-admin, no expiry — local dev only. Revoke: $(KUBECTL) delete secret headlamp-admin-token -n kube-system"

k-port-forward-stop: ## Stop this deployment's port-forwards incl. the gateway entrypoint (others untouched)
	@pids=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && $$3 == "port-forward" && \
		(/ $(K8S_NAMESPACE) / || (/envoy-gateway-system/ && /cb-tumblebug-gateway/)) {print $$1}' | xargs); \
	if [ -n "$$pids" ]; then \
		echo "Stopping port-forwards for this deployment (PID: $$pids)"; \
		kill $$pids 2>/dev/null || true; \
	fi

k-status: ## Show K8s deployment status (release/pods/services/port-forwards)
	@if $(HELM) status $(HELM_RELEASE) -n $(K8S_NAMESPACE) >/dev/null 2>&1; then \
		$(HELM) list -n $(K8S_NAMESPACE) --filter '^$(HELM_RELEASE)$$' 2>/dev/null | tail -1 | \
			awk '{print "Helm release: " $$1 " (" $$8 ", revision " $$3 ", updated " $$4 " " $$5 ")"}'; \
	else \
		echo "Helm release: not installed — run 'make k-up'"; \
	fi
	@echo ""
	@$(KUBECTL) get pods,svc -n $(K8S_NAMESPACE) 2>/dev/null || true
	@echo ""
	@echo "Active port-forwards (may be stale after pod restarts — refresh with: make k-port-forward):"
	@pf=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && $$3 == "port-forward"'); \
	if [ -n "$$pf" ]; then \
		echo "$$pf" | sed 's/^/  /'; \
	else \
		echo "  (none) — start with: make k-port-forward"; \
	fi

k-ps: ## Show K8s deployment status (alias for k-status)
	@$(MAKE) --no-print-directory k-status

k-logs: ## Show per-component log commands (`make k-logs C=cb-spider` to follow one)
ifeq ($(C),)
	@echo "Per-component logs (namespace: $(K8S_NAMESPACE)) — follow one with: make k-logs C=<name>"
	@echo ""
	@echo "  cb-tumblebug   $(KUBECTL) logs -n $(K8S_NAMESPACE) -f deploy/cb-tumblebug"
	@echo "  cb-spider      $(KUBECTL) logs -n $(K8S_NAMESPACE) -f deploy/cb-spider"
	@echo "  mc-terrarium   $(KUBECTL) logs -n $(K8S_NAMESPACE) -f deploy/mc-terrarium"
	@echo "  cb-mapui       $(KUBECTL) logs -n $(K8S_NAMESPACE) -f deploy/cb-mapui"
	@echo "  etcd           $(KUBECTL) logs -n $(K8S_NAMESPACE) -f sts/cb-tumblebug-etcd"
	@echo "  tb-postgres    $(KUBECTL) logs -n $(K8S_NAMESPACE) -f sts/cb-tumblebug-postgres"
	@echo "  sp-postgres    $(KUBECTL) logs -n $(K8S_NAMESPACE) -f sts/cb-spider-postgres"
	@echo "  openbao        $(KUBECTL) logs -n $(K8S_NAMESPACE) -f openbao-0 -c openbao"
	@echo "  openbao-init   $(KUBECTL) logs -n $(K8S_NAMESPACE) job/openbao-init"
	@echo ""
	@echo "  all (merged)   $(KUBECTL) logs -n $(K8S_NAMESPACE) -f -l app.kubernetes.io/part-of=cb-tumblebug --prefix --max-log-requests=10"
else ifeq ($(C),etcd)
	$(KUBECTL) logs -n $(K8S_NAMESPACE) -f sts/cb-tumblebug-etcd
else ifeq ($(C),tb-postgres)
	$(KUBECTL) logs -n $(K8S_NAMESPACE) -f sts/cb-tumblebug-postgres
else ifeq ($(C),sp-postgres)
	$(KUBECTL) logs -n $(K8S_NAMESPACE) -f sts/cb-spider-postgres
else ifeq ($(C),openbao)
	$(KUBECTL) logs -n $(K8S_NAMESPACE) -f openbao-0 -c openbao
else ifeq ($(C),openbao-init)
	$(KUBECTL) logs -n $(K8S_NAMESPACE) job/openbao-init
else
	$(KUBECTL) logs -n $(K8S_NAMESPACE) -f deploy/$(C)
endif

# ===== OpenBao Commands =====
init-openbao: ## Initialize OpenBao (one-time setup: generate unseal key + root token)
	@echo "Initializing OpenBao..."
	@chmod +x ./init/openbao/openbao-init.sh 2>/dev/null || true
	@./init/openbao/openbao-init.sh

unseal: ## Unseal OpenBao (needed after every container restart)
	@echo "Trying to unseal OpenBao (if not already unsealed)..."
	@chmod +x ./init/openbao/openbao-unseal.sh 2>/dev/null || true
	@./init/openbao/openbao-unseal.sh || true

gen-cred: ## Generate credentials.yaml from template (./init/genCredential.sh)
	@echo "Generating credentials.yaml from template..."
	@chmod +x ./init/genCredential.sh 2>/dev/null || true
	@./init/genCredential.sh

enc-cred: ## Encrypt credentials.yaml to credentials.yaml.enc (./init/encCredential.sh)
	@echo "Encrypting credentials.yaml..."
	@chmod +x ./init/encCredential.sh 2>/dev/null || true
	@./init/encCredential.sh

dec-cred: ## Decrypt credentials.yaml.enc to credentials.yaml (./init/decCredential.sh)
	@echo "Decrypting credentials.yaml.enc..."
	@chmod +x ./init/decCredential.sh 2>/dev/null || true
	@./init/decCredential.sh

bcrypt: ## Generate bcrypt hash for given password (`make bcrypt PASSWORD=mypassword`)
	@if [ -z "$(PASSWORD)" ]; then \
		echo "Please provide a password: make bcrypt PASSWORD=mypassword"; \
		exit 1; \
	fi
	@mkdir -p cmd/bcrypt
	@if [ ! -f "cmd/bcrypt/bcrypt" ]; then \
		echo "bcrypt binary not found, building it..."; \
		go build -o cmd/bcrypt/bcrypt cmd/bcrypt/main.go; \
		chmod +x cmd/bcrypt/bcrypt; \
	fi
	@echo "$(PASSWORD)" | ./cmd/bcrypt/bcrypt

certs: ## Generate self-signed certs (`make certs` / `make certs DOMAIN=mydomain.com IP=x.x.x.x CERT_DIR=~/.cloud-barista/certs`)
	@echo "Generating self-signed certificates..."
	@echo "DOMAIN=$(DOMAIN), IP=$(IP), CERT_DIR=$(CERT_DIR)"
	chmod +x scripts/certs/generate-certs.sh; \
	scripts/certs/generate-certs.sh DOMAIN=$(DOMAIN) IP=$(IP) CERT_DIR=$(CERT_DIR) 

help: ## Display this help screen
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "CB-Tumblebug Makefile Commands"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "🐳 Container Build & Run:"
	@echo "  \033[36mup (compose-up)\033[0m        Start services with --build (docker compose up --build) and auto init/unseal OpenBao"
	@echo "  \033[36mdown (compose-down)\033[0m    Stop services (docker compose down)"
	@echo "  \033[36mps (status)\033[0m            Show status of services (docker compose ps)"
	@echo "  \033[36mlogs\033[0m                   Follow service logs (docker compose logs -f)"
	@echo ""
	@echo "⚙️  Initialization:"
	@grep -E '^(init|init-profile|gen-cred|enc-cred|dec-cred):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🔐 OpenBao (Secrets Management):"
	@echo "  \033[36minit-openbao\033[0m           Initialize OpenBao (one-time setup)"
	@echo "  \033[36munseal\033[0m                 Unseal OpenBao (after container restart)"
	@echo ""
	@echo "☸️  Kubernetes (Helm):"
	@echo -e "  \033[36mk-up\033[0m                   Install stack via Helm (auto-creates kind cluster if needed)"
	@echo -e "  \033[36mk-init\033[0m                 Initialize the K8s deployment (port-forward + init flow)"
	@echo -e "  \033[36mk-down\033[0m                 Uninstall Helm release (data kept)"
	@echo -e "  \033[36mk-clean\033[0m                Full K8s reset (PVCs + OpenBao Secret)"
	@echo -e "  \033[36mk-status (k-ps)\033[0m        Show K8s status (release/pods/services/port-forwards)"
	@echo -e "  \033[36mk-info\033[0m                 Show access endpoints & LLM-client setup for enabled features"
	@echo -e "  \033[36mk-logs\033[0m                 Per-component log commands (k-logs C=<name> to follow)"
	@echo -e "  \033[36mk-port-forward\033[0m         Start port-forwards (API 1323, MapUI 1324; idempotent)"
	@echo -e "  \033[36mk-port-forward-stop\033[0m    Stop port-forwards for the deployment namespace"
	@echo -e "  \033[36mk-token\033[0m                Create admin token file for K8s UIs (e.g., Headlamp)"
	@echo -e "  \033[36mk-mcp-on / k-mcp-off\033[0m   Enable/disable the MCP server (persistent toggle)"
	@echo -e "  \033[36mk-agentgateway-on/-off\033[0m Enable/disable agentgateway in front of MCP"
	@echo -e "  \033[36mk-mcp-auth-on/-off\033[0m     JWT auth on the MCP route (local key, no IdP)"
	@echo -e "  \033[36mk-mcp-token\033[0m            Mint a dev JWT for the MCP endpoint"
	@echo -e "  \033[36mk-gateway-on/-off\033[0m      Enable/disable the Gateway API single entrypoint"
	@echo -e "  \033[36mk-gateway-forward\033[0m      Port-forward the gateway entrypoint to :8080"
	@echo -e "  \033[36mk-build-tb/-mapui/-mcp/-sp\033[0m Build LOCAL source into the cluster (compose --build equiv.)"
	@echo -e "  \033[36mk-build-off [C=]\033[0m       Revert to published images"
	@echo "  (aliases: make up/init/down TARGET=k8s)"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  \033[36mclean-db\033[0m               Clean database metadata (./init/cleanDB.sh)"
	@echo "  \033[36mclean-all\033[0m              Stop containers + clean databases + OpenBao (requires re-init)"
	@echo ""
	@echo "💾 Database Backup & Restore:"
	@grep -E '^(backup-assets|restore-assets):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🔧 Utilities:"
	@grep -E '^(swag|bcrypt|certs):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "📦 Binary Build & Run & Cleanup:"
	@grep -E '^(default|run|clean):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""	
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "💡 Quick Start Workflow:"
	@echo "   make up ▶ make gen-cred ▶ (edit credentials) ▶ make enc-cred ▶ make init"
	@echo ""
	@echo "   💡 During 'make init', you'll be asked if you want to use the pre-built"
	@echo "      database backup (1 min) or fetch fresh data from CSPs (20 min)."
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ===== PHONY targets (not actual files) =====
.PHONY: default run clean clean-all swag swagger init init-profile compose compose-down logs status ps clean-db backup-assets restore-assets up down gen-cred enc-cred dec-cred bcrypt certs help k-up k-init k-down k-clean k-status k-ps k-logs k-port-forward k-port-forward-stop k-token k-mcp-on k-mcp-off k-mcp-client-info k-info k-agentgateway-on k-agentgateway-off k-mcp-auth-on k-mcp-auth-off k-mcp-token k-gateway-on k-gateway-off k-gateway-forward k-build k-build-tb k-build-mapui k-build-mcp k-build-sp k-build-off k-diagnose