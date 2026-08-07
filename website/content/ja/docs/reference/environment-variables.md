---
title: "環境変数"
slug: "environment-variables"
description: "すべてのROJI_*環境変数。"
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## 変数リファレンス

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `ROJI_NETWORK` | 監視するDockerネットワーク（カンマ区切り） | `roji` |
| `ROJI_DOMAIN` | サービスホスト名のベースドメイン | `dev.localhost` |
| `ROJI_BIND` | 待ち受けアドレス（カンマ区切り、空で全インターフェース） | `127.0.0.1,::1` |
| `ROJI_HTTP_PORT` | HTTPポート（HTTPSにリダイレクト） | `80` |
| `ROJI_HTTPS_PORT` | HTTPSポート | `443` |
| `ROJI_CERTS_DIR` | TLS証明書のディレクトリ | `~/.local/share/roji/certs` |
| `ROJI_DATA_DIR` | データディレクトリ（プロジェクト履歴、ログ） | `~/.local/share/roji` |
| `ROJI_DASHBOARD` | ダッシュボードホスト名 | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | ログレベル（`debug`, `info`, `warn`, `error`） | `info` |
| `ROJI_AUTO_CERT` | TLS証明書の自動生成 | `true` |

## 使い方

```bash
# カスタム設定で起動
ROJI_DOMAIN=test.localhost ROJI_LOG_LEVEL=debug sudo roji

# 複数ネットワークを監視
ROJI_NETWORK=web,api sudo roji

# 代替ポートを使用
ROJI_HTTP_PORT=8080 ROJI_HTTPS_PORT=8443 sudo roji
```

## 優先順位

環境変数は設定ファイルの値を上書きしますが、CLIフラグには上書きされます：

**CLIフラグ** > **環境変数** > **設定ファイル** > **デフォルト値**

詳細は[設定](/ja/docs/guides/configuration/)を参照。
