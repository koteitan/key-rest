[← Back](README.md) | [English](exa.md) | [Japanese](exa-ja.md)

## Exa Search API

### Setup
```bash
./key-rest add --allow-only-header X-Api-Key user1/exa/api-key https://api.exa.ai/
# → Enter the key value: (enter Exa API Key)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.exa.ai/search',
  {
    method: 'POST',
    headers: {
      'x-api-key': 'key-rest://user1/exa/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      query: 'hello',
      num_results: 10
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := `{"query":"hello","num_results":10}`
req, _ := keyrest.NewRequest("POST",
    "https://api.exa.ai/search",
    strings.NewReader(body))
req.Header.Set("x-api-key", "key-rest://user1/exa/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.post(
    'https://api.exa.ai/search',
    headers={
        'x-api-key': 'key-rest://user1/exa/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'query': 'hello',
        'num_results': 10
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.exa.ai/search \
  -H "x-api-key: key-rest://user1/exa/api-key" \
  -H "Content-Type: application/json" \
  -d '{"query":"hello","num_results":10}'
```
