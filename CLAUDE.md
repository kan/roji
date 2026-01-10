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
- **配布**: Docker イメージ (ghcr.io/kan/roji)

## アーキテクチャ

```
┌─────────────────────────────────────────────────────────────┐
│                         roji                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Docker     │  │    Route     │  │   HTTP/HTTPS     │  │
│  │   Watcher    │→ │   Manager    │→ │  Reverse Proxy   │  │
│  │              │  │              │  │                  │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│         ↓                                     ↑             │
│   Docker Socket                          :80 / :443         │
└─────────────────────────────────────────────────────────────┘
         ↓
┌─────────────────────────────────────────────────────────────┐
│  Shared Network: "roji" (or custom)                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │ web        │  │ api        │  │ app        │            │
│  │ :80        │  │ :8080      │  │ :3000      │            │
│  │ web.       │  │ api.       │  │ app.       │            │
│  │ localhost  │  │ localhost  │  │ localhost  │            │
│  └────────────┘  └────────────┘  └────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

## ディレクトリ構造

```
roji/
├── cmd/
│   └── roji/
│       ├── main.go           # エントリーポイント
│       └── cmd/              # Cobraコマンド
│           ├── root.go       # ルートコマンド（サーバー起動）
│           ├── routes.go     # ルート一覧コマンド
│           ├── version.go    # バージョン表示コマンド
│           ├── health.go     # ヘルスチェックコマンド
│           └── server.go     # サーバー実装
├── docker/
│   ├── client.go             # Docker API ラッパー
│   └── watcher.go            # Events 監視
├── proxy/
│   ├── handler.go            # ReverseProxy 実装
│   ├── router.go             # ホスト名/パスルーティング
│   └── templates/            # HTMLテンプレート・静的アセット
│       ├── dashboard.html
│       ├── notfound.html
│       ├── warning.html      # 設定ミス警告ページ
│       └── theme.css         # 共通テーマ（CSS変数、ダークモード）
├── certgen/
│   └── generator.go          # TLS証明書生成
├── config/
│   └── labels.go             # ラベルパーサー
├── certs/                    # 生成された証明書（gitignore）
├── examples/
│   └── docker-compose.yml    # ユーザー向けサンプル
├── Dockerfile                # マルチステージ（development + production）
├── docker-compose.yml        # 開発用（air でホットリロード）
├── .air.toml                 # ホットリロード設定
├── .env.example              # 環境変数テンプレート
├── go.mod
├── go.sum
├── README.md
├── CLAUDE.md                 # 開発ガイド
└── CONTRIBUTING.md           # 開発環境セットアップ
```

## 設計仕様

### サービスディスカバリー

1. **ネットワークベース**: `roji` ネットワークに接続されたコンテナを自動検出
2. **ポート検出**: `EXPOSE` されたポートを使用（複数ある場合は最初のもの、またはラベルで指定）
3. **ホスト名生成**: `com.docker.compose.service` ラベル → `{service}.{base_domain}`

### ラベル仕様

| ラベル | 説明 | デフォルト |
|--------|------|-----------|
| `roji.host` | カスタムホスト名 | `{service}.localhost` |
| `roji.port` | ターゲットポート | 最初のEXPOSEポート |
| `roji.path` | パスプレフィックス | なし |

**例:**
```yaml
services:
  api:
    image: my-api
    labels:
      - "roji.host=api.myproject.localhost"
      - "roji.port=3000"
    networks:
      - roji
