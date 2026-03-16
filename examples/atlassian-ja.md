[← Back](README-ja.md) | [English](atlassian.md) | [Japanese](atlassian-ja.md)

## Atlassian API

> **Note:** `base64(...)` 変換関数により、key-rest-daemon が URI 置換後に引数を連結して base64 エンコードします。

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/atlassian/email https://api.bitbucket.org/
# → キーの値を入力してください: (Atlassian email を入力)
./key-rest add --allow-only-header Authorization user1/atlassian/token https://api.bitbucket.org/
# → キーの値を入力してください: (Atlassian API token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントが PR コメントの本文にクレデンシャルを埋め込み、投稿される可能性があります。エージェントは comments API でそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const prs = await fetch(
  'https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN',
  {
    headers: {
      'Authorization': 'Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}',
      'Content-Type': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN", nil)
req.Header.Set("Authorization",
    "Basic {{ base64(key-rest://user1/atlassian/email, \":\", key-rest://user1/atlassian/token) }}")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

prs = requests.get(
    'https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests',
    params={'state': 'OPEN'},
    headers={
        'Authorization': 'Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}',
        'Content-Type': 'application/json'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl "https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN" \
  -H 'Authorization: Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}'
```
