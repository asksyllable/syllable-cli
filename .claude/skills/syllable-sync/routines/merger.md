# Routine: spec-sync merger (independent verifier + merger + publisher)

**Canonical prompt for the merging routine.** Paste the prompt below into a *separate* scheduled Claude Code routine from the [reconciler](./reconciler.md), with the `asksyllable/syllable-cli` repo selected, scheduled to run shortly **after** the reconciler (e.g. reconciler at 6am, merger at 7am — enough gap for the PR's CI to finish). Keeping it separate from the reconciler is the point: an independent session that **re-derives the truth from source** instead of trusting the PR's own review prose.

> Why separate + independent: in the cycle that built this, the reconciler's self-review *and* a Codex pass both confidently described a change, and a reviewer reading that same narrative would have inherited the same blind spot. What actually decides correctness is a fresh, mechanical re-derivation from the specs and the tests — not anyone's review text (including a previous agent's, and including your own first impression).

This routine runs the **full downstream cycle** — verify → merge → version bump → publish → Homebrew — with no human in the loop for the common case. The one safety valve: it acts automatically **only when the change is purely additive and every gate is green**; anything breaking, ambiguous, or not-green it routes to a human instead. That valve isn't a brake on automation — it's what makes unattended publishing safe.

**Before you set it up — two operating requirements:**

1. **Run it where its tag-push triggers the release.** The release step only fires if the `vX.Y.Z` tag is pushed with credentials GitHub lets trigger workflows. A tag pushed by the default GitHub Actions token is deliberately suppressed (loop-prevention), so the build and Homebrew update would **silently never happen**. Run this routine locally with your own git credentials, or give it a personal access token. (The reconciler has no such requirement — it only opens a PR.)
2. **Watch the first live cycle before trusting it unattended.** This routine merges and publishes to users — the one action you cannot walk back. Observe its first clean end-to-end cycle (or two) before you stop checking. After that it's genuinely hands-off.

---

You are the independent verifier-and-merger for spec-sync reconciliation PRs in `asksyllable/syllable-cli`. You run after the reconciler routine, which opens these PRs. Your job: independently verify each PR, and if it is purely additive and fully green, **merge it, cut the version release, and ship the Homebrew update** — the complete cycle, no human in the loop. If anything is breaking, ambiguous, or not green, **STOP and leave it for a human**. When in doubt, do not merge — you are the last gate before users.

Run `git fetch origin`. For each open PR labelled `spec-sync` (`gh pr list --label spec-sync --state open`). If there are none, stop — there's nothing to do.

1. **Re-derive the diff yourself — do NOT read the PR's own review comments first.** The point of this routine is an independent check; reading the PR's narrative first would bias you toward its conclusions. Get the PR's spec (`gh pr checkout <n>`, then `scripts/syllable-cli/internal/spec/openapi.json`) and the base (`git show origin/main:scripts/syllable-cli/internal/spec/openapi.json`). Run the enum-aware diff between them:
   `python3 .claude/skills/syllable-sync/scripts/diff_specs.py --exit-code --format json <base> <pr-spec>`
   Decide for yourself what changed: paths, schemas, fields, and enum members (a removed enum value is breaking).

2. **Classify additive vs breaking from YOUR diff, conservatively.**
   - **Breaking** = any removed path/command/flag, any removed enum value, any newly-required field, a narrowed type, or changed semantics.
   - **Purely additive** = new paths/commands + new *optional* fields + new enum values, and nothing else.
   - If breaking or ambiguous → **do not merge.** Comment exactly what you found, leave it for a human, and move on to the next PR.

3. **Verify the gates are independently green** (`gh pr checks <n>`): `test` (unit) **and** `spec-live-check` (live `cli-test` org) must both be present and **passing**. If either is missing, pending, or red → do not merge; comment and leave for a human. Confirm `spec-live-check` actually exercised the **new** commands (Stage 1 coverage), not just pre-existing ones — if a new command has no integration test, treat the QA as incomplete and leave it for a human.

4. **Spot-check the code yourself** (don't rely on the author's notes): new command paths match the spec including trailing slashes (a collection path ends in `/`, an item path does not — a mismatch drops the `Syllable-API-Key` header on a 307), required fields are covered by flags, IDs handled correctly (int vs string), and each new command has a test.

5. **Merge — only if purely additive AND all gates green AND your spot-check is clean:**
   `gh pr merge <n> --squash --delete-branch`
   Confirm the linked `spec-drift` issue closed (if it didn't, close it, and note the PR was missing its `Closes #N`).

6. **Release and publish:**
   - Compute the bump from *your* diff: new paths/commands → **minor**; additive fields / enum values only → **patch**.
   - Tag `vX.Y.Z` and push it **with credentials that trigger `release.yml`** — a tag pushed by the default GitHub Actions token will NOT trigger the release workflow. GoReleaser then builds, publishes the GitHub release, and opens the Homebrew cask PR.
   - **Set the GitHub release notes from the PR's `Release notes` section** (and your diff). GoReleaser's commit-based changelog omits `chore:`/field-only changes, so write the notes explicitly — never ship an empty `## Changelog`.
   - Merge the `chore: update Homebrew cask …` PR so `brew` users get it.

7. **If anything is off** — a breaking or ambiguous change, a red/missing/pending gate, your diff disagrees with the PR, or a new command lacks a test — **comment with specifics and leave the PR for a human. Never merge or release on doubt.**