```

### 環境変数 / フラグ

| 環境変数 | フラグ | 説明 | デフォルト |
|----------|--------|------|-----------|
| `ROJI_NETWORK` | `-network` | 監視するDockerネットワーク（カンマ区切りで複数指定可） | `roji` |
| `ROJI_DOMAIN` | `-domain` | ベースドメイン | `localhost` |
| `ROJI_HTTP_PORT` | `-http-port` | HTTPポート（リダイレクト用） | `80` |
| `ROJI_HTTPS_PORT` | `-https-port` | HTTPSポート | `443` |
| `ROJI_CERTS_DIR` | `-certs-dir` | 証明書ディレクトリ | `/certs` |
| `ROJI_AUTO_CERT` | `-auto-cert` | 証明書自動生成 | `true` |
| `ROJI_DASHBOARD` | `-dashboard` | ダッシュボードホスト名 | `roji.localhost` |
| `ROJI_LOG_LEVEL` | `-log-level` | ログレベル | `info` |

## 実装の注意点

### Docker API

```go
// クライアント作成時は必ず API バージョンネゴシエーションを有効に
client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

// コンテナのネットワーク内IPを取得
network := container.NetworkSettings.Networks[networkName]
ip := network.IPAddress  // これを使ってプロキシ

// EXPOSEポートの取得
for portSpec := range container.Config.ExposedPorts {
    // portSpec は "3000/tcp" 形式
    portStr := strings.Split(string(portSpec), "/")[0]
}

// docker-compose のサービス名
serviceName := container.Config.Labels["com.docker.compose.service"]
```

### ReverseProxy

```go
// httputil.ReverseProxy を使用
proxy := httputil.NewSingleHostReverseProxy(targetURL)

// Director でヘッダーを設定
proxy.Director = func(req *http.Request) {
    // X-Forwarded-* ヘッダーを設定
    req.Header.Set("X-Forwarded-Host", originalHost)
    req.Header.Set("X-Forwarded-Proto", "https")
    
    // パスプレフィックスを除去（パスベースルーティングの場合）
    if pathPrefix != "" {
        req.URL.Path = strings.TrimPrefix(req.URL.Path, pathPrefix)
    }
}

// エラーハンドリング
proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
    // 502 Bad Gateway を返す
}
```

### 証明書生成

```go
// CA証明書の生成
ca := &x509.Certificate{
    SerialNumber: big.NewInt(1),
    Subject: pkix.Name{
        Organization: []string{"roji Dev CA"},
        CommonName:   "roji CA",
    },
    NotBefore:             time.Now(),
    NotAfter:              time.Now().AddDate(10, 0, 0), // 10年
    IsCA:                  true,
    KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
    BasicConstraintsValid: true,
}

// サーバー証明書（ワイルドカード対応）
serverCert := &x509.Certificate{
    DNSNames: []string{
        "*." + baseDomain,  // *.kan.localhost
        baseDomain,         // kan.localhost
        "localhost",
    },
    // ...
}
```

### ルーティング

```go
// ホスト名の正規化（ポート除去、小文字化）
func normalizeHost(host string) string {
    if idx := strings.LastIndex(host, ":"); idx != -1 {
        host = host[:idx]
    }
    return strings.ToLower(host)
}

// パスベースルーティングは longest match 優先
// /api/v2 → /api より優先
```

### 動的更新

```go
// Docker Events API でコンテナの開始/停止を監視
filterArgs := filters.NewArgs()
filterArgs.Add("type", "container")
filterArgs.Add("event", "start")
filterArgs.Add("event", "stop")
filterArgs.Add("event", "die")

msgCh, errCh := dockerClient.Events(ctx, events.ListOptions{
    Filters: filterArgs,
})

