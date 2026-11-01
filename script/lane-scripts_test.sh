#!/usr/bin/env bash
# Tests for the lane procedure scripts (docs/LANE-ORCHESTRATION.md): heavy.sh (ASD-V0-014, its
# remote offload ASD-V0-028),
# the dogfood-finish missing-input refusal (ASD-V0-015), lane-idle-check.sh (ASD-V0-017),
# lane-clean.sh (ASD-V0-018), lane-guard.sh (ASD-V0-024),
# install-local-bin.sh (ASD-V0-025), install-cem-verifiers.sh (TCB-V0-046),
# wsl-keepalive-check.sh (ASD-V0-039), batch-review-packet.sh (ASD-V0-043), codex-review.sh
# (ASD-V0-064), lane-setup.sh (ASD-V0-071, ASD-V0-072), requirement-ids-free.sh (ASD-V0-960). No
# Go build runs.
set -uo pipefail
export LC_ALL=C
# The fixtures run real Git commands, commits and pushes; a caller's leaked repository environment
# (GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, GIT_CONFIG_PARAMETERS..., as a Git hook exports) would
# point them at the caller's repository and its remote instead of the fixtures.
unset $(git rev-parse --local-env-vars)

script=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-lane-scripts-test.XXXXXX") || exit 1
trap 'chmod -R u+rwx "$tmp" 2> /dev/null; /bin/rm -rf "$tmp"' EXIT
# DCW-V0-358 (V1-2857): a failure is recorded with its group and line and the run goes on, so one
# run reports every failing group. A group is the code up to its `ok`; `ok` counts it passed only
# when it recorded no failure. `summary` lists the failing groups and returns 1 when there are any.
passed=0 group=1 failures=$tmp/failures.log
: > "$failures" || exit 1
fail() {
  local msg=$*
  printf 'lane-scripts_test: FAIL %s\n' "$msg" >&2
  msg=${msg//$'\n'/ } msg=${msg//$'\t'/ }
  printf '%s\t%s\t%s\n' "$group" "${BASH_LINENO[0]}" "${msg:0:300}" >> "$failures"
  return 1
}
ok() {
  cut -f1 "$failures" | grep -qx "$group" || passed=$((passed + 1))
  group=$((group + 1))
}
summary() {
  [[ -s ${heavy_escapes-} ]] && printf 'lane-scripts_test: heavy.sh fixture runs refused by the lock-dir guard:\n%s\n' "$(cat "$heavy_escapes")" >&2
  if [[ ! -s $failures ]]; then
    printf 'lane-scripts_test: %d groups passed\n' "$passed"
    return 0
  fi
  printf 'lane-scripts_test: %d groups passed, %d failed:\n' "$passed" "$(cut -f1 "$failures" | sort -un | wc -l | tr -d ' ')" >&2
  awk -F '\t' '!seen[$1]++ { n[$1] = 0; order[++k] = $1; first[$1] = "line " $2 ": " $3 } { n[$1]++ }
    END { for (i = 1; i <= k; i++) { g = order[i]; printf "  group %s: %s", g, first[g]; if (n[g] > 1) printf " (+%d more)", n[g] - 1; printf "\n" } }' "$failures" >&2
  return 1
}
# DCW-V0-358: the runner checks itself first with three synthetic groups, the first and third failing.
if [[ ${1-} == --runner-self-test ]]; then
  fail 'synthetic one'
  ok
  ok
  fail 'synthetic three'
  fail 'synthetic three again'
  ok
  summary
  exit
fi
self=$(bash "$script/${0##*/}" --runner-self-test 2>&1)
self_rc=$?
self_line=$(grep -n "^  fail 'synthetic one'" "$script/${0##*/}" | cut -d: -f1)
self_want="lane-scripts_test: FAIL synthetic one
lane-scripts_test: FAIL synthetic three
lane-scripts_test: FAIL synthetic three again
lane-scripts_test: 1 groups passed, 2 failed:
  group 1: line $self_line: synthetic one
  group 3: line $((self_line + 3)): synthetic three (+1 more)"
[[ $self_rc == 1 && $self == "$self_want" ]] || { printf 'lane-scripts_test: FAIL the group runner self-test (exit %s):\n%s\n' "$self_rc" "$self" >&2; exit 1; }

# ASD-V0-014: two concurrent commands with HEAVY_SLOTS=1 never overlap. No caller's heavy.sh
# configuration or remote host reaches the fixtures.
# V1-3631: no HEAVY_* setting of the caller (HEAVY_REMOTE_HOSTS, HEAVY_HELD, a load multiple...)
# reaches a fixture; the groups below set what they test.
# shellcheck disable=SC2046 # one word per variable name
unset $(compgen -v HEAVY_) CODEX_REVIEW_HEAVY
export HEAVY_LOCK_DIR=$tmp/slots HEAVY_CONFIG=$tmp/no-heavy-config
# ASD-V0-937/938 are exercised in their own groups: elsewhere no disk floor and no added -timeout.
export HEAVY_MIN_FREE_GB=0 HEAVY_GO_TEST_TIMEOUT=off
# ASD-V0-030 policy is exercised in its own group; elsewhere fixtures run whole packages and the
# host's real load never delays them.
export HEAVY_WHOLE_REASON='lane-scripts-test fixture' HEAVY_MAX_LOAD=999999 HEAVY_SELECTION_LOG=$tmp/selection.log
# ASD-V0-040: no fixture queries a real CI; the guard's own group sets a repository and a stub gh.
export HEAVY_CI_REPO=off
# DCW-V0-355 (V1-2853): every heavy.sh fixture run goes through $heavy, a guard that asks heavy.sh
# for its lock dir (`--print-lock-dir`, DCW-V0-357: HEAVY_LOCK_DIR, else the HEAVY_CONFIG file, else
# the shared /tmp/corvint-heavy-<uid>) and, following symlinks, refuses with exit 97 and a logged line
# unless it lies inside $tmp. HEAVY_SLOTS=0 makes a heavy.sh without that flag refuse instead of
# taking a slot. A fixture whose override is unset therefore cannot take a slot from real lanes;
# the summary group fails on any logged refusal even when a fixture expected a nonzero exit.
mkdir -p "$tmp/guard" || exit 1
tmp_real=$(cd -P "$tmp" && pwd -P) || exit 1
heavy=$tmp/guard/heavy.sh heavy_escapes=$tmp/guard/escapes.log
cat > "$heavy" << GUARD
#!/usr/bin/env bash
lock=\$(HEAVY_SLOTS=0 "$script/heavy.sh" --print-lock-dir 2> /dev/null) && [[ -n \$lock ]] || lock='(heavy.sh --print-lock-dir failed)'
[[ \$lock == /* ]] || lock=\$PWD/\$lock
real=
case /\$lock/ in
*/../* | */./*) ;;
*)
  dir=\${lock%/} rest=
  while [[ -n \$dir && ! -d \$dir ]]; do rest=/\${dir##*/}\$rest dir=\${dir%/*}; done
  real=\$(cd -P "\${dir:-/}" && pwd -P)\$rest
  ;;
esac
case \$real/ in "$tmp_real"/*) exec "$script/heavy.sh" "\$@" ;; esac
printf 'lock=%s pwd=%s cmd=%s\n' "\$lock" "\$PWD" "\$*" >> "$heavy_escapes"
printf 'lane-scripts_test guard: heavy.sh lock dir %s is outside %s\n' "\$lock" "$tmp_real" >&2
exit 97
GUARD
chmod +x "$heavy" || exit 1
# DCW-V0-355: the guard fires for an unset override, a config outside $tmp, and a symlink out of it;
# heavy.sh never runs for any of them.
env -u HEAVY_LOCK_DIR HEAVY_CONFIG="$tmp/no-heavy-config" "$heavy" sh -c ': > "$1/guard-ran"' _ "$tmp" 2> "$tmp/guard.err"
[[ $? == 97 && ! -e $tmp/guard-ran ]] && grep -q "^lock=/tmp/corvint-heavy-$(id -u) " "$heavy_escapes" &&
  grep -q 'is outside' "$tmp/guard.err" || fail "the lock-dir guard let an unset HEAVY_LOCK_DIR reach heavy.sh"
printf 'export HEAVY_LOCK_DIR="/tmp/corvint-heavy-guard-probe"\n' > "$tmp/guard/heavy.env"
env -u HEAVY_LOCK_DIR HEAVY_CONFIG="$tmp/guard/heavy.env" "$heavy" sh -c ': > "$1/guard-ran"' _ "$tmp" 2> /dev/null
[[ $? == 97 && ! -e $tmp/guard-ran ]] && grep -q '^lock=/tmp/corvint-heavy-guard-probe ' "$heavy_escapes" ||
  fail "the lock-dir guard let a config lock dir outside the test reach heavy.sh"
ln -s / "$tmp/guard/out"
HEAVY_LOCK_DIR=$tmp/guard/out/tmp/x "$heavy" sh -c ': > "$1/guard-ran"' _ "$tmp" 2> /dev/null
[[ $? == 97 && ! -e $tmp/guard-ran && $(wc -l < "$heavy_escapes") == *3 ]] ||
  fail "the lock-dir guard followed no symlink out of the test directory"
HEAVY_LOCK_DIR=$tmp/guard/slots "$heavy" sh -c ': > "$1/guard-ran"' _ "$tmp" 2> /dev/null && [[ -e $tmp/guard-ran ]] ||
  fail "the lock-dir guard refused a lock dir inside the test directory"
: > "$heavy_escapes"
# DCW-V0-357 (V1-2856): the dir --print-lock-dir names is the dir a run locks, from the environment,
# from HEAVY_CONFIG, and relative to the working directory; an empty HEAVY_LOCK_DIR means the default.
agree=$tmp/agree
mkdir -p "$agree" || exit 1
printf 'HEAVY_LOCK_DIR=%s\n' "$agree/cfg" > "$agree/heavy.env"
for case in "env|HEAVY_LOCK_DIR=$agree/env|$agree/env|$agree/env" "config|HEAVY_CONFIG=$agree/heavy.env|$agree/cfg|$agree/cfg" \
  "relative|HEAVY_LOCK_DIR=rel|rel|$agree/rel"; do
  IFS='|' read -r name assign want where <<< "$case"
  got=$(cd "$agree" && env -u HEAVY_LOCK_DIR "$assign" "$script/heavy.sh" --print-lock-dir 2> /dev/null)
  [[ $got == "$want" ]] || fail "heavy.sh --print-lock-dir ($name) printed '$got', want '$want'"
  (cd "$agree" && env -u HEAVY_LOCK_DIR "$assign" HEAVY_SLOTS=1 "$heavy" true 2> /dev/null) &&
    [[ -e $where/slot1 ]] || fail "heavy.sh ($name) did not lock the dir --print-lock-dir named, $where"
done
got=$(HEAVY_LOCK_DIR='' HEAVY_CONFIG=$agree/heavy.env "$script/heavy.sh" --print-lock-dir 2> /dev/null)
[[ $got == "/tmp/corvint-heavy-$(id -u)" ]] || fail "heavy.sh --print-lock-dir with an empty HEAVY_LOCK_DIR printed '$got'"
"$script/heavy.sh" --print-lock-dir true > /dev/null 2> "$agree/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-arguments' "$agree/err" || fail "heavy.sh --print-lock-dir accepted a command"
ok

# ASD-V0-014: two concurrent commands with HEAVY_SLOTS=1 never overlap.
probe='mkdir "$1/running" || exit 9; sleep 1; rmdir "$1/running"'
HEAVY_SLOTS=1 "$heavy" sh -c "$probe" _ "$tmp" 2> "$tmp/a.err" &
a=$!
HEAVY_SLOTS=1 "$heavy" sh -c "$probe" _ "$tmp" 2> "$tmp/b.err" &
b=$!
wait "$a" || fail "heavy.sh first concurrent command overlapped (exit $?)"
wait "$b" || fail "heavy.sh second concurrent command overlapped (exit $?)"
grep -q '^heavy.sh: slot 1/1 after [1-9]' "$tmp/a.err" "$tmp/b.err" || fail "heavy.sh: no command waited for the slot"
ok

# ASD-V0-014: the command's exit status and -p bound pass through; an explicit -p is kept.
HEAVY_SLOTS=1 "$heavy" sh -c 'exit 7' 2> /dev/null
[[ $? == 7 ]] || fail "heavy.sh did not return the command's status"
got=$(HEAVY_SLOTS=2 HEAVY_P=3 GOFLAGS= "$heavy" sh -c 'printf %s "$GOFLAGS"' 2> /dev/null)
[[ $got == -p=3 ]] || fail "heavy.sh GOFLAGS=$got, want -p=3"
got=$(HEAVY_SLOTS=2 HEAVY_P=3 GOFLAGS='-p=1 -mod=mod' "$heavy" sh -c 'printf %s "$GOFLAGS"' 2> /dev/null)
[[ $got == '-p=1 -mod=mod' ]] || fail "heavy.sh overrode an explicit GOFLAGS -p: $got"
# AHI-199 (V1-2440): the command runs at niceness 10 by default, HEAVY_NICE=0 keeps the caller's
# priority, an invalid level is refused, and the Makefile heavy recipes carry the same prefix.
want=$(ps -o nice= -p $$ | tr -d ' ')
got=$(env -u HEAVY_NICE HEAVY_SLOTS=1 "$heavy" sh -c 'ps -o nice= -p $$' 2> /dev/null | tr -d ' ')
[[ $got == $((want > 10 ? want : 10)) ]] || fail "heavy.sh default niceness $got, want 10 or the caller's higher $want"
got=$(HEAVY_NICE=0 HEAVY_SLOTS=1 "$heavy" sh -c 'ps -o nice= -p $$' 2> /dev/null | tr -d ' ')
[[ $got == "$want" ]] || fail "heavy.sh HEAVY_NICE=0 niceness $got, want the caller's $want"
HEAVY_NICE=20 "$heavy" true 2> "$tmp/err"
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
got=$(cd "$pin/mod/sub" && HOME=$pin/home PATH=$pin/path:$PATH HEAVY_SLOTS=1 "$heavy" go env GOVERSION 2> "$tmp/err")
[[ $got == go1.99.1 ]] && ! grep -q pinned "$tmp/err" || fail "heavy.sh ran $got, want the pinned go1.99.1"
/bin/rm -rf "$pin/cache/golang.org"
got=$(cd "$pin/mod/sub" && HOME=$pin/home PATH=$pin/path:$PATH HEAVY_SLOTS=1 "$heavy" go env GOVERSION 2> "$tmp/err")
[[ $got == go1.99.2 ]] && grep -q '^heavy.sh: pinned go1.99.1 not installed; running go1.99.2$' "$tmp/err" ||
  fail "heavy.sh without the pinned go: ran $got, stderr $(cat "$tmp/err")"
HEAVY_SLOTS=0 "$heavy" true 2> "$tmp/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-slots' "$tmp/err" || fail "heavy.sh accepted HEAVY_SLOTS=0"
ok

# ASD-V0-940 (V1-3247): a non-zero exit ends stderr with its cause; a zero exit adds no line.
HEAVY_SLOTS=1 "$heavy" false 2> "$tmp/err"
[[ $? == 1 && $(tail -n 1 "$tmp/err") == 'heavy.sh: exit rc=1 cause=command cmd=false' ]] ||
  fail "heavy.sh false: no exit cause line: $(cat "$tmp/err")"
HEAVY_SLOTS=1 "$heavy" sh -c 'kill -TERM $$' 2> "$tmp/err"
[[ $? == 143 && $(tail -n 1 "$tmp/err") == 'heavy.sh: exit rc=143 cause=signal-TERM cmd=sh' ]] ||
  fail "heavy.sh signalled command: no exit cause line: $(cat "$tmp/err")"
HEAVY_SLOTS=1 "$heavy" true 2> "$tmp/err" && ! grep -q 'exit rc=' "$tmp/err" || fail "heavy.sh true printed an exit line: $(cat "$tmp/err")"
(cd "$pin/mod" && env -u HEAVY_WHOLE_REASON HEAVY_SLOTS=1 "$heavy" go test ./sub > /dev/null 2> "$tmp/err")
[[ $? == 2 && $(tail -n 1 "$tmp/err") == 'heavy.sh: exit rc=2 cause=refused' ]] ||
  fail "heavy.sh refusal: no exit cause line: $(cat "$tmp/err")"
ok

# ASD-V0-014: the slot covers the workload. Each case synchronises on a marker its own workload
# writes once admitted. Killing only the wrapper keeps the slot until the workload exits; killing
# wrapper and workload frees it at once; a forwarded TERM reaches the workload's whole group.
wait_for() { for _ in $(seq 100); do [[ -s $1 ]] && return 0; sleep 0.1; done; fail "timed out waiting for $1"; }
gone() { for _ in $(seq 50); do kill -0 "$1" 2> /dev/null || return 0; sleep 0.1; done; return 1; }
HEAVY_SLOTS=1 "$heavy" sh -c 'echo $$ > "$1/w1.pid"; sleep 3' _ "$tmp" 2> /dev/null &
holder=$!
wait_for "$tmp/w1.pid"
kill -9 "$holder" 2> /dev/null
wait "$holder" 2> /dev/null
busy() { perl -MFcntl=:flock -e 'open(my $f, ">>", $ARGV[0]) or exit 2; exit(flock($f, LOCK_EX | LOCK_NB) ? 1 : 0)' "$1"; }
busy "$HEAVY_LOCK_DIR/slot1" || fail "heavy.sh slot freed while a killed wrapper's workload ran"
HEAVY_SLOTS=1 "$heavy" true 2> /dev/null || fail "heavy.sh failed after a killed wrapper"
gone "$(cat "$tmp/w1.pid")" || fail "heavy.sh slot reclaimed while the workload still ran"
HEAVY_SLOTS=1 "$heavy" sh -c 'echo $$ > "$1/w2.pid"; exec sleep 30' _ "$tmp" 2> /dev/null &
holder=$!
wait_for "$tmp/w2.pid"
kill -9 "$holder" "$(cat "$tmp/w2.pid")" 2> /dev/null
wait "$holder" 2> /dev/null
gone "$(cat "$tmp/w2.pid")" || fail "killed workload still running"
start=$(date +%s)
HEAVY_SLOTS=1 "$heavy" true 2> /dev/null || fail "heavy.sh failed after a killed owner"
(($(date +%s) - start < 3)) || fail "heavy.sh waited on a dead owner's slot"
HEAVY_SLOTS=1 "$heavy" sh -c 'sleep 30 & echo $! > "$1/g.pid"; wait' _ "$tmp" 2> /dev/null &
holder=$!
wait_for "$tmp/g.pid"
kill -TERM "$holder"
wait "$holder" 2> /dev/null
gone "$(cat "$tmp/g.pid")" || { kill "$(cat "$tmp/g.pid")" 2> /dev/null; fail "heavy.sh did not forward TERM to the workload's group"; }
ok
# ASD-V0-028: a go test offloads to HEAVY_REMOTE_HOST at the checkout's exact tree (an unpushed
# commit as a bundle, the uncommitted diff, untracked files, the working directory), returns its
# status, admits through the remote flock semaphore, cleans up the remote, and falls back to a local
# run when ssh fails or the remote clone cannot reproduce the tree. ssh is a stub running the remote
# shell on this host with its own HOME and TMPDIR; go is a stub reporting where and on what it ran.
off=$tmp/off
mkdir -p "$off/bin" "$off/rhome" "$off/rtmp" "$off/ltmp"
cat > "$off/bin/ssh" << SSH
#!/usr/bin/env bash
while [[ \$1 == -o ]]; do shift 2; done
host=\$1; shift
printf '%s\n' "\$host" >> "$off/ssh.log"
[[ \$host == down ]] && exit 255
if [[ \$host == flaky && ! -e $off/flaky.once ]]; then : > "$off/flaky.once"; exit 255; fi
[[ \$host == hang ]] && { echo \$\$ > "$off/hang.pid"; exec sleep 30; }
# V1-3631: like sshd, the fake remote gets only PATH, HOME, TMPDIR and a locale, never this shell's env.
tee "$off/payload.last" | env -i PATH="\$PATH" HOME="$off/rhome" TMPDIR="$off/rtmp" LC_ALL=C bash -c "\$*"
SSH
cat > "$off/bin/go" << GO
#!/usr/bin/env bash
if [[ \$1 == env ]]; then
  shift
  for a; do case \$a in GOOS) echo "\${GOOS:-stubos}" ;; GOARCH) echo "\${GOARCH:-stubarch}" ;; *) echo ;; esac; done
  exit 0
fi
if [[ \$1 == build ]]; then
  # ASD-V0-041: a build writes "built-WHERE GOOS/GOARCH" to its -o / -o= / NAME_BIN= output (DIR/ gets DIR/app).
  case \$PWD in */off/rhome/.cache/corvint-heavy/wt/*) where=remote ;; *) where=local ;; esac
  printf 'build where=%s\n' "\$where"
  o= prev=
  for a in "\$@"; do
    case \$a in -exit=*) exit "\${a#-exit=}" ;; -o=*) o=\${a#-o=} ;; *BIN=*) o=\${a#*BIN=} ;; esac
    [[ \$prev == -o ]] && o=\$a
    prev=\$a
  done
  [[ -n \$o ]] || exit 0
  [[ \$o == */ ]] && o=\${o}app
  mkdir -p "\$(dirname "\$o")"
  printf 'built-%s %s/%s\n' "\$where" "\${GOOS-}" "\${GOARCH-}" > "\$o"
  exit 0
fi
[[ \$1 == test || \$1 == vet || \$1 == windows-vet ]] || exit 0
case \$PWD in */off/rhome/.cache/corvint-heavy/wt/*) where=remote; printf '%s\n' "\$TMPDIR" > "$off/tmpdir.last"; printf '%s\n' "\${GOTMPDIR-}" > "$off/gotmpdir.last"; : > "\$TMPDIR/leak" ;; *) where=local ;; esac
printf 'where=%s dir=%s gocache=%s mark=%s\n' "\$where" "\${PWD##*/}" "\${GOCACHE-unset}" "\${FAKE_MARK-}"
cat ../committed.txt ../wip.txt untracked.txt 2> /dev/null
for a in "\$@"; do
  case \$a in
    -exit=*) exit "\${a#-exit=}" ;;
    -probe) mkdir "$off/running" || exit 9; sleep 1; rmdir "$off/running" ;;
    -write=*) mkdir -p "\$(dirname "\${a#-write=}")" && printf 'written-%s\n' "\$where" > "\${a#-write=}" ;;
    -delete=*) /bin/rm -f "\${a#-delete=}" ;;
    -localedit=*) printf 'local-edit\n' > "$off/lc/sub/\${a#-localedit=}" ;;
  esac
