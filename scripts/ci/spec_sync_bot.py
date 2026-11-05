#!/usr/bin/env python3
"""Plan one run of the spec-sync bot (no AI involved).

Given the embedded spec, the upstream spec (fetched at a pinned commit), the
CLI's operation map, and the map from a build with the upstream spec (where
`go generate ./cmd` named commands for new operations), classify the change
(classify_drift.py) and decide:

  none       Already in sync.
  busy       A reconciliation PR from someone else (the local routine or a
             person) is open; stay out of its way.
  ship       Syncing the spec is enough (new operations included, when they
             got generated commands): write the new spec and docs marker into
             the working tree, plus the commit message, PR title, and PR body
             files.
  reconcile  CLI code must change first (see classify_drift.py), or the CLI
             didn't build with the new spec. Note it once on the spec-drift
             issue for the local spec-sync routine.

No change waits for a person. After a shipped sync merges, `--file-followups`
files its follow-ups (flags the API's change left behind) as `cli-followup`
issues, which the local routine works through. If the test suite fails on the
bot's commit, the workflow hands the sync to the routine the same way as a
reconcile.

Writes `action=...` and `delta=...` to stdout for $GITHUB_OUTPUT.
Issue text follows CONTRIBUTING's Public repo hygiene rules.
"""
import argparse
import hashlib
import json
import os
import re
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from classify_drift import classify  # noqa: E402

REPO = os.environ.get("GITHUB_REPOSITORY", "asksyllable/syllable-cli")
BOT_BRANCH = "spec-sync/bot"
SPEC = "scripts/syllable-cli/internal/spec/openapi.json"
SKILL = ".claude/skills/syllable-cli/SKILL.md"
FOLLOWUP_LABEL = "cli-followup"
RECONCILE_MARKER = "<!-- spec-sync-bot reconcile delta: {delta} -->"


# A closing keyword only matters before an issue reference or URL.
CLOSING_KEYWORD = re.compile(r"\b(clos|fix|resolv)(?=(e|es|ed|s)?\b:?\s+(#|https?://|[\w.-]+/[\w.-]+#))", re.I)
MENTION_OR_REF = re.compile(r"([@#])(?=[A-Za-z0-9])")
CONTROL = re.compile("[\x00-\x08\x0b-\x1f\x7f-\x9f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]")


def inert(text):
    """Text from the upstream spec, made inert before the bot posts it on
    GitHub: no control characters, @mentions, issue references, closing
    keywords (a merged PR saying "fixes #12" would close #12), or HTML
    comments. Line breaks are kept."""
    text = CONTROL.sub(" ", text)
    text = CLOSING_KEYWORD.sub(lambda m: m.group(1)[:2] + "\u200b" + m.group(1)[2:], text)
    text = MENTION_OR_REF.sub("\\1\u200b", text)
    return text.replace("<!--", "&lt;!--")


def fenced(text):
    """A code block that the text inside can't close."""
    runs = re.findall(r"`+", text)
    fence = "`" * max(3, max((len(r) for r in runs), default=0) + 1)
    return f"{fence}\n{text}\n{fence}"


def gh(*args, check=True):
    return subprocess.run(["gh", *args], check=check, capture_output=True, text=True).stdout


def gh_json(*args):
    out = gh(*args)
    return json.loads(out) if out.strip() else []


def reconcile_comment(delta, items, dry_run=False):
    """Ask the local spec-sync routine to reconcile, once per change, on the
    open spec-drift issue."""
    drift = gh_json("issue", "list", "--repo", REPO, "--label", "spec-drift", "--state", "open", "--json", "number")
    marker = RECONCILE_MARKER.format(delta=delta)
    if not drift or dry_run:
        return
    n = drift[0]["number"]
    comments = gh("issue", "view", str(n), "--repo", REPO, "--json", "comments", "--jq", ".comments[].body")
    if marker in comments:
        return
    lines = "\n".join(f"- {r}" for r in items)
    gh("issue", "comment", str(n), "--repo", REPO, "--body",
       f"CLI code must change before this spec change can land, so the local spec-sync routine reconciles it:"
       f"\n\n{lines}\n\n{marker}")


