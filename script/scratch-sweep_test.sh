#!/usr/bin/env bash
# Temp-fixture tests for script/scratch-sweep.sh (V1-1978, ASD-V0-022).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
tool="$here/scratch-sweep.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/scratch-sweep-test.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
holder=
holder2=
cleanup() {
  [[ -n $holder ]] && kill "$holder" 2> /dev/null || true
  [[ -n $holder2 ]] && kill "$holder2" 2> /dev/null || true
  /bin/rm -rf "$tmp"
}
trap cleanup EXIT INT TERM
fail() { printf 'scratch-sweep_test: FAIL %s\n' "$*" >&2; exit 1; }
expect() { grep -qF -- "$1" "$2" || { cat "$2" >&2; fail "missing: $1"; }; }
export GIT_CONFIG_NOSYSTEM=1 HOME="$tmp/home" GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid \
  GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
mkdir -p "$HOME"
git config --global init.defaultBranch main
q() { git "$@" > /dev/null 2>&1; }
old=$(date -v-3d +%Y%m%d%H%M 2> /dev/null || date -d '3 days ago' +%Y%m%d%H%M)
age() { find "$1" -exec touch -h -t "$old" {} + 2> /dev/null || true; }

origin="$tmp/origin.git"
q init --bare "$origin"
seed="$tmp/seed"
q clone "$origin" "$seed"
echo one > "$seed/a.txt" && q -C "$seed" add a.txt && q -C "$seed" commit -m one && q -C "$seed" push origin main

root="$tmp/root"
mkdir -p "$root"
mkdir -p "$root/plain-old/sub" && echo x > "$root/plain-old/sub/f"
mkdir -p "$root/plain-new" && echo x > "$root/plain-new/f"
mkdir -p "$root/kept" && echo x > "$root/kept/f"
mkdir -p "$root/busy" && echo x > "$root/busy/f"
mkdir -p "$root/nest/a" && q clone "$origin" "$root/nest/a/repo"
q clone "$origin" "$root/clean"
q clone "$origin" "$root/dirty" && echo changed > "$root/dirty/a.txt" && echo new > "$root/dirty/untracked.txt"
q clone "$origin" "$root/unpushed" && echo two > "$root/unpushed/b.txt" && q -C "$root/unpushed" add b.txt && q -C "$root/unpushed" commit -m two
# Deeper than the old 4-level search; a file named by a relative --keep; staged B over a worktree
# back at HEAD's A; two stash entries; a process whose working directory (not argv) is inside.
mkdir -p "$root/deep/a/b/c" && q clone "$origin" "$root/deep/a/b/c/repo"
mkdir -p "$root/filekeep" && echo x > "$root/filekeep/f"
q clone "$origin" "$root/staged" && echo staged-b > "$root/staged/a.txt" && q -C "$root/staged" add a.txt && echo one > "$root/staged/a.txt"
q clone "$origin" "$root/stashes"
for n in 1 2; do echo "s$n" > "$root/stashes/a.txt" && q -C "$root/stashes" stash; done
mkdir -p "$root/cwdbusy" && echo x > "$root/cwdbusy/f"
q -C "$seed" worktree add -b lane "$root/linked"
q -C "$seed" worktree add -b gone "$tmp/gone-wt" && /bin/rm -rf "$tmp/gone-wt"
for d in plain-old kept busy nest clean dirty unpushed linked deep filekeep staged stashes cwdbusy; do age "$root/$d"; done
sh -c 'sleep 60; :' "$root/busy/f" &
holder=$!
(cd "$root/cwdbusy" && exec sleep 60) &
holder2=$!

# 1. Dry run: plans everything, changes nothing.
(cd "$root" && "$tool" --root "$root" --repo "$seed" --keep "$root/kept" --keep filekeep/f) > "$tmp/dry" 2>&1 || fail "dry run exited $?"
expect "WOULD-REMOVE $root/plain-old" "$tmp/dry"
expect "KEEP recent $root/plain-new" "$tmp/dry"
expect "KEEP keep-list $root/kept" "$tmp/dry"
expect "KEEP in-use $root/busy" "$tmp/dry"
expect "KEEP contains-checkout $root/nest" "$tmp/dry"
expect "WOULD-REMOVE $root/clean" "$tmp/dry"
expect "KEEP needs-archive $root/dirty" "$tmp/dry"
expect "KEEP needs-archive $root/unpushed" "$tmp/dry"
expect "WOULD-REMOVE $root/linked" "$tmp/dry"
expect "WOULD-PRUNE $seed" "$tmp/dry"
expect "KEEP contains-checkout $root/deep" "$tmp/dry"
expect "KEEP keep-list $root/filekeep" "$tmp/dry"
expect "KEEP needs-archive $root/staged" "$tmp/dry"
expect "KEEP multiple-stashes $root/stashes" "$tmp/dry"
expect "KEEP in-use $root/cwdbusy" "$tmp/dry"
expect "SUMMARY entries=14 keep=11 remove=3 archive=0 prune=1 applied=no" "$tmp/dry"
[[ -d $root/plain-old && -d $root/clean && -d $root/linked && -f $root/dirty/untracked.txt ]] || fail "dry run removed something"
[[ $(git -C "$seed" worktree list | wc -l | tr -d ' ') == 3 ]] || fail "dry run pruned a registration"

