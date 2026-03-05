---
title: "設定"
slug: "configuration"
description: "設定ファイル、環境変数、CLIフラグでrojiを設定。"
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

rojiは設定ファイル、環境変数、CLIフラグで設定できます。

## 設定ファイル

場所: `~/.config/roji/config.yaml`

```yaml
network: roji                          # 監視するDockerネットワーク（カンマ区切り）
domain: dev.localhost                  # ベースドメイン
http_port: 80                          # HTTPポート（HTTPSにリダイレクト）
https_port: 443                        # HTTPSポート
certs_dir: ~/.local/share/roji/certs   # 証明書ディレクトリ
data_dir: ~/.local/share/roji          # データディレクトリ
dashboard: roji.dev.localhost          # ダッシュボードホスト名
log_level: info                        # ログレベル（debug, info, warn, error）
auto_cert: true                        # 証明書の自動生成

static_sites:                          # 静的ファイルホスティング（静的サイトガイド参照）
  - host: docs
    root: ~/projects/docs/build
```

### 設定ファイルの管理

```bash
roji config show     # 現在の設定を表示
roji config path     # 設定ファイルのパスを表示
roji config init     # デフォルト設定ファイルを作成
roji config edit     # $EDITORで開く
```

## 環境変数

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `ROJI_NETWORK` | 監視するDockerネットワーク（カンマ区切り） | `roji` |
| `ROJI_DOMAIN` | ベースドメイン | `dev.localhost` |
| `ROJI_HTTP_PORT` | HTTPポート | `80` |
| `ROJI_HTTPS_PORT` | HTTPSポート | `443` |
| `ROJI_CERTS_DIR` | 証明書ディレクトリ | `~/.local/share/roji/certs` |
| `ROJI_DATA_DIR` | データディレクトリ（プロジェクト履歴） | `~/.local/share/roji` |
| `ROJI_DASHBOARD` | ダッシュボードホスト名 | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | ログレベル | `info` |
| `ROJI_AUTO_CERT` | 証明書の自動生成 | `true` |

## 設定の優先順位

優先度の高い順：

1. **CLIフラグ** (`--network`, `--domain` など)
2. **環境変数** (`ROJI_NETWORK`, `ROJI_DOMAIN` など)
3. **設定ファイル** (`~/.config/roji/config.yaml`)
4. **デフォルト値**

## 設定ファイルのバリデーション

rojiは起動時と `roji doctor` で設定ファイルを検証します：

- **不明なキー**: キー名のタイポを検出
- **型チェック**: 値の型（string, int, bool, array, object）を検証
- **必須フィールド**: `static_sites` の必須フィールド欠落をチェック

警告は起動時にログ出力されます。`roji doctor` で完全な検証レポートを確認できます。
