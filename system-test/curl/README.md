[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# curl System Test

End-to-end test as a shell script. Builds the binaries, starts the test-server and key-rest daemon as separate processes, registers all credentials via CLI, and tests all 26 services through [key-rest-curl](../../clients/curl/key-rest-curl).

## Run

```bash
system-test/curl/system-test.sh
```

## How it works

1. Builds `test-server` and `key-rest` binaries into a temporary directory
2. Starts `test-server` with a self-signed certificate on a random port
3. Parses test credentials from test-server stdout
4. Registers all 28 keys via `key-rest add` with piped passphrase/value
5. Starts the key-rest daemon with `SSL_CERT_FILE` set to trust the test certificate
6. Sends authenticated requests for all 26 services via `key-rest-curl` and reports pass/fail
