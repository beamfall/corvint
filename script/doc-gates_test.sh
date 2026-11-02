#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# ASD-V0-779 (V1-3615): script/doc-gates.sh runs every gate in one call, a failing gate stops no
# other (the conformance gate included), each gate prints as one block, and the run ends naming
# every failed gate with the command that reruns only those.
set -uo pipefail
script=$(cd "$(dirname "$0")" && pwd)/doc-gates.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/doc-gates-test.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT
fail() { printf 'doc-gates_test: FAIL %s\n' "$*" >&2; exit 1; }

repo=$tmp/repo
mkdir -p "$repo" && cd "$repo" || exit 2
git init -q .
printf 'ok:\n\t@echo fine one\n\t@echo fine two\nbad1:\n\t@echo broken one\n\t@exit 1\nbad2:\n\t@echo broken two\n\t@exit 3\n' > Makefile
# hang records its recipe shell and a background grandchild, then waits on it.
printf 'hang:\n\t@echo $$$$ > "$(PIDS)/shell"; sleep 60 & echo $$! > "$(PIDS)/grandchild"; wait\n' >> Makefile
printf '#!/bin/sh\necho "conformance $*"\nexit 4\n' > "$tmp/go"
chmod +x "$tmp/go"

out=$(DOC_GATES_GO=$tmp/go DOC_GATES_JOBS=3 "$script" bad1 ok bad2 conformance/use-cases-v0 2>&1)
rc=$?
[[ $rc == 1 ]] || fail "a run with failed gates exited $rc, want 1: $out"
for want in 'doc-gates: PASS ok (' 'doc-gates: FAIL bad1 (' 'doc-gates: FAIL bad2 (' 'doc-gates: FAIL conformance/use-cases-v0 (' \
  '  | broken one' '  | broken two' '  | conformance run ./conformance/use-cases-v0' \
  'doc-gates: FAIL 3 of 4 gates: bad1 bad2 conformance/use-cases-v0' \
  "rerun only those: make doc-gates DOC_GATES_RUN='bad1 bad2 conformance/use-cases-v0'"; do
  grep -qF -- "$want" <<< "$out" || fail "missing '$want' in: $out"
done
# Each block is whole: the ok gate's two lines follow its own header.
awk '/^doc-gates: PASS ok /{getline a; getline b; exit !(a == "  | fine one" && b == "  | fine two")}' <<< "$out" || fail "the ok block is split: $out"
timings=$(git rev-parse --git-path doc-gates.timings.tsv)
[[ $(cut -f1,2 "$timings" | tr '\t\n' ' ,') == 'target exit,bad1 2,ok 0,bad2 2,conformance/use-cases-v0 4,' ]] || fail "timings: $(cat "$timings")"

out=$("$script" ok 2>&1) || fail "a passing run failed: $out"
grep -qx 'doc-gates: PASS 1 gates' <<< "$out" || fail "no pass line: $out"
"$script" > /dev/null 2>&1
[[ $? == 2 ]] || fail "no gate: want usage exit 2"
# A holder killed while it holds the output lock never stalls the runner: the runner waits while the
# lock is held, prints nothing meanwhile, and finishes once the holder dies, its lock file left behind.
lock=$tmp/lock
perl -e '$^F = 1023; use Fcntl qw(:flock); open(my $l, ">>", shift) or die; flock($l, LOCK_EX) or die; exec @ARGV' "$lock" sleep 60 &
holder=$!
sleep 0.5
DOC_GATES_LOCK=$lock "$script" ok > "$tmp/held.out" 2>&1 &
runner=$!
sleep 1.5
kill -0 "$runner" 2> /dev/null || fail "the runner did not wait for the held lock: $(cat "$tmp/held.out")"
grep -q '^doc-gates: PASS ok' "$tmp/held.out" && fail "a block printed while another process held the lock"
kill -9 "$holder"
wait "$holder" 2> /dev/null
(sleep 20; kill -9 "$runner" 2> /dev/null) &
watchdog=$!
wait "$runner"
rc=$?
kill "$watchdog" 2> /dev/null
[[ -e $lock ]] || fail "the dead holder's lock file is gone, so this case tests nothing"
[[ $rc == 0 ]] && grep -qx 'doc-gates: PASS 1 gates' "$tmp/held.out" || fail "the runner stalled or failed after the holder died (rc $rc): $(cat "$tmp/held.out")"

