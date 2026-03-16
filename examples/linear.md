[← Back](README.md) | [English](linear.md) | [Japanese](linear-ja.md)

## Linear API

> **Note:** Linear is a GraphQL API.

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/linear/api-key https://api.linear.app/
# → Enter the key value: (enter Linear API Key)
```

> **Security:** Without `--allow-only-header`, an agent could embed `key-rest://user1/linear/api-key` in an issue description via GraphQL mutation, causing the key to be stored. The agent could then read it back via a query.

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const data = await fetch(
  'https://api.linear.app/graphql',
  {
    method: 'POST',
    headers: {
      'Authorization': 'key-rest://user1/linear/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      query: '{ issues { nodes { id title state { name } } } }'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := `{"query":"{ issues { nodes { id title state { name } } } }"}`
req, _ := keyrest.NewRequest("POST",
    "https://api.linear.app/graphql",
    strings.NewReader(body))
req.Header.Set("Authorization", "key-rest://user1/linear/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

data = requests.post(
    'https://api.linear.app/graphql',
    headers={
        'Authorization': 'key-rest://user1/linear/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'query': '{ issues { nodes { id title state { name } } } }'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.linear.app/graphql \
  -H "Authorization: key-rest://user1/linear/api-key" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ issues { nodes { id title state { name } } } }"}'
```
