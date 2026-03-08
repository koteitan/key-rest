[English](matrix.md) | [日本語](matrix-ja.md)

## Matrix API

> **Note:** Matrix homeserver URLs vary per instance.

### Setup
```bash
./key-rest add user1/matrix/access-token https://matrix.example.org/
# → Enter the key value: (enter Matrix Access Token)
```

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
