#!/usr/bin/env bash
# Tests for the lane procedure scripts (docs/LANE-ORCHESTRATION.md): heavy.sh (ASD-V0-014),
# the dogfood-finish missing-input refusal (ASD-V0-015), lane-idle-check.sh (ASD-V0-017),
# lane-clean.sh (ASD-V0-018), lane-guard.sh (ASD-V0-024),
# install-local-bin.sh (ASD-V0-025), install-cem-verifiers.sh (TCB-V0-046). No Go build runs.
set -uo pipefail
export LC_ALL=C
# The fixtures run real Git commands, commits and pushes; a caller's leaked repository environment
# (GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, GIT_CONFIG_PARAMETERS..., as a Git hook exports) would
# point them at the caller's repository and its remote instead of the fixtures.
unset $(git rev-parse --local-env-vars)

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
# AHI-199 (V1-2440): the command runs at niceness 10 by default, HEAVY_NICE=0 keeps the caller's
# priority, an invalid level is refused, and the Makefile heavy recipes carry the same prefix.
want=$(ps -o nice= -p $$ | tr -d ' ')
got=$(env -u HEAVY_NICE HEAVY_SLOTS=1 "$script/heavy.sh" sh -c 'ps -o nice= -p $$' 2> /dev/null | tr -d ' ')
[[ $got == $((want > 10 ? want : 10)) ]] || fail "heavy.sh default niceness $got, want 10 or the caller's higher $want"
got=$(HEAVY_NICE=0 HEAVY_SLOTS=1 "$script/heavy.sh" sh -c 'ps -o nice= -p $$' 2> /dev/null | tr -d ' ')
[[ $got == "$want" ]] || fail "heavy.sh HEAVY_NICE=0 niceness $got, want the caller's $want"
HEAVY_NICE=20 "$script/heavy.sh" true 2> "$tmp/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-nice' "$tmp/err" || fail "heavy.sh accepted HEAVY_NICE=20"
makefile="$script/../Makefile"
grep -qx 'HEAVY_NICE ?= 10' "$makefile" || fail "Makefile HEAVY_NICE default is not 10"
grep -q '^HEAVY_NICE_PREFIX = $(if $(filter-out 0,$(HEAVY_NICE)),' "$makefile" || fail "Makefile HEAVY_NICE=0 does not opt out"
for recipe in 'GO_TEST_COMMAND = $(HEAVY_NICE_PREFIX) ' '	@$(HEAVY_NICE_PREFIX) env $(GATE_LEDGER) run $* ' \
  '$(HEAVY_NICE_PREFIX) script/dogfood-change.sh ' '$(HEAVY_NICE_PREFIX) script/dogfood-check.sh '; do
  grep -qF -- "$recipe" "$makefile" || fail "Makefile heavy recipe lacks the nice prefix: $recipe"
done
ok
# ASD-V0-014 (V1-2384): the repository root go.mod's pinned release (a nested module's go line
# does not override it) goes first on PATH when installed; without it the go on PATH runs and one
# warning names it.
pin=$tmp/gopin
mkdir -p "$pin/mod/sub" "$pin/path" "$pin/home" "$pin/cache/golang.org/toolchain@v0.0.1-go1.99.1.tos-tarch/bin"
printf 'module x\n\ngo 1.99.1\n' > "$pin/mod/go.mod"
printf 'module x/sub\n\ngo 1.98.0\n' > "$pin/mod/sub/go.mod"
git init -q "$pin/mod" || fail "git init"
printf '#!/bin/sh\n[ "$2" = GOVERSION ] && echo go1.99.2 && [ $# -gt 2 ] && printf "%%s\\n" "%s" tos tarch\nexit 0\n' "$pin/cache" > "$pin/path/go"
printf '#!/bin/sh\necho go1.99.1\n' > "$pin/cache/golang.org/toolchain@v0.0.1-go1.99.1.tos-tarch/bin/go"
chmod +x "$pin/path/go" "$pin/cache/golang.org/toolchain@v0.0.1-go1.99.1.tos-tarch/bin/go"
got=$(cd "$pin/mod/sub" && HOME=$pin/home PATH=$pin/path:$PATH HEAVY_SLOTS=1 "$script/heavy.sh" go env GOVERSION 2> "$tmp/err")
[[ $got == go1.99.1 ]] && ! grep -q pinned "$tmp/err" || fail "heavy.sh ran $got, want the pinned go1.99.1"
/bin/rm -rf "$pin/cache/golang.org"
got=$(cd "$pin/mod/sub" && HOME=$pin/home PATH=$pin/path:$PATH HEAVY_SLOTS=1 "$script/heavy.sh" go env GOVERSION 2> "$tmp/err")
[[ $got == go1.99.2 ]] && grep -q '^heavy.sh: pinned go1.99.1 not installed; running go1.99.2$' "$tmp/err" ||
  fail "heavy.sh without the pinned go: ran $got, stderr $(cat "$tmp/err")"
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

