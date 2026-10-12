# CLAUDE.md - roji 開発ガイド

## プロジェクト概要

**roji**（路地）は、ローカル開発環境専用のシンプルなリバースプロキシ。Docker Composeのサービスを自動検出し、`*.localhost` でHTTPSアクセスを可能にする。

> "本番は高速道路（Traefik）、開発は路地裏（roji）で"

### 言語ルール

- 日本語で書くもの: CLAUDE.md（開発者向け内部ドキュメント）、`website/content/ja/`、コミットメッセージ
- 英語で書くもの: 上記以外（README.md、CONTRIBUTING.md、CHANGELOG.md、ソースコード、コメント、`website/content/en/`）

### コンセプト

- **Traefikより軽量・シンプル**: ローカル開発に特化することで複雑な機能を省略
- **ネットワークベースの自動検出**: 共有ネットワークに接続 = プロキシ対象
- **ゼロコンフィグ志向**: ラベルなしでも動作、必要に応じてラベルでカスタマイズ

## 技術スタック

- **言語**: Go 1.27+
- **主要ライブラリ**:
  - `github.com/moby/moby/client` / `github.com/moby/moby/api` - Docker API
  - `net/http/httputil` - ReverseProxy（標準ライブラリ）
  - `crypto/x509`, `crypto/tls` - 証明書生成（標準ライブラリ）
- **フロントエンド**: Petite Vue（~6KB、ビルド不要）
- **配布**: ネイティブバイナリ（GitHub Releases / install.sh / Homebrew）。
  Docker イメージ (ghcr.io/kan/roji) は開発・テスト用

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
│   └── cmd/                  # Cobraコマンド（root, routes, version, health, server, config, doctor, ca, service, log, credits, run）
├── docker/
│   ├── client.go            # Docker API ラッパー
│   ├── compose.go           # Docker Compose CLI実行（up/down/restart/logs）
│   └── watcher.go           # Events 監視
├── proxy/
│   ├── handler.go           # ReverseProxy 実装
│   ├── router.go            # ルーティング + SSE Pub/Sub
│   ├── tunnel.go            # トンネルリスナーのガード
│   ├── lease.go             # roji run が登録する期限付きルート
│   ├── lease_api.go         # 期限付きルートの API（/_api/leases）
│   └── templates/           # HTML/CSS/JS（embed.FS）
├── certgen/
│   ├── generator.go         # TLS証明書生成
│   ├── installer.go         # CAInstaller インターフェース
│   ├── installer_darwin.go  # macOS Keychain対応
│   ├── installer_linux.go   # Linux (Debian/RHEL) 対応
│   ├── installer_windows.go # Windows certutil対応
│   └── installer_wsl.go     # WSL→Windows対応
├── tunnel/
│   └── cloudflared.go       # cloudflared の設定生成とプロセス管理
├── config/
│   ├── labels.go            # ラベルパーサー
│   ├── paths.go             # XDGパスユーティリティ
│   ├── settings.go          # 設定ファイル読み込み
│   └── sites.go             # 宣言したサイト（static_sites / proxy_sites）の解決
├── doctor/
│   ├── check.go             # Doctor インターフェース
│   └── checks/              # 各チェック実装
├── service/                 # サービス登録（systemd / launchd / NSSM、ビルドタグで分割）
├── project/                 # プロジェクト履歴（project.Store）
├── i18n/                    # CLI / ダッシュボードのメッセージ
├── apiclient/               # CLI から稼働中の roji API を呼ぶクライアント
├── test/                    # インテグレーション/E2Eテスト
├── credits.go               # LICENSE / CREDITS / THIRD_PARTY_NOTICES の埋め込み（roji credits）
├── CREDITS                  # 依存 Go モジュールのライセンス（gocredits で生成、コミットする）
├── THIRD_PARTY_NOTICES      # gocredits が拾わないもの（Go モジュールでない Petite Vue）
├── Dockerfile               # マルチステージビルド
├── docker-compose.yml       # 開発・テスト用（Air ホットリロード。本番はネイティブバイナリ）
├── .air.toml                # Air ホットリロード設定
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
| `roji.tunnel` | トンネルで外部公開する（`true` / `1` / `yes` / `on`） | なし（非公開） |
| `roji.self` | 予約済み: コンテナをルーティング対象から除外（内部使用） | なし |

`roji.path` の挙動（v1.1.0 で修正）:

- プレフィックスはパスのセグメント境界でのみ一致する。`/api` は `/api` と
  `/api/users` を捕まえるが、`/apifoo` や `/api-docs` は捕まえない
- `roji.path=/` はラベルを書かないのと同じ。`PathPrefix` は空文字列に正規化
  され、「プレフィックスなし」の綴りが 1 つに保たれる

### ルーティングの優先順位

ホスト名を複数のものが主張したときの規則（v1.1.1 で明確化）。

1. **設定ファイルで宣言したルート**（静的サイトとローカルプロキシ。以下
   「宣言ルート」）が最優先。Docker ルートは自動検出されるものなので、後から
   起動したコンテナが既に宣言済みのホスト名を奪えない。隠れた Docker ルートは
   警告ログに出る。宣言ルートのキーはホスト名だけで、パスプレフィックスは
   持たない。同じホスト名を 2 回宣言したら後の宣言が勝つ（`static_sites` →
   `proxy_sites` の順に読む）
2. **Docker ルート同士**はルーティングキー（ホスト名 + パスプレフィックス）で
   衝突を判定する。同じキーを主張したら後から来たものが勝ち、`Backend.Warning` に
   記録されてダッシュボードに出る。同じホスト名でもプレフィックスが違えば衝突しない

