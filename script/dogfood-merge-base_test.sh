#!/usr/bin/env bash
# DWC-V0-7601 (V1-4721, V1-0527): script/dogfood-merge-base.sh derives a merge-forward lane's BASE
# as the merged mainline commit, lists the conflicted paths, keeps the BASE across later bind and
# seal commits, and refuses a missing, octopus, foreign or non-mainline merge. make
# BASE=merge-forward passes the derived SHA to a dogfood target.
set -euo pipefail
export LC_ALL=C

source_root=$(cd "$(dirname "$0")/.." && pwd)
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-merge-base-test.XXXXXX")
trap '/bin/rm -rf "$root"' EXIT
fail() {
  printf 'dogfood-merge-base_test: FAIL line=%s\n' "$1" >&2
  exit 1
}
script=$source_root/script/dogfood-merge-base.sh

git init -q -b main "$root/repo"
cd "$root/repo"
git config user.email test@example.invalid
git config user.name test
printf 'a\n' > shared.txt
git add shared.txt
git commit -q -m base
# The lane has no merge yet.
git checkout -q -b lane
status=0
"$script" > "$root/out" 2> "$root/err" || status=$?
((status == 2)) && grep -q '^dogfood-merge-base: REFUSE no-merge-forward$' "$root/err" || fail "$LINENO"
printf 'lane\n' > shared.txt
git commit -q -am lane
# The mainline moves on and conflicts with the lane.
git checkout -q main
printf 'main\n' > shared.txt
printf 'm\n' > main.txt
git add main.txt
git commit -q -am main
main=$(git rev-parse HEAD)
git update-ref refs/remotes/origin/main "$main"
git checkout -q lane
status=0
"$script" > /dev/null 2> "$root/err" || status=$?
((status == 2)) || fail "$LINENO"
# Merge the mainline forward and resolve the conflict by hand.
git merge -q origin/main > /dev/null 2>&1 || true
printf 'lane and main\n' > shared.txt
git add shared.txt
git commit -q --no-edit
merge=$(git rev-parse HEAD)
[[ $("$script") == "$main" ]] || fail "$LINENO"
[[ $("$script" --paths) == shared.txt ]] || fail "$LINENO"
[[ $("$script" "$merge") == "$main" ]] || fail "$LINENO"
# A failing git show --remerge-diff is refused, not printed as a merge with no conflicted path.
mkdir "$root/shim"
printf '#!/bin/sh\n[ "$1" != show ] || { echo "fatal: injected show failure" >&2; exit 128; }\nexec '"'%s'"' "$@"\n' "$(command -v git)" > "$root/shim/git"
chmod +x "$root/shim/git"
status=0
PATH="$root/shim:$PATH" "$script" --paths > "$root/out" 2> "$root/err" || status=$?
((status == 2)) && [[ ! -s $root/out ]] && grep -q '^dogfood-merge-base: REFUSE remerge-diff-failed$' "$root/err" &&
  grep -q 'exited 128' "$root/err" || fail "$LINENO"
# A later bind or seal commit (one parent) does not move the BASE.
printf 'bind\n' > bind.txt
git add bind.txt
git commit -q -m bind
[[ $("$script") == "$main" ]] || fail "$LINENO"
# make BASE=merge-forward hands the derived SHA to the target.
printf 'include %s/Makefile\nprobe:\n\t@printf "%%s\\n" "$(DOGFOOD_BASE)"\n' "$source_root" > "$root/probe.mk"
mkdir -p script
cp "$script" script/dogfood-merge-base.sh
[[ $(make -s -f "$root/probe.mk" probe BASE=merge-forward 2> /dev/null) == "$main" ]] || fail "$LINENO"
[[ $(make -s -f "$root/probe.mk" probe BASE="$main" 2> /dev/null) == "$main" ]] || fail "$LINENO"
/bin/rm -rf script
# A merged branch that is not on the mainline is refused.
git checkout -q -b other "$main~1"
printf 'o\n' > other.txt
git add other.txt
git commit -q -m other
git checkout -q lane
git merge -q --no-edit other > /dev/null
status=0
"$script" > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q '^dogfood-merge-base: REFUSE merged-parent-not-mainline$' "$root/err" || fail "$LINENO"
# So is a merge outside HEAD, and a missing mainline ref.
status=0
"$script" other > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q 'REFUSE merge-not-two-parent' "$root/err" || fail "$LINENO"
status=0
DOGFOOD_MAINLINE_REF=refs/remotes/nowhere/main "$script" "$merge" > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q 'REFUSE mainline-ref-missing' "$root/err" || fail "$LINENO"
git checkout -q -b elsewhere "$main"
status=0
"$script" "$merge" > /dev/null 2> "$root/err" || status=$?
((status == 2)) && grep -q 'REFUSE merge-not-in-head' "$root/err" || fail "$LINENO"
printf 'DWC-V0-7601 dogfood merge-forward BASE: PASS\n'