done
GO
printf '#!/usr/bin/env bash\nshift\nexec "$@"\n' > "$off/bin/timeout"
chmod +x "$off/bin/ssh" "$off/bin/go" "$off/bin/timeout"
/bin/cp -f "$off/bin/go" "$off/bin/make"
g() { git -c user.name=t -c user.email=t@example.invalid -c init.defaultBranch=main "$@"; }
g init -q --bare "$off/origin.git"
g clone -q "$off/origin.git" "$off/lc" 2> /dev/null
mkdir -p "$off/lc/sub"
printf 'committed-base\n' > "$off/lc/committed.txt"
printf 'wip-base\n' > "$off/lc/wip.txt"
g -C "$off/lc" add -A && g -C "$off/lc" commit -qm base && g -C "$off/lc" push -q origin main 2> /dev/null ||
  fail "offload fixture push failed"
g clone -q "$off/origin.git" "$off/rhome/corvint" 2> /dev/null || fail "offload fixture remote clone failed"
printf 'committed-unpushed\n' > "$off/lc/committed.txt"
g -C "$off/lc" commit -qam unpushed || fail "offload fixture commit failed"
printf 'wip-uncommitted\n' > "$off/lc/wip.txt"
printf 'untracked\n' > "$off/lc/sub/untracked.txt"
printf 'export FAKE_MARK=sourced\n' > "$off/rhome/go.env"
# shellcheck disable=SC2088 # the remote expands ~
hv() { (cd "$off/lc/sub" && env PATH="$off/bin:$PATH" TMPDIR="$off/ltmp" HEAVY_REMOTE_SSH="${PICK_SSH:-$off/bin/ssh}" \
  HEAVY_REMOTE_SHELL='bash -s' HEAVY_REMOTE_ENV='~/go.env' HEAVY_SLOTS=1 HEAVY_REMOTE_RETRY=0 \
  HEAVY_FALLBACK_LOG="$off/fallback.log" "$heavy" "$@"); }
out=$(HEAVY_REMOTE_HOST=box hv env GOCACHE=/no/such/cache go test -exit=0 ./... 2> "$off/err")
# The local GOCACHE assignment is dropped; the remote Go cache defaults to the remote user cache dir,
# never /tmp (V1-2691).
[[ $out == "where=remote dir=sub gocache=$off/rhome/.cache/corvint-go-build-cache mark=sourced"$'\n''committed-unpushed'$'\n''wip-uncommitted'$'\n''untracked' ]] ||
  fail "heavy.sh offload ran another tree: $out $(cat "$off/err")"
# The header names the tested content: the Git tree of the working tree, computed here
# independently in a copy of the checkout, and its 2 paths differing from HEAD.
/bin/cp -R "$off/lc/." "$off/exp"
want_tree=$(g -C "$off/exp" add -A && g -C "$off/exp" write-tree) || fail "expected tree"
/bin/rm -rf "$off/exp"
# The remote TMPDIR was a per-run directory under the remote root (not /tmp), removed with its leak.
[[ $(cat "$off/tmpdir.last") == "$off/rhome/.cache/corvint-heavy/tmp/run."* && ! -e $(cat "$off/tmpdir.last") ]] ||
  fail "heavy.sh remote TMPDIR $(cat "$off/tmpdir.last") is not a removed per-run directory under the remote root"
grep -qE "^heavy.sh: running on .* at $(g -C "$off/lc" rev-parse HEAD) \+wip tree=$want_tree dirty=2\$" "$off/err" &&
  grep -q '^heavy.sh: slot 1/' "$off/err" ||
  fail "heavy.sh offload did not report the remote run, tested tree and slot: $(cat "$off/err")"
# ASD-V0-031 (V1-2819): the remote prints its phase wall-clocks, and the first run creates the
# slot's worktree at a fixed path that later runs reuse after resetting it to the tested tree.
phase='^heavy.sh: phase receive=[0-9]+\.[0-9]s wait=[0-9]+\.[0-9]s prepare=[0-9]+\.[0-9]s run=[0-9]+\.[0-9]s worktree=heavy-slot1/'
grep -qE "${phase}created\$" "$off/err" || fail "heavy.sh first offload phase line: $(cat "$off/err")"
rwt=$off/rhome/.cache/corvint-heavy/wt/heavy-slot1
printf 'stale\n' > "$rwt/wip.txt"
printf 'stale\n' > "$rwt/sub/stale.txt"
out=$(HEAVY_REMOTE_HOST=box hv go test -exit=5 ./... 2> "$off/err")
[[ $? == 5 ]] || fail "heavy.sh offload did not return the remote command's status"
[[ $out == *wip-uncommitted* && $out != *stale* && ! -e $rwt/sub/stale.txt ]] && grep -qE "${phase}reused\$" "$off/err" ||
  fail "heavy.sh did not reset the reused worktree to the tested tree: $out $(cat "$off/err")"
# ASD-V0-940: the remote holder's cause line is relayed, then the calling shell names the offload.
grep -qx 'heavy.sh: exit rc=5 cause=command cmd=go' "$off/err" && [[ $(tail -n 1 "$off/err") == 'heavy.sh: exit rc=5 cause=offload host=box' ]] ||
  fail "heavy.sh offload failure: no exit cause lines: $(tail -n 3 "$off/err")"
# ASD-V0-037 (V1-2905): each run names its runner by hash, and the offload line carries the remote's
# phase line and the runner's short hash.
sha=$( { shasum -a 256 "$script/heavy.sh" || sha256sum "$script/heavy.sh"; } 2> /dev/null | awk 'NR == 1 { print $1 }')
grep -qx "heavy.sh: script sha256=$sha $script/heavy.sh" "$off/err" || fail "heavy.sh did not print its script hash: $(cat "$off/err")"
grep -qE "^[0-9TZ:-]+ offload host=box pack=[0-9]+\.[0-9]s remote=[0-9]+\.[0-9]s rc=5 receive=[0-9]+\.[0-9]s wait=[0-9]+\.[0-9]s prepare=[0-9]+\.[0-9]s run=[0-9]+\.[0-9]s worktree=heavy-slot1/reused pwd=.*/lc/sub cmd=go test -exit=5 \./\.\.\. script=${sha:0:12}\$" "$tmp/selection.log" ||
  fail "heavy.sh did not log the offload phases: $(cat "$tmp/selection.log")"
wts() { g -C "$off/rhome/corvint" worktree list --porcelain | grep -c '^worktree '; }
[[ $(wts) == 2 && -z $(g -C "$off/rhome/corvint" for-each-ref refs/heavy) && -z $(ls -A "$off/rtmp")$(ls -A "$off/ltmp") ]] ||
  fail "heavy.sh offload left a worktree, ref or scratch directory: $(g -C "$off/rhome/corvint" worktree list; g -C "$off/rhome/corvint" for-each-ref refs/heavy; ls -A "$off/rtmp" "$off/ltmp")"
# A worktree removed behind Git's back (or left unusable) is recreated at the same path.
/bin/rm -rf "$rwt"
out=$(HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == where=remote*wip-uncommitted* && $(wts) == 2 ]] && grep -qE "${phase}created\$" "$off/err" ||
  fail "heavy.sh did not recreate a missing slot worktree: $out $(cat "$off/err"; g -C "$off/rhome/corvint" worktree list)"
ok
# V1-2552: an offload of packages holding darwin-only files says so, once on stderr and once in the
# selection log, naming the count and first file; a package without any stays silent, and a
# `//go:build darwin || linux` file is not darwin-only.
mkdir -p "$off/lc/sub/mac" "$off/lc/sub/both"
printf 'package mac\n' > "$off/lc/sub/mac/a_darwin_test.go"
printf '//go:build darwin && cgo\n\npackage mac\n' > "$off/lc/sub/mac/b.go"
printf '//go:build darwin || linux\n\npackage both\n' > "$off/lc/sub/both/c.go"
HEAVY_REMOTE_HOST=box hv go test -run X ./... > /dev/null 2> "$off/err"
grep -qx 'heavy.sh: offloading to box, which neither builds nor runs 2 darwin-only file(s), first ./mac/a_darwin_test.go; HEAVY_REMOTE=0 with HEAVY_LOCAL_REASON runs them here' "$off/err" &&
  grep -qE '^[0-9TZ:-]+ darwin-only host=box n=2 first=\./mac/a_darwin_test\.go pwd=.*/lc/sub cmd=go test -run X \./\.\.\. script=' "$tmp/selection.log" ||
  fail "heavy.sh offloaded darwin-only files silently: $(cat "$off/err") $(tail -n 3 "$tmp/selection.log")"
HEAVY_REMOTE_HOST=box hv go test -run X ./both ./mac/missing > /dev/null 2> "$off/err"
HEAVY_REMOTE_HOST=box hv go test -run X . >> /dev/null 2>> "$off/err"
grep -q 'darwin-only' "$off/err" && fail "heavy.sh named darwin-only files outside the tested packages: $(cat "$off/err")"
# ASD-V0-934: on a Darwin host (a stub uname) the same run never offloads; it runs here without
# HEAVY_LOCAL_REASON and its admission is logged on stderr, in the fallback log and the selection log.
mkdir -p "$off/darwin" && printf '#!/bin/sh\necho Darwin\n' > "$off/darwin/uname" && chmod +x "$off/darwin/uname" || fail "uname stub"
: > "$off/ssh.log"
out=$(PATH=$off/darwin:$PATH HEAVY_REMOTE_HOSTS='box box2' hv go test -run X ./... 2> "$off/err")
[[ $out == where=local* && ! -s $off/ssh.log ]] &&
  grep -qx 'heavy.sh: 2 darwin-only file(s), first ./mac/a_darwin_test.go: running on this darwin host, not offloading' "$off/err" &&
  grep -qE 'reason=darwin-local n=2 first=\./mac/a_darwin_test\.go cmd=go test -run X \./\.\.\.$' "$off/fallback.log" &&
  grep -qE '^[0-9TZ:-]+ darwin-local n=2 first=\./mac/a_darwin_test\.go pwd=.*/lc/sub cmd=go test -run X \./\.\.\. script=' "$tmp/selection.log" ||
  fail "heavy.sh offloaded darwin-only work from a Darwin host or did not record it: $out ssh=$(cat "$off/ssh.log") $(cat "$off/err")"
