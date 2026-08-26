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
	@TB_K8S_NAMESPACE=$${TB_K8S_NAMESPACE:-$(K8S_NAMESPACE)} ./scripts/backup-assets.sh

restore-assets: ## Restore PostgreSQL database from assets backup (or FILE=<path>)
	@chmod +x ./scripts/restore-assets.sh 2>/dev/null || true
	@if [ -z "$(FILE)" ]; then \
		TB_K8S_NAMESPACE=$${TB_K8S_NAMESPACE:-$(K8S_NAMESPACE)} ./scripts/restore-assets.sh; \
	else \
		TB_K8S_NAMESPACE=$${TB_K8S_NAMESPACE:-$(K8S_NAMESPACE)} ./scripts/restore-assets.sh $(FILE); \
	fi

set-versions: ## Interactively set core component release versions (cb-tumblebug/cb-spider/cb-mapui/mc-terrarium) across compose + k8s
	@chmod +x ./scripts/misc/set-release-versions.sh 2>/dev/null || true
	@./scripts/misc/set-release-versions.sh

k-tunnel: ## Interactively open an SSH tunnel to a remote cb-tumblebug cluster (gateway/direct, TLS-aware); prints browser links
	@chmod +x ./scripts/misc/ssh-tunnel.sh 2>/dev/null || true
	@./scripts/misc/ssh-tunnel.sh

k-tunnel-stop: ## Stop the SSH tunnel opened by k-tunnel
	@chmod +x ./scripts/misc/ssh-tunnel.sh 2>/dev/null || true
	@./scripts/misc/ssh-tunnel.sh stop

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
KIND_VERSION ?= v0.32.0
# kind cluster topology. When k-up creates a NEW kind cluster, it asks interactively for the
# node layout — UNLESS you pass KIND_WORKERS on the command line (or run non-interactively, e.g.
# CI), in which case these values are used as-is with no prompt:
#   make k-up KIND_WORKERS=2 KIND_TAINT_CP=true   # 1 control-plane + 2 workers, kubeadm-like
# KIND_WORKERS>0 => multi-node (observe pod placement / affinity). KIND_TAINT_CP taints the
# control-plane NoSchedule so workloads land on workers only (ignored when KIND_WORKERS=0).
# Only applies at cluster creation; delete the old cluster first to change node count.
KIND_WORKERS ?= 0
KIND_TAINT_CP ?= false
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
# Local assets/ overlaid via ConfigMap instead of the image copy (see k-assets)
K8S_ASSETS_VALUES := deployments/helm/cb-tumblebug/values-dev-assets.yaml
DEV_ASSETS_FLAG = $(if $(wildcard $(K8S_ASSETS_VALUES)),-f $(K8S_ASSETS_VALUES))
# Interactive add-on wizard (fresh install only): pre-apply choices set the state files above;
# post-apply choices (observability, port-forward, token) are stashed here between k-up steps.
K8S_ADDONS_ANSWERS := /tmp/.cb-tumblebug-addons.$(K8S_NAMESPACE)

