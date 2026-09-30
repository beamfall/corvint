#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Negative/lifecycle tests. Mock ACKs never qualify an independent review.
set -u
set -m
source_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd -P) || exit 1
tour=$source_root/script/proof-tour.sh
test_root=$(mktemp -d /private/tmp/corvint-proof-tour-tests.XXXXXX) || exit 1
active_pid=
cleanup() {
  if [ -n "$active_pid" ]; then
    kill -TERM "$active_pid" 2>/dev/null || :
    sleep 2
    kill -KILL -- "-$active_pid" 2>/dev/null || :
    wait "$active_pid" 2>/dev/null || :
  fi
}
trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM
fail() { printf 'FAIL %s artifacts=%s\n' "$1" "$test_root" >&2; exit 1; }
pass() { printf 'PASS %s\n' "$1"; }
run_expected() {
  local want=$1 name=$2 status=0
  shift 2
  "$@" > "$test_root/$name.out" 2> "$test_root/$name.stderr" || status=$?
  [ "$status" -eq "$want" ] || fail "$name exit=$status expected=$want"
  pass "$name"
}
retired() {
  local pid=$1 state count=0
  while [ "$count" -lt 40 ]; do
    state=$(ps -p "$pid" -o stat= 2>/dev/null || :)
    case "$state" in ''|Z*|*Z*) return 0;; esac
    sleep 0.1; count=$((count + 1))
  done
  return 1
}
wait_marker() {
  local file=$1 count=0
  while [ ! -s "$file" ]; do
    kill -0 "$active_pid" 2>/dev/null || fail process-exited-before-marker
    [ "$count" -lt 100 ] || fail marker-timeout
    sleep 0.1; count=$((count + 1))
  done
}

verifier=${PROOF_TOUR_TEST_VERIFIER:-/private/tmp/corvint-v1-wow-cem01-verifier}
verifier_sha=${PROOF_TOUR_TEST_VERIFIER_SHA256:-e2f8ec833317731b430e309fdc43f06276d7db4d140abd8d2e7c08a2243e882d}
[ -f "$verifier" ] && [ -x "$verifier" ] || fail missing-test-verifier
bash -n "$tour" || fail syntax
run_expected 0 help "$tour" --help
run_expected 2 trailing-output "$tour" --out "$test_root/trailing/" --verifier "$verifier" --verifier-sha256 "$verifier_sha"
mkdir "$test_root/populated"; : > "$test_root/populated/keep"
run_expected 2 populated "$tour" --out "$test_root/populated" --verifier "$verifier" --verifier-sha256 "$verifier_sha"
ln -s "$test_root/populated" "$test_root/symlink"
run_expected 2 symlink "$tour" --out "$test_root/symlink" --verifier "$verifier" --verifier-sha256 "$verifier_sha"
run_expected 2 timeout-value "$tour" --out "$test_root/invalid-timeout" --verifier "$verifier" --verifier-sha256 "$verifier_sha" --timeout 0
run_expected 2 digest-mismatch "$tour" --out "$test_root/bad-digest" --verifier "$verifier" --verifier-sha256 "$(printf '%064d' 0)"
[ ! -e "$test_root/bad-digest/receipts/core-version.out" ] || fail executed-after-digest-mismatch

# Every mock command is isolated to this developer test's PATH and labelled.
# The TERM-ignoring child stays in the ordinary inherited process group.
mkdir "$test_root/mock-bin"
cat > "$test_root/mock-bin/corvint" <<'MOCK'
#!/usr/bin/env bash
set -u
printf 'MOCK_PROCESS_LIFECYCLE_ONLY\n'
trap '' TERM
bash -c 'trap "" TERM; printf "%s\n" "$$" > "$PROOF_TOUR_MOCK_MARKER"; while :; do sleep 1; done' &
if [ "${PROOF_TOUR_MOCK_LEADER_EXIT:-no}" = yes ]; then
  while [ ! -s "$PROOF_TOUR_MOCK_MARKER" ]; do sleep 0.1; done
  exit 9
fi
wait
MOCK
chmod +x "$test_root/mock-bin/corvint"
mock_path=$test_root/mock-bin:$PATH
run_expected 2 process-timeout env PATH="$mock_path" PROOF_TOUR_MOCK_MARKER="$test_root/timeout-child.pid" "$tour" --out "$test_root/process-timeout" --verifier "$verifier" --verifier-sha256 "$verifier_sha" --timeout 1
retired "$(cat "$test_root/timeout-child.pid")" || fail timeout-grandchild-survived
pass timeout-descendants-retired
run_expected 3 leader-exit env PATH="$mock_path" PROOF_TOUR_MOCK_MARKER="$test_root/leader-child.pid" PROOF_TOUR_MOCK_LEADER_EXIT=yes "$tour" --out "$test_root/leader-exit" --verifier "$verifier" --verifier-sha256 "$verifier_sha" --timeout 5
retired "$(cat "$test_root/leader-child.pid")" || fail exited-leader-grandchild-survived
pass exited-leader-descendants-retired
for signal in TERM INT; do
  env PATH="$mock_path" PROOF_TOUR_MOCK_MARKER="$test_root/$signal-child.pid" "$tour" --out "$test_root/process-$signal" --verifier "$verifier" --verifier-sha256 "$verifier_sha" --timeout 30 > "$test_root/$signal.out" 2> "$test_root/$signal.stderr" &
  active_pid=$!
  wait_marker "$test_root/$signal-child.pid"
  kill -"$signal" "$active_pid" || fail signal-delivery
  status=0; wait "$active_pid" 2>/dev/null || status=$?
  active_pid=
  if [ "$signal" = TERM ]; then [ "$status" -eq 143 ] || fail TERM-exit
  else [ "$status" -eq 130 ] || fail INT-exit; fi
  retired "$(cat "$test_root/$signal-child.pid")" || fail "$signal-grandchild-survived"
  [ -f "$test_root/process-$signal/receipts/core-version.out" ] || fail interrupted-receipt-lost
  pass "$signal-descendants-retired-receipts-preserved"