out=$(PATH=$off/darwin:$PATH HEAVY_REMOTE_HOST=box hv go test -run X ./both 2> "$off/err")
[[ $out == where=remote* ]] || fail "heavy.sh kept a portable package on a Darwin host: $out $(cat "$off/err")"
/bin/rm -rf "$off/lc/sub/mac" "$off/lc/sub/both"
ok
# ASD-V0-933 (V1-3518): with HEAVY_REMOTE_HOSTS each host is probed for its load and CPUs and the
# offload goes to the lowest load per CPU; an unavailable host passes the run to the next one before
# any retry; only when all are unavailable does the run retry and fall back; one host is not probed.
# ssh-pick wraps the stub: quiet (load 1 on 12 CPUs) and busy (40 on 12) answer the real probe through
# a stub sysctl and getconf; gone answers a probe of load 0.10 and refuses every other call.
for h in quiet:1.00 busy:40.00; do
  mkdir -p "$off/fake-${h%%:*}" &&
    printf '#!/bin/sh\n[ "$2" = vm.loadavg ] && echo "{ %s 1.00 1.00 }"\n' "${h#*:}" > "$off/fake-${h%%:*}/sysctl" &&
    printf '#!/bin/sh\necho 12\n' > "$off/fake-${h%%:*}/getconf" && chmod +x "$off/fake-${h%%:*}/sysctl" "$off/fake-${h%%:*}/getconf" || fail "pick fixture"
done
cat > "$off/bin/ssh-pick" << PICK
#!/usr/bin/env bash
args=("\$@")
while [[ \$1 == -o ]]; do shift 2; done
printf '%s\n' "\$1" >> "$off/pick.log"
IFS= read -r first
if [[ \$1 == gone ]]; then
  [[ \$first == __heavy_probe=1 ]] && { echo 'heavy.sh: probe load=0.10 cpus=12'; exit 0; }
  exit 255
fi
{ printf '%s\n' "\$first"; cat; } | PATH="$off/fake-\$1:\$PATH" exec "$off/bin/ssh" "\${args[@]}"
PICK
chmod +x "$off/bin/ssh-pick" || fail "ssh-pick stub"
pk() { PICK_SSH=$off/bin/ssh-pick hv "$@"; }
: > "$off/pick.log"
out=$(HEAVY_REMOTE_HOSTS='busy down quiet' HEAVY_REMOTE_HOST=ignored pk go test -run X ./... 2> "$off/err")
[[ $out == where=remote* && $(tr '\n' ' ' < "$off/pick.log") == 'busy down quiet quiet ' ]] &&
  grep -qx 'heavy.sh: remote hosts by load per CPU: busy:40.00/12,down:down,quiet:1.00/12; trying quiet first' "$off/err" &&
  grep -qE '^[0-9TZ:-]+ pick hosts=busy:40\.00/12,down:down,quiet:1\.00/12 chose=quiet probe=[0-9]+\.[0-9]s pwd=.*/lc/sub cmd=go test -run X \./\.\.\. script=' "$tmp/selection.log" &&
  grep -qE '^[0-9TZ:-]+ offload host=quiet ' "$tmp/selection.log" ||
  fail "heavy.sh did not offload to the least-loaded host: $out calls=$(cat "$off/pick.log") $(cat "$off/err")"
: > "$off/pick.log"
out=$(HEAVY_REMOTE_HOSTS='busy,gone' pk go test -run X ./... 2> "$off/err")
[[ $out == where=remote* && $(tr '\n' ' ' < "$off/pick.log") == 'busy gone gone busy ' ]] &&
  grep -qx 'heavy.sh: offload to gone unavailable; trying the next host' "$off/err" && ! grep -q 'retrying' "$off/err" ||
  fail "heavy.sh did not move on from an unavailable host: $out calls=$(cat "$off/pick.log") $(cat "$off/err")"
: > "$off/pick.log"
out=$(HEAVY_REMOTE_HOSTS='gone down' pk go test -run X ./... 2> "$off/err")
[[ $out == where=local* && $(tr '\n' ' ' < "$off/pick.log") == 'gone down gone down gone ' ]] &&
  grep -q '^heavy.sh: offload to gone unavailable; retrying in 0s$' "$off/err" &&
  grep -qE '^[0-9TZ:-]+ fallback host=gone ' "$tmp/selection.log" ||
  fail "heavy.sh did not retry once and fall back when every host was unavailable: $out calls=$(cat "$off/pick.log") $(cat "$off/err")"
: > "$off/pick.log"
# shellcheck disable=SC2088 # the remote expands ~
out=$(HEAVY_REMOTE_HOSTS=quiet HEAVY_REMOTE_CLONE='~/corvint' pk go test -run X ./... 2> "$off/err")
[[ $out == where=remote* && $(tr '\n' ' ' < "$off/pick.log") == 'quiet ' ]] && ! grep -q 'remote hosts by load' "$off/err" ||
  fail "heavy.sh probed a single configured host: calls=$(cat "$off/pick.log") $(cat "$off/err")"
HEAVY_REMOTE_HOSTS='a b' HEAVY_REMOTE_PROBE_TIMEOUT=0 pk go test -run X ./... > /dev/null 2> "$off/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-probe-timeout' "$off/err" || fail "heavy.sh accepted HEAVY_REMOTE_PROBE_TIMEOUT=0"
ok
# ASD-V0-937: a remote below HEAVY_MIN_FREE_GB is unavailable (the remote half checks its scratch and
# Go-cache file systems through a stub df), the probe orders it after the hosts with room, and the
# run falls back locally when it was the only host. ASD-V0-938: the remote GOTMPDIR is the per-run
# directory under the remote root. ASD-V0-939: a sh -c wrapped go test offloads unwrapped.
# ASD-V0-933: an empty HEAVY_REMOTE_HOST in the environment still keeps the run local.
for h in full:1048576 quiet:104857600; do
  mkdir -p "$off/fake-${h%%:*}" && printf '#!/bin/sh\nprintf "Filesystem 1024-blocks Used Available Capacity Mounted\\nfake 0 0 %s 0%%%% /\\n"\n' "${h#*:}" > "$off/fake-${h%%:*}/df" &&
    chmod +x "$off/fake-${h%%:*}/df" || fail "df stub"
done
printf '#!/bin/sh\n[ "$2" = vm.loadavg ] && echo "{ 0.10 1.00 1.00 }"\n' > "$off/fake-full/sysctl" && printf '#!/bin/sh\necho 12\n' > "$off/fake-full/getconf" &&
  chmod +x "$off/fake-full/sysctl" "$off/fake-full/getconf" || fail "full host stub"
: > "$off/pick.log"
out=$(HEAVY_MIN_FREE_GB=3 HEAVY_REMOTE_HOSTS='full quiet' pk go test -run X ./... 2> "$off/err")
[[ $out == where=remote* && $(tr '\n' ' ' < "$off/pick.log") == 'full quiet quiet ' ]] &&
  grep -qx 'heavy.sh: remote hosts by load per CPU: full:lowdisk:1G,quiet:1.00/12; trying quiet first' "$off/err" ||
  fail "heavy.sh did not order a low-disk host last: $out calls=$(cat "$off/pick.log") $(cat "$off/err")"
[[ $(cat "$off/gotmpdir.last") == "$off/rhome/.cache/corvint-heavy/tmp/run."* ]] || fail "remote GOTMPDIR is not the per-run directory: $(cat "$off/gotmpdir.last")"
out=$(HEAVY_MIN_FREE_GB=3 HEAVY_REMOTE_HOSTS=full pk go test -run X ./... 2> "$off/err")
[[ $out == where=local* ]] && grep -qE '^heavy.sh: remote .*: low-disk .*/run\.[^ ]* free=1G floor=3G$' "$off/err" &&
  grep -q 'running locally (logged)' "$off/err" || fail "heavy.sh ran on a remote below the disk floor: $out $(cat "$off/err")"
out=$(HEAVY_MIN_FREE_GB=0 HEAVY_REMOTE_HOSTS=full pk go test -run X ./... 2> "$off/err")
[[ $out == where=remote* ]] || fail "HEAVY_MIN_FREE_GB=0 did not disable the floor: $out $(cat "$off/err")"
HEAVY_MIN_FREE_GB=x HEAVY_REMOTE_HOST=box hv true 2> "$off/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-min-free' "$off/err" || fail "heavy.sh accepted HEAVY_MIN_FREE_GB=x"
out=$(HEAVY_REMOTE_HOST=box hv sh -c 'go test -run X ./...' 2> "$off/err")
[[ $out == where=remote* ]] && grep -qx 'heavy.sh: unwrapped sh -c: go test -run X ./...' "$off/err" ||
  fail "heavy.sh did not offload a sh -c wrapped go test: $out $(cat "$off/err")"
: > "$off/ssh.log"
out=$(HEAVY_REMOTE_HOST= HEAVY_REMOTE_HOSTS=box hv go test -run X ./... 2> "$off/err")
[[ $out == where=local* && ! -s $off/ssh.log ]] || fail "an empty HEAVY_REMOTE_HOST did not keep the run local: $out"
out=$(HEAVY_REMOTE_HOST=box hv --admit-only go test -run X ./... 2> "$off/err")
[[ $out == where=local* && ! -s $off/ssh.log ]] || fail "heavy.sh --admit-only offloaded: $out $(cat "$off/err")"
ok
: > "$off/ssh.log"
: > "$off/fallback.log"
out=$(HEAVY_REMOTE_HOST=down hv go test ./... 2> "$off/err")
[[ $out == 'where=local dir=sub'* ]] && grep -q '^heavy.sh: offload to down unavailable; retrying in 0s$' "$off/err" &&
  grep -q '^heavy.sh: offload to down unavailable; running locally (logged)$' "$off/err" ||
  fail "heavy.sh did not retry once and fall back to a local run when ssh failed: $out $(cat "$off/err")"
grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z pwd=.*/lc/sub reason=offload-unavailable cmd=go test \./\.\.\.$' "$off/fallback.log" ||
  fail "heavy.sh did not log the local fallback: $(cat "$off/fallback.log")"
# ASD-V0-037: a fallback is logged, and HEAVY_FALLBACK_ALERT fallbacks for one host with no
# successful offload to it between them raise one alert; a successful offload ends the streak.
[[ $(tr '\n' ' ' < "$off/ssh.log") == 'down down ' ]] || fail "heavy.sh ssh calls for down: $(cat "$off/ssh.log")"
grep -Eq '^[0-9TZ:-]+ fallback host=down pwd=.*/lc/sub cmd=go test \./\.\.\. script=[0-9a-f]{12}$' "$tmp/selection.log" &&
  ! grep -q 'ALERT' "$off/err" || fail "heavy.sh fallback line or premature alert: $(cat "$off/err" "$tmp/selection.log")"
HEAVY_REMOTE_HOST=down HEAVY_FALLBACK_ALERT=2 hv go test ./... > /dev/null 2> "$off/err"
grep -qx 'heavy.sh: ALERT remote-unavailable streak=2 host=down: the last 2 offloads ran locally; check the host (for a WSL box, script/wsl-keepalive-check.sh)' "$off/err" &&
  grep -Eq '^[0-9TZ:-]+ alert remote-unavailable streak=2 host=down pwd=' "$tmp/selection.log" ||
  fail "heavy.sh did not alert on a fallback streak: $(cat "$off/err")"
HEAVY_REMOTE_HOST=down HEAVY_FALLBACK_ALERT=0 hv go test ./... > /dev/null 2> "$off/err"
! grep -q 'ALERT' "$off/err" || fail "heavy.sh alerted with HEAVY_FALLBACK_ALERT=0"
printf '2026-10-27T00:00:00Z offload host=down pack=0.1s remote=0.1s rc=1 pwd=/x cmd=go test ./x script=0\n' >> "$tmp/selection.log"
HEAVY_REMOTE_HOST=down HEAVY_FALLBACK_ALERT=2 hv go test ./... > /dev/null 2> "$off/err"
! grep -q 'ALERT' "$off/err" || fail "heavy.sh alert ignored a successful offload: $(cat "$off/err")"
HEAVY_REMOTE_HOST=down HEAVY_FALLBACK_ALERT=x hv go test ./... > /dev/null 2> "$off/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-fallback-alert' "$off/err" || fail "heavy.sh accepted HEAVY_FALLBACK_ALERT=x"
HEAVY_REMOTE_HOST=down HEAVY_FALLBACK_ALERT=101 hv go test ./... > /dev/null 2> "$off/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-fallback-alert' "$off/err" || fail "heavy.sh accepted HEAVY_FALLBACK_ALERT=101"
# ASD-V0-037: while HEAVY_REMOTE_BUSY_PROCESS runs on the remote the offload is unavailable.
printf '#!/bin/sh\n[ "$1" = -x ] && [ "$2" = busyjob ]\n' > "$off/bin/pgrep" && chmod +x "$off/bin/pgrep" || fail "pgrep stub"
out=$(HEAVY_REMOTE_HOST=box HEAVY_REMOTE_BUSY_PROCESS=busyjob hv go test ./... 2> "$off/err")
[[ $out == where=local* ]] && grep -q '^heavy.sh: remote .*: busy: busyjob is running$' "$off/err" ||
  fail "heavy.sh offloaded while the busy process ran: $out $(cat "$off/err")"
out=$(HEAVY_REMOTE_HOST=box HEAVY_REMOTE_BUSY_PROCESS=idlejob hv go test ./... 2> "$off/err")
[[ $out == where=remote* ]] || fail "heavy.sh did not offload when the busy process was absent: $out $(cat "$off/err")"
/bin/rm -f "$off/bin/pgrep"
: > "$off/ssh.log"
out=$(HEAVY_REMOTE_HOST=flaky hv go test ./... 2> "$off/err")
[[ $out == where=remote* ]] && grep -q 'retrying in 0s' "$off/err" || fail "heavy.sh did not retry a failed offload: $out"
g -C "$off/rhome/corvint" remote set-url origin https://example.invalid/other/repo.git
out=$(HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == 'where=local'* ]] && grep -q 'is not a clone of' "$off/err" || fail "heavy.sh offloaded to a clone of another origin: $out"
g -C "$off/rhome/corvint" remote set-url origin "$off/origin.git"
[[ $(tr '\n' ' ' < "$off/ssh.log") == 'flaky flaky box box ' ]] || fail "heavy.sh ssh calls: $(cat "$off/ssh.log")"
# ASD-V0-056 (V1-3202): a plain clone of a local checkout offloads as that checkout's origin, at
# that checkout's origin/main base, rather than at the local main it cloned (here an unpushed
# commit the remote cannot have); a checkout that cannot offload names why before its local run.
g clone -q "$off/lc" "$off/plain" 2> /dev/null || fail "plain clone failed"
mkdir -p "$off/plain/sub" && printf 'plain\n' > "$off/plain/sub/untracked.txt"
hvp() { (cd "$off/plain/${1:-sub}" && shift && env PATH="$off/bin:$PATH" TMPDIR="$off/ltmp" HEAVY_REMOTE_SSH="$off/bin/ssh" \
  HEAVY_REMOTE_SHELL='bash -s' HEAVY_SLOTS=1 HEAVY_REMOTE_RETRY=0 HEAVY_FALLBACK_LOG="$off/fallback.log" "$heavy" "$@"); }
out=$(HEAVY_REMOTE_HOST=box hvp sub go test -run X ./... 2> "$off/err")
[[ $out == 'where=remote dir=sub'* ]] && ! grep -q 'offload unavailable' "$off/err" ||
  fail "heavy.sh did not offload from a plain clone of a local checkout: $out $(cat "$off/err")"
# A source checkout without origin/main names its recovery rather than bundling over its own main.
g -C "$off/lc" update-ref -d refs/remotes/origin/main
out=$(HEAVY_REMOTE_HOST=box hvp sub go test -run X ./... 2> "$off/err")
[[ $out == 'where=local'* ]] && grep -qx "heavy.sh: offload unavailable from this checkout: the source checkout $off/lc has no origin/main; run git -C $off/lc fetch origin" "$off/err" ||
  fail "a source without origin/main did not name its recovery: $out $(cat "$off/err")"
