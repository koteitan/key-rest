[English](line.md) | [日本語](line-ja.md)

## LINE Messaging API

### Setup
```bash
./key-rest add user1/line/channel-access-token https://api.line.me/
# → Enter the key value: (enter LINE Channel Access Token)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const result = await fetch(
  'https://api.line.me/v2/bot/message/push',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/line/channel-access-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      to: 'U1234567890abcdef',
      messages: [{ type: 'text', text: 'Hello from key-rest!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"to":"U1234567890abcdef","messages":[{"type":"text","text":"Hello from key-rest!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.line.me/v2/bot/message/push", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/line/channel-access-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://api.line.me/v2/bot/message/push',
    headers={
        'Authorization': 'Bearer key-rest://user1/line/channel-access-token',
        'Content-Type': 'application/json'
    },
    json={
        'to': 'U1234567890abcdef',
        'messages': [{'type': 'text', 'text': 'Hello from key-rest!'}]
    }
).json()
```
