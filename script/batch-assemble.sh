#!/usr/bin/env bash
# usage: script/batch-assemble.sh [--dispositions FILE] BRANCH...
#
# Merges sealed lane branches into the checked-out batch branch in one pass, with no per-lane rebind
# or reseal at each new batch tip (ASD-V0-785..786, NEW-D1; docs/LANE-ORCHESTRATION.md section 16).
# Each lane is sealed against its own base first, in parallel (DCW-V0-044). For each BRANCH, in
# order, the merge tree is derived the way `corvint dogfood batch-check` rederives it (DCW-V0-039,
# DCW-V0-073): `git merge-tree --write-tree` in a private bare repository that applies none of this
# repository's configuration, attributes, hooks or merge drivers, writing its objects into this
# repository's object store.
#   - a clean ort merge becomes a two-parent merge commit (derivation=ort);
#   - an ort conflict is merged with -Xours, keeping the batch side of each conflicting hunk
#     (derivation=ort-ours), and its conflicted paths are listed: batch-check fails
#     merge-side-dropped until one sealed change based on the assembled head restores each dropped
#     side, or the merge's message dispositions the hunk (DCW-V0-073): with --dispositions FILE,
#     each FILE line `BRANCH HUNK-ID REASON...` (the id a `dropped-hunk` line of batch-check printed;
#     blank and # lines skipped) adds `Corvint-Dropped-Hunk: HUNK-ID REASON...` to that branch's
#     ort-ours merge message, for a dropped side the restore change cannot hold verbatim, such as a
#     regenerated file;
#   - a branch -Xours cannot resolve (modify/delete, rename) or already in the batch is skipped and
#     the batch is unchanged by it; a skipped conflict takes the per-lane rebind path of
#     docs/DOGFOOD.md section 4 step 2 at the assembled head.
# The branch then fast-forwards to the last merge. Nothing else is written: no seal, CEM, index or
# trace. Output, also written to $(git rev-parse --absolute-git-dir)/corvint/batch-assemble-manifest.txt:
#   batch-assemble: BASE <sha>
#   batch-assemble: MERGE <sha> branch=<name> lane=<sha> derivation=ort|ort-ours [conflicted=N paths=P,...
#     [dispositions=N]]
#   batch-assemble: SKIP branch=<name> lane=<sha> reason=already-merged|ours-unresolved [paths=P,...]
#   batch-assemble: DONE head=<sha> merges=N ort=N ours=N skipped=N
#   batch-assemble: NEXT ... (the restore change, the build at the assembled head, each rebind owed)
# Exit 0 when the pass completed (DONE). Exit 2 on a refusal, which leaves the branch where it was:
# not a Git worktree, detached HEAD, a dirty index or tracked file, an unknown BRANCH or an unreadable
# or malformed dispositions FILE (both checked before any merge), a disposition for a branch that
# made no ort-ours merge, a scratch failure, or a merge-tree, commit-tree or fast-forward failure.
set -uo pipefail
export LC_ALL=C

say() { printf 'batch-assemble: %s\n' "$*"; }
refuse() {
  printf 'batch-assemble: REFUSE %s\n' "$*" >&2
  exit 2
}