# Observability (optional metrics stack: Prometheus + Grafana + exporters)
K8S_OBS_NS := monitoring
K8S_OBS_RELEASE := mon
K8S_OBS_VALUES := deployments/observability/kube-prometheus-stack.values.yaml
# Logs stack (Loki + Promtail): turns cb-tumblebug's own logs into interaction signals
K8S_LOKI_RELEASE := loki
K8S_LOKI_VALUES := deployments/observability/loki-stack.values.yaml
K8S_ASSETS_CM ?= cb-tumblebug-assets
# Everything under assets/ that the server reads at runtime. Excludes assets.dump.gz
# (34MB DB dump, host-side only) which would blow past the 1MiB ConfigMap limit.
K8S_ASSETS_FILES = $(sort $(wildcard assets/*.yaml) $(wildcard assets/*.csv))
# Free-form local overrides (gitignored; e.g. mapui.placeholderDefaults with tokens)
K8S_LOCAL_VALUES := deployments/helm/cb-tumblebug/values-local.yaml
LOCAL_VALUES_FLAG = $(if $(wildcard $(K8S_LOCAL_VALUES)),-f $(K8S_LOCAL_VALUES))
# MapUI popup params shared with docker-compose via .env (regenerated on every k-up)
K8S_ENV_PARAMS := deployments/helm/cb-tumblebug/values-env-params.yaml
MAPUI_PARAM_KEYS := HF_TOKEN VLLM_HF_TOKEN TAVILY_API_KEY DISCORD_TOKEN DISCORD_HOME_CHANNEL DISCORD_HOME_CHANNEL_NAME HERMES_API_KEY NTFY_TOPIC
# k-* output styling (disable with NO_COLOR=1); print with: printf '%b\n' '...'
ifdef NO_COLOR
KB :=
KG :=
KY :=
KR :=
KC :=
KD :=
KX :=
else
KB := \033[1m
KG := \033[32m
KY := \033[33m
KR := \033[31m
KC := \033[36m
KD := \033[2m
KX := \033[0m
endif

k-prereqs: ## Check &, on confirmation, install missing k-up prerequisites (kubectl/helm/kind + inotify sysctls); Docker is guidance-only
	@KIND_VERSION='$(KIND_VERSION)' bash scripts/misc/k-prereqs.sh

k-up: ## Install/upgrade the stack on Kubernetes (creates a kind cluster if no cluster is reachable)
	@# Fresh-VM convenience: if core tools are missing, offer the one-shot installer (interactive
	@# top-level only). Declining or CI falls through to the per-tool errors below (which also gate).
	@if [ -t 0 ] && [ "$(MAKELEVEL)" = "0" ]; then \
		missing=""; for t in helm kubectl kind; do command -v $$t >/dev/null 2>&1 || missing="$$missing $$t"; done; \
		if [ -n "$$missing" ]; then \
			printf '%b\n' "$(KY)\xe2\x9a\xa0 Missing tool(s):$(KX)$$missing"; \
			printf 'Run $(KC)make k-prereqs$(KX) to check & install them now? [Y/n]: '; \
			read -r a </dev/tty 2>/dev/null || a=""; \
			case "$$a" in [nN]*) : ;; *) $(MAKE) --no-print-directory k-prereqs || true ;; esac; \
		fi; \
	fi
	@command -v helm >/dev/null || { \
		printf '%b\n' '$(KR)\xe2\x9c\x96 helm is required$(KX) \xe2\x80\x94 install it (or run $(KC)make k-prereqs$(KX) to set up all tools at once):'; \
		printf '%b\n' '  $(KC)curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash$(KX)'; \
		printf '%b\n' '  $(KD)(docs: https://helm.sh/docs/intro/install/)$(KX)'; \
		exit 1; }
	@command -v kubectl >/dev/null || { \
		printf '%b\n' '$(KR)\xe2\x9c\x96 kubectl is required$(KX) \xe2\x80\x94 install it (or run $(KC)make k-prereqs$(KX) to set up all tools at once):'; \
		printf '%b\n' '  $(KC)curl -fsSLO "https://dl.k8s.io/release/$$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && sudo install -m 0755 kubectl /usr/local/bin/kubectl && rm kubectl$(KX)'; \
		printf '%b\n' '  $(KD)(docs: https://kubernetes.io/docs/tasks/tools/)$(KX)'; \
		exit 1; }
	@rm -f $(K8S_ADDONS_ANSWERS)
	@# Guard: the docker-compose stack binds the same host ports (1323/1024/1324/...) that
	@# this deployment is reached on. Catch the common "forgot to make down" case.
	@if [ -z "$(ALLOW_COMPOSE)" ] && command -v docker >/dev/null 2>&1; then \
		running=$$(docker compose ps -q 2>/dev/null | grep -c .); \
		if [ "$$running" -gt 0 ]; then \
			printf '%b\n' "$(KR)\xe2\x9c\x96 The docker-compose stack is running$(KX) ($$running container(s)) \xe2\x80\x94 it shares host ports with this deployment."; \
			printf '%b\n' '  Stop it first: $(KC)make down$(KX)   $(KD)(or force with $(KX)$(KC)make k-up ALLOW_COMPOSE=1$(KX)$(KD))$(KX)'; \
			exit 1; \
		fi; \
	fi
	@if ! $(KUBECTL) cluster-info >/dev/null 2>&1; then \
		if command -v kind >/dev/null; then \
			$(MAKE) --no-print-directory k-preflight-host; \
			echo "No reachable cluster. Creating kind cluster '$(KIND_CLUSTER_NAME)'..."; \
			workers="$(KIND_WORKERS)"; taint="$(KIND_TAINT_CP)"; \
			if [ "$(origin KIND_WORKERS)" = "file" ] && [ -t 0 ]; then \
				printf 'Worker nodes? [0=single-node (fast dev), 2=multi-node kubeadm-like] (default 0): '; \
				read -r ans </dev/tty 2>/dev/null || ans=""; \
				case "$$ans" in ''|*[!0-9]*) workers=0 ;; *) workers=$$ans ;; esac; \
				if [ "$$workers" -gt 0 ] 2>/dev/null; then \
					printf 'Taint control-plane NoSchedule (workloads on workers only, like kubeadm)? [Y/n]: '; \
					read -r t </dev/tty 2>/dev/null || t=""; \
					case "$$t" in n|N|no|NO) taint=false ;; *) taint=true ;; esac; \
				fi; \
			fi; \
			if [ "$$workers" -gt 0 ] 2>/dev/null; then \
				nodes=$$((workers + 1)); rec=$$((256 * nodes)); if [ "$$rec" -lt 512 ]; then rec=512; fi; \
				inst=$$(cat /proc/sys/fs/inotify/max_user_instances 2>/dev/null || echo 0); \
				if [ "$$inst" -gt 0 ] && [ "$$inst" -lt "$$rec" ]; then \
					printf '%b\n' "$(KY)\xe2\x9a\xa0 Multi-node ($$nodes nodes): fs.inotify.max_user_instances=$$inst is low$(KX) \xe2\x80\x94 nodes may fail to start (each node adds kubelet+containerd inotify pressure)."; \
					printf '%b\n' "  $(KC)sudo sysctl -w fs.inotify.max_user_instances=$$rec$(KX)   $(KD)(persist: /etc/sysctl.d/99-kind.conf, then sudo sysctl --system)$(KX)"; \
				fi; \
			fi; \
			kind_cfg=""; \
			if [ "$$workers" -gt 0 ] 2>/dev/null; then \
				kind_cfg=$$(mktemp); \
				{ echo "kind: Cluster"; echo "apiVersion: kind.x-k8s.io/v1alpha4"; echo "nodes:"; \
				  echo "  - role: control-plane"; \
				  w=0; while [ $$w -lt $$workers ]; do echo "  - role: worker"; w=$$((w+1)); done; } > $$kind_cfg; \
				echo "  (multi-node: 1 control-plane + $$workers worker(s))"; \
			fi; \
			if ! kind create cluster --name $(KIND_CLUSTER_NAME) $${kind_cfg:+--config $$kind_cfg}; then \
				rm -f $$kind_cfg; \
				$(MAKE) --no-print-directory k-diagnose-host; \
				exit 1; \
			fi; \
			rm -f $$kind_cfg; \
			if [ "$$taint" = "true" ] && [ "$$workers" -gt 0 ] 2>/dev/null; then \
				kubectl taint nodes $(KIND_CLUSTER_NAME)-control-plane node-role.kubernetes.io/control-plane=:NoSchedule --overwrite >/dev/null 2>&1 || true; \
				echo "  (control-plane tainted NoSchedule - workloads land on the $$workers worker(s) only)"; \
			fi; \
		else \
			printf '%b\n' '$(KR)\xe2\x9c\x96 No reachable Kubernetes cluster and $(KX)$(KB)kind$(KX)$(KR) is not installed.$(KX)'; \
			printf '%b\n' '  Fastest: $(KC)make k-prereqs$(KX) $(KD)(installs kubectl/helm/kind + sysctls, with confirmation)$(KX), then re-run $(KC)make k-up$(KX).'; \
			printf '%b\n' '  Manual: $(KC)go install sigs.k8s.io/kind@$(KIND_VERSION)$(KX)   $(KD)(needs Go + Docker; binary lands in $$(go env GOPATH)/bin)$(KX)'; \
			printf '%b\n' '  $(KD)Ensure $$(go env GOPATH)/bin is on PATH. Alternatives: https://kind.sigs.k8s.io$(KX)'; \
			printf '%b\n' '  $(KD)Or point kubectl at an existing cluster: $(KX)$(KC)make k-up K8S_CONTEXT=<ctx>$(KX)'; \
			exit 1; \
		fi; \
	fi
	@# Show the install target; confirm when it is not a local kind cluster
	@ctx="$(K8S_CONTEXT)"; [ -n "$$ctx" ] || ctx=$$(kubectl config current-context 2>/dev/null); \
	server=$$($(KUBECTL) config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null); \
	printf '%b\n' "$(KB)$(KC)\xe2\x96\x8c Target$(KX) context $(KB)$$ctx$(KX) ($$server), namespace $(KB)$(K8S_NAMESPACE)$(KX)"; \
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
			printf '%b\n' '$(KR)\xe2\x9c\x96 No default StorageClass$(KX) \xe2\x80\x94 PVCs would stay Pending forever.'; \
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
		printf '%b\n' "$(KY)\xe2\x9a\xa0 All $$total node(s) carry the control-plane NoSchedule taint \xe2\x80\x94 pods will stay Pending.$(KX)"; \
		printf '%b\n' '  Single-node fix: $(KC)kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule-$(KX)'; \
	fi
	@if $(HELM) status $(HELM_RELEASE) -n $(K8S_NAMESPACE) >/dev/null 2>&1; then \
		rev=$$($(HELM) list -n $(K8S_NAMESPACE) --filter '^$(HELM_RELEASE)$$' 2>/dev/null | tail -1 | awk '{print $$3}'); \
		printf '%b\n' "$(KD)Existing release (revision $$rev) \xe2\x80\x94 upgrading in place (pods restart only if templates changed)$(KX)"; \
	else \
		printf '%b\n' "$(KD)No existing release \xe2\x80\x94 fresh install into namespace $(K8S_NAMESPACE)$(KX)"; \
	fi
	@# Add-on wizard: only on a fresh, top-level, interactive install (a sub-make from
	@# k-*-on/off runs at MAKELEVEL>0, so those never re-trigger it). Sets state files before apply.
	@if [ "$(MAKELEVEL)" = "0" ] && [ -t 0 ] && [ -z "$(NO_ADDON_WIZARD)" ] && \
		! $(HELM) status $(HELM_RELEASE) -n $(K8S_NAMESPACE) >/dev/null 2>&1; then \
		$(MAKE) --no-print-directory k-addons-configure; \
	fi
	@# Apply as a sub-make: state files the wizard just wrote (or the k-*-on targets wrote) become
	@# visible to $(wildcard) in the *_VALUES_FLAG vars only in a fresh recipe expansion, not this one.
	@$(MAKE) --no-print-directory k-up-apply
	@# Add-on install/config after the app is Ready (before the summary, so k-status/k-info reflect
	@# it): observability is a separate release; then port-forward per the wizard choice.
	@if [ -f $(K8S_ADDONS_ANSWERS) ]; then \
		OBS=0; PF=none; AUTH=0; . $(K8S_ADDONS_ANSWERS); \
		printf '%b\n' '' '$(KB)$(KC)\xe2\x96\x8c Installing selected add-ons$(KX)'; \
		if [ "$$OBS" = 1 ]; then $(MAKE) --no-print-directory k-observability-on || true; fi; \
		case "$$PF" in \
			auto) $(MAKE) --no-print-directory k-port-forward || true ;; \
			*)    : ;; \
		esac; \
		rm -f $(K8S_ADDONS_ANSWERS); \
	fi
	@[ -n "$(PF_DEFER)" ] || $(MAKE) --no-print-directory k-port-forward-restore
	@# Final summary: run once, after add-ons are installed and forwards are up, so the port-forward
	@# and observability lines are accurate (they used to print before those steps ran).
	@echo ""
	@$(MAKE) --no-print-directory k-status
	@rev=$$($(HELM) list -n $(K8S_NAMESPACE) --filter '^$(HELM_RELEASE)$$' 2>/dev/null | tail -1 | awk '{print $$3}'); \
	if [ "$$rev" = "1" ]; then \
		printf '%b\n' '' '$(KB)Next:$(KX) $(KC)make k-init$(KX)   $(KD)(first deployment only \xe2\x80\x94 register credentials & load assets)$(KX)'; \
	fi
	@echo ""
	@$(MAKE) --no-print-directory k-info

# Internal: the actual helm apply + readiness wait + status (called by k-up as a sub-make so the
# *_VALUES_FLAG $(wildcard) checks re-run and see state files written earlier in the same k-up).
k-up-apply:
	@# Reuse compose's .env for MapUI popup params (single source; values-local.yaml wins)
	@rm -f $(K8S_ENV_PARAMS); \
	if [ -f .env ]; then \
		params=""; \
		for k in $(MAPUI_PARAM_KEYS); do \
			v=$$(sed -n "s/^$$k=//p" .env | tail -1); \
			[ "$$k" = "VLLM_HF_TOKEN" ] && [ -z "$$v" ] && v=$$(sed -n 's/^HF_TOKEN=//p' .env | tail -1); \
			[ -n "$$v" ] && params="$$params    $$k: \"$$v\"\n"; \
		done; \
		if [ -n "$$params" ]; then \
			printf 'mapui:\n  placeholderDefaults:\n%b' "$$params" > $(K8S_ENV_PARAMS); \
			printf '%b\n' '$(KD)MapUI popup params loaded from .env (override: values-local.yaml)$(KX)'; \
		fi; \
	fi
	@[ -n "$(PF_DEFER)" ] || $(MAKE) --no-print-directory k-port-forward-save
	@# --reset-values: state comes only from chart defaults + current flags
	@# (otherwise helm silently carries over user-supplied values from the previous revision)
	@printf '%b\n' '$(KD)Applying Helm release (timeout 10m)...$(KX)'
	@$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(K8S_NAMESPACE) --create-namespace --timeout 10m \
		--reset-values $(MCP_VALUES_FLAG) $(AGW_VALUES_FLAG) $(AGW_AUTH_VALUES_FLAG) $(GW_VALUES_FLAG) $(DEV_IMAGE_FLAGS) $(DEV_ASSETS_FLAG) \
		$$( [ -f $(K8S_ENV_PARAMS) ] && printf '%s' '-f $(K8S_ENV_PARAMS)' ) $(LOCAL_VALUES_FLAG) $(HELM_ARGS) >/dev/null || \
		{ $(MAKE) --no-print-directory k-diagnose; exit 1; }
	@printf '%b' "$(KD)Waiting for pods to become Ready (timeout 10m)... $(KX)"
	@# openbao-init restarts cb-tumblebug/mc-terrarium mid-rollout to pick up the token,
	@# so pods captured by 'kubectl wait -l' get deleted under it (NotFound). Re-resolve
	@# and retry until the whole set settles Ready, rather than trusting one snapshot.
	@deadline=$$(( $$(date +%s) + 600 )); \
	while :; do \
		if $(KUBECTL) wait pods -l app.kubernetes.io/part-of=cb-tumblebug \
			--for=condition=Ready -n $(K8S_NAMESPACE) --timeout=20s >/dev/null 2>&1; then \
			break; \
		fi; \
		if [ $$(date +%s) -ge $$deadline ]; then \
			printf '%b\n' '$(KR)\xe2\x9c\x96$(KX)'; $(MAKE) --no-print-directory k-diagnose; exit 1; \
		fi; \
		sleep 5; \
	done
	@printf '%b\n' '$(KG)\xe2\x9c\x94 all Ready$(KX)'

# Internal: dump why the deployment is unhealthy (used by k-up failure paths)
k-diagnose:
	@echo ""
	@printf '%b\n' '$(KB)$(KR)\xe2\x96\x8c Deployment diagnostics$(KX) $(KD)(namespace $(K8S_NAMESPACE))$(KX)'
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

# Internal: warn about host limits that commonly break a local kind cluster (WSL2, low inotify, cgroup v1)
k-preflight-host:
	@inst=$$(cat /proc/sys/fs/inotify/max_user_instances 2>/dev/null || echo 0); \
	if [ "$$inst" -gt 0 ] && [ "$$inst" -lt 512 ]; then \
		printf '%b\n' "$(KY)\xe2\x9a\xa0 fs.inotify.max_user_instances=$$inst (low)$(KX) \xe2\x80\x94 kind control-plane may fail to start."; \
		printf '%b\n' '  $(KC)sudo sysctl -w fs.inotify.max_user_instances=512$(KX)   $(KD)(persist: add to /etc/sysctl.d/99-kind.conf, then sudo sysctl --system)$(KX)'; \
	fi
	@watch=$$(cat /proc/sys/fs/inotify/max_user_watches 2>/dev/null || echo 0); \
	if [ "$$watch" -gt 0 ] && [ "$$watch" -lt 524288 ]; then \
		printf '%b\n' "$(KY)\xe2\x9a\xa0 fs.inotify.max_user_watches=$$watch (low)$(KX) \xe2\x80\x94 raise to 524288:"; \
		printf '%b\n' '  $(KC)sudo sysctl -w fs.inotify.max_user_watches=524288$(KX)   $(KD)(persist: add to /etc/sysctl.d/99-kind.conf)$(KX)'; \
	fi
	@if [ ! -f /sys/fs/cgroup/cgroup.controllers ]; then \
		printf '%b\n' "$(KY)\xe2\x9a\xa0 Host is on cgroup v1$(KX) \xe2\x80\x94 deprecated by kind/Kubernetes; recent node images may fail to boot."; \
		if grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null; then \
			printf '%b\n' '  WSL2 fix: add to $(KB)%UserProfile%\\.wslconfig$(KX) (Windows side), then run $(KC)wsl --shutdown$(KX) and reopen:'; \
			printf '%b\n' '    $(KD)[wsl2]$(KX)'; \
			printf '%b\n' '    $(KD)kernelCommandLine = cgroup_no_v1=all systemd.unified_cgroup_hierarchy=1$(KX)'; \
		else \
			printf '%b\n' '  $(KD)Enable cgroup v2 (unified hierarchy) on the host \xe2\x80\x94 see distro docs.$(KX)'; \
		fi; \
	fi

# Internal: explain a failed local kind cluster creation (used by k-up)
k-diagnose-host:
	@echo ""
	@printf '%b\n' '$(KB)$(KR)\xe2\x96\x8c kind cluster creation failed$(KX)'
	@if ! (command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1); then \
		printf '%b\n' '  $(KY)Docker is not running/reachable$(KX) \xe2\x80\x94 start Docker and retry.'; \
	fi
	@printf '%b\n' 'Common causes on a fresh host (especially WSL2):'
	@$(MAKE) --no-print-directory k-preflight-host
	@printf '%b\n' '' 'After applying a fix, remove the half-created cluster and retry:'
	@printf '%b\n' '  $(KC)kind delete cluster --name $(KIND_CLUSTER_NAME) && make k-up$(KX)'
	@printf '%b\n' '  $(KD)Full logs: kind export logs \xe2\x80\x94 known issues: https://kind.sigs.k8s.io/docs/user/known-issues/$(KX)'

# Internal: interactive add-on selection, invoked by k-up on a fresh, top-level, interactive install.
# Writes the same persistent state files as the k-*-on targets (so the single k-up helm apply picks
# them up) and stashes observability/port-forward/token choices in K8S_ADDONS_ANSWERS for k-up's
# post-apply step. Dependencies mirror the standalone targets (auth needs agentgateway needs MCP;
# TLS needs gateway). Non-interactive/CI installs skip this entirely (see the gate in k-up).
k-addons-configure:
	@printf '%b\n' '' '$(KB)$(KC)\xe2\x96\x8c Optional add-ons$(KX) $(KD)(fresh install \xe2\x80\x94 Enter = Yes; type n to decline; change anytime later with make k-*-on/off)$(KX)'
	@obs=0; pf=none; auth=0; \
	printf 'Gateway single entrypoint (/ MapUI, /tumblebug API, /mcp)? [Y/n]: '; read -r a </dev/tty 2>/dev/null || a=""; \
	case "$$a" in [nN]*) : ;; *) \
		$(MAKE) --no-print-directory k-gateway-install || true; \
		printf 'gateway:\n  enabled: true\n' > $(K8S_GW_VALUES); \
		printf '  \xe2\x86\xb3 HTTPS :8443 (self-signed cert)? [Y/n]: '; read -r t </dev/tty 2>/dev/null || t=""; \
		case "$$t" in [nN]*) printf '%b\n' '     $(KG)gateway$(KX) enabled (HTTP)';; \
			*) printf 'gateway:\n  enabled: true\n  tls:\n    enabled: true\n' > $(K8S_GW_VALUES); printf '%b\n' '     $(KG)gateway + HTTPS$(KX) enabled';; esac ;; \
		esac; \
	printf 'MCP server (LLM tool endpoint)? [Y/n]: '; read -r a </dev/tty 2>/dev/null || a=""; \
	case "$$a" in [nN]*) : ;; *) \
		printf 'mcp:\n  enabled: true\n' > $(K8S_MCP_VALUES); printf '%b\n' '  $(KG)MCP server$(KX) enabled'; \
		printf '  \xe2\x86\xb3 agentgateway in front of MCP? [Y/n]: '; read -r g </dev/tty 2>/dev/null || g=""; \
		case "$$g" in [nN]*) : ;; *) \
			printf 'agentgateway:\n  enabled: true\n' > $(K8S_AGW_VALUES); printf '%b\n' '     $(KG)agentgateway$(KX) enabled'; \
			printf '     \xe2\x86\xb3 JWT auth on the MCP route? [Y/n]: '; read -r j </dev/tty 2>/dev/null || j=""; \
			case "$$j" in [nN]*) : ;; *) $(MAKE) --no-print-directory k-mcp-authkey && auth=1 && printf '%b\n' '        $(KG)JWT auth$(KX) enabled';; esac ;; \
			esac ;; \
		esac; \
	printf 'Observability (Prometheus + Grafana)? [Y/n]: '; read -r a </dev/tty 2>/dev/null || a=""; \
	case "$$a" in [nN]*) : ;; *) obs=1; printf '%b\n' '  $(KG)observability$(KX) will be installed after the app is Ready';; esac; \
	printf 'Start port-forwarding after deploy (auto: gateway if enabled, else per-service; + Grafana if obs)? [Y/n]: '; read -r p </dev/tty 2>/dev/null || p=""; \
	case "$$p" in [nN]*) pf=none;; *) pf=auto;; esac; \
	printf 'OBS=%s\nPF=%s\nAUTH=%s\n' "$$obs" "$$pf" "$$auth" > $(K8S_ADDONS_ANSWERS)

k-init: ## Run initialization against the Kubernetes deployment (port-forward + headless-capable init)
	@printf '%b\n' '$(KD)Port-forwarding cb-tumblebug ($(K8S_INIT_PORT) -> 1323)...$(KX)'
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
	@printf '%b' "Waiting for pods to terminate (timeout 2m)... "
	@$(KUBECTL) wait --for=delete pods -l app.kubernetes.io/part-of=cb-tumblebug \
		-n $(K8S_NAMESPACE) --timeout=120s 2>/dev/null && printf '%b\n' '$(KG)\xe2\x9c\x94$(KX)' || \
		printf '%b\n' '$(KY)\xe2\x9a\xa0 some pods are still terminating \xe2\x80\x94 check later with: make k-status$(KX)'
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Stopped.$(KX) Data PVCs are kept $(KD)(restart: $(KX)$(KC)make k-up$(KX)$(KD); full reset: $(KX)$(KC)make k-clean$(KX)$(KD))$(KX)'
	@$(HELM) status $(K8S_OBS_RELEASE) -n $(K8S_OBS_NS) >/dev/null 2>&1 && \
		printf '%b\n' '$(KD)\xe2\x84\xb9 Observability is still running in $(K8S_OBS_NS) (kept on purpose). Remove with: $(KX)$(KC)make k-observability-off$(KX)' || true

k-clean: ## Full K8s reset: uninstall + delete PVCs and OpenBao key Secret
	@$(HELM) uninstall $(HELM_RELEASE) -n $(K8S_NAMESPACE) 2>/dev/null || true
	@$(KUBECTL) delete job openbao-init -n $(K8S_NAMESPACE) --ignore-not-found 2>/dev/null || true
	@$(KUBECTL) delete pvc --all -n $(K8S_NAMESPACE) 2>/dev/null || true
	@printf '%b' "Waiting for PVC deletion $(KD)(prevents volume reuse races on the next k-up)$(KX)... "
	@$(KUBECTL) wait --for=delete pvc --all -n $(K8S_NAMESPACE) --timeout=180s 2>/dev/null && printf '%b\n' '$(KG)\xe2\x9c\x94$(KX)' || printf '%b\n' '$(KY)\xe2\x9a\xa0 timeout$(KX)'
	@$(KUBECTL) delete secret openbao-keys -n $(K8S_NAMESPACE) 2>/dev/null || true
	@$(MAKE) --no-print-directory k-port-forward-stop
	@# Full reset also clears observability (a separate release that normally survives k-up/k-down).
	@# CLEAN_OBS=1 forces removal, CLEAN_OBS=0 keeps; otherwise ask (default Yes) when interactive,
	@# keep when not (so automation never silently drops a shared monitoring stack).
	@if $(HELM) status $(K8S_OBS_RELEASE) -n $(K8S_OBS_NS) >/dev/null 2>&1; then \
		remove=""; \
		if [ "$(CLEAN_OBS)" = "1" ]; then remove=y; \
		elif [ "$(CLEAN_OBS)" = "0" ]; then remove=n; \
		elif [ -t 0 ]; then \
			printf 'Observability is also running in $(K8S_OBS_NS) \xe2\x80\x94 remove it too (full reset)? [Y/n]: '; \
			read -r a </dev/tty 2>/dev/null || a=""; \
			case "$$a" in n|N|no|NO) remove=n;; *) remove=y;; esac; \
		else remove=n; fi; \
		if [ "$$remove" = y ]; then $(MAKE) --no-print-directory k-observability-off; \
		else printf '%b\n' '$(KD)\xe2\x84\xb9 Observability ($(K8S_OBS_NS)) kept (separate release). Remove with: $(KX)$(KC)make k-observability-off$(KX)$(KD) (or CLEAN_OBS=1)$(KX)'; fi; \
	fi
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Cleaned.$(KX) Re-deploy with: $(KC)make k-up$(KX) then $(KC)make k-init$(KX)'
	@printf '%b\n' '$(KD)(a kind cluster is kept; remove with: kind delete cluster --name $(KIND_CLUSTER_NAME))$(KX)'

# Resilient port-forwards via scripts/misc/pf-watch.sh: each forward is detached (setsid →
# survives the launching shell / an ephemeral SSH session) and self-healing (retry loop →
# reconnects after a drop from a long request or a pod rollout). PGIDs are recorded per file
# so the stop targets tear down exactly the recorded forwards.
PF := KUBECTL='$(KUBECTL)' bash scripts/misc/pf-watch.sh
K8S_PF_SVC_PIDS := /tmp/.cb-pf-watch-svc.$(K8S_NAMESPACE)
K8S_PF_GW_PIDS  := /tmp/.cb-pf-watch-gw.$(K8S_NAMESPACE)

k-port-forward: ## Start resilient port-forwards — auto-picks the gateway entrypoint (:8080/+:8443) when the gateway is on, else per-service (:1323/:1324); Grafana :3000 if obs on. Force with PF_MODE=gateway|service. Idempotent.
	@$(MAKE) --no-print-directory k-port-forward-stop
	@mode="$(PF_MODE)"; \
	gwsvc=$$($(KUBECTL) get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=cb-tumblebug-gateway -o name 2>/dev/null | head -1); \
	if [ -z "$$mode" ]; then [ -n "$$gwsvc" ] && mode=gateway || mode=service; fi; \
	https=""; \
	if [ "$$mode" = gateway ]; then \
		[ -n "$$gwsvc" ] || { echo "Gateway service not found — run 'make k-gateway-on' first (or use PF_MODE=service)."; exit 1; }; \
		$(PF) start $(K8S_PF_GW_PIDS) -n envoy-gateway-system $$gwsvc 8080:80; \
		if $(KUBECTL) get -n envoy-gateway-system $$gwsvc -o jsonpath='{.spec.ports[*].port}' 2>/dev/null | grep -qw 443; then \
			$(PF) start $(K8S_PF_GW_PIDS) -n envoy-gateway-system $$gwsvc 8443:443; https=1; \
		fi; \
	else \
		$(PF) start $(K8S_PF_SVC_PIDS) -n $(K8S_NAMESPACE) svc/cb-tumblebug 1323:1323; \
		$(KUBECTL) get svc cb-mapui -n $(K8S_NAMESPACE) >/dev/null 2>&1 && $(PF) start $(K8S_PF_SVC_PIDS) -n $(K8S_NAMESPACE) svc/cb-mapui 1324:1324 || true; \
	fi; \
	$(KUBECTL) get svc $(K8S_OBS_RELEASE)-grafana -n $(K8S_OBS_NS) >/dev/null 2>&1 && $(PF) start $(K8S_PF_SVC_PIDS) -n $(K8S_OBS_NS) svc/$(K8S_OBS_RELEASE)-grafana 3000:80 || true; \
	sleep 2; \
	printf '%b\n' "$(KB)$(KC)\xe2\x96\x8c Port-forwards started$(KX) $(KD)(mode: $$mode)$(KX)"; \
	if [ "$$mode" = gateway ]; then \
		printf '  %-14s $(KC)%s$(KX) $(KD)%s$(KX)\n' 'Entrypoint' 'http://localhost:8080' '(/ MapUI, /tumblebug API+Swagger, /mcp MCP)'; \
		[ -n "$$https" ] && printf '  %-14s $(KC)%s$(KX) $(KD)%s$(KX)\n' 'Entrypoint TLS' 'https://localhost:8443' '(same routes; self-signed)' || true; \
	else \
		printf '  %-14s $(KC)%s$(KX)\n' 'API / Swagger' 'http://localhost:1323/tumblebug/api'; \
		printf '  %-14s $(KC)%s$(KX)\n' 'MapUI' 'http://localhost:1324'; \
	fi; \
	$(KUBECTL) get svc $(K8S_OBS_RELEASE)-grafana -n $(K8S_OBS_NS) >/dev/null 2>&1 && printf '  %-14s $(KC)%s$(KX) $(KD)%s$(KX)\n' 'Grafana' 'http://localhost:3000' '(admin / admin)' || true; \
	printf '%b\n' '$(KD)Stop with: $(KX)$(KC)make k-port-forward-stop$(KX)'; \
	$(KUBECTL) get svc cb-tumblebug-mcp-server -n $(K8S_NAMESPACE) >/dev/null 2>&1 && printf '%b\n' 'MCP enabled \xe2\x80\x94 connection guide: $(KC)make k-info$(KX)' || true

k-mcp-on: ## Enable the MCP server (persists across future k-up runs)
	@printf 'mcp:\n  enabled: true\n' > $(K8S_MCP_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 MCP server enabled$(KX) $(KD)(persisted in $(K8S_MCP_VALUES)) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

k-mcp-off: ## Disable the MCP server (also disables agentgateway, which depends on it)
	@rm -f $(K8S_MCP_VALUES) $(K8S_AGW_VALUES) $(K8S_AGW_AUTH_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 MCP server (and agentgateway, if enabled) disabled$(KX) $(KD)\xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

k-agentgateway-on: ## Enable agentgateway in front of the MCP server (enables MCP too; persistent)
	@printf 'mcp:\n  enabled: true\n' > $(K8S_MCP_VALUES)
	@printf 'agentgateway:\n  enabled: true\n' > $(K8S_AGW_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 agentgateway + MCP enabled$(KX) $(KD)(persisted) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

k-info: ## Show access endpoints & LLM-client setup based on what is currently enabled
	@port_up() { ss -ltnH 2>/dev/null | awk -v pp="$$1" '{n=split($$4,a,":"); if(a[n]==pp) f=1} END{exit !f}'; }; \
	gw=$$($(KUBECTL) get svc -n envoy-gateway-system \
		-l gateway.envoyproxy.io/owning-gateway-name=cb-tumblebug-gateway -o name 2>/dev/null | head -1); \
	mcp=$$($(KUBECTL) get svc cb-tumblebug-mcp-server -n $(K8S_NAMESPACE) -o name 2>/dev/null); \
	agw=$$($(KUBECTL) get svc agentgateway -n $(K8S_NAMESPACE) -o name 2>/dev/null); \
	printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c Access guide$(KX) $(KD)(based on what is enabled)$(KX)'; \
	if [ -n "$$gw" ]; then \
		routes="/ MapUI | /tumblebug API+Swagger"; \
		[ -n "$$mcp" ] && routes="$$routes | /mcp MCP"; \
		if port_up 8080; then \
			printf '%b\n' 'Single entrypoint $(KD)(recommended)$(KX) $(KG)\xe2\x9c\x94 forward running$(KX):'; \
		else \
			printf '%b\n' 'Single entrypoint $(KD)(recommended)$(KX) \xe2\x80\x94 start: $(KC)make k-port-forward$(KX)'; \
		fi; \
		printf '%b\n' "  -> $(KC)http://localhost:8080$(KX) ($$routes)"; \
		if $(KUBECTL) get -n envoy-gateway-system $$gw -o jsonpath='{.spec.ports[*].port}' 2>/dev/null | grep -qw 443; then \
			printf '%b\n' '  -> $(KC)https://localhost:8443$(KX) $(KD)(same routes; self-signed cert \xe2\x80\x94 browser OK, MCP clients may reject)$(KX)'; \
		fi; \
		printf '%b\n' '  $(KD)Note: localhost http is safe here \xe2\x80\x94 kubectl port-forward tunnels traffic inside TLS to the cluster.$(KX)'; \
	else \
		if port_up 1323; then \
			printf '%b\n' 'Per-service access $(KG)\xe2\x9c\x94 forward running$(KX) $(KD)(API/Swagger :1323, MapUI :1324)$(KX)'; \
		else \
			printf '%b\n' 'Per-service access \xe2\x80\x94 start: $(KC)make k-port-forward$(KX)  $(KD)(API/Swagger :1323, MapUI :1324)$(KX)'; \
		fi; \
	fi; \
	auth=0; $(KUBECTL) get configmap agentgateway-jwks -n $(K8S_NAMESPACE) >/dev/null 2>&1 && auth=1; \
	if [ -n "$$mcp" ]; then \
		if [ -n "$$gw" ]; then \
			$(MAKE) --no-print-directory k-mcp-client-info MCP_URL=http://localhost:8080/mcp MCP_AUTH=$$auth; \
			if port_up 8080; then printf '%b\n' '  $(KG)\xe2\x9c\x94 gateway forward running \xe2\x80\x94 reachable now$(KX)'; \
			else printf '%b\n' '  $(KD)(prerequisite: $(KX)$(KC)make k-port-forward$(KX)$(KD))$(KX)'; fi; \
		elif [ -n "$$agw" ]; then \
			$(MAKE) --no-print-directory k-mcp-client-info MCP_URL=http://localhost:3000/mcp MCP_AUTH=$$auth; \
			if port_up 3000; then printf '%b\n' '  $(KG)\xe2\x9c\x94 agentgateway forward running \xe2\x80\x94 reachable now$(KX)'; \
			else printf '%b\n' '  $(KD)(prerequisite: $(KX)$(KC)kubectl port-forward -n $(K8S_NAMESPACE) svc/agentgateway 3000:3000$(KX)$(KD))$(KX)'; fi; \
		else \
			$(MAKE) --no-print-directory k-mcp-client-info MCP_URL=http://localhost:8000/mcp MCP_AUTH=0; \
			if port_up 8000; then printf '%b\n' '  $(KG)\xe2\x9c\x94 mcp-server forward running \xe2\x80\x94 reachable now$(KX)'; \
			else printf '%b\n' '  $(KD)(prerequisite: $(KX)$(KC)kubectl port-forward -n $(K8S_NAMESPACE) svc/cb-tumblebug-mcp-server 8000:8000$(KX)$(KD))$(KX)'; fi; \
		fi; \
	fi

# Internal: print LLM-client connection snippets for a given MCP_URL (MCP_AUTH=1 adds JWT headers)
k-mcp-client-info:
	@echo ""
ifeq ($(MCP_AUTH),1)
	@printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c MCP client setup$(KX) \xe2\x80\x94 $(KC)$(MCP_URL)$(KX) $(KD)(streamable HTTP)$(KX) $(KG)[JWT auth ON]$(KX)'
	@printf '%b\n' '  $(KB)VS Code / Copilot$(KX) $(KD)(.vscode/mcp.json)$(KX):'
	@printf '%b\n' '$(KC)      { "servers": { "cb-tumblebug": { "type": "http", "url": "$(MCP_URL)",$(KX)'
	@printf '%b\n' '$(KC)          "headers": { "Authorization": "Bearer <TOKEN>" } } } }$(KX)'
	@printf '%b\n' '  $(KB)Claude Code$(KX) $(KD)(CLI \xe2\x80\x94 copy both lines)$(KX):'
	@printf '%b\n' '$(KC)      TOKEN=$$(make -s k-mcp-token | grep -o "eyJ[A-Za-z0-9_.-]*")$(KX)'
	@printf '%b\n' '$(KC)      claude mcp add --transport http cb-tumblebug $(MCP_URL) --header "Authorization: Bearer $$TOKEN"$(KX)'
	@printf '%b\n' '  $(KB)Cursor$(KX) $(KD)(~/.cursor/mcp.json)$(KX):'
	@printf '%b\n' '$(KC)      { "mcpServers": { "cb-tumblebug": { "url": "$(MCP_URL)",$(KX)'
	@printf '%b\n' '$(KC)          "headers": { "Authorization": "Bearer <TOKEN>" } } } }$(KX)'
	@printf '%b\n' '  $(KB)MCP Inspector$(KX): Authentication > Bearer Token = <TOKEN> $(KD)(token only, no "Bearer " prefix)$(KX)'
	@printf '%b\n' '  $(KD)Mint a token: $(KX)$(KC)make k-mcp-token$(KX)'
else
	@printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c MCP client setup$(KX) \xe2\x80\x94 $(KC)$(MCP_URL)$(KX) $(KD)(streamable HTTP)$(KX)'
	@printf '%b\n' '  $(KB)VS Code / Copilot$(KX) $(KD)(.vscode/mcp.json)$(KX):'
	@printf '%b\n' '$(KC)      { "servers": { "cb-tumblebug": { "type": "http", "url": "$(MCP_URL)" } } }$(KX)'
	@printf '%b\n' '  $(KB)Claude Code$(KX) $(KD)(CLI)$(KX):'
	@printf '%b\n' '$(KC)      claude mcp add --transport http cb-tumblebug $(MCP_URL)$(KX)'
	@printf '%b\n' '  $(KB)Cursor$(KX) $(KD)(~/.cursor/mcp.json)$(KX):'
	@printf '%b\n' '$(KC)      { "mcpServers": { "cb-tumblebug": { "url": "$(MCP_URL)" } } }$(KX)'
endif
	@printf '%b\n' '  $(KB)Claude Desktop$(KX): run the stdio<->HTTP bridge ON YOUR DESKTOP $(KD)(a Claude Desktop$(KX)'
	@printf '%b\n' '      $(KD)client limitation, same for compose) \xe2\x80\x94 see src/interface/mcp/README.md$(KX)'
	@printf '%b\n' '      $(KD)(mcp-simple-proxy.py + claude_desktop_config.json example)$(KX)'
	@printf '%b\n' '  $(KB)Test/debug$(KX) with MCP Inspector:'
	@printf '%b\n' '$(KC)      npx @modelcontextprotocol/inspector$(KX)'
	@printf '%b\n' '        $(KD)-> open the printed URL; Transport: "Streamable HTTP", URL: $(MCP_URL)$(KX)'
	@printf '%b\n' '      quick CLI check: $(KC)npx @modelcontextprotocol/inspector --cli $(MCP_URL) --method tools/list$(KX)'
ifeq ($(MCP_AUTH),1)
	@printf '%b\n' '  $(KG)Security: JWT auth is ENFORCED at the gateway$(KX) \xe2\x80\x94 requests without a valid token get 401.'
else
	@printf '%b\n' '  $(KY)Security note: the MCP endpoint itself is UNAUTHENTICATED$(KX) \xe2\x80\x94 tools act with the TB API'
	@printf '%b\n' '      credentials embedded in the server. Locally it is reachable only via kubectl port-forward;'
	@printf '%b\n' '      enable gateway-level auth before any external exposure: $(KC)make k-mcp-auth-on$(KX)'
endif

# Internal: ensure the local RSA signing key + write the agentgateway JWKS values (used by k-mcp-auth-on + the k-up add-on wizard)
k-mcp-authkey:
	@mkdir -p $(MCP_AUTH_DIR) && chmod 700 $(MCP_AUTH_DIR)
	@if [ ! -f $(MCP_AUTH_DIR)/key.pem ]; then \
		echo "Generating RSA signing key ($(MCP_AUTH_DIR)/key.pem)..."; \
		openssl genrsa -out $(MCP_AUTH_DIR)/key.pem 2048 2>/dev/null && chmod 600 $(MCP_AUTH_DIR)/key.pem; \
	fi
	@openssl rsa -in $(MCP_AUTH_DIR)/key.pem -pubout -out $(MCP_AUTH_DIR)/pub.pem 2>/dev/null
	@n=$$(openssl rsa -pubin -in $(MCP_AUTH_DIR)/pub.pem -noout -modulus | cut -d= -f2 | xxd -r -p | base64 -w0 | tr '+/' '-_' | tr -d '='); \
	jwks="{\"keys\":[{\"kty\":\"RSA\",\"kid\":\"cb-tb\",\"use\":\"sig\",\"alg\":\"RS256\",\"n\":\"$$n\",\"e\":\"AQAB\"}]}"; \
	printf 'agentgateway:\n  auth:\n    enabled: true\n    jwks: '"'"'%s'"'"'\n' "$$jwks" > $(K8S_AGW_AUTH_VALUES)

k-mcp-auth-on: ## Enable JWT auth on the MCP route (local key; mint tokens with k-mcp-token; persistent)
	@[ -f $(K8S_AGW_VALUES) ] || { printf '%b\n' '$(KR)\xe2\x9c\x96 agentgateway is not enabled$(KX) \xe2\x80\x94 run $(KC)make k-agentgateway-on$(KX) first.'; exit 1; }
	@$(MAKE) --no-print-directory k-mcp-authkey
	@printf '%b\n' '$(KG)\xe2\x9c\x94 MCP JWT auth enabled$(KX) $(KD)(persisted) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up
	@printf '%b\n' '' 'Mint a token with: $(KC)make k-mcp-token$(KX)'

k-mcp-auth-off: ## Disable JWT auth on the MCP route (key file is kept)
	@rm -f $(K8S_AGW_AUTH_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 MCP JWT auth disabled$(KX) $(KD)\xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

k-mcp-token: ## Mint a dev JWT for the MCP endpoint (MCP_TOKEN_TTL_HOURS=$(MCP_TOKEN_TTL_HOURS))
	@[ -f $(MCP_AUTH_DIR)/key.pem ] || { printf '%b\n' '$(KR)\xe2\x9c\x96 No signing key$(KX) \xe2\x80\x94 run $(KC)make k-mcp-auth-on$(KX) first.'; exit 1; }
	@b64url() { base64 -w0 | tr '+/' '-_' | tr -d '='; }; \
	now=$$(date +%s); exp=$$((now + $(MCP_TOKEN_TTL_HOURS)*3600)); \
	h=$$(printf '{"alg":"RS256","typ":"JWT","kid":"cb-tb"}' | b64url); \
	p=$$(printf '{"iss":"$(MCP_JWT_ISSUER)","aud":"$(MCP_JWT_AUDIENCE)","sub":"dev","iat":%s,"exp":%s}' "$$now" "$$exp" | b64url); \
	s=$$(printf '%s.%s' "$$h" "$$p" | openssl dgst -sha256 -sign $(MCP_AUTH_DIR)/key.pem -binary | b64url); \
	printf '%b\n' "$(KB)$(KC)\xe2\x96\x8c MCP token$(KX) $(KD)(valid until $$(date -d @$$exp '+%Y-%m-%d %H:%M' 2>/dev/null || date -r $$exp); TTL: MCP_TOKEN_TTL_HOURS=$(MCP_TOKEN_TTL_HOURS))$(KX)"; \
	echo ""; \
	echo "$$h.$$p.$$s"; \
	echo ""; \
	printf '%b\n' "$(KD)Use as: Authorization: Bearer <token>   (Inspector: paste token only, no Bearer prefix)$(KX)"

k-agentgateway-off: ## Disable agentgateway (MCP server stays enabled)
	@rm -f $(K8S_AGW_VALUES) $(K8S_AGW_AUTH_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 agentgateway disabled$(KX) $(KD)(MCP server kept) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

# Internal: install Envoy Gateway if no Gateway API implementation is present (used by k-gateway-on + the k-up add-on wizard)
k-gateway-install:
	@if ! $(KUBECTL) get crd gateways.gateway.networking.k8s.io >/dev/null 2>&1; then \
		ctx="$(K8S_CONTEXT)"; [ -n "$$ctx" ] || ctx=$$(kubectl config current-context 2>/dev/null); \
		case "$$ctx" in kind-*) ;; *) \
			printf '%b\n' "$(KY)\xe2\x9a\xa0 No Gateway API implementation found \xe2\x80\x94 installing Envoy Gateway $(ENVOY_GATEWAY_VERSION) into cluster '$$ctx'.$(KX)"; \
			printf '%b\n' "  $(KD)(only runs because none exists; an already-installed implementation is left untouched. Use your own instead of this: install it before k-gateway-on.)$(KX)" ;; \
		esac; \
		printf '%b\n' '$(KD)  Installing Envoy Gateway $(ENVOY_GATEWAY_VERSION)...$(KX)'; \
		$(HELM) install eg oci://docker.io/envoyproxy/gateway-helm --version $(ENVOY_GATEWAY_VERSION) \
			-n envoy-gateway-system --create-namespace --wait --timeout 6m >/dev/null; \
	fi

k-gateway-on: ## Enable the Gateway API entrypoint (/, /tumblebug, /mcp); installs Envoy Gateway if none is present
	@$(MAKE) --no-print-directory k-gateway-install
	@# Preserve an existing state file (it may carry extra flags like tls.enabled)
	@[ -f $(K8S_GW_VALUES) ] || printf 'gateway:\n  enabled: true\n' > $(K8S_GW_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Gateway entrypoint enabled$(KX) $(KD)(persisted) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up
	@printf '%b\n' '' 'Single entrypoint: $(KC)make k-port-forward$(KX)   $(KD)(http://localhost:8080 -> / mapui, /tumblebug API, /mcp MCP)$(KX)'

k-gateway-off: ## Disable the Gateway API entrypoint (the implementation/controller is left installed)
	@rm -f $(K8S_GW_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Gateway entrypoint disabled$(KX) $(KD)\xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

k-gateway-tls-on: ## Enable HTTPS (:8443 via k-gateway-forward) on the gateway entrypoint (self-signed cert; persistent)
	@[ -f $(K8S_GW_VALUES) ] || { printf '%b\n' '$(KR)\xe2\x9c\x96 gateway is not enabled$(KX) \xe2\x80\x94 run $(KC)make k-gateway-on$(KX) first.'; exit 1; }
	@printf 'gateway:\n  enabled: true\n  tls:\n    enabled: true\n' > $(K8S_GW_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Gateway HTTPS enabled$(KX) $(KD)(self-signed cert, generated once and reused) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up
	@# A freshly generated cert can be seconds "in the future" for the controller's
	@# one-shot validation (InvalidCertificateRef, no requeue) — nudge a re-reconcile
	@sleep 3; $(KUBECTL) annotate gateway cb-tumblebug-gateway -n $(K8S_NAMESPACE) \
		cloud-barista/tls-nudge="$$(date +%s)" --overwrite >/dev/null 2>&1 || true
	@printf '%b\n' '' 'HTTPS entrypoint: $(KC)make k-port-forward$(KX)  $(KD)-> https://localhost:8443$(KX)'

k-gateway-tls-off: ## Disable HTTPS on the gateway entrypoint (HTTP :8080 stays)
	@[ -f $(K8S_GW_VALUES) ] || { printf '%b\n' '$(KR)\xe2\x9c\x96 gateway is not enabled$(KX) \xe2\x80\x94 nothing to do.'; exit 1; }
	@printf 'gateway:\n  enabled: true\n' > $(K8S_GW_VALUES)
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Gateway HTTPS disabled$(KX) $(KD)(HTTP :8080 stays) \xe2\x80\x94 applying...$(KX)'
	@$(MAKE) --no-print-directory k-up

k-build-tb: ## Build local cb-tumblebug source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=cb-tumblebug
k-build-mapui: ## Build local cb-mapui source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=cb-mapui
k-build-mcp: ## Build local MCP server source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=mcp
k-build-sp: ## Build local cb-spider source into the cluster (shortcut)
	@$(MAKE) --no-print-directory k-build C=cb-spider

# --- MCP contract checks ------------------------------------------------------
# Runs inside the MCP image so fastmcp and the rest of the runtime deps are present;
# needs no cluster and no cloud credentials. Seconds, not minutes.
MCP_TEST_IMAGE ?= cloudbaristaorg/mcp:local-dev

mcp-check: ## Tier 1 contract checks for the MCP server (tools golden file, endpoints, schema budget, secrets, errors)
	@docker image inspect $(MCP_TEST_IMAGE) >/dev/null 2>&1 || \
		docker build -q -t $(MCP_TEST_IMAGE) src/interface/mcp >/dev/null
	@docker run --rm \
		-v "$(PWD)/src/interface/mcp:/repo/src/interface/mcp:ro" \
		-v "$(PWD)/src/interface/rest/docs/swagger.json:/repo/src/interface/rest/docs/swagger.json:ro" \
		-w /repo $(MCP_TEST_IMAGE) \
		python3 /repo/src/interface/mcp/tests/check_contract.py

mcp-check-update: ## Rewrite the MCP tools golden file after an intended tool change
	@docker image inspect $(MCP_TEST_IMAGE) >/dev/null 2>&1 || \
		docker build -q -t $(MCP_TEST_IMAGE) src/interface/mcp >/dev/null
	@docker run --rm --user "$$(id -u):$$(id -g)" \
		-v "$(PWD)/src/interface/mcp:/repo/src/interface/mcp" \
		-v "$(PWD)/src/interface/rest/docs/swagger.json:/repo/src/interface/rest/docs/swagger.json:ro" \
		-w /repo $(MCP_TEST_IMAGE) \
		python3 /repo/src/interface/mcp/tests/check_contract.py --update

mcp-bench: ## Measure MCP response sizes through the gateway (needs: make k-port-forward)
	@python3 src/interface/mcp/tests/bench.py

mcp-scenarios: ## Tier 2: drive real requests through MCP alone, stopping before any spend
	@python3 src/interface/mcp/tests/scenarios.py $(ARGS)

# --- MCP spend limits ---------------------------------------------------------
# Policy and approvals live in etcd, written directly. Deliberately not an MCP tool: the
# gateway terminates JWT and the MCP server cannot tell an administrator from an agent, so
# an "admin-only tool" would be unenforceable. The agent can request; only this can grant.
ETCD_POD ?= cb-tumblebug-etcd-0

mcp-budget: ## Show the MCP spend policy and pending approval requests
	@$(KUBECTL) exec -n $(K8S_NAMESPACE) $(ETCD_POD) -- etcdctl get --prefix /mcp/policy/budget
	@$(KUBECTL) exec -n $(K8S_NAMESPACE) $(ETCD_POD) -- etcdctl get --prefix /mcp/budget/requests/ || true

mcp-budget-set: ## Set spend limits (PER_CREATION=10 PER_DAY=100 CONCURRENT=50 ENABLED=true)
	@$(KUBECTL) exec -n $(K8S_NAMESPACE) $(ETCD_POD) -- etcdctl put /mcp/policy/budget \
		'{"enabled":$(or $(ENABLED),true),"per_creation_usd_per_hour":$(or $(PER_CREATION),10),"per_day_created_usd_per_hour":$(or $(PER_DAY),100),"concurrent_running_usd_per_hour":$(or $(CONCURRENT),50)}'
	@printf '%b\n' 'Spend limits updated. Check with: make mcp-budget'

mcp-budget-approve: ## Approve one over-budget request (ID=req-...)
	@test -n "$(ID)" || { echo "usage: make mcp-budget-approve ID=req-..."; exit 1; }
	@cur=$$($(KUBECTL) exec -n $(K8S_NAMESPACE) $(ETCD_POD) -- etcdctl get --print-value-only /mcp/budget/requests/$(ID)); \
	test -n "$$cur" || { echo "no such request: $(ID)"; exit 1; }; \
	upd=$$(printf '%s' "$$cur" | python3 -c 'import json,sys; d=json.load(sys.stdin); d["status"]="approved"; print(json.dumps(d))'); \
	$(KUBECTL) exec -n $(K8S_NAMESPACE) $(ETCD_POD) -- etcdctl put /mcp/budget/requests/$(ID) "$$upd" >/dev/null
	@printf '%b\n' 'Approved $(ID) — the agent may retry with budget_ack=$(ID) (single use)'

mcp-budget-off: ## Disable spend limits
	@$(KUBECTL) exec -n $(K8S_NAMESPACE) $(ETCD_POD) -- etcdctl put /mcp/policy/budget '{"enabled":false}'
	@printf '%b\n' 'Spend limits disabled.'

k-build: ## Build LOCAL source and run it in the cluster (C=tb|mapui|mcp) — compose `--build` equivalent
	@$(MAKE) --no-print-directory k-port-forward-save
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
	$(MAKE) --no-print-directory k-up PF_DEFER=1 && \
	echo "Restarting deploy/$$deploy2 to pick up the rebuilt image..." && \
	$(KUBECTL) rollout restart deploy/$$deploy2 -n $(K8S_NAMESPACE) && \
	$(KUBECTL) rollout status deploy/$$deploy2 -n $(K8S_NAMESPACE) --timeout=600s && \
	echo "Local build active for $$canon (persists across k-up). Revert: make k-build-off C=$$canon"
	@$(MAKE) --no-print-directory k-port-forward-restore

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

k-assets: ## Apply LOCAL assets/ (cloudinfo.yaml etc.) to the cluster — compose bind-mount equivalent
	@$(MAKE) --no-print-directory k-port-forward-save
	@files="$(K8S_ASSETS_FILES)"; \
	[ -n "$$files" ] || { echo "No assets/*.yaml or assets/*.csv found."; exit 1; }; \
	total=$$(cat $$files | wc -c); \
	if [ "$$total" -gt 1000000 ]; then \
		printf '%b\n' "$(KR)\xe2\x9c\x96 assets total $$total bytes, over the ~1MiB ConfigMap limit$(KX)"; \
		echo "  Trim assets/ or switch to a PVC/initContainer for the large files."; exit 1; \
	fi; \
	$(KUBECTL) get ns $(K8S_NAMESPACE) >/dev/null 2>&1 || $(KUBECTL) create ns $(K8S_NAMESPACE); \
	args=""; for f in $$files; do args="$$args --from-file=$$f"; done; \
	: "server-side apply: client-side would stash the whole object in the" \
	  "last-applied-configuration annotation, which caps out at 256KB"; \
	$(KUBECTL) create configmap $(K8S_ASSETS_CM) -n $(K8S_NAMESPACE) $$args \
		--dry-run=client -o yaml | \
		$(KUBECTL) apply --server-side --force-conflicts -f - >/dev/null && \
	{ printf 'assetsOverride:\n  configMapName: %s\n  files:\n' "$(K8S_ASSETS_CM)"; \
	  for f in $$files; do printf '    - %s\n' "$$(basename $$f)"; done; } > $(K8S_ASSETS_VALUES) && \
	printf '%b\n' "$(KD)ConfigMap $(K8S_ASSETS_CM) updated ($$(echo $$files | wc -w) files, $$total bytes)$(KX)" && \
	$(MAKE) --no-print-directory k-up PF_DEFER=1 && \
	echo "Restarting deploy/cb-tumblebug to re-read the assets..." && \
	$(KUBECTL) rollout restart deploy/cb-tumblebug -n $(K8S_NAMESPACE) && \
	$(KUBECTL) rollout status deploy/cb-tumblebug -n $(K8S_NAMESPACE) --timeout=600s && \
	echo "Local assets active (persists across k-up). Revert: make k-assets-off"
	@$(MAKE) --no-print-directory k-port-forward-restore

k-assets-off: ## Revert to the assets baked into the container image
	@rm -f $(K8S_ASSETS_VALUES)
	@echo "Reverting to image assets. Applying..."
	@$(MAKE) --no-print-directory k-port-forward-save
	@$(MAKE) --no-print-directory k-up PF_DEFER=1
	@$(KUBECTL) delete configmap $(K8S_ASSETS_CM) -n $(K8S_NAMESPACE) --ignore-not-found
	@$(KUBECTL) rollout restart deploy/cb-tumblebug -n $(K8S_NAMESPACE)
	@$(KUBECTL) rollout status deploy/cb-tumblebug -n $(K8S_NAMESPACE) --timeout=600s
	@$(MAKE) --no-print-directory k-port-forward-restore

k-gateway-forward: ## Alias for `make k-port-forward PF_MODE=gateway` (forward the gateway entrypoint :8080/+:8443)
	@$(MAKE) --no-print-directory k-port-forward PF_MODE=gateway

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
	@$(PF) stop $(K8S_PF_SVC_PIDS); $(PF) stop $(K8S_PF_GW_PIDS)
	@pids=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && / port-forward / && \
		(/ $(K8S_NAMESPACE) / || (/envoy-gateway-system/ && /cb-tumblebug-gateway/) || (/ $(K8S_OBS_NS) / && /grafana/)) {print $$1}' | xargs); \
	if [ -n "$$pids" ]; then \
		echo "Stopping port-forwards for this deployment (PID: $$pids)"; \
		kill $$pids 2>/dev/null || true; \
	fi

# Internal: a port-forward is bound to one pod, so any rollout silently breaks it.
# k-up/k-build save what is live beforehand and re-establish the same set afterwards.
# Callers that restart pods again after k-up pass PF_DEFER=1 and restore themselves.
K8S_PF_STATE := /tmp/.cb-tumblebug-port-forward.$(K8S_NAMESPACE)

k-port-forward-save:
	@rm -f $(K8S_PF_STATE); \
	pf=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && / port-forward /'); \
	printf '%s\n' "$$pf" | grep -q " $(K8S_NAMESPACE) " && echo svc >> $(K8S_PF_STATE) || true; \
	printf '%s\n' "$$pf" | grep -q "envoy-gateway-system" && echo gw >> $(K8S_PF_STATE) || true

k-port-forward-restore:
	@[ -f $(K8S_PF_STATE) ] || exit 0; \
	printf '%b\n' '' '$(KD)Refreshing the port-forwards that were open before this run...$(KX)'; \
	if grep -qx gw $(K8S_PF_STATE); then $(MAKE) --no-print-directory k-port-forward PF_MODE=gateway; \
	elif grep -qx svc $(K8S_PF_STATE); then $(MAKE) --no-print-directory k-port-forward PF_MODE=service; fi; \
	rm -f $(K8S_PF_STATE)

K8S_DNS_SERVERS ?= 8.8.8.8 1.1.1.1

k-dns-public: ## Make cluster DNS resolve external names via public DNS (K8S_DNS_SERVERS) instead of the host resolver
	@current=$$($(KUBECTL) -n kube-system get cm coredns -o jsonpath='{.data.Corefile}' 2>/dev/null); \
	[ -n "$$current" ] || { printf '%b\n' '$(KR)\xe2\x9c\x96 coredns ConfigMap not found$(KX)'; exit 1; }; \
	printf '%s' "$$current" | sed -E 's|^([[:space:]]*)forward \. [^{]*(\{?.*)$$|\1forward . $(K8S_DNS_SERVERS) \2|' > /tmp/Corefile.$$$$; \
	$(KUBECTL) -n kube-system create cm coredns --from-file=Corefile=/tmp/Corefile.$$$$ \
		--dry-run=client -o yaml | $(KUBECTL) apply -f - >/dev/null; \
	rm -f /tmp/Corefile.$$$$
	@$(KUBECTL) -n kube-system rollout restart deploy/coredns >/dev/null
	@$(KUBECTL) -n kube-system rollout status deploy/coredns --timeout=120s >/dev/null
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Cluster DNS now forwards external names to $(K8S_DNS_SERVERS)$(KX)' \
		'$(KD)Use this when the host resolver is unreliable (e.g. VPN): pods were failing with$(KX)' \
		'$(KD)"server misbehaving" or getting IPv6-only answers for CSP endpoints.$(KX)' \
		'$(KD)Revert: $(KX)$(KC)make k-dns-host$(KX)$(KD) \xe2\x80\x94 re-apply after recreating the cluster.$(KX)'

k-dns-host: ## Revert cluster DNS to the node resolver (kubeadm default)
	@current=$$($(KUBECTL) -n kube-system get cm coredns -o jsonpath='{.data.Corefile}' 2>/dev/null); \
	[ -n "$$current" ] || { printf '%b\n' '$(KR)\xe2\x9c\x96 coredns ConfigMap not found$(KX)'; exit 1; }; \
	printf '%s' "$$current" | sed -E 's|^([[:space:]]*)forward \. [^{]*(\{?.*)$$|\1forward . /etc/resolv.conf \2|' > /tmp/Corefile.$$$$; \
	$(KUBECTL) -n kube-system create cm coredns --from-file=Corefile=/tmp/Corefile.$$$$ \
		--dry-run=client -o yaml | $(KUBECTL) apply -f - >/dev/null; \
	rm -f /tmp/Corefile.$$$$
	@$(KUBECTL) -n kube-system rollout restart deploy/coredns >/dev/null
	@$(KUBECTL) -n kube-system rollout status deploy/coredns --timeout=120s >/dev/null
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Cluster DNS reverted to the node resolver$(KX)'

k-dns-check: ## Show which DNS the cluster forwards to, and resolve a CSP endpoint from a pod
	@printf '%b' '$(KB)$(KC)\xe2\x96\x8c Cluster DNS$(KX) '
	@$(KUBECTL) -n kube-system get cm coredns -o jsonpath='{.data.Corefile}' 2>/dev/null | \
		awk '/forward \./ {sub(/^[ \t]+/,""); print; exit}'
	@printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c Resolution from cb-spider$(KX)'
	@for h in $(K8S_DNS_PROBE_HOSTS); do \
		printf '  %-40s ' "$$h"; \
		$(KUBECTL) exec -n $(K8S_NAMESPACE) deploy/cb-spider -- getent ahostsv4 "$$h" 2>/dev/null | \
			awk 'NR==1{print $$1; found=1} END{if(!found) print "$(KR)no IPv4 answer$(KX)"}'; \
	done

K8S_DNS_PROBE_HOSTS ?= ec2.ap-northeast-2.amazonaws.com jp-tok.iaas.cloud.ibm.com login.microsoftonline.com

k-observability-on: ## Enable the metrics stack (Prometheus + Grafana + exporters) to watch cb-*/etcd/node load
	@printf '%b\n' '$(KD)  Updating helm repos (prometheus-community, metrics-server, grafana)...$(KX)'
	@$(HELM) repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
	@$(HELM) repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/ >/dev/null 2>&1 || true
	@$(HELM) repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
	@$(HELM) repo update prometheus-community metrics-server grafana >/dev/null 2>&1 || true
	@printf '%b\n' '$(KD)  Installing metrics-server (enables kubectl top)...$(KX)'
	@$(HELM) upgrade --install metrics-server metrics-server/metrics-server -n kube-system \
		--set 'args[0]=--kubelet-insecure-tls' --wait --timeout 3m >/dev/null
	@$(KUBECTL) get ns $(K8S_OBS_NS) >/dev/null 2>&1 || $(KUBECTL) create ns $(K8S_OBS_NS) >/dev/null
	@printf '%b\n' '$(KD)  Installing kube-prometheus-stack into $(K8S_OBS_NS) (a few minutes)...$(KX)'
	@$(HELM) upgrade --install $(K8S_OBS_RELEASE) prometheus-community/kube-prometheus-stack \
		-n $(K8S_OBS_NS) -f $(K8S_OBS_VALUES) --wait --timeout 8m >/dev/null
	@printf '%b\n' '$(KD)  Installing loki-stack (Loki + Promtail)...$(KX)'
	@$(HELM) upgrade --install $(K8S_LOKI_RELEASE) grafana/loki-stack \
		-n $(K8S_OBS_NS) -f $(K8S_LOKI_VALUES) --wait --timeout 5m >/dev/null 2> >(grep -vF 'this chart is deprecated' >&2)
	@printf '%b\n' '$(KD)  Provisioning cb-tumblebug dashboards...$(KX)'
	@$(KUBECTL) create configmap cb-dashboards -n $(K8S_OBS_NS) \
		--from-file=deployments/observability/dashboards/ \
		--dry-run=client -o yaml | $(KUBECTL) apply -f - >/dev/null
	@$(KUBECTL) label configmap cb-dashboards -n $(K8S_OBS_NS) grafana_dashboard=1 --overwrite >/dev/null
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Observability enabled$(KX) $(KD)\xe2\x80\x94 open Grafana: $(KC)make k-port-forward$(KX)$(KD) (admin / admin — set your own on first login; resets on pod restart)$(KX)'

k-observability-off: ## Remove the metrics stack (frees resources)
	@$(HELM) uninstall $(K8S_OBS_RELEASE) -n $(K8S_OBS_NS) 2>/dev/null || true
	@$(HELM) uninstall $(K8S_LOKI_RELEASE) -n $(K8S_OBS_NS) 2>/dev/null || true
	@$(HELM) uninstall metrics-server -n kube-system 2>/dev/null || true
	@$(KUBECTL) delete ns $(K8S_OBS_NS) --ignore-not-found >/dev/null 2>&1 || true
	@printf '%b\n' '$(KG)\xe2\x9c\x94 Observability removed$(KX)'

k-images: ## Show every workload's running image (apps + infra) and whether it is a local build or released
	@printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c Component images$(KX) $(KD)(local-dev = local build via k-build; revert: make k-build-off)$(KX)'
	@rows=$$($(KUBECTL) get deploy,statefulset -n $(K8S_NAMESPACE) -o jsonpath='{range .items[*]}{.metadata.name}={.spec.template.spec.containers[0].image}{"\n"}{end}' 2>/dev/null); \
	if [ -z "$$rows" ]; then printf '%b\n' '  $(KD)(no workloads \xe2\x80\x94 run $(KC)make k-up$(KD))$(KX)'; else \
	echo "$$rows" | while IFS='=' read -r name img; do \
		[ -n "$$img" ] || continue; \
		case "$$img" in \
			*:local-dev) label='$(KY)local build$(KX)' ;; \
			*)           label='$(KG)released$(KX)' ;; \
		esac; \
		printf '  %b%-26s%b %-42s %b\n' '$(KC)' "$$name" '$(KX)' "$$img" "$$label"; \
	done; fi
	@ovr=$$(ls deployments/helm/cb-tumblebug/values-dev-image-*.yaml 2>/dev/null | sed 's|.*/values-dev-image-||; s|\.yaml$$||' | paste -sd' ' -); \
	if [ -n "$$ovr" ]; then printf '%b\n' "  $(KY)\xe2\x9a\xa0 build-mode override active$(KX) $(KD)(kept across k-up: $$ovr)$(KX)"; fi