# Two runs in one worktree leave one whole timings file, never rows of both under one header.
"$script" ok bad1 > /dev/null 2>&1 &
first=$!
"$script" ok > /dev/null 2>&1
wait "$first"
[[ $(grep -c '^target' "$timings") == 1 ]] || fail "timings hold two headers: $(cat "$timings")"
rows=$(cut -f1 "$timings" | tail -n +2 | paste -sd ' ' -)
[[ $rows == 'ok bad1' || $rows == 'ok' ]] || fail "timings mix two runs: $(cat "$timings")"
# An interrupted run leaves no worker, make recipe or grandchild alive.
pids=$tmp/pids
mkdir -p "$pids"
PIDS=$pids "$script" hang ok > "$tmp/int.out" 2>&1 &
runner=$!
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do [[ -s $pids/grandchild ]] && break; sleep 0.25; done
[[ -s $pids/shell && -s $pids/grandchild ]] || fail "the hang gate never started: $(cat "$tmp/int.out")"
kill -TERM "$runner"
wait "$runner"
rc=$?
[[ $rc == 130 ]] || fail "an interrupted run exited $rc, want 130: $(cat "$tmp/int.out")"
sleep 0.5
for who in shell grandchild; do
  pid=$(cat "$pids/$who")
  kill -0 "$pid" 2> /dev/null && { kill -9 "$pid" 2> /dev/null; fail "the $who ($pid) of an interrupted run survived it"; }
done

# A lock that cannot be opened or taken, and a perl that cannot lock, fail the gate and the run:
# no PASS without the gate's block.
lockfail() { # NAME OUT -- ENV...: the run must exit 1 and name the unprintable gate
  local what=$1 out
  shift
  out=$(env "$@" "$script" ok 2>&1)
  local rc=$?
  [[ $rc == 1 ]] || fail "$what: exit $rc, want 1: $out"
  grep -q 'doc-gates: PASS 1 gates' <<< "$out" && fail "$what: the run passed: $out"
  grep -qE 'could not be printed|left no exit status|the gate runner exited' <<< "$out" || fail "$what: the failure is not named: $out"
}
lockfail 'a lock in a missing directory' DOC_GATES_LOCK="$tmp/no-such-dir/lock"
lockfail 'a lock path that is a directory' DOC_GATES_LOCK="$tmp"
if [[ $(id -u) != 0 ]]; then
  : > "$tmp/ro.lock" && chmod 444 "$tmp/ro.lock"
  lockfail 'a read-only lock file' DOC_GATES_LOCK="$tmp/ro.lock"
fi
fakeperl=$tmp/fakeperl
mkdir -p "$fakeperl"
printf '#!/bin/sh
echo "Can'"'"'t locate Fcntl.pm in @INC" >&2
exit 2
' > "$fakeperl/perl"
chmod +x "$fakeperl/perl"
lockfail 'a perl without Fcntl' PATH="$fakeperl:$PATH"
# With no perl at all the run refuses before any gate.
noperl=$tmp/noperl
mkdir -p "$noperl"
for tool in bash sh env git make sed tr xargs mktemp cp mv rm getconf cut cat dirname id sleep; do
  found=$(command -v "$tool") && ln -sf "$found" "$noperl/$tool"
done
out=$(PATH=$noperl "$script" ok 2>&1)
[[ $? == 2 ]] && grep -q 'perl is required' <<< "$out" || fail "a run without perl did not refuse: $out"
printf 'doc-gates_test: PASS\n'
