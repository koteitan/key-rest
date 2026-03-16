[← Back](README.md) | [English](key-recreation.md) | [Japanese](key-recreation-ja.md)

# Key Recreation

## What is key recreation?

key-rest prevents the LLM agent from **seeing** credential values, but for services whose API allows creating a new credential of the same type using an existing one, key-rest cannot prevent it. The agent could call the credential creation endpoint via key-rest and obtain the new credential from the response.

We call this property **key recreation** — whether a credential can create another credential of the same type.

This document investigates whether key recreation is possible for each service's credential type.

## Investigation

### Cannot prevent key recreation

| Service | Credential | Details | Date |
|---|---|---|---|
| Atlassian | PAT (Data Center/Server) | `POST /rest/pat/latest/tokens` creates a new PAT. An existing PAT inherits the user's full permissions with no scope restriction. | 2026-03-16 |

### Preventable with proper configuration

| Service | Credential | Details | Date |
|---|---|---|---|
| Cloudflare | API token | Token creation API exists. Preventable by not granting the `API Tokens Write` permission. | 2026-03-16 |
| OpenAI | API key | Admin API Key can create keys via `POST /v1/organization/admin_api_keys`. Preventable by not granting admin keys. Regular project API keys have no creation endpoint. | 2026-03-16 |

### Key recreation not possible

| Service | Credential | Details | Date |
|---|---|---|---|
| GitHub | PAT (classic/fine-grained) | No API endpoint exists to create PATs. `POST /authorizations` was deprecated in 2020. | 2026-03-16 |
| GitHub | OAuth token | Requires the OAuth flow (browser authorization + `client_id` + `client_secret`). None obtainable from the token itself. | 2026-03-16 |
| Atlassian | API token (Cloud) | No API endpoint exists. Only creatable via the web UI. | 2026-03-16 |
| Azure | Service key (e.g., Azure OpenAI) | Service keys cannot authenticate the ARM API that manages them. Requires a separate Azure AD token. | 2026-03-16 |
| Oracle Cloud | API signing key | `UploadApiKey` API can upload a public key, but the private key is generated client-side and never returned in the response. | 2026-03-16 |
| Anthropic | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| GitLab | PAT | `POST /api/v4/user/personal_access_tokens` exists but created tokens are limited to `k8s_proxy` and `self_rotate` scopes only. | 2026-03-16 |
| Mistral | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Groq | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| xAI | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Perplexity | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| DeepSeek | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| OpenRouter | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Google (Gemini) | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Google Search | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Exa | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Brave | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Bing | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Tavily | API key | No API endpoint exists to create API keys. | 2026-03-16 |
| Slack | Bot token | Requires the OAuth flow (user authorization in browser). | 2026-03-16 |
| Discord | Bot token | Only creatable via the Developer Portal web UI. | 2026-03-16 |
| Telegram | Bot token | Only creatable via BotFather interaction. | 2026-03-16 |
| LINE | Channel access token | Requires the OAuth flow with `client_id` + `client_secret`. | 2026-03-16 |
| Matrix | Access token | Requires username + password authentication. | 2026-03-16 |
| Linear | API key | Only creatable via the web UI. | 2026-03-16 |
| Notion | Integration token | Internal tokens are created via the web UI. Public integrations require the OAuth flow with `client_id` + `client_secret`. | 2026-03-16 |
| Sentry | Auth token | Only creatable via the web UI. | 2026-03-16 |
| Trello | API key | Only creatable via the web UI. | 2026-03-16 |