# DCW-V0-090: a committed spec edit without regenerated indexes is refused before any build;
# regenerating and committing clears it. The fixture has no build helper, so a passing check
# stops at the missing dogfood-build-corvint.sh instead.
fx=$tmp/specfix
mkdir -p "$fx/script" "$fx/docs/specs"
cp "$script/dogfood-finish.sh" "$script/gen-spec-requirements.sh" "$fx/script/"
git -C "$fx" init -q -b main
printf '# One spec\n\nIntent status: proposed\nDelivery status: not-started\n\n## Agent digest\n- Claim: One does the first thing.\n\n- ONE-V0-001: The first clause.\n' > "$fx/docs/specs/one.md"
printf '[]\n' > "$fx/docs/specs/INDEX.json"
printf '# Specs\n\n| Scope | Contract | Intent | Delivery | Binding evidence or next gate |\n| --- | --- | --- | --- | --- |\n' > "$fx/docs/specs/README.md"
fxgit() { git -C "$fx" -c user.name=t -c user.email=t@example.invalid "$@"; }
fxregen() { (cd "$fx" && git add -A && d=$(mktemp -d) && script/gen-spec-requirements.sh --all "$d" &&
  cp "$d/REQUIREMENTS.tsv" "$d/INDEX.json" "$d/README.md" docs/specs/ && /bin/rm -rf "$d" && git add -A); }
fxregen || fail "spec fixture generation"
fxgit commit -qm base
fxbase=$(fxgit rev-parse HEAD)
printf -- '- ONE-V0-002: The second clause.\n' >> "$fx/docs/specs/one.md"
fxgit commit -qam 'spec edit only'
: > "$fx/plan.tsv"
out=$(cd "$fx" && env -u CORVINT_BIN script/dogfood-finish.sh "$fxbase" plan.tsv 2>&1)
[[ $? == 2 && $out == *'REFUSE spec-requirements-stale docs/specs/REQUIREMENTS.tsv'*'fix: '*'make spec-requirements'* ]] ||
  fail "finish accepted a stale requirements index: $out"
fxregen || fail "spec fixture regeneration"
fxgit commit -qm regenerate
out=$(cd "$fx" && env -u CORVINT_BIN script/dogfood-finish.sh "$fxbase" plan.tsv 2>&1)
[[ $out != *spec-requirements-stale* && $out == *dogfood-build-corvint.sh* ]] || fail "finish refused regenerated indexes: $out"
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

# ASD-V0-024 (V1-1953): lane-guard.sh refuses real polling loops and unwrapped go test, but never
# matches heredoc bodies, which are file content. Inputs are built as JSON by perl so the cases
# carry real newlines.
guard() { # guard CWD COMMAND -> prints the decision (empty when allowed)
  perl -MJSON::PP -e 'print JSON::PP->new->encode({cwd => $ARGV[0], tool_input => {command => $ARGV[1]}})' "$1" "$2" |
    "$script/lane-guard.sh"
}
denied() { [[ $(guard "$tmp" "$1") == *'"permissionDecision":"deny"'* ]]; }
loop_body=$'while true; do\n  sleep 5\ndone'
for cmd in \
  "until gh pr view 1 --json state | grep -q MERGED; do sleep 30; done" \
  "cd x && while ! test -f done; do sleep 1; done" \
  "for i in \$(seq 1 20); do gh run view 1; sleep 30; done" \
  "tail -f log | grep -m1 PASS" \
  "go test ./internal/x" \
  "cd lane && GOTOOLCHAIN=local go test -run TestA ./internal/x" \
  "make build && env GOFLAGS=-p=4 go test ./cmd/corvint"; do
  denied "$cmd" || fail "lane-guard allowed: $cmd"
