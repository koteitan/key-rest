[English](README.md) | [Japanese](README-ja.md)

# test-server

key-rest の[使用例](../examples/README-ja.md)で対応している全 26 サービスの認証動作を完全に模倣するモック HTTPS サーバーです。各サービスはクレデンシャルを検証し、認証失敗時には実際の API と同じエラーレスポンスを返します。

URL 構造: `https://localhost:PORT/サービス名/元のパス`

## ビルドと起動

```bash
go run ./test-server/
```

起動時にランダムなテスト用クレデンシャルを全サービス分生成し、標準出力に表示します:

```
=== Test Credentials ===
  openai api-key             sk-test-09c2d23f...
  anthropic api-key          sk-ant-api03-test-0b98f765...
  github token               ghp_teste0bb15cd...
  ...
========================
```

初回起動時に自己署名証明書を自動生成します（`test-server/cert.pem`, `test-server/key.pem`）。

オプション:
```
-port      HTTPS ポート (デフォルト: 9443)
-cert      TLS 証明書ファイル (デフォルト: test-server/cert.pem)
-key       TLS 秘密鍵ファイル (デフォルト: test-server/key.pem)
-gen-cert  自己署名証明書を強制再生成
```

## 対応サービス

| サービス | 認証方式 | テスト URL |
|----------|----------|-----------|
| openai | Bearer トークン | `/openai/v1/chat/completions` |
| anthropic | `X-Api-Key` ヘッダー | `/anthropic/v1/messages` |
| gemini | `key` クエリパラメータ | `/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=...` |
| github | Bearer トークン | `/github/user/repos` |
| google-search | `key` クエリパラメータ | `/google-search/customsearch/v1?key=...` |
| tavily | `api_key` ボディフィールド | `/tavily/search` |
| exa | `X-Api-Key` ヘッダー | `/exa/search` |
| gitlab | `Private-Token` ヘッダー | `/gitlab/api/v4/projects` |
| matrix | Bearer トークン | `/matrix/_matrix/client/v3/rooms/ROOM/send/m.room.message` |
| mistral | Bearer トークン | `/mistral/v1/chat/completions` |
| brave | `X-Subscription-Token` ヘッダー | `/brave/res/v1/web/search?q=...` |
| slack | Bearer トークン | `/slack/api/chat.postMessage` |
| linear | `Authorization` ヘッダー (プレフィックスなし) | `/linear/graphql` |
| atlassian | Basic 認証 (base64) | `/atlassian/2.0/repositories/...` |
| openrouter | Bearer トークン | `/openrouter/api/v1/chat/completions` |
| bing | `Ocp-Apim-Subscription-Key` ヘッダー | `/bing/v7.0/search?q=...` |
| sentry | Bearer トークン | `/sentry/api/0/projects/` |
| groq | Bearer トークン | `/groq/openai/v1/chat/completions` |
| telegram | パス埋め込み (`/botTOKEN/method`) | `/telegram/botTOKEN/sendMessage` |
| trello | `key` + `token` クエリパラメータ | `/trello/1/members/me/boards?key=...&token=...` |
| xai | Bearer トークン | `/xai/v1/chat/completions` |
| perplexity | Bearer トークン | `/perplexity/chat/completions` |
| line | Bearer トークン | `/line/v2/bot/message/push` |
| discord | `Bot` プレフィックス トークン | `/discord/api/v10/channels/CH/messages` |
| deepseek | Bearer トークン | `/deepseek/chat/completions` |
| notion | Bearer トークン | `/notion/v1/databases/DB/query` |

## レスポンス形式

### 認証成功時

```json
{
  "ok": true,
  "service": "openai",
  "auth": "sk-test-09c2d23f...",
  "method": "POST",
  "path": "/openai/v1/chat/completions"
}
```

特殊なケース:
- **trello**: 2つ目のクエリパラメータ (`token`) を `auth_extra` に含む
- **atlassian**: base64 デコードした結果を `auth_user` と `auth_pass` に含む
- **tavily**: API キー以外のボディフィールドを `body_fields` に含む

### 認証失敗時

各サービスは実際の API と同じエラーレスポンスを返します。例:

**OpenAI** (他の OpenAI 互換サービスも同様):
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

他のサービスもそれぞれの実 API のエラー形式に準拠しています。

## key-rest との連携テスト

### 1. 証明書の信頼設定

```bash
# 方法 A: システムのトラストストアに追加 (Ubuntu/WSL2)
sudo cp test-server/cert.pem /usr/local/share/ca-certificates/key-rest-test.crt
sudo update-ca-certificates

# 方法 B: 環境変数で指定
export SSL_CERT_FILE=test-server/cert.pem
```

### 2. テスト用キーの登録

```bash
# Bearer トークン系サービス
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

# カスタムヘッダー系サービス
./key-rest add user1/exa/api-key          https://localhost:9443/exa/
./key-rest add user1/brave/api-key        https://localhost:9443/brave/
./key-rest add user1/gitlab/token         https://localhost:9443/gitlab/
./key-rest add user1/bing/api-key         https://localhost:9443/bing/

# クエリパラメータ系サービス (--allow-url 必須)
./key-rest add --allow-url user1/gemini/api-key       https://localhost:9443/gemini/
./key-rest add --allow-url user1/google/api-key        https://localhost:9443/google-search/
./key-rest add --allow-url user1/trello/api-key        https://localhost:9443/trello/
./key-rest add --allow-url user1/trello/token          https://localhost:9443/trello/

# ボディフィールド系サービス (--allow-body 必須)
./key-rest add --allow-body user1/tavily/api-key       https://localhost:9443/tavily/

# パス埋め込み系サービス (--allow-url 必須)
./key-rest add --allow-url user1/telegram/bot-token    https://localhost:9443/telegram/

# Basic 認証 (2つのキー)
./key-rest add user1/atlassian/email      https://localhost:9443/atlassian/
./key-rest add user1/atlassian/token      https://localhost:9443/atlassian/
```

### 3. curl でテスト

```bash
# Bearer トークン (OpenAI)
echo '{"type":"http","method":"POST","url":"https://localhost:9443/openai/v1/chat/completions","headers":{"Authorization":"Bearer key-rest://user1/openai/api-key","Content-Type":"application/json"},"body":"{\"model\":\"gpt-4o\"}"}' | socat - UNIX-CONNECT:~/.key-rest/key-rest.sock

# カスタムヘッダー (Anthropic)
echo '{"type":"http","method":"POST","url":"https://localhost:9443/anthropic/v1/messages","headers":{"X-Api-Key":"key-rest://user1/anthropic/api-key","Content-Type":"application/json"},"body":"{\"model\":\"claude-sonnet-4-20250514\"}"}' | socat - UNIX-CONNECT:~/.key-rest/key-rest.sock

# クエリパラメータ (Gemini)
echo '{"type":"http","method":"POST","url":"https://localhost:9443/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key","headers":{"Content-Type":"application/json"},"body":"{}"}' | socat - UNIX-CONNECT:~/.key-rest/key-rest.sock
```
