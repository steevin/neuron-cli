GO := /opt/homebrew/bin/go
BINARY := neuron
BIN_DIR := bin
MODULE := github.com/danielsteevin/neuron-cli
MAIN := ./cmd/neuron

.PHONY: all build clean test lint run install help

all: build

build: ## Build the binary
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "-s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)" -o $(BIN_DIR)/$(BINARY) $(MAIN)

install: ## Install neuron to GOPATH/bin
	$(GO) install $(MAIN)

run: build ## Build and run
	./$(BIN_DIR)/$(BINARY)

test: ## Run all tests
	$(GO) test ./... -v -race

test-short: ## Run tests without race detector
	$(GO) test ./...

lint: ## Run linter
	golangci-lint run ./...

clean: ## Remove build artifacts
	@rm -rf $(BIN_DIR)
	go clean

tidy: ## Tidy go modules
	$(GO) mod tidy

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
