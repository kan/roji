---
title: "Docker Composeパターン"
slug: "docker-compose-patterns"
description: "rojiでよく使うDocker Compose構成パターン。"
weight: 6
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

rojiでよく使われるDocker Composeの構成パターンを紹介します。

## フロントエンド + API + データベース

フロントエンド、API、データベースを持つ典型的なフルスタックアプリケーション。フロントエンドとAPIのみrojiネットワークに接続し、データベースは内部ネットワークに留めます。

```yaml
services:
  frontend:
    image: node:alpine
    working_dir: /app
    command: ["npm", "run", "dev"]
    volumes:
      - ./frontend:/app
    expose:
      - "3000"
    networks:
      - roji
      - internal

  api:
    image: node:alpine
    working_dir: /app
    command: ["npm", "start"]
    volumes:
      - ./api:/app
    expose:
      - "4000"
    environment:
      - DATABASE_URL=postgres://postgres:password@db:5432/myapp
    networks:
      - roji
      - internal

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_DB: myapp
    volumes:
      - pgdata:/var/lib/postgresql/data
    networks:
      - internal    # rojiに接続しない — 外部アクセス不要

volumes:
  pgdata:

networks:
  roji:
    external: true
  internal:
    driver: bridge
```

アクセス先：
- `https://frontend.dev.localhost` — React/Vue開発サーバー
- `https://api.dev.localhost` — APIサーバー

## パスベースルーティング（単一ドメイン）

同じホスト名でフロントエンドとAPIを提供：

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

networks:
  roji:
    external: true
```

アクセス先：
- `https://myapp.dev.localhost` — フロントエンド
- `https://myapp.dev.localhost/api/*` — API

## マイクロサービス

複数サービスがそれぞれ独自のサブドメインを取得：

```yaml
services:
  auth:
    image: auth-service
    expose: ["8080"]
    networks: [roji]

  users:
    image: users-service
    expose: ["8080"]
    networks: [roji]

  orders:
    image: orders-service
    expose: ["8080"]
    networks: [roji]

  gateway:
    image: api-gateway
    expose: ["8080"]
    labels:
      - "roji.host=api.dev.localhost"
    networks: [roji]

networks:
  roji:
    external: true
```

アクセス先：
- `https://auth.dev.localhost`
- `https://users.dev.localhost`
- `https://orders.dev.localhost`
- `https://api.dev.localhost` — APIゲートウェイ

## WordPress

```yaml
services:
  wordpress:
    image: wordpress:latest
    expose:
      - "80"
    labels:
      - "roji.host=blog.dev.localhost"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: wordpress
      WORDPRESS_DB_NAME: wordpress
    volumes:
      - wp_data:/var/www/html
    networks:
      - roji
      - internal

  db:
    image: mysql:8
    environment:
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: wordpress
      MYSQL_ROOT_PASSWORD: rootpassword
    volumes:
      - db_data:/var/lib/mysql
    networks:
      - internal

volumes:
  wp_data:
  db_data:

networks:
  roji:
    external: true
  internal:
    driver: bridge
```

アクセス先: `https://blog.dev.localhost`

## WebSocketアプリケーション

`Upgrade: websocket` ヘッダーが検出されると、WebSocket接続は自動的にプロキシされます：

```yaml
services:
  chat:
    image: my-chat-app
    expose:
      - "8080"
    networks:
      - roji

networks:
  roji:
    external: true
```

`wss://chat.dev.localhost` で接続 — 追加設定は不要です。

## gRPCサービス

`Content-Type: application/grpc` が検出されると、gRPCサービスはHTTP/2でプロキシされます：

```yaml
services:
  grpc-api:
    image: my-grpc-service
    expose:
      - "50051"
    labels:
      - "roji.port=50051"
    networks:
      - roji

networks:
  roji:
    external: true
```

gRPCクライアントで `grpc-api.dev.localhost:443` に接続します。
