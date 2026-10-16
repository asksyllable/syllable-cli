#!/usr/bin/env python3
"""Plan one run of the spec-sync bot (no AI involved).

Given the embedded spec, the upstream spec (fetched at a pinned commit), and
the CLI's operation map, classify the change (classify_drift.py) and decide:

  none       Already in sync.
  busy       A reconciliation PR from someone else (the local routine or a
             person) is open; stay out of its way.
  ship       Safe change, or a breaking change a human accepted for exactly
             this delta: write the new spec and docs marker into the working
             tree, plus the commit message and PR body files.
  wait       A decision issue for this exact delta is open and not yet
             accepted. Nothing to do.
  decision   Breaking change: open a decision issue (superseding older ones).
  reconcile  Operations were added or removed: commands must change first.
             Note it once on the spec-drift issue.

Writes `action=...` (and `accepted_issue=`) to stdout for $GITHUB_OUTPUT.
Issue text follows CONTRIBUTING's Public repo hygiene rules.
"""
import argparse
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
DECISION_LABEL = "spec-decision"
# Only these GitHub users' decision:accept labels count (comma-separated).
DECIDERS = {u.strip() for u in os.environ.get("SPEC_DECIDERS", "yorkable").split(",") if u.strip()}
ACCEPT_LABEL = "decision:accept"


def gh(*args, check=True):
    return subprocess.run(["gh", *args], check=check, capture_output=True, text=True).stdout


def gh_json(*args):
    out = gh(*args)
    return json.loads(out) if out.strip() else []


def accepted_by(number):
    """Who most recently applied the accept label to this issue."""
    logins = gh("api", "--paginate", f"repos/{REPO}/issues/{number}/events", "--jq",
                f'.[] | select(.event == "labeled" and .label.name == "{ACCEPT_LABEL}") | .actor.login').split()
    return logins[-1] if logins else ""


def ensure_labels():
    gh("label", "create", DECISION_LABEL, "--repo", REPO, "--color", "D93F0B",
       "--description", "A breaking upstream API change waiting for a human decision", check=False)
    gh("label", "create", ACCEPT_LABEL, "--repo", REPO, "--color", "0E8A16",
       "--description", "Ship this exact spec change (spec-sync bot)", check=False)


def next_patch_tag():
    tags = subprocess.run(["git", "tag", "--list", "v*", "--sort=-v:refname"],
                          check=True, capture_output=True, text=True).stdout.split()
    for t in tags:
        m = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", t)
        if m:
            a, b, c = map(int, m.groups())
            return f"v{a}.{b}.{c + 1}"
    return "v0.0.1"


def bump_skill_marker(version, notes):
    text = open(SKILL).read()
    m = re.search(r"^> \*\*Docs synced to CLI v[0-9.]+\*\*(.*)$", text, re.M)
    if not m:
        return
    highlight = f"> {version} (patch) highlights: field-only spec sync. " + " ".join(notes)
    replacement = f"> **Docs synced to CLI {version}**{m.group(1)}\n>\n{highlight}"
    text = text[:m.start()] + replacement + text[m.end():]
    open(SKILL, "w").write(text)


