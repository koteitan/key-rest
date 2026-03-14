[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# System Tests

End-to-end tests that verify all 26 services work correctly through key-rest. Each test starts the [test-server](../test-server/README.md), registers credentials, and sends authenticated requests through the key-rest proxy.

## Test Variants

| Variant | Description |
|---|---|
| [go/](go/README.md) | Go test using `go test` with inline Unix socket client |
| [curl/](curl/README.md) | Shell script using [key-rest-curl](../clients/curl/key-rest-curl) |

## Prerequisites

- Go 1.21+
- bash (for curl variant)
- python3 (for curl variant, port discovery)
