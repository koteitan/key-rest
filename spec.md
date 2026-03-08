# key-rest
agent に APP key などを見せずに、REST API に App key などの credential を埋め込んで呼び出すためのプロキシです。

# ブロック図

```mermaid
graph LR
    A[LLM agent]
    B[key-rest client library]
    D[key-rest-daemon]
    S[services]

    A -->|"request with key-rest:// URI"| B
    B -->|Unix socket| D
    D -->|"GET/POST with real credentials"| S
    S -->|response| D
    D -->|Unix socket| B
    B -->|response| A
    K[(APP KEY encrypted)]
    K -->|decrypt| D
```

# シーケンス図

```mermaid
sequenceDiagram
    participant U as USER
    participant A as LLM agent
    participant K as key-rest
    participant D as key-rest-daemon
    participant S as services

    Note over U,D: セットアップフェーズ
    U->>D: ./key-rest start
    D->>U: 秘密鍵を入力してください
    U->>D: (秘密鍵を入力)
    D->>D: 秘密鍵をメモリに保持<br/>暗号化されたキーを復号
    D->>U: daemon started

    U->>D: ./key-rest add user1/brave/api-key https://api.search.brave.com/
    D->>U: キーの値を入力してください
    U->>D: (キー値を入力)
    D->>D: キーを暗号化してファイルに保存<br/>メモリにも保持

    Note over A,S: API呼び出しフェーズ
    A->>K: fetch(url, {headers: {key-rest://...}})
    K->>D: リクエスト転送 (Unix socket)
    D->>D: key-rest:// URI を実際のキー値に置換
    D->>S: HTTP request (実際の credential 付き)
    S-->>D: HTTP response
    D-->>K: レスポンス転送 (Unix socket)
    K-->>A: Response オブジェクトを返却
```

# key-rest-daemon
key-rest-daemon は REST API を呼び出すためのデーモンです。APP KEY を保持し、key-rest からのリクエストを受け取って REST API を呼び出します。

## key-rest-daemon 制御用 commands
- `./key-rest start` : key-rest-daemon を起動します。
  - 起動時に秘密鍵を入力するように求められます。入力された秘密鍵はメモリに保存されます。ファイルには保存されません。
- `./key-rest status` : key-rest-daemon の状態を確認します。
- `./key-rest stop` : key-rest-daemon を停止します。
- `./key-rest add [options] <key-uri> <url-prefix>` : key-rest-daemon にキーを追加します。key は key-uri で指定され、対応する URL プレフィックスは url-prefix で指定されます。
  - key-rest-daemon が running 状態でないときは、秘密鍵を入力するように求められます。
  - key-rest-daemon が running 状態のときは、秘密鍵は入力する必要はありません。
  - そのあとに、キーの値を入力するように求められます。入力されたキーは暗号化されてファイルに保存されます。
  - オプション:
    - `--allow-url` : URL 内での置換を許可します (クエリパラメータ認証用: Gemini, Trello 等)
    - `--allow-body` : リクエストボディ内での置換を許可します (ボディ認証用: Tavily 等)
    - デフォルトでは headers 内のみ置換が許可されます
- `./key-rest remove <key>` : key-rest-daemon からキーを削除します。
- `./key-rest list` : key-rest-daemon に登録されているキーの一覧を表示します。
  - 出力例
    ```
    key1: url-prefix1
    key2: url-prefix2
    ```

## key-rest-daemon 状態

```mermaid
stateDiagram-v2
    [*] --> stopped
    stopped --> running : start (秘密鍵入力)
    running --> stopped : stop
```

| 状態 | 説明 |
|------|------|
| `stopped` | デーモンプロセスが停止している。ソケットは存在しない。 |
| `running` | デーモンプロセスが起動中。秘密鍵がメモリに保持され、暗号化されたキーが復号されている。Unix ソケットでリクエストを待ち受けている。 |

各状態で利用可能なコマンド:

| コマンド | stopped | running |
|----------|---------|---------|
| `start`  | OK | NG (already running) |
| `stop`   | NG (not running) | OK |
| `status` | OK (stopped と表示) | OK (running と表示) |
| `add`    | OK (秘密鍵入力が必要) | OK (秘密鍵入力不要) |
| `remove` | OK | OK |
| `list`   | OK | OK |

## データ保存

- データディレクトリ: `~/.key-rest/`
- 暗号化キーファイル: `~/.key-rest/keys.enc`
- Unix ソケット: `~/.key-rest/key-rest.sock`
- PID ファイル: `~/.key-rest/key-rest.pid`

