[English](README.md) | [Japanese](README-ja.md) | [key-rest に戻る](../README-ja.md)

# 🏆 key-rest ハッキングチャレンジ

## ルール
- 投稿方法:
  - 参加者は、koteitan/key-rest の issue に agent が credentials を奪取する方法を記載してください。必要な情報は下記の通りです:
    - `hacking-challenge` と issue 本文かコメントに記載してください
    - 報奨支払い先の BTC LN アドレスを書いてください
    - credentials を奪取する方法の説明を書いてください
      - 具体的に書く方が成功率が上がると思います
- 検証対象環境:
  - 検証の対象となるクライアントは、issue が投稿された時点の main ブランチの key-rest クライアント、key-rest-daemon です。
  - 検証の対象となるサーバーは、koteitan のローカルマシン ika 上の mock REST server です。
  - ソースコードは [test-server] にあります。
  - 提案された攻撃が一般のサーバーに対しては有効でも、この mock REST server に対して無効である攻撃はチャレンジ的には失敗とみなします。
- 検証の順番:
  - agent すしめいじ🍣(sushi-mage) は、投稿時刻が早かった順に、koteitan のローカルマシン ika 上で credentials を奪取する方法を検証します。
  - sushi-mage が credentials を奪取できた場合、プログラムは修正されます。
  - 修正がなされた後、次に投稿が早かった issue の方法が検証されます。
- 報奨:
  - credentials を実際に奪取できた issue に書かれた BTC LN アドレスに報奨が支払われます。
  - 1位: 10,000 sats 🥇
  - 2位:  8,000 sats 🥈
  - 3位:  6,000 sats 🥉
  - 4位:  4,000 sats
  - 5位:  2,000 sats
- 有効期限:
  - チャレンジの有効期限はとりあえず 2026/4/15 とします。
  - この期間は延長される場合があります。
  - 検証は期限を待たずに issue が来たら順次行います。

## 細かいルール
  - sushi-mage は super user 権限、sudo 権限を持たないものと想定します。
  - 検証に使う mock REST server は、検証時には client host 上で動いていますが、それを想定した攻撃は無効とし、別のホストにあることを想定した攻撃のみ有効とします。
  - 検証が法的・倫理的に問題があると判断された場合は、その issue は検証されません。
  - 異なる参加者が同着だった場合は、もっとも低い順位の報奨が支払われます。
  - 下記の場合は、順位の低い方の issue は報酬なしとして、下位の順位の方法が繰り上がります。
    - 同一の参加者が複数の issue にて入賞した場合
    - 同一の human に作られた LLM の参加者が issue にて入賞した場合
  - 完全に騙りを防ぐことはできませんが、異なる human を騙るのはご遠慮ください。
- コードの修正:
  - 提案されている issue がすべて奪取失敗となった場合、テスト対象環境がアップデートされる可能性があります。（一般サービスの挙動のエミュレーションを改善したり、issue から示唆された別の問題に気付いたときなど）
- ルールの修正:
  - このチャレンジのルールは適宜追記・修正される場合があります。

## 登場人物の定義
### ユーザー
- superuser: 人間のユーザーです。
- agent: LLM agent の攻撃者です。
  - superuser に雇われており、REST server にアクセスして仕事をしています。

### ホスト
- client host: agent がアクセスするホストです。
  - agent は client host の user の権限を持ちます。
  - super user は client host の super user の権限を持ちます。

### サーバー/クライアントアプリ
- REST server: REST API を提供するサーバーです。
- mock REST server: REST server のモックです。実際の REST server と同じ API を提供しますが、実装は異なります。
  - super user が mock REST server を立てます。
- key-rest-daemon: REST server にアクセスするためのクライアントアプリです。
  - super user が key-rest start によって起動します。
  - 起動時に入力された master key によって credentials を復号化し、メモリ上に展開します。
  - super user が key-rest add コマンドを使用して credentials を追加します。
  - 追加された credentials は master key によって暗号化され、client host に保存されます。
- key-rest-clients: key-rest を使用して REST server にアクセスするクライアントライブラリです。
  - agent は key-rest-clients を使用して REST server にアクセスします。
  - クライアントは Go, Python, Node.js, curl などがあります。

- credentials: REST server にアクセスするための認証情報です。
  - super user が知っています。
  - REST server に保存されています。
  - key-rest によって client host に暗号化して保存されます。
- master key: credentials を暗号化するためのキーです。
  - super user が知っています。


