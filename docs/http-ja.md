[← Back](README.md) | [English](http.md) | [Japanese](http-ja.md)

# Go http.Get から TCP まで: 呼び出しチェーンと遅延置換

## http.Get から TCP 送信までの呼び出しチェーン

```
http.Get(url)
  ↓
http.Client.Do(req)
  ↓
http.Transport.RoundTrip(req)
  ↓
Transport.getConn()          ← TCP 接続の取得/作成
  │
  ├─ net.Dial("tcp", "api.example.com:443")   ← TCP 接続
  │
  └─ tls.Client(tcpConn, config)              ← TLS レイヤーでラップ
     tls.Conn.Handshake()                     ← TLS ハンドシェイク
  ↓
persistConn.roundTrip(req)
  ↓
req.Write(bufio.Writer)      ← HTTP リクエストを文字列としてシリアライズ
  │
  │  "GET /v1/chat HTTP/1.1\r\n"
  │  "Host: api.example.com\r\n"
  │  "Authorization: Bearer sk-xxxxx\r\n"    ← 平文
  │  "\r\n"
  │  (body)
  ↓
bufio.Writer.Flush()
  ↓
tls.Conn.Write(plaintext bytes)  ← ★ ここで暗号化が行われる
  ↓
tls.Conn.writeRecord()       ← 暗号化して TLS レコードにパッケージング
  ↓
net.Conn.Write(ciphertext)   ← TCP で送信
```

暗号化は `tls.Conn.Write()` 内で行われます。

## DialTLSContext による遅延置換

暗号化前の平文を傍受するために、`http.Transport` は `DialTLSContext` フックを提供しています:

```go
transport := &http.Transport{
    DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        // 実際の TLS 接続を確立
        tlsConn := tls.Dial(network, addr, tlsConfig)

        // ラッパーを返す
        return &replacingConn{inner: tlsConn, store: store}, nil
    },
}
```

`Transport` は返された `net.Conn` に平文を書き込みます（TLS は既に処理済みと認識）。ラッパーの `Write()` は:

1. mlock されたバッファにコピー
2. `key-rest://` → 実際のキーに置換
3. `inner.Write()`（実際の `tls.Conn.Write()`）で暗号化
4. mlock されたバッファをゼロクリア

これにより、`bufio.Writer` までの全ての処理は `key-rest://xxx` プレースホルダーのみを扱います。平文のキーは mlock されたバッファ内にのみ存在します。