// イベント発生時にルートを更新
// start → GetBackend() → AddBackend()
// stop/die → RemoveBackend()
```

## 機能実装チェックリスト

### Phase 1: コア機能 ✅

- [x] Docker API でコンテナ一覧取得
- [x] ラベルからルート情報抽出
- [x] httputil.ReverseProxy でプロキシ
- [x] ホスト名ベースのルーティング
- [x] ベースドメイン設定の可変化（`ROJI_DOMAIN`）

### Phase 2: 動的更新 ✅

- [x] Docker Events 監視
- [x] ルートの動的追加/削除
- [x] graceful shutdown

### Phase 3: TLS ✅

- [x] 証明書の読み込み（外部生成 / mkcert）
- [x] 証明書の自動生成（mkcert不要、CA含む）
  - [x] CA証明書生成（ECDSA P-256、10年有効）
  - [x] サーバー証明書生成（ワイルドカード対応、1年有効）
  - [x] Windows用DER形式（.crt）出力
- [x] HTTP → HTTPS リダイレクト

### Phase 4: 品質 ✅

- [x] ユニットテスト実装
  - [x] config/labels: 100% カバレッジ
  - [x] certgen/generator: 60% カバレッジ
  - [x] proxy/router: テスト済み
  - [x] proxy/handler: テスト済み
  - [x] docker/client: 69.7% カバレッジ（DockerAPIインターフェース導入）
  - [x] docker/watcher: テスト追加
  - **プロジェクト全体: 48.2% カバレッジ** (30.3% → 48.2%)
- [x] リファクタリング
  - [x] プロジェクト構造の整理（`cmd/roji/` 導入、`internal/` 削除）
  - [x] パッケージ名の改善（`internal/certs` → `certgen`）
  - [x] DockerAPIインターフェース導入（テスタビリティ向上）
  - [x] docker/client の複雑度削減（`buildProjectServiceCounts` ヘルパー導入）
  - [x] HTMLテンプレート分離（embed.FS 使用、proxy/handler.go 306行→176行に削減）
  - [x] main.go の関数分割（サーバー管理・イベント処理を分離、モジュール性向上）

### Phase 5: 配布 ✅

- [x] GitHub Actions CI/CD
  - [x] テスト実行
  - [x] ビルド確認
  - [x] Docker イメージビルド確認
  - [x] ghcr.io への自動プッシュ（タグ時）
    - `v*` タグで自動ビルド＆プッシュ
    - semver タグ生成（v1.2.3 → 1.2.3, 1.2, 1）
    - latest タグ自動更新
- [x] install.sh（curl | bash でインストール）
  - Docker/Docker Compose の前提条件チェック
  - デフォルト設定でのインストール（`~/.roji`）
  - 自動証明書生成とCA登録案内（OS別、英語）
  - ワンライナーインストール対応

### Phase 6: 便利機能 ✅

- [x] ダッシュボード（ルート一覧表示）
- [x] CLI ルート一覧表示（`roji routes`）
  - `/_api/routes` エンドポイント追加
  - Cobra導入によるCLI構造化
  - サブコマンド: routes, version, health, help, completion
- [x] ヘルスチェックエンドポイント
  - `/_api/health` (API規約準拠)
  - `/healthz` (Kubernetes/Docker標準)
  - Dockerfile HEALTHCHECK 設定
- [x] ステータスエンドポイント（`/_api/status`）
  - 証明書有効期限チェック（CA・サーバー証明書）
  - Docker接続状態
  - プロキシ設定情報（ルート数、ドメイン、ポート）
  - アップタイム
  - 総合ヘルスステータス（healthy/degraded/unhealthy）

### Phase 7: セキュリティ・パフォーマンス改善 ✅

- [x] セキュリティ強化
  - [x] Distroless イメージへの移行（gcr.io/distroless/static:nonroot）
  - [x] X-Forwarded ヘッダーのスプーフィング対策
  - [x] パストラバーサル防止（roji.path ラベル）
  - [x] Docker client ライブラリ更新（v28.5.2）
  - [x] GitHub Actions セキュリティスキャン追加（govulncheck, Trivy, hadolint）
- [x] パフォーマンス改善
  - [x] SSE 対応（FlushInterval = -1）
  - [x] 共有 Transport によるコネクションプーリング
  - [x] Docker Events 自動再接続
- [x] 堅牢性向上
  - [x] サーバータイムアウトの明示的設定
  - [x] Docker API タイムアウト追加
  - [x] `roji health` コマンド追加（Distroless 対応）

### Phase 8: リリース自動化 ✅

- [x] GoReleaser 導入
  - [x] GitHub Release の自動管理
  - [x] `roji version` コマンドで正しいバージョンを返す（ldflags対応）
  - [x] マルチプラットフォームビルド（Linux/macOS/Windows、amd64/arm64）
  - [x] マルチアーキテクチャDockerイメージ
  - [x] ビルドメタデータ（version, commit, date, built by）の埋め込み

### Phase 9: インタラクティブダッシュボード ✅

- [x] Petite Vue 導入
  - [x] petite-vue.min.js を `proxy/templates/` に同梱（オフライン対応）
  - [x] 既存の Go テンプレートを Petite Vue 対応に変換（デリミタ `[[` `]]`）
  - [x] ビルドステップ不要、`embed.FS` 構成を維持
- [x] SSE（Server-Sent Events）によるリアルタイム更新
  - [x] `/_api/events` エンドポイント追加
  - [x] Router に Pub/Sub 機能追加（Subscribe/notifySubscribers）
  - [x] Docker Events → Router更新 → SSE broadcast の連携
  - [x] 接続時に現在のルート一覧を即座に送信
- [x] ダッシュボードUI更新
  - [x] EventSource によるSSE接続
  - [x] ルート変更時のリアクティブ更新（Petite Vue）
  - [x] 接続状態インジケーター（Live / Disconnected）
  - [x] 自動再接続対応（EventSource標準動作）
  - [x] 追加/削除時のアニメーション（CSS transitions）
- [x] Web Notification 通知
  - [x] 通知許可リクエストUI
  - [x] ルート追加時「{hostname} added」通知
  - [x] ルート削除時「{hostname} removed」通知
  - [x] 通知ON/OFF設定（localStorage保存）

**技術選定: Petite Vue**
- Vue作者（Evan You）製の軽量版（~6KB gzip）
- ビルド不要、CDNまたは単一JSファイル同梱
- Vue構文（v-for, v-if, v-on）がそのまま使える
- 将来フルVue 3への移行も容易（構文互換）

**実装イメージ:**
```html
<script src="/_assets/petite-vue.min.js" defer init></script>

