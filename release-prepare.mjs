/**
 * **リリース PR の中身を作る。** tagpr が `postVersionCommand`（`.tagpr`）として、
 * main からリリース PR を作り直すたびに走らせる。**repo のルートで実行する。**
 *
 *     TAGPR_CURRENT_VERSION=vX.Y.Z TAGPR_NEXT_VERSION=vX.Y.Z node scripts/tagpr/release-prepare.mjs
 *     TAGPR_CURRENT_VERSION=vX.Y.Z TAGPR_NEXT_VERSION=vX.Y.Z node scripts/tagpr/release-prepare.mjs --changelog-only
 *
 * `TAGPR_CURRENT_VERSION` は最新のタグ、`TAGPR_NEXT_VERSION` は tagpr が提案する版。
 * どちらも tagpr が渡す。
 *
 * 1. `package-lock.json` の版を `package.json` に揃える（`npm-lock.mjs`）。
 *    `--changelog-only` では `package.json` も lock も読まない（版のファイルが無い repo や、
 *    版と lock を自分のラッパーで揃える repo 向け）
 * 2. `CHANGELOG.md` の `## [Unreleased]` を `## [X.Y.Z] - <日本時間の日付>` にし、
 *    その上に空の `## [Unreleased]` を置き直し、末尾のリンク定義の先頭にその版の
 *    リンクを足す（`changelog.mjs` の `stampRelease`）。リンク先の repo は
 *    Actions では `GITHUB_REPOSITORY`、それ以外では `origin` から決める
 *
 * **検査をすべて済ませてから書く。** 落ちるときは何も書き換えない（tagpr は失敗しても
 * リリース PR を更新するので、版だけ上がった PR を残さない）。それでも run が赤なら、
 * そのリリース PR はマージしない。
 *
 * **Node の組み込みしか使わない** —— このジョブはリポジトリへ push できるトークンを
 * 持つので、依存のコードを走らせない。
 */
import { writeFileSync } from 'node:fs';
import { ReleaseError, changelogPath, currentGithubRepo, readRootFile, releaseDate, stampRelease } from './changelog.mjs';
import { lockPath, syncPackageLock } from './npm-lock.mjs';

/** 環境変数の `vX.Y.Z` から `X.Y.Z` を取り出す。形が違えば落とす */
function versionFromEnv(name) {
  const value = process.env[name] ?? '';
  const version = value.replace(/^v/, '');
  if (!/^\d+\.\d+\.\d+$/.test(version)) {
    throw new ReleaseError(`${name} must be vX.Y.Z, got "${value}"`);
  }
  return version;
}

try {
  const args = process.argv.slice(2);
  const unknown = args.filter((a) => a !== '--changelog-only');
  if (unknown.length > 0) {
    throw new ReleaseError(`usage: node release-prepare.mjs [--changelog-only] (unknown: ${unknown.join(' ')})`);
  }
  const changelogOnly = args.includes('--changelog-only');
  const version = versionFromEnv('TAGPR_NEXT_VERSION');
  const current = versionFromEnv('TAGPR_CURRENT_VERSION');

  // --- 検査（ここでは何も書かない） ---

  const lock = changelogOnly ? null : syncPackageLock(version);
  const changelog = stampRelease(readRootFile(changelogPath), {
    version,
    current,
    repo: currentGithubRepo(),
    date: releaseDate(),
  });

  // --- 書き換え ---

  if (lock !== null) {
    writeFileSync(lockPath, lock);
    console.log(`${lockPath}: -> ${version}`);
  }
  writeFileSync(changelogPath, changelog);
  console.log(`CHANGELOG.md: "## [Unreleased]" -> "## [${version}]"`);
} catch (e) {
  if (!(e instanceof ReleaseError)) throw e;
  console.error(e.message);
  process.exit(1);
}
