---
title: "macOS"
description: "macOS固有のトラブルシューティング。"
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## キーチェーン

`roji ca install` はCAをシステムキーチェーンに追加します（パスワードが必要）。`--user` を使うとログインキーチェーンに追加（sudo不要）：

```bash
roji ca install --user    # ログインキーチェーン（sudo不要）
sudo roji ca install      # システムキーチェーン
```

## Docker Desktop

コンテナはLinux VM内で実行されます。rojiはDockerソケット経由で接続し、透過的に動作します。特別な設定は不要です。

## ポート権限

1024以下のポートにはroot権限が必要です。インストーラーは `launchd` でこれを設定します。手動で実行する場合：

```bash
sudo roji
```

または代替ポートを使用：

```yaml
# ~/.config/roji/config.yaml
http_port: 8080
https_port: 8443
```