<div v-scope="Dashboard()">
  <span v-if="connected">🟢 Live</span>
  <span v-else>🔴 Disconnected</span>

  <div v-for="route in routes" class="route">
    <a :href="'https://' + route.hostname">{{ route.hostname }}</a>
  </div>
</div>

<script type="module">
function Dashboard() {
  return {
    routes: [],
    connected: false,
    init() {
      const es = new EventSource('/_api/events')
      es.onopen = () => this.connected = true
      es.addEventListener('routes', (e) => {
        this.routes = JSON.parse(e.data)
      })
    }
  }
}
PetiteVue.createApp({ Dashboard }).mount()
</script>
```

**アーキテクチャ:**
```
Docker Events → Watcher → Router.Update() → Router.notifySubscribers()
                                                      ↓
                                              SSE endpoint (/_api/events)
                                                      ↓
                                              Dashboard (Petite Vue + EventSource)
                                                      ↓
                                              リアクティブUI更新 + Web Notification
```

**将来の拡張:**
- ルートごとのヘルスチェック表示
- フィルタリング・検索機能

### Phase 9.5: プロジェクト履歴・クイックアクセス ✅

- [x] プロジェクト情報の収集
  - [x] `com.docker.compose.project` からプロジェクト名取得
  - [x] `com.docker.compose.project.working_dir` から作業ディレクトリ取得
  - [x] `com.docker.compose.project.config_files` から設定ファイルパス取得
  - [x] 最終起動日時の記録
- [x] プロジェクト履歴の永続化
  - [x] JSON ファイルで保存（`/data/projects.json`）
  - [x] プロジェクト追加・更新のタイミング管理
- [x] ダッシュボードUI
  - [x] 現在稼働中のプロジェクト一覧
  - [x] 過去に接続したプロジェクト一覧（停止中）
  - [x] 起動コマンドのコピーボタン
    - `cd /path/to/project && docker compose up -d`
  - [x] プロジェクトごとのサービス数表示
- [x] API エンドポイント
  - [x] `/_api/projects` - アクティブ/非アクティブプロジェクト一覧
  - [x] ルート更新時にプロジェクト情報も自動更新
- [x] ベースドメインからダッシュボードホストへのリダイレクト
  - [x] 302リダイレクト実装（dev.localhost → roji.dev.localhost）
  - [x] パス・クエリ文字列の保持
  - [x] ブックマーク統一の実現

**ダッシュボード表示イメージ:**
```
┌─ Active Projects ────────────────────────────────┐
│ 🟢 myapp (3 services)                            │
│    web.localhost, api.localhost, db.localhost    │
└──────────────────────────────────────────────────┘

