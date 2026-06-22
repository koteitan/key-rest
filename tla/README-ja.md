[← Back](../README-ja.md) | [English](README.md) | [Japanese](README-ja.md)

# TLA+ 形式仕様

このディレクトリには、key-rest のクレデンシャルマスキングパイプラインに対する
TLA+ 仕様と TLC モデル検査設定が含まれています。

## 何を検証しているか

`MaskingPipeline.tla` はマスキングパイプラインを状態機械としてモデル化し、
以下の7つのサーバーエンコーディング戦略に対して `NoCredentialLeak` が
成立することを検査します：

| エンコーディング | 内容 | キャッチするステップ |
|---|---|---|
| `none` | クレデンシャルがレスポンスに含まれない | — |
| `raw` | 生のクレデンシャルバイト列 | Step 2: `maskCredentials` |
| `json_esc` | JSON-escape された形式（例：`"` → `\"`） | Step 2: `maskCredentials`（JSON形式） |
| `b64` | base64 変換出力 | Step 1: `maskTransformOutputs` |
| `pct_raw` | percent-encode された生クレデンシャル | Step 4: URL-decode → `maskCredentials` |
| `pct_b64` | percent-encode された base64 出力 | Step 4: URL-decode → `maskTransformOutputs` |
| `trunc` | 短縮キーパターン（例：`sk-****abcd`） | Step 3: `maskTruncatedKeys` |

## 前提条件

- Java ランタイム（JRE 11 以上）
- [TLA+ ツール](https://github.com/tlaplus/tlaplus/releases) — `tla2tools.jar`

`tla2tools.jar` をリポジトリルート（このディレクトリの一つ上）に配置してください。

## 検証方法

```sh
java -jar ../tla2tools.jar -config tla/MaskingPipeline.cfg tla/MaskingPipeline.tla
```

正常終了時の出力末尾：

```
Model checking completed. No error has been found.
```
