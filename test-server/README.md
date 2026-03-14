[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# test-server

A mock HTTPS server that fully mimics the authentication behavior of all 26 services supported by key-rest [examples](../examples/README.md). Each service validates credentials and returns realistic error responses matching the real API when authentication fails.

URL structure: `https://localhost:PORT/SERVICE_NAME/original-path`

## Build and Run

```bash
go run ./test-server/
```

On startup, the server generates random test credentials for all services and prints them to stdout:

```
=== Test Credentials ===
  openai api-key             sk-test-09c2d23f...
  anthropic api-key          sk-ant-api03-test-0b98f765...
  github token               ghp_teste0bb15cd...
  ...
========================
```

The server generates a self-signed certificate on first run (`test-server/cert.pem`, `test-server/key.pem`).

Options:
```
-port      HTTPS port (default: 9443)
-cert      TLS certificate file (default: test-server/cert.pem)
-key       TLS private key file (default: test-server/key.pem)
-gen-cert  Force regenerate self-signed certificate
```

## Services

| Service | Auth Method | Test URL |
|---------|-------------|----------|
| openai | Bearer token | `/openai/v1/chat/completions` |
| anthropic | `X-Api-Key` header | `/anthropic/v1/messages` |
| gemini | `key` query param | `/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=...` |
| github | Bearer token | `/github/user/repos` |
| google-search | `key` query param | `/google-search/customsearch/v1?key=...` |
| tavily | `api_key` body field | `/tavily/search` |
| exa | `X-Api-Key` header | `/exa/search` |
| gitlab | `Private-Token` header | `/gitlab/api/v4/projects` |
| matrix | Bearer token | `/matrix/_matrix/client/v3/rooms/ROOM/send/m.room.message` |
| mistral | Bearer token | `/mistral/v1/chat/completions` |
| brave | `X-Subscription-Token` header | `/brave/res/v1/web/search?q=...` |
| slack | Bearer token | `/slack/api/chat.postMessage` |
| linear | Raw `Authorization` header | `/linear/graphql` |
| atlassian | Basic auth (base64) | `/atlassian/2.0/repositories/...` |
| openrouter | Bearer token | `/openrouter/api/v1/chat/completions` |
| bing | `Ocp-Apim-Subscription-Key` header | `/bing/v7.0/search?q=...` |
| sentry | Bearer token | `/sentry/api/0/projects/` |
| groq | Bearer token | `/groq/openai/v1/chat/completions` |
| telegram | Path embedding (`/botTOKEN/method`) | `/telegram/botTOKEN/sendMessage` |
| trello | `key` + `token` query params | `/trello/1/members/me/boards?key=...&token=...` |
| xai | Bearer token | `/xai/v1/chat/completions` |
| perplexity | Bearer token | `/perplexity/chat/completions` |
| line | Bearer token | `/line/v2/bot/message/push` |
| discord | `Bot` prefix token | `/discord/api/v10/channels/CH/messages` |
| deepseek | Bearer token | `/deepseek/chat/completions` |
| notion | Bearer token | `/notion/v1/databases/DB/query` |

## Response Format

### Authentication Success

```json
{
  "ok": true,
  "service": "openai",
  "auth": "sk-test-09c2d23f...",
  "method": "POST",
  "path": "/openai/v1/chat/completions"
}
```

Special cases:
- **trello**: includes `auth_extra` for the second query param (`token`)
- **atlassian**: includes `auth_user` and `auth_pass` (decoded from base64)
- **tavily**: includes `body_fields` with the rest of the JSON body

### Authentication Failure

Each service returns error responses mimicking the real API. For example:

**OpenAI** (and other OpenAI-compatible services):
```json
{
  "error": {
    "message": "Incorrect API key provided: sk-inva********key. You can find your API key at https://platform.openai.com/account/api-keys.",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

**Anthropic**:
```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "invalid x-api-key"
  }
}
```

**GitHub**:
```json
{
  "message": "Bad credentials",
  "documentation_url": "https://docs.github.com/rest"
}
```

Other services follow their respective real API error formats.

## See Also

- [System Tests](../system-test/README.md) — automated end-to-end tests using this server
