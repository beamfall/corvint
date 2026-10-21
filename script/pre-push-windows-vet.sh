#!/usr/bin/env bash
# Git pre-push hook: runs `make windows-vet` before a push that changes a .go, go.mod or go.sum file
# and refuses the push when it fails (V1-1725, AFP-V0-054, ASD-V0-023). Install it once per
# repository (linked worktrees share the hooks); Git runs the hook from the checkout's top level:
#   printf '#!/bin/sh\nexec script/pre-push-windows-vet.sh "$@"\n' > "$(git rev-parse --git-path hooks)/pre-push"
#   chmod +x "$(git rev-parse --git-path hooks)/pre-push"
#
# Git passes `<local ref> <local sha> <remote ref> <remote sha>` lines on stdin. A deleted ref is
# skipped. The changed paths are `<remote sha>..<local sha>`, or, for a new remote ref or a remote
# sha this clone lacks, `merge-base <local sha> CORVINT_PRE_PUSH_BASE (default origin/main)`; when
# no base can be found the push counts as changing Go. The vet runs once, on the working tree, so it
# refuses when a pushed Go-changing sha is not the checked-out HEAD or the tree has uncommitted Go
# changes, or when untracked (including ignored) Go files exist, which the vet would read but the
# push does not carry: check out the commit you push, or run the vet on it and push with
# --no-verify. A pushed annotated tag is peeled to its commit before the HEAD comparison.
# CORVINT_PRE_PUSH_VET overrides the vet command (default `make windows-vet`).
set -uo pipefail

zero=0000000000000000000000000000000000000000
base_ref=${CORVINT_PRE_PUSH_BASE:-origin/main}
vet_command=${CORVINT_PRE_PUSH_VET:-make windows-vet}
go_paths=('*.go' '*go.mod' '*go.sum')
needs_vet=()
while read -r local_ref local_sha remote_ref remote_sha; do
  [[ -n ${local_sha-} && $local_sha != "${zero:0:${#local_sha}}" ]] || continue
  base=
  if [[ $remote_sha != "${zero:0:${#remote_sha}}" ]] && git cat-file -e "$remote_sha^{commit}" 2> /dev/null; then
    base=$remote_sha
  else
    base=$(git merge-base "$local_sha" "$base_ref" 2> /dev/null) || base=
  fi
  if [[ -z $base ]]; then
    needs_vet+=("$local_sha $local_ref (no base to diff against)")
  elif ! changed=$(git diff --name-only "$base" "$local_sha" -- "${go_paths[@]}" 2> /dev/null); then
    needs_vet+=("$local_sha $local_ref (diff failed)")
  elif [[ -n $changed ]]; then
    needs_vet+=("$local_sha $local_ref")
  fi
done
((${#needs_vet[@]} > 0)) || exit 0

head=$(git rev-parse HEAD 2> /dev/null) || head=
for entry in "${needs_vet[@]}"; do
  sha=${entry%% *}
  commit=$(git rev-parse -q --verify "$sha^{commit}" 2> /dev/null) || commit=$sha
  if [[ $commit != "$head" ]]; then
    printf 'pre-push: REFUSE windows-vet-not-run %s is not the checked-out HEAD; check it out and push again, or run make windows-vet on it and push with --no-verify\n' "${entry#* }" >&2
    exit 1
  fi
done
if [[ -n $(git status --porcelain --untracked-files=no -- "${go_paths[@]}" 2> /dev/null) ]]; then
  printf 'pre-push: REFUSE windows-vet-not-run the working tree has uncommitted Go changes, so the vet would not check the pushed commit; commit or stash them and push again\n' >&2
  exit 1
fi
if ! untracked=$(git ls-files --others -- "${go_paths[@]}" 2> /dev/null) || [[ -n $untracked ]]; then
  printf 'pre-push: REFUSE windows-vet-not-run untracked Go files would be vetted but are not pushed; remove or commit them and push again\n' >&2
  exit 1
fi
printf 'pre-push: Go inputs changed; running %s\n' "$vet_command" >&2
if ! bash -c "$vet_command" >&2; then
  printf 'pre-push: REFUSE windows-vet-failed fix the Windows vet errors above (GOOS=windows reads only files whose names and build tags admit windows) and push again\n' >&2
  exit 1
fi
