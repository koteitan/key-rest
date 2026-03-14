[English](mistral.md) | [Japanese](mistral-ja.md)

## Mistral API

### Setup
```bash
./key-rest add user1/mistral/api-key https://api.mistral.ai/
# → Enter the key value: (enter Mistral API key)
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

### curl
```bash
./clients/curl/key-rest-curl https://api.mistral.ai/v1/chat/completions \
  -H "Authorization: Bearer key-rest://user1/mistral/api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"mistral-large-latest","messages":[{"role":"user","content":"Hello!"}]}'
```
