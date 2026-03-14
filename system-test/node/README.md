[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# Node.js System Test

End-to-end test that builds and starts the test-server and key-rest daemon, registers all credentials, and tests all 26 services through a Node.js Unix socket client.

## Run

```bash
node system-test/node/system_test.mjs
```

## How it works

1. Builds `test-server` and `key-rest` binaries into a temporary directory
2. Starts `test-server` with a self-signed certificate on a random port
3. Parses test credentials from test-server stdout
4. Registers all 28 keys via `key-rest add` with piped passphrase/value
5. Starts the key-rest daemon with `SSL_CERT_FILE` set to trust the test certificate
6. Sends authenticated requests for all 26 services via `node:net` Unix socket client and reports pass/fail
