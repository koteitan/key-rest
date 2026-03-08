[English](groq.md) | [Japanese](groq-ja.md)

## Groq API

> **Note:** Groq provides an OpenAI-compatible API.

### Setup
```bash
./key-rest add user1/groq/api-key https://api.groq.com/
# → Enter the key value: (enter Groq API key)
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
