#!/usr/bin/env bash
# From repo root: commit implement output, push branch, open PR if missing.
# Requires: ISSUE_NUMBER set (e.g. export ISSUE_NUMBER=192).
# Requires: gh CLI and GITHUB_TOKEN (or gh auth).
#
# Prefer the TypeScript implementation when running from the assess flow:
#   assess/src/push-and-open-pr.ts (used by run-local-assess with APPLY=1).
# This script is still valid for manual "commit and open PR" from repo root
# when you have already run implement and are on the implementation branch.
#
# Usage from repo root:
#   ISSUE_NUMBER=192 ./.github/actions/issue-auto-implement/scripts/push-and-open-pr.sh
set -e
if [[ -z "$ISSUE_NUMBER" ]]; then
  echo "Set ISSUE_NUMBER (e.g. export ISSUE_NUMBER=192)" >&2
  exit 1
fi
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
BRANCH="auto-implement-issue-${ISSUE_NUMBER}"
COMMIT_MSG_FILE=".github/actions/issue-auto-implement/.commit_msg"
if [[ ! -f "$COMMIT_MSG_FILE" ]]; then
  echo "Missing $COMMIT_MSG_FILE (run implement step first from assess dir)" >&2
  exit 1
fi
# Create or checkout branch
if git show-ref --verify --quiet refs/heads/"$BRANCH"; then
  git checkout "$BRANCH"
  git merge origin/main --no-edit 2>/dev/null || true
elif git show-ref --verify --quiet refs/remotes/origin/"$BRANCH"; then
  git fetch origin "$BRANCH"
  git checkout -b "$BRANCH" origin/"$BRANCH" 2>/dev/null || git checkout "$BRANCH"
  git merge origin/main --no-edit 2>/dev/null || true
else
  git fetch origin main 2>/dev/null || true
  git checkout -b "$BRANCH" origin/main 2>/dev/null || git checkout -b "$BRANCH" main
fi
# Stage all, unstage commit message file, commit, push
git add -A
git reset -- "$COMMIT_MSG_FILE" 2>/dev/null || true
if git diff --staged --quiet; then
  echo "No changes to commit."
else
  git commit -F "$COMMIT_MSG_FILE"
  rm -f "$COMMIT_MSG_FILE"
fi
git push -u origin "$BRANCH"
# Open PR if none exists for this branch
if ! gh pr view --json number 2>/dev/null; then
  gh pr create --fill --body "Closes #${ISSUE_NUMBER}"
  echo "PR created."
else
  echo "PR already exists; branch pushed."
fi
