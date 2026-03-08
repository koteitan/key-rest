[English](bing.md) | [日本語](bing-ja.md)

## Bing Search API (Azure)

### Setup
```bash
./key-rest add user1/bing/api-key https://api.bing.microsoft.com/
# → Enter the key value: (enter Azure Bing Search API Key)
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
