#!/usr/bin/env bash
# make core-n1-replay without CORE_N1_TAG takes script/select-previous-release.sh's tag, pre-release
# tags included, and reports NOT_APPLICABLE FIRST_RELEASE when no release tag precedes HEAD
# (V1-3203, CCF-V1-007). go is a shim: no N-1 binary is built.
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-core-n1-replay.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
mkdir "$scratch/repo" "$scratch/bin" "$scratch/repo/script"
cp "$source_root/Makefile" "$scratch/repo/"
cp "$source_root/script/select-previous-release.sh" "$scratch/repo/script/"
# go shim: `go build -o P` writes P; every call is logged with the working directory.
cat >"$scratch/bin/go" <<SHIM
#!/bin/sh
printf '%s %s\n' "\$(pwd)" "\$*" >>'$scratch/go-invoked'
if [ "\$1" = build ]; then
  while [ \$# -gt 0 ]; do [ "\$1" = -o ] && { : >"\$2"; break; }; shift; done
fi
SHIM
chmod +x "$scratch/bin/go"
export PATH="$scratch/bin:$PATH"
unset CORE_N1_TAG
cd "$scratch/repo"
git init -q -b main
printf 'first\n' >marker
git add -A && git commit -q -m first
first=$(git rev-parse HEAD)
case_name=
run() {
  local want=$1; shift
  local code=0
  rm -f "$scratch/go-invoked"
  make --no-print-directory core-n1-replay GO_TEST_COMMAND='go test' "$@" >"$scratch/out" 2>"$scratch/err" || code=$?
  if [ "$code" -ne "$want" ]; then
    printf 'core-n1-replay-default_test: case %s: exit %s, want %s\n--- stdout\n' "$case_name" "$code" "$want" >&2
    cat "$scratch/out" >&2; printf -- '--- stderr\n' >&2; cat "$scratch/err" >&2; exit 1
  fi
}
fail() { printf 'core-n1-replay-default_test: case %s: %s\n' "$case_name" "$*" >&2; cat "$scratch/out" "$scratch/err" >&2; exit 1; }

case_name='no tag precedes HEAD'
run 0
grep -Fqx 'core-n1-replay: NOT_APPLICABLE FIRST_RELEASE: no release tag precedes HEAD; the N-1 checks are not applicable' "$scratch/out" || fail 'no NOT_APPLICABLE line'
test ! -e "$scratch/go-invoked" || fail 'go ran'

case_name='a tag on HEAD itself is not its predecessor'
git tag v1.0.0-rc.3
run 0
grep -Fq 'NOT_APPLICABLE FIRST_RELEASE' "$scratch/out" || fail 'no NOT_APPLICABLE line'

case_name='an explicit empty CORE_N1_TAG is refused'
run 2 CORE_N1_TAG=
grep -Fqx 'CORE_N1_TAG is required' "$scratch/err" || fail 'no refusal'
test ! -e "$scratch/go-invoked" || fail 'go ran'

case_name='the pre-release predecessor is the default'
git tag v0.8.1 "$first"
printf 'second\n' >marker
git add -A && git commit -q -m second
run 0
grep -Fqx 'core-n1-replay: CORE_N1_TAG=v1.0.0-rc.3, the newest release tag before HEAD' "$scratch/err" || fail 'rc.3 not selected'
grep -q ' build -trimpath -o .*/corvint ./cmd/corvint$' "$scratch/go-invoked" || fail 'N-1 not built'
grep -q " test -run ^(TestCoreVerbsEmitTheFrozenProfiles|TestCoreRefusalsKeepTheFrozenEnvelope)\$ ./cmd/corvint\$" "$scratch/go-invoked" ||
  fail 'replay tests not run'

case_name='a final release outranks its pre-release on the same commit'
git tag v1.0.0 v1.0.0-rc.3
run 0
grep -Fqx 'core-n1-replay: CORE_N1_TAG=v1.0.0, the newest release tag before HEAD' "$scratch/err" || fail 'v1.0.0 not selected over v1.0.0-rc.3'
git tag -d v1.0.0 >/dev/null

case_name='an explicit tag wins'
run 0 CORE_N1_TAG=v0.8.1
grep -Fq 'newest release tag' "$scratch/err" && fail 'selector ran'
grep -q ' build ' "$scratch/go-invoked" || fail 'N-1 not built'

case_name='a selector refusal fails closed'
git tag -d v1.0.0-rc.3 >/dev/null
git tag v2.0.0,unsafe "$first"
run 2
grep -Fq 'core-n1-replay: previous release selection refused (exit 2); pass CORE_N1_TAG=<tag>' "$scratch/err" || fail 'no refusal line'
test ! -e "$scratch/go-invoked" || fail 'go ran'

echo 'core-n1-replay-default_test: ok'
