#!/usr/bin/env bash
# Verify and merge GoReleaser's Homebrew cask PR for a release tag.
#
# GoReleaser opens the cask PR with GITHUB_TOKEN, and GitHub never runs
# workflows for PRs opened that way, so the PR would wait forever on the
# required `test` and `cask-verify` checks. This script runs the cask
# verification itself, then records both results as commit statuses on the
# PR's head commit:
#   - cask-verify: verify_cask.py --cask-only passed (urls and shas match the
#     release checksums, nothing else in the cask or repo changed);
#   - test: the change touches only Casks/syllable.rb, which no test builds;
#     the release's code was tested by GoReleaser's before-hook and on main.
# It then merges the PR. Safe to re-run: it exits 0 if the PR is already merged.
#
# Usage: merge_cask_pr.sh vX.Y.Z   (needs GH_TOKEN with contents, pull-requests,
# and statuses write; run from a full-history checkout of this repo)
set -euo pipefail

tag="${1:?usage: merge_cask_pr.sh vX.Y.Z}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::not a release tag: $tag"
  exit 1
fi
repo="${GITHUB_REPOSITORY:-asksyllable/syllable-cli}"
branch="goreleaser/syllable-${tag}"
here="$(cd "$(dirname "$0")" && pwd)"

pr=""
for _ in $(seq 1 12); do
  pr="$(gh pr list --repo "$repo" --head "$branch" --state all --json number,state \
        --jq '.[0] | select(.) | "\(.number) \(.state)"')"
  [ -n "$pr" ] && break
  sleep 10
done
if [ -z "$pr" ]; then
  echo "::error::No cask PR found for branch $branch"
  exit 1
fi
read -r number state <<<"$pr"
if [ "$state" = "MERGED" ]; then
  echo "Cask PR #$number is already merged."
  exit 0
fi

for attempt in 1 2 3; do
  git fetch --quiet origin main "$branch"
  python3 "$here/verify_cask.py" --base origin/main --head "origin/$branch" --cask-only

  sha="$(git rev-parse "origin/$branch")"
  gh api --silent "repos/$repo/statuses/$sha" -f state=success -f context=cask-verify \
    -f description="url/sha256 pairs match the ${tag} release checksums; cask structure unchanged"
  gh api --silent "repos/$repo/statuses/$sha" -f state=success -f context=test \
    -f description="Cask-only change; ${tag}'s code was tested in the release build and on main"

  if gh pr merge "$number" --repo "$repo" --squash --delete-branch; then
    echo "Merged cask PR #$number for $tag."
    exit 0
  fi
  echo "Merge attempt $attempt failed; updating the branch from main and re-verifying."
  gh pr update-branch "$number" --repo "$repo" || true
  sleep 10
done

echo "::error::Could not merge cask PR #$number for $tag after 3 attempts"
exit 1
