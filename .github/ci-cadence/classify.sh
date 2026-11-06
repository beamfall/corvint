#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# CIN-V0-4750..4756 (V1-4750): routine-change cadence classifier.
#
# Decides whether the slow advisory campaigns of one pull request or merge-queue entry (the native
# archive gate, the selection-baseline replay inside doc-gates, the macOS shards) may be DEFERred to
# the nightly and release gates, or must run now (FULL). Only the copy from the trusted event base
# (the pull request's base commit or the queue base) runs; the tested tree is read through git only.
#
# DEFER needs every changed path to be one of two reviewed classes, each added or modified as a
# regular 100644 blob (no rename, copy, delete, type or mode change):
#   DOCS   docs/build-log/<name>.md, the one-file-per-entry build log (decision 0423);
#   TESTS  an ordinary Go test file under cmd/, internal/ or tools/, outside the archive, selection,
#          conformance and release subtrees, with no platform file suffix, build constraint,
#          embed, cgo or TestMain.
# Anything else, and any missing, malformed or unverifiable input, prints FULL with its reason and
# still exits zero, so a classifier failure can only add work. Usage errors exit 2.
#
# Usage: classify.sh --root DIR --event pull_request|merge_group --base SHA --head SHA --target SHA
# Output: fixed key=value lines: profile, cadence (DEFER|FULL), class, base, head, target, helper,
# reason. CI_CADENCE_FORCE=FULL (repository variable) forces FULL: the rollback switch.
set -u
export LC_ALL=C GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_NO_REPLACE_OBJECTS=1 GIT_OPTIONAL_LOCKS=0

root='' event='' base='' head='' target=''
while [ $# -gt 0 ]; do
  case "$1" in
    --root | --event | --base | --head | --target)
      [ $# -ge 2 ] || { echo "classify.sh: $1 needs a value" >&2; exit 2; }
      v=$2
      case "$1" in
        --root) root=$v ;; --event) event=$v ;; --base) base=$v ;; --head) head=$v ;; --target) target=$v ;;
      esac
      shift 2 ;;
    *) echo "classify.sh: unknown argument $1" >&2; exit 2 ;;
  esac
done

self=${BASH_SOURCE[0]}
if command -v sha256sum > /dev/null 2>&1; then
  helper=$(sha256sum < "$self" | cut -d' ' -f1)
else
  helper=$(shasum -a 256 < "$self" | cut -d' ' -f1)
fi
class=NONE

emit() {
  printf 'profile=corvint-ci-cadence/0\ncadence=%s\nclass=%s\nbase=%s\nhead=%s\ntarget=%s\nhelper=sha256:%s\nreason=%s\n' \
    "$1" "$class" "$base" "$head" "$target" "$helper" "$2"
  exit 0
}
full() { emit FULL "$1"; }

[ "${CI_CADENCE_FORCE:-}" = FULL ] && full 'forced-full'
is_oid() { case "$1" in *[!0-9a-f]* | '') return 1 ;; esac; [ ${#1} -eq 40 ]; }
is_oid "$base" && is_oid "$head" && is_oid "$target" || full 'invalid-oid'
[ -n "$root" ] && [ -d "$root" ] || full 'root-missing'
g() { git -C "$root" -c core.quotePath=false "$@"; }
for c in "$base" "$head" "$target"; do
  [ "$(g rev-parse --verify -q "$c^{commit}" 2> /dev/null)" = "$c" ] || full 'commit-missing'
done
case "$event" in
  pull_request)
    # The tested commit must be GitHub's two-parent merge of exactly this base and head.
    [ "$(g rev-list --parents -n 1 "$target" 2> /dev/null)" = "$target $base $head" ] || full 'merge-topology-mismatch' ;;
  merge_group)
    # The queue head is the tested commit; the whole base..head range is classified, never a
    # constituent pull request's decision.
    [ "$head" = "$target" ] || full 'queue-head-mismatch'
    g merge-base --is-ancestor "$base" "$target" 2> /dev/null || full 'queue-base-not-ancestor' ;;
  *) full 'unsupported-event' ;;
esac

