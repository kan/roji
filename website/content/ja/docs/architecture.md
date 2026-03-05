---
title: "アーキテクチャ"
slug: "architecture"
description: "rojiの内部動作の仕組み。"
weight: 5
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## 概要

rojiはホストマシン上で動作する単一のGoバイナリで、Dockerイベントを監視し、HTTP/HTTPSリクエストをコンテナにプロキシします。

```
┌─────────────────────────────────────────────────────────────┐
│                         roji                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Docker     │  │    Route     │  │   HTTP/HTTPS     │  │
│  │   Watcher    │→ │   Manager    │→ │  Reverse Proxy   │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│         ↓                ↓                   ↑              │
│   Docker Socket    SSE Broadcast        :80 / :443          │
└─────────────────────────────────────────────────────────────┘
```

## コンポーネント

### Docker Watcher

Dockerソケットに接続し、コンテナイベント（start, stop, die）を監視します。監視対象ネットワーク上のコンテナが起動すると、以下を抽出します：

- コンテナ名とサービス名
- ネットワーク所属
- 公開ポート
- Dockerラベル（`roji.*`）
- Docker Composeプロジェクト情報（作業ディレクトリ、設定ファイル）

### Route Manager

ルーティングテーブルを管理し、変更をブロードキャストします：

- ホスト名 + パスプレフィックスをコンテナバックエンドにマッピング
- ホストベースとパスベースのルーティングをサポート
- Server-Sent Events（SSE）で接続中のダッシュボードクライアントに通知
- Docker Composeプロジェクト（アクティブ・非アクティブ）を追跡

### HTTP/HTTPS リバースプロキシ

Goの `net/http/httputil.ReverseProxy` をベースに構築：

- **HTTP（ポート80）**: すべてのリクエストをHTTPSにリダイレクト
- **HTTPS（ポート443）**: TLSを終端し、コンテナバックエンドにプロキシ
- **WebSocket**: `Upgrade: websocket` ヘッダーで検出、双方向プロキシ
- **gRPC**: `Content-Type: application/grpc` で検出、HTTP/2でプロキシ
- **モックルート**: バックエンドにアクセスせず設定済みのレスポンスを返す

### 証明書ジェネレーター

Goの標準ライブラリ `crypto/x509` と `crypto/tls` を使用してTLS証明書を管理：

- **CA証明書**: 一度生成され、証明書ディレクトリに保存
- **サーバー証明書**: `*.{domain}` のワイルドカード証明書、期限切れ時に再生成
- **システムインストール**: CAをOSの信頼ストアにインストール（macOS Keychain、Linux CAストア、Windows certutil）

### ダッシュボード

[Petite Vue](https://github.com/vuejs/petite-vue)（約6KB、ビルド不要）で構築されたシングルページアプリケーション：

- SSEによるリアルタイム更新（ポーリングなし）
- Docker Compose操作（起動/停止/再起動）
- フィルタリング対応のリクエストログビューア
- ダークモード（システム設定に追従）
- `embed.FS` でGoバイナリに埋め込み

## リクエストフロー

1. ブラウザが `https://myapp.dev.localhost` にHTTPSリクエストを送信
2. rojiのHTTPSサーバーがワイルドカード証明書でTLSを終端
3. Route Managerがルーティングテーブルで `myapp.dev.localhost` を検索
4. 見つかった場合、`httputil.ReverseProxy` がコンテナの内部IPとポートにリクエストを転送
5. レスポンスがブラウザに返される

## 証明書チェーン

```
roji CA（自己署名）
  └── *.dev.localhost（サーバー証明書）
        └── すべての *.dev.localhost リクエストにHTTPSサーバーが使用
```

CAは一度生成され、再起動後も永続化されます。サーバー証明書は設定済みドメインの全サブドメインをカバーするワイルドカードです。

## 技術選定

| コンポーネント | 技術 | 理由 |
|--------------|------|------|
| 言語 | Go | 単一バイナリ、高速起動、優れた標準ライブラリ |
| リバースプロキシ | `net/http/httputil` | 標準ライブラリ、実績あり |
| TLS | `crypto/x509`, `crypto/tls` | 外部依存なし |
| フロントエンド | Petite Vue | 約6KB、ビルド不要、リアクティブ |
| リアルタイム | Server-Sent Events | 一方向更新にはWebSocketよりシンプル |
| Docker | Docker Engine API | 直接API アクセス、コア機能にCLI依存なし |
