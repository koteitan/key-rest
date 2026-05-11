.PHONY: build test test-unit test-go test-python test-node test-system coverage coverage-html coverage-clean clean install

BINARY=key-rest
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/key-rest/

test: test-unit test-system

test-unit: test-go test-python test-node

test-go:
	go test $(shell go list ./... | grep -v -e system-test -e /scripts) -count=1 | grep -v '\[no test files\]'

# Generate Go test coverage profile (excludes system-test which runs the real daemon).
# Subprocess tests under cmd/key-rest are -cover-instrumented; their per-test
# covdata directories are NOT yet merged into coverage.out because each lives
# under t.TempDir() and is removed at end of the test. Inline tests of the
# pure helpers cover the deterministic paths.
#
# Lines marked with `// cover:ignore` (defensive branches, os.Exit
# fall-throughs, syscall failures) are stripped from the profile by
# scripts/coverignore.go so they do not count against the totals.
coverage:
	go test -coverprofile=coverage.out.raw -covermode=atomic $(shell go list ./... | grep -v -e system-test -e /scripts)
	@go run scripts/coverignore.go coverage.out.raw > coverage.out
	@rm coverage.out.raw
	@go tool cover -func=coverage.out | tail -1

# Render the coverage profile as a browsable HTML report.
# Post-process the generated HTML to inject a line-number gutter.
coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@awk 'BEGIN{ while ((getline l < "scripts/coverage-lineno.html") > 0) snip = snip l "\n"; close("scripts/coverage-lineno.html") } /<\/head>/ { sub("</head>", snip "</head>") } { print }' coverage.html > coverage.html.tmp && mv coverage.html.tmp coverage.html
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