def file_followups(followups, pr=""):
    """One `cli-followup` issue per follow-up, never filed twice (a closed one
    means the routine already handled or declined it)."""
    if not followups:
        return
    gh("label", "create", FOLLOWUP_LABEL, "--repo", REPO, "--color", "C5DEF5",
       "--description", "CLI polish the spec-sync routine picks up", check=False)
    existing = gh("issue", "list", "--repo", REPO, "--label", FOLLOWUP_LABEL, "--state", "all", "--limit", "500",
                  "--json", "body", "--jq", ".[].body")
    shipped = f" in #{pr}" if pr else ""
    for text in followups:
        marker = f"<!-- cli-followup: {hashlib.sha256(text.encode()).hexdigest()[:16]} -->"
        if marker in existing:
            continue
        title = re.sub(r"\s+", " ", text.replace("`", ""))
        title = "Follow-up: " + (title if len(title) <= 100 else title[:99].rstrip() + "…")
        gh("issue", "create", "--repo", REPO, "--label", FOLLOWUP_LABEL, "--title", title, "--body",
           f"{text}\n\nThe spec sync{shipped} shipped this API change, but the CLI's flags haven't caught up. "
           f"The local spec-sync routine works through open `{FOLLOWUP_LABEL}` issues.\n\n{marker}")


def next_tag(minor):
    tags = subprocess.run(["git", "tag", "--list", "v*", "--sort=-v:refname"],
                          check=True, capture_output=True, text=True).stdout.split()
    for t in tags:
        m = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", t)
        if m:
            a, b, c = map(int, m.groups())
            return f"v{a}.{b + 1}.0" if minor else f"v{a}.{b}.{c + 1}"
    return "v0.1.0" if minor else "v0.0.1"