┌─ Recent Projects ────────────────────────────────┐
│ ⚫ backend-api (停止中)              [📋 Copy]   │
│    /home/user/backend-api                        │
│    Last active: 2 hours ago                      │
│                                                  │
│ ⚫ frontend (停止中)                 [📋 Copy]   │
│    /home/user/frontend                           │
│    Last active: 1 day ago                        │
└──────────────────────────────────────────────────┘
```

**将来の拡張（操作機能）:**
- Docker Compose Go SDK（github.com/docker/compose/v2）統合
- ダッシュボードからの起動/停止操作
- ※ rojiがDockerコンテナ外で動作する場合に実現可能

### v0.4.0 リリース準備 ✅

- [x] examples/ ディレクトリ削除（install.shで自動生成されるため不要）
- [x] README.md 更新
  - [x] Phase 9.5機能の追記（プロジェクト履歴、リダイレクト）
  - [x] Manual Installation手順の改善（リポジトリclone方式）
  - [x] Dashboard セクションの拡充
  - [x] 環境変数テーブルに `ROJI_DATA_DIR` 追加
- [x] CLAUDE.md 更新
  - [x] Phase 9.5を完了マーク
  - [x] v0.4.0リリースタスク整理
- [x] install.sh の更新
  - [x] `ROJI_DATA_DIR` 環境変数の追加
  - [x] データディレクトリ（`/data`）のボリュームマウント追加
  - [x] 生成される docker-compose.yml の更新（roji.dev.localhost）
- [x] CHANGELOG.md 作成（v0.4.0の変更内容）

**リリース手順（v0.5.0の場合）:**

1. **ドキュメント更新**
   - `README.md`: install.sh URLを新バージョンに更新（2箇所）
     ```bash
     v0.4.0 → v0.5.0
     ```
   - `CHANGELOG.md`: 新バージョンの変更内容を追加（降順、最新版が上）

2. **変更をコミット**
   ```bash
   git add README.md CHANGELOG.md
   git commit -m "Prepare for vX.Y.Z release"
   git push origin main
   ```

3. **タグ作成とプッシュ**
   ```bash
   git tag -a vX.Y.Z -m "Release vX.Y.Z: [主要機能の説明]"
   git push origin main
   git push origin vX.Y.Z
   ```

4. **自動リリース確認**
   - GitHub Actions: https://github.com/kan/roji/actions
   - GitHub Release: https://github.com/kan/roji/releases/tag/vX.Y.Z
   - Docker Image: `ghcr.io/kan/roji:X.Y.Z`

5. **リリース後の確認**
   - pkg.go.dev が自動更新されるまで待つ（通常数分）
   - ドキュメントリンクが正しく動作することを確認

### Phase 10: テスト強化・アップグレード対応（v0.6.0）

- [ ] インテグレーションテスト
  - [ ] Docker Compose を使った実環境テスト
  - [ ] テスト用 docker-compose.yml の作成（`test/` ディレクトリ）
  - [ ] コンテナ起動 → ルート検出 → プロキシ動作の一連のフロー検証
  - [ ] ポートなし/ホスト名競合など警告ケースのテスト
  - [ ] GitHub Actions での自動実行（Docker-in-Docker または services）
- [ ] E2Eテスト
  - [ ] 実際のHTTPリクエストを使った動作検証
  - [ ] ダッシュボードのアクセス確認
  - [ ] SSE接続・リアルタイム更新の検証
  - [ ] TLS証明書の検証（自己署名CA）
- [ ] install.sh アップグレード対応
  - [ ] 既存インストールの検出（`$ROJI_INSTALL_DIR/docker-compose.yml` の存在確認）
  - [ ] バージョン確認（現在のイメージタグ取得）
  - [ ] アップグレードモード（`--upgrade` フラグまたは自動検出）
  - [ ] docker-compose.yml のイメージタグ更新
  - [ ] `docker compose pull && docker compose up -d` の実行
  - [ ] 設定ファイル（docker-compose.yml）のマイグレーション
    - 新しい環境変数の追加
    - 非推奨設定の警告
  - [ ] ロールバック手順の案内

**インテグレーションテスト構成案:**
```
test/
├── docker-compose.yml      # テスト用サービス群
├── integration_test.go     # Goテストコード
└── testdata/
    └── expected/           # 期待値ファイル
