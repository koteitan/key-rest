[← Back](README.md) | [English](atlassian.md) | [Japanese](atlassian-ja.md)

## Atlassian API

> **Note:** The `base64(...)` transform function causes key-rest-daemon to concatenate arguments and base64-encode them after URI substitution.

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/atlassian/email https://api.bitbucket.org/
# → Enter the key value: (enter Atlassian email)
./key-rest add --allow-only-header Authorization user1/atlassian/token https://api.bitbucket.org/
# → Enter the key value: (enter Atlassian API token)
```

> **Security:** Without `--allow-only-header`, an agent could embed credentials in a PR comment body, causing them to be posted. The agent could then read them back via the comments API.

> **Warning (Data Center/Server only):** Atlassian Data Center PATs inherit the user's full permissions with no scope restriction. An agent can create a new PAT via `POST /rest/pat/latest/tokens` and the new token will not be in key-rest's credential store, bypassing response masking. This cannot be prevented by key-rest configuration.

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
