#!/usr/bin/env bash
# Create or update .env for local runs. Run from action root: .github/actions/issue-auto-implement/
#
# Usage:
#   ./scripts/setup-local-env.sh           # Create .env from .env.example if missing; optionally fill GITHUB_TOKEN from gh
#   ./scripts/setup-local-env.sh --with-gh # Same, and run 'gh auth token' to set GITHUB_TOKEN (prompts if not authenticated)
#   ./scripts/setup-local-env.sh --template-only # Only create .env from .env.example, do not run gh

set -e
ACTION_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ACTION_ROOT"
ENV_FILE="$ACTION_ROOT/.env"
EXAMPLE_FILE="$ACTION_ROOT/.env.example"

if [[ "$1" == "--template-only" ]]; then
  WITH_GH=false
else
  WITH_GH=false
  [[ "$1" == "--with-gh" ]] && WITH_GH=true
fi

if [[ ! -f "$EXAMPLE_FILE" ]]; then
  echo "Missing .env.example" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  cp "$EXAMPLE_FILE" "$ENV_FILE"
  echo "Created .env from .env.example"
else
  echo ".env already exists; leaving it as-is"
fi

if [[ "$WITH_GH" == true ]]; then
  if command -v gh &>/dev/null; then
    TOKEN=$(gh auth token 2>/dev/null) || true
    if [[ -n "$TOKEN" ]]; then
      if grep -q '^GITHUB_TOKEN=' "$ENV_FILE" 2>/dev/null; then
        sed -i.bak "s|^GITHUB_TOKEN=.*|GITHUB_TOKEN=$TOKEN|" "$ENV_FILE" && rm -f "$ENV_FILE.bak"
      else
        echo "GITHUB_TOKEN=$TOKEN" >> "$ENV_FILE"
      fi
      echo "Set GITHUB_TOKEN from gh auth token"
    else
      echo "gh auth token returned empty; run 'gh auth login' if needed. GITHUB_TOKEN not updated in .env"
    fi
  else
    echo "gh not found; install GitHub CLI to fill GITHUB_TOKEN automatically"
  fi
fi

echo "Edit .env to set AUTO_IMPLEMENT_ANTHROPIC_API_KEY (and ISSUE_NUMBER, GITHUB_REPOSITORY when running implement)."
