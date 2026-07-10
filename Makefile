.PHONY: help lint test build vet fmt clean

help:
	@echo "Fenwick Makefile targets:"
	@echo "  make lint       - Run golangci-lint"
	@echo "  make test       - Run all tests"
	@echo "  make test-race  - Run tests with race detector"
	@echo "  make test-cov   - Run tests with coverage report"
	@echo "  make build      - Build the project"
	@echo "  make vet        - Run go vet"
	@echo "  make fmt        - Format code with goimports"
	@echo "  make clean      - Clean build artifacts and coverage files"

lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

test:
	@echo "Running tests..."
	go test -v ./...

test-race:
	@echo "Running tests with race detector..."
	go test -v -race ./...

test-cov:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report generated: coverage.out"

build:
	@echo "Building..."
	go build -v ./...

vet:
	@echo "Running go vet..."
	go vet ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

clean:
	@echo "Cleaning..."
	go clean -testcache
	rm -f coverage.out coverage.html
	@echo "Clean complete"
