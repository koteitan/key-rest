[← Back](README.md) | [English](discord.md) | [Japanese](discord-ja.md)

## Discord API

> **Note:** Discord uses the `Bot` prefix instead of `Bearer`.

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/discord/bot-token https://discord.com/
# → Enter the key value: (enter Discord Bot Token)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// Send a message to a channel
const result = await fetch(
  'https://discord.com/api/v10/channels/123456789/messages',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bot key-rest://user1/discord/bot-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      content: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"content":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("POST",
    "https://discord.com/api/v10/channels/123456789/messages", body)
req.Header.Set("Authorization", "Bot key-rest://user1/discord/bot-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://discord.com/api/v10/channels/123456789/messages',
    headers={
        'Authorization': 'Bot key-rest://user1/discord/bot-token',
        'Content-Type': 'application/json'
    },
    json={
        'content': 'Hello from key-rest!'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://discord.com/api/v10/channels/123456789/messages \
  -H "Authorization: Bot key-rest://user1/discord/bot-token" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hello from key-rest!"}'
```