### keys.enc 形式

キーは秘密鍵で暗号化され、以下の形式で保存されます:

```json
{
  "keys": [
    {
      "uri": "user1/brave/api-key",
      "url_prefix": "https://api.search.brave.com/",
      "allow_url": false,
      "allow_body": false,
      "encrypted_value": "<暗号化されたキー値(base64)>"
    }
  ]
}
```

暗号化方式: AES-256-GCM (秘密鍵から PBKDF2 で導出した鍵を使用)

## ソケット通信プロトコル

key-rest クライアントライブラリと key-rest-daemon の間は Unix ドメインソケット (`~/.key-rest/key-rest.sock`) で通信します。メッセージは改行区切りの JSON です。

### リクエスト形式

```json
{
  "type": "http",
  "method": "GET",
  "url": "https://api.example.com/data",
  "headers": {
    "Authorization": "Bearer key-rest://user1/example/api-key",
    "Content-Type": "application/json"
  },
  "body": null
}
```

### レスポンス形式 (成功時)

```json
{
  "status": 200,
  "statusText": "OK",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": "{\"results\": [...]}"
}
```

### レスポンス形式 (エラー時)

```json
{
  "error": {
    "code": "KEY_NOT_FOUND",
    "message": "Key 'user1/example/api-key' not found"
  }
}
```

エラーコード:

| code | 説明 |
|------|------|
| `KEY_NOT_FOUND` | 指定された key-rest:// URI が登録されていない |
| `URL_PREFIX_MISMATCH` | リクエスト先 URL が key の url_prefix と一致しない |
| `HTTP_ERROR` | 外部サービスへの HTTP リクエストが失敗した |

### key-rest:// URI の置換ルール

使用例は [examples/](examples/README.md) (2963592) を参照。

#### key-rest URI の形式

`key-rest://<key-uri>`

key-uri のパス区切りは `/`、各セグメントの有効文字は `[a-zA-Z0-9_.-]`。セグメント数に制限はない。

例: `key-rest://user1/service/key-name`, `key-rest://team/project/group/key`

#### Unenclosed (囲みなし) と Enclosed (囲みあり)

1Password CLI の secret reference syntax を参考に、2つの記法をサポートする。

**Unenclosed:** `key-rest://user1/service/key-name`
- URI の終端は `[a-zA-Z0-9/_.-]` 以外の文字、または文字列末尾
- ヘッダー値やクエリパラメータなど、URI の後に `/` が続かない場面で使用可能

**Enclosed:** `{{ key-rest://user1/service/key-name }}`
- 二重波括弧 `{{ }}` で URI の境界を明示する
- URI の直後に `/` やその他の有効文字が続く場面で必要
- 変換関数を適用できる: `{{ 変換関数(引数, ...) }}`

```
# Unenclosed: URI の後が = や行末なので曖昧さなし
Authorization: Bearer key-rest://user1/openai/api-key

# Enclosed: URI の後に /sendMessage が続くので囲みが必要
https://api.telegram.org/bot{{ key-rest://user1/telegram/bot-token }}/sendMessage

# Enclosed + 変換関数: base64 エンコードが必要な場合
Authorization: Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}
```

#### 変換関数

| 関数 | 説明 | 例 |
|------|------|-----|
| `base64(...)` | 引数を連結して base64 エンコードする | `{{ base64(key-rest://user1/email, ":", key-rest://user1/token) }}` |

- 引数はカンマ区切り
- 文字列リテラルはダブルクォートで囲む (例: `":"`)
- key-rest:// URI は置換後の値が使われる
- 将来的に他の変換関数を追加可能

#### 注入先のパターン分類

| パターン | 注入先 | 例 | 記法 |
|----------|--------|-----|------|
| URL クエリパラメータ | url | `?key=key-rest://user1/gemini/api-key` | unenclosed |
| カスタムヘッダー値 | headers | `X-Subscription-Token: key-rest://...` | unenclosed |
| Authorization ヘッダー | headers | `Authorization: Bearer key-rest://...` | unenclosed |
| Authorization Basic | headers | `Basic {{ base64(key-rest://..., ":", key-rest://...) }}` | enclosed + 変換 |
| URL パス埋め込み | url | `https://.../bot{{ key-rest://... }}/method` | enclosed |
| リクエストボディ | body | `{"api_key": "key-rest://..."}` | unenclosed |

