[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# Go システムテスト

Go テストとして記述されたエンドツーエンドテストです。test-server をビルド・起動し、キーストアとデーモンコンポーネントをインプロセスでセットアップし、Unix ソケットプロトコル経由で全 26 サービスをテストします。

## 実行方法

```bash
cd system-test/go
go test -v -count=1
```

## 動作の仕組み

1. 自己署名証明書付きで `test-server` をビルド・起動
2. test-server の標準出力からテスト用クレデンシャルをパース
3. キーストアを作成し、全キーを登録してメモリ上で復号
4. key-rest プロキシを使って Unix ソケットサーバーを起動
5. 全 26 サービスに認証付きリクエストを送信し、200 レスポンスを検証
