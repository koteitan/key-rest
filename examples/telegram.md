[← Back](README.md) | [English](telegram.md) | [Japanese](telegram-ja.md)

## Telegram Bot API

> **Note:** Telegram embeds the token in the URL path. Since `/sendMessage` follows the URI, the enclosed form `{{ }}` is required.

### Setup
```bash
./key-rest add user1/telegram/bot-token https://api.telegram.org/
# → Enter the key value: (enter Telegram Bot Token)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// Send a message
const result = await fetch(
  'https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      chat_id: 123456789,
      text: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"chat_id":123456789,"text":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("POST",
    "https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage", body)
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage',
    json={
        'chat_id': 123456789,
        'text': 'Hello from key-rest!'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl "https://api.telegram.org/bot{{key-rest://user1/telegram/bot-token}}/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{"chat_id":123456789,"text":"Hello from key-rest!"}'
```
