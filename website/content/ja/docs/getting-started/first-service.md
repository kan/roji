---
title: "最初のサービス"
slug: "first-service"
description: "rojiでマルチサービスプロジェクトをゼロから構築。"
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

このチュートリアルでは、マルチサービスプロジェクトをゼロから作成し、rojiを通じてアクセスする方法を説明します。

## 新しいプロジェクトを作成

Webフロントエンドとバックエンドの2つのサービスからなるプロジェクトを作成します：

```bash
mkdir my-project && cd my-project
```

`docker-compose.yml` を作成：

```yaml
services:
  web:
    image: nginx:alpine
    expose:
      - "80"
    networks:
      - roji

  api:
    image: node:alpine
    working_dir: /app
    command: ["node", "server.js"]
    expose:
      - "3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

## 起動とアクセス

```bash
docker compose up -d
```

サービスが利用可能になります：

- `https://web.dev.localhost` — nginxフロントエンド
- `https://api.dev.localhost` — Node.js API

## ラベルでカスタマイズ

### カスタムホスト名

デフォルトの `{service}.dev.localhost` の代わりにカスタムホスト名を設定：

```yaml
services:
  api:
    image: my-api
    labels:
      - "roji.host=api.dev.localhost"
    networks:
      - roji
```

### パスベースルーティング

パスプレフィックスを使って同じホスト名で複数サービスをルーティング：

```yaml
services:
  frontend:
    image: nginx:alpine
    labels:
      - "roji.host=myapp.dev.localhost"
    networks:
      - roji

  api:
    image: my-api
    labels:
      - "roji.host=myapp.dev.localhost"
      - "roji.path=/api"
    networks:
      - roji
```

`https://myapp.dev.localhost` がフロントエンド、`https://myapp.dev.localhost/api/*` がAPIにルーティングされます。

### 特定ポート

コンテナが複数ポートを公開している場合、rojiがプロキシするポートを指定：

```yaml
services:
  app:
    expose:
      - "3000"
      - "9229"   # デバッガー
    labels:
      - "roji.port=3000"
    networks:
      - roji
```

## ダッシュボードを確認

`https://roji.dev.localhost` を開くと、すべてのルート、リクエストログ、プロジェクト操作が確認できます。

## 次のステップ

- [設定](/ja/docs/guides/configuration/) — 設定ファイルと環境変数
- [Docker Composeパターン](/ja/docs/guides/docker-compose-patterns/) — よくあるプロジェクト構成
- [モックルート](/ja/docs/guides/mock-routes/) — モックAPIレスポンスの定義
