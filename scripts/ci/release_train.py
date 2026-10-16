#!/usr/bin/env python3
"""Decide whether a live-checked main commit should be released, and as what.

Looks at the commits between the latest v* tag and the candidate commit.
A commit is *releasable* when its subject is `feat:`/`fix:` (any scope) or
`chore: sync CLI to OpenAPI spec`, and it changes shipped code: something under
scripts/syllable-cli/ other than tests. Docs, CI, test, and dependency-only
commits ride along with the next real release instead of cutting their own.

Version bump: minor if any releasable commit is `feat`, otherwise patch.
A breaking marker (`feat!:` or a `BREAKING CHANGE` footer) stops the train:
majors stay a human decision.

Release notes come from each releasable PR's `## Release notes` section, or
fall back to its title and number.

Prints `key=value` lines for $GITHUB_OUTPUT: release, breaking, tag, last_tag.
Usage: release_train.py --candidate SHA [--since TAG] [--notes-out FILE]
"""
import argparse
import json
import re
import subprocess
import sys

REPO = "asksyllable/syllable-cli"
RELEASABLE = re.compile(r"^(feat|fix)(\([^)]*\))?!?: |^chore: sync CLI to OpenAPI spec")
BREAKING_SUBJECT = re.compile(r"^\w+(\([^)]*\))?!: ")
PR_SUFFIX = re.compile(r"\(#(\d+)\)\s*$")


def run(*args):
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout


def ships_code(sha):
    for path in run("git", "show", "--name-only", "--format=", sha).splitlines():
        if path.startswith("scripts/syllable-cli/") and "/integration/" not in path and not path.endswith("_test.go"):
            return True
    return False


def release_notes_section(body):
    m = re.search(r"^## Release notes[ \t]*\n(.*?)(?=^## |\Z)", body or "", re.M | re.S)
    return m.group(1).strip() if m else ""


def pr_info(number):
    try:
        return json.loads(run("gh", "pr", "view", str(number), "--repo", REPO, "--json", "title,body,number"))
    except subprocess.CalledProcessError:
        return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--candidate", required=True)
    ap.add_argument("--since", help="previous tag (default: latest v* tag reachable from the candidate)")
    ap.add_argument("--notes-out")
    args = ap.parse_args()

    last = args.since or run("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*", args.candidate).strip()
    m = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", last)
    if not m:
        sys.exit(f"cannot parse previous tag {last!r}")
    major, minor, patch = map(int, m.groups())

    log = run("git", "log", "--format=%H%x1f%s%x1f%b%x1e", f"{last}..{args.candidate}")
    releasable, breaking = [], False
    for entry in filter(None, (e.strip() for e in log.split("\x1e"))):
        sha, subject, body = (entry.split("\x1f") + ["", ""])[:3]
        if BREAKING_SUBJECT.match(subject) or "BREAKING CHANGE" in body:
            breaking = True
        if RELEASABLE.match(subject) and ships_code(sha):
            releasable.append((sha, subject))

    out = {"last_tag": last, "breaking": str(breaking).lower(), "release": "false", "tag": ""}
    if breaking:
        print("Breaking change since the last release; majors stay a human decision.", file=sys.stderr)
    elif releasable:
        if any(s.startswith("feat") for _, s in releasable):
            minor, patch = minor + 1, 0
        else:
            patch += 1
        out["release"], out["tag"] = "true", f"v{major}.{minor}.{patch}"

        parts, refs = [], []
        for _, subject in reversed(releasable):  # oldest first
            pm = PR_SUFFIX.search(subject)
            info = pr_info(pm.group(1)) if pm else None
            title = re.sub(r"^\w+(\([^)]*\))?: ", "", PR_SUFFIX.sub("", subject).strip())
            title = title[:1].upper() + title[1:]
            section = release_notes_section(info["body"]) if info else ""
            ref = f"#{info['number']}" if info else ""
            if section:
                parts.append(f"{section} ({ref})" if ref and "\n" not in section else f"{section}\n\n({ref})" if ref else section)
            else:
                parts.append(f"- {title} ({ref})" if ref else f"- {title}")
            if ref:
                refs.append(ref)
        notes = "## Release notes\n\n" + "\n\n".join(parts) + "\n"
        if args.notes_out:
            open(args.notes_out, "w").write(notes)
        print(f"Releasing {out['tag']} from {len(releasable)} change(s): {', '.join(refs)}", file=sys.stderr)
    else:
        print(f"Nothing releasable since {last}.", file=sys.stderr)

    for k, v in out.items():
        print(f"{k}={v}")


if __name__ == "__main__":
    main()
