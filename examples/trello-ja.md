[English](trello.md) | [Japanese](trello-ja.md)

## Trello API

> **Note:** Trello は API キーとトークンの2つを URL クエリパラメータとして送信します。

### セットアップ
```bash
./key-rest add user1/trello/api-key https://api.trello.com/
# → キーの値を入力してください: (Trello API Key を入力)
./key-rest add user1/trello/token https://api.trello.com/
# → キーの値を入力してください: (Trello Token を入力)
```

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
