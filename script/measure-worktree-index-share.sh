#!/usr/bin/env bash
# usage: measure-worktree-index-share.sh CORVINT SCRATCH [COUNT]
# Measures whether COUNT (default 3) linked worktrees of this repository at one
# commit share the immutable index snapshot, and shows the per-worktree dirty
# view (V1-0198, DIRTY-CACHE-002/003). Worktrees are created under SCRATCH and
# removed on exit; the only other writes are each temporary worktree's
# ignored `.corvint/index/`. Prints one TSV row per step, then the proof lines.
set -uo pipefail
export LC_ALL=C

corvint=${1:?usage: measure-worktree-index-share.sh CORVINT SCRATCH [COUNT]}
scratch=${2:?usage: measure-worktree-index-share.sh CORVINT SCRATCH [COUNT]}
count=${3:-3}
repo=$(cd "$(dirname "$0")/.." && pwd)
task='Where is the index snapshot file path derived from the tree OID and engine digest'
dirty_path=README.md
commit=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
common=$(git -C "$repo" rev-parse --path-format=absolute --git-common-dir) || exit 2
worktrees=()

cleanup() {
  for wt in "${worktrees[@]}"; do
    git -C "$wt" checkout -q -- "$dirty_path"
    git -C "$repo" worktree remove "$wt" || printf 'cleanup: cannot remove %s\n' "$wt" >&2
  done
  printf 'after worktree list:\n'; git -C "$repo" worktree list
  printf 'after status --short:\n'; git -C "$repo" status --short
}

# seconds OUT CMD... runs CMD with stdout in OUT and prints its wall time.
seconds() {
  local out=$1
  shift
  { TIMEFORMAT=%R; time "$@" >"$out" 2>"$out.err"; } 2>&1
}

# fail STEP WT OUT stops the run when a corvint step exits non-zero, so no
# row reports a partial result; the EXIT trap still removes the worktrees.
fail() {
  printf 'measure: %s failed in %s:\n' "$1" "$2" >&2
  cat "$3.err" >&2
  exit 3
}

snapshot_bytes() { find "$1/.corvint/index" -type f -name '*.gob' -exec cat {} + 2>/dev/null | wc -c | tr -d ' '; }
snapshot_oid() { find "$1/.corvint/index" -type f -name '*.gob' -exec git hash-object {} + 2>/dev/null | tr '\n' ' '; }
common_gobs() { find "$common" -name '*.gob' | wc -l | tr -d ' '; }

query() {
  local wt=$1 label=$2 out="$scratch/q.json" t
  t=$(seconds "$out" "$corvint" --root "$wt" query --task "$task" --limit 3) || fail "query $label" "$wt" "$out"
  printf '%s\tquery-%s\t%s\t%s\t%s\t%s\n' "${wt##*/}" "$label" "$t" \
    "$(jq -r '.context.state + "/" + .context.freshness.state + " mixed=" + (.context.freshness.mixed_paths|join(","))' "$out")" \
    "$(snapshot_bytes "$wt")" "$(common_gobs)"
}

index_if_stale() {
  local wt=$1 out="$scratch/i.json" t
  t=$(seconds "$out" "$corvint" --root "$wt" index --if-stale) || fail index "$wt" "$out"
  printf '%s\tindex-if-stale\t%s\t%s\t%s\t%s\n' "${wt##*/}" "$t" \
    "$(jq -r 'if .mutates then "BUILT " + .path else "fresh " + .path end' "$out")" \
    "$(snapshot_bytes "$wt")" "$(common_gobs)"
}

printf 'before worktree list:\n'; git -C "$repo" worktree list
printf 'before status --short:\n'; git -C "$repo" status --short
trap cleanup EXIT
for i in $(seq 1 "$count"); do
  wt="$scratch/wt-share-$i"
  git -C "$repo" worktree add -q --detach "$wt" "$commit" || exit 2
  worktrees+=("$wt")
done
printf 'commit=%s common=%s engine-binary=%s\n' "$commit" "$common" "$corvint"
printf 'worktree\tstep\tseconds\tresult\tworktree_snapshot_bytes\tcommon_dir_gob_files\n'
for wt in "${worktrees[@]}"; do
  query "$wt" cold-no-snapshot
  query "$wt" repeat-no-snapshot
  index_if_stale "$wt"
  query "$wt" warm-snapshot
done
first=${worktrees[0]}
before=$(snapshot_oid "$first")
printf 'dirty edit\n' >>"$first/$dirty_path"
index_if_stale "$first"
query "$first" dirty-snapshot
query "${worktrees[1]}" sibling-while-dirty
git -C "$first" checkout -q -- "$dirty_path"
query "$first" clean-after-dirty
printf 'snapshot oids: before-dirty=%s after-dirty=%s\n' "$before" "$(snapshot_oid "$first")"
for wt in "${worktrees[@]}"; do printf '%s snapshot oid: %s\n' "${wt##*/}" "$(snapshot_oid "$wt")"; done
command rm -f "$scratch/q.json" "$scratch/q.json.err" "$scratch/i.json" "$scratch/i.json.err"
