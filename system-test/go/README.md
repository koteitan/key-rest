[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# Go System Test

End-to-end test written as a Go test. Builds and starts the test-server, sets up the keystore and daemon components in-process, and tests all 26 services through the Unix socket protocol.

## Run

```bash
cd system-test/go
go test -v -count=1
```

## How it works

1. Builds and starts `test-server` with a self-signed certificate
2. Parses test credentials from test-server stdout
3. Creates a keystore, registers all keys, and decrypts them in-memory
4. Starts a Unix socket server with the key-rest proxy
5. Sends authenticated requests for all 26 services and verifies 200 responses
