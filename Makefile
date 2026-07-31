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
		--reset-values $(MCP_VALUES_FLAG) $(HELM_ARGS) || \
		{ $(MAKE) --no-print-directory k-diagnose; exit 1; }
	@echo ""
	@echo "Waiting for pods to become Ready (timeout 10m)..."
	@$(KUBECTL) wait pods -l app.kubernetes.io/part-of=cb-tumblebug \
		--for=condition=Ready -n $(K8S_NAMESPACE) --timeout=600s || \
		{ $(MAKE) --no-print-directory k-diagnose; exit 1; }
	@$(MAKE) --no-print-directory k-status
	@echo ""
	@echo "Next: make k-init           (first deployment only — register credentials & load assets)"
	@echo "      make k-port-forward   (port-forward API 1323 & MapUI 1324 to localhost)"

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

k-mcp-on: ## Enable the MCP server (persists across future k-up runs)
	@printf 'mcp:\n  enabled: true\n' > $(K8S_MCP_VALUES)
	@echo "MCP server enabled (persisted in $(K8S_MCP_VALUES)). Applying..."
	@$(MAKE) --no-print-directory k-up
	@echo ""
	@echo "MCP endpoint: kubectl port-forward -n $(K8S_NAMESPACE) svc/cb-tumblebug-mcp-server 8000:8000"

k-mcp-off: ## Disable the MCP server
	@rm -f $(K8S_MCP_VALUES)
	@echo "MCP server disabled. Applying..."
	@$(MAKE) --no-print-directory k-up

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

k-port-forward-stop: ## Stop port-forwards targeting namespace $(K8S_NAMESPACE) (others are untouched)
	@pids=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && $$3 == "port-forward" && / $(K8S_NAMESPACE) /{print $$1}' | xargs); \
	if [ -n "$$pids" ]; then \
		echo "Stopping port-forwards for namespace $(K8S_NAMESPACE) (PID: $$pids)"; \
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
	@echo -e "  \033[36mk-logs\033[0m                 Per-component log commands (k-logs C=<name> to follow)"
	@echo -e "  \033[36mk-port-forward\033[0m         Start port-forwards (API 1323, MapUI 1324; idempotent)"
	@echo -e "  \033[36mk-port-forward-stop\033[0m    Stop port-forwards for the deployment namespace"
	@echo -e "  \033[36mk-token\033[0m                Create admin token file for K8s UIs (e.g., Headlamp)"
	@echo -e "  \033[36mk-mcp-on / k-mcp-off\033[0m   Enable/disable the MCP server (persistent toggle)"
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
.PHONY: default run clean clean-all swag swagger init init-profile compose compose-down logs status ps clean-db backup-assets restore-assets up down gen-cred enc-cred dec-cred bcrypt certs help k-up k-init k-down k-clean k-status k-ps k-logs k-port-forward k-port-forward-stop k-token k-mcp-on k-mcp-off k-diagnose