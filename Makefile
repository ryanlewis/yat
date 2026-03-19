.PHONY: build test lint clean coverage

BINARY_NAME=yat
COVERAGE_FILE=coverage.out

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v -race -coverprofile=$(COVERAGE_FILE) ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY_NAME) $(COVERAGE_FILE) coverage.html
	go clean

coverage: test
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"
