---
title: "設定ファイル"
slug: "config-file"
description: "config.yamlの完全なリファレンス。"
weight: 4
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## ファイルの場所

- デフォルト: `~/.config/roji/config.yaml`
- XDG Base Directory仕様に準拠
- `--config` フラグで上書き、`roji config path` で確認可能

## 全リファレンス

```yaml
# ~/.config/roji/config.yaml

# 監視するDockerネットワーク（複数の場合はカンマ区切り）
network: roji

# サービスホスト名のベースドメイン
# サービスは {service}.{domain} のホスト名を取得
domain: dev.localhost

# HTTPポート（すべてのリクエストをHTTPSにリダイレクト）
http_port: 80

# HTTPSポート
https_port: 443

# TLS証明書のディレクトリ（CA + サーバー証明書）
certs_dir: ~/.local/share/roji/certs

# データディレクトリ（プロジェクト履歴、ログ）
data_dir: ~/.local/share/roji

# ダッシュボードホスト名
dashboard: roji.dev.localhost

# ログレベル: debug, info, warn, error
log_level: info

# 起動時にTLS証明書を自動生成
auto_cert: true

# 静的ファイルホスティング
static_sites:
  - host: docs                    # 短縮名 -> docs.dev.localhost
    root: ~/projects/docs/build   # 配信するディレクトリ
    index: true                   # ディレクトリ一覧（デフォルト: true）
  - host: private.example.com    # FQDN（ドットを含む）
    root: /var/www/private
    index: false
    auth:                         # BASIC認証
      basic:
        user: admin
        pass: secret
        realm: Private Area       # 任意、デフォルトは "Restricted"
```

## 設定の管理

```bash
roji config init     # デフォルト設定ファイルを作成
roji config show     # 現在の有効な設定を表示
roji config path     # 設定ファイルの場所を表示
roji config edit     # $EDITORで開く
```

## バリデーション

rojiは起動時に設定ファイルを検証します：

- **不明なキー**は警告として報告（タイポの検出）
- **型エラー**が報告される（例: intが必要な場所にstring）
- **必須フィールドの欠落** — `static_sites` のhost、rootなど

`roji doctor` で完全な検証レポートを確認できます。
