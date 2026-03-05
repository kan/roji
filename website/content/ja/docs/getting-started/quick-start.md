---
title: "クイックスタート"
slug: "quick-start"
description: "5分で最初のサービスを起動。"
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## 前提条件

- **Docker** と Docker Compose v2
- **roji** がインストール済みで起動中（[インストール](/ja/docs/getting-started/installation/)を参照）

## セットアップ確認

```bash
roji doctor
```

すべてのチェックがパスするはずです。問題がある場合は `sudo roji doctor --fix` で自動修復できます。

## 既存プロジェクトにrojiを追加

既存の `docker-compose.yml` に `roji` ネットワークを追加します：

```yaml
services:
  myapp:
    image: your-app
    expose:
      - "3000"
    networks:
      - roji      # これを追加

networks:
  roji:           # これを追加
    external: true
```

プロジェクトを起動：

```bash
docker compose up -d
```

`https://myapp.dev.localhost` を開く — それだけです！

## ダッシュボードを確認

`https://roji.dev.localhost` を開くと、ライブダッシュボードが表示されます：

- 登録済みの全ルート
- リアルタイムリクエストログ
- Docker Composeプロジェクト操作（起動/停止/再起動）
- プロジェクト履歴とクイックスタートボタン

ダッシュボードはServer-Sent Eventsによりページ更新なしで自動更新されます。

## よくあるパターン

**カスタムホスト名：**

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
```

**パスベースルーティング**（1つのホストに複数サービス）：

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
  - "roji.path=/api"
```

**特定ポート**（複数ポートが公開されている場合）：

```yaml
labels:
  - "roji.port=3000"
```

**複数プロジェクト** — 各プロジェクトを `roji` ネットワークに接続するだけ。各サービスが自動的に `*.dev.localhost` サブドメインを取得します。

## 次のステップ

- [最初のサービス](/ja/docs/getting-started/first-service/) — マルチサービスプロジェクトをゼロから構築
- [Dockerラベル](/ja/docs/reference/labels/) — すべてのラベルオプション
- [設定](/ja/docs/guides/configuration/) — 設定ファイルと環境変数
