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
- WebSocketプロキシ（`Upgrade: websocket`ヘッダー検出、双方向通信）

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

## ロードマップ（v0.7.0 → v1.0.0）

### バージョン計画

| Version | テーマ | 主要機能 |
|---------|--------|----------|
| **v0.7.0** | プロトコル拡張 | WebSocket対応、gRPC対応、ログエクスポート |
| **v0.8.0** | Native Mode | 単体バイナリ化、`roji doctor`、CA自動インストール |
| **v0.9.0** | 運用強化 | サービス登録、Docker Compose操作、httpd機能、BASIC認証 |
| **v1.0.0** | 安定版 | ドキュメント整備、破壊的変更の整理、Docker Mode廃止 |

---

### v0.7.0: プロトコル拡張

- [x] **WebSocket対応**
  - WebSocket接続のプロキシ（`Upgrade: websocket`）
  - フロントエンド開発での必須機能
  - 既存のReverseProxyにWebSocketハンドラー追加

- [x] **gRPC対応**
  - HTTP/2 + gRPCプロキシ（`Content-Type: application/grpc`で自動検出）
  - HTTPSサーバーでHTTP/2を有効化（gRPCに必須）
  - ※ gRPC-Web対応は要望があれば検討

- [x] **ログのエクスポート**
  - リクエストログをJSON/CSVでダウンロード（`/_api/logs/export`）
  - ダッシュボードにエクスポートボタン追加
  - ホスト/サービス/メソッド/期間でフィルタリング

---

### v0.8.0: Native Mode（単体バイナリ化）

v0.8.0以降は**Native Mode**を主とする。Docker Modeは1.0.0で廃止予定。

#### 概要

ホスト上で直接動作し、Docker環境を外部から操作する形態に移行。

```
v0.8.0 Architecture (Native Mode)
┌─────────────────────────────────────────────────────────────────┐
│                     roji (Native Mode)                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐      │
│  │   Docker     │  │    Route     │  │   HTTP/HTTPS     │      │
│  │   Watcher    │→ │   Manager    │→ │  Reverse Proxy   │      │
│  └──────────────┘  └──────────────┘  └──────────────────┘      │
│         ↓                ↓                   ↑                  │
│   Docker Socket    SSE Broadcast        Host :80/:443           │
│  (/var/run/...)                         (特権必要)              │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐                             │
│  │    Doctor    │  │     CA       │                             │
│  │    Command   │  │   Installer  │                             │
│  └──────────────┘  └──────────────┘                             │
│   環境チェック      OS Trust Store                               │
└─────────────────────────────────────────────────────────────────┘
```

#### 設定の優先順位

設定は以下の優先順位で適用（後が優先）：

1. デフォルト値
2. 設定ファイル（`~/.config/roji/config.yaml`）
3. 環境変数（`ROJI_*`）
4. コマンドライン引数（`--network` 等）

#### 設定ファイル仕様

```yaml
# ~/.config/roji/config.yaml
network: roji
domain: dev.localhost
certs_dir: ~/.local/share/roji/certs
data_dir: ~/.local/share/roji/data
dashboard: roji.dev.localhost
log_level: info
http_port: 80
https_port: 443
```

#### roji doctor コマンド

```
$ roji doctor

✓ Docker daemon is running
✓ Docker socket is accessible (/var/run/docker.sock)
✓ Network 'roji' exists
✓ Ports 80, 443 are available
✓ CA certificate is installed (macOS Keychain)
✓ Server certificate is valid (expires in 364 days)
✗ Firefox certificate not installed (manual step required)

Issues found: 1
Run 'roji doctor --fix' to auto-fix where possible
```

**チェック項目:**
- Docker daemon稼働
- Docker socketアクセス権
- 指定ネットワークの存在
- ポート80/443の利用可能性
- CA証明書のインストール状態
- サーバー証明書の有効期限
- DNS解決（*.localhost）

#### CA証明書の自動インストール

**新コマンド:**
- `roji ca install` - CA証明書をシステムにインストール
- `roji ca uninstall` - CA証明書を削除
- `roji ca export` - CA証明書をエクスポート（他デバイス用）

**プラットフォーム別実装:**

