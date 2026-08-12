BINARY_NAME := dockerfile-gen
OUTPUT_DIR := bin
SRC_LOCAL := github.com/hivepaas/dockerfile-generator

.PHONY: all build install run test test-coverage lint fmt mod clean help

all: build

## build: Build the CLI binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(OUTPUT_DIR)
	@go build -ldflags="-w -s" -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd/dockerfile-gen/...
	@echo "Build complete: $(OUTPUT_DIR)/$(BINARY_NAME)"

## install: Install binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME) to $(shell go env GOPATH)/bin..."
	@go install ./cmd/dockerfile-gen/...
	@echo "Install complete."

## run: Run CLI directly with go run
run:
	@go run ./cmd/dockerfile-gen/...

## test: Run unit tests
test:
	@echo "Running tests..."
	@go test -v -race ./...

## test-coverage: Run tests with HTML coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(OUTPUT_DIR)
	@go test -race -coverprofile=$(OUTPUT_DIR)/coverage.out ./...
	@go tool cover -html=$(OUTPUT_DIR)/coverage.out -o $(OUTPUT_DIR)/coverage.html
	@echo "Coverage report generated: $(OUTPUT_DIR)/coverage.html"

## mod: Tidy and verify Go modules
mod:
	@echo "Tidying go.mod..."
	@go mod tidy
	@go mod verify

## fmt: Format all Go source files
fmt:
	@echo "Formatting Go files..."
	@gofmt -w -s .
	@which goimports > /dev/null 2>&1 && find . -name '*.go' -not -wholename './vendor/*' | while read -r file; do goimports -local $(SRC_LOCAL) -w "$$file"; done || true

## lint: Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint is not installed. Run: brew install golangci-lint"

## clean: Remove build artifacts and temporary files
clean:
	@echo "Cleaning up..."
	@rm -rf $(OUTPUT_DIR)
	@echo "Clean complete."

## help: Display available make targets
help:
	@echo "Available commands in $(BINARY_NAME):"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
