---
title: "CLIリファレンス"
slug: "cli"
description: "rojiの全コマンドとフラグ。"
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## `roji`

リバースプロキシサーバーを起動。

```bash
sudo roji                                          # フォアグラウンドで起動
sudo roji --network web,api                        # 複数ネットワークを監視
sudo roji --domain test.localhost                   # カスタムドメイン
sudo roji --http-port 8080 --https-port 8443       # カスタムポート
sudo roji --log-level debug                        # 詳細ログ
```

**フラグ：**

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--network` | 監視するDockerネットワーク | `roji` |
| `--domain` | ベースドメイン | `dev.localhost` |
| `--http-port` | HTTPポート | `80` |
| `--https-port` | HTTPSポート | `443` |
| `--certs-dir` | 証明書ディレクトリ | `~/.local/share/roji/certs` |
| `--data-dir` | データディレクトリ | `~/.local/share/roji` |
| `--dashboard` | ダッシュボードホスト名 | `roji.{domain}` |
| `--log-level` | ログレベル（debug, info, warn, error） | `info` |
| `--auto-cert` | 証明書の自動生成 | `true` |
| `--config` | 設定ファイルパス | `~/.config/roji/config.yaml` |

## `roji doctor`

環境チェックと問題の自動修復。

```bash
roji doctor          # 全チェック実行
roji doctor --fix    # 修復可能な問題を自動修復
roji doctor --json   # JSON形式で出力
```

**チェック項目：**

- Dockerデーモンの稼働
- Dockerソケットのアクセス権
- Dockerネットワークの存在
- ポート80/443の利用可能性
- CA証明書の存在と有効性
- CA証明書のシステムインストール状態
- サーバー証明書の有効期限
- DNS解決（*.localhost）
- 設定ファイルのバリデーション
- トンネル（設定時のみ: cloudflared の導入、ログイン、named tunnel の存在）

## `roji config`

設定の管理。

```bash
roji config show     # 現在の設定を表示
roji config path     # 設定ファイルのパスを表示
roji config init     # デフォルト設定ファイルを作成
roji config edit     # $EDITORで開く
```

## `roji ca`

CA証明書の管理。

```bash
roji ca status              # インストール状態を確認
roji ca install             # システム信頼ストアにインストール
roji ca install --user      # ユーザーストアにインストール（sudo不要）
roji ca install --windows   # Windows（WSLから）にインストール
roji ca install --firefox   # Firefoxにもインストール（Linux）
roji ca install --force     # 強制再インストール
roji ca uninstall           # 信頼ストアから削除
roji ca export [path]       # CA証明書をエクスポート（.pemまたは.crt）
```

**プラットフォーム対応：**

| プラットフォーム | 方法 |
|------------------|------|
| macOS | Keychain (`security add-trusted-cert`) |
| Linux (Debian/Ubuntu) | `update-ca-certificates` |
| Linux (RHEL/Fedora) | `update-ca-trust` |
| Windows | `certutil -addstore` |
| WSL → Windows | `certutil.exe`（ユーザーストア） |

## `roji service`

システムサービスとしてrojiを管理。

```bash
roji service install    # サービスとして登録
roji service uninstall  # サービス登録を削除
roji service start      # サービスを起動
roji service stop       # サービスを停止
roji service restart    # サービスを再起動
roji service status     # 状態を確認
```

**プラットフォーム対応：**

| プラットフォーム | サービス管理 | 設定場所 |
|------------------|--------------|----------|
| Linux | systemd | `/etc/systemd/system/roji.service` |
| macOS | launchd | `~/Library/LaunchAgents/com.roji.agent.plist` |
| Windows | NSSM | Windows Service |

## `roji log`

サーバーログを表示。

```bash
roji log               # リアルタイムでログを追跡（tail -f相当）
roji log -n 100        # 最新100行を表示後、追跡
roji log --no-follow   # 現在のログを表示して終了
```

**ログファイルの場所：**

- Linux/WSL: `~/.local/share/roji/roji.log`
- macOS: `~/Library/Logs/roji.log`

ログは10MBを超えると自動ローテートされます。

## `roji routes`

起動中のサーバーから全登録ルートを一覧表示。

## `roji version`

バージョン、コミットハッシュ、ビルド日、Goバージョンを表示。

## `roji health`

rojiサーバーが正常かチェック。正常なら終了コード0、異常なら1で終了。
