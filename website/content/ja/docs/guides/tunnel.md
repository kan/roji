---
title: "外部公開（トンネル）"
slug: "tunnel"
description: "Cloudflare Tunnel を通して、選んだルートだけを外部から到達可能にする。"
weight: 7
date: 2026-08-14T00:00:00+00:00
lastmod: 2026-08-14T00:00:00+00:00
draft: false
toc: true
---

roji は既定ではループバックアドレスでしか待ち受けない（[設定ガイドの
`bind`](/ja/docs/guides/configuration/) を参照）。外から開発機に届かせたい
場面（webhook の受信、作りかけのアプリを人に見せる）のために、Cloudflare
Tunnel を通して**選んだルートだけ**を公開できる。

`bind` を広げるのとは違い、公開の単位はコンテナごとで、ダッシュボードと
その API は公開されない。

## 必要なもの

- Cloudflare アカウントと、そこに登録済みのゾーン（`example.com` など）
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)
  が PATH にあること。roji は cloudflared を子プロセスとして実行する

```bash
cloudflared tunnel login          # ブラウザが開き ~/.cloudflared/cert.pem を作る
cloudflared tunnel create roji    # named tunnel を作る
```

## 設定

`~/.config/roji/config.yaml` に `tunnel:` ブロックを足す。

```yaml
tunnel:
  domain: example.com   # Cloudflare 上のゾーン
  name: roji            # cloudflared tunnel create で作った named tunnel
  port: 8080            # トンネル専用リスナー（127.0.0.1 固定、既定 8080）
  auto_start: false     # roji が cloudflared を起動するか
```

`domain` と `name` の両方が揃ったときだけトンネルが有効になる。環境変数は
無く、設定ファイル専用。

`auto_start: true` にすると roji が cloudflared を起動し、roji の終了時に
停止する。`false` のままなら、リスナーだけが開くので `cloudflared tunnel run
roji` を自分で回す。

## 公開するルートを選ぶ

コンテナに `roji.tunnel` ラベルを付ける。付けたものだけが外に出る。

```yaml
# docker-compose.yml
services:
  api:
    labels:
      - "roji.tunnel=true"
```

`true` / `1` / `yes` / `on` が肯定として扱われ、それ以外（ラベルなし、空、
書き間違い）はすべて非公開になる。roji はネットワーク上のコンテナをラベル
無しで拾うので、既定で公開すると全部が外に出てしまう。だから明示的な
オプトインにしてある。

## DNS レコード

`*.{domain}` をトンネルに向ける設定は手動で行う。roji に Cloudflare の API
トークンを持たせない方針のため、この 1 手だけは roji がやらない。

```bash
cloudflared tunnel route dns roji "*.example.com"
```

Cloudflare のダッシュボードから `<uuid>.cfargotunnel.com` への CNAME を
作っても同じ。

### ドメインは 1 階層に収める

Cloudflare の Universal SSL がカバーするのは `example.com` と
`*.example.com` だけ。`tunnel.example.com` のような 2 階層の名前にすると
`*.tunnel.example.com` の証明書が必要になり、有償の Advanced Certificate
Manager が要る。設定バリデーションが警告を出す。

## ホスト名の対応

ローカル名の接尾辞を入れ替えたものが公開名になる。

| ローカル | 公開 |
|---|---|
| `web.dev.localhost` | `web.example.com` |
| `api.dev.localhost` | `api.example.com` |

バックエンドに渡される `Host` はローカル名のまま。開発サーバーの許可
ホスト設定（Vite の `server.allowedHosts` など）は、いつもどおり
`*.dev.localhost` を書いておけばよい。

## 公開されないもの

トンネル経由のリクエストは、通常のリスナーとは別のポートに届く。
cloudflared は 127.0.0.1 から接続してくるので、リクエストの中身では
ローカルのブラウザと区別がつかない。区別できるのは到達したポートだけで、
その分離を使って以下を落としている。

- `/_api/*` と `/_assets/*`。パスだけで無条件に 404 になり、設定では
  開けられない。`/_api/projects/{name}/up` は無認証でコンテナを起動でき、
  `/_api/logs` はリクエストログを読める
- ダッシュボードのホスト名とベースドメイン
- `roji.tunnel` の無い Docker ルート
- 静的サイト（`static_sites:` にオプトインの綴りが無いため、現状すべて非公開）

拒否はどの理由でも同じ素の 404 を返す。理由ごとに文言を分けると、どの
ホスト名が存在するかを外に教えてしまうため。

## 確認する

```bash
roji doctor
```

cloudflared の導入、ログイン状態、named tunnel の存在を確認する。DNS
レコードだけは roji から検証できないので、案内の表示に留まる。

ダッシュボードでは、公開中のルートに 🌐 バッジと公開 URL が出る。ラベルを
付けても `tunnel:` を設定していなければ何も公開されないので、バッジも
出ない。

## 入れていないもの

- `roji tunnel start/stop/status` コマンド。`auto_start: false` のときは
  `cloudflared tunnel run` を自分で回す前提
- 静的サイトの公開
- cloudflared がクラッシュした後の自動再起動。roji はプロセスマネージャでは
  ないので、落ちたことをログに出して放置する。認証に失敗し続けるトンネルが
  黙って再試行を繰り返す方が困るため
- Cloudflare API トークンによる DNS レコードの自動作成
