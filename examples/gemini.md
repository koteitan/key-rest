[English](gemini.md) | [Japanese](gemini-ja.md)

## Gemini API

> **Note:** Gemini passes the API key via URL query parameter `?key=`.

### Setup
```bash
./key-rest add user1/gemini/api-key https://generativelanguage.googleapis.com/
# → Enter the key value: (enter Gemini API key)
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

### curl
```bash
./clients/curl/key-rest-curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"Hello, world!"}]}]}'
```