# 2. With an archive dir the dry run plans archive-then-remove for unfinished work.
"$tool" --root "$root" --archive "$tmp/archive" > "$tmp/dry2" 2>&1
expect "WOULD-ARCHIVE-REMOVE $root/dirty" "$tmp/dry2"
expect "WOULD-ARCHIVE-REMOVE $root/unpushed" "$tmp/dry2"
[[ ! -e $tmp/archive ]] || fail "dry run created the archive directory"

# 3. --apply archives, removes and prunes.
(cd "$root" && "$tool" --root "$root" --repo "$seed" --keep "$root/kept" --keep filekeep/f --archive "$tmp/archive" --apply) > "$tmp/apply" 2>&1 || fail "apply exited $?"
expect "REMOVED $root/plain-old" "$tmp/apply"
expect "REMOVED $root/clean" "$tmp/apply"
expect "REMOVED $root/linked" "$tmp/apply"
expect "ARCHIVED $root/dirty $tmp/archive/dirty-" "$tmp/apply"
expect "ARCHIVED $root/unpushed $tmp/archive/unpushed-" "$tmp/apply"
expect "PRUNED $seed" "$tmp/apply"
expect "applied=yes" "$tmp/apply"
expect "ARCHIVED $root/staged $tmp/archive/staged-" "$tmp/apply"
for d in plain-old clean linked dirty unpushed staged; do [[ ! -e $root/$d ]] || fail "$d survived --apply"; done
for d in plain-new kept busy nest deep filekeep stashes cwdbusy; do [[ -d $root/$d ]] || fail "$d removed"; done
st=$(echo "$tmp"/archive/staged-*)
grep -q '^+staged-b' "$st/staged.patch" || fail "staged change not archived"
grep -q '^-staged-b' "$st/worktree.patch" || fail "worktree change not archived"
[[ $(git -C "$seed" worktree list | wc -l | tr -d ' ') == 1 ]] || fail "registrations not pruned"
a=$(echo "$tmp"/archive/dirty-*)
grep -q 'changed' "$a/worktree.patch" || fail "dirty patch missing"
tar -tzf "$a/untracked.tgz" | grep -qx 'untracked.txt' || fail "untracked file not archived"
grep -q '^ M a.txt' "$a/status.txt" || fail "status not archived"
u=$(echo "$tmp"/archive/unpushed-*)
# The bundle is thin (its prerequisites are on the remote): verify and restore into a clone.
git -C "$seed" bundle verify "$u/repo.bundle" > /dev/null 2>&1 || fail "bundle does not verify"
q -C "$seed" fetch "$u/repo.bundle" 'refs/heads/*:refs/restored/*' || fail "bundle fetch failed"
[[ $(git -C "$seed" show refs/restored/main:b.txt) == two ]] || fail "unpushed commit not recoverable"

# 4. An --archive destination inside a candidate keeps that candidate, dry run and --apply.
root2="$tmp/root2"
mkdir -p "$root2" && q clone "$origin" "$root2/dirty2" && echo changed > "$root2/dirty2/a.txt" && age "$root2/dirty2"
"$tool" --root "$root2" --archive "$root2/dirty2/.git/archive" > "$tmp/inside" 2>&1
expect "KEEP archive-dest $root2/dirty2" "$tmp/inside"
"$tool" --root "$root2" --archive "$root2/dirty2/.git/archive" --apply > "$tmp/inside2" 2>&1
expect "KEEP archive-dest $root2/dirty2" "$tmp/inside2"
[[ -f $root2/dirty2/a.txt ]] || fail "candidate holding the archive was removed"

# 5. A failing archive step (git ls-files) keeps the checkout.
mkdir -p "$tmp/shim"
real_git=$(command -v git)
printf '#!/bin/sh\ncase " $* " in *" ls-files "*) exit 1 ;; esac\nexec "%s" "$@"\n' "$real_git" > "$tmp/shim/git"
chmod +x "$tmp/shim/git"
root3="$tmp/root3"
mkdir -p "$root3" && q clone "$origin" "$root3/dirty3" && echo changed > "$root3/dirty3/a.txt" && age "$root3/dirty3"
PATH="$tmp/shim:$PATH" "$tool" --root "$root3" --archive "$tmp/archive3" --apply > "$tmp/lsfail" 2>&1
expect "ARCHIVE-FAILED $root3/dirty3" "$tmp/lsfail"
[[ -f $root3/dirty3/a.txt ]] || fail "checkout removed after a failed archive step"

# 6. Refusals.
set +e
"$tool" > /dev/null 2>&1; rc1=$?
"$tool" --root / > /dev/null 2>&1; rc2=$?
"$tool" --root "$root" --age-hours 0 > /dev/null 2>&1; rc3=$?
set -e
((rc1 == 2 && rc2 == 2 && rc3 == 2)) || fail "refusal exit codes $rc1 $rc2 $rc3"

echo "scratch-sweep_test: ok"
