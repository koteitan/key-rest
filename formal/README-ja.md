[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# 形式検証

このディレクトリには、key-rest のクレデンシャルマスキングパイプラインに対する
2つの相補的な形式検証の成果物が含まれています。

## リンク

| ツール | ディレクトリ | 目的 |
|---|---|---|
| Lean 4 | [formal/lean](lean/README-ja.md) | 文字列マスキング正しさのカーネル検証済み証明 |
| TLA+ / TLC | [formal/tla](tla/README-ja.md) | パイプライン調整・TOCTOU・配置ゲートのモデル検査 |

## 形式化範囲

### Lean が担う範囲

Lean はマスキング**関数そのもの**の性質を証明します — レスポンス本文内で
クレデンシャルを検索・置換する文字列操作の正しさです。

| 層 | 証明内容 |
|---|---|
| 回帰アンカー（Part 1） | 固定テストベクタに対する5つのエンコーディングシナリオが正しくマスクされる（`decide`、カーネル検証済み） |
| 全称定理（Part 2） | `percent_roundtrip`・`infix_replaceAll_false`・`masks_cred_universal` — 側条件を満たす**全ての**入力で成立 |
| バグ検出（Part 2c） | `buggy_leaks_stray_pct` で pre-v1.0.1 のバグを証明；`fixed_masks_stray_pct`・`fixed_masks_plus_b64` で両修正が正しいことを証明 |

Lean が証明できないもの：並行性・ステップ順序・配置ゲート・時系列にわたる
プログラム状態に依存する性質。

### TLA+ が担う範囲

TLA+ はパイプラインの**調整**を検証します — ステップの合成・順序・並行変更
への保護です。

| 性質 | 検証方法 |
|---|---|
| 全 covered エンコーディングがマスクされる | `MaskingPipeline.cfg` — `NoCredentialLeak` 成立 |
| 未 covered エンコーディングはマスクされない（非空虚） | `MaskingPipeline_negative.cfg` — `base64url`・`hex`・`rot13` で違反 |
| pre-v1.0.2 バグデコーダは漏洩する | `MaskingPipeline_buggy.cfg` — `pct_stray`・`pct_plus_b64` で違反 |
| スナップショットが並行 disable TOCTOU を防ぐ | `MaskingPipeline_naive.cfg` — スナップショットなしで違反 |
| Step 4 は load-bearing（順序が重要） | `MaskingPipeline_disorder.cfg` — Step 4 省略で違反 |
| 配置ゲートが不正配置リクエストを拒否 | `ResolvedImpliesAllowed` — 全正例 cfg で成立 |

TLA+ が証明できないもの：文字列レベルのマスキング正しさ（無限文字列域）、
Go ランタイム・ライブラリ関数の性質。

### どちらも証明しないもの

- **主要防御は配置検証**（クレデンシャルの送信先に対する既定拒否）。マスキングは
  二次的なブロックリストの網にすぎません。マスキングを完全に証明しても
  「クレデンシャルは漏れない」を意味しません。
- **`∀ encoding` は偽（機構上）。** マスキングは固定のエンコーディング形式を
  列挙するブロックリストです。リスト外（base64url・hex・二重 percent-encode・
  文字間スペース・ストリーム分割等）はすり抜けます。
- **自由文 LLM レスポンス本文** — 全文字種・無制限長。アローリストも
  ブロックリストも封じ込めできません。

## 全称性

| 次元 | Lean | TLA+ |
|---|---|---|
| レスポンス本文の内容 | **∀ 文字列**（全文字列、帰納法） | 有限のエンコーディング記号集合 |
| クレデンシャルの値 | **∀ 文字列**（`Disj` 側条件付き） | 単一の抽象クレデンシャル |
| URI / テンプレート | **∀ 文字列** | 固定の抽象モデル |
| エンコーディング形式 | 8 つの具体的シナリオ | **∀ enc ∈ ServerEncodings**（全探索） |
| `apiKnown` | モデル化なし | **∀ {TRUE, FALSE}**（両方探索） |
| 配置 | モデル化なし | **∀ {"header", "body"}**（両方探索） |
| 並行 disable/reload | モデル化なし | **∀ インターリービング**（全フェーズ組み合わせ） |
| デコーダ正しさ | `queryUnescape`・`percentDecodeAll` を証明 | 抽象化（文字列でなく記号） |

2つのツールは**相補的**です：Lean は TLC が届かない無限文字列域を全称化し、
TLA+ は Lean の帰納法が適用できない並行インターリービングとプログラム状態を全探索します。
