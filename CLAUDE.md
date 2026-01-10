# CLAUDE.md - roji 開発ガイド

## プロジェクト概要

**roji**（路地）は、ローカル開発環境専用のシンプルなリバースプロキシ。Docker Composeのサービスを自動検出し、`*.localhost` でHTTPSアクセスを可能にする。

> "本番は高速道路（Traefik）、開発は路地裏（roji）で"

### 言語ルール

- **CLAUDE.md**: 日本語（開発者向け内部ドキュメント）
- **その他すべて**: 英語（README.md、CONTRIBUTING.md、ソースコード、コメント）

### コンセプト

- **Traefikより軽量・シンプル**: ローカル開発に特化することで複雑な機能を省略
- **ネットワークベースの自動検出**: 共有ネットワークに接続 = プロキシ対象
- **ゼロコンフィグ志向**: ラベルなしでも動作、必要に応じてラベルでカスタマイズ

## 技術スタック

- **言語**: Go 1.25+
- **主要ライブラリ**:
  - `github.com/docker/docker/client` - Docker API
  - `net/http/httputil` - ReverseProxy（標準ライブラリ）
  - `crypto/x509`, `crypto/tls` - 証明書生成（標準ライブラリ）
- **フロントエンド**: Petite Vue（~6KB、ビルド不要）
- **配布**: Docker イメージ (ghcr.io/kan/roji)

## アーキテクチャ

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

## ディレクトリ構造

```
roji/
├── cmd/roji/
│   ├── main.go              # エントリーポイント
│   └── cmd/                  # Cobraコマンド（root, routes, version, health, server）
├── docker/
│   ├── client.go            # Docker API ラッパー
│   └── watcher.go           # Events 監視
├── proxy/
│   ├── handler.go           # ReverseProxy 実装
│   ├── router.go            # ルーティング + SSE Pub/Sub
│   └── templates/           # HTML/CSS/JS（embed.FS）
├── certgen/generator.go     # TLS証明書生成
├── config/labels.go         # ラベルパーサー
├── test/                    # インテグレーション/E2Eテスト
├── Dockerfile               # マルチステージビルド
├── docker-compose.yml       # 本番用
├── docker-compose.dev.yml   # 開発用（Air ホットリロード）
└── install.sh               # ワンライナーインストール
```

## 設計仕様

### ラベル仕様

| ラベル | 説明 | デフォルト |
|--------|------|-----------|
| `roji.host` | カスタムホスト名 | `{service}.localhost` |
| `roji.port` | ターゲットポート | 最初のEXPOSEポート |
| `roji.path` | パスプレフィックス | なし |
| `roji.mock.{METHOD}.{PATH}` | モックレスポンス | なし |
| `roji.mock.status.{METHOD}.{PATH}` | モックステータスコード | `200` |

### 環境変数

| 環境変数 | 説明 | デフォルト |
|----------|------|-----------|
| `ROJI_NETWORK` | 監視するDockerネットワーク（カンマ区切り） | `roji` |
| `ROJI_DOMAIN` | ベースドメイン | `localhost` |
| `ROJI_CERTS_DIR` | 証明書ディレクトリ | `/certs` |
| `ROJI_DATA_DIR` | データディレクトリ | `/data` |
| `ROJI_DASHBOARD` | ダッシュボードホスト名 | `roji.localhost` |
| `ROJI_LOG_LEVEL` | ログレベル | `info` |

### API エンドポイント

| エンドポイント | 説明 |
|----------------|------|
| `/_api/routes` | ルート一覧（JSON） |
| `/_api/projects` | プロジェクト一覧 |
| `/_api/events` | SSEストリーム（ルート更新） |
| `/_api/logs` | SSEストリーム（リクエストログ） |
| `/_api/health` | ヘルスチェック |
| `/_api/status` | 詳細ステータス（証明書期限等） |
| `/_api/containers/{id}/restart` | コンテナ再起動 |

## 実装済み機能（v0.1.0 → v0.6.0）

