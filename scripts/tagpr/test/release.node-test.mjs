// `node --test 'test/*.node-test.mjs'` で走る。**名前を `*.test.mjs` にしない** ——
// subtree で取り込んだ先の vitest が既定の include（`**/*.{test,spec}.?(c|m)[jt]s?(x)`）で
// 拾い、node:test のファイルなので "No test suite found" で落ちる。
//
// `changelog.mjs` の関数は直接、2 つのスクリプトは一時
// ディレクトリに repo のルートを作り、そこを作業ディレクトリにして子プロセスで実行する
// （tagpr と publish のワークフローと同じ呼ばれ方）。
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, test } from 'node:test';
import { compareVersions, findSection, githubRepo, releaseDate, versionHeadings } from '../changelog.mjs';

const prepare = join(import.meta.dirname, '..', 'release-prepare.mjs');
const notes = join(import.meta.dirname, '..', 'release-notes.mjs');

const HEADER = '# Changelog\n\nIntro.\n\n';
const MINIMAL = `${HEADER}## [Unreleased]\n\n- A.\n`;
const LINK = '[1.2.4]: https://github.com/o/r/releases/tag/v1.2.4';

const dirs = [];
after(() => {
  for (const dir of dirs) rmSync(dir, { recursive: true, force: true });
});

/**
 * 外の repo を触らない環境。git のフックから走らせると `GIT_DIR` などを引き継ぐので消し、
 * 一時ディレクトリより上の repo を見に行かないよう `GIT_CEILING_DIRECTORIES` で止める
 */
function gitEnv() {
  const env = { ...process.env, GIT_CEILING_DIRECTORIES: tmpdir() };
  for (const name of ['GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR']) delete env[name];
  return env;
}

function makeRepo({ version = '1.2.4', changelog = MINIMAL, pkg = true, lock = true, origin = null } = {}) {
  const dir = mkdtempSync(join(tmpdir(), 'tagpr-scripts-'));
  dirs.push(dir);
  if (pkg) writeFileSync(join(dir, 'package.json'), `${JSON.stringify({ name: 'x', version }, null, 2)}\n`);
  if (origin !== null) {
    for (const args of [['init', '-q'], ['remote', 'add', 'origin', origin]]) {
      assert.equal(spawnSync('git', args, { cwd: dir, env: gitEnv() }).status, 0, `git ${args.join(' ')}`);
    }
  }
  if (lock) {
    const content = { name: 'x', version: '1.2.3', lockfileVersion: 3, packages: { '': { name: 'x', version: '1.2.3' } } };
    writeFileSync(join(dir, 'package-lock.json'), `${JSON.stringify(content, null, 2)}\n`);
  }
  writeFileSync(join(dir, 'CHANGELOG.md'), changelog);
  return dir;
}

/**
 * tagpr と同じく、最新のタグ（current）と提案する版（next）を渡す。Actions の中として
 * `GITHUB_ACTIONS=true` と `GITHUB_REPOSITORY=o/r` を渡す。`actions: false` で Actions の外に、
 * `repository: null` で `GITHUB_REPOSITORY` を消す（どちらも `origin` から決めさせる）
 */
function run(script, cwd, args = [], { next = 'v1.2.4', current = 'v1.2.3', repository = 'o/r', actions = true } = {}) {
  const env = { ...gitEnv(), TAGPR_NEXT_VERSION: next, TAGPR_CURRENT_VERSION: current };
  if (actions) env.GITHUB_ACTIONS = 'true';
  else delete env.GITHUB_ACTIONS;
  if (repository === null) delete env.GITHUB_REPOSITORY;
  else env.GITHUB_REPOSITORY = repository;
  return spawnSync(process.execPath, [script, ...args], { cwd, env, encoding: 'utf8' });
}

const read = (dir, name) => readFileSync(join(dir, name), 'utf8');
const lockVersions = (dir) => {
  const lock = JSON.parse(read(dir, 'package-lock.json'));
  return [lock.version, lock.packages[''].version];
};

/**
 * prepare を走らせ、見出しの日付を `<date>` にした CHANGELOG を返す。日付は実行の前後
 * どちらかの日であること（日付をまたいでも落ちない）
 */
function prepareAndRead(dir, args = [], opts = {}) {
  const before = releaseDate();
  const r = run(prepare, dir, args, opts);
  const after = releaseDate();
  assert.equal(r.status, 0, r.stderr);
  const text = read(dir, 'CHANGELOG.md');
  const date = /^## \[1\.2\.4\] - (\S+)$/m.exec(text)?.[1];
  assert.ok(date === before || date === after, `unexpected date ${date}`);
  return text.replace(` - ${date}\n`, ' - <date>\n');
}