g -C "$off/lc" fetch -q origin || fail "fixture refetch failed"
g -C "$off/plain" checkout -q --orphan unrelated && g -C "$off/plain" commit -qm unrelated || fail "orphan commit failed"
out=$(HEAVY_REMOTE_HOST=box hvp sub go test -run X ./... 2> "$off/err")
[[ $out == 'where=local'* ]] && grep -q '^heavy.sh: offload unavailable from this checkout: HEAD shares no history with [0-9a-f]\{40\} (a pre-snapshot lineage?); base the branch on origin/main$' "$off/err" &&
  grep -q '^heavy.sh: offload to box unavailable; running locally (logged)$' "$off/err" ||
  fail "an unrelated lineage did not name why it ran locally: $out $(cat "$off/err")"
g -C "$off/plain" remote remove origin
out=$(HEAVY_REMOTE_HOST=box hvp sub go test -run X ./... 2> "$off/err")
[[ $out == 'where=local'* ]] && grep -qx 'heavy.sh: offload unavailable from this checkout: no origin remote' "$off/err" ||
  fail "a clone without origin did not name why it ran locally: $out $(cat "$off/err")"
mkdir -p "$off/nogit"
out=$(cd "$off/nogit" && env PATH="$off/bin:$PATH" TMPDIR="$off/ltmp" GIT_CEILING_DIRECTORIES="$off" HEAVY_REMOTE_SSH="$off/bin/ssh" HEAVY_SLOTS=1 \
  HEAVY_REMOTE_RETRY=0 HEAVY_FALLBACK_LOG="$off/fallback.log" HEAVY_REMOTE_HOST=box "$heavy" go test -run X ./... 2> "$off/err")
[[ $out == 'where=local'* ]] && grep -qx 'heavy.sh: offload unavailable from this checkout: not a Git checkout' "$off/err" ||
  fail "a directory outside Git did not name why it ran locally: $out $(cat "$off/err")"
# HEAVY_LOCAL_REASON rules are unchanged from a plain clone: HEAVY_REMOTE=0 still needs a reason.
HEAVY_REMOTE_HOST=box HEAVY_REMOTE=0 hvp sub go test -run X ./... > /dev/null 2> "$off/err"
[[ $? == 2 ]] || fail "HEAVY_REMOTE=0 without HEAVY_LOCAL_REASON ran from a plain clone: $(cat "$off/err")"
/bin/rm -rf "$off/plain" "$off/nogit"
# ASD-V0-038 (V1-2904): a local run uses the Makefile's cache unless the caller set GOCACHE.
out=$(unset GOCACHE CORVINT_GOCACHE; HEAVY_REMOTE_HOST= hv go test ./... 2> /dev/null)$'\n'$(unset GOCACHE
  CORVINT_GOCACHE=/x/c HEAVY_REMOTE_HOST= hv go test ./... 2> /dev/null)$'\n'$(GOCACHE=/y/c HEAVY_REMOTE_HOST= hv go test ./... 2> /dev/null)
[[ $(grep -o 'where=local dir=sub gocache=[^ ]*' <<< "$out" | tr '\n' ' ') == 'where=local dir=sub gocache=/tmp/corvint-go-build-cache where=local dir=sub gocache=/x/c where=local dir=sub gocache=/y/c ' ]] ||
  fail "heavy.sh local Go cache: $out"
ok
# ASD-V0-028: go vet and make windows-vet offload too; other commands, HEAVY_REMOTE=0, a missing host
# and a file-writing test run locally, and the offloadable ones among them are logged.
out=$(HEAVY_REMOTE_HOST=box hv go vet ./... 2> /dev/null)$'\n'$(HEAVY_REMOTE_HOST=box hv make windows-vet 2> /dev/null)
[[ $(grep -c '^where=remote dir=sub' <<< "$out") == 2 ]] || fail "heavy.sh did not offload go vet and make windows-vet: $out"
: > "$off/ssh.log"
: > "$off/fallback.log"
out=$(hv go test ./... 2> /dev/null)$(HEAVY_REMOTE_HOST=box HEAVY_REMOTE=0 HEAVY_LOCAL_REASON='darwin only' hv go test ./... 2> /dev/null)
out+=$(HEAVY_REMOTE_HOST=box HEAVY_BUILD_OFFLOAD=0 hv go build ./... 2> /dev/null)$(HEAVY_REMOTE_HOST=box hv go test -coverprofile=c.out ./... 2> /dev/null)
out+=$(HEAVY_REMOTE_HOST=box hv env 'GOFLAGS=-count=1 -coverprofile=c.out' go test ./... 2> /dev/null)
out+=$(HEAVY_REMOTE_HOST=box GOFLAGS=-cpuprofile=p.out hv go test ./... 2> /dev/null)
[[ ! -s $off/ssh.log && $out != *remote* ]] || fail "heavy.sh offloaded without a host, with HEAVY_REMOTE=0, HEAVY_BUILD_OFFLOAD=0 or a file-writing test"
[[ $(sed 's/.* reason=//' "$off/fallback.log" | tr '\n' ' ') == 'HEAVY_REMOTE=0 local_reason=darwin\ only cmd=go test ./... writes-files cmd=go test -coverprofile=c.out ./... writes-files cmd=env GOFLAGS=-count=1 -coverprofile=c.out go test ./... writes-files cmd=go test ./... ' ]] ||
  fail "heavy.sh fallback log: $(cat "$off/fallback.log")"
ok
# ASD-V0-028: a TERM to the wrapper during an offload stops its ssh at once and exits 143.
(cd "$off/lc/sub" && exec env PATH="$off/bin:$PATH" TMPDIR="$off/ltmp" HEAVY_REMOTE_SSH="$off/bin/ssh" \
  HEAVY_REMOTE_HOST=hang HEAVY_REMOTE_RETRY=0 HEAVY_FALLBACK_LOG="$off/fallback.log" "$heavy" go test ./...) 2> /dev/null &
wrapper=$!
wait_for "$off/hang.pid"
kill -TERM "$wrapper"
wait "$wrapper"
rc=$?
gone "$(cat "$off/hang.pid")" || { kill "$(cat "$off/hang.pid")" 2> /dev/null; fail "heavy.sh left ssh running after TERM"; }
[[ $rc == 143 && -z $(ls -A "$off/ltmp") ]] || fail "heavy.sh TERM during offload: exit $rc, scratch $(ls -A "$off/ltmp")"
ok
# ASD-V0-028: HEAVY_CONFIG supplies settings the environment leaves unset; the environment wins.
printf '# lane host\nHEAVY_REMOTE_HOST=box\nHEAVY_REMOTE_SHELL="bash -s"\nNOT_HEAVY=1\n' > "$off/heavy.env"
out=$(HEAVY_CONFIG=$off/heavy.env hv go test ./... 2> "$off/err")
[[ $out == where=remote* ]] && grep -q 'ignoring config line .*NOT_HEAVY=1' "$off/err" || fail "heavy.sh ignored HEAVY_CONFIG: $out"
out=$(HEAVY_CONFIG=$off/heavy.env HEAVY_REMOTE_HOST=down hv go test ./... 2> /dev/null)
[[ $out == where=local* ]] || fail "heavy.sh HEAVY_CONFIG overrode the environment: $out"
ok
# ASD-V0-028: the remote semaphore admits HEAVY_REMOTE_SLOTS=1 commands one at a time.
HEAVY_REMOTE_HOST=box HEAVY_REMOTE_SLOTS=1 hv go test -probe ./... > "$off/a.out" 2> "$off/a.err" &
a=$!
HEAVY_REMOTE_HOST=box HEAVY_REMOTE_SLOTS=1 hv go test -probe ./... > "$off/b.out" 2> "$off/b.err" &
b=$!
wait "$a" || fail "heavy.sh first remote command overlapped (exit $?)"
wait "$b" || fail "heavy.sh second remote command overlapped (exit $?)"
grep -q '^where=remote' "$off/a.out" && grep -q '^where=remote' "$off/b.out" || fail "heavy.sh slot test ran locally"
grep -qE '^heavy.sh: phase .* wait=[1-9]' "$off/a.err" "$off/b.err" || fail "heavy.sh: no remote command waited for the slot"
[[ -d $off/rhome/.cache/corvint-heavy/slots && -z $(ls -A "$off/rtmp") ]] || fail "heavy.sh remote slots or cleanup missing"
ok
# ASD-V0-028 (V1-2802): one small untracked file with no uncommitted diff still travels (its tar
# is the same 10240 bytes as an empty one), and is labelled +wip; with no untracked file and no
# diff, no untracked archive is sent and the run is not +wip.
payload() { sed -n '/^main "\$@"; heavy_exit \$?$/,$p' "$off/payload.last" | tail -n +2 | base64 -d | tar -tf - | sort | tr '\n' ' '; }
printf 'keep\n' > "$off/lc/sub/keep.txt"
g -C "$off/lc" add sub/keep.txt && g -C "$off/lc" commit -qm wip wip.txt sub/keep.txt || fail "offload fixture wip commit failed"
out=$(HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == *$'\n''untracked' ]] && grep -qE '^heavy.sh: running on .* \+wip tree=[0-9a-f]{40} dirty=1$' "$off/err" ||
  fail "heavy.sh dropped a lone small untracked file: $out $(cat "$off/err")"
[[ $(payload) == 'head.bundle meta.env untracked.tar wip.patch ' ]] || fail "heavy.sh payload with an untracked file: $(payload)"
/bin/mv -f "$off/lc/sub/untracked.txt" "$off/untracked.keep"
out=$(HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out != *untracked* ]] && grep -qE '^heavy.sh: running on .* at [0-9a-f]{40} tree=[0-9a-f]{40} dirty=0$' "$off/err" ||
  fail "heavy.sh clean tree offload: $out $(cat "$off/err")"
[[ $(payload) == 'head.bundle meta.env wip.patch ' ]] || fail "heavy.sh sent an untracked archive for no untracked files: $(payload)"
/bin/mv -f "$off/untracked.keep" "$off/lc/sub/untracked.txt"
# A remote that rebuilds a different tree (here its global excludes hide the untracked file)
# refuses to run, so the command runs locally; a local run names the tested tree too.
printf 'untracked.txt\n' > "$off/rhome/ignore"
g -C "$off/rhome/corvint" config --file "$off/rhome/.gitconfig" core.excludesFile "$off/rhome/ignore"
out=$(HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == where=local* ]] && grep -q 'differs from the local tree' "$off/err" &&
  grep -qE '^heavy.sh: running locally at [0-9a-f]{40} tree=[0-9a-f]{40} dirty=1$' "$off/err" ||
  fail "heavy.sh ran on a remote whose tree differs: $out $(cat "$off/err")"
/bin/rm -f "$off/rhome/.gitconfig" "$off/rhome/ignore"
# DCW-V0-359 (V1-2858): checkout files a remote go test writes (an -update golden) are copied back,
# deletions too, and the command's status returned, over a second ssh connection that a read-only
# run never opens; a path edited here during the run blocks the whole copy-back with exit 96.
conns() { wc -l < "$off/ssh.log" | tr -d ' '; }
n0=$(conns)
out=$(HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == where=remote* && $(conns) == $((n0 + 1)) ]] && ! grep -q 'copied back' "$off/err" ||
  fail "heavy.sh read-only offload opened $(($(conns) - n0)) ssh connections: $out $(cat "$off/err")"
n0=$(conns)
HEAVY_REMOTE_HOST=box hv go test -write=keep.txt -write=new/golden.txt -delete=untracked.txt -exit=3 ./... > /dev/null 2> "$off/err"
rc=$?
[[ $rc == 3 && $(cat "$off/lc/sub/keep.txt") == written-remote && $(cat "$off/lc/sub/new/golden.txt") == written-remote &&
  ! -e $off/lc/sub/untracked.txt && $(conns) == $((n0 + 2)) && -z $(ls -A "$off/rhome/.cache/corvint-heavy/out") ]] &&
  grep -q '^heavy.sh: copied back sub/keep.txt$' "$off/err" && grep -q '^heavy.sh: copied back sub/new/golden.txt$' "$off/err" &&
  grep -q '^heavy.sh: copied back the deletion of sub/untracked.txt$' "$off/err" ||
  fail "heavy.sh did not copy back the checkout files a remote go test wrote (exit $rc): $(cat "$off/err")"
g -C "$off/lc" checkout -q -- sub/keep.txt && /bin/rm -rf "$off/lc/sub/new" && printf 'untracked\n' > "$off/lc/sub/untracked.txt" ||
  fail "offload fixture restore failed"
HEAVY_REMOTE_HOST=box hv go test -write=new.txt -write=keep.txt -localedit=keep.txt ./... > /dev/null 2> "$off/err"
rc=$?
[[ $rc == 96 && $(cat "$off/lc/sub/keep.txt") == local-edit && ! -e $off/lc/sub/new.txt ]] &&
  grep -q 'changed here meanwhile; nothing was copied back: sub/keep.txt. Rerun locally: HEAVY_REMOTE=0 ' "$off/err" ||
  fail "heavy.sh copied back over a local edit made during the run (exit $rc): $(cat "$off/err")"
g -C "$off/lc" checkout -q -- sub/keep.txt || fail "offload fixture restore failed"
[[ -z $(hv go build ./... 2>&1 >/dev/null | grep 'running locally') ]] || fail "heavy.sh printed a tree for a non-offloadable command"
[[ -z $(g -C "$off/lc" status --porcelain --untracked-files=no) ]] || fail "heavy.sh changed the real index"
ok
# ASD-V0-040 (V1-2983): while an Actions run of the CI repository is queued or in progress, or the
# query fails, outlives HEAVY_CI_TIMEOUT or answers a non-number, an offloadable command runs
# locally without ssh and is logged; an idle CI offloads. HEAVY_CI_REPO, HEAVY_CI_STATUSES and
# HEAVY_CI_GH configure the query (from the environment or HEAVY_CONFIG); gh is a stub whose
# CORVINT_SELF_HOSTED variable reads STUB_GH_SELF_HOSTED (default on).
cat > "$off/bin/gh" << GH
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$off/gh.log"
case \${STUB_GH-} in fail) exit 1 ;; hang) exec sleep 30 ;; junk) echo busy; exit 0 ;; esac
case " \$* " in *" variable get "*) case \${STUB_GH_VAR-} in fail) exit 1 ;; hang) exec sleep 30 ;; esac; echo "\${STUB_GH_SELF_HOSTED:-on}" ;;
  *" --status queued "*) echo "\${STUB_GH_QUEUED:-0}" ;; *" --status in_progress "*) echo "\${STUB_GH_RUNNING:-0}" ;; *) echo 0 ;; esac
GH
chmod +x "$off/bin/gh" || fail "gh stub"
: > "$off/ssh.log"
: > "$off/fallback.log"
: > "$off/gh.log"
q='--limit 5 --json databaseId --jq length'
v='variable get CORVINT_SELF_HOSTED --repo beamfall/corvint'
out=$(HEAVY_CI_REPO=beamfall/corvint HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == where=remote* && $(cat "$off/gh.log") == "$v"$'\n'"run list --repo beamfall/corvint --status queued $q"$'\n'"run list --repo beamfall/corvint --status in_progress $q" ]] ||
  fail "heavy.sh did not offload under an idle CI after querying both statuses: $out $(cat "$off/err" "$off/gh.log")"
: > "$off/ssh.log"
for c in STUB_GH_RUNNING=1 STUB_GH_QUEUED=2 STUB_GH=fail STUB_GH=junk 'STUB_GH=hang HEAVY_CI_TIMEOUT=1' HEAVY_CI_GH=$off/no-such-gh; do
  # shellcheck disable=SC2086,SC2163 # each case is one or two assignments
  out=$(export $c; HEAVY_CI_REPO=beamfall/corvint HEAVY_REMOTE_HOST=box hv go vet ./... 2> "$off/err")
  [[ $out == where=local* ]] && grep -q '^heavy.sh: CI is running or unknown (repo=beamfall/corvint .*); not using box, running locally$' "$off/err" ||
    fail "heavy.sh offloaded while CI ran or its query failed ($c): $out $(cat "$off/err")"
