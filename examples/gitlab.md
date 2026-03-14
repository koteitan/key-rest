[← Back](README.md) | [English](gitlab.md) | [Japanese](gitlab-ja.md)

## GitLab API

### Setup
```bash
./key-rest add user1/gitlab/token https://gitlab.com/
# → Enter the key value: (enter GitLab Personal Access Token)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const projects = await fetch(
  'https://gitlab.com/api/v4/projects?membership=true&order_by=updated_at',
  {
    headers: {
      'PRIVATE-TOKEN': 'key-rest://user1/gitlab/token'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://gitlab.com/api/v4/projects?membership=true&order_by=updated_at", nil)
req.Header.Set("PRIVATE-TOKEN", "key-rest://user1/gitlab/token")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

projects = requests.get(
    'https://gitlab.com/api/v4/projects',
    params={'membership': 'true', 'order_by': 'updated_at'},
    headers={
        'PRIVATE-TOKEN': 'key-rest://user1/gitlab/token'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl "https://gitlab.com/api/v4/projects?membership=true&order_by=updated_at" \
  -H "PRIVATE-TOKEN: key-rest://user1/gitlab/token"
```
