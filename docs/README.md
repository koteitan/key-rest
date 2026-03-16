[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# Attack Vector Analysis

## Priority: High — Must mitigate in implementation

### 1. Key Theft via Same Service
- An agent embeds a key-rest:// URI in the request body and relays the key plaintext externally through the same service's message-sending functionality
- The url_prefix check passes (since the request goes to the same service)
- The daemon substitutes key-rest:// URIs in the body as well, so the plaintext is sent as message content
- Example: Including `key-rest://user1/slack/bot-token` in the text field of Slack's chat.postMessage causes the token plaintext to be posted to an attacker's channel
- The same attack is possible with any service that has message-sending capability: Telegram, LINE, Discord, etc.
- When a service has multiple keys, a key not used for authentication can be embedded in the body and stolen
- **Mitigation**: Restrict which fields each key can be substituted into (headers: allowed by default, url/body: explicit opt-in)

### 2. Key Leakage from Responses
- Some APIs echo back request headers (= credentials) in error responses
- The agent can see the key by receiving the response
- **Mitigation**: The daemon reverse-substitutes credential strings in responses back to key-rest:// URIs

### 3. Socket Permission Misconfiguration
- If the socket file permissions are not 0600, other users on the same machine can send requests
- **Mitigation**: Set permissions to 0600 when creating the socket

### 4. Request Injection (CRLF injection)
- Injecting \r\n into header values to tamper with HTTP headers
- Example: `Authorization: Bearer key-rest://user1/key\r\nHost: evil.com`
- **Mitigation**: Validate that substituted header values do not contain \r\n

### 5. URL Parse Inconsistency
- `https://api.example.com@evil.com/` — Exploiting the userinfo portion to send to a different host
- `https://api.example.com.evil.com/` — Prefix matching alone can be bypassed via subdomains
- **Mitigation**: Parse the URL and compare by scheme+host+port+path after normalization (not by string prefix matching)

## Priority: Medium — Should mitigate in implementation

### 6. Passphrase Brute Force
- If the PBKDF2 iteration count is low, offline attacks become practical
- Reusing salt allows multiple keys to be broken simultaneously
- **Mitigation**: Iteration count of 600,000 or more, generate a random salt per key

### 7. Process Memory Reading
- Reading decrypted keys from the same user via /proc/PID/mem or ptrace
- Plaintext keys included in core dumps
- Writing to swap files
- **Mitigation**: Prevent swapping with mlock, disable core dumps (RLIMIT_CORE=0), zero-clear after use

## Priority: Low — Mitigated by existing mechanisms

### 8. Credential Leakage on Redirect
- When a service returns a 3xx redirect, credentials leak if the HTTP client copies the Authorization header to the new request to the redirect target
- Credentials are not included in the response. The issue is whether the HTTP client carries over headers
- Go's net/http automatically removes the Authorization header on redirects to different hosts, so this is practically not exploitable
- This is an HTTP client implementation issue, not a key-rest vulnerability
- **Mitigation**: Configure the daemon's HTTP client to not follow redirects (as a precaution)

### 9. Socket Flood
- Overloading the daemon with a large volume of requests
- File descriptor exhaustion
- The daemon is intended to be used only by a local LLM agent, so external attacks are unlikely
- **Mitigation**: Set a maximum concurrent connection limit

## Deep Dives

### Attacks on HTTP clients

How key-rest intercepts and protects credentials in the Go HTTP client call chain.

- [Attacks on HTTP clients](http.md)

### Key recreation

Investigation of whether API credentials can be used to create new credentials of the same type via service APIs.

- [Key recreation](key-recreation.md)

### Credential exfiltration resistance

Per-service analysis of whether `--allow-only-*` options prevent credential exfiltration via write-then-read attacks.

- [Credential exfiltration resistance](exfiltration-resistance.md)
