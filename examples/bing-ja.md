[← Back](README-ja.md) | [English](bing.md) | [Japanese](bing-ja.md)

## Bing Search API (Azure)

### セットアップ
```bash
./key-rest add --allow-only-header Ocp-Apim-Subscription-Key user1/bing/api-key https://api.bing.microsoft.com/
# → キーの値を入力してください: (Azure Bing Search API Key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.bing.microsoft.com/v7.0/search?q=hello',
  {
    headers: {
      'Ocp-Apim-Subscription-Key': 'key-rest://user1/bing/api-key'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.bing.microsoft.com/v7.0/search?q=hello", nil)
req.Header.Set("Ocp-Apim-Subscription-Key", "key-rest://user1/bing/api-key")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.get(
    'https://api.bing.microsoft.com/v7.0/search',
    params={'q': 'hello'},
    headers={
        'Ocp-Apim-Subscription-Key': 'key-rest://user1/bing/api-key'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.bing.microsoft.com/v7.0/search?q=hello \
  -H "Ocp-Apim-Subscription-Key: key-rest://user1/bing/api-key"
```
