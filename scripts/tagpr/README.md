# tagpr-scripts

The scripts that let [tagpr](https://github.com/Songmu/tagpr) release an npm package whose
`CHANGELOG.md` is written by hand. Shared by [lily](https://github.com/kan/lily),
[wema](https://github.com/kan/wema) and [wema-kake](https://github.com/kan/wema-kake),
which pull this repository in with `git subtree` at `scripts/tagpr/`.

| File | Run by | Does |
|---|---|---|
| `release-prepare.mjs` | tagpr, as `postVersionCommand` | Turns `## [Unreleased]` into `## [X.Y.Z] - <date in JST>`, adds the version's link definition, and brings `package-lock.json` to the version tagpr put in `package.json` (not with `--changelog-only`) |
| `release-notes.mjs` | The publish workflow | Prints one section of `CHANGELOG.md`, to check that it exists before publishing and to use it as the GitHub release notes. Given a tag `vX.Y.Z` it prints `## [X.Y.Z]`; given a pre-release tag (`vX.Y.Z-rc.1`, tagged by hand on `main`) it prints `## [Unreleased]` |
| `changelog.mjs` | (imported by both) | How a `CHANGELOG.md` is read and stamped. Written in one place so the release pull request and the publish never disagree. Knows nothing about npm |
| `npm-lock.mjs` | (imported by `release-prepare.mjs`) | The `package.json` / `package-lock.json` part |

**Only Node's built-in modules.** The tagpr job holds a token that can push to the
repository, so it runs no dependency code: no `npm ci`, nothing fetched at run time. That
is also why this is a subtree and not an npm package — the code is committed in each
repository and reviewed in its diffs.

Run both from the repository root: `CHANGELOG.md`, and for `release-prepare.mjs` without
`--changelog-only` also `package.json` and `package-lock.json` (lockfile version 2 or
later), are read from the working directory.

`release-prepare.mjs --changelog-only` touches `CHANGELOG.md` alone and reads neither
`package.json` nor the lock, for a repository with no version file (`versionFile = -`) or
one that brings its own version files and locks up to date in a wrapper of its own.

## What the CHANGELOG has to look like

[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), with these rules:

- Each pull request that changes what users get adds its line under `## [Unreleased]`.
  That section becomes the release notes word for word.
- A section ends at the next `## ` heading, or at the block of link definitions
  (`[x]: url`) at the end of the file. Headings inside code fences are not headings.
- Links inside entries are written inline. A reference link's definition sits outside the
  section, so it would not be a link in the release notes.
- Each release gets `[X.Y.Z]: https://github.com/<owner>/<repo>/releases/tag/vX.Y.Z` at
  the top of that final block. `<owner>/<repo>` is `GITHUB_REPOSITORY` in GitHub Actions
  (`GITHUB_ACTIONS=true`), and the `origin` remote's anywhere else (GitHub only);
  `repository` in `package.json` is not read. There is no `[Unreleased]: .../compare/...` link; nothing
  would keep it up to date.
- LF line endings.

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
- `package.json` is not at the version tagpr proposes (not checked with `--changelog-only`).
- The GitHub repository cannot be told: outside GitHub Actions, `origin` is missing or not
  on GitHub.

tagpr still updates the release pull request when the command fails, so **do not merge a
release pull request whose tagpr run is red.**

## Using it

```bash
git subtree add --prefix=scripts/tagpr https://github.com/kan/tagpr-scripts.git main --squash
```

`.tagpr`:

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

## Changing it

**Change this repository, never the copy in `scripts/tagpr/`.** Then, in each repository:

```bash
git subtree pull --prefix=scripts/tagpr https://github.com/kan/tagpr-scripts.git main --squash
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
