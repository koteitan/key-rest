[← Back](README.md) | [English](slack.md) | [Japanese](slack-ja.md)

## Slack API

### Setup
```bash
./key-rest add --allow-only-header Authorization user1/slack/bot-token https://slack.com/
# → Enter the key value: (enter Slack Bot Token)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// Send a message to a channel
const result = await fetch(
  'https://slack.com/api/chat.postMessage',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/slack/bot-token',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      channel: 'C01234567',
      text: 'Hello from key-rest!'
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"channel":"C01234567","text":"Hello from key-rest!"}`)
req, _ := keyrest.NewRequest("POST", "https://slack.com/api/chat.postMessage", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/slack/bot-token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

result = requests.post(
    'https://slack.com/api/chat.postMessage',
    headers={
        'Authorization': 'Bearer key-rest://user1/slack/bot-token',
        'Content-Type': 'application/json'
    },
    json={
        'channel': 'C01234567',
        'text': 'Hello from key-rest!'
    }
).json()
```

### curl
```bash
./clients/curl/key-rest-curl https://slack.com/api/chat.postMessage \
  -H "Authorization: Bearer key-rest://user1/slack/bot-token" \
  -H "Content-Type: application/json" \
  -d '{"channel":"C01234567","text":"Hello from key-rest!"}'
```
