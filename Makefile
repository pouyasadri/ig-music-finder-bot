# Instagram / SoundCloud Audio Telegram Bot
BINARY     := bot
CMD        := ./cmd/bot
BIN_DIR    := bin
OUT        := $(BIN_DIR)/$(BINARY)

GO         ?= go
DOCKER     ?= docker
COMPOSE    ?= docker compose

CGO_ENABLED ?= 1
LDFLAGS    ?= -w -s

.PHONY: help build run test tidy fmt vet clean \
	docker-build docker-up docker-down docker-logs docker-restart \
	cookies deps

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  %-18s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

##@ Local development

deps: ## Download Go module dependencies
	$(GO) mod download

tidy: ## Tidy and verify Go modules
	$(GO) mod tidy
	$(GO) mod verify

fmt: ## Format Go source
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

build: ## Build the bot binary into bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="$(LDFLAGS)" -o $(OUT) $(CMD)
	@echo "Built $(OUT)"

run: build ## Build and run the bot (requires .env / env vars)
	@set -a; [ -f .env ] && . ./.env; set +a; \
		DATABASE_PATH=$${DATABASE_PATH:-./data/bot.db} \
		COOKIES_PATH=$${COOKIES_PATH:-./cookies.txt} \
		SHAZAM_SCRIPT_PATH=$${SHAZAM_SCRIPT_PATH:-./scripts/shazam_recognize.py} \
		$(OUT)

test: ## Run all unit tests
	$(GO) test ./...

test-race: ## Run tests with the race detector
	CGO_ENABLED=1 $(GO) test -race ./...

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
	rm -f $(BINARY)

##@ Docker

docker-build: ## Build the Docker image
	$(COMPOSE) build

docker-up: ## Build and start the bot in the background
	$(COMPOSE) up -d --build

docker-down: ## Stop and remove containers
	$(COMPOSE) down

docker-restart: ## Restart the running container
	$(COMPOSE) restart

docker-logs: ## Follow container logs
	$(COMPOSE) logs -f


##@ Setup

cookies: ## Copy cookies.txt.example → cookies.txt if missing
	@if [ ! -f cookies.txt ]; then \
		cp cookies.txt.example cookies.txt; \
		echo "Created cookies.txt from cookies.txt.example — fill in Instagram cookies if needed."; \
	else \
		echo "cookies.txt already exists."; \
	fi
