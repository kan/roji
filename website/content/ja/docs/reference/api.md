---
title: "APIリファレンス"
slug: "api"
description: "ダッシュボードAPIエンドポイント。"
weight: 5
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

すべてのAPIエンドポイントはダッシュボードホスト（デフォルト: `https://roji.dev.localhost`）で提供されます。

## エンドポイント

| エンドポイント | メソッド | 説明 |
|----------------|----------|------|
| `/_api/health` | GET | ヘルスチェック |
| `/healthz` | GET | `/_api/health` のエイリアス |
| `/_api/status` | GET | 詳細ステータス（バージョン、稼働時間、証明書、Docker） |
| `/_api/routes` | GET | 全登録ルート一覧 |
| `/_api/projects` | GET | アクティブ・非アクティブプロジェクト一覧 |
| `/_api/events` | GET | リアルタイムルート更新のSSEストリーム |
| `/_api/logs` | GET | 最近のリクエストログ（直近100件） |
| `/_api/logs/events` | GET | リアルタイムログ更新のSSEストリーム |
| `/_api/logs/export` | GET | ログをJSONまたはCSVでエクスポート |
| `/_api/containers/{id}/restart` | POST | コンテナを再起動 |
| `/_api/projects/{name}/up` | POST | プロジェクト起動（`docker compose up -d`） |
| `/_api/projects/{name}/down` | POST | プロジェクト停止（`docker compose down`） |
| `/_api/projects/{name}/restart` | POST | プロジェクトの全サービスを再起動 |
| `/_api/projects/{name}/logs` | GET | プロジェクトログのストリーム（SSE） |
| `/_api/projects/{name}/delete` | DELETE | プロジェクト履歴から削除 |
| `/_api/config/reload` | POST | 設定ファイルの再読み込み |

## ヘルスチェック

```bash
curl -s https://roji.dev.localhost/_api/health
```

```json
{"status": "healthy", "routes": 5}
```

**ステータス値：**

| ステータス | 説明 |
|-----------|------|
| `healthy` | すべてのシステムが正常 |
| `degraded` | 証明書の有効期限が30日以内 |
| `unhealthy` | Docker接続が切断 |

## ログエクスポート

オプションのフィルタ付きでリクエストログをエクスポート：

```bash
# JSON形式
curl -s "https://roji.dev.localhost/_api/logs/export?format=json"

# フィルタ付きCSV形式
curl -s "https://roji.dev.localhost/_api/logs/export?format=csv&service=myapp&method=GET"
```

**クエリパラメータ：**

| パラメータ | 説明 |
|-----------|------|
| `format` | `json`（デフォルト）または `csv` |
| `service` | サービス名でフィルタ |
| `host` | ホスト名でフィルタ |
| `method` | HTTPメソッドでフィルタ |
| `from` | 開始時刻（RFC 3339） |
| `to` | 終了時刻（RFC 3339） |

## SSEストリーム

### ルートイベント（`/_api/events`）

ルートの追加・更新・削除時にイベントを送信：

```javascript
const es = new EventSource("https://roji.dev.localhost/_api/events");
es.onmessage = (event) => {
  const routes = JSON.parse(event.data);
  console.log("Routes updated:", routes);
};
```

### ログイベント（`/_api/logs/events`）

リクエストログエントリをリアルタイムでストリーム：

```javascript
const es = new EventSource("https://roji.dev.localhost/_api/logs/events");
es.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(`${log.method} ${log.url} -> ${log.status}`);
};
```

## CORS

APIエンドポイントは `*.dev.localhost` サブドメインからのクロスオリジンリクエストをサポートしており、rojiを通じてプロキシされたサービスからのAPI呼び出しが可能です。