| プラットフォーム | 方法 | 実装 |
|------------------|------|------|
| macOS | Keychain | `security add-trusted-cert` |
| Linux (Debian系) | update-ca-certificates | `/usr/local/share/ca-certificates/` |
| Linux (RHEL系) | update-ca-trust | `/etc/pki/ca-trust/source/anchors/` |
| Windows | certutil | `certutil -addstore -f "ROOT"` |

---

### v0.9.0: 運用強化

#### サービスとしての稼働

**新コマンド:**
- `roji service install` - サービス登録
- `roji service uninstall` - サービス削除
- `roji service start/stop/restart` - サービス制御
- `roji service status` - 状態確認

**プラットフォーム別:**

| プラットフォーム | サービス管理 | 設定場所 |
|------------------|--------------|----------|
| Linux | systemd | `/etc/systemd/system/roji.service` |
| macOS | launchd | `~/Library/LaunchAgents/com.roji.plist` |
| Windows | Windows Service | NSSM または sc.exe |

#### Docker Compose操作

ダッシュボード/APIからdocker-composeプロジェクトを操作:

| API | 説明 |
|-----|------|
| `POST /_api/projects/{name}/up` | `docker compose up -d` |
| `POST /_api/projects/{name}/down` | `docker compose down` |
| `POST /_api/projects/{name}/restart` | 全サービス再起動 |
| `GET /_api/projects/{name}/logs` | ログ取得（SSE） |

**実装方針:**
- `project.Store`に保存済みの`working_dir`と`config_files`を使用
- `docker compose -f <files> -p <project> <command>` を実行

#### 静的ファイルホスティング（httpd機能）

**設定ファイルで定義:**

```yaml
# ~/.config/roji/config.yaml
static_sites:
  - host: docs.dev.localhost
    root: ~/projects/docs/build
  - host: assets.dev.localhost
    root: /var/www/assets
```

**または新コマンド:**

```bash
# 一時的にホスト（rojiサーバーに登録）
roji serve ~/projects/docs --host docs.dev.localhost
```

#### BASIC認証

特定ルートにBASIC認証を設定可能。

**Docker Composeラベルで設定:**

```yaml
# docker-compose.yml
services:
  admin:
    labels:
      - "roji.auth.basic.user=admin"
      - "roji.auth.basic.pass=secret"
      - "roji.auth.basic.realm=Admin Area"  # 任意
```

**設定ファイルで静的サイトに適用:**

```yaml
# ~/.config/roji/config.yaml
static_sites:
  - host: docs.dev.localhost
    root: ~/projects/docs/build
    auth:
      basic:
        user: admin
        pass: secret
        realm: Documentation  # 任意
```

**実装方針:**
- 標準ライブラリのみで実装（`net/http`）
- パスワードは平文（ローカル開発用途のため）
- 401レスポンス時に`WWW-Authenticate`ヘッダー付与

---

### v1.0.0: 安定版リリース

- [ ] **Docker Mode廃止**
  - Native Modeへの完全移行
  - Dockerfile、docker-compose.yml は開発・テスト用に維持

- [ ] **インストール方法の刷新**
  - install.shの全面書き換え
  - バイナリダウンロード + サービス登録の自動化
  - Homebrew対応（macOS）
  - APT/RPMパッケージ検討

- [ ] **ドキュメント整備**
  - README.mdの全面改訂
  - Getting Started ガイド
  - トラブルシューティングガイド

- [ ] **破壊的変更の整理**
  - 環境変数のデフォルト値見直し
  - 設定ファイルパスの標準化
  - 廃止予定のAPIの削除

---

### 将来構想（v1.x〜）

| 機能 | 説明 | 優先度 |
|------|------|--------|
| VS Code拡張 | ルート一覧、ブラウザ起動、ログ連携（API安定後） | 高 |
| ルート別ヘルスチェック | 各バックエンドの死活監視、ダッシュボード表示 | 中 |
| レスポンスタイム統計 | P50/P95/P99レイテンシ、ダッシュボードでグラフ表示 | 中 |
| プラグインシステム | カスタムミドルウェア、認証プロキシ等の拡張 | 低 |

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
