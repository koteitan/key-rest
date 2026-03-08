[English](gemini.md) | [日本語](gemini-ja.md)

## Gemini API

> **Note:** Gemini は URL クエリパラメータ `?key=` で API キーを渡します。

### セットアップ
```bash
./key-rest add user1/gemini/api-key https://generativelanguage.googleapis.com/
# → キーの値を入力してください: (Gemini API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const response = await fetch(
  'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key',
  {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      contents: [{ parts: [{ text: 'Hello, world!' }] }]
    })
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

body := strings.NewReader(`{"contents":[{"parts":[{"text":"Hello, world!"}]}]}`)
req, _ := keyrest.NewRequest("POST",
    "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key", body)
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

response = requests.post(
    'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent',
    params={'key': 'key-rest://user1/gemini/api-key'},
    json={
        'contents': [{'parts': [{'text': 'Hello, world!'}]}]
    }
).json()
```
