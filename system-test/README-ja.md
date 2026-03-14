[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# システムテスト

全 26 サービスが key-rest を通じて正しく動作することを検証するエンドツーエンドテストです。各テストは [test-server](../test-server/README-ja.md) を起動し、クレデンシャルを登録し、key-rest プロキシ経由で認証付きリクエストを送信します。

## テストバリアント

| バリアント | 説明 |
|---|---|
| [go/](go/README-ja.md) | `go test` によるGoテスト（インライン Unix ソケットクライアント使用） |
| [curl/](curl/README-ja.md) | [key-rest-curl](../clients/curl/key-rest-curl) を使用するシェルスクリプト |
| [python/](python/README-ja.md) | `key_rest.requests` クライアントライブラリを使用する Python スクリプト |
| [node/](node/README-ja.md) | `node:net` Unix ソケットクライアントを使用する Node.js スクリプト |

## 前提条件

- Go 1.21+
- bash（curl バリアント用）
- python3
- Node.js 18+
