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
│   └── cmd/                  # Cobraコマンド（root, routes, version, health, server, config, doctor, ca）
├── docker/
│   ├── client.go            # Docker API ラッパー
│   ├── compose.go           # Docker Compose CLI実行（up/down/restart/logs）
│   └── watcher.go           # Events 監視
├── proxy/
│   ├── handler.go           # ReverseProxy 実装
│   ├── router.go            # ルーティング + SSE Pub/Sub
│   └── templates/           # HTML/CSS/JS（embed.FS）
├── certgen/
│   ├── generator.go         # TLS証明書生成
│   ├── installer.go         # CAInstaller インターフェース
│   ├── installer_darwin.go  # macOS Keychain対応
│   ├── installer_linux.go   # Linux (Debian/RHEL) 対応
│   ├── installer_windows.go # Windows certutil対応
│   └── installer_wsl.go     # WSL→Windows対応
├── config/
│   ├── labels.go            # ラベルパーサー
│   ├── paths.go             # XDGパスユーティリティ
│   └── settings.go          # 設定ファイル読み込み
├── doctor/
│   ├── check.go             # Doctor インターフェース
│   └── checks/              # 各チェック実装
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
| `roji.auth.basic.user` | BASIC認証ユーザー名 | なし |
| `roji.auth.basic.pass` | BASIC認証パスワード | なし |
| `roji.auth.basic.realm` | BASIC認証レルム | `Restricted` |

### 環境変数

| 環境変数 | 説明 | デフォルト (Native Mode) |
|----------|------|-----------|
| `ROJI_NETWORK` | 監視するDockerネットワーク（カンマ区切り） | `roji` |
| `ROJI_DOMAIN` | ベースドメイン | `dev.localhost` |
| `ROJI_CERTS_DIR` | 証明書ディレクトリ | `~/.local/share/roji/certs` |
| `ROJI_DATA_DIR` | データディレクトリ | `~/.local/share/roji` |
| `ROJI_DASHBOARD` | ダッシュボードホスト名 | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | ログレベル | `info` |
| `ROJI_HTTP_PORT` | HTTPポート | `80` |
| `ROJI_HTTPS_PORT` | HTTPSポート | `443` |
| `ROJI_AUTO_CERT` | 証明書自動生成 | `true` |

### API エンドポイント

| エンドポイント | 説明 |
|----------------|------|
| `/_api/routes` | ルート一覧（JSON） |
| `/_api/projects` | プロジェクト一覧 |
| `/_api/events` | SSEストリーム（ルート更新） |
| `/_api/logs` | SSEストリーム（リクエストログ） |
| `/_api/logs/export` | ログエクスポート（JSON/CSV） |
| `/_api/health` | ヘルスチェック |
| `/_api/status` | 詳細ステータス（証明書期限等） |
| `/_api/containers/{id}/restart` | コンテナ再起動 |
| `/_api/projects/{name}/up` | プロジェクト起動（`docker compose up -d`） |
| `/_api/projects/{name}/down` | プロジェクト停止（`docker compose down`） |
| `/_api/projects/{name}/restart` | プロジェクト再起動（`docker compose restart`） |
| `/_api/projects/{name}/logs` | プロジェクトログ（SSEストリーム） |
| `/_api/projects/{name}/delete` | プロジェクト履歴削除 |
| `/_api/config/reload` | 設定ファイル再読み込み（静的サイト更新） |

## 実装済み機能（v0.1.0 → v0.9.0）

### コア機能
- ネットワークベースの自動検出（Docker Events監視）
- ホスト名/パスベースルーティング
- TLS証明書の自動生成（CA + ワイルドカード）
- HTTP → HTTPS リダイレクト
- WebSocketプロキシ（`Upgrade: websocket`ヘッダー検出、双方向通信）
- gRPCプロキシ（HTTP/2、`Content-Type: application/grpc`で自動検出）
- BASIC認証（Dockerラベル / 設定ファイルで設定）

### ダッシュボード
- リアルタイム更新（SSE + Petite Vue）
- プロジェクト履歴・クイックアクセス
- Docker Compose操作（Start/Stop/Restartボタン）
- リクエストログビューア（フィルタリング対応）
- ログエクスポート（JSON/CSV形式）
- コンテナ再起動ボタン
- ルート一覧からプロジェクト停止ボタン
- 非アクティブプロジェクトのURL直接アクセス時にStartボタン表示
- ダークモード（システム設定連動）
- 設定ミス警告表示
- 認証バッジ表示（🔒 auth）

### 開発支援
- リクエストモック（ラベルでレスポンス定義）
- 複数ネットワーク対応
- ブラウザ通知
- 静的ファイルホスティング（httpd機能）

