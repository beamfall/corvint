#!/usr/bin/env bash
# Tests for the lane procedure scripts (docs/LANE-ORCHESTRATION.md): heavy.sh (ASD-V0-014),
# the dogfood-finish missing-input refusal (ASD-V0-015), lane-idle-check.sh (ASD-V0-017) and
# lane-clean.sh (ASD-V0-018). No Go build runs.
set -uo pipefail
export LC_ALL=C

script=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-lane-scripts-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
passed=0
fail() { printf 'lane-scripts_test: FAIL %s\n' "$*" >&2; exit 1; }
ok() { passed=$((passed + 1)); }

# ASD-V0-014: two concurrent commands with HEAVY_SLOTS=1 never overlap.
export HEAVY_LOCK_DIR=$tmp/slots
probe='mkdir "$1/running" || exit 9; sleep 1; rmdir "$1/running"'
HEAVY_SLOTS=1 "$script/heavy.sh" sh -c "$probe" _ "$tmp" 2> "$tmp/a.err" &
a=$!
HEAVY_SLOTS=1 "$script/heavy.sh" sh -c "$probe" _ "$tmp" 2> "$tmp/b.err" &
b=$!
wait "$a" || fail "heavy.sh first concurrent command overlapped (exit $?)"
wait "$b" || fail "heavy.sh second concurrent command overlapped (exit $?)"
grep -q '^heavy.sh: slot 1/1 after [1-9]' "$tmp/a.err" "$tmp/b.err" || fail "heavy.sh: no command waited for the slot"
ok

# ASD-V0-014: the command's exit status and -p bound pass through; an explicit -p is kept.
HEAVY_SLOTS=1 "$script/heavy.sh" sh -c 'exit 7' 2> /dev/null
[[ $? == 7 ]] || fail "heavy.sh did not return the command's status"
got=$(HEAVY_SLOTS=2 HEAVY_P=3 GOFLAGS= "$script/heavy.sh" sh -c 'printf %s "$GOFLAGS"' 2> /dev/null)
[[ $got == -p=3 ]] || fail "heavy.sh GOFLAGS=$got, want -p=3"
got=$(HEAVY_SLOTS=2 HEAVY_P=3 GOFLAGS='-p=1 -mod=mod' "$script/heavy.sh" sh -c 'printf %s "$GOFLAGS"' 2> /dev/null)
[[ $got == '-p=1 -mod=mod' ]] || fail "heavy.sh overrode an explicit GOFLAGS -p: $got"
HEAVY_SLOTS=0 "$script/heavy.sh" true 2> "$tmp/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-slots' "$tmp/err" || fail "heavy.sh accepted HEAVY_SLOTS=0"
ok

# ASD-V0-014: a killed owner does not strand its slot.
HEAVY_SLOTS=1 "$script/heavy.sh" sleep 5 2> /dev/null &
holder=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do [[ -s $HEAVY_LOCK_DIR/slot1 ]] && break; sleep 0.2; done
kill -9 "$holder" 2> /dev/null
wait "$holder" 2> /dev/null
start=$(date +%s)
HEAVY_SLOTS=1 "$script/heavy.sh" true 2> /dev/null || fail "heavy.sh failed after a killed owner"
(($(date +%s) - start < 3)) || fail "heavy.sh waited on a dead owner's slot"
ok
unset HEAVY_LOCK_DIR

# ASD-V0-015: every missing input is named at once, before any build.
finish() { (cd "$tmp" && env -u CORVINT_BIN DOGFOOD_FINISH_LANE="$1" "$script/dogfood-finish.sh" "${@:2}" 2>&1); }
out=$(finish '' '' '')
[[ $out == *'REFUSE missing-inputs BASE CITATIONS'$'\n''fix: '* ]] || fail "finish refusal: $out"
out=$(finish 1 '' '' '' '' '')
[[ $out == *'REFUSE missing-inputs BASE CITATIONS INTENTS VERIFY OUTCOME'* ]] || fail "lane finish refusal: $out"
: > "$tmp/plan.tsv"
out=$(finish 1 0123 plan.tsv absent.txt absent.txt bogus)
[[ $out == *'REFUSE missing-inputs BASE:not-full-sha INTENTS:unreadable VERIFY:unreadable OUTCOME:invalid'* ]] || fail "finish unusable inputs: $out"
ok