done
[[ ! -s $off/ssh.log ]] || fail "heavy.sh opened ssh while CI ran: $(cat "$off/ssh.log")"
[[ $(sed 's/^[^ ]* pwd=[^ ]* reason=//' "$off/fallback.log" | tr '\n' '|') == 'ci-running repo=beamfall/corvint runs=1 cmd=go vet ./...|ci-running repo=beamfall/corvint runs=2 cmd=go vet ./...|'"$(printf 'ci-running repo=beamfall/corvint status=queued query-failed cmd=go vet ./...|%.0s' 1 2 3 4)" ]] ||
  fail "heavy.sh CI fallback log: $(cat "$off/fallback.log")"
[[ $(grep -cE '^[0-9TZ:-]+ ci-busy host=box repo=beamfall/corvint (runs=[12]|status=queued query-failed) pwd=.*/lc/sub cmd=go vet \./\.\.\. script=[0-9a-f]{12}$' "$tmp/selection.log") == 6 ]] ||
  fail "heavy.sh CI selection log: $(grep ci-busy "$tmp/selection.log")"
: > "$off/gh.log"
out=$(STUB_GH_RUNNING=1 HEAVY_REMOTE_HOST=box hv go test ./... 2> /dev/null)
[[ $out == where=remote* && ! -s $off/gh.log ]] || fail "heavy.sh queried CI with HEAVY_CI_REPO=off: $out $(cat "$off/gh.log")"
# The repository and statuses come from heavy.env as well; unset, the repository is a GitHub origin's.
printf 'HEAVY_CI_REPO=beamfall/corvint\nHEAVY_CI_STATUSES="waiting"\nHEAVY_CI_GUARD=on\n' > "$off/ci.env"
out=$(unset HEAVY_CI_REPO; STUB_GH_QUEUED=1 HEAVY_CONFIG=$off/ci.env HEAVY_REMOTE_HOST=box hv go test ./... 2> /dev/null)
[[ $out == where=remote* && $(cat "$off/gh.log") == "run list --repo beamfall/corvint --status waiting $q" ]] ||
  fail "heavy.sh ignored the heavy.env CI query: $out $(cat "$off/gh.log")"
: > "$off/gh.log"
g -C "$off/lc" remote set-url origin https://github.com/Beamfall/Corvint.git
out=$(unset HEAVY_CI_REPO; STUB_GH_RUNNING=1 HEAVY_REMOTE_HOST=box hv go test ./... 2> /dev/null)
g -C "$off/lc" remote set-url origin "$off/origin.git"
[[ $out == where=local* && $(sed -n 2p "$off/gh.log") == "run list --repo beamfall/corvint --status queued $q" ]] ||
  fail "heavy.sh did not guard on the GitHub origin by default: $out $(cat "$off/gh.log")"
for c in 'HEAVY_CI_STATUSES=bad;rm' 'HEAVY_CI_REPO=a b' HEAVY_CI_TIMEOUT=0 HEAVY_CI_GUARD=maybe; do
  # shellcheck disable=SC2163 # c is one NAME=VALUE assignment
  out=$(HEAVY_CI_REPO=beamfall/corvint; export "$c"; HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
  [[ $? == 2 ]] && grep -q '^heavy.sh: REFUSE invalid-ci-' "$off/err" || fail "heavy.sh accepted $c: $out $(cat "$off/err")"
done
ok
# ASD-V0-040 (V1-2983, hosted runners): under HEAVY_CI_GUARD=auto a CORVINT_SELF_HOSTED variable of
# off skips the run query and offloads while runs exist; any other answer, a failed or hung variable
# query and HEAVY_CI_GUARD=on keep the guard; HEAVY_CI_GUARD=off asks nothing.
: > "$off/ssh.log"
: > "$off/gh.log"
out=$(STUB_GH_RUNNING=1 STUB_GH_SELF_HOSTED=off HEAVY_CI_REPO=beamfall/corvint HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == where=remote* && $(cat "$off/gh.log") == "$v" ]] ||
  fail "heavy.sh guarded while CI ran on hosted runners: $out $(cat "$off/err" "$off/gh.log")"
: > "$off/gh.log"
out=$(STUB_GH_RUNNING=1 HEAVY_CI_GUARD=off HEAVY_CI_REPO=beamfall/corvint HEAVY_REMOTE_HOST=box hv go test ./... 2> "$off/err")
[[ $out == where=remote* && ! -s $off/gh.log ]] || fail "heavy.sh queried CI with HEAVY_CI_GUARD=off: $out $(cat "$off/gh.log")"
: > "$off/gh.log"
: > "$off/ssh.log"
for c in STUB_GH_SELF_HOSTED=on STUB_GH_SELF_HOSTED=OFF STUB_GH_VAR=fail 'STUB_GH_VAR=hang HEAVY_CI_TIMEOUT=1' 'HEAVY_CI_GUARD=on STUB_GH_SELF_HOSTED=off'; do
  # shellcheck disable=SC2086,SC2163 # each case is one or two assignments
  out=$(export $c; STUB_GH_RUNNING=1 HEAVY_CI_REPO=beamfall/corvint HEAVY_REMOTE_HOST=box hv go vet ./... 2> "$off/err")
  if [[ $out != where=local* ]] || ! grep -q '^heavy.sh: CI is running or unknown (repo=beamfall/corvint runs=1); not using box, running locally$' "$off/err"; then
    fail "heavy.sh dropped the CI guard ($c): $out $(cat "$off/err")"
  fi
done
[[ ! -s $off/ssh.log && $(grep -c '^variable get' "$off/gh.log") == 4 ]] ||
  fail "heavy.sh variable queries or ssh under the guard: $(cat "$off/gh.log" "$off/ssh.log")"
ok
# ASD-V0-041 (V1-2983): a build cross-compiles on a quiet remote for this host's GOOS/GOARCH in one
# try; an absolute output (-o P, -o=DIR/, NAME_BIN=P) is installed at P and a relative one at its
# checkout path, over an existing file; a failing build returns its status. A busy remote, running
# CI, a .. argument, a file-writing GOFLAGS or HEAVY_BUILD_OFFLOAD=0 builds locally.
: > "$off/ssh.log"
: > "$off/fallback.log"
mkdir -p "$off/out"
printf 'old\n' > "$off/out/abs-bin"
bv() { HEAVY_REMOTE_HOST=box HEAVY_REMOTE_BUILD_MAX_LOAD=${MAXL:-999999} hv "$@"; }
out=$(bv go build -o "$off/out/abs-bin" ./cmd 2> "$off/err")$'\n'$(bv go build -o="$off/out/dir/" ./cmd 2>> "$off/err")
out+=$'\n'$(bv make build CORVINT_BIN="$off/out/mk-bin" 2>> "$off/err")$'\n'$(bv go build -o bin/tool ./cmd 2>> "$off/err")
[[ $out == $'build where=remote\nbuild where=remote\nbuild where=remote\nbuild where=remote' ]] || fail "heavy.sh did not offload builds: $out $(cat "$off/err")"
for f in out/abs-bin out/dir/app out/mk-bin lc/sub/bin/tool; do
  [[ $(cat "$off/$f") == 'built-remote stubos/stubarch' ]] || fail "heavy.sh build output $f: $(cat "$off/$f") $(cat "$off/err")"
done
grep -q "^heavy.sh: copied back $off/out/abs-bin\$" "$off/err" && grep -q '^heavy.sh: copied back sub/bin/tool$' "$off/err" &&
  grep -q '^heavy.sh: building for stubos/stubarch$' "$off/err" && [[ $(conns) == 8 ]] ||
  fail "heavy.sh build copy-back: $(conns) ssh connections, $(cat "$off/err")"
/bin/rm -rf "$off/lc/sub/bin"
out=$(bv go build -exit=4 -o "$off/out/failed" ./cmd 2> "$off/err")
[[ $? == 4 && $out == 'build where=remote' && ! -e $off/out/failed ]] || fail "heavy.sh failed remote build: $out $(cat "$off/err")"
: > "$off/ssh.log"
printf 'load1() { printf 50; }\n' >> "$off/rhome/go.env"
out=$(MAXL=16 bv go build -o "$off/out/busy" ./cmd 2> "$off/err")
printf 'export FAKE_MARK=sourced\n' > "$off/rhome/go.env"
[[ $out == 'build where=local' && $(conns) == 1 && $(cat "$off/out/busy") == 'built-local /' ]] &&
  grep -q '^heavy.sh: remote .*: busy: load 50 above 16; building locally$' "$off/err" &&
  grep -q '^heavy.sh: building locally (remote busy or unavailable (rc=97))$' "$off/err" ||
  fail "heavy.sh built on a busy remote or retried: $out $(conns) $(cat "$off/err")"
out=$(STUB_GH_RUNNING=1 HEAVY_CI_REPO=beamfall/corvint bv go build ./cmd 2> "$off/err")$'\n'$(bv go build -o ../x ./cmd 2>> "$off/err")
out+=$'\n'$(GOFLAGS=-cpuprofile=p.out bv go build ./cmd 2>> "$off/err")$'\n'$(HEAVY_BUILD_OFFLOAD=0 bv go build ./cmd 2>> "$off/err")
/bin/rm -f "$off/lc/x"
[[ $out == $'build where=local\nbuild where=local\nbuild where=local\nbuild where=local' && $(conns) == 1 ]] ||
  fail "heavy.sh offloaded a build under CI, a .. argument, a writing GOFLAGS or HEAVY_BUILD_OFFLOAD=0: $out $(cat "$off/err")"
[[ $(sed 's/^[^ ]* pwd=[^ ]* reason=//' "$off/fallback.log" | tr '\n' '|') == "build-local remote busy or unavailable (rc=97) cmd=go build -o $off/out/busy ./cmd|ci-running repo=beamfall/corvint runs=1 cmd=go build ./cmd|build-local an argument has a .. component cmd=go build -o ../x ./cmd|build-local GOFLAGS writes files cmd=go build ./cmd|" ]] ||
  fail "heavy.sh build fallback log: $(cat "$off/fallback.log")"
grep -q " build-local host=box reason=remote\\\\ busy\\\\ or\\\\ unavailable\\\\ \\\\(rc=97\\\\) pwd=.*/lc/sub cmd=go build -o $off/out/busy ./cmd script=" "$tmp/selection.log" ||
  fail "heavy.sh build-local selection line: $(grep build-local "$tmp/selection.log")"
[[ -z $(g -C "$off/lc" status --porcelain --untracked-files=no) ]] || fail "heavy.sh build changed the real index"
ok
unset HEAVY_LOCK_DIR

# ASD-V0-043 (V1-2986): batch-review-packet.sh prints the reviewed revision, the paths changed by
# more than one first-parent merge of BASE..HEAD, and corvint impact and dogfood batch-check for the
# range, each bounded with its exit status; a timeout or missing corvint is named, HEAD must be the
# checkout, and the checkout, index and refs are unchanged. corvint is a stub.
rp=$tmp/review
mkdir -p "$rp/bin"
cat > "$rp/bin/corvint" << 'CV'
#!/usr/bin/env bash
[[ $1 == --version ]] && { echo 'Corvint stub'; exit 0; }
[[ ${STUB_CORVINT-} == hang ]] && exec sleep 30
printf 'args: %s\n' "$*"
[[ " $* " == *" impact "* ]] && printf 'impact line %s\n' 1 2 3 4
[[ " $* " == *" batch-check "* ]] && { echo 'obligation: unresolved'; exit 3; }
exit 0
CV
chmod +x "$rp/bin/corvint"
rg() { git -C "$rp/repo" -c user.name=t -c user.email=t@example.invalid "$@"; }
git init -q -b main "$rp/repo" && printf '%s\n' 1 2 3 4 5 6 > "$rp/repo/shared.txt" && rg add shared.txt && rg commit -qm base ||
  fail "packet fixture init"
rbase=$(rg rev-parse HEAD)
rg checkout -qb lane-a && printf 'a\n' > "$rp/repo/a.txt" && printf '%s\n' a 2 3 4 5 6 > "$rp/repo/shared.txt" && rg add -A && rg commit -qm a
rg checkout -q main && rg checkout -qb lane-b "$rbase" && printf 'b\n' > "$rp/repo/b.txt" && rg add -A && rg commit -qm b
printf '%s\n' 1 2 3 4 5 b > "$rp/repo/shared.txt" && rg commit -qam b2
rg checkout -q main && rg merge -q --no-ff -m 'merge a' lane-a && rg merge -q --no-ff -m 'merge b' lane-b > /dev/null ||
  fail "packet fixture merges"
rhead=$(rg rev-parse HEAD)
rstate() { rg status --porcelain --ignored; rg for-each-ref; rg rev-parse HEAD; rg ls-files -s | git hash-object --stdin; }
before=$(rstate)
packet() { (cd "$rp/repo" && CORVINT_BIN=$rp/bin/corvint "$script/batch-review-packet.sh" "$@" 2>&1); }
out=$(packet "$rbase")
rc=$?
m=$(rg rev-list --first-parent --merges "$rbase..$rhead" | cut -c1-12 | tr '\n' ' ')
[[ $rc == 0 && $out == *"reviewed: base=$rbase head=$rhead tree=$(rg rev-parse "$rhead^{tree}")"* &&
  $out == *'commits=5 first-parent-merges=2 corvint=Corvint stub'* ]] || fail "packet header: $out"
[[ $(sed -n '/^## Lane overlap/,/^$/p' <<< "$out" | sed 1d) == "shared.txt merges=2 ${m% }" ]] || fail "packet overlap: $out"
[[ $out == *"## corvint impact --base $rbase"$'\n'"args: --root "*" impact --base $rbase"$'\n''impact line 1'* ]] &&
  [[ $out == *$'impact line 4\nexit=0\n'* ]] || fail "packet impact section: $out"
[[ $out == *"args: --root "*" dogfood batch-check --verifier $rp/bin/corvint $rbase"$'\nobligation: unresolved\nexit=3' ]] ||
  fail "packet batch-check section: $out"
out=$(BATCH_REVIEW_LINES=2 packet "$rbase" HEAD)
[[ $out == *$'impact line 1\n... 3 more lines elided (BATCH_REVIEW_LINES=2)\nexit=0'* ]] || fail "packet bound: $out"
out=$(STUB_CORVINT=hang BATCH_REVIEW_TIMEOUT=1 packet "$rbase")
[[ $(grep -c '^exit=142 NOT_PRODUCED timeout after 1s' <<< "$out") == 2 ]] || fail "packet timeout: $out"
out=$(cd "$rp/repo" && CORVINT_BIN=no-such-corvint-stub "$script/batch-review-packet.sh" "$rbase" 2>&1)
[[ $out == *'corvint=NOT_PRODUCED'* && $(grep -c '^NOT_PRODUCED corvint-unavailable' <<< "$out") == 2 ]] || fail "packet without corvint: $out"
out=$(packet "$rbase" lane-a)
[[ $? == 2 && $out == *'REFUSE head-not-checked-out'* ]] || fail "packet accepted a HEAD other than the checkout: $out"
out=$(rg checkout -q lane-b; packet "$(rg rev-parse lane-a)"; rg checkout -q main)
[[ $out == *'REFUSE base-not-ancestor'* ]] || fail "packet accepted a base off the range: $out"
out=$(BATCH_REVIEW_LINES=0 packet "$rbase")
[[ $? == 2 && $out == *'REFUSE invalid-lines'* ]] || fail "packet accepted BATCH_REVIEW_LINES=0: $out"
[[ $(rstate) == "$before" ]] || fail "batch-review-packet.sh changed the checkout, index or refs"
ok

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
[[ $out != *propose-citations* ]] || fail "finish named propose-citations although CITATIONS was usable: $out"
# ASD-V0-045 (V1-3119): a missing plan names how to draft and pre-check it, and that nothing ran.
sha=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
out=$(finish 1 "$sha" '' plan.tsv plan.tsv passed)
[[ $out == *'REFUSE missing-inputs CITATIONS'$'\n'* && $out == *"corvint dogfood propose-citations $sha > plan.tsv"* &&
  $out == *"make dogfood-inputs-check BASE=$sha CITATIONS=plan.tsv"* && $out == *'nothing was built, prepared or bound'* ]] ||
  fail "finish refusal for a missing plan names no recovery: $out"
# ASD-V0-961 (V1-3394): the same refusal names the record-only recipe and its DOGFOOD.md section.
[[ $out == *'fix: a record-only change (only docs/build-log/ or .agent-evidence/ paths): put the entry path in the INTENTS file'*'relation decision'*'docs/DOGFOOD.md "Record-only closeout"'* ]] ||
  fail "finish refusal for a missing plan names no record-only recipe: $out"