**「宣言 > 検出」の順序は `Router.Resolve` の 1 か所で実装している。**
ハンドラもトンネルのガードも `Resolve` を呼ぶこと。`Lookup`（Docker ルート
だけを引く）を直接呼ぶと、宣言ルートが隠している Docker ルートを見てしまう。
ルートの種別を増やすときは、優先順位の段を足さず、`Route` のバックエンドを
増やして宣言ルートの層に入れる。宣言ルートの層は `Router.declaredOn` が引く
もので、中身は設定ファイル由来の `declaredRoutes` と、`roji run` が登録する
`leases` の 2 つ。マップが分かれているのは、消え方が違うため（前者は設定の
再読み込みで入れ替わり、後者は期限で消える）で、優先順位の段ではない。

`Backend.Warning` の文言は `handler.go` の `handleRouteWarning` が
`strings.Contains` で見て警告ページの種別を決めている。`"hostname conflict"`
という書き出しは変えないこと（変えると分類が unknown に落ちる）。

### 外部公開（トンネル）

v1.2.0 で追加。Cloudflare Tunnel を通して選んだルートだけを外部から
到達可能にする。設定は `tunnel:` ブロック（後述）と `roji.tunnel` ラベル。

**なぜ専用ポートなのか。** cloudflared は 127.0.0.1 から接続してくるので、
リクエストの中身ではローカルのブラウザと区別がつかない。区別できるのは
到達したポートだけ。だから通常の `bind` リスナーとは別に
`127.0.0.1:{tunnel.port}`（既定 8080）を開き、そこに来たものだけを
「外から来た」として扱う。この分離がガードの成立条件そのものなので、
トンネルを既存リスナーに相乗りさせないこと。

**ガード（`proxy/tunnel.go` の `TunnelHandler`）が落とすもの:**

1. `/_api/*` と `/_assets/*` は**パスだけで無条件 404**。ホスト名や設定で
   開けられないようにしてある。`/_api/projects/{name}/up` は無認証で
   コンテナを起動でき、`/_api/logs` はリクエストログを読める
2. ダッシュボードホストとベースドメイン
3. 公開を宣言していないルート（**明示オプトイン**）。Docker ルートは
   `roji.tunnel=true` ラベル、宣言ルートは設定ファイルの `tunnel: true`。
   判定は `Route.Published()` の 1 本。roji はネットワーク上のコンテナを
   ラベル無しで拾うので、既定で公開すると全部が外に出る

拒否は理由によらず同じ素の 404 を返す。文言を分けるとどのホスト名が
存在するかを教えてしまう。

ガードを通った後も同じ規律が要る。ガードと `Handler` はそれぞれ独立に
`Resolve` するので、その間にコンテナが消えたり設定が再読み込みされたりすると、
ガードが見たものと違うルートに当たる。`Handler` 側で次の 4 つを守ること
（いずれも `isTunneled` で分岐している）。

- ルートが無ければ素の 404。not found ページは全ルート（ホスト名・コンテナ名・
  プロジェクト名・バックエンドの ip:port）を列挙する
- 見つかったルートが `Published()` でなければ素の 404。ガードが承認したのは
  別のルートだったことになる
- ローカルプロキシの転送先に繋がらないときも素の 404。案内ページは転送先の
  アドレスを表示する

- Docker ルートに `Warning` が付いているときも素の 404。警告ページは
  サービス名とダッシュボードのホスト名を表示する

**roji が自分で生成するページを足すときは、トンネル由来のリクエストに何を
返すかを必ず決めること。** ガードを通ったリクエストも `Handler` の同じ分岐を
通るので、何も決めなければ、ローカルの開発者向けに書いたページがそのまま
外へ出る。

静的サイトは、トンネル由来のときだけ次のように振る舞う
（`Handler.serveStaticSite`）。

- 生成するページ（ディレクトリ一覧など）にダッシュボードへのリンクを出さない
- `.` で始まる要素を含むパスは素の 404 にし、ディレクトリ一覧にも出さない
  （`.env` や `.git/` を公開しないため）。`.well-known/` だけは配信する

静的サイトは GET と HEAD にしか応答しない（`ServeStaticFile`）。
`checkBasicAuth` は CORS のプリフライトのために OPTIONS を素通しするので、
メソッドを絞らないと認証付きの静的サイトのファイルが OPTIONS で読める。

トンネル用ポートが `http_port` / `https_port` と衝突したらトンネルを開かない。
どちらのリスナーが勝つかは `bind` 次第で、`bind` に 127.0.0.1 が無ければ
トンネル側が勝ち、ローカルのブラウザがガードに当たることになる。

**ホスト名の対応:** `web.dev.localhost` ⇔ `web.example.com`。接尾辞だけを
入れ替える（`proxy.TunnelDomains`）。ルーティングキーはローカル名のままなので、
ルートの保存・照合・表示はトンネルの有無で変わらない。バックエンドに渡す
Host もローカル名（dev サーバーの host 許可リストがそれを期待する）。

**cloudflared の ingress は `*.{domain}` の catch-all 1 本に固定。**
cloudflared には設定のホットリロードが無い（SIGHUP 非対応）ため、ルートごとに
ingress を書くとコンテナの起動・停止のたびに再起動＝全接続断になる。
ホスト名解決は roji 側が持つ。

**ドメインは 1 階層に収める。** Cloudflare の Universal SSL は `example.com` と
`*.example.com` しかカバーしない。`*.tunnel.example.com` にすると
Advanced Certificate Manager（有償）が必要になる。設定バリデーションが警告を出す。

**手動作業:** `*.{domain}` の DNS を `<uuid>.cfargotunnel.com` に向ける設定は
roji がやらない（Cloudflare の API トークンを持たせたくないため）。
`roji doctor` と `cloudflared tunnel route dns` で案内する。

### 環境変数