done
for cmd in \
  "cat > notes.sh <<'EOF'"$'\n'"$loop_body"$'\nEOF' \
  "cat > t.sh <<EOF"$'\n'"$loop_body"$'\ngo test ./x\nEOF\nchmod +x t.sh' \
  "cat <<-\"END\" > t.sh"$'\n\t'"until false; do sleep 1; done"$'\n\tEND' \
  "script/heavy.sh env GOTOOLCHAIN=local go test -run TestA ./internal/x" \
  "/private/tmp/cem10-build/heavy.sh env GOTOOLCHAIN=local go test -p 4 -run TestA ./internal/x" \
  "git log --oneline -3" \
  "grep -n 'while' script/x.sh"; do
  out=$(guard "$tmp" "$cmd")
  [[ -z $out ]] || fail "lane-guard refused file content or a wrapped command: $cmd -> $out"
done
# A command after the heredoc terminator is a command again.
denied "cat > a.txt <<EOF"$'\nplain\nEOF\n'"until false; do sleep 2; done" || fail "lane-guard missed a loop after a heredoc"
# A here-string is not a heredoc: its line stays a command.
denied "until grep -q x <<< \"\$v\"; do sleep 1; done" || fail "lane-guard treated <<< as a heredoc"
# LANE_GUARD_ROOTS scopes the guard; outside every root nothing is checked.
[[ -z $(LANE_GUARD_ROOTS=/nonexistent-root guard "$tmp" "go test ./x") ]] || fail "lane-guard checked outside LANE_GUARD_ROOTS"
LANE_GUARD_ROOTS=/nonexistent-root:$tmp denied "go test ./x" || fail "lane-guard skipped a cwd inside LANE_GUARD_ROOTS"
[[ $(LANE_GUARD_HEAVY=/opt/heavy.sh guard "$tmp" "go test ./x") == *'through /opt/heavy.sh'* ]] || fail "lane-guard ignored LANE_GUARD_HEAVY"
[[ -z $(printf 'not json' | "$script/lane-guard.sh") ]] || fail "lane-guard refused unparseable input"
ok

# ASD-V0-025 (V1-1979): install-local-bin.sh keeps a bounded rollback set and never prunes
# backups it did not create.
bin=$tmp/bin
mkdir -p "$bin"
printf 'v0\n' > "$tmp/v0"; printf 'v1\n' > "$tmp/v1"; printf 'v2\n' > "$tmp/v2"; printf 'v3\n' > "$tmp/v3"
printf 'old\n' > "$bin/tool.bak-foreign"
"$script/install-local-bin.sh" "$tmp/v0" "$bin/tool" > /dev/null || fail "install-local-bin first install failed"
for v in v1 v2 v3; do
  "$script/install-local-bin.sh" "$tmp/$v" "$bin/tool" > "$tmp/install.out" || fail "install-local-bin install $v failed"
done
[[ $(cat "$bin/tool") == v3 && -x $bin/tool ]] || fail "install-local-bin did not install v3"
backups=$(cd "$bin" && /bin/ls tool.corvint-prev-* | tr '\n' ' ')
[[ $backups == 'tool.corvint-prev-3 ' && $(cat "$bin/tool.corvint-prev-3") == v2 ]] || fail "install-local-bin kept unbounded backups: $backups"
[[ -f $bin/tool.bak-foreign ]] || fail "install-local-bin pruned a foreign backup"
grep -q '^REMOVED .*tool.corvint-prev-2$' "$tmp/install.out" || fail "install-local-bin did not report the pruned backup"
"$script/install-local-bin.sh" --rollback "$bin/tool" > /dev/null || fail "install-local-bin rollback failed"
[[ $(cat "$bin/tool") == v2 && $(cat "$bin/tool.corvint-prev-4") == v3 ]] || fail "install-local-bin rollback did not swap"
"$script/install-local-bin.sh" --rollback "$bin/tool" > /dev/null || fail "install-local-bin second rollback failed"
[[ $(cat "$bin/tool") == v3 ]] || fail "install-local-bin second rollback did not return"
"$script/install-local-bin.sh" --keep 2 "$tmp/v1" "$bin/tool" > /dev/null || fail "install-local-bin --keep 2 failed"
[[ $(cd "$bin" && /bin/ls tool.corvint-prev-* | wc -l | tr -d ' ') == 2 ]] || fail "install-local-bin ignored --keep 2"
"$script/install-local-bin.sh" "$tmp/missing" "$bin/tool" 2> "$tmp/install.err" && fail "install-local-bin accepted a missing source"
grep -q 'REFUSE source-unreadable' "$tmp/install.err" || fail "install-local-bin did not name the missing source"
"$script/install-local-bin.sh" --keep 0 "$tmp/v1" "$bin/tool" 2> /dev/null && fail "install-local-bin accepted --keep 0"
"$script/install-local-bin.sh" --rollback "$bin/none" 2> /dev/null && fail "install-local-bin rolled back without a backup"
ok

