[← Back](README-ja.md) | [English](memory.md) | [Japanese](memory-ja.md)

# クレデンシャルのメモリ生存期間

復号化された API キーがメモリ上でどのように保護され、どこにギャップがあるかを追跡する。

## 概要

**リクエストパス**（クレデンシャル置換 → TLS 暗号化）は mlock とゼロクリアで適切に保護されている。**レスポンスマスキングパス**でのみ、Go の不変 `string` 型によりゼロクリアできないクレデンシャルのコピーが生じる。

## フェーズ 1: 復号化と保存

`internal/keystore/keystore.go:DecryptAll()` (行 258)

| ステップ | 型 | mlock | ゼロクリア | 備考 |
|---|---|---|---|---|
| `crypto.Decrypt()` が平文を返す | `[]byte` | はい | - | 行 280 で `crypto.Mlock(value)` |
| `DecryptedKey.Value` に格納 | `[]byte` | はい | ClearAll/Disable 時 | daemon の生存中メモリに保持 |

**状態:** 安全。復号直後に mlock される。

## フェーズ 2: リクエスト検証

`internal/proxy/proxy.go:Handle()` (行 109)

| ステップ | 型 | mlock | ゼロクリア | 備考 |
|---|---|---|---|---|
| `validateField()` が `key-rest://` URI を検査 | `string` | - | - | URI 文字列のみ、クレデンシャル値は含まない |
| `store.Lookup()` がポインタを返す | pointer | はい | - | mlock データへのポインタ |
| `collectTransformOutputs()` | `string` | **いいえ** | **なし** | [脆弱性 #1](#脆弱性-1-collecttransformoutputs) 参照 |

## フェーズ 3: 遅延解決と TLS 暗号化

`internal/proxy/sectransport.go:RoundTrip()` (行 54)

key-rest のセキュリティ設計の核心：クレデンシャルは `net/http` パイプライン全体を `key-rest://` URI のまま通過し、TLS 暗号化の直前でのみ解決される。

### 3a. URI 解決

| ステップ | 行 | 型 | mlock | ゼロクリア | 備考 |
|---|---|---|---|---|---|
| `uri.ReplaceBytes(body)` | 70 | `[]byte` | はい | はい (行 164) | body 内のクレデンシャル置換 |
| `uri.ReplaceBytes(URI)` | 86 | `[]byte` | はい | はい (行 163) | URL パス内のクレデンシャル置換 |
| `uri.ReplaceBytes(header)` | 98 | `[]byte` | はい | はい (行 165-167) | ヘッダーごとの解決 |

`uri.ReplaceBytes()` 内部（`internal/uri/uri.go:227`）:
- 行 286: keystore からのコピー → 中間コピー
- 行 268: `zeroClear(r.value)` で各マッチの解決バッファをゼロクリア
- 行 316: `zeroClear(concatenated)` で base64 連結バッファをゼロクリア
- 行 263: 結果バッファは事前割当（`append` による再割当なし）

**状態:** 安全。すべての中間 `[]byte` バッファがゼロクリアされる。

### 3b. mlock バッファ構築

| ステップ | 行 | 型 | mlock | ゼロクリア | 備考 |
|---|---|---|---|---|---|
| 正確なサイズでバッファ割当 | 142 | `[]byte` | はい | - | `crypto.Mlock(buf)` |
| `copy()` で HTTP/1.1 リクエスト構築 | 145-160 | `[]byte` | はい | - | mlock バッファ内で直接構築 |
| 中間バッファをゼロクリア | 163-167 | - | - | はい | `resolvedURI`, `resolvedBody`, headers |

**状態:** 安全。再割当は発生しない（事前にサイズ計算済み）。

### 3c. TLS 書き込みとゼロクリア

| ステップ | 行 | 型 | mlock | ゼロクリア | 備考 |
|---|---|---|---|---|---|
| `conn.Write(buf)` | 191 | `[]byte` | はい | - | TLS 接続に書き込み |
| `crypto.ZeroClearAndMunlock(buf)` | 192 | - | - | はい | 書き込み直後にゼロクリア |

**状態:** 安全。平文は `conn.Write()` の間のみ存在。

## フェーズ 4: レスポンスマスキング

`internal/proxy/proxy.go:Handle()` (行 190-206)

Go の不変 `string` 型により、既知の脆弱性がある。

### <a id="脆弱性-1-collecttransformoutputs"></a>脆弱性 #1: `collectTransformOutputs()`

`proxy.go` 行 534-562

```go
resolved, err := uri.ResolveMatch(m, resolver)  // string を返す
outputs[resolved] = original                     // map キーに string 格納
```

`uri.ResolveMatch()` は内部で `[]byte` → `string` 変換を行う（不変、ゼロクリア不可）。base64 エンコードされたクレデンシャル値が、GC 回収まで Go ヒープに残留する。

### 脆弱性 #2: `maskCredentials()`

`proxy.go` 行 443-471

```go
raw := string(dk.Value)                                   // []byte → string コピー
jsonBytes, _ := json.Marshal(raw)                         // JSON エスケープされた新しい []byte
jsonEscaped := string(jsonBytes[1:len(jsonBytes)-1])      // さらに string コピー
```

ゼロクリアされない3つのコピーが生成される:
1. `raw` — 平文 `string`
2. `jsonBytes` — JSON エスケープされた `[]byte`
3. `jsonEscaped` — JSON エスケープされた `string`

いずれもゼロクリアされない（`string` は不変、`jsonBytes` は明示的にクリアされない）。

### 脆弱性 #3: `maskTruncatedKeys()`

`proxy.go` 行 484-513

```go
raw := string(dk.Value)  // []byte → string コピー
```

脆弱性 #2 と同じ問題。平文クレデンシャルが不変の `string` になる。

### 脆弱性 #4: `maskPercentEncoded()`

`proxy.go` 行 519-532

内部で `maskCredentials()` を呼び出すため、脆弱性 #2 を継承する。

## 生存期間まとめ

```
                    リクエストパス（安全）
                    =====================
keystore            key-rest://URI         mlock バッファ     TLS
[Value]  ──copy──►  [ReplaceBytes]  ──copy──►  [HTTP/1.1]  ──write──►  暗号化
 mlock      zero     []byte           zero      mlock         zero
            clear                     clear                   clear

                    レスポンスパス（脆弱）
                    ==========================
keystore            maskCredentials         strings.ReplaceAll
[Value]  ──string()──►  [raw string]  ──ReplaceAll──►  [マスク済み body]
 mlock       ⚠️ zero    ⚠️ zero クリア    GC が古い string
             クリア不可   不可能            を管理
```

## 脅威評価

| 攻撃者 | GC 残留を読めるか | 備考 |
|---|---|---|
| LLM agent（同一ユーザー） | **読めない** | `PR_SET_DUMPABLE=0` が `/proc/PID/mem` をブロック |
| 他のユーザー | **読めない** | Unix パーミッション |
| root | **読める** | 全プロセスメモリを読める |
| ディスクフォレンジック（スワップ） | **可能性あり** | GC 管理の `string` は mlock されていない、スワップされうる |

**結論:** GC の脆弱性は key-rest の脅威モデル（同一ユーザーの LLM agent）では悪用できない。`PR_SET_DUMPABLE=0` がメモリ読み取りを防いでいるため。root レベルの攻撃者やディスクフォレンジックでのみ問題となるが、これらは key-rest の保護範囲外である。

## 改善案

`string` ベースのマスキング関数を `[]byte` + mlock + ゼロクリアベースに置き換える。これにより4つの脆弱性すべてが解消されるが、`strings.ReplaceAll`、`json.Marshal`、`regexp.ReplaceAllString` を `[]byte` + mlock 版で再実装する必要がある。
