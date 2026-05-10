[English](testing.md) | [Japanese](testing-ja.md)

# Test Layout

key-rest tests fall into three categories: per-package unit tests, multi-language system tests, and PoC / regression tests for confirmed security findings.

## Per-package unit tests

Each Go package under `internal/` and `cmd/` has its own `*_test.go` files. They run with the standard `go test` workflow and have no external dependencies.

| Package | Location | Scope |
|---|---|---|
| `internal/crypto` | `crypto_test.go` | AES-256-GCM round-trip, PBKDF2 key derivation, mlock / zero-clear primitives |
| `internal/keystore` | `keystore_test.go`, `keystore_extra_test.go` | `Add` / `Remove` / `DecryptAll` / `Lookup` / `Disable` / `Enable` / `ListStatus`, on-disk format, file-permission checks, error paths |
| `internal/uri` | `uri_test.go`, `uri_bytes_test.go` | `FindAll`, `Replace`, `ReplaceBytes`, transform-function plumbing, `parseArgs` edge cases |
| `internal/proxy` | `proxy_test.go`, `coverage_test.go`, `headerinject_poc_test.go`, `raceattack_poc_test.go`, `raceattack2_poc_test.go`, `bodysmuggle_poc_test.go` | Request validation, URI resolution, response masking, secure-transport wire building, F-001 / F-002 / F-003 regressions |
| `internal/server` | `server_test.go`, `server_extra_test.go` | Unix-socket protocol dispatch (`http` / `reload` / `enable` / `disable` / `list` / `version`), connection limits, malformed-input handling |
| `internal/daemon` | `daemon_test.go` | `IsRunning`, `Start` / `Stop` lifecycle (with in-test SIGTERM trap), `reload` / `enable` handlers |
| `cmd/key-rest` | `main_pure_test.go`, `main_test.go` | Pure helpers like `formatPlacement`, plus subprocess tests of the built binary for `version` / `status` / unknown-command / `help` / `stop`-without-daemon |

Run with:

```bash
make test-go      # all packages, fail fast on regressions
make coverage     # text summary
make coverage-html # browsable HTML report at coverage.html
```

## System tests

Cross-language end-to-end tests live under `system-test/`. They start the real `key-rest` daemon, register all bundled service credentials, and drive every supported client (curl, Go, Python, Node.js) against the bundled `test-server`. See [`system-test/README.md`](../system-test/README.md) for details.

| Suite | Location | Driver |
|---|---|---|
| Curl | `system-test/curl/system-test.sh` | bash |
| Go | `system-test/go/system_test.go` | `go test` |
| Python | `system-test/python/system_test.py` | python3 |
| Node.js | `system-test/node/system_test.mjs` | node |

Run all suites with:

```bash
make test-system
```

Each suite expects the project binary to be built (`make build`) and the test-server certificate to be present.

## Security PoC / regression tests

Tests under `internal/proxy/` named `*_poc_test.go` started life as proofs of concept for the SPECA-style audit findings (see [`audit-speca.md`](audit-speca.md)). After each fix landed, the assertions were inverted so they now act as regressions:

- `headerinject_poc_test.go` — F-001 (header-key CRLF injection)
- `raceattack_poc_test.go` — F-002 (Disable / mask race), with both a deterministic gated variant and a 200-iteration natural-timing variant (the latter skipped under `-short`)
- `raceattack2_poc_test.go` — F-003 (reload-after-`keys.enc`-tamper race)
- `bodysmuggle_poc_test.go` — supplementary checks (Transfer-Encoding strip, validation race)

Run them under the race detector to verify there are no underlying data races:

```bash
go test -race -short ./internal/proxy
```

## CI

`.github/workflows/test.yml` runs the unit tests on every push to `main` and every pull request, prints the coverage summary, and uploads `coverage.out` as a build artifact.

## Test-server

`test-server/` provides a localhost HTTPS server that mocks every supported third-party service (OpenAI, Anthropic, Stripe, etc.). It is consumed by both system tests and (transitively, via the certificate) the hacking-challenge environment. The server itself has no `*_test.go` files — it is verified through the system-test suites that drive it.
