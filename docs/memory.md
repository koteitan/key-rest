[← Back](README.md) | [English](memory.md) | [Japanese](memory-ja.md)

# Credential Memory Lifecycle

This document traces the complete memory lifecycle of a decrypted API key, from decryption through TLS encryption and zero-clear, identifying where credentials are protected and where gaps exist.

## Summary

The **request path** (credential substitution → TLS encryption) is well protected with mlock and zero-clear. Credentials exist as `string` only in the **response masking path**, where Go's immutable strings prevent zero-clearing.

## Phase 1: Decryption and Storage

`internal/keystore/keystore.go:DecryptAll()` (line 258)

| Step | Type | mlocked | Zero-cleared | Notes |
|---|---|---|---|---|
| `crypto.Decrypt()` returns plaintext | `[]byte` | Yes | - | `crypto.Mlock(value)` at line 280 |
| Stored in `DecryptedKey.Value` | `[]byte` | Yes | On ClearAll/Disable | Remains in memory for daemon lifetime |

**Status:** Secure. Plaintext is mlocked immediately after decryption.

## Phase 2: Request Validation

`internal/proxy/proxy.go:Handle()` (line 109)

| Step | Type | mlocked | Zero-cleared | Notes |
|---|---|---|---|---|
| `validateField()` scans for `key-rest://` URIs | `string` | - | - | Only URI strings, no credential values |
| `store.Lookup()` returns `*DecryptedKey` | pointer | Yes | - | Returns pointer to mlocked data |
| `collectTransformOutputs()` | `string` | **No** | **No** | See [Vulnerability #1](#vulnerability-1-collecttransformoutputs) |

## Phase 3: Delayed Resolution and TLS Encryption

`internal/proxy/sectransport.go:RoundTrip()` (line 54)

This is the core security design: credentials stay as `key-rest://` URIs through the entire `net/http` pipeline and are only resolved immediately before TLS encryption.

### 3a. URI Resolution

| Step | Line | Type | mlocked | Zero-cleared | Notes |
|---|---|---|---|---|---|
| `uri.ReplaceBytes(body)` | 70 | `[]byte` | Yes | Yes (line 164) | Credential substituted into body |
| `uri.ReplaceBytes(URI)` | 86 | `[]byte` | Yes | Yes (line 163) | Credential substituted into URL path |
| `uri.ReplaceBytes(header)` | 98 | `[]byte` | Yes | Yes (line 165-167) | Per-header resolution |

Inside `uri.ReplaceBytes()` (`internal/uri/uri.go:227`):
- Line 286: `copy(cpy, val)` copies from keystore — intermediate copy
- Line 268: `zeroClear(r.value)` clears each resolved match
- Line 316: `zeroClear(concatenated)` clears base64 concatenation buffer
- Line 263: Result buffer is pre-allocated (no `append` reallocation)

**Status:** Secure. All intermediate `[]byte` buffers are zero-cleared.

### 3b. mlocked Buffer Construction

| Step | Line | Type | mlocked | Zero-cleared | Notes |
|---|---|---|---|---|---|
| Allocate exact-size buffer | 142 | `[]byte` | Yes | - | `crypto.Mlock(buf)` |
| `copy()` resolved fields into buffer | 145-160 | `[]byte` | Yes | - | Raw HTTP/1.1 request built in-place |
| Zero-clear intermediate buffers | 163-167 | - | - | Yes | `resolvedURI`, `resolvedBody`, headers |

**Status:** Secure. No reallocation possible (exact size calculated in advance).

### 3c. TLS Write and Zero-Clear

| Step | Line | Type | mlocked | Zero-cleared | Notes |
|---|---|---|---|---|---|
| `conn.Write(buf)` | 191 | `[]byte` | Yes | - | Written to TLS connection |
| `crypto.ZeroClearAndMunlock(buf)` | 192 | - | - | Yes | Immediate zero-clear after write |

**Status:** Secure. Credential exists in plaintext only during `conn.Write()`.

## Phase 4: Response Masking

`internal/proxy/proxy.go:Handle()` (line 190-206)

This phase has known vulnerabilities due to Go's immutable `string` type.

### <a id="vulnerability-1-collecttransformoutputs"></a>Vulnerability #1: `collectTransformOutputs()`

`proxy.go` line 534-562

```go
resolved, err := uri.ResolveMatch(m, resolver)  // returns string
outputs[resolved] = original                     // string key in map
```

`uri.ResolveMatch()` internally converts `[]byte` → `string` (immutable, cannot be zero-cleared). The base64-encoded credential value persists in the Go heap as a map key until GC collects it.

### Vulnerability #2: `maskCredentials()`

`proxy.go` line 443-471

```go
raw := string(dk.Value)                          // []byte → string copy
jsonBytes, _ := json.Marshal(raw)                // new []byte with JSON-escaped credential
jsonEscaped := string(jsonBytes[1:len(jsonBytes)-1])  // another string copy
```

Three uncleared copies of the credential are created:
1. `raw` — plaintext as `string`
2. `jsonBytes` — JSON-escaped as `[]byte`
3. `jsonEscaped` — JSON-escaped as `string`

None can be zero-cleared (`string` is immutable, `jsonBytes` is not explicitly cleared).

### Vulnerability #3: `maskTruncatedKeys()`

`proxy.go` line 484-513

```go
raw := string(dk.Value)  // []byte → string copy
```

Same issue as Vulnerability #2. The plaintext credential becomes an immutable `string`.

### Vulnerability #4: `maskPercentEncoded()`

`proxy.go` line 519-532

Calls `maskCredentials()` internally, inheriting Vulnerability #2.

## Lifecycle Summary

```
                    Request Path (Secure)
                    =====================
keystore            key-rest://URI         mlocked buf        TLS
[Value]  ──copy──►  [ReplaceBytes]  ──copy──►  [HTTP/1.1]  ──write──►  encrypted
 mlock      zero     []byte           zero      mlock         zero
            clear                     clear                   clear

                    Response Path (Vulnerable)
                    ==========================
keystore            maskCredentials         strings.ReplaceAll
[Value]  ──string()──►  [raw string]  ──ReplaceAll──►  [masked body]
 mlock       ⚠️ NO      ⚠️ NO zero       GC manages
             zero       clear possible    old strings
             clear
```

## Threat Assessment

| Attacker | Can read GC remnants? | Notes |
|---|---|---|
| LLM agent (same user) | **No** | `PR_SET_DUMPABLE=0` blocks `/proc/PID/mem` |
| Other user | **No** | Unix permissions |
| root | **Yes** | Can read any process memory |
| Disk forensics (swap) | **Possible** | GC-managed `string` is not mlocked, may be swapped |

**Conclusion:** The GC vulnerability is not exploitable under key-rest's threat model (LLM agent as same user) because `PR_SET_DUMPABLE=0` prevents memory reads. It becomes relevant only for root-level attackers or disk forensics, both of which are outside key-rest's protection scope.

## Potential Improvement

Replace `string`-based masking functions with `[]byte`-based equivalents that use mlock and zero-clear. This would eliminate all four vulnerabilities but requires reimplementing `strings.ReplaceAll`, `json.Marshal`, and `regexp.ReplaceAllString` with `[]byte` + mlock variants.
