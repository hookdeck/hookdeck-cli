#!/usr/bin/env bash
# Set up an isolated CLI config for one acceptance-test project, and refuse to
# hand it back unless the credential really resolves to that project.
#
#   source hd-testenv.sh <ENV_VAR_NAME> <expected tm_ id>
#
# On success exports HD_CONFIG. Use it on EVERY command:
#   go run . --hookdeck-config "$HD_CONFIG" gateway source list
#
# Two things this guarantees:
#
#   1. The default config (~/.config/hookdeck/config.toml) is never read or
#      written, so a QA run cannot clobber the operator's own login. This has
#      happened: a suite run without an isolated config wrote CI credentials
#      over a working one.
#   2. A destructive command cannot run against a project that was not named up
#      front. The project is confirmed from the credential itself, not from the
#      argument, so a mistyped or rotated key fails closed rather than pointing
#      at something unexpected.
#
# Safety here comes from where the credential lives, not from a hardcoded list
# of project IDs. Keys must come from test/acceptance/.env, which is gitignored
# and holds CI test keys, so the reachable blast radius is the test projects
# those keys can see. A list of IDs in a public repo would rot and would leak
# identifiers for no gain.
# Deliberately no `set -u` / `set -e` at this level: this file is sourced, so
# those would apply to the caller's interactive shell and break it. The
# functions below use explicit defaults instead.

# Organisation every acceptance-test project belongs to. Override if your test
# projects live elsewhere.
ALLOWED_ORG="${HOOKDECK_QA_ALLOWED_ORG:-Automated Testing}"

# Resolve the repo from the working directory rather than from the script's own
# path: BASH_SOURCE does not exist under zsh, which is where this is most often
# sourced from, and a wrong answer here silently looks for .env in the wrong place.
_hd_repo() {
  if [ -n "${HOOKDECK_QA_REPO:-}" ]; then
    printf '%s\n' "$HOOKDECK_QA_REPO"
    return 0
  fi
  git rev-parse --show-toplevel 2>/dev/null && return 0
  printf '%s\n' "$PWD"
}

hd_testenv() {
  local var="$1" expected="$2"
  local repo scratch
  repo="$(_hd_repo)"
  scratch="${HOOKDECK_QA_SCRATCH:-${TMPDIR:-/tmp}/hookdeck-qa}"
  mkdir -p "$scratch"

  case "$expected" in
    tm_*) ;;
    *) echo "REFUSING: $expected does not look like a project id" >&2; return 1 ;;
  esac

  local envfile="$repo/test/acceptance/.env"
  [ -f "$envfile" ] || { echo "REFUSING: $envfile not found — QA keys must come from there" >&2; return 1; }
  set -a; . "$envfile"; set +a

  local key
  eval "key=\${$var:-}"
  [ -n "$key" ] || { echo "REFUSING: $var is not set in test/acceptance/.env" >&2; return 1; }

  # Authenticate into a scratch path and move it into place only on success.
  # Wiping the target first meant a failed attempt destroyed a config that had
  # been working, and the next run then failed for an unrelated reason.
  local cfg="$scratch/hdcfg-$expected.toml"
  # The extension has to stay .toml: the config format is inferred from it, and
  # a temp name ending in the PID fails with Unsupported Config Type.
  local tmp="$scratch/hdcfg-$expected.new-$$.toml"
  rm -f "$tmp"
  (cd "$repo" && go run . --hookdeck-config "$tmp" ci --api-key "$key" >/dev/null 2>&1) \
    || { rm -f "$tmp"; echo "REFUSING: could not authenticate with $var (existing config left alone)" >&2; return 1; }
  mv -f "$tmp" "$cfg"

  # Verify against the credential, not against what we hoped it was.
  local who actual
  who=$(cd "$repo" && go run . --hookdeck-config "$cfg" whoami 2>&1)
  actual=$(grep -oE 'tm_[A-Za-z0-9]+' "$cfg" | head -1)

  if [ "$actual" != "$expected" ]; then
    echo "REFUSING: config resolved to $actual, expected $expected" >&2
    rm -f "$cfg"; return 1
  fi
  if ! grep -qF "$ALLOWED_ORG" <<<"$who"; then
    echo "REFUSING: not the $ALLOWED_ORG organization:" >&2
    echo "$who" >&2
    rm -f "$cfg"; return 1
  fi

  export HD_CONFIG="$cfg"
  export HD_PROJECT="$expected"
  echo "OK: $expected — $(grep -oE 'on project .*' <<<"$who")"
}

# Re-assert before anything destructive. Cheap, and it catches a config that was
# swapped or a project switched mid-run. Call it immediately before any delete,
# and let a non-zero return abort the step.
hd_assert() {
  local expected="$1" actual
  [ -n "${HD_CONFIG:-}" ] || { echo "ABORT: HD_CONFIG is not set — run hd_testenv first" >&2; return 1; }
  actual=$(grep -oE 'tm_[A-Za-z0-9]+' "$HD_CONFIG" | head -1)
  [ "$actual" = "$expected" ] || { echo "ABORT: $HD_CONFIG points at $actual, not $expected" >&2; return 1; }
}
