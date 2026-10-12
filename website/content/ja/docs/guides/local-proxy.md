---
title: "ホスト上の開発サーバー"
slug: "local-proxy"
description: "Docker の外で動かしているサーバーにホスト名を付ける。"
weight: 2
date: 2026-10-12T00:00:00+00:00
lastmod: 2026-10-12T00:00:00+00:00
draft: false
toc: true
---

roji はコンテナを自動で検出する。一方、ホスト上で直接起動した開発サーバー
（`vite dev`、`wrangler dev`、`rails s` など）はコンテナではないので検出
されない。設定ファイルの `proxy_sites` に書くと、こうしたサーバーにも
コンテナと同じようにホスト名と HTTPS が付く。

## 設定

`~/.config/roji/config.yaml` にエントリを追加する。

```yaml
proxy_sites:
  - host: app                       # -> app.dev.localhost
    target: http://localhost:5173   # 開発サーバーの待ち受け先
  - host: api
    target: http://127.0.0.1:8787
```

開発サーバーをいつもどおり起動し、`https://app.dev.localhost` を開く。

### ホスト名

`host` の規則は[静的サイト](/ja/docs/guides/static-sites/)と同じ。

- ドットを含まない名前は `{host}.{domain}` に展開する（`app` → `app.dev.localhost`）
- ドットを含む名前は書いたとおりに使う

ホスト名はベースドメインの 1 階層下に収める。roji の証明書が対応するのは
`*.{domain}` までで、`a.b.dev.localhost` のような 2 階層の名前は、別ドメインの
名前と同じくブラウザに拒否される。外れているホスト名は `roji doctor` が報告する。

ダッシュボードのホスト名（既定では `roji.dev.localhost`）とベースドメイン
そのものは使えない。ダッシュボードが先に応答するので、そこに宣言したサイトは
警告を出して登録しない。

### target

`target` はスキームまで含めた URL で書く。スキームは `http` のみ。TLS は
roji が終端する。

- `target: 8787` や `target: 127.0.0.1:8787` は受け付けない。`port:` の
  ような略記も無い。1 つのサイトの書き方を 1 通りにするため
- target にパスを書くと、すべてのリクエストの先頭に付く。
  `target: http://localhost:3000/base` なら `/items` は `/base/items` に届く
- サーバーがどのアドレスで待ち受けるか分からないときは
  `http://localhost:ポート` と書く。ツールや Node.js の版によって
  「localhost」は `127.0.0.1` にも `::1` にもなり、roji は両方を試す
- 別のマシンを指す target も使えるが、バリデーションが警告を出す。その
  リクエストは暗号化されずにこのマシンの外へ出る

### 開発サーバーが受け取るもの

コンテナの場合と同じ。

- `Host` ヘッダーは、ブラウザが要求したホスト名（`app.dev.localhost`）の
  まま渡る。target のホスト名にはならない。Vite の `server.allowedHosts` の
  ようにホスト名を検査する開発サーバーには、この名前が見える。Vite は
  `.localhost` の名前を既定で許可している
- `X-Forwarded-Proto: https`、`X-Forwarded-For`、`X-Real-IP`
- WebSocket（Vite の HMR）、Server-Sent Events、gRPC はそのまま通る

## サーバーが起動していないとき

開発サーバーをまだ起動していない状態はふつうにあるので、roji は target に
接続できるかを確かめずにサイトを登録する。その状態でアクセスすると、
target のアドレスと、起動すべきものを案内する `502` のページが返る。
Compose プロジェクトと違い、roji はそのプロセスのことを何も知らないので、
roji からは起動できない。

`roji doctor` は、待ち受けているプロセスが無い proxy サイトを警告として
一覧する。

## Docker ルートとの優先順位

proxy サイトは、静的サイトと同じくホスト名を保持する。同じホスト名に
コンテナがルーティングされている場合は proxy サイトが応答し、どの Docker
ルートが隠れたかを roji がログに出す。設定ファイルの中で同じホスト名を
2 回宣言した場合（`proxy_sites` 同士、`static_sites` 同士、両方に 1 つずつの
いずれでも）、後に書いた宣言が有効になる。`proxy_sites` は `static_sites` の
後に読まれる。

proxy サイトはホスト名全体を受け持つ。パスプレフィックスは指定できない。

## 変更の適用

再起動は不要。ダッシュボードの **Reload Config** ボタンをクリックするか、
ボタンが呼んでいる API を直接呼ぶ。

```bash
curl -X POST https://roji.dev.localhost/_api/config/reload
```

roji を動かしたまま target のポートを変えたいときも、この手順を使う。

## 認証

```yaml
proxy_sites:
  - host: api
    target: http://127.0.0.1:8787
    auth:
      basic:
        user: admin
        pass: secret
        realm: API   # 任意
```

詳細は[認証のガイド](/ja/docs/guides/basic-auth/)を参照。

## トンネルで外部公開する

開発サーバーで webhook を受けるのは、[トンネル](/ja/docs/guides/tunnel/)の
主な用途の 1 つ。`tunnel: true` でサイトを公開対象にする。

```yaml
proxy_sites:
  - host: hook
    target: http://127.0.0.1:8787
    tunnel: true        # https://hook.example.com で到達できる
```

公開用の名前は、ローカルの名前のうちベースドメインの部分をトンネルの
ドメインへ入れ替えて作る。そのため、公開できるのはベースドメインの下のホスト名だけ。

このキーが無いサイトは非公開のまま。開発サーバーが起動していないときに
トンネル経由で来たリクエストには、上の案内ページではなく、トンネルの
ほかの拒否と同じ素の 404 を返す。
