[English](sentry.md) | [日本語](sentry-ja.md)

## Sentry API

### セットアップ
```bash
./key-rest add user1/sentry/auth-token https://sentry.io/
# → キーの値を入力してください: (Sentry Auth Token を入力)
```

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
