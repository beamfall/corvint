#!/usr/bin/env bash
# usage: script/hlq-local.sh [--snapshot-dirty] [--source DIR] RUNNER-ARG...
#
# Local reproduction of the host lifecycle qualification (HLQ-V1-056). It builds
# ./conformance/host-lifecycle-v1 into a private directory and runs that binary, so the runner's
# exit code reaches the caller unchanged: 0 pass, 1 a failed or NOT_RUN case, 2 a setup error.
# `go run` reports every non-zero exit of its child as 1, which hides that distinction.
#
#   --source DIR       the checkout passed to the runner as --source; default the repository root
#   --snapshot-dirty   when --source has uncommitted tracked edits or untracked (non-ignored) files,
#                      run against a private clone of its HEAD with those edits applied and
#                      committed, instead of the runner refusing the dirty --source. Without it a
#                      dirty --source is still refused by the runner (exit 2).
#   HLQ_RUNNER         an already built runner to use instead of building one (tests)
#
# Every other argument (--host, --corvint, --base-corvint, --report, --failed-reports, --live, ...)
# goes to the runner unchanged. The private directory is removed on exit.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd -P)
source_dir=$root
snapshot=false
forward=()
while [ "$#" -gt 0 ]; do
  case $1 in
    --snapshot-dirty) snapshot=true; shift ;;
    --source)
      [ "$#" -ge 2 ] || { echo "hlq-local: --source needs a directory" >&2; exit 2; }
      source_dir=$2; shift 2 ;;
    --source=*) source_dir=${1#--source=}; shift ;;
    *) forward+=("$1"); shift ;;
  esac
done
source_dir=$(cd "$source_dir" && pwd -P) || { echo "hlq-local: --source is not a directory" >&2; exit 2; }
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-hlq-local.XXXXXX")
trap 'rm -rf "$work"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
# A failed build or snapshot is a setup error (exit 2), never confused with a runner's exit 1.
trap 'echo "hlq-local: setup failed before the runner started" >&2; exit 2' ERR

runner=${HLQ_RUNNER-}
if [ -z "$runner" ]; then
  runner=$work/host-lifecycle-v1
  (cd "$root" && GOTOOLCHAIN=local go build -o "$runner" ./conformance/host-lifecycle-v1)
fi

if [ "$snapshot" = true ] && [ -n "$(git -C "$source_dir" status --porcelain --untracked-files=all)" ]; then
  snap=$work/source
  git clone -q --no-hardlinks "$source_dir" "$snap"
  git -C "$source_dir" diff --binary HEAD >"$work/dirty.patch"
  if [ -s "$work/dirty.patch" ]; then
    git -C "$snap" apply --index --whitespace=nowarn "$work/dirty.patch"
  fi
  git -C "$source_dir" ls-files -z --others --exclude-standard >"$work/untracked"
  while IFS= read -r -d '' path; do
    mkdir -p "$snap/$(dirname "$path")"
    cp -p "$source_dir/$path" "$snap/$path"
    git -C "$snap" add -- "$path"
  done <"$work/untracked"
  git -C "$snap" -c user.name=hlq-local -c user.email=hlq-local@example.invalid \
    -c commit.gpgsign=false commit -q --no-verify -m "hlq-local snapshot of a dirty source"
  echo "hlq-local: --source $source_dir is dirty; running against snapshot $(git -C "$snap" rev-parse HEAD) of its HEAD plus the uncommitted edits" >&2
  source_dir=$snap
fi

trap - ERR
code=0
"$runner" --source "$source_dir" ${forward[@]+"${forward[@]}"} || code=$?
exit "$code"
