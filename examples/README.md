[English](README.md) | [Japanese](README-ja.md)

# REST API Usage Examples

## Authentication Patterns

| Pattern | Services | Injection Target | Termination Character |
|---------|----------|------------------|-----------------------|
| `Authorization: Bearer <key>` | OpenAI, Mistral, Groq, xAI, DeepSeek, Perplexity, OpenRouter, GitHub, Slack, LINE, Matrix, Notion, Sentry | Header value | End of string |
| `Authorization: <key>` | Linear | Header value | End of string |
| `Authorization: Bot <key>` | Discord | Header value | End of string |
| `Authorization: Basic <user>:<pass>` | Atlassian | Header value | `:` (Outside valid chars) |
| `?key=<key>` | Gemini, Google Custom Search, Trello | URL query parameter | `&` or End of string |
| `x-api-key: <key>` | Anthropic, Exa | Custom header | End of string |
| `X-Subscription-Token: <key>` | Brave Search | Custom header | End of string |
| `Ocp-Apim-Subscription-Key: <key>` | Bing Search | Custom header | End of string |
| `PRIVATE-TOKEN: <key>` | GitLab | Custom header | End of string |
| `{"api_key": "<key>"}` | Tavily | Request body | `"` (JSON string terminator) |
| `/bot<token>/<method>` | Telegram | URL path | `/` (Within valid chars → enclosed required) |

---

## AI Providers

- [OpenAI](openai.md)
- [Anthropic](anthropic.md)
- [Gemini](gemini.md)
- [OpenRouter](openrouter.md)
- [Mistral](mistral.md)
- [Groq](groq.md)
- [xAI (Grok)](xai.md)
- [DeepSeek](deepseek.md)

## Search

- [Brave Search](brave.md)
- [Perplexity](perplexity.md)
- [Google Custom Search](google-search.md)
- [Bing Search](bing.md)
- [Tavily](tavily.md)
- [Exa](exa.md)

## Community Channels

- [Slack](slack.md)
- [Discord](discord.md)
- [Telegram](telegram.md)
- [LINE](line.md)
- [Matrix](matrix.md)

## Developer Tools

- [GitHub](github.md)
- [Atlassian](atlassian.md)
- [GitLab](gitlab.md)
- [Notion](notion.md)
- [Trello](trello.md)
- [Linear](linear.md)
- [Sentry](sentry.md)
