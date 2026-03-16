[← Back](README-ja.md) | [English](sentry.md) | [Japanese](sentry-ja.md)

## Sentry API

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/sentry/auth-token https://sentry.io/
# → キーの値を入力してください: (Sentry Auth Token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントが issue コメントに `key-rest://user1/sentry/auth-token` を埋め込み、トークンが保存される可能性があります。エージェントはそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const projects = await fetch(
  'https://sentry.io/api/0/projects/',
  {
    headers: {
      'Authorization': 'Bearer key-rest://user1/sentry/auth-token'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://sentry.io/api/0/projects/", nil)
req.Header.Set("Authorization", "Bearer key-rest://user1/sentry/auth-token")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

projects = requests.get(
    'https://sentry.io/api/0/projects/',
    headers={
        'Authorization': 'Bearer key-rest://user1/sentry/auth-token'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://sentry.io/api/0/projects/ \
  -H "Authorization: Bearer key-rest://user1/sentry/auth-token"
```