```

**テストシナリオ:**
1. 基本フロー: コンテナ起動 → ダッシュボードでルート確認 → プロキシアクセス
2. 動的更新: コンテナ追加/削除 → SSEイベント受信 → ルート更新確認
3. 警告ケース: ポートなしコンテナ → 警告表示確認
4. 競合ケース: 同一ホスト名 → 競合警告確認
5. TLS: HTTPS接続 → 証明書検証

**install.sh アップグレードフロー:**
```
$ curl -fsSL .../install.sh | bash

Existing roji installation detected at ~/.roji
Current version: 0.5.0
Latest version: 0.6.0

Upgrading roji...
✓ Pulled new image
✓ Restarted roji
✓ Upgrade complete!

roji is now running at https://roji.dev.localhost
```

### Phase 10.5: UX改善・開発支援機能（v0.6.0）

- [x] ダークモード対応
  - [x] CSS変数によるテーマ切り替え（`theme.css` 外部ファイル化）
  - [x] localStorage で設定保存
  - [x] システム設定の自動検出（prefers-color-scheme）
  - [x] ダッシュボードにトグルボタン追加（太陽/月アイコン）
  - [x] 全ページ対応（dashboard, notfound, warning）
  - [x] FOUC防止（ページ読み込み前にテーマ適用）
- [ ] リクエストログビューア
  - [ ] リングバッファでログ保持（直近100件程度）
  - [ ] SSEで新規リクエストをリアルタイム配信
  - [ ] ダッシュボードにログパネル追加
  - [ ] メソッド、パス、ステータス、レイテンシ、ホスト名を表示
  - [ ] フィルタリング機能（ホスト名、ステータスコード）
- [x] 複数ネットワーク対応
  - [x] `ROJI_NETWORK` でカンマ区切り複数指定
  - [x] 各ネットワークを並行して監視
  - [x] ダッシュボードでネットワーク別表示（バッジクリックでフィルタリング）
- [ ] コンテナ再起動ボタン
  - [ ] ダッシュボードの各ルートに再起動ボタン追加
  - [ ] Docker API `ContainerRestart` 呼び出し
  - [ ] `/_api/containers/{id}/restart` エンドポイント
  - [ ] 確認ダイアログ表示
- [x] シンプルなリクエストモック
  - [x] `roji.mock.{METHOD}.{PATH}` ラベルでレスポンス定義
  - [x] JSON/テキストレスポンスのサポート
  - [x] ステータスコード指定（`roji.mock.status.{METHOD}.{PATH}=201`）
  - [x] バックエンド未実装時のフロントエンド開発支援

**リクエストログビューア実装イメージ:**
```
┌─ Request Log ─────────────────────────────────────────────┐
│ 🟢 GET  /api/users      200  45ms  api-myapp.localhost    │
│ 🔴 POST /api/login      401  12ms  api-myapp.localhost    │
│ 🟢 GET  /assets/app.js  200   3ms  web-myapp.localhost    │
│ 🟡 GET  /api/slow       200 2.1s   api-myapp.localhost    │
└───────────────────────────────────────────────────────────┘
```

**モック機能の使用例:**
```yaml
services:
  mock-api:
    image: alpine
    command: ["sleep", "infinity"]
    labels:
      - "roji.host=api.localhost"
      - "roji.mock.GET./api/users=[{\"id\":1,\"name\":\"Test\"}]"
      - "roji.mock.GET./api/health={\"status\":\"ok\"}"
      - "roji.mock.status.POST./api/users=201"
    networks:
      - roji
