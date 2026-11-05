/**
 * **tagpr が書いた版のファイルを確かめる**（`release-prepare.mjs` の検査）。import しても何も
 * 実行しない。
 *
 * tagpr は `.tagpr` の `versionFile` に並べた各ファイルで、今の版の文字列の**最初の 1 か所**を
 * 書き換える。その前に同じ文字列（依存の版など）があると、そちらが書き換わり、本来の版の行は
 * 古いまま残る。何も落ちず、マージ後のタグは先頭のファイルの版から決まる。そこで、並べた全部の
 * ファイルで、本来の版の場所が提案の版になっているかを見る。
 */
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { basename, extname } from 'node:path';
import { readPackage } from './cargo-lock.mjs';
import { ReleaseError, readRootFile } from './changelog.mjs';

/** 作業ディレクトリ（repo のルート）からの相対パス */
const tagprPath = '.tagpr';

/**
 * `.tagpr` の `versionFile` の一覧。`.tagpr` が無いか、`versionFile = -`（版をタグだけで持つ）
 * なら空。読み方は tagpr と同じ git config の形式なので、`git config` に任せる。
 * @returns {string[]}
 */
function tagprVersionFiles() {
  let value;
  try {
    value = execFileSync('git', ['config', '--file', tagprPath, '--get', 'tagpr.versionFile'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim();
  } catch (e) {
    // 終了コード 1 はキーが無い。ファイルが無いときも 1 になる
    if (e?.status === 1) return [];
    throw new ReleaseError(`Cannot read versionFile from ${tagprPath}: ${String(e?.stderr ?? e?.message).trim()}`);
  }
  if (value === '-') return [];
  return value.split(',').map((f) => f.trim()).filter((f) => f !== '');
}

/**
 * 1 つのファイルの本来の版を読む。`.json` は最上位の `"version"`、`Cargo.toml` は `[package]` の
 * `version`。ほかの種類は、どこが本来の版かを決められないので落とす。
 * @param {string} path
 * @returns {string | undefined}
 */
function ownVersion(path) {
  const json = extname(path) === '.json';
  if (!json && basename(path) !== 'Cargo.toml') {
    throw new ReleaseError(
      `${path} is in versionFile of ${tagprPath}, but only *.json ("version") and Cargo.toml ([package] version) can be checked`,
    );
  }
  // readRootFile の「ルートで実行せよ」は、ここでは versionFile の誤りのことが多い
  if (!existsSync(path)) throw new ReleaseError(`${path} is in versionFile of ${tagprPath}, but does not exist`);
  const text = readRootFile(path);
  if (!json) return readPackage(text, path).version;
  let data;
  try {
    data = JSON.parse(text);
  } catch (e) {
    throw new ReleaseError(`${path} is not valid JSON: ${e.message}`);
  }
  if (typeof data?.version !== 'string') {
    // 入れ子の版（Expo の expo.version など）は、どこが本来の版かを決められないので扱わない
    throw new ReleaseError(`${path} has no top-level "version" string to check`);
  }
  return data.version;
}

/**
 * `.tagpr` の `versionFile` の全部のファイルが、提案の版になっているかを確かめる。
 * package.json と Cargo.toml は npm-lock.mjs / cargo-lock.mjs も確かめるが、そちらは
 * 「versionFile に入っていない」ときに気付くためのもので、ここと役目が違う
 * @param {string} version tagpr が提案する版（`X.Y.Z`）
 */
export function checkVersionFiles(version) {
  for (const path of tagprVersionFiles()) {
    const found = ownVersion(path);
    if (found !== version) {
      throw new ReleaseError(
        `${path} says ${found}, but tagpr proposes ${version}. tagpr rewrites only the first occurrence of the current version in each versionFile; if the same string appears earlier in the file, that one was rewritten instead`,
      );
    }
  }
}