# TCB-V0-046 (V1-1761): install-cem-verifiers.sh builds corvint and cem01-go from the same clean
# HEAD, refuses an unstamped build or a dirty checkout before installing, and --check names a
# mismatched, modified or unrecorded pair the way the cem-verdict gate does. A stub GO records each
# build and answers `version -m` from the stub binary's content.
cem=$(cd "$tmp" && pwd)/cem-repo
mkdir -p "$cem/script" "$cem/cmd/corvint" "$cem/interop/cem01-go" "$tmp/cem-bin"
/bin/cp -f "$script/install-cem-verifiers.sh" "$script/install-local-bin.sh" "$cem/script/"
printf 'package main\n' > "$cem/cmd/corvint/main.go"
printf 'package main\n' > "$cem/interop/cem01-go/main.go"
git -C "$cem" init -q && git -C "$cem" add -A &&
  git -C "$cem" -c user.name=t -c user.email=t@t commit -qm one || fail "install-cem-verifiers fixture commit failed"
cat > "$tmp/stub-go" << 'STUB'
#!/usr/bin/env bash
case $1 in
  build)
    out=''
    while (($# > 0)); do [[ $1 == -o ]] && out=$2; shift; done
    printf '%s %s %s/%s\n' "$PWD" "$GOTOOLCHAIN" "$GOOS" "$GOARCH" >> "$STUB_LOG"
    printf 'stub-build %s %s %s %s\n' "${STUB_REV:-$(git rev-parse HEAD)}" "${STUB_MODIFIED:-false}" \
      "${STUB_GOOS:-$GOOS}" "$GOARCH" > "$out"
    ;;
  version)
    read -r tag rev modified goos goarch < "$3" 2> /dev/null && [[ $tag == stub-build ]] || exit 1
    printf '%s: go1.27.3\n\tpath\tstub\n\tbuild\tGOARCH=%s\n\tbuild\tGOOS=%s\n\tbuild\tvcs=git\n\tbuild\tvcs.revision=%s\n\tbuild\tvcs.modified=%s\n' \
      "$3" "$goarch" "$goos" "$rev" "$modified"
    ;;
  env) [[ $2 == GOHOSTOS && $3 == GOHOSTARCH ]] && printf 'tos\ntarch\n' || exit 9 ;;
  *) exit 9 ;;
