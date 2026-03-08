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