### Native Mode (v0.8.0)
- 設定ファイル対応（`~/.config/roji/config.yaml`）
- XDG Base Directory準拠のパス管理
- `roji config` コマンド（show/path/init/edit）
- `roji doctor` コマンド（環境診断 + 自動修復）
- `roji ca` コマンド（install/uninstall/export/status）
- CA証明書のシステムインストール（macOS/Linux/Windows対応）
- 設定の優先順位: CLI > 環境変数 > 設定ファイル > デフォルト

### 運用強化 (v0.9.0)
- `roji service` コマンド（install/uninstall/start/stop/restart/status）
- サービス登録（Linux systemd / macOS launchd / Windows NSSM）
- ワンライナーインストーラー刷新（Native Mode対応）
  - GitHub Releasesからバイナリダウンロード
  - 対話式インストール先選択（`~/.local/bin` / `/usr/local/bin`）
  - Docker Mode検出 → 移行フロー
  - 自動セットアップ（doctor --fix, ca install, service install）
  - 旧Docker版は `install-docker.sh` として維持
- 設定ファイルバリデーション（不明キー警告、型チェック）
- Docker Compose操作（ダッシュボード/APIからup/down/restart/logs）

### 配布・品質
- ワンライナーインストール（Native Mode / アップグレード対応）
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
| **v1.0.0** | 安定版 | Docker Mode廃止、Homebrew対応、i18n、ドキュメント整備 |

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

### v0.8.0: Native Mode（単体バイナリ化）✅

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

- [x] 設定は以下の優先順位で適用（後が優先）：

1. デフォルト値
2. 設定ファイル（`~/.config/roji/config.yaml`）
3. 環境変数（`ROJI_*`）
4. コマンドライン引数（`--network` 等）

#### 設定ファイル仕様

- [x] **設定ファイル対応**

```yaml
# ~/.config/roji/config.yaml
network: roji
domain: dev.localhost
certs_dir: ~/.local/share/roji/certs
data_dir: ~/.local/share/roji
dashboard: dev.localhost
log_level: info
http_port: 80
https_port: 443
auto_cert: true
```

- [x] **config コマンド**
  - `roji config show` - 現在の設定を表示
  - `roji config path` - 設定ファイルパスを表示
  - `roji config init` - デフォルト設定ファイルを作成
  - `roji config edit` - エディタで設定を編集

#### roji doctor コマンド

- [x] **doctor コマンド**

```
$ roji doctor

✓ Docker daemon is running [pass]
  Docker daemon is running

✓ Docker socket is accessible [pass]
  /var/run/docker.sock

✗ Docker network [fail]
  Network(s) not found: roji
  Run 'docker network create roji' or use 'roji doctor --fix'

✓ Port availability [pass]
  Ports 80, 443 are available

...

Run 'roji doctor --fix' to auto-fix where possible
```

**チェック項目:**
- Docker daemon稼働
- Docker socketアクセス権
- 指定ネットワークの存在（修復可能）
- ポート80/443の利用可能性
- CA証明書の存在（修復可能）
- CA証明書のシステムインストール状態（修復可能、WSLではWindows側も確認）
- サーバー証明書の有効期限（修復可能）
- DNS解決（*.localhost）

**フラグ:**
- `--fix` - 修復可能な問題を自動修復
- `--json` - JSON形式で出力

#### CA証明書の自動インストール

- [x] **ca コマンド**

**新コマンド:**
- `roji ca status` - CA証明書のインストール状態を確認
- `roji ca install` - CA証明書をシステムにインストール
- `roji ca uninstall` - CA証明書を削除
- `roji ca export [path]` - CA証明書をエクスポート（PEM/DER形式）

**フラグ:**
- `--user` - ユーザーストアにインストール（macOS/Windows、sudo不要）
- `--firefox` - Firefoxにもインストール（Linux、nss-tools必要）
- `--windows` - WSLからWindows証明書ストアにインストール（常にユーザーストア）
- `--force` - 同名の証明書が存在しても強制インストール

**プラットフォーム別実装:**

| プラットフォーム | 方法 | 実装 |
|------------------|------|------|
| macOS | Keychain | `security add-trusted-cert` |
| Linux (Debian系) | update-ca-certificates | `/usr/local/share/ca-certificates/` |
| Linux (RHEL系) | update-ca-trust | `/etc/pki/ca-trust/source/anchors/` |
| Windows | certutil | `certutil -addstore -f "ROOT"` |
| WSL → Windows | certutil.exe | ユーザーストア（CurrentUser\ROOT）に登録 |

---

