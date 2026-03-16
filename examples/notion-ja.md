[← Back](README-ja.md) | [English](notion.md) | [Japanese](notion-ja.md)

## Notion API

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/notion/api-key https://api.notion.com/
# → キーの値を入力してください: (Notion Integration Token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントがページコンテンツに `key-rest://user1/notion/api-key` を埋め込み、トークンが Notion ページに保存される可能性があります。エージェントは pages API でそれを読み取れます。

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
