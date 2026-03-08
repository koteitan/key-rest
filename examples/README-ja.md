[English](README.md) | [日本語](README-ja.md)

# REST API の使用例

## 認証パターン一覧

| パターン | サービス例 | 注入先 | 終端文字 |
|----------|-----------|--------|----------|
| `Authorization: Bearer <key>` | OpenAI, Mistral, Groq, xAI, DeepSeek, Perplexity, OpenRouter, GitHub, Slack, LINE, Matrix, Notion, Sentry | ヘッダー値 | 文字列末尾 |
| `Authorization: <key>` | Linear | ヘッダー値 | 文字列末尾 |
| `Authorization: Bot <key>` | Discord | ヘッダー値 | 文字列末尾 |
| `Authorization: Basic <user>:<pass>` | Atlassian | ヘッダー値 | `:` (有効文字外) |
| `?key=<key>` | Gemini, Google Custom Search, Trello | URL クエリパラメータ | `&` or 文字列末尾 |
| `x-api-key: <key>` | Anthropic, Exa | カスタムヘッダー | 文字列末尾 |
| `X-Subscription-Token: <key>` | Brave Search | カスタムヘッダー | 文字列末尾 |
| `Ocp-Apim-Subscription-Key: <key>` | Bing Search | カスタムヘッダー | 文字列末尾 |
| `PRIVATE-TOKEN: <key>` | GitLab | カスタムヘッダー | 文字列末尾 |
| `{"api_key": "<key>"}` | Tavily | リクエストボディ | `"` (JSON文字列終端) |
| `/bot<token>/<method>` | Telegram | URL パス | `/` (有効文字内→enclosed 必要) |

---

## AI プロバイダ

- [OpenAI](openai-ja.md)
- [Anthropic](anthropic-ja.md)
- [Gemini](gemini-ja.md)
- [OpenRouter](openrouter-ja.md)
- [Mistral](mistral-ja.md)
- [Groq](groq-ja.md)
- [xAI (Grok)](xai-ja.md)
- [DeepSeek](deepseek-ja.md)

## 検索

- [Brave Search](brave-ja.md)
- [Perplexity](perplexity-ja.md)
- [Google Custom Search](google-search-ja.md)
- [Bing Search](bing-ja.md)
- [Tavily](tavily-ja.md)
- [Exa](exa-ja.md)

## コミュニティチャンネル

- [Slack](slack-ja.md)
- [Discord](discord-ja.md)
- [Telegram](telegram-ja.md)
- [LINE](line-ja.md)
- [Matrix](matrix-ja.md)

## 開発ツール

- [GitHub](github-ja.md)
- [Atlassian](atlassian-ja.md)
- [GitLab](gitlab-ja.md)
- [Notion](notion-ja.md)
- [Trello](trello-ja.md)
- [Linear](linear-ja.md)
- [Sentry](sentry-ja.md)
