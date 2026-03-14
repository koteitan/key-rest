[English](openrouter.md) | [Japanese](openrouter-ja.md)

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

### curl
```bash
./clients/curl/key-rest-curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer key-rest://user1/openrouter/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"anthropic/claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello!"}]}'
```