| 環境変数 | 説明 | デフォルト (Native Mode) |
|----------|------|-----------|
| `ROJI_NETWORK` | 監視するDockerネットワーク（カンマ区切り） | `roji` |
| `ROJI_DOMAIN` | ベースドメイン | `dev.localhost` |
| `ROJI_BIND` | 待ち受けアドレス（カンマ区切り、空で全インターフェース） | `127.0.0.1,::1` |
| `ROJI_CERTS_DIR` | 証明書ディレクトリ | `~/.local/share/roji/certs` |
| `ROJI_DATA_DIR` | データディレクトリ | `~/.local/share/roji` |
| `ROJI_DASHBOARD` | ダッシュボードホスト名 | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | ログレベル | `info` |
| `ROJI_HTTP_PORT` | HTTPポート | `80` |
| `ROJI_HTTPS_PORT` | HTTPSポート | `443` |
| `ROJI_AUTO_CERT` | 証明書自動生成 | `true` |

トンネル（`tunnel:`）、静的サイト（`static_sites:`）、ローカルプロキシ
（`proxy_sites:`）は設定ファイル専用で、対応する環境変数は無い。

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
| `/_api/config/reload` | 設定ファイル再読み込み（静的サイトとローカルプロキシを更新）。応答の `sites` に各サイトの結果が入る |
| `/_api/leases` | `roji run` のルートの登録・更新（POST）と取り消し（DELETE）。CLI 専用 |

## 実装済み機能（v0.1.0 → v1.4.0）

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
- 設定ファイルバリデーション（不明キー警告、型チェック）
- Docker Compose操作（ダッシュボード/APIからup/down/restart/logs）

### 配布・品質
- ワンライナーインストール（Native Mode / アップグレード対応）
  - 取得したアーカイブは `checksums.txt` と照合する。`gh` がありログイン済みなら
    `gh attestation verify` で出所も検証する（`release-build.yml` の `actions/attest` が
    証明を付ける）。どちらかが失敗したらインストールしない。取得したバイナリを
    `sudo` で実行し CA まで入れるため
  - 検証は `--signer-workflow`（署名したワークフロー）と `--source-ref`（入れる版の
    タグ）で絞る。`--repo` だけだと、古い版の正規のアーカイブを新しい版として
    配る差し替えを検出できない
  - **署名するワークフローのパスは版で違う**（`install.sh` の `signer_workflow`）。
    v1.3.0 までは `release.yml`、それより後は `release-build.yml`。install.sh は
    常に `main` のものが使われ、`--version` で古い版も入れるので、
    `release-build.yml` を改名・移動するときは先にこの分岐を足すこと
  - attestation は v1.2.1 以降にしか無い（`install.sh` の `FIRST_ATTESTED_VERSION`）。
    それより前の版はチェックサムの照合だけで入れる。v1.2.1 はリリースしておらず、
    実際に attestation が付いた最初の版は v1.3.0
  - 最新版は API ではなく `releases/latest` のリダイレクト先から得る（未認証 API の
    レート制限で「最新」と誤表示していたため）。得られなければエラーで終了する
  - 最終行の `main "$@"` 以外は変数宣言と関数定義だけにする。ダウンロードが途中で
    切れたスクリプトを実行させないため
- 依存のライセンス表示（`CREDITS` / `THIRD_PARTY_NOTICES`）
  - アーカイブと Docker イメージに同梱し、`roji credits` でバイナリからも出力する。
    install.sh はアーカイブからバイナリしか残さないため
  - `go:embed` はビルド時にファイルが要るので、`CREDITS` はコミットする（リリース時
    だけ生成すると `go build` / `go install` が失敗する）。dependabot の PR ごとには
    更新せず、リリース手順で再生成する
  - gocredits は `go.mod` の `tool` と、テストだけで使う依存を含めない
- GoReleaser v2（マルチプラットフォーム）
- ビルドと公開の分離（v1.4.0 から）
  - `release-build.yml`（reusable workflow）がビルドと attest だけを行う。権限は
    `contents: read` と attestation 用の 3 つで、secrets は渡さない
  - `release.yml` の `publish` ジョブが、受け取った成果物からリリース、Docker
    イメージ、Homebrew cask を公開する。ここではコンパイルしない
  - GoReleaser は `--skip=publish` でビルド専用。`--split` / `publish` は Pro 版に
    しか無いため、公開は `gh` と `docker/build-push-action` で書いている
  - **ビルド側のジョブに書き込み権限や secrets を足さないこと。** 分離している
    理由そのものが無くなる
- 再現可能ビルド（`-trimpath`、`mod_timestamp`、アーカイブ内の時刻と所有者の固定）
  - 同じタグからは同じアーカイブができる。`roji version` のビルド日はコミット日時
  - **GoReleaser の版は `release-build.yml` で 1 つに固定する**（`version: "v2.18.3"`
    のように。`"~> v2"` のような範囲にしない）。アーカイブの gzip 圧縮は
    GoReleaser が行い、版が違うと出力のバイト列が変わる。v1.4.0 をタグから
    再ビルドすると、v2.13.3 と v2.18.3 で `roji` バイナリと tar の層は一致し、
    `.tar.gz` のチェックサムだけが食い違った。範囲指定のままだと、ワークフローを
    再実行したときに別のチェックサムのアーカイブができる
  - この `version:` は dependabot が更新しない（更新するのは action の SHA だけ）。
    上げるときは手で、リリースとリリースの間に行う
- ワークフローの `uses:` はコミット SHA で固定する（タグは付け替えられるため）。
  末尾の `# vX.Y.Z` コメントを dependabot が SHA と一緒に更新する
- Distrolessイメージ
- セキュリティスキャン（Trivy, govulncheck）
- インテグレーション/E2Eテスト
- lint（gofmt / golangci-lint、GOOS ごとに 3 回）

### 安全な既定値 (v1.1.0)
- `bind` 設定（カンマ区切り、既定 `127.0.0.1,::1`）
  - **破壊的変更**: それまで全インターフェースで待ち受けていた。同一ネットワーク
    上の誰でもダッシュボードと無認証の Compose エンドポイントに到達できた
  - ループバックアドレスのバインド失敗は警告してスキップ（IPv6 無効環境でも
    `127.0.0.1` で起動する）。それ以外のアドレスの失敗は致命
  - 別端末から到達したい場合は `bind` にそのアドレスを足すか、空文字列で
    全インターフェース