done

# Review negative cases use a private COPY of the real synthetic fixture.
# No accepted ACK is fabricated, and the original independently reviewed tour
# package is never mutated by these tests.
original=${PROOF_TOUR_TEST_REVIEW_OUT:-}
if [ -n "$original" ]; then
  copy=$test_root/review-copy
  mkdir -p "$copy/receipts" "$copy/tools"
  cp -R "$original/fixture" "$copy/fixture" || fail fixture-copy
  cp "$original/review-request.txt" "$copy/review-request.txt" || fail request-copy
  cp "$original/change.patch" "$copy/change.patch" || fail patch-copy
  cp "$original/tools/cem01-go" "$copy/tools/cem01-go" || fail verifier-copy
  run_expected 3 missing-ack "$tour" --resume "$copy"
  { printf 'proof-tour-ack/0\n'; sed -n '2,5p' "$copy/review-request.txt"; printf 'verdict=REJECT\nreviewer=MOCK_TEST_ONLY\n'; } > "$test_root/rejected-ack.txt"
  run_expected 3 rejected-ack "$tour" --resume "$copy" --ack "$test_root/rejected-ack.txt"
  sed 's/^head=.*/head=0000000000000000000000000000000000000000/;s/^verdict=.*/verdict=ACCEPT/' "$test_root/rejected-ack.txt" > "$test_root/stale-ack.txt"
  run_expected 3 stale-ack "$tour" --resume "$copy" --ack "$test_root/stale-ack.txt"
  # ACCEPT text here is deliberately malformed and must never pass admission.
  sed 's/^verdict=REJECT/verdict=ACCEPT/' "$test_root/rejected-ack.txt" > "$test_root/junk-ack.txt"
  printf 'unterminated-junk' >> "$test_root/junk-ack.txt"
  run_expected 3 noncanonical-ack "$tour" --resume "$copy" --ack "$test_root/junk-ack.txt"
  grep -F 'reason=noncanonical-review-ack' "$test_root/noncanonical-ack.out" >/dev/null || fail accepted-trailing-ack-junk
  printf 'dirty\n' >> "$copy/fixture/auth/auth.go"
  run_expected 3 changed-fixture "$tour" --resume "$copy" --ack "$test_root/stale-ack.txt"
  grep -F 'reason=dirty-fixture' "$test_root/changed-fixture.out" >/dev/null || fail regenerated-dirty-fixture
  # Corrupt status must not turn unreadable index state into a clean worktree.
  bad=$test_root/corrupt-index
  mkdir -p "$bad/receipts" "$bad/tools"
  cp -R "$original/fixture" "$bad/fixture" || fail corrupt-fixture-copy
  cp "$original/review-request.txt" "$bad/review-request.txt"
  cp "$original/change.patch" "$bad/change.patch"
  cp "$original/tools/cem01-go" "$bad/tools/cem01-go"
  printf '\n// UNREVIEWED test-only addition.\n' >> "$bad/fixture/auth/auth.go"
  printf 'corrupt index\n' > "$bad/fixture/.git/index"
  run_expected 3 corrupt-index "$tour" --resume "$bad" --ack "$test_root/rejected-ack.txt"
  grep -F 'reason=git-status-failed' "$test_root/corrupt-index.out" >/dev/null || fail status-failure-accepted-as-clean
  [ ! -e "$bad/receipts/resume-1-patch.out" ] || fail wrote-after-status-failure
  # Every planned output is admitted before the first receipt is created.
  for collision_name in ack.txt tool-identity.txt ready.stderr; do
    collision=$test_root/collision-$collision_name
    mkdir -p "$collision/receipts" "$collision/tools"
    cp -R "$original/fixture" "$collision/fixture" || fail collision-fixture-copy
    cp "$original/review-request.txt" "$collision/review-request.txt"
    cp "$original/change.patch" "$collision/change.patch"
    cp "$original/tools/cem01-go" "$collision/tools/cem01-go"
    printf 'PRESERVE_SENTINEL_%s\n' "$collision_name" > "$collision/sentinel.txt"
    cp "$collision/sentinel.txt" "$collision/sentinel.original"
    ln -s "$collision/sentinel.txt" "$collision/receipts/resume-1-$collision_name"
    run_expected 2 "collision-$collision_name" "$tour" --resume "$collision" --ack "$test_root/rejected-ack.txt"
    grep -F 'reason=resume-output-collision' "$test_root/collision-$collision_name.out" >/dev/null || fail collision-not-preflighted
    cmp -s "$collision/sentinel.original" "$collision/sentinel.txt" || fail sentinel-overwritten
    [ ! -e "$collision/receipts/resume-1-patch.out" ] || fail partial-write-before-collision-refusal
  done
  pass all-resume-output-collisions-preflighted
  [ "$(cat "$original/receipts/missing-witness.exit")" -eq 1 ] || fail original-refusal-lost
  pass original-refusal-retained
else
  printf 'NOT_RUN review-negatives require PROOF_TOUR_TEST_REVIEW_OUT from a paused real tour\n'
fi
printf 'PASS proof-tour-focused-tests artifacts=%s independent_review=NOT_PRODUCED_BY_TESTS hostile_group_escape=NOT_TESTED\n' "$test_root"
