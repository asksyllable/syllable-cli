#!/usr/bin/env python3
"""Verify a change to Casks/syllable.rb mechanically, in place of a human review.

A cask change passes only if:
- the lines allowed to change between releases (version, sha256, url, and the
  legacy `verified:` hint) each match an exact full-line pattern, so nothing
  can ride along after a valid value (`url "…"; system(…)` fails);
- there is exactly one version line and exactly four url/sha256 pairs, one per
  platform (darwin/linux x amd64/arm64), each url being this repo's release
  tarball and each sha256 matching that release's published checksums file;
- every other line is unchanged as a multiset, so blocks may be reordered by a
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

# Lines whose values legitimately change per release. Anything that starts like
# one of these must match its pattern exactly, end of line included.
VALUE_START = re.compile(r"^(version|sha256|url|verified)\b")
VERSION_RE = re.compile(r'^version "([0-9]+\.[0-9]+\.[0-9]+)"$')
SHA_RE = re.compile(r'^sha256 "([0-9a-f]{64})"$')
URL_RE = re.compile(
    r'^url "https://github\.com/asksyllable/syllable-cli/releases/download/v#\{version\}/'
    r'syllable_#\{version\}_(darwin|linux)_(amd64|arm64)\.tar\.gz"(,?)$')
# `verified:` is a download-host hint GoReleaser v2.18 stopped emitting. It may
# only appear as the continuation of a url line that ends with a comma.
VERIFIED_RE = re.compile(r'^verified: "github\.com/asksyllable/syllable-cli/?"$')


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def show(ref, path):
    try:
        return git("show", f"{ref}:{path}")
    except subprocess.CalledProcessError:
        return None


def lines_of(text):
    return [raw.strip() for raw in text.splitlines()]


def structure(text):
    """Cask lines that must not change: everything except values, comments, blanks."""
    return sorted(l for l in lines_of(text) if l and not l.startswith("#") and not VALUE_START.match(l))


def check(old, new, sums):
    """Return a list of problems with `new` (the cask text); empty means it passes.
    `old` is the base cask text (None if the cask is new); `sums` maps asset
    file names to sha256 values from the release's checksums file."""
    errors = []
    lines = [l for l in lines_of(new) if l and not l.startswith("#")]

    versions, pairs, pending_sha = [], [], None
    for i, line in enumerate(lines):
        if not VALUE_START.match(line):
            if pending_sha is not None:
                errors.append(f"sha256 not followed by its url: next line is {line!r}")
                pending_sha = None
            continue
        if line.startswith("version"):
            m = VERSION_RE.match(line)
            if not m:
                errors.append(f"version line is not exactly `version \"X.Y.Z\"`: {line!r}")
            else:
                versions.append(m.group(1))
        elif line.startswith("sha256"):
            m = SHA_RE.match(line)
            if not m:
                errors.append(f"sha256 line is not exactly `sha256 \"<64 hex>\"`: {line!r}")
            if pending_sha is not None:
                errors.append("two sha256 lines without a url between them")
            pending_sha = m.group(1) if m else None
        elif line.startswith("url"):
            m = URL_RE.match(line)
            if not m:
                errors.append(f"url line is not exactly this repo's release tarball: {line!r}")
            else:
                if m.group(3) == "," and not (i + 1 < len(lines) and VERIFIED_RE.match(lines[i + 1])):
                    errors.append(f"url line ends with a comma but is not followed by a verified: hint: {line!r}")
                pairs.append((f"{m.group(1)}_{m.group(2)}", pending_sha))
            pending_sha = None
        else:  # verified
            if not VERIFIED_RE.match(line):
                errors.append(f"verified: line is not exactly this repo's host: {line!r}")
            elif i == 0 or not URL_RE.match(lines[i - 1]) or not lines[i - 1].endswith(","):
                errors.append("verified: hint does not continue a url line")
    if pending_sha is not None:
        errors.append("sha256 at the end of the cask without a url")

    if len(versions) != 1:
        errors.append(f"expected exactly one version line, found {len(versions)}")
        return errors
    version = versions[0]

    if old is not None and structure(old) != structure(new):
        added = sorted(set(structure(new)) - set(structure(old)))
        removed = sorted(set(structure(old)) - set(structure(new)))
        errors.append(f"cask structure changed beyond version/url/sha256 values: added {added}, removed {removed}")

    seen = [p for p, _ in pairs]
    if sorted(seen) != sorted(PLATFORMS):
        errors.append(f"platforms present {sorted(seen)}, expected each of {sorted(PLATFORMS)} exactly once")
    for platform, sha in pairs:
        asset = f"syllable_{version}_{platform}.tar.gz"
        if sha is None:
            errors.append(f"{platform}: url has no preceding sha256")
        elif sums.get(asset) != sha:
            errors.append(f"{platform}: sha256 {sha} does not match release checksum {sums.get(asset)}")
    return errors


def parse_sums(text):
    sums = {}
    for line in text.splitlines():
        parts = line.split()
        if len(parts) == 2:
            sums[parts[1]] = parts[0]
    return sums


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
    m = re.search(r'^\s*version "([0-9]+\.[0-9]+\.[0-9]+)"\s*$', new, re.M)
    if not m:
        fail(["no valid `version` line"])
    version = m.group(1)

    if args.checksums:
        sums_text = open(args.checksums).read()
    else:
        sums_text = subprocess.run(
            ["gh", "release", "download", f"v{version}", "--repo", REPO,
             "--pattern", f"syllable_{version}_checksums.txt", "-O", "-"],
            check=True, capture_output=True, text=True).stdout

    errors = check(old, new, parse_sums(sums_text))
    if errors:
        fail(errors)
    print(f"OK: {CASK} v{version}: all four url/sha256 pairs match the v{version} release checksums; structure unchanged")


if __name__ == "__main__":
    main()
