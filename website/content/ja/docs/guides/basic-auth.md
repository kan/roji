---
title: "BASIC認証"
slug: "basic-auth"
description: "ユーザー名とパスワードでルートを保護。"
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

rojiはローカル開発環境でルートを保護するためのHTTP BASIC認証をサポートしています。

## Dockerラベルで設定

Dockerベースのルートに認証を追加：

```yaml
services:
  admin:
    image: my-admin-panel
    labels:
      - "roji.auth.basic.user=admin"
      - "roji.auth.basic.pass=secret"
      - "roji.auth.basic.realm=Admin Area"   # 任意、デフォルトは "Restricted"
    networks:
      - roji
```

## 設定ファイルで設定

静的サイトを `~/.config/roji/config.yaml` で保護：

```yaml
static_sites:
  - host: docs
    root: ~/projects/docs/build
    auth:
      basic:
        user: admin
        pass: secret
        realm: Documentation   # 任意
```

## 動作

- 認証情報が不足または不正な場合、`WWW-Authenticate` ヘッダー付きで `401 Unauthorized` を返す
- CORSプリフライトリクエスト（`OPTIONS`）は認証をバイパス
- ダッシュボードは保護されたルートにロックバッジを表示
- パスワードは平文で保存（ローカル開発用途のため）