grep -q '^Record-only closeout (`ASD-V0-961`' "$script/../docs/DOGFOOD.md" || fail "docs/DOGFOOD.md has no record-only closeout section"
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
  # A subshell scopes the shim: bash in POSIX mode (run as sh, as macOS /bin/sh is) keeps prefix
  # assignments to a function after it returns, so the failing git would reach the dry run below
  # (V1-2312).
  (PATH=$tmp/shim:$PATH LANE_TEST_REAL_GIT=$real_git LANE_TEST_FAIL_GIT=${sub%%:*}; export PATH LANE_TEST_REAL_GIT LANE_TEST_FAIL_GIT; clean > "$tmp/out")
  grep -q "^KEEP ${sub#*:} .*/merged\$" "$tmp/out" || fail "lane-clean with failing git ${sub%%:*}: $(cat "$tmp/out")"
done
[[ -z ${LANE_TEST_FAIL_GIT+x} && $(command -v git) == "$real_git" ]] || fail "lane-clean failing-git shim leaked past its case"
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

# ASD-V0-030: a whole-package go test needs HEAVY_WHOLE_REASON and every go test is logged with the
# prototype's fields; HEAVY_REMOTE=0 needs HEAVY_LOCAL_REASON; a local run waits out a high load.
pol=$tmp/policy
mkdir -p "$pol/bin" && printf '#!/bin/sh\nexit 0\n' > "$pol/bin/go" &&
  printf '#!/bin/sh\n[ "$2" = vm.loadavg ] && printf "{ %%s 1.00 1.00 }\\n" "$STUB_LOAD"\n' > "$pol/bin/sysctl" &&
  printf '#!/bin/sh\n[ -n "$STUB_CPUS" ] && { echo "$STUB_CPUS"; exit 0; }\nexec %s "$@"\n' "$(command -v getconf)" > "$pol/bin/getconf" &&
  chmod +x "$pol/bin/go" "$pol/bin/sysctl" "$pol/bin/getconf" || fail "policy fixture setup failed"
pv() { (cd "$pol" && env PATH="$pol/bin:$PATH" HEAVY_SELECTION_LOG="$pol/sel.log" HEAVY_FALLBACK_LOG="$pol/fb.log" \
  HEAVY_LOCK_DIR="$pol/slots" STUB_LOAD="${STUB_LOAD:-1.00}" HEAVY_SLOTS=1 "$heavy" "$@"); }
HEAVY_WHOLE_REASON= pv go test ./x 2> "$pol/err"
[[ $? == 2 ]] && grep -q '^heavy.sh: refused: whole-package go test without HEAVY_WHOLE_REASON' "$pol/err" ||
  fail "heavy.sh ran a whole-package go test without HEAVY_WHOLE_REASON"
HEAVY_WHOLE_REASON= pv env GOFLAGS=-count=1 go test -run 'TestA|TestB' ./x 2> /dev/null || fail "heavy.sh refused a -run go test"
HEAVY_WHOLE_REASON= pv go test -run=TestA ./x 2> /dev/null || fail "heavy.sh refused a -run= go test"
HEAVY_WHOLE_REASON='corvint FALLBACK x' pv go test ./x 2> /dev/null || fail "heavy.sh refused a go test with HEAVY_WHOLE_REASON"
HEAVY_WHOLE_REASON= pv go vet ./x 2> /dev/null || fail "heavy.sh applied the selection rule to go vet"
pv sh -c 'exit 3' 2> /dev/null
[[ $? == 3 ]] || fail "heavy.sh did not return a local command's status"
d='[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z'
pol=$(cd "$pol" && pwd)
h=' script=[0-9a-f]{12}$'
want=("^$d refused pwd=$pol cmd=go test \./x$h" "^$d selected pwd=$pol cmd=env GOFLAGS=-count=1 go test -run TestA\|TestB \./x$h"
  "^$d selected pwd=$pol cmd=go test -run=TestA \./x$h" "^$d whole reason=corvint\\\\ FALLBACK\\\\ x pwd=$pol cmd=go test \./x$h")
grep -v ' local wait=' "$pol/sel.log" > "$pol/sel.policy"
[[ $(wc -l < "$pol/sel.policy" | tr -d ' ') == 4 ]] || fail "heavy.sh selection log: $(cat "$pol/sel.log")"
for i in 0 1 2 3; do
  sed -n "$((i + 1))p" "$pol/sel.policy" | grep -Eq "${want[$i]}" || fail "heavy.sh selection log line $((i + 1)): $(sed -n "$((i + 1))p" "$pol/sel.policy")"
done
# ASD-V0-860: a -run that matches every test is a whole-package go test; a narrower one is selected.
for run in 'Test' '-run=.' '^Test.*' '.*/Sub' ''; do
  case $run in -run=*) runargs=("$run") ;; *) runargs=(-run "$run") ;; esac
  HEAVY_WHOLE_REASON= pv go test ${runargs[@]+"${runargs[@]}"} ./x > /dev/null 2> "$pol/err"
  [[ $? == 2 ]] && grep -q '^heavy.sh: refused: whole-package go test without HEAVY_WHOLE_REASON' "$pol/err" ||
    fail "heavy.sh logged go test ${runargs[*]} as a selection"
done
HEAVY_WHOLE_REASON= pv go test -run TestAll ./x 2> /dev/null || fail "heavy.sh refused go test -run TestAll"
grep -v ' local wait=' "$pol/sel.log" | tail -n 6 > "$pol/sel.all"
[[ $(grep -c ' refused pwd=' "$pol/sel.all") == 5 && $(tail -n 1 "$pol/sel.all") == *" selected pwd=$pol cmd=go test -run TestAll ./x script="* ]] ||
  fail "heavy.sh match-all selection log: $(cat "$pol/sel.all")"
# ASD-V0-037: every local run appends its slot wait, run wall time and status after it exits.
l="local wait=[0-9]+\.[0-9]s run=[0-9]+\.[0-9]s"
[[ $(grep -c ' local wait=' "$pol/sel.log") == 6 ]] &&
  grep -Eq "^$d $l rc=0 load=0s queue=[0-9]+\.[0-9]s pwd=$pol cmd=go vet \./x$h" "$pol/sel.log" &&
  grep -Eq "^$d $l rc=3 load=0s queue=[0-9]+\.[0-9]s pwd=$pol cmd=sh -c exit 3$h" "$pol/sel.log" || fail "heavy.sh local run lines: $(cat "$pol/sel.log")"
HEAVY_REMOTE=0 pv go vet ./x 2> "$pol/err"
[[ $? == 2 ]] && grep -q '^heavy.sh: refused: HEAVY_REMOTE=0 without HEAVY_LOCAL_REASON' "$pol/err" &&
  grep -Eq "^$d pwd=$pol reason=refused-local cmd=go vet \./x\$" "$pol/fb.log" ||
  fail "heavy.sh ran HEAVY_REMOTE=0 go vet without HEAVY_LOCAL_REASON"
# ASD-V0-060 (V1-3213): the documented lane test recipe (docs/LANE-ORCHESTRATION.md section 2) is
# `set -o pipefail; script/heavy.sh ... 2>&1 | tail -n N && git commit ...`. A refusing heavy.sh piped
# through tail must fail the step under it, the unprotected form passes (the hazard), and the
# one-line REFUSED marker reaches stdout so the tail shows it even without 2>&1.
rm -f "$pol/committed"
(set -o pipefail; HEAVY_REMOTE=0 pv go vet ./x 2>&1 | tail -n 5 > "$pol/tail.out" && : > "$pol/committed")
rc=$?
[[ $rc != 0 && ! -e $pol/committed ]] && grep -q '^heavy.sh: REFUSED local-reason-missing' "$pol/tail.out" ||
  fail "pipefail recipe did not fail on a refusing heavy.sh (rc=$rc): $(cat "$pol/tail.out")"
(set +o pipefail; HEAVY_REMOTE=0 pv go vet ./x 2> /dev/null | tail -n 5 > "$pol/tail.out" && : > "$pol/committed")
[[ $? == 0 && -e $pol/committed ]] && grep -q '^heavy.sh: REFUSED local-reason-missing' "$pol/tail.out" ||
  fail "unprotected pipe did not show the documented hazard or the stdout REFUSED marker"
rm -f "$pol/committed"
HEAVY_REMOTE=0 HEAVY_LOCAL_REASON='darwin syscall' pv go vet ./x 2> /dev/null || fail "heavy.sh refused HEAVY_LOCAL_REASON"
HEAVY_REMOTE=0 pv go build ./x 2> /dev/null || fail "heavy.sh applied HEAVY_LOCAL_REASON to go build"
s0=$SECONDS
STUB_LOAD=50.00 HEAVY_MAX_LOAD=10 HEAVY_LOAD_WAIT=1 pv true 2> "$pol/err" || fail "heavy.sh load wait failed"
# V1-3232: the wait stops at the 1-second bound instead of finishing a 5-second poll.
((SECONDS - s0 >= 1 && SECONDS - s0 <= 3)) && grep -q '^heavy.sh: waited [0-9]*s for host load 50 (HEAVY_MAX_LOAD=10)' "$pol/err" ||
  fail "heavy.sh did not wait out a high load: $(cat "$pol/err")"
STUB_LOAD=9.99 HEAVY_MAX_LOAD=9 pv true 2> "$pol/err" && ! grep -q waited "$pol/err" ||
  fail "heavy.sh waited at a load whose integer part is within HEAVY_MAX_LOAD"
HEAVY_MAX_LOAD=x pv true 2> "$pol/err"
[[ $? == 2 ]] && grep -q 'REFUSE invalid-max-load' "$pol/err" || fail "heavy.sh accepted HEAVY_MAX_LOAD=x"
# ASD-V0-932: unset HEAVY_MAX_LOAD, the threshold is floor(HEAVY_LOAD_PER_CPU x CPUs), default 1.0.
# ASD-V0-935: the load wait and the slot wait are reported apart from the run time.
HEAVY_MAX_LOAD= STUB_CPUS=12 STUB_LOAD=50.00 HEAVY_LOAD_PER_CPU=2 HEAVY_LOAD_WAIT=1 pv true 2> "$pol/err" &&
  grep -q '^heavy.sh: waited [0-9]*s for host load 50 (HEAVY_LOAD_PER_CPU=2 x 12 CPUs = 24)$' "$pol/err" &&
  grep -Eq '^heavy.sh: done queue=[0-9]+\.[0-9]s \(load=[1-9][0-9]*s slot=[0-9]+\.[0-9]s\) run=[0-9]+\.[0-9]s rc=0$' "$pol/err" &&
  tail -n 1 "$pol/sel.log" | grep -Eq "^$d $l rc=0 load=[1-9][0-9]*s queue=[0-9]+\.[0-9]s pwd=$pol cmd=true$h" ||
  fail "heavy.sh per-CPU load wait or its queue report: $(cat "$pol/err") $(tail -n 1 "$pol/sel.log")"
HEAVY_MAX_LOAD= STUB_CPUS=12 STUB_LOAD=20.00 HEAVY_LOAD_PER_CPU=2 pv true 2> "$pol/err" && ! grep -q waited "$pol/err" ||
  fail "heavy.sh waited at a load within HEAVY_LOAD_PER_CPU x CPUs"
HEAVY_MAX_LOAD= STUB_CPUS=12 STUB_LOAD=12.90 pv true 2> "$pol/err" && ! grep -q waited "$pol/err" ||
  fail "heavy.sh default threshold is not 1.0 x CPUs"
HEAVY_MAX_LOAD= STUB_CPUS=12 STUB_LOAD=13.00 HEAVY_LOAD_WAIT=1 pv true 2> "$pol/err" && grep -q '(HEAVY_LOAD_PER_CPU=1.0 x 12 CPUs = 12)' "$pol/err" ||
  fail "heavy.sh default per-CPU threshold did not wait above 12 on 12 CPUs: $(cat "$pol/err")"
STUB_CPUS=12 STUB_LOAD=13.00 HEAVY_MAX_LOAD=20 pv true 2> "$pol/err" && ! grep -q waited "$pol/err" || fail "HEAVY_MAX_LOAD did not win over the per-CPU default"
for bad in x 65 1.2345; do
  HEAVY_MAX_LOAD= HEAVY_LOAD_PER_CPU=$bad pv true 2> "$pol/err"
  [[ $? == 2 ]] && grep -q 'REFUSE invalid-load-per-cpu' "$pol/err" || fail "heavy.sh accepted HEAVY_LOAD_PER_CPU=$bad"
done
ok
# ASD-V0-931: the holder exports HEAVY_HELD; a heavy.sh inside an admitted command takes the
# holder's inner token (or a free slot) without waiting for the slot its own holder keeps, skips the
# load gate, and two nested commands never overlap on one token; a HEAVY_HELD of another lock dir
# is ignored.
got=$(pv sh -c 'printf "%s\n" "$HEAVY_HELD"' 2> /dev/null)
[[ $got == "$pol/slots/slot1" ]] || fail "heavy.sh did not export HEAVY_HELD: $got"
printf '#!/bin/sh\nmkdir "$1" || exit 9\nsleep 1\nrmdir "$1"\n' > "$pol/bin/excl" && chmod +x "$pol/bin/excl" || fail "excl stub"
export heavy
s0=$SECONDS
STUB_LOAD=50.00 HEAVY_MAX_LOAD=99 pv sh -c 'STUB_LOAD=50.00 HEAVY_MAX_LOAD=1 HEAVY_LOAD_WAIT=5 "$heavy" excl "$1/run" 2> "$1/n1.err" &
  STUB_LOAD=50.00 HEAVY_MAX_LOAD=1 HEAVY_LOAD_WAIT=5 "$heavy" excl "$1/run" 2> "$1/n2.err"; r=$?; wait $! && exit $r' _ "$pol" 2> "$pol/err"
rc=$?
((rc == 0 && SECONDS - s0 < 8)) && grep -q '^heavy.sh: slot 1.inner/1 after ' "$pol/n1.err" && grep -q '^heavy.sh: slot 1.inner/1 after ' "$pol/n2.err" &&
  ! grep -q waited "$pol/n1.err" "$pol/n2.err" ||
  fail "nested heavy.sh did not share the holder token without overlap or a load wait (rc=$rc, $((SECONDS - s0))s): $(cat "$pol/n1.err" "$pol/n2.err")"
pv sh -c 'sleep 2' 2> /dev/null &
holder=$!
sleep 0.5
HEAVY_HELD=/elsewhere/slot1 pv true 2> "$pol/err"
wait "$holder"
grep -Eq '^heavy.sh: slot 1/1 after [1-9]' "$pol/err" || fail "a foreign HEAVY_HELD skipped the host slot: $(cat "$pol/err")"
ok
# ASD-V0-930: the Makefile admits build, tasks-build, doc-gates, batch-precheck and dogfood-finish
# through HEAVY_ADMIT, script/heavy.sh by default and empty under CI.
adm() { (cd "$script/.." && env -u CI -u HEAVY_ADMIT "$@" make -s --no-print-directory --eval 'heavy-admit-show: ; @printf "[%s]\n" "$(HEAVY_ADMIT)"' heavy-admit-show); }
[[ $(adm) == '[script/heavy.sh --admit-only]' && $(adm CI=true) == '[]' && $(adm HEAVY_ADMIT=) == '[]' ]] || fail "HEAVY_ADMIT default: $(adm) $(adm CI=true)"
(cd "$script/.." && make -n --no-print-directory HEAVY_ADMIT=WRAP build tasks-build batch-precheck dogfood-finish 2> /dev/null) > "$tmp/admit.out"
(cd "$script/.." && make -n --no-print-directory HEAVY_ADMIT=echo doc-gates 2> /dev/null) >> "$tmp/admit.out"
[[ $(grep -c '^WRAP env GOCACHE=.* go build ' "$tmp/admit.out") == 2 ]] && grep -q '^WRAP script/batch-precheck.sh' "$tmp/admit.out" &&
  grep -q 'WRAP script/dogfood-finish.sh' "$tmp/admit.out" && grep -q '^make --no-print-directory --keep-going ' "$tmp/admit.out" &&
  grep -qx 'echo env GOTOOLCHAIN=local go run ./conformance/use-cases-v0' "$tmp/admit.out" ||
  fail "a heavy Makefile target is not admitted through HEAVY_ADMIT: $(cat "$tmp/admit.out")"
