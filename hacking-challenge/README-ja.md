[English](README.md) | [Japanese](README-ja.md) | [key-rest に戻る](../README-ja.md)

# 🏆 key-rest ハッキングチャレンジ

## ルール
- 参加者は、koteitan/key-rest の issue に agent が credentials を奪取する方法を書く。必要な情報は下記の通り:
  - issue に hacking-challenge のラベルを付ける
  - 報奨支払い先の BTC LN アドレス
  - credentials を奪取する方法の説明
- 対象となるサーバーは、koteitan のローカルマシン ika 上の mock REST server。
- agent すしめいじ🍣(sushi-mage) は、投稿時刻が早かった順に、koteitan のローカルマシン ika 上で credentials を奪取する方法を検証する。
- sushi-mage が credentials を奪取できた場合、プログラムは修正される。
- 修正がなされた後、次に投稿が早かった issue の方法が検証される。
- credentials を実際に奪取できた issue に書かれた BTC LN アドレスに報奨が支払われる。
  - 1位: 10,000 sats 🥇
  - 2位:  8,000 sats 🥈
  - 3位:  6,000 sats 🥉
  - 4位:  4,000 sats
  - 5位:  2,000 sats

## 細かいルール
  - sushi-mage は super user 権限、sudo 権限を持たないと想定する。
  - 検証に使う mock REST server は、検証時には client host 上で動いているが、それを想定した攻撃は無効とし、別のホストにあることを想定した攻撃のみ有効とする。
  - 検証が法的・倫理的に問題があると判断された場合は、その issue は検証されない。
  - 異なる参加者が同着だった場合は、もっとも低い順位の報奨が支払われる。
  - 下記の場合は、順位の低い方の issue は報酬なしとして、下位の順位の方法が繰り上がる。
    - 同一の参加者が複数の issue にて入賞した場合
    - 同一の human に作られた LLM の参加者が issue にて入賞した場合
  - 完全に騙りを防ぐことはできませんが、異なる human を騙るのは一応やめてください。
  - 実際の攻撃には、[issue9](https://github.com/koteitan/key-rest/issues/9) のようなサーバーの挙動に依存するものも存在すると思うが、今回はあくまで mock REST server の動作に対して有効な issue だけを対象とする。
  - 有効期限はとりあえず 2026/4/15 とします。検証は issue が来たらどんどんやっていきます。

## 登場人物の定義
### ユーザー
- superuser: 人間のユーザー。
- agent: LLM agent の攻撃者。
  - superuser に雇われており、REST server にアクセスして仕事をしている。

### ホスト
- client host: agent がアクセスするホスト。
  - agent は client host の user の権限を持つ。
  - super user は client host の super user の権限を持つ。

### サーバー/クライアントアプリ
- REST server: REST API を提供するサーバー。
- mock REST server: REST server のモック。実際の REST server と同じ API を提供するが、実装は異なる。
  - super user が mock REST server を立てる。
- key-rest-daemon: REST server にアクセスするためのクライアントアプリ。
  - super user が key-rest start によって起動する。
  - 起動時に入力された master key によって credentials を復号化し、メモリ上に展開する。
  - super user が key-rest add コマンドを使用して credentials を追加する。
  - 追加された credentials は master key によって暗号化され、client host に保存される。
- key-rest-clients: key-rest を使用して REST server にアクセスするクライアントライブラリ。
  - agent は key-rest-clients を使用して REST server にアクセスする。
  - クライアントは go, python, nodejs, curl などがある。

- credentials: REST server にアクセスするための認証情報。
  - super user が知っている。
  - REST server に保存されている。
  - key-rest によって client host に暗号化して保存される。
- master key: credentials を暗号化するためのキー。
  - super user が知っている。


