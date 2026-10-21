#!/usr/bin/env bash
# Tests for the lane procedure scripts (docs/LANE-ORCHESTRATION.md): heavy.sh (ASD-V0-014),
# the dogfood-finish missing-input refusal (ASD-V0-015), lane-idle-check.sh (ASD-V0-017) and
# lane-clean.sh (ASD-V0-018). No Go build runs.
set -uo pipefail
export LC_ALL=C

script=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-lane-scripts-test.XXXXXX") || exit 1
trap 'chmod -R u+rwx "$tmp" 2> /dev/null; /bin/rm -rf "$tmp"' EXIT
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

# ASD-V0-014: the slot covers the workload. Each case synchronises on a marker its own workload
# writes once admitted. Killing only the wrapper keeps the slot until the workload exits; killing
# wrapper and workload frees it at once; a forwarded TERM reaches the workload's whole group.
wait_for() { for _ in $(seq 100); do [[ -s $1 ]] && return 0; sleep 0.1; done; fail "timed out waiting for $1"; }
gone() { for _ in $(seq 50); do kill -0 "$1" 2> /dev/null || return 0; sleep 0.1; done; return 1; }
HEAVY_SLOTS=1 "$script/heavy.sh" sh -c 'echo $$ > "$1/w1.pid"; sleep 3' _ "$tmp" 2> /dev/null &
holder=$!
wait_for "$tmp/w1.pid"
kill -9 "$holder" 2> /dev/null
wait "$holder" 2> /dev/null
busy() { perl -MFcntl=:flock -e 'open(my $f, ">>", $ARGV[0]) or exit 2; exit(flock($f, LOCK_EX | LOCK_NB) ? 1 : 0)' "$1"; }
busy "$HEAVY_LOCK_DIR/slot1" || fail "heavy.sh slot freed while a killed wrapper's workload ran"
HEAVY_SLOTS=1 "$script/heavy.sh" true 2> /dev/null || fail "heavy.sh failed after a killed wrapper"
gone "$(cat "$tmp/w1.pid")" || fail "heavy.sh slot reclaimed while the workload still ran"
HEAVY_SLOTS=1 "$script/heavy.sh" sh -c 'echo $$ > "$1/w2.pid"; exec sleep 30' _ "$tmp" 2> /dev/null &
holder=$!
wait_for "$tmp/w2.pid"
kill -9 "$holder" "$(cat "$tmp/w2.pid")" 2> /dev/null
wait "$holder" 2> /dev/null
gone "$(cat "$tmp/w2.pid")" || fail "killed workload still running"
start=$(date +%s)
HEAVY_SLOTS=1 "$script/heavy.sh" true 2> /dev/null || fail "heavy.sh failed after a killed owner"
(($(date +%s) - start < 3)) || fail "heavy.sh waited on a dead owner's slot"
HEAVY_SLOTS=1 "$script/heavy.sh" sh -c 'sleep 30 & echo $! > "$1/g.pid"; wait' _ "$tmp" 2> /dev/null &
holder=$!
wait_for "$tmp/g.pid"
kill -TERM "$holder"
wait "$holder" 2> /dev/null
gone "$(cat "$tmp/g.pid")" || { kill "$(cat "$tmp/g.pid")" 2> /dev/null; fail "heavy.sh did not forward TERM to the workload's group"; }
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
: > "$tmp/locked.txt" && chmod 000 "$tmp/locked.txt"
out=$(finish 1 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa plan.tsv locked.txt locked.txt passed)
chmod 600 "$tmp/locked.txt"
[[ $out == *'REFUSE missing-inputs INTENTS:unreadable VERIFY:unreadable'$'\n''fix: '* ]] || fail "finish accepted a file it cannot read: $out"
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
mkdir -p "$tmp/lanes3/offset" "$tmp/lanes3/locked" "$tmp/lanes3/odd"
printf '{"timestamp":"2026-10-18T13:50:00.5+02:00"}\n' > "$tmp/lanes3/offset/t.jsonl"
"$script/lane-idle-check.sh" --now "$now" "$tmp/lanes3/offset" > "$tmp/out"
[[ $? == 0 ]] && grep -q '^LANE ACTIVE idle=10m last=2026-10-18T11:50:00Z ' "$tmp/out" || fail "idle-check offset: $(cat "$tmp/out")"
printf '{"timestamp":"2026-10-18T11:59:00Z"}\n' > "$tmp/lanes3/locked/t.jsonl"
chmod 000 "$tmp/lanes3/locked/t.jsonl"
"$script/lane-idle-check.sh" --now "$now" "$tmp/lanes3/locked" > "$tmp/out"
[[ $? == 1 ]] && grep -q '^LANE UNKNOWN idle=? last=? reason=unreadable:' "$tmp/out" || fail "idle-check unreadable: $(cat "$tmp/out")"
"$script/lane-idle-check.sh" --gaps "$tmp/lanes3/locked" > "$tmp/out"
[[ $? == 1 ]] && grep -q '^UNKNOWN reason=unreadable ' "$tmp/out" || fail "idle-check gaps unreadable: $(cat "$tmp/out")"
chmod 600 "$tmp/lanes3/locked/t.jsonl"
for v in '"yesterday"' 1760788740 '"2026-10-18 11:59"'; do
  printf '{"timestamp":"2026-10-18T11:58:00Z"}\n{"timestamp":%s}\n' "$v" > "$tmp/lanes3/odd/t.jsonl"
  "$script/lane-idle-check.sh" --now "$now" "$tmp/lanes3/odd" > "$tmp/out"
  [[ $? == 1 ]] && grep -q '^LANE UNKNOWN idle=? last=? reason=unparsed-timestamp:' "$tmp/out" || fail "idle-check unparsed $v: $(cat "$tmp/out")"
