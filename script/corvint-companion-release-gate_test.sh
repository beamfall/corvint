#!/usr/bin/env bash
set -euo pipefail

test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-companion-wrapper-test.XXXXXX")
cleanup() { chmod -R u+w "$test_root" 2>/dev/null || :; rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
base="$test_root/corvint-base"
corvint="$test_root/corvint"
fake="$test_root/fake"
mkdir -p "$base" "$fake" "$test_root/npm-cache" "$test_root/output"

git -C "$base" init -q
git -C "$base" config user.name test
git -C "$base" config user.email test@example.invalid
touch "$base/a"
git -C "$base" add a
git -C "$base" commit -qm corvint
git -C "$base" worktree add -q "$corvint" -b test-linked
test -f "$corvint/.git" || fail "fixture is not a linked worktree"
mkdir "$corvint/script"
cp "$source_root/script/corvint-companion-release-gate" "$corvint/script/"

real_git=$(command -v git)
cat >"$fake/go" <<'EOF'
#!/bin/sh
if [ "$1" = env ] && [ "$2" = GOVERSION ]; then printf '%s\n' go1.27.3; exit 0; fi
if [ "$1" = run ]; then
  [ -z "${CORVINT_WRAPPER_IGNORE_TERM:-}" ] || trap '' TERM
  printf '%s\n' "$@" >"$CORVINT_WRAPPER_ARGS"
  if [ -n "${CORVINT_WRAPPER_DESC_PID:-}" ]; then
    perl -MPOSIX -e 'select undef,undef,undef,0.2; POSIX::setsid(); exec "/bin/sleep", "60"' &
    child=$!; printf '%s\n' "$child" >"$CORVINT_WRAPPER_DESC_PID"
    if [ -n "${CORVINT_WRAPPER_FAIL:-}" ]; then sleep 0.4; exit 7; fi
    wait "$child"
  fi
  exit 0
fi
exit 90
EOF
cat >"$fake/git" <<EOF
#!/bin/sh
exec "$real_git" "\$@"
EOF
cat >"$fake/npm" <<'EOF'
#!/bin/sh
printf invoked >"$CORVINT_NPM_MARKER"
exit 91
EOF
chmod +x "$fake/go" "$fake/git" "$fake/npm"

CORVINT_WRAPPER_ARGS="$test_root/args" CORVINT_NPM_CACHE="$test_root/npm-cache" \
CORVINT_COMPANION_OUTPUT="$test_root/output" PATH="$fake:/usr/bin:/bin" \
  "$corvint/script/corvint-companion-release-gate"
grep -qx -- '-source-root' "$test_root/args" || fail "linked worktree source root did not reach release command"
! grep -q -- '-tasks-root' "$test_root/args" || fail "retired tasks root reached release command"
if CORVINT_WRAPPER_ARGS="$test_root/extra-args" CORVINT_COMPANION_OUTPUT="$test_root/output-extra" PATH="$fake:/usr/bin:/bin" \
  "$corvint/script/corvint-companion-release-gate" "$test_root" 2>/dev/null; then
  fail "retired taskman-repo argument was accepted"
fi
[ ! -e "$test_root/extra-args" ] || fail "retired taskman-repo argument reached release command"
grep -q -- '-npm-cache' "$test_root/args" || fail "npm cache was not passed"

CORVINT_WRAPPER_ARGS="$test_root/no-npm-args" CORVINT_NPM_MARKER="$test_root/npm-invoked" \
CORVINT_COMPANION_OUTPUT="$test_root/output-no-npm" PATH="$fake:/usr/bin:/bin" \
  env -u CORVINT_NPM_CACHE "$corvint/script/corvint-companion-release-gate"
[ ! -e "$test_root/npm-invoked" ] || fail "core bundle resolved npm"


# Every ps here costs 50 ms more, as on a loaded macOS runner: the TERM grace must stay bounded by
# the clock rather than by its poll count, or the join below exceeds 8 s (V1-4553).
slow="$test_root/slow-ps"
mkdir -p "$slow"
printf '#!/bin/sh\nsleep 0.05\nexec "%s" "$@"\n' "$(command -v ps)" >"$slow/ps"
chmod +x "$slow/ps"
CORVINT_WRAPPER_ARGS="$test_root/interrupt-args" CORVINT_WRAPPER_DESC_PID="$test_root/descendant" CORVINT_WRAPPER_IGNORE_TERM=1 \
CORVINT_NPM_CACHE="$test_root/npm-cache" CORVINT_COMPANION_OUTPUT="$test_root/output-interrupt" \
PATH="$slow:$fake:/usr/bin:/bin" "$corvint/script/corvint-companion-release-gate" &
wrapper=$!
for _ in $(seq 1 100); do [ -s "$test_root/descendant" ] && break; sleep 0.05; done
[ -s "$test_root/descendant" ] || fail "interrupt descendant did not start"
descendant=$(cat "$test_root/descendant")
sleep 0.4
[ "$(ps -p "$descendant" -o pgid= | tr -d ' ')" = "$descendant" ] || fail "fixture descendant did not detach"
started=$SECONDS
kill -TERM "$wrapper"
wait "$wrapper" 2>/dev/null && fail "interrupted wrapper succeeded"
joined=$((SECONDS-started))
[ "$joined" -le 8 ] || fail "TERM-ignoring runner cleanup exceeded bounded join: ${joined} s"
kill -0 "$descendant" 2>/dev/null && fail "interrupted wrapper left descendant alive"

if CORVINT_WRAPPER_ARGS="$test_root/fail-args" CORVINT_WRAPPER_DESC_PID="$test_root/fail-descendant" CORVINT_WRAPPER_FAIL=1 \
  CORVINT_NPM_CACHE="$test_root/npm-cache" CORVINT_COMPANION_OUTPUT="$test_root/output-fail" \
  PATH="$fake:/usr/bin:/bin" "$corvint/script/corvint-companion-release-gate"; then
  fail "failed runner was reported successful"
fi
failed_descendant=$(cat "$test_root/fail-descendant")
kill -0 "$failed_descendant" 2>/dev/null && fail "failed runner left detached descendant alive"
printf 'corvint companion wrapper test: linked worktree admitted\n'

/bin/bash "$source_root/script/release-env-compat_test.sh"
