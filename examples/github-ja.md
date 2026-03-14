[English](github.md) | [Japanese](github-ja.md)

## GitHub API

### セットアップ
```bash
./key-rest add user1/github/token https://api.github.com/
# → キーの値を入力してください: (GitHub Personal Access Token を入力)
```

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