```

### 将来の課題

（検討中）

## 出力ファイル仕様

### 証明書ディレクトリ構成

```
certs/
├── ca.pem          # CA証明書（ユーザーがブラウザにインストール）
├── ca-key.pem      # CA秘密鍵
├── cert.pem        # サーバー証明書（rojiが使用）
└── key.pem         # サーバー秘密鍵
```

### 起動時の案内表示

```
╔═══════════════════════════════════════════════════════════════╗
║  🛤️  roji - ローカル開発用リバースプロキシ                      ║
║                                                               ║
║  📁 CA証明書: ./certs/ca.pem                                   ║
║  ブラウザにインストールしてHTTPSを信頼してください                ║
║                                                               ║
║  macOS:   open ./certs/ca.pem → キーチェーンで「常に信頼」      ║
║  Linux:   ブラウザの証明書設定からインポート                     ║
║  Windows: ca.pem をダブルクリック → 証明書のインストール          ║
╚═══════════════════════════════════════════════════════════════╝

📋 Registered Routes:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  https://api.kan.localhost/ -> 172.18.0.3:3000 (api)
  https://web.kan.localhost/ -> 172.18.0.4:80 (web)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## テスト方法

### 手動テスト用 docker-compose.yml

```yaml
# test/docker-compose.yml
services:
  # 基本ケース: 自動検出
  web:
    image: nginx:alpine
    networks:
      - roji

  # カスタムホスト名
  api:
    image: nginx:alpine
    labels:
      - "roji.host=api.myapp.localhost"
    networks:
      - roji

  # ポート指定
  app:
    image: node:20-alpine
    command: ["npx", "http-server", "-p", "3000"]
    expose:
      - "3000"
      - "9229"
    labels:
      - "roji.port=3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

### 確認コマンド

```bash
# ネットワーク作成
docker network create roji

# roji 起動
docker compose up -d

# テストサービス起動
cd test && docker compose up -d

# 動作確認
curl -k https://web.localhost
curl -k https://api.myapp.localhost

# ルート確認
docker logs roji
```

## よくある問題と解決策

### `.localhost` ドメインが解決できない

- **macOS**: 自動で `127.0.0.1` に解決される
- **Linux**: `/etc/hosts` に追加、または `*.lvh.me` を使用

### コンテナが検出されない

1. `docker network inspect roji` でネットワーク確認
2. `docker inspect <container> | jq '.[0].Config.ExposedPorts'` でポート確認
3. `roji.self=true` ラベルがついていないか確認

### 証明書エラー

CA証明書がブラウザ/OSにインストールされているか確認

## 参考リンク

- [Docker Engine API](https://docs.docker.com/engine/api/)
- [Go httputil.ReverseProxy](https://pkg.go.dev/net/http/httputil#ReverseProxy)
- [Traefik](https://doc.traefik.io/traefik/) - 設計の参考