done
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
g clone -q "$tmp/origin.git" "$tmp/lanes2/host" 2> /dev/null
g -C "$tmp/lanes2/host" worktree add -q "$tmp/elsewhere/hostwt" 2> /dev/null
g -C "$tmp/seed" fetch -q
clean() { "$script/lane-clean.sh" --root "$tmp/lanes2" --repo "$tmp/seed" --active "$tmp/lanes2/active" "$@"; }
# Dry runs only: a root that cannot be resolved, or /, is refused.
mkdir "$tmp/sealed" && chmod 000 "$tmp/sealed"
"$script/lane-clean.sh" --root "$tmp/sealed" --repo "$tmp/seed" > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'REFUSE root-unusable' "$tmp/out" || fail "lane-clean unresolved root: $(head -3 "$tmp/out")"
chmod 700 "$tmp/sealed"
"$script/lane-clean.sh" --root / --repo "$tmp/seed" > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'REFUSE root-unusable' "$tmp/out" || fail "lane-clean accepted --root /: $(head -3 "$tmp/out")"
# A failing Git inspection never counts as clean (dry runs through a failing git shim).
mkdir "$tmp/shim"
printf '#!/bin/sh\nfor a in "$@"; do [ "$a" = "$LANE_TEST_FAIL_GIT" ] && exit 128; done\nexec "$LANE_TEST_REAL_GIT" "$@"\n' > "$tmp/shim/git"
chmod +x "$tmp/shim/git"
real_git=$(command -v git)
for sub in stash:git-unreadable for-each-ref:git-unreadable worktree:has-worktrees; do
  PATH=$tmp/shim:$PATH LANE_TEST_REAL_GIT=$real_git LANE_TEST_FAIL_GIT=${sub%%:*} clean > "$tmp/out"
  grep -q "^KEEP ${sub#*:} .*/merged\$" "$tmp/out" || fail "lane-clean with failing git ${sub%%:*}: $(cat "$tmp/out")"
done
clean > "$tmp/out" || fail "lane-clean dry run failed"
expect='KEEP active LANES/active
KEEP dirty LANES/dirty
KEEP has-worktrees LANES/host
WOULD-REMOVE LANES/merged
KEEP stash LANES/stash
KEEP unmerged-head LANES/unmerged
WOULD-REMOVE LANES/wt'
real=$(cd "$tmp/lanes2" && pwd -P)
[[ $(grep -v '^SUMMARY' "$tmp/out" | sed "s#$real#LANES#") == "$expect" ]] || fail "lane-clean dry run: $(cat "$tmp/out")"
grep -q '^SUMMARY checkouts=7 keep=5 remove=2 applied=no ' "$tmp/out" || fail "lane-clean summary: $(tail -1 "$tmp/out")"
[[ -d $tmp/lanes2/merged && -d $tmp/lanes2/wt ]] || fail "lane-clean dry run removed a checkout"
clean --apply > "$tmp/out" || fail "lane-clean apply failed"
[[ ! -e $tmp/lanes2/merged && ! -e $tmp/lanes2/wt ]] || fail "lane-clean --apply kept a merged checkout"
for n in unmerged dirty active stash host; do [[ -d $tmp/lanes2/$n ]] || fail "lane-clean removed $n"; done
[[ -z $(git -C "$tmp/seed" worktree list --porcelain | grep "lanes2/wt") ]] || fail "lane-clean left a worktree entry"
[[ -d $tmp/elsewhere/hostwt ]] || fail "lane-clean removed a clone's linked worktree"
ok

printf 'lane-scripts_test: %d groups passed\n' "$passed"
