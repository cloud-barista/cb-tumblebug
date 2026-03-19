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

init: ## Run initialization sequence (credential registration for OpenBao and Tumblebug)
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "CB-Tumblebug Initialization"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@IS_TMP_KEY=0; \
	cleanup_tmp_key() { \
		if [ "$$IS_TMP_KEY" = "1" ] && [ -f ~/.cloud-barista/.tmp_enc_key ]; then \
			rm -f ~/.cloud-barista/.tmp_enc_key; \
		fi; \
	}; \
	trap cleanup_tmp_key EXIT INT TERM HUP; \
	if [ ! -f ~/.cloud-barista/.tmp_enc_key ]; then \
		echo "Notice: A temporary key file will be created for initialization."; \
		echo "        It will be removed automatically after initialization."; \
		printf "Enter the password for credentials.yaml.enc: "; \
		read -s PASS; \
		echo ""; \
		printf "%s" "$$PASS" > ~/.cloud-barista/.tmp_enc_key; \
		chmod 600 ~/.cloud-barista/.tmp_enc_key; \
		IS_TMP_KEY=1; \
	fi; \
	( \
		echo "1. Registering credentials to OpenBao..."; \
		chmod +x ./conf/openbao/openbao-register-creds.sh 2>/dev/null || true; \
		./conf/openbao/openbao-register-creds.sh -y && \
		echo "" && \
		echo "2. Registering credentials to Tumblebug..." && \
		chmod +x ./init/init.sh 2>/dev/null || true; \
		./init/init.sh; \
	); \
	EXIT_CODE=$$?; \
	if [ "$$EXIT_CODE" -ne 0 ]; then \
		echo "Initialization failed."; \
	fi; \
	exit $$EXIT_CODE
	@echo "Initialization complete!"

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
	@echo "Prepared!"
# Note: OpenBao data dir ownership is fixed by entrypoint chown in docker-compose.yaml.

compose: prepare-volumes ## Start Docker Compose services (auto init/unseal OpenBao)
	@echo "Starting OpenBao..."
	@DOCKER_BUILDKIT=1 docker compose up -d openbao
	@if [ ! -f .env ] || ! grep -q '^VAULT_TOKEN=.\+' .env 2>/dev/null; then \
		echo "VAULT_TOKEN not found — running first-time OpenBao initialization..."; \
		bash conf/openbao/openbao-init.sh; \
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
	@rm -f conf/openbao/secrets/openbao-init.json
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
up: ## Start all services (alias for compose)
	@$(MAKE) compose

down: ## Quick stop (alias for compose-down)
	@$(MAKE) compose-down

# ===== OpenBao Commands =====
init-openbao: ## Initialize OpenBao (one-time setup: generate unseal key + root token)
	@echo "Initializing OpenBao..."
	@chmod +x ./conf/openbao/openbao-init.sh 2>/dev/null || true
	@./conf/openbao/openbao-init.sh

unseal: ## Unseal OpenBao (needed after every container restart)
	@echo "Trying to unseal OpenBao (if not already unsealed)..."
	@chmod +x ./conf/openbao/openbao-unseal.sh 2>/dev/null || true
	@./conf/openbao/openbao-unseal.sh || true

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
	@grep -E '^(init|gen-cred|enc-cred|dec-cred):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🔐 OpenBao (Secrets Management):"
	@echo "  \033[36minit-openbao\033[0m           Initialize OpenBao (one-time setup)"
	@echo "  \033[36munseal\033[0m                 Unseal OpenBao (after container restart)"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  \033[36mclean-db\033[0m               Clean database metadata (./init/cleanDB.sh)"
	@echo "  \033[36mclean-all\033[0m              Clean build + containers + databases + OpenBao (requires re-init)"
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
.PHONY: default run clean clean-all swag swagger init compose compose-down logs status ps clean-db backup-assets restore-assets up down gen-cred enc-cred dec-cred bcrypt certs help