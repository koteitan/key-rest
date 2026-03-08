.PHONY: build test clean install

BINARY=key-rest
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/key-rest/

test:
	go test ./... -count=1

clean:
	rm -f $(BUILD_DIR)/$(BINARY)

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
	cp $(BUILD_DIR)/$(BINARY) ~/go/bin/$(BINARY)
