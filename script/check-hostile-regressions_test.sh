#!/bin/sh
set -eu

# Drives script/check-hostile-regressions.sh with a stub `go` on PATH: the matrix names only
# test functions the tracked tree defines, `--list` prints every row and the NOT_COVERED
# categories, each row delegates the exact `-run` regex and package under GOTOOLCHAIN=local, a
# test the stub does not report is NOT_RUN and never a pass, and a `--- FAIL` or a nonzero go
# exit fails the run naming the test. The real matrix is `make hostile-regressions-check`.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-hostile-regressions-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
script="$source_root/script/check-hostile-regressions.sh"

# 1. Every listed test function is defined by a tracked _test.go file, and every category the
#    spec's acceptance matrix names appears exactly once as covered or NOT_COVERED.
listing=$("$script" --list) || fail "--list failed"
for name in $(printf '%s\n' "$listing" | sed -n 's/^category=[^ ]* package=[^ ]* tests=//p' | tr ',' '\n'); do
  (cd "$source_root" && git grep -q -E "^func $name\(" -- '*_test.go') || fail "$name is not a tracked test function"
done
for category in hostile-repository paths symlinks case-folds bounded-output time interruption-cleanup secret-screening corrupted-derived-state; do
  printf '%s\n' "$listing" | grep -q "^category=$category package=" || fail "category $category is not listed"
done
for category in memory case-folds-context-index; do
  printf '%s\n' "$listing" | grep -q "^category=$category status=NOT_COVERED reason=." || fail "category $category is not listed as NOT_COVERED"
done
case $listing in
  *"status=PASS"*|*"status=FAIL"*) fail "--list ran something: $listing" ;;
esac

# The stub reports `--- PASS` for each name in the -run regex, except STUB_SKIP (omitted) and
# STUB_FAIL (`--- FAIL`, exit 1); it logs GOTOOLCHAIN, cwd and its arguments per invocation.
mkdir -p "$test_root/bin"
cat > "$test_root/bin/go" <<'STUB'
#!/bin/sh
printf 'GOTOOLCHAIN=%s cwd=%s args=%s\n' "${GOTOOLCHAIN:-unset}" "$(pwd)" "$*" >> "$STUB_LOG"
pattern=""
while test "$#" -gt 0; do
  if test "$1" = -run; then pattern=$2; fi
  shift
done
names=$(printf '%s' "$pattern" | sed 's/^\^(//; s/)\$$//' | tr '|' ' ')
status=0
for name in $names; do
  if test "$name" = "${STUB_SKIP:-}"; then continue; fi
  if test "$name" = "${STUB_FAIL:-}"; then echo "--- FAIL: $name (0.00s)"; status=1; continue; fi
  echo "--- PASS: $name (0.00s)"
done
exit $status
STUB
chmod +x "$test_root/bin/go"

run() {
  status=0
  rm -f "$test_root/go.log"
  output=$(PATH="$test_root/bin:$PATH" STUB_LOG="$test_root/go.log" "$script" 2>&1) || status=$?
  printf '%s' "$output"
  return $status
}

# 2. A fully passing stub run: every row PASS with pass=tests, the exact delegated command line
#    per row, and the NOT_COVERED rows still printed.
output=$(run) || fail "a passing stub run failed: $output"
case $output in
  *"check-hostile-regressions: PASS"*) ;;
  *) fail "no PASS verdict: $output" ;;
esac
printf '%s\n' "$output" | grep '^category=.* package=' | while read -r line; do
  case $line in
    *" status=PASS") ;;
    *) fail "a row did not pass: $line" ;;
  esac
  tests=$(printf '%s' "$line" | sed 's/.* tests=\([0-9]*\) .*/\1/')
  passed=$(printf '%s' "$line" | sed 's/.* pass=\([0-9]*\) .*/\1/')
  test "$tests" = "$passed" || fail "pass count differs from test count: $line"
done
printf '%s\n' "$output" | grep -q '^category=memory status=NOT_COVERED' || fail "NOT_COVERED rows missing from a run"
rows=$("$script" --list | grep -c '^category=.* package=')
test "$(wc -l < "$test_root/go.log" | tr -d ' ')" = "$rows" || fail "go was invoked $(wc -l < "$test_root/go.log") times for $rows rows"
expected="GOTOOLCHAIN=local cwd=$source_root args=test -count=1 -v -run ^(TestInventoryDoesNotClaimAbsenceOverUnsafePathEntries|TestInventoryDoesNotReadUnsafePathBlobs|TestSummaryOrdersUnsafePathSamplesWithoutPanicking)\$ ./internal/genesis"
test "$(head -n 1 "$test_root/go.log")" = "$expected" || fail "first delegated invocation differed: $(head -n 1 "$test_root/go.log")"

# 3. A test the stub never reports is NOT_RUN in its row, not a pass, and the run still passes.
output=$(export STUB_SKIP=TestRunProcessTimeout; run) || fail "a skipped test failed the run: $output"
case $output in
  *"category=time package=./internal/procgroup tests=2 pass=1 not_run=1 status=PASS"*) ;;
  *) fail "a skipped test was not reported NOT_RUN: $output" ;;
esac

# 4. A row whose every test is unreported is NOT_RUN.
output=$(export STUB_SKIP=TestDogfoodPromptBoundsAndCancellation; run) || fail "an unreported row failed the run: $output"
case $output in
  *"category=time package=./internal/contextindex tests=1 pass=0 not_run=1 status=NOT_RUN"*) ;;
  *) fail "an unreported row was not NOT_RUN: $output" ;;
esac

# 5. A failing test fails the run (exit 1), naming the test in its row.
status=0
output=$(export STUB_FAIL=TestSecretPatternParityCorpus; run) || status=$?
test "$status" -eq 1 || fail "a failing test exited $status, not 1"
case $output in
  *"category=secret-screening package=./internal/secretscreen tests=2 pass=1 not_run=0 status=FAIL failed=TestSecretPatternParityCorpus"*"check-hostile-regressions: FAIL"*) ;;
  *) fail "a failing test was not reported: $output" ;;
esac

# 6. An unknown argument is a usage error.
status=0
"$script" --bogus >/dev/null 2>&1 || status=$?
test "$status" -eq 2 || fail "an unknown argument exited $status, not 2"

echo "check-hostile-regressions_test.sh: ok"
