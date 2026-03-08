[English](perplexity.md) | [Japanese](perplexity-ja.md)

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
