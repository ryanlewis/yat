.PHONY: build test lint clean coverage crap

BINARY_NAME=yat
# Keep in step with the version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION=v2.11.4
COVERAGE_FILE=coverage.out
CYCLO_FILE=cyclo.out
# Coverage counters from the e2e tests' instrumented binary (see TestMain).
E2E_COVER_DIR=.cover-e2e
CRAP_TOP=20

build:
	go build -o $(BINARY_NAME) .

# The e2e tests run the real binary, so main.go is only covered through it.
# -count=1: TestMain reads YAT_E2E_COVERDIR before go test starts tracking
# environment reads, so a cached result would leave the e2e counters empty.
# The e2e profile is appended to the unit one without its mode line; go tool
# cover sums blocks that appear in both.
test:
	rm -rf $(E2E_COVER_DIR)
	YAT_E2E_COVERDIR=$(E2E_COVER_DIR)/raw go test -v -race -count=1 -coverprofile=$(COVERAGE_FILE) ./...
	go tool covdata textfmt -i=$(E2E_COVER_DIR)/raw -o $(E2E_COVER_DIR)/e2e.out
	tail -n +2 $(E2E_COVER_DIR)/e2e.out >> $(COVERAGE_FILE)

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

clean:
	rm -f $(BINARY_NAME) $(COVERAGE_FILE) $(CYCLO_FILE) coverage.html
	rm -rf $(E2E_COVER_DIR)
	go clean

coverage: test
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"

# CRAP = complexity² × (1 − coverage)³ + complexity, top CRAP_TOP functions.
crap: test
	go tool gocyclo -over 0 -ignore '_test\.go$$' . > $(CYCLO_FILE)
	go tool cover -func=$(COVERAGE_FILE) | go run ./internal/tools/crap -cyclo $(CYCLO_FILE) -module $$(go list -m) -top $(CRAP_TOP)
