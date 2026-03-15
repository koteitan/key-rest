[← Back](README.md) | [English](tavily.md) | [Japanese](tavily-ja.md)

## Tavily Search API

> **Note:** Tavily sends the API key as a JSON field in the request body. The key-rest:// URI is also replaced within the body.

### Setup
```bash
./key-rest add --allow-only-field api_key user1/tavily/api-key https://api.tavily.com/
# → Enter the key value: (enter Tavily API Key)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.tavily.com/search',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      api_key: 'key-rest://user1/tavily/api-key',
      query: 'hello',
      search_depth: 'basic'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := `{"api_key":"key-rest://user1/tavily/api-key","query":"hello","search_depth":"basic"}`
req, _ := keyrest.NewRequest("POST",
    "https://api.tavily.com/search",
    strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.post(
    'https://api.tavily.com/search',
    json={
        'api_key': 'key-rest://user1/tavily/api-key',
        'query': 'hello',
        'search_depth': 'basic'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.tavily.com/search \
  -H "Content-Type: application/json" \
  -d '{"api_key":"key-rest://user1/tavily/api-key","query":"hello","search_depth":"basic"}'
```
