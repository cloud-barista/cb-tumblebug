default: ## Build the application ('make' without arguments)
	cd src/ && $(MAKE)

run: ## Run the built application
	cd src/ && $(MAKE) run

clean: ## Clean build artifacts
	cd src/ && $(MAKE) clean

clean-all: clean ## Clean build artifacts, containers, and databases

swag swagger: ## Generate Swagger documentation
	cd src/ && $(MAKE) swag

# ===== Initialization =====
init: ## Run initialization script (./init/init.sh)
	@echo "Running initialization script..."
	@chmod +x ./init/init.sh 2>/dev/null || true
	@./init/init.sh

# ===== Docker Compose Commands =====
compose: ## Start Docker Compose services with --build (docker compose up --build)
	DOCKER_BUILDKIT=1 docker compose up --build

compose-down: ## Stop Docker Compose services (docker compose down)
	@echo "Stopping Docker Compose services..."
	docker compose down

# ===== Database Cleanup Commands =====
clean-db: ## Clean all database metadata (./init/cleanDB.sh)
	@echo "Running cleanDB script..."
	@chmod +x ./init/cleanDB.sh 2>/dev/null || true
	@./init/cleanDB.sh

# ===== Utility Aliases =====
up: ## Quick start (alias for compose)
	$(MAKE) compose

compose-up: ## Build and start Docker Compose services (alias for compose)
	$(MAKE) compose

down: ## Quick stop (alias for compose-down)
	$(MAKE) compose-down 

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
	@echo "  \033[36mup (compose-up)\033[0m        Start services with --build (docker compose up --build)"
	@echo "  \033[36mdown (compose-down)\033[0m    Stop services (docker compose down)"
	@echo ""
	@echo "⚙️  Initialization:"
	@grep -E '^(init|gen-cred|enc-cred|dec-cred):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  \033[36mclean-db\033[0m               Clean database metadata (./init/cleanDB.sh)"
	@echo "  \033[36mclean-all\033[0m              Clean build + containers + databases"
	@echo ""
	@echo "🔧 Utilities:"
	@grep -E '^(swag|bcrypt|certs):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "📦 Binary Build & Run & Cleanup:"
	@grep -E '^(default|run|clean):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""	
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "💡 Quick Start Workflow:"
	@echo "   make up ▶  make gen-cred ▶  (edit credentials) ▶  make enc-cred ▶  make init"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ===== PHONY targets (not actual files) =====
.PHONY: default run clean clean-all swag swagger init compose compose-up compose-down clean-db up down gen-cred enc-cred dec-cred bcrypt certs help