def bump_skill_marker(version, notes, new_commands):
    text = open(SKILL).read()
    m = re.search(r"^> \*\*Docs synced to CLI v[0-9.]+\*\*(.*)$", text, re.M)
    if not m:
        return
    kind = "(minor) highlights: spec sync with new generated commands." if new_commands else "(patch) highlights: field-only spec sync."
    highlight = f"> {version} {kind} " + " ".join(notes)
    replacement = f"> **Docs synced to CLI {version}**{m.group(1)}\n>\n{highlight}"
    text = text[:m.start()] + replacement + text[m.end():]
    open(SKILL, "w").write(text)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--upstream")
    ap.add_argument("--upstream-sha")
    ap.add_argument("--operations")
    ap.add_argument("--candidate-operations",
                    help="`__operations` from a build with the upstream spec; missing or empty if that build failed")
    ap.add_argument("--out-dir")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--file-followups", metavar="CLASSIFICATION_JSON",
                    help="after a merge: file the follow-ups from this run's classification.json, then exit")
    ap.add_argument("--pr", default="", help="with --file-followups: the merged sync PR's number")
    ap.add_argument("--handover", metavar="DELTA",
                    help="the tests failed on the bot's commit: hand this change to the routine, then exit")
    ap.add_argument("--run-url", default="")
    args = ap.parse_args()
    if args.file_followups:
        return file_followups(json.load(open(args.file_followups)).get("followups", []), args.pr)
    if args.handover:
        return reconcile_comment(args.handover, [
            f"the test suite failed with the new spec{f' ({args.run_url})' if args.run_url else ''}, so CLI code or "
            f"tests must change first"])
    missing = [f"--{k.replace('_', '-')}" for k in ("upstream", "upstream_sha", "operations", "out_dir") if not getattr(args, k)]
    if missing:
        ap.error("missing " + ", ".join(missing))

    # The working tree may already hold the upstream spec (the candidate build).
    old = json.loads(subprocess.run(["git", "show", f"HEAD:{SPEC}"], check=True,
                                    capture_output=True, text=True).stdout)
    new = json.load(open(args.upstream))
    candidate = None
    if args.candidate_operations:
        try:
            candidate = json.load(open(args.candidate_operations))
        except (OSError, ValueError):
            candidate = None
    result = classify(old, new, json.load(open(args.operations)), candidate)
    for key in ("reconcile", "release_notes", "followups", "ship_reasons"):
        result[key] = [inert(line) for line in result[key]]
    # A failed candidate build blocks the sync even when the contract diff is
    # empty: an operationId rename alone breaks a hand-written mapping.
    if args.candidate_operations and candidate is None and new != old:
        result["has_changes"] = True
        result["verdict"] = "reconcile"
        result["reconcile"].append(
            "the CLI didn't build with the new spec, or `go generate ./cmd` couldn't name a command for every operation (see the bot's run log)")
    os.makedirs(args.out_dir, exist_ok=True)
    json.dump(result, open(os.path.join(args.out_dir, "classification.json"), "w"), indent=2)
    print(f"Classification: {json.dumps(result, indent=2)}", file=sys.stderr)

    def emit(action, **extra):
        print(f"action={action}")
        for k, v in extra.items():
            print(f"{k}={v}")

    if not result["has_changes"]:
        return emit("none")

    others = [p for p in gh_json("pr", "list", "--repo", REPO, "--label", "spec-sync", "--state", "open",
                                 "--json", "number,headRefName") if p["headRefName"] != BOT_BRANCH]
    if others:
        print(f"Open reconciliation PR(s) {[p['number'] for p in others]}; leaving drift to them.", file=sys.stderr)
        return emit("busy")

    if result["verdict"] == "reconcile":
        reconcile_comment(result["delta"], result["reconcile"], args.dry_run)
        return emit("reconcile", delta=result["delta"])

    # ship
    new_commands = result["new_commands"]
    version = next_tag(minor=bool(new_commands))
    notes = result["release_notes"] or ["Upstream spec refresh; no user-visible changes."]
    if not args.dry_run:
        with open(SPEC, "w") as f:
            f.write(open(args.upstream).read())
        bump_skill_marker(version, notes, new_commands)

    old_path = os.path.join(args.out_dir, "embedded-before.json")
    json.dump(old, open(old_path, "w"))
    diff_report = inert(subprocess.run(
        ["python3", ".claude/skills/syllable-sync/scripts/diff_specs.py", old_path, args.upstream],
        capture_output=True, text=True).stdout.strip())
    drift = gh_json("issue", "list", "--repo", REPO, "--label", "spec-drift", "--state", "open", "--json", "number")
    closes = "\n".join(f"Closes #{i['number']}" for i in drift)
    followups = "\n".join(f"- {f}" for f in result["followups"]) or "None."
    body = f"""Automated spec sync to upstream SDK commit `{args.upstream_sha}` by the spec-sync bot (`.github/workflows/spec-sync-bot.yml`). The classifier rated it **safe to ship**: every change is either additive or a rule the API enforces whatever CLI version sends the request. The full test suite ran on this exact commit before merge.

## diff_specs.py report
{fenced(diff_report)}

## Breaking changes
None that holding the CLI back would prevent. The release notes list each change a user can notice, including rules the API now enforces and anything it removed.

## Follow-ups
{followups}

After the merge, the bot files these as `{FOLLOWUP_LABEL}` issues for the local spec-sync routine.

## Release notes
{chr(10).join(notes)}

{closes}
"""
    # New commands are a feature, so the release train makes it a minor release.
    title = "feat: sync CLI to OpenAPI spec, with new commands" if new_commands else "chore: sync CLI to OpenAPI spec"
    open(os.path.join(args.out_dir, "pr-body.md"), "w").write(body)
    open(os.path.join(args.out_dir, "pr-title.txt"), "w").write(title + "\n")
    open(os.path.join(args.out_dir, "commit-msg.txt"), "w").write(
        f"{title}\n\nAutomated sync to upstream SDK commit {args.upstream_sha} (classifier delta {result['delta']}).\n")
    emit("ship", delta=result["delta"])


if __name__ == "__main__":
    main()
