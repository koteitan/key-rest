[← Back](README-ja.md) | [English](exfiltration-resistance.md) | [Japanese](exfiltration-resistance-ja.md)

# クレデンシャル奪取耐性

各サービスにおいて、`--allow-only-*` オプションが書き込み→読み取り攻撃パターン（[攻撃 #1](README-ja.md)）を防げるかどうかの分析。

## 攻撃パターン

1. エージェントが書き込み可能なフィールド（コメント本文、メッセージテキスト等）に `key-rest://user1/service/key` を埋め込む
2. key-rest daemon がクレデンシャルを置換する
3. クレデンシャルがサーバーに保存される（コメント、メッセージ等として）
4. エージェントがそれを読み取り、クレデンシャルを取得する

## 防御層

| 層 | 防ぐ攻撃 |
|---|---|
| `url_prefix` | クロスサービス奪取: サービス A 用のキーをサービス B へのリクエストで使うことを阻止 |
| `--allow-only-*` | 同一サービス奪取: キーは指定された認証フィールドにのみ配置され、任意の body/header/URL には配置されない |
| レスポンスマスキング | エコーバック: レスポンス内のクレデンシャル値を `key-rest://` URI に置換 |

## 結果まとめ

[examples](../examples/) で推奨される `--allow-only-*` オプションを使用する場合、全 27 サービスが保護されます。

`--allow-only-*` を付けない場合、書き込み→読み取りが可能なサービスは攻撃 #1 に対して脆弱です。

---

## メッセージングサービス

エージェントが任意のテキストを書き込み、読み取れるサービス。

### Slack

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: `chat.postMessage`, `conversations.history`
- **攻撃シナリオ**: エージェントが `key-rest://` URI をメッセージテキストとして投稿し、history API で読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Discord

- **認証**: `Authorization: Bot <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: `POST /channels/{id}/messages`, `GET /channels/{id}/messages`
- **攻撃シナリオ**: エージェントがメッセージ内容にクレデンシャルを投稿し、読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Telegram

- **認証**: URL パス `/bot<token>/` → `--allow-only-url`
- **書き込み→読み取りエンドポイント**: `sendMessage`, `getUpdates`
- **攻撃シナリオ**: エージェントがメッセージテキストとしてクレデンシャルを送信し、getUpdates で読み取る
- **結果**: 保護される。トークンは URL に制限され、body 置換はブロックされる。

### LINE

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: `POST /v2/bot/message/push`（書き込みのみ、送信メッセージの読み取り API なし）
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Matrix

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: `PUT /rooms/{id}/send/m.room.message`, `GET /rooms/{id}/messages`
- **攻撃シナリオ**: エージェントがルームメッセージとしてクレデンシャルを送信し、sync/messages API で読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

---

## コラボレーション / 開発ツール

issue、コメント、ドキュメント等の書き込み可能なリソースを持つサービス。

### GitHub

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: issue、コメント、PR、gist、README ファイル
- **攻撃シナリオ**: エージェントが issue コメントにクレデンシャルを投稿し、API で読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### GitLab

- **認証**: `Private-Token: <token>` → `--allow-only-header Private-Token`
- **書き込み→読み取りエンドポイント**: issue、コメント、マージリクエスト、スニペット
- **攻撃シナリオ**: エージェントが issue ノートにクレデンシャルを投稿し、読み取る
- **結果**: 保護される。トークンは Private-Token ヘッダーに制限され、body 置換はブロックされる。

### Atlassian

- **認証**: `Authorization: Basic <base64>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: issue、コメント、PR（Bitbucket/Jira/Confluence）
- **攻撃シナリオ**: エージェントが PR コメントにクレデンシャルを投稿し、読み取る
- **結果**: 保護される。クレデンシャルは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Notion

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: ページ、データベース、ブロック
- **攻撃シナリオ**: エージェントがクレデンシャルを含むページを作成し、読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Trello

- **認証**: `?key=<key>&token=<token>` → `--allow-only-query key`, `--allow-only-query token`
- **書き込み→読み取りエンドポイント**: カード、コメント、リスト
- **攻撃シナリオ**: エージェントがクレデンシャルをカードの説明として作成し、読み取る
- **結果**: 保護される。キーはクエリパラメータに制限され、body 置換はブロックされる。

### Linear

- **認証**: `Authorization: <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: issue、コメント（GraphQL mutation/query）
- **攻撃シナリオ**: エージェントがクレデンシャルを issue の説明として作成し、読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Sentry

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: issue コメント（限定的）
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

### Cloudflare

- **認証**: `Authorization: Bearer <token>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: DNS レコード（TXT レコードにデータをエンコード可能）、Workers スクリプト
- **攻撃シナリオ**: エージェントが DNS TXT レコードにクレデンシャルを作成し、API で読み取る
- **結果**: 保護される。トークンは Authorization ヘッダーに制限され、body 置換はブロックされる。

---

## AI プロバイダ

ステートレスなチャット/補完 API。永続的な書き込み→読み取りエンドポイントなし。

### OpenAI

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **書き込み→読み取りエンドポイント**: チャットにはなし。Files/Assistants API は存在するがレスポンスはモデル生成であり、エコーではない。
- **結果**: 保護される。キーは Authorization ヘッダーに制限される。

### Anthropic

- **認証**: `X-Api-Key: <key>` → `--allow-only-header X-Api-Key`
- **書き込み→読み取りエンドポイント**: なし。ステートレスなメッセージ API。
- **結果**: 保護される。キーは X-Api-Key ヘッダーに制限される。

### Gemini

- **認証**: `?key=<key>` → `--allow-only-query key`
- **書き込み→読み取りエンドポイント**: なし。ステートレスなコンテンツ生成 API。
- **結果**: 保護される。キーは `key` クエリパラメータに制限される。

### Mistral

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **結果**: 保護される。OpenAI と同様。

### Groq

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **結果**: 保護される。OpenAI と同様。

### xAI (Grok)

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **結果**: 保護される。OpenAI と同様。

### DeepSeek

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **結果**: 保護される。OpenAI と同様。

### Perplexity

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **結果**: 保護される。OpenAI と同様。

### OpenRouter

- **認証**: `Authorization: Bearer <key>` → `--allow-only-header Authorization`
- **結果**: 保護される。OpenAI と同様。

---

## 検索サービス

読み取り専用 API。書き込みエンドポイントなし。

### Brave Search

- **認証**: `X-Subscription-Token: <key>` → `--allow-only-header X-Subscription-Token`
- **結果**: 保護される。読み取り専用 API、書き込みエンドポイントなし。

### Google Custom Search

- **認証**: `?key=<key>` → `--allow-only-query key`
- **結果**: 保護される。読み取り専用 API、書き込みエンドポイントなし。

### Bing Search

- **認証**: `Ocp-Apim-Subscription-Key: <key>` → `--allow-only-header Ocp-Apim-Subscription-Key`
- **結果**: 保護される。読み取り専用 API、書き込みエンドポイントなし。

### Exa

- **認証**: `X-Api-Key: <key>` → `--allow-only-header X-Api-Key`
- **結果**: 保護される。読み取り専用 API、書き込みエンドポイントなし。

### Tavily

- **認証**: `{"api_key": "<key>"}` → `--allow-only-field api_key`
- **結果**: 保護される。キーは `api_key` JSON フィールドに制限され、他のフィールドはブロックされる。
