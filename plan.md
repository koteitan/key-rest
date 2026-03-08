[English](plan.md) | [Japanese](plan-ja.md)

# Implementation Plan

## Platform Selection

### daemon: Go

| Aspect | Go | Node.js | Rust |
|------|-----|---------|------|
| Secret Memory Management | ◯ Can explicitly zero-clear byte slices | ✕ Uncontrollable due to GC + immutable strings | ◎ Automatic erasure with zeroize crate |
| Crypto Primitives | ◯ crypto/aes, crypto/cipher (GCM), crypto/pbkdf2 in stdlib | ◯ crypto module available | ◯ ring/rust-crypto |
| Daemonization | ◯ Single binary, no dependencies | △ Requires node runtime | ◯ Single binary |
| Unix Socket | ◯ net.Listen("unix", ...) | ◯ net.createServer | ◯ tokio/std |
| mlock (Memory Lock) | ◯ syscall.Mlock | ✕ No standard support | ◯ memsec crate |
| Development Speed | ◯ | ◎ | △ |

**Selection Rationale:** For a daemon handling credentials, explicit memory control of secrets (zero-clear, mlock) is critical. Go has a comprehensive crypto stdlib and can be distributed as a single binary. It has a lower learning curve than Rust and handles secrets more safely than Node.js.

### clients: Native per Language

Clients are thin wrappers that send/receive JSON over Unix sockets, so they are implemented following each language's idioms.

| Client | Language | Interface |
|-------------|------|----------------|
| key-rest-fetch | Node.js (TypeScript) | fetch compatible |
| key-rest-ws | Node.js (TypeScript) | WebSocket compatible |
| key-rest-http | Go | net/http compatible |
| key-rest-requests | Python | requests compatible |
| key-rest-httpx | Python | httpx compatible |
| key-rest-curl | Shell (bash) | curl wrapper |

## Directory Structure

```
key-rest/
├── CLAUDE.md
├── spec.md
├── plan.md
├── examples/                  # Usage Examples (existing)
│   ├── README.md
│   └── *.md
│
├── go.mod                     # Go module root
├── go.sum
├── Makefile                   # build, test, install
│
├── cmd/                       # CLI entry point
│   └── key-rest/
│       └── main.go            # ./key-rest start|stop|status|add|remove|list|curl
│
├── internal/                  # Daemon internal packages (not importable externally)
│   ├── daemon/                # Process management (start/stop/status, PID file)
│   ├── crypto/                # AES-256-GCM encrypt/decrypt, PBKDF2 key derivation
│   ├── keystore/              # keys.enc read/write, key management
│   ├── server/                # Unix socket server, JSON protocol handling
│   ├── proxy/                 # HTTP/WebSocket proxy, external service calls
│   └── uri/                   # key-rest:// URI parsing, substitution (enclosed/unenclosed, transform functions)
│
├── clients/
│   ├── node/                  # Node.js client
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   └── src/
│   │       ├── fetch.ts       # createFetch()
│   │       └── ws.ts          # createWebSocket()
│   │
│   ├── go/                    # Go client
│   │   ├── go.mod
│   │   ├── client.go          # NewClient(), NewRequest()
│   │   └── client_test.go
│   │
│   ├── python/                # Python client
│   │   ├── pyproject.toml
│   │   └── key_rest/
│   │       ├── __init__.py
│   │       ├── requests.py    # requests compatible
│   │       └── httpx.py       # httpx compatible
│   │
│   └── curl/                  # curl wrapper
│       └── key-rest-curl.sh
│
└── test/                      # Integration tests
    ├── integration_test.go    # Daemon + client integration tests
    └── testdata/              # Encrypted keys for testing, etc.
```

## internal/ Package Responsibilities

| Package | Responsibilities | Security Notes |
|-----------|------|---------------------|
| `crypto` | AES-256-GCM encrypt/decrypt, PBKDF2 key derivation, salt generation | Use crypto/rand only, zero-clear key byte slices after use |
| `keystore` | keys.enc CRUD, hold decrypted keys in memory | Prevent swapping of decrypted keys with mlock, file permission 0600 |
| `daemon` | Process fork/management, PID file, signal handling | Graceful shutdown on SIGTERM, zero-clear before exit |
| `server` | Unix socket listener, JSON request/response handling | Socket permission 0600, request size limit |
| `proxy` | HTTP/WebSocket request proxying, response relay | TLS verification required, timeout configuration |
| `uri` | key-rest:// URI detection/substitution, enclosed/unenclosed parsing, transform functions | Prefix-match validation of url_prefix (prevent key leakage) |

## Implementation Order

1. `internal/crypto` — Crypto foundation
2. `internal/keystore` — Key persistence
3. `cmd/key-rest` + `internal/daemon` — CLI and add/list/remove (parts that work without the daemon)
4. `internal/uri` — URI parsing/substitution engine
5. `internal/proxy` — HTTP proxy
6. `internal/server` — Unix socket server
7. `cmd/key-rest` — start/stop/status (daemonization)
8. `clients/curl` — Simplest client (for smoke testing)
9. `clients/node` — Node.js client
10. `clients/go` — Go client
11. `clients/python` — Python client
