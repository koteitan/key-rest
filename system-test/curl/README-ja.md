[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# curl システムテスト

シェルスクリプトによるエンドツーエンドテストです。バイナリをビルドし、test-server と key-rest デーモンを別プロセスとして起動し、CLI 経由で全クレデンシャルを登録し、[key-rest-curl](../../clients/curl/key-rest-curl) を通じて全 26 サービスをテストします。

## 実行方法

```bash
system-test/curl/system-test.sh
```

## 動作の仕組み

1. `test-server` と `key-rest` バイナリを一時ディレクトリにビルド
2. ランダムポートで自己署名証明書付きの `test-server` を起動
3. test-server の標準出力からテスト用クレデンシャルをパース
4. パイプでパスフレーズ/値を渡しながら `key-rest add` で全 28 キーを登録
5. テスト証明書を信頼するよう `SSL_CERT_FILE` を設定してデーモンを起動
6. `key-rest-curl` 経由で全 26 サービスに認証付きリクエストを送信し、結果を報告
