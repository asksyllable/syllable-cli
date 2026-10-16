#!/usr/bin/env python3
"""Verify a change to Casks/syllable.rb mechanically, in place of a human review.

A cask change passes only if:
- every url is this repo's release tarball for the cask's version, for all four
  platforms (darwin/linux x amd64/arm64), and each paired sha256 matches that
  release's published checksums file;
- apart from version, url, and sha256 values (and comments or blank lines), the
  cask's lines are unchanged as a multiset, so blocks may be reordered by a
  GoReleaser upgrade but no stanza, hook, or binary line can be added or edited;
- nothing else under Casks/ changed.

With --cask-only the change must also touch nothing but Casks/syllable.rb (used
by the release workflow before it marks the bot's cask PR as tested).

Usage: verify_cask.py --base <ref> --head <ref> [--cask-only] [--checksums FILE]
Requires `gh` (with GH_TOKEN) unless --checksums is given.
"""
import argparse
import re
import subprocess
import sys

CASK = "Casks/syllable.rb"
REPO = "asksyllable/syllable-cli"
PLATFORMS = {f"{os}_{arch}" for os in ("darwin", "linux") for arch in ("amd64", "arm64")}
# Lines whose values legitimately change per release. `verified:` is a download
# host hint that GoReleaser v2.18 stopped emitting; urls are checked separately.
VALUE_LINE = re.compile(r"^(version\s|sha256\s|url\s|verified:)")


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def show(ref, path):
    try:
        return git("show", f"{ref}:{path}")
    except subprocess.CalledProcessError:
        return None


def structure(text):
    """Cask lines that must not change: everything except values, comments, blanks."""
    lines = []
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or VALUE_LINE.match(line):
            continue
        lines.append(line)
    return sorted(lines)


def pairs(text):
    """(url, sha256) pairs in file order; a sha256 line pairs with the next url line."""
    out, sha = [], None
    for raw in text.splitlines():
        line = raw.strip()
        m = re.match(r'^sha256\s+"([0-9a-f]{64})"$', line)
        if m:
            sha = m.group(1)
            continue
        m = re.match(r'^url\s+"([^"]+)"', line)
        if m:
            out.append((m.group(1), sha))
            sha = None
    return out


def fail(errors):
    for e in errors:
        print(f"FAIL: {e}")
    sys.exit(1)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", required=True)
    ap.add_argument("--head", required=True)
    ap.add_argument("--cask-only", action="store_true")
    ap.add_argument("--checksums", help="local checksums file instead of downloading from the release")
    args = ap.parse_args()

    merge_base = git("merge-base", args.base, args.head).strip()
    changed = [p for p in git("diff", "--name-only", merge_base, args.head).splitlines() if p]

    if args.cask_only and changed != [CASK]:
        fail([f"expected a change to {CASK} only, got: {changed or 'no changes'}"])
    cask_paths = [p for p in changed if p.startswith("Casks/")]
    if not cask_paths:
        print(f"OK: {CASK} untouched ({len(changed)} other file(s) changed)")
        return
    if cask_paths != [CASK]:
        fail([f"unexpected files under Casks/: {cask_paths}"])

    old, new = show(merge_base, CASK), show(args.head, CASK)
    if new is None:
        fail([f"{CASK} was deleted"])
    errors = []

    m = re.search(r'^\s*version\s+"([0-9]+\.[0-9]+\.[0-9]+)"\s*$', new, re.M)
    if not m:
        fail(["no semver `version` line"])
    version = m.group(1)

    if old is not None and structure(old) != structure(new):
        added = sorted(set(structure(new)) - set(structure(old)))
        removed = sorted(set(structure(old)) - set(structure(new)))
        errors.append(f"cask structure changed beyond version/url/sha256 values: added {added}, removed {removed}")

    if args.checksums:
        sums_text = open(args.checksums).read()
    else:
        sums_text = subprocess.run(
            ["gh", "release", "download", f"v{version}", "--repo", REPO,
             "--pattern", f"syllable_{version}_checksums.txt", "-O", "-"],
            check=True, capture_output=True, text=True).stdout
    sums = {}
    for line in sums_text.splitlines():
        parts = line.split()
        if len(parts) == 2:
            sums[parts[1]] = parts[0]

    url_re = re.compile(
        rf"^https://github\.com/{re.escape(REPO)}/releases/download/v#\{{version\}}/syllable_#\{{version\}}_"
        r"(darwin|linux)_(amd64|arm64)\.tar\.gz$")
    seen = set()
    for url, sha in pairs(new):
        m = url_re.match(url)
        if not m:
            errors.append(f"url is not this repo's release tarball: {url}")
            continue
        platform = f"{m.group(1)}_{m.group(2)}"
        asset = f"syllable_{version}_{platform}.tar.gz"
        if platform in seen:
            errors.append(f"duplicate platform {platform}")
        seen.add(platform)
        if sha is None:
            errors.append(f"{platform}: url has no preceding sha256")
        elif sums.get(asset) != sha:
            errors.append(f"{platform}: sha256 {sha} does not match release checksum {sums.get(asset)}")
    if seen != PLATFORMS:
        errors.append(f"platforms present {sorted(seen)}, expected {sorted(PLATFORMS)}")

    if errors:
        fail(errors)
    print(f"OK: {CASK} v{version}: all four url/sha256 pairs match the v{version} release checksums; structure unchanged")


if __name__ == "__main__":
    main()
