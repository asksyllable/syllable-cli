# Routine: spec-sync merger (independent reviewer + merger)

**Canonical prompt for the merging routine.** Paste this into a *separate* scheduled Claude Code routine from the [reconciler](./reconciler.md), with the `asksyllable/syllable-cli` repo selected. Run it daily (or on demand). Keeping it separate from the reconciler is the point: an independent session that **re-derives the truth from source** instead of trusting the PR's own review prose.

> Why separate + independent: in the cycle that built this, the reconciler's self-review *and* a Codex pass both confidently described a change, and a reviewer reading that same narrative would have inherited the same blind spot. What actually decides correctness is a fresh, mechanical re-derivation from the specs and the tests — not anyone's review text (including a previous agent's, and including your own first impression).

Your job: for each open reconciliation PR, independently verify it and **merge only if it is green and purely additive**. When in doubt, do not merge — you are the last gate before users.

For each open PR labelled `spec-sync` (`gh pr list --label spec-sync --state open`):

1. **Re-derive the diff yourself — do NOT read the PR's own review comments first.** Fetch the PR head. Run the enum-aware diff between `main` (or the PR's merge base) and the PR's embedded spec:
   `python3 .claude/skills/syllable-sync/scripts/diff_specs.py --exit-code --format json <base-spec> <pr-spec>`
   Decide independently what changed: paths, schemas, fields, and **enum members** (the diff now reports enum add/remove; a removed value is breaking).

2. **Classify additive vs breaking from YOUR diff, conservatively.**
   - **Breaking** = any removed path / command / flag, any **removed enum value**, any newly-required field, a narrowed type, or changed semantics.
   - **Purely additive** = new paths/commands + new *optional* fields + new enum values, and nothing else.
   - If breaking or ambiguous → **do not merge.** Comment with exactly what you found and leave it for a human.

3. **Verify the gates are independently green** (`gh pr checks <n>`):
   - `test` (unit) **and** `spec-live-check` (live `cli-test`) must both be **passing**. If either is missing or red → do not merge.
   - Confirm `spec-live-check` actually exercised the new commands (Stage 1 coverage), not just pre-existing ones — if a new command has no integration test, treat the QA as incomplete and leave it for a human.

4. **Spot-check the code yourself** (don't rely on the author's notes): new command paths match the spec including trailing slashes, required fields are covered by flags, IDs handled (int vs string), and each new command has a test.

5. **Merge only if** purely additive **and** all gates green **and** your spot-check is clean:
   `gh pr merge <n> --squash --delete-branch`. Confirm the linked `spec-drift` issue closed (if it didn't, close it, and note the PR was missing its `Closes #N`).

6. **Release** (only once you're cleared to publish — see "Rollout" below):
   - Compute the bump from *your* diff: new paths/commands → **minor**; additive fields / enum values → **patch**; anything breaking → never (it went to a human).
   - Tag `vX.Y.Z` and push it **with your own credentials** (a tag pushed by a bot/`GITHUB_TOKEN` will NOT trigger `release.yml`). GoReleaser builds, publishes the release, and opens the Homebrew cask PR.
   - **Set the GitHub release notes from the PR's `Release notes` section** (and your diff). GoReleaser's commit-based changelog omits `chore:`/field-only changes, so write the notes explicitly: list the additive changes in plain language. Do not ship an empty `## Changelog`.
   - Merge the `chore: update Homebrew cask …` PR so `brew` users get it.

7. **If anything is off** — a red or missing gate, a breaking change, your diff disagrees with the PR, or a new command lacks a test — **comment with specifics and leave the PR for a human. Never merge on doubt.**

## Rollout (do not skip)

- **Phase A — review only:** do steps 1–4 and post your independent verdict as a comment. Do **not** merge. Let a human merge while you build confidence that your verdicts match reality.
- **Phase B — auto-merge:** enable step 5 (merge purely-additive, green PRs).
- **Phase C — auto-publish:** enable step 6.

Advance one phase at a time, only after several clean observed cycles. Auto-publishing to users is the one action you cannot walk back.
