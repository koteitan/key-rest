[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# Lean 形式証明

このディレクトリには、key-rest のクレデンシャルマスキングパイプラインに対する
Lean 4 カーネル検証済み証明が含まれています。

`MaskingPipeline.lean` は2つの相補的な層から構成されます。

## Part 1 — リグレッションアンカー（`decide`）

固定テストベクタ（`cred = "ab~de"`、`b64cred = "YWJ+ZGU="`、`uri = "u/k"`）に対して
特定のサーバーエンコーディングシナリオを固定する閉じた命題です。これらは一般定理では
なく「カーネル検証済みのテストベクタ」ですが、安価かつ厳密で、具体的なすり抜けを
ロックします。

| 定理 | シナリオ |
|---|---|
| `masks_raw` | クレデンシャルが生のまま含まれる |
| `masks_json_esc` | クレデンシャルが JSON-escape された形式（例：改行 → `\n` リテラル） |
| `masks_b64` | base64 変換出力が生のまま含まれる |
| `masks_pct_raw` | 生クレデンシャルの percent-encode 形式（例：`ab%7Ede`） |
| `masks_pct_b64` | base64 出力の percent-encode 形式（`YWJ%2BZGU%3D`）— 45382b3 で修正したすり抜け |

## Part 2 — 全称定理（帰納法）

点での検査を、健全な範囲で一般保証へ昇格させた `∀` 命題です。

| 定理 | 内容 |
|---|---|
| `percent_roundtrip` | 全ての低バイト文字列 `s` について `percentDecodeAll (percentEncodeAll s) = s` |
| `infix_replaceAll_false` | `replaceAll` は、置換文字列と文字集合が互いに素な非空 needle の出現を全て除去する |
| `masks_cred_universal` | **任意の** `uri`・`b64cred`・`response` について、テンプレートと文字集合が互いに素な非空クレデンシャルはパイプライン出力に生のまま現れない |

`masks_cred_universal` は `masks_raw` より厳密に強い定理です（1つの固定ベクタではなく
4つの入力すべてを全称化）。`maskCredentials`（`replaceAll cred (key-rest://uri) …`）が
`maskPercentEncoded` の両分岐で **最も外側** の操作であるため、パイプライン全体が
吸収補題を継承します。

全定理は `sorry`／`admit`／追加公理なしで証明されています。各定理末尾で `#print axioms`
を実行しており、依存は `propext`・`Classical.choice`・`Quot.sound`（Lean カーネルの
標準公理）のみです。

## 全称化が炙り出した発見

`percent_roundtrip` は無条件では成立しません。Go のテストサーバーは **バイト** 単位で
percent-encode しますが、本モデルは **コードポイント**（`Char.toNat`）単位です。
`toNat ≥ 256` の文字では nibble 補助関数が `0..15` の16進範囲を溢れ
（`Char.ofNat 256` は `"%G0"` になり復号できない）、往復は低バイト側条件
（全文字 `< 256`）の下でのみ成立します。これは全ての ASCII/Latin-1 クレデンシャル
（実際の API キー）で成立します。proxy のバグではなく、モデルと Go の差異として
記録すべき事項です。

## 証明していないこと（射程）

これらの証明は **マスキング層のみ**（レスポンス走査）を対象とします。key-rest の
主要防御は **配置検証**（クレデンシャルの送信先に対する既定拒否）であり、マスキングは
それを補う二次的なブロックリストの網にすぎません。したがってマスキングを完全に全称化
しても「クレデンシャルが漏れない」を意味しません。

以下はソース内に限界として明記し、証明していません（偽、あるいは射程外のため）：

- **`∀ cred` は偽。** 置換テンプレート（`key`・`rest`・`://`・uri 由来テキスト）の
  部分列に等しいクレデンシャルはマスキングを生き残ります。`Disj` 側条件はこれらを
  構成的に排除します。
- **`∀ encoding` は機構由来で偽。** マスキングは raw／json-esc／base64／percent／
  percent+base64／truncated の列挙です。列挙外はすり抜けます：base64url・hex・ROT-N・
  **二重** percent-encode（proxy は1層しか復号しない）・文字間スペース挿入・1文字ずつ
  綴る・ストリーム分割。ブロックリストは原理的に不完全です。
- **自由文 LLM レスポンス本文は封じ込め不可能** — 全文字種・無制限長で、
  アローリストもブロックリストも効きません。

## `infix_replaceAll_false` に組み込まれた設計判断

単一パスの `replaceAll` は、挿入した置換文字列と後続テキストの **継ぎ目を再走査しない**
ため、`∀ cred body. isInfix cred (replaceAll cred repl body) = false` は一般には **偽**
です（例：`replaceAll "ab" "a" "abb" = "ab"`）。そのため全称定理は `Disj`（文字集合が
互いに素）側条件を持ちます — これは十分条件であり、真に必要な「repl のどの接尾辞も
cred の接頭辞にならない」より強い条件です。これが、`masks_cred_universal` が文字集合の
互いに素なクレデンシャルを対象とし、現実的な `"ab~de"`（`key-rest://` と `'e'` を共有）
は Part 1 の点アンカーに留まる理由です。現 `replaceAll` ＋側条件を維持するか、
再走査版 `replaceAll`（`cred ∉ repl` のみから証明可能だが停止性／網羅性の証明が重い）に
切り替えるかは未決の設計選択です。

## 前提条件

- [Lean 4](https://lean-lang.org/) — `lean-toolchain` に記載のバージョン（4.30.0）

[elan](https://github.com/leanprover/elan) でインストール：

```sh
curl https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh -sSf | sh
```

## 検証方法

```sh
cd lean
lean MaskingPipeline.lean
```

`#print axioms` 行は（標準）公理依存を表示します。実エラーがあれば標準エラーに表示
されます。これらの行のみで正常終了すれば全定理が検証済みです。

Lake（Lean ビルドシステム）を使う場合：

```sh
cd lean
lake build
```
