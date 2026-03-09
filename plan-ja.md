[English](plan.md) | [Japanese](plan-ja.md)

# 実装計画

## プラットフォーム選定

### daemon: Go

| 観点 | Go | Node.js | Rust |
|------|-----|---------|------|
| シークレットのメモリ管理 | ◯ byte slice を明示的にゼロクリア可能 | ✕ GC + immutable string で制御不能 | ◎ zeroize crate で自動消去 |
| 暗号プリミティブ | ◯ crypto/aes, crypto/cipher (GCM), crypto/pbkdf2 が stdlib | ◯ crypto module あり | ◯ ring/rust-crypto |
| デーモン化 | ◯ シングルバイナリ、依存なし | △ node ランタイム必要 | ◯ シングルバイナリ |
| Unix ソケット | ◯ net.Listen("unix", ...) | ◯ net.createServer | ◯ tokio/std |
| mlock (メモリロック) | ◯ syscall.Mlock | ✕ 標準サポートなし | ◯ memsec crate |
| 開発速度 | ◯ | ◎ | △ |

**選定理由:** credential を扱うデーモンでは、シークレットの明示的なメモリ制御（ゼロクリア、mlock）が重要。Go は crypto stdlib が充実しており、シングルバイナリで配布可能。Rust ほど学習コストが高くなく、Node.js より安全にシークレットを扱える。

### clients: 各言語ネイティブ

クライアントは Unix ソケットに JSON を送受信する薄いラッパーなので、各言語のイディオムに合わせて実装する。

| クライアント | 言語 | インターフェース |
|-------------|------|----------------|
| key-rest-fetch | Node.js (TypeScript) | fetch 互換 |
| key-rest-ws | Node.js (TypeScript) | WebSocket 互換 |
| key-rest-http | Go | net/http 互換 |
| key-rest-requests | Python | requests 互換 |
| key-rest-httpx | Python | httpx 互換 |
| key-rest-curl | Shell (bash) | curl ラッパー |

## フォルダ構成

```
key-rest/
├── CLAUDE.md
├── README.md
├── plan.md
├── examples/                  # 使用例 (既存)
│   ├── README.md
│   └── *.md
│
├── go.mod                     # Go module root
├── go.sum
├── Makefile                   # build, test, install
│
├── cmd/                       # CLI エントリポイント
│   └── key-rest/
│       └── main.go            # ./key-rest start|stop|status|add|remove|list|curl
│
├── internal/                  # daemon 内部パッケージ (外部から import 不可)
│   ├── daemon/                # プロセス管理 (start/stop/status, PID ファイル)
│   ├── crypto/                # AES-256-GCM 暗号化/復号, PBKDF2 鍵導出
│   ├── keystore/              # keys.enc の読み書き, キー管理
│   ├── server/                # Unix ソケットサーバー, JSON プロトコル処理
│   ├── proxy/                 # HTTP/WebSocket プロキシ, 外部サービス呼び出し
│   └── uri/                   # key-rest:// URI パース, 置換 (enclosed/unenclosed, 変換関数)
│
├── clients/
│   ├── node/                  # Node.js クライアント
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   └── src/
│   │       ├── fetch.ts       # createFetch()
│   │       └── ws.ts          # createWebSocket()
│   │
│   ├── go/                    # Go クライアント
│   │   ├── go.mod
│   │   ├── client.go          # NewClient(), NewRequest()
│   │   └── client_test.go
│   │
│   ├── python/                # Python クライアント
│   │   ├── pyproject.toml
│   │   └── key_rest/
│   │       ├── __init__.py
│   │       ├── requests.py    # requests 互換
│   │       └── httpx.py       # httpx 互換
│   │
│   └── curl/                  # curl ラッパー
│       └── key-rest-curl.sh
│
└── test/                      # 統合テスト
    ├── integration_test.go    # daemon + client の結合テスト
    └── testdata/              # テスト用の暗号化キーなど
```

## internal/ パッケージの責務

| パッケージ | 責務 | セキュリティ上の注意 |
|-----------|------|---------------------|
| `crypto` | AES-256-GCM 暗号化/復号、PBKDF2 鍵導出、salt 生成 | crypto/rand のみ使用、鍵の byte slice は使用後ゼロクリア |
| `keystore` | keys.enc の CRUD、メモリ上の復号済みキー保持 | 復号済みキーは mlock でスワップ防止、ファイル権限 0600 |
| `daemon` | プロセスの fork/管理、PID ファイル、シグナルハンドリング | SIGTERM で graceful shutdown、ゼロクリア後に終了 |
| `server` | Unix ソケットリスナー、JSON リクエスト/レスポンス処理 | ソケット権限 0600、リクエストサイズ制限 |
| `proxy` | HTTP/WebSocket リクエスト代行、レスポンス中継 | TLS 検証必須、タイムアウト設定 |
| `uri` | key-rest:// URI の検出・置換、enclosed/unenclosed パース、変換関数 | url_prefix の前方一致検証 (キー漏洩防止) |

## 実装順序

1. `internal/crypto` — 暗号化の基盤
2. `internal/keystore` — キーの永続化
3. `cmd/key-rest` + `internal/daemon` — CLI と add/list/remove (daemon なしで動作する部分)
4. `internal/uri` — URI パース・置換エンジン
5. `internal/proxy` — HTTP プロキシ
6. `internal/server` — Unix ソケットサーバー
7. `cmd/key-rest` — start/stop/status (daemon 化)
8. `clients/curl` — 最もシンプルなクライアント (動作確認用)
9. `clients/node` — Node.js クライアント
10. `clients/go` — Go クライアント
11. `clients/python` — Python クライアント
