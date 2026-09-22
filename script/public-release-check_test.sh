#!/usr/bin/env bash
set -euo pipefail
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-public-release-check-test.XXXXXX")
cleanup() { [ -z "${active:-}" ] || kill -KILL "$active" 2>/dev/null || :; chmod -R u+w "$test_root" 2>/dev/null || :; rm -rf "$test_root"; }
trap cleanup EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }
source_root=$(cd "$(dirname "$0")/.." && pwd -P)
corvint="$test_root/corvint"; fake="$test_root/fake"; mkdir -p "$corvint/script" "$fake" "$test_root/bundle" "$test_root/npm" "$test_root/browsers" "$test_root/out"
cp "$source_root/script/public-release-check" "$corvint/script/"
git -C "$corvint" init -q; git -C "$corvint" config user.name test; git -C "$corvint" config user.email test@example.invalid
git -C "$corvint" add .; git -C "$corvint" commit -qm fixture
commit=$(git -C "$corvint" rev-parse HEAD); tree=$(git -C "$corvint" rev-parse 'HEAD^{tree}')
cat >"$fake/go" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = env ]; then echo go1.27.1; exit 0; fi
if [ "$1" = build ]; then
  if [ -n "${CORVINT_WRAPPER_BUILD_FAIL:-}" ]; then
    perl -MPOSIX -e 'select undef,undef,undef,0.2; POSIX::setsid(); exec "/bin/sleep", "60"' &
    child=$!; echo "$child" >"$CORVINT_WRAPPER_BUILD_DESC_PID"; sleep 0.5; exit 7
  fi
  while [ "$1" != -o ]; do shift; done; out=$2
  cat >"$out" <<'INNER'
#!/usr/bin/env bash
set -eu
[ -z "${CORVINT_WRAPPER_IGNORE_TERM:-}" ] || trap '' TERM
output=
while [ "$#" -gt 0 ]; do [ "$1" != -output ] || { output=$2; shift; }; shift; done
if [ -n "${CORVINT_WRAPPER_DESC_PID:-}" ]; then
  perl -MPOSIX -e 'select undef,undef,undef,0.2; POSIX::setsid(); exec "/bin/sleep", "60"' &
  child=$!; echo "$child" >"$CORVINT_WRAPPER_DESC_PID"
  [ -z "${CORVINT_WRAPPER_FAIL:-}" ] || { sleep 0.5; exit 7; }
  wait "$child"
fi
[ -z "$output" ] || echo '{"status":"PASS"}' >"$output"
INNER
  chmod +x "$out"; exit 0
fi
if [ "$1" = version ] && [ "$2" = -m ]; then
  printf '%s\n' "$2: go1.27.1" "build vcs.revision=$CORVINT_RELEASE_COMMIT" "build vcs.modified=false"; exit 0
fi
exit 90
EOF
chmod +x "$fake/go"
common=(CORVINT_RELEASE_BUNDLE_DIR="$test_root/bundle" CORVINT_NPM_CACHE="$test_root/npm" CORVINT_PLAYWRIGHT_BROWSERS="$test_root/browsers" CORVINT_NODE=/bin/sh CORVINT_PYTHON=/bin/sh CORVINT_PYTHON_SHA256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa CORVINT_RELEASE_COMMIT="$commit" CORVINT_RELEASE_TREE="$tree" PATH="$fake:/usr/bin:/bin")
env "${common[@]}" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-ok" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/result.json" "$corvint/script/public-release-check"
[ -s "$test_root/out/result.json" ] || fail "success omitted result"
if env "${common[@]}" CORVINT_WRAPPER_BUILD_FAIL=1 CORVINT_WRAPPER_BUILD_DESC_PID="$test_root/build-fail.pid" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-build-fail" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/build-fail.json" "$corvint/script/public-release-check"; then fail "failed build succeeded"; fi
build_failed=$(cat "$test_root/build-fail.pid"); kill -0 "$build_failed" 2>/dev/null && fail "build failure left detached child"; [ ! -e "$test_root/out/build-fail.json" ] || fail "build failure retained result"
foreign="$test_root/out/foreign.json"
(sleep 0.2; echo foreign >"$foreign") & foreign_writer=$!
if env "${common[@]}" CORVINT_WRAPPER_BUILD_FAIL=1 CORVINT_WRAPPER_BUILD_DESC_PID="$test_root/build-race.pid" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-build-race" CORVINT_PUBLIC_RELEASE_RESULT="$foreign" "$corvint/script/public-release-check"; then fail "racing failed build succeeded"; fi
wait "$foreign_writer"; [ "$(cat "$foreign")" = foreign ] || fail "failure deleted foreign result"
if env "${common[@]}" CORVINT_WRAPPER_DESC_PID="$test_root/fail.pid" CORVINT_WRAPPER_FAIL=1 CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-fail" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/fail.json" "$corvint/script/public-release-check"; then fail "failed checker succeeded"; fi
failed=$(cat "$test_root/fail.pid"); kill -0 "$failed" 2>/dev/null && fail "normal failure left detached child"; [ ! -e "$test_root/out/fail.json" ] || fail "failure retained result"
env "${common[@]}" CORVINT_WRAPPER_DESC_PID="$test_root/int.pid" CORVINT_WRAPPER_IGNORE_TERM=1 CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-int" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/int.json" "$corvint/script/public-release-check" &
active=$!
for _ in $(seq 1 100); do [ -s "$test_root/int.pid" ] && break; sleep 0.05; done
[ -s "$test_root/int.pid" ] || fail "interrupt fixture did not start"
descendant=$(cat "$test_root/int.pid"); started=$SECONDS; kill -TERM "$active"; wait "$active" 2>/dev/null && fail "interrupted checker succeeded"; active=
[ "$((SECONDS-started))" -le 8 ] || fail "TERM-ignoring runner cleanup exceeded bounded join"
kill -0 "$descendant" 2>/dev/null && fail "interruption left detached child"; [ ! -e "$test_root/out/int.json" ] || fail "interruption retained result"
if env "${common[@]}" CORVINT_PUBLIC_RELEASE_SCRATCH="$corvint/scratch" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/overlap.json" "$corvint/script/public-release-check"; then fail "source-overlapping scratch accepted"; fi
printf 'public release check wrapper: identity, failure and interruption cleanup PASS\n'
