# REST API の使用例

## 認証パターン一覧

| パターン | サービス例 | 注入先 | 終端文字 |
|----------|-----------|--------|----------|
| `Authorization: Bearer <key>` | OpenAI, Mistral, Groq, xAI, DeepSeek, Perplexity, OpenRouter, GitHub, Slack, LINE, Matrix | ヘッダー値 | 文字列末尾 |
| `Authorization: Bot <key>` | Discord | ヘッダー値 | 文字列末尾 |
| `Authorization: Basic <user>:<pass>` | Atlassian | ヘッダー値 | `:` (有効文字外) |
| `?key=<key>` | Gemini | URL クエリパラメータ | `&` or 文字列末尾 |
| `x-api-key: <key>` | Anthropic | カスタムヘッダー | 文字列末尾 |
| `X-Subscription-Token: <key>` | Brave Search | カスタムヘッダー | 文字列末尾 |
| `/bot<token>/<method>` | Telegram | URL パス | `/` (有効文字内→enclosed 必要) |

---

## AI プロバイダ

- [OpenAI](openai.md)
- [Anthropic](anthropic.md)
- [Gemini](gemini.md)
- [OpenRouter](openrouter.md)
- [Mistral](mistral.md)
- [Groq](groq.md)
- [xAI (Grok)](xai.md)
- [DeepSeek](deepseek.md)

## 検索

- [Brave Search](brave.md)
- [Perplexity](perplexity.md)

## コミュニティチャンネル

- [Slack](slack.md)
- [Discord](discord.md)
- [Telegram](telegram.md)
- [LINE](line.md)
- [Matrix](matrix.md)

## 開発ツール

- [GitHub](github.md)
- [Atlassian](atlassian.md)
