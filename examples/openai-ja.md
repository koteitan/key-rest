[← Back](README-ja.md) | [English](openai.md) | [Japanese](openai-ja.md)

## OpenAI API

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/openai/api-key https://api.openai.com/
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

### curl
```bash
./clients/curl/key-rest-curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer key-rest://user1/openai/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hello!"}]}'
```
