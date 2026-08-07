---
title: "roji"
description: "ローカル開発環境向けのシンプルなリバースプロキシ。Dockerサービスを自動検出し、*.dev.localhostでHTTPSアクセスを提供。"
lead: "本番は高速道路、開発は路地裏で。"
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
params:
  seo:
    title: "roji — ローカル開発向けシンプルリバースプロキシ"
    description: "Docker Composeサービスを自動検出し、*.dev.localhostでHTTPSアクセスを提供。設定不要。"
---

## 秒速インストール

```bash
# macOS
brew install kan/roji/roji

# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.1.0/install.sh | bash
```

## 試してみる

```yaml
# docker-compose.yml
services:
  myapp:
    image: your-app
    expose:
      - "3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

```bash
docker compose up -d
# https://myapp.dev.localhost を開く
```
