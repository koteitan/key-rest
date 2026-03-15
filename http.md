[English](http.md) | [Japanese](http-ja.md)

# Go http.Get to TCP: Call Chain and Delayed Replacement

## Call chain from http.Get to TCP send

```
http.Get(url)
  ↓
http.Client.Do(req)
  ↓
http.Transport.RoundTrip(req)
  ↓
Transport.getConn()          ← get/create TCP connection
  │
  ├─ net.Dial("tcp", "api.example.com:443")   ← TCP connection
  │
  └─ tls.Client(tcpConn, config)              ← wrap with TLS layer
     tls.Conn.Handshake()                     ← TLS handshake
  ↓
persistConn.roundTrip(req)
  ↓
req.Write(bufio.Writer)      ← serialize HTTP request as string
  │
  │  "GET /v1/chat HTTP/1.1\r\n"
  │  "Host: api.example.com\r\n"
  │  "Authorization: Bearer sk-xxxxx\r\n"    ← plaintext
  │  "\r\n"
  │  (body)
  ↓
bufio.Writer.Flush()
  ↓
tls.Conn.Write(plaintext bytes)  ← ★ encryption happens here
  ↓
tls.Conn.writeRecord()       ← encrypt and package into TLS record
  ↓
net.Conn.Write(ciphertext)   ← send over TCP
```

Encryption happens inside `tls.Conn.Write()`.

## Delayed replacement via DialTLSContext

To intercept plaintext before encryption, `http.Transport` provides a `DialTLSContext` hook:

```go
transport := &http.Transport{
    DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        // Establish real TLS connection
        tlsConn := tls.Dial(network, addr, tlsConfig)

        // Return a wrapper
        return &replacingConn{inner: tlsConn, store: store}, nil
    },
}
```

`Transport` writes plaintext to the returned `net.Conn` (believing TLS is already handled). The wrapper's `Write()`:

1. Copy to mlocked buffer
2. Replace `key-rest://` → actual key
3. `inner.Write()` (real `tls.Conn.Write()`) encrypts
4. Zero-clear the mlocked buffer

This way, everything up to `bufio.Writer` only sees `key-rest://xxx` placeholders. The plaintext key exists only in the mlocked buffer.
