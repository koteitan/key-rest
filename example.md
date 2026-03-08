# REST API の使用例

## Gemini API

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

## Brave Search API

### セットアップ
```bash
./key-rest add user1/brave/api-key https://api.search.brave.com/
# → キーの値を入力してください: (Brave API key を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const results = await fetch(
  'https://api.search.brave.com/res/v1/web/search?q=hello+world',
  {
    headers: {
      'X-Subscription-Token': 'key-rest://user1/brave/api-key',
      'Accept': 'application/json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.search.brave.com/res/v1/web/search?q=hello+world", nil)
req.Header.Set("X-Subscription-Token", "key-rest://user1/brave/api-key")
req.Header.Set("Accept", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

results = requests.get(
    'https://api.search.brave.com/res/v1/web/search',
    params={'q': 'hello world'},
    headers={
        'X-Subscription-Token': 'key-rest://user1/brave/api-key',
        'Accept': 'application/json'
    }
).json()
```

## GitHub API

### セットアップ
```bash
./key-rest add user1/github/token https://api.github.com/
# → キーの値を入力してください: (GitHub Personal Access Token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const repos = await fetch(
  'https://api.github.com/user/repos?sort=updated',
  {
    headers: {
      'Authorization': 'Bearer key-rest://user1/github/token',
      'Accept': 'application/vnd.github+json'
    }
  }
).then(r => r.json());
```

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.github.com/user/repos?sort=updated", nil)
req.Header.Set("Authorization", "Bearer key-rest://user1/github/token")
req.Header.Set("Accept", "application/vnd.github+json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

repos = requests.get(
    'https://api.github.com/user/repos',
    params={'sort': 'updated'},
    headers={
        'Authorization': 'Bearer key-rest://user1/github/token',
        'Accept': 'application/vnd.github+json'
    }
).json()
```

## Atlassian API

### セットアップ
```bash
./key-rest add user1/atlassian/email https://api.bitbucket.org/
# → キーの値を入力してください: (Atlassian email を入力)
./key-rest add user1/atlassian/token https://api.bitbucket.org/
# → キーの値を入力してください: (Atlassian API token を入力)
```

### Node.js
```javascript
import { createFetch } from 'key-rest';
const fetch = createFetch();

const prs = await fetch(
  'https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN',
  {
    headers: {
      'Authorization': 'Basic key-rest://user1/atlassian/email:key-rest://user1/atlassian/token',
      'Content-Type': 'application/json'
    }
  }
).then(r => r.json());
```

> **Note:** `Authorization: Basic <plaintext>` の場合、key-rest-daemon が URI 置換後に自動で base64 エンコードを適用します。

### Go
```go
client := keyrest.NewClient()

req, _ := keyrest.NewRequest("GET",
    "https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests?state=OPEN", nil)
req.Header.Set("Authorization",
    "Basic key-rest://user1/atlassian/email:key-rest://user1/atlassian/token")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

### Python
```python
from key_rest import requests

prs = requests.get(
    'https://api.bitbucket.org/2.0/repositories/my-workspace/my-repo/pullrequests',
    params={'state': 'OPEN'},
    headers={
        'Authorization': 'Basic key-rest://user1/atlassian/email:key-rest://user1/atlassian/token',
        'Content-Type': 'application/json'
    }
).json()
```
