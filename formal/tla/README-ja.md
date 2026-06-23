[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# TLA+ 形式仕様

このディレクトリには、key-rest のクレデンシャルマスキングパイプラインに対する
TLA+ 仕様と TLC モデル検査設定が含まれています。

## 何を検証しているか

`MaskingPipeline.tla` は4つの定数でパラメタライズされた単一の仕様です。
`.cfg` ファイルを差し替えることで異なる性質を検証します。

### エンコーディング被覆（非空虚）

`caught` は `ServerEncodings` の列挙ではなく、ステップ被覆集合（`Covered`）から
導出されます。`ServerEncodings` は攻撃者の選択集合、`Covered` はパイプラインが
実際に処理できる集合です。`Covered` 外のエンコーディングがあれば TLC が
`NoCredentialLeak` 違反を報告します。

| エンコーディング | キャッチするステップ | Lean との対応 |
|---|---|---|
| `none` | —（クレデンシャル不在） | — |
| `raw` | Step 2: `maskCredentials` | `masks_raw` |
| `json_esc` | Step 2: `maskCredentials`（JSON形式） | `masks_json_esc` |
| `b64` | Step 1: `maskTransformOutputs` | `masks_b64` |
| `pct_raw` | Step 4: 寛容デコード → Step 2 | `masks_pct_raw` |
| `pct_b64` | Step 4: 寛容デコード → Step 1 | `masks_pct_b64` |
| `pct_stray` | Step 4: 寛容デコード（v1.0.1 修正） | `fixed_masks_stray_pct` |
| `pct_plus_b64` | Step 4: `+` を保持する寛容デコード（v1.0.2 修正） | `fixed_masks_plus_b64` |
| `trunc` | Step 3: `maskTruncatedKeys`（既知 API のみ） | — |

### 設定ファイル

| 設定 | 定数 | 期待結果 |
|---|---|---|
| `MaskingPipeline.cfg` | 全 covered エンコーディング、修正済みデコーダ、スナップショット有効 | **No error** |
| `MaskingPipeline_negative.cfg` | `base64url`・`hex`・`rot13` を追加 | **VIOLATED** — 非空虚性の確認 |
| `MaskingPipeline_buggy.cfg` | `ExtraStep4Catches={}` (pre-v1.0.2 デコーダ) | **VIOLATED** — `pct_stray`・`pct_plus_b64` 漏洩 |
| `MaskingPipeline_naive.cfg` | `UseSnapshot=FALSE`（スナップショットなし） | **VIOLATED** — `DisableKey` 競合で漏洩 |
| `MaskingPipeline_disorder.cfg` | `Step4Enabled=FALSE`（Step 4 省略） | **VIOLATED** — `pct_raw`・`pct_b64` 漏洩 |

### 全称性

TLC が網羅的に探索する組み合わせ：
- `serverEncoding` ∈ `ServerEncodings`（攻撃者の選択を全探索）
- `apiKnown` ∈ {`TRUE`, `FALSE`}（既知 API / 未知 API）
- `placement` ∈ {`"header"`, `"body"`}（エージェントの配置選択）
- `DisableKey` の発火タイミング = `snap` 〜 `s4` の全インターリービング

### 不変条件

| 不変条件 | 内容 |
|---|---|
| `TypeOK` | 型の正しさ |
| `NoCredentialLeak` | エージェントが生クレデンシャルを見ない（`IntentionalLimit` を除く） |
| `ResolvedImpliesAllowed` | クレデンシャルは許可された配置にのみ転送される |

## 前提条件

- Java ランタイム（JRE 11 以上）
- [TLA+ ツール](https://github.com/tlaplus/tlaplus/releases) — `tla2tools.jar`

`tla2tools.jar` をリポジトリルート（このディレクトリの2つ上）に配置してください。

## 検証方法

```sh
# リポジトリルートから
java -jar tla2tools.jar -config formal/tla/MaskingPipeline.cfg formal/tla/MaskingPipeline.tla
```

全設定を一括実行：

```sh
for cfg in formal/tla/MaskingPipeline*.cfg; do
    echo "=== $cfg ==="
    java -jar tla2tools.jar -config "$cfg" formal/tla/MaskingPipeline.tla 2>&1 | tail -3
done
```

正例は `No error has been found` で終わり、負例4つはいずれも
`Invariant NoCredentialLeak is violated` エラーを報告します。
