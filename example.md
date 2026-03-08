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

# AI プロバイダ

## OpenAI API

### セットアップ
```bash
./key-rest add user1/openai/api-key https://api.openai.com/
# → キーの値を入力してください: (OpenAI API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.openai.com/v1/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/openai/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'gpt-4o',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/openai/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.openai.com/v1/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/openai/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'gpt-4o',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

## Anthropic API

> **Note:** Anthropic は `Authorization: Bearer` ではなく `x-api-key` カスタムヘッダーを使用します。

### セットアップ
```bash
./key-rest add user1/anthropic/api-key https://api.anthropic.com/
# → キーの値を入力してください: (Anthropic API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.anthropic.com/v1/messages',
  {
    method: 'POST',
    headers: {
      'x-api-key': 'key-rest://user1/anthropic/api-key',
      'anthropic-version': '2023-06-01',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'claude-sonnet-4-20250514',
      max_tokens: 1024,
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"claude-sonnet-4-20250514","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.anthropic.com/v1/messages", body)
req.Header.Set("x-api-key", "key-rest://user1/anthropic/api-key")
req.Header.Set("anthropic-version", "2023-06-01")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.anthropic.com/v1/messages',
    headers={
        'x-api-key': 'key-rest://user1/anthropic/api-key',
        'anthropic-version': '2023-06-01',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'claude-sonnet-4-20250514',
        'max_tokens': 1024,
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

## Gemini API

> **Note:** Gemini は URL クエリパラメータ `?key=` で API キーを渡します。

### セットアップ
```bash
./key-rest add user1/gemini/api-key https://generativelanguage.googleapis.com/
# → キーの値を入力してください: (Gemini API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      contents: [{ parts: [{ text: 'Hello, world!' }] }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"contents":[{"parts":[{"text":"Hello, world!"}]}]}`)
req, _ := keyrest.NewRequest("POST",
    "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key", body)
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent',
    params={'key': 'key-rest://user1/gemini/api-key'},
    json={
        'contents': [{'parts': [{'text': 'Hello, world!'}]}]
    }
).json()
```

## OpenRouter API

> **Note:** OpenRouter は複数の AI モデルを統一 API で提供するアグリゲータです。認証は Bearer トークンです。

### セットアップ
```bash
./key-rest add user1/openrouter/api-key https://openrouter.ai/
# → キーの値を入力してください: (OpenRouter API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://openrouter.ai/api/v1/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/openrouter/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'anthropic/claude-sonnet-4-20250514',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"anthropic/claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/openrouter/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://openrouter.ai/api/v1/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/openrouter/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'anthropic/claude-sonnet-4-20250514',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

## Mistral API

### セットアップ
```bash
./key-rest add user1/mistral/api-key https://api.mistral.ai/
# → キーの値を入力してください: (Mistral API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.mistral.ai/v1/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/mistral/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'mistral-large-latest',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"mistral-large-latest","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.mistral.ai/v1/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/mistral/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.mistral.ai/v1/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/mistral/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'mistral-large-latest',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

## Groq API

> **Note:** Groq は OpenAI 互換 API を提供しています。

### セットアップ
```bash
./key-rest add user1/groq/api-key https://api.groq.com/
# → キーの値を入力してください: (Groq API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.groq.com/openai/v1/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/groq/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'llama-3.3-70b-versatile',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"llama-3.3-70b-versatile","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/groq/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.groq.com/openai/v1/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/groq/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'llama-3.3-70b-versatile',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

## xAI (Grok) API

### セットアップ
```bash
./key-rest add user1/xai/api-key https://api.x.ai/
# → キーの値を入力してください: (xAI API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.x.ai/v1/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/xai/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'grok-3',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"grok-3","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.x.ai/v1/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/xai/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.x.ai/v1/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/xai/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'grok-3',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

## DeepSeek API

> **Note:** DeepSeek は OpenAI 互換 API を提供しています。

### セットアップ
```bash
./key-rest add user1/deepseek/api-key https://api.deepseek.com/
# → キーの値を入力してください: (DeepSeek API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.deepseek.com/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/deepseek/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'deepseek-chat',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"deepseek-chat","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.deepseek.com/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/deepseek/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.deepseek.com/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/deepseek/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'deepseek-chat',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```

---

# 検索

## Brave Search API

### セットアップ
```bash
./key-rest add user1/brave/api-key https://api.search.brave.com/
# → キーの値を入力してください: (Brave API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.search.brave.com/res/v1/web/search?q=hello+world',
  {
    headers: {
      'X-Subscription-Token': 'key-rest://user1/brave/api-key',
      'Accept': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.search.brave.com/res/v1/web/search?q=hello+world", nil)
req.Header.Set("X-Subscription-Token", "key-rest://user1/brave/api-key")
req.Header.Set("Accept", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.get(
    'https://api.search.brave.com/res/v1/web/search',
    params={'q': 'hello world'},
    headers={
        'X-Subscription-Token': 'key-rest://user1/brave/api-key',
        'Accept': 'application/json'
    }
).json()
```

## Perplexity API

### セットアップ
```bash
./key-rest add user1/perplexity/api-key https://api.perplexity.ai/
# → キーの値を入力してください: (Perplexity API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.perplexity.ai/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/perplexity/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'sonar',
      messages: [{ role: 'user', content: 'What is key-rest?' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"sonar","messages":[{"role":"user","content":"What is key-rest?"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.perplexity.ai/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/perplexity/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.perplexity.ai/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/perplexity/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'sonar',
        'messages': [{'role': 'user', 'content': 'What is key-rest?'}]
    }
).json()
```

---

