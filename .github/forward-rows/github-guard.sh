#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-2720, AFP-V0-2721 (V1-4930): does the range BASE..TARGET change a trusted narrowing
# input under .github/? Before this guard any .github/ path forced FULL (AFP-V0-098, AFP-V0-933),
# so a pull request that added an unrelated workflow ran the whole suite. The inputs are the paths
# the trusted narrowing reads or executes, and only those:
#   .github/forward-rows        narrow.sh, narrow.jq, mq-narrow.sh, pr-base.sh, pin-lag.sh,
#                               shard-plan.sh, this guard, and the AFP-V0-097 verdict scripts
#   .github/cishards            the placement helper and its cost, allow and slice tables
#   .github/testconfine         the `go test -exec` wrapper the shards and the shadow run under
#   .github/actions             local actions, should any workflow ever use one
#   .github/workflows/ci-forward-bar.yml, .github/workflows/ci-control-plane.yml
#                               the workflows that produce the rows and the verdict
#   .github/workflows/ci.yml    only the narrowing jobs (docs-plan, narrowing-verdict, shard-plan,
#                               go-product-shard, ci-tested-tree-record, ci-narrowed-tree-record,
#                               go-product) and every line outside the `jobs:` members; another
#                               job of ci.yml is not an input
# Every other .github/ path (another workflow, ci-cadence, macos, flake-report, nightly-signal,
# gate-reuse, qualifications) is not an input and does not force FULL. This list lives in the
# trusted copy of this script, which narrow.sh runs from the pinned tool checkout; the tested
# tree's copy is never read.
# Usage: github-guard.sh BASE TARGET. Prints one word on the first line: 0 (no input changed),
# 1 (an input changed; the inputs follow, one per line) or unreadable (git could not answer; the
# caller records FULL). It exits 0 in every case and reads Git objects only.
set -u
export GIT_NO_REPLACE_OBJECTS=1
base=${1:-}
target=${2:-}
hex() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
unreadable() {
  echo unreadable
  echo "$1"
  exit 0
}
if ! hex "$base" || ! hex "$target"; then unreadable "the base or target is not a full commit id"; fi
inputs='.github/forward-rows .github/cishards .github/testconfine .github/actions .github/workflows/ci-forward-bar.yml .github/workflows/ci-control-plane.yml'
ci=.github/workflows/ci.yml
jobs='docs-plan narrowing-verdict shard-plan go-product-shard ci-tested-tree-record ci-narrowed-tree-record go-product'
tmp=$(mktemp -d "${TMPDIR:-/tmp}/github-guard.XXXXXX") || unreadable "no temporary directory"
trap '/bin/rm -rf "$tmp"' EXIT
# -z prints each path raw, so a non-ASCII path is never display-quoted (AFP-V0-933); --no-renames
# names both sides of a rename. The exit status of git, not its output, decides readability.
# shellcheck disable=SC2086 # the inputs are a fixed list of pathspecs
git diff --name-only -z --no-renames "$base" "$target" -- $inputs > "$tmp/changed" 2> /dev/null ||
  unreadable "git diff failed for the narrowing inputs"
tr '\0' '\n' < "$tmp/changed" > "$tmp/changed.txt"
# section FILE NAME: the lines of ci.yml FILE that belong to NAME: a job's block (its key line to
# the next 2-space key or top-level key), or, for "-", every line outside the members of `jobs:`.
# A job key is exactly two spaces, the name and a colon; any other layout leaves the lines where
# they are, so a guarded block that moves or vanishes differs from the base and forces FULL.
section() {
  awk -v want="$2" '
    BEGIN { injobs = 0; cur = "-" }
    {
      if (!injobs) {
        if ($0 ~ /^jobs:[ \t]*$/) { injobs = 1; cur = "-" }
        if (want == "-") print
        next
      }
      if ($0 ~ /^[^ \t#]/) { injobs = 0; cur = "-"; if (want == "-") print; next }
      if ($0 ~ /^  [A-Za-z0-9_-]+:[ \t]*$/) { cur = $0; sub(/^  /, "", cur); sub(/:[ \t]*$/, "", cur) }
      if (cur == want) print
    }' "$1"
}
rc=0
git diff --quiet --no-renames "$base" "$target" -- "$ci" > /dev/null 2>&1 || rc=$?
case $rc in
0) ;;
1)
  git show "$base:$ci" > "$tmp/base.yml" 2> /dev/null || unreadable "$ci at the base is unreadable"
  # A deleted or renamed ci.yml compares as empty, so every guarded section differs.
  git show "$target:$ci" > "$tmp/target.yml" 2> /dev/null || : > "$tmp/target.yml"
  # A YAML anchor lets one block rewrite another, which no section compare can attribute.
  if grep -Eq '(^|[[:space:]])&[A-Za-z_]' "$tmp/base.yml" "$tmp/target.yml"; then
    echo "$ci (a YAML anchor)" >> "$tmp/changed.txt"
  fi
  for name in - $jobs; do
    section "$tmp/base.yml" "$name" > "$tmp/a"
    section "$tmp/target.yml" "$name" > "$tmp/b"
    cmp -s "$tmp/a" "$tmp/b" || echo "$ci ($name)" >> "$tmp/changed.txt"
  done
  ;;
*) unreadable "git diff failed for $ci" ;;
esac
if [ -s "$tmp/changed.txt" ]; then
  echo 1
  cat "$tmp/changed.txt"
else
  echo 0
fi