# ASD-V0-017: last activity from transcript timestamps or mtime; replay flags gaps.
now=$(perl -MTime::Local -e 'print timegm(0, 0, 12, 18, 9, 2026)')
mkdir -p "$tmp/lanes/busy" "$tmp/lanes/parked"
printf '{"timestamp":"2026-10-18T11:50:00.000Z"}\n' > "$tmp/lanes/busy/t.jsonl"
printf '{"timestamp":"2026-10-17T18:00:00Z"}\n{"timestamp":"2026-10-18T10:36:00Z"}\n' > "$tmp/lanes/parked/t.jsonl"
"$script/lane-idle-check.sh" --now "$now" "$tmp/lanes/busy" > "$tmp/out"
[[ $? == 0 ]] && grep -q '^LANE ACTIVE idle=10m last=2026-10-18T11:50:00Z ' "$tmp/out" || fail "idle-check active: $(cat "$tmp/out")"
"$script/lane-idle-check.sh" --now "$now" "$tmp/lanes/busy" "$tmp/lanes/parked" > "$tmp/out"
[[ $? == 1 ]] && grep -q '^LANE IDLE idle=84m last=2026-10-18T10:36:00Z ' "$tmp/out" || fail "idle-check idle: $(cat "$tmp/out")"
"$script/lane-idle-check.sh" --gaps "$tmp/lanes" > "$tmp/out"
[[ $? == 1 && $(wc -l < "$tmp/out") -eq 1 ]] &&
  grep -q '^GAP idle=996m from=2026-10-17T18:00:00Z to=2026-10-18T10:36:00Z flagged=2026-10-17T18:30:00Z ' "$tmp/out" ||
  fail "idle-check gaps: $(cat "$tmp/out")"
: > "$tmp/lanes/out.log"
perl -e 'utime $ARGV[0], $ARGV[0], $ARGV[1] or die' "$((now - 3600))" "$tmp/lanes/out.log"
"$script/lane-idle-check.sh" --now "$now" --idle-minutes 90 "$tmp/lanes/out.log" | grep -q '^LANE ACTIVE idle=60m ' ||
  fail "idle-check mtime"
ok

# ASD-V0-018: only merged, clean, inactive checkouts are removed, and only with --apply.
g() { git -c user.name=t -c user.email=t@example.invalid -c init.defaultBranch=main "$@"; }
g init -q --bare "$tmp/origin.git"
g clone -q "$tmp/origin.git" "$tmp/seed" 2> /dev/null
g -C "$tmp/seed" commit -q --allow-empty -m base && g -C "$tmp/seed" push -q origin HEAD:main 2> /dev/null
mkdir "$tmp/lanes2"
for n in merged unmerged dirty active stash; do g clone -q "$tmp/origin.git" "$tmp/lanes2/$n" 2> /dev/null; done
g -C "$tmp/lanes2/unmerged" commit -q --allow-empty -m unmerged
printf x > "$tmp/lanes2/dirty/new.txt"
printf x > "$tmp/lanes2/stash/f" && g -C "$tmp/lanes2/stash" add f && g -C "$tmp/lanes2/stash" stash -q
g -C "$tmp/seed" worktree add -q "$tmp/lanes2/wt" 2> /dev/null
g -C "$tmp/seed" fetch -q
clean() { "$script/lane-clean.sh" --root "$tmp/lanes2" --repo "$tmp/seed" --active "$tmp/lanes2/active" "$@"; }
clean > "$tmp/out" || fail "lane-clean dry run failed"
expect='KEEP active LANES/active
KEEP dirty LANES/dirty
WOULD-REMOVE LANES/merged
KEEP stash LANES/stash
KEEP unmerged-head LANES/unmerged
WOULD-REMOVE LANES/wt'
real=$(cd "$tmp/lanes2" && pwd -P)
[[ $(grep -v '^SUMMARY' "$tmp/out" | sed "s#$real#LANES#") == "$expect" ]] || fail "lane-clean dry run: $(cat "$tmp/out")"
grep -q '^SUMMARY checkouts=6 keep=4 remove=2 applied=no ' "$tmp/out" || fail "lane-clean summary: $(tail -1 "$tmp/out")"
[[ -d $tmp/lanes2/merged && -d $tmp/lanes2/wt ]] || fail "lane-clean dry run removed a checkout"
clean --apply > "$tmp/out" || fail "lane-clean apply failed"
[[ ! -e $tmp/lanes2/merged && ! -e $tmp/lanes2/wt ]] || fail "lane-clean --apply kept a merged checkout"
for n in unmerged dirty active stash; do [[ -d $tmp/lanes2/$n ]] || fail "lane-clean removed $n"; done
[[ -z $(git -C "$tmp/seed" worktree list --porcelain | grep "lanes2/wt") ]] || fail "lane-clean left a worktree entry"
ok

printf 'lane-scripts_test: %d groups passed\n' "$passed"
