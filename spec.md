[English](spec.md) | [Japanese](spec-ja.md)

# Internal Specification

## Data Storage

- Data directory: `~/.key-rest/`
- Encrypted key file: `~/.key-rest/keys.enc`
- Unix socket: `~/.key-rest/key-rest.sock`
- PID file: `~/.key-rest/key-rest.pid`

### keys.enc Format

Keys are encrypted with the passphrase and saved in the following format:

```json
{
  "keys": [
    {
      "uri": "user1/brave/api-key",
      "url_prefix": "https://api.search.brave.com/",
      "allow_url": false,
      "allow_body": false,
      "encrypted_value": "<encrypted key value (base64)>"
    }
  ]
}
```

Encryption method: AES-256-GCM (using a key derived from the passphrase via PBKDF2)

## Socket Communication Protocol

The key-rest client library and key-rest-daemon communicate via a Unix domain socket (`~/.key-rest/key-rest.sock`). Messages are newline-delimited JSON.

### Request Format

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

### Response Format (Success)

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

### Response Format (Error)

```json
{
  "error": {
    "code": "KEY_NOT_FOUND",
    "message": "Key 'user1/example/api-key' not found"
  }
}
```

Error Codes:

| code | Description |
|------|-------------|
| `KEY_NOT_FOUND` | The specified key-rest:// URI is not registered |
| `URL_PREFIX_MISMATCH` | The request URL does not match the key's url_prefix |
| `HTTP_ERROR` | The HTTP request to the external service failed |
