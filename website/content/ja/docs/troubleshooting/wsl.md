---
title: "WSL"
description: "WSL固有のトラブルシューティング。"
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## 証明書

ブラウザがHTTPSを信頼するために、LinuxとWindowsの**両方**にCA証明書をインストールする必要があります：

```bash
sudo roji ca install             # Linux信頼ストア
sudo roji ca install --windows   # Windows信頼ストア（Windows上のChrome/Edge用）
```

`--windows` フラグは `certutil.exe` を使用してWindows ユーザー証明書ストア（CurrentUser\ROOT）に証明書をインストールします。

## PATH

`~/.local/bin` にインストールした場合、PATHに追加されていることを確認：

```bash
export PATH="$HOME/.local/bin:$PATH"  # ~/.bashrc または ~/.zshrc に追加
```

## Docker

WSL2とDocker Desktopの組み合わせはそのまま動作します。Docker DesktopのWSL統合が有効になっていることを確認：

1. Docker Desktop → 設定 → リソース → WSL統合
2. お使いのWSLディストリビューションを有効化

DockerソケットはWSLとDocker Desktopの間で自動的に共有されます。
