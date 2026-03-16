[← Back](README-ja.md) | [English](github.md) | [Japanese](github-ja.md)

## GitHub API

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/github/token https://api.github.com/
# → キーの値を入力してください: (GitHub Personal Access Token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントが issue コメント POST の `body` フィールドに `key-rest://user1/github/token` を埋め込み、トークンが公開される可能性があります。エージェントは issues API でそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const repos = await fetch(
  'https://api.github.com/user/repos?sort=updated',
  {
    headers: {
      'Authorization': 'Bearer key-rest://user1/github/token',
      'Accept': 'application/vnd.github+json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.github.com/user/repos?sort=updated", nil)
req.Header.Set("Authorization", "Bearer key-rest://user1/github/token")
req.Header.Set("Accept", "application/vnd.github+json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

repos = requests.get(
    'https://api.github.com/user/repos',
    params={'sort': 'updated'},
    headers={
        'Authorization': 'Bearer key-rest://user1/github/token',
        'Accept': 'application/vnd.github+json'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://api.github.com/user/repos?sort=updated \
  -H "Authorization: Bearer key-rest://user1/github/token" \
  -H "Accept: application/vnd.github+json"
```