def decision_body(result, upstream_sha):
    lines = "\n".join(f"- {d}" for d in result["decisions"])
    safe = "\n".join(f"- {n}" for n in result["release_notes"]) or "- (nothing else)"
    return f"""The upstream API spec (SDK commit `{upstream_sha[:10]}`) contains changes that could break existing CLI invocations, so the spec-sync bot won't ship them on its own.

**Needs a decision:**
{lines}

**Also in this change (safe on its own):**
{safe}

**Recommended: accept.** These changes already happened on the server, so syncing makes the CLI's help, `schema` output, and `--dry-run` checks match what the API enforces. Hold off only if users need a warning first, or if a command needs a new flag before the change ships.

**To ship it:** add the `{ACCEPT_LABEL}` label. The bot's next run syncs exactly this change and closes this issue. If upstream changes again first, the bot opens a fresh issue and closes this one, because an acceptance only applies to the change it was given for.

<!-- spec-decision delta: {result['delta']} upstream: {upstream_sha} -->
"""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--upstream", required=True)
    ap.add_argument("--upstream-sha", required=True)
    ap.add_argument("--operations", required=True)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    old = json.load(open(SPEC))
    new = json.load(open(args.upstream))
    result = classify(old, new, json.load(open(args.operations)))
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

    accepted_issue = ""
    if result["verdict"] == "reconcile":
        drift = gh_json("issue", "list", "--repo", REPO, "--label", "spec-drift", "--state", "open", "--json", "number")
        marker = f"<!-- spec-sync-bot reconcile delta: {result['delta']} -->"
        if drift and not args.dry_run:
            n = drift[0]["number"]
            comments = gh("issue", "view", str(n), "--repo", REPO, "--json", "comments", "--jq", ".comments[].body")
            if marker not in comments:
                items = "\n".join(f"- {r}" for r in result["reconcile"])
                gh("issue", "comment", str(n), "--repo", REPO, "--body",
                   f"This change adds or removes API operations, so CLI commands must change before the spec can land. "
                   f"The spec-sync bot can't write commands; the local spec-sync routine (or a person) needs to reconcile it:\n\n{items}\n\n{marker}")
        return emit("reconcile")

    if result["verdict"] == "decision":
        ensure_labels() if not args.dry_run else None
        issues = gh_json("issue", "list", "--repo", REPO, "--label", DECISION_LABEL, "--state", "open",
                         "--json", "number,body,labels")
        mine = [i for i in issues if f"delta: {result['delta']} " in (i["body"] or "")]
        if mine:
            issue = mine[0]
            if ACCEPT_LABEL not in {l["name"] for l in issue["labels"]}:
                return emit("wait")
            who = accepted_by(issue["number"])
            if who not in DECIDERS:
                print(f"#{issue['number']} was accepted by {who or 'unknown'}, who is not in SPEC_DECIDERS; waiting.", file=sys.stderr)
                return emit("wait")
            accepted_issue = str(issue["number"])
        else:
            if args.dry_run:
                return emit("decision")
            created = gh("issue", "create", "--repo", REPO, "--label", DECISION_LABEL, "--assignee", "yorkable",
                         "--title", "Spec decision: upstream API change needs a yes or no",
                         "--body", decision_body(result, args.upstream_sha)).strip()
            for old_issue in issues:
                gh("issue", "close", str(old_issue["number"]), "--repo", REPO, "--comment",
                   f"Upstream changed again, so this decision no longer matches what would ship. Superseded by {created}.")
            return emit("decision")

    # ship: a safe change, or exactly the change a human accepted.
    version = next_patch_tag()
    notes = result["release_notes"] or ["Upstream spec refresh; no user-visible changes."]
    if not args.dry_run:
        with open(SPEC, "w") as f:
            f.write(open(args.upstream).read())
        bump_skill_marker(version, notes)

    old_path = os.path.join(args.out_dir, "embedded-before.json")
    json.dump(old, open(old_path, "w"))
    diff_report = subprocess.run(
        ["python3", ".claude/skills/syllable-sync/scripts/diff_specs.py", old_path, args.upstream],
        capture_output=True, text=True).stdout
    drift = gh_json("issue", "list", "--repo", REPO, "--label", "spec-drift", "--state", "open", "--json", "number")
    closes = "\n".join(f"Closes #{i['number']}" for i in drift)
    if accepted_issue:
        closes += f"\nCloses #{accepted_issue}"
    accepted = (f"\n\n**Accepted in #{accepted_issue}:**\n" + "\n".join(f"- {d}" for d in result["decisions"])) if accepted_issue else ""
    body = f"""Automated spec sync to upstream SDK commit `{args.upstream_sha}` by the spec-sync bot (`.github/workflows/spec-sync-bot.yml`). The classifier rated it **{'accepted decision' if accepted_issue else 'safe to ship'}**, and the full test suite ran on this exact commit before merge.{accepted}

## diff_specs.py report
```
{diff_report.strip()}
```

## Breaking changes
{'See the accepted decision above.' if accepted_issue else ' '.join(['None.'] + ['; '.join(result['ship_reasons'][:8])]).strip()}

## Release notes
{chr(10).join(notes)}

{closes}
"""
    open(os.path.join(args.out_dir, "pr-body.md"), "w").write(body)
    open(os.path.join(args.out_dir, "commit-msg.txt"), "w").write(
        f"chore: sync CLI to OpenAPI spec\n\nAutomated sync to upstream SDK commit {args.upstream_sha} (classifier delta {result['delta']}).\n")
    emit("ship", accepted_issue=accepted_issue)


if __name__ == "__main__":
    main()