ok
# ASD-V0-936 (V1-1627): lane-pressure.sh classifies one sample by the ASD-V0-013 table, keeping the
# current level between thresholds; stub uname/getconf/sysctl and a procfs fixture supply the inputs.
lp=$tmp/pressure
mkdir -p "$lp/bin" "$lp/proc"
printf '#!/bin/sh\necho "${STUB_OS:-Linux}"\n' > "$lp/bin/uname"
printf '#!/bin/sh\necho 12\n' > "$lp/bin/getconf"
printf '#!/bin/sh\ncase $2 in vm.loadavg) echo "{ $STUB_LOAD 1.00 1.00 }" ;; hw.ncpu) echo 12 ;; kern.memorystatus_vm_pressure_level) echo "$STUB_MEM" ;; esac\n' > "$lp/bin/sysctl"
chmod +x "$lp/bin/uname" "$lp/bin/getconf" "$lp/bin/sysctl" || fail "pressure fixture"
lps() { # lps LOAD SWAPFREE_OF_100 [ARG...]
  printf '%s 1.00 1.00 1/100 1\n' "$1" > "$lp/proc/loadavg"
  printf 'SwapTotal: 100 kB\nSwapFree: %s kB\n' "$2" > "$lp/proc/meminfo"
  shift 2
  PATH=$lp/bin:$PATH LANE_PRESSURE_PROC=$lp/proc "$script/lane-pressure.sh" "$@"
}
[[ $(lps 6.00 100) == 'level=0 load_per_cpu=0.50 memory=- swap=0.00 admit=4' &&
  $(lps 18.00 100) == 'level=1 load_per_cpu=1.50 memory=- swap=0.00 admit=3' &&
  $(lps 60.00 100) == 'level=2 load_per_cpu=5.00 memory=- swap=0.00 admit=1' &&
  $(lps 6.00 10) == 'level=2 load_per_cpu=0.50 memory=- swap=0.90 admit=1' &&
  $(lps 14.40 100 --level 1) == 'level=1 load_per_cpu=1.20 memory=- swap=0.00 admit=3' &&
  $(lps 14.40 100 --level 2) == 'level=1 load_per_cpu=1.20 memory=- swap=0.00 admit=3' &&
  $(lps 24.00 100 --level 2) == 'level=2 load_per_cpu=2.00 memory=- swap=0.00 admit=1' &&
  $(lps 6.00 100 --level 2) == 'level=0 load_per_cpu=0.50 memory=- swap=0.00 admit=4' ]] ||
  fail "lane-pressure.sh levels: $(lps 14.40 100 --level 2) $(lps 24.00 100 --level 2)"
out=$(PATH=$lp/bin:$PATH STUB_OS=Darwin STUB_LOAD=6.00 STUB_MEM=4 "$script/lane-pressure.sh")
[[ $out == 'level=2 load_per_cpu=0.50 memory=4 swap=- admit=1' ]] || fail "lane-pressure.sh Darwin memory level: $out"
: > "$lp/proc/loadavg"
out=$(PATH=$lp/bin:$PATH LANE_PRESSURE_PROC=$lp/proc "$script/lane-pressure.sh" --level 1)
[[ $? == 3 && $out == 'level=1 load_per_cpu=unknown memory=- swap=0.00 admit=3 unreadable' ]] || fail "lane-pressure.sh unreadable sample: $out"
"$script/lane-pressure.sh" --level 3 > /dev/null 2>&1
[[ $? == 2 ]] || fail "lane-pressure.sh accepted --level 3"
ok
# ASD-V0-938 (V1-3630): a go test naming no timeout gets -timeout 30m after `test`; ASD-V0-939
# (V1-3628): a simple sh -c string is unwrapped, a compound one refused; ASD-V0-937: a local run
# below the floor warns and logs; ASD-V0-930: --admit-only reads no argument and a make dry run
# takes no slot; ASD-V0-931: two spellings of one lock dir are one queue; V1-3629: an apostrophe in
# the perl body is harmless; V1-3632: no lane script expands an array unguarded (bash 3.2, set -u).
to=$tmp/timeout
mkdir -p "$to/bin" && printf '#!/bin/sh\necho "args: $*"\n' > "$to/bin/go" &&
  printf '#!/bin/sh\nprintf "Filesystem 1024-blocks Used Available Capacity Mounted\\nfake 0 0 1048576 0%%%% /\\n"\n' > "$to/bin/df" &&
  printf '#!/bin/sh\nif [ "$1" = login ]; then echo "Logged in"; exit 0; fi\necho "reviewed:$*"\n' > "$to/bin/codex" &&
  chmod +x "$to/bin/go" "$to/bin/df" "$to/bin/codex" || fail "timeout fixture"
tv() { (cd "$to" && env PATH="$to/bin:$PATH" HEAVY_LOCK_DIR="${TLOCK:-$to/slots}" HEAVY_SLOTS=1 HEAVY_GO_TEST_TIMEOUT="${TO-30m}" "$heavy" "$@" 2> "$to/err"); }
[[ $(TO='' tv go test -run X ./x) == 'args: test -timeout 30m -run X ./x' &&
  $(tv env A=1 go test -run X ./x) == 'args: test -timeout 30m -run X ./x' &&
  $(tv go test -timeout 5m -run X ./x) == 'args: test -timeout 5m -run X ./x' &&
  $(tv go test -timeout=5m -run X ./x) == 'args: test -timeout=5m -run X ./x' &&
  $(GOFLAGS=-timeout=9m tv go test -run X ./x) == 'args: test -run X ./x' &&
  $(tv env GOFLAGS=-timeout=9m go test -run X ./x) == 'args: test -run X ./x' &&
  $(TO=off tv go test -run X ./x) == 'args: test -run X ./x' &&
  $(tv go vet ./x) == 'args: vet ./x' ]] || fail "heavy.sh go test timeout default: $(tv go test -run X ./x)"
TO=5 tv go test -run X ./x > /dev/null
[[ $? == 2 ]] && grep -q 'REFUSE invalid-go-test-timeout' "$to/err" || fail "heavy.sh accepted HEAVY_GO_TEST_TIMEOUT=5"
[[ $(tv sh -c 'go test -run X ./x') == 'args: test -timeout 30m -run X ./x' ]] && grep -qx 'heavy.sh: unwrapped sh -c: go test -run X ./x' "$to/err" &&
  [[ $(tv bash -c "GOTOOLCHAIN=local go test -run '^TestA\$' \"./x\"") == 'args: test -timeout 30m -run ^TestA$ ./x' ]] ||
  fail "heavy.sh did not unwrap a simple sh -c go test: $(cat "$to/err")"
for c in 'cd x && go test ./...' 'go test ./... | tail' 'go test $PKGS' 'go vet ./*'; do
  out=$(tv sh -c "$c")
  [[ $? == 2 && $out == 'heavy.sh: REFUSED wrapped-compound (exit 2)' ]] && grep -q 'run it directly, for example script/heavy.sh env K=V go test' "$to/err" ||
    fail "heavy.sh did not refuse the compound wrapper $c: $out $(cat "$to/err")"
done
tv sh -c 'echo go; exit 3' > /dev/null
[[ $? == 3 ]] || fail "heavy.sh changed a wrapper without go test"
HEAVY_MIN_FREE_GB=3 tv true > /dev/null && grep -qE "^heavy.sh: low disk here: .* free=1G floor=3G$" "$to/err" &&
  grep -qE '^[0-9TZ:-]+ lowdisk-local path=.* free=1G floor=3G pwd=' "$tmp/selection.log" || fail "heavy.sh local low-disk warning: $(cat "$to/err")"
out=$(tv --admit-only sh -c 'echo "go test ./x | tail"; exit 4')
[[ $? == 4 && $out == 'go test ./x | tail' ]] && grep -q '^heavy.sh: slot 1/1 ' "$to/err" || fail "heavy.sh --admit-only read its command: $out $(cat "$to/err")"
out=$(PATH=$to/bin:$PATH CODEX_BIN=$to/bin/codex CODEX_REVIEW_HEAVY=$heavy HEAVY_LOCK_DIR=$to/slots "$script/codex-review.sh" 'review the diff; run go test ./internal/x' 2> "$to/err")
[[ $out == 'reviewed:exec -m gpt-6.1-sol -s read-only review the diff; run go test ./internal/x' ]] && grep -q '^heavy.sh: slot 1/' "$to/err" ||
  fail "codex-review through heavy.sh misread its prompt: $out $(cat "$to/err")"
MAKEFLAGS='n --no-print-directory' tv --admit-only true && ! grep -q 'slot' "$to/err" || fail "a make dry run took a slot: $(cat "$to/err")"
MAKEFLAGS=' --no-print-directory' tv --admit-only true && grep -q 'slot 1/1' "$to/err" || fail "a long make flag read as a dry run"
TLOCK=$to/slots/ tv sh -c 'HEAVY_LOCK_DIR=$1 perl -e "alarm 10; exec @ARGV" "$heavy" true 2> "$1/../nested.err"' _ "$to/slots"
grep -q '^heavy.sh: slot 1.inner/1 ' "$to/nested.err" || fail "two spellings of one lock dir did not nest: $(cat "$to/nested.err")"
awk '/^IFS= read -r -d .. slot_perl << .PERL. \|\| :$/ { on = 1; print; next } on && /^use warnings;$/ { print; print "# the holder'"'"'s apostrophe"; next } { print }' \
  "$script/heavy.sh" > "$to/heavy-quote.sh"
grep -q "^# the holder's apostrophe$" "$to/heavy-quote.sh" && bash -n "$to/heavy-quote.sh" &&
  HEAVY_LOCK_DIR=$to/slots bash "$to/heavy-quote.sh" true 2> /dev/null || fail "an apostrophe in the slot_perl body breaks heavy.sh"
bad=$(grep -nE '(^|[^+])"\$\{[A-Za-z_][A-Za-z0-9_]*\[@\]\}"' "$script/heavy.sh" "$script"/lane-*.sh)
[[ -z $bad ]] || fail "unguarded array expansion (bash 3.2 set -u aborts on an empty one; use \${a[@]+\"\${a[@]}\"}): $bad"
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
# ASD-V0-024: a subagent (agent_id set) may not start background work: a shell-level background &,
# nohup/setsid/disown, run_in_background or a Monitor call. &&, redirections, quoted strings,
# comments and heredoc bodies are not background work; an unterminated quote does not hide a &.
# Without an agent_id none of this is checked.
sub_guard() { # sub_guard COMMAND [TOOL [RUN_IN_BACKGROUND]] -> the decision
  perl -MJSON::PP -e 'my %i = (command => $ARGV[1]); $i{run_in_background} = JSON::PP::true if $ARGV[3];
    print JSON::PP->new->encode({cwd => $ARGV[0], agent_id => "a1", tool_name => $ARGV[2], tool_input => \%i})' \
    "$tmp" "$1" "${2:-Bash}" "${3-}" | "$script/lane-guard.sh"
}
sub_denied() { [[ $(sub_guard "$@") == *'"permissionDecision":"deny"'*'no background work'* ]]; }
for cmd in \
  "make build &" \
  "make build & echo started" \
  "make build &"$'\n'"echo next" \
  "(cd x; make build) &" \
  "a && b & c" \
  "nohup make build > log 2>&1" \
  "setsid sh -c x" \
  "make build; disown" \
  "echo 'unterminated & still a command &" \
  "cat > a.txt <<EOF"$'\nplain\nEOF\n'"make build &" \
  'echo $((a & b)) &' \
  'echo $((a & b' \
  '((make build) & )' \
  '((cd x; make build) &)' \
  'echo $(( $(make build &) ))'; do
  sub_denied "$cmd" || fail "lane-guard allowed subagent background work: $cmd"
done
sub_denied "make build" Bash 1 || fail "lane-guard allowed run_in_background"
sub_denied "" Monitor || fail "lane-guard allowed a subagent Monitor call"
for cmd in \
  "make build && make test || echo failed" \
  "go vet ./x > log 2>&1; cat log >&2; ls &> /dev/null; ls &>> l; exec 3<&0; exec 4>&-" \
  "make build |& tee log" \
  "echo 'a & b' \"c && d & e\" \\& x" \
  "printf %s \$'tab\\t & here'" \
  "ls # run me & later" \
  "echo a#b" \
  "git log --grep='nohup'" \
  "cat > x_test.go <<'EOF'"$'\npackage x\n\nfunc f(p *T) bool { return p != nil && g(&p.v) }\n// it\'s & fine\nEOF\ngofmt -l x_test.go' \
  "cat > a.sh <<EOF"$'\nmake build &\nnohup x\nEOF' \
  'echo $((a & b)) && echo $(( (a & 0x0f) | c ))' \
  '(( x = y & 3 )) || echo no' \
  'n=$((n&1)); echo "$n"'; do
  out=$(sub_guard "$cmd")
  [[ -z $out ]] || fail "lane-guard refused a subagent command with no background work: $cmd -> $out"
done
[[ -z $(guard "$tmp" "make build &") ]] || fail "lane-guard checked background work without an agent_id"
ok
# ASD-V0-107 (V1-3224): a session lane (no agent_id) may not background a dogfood closeout,
# finish, check or seal step; the refusal names the foreground recipe. The same steps in the
# foreground, and other background work without an agent_id, stay allowed.
session_guard() { # session_guard COMMAND [RUN_IN_BACKGROUND] -> the decision
  perl -MJSON::PP -e 'my %i = (command => $ARGV[1]); $i{run_in_background} = JSON::PP::true if $ARGV[2];
    print JSON::PP->new->encode({cwd => $ARGV[0], tool_name => "Bash", tool_input => \%i})' "$tmp" "$1" "${2-}" | "$script/lane-guard.sh"
}
session_denied() { [[ $(session_guard "$@") == *'"permissionDecision":"deny"'*'run in the foreground'*'make dogfood-finish LANE=1'* ]]; }
for cmd in \
  "make dogfood-closeout BASE=x CITATIONS=c &" \
  "script/heavy.sh make dogfood-seal BASE=x > seal.log 2>&1 &" \
  "nohup make dogfood-finish LANE=1 BASE=x" \
  "make dogfood-check BASE=x & echo started" \
  "corvint dogfood finish-lane --base x &" \
  "script/dogfood-finish.sh x c i v passed & disown" \
  "(cd lane && make dogfood-seal BASE=x) &" \
  "make dogfood-seal BASE=x &> seal.log &"; do
  session_denied "$cmd" || fail "lane-guard allowed a background dogfood step: $cmd"
done
session_denied "make dogfood-seal BASE=x" 1 || fail "lane-guard allowed run_in_background for dogfood-seal"
for cmd in \
  "script/heavy.sh make dogfood-finish LANE=1 BASE=x CITATIONS=c INTENTS=i VERIFY=v OUTCOME=passed" \
  "make dogfood-seal BASE=x > seal.log 2>&1" \
  "make dogfood-check BASE=x && git log -1" \
  "make dogfood-change BASE=x &" \
  "make dogfood-checks-report &" \
  "git log --grep='dogfood-seal' &" \
  "make dogfood-check BASE=x; sleep 1 & wait" \
  "tail -n 30 /tmp/dogfood-finish.log & wait" \
  "cat > notes.md <<EOF"$'\nmake dogfood-seal &\nEOF'; do
  out=$(session_guard "$cmd")
  [[ -z $out ]] || fail "lane-guard refused a foreground dogfood step or unrelated background work: $cmd -> $out"
done
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

# ASD-V0-039 (V1-2888): wsl-keepalive-check.sh reports the distro's Windows-side holders from one
# read-only query; no holder fails, a holder without the running task warns, an unreachable host
# is its own status. A stub ssh prints the query's answer.
ka=$tmp/keepalive && mkdir -p "$ka/bin" || fail "cannot create the keepalive fixture"
cat > "$ka/bin/ssh" << 'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$KA_LOG"
[[ -n ${KA_RC-} ]] && exit "$KA_RC"
printf '%b' "$KA_OUT"
STUB
chmod +x "$ka/bin/ssh"
kac() { PATH=$ka/bin:$PATH KA_LOG=$ka/ssh.log "$script/wsl-keepalive-check.sh" --host win@box "$@"; }
out=$(KA_OUT='holder pid=36924 parent=2700 name=wsl.exe\r\ntask name=CorvintWslKeepAlive state=Running last-result=267009\r\nend\r\n' kac); rc=$?
[[ $rc == 0 && $out == *$'holder pid=36924 parent=2700 name=wsl.exe\n'*'wsl-keepalive-check: OK holders=1 host=win@box distro=Ubuntu' && $out != *WARN* ]] ||
  fail "keepalive check with a task-owned holder: rc=$rc out=$out"
