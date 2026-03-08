[English](google-search.md) | [日本語](google-search-ja.md)

## Google Custom Search API

### Setup
```bash
./key-rest add user1/google/api-key https://www.googleapis.com/
# → Enter the key value: (enter Google API Key)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://www.googleapis.com/customsearch/v1?q=hello&cx=YOUR_SEARCH_ENGINE_ID&key=key-rest://user1/google/api-key',
  {
    headers: {
      'Accept': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://www.googleapis.com/customsearch/v1?q=hello&cx=YOUR_SEARCH_ENGINE_ID&key=key-rest://user1/google/api-key", nil)
req.Header.Set("Accept", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.get(
    'https://www.googleapis.com/customsearch/v1',
    params={
        'q': 'hello',
        'cx': 'YOUR_SEARCH_ENGINE_ID',
        'key': 'key-rest://user1/google/api-key'
    },
    headers={
        'Accept': 'application/json'
    }
).json()
```