tmp=$(mktemp -d 2> /dev/null) || full 'tempdir-unavailable'
trap 'rm -rf "$tmp"' EXIT
g diff --raw -z -M --no-abbrev --no-ext-diff --no-textconv "$base" "$target" -- > "$tmp/diff" 2> /dev/null || full 'diff-unreadable'
[ -s "$tmp/diff" ] || full 'empty-diff'

# Directories holding a non-test Go file with a //go:embed directive (git grep: 0 found, 1 none).
g grep -l -E '^[[:space:]]*//go:embed' "$target" -- '*.go' ':!*_test.go' > "$tmp/embed.raw" 2> /dev/null
case $? in 0 | 1) ;; *) full 'embed-scan-failed' ;; esac
sed -e "s|^$target:||" -e 's|/[^/]*$||' -e 's|^[^/]*\.go$|.|' "$tmp/embed.raw" | sort -u > "$tmp/embed"
grep -qx '\.' "$tmp/embed" && full 'root-embed'

GOOSARCH='aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos unix 386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm'

# test_ok PATH OID: an ordinary Go test file the archive, selection and macOS campaigns cannot read.
test_ok() {
  case "$1" in
    cmd/*_test.go | internal/*_test.go | tools/*_test.go) ;;
    *) return 1 ;;
  esac
  case "$1" in
    */testdata/* | *[!A-Za-z0-9_./-]* | tools/selection-baseline/* | tools/gate-affected-select/* | \
      tools/docs-ci-plan/* | tools/ci-reuse-plan/* | tools/ci-forward-row/* | \
      internal/releasecandidate/* | internal/versionline/* | internal/companionrelease/* | \
      internal/testplatform/* | internal/testconfine/*) return 1 ;;
  esac
  local stem=${1##*/}
  stem=${stem%_test.go}
  local w
  for w in $GOOSARCH; do
    case "$stem" in *_"$w") return 1 ;; esac
  done
  # A test file inside a directory that a non-test //go:embed pattern can reach ships in the binary.
  local dir=${1%/*} e
  while IFS= read -r e; do
    case "$dir/" in "$e"/*) return 1 ;; esac
  done < "$tmp/embed"
  g cat-file blob "$2" > "$tmp/blob" 2> /dev/null || return 1
  # go/build drops a leading byte-order mark, which would hide a build constraint from the grep.
  [ "$(head -c 3 "$tmp/blob" | od -An -tx1 | tr -d ' \n')" = efbbbf ] && return 1
  # Build constraints, embedded files, cgo and TestMain change what a package builds or runs.
  ! grep -Eq '^[[:space:]]*//[[:space:]]*(go:build|\+build)|^[[:space:]]*//go:(embed|linkname|generate)|^[[:space:]]*(import[[:space:]]+)?"C"[[:space:]]*$|GOOS|"darwin"|/private/var|/var/folders|func[[:space:]]+TestMain[[:space:]]*\(' "$tmp/blob"
}

saw_docs=0 saw_tests=0
exec 3< "$tmp/diff"
while IFS= read -r -d '' meta <&3; do
  IFS= read -r -d '' path <&3 || full 'diff-malformed'
  # :<old mode> <new mode> <old oid> <new oid> <status>
  set -- ${meta#:}
  [ $# -eq 5 ] || full 'diff-malformed'
  omode=$1 nmode=$2 noid=$4 status=$5
  case "$status" in
    A) [ "$omode" = 000000 ] || full "unexpected-mode:$path" ;;
    M) [ "$omode" = 100644 ] || full "mode-change:$path" ;;
    R* | C*) full "rename-or-copy:$path" ;;
    D) full "delete:$path" ;;
    *) full "status-$status:$path" ;;
  esac
  [ "$nmode" = 100644 ] || full "mode-$nmode:$path"
  case "$path" in
    docs/build-log/*/* | docs/build-log/.*) full "unclassified:$path" ;;
    docs/build-log/*.md) saw_docs=1; continue ;;
  esac
  if test_ok "$path" "$noid"; then
    saw_tests=1
    continue
  fi
  full "unclassified:$path"
done
exec 3<&-

case "$saw_docs$saw_tests" in
  10) class=DOCS ;;
  01) class=TESTS ;;
  11) class=DOCS+TESTS ;;
  *) full 'empty-diff' ;;
esac
emit DEFER 'routine-change'