#### 置換手順

1. リクエストの全フィールド (url, headers の各値, body) に対して以下の2パターンを検索する:
   - Enclosed: `\{\{.*?\}\}` → `{{ }}` 内を解析し、変換関数があれば関数と引数を抽出、なければ key-uri を抽出
   - Unenclosed: `key-rest://[a-zA-Z0-9/_.-]+` → そのまま key-uri を抽出
   - Enclosed を先に処理し、置換済みの箇所を Unenclosed の対象から除外する
2. 各マッチに含まれる key-rest:// URI について:
   a. key-uri が登録されていることを確認する
   b. リクエスト先 URL が key-uri に紐づいた `url_prefix` と前方一致することを確認する (セキュリティ制約)
   c. マッチが含まれるフィールドがそのキーで許可されていることを確認する (フィールド制限)
      - headers: 常に許可
      - url: `allow_url` が true の場合のみ許可
      - body: `allow_body` が true の場合のみ許可
3. key-rest:// URI を実際のキー値に置換する
4. 変換関数がある場合は適用する (例: `base64(...)` → 引数を連結して base64 エンコード)
5. マッチ箇所全体 (Enclosed の場合は `{{ }}` を含む) を最終結果で置換する

# key-rest
key-rest は LLM agent からの key-uri 付きの REST API 呼び出しを受け取り、key-rest-daemon にリクエストを転送し、key-rest-daemon からのレスポンスを LLM agent に返します。

key-rest は様々なインターフェースがあります。

## Node.js
### key-rest-fetch
fetch 互換のインターフェースです。fetch と同様の引数を受け取り、リクエストを key-rest-daemon に転送します。レスポンスも fetch の Response 互換の形式で返します。

```javascript
import { createFetch } from 'key-rest';

// key-rest-daemon に接続する fetch 関数を作成
const fetch = createFetch();  // デフォルト: ~/.key-rest/key-rest.sock

// 通常の fetch と同じ API で使用可能
const response = await fetch('https://api.example.com/data', {
  method: 'GET',
  headers: {
    'Authorization': 'Bearer key-rest://user1/example/api-key',
    'Content-Type': 'application/json'
  }
});
const data = await response.json();
```

### key-rest-ws
WebSocket 互換のインターフェースです。WebSocket と同様の引数を受け取り、キーを注入して WebSocket 接続を確立します。

```javascript
import { createWebSocket } from 'key-rest';

const WebSocket = createWebSocket();

const ws = new WebSocket('wss://api.example.com/ws', {
  headers: {
    'Authorization': 'Bearer key-rest://user1/example/api-key'
  }
});

ws.on('message', (data) => {
  console.log(data);
});
```

WebSocket の場合、key-rest-daemon が WebSocket 接続を維持し、クライアントとの間でメッセージを中継します。

## Go
### key-rest-http
net/http 互換のインターフェースです。http.Client と同様の API を提供し、リクエストを key-rest-daemon に転送します。レスポンスも `*http.Response` 互換の形式で返します。

```go
package main

import (
    "fmt"
    keyrest "github.com/koteitan/key-rest/go"
)

func main() {
    client := keyrest.NewClient()  // デフォルト: ~/.key-rest/key-rest.sock

    req, _ := keyrest.NewRequest("GET", "https://api.example.com/data", nil)
    req.Header.Set("Authorization", "Bearer key-rest://user1/example/api-key")

    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    fmt.Println(resp.StatusCode)
}
```

## Python
### key-rest-requests
requests 互換のインターフェースです。

```python
from key_rest import requests

response = requests.get(
    'https://api.example.com/data',
    headers={
        'Authorization': 'Bearer key-rest://user1/example/api-key',
        'Content-Type': 'application/json'
    }
)
data = response.json()
```

### key-rest-httpx
httpx 互換のインターフェースです。async/await に対応しています。

```python
from key_rest import httpx

async with httpx.AsyncClient() as client:
    response = await client.get(
        'https://api.example.com/data',
        headers={
            'Authorization': 'Bearer key-rest://user1/example/api-key',
        }
    )
    data = response.json()
```

## curl
### key-rest-curl
curl のラッパーコマンドです。curl と同じ引数を受け取り、key-rest:// URI を解決して実行します。

```bash
./key-rest curl https://api.example.com/data \
  -H "Authorization: Bearer key-rest://user1/example/api-key"
```

# REST API の使用例

[examples/](examples/README.md) を参照してください。
