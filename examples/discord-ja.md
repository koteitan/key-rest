[← Back](README-ja.md) | [English](discord.md) | [Japanese](discord-ja.md)

## Discord API

> **Note:** Discord は `Bearer` ではなく `Bot` プレフィックスを使用します。

### セットアップ
```bash
./key-rest add --allow-only-header Authorization user1/discord/bot-token https://discord.com/
# → キーの値を入力してください: (Discord Bot Token を入力)
```

> **セキュリティ:** `--allow-only-header` を付けない場合、エージェントがメッセージ POST の `content` フィールドに `key-rest://user1/discord/bot-token` を埋め込み、トークンがチャンネルメッセージとして投稿される可能性があります。エージェントは `GET /channels/{id}/messages` でそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// チャンネルにメッセージを送信
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
