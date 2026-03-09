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
