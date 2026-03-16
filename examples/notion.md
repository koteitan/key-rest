[← Back](README.md) | [English](notion.md) | [Japanese](notion-ja.md)

## Notion API

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/notion/api-key https://api.notion.com/
# → Enter the key value: (enter Notion Integration Token)
```

> **Security:** Without `--allow-only-header`, an agent could embed `key-rest://user1/notion/api-key` in page content, causing the token to be stored in a Notion page. The agent could then read it back via the pages API.

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.notion.com/v1/databases/DATABASE_ID/query',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/notion/api-key',
      'Notion-Version': '2022-06-28',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      page_size: 10
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := `{"page_size":10}`
req, _ := keyrest.NewRequest("POST",
    "https://api.notion.com/v1/databases/DATABASE_ID/query",
    strings.NewReader(body))
req.Header.Set("Authorization", "Bearer key-rest://user1/notion/api-key")
req.Header.Set("Notion-Version", "2022-06-28")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.post(
    'https://api.notion.com/v1/databases/DATABASE_ID/query',
    headers={
        'Authorization': 'Bearer key-rest://user1/notion/api-key',
        'Notion-Version': '2022-06-28',
        'Content-Type': 'application/json'
    },
    json={
        'page_size': 10
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.notion.com/v1/databases/DATABASE_ID/query \
  -H "Authorization: Bearer key-rest://user1/notion/api-key" \
  -H "Notion-Version: 2022-06-28" \
  -H "Content-Type: application/json" \
  -d '{"page_size":10}'
```