// --- release-prepare.mjs ---

test('prepare: stamps Unreleased, adds the link above the others, and syncs the lock', () => {
  const dir = makeRepo({
    changelog: `${HEADER}## [Unreleased]\n\n### Fixed\n\n- A.\n\n## [1.2.3] - 2026-01-01\n\n- Old.\n\n[1.2.3]: https://github.com/o/r/releases/tag/v1.2.3\n`,
  });
  assert.equal(
    prepareAndRead(dir),
    `${HEADER}## [Unreleased]\n\n## [1.2.4] - <date>\n\n### Fixed\n\n- A.\n\n## [1.2.3] - 2026-01-01\n\n- Old.\n\n` +
      `${LINK}\n[1.2.3]: https://github.com/o/r/releases/tag/v1.2.3\n`,
  );
  assert.deepEqual(lockVersions(dir), ['1.2.4', '1.2.4']);
});

test('prepare: puts the link at the end when there are no link definitions', () => {
  assert.equal(prepareAndRead(makeRepo()), `${HEADER}## [Unreleased]\n\n## [1.2.4] - <date>\n\n- A.\n\n${LINK}\n`);
});

test('prepare: a link-shaped line inside an entry is not where the links go', () => {
  const dir = makeRepo({ changelog: `${HEADER}## [Unreleased]\n\n[BREAKING]: dropped Node 18\n\n- A.\n` });
  assert.equal(
    prepareAndRead(dir),
    `${HEADER}## [Unreleased]\n\n## [1.2.4] - <date>\n\n[BREAKING]: dropped Node 18\n\n- A.\n\n${LINK}\n`,
  );
});

test('prepare: leaves headings and link definitions inside code fences alone', () => {
  const fence = '````md\n```\n## [Unreleased]\n[x]: https://example.com\n````\n';
  const dir = makeRepo({ changelog: `${HEADER}## [Unreleased]\n\n- A.\n\n${fence}` });
  assert.equal(prepareAndRead(dir), `${HEADER}## [Unreleased]\n\n## [1.2.4] - <date>\n\n- A.\n\n${fence}\n${LINK}\n`);
});

test('prepare: takes the repository from origin without GITHUB_REPOSITORY', () => {
  const dir = makeRepo({ origin: 'git@github.com:a/b.git' });
  assert.equal(
    prepareAndRead(dir, [], { repository: null }),
    `${HEADER}## [Unreleased]\n\n## [1.2.4] - <date>\n\n- A.\n\n[1.2.4]: https://github.com/a/b/releases/tag/v1.2.4\n`,
  );
});

