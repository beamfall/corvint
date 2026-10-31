#!/bin/sh
# core-n1-source.sh: N-1 is a published release, FIRST_RELEASE is explicit, and a predecessor that
# exists but cannot be selected or verified fails closed (V1-3177).
# POSIX sh: per-call variables go through run's NAME=value arguments and env(1), never a prefix
# assignment on a function call, which persists after the call under POSIX sh (V1-3375).
set -eu
source_root=$(cd "$(dirname "$0")/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-core-n1-source.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
mkdir "$scratch/repo" "$scratch/bin" "$scratch/repo/script"
cp "$source_root/script/core-n1-source.sh" "$source_root/script/select-previous-release.sh" "$scratch/repo/script/"
cd "$scratch/repo"
git init -q -b main
git commit -q --allow-empty -m base
base=$(git rev-parse HEAD)
git commit -q --allow-empty -m candidate
candidate=$(git rev-parse HEAD)
tree=$(git rev-parse 'HEAD^{tree}')
future=$(printf 'future\n' | git commit-tree "$tree" -p "$candidate")
other=$(printf 'archived\n' | git commit-tree "$tree")
# gh shim: GH_VIEW is `release view` stdout (unset fails), GH_LIST is `release list` stdout
# (GH_LIST_FAIL fails); every call is logged.
cat >"$scratch/bin/gh" <<SHIM
#!/bin/sh
printf '%s\n' "\$*" >>'$scratch/gh-invoked'
case "\$1 \$2" in
  'release view') [ -n "\${GH_VIEW-}" ] || exit 1; printf '%s\n' "\$GH_VIEW" ;;
  'release list') [ -z "\${GH_LIST_FAIL-}" ] || exit 1; [ -z "\${GH_LIST-}" ] || printf '%s\n' "\$GH_LIST" ;;
  *) exit 9 ;;
esac
SHIM
cat >"$scratch/bin/make" <<SHIM
#!/bin/sh
printf '%s\n' "\$*" >'$scratch/make-invoked'
SHIM
chmod +x "$scratch/bin/gh" "$scratch/bin/make"
export PATH="$scratch/bin:$PATH" GITHUB_REPOSITORY=beamfall/corvint SHA="$candidate"
export PREVIOUS_RELEASE_TAG='' PREVIOUS_RELEASE_OID='' GITHUB_STEP_SUMMARY="$scratch/summary"
nl='
'
# run WANT [NAME=value...] [ARG...]: the leading NAME=value words are exported only in a subshell.
run() {
  want=$1; shift
  code=0
  rm -f "$scratch/gh-invoked" "$scratch/make-invoked" "$scratch/summary"
  (
    while case ${1-} in [A-Z_]*=*) true ;; *) false ;; esac; do export "$1"; shift; done
    exec script/core-n1-source.sh "$@"
  ) >"$scratch/out" 2>"$scratch/err" </dev/null || code=$?
  if [ "$code" -ne "$want" ]; then
    echo "exit $code, want $want" >&2; cat "$scratch/err" >&2; exit 1
  fi
}
refused() { run 2 "$@"; test ! -s "$scratch/out"; test ! -e "$scratch/make-invoked"; }

# No tag precedes the candidate and nothing is published: FIRST_RELEASE, exit 3, stdout empty.
run 3
test ! -s "$scratch/out"
grep -Fq "core-n1-source: FIRST_RELEASE: no published release of beamfall/corvint precedes $candidate" "$scratch/err"
grep -Fq -- "- N-1 FIRST_RELEASE: no published release of beamfall/corvint precedes $candidate" "$scratch/summary"
grep -Fq 'release list --repo beamfall/corvint --exclude-drafts' "$scratch/gh-invoked"
# --replay reports it as not applicable, visibly, and runs no replay.
run 0 --replay
grep -Fqx "::notice title=core-n1-replay::FIRST_RELEASE: no published release of beamfall/corvint precedes $candidate; the N-1 checks are not applicable" "$scratch/out"
grep -Fq 'core-n1-replay: NOT_APPLICABLE FIRST_RELEASE' "$scratch/out"
test ! -e "$scratch/make-invoked"
# A published release at the candidate itself or a descendant is not a predecessor.
git tag v1.0.0-rc.3 "$candidate"
git tag v1.0.0 "$future"
run 3 GH_LIST="v1.0.0${nl}v1.0.0-rc.3"
# A published release that is not a reachable predecessor, or has no local tag, fails closed.
git tag v0.9.0 "$other"
refused GH_LIST="v1.0.0${nl}v0.9.0"
grep -Fq 'published release v0.9.0 of beamfall/corvint is not a reachable predecessor' "$scratch/err"
refused GH_LIST=v0.8.1
grep -Fq 'published release v0.8.1 of beamfall/corvint has no local tag' "$scratch/err"
refused GH_LIST_FAIL=1
grep -Fq 'cannot list published releases of beamfall/corvint' "$scratch/err"
refused GH_LIST_FAIL=1 --replay
# A reachable tag must be a published, non-draft release of this repository.
git tag v0.8.1 "$base"
run 0 SHA="$future" GH_VIEW='v1.0.0-rc.3 false'
test "$(cat "$scratch/out")" = v1.0.0-rc.3
grep -Fq 'core-n1-source: N-1 v1.0.0-rc.3 is a published release of beamfall/corvint' "$scratch/err"
run 0 SHA="$future" GH_VIEW='v1.0.0-rc.3 false' --replay
test "$(cat "$scratch/make-invoked")" = 'core-n1-replay CORE_N1_TAG=v1.0.0-rc.3'
refused SHA="$future" GH_VIEW='v1.0.0-rc.3 true' --replay
grep -Fq 'N-1 tag v1.0.0-rc.3 is not a published release in beamfall/corvint' "$scratch/err"
refused SHA="$future" --replay
grep -Fq 'N-1 tag v1.0.0-rc.3 has no readable published release in beamfall/corvint' "$scratch/err"
# The explicit predecessor inputs are honoured, and a refused selection never reaches gh.
run 0 SHA="$future" PREVIOUS_RELEASE_TAG=v0.8.1 PREVIOUS_RELEASE_OID="$base" GH_VIEW='v0.8.1 false'
test "$(cat "$scratch/out")" = v0.8.1
refused SHA="$future" PREVIOUS_RELEASE_TAG=v0.8.1 PREVIOUS_RELEASE_OID="$candidate"
grep -Fq 'previous release selection refused (exit 2)' "$scratch/err"
test ! -e "$scratch/gh-invoked"
refused SHA=abc
test ! -e "$scratch/gh-invoked"
refused GITHUB_REPOSITORY='beamfall/corvint;x'
refused --bogus
refused --replay extra
printf 'core-n1 source selection: PASS\n'
