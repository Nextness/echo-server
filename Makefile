.DEFAULT_GOAL := help

.PHONY: help fmt fmt-check vet test test-e2e check-requirements build-echo-server run-infra clean

BUILD_DIR := build
GO_FILES := $(shell git ls-files '*.go')
GO_MODULE_FILES := go.mod infra/go.mod infra/go.sum
ECHO_SERVER_GO_FILES := $(filter cmd/echo/% internal/echo/%,$(GO_FILES))
INFRA_GO_FILES := $(filter infra/%,$(GO_FILES))
PULUMI_FILES := infra/Pulumi.yaml infra/Pulumi.local.yaml
E2E_FILES := scripts/local-e2e.sh scripts/check-requirements.sh Dockerfile .dockerignore $(GO_FILES) $(GO_MODULE_FILES) $(PULUMI_FILES)

help: ## Show this help message.
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} \
		/^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' \
		$(MAKEFILE_LIST)

fmt: $(GO_FILES) ## Format all tracked Go source files.
	gofmt -w $(GO_FILES)

fmt-check: scripts/check-fmt.sh $(GO_FILES) ## Fail if any tracked Go source file needs formatting.
	bash scripts/check-fmt.sh

vet: $(GO_FILES) $(GO_MODULE_FILES) ## Run go vet in both Go modules.
	go vet ./...
	cd infra && go vet ./...

test: fmt-check vet $(GO_FILES) $(GO_MODULE_FILES) ## Run formatting, vet, race, and coverage checks.
	go test -race -cover -v ./...
	cd infra && go test -race -cover -v ./...

test-e2e: $(E2E_FILES) ## Run the local end-to-end test; pass E2E_ARGS=--delete to clean up.
	bash scripts/local-e2e.sh $(E2E_ARGS)

check-requirements: scripts/check-requirements.sh ## Validate required tools, versions, and Docker access.
	bash scripts/check-requirements.sh

build-echo-server: $(ECHO_SERVER_GO_FILES) go.mod ## Build the application at build/echo-server.
	mkdir -p $(BUILD_DIR)
	go build -trimpath -o $(BUILD_DIR)/echo-server ./cmd/echo

run-infra: check-requirements $(INFRA_GO_FILES) infra/go.mod infra/go.sum $(PULUMI_FILES) ## Apply the local Pulumi stack.
	pulumi -C infra up --yes --stack local

clean: ## Remove generated files from the build directory.
	rm -rf -- build
