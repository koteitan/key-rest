[← Back](README.md) | [English](brave.md) | [Japanese](brave-ja.md)

## Brave Search API

### Setup
```bash
./key-rest add user1/brave/api-key https://api.search.brave.com/
# → Enter the key value: (enter Brave API key)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.search.brave.com/res/v1/web/search?q=hello+world',
  {
    headers: {
      'X-Subscription-Token': 'key-rest://user1/brave/api-key',
      'Accept': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.search.brave.com/res/v1/web/search?q=hello+world", nil)
req.Header.Set("X-Subscription-Token", "key-rest://user1/brave/api-key")
req.Header.Set("Accept", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.get(
    'https://api.search.brave.com/res/v1/web/search',
    params={'q': 'hello world'},
    headers={
        'X-Subscription-Token': 'key-rest://user1/brave/api-key',
        'Accept': 'application/json'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.search.brave.com/res/v1/web/search?q=hello+world \
  -H "X-Subscription-Token: key-rest://user1/brave/api-key"
```
