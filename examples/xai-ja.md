[English](xai.md) | [Japanese](xai-ja.md)

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
