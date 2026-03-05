---
title: "よくある問題"
slug: "common-issues"
description: "よくある問題と解決方法。"
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## まず `roji doctor` を実行

常に診断から始めてください：

```bash
roji doctor            # 問題をチェック
sudo roji doctor --fix # 修復可能な問題を自動修復
```

Docker、ネットワーク、証明書、ポート、DNSをチェックし、ほとんどの一般的な問題を自動修復できます。

## `.localhost` ドメインが解決されない

**macOS**: `.localhost` は自動的に `127.0.0.1` に解決されます。対応不要です。

**Linux (systemd-resolved)**: 最近のディストリビューションの多くは `.localhost` を自動的に解決します。そうでない場合：

```bash
# systemd-resolvedが処理しているか確認
resolvectl query myapp.dev.localhost

# されていない場合は /etc/hosts にエントリを追加
echo "127.0.0.1 myapp.dev.localhost roji.dev.localhost dev.localhost" | sudo tee -a /etc/hosts
```

**Firefox**: FirefoxはシステムDNSリゾルバを使用しない場合があります。`about:config` で `browser.fixup.domainsuffixwhitelist.localhost` を `true` に設定してください。

**代替方法**: `*.lvh.me` を使用 — 常に `127.0.0.1` に解決されるパブリックドメイン：

```yaml
# ~/.config/roji/config.yaml
domain: lvh.me
```

## 証明書エラー（ERR_CERT_AUTHORITY_INVALID）

CA証明書がブラウザやOSに信頼されていません：

```bash
sudo roji ca install           # システム信頼ストアにインストール
```

その後**ブラウザを完全に再起動**してください（すべてのウィンドウを閉じる）。

**Firefox** は独自の証明書ストアを使用します：

- `sudo roji ca install --firefox` を実行（Linux、`nss-tools` が必要）
- または手動でインポート: Firefoxの設定 → プライバシーとセキュリティ → 証明書 → 証明書を表示 → 認証局 → インポート

**WSL**: LinuxとWindowsの両方に証明書が必要：

```bash
sudo roji ca install             # Linux信頼ストア
sudo roji ca install --windows   # Windows信頼ストア（Windowsブラウザ用）
```

**CA証明書のエクスポート**（手動インストール用）：

```bash
roji ca export ~/roji-ca.pem    # PEM形式（macOS/Linux）
roji ca export ~/roji-ca.crt    # DER形式（Windows）
```

## コンテナが検出されない

1. コンテナが `roji` ネットワークに接続されているか確認：
   ```bash
   docker network inspect roji
   ```

2. ポートが公開されているか確認（docker-compose.ymlの `expose` またはDockerfileの `EXPOSE`）：
   ```bash
   docker inspect <container> | jq '.[0].Config.ExposedPorts'
   ```

3. ダッシュボード（`https://roji.dev.localhost`）で設定警告を確認。

## ポート80または443が使用中

ポートを使用しているプロセスを確認：

```bash
sudo lsof -i :80
sudo lsof -i :443
```

よくある原因: Apache、nginx、または別のリバースプロキシ。競合サービスを停止するか、rojiを代替ポートで設定：

```yaml
# ~/.config/roji/config.yaml
http_port: 8080
https_port: 8443
```

## ログの確認

```bash
roji log                # リアルタイムでログを追跡
roji log -n 50          # 最新50行を表示
roji log --no-follow    # 表示して終了
```

ログファイルの場所：

- Linux/WSL: `~/.local/share/roji/roji.log`
- macOS: `~/Library/Logs/roji.log`

ログは10MBを超えると自動ローテートされます。