test('prepare: ignores GITHUB_REPOSITORY outside GitHub Actions', () => {
  // 手元のシェルに別の repo の値が残っていても、origin を使う
  const dir = makeRepo({ origin: 'https://github.com/a/b.git' });
  assert.match(prepareAndRead(dir, [], { repository: 'other/repo', actions: false }), /^\[1\.2\.4\]: https:\/\/github\.com\/a\/b\//m);
});

test('prepare: does not print the credentials in origin', () => {
  const dir = makeRepo({ origin: 'https://oauth2:secret-token@gitlab.com/o/r.git' });
  const r = run(prepare, dir, [], { repository: null });
  assert.equal(r.status, 1);
  assert.match(r.stderr, /origin is https:\/\/\*\*\*@gitlab\.com\/o\/r\.git/);
  assert.doesNotMatch(r.stderr, /secret-token/);
});

test('prepare: --changelog-only needs no package.json or lock', () => {
  const dir = makeRepo({ pkg: false, lock: false });
  assert.equal(prepareAndRead(dir, ['--changelog-only']), `${HEADER}## [Unreleased]\n\n## [1.2.4] - <date>\n\n- A.\n\n${LINK}\n`);
});

test('prepare: --changelog-only leaves package-lock.json alone', () => {
  const dir = makeRepo({ version: '9.9.9' });
  prepareAndRead(dir, ['--changelog-only']);
  assert.deepEqual(lockVersions(dir), ['1.2.3', '1.2.3']);
});

// package-lock.json と CHANGELOG.md は、どの失敗でも書き換えない
for (const { name, opts, message, args, next, current, repository } of [
  { name: 'an empty Unreleased', opts: { changelog: `${HEADER}## [Unreleased]\n\n## [1.2.3]\n\n- Old.\n` }, message: /is empty/ },
  { name: 'no Unreleased', opts: { changelog: `${HEADER}## [1.2.3]\n\n- Old.\n` }, message: /no "## \[Unreleased\]"/ },
  { name: 'two Unreleased sections', opts: { changelog: `${MINIMAL}\n## [Unreleased]\n\n- B.\n` }, message: /2 "## \[Unreleased\]" sections/ },
  {
    name: 'a section for the version that is already there',
    opts: { changelog: `${HEADER}## [Unreleased]\n\n## [1.2.4] - 2026-01-02\n\n- A.\n` },
    message: /already has "## \[1\.2\.4\]", newer than the latest tag v1\.2\.3: .*merged but not tagged/,
  },
  {
    // 1.2.4 のタグが無いまま enhancement の PR が入り、tagpr が 1.2.3 から数えて minor を提案した
    name: 'an untagged section when tagpr proposes another version',
    opts: {
      version: '1.3.0',
      changelog: `${HEADER}## [Unreleased]\n\n- B.\n\n## [1.2.4] - 2026-01-02\n\n- A.\n\n## [1.2.3] - 2026-01-01\n\n- Old.\n`,
    },
    message: /already has "## \[1\.2\.4\]", newer than the latest tag v1\.2\.3/,
    next: 'v1.3.0',
  },
  {
    // 2 つのマージの run がどちらも止められた。古い方から直すよう、両方を古い順に出す
    name: 'two untagged sections',
    opts: {
      version: '1.2.6',
      changelog: `${HEADER}## [Unreleased]\n\n- C.\n\n## [1.2.5]\n\n- B.\n\n## [1.2.4]\n\n- A.\n`,
    },
    message: /already has "## \[1\.2\.4\]", "## \[1\.2\.5\]", .*merge commit for 1\.2\.4 first/,
    next: 'v1.2.6',
  },
  { name: 'a TAGPR_CURRENT_VERSION that is not vX.Y.Z', opts: {}, message: /TAGPR_CURRENT_VERSION must be vX\.Y\.Z/, current: '' },
  { name: 'CRLF line endings', opts: { changelog: MINIMAL.replaceAll('\n', '\r\n') }, message: /CR line endings/ },
  { name: 'package.json at another version', opts: { version: '1.2.5' }, message: /package\.json says 1\.2\.5/ },
  { name: 'a GITHUB_REPOSITORY that is not owner/repo', opts: {}, message: /GITHUB_REPOSITORY must be owner\/repo/, repository: 'https://github.com/o/r' },
  {
    name: 'an origin that is not GitHub',
    opts: { origin: 'https://gitlab.com/o/r.git' },
    message: /Cannot tell the GitHub repository.*origin is https:\/\/gitlab\.com\/o\/r\.git/,
    repository: null,
  },
  { name: 'neither GITHUB_REPOSITORY nor a git repository', opts: {}, message: /Cannot tell the GitHub repository/, repository: null },
  { name: 'an unknown option', opts: {}, message: /usage: .*unknown: --npm/, args: ['--npm'] },
  { name: 'a TAGPR_NEXT_VERSION that is not vX.Y.Z', opts: {}, message: /must be vX\.Y\.Z/, next: 'v1.2' },
]) {
  test(`prepare: fails on ${name} and writes nothing`, () => {
    const dir = makeRepo(opts);
    const changelog = read(dir, 'CHANGELOG.md');
    const r = run(prepare, dir, args, { next, current, repository });
    assert.equal(r.status, 1);
    assert.match(r.stderr, message);
    assert.equal(read(dir, 'CHANGELOG.md'), changelog);
    assert.deepEqual(lockVersions(dir), ['1.2.3', '1.2.3']);
  });
}

test('prepare: skips the untagged check when there are no tags yet', () => {
  // タグを打たずにリリースしてきた repo。tagpr は v0.0.0 を渡す
  const dir = makeRepo({ changelog: `${HEADER}## [Unreleased]\n\n- A.\n\n## [1.0.0]\n\n- Old.\n` });
  const r = run(prepare, dir, [], { current: 'v0.0.0' });
  assert.equal(r.status, 0, r.stderr);
  assert.match(read(dir, 'CHANGELOG.md'), /^## \[1\.2\.4\] - /m);
});

test('prepare: says where to run it when a file is missing', () => {
  const r = run(prepare, makeRepo({ lock: false }));
  assert.equal(r.status, 1);
  assert.match(r.stderr, /package-lock\.json not found .*run this from the repository root/);
  const sub = join(makeRepo(), 'sub');
  mkdirSync(sub);
  assert.match(run(notes, sub, ['1.2.4']).stderr, /CHANGELOG\.md not found/);
});

// --- release-notes.mjs ---

test('notes: prints the section for a tag, without the link definitions', () => {
  const dir = makeRepo({
    changelog: `${HEADER}## [Unreleased]\n\n## [1.2.4] - 2026-01-02\n\n### Fixed\n\n- A.\n\n## [1.2.3]\n\n- Old.\n\n[1.2.4]: https://example.com\n\n`,
  });
  const r = run(notes, dir, ['v1.2.4']);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout, '### Fixed\n\n- A.\n');
  // 最後の節に末尾のリンク定義が混ざらない
  assert.equal(run(notes, dir, ['1.2.3']).stdout, '- Old.\n');
});

test('notes: prints Unreleased for a pre-release tag', () => {
  const dir = makeRepo({ changelog: `${HEADER}## [Unreleased]\n\n- B.\n\n## [1.2.3]\n\n- Old.\n` });
  const r = run(notes, dir, ['v1.3.0-rc.1']);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout, '- B.\n');
});