- `roji doctor` が待ち受けアドレスを報告
- WebSocket を `httputil.ReverseProxy` 経由に変更（手組みの中継を廃止）
  - `permessage-deflate` が end-to-end で成立するようになった
  - アップグレードを拒否したバックエンドのレスポンスがそのまま届く

### 外部公開 (v1.2.0)
- Cloudflare Tunnel 経由の外部公開（`tunnel:` 設定 + `roji.tunnel` ラベル）
  - cloudflared は子プロセスとして exec（ライブラリとして import すると
    quic-go を引き込んでバイナリが数 MB 増えるため）
  - 専用リスナー `127.0.0.1:{tunnel.port}` + `TunnelHandler` のガード
  - `auto_start: true` で roji が cloudflared を起動・停止する
- `roji doctor` が cloudflared の導入・ログイン・named tunnel の存在を確認
- ダッシュボードに 🌐 バッジと公開 URL を表示

### ローカルプロキシ (v1.4.0)
- `proxy_sites:` でホスト上の dev サーバー（`vite dev` / `wrangler dev` 等）に
  ホスト名を付ける。バックエンドの種類は Docker コンテナ、静的ディレクトリ、
  ホストの TCP ポートの 3 つ
  - `target` はスキーム付きの URL 1 本（`http` のみ）。`port:` の略記は
    入れない。同じサイトの綴りを 2 つにしないため（`roji.path` を空文字列に
    正規化したのと同じ判断）
  - プロキシ処理は `Handler.proxyTo` が Docker ルートと共有する。WebSocket、
    SSE、gRPC、`Host` を要求元のまま渡す挙動、クエリ文字列の保持は、ここに
    書けば両方に効く。ローカルプロキシ用に `httputil` を別に呼ぶ実装を
    足さないこと
  - 転送先に繋がらないときは案内ページ（`unreachable.html`、502）を返す。
    登録時に転送先へ接続できるかは確かめない。dev サーバーが未起動なのは
    通常の状態で、案内ページを返すにはルートが要るため
- 宣言ルートの公開オプトイン（`proxy_sites` と `static_sites` の `tunnel: true`）
- `/_api/routes` の `kind`（`docker` / `static` / `local`）。`isStatic` は
  廃止した（破壊的変更）
- 設定の再読み込みは `Router.ReplaceDeclared` で宣言ルートを一括で入れ替える。
  消してから足す実装に戻さないこと（その隙間に、宣言ルートが隠していた
  Docker ルートがリクエストに答える）
- どのサイトを登録するかは `config.ResolveSites` が決め、サーバー
  （`declaredRoutes`）と `roji doctor`（`checks.Sites`）が同じ結果を読む。
  登録の条件を足すときは `ResolveSites` に足すこと。片方にだけ書くと、doctor の
  報告と実際の登録が食い違う
  - 登録しないもの: root がディレクトリでない静的サイト、`target` が解釈
    できないローカルプロキシ、ダッシュボードのホスト名かベースドメインを名乗る
    サイト（ダッシュボードが先に応答するので到達できない）、同じホスト名が
    後ろでもう一度宣言されているサイト
  - 登録するが警告するもの: 証明書が対応しないホスト名（ベースドメインの
    1 階層下だけが対応する）、`tunnel: true` なのにベースドメインの外にある
    ホスト名（公開用の名前が作れない）
- `roji doctor` の「宣言したサイト」チェック。上の内容と、ローカルプロキシの
  転送先が listen しているかを報告する。いずれも警告で、`--fix` は無い
- 証明書が対応する範囲は `config.CertCoversHost` が答える。certgen は SAN に
  `*.*.{domain}` も入れているが、ワイルドカードは先頭の 1 ラベルにしか
  効かないので 2 階層の名前には対応しない。`certgen` のテストが、生成した
  証明書と `CertCoversHost` の一致を検査している
- バックエンドへの接続は 10 秒で打ち切る（`backendDialTimeout`）。ローカル
  プロキシは応答しないアドレスを指せるため

**意図的に入れていないもの:**

- 宣言ルートの `path:`。`pathRoutes` と同じ最長一致のソートを 2 か所に持つ
  ことになる
- 転送先の死活監視
- `https` の転送先。dev サーバーの自己署名証明書を検証できない

### roji run (v1.4.0)

`roji run <name> -- <command>` は、空きポートを選び、稼働中の roji に
`https://<name>.{domain}` → `http://localhost:{port}` のルートを登録してから
コマンドを実行する。`proxy_sites` はポートを利用者が決める必要があり、
ポートを固定していないプロジェクト同士が既定のポート（5173、8787）を
取り合う問題は解決しなかった。

- **ポートの渡し方は、引数の `{port}` の置き換えと環境変数 `PORT` の両方。**
  vite と wrangler は `PORT` を読まないので、環境変数だけでは足りない。
  ツールごとに `--port` を自動で足す方式にはしない（ツールの引数の仕様に
  追従し続けることになる）
- **ルートは期限付き（リース）で持つ**（`proxy/lease.go`）。`roji run` が
  `LeaseTTL`（30 秒）の 3 分の 1 の間隔で更新し、更新が止まれば消える
  - 期限にしたのは、`roji run` が SIGKILL されても取り消しを呼べないため。
    PID の監視にしなかったのは、roji 本体と `roji run` が別のユーザーや
    名前空間で動きうるため
  - 更新は登録と同じリクエストなので、roji 本体が再起動してルートを忘れても
    次の更新で戻る
  - 期限切れの判定は引くたびに行う（`liveLease`）。定期的な掃除
    （`ExpireLeases`）はダッシュボードへの通知のためで、ルーティングの
    正しさはこれに依存しない
