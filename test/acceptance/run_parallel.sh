#!/usr/bin/env bash
# Run acceptance tests in three parallel slices (same as CI).
# Requires HOOKDECK_CLI_TESTING_API_KEY, HOOKDECK_CLI_TESTING_API_KEY_2, and HOOKDECK_CLI_TESTING_API_KEY_3 in environment or test/acceptance/.env.
# Run from the repository root.
#
# Output: each slice writes to a log file so you can see which run produced what.
# Logs are written to test/acceptance/logs/slice0.log, slice1.log, slice2.log (created on first run).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT_DIR"

if [ -f "$SCRIPT_DIR/.env" ]; then
  set -a
  # shellcheck source=/dev/null
  source "$SCRIPT_DIR/.env"
  set +a
fi

LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
SLICE0_LOG="$LOG_DIR/slice0.log"
SLICE1_LOG="$LOG_DIR/slice1.log"
SLICE2_LOG="$LOG_DIR/slice2.log"

SLICE0_TAGS="basic connection source destination gateway mcp listen project_use connection_list connection_upsert connection_error_hints connection_oauth_aws connection_update"
SLICE1_TAGS="request event"
SLICE2_TAGS="attempt metrics issue transformation"

run_slice0() {
  ACCEPTANCE_SLICE=0 go test -tags="$SLICE0_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE0_LOG" 2>&1
}

run_slice1() {
  ACCEPTANCE_SLICE=1 go test -tags="$SLICE1_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE1_LOG" 2>&1
}

run_slice2() {
  ACCEPTANCE_SLICE=2 go test -tags="$SLICE2_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE2_LOG" 2>&1
}

echo "Running acceptance tests in parallel (slice 0, 1, and 2)..."
echo "  Slice 0 -> $SLICE0_LOG"
echo "  Slice 1 -> $SLICE1_LOG"
echo "  Slice 2 -> $SLICE2_LOG"
run_slice0 &
PID0=$!
run_slice1 &
PID1=$!
run_slice2 &
PID2=$!

FAIL=0
wait $PID0 || FAIL=1
wait $PID1 || FAIL=1
wait $PID2 || FAIL=1

if [ $FAIL -eq 1 ]; then
  echo ""
  echo "One or more slices failed. Tail of failed log(s):"
  [ ! -f "$SLICE0_LOG" ] || (echo "--- slice 0 ---" && tail -50 "$SLICE0_LOG")
  [ ! -f "$SLICE1_LOG" ] || (echo "--- slice 1 ---" && tail -50 "$SLICE1_LOG")
  [ ! -f "$SLICE2_LOG" ] || (echo "--- slice 2 ---" && tail -50 "$SLICE2_LOG")
fi

echo ""
echo "Logs: $SLICE0_LOG  $SLICE1_LOG  $SLICE2_LOG"
exit $FAIL
