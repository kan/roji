# tagpr-scripts の作業メモ

**取り込み先の `scripts/tagpr/` でこのファイルを読んでいるなら、そこでは何も編集しない。**
この repo は git subtree で各 repo の `scripts/tagpr/` に丸ごと取り込まれ、このファイルも
一緒に入る。直すのは [kan/tagpr-scripts](https://github.com/kan/tagpr-scripts) の側で、
取り込み先へは `git subtree pull` で入れる（下の「取り込み先へ届ける」）。

## これは何か

[tagpr](https://github.com/Songmu/tagpr) で、手書きの `CHANGELOG.md` を持つ npm パッケージを
リリースするためのスクリプト。lily / wema / wema-kake が共有している（各 repo の
`scripts/release-*.mjs` を 1 つにまとめたもの）。npm でない repo も
`release-prepare.mjs --changelog-only` で CHANGELOG の処理だけを使える（取り込み先はまだ無い）。

| ファイル | 呼ぶもの | 役割 |
|---|---|---|
| `release-prepare.mjs` | tagpr（`.tagpr` の `postVersionCommand`） | `## [Unreleased]` を `## [X.Y.Z] - <JST の日付>` にし、空の `## [Unreleased]` を置き直し、末尾のリンク定義の先頭にその版のリンクを足す。`package-lock.json` の版を `package.json` に揃える（`--changelog-only` では揃えない） |
| `release-notes.mjs` | 取り込み先の publish のワークフロー | CHANGELOG の 1 つの節の本文を出す。publish 前の存在確認と、GitHub Release の本文に使う。プレリリースのタグ（`vX.Y.Z-*`）では `Unreleased` の節を出す |
| `changelog.mjs` | 上の 2 つが import する | CHANGELOG の読み方（節の見出し、コードフェンス、末尾のリンク定義）と書き換え方（`stampRelease`）、日付・repo 名の決め方。import しても何も実行しない。npm に依存しない |
| `npm-lock.mjs` | `release-prepare.mjs` が import する | `package.json` の版の確認と、`package-lock.json` の版を揃えた中身を作る。書くのは呼んだ側 |
| `test/release.node-test.mjs` | `node --test` | 上の 4 つのテスト |

package.json は無い。依存も無い。

## ドキュメントの分担

| 置き場 | 言語 | 誰向け |
|---|---|---|
| `README.md` | 英語 | 使う人（取り込む repo の作業者）。CHANGELOG の形の規則、`.tagpr` の設定例、publish のワークフローへの組み込み方、変更の流し方 |
| コード中のコメント | 日本語 | 中を直す人。「なぜそうしたか」はここに書く |
| エラーメッセージ、テスト名 | 英語 | CI のログを読む人 |
| `AGENTS.md`（これ） | 日本語 | 作業時の前提。README に載せるほどではない文脈 |

- **README に書いた CHANGELOG の規則と `.tagpr` の設定例は、取り込み先との約束。**
  挙動を変えたら README も同じコミットで直す
- コミットメッセージは日本語（既存の履歴に合わせる）

## 守る前提

- **Node の組み込みモジュールしか使わない。** tagpr のジョブはリポジトリへ push できる
  トークンを持つので、依存のコードを走らせない。`npm ci` も、実行時の取得もしない。
  **package.json を足して依存を入れようとしない。** npm パッケージではなく subtree に
  しているのも同じ理由（コードが各 repo にコミットされ、その差分でレビューされる）
- **CHANGELOG の読み方は `changelog.mjs` の 1 か所にだけ書く。** `release-prepare.mjs` と
  `release-notes.mjs` に別々に書くと、リリース PR は通るのに publish の検査で落ちる。
  コードフェンスの追い方も `scan` の 1 か所だけ
- **`release-prepare.mjs` は検査をすべて済ませてから書く。落ちるときは何も書き換えない。**
  tagpr は失敗してもリリース PR を更新するので、版だけ上がった PR を残さないため。
  検査を足すときは「書き換え」の区切りより前に置く
- **スクリプトは repo のルートを作業ディレクトリにして実行される。** `CHANGELOG.md` /
  `package.json` / `package-lock.json` はカレントディレクトリから読む。スクリプト自身の
  場所（`scripts/tagpr/`）からの相対パスで読まない
- **日付は日本時間。** `releaseDate` は +9 時間して `toISOString` で作る。`Intl` の既定の
  書式は ICU の版で変わりうるので使わない
- **CHANGELOG の処理（`changelog.mjs`）に npm の前提を持ち込まない。** `package.json` /
  `package-lock.json` を読むのは `npm-lock.mjs` だけ。Go や Tauri の repo が
  `--changelog-only` で使うため（issue #1）
- **リンク先の repo は、Actions（`GITHUB_ACTIONS=true`）では `GITHUB_REPOSITORY`、それ以外では
  `origin` の URL から決める**（`currentGithubRepo`）。`package.json` の `repository` は読まない。
  GitHub 以外は落とす。エラーに remote の URL を出すときは認証情報を伏せる
- 改行は LF だけを受ける。CR があれば落とす
- 取り込み先の Node は 24（CI も 24 で回している）。それより新しい機能を使わない

## テスト

```bash
node --test 'test/*.node-test.mjs'
```

- **テストのファイル名を `*.test.mjs` にしない。** 取り込み先の vitest が既定の include で
  拾い、node:test のファイルなので "No test suite found" で落ちる。`*.node-test.mjs` にする
- `changelog.mjs` の関数は直接呼ぶ。2 つのスクリプトは、一時ディレクトリに repo のルートを
  作り、そこを作業ディレクトリにして子プロセスで実行する（tagpr と publish のワークフローと
  同じ呼ばれ方）。新しいテストもこの形に揃える
- **失敗するケースでは「何も書き換えていない」ことまで確かめる。** 既存のテストは
  `CHANGELOG.md` と `package-lock.json` が元のままかを見ている
- コミットの前にこのコマンドを通す。lint とフォーマッタは入れていない

## CI と GitHub の設定

- `.github/workflows/test.yml`: main への push と PR でテストを回す。依存が無いので
  `npm ci` は無い。権限は `contents: read`
- `.github/dependabot.yml`: 見るのは GitHub Actions だけ（月 1 回）
- action は major のタグ（`@v7`）で指している。lockfile（`actions.lock`）へ移す案が
  issue #1 にある（未着手）
- **`.github/` も取り込み先の `scripts/tagpr/.github/` に入る。** GitHub はルートの
  `.github/` しか読まないので、取り込み先では動かない。取り込み先と中身を揃えるために残している

## 変更の入れ方

- この repo には `.tagpr` が無い。リリースとタグを作らず、取り込み先は `main` を pull する。
  **ブランチの運用が決まるまでは、`main` へ直接 push する前提で進める**
- **`main` に入れた時点で、取り込み先の次の `git subtree pull` に乗る。** 取り込み先の
  リリースの手順を壊す変更（`.tagpr` の設定例やワークフローの書き方が変わる、CHANGELOG の
  規則が厳しくなる）は、README に書き、取り込み先をどう直すかまで決めてから入れる

### 取り込み先へ届ける

取り込み先の repo で、PR を作って取り込む。

```bash
git subtree pull -S --prefix=scripts/tagpr https://github.com/kan/tagpr-scripts.git main --squash
```

- **`-S` を付ける。** subtree は `commit-tree` でコミットを作るので、`commit.gpgsign` が
  効かず未署名になる
- **その PR はマージコミットで入れる。** squash で入れる repo でも同じ。subtree は前回の
  取り込みを自分が作ったコミットから探すので、squash すると次の pull が壊れる
- 取り込む PR は取り込み先の CHANGELOG に書かない（利用者に届かない）
- 取り込み先: lily / wema / wema-kake。完成形の例は lily の `.github/workflows/`
  （`tagpr.yml` が `publish.yml` をタグを ref にした `workflow_dispatch` で起動する）

## 今後の予定（issue #1）

roji / pike / musql / notify-slack / hokora の CHANGELOG の処理を、ここへ合流させる案を
調べてある。**repo ごとの違いをどう揃えるかの方針は、まだ決まっていない。** issue を読まずに着手しない。

- 合流の前提になる変更（CHANGELOG の処理と版・lock の処理を分ける、リンク先の repo を
  `package.json` に頼らず決める）は済んだ。`--changelog-only` が npm でない repo の入り口
- **repo ごとに方針が割れている。** 空の `## [Unreleased]` を戻すか、リンク定義を
  `releases/tag` / `compare` / 付けない のどれにするか、節をリンク定義で切るか。
  揃えるかオプションで吸収するかはユーザーが決める。勝手に片方へ寄せない
- 進める順の案は issue #1 のコメントにある（更新されていくので、着手前に最新を読む）
- workflow の共通化と、action を `actions.lock` で固定する件もこの issue で扱っている
- **今の挙動（lily / wema / wema-kake が使っている形）は保つ。** npm の lock を揃える処理は
  `npm-lock.mjs` に残してあり、lily 系は `--changelog-only` を付けずに呼ぶ（`.tagpr` は変えない）