- **リースは宣言ルートの層に入れる。** `Resolve` は「設定ファイルの宣言 →
  リース → Docker」の順に引くが、優先順位の規則は「宣言 > 検出」の 1 本のまま。
  層を足さないこと
  - 設定ファイルが宣言しているホスト名はリースできない（409）。設定ファイルは
    意図して書いたものなので、後から起動したコマンドには奪わせない
  - リースは設定の再読み込み（`ReplaceDeclared`）で消えない。ただし
    再読み込みで同じホスト名が宣言されたら、そのリースは落とす。
    「`declaredRoutes` と `leases` は同じホスト名を持たない」が不変条件
  - 別の `roji run` が保持中のホスト名もリースできない（409）
- **API（`/_api/leases`）が受け取るのはホスト名とポートだけ。** 転送先は
  常に `http://localhost:{port}` で、呼び出し側は選べない。この API は
  無認証なので、任意の転送先を受け付けると、ホスト名を任意のアドレスへ
  向ける口になる。`target` を受け取るように変えないこと
- **このマシンからのリクエストだけを受け付ける**（`fromThisMachine`）。
  ほかの API は「誰が roji に届くか」を `bind` に任せているが、この API は
  接続元を自分で確かめる。`bind` を広げた構成で受け付けると、ループバックに
  しか bind していないサービス（デバッガ、管理 UI）に、ネットワーク上の
  誰でもホスト名を付けて到達できてしまう。接続元がループバックか、接続の
  両端が同じアドレス（`bind` にこのマシンのアドレスだけを書いた構成）の
  ときに通す
- **`Origin` ヘッダーの付いたリクエストは拒否する。** このマシンのブラウザで
  開いたページは、上の確認を通って API へ POST を送れる。ブラウザは
  クロスオリジンの POST に `Origin` を付け、CLI は付けない
- リースしたルートはトンネルで公開しない（`LocalBackend.Tunnel` は常に
  false）。無認証の API からインターネットへの公開を指示できる経路を
  作らないため。公開したいときは `proxy_sites` の `tunnel: true` を使う
- ホスト名の規則（ダッシュボードのホスト名の拒否、証明書の対応範囲）は
  `config.ResolveSiteName` を通して設定ファイルのサイトと共有する
  （`config.ResolveSites` もこの関数を呼ぶ）。ホスト名だけで決まる規則を
  足すときは `ResolveSiteName` に足すこと。`ResolveSites` の側に書くと
  リースには適用されない
- **シグナルは、コマンドが起動したプロセスまで届ける**
  （`cmd/roji/cmd/run_unix.go`）。dev サーバーは直接の子でないことが多い
  （`npx vite` は npm → sh → node）。直接の子にだけ送ると、node がポートを
  握ったまま残る
  - 端末のフォアグラウンドにいないとき: コマンドを独立したプロセスグループで
    起動し、グループへ送る
  - 端末のフォアグラウンドにいるとき: プロセスグループを分けない。
    フォアグラウンドのグループから外すと、コマンドが端末から読めなくなる
    （wrangler のホットキー等）。SIGINT と SIGHUP は端末がグループ全体へ
    配るので転送しない（割り込みが 2 回届くと、多くの dev サーバーは後始末を
    省いて終了する）。それ以外は、コマンドの子孫プロセスを `ps` でたどって
    1 つずつ送る。**自分のプロセスグループへは送らないこと。** make や npm
    スクリプトから起動されると、グループには起動元と並走中のプロセスも
    入っている
  - フォアグラウンドかどうかは端末に問い合わせる（`inForegroundGroup`）。
    stdin が文字デバイスかどうかでは判定しない（`/dev/null` も文字デバイス）
  - 既知の制約: フォアグラウンドで `kill -INT <roji run の PID>` を受けても
    コマンドへは転送しない。端末が配ったものと区別できないため。止めるには
    SIGTERM を使う
- コマンドの終了コードをそのまま返す。シグナルで終了したときは、シェルと
  同じ 128 + シグナル番号（Ctrl-C なら 130）
- ルートを取り消すのは、更新の goroutine が終わってから（`runWithLease`）。
  送信中の更新が取り消しの後に処理されると、ルートが登録し直されて期限まで
  残り、同じ名前の次の `roji run` が拒否される
- 空きポートは、ループバックの両方のアドレス（127.0.0.1 と ::1）で空いて
  いるものを選ぶ。転送先は `localhost` で、どちらにも解決されるため
- `roji run` は roji 本体が動いていなければコマンドを起動しない。起動して
  から URL が開けない状態を作らないため
- コマンドが選んだポートで待ち受けないとき（`{port}` を書かずに vite を
  起動した等）は、`roji run` が 20 秒後にその旨を表示し、案内ページ
  （`unreachable.html`）もリース向けの文面を出す

**意図的に入れていないもの:**

- プロセスの常駐や再起動。roji はプロセスマネージャではない
- `roji run` が SIGKILL されたときの dev サーバーの停止
- `roji serve`（静的ディレクトリの一時配信）
- 名前の省略（カレントディレクトリ名からの推測）と、ポートの指定

### roji config reload (v1.4.0)

`roji config reload` は、稼働中の roji に `POST /_api/config/reload` を送り、
結果を表示する。それまで再読み込みはダッシュボードのボタンと API でしか
行えなかった。

- **反映されるのは `static_sites` と `proxy_sites` だけ。** コマンド名は設定の
  全部が反映されるように読めるので、実行のたびにその旨を表示する
  （`cmd.config.reload.scope`）。この表示を省かないこと
