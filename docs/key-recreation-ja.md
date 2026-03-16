[← Back](README-ja.md) | [English](key-recreation.md) | [Japanese](key-recreation-ja.md)

# キーの再生成

## キーの再生成とは

key-rest は LLM エージェントがクレデンシャルの値を**見る**ことを防ぎますが、クレデンシャルを使った API 呼び出しにより同種の新しいクレデンシャルを生成できるサービスでは、それを防ぐことはできません。エージェントは key-rest 経由でクレデンシャル作成エンドポイントを呼び出し、レスポンスから新しいクレデンシャルを取得できます。

このように、クレデンシャルが同種のクレデンシャルを作成できる性質を**キーの再生成**と呼ぶことにします。

ここではキーの再生成ができるかどうかを各サービスのクレデンシャルごとに調査します。

## 調査結果

### 再生成を防止できないもの

| サービス | クレデンシャル | 詳細 | 調査日 |
|---|---|---|---|
| Atlassian | PAT（Data Center/Server） | `POST /rest/pat/latest/tokens` で新しい PAT を作成できる。PAT はユーザーの全権限を引き継ぎスコープ制限がない。 | 2026-03-16 |

### 設定次第で再生成を防止できるもの

| サービス | クレデンシャル | 詳細 | 調査日 |
|---|---|---|---|
| AWS | IAM アクセスキー | `CreateAccessKey` API で新しいキーを作成できる。IAM ポリシーから `iam:CreateAccessKey` 権限を除外することで防止可能。 | 2026-03-16 |
| GCP | サービスアカウントキー | `projects.serviceAccounts.keys.create` API で新しいキーを作成できる。IAM ロールから `iam.serviceAccountKeys.create` 権限を除外することで防止可能。 | 2026-03-16 |
| IBM Cloud | IAM API キー | `POST /v1/apikeys` で新しいキーを作成できる。アカウント設定「Restrict API key creation」を有効にすることで防止可能。 | 2026-03-16 |
| Cloudflare | API トークン | トークン作成 API が存在する。`API Tokens Write` 権限を付与しないことで防止可能。 | 2026-03-16 |
| OpenAI | API キー | Admin API Key で `POST /v1/organization/admin_api_keys` により作成可能。Admin Key を付与しないことで防止可能。通常の Project API Key には作成エンドポイントがない。 | 2026-03-16 |

### 再生成ができないもの

| サービス | クレデンシャル | 詳細 | 調査日 |
|---|---|---|---|
| GitHub | PAT（classic/fine-grained） | PAT を作成する API エンドポイントが存在しない。`POST /authorizations` は 2020 年に廃止。 | 2026-03-16 |
| GitHub | OAuth トークン | OAuth フロー（ブラウザ認可 + `client_id` + `client_secret`）が必要。いずれもトークン自体からは取得できない。 | 2026-03-16 |
| Atlassian | API トークン（Cloud） | API エンドポイントが存在しない。Web UI でのみ作成可能。 | 2026-03-16 |
| Azure | サービスキー（例: Azure OpenAI） | サービスキーではキーを管理する ARM API を認証できない。別途 Azure AD トークンが必要。 | 2026-03-16 |
| Oracle Cloud | API 署名キー | `UploadApiKey` API で公開鍵をアップロードできるが、秘密鍵はクライアント側で生成され API レスポンスに含まれない。 | 2026-03-16 |
| Anthropic | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| GitLab | PAT | `POST /api/v4/user/personal_access_tokens` が存在するが、作成されるトークンは `k8s_proxy` と `self_rotate` スコープに制限される。 | 2026-03-16 |
| Mistral | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Groq | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| xAI | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Perplexity | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| DeepSeek | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| OpenRouter | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Google（Gemini） | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Google Search | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Exa | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Brave | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Bing | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Tavily | API キー | API キーを作成するエンドポイントが存在しない。 | 2026-03-16 |
| Slack | Bot トークン | OAuth フロー（ブラウザでのユーザー認可）が必要。 | 2026-03-16 |
| Discord | Bot トークン | Developer Portal の Web UI でのみ作成可能。 | 2026-03-16 |
| Telegram | Bot トークン | BotFather との対話でのみ作成可能。 | 2026-03-16 |
| LINE | チャネルアクセストークン | `client_id` + `client_secret` による OAuth フローが必要。 | 2026-03-16 |
| Matrix | アクセストークン | ユーザー名 + パスワードによる認証が必要。 | 2026-03-16 |
| Linear | API キー | Web UI でのみ作成可能。 | 2026-03-16 |
| Notion | Integration トークン | Internal トークンは Web UI で作成。Public integration は `client_id` + `client_secret` による OAuth フローが必要。 | 2026-03-16 |
| Sentry | Auth トークン | Web UI でのみ作成可能。 | 2026-03-16 |
| Trello | API キー | Web UI でのみ作成可能。 | 2026-03-16 |
