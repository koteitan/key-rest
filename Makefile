.PHONY: build test test-unit test-go test-python test-node test-system coverage coverage-html coverage-clean clean install

BINARY=key-rest
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/key-rest/

test: test-unit test-system

test-unit: test-go test-python test-node

test-go:
	go test $(shell go list ./... | grep -v system-test) -count=1 | grep -v '\[no test files\]'

# Generate Go test coverage profile (excludes system-test which runs the real daemon).
coverage:
	go test -coverprofile=coverage.out -covermode=atomic $(shell go list ./... | grep -v system-test)
	@go tool cover -func=coverage.out | tail -1

# Render the coverage profile as a browsable HTML report.
coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Wrote coverage.html"

coverage-clean:
	rm -f coverage.out coverage.html

test-python:
	cd clients/python && python3 -m unittest test_requests -v

test-node:
	cd clients/node && npm run build && npm test

test-system:
	cd system-test/go && go test -v -count=1
	system-test/curl/system-test.sh
	python3 system-test/python/system_test.py
	cd clients/node && npm run build
	node system-test/node/system_test.mjs

clean:
	rm -f $(BUILD_DIR)/$(BINARY)

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
	cp $(BUILD_DIR)/$(BINARY) ~/go/bin/$(BINARY)