- **API の応答に、宣言したサイトごとの結果を入れる**（`proxy.ReloadedSite`）。
  登録されなかったサイトは roji のログにしか出ていなかったので、CLI からは
  成功に見えていた。結果は `declaredRoutes` が `config.ResolveSites` から
  作るので、サーバーが実際に行った登録と食い違わない
  - 同じホスト名を 2 回宣言したときの「後の宣言が勝つ」も `ResolveSites` が
    決め、負けた側に `ErrSiteDuplicate` を付ける。ルーターが黙って落とす
    だけだと、負けた側が「登録済み」と報告されてしまう
  - 登録されない理由の分類は `config.DeclaredSite.SkipReason` の 1 か所。
    サーバーの報告、`roji doctor`、`roji config reload` がこのコードを使い、
    文言は `checks.SiteSkipMessage` が選ぶ。理由を足すときはこの 2 つに足す
  - 応答には、判定に使ったベースドメイン（`domain`）と読んだファイル
    （`configPath`）も入れる。どちらも稼働中の roji のもので、CLI が読んだ
    設定ファイルとは食い違いうる（起動後に `domain` を編集した、`--config` で
    別のファイルを指して起動した、など）
  - `sites` は、サイトが 1 件も無いときも空の配列で返す。`sites` の無い応答は
    「この項目を返さない古い roji」を意味し、CLI はそれで見分ける
  - 既存の `status` は残してある。ダッシュボードは HTTP ステータスしか
    見ていない
- **サイトがスキップされても終了コードは 0。** 残りのサイトは反映されており、
  編集後に再読み込みするスクリプトが、以前から壊れていたサイトのせいで
  止まるのを避ける。非 0 にするのは、roji に接続できない場合と、roji が
  設定ファイル自体を読めなかった場合（何も変更されていない）
- **CLI は、現在の設定ファイルが示す場所へリクエストを送る。** 起動後に
  `domain`、`dashboard`、`https_port` を編集していると、稼働中の roji には
  届かない。これは CLI 全般（`routes`、`health`、`run`）に共通の制約で、
  解決していない。`config reload` は、応答が再読み込みのものでなければ、
  設定ファイルのせいにせず再起動を案内する
- サイトの問題を説明する文言は `roji doctor` のもの（`doctor.check.sites.*`）を
  使う。同じ内容を同じ出どころから報告するので、文言を二重に持たない
- `roji config edit` は、編集後に再読み込みしない。案内を表示するだけ。
  編集と反映を分けたい場面があるため

## 開発フロー

機能開発（バグ修正を含む）では、実装後に次の順でレビューを通してからコミットする。

1. **実装** — テストまで含めて書き、`go build ./...` / `go test ./...` /
   `gofmt -l .`（何も出ない）/ `go vet ./...` / `golangci-lint run` が通る
   状態にする。いずれも CI で検査される
2. **`/simplify`** — 大規模な変更のときだけ回す。再利用・簡素化・効率・抽象度の
   観点でコードを書き換える。実装が終わったらコミットせずに回す
3. **`/code-review medium`** — バグの観点で見る。effort は必ず `medium` と
   明示する（省くと最後に指定したレベルが引き継がれるため）。**`--fix` は
   付けない**
4. **指摘への対応** — 1 件ずつ採否を決めて自分で反映する。採用しなかった指摘は
   理由を添えて報告する。反映したら手順 1 の検査を通し直す
5. **コミット** — ユーザーの承認を得てから

どちらのスキルも Claude が自分で実行する（ユーザーに実行を頼まない）。

変更の規模で、通す手順を決める。

| 規模 | 目安 | 手順 |
|---|---|---|
| 大規模 | 新しいモジュールや機構の追加、複数ファイルにまたがる構造の変更 | `/simplify` → `/code-review medium` |
| 中規模 | 既存の構造の中でのロジックの追加や変更 | `/code-review medium` のみ |
| 小規模 | 数行の修正、定数や文言の変更、分岐が増えない修正 | スキップ可 |

**`/simplify` と `/code-review` は必ず直列で実行する。並列で起動しない。**
`/simplify` の書き換えが終わってから `/code-review` を起動する。`/code-review` を
後にするのは、書き換えが済んだ最終形のコードをバグの観点で見るため。並列に
動かすと、`/code-review` は書き換え前のコードを見ることになり、`/simplify` が
入れた不具合も見逃す。

対象外（このフローを通さない）:

- ドキュメントのみの変更（`README.md`、`CLAUDE.md`、`website/` 等）。ただし
  `CLAUDE.md` を変更したときは、コミット前に
  `/claude-api prompt-audit CLAUDE.md` を実行し、指摘を 1 件ずつ採否を決めて
  反映する
- dependabot PR のローカルマージなど、レビュー済み変更の取り込み

## リリース手順

1. **ドキュメント更新**

   CHANGELOG.md に新バージョンの変更内容と末尾のリンク定義を追加する。

   `CREDITS` を再生成する。依存が変わっていれば差分が出る。
   `release.yml` の `test` ジョブが `go tool gocredits . | diff - CREDITS` で
   検査するので、忘れるとリリースが止まる。

   ```bash
   go tool gocredits -w .
   ```

   `THIRD_PARTY_NOTICES` は手で管理する。`proxy/templates/petite-vue.min.js` を
   差し替えたときに、版と著作権表示を更新する。Go モジュールの NOTICE ファイルは
   gocredits が `CREDITS` に入れるので、手で写さない。

   install.sh の URL は `main` 固定なので、リリースのたびの置換は要らない。
   install.sh は常に最新版を取りに行くため、URL をタグで固定しても版は固定
   されない。版の固定は `ROJI_VERSION` / `--version` で行う。

2. **コミット & タグ**
   ```bash
   git add CHANGELOG.md CREDITS THIRD_PARTY_NOTICES
   git commit -m "Prepare for vX.Y.Z release"
   git tag -a vX.Y.Z -m "Release vX.Y.Z: [主要機能]"
   git push origin main
   git push origin vX.Y.Z
   ```

