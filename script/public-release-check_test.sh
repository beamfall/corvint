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
[ -z "${CORVINT_WRAPPER_ARGS:-}" ] || printf '%s\n' "$@" >"$CORVINT_WRAPPER_ARGS"
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
refuse() { local label=$1 want=$2; shift 2; if env "${common[@]}" "$@" "$corvint/script/public-release-check" 2>"$test_root/refusal.err"; then fail "$label accepted"; fi; grep -Fq "$want" "$test_root/refusal.err" || fail "$label: $(cat "$test_root/refusal.err")"; ls "$test_root/out" | grep -q '^refused' && fail "$label retained result"; return 0; }
authority="$test_root/authority.json"; printf '{}' >"$authority"; chmod 600 "$authority"
sum=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
core=(CORVINT_PUBLIC_RELEASE_QUALIFICATION=core CORVINT_NODE_SHA256=$sum CORVINT_NPM_SHA256=$sum CORVINT_GO_AUTHORITY_BUNDLE="$authority" CORVINT_GO_AUTHORITY_SHA256=$sum)
refuse "unknown qualification" "must be editor or core" CORVINT_PUBLIC_RELEASE_QUALIFICATION=full CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-q1" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q1.json"
refuse "empty qualification" "must not be empty" CORVINT_PUBLIC_RELEASE_QUALIFICATION= CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-q2" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q2.json"
refuse "editor with core digest" "editor qualification refuses CORVINT_NODE_SHA256" CORVINT_NODE_SHA256=$sum CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-q3" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q3.json"
refuse "explicit editor with authority" "editor qualification refuses CORVINT_GO_AUTHORITY_BUNDLE" CORVINT_PUBLIC_RELEASE_QUALIFICATION=editor CORVINT_GO_AUTHORITY_BUNDLE="$authority" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-q4" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q4.json"
for name in CORVINT_NODE_SHA256 CORVINT_NPM_SHA256 CORVINT_GO_AUTHORITY_BUNDLE CORVINT_GO_AUTHORITY_SHA256; do
  refuse "core without $name" "core qualification needs $name" "${core[@]}" "$name=" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-$name" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-$name.json"
done
refuse "relative authority" "must be absolute" "${core[@]}" CORVINT_GO_AUTHORITY_BUNDLE=authority.json CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-q5" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q5.json"
refuse "directory authority" "must be a regular file" "${core[@]}" CORVINT_GO_AUTHORITY_BUNDLE="$test_root/bundle" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-q6" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q6.json"
refuse "authority as scratch" "output overlaps Go authority" "${core[@]}" CORVINT_PUBLIC_RELEASE_SCRATCH="$authority" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/refused-q7.json"
for name in scratch-q1 scratch-q2 scratch-q3 scratch-q4 scratch-q5 scratch-q6; do [ ! -e "$test_root/$name" ] || fail "refusal created $name"; done
env "${common[@]}" CORVINT_WRAPPER_ARGS="$test_root/editor.args" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-editor" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/editor.json" "$corvint/script/public-release-check"
[ "$(sed -n 1,2p "$test_root/editor.args" | tr '\n' ' ')" = "-qualification editor " ] || fail "editor selection not passed through"
grep -q -- '-node-sha256\|-go-authority' "$test_root/editor.args" && fail "editor received core arguments"
env "${common[@]}" "${core[@]}" CORVINT_WRAPPER_ARGS="$test_root/core.args" CORVINT_PUBLIC_RELEASE_SCRATCH="$test_root/scratch-core" CORVINT_PUBLIC_RELEASE_RESULT="$test_root/out/core.json" "$corvint/script/public-release-check"
[ -s "$test_root/out/core.json" ] || fail "core success omitted result"
[ "$(sed -n 1,10p "$test_root/core.args" | tr '\n' ' ')" = "-qualification core -node-sha256 $sum -npm-sha256 $sum -go-authority-bundle $(cd "$test_root" && pwd -P)/authority.json -go-authority-sha256 $sum " ] || fail "core settings not passed through: $(cat "$test_root/core.args")"
printf 'public release check wrapper: identity, qualification selection, failure and interruption cleanup PASS\n'
