.PHONY: build test lint clean coverage

BINARY_NAME=yat
# Keep in step with the version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION=v2.15.0
COVERAGE_FILE=coverage.out

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v -race -coverprofile=$(COVERAGE_FILE) ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

clean:
	rm -f $(BINARY_NAME) $(COVERAGE_FILE) coverage.html
	go clean

coverage: test
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"