3. **確認**
   - GitHub Actions → GitHub Release → Docker Image
   - リリースのアーカイブに attestation が付いていること

     ```bash
     gh attestation verify roji_Linux_x86_64.tar.gz --repo kan/roji \
       --signer-workflow kan/roji/.github/workflows/release-build.yml \
       --source-ref refs/tags/vX.Y.Z
     ```

   - リリースが draft のまま残っていないこと。`publish` ジョブは draft で作り、
     Docker イメージの push が済んでから公開する。attest は `build` ジョブで
     済んでいるので、公開された時点で必ず attestation がある。draft のまま
     残っていたら `publish` ジョブの失敗を調べる。直して `publish` ジョブを
     再実行すると、残った draft は作り直される。公開後のステップ（cask の更新など）
     で失敗した場合も同じく再実行でよく、公開済みのリリースには触らず残りだけを行う
   - `build` ジョブの成果物は 1 日で消える。`publish` の再実行が翌日以降になったら、
     ワークフロー全体を再実行する（ビルドは再現可能なので同じアーカイブができる）
   - Homebrew tap（`kan/homebrew-roji`）の `Casks/roji.rb` が新しい版になって
     いること。cask の更新は公開の後に行う（cask が指すダウンロード URL は
     draft の間は解決できないため）
   - `gh release verify vX.Y.Z --repo kan/roji` が通ること。リポジトリは
     immutable releases を有効にしており、公開した時点で release attestation が
     作られる
   - **公開後はアセットとタグを差し替えられない。** 成果物の誤りは次の版で直す
     （タイトルとリリースノートは公開後も編集できる）。`publish` ジョブが draft の
     うちにアセットを揃えてから公開するのはこのため。公開後にアセットを足す
     ステップをワークフローへ入れないこと
   - プレリリース（`v1.2.3-rc.1` のように `-` を含むタグ）は、GitHub の
     pre-release と版付きの Docker イメージだけを出す。イメージの `latest` と
     cask は更新しない

## ロードマップ（v0.7.0 → v1.4.0）

### バージョン計画

| Version | テーマ | 主要機能 |
|---------|--------|----------|
| **v0.7.0** | プロトコル拡張 | WebSocket対応、gRPC対応、ログエクスポート |
| **v0.8.0** | Native Mode | 単体バイナリ化、`roji doctor`、CA自動インストール |
| **v0.9.0** | 運用強化 | サービス登録、Docker Compose操作、httpd機能、BASIC認証 |
| **v1.0.0** | 安定版 | Docker Mode廃止、Homebrew対応、i18n、ドキュメント整備 |
| **v1.1.0** | 安全な既定値 | `bind` 設定（既定をループバックのみに変更＝破壊的）、ルーティング/WebSocket の修正、lint 導入 |
| **v1.1.1** | ルーティング修正 | ホスト名+プレフィックスの衝突検出、静的サイト優先の明確化 |
| **v1.2.0** | 外部公開 | Cloudflare Tunnel ラッパー（`roji.tunnel` オプトイン、専用リスナー + ガード） |
| **v1.3.0** | 配布 | install.sh のチェックサムと attestation の検証、`--version`、`roji credits` |
| **v1.4.0** | ホスト上の dev サーバー | `proxy_sites`、`roji run`、`roji config reload`、宣言ルートのトンネル公開、ビルドと公開の分離 |

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

v0.8.0 以降は **Native Mode** を主とする。Docker Mode は 1.0.0 で廃止した。

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
bind: 127.0.0.1,::1
certs_dir: ~/.local/share/roji/certs
data_dir: ~/.local/share/roji
dashboard: roji.dev.localhost
log_level: info
http_port: 80
https_port: 443
auto_cert: true

# 外部公開（省略可。domain と name が揃ったときだけ有効）
tunnel:
  domain: example.com   # Cloudflare 上のゾーン。1階層に収める
  name: roji            # cloudflared tunnel create で作った named tunnel
  port: 8080            # トンネル専用リスナー（127.0.0.1 固定）
  auto_start: false     # roji が cloudflared を起動するか

# ホスト上の dev サーバー（省略可）
proxy_sites:
  - host: api                      # ドット無し -> api.{domain}
    target: http://127.0.0.1:8787  # スキーム必須、http のみ
    tunnel: false                  # 外部公開のオプトイン（static_sites にも同じキーがある）
