[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

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

## 証明書の設定

test-server は自己署名証明書を生成します。key-rest-daemon は test-server に HTTPS リクエストを送るため、この証明書を信頼する必要があります。**daemon 起動前に** `SSL_CERT_FILE` 環境変数を設定してください:

```bash
export SSL_CERT_FILE=test-server/cert.pem
./key-rest start
```

システムテストではこれを自動的に設定しています。手動でローカルテストする場合は、生成された `cert.pem` を指す `SSL_CERT_FILE` を daemon 起動前に export してください。

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

## 関連

- [システムテスト](../system-test/README-ja.md) — このサーバーを使用した自動エンドツーエンドテスト
