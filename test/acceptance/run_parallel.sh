#!/usr/bin/env bash
# Run acceptance tests in parallel (same as CI): four matrix slices + telemetry.
# Requires HOOKDECK_CLI_TESTING_API_KEY, HOOKDECK_CLI_TESTING_API_KEY_2, and HOOKDECK_CLI_TESTING_API_KEY_3 in environment or test/acceptance/.env.
# Slice 3 (Outpost) additionally needs HOOKDECK_CLI_OUTPOST_TESTING_API_KEY; without it that slice is
# reported as SKIPPED rather than quietly left out, because it used to be missing here entirely while
# CI ran it — a local "same as CI" pass that silently omitted the whole Outpost surface.
# Run from the repository root.
#
# Matrix slices set HOOKDECK_CLI_TELEMETRY_DISABLED=1. The telemetry slice sets it to 0 (matches CI).
# -tags=telemetry (telemetry_test.go and telemetry_listen_test.go).
#
# Output: each slice writes to a log file so you can see which run produced what.
# Logs are written to test/acceptance/logs/slice0.log, slice1.log, slice2.log, telemetry.log (created on first run).

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
SLICE3_LOG="$LOG_DIR/slice3.log"
TELEMETRY_LOG="$LOG_DIR/telemetry.log"

SLICE0_TAGS="basic guest connection source mcp listen project_use connection_list connection_upsert connection_error_hints connection_oauth_aws connection_update"
SLICE1_TAGS="request event"
SLICE2_TAGS="attempt metrics issue transformation destination gateway"
SLICE3_TAGS="outpost"

run_slice0() {
  ACCEPTANCE_SLICE=0 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="$SLICE0_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE0_LOG" 2>&1
}

run_slice1() {
  ACCEPTANCE_SLICE=1 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="$SLICE1_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE1_LOG" 2>&1
}

run_slice2() {
  ACCEPTANCE_SLICE=2 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="$SLICE2_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE2_LOG" 2>&1
}

run_slice3() {
  ACCEPTANCE_SLICE=3 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="$SLICE3_TAGS" ./test/acceptance/... -v -timeout 12m > "$SLICE3_LOG" 2>&1
}

run_telemetry() {
  (
    export ACCEPTANCE_SLICE=0
    export HOOKDECK_CLI_TELEMETRY_DISABLED=0
    go test -tags=telemetry ./test/acceptance/... -v -timeout 12m
  ) > "$TELEMETRY_LOG" 2>&1
}

echo "Running acceptance tests in parallel (slices 0-3 and telemetry)..."
echo "  Slice 0 -> $SLICE0_LOG"
echo "  Slice 1 -> $SLICE1_LOG"
echo "  Slice 2 -> $SLICE2_LOG"
echo "  Slice 3 -> $SLICE3_LOG"
echo "  Telemetry -> $TELEMETRY_LOG"
run_slice0 &
PID0=$!
run_slice1 &
PID1=$!
run_slice2 &
PID2=$!
run_telemetry &
PIDT=$!

PID3=""
if [ -n "${HOOKDECK_CLI_OUTPOST_TESTING_API_KEY:-}" ]; then
  run_slice3 &
  PID3=$!
fi

FAIL=0
wait $PID0 || FAIL=1
wait $PID1 || FAIL=1
wait $PID2 || FAIL=1
wait $PIDT || FAIL=1
[ -z "$PID3" ] || wait $PID3 || FAIL=1

# Report each slice on its own line. The previous version tailed every log
# whenever anything failed, so a run with 70 failures in one slice and passes
# everywhere else read as an undifferentiated wall of output and the failures
# were easy to miss. Status first, detail only for what actually failed.
status_of() {
  local name="$1" log="$2"
  if [ ! -f "$log" ]; then
    printf "  %-12s SKIPPED (no log)\n" "$name"
    return
  fi
  local failed
  failed=$(grep -c "^--- FAIL" "$log" || true)
  if [ "$failed" -gt 0 ]; then
    printf "  %-12s FAIL (%s failing tests)\n" "$name" "$failed"
  elif grep -q "^ok " "$log"; then
    printf "  %-12s PASS\n" "$name"
  else
    printf "  %-12s FAIL (no ok line — the run did not finish)\n" "$name"
  fi
}

echo ""
echo "Results:"
status_of "slice 0" "$SLICE0_LOG"
status_of "slice 1" "$SLICE1_LOG"
status_of "slice 2" "$SLICE2_LOG"
if [ -n "$PID3" ]; then
  status_of "slice 3" "$SLICE3_LOG"
else
  printf "  %-12s SKIPPED — HOOKDECK_CLI_OUTPOST_TESTING_API_KEY is not set, so the Outpost surface was NOT tested\n" "slice 3"
fi
status_of "telemetry" "$TELEMETRY_LOG"

if [ $FAIL -eq 1 ]; then
  echo ""
  echo "Failing tests by slice:"
  for log in "$SLICE0_LOG" "$SLICE1_LOG" "$SLICE2_LOG" "$SLICE3_LOG" "$TELEMETRY_LOG"; do
    [ -f "$log" ] || continue
    if grep -q "^--- FAIL" "$log"; then
      echo "--- $(basename "$log") ---"
      grep "^--- FAIL" "$log" | head -20
      # Rate limiting looks like a wall of unrelated failures; say so once here
      # rather than leaving it to be rediscovered in the log.
      if grep -q "Too Many Requests" "$log"; then
        echo "    NOTE: this log contains HTTP 429 responses — some or all of these"
        echo "          failures may be rate limiting rather than defects. Re-run this slice alone."
      fi
    fi
  done
fi

echo ""
echo "Logs: $LOG_DIR"
exit $FAIL
