---
title: "複数ネットワーク"
slug: "multi-network"
description: "複数のDockerネットワークを同時に監視。"
weight: 5
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

rojiは複数のDockerネットワークを同時に監視でき、サービスを別々のネットワークグループに整理できます。

## 設定

カンマ区切りで複数ネットワークを指定：

### 設定ファイル

```yaml
# ~/.config/roji/config.yaml
network: web,api,internal
```

### 環境変数

```bash
ROJI_NETWORK=web,api,internal
```

### CLIフラグ

```bash
sudo roji --network web,api,internal
```

## ネットワークの作成

事前にネットワークを作成：

```bash
docker network create web
docker network create api
```

または `roji doctor --fix` で不足しているネットワークを自動作成できます。

## 設定例

```yaml
# frontend/docker-compose.yml
services:
  app:
    image: nginx:alpine
    networks:
      - web

networks:
  web:
    external: true
```

```yaml
# backend/docker-compose.yml
services:
  api:
    image: my-api
    networks:
      - api

networks:
  api:
    external: true
```

両方のサービスがrojiを通じてアクセス可能：

- `https://app.dev.localhost`（webネットワーク）
- `https://api.dev.localhost`（apiネットワーク）

## ダッシュボード

ダッシュボードは各ルートにネットワークバッジを表示し、コンテナが所属するネットワークを確認できます。