usage='script/batch-assemble.sh [--dispositions FILE] BRANCH...'
dispositions=
if [[ ${1-} == --dispositions ]]; then
  [[ $# -ge 2 ]] || refuse "usage: $usage"
  dispositions=$2
  shift 2
fi
[[ $# -gt 0 ]] || refuse "usage: $usage"
top=$(git rev-parse --show-toplevel 2> /dev/null) || refuse 'not-a-git-worktree: run it from the batch worktree'
cd "$top" || refuse "not-a-git-worktree: $top; fix: run it from the batch worktree"
batch=$(git symbolic-ref -q --short HEAD) || refuse 'detached-head: check out the batch branch first'
[[ -z $(git status --porcelain --untracked-files=no) ]] ||
  refuse 'dirty-worktree: commit or remove the tracked changes first; nothing was merged'
base=$(git rev-parse --verify --quiet 'HEAD^{commit}') || refuse 'unborn-head'
lanes=()
for branch in "$@"; do
  lane=$(git rev-parse --verify --quiet "$branch^{commit}") || refuse "unknown-branch $branch: nothing was merged; fix: git fetch the lane branch or name it as REMOTE/BRANCH"
  lanes+=("$lane")
done
# Each disposition line is BRANCH, a 64-hex hunk id and a non-empty reason, for a named BRANCH.
disposition_lines=()
if [[ -n $dispositions ]]; then
  [[ -f $dispositions && -r $dispositions ]] || refuse "dispositions-unreadable $dispositions; fix: pass a readable FILE of BRANCH HUNK-ID REASON lines"
  number=0
  while IFS= read -r line || [[ -n $line ]]; do
    number=$((number + 1))
    [[ $line =~ ^[[:space:]]*(#|$) ]] && continue
    read -r d_branch d_id d_reason <<< "$line"
    [[ $d_id =~ ^[0-9a-f]{64}$ && -n ${d_reason-} ]] ||
      refuse "dispositions-malformed $dispositions:$number: want BRANCH HUNK-ID REASON"
    named=no
    for branch in "$@"; do [[ $branch == "$d_branch" ]] && named=yes; done
    [[ $named == yes ]] || refuse "dispositions-unknown-branch $dispositions:$number: $d_branch is not a BRANCH argument; fix: name that branch as an argument or drop the line"
    disposition_lines+=("$d_branch $d_id $d_reason")
  done < "$dispositions"
fi
used=()
objects=$(git rev-parse --path-format=absolute --git-path objects) || refuse 'objects-unavailable'
git_dir=$(git rev-parse --absolute-git-dir) || refuse 'git-dir-unavailable'

scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-batch-assemble.XXXXXX") || refuse 'scratch-unavailable'
trap '/bin/rm -rf "$scratch"' EXIT
case $scratch/ in "$top"/*) refuse "scratch-inside-repository: set TMPDIR outside $top" ;; esac
# sgit runs Git in the private bare repository: no system, global or repository configuration or
# attributes apply, so no merge driver, hook or filter can run, and new objects land in this
# repository's object store (its alternates still apply).
# Match batch-check's allowlisted Git environment, including neither GIT_CONFIG_PARAMETERS
# nor GIT_CONFIG_COUNT/KEY_*/VALUE_* inherited from the caller.
scratch_env=()
for name in PATH SystemRoot TMPDIR TEMP TMP USERPROFILE; do
  [[ ! ${!name+x} ]] || scratch_env+=("$name=${!name}")
done
scratch_env+=(LANG=C LC_ALL=C GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
  GIT_CONFIG_SYSTEM=/dev/null GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_NO_LAZY_FETCH=1
  GIT_NO_REPLACE_OBJECTS=1 GCM_INTERACTIVE=never GIT_ASKPASS= GIT_ALLOW_PROTOCOL= GIT_ATTR_NOSYSTEM=1)
sgit() {
  (cd "$scratch" && env -i ${scratch_env[@]+"${scratch_env[@]}"} GIT_CEILING_DIRECTORIES="$scratch" \
    GIT_OBJECT_DIRECTORY="$objects" git --git-dir=repository.git -c core.hooksPath=/dev/null \
    -c core.attributesFile=/dev/null -c merge.conflictStyle=merge "$@")
}
format=$(git rev-parse --show-object-format 2> /dev/null) || format=sha1
(cd "$scratch" && env -i ${scratch_env[@]+"${scratch_env[@]}"} git init -q --bare --template= --object-format="$format" repository.git) \
  > /dev/null 2>&1 || refuse 'scratch-repository-unavailable'

manifest_dir=$git_dir/corvint
mkdir -p "$manifest_dir" 2> /dev/null || refuse "manifest-unavailable $manifest_dir; fix: make the Git directory writable, then rerun"
manifest=$manifest_dir/batch-assemble-manifest.txt
: > "$manifest" 2> /dev/null || refuse "manifest-unavailable $manifest; fix: make the Git directory writable, then rerun"
note() {
  say "$*"
  printf 'batch-assemble: %s\n' "$*" >> "$manifest"
}

# merge_tree STRATEGY_OPTION... prints the tree and the conflicted paths, one per line; exit 0 clean,
# 1 conflicted, other on error.
merge_tree() { sgit merge-tree --write-tree --no-messages --name-only "$@"; }

note "BASE $base branch=$batch"
tip=$base
merges=0 ort=0 ours=0 skipped=0
owed=()
index=0
for branch in "$@"; do
  lane=${lanes[$index]}
  index=$((index + 1))
  if git merge-base --is-ancestor "$lane" "$tip"; then
    note "SKIP branch=$branch lane=$lane reason=already-merged"
    skipped=$((skipped + 1))
    continue
  fi
  out=$(merge_tree "$tip" "$lane")
  status=$?
  derivation=ort
  paths=
  if [[ $status == 1 ]]; then
    paths=$(printf '%s\n' "$out" | sed -n '2,$p' | sed '/^$/d' | paste -sd, -)
    count=$(printf '%s\n' "$out" | sed -n '2,$p' | sed '/^$/d' | wc -l | tr -d ' ')
    out=$(merge_tree -Xours "$tip" "$lane")
    status=$?
    derivation=ort-ours
    if [[ $status == 1 ]]; then
      left=$(printf '%s\n' "$out" | sed -n '2,$p' | sed '/^$/d' | paste -sd, -)
      note "SKIP branch=$branch lane=$lane reason=ours-unresolved paths=$left"
      owed+=("rebind $branch: rebase its own work commits onto the assembled head, bind, check and seal with BASE=<assembled head>, then merge it (docs/DOGFOOD.md section 4 step 2)")
      skipped=$((skipped + 1))
      continue
    fi
  fi
  tree=$(printf '%s\n' "$out" | sed -n 1p)
  if [[ $status != 0 || ! $tree =~ ^[0-9a-f]{40}([0-9a-f]{24})?$ ]]; then
    refuse "merge-tree-failed branch=$branch status=$status after $merges merges; the branch $batch is unchanged at $base; fix: run git merge-tree --write-tree $tip $branch to see the error, then rerun"
  fi
  message="Merge $branch into $batch"
  disposed=0
  if [[ $derivation == ort-ours ]]; then
    message+=$'\n\n'"Kept the $batch side of each conflicting hunk (-Xours) in: $paths."
    message+=$'\n'"One sealed change based on the assembled head restores each dropped side (DCW-V0-073)."
    trailers=
    for entry in ${disposition_lines[@]+"${disposition_lines[@]}"}; do
      [[ ${entry%% *} == "$branch" ]] || continue
      trailers+=$'\n'"Corvint-Dropped-Hunk: ${entry#* }"
      disposed=$((disposed + 1))
    done
    if [[ $disposed -gt 0 ]]; then
      message+=$'\n'"$trailers"
      used+=("$branch")
    fi
  fi
  commit=$(git commit-tree "$tree" -p "$tip" -p "$lane" -m "$message") ||
    refuse "commit-tree-failed branch=$branch after $merges merges; the branch $batch is unchanged at $base; fix: set a Git identity or repair the object store named above, then rerun"
  tip=$commit
  merges=$((merges + 1))
  if [[ $derivation == ort ]]; then
    ort=$((ort + 1))
    note "MERGE $commit branch=$branch lane=$lane derivation=ort"
  else
    ours=$((ours + 1))
    extra=
    [[ $disposed == 0 ]] || extra=" dispositions=$disposed"
    note "MERGE $commit branch=$branch lane=$lane derivation=ort-ours conflicted=$count paths=$paths$extra"
  fi
done

# A disposition that reached no merge message is a typo or a stale id list: refuse before moving.
for entry in ${disposition_lines[@]+"${disposition_lines[@]}"}; do
  hit=no
  for branch in ${used[@]+"${used[@]}"}; do [[ ${entry%% *} == "$branch" ]] && hit=yes; done
  [[ $hit == yes ]] ||
    refuse "disposition-unused ${entry%% *}: it made no ort-ours merge; the branch $batch is unchanged at $base; fix: drop that line from the dispositions file, then rerun"
done
if [[ $tip != "$base" ]]; then
  git merge --ff-only -q "$tip" ||
    refuse "fast-forward-failed: $tip holds the assembled merges; the branch $batch is unchanged at $base; fix: run git merge --ff-only $tip"
fi
note "DONE head=$tip merges=$merges ort=$ort ours=$ours skipped=$skipped"
if [[ $ours -gt 0 ]]; then
  note "NEXT restore: commit one change on $tip that restores the dropped side of each ort-ours path, bind, check and seal it with BASE=$tip (DCW-V0-073)"
fi
if [[ $merges -gt 0 ]]; then
  # A textually clean merge of two lanes can still break the build or a test; the per-lane rebind
  # caught that at each tip, so the one pass owes one build at the assembled head instead.
  note "NEXT verify: build and run the affected selection at $tip (corvint affected --base $base); fold any fix into the restore change"
fi
for item in ${owed[@]+"${owed[@]}"}; do
  note "NEXT $item"
done
note "NEXT check: make dogfood-batch-check BASE=$base"
say "manifest $manifest"
exit 0
