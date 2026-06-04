# Routine: spec-sync reconciler

**Canonical prompt for the reconciliation routine.** Paste this into a scheduled Claude Code routine (claude.ai/code/routines, the desktop app, or `/schedule`) with the `asksyllable/syllable-cli` repo selected and a daily schedule. This file is the source of truth — if you change the routine, change it here and re-paste.

Its job: detect when the embedded OpenAPI spec has drifted from upstream and, if so, open a reconciliation PR for review. **It never merges** — that's the [merger routine](./merger.md)'s job.

---

Check whether the Syllable CLI's embedded OpenAPI spec has drifted from upstream, and if so, open a reconciliation PR for review. Never merge anything yourself.

1. Get onto a clean main:
   - **Cloud (Remote) routine:** the repo is already cloned at the working directory on the default branch — skip to step 2.
   - **Local run:** `cd ~/Documents/Claude/syllable-cli && git fetch origin && git switch main && git pull --ff-only`.
2. Check for drift (either signal is enough):
   - Download the upstream spec from `https://raw.githubusercontent.com/asksyllable/syllable-sdk-typescript/main/openapi.json` and run `python3 .claude/skills/syllable-sync/scripts/diff_specs.py --exit-code scripts/syllable-cli/internal/spec/openapi.json <downloaded.json>` (exit 1 = real structural drift, exit 0 = in sync).
   - Or check for an open tracking issue: `gh issue list --label spec-drift --state open` (note its number — you'll reference it in the PR so merging closes it).
   If there is no drift, stop — there is nothing to do.
3. If there is drift, run the **`syllable-sync`** skill end to end on a new branch: diff → replace the embedded spec → update `cmd/*.go` + `cmd/root.go` + `README.md`/`AGENTS.md` → `cd scripts/syllable-cli && go test ./...`. Commit the spec, code, and docs **together** — the embedded spec must never land ahead of the code that implements it.
4. If `go test` passes, open a pull request. Title it `feat: sync CLI to OpenAPI spec` when it adds commands or flags (so it bumps the version and appears in the release changelog), or `chore: sync CLI to OpenAPI spec` for field-only / spec-only syncs. In the body include:
   - the `diff_specs.py` report;
   - an explicit **Breaking changes** section (removed commands, renamed/removed flags, **removed enum values**);
   - a **Release notes** section — a short, human-readable summary of what changed, written for end users (the merger uses this verbatim for the GitHub release);
   - `Closes #<the spec-drift issue number from step 2>` so merging auto-closes the right issue.

   Apply the `spec-sync` label if you can, so the live sandbox check runs on the PR. **Never merge it** — leave it for review.
5. **Review the diff before handing it off — two independent passes, both posted as a PR comment:**
   - **Self-review:** re-read every new/changed command against the spec — HTTP method + path (mind trailing slashes: a collection ends `/`, an item does not; a mismatch drops the `Syllable-API-Key` header on a 307), required fields, int-vs-string IDs, flag↔schema parity, table columns, and that each new command has an integration test (per the `syllable-sync` skill).
   - **Codex review:** hand the diff to Codex for an independent second opinion (use the codex agent — available locally; no API key needed).
   If either pass finds a real bug, fix it and re-run `go test` before continuing. Post both verdicts (APPROVE, or the issues found) as a comment on the PR so the reviewer sees them.
6. **Classify additive vs breaking conservatively — this is the safety switch for any auto-merge/auto-publish.** Treat as **breaking** anything beyond purely-additive: removed paths/commands/flags, *and also* **changed** fields — a dropped enum value, a newly-required field, a narrowed type, or changed semantics — not only removed ones. Purely-additive = new paths/commands and new *optional* fields, nothing else. Have the **Codex** pass independently confirm this additive-vs-breaking call rather than trusting your own classification. If it's breaking, ambiguous, or `go test` / a review surfaces something you can't safely fix, open the PR as a **draft** and summarize the decision needed instead of guessing.
