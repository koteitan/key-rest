[English](spec.md) | [Japanese](spec-ja.md)

# 内部仕様

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

## メモリセキュリティ

平文の秘密情報を保持するすべてのメモリ領域は `mlock` (スワップ防止) でロックされ、不要になった時点でゼロクリアされます。

### APIキー

#### 入力 (`key-rest add`)

| # | メモリ領域 | 箇所 | mlock | ゼロクリアタイミング |
|---|-----------|------|-------|-------------------|
| 1 | `oneByte` (1byte読み取り変数) | `main.go:317` | No (スタック、1byte) | 次の文字入力時に上書き |
| 2 | `buf` (4096byte入力バッファ) | `main.go:313` | Yes | `result` へコピー直後 (Enter押下時) |
| 3 | `result` = `value` (返り値) | `main.go:328` | Yes | `cmdAdd` 終了時 (addコマンド完了時) |
| 4 | `key` (PBKDF2派生AES鍵) | `crypto.go:39` | Yes | `Encrypt` 終了時 |
| 5 | `valueCopy` (デーモンのインメモリ保持) | `keystore.go:129` | Yes | `ClearAll` 時 (デーモン停止 or reload) |

#### 復号 (デーモン起動 / reload 時の `DecryptAll`)

| # | メモリ領域 | 箇所 | mlock | ゼロクリアタイミング |
|---|-----------|------|-------|-------------------|
| 1 | `key` (PBKDF2派生AES鍵) | `crypto.go:79` | Yes | `Decrypt` 終了時 (各キー毎) |
| 2 | `plaintext` (`gcm.Open` が返す復号済みAPIキー) | `crypto.go:93` | Yes (`keystore.go:233` にて) | `ClearAll` 時 (デーモン停止 or reload) |
| 3 | `DecryptedKey.Value` (デーモンのインメモリ保持) | `keystore.go:235` | Yes | `ClearAll` 時 (デーモン停止 or reload) |

注: #2 と #3 は同一のメモリ領域。`gcm.Open` が確保したバッファが `Decrypt` → `DecryptAll` を経由して返され、`keystore.go:233` で mlock される。

#### リクエスト処理 (各APIコール時の `proxy.Handle`)

| # | メモリ領域 | 箇所 | mlock | ゼロクリアタイミング |
|---|-----------|------|-------|-------------------|
| 1 | `string(val)` (APIキーを Go string に変換) | `uri.go:191` | **No** | GC (制御不能) |
| 2 | `resolvedURL` / `resolvedHeaders` / `resolvedBody` (APIキーが埋め込まれた文字列) | `proxy.go:68,76,86` | **No** | GC (制御不能) |
| 3 | `http.Request` のフィールド (URL, Header, Body) | `net/http` 内部 | **No** | GC (制御不能) |

**制約:** リクエスト処理時、APIキーの値は URI 置換と HTTP リクエスト構築のために Go string に変換される (`uri.go:191` の `string(val)`)。Go の string は immutable で GC 管理のため、mlock もゼロクリアも不可能。これらの文字列はリクエスト処理中のみ存在し、リクエスト完了後すぐに GC の回収対象となる。

### マスターキー (パスフレーズ)

#### 入力 — 親プロセス (`key-rest start`、ターミナル側)

| # | メモリ領域 | 箇所 | mlock | ゼロクリアタイミング |
|---|-----------|------|-------|-------------------|
| 1 | `oneByte` (1byte読み取り変数) | `main.go:317` | No (スタック、1byte) | 次の文字入力時に上書き |
| 2 | `buf` (4096byte入力バッファ) | `main.go:313` | Yes | `result` へコピー直後 (Enter押下時) |
| 3 | `result` = `passphrase` (返り値) | `main.go:328` | Yes | `cmdStart` 終了時 (fork+パイプ書き込み後) |

#### 入力 — 子プロセス (`key-rest start`、デーモン、KEY_REST_FOREGROUND=1)

| # | メモリ領域 | 箇所 | mlock | ゼロクリアタイミング |
|---|-----------|------|-------|-------------------|
| 1 | `buf` (4096byteパイプ読み取りバッファ) | `main.go:279` | Yes | `result` へコピー直後 |
| 2 | `result` = `passphrase` (返り値) | `main.go:290` | Yes | `cmdStart` 終了時 (デーモン終了後) |
| 3 | `d.passphrase` (デーモンが保持するコピー) | `daemon.go:85` | Yes | `shutdown()` 内 (SIGTERM受信時) |
