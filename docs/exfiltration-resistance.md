[← Back](README.md) | [English](exfiltration-resistance.md) | [Japanese](exfiltration-resistance-ja.md)

# Credential Exfiltration Resistance

Per-service analysis of whether the `--allow-only-*` options prevent credential exfiltration via the write-then-read attack pattern ([Attack #1](README.md)).

## Attack Pattern

1. Agent embeds `key-rest://user1/service/key` in a writable field (e.g., comment body, message text)
2. key-rest daemon substitutes the credential into the field
3. The credential is stored on the server (as a comment, message, etc.)
4. Agent reads it back and obtains the credential

## Defense Layers

| Layer | What it blocks |
|---|---|
| `url_prefix` | Cross-service exfiltration: a key for service A cannot be substituted in a request to service B |
| `--allow-only-*` | Same-service exfiltration: a key can only be placed in the designated auth field, not in arbitrary body/header/URL fields |
| Response masking | Echo-back: credential values in responses are replaced with `key-rest://` URIs |

## Result Summary

All 27 services are protected when using the recommended `--allow-only-*` option from the [examples](../examples/).

Without `--allow-only-*`, services with write-then-read endpoints are vulnerable to Attack #1.

---

## Messaging Services

Services where the agent can write arbitrary text and read it back.

### Slack

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: `chat.postMessage`, `conversations.history`
- **Attack scenario**: Agent posts `key-rest://` URI as message text, reads it back via history API
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### Discord

- **Auth**: `Authorization: Bot <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: `POST /channels/{id}/messages`, `GET /channels/{id}/messages`
- **Attack scenario**: Agent posts credential as message content, reads it back
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### Telegram

- **Auth**: URL path `/bot<token>/` → `--allow-only-url`
- **Write-read endpoints**: `sendMessage`, `getUpdates`
- **Attack scenario**: Agent sends credential as message text, reads via getUpdates
- **Result**: Protected. Token restricted to URL; body substitution blocked.

### LINE

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: `POST /v2/bot/message/push` (write-only; no read API for sent messages)
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### Matrix

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: `PUT /rooms/{id}/send/m.room.message`, `GET /rooms/{id}/messages`
- **Attack scenario**: Agent sends credential as room message, reads it via sync/messages API
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

---

## Collaboration / Developer Tools

Services with issues, comments, documents, or similar writable resources.

### GitHub

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: Issues, comments, PRs, gists, README files
- **Attack scenario**: Agent posts credential as issue comment, reads it back via API
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### GitLab

- **Auth**: `Private-Token: <token>` → `--allow-only-header Private-Token`
- **Write-read endpoints**: Issues, comments, merge requests, snippets
- **Attack scenario**: Agent posts credential as issue note, reads it back
- **Result**: Protected. Token restricted to Private-Token header; body substitution blocked.

### Atlassian

- **Auth**: `Authorization: Basic <base64>` → `--allow-only-header Authorization`
- **Write-read endpoints**: Issues, comments, PRs (Bitbucket/Jira/Confluence)
- **Attack scenario**: Agent posts credential as PR comment, reads it back
- **Result**: Protected. Credentials restricted to Authorization header; body substitution blocked.

### Notion

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: Pages, databases, blocks
- **Attack scenario**: Agent creates a page with credential in content, reads it back
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### Trello

- **Auth**: `?key=<key>&token=<token>` → `--allow-only-query key`, `--allow-only-query token`
- **Write-read endpoints**: Cards, comments, lists
- **Attack scenario**: Agent creates a card with credential as description, reads it back
- **Result**: Protected. Keys restricted to query parameters; body substitution blocked.

### Linear

- **Auth**: `Authorization: <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: Issues, comments (GraphQL mutations/queries)
- **Attack scenario**: Agent creates issue with credential in description, reads it back
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### Sentry

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: Issue comments (limited)
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

### Cloudflare

- **Auth**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **Write-read endpoints**: DNS records (TXT records could encode data), Workers scripts
- **Attack scenario**: Agent creates DNS TXT record with credential, reads it back via API
- **Result**: Protected. Token restricted to Authorization header; body substitution blocked.

---

## AI Providers

Stateless chat/completion APIs. No persistent write-then-read endpoints.

### OpenAI

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Write-read endpoints**: None for chat. Files/Assistants API exists but responses are model-generated, not echoed.
- **Result**: Protected. Key restricted to Authorization header.

### Anthropic

- **Auth**: `X-Api-Key: <key>` → `--allow-only-header X-Api-Key`
- **Write-read endpoints**: None. Stateless message API.
- **Result**: Protected. Key restricted to X-Api-Key header.

### Gemini

- **Auth**: `?key=<key>` → `--allow-only-query key`
- **Write-read endpoints**: None. Stateless content generation API.
- **Result**: Protected. Key restricted to `key` query parameter.

### Mistral

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Result**: Protected. Same as OpenAI.

### Groq

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Result**: Protected. Same as OpenAI.

### xAI (Grok)

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Result**: Protected. Same as OpenAI.

### DeepSeek

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Result**: Protected. Same as OpenAI.

### Perplexity

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Result**: Protected. Same as OpenAI.

### OpenRouter

- **Auth**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **Result**: Protected. Same as OpenAI.

---

## Search Services

Read-only APIs. No write endpoints.

### Brave Search

- **Auth**: `X-Subscription-Token: <key>` → `--allow-only-header X-Subscription-Token`
- **Result**: Protected. Read-only API; no write endpoints.

### Google Custom Search

- **Auth**: `?key=<key>` → `--allow-only-query key`
- **Result**: Protected. Read-only API; no write endpoints.

### Bing Search

- **Auth**: `Ocp-Apim-Subscription-Key: <key>` → `--allow-only-header Ocp-Apim-Subscription-Key`
- **Result**: Protected. Read-only API; no write endpoints.

### Exa

- **Auth**: `X-Api-Key: <key>` → `--allow-only-header X-Api-Key`
- **Result**: Protected. Read-only API; no write endpoints.

### Tavily

- **Auth**: `{"api_key": "<key>"}` → `--allow-only-field api_key`
- **Result**: Protected. Key restricted to `api_key` JSON field; other fields blocked.