grep -q -- '-EncodedCommand ' "$ka/ssh.log" || fail "keepalive check did not send an encoded query: $(cat "$ka/ssh.log")"
out=$(KA_OUT='task name=CorvintWslKeepAlive state=Ready last-result=1\nend\n' kac); rc=$?
[[ $rc == 1 && $out == *'FAIL no-holder host=win@box distro=Ubuntu'* ]] || fail "keepalive check without a holder: rc=$rc out=$out"
out=$(KA_OUT='holder pid=1 parent=2 name=wsl.exe\ntask name=CorvintWslKeepAlive state=missing\nend\n' kac); rc=$?
[[ $rc == 0 && $out == *'WARN task-not-running host=win@box task=CorvintWslKeepAlive'* && $out == *'OK holders=1'* ]] ||
  fail "keepalive check with an untasked holder: rc=$rc out=$out"
out=$(KA_RC=255 kac 2>&1); rc=$?
[[ $rc == 3 && $out == *'FAIL host-unreachable host=win@box rc=255'* ]] || fail "keepalive check of a down host: rc=$rc out=$out"
out=$(KA_OUT='holder pid=1 parent=2 name=wsl.exe\n' kac 2>&1); rc=$?
[[ $rc == 3 ]] || fail "keepalive check accepted a truncated answer: rc=$rc out=$out"
out=$(KA_OUT='query-error\n' kac 2>&1); rc=$?
[[ $rc == 3 && $out == *'FAIL query-error host=win@box'* && $out != *no-holder* ]] ||
  fail "keepalive check read a query error as no holder: rc=$rc out=$out"
kac --distro 'bad;name' 2> /dev/null; [[ $? == 2 ]] || fail "keepalive check accepted an invalid distro"
kac --bogus 2> /dev/null; [[ $? == 2 ]] || fail "keepalive check accepted an unknown argument"
ok

# ASD-V0-074 (V1-3120): a whole-package go test of plain tools/ or conformance/ directories whose
# *_test.go files total at most HEAVY_SMALL_TEST_BYTES runs with no HEAVY_WHOLE_REASON and is logged
# as small-package; a larger total, an internal/ package, `...`, an import path, -C or a zero
# limit is still refused.
sp=$tmp/small-pkg
mkdir -p "$sp/bin" "$sp/tools/small" "$sp/tools/big" "$sp/conformance/c" "$sp/internal/x" || fail "small-package fixture"
printf '#!/bin/sh\nexit 0\n' > "$sp/bin/go" && chmod +x "$sp/bin/go"
head -c 100 /dev/zero | tr '\0' x > "$sp/tools/small/a_test.go"
head -c 50 /dev/zero | tr '\0' x > "$sp/conformance/c/a_test.go"
head -c 300 /dev/zero | tr '\0' x > "$sp/tools/big/a_test.go"
cp "$sp/tools/small/a_test.go" "$sp/internal/x/a_test.go"
spv() { (cd "$sp" && env -u HEAVY_WHOLE_REASON PATH="$sp/bin:$PATH" HEAVY_SELECTION_LOG="$sp/sel.log" HEAVY_LOCK_DIR="$sp/slots" \
  HEAVY_SLOTS=1 HEAVY_SMALL_TEST_BYTES="${limit:-200}" "$heavy" "$@") 2> "$sp/err"; }
spv env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/small ./conformance/c || fail "a small tools package was refused: $(cat "$sp/err")"
grep -Eq ' whole reason=small-package:150B pwd=.* cmd=env GOTOOLCHAIN=local go test -count=1 -timeout 30m \./tools/small \./conformance/c script=' "$sp/sel.log" ||
  fail "small-package not logged: $(cat "$sp/sel.log")"
for args in './tools/big' './tools/small ./tools/big' './internal/x' './tools/...' 'example.invalid/tools/small' '-C tools ./small'; do
  # shellcheck disable=SC2086 # each case is a word list on purpose
  spv go test $args
  [[ $? == 2 ]] && grep -q 'without HEAVY_WHOLE_REASON' "$sp/err" || fail "heavy.sh ran whole go test $args without a reason: $(cat "$sp/err")"
done
limit=0 spv go test ./tools/small
[[ $? == 2 ]] || fail "HEAVY_SMALL_TEST_BYTES=0 did not disable the exemption"
ok

# ASD-V0-072 (V1-3230), ASD-V0-071 (V1-3227): lane-setup.sh refuses a shallow shared clone with the
# `git fetch --unshallow` recovery and creates nothing; on the full clone it adds the lane worktree
# on its branch at origin/main and installs the commit-msg hook that refuses an AI trailer.
ls_src=$tmp/lane-setup-src
git init -q "$ls_src" && git -C "$ls_src" -c user.name=t -c user.email=t@t commit -q --allow-empty -m one &&
  git -C "$ls_src" -c user.name=t -c user.email=t@t commit -q --allow-empty -m two || fail "lane-setup source"
git clone -q --depth 1 "file://$ls_src" "$tmp/lane-setup-shallow" 2> /dev/null || fail "lane-setup shallow clone"
out=$("$script/lane-setup.sh" --repo "$tmp/lane-setup-shallow" --dir "$tmp/lane-a" --branch claude/a 2>&1)
rc=$?
[[ $rc == 2 && $out == *'lane-setup: REFUSE shallow-repository'*'fetch --unshallow origin'* ]] || fail "shallow lane setup (rc $rc): $out"
[[ ! -e $tmp/lane-a ]] || fail "a refused lane setup created its directory"
git clone -q "file://$ls_src" "$tmp/lane-setup-full" 2> /dev/null || fail "lane-setup full clone"
out=$("$script/lane-setup.sh" --repo "$tmp/lane-setup-full" --dir "$tmp/lane-a" --branch claude/a 2>&1) || fail "lane setup: $out"
[[ $(git -C "$tmp/lane-a" rev-parse --abbrev-ref HEAD) == claude/a && $(git -C "$tmp/lane-a" rev-parse HEAD) == $(git -C "$ls_src" rev-parse HEAD) ]] ||
  fail "lane worktree not on claude/a at origin/main: $out"
printf 'feat: x\n\n%s\n' "$(printf 'Co-Authored-By: Claude <noreply@%s>' anthropic.com)" > "$tmp/lane-a-msg"
git -C "$tmp/lane-a" -c user.name=t -c user.email=t@t commit -q --allow-empty -F "$tmp/lane-a-msg" 2> /dev/null &&
  fail "the lane's commit-msg hook let an AI trailer through"
out=$("$script/lane-setup.sh" --repo "$tmp/lane-setup-full" --dir "$tmp/lane-a" --branch claude/b 2>&1)
[[ $? == 2 && $out == *'dir-exists'* ]] || fail "lane setup over an existing dir: $out"
# Relative --repo and --dir, run from a directory outside the repository, name the same lane
# directory for git and for the hook install (both resolve against the caller's directory).
rm -f "$(git -C "$tmp/lane-setup-full" rev-parse --path-format=absolute --git-path hooks/commit-msg)"
mkdir -p "$tmp/lane-caller"
out=$(cd "$tmp/lane-caller" && "$script/lane-setup.sh" --repo ../lane-setup-full --dir lanes/b --branch claude/b 2>&1) || fail "relative lane setup: $out"
[[ $(git -C "$tmp/lane-caller/lanes/b" rev-parse --abbrev-ref HEAD) == claude/b ]] || fail "relative lane dir not created under the caller: $out"
[[ -x $(git -C "$tmp/lane-setup-full" rev-parse --path-format=absolute --git-path hooks/commit-msg) ]] || fail "relative lane setup installed no hook: $out"
git -C "$tmp/lane-caller/lanes/b" -c user.name=t -c user.email=t@t commit -q --allow-empty -F "$tmp/lane-a-msg" 2> /dev/null &&
  fail "the relative lane's hook let an AI trailer through"
ok

# ASD-V0-064 (V1-3219): the Codex review preflight reports NOT_AVAILABLE once, with the login fix.
out=$("$script/codex-review_test.sh" 2>&1) || fail "codex-review_test.sh: $out"
ok

# ASD-V0-086 (V1-3242): the dogfood wrapper fixtures share one copy step that follows the copied
# scripts' source/run references, so a new helper a wrapper sources reaches every fixture unedited.
fsrc=$tmp/fixture-src fdst=$tmp/fixture-dst
mkdir -p "$fsrc/script" "$fdst"
printf '#!/usr/bin/env bash\nsource "$repo/script/new-helper.sh" || exit 2\n# script/only-in-comment.sh\n' > "$fsrc/script/dogfood-wrap.sh"
printf '#!/usr/bin/env bash\n"$repo/script/deep-helper.sh" "$@"\n' > "$fsrc/script/new-helper.sh"
printf '#!/usr/bin/env bash\n# script/new-helper.sh is mentioned again; the copy is single\n' > "$fsrc/script/deep-helper.sh"
# shellcheck source=script/dogfood-fixture-scripts.sh
source "$script/dogfood-fixture-scripts.sh"
dogfood_fixture_copy "$fsrc" "$fdst" dogfood-wrap.sh || fail "fixture copy refused a complete closure"
[[ $(cd "$fdst" && /bin/ls | tr '\n' ' ') == 'deep-helper.sh dogfood-wrap.sh new-helper.sh ' ]] ||
  fail "fixture copy did not follow the sourced helper: $(/bin/ls "$fdst" | tr '\n' ' ')"
# The real wrappers: every fixture named by a wrapper gets the build helper it sources.
real=$tmp/fixture-real
mkdir -p "$real"
dogfood_fixture_copy "$script/.." "$real" dogfood-change.sh dogfood-check.sh dogfood-bind-range.sh ||
  fail "fixture copy refused the real dogfood wrappers"
[[ -f $real/dogfood-build-corvint.sh ]] || fail "the real wrapper closure lacks dogfood-build-corvint.sh"
printf 'source "$repo/script/missing-helper.sh"\n' > "$fsrc/script/dogfood-broken.sh"
out=$(dogfood_fixture_copy "$fsrc" "$fdst" dogfood-broken.sh 2>&1) && fail "a missing helper was copied silently"
[[ $out == *'REFUSE missing-script script/missing-helper.sh'*'fix: '* ]] || fail "missing-helper refusal names no recovery: $out"
ok

# ASD-V0-960 (V1-3472): requirement-ids-free reports each used ID with its first location and exits
# 1, a free range with exit 0, and refuses a bad prefix, range or REF with exit 2.
ids=$tmp/ids
git init -q "$ids" && printf '# Spec\n- `ABC-V0-101`: one.\nXABC-V0-102 is another family; ABC-V0-1030 has four digits.\n' > "$ids/spec.md" &&
  printf 'see ABC-V0-101 and ABC-V0-105\n' > "$ids/b.txt" && git -C "$ids" add . &&
  git -C "$ids" -c user.name=t -c user.email=t@example.invalid commit -qm ids || fail "ids fixture"
printf 'ABC-V0-104\n' > "$ids/uncommitted.txt"
out=$(cd "$ids" && "$script/requirement-ids-free.sh" ABC-V0 100 105 HEAD 2> "$tmp/ids.err")
[[ $? == 1 && $out == $'used ABC-V0-101 b.txt:1\nused ABC-V0-105 b.txt:1' && $(cat "$tmp/ids.err") == *'2 of 6 IDs in ABC-V0-100..ABC-V0-105 are used at HEAD'* ]] ||
  fail "used IDs: $out $(cat "$tmp/ids.err")"
# Review fixes: a run from a subdirectory still searches the whole tree, and a match whose leading
# byte is a colon (`:ABC-V0-103`) keeps its path and line.
mkdir -p "$ids/sub/deeper" && printf 'key:ABC-V0-103\n' > "$ids/sub/colon.txt" && git -C "$ids" add sub/colon.txt &&
  git -C "$ids" -c user.name=t -c user.email=t@example.invalid commit -qm colon || fail "colon fixture"
out=$(cd "$ids/sub/deeper" && "$script/requirement-ids-free.sh" ABC-V0 101 103 HEAD 2> /dev/null)
[[ $? == 1 && $out == $'used ABC-V0-101 b.txt:1\nused ABC-V0-103 sub/colon.txt:1' ]] || fail "subdirectory or colon-prefixed match: $out"
git -C "$ids" rm -q sub/colon.txt && git -C "$ids" -c user.name=t -c user.email=t@example.invalid commit -qm uncolon || fail "colon fixture removal"
out=$(cd "$ids" && "$script/requirement-ids-free.sh" ABC-V0 102 104 HEAD 2>&1)
[[ $? == 0 && $out == "free ABC-V0-102..ABC-V0-104 at HEAD ($(git -C "$ids" rev-parse HEAD))" ]] || fail "free range: $out"
out=$(cd "$ids" && "$script/requirement-ids-free.sh" ZZZ-V1 1 9 HEAD 2>&1)
[[ $? == 0 && $out == 'free ZZZ-V1-001..ZZZ-V1-009 at HEAD ('* ]] || fail "unknown prefix: $out"
# ASD-V0-962 (V1-3472): with no range the prefix is a new family; one REQUIREMENTS.tsv names refuses
# it and names the next free prefix, skipping a family REQUIREMENTS.tsv or any tracked file uses.
mkdir -p "$ids/docs/specs" && printf 'ABC-V0-101\tdocs/specs/abc.md\tone\nABD-V0-001\tdocs/specs/abd.md\ttwo\n' > "$ids/docs/specs/REQUIREMENTS.tsv" &&
  printf 'ABE-V0-001 is named only here\n' > "$ids/notes.txt" && git -C "$ids" add docs notes.txt &&
  git -C "$ids" -c user.name=t -c user.email=t@example.invalid commit -qm index || fail "prefix fixture"
out=$(cd "$ids" && "$script/requirement-ids-free.sh" ABC-V0 HEAD 2>&1)
[[ $? == 1 && $out == 'used-prefix ABC-V0: 1 requirement rows in docs/specs/REQUIREMENTS.tsv at HEAD, first ABC-V0-101 in docs/specs/abc.md; next free prefix: ABF-V0' ]] ||
  fail "in-use prefix: $out"
out=$(cd "$ids" && "$script/requirement-ids-free.sh" ABX-V0 HEAD 2>&1)
[[ $? == 0 && $out == 'free-prefix ABX-V0 at HEAD ('* ]] || fail "free prefix: $out"
# Review fix: the make target refuses any RANGE but <digits>..<digits>.
for range in ..979 1..2..3 960.. abc; do
  out=$(make -s -C "$script/.." requirement-ids-free PREFIX=ABC-V0 RANGE="$range" 2>&1)
  [[ $? == 2 && $out == *"REFUSE bad-range RANGE=$range"* ]] || fail "make target admitted RANGE=$range: $out"
done
for bad in 'abc 1 2 HEAD:bad-prefix' 'ABC-V0 5 2 HEAD:bad-range' 'ABC-V0 1 1000 HEAD:bad-range' 'ABC-V0 1 2 no-such-ref:ref-not-a-commit' 'ABC-V0 1 2 3 4:bad-arguments'; do
  read -r -a argv <<< "${bad%:*}"
  out=$(cd "$ids" && "$script/requirement-ids-free.sh" ${argv[@]+"${argv[@]}"} 2>&1)
  [[ $? == 2 && $out == *"REFUSE ${bad##*:}"* ]] || fail "refusal ${bad##*:}: $out"
done
ok

# ASD-V0-014: no fixture outlives the run, so nothing can write after the summary line. Every
# background job is reaped and no heavy.sh slot the fixtures used (local, remote stub, worktree) is
# still held by a live process.
wait
for lock in "$tmp"/slots/slot* "$pol"/slots/slot* "$off"/rhome/.cache/corvint-heavy/slots/slot* \
  "$off"/rhome/.cache/corvint-heavy/wt/slot*.lock; do
  [[ -e $lock ]] || fail "fixture slot $lock was never created"
  busy "$lock" && fail "a heavy.sh fixture outlived the test holding $lock (pid $(cat "$lock"))"
done
[[ ! -s $heavy_escapes ]] || fail "a heavy.sh fixture ran with a lock dir outside the test directory"
ok

summary