### コア機能
- ネットワークベースの自動検出（Docker Events監視）
- ホスト名/パスベースルーティング
- TLS証明書の自動生成（CA + ワイルドカード）
- HTTP → HTTPS リダイレクト

### ダッシュボード
- リアルタイム更新（SSE + Petite Vue）
- プロジェクト履歴・クイックアクセス
- リクエストログビューア（フィルタリング対応）
- コンテナ再起動ボタン
- ダークモード（システム設定連動）
- 設定ミス警告表示

### 開発支援
- リクエストモック（ラベルでレスポンス定義）
- 複数ネットワーク対応
- ブラウザ通知

### 配布・品質
- ワンライナーインストール（アップグレード対応）
- GoReleaser v2（マルチプラットフォーム）
- Distrolessイメージ
- セキュリティスキャン（Trivy, govulncheck）
- インテグレーション/E2Eテスト

## リリース手順

1. **ドキュメント更新**
   ```bash
   # README.md: install.sh URLを新バージョンに更新（2箇所）
   # CHANGELOG.md: 新バージョンの変更内容を追加
   ```

2. **コミット & タグ**
   ```bash
   git add README.md CHANGELOG.md
   git commit -m "Prepare for vX.Y.Z release"
   git tag -a vX.Y.Z -m "Release vX.Y.Z: [主要機能]"
   git push origin main
   git push origin vX.Y.Z
   ```

3. **確認**
   - GitHub Actions → GitHub Release → Docker Image

## 将来の課題（v0.7.0〜）

### v0.7.0 候補

- [ ] **ログのエクスポート**
  - リクエストログをJSON/CSVでダウンロード
  - ダッシュボードにエクスポートボタン追加
  - 期間指定・フィルタ適用後のエクスポート

- [ ] **WebSocket対応**
  - WebSocket接続のプロキシ（`Upgrade: websocket`）
  - フロントエンド開発での必須機能
  - 既存のReverseProxyにWebSocketハンドラー追加

- [ ] **VS Code拡張**
  - ルート一覧表示（サイドバー）
  - ワンクリックでブラウザを開く
  - コンテナ再起動
  - ログビューア連携
  - roji APIとの通信（`/_api/*`）

### 将来構想

| 機能 | 説明 |
|------|------|
| ルート別ヘルスチェック | 各バックエンドの死活監視 |
| レスポンスタイム統計 | P50/P95/P99レイテンシ表示 |
| gRPC対応 | HTTP/2 + gRPCプロキシ |
| プロジェクト操作 | ダッシュボードからstart/stop |

## 開発メモ

### テスト実行

```bash
# ユニットテスト
go test -v ./...

# インテグレーションテスト
cd test && go test -v -tags=integration ./...

# E2Eテスト
cd test && go test -v -tags=e2e ./...
```

### GoReleaser

GoReleaser v2を使用。設定変更時は必ず検証を実行：

```bash
# インストール（未導入の場合）
go install github.com/goreleaser/goreleaser/v2@latest

# 設定検証
goreleaser check

# ローカルでスナップショットビルド（リリースなし）
goreleaser release --snapshot --clean
```

**v2の主な特徴:**
- `version: 2` ヘッダーが必須
- `dockers_v2`: 単一設定でマルチプラットフォームイメージ+マニフェスト生成
- Dockerfileで `ARG TARGETPLATFORM` を使用し `${TARGETPLATFORM}/binary` からコピー

### 手動テスト

```bash
# 開発サーバー起動（ホットリロード）
docker compose -f docker-compose.dev.yml up

# テストサービス起動
cd test && docker compose up -d

# 動作確認
curl -k https://web.dev.localhost
```

## 参考リンク

- [Docker Engine API](https://docs.docker.com/engine/api/)
- [Go httputil.ReverseProxy](https://pkg.go.dev/net/http/httputil#ReverseProxy)
- [Petite Vue](https://github.com/vuejs/petite-vue)
- [VS Code Extension API](https://code.visualstudio.com/api)
