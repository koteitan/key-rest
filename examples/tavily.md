## Tavily Search API

> **Note:** Tavily は API キーをリクエストボディの JSON フィールドとして送信します。key-rest:// URI はボディ内でも置換されます。

### セットアップ
```bash
./key-rest add user1/tavily/api-key https://api.tavily.com/
# → キーの値を入力してください: (Tavily API Key を入力)
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
