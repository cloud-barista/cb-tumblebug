default:
	cd src/ && $(MAKE)

cc:
	cd src/ && $(MAKE) cc

run:
	cd src/ && $(MAKE) run

runwithport:
	cd src/ && $(MAKE) runwithport $(PORT)

prod:
	cd src/ && $(MAKE) prod

clean:
	cd src/ && $(MAKE) clean

swag swagger:
	cd src/ && $(MAKE) swag

# make compose will build and run the docker-compose file (DOCKER_BUILDKIT is for quick build)
compose:
	DOCKER_BUILDKIT=1 docker compose up --build

bcrypt: ## Generate bcrypt hash for given password (usage: make bcrypt PASSWORD=mypassword)
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

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'