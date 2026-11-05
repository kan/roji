# tagpr-scripts

The scripts that let [tagpr](https://github.com/Songmu/tagpr) release an npm package whose
`CHANGELOG.md` is written by hand. Shared by [lily](https://github.com/kan/lily),
[wema](https://github.com/kan/wema) and [wema-kake](https://github.com/kan/wema-kake),
which pull this repository in with `git subtree` at `scripts/tagpr/`.
The Go projects [roji](https://github.com/kan/roji),
[notify-slack](https://github.com/kan/notify-slack) and [hokora](https://github.com/kan/hokora),
which have no version file, use only the CHANGELOG part (`--changelog-only`). The Tauri apps
[musql](https://github.com/kan/musql) (`--changelog-only --cargo src-tauri`) and
[pike](https://github.com/kan/pike) (`--cargo src-tauri`, with `package.json` too) also
bring `Cargo.lock` along.

| File | Run by | Does |
|---|---|---|
| `release-prepare.mjs` | tagpr, as `postVersionCommand` | Turns `## [Unreleased]` into `## [X.Y.Z] - <date in JST>`, adds the version's link definition, and brings `package-lock.json` to the version tagpr put in `package.json` (not with `--changelog-only`) |
| `release-notes.mjs` | The publish workflow | Prints one section of `CHANGELOG.md`, to check that it exists before publishing and to use it as the GitHub release notes. Given a tag `vX.Y.Z` it prints `## [X.Y.Z]`; given a pre-release tag (`vX.Y.Z-rc.1`, tagged by hand on `main`) it prints `## [Unreleased]` |
| `changelog.mjs` | (imported by both) | How a `CHANGELOG.md` is read and stamped. Written in one place so the release pull request and the publish never disagree. Knows nothing about npm |
| `npm-lock.mjs` | (imported by `release-prepare.mjs`) | The `package.json` / `package-lock.json` part |
| `cargo-lock.mjs` | (imported by `release-prepare.mjs`) | The `Cargo.toml` / `Cargo.lock` part, with `--cargo <dir>` |
| `version-files.mjs` | (imported by `release-prepare.mjs`) | Checks that every `versionFile` in `.tagpr` is at the proposed version |

**Only Node's built-in modules.** The tagpr job holds a token that can push to the
repository, so it runs no dependency code: no `npm ci`, nothing fetched at run time. That
is also why this is a subtree and not an npm package — the code is committed in each
repository and reviewed in its diffs.

Run both from the repository root: `CHANGELOG.md`, and for `release-prepare.mjs` also
`.tagpr`, the files its `versionFile` lists, and without `--changelog-only` `package.json`
and `package-lock.json` (lockfile version 2 or later), are read from the working directory.

`release-prepare.mjs` reads `versionFile` from `.tagpr` (with `git config`) and checks that
each file listed is at the proposed version: the top-level `"version"` of a `*.json`, the
`[package]` `version` of a `Cargo.toml`. tagpr rewrites only the first occurrence of the
current version in each file, so a dependency at the same version above the file's own
version line would take the change and leave the real one behind, with nothing failing;
the tag is then made from the first file's version. Other kinds of files cannot be checked
and fail. With no `.tagpr`, or `versionFile = -`, nothing is checked.

`release-prepare.mjs --changelog-only` reads neither `package.json` nor its lock, for a
repository with no npm package: one with no version file (`versionFile = -`), or one that
brings its own version files and locks up to date in a wrapper of its own.

`release-prepare.mjs --cargo <dir>` also brings `<dir>/Cargo.lock` to the version, with or
without `--changelog-only`. List `<dir>/Cargo.toml` in `versionFile`, and not the lock:
tagpr rewrites only the first occurrence of the current version in each file, which in a
`Cargo.lock` can be a dependency at the same version (and in a `package-lock.json` leaves
the second one behind). This checks that
the `version` in its `[package]` table is the proposed one, then rewrites the version line
of the package's own `[[package]]` entry in `Cargo.lock` (the one without a `source`).
`cargo` is not run, so nothing is fetched. A `version.workspace = true` is not supported.

## What the CHANGELOG has to look like

[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), with these rules:

- Each pull request that changes what users get adds its line under `## [Unreleased]`.
  That section becomes the release notes word for word.
- A section ends at the next `## ` heading, or at the block of link definitions
  (`[x]: url`) at the end of the file. Headings inside code fences are not headings.
- Entries may be wrapped. `release-notes.mjs` joins the lines of a paragraph or a list
  item into one, because GitHub shows every line break in release notes: with nothing
  between two Japanese characters (kanji, kana, full-width) or next to full-width
  punctuation and brackets (`、` `。` `（` `）` and so on), and with one space anywhere
  else. Code fences, headings, tables, block quotes, HTML blocks, thematic breaks and hard
  breaks (two trailing spaces or `\`) are left as they are. A wrapped line that starts with
  a number and a period (`2026. ...`) is taken for the next numbered item and not joined, so
  do not wrap there. Indented code blocks are not supported; use fences.
- Links inside entries are written inline. A reference link's definition sits outside the
  section, so it would not be a link in the release notes.
- Each release gets `[X.Y.Z]: https://github.com/<owner>/<repo>/releases/tag/vX.Y.Z` at
  the top of that final block. `<owner>/<repo>` is `GITHUB_REPOSITORY` in GitHub Actions
  (`GITHUB_ACTIONS=true`), and the `origin` remote's anywhere else (GitHub only);
  `repository` in `package.json` is not read. There is no `[Unreleased]: .../compare/...` link; nothing
  would keep it up to date.
- LF line endings. In a repository worked on from Windows, keep Git from checking it out
  with CRLF:

  ```gitattributes
  CHANGELOG.md text eol=lf
  ```

`release-prepare.mjs` checks everything before it writes anything, and writes nothing when
it fails. It fails when:

- `## [Unreleased]` is missing, empty or duplicated. Right after a release this is
  expected, and every push to `main` gets a red tagpr run until a pull request adds an
  entry.
- **A version section newer than the latest tag is already there** (the latest tag is
  `TAGPR_CURRENT_VERSION`, which tagpr passes along with `TAGPR_NEXT_VERSION`). tagpr
  rebuilds the release branch from `main` every time, so this means a release pull request
  was merged and its tag was never made — usually because the tagpr run of the merge
  commit was cancelled by a later push, or because the tag was deleted to be made again.
  Re-run that run (or push the tag again). Merging a new release pull request would tag a
  later commit with changes the CHANGELOG does not list.

  It checks every section, not only the proposed version's. tagpr counts from the latest
  tag, so if a pull request labelled for a minor release lands while `1.2.4` is untagged,
  it proposes `1.3.0`; a check for `## [1.3.0]` alone would pass and stack `1.3.0` on an
  untagged `1.2.4`.

  It only compares with the latest tag. **Once a later tag exists, an older untagged
  section is no longer caught** (tag `v1.2.5` by hand while `1.2.4` is untagged, and
  `1.2.4` never gets a tag). That is why the message lists every untagged section, oldest
  first: tag them in that order. With no tags at all (tagpr passes `v0.0.0`) the check is
  skipped, so a repository that released without tags can adopt tagpr.
- A file in `versionFile` is not at the version tagpr proposes, or is not a `*.json` or a
  `Cargo.toml`.
- `package.json` is not at the version tagpr proposes (not checked with `--changelog-only`).
- With `--cargo <dir>`, the `[package]` version in `<dir>/Cargo.toml` is not the proposed
  one, or `<dir>/Cargo.lock` does not have exactly one entry for the package without a
  `source`.
- The GitHub repository cannot be told: outside GitHub Actions, `origin` is missing or not
  on GitHub.

tagpr still updates the release pull request when the command fails, so **do not merge a
release pull request whose tagpr run is red.**

## Using it

```bash
git subtree add -S --prefix=scripts/tagpr https://github.com/kan/tagpr-scripts.git main --squash
```

**Pass `-S`.** `git subtree` makes its commits with `commit-tree`, which ignores
`commit.gpgsign`, so without it they are left unsigned.

**Let GitHub Actions create pull requests** (Settings → Actions → General → "Allow GitHub
Actions to create and approve pull requests", or `can_approve_pull_request_reviews` in the
API). Without it, the first tagpr run fails with 403 "GitHub Actions is not permitted to
create or approve pull requests". The default token permissions can stay read-only.

`.tagpr` for an npm package:

```ini
[tagpr]
	releaseBranch = main
	vPrefix = true
	versionFile = package.json
	changelog = false
	release = false
	postVersionCommand = node scripts/tagpr/release-prepare.mjs
	minorLabels = enhancement
```

In the publish workflow, before `npm ci`:

```yaml
- name: Extract the CHANGELOG section
  env:
    TAG: ${{ github.ref_name }}
  run: node scripts/tagpr/release-notes.mjs "$TAG" > "$RUNNER_TEMP/release-notes.md"
```

and after `npm publish`, `gh release create "$TAG" --verify-tag --notes-file "$RUNNER_TEMP/release-notes.md"`.
[lily's workflows](https://github.com/kan/lily/tree/main/.github/workflows) are a complete
example: `tagpr.yml` starts `publish.yml` with `workflow_dispatch` on the new tag.

For a repository that keeps its version only in tags (Go, for example), set
`versionFile = -` and `postVersionCommand = node scripts/tagpr/release-prepare.mjs --changelog-only`.
[notify-slack's workflows](https://github.com/kan/notify-slack/tree/main/.github/workflows)
are a complete example with GoReleaser.

For a Tauri app, list every file that holds the version, and let `--cargo` bring
`Cargo.lock` along (drop `--changelog-only` when `package.json` is in the list too):

```ini
	versionFile = src-tauri/tauri.conf.json,src-tauri/Cargo.toml
	postVersionCommand = node scripts/tagpr/release-prepare.mjs --changelog-only --cargo src-tauri
```

All of the files listed are checked, `tauri.conf.json` included, so a wrapper does not need
to check them again.

## Changing it

**Change this repository, never the copy in `scripts/tagpr/`.** Then, in each repository:

```bash
git subtree pull -S --prefix=scripts/tagpr https://github.com/kan/tagpr-scripts.git main --squash
```

**Merge that pull request with a merge commit, even in a repository that squashes
everything else.** `git subtree` finds the last import from the commits it made; a squash
merge folds them away and breaks the next pull.

The whole repository comes along, `.github/` included. In `scripts/tagpr/.github/` it is
inert — GitHub only reads the root's — and it stays so the copy matches this repository.

```bash
node --test 'test/*.node-test.mjs'
```

**The tests are not named `*.test.mjs`.** Vitest's default `include` would pick them up
in the repositories that pull this in, and fail on them as files with no Vitest suite.