k-status: ## Show K8s deployment status (release/pods/services/port-forwards)
	@if $(HELM) status $(HELM_RELEASE) -n $(K8S_NAMESPACE) >/dev/null 2>&1; then \
		printf '%b' '$(KB)$(KC)\xe2\x96\x8c Helm release$(KX) '; \
		$(HELM) list -n $(K8S_NAMESPACE) --filter '^$(HELM_RELEASE)$$' 2>/dev/null | tail -1 | \
			awk '{print $$1 " (" $$8 ", revision " $$3 ", updated " $$4 " " $$5 ")"}'; \
	else \
		printf '%b\n' '$(KY)\xe2\x96\x8c Helm release: not installed$(KX) \xe2\x80\x94 run $(KC)make k-up$(KX)'; \
	fi
	@echo ""
	@$(MAKE) --no-print-directory k-images
	@echo ""
	@$(KUBECTL) get pods -n $(K8S_NAMESPACE) -o wide 2>/dev/null \
		| awk 'NR==1{sub(/[ \t]+NOMINATED NODE.*$$/,"");print;next}{sub(/[ \t]+[^ \t]+[ \t]+[^ \t]+[ \t]*$$/,"");print}' || true
	@echo ""
	@$(KUBECTL) get svc -n $(K8S_NAMESPACE) 2>/dev/null || true
	@nodes=$$($(KUBECTL) get nodes --no-headers 2>/dev/null | wc -l); \
	if [ "$$nodes" -gt 1 ]; then \
		echo ""; \
		printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c Pod placement by node$(KX)'; \
		$(KUBECTL) get pods -n $(K8S_NAMESPACE) -o custom-columns=NODE:.spec.nodeName,POD:.metadata.name --no-headers 2>/dev/null \
			| sort | awk '{print "  " $$1 "\t" $$2}'; \
	fi
	@echo ""
	@printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c Active port-forwards$(KX) $(KD)(may be stale after pod restarts \xe2\x80\x94 refresh with: make k-port-forward)$(KX)'
	@pf=$$(ps -eo pid=,args= | awk '$$2 ~ /(^|\/)kubectl$$/ && $$3 == "port-forward"'); \
	if [ -n "$$pf" ]; then \
		echo "$$pf" | sed 's/^/  /'; \
	else \
		printf '%b\n' '  (none) \xe2\x80\x94 start with: $(KC)make k-port-forward$(KX)'; \
	fi
	@echo ""
	@printf '%b\n' '$(KB)$(KC)\xe2\x96\x8c Observability$(KX) $(KD)(separate release in $(K8S_OBS_NS); survives k-up/k-down)$(KX)'
	@if $(HELM) status $(K8S_OBS_RELEASE) -n $(K8S_OBS_NS) >/dev/null 2>&1; then \
		printf '%b\n' '  $(KG)on$(KX) $(KD)\xe2\x80\x94 disable with: $(KX)$(KC)make k-observability-off$(KX)'; \
		$(KUBECTL) get pods -n $(K8S_OBS_NS) --no-headers 2>/dev/null | awk '{print "  " $$1 "  " $$3}'; \
	else \
		printf '%b\n' '  $(KY)off$(KX) $(KD)\xe2\x80\x94 enable with: $(KX)$(KC)make k-observability-on$(KX)'; \
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
	@echo -e "  \033[36mk-prereqs\033[0m              Check & (on confirm) install kubectl/helm/kind + sysctls (Docker: guidance)"
	@echo -e "  \033[36mk-up\033[0m                   Install stack via Helm (auto-creates kind cluster if needed)"
	@echo -e "  \033[36mk-init\033[0m                 Initialize the K8s deployment (port-forward + init flow)"
	@echo -e "  \033[36mk-down\033[0m                 Uninstall Helm release (data kept)"
	@echo -e "  \033[36mk-clean\033[0m                Full K8s reset (PVCs + OpenBao Secret)"
	@echo -e "  \033[36mk-status (k-ps)\033[0m        Show K8s status (release/pods/services/port-forwards)"
	@echo -e "  \033[36mk-info\033[0m                 Show access endpoints & LLM-client setup for enabled features"
	@echo -e "  \033[36mk-logs\033[0m                 Per-component log commands (k-logs C=<name> to follow)"
	@echo -e "  \033[36mk-port-forward\033[0m         Start port-forwards; auto gateway :8080 if on, else per-service :1323/:1324 (PF_MODE=gateway|service)"
	@echo -e "  \033[36mk-port-forward-stop\033[0m    Stop port-forwards for the deployment namespace"
	@echo -e "  \033[36mk-token\033[0m                Create admin token file for K8s UIs (e.g., Headlamp)"
	@echo -e "  \033[36mk-mcp-on / k-mcp-off\033[0m   Enable/disable the MCP server (persistent toggle)"
	@echo -e "  \033[36mk-agentgateway-on/-off\033[0m Enable/disable agentgateway in front of MCP"
	@echo -e "  \033[36mk-mcp-auth-on/-off\033[0m     JWT auth on the MCP route (local key, no IdP)"
	@echo -e "  \033[36mk-mcp-token\033[0m            Mint a dev JWT for the MCP endpoint"
	@echo -e "  \033[36mk-gateway-on/-off\033[0m      Enable/disable the Gateway API single entrypoint"
	@echo -e "  \033[36mk-gateway-forward\033[0m      Alias: k-port-forward PF_MODE=gateway (:8080/+:8443)"
	@echo -e "  \033[36mk-build-tb/-mapui/-mcp/-sp\033[0m Build LOCAL source into the cluster (compose --build equiv.)"
	@echo -e "  \033[36mk-build-off [C=]\033[0m       Revert to published images"
	@echo -e "  \033[36mk-assets / k-assets-off\033[0m   Apply LOCAL assets/ (cloudinfo.yaml etc.) without an image rebuild"
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
.PHONY: default run clean clean-all swag swagger init init-profile compose compose-down logs status ps clean-db backup-assets restore-assets up down set-versions k-tunnel k-tunnel-stop gen-cred enc-cred dec-cred bcrypt certs help k-prereqs k-up k-init k-down k-clean k-status k-ps k-images k-logs k-port-forward k-port-forward-stop k-port-forward-save k-port-forward-restore k-token k-mcp-on k-mcp-off k-mcp-client-info k-info k-agentgateway-on k-agentgateway-off k-mcp-auth-on k-mcp-auth-off k-mcp-token k-gateway-on k-gateway-off k-gateway-tls-on k-gateway-tls-off k-gateway-forward k-build k-build-tb k-build-mapui k-build-mcp k-build-sp k-build-off k-assets k-assets-off k-observability-on k-observability-off k-diagnose k-preflight-host k-diagnose-host k-up-apply k-addons-configure k-gateway-install k-mcp-authkey