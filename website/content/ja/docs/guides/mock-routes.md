---
title: "モックルート"
slug: "mock-routes"
description: "フロントエンド開発向けのモックAPIレスポンス定義。"
weight: 4
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

rojiは実際のバックエンドなしでモックレスポンスを返すことができます。バックエンドAPIが未完成の段階でのフロントエンド開発に便利です。

## 仕組み

Dockerラベルを使ってコンテナにモックレスポンスを定義します。コンテナはWebサーバーを実行する必要はなく、rojiネットワークに接続されていれば動作します。

## ラベル構文

| ラベル | 説明 |
|--------|------|
| `roji.mock.{METHOD}.{PATH}` | レスポンスボディ（JSON文字列） |
| `roji.mock.status.{METHOD}.{PATH}` | HTTPステータスコード（デフォルト: `200`） |

## 例: モックAPI

```yaml
services:
  mock-api:
    image: alpine
    command: ["sleep", "infinity"]
    labels:
      - "roji.host=api.dev.localhost"
      - 'roji.mock.GET./api/users=[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]'
      - 'roji.mock.GET./api/health={"status":"ok"}'
      - "roji.mock.status.POST./api/users=201"
    networks:
      - roji

networks:
  roji:
    external: true
```

作成されるエンドポイント：

- `GET https://api.dev.localhost/api/users` → `200` でユーザーリストJSON
- `GET https://api.dev.localhost/api/health` → `200` でヘルスステータス
- `POST https://api.dev.localhost/api/users` → `201`（空ボディ）

## ユースケース

- **フロントエンド開発** — バックエンドチームがAPIを構築中にUIを開発
- **APIプロトタイピング** — 統合テスト用のエンドポイントを素早く定義
- **エラーテスト** — エラーハンドリングのテスト用にモックエラーレスポンス

```yaml
labels:
  - "roji.mock.GET./api/error={\"error\":\"Internal Server Error\"}"
  - "roji.mock.status.GET./api/error=500"
```
