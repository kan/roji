---
title: "Linux"
description: "Linux固有のトラブルシューティング。"
weight: 4
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## 証明書ストア

`roji ca install` は主要なLinuxファミリーの両方をサポート：

- **Debian/Ubuntu**: `update-ca-certificates` を使用（`/usr/local/share/ca-certificates/`）
- **RHEL/Fedora**: `update-ca-trust` を使用（`/etc/pki/ca-trust/source/anchors/`）

## Chrome/Chromium NSS

ChromeはLinuxで独自の証明書ストアを使用します。Chromeでのみ証明書エラーが発生する場合：

```bash
# nss-toolsをインストール
sudo apt install libnss3-tools   # Debian/Ubuntu
sudo dnf install nss-tools       # Fedora

# ChromeのストアにCAを追加
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "roji CA" -i ~/.local/share/roji/certs/ca.pem
```

または組み込みフラグを使用：

```bash
sudo roji ca install --firefox   # LinuxではChromeにも有効
```

## ポート権限

1024以下のポートにはroot権限が必要です。`sudo roji` を使用するか、サービスインストーラーを使用：

```bash
sudo roji service install  # 適切な権限でsystemdを使用
sudo roji service start
```

## systemdの問題

サービスの状態を確認：

```bash
roji service status
systemctl status roji
journalctl -u roji -f    # systemdログを表示
```

サービスが起動に失敗する場合、ログファイルを確認：

```bash
roji log --no-follow
```
