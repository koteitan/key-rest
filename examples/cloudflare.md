[← Back](README.md) | [English](cloudflare.md) | [Japanese](cloudflare-ja.md)

## Cloudflare API

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/cloudflare/api-token https://api.cloudflare.com/
# → Enter the key value: (enter Cloudflare API Token)
```

> **Security:** Without `--allow-only-header`, an agent could embed `key-rest://user1/cloudflare/api-token` in a DNS TXT record value, causing the token to be stored. The agent could then read it back via the zones API.

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// List zones
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
