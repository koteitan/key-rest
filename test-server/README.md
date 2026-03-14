[English](README.md) | [Japanese](README-ja.md)

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

## Testing with key-rest

### 1. Trust the certificate

```bash
# Option A: Add to system trust store (Ubuntu/WSL2)
sudo cp test-server/cert.pem /usr/local/share/ca-certificates/key-rest-test.crt
sudo update-ca-certificates

# Option B: Use environment variable
export SSL_CERT_FILE=test-server/cert.pem
```

### 2. Register test keys

```bash
# Bearer token services
./key-rest add user1/openai/api-key       https://localhost:9443/openai/
./key-rest add user1/anthropic/api-key    https://localhost:9443/anthropic/
./key-rest add user1/github/token         https://localhost:9443/github/
./key-rest add user1/mistral/api-key      https://localhost:9443/mistral/
./key-rest add user1/slack/bot-token      https://localhost:9443/slack/
./key-rest add user1/openrouter/api-key   https://localhost:9443/openrouter/
./key-rest add user1/sentry/auth-token    https://localhost:9443/sentry/
./key-rest add user1/groq/api-key         https://localhost:9443/groq/
./key-rest add user1/xai/api-key          https://localhost:9443/xai/
./key-rest add user1/perplexity/api-key   https://localhost:9443/perplexity/
./key-rest add user1/line/channel-access-token https://localhost:9443/line/
./key-rest add user1/deepseek/api-key     https://localhost:9443/deepseek/
./key-rest add user1/notion/api-key       https://localhost:9443/notion/
./key-rest add user1/matrix/access-token  https://localhost:9443/matrix/
./key-rest add user1/discord/bot-token    https://localhost:9443/discord/
./key-rest add user1/linear/api-key       https://localhost:9443/linear/

# Custom header services
./key-rest add user1/exa/api-key          https://localhost:9443/exa/
./key-rest add user1/brave/api-key        https://localhost:9443/brave/
./key-rest add user1/gitlab/token         https://localhost:9443/gitlab/
./key-rest add user1/bing/api-key         https://localhost:9443/bing/

# Query parameter services (--allow-url required)
./key-rest add --allow-url user1/gemini/api-key       https://localhost:9443/gemini/
./key-rest add --allow-url user1/google/api-key        https://localhost:9443/google-search/
./key-rest add --allow-url user1/trello/api-key        https://localhost:9443/trello/
./key-rest add --allow-url user1/trello/token          https://localhost:9443/trello/

# Body field services (--allow-body required)
./key-rest add --allow-body user1/tavily/api-key       https://localhost:9443/tavily/

# Path embedding services (--allow-url required)
./key-rest add --allow-url user1/telegram/bot-token    https://localhost:9443/telegram/

# Basic auth (two keys)
./key-rest add user1/atlassian/email      https://localhost:9443/atlassian/
./key-rest add user1/atlassian/token      https://localhost:9443/atlassian/
```

### 3. Test with key-rest-curl

```bash
# Bearer token (OpenAI)
./clients/curl/key-rest-curl https://localhost:9443/openai/v1/chat/completions \
  -H "Authorization: Bearer key-rest://user1/openai/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o"}'

# Custom header (Anthropic)
./clients/curl/key-rest-curl https://localhost:9443/anthropic/v1/messages \
  -H "X-Api-Key: key-rest://user1/anthropic/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514"}'

# Query parameter (Gemini)
./clients/curl/key-rest-curl https://localhost:9443/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key \
  -H "Content-Type: application/json" \
  -d '{}'
```
