[← Back](README.md) | [English](trello.md) | [Japanese](trello-ja.md)

## Trello API

> **Note:** Trello sends both an API key and a token as URL query parameters.

### Setup
```bash
./key-rest add --allow-only-query key user1/trello/api-key https://api.trello.com/
# → Enter the key value: (enter Trello API Key)
./key-rest add --allow-only-query token user1/trello/token https://api.trello.com/
# → Enter the key value: (enter Trello Token)
```

> **Security:** Without `--allow-only-query`, an agent could embed `key-rest://user1/trello/api-key` in a card description, causing the key to be stored in the card. The agent could then read it back via the cards API.

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const boards = await fetch(
  'https://api.trello.com/1/members/me/boards?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token',
  {
    headers: {
      'Accept': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.trello.com/1/members/me/boards?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token", nil)
req.Header.Set("Accept", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

boards = requests.get(
    'https://api.trello.com/1/members/me/boards',
    params={
        'key': 'key-rest://user1/trello/api-key',
        'token': 'key-rest://user1/trello/token'
    },
    headers={
        'Accept': 'application/json'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl "https://api.trello.com/1/members/me/boards?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token"
```
