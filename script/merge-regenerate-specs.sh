#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/merge-regenerate-specs.sh REV [MESSAGE]
#
# DRS-V0-002 (V1-1467): merge REV into HEAD with `git merge --no-ff` (MESSAGE, default
# `Merge REV`). When the only conflict is the generated docs/specs/REQUIREMENTS.tsv, as when two
# lanes each add requirement rows, it resolves it inside the merge: it keeps HEAD's side, runs
# `script/gen-spec-requirements.sh --write` over the merged, staged specs (which rewrites and stages
# REQUIREMENTS.tsv, INDEX.json and README.md) and commits the merge with --cleanup=strip, so
# `make dogfood-batch-check` derives it as `regenerated-specs` (DRS-V0-001) instead of failing
# merge-resolution-unbound, and no branch is rewritten afterwards.
# Any other conflict stays in progress for the operator: it lists the paths and exits 1. A merge
# without conflicts commits as plain `git merge --no-ff`. Exit 2 on a usage error or a checkout
# with tracked changes.
#
# Example: script/merge-regenerate-specs.sh claude/lane-x-1102 'Merge lane x into batch'
set -euo pipefail
export LC_ALL=C
usage() { sed -n '3,17p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
[[ $# -ge 1 && $# -le 2 && -n $1 && $1 != -* ]] || usage
rev=$1 message=${2:-"Merge $1"}
generated=docs/specs/REQUIREMENTS.tsv
git rev-parse --verify -q "$rev^{commit}" > /dev/null || { printf 'merge-regenerate-specs: %s is not a commit; fix: name a branch or commit to merge\n' "$rev" >&2; exit 2; }
[[ -z $(git status --porcelain --untracked-files=no) ]] ||
  { printf 'merge-regenerate-specs: the checkout has tracked changes; fix: commit them, then rerun\n' >&2; exit 2; }
if git merge --no-ff --no-edit -m "$message" "$rev"; then
  exit 0
fi
conflicts=$(git diff --name-only --diff-filter=U)
if [[ -z $conflicts ]]; then
  printf 'merge-regenerate-specs: git merge failed without a conflict; fix: read the git output above, then git merge --abort\n' >&2
  exit 1
fi
if [[ $conflicts != "$generated" ]]; then
  printf 'merge-regenerate-specs: conflicts beyond %s stay for you to resolve:\n' "$generated" >&2
  printf '%s\n' "$conflicts" | sed 's/^/  /' >&2
  printf 'merge-regenerate-specs: fix: follow the merge-resolution-unbound recovery in docs/DOGFOOD.md (rebase the conflicting change onto HEAD, resolve, bind, check and seal it), or git merge --abort\n' >&2
  exit 1
fi
git checkout --ours -- "$generated"
git add -- "$generated"
script/gen-spec-requirements.sh --write
git commit -q --no-edit --cleanup=strip
printf 'merge-regenerate-specs: merged %s; the only conflict, %s, was regenerated from the merged specs (DRS-V0-002)\n' "$rev" "$generated"
