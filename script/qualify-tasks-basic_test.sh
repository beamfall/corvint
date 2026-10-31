#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# Proves the refusals and the fault injection of script/qualify-tasks-basic.sh (TBS-V1-457 to
# TBS-V1-459): a missing OUT_DIR and a non-empty OUT_DIR refuse with exit 2, a supplied suite run
# that lacks a suite test fails before any lifecycle case, a clean run over a supplied binary and
# suite reports LIFECYCLE_PASSED (never QUALIFIED), and a tampered expectation fails exactly the
# tampered case with exit 1 while the initialization cases still pass, so the injection is shown
# reachable rather than assumed. A refusal that exits 0 and an envelope of another profile each fail
# the cutover-missing case.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-qualify-tasks-basic-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
qualify=$source_root/script/qualify-tasks-basic.sh

status=0; "$qualify" >/dev/null 2>&1 || status=$?
[ "$status" -eq 2 ] || fail "missing OUT_DIR exited $status, want 2"

mkdir "$test_root/full"; : >"$test_root/full/keep"
status=0; "$qualify" "$test_root/full" >/dev/null 2>"$test_root/full.err" || status=$?
[ "$status" -eq 2 ] || fail "non-empty OUT_DIR exited $status, want 2"
grep -q 'not an empty directory' "$test_root/full.err" || fail "non-empty OUT_DIR refusal not named"

(cd "$source_root" && GOTOOLCHAIN=local go build -o "$test_root/corvint-tasks" ./cmd/corvint-tasks)
pkg=github.com/Beamfall/corvint/internal/tasks/store
suite_names=$(sed -n '/^var QualificationSuite = \[\]string{/,/^}/s/^[[:space:]]*"\(TestCALV0019_[A-Za-z]*\)",$/\1/p' \
  "$source_root/internal/tasks/transaction/qualification.go")
[ -n "$suite_names" ] || fail "QualificationSuite names not found"
{
  printf '{"Action":"start","Package":"%s"}\n' "$pkg"
  for name in $suite_names; do
    printf '{"Action":"run","Package":"%s","Test":"%s"}\n' "$pkg" "$name"
    printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0}\n' "$pkg" "$name"
  done
  printf '{"Action":"pass","Package":"%s","Elapsed":0}\n' "$pkg"
} >"$test_root/suite.jsonl"

grep -v KilledWriterRecovers "$test_root/suite.jsonl" >"$test_root/short-suite.jsonl"
status=0
"$qualify" "$test_root/short" --tasks-binary "$test_root/corvint-tasks" --suite-run "$test_root/short-suite.jsonl" \
  >"$test_root/short.out" 2>"$test_root/short.err" || status=$?
[ "$status" -eq 1 ] || fail "suite run missing a test exited $status, want 1"
grep -q 'TestCALV0019_KilledWriterRecovers has no pass event' "$test_root/short.err" || fail "missing suite test not named"
[ ! -e "$test_root/short/results.jsonl" ] || [ ! -s "$test_root/short/results.jsonl" ] || fail "lifecycle ran after a suite refusal"

status=0
"$qualify" "$test_root/clean" --tasks-binary "$test_root/corvint-tasks" --suite-run "$test_root/suite.jsonl" \
  >"$test_root/clean.out" 2>&1 || status=$?
[ "$status" -eq 0 ] || { cat "$test_root/clean.out" >&2; fail "clean run exited $status, want 0"; }
last=$(tail -n 1 "$test_root/clean.out")
case $last in
  'LIFECYCLE_PASSED cases='*' binary=SUPPLIED suite=SUPPLIED '*) ;;
  *) fail "clean run over supplied inputs ended '$last'" ;;
esac
jq -e 'select(.pass != true)' "$test_root/clean/results.jsonl" >/dev/null && fail "clean run retained a failed case"

status=0
QUALIFY_TASKS_BASIC_TAMPER=claim-replay \
  "$qualify" "$test_root/tamper" --tasks-binary "$test_root/corvint-tasks" --suite-run "$test_root/suite.jsonl" \
  >"$test_root/tamper.out" 2>&1 || status=$?
[ "$status" -eq 1 ] || fail "tampered run exited $status, want 1"
grep -q '^FAIL case=claim-replay ' "$test_root/tamper.out" || fail "tampered case not reported FAIL"
[ "$(jq -s 'map(select(.pass != true)) | length' "$test_root/tamper/results.jsonl")" -eq 1 ] || fail "tamper failed other cases"
jq -e 'select(.case == "claim-before-cutover" and .pass == true)' "$test_root/tamper/results.jsonl" >/dev/null || fail "cutover-missing reachability case did not pass"
jq -e 'select(.case == "init-nonfixture" and .pass == true)' "$test_root/tamper/results.jsonl" >/dev/null || fail "non-fixture reachability case did not pass"
tail -n 1 "$test_root/tamper.out" | grep -q '^FAILED cases=[0-9]* failed=1$' || fail "tampered run summary not FAILED"

# TBS-V1-452: a refusal that exits 0, or an envelope of another profile, fails its case.
real=$test_root/corvint-tasks
printf '#!/bin/sh\n"%s" "$@"\nexit 0\n' "$real" >"$test_root/exit0-tasks"
# bash, not sh: dash exits 2 on `set -o pipefail` even behind `|| :` (V1-3055).
printf '#!/usr/bin/env bash\nset -o pipefail\n"%s" "$@" | sed "s#taskman-command-result/0#taskman-command-result/1#"\n' "$real" >"$test_root/profile1-tasks"
chmod +x "$test_root/exit0-tasks" "$test_root/profile1-tasks"
for wrap in exit0 profile1; do
  status=0
  "$qualify" "$test_root/$wrap" --tasks-binary "$test_root/$wrap-tasks" --suite-run "$test_root/suite.jsonl" \
    >"$test_root/$wrap.out" 2>&1 || status=$?
  [ "$status" -eq 1 ] || fail "$wrap run exited $status, want 1"
  grep -q '^FAIL case=claim-before-cutover ' "$test_root/$wrap.out" || fail "$wrap run passed the cutover refusal"
done

echo "PASS: qualify-tasks-basic refusals, supplied-input labelling, exit and profile checks, tamper injection"
