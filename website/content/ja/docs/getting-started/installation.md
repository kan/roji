---
title: "インストール"
slug: "installation"
description: "macOS、Linux、WSLにrojiをインストール。"
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Homebrew（macOS）

```bash
brew install kan/roji/roji
```

## ワンライナーインストール（推奨）

Linux・macOS対応：

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.2.0/install.sh | bash
```

実行内容：

- プラットフォーム（Linux/macOS、x86_64/arm64）に合ったバイナリをダウンロード
- デフォルトで `~/.local/bin` にインストール（対話式でインストール先を選択）
- `roji doctor --fix` で環境をセットアップ
- CA証明書をシステム信頼ストアにインストール
- rojiをシステムサービスとして登録・起動

### オプション

```bash
curl -fsSL ... | bash -s -- --global       # /usr/local/bin にインストール
curl -fsSL ... | bash -s -- --local        # ~/.local/bin にインストール（デフォルト）
curl -fsSL ... | bash -s -- --no-service   # サービス登録をスキップ
curl -fsSL ... | bash -s -- --upgrade      # アップグレード確認をスキップ
```

## 手動インストール

[GitHub Releases](https://github.com/kan/roji/releases)からダウンロード、またはソースからビルド：

```bash
git clone https://github.com/kan/roji.git && cd roji && make build
sudo ./bin/roji doctor --fix    # 環境セットアップ
sudo ./bin/roji ca install      # CA証明書インストール
sudo ./bin/roji service install && sudo ./bin/roji service start
```

## アップグレード

インストールスクリプトを再実行すると、既存のインストールを検出して自動アップグレードします：

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.2.0/install.sh | bash
```

Homebrew の場合：

```bash
brew upgrade kan/roji/roji
```

## インストール確認

```bash
roji version    # バージョン情報を表示
roji doctor     # 環境チェック
```

すべてのチェックがパスするはずです。問題がある場合は `sudo roji doctor --fix` で自動修復できます。

## 次のステップ

- [クイックスタート](/ja/docs/getting-started/quick-start/) — 最初のサービスを起動
- [設定](/ja/docs/guides/configuration/) — rojiの設定をカスタマイズ
