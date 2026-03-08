## DeepSeek API

> **Note:** DeepSeek は OpenAI 互換 API を提供しています。

### セットアップ
```bash
./key-rest add user1/deepseek/api-key https://api.deepseek.com/
# → キーの値を入力してください: (DeepSeek API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://api.deepseek.com/chat/completions',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer key-rest://user1/deepseek/api-key',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'deepseek-chat',
      messages: [{ role: 'user', content: 'Hello!' }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"model":"deepseek-chat","messages":[{"role":"user","content":"Hello!"}]}`)
req, _ := keyrest.NewRequest("POST", "https://api.deepseek.com/chat/completions", body)
req.Header.Set("Authorization", "Bearer key-rest://user1/deepseek/api-key")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://api.deepseek.com/chat/completions',
    headers={
        'Authorization': 'Bearer key-rest://user1/deepseek/api-key',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'deepseek-chat',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
).json()
```
