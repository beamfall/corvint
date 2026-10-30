#!/bin/sh
# cce-live.sh PARALLEL TIMEOUT: `make cce-live` (CCE-V0-102, V1-2908). Builds this package's test
# binary once and runs each live prove test (^TestCCEV[0-9]+Live) in its own process, at most
# PARALLEL at once, each under -test.timeout TIMEOUT.
#
# Prove sets process-wide environment (PATH and each side's Playwright base URL), and some live
# tests use t.Setenv, so the tests cannot share one process under t.Parallel. Separate processes
# keep every test and every check unchanged; only the wall time drops.
#
# Each test's start and its `--- PASS/FAIL/SKIP: name (duration)` line print as they happen; every
# test's full -test.v output follows in name order. The exit status is nonzero when the binary does
# not build, lists no live test, or any test did not exit 0. An INT or TERM stops every worker and
# every process under it before the scratch directory is removed, and exits 130 or 143.
set -eu

if [ "${1-}" = --one ]; then
	# --one BIN LOGS TIMEOUT NAME: one test, called by xargs below.
	bin=$2 logs=$3 timeout=$4 name=$5
	echo "=== START $name"
	st=0
	"$bin" -test.v -test.count=1 -test.timeout="$timeout" -test.run "^$name\$" > "$logs/$name.log" 2>&1 || st=$?
	grep -E "^--- (PASS|FAIL|SKIP): $name " "$logs/$name.log" || echo "--- FAIL: $name (no result line)"
	if [ "$st" -eq 0 ]; then
		: > "$logs/$name.ok"
	else
		echo "--- $name exited $st"
	fi
	exit 0
fi

parallel=$1 timeout=$2
cd "$(dirname "$0")"
self=$(pwd)/$(basename "$0")
work=$(mktemp -d "${TMPDIR:-/tmp}/cce-live.XXXXXX")

# runner is the pid of the step running in the background, so a trapped signal interrupts the
# wait at once instead of after the step ends.
runner=

# descendants PID prints every live descendant of PID, parents first (POSIX ps; no pgrep, setsid
# or job control, which a non-interactive dash without a tty lacks).
descendants() {
	ps -A -o pid= -o ppid= | awk -v root="$1" '
		{ kids[$2] = kids[$2] " " $1 }
		END {
			n = split(kids[root], queue, " ")
			for (i = 1; i <= n; i++) {
				print queue[i]
				m = split(kids[queue[i]], below, " ")
				for (j = 1; j <= m; j++) queue[++n] = below[j]
			}
		}'
}

# stop terminates the running step and its whole process tree, as it stood when stop began,
# waits up to 10 s for it to exit, then kills what is left.
stop() {
	[ -n "$runner" ] || return 0
	tree="$runner $(descendants "$runner")"
	runner=
	kill -TERM $tree 2>/dev/null || :
	tries=0
	while [ "$tries" -lt 10 ]; do
		alive=
		for pid in $tree; do
			if kill -0 "$pid" 2>/dev/null; then
				alive=1
				break
			fi
		done
		[ -n "$alive" ] || break
		sleep 1
		tries=$((tries + 1))
	done
	kill -KILL $tree 2>/dev/null || :
	wait 2>/dev/null || :
}

cleanup() {
	stop
	/bin/rm -rf "$work"
}
trap cleanup EXIT
trap 'trap - EXIT; cleanup; exit 130' INT
trap 'trap - EXIT; cleanup; exit 143' TERM

# step IN CMD...: run CMD in the background with standard input from IN and wait for it, returning
# its status. An asynchronous list reads /dev/null, so the input must be redirected on CMD itself.
step() {
	in=$1
	shift
	"$@" < "$in" &
	runner=$!
	st=0
	wait "$runner" || st=$?
	runner=
	return "$st"
}

step /dev/null go test -c -o "$work/changee2e.test" .
tests=$("$work/changee2e.test" -test.list '^TestCCEV[0-9]+Live')
if [ -z "$tests" ]; then
	echo "cce-live: no live test is listed" >&2
	exit 1
fi
mkdir "$work/logs"
start=$(date +%s)
printf '%s\n' $tests > "$work/tests"
step "$work/tests" xargs -n 1 -P "$parallel" sh "$self" --one "$work/changee2e.test" "$work/logs" "$timeout"
elapsed=$(($(date +%s) - start))

failed=0
for name in $tests; do
	echo "=== OUTPUT $name"
	cat "$work/logs/$name.log"
	[ -e "$work/logs/$name.ok" ] || failed=$((failed + 1))
done
if [ "$failed" -ne 0 ]; then
	echo "FAIL cce-live: $failed of $(echo $tests | wc -w | tr -d ' ') live tests failed (${elapsed}s, -P $parallel)"
	exit 1
fi
echo "ok cce-live: $(echo $tests | wc -w | tr -d ' ') live tests (${elapsed}s, -P $parallel)"
