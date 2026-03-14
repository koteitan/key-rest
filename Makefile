.PHONY: build test test-unit test-go test-python test-node test-system clean install

BINARY=key-rest
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/key-rest/

test: test-unit test-system

test-unit: test-go test-python test-node

test-go:
	go test $(shell go list ./... | grep -v system-test) -count=1

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
