[← Back](README-ja.md) | [English](matrix.md) | [Japanese](matrix-ja.md)

## Matrix API

> **Note:** Matrix はホームサーバーの URL がインスタンスごとに異なります。

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/matrix/access-token https://matrix.example.org/
# → キーの値を入力してください: (Matrix Access Token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントがルームメッセージの `body` フィールドに `key-rest://user1/matrix/access-token` を埋め込み、トークンがルームに投稿される可能性があります。エージェントは sync または messages API でそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const result = await fetch(
  'https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message',
  {
    method: 'PUT',
    headers: {
      'Authorization': 'Bearer key-rest://user1/matrix/access-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      msgtype: 'm.text',
      body: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"msgtype":"m.text","body":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("PUT",
    "https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/matrix/access-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.put(
    'https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message',
    headers={
        'Authorization': 'Bearer key-rest://user1/matrix/access-token',
        'Content-Type': 'application/json'
    },
    json={
        'msgtype': 'm.text',
        'body': 'Hello from key-rest!'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://matrix.example.org/_matrix/client/v3/rooms/!roomid:example.org/send/m.room.message \
  -X PUT \
  -H "Authorization: Bearer key-rest://user1/matrix/access-token" \
  -H "Content-Type: application/json" \
  -d '{"msgtype":"m.text","body":"Hello from key-rest!"}'
```
