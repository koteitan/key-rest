[← Back](README-ja.md) | [English](cloudflare.md) | [Japanese](cloudflare-ja.md)

## Cloudflare API

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/cloudflare/api-token https://api.cloudflare.com/
# → キーの値を入力してください: (Cloudflare API Token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントが DNS TXT レコードの値に `key-rest://user1/cloudflare/api-token` を埋め込み、トークンが保存される可能性があります。エージェントは zones API でそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// ゾーン一覧を取得
const zones = await fetch(
  'https://api.cloudflare.com/client/v4/zones',
  {
    headers: {
      'Authorization': 'Bearer key-rest://user1/cloudflare/api-token',
      'Content-Type': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.cloudflare.com/client/v4/zones", nil)
req.Header.Set("Authorization", "Bearer key-rest://user1/cloudflare/api-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

zones = requests.get(
    'https://api.cloudflare.com/client/v4/zones',
    headers={
        'Authorization': 'Bearer key-rest://user1/cloudflare/api-token',
        'Content-Type': 'application/json'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.cloudflare.com/client/v4/zones \
  -H "Authorization: Bearer key-rest://user1/cloudflare/api-token" \
  -H "Content-Type: application/json"
```
