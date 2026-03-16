[← Back](README.md) | [English](github.md) | [Japanese](github-ja.md)

## GitHub API

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/github/token https://api.github.com/
# → Enter the key value: (enter GitHub Personal Access Token)
```

> **Security:** Without `--allow-only-header`, an agent could embed `key-rest://user1/github/token` in the `body` field of an issue comment POST, causing the token to be posted publicly. The agent could then read it back via the issues API.

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
