[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# Lean 形式証明

このディレクトリには、key-rest のクレデンシャルマスキングパイプラインに対する
Lean 4 カーネル検証済み証明が含まれています。

## 何を検証しているか

`MaskingPipeline.lean` は、修正済みパイプライン（コミット 45382b3）が
以下の5つのサーバーエンコーディングシナリオに対してクレデンシャルを
安全にマスクすることを証明します：

| 定理 | シナリオ |
|---|---|
| `masks_raw` | レスポンスにクレデンシャルが生のまま含まれる |
| `masks_json_esc` | クレデンシャルが JSON-escape された形式で含まれる（例：改行 → `\n` リテラル） |
| `masks_b64` | base64 変換出力が生のまま含まれる |
| `masks_pct_raw` | 生クレデンシャルが percent-encode された形式（例：`ab%7Ede`） |
| `masks_pct_b64` | base64 出力が percent-encode された形式（例：`YWJ%2BZGU%3D`） |

全定理は `decide` タクティクで証明されています。これは Lean カーネル自体が
計算を簡約して結果を確認するため、カーネル以外の公理や手動の証明ステップは
一切不要です。

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

出力なしで終了すれば全定理が検証済みです。エラーがある場合は標準エラーに表示されます。

Lake（Lean ビルドシステム）を使う場合：

```sh
cd lean
lake build
```