### v0.9.0: 運用強化

#### ログファイル出力と `roji log` コマンド

**ログファイル:**
- サービスモード時はログをファイルにも出力
- 場所:
  - Linux/WSL: `~/.local/share/roji/roji.log`
  - macOS: `~/Library/Logs/roji.log`（launchd経由の場合）
- ログローテーション: 10MB超過時に自動ローテート

**新コマンド:**
- `roji log` - ログをリアルタイム表示（`tail -f`相当）
- `roji log -n 100` - 最新100行を表示
- `roji log --no-follow` - 現在のログを表示して終了

#### サービスとしての稼働

**新コマンド:**
- `roji service install` - サービス登録
- `roji service uninstall` - サービス削除
- `roji service start/stop/restart` - サービス制御
- `roji service status` - 状態確認

**プラットフォーム別:**

| プラットフォーム | サービス管理 | 設定場所 | 状態 |
|------------------|--------------|----------|------|
| Linux | systemd | `/etc/systemd/system/roji.service` | ✅ 実装済み |
| macOS | launchd | `~/Library/LaunchAgents/com.roji.agent.plist` | ✅ 実装済み |
| Windows | NSSM | Windows Service経由 | ✅ 実装済み |

**Windows版の注意:**
- [NSSM](https://nssm.cc/)のインストールが必要
- 管理者権限で実行する必要あり
- ログは `%USERPROFILE%\.local\share\roji\roji.log` に出力

#### Docker Compose操作 ✅

ダッシュボード/APIからdocker-composeプロジェクトを操作:

| API | 説明 |
|-----|------|
| `POST /_api/projects/{name}/up` | `docker compose up -d` |
| `POST /_api/projects/{name}/down` | `docker compose down` |
| `POST /_api/projects/{name}/restart` | 全サービス再起動 |
| `GET /_api/projects/{name}/logs` | ログ取得（SSE） |

**実装:**
- `docker/compose.go`: `exec.CommandContext`で`docker compose`コマンドを実行
- `project.Store`に保存済みの`working_dir`と`config_files`を使用
- カンマ区切りの`config_files`を`-f`フラグに展開
- プロジェクト名バリデーション（`[a-zA-Z0-9_.\-]+`のみ許可）
- CORS対応（非ダッシュボードホストからのAPI呼び出し）

**ダッシュボード連携:**
- アクティブプロジェクト: Restart / Stop ボタン
- 非アクティブプロジェクト: Start ボタン（+ 既存のCopy start command）
- ルート一覧: プロジェクト所属のルートにStopボタン
- 確認ダイアログ付き（既存のコンテナ再起動と同じUIパターン）
- Not Foundページ: ホスト名から非アクティブプロジェクトを自動検出し、Startボタンを表示

#### 静的ファイルホスティング（httpd機能）✅

**設定ファイルで定義:**

```yaml
# ~/.config/roji/config.yaml
static_sites:
  - host: docs                    # -> docs.{ROJI_DOMAIN} (例: docs.dev.localhost)
    root: ~/projects/docs/build
    # index: true                 # デフォルト: ディレクトリ一覧有効
  - host: private.example.com     # ドットを含む場合はFQDNとして使用
    root: /var/www/private
    index: false                  # ディレクトリ一覧を無効化
```

**機能:**
- ホスト名解決: ドット(`.`)なし → `{host}.{ROJI_DOMAIN}` に展開
- ディレクトリ一覧: `index: true`（デフォルト）で有効、Apache/nginx風の一覧表示
- index.html自動配信: ディレクトリアクセス時
- ディレクトリトラバーサル防止: `path.Clean()` + root内チェック
- ダークモード対応: システム設定連動
- ダッシュボード連携: 設定再読み込み、index状態表示

**ダッシュボード機能:**
- 「Reload Config」ボタンで設定ファイルを再読み込み（`/_api/config/reload`）
- 静的サイトのindex状態を📋（有効）/🔒（無効）で表示

**未実装（後続で検討）:**
- `roji serve` コマンド（一時的なホスティング）

#### BASIC認証 ✅

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

#### ワンライナーインストーラー刷新 ✅

**新 install.sh（Native Mode対応）:**

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.9.0/install.sh | bash
```

**処理フロー:**
1. OS/Arch検出（Linux/macOS, x86_64/arm64）
2. 既存インストール検出
   - Docker Mode → 移行を促す（コンテナ停止、証明書バックアップ）
   - Native Mode → バージョン比較してアップグレード
3. 対話式インストール先選択
   - `~/.local/bin`（デフォルト、sudo不要でインストール）
   - `/usr/local/bin`（グローバル、sudoでインストール）
4. GitHub Releasesからバイナリダウンロード
5. `sudo roji doctor --fix`
6. `sudo roji ca install`（WSLでは`--windows`も）
7. `sudo roji service install && start`

**オプション:**
- `--local` / `--global` - インストール先指定
- `--upgrade` - アップグレード確認スキップ
- `--migrate` - Docker Mode移行確認スキップ
- `--no-service` - サービス登録スキップ

**旧版:**
- `install-docker.sh` として維持（Docker Mode用）

#### 設定ファイルバリデーション ✅

設定読み込み時およびdoctorコマンドで包括的なバリデーションを実行。

**チェック内容:**
- 不明なキーの検出（タイポ検出）
- 値の型チェック（string/int/bool/array/object）
- 必須フィールドの欠落（static_sitesのhost/root等）

**表示:**
- 設定読み込み時: `slog.Warn`で警告出力
- `roji doctor`: Config file validationチェックとして表示

---

### v1.0.0: 安定版リリース

#### 1. Docker Mode 廃止

- [x] `install-docker.sh` を削除
- [x] README.md から「Docker Mode (Legacy)」セクションを削除
- [x] `docker-compose.yml` の `ROJI_DOMAIN` デフォルトを `dev.localhost` に統一
- [x] Dockerfile、docker-compose.yml、docker-compose.dev.yml は開発・テスト用として維持
  - docker-compose.yml のコメントに「開発・テスト用」と明記

#### 2. ダッシュボード機能強化

- [x] **Recent Projects の管理（削除）機能**
  - 不要になったプロジェクト履歴をダッシュボードから削除
  - `DELETE /_api/projects/{name}/delete` エンドポイント追加
  - ダッシュボードに削除ボタン追加（確認ダイアログ付き）
  - `project.Store` から該当プロジェクトを削除

#### 3. 破壊的変更の整理

- [x] `ROJI_DOMAIN` のデフォルト値を全環境で `dev.localhost` に統一
  - docker-compose.yml が `localhost` を使用していた不整合を解消
- [ ] `roji.self` ラベルをドキュメント化（内部使用ラベルとして公式化）
- [x] 環境変数・設定パス・APIは現状維持（変更不要と確認済み）

#### 4. パッケージマネージャー対応

- [x] **Homebrew対応**（macOS）
  - Formulaの作成（GoReleaserの `brews` セクション利用）
  - `brew install kan/roji/roji` でインストール可能に
- [ ] APT/RPMパッケージは検討のみ（v1.x以降）

#### 5. 多言語対応 (i18n)

- [ ] **ダッシュボード (Web UI)**
  - メッセージの外部化（JSON / 埋め込みリソース）
  - ブラウザの `navigator.language` で言語自動検出
  - 日本語リソース追加
- [ ] **CLIコマンド**
  - メッセージ・ヘルプテキストの外部化
  - `LANG` 環境変数で言語自動検出
  - 日本語リソース追加
- [ ] **インストーラー (install.sh)**
  - メッセージの日本語対応

#### 6. ドキュメント整備

- [ ] **README.md 全面改訂**
  - Docker Mode関連の記述を削除・整理
  - Native Mode前提の記述に統一
  - 機能一覧を最新状態に更新
- [ ] **Getting Started ガイド**（新規作成）
  - インストールから初めてのサービス公開まで
  - 設定ファイルの使い方
  - よくある構成例
- [ ] **トラブルシューティングガイド**（拡充）
  - 既存セクションの拡充
  - プラットフォーム別の注意事項（WSL、macOS、Linux）
  - `roji doctor` との連携説明

#### 実装順序

1. 破壊的変更の整理 + Docker Mode廃止（影響範囲が広いため最初に）
2. ダッシュボード機能強化（Recent Projects削除機能）
3. パッケージマネージャー対応（Homebrew Formula）
4. 多言語対応 (i18n)
5. ドキュメント整備（最後にまとめて更新）

---

### 将来構想（v1.x〜）

| 機能 | 説明 | 優先度 |
|------|------|--------|
| VS Code拡張 | ルート一覧、ブラウザ起動、ログ連携（API安定後） | 高 |
| Docker Desktop ネットワーク対応 | macOS/Windows NativeのDocker Desktopではコンテナ内部IPへの直接アクセス不可（Linux VM経由のため）。ポートフォワーディング自動化や`host.docker.internal`経由のルーティングなど、Docker Desktop固有のネットワーク制約への対応。※WSL2+Docker Desktopは既に動作 | 中 |
| 簡易daemon化 | `roji start`でバックグラウンド起動（サービス登録不要）、PID管理 | 中 |
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
