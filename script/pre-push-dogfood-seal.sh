#!/usr/bin/env bash
# Git pre-push hook: refuses pushing a branch whose commits since origin/main are not all bound
# through sealed change evidence (DCW-V0-348, V1-2805, V1-1773). For each pushed branch it runs
# `script/dogfood-batch-check.sh <merge base with origin/main>`, the read-only `corvint dogfood
# batch-check` that names every unbound commit, including one committed after the branch's seal.
# Install it once per repository (linked worktrees share the hooks); Git runs the hook from the
# checkout's top level:
#   printf '#!/bin/sh\nexec script/pre-push-dogfood-seal.sh "$@"\n' > "$(git rev-parse --git-path hooks)/pre-push"
#   chmod +x "$(git rev-parse --git-path hooks)/pre-push"
# Git's stdin can be read once, so a hook that also runs script/pre-push-windows-vet.sh keeps it:
#   refs=$(cat); printf '%s\n' "$refs" | script/pre-push-windows-vet.sh "$@" || exit 1
#   printf '%s\n' "$refs" | exec script/pre-push-dogfood-seal.sh "$@"
#
# Git passes `<local ref> <local sha> <remote ref> <remote sha>` lines on stdin. A deleted ref and a
# ref outside refs/heads/ (a tag) are skipped, and so is a branch with no commit after its merge
# base with CORVINT_PRE_PUSH_BASE (default origin/main). The batch check reads HEAD and builds the
# working tree, so it refuses (`dogfood-seal-not-checked`) a pushed branch that is not the
# checked-out HEAD, a tree with uncommitted tracked changes, or a branch with no merge base.
# CORVINT_PRE_PUSH_BATCH_CHECK overrides the check command (default script/dogfood-batch-check.sh);
# it receives the base as its one argument.
set -uo pipefail

zero=0000000000000000000000000000000000000000
base_ref=${CORVINT_PRE_PUSH_BASE:-origin/main}
check_command=${CORVINT_PRE_PUSH_BATCH_CHECK:-script/dogfood-batch-check.sh}
refuse() {
  printf 'pre-push: REFUSE %s\n' "$*" >&2
  exit 1
}
ranges=()
while read -r local_ref local_sha remote_ref _; do
  [[ -n ${local_sha-} && $local_sha != "${zero:0:${#local_sha}}" ]] || continue
  [[ $remote_ref == refs/heads/* ]] || continue
  base=$(git merge-base "$local_sha" "$base_ref" 2> /dev/null) ||
    refuse "dogfood-seal-not-checked $local_ref has no merge base with $base_ref; fetch it and push again"
  [[ $base != "$local_sha" ]] || continue
  ranges+=("$base $local_sha $local_ref")
done
((${#ranges[@]} > 0)) || exit 0

head=$(git rev-parse HEAD 2> /dev/null) || head=
for entry in "${ranges[@]}"; do
  read -r base sha ref <<< "$entry"
  [[ $sha == "$head" ]] ||
    refuse "dogfood-seal-not-checked $ref is not the checked-out HEAD; check it out and push again, or run make dogfood-batch-check BASE=$base on it and push with --no-verify"
done
[[ -z $(git status --porcelain --untracked-files=no 2> /dev/null) ]] ||
  refuse "dogfood-seal-not-checked the working tree has uncommitted tracked changes, so the batch check would not build the pushed commit; commit or stash them and push again"
for entry in "${ranges[@]}"; do
  read -r base sha ref <<< "$entry"
  printf 'pre-push: checking sealed change evidence for %s..%s (%s)\n' "$base" "$sha" "$ref" >&2
  if ! bash -c "$check_command \"\$1\"" pre-push "$base" >&2; then
    refuse "dogfood-seal-missing range=$base..$sha the batch check above names each commit no sealed CEM binds; bind and seal them (docs/DOGFOOD.md section 4: dogfood-change, dogfood-check, dogfood-seal with BASE=$base) and push again"
  fi
done