esac
STUB
chmod +x "$tmp/stub-go"
export STUB_LOG=$tmp/stub-go.log
cemv() { GO=$tmp/stub-go "$cem/script/install-cem-verifiers.sh" "$@"; }
head1=$(git -C "$cem" rev-parse HEAD)
cemv --bin-dir "$tmp/cem-bin" > "$tmp/cemv.out" 2>&1 || fail "install-cem-verifiers install failed: $(cat "$tmp/cemv.out")"
[[ $(tail -1 "$tmp/cemv.out") == "MATCH $head1" ]] || fail "install-cem-verifiers did not report the installed pair: $(cat "$tmp/cemv.out")"
[[ $(cat "$STUB_LOG") == "$cem local tos/tarch"$'\n'"$cem/interop/cem01-go local tos/tarch" ]] || fail "install-cem-verifiers built from the wrong directories: $(cat "$STUB_LOG")"
[[ $(cemv --check --bin-dir "$tmp/cem-bin") == "MATCH $head1" ]] || fail "install-cem-verifiers --check did not match a fresh pair"
git -C "$cem" -c user.name=t -c user.email=t@t commit -q --allow-empty -m two
head2=$(git -C "$cem" rev-parse HEAD)
# A cross-compilation environment does not leak into the builds (Codex P2).
: > "$STUB_LOG"
GOOS=plan9 GOARCH=386 cemv --bin-dir "$tmp/cem-bin" > /dev/null 2>&1 || fail "install-cem-verifiers reinstall failed"
[[ $(sort -u <<< "$(cut -d' ' -f3 "$STUB_LOG")") == tos/tarch ]] || fail "install-cem-verifiers cross-built: $(cat "$STUB_LOG")"
[[ $(cemv --check --bin-dir "$tmp/cem-bin") == "MATCH $head2" ]] || fail "install-cem-verifiers did not install the new HEAD"
[[ -f $tmp/cem-bin/corvint.corvint-prev-1 && -f $tmp/cem-bin/cem01-go.corvint-prev-1 ]] || fail "install-cem-verifiers kept no rollback backups"
# A stale consumer (the V1-1761 observation), a modified build and an unrecorded build each fail.
printf 'stub-build %s false\n' "$head1" > "$tmp/cem-bin/cem01-go"
got=$(cemv --check --bin-dir "$tmp/cem-bin") && fail "install-cem-verifiers --check accepted a stale consumer"
[[ $got == "MISMATCH corvint=$head2 cem01-go=$head1" ]] || fail "install-cem-verifiers stale consumer: $got"
printf 'stub-build %s true\n' "$head2" > "$tmp/cem-bin/cem01-go"
got=$(cemv --check --bin-dir "$tmp/cem-bin") && fail "install-cem-verifiers --check accepted a modified consumer"
[[ $got == "MISMATCH corvint=$head2 cem01-go=$head2+modified" ]] || fail "install-cem-verifiers modified consumer: $got"
printf '#!/bin/sh\n' > "$tmp/cem-bin/cem01-go"
got=$(cemv --check --bin-dir "$tmp/cem-bin") && fail "install-cem-verifiers --check accepted an unrecorded consumer"
[[ $got == "UNRECORDED corvint=$head2 cem01-go=-" ]] || fail "install-cem-verifiers unrecorded consumer: $got"
/bin/rm -f "$tmp/cem-bin/cem01-go"
cemv --check --bin-dir "$tmp/cem-bin" > /dev/null 2> "$tmp/cemv.err"
[[ $? == 2 ]] && grep -q 'REFUSE missing-tool' "$tmp/cemv.err" || fail "install-cem-verifiers --check did not refuse a missing tool"
# Refusals before any install: an untracked file, an unstamped build, bad arguments.
/bin/cp -f "$tmp/cem-bin/corvint" "$tmp/corvint-before"
: > "$STUB_LOG"
printf 'x\n' > "$cem/untracked"
cemv --bin-dir "$tmp/cem-bin" > /dev/null 2> "$tmp/cemv.err"
[[ $? == 2 ]] && grep -q 'REFUSE uncommitted-changes' "$tmp/cemv.err" || fail "install-cem-verifiers installed from a dirty checkout"
[[ ! -s $STUB_LOG ]] || fail "install-cem-verifiers built from a dirty checkout"
/bin/rm -f "$cem/untracked"
STUB_REV=$head1 cemv --bin-dir "$tmp/cem-bin" > /dev/null 2> "$tmp/cemv.err"
[[ $? == 2 ]] && grep -q 'REFUSE build-not-stamped corvint' "$tmp/cemv.err" || fail "install-cem-verifiers installed a build of another revision"
STUB_MODIFIED=true cemv --bin-dir "$tmp/cem-bin" > /dev/null 2> "$tmp/cemv.err"
[[ $? == 2 ]] && grep -q 'REFUSE build-not-stamped' "$tmp/cemv.err" || fail "install-cem-verifiers installed a modified build"
STUB_GOOS=other cemv --bin-dir "$tmp/cem-bin" > /dev/null 2> "$tmp/cemv.err"
[[ $? == 2 ]] && grep -q "REFUSE build-not-host corvint targets 'other/tarch'" "$tmp/cemv.err" ||
  fail "install-cem-verifiers installed a build for another target"
cmp -s "$tmp/corvint-before" "$tmp/cem-bin/corvint" && [[ ! -e $tmp/cem-bin/cem01-go ]] || fail "a refused install-cem-verifiers run wrote the bin dir"
cemv --keep 0 --bin-dir "$tmp/cem-bin" 2> /dev/null; [[ $? == 2 ]] || fail "install-cem-verifiers accepted --keep 0"
cemv --bogus 2> /dev/null; [[ $? == 2 ]] || fail "install-cem-verifiers accepted an unknown argument"
cemv --bin-dir "$tmp/no-such-dir" 2> /dev/null; [[ $? == 2 ]] || fail "install-cem-verifiers accepted a missing bin dir"
unset STUB_LOG
ok

printf 'lane-scripts_test: %d groups passed\n' "$passed"