test('notes: fails on a missing or empty section', () => {
  const dir = makeRepo({ changelog: `${HEADER}## [Unreleased]\n\n## [1.2.3]\n\n- Old.\n` });
  assert.match(run(notes, dir, ['v9.9.9']).stderr, /no "## \[9\.9\.9\]"/);
  assert.match(run(notes, dir, ['Unreleased']).stderr, /is empty/);
});

// --- changelog.mjs ---

test('findSection: a link-shaped line inside an entry does not end the section', () => {
  const text = `${HEADER}## [1.2.4]\n\n[BREAKING]: dropped Node 18\n\n- A.\n\n## [1.2.3]\n\n- Old.\n\n[1.2.3]: https://x\n`;
  assert.equal(findSection(text, '1.2.4'), '[BREAKING]: dropped Node 18\n\n- A.');
  assert.equal(findSection(text, '1.2.3'), '- Old.');
});

test('findSection: a fence closes only on the same marker, at least as long', () => {
  for (const fence of ['````\n```\n## [1.2.3]\n````', '~~~\n```\n## [1.2.3]\n~~~']) {
    const text = `## [1.2.4]\n\n${fence}\n\n## [1.2.3]\n\n- Old.\n`;
    assert.equal(findSection(text, '1.2.4'), fence);
    assert.equal(findSection(text, '1.2.3'), '- Old.');
  }
});

test('versionHeadings: lists version headings outside fences and the link block', () => {
  const text =
    `${HEADER}## [Unreleased]\n\n- B.\n\n## [1.2.4] - 2026-01-02\n\n\`\`\`\n## [9.9.9]\n\`\`\`\n\n` +
    `## [1.2.3]\n\n- Old.\n\n[1.2.4]: https://x\n`;
  assert.deepEqual(versionHeadings(text), ['1.2.4', '1.2.3']);
});

test('compareVersions: compares each part as a number', () => {
  assert.ok(compareVersions('1.10.0', '1.9.9') > 0);
  assert.ok(compareVersions('1.2.3', '1.2.4') < 0);
  assert.equal(compareVersions('1.2.3', '1.2.3'), 0);
});

test('githubRepo: takes the remote URLs git uses for GitHub', () => {
  for (const remote of [
    'git@github.com:o/r.git',
    'git@github.com:o/r',
    'org-123@github.com:o/r.git',
    'github.com:o/r.git',
    'https://github.com/o/r',
    'https://github.com/o/r.git',
    'http://www.github.com/o/r/',
    'ssh://git@github.com/o/r.git',
    'git://github.com/o/r.git',
  ]) {
    assert.equal(githubRepo(remote), 'o/r', remote);
  }
});

test('githubRepo: rejects anything that is not GitHub', () => {
  // `o/r` は remote としてはローカルのパス
  for (const remote of ['o/r', 'bitbucket.org/foo', 'git@gitlab.com:o/r.git', 'git@github-work:o/r.git', 'https://gitlab.com/o/r', 'https://github.com/o', '']) {
    assert.equal(githubRepo(remote), null, remote);
  }
});

test('releaseDate: is the date in JST', () => {
  assert.equal(releaseDate(new Date('2026-01-01T14:59:59Z')), '2026-01-01');
  assert.equal(releaseDate(new Date('2026-01-01T15:00:00Z')), '2026-01-02');
});