```

- [x] **config コマンド**
  - `roji config show` - 現在の設定を表示
  - `roji config path` - 設定ファイルパスを表示
  - `roji config init` - デフォルト設定ファイルを作成
  - `roji config edit` - エディタで設定を編集
  - `roji config reload` - `static_sites` / `proxy_sites` の変更を稼働中の roji に反映

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
- 宣言したサイト（設定時のみ: 静的サイトの root、ローカルプロキシの転送先、使えないホスト名）
- トンネル（設定時のみ: cloudflared の導入・ログイン・named tunnel の存在）

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
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash
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
- `--no-service` - サービス登録スキップ
- `--version X.Y.Z` - 最新版の代わりに指定した版を入れる（`ROJI_VERSION` でも可）

**旧版:** `install-docker.sh`（Docker Mode 用）は v1.0.0 で削除した

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
- [x] Dockerfile、docker-compose.yml は開発・テスト用として維持
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
- [x] `roji.self` ラベルをドキュメント化（内部使用ラベルとして公式化）
- [x] 環境変数・設定パス・APIは現状維持（変更不要と確認済み）

#### 4. パッケージマネージャー対応

- [x] **Homebrew対応**（macOS）
  - Cask の作成（GoReleaser の `homebrew_casks` セクション利用）
  - `brew install kan/roji/roji` でインストール可能に
- [ ] APT/RPMパッケージは検討のみ（v1.x以降）

#### 5. 多言語対応 (i18n)

- [x] **ダッシュボード (Web UI)**
  - メッセージの外部化（JSON / 埋め込みリソース）
  - ブラウザの `Accept-Language` ヘッダーで言語自動検出
  - 日本語リソース追加
- [x] **CLIコマンド**
  - メッセージ・ヘルプテキストの外部化
  - `LANG` 環境変数で言語自動検出
  - 日本語リソース追加
- [x] **インストーラー (install.sh)**
  - メッセージの日本語対応

#### 6. ドキュメント整備

- [x] **README.md 全面改訂**
  - Docker Mode関連の記述を削除・整理
  - Native Mode前提の記述に統一
  - 機能一覧を最新状態に更新
- [x] **Getting Started ガイド**（新規作成）
  - インストールから初めてのサービス公開まで
  - 設定ファイルの使い方
  - よくある構成例
- [x] **トラブルシューティングガイド**（拡充）
  - 既存セクションの拡充
  - プラットフォーム別の注意事項（WSL、macOS、Linux）
  - `roji doctor` との連携説明

---

### v1.2.0: 外部公開（トンネル）

`bind` がループバック限定になった（v1.1.0）ぶん、外から触りたいときの
手段が無くなったのを埋めるもの。用途は webhook 受信と、開発中のアプリを
外部に見せること。

- [x] `tunnel:` 設定ブロック（domain / name / port / auto_start）+ バリデーション
- [x] `roji.tunnel` ラベル（明示オプトイン）
- [x] `proxy/tunnel.go` — 専用リスナーのガード
- [x] `tunnel/cloudflared.go` — ingress 生成と cloudflared のプロセス管理
- [x] `roji doctor` のトンネルチェック
- [x] ダッシュボードの 🌐 バッジと公開 URL

**意図的に入れていないもの:**

- `roji tunnel start/stop/status` コマンド。`auto_start: false` のときは
  ユーザーが `cloudflared tunnel run <name>` を回す前提
- cloudflared のクラッシュ後の自動再起動。roji はプロセスマネージャでは
  ないので、落ちたことを報告して放置する（認証に失敗し続けるトンネルが
  黙って再試行を繰り返すのを避ける）
- Cloudflare API トークンによる DNS レコードの自動作成

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

---

### 公式サイト ✅

- **URL**: https://roji-proxy.dev
- **技術**: Hugo + Doks テーマ、GitHub Pages
- **ソース**: `website/` ディレクトリ（モノレポ）
- **多言語**: EN / JA
- **デプロイ**: GitHub Actions（`main` ブランチの `website/**` 変更時に自動デプロイ）

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

### コード品質

CI の `lint` ジョブが `gofmt` と `golangci-lint` を検査する。ローカルでも
同じものを回せる。

```bash
# CI と同じ検査
gofmt -l .                 # 何も出なければOK
golangci-lint run ./...

# プラットフォーム別ファイルの検査
# service/ と certgen/ はビルドタグで分かれており、Linux で1回走らせるだけでは
# macOS / Windows 用のファイルが一度もコンパイルされない。CI は GOOS ごとに
# 3回走らせている
GOOS=darwin golangci-lint run ./...
GOOS=windows golangci-lint run ./...
```

golangci-lint は go.mod が要求する Go 以上でビルドされたものが要る。古いまま
だと設定の読み込み自体が `the Go language version (go1.26) used to build
golangci-lint is lower than the targeted Go version (1.27.1)` で失敗する。
Go を上げるときは ci.yml の `version:` も合わせて上げること。

設定は `.golangci.yml`。リンタはデフォルト（errcheck / govet / ineffassign /
staticcheck / unused）のままで、除外は標準プリセット 4 つ。この構成で
nil デリファレンスや未チェックの `os.Rename` といった実際の欠陥を検出できて
いるので、取りこぼしが出るまでリンタは足さない方針。

意図的に無視するエラーは `_ =` で明示する（`errcheck` が求める形）。
`defer f.Close()` のようにエラーの行き場がないものは `std-error-handling`
プリセットが除外するので、書き換えなくてよい。

### go fix（modernizer）

Go 1.26 以降の `go fix` は modernizer 群を適用するツール。`strings.CutPrefix`、
`slices.Contains`、`min`/`max`、`t.Context()`、`for i := range n` などへの
機械変換を行う。

```bash
go fix -diff ./...              # 変更内容を見るだけ（差分があれば exit 1）
go fix ./...                    # 全アナライザを適用
go fix -stringscut ./...        # アナライザを選んで適用
go tool fix help                # アナライザ一覧
```

適用後の注意点:

- `go fix` は変換結果を中間変数で受けることがある（`rest := after` 等）。
  その場で使う形に詰め直す
- `stringsbuilder` のように関数を書き換えるものは、他と分けてコミットし、
  出力が変わっていないことを確認する
- CI では回していない。Go のバージョンが上がるとアナライザが増え、
  スタイルの問題で CI が落ちうるため

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

**GoReleaser がやらないこと:** リリースの作成、Docker イメージ、Homebrew tap への
push は `release.yml` の `publish` ジョブが行う（「配布・品質」の「ビルドと公開の
分離」）。`.goreleaser.yml` に `dockers_v2` や `release.draft` を戻しても
`--skip=publish` で無視される。

リリース用のビルドをローカルで再現するには次を使う（`--snapshot` は版が変わる）。

```bash
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean --skip=publish
```

公開されたアーカイブと `checksums.txt` まで一致させるには、`release-build.yml` が
固定している版の GoReleaser を、GitHub Releases の公式バイナリで使う。版が違うと
`.tar.gz` のチェックサムが変わる。`go install` でビルドしたものは、公式バイナリと
ビルドに使う Go が違いうるので、比較には使わない。

`Dockerfile.goreleaser` は `ARG TARGETPLATFORM` を使い `${TARGETPLATFORM}/roji` から
コピーする。`publish` ジョブがバイナリを `linux/amd64/roji` の形に並べて渡す。

### 手動テスト

```bash
# 開発サーバー起動（docker-compose.yml が Air でホットリロード。設定は .air.toml）
docker compose up

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
- [Hugo](https://gohugo.io/) - Go製静的サイトジェネレーター
- [Doks](https://getdoks.org/) - Hugo用ドキュメントテーマ
