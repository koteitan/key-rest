[← Back](README-ja.md) | [English](telegram.md) | [Japanese](telegram-ja.md)

## Telegram Bot API

> **Note:** Telegram はトークンを URL パスに埋め込みます。URI の後に `/sendMessage` が続くため enclosed 形式 `{{ }}` が必要です。

### セットアップ
```bash
./key-rest add --allow-only-url user1/telegram/bot-token https://api.telegram.org/
# → キーの値を入力してください: (Telegram Bot Token を入力)
```

> **セキュリティ:** `--allow-only-url` を付けない場合、エージェントが `sendMessage` の `text` フィールドに `key-rest://user1/telegram/bot-token` を埋め込み、トークンがチャットメッセージとして送信される可能性があります。エージェントは `getUpdates` でそれを読み取れます。

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

// メッセージを送信
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
