---
title: "静的ファイルホスティング"
slug: "static-sites"
description: "Dockerコンテナなしで静的ファイルを配信。"
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

rojiは設定ファイルの `static_sites` セクションを使って、Dockerコンテナなしで静的ファイルを配信できます。

## 設定

`~/.config/roji/config.yaml` にエントリを追加：

```yaml
static_sites:
  - host: docs                    # -> docs.dev.localhost
    root: ~/projects/docs/build
    # index: true                 # ディレクトリ一覧（デフォルト: 有効）
  - host: private.example.com     # FQDN（ドットを含む）
    root: /var/www/private
    index: false                  # ディレクトリ一覧を無効化
```

### ホスト名解決

- **ドットなし**: `{host}.{ROJI_DOMAIN}` に展開（例: `docs` → `docs.dev.localhost`）
- **ドットあり**: FQDNとしてそのまま使用

### ディレクトリ一覧

- `index: true`（デフォルト）— `index.html` がない場合にApache/nginx風のディレクトリ一覧を表示
- `index: false` — `index.html` なしのディレクトリアクセスに403 Forbiddenを返す

### Docker ルートとの優先順位

静的サイトはホスト名を保持します。同じホスト名にコンテナがルーティングされて
いる場合（パスプレフィックスの有無を問わず）、静的サイトが応答し、どの Docker
ルートが隠れたかを roji がログに出します。

設定ファイルで宣言したホスト名はあなたのもの、という規則です。Docker ルートは
自動検出されるものなので、後から起動したコンテナが既に主張済みの名前を奪うことは
ありません。逆にしたい場合は、コンテナの `roji.host` を変えるか、静的サイトの
定義を外してください。

## 変更の適用

再起動は不要です。以下のいずれかで適用：

```bash
roji config reload
```

またはダッシュボードの **Reload Config** ボタンをクリック。

## 認証の追加

BASIC認証で静的サイトを保護：

```yaml
static_sites:
  - host: docs.dev.localhost
    root: ~/projects/docs/build
    auth:
      basic:
        user: admin
        pass: secret
        realm: Documentation   # 任意
```

詳細は[BASIC認証](/ja/docs/guides/basic-auth/)ガイドを参照。

## トンネルで外部公開する

静的サイトは、[トンネル](/ja/docs/guides/tunnel/)の公開対象に指定しない限り非公開です。

```yaml
static_sites:
  - host: docs
    root: ~/projects/docs/build
    tunnel: true        # https://docs.example.com で到達できる
```

`root` 以下のすべてがインターネットから読めるようになります。`index: false` に
しない限り、ディレクトリ一覧も見えます。公開範囲を絞りたい場合は `auth` を
追加してください。

ドットファイルは例外です。トンネル経由では、`.` で始まる要素を含むパス
（`.env`、`.git/config` など）は 404 になり、ディレクトリ一覧にも出ません。
`.well-known/` だけは配信します。シンボリックリンクはたどるので、`root` の外を
指すリンクがあると、その先も公開されます。

## ダッシュボード連携

静的サイトはDockerベースのルートと並んでダッシュボードに表示されます。ダッシュボードでは：

- ディレクトリ一覧の状態アイコン
- Reload Configボタンで再起動なしに変更を適用
