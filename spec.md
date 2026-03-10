[English](spec.md) | [Japanese](spec-ja.md)

# Internal Specification

## Data Storage

- Data directory: `~/.key-rest/`
- Encrypted key file: `~/.key-rest/keys.enc`
- Unix socket: `~/.key-rest/key-rest.sock`
- PID file: `~/.key-rest/key-rest.pid`

### keys.enc Format

Keys are encrypted with the passphrase and saved in the following format:

```json
{
  "keys": [
    {
      "uri": "user1/brave/api-key",
      "url_prefix": "https://api.search.brave.com/",
      "allow_url": false,
      "allow_body": false,
      "encrypted_value": "<encrypted key value (base64)>"
    }
  ]
}
```

Encryption method: AES-256-GCM (using a key derived from the passphrase via PBKDF2)

## Socket Communication Protocol

The key-rest client library and key-rest-daemon communicate via a Unix domain socket (`~/.key-rest/key-rest.sock`). Messages are newline-delimited JSON.

### Request Format

```json
{
  "type": "http",
  "method": "GET",
  "url": "https://api.example.com/data",
  "headers": {
    "Authorization": "Bearer key-rest://user1/example/api-key",
    "Content-Type": "application/json"
  },
  "body": null
}
```

### Response Format (Success)

```json
{
  "status": 200,
  "statusText": "OK",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": "{\"results\": [...]}"
}
```

### Response Format (Error)

```json
{
  "error": {
    "code": "KEY_NOT_FOUND",
    "message": "Key 'user1/example/api-key' not found"
  }
}
```

Error Codes:

| code | Description |
|------|-------------|
| `KEY_NOT_FOUND` | The specified key-rest:// URI is not registered |
| `URL_PREFIX_MISMATCH` | The request URL does not match the key's url_prefix |
| `HTTP_ERROR` | The HTTP request to the external service failed |

## Memory Security

All memory regions that hold plaintext secrets are locked with `mlock` (swap prevention) and zero-cleared when no longer needed.

### API key

#### Input (`key-rest add`)

| # | Memory region | Location | mlock | Zero-clear timing |
|---|--------------|----------|-------|-------------------|
| 1 | `oneByte` (1-byte read variable) | `main.go:317` | No (stack, 1 byte) | Overwritten on each keystroke |
| 2 | `buf` (4096-byte input buffer) | `main.go:313` | Yes | Immediately after copying to `result` (on Enter) |
| 3 | `result` = `value` (return value) | `main.go:328` | Yes | When `cmdAdd` returns (add command completes) |
| 4 | `key` (PBKDF2-derived AES key) | `crypto.go:39` | Yes | When `Encrypt` returns |
| 5 | `valueCopy` (daemon in-memory copy) | `keystore.go:129` | Yes | On `ClearAll` (daemon stop or reload) |

#### Decryption (daemon start / reload via `DecryptAll`)

| # | Memory region | Location | mlock | Zero-clear timing |
|---|--------------|----------|-------|-------------------|
| 1 | `key` (PBKDF2-derived AES key) | `crypto.go:79` | Yes | When `Decrypt` returns (per-key) |
| 2 | `plaintext` from `gcm.Open` (decrypted API key) | `crypto.go:93` | Yes (at `keystore.go:233`) | On `ClearAll` (daemon stop or reload) |
| 3 | `DecryptedKey.Value` (daemon in-memory storage) | `keystore.go:235` | Yes | On `ClearAll` (daemon stop or reload) |

Note: #2 and #3 are the same underlying memory. `gcm.Open` allocates the buffer, which is returned through `Decrypt` → `DecryptAll`, and mlocked at `keystore.go:233`.

#### Request handling (each API call via `proxy.Handle`)

| # | Memory region | Location | mlock | Zero-clear timing |
|---|--------------|----------|-------|-------------------|
| 1 | `string(val)` (API key converted to Go string) | `uri.go:191` | **No** | GC (uncontrollable) |
| 2 | `resolvedURL` / `resolvedHeaders` / `resolvedBody` (strings with embedded API key) | `proxy.go:68,76,86` | **No** | GC (uncontrollable) |
| 3 | `http.Request` fields (URL, Header, Body) | `net/http` internal | **No** | GC (uncontrollable) |

**Limitation:** During request handling, API key values are converted to Go strings (`string(val)` at `uri.go:191`) for URI replacement and HTTP request construction. Go strings are immutable and GC-managed, so they cannot be mlocked or zero-cleared. These strings are short-lived (exist only for the duration of the request) and become GC-eligible immediately after the request completes.

### Master key (passphrase)

#### Input — parent process (`key-rest start`, terminal side)

| # | Memory region | Location | mlock | Zero-clear timing |
|---|--------------|----------|-------|-------------------|
| 1 | `oneByte` (1-byte read variable) | `main.go:317` | No (stack, 1 byte) | Overwritten on each keystroke |
| 2 | `buf` (4096-byte input buffer) | `main.go:313` | Yes | Immediately after copying to `result` (on Enter) |
| 3 | `result` = `passphrase` (return value) | `main.go:328` | Yes | When `cmdStart` returns (after fork + pipe write) |

#### Input — child process (`key-rest start`, daemon, KEY_REST_FOREGROUND=1)

| # | Memory region | Location | mlock | Zero-clear timing |
|---|--------------|----------|-------|-------------------|
| 1 | `buf` (4096-byte pipe read buffer) | `main.go:279` | Yes | Immediately after copying to `result` |
| 2 | `result` = `passphrase` (return value) | `main.go:290` | Yes | When `cmdStart` returns (after daemon shutdown) |
| 3 | `d.passphrase` (daemon's held copy) | `daemon.go:85` | Yes | In `shutdown()` (on SIGTERM) |
