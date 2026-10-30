.PHONY: build test lint clean coverage crap

BINARY_NAME=yat
# Keep in step with the version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION=v2.11.4
COVERAGE_FILE=coverage.out
CYCLO_FILE=cyclo.out
CRAP_TOP=20

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v -race -coverprofile=$(COVERAGE_FILE) ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

clean:
	rm -f $(BINARY_NAME) $(COVERAGE_FILE) $(CYCLO_FILE) coverage.html
	go clean

coverage: test
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"

# CRAP = complexity² × (1 − coverage)³ + complexity, top CRAP_TOP functions.
crap: test
	go tool gocyclo -over 0 -ignore '_test\.go$$' . > $(CYCLO_FILE)
	go tool cover -func=$(COVERAGE_FILE) | go run ./internal/tools/crap -cyclo $(CYCLO_FILE) -module $$(go list -m) -top $(CRAP_TOP)
