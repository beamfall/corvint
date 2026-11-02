#!/usr/bin/env bash
# tasks-carry-projection.sh: carry the tracked .taskman projection from the authoritative Tasks
# store into a worktree (ASD-V0-815, V1-4183), the post-merge step that keeps origin/main's
# projection, and so every fresh lane's, at the store's ticket count.
#
#   script/tasks-carry-projection.sh [--store DIR] [--commit] WORKTREE
#
# DIR is the worktree holding the authoritative store (default: WORKTREE's primary worktree, the
# first `git worktree list` entry). It refuses when WORKTREE is DIR, when another carry into WORKTREE
# holds its lock (an flock(2) on WORKTREE's Git directory/tasks-carry-projection.lock, held through
# the commit), when `git status` fails, when WORKTREE has any change under .taskman/, or when the two
# queue.json files name different queueIds. It runs
# `corvint-tasks queue snapshot` in DIR, replaces WORKTREE/.taskman/ with DIR's queue.json,
# policy.json, tickets/ and releases/, then requires every carried file to match the snapshot's
# sha256 and no other file to be there. One line per finding:
#   DRIFT    a carried file differs from the snapshot (the store moved during the carry; rerun)
#   MISSING  a snapshot file is absent from WORKTREE/.taskman/ after the carry
#   EXTRA    WORKTREE/.taskman/ holds a file the snapshot does not list
# With no finding the carried paths are staged; with --commit it commits the carried paths alone as
# "chore(tasks): carry the QUEUE projection to main". Exit 0 with a PASS line naming the ticket
# count before and after and the changed-file count, 1 on any finding, 2 on a usage error or a
# refusal. It never pushes. CORVINT_TASKS_BIN overrides corvint-tasks (for tests).
set -uo pipefail
export LC_ALL=C
me='tasks-carry-projection'
usage() { printf 'usage: script/tasks-carry-projection.sh [--store DIR] [--commit] WORKTREE\n' >&2; exit 2; }
refuse() { printf '%s: %s\n' "$me" "$*" >&2; exit 2; }
tasks=${CORVINT_TASKS_BIN:-corvint-tasks} store='' commit=0
while (($#)); do
  case $1 in
    --store) { (($# >= 2)) && [[ -n $2 ]]; } || usage; store=$2; shift 2 ;;
    --commit) commit=1; shift ;;
    -*) usage ;;
    *) break ;;
  esac
done
(($# == 1)) || usage
wt=$(cd "$1" 2> /dev/null && git rev-parse --show-toplevel 2> /dev/null) || refuse "'$1' is not a Git worktree; use a worktree path"
if [[ -z $store ]]; then
  store=$(git -C "$wt" worktree list --porcelain | sed -n '1s/^worktree //p')
fi
store=$(cd "$store" 2> /dev/null && pwd -P) || refuse "store '$store' is not a directory; use --store DIR with the store's worktree"
[[ $(cd "$wt" && pwd -P) != "$store" ]] || refuse "WORKTREE is the store itself; use a separate worktree, such as git worktree add --detach ../carry origin/main"
[[ -s $store/.taskman/queue.json ]] || refuse "$store/.taskman/queue.json is absent; use --store DIR to name the store"
# One carry per destination at a time: the flock(2) on fd 9 is held from this preflight through the
# replacement, the verification and the commit, and the kernel drops it when the script exits.
lock="$(git -C "$wt" rev-parse --absolute-git-dir)/$me.lock" || refuse "cannot resolve the Git directory of $wt; run git -C $wt rev-parse --absolute-git-dir to see why"
exec 9>> "$lock" || refuse "cannot open $lock; fix: make the Git directory of $wt writable"
perl -MFcntl=:flock -e 'open(my $h, ">>&=", 9) or exit 2; exit(flock($h, LOCK_EX | LOCK_NB) ? 0 : 1)' ||
  refuse "another carry into $wt holds $lock; wait for it to finish, then rerun"
status=$(git -C "$wt" status --porcelain -- .taskman) || refuse "git status failed in $wt; run git -C $wt status to see why"
[[ -z $status ]] || refuse "$wt has changes under .taskman/; fix: commit or remove them, then rerun"
qid=$(jq -er .queueId "$store/.taskman/queue.json") || refuse "$store/.taskman/queue.json has no queueId; use --store DIR to name an initialized store"
if [[ -e $wt/.taskman/queue.json ]]; then
  [[ $(jq -r .queueId "$wt/.taskman/queue.json") == "$qid" ]] || refuse "$wt/.taskman/queue.json names another queue than $qid; use --store DIR to name that queue's store"
fi
if command -v sha256sum > /dev/null; then sum=(sha256sum); else sum=(shasum -a 256); fi
stage=$(mktemp -d "${TMPDIR:-/tmp}/$me.XXXXXX") || refuse 'mktemp failed; fix: set TMPDIR to a writable directory'
trap '/bin/rm -rf "$stage"' EXIT
(cd "$store" && "$tasks" queue snapshot --out "$stage/snapshot.json" > /dev/null 2>&1) && [[ -s $stage/snapshot.json ]] ||
  refuse "corvint-tasks queue snapshot refused or wrote nothing in $store; run it there to see why"
jq -r '.files[] | "\(.sha256)  \(.path)"' "$stage/snapshot.json" | sort > "$stage/want" || refuse 'the snapshot has no files[] list; fix: use a corvint-tasks that writes taskman-intent-snapshot/0'
before=$(git -C "$wt" ls-files -- .taskman/tickets | wc -l | tr -d ' ')
/bin/rm -rf "$wt/.taskman/tickets" "$wt/.taskman/releases" "$wt/.taskman/queue.json" "$wt/.taskman/policy.json"
mkdir -p "$wt/.taskman"
for part in queue.json policy.json tickets releases; do
  [[ -e $store/.taskman/$part ]] && /bin/cp -Rf "$store/.taskman/$part" "$wt/.taskman/"
done
(cd "$wt/.taskman" && find . -type f | sed 's#^\./##' | sort | tr '\n' '\0' | xargs -0 "${sum[@]}") | sort > "$stage/have"
sed "s/^[^ ]*  //" "$stage/want" | sort > "$stage/want.paths"
sed "s/^[^ ]*  //" "$stage/have" | sort > "$stage/have.paths"
comm -23 "$stage/want" "$stage/have" | sed "s/^[^ ]*  //" | sort | comm -12 - "$stage/have.paths" | sed "s/^/$me: DRIFT /; s/\$/ differs from the snapshot; the store moved during the carry, rerun/" > "$stage/findings"
comm -23 "$stage/want.paths" "$stage/have.paths" | sed "s/^/$me: MISSING /; s/\$/ is absent after the carry/" >> "$stage/findings"
comm -13 "$stage/want.paths" "$stage/have.paths" | sed "s/^/$me: EXTRA /; s/\$/ is not in the snapshot/" >> "$stage/findings"
cat "$stage/findings"
findings=$(wc -l < "$stage/findings" | tr -d ' ')
((findings == 0)) || { printf '%s: FAIL %d findings; %s/.taskman is not committed\n' "$me" "$findings" "$wt"; exit 1; }
git -C "$wt" add -A -- .taskman || refuse "git add .taskman failed in $wt; run git status there"
changed=$(git -C "$wt" diff --cached --name-only -- .taskman | wc -l | tr -d ' ')
after=$(git -C "$wt" ls-files -- .taskman/tickets | wc -l | tr -d ' ')
if ((commit && changed > 0)); then
  git -C "$wt" commit -q -m "chore(tasks): carry the $qid projection to main" -- .taskman || refuse "git commit failed in $wt; run git status there"
fi
printf '%s: PASS %s tickets %s -> %s, %s files changed%s\n' "$me" "$qid" "$before" "$after" "$changed" \
  "$( ((commit && changed > 0)) && printf ', committed %s' "$(git -C "$wt" rev-parse --short HEAD)")"