# コミュニティチャンネル

## Slack API

### セットアップ
```bash
./key-rest add user1/slack/bot-token https://slack.com/
# → キーの値を入力してください: (Slack Bot Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// チャンネルにメッセージを送信
const result = await fetch(
  'https://slack.com/api/chat.postMessage',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/slack/bot-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      channel: 'C01234567',
      text: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"channel":"C01234567","text":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("POST", "https://slack.com/api/chat.postMessage", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/slack/bot-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://slack.com/api/chat.postMessage',
    headers={
        'Authorization': 'Bearer key-rest://user1/slack/bot-token',
        'Content-Type': 'application/json'
    },
    json={
        'channel': 'C01234567',
        'text': 'Hello from key-rest!'
    }
).json()
```

## Discord API

> **Note:** Discord は `Bearer` ではなく `Bot` プレフィックスを使用します。

### セットアップ
```bash
./key-rest add user1/discord/bot-token https://discord.com/
# → キーの値を入力してください: (Discord Bot Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// チャンネルにメッセージを送信
const result = await fetch(
  'https://discord.com/api/v10/channels/123456789/messages',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bot key-rest://user1/discord/bot-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      content: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"content":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("POST",
    "https://discord.com/api/v10/channels/123456789/messages", body)
req.Header.Set("Authorization", "Bot key-rest://user1/discord/bot-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://discord.com/api/v10/channels/123456789/messages',
    headers={
        'Authorization': 'Bot key-rest://user1/discord/bot-token',
        'Content-Type': 'application/json'
    },
    json={
        'content': 'Hello from key-rest!'
    }
).json()
```

## Telegram Bot API

> **Note:** Telegram はトークンを URL パスに埋め込みます。URI の後に `/sendMessage` が続くため enclosed 形式 `{{ }}` が必要です。

### セットアップ
```bash
./key-rest add user1/telegram/bot-token https://api.telegram.org/
# → キーの値を入力してください: (Telegram Bot Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// メッセージを送信
const result = await fetch(
  'https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      chat_id: 123456789,
      text: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"chat_id":123456789,"text":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("POST",
    "https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage", body)
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage',
    json={
        'chat_id': 123456789,
        'text': 'Hello from key-rest!'
    }
).json()
```

## LINE Messaging API

### セットアップ
```bash
./key-rest add user1/line/channel-access-token https://api.line.me/
# → キーの値を入力してください: (LINE Channel Access Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const result = await fetch(
  'https://api.line.me/v2/bot/message/push',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/line/channel-access-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      to: 'U1234567890abcdef',
      messages: [{ type: 'text', text: 'Hello from key-rest!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"to":"U1234567890abcdef","messages":[{"type":"text","text":"Hello from key-rest!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.line.me/v2/bot/message/push", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/line/channel-access-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://api.line.me/v2/bot/message/push',
    headers={
        'Authorization': 'Bearer key-rest://user1/line/channel-access-token',
        'Content-Type': 'application/json'
    },
    json={
        'to': 'U1234567890abcdef',
        'messages': [{'type': 'text', 'text': 'Hello from key-rest!'}]
    }
).json()
```

## Matrix API

> **Note:** Matrix はホームサーバーの URL がインスタンスごとに異なります。

### セットアップ
```bash
./key-rest add user1/matrix/access-token https://matrix.example.org/
# → キーの値を入力してください: (Matrix Access Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const result = await fetch(
  'https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message',
  {
    method: 'PUT',
    headers: {
      'Authorization': 'Bearer key-rest://user1/matrix/access-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      msgtype: 'm.text',
      body: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"msgtype":"m.text","body":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("PUT",
    "https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/matrix/access-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.put(
    'https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message',
    headers={
        'Authorization': 'Bearer key-rest://user1/matrix/access-token',
        'Content-Type': 'application/json'
    },
    json={
        'msgtype': 'm.text',
        'body': 'Hello from key-rest!'
    }
).json()
```

---

# 開発ツール

## GitHub API

### セットアップ
```bash
./key-rest add user1/github/token https://api.github.com/
# → キーの値を入力してください: (GitHub Personal Access Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const repos = await fetch(
  'https://api.github.com/user/repos?sort=updated',
  {
    headers: {
      'Authorization': 'Bearer key-rest://user1/github/token',
      'Accept': 'application/vnd.github+json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.github.com/user/repos?sort=updated", nil)
req.Header.Set("Authorization", "Bearer key-rest://user1/github/token")
req.Header.Set("Accept", "application/vnd.github+json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

repos = requests.get(
    'https://api.github.com/user/repos',
    params={'sort': 'updated'},
    headers={
        'Authorization': 'Bearer key-rest://user1/github/token',
        'Accept': 'application/vnd.github+json'
    }
).json()
```

## Atlassian API

> **Note:** `base64(...)` 変換関数により、key-rest-daemon が URI 置換後に引数を連結して base64 エンコードします。

### セットアップ
```bash
./key-rest add user1/atlassian/email https://api.bitbucket.org/
# → キーの値を入力してください: (Atlassian email を入力)
./key-rest add user1/atlassian/token https://api.bitbucket.org/
# → キーの値を入力してください: (Atlassian API token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const prs = await fetch(
  'https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN',
  {
    headers: {
      'Authorization': 'Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}',
      'Content-Type': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN", nil)
req.Header.Set("Authorization",
    "Basic {{ base64(key-rest://user1/atlassian/email, \":\", key-rest://user1/atlassian/token) }}")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

prs = requests.get(
    'https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests',
    params={'state': 'OPEN'},
    headers={
        'Authorization': 'Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}',
        'Content-Type': 'application/json'
    }
).json()
```
