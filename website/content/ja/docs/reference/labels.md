---
title: "Dockerラベル"
slug: "labels"
description: "コンテナ設定用のrojiDockerラベル一覧。"
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## ラベルリファレンス

| ラベル | 説明 | デフォルト |
|--------|------|-----------|
| `roji.host` | カスタムホスト名 | `{service}.dev.localhost` |
| `roji.port` | ターゲットポート | 最初のEXPOSEポート |
| `roji.path` | パスプレフィックス | なし |
| `roji.mock.{METHOD}.{PATH}` | モックレスポンスボディ | なし |
| `roji.mock.status.{METHOD}.{PATH}` | モックレスポンスステータスコード | `200` |
| `roji.auth.basic.user` | BASIC認証ユーザー名 | なし |
| `roji.auth.basic.pass` | BASIC認証パスワード | なし |
| `roji.auth.basic.realm` | BASIC認証レルム | `Restricted` |
| `roji.self` | 予約済み: コンテナをルーティングから除外（内部使用） | なし |

## `roji.host`

デフォルトのホスト名（`{service}.dev.localhost`）を上書き：

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
```

任意のホスト名を使用できます。ドットを含まない場合は設定済みドメインで展開されます。

## `roji.port`

コンテナが複数ポートを公開している場合、プロキシ先のポートを指定：

```yaml
expose:
  - "3000"
  - "9229"   # デバッガー
labels:
  - "roji.port=3000"
```

未設定の場合、最初の公開ポートが使用されます。

## `roji.path`

パスプレフィックスに一致するリクエストをこのサービスにルーティング：

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
  - "roji.path=/api"
```

`https://myapp.dev.localhost/api/*` へのリクエストがこのサービスにルーティングされます。

プレフィックスはパスのセグメント境界でのみ一致します。`/api` は `/api` と
`/api/users` を捕まえますが、`/apifoo` や `/api-docs` は捕まえず、プレフィックス
なしでそのホスト名を処理するルートへ流れます。

`roji.path=/` はラベルを書かないのと同じ意味になります。プレフィックスではなく
ホスト名全体をそのサービスが受け持ちます。

## `roji.mock.*`

実際のバックエンドなしでモックレスポンスを定義：

```yaml
labels:
  - "roji.host=api.dev.localhost"
  - 'roji.mock.GET./api/users=[{"id":1,"name":"Alice"}]'
  - 'roji.mock.GET./api/health={"status":"ok"}'
  - "roji.mock.status.POST./api/users=201"
```

詳細は[モックルート](/ja/docs/guides/mock-routes/)ガイドを参照。

## `roji.auth.basic.*`

HTTP BASIC認証でルートを保護：

```yaml
labels:
  - "roji.auth.basic.user=admin"
  - "roji.auth.basic.pass=secret"
  - "roji.auth.basic.realm=Admin Area"   # 任意
```

詳細は[BASIC認証](/ja/docs/guides/basic-auth/)ガイドを参照。

## `roji.self`

予約済みの内部ラベル。コンテナをルーティングから除外するためにマークします。Docker モードでroji自身が使用します。独自のコンテナには設定しないでください。
